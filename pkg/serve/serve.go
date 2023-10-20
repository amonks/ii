package serve

import (
	"fmt"
	"net/http"
	"path"
	"path/filepath"
)

func StaticServer(staticDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		Static(w, req, staticDir)
	})
}

func Static(w http.ResponseWriter, req *http.Request, staticDir string) {
	path := filepath.Join(staticDir, path.Base(req.URL.Path))
	fmt.Println("path", path)
	http.ServeFile(w, req, path)
}
