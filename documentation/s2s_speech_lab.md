# Speech Lab integration

AuraGo can use the local s2s Speech Lab as an optional ASR and TTS provider. AuraGo continues to own the conversation, LLM, tools, Guardian, and memory. Browser Realtime Speech and cloud live providers are separate and are not changed by this integration.

## Configuration

All surfaces default to disabled and are selected independently:

```yaml
speech_lab:
  enabled: true
  base_url: http://s2s-vulkan:8765
  language: de
  chat_llm_provider_id: ""  # optional fast AuraGo provider for Speech-Lab-transcribed webchat turns
  timeout_seconds: 60
  sip_enabled: false
  chat_input_enabled: false
  chat_output_enabled: false
```

`use_for_sip`, `use_for_chat_voice`, and the former free-form `voice` value remain load-compatible for older configuration files. The voice value is ignored and removed on the next UI save because voice is owned by the active Speech Lab stack. API paths are fixed by contract and are not configurable.

`AURAGO_SPEECH_LAB_BASE_URL` overrides `base_url` at runtime without rewriting YAML. The configuration UI marks the URL as environment-managed. AuraGo automatically opens the Browser Lab on `http://<current AuraGo host>:8766`; users do not have to discover or enter that address. `advanced_ui_url` remains an expert-only YAML override for non-standard reverse proxies or port mappings.

Only credential-free HTTP(S) URLs without query or fragment are accepted. AuraGo resolves and connects only to loopback or private network addresses. It does not provide a general Speech Lab proxy.

## Channel selection

- Telephone: first enable `sip_enabled`, then choose `speech_lab` independently for classic ASR and/or TTS in the Telephone agent. Hybrid combinations remain valid.
- Chat input: `chat_input_enabled` selects an AudioWorklet recorder that creates mono PCM16/16 kHz RIFF/WAV in the browser. It does not run Web Speech in parallel and does not fall back automatically.
- Speech-Lab-transcribed webchat: `chat_llm_provider_id` optionally selects a fast AuraGo provider for exactly that next chat turn. Typed chat, browser speech recognition, internal follow-ups, missions, and SIP retain their established providers. An unavailable selection fails with `speech_lab_llm_unavailable`; it never falls back to the main provider.
- Chat output: `chat_output_enabled` selects Speech Lab TTS for chat responses. If readiness or synthesis fails, the text response remains and the stream reports a structured audio error.

Every telephone call snapshots its selected providers, language, the active Speech Lab voice, Speech Lab backend IDs, behavior, and tool scope before answer or INVITE. A required component that is not ready rejects inbound calls with SIP 480 and outgoing requests with HTTP 503 (`speech_lab_not_ready`). No cloud provider is selected automatically.

## AuraGo API

| Method | Path | Access | Purpose |
|---|---|---|---|
| `GET` | `/api/speech-lab/status` | authenticated | Sanitized channel and readiness status |
| `GET` | `/api/speech-lab/capability` | administrator | Fixed capability endpoint |
| `GET` | `/api/speech-lab/catalog` | administrator | Available backend catalog |
| `GET` | `/api/speech-lab/suggestions` | administrator | Heuristic recommendations |
| `PUT` | `/api/speech-lab/stack` | administrator | Confirmed `{asr_id, tts_id, voice}` change |

AuraGo never sends `llm_id`. It validates backend IDs and voices against the catalog, checks the stack response `ok` field, and then checks `/ready`. A stack change is rejected while a Speech Lab operation or SIP call is active.

The native configuration page under **Media → Speech Lab** displays connectivity, readiness, the active `ASR + TTS + voice` combination, capability, recommendations, and stable available catalog entries. Voice is selected only from the chosen TTS backend's `voices` or `default_voice` catalog values. The Browser Lab and AuraGo stack editor both update the same s2s runtime stack; the next refresh or preflight observes either change. Experimental entries require an explicit visible filter. Downloads, Hugging Face tokens, benchmarks, and expert model management stay in the Speech Lab UI. AuraGo derives its link from the current browser host and the standard Lab port `8766`.

When Speech Lab is enabled, the chat integrations drawer shows it as running, starting, or offline and opens the automatically derived Browser Lab URL. An expert `advanced_ui_url` override takes precedence when configured.

## s2s contract

AuraGo uses these fixed paths:

| Method | Path | Contract |
|---|---|---|
| `GET` | `/ready` | HTTP 200 when ready, otherwise 503; includes component status, backend IDs, and active voice from one runtime snapshot |
| `GET` | `/api/v1/capability` | Hardware capability profile |
| `GET` | `/api/v1/catalog` | Backend metadata, availability, stability, languages, and voices |
| `GET` | `/api/v1/suggestions` | Heuristic stable-first recommendations |
| `PUT` | `/api/v1/stack` | HTTP 200 only for `ok:true`; omitted `llm_id` preserves the s2s LLM |
| `POST` | `/v1/audio/transcriptions` | Raw or multipart PCM-WAV, maximum 8 MiB; response includes `asr_id` |
| `POST` | `/v1/audio/speech` | JSON speech request; WAV response includes `X-S2S-TTS-ID` |

AuraGo verifies the returned ASR/TTS backend IDs against the operation snapshot and uses the `/ready` voice for chat TTS and newly started SIP calls. Unexpected external stack drift fails the local speech path instead of silently changing models.

## Deployment

Containerized AuraGo should use [the Docker network overlay](../deploy/docker/docker-compose.s2s.yml). Port 8765 stays inside the shared Docker network and is not published to the LAN.

The s2s Browser Lab uses host port `8766` in the AuraGo deployment (`WEB_PORT=8766`). AuraGo derives this browser-facing address from the hostname or IP through which the user opened AuraGo.

For a native AuraGo process, start s2s with its optional AuraGo host overlay, which publishes only `127.0.0.1:8765`. Then set `base_url: http://127.0.0.1:8765`.

For production chat or telephony, configure:

```text
S2S_LAB_IDLE_UNLOAD_SECS=0
S2S_ALLOW_EXPERIMENTAL=false
```

Start with stable ASR and TTS backends. Automatic downloads, automatic stack switching, LAN publication, audio persistence, and silent provider fallback are deliberately outside the integration contract.
