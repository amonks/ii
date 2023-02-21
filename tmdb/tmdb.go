package tmdb

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
)

type Movie struct {
	ID            int64      `json:"id"`
	Title         string     `json:"title"`
	OriginalTitle string     `json:"original_title"`
	ReleaseDate   string     `json:"release_date"`
	Genres        []Genre    `json:"genres"`
	Tagline       string     `json:"tagline"`
	Overview      string     `json:"overview"`
	RunTime       int64      `json:"runtime"`
	Languages     []Language `json:"spoken_languages"`
}

type Genre struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Language struct {
	Code string `json:"iso_639_1"`
	Name string `json:"name"`
}

type Client struct {
	httpClient *http.Client
	apiKey     string
}

func New(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{},
		apiKey:     apiKey,
	}
}

type SearchResult struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	ReleaseDate string `json:"release_date"`
}

func (c *Client) Search(q string, year int64) ([]SearchResult, error) {
	req, err := http.NewRequest(http.MethodGet, `https://api.themoviedb.org/3/search/movie`, nil)
	if err != nil {
		return nil, fmt.Errorf("building tmdb search request: %w", err)
	}

	qs := req.URL.Query()
	qs.Add("api_key", c.apiKey)
	qs.Add("query", q)
	if year != 0 {
		qs.Add("year", strconv.FormatInt(year, 10))
	}

	req.URL.RawQuery = qs.Encode()

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making tmdb search request: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error code from tmdb search request: %d", res.StatusCode)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response from tmdb search request: %w", err)
	}

	type searchResponse struct {
		Results []SearchResult `json:"results"`
	}

	var response searchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error decoding response from tmdb search: %w", err)
	}

	return response.Results, nil
}

func (c *Client) Get(id int64) (*Movie, error) {
	req, err := http.NewRequest(http.MethodGet, `https://api.themoviedb.org/3/movie/`+strconv.FormatInt(id, 10), nil)
	if err != nil {
		return nil, fmt.Errorf("building tmdb movie request: %w", err)
	}

	qs := req.URL.Query()
	qs.Add("api_key", c.apiKey)
	req.URL.RawQuery = qs.Encode()

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making tmdb movie request: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error code from tmdb movie request: %d", res.StatusCode)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response from tmdb movie request: %w", err)
	}

	var movie Movie
	if err := json.Unmarshal(body, &movie); err != nil {
		return nil, fmt.Errorf("error decoding response from tmdb movie request: %w", err)
	}

	return &movie, nil
}
