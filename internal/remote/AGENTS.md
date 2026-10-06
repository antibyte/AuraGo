# Remote execution

## Purpose

Authenticated remote device transport, enrollment and command/result lifecycle.

## Ownership

This package owns connection identity; server routes own HTTP/WebSocket acceptance.

## Local Contracts

- A reader owns only the socket actually authenticated by enrollment. Peer-supplied device IDs never select another socket. Bound authentication frames/time and pending enrollment count; consume accepted enrollment tokens once.
- Stale disconnects and heartbeats cannot replace a newer connection's state. Synchronize telemetry/read-only access. Deliver each result without blocking the reader on duplicate messages.
- A device's effective allowed_paths is its own list, or the global `remote_control.allowed_paths` (`DefaultAllowedPaths`) when its own list is empty (`effectiveAllowedPaths`). Connections hold the device's own list; the hub evaluates the global default live for its shell check and sends agents only effective lists (auth responses, config pushes). Agents receive a changed default on reload through `PushDefaultAllowedPaths` (devices without their own list) or at their next authentication. On the wire a present `allowed_paths` (even empty) replaces the agent's list; an omitted one means unchanged. An operator clearing the device list therefore falls back to the global default; only clearing both sends the empty, revoking list. AgoDesk records (tag `agodesk`) never get the global default (`EffectiveAllowedPaths`). Command results belong to both the authenticated device and the exact connection generation.
- Shell operations (`IsShellOperation`) require a non-empty effective allowed_paths; the hub refuses them before dispatch (`ShellRequiresAllowedPathsCode`) and the agent refuses them again. The hub check applies only to devices with a RemoteConnection: without one, dispatch reports the missing connection, and commands carried by a command transport (AgoDesk) are exempt because that client never receives allowed_paths and gates shell access locally.
- Private/loopback addresses never authenticate enrollment. Manual approval atomically replaces a pending observation with a fresh one-time token shown only to the administrator.
- Enrollment frames carry only the token's lookup hash (`remote_enrollments.token_hash`); the MAC key lives in the vault as `remote_enroll_key_<id>` for the token's lifetime and never travels in a frame. Auth responses echo the auth frame's nonce (`request_nonce`) only when it is well formed; refusals before a frame verifies are unsigned. Issue tokens and sweep keys only through the hub (`IssueEnrollmentToken`, `SweepExpiredEnrollments`), which serializes them.
- Frames are version 2 (`"v": 2`) and signed over the length-prefixed canonical form in `hmacData`. Unversioned frames are never verified; they get only fixed refusals, signed in the old form for pre-upgrade agents (enrollment, or reconnect of a known device) and unsigned otherwise.
- Disabling Remote Control blocks new actions and registration, cancels pending commands, closes sockets and drains the heartbeat monitor; shutdown does the same.
- SSH TCP dialing and handshake have finite budgets; cancellation closes the transport throughout commands and SFTP. Agent local transfers use an explicit os.Root, protected-path/write grants, synced temporary downloads and atomic publication. Uploads require the server atomic-rename extension.
- Secret-bearing deployment scripts use ExecuteRemoteCommand's optional stdin reader with a fixed command. Never embed Master keys, Vault bytes or encoded credentials in SSH exec arguments. Invasion publishes mode-0600 temporary files atomically as the configured SSH user; it does not escalate to root.
- HTTP drain closes accepted sockets so remote handlers finish before DB shutdown.

## Work Guidance

Keep device identity/HMAC, nonce and timestamp checks together; failed authentication closes its own socket.

## Verification

`go test ./internal/remote` and server `TestRemoteHandshake*`; use Linux CGO race tests for transport state.

## Child DOX Index

None.
