package job

import (
	"errors"
	"fmt"
	"os"
	"strings"

	internalstrings "monks.co/ii/internal/strings"
)

// ReviewFeedback is parsed feedback from the review stage.
type ReviewFeedback struct {
	Outcome ReviewOutcome
	Details string
}

// ReadReviewFeedback loads feedback from a file.
// Missing files are treated as ACCEPT.
func ReadReviewFeedback(path string) (ReviewFeedback, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ReviewFeedback{Outcome: ReviewOutcomeAccept}, nil
		}
		return ReviewFeedback{}, fmt.Errorf("read feedback: %w", err)
	}
	removeErr := removeFileIfExists(path)
	if removeErr != nil {
		removeErr = fmt.Errorf("remove feedback: %w", removeErr)
	}

	feedback, parseErr := ParseReviewFeedback(string(data))
	if removeErr != nil {
		if parseErr != nil {
			return ReviewFeedback{}, errors.Join(parseErr, removeErr)
		}
		return feedback, removeErr
	}
	return feedback, parseErr
}

// ParseReviewFeedback parses the feedback file contents.
func ParseReviewFeedback(contents string) (ReviewFeedback, error) {
	lines := strings.Split(contents, "\n")

	for i, line := range lines {
		lines[i] = internalstrings.TrimTrailingCarriageReturn(line)
	}

	firstLine := internalstrings.TrimSpace(lines[0])
	if firstLine == "" {
		return ReviewFeedback{}, ErrInvalidFeedbackFormat
	}

	var outcome ReviewOutcome
	switch {
	case strings.EqualFold(firstLine, string(ReviewOutcomeAccept)):
		outcome = ReviewOutcomeAccept
	case strings.EqualFold(firstLine, string(ReviewOutcomeAbandon)):
		outcome = ReviewOutcomeAbandon
	case strings.EqualFold(firstLine, string(ReviewOutcomeRequestChanges)):
		outcome = ReviewOutcomeRequestChanges
	default:
		return ReviewFeedback{}, ErrInvalidFeedbackFormat
	}

	blankIndex := -1
	for i := 1; i < len(lines); i++ {
		if internalstrings.IsBlank(lines[i]) {
			blankIndex = i
			break
		}
	}

	// ACCEPT without details is allowed for backward compatibility
	if blankIndex == -1 {
		if outcome == ReviewOutcomeAccept {
			return ReviewFeedback{Outcome: ReviewOutcomeAccept}, nil
		}
		return ReviewFeedback{}, ErrInvalidFeedbackFormat
	}

	details := strings.Join(lines[blankIndex+1:], "\n")
	details = internalstrings.TrimTrailingNewlines(details)

	// ABANDON and REQUEST_CHANGES require details; ACCEPT details are optional
	if details == "" && outcome != ReviewOutcomeAccept {
		return ReviewFeedback{}, ErrInvalidFeedbackFormat
	}

	return ReviewFeedback{Outcome: outcome, Details: details}, nil
}
