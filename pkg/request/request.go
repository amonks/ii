package request

import (
	"fmt"
	"io"
	"net/http"
)

// Error checks the given http response for an error code, and, if one is
// present, reads the body and returns a friendly error.
func Error(resp *http.Response) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		bs, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("http status code %d; error decoding body: %w", resp.StatusCode, err)
		} else {
			return fmt.Errorf("http status code %d: %s", resp.StatusCode, string(bs))
		}
	}
	return nil
}
