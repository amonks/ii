package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/a-h/templ"
	"monks.co/apps/movies/db"
	"monks.co/pkg/gzip"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
)

var (
	port = flag.Int("port", 3000, "port")
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	flag.Parse()

	dbPath := filepath.Join(os.Getenv("MONKS_DATA"), "movies.db")
	db := db.New(dbPath)
	if err := db.Start(); err != nil {
		return err
	}
	defer db.Stop()

	watches, err := db.AllWatches()
	if err != nil {
		return err
	}

	data := &PageData{
		Watches: watches,
	}

	mux := http.NewServeMux()
	mux.Handle("/", templ.Handler(Homepage(data)))

	ctx := sigctx.New()
	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}
