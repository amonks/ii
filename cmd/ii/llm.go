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

	"github.com/amonks/incrementum/internal/ui"
	"github.com/amonks/incrementum/llm"
	"github.com/spf13/cobra"
)

var llmCmd = &cobra.Command{
	Use:   "llm",
	Short: "LLM completions and model management",
}

var llmCompleteCmd = &cobra.Command{
	Use:   "complete [prompt]",
	Short: "Generate a completion from an LLM",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runLLMComplete,
}

var llmModelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List configured LLM models",
	RunE:  runLLMModels,
}

var llmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List completion history",
	RunE:  runLLMList,
}

var llmShowCmd = &cobra.Command{
	Use:   "show <completion-id>",
	Short: "Show a specific completion from history",
	Args:  cobra.ExactArgs(1),
	RunE:  runLLMShow,
}

var (
	llmCompleteModel       string
	llmCompleteTemperature float64
	llmCompleteMaxTokens   int
	llmModelsJSON          bool
	llmListJSON            bool
)

func init() {
	rootCmd.AddCommand(llmCmd)
	llmCmd.AddCommand(llmCompleteCmd, llmModelsCmd, llmListCmd, llmShowCmd)

	llmCompleteCmd.Flags().StringVar(&llmCompleteModel, "model", "", "Model to use (defaults to llm.model from config)")
	llmCompleteCmd.Flags().Float64Var(&llmCompleteTemperature, "temperature", 0, "Sampling temperature (0 = default)")
	llmCompleteCmd.Flags().IntVar(&llmCompleteMaxTokens, "max-tokens", 0, "Maximum output tokens (0 = default)")

	llmModelsCmd.Flags().BoolVar(&llmModelsJSON, "json", false, "Output as JSON")
	llmListCmd.Flags().BoolVar(&llmListJSON, "json", false, "Output as JSON")
}

