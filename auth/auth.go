package auth

import (
	"fmt"
	"net/http"

	"tailscale.com/client/tailscale"
)

func GetRequester(r *http.Request) (bool, string) {
	addr := r.RemoteAddr

	who, err := tailscale.WhoIs(r.Context(), addr)
	if err != nil {
		return false, ""
	}

	return true, fmt.Sprintf("%s (@%s)", who.UserProfile.DisplayName, who.Node.ComputedName)
}

func InternalHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasAuth, _ := GetRequester(r)
		if !hasAuth {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(http.StatusText(http.StatusUnauthorized)))
			return
		}

		h.ServeHTTP(w, r)
	})
}
