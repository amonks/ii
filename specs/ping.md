# Ping

## Overview

Personal relationship management tool — a "keep in touch" tracker. Maintains
a list of people to stay in contact with and logs pings (contact events)
with timestamped notes. Integrates with Beeminder for accountability.

Code: [apps/ping/](../apps/ping/)

## Data Model

### people

| Column | Type | Notes |
|--------|------|-------|
| `slug` | text PK | Person identifier |
| `is_active` | bool | Whether to include in active tracking |

### pings

| Column | Type | Notes |
|--------|------|-------|
| `person_slug` | text FK | |
| `at` | datetime | Contact timestamp |
| `notes` | text | Free-text notes |

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | List all people sorted by last ping time |
| GET | `/person/` | Detail view (`?slug=`) |
| POST | `/commands/bump/` | Post a Beeminder datapoint without pinging a person |
| POST | `/commands/ping-person/` | Record a ping; also posts to Beeminder if target is the longest-unpinged active person |
| POST | `/commands/add-person/` | Add a new person, optionally with initial ping |
| POST | `/commands/update-person/` | Toggle active/inactive status |

## Beeminder Integration

The key concept is the **longest-unpinged** active person. On the list page,
`listPeople()` performs a LEFT JOIN to find each person's most recent ping,
then identifies the active person with the oldest last-ping. That person is
shown bold.

When `PingPerson` is called for the longest-unpinged person, a Beeminder
datapoint is posted (goal: `"ping"`, user: `"ajm"`). This means Beeminder
only gets credit when clearing the overdue contact. The `Bump` command lets
you credit Beeminder independently.

## Deployment

Runs on **brigid** (local server). Database at `$MONKS_DATA/ping.db`.
Private access tier.
