# WhatsApp Channel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The owner talks to AuraGo over WhatsApp with Telegram parity, gets proactive notifications, and links the account from the Config UI by QR code or phone pairing code. The approved design is `plans/2026-10-31-whatsapp-integration-design.md`; all of its decisions are final.
**Architecture:** A new package `internal/whatsapp` wraps `go.mau.fi/whatsmeow` behind a narrow `waClient` interface. The server owns at most one `whatsapp.Bot` and one `whatsapp.LinkSession` per process, both behind `whatsAppLifecycleMu` (the Telegram pattern). The linked-device keys live in `<DataDir>/whatsapp/device.db` (modernc SQLite through `sqlstore`). That file is outside backups and closed to agent file tools. Status is a passive singleton in `internal/integrationstatus`. Admin-only `/api/whatsapp/*` endpoints drive a link panel in the Config UI. Stages 2 and 3 add the inbound filter chain, the agent pipeline, the broker, formatting, the send queue, media and tools on top of the stage-1 runtime.
**Tech Stack:** Go 1.27.3, CGO-free. `go.mau.fi/whatsmeow` at pseudo-version `v0.0.0-20261030195514-56e5ad0581ce` (MPL-2.0; it pulls in GPL-3.0 `go.mau.fi/libsignal`). `modernc.org/sqlite` v1.60.1. `internal/qrcode` renders server-side PNG QR codes. Vanilla JS Config UI (`ui/cfg/`). rod browser smoke tests.

---

## Standing rules (every stage, every task)

- **DOX:** before editing, re-read the root `AGENTS.md` and every guide on the route: `internal/server/AGENTS.md`, `internal/tools/AGENTS.md`, `internal/qrcode/AGENTS.md`, `internal/fileutil/AGENTS.md`, `licences/AGENTS.md` and `ui/AGENTS.md`. Add `internal/whatsapp/AGENTS.md` once stage 1 Task 14 has created it. Stages 2 and 3 also read `internal/agent/AGENTS.md` and `internal/security/AGENTS.md`. Each stage ends with a DOX pass and reports which guides were left unchanged and why.
- **GitNexus before editing an existing function or method:** run `node .gitnexus/run.cjs impact "<symbol>" --direction upstream --repo .` and report callers, processes and risk. Warn on HIGH or CRITICAL. Treat UNKNOWN as unresolved and confirm with `graft callers <symbol>` or a text search.
- **GitNexus before every commit:** run `node .gitnexus/run.cjs detect-changes --scope all --repo .`. A `partial` or `truncated` result means re-run.
- **Shared checkout:** other sessions commit in this checkout. Never `git add` into the shared index. Commit with:
  ```bash
  F="<space-separated own paths>"
  IDX=$(mktemp) && GIT_INDEX_FILE=$IDX git read-tree HEAD && GIT_INDEX_FILE=$IDX git add $F && GIT_INDEX_FILE=$IDX git commit -m "<msg>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git reset -q -- $F
  ```
  Before you commit:
  - Run `git diff --check -- $F`.
  - Inspect `GIT_INDEX_FILE=$IDX git diff --cached`.
  - Scan for secrets and real phone numbers. Fixtures use fake numbers only, for example `+4917012345678`.
  - Never undo with `git checkout <file>`. Revert only your own hunk.
- **Agents (user roster):**
  - maker-agent writes code.
  - test-agent runs tests.
  - docs-agent writes docs and translations.
  - security-agent reviews the security task of each stage.
  - Each stage gets one combined review (Critical/Important only) and no re-review rounds.
- **Lean tests:** only tests that prove behaviour. Use the fake `waClient`; never real WhatsApp in CI. No mutation, property or stress suites. In Git Bash: `CGO_ENABLED=0 go test ./internal/whatsapp/ -run '(TestA|TestB)' -count=1`.
- **Dependency:**
  - Run `go get go.mau.fi/whatsmeow@v0.0.0-20261030195514-56e5ad0581ce`, then `go mod tidy`.
  - `go.mod` and `go.sum` go into the first commit that imports the module (stage 1 Task 3).
  - Never move the pseudo-version inside a stage.
- **Privacy:** logs and status mask phone numbers to their last four digits. Dropped message content, QR payloads and pairing codes are never logged.
- **Files:**
  - LF line endings, no BOM.
  - Go source written through the Write tool must not contain backslash-u escapes; use literal characters or `\x5c`.
  - Temporary scripts go in `disposable/_whatsapp/`.
