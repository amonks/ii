package job

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	internaldb "monks.co/ii/internal/db"
)

func TestManager_CreateAndFind(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/my-repo"
	manager, sqlDB := openTestManager(t, repoPath, tmpDir)

	startedAt := time.Date(2025, 4, 10, 8, 30, 0, 0, time.UTC)
	created, err := manager.Create("todo-123", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	expectedID := GenerateID("todo-123", startedAt)
	if created.ID != expectedID {
		t.Fatalf("expected job id %q, got %q", expectedID, created.ID)
	}

	repoName, err := internaldb.GetOrCreateRepoName(sqlDB, repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}

	if created.Repo != repoName {
		t.Fatalf("expected repo %q, got %q", repoName, created.Repo)
	}
	if created.TodoID != "todo-123" {
		t.Fatalf("expected todo id todo-123, got %q", created.TodoID)
	}
	if created.Status != StatusActive {
		t.Fatalf("expected status active, got %q", created.Status)
	}
	if created.Stage != StageImplementing {
		t.Fatalf("expected stage implementing, got %q", created.Stage)
	}
	if !created.CreatedAt.Equal(startedAt) {
		t.Fatalf("expected created at %v, got %v", startedAt, created.CreatedAt)
	}
	if !created.StartedAt.Equal(startedAt) {
		t.Fatalf("expected started at %v, got %v", startedAt, created.StartedAt)
	}
	if !created.UpdatedAt.Equal(startedAt) {
		t.Fatalf("expected updated at %v, got %v", startedAt, created.UpdatedAt)
	}

	found, err := manager.Find(created.ID[:6])
	if err != nil {
		t.Fatalf("find job: %v", err)
	}
	if found.ID != created.ID {
		t.Fatalf("expected job id %q, got %q", created.ID, found.ID)
	}
}

func TestManager_Find_PrefixAmbiguous(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/ambiguous"
	manager, sqlDB := openTestManager(t, repoPath, tmpDir)

	repoSlug, err := internaldb.GetOrCreateRepoName(sqlDB, repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}

	startedAt := time.Date(2025, 5, 1, 12, 0, 0, 0, time.UTC)
	jobA := Job{
		ID:        "alpha-123",
		Repo:      repoSlug,
		TodoID:    "todo-1",
		Stage:     StageImplementing,
		Status:    StatusActive,
		CreatedAt: startedAt,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}
	jobB := Job{
		ID:        "alpha-456",
		Repo:      repoSlug,
		TodoID:    "todo-2",
		Stage:     StageImplementing,
		Status:    StatusActive,
		CreatedAt: startedAt.Add(2 * time.Minute),
		StartedAt: startedAt.Add(2 * time.Minute),
		UpdatedAt: startedAt.Add(2 * time.Minute),
	}

	if err := insertJob(sqlDB, jobA); err != nil {
		t.Fatalf("insert jobA: %v", err)
	}
	if err := insertJob(sqlDB, jobB); err != nil {
		t.Fatalf("insert jobB: %v", err)
	}

	_, err = manager.Find("alpha")
	if err == nil {
		t.Fatalf("expected error for ambiguous prefix")
	}
	if !errors.Is(err, ErrAmbiguousJobIDPrefix) {
		t.Fatalf("expected ErrAmbiguousJobIDPrefix, got %v", err)
	}
}

func TestManager_List_Filtering(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/listing"
	manager, sqlDB := openTestManager(t, repoPath, tmpDir)

	repoSlug, err := internaldb.GetOrCreateRepoName(sqlDB, repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}
	otherRepo, err := internaldb.GetOrCreateRepoName(sqlDB, "/Users/test/other")
	if err != nil {
		t.Fatalf("other repo slug: %v", err)
	}

	startedAt := time.Date(2025, 5, 10, 9, 0, 0, 0, time.UTC)
	activeJob := Job{
		ID:        "job-active",
		Repo:      repoSlug,
		TodoID:    "todo-active",
		Stage:     StageTesting,
		Status:    StatusActive,
		CreatedAt: startedAt,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}
	completedJob := Job{
		ID:          "job-completed",
		Repo:        repoSlug,
		TodoID:      "todo-completed",
		Stage:       StageCommitting,
		Status:      StatusCompleted,
		CreatedAt:   startedAt.Add(2 * time.Hour),
		StartedAt:   startedAt.Add(2 * time.Hour),
		UpdatedAt:   startedAt.Add(2 * time.Hour),
		CompletedAt: startedAt.Add(3 * time.Hour),
	}
	otherJob := Job{
		ID:        "job-other",
		Repo:      otherRepo,
		TodoID:    "todo-other",
		Stage:     StageImplementing,
		Status:    StatusActive,
		CreatedAt: startedAt.Add(30 * time.Minute),
		StartedAt: startedAt.Add(30 * time.Minute),
		UpdatedAt: startedAt.Add(30 * time.Minute),
	}

	if err := insertJob(sqlDB, activeJob); err != nil {
		t.Fatalf("insert active job: %v", err)
	}
	if err := insertJob(sqlDB, completedJob); err != nil {
		t.Fatalf("insert completed job: %v", err)
	}
	if err := insertJob(sqlDB, otherJob); err != nil {
		t.Fatalf("insert other job: %v", err)
	}

	listed, err := manager.List(ListFilter{})
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != activeJob.ID {
		t.Fatalf("expected only active job, got %v", listed)
	}

	allJobs, err := manager.List(ListFilter{IncludeAll: true})
	if err != nil {
		t.Fatalf("list all jobs: %v", err)
	}
	if len(allJobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(allJobs))
	}
	if allJobs[0].ID != activeJob.ID || allJobs[1].ID != completedJob.ID {
		t.Fatalf("expected jobs ordered by start time, got %v", allJobs)
	}

	status := Status("COMPLETED")
	completedOnly, err := manager.List(ListFilter{Status: &status})
	if err != nil {
		t.Fatalf("list completed jobs: %v", err)
	}
	if len(completedOnly) != 1 || completedOnly[0].ID != completedJob.ID {
		t.Fatalf("expected only completed job, got %v", completedOnly)
	}
}

