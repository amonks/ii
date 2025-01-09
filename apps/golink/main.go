package main

import (
	"errors"
	"fmt"

	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/ports"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	port := ports.Apps["golink"]

	db, err := NewModel()
	if err != nil {
		return fmt.Errorf("constructing model: %w", err)
	}

	ctx := sigctx.New()
	var errs error

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	s := NewServer(db)
	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(s)); err != nil {
		errs = errors.Join(errs, err)
	}

	if err := db.Close(); err != nil {
		errs = errors.Join(errs, err)
	}

	return errs
}
