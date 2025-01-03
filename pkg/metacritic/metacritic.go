package metacritic

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type SearchResult struct {
	URL   string
	Title string
	Year  int
	Score int
}

var sanitize = strings.NewReplacer(
	"/", "",
	"?", "")

func SearchMovies(query string) ([]*SearchResult, error) {
	encodedQuery := url.QueryEscape(sanitize.Replace(query))
	res, err := http.Get(fmt.Sprintf(`https://www.metacritic.com/search/%s/?page=1&category=2`, encodedQuery))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}
	var searchResults []*SearchResult
	var findErr error
	if doc.Find(".g-grid-container.u-grid-columns").Each(func(_ int, result *goquery.Selection) {
		if findErr != nil {
			return
		}

		title := strings.TrimSpace(result.Find(".g-text-medium-fluid.g-text-bold").Text())
		if title == "" {
			findErr = fmt.Errorf("empty movie title")
			return
		}

		href, _ := result.Find("a").Attr("href")
		if href == "" || !strings.HasPrefix(href, "/") {
			findErr = fmt.Errorf("link looks weird: '%s'", href)
			return
		}
		url := "https://metacritic.com" + href

		if !strings.Contains(url, "/movie/") {
			return
		}

		score := strings.TrimSpace(result.Find(".c-siteReviewScore").Text())
		intScore, err := strconv.ParseInt(score, 10, 64)
		if err != nil && score != "tbd" {
			return
		}

		year := strings.TrimSpace(result.Find(".u-text-uppercase").Text())
		intYear, _ := strconv.ParseInt(year, 10, 64)

		searchResults = append(searchResults, &SearchResult{
			URL:   url,
			Title: title,
			Year:  int(intYear),
			Score: int(intScore),
		})
	}); findErr != nil {
		return nil, findErr
	}
	return searchResults, nil
}
