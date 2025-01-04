package monitor

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"monks.co/pkg/request"
)

type Monitor interface {
	Check() error
	Name() string
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

func (mon *HTTPMonitor) Check() error {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(mon.url)
	if err != nil {
		return fmt.Errorf("error requesting %s: %w", mon.Name(), err)
	}
	for _, check := range mon.checks {
		if err := check(resp); err != nil {
			return fmt.Errorf("monitor %s failed: %w", mon.Name(), err)
		}
	}
	return nil
}

func WithSuccessCheck() func(*HTTPMonitor) {
	return func(mon *HTTPMonitor) {
		mon.checks = append(mon.checks, func(resp *http.Response) error {
			if err := request.Error(resp); err != nil {
				return fmt.Errorf("[%s] error response: %w", mon.Name(), err)
			}
			return nil
		})
	}
}

func WithRedirectCheck(target string) func(*HTTPMonitor) {
	return func(mon *HTTPMonitor) {
		mon.checks = append(mon.checks, func(resp *http.Response) error {
			if resp.StatusCode != 301 {
				return fmt.Errorf("expected 301, got %d", resp.StatusCode)
			}
			if loc := resp.Header.Get("Location"); loc != target {
				return fmt.Errorf("expected redirect to '%s', got '%s'", target, loc)
			}
			return nil
		})
	}
}

func WithBodyCheck(checks ...func(io.Reader) error) func(*HTTPMonitor) {
	return func(mon *HTTPMonitor) {
		mon.checks = append(mon.checks, func(resp *http.Response) error {
			defer resp.Body.Close()
			var body io.Reader = resp.Body
			for _, check := range checks {
				var buf bytes.Buffer
				r := io.TeeReader(body, &buf)
				body = &buf
				if err := check(r); err != nil {
					return fmt.Errorf("check failed against %s: %w", mon.Name(), err)
				}
			}
			return nil
		})
	}
}

func LiteralCheck(strings ...string) func(io.Reader) error {
	regexps := make([]*regexp.Regexp, len(strings))
	for i, s := range strings {
		regexps[i] = regexp.MustCompile(regexp.QuoteMeta(s))
	}
	return RegexpCheck(regexps...)
}

func RegexpCheck(regexps ...*regexp.Regexp) func(io.Reader) error {
	return func(r io.Reader) error {
		bs, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		for _, regexp := range regexps {
			if !regexp.Match(bs) {
				return fmt.Errorf("no match for '%s'", regexp.String())
			}
		}
		return nil
	}
}

func (mon *HTTPMonitor) Name() string {
	return fmt.Sprintf("HTTPMonitor<%s>", mon.url)
}
