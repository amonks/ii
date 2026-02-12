package serve

import (
	"encoding/json"
	"net/http"
)

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
