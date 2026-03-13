package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Using TokenResponse from auth.go

// RedditArchiver handles fetching and archiving saved Reddit posts
type RedditArchiver struct {
	db                *model
	clientID          string
	clientSecret      string
	archivePath       string
	username          string
	accessToken       string
	refreshToken      string
	accessTokenExpiry time.Time
	httpClient        *http.Client
	tokenFile         string

	// Rate limit tracking
	rateUsed      int
	rateRemaining int
	rateResetTime time.Time
}

// NewRedditArchiver creates a new archiver instance
func NewRedditArchiver(db *model, clientID, clientSecret, archivePath, username string) *RedditArchiver {
	return &RedditArchiver{
		db:           db,
		clientID:     clientID,
		clientSecret: clientSecret,
		archivePath:  archivePath,
		username:     username,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		tokenFile:    "/data/tank/mirror/reddit/.tokens.json",

		// Initialize rate limits conservatively
		rateRemaining: 100,
		rateUsed:      0,
		rateResetTime: time.Now(),
	}
}

// UpdateArchive fetches new saved posts from Reddit and stores them with 'new' status
func (ra *RedditArchiver) UpdateArchive(ctx context.Context) error {
	if err := ra.ensureAuthenticated(); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Start with no "after" cursor for the first page
	var after string
	var totalProcessed, totalNew int

	// Main pagination loop
	for {
		// Check if we should stop (context cancelled)
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check and respect rate limits
		if ra.rateRemaining <= 5 {
			waitTime := time.Until(ra.rateResetTime)
			if waitTime > 0 {
				fmt.Printf("Rate limit approaching (%d remaining). Waiting %s for reset...\n",
					ra.rateRemaining, waitTime.Round(time.Second))
				select {
				case <-time.After(waitTime + time.Second): // Add a buffer second
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}

		// Fetch a page of saved posts
		posts, nextAfter, err := ra.fetchSavedPosts(after)
		if err != nil {
			return fmt.Errorf("fetching saved posts failed: %w", err)
		}

		fmt.Printf("Fetched %d posts (Rate limits: %d used, %d remaining, resets in %s)\n",
			len(posts), ra.rateUsed, ra.rateRemaining,
			time.Until(ra.rateResetTime).Round(time.Second))

		totalProcessed += len(posts)

		// Process each post in this page
		shouldContinue := true
		const retryAncients = true
		for _, post := range posts {
			// Skip non-link posts (like self posts without media)
			if !strings.HasPrefix(post.Name, "t3_") {
				continue
			}

			// Check if post already exists in the database
			exists, err := ra.postExists(post.Name)
			if err != nil {
				fmt.Printf("Error checking if post exists: %v\n", err)
				continue
			}

			if exists {
				fmt.Printf("Post %s already exists, skipping\n", post.Name)
				if retryAncients {
					shouldContinue = true
					continue
				} else {
					shouldContinue = false
					break
				}
			}

			// Save the new post (media will be downloaded later)
			if err := ra.processPost(post); err != nil {
				fmt.Printf("Error saving post %s: %v\n", post.Name, err)
				continue
			}

			totalNew++
		}

		if !shouldContinue || nextAfter == "" {
			break
		}

		// Apply a small delay between requests to be gentle with the API
		time.Sleep(2 * time.Second)

		after = nextAfter
	}

	fmt.Printf("Post fetching complete. Processed: %d, New: %d\n",
		totalProcessed, totalNew)
	return nil
}

// ensureAuthenticated makes sure we have a valid access token
func (ra *RedditArchiver) ensureAuthenticated() error {
	// If token is still valid, return early
	if ra.accessToken != "" && time.Now().Before(ra.accessTokenExpiry) {
		return nil
	}

	// Load tokens from file
	helper := ra.GetAuthHelper()
	tokens, err := helper.LoadTokens()
	if err != nil {
		// Check if token file doesn't exist
		if os.IsNotExist(err) || strings.Contains(err.Error(), "does not exist") {
			return &MissingTokenError{Message: fmt.Sprintf("token file does not exist at %s", ra.tokenFile)}
		}
		return fmt.Errorf("failed to load token data: %w", err)
	}

	// Check if we have necessary tokens
	if tokens.RefreshToken == "" {
		return &MissingTokenError{Message: "refresh token is missing in token file"}
	}

	// Update tokens
	ra.accessToken = tokens.AccessToken
	ra.refreshToken = tokens.RefreshToken

	// Try to refresh the token
	if err := ra.refreshAccessToken(); err != nil {
		return fmt.Errorf("failed to refresh access token: %w", err)
	}

	return nil
}

// MissingTokenError represents an error related to missing OAuth tokens
type MissingTokenError struct {
	Message string
}

func (e *MissingTokenError) Error() string {
	return e.Message
}

// GetAuthHelper returns an authentication helper for the current instance
func (ra *RedditArchiver) GetAuthHelper() *AuthHelper {
	helper := NewAuthHelper(ra.clientID, ra.clientSecret)
	helper.TokenFile = ra.tokenFile
	return helper
}

// GenerateAuthURL generates the URL for authorizing the app
func (ra *RedditArchiver) GenerateAuthURL() string {
	return ra.GetAuthHelper().GenerateAuthURL()
}

// ExchangeCode exchanges an authorization code for tokens
func (ra *RedditArchiver) ExchangeCode(code string) error {
	helper := ra.GetAuthHelper()

	tokenResp, err := helper.ExchangeCode(code)
	if err != nil {
		return err
	}

	// Update instance state
	ra.accessToken = tokenResp.AccessToken
	ra.refreshToken = tokenResp.RefreshToken
	ra.accessTokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return nil
}

// refreshAccessToken refreshes the access token using the refresh token
func (ra *RedditArchiver) refreshAccessToken() error {
	// If we don't have a refresh token, we can't refresh
	if ra.refreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	helper := ra.GetAuthHelper()
	tokenResp, err := helper.RefreshToken(ra.refreshToken)
	if err != nil {
		return err
	}

	// Update access token and expiry
	ra.accessToken = tokenResp.AccessToken
	ra.accessTokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	fmt.Printf("Access token refreshed successfully (expires in %d seconds)\n", tokenResp.ExpiresIn)
	return nil
}

// updateRateLimits updates the rate limit information from HTTP headers
func (ra *RedditArchiver) updateRateLimits(resp *http.Response) {
	// Extract rate limit headers
	if used := resp.Header.Get("X-Ratelimit-Used"); used != "" {
		if val, err := strconv.Atoi(used); err == nil {
			ra.rateUsed = val
		}
	}

	if remaining := resp.Header.Get("X-Ratelimit-Remaining"); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil {
			ra.rateRemaining = val
		}
	}

	if reset := resp.Header.Get("X-Ratelimit-Reset"); reset != "" {
		if seconds, err := strconv.Atoi(reset); err == nil {
			ra.rateResetTime = time.Now().Add(time.Duration(seconds) * time.Second)
		}
	}
}

// fetchSavedPosts fetches a page of saved posts from Reddit
func (ra *RedditArchiver) fetchSavedPosts(after string) ([]*Post, string, error) {
	// Ensure we have a valid token
	if err := ra.ensureAuthenticated(); err != nil {
		return nil, "", err
	}

	// Build the URL with optional pagination
	apiURL := fmt.Sprintf("https://oauth.reddit.com/user/%s/saved?limit=100", ra.username)
	if after != "" {
		apiURL += "&after=" + after
	}

	// Create request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, "", err
	}

	// Set headers
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Authorization", "Bearer "+ra.accessToken)

	// Make the request
	resp, err := ra.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	// Update rate limit information
	ra.updateRateLimits(resp)

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("API returned status code %d: %s", resp.StatusCode, string(body))
	}

	// Parse the JSON response
	var response struct {
		Data struct {
			After    string `json:"after"`
			Children []struct {
				Kind string          `json:"kind"`
				Data json.RawMessage `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, "", err
	}

	// Process each post
	var posts []*Post
	for _, child := range response.Data.Children {
		// We're only interested in link posts (t3)
		if child.Kind != "t3" {
			continue
		}

		// Parse the post data
		var postData map[string]any
		if err := json.Unmarshal(child.Data, &postData); err != nil {
			fmt.Printf("Error parsing post data: %v\n", err)
			continue
		}

		// Extract basic post details
		post := &Post{}

		// Extract string fields
		if v, ok := postData["name"].(string); ok {
			post.Name = v
		}
		if v, ok := postData["title"].(string); ok {
			post.Title = v
		}
		if v, ok := postData["author"].(string); ok {
			post.Author = v
		}
		if v, ok := postData["subreddit"].(string); ok {
			post.Subreddit = v
		}
		if v, ok := postData["url"].(string); ok {
			post.Url = v
		}
		if v, ok := postData["permalink"].(string); ok {
			post.Permalink = v
		}

		// Store the original JSON
		jsonBytes, err := json.Marshal(postData)
		if err != nil {
			fmt.Printf("Error serializing post JSON: %v\n", err)
		} else {
			post.Json = &jsonBytes
		}

		posts = append(posts, post)
	}

	return posts, response.Data.After, nil
}

// postExists checks if a post is already in the database
func (ra *RedditArchiver) postExists(name string) (bool, error) {
	var count int64
	err := ra.db.DB.Table("posts").Where("name = ?", name).Count(&count).Error
	return count > 0, err
}

// processPost sets initial state for a new post and saves it to the database
func (ra *RedditArchiver) processPost(post *Post) error {
	// Set initial status and created time
	post.Status = StatusNew
	post.SetCreatedFromJSON()

	// Save post to database with "new" status
	// Media downloading will be handled separately by ProcessUnarchived
	return ra.db.DB.Create(post).Error
}

// downloadMedia attempts to download and save media from a post
func (ra *RedditArchiver) downloadMedia(post *Post) error {
	// Skip if no URL
	if post.Url == "" {
		return nil
	}

	// Check if this is a gallery URL
	if strings.Contains(post.Url, "reddit.com/gallery/") && post.Json != nil {
		return ra.processGalleryPost(post)
	}

	// Parse URL
	parsedURL, err := url.Parse(post.Url)
	if err != nil {
		return err
	}

	// Determine file extension based on URL or content type
	fileExt := determineFileExtension(parsedURL.Path)
	if fileExt == "" {
		// Mark as unsupported if we can't determine the file type
		post.Status = StatusUnsupported
		return fmt.Errorf("unsupported media: could not determine file extension for %s", post.Url)
	}

	// Only support media types we can display
	if fileExt != ".jpg" && fileExt != ".png" && fileExt != ".gif" && fileExt != ".mp4" {
		post.Status = StatusUnsupported
		return fmt.Errorf("unsupported media type: %s for URL %s", fileExt, post.Url)
	}

	// Create the filename
	filename := post.Name + fileExt
	filePath := filepath.Join(ra.archivePath, filename)

	// Download the file
	if err := ra.downloadFile(post.Url, filePath); err != nil {
		return err
	}

	// Update post with file details
	post.Status = StatusArchived
	post.Filetype = &fileExt
	archivePath := filePath
	post.Archivepath = &archivePath

	return nil
}

// processGalleryPost handles downloading all images in a Reddit gallery post
func (ra *RedditArchiver) processGalleryPost(post *Post) error {
	// Extract post ID from name (format: t3_abc123)
	postID := strings.TrimPrefix(post.Name, "t3_")

	// First try using the cached JSON
	success, err := ra.processGalleryFromJSON(post)
	if err != nil {
		fmt.Printf("Error processing gallery from cached JSON: %v\n", err)
	}

	// If we managed to download at least one item, we're done
	if success {
		return nil
	}

	// If cached JSON failed or all downloads failed with 403, try to fetch fresh data
	fmt.Printf("Cached gallery URLs failed or expired. Attempting to fetch fresh data for post %s...\n", postID)

	// Check if we are authenticated
	if err := ra.ensureAuthenticated(); err != nil {
		return fmt.Errorf("authentication error while trying to fetch fresh data: %w", err)
	}

	// Fetch fresh data for the post
	freshData, err := ra.fetchPostData(postID)
	if err != nil {
		return fmt.Errorf("failed to fetch fresh data for gallery post: %w", err)
	}

	// Update the post's JSON
	jsonBytes, err := json.Marshal(freshData)
	if err != nil {
		return fmt.Errorf("failed to marshal fresh JSON data: %w", err)
	}
	post.Json = &jsonBytes

	// Try processing with fresh data
	success, err = ra.processGalleryFromJSON(post)
	if err != nil {
		return fmt.Errorf("error processing gallery with fresh data: %w", err)
	}

	if !success {
		post.Status = StatusDeleted
		return fmt.Errorf("failed to download gallery items even with fresh data")
	}

	return nil
}

// fetchPostData fetches fresh data for a post from the Reddit API
func (ra *RedditArchiver) fetchPostData(postID string) (map[string]any, error) {
	// Ensure we have a valid token
	if err := ra.ensureAuthenticated(); err != nil {
		return nil, err
	}

	// Build the URL - get the post by its ID
	apiURL := fmt.Sprintf("https://oauth.reddit.com/api/info?id=t3_%s", postID)

	// Create request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Authorization", "Bearer "+ra.accessToken)

	// Make the request
	resp, err := ra.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Update rate limit information
	ra.updateRateLimits(resp)

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status code %d: %s", resp.StatusCode, string(body))
	}

	// Parse the JSON response
	var response struct {
		Data struct {
			Children []struct {
				Data map[string]any `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	// Check if we got any results
	if len(response.Data.Children) == 0 {
		return nil, fmt.Errorf("no data found for post ID %s", postID)
	}

	// Return the post data from the first (and should be only) result
	return response.Data.Children[0].Data, nil
}

// processGalleryFromJSON processes gallery post from its JSON data
// Returns true if at least one item was successfully downloaded
func (ra *RedditArchiver) processGalleryFromJSON(post *Post) (bool, error) {
	var data map[string]any
	if err := json.Unmarshal(*post.Json, &data); err != nil {
		return false, fmt.Errorf("failed to parse post JSON: %w", err)
	}

	// Verify this is a gallery
	isGallery, ok := data["is_gallery"].(bool)
	if !ok || !isGallery {
		post.Status = StatusUnsupported
		return false, fmt.Errorf("post is not a gallery or missing gallery data")
	}

	// Get gallery data and media metadata
	galleryData, ok := data["gallery_data"].(map[string]any)
	if !ok {
		post.Status = StatusUnsupported
		return false, fmt.Errorf("missing gallery_data")
	}

	mediaMetadata, ok := data["media_metadata"].(map[string]any)
	if !ok {
		post.Status = StatusUnsupported
		return false, fmt.Errorf("missing media_metadata")
	}

	// Get the gallery items
	items, ok := galleryData["items"].([]any)
	if !ok || len(items) == 0 {
		post.Status = StatusUnsupported
		return false, fmt.Errorf("no gallery items found")
	}

	fmt.Printf("Processing gallery with %d items\n", len(items))
	post.IsGallery = true
	post.GallerySize = len(items)

	// Track errors but continue processing all items
	var firstFiletype string
	var firstArchivepath string

	var successCount int
	var forbidden403Count int

	// Process each gallery item
	for i, itemObj := range items {
		item, ok := itemObj.(map[string]any)
		if !ok {
			fmt.Printf("Error: Invalid item format at index %d\n", i)
			continue
		}

		mediaID, ok := item["media_id"].(string)
		if !ok {
			fmt.Printf("Error: Missing media_id at index %d\n", i)
			continue
		}

		// Get media metadata for this item
		mediaItemData, ok := mediaMetadata[mediaID].(map[string]any)
		if !ok {
			fmt.Printf("Error: Missing media metadata for ID %s\n", mediaID)
			continue
		}

		// Determine media type and URL
		mediaType, ok := mediaItemData["m"].(string)
		if !ok {
			fmt.Printf("Error: Missing media type for ID %s\n", mediaID)
			continue
		}

		// Get file extension from media type
		var fileExt string
		if strings.Contains(mediaType, "image/jpeg") || strings.Contains(mediaType, "image/jpg") {
			fileExt = ".jpg"
		} else if strings.Contains(mediaType, "image/png") {
			fileExt = ".png"
		} else if strings.Contains(mediaType, "image/gif") {
			fileExt = ".gif"
		} else if strings.Contains(mediaType, "video/mp4") {
			fileExt = ".mp4"
		} else {
			fmt.Printf("Unsupported media type: %s for ID %s\n", mediaType, mediaID)
			continue
		}

		// Get the source URL for the highest resolution version
		var sourceURL string

		// First try to get the s (source) field
		if s, ok := mediaItemData["s"].(map[string]any); ok {
			// Check if this is an animated image (gif)
			if typ, ok := mediaItemData["e"].(string); ok && typ == "AnimatedImage" {
				if mp4URL, ok := s["mp4"].(string); ok && mp4URL != "" {
					sourceURL = mp4URL
					fileExt = ".mp4" // Convert gif to mp4
				} else if gifURL, ok := s["gif"].(string); ok && gifURL != "" {
					sourceURL = gifURL
				}
			} else {
				// For regular images
				if url, ok := s["u"].(string); ok && url != "" {
					sourceURL = url
				} else if url, ok := s["gif"].(string); ok && url != "" {
					sourceURL = url
				}
			}
		}

		// If we still don't have a URL, try to find it in the 'p' field (preview array)
		if sourceURL == "" {
			if p, ok := mediaItemData["p"].([]any); ok && len(p) > 0 {
				// Get the last (highest resolution) preview
				lastPreview := p[len(p)-1]
				if preview, ok := lastPreview.(map[string]any); ok {
					if url, ok := preview["u"].(string); ok && url != "" {
						sourceURL = url
					}
				}
			}
		}

		// If we still don't have a URL, skip this item
		if sourceURL == "" {
			fmt.Printf("Error: Could not find source URL for ID %s\n", mediaID)
			continue
		}

		// Decode HTML entities in the URL (e.g., &amp; to &)
		sourceURL = strings.ReplaceAll(sourceURL, "&amp;", "&")

		// Create filename with index for each gallery item
		filename := fmt.Sprintf("%s-%d%s", post.Name, i+1, fileExt)
		filePath := filepath.Join(ra.archivePath, filename)

		// Download the file
		fmt.Printf("Downloading gallery item %d/%d: %s\n", i+1, len(items), sourceURL)
		err := ra.downloadFile(sourceURL, filePath)

		// Check if it's a 403 Forbidden error
		if err != nil {
			if strings.Contains(err.Error(), "status code 403") {
				forbidden403Count++
			}
			fmt.Printf("Error downloading gallery item %d: %v\n", i+1, err)
			continue
		}

		// Save the first successful download's info to update the post
		if successCount == 0 {
			firstFiletype = fileExt
			firstArchivepath = filePath
		}

		successCount++
	}

	// Update post with details from the first successful item
	if successCount > 0 {
		post.Status = StatusArchived
		post.Filetype = &firstFiletype
		post.Archivepath = &firstArchivepath
		fmt.Printf("Successfully downloaded %d/%d gallery items\n", successCount, len(items))
		return true, nil
	}

	// If all errors were 403 Forbidden, indicate this so caller can try fresh data
	if forbidden403Count > 0 && forbidden403Count == len(items) {
		fmt.Printf("All gallery items returned 403 Forbidden. URLs may have expired.\n")
	}

	return false, fmt.Errorf("failed to download any gallery items")
}

// downloadFile downloads a file from a URL to a local path
func (ra *RedditArchiver) downloadFile(fileURL, filePath string) error {
	// Create the request
	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		return err
	}

	// Set a user agent
	req.Header.Set("User-Agent", userAgent)

	// Download the file
	resp, err := ra.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode == http.StatusNotFound {
		return &MediaDeletedError{URL: fileURL}
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status code %d", resp.StatusCode)
	}

	// Create the file
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the content to the file
	_, err = io.Copy(out, resp.Body)
	return err
}

// MediaDeletedError represents an error when media is no longer available (404)
type MediaDeletedError struct {
	URL string
}

func (e *MediaDeletedError) Error() string {
	return fmt.Sprintf("media not found (404) at URL: %s", e.URL)
}

// determineFileExtension tries to determine the file extension from a URL
func determineFileExtension(urlPath string) string {
	// Common image and video extensions
	ext := strings.ToLower(filepath.Ext(urlPath))
	switch ext {
	case ".jpg":
		return ext
	case ".jpeg":
		return ".jpg"
	case ".png", ".gif":
		return ext
	case ".mp4":
		return ext
	case ".webm":
		return ".mp4" // Convert webm to mp4 extension
	default:
		return ""
	}
}

// ProcessUnarchived processes posts that have been saved but don't have media downloaded
func (ra *RedditArchiver) ProcessUnarchived(ctx context.Context) error {
	// Find posts that are in "new" status or have NULL status
	var unarchived []*Post
	if err := ra.db.DB.Where("status = ? OR status IS NULL", StatusNew).Find(&unarchived).Error; err != nil {
		return fmt.Errorf("failed to query unarchived posts: %w", err)
	}

	fmt.Printf("Found %d unarchived posts\n", len(unarchived))

	// Create RedGifs client
	redGifsClient := NewRedGifsClient()

	// Process each unarchived post
	var totalProcessed, totalArchived, totalDeleted, totalUnsupported int
	for _, post := range unarchived {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip posts without URLs
		if post.Url == "" {
			fmt.Printf("Post %s has no URL, skipping\n", post.Name)
			continue
		}

		// Make sure status is set to 'new' (for older posts without status)
		if post.Status == "" {
			post.Status = StatusNew
		}

		// If created is not set and we have Json data, try to set it
		if post.Created == nil && post.Json != nil {
			post.SetCreatedFromJSON()
		}

		// Try to download media
		fmt.Printf("Processing post URL: %s\n", post.Url)
		var err error
		if IsRedGifsURL(post.Url) {
			fmt.Printf("Identified as RedGifs URL\n")
			err = ra.processRedGifsPost(post, redGifsClient)
		} else {
			fmt.Printf("Processing as standard media URL\n")
			err = ra.downloadMedia(post)
		}

		// Handle media errors
		if err != nil {
			var mediaDeletedErr *MediaDeletedError
			if errors.As(err, &mediaDeletedErr) {
				// File is gone (404) - mark as deleted
				post.Status = StatusDeleted
				fmt.Printf("Post %s marked as deleted: %v\n", post.Name, err)
				totalDeleted++
			} else if post.Status == StatusUnsupported {
				// Media format not supported
				fmt.Printf("Post %s marked as unsupported: %v\n", post.Name, err)
				totalUnsupported++
			} else {
				fmt.Printf("Error processing post %s: %v\n", post.Name, err)
				// Continue with next post
				continue
			}
		}

		// Update post in database
		if post.Status == StatusArchived || post.Status == StatusDeleted || post.Status == StatusUnsupported {
			if err := ra.db.DB.Save(post).Error; err != nil {
				fmt.Printf("Error saving post %s to database: %v\n", post.Name, err)
				continue
			}

			if post.Status == StatusArchived {
				totalArchived++
			}
		}

		totalProcessed++

		// Be nice to the API - add small delay between requests
		time.Sleep(1 * time.Second)
	}

	fmt.Printf("Processed %d unarchived posts: %d archived, %d deleted, %d unsupported\n",
		totalProcessed, totalArchived, totalDeleted, totalUnsupported)

	return nil
}

// processRedGifsPost handles downloading media from RedGifs
func (ra *RedditArchiver) processRedGifsPost(post *Post, redGifsClient *RedGifsClient) error {
	fmt.Printf("Processing RedGifs post: %s\n", post.Name)

	// Get the video URL
	videoURL, err := redGifsClient.GetVideoURL(post.Url)
	if err != nil {
		return fmt.Errorf("failed to get video URL from RedGifs: %w", err)
	}

	fmt.Printf("Got RedGifs video URL: %s\n", videoURL)

	// Set the file extension
	fileExt := GetFileExtension(videoURL)

	// Create the filename
	filename := post.Name + fileExt
	filePath := filepath.Join(ra.archivePath, filename)

	// Download the file
	if err := ra.downloadFile(videoURL, filePath); err != nil {
		return fmt.Errorf("failed to download RedGifs video: %w", err)
	}

	// Update post with file details
	post.Status = StatusArchived
	post.Filetype = &fileExt
	archivePath := filePath
	post.Archivepath = &archivePath

	return nil
}

// Run initiates the archive update process
func (ra *RedditArchiver) Run() error {
	ctx := context.Background()

	// First, update the archive with new saved posts (this only fetches post metadata)
	fmt.Println("Fetching new saved posts from Reddit...")
	if err := ra.UpdateArchive(ctx); err != nil {
		return err
	}

	// Then, process all posts marked as "new" to download their media
	fmt.Println("\nProcessing posts to download media...")
	if err := ra.ProcessUnarchived(ctx); err != nil {
		return fmt.Errorf("processing posts to download media: %w", err)
	}

	return nil
}
