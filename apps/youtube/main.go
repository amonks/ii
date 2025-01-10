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
	"monks.co/pkg/ports"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		log.Println(err.Error())
		os.Exit(1)
	}
}

func run() error {
	port := ports.Apps["youtube"]

	history, err := model.LoadHistory("histories")
	if err != nil {
		return err
	}

	mux := serve.NewMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		h := templ.Handler(Index(history))
		w.Header().Set("Content-type", "charset=utf-8")
		h.ServeHTTP(w, req)
		return
	})

	ctx := sigctx.New()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}
