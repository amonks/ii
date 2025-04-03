package serve

import (
	"bytes"
	"encoding/json"
	"net/http"
	"path"
	"path/filepath"
	"time"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"monks.co/pkg/env"
)

func StaticServer(staticDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		Static(w, req, staticDir)
	})
}

func Static(w http.ResponseWriter, req *http.Request, staticDir string) {
	path := filepath.Join(staticDir, path.Base(req.URL.Path))
	http.ServeFile(w, req, path)
}

func JSServer(tsFilePath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ServeJS(w, req, tsFilePath)
	})
}

func ServeJS(w http.ResponseWriter, req *http.Request, tsfilepath string) {
	result := esbuild.Build(esbuild.BuildOptions{
		EntryPoints: []string{env.InMonksRoot("apps/map/ts/index.ts")},
		Bundle:      true,
		Write:       false,
	})
	if len(result.Errors) > 0 {
		InternalServerErrorf(w, req, "%s", result.Errors[0].Text)
		return
	}
	if len(result.OutputFiles) != 1 {
		InternalServerErrorf(w, req, "failed to produce js file")
		return
	}
	http.ServeContent(w, req, "index.js", time.Now(), bytes.NewReader(result.OutputFiles[0].Contents))
}

// JSON encodes the given data as JSON and writes it to the response writer.
// If encoding fails, it responds with an internal server error.
// Uses json.Encoder to stream directly to the ResponseWriter.
func JSON(w http.ResponseWriter, req *http.Request, data interface{}) {
	// Set content type if not already set
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	
	// Write the HTTP status code
	w.WriteHeader(http.StatusOK)
	
	// Create an encoder that writes to the response writer
	enc := json.NewEncoder(w)
	
	// Encode the data directly to the ResponseWriter
	if err := enc.Encode(data); err != nil {
		// Log encode errors, but can't really respond with an error at this point
		// since we've already written the status code
		InternalServerErrorf(w, req, "failed to encode JSON response: %v", err)
		return
	}
}
