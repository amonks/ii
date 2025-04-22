package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Configuration constants
const (
	// A URL that Reddit will redirect to after authorization
	// This should match what's registered in the Reddit app settings
	redirectURI = "https://thor.ss.cx/reddit/"
	
	// File to save tokens to
	defaultTokenFile = "/data/tank/mirror/reddit/.tokens.json"
)

// AuthHelper provides utilities for Reddit OAuth authentication
type AuthHelper struct {
	ClientID     string
	ClientSecret string
	TokenFile    string
}

// NewAuthHelper creates a new authentication helper
func NewAuthHelper(clientID, clientSecret string) *AuthHelper {
	return &AuthHelper{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenFile:    defaultTokenFile,
	}
}

// GenerateAuthURL generates the URL for authorizing the app
func (a *AuthHelper) GenerateAuthURL() string {
	return fmt.Sprintf(
		"https://www.reddit.com/api/v1/authorize?"+
			"client_id=%s&"+
			"response_type=code&"+
			"state=foobar&"+
			"redirect_uri=%s&"+
			"duration=permanent&"+
			"scope=history",
		a.ClientID,
		url.QueryEscape(redirectURI),
	)
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// ExchangeCode exchanges an authorization code for tokens
func (a *AuthHelper) ExchangeCode(code string) (*TokenResponse, error) {
	// Prepare token request
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	// Create request
	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", 
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	// Set headers
	req.SetBasicAuth(a.ClientID, a.ClientSecret)
	req.Header.Set("User-Agent", "golang:monks.co.reddit:v1.0")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	// Save tokens to file
	if err := a.SaveTokens(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// RefreshToken refreshes the access token using the provided refresh token
func (a *AuthHelper) RefreshToken(refreshToken string) (*TokenResponse, error) {
	// Prepare refresh request
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	// Create request
	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", 
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	// Set headers
	req.SetBasicAuth(a.ClientID, a.ClientSecret)
	req.Header.Set("User-Agent", "golang:monks.co.reddit:v1.0")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response - note that refresh responses don't include the refresh token
	var refreshResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}

	if err := json.Unmarshal(body, &refreshResp); err != nil {
		return nil, err
	}

	// Combine with existing refresh token
	tokenResp := &TokenResponse{
		AccessToken:  refreshResp.AccessToken,
		TokenType:    refreshResp.TokenType,
		ExpiresIn:    refreshResp.ExpiresIn,
		RefreshToken: refreshToken,
		Scope:        refreshResp.Scope,
	}

	// Save updated tokens
	if err := a.SaveTokens(tokenResp); err != nil {
		return nil, err
	}

	return tokenResp, nil
}

// LoadTokens loads the OAuth tokens from the token file
func (a *AuthHelper) LoadTokens() (*TokenResponse, error) {
	// Check if file exists
	if _, err := os.Stat(a.TokenFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("token file does not exist: %s", a.TokenFile)
	}

	// Read file
	data, err := os.ReadFile(a.TokenFile)
	if err != nil {
		return nil, err
	}

	// Parse JSON
	var tokenResp TokenResponse
	if err := json.Unmarshal(data, &tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// SaveTokens saves the tokens to the token file
func (a *AuthHelper) SaveTokens(tokenResp *TokenResponse) error {
	// Convert to JSON
	jsonData, err := json.MarshalIndent(tokenResp, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(a.TokenFile, jsonData, 0600)
}

// PrintAuthHelp prints instructions for the OAuth flow
func (a *AuthHelper) PrintAuthHelp() {
	fmt.Println("\n=====================================================")
	fmt.Println("Reddit OAuth Authentication Required")
	fmt.Println("=====================================================")
	fmt.Println("This program needs authorization to access your saved Reddit posts.")
	fmt.Println("Please follow these steps:")
	fmt.Println()
	fmt.Println("1. Open this URL in any browser:")
	fmt.Println(a.GenerateAuthURL())
	fmt.Println()
	fmt.Println("2. Log in to Reddit and authorize the application")
	fmt.Println()
	fmt.Println("3. After authorizing, you'll be redirected to:")
	fmt.Println("   https://thor.ss.cx/reddit/?state=foobar&code=YOUR_CODE_HERE")
	fmt.Println()
	fmt.Println("4. Copy the entire 'code' parameter value from the URL")
	fmt.Println("   It will be a long string after 'code=' and before any '&' character")
	fmt.Println()
	fmt.Println("5. Enter the code here: ")
}