package model

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/a-h/templ"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"go.abhg.dev/goldmark/frontmatter"
)

type Posts struct {
	List   []*Post
	BySlug map[string]*Post
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
}

func (ps *Posts) Get(slug string) *Post {
	return ps.BySlug[slug]
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

	posts := &Posts{}
	for _, p := range ps {
		if p.IsDir() {
			continue
		}
		filename := p.Name()
		date, slug := filename[:10], filename[:len(filename)-3]
		bs, err := os.ReadFile(filepath.Join(dir, filename))
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
		posts.List = append(posts.List, &Post{
			Date:    date,
			Slug:    slug,
			Title:   fm.Title,
			IsDraft: fm.Draft,
			HTML:    buf.String(),
		})
	}

	posts.BySlug = make(map[string]*Post, len(posts.List))
	for _, p := range posts.List {
		posts.BySlug[p.Slug] = p
	}

	sort.Slice(posts.List, func(a, b int) bool {
		return posts.List[a].Date > posts.List[b].Date
	})

	return posts, nil
}

func (p *Post) Render(ctx context.Context, w io.Writer) error {
	_, err := io.WriteString(w, p.HTML)
	return err
}
