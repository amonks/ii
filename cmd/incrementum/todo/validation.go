package todo

import (
	"errors"
	"fmt"
	"unicode/utf8"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/internal/validation"
)

var (
	// ErrEmptyTitle is returned when a todo title is empty.
	ErrEmptyTitle = errors.New("title cannot be empty")

	// ErrTitleTooLong is returned when a todo title exceeds MaxTitleLength.
	ErrTitleTooLong = errors.New("title exceeds maximum length")

	// ErrInvalidStatus is returned when an invalid status is provided.
	ErrInvalidStatus = errors.New("invalid status")

	// ErrInvalidPriority is returned when priority is outside valid range.
	ErrInvalidPriority = errors.New("priority must be between 0 and 4")

	// ErrInvalidType is returned when an invalid todo type is provided.
	ErrInvalidType = errors.New("invalid todo type")

	// ErrTodoNotFound is returned when a todo with the given ID doesn't exist.
	ErrTodoNotFound = errors.New("todo not found")

	// ErrAmbiguousTodoIDPrefix is returned when an ID prefix matches multiple todos.
	ErrAmbiguousTodoIDPrefix = errors.New("ambiguous todo ID prefix")

	// ErrSelfDependency is returned when trying to create a dependency on itself.
	ErrSelfDependency = errors.New("todo cannot depend on itself")

	// ErrEmptyDependencyTodoID is returned when a dependency lacks a todo ID.
	ErrEmptyDependencyTodoID = errors.New("todo_id cannot be empty")

	// ErrEmptyDependencyDependsOnID is returned when a dependency lacks a depends-on ID.
	ErrEmptyDependencyDependsOnID = errors.New("depends_on_id cannot be empty")

	// ErrDuplicateDependency is returned when the dependency already exists.
	ErrDuplicateDependency = errors.New("dependency already exists")

	// ErrNoTodoStore is returned when the todo store bookmark doesn't exist.
	ErrNoTodoStore = errors.New("no todo store found (bookmark incr/tasks does not exist)")

	// ErrReadOnlyStore is returned when attempting to write using a read-only store.
	ErrReadOnlyStore = errors.New("todo store opened read-only")

	// ErrClosedTodoMissingClosedAt is returned when a closed or done todo has no closed_at timestamp.
	ErrClosedTodoMissingClosedAt = errors.New("closed or done todo must have closed_at timestamp")

	// ErrNotClosedTodoHasClosedAt is returned when a non-closed todo has a closed_at timestamp.
	ErrNotClosedTodoHasClosedAt = errors.New("non-closed todo cannot have closed_at timestamp")

	// ErrDeleteReasonRequiresDeletedAt is returned when a delete reason is provided without deleted_at.
	ErrDeleteReasonRequiresDeletedAt = errors.New("delete reason requires deleted_at timestamp")

	// ErrTombstoneMissingDeletedAt is returned when a tombstone todo has no deleted_at timestamp.
	ErrTombstoneMissingDeletedAt = errors.New("tombstone todo must have deleted_at timestamp")

	// ErrDeletedAtRequiresTombstoneStatus is returned when deleted_at is set without tombstone status.
	ErrDeletedAtRequiresTombstoneStatus = errors.New("deleted_at requires tombstone status")

	// ErrDeleteReasonRequiresTombstoneStatus is returned when delete reason is set without tombstone status.
	ErrDeleteReasonRequiresTombstoneStatus = errors.New("delete reason requires tombstone status")

	// ErrTombstoneHasClosedAt is returned when a tombstone todo has a closed_at timestamp.
	ErrTombstoneHasClosedAt = errors.New("tombstone todo cannot have closed_at timestamp")

	// ErrStartedAtRequiresActiveStatus is returned when started_at is set for a non-active todo.
	ErrStartedAtRequiresActiveStatus = errors.New("started_at requires in_progress, queued_for_merge, merging, merge_failed, or done status")

	// ErrCompletedAtRequiresDoneStatus is returned when completed_at is set for a non-complete todo.
	ErrCompletedAtRequiresDoneStatus = errors.New("completed_at requires queued_for_merge, merging, merge_failed, or done status")

	// ErrMergeStatusMissingCompletedAt is returned when a merge status is missing completed_at.
	ErrMergeStatusMissingCompletedAt = errors.New("merge statuses require completed_at")

	// ErrJobIDRequiresMergeStatus is returned when job_id is set for a non-merge todo.
	ErrJobIDRequiresMergeStatus = errors.New("job_id requires queued_for_merge, merging, merge_failed, or done status")
)

