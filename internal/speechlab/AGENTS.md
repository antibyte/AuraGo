# Speech Lab

## Purpose

Active ASR/TTS snapshots and speech-driven chat routing.

## Ownership

`internal/speechlab` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Speech Lab Integration Contract
- The signed managed bundle starts Confucius4-R2T2 GGUF as its default ASR through llama.cpp `POST /v1/audio/transcriptions`, with a pinned model and audio projector. Select its published CPU, NVIDIA CUDA, or Vulkan image from the validated hardware profile before pulling and starting; leave older bundles unchanged. Keep the stable `confucius-asr` network alias and the gateway's selected ASR protocol/model consistent with the catalog and `/ready` snapshot.
- Managed updates journal each container's network aliases. Rollback reconnects missing aliases and accepts an existing Docker endpoint only after inspecting its actual attachment and aliases; keep the recovery journal until the previous stack is ready.
- Speech Lab owns its active ASR, TTS, and effective voice stack. AuraGo must read all three from one `/ready` snapshot for chat synthesis, new SIP calls, and Desktop Live Speech `speech_lab` profiles, then require both `X-S2S-TTS-ID` and `X-S2S-Voice` to match on synthesis; legacy `speech_lab.voice` values are load-only and never become runtime defaults. Desktop Live Speech may use the managed or external s2s container through the keyless `local_s2s` realtime provider; it must not send `llm_id` or replace OpenAI/xAI/Gemini streaming adapters.
- AuraGo and the Browser Lab may both activate the shared stack, but AuraGo never sends `llm_id`. Voice choices come only from the selected TTS backend catalog, and active operations or calls keep their immutable start snapshot.
- `speech_lab.chat_llm_provider_id` applies only to the next direct webchat turn marked after Speech Lab ASR. Resolve its static Vault key, unexpired OAuth token, Copilot auth manager, or supported keyless local runtime from the turn snapshot; disable helper and fallback routing for that full turn. Typed chat, browser speech recognition, internal follow-ups, missions, and SIP retain their existing provider routing; an unavailable selected provider fails closed without main-provider fallback.
- Speech Lab ASR provenance uses a single-use five-minute token bound to the session and exact transcript; client-supplied booleans never activate Speech Lab chat routing. Realtime actions emit exactly one `final_response`, then a contentless `done`, and request-scoped cancellation must not interrupt sibling turns in the same session.
- A successful Speech Lab ASR response with empty text is retryable user input, not a provider outage. Chat upload returns `speech_lab_no_speech` with HTTP 422, records only aggregate WAV duration, sample count, peak, and RMS diagnostics, and keeps expected no-speech events below warning level; never log audio or transcript content for this diagnosis.
- When enabled, Speech Lab remains visible in the chat integrations drawer. `advanced_ui_url` is an optional external Browser Lab link; a missing URL is shown as a configuration warning and never blocks ASR/TTS activation.

## Verification

- Run `go test ./internal/speechlab` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
