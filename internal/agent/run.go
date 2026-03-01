package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/llm"
)

type streamWithRetryFunc func(ctx context.Context, model llm.Model, req llm.Request, opts llm.StreamOptions, config llm.RetryConfig) (*llm.StreamHandle, error)

var streamWithRetry streamWithRetryFunc = llm.StreamWithRetry

func setStreamWithRetry(fn streamWithRetryFunc) func() {
	prev := streamWithRetry
	streamWithRetry = fn
	return func() {
		streamWithRetry = prev
	}
}

func isRetryableStreamError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "unexpected eof")
}

// Run starts an agent run with the given prompt and configuration.
// It returns a RunHandle that provides access to events and the final result.
func Run(ctx context.Context, prompt PromptContent, config AgentConfig) (*RunHandle, error) {
	if config.CacheRetention == "" {
		config.CacheRetention = llm.CacheShort
	}
	workDir := config.WorkDir
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	// Create channels
	events := make(chan Event, 100)
	result := make(chan RunResult, 1)

	handle := &RunHandle{
		Events: events,
		result: result,
	}

	content, err := promptContextFromRepo(workDir, config.GlobalConfigDir)
	if err != nil {
		return nil, err
	}
	if len(prompt.ProjectContext) == 0 {
		prompt.ProjectContext = content.ProjectContext
	}
	if len(prompt.ContextFiles) == 0 {
		prompt.ContextFiles = content.ContextFiles
	}
	if len(prompt.TestCommands) == 0 {
		prompt.TestCommands = content.TestCommands
	}
	if prompt.PhaseContent == "" {
		prompt.PhaseContent = content.PhaseContent
	}

	// Start the agent loop in a goroutine
	go runAgent(ctx, prompt, config, workDir, events, result)

	return handle, nil
}

