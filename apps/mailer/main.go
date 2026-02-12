package main

import (
	"net/http"

	"monks.co/pkg/email"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	mux := serve.NewMux()
	mux.Handle("POST /{$}", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.ParseForm()
		subject := req.FormValue("subject")
		if subject == "" {
			serve.Errorf(w, req, http.StatusBadRequest, "'subject' is required")
			return
		}
		body := req.FormValue("body")
		if body == "" {
			serve.Errorf(w, req, http.StatusBadRequest, "'body' is required")
			return
		}

		if err := email.EmailMe(email.Message{
			Subject: subject,
			Body:    body,
		}); err != nil {
			serve.Errorf(w, req, http.StatusInternalServerError, "%s", err)
			return
		}
	}))

	ctx := sigctx.New()
	return tailnet.ListenAndServe(ctx, gzip.Middleware(mux))
}