- **UI:**
  - All 16 locales get real translations; German uses "Du" and real umlauts.
  - No external assets.
  - Run `node --check` on changed JS.
  - Run `go test -count=1 ./ui/... -run 'Precision|Config|I18N'`.
  - Browser smoke tests need `AURAGO_RUN_BROWSER_SMOKE=1`.
  - Run `npm run build:ui -- --check` whenever bundled JS changes.

## File structure (all stages)

### Stage 1 — foundation (`plans/2026-10-31-whatsapp-1-foundation.md`)

| Path | Responsibility |
| --- | --- |
| `go.mod`, `go.sum` | whatsmeow pseudo-version and its transitive modules |
| `internal/config/whatsapp.go` (+`_test`) | `WhatsAppConfig`, defaults, E.164 normalization and validation, store paths, protected paths |
| `internal/config/config_types.go`, `config.go` | `WhatsApp` field after `Telegram`; pre-decode defaults and post-decode normalization in `Load` |
| `config_template.yaml`, `internal/config/template_integrations_test.go` | reference `whatsapp:` block with safe defaults |
| `internal/audit/sqlite_backup_coverage_test.go` | `device.db` as a documented, deliberate backup exclusion |
| `internal/integrationstatus/whatsapp.go` (+`_test`) | passive status singleton with the 11 states |
| `internal/whatsapp/store.go`, `log.go`, `mask.go` | private device store (modernc DSN, `sqlstore` migration); whatsmeow→slog adapter; digit masking |
| `internal/whatsapp/errors.go`, `client.go` | sentinel errors; the `waClient`, `deviceStore` and `backend` interfaces; whatsmeow adapter; linked-device name "AuraGo" |
| `internal/whatsapp/events.go` | maps whatsmeow events to state, terminal and reconnect policy |
| `internal/whatsapp/runtime.go`, `recipients.go` | `Bot` generation (5 s→5 min backoff, health only after offline sync, presence `unavailable`), owner-recipient resolver, test message, logout |
| `internal/whatsapp/link.go`, `qr.go` | QR and phone link sessions, one at a time, 3-minute TTL; PNG data URI rendering |
| `internal/whatsapp/fake_test.go`, `*_test.go` | fake client, store and backend; unit tests |
| `internal/whatsapp/AGENTS.md` | child DOX guide |
| `internal/server/whatsapp_runtime.go` (+`_test`) | configure, stop and relink lifecycle; handler controller |
| `internal/server/whatsapp_handlers.go` (+`_test`) | `GET /api/whatsapp/status`, `GET/POST /api/whatsapp/link`, `POST /api/whatsapp/logout`, `POST /api/whatsapp/test` |
| `internal/server/server.go`, `server_routes.go`, `server_shutdown.go`, `config_handlers_main.go` | fields, revocation, startup, routes, shutdown, save validation, hot reload |
| `internal/server/backup_handlers.go` | documents why `data/whatsapp` is not backed up |
| `internal/audit/audit_manifests.go` | route contracts and network-client inventory |
| `internal/tools/protected_paths.go`, `internal/tools/whatsapp_protected_paths_test.go` | device store and sidecars denied to agent file tools, like the vault |
| `ui/cfg/whatsapp.js`, `ui/js/config/main.js`, `ui/img/config-sidebar-icons.{svg,json}`, `ui/css/config-workspace.css` | Config section, sidebar icon slot 128, link panel |
| `ui/lang/config/whatsapp/<16>.json`, `ui/lang/config/sections/<16>.json`, `ui/config_help.json` | labels, help, section names |
| `ui/whatsapp_config_browser_test.go` | link panel browser smoke test |
| `THIRD_PARTY_LIBRARIES.md`, `THIRD_PARTY_NOTICES.md`, `licences/UEBERSICHT.md`, `README.md`, `licences/AGENTS.md`, `internal/audit/whatsapp_license_docs_test.go` | MPL/GPL inventory; "MIT source, GPL-3.0 binaries" statement; guard test |
| `AGENTS.md`, `internal/server/AGENTS.md`, `internal/tools/AGENTS.md`, `ui/AGENTS.md` | routing and contracts |

