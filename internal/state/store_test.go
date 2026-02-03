package state

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestStore_LoadEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	st, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load empty state: %v", err)
	}

	if st == nil {
		t.Fatal("expected non-nil state")
	}

	if len(st.Repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(st.Repos))
	}

	if len(st.Workspaces) != 0 {
		t.Errorf("expected 0 workspaces, got %d", len(st.Workspaces))
	}

	if len(st.AgentSessions) != 0 {
		t.Errorf("expected 0 agent sessions, got %d", len(st.AgentSessions))
	}

	if len(st.Jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(st.Jobs))
	}
}

func TestStore_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	st := newState()
	st.Repos = map[string]RepoInfo{
		"my-project": {SourcePath: "/Users/test/my-project"},
	}
	st.Workspaces = map[string]WorkspaceInfo{
		"my-project/ws-001": {
			Name:        "ws-001",
			Repo:        "my-project",
			Path:        "/Users/test/.local/share/incrementum/workspaces/my-project/ws-001",
			Purpose:     "initial sync",
			Rev:         "@",
			Status:      WorkspaceStatusAcquired,
			Provisioned: true,
		},
	}
	st.Jobs = map[string]Job{
		"job-123": {
			ID:     "job-123",
			Repo:   "my-project",
			TodoID: "todo-1",
			Stage:  JobStageImplementing,
			Status: JobStatusActive,
		},
	}

	if err := store.Save(st); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if len(loaded.Repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(loaded.Repos))
	}

	if loaded.Repos["my-project"].SourcePath != "/Users/test/my-project" {
		t.Error("repo source path mismatch")
	}

	if len(loaded.Workspaces) != 1 {
		t.Errorf("expected 1 workspace, got %d", len(loaded.Workspaces))
	}

	ws := loaded.Workspaces["my-project/ws-001"]
	if ws.Name != "ws-001" {
		t.Errorf("expected name ws-001, got %s", ws.Name)
	}
	if ws.Purpose != "initial sync" {
		t.Errorf("expected purpose to persist, got %q", ws.Purpose)
	}
	if ws.Rev != "@" {
		t.Errorf("expected rev to persist, got %q", ws.Rev)
	}
	if ws.Status != WorkspaceStatusAcquired {
		t.Errorf("expected status acquired, got %s", ws.Status)
	}

	if len(loaded.Jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(loaded.Jobs))
	}

	job := loaded.Jobs["job-123"]
	if job.ID != "job-123" {
		t.Errorf("expected job id job-123, got %s", job.ID)
	}
	if job.Stage != JobStageImplementing {
		t.Errorf("expected job stage implementing, got %s", job.Stage)
	}
	if job.Status != JobStatusActive {
		t.Errorf("expected job status active, got %s", job.Status)
	}
}

func TestStore_SaveNoChange(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	st := newState()

	if err := store.Save(st); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	statePath := store.statePath()
	oldTime := time.Unix(1, 0)
	if err := os.Chtimes(statePath, oldTime, oldTime); err != nil {
		t.Fatalf("failed to set mod time: %v", err)
	}

	if err := store.Save(st); err != nil {
		t.Fatalf("failed to save identical state: %v", err)
	}

	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("failed to stat state file: %v", err)
	}

	if !info.ModTime().Equal(oldTime) {
		t.Errorf("expected mod time to stay %v, got %v", oldTime, info.ModTime())
	}
}

func TestStore_Update(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	err := store.Update(func(st *State) error {
		st.Repos["my-project"] = RepoInfo{SourcePath: "/test/path"}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to update state: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if loaded.Repos["my-project"].SourcePath != "/test/path" {
		t.Error("update did not persist")
	}
}

func TestStore_ConcurrentUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	err := store.Update(func(st *State) error {
		st.Repos["counter"] = RepoInfo{SourcePath: "0"}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to init state: %v", err)
	}

	var wg sync.WaitGroup
	numGoroutines := 10
	incrementsPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				err := store.Update(func(st *State) error {
					_ = st.Repos["counter"]
					st.Repos["counter"] = RepoInfo{SourcePath: "updated"}
					return nil
				})
				if err != nil {
					t.Errorf("concurrent update failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load final state: %v", err)
	}

	if loaded.Repos["counter"].SourcePath != "updated" {
		t.Error("final state is corrupted")
	}
}

func TestSanitizeRepoName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/Users/test/my-project", "users-test-my-project"},
		{"/Users/test/My Project", "users-test-my-project"},
		{"/home/user/some/deep/path", "home-user-some-deep-path"},
	}

	for _, tt := range tests {
		result := SanitizeRepoName(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeRepoName(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestStore_GetOrCreateRepoName(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	name1, err := store.GetOrCreateRepoName("/Users/test/my-project")
	if err != nil {
		t.Fatalf("failed to get repo name: %v", err)
	}

	name2, err := store.GetOrCreateRepoName("/Users/test/my-project")
	if err != nil {
		t.Fatalf("failed to get repo name: %v", err)
	}

	if name1 != name2 {
		t.Errorf("expected same name, got %q and %q", name1, name2)
	}

	name3, err := store.GetOrCreateRepoName("/Users/test/my/project")
	if err != nil {
		t.Fatalf("failed to get repo name: %v", err)
	}

	if name3 == name1 {
		t.Error("collision not handled - different paths got same name")
	}
}

func TestStore_RepoPathForWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	repoName, err := store.GetOrCreateRepoName("/Users/test/my-project")
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	wsPath := filepath.Join("/tmp/workspaces", repoName, "ws-001")
	if err := store.Update(func(st *State) error {
		st.Workspaces[repoName+"/ws-001"] = WorkspaceInfo{
			Name: "ws-001",
			Repo: repoName,
			Path: wsPath,
		}
		return nil
	}); err != nil {
		t.Fatalf("failed to add workspace: %v", err)
	}

	resolved, found, err := store.RepoPathForWorkspace(wsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected workspace to be found")
	}
	if resolved != "/Users/test/my-project" {
		t.Fatalf("expected repo path, got %q", resolved)
	}

	_, found, err = store.RepoPathForWorkspace(filepath.Join("/tmp/workspaces", repoName, "ws-999"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected workspace to be missing")
	}
}

func TestStore_RepoPathForWorkspace_MissingRepo(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	wsPath := filepath.Join("/tmp/workspaces", "missing", "ws-001")
	if err := store.Update(func(st *State) error {
		st.Workspaces["missing/ws-001"] = WorkspaceInfo{
			Name: "ws-001",
			Repo: "missing",
			Path: wsPath,
		}
		return nil
	}); err != nil {
		t.Fatalf("failed to add workspace: %v", err)
	}

	_, found, err := store.RepoPathForWorkspace(wsPath)
	if !found {
		t.Fatal("expected workspace to be found")
	}
	if err == nil {
		t.Fatal("expected error for missing repo path")
	}
}
