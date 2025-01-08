package main

import (
	"fmt"
	"image"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"monks.co/apps/writing/templates"
	"monks.co/pkg/gzip"
	"monks.co/pkg/ports"
	"monks.co/pkg/posts"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"

	"github.com/a-h/templ"
	"github.com/nao1215/imaging"
)

func main() {
	if err := run(); err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx := sigctx.New()

	posts, err := posts.Load("./writing")
	if err != nil {
		return fmt.Errorf("loading posts: %w", err)
	}

	mux := http.NewServeMux()
	// mux.HandleFunc("/{$}", func(w http.ResponseWriter, req *http.Request) {
	// 	h := templ.Handler(templates.Index(posts))
	// 	h.ServeHTTP(w, req)
	// })
	mux.HandleFunc("/{slug}", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, fmt.Sprintf("/writing/%s/", req.PathValue("slug")), http.StatusMovedPermanently)
	})
	mux.HandleFunc("/{slug}/{$}", func(w http.ResponseWriter, req *http.Request) {
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
	mux.HandleFunc("/{slug}/media/{mediafilename}", func(w http.ResponseWriter, req *http.Request) {
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
			serve.Errorf(w, req, http.StatusInternalServerError, "opening '%s': %w", media.Path, err)
			return
		}
		header, _, err := image.DecodeConfig(f)
		if err != nil {
			serve.Errorf(w, req, http.StatusInternalServerError, "decoding '%s': %w", media.Path, err)
			return
		}
		originalWidth := header.Width
		_ = f.Close()
		if int(width) >= originalWidth {
			http.ServeFile(w, req, media.Path)
			return
		}

		img, err := imaging.Open(media.Path, imaging.AutoOrientation(true))
		if err != nil {
			serve.Errorf(w, req, http.StatusInternalServerError, "opening image '%s' on post '%s'", mediafilename, slug)
		}

		resized := imaging.Resize(img, int(width), 0, imaging.Box)

		w.Header().Add("Cache-Control", "max-age=31536000, immutable")

		switch filepath.Ext(mediafilename) {
		case ".jpg", ".jpeg":
			w.Header().Add("Content-Type", "image/jpeg")
			if err := imaging.Encode(w, resized, imaging.JPEG); err != nil {
				log.Printf("jpeg encoding error on '%s': %w", mediafilename, err)
			}

		case ".png":
			w.Header().Add("Content-Type", "image/png")
			if err := imaging.Encode(w, resized, imaging.PNG); err != nil {
				log.Printf("png encoding error on '%s': %w", mediafilename, err)
			}

		default:
			serve.Errorf(w, req, http.StatusInternalServerError, "unsupported extension on '%s' on post '%s'", mediafilename, slug)
		}
	})

	port := ports.Apps["writing"]
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
		return err
	}
	return nil
}
