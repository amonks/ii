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

var htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - Delta Green Notes</title>
    <style>
        @font-face {
            font-family: 'Triplicate';
            src: url('/fonts/Triplicate OT A Regular.otf') format('opentype');
            font-weight: normal;
            font-style: normal;
        }
        @font-face {
            font-family: 'Triplicate';
            src: url('/fonts/Triplicate OT A Bold.otf') format('opentype');
            font-weight: bold;
            font-style: normal;
        }
        @font-face {
            font-family: 'Triplicate';
            src: url('/fonts/Triplicate OT A Italic.otf') format('opentype');
            font-weight: normal;
            font-style: italic;
        }
        @font-face {
            font-family: 'Triplicate';
            src: url('/fonts/Triplicate OT A Bold Italic.otf') format('opentype');
            font-weight: bold;
            font-style: italic;
        }
        body {
            font-family: 'Triplicate', 'Courier New', monospace;
            line-height: 1.6;
            max-width: 900px;
            margin: 0 auto;
            padding: 20px;
            background: #FAF9F6;
            color: #000;
        }
        h1 {
            color: #000;
            border-bottom: 2px solid #000;
            padding-bottom: 0.5em;
            margin-top: 1.5em;
        }
        h2 {
            color: #000;
            border-bottom: 1px solid #ccc;
            padding-bottom: 0.3em;
            margin-top: 1.5em;
        }
        h3, h4, h5, h6 {
            color: #000;
            margin-top: 1.2em;
        }
        a {
            color: #0066cc;
            text-decoration: none;
        }
        a:hover {
            text-decoration: underline;
            color: #0052a3;
        }
        pre {
            background: #f5f4f0;
            border: 1px solid #ddd;
            padding: 15px;
            border-radius: 4px;
            overflow-x: auto;
            font-family: 'Triplicate', 'Courier New', monospace;
        }
        code {
            background: #f5f4f0;
            padding: 2px 6px;
            border-radius: 3px;
            font-family: 'Triplicate', 'Courier New', monospace;
            font-size: 0.9em;
        }
        pre code {
            background: none;
            padding: 0;
        }
        blockquote {
            border-left: 4px solid #333;
            padding-left: 15px;
            margin: 1em 0;
            color: #555;
        }
        table {
            border-collapse: collapse;
            width: 100%;
            margin: 1em 0;
        }
        table th, table td {
            border: 1px solid #ccc;
            padding: 8px 12px;
            text-align: left;
        }
        table th {
            background: #f5f4f0;
            color: #000;
            font-weight: bold;
        }
        table tr:nth-child(even) {
            background: #f9f8f5;
        }
        .nav {
            margin-bottom: 30px;
            padding-bottom: 10px;
            border-bottom: 2px solid #ccc;
        }
        .file-list {
            list-style: none;
            padding: 0;
        }
        .file-list li {
            margin: 10px 0;
            padding: 5px 0;
        }
        .file-list a {
            font-size: 1.1em;
        }
        .content {
            line-height: 1.7;
        }
        .file-path {
            font-family: 'Triplicate', 'Courier New', monospace;
            font-size: 0.85em;
            color: #666;
            margin-bottom: 1.5em;
            display: block;
        }
        ul, ol {
            color: #000;
            margin: 1em 0;
            padding-left: 2em;
        }
        li {
            margin: 0.5em 0;
        }
        ul ul, ol ol, ul ol, ol ul {
            margin: 0.5em 0;
        }
        hr {
            border: none;
            border-top: 1px solid #ccc;
            margin: 2em 0;
        }
        strong {
            color: #000;
            font-weight: bold;
        }
        em {
            color: #000;
            font-style: italic;
        }
        /* Task list styling */
        .task-list-item {
            list-style: none;
            margin-left: -1.5em;
        }
        .task-list-item input[type="checkbox"] {
            margin-right: 0.5em;
        }
    </style>
</head>
<body>
    <div class="nav">
        <a href="/">📁 All Files</a>
    </div>
    {{if .IsIndex}}
    <h1>Delta Green Campaign Notes</h1>
    <ul class="file-list">
        {{range .Files}}
        <li>📄 <a href="/{{.}}">{{.}}</a></li>
        {{end}}
    </ul>
    {{else}}
    <div class="file-path">📄 {{.Path}}</div>
    <div class="content">{{.Content}}</div>
    {{end}}
</body>
</html>`

type PageData struct {
	Title   string
	Path    string
	Content template.HTML
	IsIndex bool
	Files   []string
}

func main() {
	http.HandleFunc("/fonts/", serveFonts)
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

func handleRequest(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("page").Parse(htmlTemplate))

	path := strings.TrimPrefix(r.URL.Path, "/")

	// If root, show index
	if path == "" {
		files, err := fs.Glob(markdownFiles, "notes/**/*.md")
		if err != nil {
			http.Error(w, "Error reading files", http.StatusInternalServerError)
			return
		}

		// Also get root level markdown files
		rootFiles, err := fs.Glob(markdownFiles, "notes/*.md")
		if err == nil {
			files = append(files, rootFiles...)
		}

		// Clean up paths for display
		var cleanFiles []string
		for _, f := range files {
			cleanPath := strings.TrimPrefix(f, "notes/")
			cleanFiles = append(cleanFiles, cleanPath)
		}

		sort.Strings(cleanFiles)

		data := PageData{
			Title:   "Index",
			IsIndex: true,
			Files:   cleanFiles,
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
