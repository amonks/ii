package job

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/ids"
	"github.com/amonks/incrementum/internal/paths"
	statestore "github.com/amonks/incrementum/internal/state"
	internalstrings "github.com/amonks/incrementum/internal/strings"
)

// StaleJobTimeout is the duration after which an active job is considered stale
// and should be marked as failed. Jobs that haven't been updated within this
// duration are assumed to be orphaned (e.g., the process crashed or was killed).
const StaleJobTimeout = 10 * time.Minute

// OpenOptions configures a job manager.
type OpenOptions struct {
	// StateDir is the directory where job state is stored.
	StateDir string
}

// Manager provides access to job state for a repo.
type Manager struct {
	repoPath   string
	stateStore *statestore.Store
}

// Open opens a job manager for the given repo.
func Open(repoPath string, opts OpenOptions) (*Manager, error) {
	stateDir, err := resolveStateDir(opts)
	if err != nil {
		return nil, err
	}

	return &Manager{
		repoPath:   repoPath,
		stateStore: statestore.NewStore(stateDir),
	}, nil
}

// CreateOptions configures new job creation.
type CreateOptions struct {
	Agent               string
	ImplementationModel string
	CodeReviewModel     string
	ProjectReviewModel  string
}

// Create stores a new job with active status and implementing stage.
func (m *Manager) Create(todoID string, startedAt time.Time, opts CreateOptions) (Job, error) {
	if internalstrings.IsBlank(todoID) {
		return Job{}, fmt.Errorf("todo id is required")
	}

	repoName, err := m.stateStore.GetOrCreateRepoName(m.repoPath)
	if err != nil {
		return Job{}, fmt.Errorf("get repo name: %w", err)
	}

	jobID := GenerateID(todoID, startedAt)
	created := Job{
		ID:                  jobID,
		Repo:                repoName,
		TodoID:              todoID,
		Agent:               internalstrings.TrimSpace(opts.Agent),
		ImplementationModel: internalstrings.TrimSpace(opts.ImplementationModel),
		CodeReviewModel:     internalstrings.TrimSpace(opts.CodeReviewModel),
		ProjectReviewModel:  internalstrings.TrimSpace(opts.ProjectReviewModel),
		Stage:               StageImplementing,
		Status:              StatusActive,
		CreatedAt:           startedAt,
		StartedAt:           startedAt,
		UpdatedAt:           startedAt,
	}

	err = m.stateStore.Update(func(st *statestore.State) error {
		st.Jobs[repoName+"/"+jobID] = created
		return nil
	})
	if err != nil {
		return Job{}, err
	}

	return created, nil
}

// UpdateOptions configures job updates.
// Nil fields mean "do not update".
type UpdateOptions struct {
	Stage              *Stage
	Status             *Status
	Feedback           *string
	AppendAgentSession *AgentSession
}

// Update updates an existing job by id or prefix.
func (m *Manager) Update(jobID string, opts UpdateOptions, updatedAt time.Time) (Job, error) {
	if internalstrings.IsBlank(jobID) {
		return Job{}, ErrJobNotFound
	}

	if opts.Stage != nil {
		normalized := normalizeStage(*opts.Stage)
		opts.Stage = &normalized
		if !opts.Stage.IsValid() {
			return Job{}, formatInvalidStageError(*opts.Stage)
		}
	}
	if opts.Status != nil {
		normalized := normalizeStatus(*opts.Status)
		opts.Status = &normalized
		if !opts.Status.IsValid() {
			return Job{}, formatInvalidStatusError(*opts.Status)
		}
	}

	found, err := m.Find(jobID)
	if err != nil {
		return Job{}, err
	}

	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	var updated Job
	err = m.stateStore.Update(func(st *statestore.State) error {
		key := found.Repo + "/" + found.ID
		job, ok := st.Jobs[key]
		if !ok {
			return ErrJobNotFound
		}
		if opts.Stage != nil {
			job.Stage = *opts.Stage
		}
		if opts.Status != nil {
			job.Status = *opts.Status
			if job.Status != StatusActive {
				job.CompletedAt = updatedAt
			}
		}
		if opts.Feedback != nil {
			job.Feedback = *opts.Feedback
		}
		if opts.AppendAgentSession != nil {
			job.AgentSessions = append(job.AgentSessions, statestore.JobAgentSession{
				Purpose: opts.AppendAgentSession.Purpose,
				ID:      opts.AppendAgentSession.ID,
			})
		}
		job.UpdatedAt = updatedAt
		st.Jobs[key] = job
		updated = job
		return nil
	})
	if err != nil {
		return Job{}, err
	}

	return updated, nil
}