func TestManager_Update(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/update"
	manager, sqlDB := openTestManager(t, repoPath, tmpDir)

	repoName, err := internaldb.GetOrCreateRepoName(sqlDB, repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}
	if err := ensureAgentSessions(sqlDB, repoName, []JobAgentSession{{ID: "oc-123", Purpose: "implement"}}); err != nil {
		t.Fatalf("ensure agent session: %v", err)
	}

	startedAt := time.Date(2025, 6, 1, 9, 30, 0, 0, time.UTC)
	created, err := manager.Create("todo-456", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	updatedAt := startedAt.Add(2 * time.Hour)
	stage := Stage("TESTING")
	status := Status("FAILED")
	feedback := "tests failed"
	agentSession := AgentSession{Purpose: "implement", ID: "oc-123"}

	updated, err := manager.Update(created.ID[:6], UpdateOptions{
		Stage:              &stage,
		Status:             &status,
		Feedback:           &feedback,
		AppendAgentSession: &agentSession,
	}, updatedAt)
	if err != nil {
		t.Fatalf("update job: %v", err)
	}

	if updated.Stage != StageTesting {
		t.Fatalf("expected stage testing, got %q", updated.Stage)
	}
	if updated.Status != StatusFailed {
		t.Fatalf("expected status failed, got %q", updated.Status)
	}
	if updated.Feedback != feedback {
		t.Fatalf("expected feedback %q, got %q", feedback, updated.Feedback)
	}
	if len(updated.AgentSessions) != 1 {
		t.Fatalf("expected 1 agent session, got %d", len(updated.AgentSessions))
	}
	if updated.AgentSessions[0] != agentSession {
		t.Fatalf("expected agent session %+v, got %+v", agentSession, updated.AgentSessions[0])
	}
	if !updated.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("expected updated at %v, got %v", updatedAt, updated.UpdatedAt)
	}
	if !updated.CompletedAt.Equal(updatedAt) {
		t.Fatalf("expected completed at %v, got %v", updatedAt, updated.CompletedAt)
	}

	stored, err := manager.Find(created.ID)
	if err != nil {
		t.Fatalf("find job: %v", err)
	}
	if stored.Status != StatusFailed {
		t.Fatalf("expected stored status failed, got %q", stored.Status)
	}
}

