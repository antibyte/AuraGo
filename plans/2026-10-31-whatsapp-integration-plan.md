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
| `internal/agent/{tool_route,task_router,operational_issues,meshcore_notice,emotion_behavior}.go`, `internal/prompts/builder.go` (+tests) | MessageSource `whatsapp` in the chat-source switches and the channel label (moved from stage 3, see Task 7) |
| `internal/server/dashboard_integrations.go`, `ui/js/dashboard/sections/system.js`, `ui/lang/dashboard/<16>.json`, `internal/server/self_test_{handlers,probes}.go` (+tests) | dashboard integration tile and read-only self-test capability |
| `internal/commands/commands.go` | operator-command comment names WhatsApp next to Telegram |

### Stage 3 — media and tools (`plans/2026-10-31-whatsapp-3-media-tools.md`)

| Path | Responsibility |
| --- | --- |
| `internal/whatsapp/media.go` (+test) | downloads (50 MB cap), uploads, MIME→message type, ffmpeg libopus 16 kHz mono voip voice notes |
| `internal/whatsapp/pipeline.go`, `broker.go` | inbound voice, image, document, placeholders, location, vCard, quoted reply; outbound images, audio, YouTube, voice mode |
| `internal/whatsapp/outbound_api.go` (+test) | owner-only `SendOwnerMessage(ctx, text, filePath)` with readonly gate |
| `internal/agent/{native_tools_integrations,tooling_policy,native_tools_registry,dispatch_messaging,tool_precheck,action_policy,action_policy_risk,tool_categories}.go`, `internal/audit/audit_manifests.go` | `send_whatsapp` registration and gates |
| `internal/tools/notification.go`, `internal/tools/question_user.go` | `ChannelWhatsApp`, question source |
| `internal/agent/{chat_channels,agent_loop_helpers,memory_analysis}.go`, `internal/memory/core_memory_policy.go` | reachable channels via `send_whatsapp`; "whatsapp" as channel and capability cue (the MessageSource switches and label moved to stage 2) |
| `prompts/tools_manuals/send_whatsapp.md` | tool manual |
| `../agotrain/training/tool_tiers.json`, `operation_contracts.json` | catalog reconciliation per `documentation/training.md` |
| `internal/agent/agent_dispatch_comm.go` | `send_whatsapp` in the outer messaging `case` list that hands off to `dispatchMessagingCases` |
| `internal/agent/agent_parse.go`, `internal/security/directive_shape.go` (+tests) | LLM Guardian sees message and file of `send_whatsapp`; directive-shape action-tool list |
| `internal/agent/system_prompt_cache.go`, `internal/prompts/{builder,builder_modules,tool_manual_bindings}.go`, `internal/agent/tool_search_terms/media_communication.yaml`, `internal/agent/tool_family_terms.go`, `internal/agent/testdata/discovery_tasks_v2.json` | prompt tool flag, module flag, manual binding, discovery terms and eval tasks for `send_whatsapp` |
| `internal/server/python_tool_bridge_handler.go`, `ui/js/desktop/apps/sysworld-data.js`, `internal/heartbeat/runner.go` | `send_whatsapp` in the Python tool-bridge group, the System World tool list and the heartbeat prompt |
| `internal/config/go2rtc_detection.go`, `internal/server/person_detect_alerts.go`, `internal/persondetect/alerts.go`, `ui/js/desktop/apps/network-cameras-detection-views.js`, `config_template.yaml` (+tests) | camera person-detection alerts to WhatsApp, with photo |
| `internal/flows/{catalog_actions_notify,catalog_notify_args,catalog_generic}.go`, `internal/server/flows_{catalog_env,handlers_options,bridge_notify,tool_invoker,tool_invoker_args,tool_invoker_status}.go`, `ui/js/desktop/apps/easydrag-{core,dialogs}.js`, `ui/lang/easydrag/<16>.json` (+tests) | EasyDrag `notify.whatsapp` node, notification-channel option, `notify_on_error` channel, generic tool allowlist, invoker rules |
| `internal/config/newspaper.go`, `internal/newspaper/{service,types,store}.go`, `internal/server/newspaper_{delivery,service,handlers}.go`, `ui/cfg/newspaper.js`, `ui/js/desktop/apps/newspaper.js`, `ui/lang/config/newspaper/<16>.json`, `ui/lang/desktop/<16>.json`, `config_template.yaml` (+tests) | newspaper delivery to WhatsApp: allow switch, daily delivery, test send |
| `Dockerfile`, `install.sh`, `internal/agent/native_tools_core.go` | ffmpeg comments and the `send_image` / `send_youtube_video` descriptions name WhatsApp |

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
     - MessageSource `whatsapp` is a chat source wherever `telegram` is one: the supervisor tool route (`internal/agent/tool_route.go:55`), task routing (`task_router.go:95`), operational notices (`operational_issues.go:439`), the MeshCore notice (`meshcore_notice.go:15`), the short-channel tone (`emotion_behavior.go:116,125`) and the label in `internal/prompts/builder.go:1061-1085`. These switches return false for an unknown source, so without this a WhatsApp chat loses the tool route, task routing and the notices as soon as this stage goes live. (Moved from stage 3 Task 7.)
     - the operator-command comments in `internal/commands/commands.go:3,39` name WhatsApp next to Telegram
   - Acceptance: `agent.ExecuteAgentLoop` receives the request.
