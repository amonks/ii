// Package changedetect determines which Fly apps are affected by
// code changes, using dependency graph analysis.
package changedetect

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"monks.co/pkg/config"
)

// LoadFlyAppsConfig reads config/apps.toml and returns the full config.
func LoadFlyAppsConfig(root string) (*config.AppsConfig, error) {
	path := filepath.Join(root, "config", "apps.toml")
	return config.LoadAppsFrom(path)
}

// LoadFlyApps reads config/apps.toml and returns the sorted fly app names.
func LoadFlyApps(root string) ([]string, error) {
	cfg, err := LoadFlyAppsConfig(root)
	if err != nil {
		return nil, err
	}
	return cfg.FlyApps(), nil
}

// ChangedFiles returns the list of files changed between baseSHA and HEAD.
// It tries jj first (jj diff --name-only), falling back to git.
// If baseSHA is all zeros (initial push), returns nil to signal "deploy all".
func ChangedFiles(root, baseSHA string) ([]string, error) {
	if strings.TrimLeft(baseSHA, "0") == "" {
		// Initial push: all zeros → deploy everything.
		return nil, nil
	}

	// Try jj first.
	out, err := tryJJ(root, baseSHA)
	if err != nil {
		// Fall back to git.
		out, err = tryGit(root, baseSHA)
		if err != nil {
			return nil, err
		}
	}

	var files []string
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func tryJJ(root, baseSHA string) (string, error) {
	cmd := exec.Command("jj", "diff", "--from", baseSHA, "--to", "@", "--name-only")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("jj diff: %w", err)
	}
	return string(out), nil
}

func tryGit(root, baseSHA string) (string, error) {
	cmd := exec.Command("git", "diff", "--name-only", baseSHA, "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git diff: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

// AffectedApps determines which Fly apps need to be deployed based on
// the changed files and package-level dependency analysis.
//
// resolveDeps returns the transitive package-dir dependencies for a given
// package directory. In production this wraps depgraph.PackageDeps; in tests
// it can be a simple map lookup.
//
// Rules:
//   - nil changed files (initial push) → all apps
//   - any file under a dep's directory → that app
//   - go.mod, go.sum (root) → all apps
//   - config/apps.toml → all apps
//   - anything else → nothing
func AffectedApps(flyApps []string, changed []string, resolveDeps func(pkgPath string) ([]string, error)) ([]string, error) {
	flyAppSet := map[string]bool{}
	for _, app := range flyApps {
		flyAppSet[app] = true
	}

	// nil means "deploy everything" (initial push).
	if changed == nil {
		return flyApps, nil
	}

	// Build reverse dependency map: for each package dir, which apps depend on it?
	reverseDeps := map[string][]string{}
	for _, app := range flyApps {
		appDir := filepath.Join("apps", app)
		// Include self so own-source changes are detected.
		reverseDeps[appDir] = append(reverseDeps[appDir], app)

		deps, err := resolveDeps(appDir)
		if err != nil {
			return nil, fmt.Errorf("resolving deps for %s: %w", app, err)
		}
		for _, dep := range deps {
			reverseDeps[dep] = append(reverseDeps[dep], app)
		}
	}

	affected := map[string]bool{}

	for _, file := range changed {
		// Root go.mod/go.sum or config/ changes → deploy all.
		// config/apps.toml affects routing, config/publish.toml affects
		// the proxy's vanity import handler.
		if file == "go.mod" || file == "go.sum" || strings.HasPrefix(file, "config/") {
			return flyApps, nil
		}

		// Check prefix match against all reverse dep keys.
		for dir, apps := range reverseDeps {
			if strings.HasPrefix(file, dir+"/") {
				for _, app := range apps {
					affected[app] = true
				}
			}
		}
	}

	var result []string
	for _, app := range flyApps {
		if affected[app] {
			result = append(result, app)
		}
	}
	return result, nil
}

// IsImageAffected returns true if the given Dockerfile or any of the package's
// transitive dependencies have changed. For images with no Go code (like the
// base image), pass empty pkgPath to only check the Dockerfile.
func IsImageAffected(changed []string, dockerfilePath string, resolveDeps func(string) ([]string, error), pkgPath string) (bool, error) {
	if slices.Contains(changed, dockerfilePath) {
		return true, nil
	}

	if pkgPath == "" {
		return false, nil
	}

	deps, err := resolveDeps(pkgPath)
	if err != nil {
		return false, fmt.Errorf("resolving deps for %s: %w", pkgPath, err)
	}

	// Include self.
	allDirs := append([]string{pkgPath}, deps...)

	for _, file := range changed {
		for _, dir := range allDirs {
			if strings.HasPrefix(file, dir+"/") {
				return true, nil
			}
		}
	}

	return false, nil
}

// sortStrings sorts a string slice in place.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
