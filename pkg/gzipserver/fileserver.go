package gzipserver

import (
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/kevinpollet/nego"
)

// List of encodings we would prefer to use, in order of preference, best first.
var preferredEncodings = []string{"br", "gzip"}

// File extension to use for different encodings.
func extensionForEncoding(encname string) string {
	switch encname {
	case "gzip":
		return ".gz"
	case "br":
		return ".br"
	}
	return ""
}

// Function to negotiate the best content encoding
func negotiate(r *http.Request, available []string) string {
	return nego.NegotiateContentEncoding(r, available...)
}

type fileHandler struct {
	root FileSystem
}

// FileServer is a drop-in replacement for Go's standard http.FileServer
// which adds support for static resources precompressed with gzip, at
// the cost of removing the support for directory browsing.
//
// If file filename.ext has a compressed version filename.ext.gz alongside
// it, if the client indicates that it accepts gzip-compressed data, and
// if the .gz file can be opened, then the compressed version of the file
// will be sent to the client. Otherwise the request is passed on to
// http.ServeContent, and the raw (uncompressed) version is used.
//
// It is up to you to ensure that the compressed and uncompressed versions
// of files match and have sensible timestamps.
//
// Compressed or not, requests are fulfilled using http.ServeContent, and
// details like accept ranges and content-type sniffing are handled by that
// method.
func FileServer(root FileSystem) http.Handler {
	return &fileHandler{root}
}

func (f *fileHandler) openAndStat(path string) (http.File, os.FileInfo, error) {
	file, err := f.root.Open(path)
	var info os.FileInfo
	// This slightly weird variable reuse is so we can get 100% test coverage
	// without having to come up with a test file that can be opened, yet
	// fails to stat.
	if err == nil {
		info, err = file.Stat()
	}
	if err != nil {
		return file, nil, err
	}
	return file, info, nil
}

const (
	acceptEncodingHeader  = "Accept-Encoding"
	contentEncodingHeader = "Content-Encoding"
	contentLengthHeader   = "Content-Length"
	rangeHeader           = "Range"
	varyHeader            = "Vary"
)

// Find the best file to serve based on the client's Accept-Encoding, and which
// files actually exist on the filesystem. If no file was found that can satisfy
// the request, the error field will be non-nil.
func (f *fileHandler) findBestFile(w http.ResponseWriter, r *http.Request, fpath string) (http.File, os.FileInfo, error) {
	ae := r.Header.Get(acceptEncodingHeader)
	if ae == "" {
		return f.openAndStat(fpath)
	}
	// Got an accept header? See what possible encodings we can send by looking for files
	var available []string
	for _, posenc := range preferredEncodings {
		ext := extensionForEncoding(posenc)
		fname := fpath + ext
		if f.root.Exists(fname) {
			available = append(available, posenc)
		}
	}
	if len(available) == 0 {
		return f.openAndStat(fpath)
	}
	// Carry out standard HTTP negotiation
	negenc := negotiate(r, available)
	if negenc == "" {
		// If we fail to negotiate anything or if we negotiated the identity encoding, again try the base file
		return f.openAndStat(fpath)
	}
	ext := extensionForEncoding(negenc)
	if file, info, err := f.openAndStat(fpath + ext); err == nil {
		wHeader := w.Header()
		wHeader[contentEncodingHeader] = []string{negenc}
		wHeader.Add(varyHeader, acceptEncodingHeader)

		if len(r.Header[rangeHeader]) == 0 {
			// If not a range request then we can easily set the content length which the
			// Go standard library does not do if "Content-Encoding" is set.
			wHeader[contentLengthHeader] = []string{strconv.FormatInt(info.Size(), 10)}
		}
		return file, info, nil
	}

	// If all else failed, fall back to base file once again
	return f.openAndStat(fpath)
}

// tryFile attempts to open and stat a file, closing it and returning only the FileInfo if successful
func (f *fileHandler) tryFile(name string) (os.FileInfo, error) {
	file, info, err := f.openAndStat(name)
	if err != nil {
		return nil, err
	}
	file.Close()
	return info, nil
}

func (f *fileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
		r.URL.Path = upath
	}
	fpath := path.Clean(upath)

	// Check if this is a directory
	if file, info, err := f.openAndStat(fpath); err == nil {
		if info.IsDir() {
			file.Close()
			// Redirect if the directory name doesn't end in /
			if !strings.HasSuffix(r.URL.Path, "/") {
				localRedirect(w, r, path.Base(r.URL.Path)+"/")
				return
			}
			// Try to serve index.html from this directory
			fpath = path.Join(fpath, "index.html")
		} else {
			file.Close()
			// If we're serving a real file, check if it's an index.html
			// or if we're accessing a directory without a trailing slash
			_, file := path.Split(fpath)
			if file == "index.html" || f.root.Exists(path.Join(fpath, "index.html")) {
				localRedirect(w, r, path.Dir(r.URL.Path)+"/")
				return
			}
		}
	}

	// Find the best acceptable file, including trying uncompressed
	if file, info, err := f.findBestFile(w, r, fpath); err == nil {
		http.ServeContent(w, r, fpath, info.ModTime(), file)
		file.Close()
		return
	}

	// Doesn't exist, compressed or uncompressed
	http.NotFound(w, r)
}

// localRedirect gives a Moved Permanently response.
// It does not convert relative paths to absolute paths like Redirect does.
func localRedirect(w http.ResponseWriter, r *http.Request, newPath string) {
	if q := r.URL.RawQuery; q != "" {
		newPath += "?" + q
	}
	w.Header().Set("Location", newPath)
	w.WriteHeader(http.StatusMovedPermanently)
}
