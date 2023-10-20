package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/a-h/templ"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"go.abhg.dev/goldmark/frontmatter"
)

type Posts struct {
	list   []*Post
	bySlug map[string]*Post
}

var _ templ.Component = &Post{}

type Post struct {
	Slug    string
	Title   string
	Date    string
	IsDraft bool
	HTML    string
}

type PostFrontmatter struct {
	Draft bool
	Title string
	Date  string
}

func (ps *Posts) Get(slug string) *Post {
	return ps.bySlug[slug]
}

func LoadPosts(dir string) (*Posts, error) {
	gm := goldmark.New(
		goldmark.WithExtensions(
			&frontmatter.Extender{},
		),
	)
	ps, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var posts *Posts
	for _, p := range ps {
		if p.IsDir() {
			continue
		}
		name := p.Name()
		bs, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		var fm PostFrontmatter
		ctx := parser.NewContext()
		if err := gm.Convert(bs, &buf, parser.WithContext(ctx)); err != nil {
			return nil, err
		}
		d := frontmatter.Get(ctx)
		d.Decode(&fm)
		posts.list = append(posts.list, &Post{
			HTML:    buf.String(),
			Slug:    strings.TrimSuffix(name, ".md"),
			Title:   fm.Title,
			IsDraft: fm.Draft,
			Date:    fm.Date,
		})
	}

	posts.bySlug = make(map[string]*Post, len(posts.list))
	for _, p := range posts.list {
		posts.bySlug[p.Slug] = p
	}

	return posts, nil
}

func (p *Post) Render(ctx context.Context, w io.Writer) error {
	_, err := io.WriteString(w, p.HTML)
	return err
}
