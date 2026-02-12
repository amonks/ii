// Letterboxd fetches diary entries from the Letterboxd RSS feed.
package letterboxd

import (
	"encoding/gob"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"monks.co/pkg/env"
	"monks.co/pkg/flock"
)

type Watch struct {
	Date               time.Time
	Review             string
	MovieTitle         string
	Rating             int
	LetterboxdURL      string `gorm:"primaryKey"`
	MovieReleaseYear   int
	MovieLetterboxdURL string
	IsLiked            bool
	IsRewatch          bool
}

const (
	feedURL       = "https://letterboxd.com/amonks/rss/"
	username      = "amonks"
	defaultMaxAge = time.Hour
)

var FetchDiary = fetchDiary

func fetchDiary() ([]*Watch, error) {
	body, err := cachedFetch()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch RSS feed: %w", err)
	}

	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("failed to parse RSS feed: %w", err)
	}

	var watches []*Watch
	var parseErrors []error
	for i, item := range feed.Channel.Items {
		if item.WatchedDate == "" {
			continue // not a diary entry (e.g. a list)
		}
		watch, err := itemToWatch(&item)
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("entry %d: %w", i, err))
			continue
		}
		watches = append(watches, watch)
	}

	if len(parseErrors) > 0 {
		return watches, errors.Join(parseErrors...)
	}
	return watches, nil
}

func itemToWatch(item *rssItem) (*Watch, error) {
	date, err := time.Parse("2006-01-02", item.WatchedDate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse date '%s': %w", item.WatchedDate, err)
	}

	var rating int
	if item.MemberRating != "" {
		f, err := strconv.ParseFloat(item.MemberRating, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse rating '%s': %w", item.MemberRating, err)
		}
		rating = int(math.Round(f * 2))
	}

	var year int
	if item.FilmYear != "" {
		y, err := strconv.ParseInt(item.FilmYear, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse year '%s': %w", item.FilmYear, err)
		}
		year = int(y)
	}

	return &Watch{
		Date:               date,
		Review:             extractReview(item.Description),
		MovieTitle:         item.FilmTitle,
		Rating:             rating,
		LetterboxdURL:      item.Link,
		MovieReleaseYear:   year,
		MovieLetterboxdURL: deriveMovieURL(item.Link),
		IsLiked:            item.MemberLike == "Yes",
		IsRewatch:          item.Rewatch == "Yes",
	}, nil
}

// RSS XML types

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Link        string `xml:"link"`
	Description string `xml:"description"`

	WatchedDate  string `xml:"https://letterboxd.com watchedDate"`
	Rewatch      string `xml:"https://letterboxd.com rewatch"`
	FilmTitle    string `xml:"https://letterboxd.com filmTitle"`
	FilmYear     string `xml:"https://letterboxd.com filmYear"`
	MemberRating string `xml:"https://letterboxd.com memberRating"`
	MemberLike   string `xml:"https://letterboxd.com memberLike"`
}

// Review extraction

var (
	htmlTagRe     = regexp.MustCompile(`<[^>]*>`)
	boilerplateRe = regexp.MustCompile(`(?i)^\s*watched on .+\.\s*$`)
)

func extractReview(desc string) string {
	text := strings.TrimSpace(htmlTagRe.ReplaceAllString(desc, ""))
	if text == "" || boilerplateRe.MatchString(text) {
		return ""
	}
	return text
}

func deriveMovieURL(link string) string {
	prefix := fmt.Sprintf("https://letterboxd.com/%s", username)
	if strings.HasPrefix(link, prefix) {
		return "https://letterboxd.com" + strings.TrimPrefix(link, prefix)
	}
	return link
}

// HTTP caching

type httpCache struct {
	Body         []byte
	ETag         string
	LastModified string
	MaxAge       time.Duration
	FetchedAt    time.Time
}

func cachedFetch() ([]byte, error) {
	cachefile := env.InMonksData("letterboxd-rss.gob")
	lockfile := env.InMonksData("letterboxd-rss.lock")

	mustHaveFile(lockfile)
	mustHaveFile(cachefile)

	lock, err := flock.Lock(lockfile)
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()

	cache := loadCache(cachefile)

	if cache != nil && time.Since(cache.FetchedAt) < cache.MaxAge {
		log.Println("returning letterboxd RSS from cache")
		return cache.Body, nil
	}

	req, err := http.NewRequest("GET", feedURL, nil)
	if err != nil {
		return nil, err
	}
	if cache != nil {
		if cache.ETag != "" {
			req.Header.Set("If-None-Match", cache.ETag)
		}
		if cache.LastModified != "" {
			req.Header.Set("If-Modified-Since", cache.LastModified)
		}
	}

	log.Printf("fetch %s", feedURL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if cache != nil {
			log.Printf("network error fetching RSS, using stale cache: %s", err)
			return cache.Body, nil
		}
		return nil, fmt.Errorf("failed to fetch RSS feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified && cache != nil {
		log.Println("letterboxd RSS not modified (304)")
		cache.FetchedAt = time.Now()
		cache.MaxAge = parseMaxAge(resp.Header)
		saveCache(cachefile, cache)
		return cache.Body, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		if cache != nil {
			log.Printf("HTTP %d fetching RSS, using stale cache: %s", resp.StatusCode, string(body))
			return cache.Body, nil
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read RSS response: %w", err)
	}

	saveCache(cachefile, &httpCache{
		Body:         body,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		MaxAge:       parseMaxAge(resp.Header),
		FetchedAt:    time.Now(),
	})

	return body, nil
}

func parseMaxAge(header http.Header) time.Duration {
	cc := header.Get("Cache-Control")
	for _, part := range strings.Split(cc, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "max-age=") {
			if secs, err := strconv.Atoi(strings.TrimPrefix(part, "max-age=")); err == nil {
				return time.Duration(secs) * time.Second
			}
		}
	}
	return defaultMaxAge
}

func loadCache(path string) *httpCache {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || info.Size() == 0 {
		return nil
	}
	var cache httpCache
	if err := gob.NewDecoder(f).Decode(&cache); err != nil {
		return nil
	}
	return &cache
}

func saveCache(path string, cache *httpCache) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		log.Printf("failed to save RSS cache: %s", err)
		return
	}
	defer f.Close()
	if err := gob.NewEncoder(f).Encode(cache); err != nil {
		log.Printf("failed to encode RSS cache: %s", err)
	}
}

func mustHaveFile(filename string) {
	if _, err := os.Stat(filename); err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(err)
	} else if err != nil {
		f, err := os.Create(filename)
		if err != nil {
			panic(err)
		}
		f.Close()
	}
}
