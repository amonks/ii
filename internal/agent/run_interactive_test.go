package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/amonks/incrementum/internal/llm"
)

func newFakeStreamHandle(msg llm.AssistantMessage) *llm.StreamHandle {
	events := make(chan llm.StreamEvent)
	done := make(chan llm.AssistantMessage, 1)
	errCh := make(chan error, 1)
	close(events)
	done <- msg
	return llm.NewStreamHandle(events, done, errCh)
}

func TestRunAgent_InteractiveInputContinues(t *testing.T) {
	inputCh := make(chan string, 2)
	inputCh <- "   \t"
	inputCh <- "  next step  "
	close(inputCh)

	callCount := 0
	restore := setStreamWithRetry(func(ctx context.Context, model llm.Model, req llm.Request, opts llm.StreamOptions, config llm.RetryConfig) (*llm.StreamHandle, error) {
		callCount++
		if callCount == 1 {
			return newFakeStreamHandle(llm.AssistantMessage{
				Role:       "assistant",
				StopReason: llm.StopReasonEnd,
				Timestamp:  time.Now(),
			}), nil
		}
		if callCount == 2 {
			if len(req.Messages) < 2 {
				return nil, errors.New("missing follow-up user message")
			}
			if userMsg, ok := req.Messages[len(req.Messages)-1].(llm.UserMessage); ok {
				if len(userMsg.Content) == 0 {
					return nil, errors.New("empty follow-up content")
				}
				text, ok := userMsg.Content[0].(llm.TextContent)
				if !ok || text.Text != "  next step  " {
					return nil, errors.New("unexpected follow-up content")
				}
			} else {
				return nil, errors.New("expected follow-up user message")
			}
			return newFakeStreamHandle(llm.AssistantMessage{
				Role:       "assistant",
				StopReason: llm.StopReasonEnd,
				Timestamp:  time.Now(),
			}), nil
		}
		return nil, errors.New("unexpected stream call")
	})
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	handle, err := Run(ctx, "first prompt", AgentConfig{InputCh: inputCh})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	for range handle.Events {
	}

	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 stream calls, got %d", callCount)
	}
	if len(result.Messages) < 3 {
		t.Fatalf("expected at least 3 messages, got %d", len(result.Messages))
	}
}

func TestRunAgent_InteractiveInputClosedEnds(t *testing.T) {
	inputCh := make(chan string)
	close(inputCh)

	callCount := 0
	restore := setStreamWithRetry(func(ctx context.Context, model llm.Model, req llm.Request, opts llm.StreamOptions, config llm.RetryConfig) (*llm.StreamHandle, error) {
		callCount++
		return newFakeStreamHandle(llm.AssistantMessage{
			Role:       "assistant",
			StopReason: llm.StopReasonEnd,
			Timestamp:  time.Now(),
		}), nil
	})
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	handle, err := Run(ctx, "first prompt", AgentConfig{InputCh: inputCh})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	for range handle.Events {
	}

	_, err = handle.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 stream call, got %d", callCount)
	}
}
