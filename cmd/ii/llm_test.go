package main

import (
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/llm"
)

func TestResolveLLMPrompt_Argument(t *testing.T) {
	prompt, err := resolveLLMPrompt([]string{"hello world"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt != "hello world" {
		t.Errorf("prompt = %q, want %q", prompt, "hello world")
	}
}

func TestResolveLLMPrompt_Stdin(t *testing.T) {
	reader := strings.NewReader("hello from stdin")
	prompt, err := resolveLLMPrompt(nil, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt != "hello from stdin" {
		t.Errorf("prompt = %q, want %q", prompt, "hello from stdin")
	}
}

func TestResolveLLMPrompt_EmptyArgument(t *testing.T) {
	_, err := resolveLLMPrompt([]string{"   "}, nil)
	if err != ErrEmptyLLMPrompt {
		t.Errorf("err = %v, want ErrEmptyLLMPrompt", err)
	}
}

func TestResolveLLMPrompt_EmptyStdin(t *testing.T) {
	reader := strings.NewReader("")
	_, err := resolveLLMPrompt(nil, reader)
	if err != ErrEmptyLLMPrompt {
		t.Errorf("err = %v, want ErrEmptyLLMPrompt", err)
	}
}

func TestFormatCompletionAge(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		completion llm.Completion
		want       string
	}{
		{
			name:       "zero time",
			completion: llm.Completion{},
			want:       "-",
		},
		{
			name: "recent",
			completion: llm.Completion{
				CreatedAt: now.Add(-5 * time.Minute),
			},
			want: "5m",
		},
		{
			name: "hours ago",
			completion: llm.Completion{
				CreatedAt: now.Add(-3 * time.Hour),
			},
			want: "3h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCompletionAge(tt.completion, now)
			if got != tt.want {
				t.Errorf("formatCompletionAge() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompletionPrefixLengths(t *testing.T) {
	completions := []llm.Completion{
		{ID: "abc123"},
		{ID: "abc456"},
		{ID: "def789"},
	}

	lengths := completionPrefixLengths(completions)

	// abc123 and abc456 share "abc" prefix, so need at least 4 chars
	if lengths["abc123"] < 4 {
		t.Errorf("abc123 prefix length = %d, want >= 4", lengths["abc123"])
	}
	if lengths["abc456"] < 4 {
		t.Errorf("abc456 prefix length = %d, want >= 4", lengths["abc456"])
	}
	// def789 is unique, so should need minimal prefix
	if lengths["def789"] > 3 {
		t.Errorf("def789 prefix length = %d, want <= 3", lengths["def789"])
	}
}
