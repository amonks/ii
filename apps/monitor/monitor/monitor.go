package monitor

import (
	"fmt"
	"io"
	"net/http"
	"regexp"

	"monks.co/pkg/request"
)

type Monitor interface {
	Check() error
}

type HTTPMonitor struct {
	url    string
	checks []func(*http.Response) error
}

func NewHTTPMonitor(url string, options ...func(*HTTPMonitor)) *HTTPMonitor {
	mon := &HTTPMonitor{url: url}
	for _, opt := range options {
		opt(mon)
	}
	return mon
}

func WithRegexpCheck(regexps ...regexp.Regexp) func(*HTTPMonitor) {
	return func(mon *HTTPMonitor) {
		mon.checks = append(mon.checks, func(resp *http.Response) error {
			defer resp.Body.Close()
			bs, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("error reading response: %w", err)
			}
			for _, regexp := range regexps {
				if !regexp.Match(bs) {
					return fmt.Errorf("regexp match not found in response")
				}
			}
			return nil
		})
	}
}

func (mon *HTTPMonitor) Check() error {
	resp, err := http.Get(mon.url)
	if err != nil {
		return fmt.Errorf("error requesting '%s'", mon.url)
	}
	if err := request.Error(resp); err != nil {
		return fmt.Errorf("error response from '%s': %w", mon.url, err)
	}
	return nil
}
