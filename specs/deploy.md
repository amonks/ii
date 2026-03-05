# Deploy

## Overview

Automated Fly.io deployment via the CI service. On pushes to `main`, CI
determines which apps are affected by the change and deploys only those
apps. Change detection uses the monorepo's internal dependency graph, so a
change to a shared package triggers deployment of every app that
transitively depends on it.

Change detection library: [pkg/ci/changedetect/](../pkg/ci/changedetect/)

Config: [config/apps.toml](../config/apps.toml)

Dependency graph: [pkg/depgraph/](../pkg/depgraph/)

## Config

`config/apps.toml` declares all apps and their deployment targets. Fly
apps are those with at least one route where `host = "fly"`. The deploy
system reads app names and Fly-specific fields (vm_size, vm_memory,
files, cmd, public, packages) from this config.

## Change Detection

The CI builder uses `pkg/ci/changedetect` to diff changed files
between a base commit and HEAD (trying jj first, falling back to git),
then maps those changes to affected apps:

| Changed path | Deploy |
|---|---|
| `apps/<name>/**` | That app (if it's a Fly app) |
| `pkg/<name>/**` | Any Fly app that transitively depends on that package |
| `go.mod`, `go.sum` (root) | All Fly apps |
| `config/apps.toml` | All Fly apps |
| Anything else | Nothing |

Edge cases:

- Base SHA is all zeros (initial push) -> deploy all apps
- No changed files -> nothing to deploy

## Dependency Graph

The `pkg/depgraph` package builds a dependency graph using
`go/packages` to load `monks.co/*` imports from the workspace. It
resolves import paths to directories using `go.mod` module paths.

Key functions:

- `BuildDepGraph(root) -> map[dir][]deps` -- module-level dep graph
- `PackageDeps(root, pkgDir) -> []dir` -- package-level transitive deps
- `TransitiveDeps(graph, dir) -> set[dir]` -- walks a BuildDepGraph graph
- `BuildModuleMap(root) -> map[modulePath]dir` -- reads go.mod files
- `ResolveImportDir(importPath, moduleMap) -> dir` -- longest prefix match

`PackageDeps` provides finer-grained change detection than
module-level `BuildDepGraph`. This package is also used by
`cmd/publish` for validation.

## OCI Image Building

The CI builder compiles app binaries, builds OCI images using
`pkg/oci`, pushes to the Fly registry, and deploys via `fly deploy
--image`. See [ci spec](ci.md) for details.

## Dependencies

- `flyctl` CLI (for deploying to Fly.io)
- `FLY_API_TOKEN` environment variable
- `MONKS_ROOT` environment variable
