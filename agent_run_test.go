package main

import (
	"io"
	"strings"
	"testing"
)

func TestResolveAgentPromptFromArg(t *testing.T) {
	prompt, err := resolveAgentPrompt([]string{"test prompt"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt != "test prompt" {
		t.Fatalf("expected 'test prompt', got %q", prompt)
	}
}

func TestResolveAgentPromptFromStdin(t *testing.T) {
	reader := strings.NewReader("stdin prompt\n")
	prompt, err := resolveAgentPrompt(nil, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt != "stdin prompt" {
		t.Fatalf("expected 'stdin prompt', got %q", prompt)
	}
}

func TestResolveAgentPromptTrimsTrailingNewline(t *testing.T) {
	reader := strings.NewReader("prompt with newline\n")
	prompt, err := resolveAgentPrompt(nil, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt != "prompt with newline" {
		t.Fatalf("expected 'prompt with newline', got %q", prompt)
	}
}

func TestResolveAgentPromptTrimsTrailingCarriageReturn(t *testing.T) {
	reader := strings.NewReader("prompt with crlf\r\n")
	prompt, err := resolveAgentPrompt(nil, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt != "prompt with crlf" {
		t.Fatalf("expected 'prompt with crlf', got %q", prompt)
	}
}

func TestResolveAgentPromptArgTakesPrecedence(t *testing.T) {
	// When an argument is provided, it should be used even if stdin has content
	reader := strings.NewReader("stdin prompt")
	prompt, err := resolveAgentPrompt([]string{"arg prompt"}, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt != "arg prompt" {
		t.Fatalf("expected 'arg prompt', got %q", prompt)
	}
}

func TestResolveAgentPromptEmptyStdin(t *testing.T) {
	reader := strings.NewReader("")
	_, err := resolveAgentPrompt(nil, reader)
	if err == nil {
		t.Fatal("expected error for empty stdin, got nil")
	}
	if err != ErrEmptyPrompt {
		t.Fatalf("expected ErrEmptyPrompt, got: %v", err)
	}
}

func TestResolveAgentPromptMultilineStdin(t *testing.T) {
	reader := strings.NewReader("line 1\nline 2\nline 3\n")
	prompt, err := resolveAgentPrompt(nil, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "line 1\nline 2\nline 3"
	if prompt != expected {
		t.Fatalf("expected %q, got %q", expected, prompt)
	}
}

// errorReader is a reader that always returns an error
type errorReader struct{}

func (r errorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func TestResolveAgentPromptStdinError(t *testing.T) {
	reader := errorReader{}
	_, err := resolveAgentPrompt(nil, reader)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "read prompt from stdin") {
		t.Fatalf("expected stdin error, got: %v", err)
	}
}

func TestResolveAgentPromptEmptyArg(t *testing.T) {
	_, err := resolveAgentPrompt([]string{""}, nil)
	if err == nil {
		t.Fatal("expected error for empty arg, got nil")
	}
	if err != ErrEmptyPrompt {
		t.Fatalf("expected ErrEmptyPrompt, got: %v", err)
	}
}

func TestResolveAgentPromptWhitespaceOnlyArg(t *testing.T) {
	_, err := resolveAgentPrompt([]string{"   \t\n"}, nil)
	if err == nil {
		t.Fatal("expected error for whitespace-only arg, got nil")
	}
	if err != ErrEmptyPrompt {
		t.Fatalf("expected ErrEmptyPrompt, got: %v", err)
	}
}

func TestResolveAgentPromptWhitespaceOnlyStdin(t *testing.T) {
	reader := strings.NewReader("   \n\t  \n")
	_, err := resolveAgentPrompt(nil, reader)
	if err == nil {
		t.Fatal("expected error for whitespace-only stdin, got nil")
	}
	if err != ErrEmptyPrompt {
		t.Fatalf("expected ErrEmptyPrompt, got: %v", err)
	}
}
