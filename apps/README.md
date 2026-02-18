# Apps

## Creating a new app

### 1. Create the app directory

    mkdir apps/$name

### 2. Create `apps/$name/main.go`

Use `apps/template/main.go` as a starting point. All apps follow the same
boilerplate: `main()` calls `run()`, which sets up logging, creates a mux,
waits for tailnet, and calls `tailnet.ListenAndServe`.

Apps are reachable both directly on the tailnet (e.g.
`https://monks-$name-$machine/`) and through the proxy (e.g.
`https://monks.co/$name/`). The proxy strips the path prefix on inbound
requests and sets the `X-Forwarded-Prefix` header (e.g. `/map`). It also
rewrites `Location` headers on responses so app-root-relative redirects
(`/path`) get the prefix prepended automatically.

In templates, use `serve.BasePath(r)` (or `serve.BasePathFromContext(ctx)` in
templ components) in a `<base href>` tag so relative URLs resolve correctly
regardless of mount point. Apps using `serve.Mux` get the BasePath set in
context automatically.

Use app-root-relative paths (`/path`) in `http.Redirect` calls -- the proxy
rewrites these. Use relative URLs (no leading `/`) in templates so they resolve
from the `<base>` tag.

### 3. Create `apps/$name/tasks.toml`

Use `apps/template/tasks.toml` as a starting point. Every app needs at least
three tasks:

- **dev**: long-running, depends on build, runs the binary
- **start**: long-running, runs the binary (no rebuild)
- **build**: short, watches `*.go`, runs `go build -o ../../bin/ .`

### 4. Add the app to a machine config

Edit the appropriate file in `config/`:

- `config/brigid.toml` -- brigid (home server)
- `config/thor.toml` -- thor (home server)

Add the app name to the `apps` list (keep it sorted alphabetically).

Fly-hosted apps are configured differently: add an entry to
`config/fly-apps.toml` instead. Taskmaker will generate `fly.toml`,
`Dockerfile.fly`, and `Dockerfile.fly.dockerignore` for you.

### 5. Run taskmaker

    go run ./cmd/taskmaker

This regenerates the root `tasks.toml` with the new app included in the
appropriate machine task and the `build` task.

### 6. Add a tailscale capability grant

The app won't be reachable through the proxy until you add a routing entry in
the tailscale ACL policy's capability grants. Add an entry like:

    {"path": "$name", "backend": "monks-$name-$machine"}

to the appropriate `monks.co/cap/public` grant based on who should have access
(see the `<routing>` section in `AGENTS.md` for examples).

### 7. Deploy

For brigid/thor apps, build and deploy to the machine:

    cd apps/$name && go build -o ../../bin/ . # on the machine

For fly apps:

    fly deploy -c apps/$name/fly.toml
