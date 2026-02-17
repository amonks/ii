package llm

import (
	"encoding/json"
	"fmt"
	"time"
)

// completionJSON is the JSON representation of a Completion.
// It uses concrete types with discriminator fields for proper serialization.
type completionJSON struct {
	ID        string                 `json:"id"`
	Model     string                 `json:"model"`
	Request   completionRequestJSON  `json:"request"`
	Response  completionResponseJSON `json:"response"`
	CreatedAt time.Time              `json:"created_at"`
}

type completionRequestJSON struct {
	SystemPrompt string        `json:"system_prompt,omitempty"`
	Messages     []messageJSON `json:"messages"`
	Tools        []toolJSON    `json:"tools,omitempty"`
}

type messageJSON struct {
	Role       string             `json:"role"`
	Content    []contentBlockJSON `json:"content,omitempty"`
	Timestamp  time.Time          `json:"timestamp"`
	ToolCallID string             `json:"tool_call_id,omitempty"`
	ToolName   string             `json:"tool_name,omitempty"`
	IsError    bool               `json:"is_error,omitempty"`
}

type contentBlockJSON struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	Thinking  string         `json:"thinking,omitempty"`
	Data      string         `json:"data,omitempty"`
	MimeType  string         `json:"mime_type,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type toolJSON struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Parameters is omitted since it's a Go struct type that can't be serialized directly
}

type completionResponseJSON struct {
	Role         string             `json:"role"`
	Content      []contentBlockJSON `json:"content"`
	Model        string             `json:"model"`
	API          string             `json:"api"`
	Provider     string             `json:"provider"`
	Usage        usageJSON          `json:"usage"`
	StopReason   string             `json:"stop_reason"`
	ErrorMessage string             `json:"error_message,omitempty"`
	Timestamp    time.Time          `json:"timestamp"`
}

type usageJSON struct {
	Input      int      `json:"input"`
	Output     int      `json:"output"`
	CacheRead  int      `json:"cache_read"`
	CacheWrite int      `json:"cache_write"`
	Total      int      `json:"total"`
	Cost       costJSON `json:"cost"`
}

type costJSON struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
	Total      float64 `json:"total"`
}

// MarshalJSON implements json.Marshaler for Completion.
func (c Completion) MarshalJSON() ([]byte, error) {
	cj := completionJSON{
		ID:        c.ID,
		Model:     c.Model,
		CreatedAt: c.CreatedAt,
	}

	// Convert Request
	cj.Request.SystemPrompt = c.Request.SystemPrompt
	for _, msg := range c.Request.Messages {
		cj.Request.Messages = append(cj.Request.Messages, messageToJSON(msg))
	}
	for _, tool := range c.Request.Tools {
		cj.Request.Tools = append(cj.Request.Tools, toolJSON{
			Name:        tool.Name,
			Description: tool.Description,
		})
	}

	// Convert Response
	cj.Response = completionResponseJSON{
		Role:         c.Response.Role,
		Model:        c.Response.Model,
		API:          string(c.Response.API),
		Provider:     c.Response.Provider,
		StopReason:   string(c.Response.StopReason),
		ErrorMessage: c.Response.ErrorMessage,
		Timestamp:    c.Response.Timestamp,
		Usage: usageJSON{
			Input:      c.Response.Usage.Input,
			Output:     c.Response.Usage.Output,
			CacheRead:  c.Response.Usage.CacheRead,
			CacheWrite: c.Response.Usage.CacheWrite,
			Total:      c.Response.Usage.Total,
			Cost: costJSON{
				Input:      c.Response.Usage.Cost.Input,
				Output:     c.Response.Usage.Cost.Output,
				CacheRead:  c.Response.Usage.Cost.CacheRead,
				CacheWrite: c.Response.Usage.Cost.CacheWrite,
				Total:      c.Response.Usage.Cost.Total,
			},
		},
	}
	for _, block := range c.Response.Content {
		cj.Response.Content = append(cj.Response.Content, contentBlockToJSON(block))
	}

	return json.Marshal(cj)
}

// UnmarshalJSON implements json.Unmarshaler for Completion.
func (c *Completion) UnmarshalJSON(data []byte) error {
	var cj completionJSON
	if err := json.Unmarshal(data, &cj); err != nil {
		return err
	}

	c.ID = cj.ID
	c.Model = cj.Model
	c.CreatedAt = cj.CreatedAt

	// Convert Request
	c.Request.SystemPrompt = cj.Request.SystemPrompt
	for _, mj := range cj.Request.Messages {
		c.Request.Messages = append(c.Request.Messages, messageFromJSON(mj))
	}
	for _, tj := range cj.Request.Tools {
		c.Request.Tools = append(c.Request.Tools, Tool{
			Name:        tj.Name,
			Description: tj.Description,
		})
	}

	// Convert Response
	c.Response = AssistantMessage{
		Role:         cj.Response.Role,
		Model:        cj.Response.Model,
		API:          API(cj.Response.API),
		Provider:     cj.Response.Provider,
		StopReason:   StopReason(cj.Response.StopReason),
		ErrorMessage: cj.Response.ErrorMessage,
		Timestamp:    cj.Response.Timestamp,
		Usage: Usage{
			Input:      cj.Response.Usage.Input,
			Output:     cj.Response.Usage.Output,
			CacheRead:  cj.Response.Usage.CacheRead,
			CacheWrite: cj.Response.Usage.CacheWrite,
			Total:      cj.Response.Usage.Total,
			Cost: UsageCost{
				Input:      cj.Response.Usage.Cost.Input,
				Output:     cj.Response.Usage.Cost.Output,
				CacheRead:  cj.Response.Usage.Cost.CacheRead,
				CacheWrite: cj.Response.Usage.Cost.CacheWrite,
				Total:      cj.Response.Usage.Cost.Total,
			},
		},
	}
	for _, bj := range cj.Response.Content {
		c.Response.Content = append(c.Response.Content, contentBlockFromJSON(bj))
	}

	return nil
}

func messageToJSON(msg Message) messageJSON {
	switch m := msg.(type) {
	case UserMessage:
		var content []contentBlockJSON
		for _, block := range m.Content {
			content = append(content, contentBlockToJSON(block))
		}
		return messageJSON{
			Role:      "user",
			Content:   content,
			Timestamp: m.Timestamp,
		}
	case AssistantMessage:
		var content []contentBlockJSON
		for _, block := range m.Content {
			content = append(content, contentBlockToJSON(block))
		}
		return messageJSON{
			Role:      "assistant",
			Content:   content,
			Timestamp: m.Timestamp,
		}
	case ToolResultMessage:
		var content []contentBlockJSON
		for _, block := range m.Content {
			content = append(content, contentBlockToJSON(block))
		}
		return messageJSON{
			Role:       "tool_result",
			Content:    content,
			Timestamp:  m.Timestamp,
			ToolCallID: m.ToolCallID,
			ToolName:   m.ToolName,
			IsError:    m.IsError,
		}
	default:
		return messageJSON{}
	}
}

func messageFromJSON(mj messageJSON) Message {
	var content []ContentBlock
	for _, bj := range mj.Content {
		content = append(content, contentBlockFromJSON(bj))
	}

	switch mj.Role {
	case "user":
		return UserMessage{
			Role:      mj.Role,
			Content:   content,
			Timestamp: mj.Timestamp,
		}
	case "assistant":
		return AssistantMessage{
			Role:      mj.Role,
			Content:   content,
			Timestamp: mj.Timestamp,
		}
	case "tool_result":
		return ToolResultMessage{
			Role:       mj.Role,
			ToolCallID: mj.ToolCallID,
			ToolName:   mj.ToolName,
			Content:    content,
			IsError:    mj.IsError,
			Timestamp:  mj.Timestamp,
		}
	default:
		return nil
	}
}

func contentBlockToJSON(block ContentBlock) contentBlockJSON {
	switch b := block.(type) {
	case TextContent:
		return contentBlockJSON{
			Type: "text",
			Text: b.Text,
		}
	case ThinkingContent:
		return contentBlockJSON{
			Type:     "thinking",
			Thinking: b.Thinking,
		}
	case ImageContent:
		return contentBlockJSON{
			Type:     "image",
			Data:     b.Data,
			MimeType: b.MimeType,
		}
	case ToolCall:
		return contentBlockJSON{
			Type:      "tool_call",
			ID:        b.ID,
			Name:      b.Name,
			Arguments: b.Arguments,
		}
	default:
		return contentBlockJSON{}
	}
}

func contentBlockFromJSON(bj contentBlockJSON) ContentBlock {
	switch bj.Type {
	case "text":
		return TextContent{
			Type: "text",
			Text: bj.Text,
		}
	case "thinking":
		return ThinkingContent{
			Type:     "thinking",
			Thinking: bj.Thinking,
		}
	case "image":
		return ImageContent{
			Type:     "image",
			Data:     bj.Data,
			MimeType: bj.MimeType,
		}
	case "tool_call":
		return ToolCall{
			Type:      "toolCall",
			ID:        bj.ID,
			Name:      bj.Name,
			Arguments: bj.Arguments,
		}
	default:
		// Try to create a text content block for unknown types
		if bj.Text != "" {
			return TextContent{
				Type: "text",
				Text: bj.Text,
			}
		}
		return nil
	}
}

// String returns a human-readable representation of the Completion.
func (c Completion) String() string {
	return fmt.Sprintf("Completion{ID: %s, Model: %s, CreatedAt: %s}", c.ID, c.Model, c.CreatedAt.Format(time.RFC3339))
}
