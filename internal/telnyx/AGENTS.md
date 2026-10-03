# Telnyx

## Purpose

Optional SMS and call-control integration with authenticated webhooks.

## Ownership

Client owns API transport; WebhookHandler owns active call state and reconciliation.

## Local Contracts

- Base URL contains exactly one /v2; call/SMS paths are relative. Escape call IDs as one segment. Retry reads only; mutations with an unconfirmed result are not replayed automatically.
- Reconcile missing hangups through bounded provider status reads, at most once per minute. Remove only a matching call with is_alive=false and a valid end_time if its local activity did not change. Errors or pending/asynchronous calls remain retained.

## Work Guidance

Tests use synthetic RoundTrippers and never send real calls/messages. Provider status contract: https://github.com/team-telnyx/telnyx-go/blob/main/call.go.

## Verification

`go test ./internal/telnyx`, especially production path/retry and reconciliation tests.

## Child DOX Index

None.
