package job

import (
	"strings"
	"testing"

	"github.com/amonks/incrementum/habit"
	"github.com/amonks/incrementum/internal/config"
)

func TestRunHabitRequiresHabitName(t *testing.T) {
	repoPath := t.TempDir()

	_, err := RunHabit(repoPath, "", HabitRunOptions{})
	if err == nil {
		t.Fatal("expected error for empty habit name")
	}
	if !strings.Contains(err.Error(), "habit name is required") {
		t.Fatalf("expected 'habit name is required' error, got: %v", err)
	}
}

func TestRunHabitRequiresHabitExists(t *testing.T) {
	repoPath := t.TempDir()

	_, err := RunHabit(repoPath, "nonexistent", HabitRunOptions{})
	if err == nil {
		t.Fatal("expected error for nonexistent habit")
	}
	if !strings.Contains(err.Error(), "habit not found") {
		t.Fatalf("expected 'habit not found' error, got: %v", err)
	}
}

func TestFormatHabitCommitMessage(t *testing.T) {
	h := &habit.Habit{
		Name:         "cleanup",
		Instructions: "Look for cleanup opportunities.\n\n- Remove dead code\n- Simplify logic",
	}

	message := "fix: remove unused function\n\nThe helper function was no longer used after refactoring."
	formatted := formatHabitCommitMessage(h, message, "")

	// Check that the summary is at the start
	if !strings.HasPrefix(formatted, "fix: remove unused function") {
		t.Errorf("expected message to start with summary, got:\n%s", formatted)
	}

	// Check that the habit attribution is included
	if !strings.Contains(formatted, "This commit was created as part of the 'cleanup' habit:") {
		t.Error("expected habit attribution in message")
	}

	// Check that the instructions are included
	if !strings.Contains(formatted, "Look for cleanup opportunities") {
		t.Error("expected habit instructions in message")
	}
}

func TestFormatHabitCommitMessageBody(t *testing.T) {
	h := &habit.Habit{
		Name:         "docs",
		Instructions: "Update docs.",
	}

	// Message with body
	message := "docs: update README\n\nAdded installation instructions."
	formatted := formatHabitCommitMessage(h, message, "")

	if !strings.Contains(formatted, "Added installation instructions") {
		t.Error("expected body in formatted message")
	}
	if !strings.Contains(formatted, "'docs' habit") {
		t.Error("expected habit name in attribution")
	}
}

func TestFormatHabitCommitMessageNoBody(t *testing.T) {
	h := &habit.Habit{
		Name:         "lint",
		Instructions: "Fix lint issues.",
	}

	// Message without body (just summary)
	message := "style: fix formatting"
	formatted := formatHabitCommitMessage(h, message, "")

	if !strings.HasPrefix(formatted, "style: fix formatting") {
		t.Error("expected summary at start")
	}
	if !strings.Contains(formatted, "'lint' habit") {
		t.Error("expected habit name in attribution")
	}
}

func TestResolveHabitModel(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *config.Config
		override   string
		habitModel string
		purpose    string
		want       string
	}{
		{
			name:     "override takes precedence",
			cfg:      &config.Config{Job: config.Job{ImplementationModel: "config-model"}},
			override: "override-model",
			want:     "override-model",
		},
		{
			name:       "habit model takes precedence over config",
			cfg:        &config.Config{Job: config.Job{ImplementationModel: "config-model"}},
			habitModel: "habit-model",
			purpose:    "implement",
			want:       "habit-model",
		},
		{
			name:    "config model used when no override or habit model",
			cfg:     &config.Config{Job: config.Job{ImplementationModel: "config-impl"}},
			purpose: "implement",
			want:    "config-impl",
		},
		{
			name:    "config review model for review purpose",
			cfg:     &config.Config{Job: config.Job{CodeReviewModel: "config-review"}},
			purpose: "review",
			want:    "config-review",
		},
		{
			name:    "falls back to model when purpose model empty",
			cfg:     &config.Config{Job: config.Job{Model: "default-model"}},
			purpose: "implement",
			want:    "default-model",
		},
		{
			name: "returns empty when no config",
			cfg:  nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveHabitModel(tt.cfg, tt.override, tt.habitModel, tt.purpose)
			if got != tt.want {
				t.Errorf("resolveHabitModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatHabitCommitMessageWithReviewComments(t *testing.T) {
	h := &habit.Habit{
		Name:         "cleanup",
		Instructions: "Look for cleanup opportunities.",
	}

	message := "fix: remove dead code\n\nRemoved unused helper function."
	reviewComments := "Good cleanup, the function was clearly unused."

	formatted := formatHabitCommitMessage(h, message, reviewComments)
	if !strings.Contains(formatted, "Review comments:") {
		t.Fatalf("expected habit commit message to include review comments section")
	}
	if !strings.Contains(formatted, "  Good cleanup, the function was clearly unused.") {
		t.Fatalf("expected habit commit message to include indented review text")
	}
}

func TestNewHabitPromptData(t *testing.T) {
	data := newHabitPromptData("cleanup", "Clean up code.", "", "", nil, "/path/to/repo")

	if data.HabitName != "cleanup" {
		t.Errorf("HabitName = %q, want %q", data.HabitName, "cleanup")
	}
	if !strings.Contains(data.HabitInstructions, "Clean up code") {
		t.Errorf("HabitInstructions should contain instructions, got: %s", data.HabitInstructions)
	}
	if data.ReviewInstructions == "" {
		t.Error("ReviewInstructions should be set")
	}
}
