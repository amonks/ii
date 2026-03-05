package publish

import (
	"strings"
	"testing"
)

func TestRewriteGoMod(t *testing.T) {
	t.Run("add new require", func(t *testing.T) {
		input := []byte("module example.com/foo\n\ngo 1.26.0\n")
		out, err := RewriteGoMod(input, map[string]string{
			"monks.co/pkg/migrate": "v0.0.1",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), "monks.co/pkg/migrate v0.0.1") {
			t.Errorf("expected require directive, got:\n%s", out)
		}
	})

	t.Run("update existing require", func(t *testing.T) {
		input := []byte("module example.com/foo\n\ngo 1.26.0\n\nrequire monks.co/pkg/migrate v0.0.1\n")
		out, err := RewriteGoMod(input, map[string]string{
			"monks.co/pkg/migrate": "v0.0.2",
		})
		if err != nil {
			t.Fatal(err)
		}
		s := string(out)
		if !strings.Contains(s, "v0.0.2") {
			t.Errorf("expected updated version, got:\n%s", s)
		}
		if strings.Contains(s, "v0.0.1") {
			t.Errorf("old version should be gone, got:\n%s", s)
		}
	})

	t.Run("update in require block", func(t *testing.T) {
		input := []byte("module example.com/foo\n\ngo 1.26.0\n\nrequire (\n\tmonks.co/pkg/migrate v0.0.1\n\tother.com/bar v1.0.0\n)\n")
		out, err := RewriteGoMod(input, map[string]string{
			"monks.co/pkg/migrate": "v0.0.2",
		})
		if err != nil {
			t.Fatal(err)
		}
		s := string(out)
		if !strings.Contains(s, "monks.co/pkg/migrate v0.0.2") {
			t.Errorf("expected updated version, got:\n%s", s)
		}
		if !strings.Contains(s, "other.com/bar v1.0.0") {
			t.Errorf("other require should be preserved, got:\n%s", s)
		}
	})

	t.Run("multiple requires", func(t *testing.T) {
		input := []byte("module example.com/foo\n\ngo 1.26.0\n")
		out, err := RewriteGoMod(input, map[string]string{
			"monks.co/pkg/migrate": "v0.0.1",
			"monks.co/pkg/set":     "v0.0.3",
		})
		if err != nil {
			t.Fatal(err)
		}
		s := string(out)
		if !strings.Contains(s, "monks.co/pkg/migrate v0.0.1") {
			t.Errorf("expected migrate require, got:\n%s", s)
		}
		if !strings.Contains(s, "monks.co/pkg/set v0.0.3") {
			t.Errorf("expected set require, got:\n%s", s)
		}
	})

	t.Run("empty requires is no-op", func(t *testing.T) {
		input := []byte("module example.com/foo\n\ngo 1.26.0\n")
		out, err := RewriteGoMod(input, nil)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != string(input) {
			t.Errorf("expected no change, got:\n%s", out)
		}
	})
}
