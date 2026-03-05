package publish

import (
	"golang.org/x/mod/modfile"
)

// RewriteGoMod adds or updates require directives in a go.mod file.
// requires maps module path to version (e.g., "monks.co/pkg/migrate" → "v0.20260305143022.0").
func RewriteGoMod(data []byte, requires map[string]string) ([]byte, error) {
	if len(requires) == 0 {
		return data, nil
	}
	f, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, err
	}
	for mod, ver := range requires {
		if err := f.AddRequire(mod, ver); err != nil {
			return nil, err
		}
	}
	return f.Format()
}
