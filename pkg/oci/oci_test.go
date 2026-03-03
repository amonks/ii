package oci_test

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"monks.co/pkg/oci"
)

func TestBinaryLayer(t *testing.T) {
	// Create a temp file to act as the binary.
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "myapp")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho hello"), 0755); err != nil {
		t.Fatal(err)
	}

	layer, err := oci.BinaryLayer(binPath, "/usr/local/bin/myapp")
	if err != nil {
		t.Fatal(err)
	}

	// Read back the layer and inspect tar contents.
	rc, err := layer.Uncompressed()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	tr := tar.NewReader(rc)

	// Read all entries; expect directory entries followed by the binary.
	var entries []tar.Header
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, *hdr)
	}

	// Last entry should be the binary file.
	last := entries[len(entries)-1]
	if last.Name != "usr/local/bin/myapp" {
		t.Errorf("expected last tar entry %q, got %q", "usr/local/bin/myapp", last.Name)
	}
	if last.Mode != 0755 {
		t.Errorf("expected mode 0755, got %o", last.Mode)
	}

	// Parent directories should precede the binary.
	if len(entries) < 4 {
		t.Errorf("expected at least 4 entries (dirs + file), got %d", len(entries))
	}
}

func TestFilesLayer(t *testing.T) {
	tmp := t.TempDir()

	// Create two source files.
	file1 := filepath.Join(tmp, "config.toml")
	if err := os.WriteFile(file1, []byte("[server]\nport = 8080"), 0644); err != nil {
		t.Fatal(err)
	}
	file2 := filepath.Join(tmp, "data.json")
	if err := os.WriteFile(file2, []byte(`{"key":"value"}`), 0644); err != nil {
		t.Fatal(err)
	}

	mappings := map[string]string{
		file1: "/etc/myapp/config.toml",
		file2: "/var/data/data.json",
	}

	layer, err := oci.FilesLayer(mappings)
	if err != nil {
		t.Fatal(err)
	}

	rc, err := layer.Uncompressed()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	found := make(map[string]string)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		found[hdr.Name] = string(data)
	}

	if len(found) != 2 {
		t.Fatalf("expected 2 tar entries, got %d", len(found))
	}

	if content, ok := found["etc/myapp/config.toml"]; !ok {
		t.Error("missing etc/myapp/config.toml")
	} else if content != "[server]\nport = 8080" {
		t.Errorf("unexpected config content: %q", content)
	}

	if content, ok := found["var/data/data.json"]; !ok {
		t.Error("missing var/data/data.json")
	} else if content != `{"key":"value"}` {
		t.Errorf("unexpected data content: %q", content)
	}
}

func TestBuildAppImage(t *testing.T) {
	tmp := t.TempDir()

	// Create a fake binary.
	binPath := filepath.Join(tmp, "server")
	if err := os.WriteFile(binPath, []byte("binary-content"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a config file.
	cfgPath := filepath.Join(tmp, "app.toml")
	if err := os.WriteFile(cfgPath, []byte("config-data"), 0644); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		cfgPath: "/etc/app.toml",
	}

	cfg := oci.ImageConfig{
		Cmd:     []string{"/app/server"},
		Env:     []string{"PORT=8080", "ENV=production"},
		WorkDir: "/app",
	}

	img, err := oci.BuildAppImage(empty.Image, binPath, files, cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Verify layers: base (0) + binary + files + workdir = 3 layers.
	layers, err := img.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 3 {
		t.Errorf("expected 3 layers, got %d", len(layers))
	}

	// Verify the image config was set.
	imgCfg, err := img.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}

	if len(imgCfg.Config.Cmd) != 1 || imgCfg.Config.Cmd[0] != "/app/server" {
		t.Errorf("unexpected Cmd: %v", imgCfg.Config.Cmd)
	}
	if len(imgCfg.Config.Env) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(imgCfg.Config.Env))
	}
	if imgCfg.Config.WorkingDir != "/app" {
		t.Errorf("expected WorkingDir /app, got %q", imgCfg.Config.WorkingDir)
	}
}

func TestBuildAppImageSetsPlatform(t *testing.T) {
	cfg := oci.ImageConfig{
		Cmd: []string{"/bin/app"},
	}

	img, err := oci.BuildAppImage(empty.Image, "", nil, cfg)
	if err != nil {
		t.Fatal(err)
	}

	imgCfg, err := img.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}

	if imgCfg.Architecture != "amd64" {
		t.Errorf("expected Architecture %q, got %q", "amd64", imgCfg.Architecture)
	}
	if imgCfg.OS != "linux" {
		t.Errorf("expected OS %q, got %q", "linux", imgCfg.OS)
	}
}

func TestLayerMediaTypesAreOCI(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "app")
	if err := os.WriteFile(binPath, []byte("binary"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := oci.ImageConfig{Cmd: []string{"/app/app"}}
	img, err := oci.BuildAppImage(empty.Image, binPath, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}

	layers, err := img.Layers()
	if err != nil {
		t.Fatal(err)
	}

	for i, l := range layers {
		mt, err := l.MediaType()
		if err != nil {
			t.Fatal(err)
		}
		if mt != "application/vnd.oci.image.layer.v1.tar+gzip" {
			t.Errorf("layer %d media type = %q, want OCI gzip", i, mt)
		}
	}
}

func TestImageConfigRoundtrip(t *testing.T) {
	cfg := oci.ImageConfig{
		Cmd:     []string{"/bin/app", "--flag"},
		Env:     []string{"A=1", "B=2"},
		WorkDir: "/opt",
	}

	img, err := oci.BuildAppImage(empty.Image, "", nil, cfg)
	if err != nil {
		t.Fatal(err)
	}

	imgCfg, err := img.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}

	// Verify Cmd roundtrips.
	if len(imgCfg.Config.Cmd) != 2 || imgCfg.Config.Cmd[0] != "/bin/app" || imgCfg.Config.Cmd[1] != "--flag" {
		t.Errorf("Cmd roundtrip failed: %v", imgCfg.Config.Cmd)
	}

	// Verify Env roundtrips.
	envMap := make(map[string]bool)
	for _, e := range imgCfg.Config.Env {
		envMap[e] = true
	}
	if !envMap["A=1"] || !envMap["B=2"] {
		t.Errorf("Env roundtrip failed: %v", imgCfg.Config.Env)
	}

	// Verify WorkDir roundtrips.
	if imgCfg.Config.WorkingDir != "/opt" {
		t.Errorf("WorkDir roundtrip failed: got %q", imgCfg.Config.WorkingDir)
	}
}
