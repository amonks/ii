package aranet4

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

func GetDevices(count int, timeout time.Duration) ([]*Device, error) {
	adapter.Enable()
	results := map[string]bluetooth.ScanResult{}
	scanComplete := make(chan struct{})
	scanError := make(chan error, 1)

	// Set up the scan with a timeout
	go func() {
		err := adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
			if strings.HasPrefix(result.LocalName(), "Aranet4") {
				prevCount := len(results)
				results[result.Address.String()] = result
				if len(results) > prevCount {
					log.Printf("found device: %s %s", result.LocalName(), result.Address.String())
				}
				if len(results) >= count {
					adapter.StopScan()
					close(scanComplete)
				}
			}
		})
		if err != nil {
			scanError <- err
		}
	}()

	// Wait for either the scan to complete, an error, or a timeout
	select {
	case <-scanComplete:
		// Scan completed by finding all requested devices
	case err := <-scanError:
		return nil, err
	case <-time.After(timeout):
		// Timeout reached, stop the scan
		adapter.StopScan()
		log.Printf("Scan timeout reached after %v. Found %d/%d devices", timeout, len(results), count)
	}

	// No devices found at all
	if len(results) == 0 {
		return nil, fmt.Errorf("no Aranet4 devices found within timeout (%v)", timeout)
	}

	var devices []*Device
	for _, result := range results {
		var bs []byte
		for _, mfd := range result.AdvertisementPayload.ManufacturerData() {
			if mfd.CompanyID == 0x0702 {
				bs = mfd.Data
			}
		}
		if bs == nil {
			log.Printf("warning: device %s has no Aranet data", result.Address.String())
			continue
		}
		log.Printf("%d bytes", len(bs))
		rawData := readRawData(bs)
		device, err := convertToDevice(rawData, result.Address.String(), result.LocalName())
		if err != nil {
			log.Printf("warning: error converting device %s: %v", result.Address.String(), err)
			continue
		}
		devices = append(devices, device)
	}

	// If we didn't find any valid devices with data, return an error
	if len(devices) == 0 {
		return nil, fmt.Errorf("no valid Aranet4 devices found within timeout (%v)", timeout)
	}

	return devices, nil
}

func readRawData(bs []byte) *Data {
	log.Printf("% x", bs)
	var data Data
	buf := bytes.NewReader(bs)
	if len(bs) != 22 {
		panic("bad bytes")
	}
	must(binary.Read(buf, binary.LittleEndian, &data.flags))
	must(binary.Read(buf, binary.LittleEndian, &data.version))
	must(binary.Read(buf, binary.LittleEndian, &data.pad))
	must(binary.Read(buf, binary.LittleEndian, &data.co2))
	must(binary.Read(buf, binary.LittleEndian, &data.temperature))
	must(binary.Read(buf, binary.LittleEndian, &data.pressure))
	must(binary.Read(buf, binary.LittleEndian, &data.humidity))
	must(binary.Read(buf, binary.LittleEndian, &data.battery))
	must(binary.Read(buf, binary.LittleEndian, &data.status))
	must(binary.Read(buf, binary.LittleEndian, &data.interval))
	must(binary.Read(buf, binary.LittleEndian, &data.ago))
	return &data
}

func convertToDevice(data *Data, addr, name string) (*Device, error) {
	// Create new device with eagerly interpreted values
	device := &Device{
		Address:  addr,
		Name:     name,
		Flags:    data.flags,
		Version:  data.version,
		Battery:  data.battery,
		Status:   data.status,
		Interval: data.interval,
		Ago:      data.ago,
	}

	// Parse CO2
	co2Val, err := data.CO2()
	if err == nil {
		device.CO2 = co2Val
		device.CO2Valid = true
	}

	// Parse Temperature
	tempC, err := data.Temperature()
	if err == nil {
		// Round to 1 decimal place to avoid floating point precision issues
		device.TemperatureC = math.Round(tempC*10) / 10
		device.TemperatureF = math.Round((tempC*(float64(9)/float64(5))+float64(32))*10) / 10
		device.TemperatureValid = true
	}

	// Parse Pressure
	pressure, err := data.Pressure()
	if err == nil {
		// Round to 1 decimal place to avoid floating point precision issues
		device.Pressure = math.Round(pressure*10) / 10
		device.PressureValid = true
	}

	// Parse Humidity
	humidity, err := data.Humidity()
	if err == nil {
		device.Humidity = humidity
		device.HumidityValid = true
	}

	return device, nil
}

// Data is the raw binary structure directly marshaled from bluetooth data
type Data struct {
	addr        string
	name        string
	flags       byte
	version     Version
	pad         [4]byte
	co2         uint16
	temperature uint16
	pressure    uint16
	humidity    uint8
	battery     uint8
	status      uint8
	interval    uint16
	ago         uint16
}

