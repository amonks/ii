package job

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/amonks/incrementum/habit"
	"github.com/amonks/incrementum/internal/config"
	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/todo"
)

// HabitRunOptions configures habit execution.
type HabitRunOptions struct {
	OnStart       func(HabitStartInfo)
	OnStageChange func(Stage)
	// EventStream receives job events as they are recorded. The channel is closed
	// when RunHabit completes.
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
	// RunLLM runs an LLM session using the agent package.
	RunLLM func(AgentRunOptions) (AgentRunResult, error)
	// Model overrides model selection for all stages when set.
	Model              string
	CurrentCommitID    func(string) (string, error)
	CurrentChangeID    func(string) (string, error)
	ChangeIDAt         func(string, string) (string, error)
	ChangeIDsForRevset func(string, string) ([]string, error)
	CurrentChangeEmpty func(string) (bool, error)
	DiffStat           func(string, string, string) (string, error)
	CommitIDAt         func(string, string) (string, error)
	Commit             func(string, string) error
	RestoreWorkspace   func(string, string) error
	UpdateStale        func(string) error
	Snapshot           func(string) error
	// Transcripts retrieves transcripts for LLM sessions.
	Transcripts     func(string, []AgentSession) ([]AgentTranscript, error)
	EventLog        *EventLog
	EventLogOptions EventLogOptions
	Logger          Logger
}

// HabitRunResult captures the output of running a habit.
type HabitRunResult struct {
	Job           Job
	CommitMessage string
	Artifact      *todo.Todo
	Abandoned     bool
}

// HabitStartInfo captures context when starting a habit run.
type HabitStartInfo struct {
	JobID     string
	Workdir   string
	HabitName string
}

