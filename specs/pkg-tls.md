# TLS Package

## Overview

Wraps CertMagic to provision and manage ACME TLS certificates. Supports
multiple challenge strategies for Let's Encrypt certificate issuance.

Code: [pkg/tls/](../pkg/tls/)

## API

`NewTLSConfig(ctx, ACME) (*tls.Config, func(), error)` — returns a TLS
config with automatic certificate management, a cleanup function, and
any error.

## ACME Configuration

```go
type ACME struct {
    OnDemand    bool
    StoragePath string
    Domains     []string
    Strategies  []ACMEStrategy
    Production  bool
}

type ACMEStrategy struct {
    Strategy     string  // "dns-route53", "tls-alpn", "http"
    ExternalPort int
    InternalPort int
}
```

## Challenge Strategies

- **DNS-01** (`dns-route53`): Proves domain ownership via Route53 DNS
  records. Used for wildcard certificates.
- **TLS-ALPN**: Standard ACME TLS-ALPN-01 challenge on the TLS port.
- **HTTP**: Standard ACME HTTP-01 challenge.

## Dependencies

- `github.com/caddyserver/certmagic`
- `github.com/libdns/route53` (for DNS-01 challenges)