8. **Runtime and server wiring** — Files: `runtime.go`, `internal/server/whatsapp_runtime.go` (+tests). Test focus: the server runtime test stays green; a fake inbound message reaches the pipeline. Acceptance: one Bot per process still holds.
9. **Status surfaces** — Files: `internal/server/dashboard_integrations.go:12`, `ui/js/dashboard/sections/system.js:155,176`, `ui/lang/dashboard/<16>.json`, `internal/server/self_test_handlers.go:52-54`, `internal/server/self_test_probes.go:20` (+tests).
   - Test focus:
     - the dashboard integration tile appears when WhatsApp is enabled, with icon and translated label
     - the self-test capability `integration:whatsapp` reads `integrationstatus` only and never sends a test message
   - Acceptance: dashboard and self-test show WhatsApp like Telegram. This task needs only the stage-1 status, so it may run earlier.
10. **Audit and DOX** — Files: `internal/audit/audit_manifests.go` (+test), `internal/whatsapp/AGENTS.md`, `internal/server/AGENTS.md`. Test focus: the ingress manifest test. Acceptance: contracts are documented.
11. **Real-number check** — text round trip in both modes, formatting on the phone, receipts and typing.

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
   - Also: the ffmpeg comments in `Dockerfile:54` and `install.sh:2155` name WhatsApp voice notes next to Telegram.
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
   - Also: the `send_image` and `send_youtube_video` descriptions (`internal/agent/native_tools_core.go:273,396`) say how WhatsApp shows them.
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
   - Touch points found by the parity sweep (not in the design list):
     - `internal/agent/agent_dispatch_comm.go:1272`: `send_whatsapp` joins the outer `case` list that hands off to `dispatchMessagingCases`. Without it the tool never reaches `dispatch_messaging.go`. Test: a `send_whatsapp` call reaches its dispatch case.
     - LLM Guardian: `internal/agent/agent_parse.go:2170` (`addGuardianTelegramParams`). The guardian sees the message and the file of `send_whatsapp` with the same bounds as for `send_telegram`.
     - `internal/security/directive_shape.go:96`: `send_whatsapp` in the action-tool list.
     - Prompt and discovery: tool flag in `internal/agent/system_prompt_cache.go:435`; module flag `whatsapp_enabled` in `internal/prompts/builder.go:188,2694` and `builder_modules.go:422`; manual binding in `internal/prompts/tool_manual_bindings.go:87`; en/de search terms in `internal/agent/tool_search_terms/media_communication.yaml:101` (and WhatsApp in the `send_image` terms at 96-97); `internal/agent/tool_family_terms.go:99`; discovery eval tasks in `internal/agent/testdata/discovery_tasks_v2.json`.
     - `internal/server/python_tool_bridge_handler.go:49` tool-bridge group; System World tool list `ui/js/desktop/apps/sysworld-data.js:72`; the heartbeat prompt `internal/heartbeat/runner.go:62`.
6. **Notifications and question source** — Files: `internal/tools/notification.go`, `question_user.go`, `native_tools_integrations.go`.
   - Test focus:
     - `ChannelWhatsApp` and its participation in `ChannelAll`
     - the enum and the question source
   - Acceptance: `send_notification` reaches WhatsApp.
   - Also: the `send_notification` description (`internal/agent/native_tools_integrations.go:1272`) and its category text (`internal/agent/tool_categories.go:333`) list WhatsApp.
