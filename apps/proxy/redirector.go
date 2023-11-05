package main

import (
	"net/http"
	"strings"
)

func RedirectorHandler(redirects map[string]string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		for host, target := range redirects {
			if strings.Contains(req.Host, host) {
				http.Redirect(w, req, target, 301)
				return
			}
		}
		h.ServeHTTP(w, req)
	})
}
