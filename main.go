package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"monks.co/movietagger/db"
	"monks.co/movietagger/moviecopier"
	"monks.co/movietagger/movietagger"
	"monks.co/movietagger/tmdb"
)

func main() {
	tmdb := tmdb.New("88f973483e2dc73cfb5053bc059ae33b")
	db := db.New("/mypool/tank/movies/.movies.db")

	fmt.Printf("migrating...")
	if err := db.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf(" ok\n")

	mt := movietagger.New(tmdb, db)
	mc := moviecopier.New(db)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			if err := mt.Run(ctx); err != nil {
				fmt.Println(err)
				cancel()
			}

			time.Sleep(1 * time.Minute)
		}
	}()

	go func() {
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

	go func() {
		w, err := fsnotify.NewWatcher()
		if err != nil {
			fmt.Println("error watching for kill file", err)
			cancel()
		}

		if err := w.Add("."); err != nil {
			fmt.Println("error watching for kill file", err)
			cancel()
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

	<-ctx.Done()
}
