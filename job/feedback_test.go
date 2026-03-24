package job

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseReviewFeedbackAccept(t *testing.T) {
	feedback, err := ParseReviewFeedback("ACCEPT")
	if err != nil {
		t.Fatalf("parse feedback: %v", err)
	}
	if feedback.Outcome != ReviewOutcomeAccept {
		t.Fatalf("expected ACCEPT, got %q", feedback.Outcome)
	}
	if feedback.Details != "" {
		t.Fatalf("expected no details, got %q", feedback.Details)
	}
}

func TestParseReviewFeedbackAcceptWithDetails(t *testing.T) {
	feedback, err := ParseReviewFeedback("ACCEPT\n\nLooks good, clean implementation.")
	if err != nil {
		t.Fatalf("parse feedback: %v", err)
	}
	if feedback.Outcome != ReviewOutcomeAccept {
		t.Fatalf("expected ACCEPT, got %q", feedback.Outcome)
	}
	if feedback.Details != "Looks good, clean implementation." {
		t.Fatalf("expected details %q, got %q", "Looks good, clean implementation.", feedback.Details)
	}
}

func TestParseReviewFeedbackRequestChanges(t *testing.T) {
	contents := "REQUEST_CHANGES\n\nPlease update the tests.\nAdd coverage.\n"
	feedback, err := ParseReviewFeedback(contents)
	if err != nil {
		t.Fatalf("parse feedback: %v", err)
	}
	if feedback.Outcome != ReviewOutcomeRequestChanges {
		t.Fatalf("expected REQUEST_CHANGES, got %q", feedback.Outcome)
	}
	expected := "Please update the tests.\nAdd coverage."
	if feedback.Details != expected {
		t.Fatalf("expected details %q, got %q", expected, feedback.Details)
	}
}

func TestParseReviewFeedbackAbandon(t *testing.T) {
	contents := "ABANDON\n\nThe approach is fundamentally flawed.\nNeed to reconsider.\n"
	feedback, err := ParseReviewFeedback(contents)
	if err != nil {
		t.Fatalf("parse feedback: %v", err)
	}
	if feedback.Outcome != ReviewOutcomeAbandon {
		t.Fatalf("expected ABANDON, got %q", feedback.Outcome)
	}
	expected := "The approach is fundamentally flawed.\nNeed to reconsider."
	if feedback.Details != expected {
		t.Fatalf("expected details %q, got %q", expected, feedback.Details)
	}
}

func TestParseReviewFeedbackAbandonMissingDetails(t *testing.T) {
	_, err := ParseReviewFeedback("ABANDON")
	if !errors.Is(err, ErrInvalidFeedbackFormat) {
		t.Fatalf("expected invalid feedback error, got %v", err)
	}
}

func TestParseReviewFeedbackInvalid(t *testing.T) {
	_, err := ParseReviewFeedback("REQUEST_CHANGES\nmissing blank")
	if !errors.Is(err, ErrInvalidFeedbackFormat) {
		t.Fatalf("expected invalid feedback error, got %v", err)
	}
}

func TestReadReviewFeedbackMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-feedback")
	feedback, err := ReadReviewFeedback(path)
	if err != nil {
		t.Fatalf("read feedback: %v", err)
	}
	if feedback.Outcome != ReviewOutcomeAccept {
		t.Fatalf("expected ACCEPT, got %q", feedback.Outcome)
	}
}

func TestReadReviewFeedbackDeletesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "feedback")
	if err := os.WriteFile(path, []byte("ACCEPT"), 0o644); err != nil {
		t.Fatalf("write feedback: %v", err)
	}

	_, err := ReadReviewFeedback(path)
	if err != nil {
		t.Fatalf("read feedback: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected feedback file to be deleted")
	}
}

func TestAbandonedError(t *testing.T) {
	err := &AbandonedError{Reason: "test reason"}
	if err.Error() != "job abandoned" {
		t.Fatalf("expected error message %q, got %q", "job abandoned", err.Error())
	}
	if !errors.Is(err, ErrJobAbandoned) {
		t.Fatalf("expected error to wrap ErrJobAbandoned")
	}
}

func TestAbandonedErrorAs(t *testing.T) {
	var err error = &AbandonedError{Reason: "the approach is flawed"}
	var abandonedErr *AbandonedError
	if !errors.As(err, &abandonedErr) {
		t.Fatalf("expected errors.As to succeed")
	}
	if abandonedErr.Reason != "the approach is flawed" {
		t.Fatalf("expected reason %q, got %q", "the approach is flawed", abandonedErr.Reason)
	}
}