func runAgent(ctx context.Context, prompt PromptContent, config AgentConfig, workDir string, events chan<- Event, result chan<- RunResult) {
	defer close(events)
	defer close(result)

	// Send start event
	events <- AgentStartEvent{Config: config}

	// Initialize conversation with user message
	userContent := prompt.UserContent
	messages := []llm.Message{}
	if strings.TrimSpace(userContent) != "" {
		messages = append(messages, llm.UserMessage{
			Role: "user",
			Content: []llm.ContentBlock{
				llm.TextContent{Type: "text", Text: userContent},
			},
			Timestamp: time.Now(),
		})
	}

	// Create tool executor with config (enables task tool for spawning subagents)
	executor := &toolExecutor{
		workDir:     workDir,
		permissions: config.Permissions,
		env:         config.Env,
		config:      &config,
	}

	// Track aggregate usage
	var totalUsage llm.Usage

	turnIndex := 0
	for {
		// Check for context cancellation
		if ctx.Err() != nil {
			result <- RunResult{
				Messages: messages,
				Usage:    totalUsage,
				Error:    ctx.Err(),
			}
			return
		}

		// Send turn start event
		events <- TurnStartEvent{TurnIndex: turnIndex}

		// Build request (parent agents have the task tool)
		req := llm.Request{
			System:   BuildSystemBlocks(workDir, prompt),
			Messages: messages,
			Tools:    builtInToolsWithTask(true),
		}

		streamErrRetries := 0
		var assistantMsg llm.AssistantMessage
		for {
			// Send message start event (we'll update it as we receive deltas)
			started := false

			streamHandle, err := streamWithRetry(ctx, config.Model, req, llm.StreamOptions{
				CacheRetention: config.CacheRetention,
				SessionID:      config.SessionID,
				UserAgent:      UserAgent(workDir, config.Version),
			}, llm.DefaultRetryConfig())
			if err != nil {
				if isRetryableStreamError(err) && streamErrRetries < 2 {
					streamErrRetries++
					continue
				}
				result <- RunResult{
					Messages: messages,
					Usage:    totalUsage,
					Error:    fmt.Errorf("start stream: %w", err),
				}
				return
			}

			// Process stream events
			var streamErr error

			for streamEvent := range streamHandle.Events {
				switch e := streamEvent.(type) {
				case llm.StartEvent:
					if !started {
						events <- MessageStartEvent{
							TurnIndex: turnIndex,
							Partial:   e.Partial,
						}
						started = true
					}

				case llm.TextDeltaEvent:
					events <- MessageUpdateEvent{
						TurnIndex:   turnIndex,
						StreamEvent: e,
						Partial:     e.Partial,
					}

				case llm.ThinkingDeltaEvent:
					events <- MessageUpdateEvent{
						TurnIndex:   turnIndex,
						StreamEvent: e,
						Partial:     e.Partial,
					}

				case llm.ToolCallDeltaEvent:
					events <- MessageUpdateEvent{
						TurnIndex:   turnIndex,
						StreamEvent: e,
						Partial:     e.Partial,
					}

				case llm.ToolCallEndEvent:
					events <- MessageUpdateEvent{
						TurnIndex:   turnIndex,
						StreamEvent: e,
						Partial:     e.Partial,
					}

				case llm.DoneEvent:
					assistantMsg = e.Message

				case llm.ErrorEvent:
					assistantMsg = e.Message
				}
			}

			// Wait for stream completion and get final message
			finalMsg, err := streamHandle.Wait()
			if err != nil {
				streamErr = err
			} else {
				assistantMsg = finalMsg
			}

			if streamErr != nil {
				if isRetryableStreamError(streamErr) && streamErrRetries < 2 {
					streamErrRetries++
					continue
				}
				result <- RunResult{
					Messages: messages,
					Usage:    totalUsage,
					Error:    fmt.Errorf("stream error: %w", streamErr),
				}
				return
			}

			break
		}

		// Send message end event
		events <- MessageEndEvent{
			TurnIndex: turnIndex,
			Message:   assistantMsg,
		}

		// Update total usage
		totalUsage = addUsage(totalUsage, assistantMsg.Usage)

		// Add assistant message to conversation
		messages = append(messages, assistantMsg)

		// Check stop reason
		switch assistantMsg.StopReason {
		case llm.StopReasonMaxTokens:
			result <- RunResult{
				Messages: messages,
				Usage:    totalUsage,
				Error:    fmt.Errorf("context overflow: max tokens reached"),
			}
			return

		case llm.StopReasonError:
			result <- RunResult{
				Messages: messages,
				Usage:    totalUsage,
				Error:    fmt.Errorf("LLM error: %s", assistantMsg.ErrorMessage),
			}
			return

		case llm.StopReasonAborted:
			result <- RunResult{
				Messages: messages,
				Usage:    totalUsage,
				Error:    ctx.Err(),
			}
			return

		case llm.StopReasonEnd:
			// Natural completion - no tool calls
			events <- TurnEndEvent{
				TurnIndex:   turnIndex,
				Message:     assistantMsg,
				ToolResults: nil,
			}
			if config.InputCh != nil {
				events <- WaitingForInputEvent{TurnIndex: turnIndex}
				for {
					select {
					case input, ok := <-config.InputCh:
						if !ok {
							goto finishRun
						}
						raw := input
						if strings.TrimSpace(raw) == "" {
							continue
						}
						messages = append(messages, llm.UserMessage{
							Role: "user",
							Content: []llm.ContentBlock{
								llm.TextContent{Type: "text", Text: raw},
							},
							Timestamp: time.Now(),
						})
						turnIndex++
						goto continueRun
					case <-ctx.Done():
						result <- RunResult{
							Messages: messages,
							Usage:    totalUsage,
							Error:    ctx.Err(),
						}
						return
					}
				}
			}
		finishRun:
			events <- AgentEndEvent{
				Messages: messages,
				Usage:    totalUsage,
			}
			result <- RunResult{
				Messages: messages,
				Usage:    totalUsage,
			}
			return
		continueRun:
			continue

		case llm.StopReasonToolUse:
			// Execute tool calls
			toolCalls := extractToolCalls(assistantMsg)
			var toolResults []llm.ToolResultMessage

			for _, tc := range toolCalls {
				// Send tool execution start event
				events <- ToolExecutionStartEvent{
					TurnIndex:  turnIndex,
					ToolCallID: tc.ID,
					ToolName:   tc.Name,
					Arguments:  tc.Arguments,
				}

				// Execute the tool
				toolResult := executor.executeTool(ctx, tc)
				toolResults = append(toolResults, toolResult)

				// Send tool execution end event
				events <- ToolExecutionEndEvent{
					TurnIndex:  turnIndex,
					ToolCallID: tc.ID,
					ToolName:   tc.Name,
					Arguments:  tc.Arguments,
					Result:     toolResult,
				}

				// Add tool result to conversation
				messages = append(messages, toolResult)
			}

			// Send turn end event
			events <- TurnEndEvent{
				TurnIndex:   turnIndex,
				Message:     assistantMsg,
				ToolResults: toolResults,
			}

			// Continue to next turn
			turnIndex++
		}
	}
}

