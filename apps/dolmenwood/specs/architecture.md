# Architecture

## Overview

Dolmenwood is a server-rendered web application for managing character sheets for the Dolmenwood tabletop RPG (an OSR/B/X-style game). It runs on the monks.co tailnet and is accessed via the proxy.

## Tech Stack

- **Go** server with `net/http` (Go 1.22+ method-based routing)
- **templ** for HTML templates (compiled to Go)
- **HTMX** for interactive partial-page updates (no client-side JS framework)
- **Tailwind CSS** via CDN
- **SQLite** with GORM ORM for persistence
- **Tailscale** for networking (served on tailnet)

## Package Structure

```
apps/dolmenwood/
  main.go          -- Entrypoint: opens DB, creates server, serves on tailnet
  tasks.toml       -- Build task configuration (dev/build/templ)
  db/              -- Persistence layer (SQLite + GORM)
    db.go          -- Schema, models, CRUD, migrations
    db_test.go
  engine/          -- Pure game rules engine (stateless, no I/O)
    abilities.go   -- Ability score modifiers, AC from armor
    advancement.go -- Class advancement tables (all 9 classes)
    bank.go        -- Bank deposit maturity, withdrawal planning
    breggle.go     -- Breggle kindred abilities (gaze, horns)
    calendar.go    -- Dolmenwood calendar (12 months, wysendays)
    companions.go  -- Companion breeds, saddles, barding, retainers
    encumbrance.go -- Slot-based encumbrance, speed calculation
    class.go       -- Class helpers (advancement parsing, primes, validation)
    equipment.go   -- Item catalog (weapons, armor, containers, weights)
    human.go       -- Human XP bonus
    knight.go      -- Deprecated knight wrappers
    magic_resistance.go -- Magic resistance by kindred + WIS
    moon.go        -- Moon signs from birthday
    save_bonuses.go -- Conditional save bonuses from traits/moon
    traits.go      -- Kindred and class traits (level-dependent)
    wealth.go      -- Coin parsing, formatting, purse calculations
    xp.go          -- XP modifiers, level-up detection
  server/          -- HTTP handlers, views, templates
    server.go      -- Route registration (46 routes)
    handlers.go    -- Handler implementations (~1500 lines)
    views.go       -- View model construction (CharacterView)
    views_test.go
    store.go       -- Store/shop buying, selling, changemaking
    font.go        -- Embedded web fonts (4 woff2 files)
    *.templ        -- Template files (14 templates)
    *_templ.go     -- Generated Go code from templ (do not edit)
  rules/           -- Markdown reference docs for Dolmenwood RPG rules (54 files)
```

## Three-Layer Architecture

1. **`db`** -- Persistence only. Defines GORM models, runs migrations, provides CRUD methods. The one exception is `ReturnToSafety()` which computes XP from found treasure.
2. **`engine`** -- Pure stateless game rules. Every function takes inputs and returns outputs with no side effects, no database access, no I/O. This makes the engine fully testable in isolation.
3. **`server`** -- HTTP handlers that bridge `db` and `engine`. Handlers parse forms, call DB methods, call engine functions, and render templ components.

## Key Data Flow

1. HTTP request arrives at a handler in `server/handlers.go`
2. Handler parses form data and loads DB records
3. Handler calls `engine` functions for game calculations
4. Handler updates DB records
5. Handler builds a `CharacterView` (via `views.go:buildCharacterView()`)
6. Handler renders templ components with the view model
7. Response is either a full page or an HTMX partial update

## HTMX Patterns

Most form submissions use HTMX to update specific page sections without full reloads:

- `hx-post` on forms targets specific card sections
- `hx-target` / `hx-swap` for replacing section content
- OOB (Out of Band) swaps: handlers return multiple HTML fragments to update several sections at once (e.g., updating inventory also updates encumbrance and companion sections)

## Build System

Defined in `tasks.toml`:
- `templ` task: watches `**/*.templ`, runs `go tool templ generate ./server/`
- `build` task: watches `**/*.go`, depends on `templ`, runs `go build`
- `dev` task: long-running, depends on `build`, runs the binary
- Build uses `SQLITE_ENABLE_LOCKING_STYLE=1` environment variable

## Entrypoint (`main.go`)

The `run()` function:
1. Opens SQLite database via `db.New()`
2. Creates server via `server.New(d)`
3. Mounts routes at both `/` (direct tailnet) and `/dolmenwood/` (proxy access)
4. Applies middleware: request logging (`reqlog`) and gzip compression
5. Waits for tailnet readiness, then starts serving
6. Graceful shutdown via signal-based context (`sigctx`)
