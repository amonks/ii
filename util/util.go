package util

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path"
)

func ReadTemplates(fs embed.FS, root string) (map[string]*template.Template, error) {
	fmt.Println("init tmplates")
	templates := make(map[string]*template.Template)

	files, err := fs.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		fmt.Println("file", f.Name())
		if f.IsDir() {
			continue
		}
		name := f.Name()
		fmt.Println("loaded templ", name)
		t, err := template.ParseFS(fs, path.Join("templates", name))
		if err != nil {
			return nil, err
		}
		templates[name] = t
	}

	return templates, nil
}

func HTTPError(app string, w http.ResponseWriter, req *http.Request, code int, message string, args ...interface{}) {
	msg := fmt.Sprintf(message, args...)
	log.Printf("[%d] %s/%s: %s", code, app, req.URL.Path, msg)
	http.Error(w, http.StatusText(code), code)
}
