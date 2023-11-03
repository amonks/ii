package email

import (
	"errors"
	"net/smtp"
	"os"
	"regexp"
	"strings"
)

type Message struct {
	To      string
	Subject string
	Body    string
}

func EmailMe(message Message) error {
	return SendEmail("monks.co <no-reply@mail.ss.cx>", Message{
		To:      "Andrew Monks <a@monks.co>",
		Subject: message.Subject,
		Body:    message.Body,
	})
}

func SendEmail(from string, message Message) error {
	return smtp.SendMail(
		"email-smtp.us-east-1.amazonaws.com:25",
		&loginAuth{os.Getenv("SMTP_USERNAME"), os.Getenv("SMTP_PASSWORD")},
		extractAddress(from),
		[]string{extractAddress(message.To)},
		[]byte(strings.Join([]string{
			"From: " + from,
			"To: " + message.To,
			"Subject: " + message.Subject,
			"",
			message.Body,
		}, "\r\n")),
	)
}

var emailRegexp = regexp.MustCompile(`^.* <(?P<addr>.*)>$`)

func extractAddress(s string) string {
	if !emailRegexp.MatchString(s) {
		return s
	}
	matches := emailRegexp.FindStringSubmatch(s)
	idx := emailRegexp.SubexpIndex("addr")
	return matches[idx]
}

type loginAuth struct {
	username string
	password string
}

func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, errors.New("Unkown fromServer")
		}
	}
	return nil, nil
}
