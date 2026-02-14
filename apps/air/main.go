package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"

	"monks.co/pkg/errlogger"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
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
	reqlog.SetupLogging()

	flag.Parse()

	db, err := NewDB()
	if err != nil {
		return err
	}

	var errs error
	switch *mode {
	case "fetch":
		log.Printf("run fetch")
		if err := tailnet.WaitReady(context.Background()); err != nil {
			return fmt.Errorf("tailnet: %w", err)
		}
		if err := fetch(db); err != nil {
			errs = errors.Join(errs, fmt.Errorf("fetch error: %w", err))
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
		if err := tailnet.WaitReady(ctx); err != nil {
			return fmt.Errorf("tailnet: %w", err)
		}

		if err := serveAir(ctx, db); err != nil {
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
