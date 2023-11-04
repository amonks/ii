package tmdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"monks.co/movietagger/ui"
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
	TMDBJSON      string     `json:"tmdb_json"`
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
	readToken  string
	writeToken string
}

func New(apiKey string, apiReadAccessToken string) *Client {
	return &Client{
		httpClient: &http.Client{},
		apiKey:     apiKey,
		readToken:  apiReadAccessToken,
	}
}

type SearchResult struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	ReleaseDate string `json:"release_date"`
}

func (c *Client) AuthorizeV4WriteAPI() error {
	if c.writeToken != "" {
		return nil
	}

	requestToken, err := c.getV4RequestToken()
	if err != nil {
		return err
	}

	fmt.Println(requestToken)

	if err := c.approveV4RequestToken(requestToken); err != nil {
		return err
	}

	if err := c.getV4WriteAccessToken(requestToken); err != nil {
		return err
	}

	return nil
}

func (c *Client) getV4RequestToken() (string, error) {
	req, err := http.NewRequest(http.MethodPost, "https://api.themoviedb.org/4/auth/request_token", nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.readToken)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making tmdb get-request-token request: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error code from tmdb get-request-token request: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response from tmdb get-request-token request: %w", err)
	}

	fmt.Println(string(body))

	var response struct {
		RequestToken string `json:"request_token"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("error decoding response from tmdb get-request-token request: %w", err)
	}

	return response.RequestToken, nil
}

func (c *Client) approveV4RequestToken(token string) error {
	fmt.Println("https://www.themoviedb.org/auth/access?request_token=" + token)
	ui.Prompt("please approve the token")
	return nil
}

func (c *Client) getV4WriteAccessToken(token string) error {
	req, err := http.NewRequest(http.MethodPost,
		"https://api.themoviedb.org/4/auth/access_token",
		bytes.NewBuffer([]byte(fmt.Sprintf(`{"request_token":"%s"}`, token))),
	)
	if err != nil {
		return err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.readToken)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making tmdb get-write-token request: %w", err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error reading response from tmdb get-write-token request: %w", err)
	}

	fmt.Println(string(body))

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("error code from tmdb get-write-token request: %d", res.StatusCode)
	}

	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("error decoding response from tmdb get-write-token request: %w", err)
	}

	c.writeToken = response.AccessToken

	return nil
}

func (c *Client) AddToList(listId, movieID int64) error {
	if c.writeToken == "" {
		return fmt.Errorf("not authorized for v4 write api; can't add to list")
	}
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("https://api.themoviedb.org/4/list/%d/items", listId),
		bytes.NewBuffer([]byte(fmt.Sprintf(`{"items":[{"media_type":"movie","media_id":"%d"}]}`, movieID))),
	)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.writeToken)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making tmdb list request: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("error code from tmdb add to list request: %d", res.StatusCode)
	}
	return nil
}

func (c *Client) List(listID int64) ([]SearchResult, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(`https://api.themoviedb.org/3/list/%d`, listID), nil)
	if err != nil {
		return nil, err
	}

	qs := req.URL.Query()
	qs.Add("api_key", c.apiKey)
	req.URL.RawQuery = qs.Encode()

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making tmdb list request: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error code from tmdb list request: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response from tmdb list request: %w", err)
	}

	var response struct {
		Results []SearchResult `json:"items"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error decoding response from tmdb list: %w", err)
	}

	fmt.Println("tmdb list success")

	return response.Results, nil
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

	body, err := io.ReadAll(res.Body)
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

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response from tmdb movie request: %w", err)
	}

	var movie Movie
	if err := json.Unmarshal(body, &movie); err != nil {
		return nil, fmt.Errorf("error decoding response from tmdb movie request: %w", err)
	}
	movie.TMDBJSON = string(body)

	return &movie, nil
}

type Credits struct {
	Cast []Person `json:"cast"`
	Crew []Person `json:"crew"`
}

type Person struct {
	Name string `json:"name"`
	Job  string `json:"job"`
}

func (c *Client) GetCredits(id int64) (*Credits, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, `https://api.themoviedb.org/3/movie/`+strconv.FormatInt(id, 10)+"/credits", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("building tmdb credits request: %w", err)
	}

	qs := req.URL.Query()
	qs.Add("api_key", c.apiKey)
	req.URL.RawQuery = qs.Encode()

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("error making tmdb credits request: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("error code from tmdb credits request: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading response from tmdb credits request: %w", err)
	}

	var credits Credits
	if err := json.Unmarshal(body, &credits); err != nil {
		return nil, nil, fmt.Errorf("error decoding response from tmdb credits request: %w", err)
	}

	return &credits, body, nil
}
