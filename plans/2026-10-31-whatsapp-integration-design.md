# WhatsApp channel — design (2026-10-31)

Status: approved design. Implementation plan follows in `plans/2026-10-31-whatsapp-integration-plan.md`.

## Goal

The owner talks to AuraGo over WhatsApp with Telegram parity, receives proactive notifications, and links the account from the Config UI by QR code or phone pairing code.

## Decisions (approved 2026-10-31)

1. **Transport.** `go.mau.fi/whatsmeow` in-process (WhatsApp Web multi-device; AuraGo is a linked companion device). A narrow internal `waClient` interface wraps it for test fakes and a possible future official backend.
2. **Number.** Default mode `dedicated`: own bot number (e.g. a second eSIM); the owner writes to it. Optional mode `self_chat` on the personal number: only the "Message yourself" chat reaches the agent.
3. **Access.** Owner only. One or more owner numbers; everyone else is dropped silently (counter plus log without content). No groups in v1. Nobody but the owner ever receives a message from AuraGo.
4. **Scope v1.** Telegram parity: text with WhatsApp formatting; voice notes in (transcription) and out (TTS as voice note); images and documents in; files out; notifications; typing indicator; read receipts.
5. **Licensing.** Compile whatsmeow into the main binary and accept GPL-3.0 for distributed binaries. Reason: whatsmeow is MPL-2.0, but its required dependency `go.mau.fi/libsignal` is GPL-3.0 (verified from its LICENSE file 2026-10-31). AuraGo's own source stays MIT; release binaries become GPL-3.0 combined works.
   - Rejected: separate GPL helper binary over stdio; `-tags whatsapp` build tag with a second release set.

## Research summary

### How other agents do it

| Project | Transport | Process model | Pairing | Access model | Notes |
| --- | --- | --- | --- | --- | --- |
| OpenClaw (TypeScript) | Baileys `7.0.0-rc14` only (no Cloud API) | In-process gateway plugin `@openclaw/whatsapp` | QR only (terminal or Control UI PNG data URL); phone pairing codes closed NOT_PLANNED in issue #81889 | `dmPolicy` pairing (default), allowlist, open, disabled; 8-char owner-approved pairing codes valid 1 h; group allowlist + `requireMention` | `selfChatMode`; multi-account; credentials as Baileys multi-file JSON, unencrypted |
| Pi (badlogic/pi-mono) | None. pi-mom is Slack-only; pi-chat covers Discord/Telegram | Third-party extensions (pi-harness-whatsapp, whatsapp-pi) use Baileys in-process | Extensions: QR via `/whatsapp` command | pi-harness-whatsapp: first sender approved in the terminal becomes owner; whatsapp-pi: contact/group allowlists | OpenClaw used to embed Pi's agent core; it now ships its own `@openclaw/agent-core` |
| nanobot (Python) | neonize (whatsmeow compiled as a shared library) | No sidecar; earlier Node Baileys bridge dropped | QR in terminal or via a WebUI callback | `allowFrom` + pairing codes; `group_policy` open or mention | Drops `IsFromMe` (no self-chat); learns LID→phone mappings |
| NanoClaw (Node) | Baileys rc.9 | In-process | QR or 8-char pairing code | Shared number: self-chat only; dedicated number: approval request for unknown senders, groups need an @mention | `ASSISTANT_HAS_OWN_NUMBER` switches shared vs dedicated number |
| Hermes Agent (Python) | Baileys; also an official Cloud API backend | Node sidecar on 127.0.0.1:3000 | QR via `hermes whatsapp` | `WHATSAPP_ALLOWED_USERS`; denies all when empty; group policy pairing/allowlist/open + `require_mention` | Bot mode or self-chat (reply prefix + recently-sent-ID cache against loops) |
| PicoClaw (Go) | whatsmeow behind build tag `whatsapp_native` (reviewer cited ~25 extra transitive deps), or external WebSocket bridge | Build-tag in-process, or external bridge | Terminal QR | `allow_from`; silent drop | modernc SQLite; reconnect backoff 5 s doubling to 5 min; no LID handling |
| GoClaw (Go, nextlevelbuilder) | whatsmeow in-process | In-process | QR sent as base64 PNG to a web UI wizard | `dm_policy` / `group_policy`: pairing, open, allowlist, disabled | Fails closed when the bot JID is unknown |
| ZeroClaw (Rust) | Cloud API or wa-rs Web mode | Not stated | Not stated | Allowlist defaults | `self_chat_mode` |

