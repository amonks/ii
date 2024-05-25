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
	"monks.co/apps/posts/model"
	"monks.co/pkg/gzip"
	"monks.co/pkg/letterboxd"
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

	posts, err := model.LoadPosts("../posts/posts")
	if err != nil {
		return err
	}

	watchlog := periodically(time.Hour, lastFiveWatches)
	defer watchlog.stop()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		v, err := watchlog.get()
		if err != nil {
			serve.InternalServerError(w, req, err)
			return
		}
		h := templ.Handler(Homepage(&PageData{
			Posts:   posts,
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

func lastFiveWatches() ([]*letterboxd.Watch, error) {
	var watches []*letterboxd.Watch
	if err := letterboxd.FetchDiary("amonks", 1, 1, func(entry *letterboxd.Watch) error {
		watches = append(watches, entry)
		return nil
	}); err != nil {
		return nil, err
	}
	return watches, nil
}

type periodic[T any] struct {
	stopped bool
	mu      sync.Mutex

	v   T
	err error
}

var errUnset = fmt.Errorf("unset")

func periodically[T any](dur time.Duration, f func() (T, error)) *periodic[T] {
	val, err := f()
	p := &periodic[T]{
		err: err,
		v:   val,
	}
	go func() {
		for {
			time.Sleep(dur)

			log.Println("reload")
			if p.isStopped() {
				return
			}
			val, err := f()
			p.set(val, err)
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
