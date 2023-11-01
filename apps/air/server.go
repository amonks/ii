package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"gorm.io/gorm"
	"monks.co/pkg/gzip"
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

func serve(db *gorm.DB, addr string) error {
	tmpl := template.New("movies")
	tmpl, err := tmpl.Parse(tmplSrc)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		cond := "created_at > date('now', '-3 day')"
		fmt.Println(req.URL.String())
		if q := req.URL.Query().Get("days"); q != "" {
			fmt.Println("q", q)
			if i, err := strconv.ParseInt(q, 10, 64); err == nil {
				fmt.Println("i", i)
				cond = fmt.Sprintf("created_at > date('now', '-%d day')", i)
			}
		}
		fmt.Println(cond)

		var ps []Parameters
		if tx := db.Find(&ps, cond); tx.Error != nil {
			w.WriteHeader(500)
			return
		} else {
			fmt.Println("rows", tx.RowsAffected)
		}

		if err := tmpl.Execute(w, &Data{ps}); err != nil {
			fmt.Println(err)
		}
	})

	s := &http.Server{Addr: addr, Handler: gzip.Handler(mux)}

	fmt.Println("listening at", addr)
	if err := s.ListenAndServe(); err != nil {
		return err
	}

	return nil
}
