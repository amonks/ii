package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	proxyproto "github.com/pires/go-proxyproto"

	"monks.co/pkg/config"
	"monks.co/pkg/middleware"
	"monks.co/pkg/ports"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tls"
	"monks.co/pkg/traffic"
	"tailscale.com/tsnet"
)

var machine = flag.String("machine", "", "machine name; must have a corresponding toml file in config/.")

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	flag.Parse()

	config, err := config.Load(*machine)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	var wg sync.WaitGroup
	ctx, cancel := sigctx.NewWithCancel()

	for _, serviceConfig := range config.Services {
		wg.Add(1)
		serviceConfig := serviceConfig
		go func() {
			defer wg.Done()
			routes := map[string]int{}
			for _, app := range serviceConfig.Apps {
				routes[app] = ports.Apps[app]
			}
			service := &Service{
				routes:    routes,
				service:   serviceConfig,
				acme:      config.ACME,
				redirects: config.Redirects,
			}

			log.Printf("listening at %s", serviceConfig.Addr)
			if err := service.ListenAndServe(ctx); err != nil {
				fmt.Println(err)
				log.Printf("service at '%s' failed; canceling run", service.service.Addr)
				cancel(err)
			}
		}()
	}

	wg.Wait()
	return nil
}

type Service struct {
	routes    map[string]int
	service   config.Service
	acme      tls.ACME
	redirects map[string]string
}

func (s *Service) ListenAndServe(ctx context.Context) error {
	switch s.service.Type {
	case "redirect-to-https":
		return s.listenAndServeRedirects(ctx)
	case "https":
		return s.listenAndServeHTTPS(ctx)
	case "tsnet":
		return s.listenAndServeTSNet(ctx)
	default:
		return fmt.Errorf("unsupported service type: '%s'", s.service.Type)
	}
}

func (s *Service) listenAndServeRedirects(ctx context.Context) error {
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		u := req.URL
		u.Host = net.JoinHostPort(req.Host, "443")
		u.Scheme = "https"
		http.Redirect(w, req, u.String(), 301)
	})
	srv := &http.Server{
		ConnContext: deriveConnectionContext,
		Addr:        s.service.Addr,
		Handler:     handler,
	}

	ln, err := net.Listen("tcp", s.service.Addr)
	if err != nil {
		return err
	}

	proxyListener := &proxyproto.Listener{
		Listener:          ln,
		ReadHeaderTimeout: 10 * time.Second,
	}
	defer proxyListener.Close()

	errs := make(chan error)
	go func() {
		errs <- srv.Serve(proxyListener)
	}()
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return srv.Shutdown(context.Background())
	}
}

func (s *Service) listenAndServeHTTPS(ctx context.Context) error {
	tlsConfig, stopTLS, err := tls.NewTLSConfig(ctx, s.acme)
	if err != nil {
		return fmt.Errorf("error creating tls config: %w", err)
	}
	defer stopTLS()

	traf, err := traffic.New(s.service.Addr)
	if err != nil {
		return fmt.Errorf("error starting traffic logger: %w", err)
	}
	defer traf.Close()

	mw := middleware.Combine(RedirectorMiddleware(s.redirects), traf)
	handler := mw.ModifyHandler(&proxy{s.routes})
	srv := &http.Server{
		ConnContext: deriveConnectionContext,
		Addr:        s.service.Addr,
		Handler:     handler,
		TLSConfig:   tlsConfig,
	}

	ln, err := net.Listen("tcp", s.service.Addr)
	if err != nil {
		return err
	}

	proxyListener := &proxyproto.Listener{
		Listener:          ln,
		ReadHeaderTimeout: 10 * time.Second,
	}
	defer proxyListener.Close()

	errs := make(chan error)
	go func() {
		errs <- srv.ServeTLS(proxyListener, "", "")
	}()
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return srv.Shutdown(context.Background())
	}
}

func (s *Service) listenAndServeTSNet(ctx context.Context) error {
	traf, err := traffic.New(s.service.Addr)
	if err != nil {
		return fmt.Errorf("error starting traffic logger: %w", err)
	}
	defer traf.Close()

	mw := middleware.Combine(RedirectorMiddleware(s.redirects), traf)
	handler := mw.ModifyHandler(&proxy{s.routes})
	httpSrv := &http.Server{
		ConnContext: deriveConnectionContext,
		Addr:        s.service.Addr,
		Handler:     handler,
	}

	tsSrv := &tsnet.Server{
		Hostname:  "monksgo",
		Dir:       s.service.StoragePath,
		Ephemeral: true,
		AuthKey:   os.Getenv("TS_AUTHKEY"),
	}

	ln, err := tsSrv.Listen("tcp", ":80")
	if err != nil {
		return fmt.Errorf("error listening on tsnet: %w", err)
	}

	proxyListener := &proxyproto.Listener{
		Listener:          ln,
		ReadHeaderTimeout: 10 * time.Second,
	}
	defer proxyListener.Close()

	errs := make(chan error)
	go func() {
		errs <- httpSrv.Serve(proxyListener)
	}()
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return httpSrv.Shutdown(context.Background())
	}
}

func deriveConnectionContext(ctx context.Context, conn net.Conn) context.Context {
	if conn, ok := conn.(*proxyproto.Conn); ok {
		if conn.LocalAddr() == nil {
			fmt.Printf("couldn't retrieve local address")
		}
		if conn.RemoteAddr() == nil {
			fmt.Printf("couldn't retrieve remote address")
		}

		return context.WithValue(ctx, traffic.RemoteAddrKey, conn.RemoteAddr().String())
	}
	return context.WithValue(ctx, traffic.RemoteAddrKey, conn.RemoteAddr().String())
}
