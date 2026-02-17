// Package state manages the shared incrementum state file.
//
// The state file (~/.local/state/incrementum/state.json) stores persistent
// state for workspaces, agent sessions, and jobs. All access is
// serialized through file locking to allow safe concurrent access from
// multiple processes.
package state

import (
	"time"

	"github.com/amonks/incrementum/internal/validation"
)

// State represents the persisted state file.
type State struct {
	Repos         map[string]RepoInfo      `json:"repos"`
	Workspaces    map[string]WorkspaceInfo `json:"workspaces"`
	AgentSessions map[string]AgentSession  `json:"agent_sessions"`
	Jobs          map[string]Job           `json:"jobs"`
}

// RepoInfo stores information about a tracked repository.
type RepoInfo struct {
	SourcePath string `json:"source_path"`
}

// WorkspaceStatus represents the state of a workspace.
type WorkspaceStatus string

const (
	// WorkspaceStatusAvailable indicates the workspace is free to be acquired.
	WorkspaceStatusAvailable WorkspaceStatus = "available"
	// WorkspaceStatusAcquired indicates the workspace is currently in use.
	WorkspaceStatusAcquired WorkspaceStatus = "acquired"
)

// ValidWorkspaceStatuses returns all valid workspace status values.
func ValidWorkspaceStatuses() []WorkspaceStatus {
	return []WorkspaceStatus{WorkspaceStatusAvailable, WorkspaceStatusAcquired}
}

// IsValid returns true if the status is a known value.
func (s WorkspaceStatus) IsValid() bool {
	return validation.IsValidValue(s, ValidWorkspaceStatuses())
}

// WorkspaceInfo stores information about a workspace.
type WorkspaceInfo struct {
	Name          string          `json:"name"`
	Repo          string          `json:"repo"`
	Path          string          `json:"path"`
	Purpose       string          `json:"purpose,omitempty"`
	Rev           string          `json:"rev,omitempty"`
	Status        WorkspaceStatus `json:"status"`
	AcquiredByPID int             `json:"acquired_by_pid,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	AcquiredAt    time.Time       `json:"acquired_at"`
	Provisioned   bool            `json:"provisioned"`
}

// AgentSessionStatus represents the state of an agent session.
type AgentSessionStatus string

const (
	// AgentSessionActive indicates the session is active.
	AgentSessionActive AgentSessionStatus = "active"
	// AgentSessionCompleted indicates the session completed successfully.
	AgentSessionCompleted AgentSessionStatus = "completed"
	// AgentSessionFailed indicates the session failed.
	AgentSessionFailed AgentSessionStatus = "failed"
)

// AgentSession stores agent session state for a repo.
type AgentSession struct {
	ID              string             `json:"id"`
	Repo            string             `json:"repo"`
	Status          AgentSessionStatus `json:"status"`
	Model           string             `json:"model"`
	CreatedAt       time.Time          `json:"created_at"`
	StartedAt       time.Time          `json:"started_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
	CompletedAt     time.Time          `json:"completed_at"`
	ExitCode        *int               `json:"exit_code,omitempty"`
	DurationSeconds int                `json:"duration_seconds,omitempty"`
	TokensUsed      int                `json:"tokens_used,omitempty"`
	Cost            float64            `json:"cost,omitempty"`
}

// JobStage represents the current workflow stage for a job.
type JobStage string

const (
	// JobStageImplementing indicates the LLM implementation stage.
	JobStageImplementing JobStage = "implementing"
	// JobStageTesting indicates the test execution stage.
	JobStageTesting JobStage = "testing"
	// JobStageReviewing indicates the LLM review stage.
	JobStageReviewing JobStage = "reviewing"
	// JobStageCommitting indicates the commit message generation stage.
	JobStageCommitting JobStage = "committing"
)

// ValidJobStages returns all valid job stage values.
func ValidJobStages() []JobStage {
	return []JobStage{JobStageImplementing, JobStageTesting, JobStageReviewing, JobStageCommitting}
}

// IsValid returns true if the stage is a known value.
func (s JobStage) IsValid() bool {
	return validation.IsValidValue(s, ValidJobStages())
}

// JobStatus represents the lifecycle status for a job.
type JobStatus string

const (
	// JobStatusActive indicates the job is still running.
	JobStatusActive JobStatus = "active"
	// JobStatusCompleted indicates the job completed successfully.
	JobStatusCompleted JobStatus = "completed"
	// JobStatusFailed indicates the job failed.
	JobStatusFailed JobStatus = "failed"
	// JobStatusAbandoned indicates the job was abandoned.
	JobStatusAbandoned JobStatus = "abandoned"
)

// ValidJobStatuses returns all valid job status values.
func ValidJobStatuses() []JobStatus {
	return []JobStatus{JobStatusActive, JobStatusCompleted, JobStatusFailed, JobStatusAbandoned}
}

// IsValid returns true if the status is a known value.
func (s JobStatus) IsValid() bool {
	return validation.IsValidValue(s, ValidJobStatuses())
}

// ReviewOutcome represents a review verdict.
type ReviewOutcome string

const (
	ReviewOutcomeAccept         ReviewOutcome = "ACCEPT"
	ReviewOutcomeRequestChanges ReviewOutcome = "REQUEST_CHANGES"
	ReviewOutcomeAbandon        ReviewOutcome = "ABANDON"
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

// Job stores job state for a repo.
type Job struct {
	ID                  string            `json:"id"`
	Repo                string            `json:"repo"`
	TodoID              string            `json:"todo_id"`
	Agent               string            `json:"agent"`
	ImplementationModel string            `json:"implementation_model,omitempty"`
	CodeReviewModel     string            `json:"code_review_model,omitempty"`
	ProjectReviewModel  string            `json:"project_review_model,omitempty"`
	Stage               JobStage          `json:"stage"`
	Feedback            string            `json:"feedback,omitempty"`
	AgentSessions       []JobAgentSession `json:"agent_sessions,omitempty"`
	// Changes created by this job, in order of creation.
	Changes []JobChange `json:"changes,omitempty"`
	// ProjectReview captures the final project review (after all changes complete).
	ProjectReview *JobReview `json:"project_review,omitempty"`
	Status        JobStatus  `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	StartedAt     time.Time  `json:"started_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	CompletedAt   time.Time  `json:"completed_at"`
}

// CurrentChange returns the current in-progress change.
//
// Note: this is derived solely from change/commit review outcomes and does not
// gate on Job status/stage (e.g. an abandoned job can still have a non-nil
// CurrentChange). Callers must ensure stage/status allow iterating.
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
