package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"monks.co/pkg/gzip"
	"monks.co/pkg/meta"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
)

const (
	archivePath = "/data/tank/mirror/reddit/"
	dbPath      = "/data/tank/mirror/reddit/.reddit.db"
	clientID    = "-RT9cp4AERMlAEhwR01isQ"
	secret      = "mgo2f7coeJj31sIZDsdIlLZfjBfSiA"
	username    = "richbizzness"
	userAgent   = "golang:monks.co.reddit:v1.0 (by /u/richbizzness)"
)

func main() {
	if err := run(); err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.Error("fatal", "error", err.Error(), "app.name", meta.AppName())
		}
		os.Exit(1)
	}
}

func run() error {
	reqlog.SetupLogging()
	defer reqlog.Shutdown()

	// If first argument is "update", run the archive update process
	if len(os.Args) > 1 && os.Args[1] == "update" {
		return runUpdate()
	}

	// Default behavior - serve the web interface
	return runServer()
}

func runServer() error {
	ctx := sigctx.New()
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}

	db, err := NewModel()
	if err != nil {
		return fmt.Errorf("constructing model: %w", err)
	}
	var errs error

	s := newServer(db)
	if err := tailnet.ListenAndServe(ctx, reqlog.Middleware().ModifyHandler(gzip.Middleware(s))); err != nil {
		if errs != nil {
			errs = fmt.Errorf("%v; %v", errs, err)
		} else {
			errs = err
		}
	}

	if err := db.Close(); err != nil {
		if errs != nil {
			errs = fmt.Errorf("%v; %v", errs, err)
		} else {
			errs = err
		}
	}

	return errs
}

func runUpdate() error {
	ctx := sigctx.New()

	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}

	db, err := NewModel()
	if err != nil {
		return fmt.Errorf("constructing model: %w", err)
	}
	defer db.Close()

	archiver := NewRedditArchiver(db, clientID, secret, archivePath, username)
	err = archiver.Run()

	// Handle token-related errors
	if err != nil {
		if strings.Contains(err.Error(), "token file does not exist") ||
			strings.Contains(err.Error(), "refresh token is missing") {
			// Token is missing, guide the user through authentication
			authHelper := archiver.GetAuthHelper()
			authHelper.PrintAuthHelp()

			// Read the code from stdin
			var code string
			fmt.Print("> ")
			fmt.Scanln(&code)

			// Clean up the code (remove any URL parts if the user pasted the whole URL)
			code = strings.TrimSpace(code)
			if strings.Contains(code, "code=") {
				parts := strings.Split(code, "code=")
				if len(parts) > 1 {
					code = parts[1]
					if idx := strings.Index(code, "&"); idx > 0 {
						code = code[:idx]
					}
				}
			}

			fmt.Println("Using authorization code:", code)

			// Exchange the code for tokens
			if err := archiver.ExchangeCode(code); err != nil {
				return fmt.Errorf("failed to exchange authorization code: %w", err)
			}

			fmt.Println("\nAuthentication successful!")
			fmt.Println("Running archive update...")

			// Try again with the new tokens
			if err := archiver.Run(); err != nil {
				return fmt.Errorf("archive update failed after authentication: %w", err)
			}

			return nil
		}

		// Other types of errors
		return fmt.Errorf("archive update failed: %w", err)
	}

	return nil
}
