# Tailscale Service Mesh + Separate Fly Apps

## Context

The goal is to decouple apps so they can be deployed and scaled independently,
and so apps on different machines (Fly, brigid, thor) can be reached through a
single proxy. Tailscale becomes the universal service mesh: every app gets a
tsnet listener with hostname `monks-{app}-{machine}`, and the Fly proxy routes
to backends by tailscale hostname. Localhost listeners and the `config/ports`
system go away entirely.

## Architecture After

```
                    Internet
                       |
                  monks-proxy-fly (Fly app, public)
                  TLS termination, ACME, domain redirects
                       |
            tailscale (all backends reached via tsnet)
         /        |        |         \          \
  monks-      monks-    monks-    monks-     monks-
  homepage    dogs      map       writing    golink
  -fly-ord    -fly-ord  -fly-ord  -fly-ord   -brigid
```

- Every app is a separate Fly app (or runs on brigid/thor)
- Every app has a tsnet listener; no localhost, no port mapping
- The proxy routes via tailscale hostnames specified in config
- Local dev (`run brigid`) also uses tsnet — no proxy needed on dev machines
- `config/ports` and `pkg/ports` are deleted

## Phases

### Phase 1: Foundational Code Changes

These changes prepare the codebase without changing how anything is deployed.
The existing single-machine Fly deployment keeps working throughout.

#### 1a. Delete the credentials package; per-app credentials

**Problem**: `credentials/credentials.go` uses package-level `var X = require("ENV")`
which panics at import time if ANY env var is missing. Since almost every app
imports `errlogger` -> `tailnet` -> `credentials`, every app needs all 11 env
vars set.

**Change**: Delete `credentials/credentials.go`. Each app (or package) that
needs secrets defines its own `credentials.go` with only the env vars it
actually uses, failing immediately at startup. This way each app only requires
the secrets it needs.

For example, `pkg/tailnet/credentials.go`:
```go
package tailnet
import "monks.co/pkg/requireenv"
var tailscaleAuthKey = requireenv.Require("TS_AUTHKEY")
```

`pkg/emailclient/credentials.go`:
```go
package emailclient
import "monks.co/pkg/requireenv"
var smtpPassword = requireenv.Require("SMTP_PASSWORD")
var smtpUsername = requireenv.Require("SMTP_USERNAME")
```

A small `pkg/requireenv` helper replaces the old package:
```go
func Require(env string) string {
    v := os.Getenv(env)
    if v == "" { panic(fmt.Errorf("env '%s' not set", env)) }
    return v
}
```

**Files deleted**: `credentials/credentials.go`
**Files created**: `pkg/requireenv/requireenv.go`
**Files modified**: Every file that imports `credentials` — move the relevant
env var into the package that uses it.

#### 1b. Refactor tailnet package

**File**: `pkg/tailnet/tailnet.go`

Replace the current auto-starting global server with two explicit functions:

- `ListenAndServe(ctx, handler)` — starts a tsnet.Server with hostname
  `monks-{meta.AppName()}-{meta.MachineName()}`, listens on `:80`, serves HTTP.
  Uses `os.TempDir()` for state (ephemeral nodes, no volume needed).

- `Client()` — lazily starts a client-only tsnet node for making outbound
  requests to other tailscale hosts. Used by the proxy and by
  `errlogger`/`emailclient`.

Remove the `init()` auto-start. Remove the global `var server`.

#### 1c. Keep multi-region machine names

**File**: `pkg/meta/meta.go`

No change needed. `MachineName()` already returns `"fly-ord"` on Fly, which
gives tailscale hostnames like `monks-homepage-fly-ord`. This preserves the
ability to run the same app in multiple regions without hostname collisions.

Config references use the full name: `monks-homepage-fly-ord`.

#### 1d. Update every app to use tailnet.ListenAndServe

**Files**: `apps/*/main.go` (all ~20 apps)

Replace:
```go
port := ports.Apps["appname"]
addr := fmt.Sprintf("127.0.0.1:%d", port)
serve.ListenAndServe(ctx, addr, gzip.Middleware(mux))
```

With:
```go
tailnet.ListenAndServe(ctx, gzip.Middleware(mux))
```

Remove imports of `pkg/ports`, `fmt` (where no longer needed).

#### 1e. Delete `config/ports` and `pkg/ports`

Once no app uses them.

**Files to delete**:
- `config/ports`
- `pkg/ports/ports.go`

#### 1f. Refactor proxy to route via tailscale hostnames

**Files**: `apps/proxy/main.go`, `apps/proxy/proxy.go`, `pkg/config/config.go`

Change config format for prod (Fly):

