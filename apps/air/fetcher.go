package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"monks.co/pkg/tailnet"
)

func fetch(db *DB) error {
	startTime := time.Now()
	log.Printf("Fetch: starting data collection")

	// Initialize a slice to hold all data points
	var allDataPoints []DataPoint
	createdAt := time.Now()

	// Helper to add a datapoint
	addDataPoint := func(room RoomID, device, parameter string, value float64) {
		allDataPoints = append(allDataPoints, DataPoint{
			CreatedAt: createdAt,
			Room:      string(room),
			Device:    device,
			Parameter: parameter,
			Value:     value,
		})
	}

	// Get Aranet parameters
	aranetStart := time.Now()
	aranetDevices, err := getAranetDevices()
	log.Printf("Fetch: Aranet data fetch took %v", time.Since(aranetStart))
	if err != nil {
		return fmt.Errorf("failed to fetch Aranet data: %w", err)
	} else if aranetDevices == nil {
		// No new data (304 Not Modified or duplicate timestamp)
		log.Printf("No new Aranet data to process")
	} else {
		// Add Aranet data points
		for _, device := range aranetDevices {
			deviceId := device.Name
			room, found := DeviceToRoom(deviceId)
			if !found {
				log.Printf("Warning: Unknown device %s, skipping", deviceId)
				continue
			}

			if device.TemperatureValid {
				addDataPoint(room, deviceId, "temperature", device.TemperatureF)
			}

			if device.HumidityValid {
				addDataPoint(room, deviceId, "humidity", float64(device.Humidity))
			}

			if device.CO2Valid {
				addDataPoint(room, deviceId, "co2", float64(device.CO2))
			}

			if device.PressureValid {
				addDataPoint(room, deviceId, "pressure", device.Pressure)
			}

			addDataPoint(room, deviceId, "battery", float64(device.Battery))
		}
	}

	// Insert all new data points
	dbInsertStart := time.Now()
	if err := db.InsertDataPoints(allDataPoints); err != nil {
		return fmt.Errorf("error inserting data points: %w", err)
	}
	log.Printf("Fetch: DB insert of %d data points took %v", len(allDataPoints), time.Since(dbInsertStart))

	// Log total execution time
	log.Printf("Fetch: completed in %v with %d data points collected", time.Since(startTime), len(allDataPoints))

	return nil
}

// AranetDevice represents the data from an Aranet device
type AranetDevice struct {
	Name             string  `json:"name"`
	Address          string  `json:"address"`
	Version          string  `json:"version"`
	Flags            int     `json:"flags"`
	CO2Valid         bool    `json:"co2_valid"`
	CO2              int     `json:"co2"`
	TemperatureValid bool    `json:"temperature_valid"`
	TemperatureC     float64 `json:"temperature_c"`
	TemperatureF     float64 `json:"temperature_f"`
	HumidityValid    bool    `json:"humidity_valid"`
	Humidity         int     `json:"humidity"`
	PressureValid    bool    `json:"pressure_valid"`
	Pressure         float64 `json:"pressure"`
	Battery          int     `json:"battery"`
	Status           int     `json:"status"`
	Interval         int     `json:"interval"`
	Ago              int     `json:"ago"`
}

// AranetResponse represents the new response format from the Aranet service
type AranetResponse struct {
	Timestamp time.Time      `json:"timestamp"`
	Devices   []AranetDevice `json:"devices"`
}

// Store the last timestamp we received from the Aranet service
var lastAranetTimestamp time.Time

// getAranetDevices fetches data from the Aranet API endpoint
func getAranetDevices() ([]AranetDevice, error) {
	log.Printf("getAranetDevices: starting fetch")
	httpStart := time.Now()

	client := tailnet.Client()

	req, err := http.NewRequest("GET", "http://monks-aranet-brigid/", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Send If-Modified-Since header if we have a previous timestamp
	if !lastAranetTimestamp.IsZero() {
		req.Header.Set("If-Modified-Since", lastAranetTimestamp.Format(http.TimeFormat))
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching Aranet data: %w", err)
	}
	defer res.Body.Close()
	log.Printf("getAranetDevices: HTTP request took %v", time.Since(httpStart))

	// Check if the response is 304 Not Modified
	if res.StatusCode == http.StatusNotModified {
		log.Printf("Aranet data not modified since last fetch")
		return nil, nil
	}

	if res.StatusCode != http.StatusOK {
		//lint:ignore ST1005 proper noun
		return nil, fmt.Errorf("Aranet API returned non-OK status: %d", res.StatusCode)
	}

	// Parse the Last-Modified header
	lastModified := res.Header.Get("Last-Modified")
	if lastModified != "" {
		modTime, err := time.Parse(http.TimeFormat, lastModified)
		if err == nil {
			// Check if this is a duplicate timestamp
			if !lastAranetTimestamp.IsZero() && modTime.Equal(lastAranetTimestamp) {
				log.Printf("Skipping duplicate Aranet data with timestamp: %v", modTime)
				return nil, nil
			}
			lastAranetTimestamp = modTime
		}
	}

	readStart := time.Now()
	bs, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading Aranet response: %w", err)
	}
	log.Printf("getAranetDevices: reading response body took %v", time.Since(readStart))

	jsonStart := time.Now()
	var response AranetResponse
	if err := json.Unmarshal(bs, &response); err != nil {
		// Try parsing the old format as a fallback
		var devices []AranetDevice
		if err2 := json.Unmarshal(bs, &devices); err2 == nil {
			log.Printf("Warning: Aranet service returned old format without timestamp")
			return devices, nil
		}
		return nil, fmt.Errorf("error parsing Aranet JSON: %w", err)
	}
	log.Printf("getAranetDevices: JSON unmarshaling took %v with %d devices found", time.Since(jsonStart), len(response.Devices))

	// Update lastAranetTimestamp with the timestamp from the response
	lastAranetTimestamp = response.Timestamp

	return response.Devices, nil
}
