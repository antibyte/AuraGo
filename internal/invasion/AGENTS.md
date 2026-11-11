# Invasion worker trust and deployment

## Purpose and ownership

`internal/invasion` owns Egg/nest state and deployment; `bridge` owns the shared
Master/Egg WebSocket protocol. These contracts also apply to server handlers and
the Egg runtime in `cmd/aurago`.

## Local contracts

- `enabled` remains operator-owned even when the database is initialized.
  Master HTTP admission and the hub's live `Policy` callback enforce enabled
  and read-only state. Disabling closes connections without stopping remote
  processes; registering and sending recheck policy at their shared boundary.

- Protocol v3 signs the structured envelope, including ID, identities, version,
  direction, server challenge, sequence and payload. Every socket gets a random
  256-bit challenge. Accept only increasing sequences in that session, timestamps
  at most two minutes old or thirty seconds ahead, and the authenticated peer's
  identities. The handshake acknowledgement is authenticated and correlated.
- Update Master and Eggs together. Legacy connections fail with
  `invasion_protocol_upgrade_required`; never add an unsigned/legacy fallback.
  HMAC authenticates traffic; use WSS or an authenticated encrypted network for
  transport confidentiality.
- Generated Eggs require security policy version 1. Reject tls_skip_verify and
  malformed pins at startup and before deployment. Missing/unreadable master
  certificates fail configuration generation before publishing credentials.
  A pinned leaf or a normally trusted chain for master_url remains supported.
- Only an active Egg assigned to the active nest may authenticate. Apply the
  pre-authentication frame limit and finite handshake/write budgets.
- Connection-owned cleanup, heartbeat expiry and acknowledgements bind to the
  exact socket generation. Replacement must survive an old reader's exit.
- Signing, sequencing, socket writes and key rotation are serialized. Heartbeat
  state and HTTP signing read keys under the owning lock. The Master accepts the
  previous key only while a rotation awaits the Egg's ack (at most sixty
  seconds) and drops it on commit or rollback. Preserve these bounds; protocol
  checks still apply under the previous key.
