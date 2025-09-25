package credentials

import (
	"fmt"
	"os"
)

var (
	LastFmAPIKey = require("LASTFM_API_KEY")

	PlacesBackendAPIKey = require("GOOGLE_PLACES_BACKEND_API_KEY")
	PlacesBrowserAPIKey = require("GOOGLE_PLACES_BROWSER_API_KEY")

	SMTPPassword = require("SMTP_PASSWORD")
	SMTPUsername = require("SMTP_USERNAME")

	TailscaleAuthKey      = require("TS_AUTHKEY")
	TwilioAccountSID      = require("TWILIO_ACCOUNT_SID")
	TwilioAuthToken       = require("TWILIO_AUTH_TOKEN")
	TwilioPhoneNumberFrom = require("TWILIO_PHONE_NUMBER_FROM")
	TwilioPhoneNumberMe   = require("TWILIO_PHONE_NUMBER_ME")
)

func require(env string) string {
	got := os.Getenv(env)
	if got == "" {
		panic(fmt.Errorf("env '%s' not set", env))
	}
	return got
}
