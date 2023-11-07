package serve

import (
	"fmt"
	"log"
	"net/http"
)

func Errorf(w http.ResponseWriter, req *http.Request, statusCode int, msg string, args ...interface{}) {
	msg = fmt.Sprintf(msg, args...)
	log.Printf("[%d] %s: %s\n", statusCode, req.URL.Path, msg)
	http.Error(w, http.StatusText(statusCode), statusCode)
}

func Error(w http.ResponseWriter, req *http.Request, statusCode int, err error) {
	Errorf(w, req, statusCode, "%s", err)
}

func InternalServerErrorf(w http.ResponseWriter, req *http.Request, msg string, args ...interface{}) {
	Errorf(w, req, http.StatusInternalServerError, msg, args...)
}

func InternalServerError(w http.ResponseWriter, req *http.Request, err error) {
	Error(w, req, http.StatusInternalServerError, err)
}
