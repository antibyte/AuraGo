# Remote execution

## Purpose

Authenticated remote device transport, enrollment and command/result lifecycle.

## Ownership

This package owns connection identity; server routes own HTTP/WebSocket acceptance.

## Local Contracts

- A reader owns only the socket actually authenticated by enrollment. Peer-supplied device IDs never select another socket. Bound authentication frames/time and pending enrollment count; consume accepted enrollment tokens once.
- Stale disconnects and heartbeats cannot replace a newer connection's state. Synchronize telemetry/read-only access. Deliver each result without blocking the reader on duplicate messages.
- A device's effective allowed_paths is its own list, or the global `remote_control.allowed_paths` when its own list is empty (`effectiveAllowedPaths`). Connections hold the device's own list. Effective ReadOnly is global OR per-device; never persist a global restriction into the individual setting. Read runtime settings from one `RuntimePolicy` snapshot. On reload invalidate admission before pushing a complete snapshot through `RefreshPolicy`. Classic agents confirm authentication/config snapshots with an ACK bound to the message ID and exact connection; close after ten seconds without confirmation. Restrictive changes cancel affected work. On the wire a present `allowed_paths` (even empty) replaces the agent's list; an omitted one means unchanged. Clearing a device list falls back to the global default; clearing both revokes file access. AgoDesk records (tag `agodesk`) never get the global default and retain local permissions; report server enforcement separately from client confirmation. Command results and disconnect/status changes belong to the exact connection generation.
- Classic clients and AgoDesk require WSS, including loopback. Honor TLS termination only through the existing trusted-proxy boundary; client TLS certificate verification is mandatory. Preserve external hostnames/ports in downloads. Classic authentication requires `ClientSecurityRevision` and capabilities before token consumption or activation; refuse older clients with `CLIENT_UPGRADE_REQUIRED`.
- Both enrollment paths use `FinalizeEnrollment`: consume a valid token and activate one device transactionally. Clean up failed provisional records and keys, never reactivate a consumed token. Revocation persists the block before invalidating all sessions and transports.
- Writes use a propagated context and at most five seconds or the shorter command deadline. Closing/cancelling must not wait for a writer lock. Snapshot connections before heartbeat I/O; read telemetry through synchronization. Missing telemetry is JSON null, not a measured zero.
- The classic executor admits at most four commands before creating goroutines. Keep all active IDs and at most 1,024 completed IDs/32 MiB cached payload; evicted payload tombstones prevent replay. Text output has a visible 1 MiB truncation marker; binary reads fail on size overflow. Frame limits account for the configured file limit and encoding.
- All classic file/editor/search operations use authorized `os.Root` handles with bounded actual reads and final writes. Serialize edits per file, preserve permissions, sync a temporary file and publish through `fileutil.RenameRootContext` under the connection's publication gate. Cancellation or revoked rights must leave the original intact.
- Service installers store protected configuration before starting, pass an absolute `--config` path without secrets, and preserve a usable identity on failed enrollment/migration. Windows uses SCM; systemd uses `/var/lib/aurago-remote`, Windows uses ProgramData and macOS uses the user's Application Support directory. An explicit replacement token forces enrollment without first destroying the stored identity.
- Shell operations (`IsShellOperation`) require a non-empty effective allowed_paths; the hub refuses them before dispatch (`ShellRequiresAllowedPathsCode`) and the agent refuses them again. The hub check applies only to devices with a RemoteConnection: without one, dispatch reports the missing connection, and commands carried by a command transport (AgoDesk) are exempt because that client never receives allowed_paths and gates shell access locally.
- Private/loopback addresses never authenticate enrollment. Manual approval atomically replaces a pending observation with a fresh one-time token shown only to the administrator.
- Enrollment frames carry only the token's lookup hash (`remote_enrollments.token_hash`); the MAC key lives in the vault as `remote_enroll_key_<id>` for the token's lifetime and never travels in a frame. Auth responses echo the auth frame's nonce (`request_nonce`) only when it is well formed; refusals before a frame verifies are unsigned. Issue tokens and sweep keys only through the hub (`IssueEnrollmentToken`, `SweepExpiredEnrollments`), which serializes them.
- Frames are version 2 (`"v": 2`) and signed over the length-prefixed canonical form in `hmacData`. Unversioned frames are never verified; they get only fixed refusals, signed in the old form for pre-upgrade agents (enrollment, or reconnect of a known device) and unsigned otherwise.
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
