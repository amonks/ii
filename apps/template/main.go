package main

import (
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/serve"
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
	mux := serve.NewMux()

	ctx := sigctx.New()
	if err := tailnet.ListenAndServe(ctx, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}
