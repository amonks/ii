package todo

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateTitle(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		wantErr error
	}{
		{"valid short", "Fix bug", nil},
		{"valid long", strings.Repeat("a", MaxTitleLength), nil},
		{"valid long unicode", strings.Repeat("a", MaxTitleLength-1) + "\u00e9", nil},
		{"empty", "", ErrEmptyTitle},
		{"whitespace", "   ", ErrEmptyTitle},
		{"too long", strings.Repeat("a", MaxTitleLength+1), ErrTitleTooLong},
		{"too long unicode", strings.Repeat("a", MaxTitleLength) + "\u00e9", ErrTitleTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTitle(tt.title)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateTitle(%q) unexpected error: %v", tt.title, err)
				}
			} else {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidateTitle(%q) = %v, want %v", tt.title, err, tt.wantErr)
				}
			}
		})
	}
}

func TestValidatePriority(t *testing.T) {
	tests := []struct {
		priority int
		wantErr  error
	}{
		{0, nil},
		{1, nil},
		{2, nil},
		{3, nil},
		{4, nil},
		{-1, ErrInvalidPriority},
		{5, ErrInvalidPriority},
		{100, ErrInvalidPriority},
	}

	for _, tt := range tests {
		t.Run(PriorityName(tt.priority), func(t *testing.T) {
			err := ValidatePriority(tt.priority)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidatePriority(%d) unexpected error: %v", tt.priority, err)
				}
			} else {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidatePriority(%d) = %v, want %v", tt.priority, err, tt.wantErr)
				}
			}
		})
	}
}

