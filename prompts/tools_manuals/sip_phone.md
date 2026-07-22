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
- `dial`: Start one outgoing call to a canonical `sip:user@domain` target.
- `answer` / `reject`: Decide a pending manually routed incoming call.
- `hangup`: End the active call.
- `send_dtmf`: Send exactly one digit from `0-9`, `*`, `#`, or `A-D`.

Mutating operations disappear in read-only mode and remain subject to their
individual permission flags. Incoming callers must match both a trusted proxy
source and the caller allowlist. Outgoing targets must match an exact domain
and either an exact user or an allowed E.164 prefix. Empty allowlists deny all.

```json
{"action":"sip_phone","operation":"dial","target":"sip:+49123456789@pbx.example"}
```

```json
{"action":"sip_phone","operation":"send_dtmf","call_id":"CALL_ID","digits":"1"}
```

AuraGo exposes no SIP password, full headers, RTP packets, audio, or raw
transcripts through this tool. Only one call may be active at a time.
