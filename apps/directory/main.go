package main

import (
	"net/http"

	"github.com/a-h/templ"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	dir, err := LoadTable()
	if err != nil {
		return err
	}

	mux := serve.NewMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, req *http.Request) {
		h := templ.Handler(IndexTempl(dir))
		w.Header().Set("Content-type", "charset=utf-8")
		h.ServeHTTP(w, req)
	})
	ctx := sigctx.New()
	return tailnet.ListenAndServe(ctx, gzip.Middleware(mux))
}
