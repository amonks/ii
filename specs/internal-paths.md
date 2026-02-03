# Internal Paths

## Overview
The paths package centralizes default filesystem locations for incrementum state, workspaces, and event logs. It uses shared helpers around `os.UserHomeDir` and `os.Getwd` to keep error handling consistent.

## Defaults
- State: `~/.local/state/incrementum`
- Workspaces: `~/.local/share/incrementum/workspaces`
- Job events: `~/.local/share/incrementum/jobs/events`

## API
- `HomeDir() (string, error)`: returns the current user's home directory using `os.UserHomeDir`.
- `DefaultStateDir() (string, error)`: returns the default state directory using `os.UserHomeDir`.
- `DefaultWorkspacesDir() (string, error)`: returns the default workspaces directory using `os.UserHomeDir`.
- `DefaultJobEventsDir() (string, error)`: returns the default job events directory using `os.UserHomeDir`.
- `WorkingDir() (string, error)`: returns the current working directory using `os.Getwd`, preferring a non-`/private` path when it resolves to the same location.
- `NormalizePath(path string) string`: normalizes a file path by removing macOS-specific prefixes like `/private` when the path refers to the same location. Works even for non-existent paths by checking ancestor directories. This handles the common case where `/var` → `/private/var` on macOS.
- `ResolveWithDefault(override string, defaultFn func() (string, error)) (string, error)`: returns the override if non-empty, otherwise calls defaultFn. Used to consolidate the common pattern of "use provided path or fall back to default".
