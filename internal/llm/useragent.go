package llm

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// UserAgent returns the HTTP User-Agent header value to use for LLM API requests.
//
// Normal runtime:
//
//	incrementum [$version] $dirname
//
// Where $version is the `commit_id` shown by `ii -v` and $dirname is the base
// name of the repository root.
//
// During `go test` it is overridden to:
//
//	incrementum TEST
func UserAgent(repoPath string, version string) string {
	if testing.Testing() {
		return "incrementum TEST"
	}
	if version == "" {
		version = "unknown"
	}
	repoPath = strings.TrimSpace(repoPath)
	dir := filepath.Base(repoPath)
	if dir == "." || dir == string(filepath.Separator) || dir == "" {
		dir = "unknown"
	}
	return fmt.Sprintf("incrementum [%s] %s", version, dir)
}
