package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/agent"
	internalstrings "github.com/amonks/incrementum/internal/strings"
)

func TestFormatAgentTablePreservesAlignmentWithANSI(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-2 * time.Minute)

	sessions := []agent.Session{
		{
			ID:        "sess1",
			Status:    agent.SessionActive,
			Model:     "claude-sonnet-4",
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		},
		{
			ID:          "sess2",
			Status:      agent.SessionCompleted,
			Model:       "claude-sonnet-4",
			CreatedAt:   createdAt.Add(-time.Minute),
			UpdatedAt:   now,
			CompletedAt: now,
		},
	}

	plain := formatAgentTable(sessions, func(id string, prefix int) string { return id }, now, nil)
	ansi := formatAgentTable(sessions, func(id string, prefix int) string {
		if prefix <= 0 || prefix > len(id) {
			return id
		}
		return "\x1b[1m\x1b[36m" + id[:prefix] + "\x1b[0m" + id[prefix:]
	}, now, nil)

	if stripANSICodes(ansi) != plain {
		t.Fatalf("expected ANSI output to align with plain output\nplain:\n%s\nansi:\n%s", plain, ansi)
	}
}

func trimmedAgentTable(sessions []agent.Session, highlight func(string, int) string, now time.Time) string {
	return internalstrings.TrimSpace(formatAgentTable(sessions, highlight, now, nil))
}

func TestFormatAgentTableIncludesSessionID(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	sessions := []agent.Session{
		{
			ID:        "sess-123",
			Status:    agent.SessionActive,
			Model:     "claude-sonnet-4",
			CreatedAt: now.Add(-time.Minute),
			UpdatedAt: now.Add(-time.Minute),
		},
	}

	output := trimmedAgentTable(sessions, func(id string, prefix int) string { return id }, now)
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	header := lines[0]
	sessionIndex := strings.Index(header, "SESSION")
	statusIndex := strings.Index(header, "STATUS")
	if sessionIndex == -1 || statusIndex == -1 || sessionIndex > statusIndex {
		t.Fatalf("expected SESSION column before STATUS in header, got: %q", header)
	}

	row := lines[1]
	if !strings.Contains(row, "sess-123") {
		t.Fatalf("expected session id in row, got: %q", row)
	}
}

func TestFormatAgentTableUsesCompactAge(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-2 * time.Minute)

	sessions := []agent.Session{
		{
			ID:        "sess-001",
			Status:    agent.SessionActive,
			Model:     "claude-sonnet-4",
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		},
	}

	output := trimmedAgentTable(sessions, func(id string, prefix int) string { return id }, now)
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	// Find the AGE column
	if !strings.Contains(lines[1], "2m") {
		t.Fatalf("expected compact age 2m in output, got: %q", lines[1])
	}
}

func TestFormatAgentTableShowsMissingAgeAsDash(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	sessions := []agent.Session{
		{
			ID:     "sess-1",
			Status: agent.SessionActive,
			Model:  "claude-sonnet-4",
		},
	}

	output := trimmedAgentTable(sessions, func(value string, prefix int) string { return value }, now)
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	// Dash should appear for age when CreatedAt is zero
	fields := strings.Fields(lines[1])
	// SESSION STATUS MODEL AGE DURATION TOKENS COST
	// Find the AGE field (after MODEL)
	ageIndex := -1
	for i, f := range fields {
		if f == "-" && i >= 3 {
			ageIndex = i
			break
		}
	}
	if ageIndex == -1 {
		t.Fatalf("expected dash for missing age, got: %q", lines[1])
	}
}

func TestFormatAgentTableShowsAgeForCompletedSession(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	sessions := []agent.Session{
		{
			ID:        "sess-complete",
			Status:    agent.SessionCompleted,
			Model:     "claude-sonnet-4",
			CreatedAt: now.Add(-5 * time.Minute),
		},
	}

	output := trimmedAgentTable(sessions, func(id string, prefix int) string { return id }, now)
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	if !strings.Contains(lines[1], "5m") {
		t.Fatalf("expected age 5m in output, got: %q", lines[1])
	}
}

func TestFormatAgentTableShowsDuration(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-10 * time.Minute)
	completedAt := now.Add(-7 * time.Minute)

	sessions := []agent.Session{
		{
			ID:          "sess-duration",
			Status:      agent.SessionCompleted,
			Model:       "claude-sonnet-4",
			CreatedAt:   createdAt,
			UpdatedAt:   completedAt,
			CompletedAt: completedAt,
		},
	}

	output := trimmedAgentTable(sessions, func(id string, prefix int) string { return id }, now)
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	// Duration should be 3m (completed_at - created_at)
	if !strings.Contains(lines[1], "3m") {
		t.Fatalf("expected duration 3m in output, got: %q", lines[1])
	}
}

