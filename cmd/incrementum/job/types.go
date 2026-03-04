package job

import (
	"time"

	"monks.co/incrementum/internal/validation"
	"monks.co/incrementum/todo"
)

// Status represents the job lifecycle state.
type Status string

const (
	// StatusActive indicates the job is running.
	StatusActive Status = "active"
	// StatusCompleted indicates the job completed successfully.
	StatusCompleted Status = "completed"
	// StatusFailed indicates the job failed.
	StatusFailed Status = "failed"
	// StatusAbandoned indicates the job was abandoned.
	StatusAbandoned Status = "abandoned"
)

// ValidStatuses returns all valid job status values.
func ValidStatuses() []Status {
	return []Status{StatusActive, StatusCompleted, StatusFailed, StatusAbandoned}
}

// IsValid returns true if the status is a known value.
func (s Status) IsValid() bool {
	return validation.IsValidValue(s, ValidStatuses())
}

// Stage represents the job workflow stage.
type Stage string

const (
	// StageImplementing indicates the implementation stage.
	StageImplementing Stage = "implementing"
	// StageTesting indicates the test execution stage.
	StageTesting Stage = "testing"
	// StageReviewing indicates the review stage.
	StageReviewing Stage = "reviewing"
	// StageCommitting indicates the commit message stage.
	StageCommitting Stage = "committing"
)

// ValidStages returns all valid job stage values.
func ValidStages() []Stage {
	return []Stage{StageImplementing, StageTesting, StageReviewing, StageCommitting}
}

// IsValid returns true if the stage is a known value.
func (s Stage) IsValid() bool {
	return validation.IsValidValue(s, ValidStages())
}

// ReviewOutcome captures the outcome of review feedback.
type ReviewOutcome string

const (
	ReviewOutcomeAccept         ReviewOutcome = "ACCEPT"
	ReviewOutcomeAbandon        ReviewOutcome = "ABANDON"
	ReviewOutcomeRequestChanges ReviewOutcome = "REQUEST_CHANGES"
)

// ValidReviewOutcomes returns all valid review outcome values.
func ValidReviewOutcomes() []ReviewOutcome {
	return []ReviewOutcome{ReviewOutcomeAccept, ReviewOutcomeRequestChanges, ReviewOutcomeAbandon}
}

// IsValid returns true if the outcome is a known value.
func (o ReviewOutcome) IsValid() bool {
	return validation.IsValidValue(o, ValidReviewOutcomes())
}

// JobReview captures a review decision for a commit or the project.
type JobReview struct {
	Outcome        ReviewOutcome `json:"outcome"`
	Comments       string        `json:"comments,omitempty"`
	AgentSessionID string        `json:"agent_session_id"`
	ReviewedAt     time.Time     `json:"reviewed_at"`
}

// JobCommit represents one commit within a change.
type JobCommit struct {
	CommitID       string     `json:"commit_id"`
	DraftMessage   string     `json:"draft_message"`
	TestsPassed    *bool      `json:"tests_passed,omitempty"`
	Review         *JobReview `json:"review,omitempty"`
	AgentSessionID string     `json:"agent_session_id"`
	CreatedAt      time.Time  `json:"created_at"`
}

// JobChange represents a change being built up during a job.
// Maps to a jj change (stable change ID across rebases).
type JobChange struct {
	ChangeID  string      `json:"change_id"`
	Commits   []JobCommit `json:"commits"`
	CreatedAt time.Time   `json:"created_at"`
}

func (c JobChange) IsComplete() bool {
	if len(c.Commits) == 0 {
		return false
	}
	last := c.Commits[len(c.Commits)-1]
	return last.Review != nil && last.Review.Outcome == ReviewOutcomeAccept
}

// JobAgentSession tracks an LLM session started by a job.
type JobAgentSession struct {
	Purpose string `json:"purpose"`
	ID      string `json:"id"`
}

// Job captures job metadata for a todo.
type Job struct {
	ID                  string            `json:"id"`
	Repo                string            `json:"repo"`
	TodoID              string            `json:"todo_id"`
	Agent               string            `json:"agent"`
	ImplementationModel string            `json:"implementation_model,omitempty"`
	CodeReviewModel     string            `json:"code_review_model,omitempty"`
	ProjectReviewModel  string            `json:"project_review_model,omitempty"`
	Stage               Stage             `json:"stage"`
	Feedback            string            `json:"feedback,omitempty"`
	AgentSessions       []JobAgentSession `json:"agent_sessions,omitempty"`
	Changes             []JobChange       `json:"changes,omitempty"`
	ProjectReview       *JobReview        `json:"project_review,omitempty"`
	Status              Status            `json:"status"`
	CreatedAt           time.Time         `json:"created_at"`
	StartedAt           time.Time         `json:"started_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
	CompletedAt         time.Time         `json:"completed_at"`
}

// CurrentChange returns the current in-progress change.
func (j *Job) CurrentChange() *JobChange {
	if j == nil || len(j.Changes) == 0 {
		return nil
	}
	last := &j.Changes[len(j.Changes)-1]
	if last.IsComplete() {
		return nil
	}
	return last
}

// CurrentCommit returns the current in-progress commit.
func (j *Job) CurrentCommit() *JobCommit {
	change := j.CurrentChange()
	if change == nil || len(change.Commits) == 0 {
		return nil
	}
	return &change.Commits[len(change.Commits)-1]
}

// StartInfo captures context when starting a job run.
type StartInfo struct {
	JobID   string
	Workdir string
	Todo    todo.Todo
}
