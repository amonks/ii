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
	// Venta data
	Dust        []Aggregate
	Temperature []Aggregate
	Humidity    []Aggregate
	WaterLevel  []Aggregate
	
	// Office Aranet data
	OfficeTemperature []Aggregate
	OfficeHumidity    []Aggregate
	OfficeCO2         []Aggregate
	OfficePressure    []Aggregate
	
	// Living Room Aranet data
	LivingRoomTemperature []Aggregate
	LivingRoomHumidity    []Aggregate
	LivingRoomCO2         []Aggregate
	LivingRoomPressure    []Aggregate
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
	mux.HandleFunc("GET /", func(w http.ResponseWriter, req *http.Request) {
		days := int64(3)
		if q := req.URL.Query().Get("days"); q != "" {
			if i, err := strconv.ParseInt(q, 10, 64); err == nil {
				days = i
			}
		}

		var errs error
		data := &Data{}

		// Fetch Venta data
		dust, err := db.Aggregate(AggregateIDDust, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.Dust = dust

		temperature, err := db.Aggregate(AggregateIDTemperature, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.Temperature = temperature

		humidity, err := db.Aggregate(AggregateIDHumidity, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.Humidity = humidity

		waterLevel, err := db.Aggregate(AggregateIDWaterLevel, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.WaterLevel = waterLevel

		// Fetch Office Aranet data
		officeTemp, err := db.Aggregate(AggregateIDOfficeTemperature, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.OfficeTemperature = officeTemp

		officeHumidity, err := db.Aggregate(AggregateIDOfficeHumidity, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.OfficeHumidity = officeHumidity

		officeCO2, err := db.Aggregate(AggregateIDOfficeCO2, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.OfficeCO2 = officeCO2

		officePressure, err := db.Aggregate(AggregateIDOfficePressure, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.OfficePressure = officePressure

		// Fetch Living Room Aranet data
		livingRoomTemp, err := db.Aggregate(AggregateIDLivingRoomTemperature, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.LivingRoomTemperature = livingRoomTemp

		livingRoomHumidity, err := db.Aggregate(AggregateIDLivingRoomHumidity, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.LivingRoomHumidity = livingRoomHumidity

		livingRoomCO2, err := db.Aggregate(AggregateIDLivingRoomCO2, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.LivingRoomCO2 = livingRoomCO2

		livingRoomPressure, err := db.Aggregate(AggregateIDLivingRoomPressure, days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.LivingRoomPressure = livingRoomPressure

		// Handle errors
		if errs != nil {
			log.Println(errs)
			w.WriteHeader(500)
			w.Write([]byte(errs.Error()))
			return
		}

		// Execute template with data
		if err := tmpl.Execute(w, data); err != nil {
			log.Println(err)
		}
	})

	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}
