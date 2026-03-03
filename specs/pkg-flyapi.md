# pkg/flyapi

## Overview

Thin REST client for the Fly.io Machines API
(`https://api.machines.dev/v1`). No external dependencies — stdlib
only.

Code: [pkg/flyapi/](../pkg/flyapi/)

## Client

```go
client := flyapi.NewClient(token, appName)
```

Creates a client with default base URL. The token is sent as
`Authorization: Bearer {token}` on all requests.

## Methods

- `CreateMachine(ctx, MachineCreateInput) (*MachineInfo, error)` —
  POST /apps/{app}/machines
- `WaitForState(ctx, machineID, targetState, timeout) error` —
  GET /apps/{app}/machines/{id}/wait?state={state}&timeout={seconds}
- `GetMachine(ctx, machineID) (*MachineInfo, error)` —
  GET /apps/{app}/machines/{id}
- `StopMachine(ctx, machineID) error` —
  POST /apps/{app}/machines/{id}/stop

## Types

- `MachineCreateInput`: Name, Region, Config
- `MachineConfig`: Image, Cmd, Guest, Env, Mounts, AutoDestroy,
  Restart
- `Guest`: CPUKind, CPUs, MemoryMB
- `Mount`: Volume, Path
- `RestartPolicy`: Policy
- `MachineInfo`: ID, Name, State, Region, CreatedAt, Config, Events
- `MachineEvent`: Type, Status, Timestamp
- `APIError`: StatusCode, Message (returned on non-2xx responses)
