package main

import (
	"errors"
	"flag"
	"fmt"

	"monks.co/pkg/gzip"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/traffic"
)

var port = flag.Int("port", 3001, "port")

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	flag.Parse()

	db, err := traffic.Open()
	if err != nil {
		panic(err)
	}

	ctx := sigctx.New()
	var errs error

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	s := NewServer(db)
	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(s)); err != nil {
		errs = errors.Join(errs, err)
	}

	if err := db.Close(); err != nil {
		errs = errors.Join(errs, err)
	}

	return errs
}
