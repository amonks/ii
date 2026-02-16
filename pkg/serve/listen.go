package serve

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
)

func ListenAndServe(ctx context.Context, addr string, handler http.Handler) error {
	srv := http.Server{Addr: addr, Handler: handler}
	errs := make(chan error)
	go func() {
		errs <- srv.ListenAndServe()
	}()
	slog.Info("started", "addr", addr)
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		cause := context.Cause(ctx)
		shutdownErr := srv.Shutdown(context.Background())
		return errors.Join(cause, shutdownErr)
	}
}
