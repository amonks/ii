package tls

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/route53"
)

type ACME struct {
	OnDemand    bool    `toml:"on_demand"`
	StoragePath *string `toml:"storage_path"`
	Strategies  []ACMEStrategy
	Domains     []string
	Production  bool
}

type ACMEStrategy struct {
	Strategy     string
	ExternalPort int `toml:"external_port"`
	InternalPort int `toml:"internal_port"`
}

func NewTLSConfig(ctx context.Context, acmeConfig ACME) (*tls.Config, func(), error) {
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

	if acmeConfig.OnDemand {
		config.OnDemand = &certmagic.OnDemandConfig{
			DecisionFunc: func(ctx context.Context, name string) error {
				for _, domain := range acmeConfig.Domains {
					if strings.HasSuffix(name, domain) {
						return nil
					}
				}
				return fmt.Errorf("unknown domain %s", name)
			},
		}
	}

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
				DNSManager: certmagic.DNSManager{
					DNSProvider: &route53.Provider{
						Region:             "us-east-1",
						WaitForPropagation: true,
					},
					PropagationTimeout: -1,
				},
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

	var domains []string
	for _, domain := range acmeConfig.Domains {
		domains = append(domains, domain)

		// If this is a second-level domain, also get a wildcard
		// certificate for `www.`
		// XXX: counting periods fails here for, eg, .co.uk.
		if len(strings.Split(domain, ".")) == 2 {
			domains = append(domains, "*."+domain)
		}
	}

	config.OnDemand = &certmagic.OnDemandConfig{}

	err := config.ManageSync(ctx, domains)
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
