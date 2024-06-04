package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
	"monks.co/pkg/database"
)

type DB struct {
	*database.DB
}

func NewDB() (*DB, error) {
	db, err := database.Open("/data/tank/venta/venta.db")
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&Parameters{}, &Aggregate{}); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func (db *DB) Aggregate(param AggregateID, days int64) ([]Aggregate, error) {
	agg := int64(time.Hour)
	if days > 30 {
		agg = int64(24 * time.Hour)
	}
	daysQ := fmt.Sprintf("date('now', '-%d day')", days)
	var points []Aggregate
	if err := db.Table("aggregates").
		Where("window_start_at >= "+daysQ+" AND parameter = ? AND window_duration = ?", param, agg).
		Find(&points).
		Error; err != nil {
		return nil, err
	}
	return points, nil
}

func (db *DB) Last() (*Parameters, error) {
	var last Parameters
	if err := db.
		Table("parameters").
		Order("created_at desc").
		First(&last).
		Error; err != nil {
		return nil, fmt.Errorf("error getting last aggregate: %w", err)
	}
	return &last, nil
}

func (db *DB) calculateAggregates() error {
	all := []*Parameters{}
	if err := db.Table("parameters").Order("created_at asc").Find(&all).Error; err != nil {
		return fmt.Errorf("error getting all params: %w", err)
	}
	for i, p := range all {
		if i%100 == 0 {
			log.Println(p.CreatedAt)
		}
		if err := db.updateAggregates(p); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) Insert(parameters *Parameters) error {
	if parameters == nil {
		panic("nil params")
	}
	if db == nil {
		panic("nil db")
	}
	if err := db.Create(parameters).Error; err != nil {
		return fmt.Errorf("error inserting parameters: %w", err)
	}

	if err := db.updateAggregates(parameters); err != nil {
		return err
	}

	return nil
}

func (db *DB) updateAggregates(parameters *Parameters) error {
	dayStart := parameters.CreatedAt.Truncate(24 * time.Hour)
	if err := db.updateAggregate(AggregateIDDust, time.Hour*24, dayStart, float64(parameters.Dust)); err != nil {
		return fmt.Errorf("error updating daily temp aggregate: %w", err)
	}
	if err := db.updateAggregate(AggregateIDTemperature, time.Hour*24, dayStart, parameters.Temperature); err != nil {
		return fmt.Errorf("error updating daily temp aggregate: %w", err)
	}
	if err := db.updateAggregate(AggregateIDHumidity, time.Hour*24, dayStart, parameters.Humidity); err != nil {
		return fmt.Errorf("error updating daily humidity aggregate: %w", err)
	}

	hourStart := parameters.CreatedAt.Truncate(time.Hour)
	if err := db.updateAggregate(AggregateIDDust, time.Hour, hourStart, float64(parameters.Dust)); err != nil {
		return fmt.Errorf("error updating hourly temp aggregate: %w", err)
	}
	if err := db.updateAggregate(AggregateIDTemperature, time.Hour, hourStart, parameters.Temperature); err != nil {
		return fmt.Errorf("error updating hourly temp aggregate: %w", err)
	}
	if err := db.updateAggregate(AggregateIDHumidity, time.Hour, hourStart, parameters.Humidity); err != nil {
		return fmt.Errorf("error updating hourly humidity aggregate: %w", err)
	}

	return nil
}

func (db *DB) updateAggregate(parameter AggregateID, windowDuration time.Duration, windowStartAt time.Time, value float64) error {
	agg, err := db.getAggregate(parameter, windowDuration, windowStartAt)
	if err != nil {
		return fmt.Errorf("error getting aggregate: %w", err)
	}
	if agg.Count == 0 {
		agg.Min, agg.Max, agg.Mean = value, value, value
		agg.Count = 1
		if err := db.Create(agg).Error; err != nil {
			return fmt.Errorf("error inserting aggregate: %w", err)
		}
		return nil
	}

	if value < agg.Min {
		agg.Min = value
	}
	if value > agg.Max {
		agg.Max = value
	}
	agg.Mean += (value - agg.Mean) / float64(agg.Count+1)
	agg.Count += 1
	if err := db.Updates(agg).Error; err != nil {
		return fmt.Errorf("error updating aggregate: %w", err)
	}
	return nil
}

func (db *DB) getAggregate(parameter AggregateID, windowDuration time.Duration, windowStartAt time.Time) (*Aggregate, error) {
	var agg Aggregate
	tx := db.Table("aggregates").
		Where(&Aggregate{
			Parameter:      parameter,
			WindowDuration: windowDuration,
			WindowStartAt:  windowStartAt,
		}).First(&agg)
	if err := tx.Error; err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("error getting aggregate: %w", err)
	}
	if err := tx.Error; err != nil {
		return &Aggregate{
			Parameter:      parameter,
			WindowDuration: windowDuration,
			WindowStartAt:  windowStartAt,
		}, nil
	}
	return &agg, nil
}

type AggregateID int

const (
	aggregateIDUnknown AggregateID = iota
	AggregateIDTemperature
	AggregateIDHumidity
	AggregateIDDust
)

type Aggregate struct {
	Parameter      AggregateID   `gorm:"primaryKey"`
	WindowDuration time.Duration `gorm:"primaryKey"`
	WindowStartAt  time.Time     `gorm:"primaryKey"`

	Min   float64
	Max   float64
	Mean  float64
	Count int64
}

type Parameters struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time

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
