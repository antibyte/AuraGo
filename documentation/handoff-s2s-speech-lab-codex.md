# Handoff for Codex: AuraGo × s2s Speech Lab (remaining work)

**Audience:** Codex (or any implementer) continuing **AuraGo-side** work only.  
**Date:** 2026-08-01 (rewritten — previous §4 “P0 baseline” is already implemented)  
**Working tree:** `C:\Users\Andi\Documents\repo\AuraGo` on `main` (local commits ahead of `origin/main` may include Speech Lab)  
**s2s API owner:** `D:\repo\s2s\s2s-vulkan` — contract `docs/aurago-integration.md`

---

## 0. Read this first

| Do | Don't |
|----|--------|
| Treat **§1–§3 as done**. Verify with tests; do not rewrite the client or config shape. | Rebuild Speech Lab from the old baseline handoff |
| Implement **§4 remaining gaps** against the live s2s contract | Send `llm_id` on stack activate; scrape SPA `:8088` for `/ready` |
| Keep speech_lab commits **isolated** from the huge dirty AuraGo tree (tsnet, copilot, etc.) | Mix unrelated local mods into Speech Lab PRs |
| Prefer additive changes + tests | Silent cloud fallback when Speech Lab is selected |

**Product goal (unchanged)**

1. User chooses ASR+TTS (s2s suggestions are **heuristic only** — never auto-apply).  
2. Live chat / classic SIP use the **fixed gateway** (`/v1/audio/*`, `/ready`).  
3. **LLM stays in AuraGo** (optional `chat_llm_provider_id` only for Speech-Lab-transcribed webchat turns).

---

## 1. Status summary

| Layer | Status |
|-------|--------|
| s2s capability / suggestions / gateway / voice_mode / readiness snapshot | **Done** in `D:\repo\s2s\s2s-vulkan` |
| AuraGo config, HTTP client, SIP/chat channels, Config UI stack editor, native APIs | **Baseline + hardening done** (see §2 commits) |
| AuraGo polish for **voice_mode**, manuals, suggestion UX, structured error codes | **Remaining — this handoff §4** |

### Recent AuraGo commits (do not re-do)

```
fd1aa10ce fix: harden Speech Lab runtime routing
588b37876 feat: follow active Speech Lab voice stack
e40006ac0 fix: keep Speech Lab config fields in flow
c6caaa602 fix: serialize Speech Lab SIP stack changes
9c4cedea1 feat: integrate local Speech Lab providers
```

Verify: `git log --oneline origin/main..HEAD` and `git show --stat fd1aa10ce`.

---

## 2. Already implemented (baseline — verify, don’t invent)

### Config (`speech_lab`)

Canonical shape (see `config_template.yaml` + `internal/config/speech_lab.go`):

```yaml
speech_lab:
  enabled: false
  base_url: http://s2s-vulkan:8765
  advanced_ui_url: ""          # optional external Browser Lab link only — never iframe/proxy
  language: de
  chat_llm_provider_id: ""     # AuraGo provider for Speech-Lab ASR webchat turns only
  timeout_seconds: 60
  sip_enabled: false           # permits explicit speech_lab in Telephone agent
  chat_input_enabled: false    # AudioWorklet → PCM16/16 kHz WAV → local ASR
  chat_output_enabled: false   # local TTS for chat; text survives synthesis errors
```

- Env override: `AURAGO_SPEECH_LAB_BASE_URL`  
- Legacy `use_for_sip` / `use_for_chat_voice` / `voice` load-compatible; **voice is never a runtime default** (owned by s2s `/ready` stack).  
- Private-network dial policy on the client (loopback/private only).

### Packages & wiring

| Area | Location |
|------|----------|
| Client | `internal/speechlab/` (`client.go`, `wav.go`, tests) |
| Config | `internal/config/speech_lab.go` (+ tests) |
| Admin APIs | `internal/server/speech_lab_handlers.go` → `/api/speech-lab/{status,capability,catalog,suggestions,stack}` |
| Chat LLM turn | `internal/server/speech_lab_chat.go` |
| Chat TTS | `internal/server/chat_voice_output.go` |
| SIP classic | `internal/server/sip_voice.go` (+ stack-change reservation) |
| Config UI | `ui/cfg/speech_lab.js`, `ui/lang/config/speech_lab/*` |
| Chat recorder | `ui/js/chat/modules/speech-lab-recorder.js` (+ worklet) |
| Compose | `deploy/docker/docker-compose.s2s.yml` |
| Operator doc | `documentation/s2s_speech_lab.md` |
| Contracts in tree | `AGENTS.md` § Speech Lab Integration Contract |

### Runtime contracts already enforced

