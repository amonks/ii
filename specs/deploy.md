# Deploy

## Overview

Automated Fly.io deployment tool. On pushes to `main`, CI determines
which apps are affected by the change and deploys only those apps.
Change detection uses the monorepo's internal dependency graph, so a
change to a shared package triggers deployment of every app that
transitively depends on it.

Code: [cmd/deploy/](../cmd/deploy/)

Change detection library: [pkg/ci/changedetect/](../pkg/ci/changedetect/)

Config: [config/fly-apps.toml](../config/fly-apps.toml)

Dependency graph: [pkg/depgraph/](../pkg/depgraph/)

## Config

`config/fly-apps.toml` declares which apps are deployed to Fly. The
deploy tool only reads the app names from the `[apps.*]` keys; the
rest of the config is used by the Fly deployment infrastructure.

## Change Detection

The deploy tool uses `pkg/ci/changedetect` to diff changed files
between a base commit and HEAD (trying jj first, falling back to git),
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

The `pkg/depgraph` package builds a dependency graph using
`go/packages` to load `monks.co/*` imports from the workspace. It
resolves import paths to directories using `go.mod` module paths.

Key functions:

- `BuildDepGraph(root) → map[dir][]deps` — module-level dep graph
- `PackageDeps(root, pkgDir) → []dir` — package-level transitive deps
- `TransitiveDeps(graph, dir) → set[dir]` — walks a BuildDepGraph graph
- `BuildModuleMap(root) → map[modulePath]dir` — reads go.mod files
- `ResolveImportDir(importPath, moduleMap) → dir` — longest prefix match

`PackageDeps` provides finer-grained change detection than
module-level `BuildDepGraph`. This package is also used by
`cmd/publish` for validation.

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

## Migration to CI Service

`cmd/deploy` is being replaced by the CI service (`apps/ci`). The CI
service builder uses `pkg/ci/changedetect` (same logic) and builds OCI
images directly with `pkg/oci` instead of using Docker. See
[ci spec](ci.md).

During migration, both systems run in parallel. Once the CI service is
stable, `cmd/deploy` will be removed.

## Dependencies

- `flyctl` CLI (for deploying to Fly.io)
- `FLY_API_TOKEN` environment variable
- `MONKS_ROOT` environment variable
