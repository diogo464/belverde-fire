package main

import (
	"log"
	"net/http"
	"os"

	_ "embed"

	_ "github.com/mattn/go-sqlite3"
)

const (
	ENV_VAR_HTTP_USERNAME = "HTTP_USERNAME"
	ENV_VAR_HTTP_PASSWORD = "HTTP_PASSWORD"
)

func main() {
	if os.Getenv(ENV_VAR_HTTP_USERNAME) == "" || os.Getenv(ENV_VAR_HTTP_PASSWORD) == "" {
		log.Fatalf("http username and password cannot be empty")
	}

	config := &AppConfig{
		HttpUsername:      os.Getenv(ENV_VAR_HTTP_USERNAME),
		HttpPassword:      os.Getenv(ENV_VAR_HTTP_PASSWORD),
		PicturesDirectory: "pictures",
	}
	app, err := NewApp(config)
	if err != nil {
		log.Fatalf("failed to create app: %v\n", err)
	}
	defer app.Close()

	mux := app.Mux()
	if err := http.ListenAndServe("0.0.0.0:8000", mux); err != nil {
		log.Fatalf("http.ListenAndServe failed: %v", err)
	}
}
