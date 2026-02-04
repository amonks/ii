package agent

import "testing"

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		pattern string
		str     string
		want    bool
	}{
		// Empty pattern
		{"", "", true},
		{"", "abc", false},

		// Exact match
		{"abc", "abc", true},
		{"abc", "abcd", false},
		{"abc", "ab", false},

		// Single wildcard
		{"*", "", true},
		{"*", "abc", true},
		{"*", "anything at all", true},

		// Wildcard at end
		{"abc*", "abc", true},
		{"abc*", "abcd", true},
		{"abc*", "abcdef", true},
		{"abc*", "ab", false},
		{"abc*", "xabc", false},

		// Wildcard at start
		{"*abc", "abc", true},
		{"*abc", "xabc", true},
		{"*abc", "xxxabc", true},
		{"*abc", "abcd", false},

		// Wildcard in middle
		{"a*c", "ac", true},
		{"a*c", "abc", true},
		{"a*c", "aXXXc", true},
		{"a*c", "acd", false},

		// Multiple wildcards
		{"a*b*c", "abc", true},
		{"a*b*c", "aXbYc", true},
		{"a*b*c", "aXXbYYc", true},
		{"a*b*c", "ac", false},

		// Question mark
		{"a?c", "abc", true},
		{"a?c", "aXc", true},
		{"a?c", "ac", false},
		{"a?c", "abbc", false},

		// Command-like patterns
		{"jj diff", "jj diff", true},
		{"jj diff", "jj diffx", false},
		{"jj diff *", "jj diff foo", true},
		{"jj diff *", "jj diff foo bar", true},
		{"jj diff *", "jj diff", false},
		{"jj *", "jj anything", true},
		{"jj *", "jj", false},

		// Edge cases
		{"**", "anything", true},
		{"a**b", "ab", true},
		{"a**b", "aXb", true},
		{"a**b", "aXXb", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.str, func(t *testing.T) {
			got := globMatch(tt.pattern, tt.str)
			if got != tt.want {
				t.Errorf("globMatch(%q, %q) = %v, want %v", tt.pattern, tt.str, got, tt.want)
			}
		})
	}
}

func TestBashPermissions_IsAllowed(t *testing.T) {
	// Permissions matching the spec example and default rules in agent/store.go
	perms := BashPermissions{
		Rules: []BashRule{
			{Pattern: "jj diff", Allow: true},
			{Pattern: "jj diff *", Allow: true},
			{Pattern: "jj file", Allow: true},
			{Pattern: "jj file *", Allow: true},
			{Pattern: "jj log", Allow: true},
			{Pattern: "jj log *", Allow: true},
			{Pattern: "jj show", Allow: true},
			{Pattern: "jj show *", Allow: true},
			{Pattern: "jj status", Allow: true},
			{Pattern: "jj status *", Allow: true},
			{Pattern: "jj *", Allow: false},
			{Pattern: "git *", Allow: false},
			{Pattern: "ii todo create *", Allow: true},
			{Pattern: "ii todo show *", Allow: true},
			{Pattern: "ii *", Allow: false},
			{Pattern: "*", Allow: true},
		},
	}

	tests := []struct {
		command string
		want    bool
	}{
		// Allowed jj commands
		{"jj diff", true},
		{"jj diff foo.txt", true},
		{"jj diff --stat", true},
		{"jj file", true},
		{"jj file list", true},
		{"jj log", true},
		{"jj log --limit 10", true},
		{"jj show", true},
		{"jj show @", true},
		{"jj status", true},
		{"jj status --no-pager", true},

		// Denied jj commands
		{"jj commit", false},
		{"jj commit -m message", false},
		{"jj new", false},
		{"jj squash", false},
		{"jj abandon", false},

		// Denied git commands
		{"git status", false},
		{"git diff", false},
		{"git commit -m message", false},

		// Allowed ii commands
		{"ii todo create --title=foo", true},
		{"ii todo create --title=foo --description=bar", true},
		{"ii todo show abc123", true},
		{"ii todo show abc123 --json", true},

		// Denied ii commands
		{"ii todo", false},
		{"ii todo list", false},
		{"ii todo create", false}, // missing args, no trailing *
		{"ii todo show", false},   // missing args, no trailing *
		{"ii agent run", false},
		{"ii agent list", false},
		{"ii job run", false},
		{"ii workspace list", false},

		// Other commands allowed
		{"ls", true},
		{"cat foo.txt", true},
		{"grep pattern file.txt", true},
		{"echo hello", true},
		{"cd /tmp && ls", true},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := perms.IsAllowed(tt.command)
			if got != tt.want {
				t.Errorf("IsAllowed(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestBashPermissions_DefaultDeny(t *testing.T) {
	// Empty permissions should deny everything
	perms := BashPermissions{}

	tests := []string{
		"ls",
		"cat file",
		"echo hello",
	}

	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			if perms.IsAllowed(cmd) {
				t.Errorf("empty permissions should deny %q", cmd)
			}
		})
	}
}

