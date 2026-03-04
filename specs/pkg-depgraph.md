# pkg/depgraph

## Overview

Builds a dependency graph of `monks.co/*` modules and packages using
`go/packages` for import analysis. Used by `cmd/publish` (validation),
`cmd/deploy` (change detection), and `apps/ci/cmd/builder` (deploy +
image rebuild change detection).

Code: [pkg/depgraph/](../pkg/depgraph/)

## API

### BuildDepGraph

```go
func BuildDepGraph(root string) (map[string][]string, error)
```

Uses `go/packages` to load all `monks.co/*` packages in the workspace,
then aggregates imports to module-level. Returns a map from directory
(e.g. `"apps/proxy"`) to its direct internal dependencies (e.g.
`["pkg/serve", "pkg/tailnet"]`).

Self-references are filtered out (e.g. `cmd/beetman` importing
`monks.co/beetman` does not produce a self-dependency).

### PackageDeps

```go
func PackageDeps(root, pkgDir string) ([]string, error)
```

Returns the package directories (relative to root) that the Go package
at `pkgDir` transitively depends on. Uses `go/packages` to load and
walk the import graph, filtered for `monks.co/*` imports.

Returns package-dir granularity (e.g. `pkg/ci/changedetect` not
`pkg/ci`), enabling finer-grained change detection than module-level
`BuildDepGraph`.

### TransitiveDeps

```go
func TransitiveDeps(graph map[string][]string, dir string) map[string]bool
```

Walks the graph to return all transitive dependencies of a directory.
Does not include the directory itself.

### BuildModuleMap

```go
func BuildModuleMap(root string) (map[string]string, error)
```

Reads `go.mod` files in `{apps,pkg,cmd}/*` and returns a map from
`monks.co/*` module path to directory. Only includes modules with
`monks.co/` prefixed paths.

### ResolveImportDir

```go
func ResolveImportDir(importPath string, modPathToDir map[string]string) (string, bool)
```

Finds the module directory for a `monks.co/*` import path using longest
prefix match. Handles cases where the module path doesn't match the
directory (e.g. `monks.co/beetman` at `cmd/beetman`).

### ReadModulePath

```go
func ReadModulePath(goModPath string) string
```

Reads the module path from a `go.mod` file. Returns empty string on
error.
