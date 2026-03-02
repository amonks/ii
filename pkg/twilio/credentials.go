package twilio

import "monks.co/pkg/requireenv"

var (
	twilioAccountSID      = requireenv.Lazy("TWILIO_ACCOUNT_SID")
	twilioAuthToken       = requireenv.Lazy("TWILIO_AUTH_TOKEN")
	twilioPhoneNumberFrom = requireenv.Lazy("TWILIO_PHONE_NUMBER_FROM")
	twilioPhoneNumberMe   = requireenv.Lazy("TWILIO_PHONE_NUMBER_ME")
)
