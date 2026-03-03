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

// DeployJob stores deploy-specific job data.
type DeployJob struct {
	JobID           int64   `gorm:"primaryKey;column:job_id"`
	App             string  `gorm:"column:app"`
	ImageRef        string  `gorm:"column:image_ref"`
	PreviousImage   *string `gorm:"column:previous_image"`
	BinaryBytes     *int64  `gorm:"column:binary_bytes"`
	ImageBytes      *int64  `gorm:"column:image_bytes"`
	CompileMs       *int64  `gorm:"column:compile_ms"`
	PushMs          *int64  `gorm:"column:push_ms"`
	DeployMs        *int64  `gorm:"column:deploy_ms"`
	PackagesChanged *string `gorm:"column:packages_changed"`
}

func (DeployJob) TableName() string { return "deploy_jobs" }

// TerraformJob stores terraform-specific job data.
type TerraformJob struct {
	JobID              int64 `gorm:"primaryKey;column:job_id"`
	ResourcesAdded     int   `gorm:"column:resources_added"`
	ResourcesChanged   int   `gorm:"column:resources_changed"`
	ResourcesDestroyed int   `gorm:"column:resources_destroyed"`
}

func (TerraformJob) TableName() string { return "terraform_jobs" }

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

// FinishRun marks a run as complete.
func (m *Model) FinishRun(runID int64, status, errMsg string) error {
	t := now()
	updates := map[string]any{
		"status":      status,
		"finished_at": t,
	}
	if errMsg != "" {
		updates["error"] = errMsg
	}
	return m.db.Model(&Run{}).Where("id = ?", runID).Updates(updates).Error
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
		"duration_ms": durationMs,
	}
	if errorMsg != "" {
		updates["error"] = errorMsg
	}
	if outputPath != "" {
		updates["output_path"] = outputPath
	}
	return m.db.Model(&Job{}).Where("id = ?", jobID).Updates(updates).Error
}

// FinishDeployJob stores deploy-specific data.
func (m *Model) FinishDeployJob(dj *DeployJob) error {
	return m.db.Create(dj).Error
}

// FinishTerraformJob stores terraform-specific data.
func (m *Model) FinishTerraformJob(tj *TerraformJob) error {
	return m.db.Create(tj).Error
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

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
