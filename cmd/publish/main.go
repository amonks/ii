// Package main is the publish tool for publishing monorepo subtrees
// as read-only GitHub mirrors.
package main

import (
	"flag"
	"fmt"
	"os"

	"monks.co/pkg/ci/publish"
	"monks.co/pkg/env"
)

var dryRun = flag.Bool("dry-run", false, "print what would be done without actually doing it")

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

	return publish.Run(root, cfg, *dryRun)
}
