package job

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/amonks/incrementum/agent"
	"github.com/amonks/incrementum/internal/config"
	internalagent "github.com/amonks/incrementum/internal/agent"
	"github.com/amonks/incrementum/internal/jj"
	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/todo"
)

const (
	feedbackFilename      = ".incrementum-feedback"
	commitMessageFilename = ".incrementum-commit-message"
)

var promptMessagePattern = regexp.MustCompile(`\{\{[^}]*\.(Message|CommitMessageBlock)[^}]*\}\}`)

// RunOptions configures job execution.
type RunOptions struct {
	OnStart       func(StartInfo)
	OnStageChange func(Stage)
	// EventStream receives job events as they are recorded. The channel is closed
	// when Run completes.
	EventStream chan<- Event
	// WorkspacePath is the path to run the job from.
	// Defaults to repoPath when empty.
	WorkspacePath string
	// Interrupts delivers signals that should interrupt the job.
	// If nil, os.Interrupt is used.
	Interrupts <-chan os.Signal
	Now        func() time.Time
	LoadConfig func(string) (*config.Config, error)
	// Config provides loaded configuration for the job run.
	// When nil, LoadConfig is used.
	Config   *config.Config
	RunTests func(string, []string) ([]TestCommandResult, error)
	// RunLLM runs an LLM session using an agent backend.
	RunLLM func(AgentRunOptions) (AgentRunResult, error)
	// Model overrides model selection for all stages when set.
	Model              string
	CurrentCommitID    func(string) (string, error)
	CurrentChangeID    func(string) (string, error)
	// ChangeIDAt returns the change ID at the given revision.
	ChangeIDAt func(string, string) (string, error)
	// ChangeIDsForRevset returns change IDs matching the revset expression.
	ChangeIDsForRevset func(string, string) ([]string, error)
	CurrentChangeEmpty func(string) (bool, error)
	DiffStat           func(string, string, string) (string, error)
	CommitIDAt         func(string, string) (string, error)
	Commit             func(string, string) error
	RestoreWorkspace   func(string, string) error
	UpdateStale        func(string) error
	Snapshot           func(string) error
	// SeriesLog retrieves the log of commits from fork_point(@|main) to @-.
	SeriesLog func(string) (string, error)
	// Transcripts retrieves transcripts for LLM sessions.
	Transcripts     func(string, []AgentSession) ([]AgentTranscript, error)
	EventLog        *EventLog
	EventLogOptions EventLogOptions
	Logger          Logger
}

// RunResult captures the output of running a job.
type RunResult struct {
	Job           Job
	CommitMessage string
}

type reviewScope int

const (
	reviewScopeStep reviewScope = iota
	reviewScopeProject
)

type ImplementingStageResult struct {
	Job           Job
	CommitMessage string
	Changed       bool
}

type ReviewingStageResult struct {
	Job            Job
	ReviewComments string
}

// Resume continues an interrupted job.
func Resume(repoPath, jobID string, opts RunOptions) (*RunResult, error) {
	if internalstrings.IsBlank(jobID) {
		return nil, fmt.Errorf("job id is required")
	}

	opts = normalizeRunOptions(opts)
	if opts.EventStream != nil {
		defer close(opts.EventStream)
	}
	result := &RunResult{}
	repoPath = filepath.Clean(repoPath)
	if abs, absErr := filepath.Abs(repoPath); absErr == nil {
		repoPath = abs
	}
	workspacePath := repoPath
	if !internalstrings.IsBlank(opts.WorkspacePath) {
		workspacePath = opts.WorkspacePath
	}
	workspacePath = filepath.Clean(workspacePath)
	workspaceAbs := workspacePath
	if abs, absErr := filepath.Abs(workspacePath); absErr == nil {
		workspaceAbs = abs
	}
	workspacePath = workspaceAbs

	manager, err := Open(repoPath, OpenOptions{})
	if err != nil {
		return result, err
	}
	if opts.Config == nil {
		cfg, err := opts.LoadConfig(repoPath)
		if err != nil {
			return result, fmt.Errorf("load config: %w", err)
		}
		if cfg == nil {
			cfg = &config.Config{}
		}
		opts.Config = cfg
	}
	current, err := manager.Find(jobID)
	if err != nil {
		return result, err
	}
	item, err := reloadTodo(repoPath, current.TodoID)
	if err != nil {
		return result, err
	}
	if current.Status == StatusActive {
		if !IsJobStale(current, opts.Now()) {
			return result, fmt.Errorf("job %s is active and may still be running", current.ID)
		}
	} else if current.Status != StatusFailed {
		return result, fmt.Errorf("job %s is not resumable (status %s)", current.ID, current.Status)
	}
	if err := checkWorkspaceForResume(workspacePath, current.Changes, opts); err != nil {
		return result, err
	}
	if err := startTodo(repoPath, item.ID); err != nil {
		return result, err
	}

	status := StatusActive
	stage := StageImplementing
	updated, err := manager.Update(current.ID, UpdateOptions{Status: &status, Stage: &stage}, opts.Now())
	if err != nil {
		reopenErr := reopenTodo(repoPath, item.ID)
		return result, errors.Join(err, reopenErr)
	}
	result.Job = updated

	if opts.OnStart != nil {
		opts.OnStart(StartInfo{JobID: updated.ID, Workdir: workspaceAbs, Todo: item})
	}

	createdEventLog := false
	if opts.EventLog == nil {
		eventLog, err := OpenEventLogAppend(updated.ID, opts.EventLogOptions)
		if err != nil {
			status := StatusFailed
			updated, updateErr := manager.Update(updated.ID, UpdateOptions{Status: &status}, opts.Now())
			result.Job = updated
			finalizeErr := finalizeTodo(repoPath, item.ID, StatusFailed)
			return result, errors.Join(err, updateErr, finalizeErr)
		}
		opts.EventLog = eventLog
		createdEventLog = true
	}
	if createdEventLog {
		defer func() {
			_ = opts.EventLog.Close()
		}()
	}
	if opts.EventStream != nil {
		opts.EventLog.SetStream(opts.EventStream)
	}
	if err := appendJobEvent(opts.EventLog, jobEventStage, stageEventData{Stage: stage}); err != nil {
		status := StatusFailed
		updated, updateErr := manager.Update(updated.ID, UpdateOptions{Status: &status}, opts.Now())
		result.Job = updated
		finalizeErr := finalizeTodo(repoPath, item.ID, StatusFailed)
		return result, errors.Join(err, updateErr, finalizeErr)
	}
	if opts.OnStageChange != nil {
		opts.OnStageChange(stage)
	}

	interrupts := opts.Interrupts
	if interrupts == nil {
		localInterrupts := make(chan os.Signal, 1)
		signal.Notify(localInterrupts, os.Interrupt)
		defer signal.Stop(localInterrupts)
		interrupts = localInterrupts
	}

	runCtx := runContext{
		repoPath:      repoPath,
		workspacePath: workspacePath,
		item:          item,
		opts:          opts,
		manager:       manager,
		result:        result,
	}
	finalJob, err := runJobStages(&runCtx, updated, interrupts)
	result.Job = finalJob
	statusErr := finalizeTodo(repoPath, item.ID, finalJob.Status)
	if err != nil {
		return result, errors.Join(err, statusErr)
	}
	if statusErr != nil {
		return result, statusErr
	}
	return result, nil
}

