package main

import (
	"fmt"

	"github.com/amonks/incrementum/todo"
)

// printTodoDetail prints detailed information about a todo.
func printTodoDetail(t todo.Todo, highlight func(string) string) {
	fmt.Printf("ID:       %s\n", highlight(t.ID))
	fmt.Printf("Title:    %s\n", t.Title)
	fmt.Printf("Type:     %s\n", t.Type)
	fmt.Printf("Status:   %s\n", t.Status)
	fmt.Printf("Priority: %s (%d)\n", todo.PriorityName(t.Priority), t.Priority)
	if t.ImplementationModel != "" {
		fmt.Printf("Implementation Model: %s\n", t.ImplementationModel)
	}
	if t.CodeReviewModel != "" {
		fmt.Printf("Code Review Model: %s\n", t.CodeReviewModel)
	}
	if t.ProjectReviewModel != "" {
		fmt.Printf("Project Review Model: %s\n", t.ProjectReviewModel)
	}
	fmt.Printf("Created:  %s\n", t.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:  %s\n", t.UpdatedAt.Format("2006-01-02 15:04:05"))

	if t.ClosedAt != nil {
		fmt.Printf("Closed:   %s\n", t.ClosedAt.Format("2006-01-02 15:04:05"))
	}

	if t.DeletedAt != nil {
		fmt.Printf("Deleted:  %s\n", t.DeletedAt.Format("2006-01-02 15:04:05"))
	}

	if t.DeleteReason != "" {
		fmt.Printf("Delete Reason: %s\n", t.DeleteReason)
	}

	if t.Description != "" {
		fmt.Printf("\nDescription:\n%s\n", formatTodoDescription(t.Description))
	}
}

const todoDetailLineWidth = 80

func formatTodoDescription(value string) string {
	return renderMarkdownWithoutMargin(value, todoDetailLineWidth)
}
