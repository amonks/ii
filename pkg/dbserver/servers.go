package dbserver

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"crawshaw.io/sqlite"
	esbuild "github.com/evanw/esbuild/pkg/api"
	"monks.co/pkg/serve"
)

func (s *DBServer) StaticServer(staticDir string) func(_ *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	return func(_ *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
		serve.Static(w, req, staticDir)
	}
}

func (s *DBServer) JSServer(tsFilePath string) func(*sqlite.Conn, http.ResponseWriter, *http.Request) {
	return func(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
		s.ServeJS(w, req, tsFilePath)
	}
}

func (s *DBServer) ServeJS(w http.ResponseWriter, req *http.Request, tsfilepath string) {
	result := esbuild.Build(esbuild.BuildOptions{
		EntryPoints: []string{filepath.Join(os.Getenv("MONKS_ROOT"), "apps/map/ts/index.ts")},
		Bundle:      true,
		Write:       false,
	})
	if len(result.Errors) > 0 {
		s.InternalServerErrorf(w, req, "%s", result.Errors[0].Text)
		return
	}
	if len(result.OutputFiles) != 1 {
		s.InternalServerErrorf(w, req, "failed to produce js file")
		return
	}
	http.ServeContent(w, req, "index.js", time.Now(), bytes.NewReader(result.OutputFiles[0].Contents))
}


