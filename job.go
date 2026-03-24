package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"monks.co/pkg/jj"
	"monks.co/ii/internal/paths"

	"monks.co/ii/internal/listflags"
	internalstrings "monks.co/ii/internal/strings"
	"monks.co/ii/internal/ui"
	jobpkg "monks.co/ii/job"
	"monks.co/ii/todo"
	"github.com/spf13/cobra"
)

var jobCmd = &cobra.Command{
	Use:   "job",
	Short: "Manage jobs",
}

var jobShowCmd = &cobra.Command{
	Use:   "show <job-id>",
	Short: "Show job details",
	Args:  cobra.ExactArgs(1),
	RunE:  runJobShow,
}

var jobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List jobs",
	RunE:  runJobList,
}

var jobResumeCmd = &cobra.Command{
	Use:   "resume <job-id>",
	Short: "Resume a job",
	Args:  cobra.ExactArgs(1),
	RunE:  runJobResume,
}


var (
	jobOpen   = jobpkg.Open
	jobResume = jobpkg.Resume
)

var (
	jobListJSON   bool
	jobListStatus string
	jobListAll    bool
)

func init() {
	rootCmd.AddCommand(jobCmd)
	jobCmd.AddCommand(jobShowCmd, jobListCmd, jobResumeCmd)

	jobListCmd.Flags().BoolVar(&jobListJSON, "json", false, "Output as JSON")
	jobListCmd.Flags().StringVar(&jobListStatus, "status", "", "Filter by status")
	listflags.AddAllFlag(jobListCmd, &jobListAll)
}

func runJobShow(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	manager, err := jobOpen(repoPath, jobpkg.OpenOptions{})
	if err != nil {
		return err
	}

	item, err := manager.Find(args[0])
	if err != nil {
		return err
	}

	jobPrefixLengths, err := jobShowPrefixLengths(manager)
	if err != nil {
		return err
	}

	todoTitle, todoPrefixLengths, err := jobShowTodoInfo(repoPath, item.TodoID, todoStorePurpose(cmd, args))
	if err != nil {
		return err
	}

	jobHighlight := logHighlighter(jobPrefixLengths, ui.HighlightID)
	todoHighlight := logHighlighter(todoPrefixLengths, ui.HighlightID)
	printJobDetail(item, todoTitle, jobHighlight, todoHighlight)
	return nil
}

func runJobList(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	manager, err := jobOpen(repoPath, jobpkg.OpenOptions{})
	if err != nil {
		return err
	}

	// Mark stale jobs as failed before listing
	if _, err := manager.MarkStaleJobsFailed(time.Now()); err != nil {
		return err
	}

	filter := jobpkg.ListFilter{IncludeAll: jobListAll}
	if jobListStatus != "" {
		status := jobpkg.Status(jobListStatus)
		filter.Status = &status
	}

	jobs, err := manager.List(filter)
	if err != nil {
		return err
	}

	if jobListJSON {
		return encodeJSONToStdout(jobs)
	}

	allJobs := jobs
	if jobListStatus != "" || !jobListAll {
		allJobs, err = manager.List(jobpkg.ListFilter{IncludeAll: true})
		if err != nil {
			return err
		}
	}

	if len(jobs) == 0 {
		fmt.Println(jobEmptyListMessage(len(allJobs), jobListStatus, jobListAll))
		return nil
	}

	jobPrefixLengths := jobIDPrefixLengths(allJobs)
	if len(jobPrefixLengths) == 0 {
		jobPrefixLengths = nil
	}

	todoPrefixLengths, todoTitles, err := jobTodoTableInfo(repoPath, todoStorePurpose(cmd, args))
	if err != nil {
		return err
	}

	fmt.Print(formatJobTable(TableFormatOptions{
		Jobs:              jobs,
		Highlight:         ui.HighlightID,
		Now:               time.Now(),
		TodoPrefixLengths: todoPrefixLengths,
		TodoTitles:        todoTitles,
		JobPrefixLengths:  jobPrefixLengths,
	}))
	return nil
}