// Run creates and executes a job for the given todo.
func Run(repoPath, todoID string, opts RunOptions) (*RunResult, error) {
	return runJob(repoPath, todoID, opts)
}

func runJob(repoPath, todoID string, opts RunOptions) (*RunResult, error) {
	if internalstrings.IsBlank(todoID) {
		return nil, fmt.Errorf("todo id is required")
	}

	opts = normalizeRunOptions(opts)
	if opts.EventStream != nil {
		defer close(opts.EventStream)
	}
	result := &RunResult{}
	repoPath = filepath.Clean(repoPath)
	if abs, absErr := filepath.Abs(repoPath); absErr == nil {
		repoPath = abs
	}
	workspacePath := repoPath
	if !internalstrings.IsBlank(opts.WorkspacePath) {
		workspacePath = opts.WorkspacePath
	}
	workspacePath = filepath.Clean(workspacePath)
	workspaceAbs := workspacePath
	if abs, absErr := filepath.Abs(workspacePath); absErr == nil {
		workspaceAbs = abs
	}
	workspacePath = workspaceAbs

	manager, err := Open(repoPath, OpenOptions{})
	if err != nil {
		return result, err
	}
	if opts.Config == nil {
		cfg, err := opts.LoadConfig(repoPath)
		if err != nil {
			return result, fmt.Errorf("load config: %w", err)
		}
		if cfg == nil {
			cfg = &config.Config{}
		}
		opts.Config = cfg
	}
	item, err := reloadTodo(repoPath, todoID)
	if err != nil {
		return result, err
	}
	if err := startTodo(repoPath, item.ID); err != nil {
		return result, err
	}

	model := resolveModelForPurpose(opts.Config, opts.Model, "implement", item)
	implementationModel := resolveModelForPurpose(opts.Config, opts.Model, "implement", item)
	codeReviewModel := resolveModelForPurpose(opts.Config, opts.Model, "review", item)
	projectReviewModel := resolveModelForPurpose(opts.Config, opts.Model, "project-review", item)
	created, err := manager.Create(item.ID, opts.Now(), CreateOptions{
		Agent:               model,
		ImplementationModel: implementationModel,
		CodeReviewModel:     codeReviewModel,
		ProjectReviewModel:  projectReviewModel,
	})
	if err != nil {
		reopenErr := reopenTodo(repoPath, item.ID)
		return result, errors.Join(err, reopenErr)
	}
	result.Job = created

	if opts.OnStart != nil {
		opts.OnStart(StartInfo{JobID: created.ID, Workdir: workspaceAbs, Todo: item})
	}

	createdEventLog := false
	if opts.EventLog == nil {
		eventLog, err := OpenEventLog(created.ID, opts.EventLogOptions)
		if err != nil {
			status := StatusFailed
			updated, updateErr := manager.Update(created.ID, UpdateOptions{Status: &status}, opts.Now())
			result.Job = updated
			finalizeErr := finalizeTodo(repoPath, item.ID, StatusFailed)
			return result, errors.Join(err, updateErr, finalizeErr)
		}
		opts.EventLog = eventLog
		createdEventLog = true
	}
	if createdEventLog {
		defer func() {
			_ = opts.EventLog.Close()
		}()
	}
	if opts.EventStream != nil {
		opts.EventLog.SetStream(opts.EventStream)
	}
	if err := appendJobEvent(opts.EventLog, jobEventStage, stageEventData{Stage: created.Stage}); err != nil {
		status := StatusFailed
		updated, updateErr := manager.Update(created.ID, UpdateOptions{Status: &status}, opts.Now())
		result.Job = updated
		finalizeErr := finalizeTodo(repoPath, item.ID, StatusFailed)
		return result, errors.Join(err, updateErr, finalizeErr)
	}
	if opts.OnStageChange != nil {
		opts.OnStageChange(created.Stage)
	}

	interrupts := opts.Interrupts
	if interrupts == nil {
		localInterrupts := make(chan os.Signal, 1)
		signal.Notify(localInterrupts, os.Interrupt)
		defer signal.Stop(localInterrupts)
		interrupts = localInterrupts
	}

	runCtx := runContext{
		repoPath:      repoPath,
		workspacePath: workspacePath,
		item:          item,
		opts:          opts,
		manager:       manager,
		result:        result,
	}
	finalJob, err := runJobStages(&runCtx, created, interrupts)
	result.Job = finalJob
	statusErr := finalizeTodo(repoPath, item.ID, finalJob.Status)
	if err != nil {
		return result, errors.Join(err, statusErr)
	}
	if statusErr != nil {
		return result, statusErr
	}
	return result, nil
}

type runContext struct {
	repoPath       string
	workspacePath  string
	item           todo.Todo
	opts           RunOptions
	manager        *Manager
	result         *RunResult
	reviewScope    reviewScope
	commitMessage  string
	reviewComments string
	workComplete   bool
}

func runJobStages(ctx *runContext, current Job, interrupts <-chan os.Signal) (Job, error) {
	ctx.reviewScope = reviewScopeStep
	for current.Status == StatusActive {
		if current.Stage != StageImplementing {
			return current, fmt.Errorf("invalid job stage: %s", current.Stage)
		}

		next, stageErr := ctx.runStageWithInterrupt(current, ctx.runImplementingStage(current), interrupts)
		if stageErr != nil && errors.Is(stageErr, ErrJobInterrupted) {
			return next, stageErr
		}
		current, stageErr = ctx.handleStageOutcome(current, next, stageErr)
		if stageErr != nil {
			return current, stageErr
		}
		if current.Status != StatusActive {
			break
		}
		if current.Stage == StageImplementing {
			// Implementing stage requested retry (e.g., missing commit message).
			ctx.reviewScope = reviewScopeStep
			continue
		}
		if ctx.workComplete {
			ctx.reviewScope = reviewScopeProject
		}

		if current.Stage == StageTesting {
			next, stageErr = ctx.runStageWithInterrupt(current, ctx.runTestingStage(current), interrupts)
			if stageErr != nil && errors.Is(stageErr, ErrJobInterrupted) {
				return next, stageErr
			}
			current, stageErr = ctx.handleStageOutcome(current, next, stageErr)
			if stageErr != nil {
				return current, stageErr
			}
			if current.Status != StatusActive {
				break
			}
			if current.Stage == StageImplementing {
				ctx.reviewScope = reviewScopeStep
				continue
			}
		}

		next, stageErr = ctx.runStageWithInterrupt(current, ctx.runReviewingStage(current), interrupts)
		if stageErr != nil && errors.Is(stageErr, ErrJobInterrupted) {
			return next, stageErr
		}
		current, stageErr = ctx.handleStageOutcome(current, next, stageErr)
		if stageErr != nil {
			return current, stageErr
		}
		if current.Status != StatusActive {
			break
		}
		if current.Stage == StageImplementing {
			ctx.reviewScope = reviewScopeStep
			continue
		}
		if ctx.reviewScope == reviewScopeProject {
			continue
		}

		next, stageErr = ctx.runStageWithInterrupt(current, ctx.runCommittingStage(current), interrupts)
		if stageErr != nil && errors.Is(stageErr, ErrJobInterrupted) {
			return next, stageErr
		}
		current, stageErr = ctx.handleStageOutcome(current, next, stageErr)
		if stageErr != nil {
			return current, stageErr
		}
	}

	return current, nil
}

