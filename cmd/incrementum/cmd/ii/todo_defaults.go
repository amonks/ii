package main

import (
	"monks.co/incrementum/internal/todoenv"
	"monks.co/incrementum/todo"
)

func defaultTodoStatus() todo.Status {
	return todoenv.DefaultStatus()
}
