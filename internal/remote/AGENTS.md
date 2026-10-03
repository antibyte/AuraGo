# Remote execution

## Purpose

Authenticated remote device transport, enrollment and command/result lifecycle.

## Ownership

This package owns connection identity; server routes own HTTP/WebSocket acceptance.

## Local Contracts

- A reader owns only the socket actually authenticated by enrollment. Peer-supplied device IDs never select another socket. Bound authentication frames/time and pending enrollment count; consume accepted enrollment tokens once.
- Stale disconnects and heartbeats cannot replace a newer connection's state. Synchronize telemetry/read-only access. Deliver each result without blocking the reader on duplicate messages.
- HTTP drain closes accepted sockets so remote handlers finish before DB shutdown.

## Work Guidance

Keep device identity/HMAC, nonce and timestamp checks together; failed authentication closes its own socket.

## Verification

`go test ./internal/remote` and server `TestRemoteHandshake*`; use Linux CGO race tests for transport state.

## Child DOX Index

None.
