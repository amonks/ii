package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Post struct {
	Name      string `gorm:"primaryKey"`
	Title     string
	Author    string
	Subreddit string
	Url       string
	Permalink string

	Json *[]byte

	Status      string
	Filetype    *string
	Archivepath *string
	Created     *time.Time
	IsGallery   bool // Indicates if this post is a gallery
	GallerySize int  // Number of items in the gallery
}

// Status constants
const (
	StatusNew         = "new"         // Post fetched but media not processed yet
	StatusArchived    = "archived"    // Media successfully downloaded
	StatusDeleted     = "deleted"     // Media not available (404, etc.)
	StatusUnsupported = "unsupported" // Media format not supported
)

func (p *Post) Src() string {
	if p.IsGallery && p.Archivepath != nil {
		// For galleries, archivepath points to the first item
		// but we'll return a base path without the -1 suffix
		basePath := strings.TrimSuffix(*p.Archivepath, "-1"+*p.Filetype)
		return strings.Replace(basePath, archivePath, "/reddit/media/", 1)
	}
	return strings.Replace(*p.Archivepath, archivePath, "/reddit/media/", 1)
}

// GallerySrc returns the src path for a specific gallery item
func (p *Post) GallerySrc(index int) string {
	if !p.IsGallery || p.Archivepath == nil || p.Filetype == nil {
		return ""
	}

	// For galleries, construct the path with the item index
	basePath := strings.TrimSuffix(*p.Archivepath, "-1"+*p.Filetype)
	itemPath := fmt.Sprintf("%s-%d%s", basePath, index, *p.Filetype)
	return strings.Replace(itemPath, archivePath, "/reddit/media/", 1)
}

// SetCreatedFromJSON extracts and sets the created time from the Reddit JSON
func (p *Post) SetCreatedFromJSON() {
	if p.Json == nil {
		return
	}

	// Use a map to extract the created_utc field
	var data map[string]interface{}
	if err := json.Unmarshal(*p.Json, &data); err != nil {
		return
	}

	// Try to get the created_utc field
	if createdUtc, ok := data["created_utc"].(float64); ok {
		t := time.Unix(int64(createdUtc), 0)
		p.Created = &t
	}

	// Check if this is a gallery post
	if isGallery, ok := data["is_gallery"].(bool); ok && isGallery {
		p.IsGallery = true

		// Try to get gallery data
		if galleryData, ok := data["gallery_data"].(map[string]interface{}); ok {
			if items, ok := galleryData["items"].([]interface{}); ok {
				p.GallerySize = len(items)
			}
		}
	}
}
