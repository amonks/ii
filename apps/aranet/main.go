package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"monks.co/pkg/aranet4"
	"monks.co/pkg/gzip"
	"monks.co/pkg/meta"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
)

func main() {
	if err := run(); err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.Error("fatal", "error", err.Error(), "app.name", meta.AppName())
		}
		reqlog.Shutdown()
		os.Exit(1)
	}
}

var (
	deviceCount = 5
	scanTimeout = time.Minute
)

// DeviceData holds the latest device readings and their timestamp
type DeviceData struct {
	Devices   []*aranet4.Device
	Timestamp time.Time
	Error     error
	mu        sync.RWMutex
}

// Update atomically updates the device data
func (d *DeviceData) Update(devices []*aranet4.Device, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Devices = devices
	d.Timestamp = time.Now()
	d.Error = err
}

// Get atomically retrieves the device data
func (d *DeviceData) Get() ([]*aranet4.Device, time.Time, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Devices, d.Timestamp, d.Error
}

func run() error {
	reqlog.SetupLogging()

	// Create a shared data structure for the latest readings
	deviceData := &DeviceData{}

	// Start the Bluetooth scanning goroutine
	ctx := sigctx.New()
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		// Run the first scan immediately
		scanForDevices(deviceData)

		for {
			select {
			case <-ticker.C:
				scanForDevices(deviceData)
			case <-ctx.Done():
				return
			}
		}
	}()

	mux := serve.NewMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, req *http.Request) {
		devices, timestamp, err := deviceData.Get()

		if err != nil {
			serve.InternalServerError(w, req, err)
			return
		}

		if devices == nil {
			serve.InternalServerError(w, req, fmt.Errorf("no data available yet"))
			return
		}

		// Set Last-Modified header for caching/change detection
		w.Header().Set("Last-Modified", timestamp.Format(http.TimeFormat))
		w.Header().Set("Content-Type", "application/json")

		// Create response with timestamp and devices
		response := map[string]any{
			"timestamp": timestamp,
			"devices":   devices,
		}

		serve.JSON(w, req, response)
	})

	if err := tailnet.ListenAndServe(ctx, reqlog.Middleware().ModifyHandler(gzip.Middleware(mux))); err != nil {
		return err
	}

	return nil
}

// scanForDevices performs a Bluetooth scan and updates the shared device data
func scanForDevices(deviceData *DeviceData) {
	devices, err := aranet4.GetDevices(deviceCount, scanTimeout)

	if err != nil {
		// Even if we got an error, we might have found some devices
		if len(devices) > 0 {
			fmt.Printf("Warning: partial scan results (%d/%d devices): %v\n",
				len(devices), deviceCount, err)

			// Continue with the partial results
			for _, dev := range devices {
				fmt.Println(dev)
				fmt.Println()
			}

			// Update with partial results but no error
			deviceData.Update(devices, nil)
			return
		}

		// No devices found at all
		fmt.Printf("Error scanning for devices: %v\n", err)
		deviceData.Update(nil, err)
		return
	}

	// Print to console for logging
	for _, dev := range devices {
		fmt.Println(dev)
		fmt.Println()
	}

	deviceData.Update(devices, nil)
}
