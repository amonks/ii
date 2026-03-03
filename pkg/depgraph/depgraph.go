// Package depgraph builds a dependency graph of monks.co/* modules
// by scanning Go source files for internal imports.
package depgraph

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ReadModulePath reads the module path from a go.mod file.
func ReadModulePath(goModPath string) string {
	bs, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(bs), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "module "); ok {
			return after
		}
	}
	return ""
}

// BuildModuleMap scans all {apps,pkg,cmd}/* directories for go.mod files
// and returns a map from monks.co/* module path to directory.
func BuildModuleMap(root string) (map[string]string, error) {
	modPathToDir := map[string]string{}
	for _, prefix := range []string{"apps", "pkg", "cmd"} {
		prefixDir := filepath.Join(root, prefix)
		entries, err := os.ReadDir(prefixDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(prefix, e.Name())
			modPath := ReadModulePath(filepath.Join(root, dir, "go.mod"))
			if modPath != "" && strings.HasPrefix(modPath, "monks.co/") {
				modPathToDir[modPath] = dir
			}
		}
	}
	return modPathToDir, nil
}

// ResolveImportDir finds the module directory for a monks.co/* import path
// using longest prefix match against known module paths.
func ResolveImportDir(importPath string, modPathToDir map[string]string) (string, bool) {
	bestMatch := ""
	bestDir := ""
	for modPath, dir := range modPathToDir {
		if importPath == modPath || strings.HasPrefix(importPath, modPath+"/") {
			if len(modPath) > len(bestMatch) {
				bestMatch = modPath
				bestDir = dir
			}
		}
	}
	if bestMatch == "" {
		return "", false
	}
	return bestDir, true
}

// findInternalImports scans .go files in a directory tree for monks.co/* imports,
// resolving each to its module directory via longest prefix match.
func findInternalImports(dir string, modPathToDir map[string]string) ([]string, error) {
	seen := map[string]bool{}

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}

		bs, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		content := string(bs)
		for line := range strings.SplitSeq(content, "\n") {
			line = strings.TrimSpace(line)
			idx := strings.Index(line, `"monks.co/`)
			if idx < 0 {
				continue
			}
			rest := line[idx+1:]
			before, _, ok := strings.Cut(rest, `"`)
			if !ok {
				continue
			}
			importPath := before

			if modDir, ok := ResolveImportDir(importPath, modPathToDir); ok {
				seen[modDir] = true
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	var deps []string
	for dep := range seen {
		deps = append(deps, dep)
	}
	sort.Strings(deps)
	return deps, nil
}

// BuildDepGraph builds a dependency graph of monks.co/* imports
// for all {apps,pkg,cmd}/* directories.
func BuildDepGraph(root string) (map[string][]string, error) {
	modPathToDir, err := BuildModuleMap(root)
	if err != nil {
		return nil, fmt.Errorf("building module map: %w", err)
	}

	graph := map[string][]string{}

	for _, prefix := range []string{"apps", "pkg", "cmd"} {
		prefixDir := filepath.Join(root, prefix)
		entries, err := os.ReadDir(prefixDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(prefix, e.Name())
			absDir := filepath.Join(root, dir)

			deps, err := findInternalImports(absDir, modPathToDir)
			if err != nil {
				return nil, fmt.Errorf("scanning %s: %w", dir, err)
			}
			// Filter out self-references (e.g. cmd/beetman/beetman
			// imports monks.co/beetman which resolves to cmd/beetman).
			filtered := deps[:0]
			for _, dep := range deps {
				if dep != dir {
					filtered = append(filtered, dep)
				}
			}
			graph[dir] = filtered
		}
	}

	return graph, nil
}

// TransitiveDeps returns all transitive dependencies of a directory.
func TransitiveDeps(graph map[string][]string, dir string) map[string]bool {
	visited := map[string]bool{}
	var walk func(string)
	walk = func(d string) {
		if visited[d] {
			return
		}
		visited[d] = true
		for _, dep := range graph[d] {
			walk(dep)
		}
	}
	walk(dir)
	delete(visited, dir) // don't include self
	return visited
}
