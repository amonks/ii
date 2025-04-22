package main

import (
	"fmt"
	"os"
	"strings"

	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/ports"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
)

const (
	archivePath = "/data/tank/mirror/reddit/"
	dbPath      = "/data/tank/mirror/reddit/.reddit.db"
	clientID    = "-RT9cp4AERMlAEhwR01isQ"
	secret      = "mgo2f7coeJj31sIZDsdIlLZfjBfSiA"
	username    = "richbizzness"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	// If first argument is "update", run the archive update process
	if len(os.Args) > 1 && os.Args[1] == "update" {
		return runUpdate()
	}
	
	// Default behavior - serve the web interface
	return runServer()
}

func runServer() error {
	port := ports.Apps["reddit"]

	db, err := NewModel()
	if err != nil {
		return fmt.Errorf("constructing model: %w", err)
	}

	ctx := sigctx.New()
	var errs error

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	s := newServer(db)
	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(s)); err != nil {
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