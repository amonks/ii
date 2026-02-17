// Package todo implements a Jira-like todo tracker for jujutsu repositories.
//
// Todos are stored in a special orphan change (parented at root()) that is
// bookmarked as "incr/tasks". This allows todos to sync across workspaces
// without polluting the code history.
//
// The public API mirrors the CLI commands:
//   - Create, Update, Start, Close, Finish, Reopen for todo lifecycle
//   - Show, List, Ready for querying
//   - DepAdd, DepTree for dependency management
package todo

import "github.com/amonks/incrementum/internal/validation"

// Status represents the state of a todo.
type Status string

const (
	// StatusOpen indicates the todo is ready to be worked on.
	StatusOpen Status = "open"

	// StatusProposed indicates the todo is awaiting review before starting.
	StatusProposed Status = "proposed"

	// StatusQueued indicates the todo is queued for batch processing.
	// Used by `job do` when multiple todos are specified to show that work
	// on this todo is scheduled. The CLI is responsible for cleanup:
	// queued todos must be reset to open if the batch exits before processing them.
	StatusQueued Status = "queued"

	// StatusInProgress indicates the todo is currently being worked on.
	StatusInProgress Status = "in_progress"

	// StatusClosed indicates the todo has been completed.
	StatusClosed Status = "closed"

	// StatusDone indicates the todo is finished without closing it.
	StatusDone Status = "done"

	// StatusWaiting indicates the todo is blocked on external factors.
	// Unlike dependency blocking (for internal task ordering), waiting is for
	// external factors like upstream PRs, API availability, etc. The reason
	// for waiting lives in the description field.
	StatusWaiting Status = "waiting"

	// StatusStuck indicates the todo has been marked as stuck.
	// Stuck todos are not considered ready and indicate that automated work
	// should not continue without human intervention.
	StatusStuck Status = "stuck"

	// StatusTombstone indicates the todo has been soft-deleted.
	StatusTombstone Status = "tombstone"
)

// ValidStatuses returns all valid status values.
func ValidStatuses() []Status {
	return []Status{StatusOpen, StatusProposed, StatusQueued, StatusInProgress, StatusClosed, StatusDone, StatusWaiting, StatusStuck, StatusTombstone}
}

// IsValid returns true if the status is a known valid value.
func (s Status) IsValid() bool {
	return validation.IsValidValue(s, ValidStatuses())
}

// IsResolved returns true when a status is considered resolved for dependencies.
func (s Status) IsResolved() bool {
	switch s {
	case StatusClosed, StatusDone, StatusStuck, StatusTombstone:
		return true
	default:
		return false
	}
}

// TodoType represents the category of a todo.
type TodoType string

const (
	// TypeTask is a general task (default).
	TypeTask TodoType = "task"

	// TypeBug is a bug fix.
	TypeBug TodoType = "bug"

	// TypeFeature is a new feature.
	TypeFeature TodoType = "feature"

	// TypeDesign is a design or specification task requiring interactive work.
	TypeDesign TodoType = "design"
)

// ValidTodoTypes returns all valid todo type values.
func ValidTodoTypes() []TodoType {
	return []TodoType{TypeTask, TypeBug, TypeFeature, TypeDesign}
}

// IsValid returns true if the type is a known valid value.
func (t TodoType) IsValid() bool {
	return validation.IsValidValue(t, ValidTodoTypes())
}

// IsInteractive returns true for todo types that require interactive sessions.
// Design todos produce specifications and require user collaboration.
func (t TodoType) IsInteractive() bool {
	return t == TypeDesign
}

// TodoTypeRank returns the sort rank for a todo type.
func TodoTypeRank(t TodoType) int {
	switch t {
	case TypeBug:
		return 0
	case TypeTask:
		return 1
	case TypeFeature:
		return 2
	case TypeDesign:
		return 3
	default:
		return 4
	}
}

// Priority constants for todos.
const (
	PriorityCritical = 0
	PriorityHigh     = 1
	PriorityMedium   = 2 // default
	PriorityLow      = 3
	PriorityBacklog  = 4

	PriorityMin = 0
	PriorityMax = 4
)

// PriorityName returns a human-readable name for the priority level.
func PriorityName(p int) string {
	switch p {
	case PriorityCritical:
		return "critical"
	case PriorityHigh:
		return "high"
	case PriorityMedium:
		return "medium"
	case PriorityLow:
		return "low"
	case PriorityBacklog:
		return "backlog"
	default:
		return "unknown"
	}
}

// MaxTitleLength is the maximum allowed length for a todo title.
const MaxTitleLength = 500
