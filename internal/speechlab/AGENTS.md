# Speech Lab

## Purpose

Active ASR/TTS snapshots and speech-driven chat routing.

## Ownership

`internal/speechlab` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Speech Lab Integration Contract
- The signed managed bundle starts Confucius4-R2T2 GGUF as its default ASR through llama.cpp `POST /v1/audio/transcriptions`, with a pinned model and audio projector. Select its published CPU, NVIDIA CUDA, or Vulkan image from the validated hardware profile before pulling and starting; leave older bundles unchanged. Keep the stable `confucius-asr` network alias and the gateway's selected ASR protocol/model consistent with the catalog and `/ready` snapshot.
- Pass the selected accelerator and detected host DRM vendor into the managed gateway catalog profile. The gateway must resolve the same GPU variant that the managed ASR container runs, so the active backend remains available in the Browser Lab and AuraGo Config UI.
- The signed managed bundle exposes published experimental variants only when they match the host platform and hardware. Browser Lab labels them and requires a warning modal before installation or activation; cancellation leaves the active stack and installed models unchanged. Unpublished runtimes and incompatible variants stay unavailable.
- Managed updates journal each container's network aliases. Rollback reconnects missing aliases and accepts an existing Docker endpoint only after inspecting its actual attachment and aliases; keep the recovery journal until the previous stack is ready.
- Managed updates snapshot installed module variants and the active ASR, TTS, LLM, and voice selection before replacing services. An active stack transition blocks the update before container mutation and preserves the previous ready state for retry. Restore modules only through the new signed bundle's controller, verify their labels and runtime readiness, then reactivate and verify the saved stack before removing old backups. A failed or interrupted restore rolls back the new containers and restores the previous modules and stack; an unchanged bundle reconciles module IDs without replacing them.
- Speech Lab owns its active ASR, TTS, and effective voice stack. AuraGo must read all three from one `/ready` snapshot for chat synthesis, new SIP calls, and Desktop Live Speech `speech_lab` profiles, then require both `X-S2S-TTS-ID` and `X-S2S-Voice` to match on synthesis; legacy `speech_lab.voice` values are load-only and never become runtime defaults. Desktop Live Speech may use the managed or external s2s container through the keyless `local_s2s` realtime provider; it must not send `llm_id` or replace OpenAI/xAI/Gemini streaming adapters.
- Live Speech progress audio for a Speech Lab profile uses that same ready TTS/voice snapshot. Progress text is server-owned and transient; it must never become a chat answer or override the active stack.
- AuraGo and the Browser Lab may both activate the shared stack. Normal AuraGo stack changes never send `llm_id`; the managed updater sends only the LLM ID captured from the previous stack while restoring it. Voice choices come only from the selected TTS backend catalog, and active operations or calls keep their immutable start snapshot.
- `speech_lab.chat_llm_provider_id` applies only to the next direct webchat turn marked after Speech Lab ASR. Resolve its static Vault key, unexpired OAuth token, Copilot auth manager, or supported keyless local runtime from the turn snapshot; disable helper and fallback routing for that full turn. Typed chat, browser speech recognition, internal follow-ups, missions, and SIP retain their existing provider routing; an unavailable selected provider fails closed without main-provider fallback.
- Speech Lab ASR provenance uses a single-use five-minute token bound to the session and exact transcript; client-supplied booleans never activate Speech Lab chat routing. Realtime actions emit exactly one `final_response`, then a contentless `done`, and request-scoped cancellation must not interrupt sibling turns in the same session.
- A successful Speech Lab ASR response with empty text is retryable user input, not a provider outage. Chat upload returns `speech_lab_no_speech` with HTTP 422, records only aggregate WAV duration, sample count, peak, and RMS diagnostics, and keeps expected no-speech events below warning level; never log audio or transcript content for this diagnosis.
- When enabled, Speech Lab remains visible in the chat integrations drawer. Browser links use only `/speech-lab/` under AuraGo authentication; `advanced_ui_url` remains load-only. External installations configure `browser_backend_url` separately. A missing backend URL warns but never blocks ASR/TTS activation.

## Verification

- Run `go test ./internal/speechlab` and the named cross-component checks in the contracts above when those paths change.
- Run `go test ./internal/speechlab/deployer` for managed update, rollback, and interrupted-recovery regressions.

## Child DOX Index

None.
