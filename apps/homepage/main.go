package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/a-h/templ"
	"monks.co/pkg/gzip"
	"monks.co/pkg/letterboxd"
	"monks.co/pkg/meta"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/rotate"
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
	ctx := sigctx.New()
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}

	mux := serve.NewMux()
	mux.HandleFunc("GET /error/{$}", func(w http.ResponseWriter, req *http.Request) {
		code := 500
		if qCode := req.URL.Query().Get("code"); qCode != "" {
			if qCode, err := strconv.ParseInt(qCode, 10, 64); err == nil {
				code = int(qCode)
			}
		}
		serve.Errorf(w, req, code, "%d, as requested", code)
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, req *http.Request) {
		diary, err := letterboxd.FetchDiary()
		if err != nil {
			reqlog.Set(req.Context(), "err.message", err.Error())
			h := templ.Handler(Homepage(&PageData{
				Watches: nil,
			}))
			h.ServeHTTP(w, req)
			return
		}
		h := templ.Handler(Homepage(&PageData{
			Watches:   diary,
			GoSynonym: goSynonyms.Next(),
		}))
		h.ServeHTTP(w, req)
	})

	if err := tailnet.ListenAndServe(ctx, reqlog.Middleware().ModifyHandler(gzip.Middleware(mux))); err != nil {
		return err
	}

	return nil
}

var goSynonyms = rotate.New(
	"Go",
	"囲碁",
	"いご",
	"바둑",
	"baduk",
	"圍棋",
	"wéiqí",
)
