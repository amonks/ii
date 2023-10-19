package main

import (
	"encoding/json"
	"io"
	"net/http"

	"gorm.io/gorm"
)

func fetch(db *gorm.DB) error {
	params, err := getDeviceParameters("60:8A:10:B5:58:A0")
	if err != nil {
		return err
	}

	modiCategory, err := getModiCategory()
	if err != nil {
		return err
	}
	params.ModiCategory = modiCategory

	if tx := db.Create(params); tx.Error != nil {
		return tx.Error
	}

	return nil
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
		WaterLevel:  parameters.Measure.WaterLevel,
		Humidity:    parameters.Measure.Humidity,
	}, nil
}