- Preflight `/ready` before SIP answer / chat TTS; fail closed (`speech_lab_not_ready` → SIP 480 / HTTP 503).  
- Snapshot `asr_id` / `tts_id` / `voice` from **one** `/ready` response for the call/turn.  
- Synthesis requires matching `X-S2S-TTS-ID` **and** `X-S2S-Voice`.  
- Stack `PUT` never sends `llm_id`; blocked while SIP call or speech operation active.  
- No automatic cloud ASR/TTS fallback when Speech Lab is the selected channel.

### Tests to run before changing code

```powershell
cd C:\Users\Andi\Documents\repo\AuraGo
go test ./internal/speechlab ./internal/config -run SpeechLab -count=1
go test ./internal/server -run SpeechLab -count=1
go test ./ui -run SpeechLab -count=1
```

Broader: `go test ./internal/tools ./internal/server ./internal/config -count=1` if you touch shared TTS/SIP paths.

---

## 3. s2s HTTP contract (source of truth)

Full doc: `D:\repo\s2s\s2s-vulkan\docs\aurago-integration.md`

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/v1/capability` | HardwareProfile |
| `GET` | `/api/v1/suggestions?language=de&stable_only=true` | Heuristic pairs/presets (`scoring: heuristic_v1`) |
| `GET` | `/api/v1/catalog` | Backends + availability + `voice_mode` + voices |
| `PUT` | `/api/v1/stack` | `{asr_id,tts_id,voice?}` — **200 only if `ok:true`**, else 409 |
| `POST` | `/v1/audio/transcriptions` | ASR (multipart WAV ≤ 8 MiB) |
| `POST` | `/v1/audio/speech` | TTS JSON → WAV; non-empty `model` → 400 |
| `GET` | `/ready` | 200 ready / 503 not ready; `asr_id,tts_id,voice,asr_ok,tts_ok` |
| `GET` | `/health` | Liveness |

**Call orchestrator `:8765`**, not lab SPA `:8088` (SPA only proxies `/api/` + `/ws`).

**Production s2s env:**

```text
S2S_LAB_IDLE_UNLOAD_SECS=0
S2S_ALLOW_EXPERIMENTAL=false
```

### Capability JSON (real fields)

```json
{
  "vendor": "intel",
  "device_name": "Intel Arc B580",
  "accelerators": ["cpu", "vulkan"],
  "platform": "windows",
  "in_container": true,
  "allow_experimental": false,
  "vram_total_gb": null,
  "ram_total_gb": null,
  "host_agent_online": true,
  "host_profiles": ["piper", "..."],
  "tier": "host-vulkan"
}
```

There is **no** nested `gpu` object. UI currently falls back through several keys — align deliberately (see §4.2).

### TTS `voice_mode` (s2s — critical for remaining work)

Every TTS catalog entry declares `voice_mode`:

| Mode | Meaning | AuraGo implication |
|------|---------|-------------------|
| `request` | Voice may change per synthesis request | Voice dropdown free; gateway accepts `voice` each call |
| `restart` | Voice is part of loaded sidecar identity (e.g. Piper) | Voice only via stack activate / restart; wrong voice at runtime → **HTTP 409** `voice_not_active` |
| `fixed` | Only catalog default | UI should not offer alternate speakers |

Missing `voice_mode` on external catalogs defaults to **`fixed`** on s2s.  
Gateway never substitutes another speaker.

Response headers on successful TTS: `X-S2S-TTS-ID`, `X-S2S-Voice`.

---

## 4. Remaining AuraGo work (prioritized)

### P0 — Contract alignment (do these)

#### 4.1 Parse and honor `voice_mode` in stack UI + client validation

**Problem:**  
`internal/speechlab.CatalogBackend` does **not** include `voice_mode`.  
`ui/cfg/speech_lab.js` always offers a free voice list for any TTS backend.  
`validateStackRequest` only checks membership in `voices[]`.

**Required**

1. Add `VoiceMode string \`json:"voice_mode"\`` (or enum) on catalog parse path used by `ActivateStack` validation.  
   - Accept: `request` | `restart` | `fixed` (case-insensitive); empty → treat as `fixed` (match s2s).  
2. Config UI (`speech_lab.js`):  
   - Show mode badge / help text per selected TTS.  
   - For `fixed`: disable voice select (or single option = default).  
   - For `restart`: allow selecting voice **only as part of stack apply** (current PUT is correct); after apply, voice comes from `/ready` status — do not imply mid-call changes.  
   - For `request`: free select; runtime may pass voice on synthesize.  
