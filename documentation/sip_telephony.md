# Native SIP telephony

AuraGo includes a native SIP user agent for one account and one concurrent
call. It runs inside the Go process and does not require Asterisk, FreeSWITCH,
PJSIP, ffmpeg, CGO, or a sidecar. The supported telephone codecs are G.711
PCMA (preferred) and PCMU.

The feature is disabled and read-only by default. Configure it under `sip` in
`config.yaml` or in **Configuration → SIP Phone**. Store the digest password
through the UI; it is written only to the encrypted Vault key
`sip_endpoint_password`.

## Network setup

For a LAN PBX such as a FritzBox, bind signaling to the host's private address
or `0.0.0.0` and allow the selected UDP, TCP, or TLS signaling port. Allow the
configured RTP range (default UDP `30000-30099`) in both directions. Public
providers usually also require explicit advertised signaling/media addresses
and manual router port forwarding. V1 does not include STUN, ICE, WS/WSS, or
automatic router configuration.

The normal native installation is preferred. Docker users must either publish
the signaling port and the complete UDP RTP range or use Linux host networking:

```yaml
ports:
  - "5060:5060/udp"
  - "30000-30099:30000-30099/udp"
```

Docker Desktop NAT is unsuitable when the PBX cannot reach the advertised RTP
address. Linux `network_mode: host` avoids that NAT boundary, but removes the
network isolation provided by Compose and must be chosen explicitly.

## Policy and privacy

- Incoming calls require both a trusted source IP/CIDR and an exact caller
  allowlist match. A spoofable `From` header alone is never trusted.
- Outgoing calls require a canonical `sip:` URI, an exact allowed domain, and
  either an exact user or an allowed E.164 prefix. Empty lists deny all.
- `readonly: true` permits registration, status, history, and explicit
  connection tests, but prevents answering or originating calls.
- Audio, RTP packets, complete SIP headers, and authentication material are
  never logged or stored. Call history contains only IDs,
  direction, normalized peer, timestamps, state, end reason, backend, and an
  optional chat-session link.
- Transcripts are transient by default. These calls also suppress derived
  memory, personality, activity, journal, and reuse artifacts before the chat
  session is purged. Enabling `persist_transcripts` retains only the existing
  chat session; it does not create a call recording.

## Voice backends

`classic` uses adaptive VAD, the configured ASR, AuraGo's normal agent path,
and the configured TTS. Speech during playback cancels the active agent turn
and flushes queued audio. The allowed agent tools are an explicit list; an
empty list means no native tools. ASR audio stays in memory. Decoded provider
audio is normalized to the fixed telephone media rates, and continuous speech
is bounded to 120 seconds with oldest audio discarded on overflow.

`gemini_live` opens a server-side Gemini Live WebSocket using an enabled
Realtime Speech profile and its Vault key. The browser never receives the
provider credential. It supports duplex PCM, manual activity, transcription,
interruptions, session resumption, and only the private functions
`aurago_execute`, `aurago_cancel_current_task`, and `aurago_end_call`. Session
reconnection requires a provider-confirmed resumption handle; a contextless
new session is never reported as resumed.

## Public interfaces and future clients

Administrative APIs live under `/api/sip/` and include configuration, test,
status, call history/actions, and an SSE event stream. The native `sip_phone`
agent tool applies the same runtime permissions.

The PCM media boundary and incoming-call handler are intentionally independent
of SIP. A future Virtual Desktop phone can attach an authenticated WebRTC media
peer without exposing SIP credentials or RTP to the browser. A future answering
machine can implement the same incoming-call contract and store bounded messages
through the Media Registry without changing the SIP transport core.