func TestManager_Update_InvalidStage(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/update-invalid"
	manager, _ := openTestManager(t, repoPath, tmpDir)

	startedAt := time.Date(2025, 6, 2, 11, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-789", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	stage := Stage("unknown")
	_, err = manager.Update(created.ID, UpdateOptions{Stage: &stage}, startedAt.Add(time.Minute))
	if err == nil {
		t.Fatalf("expected invalid stage error")
	}
	if !errors.Is(err, ErrInvalidStage) {
		t.Fatalf("expected ErrInvalidStage, got %v", err)
	}
}

func TestManager_Update_InvalidStatus(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/update-invalid-status"
	manager, _ := openTestManager(t, repoPath, tmpDir)

	startedAt := time.Date(2025, 6, 2, 12, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-790", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	status := Status("unknown")
	_, err = manager.Update(created.ID, UpdateOptions{Status: &status}, startedAt.Add(time.Minute))
	if err == nil {
		t.Fatalf("expected invalid status error")
	}
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestManager_ChangeTrackingLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/changes"
	manager, _ := openTestManager(t, repoPath, tmpDir)

	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-changes", now, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	change, err := manager.AppendChange(created.ID, JobChange{ChangeID: "chg-1"}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("append change: %v", err)
	}
	if len(change.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(change.Changes))
	}
	if change.Changes[0].ChangeID != "chg-1" {
		t.Fatalf("expected change id %q, got %q", "chg-1", change.Changes[0].ChangeID)
	}
	if len(change.Changes[0].Commits) != 0 {
		t.Fatalf("expected no commits initially, got %d", len(change.Changes[0].Commits))
	}

	commit := JobCommit{
		CommitID:       "commit-1",
		DraftMessage:   "feat: example",
		AgentSessionID: "ses-1",
	}
	withCommit, err := manager.AppendCommitToCurrentChange(created.ID, commit, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("append commit: %v", err)
	}
	if len(withCommit.Changes[0].Commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(withCommit.Changes[0].Commits))
	}
	if withCommit.Changes[0].Commits[0].CommitID != "commit-1" {
		t.Fatalf("expected commit id %q, got %q", "commit-1", withCommit.Changes[0].Commits[0].CommitID)
	}

	passed := true
	review := JobReview{Outcome: ReviewOutcomeAccept, Comments: "looks good", AgentSessionID: "ses-review"}
	withReview, err := manager.UpdateCurrentCommit(created.ID, JobCommitUpdate{TestsPassed: &passed, Review: &review}, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("update current commit: %v", err)
	}
	if got := withReview.Changes[0].Commits[0].TestsPassed; got == nil || *got != true {
		t.Fatalf("expected tests passed true, got %v", got)
	}
	passed = false
	if got := withReview.Changes[0].Commits[0].TestsPassed; got == nil || *got != true {
		t.Fatalf("expected tests passed true after caller mutation, got %v", got)
	}
	if withReview.Changes[0].Commits[0].Review == nil || withReview.Changes[0].Commits[0].Review.Outcome != ReviewOutcomeAccept {
		t.Fatalf("expected accepted review")
	}
	if withReview.CurrentChange() != nil {
		t.Fatalf("expected no current change after accepted review")
	}

	projectReview := JobReview{Outcome: ReviewOutcomeAccept, AgentSessionID: "ses-project"}
	final, err := manager.SetProjectReview(created.ID, projectReview, now.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("set project review: %v", err)
	}
	if final.ProjectReview == nil {
		t.Fatalf("expected project review set")
	}
	if final.ProjectReview.Outcome != ReviewOutcomeAccept {
		t.Fatalf("expected project review accept, got %q", final.ProjectReview.Outcome)
	}
}