### Stage 2 — text channel (`plans/2026-10-31-whatsapp-2-text.md`)

| Path | Responsibility |
| --- | --- |
| `internal/whatsapp/client.go`, `fake_test.go` | add `MarkRead`, `SendChatPresence`, `OwnLID`, `PNForLID` (`Store.LIDs`) |
| `internal/whatsapp/cache.go` (+test) | bounded TTL ID caches: sent IDs (20 min, max 5000) and the dedupe LRU |
| `internal/whatsapp/inbound.go` (+test) | design filter steps 1–8 and drop counters |
| `internal/whatsapp/format.go`, `chunks.go` (+tests) | Markdown→WhatsApp conversion; 4000-rune chunking that reopens code fences |
| `internal/whatsapp/send.go` (+test) | serialized queue; token bucket of 20/min with burst 5; owner-only recipient guard; echo-cache registration |
| `internal/whatsapp/broker.go` (+test) | `agent.FeedbackBroker` parity with `TelegramBroker` |
| `internal/whatsapp/pipeline.go` (+test) | agent hand-off mirroring `telegram.processUpdate` |
| `internal/whatsapp/runtime.go`, `internal/server/whatsapp_runtime.go` | agent dependencies passed like `telegram.StartLongPolling`; offline cutoff from the latest `Connected` |
| `internal/integrationstatus/whatsapp.go` | drop counters |
| `internal/audit/audit_manifests.go` (+test) | `MessagingIngressManifest` entry (the ingress lands in this stage) |

### Stage 3 — media and tools (`plans/2026-10-31-whatsapp-3-media-tools.md`)

| Path | Responsibility |
| --- | --- |
| `internal/whatsapp/media.go` (+test) | downloads (50 MB cap), uploads, MIME→message type, ffmpeg libopus 16 kHz mono voip voice notes |
| `internal/whatsapp/pipeline.go`, `broker.go` | inbound voice, image, document, placeholders, location, vCard, quoted reply; outbound images, audio, YouTube, voice mode |
| `internal/whatsapp/outbound_api.go` (+test) | owner-only `SendOwnerMessage(ctx, text, filePath)` with readonly gate |
| `internal/agent/{native_tools_integrations,tooling_policy,native_tools_registry,dispatch_messaging,tool_precheck,action_policy,action_policy_risk,tool_categories}.go`, `internal/audit/audit_manifests.go` | `send_whatsapp` registration and gates |
| `internal/tools/notification.go`, `internal/tools/question_user.go` | `ChannelWhatsApp`, question source |
| `internal/prompts/builder.go`, `internal/agent/chat_channels.go`, plus the review list in the design | agent identity and reachable channels |
| `prompts/tools_manuals/send_whatsapp.md` | tool manual |
| `../agotrain/training/tool_tiers.json`, `operation_contracts.json` | catalog reconciliation per `documentation/training.md` |

### Stage 4 — finish (`plans/2026-10-31-whatsapp-4-finish.md`)

| Path | Responsibility |
| --- | --- |
| `documentation/whatsapp_setup.md`, `documentation/manuals/{en,de}/08-integrations.md`, `README.md` | operator docs (14-day rule, max 4 devices, dedicated number, no bulk sending); feature line; keep the README strings pinned by audit tests |
| `internal/server/security_check.go` (+test) | warnings for self-chat on a personal account and for dedicated mode without owners |
| `internal/agent/maintenance_briefing.go` (+test) | WhatsApp integration check like `runTelegramIntegrationCheck` |
| `ui/lang/**` | locale review for keys added in stages 2–3 |
| `make_deploy.sh`, `make_release.ps1` and its scripts, `Dockerfile`, `.github/workflows/release.yml` | GPL-3.0 text and notices beside the binary; release notes link the tagged source |
| license check (`go-licenses report ./cmd/aurago`) | blocks a release on a GPL-3.0-incompatible linked module |

## Stage 1 — Foundation

Detailed plan: `plans/2026-10-31-whatsapp-1-foundation.md` (15 tasks).

Stage 1 delivers:
- the dependency and its license docs
- the config section
- the device store and client adapter
- event and status mapping
- the Bot runtime and link sessions
- server wiring and admin endpoints
- the Config UI link panel and the owner-only test message
- the file-tool denial for the device store
- the DOX updates

Inbound messages are only counted in this stage.

