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

## Verification

Run bridge and invasion tests, server Invasion/MissionRemote regressions, and
Linux CGO race tests for concurrent heartbeat, result and rekey traffic. Exercise
replayed authentication on a fresh socket and old-connection replacement cleanup.

## Child DOX index

None. This file also owns `bridge/`.
