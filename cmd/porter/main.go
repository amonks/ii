package main

import (
	"flag"
	"fmt"
	"net/http"

	"monks.co/pkg/errlogger"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

var addr = flag.String("addr", "0.0.0.0:5000", "addr to bind")

func run() error {
	flag.Parse()
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Println("got one")
	})
	return http.ListenAndServe(*addr, nil)
}
