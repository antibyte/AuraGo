# Remote Control Hardening Plan

## Scope

Validate the findings from [`reports/remote_control_analysis.md`](../reports/remote_control_analysis.md) against the current codebase and implement only the gaps that are still real.

Primary code areas:

- [`internal/remote/hub.go`](../internal/remote/hub.go)
- [`internal/remote/protocol.go`](../internal/remote/protocol.go)
- [`cmd/remote/main.go`](../cmd/remote/main.go)
- [`cmd/remote/executor.go`](../cmd/remote/executor.go)
- [`internal/config/config.go`](../internal/config/config.go)
- [`internal/config/config_types.go`](../internal/config/config_types.go)

## Report validation result

The report is only partially current.

Confirmed open issues:

1. `AuthResponse` is still sent without message signing in [`sendAuthResponse()`](../internal/remote/hub.go).
2. Replay protection still relies on timestamp validation only; nonce reuse is not tracked server-side.
3. `OpFileWrite` still does not enforce `remote_control.max_file_size_mb` in [`cmd/remote/executor.go`](../cmd/remote/executor.go).
4. `auto_approve` still allows direct enrollment without an additional trust gate in [`completeEnrollment()`](../internal/remote/hub.go).
5. Remote-control config defaults and runtime semantics need a focused pass, especially for `max_file_size_mb`, `audit_log`, and `readonly`.

Already fixed or outdated report items:

1. `MaxTimestampDrift` is already `15 * time.Minute` in [`internal/remote/protocol.go`](../internal/remote/protocol.go).
2. Binary trailer loading in [`cmd/remote/main.go`](../cmd/remote/main.go) already avoids reading the full executable for large binaries.
3. The shell execution error-handling note in the report is outdated; [`shellExec()`](../cmd/remote/executor.go) already returns `cmdErr`.

## Remediation goals

- Close real message-authentication and replay gaps without breaking existing remote clients.
- Enforce configured file-write safety limits in the remote executor.
- Clarify and harden remote-control defaults so the effective behavior matches the documented security posture.
- Keep the work regression-safe with focused protocol, executor, and config tests.

## Phase 1: Protocol and enrollment hardening

### 1. Sign server auth responses

Relevant code:

- [`internal/remote/hub.go`](../internal/remote/hub.go)
- [`internal/remote/protocol.go`](../internal/remote/protocol.go)

Current issue:

`MsgAuthResponse` is emitted unsigned. That leaves the initial server-to-client acceptance message outside the normal integrity guarantees.

Implementation direction:

- Introduce a dedicated signing strategy for `AuthResponse`.
- Use the best available shared secret for the response path.
- If enrollment/auth bootstrap prevents normal signing on the first hop, add an explicit bootstrap signing rule instead of leaving the message unsigned.
- Keep client compatibility in mind and update the remote client verifier if needed.

Tests to add:

- signed `AuthResponse` round-trip test
- rejection test for tampered `AuthResponse`
- compatibility test for the initial enrollment handshake

### 2. Add nonce replay protection

Relevant code:

- [`internal/remote/hub.go`](../internal/remote/hub.go)
- [`internal/remote/protocol.go`](../internal/remote/protocol.go)

Current issue:

Timestamp validation exists, but valid signed messages can still be replayed within the accepted time window.

Implementation direction:

- Add a bounded nonce cache on the hub side keyed by device plus nonce.
- Expire entries automatically using the same rough horizon as timestamp drift.
- Reject reused nonces before dispatching the message.
- Prefer a TTL-based structure over an unbounded `sync.Map`.

Tests to add:

- first message with nonce is accepted
- second message with same nonce is rejected
- same nonce after expiry is accepted again if the timestamp is valid

### 3. Revisit `auto_approve`

Relevant code:

- [`internal/remote/hub.go`](../internal/remote/hub.go)
- [`internal/config/config_types.go`](../internal/config/config_types.go)

Current issue:

`auto_approve` still acts as a broad trust bypass. In home-lab deployments that may be intentional, but the current behavior is too coarse for an internet-facing or mixed-trust setup.

Implementation direction:

- Keep the feature, but tighten the trust model.
- Add one or more additional gates, such as LAN-only approval, explicit allowlist matching, or a required bootstrap token even when auto-approve is enabled.
- If behavior is intentionally unchanged for compatibility, at least surface a stronger warning and document the risk clearly.

Tests to add:

- auto-approve allowed under the chosen trust condition
- auto-approve denied when the extra condition is not met

## Phase 2: Executor and config correctness

### 4. Enforce `max_file_size_mb` for file writes

Relevant code:

- [`cmd/remote/executor.go`](../cmd/remote/executor.go)
- [`internal/config/config_types.go`](../internal/config/config_types.go)

Current issue:

`OpFileWrite` decodes and writes arbitrary payload size without checking the configured file-size cap.

Implementation direction:

- Apply the limit before disk write.
- Validate both encoded input size and decoded payload size where useful.
- Return a clear user-facing error when the cap is exceeded.
- Reuse the same effective limit semantics everywhere remote file writes are accepted.

Tests to add:

- file write below limit succeeds
- file write above limit fails
- zero or unset config follows documented default behavior

### 5. Normalize remote-control defaults

Relevant code:

- [`internal/config/config.go`](../internal/config/config.go)
- [`internal/config/config_types.go`](../internal/config/config_types.go)

Current issue:

The report mixes type comments, YAML examples, and runtime behavior. The effective defaults for remote control need to be defined in one place and verified by tests.

Implementation direction:

- Audit the actual default assignment path for `RemoteControl`.
- Set explicit defaults in config loading where they are missing.
- Make documentation, config template comments, and runtime behavior agree on:
  - `enabled`
  - `readonly`
  - `discovery_port`
  - `max_file_size_mb`
  - `auto_approve`
  - `audit_log`
  - `ssh_insecure_host_key`

Tests to add:

- config default test for the full `RemoteControl` block
- regression test for documented `readonly` default
- regression test for documented `max_file_size_mb` default

## Phase 3: Documentation and regression coverage

### 6. Update the report-derived documentation

After implementation:

- correct the outdated findings from the original report
- align config examples with runtime defaults
- document the replay-protection and auth-response signing model

### 7. Expand regression coverage

Required automated coverage:

- protocol signing tests for auth bootstrap
- nonce replay tests
- executor size-limit tests
- config default tests

Recommended follow-up coverage:

- enrollment approval policy tests
- remote client/server compatibility tests around bootstrap auth

## Execution order

1. Implement `AuthResponse` signing with compatibility-safe tests.
2. Add nonce replay protection on the hub.
3. Enforce `max_file_size_mb` in the executor.
4. Normalize and test remote-control defaults.
5. Tighten or clearly gate `auto_approve`.
6. Refresh documentation and regression coverage.

## Notes

- Do not spend time re-fixing the timestamp drift or trailer-loading items; those findings are already outdated.
- Treat `auto_approve` as a product/security decision as well as an implementation task.
- Any protocol change in the auth bootstrap path must be tested against the embedded remote client, not just server-side helpers.
