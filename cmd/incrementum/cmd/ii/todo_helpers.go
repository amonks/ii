package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/todo"
	"github.com/spf13/cobra"
)

func openTodoStoreWithOptions(cmd *cobra.Command, args []string, opts todo.OpenOptions) (*todo.Store, error) {
	repoPath, err := getRepoPath()
	if err != nil {
		return nil, err
	}

	opts.Purpose = todoStorePurpose(cmd, args)
	return todo.Open(repoPath, opts)
}

func openTodoStore(cmd *cobra.Command, args []string) (*todo.Store, error) {
	return openTodoStoreWithOptions(cmd, args, todo.OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  true,
	})
}

func openTodoStoreReadOnly(cmd *cobra.Command, args []string) (*todo.Store, error) {
	return openTodoStoreWithOptions(cmd, args, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		ReadOnly:        true,
	})
}

func openTodoStoreReadOnlyOrEmpty(cmd *cobra.Command, args []string, jsonOutput bool, emptyMessage func() error) (*todo.Store, bool, error) {
	store, err := openTodoStoreReadOnly(cmd, args)
	if err != nil {
		if errors.Is(err, todo.ErrNoTodoStore) {
			if jsonOutput {
				return nil, true, encodeJSONToStdout([]todo.Todo{})
			}
			if emptyMessage == nil {
				return nil, true, nil
			}
			return nil, true, emptyMessage()
		}
		return nil, false, err
	}
	return store, false, nil
}

func todoStorePurpose(cmd *cobra.Command, args []string) string {
	parts := []string{cmd.CommandPath()}
	parts = append(parts, args...)
	value := strings.Join(parts, " ")
	value = internalstrings.NormalizeWhitespace(value)
	if value == "" {
		return "todo store"
	}
	return fmt.Sprintf("todo store (%s)", value)
}

func resolveDescriptionFromStdin(description string, reader io.Reader) (string, error) {
	if description != "-" {
		return description, nil
	}

	input, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read description from stdin: %w", err)
	}

	value := internalstrings.TrimTrailingNewlines(string(input))
	return value, nil
}

func resolveDescriptionFlag(cmd *cobra.Command, description *string, reader io.Reader) error {
	if !cmd.Flags().Changed("description") {
		return nil
	}
	resolved, err := resolveDescriptionFromStdin(*description, reader)
	if err != nil {
		return err
	}
	*description = resolved
	return nil
}

func hasTodoCreateFlags(cmd *cobra.Command) bool {
	return hasChangedFlags(cmd, "title", "type", "priority", "description", "implementation-model", "code-review-model", "project-review-model", "deps")
}
