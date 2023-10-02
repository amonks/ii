package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"monks.co/config"
	"monks.co/confrunner"
	"monks.co/tls"
)

func main() {
	ctx, cancel := context.WithCancelCause(context.Background())
	var wg sync.WaitGroup

	interrupt := make(chan os.Signal)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-interrupt
		cancel(context.Canceled)
	}()

	mux := http.NewServeMux()
	for _, a := range config.Get().Apps {
		h := confrunner.NewServer(a)

		if err := h.Start(ctx); err != nil {
			cancel(err)
			break
		}

		mux.Handle(a.Path, h)

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ctx.Done()
			if err := h.Stop(); err != nil {
				fmt.Println("shutdown error:", err)
			}
		}()
	}

	for _, s := range config.Get().Services {
		if ctx.Err() != nil {
			break
		}

		service := s

		switch service.Protocol {
		case "http":
			addr := fmt.Sprintf(":%d", service.Port)
			srv := &http.Server{Addr: addr, Handler: mux}

			wg.Add(1)
			go func() {
				defer wg.Done()

				fmt.Println("listening for HTTP requests on " + addr)
				if err := srv.ListenAndServe(); err != http.ErrServerClosed {
					cancel(err)
				}
				fmt.Println("stopped listening for HTTP requests on " + addr)
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				<-ctx.Done()
				// TODO: context.WithoutCancel(ctx)
				if err := srv.Shutdown(context.TODO()); err != nil {
					fmt.Println("shutdown error:", err)
				}
			}()

		case "https":
			if service.ACME == nil {
				cancel(fmt.Errorf("no ACME config"))
				break
			}
			addr := fmt.Sprintf(":%d", service.Port)

			cfg, stop, err := tls.NewTLSConfig(ctx, *service.ACME)
			if err != nil {
				cancel(err)
				break
			}

			srv := &http.Server{
				Addr:        addr,
				Handler:     mux,
				TLSConfig:   cfg,
				BaseContext: func(net.Listener) context.Context { return ctx },
			}

			wg.Add(1)
			go func() {
				defer wg.Done()

				fmt.Println("listening for HTTPS requests on " + addr)
				if err := srv.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
					cancel(err)
				}
				fmt.Println("stopped listening for HTTPS requests on " + addr)
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				<-ctx.Done()
				// TODO: context.WithoutCancel(ctx)
				if err := srv.Shutdown(context.TODO()); err != nil {
					fmt.Println("shutdown error:", err)
				}
				stop()
			}()

		default:
			cancel(fmt.Errorf("protocol not supported: %s", service.Protocol))
			break
		}
	}

	wg.Wait()
	fmt.Println(context.Cause(ctx))
}
