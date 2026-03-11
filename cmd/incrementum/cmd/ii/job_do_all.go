package main

import (
	"errors"
	"fmt"

	"monks.co/incrementum/habit"
	"monks.co/incrementum/internal/jj"
	"monks.co/incrementum/internal/paths"
	internalstrings "monks.co/incrementum/internal/strings"
	"monks.co/incrementum/internal/validation"
	jobpkg "monks.co/incrementum/job"
	"monks.co/incrementum/todo"
	"github.com/spf13/cobra"
)

var jobDoAllCmd = &cobra.Command{
	Use:   "do-all",
	Short: "Run jobs for all ready todos",
	Args:  cobra.NoArgs,
	RunE:  runJobDoAll,
}

var (
	jobDoAllPriority int
	jobDoAllType     string
	jobDoAllHabits   bool
)

type jobDoAllFilter struct {
	maxPriority *int
	todoType    *todo.TodoType
}

func init() {
	jobCmd.AddCommand(jobDoAllCmd)

	jobDoAllCmd.Flags().IntVar(&jobDoAllPriority, "priority", -1, "Filter by priority (0-4, includes higher priorities)")
	jobDoAllCmd.Flags().StringVar(&jobDoAllType, "type", "", "Filter by type (task, bug, feature); design todos are excluded")
	jobDoAllCmd.Flags().BoolVar(&jobDoAllHabits, "habits", false, "Run habits after todo queue is empty (round-robin)")
}

func runJobDoAll(cmd *cobra.Command, args []string) error {
	filter, err := jobDoAllFilters(cmd)
	if err != nil {
		return err
	}

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	// Track habit round-robin state
	var habitNames []string
	habitIndex := 0
	if jobDoAllHabits {
		habitNames, err = habit.List(repoPath)
		if err != nil {
			return err
		}
	}

	for {
		store, handled, err := openTodoStoreReadOnlyOrEmpty(cmd, args, false, func() error {
			if !jobDoAllHabits || len(habitNames) == 0 {
				fmt.Println("nothing left to do")
			}
			return nil
		})
		if err != nil {
			return err
		}
		if handled {
			// No todos - check if we should run habits
			if !jobDoAllHabits || len(habitNames) == 0 {
				return nil
			}
			// Run the next habit in round-robin order
			if err := runDoAllHabit(cmd, repoPath, habitNames[habitIndex]); err != nil {
				return err
			}
			habitIndex = (habitIndex + 1) % len(habitNames)
			continue
		}

		todoID, err := nextJobDoAllTodoID(store, filter)
		store.Release()
		if err != nil {
			return err
		}
		if todoID == "" {
			// No ready todos - check if we should run habits
			if !jobDoAllHabits || len(habitNames) == 0 {
				fmt.Println("nothing left to do")
				return nil
			}
			// Run the next habit in round-robin order
			if err := runDoAllHabit(cmd, repoPath, habitNames[habitIndex]); err != nil {
				return err
			}
			habitIndex = (habitIndex + 1) % len(habitNames)
			continue
		}

		// Reset habit index when we have todos (prioritize todos)
		habitIndex = 0

		if err := runJobDoTodo(cmd, todoID); err != nil {
			return err
		}
	}
}

func runDoAllHabit(cmd *cobra.Command, repoPath, habitName string) error {
	h, err := habit.Load(repoPath, habitName)
	if err != nil {
		return err
	}

	// Get the workspace path from the current working directory.
	// This ensures we run jobs in the workspace we're currently in,
	// not the source repo root.
	cwd, err := paths.WorkingDir()
	if err != nil {
		return err
	}
	client := jj.New()
	workspacePath, err := client.WorkspaceRoot(cwd)
	if err != nil {
		return err
	}

	runner, err := makeAgentRunnerFunc(repoPath)
	if err != nil {
		return err
	}

	// Set up LLM runner
	runLLM, err := makeRunLLMFunc(repoPath, runner)
	if err != nil {
		return err
	}
	logger := jobpkg.NewConsoleLogger(nil)
	reporter := newJobStageReporter(logger)
	onStageChange := reporter.OnStageChange
	onStart := func(info jobpkg.HabitStartInfo) {
		printHabitJobStart(info, h)
	}

	result, err := jobpkg.RunHabit(repoPath, h.Name, jobpkg.HabitRunOptions{
		OnStart:       onStart,
		OnStageChange: onStageChange,
		Logger:        logger,
		RunLLM:        runLLM,
		WorkspacePath: workspacePath,
	})
	if err != nil {
		var abandonedErr *jobpkg.AbandonedError
		if errors.As(err, &abandonedErr) {
			fmt.Printf("\n%s\n", formatAbandonReasonOutput(abandonedErr.Reason))
			return err
		}
		return err
	}

	if result.Abandoned {
		fmt.Println("\nNothing worth doing right now.")
		return nil
	}

	if result.Artifact != nil {
		fmt.Printf("\nCreated artifact todo: %s\n", result.Artifact.ID)
	}

	return nil
}

func jobDoAllFilters(cmd *cobra.Command) (jobDoAllFilter, error) {
	filter := jobDoAllFilter{}
	if cmd.Flags().Changed("priority") {
		if err := todo.ValidatePriority(jobDoAllPriority); err != nil {
			return filter, err
		}
		filter.maxPriority = &jobDoAllPriority
	}

	if cmd.Flags().Changed("type") {
		normalized := todo.TodoType(internalstrings.NormalizeLowerTrimSpace(jobDoAllType))
		if !normalized.IsValid() {
			return filter, validation.FormatInvalidValueError(todo.ErrInvalidType, normalized, todo.ValidTodoTypes())
		}
		if normalized.IsInteractive() {
			return filter, fmt.Errorf("%s todos require interactive sessions and cannot be run with do-all", normalized)
		}
		filter.todoType = &normalized
	}

	return filter, nil
}

func nextJobDoAllTodoID(store *todo.Store, filter jobDoAllFilter) (string, error) {
	todos, err := store.Ready(0)
	if err != nil {
		return "", err
	}

	for _, item := range todos {
		// Skip interactive todos (e.g., design) since they require user collaboration
		if item.Type.IsInteractive() {
			continue
		}
		if filter.maxPriority != nil && item.Priority > *filter.maxPriority {
			continue
		}
		if filter.todoType != nil && item.Type != *filter.todoType {
			continue
		}
		return item.ID, nil
	}
	return "", nil
}
