# SIP telephony

## Purpose

Native telephone registration, calls, media, and agent policy.

## Ownership

`internal/sipphone` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Native SIP Telephony Contract

- AuraGo owns one in-process Diago SIP endpoint, one Vault-backed account, and at most one active call. Keep Diago pinned to v0.31.0, sipgo pinned to v1.4.3, G.711-only media, and CGO-free builds; do not add Asterisk, FreeSWITCH, PJSIP, ffmpeg, or a SIP sidecar.
- Registration and explicit connection tests remain available in read-only mode. Answering, dialing, DTMF, and agent hangup require both `readonly: false` and their granular permissions. Empty caller or destination allowlists deny all.
- Trust incoming calls only when both the network peer matches a configured CIDR and the normalized caller matches the allowlist. Outgoing calls require canonical `sip:` URIs, an exact allowed domain, and an exact user or allowed E.164 prefix.
- SIP config/setup persistence must reserve reconfiguration before any Vault or YAML mutation and return HTTP 409 while a call is active or being prepared. Legacy wildcard outbound allow entries remain loadable but authorize nothing, surface `outbound_policy_migration_required`, and must be replaced on the next save.
- Keep the SIP password only under Vault key `sip_endpoint_password`; block every `sip_` secret from Python, skills, and agent export. Never log or store full SIP headers, RTP/audio, authentication data, or raw transcripts.
- Both classic ASR/agent/TTS and server-side Gemini Live use `internal/voice` PCM contracts and the shared `VoiceActionRunner`. SIP turns always carry an explicit `AllowedTools` list whose empty form allows no native tools, including through `invoke_tool`.
- `sip.voice` is the single Telephone agent profile for agent-led inbound and outbound calls. It selects explicit agent-LLM, classic ASR/mode/TTS, or Gemini Live references plus additive behavior, privacy, and duration rules. Legacy empty provider fields inherit the global LLM/Whisper/TTS choice only until the first Telephone agent save materializes the effective IDs.
- Preflight agent routes before answer or outbound INVITE and fail closed on permission, provider, Vault, service, or tool-scope blockers. Never switch telephone pipelines or providers silently. Saving `/api/sip/agent` updates only future-call settings and must not restart SIP registration or browser media; every active call keeps its complete provider, tool, behavior, limit, and transcript-retention snapshot.
- Keep Browser Realtime Speech on its established surface-specific streaming handlers; SIP must not replace browser SSE routing or inherit the Virtual Desktop provider selection. Transient SIP turns suppress derived memory, personality, activity, journal, and reuse-first side effects before their chat session is purged.
- Terminate established local/outbound call failures with one BYE and close every dialog exactly once; cancellation while an outbound INVITE is pending must flow through its context-driven CANCEL. Keep provider audio and VAD buffers bounded, normalize external sample rates before the fixed 8/16/24 kHz media bus, and never write SIP ASR audio to disk.
- The PCM `MediaPeer`, incoming-call handler, history schema, REST actions, and SSE events are compatibility anchors for the future authenticated WebRTC desktop phone and bounded Media-Registry answering machine; neither future feature may expose SIP credentials or raw RTP to the browser.

## Verification

- Run `go test ./internal/sipphone` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
