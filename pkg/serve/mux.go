package serve

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"monks.co/pkg/errlogger"
)

type Mux struct {
	http.ServeMux
}

func NewMux() *Mux {
	return &Mux{}
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("panic: %v\n%s", r, debug.Stack())
			log.Printf("[500] %s: %s", req.URL.Path, msg)
			errlogger.Report(500, msg)
			http.Error(w, http.StatusText(500), 500)
		}
	}()
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
