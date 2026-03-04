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

	"monks.co/pkg/ci/changedetect"
	"monks.co/pkg/depgraph"
	"monks.co/pkg/env"
)

var (
	baseSHA = flag.String("base", "", "git SHA to diff against (default HEAD~1)")
	dryRun  = flag.Bool("dry-run", false, "print what would be deployed without actually deploying")
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()

	root := env.InMonksRoot()

	apps, err := changedetect.LoadFlyApps(root)
	if err != nil {
		return fmt.Errorf("loading fly apps: %w", err)
	}

	base := *baseSHA
	if base == "" {
		base = "HEAD~1"
	}

	changed, err := changedetect.ChangedFiles(root, base)
	if err != nil {
		return fmt.Errorf("getting changed files: %w", err)
	}

	resolveDeps := func(pkgPath string) ([]string, error) {
		return depgraph.PackageDeps(root, pkgPath)
	}

	affected, err := changedetect.AffectedApps(apps, changed, resolveDeps)
	if err != nil {
		return fmt.Errorf("computing affected apps: %w", err)
	}

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
