package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"monks.co/pkg/gzip"
	"monks.co/pkg/serve"
)

var (
	//go:embed templates/graph.gohtml
	tmplSrc string
)

type Data struct {
	Dust        []Aggregate
	Temperature []Aggregate
	Humidity    []Aggregate
	WaterLevel  []Aggregate
}

func (d *Data) JSON() (template.JS, error) {
	bs, err := json.Marshal(d)
	if err != nil {
		return "", err
	}

	return template.JS("window.data = " + string(bs) + ";"), nil
}

func serveAir(ctx context.Context, db *DB, addr string) error {
	tmpl := template.New("movies")
	tmpl, err := tmpl.Parse(tmplSrc)
	if err != nil {
		return err
	}

	mux := serve.NewMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		days := int64(3)
		if q := req.URL.Query().Get("days"); q != "" {
			if i, err := strconv.ParseInt(q, 10, 64); err == nil {
				days = i
			}
		}

		var errs error

		dust, err := db.Aggregate(AggregateIDDust, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}

		temperature, err := db.Aggregate(AggregateIDTemperature, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}

		humidity, err := db.Aggregate(AggregateIDHumidity, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}

		waterLevel, err := db.Aggregate(AggregateIDWaterLevel, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}

		if errs != nil {
			log.Println(errs)
			w.WriteHeader(500)
			w.Write([]byte(errs.Error()))
			return
		}

		if err := tmpl.Execute(w, &Data{
			Dust:        dust,
			Temperature: temperature,
			Humidity:    humidity,
			WaterLevel:  waterLevel,
		}); err != nil {
			log.Println(err)
		}
	})

	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}
