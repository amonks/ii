# SMS

## Overview

Minimal internal service that sends SMS messages to the site owner's phone
via the Twilio API. Other tailnet services POST to it to trigger alerts.

Code: [apps/sms/](../apps/sms/)

## Routes

| Method | Path | Description |
|--------|------|-------------|
| POST | `/` | Send SMS. Query param: `message` (required). Returns 400 if missing. |

## Twilio Integration

Delegates to `pkg/twilio`, which wraps the Twilio Go SDK. `SMSMe(msg)` calls
`client.Api.CreateMessage` with hardcoded To/From phone numbers from env
vars: `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_PHONE_NUMBER_FROM`,
`TWILIO_PHONE_NUMBER_ME`.

## Data Storage

None. Fully stateless.

## Deployment

Runs on **brigid** (local server). Access tier: `tag:service` (only other
services can call it).
