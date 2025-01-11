package gzip

import (
	"compress/gzip"
	"io"
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
		gzw := gzipResponseWriter{w, gz}
		h.ServeHTTP(gzw, req)
	})
})

type gzipResponseWriter struct {
	http.ResponseWriter
	gzip   io.Writer
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.gzip.Write(b)
}
