package job

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	internalstrings "github.com/amonks/incrementum/internal/strings"
)

type opencodeEventPayload struct {
	Type       string          `json:"type"`
	Properties json.RawMessage `json:"properties"`
}

type opencodeMessageUpdated struct {
	Info opencodeMessageInfo `json:"info"`
}

type opencodeMessageInfo struct {
	ID     string `json:"id"`
	Role   string `json:"role"`
	Finish string `json:"finish"`
	Time   struct {
		Completed int64 `json:"completed"`
	} `json:"time"`
}

type opencodeMessagePartUpdated struct {
	Part opencodeMessagePart `json:"part"`
}

type opencodeMessagePart struct {
	ID        string            `json:"id"`
	MessageID string            `json:"messageID"`
	Type      string            `json:"type"`
	Text      string            `json:"text"`
	Tool      string            `json:"tool"`
	State     opencodeToolState `json:"state"`
}

type opencodeToolState struct {
	Status string         `json:"status"`
	Input  map[string]any `json:"input"`
}

type opencodeRenderedEvent struct {
	Kind   string
	Label  string
	Body   string
	Inline string
}

type opencodeMessageState struct {
	textParts       map[string]string
	textOrder       []string
	reasoningParts  map[string]string
	reasoningOrder  []string
	promptEmitted   bool
	responseEmitted bool
	thinkingEmitted bool
}

type opencodeEventInterpreter struct {
	switches     map[string]bool
	messageRoles map[string]string
	messages     map[string]*opencodeMessageState
	repoPath     string
}

func newOpencodeEventInterpreter(switches map[string]bool, repoPath string) *opencodeEventInterpreter {
	resolved := defaultOpencodeEventSwitches()
	if switches != nil {
		for key, value := range switches {
			resolved[key] = value
		}
	}
	if !internalstrings.IsBlank(repoPath) {
		repoPath = filepath.Clean(repoPath)
	}
	return &opencodeEventInterpreter{
		switches:     resolved,
		messageRoles: make(map[string]string),
		messages:     make(map[string]*opencodeMessageState),
		repoPath:     repoPath,
	}
}

func defaultOpencodeEventSwitches() map[string]bool {
	return map[string]bool{
		"server.connected":       false,
		"server.heartbeat":       false,
		"session.created":        false,
		"session.updated":        false,
		"session.status":         false,
		"session.idle":           false,
		"session.diff":           false,
		"message.updated":        false,
		"message.part.updated":   true,
		"file.edited":            false,
		"file.watcher.updated":   false,
		"lsp.updated":            false,
		"lsp.client.diagnostics": false,
		"todo.updated":           false,
	}
}

func (i *opencodeEventInterpreter) Handle(event Event) ([]opencodeRenderedEvent, error) {
	if internalstrings.IsBlank(event.Data) {
		return nil, nil
	}
	payload, err := parseOpencodeEventPayload(event.Data)
	if err != nil || internalstrings.IsBlank(payload.Type) {
		return rawOpencodeEvent(legacyEventLabel(event.Name), event.Data), nil
	}

	switch payload.Type {
	case "message.updated":
		outputs, err := i.handleMessageUpdated(payload.Properties)
		return handleOpencodePayload(payload.Type, event.Data, outputs, err)
	case "message.part.updated":
		outputs, err := i.handleMessagePartUpdated(payload.Properties)
		return handleOpencodePayload(payload.Type, event.Data, outputs, err)
	default:
		if !i.enabled(payload.Type) {
			return nil, nil
		}
		return rawOpencodeEvent(legacyEventLabel(payload.Type), event.Data), nil
	}
}

func handleOpencodePayload(payloadType, data string, outputs []opencodeRenderedEvent, err error) ([]opencodeRenderedEvent, error) {
	if err != nil {
		return rawOpencodeEvent(legacyEventLabel(payloadType), data), nil
	}
	return outputs, nil
}

func rawOpencodeEvent(label, body string) []opencodeRenderedEvent {
	return []opencodeRenderedEvent{{Kind: "raw", Label: label, Body: body}}
}

func (i *opencodeEventInterpreter) enabled(eventType string) bool {
	if i == nil {
		return false
	}
	return i.switches[eventType]
}

