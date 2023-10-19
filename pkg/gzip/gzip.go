package gzip

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter

	isZipped bool
}

func (w gzipResponseWriter) WriteHeader(code int) {
	if w.ResponseWriter.Header().Get("Content-Encoding") == "gzip" {
		w.isZipped = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	if w.isZipped {
		return w.ResponseWriter.Write(b)
	} else {
		return w.Writer.Write(b)
	}
}

func GzipHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
			h.ServeHTTP(w, req)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		gzw := gzipResponseWriter{Writer: gz, ResponseWriter: w}
		h.ServeHTTP(gzw, req)
	})
}
