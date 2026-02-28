package job

import "testing"

func TestAliasesMatchJobModel(t *testing.T) {
	var status Status = StatusActive
	if status != StatusActive {
		t.Fatalf("expected status alias to match model")
	}

	var stage Stage = StageImplementing
	if stage != StageImplementing {
		t.Fatalf("expected stage alias to match model")
	}

	var item Job
	if item.ID != "" {
		t.Fatalf("expected job alias to match model")
	}
}

func TestValidStagesReturnsModelValues(t *testing.T) {
	stages := ValidStages()
	if len(stages) != 4 {
		t.Fatalf("expected 4 stages, got %d", len(stages))
	}
}

func TestValidStatusesReturnsModelValues(t *testing.T) {
	statuses := ValidStatuses()
	if len(statuses) != 4 {
		t.Fatalf("expected 4 statuses, got %d", len(statuses))
	}
}
