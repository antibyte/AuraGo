# Native SIP telephony

AuraGo includes a native SIP user agent for one account and one concurrent
call. It runs inside the Go process and does not require Asterisk, FreeSWITCH,
PJSIP, ffmpeg, CGO, or a sidecar. The supported telephone codecs are G.711
PCMA (preferred) and PCMU.

The feature is disabled and read-only by default. Configure it under `sip` in
`config.yaml` or in **Configuration → SIP Phone**. Store the digest password
through the UI; it is written only to the encrypted Vault key
`sip_endpoint_password`.

SIP account, network, trust lists, destination policy, and the authenticated
browser phone remain under **SIP Phone**. Agent-led call behavior is configured
separately under **Agent & AI → Telephone agent**. That profile applies to
agent-led incoming and outgoing calls, never to manual browser conversations.
It stays visible while SIP is disabled and reports the concrete blockers.

Changing the guided account setup or deleting the SIP account preserves the
Telephone agent profile and its auto-answer delay. Deletion disables SIP and
sets incoming routing to `reject`. Guided setup also selects `reject` when
incoming calls are disabled. If setup enables incoming calls for an existing
account that already uses agent routing, that route is retained. A new account
starts with manual answering.

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

In every Docker mode, set both `advertised_signaling_host` and
`media.advertised_host` to an address that the PBX can route to. AuraGo does
not guess these values from a container bridge address: registration and the
connection diagnostic remain available, while answering and dialing fail with
`docker_advertised_host_required` until both values are explicit. For a
FRITZ!Box, the advertised values normally point to the Docker host on the same
LAN as the box. TLS covers SIP signaling only, requires local certificate/key
files, and does not add SRTP media encryption or WS/WSS support.

## Policy and privacy

- Incoming calls require both a trusted source IP/CIDR and a matching caller
  allowlist entry. Caller entries match normalized callers and support `*` and
  `?` patterns. A spoofable `From` header alone is never trusted.
- The guided **All callers** choice stores `*` in `inbound.allowed_callers`.
  It accepts any caller from the trusted registrar IPs resolved during setup,
  subject to `inbound.denied_callers`. It does not bypass the source-IP/CIDR check.
- Outgoing calls require a canonical `sip:` URI, an exact allowed domain, and
  either an exact user or an allowed E.164 prefix. Empty lists deny all.
- Destination domains never include a port. FRITZ!Box internal destinations
  such as `**610` are exact users rather than wildcard patterns.
- Incoming `00...` numbers are canonicalized to `+...`. National `0...`
  numbers are expanded only when the selected preset has one unambiguous
  country or `inbound.number_region` explicitly names it. The same conversion
  applies to numeric allow/deny entries, and deny rules retain precedence.
- `readonly: true` permits registration, status, history, and explicit
  connection tests, but prevents answering or originating calls.
- Audio, RTP packets, complete SIP headers, and authentication material are
  never logged or stored. Call history contains only IDs,
  direction, normalized peer, timestamps, state, end reason, backend,
  `media_mode` (agent or browser), and an optional chat-session link.
- Transcripts are transient by default. These calls also suppress derived
  memory, personality, activity, journal, and reuse artifacts before the chat
  session is purged. Enabling `persist_transcripts` retains only the existing
  chat session; it does not create a call recording.

## Voice backends

`classic` uses adaptive VAD, its selected ASR provider/mode, its selected agent
LLM provider, and its selected central TTS provider. Speech during playback cancels the active agent turn
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

Both backends use the same structured telephone profile: greeting, purpose,
speaking style, additional prohibitions, behavior for unsupported requests,
technical failure message, farewell, language, tool scope, transcript
retention, maximum duration, and inactivity limit. AuraGo's identity,
prompt-injection protection, permission checks, and security rules are
immutable; telephone rules can only add restrictions. Greetings use the
selected pipeline: classic uses the selected TTS, while Gemini Live receives a
server-side text turn after setup and speaks with its configured Live voice.