func (ctx *runContext) runStageWithInterrupt(current Job, stageFn func() (Job, error), interrupts <-chan os.Signal) (Job, error) {
	stageResult := make(chan struct {
		job Job
		err error
	}, 1)
	go func() {
		job, err := stageFn()
		stageResult <- struct {
			job Job
			err error
		}{job: job, err: err}
	}()

	select {
	case <-interrupts:
		return ctx.handleInterrupt(current)
	case res := <-stageResult:
		return res.job, res.err
	}
}

func (ctx *runContext) handleInterrupt(current Job) (Job, error) {
	status := StatusFailed
	updated, updateErr := ctx.manager.Update(current.ID, UpdateOptions{Status: &status}, ctx.opts.Now())
	if updateErr != nil {
		fallback := current
		fallback.Status = status
		now := ctx.opts.Now()
		fallback.UpdatedAt = now
		fallback.CompletedAt = now
		return fallback, errors.Join(ErrJobInterrupted, updateErr)
	}
	return updated, errors.Join(ErrJobInterrupted, updateErr)
}

func (ctx *runContext) handleStageOutcome(current, next Job, stageErr error) (Job, error) {
	if stageErr != nil {
		if next.Status == StatusAbandoned {
			ctx.result.Job = next
			return next, stageErr
		}
		status := StatusFailed
		updated, updateErr := ctx.manager.Update(current.ID, UpdateOptions{Status: &status}, ctx.opts.Now())
		ctx.result.Job = updated
		return updated, errors.Join(stageErr, updateErr)
	}
	if next.ID != "" {
		if next.Stage != current.Stage {
			if err := appendJobEvent(ctx.opts.EventLog, jobEventStage, stageEventData{Stage: next.Stage}); err != nil {
				status := StatusFailed
				updated, updateErr := ctx.manager.Update(next.ID, UpdateOptions{Status: &status}, ctx.opts.Now())
				ctx.result.Job = updated
				return updated, errors.Join(err, updateErr)
			}
			if ctx.opts.OnStageChange != nil {
				ctx.opts.OnStageChange(next.Stage)
			}
		}
		current = next
		ctx.result.Job = next
	}
	return current, nil
}

func (ctx *runContext) runImplementingStage(current Job) func() (Job, error) {
	return func() (Job, error) {
		// Re-read the todo at the start of each implementation run so that
		// edits made from another process are picked up.
		item, err := reloadTodo(ctx.repoPath, ctx.item.ID)
		if err != nil {
			return Job{}, fmt.Errorf("reload todo: %w", err)
		}
		ctx.item = item

		result, err := runImplementingStage(ctx.manager, current, ctx.item, ctx.repoPath, ctx.workspacePath, ctx.opts, ctx.commitMessage)
		if err != nil {
			return Job{}, err
		}
		ctx.commitMessage = result.CommitMessage
		ctx.workComplete = !result.Changed
		return result.Job, nil
	}
}

func (ctx *runContext) runTestingStage(current Job) func() (Job, error) {
	return func() (Job, error) {
		return runTestingStage(ctx.manager, current, ctx.repoPath, ctx.workspacePath, ctx.opts)
	}
}

func (ctx *runContext) runReviewingStage(current Job) func() (Job, error) {
	return func() (Job, error) {
		result, err := runReviewingStage(ctx.manager, current, ctx.item, ctx.repoPath, ctx.workspacePath, ctx.opts, ctx.commitMessage, ctx.reviewScope)
		if err != nil {
			return result.Job, err
		}
		ctx.reviewComments = result.ReviewComments
		return result.Job, nil
	}
}

func (ctx *runContext) runCommittingStage(current Job) func() (Job, error) {
	return func() (Job, error) {
		return runCommittingStage(CommittingStageOptions{
			Manager:        ctx.manager,
			Current:        current,
			Item:           ctx.item,
			RepoPath:       ctx.repoPath,
			WorkspacePath:  ctx.workspacePath,
			RunOptions:     ctx.opts,
			Result:         ctx.result,
			CommitMessage:  ctx.commitMessage,
			ReviewComments: ctx.reviewComments,
		})
	}
}

func normalizeRunOptions(opts RunOptions) RunOptions {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.LoadConfig == nil {
		opts.LoadConfig = config.Load
	}
	if opts.RunTests == nil {
		opts.RunTests = RunTestCommands
	}
	if opts.RunLLM == nil {
		opts.RunLLM = defaultRunLLM
	}
	var jjClient *jj.Client
	getJJ := func() *jj.Client {
		if jjClient == nil {
			jjClient = jj.New()
		}
		return jjClient
	}
	if opts.CurrentCommitID == nil {
		opts.CurrentCommitID = getJJ().CurrentCommitID
	}
	if opts.CurrentChangeID == nil {
		opts.CurrentChangeID = getJJ().CurrentChangeID
	}
	if opts.ChangeIDAt == nil {
		opts.ChangeIDAt = getJJ().ChangeIDAt
	}
	if opts.ChangeIDsForRevset == nil {
		opts.ChangeIDsForRevset = getJJ().ChangeIDsForRevset
	}
	if opts.CurrentChangeEmpty == nil {
		opts.CurrentChangeEmpty = getJJ().CurrentChangeEmpty
	}
	if opts.DiffStat == nil {
		opts.DiffStat = getJJ().DiffStat
	}
	if opts.CommitIDAt == nil {
		opts.CommitIDAt = getJJ().CommitIDAt
	}
	if opts.Commit == nil {
		opts.Commit = getJJ().Commit
	}
	if opts.RestoreWorkspace == nil {
		opts.RestoreWorkspace = getJJ().Edit
	}
	if opts.UpdateStale == nil {
		opts.UpdateStale = getJJ().WorkspaceUpdateStale
	}
	if opts.Snapshot == nil {
		opts.Snapshot = getJJ().Snapshot
	}
	if opts.SeriesLog == nil {
		opts.SeriesLog = getJJ().SeriesLog
	}
	if opts.Transcripts == nil {
		opts.Transcripts = defaultTranscripts
	}
	opts.Logger = resolveLogger(opts.Logger)
	return opts
}

