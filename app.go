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
	"path"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

const PicturesFormat string = ".png"

type AppConfig struct {
	HttpUsername string
	HttpPassword string

	PicturesDirectory string
}

type App struct {
	db        *sql.DB
	contentFs fs.FS
	config    *AppConfig

	// access by multiple goroutines. requires lock
	pictures     []Picture
	picturesLock sync.Mutex

	// accessed by a single goroutine, no lock required
	pictureFiles map[string]*PictureFile
}

type LocationMarker struct {
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

type Picture struct {
	Filename  string  `json:"filename"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type PictureFile struct {
	Filename    string
	Latitude    float64
	Longitude   float64
	LastModTime time.Time
}

//go:embed content/*
var appContentFs embed.FS

//go:embed schema.sql
var appDbSchema string

func NewApp(config *AppConfig) (*App, error) {
	db, err := sql.Open("sqlite3", "file:database.db")
	if err != nil {
		return nil, errors.Wrapf(err, "opening sqlite database")
	}

	if _, err := db.Exec(appDbSchema); err != nil {
		return nil, errors.Wrapf(err, "executing db schema")
	}

	contentFs, err := fs.Sub(appContentFs, "content")
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(config.PicturesDirectory, 0o755); err != nil {
		return nil, errors.Wrapf(err, "creating pictures directory")
	}

	a := &App{
		db:           db,
		contentFs:    contentFs,
		config:       config,
		pictures:     []Picture{},
		pictureFiles: make(map[string]*PictureFile),
	}

	go func() {
		for {
			a.updatePictures()
			time.Sleep(time.Minute)
		}
	}()

	return a, nil
}

func (a *App) Close() {
	a.db.Close()
}

func (a *App) Mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/location", a.requireBasicAuth(a.handleGetLocationMarkers))
	mux.HandleFunc("POST /api/location", a.requireBasicAuth(a.handleCreateLocationMarker))
	mux.HandleFunc("GET /api/shapes", a.handleGetShapes)
	mux.HandleFunc("POST /api/shapes", a.requireBasicAuth(a.handleUpdateShapes))
	mux.HandleFunc("GET /api/pictures", a.handleGetPictures)
	mux.Handle("GET /static/", http.FileServerFS(a.contentFs))
	mux.HandleFunc("GET /picture/{filename}", a.handleServePicture)
	mux.HandleFunc("GET /logger", a.requireBasicAuth(a.serveContentFile("logger.html")))
	mux.HandleFunc("GET /editor", a.requireBasicAuth(a.serveContentFile("editor.html")))
	mux.HandleFunc("GET /", a.serveContentFile("index.html"))
	return mux
}

func (a *App) writeJson(w http.ResponseWriter, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "failed to encoded response as json", http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.Write(body)
}

func (a *App) requireBasicAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		required_username := a.config.HttpUsername
		required_password := a.config.HttpPassword
		if !ok || username != required_username || password != required_password {
			w.Header().Add("WWW-Authenticate", "Basic realm=\"User Visible Realm\"")
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

func (a *App) serveContentFile(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, a.contentFs, path)
	}
}

func (a *App) loadLocationMarkers(ctx context.Context) ([]LocationMarker, error) {
	rows, err := a.db.QueryContext(ctx, "SELECT * FROM location")
	if err != nil {
		return nil, errors.Wrapf(err, "fetching all locations")
	}

	locations := []LocationMarker{}
	for rows.Next() {
		loc := LocationMarker{}
		if err := rows.Scan(&loc.Timestamp, &loc.Latitude, &loc.Longitude, &loc.Accuracy, &loc.Heading); err != nil {
			return nil, errors.Wrapf(err, "failed to scan location row")
		}
		locations = append(locations, loc)
	}

	return locations, nil
}

func (a *App) appendLocationMarker(ctx context.Context, loc LocationMarker) error {
	if _, err := a.db.ExecContext(ctx, "INSERT INTO location(timestamp, latitude, longitude, accuracy, heading) VALUES (?, ?, ?, ?, ?)", loc.Timestamp, loc.Latitude, loc.Longitude, loc.Accuracy, loc.Heading); err != nil {
		return errors.Wrapf(err, "failed to insert location log")
	}
	return nil
}

func (a *App) updateShapes(ctx context.Context, shapes []Shape) error {
	tx, err := a.db.Begin()
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

func (a *App) loadShapes(ctx context.Context) ([]Shape, error) {
	shapes := []Shape{}
	srows, err := a.db.QueryContext(ctx, "SELECT id, kind FROM shape")
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
		prows, err := a.db.QueryContext(ctx, "SELECT latitude, longitude FROM shape_point WHERE shape = ?", id)
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

func (a *App) handleCreateLocationMarker(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("failed to read request body: %v\n", err)
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	l := LocationMarker{}
	if err := json.Unmarshal(body, &l); err != nil {
		log.Printf("failed to parse request body: %v\n", err)
		http.Error(w, "failed to parse request body", http.StatusBadRequest)
		return
	}

	log.Printf("storing %v\n", l)
	if err := a.appendLocationMarker(r.Context(), l); err != nil {
		log.Printf("%v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
}

func (a *App) handleGetLocationMarkers(w http.ResponseWriter, r *http.Request) {
	locations, err := a.loadLocationMarkers(r.Context())
	if err != nil {
		log.Printf("internal error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	a.writeJson(w, locations)
}

func (a *App) handleGetShapes(w http.ResponseWriter, r *http.Request) {
	shapes, err := a.loadShapes(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to fetch shapes: %v", err), http.StatusInternalServerError)
		return
	}
	a.writeJson(w, shapes)
}

func (a *App) handleUpdateShapes(w http.ResponseWriter, r *http.Request) {
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

	if err := a.updateShapes(r.Context(), shapes); err != nil {
		http.Error(w, fmt.Sprintf("failed to store shapes: %v", err), http.StatusInternalServerError)
		return
	}
}

func (a *App) updatePictures() {
	entries, err := os.ReadDir(a.config.PicturesDirectory)
	if err != nil {
		log.Fatalf("failed to read pictures directory: %v\n", err)
	}

	for _, entry := range entries {
		// ignore non files
		if !entry.Type().IsRegular() {
			continue
		}

		// convert if required
		filePath := path.Join(a.config.PicturesDirectory, entry.Name())
		fileExt := path.Ext(entry.Name())
		if fileExt != PicturesFormat {
			filePathNew := strings.TrimSuffix(filePath, fileExt) + PicturesFormat
			if _, err := os.Stat(filePathNew); err == nil {
				continue
			}
			if err := magickConvert(filePath, filePathNew); err != nil {
				log.Printf("failed to convert image: %v", err)
			}
		}

		// process file
		info, err := os.Stat(filePath)
		if err != nil {
			log.Printf("failed to stat %v: %v\n", filePath, err)
			continue
		}
		if pfile, ok := a.pictureFiles[filePath]; !ok || info.ModTime().After(pfile.LastModTime) {
			exifData, err := exiftool(filePath)
			if err != nil {
				log.Printf("failed to extract exif data from %v: %v\n", filePath, err)
				continue
			}

			pictureFile := &PictureFile{
				Filename:    entry.Name(),
				Latitude:    exifData.Latitude,
				Longitude:   exifData.Longitude,
				LastModTime: info.ModTime(),
			}
			a.pictureFiles[filePath] = pictureFile
		}
	}

	a.picturesLock.Lock()
	defer a.picturesLock.Unlock()
	a.pictures = make([]Picture, 0, len(a.pictureFiles))
	for _, pfile := range a.pictureFiles {
		a.pictures = append(a.pictures, Picture{
			Filename:  pfile.Filename,
			Latitude:  pfile.Latitude,
			Longitude: pfile.Longitude,
		})
	}
}

func (a *App) handleGetPictures(w http.ResponseWriter, r *http.Request) {
	a.picturesLock.Lock()
	response, err := json.Marshal(a.pictures)
	a.picturesLock.Unlock()
	if err != nil {
		http.Error(w, "failed to marshal pictures", http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.Write(response)
}

func (a *App) handleServePicture(w http.ResponseWriter, r *http.Request) {
	filename := path.Base(r.PathValue("filename"))
	if !strings.HasSuffix(filename, PicturesFormat) {
		http.Error(w, "picture not found", http.StatusNotFound)
		return
	}

	filepath := path.Join(a.config.PicturesDirectory, filename)
	http.ServeFile(w, r, filepath)
}
