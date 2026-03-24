package node

import (
	"testing"

	"monks.co/pkg/color"
)

func TestBuildTagTree(t *testing.T) {
	t.Run("flat tags produce single-level tree", func(t *testing.T) {
		summaries := []TagSummary{
			{Name: "sleep", TotalSecs: 100, Color: color.Hash("sleep"), Sparkline: []float64{50, 50}},
			{Name: "work", TotalSecs: 200, Color: color.Hash("work"), Sparkline: []float64{100, 100}},
		}
		tree := BuildTagTree(summaries)
		if len(tree) != 2 {
			t.Fatalf("got %d roots, want 2", len(tree))
		}
		// Sorted by TotalSecs desc.
		if tree[0].FullPath != "work" {
			t.Errorf("tree[0].FullPath = %q, want work", tree[0].FullPath)
		}
		if tree[0].TotalSecs != 200 {
			t.Errorf("tree[0].TotalSecs = %v, want 200", tree[0].TotalSecs)
		}
		if tree[0].Children != nil {
			t.Errorf("tree[0].Children should be nil for flat tag")
		}
		if tree[1].FullPath != "sleep" {
			t.Errorf("tree[1].FullPath = %q, want sleep", tree[1].FullPath)
		}
	})

	t.Run("hierarchical tags produce nested tree", func(t *testing.T) {
		summaries := []TagSummary{
			{Name: "coding/monks.co/tagtime", TotalSecs: 500, Sparkline: []float64{250, 250}},
			{Name: "coding/monks.co/ci", TotalSecs: 300, Sparkline: []float64{150, 150}},
			{Name: "coding/ci", TotalSecs: 100, Sparkline: []float64{50, 50}},
		}
		tree := BuildTagTree(summaries)
		if len(tree) != 1 {
			t.Fatalf("got %d roots, want 1 (coding)", len(tree))
		}
		coding := tree[0]
		if coding.Name != "coding" {
			t.Errorf("root.Name = %q, want coding", coding.Name)
		}
		if coding.FullPath != "coding" {
			t.Errorf("root.FullPath = %q, want coding", coding.FullPath)
		}
		if coding.TotalSecs != 900 {
			t.Errorf("coding.TotalSecs = %v, want 900", coding.TotalSecs)
		}
		if coding.OwnSecs != 0 {
			t.Errorf("coding.OwnSecs = %v, want 0 (pure aggregator)", coding.OwnSecs)
		}
		// Check sparkline aggregation.
		if coding.Sparkline[0] != 450 || coding.Sparkline[1] != 450 {
			t.Errorf("coding.Sparkline = %v, want [450, 450]", coding.Sparkline)
		}
		if len(coding.Children) != 2 {
			t.Fatalf("coding has %d children, want 2", len(coding.Children))
		}
		// Children sorted by TotalSecs desc: monks.co (800) before ci (100).
		monksCo := coding.Children[0]
		if monksCo.Name != "monks.co" {
			t.Errorf("child[0].Name = %q, want monks.co", monksCo.Name)
		}
		if monksCo.FullPath != "coding/monks.co" {
			t.Errorf("child[0].FullPath = %q, want coding/monks.co", monksCo.FullPath)
		}
		if monksCo.TotalSecs != 800 {
			t.Errorf("monks.co.TotalSecs = %v, want 800", monksCo.TotalSecs)
		}
		ci := coding.Children[1]
		if ci.Name != "ci" {
			t.Errorf("child[1].Name = %q, want ci", ci.Name)
		}
		if ci.TotalSecs != 100 {
			t.Errorf("ci.TotalSecs = %v, want 100", ci.TotalSecs)
		}
	})

	t.Run("parent with own pings and children", func(t *testing.T) {
		summaries := []TagSummary{
			{Name: "coding", TotalSecs: 200, Sparkline: []float64{100, 100}},
			{Name: "coding/tagtime", TotalSecs: 300, Sparkline: []float64{150, 150}},
		}
		tree := BuildTagTree(summaries)
		if len(tree) != 1 {
			t.Fatalf("got %d roots, want 1", len(tree))
		}
		coding := tree[0]
		if coding.OwnSecs != 200 {
			t.Errorf("coding.OwnSecs = %v, want 200", coding.OwnSecs)
		}
		if coding.TotalSecs != 500 {
			t.Errorf("coding.TotalSecs = %v, want 500", coding.TotalSecs)
		}
		if coding.Sparkline[0] != 250 || coding.Sparkline[1] != 250 {
			t.Errorf("coding.Sparkline = %v, want [250, 250]", coding.Sparkline)
		}
	})

	t.Run("color uses full path", func(t *testing.T) {
		summaries := []TagSummary{
			{Name: "coding/tagtime", TotalSecs: 100, Color: color.Hash("coding/tagtime"), Sparkline: []float64{100}},
		}
		tree := BuildTagTree(summaries)
		leaf := tree[0].Children[0]
		if leaf.Color != color.Hash("coding/tagtime") {
			t.Errorf("leaf.Color = %q, want %q", leaf.Color, color.Hash("coding/tagtime"))
		}
		// Aggregator parent gets color from its own full path.
		if tree[0].Color != color.Hash("coding") {
			t.Errorf("parent.Color = %q, want %q", tree[0].Color, color.Hash("coding"))
		}
	})
}
