package main

import (
	"monks.co/pkg/middleware"

	"net/http"
)

var _ middleware.Middleware = RedirectorMiddleware{}

type RedirectorMiddleware map[string]string

func (redirects RedirectorMiddleware) ModifyHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		for host, target := range redirects {
			if req.Host == host || req.Host == "www."+host {
				http.Redirect(w, req, target, 301)
				return
			}
		}
		h.ServeHTTP(w, req)
	})
}
