package main

import (
	"context"
	cryptotls "crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	proxyproto "github.com/pires/go-proxyproto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"monks.co/pkg/config"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/middleware"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
	"monks.co/pkg/tls"
	"monks.co/pkg/trafficclient"
)

var machine = flag.String("machine", "", "machine name; must have a corresponding toml file in config/.")

func main() {
	if err := run(); err != nil {
		errlogger.ReportError(err)
		panic(err)
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

	config, err := config.Load(*machine)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	var wg sync.WaitGroup
	ctx, cancel := sigctx.NewWithCancel()

	reg := prometheus.NewRegistry()
	reg.MustRegister(requestsMetric, requestDurationsMetric)

	for _, serviceConfig := range config.Services {
		wg.Add(1)
		serviceConfig := serviceConfig
		go func() {
			defer wg.Done()
			routes := map[string]string{}
			for app, backend := range serviceConfig.Apps {
				routes[app] = backend
			}
			for path, port := range serviceConfig.ExtraRoutes {
				log.Printf("extra route %s %d", path, port)
				routes[path] = fmt.Sprintf("127.0.0.1:%d", port)
			}
			service := &Service{
				routes:    routes,
				service:   serviceConfig,
				acme:      config.ACME,
				redirects: config.Redirects,
			}

			log.Printf("listening at %s", serviceConfig.Addr)
			if err := service.ListenAndServe(ctx); err != nil {
				log.Println(err)
				log.Printf("service at '%s' failed; canceling run", service.service.Addr)
				cancel(err)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		mux := http.NewServeMux()
		mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		if err := serve.ListenAndServe(ctx, "0.0.0.0:9999", mux); err != nil {
			log.Println(err)
			cancel(err)
		}
	}()

	wg.Wait()
	return nil
}

type Service struct {
	routes    map[string]string
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
	traf := trafficclient.New("http://monks-traffic-fly-ord/log", tsClient)
	defer traf.Close()

	// Create tsnet listener first — Listen blocks until the server is
	// fully connected and has its netmap, which AnonCaps needs.
	tsLn, err := tailnet.Listen("tcp", ":443")
	if err != nil {
		return fmt.Errorf("tsnet listen: %w", err)
	}
	defer tsLn.Close()

	anonCaps, err := tailnet.AnonCaps(ctx)
	if err != nil {
		log.Printf("tailauth: failed to get anon caps: %v", err)
	}

	p := &proxy{s.routes, s.service.Rewrites, tsClient.Transport}

	// Public handler: anon caps → redirector → traffic → proxy
	publicMW := middleware.Combine(anonCapsMiddleware{anonCaps}, RedirectorMiddleware(s.redirects), traf)
	publicHandler := publicMW.ModifyHandler(p)

	// Tailnet handler: tailscale auth → traffic → proxy
	tailnetMW := middleware.Combine(tailscaleAuthMiddleware{}, traf)
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
				return context.WithValue(ctx, trafficclient.RemoteAddrKey, conn.RemoteAddr().String())
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
		if conn.LocalAddr() == nil {
			log.Printf("couldn't retrieve local address")
		}
		if conn.RemoteAddr() == nil {
			log.Printf("couldn't retrieve remote address")
		}

		return context.WithValue(ctx, trafficclient.RemoteAddrKey, conn.RemoteAddr().String())
	}
	return context.WithValue(ctx, trafficclient.RemoteAddrKey, conn.RemoteAddr().String())
}
