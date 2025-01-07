package imgres

import (
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"monks.co/pkg/env"
	"monks.co/pkg/image"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var SlugContextKey parser.ContextKey = parser.NewContextKey()
var DirContextKey parser.ContextKey = parser.NewContextKey()

// Imgres is a goldmark extension that adapts images to use the srcset
// attribute.
var Imgres = &imgres{}

type imgres struct{}

var _ goldmark.Extender = &imgres{}

func (*imgres) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(&astTransformer{}, -10000),
	))
}

type astTransformer struct{}

var _ parser.ASTTransformer = &astTransformer{}

func (*astTransformer) Transform(doc *ast.Document, r text.Reader, ctx parser.Context) {
	slug := ctx.Get(SlugContextKey).(string)
	dir := ctx.Get(DirContextKey).(string)

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if n.Kind() != ast.KindImage {
			return ast.WalkContinue, nil
		}

		imgNode := n.(*ast.Image)

		filename := string(imgNode.Destination)

		imgPath := env.InMonksRoot("writing", dir, filename)
		f, err := os.Open(imgPath)
		if err != nil {
			err := fmt.Errorf("opening '%s' for post '%s': %w", imgPath, slug, err)
			fmt.Println("fileread error", err)
			return ast.WalkStop, err
		}

		image, _, err := image.DecodeConfig(f)
		if err != nil {
			err := fmt.Errorf("decoding '%s' for post '%s': %w", imgPath, slug, err)
			fmt.Println("image decoding error", err)
			return ast.WalkStop, err
		}

		serverpath := filepath.Join("./media", filename)
		sizes := []string{fmt.Sprintf("%s %dw", serverpath, image.Width)}
		for w := 100; w <= image.Width; w += 100 {
			sizes = append(sizes, fmt.Sprintf("%s?width=%d %dw", serverpath, w, w))
		}

		imgNode.Destination = []byte(serverpath)
		imgNode.SetAttributeString("srcset", strings.Join(sizes, ", "))
		imgNode.SetAttributeString("sizes", fmt.Sprintf("calc(40vh * %f)", float64(image.Width)/float64(image.Height)))

		parent := imgNode.Parent()
		if parent.Kind() == ast.KindLink {
			return ast.WalkContinue, nil
		}

		anchor := ast.NewLink()
		anchor.SetAttributeString("target", "_blank")
		anchor.Destination = []byte(serverpath)
		parent.ReplaceChild(parent, imgNode, anchor)
		anchor.AppendChild(anchor, imgNode)

		return ast.WalkContinue, nil
	})
}