func (i *opencodeEventInterpreter) handleMessageUpdated(payload json.RawMessage) ([]opencodeRenderedEvent, error) {
	var update opencodeMessageUpdated
	if err := json.Unmarshal(payload, &update); err != nil {
		return nil, fmt.Errorf("decode opencode message.updated: %w", err)
	}
	info := update.Info
	if internalstrings.IsBlank(info.ID) {
		return nil, nil
	}
	role := internalstrings.NormalizeLowerTrimSpace(info.Role)
	if role != "" {
		i.messageRoles[info.ID] = role
	}

	state := i.ensureMessageState(info.ID)
	if role == "user" {
		if prompt := i.maybeEmitPrompt(state); prompt != nil {
			return prompt, nil
		}
	}

	if role == "assistant" && !state.responseEmitted && messageCompleted(info) && i.enabled("message.part.updated") {
		events := make([]opencodeRenderedEvent, 0, 2)
		thinking := combineParts(state.reasoningOrder, state.reasoningParts)
		if !internalstrings.IsBlank(thinking) && !state.thinkingEmitted {
			state.thinkingEmitted = true
			events = append(events, renderThinking(thinking))
		}
		response := combineParts(state.textOrder, state.textParts)
		if !internalstrings.IsBlank(response) {
			state.responseEmitted = true
			events = append(events, renderResponse(response))
		}
		return events, nil
	}

	return nil, nil
}

func (i *opencodeEventInterpreter) handleMessagePartUpdated(payload json.RawMessage) ([]opencodeRenderedEvent, error) {
	var update opencodeMessagePartUpdated
	if err := json.Unmarshal(payload, &update); err != nil {
		return nil, fmt.Errorf("decode opencode message.part.updated: %w", err)
	}
	part := update.Part
	if internalstrings.IsBlank(part.MessageID) {
		return nil, nil
	}
	state := i.ensureMessageState(part.MessageID)
	partType := internalstrings.NormalizeLowerTrimSpace(part.Type)
	switch partType {
	case "text":
		i.storePartText(state, part.ID, part.Text, false)
		role := i.messageRoles[part.MessageID]
		if role == "user" {
			if prompt := i.maybeEmitPrompt(state); prompt != nil {
				return prompt, nil
			}
		}
	case "reasoning":
		i.storePartText(state, part.ID, part.Text, true)
	case "tool":
		if !i.enabled("message.part.updated") {
			return nil, nil
		}
		summary := i.summarizeToolCall(part.Tool, part.State.Input)
		if internalstrings.IsBlank(summary) {
			return nil, nil
		}
		status := internalstrings.NormalizeLowerTrimSpace(part.State.Status)
		if status == "" {
			return nil, nil
		}
		// For legacy events, just render each tool update with its status.
		// We don't try to infer start/end pairing because status updates
		// from external tools don't reliably arrive in order or at all.
		return []opencodeRenderedEvent{renderToolStatus(summary, status)}, nil
	}
	return nil, nil
}

func (i *opencodeEventInterpreter) ensureMessageState(messageID string) *opencodeMessageState {
	if i.messages[messageID] == nil {
		i.messages[messageID] = &opencodeMessageState{
			textParts:      make(map[string]string),
			reasoningParts: make(map[string]string),
		}
	}
	return i.messages[messageID]
}

func (i *opencodeEventInterpreter) storePartText(state *opencodeMessageState, partID, text string, reasoning bool) {
	if state == nil || internalstrings.IsBlank(partID) {
		return
	}
	if reasoning {
		if _, ok := state.reasoningParts[partID]; !ok {
			state.reasoningOrder = append(state.reasoningOrder, partID)
		}
		state.reasoningParts[partID] = text
		return
	}
	if _, ok := state.textParts[partID]; !ok {
		state.textOrder = append(state.textOrder, partID)
	}
	state.textParts[partID] = text
}

func (i *opencodeEventInterpreter) maybeEmitPrompt(state *opencodeMessageState) []opencodeRenderedEvent {
	if state == nil || state.promptEmitted || !i.enabled("message.part.updated") {
		return nil
	}
	prompt := combineParts(state.textOrder, state.textParts)
	if internalstrings.IsBlank(prompt) {
		return nil
	}
	state.promptEmitted = true
	return []opencodeRenderedEvent{renderPrompt(prompt)}
}

func renderToolStatus(summary, status string) opencodeRenderedEvent {
	// Format as "Tool (<status>): <summary>" to show the current state
	// without trying to infer start/end pairing
	label := "Tool"
	if status != "" {
		label = fmt.Sprintf("Tool (%s)", status)
	}
	return opencodeRenderedEvent{Kind: "tool", Label: label + ":", Inline: summary}
}

func renderPrompt(text string) opencodeRenderedEvent {
	return opencodeRenderedEvent{Kind: "prompt", Label: "LLM prompt:", Body: text}
}

func renderResponse(text string) opencodeRenderedEvent {
	return opencodeRenderedEvent{Kind: "response", Label: "LLM response:", Body: text}
}

func renderThinking(text string) opencodeRenderedEvent {
	return opencodeRenderedEvent{Kind: "thinking", Label: "LLM thinking:", Body: text}
}

func combineParts(order []string, parts map[string]string) string {
	if len(order) == 0 {
		return ""
	}
	merged := make([]string, 0, len(order))
	for _, key := range order {
		text := internalstrings.TrimTrailingNewlines(parts[key])
		if internalstrings.IsBlank(text) {
			continue
		}
		merged = append(merged, text)
	}
	return strings.Join(merged, "\n\n")
}

