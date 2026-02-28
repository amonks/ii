package workspace

import (
	"errors"
	"testing"

	"github.com/amonks/incrementum/internal/db"
)

func TestWorkspaceErrorsAliasModel(t *testing.T) {
	if !errors.Is(ErrRepoPathNotFound, db.ErrRepoPathNotFound) {
		t.Fatalf("expected ErrRepoPathNotFound to wrap the state error")
	}
}
