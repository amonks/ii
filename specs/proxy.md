# Proxy

## Overview

Public-facing TLS-terminating reverse proxy for all of monks.co. Terminates
HTTPS, enforces Tailscale-based access control, and routes requests to
backend apps by path prefix. Also serves a large collection of static HTML
pages.

Code: [apps/proxy/](../apps/proxy/)

See also: [tailnet-routing](tailnet-routing.md) for the routing architecture.

## Startup

1. Parses `-machine` flag to load `config/<machine>.toml` via `pkg/config`.
2. Waits for Tailscale tsnet to be ready.
3. For each `[[service]]` in config, launches a goroutine:
   - `"redirect-to-https"`: listens on port 80, redirects to HTTPS.
   - `"https"`: listens on port 4433 (public, ProxyProto) and :443
     (tailnet), terminates TLS with ACME certs.
4. Runs a Prometheus metrics server on `:9999`.

## Dual-Listener Architecture

### Public Listener (port 443 via Fly ProxyProto)

Middleware: reqlog → `anonCapsMiddleware` → `RedirectorMiddleware` → proxy.

`anonCapsMiddleware` strips incoming `Tailscale-*` headers (anti-spoofing),
then injects capability headers for `autogroup:danger-all`.

### Tailnet Listener (tsnet :443)

Middleware: reqlog → `tailscaleAuthMiddleware` → proxy.

`tailscaleAuthMiddleware` calls `WhoIs` to identify the peer and sets
`Tailscale-User` and `Tailscale-Cap-*` headers from actual capabilities.

## Vanity Import Paths

Before route matching, the proxy checks if the request path matches a
public Go module declared in `config/publish.toml`. If so:

- `?go-get=1` requests get a `<meta name="go-import">` tag pointing at
  the GitHub mirror, enabling `go get monks.co/pkg/foo`.
- Human visitors get a redirect to the GitHub mirror.

For packages in the shared default mirror, the go-import prefix is
`monks.co` (the VCS root), so Go can find modules in subdirectories.
The root path `/?go-get=1` also serves this meta tag for Go's VCS root
verification. For packages with explicit mirrors, the go-import prefix
is the module path itself.

Paths are matched against the module list from `config/publish.toml`.
Both multi-segment (`/pkg/serve`, `/cmd/run/runner`) and single-segment
(`/run`, `/run/taskfile`) paths are matched if they correspond to a
published module. Unknown single-segment paths (`/dogs`, `/map`) fall
through to app routing.

See `apps/proxy/vanity.go`.

## Routing

Routes come from Tailscale capability grants (not hardcoded). The
`routesFromCaps()` function reads `Tailscale-Cap-*` headers, each containing
a JSON array of `{path, backend}` entries. The first path segment of the
request URL is matched against route keys.

When proxying:
- Strips the matched path prefix: `/map/foo` → `/foo`
- Sets `X-Forwarded-Prefix: /map`
- Rewrites root-relative `Location` headers to re-add the prefix
- Forwards `X-Request-ID` for distributed tracing
- Records Prometheus metrics (request count and duration)

Unmatched requests fall through to `pkg/gzipserver`, which serves
pre-compressed static files from the `static/` directory.

### Domain Redirects

`RedirectorMiddleware` handles domain-level redirects configured in the
machine TOML (e.g., `amonks.co` → `monks.co`).

## TLS

ACME certificates via `pkg/tls` (wrapping CertMagic). Supports DNS-01
(Route53), TLS-ALPN, and HTTP challenge strategies.

## Static Files

The `static/` directory contains hundreds of pre-compressed HTML pages
(`.gz` and `.br` variants). These are served by `pkg/gzipserver` with
content negotiation.

## Deployment

Runs on **fly.io** (Chicago ORD). `shared-cpu-8x`, 2GB. App name:
`monks-proxy`.