func TestManager_ChangeTrackingInvariants(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/changes-invariants"
	manager, _ := openTestManager(t, repoPath, tmpDir)

	now := time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-changes-invariants", now, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	commit := JobCommit{CommitID: "commit-1", DraftMessage: "feat: example", AgentSessionID: "ses-1"}
	if _, err := manager.AppendCommitToCurrentChange(created.ID, commit, now.Add(time.Minute)); !errors.Is(err, ErrNoCurrentChange) {
		t.Fatalf("append commit without change: expected ErrNoCurrentChange, got %v", err)
	}
	if _, err := manager.UpdateCurrentCommit(created.ID, JobCommitUpdate{TestsPassed: new(true)}, now.Add(2*time.Minute)); !errors.Is(err, ErrNoCurrentChange) {
		t.Fatalf("update commit without change: expected ErrNoCurrentChange, got %v", err)
	}

	if _, err := manager.AppendChange(created.ID, JobChange{ChangeID: "chg-1"}, now.Add(3*time.Minute)); err != nil {
		t.Fatalf("append change: %v", err)
	}
	if _, err := manager.UpdateCurrentCommit(created.ID, JobCommitUpdate{TestsPassed: new(true)}, now.Add(4*time.Minute)); !errors.Is(err, ErrNoCurrentCommit) {
		t.Fatalf("update commit with no commits: expected ErrNoCurrentCommit, got %v", err)
	}

	withCommit, err := manager.AppendCommitToCurrentChange(created.ID, commit, now.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("append commit: %v", err)
	}
	if withCommit.CurrentCommit() == nil {
		t.Fatalf("expected current commit after append")
	}

	review := JobReview{Outcome: ReviewOutcomeAccept, Comments: "ok", AgentSessionID: "ses-review"}
	withReview, err := manager.UpdateCurrentCommit(created.ID, JobCommitUpdate{Review: &review}, now.Add(6*time.Minute))
	if err != nil {
		t.Fatalf("accept review: %v", err)
	}
	if withReview.CurrentChange() != nil {
		t.Fatalf("expected no current change after accepted review")
	}

	if _, err := manager.AppendCommitToCurrentChange(created.ID, JobCommit{CommitID: "commit-2", DraftMessage: "fix: example", AgentSessionID: "ses-2"}, now.Add(7*time.Minute)); !errors.Is(err, ErrNoCurrentChange) {
		t.Fatalf("append commit to completed change: expected ErrNoCurrentChange, got %v", err)
	}
	if _, err := manager.UpdateCurrentCommit(created.ID, JobCommitUpdate{TestsPassed: new(false)}, now.Add(8*time.Minute)); !errors.Is(err, ErrNoCurrentChange) {
		t.Fatalf("update commit on completed change: expected ErrNoCurrentChange, got %v", err)
	}

	stored, err := manager.Find(created.ID)
	if err != nil {
		t.Fatalf("find job: %v", err)
	}
	if len(stored.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(stored.Changes))
	}
	if len(stored.Changes[0].Commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(stored.Changes[0].Commits))
	}
	if stored.Changes[0].Commits[0].Review == nil || stored.Changes[0].Commits[0].Review.Outcome != ReviewOutcomeAccept {
		t.Fatalf("expected stored commit review accept")
	}
}

