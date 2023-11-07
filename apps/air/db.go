package main

import (
	"time"

	"gorm.io/gorm"
	"monks.co/pkg/database"
)

func NewDB() (*database.DB, error) {
	db, err := database.Open("/data/tank/venta/venta.db")
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&Parameters{}); err != nil {
		return nil, err
	}

	return db, nil
}

type Parameters struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

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
