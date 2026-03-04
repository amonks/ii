package job

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	internalstrings "github.com/amonks/incrementum/internal/strings"
)

// agentEventInterpreter renders agent events from the agent package.
// Agent events use a Name field with values like "agent.start", "tool.start", etc.
type agentEventInterpreter struct {
	repoPath string
}

func newAgentEventInterpreter(repoPath string) *agentEventInterpreter {
	if !internalstrings.IsBlank(repoPath) {
		repoPath = filepath.Clean(repoPath)
	}
	return &agentEventInterpreter{
		repoPath: repoPath,
	}
}

// IsAgentEvent returns true if the event appears to be from the agent package.
func IsAgentEvent(event Event) bool {
	// Agent events have a Name field with specific prefixes
	switch event.Name {
	case "agent.start", "agent.end",
		"turn.start", "turn.end",
		"message.start", "message.update", "message.end",
		"tool.start", "tool.end":
		return true
	}
	return false
}

// agentRenderedEvent represents a formatted agent event.
type agentRenderedEvent struct {
	Kind   string
	Label  string
	Body   string
	Inline string
}

// Handle processes an agent event and returns rendered output.
func (i *agentEventInterpreter) Handle(event Event) ([]agentRenderedEvent, error) {
	if internalstrings.IsBlank(event.Name) || internalstrings.IsBlank(event.Data) {
		return nil, nil
	}

	switch event.Name {
	case "agent.start":
		return i.handleAgentStart(event.Data)
	case "agent.end":
		return i.handleAgentEnd(event.Data)
	case "turn.start":
		return i.handleTurnStart(event.Data)
	case "turn.end":
		return nil, nil // Don't render turn.end separately
	case "message.start":
		return nil, nil // Will accumulate deltas until message.end
	case "message.update":
		return nil, nil // Will accumulate deltas until message.end
	case "message.end":
		return i.handleMessageEnd(event.Data)
	case "tool.start":
		return i.handleToolStart(event.Data)
	case "tool.end":
		return i.handleToolEnd(event.Data)
	default:
		return nil, nil
	}
}

// agentStartData is the JSON structure for agent.start events.
type agentStartData struct {
	Config struct {
		Model struct {
			ID string `json:"ID"`
		} `json:"Model"`
		WorkDir string `json:"WorkDir"`
	} `json:"Config"`
}

func (i *agentEventInterpreter) handleAgentStart(data string) ([]agentRenderedEvent, error) {
	var payload agentStartData
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return nil, nil // Silently skip malformed events
	}

	parts := []string{}
	if payload.Config.Model.ID != "" {
		parts = append(parts, "model "+payload.Config.Model.ID)
	}
	if payload.Config.WorkDir != "" {
		parts = append(parts, "workdir "+i.relativePathForLog(payload.Config.WorkDir))
	}

	inline := ""
	if len(parts) > 0 {
		inline = strings.Join(parts, ", ")
	}

	return []agentRenderedEvent{{
		Kind:   "tool",
		Inline: fmt.Sprintf("Agent started (%s)", inline),
	}}, nil
}

// agentEndData is the JSON structure for agent.end events.
type agentEndData struct {
	Usage struct {
		Input  int `json:"Input"`
		Output int `json:"Output"`
		Total  int `json:"Total"`
		Cost   struct {
			Total float64 `json:"Total"`
		} `json:"Cost"`
	} `json:"Usage"`
}

func (i *agentEventInterpreter) handleAgentEnd(data string) ([]agentRenderedEvent, error) {
	var payload agentEndData
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return nil, nil
	}

	summary := fmt.Sprintf("Agent finished (tokens: %d, cost: $%.4f)",
		payload.Usage.Total, payload.Usage.Cost.Total)

	return []agentRenderedEvent{{
		Kind:   "tool",
		Inline: summary,
	}}, nil
}

func (i *agentEventInterpreter) handleTurnStart(data string) ([]agentRenderedEvent, error) {
	return nil, nil // Don't render turn start
}

// messageEndData is the JSON structure for message.end events.
type messageEndData struct {
	TurnIndex int `json:"TurnIndex"`
	Message   struct {
		Role    string `json:"Role"`
		Content []struct {
			Type     string `json:"Type"`
			Text     string `json:"Text"`
			Thinking string `json:"Thinking"`
		} `json:"Content"`
		StopReason string `json:"StopReason"`
	} `json:"Message"`
}

