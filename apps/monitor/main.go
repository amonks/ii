package main

import (
	"fmt"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
	"monks.co/apps/monitor/monitor"
	"monks.co/pkg/gzip"
	"monks.co/pkg/ports"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

var reporter = monitor.Reporter{
	"2cf481a128": monitor.NewHTTPMonitor("https://monks.co", monitor.WithRegexpCheck()),
}

func run() error {
	port := ports.Apps["monitor"]

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("ok"))
	})

	ctx, cancel := sigctx.NewWithCancel()
	wg := new(errgroup.Group)

	wg.Go(func() error {
		if err := reporter.Run(ctx, time.Minute); err != nil {
			cancel(err)
			return err
		}
		return nil
	})

	wg.Go(func() error {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
			cancel(err)
			return err
		}
		return nil
	})

	return wg.Wait()
}

