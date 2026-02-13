package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/a-h/templ"
	"monks.co/apps/youtube/model"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		log.Println(err.Error())
		os.Exit(1)
	}
}

func run() error {
	history, err := model.LoadHistory("histories")
	if err != nil {
		return err
	}

	mux := serve.NewMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, req *http.Request) {
		h := templ.Handler(Index(history))
		w.Header().Set("Content-type", "charset=utf-8")
		h.ServeHTTP(w, req)
	})

	ctx := sigctx.New()
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}

	if err := tailnet.ListenAndServe(ctx, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}
