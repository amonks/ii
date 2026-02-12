package email

import "monks.co/pkg/requireenv"

var (
	smtpUsername = requireenv.Require("SMTP_USERNAME")
	smtpPassword = requireenv.Require("SMTP_PASSWORD")
)
