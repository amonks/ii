package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"monks.co/apps/errlog/model"
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
		var report model.ErrorReport
		dec := json.NewDecoder(req.Body)
		dec.DisallowUnknownFields()
		defer req.Body.Close()
		if err := dec.Decode(&report); err != nil {
			serve.Error(w, req, 400, err)
			return
		}

		if err := db.Capture(&report); err != nil {
			serve.Error(w, req, 500, err)
			return
		}

		w.Write([]byte("ok"))
	})

	ctx := sigctx.New()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}
