package jj

import "testing"

func TestIsFileNotFoundOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected bool
	}{
		{
			name:     "no such file",
			output:   "Error: No such file or directory",
			expected: true,
		},
		{
			name:     "no such path",
			output:   "Error: No such path: dependencies.jsonl",
			expected: true,
		},
		{
			name:     "path does not exist",
			output:   "Error: Path does not exist",
			expected: true,
		},
		{
			name:     "other error",
			output:   "Error: permission denied",
			expected: false,
		},
	}

	for _, test := range tests {
		if got := isFileNotFoundOutput([]byte(test.output)); got != test.expected {
			t.Fatalf("%s: expected %v, got %v", test.name, test.expected, got)
		}
	}
}
