// Package reqlog provides wide-event structured logging for HTTP requests.
//
// Each request produces a single slog event containing all attributes
// accumulated during request processing. Use [Set] within handlers to
// add app-specific attributes using namespaced keys (e.g. "proxy.backend").
//
// Use [Logger] to get a slog.Logger pre-populated with the request ID
// and app name for mid-request debug logging.
package reqlog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"monks.co/pkg/errlogger"
	"monks.co/pkg/meta"
	"monks.co/pkg/middleware"
)

const RequestIDHeader = "X-Request-ID"

// RemoteAddrKey is a context key for the real client remote address.
// Set this via http.Server.ConnContext when using ProxyProto or similar.
var RemoteAddrKey = &struct{}{}

// SetupLogging configures the default slog logger to output JSON to stderr.
// Additional writers can be passed to tee log output (e.g. a logsclient).
func SetupLogging(writers ...io.Writer) {
	var w io.Writer = os.Stderr
	if len(writers) > 0 {
		all := make([]io.Writer, 0, len(writers)+1)
		all = append(all, os.Stderr)
		all = append(all, writers...)
		w = io.MultiWriter(all...)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
}

type ctxKey struct{}

type requestLog struct {
	mu    sync.Mutex
	attrs []slog.Attr

	rw      *responseWriter
	reqID   string
	appName string
}

func (rl *requestLog) set(key string, value any) {
	rl.mu.Lock()
	rl.attrs = append(rl.attrs, slog.Any(key, value))
	rl.mu.Unlock()
}

func fromContext(ctx context.Context) *requestLog {
	rl, _ := ctx.Value(ctxKey{}).(*requestLog)
	return rl
}

// Set adds a key-value pair to the request's wide event log.
// Use namespaced keys like "proxy.backend" or "traffic.entries".
func Set(ctx context.Context, key string, value any) {
	if rl := fromContext(ctx); rl != nil {
		rl.set(key, value)
	}
}

// Status returns the HTTP response status code for this request.
// Returns 0 if called before the status has been written or outside
// reqlog middleware.
func Status(ctx context.Context) int {
	if rl := fromContext(ctx); rl != nil && rl.rw != nil {
		return rl.rw.statusCode
	}
	return 0
}

// RequestID returns the request ID from context, or "" if not available.
func RequestID(ctx context.Context) string {
	if rl := fromContext(ctx); rl != nil {
		return rl.reqID
	}
	return ""
}

// Logger returns a slog.Logger pre-populated with the request ID and app
// name. Use this for mid-request debug logging that should correlate with
// the wide event.
func Logger(ctx context.Context) *slog.Logger {
	if rl := fromContext(ctx); rl != nil {
		return slog.With("req.id", rl.reqID, "app.name", rl.appName)
	}
	return slog.Default()
}

// Middleware returns HTTP middleware that emits a wide structured log
// event for each completed request. It captures panics, records timing,
// and includes all attributes added via [Set].
//
// This should be the outermost middleware in the chain.
func Middleware() middleware.Middleware {
	appName := meta.AppName()
	return middleware.MiddlewareFunc(func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			rl := &requestLog{appName: appName}
			ctx := context.WithValue(req.Context(), ctxKey{}, rl)
			req = req.WithContext(ctx)

			// Request ID: extract from upstream or generate.
			rl.reqID = req.Header.Get(RequestIDHeader)
			if rl.reqID == "" {
				rl.reqID = generateID()
			}
			w.Header().Set(RequestIDHeader, rl.reqID)

			// Wrap ResponseWriter to capture status and bytes.
			rw := &responseWriter{ResponseWriter: w, statusCode: 200}
			rl.rw = rw

			// Resolve real remote address.
			remoteAddr := req.RemoteAddr
			if v, ok := req.Context().Value(RemoteAddrKey).(string); ok {
				remoteAddr = v
			}

			start := time.Now()
			defer func() {
				dur := time.Since(start)

				if r := recover(); r != nil {
					stack := string(debug.Stack())
					rl.set("err.panic", fmt.Sprint(r))
					rl.set("err.stack", stack)
					errlogger.Report(500, fmt.Sprintf("panic: %v\n%s", r, stack))
					if !rw.wroteHeader {
						http.Error(rw, http.StatusText(500), 500)
					}
				}

				// Build the wide event: identity, request, app-specific, response.
				args := make([]any, 0, 16+len(rl.attrs))

				args = append(args,
					slog.String("app.name", appName),
					slog.String("req.id", rl.reqID),
					slog.String("http.method", req.Method),
					slog.String("http.host", req.Host),
					slog.String("http.path", req.URL.Path),
					slog.String("http.remote_addr", remoteAddr),
				)
				if req.URL.RawQuery != "" {
					args = append(args, slog.String("http.query", req.URL.RawQuery))
				}
				if ua := req.UserAgent(); ua != "" {
					args = append(args, slog.String("http.user_agent", ua))
				}
				if ref := req.Header.Get("Referer"); ref != "" {
					args = append(args, slog.String("http.referer", ref))
				}

				// App-specific attrs added via Set().
				rl.mu.Lock()
				for _, a := range rl.attrs {
					args = append(args, a)
				}
				rl.mu.Unlock()

				// Response attrs.
				args = append(args,
					slog.Int("http.status", rw.statusCode),
					slog.Int("http.bytes_written", rw.bytesWritten),
					slog.Int64("http.duration_ms", dur.Milliseconds()),
				)
				if req.Pattern != "" {
					args = append(args, slog.String("http.route", req.Pattern))
				}

				level := slog.LevelInfo
				if rw.statusCode >= 500 {
					level = slog.LevelError
				} else if rw.statusCode >= 400 {
					level = slog.LevelWarn
				}
				slog.Log(req.Context(), level, "request", args...)
			}()

			handler.ServeHTTP(rw, req)
		})
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	wroteHeader  bool
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// Unwrap returns the underlying ResponseWriter, enabling
// http.ResponseController to discover interface implementations
// like http.Flusher on the original writer.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
