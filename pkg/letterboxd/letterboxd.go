// Letterboxd implements a scraper for the letterboxd website.
//
// They also have a documented API, but it isn't public. This package
// implemented a client for that API at commit,
//
//	cd28e26a6110e819c3c923d2cc8dd37117ec05fc
package letterboxd

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
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

func FetchDiary(username string, pageno, pagelimit int) ([]*Watch, error) {
	var entries []*Watch

	for {
		if pageno > pagelimit {
			break
		}

		url := fmt.Sprintf("https://letterboxd.com/%s/films/diary/page/%d/", username, pageno)
		fmt.Println("fetch ", url)
		doc, err := fetch(url)
		if err != nil {
			return nil, err
		}

		var findErr error
		if doc.Find("tr.diary-entry-row").Each(func(_ int, result *goquery.Selection) {
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

			entries = append(entries, &Watch{
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

		hasNextPage := doc.Find(".pagination .paginate-nextprev > a.next").Length() != 0
		if !hasNextPage {
			break
		}

		pageno++
	}

	return entries, nil
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
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if bs, err := io.ReadAll(resp.Body); err != nil {
			return nil, fmt.Errorf("http error %s", resp.Status)
		} else {
			return nil, fmt.Errorf("http error %s: %s", resp.Status, string(bs))
		}
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	return doc, nil
}
