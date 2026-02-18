package serve

import (
	"net/http"
)

type Mux struct {
	http.ServeMux
}

func NewMux() *Mux {
	return &Mux{}
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	bp := BasePath(req)
	ctx := WithBasePath(req.Context(), bp)
	req = req.WithContext(ctx)
	m.ServeMux.ServeHTTP(&smuggler{w, false, req}, req)
}

type smuggler struct {
	http.ResponseWriter
	hasSmuggled bool
	req         *http.Request
}

func (sm *smuggler) Write(bs []byte) (int, error) {
	if !sm.hasSmuggled {
		sm.smuggle()
	}
	return sm.ResponseWriter.Write(bs)
}

func (sm *smuggler) smuggle() {
	sm.hasSmuggled = true
	sm.ResponseWriter.Header().Set("x-mux-route", sm.req.Pattern)
}

func (sm *smuggler) WriteHeader(code int) {
	sm.smuggle()
	sm.ResponseWriter.WriteHeader(code)
}
