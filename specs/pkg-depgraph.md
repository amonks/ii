# pkg/depgraph

## Overview

Builds a dependency graph of `monks.co/*` modules by scanning Go
source files for internal imports. Used by `cmd/publish` (validation)
and `cmd/deploy` (change detection).

Code: [pkg/depgraph/](../pkg/depgraph/)

## API

### BuildDepGraph

```go
func BuildDepGraph(root string) (map[string][]string, error)
```

Scans `{apps,pkg,cmd}/*` for Go files containing `monks.co/*` imports.
Returns a map from directory (e.g. `"apps/proxy"`) to its direct
internal dependencies (e.g. `["pkg/serve", "pkg/tailnet"]`).

Self-references are filtered out (e.g. `cmd/beetman` importing
`monks.co/beetman` does not produce a self-dependency).

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
