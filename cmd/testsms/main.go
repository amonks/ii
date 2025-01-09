package main

import (
	"monks.co/pkg/errlogger"
	"monks.co/pkg/twilio"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	return twilio.SMSMe("test message")
}