func resolveModelForPurpose(cfg *config.Config, override, purpose string, item todo.Todo) string {
	if !internalstrings.IsBlank(override) {
		return internalstrings.TrimSpace(override)
	}
	modelOverride := todoModelForPurpose(item, purpose)
	if !internalstrings.IsBlank(modelOverride) {
		return internalstrings.TrimSpace(modelOverride)
	}
	if cfg == nil {
		return ""
	}
	model := ""
	switch purpose {
	case "implement":
		model = cfg.Job.ImplementationModel
	case "review":
		model = cfg.Job.CodeReviewModel
	case "project-review":
		model = cfg.Job.ProjectReviewModel
	default:
		model = cfg.Job.Model
	}
	if internalstrings.IsBlank(model) {
		model = cfg.Job.Model
	}
	return internalstrings.TrimSpace(model)
}

func todoModelForPurpose(item todo.Todo, purpose string) string {
	switch purpose {
	case "implement":
		return item.ImplementationModel
	case "review":
		return item.CodeReviewModel
	case "project-review":
		return item.ProjectReviewModel
	default:
		return ""
	}
}

func runImplementingStage(manager *Manager, current Job, item todo.Todo, repoPath, workspacePath string, opts RunOptions, previousMessage string) (ImplementingStageResult, error) {
	logger := resolveLogger(opts.Logger)
	updateStaleWorkspace(opts.UpdateStale, workspacePath)
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	if err := removeFileIfExists(feedbackPath); err != nil {
		return ImplementingStageResult{}, err
	}

	beforeCommitID, err := opts.CurrentCommitID(workspacePath)
	if err != nil {
		return ImplementingStageResult{}, err
	}

	// Ensure we have a current change to track commits against.
	// Create a new JobChange if there's no in-progress change.
	updated := current
	if updated.CurrentChange() == nil {
		changeID, err := opts.CurrentChangeID(workspacePath)
		if err != nil {
			return ImplementingStageResult{}, fmt.Errorf("get current change id: %w", err)
		}
		updated, err = manager.AppendChange(updated.ID, JobChange{ChangeID: changeID}, opts.Now())
		if err != nil {
			return ImplementingStageResult{}, fmt.Errorf("append change: %w", err)
		}
	}

	promptName := "prompt-implementation.tmpl"
	if !internalstrings.IsBlank(current.Feedback) {
		promptName = "prompt-feedback.tmpl"
		messagePath := filepath.Join(workspacePath, commitMessageFilename)
		if err := writeCommitMessageSeed(messagePath, previousMessage); err != nil {
			return ImplementingStageResult{}, err
		}
	}

	// Get the series log (commits in this patch series) for context.
	// Best-effort: if this fails, we continue without it but log a warning.
	seriesLog := ""
	if opts.SeriesLog != nil {
		var seriesLogErr error
		seriesLog, seriesLogErr = opts.SeriesLog(workspacePath)
		if seriesLogErr != nil {
			_ = appendJobEvent(opts.EventLog, jobEventWarning, warningEventData{
				Context: "series_log",
				Message: seriesLogErr.Error(),
			})
		}
	}

	model := resolveModelForPurpose(opts.Config, opts.Model, "implement", item)
	var lastSessionID string
	phase, userContent, err := renderPromptParts(item, current.Feedback, previousMessage, seriesLog, nil, promptName, workspacePath)
	if err != nil {
		return ImplementingStageResult{}, err
	}
	runOpts := AgentRunOptions{
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		Prompt:        buildPromptContent(phase, userContent, workspacePath, opts),
		Model:         model,
		StartedAt:     opts.Now(),
		EventLog:      opts.EventLog,
		Env:           agentRunEnv(),
	}
	prompt := renderPromptLog(runOpts.Prompt)
	if err := appendJobEvent(opts.EventLog, jobEventPrompt, promptEventData{Purpose: "implement", Template: promptName, Prompt: prompt}); err != nil {
		return ImplementingStageResult{}, err
	}
	runAttempt := func() (AgentRunResult, error) {
		result, err := runLLMWithEvents(opts, runOpts, "implement")
		if err != nil {
			return AgentRunResult{}, err
		}

		lastSessionID = result.SessionID
		session := AgentSession{Purpose: "implement", ID: result.SessionID}
		updated, err = manager.Update(updated.ID, UpdateOptions{AppendAgentSession: &session}, opts.Now())
		if err != nil {
			return AgentRunResult{}, err
		}
		transcript := loadTranscript(opts, session)
		if !internalstrings.IsBlank(transcript) {
			if err := appendJobEvent(opts.EventLog, jobEventTranscript, transcriptEventData{Purpose: "implement", Transcript: transcript}); err != nil {
				return AgentRunResult{}, err
			}
		}
		logger.Prompt(PromptLog{Purpose: "implement", Template: promptName, Prompt: prompt, Transcript: transcript})
		return result, nil
	}

	llmResult, err := runAttempt()
	if err != nil {
		return ImplementingStageResult{}, err
	}

	retryCount := 0
	eofRetryCount := 0
	for llmResult.ExitCode != 0 {
		if isModelEOFError(llmResult.Error) && eofRetryCount < 2 {
			eofRetryCount++
			llmResult, err = runAttempt()
			if err != nil {
				return ImplementingStageResult{}, err
			}
			continue
		}
		afterCommitID := ""
		var afterCommitErr error
		if opts.CurrentCommitID != nil && !internalstrings.IsBlank(workspacePath) {
			afterCommitID, afterCommitErr = opts.CurrentCommitID(workspacePath)
		}
		restored := false
		var restoreErr error
		if llmResult.ExitCode < 0 && afterCommitErr == nil && afterCommitID != "" && beforeCommitID != "" && afterCommitID != beforeCommitID {
			if opts.RestoreWorkspace != nil {
				restoreErr = opts.RestoreWorkspace(workspacePath, beforeCommitID)
				if restoreErr == nil {
					restored = true
				}
			}
		}
		if restored && retryCount == 0 {
			retryCount++
			llmResult, err = runAttempt()
			if err != nil {
				return ImplementingStageResult{}, err
			}
			continue
		}
		// Context overflow: retry without restoring the workspace. The agent may
		// have made partial progress that we want to keep. We only retry once
		// within this stage invocation.
		if isContextOverflowError(llmResult.Error) && retryCount == 0 {
			retryCount++
			llmResult, err = runAttempt()
			if err != nil {
				return ImplementingStageResult{}, err
			}
			continue
		}
		// Context overflow after retry: stay in implementing stage with feedback
		// instead of failing the job. This allows the agent to continue from where
		// it left off with a fresh context window.
		if isContextOverflowError(llmResult.Error) {
			feedback := "Context overflow: the conversation exceeded the model's context window. " +
				"The working tree has been preserved with any partial progress. " +
				"Please continue your work from where you left off."
			nextStage := StageImplementing
			updated, err = manager.Update(updated.ID, UpdateOptions{Stage: &nextStage, Feedback: &feedback}, opts.Now())
			if err != nil {
				return ImplementingStageResult{}, err
			}
			return ImplementingStageResult{Job: updated, CommitMessage: previousMessage, Changed: true}, nil
		}
		return ImplementingStageResult{}, errors.New(buildLLMFailureMessage("implement", promptName, llmResult, runOpts, beforeCommitID, afterCommitID, afterCommitErr, restored, restoreErr, retryCount))
	}

	afterCommitID, err := opts.CurrentCommitID(workspacePath)
	if err != nil {
		return ImplementingStageResult{}, err
	}

	// Check if the current change has work to commit. We use the empty check
	// rather than comparing commit IDs because a previous job run may have
	// left uncommitted work in @ if it failed after making changes.
	if opts.CurrentChangeEmpty == nil {
		return ImplementingStageResult{}, fmt.Errorf("current change empty check is required")
	}
	empty, err := opts.CurrentChangeEmpty(workspacePath)
	if err != nil {
		return ImplementingStageResult{}, err
	}
	changed := !empty
	message := ""
	if changed {
		messagePath := filepath.Join(workspacePath, commitMessageFilename)
		message, err = readCommitMessage(messagePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Commit message is missing but there are changes. Instead of failing,
				// set feedback and retry. This handles cases where the LLM made changes
				// but forgot to write the commit message file.
				feedback := fmt.Sprintf(
					"You made changes to the workspace but did not write a commit message. "+
						"Please write a commit message describing your changes to %s.",
					commitMessageFilename,
				)
				nextStage := StageImplementing
				updated, err = manager.Update(updated.ID, UpdateOptions{Stage: &nextStage, Feedback: &feedback}, opts.Now())
				if err != nil {
					return ImplementingStageResult{}, err
				}
				return ImplementingStageResult{Job: updated, CommitMessage: "", Changed: true}, nil
			}
			return ImplementingStageResult{}, err
		}
		logger.CommitMessage(CommitMessageLog{Label: "Draft", Message: message})
		if err := appendJobEvent(opts.EventLog, jobEventCommitMessage, commitMessageEventData{Label: "Draft", Message: message}); err != nil {
			return ImplementingStageResult{}, err
		}

		// Record the commit in the current change.
		commit := JobCommit{
			CommitID:       afterCommitID,
			DraftMessage:   message,
			AgentSessionID: lastSessionID,
		}
		updated, err = manager.AppendCommitToCurrentChange(updated.ID, commit, opts.Now())
		if err != nil {
			return ImplementingStageResult{}, fmt.Errorf("append commit to change: %w", err)
		}
	} else {
		messagePath := filepath.Join(workspacePath, commitMessageFilename)
		if err := removeFileIfExists(messagePath); err != nil {
			return ImplementingStageResult{}, err
		}
	}

	nextStage := StageTesting
	if !changed {
		nextStage = StageReviewing
	}
	updated, err = manager.Update(updated.ID, UpdateOptions{Stage: &nextStage}, opts.Now())
	if err != nil {
		return ImplementingStageResult{}, err
	}
	return ImplementingStageResult{Job: updated, CommitMessage: message, Changed: changed}, nil
}

