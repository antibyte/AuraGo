# Optional webhook replay protection

Enable **Replay protection** in an incoming webhook's Config editor only when
the sender supports this AuraGo protocol. It is off by default; existing hooks
continue to verify their original raw-body signature. A preset alone does not
enable it. Set HMAC-SHA256, a signature header (for example `X-Signature`) and
the shared secret through Config; the secret stays in the encrypted Vault.

For each new event, the sender supplies:

- `X-AuraGo-Webhook-Timestamp`: canonical Unix seconds, as decimal text.
- `X-AuraGo-Webhook-Nonce`: a unique 16–128 character value using ASCII letters,
  digits, `_` and `-`. Use a cryptographically random value.
- The configured signature header: `sha256=` followed by the lowercase hex
  HMAC-SHA256 of `timestamp + "." + nonce + "." + rawBody`, using the Vault
  secret as the key. Sign the exact bytes sent, before JSON parsing.

The receiver allows five minutes of clock skew in either direction. Missing,
malformed or stale headers and incorrect signatures return 403. Invalid
signature configuration or an unavailable Vault secret returns 503.

Nonce claims are scoped to the webhook ID, survive restarts and expire after
ten minutes. The private sibling `<webhooks-file>.replay.json` and its lock file
hold digests and expiry times, never payloads or secrets. All protected hooks
share a 10,000-active-claim cap; unexpired entries are never displaced to admit
new traffic. Storage errors, corrupt state and capacity exhaustion return 503.
Expired entries are pruned during subsequent claims.

The first valid event returns `200 {"status":"accepted"}` after admission.
A valid repetition returns `200 {"status":"duplicate"}` without repeating
Guardian/model checks, agent delivery, SSE, mission callbacks or fire counts.
An accepted claim remains even if later checks quarantine the event or delivery
has an uncertain outcome. A duplicate receipt confirms previous admission,
not successful business processing. Do not change the nonce to automatically
retry an uncertain event; inspect the event log first.

API clients set `format.replay_protection` to a boolean. Omitting this field on
update retains its saved value; explicit `false` restores raw-body signing.
Changing the signature algorithm or removing its secret while protection is
enabled is rejected. Regular bearer-token rules still apply when one is sent.

Keep the replay receipt file with installation state during moves/restores.
Restoring an older receipt snapshot can forget newer events: keep ingress
paused for at least ten minutes before reopening it, covering both directions
of the five-minute clock-skew window.
