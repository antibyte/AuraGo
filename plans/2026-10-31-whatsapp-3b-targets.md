# WhatsApp Stage 3b: Notification Targets Implementation Plan (Tasks 7–9)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Three existing notification targets reach the owner on WhatsApp:
- Camera person-detection alerts arrive with the annotated photo. `all` includes the WhatsApp photo.
- EasyDrag gets a curated `notify.whatsapp` node, a `whatsapp` notification-channel option and a `whatsapp` value for `notify_on_error`.
- The newspaper can deliver an edition to WhatsApp (allow switch, daily delivery, manual send, test send), with delivery records on the channel `whatsapp`.

**Master plan:** `plans/2026-10-31-whatsapp-integration-plan.md`, stage 3, Tasks 7–9 (the stage-3 table rows at lines 115–117 and the task text at lines 269–291). Task 12 (real phone) gets the 3b items below.
**Stage-3 plan:** `plans/2026-10-31-whatsapp-3-media-tools.md`. Its deviations 10, 11, 14, 36, 37 and 38 are the base of this plan.
**Design:** `plans/2026-10-31-whatsapp-integration-design.md`. It is final.
**Base:** worktree `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\whatsapp-stage2`, branch `claude/whatsapp-stage2`, HEAD around `c9cb9bb34`. Stage 3 Tasks 1–6 and 10 are done there.

All file:line references were re-verified against this worktree. The master plan's lines have moved where noted.

---

## Stage ground rules (on top of the master plan's standing rules)

The stage-3 ground rules apply unchanged: DOX, commits in the worktree, GitNexus from the worktree root, privacy, Go source, lean tests. On top of them:

- **DOX before editing.** Read these guides:
  - root `AGENTS.md`
  - `internal/whatsapp/AGENTS.md` ("Owner API", "Stage scope")
  - `internal/server/AGENTS.md` ("Newspaper integration" ~447, "EasyDrag flows" ~671, "WhatsApp Runtime Contract" ~917)
  - `internal/flows/AGENTS.md` ("Catalog" ~384, generic nodes ~486, invoker ~730, locales ~792)
  - `internal/persondetect/AGENTS.md` (~206–222)
  - `internal/newspaper/AGENTS.md` (~28)
  - `internal/tools/AGENTS.md`
  - `ui/AGENTS.md`
