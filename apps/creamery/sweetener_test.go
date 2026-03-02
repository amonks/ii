package creamery

import (
	"math"
	"testing"
)

func TestRelativeSoftnessClassification(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		pac  float64
		want string
	}{
		{"very hard", 19.9, "very hard"},
		{"hard", 20.0, "hard"},
		{"firm", 25.0, "firm (good for scooping)"},
		{"soft", 32.0, "soft (good for serving)"},
		{"very soft", 38.0, "very soft"},
		{"too soft", 45.0, "too soft (may not hold shape)"},
		{"nan guard", math.NaN(), "softness unavailable"},
		{"inf guard", math.Inf(1), "softness unavailable"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			analysis := SweetenerAnalysis{TotalPAC: tc.pac}
			got := analysis.RelativeSoftness()
			if got != tc.want {
				t.Fatalf("RelativeSoftness() = %q, want %q", got, tc.want)
			}
		})
	}
}
