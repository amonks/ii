package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"monks.co/movietagger/db"
	"monks.co/movietagger/libraryserver"
	"monks.co/movietagger/moviecopier"
	"monks.co/movietagger/moviefetcher"
	"monks.co/movietagger/movietagger"
	"monks.co/movietagger/posterfetcher"
	"monks.co/movietagger/tmdb"
)

func main() {
	// how to get a token from an api key:
	// http://dev.travisbell.com/play/v4_auth.html
	tmdb := tmdb.New(
		"88f973483e2dc73cfb5053bc059ae33b",
		"eyJhbGciOiJIUzI1NiJ9.eyJhdWQiOiI4OGY5NzM0ODNlMmRjNzNjZmI1MDUzYmMwNTlhZTMzYiIsInN1YiI6IjYzZjQ1ZWVkY2FhY2EyMDBhMTljZmQ5OCIsInNjb3BlcyI6WyJhcGlfcmVhZCJdLCJ2ZXJzaW9uIjoxfQ.BkWBY-B7s9Tr6PObyAukp9mC3nerpHeOZcCX9t4BTRE",
	)
	db := db.New("/mypool/tank/movies/.movies.db")

	fmt.Printf("migrating...")
	if err := db.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf(" ok\n")

	// run movie fetcher
	mf := moviefetcher.New(tmdb, db)
	if err := mf.Run(context.Background()); err != nil {
		fmt.Println(err)
		return
	}

	// run poster fetcher
	pf := posterfetcher.New(tmdb, db)
	if err := pf.Run(context.Background()); err != nil {
		fmt.Println(err)
		return
	}

	// // run movie syncer
	// ms := moviesyncer.New(tmdb, db)
	// if err := ms.Run(context.Background()); err != nil {
	// 	fmt.Println(err)
	// 	return
	// }

	wg := &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())

	// Run library server
	ls := libraryserver.New(tmdb, db)
	wg.Add(1)
	go func() {
		if err := ls.Run(context.Background()); err != nil {
			fmt.Println(err)
			cancel()
			return
		}
	}()

	// Launch movietagger, rerunning every minute.
	mt := movietagger.New(tmdb, db)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if err := mt.Run(ctx); err != nil {
				fmt.Println(err)
				cancel()
			}

			time.Sleep(1 * time.Minute)
		}
	}()

	// Launch moviecopier, then wait. If the context is canceled, exit. If
	// movietagger updates, rerun moviecopier, waiting again when it exits.
	wg.Add(1)
	mc := moviecopier.New(db)
	go func() {
		defer wg.Done()
	run:
		if err := mc.Run(ctx); err != nil {
			fmt.Println(err)
			cancel()
		}

		for {
			select {
			case <-ctx.Done():
				break
			case <-mt.Subscribe():
				goto run
			}
		}
	}()

	// Watch the file ./cancel.stamp. If it has an fsevent, kill the program.
	wg.Add(1)
	go func() {
		defer wg.Done()
		w, err := fsnotify.NewWatcher()
		if err != nil {
			fmt.Println("error watching for kill file", err)
			cancel()
			return
		}

		if err := w.Add("."); err != nil {
			fmt.Println("error watching for kill file", err)
			cancel()
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-w.Events:
				if !ok {
					return
				}
				if !strings.HasSuffix(event.Name, "cancel.stamp") {
					continue
				}
				fmt.Println("got cancellation stamp; cancelling")
				cancel()
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				fmt.Println("file watcher failed; cancelling", err)
				cancel()
			}
		}
	}()

	wg.Wait()
}
