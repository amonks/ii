package email

import "monks.co/pkg/requireenv"

var (
	smtpUsername = requireenv.Lazy("SMTP_USERNAME")
	smtpPassword = requireenv.Lazy("SMTP_PASSWORD")
)