func TestValidateTodo(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		todo    Todo
		wantErr error
	}{
		{
			name: "valid open todo",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    StatusOpen,
				Priority:  2,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: nil,
		},
		{
			name: "valid proposed todo",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    StatusProposed,
				Priority:  2,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: nil,
		},
		{
			name: "valid closed todo",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    StatusClosed,
				Priority:  2,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
				ClosedAt:  &now,
			},
			wantErr: nil,
		},
		{
			name: "valid done todo",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    StatusDone,
				Priority:  2,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
				ClosedAt:  &now,
				StartedAt: &now,
				JobID:     "job-321",
			},
			wantErr: nil,
		},
		{
			name: "valid queued_for_merge todo",
			todo: Todo{
				ID:          "abc12345",
				Title:       "Fix bug",
				Status:      StatusQueuedForMerge,
				Priority:    2,
				Type:        TypeTask,
				CreatedAt:   now,
				UpdatedAt:   now,
				StartedAt:   &now,
				CompletedAt: &now,
				JobID:       "job-123",
			},
			wantErr: nil,
		},
		{
			name: "queued_for_merge without completed_at",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    StatusQueuedForMerge,
				Priority:  2,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
				StartedAt: &now,
				JobID:     "job-omit",
			},
			wantErr: ErrMergeStatusMissingCompletedAt,
		},
		{
			name: "merging without completed_at",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    StatusMerging,
				Priority:  2,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
				StartedAt: &now,
				JobID:     "job-merge",
			},
			wantErr: ErrMergeStatusMissingCompletedAt,
		},
		{
			name: "merge_failed with completed_at",
			todo: Todo{
				ID:          "abc12345",
				Title:       "Fix bug",
				Status:      StatusMergeFailed,
				Priority:    2,
				Type:        TypeTask,
				CreatedAt:   now,
				UpdatedAt:   now,
				StartedAt:   &now,
				CompletedAt: &now,
				JobID:       "job-789",
			},
			wantErr: nil,
		},
		{
			name: "job_id without merge status",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    StatusOpen,
				Priority:  2,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
				JobID:     "job-999",
			},
			wantErr: ErrJobIDRequiresMergeStatus,
		},
		{
			name: "merging without job_id",
			todo: Todo{
				ID:          "abc12345",
				Title:       "Fix bug",
				Status:      StatusMerging,
				Priority:    2,
				Type:        TypeTask,
				CreatedAt:   now,
				UpdatedAt:   now,
				StartedAt:   &now,
				CompletedAt: &now,
			},
			wantErr: nil,
		},
		{
			name: "open with started_at",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    StatusOpen,
				Priority:  2,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
				StartedAt: &now,
			},
			wantErr: ErrStartedAtRequiresActiveStatus,
		},
		{
			name: "in progress with completed_at",
			todo: Todo{
				ID:          "abc12345",
				Title:       "Fix bug",
				Status:      StatusInProgress,
				Priority:    2,
				Type:        TypeTask,
				CreatedAt:   now,
				UpdatedAt:   now,
				CompletedAt: &now,
				JobID:       "job-1",
			},
			wantErr: ErrCompletedAtRequiresDoneStatus,
		},
		{
			name: "queued_for_merge with completed_at",
			todo: Todo{
				ID:          "abc12345",
				Title:       "Fix bug",
				Status:      StatusQueuedForMerge,
				Priority:    2,
				Type:        TypeTask,
				CreatedAt:   now,
				UpdatedAt:   now,
				StartedAt:   &now,
				CompletedAt: &now,
			},
			wantErr: nil,
		},
		{
			name: "merging with completed_at",
			todo: Todo{
				ID:          "abc12345",
				Title:       "Fix bug",
				Status:      StatusMerging,
				Priority:    2,
				Type:        TypeTask,
				CreatedAt:   now,
				UpdatedAt:   now,
				StartedAt:   &now,
				CompletedAt: &now,
				JobID:       "job-2",
			},
			wantErr: nil,
		},
		{
			name: "open with completed_at",
			todo: Todo{
				ID:          "abc12345",
				Title:       "Fix bug",
				Status:      StatusOpen,
				Priority:    2,
				Type:        TypeTask,
				CreatedAt:   now,
				UpdatedAt:   now,
				CompletedAt: &now,
			},
			wantErr: ErrCompletedAtRequiresDoneStatus,
		},
		{
			name: "empty title",
			todo: Todo{
				ID:        "abc12345",
				Title:     "",
				Status:    StatusOpen,
				Priority:  2,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: ErrEmptyTitle,
		},
		{
			name: "invalid status",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    Status("invalid"),
				Priority:  2,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: ErrInvalidStatus,
		},
		{
			name: "invalid priority",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    StatusOpen,
				Priority:  10,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: ErrInvalidPriority,
		},
		{
			name: "invalid type",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    StatusOpen,
				Priority:  2,
				Type:      TodoType("invalid"),
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: ErrInvalidType,
		},
		{
			name: "closed without closed_at",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    StatusClosed,
				Priority:  2,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: ErrClosedTodoMissingClosedAt,
		},
		{
			name: "open with closed_at",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    StatusOpen,
				Priority:  2,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
				ClosedAt:  &now,
			},
			wantErr: ErrNotClosedTodoHasClosedAt,
		},
		{
			name: "tombstone missing deleted_at",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    StatusTombstone,
				Priority:  2,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: ErrTombstoneMissingDeletedAt,
		},
		{
			name: "tombstone with closed_at",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    StatusTombstone,
				Priority:  2,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
				ClosedAt:  &now,
				DeletedAt: &now,
			},
			wantErr: ErrTombstoneHasClosedAt,
		},
		{
			name: "tombstone with delete reason",
			todo: Todo{
				ID:           "abc12345",
				Title:        "Fix bug",
				Status:       StatusTombstone,
				Priority:     2,
				Type:         TypeTask,
				CreatedAt:    now,
				UpdatedAt:    now,
				DeletedAt:    &now,
				DeleteReason: "Duplicate",
			},
			wantErr: nil,
		},
		{
			name: "open with deleted_at",
			todo: Todo{
				ID:        "abc12345",
				Title:     "Fix bug",
				Status:    StatusOpen,
				Priority:  2,
				Type:      TypeTask,
				CreatedAt: now,
				UpdatedAt: now,
				DeletedAt: &now,
			},
			wantErr: ErrDeletedAtRequiresTombstoneStatus,
		},
		{
			name: "open with delete reason",
			todo: Todo{
				ID:           "abc12345",
				Title:        "Fix bug",
				Status:       StatusOpen,
				Priority:     2,
				Type:         TypeTask,
				CreatedAt:    now,
				UpdatedAt:    now,
				DeleteReason: "Duplicate",
			},
			wantErr: ErrDeleteReasonRequiresTombstoneStatus,
		},
		{
			name: "delete reason without deleted_at",
			todo: Todo{
				ID:           "abc12345",
				Title:        "Fix bug",
				Status:       StatusTombstone,
				Priority:     2,
				Type:         TypeTask,
				CreatedAt:    now,
				UpdatedAt:    now,
				DeleteReason: "Duplicate",
			},
			wantErr: ErrTombstoneMissingDeletedAt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTodo(&tt.todo)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateTodo() unexpected error: %v", err)
				}
			} else {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidateTodo() = %v, want %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestValidateDependency(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		dep     Dependency
		wantErr error
	}{
		{
			name: "missing todo id",
			dep: Dependency{
				DependsOnID: "def67890",
				CreatedAt:   now,
			},
			wantErr: ErrEmptyDependencyTodoID,
		},
		{
			name: "missing depends on id",
			dep: Dependency{
				TodoID:    "abc12345",
				CreatedAt: now,
			},
			wantErr: ErrEmptyDependencyDependsOnID,
		},
		{
			name: "valid dependency",
			dep: Dependency{
				TodoID:      "abc12345",
				DependsOnID: "def67890",
				CreatedAt:   now,
			},
			wantErr: nil,
		},
		{
			name: "self dependency",
			dep: Dependency{
				TodoID:      "abc12345",
				DependsOnID: "abc12345",
				CreatedAt:   now,
			},
			wantErr: ErrSelfDependency,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDependency(&tt.dep)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateDependency() unexpected error: %v", err)
				}
			} else {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidateDependency() = %v, want %v", err, tt.wantErr)
				}
			}
		})
	}
}
