# Incoming webhooks

## Purpose

Persistent webhook configuration, bounded ingress, authenticated delivery and logs.

## Ownership

- This package owns format validation, receipts, payload isolation and dispatch.
- Server handlers own admin/Vault publication; `ui/cfg/webhooks.js` owns its editor.
- `documentation/webhook-replay-protection.md` owns the opt-in sender protocol.

## Local Contracts

- Replay protection defaults off. Preserve omitted update fields. Enabled hooks
  require HMAC-SHA256 and a Vault secret; legacy hooks retain raw-body signing.
- Authenticate the timestamp, nonce and raw body before claiming a nonce.
  Reject clock skew over five minutes and nonces outside ASCII `[A-Za-z0-9_-]`
  with 16–128 characters. Claim durably before paid scans or delivery.
- `<webhooks-file>.replay.json` contains only hashed webhook/nonce pairs and
  ten-minute expiry times. Its advisory lock serializes all manager instances.
  Atomic, synced publication precedes acceptance. Keep at most 10,000 active
  claims; never evict an unexpired claim. Invalid/unavailable/full stores fail
  protected admission with 503, without changing legacy ingress.
- Valid duplicates return 200/`duplicate`, even when delivery slots are full;
  they run no scan, agent, SSE, mission callback or fire counter. A new claim
  reserves a delivery slot before persistence; a failed publication releases it.
  A persisted claim remains after uncertain delivery or quarantine.
- Payloads are untrusted, callback bytes stay unchanged, configuration reads
  are detached, and Vault secrets never appear in persistent webhook JSON.
  Cross-component publication and lifetime contracts: `../server/AGENTS.md`.

## Work Guidance

- Preserve existing bearer and raw-body signature compatibility. Never auto-enable
  replay protection for sender presets that cannot produce this protocol.
- Use temporary synthetic fixtures; never replay production events in tests.

## Verification

- `go test ./internal/webhooks ./internal/server` webhook and publication tests.
- Run replay concurrency tests with the Linux race detector. UI checks cover the
  editor, all sixteen locales and boolean save/reopen behavior.

## Child DOX Index

None.
