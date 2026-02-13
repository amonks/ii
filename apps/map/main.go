package main

import (
	"errors"
	"fmt"

	"monks.co/apps/map/model"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
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
	db, err := model.NewModel()
	if err != nil {
		return fmt.Errorf("constructing model: %w", err)
	}

	ctx := sigctx.New()
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}
	var errs error

	s := NewServer(db)
	if err := tailnet.ListenAndServe(ctx, gzip.Middleware(s)); err != nil {
		errs = errors.Join(errs, err)
	}

	if err := db.Close(); err != nil {
		errs = errors.Join(errs, err)
	}

	return errs
}
