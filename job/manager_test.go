package job

import (
	"errors"
	"testing"
	"time"

	statestore "github.com/amonks/incrementum/internal/state"
)

func TestManager_CreateAndFind(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/my-repo"
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2025, 4, 10, 8, 30, 0, 0, time.UTC)
	created, err := manager.Create("todo-123", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	expectedID := GenerateID("todo-123", startedAt)
	if created.ID != expectedID {
		t.Fatalf("expected job id %q, got %q", expectedID, created.ID)
	}

	stateStore := statestore.NewStore(tmpDir)
	repoSlug, err := stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}

	if created.Repo != repoSlug {
		t.Fatalf("expected repo %q, got %q", repoSlug, created.Repo)
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
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	store := statestore.NewStore(tmpDir)
	repoSlug, err := store.GetOrCreateRepoName(repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}

	startedAt := time.Date(2025, 5, 1, 12, 0, 0, 0, time.UTC)
	jobA := statestore.Job{
		ID:        "alpha-123",
		Repo:      repoSlug,
		TodoID:    "todo-1",
		Stage:     statestore.JobStageImplementing,
		Status:    statestore.JobStatusActive,
		CreatedAt: startedAt,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}
	jobB := statestore.Job{
		ID:        "alpha-456",
		Repo:      repoSlug,
		TodoID:    "todo-2",
		Stage:     statestore.JobStageImplementing,
		Status:    statestore.JobStatusActive,
		CreatedAt: startedAt.Add(2 * time.Minute),
		StartedAt: startedAt.Add(2 * time.Minute),
		UpdatedAt: startedAt.Add(2 * time.Minute),
	}

	if err := insertJob(store, repoSlug, jobA); err != nil {
		t.Fatalf("insert jobA: %v", err)
	}
	if err := insertJob(store, repoSlug, jobB); err != nil {
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
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	store := statestore.NewStore(tmpDir)
	repoSlug, err := store.GetOrCreateRepoName(repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}
	otherRepo, err := store.GetOrCreateRepoName("/Users/test/other")
	if err != nil {
		t.Fatalf("other repo slug: %v", err)
	}

	startedAt := time.Date(2025, 5, 10, 9, 0, 0, 0, time.UTC)
	activeJob := statestore.Job{
		ID:        "job-active",
		Repo:      repoSlug,
		TodoID:    "todo-active",
		Stage:     statestore.JobStageTesting,
		Status:    statestore.JobStatusActive,
		CreatedAt: startedAt,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}
	completedJob := statestore.Job{
		ID:          "job-completed",
		Repo:        repoSlug,
		TodoID:      "todo-completed",
		Stage:       statestore.JobStageCommitting,
		Status:      statestore.JobStatusCompleted,
		CreatedAt:   startedAt.Add(2 * time.Hour),
		StartedAt:   startedAt.Add(2 * time.Hour),
		UpdatedAt:   startedAt.Add(2 * time.Hour),
		CompletedAt: startedAt.Add(3 * time.Hour),
	}
	otherJob := statestore.Job{
		ID:        "job-other",
		Repo:      otherRepo,
		TodoID:    "todo-other",
		Stage:     statestore.JobStageImplementing,
		Status:    statestore.JobStatusActive,
		CreatedAt: startedAt.Add(30 * time.Minute),
		StartedAt: startedAt.Add(30 * time.Minute),
		UpdatedAt: startedAt.Add(30 * time.Minute),
	}

	if err := insertJob(store, repoSlug, activeJob); err != nil {
		t.Fatalf("insert active job: %v", err)
	}
	if err := insertJob(store, repoSlug, completedJob); err != nil {
		t.Fatalf("insert completed job: %v", err)
	}
	if err := insertJob(store, otherRepo, otherJob); err != nil {
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
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
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
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

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
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

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
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

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
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

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
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

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

func insertJob(store *statestore.Store, repoSlug string, item statestore.Job) error {
	return store.Update(func(st *statestore.State) error {
		st.Jobs[repoSlug+"/"+item.ID] = item
		return nil
	})
}

func TestManager_MarkStaleJobsFailed(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/stale"
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	store := statestore.NewStore(tmpDir)
	repoSlug, err := store.GetOrCreateRepoName(repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}

	now := time.Date(2025, 5, 10, 12, 0, 0, 0, time.UTC)
	staleTime := now.Add(-15 * time.Minute) // 15 minutes ago (> 10 min threshold)
	recentTime := now.Add(-5 * time.Minute) // 5 minutes ago (< 10 min threshold)

	staleJob := statestore.Job{
		ID:        "stale-job",
		Repo:      repoSlug,
		TodoID:    "habit:cleanup",
		Stage:     statestore.JobStageImplementing,
		Status:    statestore.JobStatusActive,
		CreatedAt: staleTime.Add(-time.Hour),
		StartedAt: staleTime.Add(-time.Hour),
		UpdatedAt: staleTime,
	}
	recentJob := statestore.Job{
		ID:        "recent-job",
		Repo:      repoSlug,
		TodoID:    "todo-123",
		Stage:     statestore.JobStageImplementing,
		Status:    statestore.JobStatusActive,
		CreatedAt: recentTime.Add(-time.Hour),
		StartedAt: recentTime.Add(-time.Hour),
		UpdatedAt: recentTime,
	}
	completedJob := statestore.Job{
		ID:          "completed-job",
		Repo:        repoSlug,
		TodoID:      "todo-456",
		Stage:       statestore.JobStageCommitting,
		Status:      statestore.JobStatusCompleted,
		CreatedAt:   staleTime.Add(-2 * time.Hour),
		StartedAt:   staleTime.Add(-2 * time.Hour),
		UpdatedAt:   staleTime,
		CompletedAt: staleTime,
	}

	if err := insertJob(store, repoSlug, staleJob); err != nil {
		t.Fatalf("insert stale job: %v", err)
	}
	if err := insertJob(store, repoSlug, recentJob); err != nil {
		t.Fatalf("insert recent job: %v", err)
	}
	if err := insertJob(store, repoSlug, completedJob); err != nil {
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
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	store := statestore.NewStore(tmpDir)
	repoSlug, err := store.GetOrCreateRepoName(repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}
	otherSlug, err := store.GetOrCreateRepoName(otherRepoPath)
	if err != nil {
		t.Fatalf("other repo slug: %v", err)
	}

	now := time.Date(2025, 5, 10, 12, 0, 0, 0, time.UTC)
	staleTime := now.Add(-15 * time.Minute)

	staleJobOurs := statestore.Job{
		ID:        "stale-ours",
		Repo:      repoSlug,
		TodoID:    "todo-ours",
		Stage:     statestore.JobStageImplementing,
		Status:    statestore.JobStatusActive,
		CreatedAt: staleTime.Add(-time.Hour),
		StartedAt: staleTime.Add(-time.Hour),
		UpdatedAt: staleTime,
	}
	staleJobOther := statestore.Job{
		ID:        "stale-other",
		Repo:      otherSlug,
		TodoID:    "todo-other",
		Stage:     statestore.JobStageImplementing,
		Status:    statestore.JobStatusActive,
		CreatedAt: staleTime.Add(-time.Hour),
		StartedAt: staleTime.Add(-time.Hour),
		UpdatedAt: staleTime,
	}

	if err := insertJob(store, repoSlug, staleJobOurs); err != nil {
		t.Fatalf("insert stale job ours: %v", err)
	}
	if err := insertJob(store, otherSlug, staleJobOther); err != nil {
		t.Fatalf("insert stale job other: %v", err)
	}

	marked, err := manager.MarkStaleJobsFailed(now)
	if err != nil {
		t.Fatalf("mark stale jobs: %v", err)
	}
	if marked != 1 {
		t.Fatalf("expected 1 job marked, got %d", marked)
	}

	st, err := store.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	otherJob := st.Jobs[otherSlug+"/"+staleJobOther.ID]
	if otherJob.Status != statestore.JobStatusActive {
		t.Fatalf("expected other repo job unchanged, got status %q", otherJob.Status)
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
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	store := statestore.NewStore(tmpDir)
	repoSlug, err := store.GetOrCreateRepoName(repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}
	otherRepoPath := "/Users/test/other-repo"
	otherSlug, err := store.GetOrCreateRepoName(otherRepoPath)
	if err != nil {
		t.Fatalf("other repo slug: %v", err)
	}

	now := time.Date(2025, 5, 10, 12, 0, 0, 0, time.UTC)

	// Create habit jobs
	habitJob1 := statestore.Job{
		ID:        "habit-job-1",
		Repo:      repoSlug,
		TodoID:    "habit:cleanup",
		Stage:     statestore.JobStageImplementing,
		Status:    statestore.JobStatusCompleted,
		CreatedAt: now,
		StartedAt: now,
		UpdatedAt: now,
	}
	habitJob2 := statestore.Job{
		ID:        "habit-job-2",
		Repo:      repoSlug,
		TodoID:    "habit:cleanup",
		Stage:     statestore.JobStageImplementing,
		Status:    statestore.JobStatusActive,
		CreatedAt: now,
		StartedAt: now,
		UpdatedAt: now,
	}
	habitJob3 := statestore.Job{
		ID:        "habit-job-3",
		Repo:      repoSlug,
		TodoID:    "habit:docs",
		Stage:     statestore.JobStageImplementing,
		Status:    statestore.JobStatusCompleted,
		CreatedAt: now,
		StartedAt: now,
		UpdatedAt: now,
	}
	// Non-habit job
	todoJob := statestore.Job{
		ID:        "todo-job-1",
		Repo:      repoSlug,
		TodoID:    "todo-123",
		Stage:     statestore.JobStageImplementing,
		Status:    statestore.JobStatusCompleted,
		CreatedAt: now,
		StartedAt: now,
		UpdatedAt: now,
	}
	// Job in different repo
	otherRepoJob := statestore.Job{
		ID:        "other-habit-job",
		Repo:      otherSlug,
		TodoID:    "habit:cleanup",
		Stage:     statestore.JobStageImplementing,
		Status:    statestore.JobStatusCompleted,
		CreatedAt: now,
		StartedAt: now,
		UpdatedAt: now,
	}

	if err := insertJob(store, repoSlug, habitJob1); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := insertJob(store, repoSlug, habitJob2); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := insertJob(store, repoSlug, habitJob3); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := insertJob(store, repoSlug, todoJob); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := insertJob(store, otherSlug, otherRepoJob); err != nil {
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
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	counts, err := manager.CountByHabit()
	if err != nil {
		t.Fatalf("CountByHabit failed: %v", err)
	}

	if len(counts) != 0 {
		t.Errorf("got %d counts, want 0", len(counts))
	}
}
