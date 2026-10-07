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
  transport confidentiality. Do not add automatic TLS exceptions beyond the
  missing-certificate fallback below.
- Eggs of a self-signed master pin its certificate: `GenerateEggConfig` writes
  `egg_mode.tls_pin_sha256` (SHA-256 hex of the DER leaf in the first
  CERTIFICATE block of `<data_dir>/certs/selfsigned.crt`, the path
  `server.NewTLSConfigFromConfig` uses). `EggClient` checks every HTTP and
  WebSocket handshake in `pinnedTLSConfig`'s `VerifyConnection` (a pin wins
  over `tls_skip_verify`): the leaf matches the pin (direct routes), or its
  chain verifies against the Egg host's trust store for the host of
  `master_url` (never the SNI, which is empty for IP addresses). The second
  branch accepts a `custom` route through a TLS-terminating proxy only when the
  proxy's certificate is trusted by the Egg host (public CA, or a private CA in
  the host trust store). The one automatic exception: an unreadable
  certificate at generation time falls back to `tls_skip_verify: true` with a
  warning so hatching keeps working. `tls_skip_verify` from older configs is
  still honoured, with a startup warning; a malformed pin is logged as an
  error at startup. Regenerating the master certificate locks pinned Eggs out
  until the master restarts and a safe-reconfigure (which regenerates the
  whole config) delivers the new pin. Tests: `TestGenerateEggConfig_*TLSPin*`,
  `TestGenerateEggConfig_SelfSignedWithoutCertificate_FallsBackToTLSSkipVerify`,
  `TestEggClientAcceptsPinnedCertAndRejectsOthers`,
  `TestEggClientAcceptsTrustedChainWhenPinDiffers`,
  `TestEggClientRejectsTrustedChainForAnotherHost`,
  `TestEggClientConnectDialsOnlyThePinnedMaster`, `TestPinnedTLSConfigVerifier`
  (bridge) and `TestCertRegenerateTellsOperatorToSafeReconfigureEggs` (server).
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
- The Egg's `config.yaml` must stay writable. The hatch-time
  `egg_mode.shared_key` is migrated into the vault and removed from the file
  at startup; if it cannot be removed, the overwriting migration copies the
  hatch-time key over a persisted rotation at every restart and the Egg is
  locked out.
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
