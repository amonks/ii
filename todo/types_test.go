package todo

import "testing"

func TestStatus_IsValid(t *testing.T) {
	tests := []struct {
		status Status
		valid  bool
	}{
		{StatusOpen, true},
		{StatusProposed, true},
		{StatusQueued, true},
		{StatusInProgress, true},
		{StatusQueuedForMerge, true},
		{StatusMerging, true},
		{StatusMergeFailed, true},
		{StatusClosed, true},
		{StatusDone, true},
		{StatusWaiting, true},
		{StatusStuck, true},
		{StatusTombstone, true},
		{Status("invalid"), false},
		{Status(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.valid {
				t.Errorf("Status(%q).IsValid() = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

func TestStatus_IsResolved(t *testing.T) {
	tests := []struct {
		status   Status
		resolved bool
	}{
		{StatusOpen, false},
		{StatusProposed, false},
		{StatusQueued, false},
		{StatusInProgress, false},
		{StatusQueuedForMerge, false},
		{StatusMerging, false},
		{StatusMergeFailed, false},
		{StatusClosed, true},
		{StatusDone, true},
		{StatusWaiting, false},
		{StatusStuck, true},
		{StatusTombstone, true},
		{Status("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsResolved(); got != tt.resolved {
				t.Errorf("Status(%q).IsResolved() = %v, want %v", tt.status, got, tt.resolved)
			}
		})
	}
}

func TestTodoType_IsValid(t *testing.T) {
	tests := []struct {
		typ   TodoType
		valid bool
	}{
		{TypeTask, true},
		{TypeBug, true},
		{TypeFeature, true},
		{TypeDesign, true},
		{TodoType("invalid"), false},
		{TodoType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.typ), func(t *testing.T) {
			if got := tt.typ.IsValid(); got != tt.valid {
				t.Errorf("TodoType(%q).IsValid() = %v, want %v", tt.typ, got, tt.valid)
			}
		})
	}
}

func TestTodoType_IsInteractive(t *testing.T) {
	tests := []struct {
		typ         TodoType
		interactive bool
	}{
		{TypeTask, false},
		{TypeBug, false},
		{TypeFeature, false},
		{TypeDesign, true},
		{TodoType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.typ), func(t *testing.T) {
			if got := tt.typ.IsInteractive(); got != tt.interactive {
				t.Errorf("TodoType(%q).IsInteractive() = %v, want %v", tt.typ, got, tt.interactive)
			}
		})
	}
}

func TestPriorityName(t *testing.T) {
	tests := []struct {
		priority int
		name     string
	}{
		{PriorityCritical, "critical"},
		{PriorityHigh, "high"},
		{PriorityMedium, "medium"},
		{PriorityLow, "low"},
		{PriorityBacklog, "backlog"},
		{-1, "unknown"},
		{5, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PriorityName(tt.priority); got != tt.name {
				t.Errorf("PriorityName(%d) = %q, want %q", tt.priority, got, tt.name)
			}
		})
	}
}
