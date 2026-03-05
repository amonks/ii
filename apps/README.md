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

### 4. Add the app to `config/apps.toml`

Add an `[apps.$name]` entry with at least one route:

```toml
[apps.$name]
  [[apps.$name.routes]]
    path = "$name"
    host = "$machine"    # brigid, thor, or fly
    access = "$access"   # autogroup:danger-all, ajm@passkey, tag:service, autogroup:member
```

For Fly-hosted apps, you can also set `vm_size`, `vm_memory`, `packages`,
`files`, `cmd`, and `public`.

### 5. Run taskmaker

    go run ./cmd/taskmaker

This regenerates the root `tasks.toml` with the new app included in the
appropriate machine task and the `build` task. For Fly apps, it also
generates `fly.toml`.

### 6. Generate Tailscale ACL (optional)

If you want to verify the ACL routing:

    go run ./cmd/tailscale-acl

The ACL is generated from `config/apps.toml` routes. No manual ACL editing
is required -- update `config/apps.toml` and the routing grants are derived
automatically.

### 7. Deploy

For brigid/thor apps, build and run on the machine:

    cd apps/$name && go build -o ../../bin/ . # on the machine

For fly apps, CI deploys automatically on push to main.
