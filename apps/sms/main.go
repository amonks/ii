package main

import (
	"fmt"
	"net/http"

	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/ports"
	"monks.co/pkg/serve"
	"monks.co/pkg/twilio"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	port := ports.Apps["sms"]

	mux := serve.NewMux()
	mux.Handle("POST /{$}", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		msg := req.URL.Query().Get("message")
		if msg == "" {
			serve.Errorf(w, req, http.StatusBadRequest, "'message' is required")
			return
		}
		if err := twilio.SMSMe(msg); err != nil {
			serve.Errorf(w, req, http.StatusInternalServerError, "%s", err)
			return
		}
	}))

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	http.ListenAndServe(addr, gzip.Middleware(mux))

	return nil
}
