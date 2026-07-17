# Realtime Speech

AuraGo's Realtime Speech interface provides one continuous voice experience across the web chat and virtual desktop. It keeps live-provider configuration independent from the regular LLM provider list and routes every request that needs AuraGo state or capabilities through the existing agent loop.

## User experience and execution contract

The live provider may answer casual conversation and general knowledge directly. Requests involving AuraGo context, current data, memories, files, devices, integrations, or state changes must call one of two private functions:

- `aurago_execute({request})`
- `aurago_cancel_current_task()`

The live provider never receives AuraGo's native tool catalog. `aurago_execute` binds the request to the active chat session and runs the normal AuraGo agent loop, including Guardian checks, capability controls, confirmation questions, tool events, and cancellation. The voice contract requires first-person AuraGo wording and prohibits references to forwarding work to another model or internal agent. Progress may be acknowledged, but success may only be stated after the action has completed.

Existing AuraGo TTS is suppressed only through a request-local context value for these action turns. Global speaker settings and `RunConfig` remain unchanged, which prevents duplicate speech without affecting another concurrent request.

## Provider catalog

Catalog version: `2026-07-17`.

| Provider | Offered models | Default | Browser transport | Park strategy |
|---|---|---|---|---|
| OpenAI | `gpt-realtime-2.1`, `gpt-realtime-2.1-mini`, `gpt-realtime-2`, `gpt-realtime-1.5`, `gpt-realtime`, `gpt-realtime-mini` | `gpt-realtime-2.1` | WebRTC with a server-side SDP exchange | Keep WebRTC and its data channel warm; gate the audio track |
| xAI | `grok-voice-think-fast-1.0`, `grok-voice-latest` | `grok-voice-think-fast-1.0` | WebSocket with a short-lived client secret | Close and resume with `conversation_id` |
| Gemini Developer API | `gemini-3.1-flash-live-preview`, `gemini-2.5-flash-native-audio-preview-12-2025` | `gemini-3.1-flash-live-preview` | Live API WebSocket with a constrained ephemeral token | Close and resume with the latest resumption handle |

`grok-voice-fast-1.0`, `gemini-live-2.5-flash-preview`, and `gemini-2.0-flash-live-001` may be retained when loading an older profile but cannot be selected for a new one. Translation- and transcription-only OpenAI realtime models are intentionally excluded from the voice-agent catalog.

Provider references:

- [OpenAI model catalog](https://developers.openai.com/api/docs/models/all), [Realtime WebRTC](https://developers.openai.com/api/docs/guides/realtime-webrtc), and [Realtime costs](https://developers.openai.com/api/docs/guides/realtime-costs)
- [xAI Voice Agent API](https://docs.x.ai/developers/model-capabilities/audio/voice-agent), [ephemeral tokens](https://docs.x.ai/developers/model-capabilities/audio/ephemeral-tokens), and [voice-agent models](https://docs.x.ai/developers/models/voice-agent-api)
- [Gemini models](https://ai.google.dev/gemini-api/docs/models), [ephemeral tokens](https://ai.google.dev/gemini-api/docs/live-api/ephemeral-tokens), and [session management](https://ai.google.dev/gemini-api/docs/live-api/session-management)

The API does not accept free-form models. Update `internal/realtimespeech/catalog.go` and its catalog version together when providers change their supported models or voices. xAI voices can additionally be refreshed from its live voice catalog.

## Configuration and secrets

```yaml
realtime_speech:
  enabled: false
  default_profile: ""
  park_after_seconds: 5
  profiles:
    - id: openai-live
      name: "OpenAI Live"
      provider: openai
      model: gpt-realtime-2.1
      voice: marin
      enabled: true
```

Profile IDs are immutable. API keys are never serialized into YAML. The Config UI stores each key under `realtime_speech_profile_<sanitized-profile-id>_api_key` in the encrypted Vault:

- leaving the API-key input blank retains its current value;
- selecting “remove stored key” removes it when the configuration is saved;
- deleting a profile removes its Vault entry as part of the atomic update.

AuraGo registers every hydrated key as sensitive and blocks the entire `realtime_speech_` prefix from Python secret injection.

## Local audio gate and lifecycle

The browser runs the pinned Silero VAD v6.2.1 ONNX model through ONNX Runtime Web. Raw microphone audio stays in the page. The provider receives only PCM16 frames classified as speech.

V1 timing is fixed:

- 16 kHz local analysis;
- 300 ms pre-roll;
- 96 ms speech confirmation;
- 650 ms silence to close a turn;
- 5 seconds of complete inactivity before parking by default;
- up to 3 seconds of wake audio buffered while reconnecting.

The park timer runs only when the user and provider are both silent and no AuraGo action is active. Speech immediately interrupts provider playback. It does not cancel an AuraGo action unless the user explicitly says to stop or uses the cancel control.

A `BroadcastChannel` lease and a backend lease permit only one microphone session per browser origin. Moving the session between tabs requires an AuraGo modal confirmation. Closing the Live Speech app or leaving the page releases the microphone and provider connection; minimizing the desktop app keeps the session active.

## Persistence and privacy

- Only final direct-dialog transcripts are saved.
- Partial transcripts and raw audio are never persisted.
- An AuraGo action result creates exactly one assistant turn; the provider's spoken paraphrase does not create a duplicate.
- Provider context contains at most the latest 20 visible messages and 20,000 sanitized characters. System messages, tool messages, and secrets are excluded.
- Text messages arriving during a voice session are synchronized into the provider context without prompting a second response.
- Telemetry is content-free: session state, reconnect count, parked transitions, usage metadata, and error counters.

## HTTP API

| Method and path | Purpose |
|---|---|
| `GET /api/realtime-speech/config` | Return masked profiles and `api_key_set` |
| `PUT /api/realtime-speech/config` | Atomically update configuration and Vault entries |
| `GET /api/realtime-speech/catalog` | Return the versioned model, voice, and capability catalog |
| `POST /api/realtime-speech/test` | Validate credentials, model, and voice without generating speech |
| `POST /api/realtime-speech/sessions` | Start or resume a provider session |
| `PATCH /api/realtime-speech/sessions/{id}` | Update lease-owned, privacy-safe session state and resumption metadata |
| `DELETE /api/realtime-speech/sessions/{id}` | End a session and release its lease |
| `POST /api/realtime-speech/actions` | Stream an AuraGo action as server-sent events |
| `DELETE /api/realtime-speech/actions/{request_id}` | Interrupt the bound AuraGo session |
| `POST /api/realtime-speech/turns` | Idempotently persist a final direct voice turn |

Mutation endpoints enforce same-origin requests, authenticated AuraGo routing, bounded request sizes, lease ownership, and rate limits. Standard tests use simulated provider endpoints. Live provider tests must remain opt-in and require explicit environment credentials.
