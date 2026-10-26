---
id: "tool_sip_phone"
tags: ["tool", "sip", "phone", "voice", "calls"]
priority: 50
---
# Native SIP phone

Inspect and operate AuraGo's native single-account SIP endpoint. The tool is
available only when SIP is enabled. Its operation enum is reduced at runtime
to the actions permitted by the saved configuration.

## Operations

- `status`: Show registration and active-call state without credentials.
- `list_calls`: Return privacy-safe call history. Set `limit` from 1 to 200.
- `dial`: Asynchronously accept one outgoing call request to a canonical `sip:user@domain` target. Verify the result with `status` or `list_calls` and inspect `end_reason`.
- `answer` / `reject`: Decide a pending manually routed incoming call.
- `hangup`: End the active call.
- `send_dtmf`: Send exactly one RTP-DTMF digit from `0-9`, `*`, `#`, or uppercase `A-D`; SIP INFO is not used.

Mutating operations disappear in read-only mode and remain subject to their
individual permission flags. Incoming callers must match both a trusted proxy
source and the caller allowlist. Outgoing targets must match an exact domain
and either an exact user or an allowed E.164 prefix. Empty allowlists deny all.

An accepted `dial` result is asynchronous, not confirmation that the peer
answered. Check `status` or `list_calls` before deciding whether another attempt
is needed. Admitted agent-led outgoing attempts count toward the daily limit,
including later failures or cancellations. Invalid targets, policy denials, and
failed provider preflights before admission do not count. Incoming calls and
manual browser calls do not consume that limit.

Errors have `status: "error"`, a public `message`, and a `code` such as
`disabled`, `read_only`, `busy`, `permission_denied`, `invalid_target`,
`docker_advertised_host_required`, `call_not_found`, or `operation_failed`.
`agent_daily_call_limit_reached` also includes `used`, `limit`,
`retry_after_seconds`, and `resets_at`. Wait until the local daily reset rather
than retrying immediately.

```json
{"action":"sip_phone","operation":"dial","target":"sip:+49123456789@pbx.example"}
```

```json
{"action":"sip_phone","operation":"send_dtmf","call_id":"CALL_ID","digits":"1"}
```

AuraGo exposes no SIP password, full headers, RTP packets, audio, or raw
transcripts through this tool. Only one call may be active at a time.
