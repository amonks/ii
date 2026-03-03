package gzip

import (
	"compress/gzip"
	"net/http"
	"strings"

	"monks.co/pkg/middleware"
)

var Middleware = middleware.MiddlewareFunc(func(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
			h.ServeHTTP(w, req)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		gzw := &gzipResponseWriter{ResponseWriter: w, gzip: gz}
		h.ServeHTTP(gzw, req)
	})
})

type gzipResponseWriter struct {
	http.ResponseWriter
	gzip *gzip.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.gzip.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	w.gzip.Flush()
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