7. **Camera person-detection alerts** — Files: `internal/config/go2rtc_detection.go:35` (channel allowlist), `internal/server/person_detect_alerts.go:37,150,184,262`, `internal/persondetect/alerts.go:220`, `ui/js/desktop/apps/network-cameras-detection-views.js:20-21`, `config_template.yaml:1706`.
   - Test focus:
     - the channel `whatsapp` is accepted in the detection config and the camera UI
     - the annotated photo goes out through `SendOwnerMessage` as an image message with the title as caption, and falls back to text like Telegram
     - `all` includes the WhatsApp photo
     - a WhatsApp failure is not hidden by a desktop toast that went out (`internal/persondetect/alert_rules.go:8`)
   - Acceptance: a person alert arrives on WhatsApp with its photo.
8. **EasyDrag flows** — Files: `internal/flows/catalog_actions_notify.go:17`, `catalog_notify_args.go`, `catalog_generic.go:52`, `internal/server/flows_catalog_env.go:225`, `flows_handlers_options.go:121`, `flows_bridge_notify.go:96-111`, `flows_tool_invoker.go`, `flows_tool_invoker_args.go`, `flows_tool_invoker_status.go`, `ui/js/desktop/apps/easydrag-core.js`, `easydrag-dialogs.js:228`, `ui/lang/easydrag/<16>.json`.
   - Test focus:
     - a new `notify.whatsapp` node (pattern of the Telegram and Discord nodes) calls `send_whatsapp` and validates its message
     - `send_whatsapp` is allowed in the generic tool node and maps to the integration `whatsapp`
     - the `notification_channels` option list offers WhatsApp when it is enabled, linked and not readonly
     - `notify_on_error` accepts `whatsapp` with its own send slot and the flood rule
     - invoker status rules and refusals cover `send_whatsapp`; there is no bot token to redact
     - the icon `brand-whatsapp`; the built-in templates stay unchanged
   - Acceptance: a flow sends to WhatsApp and reports its failures there.
9. **Newspaper delivery** — Files: `internal/config/newspaper.go:36`, `internal/newspaper/types.go:67`, `service.go:20,329,345`, `store.go:487`, `internal/server/newspaper_delivery.go:72`, `newspaper_service.go:51`, `newspaper_handlers.go:154,270`, `ui/cfg/newspaper.js:13`, `ui/js/desktop/apps/newspaper.js:142,216`, `ui/lang/config/newspaper/<16>.json`, `ui/lang/desktop/<16>.json`, `config_template.yaml:1278`, `internal/audit/audit_manifests.go:272`.
   - Test focus:
     - `allow_whatsapp` and the readonly gates
     - `whatsapp_daily` and manual delivery: digest text, the PDF as a document within 50 MB, text chunks
     - the destination is the owner from the recipient resolver; delivery records carry the channel `whatsapp`
     - the `whatsapp_ready` capability and the test send
   - Acceptance: an edition arrives on WhatsApp.
10. **Agent identity** — Files: `internal/agent/chat_channels.go`, `agent_loop_helpers.go:2297,2377`, `memory_analysis.go:586`, `internal/memory/core_memory_policy.go:322,428`. Test focus: the reachable-channel list names `send_whatsapp`; "whatsapp" counts as a channel and capability cue. Acceptance: the agent knows the channel. The MessageSource switches and the label moved to stage 2 Task 7.

Tasks 7–9 need `SendOwnerMessage` (Task 4) and `ChannelWhatsApp` (Task 6). If stage 3 grows too large, they may become their own detailed plan `plans/2026-10-31-whatsapp-3b-targets.md` after tasks 1–6 land.

11. **Training catalog** — Files: agotrain JSON. Test focus: the catalog `--check`. Acceptance: German and English coverage is kept.
12. **Real-phone check** — the voice note plays, images and documents arrive, a file goes out, a notification arrives, and a camera alert, a flow notification and a newspaper edition arrive.

## Stage 4 — Finish

The detailed plan `plans/2026-10-31-whatsapp-4-finish.md` is written after stage 3 lands.