3. Map HTTP **409** from s2s (body may include `code: "voice_not_active"`) to a clear UI toast + structured client error (do not surface as generic “unavailable”).  
4. Unit tests: catalog fixture with each mode; invalid voice for fixed/restart rejected before PUT where possible; 409 handling.

**Files:** `internal/speechlab/client.go`, `client_test.go`, `ui/cfg/speech_lab.js`, `ui/config_speech_lab_test.go`, `ui/lang/config/speech_lab/{en,de}.json` (+ other locales if project policy requires all locales in the same PR).

#### 4.2 Capability panel field mapping

**Problem:** UI uses `speechLabCapability.gpu || speechLabCapability.hardware` and assorted aliases; s2s returns a **flat** HardwareProfile (`vendor`, `device_name`, `tier`, `accelerators`, `host_agent_online`, …).

**Required**

- Render: `tier`, `device_name`/`vendor`, `accelerators`, `host_agent_online`, optional VRAM/RAM.  
- Copy: suggestions are **“voraussichtlich” / predicted**, not measured (already partially in i18n — keep consistent).  
- Test with a fixture matching s2s JSON (no nested `gpu`).

#### 4.3 Manual documentation

**Problem:** `documentation/manual/{en,de}/08-integrations.md` has **no** Speech Lab section (operator doc `s2s_speech_lab.md` exists).

**Required**

- Add a short Integrations chapter section (EN + DE): enablement, channels, Config path **Media → Speech Lab**, link to Browser Lab via `advanced_ui_url`, compose overlay, production idle=0 note, no cloud fallback.  
- Cross-link `documentation/s2s_speech_lab.md`.  
- Do **not** paste secrets or HF token instructions into AuraGo manuals (HF stays s2s-side).

### P1 — Product UX polish

#### 4.4 Suggestion chips → prefill stack form

Today suggestion chips are display-only. Make them **clickable**: set ASR/TTS (and default voice from catalog) selects, still require **Apply** + confirm (never auto-PUT).

#### 4.5 Clickable “recommended” reasons

Show `score` / `reason` / `vram_gb` from `suggested_pairs` in the chip title or help text (heuristic_v1). Never claim “benchmarked”.

#### 4.6 Integrations drawer / status polish

When `enabled` but `/ready` false, drawer already shows state — ensure message points to `/config#speech_lab` and optional `advanced_ui_url`.

### P2 — Ops & packaging

#### 4.7 Host vs Docker base_url matrix

Document clearly in `s2s_speech_lab.md` (expand if needed):

| AuraGo deployment | `base_url` | s2s requirement |
|-------------------|------------|-----------------|
| Same Docker network | `http://s2s-vulkan:8765` | join `S2S_DOCKER_NETWORK` via `deploy/docker/docker-compose.s2s.yml` |
| Native Windows process | `http://127.0.0.1:8765` | s2s host overlay publishing **only** `127.0.0.1:8765` (never LAN by default) |

#### 4.8 Optional dashboard tile

Sparse poll of `/api/speech-lab/status` (authenticated) — ready + active ASR/TTS/voice. No high-frequency polling.

### P3 — Out of scope unless product asks

- Matrix benchmarks / WER-RTF UI  
- Replacing Gemini Live / OpenAI Realtime with s2s full lab pipeline  
- Proxying Docker socket or HF tokens through AuraGo  
- Auto stack switch without user confirm  

---

## 5. Implementation notes for Codex

### Client entry points (existing)

```go
// internal/speechlab
client.Ready(ctx)
client.Require(ctx, needASR, needTTS)
client.ActivateStack(ctx, StackRequest{ASRID, TTSID, Voice}) // never llm_id
client.Transcribe(ctx, wav, language, expectedASRID)
client.Synthesize(ctx, text, language, voice, expectedTTSID, expectedVoice)
```

### SIP / chat channel selection

- Telephone: `sip_enabled` **and** classic ASR/TTS mode `speech_lab` (hybrid allowed).  
- Chat in: `chat_input_enabled` → worklet recorder only (no Web Speech parallel).  
- Chat out: `chat_output_enabled` → Speech Lab TTS; text remains on audio error.  
- `chat_llm_provider_id`: only next webchat turn after Speech Lab ASR; fail closed `speech_lab_llm_unavailable`.

### Compose attach (AuraGo does not start s2s)

```powershell
# s2s stack first, idle unload off
# $env:S2S_LAB_IDLE_UNLOAD_SECS = "0"
docker network ls   # note e.g. s2s-vulkan_s2s
$env:S2S_DOCKER_NETWORK = "s2s-vulkan_s2s"
docker compose -f docker-compose.yml -f deploy/docker/docker-compose.s2s.yml up -d
```

### Git hygiene

