package llm

import (
	"testing"
	"time"
)

func TestConvertMessagesToAnthropic_MergesToolResults(t *testing.T) {
	// Simulate a conversation with parallel tool calls:
	// User asks -> Assistant makes 2 tool calls -> Both results come back
	messages := []Message{
		UserMessage{
			Role: "user",
			Content: []ContentBlock{
				TextContent{Type: "text", Text: "Please read two files"},
			},
			Timestamp: time.Now(),
		},
		AssistantMessage{
			Role: "assistant",
			Content: []ContentBlock{
				TextContent{Type: "text", Text: "I'll read both files."},
				ToolCall{Type: "toolCall", ID: "tool_1", Name: "read", Arguments: map[string]any{"path": "/a.txt"}},
				ToolCall{Type: "toolCall", ID: "tool_2", Name: "read", Arguments: map[string]any{"path": "/b.txt"}},
			},
			Timestamp: time.Now(),
		},
		ToolResultMessage{
			Role:       "toolResult",
			ToolCallID: "tool_1",
			ToolName:   "read",
			Content:    []ContentBlock{TextContent{Type: "text", Text: "content of a.txt"}},
			Timestamp:  time.Now(),
		},
		ToolResultMessage{
			Role:       "toolResult",
			ToolCallID: "tool_2",
			ToolName:   "read",
			Content:    []ContentBlock{TextContent{Type: "text", Text: "content of b.txt"}},
			Timestamp:  time.Now(),
		},
	}

	result := convertMessagesToAnthropic(messages)

	// Should have 3 messages: user, assistant, user (with merged tool results)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}

	// First message: user
	if result[0].Role != "user" {
		t.Errorf("expected first message role 'user', got %q", result[0].Role)
	}

	// Second message: assistant with text and tool calls
	if result[1].Role != "assistant" {
		t.Errorf("expected second message role 'assistant', got %q", result[1].Role)
	}
	if len(result[1].Content) != 3 {
		t.Errorf("expected 3 content blocks in assistant message, got %d", len(result[1].Content))
	}

	// Third message: user with BOTH tool results merged
	if result[2].Role != "user" {
		t.Errorf("expected third message role 'user', got %q", result[2].Role)
	}
	if len(result[2].Content) != 2 {
		t.Fatalf("expected 2 tool_result blocks in merged message, got %d", len(result[2].Content))
	}

	// Verify both tool results are present
	if result[2].Content[0].Type != "tool_result" {
		t.Errorf("expected first content type 'tool_result', got %q", result[2].Content[0].Type)
	}
	if result[2].Content[0].ToolUseID != "tool_1" {
		t.Errorf("expected first tool_use_id 'tool_1', got %q", result[2].Content[0].ToolUseID)
	}
	if result[2].Content[1].Type != "tool_result" {
		t.Errorf("expected second content type 'tool_result', got %q", result[2].Content[1].Type)
	}
	if result[2].Content[1].ToolUseID != "tool_2" {
		t.Errorf("expected second tool_use_id 'tool_2', got %q", result[2].Content[1].ToolUseID)
	}
}

func TestConvertMessagesToAnthropic_ExcludesThinkingBlocks(t *testing.T) {
	messages := []Message{
		AssistantMessage{
			Role: "assistant",
			Content: []ContentBlock{
				ThinkingContent{Type: "thinking", Thinking: "Let me think about this..."},
				TextContent{Type: "text", Text: "Here's my answer."},
			},
			Timestamp: time.Now(),
		},
	}

	result := convertMessagesToAnthropic(messages)

	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}

	// Should only have the text content, not the thinking content
	if len(result[0].Content) != 1 {
		t.Fatalf("expected 1 content block (thinking excluded), got %d", len(result[0].Content))
	}

	if result[0].Content[0].Type != "text" {
		t.Errorf("expected content type 'text', got %q", result[0].Content[0].Type)
	}
	if result[0].Content[0].Text != "Here's my answer." {
		t.Errorf("expected text 'Here's my answer.', got %q", result[0].Content[0].Text)
	}
}

func TestConvertMessagesToAnthropic_ToolResultsWithInterleaved(t *testing.T) {
	// Test that tool results separated by other messages are NOT merged
	messages := []Message{
		UserMessage{
			Role:      "user",
			Content:   []ContentBlock{TextContent{Type: "text", Text: "first question"}},
			Timestamp: time.Now(),
		},
		AssistantMessage{
			Role: "assistant",
			Content: []ContentBlock{
				ToolCall{Type: "toolCall", ID: "tool_1", Name: "read", Arguments: map[string]any{}},
			},
			Timestamp: time.Now(),
		},
		ToolResultMessage{
			Role:       "toolResult",
			ToolCallID: "tool_1",
			Content:    []ContentBlock{TextContent{Type: "text", Text: "result 1"}},
			Timestamp:  time.Now(),
		},
		AssistantMessage{
			Role: "assistant",
			Content: []ContentBlock{
				TextContent{Type: "text", Text: "Got it. Let me do another."},
				ToolCall{Type: "toolCall", ID: "tool_2", Name: "read", Arguments: map[string]any{}},
			},
			Timestamp: time.Now(),
		},
		ToolResultMessage{
			Role:       "toolResult",
			ToolCallID: "tool_2",
			Content:    []ContentBlock{TextContent{Type: "text", Text: "result 2"}},
			Timestamp:  time.Now(),
		},
	}

	result := convertMessagesToAnthropic(messages)

	// Should have 5 messages: user, assistant, user(tool1), assistant, user(tool2)
	if len(result) != 5 {
		t.Fatalf("expected 5 messages (tool results not merged due to interleaving), got %d", len(result))
	}

	// Verify the tool results are in separate messages
	if result[2].Content[0].ToolUseID != "tool_1" {
		t.Errorf("expected tool_use_id 'tool_1' in message 3")
	}
	if result[4].Content[0].ToolUseID != "tool_2" {
		t.Errorf("expected tool_use_id 'tool_2' in message 5")
	}
}

func TestConvertMessagesToAnthropic_EmptyAssistantMessageExcluded(t *testing.T) {
	// If an assistant message only had thinking content, it should be excluded
	messages := []Message{
		UserMessage{
			Role:      "user",
			Content:   []ContentBlock{TextContent{Type: "text", Text: "question"}},
			Timestamp: time.Now(),
		},
		AssistantMessage{
			Role: "assistant",
			Content: []ContentBlock{
				ThinkingContent{Type: "thinking", Thinking: "thinking only..."},
			},
			Timestamp: time.Now(),
		},
	}

	result := convertMessagesToAnthropic(messages)

	// Should only have the user message since assistant message becomes empty
	if len(result) != 1 {
		t.Fatalf("expected 1 message (empty assistant excluded), got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("expected role 'user', got %q", result[0].Role)
	}
}
