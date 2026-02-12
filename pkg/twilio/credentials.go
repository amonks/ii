package twilio

import "monks.co/pkg/requireenv"

var (
	twilioAccountSID      = requireenv.Require("TWILIO_ACCOUNT_SID")
	twilioAuthToken       = requireenv.Require("TWILIO_AUTH_TOKEN")
	twilioPhoneNumberFrom = requireenv.Require("TWILIO_PHONE_NUMBER_FROM")
	twilioPhoneNumberMe   = requireenv.Require("TWILIO_PHONE_NUMBER_ME")
)
