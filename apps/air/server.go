package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"monks.co/pkg/database"
	"monks.co/pkg/gzip"
	"monks.co/pkg/serve"
)

var (
	//go:embed templates/graph.gohtml
	tmplSrc string
)

type Data struct {
	parameters []Parameters
}

func (d *Data) JSON() (template.JS, error) {
	bs, err := json.Marshal(d.parameters)
	if err != nil {
		return "", err
	}

	return template.JS("window.data = " + string(bs) + ";"), nil
}

func serveAir(ctx context.Context, db *database.DB, addr string) error {
	tmpl := template.New("movies")
	tmpl, err := tmpl.Parse(tmplSrc)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		cond := "created_at > date('now', '-3 day')"
		if q := req.URL.Query().Get("days"); q != "" {
			if i, err := strconv.ParseInt(q, 10, 64); err == nil {
				cond = fmt.Sprintf("created_at > date('now', '-%d day')", i)
			}
		}

		var ps []Parameters
		if tx := db.Find(&ps, cond); tx.Error != nil {
			w.WriteHeader(500)
			return
		}

		if err := tmpl.Execute(w, &Data{ps}); err != nil {
			log.Println(err)
		}
	})

	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}
