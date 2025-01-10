package serve

import "net/http"

type Mux struct {
	http.ServeMux
}

func NewMux() *Mux {
	return &Mux{}
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("x-mux-route", req.Pattern)
	m.ServeMux.ServeHTTP(w, req)
}

