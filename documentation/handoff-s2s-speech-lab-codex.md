# Speech Lab maintenance and acceptance

## Ownership and current contract

AuraGo owns conversation control, LLM routing, tools, Guardian, and memory. s2s owns the active ASR, TTS, and voice stack. Read [the operator guide](s2s_speech_lab.md), `internal/speechlab/AGENTS.md`, the server/UI owners, and the s2s repository's `s2s-vulkan/docs/aurago-integration.md` before changing this boundary.

- Managed deployment installs a signed, digest-pinned bundle after administrator confirmation; external deployment keeps an existing s2s installation. Never infer that enabling a channel permits a first image download.
- Browser access uses the authenticated AuraGo `/speech-lab/` route. External mode configures `browser_backend_url`; legacy `advanced_ui_url` is load-only, not a browser link. There is no separate Tailscale listener.
- Chat input, chat output, classic SIP and Desktop Live Speech keep their independent opt-ins. `chat_llm_provider_id` applies only to the next provenance-marked webchat turn. No silent cloud or main-provider fallback.
- Each conversation reads ASR, TTS and voice from one `/ready` snapshot. TTS verifies both `X-S2S-TTS-ID` and `X-S2S-Voice`; existing calls keep their start snapshot. Normal stack changes omit `llm_id` and are blocked during speech operations/SIP calls.
- Catalog `voice_mode` controls validation and the editor: `fixed` uses only `default_voice`, `restart` permits a confirmed stack switch with backend restart, and `request` permits catalog voices. Missing/empty modes mean fixed; unknown modes block activation.
- Known gateway `code` and stack `error_code` conflicts retain HTTP 409 and safe application-owned messages. `voice_not_active` requires refresh; missing modules require checking Browser Lab installation. Unknown conflicts become `speech_lab_conflict`; upstream error bodies never reach messages.
- Hardware capability uses flat `vendor`, `device_name`, `tier`, `accelerators`, `host_agent_online` and optional memory fields. Missing memory and an offline host are valid observations.
- Recommendation buttons prefill available ASR/TTS choices and a permitted default voice only. Score/reason are escaped heuristic metadata, not benchmarks. Apply still requires explicit confirmation.
- The drawer respects server-provided browser availability, preserves sign-in/status hints, and offers `/config#speech_lab` when attention is needed.

Both [English](manuals/en/08-integrations.md#speech-lab-integration) and [German](manuals/de/08-integrations.md#speech-lab-integration) manuals already contain Speech Lab sections. Update them in place and translate new interface text into all 16 UI locales.

## Local verification

```powershell
go test ./internal/speechlab -count=1
go test -p 1 ./internal/config ./internal/server ./ui -run SpeechLab -count=1
$env:AURAGO_RUN_BROWSER_SMOKE='1'
go test ./ui -run 'TestConfigSpeechLab.*Browser|TestSpeechLabDrawerRespectsAvailabilityBrowser' -count=1
npm run check:ui
```

Client tests cover modes, validation before mutation, conflict decoding and bounded error bodies. Server tests cover REST and SSE propagation. Browser fixtures cover the flat hardware profile, voice controls, suggestion prefill without automatic PUT, cancelled confirmation, conflict messaging and drawer availability. Existing tests cover disabled channels and immutable backend/voice snapshots. Managed lifecycle changes also require `go test ./internal/speechlab/deployer`.

## Live acceptance still requires a healthy installation

Fixtures do not prove microphone, GPU, model delivery or audio-device behavior. On the intended installation:

1. Read `/health`, `/ready`, `/api/v1/catalog` and `/api/v1/capability` from the configured orchestrator, not the Browser Lab frontend. Record the active backend IDs and voice without recording speech or credentials.
2. Confirm a catalog-valid stack activation in AuraGo Config. Check the returned readiness and effective voice. Do not change an in-use stack.
3. Generate a short chat TTS response or make an explicitly authorized classic SIP test call. Verify the selected local ASR/TTS path, both TTS identity headers, playable WAV, and no cloud fallback.
4. Check a controlled voice conflict produces HTTP 409 / `voice_not_active` and readable guidance while retaining chat text.
5. Restore any temporary settings. With Speech Lab/channel flags disabled, verify the previous provider behavior.

Keep live evidence in ignored `reports/`, not this document. Use `S2S_LAB_IDLE_UNLOAD_SECS=0` for production speech. Do not widen the private-address policy to CGNAT merely to complete a smoke test; external Tailscale backends need an explicit separate requirement.

## Change hygiene

Stage only the intended Speech Lab code, tests, translations, generated bundle changes and documentation. Preserve unrelated local work. Review the owning DOX contracts, perform impact/change analysis, and commit the completed change locally. Publishing or deployment is a separate action.
