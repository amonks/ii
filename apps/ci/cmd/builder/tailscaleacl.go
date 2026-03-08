package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	tsclient "github.com/tailscale/tailscale-client-go/v2"
	"monks.co/pkg/tailscaleacl"
)

// TailscaleACLApply generates the Tailscale ACL policy from config and
// pushes it via the Tailscale API. It runs as a "tailscale-acl" stream
// within the "deploy" job.
func TailscaleACLApply(reporter *Reporter) error {
	reporter.StartStream("deploy", "tailscale-acl")

	w := reporter.StreamWriter("deploy", "tailscale-acl")
	defer w.Close()

	start := time.Now()

	clientID := os.Getenv("TAILSCALE_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("TAILSCALE_OAUTH_CLIENT_SECRET")
	tailnetID := os.Getenv("TAILSCALE_TAILNET_ID")

	if clientID == "" || clientSecret == "" || tailnetID == "" {
		errMsg := "tailscale OAuth credentials not configured (need TAILSCALE_OAUTH_CLIENT_ID, TAILSCALE_OAUTH_CLIENT_SECRET, TAILSCALE_TAILNET_ID)"
		slog.Error(errMsg)
		fmt.Fprintf(w, "%s\n", errMsg)
		reporter.FinishStream("deploy", "tailscale-acl", FinishStreamResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return fmt.Errorf("%s", errMsg)
	}

	// Generate ACL JSON.
	fmt.Fprintf(w, "=== generating ACL policy\n")
	aclBytes, err := tailscaleacl.Generate()
	if err != nil {
		errMsg := fmt.Sprintf("generating ACL: %v", err)
		fmt.Fprintf(w, "=== %s\n", errMsg)
		reporter.FinishStream("deploy", "tailscale-acl", FinishStreamResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return fmt.Errorf("generating ACL: %w", err)
	}

	// Push to Tailscale. Pass the JSON bytes as a string so the SDK
	// treats it as HuJSON content (the Set method rejects map[string]interface{}).
	fmt.Fprintf(w, "=== pushing ACL to tailnet %s\n", tailnetID)
	client := &tsclient.Client{
		Tailnet: tailnetID,
		HTTP: tsclient.OAuthConfig{
			ClientID:     clientID,
			ClientSecret: clientSecret,
		}.HTTPClient(),
	}

	ctx := context.Background()
	if err := client.PolicyFile().Set(ctx, string(aclBytes), ""); err != nil {
		errMsg := fmt.Sprintf("pushing ACL: %v", err)
		fmt.Fprintf(w, "=== %s\n", errMsg)
		reporter.FinishStream("deploy", "tailscale-acl", FinishStreamResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return fmt.Errorf("pushing ACL: %w", err)
	}

	duration := time.Since(start).Milliseconds()
	fmt.Fprintf(w, "=== done (%dms)\n", duration)
	reporter.FinishStream("deploy", "tailscale-acl", FinishStreamResult{
		Status:     "success",
		DurationMs: duration,
	})
	return nil
}
