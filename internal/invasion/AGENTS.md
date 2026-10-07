# Invasion worker trust and deployment

## Purpose and ownership

`internal/invasion` owns Egg/nest state and deployment; `bridge` owns the shared
Master/Egg WebSocket protocol. These contracts also apply to server handlers and
the Egg runtime in `cmd/aurago`.

## Local contracts

- Protocol v2 signs the structured envelope, including ID, identities, version,
  direction, server challenge, sequence and payload. Every socket gets a random
  256-bit challenge. Accept only increasing sequences in that session, timestamps
  at most two minutes old or thirty seconds ahead, and the authenticated peer's
  identities. The handshake acknowledgement is authenticated and correlated.
- Update Master and Eggs together. Legacy connections fail with
  `invasion_protocol_upgrade_required`; never add an unsigned/legacy fallback.
  HMAC authenticates traffic; use WSS or an authenticated encrypted network for
  transport confidentiality. Do not enable TLS exceptions automatically.
- Only an active Egg assigned to the active nest may authenticate. Apply the
  pre-authentication frame limit and finite handshake/write budgets.
- Connection-owned cleanup, heartbeat expiry and acknowledgements bind to the
  exact socket generation. Replacement must survive an old reader's exit.
- Signing, sequencing, socket writes and key rotation are serialized. Heartbeat
  state and HTTP signing read keys under the owning lock. The Master permits one
  previous key for sixty seconds for messages already in flight. Preserve these
  bounds; protocol checks still apply under the previous key.
- SSH deployment and reconfiguration transmit secret-bearing file content over
  encrypted stdin with a fixed exec command. Publish private mode-0600 temporary
  files atomically as the configured SSH user; keep the last valid file on error.
- `docker_remote` nests with an empty `docker_tls` keep plain HTTP (default port
  2375). `tls`/`mtls` use HTTPS (default 2376), TLS 1.2+, the stored CA or system
  roots and never skip verification; unusable material fails every request
  instead of falling back to HTTP. PEMs live only in the vault under
  `nest_docker_tls_<id>` (never DB, API responses or logs) and are removed with
  the nest or when TLS is switched off. Updates without `docker_tls` keep the mode.
- `docker_ssh` reaches `/var/run/docker.sock` via direct-streamlocal through
  `remote.DialSSH` (known_hosts unless the global opt-in); one SSH client per
  Engine connection, closed with it (`DisableKeepAlives` must stay on); the
  credential is the nest's SSH secret, never TLS material; older binaries map it
  to the SSH binary deploy. Its version probe allows 20 s (the 10 s SSH dial
  budget plus the socket open and `/version`); other transports keep the default probe.
- A hatch stores the new `egg_shared_<id>` before Deploy and puts the previous key back (vault compare-and-swap) only when Deploy returns `ErrEggConfigNotDelivered`: Docker before `copyConfigToContainer`, SSH before the config write. Never after a possible delivery or a rollback; marked errors keep their text.
- SSH eggs live in `~/.aurago-egg-<prefix>`: shell steps write `$HOME` (shellPath), SFTP gets the home-relative path (`sftpPath`, SFTP never expands `~`), the user unit uses `%h`. Every process lookup (deploy start, Stop, Status, HealthCheck) matches the user's `aurago` processes by executable (`<base dir>/aurago` or "(deleted)", also via the `pwd -P` path when `$HOME` is a symlink), never by command line (`pgrep -f`/`pkill -f` also match the remote shell, and a process-mode egg runs as `./aurago`). A hatch stops a running egg (SIGTERM, exe re-checked while waiting up to 10 s, then SIGKILL) before it starts the new one, also before `systemctl --user restart` of a permanent egg. Only `nohup` runs in the background, with stdin from /dev/null, so start commands return.
- `export_nest_secret` decides whether `include_vault` copies the nest secret (`nest_<id>`) into the egg vault. Rows from before the column migrate to 1, new nests start at 0, and updates without the field keep it. `InitDB` copies a pre-existing database to `<db>.pre-export-nest-secret.bak` (VACUUM INTO, never overwritten) before adding the column; a failed copy is logged and the migration still runs.

## Verification

Run bridge and invasion tests, server Invasion/MissionRemote regressions, and
Linux CGO race tests for concurrent heartbeat, result and rekey traffic. Exercise
replayed authentication on a fresh socket and old-connection replacement cleanup.

## Child DOX index

None. This file also owns `bridge/`.
