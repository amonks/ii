package publish

import (
	"testing"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

func TestNextVersion(t *testing.T) {
	tests := []struct {
		name      string
		track     string
		latestTag string
		dir       string
		want      string
	}{
		{
			name: "first publish, default track",
			track: "", latestTag: "", dir: "pkg/set",
			want: "v0.0.1",
		},
		{
			name: "first publish, explicit track",
			track: "1.0", latestTag: "", dir: "cmd/run",
			want: "v1.0.1",
		},
		{
			name: "increment patch",
			track: "0.0", latestTag: "pkg/set/v0.0.3", dir: "pkg/set",
			want: "v0.0.4",
		},
		{
			name: "increment patch with explicit track",
			track: "1.0", latestTag: "cmd/run/v1.0.5", dir: "cmd/run",
			want: "v1.0.6",
		},
		{
			name: "track changed, start fresh",
			track: "2.0", latestTag: "cmd/run/v1.0.5", dir: "cmd/run",
			want: "v2.0.1",
		},
		{
			name: "track changed from default",
			track: "1.0", latestTag: "pkg/set/v0.0.12", dir: "pkg/set",
			want: "v1.0.1",
		},
		{
			name: "pre-release track, first publish",
			track: "1.0.0-beta", latestTag: "", dir: "cmd/run",
			want: "v1.0.0-beta.1",
		},
		{
			name: "pre-release track, increment",
			track: "1.0.0-beta", latestTag: "cmd/run/v1.0.0-beta.37", dir: "cmd/run",
			want: "v1.0.0-beta.38",
		},
		{
			name: "pre-release to release",
			track: "1.0", latestTag: "cmd/run/v1.0.0-beta.37", dir: "cmd/run",
			want: "v1.0.1",
		},
		{
			name: "release to pre-release",
			track: "2.0.0-rc", latestTag: "cmd/run/v1.0.5", dir: "cmd/run",
			want: "v2.0.0-rc.1",
		},
		{
			name: "pre-release track change",
			track: "1.0.0-rc", latestTag: "cmd/run/v1.0.0-beta.37", dir: "cmd/run",
			want: "v1.0.0-rc.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NextVersion(tt.track, tt.latestTag, tt.dir)
			if got != tt.want {
				t.Errorf("NextVersion(%q, %q, %q) = %q, want %q", tt.track, tt.latestTag, tt.dir, got, tt.want)
			}
		})
	}
}

func TestNextVersionProducesValidSemver(t *testing.T) {
	versions := []string{
		NextVersion("", "", "pkg/set"),
		NextVersion("0.0", "pkg/set/v0.0.1", "pkg/set"),
		NextVersion("1.0", "", "cmd/run"),
		NextVersion("1.0", "cmd/run/v1.0.99", "cmd/run"),
		NextVersion("1.0.0-beta", "", "cmd/run"),
		NextVersion("1.0.0-beta", "cmd/run/v1.0.0-beta.37", "cmd/run"),
		NextVersion("2.0.0-rc", "cmd/run/v1.0.5", "cmd/run"),
	}
	for _, v := range versions {
		if !semver.IsValid(v) {
			t.Errorf("%q is not valid semver", v)
		}
		if c := semver.Canonical(v); v != c {
			t.Errorf("%q is not canonical (canonical: %q)", v, c)
		}
	}
}

func TestNextVersionModuleCheck(t *testing.T) {
	// Verify versions pass Go's module version validation.
	versions := []string{
		NextVersion("0.0", "", "pkg/set"),
		NextVersion("1.0", "cmd/run/v1.0.3", "cmd/run"),
		NextVersion("1.0.0-beta", "cmd/run/v1.0.0-beta.5", "cmd/run"),
	}
	for _, v := range versions {
		if err := module.Check("monks.co/pkg/example", v); err != nil {
			t.Errorf("%q fails module.Check: %v", v, err)
		}
	}
}

func TestNextVersionSortsCorrectly(t *testing.T) {
	v1 := NextVersion("0.0", "", "pkg/a")
	v2 := NextVersion("0.0", "pkg/a/"+v1, "pkg/a")
	v3 := NextVersion("0.0", "pkg/a/"+v2, "pkg/a")

	if semver.Compare(v1, v2) >= 0 {
		t.Errorf("expected %s < %s", v1, v2)
	}
	if semver.Compare(v2, v3) >= 0 {
		t.Errorf("expected %s < %s", v2, v3)
	}
}

func TestNextVersionPreReleaseSortsCorrectly(t *testing.T) {
	v1 := NextVersion("1.0.0-beta", "", "cmd/run")
	v2 := NextVersion("1.0.0-beta", "cmd/run/"+v1, "cmd/run")
	v3 := NextVersion("1.0.0-beta", "cmd/run/"+v2, "cmd/run")

	if semver.Compare(v1, v2) >= 0 {
		t.Errorf("expected %s < %s", v1, v2)
	}
	if semver.Compare(v2, v3) >= 0 {
		t.Errorf("expected %s < %s", v2, v3)
	}

	// Release should sort above pre-release.
	release := NextVersion("1.0", "", "cmd/run")
	if semver.Compare(v3, release) >= 0 {
		t.Errorf("expected pre-release %s < release %s", v3, release)
	}
}
