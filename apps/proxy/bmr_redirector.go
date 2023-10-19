package main

import (
	"net/http"
	"strings"
)

func BMRRedirectorHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.Host, "belgianman.com") || strings.Contains(req.Host, "blgn.mn") {
			http.Redirect(w, req, "https://belgianman.bandcamp.com", 301)
			return
		}
		h.ServeHTTP(w, req)
	})
}
