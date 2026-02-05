package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/amonks/incrementum/agent"
	"github.com/amonks/incrementum/habit"
	"github.com/amonks/incrementum/internal/editor"
	"github.com/amonks/incrementum/internal/jj"
	"github.com/amonks/incrementum/internal/paths"
	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/internal/todoenv"
	jobpkg "github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/todo"
	"github.com/muesli/reflow/wordwrap"
	"github.com/spf13/cobra"
)

var jobDoCmd = &cobra.Command{
	Use:   "do [todo-id...]",
	Short: "Run a job for one or more todos",
	Args:  cobra.ArbitraryArgs,
	RunE:  runJobDo,
}

var jobRun = jobpkg.Run

// jobDoTodo is the function called to run a single todo. It can be overridden for testing.
var jobDoTodo = runJobDoTodo

// runInteractiveSession is the function called to run an interactive agent session.
// It can be overridden for testing.
var runInteractiveSession = defaultRunInteractiveSession

// designTodoStore is an interface for the store operations needed by runDesignTodo.
// This allows mocking store operations in tests.
type designTodoStore interface {
	Start(ids []string) ([]todo.Todo, error)
	Release() error
}

// openDesignTodoStore opens a store for design todo operations.
// It can be overridden for testing.
var openDesignTodoStore = func(repoPath, purpose string) (designTodoStore, error) {
	return todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		Purpose:         purpose,
	})
}

// queueStore is an interface for the store operations needed for queueing/cleanup.
// This allows mocking store operations in tests.
type queueStore interface {
	Queue(ids []string) ([]todo.Todo, error)
	Reopen(ids []string) ([]todo.Todo, error)
	Release() error
}

// openQueueStore opens a store for queue operations.
// It can be overridden for testing.
var openQueueStore = func(repoPath, purpose string) (queueStore, error) {
	return todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		Purpose:         purpose,
	})
}
var (
	jobDoTitle               string
	jobDoType                string
	jobDoPriority            int
	jobDoDescription         string
	jobDoImplementationModel string
	jobDoCodeReviewModel     string
	jobDoProjectReviewModel  string
	jobDoDeps                []string
	jobDoEdit    bool
	jobDoNoEdit  bool
	jobDoHabit   string
)

func init() {
	jobCmd.AddCommand(jobDoCmd)
	addDescriptionFlagAliases(jobDoCmd)

	jobDoCmd.Flags().StringVar(&jobDoTitle, "title", "", "Todo title")
	jobDoCmd.Flags().StringVarP(&jobDoType, "type", "t", "task", "Todo type (task, bug, feature, design)")
	jobDoCmd.Flags().IntVarP(&jobDoPriority, "priority", "p", todo.PriorityMedium, "Priority (0=critical, 1=high, 2=medium, 3=low, 4=backlog)")
	jobDoCmd.Flags().StringVarP(&jobDoDescription, "description", "d", "", "Description (use '-' to read from stdin)")
	jobDoCmd.Flags().StringVar(&jobDoImplementationModel, "implementation-model", "", "LLM model for implementation")
	jobDoCmd.Flags().StringVar(&jobDoCodeReviewModel, "code-review-model", "", "LLM model for commit review")
	jobDoCmd.Flags().StringVar(&jobDoProjectReviewModel, "project-review-model", "", "LLM model for project review")
	jobDoCmd.Flags().StringArrayVar(&jobDoDeps, "deps", nil, "Dependencies in format <id> (e.g., abc123)")
	jobDoCmd.Flags().BoolVarP(&jobDoEdit, "edit", "e", false, "Open $EDITOR (default if interactive and no create flags)")
	jobDoCmd.Flags().BoolVar(&jobDoNoEdit, "no-edit", false, "Do not open $EDITOR")
	jobDoCmd.Flags().StringVar(&jobDoHabit, "habit", "", "Run a habit instead of a todo (use habit name or empty for first)")
	// Allow --habit without a value to run the first habit alphabetically
	jobDoCmd.Flags().Lookup("habit").NoOptDefVal = " "
}

