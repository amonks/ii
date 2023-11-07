package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"monks.co/movietagger/config"
	"monks.co/movietagger/creditsfetcher"
	"monks.co/movietagger/db"
	"monks.co/movietagger/libraryserver"
	"monks.co/movietagger/loggingwaitgroup"
	"monks.co/movietagger/moviecopier"
	"monks.co/movietagger/movieimporter"
	"monks.co/movietagger/moviemetadatafetcher"
	"monks.co/movietagger/posterfetcher"
	"monks.co/movietagger/tmdb"
)

func main() {
	if err := run(); err != nil {
		log.Printf("stopped: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	// how to get a token from an api key:
	// http://dev.travisbell.com/play/v4_auth.html
	tmdb := tmdb.New(
		"88f973483e2dc73cfb5053bc059ae33b",
		"eyJhbGciOiJIUzI1NiJ9.eyJhdWQiOiI4OGY5NzM0ODNlMmRjNzNjZmI1MDUzYmMwNTlhZTMzYiIsInN1YiI6IjYzZjQ1ZWVkY2FhY2EyMDBhMTljZmQ5OCIsInNjb3BlcyI6WyJhcGlfcmVhZCJdLCJ2ZXJzaW9uIjoxfQ.BkWBY-B7s9Tr6PObyAukp9mC3nerpHeOZcCX9t4BTRE",
	)
	db := db.New(config.DBPath)

	if err := db.Start(); err != nil {
		return err
	}

	if movies, err := db.AllMovies(); err != nil {
		return err
	} else {
		log.Printf("%d movies in the library.\n", len(movies))
	}

	wg := &loggingwaitgroup.WaitGroup{}
	ctx, cancel := context.WithCancelCause(context.Background())

	// Run library server
	ls := libraryserver.New(tmdb, db)
	wg.Add("libraryserver")
	go func() {
		defer wg.Done("libraryserver")
		if err := ls.Run(ctx); err != nil {
			cancel(fmt.Errorf("libraryserver error: %w", err))
			return
		}
	}()

	// Launch movieimporter, rerunning every minute.
	mt := movieimporter.New(tmdb, db)
	wg.Add("movieimporter")
	go func() {
		defer wg.Done("movieimporter")
		for {
			if err := mt.Run(ctx); err != nil {
				cancel(fmt.Errorf("movieimporter error: %w", err))
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Minute):
			}
		}
	}()

	runAfterImport := func(name string, run func(ctx context.Context) error) {
		wg.Add(name)
		go func() {
			defer wg.Done(name)
		run:
			if err := run(ctx); err != nil {
				cancel(fmt.Errorf("%s error: %w", name, err))
				return
			}

			for {
				select {
				case <-ctx.Done():
					return
				case <-db.Subscribe():
					goto run
				}
			}
		}()
	}

	// For each post-import task, launch it, then wait. If the context is
	// canceled, exit. If movieimporter updates, rerun it, waiting again
	// when it exits.
	cf := creditsfetcher.New(tmdb, db)
	mc := moviecopier.New(db)
	mf := moviemetadatafetcher.New(tmdb, db)
	pf := posterfetcher.New(tmdb, db)
	// ms := moviesyncer.New(tmdb, db)
	runAfterImport("creditsfetcher", cf.Run)
	runAfterImport("moviecopier", mc.Run)
	runAfterImport("moviemetadatafetcher", mf.Run)
	runAfterImport("posterfetcher", pf.Run)
	// runAfterImport("moviesyncer", ms.Run)

	// Handle signals. If we get one, kill the program.
	wg.Add("signalhandler")
	go func() {
		defer wg.Done("signalhandler")

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

		select {
		case <-ctx.Done():
			return
		case sig := <-sigs:
			log.Println("interrupt")
			cancel(fmt.Errorf("interrupt signal: %s", sig))
			return
		}
	}()

	wg.Wait()

	errs := context.Cause(ctx)
	if err := db.Stop(); err != nil {
		errs = errors.Join(errs, fmt.Errorf("failed to close db: %w", err))
	}
	return errs
}
