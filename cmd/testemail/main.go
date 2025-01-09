package main

import (
	"monks.co/pkg/email"
	"monks.co/pkg/emailclient"
	"monks.co/pkg/errlogger"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	return emailclient.EmailMe(email.Message{
		Subject: "test",
		Body:    "Just testing.",
	})
}