func (i *agentEventInterpreter) handleMessageEnd(data string) ([]agentRenderedEvent, error) {
	var payload messageEndData
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return nil, nil
	}

	var results []agentRenderedEvent

	// Extract thinking content
	var thinking []string
	for _, block := range payload.Message.Content {
		if block.Type == "thinking" && block.Thinking != "" {
			thinking = append(thinking, block.Thinking)
		}
	}
	if len(thinking) > 0 {
		results = append(results, agentRenderedEvent{
			Kind:  "thinking",
			Label: "Agent thinking:",
			Body:  strings.Join(thinking, "\n\n"),
		})
	}

	// Extract text content
	var text []string
	for _, block := range payload.Message.Content {
		if block.Type == "text" && block.Text != "" {
			text = append(text, block.Text)
		}
	}
	if len(text) > 0 {
		results = append(results, agentRenderedEvent{
			Kind:  "response",
			Label: "Agent response:",
			Body:  strings.Join(text, "\n\n"),
		})
	}

	return results, nil
}

// toolStartData is the JSON structure for tool.start events.
type toolStartData struct {
	TurnIndex  int            `json:"TurnIndex"`
	ToolCallID string         `json:"ToolCallID"`
	ToolName   string         `json:"ToolName"`
	Arguments  map[string]any `json:"Arguments"`
}

func (i *agentEventInterpreter) handleToolStart(data string) ([]agentRenderedEvent, error) {
	var payload toolStartData
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return nil, nil
	}

	summary := i.summarizeToolCall(payload.ToolName, payload.Arguments)
	if summary == "" {
		summary = i.summarizeToolName(payload.ToolName)
	}
	if summary == "" {
		return nil, nil
	}

	return []agentRenderedEvent{{
		Kind:   "tool",
		Inline: fmt.Sprintf("Tool start: %s", summary),
	}}, nil
}

// toolEndData is the JSON structure for tool.end events.
type toolEndData struct {
	TurnIndex  int            `json:"TurnIndex"`
	ToolCallID string         `json:"ToolCallID"`
	ToolName   string         `json:"ToolName"`
	Arguments  map[string]any `json:"Arguments"`
	Result     struct {
		IsError bool `json:"IsError"`
		Content []struct {
			Type string `json:"Type"`
			Text string `json:"Text"`
		} `json:"Content"`
	} `json:"Result"`
}

func (i *agentEventInterpreter) handleToolEnd(data string) ([]agentRenderedEvent, error) {
	var payload toolEndData
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return nil, nil
	}

	summary := i.summarizeToolCall(payload.ToolName, payload.Arguments)
	if summary == "" {
		summary = i.summarizeToolName(payload.ToolName)
	}
	if summary == "" {
		return nil, nil
	}

	status := "completed"
	if payload.Result.IsError {
		status = "failed"
	}

	statusSuffix := ""
	if status == "failed" {
		statusSuffix = " (failed)"
	}

	return []agentRenderedEvent{{
		Kind:   "tool",
		Inline: fmt.Sprintf("Tool end: %s%s", summary, statusSuffix),
	}}, nil
}

func (i *agentEventInterpreter) summarizeToolCall(tool string, args map[string]any) string {
	switch tool {
	case "bash":
		if cmd, ok := args["command"].(string); ok && cmd != "" {
			cmd = internalstrings.TrimSpace(cmd)
			return fmt.Sprintf("bash '%s'", cmd)
		}
		return "" // No command yet; caller will fall back to summarizeToolName
	case "read":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("read file '%s'", i.relativePathForLog(path))
		}
		return "read file"
	case "write":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("write file '%s'", i.relativePathForLog(path))
		}
		return "write file"
	case "edit":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("edit file '%s'", i.relativePathForLog(path))
		}
		return "edit file"
	case "glob":
		pattern, _ := args["pattern"].(string)
		path, _ := args["path"].(string)
		if pattern != "" && path != "" {
			return fmt.Sprintf("glob '%s' in '%s'", pattern, i.relativePathForLog(path))
		}
		if pattern != "" {
			return fmt.Sprintf("glob '%s'", pattern)
		}
		return "glob"
	case "grep":
		pattern, _ := args["pattern"].(string)
		path, _ := args["path"].(string)
		if pattern != "" && path != "" {
			return fmt.Sprintf("grep '%s' in '%s'", pattern, i.relativePathForLog(path))
		}
		if pattern != "" {
			return fmt.Sprintf("grep '%s'", pattern)
		}
		return "grep"
	default:
		return tool
	}
}

func (i *agentEventInterpreter) summarizeToolName(tool string) string {
	switch tool {
	case "bash":
		return "bash"
	case "read":
		return "read file"
	case "write":
		return "write file"
	case "edit":
		return "edit file"
	case "glob":
		return "glob"
	case "grep":
		return "grep"
	default:
		return tool
	}
}

func (i *agentEventInterpreter) relativePathForLog(path string) string {
	if i.repoPath == "" || path == "" {
		return path
	}
	if rel, err := filepath.Rel(i.repoPath, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}
