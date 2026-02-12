package emailclient

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"monks.co/pkg/email"
	"monks.co/pkg/tailnet"
)

func EmailMe(message email.Message) error {
	form := url.Values{}
	form.Set("subject", message.Subject)
	form.Set("body", message.Body)

	const url = "http://monks-mailer-fly-ord/"

	req, err := http.NewRequest("POST", url, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}

	req.Header.Add("Content-type", "application/x-www-form-urlencoded")

	hc := tailnet.Client()
	res, err := hc.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if bs, err := io.ReadAll(res.Body); err != nil {
			return fmt.Errorf("mailer error %s", res.Status)
		} else {
			return fmt.Errorf("mailer error %d: %s", res.StatusCode, string(bs))
		}
	}

	return nil
}
