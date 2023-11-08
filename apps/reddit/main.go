package main

import (
	"errors"
	"flag"
	"fmt"

	"monks.co/pkg/gzip"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
)

var port = flag.Int("port", 3001, "port")

const (
	archivePath = "/data/tank/mirror/reddit/"
	dbPath      = "/data/tank/mirror/reddit/.reddit.db"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	flag.Parse()

	db, err := NewModel()
	if err != nil {
		return fmt.Errorf("constructing model: %w", err)
	}

	ctx := sigctx.New()
	var errs error

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	s := newServer(db)
	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(s)); err != nil {
		errs = errors.Join(errs, err)
	}

	if err := db.Close(); err != nil {
		errs = errors.Join(errs, err)
	}

	return errs
}
