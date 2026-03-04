// Package depgraph builds a dependency graph of monks.co/* modules
// using go/packages for import analysis.
package depgraph

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
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

// resolvePackageDir finds the package directory for a monks.co/* import path.
// Unlike ResolveImportDir which returns the module directory, this returns the
// full package directory including sub-paths within the module.
func resolvePackageDir(importPath string, modPathToDir map[string]string) (string, bool) {
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
	suffix := strings.TrimPrefix(importPath, bestMatch)
	suffix = strings.TrimPrefix(suffix, "/")
	if suffix == "" {
		return bestDir, true
	}
	return filepath.Join(bestDir, suffix), true
}

// resolveImportPath converts a package directory (relative to root) to its
// Go import path using the module map.
func resolveImportPath(pkgDir string, modPathToDir map[string]string) (string, bool) {
	bestDir := ""
	bestPath := ""
	for modPath, dir := range modPathToDir {
		if pkgDir == dir || strings.HasPrefix(pkgDir, dir+"/") {
			if len(dir) > len(bestDir) {
				bestDir = dir
				bestPath = modPath
			}
		}
	}
	if bestDir == "" {
		return "", false
	}
	suffix := strings.TrimPrefix(pkgDir, bestDir)
	suffix = strings.TrimPrefix(suffix, "/")
	if suffix == "" {
		return bestPath, true
	}
	return bestPath + "/" + suffix, true
}

// loadMonksPackages loads all monks.co/* packages from the workspace rooted
// at root and returns a map from import path to package.
func loadMonksPackages(root string) (map[string]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedImports | packages.NeedDeps | packages.NeedName,
		Dir:  root,
	}
	initial, err := packages.Load(cfg, "monks.co/...")
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}

	result := make(map[string]*packages.Package)
	var visit func(*packages.Package)
	visit = func(p *packages.Package) {
		if _, ok := result[p.PkgPath]; ok {
			return
		}
		if !strings.HasPrefix(p.PkgPath, "monks.co/") {
			return
		}
		result[p.PkgPath] = p
		for _, imp := range p.Imports {
			visit(imp)
		}
	}
	for _, p := range initial {
		visit(p)
	}
	return result, nil
}

// BuildDepGraph builds a dependency graph of monks.co/* imports
// for all {apps,pkg,cmd}/* directories.
func BuildDepGraph(root string) (map[string][]string, error) {
	modPathToDir, err := BuildModuleMap(root)
	if err != nil {
		return nil, fmt.Errorf("building module map: %w", err)
	}

	pkgs, err := loadMonksPackages(root)
	if err != nil {
		return nil, err
	}

	// Build module-level graph by aggregating package imports.
	moduleDeps := map[string]map[string]bool{}
	for _, dir := range modPathToDir {
		moduleDeps[dir] = map[string]bool{}
	}

	for pkgPath, pkg := range pkgs {
		srcDir, ok := ResolveImportDir(pkgPath, modPathToDir)
		if !ok {
			continue
		}
		if moduleDeps[srcDir] == nil {
			moduleDeps[srcDir] = map[string]bool{}
		}
		for impPath := range pkg.Imports {
			if !strings.HasPrefix(impPath, "monks.co/") {
				continue
			}
			depDir, ok := ResolveImportDir(impPath, modPathToDir)
			if !ok {
				continue
			}
			if depDir != srcDir {
				moduleDeps[srcDir][depDir] = true
			}
		}
	}

	graph := map[string][]string{}
	for dir, deps := range moduleDeps {
		var sorted []string
		for dep := range deps {
			sorted = append(sorted, dep)
		}
		sort.Strings(sorted)
		graph[dir] = sorted
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

// PackageDeps returns the package directories (relative to root) that the
// Go package at pkgDir transitively depends on.
func PackageDeps(root, pkgDir string) ([]string, error) {
	modPathToDir, err := BuildModuleMap(root)
	if err != nil {
		return nil, fmt.Errorf("building module map: %w", err)
	}

	pkgPath, ok := resolveImportPath(pkgDir, modPathToDir)
	if !ok {
		return nil, fmt.Errorf("cannot resolve import path for %s", pkgDir)
	}

	pkgs, err := loadMonksPackages(root)
	if err != nil {
		return nil, err
	}

	target, ok := pkgs[pkgPath]
	if !ok {
		return nil, fmt.Errorf("package %s not found", pkgPath)
	}

	// Walk transitive imports.
	visited := map[string]bool{}
	var walk func(*packages.Package)
	walk = func(p *packages.Package) {
		if visited[p.PkgPath] {
			return
		}
		visited[p.PkgPath] = true
		for _, imp := range p.Imports {
			if strings.HasPrefix(imp.PkgPath, "monks.co/") {
				walk(imp)
			}
		}
	}
	walk(target)
	delete(visited, pkgPath)

	var dirs []string
	for impPath := range visited {
		dir, ok := resolvePackageDir(impPath, modPathToDir)
		if ok {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}
