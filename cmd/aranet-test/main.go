package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"tinygo.org/x/bluetooth"
)

func main() {
	timeout := 30 * time.Second
	if len(os.Args) > 1 {
		d, err := time.ParseDuration(os.Args[1])
		if err == nil {
			timeout = d
		}
	}

	fmt.Printf("Testing Bluetooth scanning for %v...\n\n", timeout)

	adapter := bluetooth.DefaultAdapter
	fmt.Println("Enabling adapter...")
	if err := adapter.Enable(); err != nil {
		log.Fatalf("Failed to enable adapter: %v", err)
	}
	fmt.Println("Adapter enabled.\n")

	found := make(map[string]string) // addr -> name
	done := make(chan struct{})

	go func() {
		fmt.Println("Starting scan...")
		err := adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
			addr := result.Address.String()
			name := result.LocalName()

			// Only print each device once
			if _, seen := found[addr]; !seen {
				found[addr] = name
				isAranet := strings.HasPrefix(name, "Aranet4")
				marker := ""
				if isAranet {
					marker = " <-- ARANET4"
				}
				fmt.Printf("Found: %-20s %s%s\n", name, addr, marker)
			}
		})
		if err != nil {
			log.Printf("Scan error: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("\nScan finished")
	case <-time.After(timeout):
		fmt.Println("\nTimeout reached, stopping scan...")
		adapter.StopScan()
	}

	fmt.Printf("\nTotal devices found: %d\n", len(found))

	aranetCount := 0
	for _, name := range found {
		if strings.HasPrefix(name, "Aranet4") {
			aranetCount++
		}
	}
	fmt.Printf("Aranet4 devices found: %d\n", aranetCount)
}
