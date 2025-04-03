package main

import (
	"fmt"
	"net/http"

	"tinygo.org/x/bluetooth"

	"monks.co/pkg/aranet4"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/ports"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

var (
	adapter     = bluetooth.DefaultAdapter
	deviceCount = 2
)

func run() error {
	port := ports.Apps["aranet"]

	mux := serve.NewMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, req *http.Request) {
		devices, err := aranet4.GetDevices(deviceCount)
		if err != nil {
			serve.InternalServerError(w, req, err)
			return
		}

		// Print to console for logging
		for _, dev := range devices {
			fmt.Println(dev)
			fmt.Println()
		}
		
		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		serve.JSON(w, req, devices)
	})

	ctx := sigctx.New()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}