- **3b changes no Go file in `internal/whatsapp`.** Only its `AGENTS.md` changes. Everything goes through the stage-3 owner API, which runs `ownerSendAllowed` (live readonly, egg mode, generation match) and sends to owners only.
- **GitNexus.** The index is main, so the stage-3 symbols (`whatsAppOwnerSender`, `SendOwnerAttachment`, `WhatsAppAvailable`, and P0's new seam) come back UNKNOWN. Confirm them with `graft callers <symbol>` or a text search, and say so in the report.
  - Command: `node ../../.gitnexus/run.cjs impact "<symbol>" --direction upstream --repo ../..`
  - Before each commit, run `node ../../.gitnexus/run.cjs detect-changes --scope all --repo .`. A `partial` or `truncated` result means run it again.
- **Privacy:**
  - The owner API's errors are already masked (`safeOwnerError`). Log them as `"error", err` and never re-wrap them with raw data.
  - Never log a caption, an alert title or body, a photo size, an edition text or a destination string.
  - Fixtures use owner `+4917012345678` and second owner `+4917099998888`.
- **Go source.** No backslash-u escapes. Write `–` and umlauts literally. Use `"\x00"` for NUL. LF line endings, no BOM.
- **UI strings.** 3b adds 13 keys in 16 locales: 4 EasyDrag, 4 config/newspaper and 5 desktop/newspaper. They are listed with every translation under each task.
  - German uses "du" (lowercase, as the existing files do) and real umlauts.
  - The locale files are sorted by key. Insert each key at its sorted position.
  - docs-agent writes the JSON in the same batch, before the maker commits.
- **Lean tests.** No `t.Parallel` in tests that swap `serverWhatsAppOwner`, `tools.SetWhatsAppOwnerSender`, `personAlert*`, `flowSendNotification` or `whatsAppTestLast`.

---

## Deviations from the master plan

1. **`SendOwnerAttachment`, not `SendOwnerMessage`, for the photo and the PDF** (Tasks 7 and 9; stage-3 deviation 10). Both are bytes AuraGo made itself, so they cannot pass `OpenOutgoingAttachment`.
2. **A server-side seam (P0), not listed in the master table.** It lives in `internal/server/whatsapp_tool.go` and consists of:
   - `whatsAppOwnerAPI`: the subset of `*whatsapp.Bot` that server features use;
   - `serverWhatsAppOwner`: a test seam;
   - `(*Server).whatsAppOwnerReady(cfg)`: enabled, not read-only, and a linked generation runs.

   The `tools.WhatsAppOwnerSender` hook stays text/file-only, because `internal/tools` never needs attachment bytes.
3. **The generic node stays closed** (master Task 8 said "allowed in the generic tool node").
   - In `genericExcluded`, `send_whatsapp` moves from "closed until 3b" to "fully covered by curated nodes", next to `send_telegram`.
   - `TestGenericExclusionCoversCuratedTools` requires a curated tool to be excluded or to carry an allow reason.
   - A generic node would also bypass the curated node's limit, success check and result check.
   - The duplicate-text guard is keyed by tool, so it would protect a generic node too (deviation 7).
4. **`flows_catalog_env.go` is unchanged.** Stage 3 already mapped `flowToolConfigSection("send_whatsapp")` to `whatsapp` (`flows_catalog_env.go:227-228`).
5. **Node availability follows the configuration, not the link.**
   - `notify.whatsapp` is available when `agent.ConfiguredToolSchemas` lists `send_whatsapp`, which means `whatsapp.enabled && !readonly`.
   - An unlinked device fails at run time with the dispatcher's `needs_setup` (`runtime_dependency_unavailable`), which becomes `FLOW_NODE_UNAVAILABLE` and is not retried.
   - Why: the catalog and the node-type answers are cached per configuration snapshot (`flowCatalogEnv`, `flowNodeTypesCache`), so a link-state flag would freeze until the next config save.
   - The `notification_channels` option list, the camera readiness and the newspaper capability are computed per request, so they do check the link.
6. **`notify.whatsapp` has no `title` parameter.** The `send_whatsapp` schema is `{message, file_path}` (stage-3 deviation 14). The message limit is 4000 runes, `tools.WhatsAppMaxMessageRunes`, pinned by literal rows in the test.
7. **The invoker's `text_sent` guard is extended to `send_whatsapp`.** `SendWhatsAppMessage` sets `text_sent` when text went out first and the file failed, and when a long text partly went out (stage-3 deviation 40). The Telegram refusal text stays byte-identical.
8. **No `flowCallSecrets` case for `send_whatsapp`.** WhatsApp has no credential in the configuration; its keys live in `device.db`.
9. **Newspaper: the digest is the caption of the PDF**, one owner-API call. It goes first as its own message only when it exceeds the 1024-rune caption limit. Telegram sends the digest, then the document.
10. **Newspaper: the provider ID stays empty**, as for SMTP. The owner API returns no message IDs, and 3b keeps `internal/whatsapp` untouched. The stage-3 Task 6 note "may add a message ID" is not taken up.
11. **Newspaper ledger destination contains no full phone number.**
    - Dedicated mode uses `dedicated` plus the sorted last four digits of each valid owner number. Self-chat mode uses `self_chat`.
    - The ledger stores SHA-256(channel, destination), and a hash of a whole phone number can be reversed by enumeration.
    - The send itself uses the Bot's resolver (`ownerRecipients`), as the master plan requires.
12. **Newspaper test send goes through `SendOwnerMessage` with a fixed text**, not `Bot.SendTestMessage`, which has no readonly gate. It shares the one-per-minute limit of `/api/whatsapp/test` through the extracted `takeWhatsAppTestMinute`.
13. **Safe versus uncertain.**
    - `newspaper.SafeDelivery` covers only:
      - the server's own pre-checks;
      - the owner API's pre-send refusals (`ErrReadOnly`, `ErrMediaTooLarge`) while nothing of this delivery has gone out;
      - a cancellation before the first send.
    - Everything else is `uncertain` and is never replayed. Reason: with several owners, or a send that timed out, something may have arrived.
14. **No `internal/audit/audit_manifests.go` change** (master line 272, now 273).
    - The WhatsApp delivery opens no network client in `internal/server`; the Bot's traffic is classified under `internal/whatsapp/` (line 246).
    - The new sub-route `/api/desktop/newspaper/whatsapp/test` falls under the existing `/api/desktop/` route entry (line 169).
    - Verified by the audit sweep.
15. **`internal/persondetect/alert_rules.go` is unchanged.** The cooldown is per action and independent of the channel (`TestDecideFireCooldownIsPerAction`). 3b proves at the sink that a WhatsApp send whose photo and text both fail returns a failure, not a skip, so no cooldown starts.
16. **Camera readiness includes read-only** (the Discord pattern, `personAlertDiscordReady`). A read-only WhatsApp reports `skipped: channel unavailable`. Telegram's readiness ignores read-only; that is pre-existing and not changed here.
17. **Shared helpers extracted from Telegram code**, with Telegram output byte-identical:
    - from `sendNewspaperTelegram`: `newspaperDigest`, `newspaperPDFName` and `newspaperDigestRunes` (3000);
    - from `handleWhatsAppTest`: `takeWhatsAppTestMinute`.
18. **Delivery-history label fix.** `ui/js/desktop/apps/newspaper.js:81` labels every non-email record "Telegram". It now maps the channel name.
19. **Feature docs get one-line updates:**
    - `documentation/go2rtc.md:128,193`;
    - `documentation/manuals/{en,de}/07-configuration.md`, `11-missions.md` and `24-easydrag.md`.

    The operator docs stay in stage 4.

## Dispatch grouping

| Batch | Tasks | Agents | Depends on | Files (disjoint across B1–B3) |
| --- | --- | --- | --- | --- |
| P0 | shared seam | maker-agent | — | `internal/server/whatsapp_tool.go`, `whatsapp_tool_test.go` |
| B1 | 7 camera | maker-agent | P0 | `internal/config/go2rtc_detection{,_test}.go`, `internal/persondetect/alerts{,_test}.go`, `internal/server/person_detect_alerts{,_test}.go`, `internal/server/person_detection_test.go`, `ui/js/desktop/apps/network-cameras-detection-views.js`, `ui/desktop_network_cameras_detection_test.go`, `config_template.yaml` (line 1706 only) |
| B2 | 8 EasyDrag | maker-agent; docs-agent for 16 `ui/lang/easydrag` files | P0 | `internal/flows/{catalog_actions_notify,catalog_notify_args,catalog_generic}.go` and their tests (including `catalog_test.go` and `catalog_hardening_test.go`), `internal/server/flows_{handlers_options,bridge_notify,tool_invoker,tool_invoker_args,tool_invoker_status}.go` and their tests, `ui/js/desktop/apps/easydrag-{core,dialogs}.js`, `ui/lang/easydrag/<16>.json` |
| B3 | 9 newspaper | maker-agent; docs-agent for `ui/lang/config/newspaper/<16>` and `ui/lang/desktop/<16>` | P0 | `internal/config/newspaper.go`, `internal/newspaper/{service,types,store}.go`, `newspaper_test.go`, `internal/server/newspaper_{delivery,service,handlers}.go`, `whatsapp_handlers.go`, the new `newspaper_whatsapp_test.go`, `ui/cfg/newspaper.js`, `ui/js/desktop/apps/newspaper.js`, `ui/lang/config/newspaper/<16>.json`, `ui/lang/desktop/<16>.json`, `config_template.yaml` (after line 1278 only) |
| B4 | DOX and docs | docs-agent | B1–B3 | `AGENTS.md` (root), `internal/{whatsapp,server,flows,persondetect,newspaper}/AGENTS.md`, `documentation/go2rtc.md`, `documentation/manuals/{en,de}/{07-configuration,11-missions,24-easydrag}.md` |
| — | 12 | the user | all | real-phone checklist |

**Parallelism:**
- B1, B2 and B3 are independent and file-disjoint after P0.
- They cannot run at the same time in the one shared worktree:
  - all three compile `internal/server`, and B1 and B3 also compile `internal/config`;
  - one maker's half-edited file breaks the others' `go test`.
- Options:
  - **Sequential in the shared worktree (recommended):** P0, then B1, B2 and B3 in any order, then B4.
  - **Parallel:** one extra worktree per batch (`git worktree add ../wa3b-b1 <P0 commit>`, and the same for b2 and b3), each committed there and cherry-picked onto `claude/whatsapp-stage2`.
- **Shared files that need sequencing:**
  - `config_template.yaml`: B1 edits line 1706 and B3 inserts after 1278. The hunks are far apart, so cherry-picks merge cleanly, but in one worktree only one batch edits the file at a time.
  - `internal/audit/audit_manifests.go`: no change (deviation 14).
  - The `AGENTS.md` files go only to B4.

**Security review:**
- One combined security-agent review (Critical and Important only, no re-review round) over the P0–B3 diffs, after B3 is committed and before B4.
- B1 (camera photos leave the house) and B2 (flow data reaches the owner) require it.
- B3 is in the same pass: an edition is third-party web text, and the ledger stores a destination hash.
- The checklist per batch is under "Security review checklist" at the end.

## Open questions for the user

None. Every decision follows from the plans, the design and the Telegram code. The one judgement call, keeping the generic node closed (deviation 3), follows the existing curated-tool rule and its test.

---

## Task P0: shared server seam

**Files:** `internal/server/whatsapp_tool.go` and `whatsapp_tool_test.go` (add `aurago/internal/config`).

```go
// whatsAppOwnerAPI is the part of the running Bot that server features other than the agent
// tools send through: camera alert photos and newspaper editions. Every method passes the
// Bot's own gate (ownerSendAllowed: live readonly, egg mode, generation) and reaches the owners
// only.
type whatsAppOwnerAPI interface {
	OwnerSendReady() bool
	SendOwnerMessage(ctx context.Context, text, filePath string) (tools.WhatsAppSendResult, error)
	SendOwnerAttachment(ctx context.Context, caption string, att whatsapp.OwnerAttachment) (tools.WhatsAppSendResult, error)
}

var _ whatsAppOwnerAPI = (*whatsapp.Bot)(nil)

// serverWhatsAppOwner returns the running Bot, or nil (never a typed nil). Tests replace it.
var serverWhatsAppOwner = func(s *Server) whatsAppOwnerAPI {
	if bot := s.whatsAppBot.Load(); bot != nil {
		return bot
	}
	return nil
}

// whatsAppOwnerReady reports that a server feature may send to WhatsApp now: the section is
// enabled and writable and a linked generation runs. The owner API checks readonly again on
// the live configuration, so a toggle between this check and the send still wins.
func (s *Server) whatsAppOwnerReady(cfg *config.Config) bool {
	if cfg == nil || !cfg.WhatsApp.Enabled || cfg.WhatsApp.ReadOnly {
		return false
	}
	api := serverWhatsAppOwner(s)
	return api != nil && api.OwnerSendReady()
}
```

**Test fake** (`whatsapp_tool_test.go`, reused by B1 and B3): `fakeWhatsAppOwner`.

| Field | Meaning |
| --- | --- |
| `mu sync.Mutex` | guards the fields |
| `ready bool` | what `OwnerSendReady` returns |
| `attachErr error` | returned by `SendOwnerAttachment` |
| `attachTextSent bool` | returned in the attachment result |
| `msgErrs []error` | consumed one per `SendOwnerMessage` call; nil once used up |
| `attachments []string` | recorded as `name|caption|len` |
| `messages []string` | recorded message texts |

- `useFakeWhatsAppOwner(t, f)` swaps `serverWhatsAppOwner` (a nil `f` returns nil) and restores it in `t.Cleanup`.

**Tests:**
- `TestWhatsAppOwnerReadyNeedsEnabledWritableAndRunning`:
  - disabled → false;
  - enabled and read-only with a ready fake → false;
  - enabled with a fake that is not ready → false;
  - enabled, writable and ready → true;
  - with the production seam and no Bot, `serverWhatsAppOwner(s) == nil`, and the `== nil` check holds (no typed nil).

`CGO_ENABLED=0 go test ./internal/server/ -run '(TestWhatsAppOwner)' -count=1`

**GitNexus impact:** none. These are new symbols, and `whatsAppOwnerSender` is unchanged. Run `detect-changes`.
**Commit:** `feat(server): add the WhatsApp owner seam for server-side notification targets`

---

## Task 7: Camera person-detection alerts

**Files:**
- `internal/config/go2rtc_detection.go:35` and `go2rtc_detection_test.go:192`
- `internal/persondetect/alerts.go:83-86,219-222` and `alerts_test.go:220,231`
- `internal/server/person_detect_alerts.go:31-46,141-199,261-269`, `person_detect_alerts_test.go` and `person_detection_test.go:168`
- `ui/js/desktop/apps/network-cameras-detection-views.js:20-21` and `ui/desktop_network_cameras_detection_test.go`
- `config_template.yaml:1706`

**Allowlists** (all three name the same 9 channels):
- `go2RTCDetectionChannels = []string{"ntfy", "pushover", "telegram", "whatsapp", "discord", "push", "telnyx", "cyd", "all"}`. `ntfy` stays first and `all` last.
- `personAlertChannels` gains `"whatsapp": true`.
- JS: `CHANNELS = ['all', 'telegram', 'whatsapp', 'discord', 'ntfy', 'pushover', 'push', 'telnyx', 'cyd']`, and `CHANNEL_NAMES` gains `whatsapp: 'WhatsApp'`. It is a brand name, so no locale key.
- `persondetect/alerts.go:220`: `channel == "telegram" || channel == "discord" || channel == "whatsapp" || channel == "all"`. The `AlertSinks.Message` doc comment (`:83-85`) names whatsapp.
- `config_template.yaml:1706` comment: `# ntfy, pushover, telegram, whatsapp, discord, push, telnyx, cyd, all`.

**Server** (`person_detect_alerts.go`). The photo send with text fallback is the subtle part:

```go
// personAlertWhatsAppPhoto sends the alert photo to the WhatsApp owners; textSent says the caption
// went out as a message of its own first (a caption over 1024 runes), so a text fallback would
// repeat it. Tests replace serverWhatsAppOwner, not this function.
func (s *Server) personAlertWhatsAppPhoto(ctx context.Context, photo []byte, caption string) (textSent bool, err error) {
	api := serverWhatsAppOwner(s)
	if api == nil {
		return false, whatsapp.ErrNotConnected
	}
	res, err := api.SendOwnerAttachment(ctx, caption, whatsapp.OwnerAttachment{Name: personAlertPhotoName, Data: photo})
	return res.TextSent, err
}

// in deliverPersonAlert, after the telegram case:
case "whatsapp":
	if !s.whatsAppOwnerReady(cfg) {
		return persondetect.Skipped("channel unavailable")
	}
	if len(photo) > 0 {
		textSent, err := s.personAlertWhatsAppPhoto(ctx, photo, caption)
		switch {
		case err == nil:
			return nil
		case textSent:
			logger.Warn("Person detection photo could not be sent to WhatsApp; its text was sent", "error", err)
			return nil
		case ctx.Err() != nil:
			return ctx.Err()
		case errors.Is(err, whatsapp.ErrReadOnly), errors.Is(err, whatsapp.ErrNotConnected):
			return persondetect.Skipped("channel unavailable") // the text would be refused the same way
		}
		logger.Warn("Person detection photo could not be sent to WhatsApp; sending text", "error", err)
	}
	return personAlertOutcome(personAlertSendNotification(cfg, logger, "whatsapp", title, body, "high", nil))

// in the "all" case, after the Discord photo block (inside if len(photo) > 0):
if s.whatsAppOwnerReady(cfg) && ctx.Err() == nil {
	if _, perr := s.personAlertWhatsAppPhoto(ctx, photo, title); perr != nil {
		logger.Warn("Person detection photo could not be sent to WhatsApp", "error", perr)
	}
}
```

- The `all` text already reaches WhatsApp through `tools.SendNotification` "all" (stage-3 Task 6). Read-only gives an error entry there; parity with the bot.
- The `deliverPersonAlert` doc comment names WhatsApp next to Telegram and Discord. The `personAlertPhotoName` comment says "Telegram, Discord and WhatsApp".
- The slot key `"whatsapp"` gives WhatsApp its own four slots (`personAlertSlots`), with no code change.
- `persondetect.Skipped` does not start the cooldown, so the next event retries.

**Mirror:** the Telegram case (`person_detect_alerts.go:150-164`) and `personAlertDiscordReady` (`:267-269`) for read-only.

**Tests:**
- `persondetect`: `TestDispatchSendsAnnotatedPhotoOnlyToPhotoChannels` adds `"whatsapp"` to the channel list (`:220`) and to `wantPhoto` (`:231`).
- `config`: `TestGo2RTCDetectionChannelsReturnsACopy` expects `len(channels) != 9` (`:192`).
- `server/person_detection_test.go:168`: the loop adds `"whatsapp"`.
- `server/person_detect_alerts_test.go`, new tests (use `stubPersonAlertSenders`, `useFakeWhatsAppOwner`, and `personAlertTestServer()` with `Cfg.WhatsApp.Enabled = true`):
  - `TestPersonAlertWhatsAppSendsOnePhotoWithCaption`: a ready fake gives `attachments == ["person-detection.jpg|Title\nBody|7"]` and no text.
  - `TestPersonAlertWhatsAppPhotoFailureFallsBackToTextOnce`:
    - `attachErr = boom` → texts `["whatsapp|Title|Body|high"]`;
    - `attachErr = boom, attachTextSent = true` → nil and no text;
    - `attachErr = whatsapp.ErrReadOnly` → `SkipError` "channel unavailable" and no text;
    - photo fails and the text stub answers `{"status":"success","results":[{"channel":"whatsapp","status":"error","detail":"whatsapp is not linked or not running"}]}` → a non-skip error containing `whatsapp:`. That is a failed action, so no cooldown starts (the `alert_rules.go:8` contract).
  - `TestPersonAlertWhatsAppUnavailableIsSkipped`: disabled, read-only and a fake that is not ready each give `SkipError` "channel unavailable", and the fake records nothing.
  - `TestPersonAlertAllSendsTextAndPhotos` (extended): with a ready fake, `attachments == ["person-detection.jpg|Title|7"]` besides Telegram and Discord; read-only gives no WhatsApp photo.
  - `TestPersonAlertChannelsFollowTheConfigAllowlist`: the keys of `personAlertChannels` equal `config.Go2RTCDetectionChannels()` as a set.
- `ui`: `TestDesktopNetworkCamerasDetectionChannelsMatchTheConfig` reads `network-cameras-detection-views.js`, parses the `const CHANNELS = [...]` line, and compares it as a set with `config.Go2RTCDetectionChannels()`. `CHANNEL_NAMES` contains `whatsapp: 'WhatsApp'`.

Commands:
- `CGO_ENABLED=0 go test ./internal/config/ -run '(Go2RTCDetection)' -count=1`
- `CGO_ENABLED=0 go test ./internal/persondetect/ -run '(TestDispatch|TestDecideFire)' -count=1`
- `CGO_ENABLED=0 go test ./internal/server/ -run '(PersonAlert|PersonDetection|TestWhatsAppOwner)' -count=1`
- `go test -count=1 ./ui/ -run '(TestDesktopNetworkCamerasDetection)'`
- `node --check ui/js/desktop/apps/network-cameras-detection-views.js`

**GitNexus impact:**
- `deliverPersonAlert` and `sendPersonAlertMessage`: LOW expected.
- `(*Dispatcher).sendMessage`: LOW to MEDIUM.
- `Go2RTCDetectionChannels` and `validGo2RTCDetectionChannel` (via `ValidateGo2RTCStreamDetection`, which config load and the camera save path call): MEDIUM. Only an accepted value is added.

**Commit:** `feat(cameras): send person-detection alerts with their photo to WhatsApp`

---

## Task 8: EasyDrag flows

**Files:**
- `internal/flows/catalog_actions_notify.go:9-23`, `catalog_notify_args.go:21-67` (add after `discordMessage`, `:229-238`) and `catalog_generic.go:51-56`
- tests: `catalog_actions_notify_test.go`, `catalog_actions_notify_hardening_test.go:293-355,512`, `catalog_test.go:9-17,30` and `catalog_hardening_test.go:286`
- `internal/server/flows_tool_invoker.go:46-50,71-72,200-207`, `flows_tool_invoker_args.go:119-121,131-132,161-171` (comments only), `flows_tool_invoker_status.go:83-112,272`, `flows_handlers_options.go:70-73,118-132` and `flows_bridge_notify.go:28-29,76-77,95-111,138,159-166`
- server tests: `flows_tool_invoker_hardening_test.go`, `flows_tool_invoker_args_hardening_test.go`, `flows_handlers_catalog_hardening_test.go:485-517`, `flows_bridge_notify_hardening_test.go` and `flows_tool_invoker_provenance_test.go:59-103`
- `ui/js/desktop/apps/easydrag-core.js:32-34`, `easydrag-dialogs.js:228` and `ui/lang/easydrag/<16>.json`

**Node** (`catalog_actions_notify.go`):
- `TypeWhatsApp = "notify.whatsapp"`.
- The `registerNotifyNodes` list becomes `{telegramDef, whatsAppDef, emailDef, pushDef, discordDef}`.

```go
// whatsAppDef defines notify.whatsapp. send_whatsapp sends to the configured owners only: there
// is no recipient parameter and no title (the tool takes message and file_path). The file is a
// sensitive sink; the tool opens it only through tools.OpenOutgoingAttachment (workspace or
// documents folder, at most 50 MB). A text that does not fit a caption, and the text of an audio
// file, goes out first as a message of its own; when the file then fails, the answer carries
// "text_sent": true, which the flow tool invoker reports as denied and remembers for the run and
// node, so a retry never sends the text twice (as for notify.telegram).
func whatsAppDef(env CatalogEnv) *NodeDef {
	def := notifyDef(TypeWhatsApp, "brand-whatsapp", "send_whatsapp", env)
	def.PrimaryInput = "message"
	def.Params = []ParamSpec{
		{Name: "message", Kind: ParamTextarea, LabelKey: "easydrag.param.message", Required: true, Templatable: true, OutputIndependent: true},
		{Name: "file", Kind: ParamFile, LabelKey: "easydrag.param.attachment", Templatable: true, SensitiveSink: true},
	}
	// Validate: literalIssueIfSet(n, "message", whatsAppMessage) and literalIssueIfSet(n, "file", attachmentParam).
	// Execute: whatsAppMessage, attachmentParam, args {"message": msg} + setArg(args, "file_path", file),
	// callTool(ctx, in, "send_whatsapp", args), requireSuccess(out, "WhatsApp tool"),
	// notificationResults(out), then ExecResult{Output: {"sent": true}} — exactly telegramDef's shape.
	return def
}
```

**Limit** (`catalog_notify_args.go`):
- `whatsAppTextLimit = 4000`. The comment says: "one message of the Bot's sender, tools.WhatsAppMaxMessageRunes / internal/whatsapp maxChunkRunes; a longer text would go out as several paced messages".
- `whatsAppMessage(v)` calls `notifyMessage`, then `utf8.RuneCountInString(s) > whatsAppTextLimit` gives `tooLong("message", n, whatsAppTextLimit, "characters for WhatsApp")`.

**Generic** (`catalog_generic.go:51-56`):
- Delete the "closed until …" comment and entry.
- Add `"send_whatsapp": true` to the "fully covered by curated nodes" lines.

**Invoker guard** (`flows_tool_invoker.go:200-207`):

```go
// flowTextSentRefusals are the tools whose failed answer can say the text already went out
// ("text_sent": true; tools.SendTelegramFile and tools.SendWhatsAppMessage), with the refusal a
// later attempt of the same run and node gets.
var flowTextSentRefusals = map[string]string{
	"send_telegram": "the text of this Telegram message went out in an earlier attempt of the node " +
		"and its document failed; the node is not tried again, so the text is not sent twice",
	"send_whatsapp": "the text of this WhatsApp message went out in an earlier attempt of the node " +
		"and the rest failed; the node is not tried again, so the text is not sent twice",
}

sentKey := ""
if refusal, guarded := flowTextSentRefusals[req.Tool]; guarded {
	sentKey = flowTextSentKey(req)
	if i.textAlreadySent(sentKey) {
		return flowToolRefusal(agent.ToolResultDenied, refusal), nil
	}
}
```

- `flowTelegramTextSent` keeps its name (it parses a generic `text_sent`). Its doc comment and the comments at `:46-50`, `:71-72` and `flows_tool_invoker_args.go:119,131` name both tools.

**Refusal rules** (`flows_tool_invoker_status.go`). Each one cites its real message:
- Read-only rule (`:87`): `(?:home assistant|mqtt|discord|telnyx|whatsapp) is in read-only mode`. Real messages: `internal/agent/dispatch_messaging.go:54`, `internal/tools/whatsapp_send.go:27`, `internal/whatsapp/errors.go:20`.
- Not-enabled rule (`:98`): add `|whatsapp`. Real messages: `dispatch_messaging.go:51`, `whatsapp_send.go:26`.
- New after `:109`:
  - `flowRule(`whatsapp is not linked or not running`, agent.ToolResultNeedsSetup)` (`whatsapp_send.go:28`);
  - `flowRule(`whatsapp owner number is not registered on whatsapp`, agent.ToolResultNeedsSetup)` (`internal/whatsapp/errors.go:16`, wrapped at `send.go:241`).
- `flowNotifyRefusal`'s comment (`:272`) is unchanged in behaviour: `send_whatsapp` never answers success with failed channels.

**Options** (`flows_handlers_options.go`, after the Telegram line at `:121-123`):

```go
if s.whatsAppOwnerReady(cfg) {
	out = append(out, flowOption{Value: "whatsapp", Label: "WhatsApp"})
}
```

The doc comment (`:70-73`) adds: "WhatsApp when it is enabled, not read-only and a linked device runs".

**`notify_on_error`** (`flows_bridge_notify.go:111`). The flood and slot rules carry over with one change:

```go
remote := channel == "push" || channel == "telegram" || channel == "whatsapp"
```

- WhatsApp takes its own `flowNotifyMaxInFlight` slots (keyed `"whatsapp"`) before the flood rule. A suppressed notification gives its slot back.
- `sendFlowNotification` hands it to `tools.SendNotification(…, "whatsapp", …)`, which refuses read-only and bounds the send to 30 s (`whatsAppNotifyBudget`). The message (name 80 runes plus error 300 runes) stays under 4000.
- An unavailable WhatsApp logs SendNotification's Warn and does not fall back to the desktop, which is Telegram parity.
- Comments at `:28-29`, `:76-77`, `:96-104`, `:138-147` and `:159-166` say "push, Telegram or WhatsApp".

**UI:**
- `easydrag-core.js`, after `'brand-telegram'`, adds the Tabler (MIT) outline icon, inline with no external asset: `'brand-whatsapp': '<path d="M3 21l1.65 -3.8a9 9 0 1 1 3.4 2.9l-5.05 .9"/><path d="M9 10a.5 .5 0 0 0 1 0v-1a.5 .5 0 0 0 -1 0v1a5 5 0 0 0 5 5h1a.5 .5 0 0 0 0 -1h-1a.5 .5 0 0 0 0 1"/>'`.
- `easydrag-dialogs.js:228`: `['desktop', 'push', 'telegram', 'whatsapp', 'off']`.
- The built-in templates stay unchanged.

**UI strings** (`ui/lang/easydrag/<locale>.json`, 4 keys):
- `easydrag.node.notify_whatsapp.summary` = `{message}` in all 16.
- `easydrag.ui.notify_whatsapp` = `WhatsApp` in all 16.
- `easydrag.node.notify_whatsapp.label`:

  | Locale | Text |
  | --- | --- |
  | cs | Zpráva na WhatsApp |
  | da | WhatsApp-besked |
  | de | WhatsApp-Nachricht |
  | el | Μήνυμα WhatsApp |
  | en | WhatsApp message |
  | es | Mensaje de WhatsApp |
  | fr | Message WhatsApp |
  | hi | WhatsApp संदेश |
  | it | Messaggio WhatsApp |
  | ja | WhatsApp メッセージ |
  | nl | WhatsApp-bericht |
  | no | WhatsApp-melding |
  | pl | Wiadomość WhatsApp |
  | pt | Mensagem do WhatsApp |
  | sv | WhatsApp-meddelande |
  | zh | WhatsApp 消息 |

- `easydrag.node.notify_whatsapp.description`:

  | Locale | Text |
  | --- | --- |
  | cs | Pošle ti zprávu přes WhatsApp, volitelně se souborem. |
  | da | Sender dig en WhatsApp-besked, eventuelt med en fil. |
  | de | Sendet dir eine WhatsApp-Nachricht, auf Wunsch mit Datei. |
  | el | Σου στέλνει ένα μήνυμα WhatsApp, προαιρετικά με αρχείο. |
  | en | Sends you a WhatsApp message, optionally with a file. |
  | es | Te envía un mensaje de WhatsApp, con un archivo si quieres. |
  | fr | Vous envoie un message WhatsApp, avec un fichier si besoin. |
  | hi | आपको WhatsApp संदेश भेजता है, चाहें तो फ़ाइल के साथ। |
  | it | Ti invia un messaggio WhatsApp, se vuoi con un file. |
  | ja | あなたに WhatsApp メッセージを送信します。ファイルを添付することもできます。 |
  | nl | Stuurt je een WhatsApp-bericht, eventueel met een bestand. |
  | no | Sender deg en WhatsApp-melding, eventuelt med en fil. |
  | pl | Wysyła ci wiadomość WhatsApp, opcjonalnie z plikiem. |
  | pt | Envia para você uma mensagem do WhatsApp, com arquivo se quiser. |
  | sv | Skickar ett WhatsApp-meddelande till dig, om du vill med en fil. |
  | zh | 向你发送 WhatsApp 消息，可附带文件。 |

**Mirror:**
- `telegramDef` (`catalog_actions_notify.go:98-146`)
- the Telegram `text_sent` guard (`flows_tool_invoker.go:200-259`)
- the Telegram option line (`flows_handlers_options.go:121`)
- `remote` (`flows_bridge_notify.go:111`)

**Tests:**
- `flows`:
  - `TestWhatsAppNode`:
    - `{"message": "Hallo", "file": FileRef("/data/documents/ki.pdf", …)}` → args exactly `{"message": "Hallo", "file_path": "/data/documents/ki.pdf"}` (no `title`), `sent == true`, `def.Tool == "send_whatsapp"`, `def.Icon == "brand-whatsapp"` and `Effects == [sends_message]`;
    - a result entry `{"channel":"whatsapp","status":"error","detail":"x"}` → `FLOW_NOTIFY_FAILED` "x";
    - a blank message → `FLOW_PARAM_INVALID`.
  - Rows in existing tests:
    - `TestNotifySinkFlags`: `TypeWhatsApp: {"file"}`, the type in the effects loop, and a lint node `"wa"` warned on `file` only;
    - `TestNotifyLimitsAreTheDocumentedNumbers`: `whatsapp 4000` ok, `4001` fails, `whatsapp file 4096` ok, `4097` fails;
    - `TestRegisterCatalogCounts`: 38 ("14 triggers + 24 others");
    - `catalog_hardening_test.go:286`: `38+registered`;
    - `fullEnv()` adds `"send_whatsapp"`.
  - Must pass unchanged: `TestGenericExclusionCoversCuratedTools`, `TestCatalogTranslationsAreComplete`, `TestTemplatesAreValidInEveryLocale`.
- `server`:
  - `TestC13RefusalTextsBecomeNonRetryableStatuses`, new rows:
    - `Tool Output: {"status":"error","message":"WhatsApp is in read-only mode. Disable whatsapp.readonly to allow sending."}` → denied;
    - `{"status":"error","message":"whatsapp is in read-only mode","results":[{"channel":"whatsapp","status":"error","detail":"whatsapp is in read-only mode"}]}` → denied;
    - `Tool Output: {"status":"error","message":"WhatsApp is not enabled."}` → needs_setup;
    - `{"status":"error","message":"whatsapp is not linked or not running",…}` → needs_setup;
    - `{"status":"error","message":"whatsapp owner number is not registered on WhatsApp: no LID found for ***5678@s.whatsapp.net",…}` → needs_setup.
  - `TestC13NotificationChannelRefusals`, rows: a whatsapp `read-only` detail → denied, and `not linked or not running` → needs_setup.
  - `TestC13WhatsAppTextIsNotSentTwice`, a copy of `TestC13TelegramTextIsNotSentTwice` with `send_whatsapp`, `TypeWhatsApp` and the answer `{"status":"error","message":"…","results":[{"channel":"whatsapp",…}],"text_sent":true}`:
    - the first attempt is denied;
    - a second attempt in the same run is refused with the WhatsApp text and no dispatch (`len(*calls) == 1`);
    - a new run dispatches;
    - an answer without `text_sent` is retried normally.
  - `TestC19NotificationChannelOptions` (extended):
    - `accepted` gains `string(tools.ChannelWhatsApp)`;
    - with `Cfg.WhatsApp.Enabled` and a ready fake → `[all push telegram whatsapp ntfy pushover]`;
    - read-only or a fake that is not ready → no `whatsapp`.
  - `TestC15WhatsAppFailureNotificationHasItsOwnSlots`:
    - `c15BlockingSender` with four Telegram failures filling their slots, then a whatsapp failure still starts (`c15Started(t, started, "whatsapp", 1)`);
    - unblock, then `c15WaitIdle(t, e, "whatsapp")`;
    - the same flow fails again → `c15NoSend` (flood rule) and `c15Busy(e, "whatsapp") == 0`.
  - `TestFlowCopyRefusalDoesNotRecommendAnAllowRule`, extended over `send_whatsapp` with `cfg.WhatsApp.Enabled = true` and `tools.SetWhatsAppOwnerSender(readyFake)`, whose `SendOwnerMessage` calls `t.Fatal`:
    - no rule → `FLOW_ACTION_RULE_REQUIRED`;
    - copied text → `FLOW_TOOL_DENIED`.

    This proves flow data reaching WhatsApp passes the action policy and the copy gate.

Commands:
- `CGO_ENABLED=0 go test ./internal/flows/ -count=1`
- `CGO_ENABLED=0 go test ./internal/server/ -run '(TestC13|TestC15|TestC19|TestFlowCopyRefusal|TestFlowPDFCreate|TestFlowsAPICatalog|TestFlowOptions)' -count=1`
- `node --check ui/js/desktop/apps/easydrag-core.js && node --check ui/js/desktop/apps/easydrag-dialogs.js`
- `go test -count=1 ./ui/... -run '(Precision|Config|I18N)'`

**GitNexus impact:**
- **HIGH likely:** `(*flowToolInvoker).InvokeTool` and `flowToolOutcome`, because every flow tool call passes them. The report must state that the change adds a map lookup and three regex alternatives and changes no existing output (the Telegram refusal is byte-identical).
- MEDIUM: `registerNotifyNodes`/`RegisterCatalog`, `RefreshGenericTools`/`IsGenericToolExcluded`, `flowRefusalStatus`, `flowOptionList` and `notifyFlowFailure`.

**Commit:** `feat(flows): add the notify.whatsapp node, the WhatsApp channel option and failure notifications`

---

## Task 9: Newspaper delivery

**Files:**
- `internal/config/newspaper.go:36`
- `internal/newspaper/types.go:67`, `service.go:14-22,329-331,345`, `store.go:487` and `newspaper_test.go`
- `internal/server/newspaper_delivery.go:61-80,242-299`, `newspaper_service.go:35,48-58`, `newspaper_handlers.go:154,270-281` and `whatsapp_handlers.go:228-248`
- new `internal/server/newspaper_whatsapp_test.go`
- `ui/cfg/newspaper.js:7,13,145` and `ui/js/desktop/apps/newspaper.js:81,142,166,199,216`
- `ui/lang/config/newspaper/<16>.json` and `ui/lang/desktop/<16>.json`
- `config_template.yaml` (after `:1278`)

**Types and gates:**
- `NewspaperConfig.AllowWhatsApp bool `yaml:"allow_whatsapp" json:"allow_whatsapp"``, after `AllowTelegram`. Template: `  allow_whatsapp: false                                 # permit delivery to the linked WhatsApp owner chat`.
- `Profile.WhatsAppDaily bool `json:"whatsapp_daily"``, after `TelegramDaily`. The profile is a JSON body, so no migration.
- `Policy.WhatsApp bool`. In `newspaper_service.go:35`: `WhatsApp: cfg.Newspaper.AllowWhatsApp`.
- `Service.Deliver:345`: `|| channel == "whatsapp" && !policy.WhatsApp` gives `ErrDisabled`.
- `Store.ClaimDelivery:487`: `channel != "email" && channel != "telegram" && channel != "whatsapp"`. The record carries the channel `whatsapp`.
- `execute`, after `:331`: `if p.WhatsAppDaily && s.policy().WhatsApp { _, _ = s.Deliver(publicationCtx, e.ID, "whatsapp", "daily", "daily") }`.
- Destination (`newspaper_service.go:48-58`): `case "whatsapp": return newspaperWhatsAppDestination(cfg)`.
- `newspaperDelivery`, after the Telegram case:

  ```go
  case "whatsapp":
  	if !s.newspaperWhatsAppReady(cfg) {
  		return "", newspaper.SafeDelivery(newspaper.ErrDisabled)
  	}
  	return s.sendNewspaperWhatsApp(ctx, e)
  ```

The read-only gates sit at four layers:
1. `Service.canWrite` (newspaper or desktop read-only) plus `policy.WhatsApp`.
2. `newspaperDelivery` (`Newspaper.Enabled/ReadOnly`, then `newspaperWhatsAppReady`: allow switch, `WhatsApp.Enabled`, `!WhatsApp.ReadOnly`, a running generation).
3. The owner API's live `ownerSendAllowed`.
4. The UI's `readOnly()` and `whatsapp_ready`.

**Delivery** (`newspaper_delivery.go`). This is the subtle part:

```go
// newspaperDigestRunes bounds the digest both chat deliveries send first (one message).
const newspaperDigestRunes = 3000

// newspaperDigest is the digest text of an edition; newspaperPDFName the PDF's file name. Both
// are extracted unchanged from sendNewspaperTelegram, which now calls them.
func newspaperDigest(e newspaper.Edition) string
func newspaperPDFName(e newspaper.Edition) string // "newspaper-" + LocalDate + "-r" + Revision + ".pdf"

// newspaperWhatsAppReady reports that an edition can go to WhatsApp now.
func (s *Server) newspaperWhatsAppReady(cfg *config.Config) bool {
	return cfg != nil && cfg.Newspaper.AllowWhatsApp && s.whatsAppOwnerReady(cfg)
}

// sendNewspaperWhatsApp delivers an edition to the WhatsApp owners through the owner API: the
// PDF as a document with the digest as its caption (a digest over the caption limit goes first
// as its own message), or, when the edition has no safe PDF, the digest and then the full text
// in messages of at most 3000 runes. The owner API sends plain text without a link preview, so
// no URL a third-party page put into the edition is fetched. The provider ID stays empty.
func (s *Server) sendNewspaperWhatsApp(ctx context.Context, e newspaper.Edition) (string, error) {
	api := serverWhatsAppOwner(s)
	if api == nil || !api.OwnerSendReady() {
		return "", newspaper.SafeDelivery(errors.New("WhatsApp is not linked or not running"))
	}
	digest := newspaperDigest(e)
	if utf8.RuneCountInString(digest) > newspaperDigestRunes {
		return "", newspaper.SafeDelivery(errors.New("WhatsApp digest exceeds message limit"))
	}
	if data, pdfErr := newspaper.PDF(e); pdfErr == nil {
		_, err := api.SendOwnerAttachment(ctx, digest, whatsapp.OwnerAttachment{Name: newspaperPDFName(e), Data: data})
		return "", newspaperWhatsAppError(err, false)
	}
	chunks, err := newspaperTelegramChunks(newspaper.Text(e), newspaperDigestRunes)
	if err != nil {
		return "", newspaper.SafeDelivery(err)
	}
	sent := false
	for _, text := range append([]string{digest}, chunks...) {
		if err := ctx.Err(); err != nil {
			return "", newspaperWhatsAppError(err, sent)
		}
		if _, err := api.SendOwnerMessage(ctx, text, ""); err != nil {
			return "", newspaperWhatsAppError(err, sent)
		}
		sent = true
	}
	return "", nil
}

// newspaperWhatsAppError classifies a failure for the ledger. Before anything of this delivery
// went out, a cancellation and the owner API's pre-send refusals (read-only, a file over 50 MB)
// are safe to retry. Every other error leaves the outcome unknown: with several owners or a send
// that timed out, a message may have arrived, and the ledger never replays an uncertain send.
func newspaperWhatsAppError(err error, sentBefore bool) error {
	switch {
	case err == nil:
		return nil
	case !sentBefore && (errors.Is(err, whatsapp.ErrReadOnly) || errors.Is(err, whatsapp.ErrMediaTooLarge) ||
		errors.Is(err, context.Canceled)):
		return newspaper.SafeDelivery(err)
	}
	return fmt.Errorf("WhatsApp delivery outcome unknown: %w", err)
}

// newspaperWhatsAppDestination keys the ledger to the owners without a full phone number: the
// ledger keeps SHA-256(channel, destination), and a hash of a whole number can be reversed by
// trying every number. Dedicated mode: the sorted, de-duplicated last four digits of each valid
// owner number; self-chat mode: the linked account itself. The send does not use this value.
func newspaperWhatsAppDestination(cfg *config.Config) (string, error)
// self_chat → "self_chat"; dedicated → "dedicated\x00" + strings.Join(tails, ","); no valid
// owner (config.NormalizeE164) → errors.New("WhatsApp destination unavailable").
```

**Capability and test send:**
- `newspaper_handlers.go:154` map gains:
  - `"whatsapp_allowed": cfg.Newspaper.AllowWhatsApp`
  - `"whatsapp_ready": s.newspaperWhatsAppReady(cfg)`
- `whatsapp_handlers.go`: extract `takeWhatsAppTestMinute(now time.Time) bool` from `handleWhatsAppTest:240-248`, with unchanged behaviour. Comment: "one WhatsApp test send a minute, shared by /api/whatsapp/test and the newspaper's WhatsApp test".
- In `newspaper_delivery.go`:

  ```go
  const newspaperWhatsAppTestText = "AuraGo Newspaper: WhatsApp delivery test."
  var errNewspaperWhatsAppTestRateLimited = errors.New("wait a minute before sending another WhatsApp test")

  // sendNewspaperWhatsAppTest checks the gates before it claims the minute, so a refused request does
  // not use it up; the send passes the owner API's own read-only gate.
  func (s *Server) sendNewspaperWhatsAppTest(ctx context.Context, cfg *config.Config, now time.Time) error
  // gates: !Newspaper.Enabled || Newspaper.ReadOnly || VirtualDesktop.ReadOnly || !s.newspaperWhatsAppReady(cfg)
  //        → newspaper.ErrDisabled; then takeWhatsAppTestMinute → errNewspaperWhatsAppTestRateLimited;
  //        then serverWhatsAppOwner(s).SendOwnerMessage(ctx, newspaperWhatsAppTestText, "").
  ```

- Handler route after `telegram/test` (`:281`): `path == "whatsapp/test" && POST`, `ctx` with `whatsAppActionTimeout`.
  - The rate-limit error gives `newspaperJSON(w, 429, {"error": …})`.
  - Other errors go to `newspaperError` (masked text from the owner API).
  - Success gives `200 {"status":"sent"}`.

**UI:**
- `ui/cfg/newspaper.js`:
  - `:7` default `allow_whatsapp:false`;
  - `:13` toggles `[…, 'allow_telegram', 'allow_whatsapp']`;
  - `:145` summary adds `caps.whatsapp_ready ? t('config.newspaper.whatsapp_ready') : t('config.newspaper.whatsapp_pending')`.
- `ui/js/desktop/apps/newspaper.js`:
  - `:81`:
    - a `data-send="whatsapp"` button after Telegram (`readOnly()||!st.caps.whatsapp_ready` disables it; label `tr('send_whatsapp')`);
    - the setup hint condition adds `||!st.caps.whatsapp_ready`;
    - the history label becomes `({email:tr('email'),telegram:'Telegram',whatsapp:'WhatsApp'})[d.channel]||d.channel`.
  - `:142` and `:199`: `if(st.caps.whatsapp_ready)… whatsapp_daily=data.has('whatsapp_daily')`.
  - `:166`:
    - a `<div><h4>WhatsApp</h4>` channel block (`whatsapp_ready`/`whatsapp_unavailable` text, a `whatsapp_daily` checkbox disabled unless ready);
    - a `data-action="whatsapp-test"` button (`send_whatsapp_test`).
  - `:216`: `if(action==='whatsapp-test'){act(async()=>{await request('/whatsapp/test',{method:'POST',body:{}});st.notice=tr('test_sent');});return;}`.

**UI strings, `ui/lang/config/newspaper/<locale>.json`:**

| Locale | `config.newspaper.allow_whatsapp` | `config.newspaper.whatsapp_ready` | `config.newspaper.whatsapp_pending` |
| --- | --- | --- | --- |
| cs | Povolit doručení přes WhatsApp | WhatsApp připraven | WhatsApp není propojený nebo je jen pro čtení |
| da | Tillad levering via WhatsApp | WhatsApp klar | WhatsApp er ikke tilknyttet eller er skrivebeskyttet |
| de | WhatsApp-Versand erlauben | WhatsApp bereit | WhatsApp nicht verknüpft oder auf Nur lesen |
| el | Να επιτρέπεται η παράδοση μέσω WhatsApp | WhatsApp έτοιμο | Το WhatsApp δεν είναι συνδεδεμένο ή είναι μόνο για ανάγνωση |
| en | Allow WhatsApp delivery | WhatsApp ready | WhatsApp not linked or read-only |
| es | Permitir entrega por WhatsApp | WhatsApp listo | WhatsApp no vinculado o en solo lectura |
| fr | Autoriser l'envoi par WhatsApp | WhatsApp prêt | WhatsApp non connecté ou en lecture seule |
| hi | WhatsApp डिलीवरी की अनुमति दें | WhatsApp तैयार | WhatsApp लिंक नहीं है या केवल पढ़ने के लिए है |
| it | Consenti la consegna tramite WhatsApp | WhatsApp pronto | WhatsApp non collegato o in sola lettura |
| ja | WhatsApp 配信を許可する | WhatsApp 準備完了 | WhatsApp が未リンクか読み取り専用です |
| nl | Bezorging via WhatsApp toestaan | WhatsApp gereed | WhatsApp niet gekoppeld of alleen-lezen |
| no | Tillat levering via WhatsApp | WhatsApp klar | WhatsApp er ikke tilknyttet eller er skrivebeskyttet |
| pl | Zezwól na dostarczanie przez WhatsApp | WhatsApp gotowy | WhatsApp niepołączony lub tylko do odczytu |
| pt | Permitir entrega via WhatsApp | WhatsApp pronto | WhatsApp não conectado ou somente leitura |
| sv | Tillåt leverans via WhatsApp | WhatsApp redo | WhatsApp är inte länkat eller är skrivskyddat |
| zh | 允许通过 WhatsApp 发送 | WhatsApp 已就绪 | WhatsApp 未关联或处于只读模式 |

`config.newspaper.allow_whatsapp_help`:

| Locale | Text |
| --- | --- |
| cs | Povolí odesílání hotových vydání do tvého chatu na WhatsAppu. WhatsApp musí být propojený a nesmí být jen pro čtení. |
| da | Tillader afsendelse af færdige udgaver til din egen WhatsApp-chat. WhatsApp skal være tilknyttet og må ikke være skrivebeskyttet. |
| de | Erlaubt, fertige Ausgaben in deinen eigenen WhatsApp-Chat zu senden. Dafür muss WhatsApp verknüpft und nicht auf Nur lesen gestellt sein. |
| el | Επιτρέπει την αποστολή έτοιμων εκδόσεων στη δική σου συνομιλία WhatsApp. Το WhatsApp πρέπει να είναι συνδεδεμένο και όχι μόνο για ανάγνωση. |
| en | Allows sending finished editions to your own WhatsApp chat. WhatsApp must be linked and not read-only. |
| es | Permite enviar las ediciones terminadas a tu propio chat de WhatsApp. WhatsApp debe estar vinculado y no en solo lectura. |
| fr | Autorise l'envoi des éditions terminées dans votre propre discussion WhatsApp. WhatsApp doit être connecté et ne pas être en lecture seule. |
| hi | पूरे हो चुके संस्करणों को आपकी अपनी WhatsApp चैट में भेजने की अनुमति देता है। WhatsApp लिंक होना चाहिए और केवल पढ़ने के लिए नहीं होना चाहिए। |
| it | Consente l'invio delle edizioni completate nella tua chat WhatsApp. WhatsApp deve essere collegato e non in sola lettura. |
| ja | 完成した号を自分の WhatsApp チャットに送信できるようにします。WhatsApp がリンク済みで、読み取り専用でないことが必要です。 |
| nl | Staat toe dat afgeronde edities naar je eigen WhatsApp-chat worden verzonden. WhatsApp moet gekoppeld zijn en mag niet alleen-lezen zijn. |
| no | Tillater sending av ferdige utgaver til din egen WhatsApp-chat. WhatsApp må være tilknyttet og kan ikke være skrivebeskyttet. |
| pl | Zezwala na wysyłanie gotowych wydań do twojego czatu WhatsApp. WhatsApp musi być połączony i nie może być tylko do odczytu. |
| pt | Permite enviar edições concluídas para o seu próprio chat do WhatsApp. O WhatsApp precisa estar conectado e não pode estar em somente leitura. |
| sv | Tillåter att färdiga utgåvor skickas till din egen WhatsApp-chatt. WhatsApp måste vara länkat och får inte vara skrivskyddat. |
| zh | 允许将完成的各期报纸发送到你自己的 WhatsApp 聊天。WhatsApp 必须已关联且不处于只读模式。 |

**UI strings, `ui/lang/desktop/<locale>.json`:**

| Locale | `newspaper.send_whatsapp` | `newspaper.send_whatsapp_test` | `newspaper.whatsapp_daily` |
| --- | --- | --- | --- |
| cs | Odeslat na WhatsApp | Odeslat test na WhatsApp | Posílat každé denní vydání na WhatsApp |
| da | Send til WhatsApp | Send WhatsApp-test | Send hver dagsudgave til WhatsApp |
| de | An WhatsApp senden | WhatsApp-Test senden | Jede Tagesausgabe an WhatsApp senden |
| el | Αποστολή στο WhatsApp | Αποστολή δοκιμής WhatsApp | Αποστολή κάθε ημερήσιας έκδοσης στο WhatsApp |
| en | Send to WhatsApp | Send WhatsApp test | Send each daily issue to WhatsApp |
| es | Enviar a WhatsApp | Enviar prueba de WhatsApp | Enviar cada edición diaria a WhatsApp |
| fr | Envoyer sur WhatsApp | Envoyer un test WhatsApp | Envoyer chaque édition quotidienne sur WhatsApp |
| hi | WhatsApp पर भेजें | WhatsApp टेस्ट भेजें | हर दैनिक अंक WhatsApp पर भेजें |
| it | Invia su WhatsApp | Invia test WhatsApp | Invia ogni edizione quotidiana su WhatsApp |
| ja | WhatsApp に送信 | WhatsApp テストを送信 | 毎日の号を WhatsApp に送信 |
| nl | Naar WhatsApp sturen | WhatsApp-test sturen | Elke dagelijkse editie naar WhatsApp sturen |
| no | Send til WhatsApp | Send WhatsApp-test | Send hver dagsutgave til WhatsApp |
| pl | Wyślij na WhatsApp | Wyślij test WhatsApp | Wysyłaj każde wydanie dzienne na WhatsApp |
| pt | Enviar para o WhatsApp | Enviar teste do WhatsApp | Enviar cada edição diária para o WhatsApp |
| sv | Skicka till WhatsApp | Skicka WhatsApp-test | Skicka varje dagsutgåva till WhatsApp |
| zh | 发送到 WhatsApp | 发送 WhatsApp 测试 | 将每期日报发送到 WhatsApp |

| Locale | `newspaper.whatsapp_ready` | `newspaper.whatsapp_unavailable` |
| --- | --- | --- |
| cs | Tvůj propojený chat na WhatsAppu je připraven. | Nejdřív propoj WhatsApp v nastavení a vypni režim jen pro čtení. |
| da | Din tilknyttede WhatsApp-chat er klar. | Tilknyt først WhatsApp i indstillingerne, og slå skrivebeskyttelsen fra. |
| de | Dein verknüpfter WhatsApp-Chat ist bereit. | Verknüpfe zuerst WhatsApp in den Einstellungen und schalte Nur lesen aus. |
| el | Η συνδεδεμένη συνομιλία σου στο WhatsApp είναι έτοιμη. | Σύνδεσε πρώτα το WhatsApp στις ρυθμίσεις και απενεργοποίησε τη λειτουργία μόνο για ανάγνωση. |
| en | Your linked WhatsApp chat is ready. | Link WhatsApp in the settings first and switch off its read-only mode. |
| es | Tu chat de WhatsApp vinculado está listo. | Vincula primero WhatsApp en los ajustes y desactiva su modo de solo lectura. |
| fr | Votre discussion WhatsApp connectée est prête. | Connectez d'abord WhatsApp dans les paramètres et désactivez sa lecture seule. |
| hi | आपकी लिंक की गई WhatsApp चैट तैयार है। | पहले सेटिंग्स में WhatsApp लिंक करें और उसका केवल-पढ़ने वाला मोड बंद करें। |
| it | La tua chat WhatsApp collegata è pronta. | Collega prima WhatsApp nelle impostazioni e disattiva la sola lettura. |
| ja | リンク済みの WhatsApp チャットの準備ができています。 | 先に設定で WhatsApp をリンクし、読み取り専用をオフにしてください。 |
| nl | Je gekoppelde WhatsApp-chat is klaar. | Koppel eerst WhatsApp in de instellingen en zet alleen-lezen uit. |
| no | Den tilknyttede WhatsApp-chatten din er klar. | Koble først til WhatsApp i innstillingene og slå av skrivebeskyttelsen. |
| pl | Twój połączony czat WhatsApp jest gotowy. | Najpierw połącz WhatsApp w ustawieniach i wyłącz tryb tylko do odczytu. |
| pt | Seu chat do WhatsApp conectado está pronto. | Conecte primeiro o WhatsApp nas configurações e desative o somente leitura. |
| sv | Din länkade WhatsApp-chatt är redo. | Länka först WhatsApp i inställningarna och stäng av skrivskyddet. |
| zh | 你已关联的 WhatsApp 聊天已就绪。 | 请先在设置中关联 WhatsApp，并关闭其只读模式。 |

**Mirror:**
- the Telegram channel in `newspaperDelivery`, `sendNewspaperTelegram`, the Telegram destination, `telegram_ready` and `telegram/test`;
- the Telegram link-preview fixture (`newspaper_delivery_link_preview_test.go:56-103`) for the edition shapes ("Calm headline" gives a PDF; a Hebrew headline gives the text fallback).

**Tests:**
- `internal/newspaper` (`newspaper_test.go`):
  - `TestWhatsAppDeliveryFollowsItsPolicyAndRecordsTheChannel` (the pattern of `TestDeliveryFailureReceiptDistinguishesSafeAndUncertain`):
    - `Policy{Enabled: true}` → `Deliver(…, "whatsapp", "manual", key)` gives `ErrDisabled` and the deliver stub is not called;
    - `WhatsApp: true` → the stub got the channel `"whatsapp"`, and `Deliveries(e.ID)` holds one record with channel `whatsapp` and status `sent`;
    - `ClaimDelivery(…, "sms", …)` → "invalid delivery channel or kind".
  - `TestDailyEditionGoesToWhatsAppWhenAllowed`: `s.execute(ctx, ctx, run, p)` with `p.WhatsAppDaily = true`:
    - policy `WhatsApp: true` → exactly one stub call `("whatsapp", kind daily)`;
    - `WhatsApp: false` → none.
- `internal/server` (`newspaper_whatsapp_test.go`, using `useFakeWhatsAppOwner`):
  - `TestNewspaperWhatsAppSendsThePDFWithTheDigestAsCaption`: a ready fake and "Calm headline" → one attachment `newspaper-2026-10-28-r1.pdf|<newspaperDigest(e)>|n` with `n > 0`, no messages, provider ID `""`.
  - `TestNewspaperWhatsAppFallsBackToTextChunks`: a Hebrew edition → `messages[0] == newspaperDigest(e)`, `len(messages) >= 2`, every message ≤ 3000 runes, no attachment.
  - `TestNewspaperWhatsAppFailuresAreSafeOnlyBeforeSending` (check with `errors.As(&SafeDeliveryError)`):
    - not ready → safe;
    - `attachErr` `ErrReadOnly` or `ErrMediaTooLarge` (wrapped as the Bot wraps them) → safe;
    - `attachErr` boom → not safe, and the text contains "outcome unknown";
    - text path with `msgErrs = [nil, boom]` → not safe;
    - text path with `msgErrs = [whatsapp.ErrReadOnly]` → safe.
  - `TestNewspaperWhatsAppGatesAndDestination`:
    - `newspaperDelivery` with `AllowWhatsApp` false, with `WhatsApp.ReadOnly`, or with a fake that is not ready → safe `ErrDisabled`, and the fake records nothing;
    - destination, dedicated with `+4917012345678`, `+4917099998888` and `+4917012345678` → `"dedicated\x00" + "5678,8888"`, containing neither full number;
    - `self_chat` → `"self_chat"`;
    - dedicated without a valid owner → error.
  - `TestNewspaperWhatsAppTestSharesTheWhatsAppTestMinute` (reset `whatsAppTestLast` in Cleanup):
    - `newspaper.Enabled` with `AllowWhatsApp`, and a ready fake → the first call sends `newspaperWhatsAppTestText`;
    - a second call within a minute → `errNewspaperWhatsAppTestRateLimited`, no second message;
    - read-only → `ErrDisabled`, the minute is not claimed.
  - Must pass unchanged: `TestNewspaperTelegramTextMessagesDisableLinkPreviews`, `TestNewspaperTelegramChunksKeepCompleteText`, `TestWhatsAppHandlers*`.
- `ui`: `TestNewspaperTranslations` must pass unchanged; it covers the new desktop and config keys.

Commands:
- `CGO_ENABLED=0 go test ./internal/newspaper/ -count=1`
- `CGO_ENABLED=0 go test ./internal/config/ -run '(Newspaper)' -count=1`
- `CGO_ENABLED=0 go test ./internal/server/ -run '(TestNewspaper|TestWhatsAppHandlers|TestWhatsAppOwner)' -count=1`
- `node --check ui/cfg/newspaper.js && node --check ui/js/desktop/apps/newspaper.js`
- `go test -count=1 ./ui/... -run '(Precision|Config|I18N|TestNewspaperTranslations)'`

**GitNexus impact:**
- **HIGH likely:** `(*Service).execute` (every newspaper run) and `(*Server).handleNewspaper` (the whole newspaper API). The report states that each adds one branch or map entry and changes no existing output.
- MEDIUM: `(*Service).Deliver`, the `Profile` and `NewspaperConfig` types (JSON and YAML field addition) and `handleWhatsAppTest` (extraction).
- LOW: `(*Store).ClaimDelivery`, `(*Server).newspaperDelivery`, `sendNewspaperTelegram` (helper extraction; its test guards byte equality) and `(*Server).initNewspaper`.

**Commit:** `feat(newspaper): deliver editions to WhatsApp with daily, manual and test sends`

---

## Task B4: DOX and feature docs (docs-agent, after the review fixes)

- **`internal/whatsapp/AGENTS.md`:**
  - In "Owner API", replace "has no caller in stage 3" with: callers are the camera photo (`person_detect_alerts.go`) and the newspaper PDF (`newspaper_delivery.go`), both through `whatsAppOwnerAPI`.
  - In "Stage scope", 3b is done: `send_whatsapp` is excluded from generic flows because the curated `notify.whatsapp` covers it.
- **`internal/server/AGENTS.md`:**
  - WhatsApp Runtime Contract: the `whatsAppOwnerAPI`/`serverWhatsAppOwner` seam and `whatsAppOwnerReady` (enabled, writable, running; the owner API re-checks read-only).
  - Newspaper (~452), add:
    - WhatsApp reaches only the Bot's owners through the owner API;
    - the PDF goes with the digest as its caption, and the text fallback is chunked at 3000 runes;
    - text messages carry no link preview;
    - the ledger destination is the mode plus the last four digits;
    - only pre-send refusals are safe;
    - the test send shares the WhatsApp test minute.
  - EasyDrag (~682): `notify_on_error` includes `whatsapp` with its own slots. The options list WhatsApp when ready. The refusal rules and the `text_sent` guard cover `send_whatsapp`.
  - Verification lines: the new test names.
- **`internal/flows/AGENTS.md`:**
  - `:385-387` counts: 38 curated types, triggers 14, actions 16, notify 5. The current text (36, 13 triggers) is stale against the test.
  - `:732` `text_sent` sentence: names `send_whatsapp`.
  - `:486-491`: `send_whatsapp` among the curated tools.
- **`internal/persondetect/AGENTS.md:219-222`:** photos go to `telegram`, `discord`, `whatsapp` and `all`. A WhatsApp image caption carries no link preview.
- **`internal/newspaper/AGENTS.md:28`:** channels `email`, `telegram`, `whatsapp` (the `ClaimDelivery` allowlist).
- **Root `AGENTS.md:248`:** append "camera alert photos, EasyDrag `notify.whatsapp` and the failure channel, newspaper delivery". Before committing, check that `git ls-files -v AGENTS.md` prints `H`.
- **Docs:**
  - `documentation/go2rtc.md:128` ("Telegram, Discord and WhatsApp receive the cover frame") and `:193` (channel list);
  - `documentation/manuals/{en,de}/07-configuration.md` (`allow_whatsapp: false` next to every `allow_telegram`: en `:1071,1244,1338`, de `:565,738`);
  - `11-missions.md` (`notify_on_error` sentence: en `:252`, de `:233`);
  - `24-easydrag.md` (node list, `:3`). German uses "du".
- **Unchanged, with reasons:**
  - `internal/tools/AGENTS.md`: no tools change;
  - `ui/AGENTS.md`: no new UI contract;
  - `internal/security`, `qrcode`, `fileutil` and `licences` guides: not touched.
- **Commit:** `docs(whatsapp): document the stage-3b notification targets`. Stage child `AGENTS.md` files with `git add -f`.

---

## Task 12 additions: real-phone checklist (user)

- [ ] **Camera alert with photo:**
  - A camera on channel `whatsapp`, armed, a person walks in. An image message arrives with the drawn boxes and the caption "Person detected – <camera>" plus the detail line. The event shows `message: sent`.
  - Channel `all`: the text notification and a photo with the title caption arrive (Telegram as well, if configured).
  - `whatsapp.readonly: true`: the event shows `skipped: channel unavailable` and nothing arrives.
  - In self-chat mode, the caption carries the prefix.
- [ ] **Flow notification:**
  - A flow with `doc.pdf_create` → `notify.whatsapp` (message plus the PDF file): the text arrives as the caption of the document.
  - A flow with `notify_on_error: whatsapp` that fails: one failure message. A second failure within the hour: none.
  - With read-only on, the node fails as not available or denied and is not retried.
- [ ] **Newspaper edition:**
  - With `allow_whatsapp` on, "Send WhatsApp test" arrives. A second click within a minute reports the wait.
  - "Send to WhatsApp": the PDF arrives as a document with the digest as its caption. No link in the edition shows a preview card. The delivery history reads "WhatsApp · sent".
  - With `whatsapp_daily` on, the daily edition arrives at the ready time.
  - An edition in a script without PDF glyphs (Hebrew) arrives as the digest plus text messages.

---

## Security review checklist (security-agent, one pass over P0–B3)

- **P0/B1:**
  - The photo goes only through `SendOwnerAttachment` to owner recipients. It is sent only when the camera's channel is `whatsapp` or `all` and WhatsApp is enabled, writable and linked.
  - No temp file is written (unlike Discord).
  - Logs carry only the masked error: no caption, title or body, and no photo bytes or size.
  - A caption that went out as text first is not repeated.
  - `all` now sends the photo to WhatsApp (master plan decision, as for Telegram).
- **B2:**
  - `message` is OutputIndependent; `file` is a SensitiveSink (the lint warns on untrusted paths).
  - The file is opened only by `OpenOutgoingAttachment` inside `SendOwnerMessage`.
  - The action policy and the copy gate apply (`egressPolicyTool`), proven by the provenance test.
  - The refusal rules match AuraGo's own wording at the start of the message and never remote text (`flowTransportPrefixes` still win).
  - The `text_sent` guard covers `send_whatsapp`.
  - The generic node stays excluded.
  - There are no credentials to redact.
- **B3:**
  - Edition text is third-party: WhatsApp text is plain `Conversation`, with no preview fetched.
  - Owner-only recipients.
  - Read-only gates at four layers.
  - Safe versus uncertain is classified conservatively (no auto-replay).
  - The destination key has no full number.
  - The test send is rate-limited and gated before it claims the minute.
  - API errors are masked.

---

## Verification sweep (test-agent, after B4)

```bash
CGO_ENABLED=0 go build ./...
CGO_ENABLED=0 go vet ./internal/server/ ./internal/flows/ ./internal/newspaper/ ./internal/persondetect/ ./internal/config/
CGO_ENABLED=0 go test ./internal/flows/ ./internal/newspaper/ -count=1
CGO_ENABLED=0 go test ./internal/persondetect/ -run '(TestDispatch|TestDecideFire|TestAlert)' -count=1
CGO_ENABLED=0 go test ./internal/config/ -run '(Go2RTCDetection|Newspaper|WhatsApp)' -count=1
CGO_ENABLED=0 go test ./internal/server/ -run '(WhatsApp|PersonAlert|PersonDetection|TestC13|TestC15|TestC19|TestFlowCopyRefusal|TestFlowPDFCreate|TestFlowsAPICatalog|TestFlowOptions|TestNewspaper)' -count=1
CGO_ENABLED=0 go test ./internal/whatsapp/ ./internal/tools/ -run '(WhatsApp|SendOwner|Notification|Owner)' -count=1
CGO_ENABLED=0 go test ./internal/audit/ -run '(RouteContract|NetworkClient|WhatsApp|ToolPermissionMatrix)' -count=1
CGO_ENABLED=0 go test ./internal/telegram/ -count=1
go test -count=1 ./ui/... -run '(Precision|Config|I18N|TestNewspaperTranslations|TestDesktopNetworkCamerasDetection)'
node --check ui/js/desktop/apps/network-cameras-detection-views.js
node --check ui/js/desktop/apps/easydrag-core.js
node --check ui/js/desktop/apps/easydrag-dialogs.js
node --check ui/js/desktop/apps/newspaper.js
node --check ui/cfg/newspaper.js
```

- None of the changed JS files is in `scripts/build-ui-bundles.js`, so `npm run build:ui -- --check` is not needed.
- **Browser smoke:** none is required. If the test-agent has Chrome, it runs the existing `AURAGO_RUN_BROWSER_SMOKE=1 go test -count=1 ./ui -run 'TestDesktopNewspaperBrowser'` once, because it renders the changed delivery panel. Otherwise the user's server test (Task 12) covers it.
- Report the PASS counts. Then run `node ../../.gitnexus/run.cjs detect-changes --scope compare --base-ref main --repo .`; `partial` or `truncated` means run it again.

## Risks

- **Daily deliveries share one 30-second context** (`internal/newspaper/service.go:302`).
  - Email, then Telegram, then WhatsApp run inside it. A slow SMTP send can leave the WhatsApp record `uncertain`, which is never replayed.
  - This is pre-existing. Task 12 watches for it; the follow-up would be a context per channel.
- **Pacing under `all`.** Each alert sends text plus photo through the 20-per-minute bucket (burst 5). Several cameras at once can exceed the 30 s message timeout and be reported as `timeout`, as Telegram would under rate limits.
- **Formatting.** The Bot's Markdown converter runs over captions and edition text; a `*` or `_` in a headline may render as emphasis. This is cosmetic.
- **Link previews.** WhatsApp previews are made by the sender, and AuraGo sends plain `Conversation` text. Task 12 confirms that no preview card appears.
- **The GitNexus index is main.** P0 and the stage-3 symbols resolve as UNKNOWN; confirm callers with `graft callers` or a text search.
