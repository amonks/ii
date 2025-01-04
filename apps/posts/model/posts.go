package model

import (
	"bytes"
	"context"
	"io"
	"iter"
	"os"
	"path/filepath"
	"sort"

	"github.com/a-h/templ"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/anchor"
	"go.abhg.dev/goldmark/frontmatter"
)

type Post struct {
	Slug    string
	Title   string
	Date    string
	IsDraft bool
	HTML    string
}

var _ templ.Component = &Post{}

func (p *Post) Render(ctx context.Context, w io.Writer) error {
	_, err := io.WriteString(w, p.HTML)
	return err
}

type PostFrontmatter struct {
	Published bool
	Title     string
}

type Posts struct {
	List   []*Post
	bySlug map[string]*Post
}

func (ps *Posts) Get(slug string) *Post {
	return ps.bySlug[slug]
}

func (ps *Posts) All() iter.Seq[*Post] {
	return func(yield func(*Post) bool) {
		for _, post := range ps.List {
			if !yield(post) {
				break
			}
		}
	}
}

var gm = goldmark.New(
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithExtensions(
		extension.Linkify,
		extension.Footnote,
		extension.Typographer,
		&frontmatter.Extender{},
		&anchor.Extender{},
	),
	goldmark.WithRendererOptions(
		html.WithUnsafe(),
	),
)

func LoadPosts(dir string) (*Posts, error) {
	ps, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	posts := &Posts{bySlug: map[string]*Post{}}
	for _, p := range ps {
		var date, slug, mdpath string
		if p.IsDir() {
			dirname := p.Name()
			date = dirname[:10]
			slug = dirname[11:]
			mdpath = filepath.Join(dir, dirname, "post.md")
		} else {
			filename := p.Name()
			date = filename[:10]
			slug = filename[11 : len(filename)-3]
			mdpath = filepath.Join(dir, filename)
		}

		bs, err := os.ReadFile(mdpath)
		if err != nil {
			return nil, err
		}

		post, err := ReadPost(date, slug, bs)
		posts.List = append(posts.List, post)
		posts.bySlug[post.Slug] = post
	}
	sort.Slice(posts.List, func(a, b int) bool {
		return posts.List[a].Date > posts.List[b].Date
	})

	return posts, nil
}

func ReadPost(date, slug string, file []byte) (*Post, error) {
	var buf bytes.Buffer
	var fm PostFrontmatter
	parse := parser.NewContext()
	if err := gm.Convert(file, &buf, parser.WithContext(parse)); err != nil {
		return nil, err
	}
	d := frontmatter.Get(parse)
	if err := d.Decode(&fm); err != nil {
		return nil, err
	}

	return &Post{
		Date:    date,
		Slug:    slug,
		Title:   fm.Title,
		IsDraft: !fm.Published,
		HTML:    buf.String(),
	}, nil
}