func runLLMComplete(cmd *cobra.Command, args []string) error {
	store, err := openLLMStore()
	if err != nil {
		return err
	}

	prompt, err := resolveLLMPrompt(args, os.Stdin)
	if err != nil {
		return err
	}

	model, err := resolveLLMModel(store, llmCompleteModel)
	if err != nil {
		return fmt.Errorf("resolve model: %w", err)
	}

	// Build request
	req := llm.Request{
		Messages: []llm.Message{
			llm.UserMessage{
				Role: "user",
				Content: []llm.ContentBlock{
					llm.TextContent{Type: "text", Text: prompt},
				},
				Timestamp: time.Now(),
			},
		},
	}

	// Build options
	opts := llm.StreamOptions{}
	if llmCompleteTemperature > 0 {
		opts.Temperature = &llmCompleteTemperature
	}
	if llmCompleteMaxTokens > 0 {
		opts.MaxTokens = &llmCompleteMaxTokens
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

	// Start streaming
	handle, err := store.Stream(ctx, model, req, opts)
	if err != nil {
		return fmt.Errorf("stream: %w", err)
	}

	// Stream text to stdout
	for event := range handle.Events {
		switch e := event.(type) {
		case llm.TextDeltaEvent:
			fmt.Print(e.Delta)
		}
	}

	// Wait for completion
	msg, err := handle.Wait()
	if err != nil {
		return err
	}

	// Ensure newline at end
	if len(msg.Content) > 0 {
		fmt.Println()
	}

	return nil
}

func runLLMModels(cmd *cobra.Command, args []string) error {
	store, err := openLLMStore()
	if err != nil {
		return err
	}

	models, err := store.ListModels()
	if err != nil {
		return fmt.Errorf("list models: %w", err)
	}

	if llmModelsJSON {
		return encodeJSONToStdout(models)
	}

	if len(models) == 0 {
		fmt.Println("No models configured.")
		return nil
	}

	rows := make([][]string, 0, len(models))
	for _, m := range models {
		rows = append(rows, []string{
			m.ID,
			m.Provider,
			string(m.API),
		})
	}

	fmt.Print(ui.FormatTable([]string{"MODEL", "PROVIDER", "API"}, rows))
	return nil
}

func runLLMList(cmd *cobra.Command, args []string) error {
	store, err := openLLMStore()
	if err != nil {
		return err
	}

	completions, err := store.ListCompletions()
	if err != nil {
		return fmt.Errorf("list completions: %w", err)
	}

	if llmListJSON {
		return encodeJSONToStdout(completions)
	}

	if len(completions) == 0 {
		fmt.Println("No completions in history.")
		return nil
	}

	now := time.Now()
	rows := make([][]string, 0, len(completions))

	// Calculate prefix lengths for unique ID display
	prefixLengths := completionPrefixLengths(completions)

	for _, c := range completions {
		age := formatCompletionAge(c, now)
		prefixLen := ui.PrefixLength(prefixLengths, c.ID)

		rows = append(rows, []string{
			ui.HighlightID(c.ID, prefixLen),
			c.Model,
			age,
		})
	}

	fmt.Print(ui.FormatTable([]string{"ID", "MODEL", "AGE"}, rows))
	return nil
}

func runLLMShow(cmd *cobra.Command, args []string) error {
	store, err := openLLMStore()
	if err != nil {
		return err
	}

	completion, err := store.GetCompletion(args[0])
	if err != nil {
		return err
	}

	// Print request
	fmt.Println("## Request")
	fmt.Println()
	if len(completion.Request.System) > 0 {
		var parts []string
		for _, block := range completion.Request.System {
			parts = append(parts, block.Text)
		}
		fmt.Printf("**System:** %s\n\n", strings.Join(parts, "\n\n"))
	}
	for _, msg := range completion.Request.Messages {
		printMessage(msg)
	}

	// Print response
	fmt.Println("## Response")
	fmt.Println()
	printMessage(completion.Response)

	// Print metadata
	fmt.Println("## Metadata")
	fmt.Println()
	fmt.Printf("- **ID:** %s\n", completion.ID)
	fmt.Printf("- **Model:** %s\n", completion.Model)
	fmt.Printf("- **Created:** %s\n", completion.CreatedAt.Format(time.RFC3339))
	fmt.Printf("- **Tokens:** input=%d, output=%d, total=%d\n",
		completion.Response.Usage.Input,
		completion.Response.Usage.Output,
		completion.Response.Usage.Total)
	fmt.Printf("- **Cost:** $%.6f\n", completion.Response.Usage.Cost.Total)

	return nil
}

func printMessage(msg llm.Message) {
	switch m := msg.(type) {
	case llm.UserMessage:
		fmt.Println("**User:**")
		for _, block := range m.Content {
			if tc, ok := block.(llm.TextContent); ok {
				fmt.Printf("%s\n", tc.Text)
			}
		}
		fmt.Println()
	case llm.AssistantMessage:
		fmt.Println("**Assistant:**")
		for _, block := range m.Content {
			switch b := block.(type) {
			case llm.TextContent:
				fmt.Printf("%s\n", b.Text)
			case llm.ThinkingContent:
				fmt.Printf("*[Thinking]*\n%s\n", b.Thinking)
			case llm.ToolCall:
				fmt.Printf("*[Tool: %s]*\n", b.Name)
			}
		}
		fmt.Println()
	case llm.ToolResultMessage:
		fmt.Printf("**Tool Result (%s):**\n", m.ToolName)
		for _, block := range m.Content {
			if tc, ok := block.(llm.TextContent); ok {
				fmt.Printf("%s\n", tc.Text)
			}
		}
		fmt.Println()
	}
}

// ErrEmptyLLMPrompt is returned when no prompt is provided.
var ErrEmptyLLMPrompt = errors.New("prompt is required: provide as argument or via stdin")

func resolveLLMPrompt(args []string, reader io.Reader) (string, error) {
	if len(args) > 0 {
		prompt := args[0]
		if strings.TrimSpace(prompt) == "" {
			return "", ErrEmptyLLMPrompt
		}
		return prompt, nil
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read prompt from stdin: %w", err)
	}

	prompt := strings.TrimSpace(string(data))
	if prompt == "" {
		return "", ErrEmptyLLMPrompt
	}

	return prompt, nil
}

// resolveLLMModel resolves the model to use for a completion.
// If modelFlag is provided, it is used directly.
// Otherwise, the default model from config (llm.model) is used.
func resolveLLMModel(store *llm.Store, modelFlag string) (llm.Model, error) {
	if modelFlag != "" {
		return store.GetModel(modelFlag)
	}
	return store.GetDefaultModel()
}

func openLLMStore() (*llm.Store, error) {
	repoPath, err := getRepoPath()
	if err != nil {
		// If we can't find a repo path, open without one (global only)
		return llm.Open()
	}
	return llm.OpenWithOptions(llm.Options{
		RepoPath: repoPath,
	})
}

func completionPrefixLengths(completions []llm.Completion) map[string]int {
	ids := make([]string, 0, len(completions))
	for _, c := range completions {
		ids = append(ids, c.ID)
	}
	return ui.UniqueIDPrefixLengths(ids)
}

func formatCompletionAge(completion llm.Completion, now time.Time) string {
	if completion.CreatedAt.IsZero() {
		return "-"
	}
	return ui.FormatDurationShort(now.Sub(completion.CreatedAt))
}
