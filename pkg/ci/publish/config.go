// Package publish provides configuration and validation for publishing
// monorepo subtrees as read-only GitHub mirrors.
package publish

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"monks.co/pkg/depgraph"
)

// Config represents config/publish.toml.
type Config struct {
	DefaultMirror string    `toml:"default_mirror"`
	Package       []Package `toml:"package"`
}

// Package is the configuration for a single published package.
type Package struct {
	Dir        string `toml:"dir"`
	ModulePath string `toml:"module_path"`
	Mirror     string `toml:"mirror"`
}

// LoadConfig loads config/publish.toml.
func LoadConfig(root string) (*Config, error) {
	path := filepath.Join(root, "config", "publish.toml")
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &cfg, nil
}

// PublicDirs returns the set of directory paths that are public.
func (c *Config) PublicDirs() map[string]bool {
	dirs := map[string]bool{}
	for _, pkg := range c.Package {
		dirs[pkg.Dir] = true
	}
	return dirs
}

// PackageByDir returns the package config for a directory, or nil.
func (c *Config) PackageByDir(dir string) *Package {
	for i := range c.Package {
		if c.Package[i].Dir == dir {
			return &c.Package[i]
		}
	}
	return nil
}

// ExpectedModulePath returns the expected module path for a directory.
func (c *Config) ExpectedModulePath(dir string) string {
	for _, pkg := range c.Package {
		if pkg.Dir == dir && pkg.ModulePath != "" {
			return pkg.ModulePath
		}
	}
	return "monks.co/" + dir
}

// ValidatePublicDeps checks that no public package transitively depends
// on a private package. Returns a list of errors.
func ValidatePublicDeps(graph map[string][]string, publicDirs map[string]bool) []string {
	var errs []string
	for dir := range publicDirs {
		transitive := depgraph.TransitiveDeps(graph, dir)
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
func ValidateGoModPaths(root string, cfg *Config) []string {
	var errs []string
	for _, pkg := range cfg.Package {
		goModPath := filepath.Join(root, pkg.Dir, "go.mod")
		bs, err := os.ReadFile(goModPath)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: missing go.mod", pkg.Dir))
			continue
		}

		expected := cfg.ExpectedModulePath(pkg.Dir)
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
