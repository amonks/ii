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
	
	// Get Venta parameters (legacy approach)
	ventaParams, err := getDeviceParameters("60:8A:10:B5:58:A0")
	if err != nil {
		return err
	}

	modiCategory, err := getModiCategory()
	if err != nil {
		return err
	}
	ventaParams.ModiCategory = modiCategory
	
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
			
			// Also update legacy Parameters struct so we maintain backward compatibility
			addAranetDataToParameters(ventaParams, aranetDevices)
		}
	}
	
	// Notify if Venta water level stopped being full and stayed stably not-full for 2 consecutive checks
	// Use the new data model to check water levels using the ventaRoom and ventaDevice defined earlier
	waterLevelPoints, err := db.GetLastDatapointsByParameter(ventaRoom, ventaDevice, "water_level", 2)
	if err != nil {
		return err
	}
	
	// Check current water level value from ventaParams
	currentWaterLevelFull := ventaParams.WaterLevel.IsFull()
	
	// If we have enough history to check
	if len(waterLevelPoints) >= 2 {
		// We check in reverse because results are ordered by created_at desc
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
	
	// Log Venta values
	fmt.Printf("Venta - temp: %f, humid: %f, water: %d\n", 
		ventaParams.Temperature, ventaParams.Humidity, ventaParams.WaterLevel)
	
	// Log Aranet values
	if ventaParams.OfficeAranetValid {
		fmt.Printf("Office - temp: %f, humid: %d, co2: %d, pressure: %f\n", 
			ventaParams.OfficeTemperature, ventaParams.OfficeHumidity, 
			ventaParams.OfficeCO2, ventaParams.OfficePressure)
	}
	
	if ventaParams.LivingRoomAranetValid {
		fmt.Printf("Living Room - temp: %f, humid: %d, co2: %d, pressure: %f\n", 
			ventaParams.LivingRoomTemperature, ventaParams.LivingRoomHumidity, 
			ventaParams.LivingRoomCO2, ventaParams.LivingRoomPressure)
	}

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

// getAranetDevices fetches data from the Aranet API endpoint
func getAranetDevices() ([]AranetDevice, error) {
	res, err := http.Get("https://brigid.ss.cx/aranet/")
	if err != nil {
		return nil, fmt.Errorf("error fetching Aranet data: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Aranet API returned non-OK status: %d", res.StatusCode)
	}

	bs, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading Aranet response: %w", err)
	}

	var devices []AranetDevice
	if err := json.Unmarshal(bs, &devices); err != nil {
		return nil, fmt.Errorf("error parsing Aranet JSON: %w", err)
	}

	return devices, nil
}

// addAranetDataToParameters adds Aranet data to the parameters struct
func addAranetDataToParameters(params *Parameters, devices []AranetDevice) {
	for _, device := range devices {
		// Determine which location this device is for
		if isOfficeDevice(device.Name) {
			params.OfficeAranetValid = device.TemperatureValid && device.HumidityValid
			params.OfficeTemperature = device.TemperatureF
			params.OfficeHumidity = device.Humidity
			params.OfficeCO2 = device.CO2
			params.OfficePressure = device.Pressure
			params.OfficeBattery = device.Battery
			params.OfficeVersion = device.Version
		} else if isLivingRoomDevice(device.Name) {
			params.LivingRoomAranetValid = device.TemperatureValid && device.HumidityValid
			params.LivingRoomTemperature = device.TemperatureF
			params.LivingRoomHumidity = device.Humidity
			params.LivingRoomCO2 = device.CO2
			params.LivingRoomPressure = device.Pressure
			params.LivingRoomBattery = device.Battery
			params.LivingRoomVersion = device.Version
		}
	}
}

// isOfficeDevice determines if the device is the office sensor
func isOfficeDevice(deviceName string) bool {
	return deviceName == "Aranet4 0AC6E"
}

// isLivingRoomDevice determines if the device is the living room sensor
func isLivingRoomDevice(deviceName string) bool {
	return deviceName == "Aranet4 069F9"
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

func getDeviceParameters(deviceMAC string) (*Parameters, error) {
	res, err := http.Get("https://venta-app-gateway-prod.azurewebsites.net/1/devices/60:8A:10:B5:58:A0/parameters")
	if err != nil {
		return nil, err
	}

	bs, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	parameters := &DeviceParametersResponse{}
	if err := json.Unmarshal(bs, parameters); err != nil {
		return nil, err
	}
	return &Parameters{
		CreatedAt: time.Now(),

		DeviceType: parameters.Header.DeviceType,
		MacAddress: parameters.Header.MacAdress,
		Error:      parameters.Header.Error,
		Hash:       parameters.Header.Hash,
		DeviceName: parameters.Header.DeviceName,

		SWDisplay: parameters.Info.SWDisplay,
		SWWifi:    parameters.Info.SWWIFI,
		UVCOffT:   parameters.Info.UVCOffT,

		RelState0: parameters.Info.RelState[0],
		RelState1: parameters.Info.RelState[1],
		RelState2: parameters.Info.RelState[2],
		RelState3: parameters.Info.RelState[3],

		DiscIonT:   parameters.Info.DiscIonT,
		ServiceT:   parameters.Info.ServiceT,
		UVCOnT:     parameters.Info.UVCOnT,
		SWTouch:    parameters.Info.SWTouch,
		FilterT:    parameters.Info.FilterT,
		SWPower:    parameters.Info.SWPower,
		CleaningT:  parameters.Info.CleaningT,
		CleanMode:  parameters.Info.CleanMode,
		CleaningR:  parameters.Info.CleaningR,
		OperationT: parameters.Info.OperationT,
		Warnings:   parameters.Info.Warnings,
		TimerT:     parameters.Info.TimerT,

		FanRPM:      parameters.Measure.FanRpm,
		Temperature: parameters.Measure.Temperature,
		Dust:        parameters.Measure.Dust,
		WaterLevel:  WaterLevel(parameters.Measure.WaterLevel),
		Humidity:    parameters.Measure.Humidity,
	}, nil
}