// Device contains the interpreted data from an Aranet4 device
type Device struct {
	Name    string  `json:"name"`
	Address string  `json:"address"`
	Version Version `json:"version"`
	Flags   byte    `json:"flags"`

	CO2Valid bool `json:"co2_valid"`
	CO2      int  `json:"co2,omitempty"`

	TemperatureValid bool    `json:"temperature_valid"`
	TemperatureC     float64 `json:"temperature_c,omitempty"`
	TemperatureF     float64 `json:"temperature_f,omitempty"`

	HumidityValid bool `json:"humidity_valid"`
	Humidity      int  `json:"humidity,omitempty"`

	PressureValid bool    `json:"pressure_valid"`
	Pressure      float64 `json:"pressure,omitempty"`

	Battery  uint8  `json:"battery"`
	Status   uint8  `json:"status"`
	Interval uint16 `json:"interval"`
	Ago      uint16 `json:"ago"`
}

func (d *Data) String() string {
	// This function remains for backward compatibility
	device, _ := convertToDevice(d, d.addr, d.name)
	return device.String()
}

func (d *Device) String() string {
	// CO2
	co2Str := "invalid"
	if d.CO2Valid {
		co2Str = fmt.Sprintf("%d ppm", d.CO2)
	}

	// Temperature
	tempStrC := "invalid"
	tempStrF := "invalid"
	if d.TemperatureValid {
		tempStrC = fmt.Sprintf("%.1f °C", d.TemperatureC)
		tempStrF = fmt.Sprintf("%.1f °F", d.TemperatureF)
	}

	// Pressure
	pressureStr := "invalid"
	if d.PressureValid {
		pressureStr = fmt.Sprintf("%.1f hPa", d.Pressure)
	}

	// Humidity
	humStr := "invalid"
	if d.HumidityValid {
		humStr = fmt.Sprintf("%d %%", d.Humidity)
	}

	// Age is represented as "ago/interval s"
	ageStr := fmt.Sprintf("%d/%d s", d.Ago, d.Interval)

	// Format the output with spacing similar to the sample.
	return fmt.Sprintf(""+
		"  Name:           %s\n"+
		"  Address:        %s\n"+
		"  Version:        %s\n"+
		"-------------------------------------------\n"+
		"  CO2:            %s\n"+
		"  Temperature:    %s\n"+
		"  Temperature:    %s\n"+
		"  Humidity:       %s\n"+
		"  Pressure:       %s\n"+
		"  Battery:        %d %%\n"+
		"  Age:            %s",
		d.Name,
		d.Address,
		d.Version,
		co2Str,
		tempStrC,
		tempStrF,
		humStr,
		pressureStr,
		d.Battery,
		ageStr,
	)
}

// Flags returns the flags byte.
func (d *Data) Flags() byte {
	return d.flags
}

// Version returns the Version.
func (d *Data) Version() Version {
	return d.version
}

// Pad returns the pad bytes.
func (d *Data) Pad() [4]byte {
	return d.pad
}

// CO2 returns the CO2 reading. If the most significant bit is set,
// it indicates an invalid reading.
func (d *Data) CO2() (int, error) {
	if d.co2>>15 == 1 {
		return -1, fmt.Errorf("invalid CO2 reading")
	}
	// Multiplier is 1, so we simply return the value.
	return int(d.co2), nil
}

// Temperature returns the Temperature reading in celcius.
// It returns an error if the invalid flag (bit 14) is set.
func (d *Data) Temperature() (float64, error) {
	if (d.temperature>>14)&1 == 1 {
		return -1, fmt.Errorf("invalid Temperature reading")
	}
	return float64(d.temperature) * 0.05, nil
}

// TemperatureF returns the Temperature reading in Farenheight.
// It returns an error if the invalid flag (bit 14) is set.
func (d *Data) TemperatureF() (float64, error) {
	celc, err := d.Temperature()
	if err != nil {
		return 0, err
	}
	return celc*(float64(9)/float64(5)) + float64(32), nil
}

// Pressure returns the Pressure reading scaled by 0.1.
// It returns an error if the most significant bit is set.
func (d *Data) Pressure() (float64, error) {
	if d.pressure>>15 == 1 {
		return -1, fmt.Errorf("invalid Pressure reading")
	}
	return float64(d.pressure) * 0.1, nil
}

// Humidity returns the humidity reading.
// Following the Python logic, the reading is considered invalid
// if any bits beyond the lower 8 are set.
// Since the field is stored as a uint8, this check will always pass;
// if needed, change the field type to uint16.
func (d *Data) Humidity() (int, error) {
	value := uint16(d.humidity)
	if value>>8 != 0 {
		return -1, fmt.Errorf("invalid Humidity reading")
	}
	return int(value), nil
}

// Battery returns the battery level out of 100.
func (d *Data) Battery() uint8 {
	return d.battery
}

// Status returns the status byte.
func (d *Data) Status() uint8 {
	return d.status
}

// Interval returns the measurement interval in seconds.
func (d *Data) Interval() uint16 {
	return d.interval
}

// Ago returns the 'ago' value.
func (d *Data) Ago() uint16 {
	return d.ago
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

type Version [3]byte

func (v Version) String() string {
	return fmt.Sprintf(`%d.%d.%d`, v[2], v[1], v[0])
}

func (v Version) MarshalJSON() ([]byte, error) {
	return []byte("\"" + v.String() + "\""), nil
}