func TestManager_ChangeTracking_RequestChangesKeepsCurrentChange(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/changes-request-changes"
	manager, _ := openTestManager(t, repoPath, tmpDir)

	now := time.Date(2026, 1, 17, 10, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-changes-request-changes", now, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	if _, err := manager.AppendChange(created.ID, JobChange{ChangeID: "chg-1"}, now.Add(time.Minute)); err != nil {
		t.Fatalf("append change: %v", err)
	}

	commit1 := JobCommit{CommitID: "commit-1", DraftMessage: "feat: example", AgentSessionID: "ses-1"}
	withCommit1, err := manager.AppendCommitToCurrentChange(created.ID, commit1, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("append commit: %v", err)
	}
	if withCommit1.CurrentChange() == nil {
		t.Fatalf("expected current change after first commit")
	}
	if got := withCommit1.CurrentCommit(); got == nil || got.CommitID != "commit-1" {
		t.Fatalf("expected current commit commit-1, got %v", got)
	}

	review := JobReview{Outcome: ReviewOutcomeRequestChanges, Comments: "needs work", AgentSessionID: "ses-review"}
	withRequestChanges, err := manager.UpdateCurrentCommit(created.ID, JobCommitUpdate{TestsPassed: new(true), Review: &review}, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("request changes review: %v", err)
	}
	if withRequestChanges.CurrentChange() == nil {
		t.Fatalf("expected current change after REQUEST_CHANGES review")
	}
	if got := withRequestChanges.CurrentCommit(); got == nil || got.Review == nil || got.Review.Outcome != ReviewOutcomeRequestChanges {
		t.Fatalf("expected current commit reviewed REQUEST_CHANGES, got %v", got)
	}

	commit2 := JobCommit{CommitID: "commit-2", DraftMessage: "fix: example", AgentSessionID: "ses-2"}
	withCommit2, err := manager.AppendCommitToCurrentChange(created.ID, commit2, now.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("append commit after REQUEST_CHANGES: %v", err)
	}
	if withCommit2.CurrentChange() == nil {
		t.Fatalf("expected current change after second commit")
	}
	if got := withCommit2.CurrentCommit(); got == nil || got.CommitID != "commit-2" {
		t.Fatalf("expected current commit commit-2, got %v", got)
	}
	commitsLen := 0
	if len(withCommit2.Changes) > 0 {
		commitsLen = len(withCommit2.Changes[0].Commits)
	}
	if len(withCommit2.Changes) != 1 || commitsLen != 2 {
		t.Fatalf("expected 1 change with 2 commits, got %d changes / %d commits", len(withCommit2.Changes), commitsLen)
	}
}

