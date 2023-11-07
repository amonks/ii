package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/a-h/templ"
	"monks.co/apps/youtube/model"
	"monks.co/pkg/gzip"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
)

var port = flag.Int("port", 3000, "port")

func main() {
	if err := run(); err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()

	history, err := model.LoadHistory("histories")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		h := templ.Handler(Index(history))
		w.Header().Set("Content-type", "charset=utf-8")
		h.ServeHTTP(w, req)
		return
	})

	ctx := sigctx.New()

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}
