package tmdb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// TVShow represents a TV series from TMDb
type TVShow struct {
	ID            int64       `json:"id"`
	Name          string      `json:"name"`
	OriginalName  string      `json:"original_name"`
	FirstAirDate  string      `json:"first_air_date"`
	LastAirDate   string      `json:"last_air_date"`
	Overview      string      `json:"overview"`
	Status        string      `json:"status"`
	Seasons       []Season    `json:"seasons"`
	Genres        []Genre     `json:"genres"`
	EpisodeRunTime []int64    `json:"episode_run_time"`
	Languages     []Language  `json:"spoken_languages"`
	PosterPath    string      `json:"poster_path"`
	TMDBJSON      string      `json:"tmdb_json"`
}

// Season represents a TV season from TMDb
type Season struct {
	ID            int64       `json:"id"`
	Name          string      `json:"name"`
	SeasonNumber  int         `json:"season_number"`
	EpisodeCount  int         `json:"episode_count"`
	AirDate       string      `json:"air_date"`
	Overview      string      `json:"overview"`
	PosterPath    string      `json:"poster_path"` 
}

// Episode represents a TV episode from TMDb
type Episode struct {
	ID            int64       `json:"id"`
	Name          string      `json:"name"`
	EpisodeNumber int         `json:"episode_number"`
	SeasonNumber  int         `json:"season_number"`
	AirDate       string      `json:"air_date"`
	Overview      string      `json:"overview"`
	StillPath     string      `json:"still_path"`
	Runtime       int64       `json:"runtime"`
}

// TVSearchResult for TV show search results
type TVSearchResult struct {
	ID            int64       `json:"id"`
	Name          string      `json:"name"`
	FirstAirDate  string      `json:"first_air_date"`
}

// SearchTV searches for TV shows by name
func (c *Client) SearchTV(q, year string) ([]TVSearchResult, error) {
	req, err := http.NewRequest(http.MethodGet, `https://api.themoviedb.org/3/search/tv`, nil)
	if err != nil {
		return nil, fmt.Errorf("building tmdb tv search request: %w", err)
	}

	qs := req.URL.Query()
	qs.Add("api_key", c.apiKey)
	qs.Add("query", q)
	if year != "" {
		qs.Add("first_air_date_year", year)
	}

	req.URL.RawQuery = qs.Encode()

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making tmdb tv search request: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error code from tmdb tv search request: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response from tmdb tv search request: %w", err)
	}

	type searchResponse struct {
		Results []TVSearchResult `json:"results"`
	}

	var response searchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error decoding response from tmdb tv search: %w", err)
	}

	return response.Results, nil
}

// GetTV fetches details for a TV show by ID
func (c *Client) GetTV(id int64) (*TVShow, error) {
	req, err := http.NewRequest(http.MethodGet, `https://api.themoviedb.org/3/tv/`+strconv.FormatInt(id, 10), nil)
	if err != nil {
		return nil, fmt.Errorf("building tmdb tv request: %w", err)
	}

	qs := req.URL.Query()
	qs.Add("api_key", c.apiKey)
	req.URL.RawQuery = qs.Encode()

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making tmdb tv request: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error code from tmdb tv request: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response from tmdb tv request: %w", err)
	}

	var tvShow TVShow
	if err := json.Unmarshal(body, &tvShow); err != nil {
		return nil, fmt.Errorf("error decoding response from tmdb tv request: %w", err)
	}
	tvShow.TMDBJSON = string(body)

	return &tvShow, nil
}

// GetSeason fetches details for a specific season of a TV show
func (c *Client) GetSeason(tvID int64, seasonNumber int) (*Season, []Episode, error) {
	req, err := http.NewRequest(http.MethodGet, 
		fmt.Sprintf(`https://api.themoviedb.org/3/tv/%d/season/%d`, tvID, seasonNumber), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("building tmdb season request: %w", err)
	}

	qs := req.URL.Query()
	qs.Add("api_key", c.apiKey)
	req.URL.RawQuery = qs.Encode()

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("error making tmdb season request: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("error code from tmdb season request: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading response from tmdb season request: %w", err)
	}

	var response struct {
		ID           int64    `json:"id"`
		Name         string   `json:"name"`
		SeasonNumber int      `json:"season_number"`
		AirDate      string   `json:"air_date"`
		Overview     string   `json:"overview"`
		Episodes     []Episode `json:"episodes"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, nil, fmt.Errorf("error decoding response from tmdb season request: %w", err)
	}

	season := &Season{
		ID:           response.ID,
		Name:         response.Name,
		SeasonNumber: response.SeasonNumber,
		EpisodeCount: len(response.Episodes),
		AirDate:      response.AirDate,
		Overview:     response.Overview,
	}

	return season, response.Episodes, nil
}

// GetEpisode fetches details for a specific episode of a TV show
func (c *Client) GetEpisode(tvID int64, seasonNumber, episodeNumber int) (*Episode, error) {
	req, err := http.NewRequest(http.MethodGet, 
		fmt.Sprintf(`https://api.themoviedb.org/3/tv/%d/season/%d/episode/%d`, 
			tvID, seasonNumber, episodeNumber), nil)
	if err != nil {
		return nil, fmt.Errorf("building tmdb episode request: %w", err)
	}

	qs := req.URL.Query()
	qs.Add("api_key", c.apiKey)
	req.URL.RawQuery = qs.Encode()

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making tmdb episode request: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error code from tmdb episode request: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response from tmdb episode request: %w", err)
	}

	var episode Episode
	if err := json.Unmarshal(body, &episode); err != nil {
		return nil, fmt.Errorf("error decoding response from tmdb episode request: %w", err)
	}

	return &episode, nil
}