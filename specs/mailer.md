# Mailer

## Overview

Internal microservice that sends email to the site owner (`a@monks.co`) on
behalf of other apps. Accessible only over the tailnet.

Code: [apps/mailer/](../apps/mailer/)

## Routes

| Method | Path | Description |
|--------|------|-------------|
| POST | `/` | Send an email. Form fields: `subject` (required), `body` (required). Returns 400 if missing, 500 on failure. |

## Email Sending

Delegates to `pkg/email`:
- Sender: `monks.co <no-reply@mail.ss.cx>`
- Recipient: `Andrew Monks <a@monks.co>`
- SMTP: `email-smtp.us-east-1.amazonaws.com:25` (AWS SES)
- Auth: custom `LOGIN` mechanism using `SMTP_USERNAME` / `SMTP_PASSWORD`
  env vars.

## Client Package

`pkg/emailclient` provides `EmailMe(message)` which POSTs to
`http://monks-mailer-fly-ord/` via the tailnet HTTP client. Apps use this
instead of sending SMTP directly.

## Deployment

Runs on **fly.io** (Chicago ORD). `shared-cpu-1x`, 256 MB. Stateless.
