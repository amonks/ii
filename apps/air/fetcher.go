package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"monks.co/pkg/twilio"
)

func fetch(db *DB) error {
	// Initialize a slice to hold all data points
	var allDataPoints []DataPoint
	createdAt := time.Now()
	
	// Get Venta parameters (legacy approach for water level checking)
	ventaParams, err := getDeviceParameters("60:8A:10:B5:58:A0")
	if err != nil {
		return err
	}

	// Add Venta data points
	ventaDevice := "60:8A:10:B5:58:A0"
	ventaRoom, _ := DeviceToRoom(ventaDevice)
	
	// Helper to add a datapoint
	addDataPoint := func(room, device, parameter string, value float64) {
		allDataPoints = append(allDataPoints, DataPoint{
			CreatedAt: createdAt,
			Room:      room,
			Device:    device,
			Parameter: parameter,
			Value:     value,
		})
	}
	
	// Add Venta data points
	addDataPoint(ventaRoom, ventaDevice, "temperature", ventaParams.Temperature)
	addDataPoint(ventaRoom, ventaDevice, "humidity", ventaParams.Humidity)
	addDataPoint(ventaRoom, ventaDevice, "dust", float64(ventaParams.Dust))
	addDataPoint(ventaRoom, ventaDevice, "water_level", float64(ventaParams.WaterLevel))
	addDataPoint(ventaRoom, ventaDevice, "fan_rpm", float64(ventaParams.FanRPM))

	// Get Aranet parameters
	aranetDevices, err := getAranetDevices()
	if err != nil {
		log.Printf("Warning: Failed to fetch Aranet data: %v", err)
		// Continue even if Aranet fetch fails
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
	
	// Notify if Venta water level stopped being full and stayed stably not-full for 2 consecutive checks
	waterLevelPoints, err := db.GetLastDatapointsByParameter(ventaRoom, ventaDevice, "water_level", 2)
	if err != nil {
		return err
	}
	
	// Calculate current water level value
	currentWaterLevelValue := float64(ventaParams.WaterLevel)
	currentWaterLevelFull := IsWaterLevelFull(currentWaterLevelValue)
	
	// If we have enough history to check
	if len(waterLevelPoints) >= 2 {
		// Results are ordered by created_at desc
		back1WaterLevelFull := IsWaterLevelFull(waterLevelPoints[0].Value)
		back2WaterLevelFull := IsWaterLevelFull(waterLevelPoints[1].Value)
		
		// If it was full, then not full, and still not full
		if back2WaterLevelFull && !back1WaterLevelFull && !currentWaterLevelFull {
			if err := twilio.SMSMe("alert: low water in air purifier"); err != nil {
				log.Printf("twilio error: %s", err)
			}
		}
	}

	// Insert all new data points
	if err := db.InsertDataPoints(allDataPoints); err != nil {
		return fmt.Errorf("error inserting data points: %w", err)
	}

	// Log data values
	fmt.Printf("Collected %d data points\n", len(allDataPoints))
	
	return nil
}

// AranetDevice represents the data from an Aranet device
type AranetDevice struct {
	Name            string  `json:"name"`
	Address         string  `json:"address"`
	Version         string  `json:"version"`
	Flags           int     `json:"flags"`
	CO2Valid        bool    `json:"co2_valid"`
	CO2             int     `json:"co2"`
	TemperatureValid bool   `json:"temperature_valid"`
	TemperatureC    float64 `json:"temperature_c"`
	TemperatureF    float64 `json:"temperature_f"`
	HumidityValid   bool    `json:"humidity_valid"`
	Humidity        int     `json:"humidity"`
	PressureValid   bool    `json:"pressure_valid"`
	Pressure        float64 `json:"pressure"`
	Battery         int     `json:"battery"`
	Status          int     `json:"status"`
	Interval        int     `json:"interval"`
	Ago             int     `json:"ago"`
}

// AranetResponse represents the new response format from the Aranet service
type AranetResponse struct {
	Timestamp time.Time       `json:"timestamp"`
	Devices   []AranetDevice  `json:"devices"`
}

// Store the last timestamp we received from the Aranet service
var lastAranetTimestamp time.Time

// getAranetDevices fetches data from the Aranet API endpoint
func getAranetDevices() ([]AranetDevice, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", "https://brigid.ss.cx/aranet/", nil)
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

	// Check if the response is 304 Not Modified
	if res.StatusCode == http.StatusNotModified {
		log.Printf("Aranet data not modified since last fetch")
		return nil, nil
	}

	if res.StatusCode != http.StatusOK {
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

	bs, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading Aranet response: %w", err)
	}

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

	// Update lastAranetTimestamp with the timestamp from the response
	lastAranetTimestamp = response.Timestamp

	return response.Devices, nil
}

type GetRoomResponse []struct {
	ModiCategory struct {
		Name string `json:"name"`
	} `json:"modi_category"`
}

func getModiCategory() (string, error) {
	res, err := http.Get("https://venta-app-gateway-prod.azurewebsites.net/1/rooms?owner_id=5ad200d8-9b4f-45e9-ab69-3df9f7c75efb")
	if err != nil {
		return "", err
	}

	bs, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	room := GetRoomResponse{}
	if err := json.Unmarshal(bs, &room); err != nil {
		return "", err
	}
	return room[0].ModiCategory.Name, nil
}

type DeviceParametersResponse struct {
	Header struct {
		DeviceType int
		MacAdress  string
		Error      int
		Hash       string
		DeviceName string
	} `json:"header"`
	Info struct {
		SWDisplay  string
		SWWIFI     string
		UVCOffT    int
		RelState   []bool
		DiscIonT   int
		ServiceT   int
		UVCOnT     int
		SWTouch    string
		FilterT    int
		SWPower    string
		CleaningT  int
		CleanMode  bool
		CleaningR  int
		OperationT int
		Warnings   int
		TimerT     int
	} `json:"info"`
	Measure struct {
		FanRpm      int
		Temperature float64
		Dust        int
		WaterLevel  int
		Humidity    float64
	} `json:"measure"`
}

func getDeviceParameters(deviceMAC string) (*VentaParameters, error) {
	res, err := http.Get("https://venta-app-gateway-prod.azurewebsites.net/1/devices/60:8A:10:B5:58:A0/parameters")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	bs, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	
	parameters := &DeviceParametersResponse{}
	if err := json.Unmarshal(bs, parameters); err != nil {
		return nil, err
	}
	
	// Only extract the fields we actually use
	return &VentaParameters{
		CreatedAt:    time.Now(),
		FanRPM:      parameters.Measure.FanRpm,
		Temperature: parameters.Measure.Temperature,
		Dust:        parameters.Measure.Dust,
		WaterLevel:  WaterLevel(parameters.Measure.WaterLevel),
		Humidity:    parameters.Measure.Humidity,
	}, nil
}

// VentaParameters contains only the essential data from the Venta device
type VentaParameters struct {
	CreatedAt time.Time

	// Only keep the fields we actually need
	FanRPM      int
	Temperature float64
	Dust        int
	WaterLevel  WaterLevel
	Humidity    float64
}