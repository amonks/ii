package main

import (
	"fmt"
	"net/http"

	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
	"monks.co/pkg/twilio"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	reqlog.SetupLogging()

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

	ctx := sigctx.New()
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}
	return tailnet.ListenAndServe(ctx, reqlog.Middleware().ModifyHandler(gzip.Middleware(mux)))
}
