package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/amonks/incrementum/internal/llm"
)

// Run starts an agent run with the given prompt and configuration.
// It returns a RunHandle that provides access to events and the final result.
func Run(ctx context.Context, prompt string, config AgentConfig) (*RunHandle, error) {
	// Resolve working directory
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

	// Start the agent loop in a goroutine
	go runAgent(ctx, prompt, config, workDir, events, result)

	return handle, nil
}

func runAgent(ctx context.Context, prompt string, config AgentConfig, workDir string, events chan<- Event, result chan<- RunResult) {
	defer close(events)
	defer close(result)

	// Send start event
	events <- AgentStartEvent{Config: config}

	// Initialize conversation with user message
	prelude, err := agentsPrelude(workDir)
	if err != nil {
		result <- RunResult{Error: err}
		return
	}

	messages := []llm.Message{
		llm.UserMessage{
			Role: "user",
			Content: []llm.ContentBlock{
				llm.TextContent{Type: "text", Text: prelude + prompt},
			},
			Timestamp: time.Now(),
		},
	}

	// Create tool executor
	executor := &toolExecutor{
		workDir:     workDir,
		permissions: config.Permissions,
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

		// Build request
		req := llm.Request{
			SystemPrompt: BuildSystemPrompt(workDir),
			Messages:     messages,
			Tools:        builtInTools(),
		}

		// Stream completion from LLM with retry for transient errors
		streamHandle, err := llm.StreamWithRetry(ctx, config.Model, req, llm.StreamOptions{
			SessionID: config.SessionID,
			UserAgent: UserAgent(workDir, config.Version),
		}, llm.DefaultRetryConfig())
		if err != nil {
			result <- RunResult{
				Messages: messages,
				Usage:    totalUsage,
				Error:    fmt.Errorf("start stream: %w", err),
			}
			return
		}

		// Process stream events
		var assistantMsg llm.AssistantMessage
		var streamErr error

		// Send message start event (we'll update it as we receive deltas)
		started := false

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
			result <- RunResult{
				Messages: messages,
				Usage:    totalUsage,
				Error:    fmt.Errorf("stream error: %w", streamErr),
			}
			return
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
			events <- AgentEndEvent{
				Messages: messages,
				Usage:    totalUsage,
			}
			result <- RunResult{
				Messages: messages,
				Usage:    totalUsage,
			}
			return

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
		data = e
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
