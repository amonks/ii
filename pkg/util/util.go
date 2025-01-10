package util

import (
	"embed"
	"html/template"
	"path"
)

func ReadTemplates(fs embed.FS, root string) (map[string]*template.Template, error) {
	templates := make(map[string]*template.Template)

	files, err := fs.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		t, err := template.ParseFS(fs, path.Join("templates", name))
		if err != nil {
			return nil, err
		}
		templates[name] = t
	}

	return templates, nil
}
