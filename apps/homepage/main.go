package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/a-h/templ"
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

	mux := http.NewServeMux()
	mux.Handle("/", templ.Handler(Homepage()))

	ctx := sigctx.New()
	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}
