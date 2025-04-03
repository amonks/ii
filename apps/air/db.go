package main

import (
	"fmt"
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

	// Migrate the database to include both legacy and new structures
	if err := db.AutoMigrate(&Parameters{}, &Aggregate{}, &DataPoint{}, &WindowAggregate{}); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

// MigrateData migrates data from the legacy schema to the new schema
func (db *DB) MigrateData() error {
	// Skip parameters migration since it's already completed
	fmt.Println("Skipping Parameters migration - already completed")

	// Just migrate aggregates after filtering out string-based ones
	if err := db.migrateAggregates(); err != nil {
		return fmt.Errorf("failed to migrate aggregates: %w", err)
	}
	fmt.Println("Aggregates migration completed successfully!")

	return nil
}

// migrateParameters converts old Parameters records to new DataPoint records
func (db *DB) migrateParameters() error {
	var parameters []Parameters
	if err := db.Table("parameters").Find(&parameters).Error; err != nil {
		return fmt.Errorf("error fetching parameters: %w", err)
	}

	fmt.Printf("Migrating %d parameter records...\n", len(parameters))

	for i, param := range parameters {
		if i%100 == 0 {
			fmt.Printf("Processed %d/%d parameters\n", i, len(parameters))
		}

		dataPoints := parametersToDataPoints(&param)
		for _, dp := range dataPoints {
			if err := db.Create(&dp).Error; err != nil {
				return fmt.Errorf("error creating data point: %w", err)
			}
		}
	}

	return nil
}

// parametersToDataPoints converts a single Parameters record to multiple DataPoint records
func parametersToDataPoints(param *Parameters) []DataPoint {
	var dataPoints []DataPoint

	// Helper to add a datapoint
	addDataPoint := func(room, device, parameter string, value float64) {
		dataPoints = append(dataPoints, DataPoint{
			CreatedAt: param.CreatedAt,
			Room:      room,
			Device:    device,
			Parameter: parameter,
			Value:     value,
		})
	}

	// Venta Air Purifier data
	ventaDeviceID := "60:8A:10:B5:58:A0"
	ventaRoom := "living room" // Hardcoded based on our mapping

	addDataPoint(ventaRoom, ventaDeviceID, "temperature", param.Temperature)
	addDataPoint(ventaRoom, ventaDeviceID, "humidity", param.Humidity)
	addDataPoint(ventaRoom, ventaDeviceID, "dust", float64(param.Dust))
	addDataPoint(ventaRoom, ventaDeviceID, "water_level", float64(param.WaterLevel))
	addDataPoint(ventaRoom, ventaDeviceID, "fan_rpm", float64(param.FanRPM))

	// Office Aranet data (if valid)
	if param.OfficeAranetValid {
		officeAranetID := "Aranet4 0AC6E"
		officeRoom := "office"

		addDataPoint(officeRoom, officeAranetID, "temperature", param.OfficeTemperature)
		addDataPoint(officeRoom, officeAranetID, "humidity", float64(param.OfficeHumidity))
		addDataPoint(officeRoom, officeAranetID, "co2", float64(param.OfficeCO2))
		addDataPoint(officeRoom, officeAranetID, "pressure", param.OfficePressure)
		addDataPoint(officeRoom, officeAranetID, "battery", float64(param.OfficeBattery))
	}

	// Living Room Aranet data (if valid)
	if param.LivingRoomAranetValid {
		livingRoomAranetID := "Aranet4 069F9"
		livingRoomRoom := "living room"

		addDataPoint(livingRoomRoom, livingRoomAranetID, "temperature", param.LivingRoomTemperature)
		addDataPoint(livingRoomRoom, livingRoomAranetID, "humidity", float64(param.LivingRoomHumidity))
		addDataPoint(livingRoomRoom, livingRoomAranetID, "co2", float64(param.LivingRoomCO2))
		addDataPoint(livingRoomRoom, livingRoomAranetID, "pressure", param.LivingRoomPressure)
		addDataPoint(livingRoomRoom, livingRoomAranetID, "battery", float64(param.LivingRoomBattery))
	}

	return dataPoints
}

// migrateAggregates converts legacy aggregates to new aggregates
func (db *DB) migrateAggregates() error {
	// First, delete the string-based records which are just a handful
	fmt.Println("Deleting the string-based aggregate records...")
	if err := db.Exec("DELETE FROM aggregates WHERE typeof(parameter) = 'text'").Error; err != nil {
		return fmt.Errorf("error deleting string-based aggregates: %w", err)
	}
	
	var legacyAggregates []Aggregate
	if err := db.Table("aggregates").Find(&legacyAggregates).Error; err != nil {
		return fmt.Errorf("error fetching legacy aggregates: %w", err)
	}

	fmt.Printf("Migrating %d aggregate records...\n", len(legacyAggregates))

	for i, legacyAgg := range legacyAggregates {
		if i%100 == 0 {
			fmt.Printf("Processed %d/%d aggregates\n", i, len(legacyAggregates))
		}

		// Look up the mapping for this aggregate ID
		mapping, exists := AggregateIDMapping[legacyAgg.Parameter]
		if !exists {
			fmt.Printf("Warning: No mapping found for aggregate ID %d\n", legacyAgg.Parameter)
			continue
		}

		// Create new aggregate
		newAgg := WindowAggregate{
			Room:           mapping.Room,
			Device:         mapping.Device,
			Parameter:      mapping.Parameter,
			WindowDuration: legacyAgg.WindowDuration,
			WindowStartAt:  legacyAgg.WindowStartAt,
			Min:            legacyAgg.Min,
			Max:            legacyAgg.Max,
			Mean:           legacyAgg.Mean,
			Count:          legacyAgg.Count,
		}

		if err := db.Create(&newAgg).Error; err != nil {
			return fmt.Errorf("error creating new aggregate: %w", err)
		}
	}

	return nil
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

// Legacy compatibility function - maps old AggregateID to new room/device/parameter structure
func (db *DB) Aggregate(param AggregateID, days int64) ([]WindowAggregate, error) {
	// Look up the mapping for this parameter
	mapping, exists := AggregateIDMapping[param]
	if !exists {
		return nil, fmt.Errorf("no mapping found for parameter ID %d", param)
	}

	return db.GetAggregates(mapping.Room, mapping.Device, mapping.Parameter, days)
}

// GetLastDatapointsByParameter returns the latest n datapoints for a specific room, device, and parameter
func (db *DB) GetLastDatapointsByParameter(room, device, parameter string, n int) ([]DataPoint, error) {
	var datapoints []DataPoint
	if err := db.
		Table("data_points").
		Where("room = ? AND device = ? AND parameter = ?", room, device, parameter).
		Order("created_at desc").
		Limit(n).
		Find(&datapoints).
		Error; err != nil {
		return nil, fmt.Errorf("error getting last datapoints by parameter: %w", err)
	}
	return datapoints, nil
}

// IsWaterLevelFull checks if a water level value corresponds to "full"
func IsWaterLevelFull(value float64) bool {
	return int(value) == int(WaterLevelFull)
}

// Legacy compatibility function to support existing code
// This will be removed after the transition is complete
func (db *DB) LastN(n int) ([]Parameters, error) {
	var last []Parameters
	if err := db.
		Table("parameters").
		Order("created_at desc").
		Limit(n).
		Find(&last).
		Error; err != nil {
		return nil, fmt.Errorf("error getting last parameters: %w", err)
	}
	return last, nil
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
					if err := db.updateNewAggregate(room, device, param, time.Hour*24, dayStart, value); err != nil {
						return fmt.Errorf("error updating daily aggregate for %s/%s/%s: %w", room, device, param, err)
					}

					// Hourly aggregates
					if err := db.updateNewAggregate(room, device, param, time.Hour, hourStart, value); err != nil {
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

// updateNewAggregate updates the aggregate for a given room/device/parameter
func (db *DB) updateNewAggregate(room, device, parameter string, windowDuration time.Duration, windowStartAt time.Time, value float64) error {
	agg, err := db.getNewAggregate(room, device, parameter, windowDuration, windowStartAt)
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

// getNewAggregate retrieves an aggregate or returns a new one if not found
func (db *DB) getNewAggregate(room, device, parameter string, windowDuration time.Duration, windowStartAt time.Time) (*WindowAggregate, error) {
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

// WindowAggregate is the new aggregate structure that matches the DataPoint structure
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

// Legacy aggregate structure (kept for migration)
type Aggregate struct {
	Parameter      AggregateID   `gorm:"primaryKey;column:parameter"`
	WindowDuration time.Duration `gorm:"primaryKey;column:window_duration"`
	WindowStartAt  time.Time     `gorm:"primaryKey;column:window_start_at"`

	Min   float64
	Max   float64
	Mean  float64
	Count int64
}

type AggregateID int

const (
	aggregateIDUnknown AggregateID = iota
	AggregateIDTemperature
	AggregateIDHumidity
	AggregateIDDust
	AggregateIDWaterLevel

	// New Aranet-specific aggregates
	AggregateIDOfficeTemperature
	AggregateIDOfficeHumidity
	AggregateIDOfficeCO2
	AggregateIDOfficePressure

	AggregateIDLivingRoomTemperature
	AggregateIDLivingRoomHumidity
	AggregateIDLivingRoomCO2
	AggregateIDLivingRoomPressure
)

// Map to convert legacy AggregateID to new room, device, parameter format
var AggregateIDMapping = map[AggregateID]struct {
	Room      string
	Device    string
	Parameter string
}{
	AggregateIDTemperature:          {"living room", "60:8A:10:B5:58:A0", "temperature"},
	AggregateIDHumidity:             {"living room", "60:8A:10:B5:58:A0", "humidity"},
	AggregateIDDust:                 {"living room", "60:8A:10:B5:58:A0", "dust"},
	AggregateIDWaterLevel:           {"living room", "60:8A:10:B5:58:A0", "water_level"},

	AggregateIDOfficeTemperature:    {"office", "Aranet4 0AC6E", "temperature"},
	AggregateIDOfficeHumidity:       {"office", "Aranet4 0AC6E", "humidity"},
	AggregateIDOfficeCO2:            {"office", "Aranet4 0AC6E", "co2"},
	AggregateIDOfficePressure:       {"office", "Aranet4 0AC6E", "pressure"},

	AggregateIDLivingRoomTemperature: {"living room", "Aranet4 069F9", "temperature"},
	AggregateIDLivingRoomHumidity:    {"living room", "Aranet4 069F9", "humidity"},
	AggregateIDLivingRoomCO2:         {"living room", "Aranet4 069F9", "co2"},
	AggregateIDLivingRoomPressure:    {"living room", "Aranet4 069F9", "pressure"},
}

// DataPoint represents a single measurement from a sensor
type DataPoint struct {
	ID        uint      `gorm:"primarykey"`
	CreatedAt time.Time

	Room       string  // e.g., "living room", "office"
	Device     string  // e.g., "aranet4 069F9", "venta air purifier"
	Parameter  string  // e.g., "temperature", "humidity", "co2", "water_level", "fan_rpm", "battery"
	Value      float64 // all values stored as float64 for consistency
}

// Legacy struct kept for migration - will be removed after migration is complete
type Parameters struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time

	// Venta air purifier data
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
	Temperature float64  // Venta temperature
	Dust        int
	WaterLevel  WaterLevel
	Humidity    float64  // Venta humidity

	// Aranet Office sensor data
	OfficeTemperature    float64 // Fahrenheit
	OfficeHumidity       int
	OfficeCO2            int
	OfficePressure       float64
	OfficeBattery        int
	OfficeVersion        string
	OfficeAranetValid    bool    // Whether the data is valid

	// Aranet Living Room sensor data
	LivingRoomTemperature float64 // Fahrenheit
	LivingRoomHumidity    int
	LivingRoomCO2         int
	LivingRoomPressure    float64
	LivingRoomBattery     int
	LivingRoomVersion     string
	LivingRoomAranetValid bool    // Whether the data is valid
}

type WaterLevel int

const (
	WaterLevelError WaterLevel = 0
	WaterLevelFull  WaterLevel = 3
	WaterLevelLow   WaterLevel = 1
	WaterLevelEmpty WaterLevel = 2
)

func (wl WaterLevel) IsFull() bool {
	return wl == WaterLevelFull
}

// Room-to-devices mapping
var RoomDevices = map[string][]string{
	"living room": {"Aranet4 069F9", "60:8A:10:B5:58:A0"}, // Aranet and Venta
	"office":      {"Aranet4 0AC6E"},                      // Aranet
}

// DeviceToRoom returns the room for a given device
func DeviceToRoom(device string) (string, bool) {
	for room, devices := range RoomDevices {
		for _, d := range devices {
			if d == device {
				return room, true
			}
		}
	}
	return "", false
}
