package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"monks.co/apps/errlog/model"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	reqlog.SetupLogging()

	db, err := model.New()
	if err != nil {
		return err
	}

	mux := serve.NewMux()
	mux.HandleFunc("POST /", func(w http.ResponseWriter, req *http.Request) {
		var report errlogger.ErrorReport
		dec := json.NewDecoder(req.Body)
		dec.DisallowUnknownFields()
		defer req.Body.Close()
		if err := dec.Decode(&report); err != nil {
			serve.Error(w, req, 400, err)
			return
		}

		if err := db.Capture(&model.ErrorReport{
			UUID:   uuid.NewString(),
			Report: report,
		}); err != nil {
			serve.Error(w, req, 500, err)
			return
		}

		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, req *http.Request) {
		var where model.ErrorReport

		q := req.URL.Query()
		if machine := q.Get("machine"); machine != "" {
			where.Report.Machine = machine
		}
		if app := q.Get("app"); app != "" {
			where.Report.App = app
		}
		if status_code := q.Get("status_code"); status_code != "" {
			if status_code, err := strconv.ParseInt(status_code, 10, 64); err == nil {
				where.Report.StatusCode = int(status_code)
			}
		}
		count := 1000
		if n := q.Get("n"); n != "" {
			if n, err := strconv.ParseInt(n, 10, 64); err == nil {
				count = int(n)
			}
		}

		cmds, err := db.LastN(count, where)
		if err != nil {
			serve.Error(w, req, 500, err)
			return
		}

		templ.Handler(index(cmds, where)).ServeHTTP(w, req)
	})

	ctx := sigctx.New()
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}
	if err := tailnet.ListenAndServe(ctx, reqlog.Middleware().ModifyHandler(gzip.Middleware(mux))); err != nil {
		return err
	}

	return nil
}
