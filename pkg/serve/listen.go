package serve

import (
	"context"
	"errors"
	"log"
	"net/http"
)

func ListenAndServe(ctx context.Context, addr string, handler http.Handler) error {
	srv := http.Server{Addr: addr, Handler: handler}
	errs := make(chan error)
	go func() {
		errs <- srv.ListenAndServe()
	}()
	log.Printf("listening at %s", addr)
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		cause := context.Cause(ctx)
		shutdownErr := srv.Shutdown(context.Background())
		return errors.Join(cause, shutdownErr)
	}
}
