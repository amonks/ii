package main

import (
	"errors"
	"flag"
	"fmt"
	"log"

	"monks.co/pkg/errlogger"
	"monks.co/pkg/ports"
	"monks.co/pkg/sigctx"
)

var (
	mode = flag.String("mode", "fetch", "mode: fetch, serve, migrate")
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
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
		log.Printf("run fetch")
		if err := fetch(db); err != nil {
			errs = errors.Join(errs, err)
		}

	case "aggregates":
		log.Printf("run aggregates")
		if err := db.calculateAggregates(); err != nil {
			errs = errors.Join(errs, err)
		}

	// Migration has been completed, code removed

	case "serve":
		log.Printf("run serve")
		ctx := sigctx.New()

		addr := fmt.Sprintf("127.0.0.1:%d", port)
		if err := serveAir(ctx, db, addr); err != nil {
			errs = errors.Join(errs, err)
		}

	default:
		errs = errors.Join(errs, fmt.Errorf("command %s not supported", *mode))

	}

	if err := db.Close(); err != nil {
		errs = errors.Join(errs, err)
	}

	return errs
}
