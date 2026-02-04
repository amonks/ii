package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/amonks/incrementum/agents"
	"github.com/amonks/incrementum/habit"
	"github.com/amonks/incrementum/internal/editor"
	internalstrings "github.com/amonks/incrementum/internal/strings"
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

type jobAgentKind string

const (
	jobAgentInternal jobAgentKind = "internal"
	jobAgentClaude   jobAgentKind = "claude"
	jobAgentCodex    jobAgentKind = "codex"
)

var (
	jobDoTitle               string
	jobDoType                string
	jobDoPriority            int
	jobDoDescription         string
	jobDoImplementationModel string
	jobDoCodeReviewModel     string
	jobDoProjectReviewModel  string
	jobDoDeps                []string
	jobDoEdit                bool
	jobDoNoEdit              bool
	jobDoAgent               string
	jobDoHabit               string
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
	jobDoCmd.Flags().StringVar(&jobDoAgent, "agent", "", "Agent backend (internal, claude, codex)")
	jobDoCmd.Flags().StringVar(&jobDoHabit, "habit", "", "Run a habit instead of a todo (use habit name or empty for first)")
	// Allow --habit without a value to run the first habit alphabetically
	jobDoCmd.Flags().Lookup("habit").NoOptDefVal = " "
}

func parseJobDoAgentKind(cmd *cobra.Command) (jobAgentKind, error) {
	value := internalstrings.NormalizeLowerTrimSpace(jobDoAgent)
	if value == "" {
		return jobAgentInternal, nil
	}

	switch jobAgentKind(value) {
	case jobAgentInternal, jobAgentClaude, jobAgentCodex:
		return jobAgentKind(value), nil
	default:
		return "", fmt.Errorf("unknown agent %q (use internal, claude, or codex)", value)
	}
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

	for _, todoID := range todoIDs {
		if err := jobDoTodo(cmd, todoID); err != nil {
			return err
		}
	}

	return nil
}

// runHabitJob runs a habit job using the --habit flag value.
func runHabitJob(cmd *cobra.Command) error {
	repoPath, err := getRepoPath()
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

	agentKind, err := parseJobDoAgentKind(cmd)
	if err != nil {
		return err
	}

	runner, err := makeAgentRunner(repoPath, agentKind)
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
	repoPath  string
	prompt    string
	model     string
	agentKind jobAgentKind
}

// interactiveSessionResult contains the result of an interactive session.
type interactiveSessionResult struct {
	exitCode int
}

// defaultRunInteractiveSession runs an interactive agent session.
func defaultRunInteractiveSession(opts interactiveSessionOptions) (interactiveSessionResult, error) {
	runner, err := makeAgentRunner(opts.repoPath, opts.agentKind)
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

	handle, err := runner.Run(ctx, agents.RunOptions{
		RepoPath:  opts.repoPath,
		WorkDir:   opts.repoPath,
		Prompt:    opts.prompt,
		Model:     opts.model,
		StartedAt: time.Now(),
		Version:   buildCommitID,
	})
	if err != nil {
		return interactiveSessionResult{}, err
	}

	// Stream events to stderr (same as agent run command)
	streamAgentEventsToStderr(handle.Events())

	result, err := handle.Wait()
	if err != nil {
		return interactiveSessionResult{}, err
	}

	return interactiveSessionResult{exitCode: result.ExitCode}, nil
}

// runDesignTodo runs an interactive agent session for design todos.
func runDesignTodo(cmd *cobra.Command, repoPath string, item todo.Todo) error {
	agentKind, err := parseJobDoAgentKind(cmd)
	if err != nil {
		return err
	}

	// Mark the todo as started
	store, err := todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		Purpose:         fmt.Sprintf("design todo %s start", item.ID),
	})
	if err != nil {
		return err
	}
	if _, err := store.Start([]string{item.ID}); err != nil {
		releaseErr := store.Release()
		return errors.Join(err, releaseErr)
	}
	if err := store.Release(); err != nil {
		return err
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
		repoPath:  repoPath,
		prompt:    prompt,
		model:     item.ImplementationModel,
		agentKind: agentKind,
	})
	if err != nil {
		return err
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
		return exitError{code: result.exitCode}
	}
	return nil
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
	agentKind, err := parseJobDoAgentKind(cmd)
	if err != nil {
		return err
	}

	runner, err := makeAgentRunner(repoPath, agentKind)
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

	if len(result.CommitLog) > 0 {
		fmt.Printf("\n%s\n", formatCommitMessagesOutput(result.CommitLog))
	} else if !internalstrings.IsBlank(result.CommitMessage) {
		fmt.Printf("\n%s\n", formatCommitMessageOutput(result.CommitMessage))
	}
	return nil
}

func formatCommitMessagesOutput(entries []jobpkg.CommitLogEntry) string {
	var out strings.Builder
	out.WriteString("Commit messages:\n")
	for _, entry := range entries {
		out.WriteString("\n")
		label := "Commit"
		if !internalstrings.IsBlank(entry.ID) {
			label = fmt.Sprintf("Commit %s", entry.ID)
		}
		out.WriteString(jobpkg.IndentBlock(label+":", jobDocumentIndent))
		out.WriteString("\n")
		out.WriteString(formatCommitMessageBody(entry.Message, jobSubdocumentIndent))
		out.WriteString("\n")
	}
	return internalstrings.TrimTrailingNewlines(out.String())
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

		parsed, err := editor.EditTodoWithData(data)
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
