package main

import "monks.co/pkg/email"

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	return email.EmailMe(email.Message{
		Subject: "test",
		Body:    "Just testing.",
	})
}
