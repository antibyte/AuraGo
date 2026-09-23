# MQTT

## Purpose

Broker configuration, subscriptions, relays, and mission dispatch.

## Ownership

`internal/mqtt` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### MQTT Configuration Contract

- MQTT config API patches are typed and validated before Vault or YAML writes.
  Topics are arrays, QoS is numeric and zero remains valid. Config validation
  must clone mutable values before unmarshalling over an existing snapshot.
- The broker URL selects the transport. Secure schemes always install TLS;
  `tls.enabled` with a plaintext scheme is rejected without rewriting the URL
  or port. Explicit CA and client certificate material must validate before
  publication. Client certificates require both certificate and key.
- MQTT credentials resolve from Vault `mqtt_password`, then raw `MQTT_PASSWORD`,
  then empty. Preserve whitespace and never retain stale credentials after a
  deletion. Vault mutations publish a fresh config snapshot; expose credential
  source only, never the value.
- The server owns an `MQTTController` even when disabled. Snapshot publication
  only enqueues immutable desired settings; network reconciliation is serialized
  outside the config lock. Connection-affecting edits retire the old generation
  before starting its replacement. Logical edits retain the open connection.
- Status separates desired and applied revisions; `connected` requires an open
  confirmed connection. Test connections use their own disposable client and a
  context-bound socket, including TLS and WebSocket handshakes. Shutdown cancels
  and joins MQTT workers before closing agent/database dependencies.
- Exact subscription filters have independent config, Frigate, manual and
  mission-key owners; use their maximum requested QoS. Only a matching SUBACK
  grant of 0, 1 or 2 establishes success. Preserve failed desired work and report
  partial grants separately. Manual removal must preserve other owners.
- Persistent-session ledgers contain ownership and confirmed/pending filter
  changes only. Persist atomically per broker/client identity; never store
  credentials or payloads. Restore confirmed manual owners, reconcile managed
  owners from current configuration and reject traffic outside desired filters.
- Mission dispatch uses one worker and at most 256 waiting jobs. Drop new work
  on overload and recheck registration/generation before execution; dropped or
  stale jobs never consume a trigger interval.
- MQTT and Frigate relays use separate internal autonomous sessions, exclude
  global chat history and suppress derived conversation, memory, personality
  and planner-reminder effects. Keep explicit authorized tools and operational
  issue recording available. Relay registration is synchronized and delivery
  uses the controller's cancellable context and bounded queue.
- Build direct runtime gates with `tools.RuntimePermissionsFromConfig` at
  startup, reload and agent dispatch. MQTT bridge/CYD gates resolve the live
  server snapshot independently of earlier agent turns; dispatch also retains
  the current run's stricter limits. Record CYD publication failures.
- Serialize native MQTT results as JSON. New Python MQTT skills must check
  CONNACK, immediate return codes, per-topic SUBACK and completed publication,
  bound reception buffers and clean up on failure. Never rewrite existing
  generated skills when updating the bundled template.

## Verification

- Run `go test ./internal/mqtt` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
