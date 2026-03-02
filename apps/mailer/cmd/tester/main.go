package main

import (
	"monks.co/pkg/email"
	"monks.co/pkg/emailclient"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	return emailclient.EmailMe(email.Message{
		Subject: "test",
		Body:    "Just testing.",
	})
}
