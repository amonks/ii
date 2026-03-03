package mockbeet

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Mock sets up a mock beet in the given directory and returns a cleanup function
func Mock(t *testing.T, dir string) func() {
	// Save original PATH
	origPath := os.Getenv("PATH")

	setup(t, dir)

	// Add mock directory to PATH
	newPath := fmt.Sprintf("%s%c%s", dir, os.PathListSeparator, origPath)
	if err := os.Setenv("PATH", newPath); err != nil {
		t.Fatalf("Failed to set PATH: %v", err)
	}

	// Return cleanup function
	return func() {
		os.Setenv("PATH", origPath)
	}
}

// setup creates a mock beet executable for testing
func setup(t *testing.T, tmpDir string) {
	// Create mock beet script
	mockScript := `#!/bin/bash

# Parse arguments
quiet=0
log_file=""
albums=()
command=""

while [ $# -gt 0 ]; do
    case "$1" in
        import)
            command="import"
            shift
            ;;
        rm)
            command="rm"
            shift
            ;;
        --quiet)
            quiet=1
            shift
            ;;
        -v)
            shift
            ;;
        -l)
            log_file="$2"
            shift 2
            ;;
        *)
            if [ "$command" = "rm" ]; then
                # For rm command, treat remaining args as query
                query="$1"
                shift
            else
                # For import command, treat as album paths
                albums+=("$1")
                shift
            fi
            ;;
    esac
done

# Handle rm command
if [ "$command" = "rm" ]; then
    # Mock successful removal
    exit 0
fi

# Handle import command
if [ "$command" = "import" ]; then
    # If no log file specified, error out
    if [ -z "$log_file" ]; then
        echo "Error: no log file specified" >&2
        exit 1
    fi

    # Create log file directory if it doesn't exist
    mkdir -p "$(dirname "$log_file")"

    # Create or truncate log file
    : > "$log_file"

    # Process each album
    has_error=0
    for album in "${albums[@]}"; do
        # Skip non-existent albums
        if [ ! -d "$album" ]; then
            echo "skip $album; does not exist" >> "$log_file"
            continue
        fi

        # Skip albums with "skip" in their name
        if [[ "$album" == *skip* ]]; then
            echo "skip $album; test skip condition" >> "$log_file"
            continue
        fi

        # Check for error albums
        if [[ "$album" == *error* ]]; then
            has_error=1
            continue
        fi

        # Otherwise, mark as added
        echo "added $album" >> "$log_file"
    done

    # Exit with error if any album had "error" in its name
    if [ "$has_error" = "1" ]; then
        exit 1
    fi

    exit 0
fi

# Unknown command
echo "Error: unknown command" >&2
exit 1`

	// Create mock beet executable
	mockPath := filepath.Join(tmpDir, "beet")
	if err := os.WriteFile(mockPath, []byte(mockScript), 0755); err != nil {
		t.Fatalf("Failed to create mock beet: %v", err)
	}
}
