# App Boilerplate

## Overview

All monks.co apps follow a standard startup and serving pattern built on
a shared set of infrastructure packages. The `apps/template` directory
provides a skeleton that can be copied to bootstrap a new app.

## Standard Startup Sequence

Every app's `main()` calls `run()`, which follows this pattern:

```go
func run() error {
    ctx := sigctx.New()                              // 1. Signal handling
    reqlog.SetupLogging()                             // 2. Structured logging
    defer reqlog.Shutdown()

    // ... app-specific initialization (DB, config) ...

    mux := serve.NewMux()                             // 3. HTTP mux
    // ... register routes on mux ...

    if err := tailnet.WaitReady(ctx); err != nil {    // 4. Join tailnet
        return err
    }

    handler := reqlog.Middleware().ModifyHandler(      // 5. Middleware stack
        gzip.Middleware(mux))

    return tailnet.ListenAndServe(ctx, handler)        // 6. Serve
}
```

## Shared Packages

| Package | Role |
|---------|------|
| `pkg/sigctx` | Context cancelled on SIGHUP/SIGTERM/SIGINT/SIGQUIT |
| `pkg/reqlog` | Wide-event request logging + remote log shipping |
| `pkg/serve` | `http.ServeMux` wrapper with base-path awareness and `x-mux-route` header |
| `pkg/tailnet` | tsnet node management, readiness gate, HTTP client |
| `pkg/gzip` | Response gzip compression middleware |
| `pkg/meta` | App name (from `$MONKS_APP_NAME`) and machine name (from `$FLY_REGION` or `~/locals.fish`) |
| `pkg/database` | GORM + SQLite wrapper with WAL mode and migrations |
| `pkg/templib` | Shared templ page/card/form components |

## Environment Variables

| Variable | Source | Purpose |
|----------|--------|---------|
| `MONKS_APP_NAME` | Set per app | App identity for logging and tailnet hostname |
| `MONKS_ROOT` | Set per host | Repo root path for config files |
| `MONKS_DATA` | Set per host | Runtime data directory (databases, caches) |
| `TS_AUTHKEY` | Tailscale | tsnet authentication key |
| `FLY_REGION` | Fly.io | Machine region (e.g., `ord`) |

## Deployment

### Fly.io Apps

Apps deployed to Fly.io include a `fly.toml` and typically a `Dockerfile`.
Common Dockerfile pattern: Alpine-based, CGO enabled for SQLite, copies
the binary and any content directories. Persistent storage is a Fly volume
mounted at `/data`.

Deploy from the repo root:
```
fly deploy -c apps/$app/fly.toml
```

### Local Server Apps

Apps running on `brigid` or `thor` are built locally and run directly.
They use `tasks.toml` for build configuration and store data on local
filesystems or NAS mounts.

## Templating

Apps use one of two templating approaches:
- `a-h/templ` (preferred): Type-safe Go HTML templates compiled from `.templ` files
- `html/template`: Standard Go templates loaded from embedded `.gohtml` files via `pkg/util.ReadTemplates`

Both approaches embed template files into the binary via `//go:embed`.

## Database Pattern

Apps using SQLite open their database via `database.Open(path)` or
`database.OpenFromDataFolder(name)`. The wrapper enables WAL mode and
sets a 5-second busy timeout. Schema migrations are either:
- GORM auto-migrate (for simple schemas)
- Embedded `.sql` migration files loaded via `database.LoadMigrationsFromFS`
