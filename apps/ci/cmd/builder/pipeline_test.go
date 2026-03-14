package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"monks.co/pkg/config"
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

// stubPipeline creates a Pipeline with all external functions stubbed out,
// plus a no-op HTTP server for the reporter. It returns the pipeline and
// a cleanup function that restores the original function vars.
func stubPipeline(t *testing.T, phase string) *Pipeline {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)
	cfg := &Config{
		RunID:   1,
		HeadSHA: "abc123",
		BaseSHA: "000000",
		DataDir: t.TempDir(),
		Root:    t.TempDir(),
		Phase:   phase,
	}
	return &Pipeline{Config: cfg, Reporter: reporter}
}

// saveFuncs saves the current function vars and returns a cleanup function.
// It also installs no-op stubs for fetch and deploy by default.
func saveFuncs(t *testing.T) {
	t.Helper()
	origDetect := detectChangesFunc
	origTests := runTestsFunc
	origDeploy := deployAppFunc
	origRebuild := rebuildImageFunc
	origFetch := fetchCodeFunc
	origRunDeploy := runDeployFunc
	t.Cleanup(func() {
		detectChangesFunc = origDetect
		runTestsFunc = origTests
		deployAppFunc = origDeploy
		rebuildImageFunc = origRebuild
		fetchCodeFunc = origFetch
		runDeployFunc = origRunDeploy
	})
	// Default: no-op fetch and deploy for pipeline Run() tests.
	fetchCodeFunc = func(p *Pipeline) error { return nil }
	runDeployFunc = func(ctx context.Context, p *Pipeline, root string, analysis *ChangeAnalysis) error { return nil }
}

func TestRunInitialBuilderAffected_RebuildsBeforeTests(t *testing.T) {
	saveFuncs(t)
	p := stubPipeline(t, "initial")

	var steps []string
	detectChangesFunc = func(root, baseSHA string) (*ChangeAnalysis, error) {
		steps = append(steps, "detect")
		return &ChangeAnalysis{
			BuilderAffected: true,
			Cfg:             &config.AppsConfig{},
		}, nil
	}
	runTestsFunc = func(ctx context.Context, root string, reporter *Reporter, suffix string) error {
		steps = append(steps, "test")
		return nil
	}
	rebuildImageFunc = func(root, name, tomlPath string, reporter *Reporter, jobName string) error {
		steps = append(steps, "rebuild")
		return nil
	}

	status, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "restart-builder-image" {
		t.Errorf("expected restart-builder-image, got %s", status)
	}
	// Tests must NOT have run.
	expected := []string{"detect", "rebuild"}
	if len(steps) != len(expected) {
		t.Fatalf("expected steps %v, got %v", expected, steps)
	}
	for i, s := range expected {
		if steps[i] != s {
			t.Errorf("step %d: expected %s, got %s", i, s, steps[i])
		}
	}
}

func TestRunInitialCIAffected_TestsRunBeforeDeploy(t *testing.T) {
	saveFuncs(t)
	p := stubPipeline(t, "initial")

	var steps []string
	detectChangesFunc = func(root, baseSHA string) (*ChangeAnalysis, error) {
		steps = append(steps, "detect")
		return &ChangeAnalysis{
			CIAffected: true,
			Cfg:        &config.AppsConfig{},
		}, nil
	}
	runTestsFunc = func(ctx context.Context, root string, reporter *Reporter, suffix string) error {
		steps = append(steps, "test")
		return nil
	}
	deployAppFunc = func(root, app, sha, flyToken, baseImageRef string, cfg *config.AppsConfig, reporter *Reporter, jobName string) error {
		steps = append(steps, "deploy-"+app)
		return nil
	}

	status, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "restart-orchestrator" {
		t.Errorf("expected restart-orchestrator, got %s", status)
	}
	// detect runs twice (once early for builder check, once after tests for deploy decisions),
	// then orchestrator deploy via deployAppFunc.
	expected := []string{"detect", "test", "detect", "deploy-ci"}
	if len(steps) != len(expected) {
		t.Fatalf("expected steps %v, got %v", expected, steps)
	}
	for i, s := range expected {
		if steps[i] != s {
			t.Errorf("step %d: expected %s, got %s", i, s, steps[i])
		}
	}
}

