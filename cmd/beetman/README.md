# beetman

A Go program that manages the import of music albums into a beet library. It maintains a SQLite database to track album import status and provides both interactive and non-interactive modes for handling imports.

## Features

- Tracks album import status in a SQLite database
- Supports both interactive and non-interactive import modes
- Automatically discovers new albums
- Handles skipped albums that need manual intervention
- Retries skipped albums automatically without interaction
- Retries failed albums one by one with optional beet removal
- Reports album stats by status
- Searches skipped albums by query
- Ensures only one instance runs at a time

## Installation

1. Ensure you have Go installed
2. Clone this repository
3. Run `go install ./beetman` from the `cmd/beetman` directory

## Usage

```
beetman [--data-dir=PATH] [--flac-dir=PATH] <command>

Commands:
  setup         Initialize database and process existing albums
                Required flags:
                  --cutoff-time    Timestamp before which albums are considered processed
                  --previous-log   Path to existing beet import log file
  import        Discover and import new albums (skipping those requiring interaction)
  handle-skips  Import previously skipped albums that need interaction
  handle-skip   Import previously skipped albums matching a search query
  handle-errors Retry failed albums one by one
  stats         Get album stats
  retry-skips   Retry importing previously skipped albums without interaction

Flags:
  --data-dir    Path to the application data directory (default: ~/.local/share/beet-import-manager)
  --flac-dir    Path to the directory containing FLAC albums (default: ~/mnt/whatbox/files/flac)
```

## Data Storage

The program stores its data in `~/.local/share/beet-import-manager/`:
- `db.sqlite`: SQLite database containing album import status
- `lock`: Lock file to prevent multiple instances from running

## Development

### Project Structure

```
.
├── beetman/
│   └── main.go               # CLI entrypoint
├── internal/
│   ├── albums/                # Album directory management
│   ├── beet/                  # Beet command execution
│   ├── database/              # Database interface and implementation
│   ├── fixtures/              # Test fixtures
│   ├── log/                   # Log parsing utilities
│   └── mockbeet/              # Mock beet for testing
├── manager.go                 # Core manager logic
├── lock.go                    # Process lock management
└── README.md
```

### Testing

```bash
go test ./...
```

## License

MIT License
