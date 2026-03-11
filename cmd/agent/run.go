package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"monks.co/pkg/agent"
	"monks.co/pkg/llm"
)

func run() error {
	fs := flag.NewFlagSet("agent run", flag.ExitOnError)
	var (
		model      = fs.String("model", "", "LLM model ID (e.g., claude-sonnet-4-5)")
		workDir    = fs.String("workdir", "", "working directory (default: cwd)")
		prompt     = fs.String("p", "", "prompt text")
		promptFile = fs.String("prompt-file", "", "path to file containing the prompt")
	)

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: agent run [--model MODEL] [--workdir DIR] [-p PROMPT | --prompt-file FILE]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
		fs.PrintDefaults()
	}

	// Skip "run" subcommand if present
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "run" {
		args = args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Resolve prompt
	promptText, err := resolvePrompt(*prompt, *promptFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(promptText) == "" {
		return fmt.Errorf("prompt is required: use -p or --prompt-file")
	}

	// Resolve model
	mdl, err := resolveModel(*model)
	if err != nil {
		return err
	}

	// Resolve working directory
	wd := *workDir
	if wd == "" {
		wd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
	}

	// Set up context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Build agent config
	config := agent.AgentConfig{
		Model:   mdl,
		WorkDir: wd,
		Env:     os.Environ(),
		Permissions: agent.BashPermissions{
			Rules: []agent.BashRule{{Pattern: "*", Allow: true}},
		},
	}

	content := agent.PromptContent{
		UserContent: promptText,
	}

	// Run the agent
	handle, err := agent.Run(ctx, content, config)
	if err != nil {
		return fmt.Errorf("start agent: %w", err)
	}

	result, err := handle.Wait()
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// Print final text response to stdout
	response := extractFinalText(result.Messages)
	if response != "" {
		fmt.Println(response)
	}

	// Print token usage to stderr
	fmt.Fprintf(os.Stderr, "tokens: input=%d output=%d total=%d\n",
		result.Usage.Input, result.Usage.Output, result.Usage.Total)

	return nil
}

func resolvePrompt(promptFlag, promptFileFlag string) (string, error) {
	if promptFlag != "" && promptFileFlag != "" {
		return "", fmt.Errorf("cannot use both -p and --prompt-file")
	}
	if promptFlag != "" {
		return promptFlag, nil
	}
	if promptFileFlag != "" {
		data, err := os.ReadFile(promptFileFlag)
		if err != nil {
			return "", fmt.Errorf("read prompt file: %w", err)
		}
		return string(data), nil
	}
	return "", nil
}

func resolveModel(modelID string) (llm.Model, error) {
	if modelID == "" {
		modelID = os.Getenv("ANTHROPIC_MODEL")
	}
	if modelID == "" {
		modelID = "claude-sonnet-4-5"
	}

	// Determine API and key from environment
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	openaiKey := os.Getenv("OPENAI_API_KEY")

	// Auto-detect API based on model name
	api := llm.APIAnthropicMessages
	apiKey := anthropicKey
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")

	if strings.HasPrefix(modelID, "gpt-") || strings.HasPrefix(modelID, "o1") || strings.HasPrefix(modelID, "o3") || strings.HasPrefix(modelID, "o4") {
		api = llm.APIOpenAICompletions
		apiKey = openaiKey
		baseURL = os.Getenv("OPENAI_BASE_URL")
	}

	if apiKey == "" {
		return llm.Model{}, fmt.Errorf("no API key: set ANTHROPIC_API_KEY or OPENAI_API_KEY")
	}

	return llm.Model{
		ID:      modelID,
		API:     api,
		BaseURL: baseURL,
		APIKey:  apiKey,
	}, nil
}

func extractFinalText(messages []llm.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if msg, ok := messages[i].(llm.AssistantMessage); ok {
			var parts []string
			for _, block := range msg.Content {
				if tc, ok := block.(llm.TextContent); ok {
					parts = append(parts, tc.Text)
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "\n")
			}
		}
	}
	return ""
}