func runJobDo(cmd *cobra.Command, args []string) error {
	if err := resolveDescriptionFlag(cmd, &jobDoDescription, os.Stdin); err != nil {
		return err
	}

	// Handle --habit flag
	if cmd.Flags().Changed("habit") {
		hasCreateFlags := hasTodoCreateFlags(cmd)
		if hasCreateFlags || jobDoEdit || jobDoNoEdit {
			return fmt.Errorf("--habit cannot be combined with todo creation flags")
		}
		if len(args) > 0 {
			return fmt.Errorf("--habit cannot be combined with todo ids")
		}
		return runHabitJob(cmd)
	}

	hasCreateFlags := hasTodoCreateFlags(cmd)
	if len(args) > 0 && (hasCreateFlags || jobDoEdit || jobDoNoEdit) {
		return fmt.Errorf("todo id cannot be combined with todo creation flags")
	}

	todoIDs := args
	if len(todoIDs) == 0 {
		createdID, err := createTodoForJob(cmd, hasCreateFlags)
		if err != nil {
			return err
		}
		todoIDs = []string{createdID}
	}

	// When multiple todos are specified, mark them all as queued first.
	// This shows users that all specified todos are accounted for.
	// We need to clean up (reset to open) any remaining queued todos on exit.
	if len(todoIDs) > 1 {
		repoPath, err := getRepoPath()
		if err != nil {
			return err
		}
		store, err := openQueueStore(repoPath, "job do queue todos")
		if err != nil {
			return err
		}
		if _, err := store.Queue(todoIDs); err != nil {
			releaseErr := store.Release()
			return errors.Join(err, releaseErr)
		}
		if err := store.Release(); err != nil {
			return err
		}
	}

	// Track which todos remain to be processed so we can clean them up on early exit.
	// nextIndex points to the todo currently being processed; todos at [nextIndex+1:]
	// are still queued and untouched. On error, we only reopen those (not the
	// currently-running one which may have transitioned to in_progress).
	nextIndex := 0

	// Ensure we clean up any remaining queued todos if we exit early.
	// This runs on both success (no-op, no remaining todos) and failure.
	var jobErr error
	defer func() {
		// Only reopen todos that were never started (after the one that errored).
		// The currently-running todo may have transitioned to in_progress before
		// erroring, so we don't want to clobber that state.
		startOfRemaining := nextIndex + 1
		if startOfRemaining > len(todoIDs) {
			startOfRemaining = len(todoIDs)
		}
		todosToReopen := todoIDs[startOfRemaining:]
		if len(todosToReopen) > 0 && len(todoIDs) > 1 {
			cleanupErr := cleanupQueuedTodos(todosToReopen)
			if cleanupErr != nil {
				// Combine job error and cleanup error so operators see both
				jobErr = errors.Join(jobErr, cleanupErr)
			}
		}
	}()

	for i, todoID := range todoIDs {
		nextIndex = i
		if err := jobDoTodo(cmd, todoID); err != nil {
			jobErr = err
			return jobErr
		}
	}

	return jobErr
}

// cleanupQueuedTodos resets any remaining queued todos back to open status.
// This is called when job do exits before processing all queued todos.
func cleanupQueuedTodos(todoIDs []string) error {
	if len(todoIDs) == 0 {
		return nil
	}
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}
	store, err := openQueueStore(repoPath, "job do cleanup queued todos")
	if err != nil {
		return err
	}

	// Reopen resets todos to open status
	_, reopenErr := store.Reopen(todoIDs)
	releaseErr := store.Release()
	return errors.Join(reopenErr, releaseErr)
}

