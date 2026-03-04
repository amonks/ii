package ui

import "testing"

func TestPrefixLength(t *testing.T) {
	tests := []struct {
		name   string
		length map[string]int
		id     string
		want   int
	}{
		{
			name:   "case insensitive lookup",
			length: map[string]int{"abc123": 4},
			id:     "ABC123",
			want:   4,
		},
		{
			name:   "missing id",
			length: map[string]int{"abc123": 4},
			id:     "",
			want:   0,
		},
		{
			name:   "nil map",
			length: nil,
			id:     "ABC123",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrefixLength(tt.length, tt.id); got != tt.want {
				t.Fatalf("PrefixLength() = %d, want %d", got, tt.want)
			}
		})
	}
}