func openTestManager(t *testing.T, repoPath, stateDir string) (*Manager, *sql.DB) {
	t.Helper()

	statePath := filepath.Join(stateDir, "state.db")
	legacyPath := filepath.Join(stateDir, "state.json")
	store, err := internaldb.Open(statePath, internaldb.OpenOptions{
		LegacyJSONPath: legacyPath,
		SkipConfirm:     true,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	manager, err := Open(repoPath, OpenOptions{DB: store.SqlDB()})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	return manager, store.SqlDB()
}

func ensureAgentSessions(db *sql.DB, repo string, sessions []JobAgentSession) error {
	now := time.Now()
	for _, session := range sessions {
		if session.ID == "" {
			continue
		}
		if _, err := db.Exec(`INSERT OR IGNORE INTO agent_sessions (
			repo, id, status, model, created_at, started_at, updated_at, completed_at,
			exit_code, duration_seconds, tokens_used, cost
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
			repo,
			session.ID,
			string(StatusActive),
			"",
			formatJobTime(now),
			"",
			formatJobTime(now),
			"",
			nil,
			0,
			0,
			0,
		); err != nil {
			return err
		}
	}
	return nil
}

func insertJob(db *sql.DB, item Job) error {
	if db == nil {
		return errors.New("insert job: db is nil")
	}

	projectOutcome, projectComments, projectAgentID, projectReviewedAt := reviewFields(item.ProjectReview)
	_, err := db.Exec(`INSERT INTO jobs (
		repo, id, todo_id, agent, implementation_model, code_review_model,
		project_review_model, stage, status, feedback,
		project_review_outcome, project_review_comments,
		project_review_agent_session_id, project_review_reviewed_at,
		created_at, started_at, updated_at, completed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		item.Repo,
		item.ID,
		item.TodoID,
		item.Agent,
		item.ImplementationModel,
		item.CodeReviewModel,
		item.ProjectReviewModel,
		string(item.Stage),
		string(item.Status),
		item.Feedback,
		projectOutcome,
		projectComments,
		projectAgentID,
		projectReviewedAt,
		formatJobTime(item.CreatedAt),
		formatOptionalJobTime(item.StartedAt),
		formatJobTime(item.UpdatedAt),
		formatOptionalJobTime(item.CompletedAt),
	)
	if err != nil {
		return err
	}


	if err := ensureAgentSessions(db, item.Repo, item.AgentSessions); err != nil {
		return err
	}
	for index, session := range item.AgentSessions {
		if _, err := db.Exec(`INSERT INTO job_agent_sessions (
			repo, job_id, session_id, purpose, position
		) VALUES (?, ?, ?, ?, ?);`,
			item.Repo,
			item.ID,
			session.ID,
			session.Purpose,
			index,
		); err != nil {
			return err
		}
	}

	for changeIndex, change := range item.Changes {
		result, err := db.Exec(`INSERT INTO job_changes (
			repo, job_id, change_id, created_at, position
		) VALUES (?, ?, ?, ?, ?);`,
			item.Repo,
			item.ID,
			change.ChangeID,
			formatJobTime(change.CreatedAt),
			changeIndex,
		)
		if err != nil {
			return err
		}
		changeRowID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		for commitIndex, commit := range change.Commits {
			reviewOutcome, reviewComments, reviewAgentID, reviewReviewedAt := reviewFields(commit.Review)
			if _, err := db.Exec(`INSERT INTO job_commits (
				job_change_id, commit_id, draft_message, tests_passed, agent_session_id,
				review_outcome, review_comments, review_agent_session_id, review_reviewed_at,
				created_at, position
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
				changeRowID,
				commit.CommitID,
				commit.DraftMessage,
				boolPointerToSQLite(commit.TestsPassed),
				commit.AgentSessionID,
				reviewOutcome,
				reviewComments,
				reviewAgentID,
				reviewReviewedAt,
				formatJobTime(commit.CreatedAt),
				commitIndex,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func TestManager_MarkStaleJobsFailed(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/stale"
	manager, sqlDB := openTestManager(t, repoPath, tmpDir)

	repoSlug, err := internaldb.GetOrCreateRepoName(sqlDB, repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}

	now := time.Date(2025, 5, 10, 12, 0, 0, 0, time.UTC)
	staleTime := now.Add(-15 * time.Minute) // 15 minutes ago (> 10 min threshold)
	recentTime := now.Add(-5 * time.Minute) // 5 minutes ago (< 10 min threshold)

	staleJob := Job{
		ID:        "stale-job",
		Repo:      repoSlug,
		TodoID:    "habit:cleanup",
		Stage:     StageImplementing,
		Status:    StatusActive,
		CreatedAt: staleTime.Add(-time.Hour),
		StartedAt: staleTime.Add(-time.Hour),
		UpdatedAt: staleTime,
	}
	recentJob := Job{
		ID:        "recent-job",
		Repo:      repoSlug,
		TodoID:    "todo-123",
		Stage:     StageImplementing,
		Status:    StatusActive,
		CreatedAt: recentTime.Add(-time.Hour),
		StartedAt: recentTime.Add(-time.Hour),
		UpdatedAt: recentTime,
	}
	completedJob := Job{
		ID:          "completed-job",
		Repo:        repoSlug,
		TodoID:      "todo-456",
		Stage:       StageCommitting,
		Status:      StatusCompleted,
		CreatedAt:   staleTime.Add(-2 * time.Hour),
		StartedAt:   staleTime.Add(-2 * time.Hour),
		UpdatedAt:   staleTime,
		CompletedAt: staleTime,
	}

	if err := insertJob(sqlDB, staleJob); err != nil {
		t.Fatalf("insert stale job: %v", err)
	}
	if err := insertJob(sqlDB, recentJob); err != nil {
		t.Fatalf("insert recent job: %v", err)
	}
	if err := insertJob(sqlDB, completedJob); err != nil {
		t.Fatalf("insert completed job: %v", err)
	}

	marked, err := manager.MarkStaleJobsFailed(now)
	if err != nil {
		t.Fatalf("mark stale jobs: %v", err)
	}
	if marked != 1 {
		t.Fatalf("expected 1 job marked, got %d", marked)
	}

	found, err := manager.Find(staleJob.ID)
	if err != nil {
		t.Fatalf("find stale job: %v", err)
	}
	if found.Status != StatusFailed {
		t.Fatalf("expected stale job status failed, got %q", found.Status)
	}
	if !found.CompletedAt.Equal(now) {
		t.Fatalf("expected stale job completed at %v, got %v", now, found.CompletedAt)
	}

	found, err = manager.Find(recentJob.ID)
	if err != nil {
		t.Fatalf("find recent job: %v", err)
	}
	if found.Status != StatusActive {
		t.Fatalf("expected recent job status active, got %q", found.Status)
	}

	found, err = manager.Find(completedJob.ID)
	if err != nil {
		t.Fatalf("find completed job: %v", err)
	}
	if found.Status != StatusCompleted {
		t.Fatalf("expected completed job status unchanged, got %q", found.Status)
	}
}

func TestManager_MarkStaleJobsFailed_OnlyAffectsCurrentRepo(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/stale-repo"
	otherRepoPath := "/Users/test/other-repo"
	manager, sqlDB := openTestManager(t, repoPath, tmpDir)

	repoSlug, err := internaldb.GetOrCreateRepoName(sqlDB, repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}
	otherSlug, err := internaldb.GetOrCreateRepoName(sqlDB, otherRepoPath)
	if err != nil {
		t.Fatalf("other repo slug: %v", err)
	}

	now := time.Date(2025, 5, 10, 12, 0, 0, 0, time.UTC)
	staleTime := now.Add(-15 * time.Minute)

	staleJobOurs := Job{
		ID:        "stale-ours",
		Repo:      repoSlug,
		TodoID:    "todo-ours",
		Stage:     StageImplementing,
		Status:    StatusActive,
		CreatedAt: staleTime.Add(-time.Hour),
		StartedAt: staleTime.Add(-time.Hour),
		UpdatedAt: staleTime,
	}
	staleJobOther := Job{
		ID:        "stale-other",
		Repo:      otherSlug,
		TodoID:    "todo-other",
		Stage:     StageImplementing,
		Status:    StatusActive,
		CreatedAt: staleTime.Add(-time.Hour),
		StartedAt: staleTime.Add(-time.Hour),
		UpdatedAt: staleTime,
	}

	if err := insertJob(sqlDB, staleJobOurs); err != nil {
		t.Fatalf("insert stale job ours: %v", err)
	}
	if err := insertJob(sqlDB, staleJobOther); err != nil {
		t.Fatalf("insert stale job other: %v", err)
	}

	marked, err := manager.MarkStaleJobsFailed(now)
	if err != nil {
		t.Fatalf("mark stale jobs: %v", err)
	}
	if marked != 1 {
		t.Fatalf("expected 1 job marked, got %d", marked)
	}

	var otherStatus string
	if err := sqlDB.QueryRow("SELECT status FROM jobs WHERE repo = ? AND id = ?;", otherSlug, staleJobOther.ID).Scan(&otherStatus); err != nil {
		t.Fatalf("load other repo job: %v", err)
	}
	if otherStatus != string(StatusActive) {
		t.Fatalf("expected other repo job unchanged, got status %q", otherStatus)
	}
}

func TestIsJobStale(t *testing.T) {
	now := time.Date(2025, 5, 10, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		job       Job
		wantStale bool
	}{
		{
			name: "active job updated recently",
			job: Job{
				Status:    StatusActive,
				UpdatedAt: now.Add(-5 * time.Minute),
			},
			wantStale: false,
		},
		{
			name: "active job updated at threshold",
			job: Job{
				Status:    StatusActive,
				UpdatedAt: now.Add(-10 * time.Minute),
			},
			wantStale: true,
		},
		{
			name: "active job updated long ago",
			job: Job{
				Status:    StatusActive,
				UpdatedAt: now.Add(-1 * time.Hour),
			},
			wantStale: true,
		},
		{
			name: "completed job updated long ago",
			job: Job{
				Status:    StatusCompleted,
				UpdatedAt: now.Add(-1 * time.Hour),
			},
			wantStale: false,
		},
		{
			name: "failed job updated long ago",
			job: Job{
				Status:    StatusFailed,
				UpdatedAt: now.Add(-1 * time.Hour),
			},
			wantStale: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsJobStale(tt.job, now)
			if got != tt.wantStale {
				t.Fatalf("IsJobStale() = %v, want %v", got, tt.wantStale)
			}
		})
	}
}

func TestManager_CountByHabit(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/count-repo"
	manager, sqlDB := openTestManager(t, repoPath, tmpDir)

	repoSlug, err := internaldb.GetOrCreateRepoName(sqlDB, repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}
	otherRepoPath := "/Users/test/other-repo"
	otherSlug, err := internaldb.GetOrCreateRepoName(sqlDB, otherRepoPath)
	if err != nil {
		t.Fatalf("other repo slug: %v", err)
	}

	now := time.Date(2025, 5, 10, 12, 0, 0, 0, time.UTC)

	// Create habit jobs
	habitJob1 := Job{
		ID:        "habit-job-1",
		Repo:      repoSlug,
		TodoID:    "habit:cleanup",
		Stage:     StageImplementing,
		Status:    StatusCompleted,
		CreatedAt: now,
		StartedAt: now,
		UpdatedAt: now,
	}
	habitJob2 := Job{
		ID:        "habit-job-2",
		Repo:      repoSlug,
		TodoID:    "habit:cleanup",
		Stage:     StageImplementing,
		Status:    StatusActive,
		CreatedAt: now,
		StartedAt: now,
		UpdatedAt: now,
	}
	habitJob3 := Job{
		ID:        "habit-job-3",
		Repo:      repoSlug,
		TodoID:    "habit:docs",
		Stage:     StageImplementing,
		Status:    StatusCompleted,
		CreatedAt: now,
		StartedAt: now,
		UpdatedAt: now,
	}
	// Non-habit job
	todoJob := Job{
		ID:        "todo-job-1",
		Repo:      repoSlug,
		TodoID:    "todo-123",
		Stage:     StageImplementing,
		Status:    StatusCompleted,
		CreatedAt: now,
		StartedAt: now,
		UpdatedAt: now,
	}
	// Job in different repo
	otherRepoJob := Job{
		ID:        "other-habit-job",
		Repo:      otherSlug,
		TodoID:    "habit:cleanup",
		Stage:     StageImplementing,
		Status:    StatusCompleted,
		CreatedAt: now,
		StartedAt: now,
		UpdatedAt: now,
	}

	if err := insertJob(sqlDB, habitJob1); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := insertJob(sqlDB, habitJob2); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := insertJob(sqlDB, habitJob3); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := insertJob(sqlDB, todoJob); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := insertJob(sqlDB, otherRepoJob); err != nil {
		t.Fatalf("insert job: %v", err)
	}

	counts, err := manager.CountByHabit()
	if err != nil {
		t.Fatalf("CountByHabit failed: %v", err)
	}

	if counts["cleanup"] != 2 {
		t.Errorf("cleanup count = %d, want 2", counts["cleanup"])
	}
	if counts["docs"] != 1 {
		t.Errorf("docs count = %d, want 1", counts["docs"])
	}
	if len(counts) != 2 {
		t.Errorf("got %d habits, want 2", len(counts))
	}
}

func TestManager_CountByHabit_EmptyRepo(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/empty-repo"
	manager, _ := openTestManager(t, repoPath, tmpDir)

	counts, err := manager.CountByHabit()
	if err != nil {
		t.Fatalf("CountByHabit failed: %v", err)
	}

	if len(counts) != 0 {
		t.Errorf("got %d counts, want 0", len(counts))
	}
}
