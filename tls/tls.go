package tls

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/route53"
	"monks.co/config"
)

func NewTLSConfig(ctx context.Context, acmeConfig config.ACME) (*tls.Config, func(), error) {
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
	baseConfig := certmagic.Config{}
	if acmeConfig.StoragePath != nil {
		baseConfig.Storage = &certmagic.FileStorage{
			Path: *acmeConfig.StoragePath,
		}
	}
	config = certmagic.New(cache, baseConfig)

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
