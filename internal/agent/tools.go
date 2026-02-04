package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/llm"
)

// formatValidationError creates a detailed error message for missing/invalid tool arguments.
func formatValidationError(toolName string, paramName string, expectedType string, args map[string]any) string {
	argsJSON, _ := json.Marshal(args)
	return fmt.Sprintf("Validation error for tool %q:\n  - %s (%s): parameter is required\n\nReceived arguments:\n%s",
		toolName, paramName, expectedType, string(argsJSON))
}

// Tool parameter types for JSON schema generation

// BashParams contains parameters for the bash tool.
type BashParams struct {
	// Command is the command to execute.
	Command string `json:"command" jsonschema:"description=The command to execute"`

	// Timeout is the timeout in seconds (default 120).
	Timeout *int `json:"timeout,omitempty" jsonschema:"description=Timeout in seconds (default 120),optional"`
}

// ReadParams contains parameters for the read tool.
type ReadParams struct {
	// Path is the absolute path to the file to read.
	Path string `json:"path" jsonschema:"description=Absolute path to the file to read"`

	// Offset is the line offset (0-based).
	Offset *int `json:"offset,omitempty" jsonschema:"description=Line offset (0-based),optional"`

	// Limit is the number of lines to read (default 2000).
	Limit *int `json:"limit,omitempty" jsonschema:"description=Number of lines to read (default 2000),optional"`
}

// WriteParams contains parameters for the write tool.
type WriteParams struct {
	// Path is the absolute path to the file to write.
	Path string `json:"path" jsonschema:"description=Absolute path to the file to write"`

	// Content is the content to write to the file.
	Content string `json:"content" jsonschema:"description=Content to write to the file"`
}

// EditParams contains parameters for the edit tool.
type EditParams struct {
	// Path is the absolute path to the file to edit.
	Path string `json:"path" jsonschema:"description=Absolute path to the file to edit"`

	// OldString is the text to find and replace.
	OldString string `json:"old_string" jsonschema:"description=Text to find and replace"`

	// NewString is the replacement text.
	NewString string `json:"new_string" jsonschema:"description=Replacement text"`

	// ReplaceAll replaces all occurrences if true (default false).
	ReplaceAll *bool `json:"replace_all,omitempty" jsonschema:"description=Replace all occurrences (default false),optional"`
}

// builtInTools returns the list of built-in agent tools.
func builtInTools() []llm.Tool {
	return []llm.Tool{
		{
			Name:        "bash",
			Description: "Execute a shell command in the working directory. Commands are checked against permission rules before execution.",
			Parameters:  BashParams{},
		},
		{
			Name:        "read",
			Description: "Read the contents of a file. Returns the file content with line numbers. Handles binary files, missing files, and permission errors gracefully.",
			Parameters:  ReadParams{},
		},
		{
			Name:        "write",
			Description: "Write content to a file. Creates parent directories as needed.",
			Parameters:  WriteParams{},
		},
		{
			Name:        "edit",
			Description: "Perform text replacement in a file. Returns an error if old_string is not found or found multiple times (when replace_all is false).",
			Parameters:  EditParams{},
		},
	}
}

// toolExecutor executes tools and returns results.
type toolExecutor struct {
	workDir     string
	permissions BashPermissions
	env         []string
}

// executeTool executes a tool call and returns the result.
func (e *toolExecutor) executeTool(ctx context.Context, toolCall llm.ToolCall) llm.ToolResultMessage {
	var content string
	var isError bool

	switch toolCall.Name {
	case "bash":
		content, isError = e.executeBash(ctx, toolCall.Arguments)
	case "read":
		content, isError = e.executeRead(toolCall.Arguments)
	case "write":
		content, isError = e.executeWrite(toolCall.Arguments)
	case "edit":
		content, isError = e.executeEdit(toolCall.Arguments)
	default:
		content = fmt.Sprintf("Unknown tool: %s", toolCall.Name)
		isError = true
	}

	return llm.ToolResultMessage{
		Role:       "toolResult",
		ToolCallID: toolCall.ID,
		ToolName:   toolCall.Name,
		Content: []llm.ContentBlock{
			llm.TextContent{
				Type: "text",
				Text: content,
			},
		},
		IsError:   isError,
		Timestamp: time.Now(),
	}
}

func (e *toolExecutor) executeBash(ctx context.Context, args map[string]any) (string, bool) {
	command, ok := args["command"].(string)
	if !ok || command == "" {
		return formatValidationError("bash", "command", "string", args), true
	}

	// Check permissions
	if !e.permissions.IsAllowed(command) {
		return fmt.Sprintf("Permission denied: command %q is not allowed by bash permissions", command), true
	}

	timeout := 120
	if t, ok := args["timeout"]; ok {
		switch v := t.(type) {
		case float64:
			timeout = int(v)
		case int:
			timeout = v
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Execute command
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = e.workDir
	if len(e.env) > 0 {
		cmd.Env = append(os.Environ(), e.env...)
	}

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Build output
	var result strings.Builder
	if stdout.Len() > 0 {
		result.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString(stderr.String())
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Sprintf("Command timed out after %d seconds\n%s", timeout, result.String()), true
		}
		if result.Len() > 0 {
			return result.String(), true
		}
		return fmt.Sprintf("Command failed: %v", err), true
	}

	output := result.String()
	if output == "" {
		output = "(command completed successfully with no output)"
	}
	return output, false
}

func (e *toolExecutor) executeRead(args map[string]any) (string, bool) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return formatValidationError("read", "path", "string", args), true
	}

	// Resolve path
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.workDir, path)
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("File not found: %s", path), true
		}
		if os.IsPermission(err) {
			return fmt.Sprintf("Permission denied: %s", path), true
		}
		return fmt.Sprintf("Error reading file: %v", err), true
	}

	// Check for binary content
	if isBinary(data) {
		return fmt.Sprintf("File appears to be binary: %s", path), true
	}

	// Parse offset and limit
	offset := 0
	if o, ok := args["offset"]; ok {
		switch v := o.(type) {
		case float64:
			offset = int(v)
		case int:
			offset = v
		}
	}

	limit := 2000
	if l, ok := args["limit"]; ok {
		switch v := l.(type) {
		case float64:
			limit = int(v)
		case int:
			limit = v
		}
	}

	// Format with line numbers
	lines := strings.Split(string(data), "\n")
	if offset >= len(lines) {
		return fmt.Sprintf("Offset %d is past end of file (%d lines)", offset, len(lines)), true
	}

	end := offset + limit
	if end > len(lines) {
		end = len(lines)
	}

	var result strings.Builder
	for i := offset; i < end; i++ {
		line := lines[i]
		// Truncate lines longer than 2000 characters
		if len(line) > 2000 {
			line = line[:2000] + "..."
		}
		fmt.Fprintf(&result, "%6d\t%s\n", i+1, line)
	}

	return result.String(), false
}