Acceptance:
- all stage-1 tests are green
- linking works on a real second number, by QR and by pairing code
- the test message arrives
- unlink works
- after a restart AuraGo reconnects without relinking

## Stage 2 — Text channel

The detailed plan `plans/2026-10-31-whatsapp-2-text.md` is written after stage 1 lands. It depends on whatsmeow behaviour that stage-1 acceptance verifies: the event order around `Connected`/`OfflineSyncCompleted`, whether `SenderAlt` is populated, and the own LID.

1. **Client extension** — Files: `client.go`, `fake_test.go`. Test focus: the fake still satisfies the interface. Acceptance: `MarkRead`, `SendChatPresence`, `OwnLID` and `PNForLID` are available.
2. **ID caches** — Files: `cache.go`. Test focus: TTL expiry and capacity eviction. Acceptance: the sent-ID cache (20 min / 5000 entries) and the dedupe LRU are ready.
3. **Inbound filter chain** — Files: `inbound.go`, `integrationstatus/whatsapp.go`.
   - Test focus:
     - owner via phone JID, via `SenderAlt`, via the LID store, and unresolved (masked WARN plus counter)
     - drops for groups, broadcasts, newsletters and `status@broadcast`
     - drops for protocol messages, reactions, edits and polls
     - unwrapping of ephemeral and view-once messages
     - echo guard, dedupe, and the offline cutoff (10 min before the latest `Connected`)
     - self-chat accepts only `IsFromMe` in the own phone or LID chat
   - Acceptance: counters appear in status; no content is ever logged.
4. **Formatting and chunking** — Files: `format.go`, `chunks.go`.
   - Test focus:
     - every conversion rule from the design
     - nothing changes inside code spans or fences
     - tables become one bullet per row
     - chunks break at paragraph, then line, boundaries and reopen code fences
   - Acceptance: 4000-rune chunks.
5. **Send queue and recipient guard** — Files: `send.go`, `runtime.go`.
   - Test focus:
     - non-owner JIDs are refused
     - token bucket behaviour
     - sent IDs enter the echo cache
     - delivery failures are counted
     - `ErrRecipientNotOnWhatsApp` is surfaced
   - Acceptance: `SendTestMessage` uses the queue.
6. **Broker** — Files: `broker.go`.
   - Test focus:
     - `question_user`
     - at most one progress message per 10 s
     - budget warnings
   - Acceptance: parity with `TelegramBroker` for text events.
7. **Pipeline** — Files: `pipeline.go`.
   - Test focus:
     - the `default` session
     - a pending-question reply is handled first
     - slash commands run with operator commands allowed
     - a forwarded message gets the marker, no HumanInput, no commands and no pending-question answer
     - the ingress guard blocks at ThreatHigh, then `IsolateExternalData`
     - history uses MessageSource `whatsapp`
     - the HumanInput ID is `whatsapp:<chat>:<msgid>`
     - read receipts go out only in dedicated mode with the setting on
     - `composing` is sent every 8 s, then `paused`
   - Acceptance: `agent.ExecuteAgentLoop` receives the request.
8. **Runtime and server wiring** — Files: `runtime.go`, `internal/server/whatsapp_runtime.go` (+tests). Test focus: the server runtime test stays green; a fake inbound message reaches the pipeline. Acceptance: one Bot per process still holds.
9. **Audit and DOX** — Files: `internal/audit/audit_manifests.go` (+test), `internal/whatsapp/AGENTS.md`, `internal/server/AGENTS.md`. Test focus: the ingress manifest test. Acceptance: contracts are documented.
10. **Real-number check** — text round trip in both modes, formatting on the phone, receipts and typing.

## Stage 3 — Media and tools

The detailed plan `plans/2026-10-31-whatsapp-3-media-tools.md` is written after stage 2 lands. It depends on the verified download, upload and voice behaviour.

1. **Media primitives** — Files: `media.go`.
   - Test focus:
     - the 50 MB cap
     - MIME mapping
     - ffmpeg arguments: libopus, mono, 16 kHz, voip
     - PTT messages use `audio/ogg; codecs=opus`
     - the test is skipped without ffmpeg
   - Acceptance: voice-note bytes and messages are produced.
