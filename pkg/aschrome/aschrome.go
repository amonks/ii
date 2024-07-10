package aschrome

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"

	"github.com/andybalholm/brotli"
)

// Get implements an HTTP get request that mimicks the chrome browser, passing
// the same headers.
//
// XXX: it sends "Accept-Encoding" headers including deflate and zstd, because
// Chrome does that, but it really only knows how to decode gzip and brotli.
func Get(url string) (io.Reader, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", `text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7`)
	req.Header.Set("Accept-Encoding", `gzip, deflate, br, zstd`)
	req.Header.Set("Accept-Language", `en-US,en;q=0.9`)
	req.Header.Set("Cache-Control", `no-cache`)
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

	return reader, nil
}
