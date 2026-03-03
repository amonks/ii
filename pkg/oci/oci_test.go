package oci_test

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
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
	hdr, err := tr.Next()
	if err != nil {
		t.Fatal(err)
	}

	if hdr.Name != "usr/local/bin/myapp" {
		t.Errorf("expected tar entry name %q, got %q", "usr/local/bin/myapp", hdr.Name)
	}
	if hdr.Mode != 0755 {
		t.Errorf("expected mode 0755, got %o", hdr.Mode)
	}

	content, err := io.ReadAll(tr)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "#!/bin/sh\necho hello" {
		t.Errorf("unexpected content: %q", string(content))
	}

	// There should be no more entries.
	_, err = tr.Next()
	if err != io.EOF {
		t.Errorf("expected EOF after single entry, got %v", err)
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

	// Verify layers: base (empty has 0 layers) + binary + files = 2 layers.
	layers, err := img.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 2 {
		t.Errorf("expected 2 layers, got %d", len(layers))
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

// layerEntries is a helper that reads all tar entries from a layer.
func layerEntries(t *testing.T, l v1.Layer) map[string]*tar.Header {
	t.Helper()
	rc, err := l.Uncompressed()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	result := make(map[string]*tar.Header)
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		result[hdr.Name] = hdr
	}
	return result
}