2. **Inbound media** — Files: `pipeline.go`.
   - Test focus:
     - voice and PTT → MP3 → `tools.TranscribeAudio`
     - images go to vision with the "[USER SENT AN IMAGE]" prefix
     - documents go through `media.SaveAttachmentContext`
     - video and sticker get placeholders
     - location becomes a `[Location: …]` line
     - vCards are fenced as third-party data
     - quoted replies get a `[Reply to: …]` prefix of at most 200 characters
   - Acceptance: every inbound type is handled.
3. **Outbound media** — Files: `broker.go`.
   - Test focus:
     - Markdown images go out as image messages with captions of at most 1024 characters
     - audio and YouTube events are delivered after the run
     - voice mode sends TTS as a voice note
   - Acceptance: parity with Telegram.
4. **Owner send API** — Files: `outbound_api.go`.
   - Test focus:
     - the readonly gate
     - `OpenOutgoingAttachment` only
     - the 50 MB cap
     - owner-only delivery
   - Acceptance: `SendOwnerMessage` works.
5. **`send_whatsapp` tool** — Files: the agent touch points from the design, `prompts/tools_manuals/send_whatsapp.md`, `internal/audit/audit_manifests.go`.
   - Test focus:
     - registry, policy, precheck and risk tests
     - readonly blocks the tool
     - the tool is offered only when WhatsApp is enabled and linked
   - Acceptance: the tool is visible to the agent.
6. **Notifications and question source** — Files: `internal/tools/notification.go`, `question_user.go`, `native_tools_integrations.go`.
   - Test focus:
     - `ChannelWhatsApp` and its participation in `ChannelAll`
     - the enum and the question source
   - Acceptance: `send_notification` reaches WhatsApp.
7. **Agent identity** — Files: `internal/prompts/builder.go`, `internal/agent/chat_channels.go` and the reviewed files from the design. Test focus: the MessageSource label and the reachable-channel list. Acceptance: the agent knows the channel.
8. **Training catalog** — Files: agotrain JSON. Test focus: the catalog `--check`. Acceptance: German and English coverage is kept.
9. **Real-phone check** — the voice note plays, images and documents arrive, a file goes out, a notification arrives.

## Stage 4 — Finish

The detailed plan `plans/2026-10-31-whatsapp-4-finish.md` is written after stage 3 lands.

1. **Operator docs** — Files: `documentation/whatsapp_setup.md`, `documentation/manuals/{en,de}/08-integrations.md`, `README.md`. Test focus: the existing README and docs audit tests. Acceptance: the 14-day rule, the 4-device limit, the dedicated-number advice and the no-bulk rule are documented.
2. **Security check** — Files: `internal/server/security_check.go` (+test). Test focus: both warnings. Acceptance: shown on the security page.
3. **Maintenance briefing** — Files: `internal/agent/maintenance_briefing.go` (+test). Test focus: skipped, passed and failed states. Acceptance: it appears in the briefing.
4. **Locale review** — Files: `ui/lang/**`. Test focus: `TestTranslations_*`. Acceptance: all 16 locales are complete.
5. **Audit re-verification** — Files: `internal/audit/*`. Test focus: route, ingress, tool and network manifests. Acceptance: green.
6. **License check and release packaging** — Files: release scripts, `Dockerfile`, `release.yml`.
   - Test focus:
     - `go-licenses report ./cmd/aurago` contains no GPL-incompatible module
     - existing release-script tests pass
   - Acceptance: archives and the image carry the GPL-3.0 text and the notices; release notes link the tagged source.
7. **Full acceptance and DOX closeout** — the full design checklist on a real second number.

## Risks and mitigations

- **whatsmeow with modernc:** stage 1 Task 3 verifies the combination on a real temp file. `Upgrade` refuses to run without foreign keys.
- **Link hand-off at `PairSuccess`:** the link client is cancelled and disconnected before the Bot connects. Stage 1 Task 15 verifies this on a real device. Fallback: adopt the link client into the Bot.
- **Passkey pairing:** this whatsmeow version can emit passkey-request events, which AuraGo does not support. The session reports the fixed code `passkey_unsupported` and the UI advises the QR method.
- **"Client outdated" outages:** each one needs an AuraGo release with a newer pseudo-version. The `outdated` state stops retrying and tells the user.
- **GPL-3.0 obligations:** every release carries them. The stage 1 Task 4 guard test keeps the notices aligned; stage 4 adds the release checks.