// runHabitJob runs a habit job using the --habit flag value.
func runHabitJob(cmd *cobra.Command) error {
	repoPath, err := getRepoPath()
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

	var h *habit.Habit
	habitName := strings.TrimSpace(jobDoHabit)
	if habitName == "" {
		// Empty --habit means run the first habit alphabetically
		h, err = habit.First(repoPath)
		if err != nil {
			return err
		}
		if h == nil {
			return fmt.Errorf("no habits found in %s", habit.HabitsDir)
		}
	} else {
		// Use Find to support prefix matching, consistent with habit show/edit
		h, err = habit.Find(repoPath, habitName)
		if err != nil {
			if errors.Is(err, habit.ErrHabitNotFound) {
				return fmt.Errorf("habit not found: %s", habitName)
			}
			if errors.Is(err, habit.ErrAmbiguousHabitPrefix) {
				return fmt.Errorf("ambiguous habit prefix: %s", habitName)
			}
			return err
		}
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
	transcripts := makeTranscriptsFunc()

	logger := jobpkg.NewConsoleLogger(os.Stdout)
	reporter := newJobStageReporter(logger)
	onStageChange := reporter.OnStageChange
	onStart := func(info jobpkg.HabitStartInfo) {
		printHabitJobStart(info, h)
	}
	eventStream := make(chan jobpkg.Event, 128)
	eventErrs := make(chan error, 1)
	eventDone := make(chan struct{})
	go func() {
		formatter := jobpkg.NewEventFormatterWithRepoPath(repoPath)
		var streamErr error
		for {
			select {
			case event, ok := <-eventStream:
				if !ok {
					eventErrs <- streamErr
					return
				}
				if strings.HasPrefix(event.Name, "job.") {
					continue
				}
				if err := appendAndPrintEvent(formatter, event); err != nil {
					if streamErr == nil {
						streamErr = err
					}
				}
			case <-eventDone:
				eventErrs <- streamErr
				return
			}
		}
	}()

	result, err := jobpkg.RunHabit(repoPath, h.Name, jobpkg.HabitRunOptions{
		OnStart:       onStart,
		OnStageChange: onStageChange,
		Logger:        logger,
		EventStream:   eventStream,
		RunLLM:        runLLM,
		Transcripts:   transcripts,
		WorkspacePath: workspacePath,
	})
	close(eventDone)
	streamErr := <-eventErrs
	if err != nil {
		var abandonedErr *jobpkg.AbandonedError
		if errors.As(err, &abandonedErr) {
			fmt.Printf("\n%s\n", formatAbandonReasonOutput(abandonedErr.Reason))
			return err
		}
		return err
	}
	if streamErr != nil {
		return streamErr
	}

	if result.Abandoned {
		fmt.Println("\nNothing worth doing right now.")
		return nil
	}

	if result.Artifact != nil {
		fmt.Printf("\nCreated artifact todo: %s\n", result.Artifact.ID)
		fmt.Printf("Title: %s\n", result.Artifact.Title)
	}

	if !internalstrings.IsBlank(result.CommitMessage) {
		fmt.Printf("\n%s\n", formatCommitMessageOutput(result.CommitMessage))
	}
	return nil
}

func printHabitJobStart(info jobpkg.HabitStartInfo, h *habit.Habit) {
	fmt.Printf("Doing habit job %s\n", info.JobID)
	fmt.Printf("Workdir: %s\n", info.Workdir)
	fmt.Printf("Habit: %s\n", info.HabitName)
	fmt.Printf("Instructions:\n%s\n\n", jobpkg.IndentBlock(h.Instructions, jobDocumentIndent))
}

func runJobDoTodo(cmd *cobra.Command, todoID string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	// Look up the todo to check its type
	store, err := todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		Purpose:         fmt.Sprintf("job do %s", todoID),
	})
	if err != nil {
		return err
	}

	items, err := store.Show([]string{todoID})
	if err != nil {
		releaseErr := store.Release()
		return errors.Join(err, releaseErr)
	}
	if len(items) == 0 {
		releaseErr := store.Release()
		return errors.Join(fmt.Errorf("todo not found: %s", todoID), releaseErr)
	}
	item := items[0]
	if err := store.Release(); err != nil {
		return err
	}

	// Design todos require interactive sessions
	if item.Type.IsInteractive() {
		return runDesignTodo(cmd, repoPath, item)
	}

	return runHeadlessJob(cmd, repoPath, todoID)
}

// interactiveSessionOptions contains the parameters for running an interactive session.
type interactiveSessionOptions struct {
	repoPath      string
	workspacePath string
	prompt        string
	model         string
}

// interactiveSessionResult contains the result of an interactive session.
type interactiveSessionResult struct {
	exitCode int
}

