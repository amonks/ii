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

// Data is the structure passed to the template for rendering
type Data struct {
	// For backward compatibility we keep the same structure
	// but populate it with data from the new model
	
	// Venta data
	Dust        []WindowAggregate
	Temperature []WindowAggregate
	Humidity    []WindowAggregate
	WaterLevel  []WindowAggregate
	
	// Office Aranet data
	OfficeTemperature []WindowAggregate
	OfficeHumidity    []WindowAggregate
	OfficeCO2         []WindowAggregate
	OfficePressure    []WindowAggregate
	
	// Living Room Aranet data
	LivingRoomTemperature []WindowAggregate
	LivingRoomHumidity    []WindowAggregate
	LivingRoomCO2         []WindowAggregate
	LivingRoomPressure    []WindowAggregate
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
		
		// Using the new data model to fetch aggregates by room/device/parameter
		
		// Venta data (living room)
		ventaDevice := "60:8A:10:B5:58:A0"
		ventaRoom := "living room"
		
		dust, err := db.GetAggregates(ventaRoom, ventaDevice, "dust", days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.Dust = dust

		temperature, err := db.GetAggregates(ventaRoom, ventaDevice, "temperature", days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.Temperature = temperature

		humidity, err := db.GetAggregates(ventaRoom, ventaDevice, "humidity", days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.Humidity = humidity

		waterLevel, err := db.GetAggregates(ventaRoom, ventaDevice, "water_level", days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.WaterLevel = waterLevel

		// Office Aranet data
		officeDevice := "Aranet4 0AC6E"
		officeRoom := "office"
		
		officeTemp, err := db.GetAggregates(officeRoom, officeDevice, "temperature", days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.OfficeTemperature = officeTemp

		officeHumidity, err := db.GetAggregates(officeRoom, officeDevice, "humidity", days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.OfficeHumidity = officeHumidity

		officeCO2, err := db.GetAggregates(officeRoom, officeDevice, "co2", days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.OfficeCO2 = officeCO2

		officePressure, err := db.GetAggregates(officeRoom, officeDevice, "pressure", days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.OfficePressure = officePressure

		// Living Room Aranet data
		livingRoomDevice := "Aranet4 069F9"
		livingRoomRoom := "living room"
		
		livingRoomTemp, err := db.GetAggregates(livingRoomRoom, livingRoomDevice, "temperature", days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.LivingRoomTemperature = livingRoomTemp

		livingRoomHumidity, err := db.GetAggregates(livingRoomRoom, livingRoomDevice, "humidity", days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.LivingRoomHumidity = livingRoomHumidity

		livingRoomCO2, err := db.GetAggregates(livingRoomRoom, livingRoomDevice, "co2", days)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		data.LivingRoomCO2 = livingRoomCO2

		livingRoomPressure, err := db.GetAggregates(livingRoomRoom, livingRoomDevice, "pressure", days)
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