func runTestingStage(manager *Manager, current Job, repoPath, workspacePath string, opts RunOptions) (Job, error) {
	logger := resolveLogger(opts.Logger)
	cfg := opts.Config
	if cfg == nil {
		var err error
		cfg, err = opts.LoadConfig(repoPath)
		if err != nil {
			return Job{}, fmt.Errorf("load config: %w", err)
		}
	}
	if len(cfg.Job.TestCommands) < 1 {
		return Job{}, fmt.Errorf("job test-commands must be configured")
	}

	results, err := opts.RunTests(workspacePath, cfg.Job.TestCommands)
	if err != nil {
		return Job{}, err
	}
	logger.Tests(TestLog{Results: results})
	if err := appendJobEvent(opts.EventLog, jobEventTests, buildTestsEventData(results)); err != nil {
		return Job{}, err
	}

	nextStage, feedback := testingStageOutcome(results)

	// Record test result on the current commit.
	updated := current
	if updated.CurrentCommit() != nil {
		passed := feedback == ""
		updated, err = manager.UpdateCurrentCommit(updated.ID, JobCommitUpdate{TestsPassed: &passed}, opts.Now())
		if err != nil {
			return Job{}, fmt.Errorf("update commit tests passed: %w", err)
		}
	}

	update := UpdateOptions{Stage: &nextStage}
	if feedback != "" {
		update.Feedback = &feedback
	} else {
		empty := ""
		update.Feedback = &empty
	}
	updated, err = manager.Update(updated.ID, update, opts.Now())
	if err != nil {
		return Job{}, err
	}
	return updated, nil
}

