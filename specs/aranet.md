# Aranet

## Overview

Bluetooth scanning service that continuously scans for Aranet4 CO2 sensors
via BLE, caches the latest readings in memory, and exposes them as a JSON
API over the tailnet. Acts as a hardware bridge between physical sensors and
the [air](air.md) app.

Code: [apps/aranet/](../apps/aranet/)

## Scanning

Uses `pkg/aranet4` which wraps `tinygo.org/x/bluetooth`. Scans for devices
whose local name starts with `"Aranet4"`, reading 22-byte manufacturer data
(company ID `0x0702`) to extract CO2, temperature, humidity, pressure, and
battery readings.

Configured for 5 devices with a 1-minute scan timeout. A background
goroutine scans immediately on startup, then every minute via a ticker.
Partial results (fewer than 5 devices found) are accepted.

Invalid readings are flagged via `*Valid bool` fields rather than omitting
the device.

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | JSON response with `timestamp` and `devices` array. Sets `Last-Modified` header. |

## Data Storage

None. In-memory only: a `DeviceData` struct protected by `sync.RWMutex`.

## Deployment

Runs on **brigid** (local server with Bluetooth hardware).