// extractToolCalls extracts tool calls from an assistant message.
func extractToolCalls(msg llm.AssistantMessage) []llm.ToolCall {
	var toolCalls []llm.ToolCall
	for _, block := range msg.Content {
		if tc, ok := block.(llm.ToolCall); ok {
			toolCalls = append(toolCalls, tc)
		}
	}
	return toolCalls
}

// runSubagent runs an agent synchronously with a custom set of tools.
// Unlike runAgent, this blocks until completion and returns the result directly.
// It does not emit events (subagent activity is internal to the parent).
func runSubagent(ctx context.Context, prompt PromptContent, config AgentConfig, tools []llm.Tool) (RunResult, error) {
	if config.CacheRetention == "" {
		config.CacheRetention = llm.CacheShort
	}
	workDir := config.WorkDir
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return RunResult{}, fmt.Errorf("get working directory: %w", err)
		}
	}

	content, err := promptContextFromRepo(workDir, config.GlobalConfigDir)
	if err != nil {
		return RunResult{}, err
	}
	if len(prompt.ProjectContext) == 0 {
		prompt.ProjectContext = content.ProjectContext
	}
	if len(prompt.ContextFiles) == 0 {
		prompt.ContextFiles = content.ContextFiles
	}
	if len(prompt.TestCommands) == 0 {
		prompt.TestCommands = content.TestCommands
	}
	if prompt.PhaseContent == "" {
		prompt.PhaseContent = content.PhaseContent
	}

	// Initialize conversation with user message
	userContent := prompt.UserContent
	messages := []llm.Message{}
	if strings.TrimSpace(userContent) != "" {
		messages = append(messages, llm.UserMessage{
			Role: "user",
			Content: []llm.ContentBlock{
				llm.TextContent{Type: "text", Text: userContent},
			},
			Timestamp: time.Now(),
		})
	}

	executor := &toolExecutor{
		workDir:     workDir,
		permissions: config.Permissions,
		env:         config.Env,
		config:      nil, // Prevents task tool from working
	}

	// Track aggregate usage
	var totalUsage llm.Usage

	for {
		// Check for context cancellation
		if ctx.Err() != nil {
			return RunResult{
				Messages: messages,
				Usage:    totalUsage,
				Error:    ctx.Err(),
			}, ctx.Err()
		}

		// Build request with custom tools
		req := llm.Request{
			System:   BuildSystemBlocks(workDir, prompt),
			Messages: messages,
			Tools:    tools,
		}

		streamErrRetries := 0
		var assistantMsg llm.AssistantMessage
		for {
			streamHandle, err := streamWithRetry(ctx, config.Model, req, llm.StreamOptions{
				CacheRetention: config.CacheRetention,
				SessionID:      config.SessionID,
				UserAgent:      UserAgent(workDir, config.Version),
			}, llm.DefaultRetryConfig())
			if err != nil {
				if isRetryableStreamError(err) && streamErrRetries < 2 {
					streamErrRetries++
					continue
				}
				return RunResult{
					Messages: messages,
					Usage:    totalUsage,
					Error:    fmt.Errorf("start stream: %w", err),
				}, err
			}

			// Drain stream events (we don't emit them for subagents)
			for range streamHandle.Events {
			}

			// Wait for stream completion and get final message
			assistantMsg, err = streamHandle.Wait()
			if err != nil {
				if isRetryableStreamError(err) && streamErrRetries < 2 {
					streamErrRetries++
					continue
				}
				return RunResult{
					Messages: messages,
					Usage:    totalUsage,
					Error:    fmt.Errorf("stream error: %w", err),
				}, err
			}
			break
		}

		// Update total usage
		totalUsage = addUsage(totalUsage, assistantMsg.Usage)

		// Add assistant message to conversation
		messages = append(messages, assistantMsg)

		// Check stop reason
		switch assistantMsg.StopReason {
		case llm.StopReasonMaxTokens:
			err := fmt.Errorf("context overflow: max tokens reached")
			return RunResult{
				Messages: messages,
				Usage:    totalUsage,
				Error:    err,
			}, err

		case llm.StopReasonError:
			err := fmt.Errorf("LLM error: %s", assistantMsg.ErrorMessage)
			return RunResult{
				Messages: messages,
				Usage:    totalUsage,
				Error:    err,
			}, err

		case llm.StopReasonAborted:
			return RunResult{
				Messages: messages,
				Usage:    totalUsage,
				Error:    ctx.Err(),
			}, ctx.Err()

		case llm.StopReasonEnd:
			// Natural completion - no tool calls
			return RunResult{
				Messages: messages,
				Usage:    totalUsage,
			}, nil

		case llm.StopReasonToolUse:
			// Execute tool calls
			toolCalls := extractToolCalls(assistantMsg)
			for _, tc := range toolCalls {
				toolResult := executor.executeTool(ctx, tc)
				messages = append(messages, toolResult)
			}
			// Continue to next turn
		}
	}
}

