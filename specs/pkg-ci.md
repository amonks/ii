# pkg/ci

## Overview

Shared library for CI operations. Contains two subpackages.

Code: [pkg/ci/](../pkg/ci/)

## changedetect

Change detection and app config loading for Fly deployment.

Code: [pkg/ci/changedetect/](../pkg/ci/changedetect/)

### Functions

- `LoadFlyAppsConfig(root) (*config.AppsConfig, error)` — reads
  `config/apps.toml` via `pkg/config`.
- `LoadFlyApps(root) ([]string, error)` — sorted Fly app names only.
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

Publish config, validation, versioning, and git operations for mirror
publishing.

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
- `ValidateGoModCompleteness(root, graph, modPathToDir, publicDirs) []string` —
  errors if a public module imports a monks.co dependency that isn't
  declared in its go.mod (workspace resolution hides missing requires).
  Not used by the validate task since the publish flow handles go.mod
  rewriting automatically.
- `ValidateGoModPaths(root, cfg) []string` — errors if go.mod module
  paths don't match config.

### Versioning Functions

- `NextVersion(track, latestTag, dir) string` — computes next version
  by appending an incrementing number to the version track (e.g.,
  track `"1.0"` + latest `v1.0.3` → `v1.0.4`; track `"1.0.0-beta"` +
  latest `v1.0.0-beta.37` → `v1.0.0-beta.38`). If the track changed
  or there's no prior tag, starts at 1.
- `LatestTag(root, dir) (string, error)` — most recent publish tag
  for a directory, using semver ordering.
- `ChangedSinceTag(root, dir, tag) (bool, error)` — true if files
  in dir changed between tag and HEAD.
- `CreateTag(root, tag) error` — create a git tag at HEAD.

### go.mod Rewriting

- `RewriteGoMod(data []byte, requires map[string]string) ([]byte, error)` —
  adds or updates require directives in a go.mod file. Used at publish
  time to inject monks.co/* dependencies that the workspace normally
  resolves.

### Git Operations

- `TopoSort(publicDirs, graph) ([]string, error)` — Kahn's algorithm.
- `GitEnv(root) []string` — env vars with jj backend support.
- `CloneSource(root) string` — clone path (jj git dir or repo root).
- `SubtreeSplit`, `MirrorExists`, `CreateMirror`, `PushToMirror`,
  `PushTagToMirror`, `FindMonorepoTags`, `MirrorTag`, `FilterRepo`.
- `Run(w, root, cfg, dryRun) error` — full publish flow with change
  detection, versioning, go.mod rewriting, and tagging.
