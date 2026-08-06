package todo

import (
	"slices"
	"testing"
)

// TestValidStatusesHasNoMergePipelineStatuses pins the status vocabulary. The
// merge-pipeline statuses belonged to a merge driver that no longer exists;
// nothing can reach them, so they are not valid values.
func TestValidStatusesHasNoMergePipelineStatuses(t *testing.T) {
	want := []Status{
		StatusOpen, StatusProposed, StatusInProgress, StatusClosed,
		StatusDone, StatusWaiting, StatusStuck, StatusTombstone,
	}

	got := ValidStatuses()
	if !slices.Equal(got, want) {
		t.Fatalf("ValidStatuses() = %v, want %v", got, want)
	}

	for _, retired := range []Status{"queued", "queued_for_merge", "merging", "merge_failed"} {
		if retired.IsValid() {
			t.Errorf("status %q should no longer be valid", retired)
		}
	}
}
