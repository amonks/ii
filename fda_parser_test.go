package creamery

import (
	"os"
	"testing"
)

func TestParseLabel_LabelAndName(t *testing.T) {
	content, err := os.ReadFile("testdata/label_v2.fda")
	if err != nil {
		t.Fatalf("read test file: %v", err)
	}

	label, err := ParseLabel(string(content))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if label.ID != "test" {
		t.Errorf("got ID %q, want %q", label.ID, "test")
	}
	if label.Name != "Test Product" {
		t.Errorf("got Name %q, want %q", label.Name, "Test Product")
	}
}
