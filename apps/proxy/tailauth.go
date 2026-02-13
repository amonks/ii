package main

import (
	"encoding/json"
	"log/slog"
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
			slog.Warn("tailauth: whois failed",
				"error", err,
				"remote_addr", r.RemoteAddr,
			)
			h.ServeHTTP(w, r)
			return
		}
		user := ""
		if whois.UserProfile != nil {
			user = whois.UserProfile.LoginName
			r.Header.Set("Tailscale-User", user)
		}
		caps := capNames(whois.CapMap)
		setCapsHeaders(r, whois.CapMap)
		slog.Info("tailauth: identified user",
			"listener", "tailnet",
			"user", user,
			"node", whois.Node.Name,
			"remote_addr", r.RemoteAddr,
			"caps", caps,
		)
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
	caps := capNames(m.caps)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key := range r.Header {
			if strings.HasPrefix(key, "Tailscale-") {
				r.Header.Del(key)
			}
		}
		setCapsHeaders(r, m.caps)
		slog.Debug("tailauth: anon request",
			"listener", "public",
			"caps", caps,
			"remote_addr", r.RemoteAddr,
		)
		h.ServeHTTP(w, r)
	})
}

// capNames returns the short app names from a PeerCapMap (e.g. ["map", "dogs"]).
func capNames(caps tailcfg.PeerCapMap) []string {
	var names []string
	for cap := range caps {
		if s := string(cap); strings.HasPrefix(s, capPrefix) {
			names = append(names, strings.TrimPrefix(s, capPrefix))
		}
	}
	return names
}
