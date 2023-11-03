package emailclient

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"monks.co/pkg/email"
)

func EmailMe(message email.Message) error {
	form := url.Values{}
	form.Set("subject", message.Subject)
	form.Set("body", message.Body)

	const url = "http://go.ss.cx/mailer"

	req, err := http.NewRequest("POST", url, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}

	hc := http.Client{}
	res, err := hc.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("mailer error %d", res.StatusCode)
	}

	return nil
}
