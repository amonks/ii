package main

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func NewDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("/data/tank/venta/venta.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Migrate the schema
	if err := db.AutoMigrate(&Parameters{}); err != nil {
		return nil, err
	}

	return db, nil
}

type Parameters struct {
	gorm.Model

	ModiCategory string

	DeviceType int
	MacAddress string
	Error      int
	Hash       string
	DeviceName string

	SWDisplay string
	SWWifi    string
	UVCOffT   int

	RelState0 bool
	RelState1 bool
	RelState2 bool
	RelState3 bool

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

	FanRPM      int
	Temperature float64
	Dust        int
	WaterLevel  int
	Humidity    float64
}
