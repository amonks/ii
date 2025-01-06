package posts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/a-h/templ"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/anchor"
	"go.abhg.dev/goldmark/frontmatter"
	"monks.co/pkg/markdown/ampersand"
	"monks.co/pkg/markdown/imgres"
)

type Post struct {
	Slug     string
	Title    string
	Subtitle string
	Date     string
	IsDraft  bool
	HTML     string
	Media    map[string]Media
}

type Media struct {
	Filename string
	Path     string
}

var _ templ.Component = &Post{}

func (p *Post) Render(ctx context.Context, w io.Writer) error {
	_, err := io.WriteString(w, p.HTML)
	return err
}

type PostFrontmatter struct {
	Published bool
	Title     string
	Subtitle  string
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
		ampersand.Ampersand,
		imgres.Imgres,
		extension.Linkify,
		extension.Footnote,
		extension.Typographer,
		&frontmatter.Extender{},
		&anchor.Extender{
			Position: anchor.Before,
			Texter:   anchor.Text("#"),
		},
	),
	goldmark.WithRendererOptions(
		html.WithUnsafe(),
	),
)

func Load(dir string) (*Posts, error) {
	ps, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading posts dir: %w", err)
	}

	posts := &Posts{bySlug: map[string]*Post{}}
	for _, p := range ps {
		var date, slug, mdpath string
		media := map[string]Media{}
		if p.IsDir() {
			dirname := p.Name()
			date = dirname[:10]
			slug = dirname[11:]
			mdpath = filepath.Join(dir, dirname, "post.md")
			ps, err := os.ReadDir(filepath.Join(dir, dirname))
			if err != nil {
				return nil, fmt.Errorf("reading dir of post '%s': %w", p.Name(), err)
			}
			for _, p := range ps {
				filename := p.Name()
				if filename == "post.md" {
					continue
				}
				media[filename] = Media{
					Filename: filename,
					Path:     filepath.Join(dir, dirname, filename),
				}
			}
		} else {
			filename := p.Name()
			if !strings.HasSuffix(filename, ".md") {
				continue
			}
			date = filename[:10]
			slug = filename[11 : len(filename)-3]
			mdpath = filepath.Join(dir, filename)
		}

		bs, err := os.ReadFile(mdpath)
		if err != nil {
			return nil, fmt.Errorf("reading post file '%s': %w", mdpath, err)
		}

		post, err := ReadPost(date, slug, bs)
		post.Media = media
		posts.List = append(posts.List, post)
		posts.bySlug[post.Slug] = post
	}
	sort.Slice(posts.List, func(a, b int) bool {
		return posts.List[a].Date > posts.List[b].Date
	})

	return posts, nil
}

func ReadPost(date, slug string, file []byte) (*Post, error) {
	parseCtx := parser.NewContext()
	parseCtx.Set(imgres.SlugContextKey, slug)
	parseCtx.Set(imgres.DirContextKey, date+"-"+slug)

	var buf bytes.Buffer
	var fm PostFrontmatter

	if err := gm.Convert(file, &buf, parser.WithContext(parseCtx)); err != nil {
		return nil, err
	}
	d := frontmatter.Get(parseCtx)
	if err := d.Decode(&fm); err != nil {
		return nil, err
	}

	return &Post{
		Date:     date,
		Slug:     slug,
		Title:    fm.Title,
		Subtitle: fm.Subtitle,
		IsDraft:  !fm.Published,
		HTML:     buf.String(),
	}, nil
}