**Patterns.** The linked-device protocol dominates. Sidecars are being dropped (nanobot, GoClaw). Dedicated numbers are recommended because of ban risk. Access is deny-by-default. LID mapping and one-connection-per-session are the recurring operational problems.

### Transport options

| Option | Facts | Verdict |
| --- | --- | --- |
| whatsmeow | Very active (last commit 2026-10-30; no tags, pseudo-versions only). Requires Go ≥ 1.26 (AuraGo uses 1.27.3). Pure Go. Works with modernc SQLite through `sqlstore` dialect `sqlite` and DSN `_pragma=foreign_keys(1)` (inferred from source; verify in stage 1). QR via `GetQRChannel` (call before `Connect`) and `PairPhone`. Risks: unofficial (WhatsApp ToS forbid automated messaging); temp-ban reason codes 101–106 tied to bulk/spam behaviour; "client outdated" (405) outages in 2025-05, 2025-09, 2025-12 (twice) and 2026-06-09, each fixed by updating the dependency; primary phone must come online at least every 14 days; at most 4 linked devices. | Chosen |
| Official Cloud API (Meta) | Needs business portfolio and WABA; a number not used in the WhatsApp app; public HTTPS webhook with a valid certificate and `X-Hub-Signature-256` HMAC; paid templates for proactive messages outside the 24 h window. General-purpose AI assistants banned since 2026-01-15 outside regulatory carve-outs (EEA free of charge since 2026-05-13 per Meta's AI-provider pricing page; Italy, Brazil, Türkiye special terms). | Rejected for v1 |
| WhatsApp Agent Platform (beta) | `https://api.whatsapp.com/agent/v1`; long-poll; the agent can message only its creator; not end-to-end encrypted; 12 messages/min; limited countries. Only third-party code and docs found. | Candidate future backend |
| WAHA / Evolution API | Docker REST wrappers around whatsapp-web.js, Baileys or whatsmeow. | Extra sidecar with no benefit for a Go binary |

### Lessons and countermeasures

| Observed problem | Our countermeasure |
| --- | --- |
| OpenClaw #834: after a reconnect, pairing replies went to ~20 personal contacts | AuraGo never sends anything to non-owners; there are no pairing replies. |
| OpenClaw #119475: LID-only senders dropped silently (79 senders in 24 h) | Explicit LID→phone resolution; unresolved senders dropped with a WARN log and a status counter. |
| OpenClaw #9544, #19618, #67337: multi-file JSON credentials corrupted; 7,402 pre-key files | whatsmeow `sqlstore` in SQLite. |
| OpenClaw #45474, #55030: reconnect loops while status showed OK | Terminal states (logged_out, conflict, outdated, temp_banned) stop retrying and are shown in status. |
| OpenClaw #146999: a DNS failure became a permanent stop | Network errors always retry with backoff. |
| OpenClaw #46659: duplicated listener registry | Exactly one Bot and connection per process, owned by the server. |
| OpenClaw #144502: 48 kHz TTS voice notes did not play on phones | 16 kHz mono Opus; verified on a real phone during acceptance. |
| OpenClaw #33961: QR pairing failed on restart code 515 | Pairing covered by the acceptance test on a real device. |

- Pattern source: mautrix-whatsapp (AGPL; patterns only, no code copied). Treat StreamReplaced/ClientOutdated/TemporaryBan as hard errors; send presence `unavailable` after connect so the phone keeps notifications; report healthy only after `OfflineSyncCompleted`.

## Architecture

New package `internal/whatsapp`:

- `client.go`: `waClient` interface and whatsmeow adapter (Connect, Disconnect, IsConnected, GetQRChannel, PairPhone, Logout, SendMessage, Upload, Download, MarkRead, SendPresence, SendChatPresence, own JID, LID↔PN lookups via `Store.LIDs`).
- `store.go`: opens `<DataDir>/whatsapp/device.db` (directory 0700, file 0600) with `sql.Open("sqlite", "file:<path>?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")`, `sqlstore.NewWithDB(db, "sqlite", log)`, `Upgrade(ctx)`, `GetFirstDevice`. Linked-device name "AuraGo"; no full history sync requested.
- `runtime.go`: `Bot` with ctx, cancel and done; `MatchesConfig`, `CancelIfConfigChanged`, `Stop` (pattern `internal/telegram/runtime.go:94-118`).
- `events.go`: whatsmeow events to status states and the reconnect policy.
- `link.go`: link sessions (QR or phone code), one at a time, 3-minute TTL.
- `inbound.go`: filters, owner resolution, self-chat rule, dedupe, echo guard, offline cutoff.
- `pipeline.go`: hand-off to the agent, mirroring `processUpdate` in `internal/telegram/bot.go:294-736`.
- `broker.go`: `agent.FeedbackBroker` implementation mirroring `TelegramBroker` (`internal/telegram/bot.go:934-1025`).
- `format.go`, `chunks.go`: Markdown → WhatsApp conversion and 4000-rune chunking.
- `send.go`: serialized outbound queue, rate limit, owner-only recipient guard, sent-ID cache.
- `media.go`: downloads (50 MB cap), uploads, voice conversion.
- whatsmeow logger adapter to `slog`.

Server side:

- `internal/server/whatsapp_runtime.go`: configure and stop (pattern `internal/server/telegram_runtime.go:9-44`).
- `internal/server/whatsapp_handlers.go`.
- `internal/integrationstatus/whatsapp.go` (pattern `internal/integrationstatus/telegram.go`).

Reuse without refactoring Telegram:

- `commands.Handle`, `tools.TranscribeAudio`, `telegram.ConvertOggToMp3Context` (or a shared media helper), `tools.AnalyzeTrustedImageFileWithPromptContext`, `media.SaveAttachmentContext`.
- `telegram.OriginLabel` and the forwarded marker (Discord already does this).
- The ingress guard and `security.IsolateExternalData`; `agent.ExecuteAgentLoop`.

## Configuration

Section `whatsapp:`

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `enabled` | bool | `false` | Master switch; the Bot runs only when the conditions below hold |
| `mode` | string | `dedicated` | `dedicated` or `self_chat` |
| `owner_numbers` | []string | `[]` | E.164; required in `dedicated`; ignored in `self_chat` (owner = linked account) |
| `readonly` | bool | `false` | Like `telegram.readonly` (`internal/agent/dispatch_messaging.go:30`, `internal/tools/notification.go:104`, `internal/server/flows_handlers_options.go:121`): blocks `send_whatsapp`, notifications and Flows targets; conversational replies to the owner still work |
| `send_read_receipts` | bool | `true` | Dedicated mode only |
| `self_chat_prefix` | string | `🤖 ` | Prefix on agent messages in self-chat so they stand out |

- No vault secrets: the device store is the credential.
- Runs only when `enabled` is true, DataDir is set, a device is linked, and (`self_chat` or owner numbers are present).
- Config changes restart the Bot without relinking.
- The Config UI form is schema-driven (`internal/server/config_handlers_providers.go:219-268`); hot-reload wiring follows Telegram (`internal/server/config_handlers_main.go:1006-1016`, `1711-1716`).

## Linking and status

Endpoints (admin only; `Cache-Control: no-store`; declared in the route contract manifest):

- `GET /api/whatsapp/status`.
- `POST /api/whatsapp/link` `{method: "qr"|"phone", phone?, force?}`: 409 when already linked unless `force` (logs out first); one active link session.
- `GET /api/whatsapp/link` returns `{state: waiting|qr|code|success|timeout|error, qr_png, pairing_code, expires_at}`; the UI polls every 2 s.
- `POST /api/whatsapp/logout`: `Logout` (removes the linked device on the phone), then deletes the local device.
- `POST /api/whatsapp/test`: test message to the owner; 60 s rate limit like `/api/telegram/test`.

QR and pairing:

- QR codes rotate. The server renders a PNG data URI with `internal/qrcode`, not `/api/qr/` (Desktop-gated). The UI shows it like TOTP enrollment (`ui/cfg/auth.js:294-297`).
- Pairing code: `PairPhone(ctx, phone, true, whatsmeow.PairClientChrome, "Chrome (Linux)")` after `Connect` and the first QR event; shown as `XXXX-XXXX`.
- UI: a link panel inside the Config WhatsApp section with a state badge, masked linked number, buttons "Link via QR", "Link via phone number", "Unlink" and "Send test message", and a hint about the 14-day phone rule.

States:

| State | Trigger | Behaviour |
| --- | --- | --- |
| `disabled` | `enabled` is false | No Bot runs |
| `not_configured` | Enabled, but DataDir unset or owner numbers missing in `dedicated` | No Bot runs |
| `not_linked` | Enabled and configured; no device in the store | No Bot runs; the link panel offers linking |
| `linking` | Link session active (QR or phone code) | The link session owns a temporary client; waits for scan or code entry; 3-minute TTL; on `PairSuccess` the server starts the Bot |
| `connecting` | Device linked; connection being opened | Bot starts; not yet healthy |
| `healthy` | Connected and `OfflineSyncCompleted` | Messages flow in both directions |
| `degraded` | `Disconnected` / `KeepAliveTimeout`; auto-reconnect running | Not terminal; reconnect continues |
| `logged_out` | `LoggedOut` (terminal) | Stops retrying; user relinks |
| `conflict` | `StreamReplaced` (terminal) | Another client uses the same keys; stops retrying |
| `temp_banned` | `TemporaryBan` (terminal until expiry) | Stops until expiry; reason code and expiry shown in status |
| `outdated` | `ClientOutdated` (terminal) | Stops; user updates AuraGo |

- Status fields: `state`, `mode`, `linked_jid_masked`, `connected_since`, `last_inbound_at`, `last_outbound_at`, `dropped_non_owner`, `dropped_unresolved_lid`, `delivery_failures`, `last_error`, `temp_ban_expires_at`.
- Maintenance briefing hook like Telegram (`internal/agent/maintenance_briefing.go:164-165`).

## Inbound

Filter order. A drop increments a counter; the content of dropped messages is never logged.

1. Handle only `*events.Message`; ignore `*events.HistorySync`.
2. Drop `status@broadcast`, broadcast lists, newsletters and groups (v1).
3. Unwrap ephemeral and view-once wrappers; drop protocol messages, reactions, edits and poll updates.
4. Echo guard: drop IDs in the sent-ID cache (20 min, max 5000).
5. Dedupe by message ID (in-memory LRU).
6. Offline cutoff: drop messages whose timestamp is more than 10 minutes before the latest successful `Connected` event (so stale commands are not executed after a long outage).
7. `dedicated`: drop `IsFromMe`. Resolve the sender's phone number: a phone JID is used as is; else `SenderAlt`; else `Store.LIDs.GetPNForLID`. Unresolved: drop, WARN (masked), counter. Not an owner number: drop, counter. Remember the owner's LID.
8. `self_chat`: accept only `IsFromMe` messages in the account's own phone or LID chat; drop everything else before reading content.

Pipeline (mirrors Telegram):

- Session: `ResolveUserSession(ctx, "default")`, shared with web chat and Telegram (`internal/telegram/bot.go:309`).
- Register the run; pending question replies first (`bot.go:521-531`).
- Turn lock. Slash commands through `commands.Handle`, with operator commands allowed in owner DM.
- Voice or PTT: download → MP3 → `tools.TranscribeAudio`.
- Image: vision with the "[USER SENT AN IMAGE]" prefix plus caption.
- Document: `media.SaveAttachmentContext` with attachment note.
- Video and sticker: short placeholder text (not processed in v1).
- Location: `[Location: lat, lon (name)]`. vCard: fenced third-party data.
- Quoted reply: `[Reply to: …]` prefix (≤ 200 chars).
- Forwarded (`ContextInfo.IsForwarded`): third-party marker; skips HumanInput, pending-question replies and slash commands (as Telegram `bot.go:514-543`).
- Ingress guard blocks at ThreatHigh (like `shouldBlockTelegramPromptInjection`, `bot.go:796-809`), then `security.IsolateExternalData`.
- History insert; `MessageSource "whatsapp"`; HumanInput event ID `whatsapp:<chat>:<msgid>`.
- Read receipt after acceptance (dedicated mode and `send_read_receipts`).
- Typing presence `composing` every 8 s during the run; `paused` at the end.
- `agent.ExecuteAgentLoop(ctx, req, runCfg, false, broker)`.

## Outbound

- Final answer: strip thinking tags → `media.ExtractMarkdownImages` (images sent as image messages, caption ≤ 1024 chars) → Markdown → WhatsApp → `security.Scrub` → 4000-rune chunks → send.
- Conversion rules:
  - `**x**` and `__x__` → `*x*`; `*x*` and `_x_` → `_x_`; `~~x~~` → `~x~`.
  - Inline code and fenced code unchanged; never converted inside.
  - Headings → `*Heading*`; `[text](url)` → `text (url)`.
  - `- `, `1. ` and `> ` kept (native WhatsApp).
  - Tables → one `• a — b` line per row.
- Chunking prefers paragraph, then line boundaries; never splits inside a code fence without closing and reopening it.
- Plain text messages; no link previews.
- Broker parity with `TelegramBroker`:
  - `question_user` handled as in Telegram.
  - tool_start, error_recovery, api_retry and progress as short prefixed text, coalesced to at most one progress message per 10 s.
  - Budget warnings.
  - Audio and YouTube events captured and delivered after the run.
- Voice replies: in voice mode (`/voice`, as Telegram) TTS audio → ffmpeg libopus, mono, 16 kHz, voip → audio message with PTT and mimetype `audio/ogg; codecs=opus`. Otherwise a normal audio file.
- After connect: presence `unavailable`.
- Single serialized send queue per Bot; token bucket ≈ 20 messages/min, burst 5; every sent ID goes into the echo cache; delivery failures counted in status.
- Recipient guard: every send goes through one owner-recipient resolver. `dedicated`: owner numbers (whatsmeow resolves phone → LID; `ErrRecipientNotOnWhatsApp` surfaced). `self_chat`: own JID. No API accepts arbitrary numbers in v1.

## Agent tools and notifications

- New native tool `send_whatsapp` `{message, file_path?}`: owner only; available when WhatsApp is enabled, linked and not readonly; files sent as image, video, audio or document by MIME type; 50 MB cap.
- `send_notification`: new `ChannelWhatsApp` (`internal/tools/notification.go:25-30`; switch `98-128`; ChannelAll `138-160`), enum in `internal/agent/native_tools_integrations.go:1267`, and question source (`internal/tools/question_user.go:24-33`).
- Registration touch points:
  - `internal/agent/native_tools_integrations.go:1303-1313` (next to send_telegram).
  - `internal/agent/tooling_policy.go:368-369`.
  - `internal/agent/native_tools_registry.go:98` and `225-261`.
  - `internal/agent/dispatch_messaging.go:29-66`.
  - `internal/agent/tool_precheck.go:19-40`.
  - `internal/agent/action_policy.go:94`.
  - `internal/agent/action_policy_risk.go:354-356`.
  - `internal/audit/audit_manifests.go:62-64`.
  - `internal/agent/tool_categories.go:328-351`.
- Training catalog: reconcile the new tool with agotrain `tool_tiers.json` and `operation_contracts.json` per `AGENTS.md` and `documentation/training.md`.
- Agent identity: MessageSource label in `internal/prompts/builder.go:1061-1085`; reachable channels in `internal/agent/chat_channels.go:11-71`; review Telegram mentions in `internal/agent/emotion_behavior.go:116,125`, `operational_issues.go:439`, `task_router.go:95`, `tool_route.go:55` and `agent_loop_helpers.go:2297,2377`.

## Security

- The device store grants full account access: directory 0700, file 0600. It is never added to backups: backup collects an explicit SQLite list, and after a restore the user relinks. File tools and the Python sandbox deny `<DataDir>/whatsapp`, like the vault (locate the vault protection list in stage 1).
- Owner-only in both directions. Non-owners never get a reply. Logs mask phone numbers (last 4 digits) and never contain dropped content. Self-chat mode reads only the self chat.
- Forwarded and vCard content is marked third-party; the ingress guard applies. `MessagingIngressManifest` (`internal/audit/audit_manifests.go:479-497`) and its test (`internal/audit/audit_manifests_test.go:1654-1703`) are updated.
- WhatsApp is an owner channel like Telegram, so it is not added to `internal/security/untrusted_source.go` or `internal/agent/private_url_egress.go`.
- Security check (`internal/server/security_check.go:509-516`): warn when self-chat mode links a personal account, and when dedicated mode has no owner numbers.

## Licensing obligations

- Add whatsmeow (MPL-2.0), `go.mau.fi/libsignal` (GPL-3.0), `go.mau.fi/util` (MPL-2.0) and the other new transitive modules to `THIRD_PARTY_LIBRARIES.md`, `THIRD_PARTY_NOTICES.md` and `licences/UEBERSICHT.md`. Required license texts go to `licences/texts/` (verbatim SPDX texts).
- `LICENSE` stays the MIT text (`licences/AGENTS.md`). The README license section and `THIRD_PARTY_NOTICES.md` state clearly: the source code is MIT; distributed binaries contain GPL-3.0 code and are distributed under GPL-3.0 terms; release notes point to the tagged source.
- Release archives and the Docker image carry the GPL-3.0 text and the notices next to the binary.
- Before the first release with WhatsApp, run a license-compatibility check (e.g. go-licenses) over every module linked into the binary. A GPL-3.0-incompatible linked module blocks the release.
- Separately shipped resources (UI resource set, Docker images) are aggregates and unaffected.
- Keep the two README strings pinned by the README audit test.

## Reliability and operations

- whatsmeow auto-reconnect handles transient drops. Startup connect failures and network errors use backoff 5 s doubling to 5 min and never stop permanently.
- Terminal states wait for user action (relink, update, restart).
- Healthy only after `OfflineSyncCompleted`.
- Exactly one connection per process; the server owns the Bot behind a lifecycle mutex, like Telegram.
- whatsmeow has no tags: pin a pseudo-version. A "client outdated" outage requires an AuraGo release with an updated dependency.
- Operator docs: 14-day phone rule; max 4 linked devices; dedicated number recommended (ban risk); never bulk-send.

## Testing (lean)

- Fake `waClient`; no real WhatsApp in CI.
- Unit coverage:
  - Owner resolution: phone JID, LID via `SenderAlt`, LID via store, unresolved.
  - Self-chat filter and echo guard; offline cutoff; group, broadcast and newsletter drops; forwarded routing.
  - Markdown conversion and chunking, including code fences.
  - Event → state mapping, including terminal states; recipient guard.
  - Link sessions (QR, phone, timeout, force).
  - Runtime start, stop and hot reload (pattern `internal/server/telegram_runtime_test.go`).
  - Handlers, including route manifest; audit manifest.
  - Store opening with modernc and foreign keys on a real temp file.
- Manual acceptance on a real second number (the user scans the QR):
  - Link by QR and by code.
  - Text round trip; voice note in and out (plays on the phone); image and document in; file out; notification.
  - Unlink; restart without relink; self-chat mode.

## Delivery stages

1. **Foundation:** dependency and license docs, store, config, runtime, events/status, link endpoints and Config UI panel, test message.
2. **Text channel:** inbound filters, owner/LID/self-chat, pipeline, broker, formatting, chunking, typing and read receipts, send queue and recipient guard.
3. **Media and tools:** voice in/out, images, documents, placeholders, files out, `send_whatsapp`, `send_notification`, agent identity, training catalog.
4. **Finish:** `documentation/whatsapp_setup.md`, manuals `documentation/manuals/{en,de}/08-integrations.md`, 16 locales (config sections and help), security check, audit manifests, license check, acceptance on a real number.

## Out of scope / follow-ups

- Groups with mention gating.
- Multiple contacts and pairing codes for strangers.
- Official backends (Cloud API, Agent Platform) behind `waClient`.
- Flows actions.
- Reactions as status; outbound quoted replies; polls for question_user.
- Phone-liveness ping (mautrix pattern).
- Backfill of missed messages.
- Re-requesting expired media.
- A shared ingress package for Telegram and WhatsApp.

## Open risks

- Unofficial protocol: ban risk, ToS exposure, breakage that needs fast releases.
- LID-only senders after the 2026 username rollout may lack a phone mapping (drops visible in status).
- whatsmeow + modernc combination verified only in stage 1.
- GPL-3.0 obligations apply to every release.
- Voice-note format must be verified on real phones.
