package workspace

import (
	"errors"
	"testing"

	"monks.co/incrementum/internal/db"
)

func TestWorkspaceErrorsAliasModel(t *testing.T) {
	if !errors.Is(ErrRepoPathNotFound, db.ErrRepoPathNotFound) {
		t.Fatalf("expected ErrRepoPathNotFound to wrap the state error")
	}
}
