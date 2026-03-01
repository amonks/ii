# Tailnet Package

## Overview

Wraps Tailscale's `tsnet` to embed apps directly into the tailnet. Provides
a shared HTTP client, listener, peer identification, and capability-grant
inspection.

Code: [pkg/tailnet/](../pkg/tailnet/)

## Exports

### Node Lifecycle

- `WaitReady(ctx) error` — blocks until the tsnet node is authenticated.
  The node hostname is `monks-<appname>-<machinename>`.
- `ReadyChan() <-chan struct{}` — non-blocking readiness channel.

### Serving

- `ListenAndServe(ctx, handler) error` — listens on `:80` via tsnet and
  serves HTTP. Shuts down gracefully on context cancellation.
- `Listen(network, addr) (net.Listener, error)` — raw listener on the
  tailnet interface.

### HTTP Client

- `Client() *http.Client` — returns an HTTP client whose transport routes
  through the tailnet. Used by all inter-app communication.

### Peer Identification

- `WhoIs(ctx, remoteAddr) (*apitype.WhoIsResponse, error)` — identifies
  the Tailscale peer behind a request by remote address.
- `AnonCaps(ctx) (tailcfg.PeerCapMap, error)` — extracts capability grants
  for `autogroup:danger-all` from the tailnet filter rules. Used by the
  proxy to build the anonymous route table.

## Configuration

| Env Var | Purpose |
|---------|---------|
| `TS_AUTHKEY` | Tailscale auth key for the tsnet node |
| `MONKS_DATA` | Parent directory for tsnet state storage |
| `MONKS_APP_NAME` | App name component of the tailnet hostname |

The tsnet state directory is `$MONKS_DATA/tsnet-monks-<app>-<machine>`.

## Dependencies

- `pkg/meta` — for app name and machine name
- `pkg/requireenv` — for `TS_AUTHKEY`
- `tailscale.com/tsnet` — the underlying Tailscale library
