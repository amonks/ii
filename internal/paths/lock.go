package paths

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	repoNameDisallowed = regexp.MustCompile(`[^a-z0-9-]`)
	repoNameHyphenRun  = regexp.MustCompile(`-+`)
)

// TodoLockPath returns the cross-process lock file for a repo's todo store.
// The repo path is flattened into the filename so that every workspace of the
// same repo contends for one lock while unrelated repos never do.
func TodoLockPath(repoPath string) (string, error) {
	stateDir, err := DefaultStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, fmt.Sprintf("todo-%s.lock", sanitizeRepoName(repoPath))), nil
}

// sanitizeRepoName converts a file path into a single filename-safe segment.
func sanitizeRepoName(path string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := HomeDir(); err == nil {
			path = filepath.Join(home, rest)
		} else {
			path = rest
		}
	}

	path = strings.ToLower(strings.TrimPrefix(path, "/"))
	path = strings.NewReplacer("/", "-", " ", "-").Replace(path)
	path = repoNameDisallowed.ReplaceAllString(path, "")
	path = repoNameHyphenRun.ReplaceAllString(path, "-")

	return strings.Trim(path, "-")
}
