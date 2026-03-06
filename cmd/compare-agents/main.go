// Command compare-agents runs four agents with the same prompt, capturing
// HTTP traffic (as HAR files) and stdout/stderr for each. The four agents
// are:
//
//   - codex:      OpenAI Codex CLI (Responses API)
//   - claude:     Claude Code CLI (Anthropic Messages API)
//   - ii-openai:  ii agent with an OpenAI model
//   - ii-claude:  ii agent with an Anthropic model
//
// This is for debugging differences in API call patterns (caching, token
// usage, etc.) between tools and API backends.
//
// Usage:
//
//	go run . "List all go.mod files and describe each module"
//	go run . -output ./results "some prompt"
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func main() {
	var outputDir string
	var upstream string
	var timeout time.Duration
	var openaiModel string
	var claudeModel string
	flag.StringVar(&outputDir, "output", "./compare-agents-output", "base directory for output")
	flag.StringVar(&upstream, "upstream", "https://ai.tail98579.ts.net", "upstream API proxy URL")
	flag.DurationVar(&timeout, "timeout", 5*time.Minute, "per-agent timeout")
	flag.StringVar(&openaiModel, "openai-model", "gpt-5.2-codex", "OpenAI model for ii-openai")
	flag.StringVar(&claudeModel, "claude-model", "claude-sonnet-4-5", "Anthropic model for ii-claude")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: compare-agents [flags] <prompt>\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if len(flag.Args()) < 1 {
		flag.Usage()
		os.Exit(1)
	}
	prompt := flag.Args()[0]

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Create timestamped output directory.
	runDir := filepath.Join(outputDir, time.Now().Format("2006-01-02T15-04-05"))
	if err := os.MkdirAll(runDir, 0755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}
	log.Printf("output → %s", runDir)

	target, err := url.Parse(upstream)
	if err != nil {
		log.Fatalf("bad upstream URL: %v", err)
	}

	// Start four proxy instances.
	type namedProxy struct {
		name  string
		proxy *proxy
	}
	names := []string{"codex", "claude-code", "ii-openai", "ii-claude"}
	proxies := make([]namedProxy, len(names))
	for i, name := range names {
		p, err := startProxy(target, filepath.Join(runDir, name+".har"))
		if err != nil {
			log.Fatalf("start %s proxy: %v", name, err)
		}
		defer p.close()
		proxies[i] = namedProxy{name: name, proxy: p}
		log.Printf("%-12s proxy → %s", name, p.url())
	}

	// Run all four agents in parallel.
	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		runCodex(ctx, timeout, prompt, proxies[0].proxy.url(), upstream, runDir)
	}()
	go func() {
		defer wg.Done()
		runClaudeCode(ctx, timeout, prompt, proxies[1].proxy.url(), runDir)
	}()
	go func() {
		defer wg.Done()
		runIIOpenAI(ctx, timeout, prompt, openaiModel, proxies[2].proxy.url(), upstream, runDir)
	}()
	go func() {
		defer wg.Done()
		runIIClaude(ctx, timeout, prompt, claudeModel, proxies[3].proxy.url(), upstream, runDir)
	}()

	wg.Wait()
}

// --- Agent runners ---

func runCodex(ctx context.Context, timeout time.Duration, prompt, proxyURL, upstream, runDir string) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	fakeHome, err := createFakeCodexHome(proxyURL, upstream)
	if err != nil {
		log.Printf("[codex] fake home setup failed: %v", err)
		return
	}
	defer os.RemoveAll(fakeHome)

	cmd := exec.CommandContext(ctx, "codex", "exec", prompt)
	env := replaceEnv(os.Environ(), "HOME", fakeHome)
	cmd.Env = env

	runAgent(cmd, runDir, "codex")
}

func runClaudeCode(ctx context.Context, timeout time.Duration, prompt, proxyURL, runDir string) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude",
		"-p", prompt,
		"--max-turns", "5",
		"--output-format", "text",
	)

	// Build env: inherit everything, override ANTHROPIC_BASE_URL, remove CLAUDECODE.
	env := filterEnv(os.Environ(), "CLAUDECODE")
	env = append(env, "ANTHROPIC_BASE_URL="+proxyURL)
	cmd.Env = env

	runAgent(cmd, runDir, "claude-code")
}

func runIIOpenAI(ctx context.Context, timeout time.Duration, prompt, model, proxyURL, upstream, runDir string) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	fakeHome, err := createFakeIIHome(proxyURL, upstream)
	if err != nil {
		log.Printf("[ii-openai] fake home setup failed: %v", err)
		return
	}
	defer os.RemoveAll(fakeHome)

	cmd := exec.CommandContext(ctx, "ii", "agent", "run", "--model", model, prompt)
	env := replaceEnv(os.Environ(), "HOME", fakeHome)
	cmd.Env = env

	runAgent(cmd, runDir, "ii-openai")
}

