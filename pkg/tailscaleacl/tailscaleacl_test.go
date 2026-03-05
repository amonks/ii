package tailscaleacl

import (
	"encoding/json"
	"testing"

	"monks.co/pkg/config"
)

func TestGenerateGrants(t *testing.T) {
	cfg := &config.AppsConfig{
		Defaults: config.Defaults{Region: "ord"},
		Apps: map[string]config.AppEntry{
			"dogs": {Routes: []config.Route{
				{Path: "dogs", Host: "fly", Access: "autogroup:danger-all"},
				{Path: "dogs", Host: "thor", Access: "ajm@passkey"},
			}},
			"movies": {Routes: []config.Route{
				{Path: "movies", Host: "thor", Access: "autogroup:danger-all"},
				{Path: "movies", Host: "thor", Access: "ajm@passkey", Capabilities: []string{"movies-write"}},
			}},
			"logs": {Routes: []config.Route{
				{Path: "logs", Host: "fly", Access: "tag:service"},
			}},
		},
	}

	grants := generateGrants(cfg)

	// Should have grants for 3 access values + 1 capability grant.
	// autogroup:danger-all, ajm@passkey, tag:service, movies-write cap.
	if len(grants) < 3 {
		t.Fatalf("expected at least 3 grants, got %d", len(grants))
	}

	// Verify each grant is valid JSON.
	for i, g := range grants {
		bs, err := json.Marshal(g)
		if err != nil {
			t.Errorf("grant %d: marshal error: %v", i, err)
			continue
		}

		var parsed map[string]any
		if err := json.Unmarshal(bs, &parsed); err != nil {
			t.Errorf("grant %d: unmarshal error: %v", i, err)
		}
	}

	// Find the ajm@passkey grant and check for movies-write capability.
	var foundMoviesWrite bool
	for _, g := range grants {
		if len(g.Src) == 1 && g.Src[0] == "ajm@passkey" {
			if _, ok := g.App["monks.co/cap/movies-write"]; ok {
				foundMoviesWrite = true
			}
		}
	}
	if !foundMoviesWrite {
		t.Error("expected movies-write capability grant for ajm@passkey")
	}
}

func TestDeriveBackend(t *testing.T) {
	tests := []struct {
		app, host, region string
		want              string
	}{
		{"dogs", "fly", "ord", "monks-dogs-fly-ord"},
		{"movies", "thor", "ord", "monks-movies-thor"},
		{"aranet", "brigid", "ord", "monks-aranet-brigid"},
	}
	for _, tt := range tests {
		got := deriveBackend(tt.app, tt.host, tt.region)
		if got != tt.want {
			t.Errorf("deriveBackend(%q, %q, %q) = %q, want %q", tt.app, tt.host, tt.region, got, tt.want)
		}
	}
}

func TestStripJSONCComments(t *testing.T) {
	input := `{
    // this is a comment
    "key": "value" // inline comment
}`
	got := string(stripJSONCComments([]byte(input)))
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("stripped JSONC should be valid JSON: %v\ngot: %s", err, got)
	}
	if parsed["key"] != "value" {
		t.Errorf("expected key=value, got %v", parsed["key"])
	}
}
