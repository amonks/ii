package main

import (
	"context"
	cryptotls "crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	proxyproto "github.com/pires/go-proxyproto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"monks.co/pkg/config"
	"monks.co/pkg/meta"
	"monks.co/pkg/middleware"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
	"monks.co/pkg/tls"
)

var _ = flag.String("machine", "", "deprecated: ignored")

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err.Error(), "app.name", meta.AppName())
		reqlog.Shutdown()
		os.Exit(1)
	}
}

var (
	labels         = []string{"host", "app", "path", "status_code", "user_agent"}
	requestsMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "requests",
		},
		labels,
	)
	requestDurationsMetric = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "request_durations",
			MaxAge:     time.Hour,
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		labels,
	)
)

func run() error {
	flag.Parse()

	config, err := config.LoadProxy()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	var wg sync.WaitGroup
	ctx, cancel := sigctx.NewWithCancel()

	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}

	reqlog.SetupLogging()

	reg := prometheus.NewRegistry()
	reg.MustRegister(requestsMetric, requestDurationsMetric)

	for _, serviceConfig := range config.Services {
		wg.Add(1)
		serviceConfig := serviceConfig
		go func() {
			defer wg.Done()
			service := &Service{
				service:   serviceConfig,
				acme:      config.ACME,
				redirects: config.Redirects,
			}

			slog.Info("started", "addr", serviceConfig.Addr)
			if err := service.ListenAndServe(ctx); err != nil {
				slog.Error("fatal", "detail", "service failed", "addr", service.service.Addr, "error", err)
				cancel(err)
			}
		}()
	}

	wg.Go(func() {
		mux := http.NewServeMux()
		mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		if err := serve.ListenAndServe(ctx, "0.0.0.0:9999", mux); err != nil {
			slog.Error("fatal", "detail", "metrics server failed", "error", err)
			cancel(err)
		}
	})

	wg.Wait()
	return nil
}

type Service struct {
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
	default:
		return fmt.Errorf("unsupported service type: '%s'", s.service.Type)
	}
}

func (s *Service) listenAndServeRedirects(ctx context.Context) error {
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		u := req.URL
		u.Host = net.JoinHostPort(req.Host, "443")
		u.Scheme = "https"
		http.Redirect(w, req, u.String(), http.StatusMovedPermanently)
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

	tsClient := tailnet.Client()

	tsLn, err := tailnet.Listen("tcp", ":443")
	if err != nil {
		return fmt.Errorf("tsnet listen: %w", err)
	}
	defer tsLn.Close()

	anonCaps, anonCapsErr := tailnet.AnonCaps(ctx)
	if anonCapsErr != nil {
		slog.Info("started", "addr", s.service.Addr, "tailauth.error", anonCapsErr.Error())
	} else {
		slog.Info("started", "addr", s.service.Addr, "tailauth.caps", capNames(anonCaps))
	}

	vanityMods, defaultMirror, err := loadVanityModules()
	if err != nil {
		slog.Warn("vanity import handler disabled", "error", err)
	}

	p := &proxy{
		rewrites:  s.service.Rewrites,
		transport: tsClient.Transport,
		vanity:    vanityHandler(vanityMods, defaultMirror),
	}

	// Public handler: reqlog → anon caps → redirector → proxy
	publicMW := middleware.Combine(reqlog.Middleware(), anonCapsMiddleware{anonCaps}, RedirectorMiddleware(s.redirects))
	publicHandler := publicMW.ModifyHandler(p)

	// Tailnet handler: reqlog → tailscale auth → proxy
	tailnetMW := middleware.Combine(reqlog.Middleware(), tailscaleAuthMiddleware{})
	tailnetHandler := tailnetMW.ModifyHandler(p)

	// Public listener (with ProxyProto)
	publicSrv := &http.Server{
		ConnContext: deriveConnectionContext,
		Addr:        s.service.Addr,
		Handler:     publicHandler,
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

	errs := make(chan error, 2)
	go func() {
		errs <- publicSrv.ServeTLS(proxyListener, "", "")
	}()

	// Tailnet listener (tsnet with TLS)
	go func() {

		tlsLn := cryptotls.NewListener(tsLn, tlsConfig)
		defer tlsLn.Close()

		tailnetSrv := &http.Server{
			Handler: tailnetHandler,
			ConnContext: func(ctx context.Context, conn net.Conn) context.Context {
				return context.WithValue(ctx, reqlog.RemoteAddrKey, conn.RemoteAddr().String())
			},
		}
		errs <- tailnetSrv.Serve(tlsLn)
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return publicSrv.Shutdown(context.Background())
	}
}

func deriveConnectionContext(ctx context.Context, conn net.Conn) context.Context {
	if conn, ok := conn.(*proxyproto.Conn); ok {
		return context.WithValue(ctx, reqlog.RemoteAddrKey, conn.RemoteAddr().String())
	}
	return context.WithValue(ctx, reqlog.RemoteAddrKey, conn.RemoteAddr().String())
}
