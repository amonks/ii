// Letterboxd implements a scraper for the letterboxd website.
//
// They also have a documented API, but it isn't public. This package
// implemented a client for that API at commit,
//
//	cd28e26a6110e819c3c923d2cc8dd37117ec05fc
package letterboxd

import (
	"compress/gzip"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/brotli"
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

var (
	diaryCacheFile       = filepath.Join(os.Getenv("MONKS_DATA"), "letterboxd-diary.gob")
	diaryLockFile        = filepath.Join(os.Getenv("MONKS_DATA"), "letterboxd-diary.lock")
	diaryCacheExpiration = time.Hour * 6
)

func init() {
	mustHaveFile(diaryCacheFile)
	mustHaveFile(diaryLockFile)
}

func mustHaveFile(filename string) {
	if _, err := os.Stat(filename); err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(err)
	} else if err != nil {
		f, err := os.Create(diaryCacheFile)
		if err != nil {
			panic(err)
		}
		f.Close()
	}
}

type diaryResult struct {
	Diary []*Watch
	Err   string
}

func FetchDiary() ([]*Watch, error) {
	lock, err := flock.Lock(diaryLockFile)
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()

	if fileinfo, err := os.Stat(diaryCacheFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	} else if err == nil && fileinfo.ModTime().After(time.Now().Add(-diaryCacheExpiration)) && fileinfo.Size() > 0 {
		log.Printf("using letterboxd diary from cache")
		cachefile, err := os.Open(diaryCacheFile)
		if err != nil {
			return nil, err
		}
		dec := gob.NewDecoder(cachefile)
		var cached diaryResult
		if err := dec.Decode(&cached); err != nil {
			return nil, err
		}
		if cached.Err != "" {
			return nil, errors.New(cached.Err)
		}
		return cached.Diary, nil
	} else {
		log.Printf("fetching letterboxd diary")
		diary, err := fetchDiary()
		var result diaryResult
		if err != nil {
			result = diaryResult{Err: err.Error()}
		} else {
			result = diaryResult{Diary: diary}
		}
		cache, err := os.OpenFile(diaryCacheFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			return nil, err
		}
		enc := gob.NewEncoder(cache)
		if err := enc.Encode(result); err != nil {
			return nil, err
		}

		if result.Err != "" {
			return nil, errors.New(result.Err)
		} else {
			return result.Diary, nil
		}
	}
}

func fetchDiary() ([]*Watch, error) {
	const username = "amonks"
	const pageno = 1

	url := fmt.Sprintf("https://letterboxd.com/%s/films/diary/page/%d/", username, pageno)
	fmt.Println("fetch ", url)
	doc, err := fetch(url)
	if err != nil {
		return nil, err
	}

	var diary []*Watch
	var findErr error
	if doc.Find("tr.diary-entry-row").Each(func(_ int, result *goquery.Selection) {
		fmt.Println("handle diary entry")
		if findErr != nil {
			return
		}

		row := &diaryRow{result, username}

		date, err := row.Date()
		if err != nil {
			findErr = err
			return
		}

		review, err := row.Review()
		if err != nil {
			findErr = err
			return
		}

		movieReleaseYear, err := row.MovieReleaseYear()
		if err != nil {
			findErr = err
			return
		}

		rating, err := row.Rating()
		if err != nil {
			findErr = err
			return
		}

		letterboxdURL, err := row.LetterboxdURL()
		if err != nil {
			findErr = err
			return
		}

		movieLetterboxdURL, err := row.MovieLetterboxdURL()
		if err != nil {
			findErr = err
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
	}); findErr != nil {
		return nil, findErr
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
			s.Find("td.td-day.diary-day > a").AttrOr("href", ""),
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
	path, found := s.Find("td.td-review > a").Attr("href")
	if !found {
		return "", nil
	}

	url := "https://letterboxd.com" + path
	doc, err := fetch(url)
	if err != nil {
		return "", err
	}

	return doc.Find("div.review.body-text").Text(), nil
}

func (s *diaryRow) LetterboxdURL() (string, error) {
	href, got := s.Find("td.td-film-details > h3 > a").Attr("href")
	if !got {
		return "", fmt.Errorf("could not find letterboxd url")
	}
	return "https://letterboxd.com" + href, nil
}

func (s *diaryRow) Rating() (int, error) {
	str := s.Find("td.td-rating > .editable-rating > input").AttrOr("value", "0")
	rating, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0, err
	}
	return int(rating), nil
}

func (s *diaryRow) IsLiked() bool {
	return s.Find("td.td-like > span.icon-liked").Length() > 0
}

func (s *diaryRow) IsRewatch() bool {
	return s.Find("td.td-rewatch.icon-status-off").Length() == 0
}

func (s *diaryRow) MovieTitle() string {
	return s.Find("td.td-film-details > h3").Text()
}

func (s *diaryRow) MovieReleaseYear() (int, error) {
	str := s.Find("td.td-released").Text()
	year, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0, err
	}
	return int(year), nil
}

func (s *diaryRow) MovieLetterboxdURL() (string, error) {
	href, got := s.Find("td.td-film-details > h3 > a").Attr("href")
	if !got {
		return "", fmt.Errorf("could not find letterboxd url")
	}
	return "https://letterboxd.com" + strings.TrimPrefix(href, fmt.Sprintf("/%s", s.username)), nil
}

func fetch(url string) (*goquery.Document, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", `text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7`)
	req.Header.Set("Accept-Encoding", `gzip, deflate, br, zstd`)
	req.Header.Set("Accept-Language", `en-US,en;q=0.9`)
	req.Header.Set("Cache-Control", `no-cache`)
	req.Header.Set("Cookie", `com.xk72.webparts.csrf=1c4f14f319aba35c5eec`)
	req.Header.Set("Pragma", `no-cache`)
	req.Header.Set("Priority", `u=0, i`)
	req.Header.Set("Sec-Ch-Ua", `"Not/A)Brand";v="8", "Chromium";v="126", "Google Chrome";v="126"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", `?0`)
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", `document`)
	req.Header.Set("Sec-Fetch-Mode", `navigate`)
	req.Header.Set("Sec-Fetch-Site", `none`)
	req.Header.Set("Sec-Fetch-User", `?1`)
	req.Header.Set("Upgrade-Insecure-Requests", `1`)
	req.Header.Set("User-Agent", `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36`)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	var reader io.Reader
	fmt.Println("content encoding:", resp.Header.Get("Content-Encoding"))
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		defer reader.(io.ReadCloser).Close()
	case "br":
		reader = brotli.NewReader(resp.Body)
	default:
		reader = resp.Body
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if bs, err := io.ReadAll(reader); err != nil {
			return nil, fmt.Errorf("http error %s", resp.Status)
		} else {
			return nil, fmt.Errorf("http error %s: %s", resp.Status, string(bs))
		}
	}

	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, err
	}

	fmt.Printf("node count: %d\n", len(doc.Nodes))

	return doc, nil
}