func runReviewingStage(manager *Manager, current Job, item todo.Todo, repoPath, workspacePath string, opts RunOptions, commitMessage string, scope reviewScope) (ReviewingStageResult, error) {
	logger := resolveLogger(opts.Logger)
	updateStaleWorkspace(opts.UpdateStale, workspacePath)
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	if err := removeFileIfExists(feedbackPath); err != nil {
		return ReviewingStageResult{}, err
	}

	message, err := resolveReviewCommitMessage(commitMessage, workspacePath, scope == reviewScopeStep)
	if err != nil {
		return ReviewingStageResult{}, err
	}

	promptName := "prompt-commit-review.tmpl"
	purpose := "review"
	if scope == reviewScopeProject {
		promptName = "prompt-project-review.tmpl"
		purpose = "project-review"
	}
	model := resolveModelForPurpose(opts.Config, opts.Model, purpose, item)

	promptTemplate, err := LoadPrompt(workspacePath, promptName)
	if err != nil {
		return ReviewingStageResult{}, err
	}
	promptTemplate = ensureCommitMessageInPrompt(promptTemplate, message)
	context, err := loadPromptContext(workspacePath)
	if err != nil {
		return ReviewingStageResult{}, err
	}
	parts, err := buildPromptParts(item, "", message, "", nil, workspacePath, nil, context, promptTemplate)
	if err != nil {
		return ReviewingStageResult{}, err
	}
	testCommands := []string{}
	if opts.Config != nil {
		testCommands = opts.Config.Job.TestCommands
	} else if opts.LoadConfig != nil {
		if cfg, cfgErr := opts.LoadConfig(repoPath); cfgErr == nil && cfg != nil {
			testCommands = cfg.Job.TestCommands
		}
	}
	promptContent := promptContentFromParts(parts)
	if len(testCommands) > 0 {
		promptContent.TestCommands = testCommands
	}
	runOpts := AgentRunOptions{
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		Prompt:        promptContent,
		Model:         model,
		StartedAt:     opts.Now(),
		EventLog:      opts.EventLog,
		Env:           agentRunEnv(),
	}
	prompt := renderPromptLog(promptContent)
	if err := appendJobEvent(opts.EventLog, jobEventPrompt, promptEventData{Purpose: purpose, Template: promptName, Prompt: prompt}); err != nil {
		return ReviewingStageResult{}, err
	}

	llmResult, err := runLLMWithEvents(opts, runOpts, purpose)
	if err != nil {
		return ReviewingStageResult{}, err
	}

	session := AgentSession{Purpose: purpose, ID: llmResult.SessionID}
	updated, err := manager.Update(current.ID, UpdateOptions{AppendAgentSession: &session}, opts.Now())
	if err != nil {
		return ReviewingStageResult{}, err
	}
	transcript := loadTranscript(opts, session)
	if !internalstrings.IsBlank(transcript) {
		if err := appendJobEvent(opts.EventLog, jobEventTranscript, transcriptEventData{Purpose: purpose, Transcript: transcript}); err != nil {
			return ReviewingStageResult{}, err
		}
	}
	logger.Prompt(PromptLog{Purpose: purpose, Template: promptName, Prompt: prompt, Transcript: transcript})

	if llmResult.ExitCode != 0 {
		return ReviewingStageResult{}, fmt.Errorf("%s", buildReviewFailureMessage(purpose, llmResult, model))
	}

	feedback, err := ReadReviewFeedback(feedbackPath)
	if err != nil {
		return ReviewingStageResult{}, err
	}
	logger.Review(ReviewLog{Purpose: purpose, Feedback: feedback})
	if err := appendJobEvent(opts.EventLog, jobEventReview, reviewEventData{Purpose: purpose, Outcome: feedback.Outcome, Details: feedback.Details}); err != nil {
		return ReviewingStageResult{}, err
	}

	// Record the review in the appropriate place.
	review := JobReview{
		Outcome:        feedback.Outcome,
		Comments:       feedback.Details,
		AgentSessionID: llmResult.SessionID,
	}
	if scope == reviewScopeProject {
		updated, err = manager.SetProjectReview(updated.ID, review, opts.Now())
		if err != nil {
			return ReviewingStageResult{}, fmt.Errorf("set project review: %w", err)
		}
	} else if updated.CurrentCommit() != nil {
		updated, err = manager.UpdateCurrentCommit(updated.ID, JobCommitUpdate{Review: &review}, opts.Now())
		if err != nil {
			return ReviewingStageResult{}, fmt.Errorf("update commit review: %w", err)
		}
	}

	switch feedback.Outcome {
	case ReviewOutcomeAccept:
		if scope == reviewScopeProject {
			status := StatusCompleted
			updated, err = manager.Update(updated.ID, UpdateOptions{Status: &status}, opts.Now())
			if err != nil {
				return ReviewingStageResult{}, err
			}
			return ReviewingStageResult{Job: updated, ReviewComments: feedback.Details}, nil
		}
		nextStage := StageCommitting
		empty := ""
		updated, err = manager.Update(updated.ID, UpdateOptions{Stage: &nextStage, Feedback: &empty}, opts.Now())
		if err != nil {
			return ReviewingStageResult{}, err
		}
		return ReviewingStageResult{Job: updated, ReviewComments: feedback.Details}, nil
	case ReviewOutcomeAbandon:
		status := StatusAbandoned
		updated, err = manager.Update(updated.ID, UpdateOptions{Status: &status}, opts.Now())
		if err != nil {
			return ReviewingStageResult{}, err
		}
		return ReviewingStageResult{Job: updated}, &AbandonedError{Reason: feedback.Details}
	case ReviewOutcomeRequestChanges:
		nextStage := StageImplementing
		updated, err = manager.Update(updated.ID, UpdateOptions{Stage: &nextStage, Feedback: &feedback.Details}, opts.Now())
		if err != nil {
			return ReviewingStageResult{}, err
		}
		return ReviewingStageResult{Job: updated}, nil
	default:
		return ReviewingStageResult{}, ErrInvalidFeedbackFormat
	}
}

type CommittingStageOptions struct {
	Manager        *Manager
	Current        Job
	Item           todo.Todo
	RepoPath       string
	WorkspacePath  string
	RunOptions     RunOptions
	Result         *RunResult
	CommitMessage  string
	ReviewComments string
}

func runCommittingStage(opts CommittingStageOptions) (Job, error) {
	logger := resolveLogger(opts.RunOptions.Logger)
	updateStaleWorkspace(opts.RunOptions.UpdateStale, opts.WorkspacePath)
	if opts.RunOptions.DiffStat == nil {
		return Job{}, fmt.Errorf("diff stat is required")
	}
	diffStat, err := opts.RunOptions.DiffStat(opts.WorkspacePath, "@-", "@")
	if err != nil {
		return Job{}, err
	}
	if !diffStatHasChanges(diffStat) {
		nextStage := StageImplementing
		updated, err := opts.Manager.Update(opts.Current.ID, UpdateOptions{Stage: &nextStage}, opts.RunOptions.Now())
		if err != nil {
			return Job{}, err
		}
		return updated, nil
	}
	message := internalstrings.TrimSpace(opts.CommitMessage)
	if message == "" {
		return Job{}, fmt.Errorf("commit message is required")
	}

	finalMessage := formatCommitMessage(opts.Item, message, opts.ReviewComments)
	logMessage := formatCommitMessageWithWidth(opts.Item, message, opts.ReviewComments, lineWidth-subdocumentIndent)
	opts.Result.CommitMessage = finalMessage
	logger.CommitMessage(CommitMessageLog{Label: "Final", Message: logMessage, Preformatted: true})
	if err := appendJobEvent(opts.RunOptions.EventLog, jobEventCommitMessage, commitMessageEventData{Label: "Final", Message: logMessage, Preformatted: true}); err != nil {
		return Job{}, err
	}

	updateStaleWorkspace(opts.RunOptions.UpdateStale, opts.WorkspacePath)
	if err := opts.RunOptions.Commit(opts.WorkspacePath, finalMessage); err != nil {
		return Job{}, err
	}

	nextStage := StageImplementing
	updated, err := opts.Manager.Update(opts.Current.ID, UpdateOptions{Stage: &nextStage}, opts.RunOptions.Now())
	if err != nil {
		return Job{}, err
	}
	return updated, nil
}

// defaultTranscripts loads transcripts for LLM sessions.
// The repoPath argument is ignored since agent session IDs are globally unique.
func defaultTranscripts(_ string, sessions []AgentSession) ([]AgentTranscript, error) {
	if len(sessions) == 0 {
		return nil, nil
	}

	store, err := agent.Open()
	if err != nil {
		return nil, err
	}

	transcripts := make([]AgentTranscript, 0, len(sessions))
	for _, session := range sessions {
		transcript, err := store.TranscriptSnapshot(session.ID)
		if err != nil {
			// If we can't get a transcript, just use an empty one
			transcript = "-"
		}
		text := internalstrings.TrimTrailingNewlines(transcript)
		if text == "" {
			text = "-"
		}
		transcripts = append(transcripts, AgentTranscript{Purpose: session.Purpose, Transcript: text})
	}
	return transcripts, nil
}

