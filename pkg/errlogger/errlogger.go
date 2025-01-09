package errlogger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"monks.co/pkg/meta"
	"monks.co/pkg/request"
)

type ErrorReport struct {
	App        string `json:"app"`
	Machine    string `json:"machine"`
	StatusCode int    `json:"status_code"`

	HappenedAt time.Time `json:"happened_at"`
	Report     string    `json:"report"`
}

func ReportPanic(err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	if err := sendReport(&ErrorReport{
		HappenedAt: time.Now(),
		App:        meta.AppName(),
		StatusCode: -1,
		Report:     err.Error(),
	}); err != nil {
		log.Println(fmt.Errorf("error-reporting error: %w", err))
	}
}

func ReportError(err error) {
	if err := sendReport(&ErrorReport{
		HappenedAt: time.Now(),
		App:        meta.AppName(),
		StatusCode: 500,
		Report:     err.Error(),
	}); err != nil {
		log.Println(fmt.Errorf("error-reporting error: %w", err))
	}
}

func Report(statusCode int, report string) {
	if err := sendReport(&ErrorReport{
		HappenedAt: time.Now(),
		App:        meta.AppName(),
		Machine:    meta.MachineName(),
		StatusCode: statusCode,
		Report:     report,
	}); err != nil {
		log.Println(fmt.Errorf("error-reporting error: %w", err))
	}
}

func sendReport(report *ErrorReport) error {
	const url = "http://fly.ss.cx/errlog/"

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
