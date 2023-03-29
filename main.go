package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/route53"
	"monks.co/auth"
	"monks.co/config"
	"monks.co/golink"
	"monks.co/ping"
	"monks.co/places"
	"monks.co/promises"
	"monks.co/weblog"
)

func main() {
	mux := http.NewServeMux()

	mux.Handle("/promises/", auth.InternalHandler(promises.Server()))
	mux.Handle("/ping/", auth.InternalHandler(ping.Server()))
	mux.Handle("/go/", auth.InternalHandler(golink.Server()))

	mux.Handle("/places/", places.Server())

	mux.Handle("/", weblog.Server())

	ctx, cancel := context.WithCancelCause(context.Background())
	var wg sync.WaitGroup

	for _, s := range config.Get().Services {
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

			go cancelWhenDone(ctx, srv)

		case "https":
			if service.ACME == nil {
				cancel(fmt.Errorf("no ACME config"))
				break
			}
			addr := fmt.Sprintf(":%d", service.Port)

			cfg, stop, err := newTLSConfig(ctx, *service.ACME)
			if err != nil {
				cancel(err)
				break
			}

			srv := &http.Server{
				Addr:      addr,
				Handler:   mux,
				TLSConfig: cfg,
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

			go cancelWhenDone(ctx, srv, stop)

		default:
			cancel(fmt.Errorf("protocol not supported: %s", service.Protocol))
		}
	}

	interrupt := make(chan os.Signal)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-interrupt
		cancel(context.Canceled)
	}()

	wg.Wait()
	fmt.Println(context.Cause(ctx))
}

func newTLSConfig(ctx context.Context, acmeConfig config.ACME) (*tls.Config, func(), error) {
	if len(acmeConfig.Strategies) == 0 {
		return nil, nil, fmt.Errorf("error: no ACME strategies")
	}

	var config *certmagic.Config

	cache := certmagic.NewCache(certmagic.CacheOptions{
		GetConfigForCert: func(cert certmagic.Certificate) (*certmagic.Config, error) {
			return config, nil
		},
	})

	// create config
	config = certmagic.New(cache, certmagic.Config{})

	ca := certmagic.LetsEncryptStagingCA
	if acmeConfig.Production {
		ca = certmagic.LetsEncryptProductionCA
	}
	acmeIssuerConfig := certmagic.ACMEIssuer{
		CA:                      ca,
		Email:                   "a@monks.co",
		Agreed:                  true,
		DisableHTTPChallenge:    true,
		DisableTLSALPNChallenge: true,
	}
	for _, strategy := range acmeConfig.Strategies {
		switch strategy.Strategy {
		case "dns":
			acmeIssuerConfig.DNS01Solver = &certmagic.DNS01Solver{
				DNSProvider: &route53.Provider{},
			}

		case "alpn":
			if strategy.ExternalPort != certmagic.TLSALPNChallengePort {
				cache.Stop()
				return nil, nil, fmt.Errorf("bad external port for TLS ALPN ACME challenge: %d", strategy.ExternalPort)
			}
			acmeIssuerConfig.DisableTLSALPNChallenge = false
			acmeIssuerConfig.AltTLSALPNPort = strategy.InternalPort

		case "http":
			if strategy.ExternalPort != certmagic.HTTPChallengePort {
				cache.Stop()
				return nil, nil, fmt.Errorf("bad external port for TLS HTTP challenge: %d", strategy.ExternalPort)
			}
			acmeIssuerConfig.DisableHTTPChallenge = false
			acmeIssuerConfig.AltHTTPPort = strategy.InternalPort

		default:
			return nil, nil, fmt.Errorf("unsupported ACME strategy: '%s'", strategy.Strategy)
		}
	}

	acmeIssuer := certmagic.NewACMEIssuer(config, acmeIssuerConfig)
	config.Issuers = []certmagic.Issuer{acmeIssuer}
	// config.Issuers = append(config.Issuers, acmeIssuer)

	err := config.ManageSync(ctx, acmeConfig.Domains)
	if err != nil {
		cache.Stop()
		return nil, nil, err
	}

	tlsConfig := config.TLSConfig()

	// be sure to customize NextProtos if serving a specific
	// application protocol after the TLS handshake, for example:
	tlsConfig.NextProtos = append([]string{"h2", "http/1.1"}, tlsConfig.NextProtos...)

	return tlsConfig, cache.Stop, nil
}

func cancelWhenDone(ctx context.Context, srv *http.Server, callbacks ...func()) {
	<-ctx.Done()
	if err := srv.Shutdown(context.Background()); err != nil {
		fmt.Println("shutdown error:", err)
	}
	for _, cb := range callbacks {
		cb()
	}
}
