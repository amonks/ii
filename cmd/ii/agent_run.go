package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/amonks/incrementum/agent"
	"github.com/amonks/incrementum/internal/paths"
	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/internal/todoenv"
	"github.com/amonks/incrementum/llm"
	"github.com/spf13/cobra"
)

var agentRunCmd = &cobra.Command{
	Use:   "run [prompt]",
	Short: "Start a new agent session",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAgentRun,
}

var agentRunModel string

func init() {
	agentCmd.AddCommand(agentRunCmd)
	agentRunCmd.Flags().StringVar(&agentRunModel, "model", "", "Model to use for the agent")
}

func runAgentRun(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	workDir, err := paths.WorkingDir()
	if err != nil {
		return err
	}

	store, err := agent.OpenWithOptions(agent.Options{
		RepoPath: repoPath,
	})
	if err != nil {
		return err
	}

	prompt, err := resolveAgentPrompt(args, os.Stdin)
	if err != nil {
		return err
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
		RepoPath:  repoPath,
		WorkDir:   workDir,
		Prompt:    prompt,
		Model:     agentRunModel,
		StartedAt: time.Now(),
		Version:   buildCommitID,
		Env:       []string{todoenv.ProposerEnvVar + "=true"},
	})
	if err != nil {
		return err
	}

	// Stream events to stderr
	streamAgentEventsToStderr(handle.Events)

	result, err := handle.Wait()
	if err != nil {
		return err
	}

	// Print final response to stdout
	printFinalAgentResponse(result.Messages)

	if result.ExitCode != 0 {
		return exitError{code: result.ExitCode}
	}
	return nil
}

// ErrEmptyPrompt is returned when no prompt is provided to the agent.
var ErrEmptyPrompt = errors.New("prompt is required: provide as argument or via stdin")

func resolveAgentPrompt(args []string, reader io.Reader) (string, error) {
	if len(args) > 0 {
		prompt := args[0]
		if strings.TrimSpace(prompt) == "" {
			return "", ErrEmptyPrompt
		}
		return prompt, nil
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read prompt from stdin: %w", err)
	}

	prompt := strings.TrimSuffix(string(data), "\n")
	prompt = internalstrings.TrimTrailingCarriageReturn(prompt)

	if strings.TrimSpace(prompt) == "" {
		return "", ErrEmptyPrompt
	}

	return prompt, nil
}

func streamAgentEventsToStderr(events <-chan agent.Event) {
	for event := range events {
		switch e := event.(type) {
		case agent.TurnStartEvent:
			fmt.Fprintf(os.Stderr, "--- Turn %d ---\n", e.TurnIndex+1)

		case agent.MessageUpdateEvent:
			// Stream text and thinking deltas
			switch se := e.StreamEvent.(type) {
			case llm.TextDeltaEvent:
				fmt.Fprint(os.Stderr, se.Delta)
			case llm.ThinkingDeltaEvent:
				fmt.Fprint(os.Stderr, se.Delta)
			}

		case agent.MessageEndEvent:
			fmt.Fprintln(os.Stderr) // Newline after message

		case agent.ToolExecutionStartEvent:
			fmt.Fprintf(os.Stderr, "\n[Tool: %s]\n", e.ToolName)

		case agent.ToolExecutionEndEvent:
			// Show tool result summary (truncated)
			for _, block := range e.Result.Content {
				if tc, ok := block.(llm.TextContent); ok {
					output := tc.Text
					if len(output) > 500 {
						output = output[:500] + "..."
					}
					fmt.Fprintf(os.Stderr, "%s\n", output)
				}
			}

		case agent.WaitingForInputEvent:
			fmt.Fprint(os.Stderr, "\n> ")

		case agent.AgentEndEvent:
			fmt.Fprintf(os.Stderr, "\n--- Agent finished (tokens: %d, cost: $%.4f) ---\n",
				e.Usage.Total, e.Usage.Cost.Total)
		}
	}
}

func printFinalAgentResponse(messages []llm.Message) {
	// Find the last assistant message with text content
	var lastText string
	for i := len(messages) - 1; i >= 0; i-- {
		if am, ok := messages[i].(llm.AssistantMessage); ok {
			for _, block := range am.Content {
				if tc, ok := block.(llm.TextContent); ok && tc.Text != "" {
					lastText = tc.Text
					break
				}
			}
			if lastText != "" {
				break
			}
		}
	}

	if lastText != "" {
		fmt.Println(lastText)
	}
}
