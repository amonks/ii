package mastodon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Mastodon struct {
	httpClient http.Client

	clientId          string
	clientSecret      string
	clientAccessToken string

	userAuthorizationCode string
	userAccessToken       string
}

func NewMastodon() *Mastodon {
	return &Mastodon{}
}

func (m *Mastodon) Request(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, "https://mastodon.social/api/v1/"+path, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request to '%s': %w", path, err)
	}

	req.Header.Set("Authorization", "Bearer "+m.userAccessToken)
	res, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", path, err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", path, err)
	}
	return body, nil
}

func (m *Mastodon) GetHomeTimeline() (Post, error) {
	res, err := m.Request("timelines/home")
	if err != nil {
		return Post{}, fmt.Errorf("error getting home timeline: %w", err)
	}

	var post Post
	err = json.Unmarshal(res, &post)
	if err != nil {
		return post, fmt.Errorf("error unmarshalling home timeline: %w", err)
	}

	return post, nil
}