```toml
# config/fly.toml
[[service]]
  type = "https"
  [service.apps]
    homepage = "monks-homepage-fly-ord"
    golink   = "monks-golink-brigid"
```

Change `config.Service.Apps` from `[]string` to `map[string]string`.

In `proxy.go`, change `routes` from `map[string]int` to `map[string]string`.
Use `tailnet.Client().Transport` as the reverse proxy transport.

```go
proxy := &httputil.ReverseProxy{
    Transport: p.client.Transport,
    Rewrite: func(r *httputil.ProxyRequest) {
        r.Out.URL.Scheme = "http"
        r.Out.URL.Host = backend  // "monks-homepage-fly-ord"
        ...
    },
}
```

The proxy still listens on `0.0.0.0:8080` and `0.0.0.0:4433` (for Fly's edge)
— it's the one app that does NOT use tailnet.ListenAndServe for its primary
listeners. But it uses `tailnet.Client()` for outbound connections.

#### 1g. Remove tsnet service from proxy

The proxy currently runs a tsnet listener for mailer/traffic/errlog. Remove it.
Those apps now have their own tsnet listeners and clients reach them directly.

**File**: `config/fly.toml` — remove the `[[service]] type = "tsnet"` block.

#### 1h. Update errlogger and emailclient URLs

**Files**: `pkg/errlogger/errlogger.go`, `pkg/emailclient/emailclient.go`

Replace hardcoded `http://fly.ss.cx/mailer/` and `http://fly.ss.cx/errlog/`
with tailscale hostnames:

```go
const errlogURL = "http://monks-errlog-fly-ord/"
const mailerURL = "http://monks-mailer-fly-ord/"
```

These use `tailnet.Client()` to make the request (already do today).

#### 1i. Refactor traffic logging for separate apps

**Problem**: The proxy currently uses `pkg/traffic.New()` as middleware, which
opens `traffic.db` directly. With separate apps, the proxy and traffic app are
on different machines.

**Solution**: Split `pkg/traffic` into two parts:

1. `pkg/traffic` (model + query) — used by the traffic app to own the DB and
   serve the dashboard. Also exposes a `POST /log` endpoint.

2. `pkg/trafficclient` — HTTP client used by the proxy to POST log entries
   to the traffic app via tailscale. Implements the same `middleware.Middleware`
   interface so the proxy code barely changes.

The trafficclient should batch log entries (buffer locally, flush
periodically or when the buffer is full) to avoid a network round-trip per
request.

**Files**:
- `pkg/traffic/traffic.go` — keep model, add HTTP handler for receiving logs
- `pkg/trafficclient/trafficclient.go` — new, HTTP client middleware with batching
- `apps/traffic/main.go` — add POST endpoint (accepts single + batch)
- `apps/proxy/main.go` — use trafficclient instead of traffic.New()

#### 1j. Fix file access patterns

The direction is eventually apps in their own repos, so "CWD is app directory"
is a useful property to preserve. Generated Dockerfiles should set
`WORKDIR /app/apps/{name}`. But two specific patterns should be improved:

- **dogs** `migrate.sql`: Change `os.ReadFile("migrate.sql")` to `//go:embed`
  (matching the pattern errlog already uses for `schema.sql`). Embed is better
  than a runtime file read regardless of CWD.
  **File**: `apps/dogs/main.go`

- **traffic** `./static/`: This works with CWD set correctly. But the CSS
  file is generated at build time and could be embedded instead. Use
  `//go:embed static/index.css` so the binary is self-contained.
  **File**: `apps/traffic/server.go`

#### 1k. Simplify machine configs (brigid, thor)

**Files**: `config/brigid.toml`, `config/thor.toml`

Replace service/proxy/TLS config with just app lists:

```toml
# config/brigid.toml
apps = ["golink", "scrobbles", "calendar", "aranet"]
```

No proxy, no ACME, no service definitions. Taskmaker generates run tasks from these.

#### 1l. Update taskmaker

**File**: `cmd/taskmaker/main.go`

- Machine configs with `mode = "dev"` now have flat `apps = [...]` lists
  (no service blocks). Generate tasks like `apps/golink/dev` without a
  proxy dependency.
- Machine configs with `mode = "prod"` (fly.toml) are ignored by taskmaker
  (Fly apps deploy independently, not via `run`).
- The `fly` task and `proxy-fly` task are removed from generation.
- `start.sh` is deleted (no more `run fly`).

### Phase 2: Extend Taskmaker

Extend the existing `taskmaker` command to also generate per-app Dockerfiles
and fly.tomls, in addition to `tasks.toml`. One invocation produces everything.

#### 2a. Create the app registry config

**File**: `config/fly-apps.toml`

```toml
[defaults]
  region = "ord"
  vm_size = "shared-cpu-1x"
  vm_memory = "256mb"

[apps.proxy]
  vm_size = "shared-cpu-2x"
  vm_memory = "512mb"
  volume = "monks-proxy-data"
  public = true
  packages = ["iptables", "ip6tables", "sqlite"]
  files = ["config/fly.toml", "static/"]

[apps.homepage]
  files = ["writing/"]

[apps.writing]
  files = ["writing/"]

[apps.dogs]
  volume = "monks-dogs-data"
  packages = ["sqlite"]

[apps.traffic]
  volume = "monks-traffic-data"
  packages = ["sqlite"]

[apps.errlog]
  volume = "monks-errlog-data"
  packages = ["sqlite"]

[apps.map]
  volume = "monks-map-data"
  packages = ["sqlite"]

[apps.mailer]
  # no volume, no extra files
```

#### 2b. Extend taskmaker to generate Fly configs

**File**: `cmd/taskmaker/main.go` (extend existing)

In addition to `tasks.toml`, taskmaker reads `config/fly-apps.toml` and generates:

**Per-app Dockerfile** (`apps/{name}/Dockerfile.fly`):

```dockerfile
# Generated by taskmaker - DO NOT EDIT.
FROM golang:alpine AS gobuild
  RUN apk update && apk add build-base gcc bash nodejs npm
  WORKDIR /app
  COPY . .
  RUN go install github.com/amonks/run/cmd/run@latest
  RUN run apps/air/build && run build

FROM alpine
  RUN apk add --no-cache ca-certificates {packages}
  WORKDIR /app/apps/{name}
  COPY --from=gobuild /app/bin/{name} /app/bin/app
  {extra COPY lines for files}
  ENV MONKS_ROOT=/app
  ENV MONKS_DATA=/data
  CMD ["/app/bin/app"]
```

Build stage is identical for all apps (layer-cached). Runtime stage varies
per app (binary, packages, files, volume). WORKDIR is set to
`/app/apps/{name}` so CWD matches what apps expect.

**Per-app fly.toml** (`apps/{name}/fly.toml`):

```toml
# Generated by taskmaker - DO NOT EDIT.
app = "monks-{name}"
primary_region = "ord"

[build]
  dockerfile = "Dockerfile.fly"

[env]
  MONKS_ROOT = "/app"
  MONKS_DATA = "/data"

[[vm]]
  size = "{vm_size}"
  memory = "{vm_memory}"

# if volume:
[[mounts]]
  source = "{volume}"
  destination = "/data"

# if public (proxy only):
[[services]]
  ...
```

The proxy's fly.toml also gets `[metrics]`, `[[services]]` with proxy_proto, etc.
Non-proxy apps get no services section (they're only reachable via tailscale).

#### 2c. Delete cmd/deploy

**Files deleted**: `cmd/deploy/main.go`, `.version`

Deploy directly with `fly deploy -c apps/{name}/fly.toml`. No wrapper needed.

### Phase 3: Deployment Migration

One-time steps to cut over from the monolithic monks-go app.

#### 3a. Create Fly apps

```bash
fly apps create monks-proxy
fly apps create monks-homepage
fly apps create monks-dogs
fly apps create monks-map
fly apps create monks-traffic
fly apps create monks-errlog
fly apps create monks-mailer
fly apps create monks-writing
```

#### 3b. Set secrets on each app

After the credentials refactoring (1a), each app only needs the env vars it
actually uses. Every app needs `TS_AUTHKEY` (for tsnet). Beyond that:

- **proxy**: just TS_AUTHKEY
- **mailer**: TS_AUTHKEY, SMTP_USERNAME, SMTP_PASSWORD
- **homepage**: TS_AUTHKEY
- **dogs**: TS_AUTHKEY
- **map**: TS_AUTHKEY, GOOGLE_PLACES_BACKEND_API_KEY, GOOGLE_PLACES_BROWSER_API_KEY
- **traffic**: TS_AUTHKEY
- **errlog**: TS_AUTHKEY
- **writing**: TS_AUTHKEY

```bash
# Every app needs at least TS_AUTHKEY
for app in proxy homepage dogs map traffic errlog mailer writing; do
  fly secrets set -a monks-$app TS_AUTHKEY=...
done
# Then per-app secrets
fly secrets set -a monks-mailer SMTP_USERNAME=... SMTP_PASSWORD=...
# etc.
```

#### 3c. Create volumes

```bash
fly volumes create monks-proxy-data   -a monks-proxy   --region ord --size 3
fly volumes create monks-dogs-data    -a monks-dogs    --region ord --size 1
fly volumes create monks-traffic-data -a monks-traffic  --region ord --size 1
fly volumes create monks-errlog-data  -a monks-errlog   --region ord --size 1
fly volumes create monks-map-data     -a monks-map      --region ord --size 1
```

#### 3d. Migrate data from the old volume

```bash
# SSH into old monks-go machine and copy DBs locally
fly ssh console -a monks-go
# ... or use fly sftp to download each .db file

# Upload to new volumes
fly sftp shell -a monks-traffic
> put traffic.db /data/traffic.db

fly sftp shell -a monks-errlog
> put errlog.db /data/errlog.db

fly sftp shell -a monks-map
> put map.db /data/map.db

fly sftp shell -a monks-dogs
> put dogs.db /data/dogs/dogs.db
# also upload dogs/images/ directory

# Proxy needs ACME cert data from the old /data volume
fly sftp shell -a monks-proxy
> put certificates/ /data/certificates/
# (whatever certmagic stores — check /data on the old machine)
```

#### 3e. Deploy each app

```bash
for app in proxy homepage dogs map traffic errlog mailer writing; do
  fly deploy -c apps/$app/fly.toml
done
```

#### 3f. Update DNS

Fly's edge routing: monks-proxy gets the IP that monks.co points to.
The old monks-go app can be scaled down once everything works.

#### 3g. Update update-db script

**File**: `update-db`

Change target app from `monks-go` to per-app names:
```bash
fly sftp shell -a monks-$dbname
```

#### 3h. Decommission old monks-go app

Once everything is verified:
```bash
fly apps destroy monks-go
```

## Per-App Summary

| App | Volume | Extra files in image | Notes |
|---|---|---|---|
| proxy | monks-proxy-data (ACME certs) | config/fly.toml, static/ | Public services, metrics, tailnet client |
| homepage | none | writing/ | Letterboxd cache uses /tmp as MONKS_DATA |
| writing | none | writing/ | Serves media from writing/ dir |
| dogs | monks-dogs-data | (none) | DB + images in volume, embed migrate.sql |
| traffic | monks-traffic-data | (none) | Owns traffic.db, receives POSTs from proxy |
| errlog | monks-errlog-data | (none) | Owns errlog.db |
| map | monks-map-data | (none) | Read-only DB, seeded via update-db |
| mailer | none | (none) | Stateless, sends email via SES |

## Files Changed/Created/Deleted

**Modified**:
- `pkg/tailnet/tailnet.go` — ListenAndServe + Client, remove init/global, own credentials
- `pkg/config/config.go` — Service.Apps becomes map[string]string for prod
- `pkg/errlogger/errlogger.go` — direct tailscale URL, own credentials
- `pkg/emailclient/emailclient.go` — direct tailscale URL, own credentials
- `pkg/traffic/traffic.go` — add POST handler for receiving logs
- `apps/proxy/main.go` — use trafficclient, use tailnet.Client transport
- `apps/proxy/proxy.go` — routes map[string]string, use tailscale backends
- `apps/dogs/main.go` — embed migrate.sql, use tailnet.ListenAndServe
- `apps/traffic/main.go` — add POST endpoint, use tailnet.ListenAndServe
- `apps/traffic/server.go` — embed static CSS
- `apps/*/main.go` — all apps: tailnet.ListenAndServe, remove ports import
- `config/fly.toml` — service.apps as map, remove tsnet service
- `config/brigid.toml` — simplify to app list
- `config/thor.toml` — simplify to app list
- `cmd/taskmaker/main.go` — also generates Dockerfiles + fly.tomls
- `update-db` — per-app targeting

**Created**:
- `pkg/requireenv/requireenv.go` — tiny helper replacing credentials package
- `pkg/trafficclient/trafficclient.go` — HTTP middleware for proxy with batching
- `config/fly-apps.toml` — app registry for taskmaker
- `apps/*/Dockerfile.fly` — generated per-app Dockerfiles
- `apps/*/fly.toml` — generated per-app Fly configs

**Deleted**:
- `credentials/credentials.go` — replaced by per-package credentials
- `config/ports`
- `pkg/ports/ports.go`
- `start.sh`
- `cmd/deploy/` — use `fly deploy -c` directly
- `.version`
- `Dockerfile` (replaced by per-app generated Dockerfiles)

## Verification

- Run `go test ./...` after each phase
- Phase 1 verification: run `run brigid` locally, confirm apps are reachable
  via tailscale hostnames from another machine on the tailnet
- Phase 2 verification: run `go run ./cmd/taskmaker`, inspect generated files
- Phase 3 verification: deploy to Fly, check that https://monks.co serves
  correctly, check tailscale admin for expected nodes, check traffic dashboard
  is logging, check errlog receives reports
