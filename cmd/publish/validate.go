package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// PublishConfig represents config/publish.toml.
type PublishConfig struct {
	DefaultMirror string           `toml:"default_mirror"`
	Package       []PublishPackage `toml:"package"`
}

type PublishPackage struct {
	Dir        string `toml:"dir"`
	ModulePath string `toml:"module_path"`
	Mirror     string `toml:"mirror"`
}

// LoadPublishConfig loads config/publish.toml.
func LoadPublishConfig(root string) (*PublishConfig, error) {
	path := filepath.Join(root, "config", "publish.toml")
	var cfg PublishConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &cfg, nil
}

// PublicDirs returns the set of directory paths that are public.
func (c *PublishConfig) PublicDirs() map[string]bool {
	dirs := map[string]bool{}
	for _, pkg := range c.Package {
		dirs[pkg.Dir] = true
	}
	return dirs
}

// PackageByDir returns the package config for a directory, or nil.
func (c *PublishConfig) PackageByDir(dir string) *PublishPackage {
	for i := range c.Package {
		if c.Package[i].Dir == dir {
			return &c.Package[i]
		}
	}
	return nil
}

// ExpectedModulePath returns the expected module path for a directory.
func (c *PublishConfig) ExpectedModulePath(dir string) string {
	for _, pkg := range c.Package {
		if pkg.Dir == dir && pkg.ModulePath != "" {
			return pkg.ModulePath
		}
	}
	return "monks.co/" + dir
}

// readModulePath reads the module path from a go.mod file.
func readModulePath(goModPath string) string {
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

// buildModuleMap scans all {apps,pkg,cmd}/* directories for go.mod files
// and returns a map from monks.co/* module path to directory.
func buildModuleMap(root string) (map[string]string, error) {
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
			modPath := readModulePath(filepath.Join(root, dir, "go.mod"))
			if modPath != "" && strings.HasPrefix(modPath, "monks.co/") {
				modPathToDir[modPath] = dir
			}
		}
	}
	return modPathToDir, nil
}

// resolveImportDir finds the module directory for a monks.co/* import path
// using longest prefix match against known module paths.
func resolveImportDir(importPath string, modPathToDir map[string]string) (string, bool) {
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

// BuildDepGraph builds a dependency graph of monks.co/* imports
// for all {apps,pkg,cmd}/* directories.
func BuildDepGraph(root string) (map[string][]string, error) {
	modPathToDir, err := buildModuleMap(root)
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

			if modDir, ok := resolveImportDir(importPath, modPathToDir); ok {
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

// ValidatePublicDeps checks that no public package transitively depends
// on a private package. Returns a list of errors.
func ValidatePublicDeps(graph map[string][]string, publicDirs map[string]bool) []string {
	var errs []string
	for dir := range publicDirs {
		transitive := TransitiveDeps(graph, dir)
		for dep := range transitive {
			if !publicDirs[dep] {
				errs = append(errs, fmt.Sprintf("%s is public but depends on private %s", dir, dep))
			}
		}
	}
	sort.Strings(errs)
	return errs
}

// ValidateLicenses checks that every public package has a LICENSE file.
func ValidateLicenses(root string, publicDirs map[string]bool) []string {
	var errs []string
	for dir := range publicDirs {
		// Check for LICENSE or LICENSE.md
		found := false
		for _, name := range []string{"LICENSE", "LICENSE.md", "LICENSE.txt"} {
			if _, err := os.Stat(filepath.Join(root, dir, name)); err == nil {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, fmt.Sprintf("%s is public but has no LICENSE file", dir))
		}
	}
	sort.Strings(errs)
	return errs
}

// ValidateGoModPaths checks that every public package's go.mod has the
// expected module path.
func ValidateGoModPaths(root string, cfg *PublishConfig) []string {
	var errs []string
	for _, pkg := range cfg.Package {
		goModPath := filepath.Join(root, pkg.Dir, "go.mod")
		bs, err := os.ReadFile(goModPath)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: missing go.mod", pkg.Dir))
			continue
		}

		expected := cfg.ExpectedModulePath(pkg.Dir)
		// Parse first line: "module monks.co/pkg/serve"
		for line := range strings.SplitSeq(string(bs), "\n") {
			line = strings.TrimSpace(line)
			if after, ok := strings.CutPrefix(line, "module "); ok {
				actual := after
				if actual != expected {
					errs = append(errs, fmt.Sprintf("%s: go.mod has module %s, expected %s", pkg.Dir, actual, expected))
				}
				break
			}
		}
	}
	sort.Strings(errs)
	return errs
}