func TestRunInitialNoInfraChanges_TestsThenDeploy(t *testing.T) {
	saveFuncs(t)
	p := stubPipeline(t, "initial")

	var steps []string
	detectChangesFunc = func(root, baseSHA string) (*ChangeAnalysis, error) {
		steps = append(steps, "detect")
		return &ChangeAnalysis{
			Affected: []string{"dogs"},
			Cfg:      &config.AppsConfig{},
		}, nil
	}
	runTestsFunc = func(ctx context.Context, root string, reporter *Reporter, suffix string) error {
		steps = append(steps, "test")
		return nil
	}
	runDeployFunc = func(ctx context.Context, p *Pipeline, root string, analysis *ChangeAnalysis) error {
		steps = append(steps, "deploy")
		return nil
	}

	status, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "success" {
		t.Errorf("expected success, got %s", status)
	}
	// detect runs twice in initial (early builder check + post-test deploy check), tests between.
	expected := []string{"detect", "test", "detect", "deploy"}
	if len(steps) != len(expected) {
		t.Fatalf("expected steps %v, got %v", expected, steps)
	}
	for i, s := range expected {
		if steps[i] != s {
			t.Errorf("step %d: expected %s, got %s", i, s, steps[i])
		}
	}
}

func TestRunPostOrchestrator_NoBuilderCheck(t *testing.T) {
	saveFuncs(t)
	p := stubPipeline(t, "post-orchestrator")

	var steps []string
	detectChangesFunc = func(root, baseSHA string) (*ChangeAnalysis, error) {
		steps = append(steps, "detect")
		return &ChangeAnalysis{
			BuilderAffected: true, // should be ignored in post-orchestrator
			Affected:        []string{"dogs"},
			Cfg:             &config.AppsConfig{},
		}, nil
	}
	runTestsFunc = func(ctx context.Context, root string, reporter *Reporter, suffix string) error {
		steps = append(steps, "test")
		return nil
	}
	runDeployFunc = func(ctx context.Context, p *Pipeline, root string, analysis *ChangeAnalysis) error {
		steps = append(steps, "deploy")
		return nil
	}
	rebuildImageFunc = func(root, name, tomlPath string, reporter *Reporter, jobName string) error {
		steps = append(steps, "rebuild")
		return nil
	}

	status, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "success" {
		t.Errorf("expected success, got %s", status)
	}
	// No rebuild should have happened.
	for _, s := range steps {
		if s == "rebuild" {
			t.Error("post-orchestrator should not rebuild builder image")
		}
	}
}

func TestRunPostBuilder_NoInfraChecks(t *testing.T) {
	saveFuncs(t)
	p := stubPipeline(t, "post-builder")

	var steps []string
	detectChangesFunc = func(root, baseSHA string) (*ChangeAnalysis, error) {
		steps = append(steps, "detect")
		return &ChangeAnalysis{
			Affected: []string{"dogs"},
			Cfg:      &config.AppsConfig{},
		}, nil
	}
	runTestsFunc = func(ctx context.Context, root string, reporter *Reporter, suffix string) error {
		steps = append(steps, "test")
		return nil
	}
	runDeployFunc = func(ctx context.Context, p *Pipeline, root string, analysis *ChangeAnalysis) error {
		steps = append(steps, "deploy")
		return nil
	}
	rebuildImageFunc = func(root, name, tomlPath string, reporter *Reporter, jobName string) error {
		steps = append(steps, "rebuild")
		return nil
	}

	status, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "success" {
		t.Errorf("expected success, got %s", status)
	}
	// Only test + detect + deploy, no rebuild.
	for _, s := range steps {
		if s == "rebuild" {
			t.Error("post-builder should not rebuild builder image")
		}
	}
}

func TestRunInitialTestFailure_NoDeployOrRebuild(t *testing.T) {
	saveFuncs(t)
	p := stubPipeline(t, "initial")

	detectChangesFunc = func(root, baseSHA string) (*ChangeAnalysis, error) {
		return &ChangeAnalysis{Cfg: &config.AppsConfig{}}, nil
	}
	runTestsFunc = func(ctx context.Context, root string, reporter *Reporter, suffix string) error {
		return fmt.Errorf("tests failed")
	}
	rebuildImageFunc = func(root, name, tomlPath string, reporter *Reporter, jobName string) error {
		t.Error("should not rebuild on test failure")
		return nil
	}

	status, err := p.Run(context.Background())
	if status != "failed" {
		t.Errorf("expected failed, got %s", status)
	}
	if err == nil {
		t.Error("expected error from test failure")
	}
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