// defaultRunInteractiveSession runs an interactive agent session.
func defaultRunInteractiveSession(opts interactiveSessionOptions) (interactiveSessionResult, error) {
	store, err := makeAgentRunnerFunc(opts.repoPath)
	if err != nil {
		return interactiveSessionResult{}, err
	}

	// Set up signal handling for graceful cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		cancel()
	}()

	handle, err := store.Run(ctx, agent.RunOptions{
		RepoPath:  opts.repoPath,
		WorkDir:   opts.workspacePath,
		Prompt:    opts.prompt,
		Model:     opts.model,
		StartedAt: time.Now(),
		Version:   buildCommitID,
		Env:       []string{todoenv.ProposerEnvVar + "=true"},
	})
	if err != nil {
		return interactiveSessionResult{}, err
	}

	// Stream events to stderr (same as agent run command)
	streamAgentEventsToStderr(handle.Events)

	result, err := handle.Wait()
	if err != nil {
		return interactiveSessionResult{}, err
	}

	return interactiveSessionResult{exitCode: result.ExitCode}, nil
}

// runDesignTodo runs an interactive agent session for design todos.
func runDesignTodo(cmd *cobra.Command, repoPath string, item todo.Todo) error {
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

	// Mark the todo as started
	store, err := openDesignTodoStore(repoPath, fmt.Sprintf("design todo %s start", item.ID))
	if err != nil {
		return err
	}
	if _, err := store.Start([]string{item.ID}); err != nil {
		releaseErr := store.Release()
		return errors.Join(err, releaseErr)
	}
	if releaseErr := store.Release(); releaseErr != nil {
		// Todo was marked in_progress but we failed to release the store.
		// Attempt to reopen it to avoid leaving it stuck in in_progress.
		// This is best-effort: if the same underlying issue (e.g., lock
		// contention) prevents reopening, we return both errors.
		reopenErr := reopenDesignTodo(repoPath, item.ID)
		return errors.Join(releaseErr, reopenErr)
	}

	fmt.Printf("Starting design session for todo %s\n", item.ID)
	fmt.Printf("Title: %s\n", item.Title)
	if !internalstrings.IsBlank(item.Description) {
		fmt.Printf("Description:\n%s\n", jobpkg.IndentBlock(item.Description, jobDocumentIndent))
	}
	fmt.Println()

	// Build a prompt for the design session
	prompt := fmt.Sprintf("You are working on a design todo.\n\n%s", formatDesignTodoBlock(item))

	// For design todos, use the todo's implementation model if set,
	// otherwise the agent store will resolve the model via priority chain
	// (INCREMENTUM_AGENT_MODEL env var -> config implementation-model)
	result, err := runInteractiveSession(interactiveSessionOptions{
		repoPath:      repoPath,
		workspacePath: workspacePath,
		prompt:        prompt,
		model:         item.ImplementationModel,
	})
	if err != nil {
		reopenErr := reopenDesignTodo(repoPath, item.ID)
		return errors.Join(err, reopenErr)
	}

	// Mark todo as done on successful completion
	if result.exitCode == 0 {
		store, err := todo.Open(repoPath, todo.OpenOptions{
			CreateIfMissing: false,
			PromptToCreate:  false,
			Purpose:         fmt.Sprintf("design todo %s finish", item.ID),
		})
		if err != nil {
			return err
		}
		if _, err := store.Finish([]string{item.ID}); err != nil {
			releaseErr := store.Release()
			return errors.Join(err, releaseErr)
		}
		if err := store.Release(); err != nil {
			return err
		}
		fmt.Printf("\nDesign todo %s marked as done.\n", item.ID)
	}

	if result.exitCode != 0 {
		reopenErr := reopenDesignTodo(repoPath, item.ID)
		return errors.Join(exitError{code: result.exitCode}, reopenErr)
	}
	return nil
}

// reopenDesignTodo reopens a design todo after a failed interactive session.
// This uses todo.Open directly rather than openDesignTodoStore because:
// 1. The reopen operation needs Reopen() which isn't in designTodoStore interface
// 2. Reopen is best-effort recovery - we don't need to mock it in tests
// 3. Tests verify reopen behavior by checking the final todo status
func reopenDesignTodo(repoPath, todoID string) error {
	store, err := todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		Purpose:         fmt.Sprintf("design todo %s reopen", todoID),
	})
	if err != nil {
		return err
	}
	_, err = store.Reopen([]string{todoID})
	releaseErr := store.Release()
	return errors.Join(err, releaseErr)
}