func testingStageOutcome(results []TestCommandResult) (Stage, string) {
	var failed []TestCommandResult
	for _, result := range results {
		if result.ExitCode != 0 {
			failed = append(failed, result)
		}
	}
	if len(failed) == 0 {
		return StageReviewing, ""
	}
	return StageImplementing, FormatTestFeedback(results)
}

func diffStatHasChanges(diffStat string) bool {
	lines := strings.Split(diffStat, "\n")
	seenChangeLine := false
	seenSummary := false
	changedSummary := false
	for _, line := range lines {
		line = internalstrings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "No changes") {
			return false
		}
		if strings.Contains(line, " file changed") || strings.Contains(line, " files changed") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				count, err := strconv.Atoi(fields[0])
				if err == nil {
					seenSummary = true
					changedSummary = count != 0
				}
			}
			continue
		}
		if strings.Contains(line, " | ") {
			seenChangeLine = true
		}
	}
	if seenSummary {
		return changedSummary || seenChangeLine
	}
	return seenChangeLine
}

func renderPromptParts(item todo.Todo, feedback, message, seriesLog string, transcripts []AgentTranscript, name, workspacePath string) (string, string, error) {
	prompt, err := LoadPrompt(workspacePath, name)
	if err != nil {
		return "", "", err
	}
	context, err := loadPromptContext(workspacePath)
	if err != nil {
		return "", "", err
	}
	parts, err := buildPromptParts(item, feedback, message, seriesLog, transcripts, workspacePath, nil, context, prompt)
	if err != nil {
		return "", "", err
	}
	promptContent := promptContentFromParts(parts)
	return promptContent.PhaseContent, promptContent.UserContent, nil
}

func renderPromptLog(prompt internalagent.PromptContent) string {
	content := prompt.UserContent
	if content == "" {
		content = "(no user content)"
	}
	return fmt.Sprintf("System prompt blocks:\n\n%s\n\nUser message:\n\n%s", formatSystemPromptBlocks(prompt), content)
}

func formatSystemPromptBlocks(prompt internalagent.PromptContent) string {
	lines := []string{}
	appendBlocks := func(label string, blocks []string) {
		for i, block := range blocks {
			block = internalstrings.TrimTrailingNewlines(block)
			if internalstrings.IsBlank(block) {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s %d:", label, i+1))
			lines = append(lines, indentPromptBlock(internalstrings.TrimLeadingNewlines(block)))
		}
	}

	appendBlocks("Project context block", prompt.ProjectContext)
	appendBlocks("Context file block", prompt.ContextFiles)
	if len(prompt.TestCommands) > 0 {
		lines = append(lines, "Test commands:")
		for _, command := range prompt.TestCommands {
			command = internalstrings.TrimSpace(command)
			if internalstrings.IsBlank(command) {
				continue
			}
			lines = append(lines, fmt.Sprintf("  - %s", command))
		}
	}
	if !internalstrings.IsBlank(prompt.PhaseContent) {
		lines = append(lines, "Phase content:")
		lines = append(lines, indentPromptBlock(prompt.PhaseContent))
	}
	if len(lines) == 0 {
		return "(no system prompt content)"
	}
	return strings.Join(lines, "\n")
}

func indentPromptBlock(block string) string {
	block = internalstrings.TrimTrailingNewlines(block)
	block = internalstrings.TrimLeadingNewlines(block)
	if internalstrings.IsBlank(block) {
		return ""
	}
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		trimmed := strings.TrimRight(line, " ")
		if trimmed == "" {
			lines[i] = ""
			continue
		}
		lines[i] = "  " + trimmed
	}
	return strings.Join(lines, "\n")
}

func buildLLMFailureMessage(purpose, promptName string, result AgentRunResult, runOpts AgentRunOptions, beforeCommitID, afterCommitID string, afterCommitErr error, restored bool, restoreErr error, retryCount int) string {
	parts := []string{}
	// Include the error reason first if available - this is the most important context
	if !internalstrings.IsBlank(result.Error) {
		parts = append(parts, fmt.Sprintf("error: %s", result.Error))
	}
	if !internalstrings.IsBlank(result.SessionID) {
		parts = append(parts, fmt.Sprintf("session %s", result.SessionID))
	}
	if !internalstrings.IsBlank(runOpts.Model) {
		parts = append(parts, fmt.Sprintf("model %q", runOpts.Model))
	}
	if !internalstrings.IsBlank(promptName) {
		parts = append(parts, fmt.Sprintf("prompt %s", promptName))
	}
	if !internalstrings.IsBlank(runOpts.RepoPath) {
		parts = append(parts, fmt.Sprintf("repo %s", runOpts.RepoPath))
	}
	if !internalstrings.IsBlank(runOpts.WorkspacePath) {
		parts = append(parts, fmt.Sprintf("workspace %s", runOpts.WorkspacePath))
	}
	if !internalstrings.IsBlank(beforeCommitID) {
		parts = append(parts, fmt.Sprintf("before %s", beforeCommitID))
	}
	if !internalstrings.IsBlank(afterCommitID) {
		parts = append(parts, fmt.Sprintf("after %s", afterCommitID))
	}
	if afterCommitErr != nil {
		parts = append(parts, fmt.Sprintf("after_commit_error %v", afterCommitErr))
	}
	if restored {
		parts = append(parts, fmt.Sprintf("restored %s", beforeCommitID))
	}
	if restoreErr != nil {
		parts = append(parts, fmt.Sprintf("restore_error %v", restoreErr))
	}
	if retryCount > 0 {
		parts = append(parts, fmt.Sprintf("retry %d", retryCount))
	}
	message := fmt.Sprintf("agent %s failed with exit code %d", purpose, result.ExitCode)
	if result.ExitCode < 0 {
		message += " (process did not exit cleanly)"
	}
	if len(parts) == 0 {
		return message
	}
	return fmt.Sprintf("%s: %s", message, strings.Join(parts, ", "))
}

// buildReviewFailureMessage builds the full error message for agent review failures.
// Format: "agent <purpose> failed with exit code <n>: <details>" matching buildLLMFailureMessage.
func buildReviewFailureMessage(purpose string, result AgentRunResult, model string) string {
	parts := []string{}
	// Include the error reason first if available - this is the most important context
	if !internalstrings.IsBlank(result.Error) {
		parts = append(parts, fmt.Sprintf("error: %s", result.Error))
	}
	if !internalstrings.IsBlank(result.SessionID) {
		parts = append(parts, fmt.Sprintf("session %s", result.SessionID))
	}
	if !internalstrings.IsBlank(model) {
		parts = append(parts, fmt.Sprintf("model %q", model))
	}
	message := fmt.Sprintf("agent %s failed with exit code %d", purpose, result.ExitCode)
	if result.ExitCode < 0 {
		message += " (process did not exit cleanly)"
	}
	if len(parts) == 0 {
		return message
	}
	return fmt.Sprintf("%s: %s", message, strings.Join(parts, ", "))
}