func (e *toolExecutor) executeWrite(args map[string]any) (string, bool) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return formatValidationError("write", "path", "string", args), true
	}

	content, ok := args["content"].(string)
	if !ok {
		return formatValidationError("write", "content", "string", args), true
	}

	// Resolve path
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.workDir, path)
	}

	// Create parent directories
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Sprintf("Error creating directory: %v", err), true
	}

	// Write file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		if os.IsPermission(err) {
			return fmt.Sprintf("Permission denied: %s", path), true
		}
		return fmt.Sprintf("Error writing file: %v", err), true
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), false
}

func (e *toolExecutor) executeEdit(args map[string]any) (string, bool) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return formatValidationError("edit", "path", "string", args), true
	}

	oldString, ok := args["old_string"].(string)
	if !ok {
		return formatValidationError("edit", "old_string", "string", args), true
	}

	newString, ok := args["new_string"].(string)
	if !ok {
		return formatValidationError("edit", "new_string", "string", args), true
	}

	replaceAll := false
	if r, ok := args["replace_all"]; ok {
		switch v := r.(type) {
		case bool:
			replaceAll = v
		}
	}

	// Resolve path
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.workDir, path)
	}

	// Read existing content
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("File not found: %s", path), true
		}
		if os.IsPermission(err) {
			return fmt.Sprintf("Permission denied: %s", path), true
		}
		return fmt.Sprintf("Error reading file: %v", err), true
	}

	content := string(data)

	// Count occurrences
	count := strings.Count(content, oldString)
	if count == 0 {
		return fmt.Sprintf("old_string not found in %s", path), true
	}

	if count > 1 && !replaceAll {
		return fmt.Sprintf("old_string found %d times in %s; use replace_all=true to replace all occurrences or provide more context to match uniquely", count, path), true
	}

	// Perform replacement
	var newContent string
	if replaceAll {
		newContent = strings.ReplaceAll(content, oldString, newString)
	} else {
		newContent = strings.Replace(content, oldString, newString, 1)
	}

	// Write back
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		if os.IsPermission(err) {
			return fmt.Sprintf("Permission denied: %s", path), true
		}
		return fmt.Sprintf("Error writing file: %v", err), true
	}

	if replaceAll {
		return fmt.Sprintf("Replaced %d occurrences in %s", count, path), false
	}
	return fmt.Sprintf("Successfully edited %s", path), false
}

// isBinary checks if data appears to be binary content.
func isBinary(data []byte) bool {
	// Check for null bytes in the first 8000 bytes
	checkLen := len(data)
	if checkLen > 8000 {
		checkLen = 8000
	}

	for i := 0; i < checkLen; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// IsAllowed checks if a command is allowed by the permission rules.
// Rules are evaluated in order; first match wins. Default is deny.
func (p BashPermissions) IsAllowed(command string) bool {
	for _, rule := range p.Rules {
		if matchPattern(rule.Pattern, command) {
			return rule.Allow
		}
	}
	// Default deny
	return false
}

// matchPattern matches a command against a glob pattern.
// The pattern supports:
//   - * matches any sequence of characters
//   - ? matches any single character
func matchPattern(pattern, command string) bool {
	// Use filepath.Match for glob matching
	// However, filepath.Match doesn't handle arbitrary command strings well
	// because it treats / specially. Use a custom implementation.
	return globMatch(pattern, command)
}

// globMatch performs glob-style pattern matching.
func globMatch(pattern, str string) bool {
	// Handle empty pattern
	if pattern == "" {
		return str == ""
	}

	// Handle wildcard-only pattern
	if pattern == "*" {
		return true
	}

	pi := 0 // pattern index
	si := 0 // string index
	lastWildcard := -1
	lastMatch := -1

	for si < len(str) {
		if pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == str[si]) {
			// Match single character or ?
			pi++
			si++
		} else if pi < len(pattern) && pattern[pi] == '*' {
			// Record position for backtracking
			lastWildcard = pi
			lastMatch = si
			pi++
		} else if lastWildcard >= 0 {
			// Backtrack: try matching one more character with *
			pi = lastWildcard + 1
			lastMatch++
			si = lastMatch
		} else {
			return false
		}
	}

	// Check remaining pattern characters (should all be *)
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}

	return pi == len(pattern)
}
