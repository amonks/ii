# Tailnet Routing

## Overview

All monks.co apps communicate over a shared Tailscale tailnet rather than
the public internet. Each app joins the tailnet as an embedded `tsnet` node
with hostname `monks-<app>-<machine>[-<region>]` and listens on TCP port 80.
The proxy app terminates public HTTPS traffic and forwards requests to
backend apps based on capability grants configured in the Tailscale ACL
policy.

## Tailnet Membership

Every app calls `pkg/tailnet.WaitReady(ctx)` at startup, which blocks until
the embedded tsnet node has authenticated with the Tailscale coordination
server. Authentication requires a `TS_AUTHKEY` environment variable. The
tsnet state directory is stored at `$MONKS_DATA/tsnet-monks-<app>-<machine>`.

Apps that need to make outbound tailnet HTTP requests use
`tailnet.Client()`, which returns an `*http.Client` configured to route
through the tsnet transport.

## Capability-Based Routing

Routes are not hardcoded in the proxy. Instead, they come from Tailscale ACL
capability grants on `tag:monks-co` nodes using the `monks.co/cap/public`
capability. Each grant entry specifies a `path` (first URL path segment) and
a `backend` (tailnet hostname).

When the proxy receives a request, it reads `Tailscale-Cap-*` headers
(injected by its own auth middleware) and builds a route table dynamically.
The first path segment of the URL is matched against route keys. If a match
is found, the request is reverse-proxied to the backend. If no match is
found, the proxy serves static files.

### Path Rewriting

When proxying, the proxy strips the matched path prefix before forwarding
(e.g., `/map/foo` becomes `/foo` at the backend) and sets
`X-Forwarded-Prefix: /map` so backends can construct absolute URLs. The
`pkg/serve` mux reads this header to provide `serve.BasePath(r)`.

Root-relative `Location` response headers are rewritten to re-add the
prefix (e.g., `/` becomes `/map/`).

## Access Tiers

Access is controlled through Tailscale ACL capability grants. Each app's
`access` field in `config/apps.toml` specifies the ACL source. The
`cmd/tailscale-acl` tool generates the routing portion of the Tailscale
ACL from this config, merged with `config/tailscale-acl-base.jsonc`.

Current tiers:

- **Public** (`autogroup:danger-all`): dogs, dungeon, homepage, map, writing
- **Tailnet members** (`autogroup:member`): air, movies
- **Services** (`tag:service`): aranet, ci, logs, mailer, monitor, proxy, sms
- **Private** (`ajm@passkey`): calendar, ci, creamery, directory, dogs, dolmenwood, golink, homepage, logs, map, movies, ping, reddit, scrobbles, writing, youtube

### Dual-Listener Architecture

The proxy runs two listener pipelines on the same process:

1. **Public listener** (port 443 via Fly.io ProxyProto): strips any
   incoming `Tailscale-*` headers (anti-spoofing), then injects capability
   headers for `autogroup:danger-all` (anonymous internet traffic).

2. **Tailnet listener** (tsnet :443): calls `WhoIs` to identify the
   Tailscale peer and sets `Tailscale-User` and `Tailscale-Cap-*` headers
   based on that peer's actual capabilities.

This allows different middleware pipelines for anonymous vs. authenticated
traffic without running separate processes.

## Inter-App Communication

Apps communicate with each other exclusively over the tailnet:

- The `air` app fetches sensor data from `http://monks-aranet-brigid/`
- All apps ship logs to `http://monks-logs-fly-ord/ingest`
- The `emailclient` package posts to `http://monks-mailer-fly-ord/`
- The `sms` app is called by other services to send SMS alerts

There is no direct public-internet communication between apps.
