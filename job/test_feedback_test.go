package job

import "testing"

func TestFormatTestFeedback(t *testing.T) {
	results := []TestCommandResult{
		{Command: "go test ./...", ExitCode: 1},
		{Command: "golangci-lint run", ExitCode: 2},
	}

	output := FormatTestFeedback(results)
	if output == "" {
		t.Fatalf("expected feedback output, got empty string")
	}

	expected := "- go test ./... is failing\n- golangci-lint run is failing"
	if output != expected {
		t.Fatalf("expected %q, got %q", expected, output)
	}
}

func TestFormatTestFeedbackIncludesPassingResults(t *testing.T) {
	results := []TestCommandResult{
		{Command: "go test ./...", ExitCode: 0},
		{Command: "golangci-lint run", ExitCode: 1},
	}

	output := FormatTestFeedback(results)
	if output == "" {
		t.Fatalf("expected feedback output, got empty string")
	}

	expected := "- go test ./... is passing\n- golangci-lint run is failing"
	if output != expected {
		t.Fatalf("expected %q, got %q", expected, output)
	}
}
