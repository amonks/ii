package main

import (
	"context"
	"embed"
	"fmt"
	"time"

	"gorm.io/gorm"
	"monks.co/pkg/database"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	*database.DB
}

func NewDB() (*DB, error) {
	db, err := database.Open("/data/tank/venta/venta.db")
	if err != nil {
		return nil, fmt.Errorf("opening /data/tank/venta/venta.db: %w", err)
	}

	if err := db.MigrateFS(context.Background(), migrationsFS, "migrations", "001_baseline.sql"); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &DB{db}, nil
}

// DataPoint represents a single measurement from a sensor
type DataPoint struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time

	Room      string  // e.g., "living room", "office"
	Device    string  // e.g., "aranet4 069F9", "venta air purifier"
	Parameter string  // e.g., "temperature", "humidity", "co2", "water_level", "fan_rpm", "battery"
	Value     float64 // all values stored as float64 for consistency
}

// WindowAggregate is the structure for aggregated data points
type WindowAggregate struct {
	Room           string        `gorm:"primaryKey"`
	Device         string        `gorm:"primaryKey"`
	Parameter      string        `gorm:"primaryKey"`
	WindowDuration time.Duration `gorm:"primaryKey"`
	WindowStartAt  time.Time     `gorm:"primaryKey"`

	Min   float64
	Max   float64
	Mean  float64
	Count int64
}

// TableName overrides the table name used by GORM
func (WindowAggregate) TableName() string {
	return "window_aggregates"
}

// GetAggregates retrieves aggregates for a specific room, device, and parameter
func (db *DB) GetAggregates(room, device, parameter string, days int64) ([]WindowAggregate, error) {
	windowDuration := int64(time.Hour)
	if days > 30 {
		windowDuration = int64(24 * time.Hour)
	}
	daysQ := fmt.Sprintf("date('now', '-%d day')", days)
	var points []WindowAggregate
	if err := db.Table("window_aggregates").
		Where("window_start_at >= "+daysQ+" AND room = ? AND device = ? AND parameter = ? AND window_duration = ?", room, device, parameter, windowDuration).
		Find(&points).
		Error; err != nil {
		return nil, err
	}
	return points, nil
}

// InsertDataPoints inserts multiple data points from various devices
func (db *DB) InsertDataPoints(points []DataPoint) error {
	if len(points) == 0 {
		return nil
	}

	// Keep track of all created points, to aggregate them later
	if err := db.Create(&points).Error; err != nil {
		return fmt.Errorf("error inserting data points: %w", err)
	}

	// Calculate the start time based on the first point
	// (assumes points are all from the same approximate time)
	createdAt := points[0].CreatedAt
	dayStart := createdAt.Truncate(24 * time.Hour)
	hourStart := createdAt.Truncate(time.Hour)

	// Group by room/device/parameter for aggregation
	dataByKey := make(map[string]map[string]map[string][]float64)

	// Organize data for aggregation
	for _, point := range points {
		// Initialize nested maps if they don't exist
		if _, exists := dataByKey[point.Room]; !exists {
			dataByKey[point.Room] = make(map[string]map[string][]float64)
		}
		if _, exists := dataByKey[point.Room][point.Device]; !exists {
			dataByKey[point.Room][point.Device] = make(map[string][]float64)
		}
		if _, exists := dataByKey[point.Room][point.Device][point.Parameter]; !exists {
			dataByKey[point.Room][point.Device][point.Parameter] = []float64{}
		}

		// Add this value to the appropriate slice
		dataByKey[point.Room][point.Device][point.Parameter] = append(
			dataByKey[point.Room][point.Device][point.Parameter],
			point.Value,
		)
	}

	// Update aggregates for each room/device/parameter
	for room, deviceMap := range dataByKey {
		for device, paramMap := range deviceMap {
			for param, values := range paramMap {
				for _, value := range values {
					// Daily aggregates
					if err := db.updateAggregate(room, device, param, time.Hour*24, dayStart, value); err != nil {
						return fmt.Errorf("error updating daily aggregate for %s/%s/%s: %w", room, device, param, err)
					}

					// Hourly aggregates
					if err := db.updateAggregate(room, device, param, time.Hour, hourStart, value); err != nil {
						return fmt.Errorf("error updating hourly aggregate for %s/%s/%s: %w", room, device, param, err)
					}
				}
			}
		}
	}

	return nil
}

// calculateAggregates recalculates all window aggregates from the data points
func (db *DB) calculateAggregates() error {
	// Get all data points ordered by time
	var datapoints []DataPoint
	if err := db.Table("data_points").Order("created_at asc").Find(&datapoints).Error; err != nil {
		return fmt.Errorf("error getting all data points: %w", err)
	}

	fmt.Printf("Calculating aggregates for %d data points\n", len(datapoints))

	// Clear existing aggregates
	if err := db.Exec("DELETE FROM window_aggregates").Error; err != nil {
		return fmt.Errorf("error clearing aggregates: %w", err)
	}

	// Process by day to reduce memory usage
	currentDay := time.Time{}
	batchPoints := []DataPoint{}

	for i, dp := range datapoints {
		if i%1000 == 0 {
			fmt.Printf("Processed %d/%d data points\n", i, len(datapoints))
		}

		dpDay := dp.CreatedAt.Truncate(24 * time.Hour)

		// If we've moved to a new day, process the batch
		if !currentDay.IsZero() && !dpDay.Equal(currentDay) {
			if err := db.InsertDataPoints(batchPoints); err != nil {
				return err
			}
			batchPoints = []DataPoint{}
		}

		currentDay = dpDay
		batchPoints = append(batchPoints, dp)
	}

	// Process the final batch
	if len(batchPoints) > 0 {
		if err := db.InsertDataPoints(batchPoints); err != nil {
			return err
		}
	}

	return nil
}

// updateAggregate updates the aggregate for a given room/device/parameter
func (db *DB) updateAggregate(room, device, parameter string, windowDuration time.Duration, windowStartAt time.Time, value float64) error {
	agg, err := db.getAggregate(room, device, parameter, windowDuration, windowStartAt)
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

// getAggregate retrieves an aggregate or returns a new one if not found
func (db *DB) getAggregate(room, device, parameter string, windowDuration time.Duration, windowStartAt time.Time) (*WindowAggregate, error) {
	var agg WindowAggregate
	tx := db.Table("window_aggregates").
		Where(&WindowAggregate{
			Room:           room,
			Device:         device,
			Parameter:      parameter,
			WindowDuration: windowDuration,
			WindowStartAt:  windowStartAt,
		}).First(&agg)

	if err := tx.Error; err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("error getting aggregate: %w", err)
	}

	if err := tx.Error; err != nil {
		return &WindowAggregate{
			Room:           room,
			Device:         device,
			Parameter:      parameter,
			WindowDuration: windowDuration,
			WindowStartAt:  windowStartAt,
		}, nil
	}

	return &agg, nil
}
