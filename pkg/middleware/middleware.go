package middleware

import "net/http"

type Middleware interface {
	ModifyHandler(http.Handler) http.Handler
}

type MiddlewareFunc func(http.Handler) http.Handler

func (f MiddlewareFunc) ModifyHandler(h http.Handler) http.Handler {
	return f(h)
}

func Combine(middlewares ...Middleware) Middleware {
	return MiddlewareFunc(func(h http.Handler) http.Handler {
		for _, mw := range middlewares {
			h = mw.ModifyHandler(h)
		}
		return h
	})
}
