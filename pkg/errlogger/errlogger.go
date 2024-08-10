package errlogger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"monks.co/pkg/request"
)

type AppReporter struct {
	App string
}

func (rep *AppReporter) ReportError(string, err error) {
	ReportError(rep.App, err)
}

func (rep *AppReporter) Report(statusCode int, report string) {
	Report(statusCode, rep.App, report)
}

type ErrorReport struct {
	App        string `json:"app"`
	StatusCode int    `json:"status_code"`

	HappenedAt time.Time `json:"happened_at"`
	Report     string    `json:"report"`
}

func ReportError(app string, err error) {
	if err := sendReport(&ErrorReport{
		App:        app,
		StatusCode: 500,

		HappenedAt: time.Now(),
	}); err != nil {
		log.Println(fmt.Errorf("error-reporting error: %w", err))
	}
}

func Report(statusCode int, app, report string) {
	if err := sendReport(&ErrorReport{
		App:        app,
		StatusCode: statusCode,

		HappenedAt: time.Now(),
		Report:     report,
	}); err != nil {
		log.Println(fmt.Errorf("error-reporting error: %w", err))
	}
}

func sendReport(report *ErrorReport) error {
	const url = "http://fly.ss.cx/errlog"

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(report); err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return fmt.Errorf("errlog request error: %w", err)
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("errlog network error: %w", err)
	}
	if err := request.Error(resp); err != nil {
		return fmt.Errorf("bad response from errlog: %w", err)
	}
	return nil
}
