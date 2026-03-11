// Package adapter defines the interface for CLI agent backends
// and provides implementations for different agent binaries.
package agents

import "context"

// Adapter invokes a CLI agent and returns its result.
type Adapter interface {
	Run(ctx context.Context, opts RunOptions) (RunResult, error)
}

// RunOptions contains the inputs for an adapter invocation.
type RunOptions struct {
	WorkDir string
	Prompt  string   // Flattened text prompt
	Model   string   // Model hint (adapter may ignore)
	Env     []string // Additional environment variables
}

// RunResult contains the outputs from an adapter invocation.
type RunResult struct {
	ExitCode     int
	Stdout       string
	Stderr       string
	InputTokens  int // Best-effort, 0 if unknown
	OutputTokens int // Best-effort, 0 if unknown
}
