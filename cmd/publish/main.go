// Package main is the publish tool for publishing monorepo subtrees
// as read-only GitHub mirrors.
package main

import (
	"flag"
	"fmt"
	"os"

	"monks.co/pkg/ci/publish"
	"monks.co/pkg/depgraph"
	"monks.co/pkg/env"
)

var (
	dryRun   = flag.Bool("dry-run", false, "print what would be done without actually doing it")
	validate = flag.Bool("validate", false, "validate public package constraints and exit")
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

	cfg, err := publish.LoadConfig(root)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(cfg.Package) == 0 {
		fmt.Println("no public packages configured")
		return nil
	}

	if *validate {
		return runValidate(root, cfg)
	}

	return publish.Run(os.Stdout, root, cfg, *dryRun)
}

func runValidate(root string, cfg *publish.Config) error {
	publicDirs := cfg.PublicDirs()

	graph, err := depgraph.BuildDepGraph(root)
	if err != nil {
		return fmt.Errorf("building dep graph: %w", err)
	}

	var allErrs []string
	allErrs = append(allErrs, publish.ValidatePublicDeps(graph, publicDirs)...)
	allErrs = append(allErrs, publish.ValidateLicenses(root, publicDirs)...)
	allErrs = append(allErrs, publish.ValidateGoModPaths(root, cfg)...)
	// Note: go.mod completeness (monks.co/* requires) is NOT validated here
	// because the publish flow rewrites go.mods at publish time to inject
	// the correct version-pinned require directives.

	if len(allErrs) > 0 {
		for _, e := range allErrs {
			fmt.Fprintln(os.Stderr, e)
		}
		return fmt.Errorf("%d publish validation error(s)", len(allErrs))
	}
	return nil
}
