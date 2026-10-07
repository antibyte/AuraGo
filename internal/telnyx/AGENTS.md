# Telnyx

## Purpose

Optional SMS and call-control integration with authenticated webhooks.

## Ownership

Client owns API transport; WebhookHandler owns active call state and reconciliation.

## Local Contracts

- Base URL contains exactly one /v2; call/SMS paths are relative. Escape call IDs as one segment. Retry reads only; mutations with an unconfirmed result are not replayed automatically.
- Reconcile missing hangups through bounded provider status reads, at most once per minute. Remove only a matching call with is_alive=false and a valid end_time if its local activity did not change. Errors or pending/asynchronous calls remain retained.
- Apply the same fail-closed E.164 allowlist to incoming SMS/calls, outgoing calls, transfers, SMS/MMS and agent SMS feedback. Configured clients (`NewConfiguredClient`) enforce read-only before mutation; a bare `NewClient` has no number policy, so its POSTs and destination checks fail closed and it serves only read-only account queries (the balance check and `telnyx_manage`'s number, message and call history listings). Notification SMS use the configured client. Reject disallowed or busy incoming calls through the provider; this defensive rejection remains available in read-only mode. Answer allowed calls only once and retain uncertain reservations for reconciliation.
- `webhook_path` is a local handler route. Outgoing calls use the callback configured on the Telnyx application; never send a relative override. `call_timeout` limits answered-call duration through `time_limit_secs`; the tool's optional `timeout_secs` sets ringing (30 seconds by default, 5..600), separately from DTMF gathering.

## Work Guidance

Tests use synthetic RoundTrippers and never send real calls/messages. Provider status contract: https://github.com/team-telnyx/telnyx-go/blob/main/call.go.

## Verification

`go test ./internal/telnyx`, especially production path/retry and reconciliation tests.

## Child DOX Index

None.