// addUsage adds two usage values together.
func addUsage(a, b llm.Usage) llm.Usage {
	return llm.Usage{
		Input:      a.Input + b.Input,
		Output:     a.Output + b.Output,
		CacheRead:  a.CacheRead + b.CacheRead,
		CacheWrite: a.CacheWrite + b.CacheWrite,
		Total:      a.Total + b.Total,
		Cost: llm.UsageCost{
			Input:      a.Cost.Input + b.Cost.Input,
			Output:     a.Cost.Output + b.Cost.Output,
			CacheRead:  a.Cost.CacheRead + b.Cost.CacheRead,
			CacheWrite: a.Cost.CacheWrite + b.Cost.CacheWrite,
			Total:      a.Cost.Total + b.Cost.Total,
		},
	}
}

// EventToSSE converts a typed event to SSE format.
// Sensitive data (like API keys) is redacted.
func EventToSSE(event Event) SSEEvent {
	var name string
	var data any

	switch e := event.(type) {
	case AgentStartEvent:
		name = "agent.start"
		// Redact API key from config before serialization
		redactedConfig := e.Config
		redactedConfig.Model.APIKey = redactAPIKey(redactedConfig.Model.APIKey)
		data = AgentStartEvent{Config: redactedConfig}
	case AgentEndEvent:
		name = "agent.end"
		// Omit Messages to avoid duplicating the full conversation history
		// already captured by individual turn.end / message.end events.
		data = struct {
			Usage llm.Usage `json:"Usage"`
		}{Usage: e.Usage}
	case TurnStartEvent:
		name = "turn.start"
		data = e
	case TurnEndEvent:
		name = "turn.end"
		data = e
	case MessageStartEvent:
		name = "message.start"
		data = e
	case MessageUpdateEvent:
		name = "message.update"
		data = e
	case MessageEndEvent:
		name = "message.end"
		data = e
	case ToolExecutionStartEvent:
		name = "tool.start"
		data = e
	case ToolExecutionEndEvent:
		name = "tool.end"
		data = e
	case WaitingForInputEvent:
		name = "agent.waiting_for_input"
		data = e
	default:
		name = "unknown"
		data = event
	}

	jsonData, _ := json.Marshal(data)

	return SSEEvent{
		ID:   "", // Caller can set this
		Name: name,
		Data: string(jsonData),
	}
}

// redactAPIKey replaces an API key with a redacted placeholder.
func redactAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "[REDACTED]"
	}
	// Show first 4 and last 4 characters
	return key[:4] + "..." + key[len(key)-4:] + " [REDACTED]"
}
