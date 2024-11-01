package lastfm

import (
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"monks.co/pkg/request"
)

type Lastfm struct {
	apiKey string
}

func New(apiKey string) *Lastfm {
	return &Lastfm{apiKey}
}

type Scrobble struct {
	Artist struct {
		URL  string `json:"url"`
		MBID string `json:"mbid"`
		Name string `json:"name"`
	} `json:"artist"`
	Streamable string `json:"streamable"`
	MBID       string `json:"mbid"`
	Album      struct {
		MBID string `json:"mbid"`
		Text string `json:"#text"`
	} `json:"album"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Date struct {
		UTS  string `json:"uts"`
		Text string `json:"#text"`
	} `json:"date"`
}

func (sc *Scrobble) Time() time.Time {
	var zero time.Time
	dateInt, err := strconv.ParseInt(sc.Date.UTS, 10, 64)
	if err != nil {
		return zero
	}
	return time.Unix(dateInt, 0)
}

type paginationInfo struct {
	User       string `json:"user"`
	TotalPages string `json:"totalPages"`
	Page       string `json:"page"`
	PerPage    string `json:"perPage"`
	Total      string `json:"total"`
}

func (pi paginationInfo) totalPages() int64 {
	if parsed, err := strconv.ParseInt(pi.TotalPages, 10, 64); err != nil {
		panic(err)
	} else {
		return parsed
	}
}

func (pi paginationInfo) page() int64 {
	if parsed, err := strconv.ParseInt(pi.Page, 10, 64); err != nil {
		panic(err)
	} else {
		return parsed
	}
}

func (pi paginationInfo) perPage() int64 {
	if parsed, err := strconv.ParseInt(pi.PerPage, 10, 64); err != nil {
		panic(err)
	} else {
		return parsed
	}
}

func (pi paginationInfo) total() int64 {
	if parsed, err := strconv.ParseInt(pi.Total, 10, 64); err != nil {
		panic(err)
	} else {
		return parsed
	}
}

type scrobblesResponse struct {
	RecentTracks struct {
		Track []Scrobble     `json:"track"`
		Attr  paginationInfo `json:"@attr"`
	} `json:"recenttracks"`
}

func (lastfm *Lastfm) FetchRecentScrobbles(user string) iter.Seq2[*Scrobble, error] {
	return func(yield func(*Scrobble, error) bool) {
		for page := 1; true; page += 1 {
			scrobs, pagination, err := lastfm.fetchRecentScrobblesPageWithRetry(user, page)
			if err != nil {
				yield(nil, err)
			}
			for _, scrob := range scrobs {
				if !yield(&scrob, nil) {
					return
				}
			}
			if pagination.page() >= pagination.totalPages() {
				return
			}
		}
	}
}

func (lastfm *Lastfm) fetchRecentScrobblesPageWithRetry(user string, pageno int) ([]Scrobble, *paginationInfo, error) {
	try := 0
	for {
		scrobs, pag, err := lastfm.fetchRecentScrobblesPage(user, pageno)
		if err != nil && try >= 5 {
			return nil, nil, err
		} else if err != nil {
			try++
			time.Sleep(time.Second * time.Duration(try))
			continue
		} else {
			return scrobs, pag, nil
		}
	}
}

func (lastfm *Lastfm) fetchRecentScrobblesPage(user string, pageno int) ([]Scrobble, *paginationInfo, error) {
	url, err := url.Parse(`http://ws.audioscrobbler.com/2.0/`)
	if err != nil {
		panic(err)
	}
	q := url.Query()
	q.Set("method", "user.getrecenttracks")
	q.Set("user", user)
	q.Set("api_key", lastfm.apiKey)
	q.Set("format", "json")
	q.Set("page", fmt.Sprintf("%d", pageno))
	q.Set("extended", "1")
	url.RawQuery = q.Encode()

	resp, err := http.Get(url.String())
	if err != nil {
		return nil, nil, err
	}
	if err := request.Error(resp); err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)

	var data scrobblesResponse
	if err := decoder.Decode(&data); err != nil {
		return nil, nil, err
	}

	// bs, err := json.MarshalIndent(data, "", "  ")
	// if err != nil {
	// 	return nil, nil, err
	// }
	// fmt.Println(string(bs))

	return data.RecentTracks.Track, &data.RecentTracks.Attr, nil
}