func TestBashPermissions_FirstMatchWins(t *testing.T) {
	// Test that first match wins
	perms := BashPermissions{
		Rules: []BashRule{
			{Pattern: "rm *", Allow: false},
			{Pattern: "rm -rf *", Allow: true}, // This should never match
			{Pattern: "*", Allow: true},
		},
	}

	// rm -rf should be denied because "rm *" matches first
	if perms.IsAllowed("rm -rf /") {
		t.Error("expected 'rm -rf /' to be denied")
	}

	// ls should be allowed by catch-all
	if !perms.IsAllowed("ls") {
		t.Error("expected 'ls' to be allowed")
	}
}

func TestBashPermissions_AllowAll(t *testing.T) {
	perms := BashPermissions{
		Rules: []BashRule{
			{Pattern: "*", Allow: true},
		},
	}

	tests := []string{
		"ls",
		"rm -rf /",
		"cat /etc/passwd",
		"any command at all",
	}

	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			if !perms.IsAllowed(cmd) {
				t.Errorf("allow-all should allow %q", cmd)
			}
		})
	}
}

func TestBashPermissions_DenyAll(t *testing.T) {
	perms := BashPermissions{
		Rules: []BashRule{
			{Pattern: "*", Allow: false},
		},
	}

	tests := []string{
		"ls",
		"echo hello",
	}

	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			if perms.IsAllowed(cmd) {
				t.Errorf("deny-all should deny %q", cmd)
			}
		})
	}
}

func TestBashPermissions_ComplexPatterns(t *testing.T) {
	perms := BashPermissions{
		Rules: []BashRule{
			// Allow read-only git commands
			{Pattern: "git status", Allow: true},
			{Pattern: "git status *", Allow: true},
			{Pattern: "git diff", Allow: true},
			{Pattern: "git diff *", Allow: true},
			{Pattern: "git log", Allow: true},
			{Pattern: "git log *", Allow: true},
			{Pattern: "git show", Allow: true},
			{Pattern: "git show *", Allow: true},
			// Deny all other git commands
			{Pattern: "git *", Allow: false},
			// Allow everything else
			{Pattern: "*", Allow: true},
		},
	}

	tests := []struct {
		command string
		want    bool
	}{
		// Allowed git commands
		{"git status", true},
		{"git status --short", true},
		{"git diff", true},
		{"git diff HEAD~1", true},
		{"git log", true},
		{"git log --oneline -10", true},
		{"git show", true},
		{"git show HEAD", true},

		// Denied git commands
		{"git commit", false},
		{"git commit -m message", false},
		{"git push", false},
		{"git push origin main", false},
		{"git rebase", false},
		{"git reset --hard", false},

		// Other commands allowed
		{"make build", true},
		{"go test ./...", true},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := perms.IsAllowed(tt.command)
			if got != tt.want {
				t.Errorf("IsAllowed(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

