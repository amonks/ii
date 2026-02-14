package main

import (
	"errors"
	"fmt"

	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
	"monks.co/pkg/traffic"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	reqlog.SetupLogging()
	db, err := traffic.Open()
	if err != nil {
		panic(err)
	}

	ctx := sigctx.New()
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}
	var errs error

	s := NewServer(db)
	if err := tailnet.ListenAndServe(ctx, reqlog.Middleware().ModifyHandler(gzip.Middleware(s))); err != nil {
		errs = errors.Join(errs, err)
	}

	if err := db.Close(); err != nil {
		errs = errors.Join(errs, err)
	}

	return errs
}
