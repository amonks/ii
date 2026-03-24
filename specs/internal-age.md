# Internal Age

## Overview
The age package calculates display durations for items that may be active or completed.

## Responsibilities
- `AgeData` returns `now - startedAt` and a flag indicating whether timing data exists, clamping to zero when `now` precedes `startedAt`.
- `Age` returns the computed age, defaulting to zero.
- `DurationData` returns a duration and a flag indicating whether timing data exists.
- Active items require `startedAt`; the duration is `now - startedAt`, clamped to zero when `now` precedes `startedAt`.
- Completed items prefer an explicit `durationSeconds`, falling back to `completedAt - startedAt`, clamped to zero when the timestamps are inverted.
- Returns `(0, false)` when no timing data is available.
