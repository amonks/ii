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
	"monks.co/pkg/tailnet"
)

var (
	//go:embed templates/graph.gohtml
	tmplSrc string
)

// DeviceData holds aggregates for a specific device and includes device/room info
type DeviceData struct {
	DeviceName string            // Display name for the device
	RoomName   string            // Display name for the room
	Data       []WindowAggregate // The actual data points
}

// Data is the structure passed to the template for rendering
type Data struct {
	// Data organized by parameter type
	Temperatures map[string]DeviceData // Key is "room-device"
	Humidities   map[string]DeviceData
	CO2s         map[string]DeviceData
	Pressures    map[string]DeviceData

	// Device mappings - to help with display names
	DeviceDisplayNames map[string]string // Maps device ID to human readable name
	RoomDisplayNames   map[string]string // Maps room ID to human readable name
}

func (d *Data) JSON() (template.JS, error) {
	bs, err := json.Marshal(d)
	if err != nil {
		return "", err
	}

	return template.JS("window.data = " + string(bs) + ";"), nil
}

func serveAir(ctx context.Context, db *DB) error {
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

		// Initialize data structure with maps
		data := &Data{
			Temperatures:       make(map[string]DeviceData),
			Humidities:         make(map[string]DeviceData),
			CO2s:               make(map[string]DeviceData),
			Pressures:          make(map[string]DeviceData),
			DeviceDisplayNames: GetDeviceDisplayNames(),
			RoomDisplayNames:   GetRoomDisplayNames(),
		}

		// Use the devices configuration from devices.go
		var deviceConfigs []struct {
			DeviceId   string
			Room       string
			Parameters []string
		}

		// Convert our centralized device configuration
		for _, device := range Devices {
			deviceConfigs = append(deviceConfigs, struct {
				DeviceId   string
				Room       string
				Parameters []string
			}{
				DeviceId:   device.ID,
				Room:       string(device.RoomID),
				Parameters: device.GetParameters(),
			})
		}

		// Fetch data for each device and parameter
		for _, device := range deviceConfigs {
			deviceKey := device.Room + "-" + device.DeviceId

			for _, param := range device.Parameters {
				// Get aggregates for this device/parameter
				aggs, err := db.GetAggregates(device.Room, device.DeviceId, param, days)
				if err != nil {
					errs = errors.Join(errs, err)
					continue
				}

				// Skip if no data
				if len(aggs) == 0 {
					continue
				}

				// Create device data
				deviceData := DeviceData{
					DeviceName: data.DeviceDisplayNames[device.DeviceId],
					RoomName:   data.RoomDisplayNames[device.Room],
					Data:       aggs,
				}

				// Store in the appropriate collection based on parameter
				switch param {
				case "temperature":
					data.Temperatures[deviceKey] = deviceData
				case "humidity":
					data.Humidities[deviceKey] = deviceData
				case "co2":
					data.CO2s[deviceKey] = deviceData
				case "pressure":
					data.Pressures[deviceKey] = deviceData
				}
			}
		}

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

	if err := tailnet.ListenAndServe(ctx, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}