Before answering an agent-routed incoming call or sending an agent-led
outgoing INVITE, AuraGo validates the route, permissions, provider references,
Vault-backed readiness, and tool scope. A blocker fails closed. Runtime errors
use the configured technical message when the failing pipeline can still speak
and then end the call. AuraGo never switches voice pipelines or providers
silently.

Call media ends after `media.rtp_idle_timeout_seconds` without inbound RTP;
manual incoming calls stop ringing after `inbound.ring_timeout_seconds`.
`voice.turn_timeout_seconds` bounds each classic processing turn and private
Gemini action. Spoken responses and Gemini tool results are capped by
`voice.max_response_chars` before classic TTS chunking. DTMF uses RTP telephone
events; SIP INFO DTMF is not implemented.

Saving Telephone agent atomically updates future calls without restarting SIP
registration or browser media. An active call retains its complete provider,
tool, behavior, duration, and transcript-retention snapshot. Legacy
configurations with empty telephone provider fields inherit the existing global
LLM, Whisper, and TTS selections until the Telephone agent page is first saved;
that save records the available effective IDs. Inactive classic fields can stay
empty when Gemini Live is selected and no global choice exists. The inherited
indicators describe the fields still empty in the saved profile.

## Telephone agent limits

The fields below are relative to `sip` in `config.yaml`.

| Field | Default | Accepted range | Purpose |
| --- | --- | --- | --- |
| `inbound.auto_answer_delay_ms` | 1000 | 0–60000 ms | Delay before agent answering; 0 answers immediately |
| `voice.max_call_duration_seconds` | 3600 | 30–86400 s | Maximum agent call duration |
| `voice.idle_timeout_seconds` | 120 | 15–3600 s | Agent conversation inactivity limit |
| `voice.turn_timeout_seconds` | 60 | 15–300 s | Classic turn or Gemini action budget |
| `voice.max_response_chars` | 1200 | 200–5000 characters | Spoken response and tool-result cap |
| `voice.max_outbound_calls_per_day` | 10 | 1–1000 | Admitted agent-led outgoing attempts per local calendar day |

The daily limit counts an attempt when call history admits it, before INVITE.
Failures and cancellations after admission still count, including cancellation
by a configuration change. Invalid targets, policy denials, and failed provider
preflights before admission do not count. Incoming calls and manual browser
calls do not consume this agent limit. The reset is midnight in the server's
local time zone. The Telephone agent API and page report usage and reset time;
`agent_daily_call_limit_reached` also returns `used`, `limit`,
`retry_after_seconds`, and `resets_at`.

## Public interfaces and future clients

Administrative APIs live under `/api/sip/` and include configuration, test,
status, call history/actions, and an SSE event stream. Telephone agent adds
`GET/PUT /api/sip/agent`, the secret-free
`GET /api/sip/agent/catalog`, and `POST /api/sip/agent/test`. The default test
is a free local reference, token, and pipeline-readiness preflight; it does not
send a paid completion or require an outbound destination. An explicitly
confirmed live test is limited to three attempts per hour per client IP,
uses no agent tools, and stores neither audio nor a test conversation. The
classic audio check resamples synthesized speech to 16 kHz before ASR. The
native `sip_phone` agent tool applies the same runtime permissions.

Use the dedicated SIP configuration/setup and Telephone agent endpoints for
updates. A top-level `sip` field in `PUT /api/config` is rejected with HTTP 400
and code `sip_use_dedicated_api` before configuration or Vault changes occur.

Status exposes sanitized registration codes. `registration_failed_401` and
`registration_failed_403` normally indicate account credentials or provider
authorization, `registration_failed_404` an unknown account/address, and
`registration_failed_408` a DNS, routing, firewall, or registrar timeout.
AuraGo retries with exponential backoff and refreshes successful registrations
at 75 percent of the registrar-negotiated expiry.

The PCM media boundary and incoming-call handler are intentionally independent
of SIP. A future Virtual Desktop phone can attach an authenticated WebRTC media
peer without exposing SIP credentials or RTP to the browser. A future answering
machine can implement the same incoming-call contract and store bounded messages
through the Media Registry without changing the SIP transport core.