func formatDesignTodoBlock(item todo.Todo) string {
	description := internalstrings.TrimTrailingNewlines(item.Description)
	if internalstrings.IsBlank(description) {
		description = "-"
	}
	description = jobpkg.ReflowIndentedText(description, jobLineWidth, jobSubdocumentIndent)
	fields := []string{
		fmt.Sprintf("ID: %s", item.ID),
		fmt.Sprintf("Title: %s", item.Title),
		fmt.Sprintf("Type: %s", item.Type),
		fmt.Sprintf("Priority: %d", item.Priority),
		"Description:",
	}
	fieldBlock := jobpkg.IndentBlock(strings.Join(fields, "\n"), jobDocumentIndent)
	return fmt.Sprintf("Todo\n\n%s\n%s", fieldBlock, description)
}

func runHeadlessJob(cmd *cobra.Command, repoPath, todoID string) error {
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
	transcripts := makeTranscriptsFunc()

	logger := jobpkg.NewConsoleLogger(os.Stdout)
	reporter := newJobStageReporter(logger)
	onStageChange := reporter.OnStageChange
	onStart := func(info jobpkg.StartInfo) {
		printJobStart(info)
	}
	eventStream := make(chan jobpkg.Event, 128)
	eventErrs := make(chan error, 1)
	eventDone := make(chan struct{})
	go func() {
		formatter := jobpkg.NewEventFormatterWithRepoPath(repoPath)
		var streamErr error
		for {
			select {
			case event, ok := <-eventStream:
				if !ok {
					eventErrs <- streamErr
					return
				}
				if strings.HasPrefix(event.Name, "job.") {
					continue
				}
				if err := appendAndPrintEvent(formatter, event); err != nil {
					if streamErr == nil {
						streamErr = err
					}
				}
			case <-eventDone:
				eventErrs <- streamErr
				return
			}
		}
	}()

	result, err := jobRun(repoPath, todoID, jobpkg.RunOptions{
		OnStart:       onStart,
		OnStageChange: onStageChange,
		Logger:        logger,
		EventStream:   eventStream,
		RunLLM:        runLLM,
		Transcripts:   transcripts,
		WorkspacePath: workspacePath,
	})
	close(eventDone)
	streamErr := <-eventErrs
	if err != nil {
		var abandonedErr *jobpkg.AbandonedError
		if errors.As(err, &abandonedErr) {
			fmt.Printf("\n%s\n", formatAbandonReasonOutput(abandonedErr.Reason))
			return err
		}
		return err
	}
	if streamErr != nil {
		return streamErr
	}

	if !internalstrings.IsBlank(result.CommitMessage) {
		fmt.Printf("\n%s\n", formatCommitMessageOutput(result.CommitMessage))
	}
	return nil
}

func formatCommitMessageOutput(message string) string {
	formatted := formatCommitMessageBody(message, jobDocumentIndent)
	return fmt.Sprintf("Commit message:\n\n%s", formatted)
}

func formatAbandonReasonOutput(reason string) string {
	formatted := formatCommitMessageBody(reason, jobDocumentIndent)
	return fmt.Sprintf("Job abandoned:\n\n%s", formatted)
}

func formatCommitMessageBody(message string, indent int) string {
	message = internalstrings.TrimTrailingNewlines(message)
	return jobpkg.ReflowIndentedText(message, jobLineWidth, indent)
}

func printJobStart(info jobpkg.StartInfo) {
	fmt.Printf("Doing job %s\n", info.JobID)
	fmt.Printf("Workdir: %s\n", info.Workdir)
	fmt.Println("Todo:")
	indent := strings.Repeat(" ", jobDocumentIndent)
	fmt.Printf("%s\n", formatJobField("ID", info.Todo.ID))
	fmt.Printf("%s\n", formatJobField("Title", info.Todo.Title))
	fmt.Printf("%s\n", formatJobField("Type", string(info.Todo.Type)))
	fmt.Printf("%s\n", formatJobField("Priority", fmt.Sprintf("%d (%s)", info.Todo.Priority, todo.PriorityName(info.Todo.Priority))))
	fmt.Printf("%sDescription:\n", indent)
	description := reflowJobText(info.Todo.Description, jobLineWidth-jobSubdocumentIndent)
	fmt.Printf("%s\n\n", jobpkg.IndentBlock(description, jobSubdocumentIndent))
}