AuraGo working tree is often **very dirty** with unrelated features. For Speech Lab work:

```powershell
git status -sb
# stage only speech_lab / speechlab / manual speech sections
git add documentation/handoff-s2s-speech-lab-codex.md `
  documentation/s2s_speech_lab.md `
  documentation/manual/en/08-integrations.md `
  documentation/manual/de/08-integrations.md `
  internal/speechlab/ `
  ui/cfg/speech_lab.js `
  ui/config_speech_lab_test.go `
  ui/lang/config/speech_lab/
# …plus any files you actually edit
```

Do **not** commit tsnet/copilot/maintenance noise into the Speech Lab commit.

---

## 6. Acceptance criteria (remaining work complete)

- [ ] Catalog `voice_mode` drives UI + validation for request/restart/fixed.  
- [ ] Wrong voice on restart/fixed surfaces clear **409 / voice_not_active** messaging (not generic gateway error).  
- [ ] Capability panel matches flat s2s HardwareProfile fields.  
- [ ] EN + DE manual sections for Speech Lab exist and link operator doc.  
- [ ] Suggestion chips can prefill stack form; apply still requires confirm.  
- [ ] Existing tests still pass; new tests cover voice_mode + 409.  
- [ ] Live smoke: s2s ready → AuraGo Config stack apply → chat TTS **or** classic SIP with no cloud ASR/TTS.  
- [ ] Disabling Speech Lab / channel flags restores prior provider behavior (regression).

### Live smoke (s2s must be healthy first)

```powershell
# from host with published 8765, or docker exec s2s-vulkan
curl -fsS http://127.0.0.1:8765/health
curl -fsS http://127.0.0.1:8765/ready
curl -fsS "http://127.0.0.1:8765/api/v1/capability"
curl -fsS "http://127.0.0.1:8765/api/v1/suggestions?language=de&stable_only=true&limit=4"
# TTS (voice must match active stack / mode)
curl -fsS -X POST http://127.0.0.1:8765/v1/audio/speech `
  -H "Content-Type: application/json" `
  -d "{\"input\":\"Hallo\",\"language\":\"de\",\"voice\":\"M1\",\"response_format\":\"wav\"}" `
  -o $env:TEMP\s2s-tts.wav
```

AuraGo: Config → Media → Speech Lab → enable channels → Apply stack → verify status ready → chat mic or SIP classic.

---

## 7. Known pitfalls

| Pitfall | Mitigation |
|---------|------------|
| SPA `:8088` for `/ready` / `/v1/audio/*` | Orchestrator `:8765` only |
| Idle unload kills backends mid-call | `S2S_LAB_IDLE_UNLOAD_SECS=0` on s2s |
| Language `german` vs `de` | AuraGo sends ISO `de`/`en`; s2s maps synonyms |
| Host-managed Piper voices only `thorsten` / `libritts` | Catalog voices list is source of truth |
| Host agent offline | Capability `host_agent_online`; prefer Docker-stable backends for default stack |
| Experimental backends hidden | UI filter + s2s `S2S_ALLOW_EXPERIMENTAL` |
| Concurrent AuraGo dirty tree | Isolate speech_lab commits |
| s2s container restarting | Wait for healthy; 502 on web proxy means orchestrator not ready |

---

## 8. Suggested Codex checklist

1. Read this file + `documentation/s2s_speech_lab.md` + s2s `docs/aurago-integration.md`.  
2. Run existing Speech Lab unit tests (must be green before edits).  
3. Implement **§4.1 voice_mode** + tests.  
4. Implement **§4.2 capability UI** + **§4.3 manuals**.  
5. Optional P1: suggestion prefill.  
6. Live E2E smoke against s2s with idle unload 0.  
7. Commit only Speech Lab files; push / open PR focused on “Speech Lab contract polish”.

---

## 9. If s2s API gaps appear

| Area | s2s path |
|------|----------|
| Gateway / ready / TTS voice enforcement | `s2s-vulkan/src/gateway.rs` |
| Stack activate / voice restart | `s2s-vulkan/src/lab.rs` |
| Suggestions | `s2s-vulkan/src/suggest.rs` |
| HardwareProfile / catalog / voice_mode | `s2s-vulkan/src/registry.rs` |
| HTTP routes | `s2s-vulkan/src/io/websocket.rs` |

Prefer **additive** JSON fields on `/ready` or catalog over scraping lab UI. Document any contract change in `docs/aurago-integration.md` in the **s2s** repo (not only AuraGo).

Pointer from s2s: `s2s-vulkan/docs/handoff-aurago-codex.md`.

---

**End of handoff.** Codex should implement §4 only; §2 is the verified baseline.
