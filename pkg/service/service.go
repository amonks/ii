package service

import (
	"context"
	"net/http"
)

type Service interface {
	Start(ctx context.Context) error
	AddMiddleware(func(http.Handler) http.Handler)
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	Stop() error
}

