package serve

import (
	"context"
	"net/http"
)

type basePathKey struct{}

// BasePath returns the app's mount prefix from the proxy, or "/" for direct
// tailnet access. Suitable for use in a <base href> tag.
func BasePath(r *http.Request) string {
	if p := r.Header.Get("X-Forwarded-Prefix"); p != "" {
		return p + "/"
	}
	return "/"
}

// BasePathFromContext returns the BasePath stored in context, or "/" if unset.
func BasePathFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(basePathKey{}).(string); ok {
		return v
	}
	return "/"
}

// WithBasePath stores the BasePath in context for use by templates.
func WithBasePath(ctx context.Context, basePath string) context.Context {
	return context.WithValue(ctx, basePathKey{}, basePath)
}