func TestFormatAgentTableUsesSessionPrefixLengths(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-5 * time.Minute)

	sessions := []agent.Session{
		{
			ID:        "abc123",
			Status:    agent.SessionActive,
			Model:     "claude-sonnet-4",
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		},
		{
			ID:          "abd999",
			Status:      agent.SessionCompleted,
			Model:       "claude-sonnet-4",
			CreatedAt:   createdAt,
			UpdatedAt:   now,
			CompletedAt: now,
		},
	}

	output := formatAgentTable(sessions, func(id string, prefix int) string {
		return id + ":" + strconv.Itoa(prefix)
	}, now, nil)

	// Both IDs share "ab" prefix, so need 3 chars to distinguish (abc vs abd)
	if !strings.Contains(output, "abc123:3") {
		t.Fatalf("expected session prefix length 3, got: %q", output)
	}
	if !strings.Contains(output, "abd999:3") {
		t.Fatalf("expected session prefix length 3, got: %q", output)
	}
}

func TestFormatAgentTableShowsTokensAndCost(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-5 * time.Minute)

	sessions := []agent.Session{
		{
			ID:          "sess-usage",
			Status:      agent.SessionCompleted,
			Model:       "claude-sonnet-4",
			CreatedAt:   createdAt,
			UpdatedAt:   now,
			CompletedAt: now,
			TokensUsed:  1234,
			Cost:        0.0567,
		},
	}

	output := trimmedAgentTable(sessions, func(id string, prefix int) string { return id }, now)
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	if !strings.Contains(lines[1], "1234") {
		t.Fatalf("expected tokens 1234 in output, got: %q", lines[1])
	}
	if !strings.Contains(lines[1], "$0.0567") {
		t.Fatalf("expected cost $0.0567 in output, got: %q", lines[1])
	}
}

func TestAgentSessionPrefixLengths(t *testing.T) {
	sessions := []agent.Session{
		{ID: "abc123"},
		{ID: "abd456"},
		{ID: "xyz789"},
	}

	lengths := agentSessionPrefixLengths(sessions)

	// abc and abd share "ab", so need 3 chars to distinguish
	if lengths["abc123"] != 3 {
		t.Fatalf("expected prefix length 3 for abc123, got %d", lengths["abc123"])
	}
	if lengths["abd456"] != 3 {
		t.Fatalf("expected prefix length 3 for abd456, got %d", lengths["abd456"])
	}
	// xyz is unique from position 1
	if lengths["xyz789"] != 1 {
		t.Fatalf("expected prefix length 1 for xyz789, got %d", lengths["xyz789"])
	}
}

func TestFormatAgentAge(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		session  agent.Session
		expected string
	}{
		{
			name:     "zero time returns dash",
			session:  agent.Session{},
			expected: "-",
		},
		{
			name: "minutes ago",
			session: agent.Session{
				CreatedAt: now.Add(-5 * time.Minute),
			},
			expected: "5m",
		},
		{
			name: "hours ago",
			session: agent.Session{
				CreatedAt: now.Add(-2 * time.Hour),
			},
			expected: "2h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAgentAge(tt.session, now)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestFormatAgentDuration(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		session  agent.Session
		expected string
	}{
		{
			name:     "zero created_at returns dash",
			session:  agent.Session{},
			expected: "-",
		},
		{
			name: "active session uses now minus created_at",
			session: agent.Session{
				Status:    agent.SessionActive,
				CreatedAt: now.Add(-10 * time.Minute),
			},
			expected: "10m",
		},
		{
			name: "completed session uses completed_at minus created_at",
			session: agent.Session{
				Status:      agent.SessionCompleted,
				CreatedAt:   now.Add(-10 * time.Minute),
				CompletedAt: now.Add(-5 * time.Minute),
			},
			expected: "5m",
		},
		{
			name: "completed session without completed_at uses updated_at",
			session: agent.Session{
				Status:    agent.SessionCompleted,
				CreatedAt: now.Add(-10 * time.Minute),
				UpdatedAt: now.Add(-3 * time.Minute),
			},
			expected: "7m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAgentDuration(tt.session, now)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestFilterAgentSessionsForList(t *testing.T) {
	sessions := []agent.Session{
		{ID: "1", Status: agent.SessionActive},
		{ID: "2", Status: agent.SessionCompleted},
		{ID: "3", Status: agent.SessionActive},
		{ID: "4", Status: agent.SessionFailed},
	}

	t.Run("include all returns all sessions", func(t *testing.T) {
		filtered := filterAgentSessionsForList(sessions, true)
		if len(filtered) != 4 {
			t.Fatalf("expected 4 sessions, got %d", len(filtered))
		}
	})

	t.Run("default returns only active sessions", func(t *testing.T) {
		filtered := filterAgentSessionsForList(sessions, false)
		if len(filtered) != 2 {
			t.Fatalf("expected 2 active sessions, got %d", len(filtered))
		}
		for _, s := range filtered {
			if s.Status != agent.SessionActive {
				t.Fatalf("expected only active sessions, got %s", s.Status)
			}
		}
	})
}
