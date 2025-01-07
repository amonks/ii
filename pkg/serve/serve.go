package serve

import (
	"bytes"
	"net/http"
	"path"
	"path/filepath"
	"time"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"monks.co/pkg/env"
)

func StaticServer(staticDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		Static(w, req, staticDir)
	})
}

func Static(w http.ResponseWriter, req *http.Request, staticDir string) {
	path := filepath.Join(staticDir, path.Base(req.URL.Path))
	http.ServeFile(w, req, path)
}

func JSServer(tsFilePath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ServeJS(w, req, tsFilePath)
	})
}

func ServeJS(w http.ResponseWriter, req *http.Request, tsfilepath string) {
	result := esbuild.Build(esbuild.BuildOptions{
		EntryPoints: []string{env.InMonksRoot("apps/map/ts/index.ts")},
		Bundle:      true,
		Write:       false,
	})
	if len(result.Errors) > 0 {
		InternalServerErrorf(w, req, "%s", result.Errors[0].Text)
		return
	}
	if len(result.OutputFiles) != 1 {
		InternalServerErrorf(w, req, "failed to produce js file")
		return
	}
	http.ServeContent(w, req, "index.js", time.Now(), bytes.NewReader(result.OutputFiles[0].Contents))
}
