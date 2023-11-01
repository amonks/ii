package main

import (
	"flag"
	"fmt"
	"net/http"

	"monks.co/pkg/gzip"
	"monks.co/pkg/twilio"
	"monks.co/pkg/util"
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

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		msg := req.URL.Query().Get("message")
		if err := twilio.SMSMe(msg); err != nil {
			util.HTTPError("sms", w, req, http.StatusInternalServerError, "%s", err)
		}
	}))

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	http.ListenAndServe(addr, gzip.Handler(mux))

	return nil
}