func parseOpencodeEventPayload(data string) (opencodeEventPayload, error) {
	var payload opencodeEventPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

func messageCompleted(info opencodeMessageInfo) bool {
	return info.Time.Completed != 0 || !internalstrings.IsBlank(info.Finish)
}

func (i *opencodeEventInterpreter) summarizeToolCall(tool string, input map[string]any) string {
	name := internalstrings.NormalizeLowerTrimSpace(tool)
	switch name {
	case "read", "write", "edit":
		if summary := i.fileToolSummary(name, input); summary != "" {
			return summary
		}
	case "apply_patch":
		if files := i.extractPatchFiles(input); len(files) > 0 {
			if len(files) == 1 {
				return fmt.Sprintf("patch file %s", quoteForLog(files[0]))
			}
			return fmt.Sprintf("patch files %s", quoteForLog(strings.Join(files, ", ")))
		}
		return "apply patch"
	case "glob":
		pattern := stringFromMap(input, "pattern")
		path := stringFromMap(input, "path")
		if pattern != "" && path != "" {
			return fmt.Sprintf("glob %s in %s", quoteForLog(pattern), quoteForLog(path))
		}
		if pattern != "" {
			return fmt.Sprintf("glob %s", quoteForLog(pattern))
		}
	case "grep":
		pattern := stringFromMap(input, "pattern")
		include := stringFromMap(input, "include")
		if pattern != "" && include != "" {
			return fmt.Sprintf("search %s in %s", quoteForLog(pattern), quoteForLog(include))
		}
		if pattern != "" {
			return fmt.Sprintf("search %s", quoteForLog(pattern))
		}
	case "bash":
		if command := stringFromMap(input, "command"); command != "" {
			return fmt.Sprintf("run %s", quoteForLog(command))
		}
		// Return empty string to suppress redundant "bash" log when command is empty.
		// The command will arrive in a subsequent event.
		return ""
	case "webfetch":
		if url := stringFromMap(input, "url"); url != "" {
			return fmt.Sprintf("fetch %s", quoteForLog(url))
		}
	case "question":
		if question := firstQuestionText(input); question != "" {
			return fmt.Sprintf("ask %s", quoteForLog(truncateForLog(question)))
		}
	}
	if name != "" {
		return name
	}
	return "tool call"
}

func (i *opencodeEventInterpreter) fileToolSummary(action string, input map[string]any) string {
	if path := stringFromMap(input, "filePath"); path != "" {
		return fmt.Sprintf("%s file %s", action, quoteForLog(i.relativePathForLog(path)))
	}
	return ""
}

func (i *opencodeEventInterpreter) extractPatchFiles(input map[string]any) []string {
	patch := stringFromMap(input, "patch")
	if patch == "" {
		return nil
	}
	seen := make(map[string]bool)
	var files []string
	for _, line := range strings.Split(patch, "\n") {
		// Unified diff format: +++ b/path/to/file or +++ path/to/file
		if strings.HasPrefix(line, "+++ ") {
			path := strings.TrimPrefix(line, "+++ ")
			// Strip a/ or b/ prefix commonly used in git diffs
			path = strings.TrimPrefix(path, "b/")
			path = strings.TrimPrefix(path, "a/")
			// Handle timestamp suffix (e.g., "file.txt	2024-01-01 12:00:00")
			if idx := strings.Index(path, "\t"); idx != -1 {
				path = path[:idx]
			}
			path = internalstrings.TrimSpace(path)
			if path != "" && path != "/dev/null" && !seen[path] {
				seen[path] = true
				files = append(files, i.relativePathForLog(path))
			}
		}
	}
	return files
}

func (i *opencodeEventInterpreter) relativePathForLog(path string) string {
	if internalstrings.IsBlank(path) {
		return path
	}
	if !filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	if internalstrings.IsBlank(i.repoPath) {
		return filepath.Clean(path)
	}
	path = filepath.Clean(path)
	repoPath := filepath.Clean(i.repoPath)
	rel, err := filepath.Rel(repoPath, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return path
	}
	return rel
}

func stringFromMap(input map[string]any, key string) string {
	value, _ := input[key].(string)
	return internalstrings.TrimSpace(value)
}

func firstQuestionText(input map[string]any) string {
	items, _ := input["questions"].([]any)
	if len(items) == 0 {
		return ""
	}
	first, _ := items[0].(map[string]any)
	return stringFromMap(first, "question")
}

func quoteForLog(value string) string {
	if value == "" {
		return "''"
	}
	escaped := strings.ReplaceAll(value, "'", "\\'")
	return "'" + escaped + "'"
}

func truncateForLog(value string) string {
	const maxLen = 160
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen-3] + "..."
}

func legacyEventLabel(eventType string) string {
	if eventType == "" {
		return "LLM event:"
	}
	return fmt.Sprintf("LLM event (%s):", eventType)
}
