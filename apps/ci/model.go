package main

import (
	"embed"
	"fmt"
	"time"

	"monks.co/pkg/database"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Model provides database operations for the CI orchestrator.
type Model struct {
	db *database.DB
}

// NewModel opens the CI database and runs migrations.
func NewModel() (*Model, error) {
	db, err := database.OpenFromDataFolder("ci")
	if err != nil {
		return nil, fmt.Errorf("opening ci database: %w", err)
	}

	migrations, err := database.LoadMigrationsFromFS(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("loading migrations: %w", err)
	}
	if err := db.Migrate(migrations); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &Model{db: db}, nil
}

// NewModelFromDB creates a model from an existing database connection (for testing).
func NewModelFromDB(db *database.DB) (*Model, error) {
	migrations, err := database.LoadMigrationsFromFS(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("loading migrations: %w", err)
	}
	if err := db.Migrate(migrations); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return &Model{db: db}, nil
}

// Run represents a CI pipeline run.
type Run struct {
	ID         int64   `gorm:"primaryKey"`
	HeadSHA    string  `gorm:"column:head_sha"`
	BaseSHA    string  `gorm:"column:base_sha"`
	MachineID  *string `gorm:"column:machine_id"`
	StartedAt  string  `gorm:"column:started_at"`
	FinishedAt *string `gorm:"column:finished_at"`
	Status     string  `gorm:"column:status"`
	Trigger    string  `gorm:"column:trigger"`
	Error      *string `gorm:"column:error"`
}

func (Run) TableName() string { return "runs" }

// Job represents a single unit of work within a run.
type Job struct {
	ID         int64   `gorm:"primaryKey"`
	RunID      int64   `gorm:"column:run_id"`
	Kind       string  `gorm:"column:kind"`
	Name       string  `gorm:"column:name"`
	StartedAt  *string `gorm:"column:started_at"`
	FinishedAt *string `gorm:"column:finished_at"`
	DurationMs *int64  `gorm:"column:duration_ms"`
	Status     string  `gorm:"column:status"`
	Error      *string `gorm:"column:error"`
	OutputPath *string `gorm:"column:output_path"`
}

func (Job) TableName() string { return "jobs" }

// Stream represents a named output stream within a job.
type Stream struct {
	ID         int64   `gorm:"primaryKey"`
	JobID      int64   `gorm:"column:job_id"`
	Name       string  `gorm:"column:name"`
	Status     string  `gorm:"column:status"`
	StartedAt  *string `gorm:"column:started_at"`
	FinishedAt *string `gorm:"column:finished_at"`
	DurationMs *int64  `gorm:"column:duration_ms"`
	Error      *string `gorm:"column:error"`
}

func (Stream) TableName() string { return "streams" }

// Deployment records a successful deployment.
type Deployment struct {
	ID          int64  `gorm:"primaryKey"`
	JobID       *int64 `gorm:"column:job_id"`
	App         string `gorm:"column:app"`
	CommitSHA   string `gorm:"column:commit_sha"`
	ImageRef    string `gorm:"column:image_ref"`
	BinaryBytes *int64 `gorm:"column:binary_bytes"`
	DeployedAt  string `gorm:"column:deployed_at"`
}

func (Deployment) TableName() string { return "deployments" }

// CreateRun inserts a new run.
func (m *Model) CreateRun(headSHA, baseSHA, trigger string) (*Run, error) {
	run := Run{
		HeadSHA:   headSHA,
		BaseSHA:   baseSHA,
		StartedAt: now(),
		Status:    "running",
		Trigger:   trigger,
	}
	if err := m.db.Create(&run).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

// SetMachineID updates the machine ID for a run.
func (m *Model) SetMachineID(runID int64, machineID string) error {
	return m.db.Model(&Run{}).Where("id = ?", runID).Update("machine_id", machineID).Error
}

// FinishRun marks a run as complete. Any streams still in_progress are
// marked as "unknown" — this handles cases where a FinishStream call was
// lost (e.g. during an orchestrator restart).
func (m *Model) FinishRun(runID int64, status, errMsg string) error {
	t := now()
	updates := map[string]any{
		"status":      status,
		"finished_at": t,
	}
	if errMsg != "" {
		updates["error"] = errMsg
	}
	if err := m.db.Model(&Run{}).Where("id = ?", runID).Updates(updates).Error; err != nil {
		return err
	}

	// Close any orphaned in_progress streams.
	return m.db.Model(&Stream{}).
		Where("status = ? AND job_id IN (SELECT id FROM jobs WHERE run_id = ?)", "in_progress", runID).
		Updates(map[string]any{"status": "unknown", "finished_at": t}).Error
}

// RecentRuns returns the most recent runs.
func (m *Model) RecentRuns(limit int) ([]Run, error) {
	var runs []Run
	if err := m.db.Order("id DESC").Limit(limit).Find(&runs).Error; err != nil {
		return nil, err
	}
	return runs, nil
}

// RunWithJobs returns a run with its jobs.
func (m *Model) RunWithJobs(runID int64) (*Run, []Job, error) {
	var run Run
	if err := m.db.First(&run, runID).Error; err != nil {
		return nil, nil, err
	}
	var jobs []Job
	if err := m.db.Where("run_id = ?", runID).Order("id").Find(&jobs).Error; err != nil {
		return nil, nil, err
	}
	return &run, jobs, nil
}

// LastSuccessfulSHA returns the head SHA of the most recent successful run.
func (m *Model) LastSuccessfulSHA() (string, error) {
	var run Run
	if err := m.db.Where("status = ?", "success").Order("id DESC").First(&run).Error; err != nil {
		return "", err
	}
	return run.HeadSHA, nil
}

// HasRunningRun returns true if there's a run in "running" state.
func (m *Model) HasRunningRun() (bool, error) {
	var count int64
	if err := m.db.Model(&Run{}).Where("status = ?", "running").Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// StartJob creates a new job and marks it as in_progress.
func (m *Model) StartJob(runID int64, kind, name, outputPath string) (*Job, error) {
	t := now()
	job := Job{
		RunID:      runID,
		Kind:       kind,
		Name:       name,
		StartedAt:  &t,
		Status:     "in_progress",
		OutputPath: &outputPath,
	}
	if err := m.db.Create(&job).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

// FinishJob marks a job as complete.
func (m *Model) FinishJob(jobID int64, status string, durationMs int64, errorMsg, outputPath string) error {
	t := now()
	updates := map[string]any{
		"status":      status,
		"finished_at": t,
	}
	if durationMs > 0 {
		updates["duration_ms"] = durationMs
	}
	if errorMsg != "" {
		updates["error"] = errorMsg
	}
	if outputPath != "" {
		updates["output_path"] = outputPath
	}
	return m.db.Model(&Job{}).Where("id = ?", jobID).Updates(updates).Error
}

// StartStream creates a new stream and marks it as in_progress.
func (m *Model) StartStream(jobID int64, name string) (*Stream, error) {
	t := now()
	s := Stream{
		JobID:     jobID,
		Name:      name,
		Status:    "in_progress",
		StartedAt: &t,
	}
	if err := m.db.Create(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// FinishStream marks a stream as complete.
func (m *Model) FinishStream(streamID int64, status string, durationMs int64, errMsg string) error {
	t := now()
	updates := map[string]any{
		"status":      status,
		"finished_at": t,
	}
	if durationMs > 0 {
		updates["duration_ms"] = durationMs
	}
	if errMsg != "" {
		updates["error"] = errMsg
	}
	return m.db.Model(&Stream{}).Where("id = ?", streamID).Updates(updates).Error
}

// StreamsForJob returns all streams for a job, sorted by name.
func (m *Model) StreamsForJob(jobID int64) ([]Stream, error) {
	var streams []Stream
	err := m.db.Where("job_id = ?", jobID).Order("name").Find(&streams).Error
	return streams, err
}

// StreamsForRun returns all streams for all jobs in a run, sorted by job ID then stream name.
func (m *Model) StreamsForRun(runID int64) ([]Stream, error) {
	var streams []Stream
	err := m.db.Where("job_id IN (SELECT id FROM jobs WHERE run_id = ?)", runID).
		Order("job_id, name").Find(&streams).Error
	return streams, err
}

// RecordDeployment records a deployment.
func (m *Model) RecordDeployment(d *Deployment) error {
	if d.DeployedAt == "" {
		d.DeployedAt = now()
	}
	return m.db.Create(d).Error
}

// CurrentDeployments returns the latest deployment for each app.
func (m *Model) CurrentDeployments() ([]Deployment, error) {
	var deployments []Deployment
	err := m.db.Raw(`
		SELECT d.* FROM deployments d
		INNER JOIN (
			SELECT app, MAX(id) as max_id FROM deployments GROUP BY app
		) latest ON d.id = latest.max_id
		ORDER BY d.app
	`).Scan(&deployments).Error
	return deployments, err
}

// DeploymentHistory returns deployment history for an app.
func (m *Model) DeploymentHistory(app string) ([]Deployment, error) {
	var deployments []Deployment
	err := m.db.Where("app = ?", app).Order("id DESC").Limit(50).Find(&deployments).Error
	return deployments, err
}

// SetPendingTrigger sets (or replaces) the pending trigger SHA.
func (m *Model) SetPendingTrigger(sha string) error {
	return m.db.Exec(`
		INSERT INTO pending_trigger (id, sha, created_at) VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET sha = excluded.sha, created_at = excluded.created_at
	`, sha, now()).Error
}

// PopPendingTrigger returns and deletes the pending trigger SHA, if any.
func (m *Model) PopPendingTrigger() (string, bool, error) {
	var results []struct {
		SHA string `gorm:"column:sha"`
	}
	if err := m.db.Raw("SELECT sha FROM pending_trigger WHERE id = 1").Scan(&results).Error; err != nil {
		return "", false, err
	}
	if len(results) == 0 {
		return "", false, nil
	}
	if err := m.db.Exec("DELETE FROM pending_trigger WHERE id = 1").Error; err != nil {
		return "", false, err
	}
	return results[0].SHA, true, nil
}

// RunningRunPhase returns the current phase of the running run.
// It returns the run and the name of the most recently started job (or "" if none).
func (m *Model) RunningRun() (*Run, string, error) {
	var run Run
	if err := m.db.Where("status = ?", "running").Order("id DESC").First(&run).Error; err != nil {
		return nil, "", err
	}
	var job Job
	err := m.db.Where("run_id = ? AND status = ?", run.ID, "in_progress").Order("id DESC").First(&job).Error
	if err != nil {
		return &run, "", nil
	}
	return &run, job.Name, nil
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
