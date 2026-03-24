package main

import (
	"monks.co/ii/internal/todoenv"
	"monks.co/ii/todo"
)

func defaultTodoStatus() todo.Status {
	return todoenv.DefaultStatus()
}
