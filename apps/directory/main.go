package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"monks.co/pkg/gzip"
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

	dir, err := LoadTable()
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		h := templ.Handler(IndexTempl(dir))
		w.Header().Set("Content-type", "charset=utf-8")
		h.ServeHTTP(w, req)
	})
	http.ListenAndServe(addr, gzip.Middleware(mux))

	return nil
}
