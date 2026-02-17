package auth

import (
	"fmt"
	"net/http"

	"tailscale.com/client/local"
)

func GetRequester(r *http.Request) (bool, string) {
	addr := r.RemoteAddr

	lc := new(local.Client)
	who, err := lc.WhoIs(r.Context(), addr)
	if err != nil {
		return false, ""
	}

	return true, fmt.Sprintf("%s (@%s)", who.UserProfile.DisplayName, who.Node.ComputedName)
}

func InternalHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasAuth, _ := GetRequester(r)
		if !hasAuth {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(http.StatusText(http.StatusUnauthorized)))
			return
		}
		next.ServeHTTP(w, r)
	})
}
