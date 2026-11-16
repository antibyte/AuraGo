# Embedded Tailscale lifecycle

## Purpose

Manage the embedded Tailscale node and its private/public HTTP exposures.

## Ownership

`internal/tsnetnode` owns config snapshots, operations, authentication and listener cleanup. Server handlers publish snapshots and expose existing operation status.

## Local Contracts

- `UpdateConfig` disables the runtime at the manager boundary: reject new operations, cancel active work and schedule one bounded asynchronous stop. A config save may arrive through the very HTTP server being stopped.
- Re-enabling does not start a node. Explicit starts wait until pending disable cleanup completes.
- Publication checks cancellation and current enablement under the resource mutex; main-node listeners also check node identity. Child nodes retain their generation checks. Late results close their resources.
- Stop detaches and closes main/Funnel, Homepage, Speech Lab, Manifest, Space Agent and Store resources. Keep operation snapshots immutable and errors free of credentials/state contents.
- All managed backend proxies use direct transports, including Homepage, Speech Lab, Manifest, Space Agent and Store routes. Never forward their traffic through environment proxies. Register node/shared/environment auth keys when resolving them, before tsnet receives them.

## Work Guidance

- Preserve operation serialization and the lock order `operationMu` then `mu`. Never wait for shutdown while holding either lock.

## Verification

- Run `go test -p 1 ./internal/tsnetnode`, its Linux race check, and server `TestTsNet*`/config exposure checks. Fixtures must not contact a real tailnet.

## Child DOX Index

None.
