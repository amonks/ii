package main

import (
	"flag"
	"fmt"
	"net/http"

	"monks.co/pkg/gzip"
)

var (
	port int
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	flag.IntVar(&port, "port", 3000, "port")
	flag.Parse()

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	mux := http.NewServeMux()
	http.ListenAndServe(addr, gzip.Handler(mux))

	return nil
}
