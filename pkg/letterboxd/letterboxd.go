// Letterboxd implements a scraper for the letterboxd website.
//
// They also have a documented API, but it isn't public. This package
// implemented a client for that API at commit,
//
//	cd28e26a6110e819c3c923d2cc8dd37117ec05fc
package letterboxd

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/brotli"
	"monks.co/pkg/hardmemo"
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

var (
	diaryCacheExpiration = time.Hour * 6
)

var FetchDiary = hardmemo.Memoize[[]*Watch]("letterboxd-diary", diaryCacheExpiration, fetchDiary)

func fetchDiary() ([]*Watch, error) {
	const username = "amonks"
	const pageno = 1

	url := fmt.Sprintf("https://letterboxd.com/%s/films/diary/page/%d/", username, pageno)
	fmt.Println("fetch ", url)
	doc, err := fetch(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch diary page: %w", err)
	}

	var diary []*Watch
	var parseErrors []error
	doc.Find("tr.diary-entry-row").Each(func(i int, result *goquery.Selection) {
		fmt.Println("handle diary entry")

		row := &diaryRow{result, username}

		date, err := row.Date()
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("entry %d: failed to parse date: %w", i, err))
			return
		}

		review, err := row.Review()
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("entry %d: failed to fetch review: %w", i, err))
			return
		}

		movieReleaseYear, err := row.MovieReleaseYear()
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("entry %d: failed to parse movie release year: %w", i, err))
			return
		}

		rating, err := row.Rating()
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("entry %d: failed to parse rating: %w", i, err))
			return
		}

		letterboxdURL, err := row.LetterboxdURL()
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("entry %d: failed to get letterboxd URL: %w", i, err))
			return
		}

		movieLetterboxdURL, err := row.MovieLetterboxdURL()
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("entry %d: failed to get movie letterboxd URL: %w", i, err))
			return
		}

		diary = append(diary, &Watch{
			Date:          date,
			Review:        review,
			Rating:        rating,
			LetterboxdURL: letterboxdURL,
			IsLiked:       row.IsLiked(),
			IsRewatch:     row.IsRewatch(),

			MovieTitle:         row.MovieTitle(),
			MovieReleaseYear:   movieReleaseYear,
			MovieLetterboxdURL: movieLetterboxdURL,
		})
	})

	if len(parseErrors) > 0 {
		return diary, errors.Join(parseErrors...)
	}
	return diary, nil
}

type diaryRow struct {
	*goquery.Selection
	username string
}

func (s *diaryRow) Date() (time.Time, error) {
	dateStr := strings.TrimSuffix(
		strings.TrimPrefix(
			s.Find("td.col-daydate.diary-day > a").AttrOr("href", ""),
			fmt.Sprintf("/%s/films/diary/for/", s.username),
		),
		"/",
	)
	if dateStr == "" {
		return time.Time{}, nil
	}

	date, err := time.Parse("2006/01/02", dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("error parsing date '%s': %w", dateStr, err)
	}

	return date, nil
}

func (s *diaryRow) Review() (string, error) {
	path, found := s.Find("td.col-review a").Attr("href")
	if !found {
		return "", nil
	}

	url := "https://letterboxd.com" + path
	doc, err := fetch(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch review from %s: %w", url, err)
	}

	return doc.Find("div.review.body-text").Text(), nil
}

func (s *diaryRow) LetterboxdURL() (string, error) {
	href, got := s.Find("td.col-production a").Attr("href")
	if !got {
		html, _ := s.Html()
		return "", fmt.Errorf("could not find letterboxd url in:\n%s", html)
	}
	return "https://letterboxd.com" + href, nil
}

func (s *diaryRow) Rating() (int, error) {
	str := s.Find("td.col-rating .editable-rating > input").AttrOr("value", "0")
	rating, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse rating '%s': %w", str, err)
	}
	return int(rating), nil
}

func (s *diaryRow) IsLiked() bool {
	return s.Find("td.col-like > span.icon-liked").Length() > 0
}

func (s *diaryRow) IsRewatch() bool {
	return s.Find("td.col-rewatch.icon-status-off").Length() == 0
}

func (s *diaryRow) MovieTitle() string {
	return s.Find("td.col-production h2").Text()
}

func (s *diaryRow) MovieReleaseYear() (int, error) {
	str := s.Find("td.col-releaseyear").Text()
	year, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse movie release year '%s': %w", str, err)
	}
	return int(year), nil
}

func (s *diaryRow) MovieLetterboxdURL() (string, error) {
	href, got := s.Find("td.col-production a").Attr("href")
	if !got {
		html, _ := s.Html()
		return "", fmt.Errorf("could not find movie letterboxd url in:\n%s", html)
	}
	return "https://letterboxd.com" + strings.TrimPrefix(href, fmt.Sprintf("/%s", s.username)), nil
}

func fetch(url string) (*goquery.Document, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers to mimic Safari browser to avoid detection
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.2 Safari/605.1.15")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Priority", "u=0, i")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get page content from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("http error %s: %s", resp.Status, string(body))
	}

	var reader io.Reader = resp.Body
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer reader.(io.ReadCloser).Close()
	case "br":
		reader = brotli.NewReader(resp.Body)
	}

	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML document from %s: %w", url, err)
	}

	return doc, nil
}
