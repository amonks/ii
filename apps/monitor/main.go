package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
	"monks.co/apps/monitor/monitor"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/ports"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
)

func main() {
	if err := run(); err != nil && !errors.Is(err, context.Canceled) {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

var reporter = monitor.Reporter{
	"7135661159": monitor.NewHTTPMonitor("https://belgianman.com", monitor.WithRedirectCheck("https://belgianman.bandcamp.com/")),
	"cf01c434ed": monitor.NewHTTPMonitor("https://blgn.mn", monitor.WithRedirectCheck("https://belgianman.bandcamp.com/")),

	"10e94a97f5": monitor.NewHTTPMonitor("https://andrewmonks.org", monitor.WithRedirectCheck("https://monks.co/")),
	"15e8dd93d3": monitor.NewHTTPMonitor("https://lyrics.gy", monitor.WithRedirectCheck("https://monks.co/")),
	"1701700ff4": monitor.NewHTTPMonitor("https://fuckedcars.com", monitor.WithRedirectCheck("https://monks.co/")),
	"1e4c151574": monitor.NewHTTPMonitor("https://popefucker.com", monitor.WithRedirectCheck("https://monks.co/")),
	"230cd6a1c7": monitor.NewHTTPMonitor("https://docrimes.com", monitor.WithRedirectCheck("https://monks.co/")),
	"2e2a54d014": monitor.NewHTTPMonitor("https://fmail.email", monitor.WithRedirectCheck("https://monks.co/")),
	"927132497c": monitor.NewHTTPMonitor("https://andrewmonks.net", monitor.WithRedirectCheck("https://monks.co/")),
	"a24120b740": monitor.NewHTTPMonitor("https://needsyourhelp.org", monitor.WithRedirectCheck("https://monks.co/")),
	"cf89105615": monitor.NewHTTPMonitor("https://andrewmonks.com", monitor.WithRedirectCheck("https://monks.co/")),
	"e2b6c2c6d3": monitor.NewHTTPMonitor("https://amonks.co", monitor.WithRedirectCheck("https://monks.co/")),
	"e34a530a1c": monitor.NewHTTPMonitor("https://ss.cx", monitor.WithRedirectCheck("https://monks.co/")),

	"2cf481a128": monitor.NewHTTPMonitor("https://monks.co", monitor.WithBodyCheck(
		monitor.LiteralCheck(`I watch movies most days.`),
	)),
	"3927514be4": monitor.NewHTTPMonitor("https://piano.computer", monitor.WithBodyCheck(
		monitor.LiteralCheck(`6 pianists`),
	)),
}

func run() error {
	port := ports.Apps["monitor"]

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "https://deadmanssnitch.com/cases/20c59c12-7c79-443a-9bb1-b9feb56c3159/snitches", 301)
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
