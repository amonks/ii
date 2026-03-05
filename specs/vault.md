# Vault

## Overview

Vault is an SFTP server on thor that receives litestream replicas from all
apps with SQLite databases. It stores replicas on a ZFS dataset, providing
continuous backup with point-in-time recovery via ZFS snapshots.

Code: [apps/vault/](../apps/vault/)

## Architecture

### SFTP Server

Vault uses [charm.sh/wish](https://github.com/charmbracelet/wish) to run
an SSH server with an SFTP subsystem. It listens only on the tailnet
(port 22 on its tsnet address `monks-vault-thor`). No authentication is
configured — the tailnet is the trust boundary.

On each SSH connection, the server calls `tailnet.WhoIs` on the remote
address to identify the connecting peer. Only peers tagged `tag:monks-co`
are accepted. The SFTP session is rooted at `$VAULT_ROOT/<machine-name>/`,
where the machine name comes from the WhoIs response (e.g.,
`monks-ci-fly-ord`). The client has no control over its directory.

### Storage Layout

```
$VAULT_ROOT/
  monks-ci-fly-ord/
    ci.db/
      ltx/0/...
      ltx/1/...
  monks-ping-brigid/
    ping.db/
      ltx/0/...
  monks-dogs-fly-ord/
    dogs.db/
      ltx/0/...
```

ZFS snapshots on the vault dataset provide point-in-time recovery on top
of litestream's continuous WAL shipping.

### Restore

To restore a database, use `fly ssh sftp` or connect to the vault's SFTP
server directly and retrieve the litestream replica files.

## Environment Variables

| Variable     | Purpose                          |
| ------------ | -------------------------------- |
| `VAULT_ROOT` | Root directory for replica storage (e.g., `/data/zfs/vault`) |

## Replication Client (pkg/database)

The `pkg/database` package integrates litestream replication into
`database.Open`. When the tailnet is ready and the path is not `:memory:`,
Open starts a litestream Store that continuously replicates WAL changes
to vault over SFTP.

The replication uses a custom `ReplicaClient` implementation
(`vaultClient`) that dials through tsnet rather than standard `net.Dial`.
This is necessary because on Fly.io, apps use embedded tsnet (userspace
networking) and cannot reach tailnet hosts via the system network stack.

### Startup Order

All apps must call `tailnet.WaitReady(ctx)` before opening any database.
This ensures the tailnet is available for litestream to connect to vault.

### Test Databases

`:memory:` databases and databases opened when the tailnet is not ready
(i.e., in tests) skip replication. This is not a toggle — it reflects
that replication is physically impossible without network connectivity.
