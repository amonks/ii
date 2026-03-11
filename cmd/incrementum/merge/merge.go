package merge

import (
	"context"
	"fmt"
	"time"

	internalagent "monks.co/pkg/agent"
	"monks.co/incrementum/internal/jj"
	"monks.co/incrementum/internal/paths"
	internalstrings "monks.co/incrementum/internal/strings"
	jobpkg "monks.co/incrementum/job"
)

// RunLLMFunc runs a conflict resolution agent.
type RunLLMFunc func(jobpkg.AgentRunOptions) (jobpkg.AgentRunResult, error)

// Options configures a merge operation.
type Options struct {
	RepoPath      string
	WorkspacePath string
	ChangeID      string
	Target        string
	RunLLM        RunLLMFunc
	Now           func() time.Time
}

// Merge rebases a change onto a target bookmark and resolves conflicts.
func Merge(ctx context.Context, opts Options) error {
	opts = normalizeOptions(opts)
	if internalstrings.IsBlank(opts.RepoPath) {
		return fmt.Errorf("repo path is required")
	}
	if internalstrings.IsBlank(opts.WorkspacePath) {
		return fmt.Errorf("workspace path is required")
	}
	if internalstrings.IsBlank(opts.ChangeID) {
		return fmt.Errorf("change ID is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	client := jj.New()
	if err := client.Rebase(opts.WorkspacePath, opts.ChangeID, opts.Target); err != nil {
		return fmt.Errorf("rebase onto %s: %w", opts.Target, err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	for {
		revset := fmt.Sprintf("%s::%s", opts.Target, opts.ChangeID)
		conflicts, err := client.ConflictedInRange(opts.WorkspacePath, revset)
		if err != nil {
			return fmt.Errorf("list conflicts: %w", err)
		}
		if len(conflicts) == 0 {
			break
		}
		if opts.RunLLM == nil {
			return fmt.Errorf("merge conflicts detected but RunLLM is not configured")
		}
		if err := resolveConflicts(ctx, opts, client, conflicts); err != nil {
			return err
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := client.BookmarkSet(opts.WorkspacePath, opts.Target, opts.ChangeID); err != nil {
		return fmt.Errorf("advance bookmark %s: %w", opts.Target, err)
	}
	finalRevset := fmt.Sprintf("%s::%s", opts.Target, opts.ChangeID)
	remaining, err := client.ConflictedInRange(opts.WorkspacePath, finalRevset)
	if err != nil {
		return fmt.Errorf("list conflicts: %w", err)
	}
	if len(remaining) > 0 {
		return fmt.Errorf("conflicts remain after merge")
	}
	return nil
}

func normalizeOptions(opts Options) Options {
	if opts.Target == "" {
		opts.Target = "main"
	}
	if opts.WorkspacePath == "" {
		opts.WorkspacePath = opts.RepoPath
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return opts
}

func resolveConflicts(ctx context.Context, opts Options, client *jj.Client, conflicts []string) error {
	for _, changeID := range conflicts {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := resolveConflict(ctx, opts, client, changeID); err != nil {
			return err
		}
		revset := fmt.Sprintf("%s::%s", opts.Target, opts.ChangeID)
		remaining, err := client.ConflictedInRange(opts.WorkspacePath, revset)
		if err != nil {
			return fmt.Errorf("list conflicts: %w", err)
		}
		if len(remaining) == 0 {
			return nil
		}
	}
	return fmt.Errorf("conflicts remain after resolution")
}

func resolveConflict(ctx context.Context, opts Options, client *jj.Client, changeID string) error {
	if _, err := client.NewChange(opts.WorkspacePath, changeID); err != nil {
		return fmt.Errorf("create resolution change for %s: %w", changeID, err)
	}
	prompt, err := buildConflictPrompt(opts.WorkspacePath, changeID, opts.Target)
	if err != nil {
		return err
	}
	result, err := opts.RunLLM(jobpkg.AgentRunOptions{
		RepoPath:      opts.RepoPath,
		WorkspacePath: opts.WorkspacePath,
		Prompt:        prompt,
		StartedAt:     opts.Now(),
	})
	if err != nil {
		return fmt.Errorf("resolve conflicts for %s: %w", changeID, err)
	}
	if result.ExitCode != 0 {
		if internalstrings.IsBlank(result.Error) {
			return fmt.Errorf("conflict resolution failed for %s", changeID)
		}
		return fmt.Errorf("conflict resolution failed for %s: %s", changeID, result.Error)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := client.Snapshot(opts.WorkspacePath); err != nil {
		return fmt.Errorf("snapshot conflict resolution: %w", err)
	}
	if err := client.Squash(opts.WorkspacePath); err != nil {
		return fmt.Errorf("squash conflict resolution: %w", err)
	}
	return nil
}

func buildConflictPrompt(workDir, changeID, target string) (internalagent.PromptContent, error) {
	contextFiles, err := loadContextFiles(workDir)
	if err != nil {
		return internalagent.PromptContent{}, err
	}
	phase := "Resolve merge conflicts in the current workspace.\n\n" +
		"- Locate and fix conflict markers (<<<<<<<, =======, >>>>>>>).\n" +
		"- Keep only the intended final code.\n" +
		"- Do not introduce unrelated changes.\n" +
		"- Ensure no conflict markers remain in the tree."
	user := fmt.Sprintf("Resolve merge conflicts for change %s rebased onto %s.", changeID, target)
	return internalagent.PromptContent{
		ContextFiles: contextFiles,
		PhaseContent: phase,
		UserContent:  user,
	}, nil
}

func loadContextFiles(workDir string) ([]string, error) {
	globalConfigDir, err := paths.DefaultConfigDir()
	if err != nil {
		return nil, err
	}
	files, err := internalagent.LoadContextFiles(internalagent.LoadContextFilesOptions{
		WorkDir:         workDir,
		GlobalConfigDir: globalConfigDir,
	})
	if err != nil {
		return nil, err
	}
	contents := make([]string, 0, len(files))
	for _, file := range files {
		trimmed := internalstrings.TrimTrailingNewlines(file.Content)
		if internalstrings.IsBlank(trimmed) {
			continue
		}
		contents = append(contents, trimmed)
	}
	return contents, nil
}
