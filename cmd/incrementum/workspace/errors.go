package workspace

import (
	"errors"

	"monks.co/incrementum/internal/db"
)

var (
	// ErrWorkspaceRootNotFound indicates a path is not in a jj workspace.
	ErrWorkspaceRootNotFound = errors.New("workspace root not found")
	// ErrRepoPathNotFound indicates a workspace is tracked but missing repo info.
	ErrRepoPathNotFound = db.ErrRepoPathNotFound
)
