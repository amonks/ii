package main

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

//go:embed notes
var markdownFiles embed.FS

//go:embed fonts
var fontFiles embed.FS

//go:embed template.html
var templateFile embed.FS

type PageData struct {
	Title    string
	Path     string
	Content  template.HTML
	IsIndex  bool
	Files    []string
	TreeHTML template.HTML
}

type TreeNode struct {
	Name     string
	Path     string
	IsDir    bool
	Children []*TreeNode
}

func buildTree(files []string) *TreeNode {
	root := &TreeNode{Name: "notes", IsDir: true, Children: []*TreeNode{}}

	for _, file := range files {
		parts := strings.Split(file, "/")
		current := root

		for i, part := range parts {
			isLast := i == len(parts)-1

			// Find or create child node
			var child *TreeNode
			for _, c := range current.Children {
				if c.Name == part {
					child = c
					break
				}
			}

			if child == nil {
				child = &TreeNode{
					Name:  part,
					IsDir: !isLast,
					Path:  file,
				}
				current.Children = append(current.Children, child)
			}

			current = child
		}
	}

	// Sort children at each level
	sortTree(root)
	return root
}

func sortTree(node *TreeNode) {
	sort.Slice(node.Children, func(i, j int) bool {
		// Directories first, then files
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return node.Children[i].Name < node.Children[j].Name
	})

	for _, child := range node.Children {
		if child.IsDir {
			sortTree(child)
		}
	}
}

func renderTree(node *TreeNode, prefix string, isLast bool) string {
	if node.Name == "notes" {
		// Root node
		var result strings.Builder
		result.WriteString("<div class=\"tree\">notes/\n")
		for i, child := range node.Children {
			result.WriteString(renderTree(child, "", i == len(node.Children)-1))
		}
		result.WriteString("</div>")
		return result.String()
	}

	var result strings.Builder

	// Draw the tree structure
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	result.WriteString(prefix + connector)

	if node.IsDir {
		result.WriteString("📁 " + node.Name + "/\n")

		// Add children
		newPrefix := prefix
		if isLast {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}

		for i, child := range node.Children {
			result.WriteString(renderTree(child, newPrefix, i == len(node.Children)-1))
		}
	} else {
		result.WriteString("📄 <a href=\"/" + node.Path + "\">" + node.Name + "</a>\n")
	}

	return result.String()
}

func main() {
	http.HandleFunc("/fonts/", serveFonts)
	http.HandleFunc("/search", handleSearch)
	http.HandleFunc("/", handleRequest)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func serveFonts(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	content, err := fontFiles.ReadFile(path)
	if err != nil {
		http.Error(w, "Font not found", http.StatusNotFound)
		return
	}

	// Set content type for fonts
	if strings.HasSuffix(path, ".otf") {
		w.Header().Set("Content-Type", "font/otf")
	} else if strings.HasSuffix(path, ".ttf") {
		w.Header().Set("Content-Type", "font/ttf")
	} else if strings.HasSuffix(path, ".woff") {
		w.Header().Set("Content-Type", "font/woff")
	} else if strings.HasSuffix(path, ".woff2") {
		w.Header().Set("Content-Type", "font/woff2")
	}

	w.Write(content)
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		w.Write([]byte(""))
		return
	}

	var results strings.Builder
	searchLower := strings.ToLower(query)
	matchCount := 0
	maxResults := 50 // Limit results for performance

	// Walk through all markdown files
	err := fs.WalkDir(markdownFiles, "notes", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Stop if we've hit the limit
		if matchCount >= maxResults {
			return nil
		}

		// Only process markdown files
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			cleanPath := strings.TrimPrefix(path, "notes/")

			// Skip character sheets
			if strings.HasPrefix(cleanPath, "character_sheets/") {
				return nil
			}

			// Read file content
			content, err := markdownFiles.ReadFile(path)
			if err != nil {
				return nil // Skip files we can't read
			}

			// Search line by line
			lines := strings.Split(string(content), "\n")
			for i, line := range lines {
				if matchCount >= maxResults {
					break
				}
				if strings.Contains(strings.ToLower(line), searchLower) {
					matchCount++
					lineNum := i + 1
					// Escape HTML in the line
					escapedLine := template.HTMLEscapeString(line)
					// Build fzf-style result - pass the matched line text as query param
					results.WriteString(fmt.Sprintf(
						"<div class=\"search-result\"><a href=\"/%s?highlight=%s\">%s:%d</a>: %s</div>\n",
						cleanPath, template.URLQueryEscaper(strings.TrimSpace(line)), cleanPath, lineNum, escapedLine,
					))
				}
			}
		}

		return nil
	})

	if err != nil {
		http.Error(w, "Error searching files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if matchCount == 0 {
		w.Write([]byte("<div class=\"search-result\">No results found</div>"))
	} else {
		if matchCount >= maxResults {
			results.WriteString(fmt.Sprintf("<div class=\"search-result search-meta\">Showing first %d results...</div>\n", maxResults))
		}
		w.Write([]byte(results.String()))
	}
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	templateContent, err := templateFile.ReadFile("template.html")
	if err != nil {
		http.Error(w, "Error reading template", http.StatusInternalServerError)
		return
	}
	tmpl := template.Must(template.New("page").Parse(string(templateContent)))

	path := strings.TrimPrefix(r.URL.Path, "/")

	// If root, show index
	if path == "" {
		var cleanFiles []string

		// Walk the entire notes directory
		err := fs.WalkDir(markdownFiles, "notes", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Only process markdown files
			if !d.IsDir() && strings.HasSuffix(path, ".md") {
				cleanPath := strings.TrimPrefix(path, "notes/")
				// Skip character sheets
				if !strings.HasPrefix(cleanPath, "character_sheets/") {
					cleanFiles = append(cleanFiles, cleanPath)
				}
			}

			return nil
		})

		if err != nil {
			http.Error(w, "Error reading files", http.StatusInternalServerError)
			return
		}

		sort.Strings(cleanFiles)

		// Build tree structure
		tree := buildTree(cleanFiles)
		treeHTML := renderTree(tree, "", false)

		data := PageData{
			Title:    "Index",
			IsIndex:  true,
			Files:    cleanFiles,
			TreeHTML: template.HTML(treeHTML),
		}

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, "Error rendering template", http.StatusInternalServerError)
		}
		return
	}

	// Serve markdown file
	if !strings.HasSuffix(path, ".md") {
		path += ".md"
	}

	// Block access to character sheets
	if strings.HasPrefix(path, "character_sheets/") {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Add notes/ prefix for embed.FS
	fullPath := "notes/" + path

	content, err := markdownFiles.ReadFile(fullPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Convert markdown to HTML using goldmark
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert(content, &buf); err != nil {
		http.Error(w, "Error converting markdown", http.StatusInternalServerError)
		return
	}

	contentStr := buf.String()

	data := PageData{
		Title:   strings.TrimSuffix(filepath.Base(path), ".md"),
		Path:    path,
		Content: template.HTML(contentStr),
		IsIndex: false,
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, fmt.Sprintf("Error rendering template: %v", err), http.StatusInternalServerError)
	}
}
