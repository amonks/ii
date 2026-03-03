package main

import (
	"testing"
)

func TestParseTerraformOutput(t *testing.T) {
	tests := []struct {
		output                          string
		wantAdded, wantChanged, wantDel int
	}{
		{
			output:      "Apply complete! Resources: 3 added, 1 changed, 0 destroyed.",
			wantAdded:   3,
			wantChanged: 1,
			wantDel:     0,
		},
		{
			output:      "Apply complete! Resources: 0 added, 0 changed, 2 destroyed.",
			wantAdded:   0,
			wantChanged: 0,
			wantDel:     2,
		},
		{
			output:    "No changes. Infrastructure is up-to-date.",
			wantAdded: 0,
		},
	}

	for _, tt := range tests {
		added, changed, destroyed := parseTerraformOutput(tt.output)
		if added != tt.wantAdded || changed != tt.wantChanged || destroyed != tt.wantDel {
			t.Errorf("parseTerraformOutput(%q) = (%d, %d, %d), want (%d, %d, %d)",
				tt.output, added, changed, destroyed, tt.wantAdded, tt.wantChanged, tt.wantDel)
		}
	}
}
