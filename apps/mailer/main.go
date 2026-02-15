package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"monks.co/pkg/email"
	"monks.co/pkg/gzip"
	"monks.co/pkg/meta"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
)

func main() {
	if err := run(); err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.Error("fatal", "error", err.Error(), "app.name", meta.AppName())
		}
		reqlog.Shutdown()
		os.Exit(1)
	}
}

func run() error {
	reqlog.SetupLogging()

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
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}
	return tailnet.ListenAndServe(ctx, reqlog.Middleware().ModifyHandler(gzip.Middleware(mux)))
}
