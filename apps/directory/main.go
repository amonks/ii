package main

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/ports"
	"monks.co/pkg/serve"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	port := ports.Apps["directory"]

	dir, err := LoadTable()
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	mux := serve.NewMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		h := templ.Handler(IndexTempl(dir))
		w.Header().Set("Content-type", "charset=utf-8")
		h.ServeHTTP(w, req)
	})
	http.ListenAndServe(addr, gzip.Middleware(mux))

	return nil
}
