# Posts Package

## Overview

Loads markdown posts from a directory on disk, parses YAML front matter,
renders to HTML with responsive image support. Used by the
[writing](writing.md) app.

Code: [pkg/posts/](../pkg/posts/)

## Post Loading

`Load(ctx) ([]*Post, error)` reads all posts from
`$MONKS_ROOT/apps/writing/writing/`. Two formats:

- **Flat file**: `YYYY-MM-DD-slug.md`
- **Directory**: `YYYY-MM-DD-slug/post.md` with sibling media files

Front matter fields: `title` (string), `subtitle` (string), `published`
(bool). Posts with `published: false` are marked as drafts.

## Markdown Rendering

Uses `goldmark` with extensions:
- `frontmatter` — YAML front matter
- `anchor` — heading anchors prepended with `#`
- `linkify`, `footnote`, `typographer`
- `pkg/markdown/ampersand` — `&` → `<span class="ampersand">`
- `pkg/markdown/imgres` — responsive `srcset` and `sizes` attributes

## Post Type

```go
type Post struct {
    Slug      string
    Title     string
    Subtitle  string
    Date      time.Time
    Published bool
    HTML      template.HTML
    MediaDir  string  // empty for flat-file posts
}
```

## Dependencies

- `pkg/env` — for `$MONKS_ROOT`
- `pkg/markdown/ampersand`, `pkg/markdown/imgres`
- `github.com/yuin/goldmark` and extensions
