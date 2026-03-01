# dungeon

Player-facing TTRPG mapping tool for real-time collaborative cartography during play sessions. Two map modes: dungeon (square grid with rooms, walls, doors) and hex (exploration with explored/unexplored hexes). Optimized for phone-first touch interaction.

## Code

- Entry point: [apps/dungeon/main.go](../apps/dungeon/main.go)
- Database: [apps/dungeon/db/](../apps/dungeon/db/)
- Server: [apps/dungeon/server/](../apps/dungeon/server/)
- TypeScript: [apps/dungeon/ts/](../apps/dungeon/ts/)
- CSS: [apps/dungeon/css/](../apps/dungeon/css/)

## Host

brigid (local server)

## Access

Public (autogroup:danger-all)

## Data Model

### Maps

Each map has a name and type (`"dungeon"` or `"hex"`).

### Cells

Grid cells keyed by `(map_id, x, y)`. Fields: `is_explored`, `text` (notes), `hue` (nullable 0-360 color angle), `room_id` (nullable, groups cells into rooms in dungeon mode). Both hex and dungeon modes use the same cell model; hex mode uses offset coordinates.

### Walls

Dungeon mode only. Stores wall overrides between adjacent cells. By default, walls are drawn between cells that belong to different rooms (or between a room cell and empty space); cells in the same room have no wall between them. This table stores exceptions: `"open"` (explicitly no wall, for backwards compatibility) or `"door"`. Keyed by the two adjacent cells `(x1,y1)-(x2,y2)`, normalized so the smaller coordinate pair comes first.

### Markers

Letter markers placed on cells for stairs, portals, etc. One marker per cell position.

## Architecture

Standard monks.co app pattern: `db.New()` → `server.New(db)` → `server.Mux()`. The server package contains an SSE pub/sub `Hub` for real-time broadcasting. On every mutation (cell update, wall change, marker add/remove), the handler publishes an event to the hub. Clients connect via EventSource and receive JSON events.

### SSE Hub

In-memory pub/sub with per-map subscriber channels. Subscribers connect via `GET /maps/{id}/events/`. Events are JSON with `type` and `data` fields. The hub drops events for slow subscribers rather than blocking.

### Frontend

HTML5 Canvas rendering with TypeScript. Pan/zoom via two-finger touch or mouse wheel. Tool system with state machine pattern. SSE client for receiving real-time updates. HTTP POST for sending mutations.

## API Routes

| Method | Path | Purpose |
| ------ | ---- | ------- |
| GET | `/` | Map index page |
| POST | `/maps/` | Create map (form: name, type) |
| GET | `/maps/{id}/` | Map view page with canvas |
| GET | `/maps/{id}/state/` | Full map state as JSON |
| GET | `/maps/{id}/events/` | SSE stream |
| POST | `/maps/{id}/cells/` | Create/update cells (JSON array) |
| POST | `/maps/{id}/walls/` | Create/update wall override (JSON) |
| POST | `/maps/{id}/markers/` | Create/update marker (JSON) |
| POST | `/maps/{id}/markers/delete/` | Remove marker (JSON: x, y) |

## Tools

### Dungeon Mode
- **Select** — drag to box-select rooms; rooms fully enclosed by the drag box are selected; opens properties panel
- **Box** — drag to draw a rectangle; if a room is selected, drawn cells merge into that room (merge mode); if no room is selected, only unoccupied cells become a new room (subtract mode), enabling ring-shaped rooms by drawing concentric boxes
- **Door** — hover highlights the nearest wall edge; tap to place/toggle a door
- **Letter** — tap a cell to place/remove a letter marker

### Hex Mode
- **Select** — tap a hex to select; opens properties panel
- **Paint** — tap/drag to toggle explored state

## Properties Panel

Slides up from bottom when cells are selected. Provides:
- Explored toggle
- Hue color swatches (9 preset colors + none)
- Text/notes field with debounced auto-save

## Dependencies

- `pkg/database` — SQLite with GORM
- `pkg/gzip` — response compression
- `pkg/reqlog` — request logging
- `pkg/sigctx` — signal-based context
- `pkg/tailnet` — tailnet membership
- `templ` — HTML templating
- `esbuild` — TypeScript bundling
- `tailwindcss` — CSS utility framework
