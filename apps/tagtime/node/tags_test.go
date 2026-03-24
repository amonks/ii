package node

import (
	"slices"
	"testing"
)

func TestExtractTags(t *testing.T) {
	tests := []struct {
		name  string
		blurb string
		want  []string
	}{
		{"empty", "", nil},
		{"no tags", "just working on stuff", nil},
		{"single tag", "working on #code", []string{"code"}},
		{"multiple tags", "working on #code for #project", []string{"code", "project"}},
		{"tag at start", "#meeting with team", []string{"meeting"}},
		{"tag at end", "lunch #eating", []string{"eating"}},
		{"only tag", "#sleeping", []string{"sleeping"}},
		{"duplicates removed", "#code #code #code", []string{"code"}},
		{"adjacent tags", "#code#work", []string{"code", "work"}},
		{"tag with numbers", "#project2", []string{"project2"}},
		{"hash in url ignored", "see http://example.com/#foo", []string{"foo"}},
		{"bare hash", "# not a tag", nil},
		{"underscore in tag", "#long_meeting", []string{"long_meeting"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTags(tt.blurb)
			if !slices.Equal(got, tt.want) {
				t.Errorf("ExtractTags(%q) = %v, want %v", tt.blurb, got, tt.want)
			}
		})
	}
}