func createTodoForJob(cmd *cobra.Command, hasCreateFlags bool) (string, error) {
	return createTodoFromJobFlags(cmd, hasCreateFlags, func() (*todo.Store, error) {
		return openTodoStore(cmd, nil)
	})
}

func createTodoFromJobFlags(cmd *cobra.Command, hasCreateFlags bool, openStore func() (*todo.Store, error)) (string, error) {
	useEditor := shouldUseEditor(hasCreateFlags, jobDoEdit, jobDoNoEdit, editor.IsInteractive())
	if useEditor {
		data := editor.DefaultCreateData()
		data.Status = string(defaultTodoStatus())
		if cmd.Flags().Changed("title") {
			data.Title = jobDoTitle
		}
		if cmd.Flags().Changed("type") {
			data.Type = jobDoType
		}
		if cmd.Flags().Changed("priority") {
			data.Priority = jobDoPriority
		}
		if cmd.Flags().Changed("description") {
			data.Description = jobDoDescription
		}
		if cmd.Flags().Changed("implementation-model") {
			data.ImplementationModel = jobDoImplementationModel
		}
		if cmd.Flags().Changed("code-review-model") {
			data.CodeReviewModel = jobDoCodeReviewModel
		}
		if cmd.Flags().Changed("project-review-model") {
			data.ProjectReviewModel = jobDoProjectReviewModel
		}

		parsed, err := editor.EditTodoWithDataRetry(data, todo.StdioPrompter{})
		if err != nil {
			return "", err
		}

		store, err := openStore()
		if err != nil {
			return "", err
		}
		defer store.Release()

		opts := parsed.ToCreateOptions()
		opts.Dependencies = jobDoDeps
		created, err := store.Create(parsed.Title, opts)
		if err != nil {
			return "", err
		}
		return created.ID, nil
	}

	if jobDoTitle == "" {
		return "", fmt.Errorf("title is required (use --edit to open editor)")
	}

	store, err := openStore()
	if err != nil {
		return "", err
	}
	defer store.Release()

	created, err := store.Create(jobDoTitle, todo.CreateOptions{
		Status:              defaultTodoStatus(),
		Type:                todo.TodoType(jobDoType),
		Priority:            jobDoPriorityValue(cmd),
		Description:         jobDoDescription,
		ImplementationModel: jobDoImplementationModel,
		CodeReviewModel:     jobDoCodeReviewModel,
		ProjectReviewModel:  jobDoProjectReviewModel,
		Dependencies:        jobDoDeps,
	})
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func jobDoPriorityValue(cmd *cobra.Command) *int {
	if cmd.Flags().Changed("priority") {
		return todo.PriorityPtr(jobDoPriority)
	}
	return nil
}

type jobStageReporter struct {
	logger  *jobpkg.ConsoleLogger
	started bool
}

func newJobStageReporter(logger *jobpkg.ConsoleLogger) *jobStageReporter {
	return &jobStageReporter{logger: logger}
}

func (reporter *jobStageReporter) OnStageChange(stage jobpkg.Stage) {
	if reporter.started {
		fmt.Println()
	}
	reporter.started = true
	fmt.Println(jobpkg.StageMessage(stage))
	if reporter.logger != nil {
		reporter.logger.ResetSpacing()
	}
}

const (
	jobLineWidth         = 80
	jobDocumentIndent    = 4
	jobSubdocumentIndent = 8
)

func formatJobField(label, value string) string {
	prefix := fmt.Sprintf("%s: ", label)
	value = internalstrings.NormalizeWhitespace(value)
	if value == "" {
		value = "-"
	}

	wrapWidth := jobLineWidth - jobDocumentIndent - len(prefix)
	if wrapWidth < 1 {
		wrapWidth = 1
	}
	wrapped := wordwrap.String(value, wrapWidth)
	lines := strings.Split(wrapped, "\n")
	for i, line := range lines {
		if i == 0 {
			lines[i] = prefix + line
			continue
		}
		lines[i] = strings.Repeat(" ", len(prefix)) + line
	}
	return jobpkg.IndentBlock(strings.Join(lines, "\n"), jobDocumentIndent)
}

func reflowJobText(value string, width int) string {
	return renderMarkdownOrDash(value, width)
}
