package paths

import (
	"path/filepath"
	"testing"
)

func TestSanitizeRepoName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/Users/test/my-project", "users-test-my-project"},
		{"/Users/test/My Project", "users-test-my-project"},
		{"/home/user/some/deep/path", "home-user-some-deep-path"},
		{"/weird//double__slash", "weird-doubleslash"},
		{"/trailing/slash/", "trailing-slash"},
	}

	for _, tt := range tests {
		if result := sanitizeRepoName(tt.input); result != tt.expected {
			t.Errorf("sanitizeRepoName(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestSanitizeRepoNameExpandsHome(t *testing.T) {
	home, err := HomeDir()
	if err != nil {
		t.Fatalf("HomeDir: %v", err)
	}

	got := sanitizeRepoName("~/proj")
	want := sanitizeRepoName(filepath.Join(home, "proj"))
	if got != want {
		t.Fatalf("sanitizeRepoName(~/proj) = %q, want %q", got, want)
	}
}

func TestTodoLockPath(t *testing.T) {
	stateDir, err := DefaultStateDir()
	if err != nil {
		t.Fatalf("DefaultStateDir: %v", err)
	}

	got, err := TodoLockPath("/Users/test/my-project")
	if err != nil {
		t.Fatalf("TodoLockPath: %v", err)
	}

	want := filepath.Join(stateDir, "todo-users-test-my-project.lock")
	if got != want {
		t.Fatalf("TodoLockPath = %q, want %q", got, want)
	}
}

// TestTodoLockPathDistinguishesRepos guards the property the lock depends on:
// different repos must never collide on one lock file.
func TestTodoLockPathDistinguishesRepos(t *testing.T) {
	a, err := TodoLockPath("/Users/test/alpha")
	if err != nil {
		t.Fatalf("TodoLockPath: %v", err)
	}
	b, err := TodoLockPath("/Users/test/beta")
	if err != nil {
		t.Fatalf("TodoLockPath: %v", err)
	}
	if a == b {
		t.Fatalf("distinct repos shared lock path %q", a)
	}
}
