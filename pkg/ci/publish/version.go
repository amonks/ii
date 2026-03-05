package publish

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

// NextVersion computes the next version for a package given its version
// track and the latest existing tag.
//
// The version track defines a prefix; the tool appends an incrementing
// numeric suffix separated by ".":
//
//   - track "0.0"         → v0.0.1, v0.0.2, ...
//   - track "1.0"         → v1.0.1, v1.0.2, ...
//   - track "1.0.0-beta"  → v1.0.0-beta.1, v1.0.0-beta.2, ...
//
// If the latest tag matches the track prefix, its trailing number is
// incremented. If the track changed or there's no prior tag, starts at 1.
func NextVersion(track, latestTag, dir string) string {
	if track == "" {
		track = "0.0"
	}
	prefix := "v" + track + "."

	if latestTag != "" {
		ver := MirrorTag(latestTag, dir)
		if strings.HasPrefix(ver, prefix) {
			numStr := strings.TrimPrefix(ver, prefix)
			if n, err := strconv.Atoi(numStr); err == nil {
				return fmt.Sprintf("%s%d", prefix, n+1)
			}
		}
	}

	return prefix + "1"
}

// LatestTag returns the most recent publish tag for a directory, or "" if none.
// Tags are compared using semver ordering.
func LatestTag(root, dir string) (string, error) {
	tags, err := FindMonorepoTags(root, dir)
	if err != nil {
		return "", err
	}
	if len(tags) == 0 {
		return "", nil
	}
	sort.Slice(tags, func(i, j int) bool {
		vi := MirrorTag(tags[i], dir)
		vj := MirrorTag(tags[j], dir)
		return semver.Compare(vi, vj) < 0
	})
	return tags[len(tags)-1], nil
}

// ChangedSinceTag returns true if files in dir changed between tag and HEAD.
// If tag is empty, returns true (first publish).
func ChangedSinceTag(root, dir, tag string) (bool, error) {
	if tag == "" {
		return true, nil
	}
	cmd := exec.Command("git", "diff", "--name-only", tag+"..HEAD", "--", dir)
	cmd.Dir = root
	cmd.Env = GitEnv(root)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git diff %s..HEAD -- %s: %w", tag, dir, err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// CreateTag creates a git tag pointing at HEAD.
func CreateTag(root, tag string) error {
	cmd := exec.Command("git", "tag", tag)
	cmd.Dir = root
	cmd.Env = GitEnv(root)
	return cmd.Run()
}
