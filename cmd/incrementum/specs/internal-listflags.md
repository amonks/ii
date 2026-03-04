# Internal List Flags

## Overview
The listflags package centralizes shared Cobra flag definitions.

## Behavior
- `AddAllFlag` attaches a `--all` flag to list commands.
- When a target pointer is provided, it is bound to the flag value.