// ValidateTitle checks if the title is valid.
func ValidateTitle(title string) error {
	if internalstrings.IsBlank(title) {
		return ErrEmptyTitle
	}
	length := utf8.RuneCountInString(title)
	if length > MaxTitleLength {
		return fmt.Errorf("%w: %d > %d", ErrTitleTooLong, length, MaxTitleLength)
	}
	return nil
}

// ValidatePriority checks if the priority is valid.
func ValidatePriority(priority int) error {
	if priority < PriorityMin || priority > PriorityMax {
		return fmt.Errorf("%w: got %d", ErrInvalidPriority, priority)
	}
	return nil
}

// ValidateTodo checks if a todo struct is valid.
func ValidateTodo(t *Todo) error {
	if err := ValidateTitle(t.Title); err != nil {
		return err
	}

	if !t.Status.IsValid() {
		return formatInvalidStatusError(t.Status)
	}

	if err := ValidatePriority(t.Priority); err != nil {
		return err
	}

	if !t.Type.IsValid() {
		return formatInvalidTypeError(t.Type)
	}

	if err := validateClosedAt(t); err != nil {
		return err
	}
	if err := validateDeletedAt(t); err != nil {
		return err
	}
	if err := validateStartedAt(t); err != nil {
		return err
	}
	if err := validateCompletedAt(t); err != nil {
		return err
	}
	if err := validateJobID(t); err != nil {
		return err
	}

	return nil
}

func validateClosedAt(t *Todo) error {
	switch t.Status {
	case StatusClosed, StatusDone:
		if t.ClosedAt == nil {
			return ErrClosedTodoMissingClosedAt
		}
	case StatusTombstone:
		if t.ClosedAt != nil {
			return ErrTombstoneHasClosedAt
		}
	default:
		if t.ClosedAt != nil {
			return ErrNotClosedTodoHasClosedAt
		}
	}
	return nil
}

func validateDeletedAt(t *Todo) error {
	if t.Status == StatusTombstone {
		if t.DeletedAt == nil {
			return ErrTombstoneMissingDeletedAt
		}
	} else {
		if t.DeletedAt != nil {
			return ErrDeletedAtRequiresTombstoneStatus
		}
		if t.DeleteReason != "" {
			return ErrDeleteReasonRequiresTombstoneStatus
		}
	}

	if t.DeletedAt == nil && t.DeleteReason != "" {
		return ErrDeleteReasonRequiresDeletedAt
	}

	return nil
}

func validateJobID(t *Todo) error {
	if t.JobID == "" {
		return nil
	}
	switch t.Status {
	case StatusQueuedForMerge, StatusMerging, StatusMergeFailed, StatusDone:
		return nil
	default:
		return ErrJobIDRequiresMergeStatus
	}
}

func validateStartedAt(t *Todo) error {
	if t.Status != StatusInProgress && t.Status != StatusQueuedForMerge && t.Status != StatusMerging && t.Status != StatusMergeFailed && t.Status != StatusDone {
		if t.StartedAt != nil {
			return ErrStartedAtRequiresActiveStatus
		}
	}
	return nil
}

func validateCompletedAt(t *Todo) error {
	switch t.Status {
	case StatusQueuedForMerge, StatusMerging, StatusMergeFailed:
		if t.CompletedAt == nil {
			return ErrMergeStatusMissingCompletedAt
		}
	case StatusDone:
		return nil
	default:
		if t.CompletedAt != nil {
			return ErrCompletedAtRequiresDoneStatus
		}
	}
	return nil
}

func normalizeStatusInput(status Status) (Status, error) {
	normalized := normalizeStatus(status)
	if !normalized.IsValid() {
		return "", formatInvalidStatusError(normalized)
	}
	return normalized, nil
}

func normalizeTodoTypeInput(todoType TodoType) (TodoType, error) {
	normalized := normalizeTodoType(todoType)
	if !normalized.IsValid() {
		return "", formatInvalidTypeError(normalized)
	}
	return normalized, nil
}

func formatInvalidStatusError(status Status) error {
	return validation.FormatInvalidValueError(ErrInvalidStatus, status, ValidStatuses())
}

func formatInvalidTypeError(todoType TodoType) error {
	return validation.FormatInvalidValueError(ErrInvalidType, todoType, ValidTodoTypes())
}

// ValidateDependency checks if a dependency is valid.
func ValidateDependency(d *Dependency) error {
	if d.TodoID == "" {
		return ErrEmptyDependencyTodoID
	}
	if d.DependsOnID == "" {
		return ErrEmptyDependencyDependsOnID
	}
	if d.TodoID == d.DependsOnID {
		return ErrSelfDependency
	}
	return nil
}
