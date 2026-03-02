package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// loadRedundantCommands loads config/redundant-commands, a newline-delimited
// list of commands that are already run at the root level and should not be
// aggregated from app tasks.
func loadRedundantCommands(root string) (map[string]bool, error) {
	path := filepath.Join(root, "config", "redundant-commands")
	bs, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	cmds := map[string]bool{}
	for line := range strings.SplitSeq(string(bs), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			cmds[line] = true
		}
	}
	return cmds, nil
}

// taskfileTasks represents the TOML structure of a tasks.toml file.
type taskfileTasks struct {
	Task []taskfileEntry `toml:"task"`
}

type taskfileEntry struct {
	ID           string   `toml:"id"`
	Type         string   `toml:"type"`
	Cmd          string   `toml:"cmd"`
	Dependencies []string `toml:"dependencies"`
}

// findAppTestTasks discovers test tasks from app/pkg/cmd taskfiles.
// It returns fully-qualified task IDs (e.g., "apps/dungeon/test-js") for
// tasks that have a test-related ID, have a cmd, and whose cmd is not
// in the redundant set.
func findAppTestTasks(root string, redundant map[string]bool) ([]string, error) {
	var result []string

	for _, prefix := range []string{"apps", "pkg", "cmd"} {
		prefixDir := filepath.Join(root, prefix)
		entries, err := os.ReadDir(prefixDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", prefixDir, err)
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}

			taskfilePath := filepath.Join(prefixDir, e.Name(), "tasks.toml")
			if _, err := os.Stat(taskfilePath); os.IsNotExist(err) {
				continue
			}

			var tf taskfileTasks
			if _, err := toml.DecodeFile(taskfilePath, &tf); err != nil {
				return nil, fmt.Errorf("parsing %s: %w", taskfilePath, err)
			}

			dir := filepath.Join(prefix, e.Name())
			for _, t := range tf.Task {
				if !isTestTask(t) {
					continue
				}
				if t.Cmd == "" {
					// Umbrella task with only deps, skip it.
					continue
				}
				if redundant[strings.TrimSpace(t.Cmd)] {
					continue
				}
				result = append(result, dir+"/"+t.ID)
			}
		}
	}

	sort.Strings(result)
	return result, nil
}

// isTestTask returns true if the task ID looks like a test task.
func isTestTask(t taskfileEntry) bool {
	return t.ID == "test" || strings.HasPrefix(t.ID, "test-")
}
