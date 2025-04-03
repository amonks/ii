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
	// Get Venta parameters
	next, err := getDeviceParameters("60:8A:10:B5:58:A0")
	if err != nil {
		return err
	}

	modiCategory, err := getModiCategory()
	if err != nil {
		return err
	}
	next.ModiCategory = modiCategory

	// Get Aranet parameters
	aranetDevices, err := getAranetDevices()
	if err != nil {
		log.Printf("Warning: Failed to fetch Aranet data: %v", err)
		// Continue even if Aranet fetch fails
	} else {
		// Add Aranet data to parameters
		addAranetDataToParameters(next, aranetDevices)
	}

	// Notify if it stopped being full and stayed stably not-full for 2
	// consecutive checks.
	last2, err := db.LastN(2)
	if err != nil {
		return err
	}
	
	if len(last2) >= 2 {
		back1, back2 := last2[0], last2[1]
		if back2.WaterLevel.IsFull() && !back1.WaterLevel.IsFull() && !next.WaterLevel.IsFull() {
			if err := twilio.SMSMe("alert: low water in air purifier"); err != nil {
				log.Printf("twilio error: %s", err)
			}
		}
	}

	if err := db.Insert(next); err != nil {
		return fmt.Errorf("error inserting fetched parameters: %w", err)
	}

	// Log data values
	fmt.Printf("Venta - temp: %f, humid: %f, water: %d\n", next.Temperature, next.Humidity, next.WaterLevel)
	
	if next.OfficeAranetValid {
		fmt.Printf("Office - temp: %f, humid: %d, co2: %d, pressure: %f\n", 
			next.OfficeTemperature, next.OfficeHumidity, next.OfficeCO2, next.OfficePressure)
	}
	
	if next.LivingRoomAranetValid {
		fmt.Printf("Living Room - temp: %f, humid: %d, co2: %d, pressure: %f\n", 
			next.LivingRoomTemperature, next.LivingRoomHumidity, next.LivingRoomCO2, next.LivingRoomPressure)
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
