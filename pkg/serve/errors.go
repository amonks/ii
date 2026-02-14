package serve

import (
	"fmt"
	"net/http"

	"monks.co/pkg/errlogger"
	"monks.co/pkg/reqlog"
)

func Errorf(w http.ResponseWriter, req *http.Request, statusCode int, msg string, args ...interface{}) {
	msg = fmt.Sprintf(msg, args...)
	reqlog.Set(req.Context(), "err.message", msg)
	if statusCode >= 500 && statusCode < 600 {
		errlogger.Report(statusCode, msg)
	}
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
