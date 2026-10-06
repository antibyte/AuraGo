# Remote execution

## Purpose

Authenticated remote device transport, enrollment and command/result lifecycle.

## Ownership

This package owns connection identity; server routes own HTTP/WebSocket acceptance.

## Local Contracts

- A reader owns only the socket actually authenticated by enrollment. Peer-supplied device IDs never select another socket. Bound authentication frames/time and pending enrollment count; consume accepted enrollment tokens once.
- Stale disconnects and heartbeats cannot replace a newer connection's state. Synchronize telemetry/read-only access. Deliver each result without blocking the reader on duplicate messages.
- Empty allowed_paths is an explicit revocation in full snapshots; only omitted partial fields mean unchanged. Command results belong to both the authenticated device and the exact connection generation.
- Private/loopback addresses never authenticate enrollment. Manual approval atomically replaces a pending observation with a fresh one-time token shown only to the administrator.
- Disabling Remote Control blocks new actions and registration, cancels pending commands, closes sockets and drains the heartbeat monitor; shutdown does the same.
- SSH TCP dialing and handshake have finite budgets; cancellation closes the transport throughout commands and SFTP. `DialSSH` applies the same budget and cancellation to dial and handshake only: afterwards the client is detached from ctx and the caller owns `Close`. Agent local transfers use an explicit os.Root, protected-path/write grants, synced temporary downloads and atomic publication. Uploads require the server atomic-rename extension.
- Secret-bearing deployment scripts use ExecuteRemoteCommand's optional stdin reader with a fixed command. Never embed Master keys, Vault bytes or encoded credentials in SSH exec arguments. Invasion publishes mode-0600 temporary files atomically as the configured SSH user; it does not escalate to root.
- HTTP drain closes accepted sockets so remote handlers finish before DB shutdown.

## Work Guidance

Keep device identity/HMAC, nonce and timestamp checks together; failed authentication closes its own socket.

## Verification

`go test ./internal/remote` and server `TestRemoteHandshake*`; use Linux CGO race tests for transport state.

## Child DOX Index

None.
