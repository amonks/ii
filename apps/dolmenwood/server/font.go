package server

import (
	_ "embed"
	"net/http"
)

//go:embed ATKyriosStandard-Medium.woff2
var kyriosFont []byte

//go:embed martina-plantijn-bold.woff2
var martinaBoldFont []byte

//go:embed martina-plantijn-regular.woff2
var martinaRegularFont []byte

//go:embed AtTextual-Retina.woff2
var atTextualFont []byte

func (s *Server) handleFontKyrios(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "font/woff2")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Write(kyriosFont)
}

func (s *Server) handleFontMartinaBold(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "font/woff2")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Write(martinaBoldFont)
}

func (s *Server) handleFontMartinaRegular(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "font/woff2")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Write(martinaRegularFont)
}

func (s *Server) handleFontAtTextual(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "font/woff2")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Write(atTextualFont)
}
