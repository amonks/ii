# Writing

## Overview

Personal blog. Serves markdown posts (logs, weeknotes, photo essays, and
articles) with on-the-fly image resizing for responsive media.

Code: [apps/writing/](../apps/writing/)

## Post Format

Posts live in `apps/writing/writing/`. Two formats:

- **Flat file**: `YYYY-MM-DD-slug.md`
- **Directory**: `YYYY-MM-DD-slug/post.md` with sibling media files

YAML front matter fields: `title`, `subtitle`, `published` (bool). Posts
with `published: false` are drafts (hidden from index, but accessible by
direct URL).

## Markdown Rendering

Uses `goldmark` with extensions:
- `frontmatter`: YAML front matter parsing
- `anchor`: auto heading anchors
- `linkify`, `footnote`, `typographer`
- **`ampersand`** (`pkg/markdown/ampersand`): wraps bare `&` in
  `<span class="ampersand">` for typographic styling.
- **`imgres`** (`pkg/markdown/imgres`): rewrites `<img>` tags to use
  `srcset` with multiple widths (100px increments up to original) and
  computed `sizes` based on aspect ratio. Wraps non-linked images in
  `<a target="_blank">` anchors.

All posts are loaded into memory at startup (no hot reload).

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/{slug}` | 301 redirect to `/{slug}/` |
| GET | `/{slug}/` | Render post by slug; 404 if not found |
| GET | `/{slug}/media/{file}` | Serve media. `?width=N` triggers on-the-fly resize (max 16 concurrent, semaphore-limited). Resized images get `Cache-Control: max-age=31536000, immutable`. |

The index route (`GET /`) is currently commented out — posts are only
accessible by direct slug URL.

## Image Resizing

When `?width=N` is provided and N < original width, the image is resized
using `github.com/nao1215/imaging`. Supports JPEG and PNG. A semaphore
limits concurrent resize operations to 16.

## Typography

Custom fonts: Emeritus Display, Rules, Rules Gothic. The template includes
extensive inline CSS with responsive layout and `<photo-set>` custom element
styling for photo grid layouts.

## Deployment

Runs on **fly.io** (Chicago ORD). `shared-cpu-4x`, 2GB. The `writing/`
content directory is copied into the Docker image. Public access tier.
