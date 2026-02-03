package todo

import "time"

// Todo represents a single task.
type Todo struct {
	// ID is a unique identifier (8-char alphanumeric, derived from initial title + timestamp).
	ID string `json:"id"`

	// Title is the short summary of the todo (max 500 chars).
	Title string `json:"title"`

	// Description provides additional context about the todo.
	Description string `json:"description"`

	// Status is the current state of the todo.
	Status Status `json:"status"`

	// Priority is the importance level (0=critical, 4=backlog).
	Priority int `json:"priority"`

	// Type categorizes the todo (task, bug, feature, design).
	Type TodoType `json:"type"`

	// ImplementationModel selects the model for implementing this todo.
	ImplementationModel string `json:"implementation_model,omitempty"`

	// CodeReviewModel selects the model for commit review on this todo.
	CodeReviewModel string `json:"code_review_model,omitempty"`

	// ProjectReviewModel selects the model for final project review on this todo.
	ProjectReviewModel string `json:"project_review_model,omitempty"`

	// CreatedAt is when the todo was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the todo was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// ClosedAt is when the todo was closed or marked done (nil if not closed/done).
	ClosedAt *time.Time `json:"closed_at,omitempty"`

	// StartedAt is when the todo entered in_progress (nil when not tracking).
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt is when the todo completed (nil when not completed).
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// DeletedAt is when the todo was soft-deleted (nil if not deleted).
	DeletedAt *time.Time `json:"deleted_at,omitempty"`

	// DeleteReason explains why the todo was deleted.
	DeleteReason string `json:"delete_reason,omitempty"`

	// Source tracks the origin of the todo.
	// Empty or omitted means user-created. Values like "habit:<name>" indicate
	// the todo was created by running a habit.
	Source string `json:"source,omitempty"`
}
