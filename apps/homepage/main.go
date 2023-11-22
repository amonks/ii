package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

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

	fetcher := periodically(db.AllWatches)
	defer fetcher.stop()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		v, err := fetcher.get()
		if err != nil {
			serve.InternalServerError(w, req, err)
			return
		}
		h := templ.Handler(Homepage(&PageData{
			Watches: v,
		}))
		h.ServeHTTP(w, req)
	})

	ctx := sigctx.New()
	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}

type periodic[T any] struct {
	stopped bool
	mu      sync.Mutex

	v   T
	err error
}

var errUnset = fmt.Errorf("unset")

func periodically[T any](f func() (T, error)) *periodic[T] {
	p := &periodic[T]{
		err: errUnset,
	}
	go func() {
		for {
			log.Println("reload")
			if p.isStopped() {
				return
			}
			val, err := f()
			p.set(val, err)

			time.Sleep(30 * time.Second)
		}
	}()
	return p
}

func (p *periodic[T]) isStopped() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stopped
}

func (p *periodic[T]) stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopped = true
}

func (p *periodic[T]) get() (T, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.v, p.err
}

func (p *periodic[T]) set(v T, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.v, p.err = v, err
}
