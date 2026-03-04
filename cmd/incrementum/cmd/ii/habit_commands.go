package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/amonks/incrementum/habit"
	"github.com/amonks/incrementum/internal/editor"
	"github.com/amonks/incrementum/internal/ui"
	"github.com/amonks/incrementum/job"
	"github.com/spf13/cobra"
)

var habitCmd = &cobra.Command{
	Use:   "habit",
	Short: "Manage habits for the current repository",
}

// habit list
var habitListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all habits",
	Args:  cobra.NoArgs,
	RunE:  runHabitList,
}

// habit show
var habitShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show habit instructions",
	Args:  cobra.ExactArgs(1),
	RunE:  runHabitShow,
}

// habit edit
var habitEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Edit habit instructions in $EDITOR",
	Aliases: []string{
		"update",
	},
	Args: cobra.ExactArgs(1),
	RunE: runHabitEdit,
}

// habit create
var habitCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new habit and open it in $EDITOR",
	Args:  cobra.ExactArgs(1),
	RunE:  runHabitCreate,
}

func init() {
	rootCmd.AddCommand(habitCmd)
	habitCmd.AddCommand(habitListCmd, habitShowCmd, habitEditCmd, habitCreateCmd)
}

func runHabitList(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	habits, err := habit.LoadAll(repoPath)
	if err != nil {
		return err
	}

	if len(habits) == 0 {
		fmt.Println("No habits found.")
		return nil
	}

	// Get job counts per habit
	manager, err := job.Open(repoPath, job.OpenOptions{})
	if err != nil {
		return fmt.Errorf("open job manager: %w", err)
	}
	jobCounts, err := manager.CountByHabit()
	if err != nil {
		return fmt.Errorf("count jobs by habit: %w", err)
	}

	prefixLengths := habit.PrefixLengths(habits)
	printHabitTable(habits, prefixLengths, jobCounts)
	return nil
}

func printHabitTable(habits []*habit.Habit, prefixLengths map[string]int, jobCounts map[string]int) {
	builder := ui.NewTableBuilder([]string{"NAME", "IMPL MODEL", "REVIEW MODEL", "JOBS"}, len(habits))

	for _, h := range habits {
		prefixLen := ui.PrefixLength(prefixLengths, h.Name)
		highlighted := ui.HighlightID(h.Name, prefixLen)

		implModel := h.ImplementationModel
		if implModel == "" {
			implModel = "-"
		}
		reviewModel := h.ReviewModel
		if reviewModel == "" {
			reviewModel = "-"
		}

		jobCount := strconv.Itoa(jobCounts[h.Name])

		row := []string{
			highlighted,
			implModel,
			reviewModel,
			jobCount,
		}
		builder.AddRow(row)
	}

	fmt.Print(builder.String())
}

func runHabitShow(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	nameOrPrefix := args[0]
	h, err := habit.Find(repoPath, nameOrPrefix)
	if err != nil {
		if errors.Is(err, habit.ErrHabitNotFound) {
			return fmt.Errorf("habit not found: %s", nameOrPrefix)
		}
		if errors.Is(err, habit.ErrAmbiguousHabitPrefix) {
			return fmt.Errorf("ambiguous habit prefix: %s", nameOrPrefix)
		}
		return err
	}

	path, err := habit.Path(repoPath, h.Name)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read habit: %w", err)
	}

	fmt.Print(string(content))
	return nil
}

func runHabitEdit(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	nameOrPrefix := args[0]
	h, err := habit.Find(repoPath, nameOrPrefix)
	if err != nil {
		if errors.Is(err, habit.ErrHabitNotFound) {
			return fmt.Errorf("habit not found: %s", nameOrPrefix)
		}
		if errors.Is(err, habit.ErrAmbiguousHabitPrefix) {
			return fmt.Errorf("ambiguous habit prefix: %s", nameOrPrefix)
		}
		return err
	}

	path, err := habit.Path(repoPath, h.Name)
	if err != nil {
		return err
	}

	return editor.Edit(path)
}

func runHabitCreate(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	name := args[0]
	path, err := habit.Create(repoPath, name)
	if err != nil {
		return err
	}

	fmt.Printf("Created habit: %s\n", path)
	return editor.Edit(path)
}
