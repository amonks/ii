package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/amonks/incrementum/internal/ui"
	"github.com/amonks/incrementum/todo"
)

// printTodoTable prints todos in a table format.
func printTodoTable(todos []todo.Todo, prefixLengths map[string]int, now time.Time) {
	if len(todos) == 0 {
		fmt.Println("No todos found.")
		return
	}

	fmt.Print(formatTodoTable(todos, prefixLengths, ui.HighlightID, now))
}

func formatTodoTable(todos []todo.Todo, prefixLengths map[string]int, highlight func(string, int) string, now time.Time) string {
	builder := ui.NewTableBuilder([]string{"ID", "PRI", "TYPE", "STATUS", "AGE", "UPDATED", "DURATION", "TITLE"}, len(todos))

	if prefixLengths == nil {
		prefixLengths = todoIDPrefixLengths(todos)
	}

	for _, t := range todos {
		title := ui.TruncateTableCell(t.Title)
		prefixLen := ui.PrefixLength(prefixLengths, t.ID)
		highlighted := highlight(t.ID, prefixLen)
		age := formatTodoAge(t, now)
		updated := formatTodoUpdated(t, now)
		duration := formatTodoDuration(t, now)
		row := []string{
			highlighted,
			priorityShort(t.Priority),
			string(t.Type),
			string(t.Status),
			age,
			updated,
			duration,
			title,
		}
		builder.AddRow(row)
	}

	return builder.String()
}

func todoIDPrefixLengths(todos []todo.Todo) map[string]int {
	index := todo.NewIDIndex(todos)
	return index.PrefixLengths()
}

func formatTodoAge(item todo.Todo, now time.Time) string {
	return formatOptionalDuration(todo.AgeData(item, now))
}

func formatTodoDuration(item todo.Todo, now time.Time) string {
	return formatOptionalDuration(todo.DurationData(item, now))
}

func formatTodoUpdated(item todo.Todo, now time.Time) string {
	return formatOptionalDuration(todo.UpdatedData(item, now))
}

// priorityShort returns a short representation of priority.
func priorityShort(p int) string {
	switch p {
	case 0:
		return "P0"
	case 1:
		return "P1"
	case 2:
		return "P2"
	case 3:
		return "P3"
	case 4:
		return "P4"
	default:
		return "P" + strconv.Itoa(p)
	}
}
