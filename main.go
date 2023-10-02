package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"monks.co/pkg/auth"
	"monks.co/pkg/config"
	"monks.co/pkg/service"
	"monks.co/pkg/tls"
)

func main() {
	ctx, cancel := context.WithCancelCause(context.Background())

	// We'll populate this WaitGroup with:
	//  - Service.Start() for each of [config.Current.Apps]
	//  - Listen() for each of [config.Service]
	//  - cancel handlers for some of the above (XXX: [why] is this necessary?)
	var wg sync.WaitGroup

	// Handle SIGTERM
	interrupt := make(chan os.Signal)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-interrupt
		cancel(context.Canceled)
	}()

	// Populate an http.ServeMux based on config.Apps.
	// Also start all the services in config.Apps.
	// Also set up context-cancel handlers for each service.
	mux := http.NewServeMux()
	for _, a := range config.Current.Apps {
		h := ApplyConfiguration(a, a.Service)

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

	// Using our http.ServeMux from before, start a listening http.Server
	// for each service in config.Services.
	for _, s := range config.Current.Services {
		if ctx.Err() != nil {
			break
		}

		service := s

		switch service.Protocol {
		case "http":
			addr := fmt.Sprintf(":%d", service.Port)
			srv := &http.Server{Addr: addr, Handler: mux}

			// Start the server.
			wg.Add(1)
			go func() {
				defer wg.Done()

				fmt.Println("listening for HTTP requests on " + addr)
				if err := srv.ListenAndServe(); err != http.ErrServerClosed {
					cancel(err)
				}
				fmt.Println("stopped listening for HTTP requests on " + addr)
			}()

			// Wait for cancel signal.
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

func ApplyConfiguration(c config.App, s service.Service) service.Service {
	// hack
	s.AddMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.Host, "belgianman.com") || strings.Contains(r.Host, "blgn.mn") {
				http.Redirect(w, r, "https://music.belgianman.com", http.StatusMovedPermanently)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	if !c.IsPublic {
		s.AddMiddleware(auth.InternalHandler)
	}

	return s
}
