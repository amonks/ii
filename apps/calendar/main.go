package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"

	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	reqlog.SetupLogging()

	mux := serve.NewMux()

	// Initialize storage
	dataDir := "data"
	storage, err := NewStorage(filepath.Join(dataDir, "storage.json"))
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Handler for the main page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		data, err := content.ReadFile("index.html")
		if err != nil {
			http.Error(w, "Could not read index.html", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})

	// Handler for storage API
	mux.HandleFunc("/storage", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case "GET":
			// Return all stored data
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(storage.data)
			w.Write(response)

		case "POST":
			// Read request body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Error reading request body", http.StatusBadRequest)
				return
			}

			// Parse JSON
			var data map[string]string
			if err := json.Unmarshal(body, &data); err != nil {
				http.Error(w, "Error parsing JSON", http.StatusBadRequest)
				return
			}

			// Update storage
			for key, value := range data {
				storage.Set(key, value)
			}

			// Save to disk
			if err := storage.Save(); err != nil {
				http.Error(w, "Error saving data", http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	ctx := sigctx.New()
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}
	if err := tailnet.ListenAndServe(ctx, reqlog.Middleware().ModifyHandler(gzip.Middleware(mux))); err != nil {
		return err
	}

	return nil
}

//go:embed index.html
var content embed.FS
