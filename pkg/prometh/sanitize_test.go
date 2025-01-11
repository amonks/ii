package prometh_test

import (
	"testing"

	"monks.co/pkg/prometh"
)

func TestSanitizeLabel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "already valid",
			input: "valid_label_123",
			want:  "valid_label_123",
		},
		{
			name:  "empty string",
			input: "",
			want:  "_",
		},
		{
			name:  "special characters",
			input: "hello!@#$%^&*()",
			want:  "hello__________",
		},
		{
			name:  "spaces and hyphens",
			input: "my-label with spaces",
			want:  "my_label_with_spaces",
		},
		{
			name:  "unicode characters",
			input: "métric_名前",
			want:  "m_tric____",
		},
		{
			name:  "single underscore",
			input: "_",
			want:  "_",
		},
		{
			name:  "all invalid",
			input: "!@#$",
			want:  "____",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prometh.SanitizeLabel(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeLabel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}


