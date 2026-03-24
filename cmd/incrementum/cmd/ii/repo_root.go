package main

import (
	"errors"
	"fmt"

	"monks.co/ww/ww"
)

func resolveRepoRoot(path string) (string, error) {
	root, err := ww.RepoRootFromPath(path)
	if err != nil {
		return "", formatRepoRootError(err)
	}
	return root, nil
}

func formatRepoRootError(err error) error {
	if errors.Is(err, ww.ErrWorkspaceRootNotFound) {
		return fmt.Errorf("not in a jj repository: %w", err)
	}
	if errors.Is(err, ww.ErrRepoPathNotFound) {
		return fmt.Errorf("workspace repo mapping missing: %w", err)
	}
	return err
}
