# Deploy

## Overview

Automated Fly.io deployment tool. On pushes to `main`, CI determines
which apps are affected by the change and deploys only those apps.
Change detection uses the monorepo's internal dependency graph, so a
change to a shared package triggers deployment of every app that
transitively depends on it.

Code: [cmd/deploy/](../cmd/deploy/)

Config: [config/fly-apps.toml](../config/fly-apps.toml)

Dependency graph: [pkg/depgraph/](../pkg/depgraph/)

## Config

`config/fly-apps.toml` declares which apps are deployed to Fly. The
deploy tool only reads the app names from the `[apps.*]` keys; the
rest of the config is used by the Fly deployment infrastructure.

## Change Detection

The deploy tool diffs changed files between a base commit and HEAD,
then maps those changes to affected apps:

| Changed path | Deploy |
|---|---|
| `apps/<name>/**` | That app (if it's a Fly app) |
| `pkg/<name>/**` | Any Fly app that transitively depends on that package |
| `go.mod`, `go.sum` (root) | All Fly apps |
| `config/fly-apps.toml` | All Fly apps |
| Anything else | Nothing |

Edge cases:

- `--base` is all zeros (initial push) → deploy all apps
- `--base` is empty/unset → default to `HEAD~1`

## Dependency Graph

The `pkg/depgraph` package builds a dependency graph by scanning
`{apps,pkg,cmd}/*` directories for Go source files containing
`monks.co/*` imports. It resolves import paths to module directories
using longest prefix match against `go.mod` files.

Key functions:

- `BuildDepGraph(root) → map[dir][]deps` — scans all modules
- `TransitiveDeps(graph, dir) → set[dir]` — walks the graph
- `BuildModuleMap(root) → map[modulePath]dir` — reads go.mod files
- `ResolveImportDir(importPath, moduleMap) → dir` — longest prefix match

This package is also used by `cmd/publish` for validation.

## Usage

```
go run ./cmd/deploy [--base <sha>] [--dry-run]
```

Flags:

- `--base <sha>`: Git SHA to diff against. Defaults to `HEAD~1`.
  Set to all zeros to deploy everything (initial push behavior).
- `--dry-run`: Print which apps would be deployed without deploying.

Requires `MONKS_ROOT` environment variable (set by CI).

## CI

GitHub Actions workflow (`.github/workflows/ci.yml`) runs the deploy
job after tests pass:

1. Checks out with full history (`fetch-depth: 0`).
2. Sets up Go 1.26.0.
3. Installs `flyctl` via `superfly/flyctl-action`.
4. Runs `go run ./cmd/deploy --base ${{ github.event.before }}`.

The `FLY_API_TOKEN` secret provides Fly.io authentication.

The deploy job uses `concurrency: fly-deploy` with
`cancel-in-progress: false` to ensure deployments run sequentially.

## Dependencies

- `flyctl` CLI (for deploying to Fly.io)
- `FLY_API_TOKEN` environment variable
- `MONKS_ROOT` environment variable