func runJobResume(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}
	workspacePath, err := resolveWorkspaceRoot()
	if err != nil {
		return err
	}

	runner, err := makeAgentRunnerFunc(repoPath)
	if err != nil {
		return err
	}
	runLLM, err := makeRunLLMFunc(repoPath, runner)
	if err != nil {
		return err
	}

	logger := jobpkg.NewConsoleLogger(os.Stdout)
	reporter := newJobStageReporter(logger)
	onStageChange := reporter.OnStageChange
	onStart := func(info jobpkg.StartInfo) {
		printJobStart(info)
	}

	result, err := jobResume(repoPath, args[0], jobpkg.RunOptions{
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

	if !internalstrings.IsBlank(result.CommitMessage) {
		fmt.Printf("\n%s\n", formatCommitMessageOutput(result.CommitMessage))
	}
	return nil
}

func resolveWorkspaceRoot() (string, error) {
	cwd, err := paths.WorkingDir()
	if err != nil {
		return "", err
	}
	client := jj.New()
	root, err := client.WorkspaceRoot(cwd)
	if err != nil {
		return "", formatRepoRootError(err)
	}
	return root, nil
}

func jobIDPrefixLengths(jobs []jobpkg.Job) map[string]int {
	ids := make([]string, 0, len(jobs))
	for _, item := range jobs {
		ids = append(ids, item.ID)
	}
	return ui.UniqueIDPrefixLengths(ids)
}

func jobTodoTableInfo(repoPath string, purpose string) (map[string]int, map[string]string, error) {
	store, err := openTodoStoreForJob(repoPath, purpose)
	if err != nil {
		return nil, nil, err
	}
	if store == nil {
		return nil, nil, nil
	}
	defer store.Release()

	todos, err := store.List(todo.ListFilter{IncludeTombstones: true})
	if err != nil {
		return nil, nil, err
	}
	if len(todos) == 0 {
		return nil, nil, nil
	}

	index := todo.NewIDIndex(todos)
	prefixLengths := index.PrefixLengths()
	titles := make(map[string]string, len(todos))
	for _, item := range todos {
		titles[internalstrings.NormalizeLower(item.ID)] = item.Title
	}
	return prefixLengths, titles, nil
}

type TableFormatOptions struct {
	Jobs              []jobpkg.Job
	Highlight         func(string, int) string
	Now               time.Time
	TodoPrefixLengths map[string]int
	TodoTitles        map[string]string
	JobPrefixLengths  map[string]int
}

func formatJobTable(opts TableFormatOptions) string {
	jobs := opts.Jobs
	highlight := opts.Highlight
	now := opts.Now
	todoPrefixLengths := opts.TodoPrefixLengths
	jobPrefixLengths := opts.JobPrefixLengths
	builder := ui.NewTableBuilder([]string{"JOB", "TODO", "STAGE", "STATUS", "IMPL", "REVIEW", "PROJECT", "AGE", "DURATION", "TITLE"}, len(jobs))

	jobIDs := make([]string, 0, len(jobs))
	todoIDs := make([]string, 0, len(jobs))
	for _, item := range jobs {
		jobIDs = append(jobIDs, item.ID)
		todoIDs = append(todoIDs, item.TodoID)
	}

	if jobPrefixLengths == nil {
		jobPrefixLengths = ui.UniqueIDPrefixLengths(jobIDs)
	}
	todoFallback := ui.UniqueIDPrefixLengths(todoIDs)
	if todoPrefixLengths == nil {
		todoPrefixLengths = todoFallback
	} else {
		for _, id := range todoIDs {
			if ui.PrefixLength(todoPrefixLengths, id) == 0 {
				todoPrefixLengths = todoFallback
				break
			}
		}
	}

	for _, item := range jobs {
		jobPrefixLen := ui.PrefixLength(jobPrefixLengths, item.ID)
		jobID := highlight(item.ID, jobPrefixLen)
		todoPrefixLen := 0
		todoPrefixLen = ui.PrefixLength(todoPrefixLengths, item.TodoID)
		todoID := highlight(item.TodoID, todoPrefixLen)
		implementationModel := formatJobModelCell(item.ImplementationModel)
		codeReviewModel := formatJobModelCell(item.CodeReviewModel)
		projectReviewModel := formatJobModelCell(item.ProjectReviewModel)
		age := formatJobAge(item, now)
		duration := formatJobDuration(item, now)
		title := ""
		if opts.TodoTitles != nil {
			if value, ok := opts.TodoTitles[internalstrings.NormalizeLower(item.TodoID)]; ok {
				title = ui.TruncateTableCell(value)
			}
		}

		row := []string{
			jobID,
			todoID,
			string(item.Stage),
			string(item.Status),
			implementationModel,
			codeReviewModel,
			projectReviewModel,
			age,
			duration,
			title,
		}
		builder.AddRow(row)
	}

	return builder.String()
}

func formatJobAge(item jobpkg.Job, now time.Time) string {
	return formatOptionalDuration(jobpkg.AgeData(item, now))
}

func formatJobDuration(item jobpkg.Job, now time.Time) string {
	return formatOptionalDuration(jobpkg.DurationData(item, now))
}

func formatJobModelCell(value string) string {
	value = internalstrings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return ui.TruncateTableCell(value)
}

func printJobDetail(item jobpkg.Job, todoTitle string, highlightJob func(string) string, highlightTodo func(string) string) {
	todoLine := highlightTodo(item.TodoID)
	if todoTitle != "" {
		todoLine = fmt.Sprintf("%s - %s", todoLine, todoTitle)
	}

	fmt.Printf("ID:      %s\n", highlightJob(item.ID))
	fmt.Printf("Todo:    %s\n", todoLine)
	fmt.Printf("Stage:   %s\n", item.Stage)
	fmt.Printf("Status:  %s\n", item.Status)

	if len(item.AgentSessions) > 0 {
		fmt.Printf("\nLLM Sessions:\n")
		for _, session := range item.AgentSessions {
			fmt.Printf("- %s: %s\n", session.Purpose, session.ID)
		}
	}

	if item.Feedback != "" {
		fmt.Printf("\nFeedback:\n%s\n", item.Feedback)
	}
}

func jobShowPrefixLengths(manager *jobpkg.Manager) (map[string]int, error) {
	allJobs, err := manager.List(jobpkg.ListFilter{IncludeAll: true})
	if err != nil {
		return nil, err
	}
	if len(allJobs) == 0 {
		return nil, nil
	}
	return jobIDPrefixLengths(allJobs), nil
}

func jobShowTodoInfo(repoPath string, todoID string, purpose string) (string, map[string]int, error) {
	store, err := openTodoStoreForJob(repoPath, purpose)
	if err != nil {
		return "", nil, err
	}
	if store == nil {
		return "", nil, nil
	}
	defer store.Release()

	index, err := store.IDIndex()
	if err != nil {
		return "", nil, err
	}
	prefixLengths := index.PrefixLengths()

	todos, err := store.Show([]string{todoID})
	if err != nil {
		if errors.Is(err, todo.ErrTodoNotFound) {
			return "", prefixLengths, nil
		}
		return "", nil, err
	}
	if len(todos) == 0 {
		return "", prefixLengths, nil
	}

	return todos[0].Title, prefixLengths, nil
}

func openTodoStoreForJob(repoPath string, purpose string) (*todo.Store, error) {
	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false, Purpose: purpose})
	if err != nil {
		if errors.Is(err, todo.ErrNoTodoStore) {
			return nil, nil
		}
		return nil, err
	}
	return store, nil
}
