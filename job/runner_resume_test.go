package job

import (
	"strings"
	"testing"
)

func TestCheckWorkspaceForResumeRequiresEmptyWorkingCopy(t *testing.T) {
	opts := RunOptions{
		CurrentChangeEmpty: func(string) (bool, error) {
			return false, nil
		},
	}

	err := checkWorkspaceForResume("/workspace", nil, opts)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "working copy is not empty") {
		t.Fatalf("expected empty working copy error, got %v", err)
	}
}

func TestCheckWorkspaceForResumeSkipsHistoryWhenNoCompletedChanges(t *testing.T) {
	change := JobChange{ChangeID: "change-1"}
	called := false
	opts := RunOptions{
		CurrentChangeEmpty: func(string) (bool, error) {
			return true, nil
		},
		ChangeIDsForRevset: func(string, string) ([]string, error) {
			called = true
			return nil, nil
		},
	}

	err := checkWorkspaceForResume("/workspace", []JobChange{change}, opts)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if called {
		t.Fatal("expected change history lookup to be skipped")
	}
}

func TestCheckWorkspaceForResumeValidatesCompletedChanges(t *testing.T) {
	changes := []JobChange{completedChange("change-1"), completedChange("change-2")}
	revsetSeen := ""
	opts := RunOptions{
		CurrentChangeEmpty: func(string) (bool, error) {
			return true, nil
		},
		ChangeIDsForRevset: func(_ string, revset string) ([]string, error) {
			revsetSeen = revset
			return []string{"change-1", "change-2"}, nil
		},
	}

	err := checkWorkspaceForResume("/workspace", changes, opts)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(revsetSeen, "ancestors(@)") {
		t.Fatalf("expected revset to include ancestors, got %q", revsetSeen)
	}
}

func TestCheckWorkspaceForResumeReportsMissingChanges(t *testing.T) {
	changes := []JobChange{completedChange("change-1"), completedChange("change-2")}
	opts := RunOptions{
		CurrentChangeEmpty: func(string) (bool, error) {
			return true, nil
		},
		ChangeIDsForRevset: func(string, string) ([]string, error) {
			return []string{"change-1"}, nil
		},
	}

	err := checkWorkspaceForResume("/workspace", changes, opts)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "missing completed changes") {
		t.Fatalf("expected missing changes error, got %v", err)
	}
	if !strings.Contains(err.Error(), "change-2") {
		t.Fatalf("expected missing change id in error, got %v", err)
	}
}

func completedChange(id string) JobChange {
	return JobChange{
		ChangeID: id,
		Commits: []JobCommit{
			{Review: &JobReview{Outcome: ReviewOutcomeAccept}},
		},
	}
}
