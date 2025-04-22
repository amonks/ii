package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
)

// RedGifsClient is a client for the RedGifs API
type RedGifsClient struct {
	httpClient   *http.Client
	token        string
	tokenExpiry  time.Time
	retryAttempt int
}

// NewRedGifsClient creates a new RedGifs client
func NewRedGifsClient() *RedGifsClient {
	return &RedGifsClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetVideoURL extracts the MP4 URL from a RedGifs watch URL
func (r *RedGifsClient) GetVideoURL(watchURL string) (string, error) {
	// Extract the GIF ID from the URL
	id, err := extractRedGifsID(watchURL)
	if err != nil {
		return "", err
	}

	// Ensure we have a valid token
	if err := r.ensureToken(); err != nil {
		return "", fmt.Errorf("failed to get RedGifs token: %w", err)
	}

	// Fetch the GIF info from the API
	videoURL, err := r.fetchGifInfo(id)
	if err != nil {
		return "", err
	}

	return videoURL, nil
}

// extractRedGifsID extracts the GIF ID from a RedGifs URL
func extractRedGifsID(url string) (string, error) {
	// Handle different URL formats
	// Format 1: https://www.redgifs.com/watch/[id]
	// Format 2: https://v3.redgifs.com/watch/[id]
	// Format 3: https://www.redgifs.com/ifr/[id]
	// Format 4: https://i.redgifs.com/i/[id].jpg
	re1 := regexp.MustCompile(`redgifs\.com/(?:watch|ifr)/([a-zA-Z0-9]+)`)
	matches := re1.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1], nil
	}
	
	// For format 4: https://i.redgifs.com/i/palejaggednutcracker.jpg
	re2 := regexp.MustCompile(`i\.redgifs\.com/i/([a-zA-Z0-9]+)\.`)
	matches = re2.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("could not extract RedGifs ID from URL: %s", url)
}

// ensureToken ensures we have a valid RedGifs API token
func (r *RedGifsClient) ensureToken() error {
	// If we have a valid token, return early
	if r.token != "" && time.Now().Before(r.tokenExpiry) {
		return nil
	}

	// Set up the request
	req, err := http.NewRequest("GET", "https://api.redgifs.com/v2/auth/temporary", nil)
	if err != nil {
		return err
	}

	// Add common headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", "https://www.redgifs.com/")
	req.Header.Set("Origin", "https://www.redgifs.com")

	// Send the request
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check the response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get token, status: %d", resp.StatusCode)
	}

	// Parse the response
	var tokenResp struct {
		Token   string `json:"token"`
		Status  int    `json:"status"`
		Message string `json:"message"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return err
	}

	// Check that we got a token
	if tokenResp.Token == "" {
		return fmt.Errorf("no token in response: %s", string(body))
	}

	// Store the token (valid for 1 hour, we'll refresh after 50 minutes)
	r.token = tokenResp.Token
	r.tokenExpiry = time.Now().Add(50 * time.Minute)

	return nil
}

// fetchGifInfo fetches information about a GIF from the API
func (r *RedGifsClient) fetchGifInfo(id string) (string, error) {
	// Set up the request
	url := fmt.Sprintf("https://api.redgifs.com/v2/gifs/%s", id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	// Add headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", "https://www.redgifs.com/")
	req.Header.Set("Origin", "https://www.redgifs.com")

	// Send the request
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check for 401 Unauthorized - token might have expired
	if resp.StatusCode == http.StatusUnauthorized && r.retryAttempt < 3 {
		r.retryAttempt++
		// Force token refresh
		r.token = ""
		r.tokenExpiry = time.Time{}
		if err := r.ensureToken(); err != nil {
			return "", err
		}
		// Retry the request
		return r.fetchGifInfo(id)
	}

	// For 404 errors, return a MediaDeletedError
	if resp.StatusCode == http.StatusNotFound {
		return "", &MediaDeletedError{URL: fmt.Sprintf("https://redgifs.com/watch/%s", id)}
	}

	// For other errors
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d for ID %s", resp.StatusCode, id)
	}

	// Reset retry counter on success
	r.retryAttempt = 0

	// Parse the response
	var gifResp struct {
		Gif struct {
			URLs struct {
				HD      string `json:"hd"`
				SD      string `json:"sd"`
				Poster  string `json:"poster"`
				Thumbnail string `json:"thumbnail"`
			} `json:"urls"`
			Type int `json:"type"`
		} `json:"gif"`
		Status  int    `json:"status"`
		Message string `json:"message"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(body, &gifResp); err != nil {
		return "", err
	}

	// Prefer HD URL, fallback to SD
	videoURL := gifResp.Gif.URLs.HD
	if videoURL == "" {
		videoURL = gifResp.Gif.URLs.SD
	}

	if videoURL == "" {
		return "", errors.New("no video URL found in API response")
	}

	return videoURL, nil
}

// IsRedGifsURL checks if a URL is a RedGifs URL
func IsRedGifsURL(url string) bool {
	return strings.Contains(url, "redgifs.com")
}

// GetFileExtension gets the file extension from a URL
func GetFileExtension(url string) string {
	// Get the file extension from the URL path
	ext := path.Ext(url)

	// For URLs without a clear extension, infer from content type or format
	if ext == "" || ext == ".html" || ext == ".php" {
		if strings.Contains(url, ".mp4") {
			return ".mp4"
		}
		if strings.Contains(url, ".webm") {
			return ".mp4" // We'll convert to mp4
		}
		// Default to mp4 for RedGifs
		return ".mp4"
	}

	return ext
}
