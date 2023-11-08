package main

import (
	"flag"
	"fmt"
	"net/http"

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

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	mux := http.NewServeMux()
	http.ListenAndServe(addr, gzip.Middleware(mux))

	return nil
}