1. **Operator docs** — Files: `documentation/whatsapp_setup.md`, `documentation/manuals/{en,de}/08-integrations.md`, `README.md`. Test focus: the existing README and docs audit tests. Acceptance: the 14-day rule, the 4-device limit, the dedicated-number advice and the no-bulk rule are documented.
2. **Security check** — Files: `internal/server/security_check.go` (+test). Test focus: both warnings. Acceptance: shown on the security page. Also name the WhatsApp device keys next to the vault in the Danger Zone unsafe-host-execution warning (16 locales), since shell/Python can read both there (stage-1 deviation 16).
3. **Maintenance briefing** — Files: `internal/agent/maintenance_briefing.go` (+test). Test focus: skipped, passed and failed states. Acceptance: it appears in the briefing.
4. **Locale review** — Files: `ui/lang/**`. Test focus: `TestTranslations_*`. Acceptance: all 16 locales are complete.
5. **Audit re-verification** — Files: `internal/audit/*`. Test focus: route, ingress, tool and network manifests. Acceptance: green.
6. **License check and release packaging** — Files: release scripts, `Dockerfile`, `release.yml`.
   - Test focus:
     - `go-licenses report ./cmd/aurago` contains no GPL-incompatible module
     - existing release-script tests pass
   - Acceptance: archives and the image carry the GPL-3.0 text and the notices; release notes link the tagged source.
7. **Full acceptance and DOX closeout** — the full design checklist on a real second number. Re-run the Telegram parity sweep below: every new hit is either mirrored or listed under "Deliberately not mirrored".

## Telegram parity sweep (2026-11-01)

After stage 1 landed, every file that names Telegram was compared with the files that name WhatsApp:

~~~bash
git grep -il telegram -- ':!vendor' ':!graft' ':!.gitnexus' ':!*.md' ':!plans' ':!reports' ':!documentation' ':!disposable' | sort > /tmp/tg.txt
git grep -il whatsapp -- ':!vendor' ':!graft' ':!.gitnexus' | sort > /tmp/wa.txt
comm -23 /tmp/tg.txt /tmp/wa.txt
~~~

Files that name both were checked for lists where only Telegram appears (`git grep -n -i telegram -- <file> | grep -v -i whatsapp`).

Targets that the design did not list were added to stage 2 Tasks 7 and 9 and stage 3 Tasks 1, 3, 5, 6 and 7–10. Two of them block stage 3: the outer dispatch `case` list (`internal/agent/agent_dispatch_comm.go:1272`) and the LLM Guardian parameters (`internal/agent/agent_parse.go:2170`). The MessageSource switches moved from stage 3 to stage 2, because they decide the tool route and task routing of every WhatsApp chat.

### Deliberately not mirrored

- `telegram_read` in `internal/security/untrusted_source.go:25` and `internal/security/tool_output_trust.go:122`: WhatsApp is an owner channel (design, Security).
- Vault secret mapping (`internal/config/config_migrate.go:819`, `internal/config/config_secret_migration.go:18`, `internal/server/config_handlers_main.go:2517`, `internal/tools/python_secrets.go:89`): WhatsApp has no token; its keys live in the device store.
- The Python skill template `internal/tools/skilltemplates/notification_sender.py` and the `python-telegram-bot` mapping in `internal/tools/skills.go:788`: there is no WhatsApp HTTP API a skill could call, and owner-only sending stays in Go.
- The Telegram connection test (`/api/telegram/test-connection`, `ui/js/config/integration_actions.js`, `ui/js/config/catalog.js:47,189`): the WhatsApp link panel has its own status and test message.
- The egg config (`internal/invasion/eggconfig.go:157`): the WhatsApp runtime refuses to start in egg mode, and `whatsapp.enabled` defaults to false.
- Reuse of Telegram helpers in Discord and the voice upload (`internal/discord/bot.go`, `internal/server/handlers_voice.go`), and the example texts in `ui/lang/knowledge` and `ui/lang/help`: no target.
- Comments only: `internal/rocketchat/bot.go:143`, `internal/agent/question_user_answer.go:32`, `internal/security/scrubber.go:315`, `internal/tools/outgoing_attachments.go`, `internal/tools/vision.go`.

## Risks and mitigations

- **whatsmeow with modernc:** stage 1 Task 3 verifies the combination on a real temp file. `Upgrade` refuses to run without foreign keys.
- **Link hand-off at `PairSuccess`:** the link client is cancelled and disconnected before the Bot connects. Stage 1 Task 15 verifies this on a real device. Fallback: adopt the link client into the Bot.
- **Passkey pairing:** this whatsmeow version can emit passkey-request events, which AuraGo does not support. The session reports the fixed code `passkey_unsupported` and the UI advises the QR method.
- **"Client outdated" outages:** each one needs an AuraGo release with a newer pseudo-version. The `outdated` state stops retrying and tells the user.
- **GPL-3.0 obligations:** every release carries them. The stage 1 Task 4 guard test keeps the notices aligned; stage 4 adds the release checks.
