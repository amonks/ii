package dbserver

import (
	"net/http"

	"monks.co/pkg/util"
)

func (s *DBServer) Errorf(w http.ResponseWriter, req *http.Request, statusCode int, msg string, args ...interface{}) {
	util.HTTPError(s.name, w, req, statusCode, msg, args...)
}

func (s *DBServer) Error(w http.ResponseWriter, req *http.Request, statusCode int, err error) {
	s.Errorf(w, req, statusCode, "%s", err)
}

func (s *DBServer) InternalServerErrorf(w http.ResponseWriter, req *http.Request, msg string, args ...interface{}) {
	s.Errorf(w, req, http.StatusInternalServerError, msg, args...)
}

func (s *DBServer) InternalServerError(w http.ResponseWriter, req *http.Request, err error) {
	s.Error(w, req, http.StatusInternalServerError, err)
}
