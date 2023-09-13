package letterboxd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Film struct {
	ID               string               `json:"id"`
	Name             string               `json:"name"`
	OriginalName     string               `json:"originalName"`
	SortingName      string               `json:"sortingName"`
	AlternativeNames []string             `json:"alternativeNames"`
	ReleaseYear      string               `json:"releaseYear"`
	Directors        []ContributorSummary `json:"directors"`
	Genres           []Genre              `json:"genres"`
	Tagline          string               `json:"tagline"`
	Description      string               `json:"description"`
	RunTime          int64                `json:"runTime"`
	OriginalLanguage Language             `json:"originalLanguage"`
	Languages        []Language           `json:"languages"`
}

type ContributorSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Genre struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type AccessTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Issuer       string `json:"issuer"`
}

type accessToken struct {
	token        string
	refreshToken string
	expiresAt    time.Time
}

type Client struct {
	httpClient  *http.Client
	accessToken *accessToken
}

func New() *Client {
	return &Client{
		httpClient: &http.Client{},
	}
}

const apiPrefix = `https://api.letterboxd.com/api/v0/`

func (l *Client) SignIn(username, password string) error {
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", "username")
	data.Set("password", "password")

	req, err := http.NewRequest(http.MethodPost, apiPrefix+"auth/token", strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := l.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error hitting letterboxd token endpoint: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("error code from letterboxd token endpoint: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error reading response from letterboxd token endpoint: %w", err)
	}

	var accessTokenResponse AccessTokenResponse
	json.Unmarshal(body, &accessTokenResponse)

	l.accessToken = &accessToken{
		token: accessTokenResponse.AccessToken,
		refreshToken: accessTokenResponse.RefreshToken,
		expiresAt: time.Now().Add(time.Duration(accessTokenResponse.ExpiresIn) * time.Second),
	}

	return nil
}

func (l *Client) Search(q string) ([]Film, error) {
	if l.accessToken == nil {
		return nil, errors.New("must sign in first")
	}

	data := url.Values{}
	data.Set("input", q)

	req, err := http.NewRequest(http.MethodPost, apiPrefix+"search", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error hitting letterboxd token endpoint: %w", err)
	}

	fmt.Println(res)

	return nil, nil
}
