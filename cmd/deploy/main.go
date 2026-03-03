// Package main is the deploy tool for deploying affected Fly apps
// based on changed files since a given git commit.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"monks.co/pkg/depgraph"
	"monks.co/pkg/env"
)

var (
	baseSHA = flag.String("base", "", "git SHA to diff against (default HEAD~1)")
	dryRun  = flag.Bool("dry-run", false, "print what would be deployed without actually deploying")
)

// FlyAppsConfig represents the [apps] section of config/fly-apps.toml.
type FlyAppsConfig struct {
	Apps map[string]any `toml:"apps"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()

	root := env.InMonksRoot()

	apps, err := loadFlyApps(root)
	if err != nil {
		return fmt.Errorf("loading fly apps: %w", err)
	}

	base := *baseSHA
	if base == "" {
		base = "HEAD~1"
	}

	changed, err := changedFiles(root, base)
	if err != nil {
		return fmt.Errorf("getting changed files: %w", err)
	}

	graph, err := depgraph.BuildDepGraph(root)
	if err != nil {
		return fmt.Errorf("building dep graph: %w", err)
	}

	affected := affectedApps(apps, changed, graph)

	if len(affected) == 0 {
		fmt.Println("no apps affected by changes")
		return nil
	}

	fmt.Printf("affected apps: %s\n", strings.Join(affected, ", "))

	if *dryRun {
		fmt.Println("[dry-run] would deploy the above apps")
		return nil
	}

	for _, app := range affected {
		if err := deploy(root, app); err != nil {
			return fmt.Errorf("deploying %s: %w", app, err)
		}
	}

	return nil
}

// loadFlyApps reads config/fly-apps.toml and returns the app names.
func loadFlyApps(root string) ([]string, error) {
	path := filepath.Join(root, "config", "fly-apps.toml")
	var cfg FlyAppsConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	var apps []string
	for name := range cfg.Apps {
		apps = append(apps, name)
	}
	sortStrings(apps)
	return apps, nil
}

// changedFiles returns the list of files changed between base and HEAD.
// If base is all zeros (initial push), returns nil to signal "deploy all".
func changedFiles(root, base string) ([]string, error) {
	if strings.TrimLeft(base, "0") == "" {
		// Initial push: all zeros → deploy everything.
		return nil, nil
	}

	cmd := exec.Command("git", "diff", "--name-only", base, "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git diff: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("git diff: %w", err)
	}

	var files []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// affectedApps determines which Fly apps need to be deployed based on
// the changed files and dependency graph.
//
// Rules:
//   - nil changed files (initial push) → all apps
//   - apps/<name>/** → that app (if it's a Fly app)
//   - pkg/<name>/** → any Fly app that transitively depends on that package
//   - go.mod, go.sum (root) → all apps
//   - config/fly-apps.toml → all apps
//   - anything else → nothing
func affectedApps(flyApps []string, changed []string, graph map[string][]string) []string {
	flyAppSet := map[string]bool{}
	for _, app := range flyApps {
		flyAppSet[app] = true
	}

	// nil means "deploy everything" (initial push).
	if changed == nil {
		return flyApps
	}

	affected := map[string]bool{}

	// Build reverse dependency map: for each package, which apps depend on it?
	reverseDeps := map[string][]string{}
	for _, app := range flyApps {
		appDir := filepath.Join("apps", app)
		for dep := range depgraph.TransitiveDeps(graph, appDir) {
			reverseDeps[dep] = append(reverseDeps[dep], app)
		}
	}

	for _, file := range changed {
		// Root go.mod/go.sum or config/fly-apps.toml → deploy all.
		if file == "go.mod" || file == "go.sum" || file == "config/fly-apps.toml" {
			return flyApps
		}

		// apps/<name>/... → that app.
		if strings.HasPrefix(file, "apps/") {
			parts := strings.SplitN(file, "/", 3)
			if len(parts) >= 2 {
				appName := parts[1]
				if flyAppSet[appName] {
					affected[appName] = true
				}
			}
			continue
		}

		// pkg/<name>/... → any app that transitively depends on it.
		if strings.HasPrefix(file, "pkg/") {
			parts := strings.SplitN(file, "/", 3)
			if len(parts) >= 2 {
				pkgDir := filepath.Join("pkg", parts[1])
				for _, app := range reverseDeps[pkgDir] {
					affected[app] = true
				}
			}
			continue
		}
	}

	var result []string
	for _, app := range flyApps {
		if affected[app] {
			result = append(result, app)
		}
	}
	return result
}

// deploy runs flyctl deploy for an app.
func deploy(root, app string) error {
	tomlPath := filepath.Join("apps", app, "fly.toml")
	fmt.Printf("deploying %s (%s)...\n", app, tomlPath)

	cmd := exec.Command("flyctl", "deploy", "-c", tomlPath)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("flyctl deploy: %w", err)
	}

	fmt.Printf("deployed %s\n", app)
	return nil
}

// sortStrings is a small helper to sort a string slice in place.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
