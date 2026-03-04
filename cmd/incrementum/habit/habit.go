// Package habit implements loading and managing habit instruction documents.
//
// Habits are ongoing improvement work without completion state. Unlike regular
// todos, habits are never "done" - they represent continuous practices.
// Habit instructions live in .incrementum/habits/<name>.md files.
package habit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"monks.co/incrementum/internal/ids"
	internalstrings "monks.co/incrementum/internal/strings"
)

// HabitsDir is the directory containing habit instruction documents.
const HabitsDir = ".incrementum/habits"

// Habit represents a loaded habit instruction document.
type Habit struct {
	// Name is the habit name (filename without extension).
	Name string

	// Instructions is the full text of the habit document body (after frontmatter).
	Instructions string

	// ImplementationModel is the model to use for implementation, if specified in frontmatter.
	ImplementationModel string

	// ReviewModel is the model to use for review, if specified in frontmatter.
	ReviewModel string
}

// Load loads a habit by name from the given repo path.
func Load(repoPath, name string) (*Habit, error) {
	name = internalstrings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("habit name is required")
	}

	habitPath := filepath.Join(repoPath, HabitsDir, name+".md")
	data, err := os.ReadFile(habitPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("habit not found: %s", name)
		}
		return nil, fmt.Errorf("read habit %s: %w", name, err)
	}

	return parseHabit(name, data)
}

// List returns the names of all habits in the repo, sorted alphabetically.
func List(repoPath string) ([]string, error) {
	habitsPath := filepath.Join(repoPath, HabitsDir)

	entries, err := os.ReadDir(habitsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read habits directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		names = append(names, strings.TrimSuffix(name, ".md"))
	}

	sort.Strings(names)
	return names, nil
}

// First returns the first habit alphabetically, or nil if no habits exist.
func First(repoPath string) (*Habit, error) {
	names, err := List(repoPath)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}
	return Load(repoPath, names[0])
}

// ErrHabitNotFound is returned when a habit cannot be found.
var ErrHabitNotFound = fmt.Errorf("habit not found")

// ErrAmbiguousHabitPrefix is returned when a habit prefix matches multiple habits.
var ErrAmbiguousHabitPrefix = fmt.Errorf("ambiguous habit prefix")

// Find finds a habit by name or unique prefix.
func Find(repoPath, nameOrPrefix string) (*Habit, error) {
	nameOrPrefix = internalstrings.TrimSpace(nameOrPrefix)
	if nameOrPrefix == "" {
		return nil, ErrHabitNotFound
	}

	names, err := List(repoPath)
	if err != nil {
		return nil, err
	}

	matchName, found, ambiguous := ids.MatchPrefix(names, nameOrPrefix)
	if ambiguous {
		return nil, fmt.Errorf("%w: %s", ErrAmbiguousHabitPrefix, nameOrPrefix)
	}
	if !found {
		return nil, ErrHabitNotFound
	}

	return Load(repoPath, matchName)
}

// LoadAll loads all habits from the repo.
func LoadAll(repoPath string) ([]*Habit, error) {
	names, err := List(repoPath)
	if err != nil {
		return nil, err
	}

	habits := make([]*Habit, 0, len(names))
	for _, name := range names {
		habit, err := Load(repoPath, name)
		if err != nil {
			return nil, err
		}
		habits = append(habits, habit)
	}

	return habits, nil
}

// PrefixLengths returns the unique prefix length for each habit name.
func PrefixLengths(habits []*Habit) map[string]int {
	names := make([]string, len(habits))
	for i, h := range habits {
		names[i] = h.Name
	}
	return ids.UniquePrefixLengths(names)
}

// Path returns the file path for a habit by name.
// It does not check whether the file exists.
func Path(repoPath, name string) (string, error) {
	name = internalstrings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("habit name is required")
	}
	return filepath.Join(repoPath, HabitsDir, name+".md"), nil
}

// Exists returns true if a habit with the given name exists.
func Exists(repoPath, name string) (bool, error) {
	path, err := Path(repoPath, name)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// DefaultTemplate is the template content for a new habit file.
const DefaultTemplate = `# %s

Describe the habit instructions here.

## Guidelines

- Guideline 1
- Guideline 2
`

// Create creates a new habit file with a template.
// Returns the file path and an error if the habit already exists or creation fails.
func Create(repoPath, name string) (string, error) {
	name = internalstrings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("habit name is required")
	}

	path, err := Path(repoPath, name)
	if err != nil {
		return "", err
	}

	// Check if habit already exists
	exists, err := Exists(repoPath, name)
	if err != nil {
		return "", err
	}
	if exists {
		return "", fmt.Errorf("habit already exists: %s", name)
	}

	// Ensure the habits directory exists
	habitsDir := filepath.Join(repoPath, HabitsDir)
	if err := os.MkdirAll(habitsDir, 0755); err != nil {
		return "", fmt.Errorf("create habits directory: %w", err)
	}

	// Write template content
	// Convert name to title case for the template heading
	title := strings.ReplaceAll(name, "-", " ")
	title = strings.ReplaceAll(title, "_", " ")
	content := fmt.Sprintf(DefaultTemplate, title)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write habit file: %w", err)
	}

	return path, nil
}

// parseHabit parses a habit document, extracting frontmatter and body.
func parseHabit(name string, data []byte) (*Habit, error) {
	content := string(data)

	habit := &Habit{Name: name}

	// Check for frontmatter (starts with ---)
	if !strings.HasPrefix(content, "---") {
		habit.Instructions = internalstrings.TrimSpace(content)
		return habit, nil
	}

	// Find the end of frontmatter
	rest := content[3:]
	endIdx := strings.Index(rest, "\n---")
	if endIdx == -1 {
		// No closing ---, treat entire content as instructions
		habit.Instructions = internalstrings.TrimSpace(content)
		return habit, nil
	}

	// Parse frontmatter (simple key-value parsing for our limited schema)
	fmData := rest[:endIdx]
	implModel, reviewModel := parseFrontmatter(fmData)
	habit.ImplementationModel = implModel
	habit.ReviewModel = reviewModel

	// Extract body after frontmatter
	bodyStart := endIdx + 4 // Skip "\n---"
	if bodyStart < len(rest) {
		body := rest[bodyStart:]
		// Skip leading newline after closing ---
		body = strings.TrimPrefix(body, "\n")
		habit.Instructions = internalstrings.TrimSpace(body)
	}

	return habit, nil
}

// parseFrontmatter extracts model configuration from simple YAML frontmatter.
// Expected format:
//
//	models:
//	  implementation: <model>
//	  review: <model>
func parseFrontmatter(data string) (implementationModel, reviewModel string) {
	lines := strings.Split(data, "\n")
	inModels := false

	for _, line := range lines {
		trimmed := internalstrings.TrimSpace(line)

		// Check for models: section
		if trimmed == "models:" {
			inModels = true
			continue
		}

		// Outside models section, check if we've left it
		if inModels && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && trimmed != "" {
			inModels = false
		}

		if !inModels {
			continue
		}

		// Parse implementation: or review: within models section
		if after, ok := strings.CutPrefix(trimmed, "implementation:"); ok {
			implementationModel = internalstrings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(trimmed, "review:"); ok {
			reviewModel = internalstrings.TrimSpace(after)
		}
	}

	return implementationModel, reviewModel
}
