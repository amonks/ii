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

	if err := m.FinishRun(run.ID, "success", ""); err != nil {
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
	if err := m.FinishRun(run.ID, "success", ""); err != nil {
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
	if err := m.FinishRun(run2.ID, "success", ""); err != nil {
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

	if err := m.FinishRun(run.ID, "success", ""); err != nil {
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

	job, err := m.StartJob(run.ID, "test", "go-test", "/output/1/go-test")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "in_progress" {
		t.Errorf("expected status in_progress, got %s", job.Status)
	}

	if err := m.FinishJob(job.ID, "success", 1234, "", ""); err != nil {
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

func TestStreams(t *testing.T) {
	m := testModel(t)

	run, err := m.CreateRun("sha1", "base1", "webhook")
	if err != nil {
		t.Fatal(err)
	}

	job, err := m.StartJob(run.ID, "deploy", "deploy", "/output/1/deploy")
	if err != nil {
		t.Fatal(err)
	}

	// Start a stream.
	s, err := m.StartStream(job.ID, "dogs")
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != "in_progress" {
		t.Errorf("expected status in_progress, got %s", s.Status)
	}
	if s.StartedAt == nil {
		t.Error("expected started_at to be set")
	}

	// Finish the stream.
	if err := m.FinishStream(s.ID, "success", 1500, ""); err != nil {
		t.Fatal(err)
	}

	// Load streams for job.
	streams, err := m.StreamsForJob(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(streams))
	}
	if streams[0].Status != "success" {
		t.Errorf("expected status success, got %s", streams[0].Status)
	}
	if streams[0].DurationMs == nil || *streams[0].DurationMs != 1500 {
		t.Error("expected duration_ms 1500")
	}

	// Start another stream with error.
	s2, err := m.StartStream(job.ID, "logs")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.FinishStream(s2.ID, "failed", 500, "compile error"); err != nil {
		t.Fatal(err)
	}

	streams, err = m.StreamsForJob(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(streams))
	}
	// Sorted by name: dogs, logs.
	if streams[0].Name != "dogs" {
		t.Errorf("expected first stream dogs, got %s", streams[0].Name)
	}
	if streams[1].Name != "logs" {
		t.Errorf("expected second stream logs, got %s", streams[1].Name)
	}
	if streams[1].Error == nil || *streams[1].Error != "compile error" {
		t.Errorf("expected error 'compile error', got %v", streams[1].Error)
	}
}

func TestStreamsForRun(t *testing.T) {
	m := testModel(t)

	run, err := m.CreateRun("sha1", "base1", "webhook")
	if err != nil {
		t.Fatal(err)
	}

	job1, err := m.StartJob(run.ID, "task", "test", "")
	if err != nil {
		t.Fatal(err)
	}
	job2, err := m.StartJob(run.ID, "deploy", "deploy", "")
	if err != nil {
		t.Fatal(err)
	}

	m.StartStream(job1.ID, "go-test")
	m.StartStream(job1.ID, "staticcheck")
	m.StartStream(job2.ID, "dogs")
	m.StartStream(job2.ID, "proxy")

	streams, err := m.StreamsForRun(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 4 {
		t.Fatalf("expected 4 streams, got %d", len(streams))
	}
}

func TestStreamSkipped(t *testing.T) {
	m := testModel(t)

	run, err := m.CreateRun("sha1", "base1", "webhook")
	if err != nil {
		t.Fatal(err)
	}

	job, err := m.StartJob(run.ID, "deploy", "deploy", "")
	if err != nil {
		t.Fatal(err)
	}

	s, err := m.StartStream(job.ID, "homepage")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.FinishStream(s.ID, "skipped", 0, ""); err != nil {
		t.Fatal(err)
	}

	streams, err := m.StreamsForJob(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(streams))
	}
	if streams[0].Status != "skipped" {
		t.Errorf("expected status skipped, got %s", streams[0].Status)
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


func TestFinishRunClosesOrphanedStreams(t *testing.T) {
	m := testModel(t)

	run, err := m.CreateRun("sha1", "base1", "webhook")
	if err != nil {
		t.Fatal(err)
	}

	job, err := m.StartJob(run.ID, "deploy", "deploy", "")
	if err != nil {
		t.Fatal(err)
	}

	// Create two streams: one finished, one orphaned (still in_progress).
	s1, err := m.StartStream(job.ID, "dogs")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.FinishStream(s1.ID, "success", 1000, ""); err != nil {
		t.Fatal(err)
	}

	_, err = m.StartStream(job.ID, "publish")
	if err != nil {
		t.Fatal(err)
	}

	// Finish the run. The orphaned "publish" stream should be auto-closed.
	if err := m.FinishRun(run.ID, "failed", "publish failed"); err != nil {
		t.Fatal(err)
	}

	streams, err := m.StreamsForJob(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(streams))
	}
	for _, s := range streams {
		if s.Status == "in_progress" {
			t.Errorf("stream %q should not be in_progress after run finished, got status %q", s.Name, s.Status)
		}
	}
	// The orphaned stream should be marked as "unknown".
	for _, s := range streams {
		if s.Name == "publish" {
			if s.Status != "unknown" {
				t.Errorf("expected orphaned stream status 'unknown', got %q", s.Status)
			}
		}
	}
}

func TestPendingTrigger(t *testing.T) {
	m := testModel(t)

	// Initially empty.
	_, ok, err := m.PopPendingTrigger()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected no pending trigger")
	}

	// Set one.
	if err := m.SetPendingTrigger("sha1"); err != nil {
		t.Fatal(err)
	}

	// Overwrite (LWW).
	if err := m.SetPendingTrigger("sha2"); err != nil {
		t.Fatal(err)
	}

	// Pop should return the latest.
	sha, ok, err := m.PopPendingTrigger()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected pending trigger")
	}
	if sha != "sha2" {
		t.Errorf("expected sha2, got %s", sha)
	}

	// Should be empty after pop.
	_, ok, err = m.PopPendingTrigger()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected no pending trigger after pop")
	}
}

func TestRunningRun(t *testing.T) {
	m := testModel(t)

	// No running run.
	_, _, err := m.RunningRun()
	if err == nil {
		t.Error("expected error when no running run")
	}

	// Create a running run with no jobs.
	run, err := m.CreateRun("sha1", "base1", "webhook")
	if err != nil {
		t.Fatal(err)
	}

	r, phase, err := m.RunningRun()
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != run.ID {
		t.Errorf("expected run %d, got %d", run.ID, r.ID)
	}
	if phase != "" {
		t.Errorf("expected empty phase, got %q", phase)
	}

	// Start a test job.
	job, err := m.StartJob(run.ID, "test", "test", "")
	if err != nil {
		t.Fatal(err)
	}

	_, phase, err = m.RunningRun()
	if err != nil {
		t.Fatal(err)
	}
	if phase != "test" {
		t.Errorf("expected phase test, got %q", phase)
	}

	// Finish test, start deploy.
	m.FinishJob(job.ID, "success", 100, "", "")
	m.StartJob(run.ID, "deploy", "deploy", "")

	_, phase, err = m.RunningRun()
	if err != nil {
		t.Fatal(err)
	}
	if phase != "deploy" {
		t.Errorf("expected phase deploy, got %q", phase)
	}
}

func init() {
	// Ensure MONKS_DATA is set for tests that use OpenFromDataFolder.
	if os.Getenv("MONKS_DATA") == "" {
		os.Setenv("MONKS_DATA", os.TempDir())
	}
}
