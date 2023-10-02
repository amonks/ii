package confrunner

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"monks.co/auth"
	"monks.co/config"
	"monks.co/golink"
	"monks.co/ping"
	"monks.co/places"
	"monks.co/promises"
	"monks.co/weblog"
)

type Server interface {
	Start(ctx context.Context) error
	AddMiddleware(func(http.Handler) http.Handler)
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	Stop() error
}

func NewServer(c config.App) Server {
	s := getServer(c)

	// hack
	s.AddMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.Host, "belgianman.com") || strings.Contains(r.Host, "blgn.mn") {
				http.Redirect(w, r, "https://music.belgianman.com", http.StatusMovedPermanently)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	if !c.IsPublic {
		s.AddMiddleware(auth.InternalHandler)
	}

	return s
}

func getServer(c config.App) Server {
	switch c.Name {
	case "promises":
		return promises.New()
	case "weblog":
		return weblog.New()
	case "ping":
		return ping.New()
	case "go":
		return golink.New()
	case "places":
		return places.New()
	}

	panic(fmt.Errorf("unsupported app: '%s'", c.Name))
}
