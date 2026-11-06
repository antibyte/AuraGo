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
- Reconciliation publishes copies of pending subscription/ledger maps under
  `subscriptionMu` before broker waits. Its subsequent unlocked result updates
  must never share those maps with status or ledger readers. Status remains
  available while SUBACK or unsubscribe replies are pending.
- Serialize desired-owner snapshot replacement with broker reconciliation using
  `subscriptionOpMu`; an older reply must never overwrite newer owners. Verify
  `TestMQTTBrokerReplyCannotOverwriteNewSubscriptionOwners`.
- Persistent-session ledgers contain ownership and confirmed/pending filter
  changes only. Persist atomically per broker/client identity; never store
  credentials or payloads. Restore confirmed manual owners, reconcile managed
  owners from current configuration and reject traffic outside desired filters.
- Receive callbacks only validate, store and enqueue; never perform network or
  agent work there. Paho uses serial routing so a broker flood cannot spawn an
  unbounded goroutine per message before the application queues.
- Retained strings own their bounded allocation. The observation buffer enforces
  message count, age and `max_bytes` (default 16 MiB) across topics and payloads;
  clear removed backing-slice entries. UTF-8 truncation is linear. Invalid UTF-8
  or NUL payloads retain metadata (`payload_binary`, original `payload_bytes`)
  only and never trigger agent work. Buffer retrieval uses the shared MQTT
  filter matcher, including the reserved `$` namespace rule.
- Generic and Frigate relays require explicit `mqtt.relay_topics` and
  `mqtt.relay_grants`; empty lists pause them without changing broker ownership.
  Relay filters must start with a literal level. Grants reuse
  `config.ActionPolicyRule` with concrete tool, operation and target; reject
  global wildcards and unsupported tools. MQTT missions retain their own filters
  and trusted mission instructions instead of inheriting generic relay grants.
- One controller admits at most ten relay/mission jobs per rolling minute,
  counting mission fan-out separately. Admission precedes queues and survives
  generation replacement. Relay topics debounce for two seconds at reception.
  Keep one relay worker (100 waiting messages) and one mission worker (256 jobs).
  Drop new work on overload; expire waiting jobs after 60 seconds and recheck
  registration/generation at execution. Expose drop causes and rate-limit logs.
- Mission payloads are independent from observation-buffer truncation: match,
  scan and deliver the same complete text through 64 KiB; reject larger or
  binary input. Gate and scan before the trigger-interval claim. Refused,
  quarantined, stale and queue-dropped jobs do not consume that interval.
- MQTT and Frigate use their own autonomous sessions without global chat history
  or automatic memory, personality and planner effects. Restrict native tools
  to configured grants and pass an explicit empty Agent Skills scope. Each
  relay runs for at most 120 seconds and four tool calls, respecting a lower
  configured tool limit. Charge category `mqtt` so exhausted partial budgets
  stop these runs before a model request.
- The relay sink scans the topic and `mqttRelayText` (16 KiB payload plus a
  fixed truncation marker) locally regardless of the LLM Guardian toggle.
  `ThreatHigh` or an incomplete scan starts no model run. Count it and record a
  fixed `prompt_security` operational issue keyed per source, with a digest and
  no raw topic/payload. MQTT mission admission likewise scans its whole bounded
  text and drops High/incomplete content without claiming an interval. Verify
  `TestMQTTRelayHandlerQuarantinesInjectedBrokerMessages` and
  `TestMissionMQTTAdapterDropsInjectedBrokerMessages`.
- `config.MQTTAgentRunsAuthorized` is the shared queue/server gate. Broker login
  requires username plus the resolved password, or a client certificate/key pair
  over TLS. Anonymous permission and untrusted-transport permission are independent;
  neither bypasses the other. Missing/null values stay false in Load and
  config-merger, including upgrades. Never materialize a transport grandfather.
  A client login does not identify publishers; broker ACLs must restrict them.
- Every MQTT/Frigate tool dispatch rechecks exact grants and its server-owned
  ingress topic and source enablement against captured and live authorization
  snapshots, even with the general action policy disabled. Preserve stricter
  run scopes, read-only gates, HA service rules and registered-secret egress
  refusal. Supported actions are MQTT read/publish/subscribe/unsubscribe,
  HA entity reads/services and owner
  notifications. Bind MQTT targets to exact topics/filters, HA writes to every
  explicit entity and service, and notifications to one configured channel.
  Reject broad HA selectors, unresolved targets, code, MCP and delegation.
  Revocation never widens an existing run; connection/credential replacement
  must not keep using a captured HA/notification destination.
  Disabling the originating generic/Frigate relay or its event/review kind, or
  changing its prefix blocks further dispatches; another relay cannot substitute.
- Frigate prefixes remain literal MQTT topics. Invalid prefixes create neither
  subscriptions nor runs; the loader accepts the file and security hints explain
  the refusal. Status keeps connection/subscription diagnostics and adds pause
  reasons, actual buffer bytes, queue lengths and distinct drop counters.
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
- Subscription concurrency: `go test -race ./internal/mqtt` and
  `go test -race ./internal/server -run MQTT`. Keep the deterministic
  `TestMQTTReconciliationKeepsPublishedMapsSeparateFromBrokerResults` regression.

## Child DOX Index

None.
