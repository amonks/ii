# Air

## Overview

Home environmental monitoring dashboard. Polls sensor readings from Aranet4
CO2/temperature/humidity/pressure sensors, stores historical data in SQLite,
and serves an interactive web page with D3.js time-series graphs.

Code: [apps/air/](../apps/air/)

## Modes

The binary runs in one of three modes via the `-mode` flag:

- `fetch` (default): one-shot data pull from the aranet service over the
  tailnet, inserted into SQLite.
- `serve`: HTTP server rendering the dashboard.
- `aggregates`: recalculates all window aggregates from raw data (recovery).

## Data Model

### data_points

Raw sensor readings: `(id, created_at, room, device, parameter, value)`.

### window_aggregates

Pre-computed aggregates keyed on `(room, device, parameter, window_duration,
window_start_at)` with `min`, `max`, `mean`, `count`. Hourly windows for
<= 30 days, daily windows for longer ranges.

On each insert, aggregates are updated incrementally:
`mean += (value - mean) / (count + 1)`.

## Devices

Five hardcoded Aranet4 devices mapped to rooms: `office`, `banjo`, `fridge`,
`tv`, `bedroom`. Parameters per device: temperature (°F), humidity, CO2,
pressure, battery.

## Fetch Behavior

In fetch mode, the app spins up an ephemeral tsnet node and makes an HTTP
GET to `http://monks-aranet-brigid/`. Uses `If-Modified-Since` /
`Last-Modified` headers to avoid re-inserting duplicate data.

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Dashboard with D3.js graphs. `?days=N` controls range (default 3). |

## Deployment

Runs on **thor** (local server). Database at `/data/tank/venta/venta.db`.
Depends on the [aranet](aranet.md) app for sensor data.
