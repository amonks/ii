package job

import (
	statestore "github.com/amonks/incrementum/internal/state"
	"github.com/amonks/incrementum/todo"
)

// Status represents the job lifecycle state.
type Status = statestore.JobStatus

const (
	// StatusActive indicates the job is running.
	StatusActive Status = statestore.JobStatusActive
	// StatusCompleted indicates the job completed successfully.
	StatusCompleted Status = statestore.JobStatusCompleted
	// StatusFailed indicates the job failed.
	StatusFailed Status = statestore.JobStatusFailed
	// StatusAbandoned indicates the job was abandoned.
	StatusAbandoned Status = statestore.JobStatusAbandoned
)

// ValidStatuses returns all valid job status values.
func ValidStatuses() []Status {
	return statestore.ValidJobStatuses()
}

// Stage represents the job workflow stage.
type Stage = statestore.JobStage

const (
	// StageImplementing indicates the implementation stage.
	StageImplementing Stage = statestore.JobStageImplementing
	// StageTesting indicates the test execution stage.
	StageTesting Stage = statestore.JobStageTesting
	// StageReviewing indicates the review stage.
	StageReviewing Stage = statestore.JobStageReviewing
	// StageCommitting indicates the commit message stage.
	StageCommitting Stage = statestore.JobStageCommitting
)

// ValidStages returns all valid job stage values.
func ValidStages() []Stage {
	return statestore.ValidJobStages()
}

// ReviewOutcome captures the outcome of review feedback.
type ReviewOutcome = statestore.ReviewOutcome

const (
	ReviewOutcomeAccept         ReviewOutcome = statestore.ReviewOutcomeAccept
	ReviewOutcomeAbandon        ReviewOutcome = statestore.ReviewOutcomeAbandon
	ReviewOutcomeRequestChanges ReviewOutcome = statestore.ReviewOutcomeRequestChanges
)

// JobChange represents a change being built up during a job.
type JobChange = statestore.JobChange

// JobCommit represents one commit within a change.
type JobCommit = statestore.JobCommit

// JobReview captures a review decision for a commit or the project.
type JobReview = statestore.JobReview

// Job captures job metadata for a todo.
type Job = statestore.Job

// StartInfo captures context when starting a job run.
type StartInfo struct {
	JobID   string
	Workdir string
	Todo    todo.Todo
}
