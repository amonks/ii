package main

import (
	"monks.co/pkg/twilio"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	return twilio.SMSMe("test message")
}
