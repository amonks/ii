package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/sync/semaphore"
	"monks.co/apps/writing/templates"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/posts"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/serve"
	"monks.co/pkg/tailnet"
	"monks.co/pkg/sigctx"

	"github.com/a-h/templ"
	"github.com/nao1215/imaging"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		log.Println(err.Error())
		os.Exit(1)
	}
}

func run() error {
	reqlog.SetupLogging()

	ctx := sigctx.New()
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}

	posts, err := posts.Load(ctx)
	if err != nil {
		return fmt.Errorf("loading posts: %w", err)
	}

	mux := serve.NewMux()
	// mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, req *http.Request) {
	// 	h := templ.Handler(templates.Index(posts))
	// 	h.ServeHTTP(w, req)
	// })
	mux.HandleFunc("GET /{slug}", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, fmt.Sprintf("/writing/%s/", req.PathValue("slug")), http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /{slug}/{$}", func(w http.ResponseWriter, req *http.Request) {
		slug := req.PathValue("slug")
		post := posts.Get(slug)
		if post == nil {
			serve.Errorf(w, req, http.StatusNotFound, "post '%s' not found", slug)
			return
		}
		component := templates.Post(post)
		h := templ.Handler(component)
		h.ServeHTTP(w, req)
	})
	transcodeLimiter := semaphore.NewWeighted(16)
	mux.HandleFunc("GET /{slug}/media/{mediafilename}", func(w http.ResponseWriter, req *http.Request) {
		slug := req.PathValue("slug")
		post := posts.Get(slug)
		if post == nil {
			serve.Errorf(w, req, http.StatusNotFound, "post '%s' not found", slug)
			return
		}
		mediafilename := req.PathValue("mediafilename")
		media, hasMedia := post.Media[mediafilename]
		if !hasMedia {
			serve.Errorf(w, req, http.StatusNotFound, "media '%s' not found on post '%s'", mediafilename, slug)
			return
		}

		widthStr := req.URL.Query().Get("width")
		if widthStr == "" {
			http.ServeFile(w, req, media.Path)
			return
		}
		width, err := strconv.ParseInt(widthStr, 10, 64)
		if err != nil {
			serve.Errorf(w, req, http.StatusBadRequest, "invalid width '%s': %w", widthStr, err)
			return
		}

		f, err := os.Open(media.Path)
		if err != nil {
			serve.InternalServerErrorf(w, req, "opening '%s': %w", media.Path, err)
			return
		}
		// f is closed after this next stanza
		header, _, err := image.DecodeConfig(f)
		if err != nil {
			serve.InternalServerErrorf(w, req, "decoding '%s': %w", media.Path, err)
			return
		}
		originalWidth := header.Width
		_ = f.Close()
		if int(width) >= originalWidth {
			http.ServeFile(w, req, media.Path)
			return
		}

		if err := transcodeLimiter.Acquire(req.Context(), 1); err != nil && !errors.Is(err, context.Canceled) {
			serve.InternalServerErrorf(w, req, "semaphore error: %w", err)
			return
		} else if err != nil {
			return
		}
		defer transcodeLimiter.Release(1)

		img, err := imaging.Open(media.Path, imaging.AutoOrientation(true))
		if err != nil {
			serve.InternalServerErrorf(w, req, "opening image '%s' on post '%s'", mediafilename, slug)
			return
		}

		resized := imaging.Resize(img, int(width), 0, imaging.Box)

		w.Header().Add("Cache-Control", "max-age=31536000, immutable")

		switch filepath.Ext(mediafilename) {
		case ".jpg", ".jpeg":
			w.Header().Add("Content-Type", "image/jpeg")
			if err := imaging.Encode(w, resized, imaging.JPEG); err != nil {
				serve.InternalServerErrorf(w, req, "jpeg encoding error on '%s': %s", mediafilename, err)
				return
			}

		case ".png":
			w.Header().Add("Content-Type", "image/png")
			if err := imaging.Encode(w, resized, imaging.PNG); err != nil {
				serve.InternalServerErrorf(w, req, "png encoding error on '%s': %s", mediafilename, err)
				return
			}

		default:
			serve.InternalServerErrorf(w, req, "unsupported extension on '%s' on post '%s'", mediafilename, slug)
			return
		}
	})

	if err := tailnet.ListenAndServe(ctx, reqlog.Middleware().ModifyHandler(gzip.Middleware(mux))); err != nil {
		return err
	}
	return nil
}
