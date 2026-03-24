package main

import (
	"fmt"

	"monks.co/ii/todo"
)

// printDepTree prints a dependency tree with ASCII art.
func printDepTree(node *todo.DepTreeNode, prefix string, isLast bool, highlight func(string) string) {
	// Print current node
	connector := "├── "
	if isLast {
		connector = "└── "
	}
	if prefix == "" {
		connector = ""
	}

	statusIcon := statusIcon(node.Todo.Status)

	fmt.Printf("%s%s%s %s (%s)\n",
		prefix, connector, statusIcon, node.Todo.Title, highlight(node.Todo.ID))

	// Print children
	childPrefix := prefix
	if prefix != "" {
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	for i, child := range node.Children {
		isLastChild := i == len(node.Children)-1
		printDepTree(child, childPrefix, isLastChild, highlight)
	}
}

// statusIcon returns an icon for the status.
func statusIcon(s todo.Status) string {
	switch s {
	case todo.StatusOpen:
		return "[ ]"
	case todo.StatusQueued:
		return "[q]"
	case todo.StatusInProgress:
		return "[~]"
	case todo.StatusClosed:
		return "[x]"
	case todo.StatusDone:
		return "[d]"
	case todo.StatusTombstone:
		return "[-]"
	default:
		return "[?]"
	}
}
