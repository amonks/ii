package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"monks.co/incrementum/todo"
)

func TestResolveDescriptionFromStdin(t *testing.T) {
	cases := []struct {
		name string
		desc string
		in   string
		want string
	}{
		{
			name: "stdin with newline",
			desc: "-",
			in:   "Hello from stdin\n",
			want: "Hello from stdin",
		},
		{
			name: "stdin without newline",
			desc: "-",
			in:   "No newline",
			want: "No newline",
		},
		{
			name: "stdin with multiple newlines",
			desc: "-",
			in:   "Trim me\n\n\r\n",
			want: "Trim me",
		},
		{
			name: "literal description",
			desc: "Already set",
			in:   "ignored",
			want: "Already set",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveDescriptionFromStdin(tc.desc, bytes.NewBufferString(tc.in))
			if err != nil {
				t.Fatalf("resolveDescriptionFromStdin failed: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestLogHighlighterUsesProvidedPrefixLengths(t *testing.T) {
	prefixLengths := map[string]int{"abc123": 4, "abd456": 3}
	highlight := logHighlighter(prefixLengths, func(id string, prefix int) string {
		return fmt.Sprintf("%s:%d", id, prefix)
	})

	if got := highlight("abc123"); got != "abc123:4" {
		t.Fatalf("expected abc123 to use prefix 4, got %q", got)
	}
	if got := highlight("abd456"); got != "abd456:3" {
		t.Fatalf("expected abd456 to use prefix 3, got %q", got)
	}
}

func TestLogHighlighterHandlesMissingID(t *testing.T) {
	prefixLengths := map[string]int{"abc123": 4}
	highlight := logHighlighter(prefixLengths, func(id string, prefix int) string {
		return fmt.Sprintf("%s:%d", id, prefix)
	})

	if got := highlight("zzz999"); got != "zzz999:0" {
		t.Fatalf("expected missing id to use prefix 0, got %q", got)
	}
}

func TestShouldUseTodoUpdateEditor(t *testing.T) {
	cases := []struct {
		name           string
		hasUpdateFlags bool
		editFlag       bool
		noEditFlag     bool
		interactive    bool
		want           bool
	}{
		{
			name:           "flags set without edit",
			hasUpdateFlags: true,
			interactive:    true,
			want:           false,
		},
		{
			name:           "flags set with edit",
			hasUpdateFlags: true,
			editFlag:       true,
			interactive:    true,
			want:           true,
		},
		{
			name:           "flags set with no-edit",
			hasUpdateFlags: true,
			noEditFlag:     true,
			interactive:    true,
			want:           false,
		},
		{
			name:        "no flags interactive",
			interactive: true,
			want:        true,
		},
		{
			name: "no flags non-interactive",
			want: false,
		},
		{
			name:        "no flags with edit",
			editFlag:    true,
			interactive: false,
			want:        true,
		},
		{
			name:        "no flags with no-edit",
			noEditFlag:  true,
			interactive: true,
			want:        false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldUseEditor(tc.hasUpdateFlags, tc.editFlag, tc.noEditFlag, tc.interactive)
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestShouldUseTodoCreateEditor(t *testing.T) {
	cases := []struct {
		name           string
		hasCreateFlags bool
		editFlag       bool
		noEditFlag     bool
		interactive    bool
		want           bool
	}{
		{
			name:        "no flags interactive",
			interactive: true,
			want:        true,
		},
		{
			name:        "no flags non-interactive",
			interactive: false,
			want:        false,
		},
		{
			name:           "create flags interactive",
			hasCreateFlags: true,
			interactive:    true,
			want:           false,
		},
		{
			name:           "create flags with edit",
			hasCreateFlags: true,
			editFlag:       true,
			interactive:    true,
			want:           true,
		},
		{
			name:        "no flags with edit",
			editFlag:    true,
			interactive: false,
			want:        true,
		},
		{
			name:        "no flags with no-edit",
			noEditFlag:  true,
			interactive: true,
			want:        false,
		},
		{
			name:           "create flags with no-edit",
			hasCreateFlags: true,
			noEditFlag:     true,
			interactive:    true,
			want:           false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldUseEditor(tc.hasCreateFlags, tc.editFlag, tc.noEditFlag, tc.interactive)
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestTodoListPriorityFilter(t *testing.T) {
	valid := todo.PriorityMedium
	priority, err := todoListPriorityFilter(valid, true)
	if err != nil {
		t.Fatalf("expected valid priority, got error: %v", err)
	}
	if priority == nil || *priority != valid {
		t.Fatalf("expected priority %d, got %v", valid, priority)
	}

	priority, err = todoListPriorityFilter(-1, false)
	if err != nil {
		t.Fatalf("expected no error when priority not set, got %v", err)
	}
	if priority != nil {
		t.Fatalf("expected nil priority when not set, got %v", priority)
	}

	_, err = todoListPriorityFilter(-1, true)
	if err == nil || !errors.Is(err, todo.ErrInvalidPriority) {
		t.Fatalf("expected invalid priority error, got %v", err)
	}
}

func TestPrintTodoDetailIncludesDeleteMetadata(t *testing.T) {
	deletedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	item := todo.Todo{
		ID:           "abc12345",
		Title:        "Deprecated",
		Type:         todo.TypeTask,
		Status:       todo.StatusTombstone,
		Priority:     todo.PriorityLow,
		CreatedAt:    time.Date(2026, 1, 1, 1, 2, 3, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 1, 1, 2, 3, 4, 0, time.UTC),
		DeletedAt:    &deletedAt,
		DeleteReason: "no longer needed",
	}

	output := captureStdout(t, func() {
		printTodoDetail(item, func(id string) string { return id })
	})

	if !strings.Contains(output, "Deleted:  2026-01-02 03:04:05") {
		t.Fatalf("expected deleted timestamp in output, got: %q", output)
	}
	if !strings.Contains(output, "Delete Reason: no longer needed") {
		t.Fatalf("expected delete reason in output, got: %q", output)
	}
}

func TestPrintTodoDetailIncludesModels(t *testing.T) {
	item := todo.Todo{
		ID:                  "abc12345",
		Title:               "Modelled",
		Type:                todo.TypeTask,
		Status:              todo.StatusOpen,
		Priority:            todo.PriorityLow,
		CreatedAt:           time.Date(2026, 1, 1, 1, 2, 3, 0, time.UTC),
		UpdatedAt:           time.Date(2026, 1, 1, 2, 3, 4, 0, time.UTC),
		ImplementationModel: "impl-model",
		CodeReviewModel:     "review-model",
		ProjectReviewModel:  "project-model",
	}

	output := captureStdout(t, func() {
		printTodoDetail(item, func(id string) string { return id })
	})

	if !strings.Contains(output, "Implementation Model: impl-model") {
		t.Fatalf("expected implementation model in output, got: %q", output)
	}
	if !strings.Contains(output, "Code Review Model: review-model") {
		t.Fatalf("expected code review model in output, got: %q", output)
	}
	if !strings.Contains(output, "Project Review Model: project-model") {
		t.Fatalf("expected project review model in output, got: %q", output)
	}
}

func TestPrintTodoDetailRendersMarkdownDescription(t *testing.T) {
	item := todo.Todo{
		ID:          "abc12345",
		Title:       "Rendered",
		Type:        todo.TypeTask,
		Status:      todo.StatusOpen,
		Priority:    todo.PriorityLow,
		CreatedAt:   time.Date(2026, 1, 1, 1, 2, 3, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 1, 1, 2, 3, 4, 0, time.UTC),
		Description: "Checklist:\n\n- First item\n- Second item\n\n```bash\necho first\necho second\n```",
	}

	output := captureStdout(t, func() {
		printTodoDetail(item, func(id string) string { return id })
	})

	checks := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^Description:$`),
		regexp.MustCompile(`(?m)^Checklist:$`),
		regexp.MustCompile(`(?m)^\s*- First item$`),
		regexp.MustCompile(`(?m)^\s*- Second item$`),
		regexp.MustCompile(`(?m)^\s*echo first$`),
		regexp.MustCompile(`(?m)^\s*echo second$`),
	}
	for _, check := range checks {
		if !check.MatchString(output) {
			t.Fatalf("expected markdown output to match %q, got %q", check.String(), output)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	defer func() {
		os.Stdout = old
		_ = r.Close()
	}()

	os.Stdout = w
	fn()
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}

	return buf.String()
}
