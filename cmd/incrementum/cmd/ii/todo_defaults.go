package main

import (
	"github.com/amonks/incrementum/internal/todoenv"
	"github.com/amonks/incrementum/todo"
)

func defaultTodoStatus() todo.Status {
	return todoenv.DefaultStatus()
}
