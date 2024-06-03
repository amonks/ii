package main

import (
	"errors"
	"flag"
	"fmt"
	"log"

	"monks.co/pkg/ports"
	"monks.co/pkg/sigctx"
)

var (
	mode = flag.String("mode", "fetch", "mode: fetch, serve")
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
	log.Println("done")
}

func run() error {
	flag.Parse()

	port := ports.Apps["air"]

	db, err := NewDB()
	if err != nil {
		return err
	}

	var errs error
	switch *mode {
	case "fetch":
		if err := fetch(db); err != nil {
			errs = errors.Join(errs, err)
		}

	case "serve":
		ctx := sigctx.New()

		addr := fmt.Sprintf("127.0.0.1:%d", port)
		if err := serveAir(ctx, db, addr); err != nil {
			errs = errors.Join(errs, err)
		}

	}

	if err := db.Close(); err != nil {
		errs = errors.Join(errs, err)
	}

	return errs
}