- Key rotation is two-phase. The handler reserves the nest
  (`EggHub.BeginKeyRotation`) and first checks that the vault's current key is
  the live connection's (`EggHub.ConnectionKeyMatches`): if only `_next`
  matches (a failed commit or promotion) it commits `_next` as persisted,
  otherwise it answers 409 without staging. It then stages
  `egg_shared_<nest>_next`, and `SendRekey` sends under the current key. The Egg persists the key
  (`EggClient.OnRekey` → vault `egg_shared_key`) before switching and acking
  under the new key with `AckPayload.Persisted` set; no handler, a version
  other than current+1, or a persist error is a signed rejection under the old
  key. The Master commits only after the ack and rolls back otherwise. With
  `Persisted` the commit is the new key alone (the old key dies at commit);
  an ack without it (an Egg predating the flag, key possibly only in memory)
  also keeps the replaced live key as dated `_prev`, unless a still-fresh
  `_prev` exists (that older key is what such an Egg's disk holds).
  `_next` is dropped either way (`commitEggKeyRotation`). A
  timed-out rotation stays unresolved (no new rotation) until the Egg's
  rejection arrives or the socket ends; `awaitAck` also stops when the socket
  closes. Frames the Master sends while a rekey is in flight are signed with
  the new key, so an Egg that rejects (or adopts late) drops the socket and
  those commands are lost (`TestCommandSentDuringRejectedRekeyIsLost`). The
  handshake reads all slots from one vault snapshot (a read error changes
  nothing) and tries current, `_next`,
  `_prev`; a `_next`/`_prev` match is promoted and the others removed, and a
  current-key match retires `_prev`. `_prev` is dated by `_prev_at` (written
  in the commit) and honoured only within `eggPrevKeyGrace` (one hour); an
  undated, expired or future-dated `_prev` is deleted at the next handshake.
  Re-hatching (`storeEggSharedKey`) replaces the key and drops every
  candidate atomically; when the hatch never delivered the new egg config
  (`replaceEggSharedKey` undo), the previous key and the candidate slots that
  are still empty come back
  (`TestReplaceEggSharedKeyRestoresRotationCandidatesWhenTheConfigWasNotDelivered`).
  The `egg_shared_` and `egg_master_key_` vault
  prefixes are reserved, and secrets sent to an Egg may not use them
  (send-secret handler and `EggClient` both refuse; `IsReservedEggSecretName`).
  Tests: `TestSendRekey*`, `TestEggRekeyAckCarriesPersistedFlagOnlyOnSuccess`,
  `TestEggRejectsSecretsWithReservedNames`,
  `TestEggRejectsRekeyWithUnexpectedVersion`,
  `TestBeginKeyRotationSerializesRotationsPerNest`,
  `TestHeartbeatAndRekeyRemainOrderedUnderConcurrentTraffic` (bridge) and
  `TestInvasionHandshake*`, `TestInvasionRehatchRevokesRotationCandidates`,
  `TestInvasionRotateKey*`, `TestEggPrevKeyFreshBoundaries`,
  `TestInvasionSendSecretRefusesReservedNames` (server).
- Generated YAML contains provider bindings and no master/HMAC/API keys.
  Docker and SSH deliver mode-0600 egg-bootstrap.json before startup. Import
  selected exports and explicit secret updates with Vault.WriteSecrets, retaining
  the local Vault key and unrelated entries. An atomic receipt prevents leftover
  inputs from overwriting later rotations. Failed import or removal blocks startup.
- Preserve existing .env keys. A Vault without its actual key requires authenticated
  operator recovery; never overwrite it with a newly generated key.
- Empty tool lists grant neither shell nor Python. Only the documented shell/Python
  selectors are valid. File writes, network and unsandboxed shell are explicit,
  default-off grants; unsandboxed shell additionally requires shell selection.
  Every applied configuration revision contains a full permission snapshot.
  Omitted patch fields retain rights; false and [] revoke. Legacy snapshots
  cannot be restored. Reconfiguration of legacy/uncertain state requires review.
- SSH transmits private content over encrypted stdin with a fixed command and
  atomically publishes mode-0600 files. Stop the old process before replacement.
- Remote Docker secret bootstrap requires TLS, mTLS or SSH. Plain HTTP is refused.
  Stop the previous container before writing its shared data volume. Only backups
  marked with the current Egg security version may be started by binary rollback.
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
- The Docker TLS configuration (`dockerTLSClientConfig`, with `DockerTLSMaterial` as a type alias) and the SSH Engine dialer (`dialDockerEngineOverSSH`) are thin wrappers over `internal/dockerhost/transport`, which the Docker Manager shares. Change TLS or tunnel behaviour there; the invasion names, the `dockerSSHSocketOpenTimeout` test hook (passed in as the socket-open budget) and every error text stay as they are.
- A hatch stores the new `egg_shared_<id>` before Deploy and puts the previous key back (vault compare-and-swap) only when Deploy returns `ErrEggConfigNotDelivered`: Docker before `copyConfigToContainer`, SSH before the config write. Never after a possible delivery or a rollback; marked errors keep their text.
- SSH eggs live in `~/.aurago-egg-<prefix>`: shell steps write `$HOME` (shellPath), SFTP gets the home-relative path (`sftpPath`, SFTP never expands `~`), the user unit uses `%h`. Every process lookup (deploy start, Stop, Status, HealthCheck) matches the user's `aurago` processes by executable (`<base dir>/aurago` or "(deleted)", also via the `pwd -P` path when `$HOME` is a symlink), never by command line (`pgrep -f`/`pkill -f` also match the remote shell, and a process-mode egg runs as `./aurago`). A hatch stops a running egg (SIGTERM, exe re-checked while waiting up to 10 s, then SIGKILL) before it starts the new one, also before `systemctl --user restart` of a permanent egg. Only `nohup` runs in the background, with stdin from /dev/null, so start commands return.
- `export_nest_secret` decides whether `include_vault` copies the nest secret (`nest_<id>`) into the egg vault. Rows from before the column migrate to 1, new nests start at 0, and updates without the field keep it. `InitDB` copies a pre-existing database to `<db>.pre-export-nest-secret.bak` (VACUUM INTO, never overwritten) before adding the column; a failed copy aborts migration. New Egg grant/snapshot columns also require a private, consistent pre-migration backup.

## Verification

The coordinated cutover and operator recovery path are documented in
`../../documentation/invasion-security.md`.

Run bridge and invasion tests, server Invasion/MissionRemote regressions, and
Linux CGO race tests for concurrent heartbeat, result and rekey traffic. Exercise
replayed authentication on a fresh socket and old-connection replacement cleanup.

## Child DOX index

None. This file also owns `bridge/`.
