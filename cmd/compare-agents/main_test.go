package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProxyCapture(t *testing.T) {
	// Fake upstream.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"echo":"` + string(body) + `"}`))
	}))
	defer upstream.Close()

	target, _ := url.Parse(upstream.URL)
	harPath := filepath.Join(t.TempDir(), "test.har")

	p, err := startProxy(target, harPath)
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()

	// Make a request through the proxy.
	resp, err := http.Post(p.url()+"/v1/messages", "application/json", strings.NewReader(`{"model":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
	if !strings.Contains(string(respBody), `"echo"`) {
		t.Fatalf("unexpected body: %s", respBody)
	}

	// Verify HAR.
	harData, err := os.ReadFile(harPath)
	if err != nil {
		t.Fatal(err)
	}

	var har HAR
	if err := json.Unmarshal(harData, &har); err != nil {
		t.Fatalf("invalid HAR: %v", err)
	}

	if len(har.Log.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(har.Log.Entries))
	}

	entry := har.Log.Entries[0]
	if entry.Request.Method != "POST" {
		t.Errorf("expected POST, got %s", entry.Request.Method)
	}
	if !strings.Contains(entry.Request.URL, "/v1/messages") {
		t.Errorf("expected /v1/messages in URL, got %s", entry.Request.URL)
	}
	if entry.Request.PostData.Text != `{"model":"test"}` {
		t.Errorf("unexpected request body: %s", entry.Request.PostData.Text)
	}
	if !strings.Contains(entry.Response.Content.Text, `"echo"`) {
		t.Errorf("unexpected response body: %s", entry.Response.Content.Text)
	}
}

func TestMultipleRequests(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`ok`))
	}))
	defer upstream.Close()

	target, _ := url.Parse(upstream.URL)
	harPath := filepath.Join(t.TempDir(), "test.har")

	p, err := startProxy(target, harPath)
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()

	for i := range 3 {
		resp, err := http.Post(p.url()+"/v1/messages", "application/json",
			strings.NewReader(`{"call":`+string(rune('0'+i))+`}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}

	harData, _ := os.ReadFile(harPath)
	var har HAR
	json.Unmarshal(harData, &har)

	if len(har.Log.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(har.Log.Entries))
	}
}

func TestFilterEnv(t *testing.T) {
	env := []string{"FOO=1", "BAR=2", "BAZ=3"}
	filtered := filterEnv(env, "BAR")
	if len(filtered) != 2 {
		t.Fatalf("expected 2, got %d", len(filtered))
	}
	for _, e := range filtered {
		if strings.HasPrefix(e, "BAR=") {
			t.Fatal("BAR should have been filtered")
		}
	}
}

func TestReplaceEnv(t *testing.T) {
	env := []string{"HOME=/old", "PATH=/usr/bin"}
	replaced := replaceEnv(env, "HOME", "/new")
	found := false
	for _, e := range replaced {
		if e == "HOME=/new" {
			found = true
		}
		if e == "HOME=/old" {
			t.Fatal("old HOME should be removed")
		}
	}
	if !found {
		t.Fatal("new HOME not found")
	}
}

func TestCreateFakeIIHome(t *testing.T) {
	// This test only verifies the function runs without error and creates
	// the expected structure. It uses the real home dir for symlink sources.
	fakeHome, err := createFakeIIHome("http://localhost:12345", "https://ai.tail98579.ts.net")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fakeHome)

	// Check that .config exists.
	info, err := os.Stat(filepath.Join(fakeHome, ".config"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal(".config should be a directory")
	}

	// Check that incrementum config was overridden (if it exists on this machine).
	iiConfig := filepath.Join(fakeHome, ".config", "incrementum", "config.toml")
	data, err := os.ReadFile(iiConfig)
	if err == nil {
		if strings.Contains(string(data), "ai.tail98579.ts.net") {
			t.Fatal("incrementum config should have proxy URL, not original")
		}
		if !strings.Contains(string(data), "localhost:12345") {
			t.Fatal("incrementum config should contain proxy URL")
		}
	}
}

func TestCreateFakeCodexHome(t *testing.T) {
	// Skip if codex config doesn't exist on this machine.
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home dir")
	}
	if _, err := os.Stat(filepath.Join(realHome, ".codex", "config.toml")); err != nil {
		t.Skip("no codex config found")
	}

	fakeHome, err := createFakeCodexHome("http://localhost:12345", "https://ai.tail98579.ts.net")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fakeHome)

	// Check that .codex/config.toml was overridden.
	codexConfig := filepath.Join(fakeHome, ".codex", "config.toml")
	data, err := os.ReadFile(codexConfig)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "ai.tail98579.ts.net") {
		t.Fatal("codex config should have proxy URL, not original")
	}
	if !strings.Contains(string(data), "localhost:12345") {
		t.Fatal("codex config should contain proxy URL")
	}
}