func runIIClaude(ctx context.Context, timeout time.Duration, prompt, model, proxyURL, upstream, runDir string) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	fakeHome, err := createFakeIIHome(proxyURL, upstream)
	if err != nil {
		log.Printf("[ii-claude] fake home setup failed: %v", err)
		return
	}
	defer os.RemoveAll(fakeHome)

	cmd := exec.CommandContext(ctx, "ii", "agent", "run", "--model", model, prompt)
	env := replaceEnv(os.Environ(), "HOME", fakeHome)
	cmd.Env = env

	runAgent(cmd, runDir, "ii-claude")
}

func runAgent(cmd *exec.Cmd, runDir, name string) {
	stdoutPath := filepath.Join(runDir, name+".stdout")
	stderrPath := filepath.Join(runDir, name+".stderr")

	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		log.Printf("[%s] create stdout file: %v", name, err)
		return
	}
	defer stdoutFile.Close()

	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		log.Printf("[%s] create stderr file: %v", name, err)
		return
	}
	defer stderrFile.Close()

	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	log.Printf("[%s] starting: %s", name, strings.Join(cmd.Args, " "))
	start := time.Now()

	err = cmd.Run()
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("[%s] finished with error after %s: %v", name, elapsed.Round(time.Second), err)
	} else {
		log.Printf("[%s] finished in %s", name, elapsed.Round(time.Second))
	}
}

// --- Fake HOME helpers ---

// createFakeCodexHome creates a temporary HOME with a modified
// ~/.codex/config.toml that routes API traffic through proxyURL.
func createFakeCodexHome(proxyURL, upstream string) (string, error) {
	realHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	fakeHome, err := os.MkdirTemp("", "compare-agents-codex-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// Symlink all dotfiles from real home except .codex.
	if err := symlinkDotfiles(realHome, fakeHome, ".codex"); err != nil {
		os.RemoveAll(fakeHome)
		return "", err
	}

	// Build .codex: symlink everything inside except config.toml.
	realCodex := filepath.Join(realHome, ".codex")
	fakeCodex := filepath.Join(fakeHome, ".codex")
	os.MkdirAll(fakeCodex, 0755)

	entries, _ := os.ReadDir(realCodex)
	for _, e := range entries {
		name := e.Name()
		if name == "config.toml" {
			continue
		}
		os.Symlink(filepath.Join(realCodex, name), filepath.Join(fakeCodex, name))
	}

	// Rewrite config.toml: replace the upstream URL with the proxy URL.
	configData, err := os.ReadFile(filepath.Join(realCodex, "config.toml"))
	if err != nil {
		os.RemoveAll(fakeHome)
		return "", fmt.Errorf("read codex config: %w", err)
	}
	replaced := strings.ReplaceAll(string(configData), upstream, proxyURL)
	if err := os.WriteFile(filepath.Join(fakeCodex, "config.toml"), []byte(replaced), 0644); err != nil {
		os.RemoveAll(fakeHome)
		return "", fmt.Errorf("write codex config: %w", err)
	}

	return fakeHome, nil
}

// createFakeIIHome creates a temporary HOME with a modified
// ~/.config/incrementum/config.toml that routes API traffic through proxyURL.
func createFakeIIHome(proxyURL, upstream string) (string, error) {
	realHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	fakeHome, err := os.MkdirTemp("", "compare-agents-ii-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// Symlink all dotfiles from real home except .config and .local.
	// (.local contains SQLite databases that break when accessed through
	// symlinks due to WAL file path resolution).
	if err := symlinkDotfiles(realHome, fakeHome, ".config", ".local"); err != nil {
		os.RemoveAll(fakeHome)
		return "", err
	}

	// Create .local/state/incrementum with a fresh database.
	os.MkdirAll(filepath.Join(fakeHome, ".local", "state", "incrementum"), 0755)
	os.MkdirAll(filepath.Join(fakeHome, ".local", "share", "incrementum", "workspaces"), 0755)

	// Build .config with overrides for incrementum.
	realConfig := filepath.Join(realHome, ".config")
	fakeConfig := filepath.Join(fakeHome, ".config")
	os.MkdirAll(fakeConfig, 0755)

	configEntries, _ := os.ReadDir(realConfig)
	for _, e := range configEntries {
		name := e.Name()
		if name == "incrementum" {
			continue
		}
		os.Symlink(filepath.Join(realConfig, name), filepath.Join(fakeConfig, name))
	}

	// Override incrementum config with proxy URL.
	iiConfigSrc := filepath.Join(realConfig, "incrementum", "config.toml")
	iiConfigData, err := os.ReadFile(iiConfigSrc)
	if err == nil {
		replaced := strings.ReplaceAll(string(iiConfigData), upstream, proxyURL)
		iiConfigDir := filepath.Join(fakeConfig, "incrementum")
		os.MkdirAll(iiConfigDir, 0755)
		os.WriteFile(filepath.Join(iiConfigDir, "config.toml"), []byte(replaced), 0644)
	}

	return fakeHome, nil
}

// symlinkDotfiles symlinks all dotfiles from src to dst, skipping any names in exclude.
func symlinkDotfiles(src, dst string, exclude ...string) error {
	excludeSet := make(map[string]bool, len(exclude))
	for _, name := range exclude {
		excludeSet[name] = true
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", src, err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, ".") {
			continue
		}
		if excludeSet[name] {
			continue
		}
		os.Symlink(filepath.Join(src, name), filepath.Join(dst, name))
	}
	return nil
}

// --- Env helpers ---

func filterEnv(env []string, removeKeys ...string) []string {
	var out []string
	for _, e := range env {
		key, _, _ := strings.Cut(e, "=")
		skip := false
		for _, rk := range removeKeys {
			if strings.EqualFold(key, rk) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, e)
		}
	}
	return out
}

