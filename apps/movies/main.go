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

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/creditsfetcher"
	"monks.co/apps/movies/db"
	"monks.co/apps/movies/letterboxdimporter"
	"monks.co/apps/movies/libraryserver"
	"monks.co/apps/movies/moviecopier"
	"monks.co/apps/movies/movieimporter"
	"monks.co/apps/movies/moviemetadatafetcher"
	"monks.co/apps/movies/posterfetcher"
	"monks.co/apps/movies/ratingfetcher"
	"monks.co/apps/movies/stubquerygenerator"
	"monks.co/apps/movies/tvcopier"
	"monks.co/apps/movies/tvimporter"
	"monks.co/apps/movies/tvmetadatafetcher"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/llm"
	"monks.co/pkg/loggingwaitgroup"
	"monks.co/pkg/tmdb"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)

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
		if stopErr := db.Stop(); stopErr != nil {
			err = errors.Join(err, stopErr)
		}
		return err
	} else {
		log.Printf("%d movies in the library.\n", len(movies))
	}

	wg := &loggingwaitgroup.WaitGroup{}
	ctx, cancel := context.WithCancelCause(context.Background())

	runAfterImport := func(name string, run func(ctx context.Context) error) {
		wg.Add(name)
		go func() {
			defer wg.Done(name)
		run:
			if err := run(ctx); err != nil {
				err := fmt.Errorf("%s error: %w", name, err)
				cancel(err)
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

	runAfterTVImport := func(name string, run func(ctx context.Context) error) {
		wg.Add(name)
		go func() {
			defer wg.Done(name)
		run:
			if err := run(ctx); err != nil {
				err := fmt.Errorf("%s error: %w", name, err)
				cancel(err)
				return
			}

			for {
				select {
				case <-ctx.Done():
					return
				case <-db.SubscribeTV():
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
	mmf := moviemetadatafetcher.New(tmdb, db)
	pf := posterfetcher.New(tmdb, db)
	rf := ratingfetcher.New(db)
	// ms := moviesyncer.New(tmdb, db)
	runAfterImport("creditsfetcher", cf.Run)
	runAfterImport("moviecopier", mc.Run)
	runAfterImport("moviemetadatafetcher", mmf.Run)
	runAfterImport("posterfetcher", pf.Run)
	runAfterImport("ratingfetcher", rf.Run)
	// runAfterImport("moviesyncer", ms.Run)

	// Run library server
	ls := libraryserver.New(tmdb, db)
	wg.Add("libraryserver")
	go func() {
		defer wg.Done("libraryserver")
		err := ls.Run(ctx)
		if err != nil {
			cancel(fmt.Errorf("libraryserver error: %w", err))
		} else {
			cancel(fmt.Errorf("libraryserver stopped"))
		}
	}()

	// Launch mcimporter, running every minute.
	li := letterboxdimporter.New(db)
	wg.Add("letterboxdimporter")
	go func() {
		defer wg.Done("letterboxdimporter")
		for {
			if err := li.Run(); err != nil {
				cancel(fmt.Errorf("letterboxdimporter error: %w", err))
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Minute):
			}
		}
	}()

	// Launch movieimporter, rerunning every minute.
	mi := movieimporter.New(tmdb, db)
	wg.Add("movieimporter")
	go func() {
		defer wg.Done("movieimporter")
		for {
			if err := mi.Run(ctx); err != nil {
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

	// Launch tvimporter, rerunning every minute.
	ti := tvimporter.New(tmdb, db)
	wg.Add("tvimporter")
	go func() {
		defer wg.Done("tvimporter")
		for {
			if err := ti.Run(ctx); err != nil {
				cancel(fmt.Errorf("tvimporter error: %w", err))
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Minute):
			}
		}
	}()

	// Add TV copier and metadata fetcher
	tc := tvcopier.New(db)
	tmf := tvmetadatafetcher.New(tmdb, db)
	runAfterTVImport("tvcopier", tc.Run)
	runAfterTVImport("tvmetadatafetcher", tmf.Run)

	// Initialize the LLM client
	llmClient := llm.New("4o-mini")

	// Add stub query generator for movies
	sqg := stubquerygenerator.New(llmClient, tmdb, db)
	runAfterImport("stubquerygenerator_movies", sqg.RunMovies)

	// Add stub query generator for TV shows
	runAfterTVImport("stubquerygenerator_tv", sqg.RunTV)

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
