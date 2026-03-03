package main

import (
	"os"
	"path/filepath"
	"testing"

	"monks.co/pkg/database"
)

func testModel(t *testing.T) *Model {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ci_test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	m, err := NewModelFromDB(db)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCreateAndFetchRun(t *testing.T) {
	m := testModel(t)

	run, err := m.CreateRun("abc123", "def456", "webhook")
	if err != nil {
		t.Fatal(err)
	}
	if run.ID == 0 {
		t.Error("expected non-zero run ID")
	}
	if run.HeadSHA != "abc123" {
		t.Errorf("expected head_sha abc123, got %s", run.HeadSHA)
	}
	if run.Status != "running" {
		t.Errorf("expected status running, got %s", run.Status)
	}

	runs, err := m.RecentRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].ID != run.ID {
		t.Errorf("expected run ID %d, got %d", run.ID, runs[0].ID)
	}
}

func TestFinishRun(t *testing.T) {
	m := testModel(t)

	run, err := m.CreateRun("abc123", "def456", "webhook")
	if err != nil {
		t.Fatal(err)
	}

	if err := m.FinishRun(run.ID, "success"); err != nil {
		t.Fatal(err)
	}

	_, jobs, err := m.RunWithJobs(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestLastSuccessfulSHA(t *testing.T) {
	m := testModel(t)

	// No successful runs yet.
	_, err := m.LastSuccessfulSHA()
	if err == nil {
		t.Error("expected error when no successful runs")
	}

	// Create a successful run.
	run, err := m.CreateRun("sha1", "base1", "webhook")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.FinishRun(run.ID, "success"); err != nil {
		t.Fatal(err)
	}

	sha, err := m.LastSuccessfulSHA()
	if err != nil {
		t.Fatal(err)
	}
	if sha != "sha1" {
		t.Errorf("expected sha1, got %s", sha)
	}

	// Create another successful run.
	run2, err := m.CreateRun("sha2", "sha1", "webhook")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.FinishRun(run2.ID, "success"); err != nil {
		t.Fatal(err)
	}

	sha, err = m.LastSuccessfulSHA()
	if err != nil {
		t.Fatal(err)
	}
	if sha != "sha2" {
		t.Errorf("expected sha2, got %s", sha)
	}
}

func TestHasRunningRun(t *testing.T) {
	m := testModel(t)

	running, err := m.HasRunningRun()
	if err != nil {
		t.Fatal(err)
	}
	if running {
		t.Error("expected no running runs")
	}

	run, err := m.CreateRun("sha1", "base1", "webhook")
	if err != nil {
		t.Fatal(err)
	}

	running, err = m.HasRunningRun()
	if err != nil {
		t.Fatal(err)
	}
	if !running {
		t.Error("expected running run")
	}

	if err := m.FinishRun(run.ID, "success"); err != nil {
		t.Fatal(err)
	}

	running, err = m.HasRunningRun()
	if err != nil {
		t.Fatal(err)
	}
	if running {
		t.Error("expected no running runs after finish")
	}
}

func TestJobs(t *testing.T) {
	m := testModel(t)

	run, err := m.CreateRun("sha1", "base1", "webhook")
	if err != nil {
		t.Fatal(err)
	}

	job, err := m.StartJob(run.ID, "test", "go-test")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "in_progress" {
		t.Errorf("expected status in_progress, got %s", job.Status)
	}

	if err := m.FinishJob(job.ID, "success", 1234, "", "/output/test.log"); err != nil {
		t.Fatal(err)
	}

	_, jobs, err := m.RunWithJobs(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Status != "success" {
		t.Errorf("expected job status success, got %s", jobs[0].Status)
	}
	if jobs[0].DurationMs == nil || *jobs[0].DurationMs != 1234 {
		t.Error("expected duration_ms 1234")
	}
}

func TestDeployJob(t *testing.T) {
	m := testModel(t)

	run, err := m.CreateRun("sha1", "base1", "webhook")
	if err != nil {
		t.Fatal(err)
	}

	job, err := m.StartJob(run.ID, "deploy", "deploy-dogs")
	if err != nil {
		t.Fatal(err)
	}

	binaryBytes := int64(1024)
	imageBytes := int64(2048)
	compileMs := int64(500)
	dj := &DeployJob{
		JobID:       job.ID,
		App:         "dogs",
		ImageRef:    "registry.fly.io/monks-dogs:sha1",
		BinaryBytes: &binaryBytes,
		ImageBytes:  &imageBytes,
		CompileMs:   &compileMs,
	}
	if err := m.FinishDeployJob(dj); err != nil {
		t.Fatal(err)
	}
}

func TestDeployments(t *testing.T) {
	m := testModel(t)

	d := &Deployment{
		App:       "dogs",
		CommitSHA: "sha1",
		ImageRef:  "registry.fly.io/monks-dogs:sha1",
	}
	if err := m.RecordDeployment(d); err != nil {
		t.Fatal(err)
	}
	if d.DeployedAt == "" {
		t.Error("expected deployed_at to be set")
	}

	// Another deployment for same app.
	d2 := &Deployment{
		App:       "dogs",
		CommitSHA: "sha2",
		ImageRef:  "registry.fly.io/monks-dogs:sha2",
	}
	if err := m.RecordDeployment(d2); err != nil {
		t.Fatal(err)
	}

	// Different app.
	d3 := &Deployment{
		App:       "logs",
		CommitSHA: "sha2",
		ImageRef:  "registry.fly.io/monks-logs:sha2",
	}
	if err := m.RecordDeployment(d3); err != nil {
		t.Fatal(err)
	}

	// Current deployments should return latest for each app.
	current, err := m.CurrentDeployments()
	if err != nil {
		t.Fatal(err)
	}
	if len(current) != 2 {
		t.Fatalf("expected 2 current deployments, got %d", len(current))
	}
	if current[0].App != "dogs" || current[0].CommitSHA != "sha2" {
		t.Errorf("expected dogs/sha2, got %s/%s", current[0].App, current[0].CommitSHA)
	}
	if current[1].App != "logs" || current[1].CommitSHA != "sha2" {
		t.Errorf("expected logs/sha2, got %s/%s", current[1].App, current[1].CommitSHA)
	}

	// History for an app.
	history, err := m.DeploymentHistory("dogs")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
	// Most recent first.
	if history[0].CommitSHA != "sha2" {
		t.Errorf("expected most recent sha2, got %s", history[0].CommitSHA)
	}
}

func TestSetMachineID(t *testing.T) {
	m := testModel(t)

	run, err := m.CreateRun("sha1", "base1", "webhook")
	if err != nil {
		t.Fatal(err)
	}

	if err := m.SetMachineID(run.ID, "machine-abc"); err != nil {
		t.Fatal(err)
	}

	fetched, _, err := m.RunWithJobs(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fetched.MachineID == nil || *fetched.MachineID != "machine-abc" {
		t.Error("expected machine_id to be set")
	}
}

func init() {
	// Ensure MONKS_DATA is set for tests that use OpenFromDataFolder.
	if os.Getenv("MONKS_DATA") == "" {
		os.Setenv("MONKS_DATA", os.TempDir())
	}
}
