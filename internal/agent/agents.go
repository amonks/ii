package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// agentsPrelude loads AGENTS.md from the working directory if present and
// returns it formatted as a prelude to the user's first prompt.
//
// If the file does not exist, it returns an empty string.
func agentsPrelude(workDir string) (string, error) {
	path := filepath.Join(workDir, "AGENTS.md")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	content := strings.TrimSpace(string(b))
	if content == "" {
		return "", nil
	}

	return content + "\n\n", nil
}
