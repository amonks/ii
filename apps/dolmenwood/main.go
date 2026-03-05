package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"monks.co/apps/dolmenwood/db"
	"monks.co/apps/dolmenwood/server"
	"monks.co/pkg/gzip"
	"monks.co/pkg/meta"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
)

func main() {
	if err := run(); err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.Error("fatal", "error", err.Error(), "app.name", meta.AppName())
		}
		reqlog.Shutdown()
		os.Exit(1)
	}
}

func run() error {
	reqlog.SetupLogging()

	ctx := sigctx.New()
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}

	d, err := db.New()
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}

	srv := server.New(d)
	appMux := srv.Mux()

	// Handle both direct tailnet access (/) and proxy access (/dolmenwood/)
	mux := http.NewServeMux()
	mux.Handle("/dolmenwood/", http.StripPrefix("/dolmenwood", appMux))
	mux.Handle("/", appMux)
	if err := tailnet.ListenAndServe(ctx, reqlog.Middleware().ModifyHandler(gzip.Middleware(mux))); err != nil {
		return err
	}

	return nil
}