func replaceEnv(env []string, key, value string) []string {
	out := filterEnv(env, key)
	return append(out, key+"="+value)
}

// --- Proxy ---

type proxy struct {
	server   *http.Server
	listener net.Listener
	har      *harLog
}

func startProxy(target *url.URL, harPath string) (*proxy, error) {
	h := &harLog{
		file: harPath,
		log: HAR{
			Log: HARLog{
				Version: "1.2",
				Creator: HARCreator{Name: "compare-agents", Version: "0.1"},
			},
		},
	}

	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
	}
	rp.Transport = &loggingTransport{inner: http.DefaultTransport, har: h}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	srv := &http.Server{Handler: rp}
	go srv.Serve(ln)

	return &proxy{server: srv, listener: ln, har: h}, nil
}

func (p *proxy) url() string {
	return "http://" + p.listener.Addr().String()
}

func (p *proxy) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	p.server.Shutdown(ctx)
}

// --- Logging transport ---

type loggingTransport struct {
	inner http.RoundTripper
	har   *harLog
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
	}

	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(respBody))

	// Decompress for the HAR log so entries are human-readable.
	harRespBody := respBody
	if resp.Header.Get("Content-Encoding") == "gzip" {
		if gr, err := gzip.NewReader(bytes.NewReader(respBody)); err == nil {
			if decompressed, err := io.ReadAll(gr); err == nil {
				harRespBody = decompressed
			}
			gr.Close()
		}
	}

	elapsed := time.Since(start)

	entry := HAREntry{
		StartedDateTime: start.UTC().Format(time.RFC3339Nano),
		Time:            float64(elapsed.Milliseconds()),
		Request: HARRequest{
			Method:      req.Method,
			URL:         req.URL.String(),
			HTTPVersion: "HTTP/1.1",
			Headers:     harHeaders(req.Header),
			PostData: &HARPostData{
				MimeType: req.Header.Get("Content-Type"),
				Text:     string(reqBody),
			},
			BodySize: len(reqBody),
		},
		Response: HARResponse{
			Status:      resp.StatusCode,
			StatusText:  resp.Status,
			HTTPVersion: "HTTP/1.1",
			Headers:     harHeaders(resp.Header),
			Content: HARContent{
				Size:     len(harRespBody),
				MimeType: resp.Header.Get("Content-Type"),
				Text:     string(harRespBody),
			},
			BodySize: len(respBody),
		},
		Timings: HARTimings{
			Send:    0,
			Wait:    float64(elapsed.Milliseconds()),
			Receive: 0,
		},
	}

	t.har.add(entry)
	return resp, nil
}

func harHeaders(h http.Header) []HARHeader {
	var out []HARHeader
	for k, vs := range h {
		for _, v := range vs {
			out = append(out, HARHeader{Name: k, Value: v})
		}
	}
	return out
}

// --- HAR accumulator ---

type harLog struct {
	mu   sync.Mutex
	log  HAR
	file string
}

func (h *harLog) add(e HAREntry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.log.Log.Entries = append(h.log.Log.Entries, e)
	h.flush()
}

func (h *harLog) flush() {
	data, err := json.MarshalIndent(h.log, "", "  ")
	if err != nil {
		log.Printf("HAR marshal error: %v", err)
		return
	}
	if err := os.WriteFile(h.file, data, 0644); err != nil {
		log.Printf("HAR write error: %v", err)
	}
}

// --- HAR types ---

type HAR struct {
	Log HARLog `json:"log"`
}

type HARLog struct {
	Version string     `json:"version"`
	Creator HARCreator `json:"creator"`
	Entries []HAREntry `json:"entries"`
}

type HARCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type HAREntry struct {
	StartedDateTime string      `json:"startedDateTime"`
	Time            float64     `json:"time"`
	Request         HARRequest  `json:"request"`
	Response        HARResponse `json:"response"`
	Timings         HARTimings  `json:"timings"`
}

type HARRequest struct {
	Method      string       `json:"method"`
	URL         string       `json:"url"`
	HTTPVersion string       `json:"httpVersion"`
	Headers     []HARHeader  `json:"headers"`
	PostData    *HARPostData `json:"postData,omitempty"`
	BodySize    int          `json:"bodySize"`
}

type HARResponse struct {
	Status      int         `json:"status"`
	StatusText  string      `json:"statusText"`
	HTTPVersion string      `json:"httpVersion"`
	Headers     []HARHeader `json:"headers"`
	Content     HARContent  `json:"content"`
	BodySize    int         `json:"bodySize"`
}

type HARHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type HARPostData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

type HARContent struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

type HARTimings struct {
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
}
