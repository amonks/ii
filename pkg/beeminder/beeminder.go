package beeminder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type addDatapointRequest struct {
	Value   int64  `json:"value"`
	Comment string `json:"comment,omitempty"`
}

type Datapoint struct {
	User    string
	Goal    string
	Value   int64
	Comment string
}

func Insert(dp Datapoint) error {
	const token = "xFzXNx4dpS5EqTA54BdP"
	url := fmt.Sprintf("https://www.beeminder.com/api/v1/users/%s/goals/%s/datapoints.json?auth_token=%s", dp.User, dp.Goal, token)

	json, err := json.Marshal(addDatapointRequest{
		Value:   dp.Value,
		Comment: dp.Comment,
	})
	if err != nil {
		return err
	}

	body := bytes.NewReader(json)

	resp, err := http.Post(url, "application/json", body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		buf := new(strings.Builder)
		_, err := io.Copy(buf, resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("beeminder error [%d]: %s", resp.StatusCode, buf.String())
	}

	return nil
}
