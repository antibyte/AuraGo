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
  state and HTTP signing read keys under the owning lock. The Master accepts the
  previous key only while a rotation awaits the Egg's ack (at most sixty
  seconds) and drops it on commit or rollback. Preserve these bounds; protocol
  checks still apply under the previous key.
- Key rotation is two-phase. The handler reserves the nest
  (`EggHub.BeginKeyRotation`), stages `egg_shared_<nest>_next`, and
  `SendRekey` sends under the current key. The Egg persists the key
  (`EggClient.OnRekey` → vault `egg_shared_key`) before switching and acking
  under the new key; no handler, a version other than current+1, or a persist
  error is a signed rejection under the old key. The Master commits current +
  `_prev` and drops `_next` only after the ack and rolls back otherwise; a
  timed-out rotation stays unresolved (no new rotation) until the Egg's
  rejection arrives or the socket ends. The handshake tries current, `_next`,
  `_prev`; a `_next`/`_prev` match is promoted and the others removed, and a
  current-key match retires `_prev`. The `egg_shared_` vault prefix is
  reserved. Tests: `TestSendRekey*`, `TestEggRejectsRekeyWithUnexpectedVersion`,
  `TestBeginKeyRotationSerializesRotationsPerNest`,
  `TestHeartbeatAndRekeyRemainOrderedUnderConcurrentTraffic` (bridge) and
  `TestInvasionHandshake*`, `TestInvasionRotateKey*` (server).
- SSH deployment and reconfiguration transmit secret-bearing file content over
  encrypted stdin with a fixed exec command. Publish private mode-0600 temporary
  files atomically as the configured SSH user; keep the last valid file on error.

## Verification

Run bridge and invasion tests, server Invasion/MissionRemote regressions, and
Linux CGO race tests for concurrent heartbeat, result and rekey traffic. Exercise
replayed authentication on a fresh socket and old-connection replacement cleanup.

## Child DOX index

None. This file also owns `bridge/`.