func TestSymlinkDotfiles(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create some dotfiles and a regular file.
	os.WriteFile(filepath.Join(src, ".foo"), []byte("foo"), 0644)
	os.WriteFile(filepath.Join(src, ".bar"), []byte("bar"), 0644)
	os.WriteFile(filepath.Join(src, ".baz"), []byte("baz"), 0644)
	os.WriteFile(filepath.Join(src, "notdot"), []byte("nope"), 0644)

	err := symlinkDotfiles(src, dst, ".bar")
	if err != nil {
		t.Fatal(err)
	}

	// .foo and .baz should be symlinked, .bar and notdot should not.
	if _, err := os.Lstat(filepath.Join(dst, ".foo")); err != nil {
		t.Error(".foo should exist")
	}
	if _, err := os.Lstat(filepath.Join(dst, ".baz")); err != nil {
		t.Error(".baz should exist")
	}
	if _, err := os.Lstat(filepath.Join(dst, ".bar")); err == nil {
		t.Error(".bar should be excluded")
	}
	if _, err := os.Lstat(filepath.Join(dst, "notdot")); err == nil {
		t.Error("notdot should not be symlinked (not a dotfile)")
	}
}

func TestPrepareWorkDir(t *testing.T) {
	// Create a fake template directory.
	tmpl := t.TempDir()
	os.WriteFile(filepath.Join(tmpl, "hello.go"), []byte("package main\n"), 0644)
	os.MkdirAll(filepath.Join(tmpl, "sub"), 0755)
	os.WriteFile(filepath.Join(tmpl, "sub", "nested.txt"), []byte("nested\n"), 0644)

	workDir, err := prepareWorkDir(tmpl, "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)

	// Check files were copied.
	data, err := os.ReadFile(filepath.Join(workDir, "hello.go"))
	if err != nil {
		t.Fatal("hello.go should exist")
	}
	if string(data) != "package main\n" {
		t.Errorf("hello.go content = %q", data)
	}

	data, err = os.ReadFile(filepath.Join(workDir, "sub", "nested.txt"))
	if err != nil {
		t.Fatal("sub/nested.txt should exist")
	}
	if string(data) != "nested\n" {
		t.Errorf("sub/nested.txt content = %q", data)
	}

	// Check git repo was initialized with a commit and baseline tag.
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	if !strings.Contains(string(out), "init") {
		t.Errorf("expected initial commit, got: %s", out)
	}

	tagCmd := exec.Command("git", "tag", "-l", "baseline")
	tagCmd.Dir = workDir
	tagOut, err := tagCmd.Output()
	if err != nil {
		t.Fatalf("git tag failed: %v", err)
	}
	if !strings.Contains(string(tagOut), "baseline") {
		t.Error("expected baseline tag")
	}
}

func TestCaptureDiff(t *testing.T) {
	// Set up a git repo with an initial commit.
	tmpl := t.TempDir()
	os.WriteFile(filepath.Join(tmpl, "file.txt"), []byte("original\n"), 0644)

	workDir, err := prepareWorkDir(tmpl, "test-diff")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)

	// Modify a file and also commit it (to test that captureDiff
	// catches committed changes, not just working tree changes).
	os.WriteFile(filepath.Join(workDir, "file.txt"), []byte("modified\n"), 0644)
	commitCmd := exec.Command("git", "add", ".")
	commitCmd.Dir = workDir
	commitCmd.Run()
	commitCmd2 := exec.Command("git", "-c", "user.name=test", "-c", "user.email=test@test", "commit", "-m", "modify")
	commitCmd2.Dir = workDir
	commitCmd2.Run()

	// Capture the diff.
	runDir := t.TempDir()
	captureDiff(workDir, runDir, "test-agent")

	diffData, err := os.ReadFile(filepath.Join(runDir, "test-agent.diff"))
	if err != nil {
		t.Fatal("diff file should exist")
	}
	if !strings.Contains(string(diffData), "-original") || !strings.Contains(string(diffData), "+modified") {
		t.Errorf("unexpected diff content: %s", diffData)
	}
}
