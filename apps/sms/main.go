package main

import (
	"fmt"
	"net/http"

	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/ports"
	"monks.co/pkg/twilio"
	"monks.co/pkg/util"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	port := ports.Apps["sms"]

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		msg := req.URL.Query().Get("message")
		if msg == "" {
			util.HTTPError("sms", w, req, http.StatusBadRequest, "'message' is required")
			return
		}
		if err := twilio.SMSMe(msg); err != nil {
			util.HTTPError("sms", w, req, http.StatusInternalServerError, "%s", err)
			return
		}
	}))

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	http.ListenAndServe(addr, gzip.Middleware(mux))

	return nil
}
