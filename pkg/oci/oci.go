// Package oci provides pure Go functions for building OCI container images
// using google/go-containerregistry. No Docker daemon is needed.
package oci

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// ImageConfig holds container configuration for the built image.
type ImageConfig struct {
	Cmd     []string
	Env     []string
	WorkDir string
}

// BinaryLayer creates a tar layer containing one executable file at destPath
// with permissions 0755. The destPath should be absolute (e.g. "/usr/local/bin/myapp");
// the leading slash is stripped for the tar entry name.
func BinaryLayer(binaryPath, destPath string) (v1.Layer, error) {
	if binaryPath == "" {
		return layerFromTar(emptyTar())
	}

	data, err := os.ReadFile(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("reading binary %s: %w", binaryPath, err)
	}

	entryName := strings.TrimPrefix(destPath, "/")

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name: entryName,
		Mode: 0755,
		Size: int64(len(data)),
	}); err != nil {
		return nil, fmt.Errorf("writing tar header: %w", err)
	}
	if _, err := tw.Write(data); err != nil {
		return nil, fmt.Errorf("writing tar data: %w", err)
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("closing tar writer: %w", err)
	}

	return layerFromTar(buf.Bytes())
}

// FilesLayer creates a tar layer with multiple files. Keys in mappings are
// source paths on disk; values are destination paths in the container
// (leading slashes are stripped for tar entry names).
func FilesLayer(mappings map[string]string) (v1.Layer, error) {
	if len(mappings) == 0 {
		return layerFromTar(emptyTar())
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for src, dest := range mappings {
		data, err := os.ReadFile(src)
		if err != nil {
			return nil, fmt.Errorf("reading file %s: %w", src, err)
		}

		entryName := strings.TrimPrefix(dest, "/")
		if err := tw.WriteHeader(&tar.Header{
			Name: entryName,
			Mode: 0644,
			Size: int64(len(data)),
		}); err != nil {
			return nil, fmt.Errorf("writing tar header for %s: %w", dest, err)
		}
		if _, err := tw.Write(data); err != nil {
			return nil, fmt.Errorf("writing tar data for %s: %w", dest, err)
		}
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("closing tar writer: %w", err)
	}

	return layerFromTar(buf.Bytes())
}

// BuildAppImage composes a container image from a base image, a binary layer,
// an optional files layer, and container configuration.
//
// If binary is empty, no binary layer is added. If files is nil or empty,
// no files layer is added. The config is always applied.
//
// The binary is placed at /app/<basename> inside the container.
func BuildAppImage(base v1.Image, binary string, files map[string]string, cfg ImageConfig) (v1.Image, error) {
	img := base

	// Add binary layer if a binary path was provided.
	if binary != "" {
		destPath := "/app/" + filepath.Base(binary)
		binLayer, err := BinaryLayer(binary, destPath)
		if err != nil {
			return nil, fmt.Errorf("creating binary layer: %w", err)
		}
		img, err = mutate.AppendLayers(img, binLayer)
		if err != nil {
			return nil, fmt.Errorf("appending binary layer: %w", err)
		}
	}

	// Add files layer if any file mappings were provided.
	if len(files) > 0 {
		filesLayer, err := FilesLayer(files)
		if err != nil {
			return nil, fmt.Errorf("creating files layer: %w", err)
		}
		img, err = mutate.AppendLayers(img, filesLayer)
		if err != nil {
			return nil, fmt.Errorf("appending files layer: %w", err)
		}
	}

	// Apply container config.
	imgCfg, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("reading image config: %w", err)
	}
	imgCfg.Config.Cmd = cfg.Cmd
	imgCfg.Config.Env = cfg.Env
	imgCfg.Config.WorkingDir = cfg.WorkDir

	img, err = mutate.Config(img, imgCfg.Config)
	if err != nil {
		return nil, fmt.Errorf("setting image config: %w", err)
	}

	return img, nil
}

// Push pushes an image to a remote registry at the given ref string.
func Push(img v1.Image, ref string, opts ...remote.Option) error {
	r, err := name.ParseReference(ref)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %w", ref, err)
	}
	return remote.Write(r, img, opts...)
}

// layerFromTar creates a layer from tar bytes using LayerFromOpener.
func layerFromTar(data []byte) (v1.Layer, error) {
	return tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	})
}

// emptyTar returns the bytes of an empty but valid tar archive.
func emptyTar() []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	tw.Close()
	return buf.Bytes()
}
