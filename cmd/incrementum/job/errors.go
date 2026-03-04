package job

import (
	"errors"

	"github.com/amonks/incrementum/internal/validation"
)

var (
	// ErrInvalidStatus indicates a job status is invalid.
	ErrInvalidStatus = errors.New("invalid status")
	// ErrInvalidStage indicates a job stage is invalid.
	ErrInvalidStage = errors.New("invalid stage")
	// ErrInvalidFeedbackFormat indicates a feedback file could not be parsed.
	ErrInvalidFeedbackFormat = errors.New("invalid feedback format")
	// ErrJobInterrupted indicates the job was interrupted.
	ErrJobInterrupted = errors.New("job interrupted")
	// ErrJobAbandoned indicates the job was abandoned.
	ErrJobAbandoned = errors.New("job abandoned")
	// ErrJobNotFound indicates the requested job is missing.
	ErrJobNotFound = errors.New("job not found")
	// ErrAmbiguousJobIDPrefix indicates a prefix matches multiple jobs.
	ErrAmbiguousJobIDPrefix = errors.New("ambiguous job id prefix")
	// ErrNoCurrentChange indicates a job has no current change.
	ErrNoCurrentChange = errors.New("no current change")
	// ErrNoCurrentCommit indicates a job has no current commit.
	ErrNoCurrentCommit = errors.New("no current commit")
)

// AbandonedError is returned when a job is abandoned with a reason.
type AbandonedError struct {
	Reason string
}

func (e *AbandonedError) Error() string {
	return "job abandoned"
}

func (e *AbandonedError) Unwrap() error {
	return ErrJobAbandoned
}

func formatInvalidStatusError(status Status) error {
	return validation.FormatInvalidValueError(ErrInvalidStatus, status, ValidStatuses())
}

func formatInvalidStageError(stage Stage) error {
	return validation.FormatInvalidValueError(ErrInvalidStage, stage, ValidStages())
}
