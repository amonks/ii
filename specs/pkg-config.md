# Config Package

## Overview

Loads and parses per-machine TOML configuration files from the monks.co
config directory. Used by the proxy and directory apps to understand
deployment topology.

Code: [pkg/config/](../pkg/config/)

## Config Structure

```toml
mode = "production"

[[services]]
type = "https"          # or "redirect-to-https"
addr = ":4433"
storage_path = "/data/certmagic"

[services.acme]
production = true
domains = ["monks.co", "*.monks.co"]

[[services.acme.strategies]]
strategy = "dns-route53"
external_port = 443

[services.apps]
dogs = "monks-dogs-fly-ord"
map = "monks-map-fly-ord"
# ...

[[services.rewrites]]
from = "/old-path"
to = "/new-path"

[[redirects]]
from = "amonks.co"
to = "monks.co"
```

## API

- `Load(machine string) (*Config, error)` — reads
  `$MONKS_ROOT/config/<machine>.toml`, resolves variables.
- `ListMachines() ([]string, error)` — enumerates available machine configs.
- `(c *Config) Apps() []string` — derives app list from config.

## Key Types

- `Config` — top-level: `Mode`, `AppList`, `Services []Service`,
  `ACME`, `Redirects`.
- `Service` — `Type`, `Addr`, `Apps map[string]string`, `ExtraRoutes`,
  `StoragePath`, `Rewrites`.

## Dependencies

- `pkg/env` — for `$MONKS_ROOT`
- `pkg/tls` — for ACME config types
- `github.com/BurntSushi/toml`
