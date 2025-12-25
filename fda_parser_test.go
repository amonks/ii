package creamery

import (
	"os"
	"testing"
)

func TestParseLabel_LabelOnly(t *testing.T) {
	content, err := os.ReadFile("testdata/label_v1.fda")
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
}
