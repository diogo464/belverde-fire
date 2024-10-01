package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	_ "embed"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
)

type LocationLog struct {
	Timestamp float64 `json:"timestamp"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Accuracy  float64 `json:"accuracy"`
	Heading   float64 `json:"heading"`
}

type ShapePoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

const (
	ShapeKindUnburned string = "unburned"
	ShapeKindBurned   string = "burned"
)

type Shape struct {
	Kind   string       `json:"kind"`
	Points []ShapePoint `json:"points"`
}

var db *sql.DB

func dbLoadLocationLogs(ctx context.Context) ([]LocationLog, error) {
	rows, err := db.QueryContext(ctx, "SELECT * FROM location")
	if err != nil {
		return nil, errors.Wrapf(err, "fetching all locations")
	}

	locations := []LocationLog{}
	for rows.Next() {
		loc := LocationLog{}
		if err := rows.Scan(&loc.Timestamp, &loc.Latitude, &loc.Longitude, &loc.Accuracy, &loc.Heading); err != nil {
			return nil, errors.Wrapf(err, "failed to scan location row")
		}
		locations = append(locations, loc)
	}

	return locations, nil
}

func dbAppendLocationLog(ctx context.Context, loc LocationLog) error {
	if _, err := db.ExecContext(ctx, "INSERT INTO location(timestamp, latitude, longitude, accuracy, heading) VALUES (?, ?, ?, ?, ?)", loc.Timestamp, loc.Latitude, loc.Longitude, loc.Accuracy, loc.Heading); err != nil {
		return errors.Wrapf(err, "failed to insert location log")
	}
	return nil
}

func dbSetShapes(ctx context.Context, shapes []Shape) error {
	tx, err := db.Begin()
	if err != nil {
		return errors.Wrapf(err, "starting database transaction")
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM shape"); err != nil {
		return errors.Wrapf(err, "failed to truncate shapes table")
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM shape_point"); err != nil {
		return errors.Wrapf(err, "faile to truncate shape points tables")
	}

	for shapeId, shape := range shapes {
		if _, err := tx.ExecContext(ctx, "INSERT INTO shape(id, kind) VALUES (?, ?)", shapeId, shape.Kind); err != nil {
			return errors.Wrapf(err, "failed to insert shape")
		}

		for _, point := range shape.Points {
			if _, err := tx.ExecContext(ctx, "INSERT INTO shape_point(shape, latitude, longitude) VALUES (?, ?, ?)", shapeId, point.Latitude, point.Longitude); err != nil {
				return errors.Wrapf(err, "failed to insert shape point")
			}
		}
	}

	return tx.Commit()
}

func dbGetShapes(ctx context.Context) ([]Shape, error) {
	shapes := []Shape{}
	srows, err := db.QueryContext(ctx, "SELECT id, kind FROM shape")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch shapes")
	}

	for srows.Next() {
		var id int
		var kind string
		if err := srows.Scan(&id, &kind); err != nil {
			return nil, errors.Wrapf(err, "failed to scan shape")
		}

		points := []ShapePoint{}
		prows, err := db.QueryContext(ctx, "SELECT latitude, longitude FROM shape_point WHERE shape = ?", id)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to fetch shape points")
		}

		for prows.Next() {
			var lat float64
			var lon float64
			if err := prows.Scan(&lat, &lon); err != nil {
				return nil, errors.Wrapf(err, "failed to scan shape point")
			}

			points = append(points, ShapePoint{
				Latitude:  lat,
				Longitude: lon,
			})
		}

		shapes = append(shapes, Shape{
			Kind:   kind,
			Points: points,
		})
	}

	return shapes, nil
}

func handlePostApiLog(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("failed to read request body: %v\n", err)
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	l := LocationLog{}
	if err := json.Unmarshal(body, &l); err != nil {
		log.Printf("failed to parse request body: %v\n", err)
		http.Error(w, "failed to parse request body", http.StatusBadRequest)
		return
	}

	log.Printf("storing %v\n", l)
	if err := dbAppendLocationLog(r.Context(), l); err != nil {
		log.Printf("%v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
}

func handleGetApiLog(w http.ResponseWriter, r *http.Request) {
	locations, err := dbLoadLocationLogs(r.Context())
	if err != nil {
		log.Printf("internal error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	marshaled, err := json.Marshal(locations)
	if err != nil {
		log.Printf("failed to marshal locations: %v\n", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Write(marshaled)
}

func handleGetApiShapes(w http.ResponseWriter, r *http.Request) {
	shapes, err := dbGetShapes(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to fetch shapes: %v", err), http.StatusInternalServerError)
		return
	}

	marshaled, err := json.Marshal(shapes)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to marshal shapes: %v", err), http.StatusInternalServerError)
		return
	}

	w.Write(marshaled)
}

func handlePostApiShapes(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("failed to read request body: %v\n", err)
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	shapes := []Shape{}
	if err := json.Unmarshal(body, &shapes); err != nil {
		http.Error(w, fmt.Sprintf("failed to unmarshal request body: %v", err), http.StatusBadRequest)
		return
	}

	if err := dbSetShapes(r.Context(), shapes); err != nil {
		http.Error(w, fmt.Sprintf("failed to store shapes: %v", err), http.StatusInternalServerError)
		return
	}
}

func requireBasicAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		required_username := os.Getenv("HTTP_USERNAME")
		required_password := os.Getenv("HTTP_PASSWORD")
		if !ok || username != required_username || password != required_password {
			w.Header().Add("WWW-Authenticate", "Basic realm=\"User Visible Realm\"")
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

//go:embed schema.sql
var schema string

//go:embed content/*
var contentFs embed.FS

func main() {
	var err error

	db, err = sql.Open("sqlite3", "file:database.db")
	if err != nil {
		log.Fatalf("failed to create sqlite database: %v\n", err)
	}
	defer db.Close()

	if _, err := db.Exec(schema); err != nil {
		log.Fatalf("failed to execute db schema: %v\n", err)
	}

	fs, err := fs.Sub(contentFs, "content")
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("GET /api/location", handleGetApiLog)
	http.HandleFunc("POST /api/location", requireBasicAuth(handlePostApiLog))
	http.HandleFunc("GET /api/shapes", handleGetApiShapes)
	http.HandleFunc("POST /api/shapes", requireBasicAuth(handlePostApiShapes))
	http.Handle("GET /static/", http.FileServerFS(fs))
	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, fs, "index.html")
	})
	http.HandleFunc("GET /logger", requireBasicAuth(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, fs, "logger.html")
	}))
	http.HandleFunc("GET /shapes", requireBasicAuth(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, fs, "shapes.html")
	}))

	fsHandler := http.FileServerFS(fs)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && !strings.Contains(r.URL.Path, ".") {
			r.URL.Path += ".html"
		}
		fsHandler.ServeHTTP(w, r)
	})

	if err := http.ListenAndServe("0.0.0.0:8000", nil); err != nil {
		log.Fatalf("http.ListenAndServe failed: %v", err)
	}
}
