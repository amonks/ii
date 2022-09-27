package dbserver

import (
	"bytes"
	"net/http"
	"path"
	"time"

	"crawshaw.io/sqlite"
	"github.com/evanw/esbuild/pkg/api"
)

func (s *DBServer) StaticServer(staticDir string) func(*sqlite.Conn, http.ResponseWriter, *http.Request) {
	return func(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
		s.ServeStatic(w, req, staticDir)
	}
}

func (s *DBServer) ServeStatic(w http.ResponseWriter, req *http.Request, staticDir string) {
	path := path.Join(staticDir, path.Base(req.URL.Path))
	http.ServeFile(w, req, path)
}

func (s *DBServer) JSServer(tsFilePath string) func(*sqlite.Conn, http.ResponseWriter, *http.Request) {
	return func(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
		s.ServeJS(w, req, tsFilePath)
	}
}

func (s *DBServer) ServeJS(w http.ResponseWriter, req *http.Request, tsfilepath string) {
	result := api.Build(api.BuildOptions{
		EntryPoints: []string{"./places/ts/index.ts"},
		Bundle:      true,
		Write:       false,
	})
	if len(result.Errors) > 0 {
		s.InternalServerErrorf(w, req, "%s", result.Errors[0])
		return
	}
	if len(result.OutputFiles) != 1 {
		s.InternalServerErrorf(w, req, "failed to produce js file")
		return
	}
	http.ServeContent(w, req, "index.js", time.Now(), bytes.NewReader(result.OutputFiles[0].Contents))
}