// RunHabit runs a habit job for the given habit name.
func RunHabit(repoPath, habitName string, opts HabitRunOptions) (*HabitRunResult, error) {
	if internalstrings.IsBlank(habitName) {
		return nil, fmt.Errorf("habit name is required")
	}

	opts = normalizeHabitRunOptions(opts)
	if opts.EventStream != nil {
		defer close(opts.EventStream)
	}
	result := &HabitRunResult{}
	repoPath = filepath.Clean(repoPath)
	if abs, absErr := filepath.Abs(repoPath); absErr == nil {
		repoPath = abs
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

	// Load the habit
	h, err := habit.Load(repoPath, habitName)
	if err != nil {
		return result, err
	}

	startedAt := opts.Now()
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

	implModel := resolveHabitModel(opts.Config, opts.Model, h.ImplementationModel, "implement")
	reviewModel := resolveHabitModel(opts.Config, opts.Model, h.ReviewModel, "review")

	// Create a synthetic job for tracking - we use habitName as the todo ID prefix
	created, err := manager.Create("habit:"+habitName, startedAt, CreateOptions{
		Agent:               implModel,
		ImplementationModel: implModel,
		CodeReviewModel:     reviewModel,
	})
	if err != nil {
		return result, err
	}
	result.Job = created

	if opts.OnStart != nil {
		opts.OnStart(HabitStartInfo{
			JobID:     created.ID,
			Workdir:   workspaceAbs,
			HabitName: habitName,
		})
	}

	createdEventLog := false
	if opts.EventLog == nil {
		eventLog, err := OpenEventLog(created.ID, opts.EventLogOptions)
		if err != nil {
			status := StatusFailed
			updated, updateErr := manager.Update(created.ID, UpdateOptions{Status: &status}, opts.Now())
			result.Job = updated
			return result, errors.Join(err, updateErr)
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
		return result, errors.Join(err, updateErr)
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

	habitCtx := habitRunContext{
		repoPath:      repoPath,
		workspacePath: workspacePath,
		habit:         h,
		opts:          opts,
		manager:       manager,
		result:        result,
	}
	finalJob, err := runHabitStages(&habitCtx, created, interrupts)
	result.Job = finalJob
	if err != nil {
		return result, err
	}
	return result, nil
}

type habitRunContext struct {
	repoPath       string
	workspacePath  string
	habit          *habit.Habit
	opts           HabitRunOptions
	manager        *Manager
	result         *HabitRunResult
	commitMessage  string
	reviewComments string
}

func runHabitStages(ctx *habitRunContext, current Job, interrupts <-chan os.Signal) (Job, error) {
	for current.Status == StatusActive {
		if current.Stage != StageImplementing {
			return current, fmt.Errorf("invalid job stage: %s", current.Stage)
		}

		// Implementation stage
		next, stageErr := ctx.runStageWithInterrupt(current, ctx.runHabitImplementingStage(current), interrupts)
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

		// Check if no changes were made (nothing to do)
		if ctx.commitMessage == "" {
			// No changes = abandon (nothing worth doing right now)
			status := StatusCompleted
			updated, err := ctx.manager.Update(current.ID, UpdateOptions{Status: &status}, ctx.opts.Now())
			if err != nil {
				return current, err
			}
			ctx.result.Abandoned = true
			return updated, nil
		}

		// Testing stage
		if current.Stage == StageTesting {
			next, stageErr = ctx.runStageWithInterrupt(current, ctx.runHabitTestingStage(current), interrupts)
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
				continue
			}
		}

		// Review stage (step review only, no project review for habits)
		next, stageErr = ctx.runStageWithInterrupt(current, ctx.runHabitReviewingStage(current), interrupts)
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
			continue
		}

		// Committing stage
		next, stageErr = ctx.runStageWithInterrupt(current, ctx.runHabitCommittingStage(current), interrupts)
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

func (ctx *habitRunContext) runStageWithInterrupt(current Job, stageFn func() (Job, error), interrupts <-chan os.Signal) (Job, error) {
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

func (ctx *habitRunContext) handleInterrupt(current Job) (Job, error) {
	status := StatusFailed
	updated, updateErr := ctx.manager.Update(current.ID, UpdateOptions{Status: &status}, ctx.opts.Now())
	return updated, errors.Join(ErrJobInterrupted, updateErr)
}

func (ctx *habitRunContext) handleStageOutcome(current, next Job, stageErr error) (Job, error) {
	if stageErr != nil {
		if next.Status == StatusAbandoned {
			ctx.result.Job = next
			ctx.result.Abandoned = true
			return next, nil // Abandon is successful for habits
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

func (ctx *habitRunContext) runHabitImplementingStage(current Job) func() (Job, error) {
	return func() (Job, error) {
		logger := resolveLogger(ctx.opts.Logger)
		updateStaleWorkspace(ctx.opts.UpdateStale, ctx.workspacePath)
		feedbackPath := filepath.Join(ctx.workspacePath, feedbackFilename)
		if err := removeFileIfExists(feedbackPath); err != nil {
			return Job{}, err
		}

		beforeCommitID, err := ctx.opts.CurrentCommitID(ctx.workspacePath)
		if err != nil {
			return Job{}, err
		}

		promptName := "prompt-habit-implementation.tmpl"
		if !internalstrings.IsBlank(current.Feedback) {
			promptName = "prompt-feedback.tmpl"
		}
		phase, userContent, err := renderHabitPromptParts(ctx.habit, current.Feedback, ctx.commitMessage, nil, promptName, ctx.workspacePath)
		if err != nil {
			return Job{}, err
		}
		updated := current
		model := resolveHabitModel(ctx.opts.Config, ctx.opts.Model, ctx.habit.ImplementationModel, "implement")
		runOpts := AgentRunOptions{
			RepoPath:      ctx.repoPath,
			WorkspacePath: ctx.workspacePath,
			Prompt:        buildPromptContent(phase, userContent, ctx.workspacePath, ctx.opts.toRunOptions()),
			Model:         model,
			StartedAt:     ctx.opts.Now(),
			EventLog:      ctx.opts.EventLog,
			Env:           agentRunEnv(),
		}
		prompt := renderPromptLog(runOpts.Prompt)
		if err := appendJobEvent(ctx.opts.EventLog, jobEventPrompt, promptEventData{Purpose: "implement", Template: promptName, Prompt: prompt}); err != nil {
			return Job{}, err
		}
		runAttempt := func() (AgentRunResult, error) {
			result, err := runLLMWithEvents(ctx.opts.toRunOptions(), runOpts, "implement")
			if err != nil {
				return AgentRunResult{}, err
			}

			session := AgentSession{Purpose: "implement", ID: result.SessionID}
			updated, err = ctx.manager.Update(updated.ID, UpdateOptions{AppendAgentSession: &session}, ctx.opts.Now())
			if err != nil {
				return AgentRunResult{}, err
			}
			transcript := loadTranscript(ctx.opts.toRunOptions(), session)
			if !internalstrings.IsBlank(transcript) {
				if err := appendJobEvent(ctx.opts.EventLog, jobEventTranscript, transcriptEventData{Purpose: "implement", Transcript: transcript}); err != nil {
					return AgentRunResult{}, err
				}
			}
			logger.Prompt(PromptLog{Purpose: "implement", Template: promptName, Prompt: prompt, Transcript: transcript})
			return result, nil
		}

		llmResult, err := runAttempt()
		if err != nil {
			return Job{}, err
		}

		retryCount := 0
		eofRetryCount := 0
		for llmResult.ExitCode != 0 {
			if isModelEOFError(llmResult.Error) && eofRetryCount < 2 {
				eofRetryCount++
				llmResult, err = runAttempt()
				if err != nil {
					return Job{}, err
				}
				continue
			}
			afterCommitID := ""
			var afterCommitErr error
			if ctx.opts.CurrentCommitID != nil && !internalstrings.IsBlank(ctx.workspacePath) {
				afterCommitID, afterCommitErr = ctx.opts.CurrentCommitID(ctx.workspacePath)
			}
			restored := false
			var restoreErr error
			if llmResult.ExitCode < 0 && afterCommitErr == nil && afterCommitID != "" && beforeCommitID != "" && afterCommitID != beforeCommitID {
				if ctx.opts.RestoreWorkspace != nil {
					restoreErr = ctx.opts.RestoreWorkspace(ctx.workspacePath, beforeCommitID)
					if restoreErr == nil {
						restored = true
					}
				}
			}
			if restored && retryCount == 0 {
				retryCount++
				llmResult, err = runAttempt()
				if err != nil {
					return Job{}, err
				}
				continue
			}
			return Job{}, errors.New(buildLLMFailureMessage("implement", promptName, llmResult, runOpts, beforeCommitID, afterCommitID, afterCommitErr, restored, restoreErr, retryCount))
		}

		afterCommitID, err := ctx.opts.CurrentCommitID(ctx.workspacePath)
		if err != nil {
			return Job{}, err
		}

		changed := beforeCommitID != afterCommitID
		if changed {
			if ctx.opts.CurrentChangeEmpty == nil {
				return Job{}, fmt.Errorf("current change empty check is required")
			}
			empty, err := ctx.opts.CurrentChangeEmpty(ctx.workspacePath)
			if err != nil {
				return Job{}, err
			}
			if empty {
				changed = false
			}
		}
		message := ""
		if changed {
			messagePath := filepath.Join(ctx.workspacePath, commitMessageFilename)
			message, err = readCommitMessage(messagePath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					// No commit message means nothing to commit for habits
					changed = false
				} else {
					return Job{}, err
				}
			}
			if message != "" {
				logger.CommitMessage(CommitMessageLog{Label: "Draft", Message: message})
				if err := appendJobEvent(ctx.opts.EventLog, jobEventCommitMessage, commitMessageEventData{Label: "Draft", Message: message}); err != nil {
					return Job{}, err
				}
			}
		}
		if !changed {
			messagePath := filepath.Join(ctx.workspacePath, commitMessageFilename)
			if err := removeFileIfExists(messagePath); err != nil {
				return Job{}, err
			}
		}

		ctx.commitMessage = message

		nextStage := StageTesting
		if !changed {
			nextStage = StageReviewing
		}
		updated, err = ctx.manager.Update(updated.ID, UpdateOptions{Stage: &nextStage}, ctx.opts.Now())
		if err != nil {
			return Job{}, err
		}
		return updated, nil
	}
}

func (ctx *habitRunContext) runHabitTestingStage(current Job) func() (Job, error) {
	return func() (Job, error) {
		logger := resolveLogger(ctx.opts.Logger)
		cfg := ctx.opts.Config
		if cfg == nil {
			var err error
			cfg, err = ctx.opts.LoadConfig(ctx.repoPath)
			if err != nil {
				return Job{}, fmt.Errorf("load config: %w", err)
			}
		}
		if len(cfg.Job.TestCommands) < 1 {
			return Job{}, fmt.Errorf("job test-commands must be configured")
		}

		results, err := ctx.opts.RunTests(ctx.workspacePath, cfg.Job.TestCommands)
		if err != nil {
			return Job{}, err
		}
		logger.Tests(TestLog{Results: results})
		if err := appendJobEvent(ctx.opts.EventLog, jobEventTests, buildTestsEventData(results)); err != nil {
			return Job{}, err
		}

		nextStage, feedback := testingStageOutcome(results)
		update := UpdateOptions{Stage: &nextStage}
		if feedback != "" {
			update.Feedback = &feedback
		} else {
			empty := ""
			update.Feedback = &empty
		}
		updated, err := ctx.manager.Update(current.ID, update, ctx.opts.Now())
		if err != nil {
			return Job{}, err
		}
		return updated, nil
	}
}

func (ctx *habitRunContext) runHabitReviewingStage(current Job) func() (Job, error) {
	return func() (Job, error) {
		logger := resolveLogger(ctx.opts.Logger)
		updateStaleWorkspace(ctx.opts.UpdateStale, ctx.workspacePath)
		feedbackPath := filepath.Join(ctx.workspacePath, feedbackFilename)
		if err := removeFileIfExists(feedbackPath); err != nil {
			return Job{}, err
		}

		message, err := resolveReviewCommitMessage(ctx.commitMessage, ctx.workspacePath, true)
		if err != nil {
			return Job{}, err
		}

		promptName := "prompt-habit-review.tmpl"
		model := resolveHabitModel(ctx.opts.Config, ctx.opts.Model, ctx.habit.ReviewModel, "review")

		promptTemplate, err := LoadPrompt(ctx.workspacePath, promptName)
		if err != nil {
			return Job{}, err
		}
		context, err := loadPromptContext(ctx.workspacePath)
		if err != nil {
			return Job{}, err
		}
		parts, err := buildHabitPromptParts(ctx.habit.Name, ctx.habit.Instructions, "", message, nil, ctx.workspacePath, context, promptTemplate)
		if err != nil {
			return Job{}, err
		}
		promptContent := promptContentFromParts(parts)
		prompt := renderPromptLog(promptContent)
		if err := appendJobEvent(ctx.opts.EventLog, jobEventPrompt, promptEventData{Purpose: "review", Template: promptName, Prompt: prompt}); err != nil {
			return Job{}, err
		}

		llmResult, err := runLLMWithEvents(ctx.opts.toRunOptions(), AgentRunOptions{
			RepoPath:      ctx.repoPath,
			WorkspacePath: ctx.workspacePath,
			Prompt:        promptContent,
			Model:         model,
			StartedAt:     ctx.opts.Now(),
			EventLog:      ctx.opts.EventLog,
			Env:           agentRunEnv(),
		}, "review")
		if err != nil {
			return Job{}, err
		}

		session := AgentSession{Purpose: "review", ID: llmResult.SessionID}
		updated, err := ctx.manager.Update(current.ID, UpdateOptions{AppendAgentSession: &session}, ctx.opts.Now())
		if err != nil {
			return Job{}, err
		}
		transcript := loadTranscript(ctx.opts.toRunOptions(), session)
		if !internalstrings.IsBlank(transcript) {
			if err := appendJobEvent(ctx.opts.EventLog, jobEventTranscript, transcriptEventData{Purpose: "review", Transcript: transcript}); err != nil {
				return Job{}, err
			}
		}
		logger.Prompt(PromptLog{Purpose: "review", Template: promptName, Prompt: prompt, Transcript: transcript})

		if llmResult.ExitCode != 0 {
			return Job{}, fmt.Errorf("%s", buildReviewFailureMessage("review", llmResult, model))
		}

		feedback, err := ReadReviewFeedback(feedbackPath)
		if err != nil {
			return Job{}, err
		}
		logger.Review(ReviewLog{Purpose: "review", Feedback: feedback})
		if err := appendJobEvent(ctx.opts.EventLog, jobEventReview, reviewEventData{Purpose: "review", Outcome: feedback.Outcome, Details: feedback.Details}); err != nil {
			return Job{}, err
		}

		switch feedback.Outcome {
		case ReviewOutcomeAccept:
			ctx.reviewComments = feedback.Details
			nextStage := StageCommitting
			empty := ""
			updated, err = ctx.manager.Update(updated.ID, UpdateOptions{Stage: &nextStage, Feedback: &empty}, ctx.opts.Now())
			if err != nil {
				return Job{}, err
			}
			return updated, nil
		case ReviewOutcomeAbandon:
			status := StatusAbandoned
			updated, err = ctx.manager.Update(updated.ID, UpdateOptions{Status: &status}, ctx.opts.Now())
			if err != nil {
				return Job{}, err
			}
			return updated, &AbandonedError{Reason: feedback.Details}
		case ReviewOutcomeRequestChanges:
			nextStage := StageImplementing
			updated, err = ctx.manager.Update(updated.ID, UpdateOptions{Stage: &nextStage, Feedback: &feedback.Details}, ctx.opts.Now())
			if err != nil {
				return Job{}, err
			}
			return updated, nil
		default:
			return Job{}, ErrInvalidFeedbackFormat
		}
	}
}

func (ctx *habitRunContext) runHabitCommittingStage(current Job) func() (Job, error) {
	return func() (Job, error) {
		logger := resolveLogger(ctx.opts.Logger)
		updateStaleWorkspace(ctx.opts.UpdateStale, ctx.workspacePath)
		if ctx.opts.DiffStat == nil {
			return Job{}, fmt.Errorf("diff stat is required")
		}
		diffStat, err := ctx.opts.DiffStat(ctx.workspacePath, "@-", "@")
		if err != nil {
			return Job{}, err
		}
		if !diffStatHasChanges(diffStat) {
			nextStage := StageImplementing
			updated, err := ctx.manager.Update(current.ID, UpdateOptions{Stage: &nextStage}, ctx.opts.Now())
			if err != nil {
				return Job{}, err
			}
			return updated, nil
		}
		message := internalstrings.TrimSpace(ctx.commitMessage)
		if message == "" {
			return Job{}, fmt.Errorf("commit message is required")
		}

		finalMessage := formatHabitCommitMessage(ctx.habit, message, ctx.reviewComments)
		logMessage := formatHabitCommitMessageWithWidth(ctx.habit, message, ctx.reviewComments, lineWidth-subdocumentIndent)
		ctx.result.CommitMessage = finalMessage
		logger.CommitMessage(CommitMessageLog{Label: "Final", Message: logMessage, Preformatted: true})
		if err := appendJobEvent(ctx.opts.EventLog, jobEventCommitMessage, commitMessageEventData{Label: "Final", Message: logMessage, Preformatted: true}); err != nil {
			return Job{}, err
		}

		updateStaleWorkspace(ctx.opts.UpdateStale, ctx.workspacePath)
		if err := ctx.opts.Commit(ctx.workspacePath, finalMessage); err != nil {
			return Job{}, err
		}

		// Create artifact todo
		artifact, err := createHabitArtifact(ctx.repoPath, ctx.habit.Name, message)
		if err != nil {
			return Job{}, fmt.Errorf("create artifact todo: %w", err)
		}
		ctx.result.Artifact = artifact

		status := StatusCompleted
		updated, err := ctx.manager.Update(current.ID, UpdateOptions{Status: &status}, ctx.opts.Now())
		if err != nil {
			return Job{}, err
		}
		return updated, nil
	}
}

func (opts *HabitRunOptions) toRunOptions() RunOptions {
	return RunOptions{
		Now:                opts.Now,
		LoadConfig:         opts.LoadConfig,
		Config:             opts.Config,
		RunTests:           opts.RunTests,
		RunLLM:             opts.RunLLM,
		Model:              opts.Model,
		CurrentCommitID:    opts.CurrentCommitID,
		CurrentChangeID:    opts.CurrentChangeID,
		ChangeIDAt:         opts.ChangeIDAt,
		ChangeIDsForRevset: opts.ChangeIDsForRevset,
		CurrentChangeEmpty: opts.CurrentChangeEmpty,
		DiffStat:           opts.DiffStat,
		CommitIDAt:         opts.CommitIDAt,
		Commit:             opts.Commit,
		RestoreWorkspace:   opts.RestoreWorkspace,
		UpdateStale:        opts.UpdateStale,
		Snapshot:           opts.Snapshot,
		Transcripts:        opts.Transcripts,
		EventLog:           opts.EventLog,
		Logger:             opts.Logger,
	}
}

func normalizeHabitRunOptions(opts HabitRunOptions) HabitRunOptions {
	runOpts := normalizeRunOptions(RunOptions{
		Now:                opts.Now,
		LoadConfig:         opts.LoadConfig,
		Config:             opts.Config,
		RunTests:           opts.RunTests,
		RunLLM:             opts.RunLLM,
		Model:              opts.Model,
		CurrentCommitID:    opts.CurrentCommitID,
		CurrentChangeID:    opts.CurrentChangeID,
		ChangeIDAt:         opts.ChangeIDAt,
		ChangeIDsForRevset: opts.ChangeIDsForRevset,
		CurrentChangeEmpty: opts.CurrentChangeEmpty,
		DiffStat:           opts.DiffStat,
		CommitIDAt:         opts.CommitIDAt,
		Commit:             opts.Commit,
		RestoreWorkspace:   opts.RestoreWorkspace,
		UpdateStale:        opts.UpdateStale,
		Snapshot:           opts.Snapshot,
		Transcripts:        opts.Transcripts,
		EventLog:           opts.EventLog,
		Logger:             opts.Logger,
	})

	opts.Now = runOpts.Now
	opts.LoadConfig = runOpts.LoadConfig
	opts.RunTests = runOpts.RunTests
	opts.RunLLM = runOpts.RunLLM
	opts.CurrentCommitID = runOpts.CurrentCommitID
	opts.CurrentChangeID = runOpts.CurrentChangeID
	opts.ChangeIDAt = runOpts.ChangeIDAt
	opts.ChangeIDsForRevset = runOpts.ChangeIDsForRevset
	opts.CurrentChangeEmpty = runOpts.CurrentChangeEmpty
	opts.DiffStat = runOpts.DiffStat
	opts.CommitIDAt = runOpts.CommitIDAt
	opts.Commit = runOpts.Commit
	opts.RestoreWorkspace = runOpts.RestoreWorkspace
	opts.UpdateStale = runOpts.UpdateStale
	opts.Snapshot = runOpts.Snapshot
	opts.Transcripts = runOpts.Transcripts
	opts.Logger = runOpts.Logger
	return opts
}

func resolveHabitModel(cfg *config.Config, override, habitModel, purpose string) string {
	if !internalstrings.IsBlank(override) {
		return internalstrings.TrimSpace(override)
	}
	if !internalstrings.IsBlank(habitModel) {
		return internalstrings.TrimSpace(habitModel)
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
	default:
		model = cfg.Job.Model
	}
	if internalstrings.IsBlank(model) {
		model = cfg.Job.Model
	}
	return internalstrings.TrimSpace(model)
}

func renderHabitPromptParts(h *habit.Habit, feedback, message string, transcripts []AgentTranscript, name, workspacePath string) (string, string, error) {
	prompt, err := LoadPrompt(workspacePath, name)
	if err != nil {
		return "", "", err
	}
	context, err := loadPromptContext(workspacePath)
	if err != nil {
		return "", "", err
	}
	data := newHabitPromptData(h.Name, h.Instructions, feedback, message, transcripts, workspacePath, context)
	phase, err := RenderPrompt(workspacePath, prompt, data)
	if err != nil {
		return "", "", err
	}
	userTemplate := "{{.CommitMessageBlock}}"
	if feedback != "" {
		userTemplate = "{{.FeedbackBlock}}"
		if message != "" {
			userTemplate += "\n\nDraft change description (update to reflect your changes):\n{{.CommitMessageBlock}}"
		}
	}
	userContent, err := RenderPrompt(workspacePath, userTemplate, data)
	if err != nil {
		return "", "", err
	}
	promptContent := toPromptContent(phase, userContent, context, nil)
	return promptContent.PhaseContent, promptContent.UserContent, nil
}

// formatHabitCommitMessage formats a commit message for a habit commit.
func formatHabitCommitMessage(h *habit.Habit, message, reviewComments string) string {
	return formatHabitCommitMessageWithWidth(h, message, reviewComments, lineWidth)
}

func formatHabitCommitMessageWithWidth(h *habit.Habit, message, reviewComments string, width int) string {
	summary, body := splitCommitMessage(message)
	formatted := renderMarkdownText(summary, width)

	if body != "" {
		bodyText := renderMarkdownTextOrDash(body, width-documentIndent)
		formatted += "\n\n"
		formatted += IndentBlock(bodyText, documentIndent)
	}

	if reviewComments != "" {
		reviewText := renderMarkdownText(reviewComments, width-documentIndent)
		formatted += "\n\nReview comments:\n\n"
		formatted += IndentBlock(reviewText, documentIndent)
	}

	formatted += fmt.Sprintf("\n\nThis commit was created as part of the '%s' habit:\n\n", h.Name)
	formatted += IndentBlock(h.Instructions, documentIndent)
	return normalizeFormattedCommitMessage(formatted)
}

// createHabitArtifact creates a todo artifact for a completed habit commit.
func createHabitArtifact(repoPath, habitName, commitMessage string) (*todo.Todo, error) {
	store, err := todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  true,
		Purpose:         fmt.Sprintf("habit artifact (habit %s)", habitName),
	})
	if err != nil {
		return nil, err
	}
	defer store.Release()

	summary, body := splitCommitMessage(commitMessage)

	// Create the artifact as in_progress first, then finish it to set ClosedAt correctly
	artifact, err := store.Create(summary, todo.CreateOptions{
		Status:      todo.StatusInProgress,
		Type:        todo.TypeTask,
		Description: body,
	})
	if err != nil {
		return nil, err
	}

	// Finish the todo to set it to done with correct ClosedAt and CompletedAt
	finished, err := store.Finish([]string{artifact.ID})
	if err != nil {
		return nil, err
	}
	if len(finished) == 0 {
		return nil, fmt.Errorf("finish returned no results")
	}

	// Set the source and started_at fields via update.
	// We explicitly set StartedAt because Create doesn't set it when creating
	// with StatusInProgress (since there's no status transition). Per spec,
	// artifact started_at should be set to creation time.
	source := fmt.Sprintf("habit:%s", habitName)
	startedAt := artifact.CreatedAt
	_, err = store.Update([]string{artifact.ID}, todo.UpdateOptions{
		Source:    &source,
		StartedAt: &startedAt,
	})
	if err != nil {
		return nil, err
	}

	// Re-fetch to get updated artifact
	items, err := store.Show([]string{artifact.ID})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return &finished[0], nil
	}
	return &items[0], nil
}
