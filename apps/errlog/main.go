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
	"monks.co/pkg/ports"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	port := ports.Apps["errlog"]

	db, err := model.New()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
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
		if machine := req.URL.Query().Get("machine"); machine != "" {
			where.Report.Machine = machine
		}
		if app := req.URL.Query().Get("app"); app != "" {
			where.Report.App = app
		}
		if status_code := req.URL.Query().Get("status_code"); status_code != "" {
			if status_code, err := strconv.ParseInt(status_code, 10, 64); err == nil {
				where.Report.StatusCode = int(status_code)
			}
		}

		cmds, err := db.LastN(1000, where)
		if err != nil {
			serve.Error(w, req, 500, err)
			return
		}
		templ.Handler(index(cmds)).ServeHTTP(w, req)
	})

	ctx := sigctx.New()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}
