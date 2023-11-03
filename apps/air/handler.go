package main

import (
	"monks.co/pkg/email"
	"monks.co/pkg/emailclient"
)

func handleChange(old, new *Parameters) error {
	if old.WaterLevel == 3 && new.WaterLevel == 1 {
		emailclient.EmailMe(email.Message{
			Subject: "low water",
			Body:    "Venta water level is getting low.",
		})
	}
	return nil
}
