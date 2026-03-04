# pkg/ci

## Overview

Shared library for CI operations, extracted from `cmd/deploy` and
`cmd/publish`. Contains two subpackages.

Code: [pkg/ci/](../pkg/ci/)

## changedetect

Change detection and Fly app config loading.

Code: [pkg/ci/changedetect/](../pkg/ci/changedetect/)

### Types

- `FlyAppsConfig` — full `config/fly-apps.toml` structure including
  defaults and per-app settings (VM size, memory, volume, packages,
  files, cmd).
- `FlyAppDefaults` — default region, VM size, memory.
- `FlyAppEntry` — per-app config.

### Functions

- `LoadFlyAppsConfig(root) (*FlyAppsConfig, error)` — parse full config.
- `LoadFlyApps(root) ([]string, error)` — sorted app names only.
- `ChangedFiles(root, baseSHA) ([]string, error)` — files changed
  between baseSHA and HEAD. Tries `jj diff --name-only` first, falls
  back to `git diff --name-only`. Returns nil for all-zeros SHA
  (initial push).
- `AffectedApps(flyApps, changed, resolveDeps) ([]string, error)` —
  which Fly apps need deployment. `resolveDeps` returns transitive
  package-dir deps for a given package directory. See
  [deploy spec](deploy.md#change-detection) for rules.
- `IsImageAffected(changed, dockerfilePath, resolveDeps, pkgPath) (bool, error)` —
  whether a Dockerfile or its Go dependencies changed. For images
  with no Go code (base image), pass empty pkgPath.

## publish

Publish config, validation, and git operations for mirror publishing.

Code: [pkg/ci/publish/](../pkg/ci/publish/)

### Types

- `Config` — represents `config/publish.toml`.
- `Package` — per-package config (dir, module_path, mirror).

### Config Functions

- `LoadConfig(root) (*Config, error)` — parse publish.toml.
- `Config.PublicDirs() map[string]bool`
- `Config.PackageByDir(dir) *Package`
- `Config.ExpectedModulePath(dir) string`

### Validation Functions

- `ValidatePublicDeps(graph, publicDirs) []string` — errors if public
  packages depend on private packages.
- `ValidateLicenses(root, publicDirs) []string` — errors if public
  packages lack LICENSE files.
- `ValidateGoModPaths(root, cfg) []string` — errors if go.mod module
  paths don't match config.

### Git Operations

- `TopoSort(publicDirs, graph) ([]string, error)` — Kahn's algorithm.
- `GitEnv(root) []string` — env vars with jj backend support.
- `CloneSource(root) string` — clone path (jj git dir or repo root).
- `SubtreeSplit`, `MirrorExists`, `CreateMirror`, `PushToMirror`,
  `PushTagToMirror`, `FindMonorepoTags`, `MirrorTag`, `FilterRepo`.
- `Run(root, cfg, dryRun) error` — full publish flow.
