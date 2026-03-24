# Internal UI

## Overview
The ui package provides formatting helpers for CLI output.

## Time Formatting
- `FormatTimeAgo` returns a compact age string like `2m ago`.
- `FormatTimeAgeShort` returns a compact age string without suffix.
- `FormatDurationShort` formats durations in `s/m/h/d` units.

## ID Highlighting
- `HighlightID` emphasizes unique prefixes when ANSI output is available.
- ANSI output is disabled for `NO_COLOR`, `TERM=dumb`, or non-terminals.
- `UniqueIDPrefixLengths` delegates to `internal/ids` for prefix computation.
- `PrefixLength` returns the unique prefix length for case-insensitive ID lookups.

## Table Formatting
- `FormatTable` uses lipgloss tables to size output to the current viewport width.
- Viewport width detection prefers stdout, then falls back to stderr when stdout is not a terminal.
- `FormatTable` normalizes table output for CLI listings.
- `FormatTable` disables wrapping and relies on truncation plus viewport sizing to keep rows single-line.
- `TruncateTableCell` enforces width limits while respecting visible characters.
- `TableBuilder` provides a small helper to collect rows and render tables.
