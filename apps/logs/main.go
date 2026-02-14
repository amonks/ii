package main

import (
	"errors"
	"fmt"
	"net/http"

	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/logs"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	reqlog.SetupLogging()
	db, err := logs.Open()
	if err != nil {
		panic(err)
	}

	ctx := sigctx.New()
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}
	var errs error

	s := NewServer(db)
	// Mount ingest without reqlog middleware to avoid a log shipping loop.
	mux := http.NewServeMux()
	mux.Handle("POST /ingest", s.IngestHandler())
	mux.Handle("/", reqlog.Middleware().ModifyHandler(gzip.Middleware(s)))
	if err := tailnet.ListenAndServe(ctx, mux); err != nil {
		errs = errors.Join(errs, err)
	}

	if err := db.Close(); err != nil {
		errs = errors.Join(errs, err)
	}

	return errs
}