// isContextOverflowError returns true if the error message indicates a context
// overflow (max tokens reached or prompt too long). These errors can be retried
// with a fresh context.
func isContextOverflowError(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	return strings.Contains(lower, "context overflow") ||
		strings.Contains(lower, "maximum context length") ||
		strings.Contains(lower, "context_length_exceeded") ||
		strings.Contains(lower, "prompt is too long") ||
		strings.Contains(lower, "request too large")
}

func isModelEOFError(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	return strings.Contains(lower, "unexpected eof")
}

func ensureCommitMessageInPrompt(prompt, message string) string {
	if internalstrings.IsBlank(message) {
		return prompt
	}
	if promptMessagePattern.MatchString(prompt) {
		return prompt
	}
	trimmed := internalstrings.TrimTrailingNewlines(prompt)
	return trimmed + "\n\n{{.CommitMessageBlock}}\n"
}

type commitMessageMissingError struct {
	Path string
	Err  error
}

func (err commitMessageMissingError) Error() string {
	return fmt.Sprintf("commit message missing; expected at %s: %v", err.Path, err.Err)
}

func (err commitMessageMissingError) Unwrap() error {
	return err.Err
}

func writeCommitMessageSeed(path, message string) error {
	if internalstrings.IsBlank(message) {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat commit message seed: %w", err)
	}
	if err := os.WriteFile(path, []byte(message+"\n"), 0o644); err != nil {
		return fmt.Errorf("write commit message seed: %w", err)
	}
	return nil
}

func readCommitMessage(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", commitMessageMissingError{Path: path, Err: err}
		}
		return "", fmt.Errorf("read commit message: %w", err)
	}
	removeErr := removeFileIfExists(path)
	if removeErr != nil {
		removeErr = fmt.Errorf("remove commit message: %w", removeErr)
	}
	message := normalizeCommitMessage(string(data))
	if internalstrings.IsBlank(message) {
		return "", errors.Join(fmt.Errorf("commit message is empty"), removeErr)
	}
	if removeErr != nil {
		return "", removeErr
	}
	return message, nil
}

func resolveReviewCommitMessage(commitMessage, workspacePath string, requireMessage bool) (string, error) {
	if !internalstrings.IsBlank(commitMessage) {
		return commitMessage, nil
	}
	if internalstrings.IsBlank(workspacePath) {
		return "", nil
	}
	messagePath := filepath.Join(workspacePath, commitMessageFilename)
	message, err := readCommitMessage(messagePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if !requireMessage {
				return "", nil
			}
			return "", fmt.Errorf(
				"commit message missing before LLM review; LLM implementation was instructed to write %s: %w",
				messagePath,
				err,
			)
		}
		return "", err
	}
	return message, nil
}

func removeFileIfExists(path string) error {
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func updateStaleWorkspace(update func(string) error, workspacePath string) {
	if update == nil {
		return
	}
	_ = update(workspacePath)
}

func snapshotWorkspace(snapshot func(string) error, workspacePath string) {
	if snapshot == nil {
		return
	}
	_ = snapshot(workspacePath)
}

func finalizeTodo(repoPath, todoID string, status Status) error {
	switch status {
	case StatusCompleted:
		return finishTodo(repoPath, todoID)
	case StatusFailed, StatusAbandoned:
		return reopenTodo(repoPath, todoID)
	default:
		return nil
	}
}

// reloadTodo re-reads a todo from the store. This allows the job runner to
// pick up edits made to a todo from another process between implementation runs.
func reloadTodo(repoPath, todoID string) (todo.Todo, error) {
	store, err := todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		Purpose:         fmt.Sprintf("reload todo %s", todoID),
	})
	if err != nil {
		return todo.Todo{}, err
	}
	items, err := store.Show([]string{todoID})
	releaseErr := store.Release()
	if err != nil {
		return todo.Todo{}, errors.Join(err, releaseErr)
	}
	if releaseErr != nil {
		return todo.Todo{}, releaseErr
	}
	if len(items) == 0 {
		return todo.Todo{}, fmt.Errorf("todo not found: %s", todoID)
	}
	return items[0], nil
}

type todoStore interface {
	Start([]string) ([]todo.Todo, error)
	Release() error
}

var openTodoStore = func(repoPath string, opts todo.OpenOptions) (todoStore, error) {
	return todo.Open(repoPath, opts)
}

func updateTodoStatus(repoPath, todoID string, update func(*todo.Store, string) ([]todo.Todo, error)) error {
	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
	if err != nil {
		return err
	}
	_, err = update(store, todoID)
	releaseErr := store.Release()
	if err != nil {
		return errors.Join(err, releaseErr)
	}
	return releaseErr
}

func startTodo(repoPath, todoID string) error {
	store, err := openTodoStore(repoPath, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		Purpose:         fmt.Sprintf("todo store (job start %s)", todoID),
	})
	if err != nil {
		return err
	}
	_, startErr := store.Start([]string{todoID})
	releaseErr := store.Release()
	if startErr != nil {
		return errors.Join(startErr, releaseErr)
	}
	if releaseErr == nil {
		return nil
	}
	return errors.Join(reopenTodo(repoPath, todoID), releaseErr)
}

func finishTodo(repoPath, todoID string) error {
	return updateTodoStatus(repoPath, todoID, func(store *todo.Store, id string) ([]todo.Todo, error) {
		return store.Finish([]string{id})
	})
}

func reopenTodo(repoPath, todoID string) error {
	return updateTodoStatus(repoPath, todoID, func(store *todo.Store, id string) ([]todo.Todo, error) {
		return store.Reopen([]string{id})
	})
}

func checkWorkspaceForResume(workspacePath string, changes []JobChange, opts RunOptions) error {
	if opts.CurrentChangeEmpty == nil {
		return fmt.Errorf("current change empty check is required")
	}
	currentEmpty, err := opts.CurrentChangeEmpty(workspacePath)
	if err != nil {
		return fmt.Errorf("check working copy: %w", err)
	}
	if !currentEmpty {
		return fmt.Errorf("working copy is not empty; run jj abandon first")
	}

	completedIDs := make([]string, 0)
	for _, change := range changes {
		if change.IsComplete() {
			completedIDs = append(completedIDs, change.ChangeID)
		}
	}
	if len(completedIDs) == 0 {
		return nil
	}
	if opts.ChangeIDsForRevset == nil {
		return fmt.Errorf("change id lookup is required")
	}
	revset := fmt.Sprintf("ancestors(@) & (%s)", strings.Join(completedIDs, " | "))
	found, err := opts.ChangeIDsForRevset(workspacePath, revset)
	if err != nil {
		return fmt.Errorf("check change history: %w", err)
	}
	if len(found) == 0 {
		return fmt.Errorf("missing completed changes: %s", strings.Join(completedIDs, ", "))
	}
	foundSet := make(map[string]struct{}, len(found))
	for _, id := range found {
		foundSet[id] = struct{}{}
	}
	missing := make([]string, 0)
	for _, id := range completedIDs {
		if _, ok := foundSet[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing completed changes: %s", strings.Join(missing, ", "))
	}
	return nil
}

