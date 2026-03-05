# Config Package

## Overview

Loads and parses the unified app configuration from `config/apps.toml` and
the proxy runtime configuration from `config/proxy.toml`. Used by the proxy,
directory, taskmaker, change detection, and CI builder to understand
deployment topology and app routing.

Code: [pkg/config/](../pkg/config/)

## Apps Config (`config/apps.toml`)

Single source of truth for all apps, their routes, access tiers, and
Fly deployment settings.

```toml
[defaults]
  region = "ord"
  vm_size = "shared-cpu-1x"
  vm_memory = "256"

[apps.dogs]
  vm_size = "shared-cpu-2x"
  vm_memory = "1gb"
  packages = ["sqlite"]

  [[apps.dogs.routes]]
    path = "dogs"
    host = "fly"
    access = "autogroup:danger-all"

  [[apps.dogs.routes]]
    path = "dogs"
    host = "thor"
    access = "ajm@passkey"
```

Each route has:
- `path` (required): URL path segment for proxy routing
- `host` (required): machine name (`fly`, `brigid`, `thor`)
- `access` (required): Tailscale ACL source (`autogroup:danger-all`,
  `ajm@passkey`, `tag:service`, `autogroup:member`)
- `capabilities` (optional): additional capability grants (e.g.
  `["movies-write"]`)

Fly-specific fields on the app: `vm_size`, `vm_memory`, `public`,
`packages`, `files`, `cmd`.

## Proxy Config (`config/proxy.toml`)

Runtime config for the proxy app: listener definitions, ACME/TLS
settings, and domain redirects.

## API

### Apps Config

- `LoadApps() (*AppsConfig, error)` — reads `$MONKS_ROOT/config/apps.toml`.
- `LoadAppsFrom(path) (*AppsConfig, error)` — reads from a specific path.
- `(*AppsConfig) ListHosts() []string` — sorted unique hosts from all routes.
- `(*AppsConfig) AppsForHost(host) []string` — sorted app names with a
  route on the given host.
- `(*AppsConfig) FlyApps() []string` — apps with `host = "fly"` routes.

### Proxy Config

- `LoadProxy() (*ProxyConfig, error)` — reads `$MONKS_ROOT/config/proxy.toml`.

## Key Types

- `AppsConfig` — top-level: `Defaults`, `Apps map[string]AppEntry`.
- `Defaults` — `Region`, `VMSize`, `VMMemory`.
- `AppEntry` — `VMSize`, `VMMemory`, `Public`, `Packages`, `Files`,
  `Cmd`, `Routes []Route`.
- `Route` — `Path`, `Host`, `Access`, `Capabilities`.
- `ProxyConfig` — `Services []Service`, `ACME`, `Redirects`.
- `Service` — `Type`, `Addr`, `Rewrites`.

## Dependencies

- `pkg/env` — for `$MONKS_ROOT`
- `pkg/tls` — for ACME config types
- `github.com/BurntSushi/toml`