// JobCommitUpdate describes in-place updates to the current commit.
// Nil fields mean "do not update".
type JobCommitUpdate struct {
	TestsPassed *bool
	Review      *JobReview
}

// AppendChange appends a change to the job.
func (m *Manager) AppendChange(jobID string, change JobChange, now time.Time) (Job, error) {
	found, err := m.Find(jobID)
	if err != nil {
		return Job{}, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	if change.CreatedAt.IsZero() {
		change.CreatedAt = now
	}
	if change.Commits == nil {
		change.Commits = make([]JobCommit, 0)
	}

	var updated Job
	err = m.stateStore.Update(func(st *statestore.State) error {
		key := found.Repo + "/" + found.ID
		job, ok := st.Jobs[key]
		if !ok {
			return ErrJobNotFound
		}
		job.Changes = append(job.Changes, change)
		job.UpdatedAt = now
		st.Jobs[key] = job
		updated = job
		return nil
	})
	if err != nil {
		return Job{}, err
	}

	return updated, nil
}

// AppendCommitToCurrentChange appends a commit to the job's current change.
// Returns ErrNoCurrentChange if there are no changes, or if the last change is complete.
func (m *Manager) AppendCommitToCurrentChange(jobID string, commit JobCommit, now time.Time) (Job, error) {
	found, err := m.Find(jobID)
	if err != nil {
		return Job{}, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	if commit.CreatedAt.IsZero() {
		commit.CreatedAt = now
	}

	var updated Job
	err = m.stateStore.Update(func(st *statestore.State) error {
		key := found.Repo + "/" + found.ID
		job, ok := st.Jobs[key]
		if !ok {
			return ErrJobNotFound
		}
		if len(job.Changes) == 0 {
			return ErrNoCurrentChange
		}
		idx := len(job.Changes) - 1
		if job.Changes[idx].IsComplete() {
			return ErrNoCurrentChange
		}
		job.Changes[idx].Commits = append(job.Changes[idx].Commits, commit)
		job.UpdatedAt = now
		st.Jobs[key] = job
		updated = job
		return nil
	})
	if err != nil {
		return Job{}, err
	}

	return updated, nil
}

// UpdateCurrentCommit updates the current in-progress commit.
// Returns ErrNoCurrentChange if there are no changes, or if the last change is complete.
// Returns ErrNoCurrentCommit if the current change has no commits.
func (m *Manager) UpdateCurrentCommit(jobID string, update JobCommitUpdate, now time.Time) (Job, error) {
	found, err := m.Find(jobID)
	if err != nil {
		return Job{}, err
	}
	if now.IsZero() {
		now = time.Now()
	}

	var updated Job
	err = m.stateStore.Update(func(st *statestore.State) error {
		key := found.Repo + "/" + found.ID
		job, ok := st.Jobs[key]
		if !ok {
			return ErrJobNotFound
		}
		if len(job.Changes) == 0 {
			return ErrNoCurrentChange
		}
		changeIdx := len(job.Changes) - 1
		if job.Changes[changeIdx].IsComplete() {
			return ErrNoCurrentChange
		}
		if len(job.Changes[changeIdx].Commits) == 0 {
			return ErrNoCurrentCommit
		}
		commitIdx := len(job.Changes[changeIdx].Commits) - 1
		commit := job.Changes[changeIdx].Commits[commitIdx]
		if update.TestsPassed != nil {
			v := *update.TestsPassed
			commit.TestsPassed = &v
		}
		if update.Review != nil {
			review := *update.Review
			if review.ReviewedAt.IsZero() {
				review.ReviewedAt = now
			}
			commit.Review = &review
		}
		job.Changes[changeIdx].Commits[commitIdx] = commit
		job.UpdatedAt = now
		st.Jobs[key] = job
		updated = job
		return nil
	})
	if err != nil {
		return Job{}, err
	}

	return updated, nil
}

// SetProjectReview sets the project's final review on the job.
func (m *Manager) SetProjectReview(jobID string, review JobReview, now time.Time) (Job, error) {
	found, err := m.Find(jobID)
	if err != nil {
		return Job{}, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	if review.ReviewedAt.IsZero() {
		review.ReviewedAt = now
	}

	var updated Job
	err = m.stateStore.Update(func(st *statestore.State) error {
		key := found.Repo + "/" + found.ID
		job, ok := st.Jobs[key]
		if !ok {
			return ErrJobNotFound
		}
		copied := review
		job.ProjectReview = &copied
		job.UpdatedAt = now
		st.Jobs[key] = job
		updated = job
		return nil
	})
	if err != nil {
		return Job{}, err
	}

	return updated, nil
}

// ListFilter configures which jobs to return.
type ListFilter struct {
	// Status filters by exact status match.
	Status *Status
	// IncludeAll includes jobs regardless of status.
	IncludeAll bool
}

// List returns jobs for the repo.
func (m *Manager) List(filter ListFilter) ([]Job, error) {
	if filter.Status != nil {
		normalized := normalizeStatus(*filter.Status)
		filter.Status = &normalized
		if !filter.Status.IsValid() {
			return nil, formatInvalidStatusError(*filter.Status)
		}
	}

	repoName, err := m.stateStore.GetOrCreateRepoName(m.repoPath)
	if err != nil {
		return nil, fmt.Errorf("get repo name: %w", err)
	}

	st, err := m.stateStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	items := make([]Job, 0)
	for _, job := range st.Jobs {
		if job.Repo != repoName {
			continue
		}
		if filter.Status != nil {
			if job.Status != *filter.Status {
				continue
			}
		} else if !filter.IncludeAll && job.Status != StatusActive {
			continue
		}
		items = append(items, job)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].StartedAt.Equal(items[j].StartedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].StartedAt.Before(items[j].StartedAt)
	})

	return items, nil
}

// Find returns the job with the given id or prefix for the repo.
func (m *Manager) Find(jobID string) (Job, error) {
	if jobID == "" {
		return Job{}, ErrJobNotFound
	}

	repoName, err := m.stateStore.GetOrCreateRepoName(m.repoPath)
	if err != nil {
		return Job{}, fmt.Errorf("get repo name: %w", err)
	}

	st, err := m.stateStore.Load()
	if err != nil {
		return Job{}, fmt.Errorf("load state: %w", err)
	}

	jobIDs := make([]string, 0, len(st.Jobs))
	jobsByID := make(map[string]Job, len(st.Jobs))
	for _, job := range st.Jobs {
		if job.Repo != repoName {
			continue
		}
		jobIDs = append(jobIDs, job.ID)
		jobsByID[job.ID] = job
	}

	matchID, found, ambiguous := ids.MatchPrefix(jobIDs, jobID)
	if ambiguous {
		return Job{}, fmt.Errorf("%w: %s", ErrAmbiguousJobIDPrefix, jobID)
	}
	if !found {
		return Job{}, ErrJobNotFound
	}

	return jobsByID[matchID], nil
}

// MarkStaleJobsFailed finds active jobs that haven't been updated within the
// StaleJobTimeout and marks them as failed. Returns the number of jobs marked.
func (m *Manager) MarkStaleJobsFailed(now time.Time) (int, error) {
	repoName, err := m.stateStore.GetOrCreateRepoName(m.repoPath)
	if err != nil {
		return 0, fmt.Errorf("get repo name: %w", err)
	}

	cutoff := now.Add(-StaleJobTimeout)
	marked := 0

	err = m.stateStore.Update(func(st *statestore.State) error {
		for key, job := range st.Jobs {
			if job.Repo != repoName {
				continue
			}
			if job.Status != StatusActive {
				continue
			}
			if job.UpdatedAt.After(cutoff) {
				continue
			}
			// Job is stale - mark it as failed
			job.Status = StatusFailed
			job.CompletedAt = now
			job.UpdatedAt = now
			st.Jobs[key] = job
			marked++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	return marked, nil
}

// IsJobStale returns true if the job is active but hasn't been updated within
// the StaleJobTimeout.
func IsJobStale(job Job, now time.Time) bool {
	if job.Status != StatusActive {
		return false
	}
	cutoff := now.Add(-StaleJobTimeout)
	return !job.UpdatedAt.After(cutoff)
}

// CountByHabit returns a map of habit name to job count for all habits in the repo.
// Jobs for habits have TodoID formatted as "habit:<name>".
func (m *Manager) CountByHabit() (map[string]int, error) {
	repoName, err := m.stateStore.GetOrCreateRepoName(m.repoPath)
	if err != nil {
		return nil, fmt.Errorf("get repo name: %w", err)
	}

	st, err := m.stateStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	counts := make(map[string]int)
	const habitPrefix = "habit:"
	for _, job := range st.Jobs {
		if job.Repo != repoName {
			continue
		}
		if !strings.HasPrefix(job.TodoID, habitPrefix) {
			continue
		}
		habitName := strings.TrimPrefix(job.TodoID, habitPrefix)
		counts[habitName]++
	}

	return counts, nil
}

func resolveStateDir(opts OpenOptions) (string, error) {
	return paths.ResolveWithDefault(opts.StateDir, paths.DefaultStateDir)
}
