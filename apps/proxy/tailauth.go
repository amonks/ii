package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"monks.co/pkg/middleware"
	"monks.co/pkg/tailnet"
	"tailscale.com/tailcfg"
)

const capPrefix = "monks.co/cap/"

func setCapsHeaders(r *http.Request, caps tailcfg.PeerCapMap) {
	for cap, vals := range caps {
		capStr := string(cap)
		if !strings.HasPrefix(capStr, capPrefix) {
			continue
		}
		appName := strings.TrimPrefix(capStr, capPrefix)
		headerName := "Tailscale-Cap-" + http.CanonicalHeaderKey(appName)
		raw, _ := json.Marshal(vals)
		r.Header.Set(headerName, string(raw))
	}
}

var _ middleware.Middleware = tailscaleAuthMiddleware{}

// tailscaleAuthMiddleware identifies tailnet users via WhoIs and forwards
// their identity and capabilities as headers.
type tailscaleAuthMiddleware struct{}

func (tailscaleAuthMiddleware) ModifyHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		whois, err := tailnet.WhoIs(r.Context(), r.RemoteAddr)
		if err != nil {
			log.Printf("tailauth: whois error: %v", err)
			h.ServeHTTP(w, r)
			return
		}
		if whois.UserProfile != nil {
			r.Header.Set("Tailscale-User", whois.UserProfile.LoginName)
		}
		setCapsHeaders(r, whois.CapMap)
		h.ServeHTTP(w, r)
	})
}

var _ middleware.Middleware = anonCapsMiddleware{}

// anonCapsMiddleware strips incoming Tailscale-* headers (to prevent spoofing)
// and sets capability headers from the cached anon caps.
type anonCapsMiddleware struct {
	caps tailcfg.PeerCapMap
}

func (m anonCapsMiddleware) ModifyHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key := range r.Header {
			if strings.HasPrefix(key, "Tailscale-") {
				r.Header.Del(key)
			}
		}
		setCapsHeaders(r, m.caps)
		h.ServeHTTP(w, r)
	})
}
