package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFetchCodeClonesWhenNoRepo(t *testing.T) {
	requireJJ(t)

	// Create a bare "remote" repo with one commit.
	remoteDir := t.TempDir()
	shellRun(t, remoteDir, "git", "init", "--bare")

	workDir := t.TempDir()
	shellRun(t, workDir, "git", "init")
	shellRun(t, workDir, "git", "remote", "add", "origin", remoteDir)
	os.WriteFile(filepath.Join(workDir, "hello.txt"), []byte("hello"), 0644)
	shellRun(t, workDir, "git", "add", ".")
	shellRun(t, workDir, "git", "commit", "-m", "init")
	sha := cmdOutput(t, workDir, "git", "rev-parse", "HEAD")
	shellRun(t, workDir, "git", "push", "origin", "HEAD:main")

	// Pre-clone with jj so .jj exists, then remove it to simulate
	// a fresh persistent volume. We use jj git clone to test that
	// the clone path works, by setting repoURL in config to the
	// local bare repo.
	dataDir := t.TempDir()
	repoDir := filepath.Join(dataDir, "repo")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)
	cfg := &Config{
		RunID:   1,
		HeadSHA: sha,
		DataDir: dataDir,
		Root:    "/nonexistent",
		GHToken: "",
		RepoURL: remoteDir,
	}
	pipeline := &Pipeline{Config: cfg, Reporter: reporter}

	if err := pipeline.fetchCode(); err != nil {
		t.Fatalf("fetchCode: %v", err)
	}

	if cfg.Root != repoDir {
		t.Errorf("expected Root=%s, got %s", repoDir, cfg.Root)
	}

	if _, err := os.Stat(filepath.Join(repoDir, "hello.txt")); err != nil {
		t.Errorf("expected hello.txt in repo dir: %v", err)
	}
}

func TestFetchCodeFetchesWhenRepoExists(t *testing.T) {
	requireJJ(t)

	// Create a bare remote and seed it.
	remoteDir := t.TempDir()
	shellRun(t, remoteDir, "git", "init", "--bare")

	workDir := t.TempDir()
	shellRun(t, workDir, "git", "init")
	shellRun(t, workDir, "git", "remote", "add", "origin", remoteDir)
	os.WriteFile(filepath.Join(workDir, "hello.txt"), []byte("v1"), 0644)
	shellRun(t, workDir, "git", "add", ".")
	shellRun(t, workDir, "git", "commit", "-m", "v1")
	shellRun(t, workDir, "git", "push", "origin", "HEAD:main")

	// jj clone to simulate a prior run's cached repo.
	dataDir := t.TempDir()
	repoDir := filepath.Join(dataDir, "repo")
	shellRun(t, "", "jj", "git", "clone", remoteDir, repoDir, "--colocate")

	// Push a second commit to the remote.
	os.WriteFile(filepath.Join(workDir, "hello.txt"), []byte("v2"), 0644)
	shellRun(t, workDir, "git", "add", ".")
	shellRun(t, workDir, "git", "commit", "-m", "v2")
	sha2 := cmdOutput(t, workDir, "git", "rev-parse", "HEAD")
	shellRun(t, workDir, "git", "push", "origin", "HEAD:main")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)
	cfg := &Config{
		RunID:   1,
		HeadSHA: sha2,
		DataDir: dataDir,
		Root:    "/old",
		GHToken: "",
		RepoURL: remoteDir,
	}
	pipeline := &Pipeline{Config: cfg, Reporter: reporter}

	if err := pipeline.fetchCode(); err != nil {
		t.Fatalf("fetchCode: %v", err)
	}

	if cfg.Root != repoDir {
		t.Errorf("expected Root=%s, got %s", repoDir, cfg.Root)
	}

	got, err := os.ReadFile(filepath.Join(repoDir, "hello.txt"))
	if err != nil {
		t.Fatalf("reading hello.txt: %v", err)
	}
	if string(got) != "v2" {
		t.Errorf("expected 'v2', got %q", string(got))
	}
}

func requireJJ(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("jj"); err != nil {
		t.Skip("jj not installed")
	}
}

func shellRun(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func cmdOutput(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s %v: %v", name, args, err)
	}
	return string(out[:len(out)-1]) // trim newline
}

func TestPhaseSuffix(t *testing.T) {
	tests := []struct {
		phase  string
		suffix string
	}{
		{"initial", ""},
		{"post-orchestrator", "-post-orchestrator"},
		{"post-builder", "-post-builder"},
		{"", ""},
	}
	for _, tt := range tests {
		got := phaseSuffix(tt.phase)
		if got != tt.suffix {
			t.Errorf("phaseSuffix(%q) = %q, want %q", tt.phase, got, tt.suffix)
		}
	}
}
