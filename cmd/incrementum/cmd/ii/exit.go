package main

import "fmt"

type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return fmt.Sprintf("exit %d", e.code)
}

func (e exitError) ExitCode() int {
	return e.code
}

func (e exitError) Unwrap() error {
	return e.err
}
