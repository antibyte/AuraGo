# WhatsApp Stage 2 — Text Channel: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The owner talks to AuraGo over WhatsApp in text. Inbound messages pass the design's filter steps 1–8 and go to `agent.ExecuteAgentLoop` the way `telegram.processUpdate` hands them over. Replies use WhatsApp formatting, 4000-rune chunks and one rate-limited, owner-only send queue. Typing and read receipts work. The dashboard and the self-test show WhatsApp.
**Master plan:** `plans/2026-10-31-whatsapp-integration-plan.md`, Stage 2 tasks 1–11. **Design:** `plans/2026-10-31-whatsapp-integration-design.md`. Both are final. Where this plan departs from them, "Deviations" below says so and keeps their intent.
**Base:** worktree `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\whatsapp-stage2`, branch `claude/whatsapp-stage2`, at main `1fb47da49`. Stage-1 code and `internal/whatsapp/AGENTS.md` describe what actually landed.
**Pinned module:** `go.mau.fi/whatsmeow@v0.0.0-20261030195514-56e5ad0581ce`. Never move it in this stage. All whatsmeow citations below are relative to `C:\Users\Andi\go\pkg\mod\go.mau.fi\whatsmeow@v0.0.0-20261030195514-56e5ad0581ce\`.

---

## Stage ground rules (in addition to the master plan's standing rules)

- **DOX before editing:** read the root `AGENTS.md`, `internal/whatsapp/AGENTS.md`, `internal/server/AGENTS.md` (section "WhatsApp Runtime Contract", ~917), `internal/agent/AGENTS.md`, `internal/security/AGENTS.md`, `internal/tools/AGENTS.md` and `ui/AGENTS.md`.
- **Commits:** commit in the worktree with plain `git add <own paths>` and `git commit`; the temp-index rule applies only to the shared main checkout. Use `-m "<msg>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"`. Child `AGENTS.md` files are gitignored, so stage them with `git add -f`. Before committing the root `AGENTS.md`, check that `git ls-files -v AGENTS.md` prints `H` and not `h` (the assume-unchanged trap). Before each commit, run `git diff --cached --check` and scan the diff for real phone numbers and secrets.
- **GitNexus:** the worktree has no `.gitnexus/`. Run it from the worktree root as `node ../../.gitnexus/run.cjs impact "<symbol>" --direction upstream --repo ../..`. The index is main HEAD, which equals the worktree base. Before each commit, run `node ../../.gitnexus/run.cjs detect-changes --scope all --repo .`. If the result is `partial` or `truncated`, run it again. If the CLI cannot resolve the worktree, use `graft callers <symbol>` or a text search and say so in the report. Warn on HIGH or CRITICAL. Treat UNKNOWN as unresolved.
- **Privacy:** log errors only as `maskDigits(security.Scrub(err.Error()))` and JIDs only through `maskJID`. Never log message text, a dropped message, or anything from a stranger beyond the masked JID. Fixtures use fake numbers: owner `+4917012345678` (`4917012345678@s.whatsapp.net`), stranger `+4917099998888`, own account `testOwn` = `4917099990000`, LIDs such as `111122223333444@lid`.
- **Go source:** never write a backslash-u escape; use literal characters or `\x5c`. Use LF line endings and no BOM. Status prefixes that Telegram writes as emoji are copied from `internal/telegram/bot.go:983-998` verbatim.
- **Lean tests:** use the fake `waClient` only. Pipeline tests use `memory.NewSQLiteMemory(":memory:", logger)`, as the Telegram tests do. No mutation, property or stress suites. One combined review per batch, Critical and Important findings only, no re-review rounds.

---

## Verified whatsmeow behaviour (from source; there was no real-device acceptance)

### Q1 — `Connected` vs offline sync, and when offline messages arrive

- `handleConnectSuccess` handles the `success` node on the handler queue (node map `client.go:317,325,329`). Before anything else it runs `cli.LastSuccessfulConnect = time.Now()` (`connectionevents.go:164`) and updates `Store.LID` (`connectionevents.go:171-185`). It then starts a goroutine. That goroutine uploads prekeys, sends `SetPassive(false)` (`connectionevents.go:200`) and only then dispatches `events.Connected` (`connectionevents.go:204`).
- `handleIB` dispatches `OfflineSyncPreview` (`offline_preview`, `connectionevents.go:79-86`) and `OfflineSyncCompleted` (`offline`, `connectionevents.go:87-90`) synchronously, from the handler queue. `OfflineSyncCompleted` is "emitted after the server has finished sending missed events" (`types/events/events.go:583`).
- The handler queue processes one node at a time. Each node's goroutine must close `doneChan` before the next node starts (`client.go:912-971`), and frames are queued in arrival order (`client.go:873-906`). Messages (`"message"` → `handleEncryptedMessage`) use the same queue. The order Preview → missed messages → Completed is therefore strict. `LastSuccessfulConnect` is written before any later node of that connection is handled, and the `doneChan` hand-off gives a happens-before edge, so a message handler can read it race-free.
- `Connected` comes from a separate goroutine. Nothing orders it against message dispatch, so offline messages can be handled before, during or after `Connected`. Stage 1 already handles `OfflineSyncCompleted` arriving before `Connected` (`internal/whatsapp/events.go:36-41`). `Disconnected` is also dispatched from a goroutine (`client.go:621`).
- **Design consequence:** the offline-cutoff reference is `Client.LastSuccessfulConnect`, exposed as a new `waClient.LastSuccessfulConnect()`. The Bot's observation of `Connected` is not used. The cutoff applies only while the current connection has not finished its offline sync (`!b.synced`). Messages after `OfflineSyncCompleted` are live by definition and are never compared against the local clock. Without that gate, a local clock more than 10 minutes ahead of the server (message timestamps are server time, `message.go:234`) would drop every live message.

### Q2 — `MessageInfo` / `MessageSource`

- Fields: `types/message.go:27-41` (`Chat`, `Sender`, `IsFromMe`, `IsGroup`, `AddressingMode`, `SenderAlt`, `RecipientAlt`, `BroadcastListOwner`, `BroadcastRecipients`) and `types/message.go:99-118` (`ID`, `ServerID`, `Type`, `PushName`, `Timestamp`, `Category`, `Edit`, `DeviceSentMeta`, …). `parseMessageInfo` reads `ID` from attr `id`, `Timestamp` from `t` (unix seconds, server time) and `Edit` from `edit` (`message.go:224-241`). The `EditAttribute` values (edit "1", pin "2", revoke "7"/"8") are at `types/message.go:56-65`.
- `parseMessageSource` (`message.go:88-191`):
  - **Group or broadcast** (`from.Server` is `g.us` or `broadcast`, which includes `status@broadcast`): `IsGroup=true`, `Chat=from`, `Sender=participant`, `SenderAlt=participant_pn` or else `participant_lid` (`98-109`). `IsFromMe` is set when the participant user matches the own PN or own LID user (`110-112`). For broadcast lists, `BroadcastListOwner` comes from the `recipient` attr (`113-114`).
  - **Newsletter:** `Chat=Sender=from`, and `IsFromMe` is never set (`139-142`).
  - **Own account** (`from.User` equals the own PN user or own LID user): `IsFromMe=true`, `Sender=from`, `Chat` = the `recipient` attr or else `from.ToNonAD()` (`143-161`). This is how a self-chat message arrives.
  - **Other DM:** `Chat=from.ToNonAD()`, `Sender=from`. `SenderAlt` is `sender_pn` when the sender is a LID, and `sender_lid` when the sender is a PN (`172-185`). The hosted servers are normalized to `s.whatsapp.net` and `lid` (`173-177`).
  - `SenderAlt` takes the sender's device (`186-188`).
  - `AddressingMode` (`97`) matters only for group `SenderAlt`; AuraGo ignores it.
- **`SenderAlt` can be a LID even when `Sender` is a PN.** For a PN sender without a LID alt, `decryptMessages` fills `info.SenderAlt` from `Store.LIDs.GetLIDForPN` (`message.go:339-353`). **Consequence:** the filter uses `SenderAlt` only when its server is `s.whatsapp.net`.
- When `SenderAlt` or `RecipientAlt` is set, `handleEncryptedMessage` stores the LID↔PN mapping (`message.go:51-55`). A later `GetPNForLID` therefore finds it.

### Q3 — Own LID and LID→PN lookup

- `store.Device.LID types.JID` (`store/store.go:229`) and `Device.GetLID()` (`store/store.go:270-275`). The own PN is `Device.GetJID()` (`259-268`), which stage 1 already uses. `handleConnectSuccess` sets or corrects the LID and stores the own LID→PN mapping (`connectionevents.go:171-185`).
- `Store.LIDs` is a `store.LIDStore` (`store/store.go:255`) with `GetPNForLID(ctx context.Context, lid types.JID) (types.JID, error)` (`store/store.go:190`). The sqlstore implementation returns `types.JID{}` and a nil error when no mapping exists (`store/sqlstore/lidmap.go:91-116`). It returns an error for a non-LID input (`lidmap.go:128-131`), and its result copies the input's `Device` (`lidmap.go:100,113`). **Consequence:** call it with `lid.ToNonAD()` only, and treat an empty result as unresolved.

### Q4 — Send-side signatures

- `func (cli *Client) MarkRead(ctx context.Context, ids []types.MessageID, timestamp time.Time, chat, sender types.JID, receiptTypeExtra ...types.ReceiptType) error` (`receipt.go:197`). DMs (chat server `s.whatsapp.net` or `lid`) get no participant attr (`223-225`). When the account's privacy setting turns read receipts off, the receipt becomes read-self (`216-221`).
- `func (cli *Client) SendChatPresence(ctx context.Context, jid types.JID, state types.ChatPresence, media types.ChatPresenceMedia) error` (`presence.go:130-144`). Constants: `types.ChatPresenceComposing = "composing"`, `types.ChatPresencePaused = "paused"`, `types.ChatPresenceMediaText = ""`, `types.ChatPresenceMediaAudio = "audio"` (`types/presence.go:20-31`).
- `func (cli *Client) SendMessage(ctx context.Context, to types.JID, message *waE2E.Message, extra ...SendRequestExtra) (resp SendResponse, err error)` (`send.go:191`).
  - `SendResponse{Timestamp, ID, ServerID, DebugTimings, Sender, Chat}` (`send.go:114-133`). `resp.ID` is set before sending (`227`).
  - Device JIDs are refused (`203-206`).
  - A PN destination is swapped for its LID. With no LID, it returns `fmt.Errorf("%w: no LID found for %s from server", ErrRecipientNotOnWhatsApp, to)` (`send.go:341-365`; sentinel at `errors.go:115`).
- Text messages are built as `&waE2E.Message{Conversation: proto.String(text)}`, as the `SendMessage` doc shows (`send.go:176-180`) and stage 1 already does (`runtime.go:421`). `ExtendedTextMessage` is needed only for `ContextInfo` (quotes, forward flags, mentions) or preview fields. A `Conversation` message carries no link preview, which meets "plain text, no link previews".

### Q5 — Unwrapping and dropping

- Every decrypted message, protocol messages included, is dispatched as `*events.Message` after `UnwrapRaw()` (`message.go:1172-1179`). Newsletter plaintext takes the same path (`message.go:307`). Protocol parts are processed internally first (`message.go:884-962`) and are still dispatched.
- `UnwrapRaw` (`types/events/events.go:376-425`) peels these wrappers: DeviceSent, BotInvoke, Ephemeral, ViewOnce v1/v2/v2-extension, LottieSticker, DocumentWithCaption and EditedMessage. It sets `IsEphemeral`, `IsViewOnce`, `IsViewOnceV2`, `IsViewOnceV2Extension`, `IsDocumentWithCaption`, `IsLottieSticker`, `IsBotInvoke` and `IsEdit` (`events.go:322-347`). For an edit, the inner message is a `ProtocolMessage`, and `IsEdit` is true.
- Getters (`proto/waE2E/WAWebProtobufsE2E.pb.go`):
  - Content: `GetConversation` 11380, `GetExtendedTextMessage` 11415 (with `ExtendedTextMessage.GetText` 8967 and `GetContextInfo` 9030).
  - Drop checks: `GetProtocolMessage` 11457, `GetReactionMessage` 11639, `GetEncReactionMessage` 11702, `GetEditedMessage` 11709.
  - Polls: `GetPollCreationMessage` 11660, `…V2` 11723, `…V3` 11751, `…V4` 11912 (`*FutureProofMessage`), `…V5` 12024, `…V6` 12059, `GetPollUpdateMessage` 11667, `GetPollResultSnapshotMessage` 11884, `…V3` 12038.
  - Forwards: `ContextInfo.GetIsForwarded` 9976, `GetForwardingScore` 9969, `GetForwardedNewsletterMessageInfo` 10109 (with `.GetNewsletterName` 23053).
- **Consequence:** the filter does not unwrap again. It reads `evt.Message` and the `Is*` flags. Only a hand-built event (`Message == nil && RawMessage != nil`) gets `evt.UnwrapRaw()`. Test fixtures build events as `(&events.Message{Info: …, RawMessage: wrapped}).UnwrapRaw()`, exactly like whatsmeow.

### Q6 — Special JIDs

- Servers (`types/jid.go:22-33`): `DefaultUserServer="s.whatsapp.net"`, `GroupServer="g.us"`, `BroadcastServer="broadcast"`, `HiddenUserServer="lid"`, `NewsletterServer="newsletter"`, `HostedServer`, `HostedLIDServer`, `BotServer="bot"`.
- `StatusBroadcastJID = NewJID("status", BroadcastServer)` (`jid.go:42`). `JID.IsBroadcastList()` excludes status (`jid.go:122-124`). `IsBot()` is at `jid.go:128-130`.

### Handler constraints (they shape Task 8)

- `dispatchEvent` calls the handlers synchronously, in registration order, and recovers from panics (`client.go:999-1014`).
- A node that takes longer than 5 s logs a warning. A node still running after 10 × 30 s continues in the background, and ordering is lost (`client.go:941-966`).
- The filter is therefore cheap and runs inline. Every agent turn runs on a worker goroutine.

---

## Deviations from the master plan

1. **Offline cutoff reference** (Task 8). The cutoff is `client.LastSuccessfulConnect() − 10 min`. It applies only while the connection's offline sync is still running. Reason: Q1. The intent, "stale commands are not executed after a long outage", is unchanged.
2. **`waClient` gets five methods, not four** (Task 1). `LastSuccessfulConnect() time.Time` joins `MarkRead`, `SendChatPresence`, `OwnLID` and `PNForLID`.
3. **No second unwrap** (Task 3). whatsmeow already unwraps (Q5). The ephemeral and view-once test proves the filter accepts the unwrapped content.
4. **`SenderAlt` counts only when it is a phone JID** (Task 3). Reason: Q2.
5. **`tools.QuestionSourceWhatsApp` moves from stage 3 Task 6 to stage 2 Task 7.** Pending-question replies need a source; `internal/agent/AGENTS.md:714` requires every `CompleteQuestion` caller to set one. Like Telegram, it is untrusted, so `questionAnswerFromAdminSurface` is unchanged. Stage 3 Task 6 keeps `ChannelWhatsApp` and the notification enum.
6. **Self-test inventory needs no edit** (Task 9). `selfTestIntegrations` already lists `integration:whatsapp` by reflection over `WhatsApp.Enabled` and the yaml tag `whatsapp` (`internal/server/self_test_handlers.go:56-72`, `internal/config/config_types.go:1481`). Only the probe map in `self_test_probes.go:19-24` changes. `self_test_handlers.go` stays untouched; a test proves the inventory entry.
7. **`ErrRecipientNotOnWhatsApp` becomes user-visible** (Task 5 defines it, Task 9 shows it). The new sentinel in `errors.go` wraps whatsmeow's. Task 9 maps it to HTTP 409 `recipient_not_on_whatsapp` and adds one Config-panel string. That adds `internal/server/whatsapp_handlers.go`, `ui/cfg/whatsapp.js` and `ui/lang/config/whatsapp/<16>.json`, which are not in the master plan's table.
8. **The self-chat prefix moves into the sender** (Task 5). Every outgoing text in `self_chat` mode gets `self_chat_prefix` in one place, once per chunk. `testMessageText` loses its own prefix logic. The test message's bytes stay identical, and the existing `TestBotSendTestMessageReachesOnlyOwners` stays green unchanged.
9. **Drop counters** (Task 3). The design names `dropped_non_owner` and `dropped_unresolved_lid`; those keep their fields. Every other drop ("a drop increments a counter") goes into a new `dropped_filtered` map keyed by fixed reason codes.
10. **The dedupe cache records accepted IDs only** (Task 3). Strangers' traffic in `self_chat` mode then cannot evict owner IDs. Rejected duplicates are rejected again anyway.
11. **Stage-2 text scope** (Tasks 6 and 7):
    - The broker ignores `audio` and `youtube_video` events; stage 3 Task 3 adds them.
    - Markdown images in answers become `[Image: <caption>]` lines until stage 3 sends them as image messages.
    - Inbound text comes only from `Conversation` and `ExtendedTextMessage.Text`. Media-only messages pass the filter but start no turn, logged at Info with their kind only. Stage 3 Task 2 adds them.
12. **`Start` takes an `AgentDeps` struct** (Task 8), not Telegram's 20 positional parameters. `KG` is left out: Telegram passes it but sets `RunConfig.KG: nil` (`internal/telegram/bot.go:600`).
13. **The "fake inbound reaches the pipeline" test lives in `internal/whatsapp`** (Task 8). The server cannot inject a fake client. The server test checks only that `whatsAppAgentDeps()` carries the collaborators.
14. **DeviceSent self-chat destination** (Task 3). An own-device copy whose `DestinationJID` is not the own account is dropped as `not_self_chat`, even when `Chat` is the own chat, because whatsmeow uses the own account as `Chat` for a stanza that names no recipient.
15. **`empty_id` and bot senders** (Task 3). A message without an ID is dropped as `empty_id`, because neither echo nor duplicate can be judged for it. A bot sender is dropped as `unsupported_sender` before owner resolution.
16. **Scrub before format** (Task 5 step 4, `deliverText`). The sender runs `security.Scrub` on the Markdown before `toWhatsApp`, which reverses the order that step gave. A registered secret that contains `|` or `**` would otherwise be altered by the formatter and slip past the scrub.
17. **Semaphore instead of mutex** (Task 5). The queue is a one-slot channel, so a send that waits for the slot can give up with its context. A `sync.Mutex` cannot be waited on with a context.
18. **Chunker fence rules** (Task 4). A fence line longer than `limit/2` runes counts as plain text, because it could not be reopened. A bare-fence reopen applies when the opening line is longer than `limit/4` runes; the code measures the whole line, not only the info string.
19. **Blank lines inside fences** (Task 4). Blank lines inside a fence are code and stay in the part, as `TestChunkTextKeepsBlankLinesInsideFences` checks. Open: `chunker.emit` still drops the blank lines at the end of a part that is cut inside a fence.
20. **Invalid UTF-8 is replaced** (Task 4). `chunkText` replaces invalid UTF-8 with U+FFFD before it counts runes, so no invalid byte reaches WhatsApp unchanged or counts as one rune.
21. **`tokenBucket.skew` and the 1 ms floor** (Task 5). A clock that does not move during a sleep counts the sleep as elapsed through `skew`, and a wait is at least 1 ms, so a frozen test clock cannot stall or spin the bucket. With `time.Now` the skew stays zero.
22. **Pipeline owner re-check** (Task 7). `pipeline.handle` verifies that `in.Owner` is an `ownerRecipients` entry before the receipt, history, commands, pending questions or the agent run, so the filter is not the only gate.
23. **Newsletter-only forwards** (Task 7). Task 7 step 7 reads three forward signals, not two. A message with only `ForwardedNewsletterMessageInfo` set counts as forwarded: it gets the marker, no HumanInput, no command and no pending-question answer.
24. **Voice output suppressed until stage 3** (Task 7). The agent loop runs with `agent.WithVoiceOutputSuppressed` and `VoiceOutputActive: false`, because the broker drops audio in stage 2. Stage 3 removes both when voice notes land.
25. **Quoted-reply prefix stays in stage 3** (Task 7). The `[Reply to: …]` prefix for quoted replies is not part of stage 2. It stays in stage 3 Task 2 of the master plan.
26. **Stricter `ConfigState`** (Task 8). In dedicated mode `ConfigState` reports `not_configured` when no `owner_numbers` entry survives `config.NormalizeE164` (`hasValidOwnerNumber`). A Bot without a valid owner would drop every message as a stranger's.
27. **Cutoff gate `!synced && !resumeSynced`** (Task 8). The offline cutoff applies only while the sync is unfinished and no keepalive timeout has hidden a finished sync. With a plain `!synced` gate, a keepalive timeout on a finished connection would bring the local-clock cutoff back for live messages (Q1).

## Dispatch grouping (lean execution, in dependency order)

| Batch | Tasks | Agent(s) | Review | Why grouped |
| --- | --- | --- | --- | --- |
| B1 | 1, 2, 3 | maker-agent | security-agent (combined) | Task 3 needs the client methods and the caches; the filter is the stage's main security surface |
| B2 | 4, 5, 6 | maker-agent | one combined review | The outbound path: formatting → queue → broker |
| B3 | 7 (two commits: 7a pipeline, 7b chat-source switches) | maker-agent | security-agent | Ingress guard, forwarded handling, operator commands |
| B4 | 8, 9, 10 | maker-agent for code; docs-agent for the 16-locale strings of Task 9 and the DOX of Task 10 | one combined review | Wiring, user-visible surfaces and contracts. Order inside the batch: Task 8 commit; Task 9 code (maker) → locales (docs-agent) → Task 9 commit; Task 10 manifest (maker) + DOX (docs-agent) → Task 10 commit. test-agent runs the stage sweep at the end. |
| — | 11 | the user (real number) | — | Checklist only |

Each maker dispatch runs its per-task test commands before committing.

## Open questions for the user

None. Everything else follows from the design, the master plan and the whatsmeow source.

---

## Task 1 — Client extension

**Files:** modify `internal/whatsapp/client.go` and `internal/whatsapp/fake_test.go`.

**Interface additions** (`waClient`, after `SendPresence`; add `time` to the imports):

```go
MarkRead(ctx context.Context, ids []types.MessageID, timestamp time.Time, chat, sender types.JID) error
SendChatPresence(ctx context.Context, chat types.JID, state types.ChatPresence, media types.ChatPresenceMedia) error
// OwnLID is the linked account's LID without a device part, or EmptyJID before the first login.
OwnLID() types.JID
// PNForLID maps a LID to its phone JID through Store.LIDs. An unknown LID returns EmptyJID and no error.
PNForLID(ctx context.Context, lid types.JID) (types.JID, error)
// LastSuccessfulConnect is the time whatsmeow accepted the current login. Read it only from an
// event handler: whatsmeow writes it on its handler queue without a lock (connectionevents.go:164).
LastSuccessfulConnect() time.Time
```

**Adapter** (`whatsmeowClient`), with one-line delegations:
- `MarkRead` → `c.cli.MarkRead(ctx, ids, timestamp, chat, sender)`.
- `SendChatPresence` → `c.cli.SendChatPresence(ctx, chat, state, media)`.
- `OwnLID` → `c.cli.Store.GetLID().ToNonAD()`.
- `PNForLID`: `pn, err := c.cli.Store.LIDs.GetPNForLID(ctx, lid.ToNonAD()); return pn.ToNonAD(), err`.
- `LastSuccessfulConnect` → `c.cli.LastSuccessfulConnect`.
- Update the interface comment: "Stage 2 adds receipts, chat presence, LID lookups and the login time; stage 3 adds media."

**Fake** (`fakeClient`). Add these fields and methods, each guarded by `f.mu`:
- `ownLID types.JID`
- `lidPN map[types.JID]types.JID`, keyed by the non-AD LID
- `lidErr error`
- `lastConnect time.Time`
- `reads []readCall`, with `type readCall struct{ ids []types.MessageID; chat, sender types.JID }`
- `chatStates []chatStateCall`, with `type chatStateCall struct{ chat types.JID; state types.ChatPresence }`
- `sendErrs []error`: `SendMessage` consumes one entry per call before it falls back to `sendErr`.

`SendMessage` keeps returning IDs `fake-N`.

**Tests:** the existing compile-time assertions `var _ waClient = (*fakeClient)(nil)` (`client_test.go:22`) and `NewClient() waClient { return newWhatsmeowClient(…) }` prove that both implementations satisfy the interface. No new test.
`CGO_ENABLED=0 go test ./internal/whatsapp/ -count=1`

**GitNexus impact:** `waClient` (interface). Implementations: `whatsmeowClient`, `fakeClient`. Callers include `link.go`.
**Commit:** `feat(whatsapp): add read receipts, chat presence, LID lookups and login time to the client`

## Task 2 — ID caches

**Files:** create `internal/whatsapp/cache.go` and `internal/whatsapp/cache_test.go`.

```go
// idCache remembers message IDs, at most max of them, each for ttl (0 = no expiry).
// The oldest entry is evicted first. It is safe for concurrent use.
type idCache struct {
	mu    sync.Mutex
	max   int
	ttl   time.Duration
	now   func() time.Time
	order *list.List               // front = newest; values are idEntry
	items map[string]*list.Element
}
type idEntry struct { id string; at time.Time }

func newIDCache(max int, ttl time.Duration, now func() time.Time) *idCache
func (c *idCache) Add(id string)           // refreshes an existing entry; evicts expired entries from the back, then the oldest beyond max
func (c *idCache) Contains(id string) bool // an expired entry counts as absent and is removed
func (c *idCache) Len() int

const (
	sentIDCacheSize  = 5000
	sentIDCacheTTL   = 20 * time.Minute
	dedupeCacheSize  = 2048 // no TTL: bounded FIFO of accepted IDs
)
```

An empty `id` is ignored by `Add` and never `Contains`.

**Tests:**
- `TestIDCacheExpiresAfterTTL`: with a fake clock, an ID is present at 19m59s and absent at 20m01s; `Len` shrinks after the next `Add`.
- `TestIDCacheEvictsOldestBeyondCapacity`: with `max=3`, adding a–d evicts a; re-adding b refreshes it, so the next `Add` evicts c rather than b.

`CGO_ENABLED=0 go test ./internal/whatsapp/ -run '(TestIDCache)' -count=1`

**GitNexus impact:** none (new code).
**Commit:** `feat(whatsapp): add bounded message ID caches`

## Task 3 — Inbound filter chain and drop counters

**Files:** create `internal/whatsapp/inbound.go` and `inbound_test.go`; modify `internal/integrationstatus/whatsapp.go` and `whatsapp_test.go`.

### Status additions (`integrationstatus/whatsapp.go`)

```go
// Drop reasons with their own status field; every other reason is counted in DroppedFiltered.
const (
	WhatsAppDropNonOwner      = "non_owner"
	WhatsAppDropUnresolvedLID = "unresolved_lid"
)
// in WhatsAppRuntimeStatus, after DroppedUnresolvedLID:
DroppedFiltered map[string]int `json:"dropped_filtered,omitempty"` // fixed reason code -> count

// CountWhatsAppDrop counts one dropped inbound message by its fixed reason code.
func CountWhatsAppDrop(reason string)
```

- `WhatsAppStatus()` returns a copy whose `DroppedFiltered` is `maps.Clone`d, so the snapshot stays passive.
- Counters survive generations. `SetWhatsAppConfigState` does not reset them.

### Filter (`inbound.go`)

```go
// Fixed drop reason codes. Status counts them; logs never carry message content.
const (
	dropStatusBroadcast   = "status"
	dropBroadcast         = "broadcast"
	dropNewsletter        = "newsletter"
	dropGroup             = "group"
	dropEmpty             = "empty"
	dropEdit              = "edit" // edits, pins and revokes (Info.Edit)
	dropProtocol          = "protocol"
	dropReaction          = "reaction"
	dropPoll              = "poll"
	dropEcho              = "echo"
	dropDuplicate         = "duplicate"
	dropStale             = "stale"
	dropFromMe            = "from_me"
	dropUnsupportedSender = "unsupported_sender"
	dropNotSelfChat       = "not_self_chat"
	dropNonOwner          = integrationstatus.WhatsAppDropNonOwner
	dropUnresolvedLID     = integrationstatus.WhatsAppDropUnresolvedLID
)

// inboundMessage is an accepted owner message.
type inboundMessage struct {
	Chat      types.JID // chat the owner wrote in (receipts, typing)
	Sender    types.JID // whatsmeow sender (receipts)
	Owner     types.JID // owner phone JID: the reply target, always a member of ownerRecipients
	ID        types.MessageID
	Timestamp time.Time
	Message   *waE2E.Message // unwrapped content
}

type inboundFilter struct {
	mode      string
	owners    map[string]bool // owner numbers as JID users (E.164 without "+")
	sent      *idCache        // shared with the sender: the echo guard
	seen      *idCache        // dedupe of accepted IDs
	logger    *slog.Logger
	mu        sync.Mutex
	ownerLIDs map[string]types.JID // LID user -> owner phone JID, learned on acceptance
}

func newInboundFilter(cfg config.WhatsAppConfig, sent *idCache, logger *slog.Logger) *inboundFilter

// accept runs design steps 2-8. Step 1 (only *events.Message; HistorySync ignored) is the
// caller's type switch. cutoff is zero outside the offline sync (Task 8).
func (f *inboundFilter) accept(ctx context.Context, client waClient, evt *events.Message, cutoff time.Time) (inboundMessage, string)
```

Order — **keep exactly**:

```go
info := evt.Info
chat := info.Chat.ToNonAD()
switch { // 2. chats AuraGo never serves; status and broadcast lists share the broadcast server
case chat.Server == types.BroadcastServer && chat.User == types.StatusBroadcastJID.User:
	return inboundMessage{}, dropStatusBroadcast
case chat.Server == types.BroadcastServer:
	return inboundMessage{}, dropBroadcast
case chat.Server == types.NewsletterServer || info.Sender.Server == types.NewsletterServer:
	return inboundMessage{}, dropNewsletter
case info.IsGroup || chat.Server == types.GroupServer:
	return inboundMessage{}, dropGroup
}
if evt.Message == nil && evt.RawMessage != nil { // 3. whatsmeow unwraps already (UnwrapRaw)
	evt.UnwrapRaw()
}
if reason := kindDropReason(evt); reason != "" {
	return inboundMessage{}, reason
}
id := string(info.ID)
if f.sent.Contains(id) { return inboundMessage{}, dropEcho }      // 4.
if f.seen.Contains(id) { return inboundMessage{}, dropDuplicate } // 5. recorded only on acceptance
if !cutoff.IsZero() && info.Timestamp.Before(cutoff) {            // 6.
	return inboundMessage{}, dropStale
}
var owner types.JID
var reason string
if f.mode == config.WhatsAppModeSelfChat {                         // 8. before any content is read
	owner, reason = selfChatOwner(info, client.OwnJID(), client.OwnLID())
} else {                                                           // 7.
	owner, reason = f.dedicatedOwner(ctx, client, info)
}
if reason != "" { return inboundMessage{}, reason }
f.seen.Add(id)
return inboundMessage{Chat: chat, Sender: info.Sender, Owner: owner, ID: info.ID, Timestamp: info.Timestamp, Message: evt.Message}, ""
```

`kindDropReason(evt)` returns, in this order:
- `dropEmpty` when `evt.Message == nil`.
- `dropEdit` when `evt.IsEdit || evt.Info.Edit != types.EditAttributeEmpty`.
- `dropProtocol` when `GetProtocolMessage() != nil`.
- `dropReaction` when `GetReactionMessage()` or `GetEncReactionMessage()` is set.
- `dropPoll` for any `GetPollCreationMessage`/`V2`/`V3`/`V4`/`V5`/`V6`, `GetPollUpdateMessage`, `GetPollResultSnapshotMessage` or `V3`.

**Dedicated owner (step 7)**, `func (f *inboundFilter) dedicatedOwner(ctx, client, info) (types.JID, string)`:
1. `info.IsFromMe` → `dropFromMe`.
2. Resolve the sender's phone with `senderPhone`. It always works on `info.Sender.ToNonAD()`:
   - `s.whatsapp.net` → use the sender as is.
   - `lid` →
     - `info.SenderAlt.ToNonAD()` when its server is `s.whatsapp.net` and its user is non-empty;
     - else `client.PNForLID(ctx, sender)` when that yields a non-empty phone JID (an error counts as a miss);
     - else `f.ownerLIDs[sender.User]`;
     - else `dropUnresolvedLID`.
   - any other server → `dropUnsupportedSender`.
3. `dropUnresolvedLID` → `f.logger.Warn("WhatsApp sender has no phone mapping; message dropped", "sender", maskJID(info.Sender))`.
4. Phone not in `f.owners` → `f.logger.Info("WhatsApp message from a non-owner dropped", "sender", maskJID(pn))` and `dropNonOwner`.
5. Accepted → remember the owner's LID: `info.Sender` when it is a LID, else `info.SenderAlt` when it is a LID. Store it in `f.ownerLIDs` as LID user → phone JID. Return the phone JID.

**Self chat (step 8):**

```go
func selfChatOwner(info types.MessageInfo, own, ownLID types.JID) (types.JID, string) {
	chat := info.Chat.ToNonAD()
	switch {
	case !info.IsFromMe || own.IsEmpty():
		return types.EmptyJID, dropNotSelfChat
	case chat.Server == types.DefaultUserServer && chat.User == own.User,
		chat.Server == types.HiddenUserServer && !ownLID.IsEmpty() && chat.User == ownLID.User:
		return own.ToNonAD(), ""
	}
	return types.EmptyJID, dropNotSelfChat
}
```

**Tests** (`inbound_test.go`). Use a helper `msgEvent(info types.MessageInfo, raw *waE2E.Message) *events.Message` that returns `(&events.Message{Info: info, RawMessage: raw}).UnwrapRaw()`. Set every `Timestamp` to `fixedNow()`. Log into a `bytes.Buffer` handler.
- `TestInboundResolvesOwnerByPhoneAltStoreAndMemory`. Table cases, each accepted with `Owner == 4917012345678@s.whatsapp.net`:
  - phone sender;
  - LID sender with a phone `SenderAlt`;
  - LID sender resolved through `fake.lidPN`;
  - phone sender with a whatsmeow-filled LID `SenderAlt` (still resolved by `Sender`);
  - after the `SenderAlt` case, the same LID with no alt and an empty store (learned LID).
- `TestInboundDropsUnresolvedAndNonOwnerSendersWithoutContent`:
  - an unresolved LID → `unresolved_lid`; the log has `***4444` and neither the full LID digits nor the message text;
  - a stranger's phone → `non_owner`; the log has only the masked number;
  - `IsFromMe` in dedicated mode → `from_me`.
- `TestInboundDropsChatsAuraGoNeverServes`: group (`…@g.us`, `IsGroup`) → `group`; broadcast list → `broadcast`; `status@broadcast` → `status`; newsletter → `newsletter`.
- `TestInboundDropsNonConversationKinds`:
  - `ProtocolMessage` → `protocol`;
  - `ReactionMessage` and `EncReactionMessage` → `reaction`;
  - an `EditedMessage` wrapper → `edit`;
  - `Info.Edit = EditAttributeSenderRevoke` → `edit`;
  - `PollCreationMessageV3` and `PollUpdateMessage` → `poll`;
  - nil content → `empty`.
- `TestInboundAcceptsEphemeralAndViewOnceContent`: an `EphemeralMessage` and a `ViewOnceMessageV2` each wrapping `ExtendedTextMessage{Text:"hi"}` are accepted, and `Message.GetExtendedTextMessage().GetText() == "hi"`.
- `TestInboundEchoDedupeAndOfflineCutoff`:
  - an ID in `sent` → `echo`;
  - the same accepted ID twice → the second is `duplicate`;
  - a rejected ID does not enter `seen`;
  - `Timestamp = cutoff − 1s` → `stale`; a zero cutoff accepts an old timestamp.
- `TestInboundSelfChatAcceptsOnlyTheOwnChat`, in `self_chat` mode:
  - `IsFromMe` in the own phone chat → accepted, `Owner == testOwn`;
  - in the own LID chat → accepted;
  - `IsFromMe` in a contact's chat → `not_self_chat`;
  - not `IsFromMe` in the own chat → `not_self_chat`.
- `TestWhatsAppDropCountersAreSeparatedAndCopied` (`integrationstatus`): `non_owner` and `unresolved_lid` go to their fields; `group` goes to `DroppedFiltered["group"]`; mutating a returned map does not change the next snapshot.

Commands:
- `CGO_ENABLED=0 go test ./internal/whatsapp/ -run '(TestInbound)' -count=1`
- `CGO_ENABLED=0 go test ./internal/integrationstatus/ -run '(TestWhatsApp)' -count=1`

**GitNexus impact:** `WhatsAppStatus`, `WhatsAppRuntimeStatus`.
**Commit:** `feat(whatsapp): filter inbound messages down to the owner`

## Task 4 — Formatting and chunking

**Files:** create `internal/whatsapp/format.go`, `format_test.go`, `chunks.go` and `chunks_test.go`.

```go
// toWhatsApp converts the agent's Markdown to WhatsApp formatting. Code spans and
// fenced blocks pass through byte for byte.
func toWhatsApp(markdown string) string

const maxChunkRunes = 4000

// chunkText splits text into parts of at most limit runes (see the rules below).
func chunkText(text string, limit int) []string
```

**Conversion rules** (design "Outbound"), applied line by line outside fences:
- **Fences:** a line whose trimmed form starts with "```" toggles fence state. Lines inside a fence, and the fence lines themselves, are copied unchanged.
- **Tables:** a run of lines starting with `|` whose second line is a separator (`|---|:--:|`) becomes one line per row, for every row except the separator: `• ` + the trimmed non-empty cells, each inline-converted and joined with ` — `. A run without a separator stays as plain lines.
- **Headings:** `^#{1,6}\s+text\s*#*$` → `*` + the inline-converted text with leading and trailing `*`/`_` trimmed + `*`.
- **Bullets:** a leading `* ` or `+ ` (after indentation) becomes `- `. `- `, `1. ` and `> ` stay as they are.
- **Inline**, outside backtick code spans only. A hand-written scanner is recommended over regexes, because Go regexp has no lookbehind and a consumed boundary breaks adjacent matches.
  - `**x**` and `__x__` → `*x*`.
  - `*x*` → `_x_`.
  - `_x_` stays as it is.
  - `~~x~~` → `~x~`.
  - An opener must be followed by a non-space and a closer preceded by one, on the same line. For `_`, `__` and single `*`, the opener needs a non-word rune or the line start before it, and the closer a non-word rune or the line end after it. This keeps `snake_case_name` and `2*3*4` unchanged.
  - Convert bold to a placeholder first, then italic, then turn the placeholder into `*`. Otherwise `**x**` would end as `_x_`.
- **Links:** `[text](url)` → `text (url)`, or just `url` when text equals url. A leftover `![alt](url)` becomes `alt (url)`, or `url` when alt is empty.

**Chunking rules:**
- Trim leading and trailing newlines. Empty text gives no parts. Text of at most `limit` runes is one part.
- Cut preference: after a blank line outside a fence, then at a line end, then (for a single oversize line) at the last space inside the window, then a hard cut at a rune boundary. Count runes with `utf8`, never bytes.
- **Fence reopening:** while a part ends inside a fence, close it with `"\n```"` and start the next part with the fence's opening line (info string included, for example "```go"). The budget of a part that may need the close reserves 4 runes, and a reopened part counts its opening line.

```go
// reference shape of the fence carry
if open != "" { body += "\n```" }   // closing the current part
...
if open != "" { next = append(next, open) } // the next part starts by reopening
```

- Every part satisfies all of these: at most `limit` runes; an even count of fence lines; no part is whitespace-only.

**Tests:**
- `TestToWhatsAppConvertsMarkdown`. Table of input → output:
  - `**b**` and `__b__` → `*b*`; `*i*` → `_i_`; `_i_` unchanged; `~~s~~` → `~s~`;
  - `*a* and *b*` → `_a_ and _b_`;
  - `snake_case_name` and `2*3*4` unchanged;
  - `### **Title**` → `*Title*`;
  - `[docs](https://example.org)` → `docs (https://example.org)`;
  - `[https://example.org](https://example.org)` → `https://example.org`;
  - `* item` → `- item`; `- x`, `1. x` and `> q` unchanged.
- `TestToWhatsAppNeverTouchesCode`: `` `**x**` `` inline, and a fenced block holding `**x**`, `# not a heading` and `| a | b |`, come out byte-identical.
- `TestToWhatsAppTurnsTablesIntoBullets`: a 2-column table with header, separator and two rows gives three `• a — b` lines; a run of pipe lines without a separator stays unchanged.
- `TestChunkTextPrefersParagraphThenLine`: with a small limit (for example 40), cuts land after a blank line when one fits, else at a line end; the joined parts equal the input apart from whitespace at the cuts.
- `TestChunkTextReopensCodeFences`: an ```` ```go ```` block longer than three limits; every part has at most `limit` runes and balanced fences; every part after the first starts with ```` ```go ````; removing the inserted fence lines restores the code.
- `TestChunkTextHardSplitsLongLinesOnRunes`: a single line of umlauts and multi-byte runes splits on rune boundaries (`utf8.ValidString` holds for each part) and at the last space when there is one.

`CGO_ENABLED=0 go test ./internal/whatsapp/ -run '(TestToWhatsApp|TestChunkText)' -count=1`

**GitNexus impact:** none (new code). Do not reuse `telegramTextChunks`: it counts UTF-16 units and knows nothing about fences.
**Commit:** `feat(whatsapp): convert Markdown to WhatsApp formatting and chunk replies`

## Task 5 — Send queue and recipient guard

**Files:**
- create `internal/whatsapp/send.go` and `send_test.go`;
- modify `runtime.go` (`Bot` fields, `start`, `SendTestMessage`), `recipients.go` (`testMessageText`) and `errors.go`.

**Errors (`errors.go`):**

```go
ErrRecipientNotOnWhatsApp = errors.New("whatsapp owner number is not registered on WhatsApp")
errNotOwnerRecipient      = errors.New("whatsapp refuses a recipient that is not an owner") // internal; no API takes numbers
```

**Sender:**

```go
const (
	sendBurst    = 5
	sendInterval = 3 * time.Second // 20 messages per minute
)

// tokenBucket paces the send queue. The caller holds sender.mu, so take runs serially.
type tokenBucket struct {
	tokens float64
	last   time.Time
	now    func() time.Time
	sleep  func(ctx context.Context, d time.Duration) bool
}

func (b *tokenBucket) take(ctx context.Context) bool {
	for {
		now := b.now()
		if b.last.IsZero() {
			b.tokens, b.last = sendBurst, now
		}
		b.tokens = math.Min(sendBurst, b.tokens+float64(now.Sub(b.last))/float64(sendInterval))
		b.last = now
		if b.tokens >= 1 {
			b.tokens--
			return true
		}
		if !b.sleep(ctx, time.Duration((1-b.tokens)*float64(sendInterval))) {
			return false
		}
	}
}

// sender is the Bot's one serialized outbound queue. Every text AuraGo sends goes through
// deliverText: owner guard, formatting, scrubbing, chunking, self-chat prefix, pacing,
// echo-cache registration and delivery accounting.
type sender struct {
	mu     sync.Mutex // held for all chunks of one message, so chunks stay together
	cfg    config.WhatsAppConfig
	client func() waClient
	sent   *idCache
	bucket tokenBucket
	now    func() time.Time
	logger *slog.Logger
}

func newSender(cfg config.WhatsAppConfig, client func() waClient, sent *idCache, deps runtimeDeps, logger *slog.Logger) *sender
func (s *sender) deliverText(ctx context.Context, to types.JID, markdown, what string) error
func (s *sender) deliverToOwners(ctx context.Context, markdown, what string) error
```

`deliverText` steps, in order:
1. If `ctx.Err() != nil`, return it. A cancelled send is never counted.
2. Get `client := s.client()`. If it is nil or not logged in, return `ErrNotConnected`.
3. **Guard:** `owners, err := ownerRecipients(s.cfg, client.OwnJID())`. Unless `to.ToNonAD()` equals one entry, log a WARN with the masked recipient and return `errNotOwnerRecipient`, without sending and without counting.
4. Build the body with `security.Scrub(toWhatsApp(markdown))`. The prefix is `s.cfg.SelfChatPrefix` in `self_chat` mode, else empty. Split with `chunkText(body, maxChunkRunes-utf8.RuneCountInString(prefix))`.
5. Under `s.mu`, for each chunk:
   - call `s.bucket.take(ctx)`; if that fails, return `ctx.Err()`.
   - `resp, err := client.SendMessage(ctx, to, &waE2E.Message{Conversation: proto.String(prefix + chunk)})`.
   - **On error:** if `ctx.Err() != nil`, return without counting. Otherwise call `integrationstatus.MarkWhatsAppDeliveryFailure()` and `s.logger.Warn("Failed to send WhatsApp "+what, "error", maskDigits(security.Scrub(err.Error())))`. If `errors.Is(err, whatsmeow.ErrRecipientNotOnWhatsApp)`, return `fmt.Errorf("%w: %w", ErrRecipientNotOnWhatsApp, err)`. Stop at the first failed chunk.
   - **On success:** `s.sent.Add(string(resp.ID))` and `integrationstatus.MarkWhatsAppOutbound(s.now())`.

`deliverToOwners` sends to every `ownerRecipients` entry. It continues after a failure and returns the first error, like stage 1.

**Runtime and recipients changes:**
- `Bot` gains `sent *idCache` and `sender *sender`, built in `start()` from `cfg.WhatsApp`, `b.currentClient` and `deps`.
- `SendTestMessage` becomes `deliverToOwners(ctx, testMessageText(now), "test message")`, then `MarkWhatsAppTest(now)` on success. Remove its direct `SendMessage` call.
- `testMessageText(now time.Time) string` drops the `cfg` parameter and the prefix branch; the sender adds the prefix (deviation 8). Its doc comment says so.

**Mirror:** `sendTelegramMessage` scrubs and chunks (`internal/telegram/bot.go:815-825`). `recordTelegramDeliveryFailure` counts and logs every failed content path (`bot.go:781-788`).

**Tests:**
- `TestSenderRefusesNonOwnerRecipients`: dedicated owner `+4917012345678`. A send to the stranger → `errNotOwnerRecipient`, `fake.sent` stays empty and `DeliveryFailures` is unchanged. A send to the owner → one message.
- `TestTokenBucketAllowsBurstThenTwentyPerMinute`: a fake clock whose `sleep` advances it and records the wait. Five takes record no sleeps; the sixth sleeps 3 s; after a 60 s pause the next five need no sleep and the sixth sleeps again (the burst is capped at 5).
- `TestSenderRegistersSentIDsAndCountsFailures`: after a send, `sent.Contains("fake-1")`. With `sendErrs = [boom]`, `DeliveryFailures` rises by 1 and no ID is added. With a cancelled ctx, the failure is not counted.
- `TestSenderSurfacesRecipientNotOnWhatsApp`: `sendErrs = [fmt.Errorf("%w: no LID found", whatsmeow.ErrRecipientNotOnWhatsApp)]` → `errors.Is(err, ErrRecipientNotOnWhatsApp)`.
- `TestSenderPrefixesEveryChunkInSelfChat`: `self_chat` mode with a 9000-rune body. Every sent text starts with the prefix and has at most 4000 runes; the chunk order is preserved.
- `TestOnlyTheSenderCallsSendMessage`: parse the package's non-test `.go` files with `go/parser`. The only `.SendMessage(` selector calls are in `client.go` (the adapter) and `send.go`. This proves the single-resolver contract.
- Unchanged and still green: `TestBotSendTestMessageReachesOnlyOwners` (same text, prefix once).

`CGO_ENABLED=0 go test ./internal/whatsapp/ -run '(TestSender|TestTokenBucket|TestOnlyTheSender|TestBotSendTestMessage)' -count=1`

**GitNexus impact:** `SendTestMessage`, `start`, `testMessageText`, `Bot`.
**Commit:** `feat(whatsapp): send through one rate-limited, owner-only queue`

## Task 6 — Broker

**Files:** create `internal/whatsapp/broker.go` and `broker_test.go`.

```go
const statusCoalesceWindow = 10 * time.Second

// whatsAppBroker implements agent.FeedbackBroker for one owner turn (TelegramBroker parity
// for text events; audio and YouTube capture arrive in stage 3).
type whatsAppBroker struct {
	ctx        context.Context
	sender     *sender
	to         types.JID
	now        func() time.Time
	mu         sync.Mutex
	lastStatus time.Time
}

func (b *whatsAppBroker) Send(event, message string)
func (b *whatsAppBroker) SendJSON(string)                                          {}
func (b *whatsAppBroker) SendLLMStreamDelta(string, string, string, int, string)   {}
func (b *whatsAppBroker) SendLLMStreamDone(string)                                 {}
func (b *whatsAppBroker) SendTokenUpdate(int, int, int, int, int, bool, bool, string) {}
func (b *whatsAppBroker) SendThinkingBlock(string, string, string)                 {}
```

`Send` mirrors `TelegramBroker.Send` (`internal/telegram/bot.go:942-999`):
- `question_user` → deliver the message as is, never coalesced.
- `tool_start`, `error_recovery`, `api_retry`, `progress` → the same texts and prefixes as `bot.go:983-990` (progress gets its own prefix plus the message; the others get prefix + `UPPER(event): message`). Deliver only if `now − lastStatus ≥ 10 s`, then set `lastStatus`; drop the others silently. Unlike Telegram, log nothing: `bot.go:982` logs the message content, which the privacy rule forbids.
- `budget_warning` and `budget_blocked` → the `bot.go:993-998` prefixes, never coalesced.
- Everything else, including `audio` and `youtube_video` in stage 2, is ignored.
- Delivery is `b.sender.deliverText(b.ctx, b.to, text, "<label>")`; the sender counts and logs failures.

Add a compile-time assertion `var _ agent.FeedbackBroker = (*whatsAppBroker)(nil)` in `broker.go`.

**Tests:**
- `TestBrokerDeliversQuestionsAndBudgetNoticesImmediately`: `question_user`, `budget_warning` and `budget_blocked` within 1 s → three sends, with Telegram's texts.
- `TestBrokerCoalescesStatusToOnePerTenSeconds`: with a fake clock, `tool_start` at t0 → sent; `progress` at t0+3s → dropped; `api_retry` at t0+11s → sent. `audio` → nothing sent.

`CGO_ENABLED=0 go test ./internal/whatsapp/ -run '(TestBroker)' -count=1`

**GitNexus impact:** none (new code).
**Commit:** `feat(whatsapp): add the agent feedback broker`

## Task 7 — Pipeline and chat-source parity

### 7a — Pipeline

**Files:** create `internal/whatsapp/pipeline.go` and `pipeline_test.go`; modify `internal/tools/question_user.go` (add `QuestionSourceWhatsApp`) and `internal/agent/question_user_answer_test.go` (one row).

```go
// AgentDeps are the collaborators the pipeline hands to agent.ExecuteAgentLoop, the same set
// telegram.StartLongPolling receives (internal/telegram/bot.go:65).
type AgentDeps struct {
	LLMClient        llm.ChatClient
	ShortTermMem     *memory.SQLiteMemory
	LongTermMem      memory.VectorDB
	Vault            *security.Vault
	Registry         *tools.ProcessRegistry
	CronManager      *tools.CronManager
	HistoryManager   *memory.HistoryManager
	InventoryDB      *sql.DB
	MissionManagerV2 *tools.MissionManagerV2
	RemoteHub        *remote.RemoteHub
	Guardian         *security.Guardian
	BudgetTracker    func() *budget.Tracker
	WebhookManager   *webhooks.Manager
	Snapshot         func() (*config.Config, llm.ChatClient) // current config and client per message
	RoutingDecisions agent.TaskDecisionService
}

type agentLoopFunc func(ctx context.Context, req openai.ChatCompletionRequest, runCfg agent.RunConfig, stream bool, broker agent.FeedbackBroker) (openai.ChatCompletionResponse, error)

const typingRefresh = 8 * time.Second

type pipeline struct {
	base        *config.Config // the generation's config clone
	deps        AgentDeps
	sender      *sender
	client      func() waClient
	agentLoop   agentLoopFunc
	now         func() time.Time
	typingEvery time.Duration
	logger      *slog.Logger
}

func (p *pipeline) handle(ctx context.Context, in inboundMessage)
```

`tools.QuestionSourceWhatsApp QuestionSource = "whatsapp"` has the doc comment "a WhatsApp owner-chat reply". Add the row `tools.QuestionSourceWhatsApp: false` to `TestQuestionAnswerFromAdminSurface`.

`handle` mirrors `processUpdate` (`internal/telegram/bot.go:294-736`), text only. Step by step:
1. **Config:** take `cfg, client := p.deps.Snapshot()` (fall back to `p.base` and `p.deps.LLMClient`). Return when `cfg` is nil, egg mode is on, or `!cfg.WhatsApp.Equal(p.base.WhatsApp)`; the generation is being replaced (mirror of `bot.go:74`). If `p.deps.ShortTermMem == nil`, log an Error without content and return.
2. **Read receipt:** when `cfg.WhatsApp.Mode == dedicated && cfg.WhatsApp.SendReadReceipts`, call `client.MarkRead(ctx, []types.MessageID{in.ID}, p.now(), in.Chat, in.Sender)`. Errors are logged masked and not counted (no content is lost).
3. **Budget and session:** `budget.WithTracking(ctx, tracker, "chat")` and the `tools.IsBusy()` Info log (`bot.go:295-307`). Resolve the session with `ResolveUserSession(ctx, "default")` and call `RegisterSessionRun`. When the session is not `default`, set `historyManager = nil` (`bot.go:309-321`).
4. **Text:** `messageText(in.Message)` reads `GetConversation()`, else `GetExtendedTextMessage().GetText()`. With no text, log Info `"WhatsApp message kind not handled yet", "kind", <image|audio|video|document|sticker|location|contact|other>` and return.
5. **Received log:** `logger.Info("Received authorized WhatsApp message", "sender", maskJID(in.Owner))`. Never the text.
6. **HumanInput:** `routingText := inputText`, then `memory.NewHumanInputEvent(inputText, sessionID, "whatsapp:"+in.Chat.User+":"+string(in.ID), in.Timestamp)`. The ID is hashed into `TurnID` (`internal/memory/personality_dynamics.go:163`), so the number is not stored in clear.
7. **Forwarded:** `ci := GetExtendedTextMessage().GetContextInfo()`. When `ci.GetIsForwarded() || ci.GetForwardingScore() > 0`:
   - `marker := telegram.ForwardedContentMarker(ci.GetForwardedNewsletterMessageInfo().GetNewsletterName())`;
   - `inputText = marker + "\n" + inputText`;
   - `routingText = forwardedRoutingText(marker, routingText)` (a local copy of `bot.go`'s `telegramForwardedRoutingText`, `forwarded.go:88-93`);
   - `humanInput = nil`.
8. **Pending question** (`bot.go:521-531`): when `tools.HasPendingQuestion(sessionID)` and the message is not forwarded, try `ResolveQuestionReply`. On a hit, set `Source = tools.QuestionSourceWhatsApp`, call `CompleteQuestion` and return. Otherwise deliver `"Please reply with one of the listed numbers."` and return.
9. **Turn lock:** `LockSessionTurn` with deferred release (`bot.go:535-539`).
10. **Commands** (`bot.go:543-564`): when not forwarded and the text starts with `/`, call `commands.Handle(text, commands.Context{STM, SessionID, HM, Vault, InventoryDB, Cfg, PromptsDir, AllowOperator: true})`. `AllowOperator` is always true: the filter guarantees an owner DM or the self chat, and groups never arrive. Deliver the result, or Telegram's command-error text, and return when it was a command.
11. **Guardian:** `shouldBlockWhatsAppPromptInjection(text, guardian, logger, in.Owner)` is a copy of `bot.go:796-809` that logs `"sender", maskJID(owner)`, `level` and `patterns`. When it blocks, deliver `"Your message was blocked by the security guardian."` and return. Otherwise `inputText = security.IsolateExternalData(inputText)`.
12. **History and run config:** `InsertMessage` plus `historyManager.Add` (`bot.go:576-582`). Build `runCfg` as in `bot.go:585-615` with `MessageSource: "whatsapp"`, `HumanInput`, `TaskRoutingText: &routingText` and `SessionID`. Build the messages with a local copy of `buildTelegramAgentMessages` (`bot.go:37-52`), plus the recent-messages fallback (`bot.go:618-630`).
13. **Typing:** a goroutine sends `client.SendChatPresence(ctx, in.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)` at once and then every `typingEvery`. After the loop, stop it and send `ChatPresencePaused` once (skipped when ctx is done). Errors are ignored and uncounted, like `sendChatAction` (`bot.go:877-882`).
14. **Agent loop:** `broker := &whatsAppBroker{ctx, sender, to: in.Owner, now}`, then `resp, err := p.agentLoop(ctx, req, runCfg, false, broker)`. On error, deliver Telegram's error notice (`bot.go:666-670`).
15. **Answer:** `security.StripThinkingTags`, then `media.ExtractMarkdownImages`. Each image appends the line `[Image: <caption or "image">]` (deviation 11). Deliver through `p.sender.deliverText(ctx, in.Owner, cleanText, "answer")`.

All replies go to `in.Owner` through the sender. Check `ctx.Err()` at the points `processUpdate` checks it.

**Tests** (`pipeline_test.go`). A fixture builds an in-memory `SQLiteMemory`, `fakeClient{loggedIn: true, own: testOwn}`, a sender on a fake clock, an `agentLoop` stub that records `(req, runCfg)` and returns `**Done** with `x``, and `typingEvery = 5ms`. Clean up `tools.CancelQuestion`.
- `TestPipelineRunsTheAgentInTheDefaultSession`:
  - the stub is called once with `runCfg.SessionID == "default"`, `MessageSource == "whatsapp"`, `TaskRoutingText == "hi"`, and `runCfg.HumanInput.TurnID` equal to `memory.NewHumanInputEvent("hi", "default", "whatsapp:4917012345678:ID1", ts).TurnID`;
  - the stored user text is `security.IsolateExternalData("hi")`;
  - the owner receives `*Done* with `x``.
- `TestPipelineAnswersAPendingQuestionFirst`: with the turn lock held and a question registered, `"continue"` → the response has `Source == tools.QuestionSourceWhatsApp` and `FreeText "continue"`; no stub call and no history row (mirror of `internal/telegram/turn_serialization_test.go:62-108`).
- `TestPipelineRunsSlashCommandsWithOperatorAllowed`: `/debug bogus` → the reply contains `i18n.T("de", "backend.cmd_debug_invalid")`, not the operator-refusal text; no stub call, no history.
- `TestPipelineMarksForwardedMessagesAndSkipsHumanInputCommandsAndAnswers`:
  - a forwarded `/help` (`ExtendedTextMessage` with `ContextInfo{IsForwarded: true}`) is not run as a command;
  - its stored text equals `IsolateExternalData("[Forwarded message; third-party content]\n/help")`;
  - `runCfg.HumanInput == nil`;
  - with a pending question, a forwarded `"yes"` leaves the question open and gets the "Please reply…" text.
- `TestPipelineBlocksHighThreatInputThenIsolates`: with `Guardian = security.NewGuardian`, the injection phrase from `internal/telegram/guardian_test.go:16` gets the blocked notice, no stub call and no history.
- `TestPipelineReadReceiptsOnlyInDedicatedModeWhenEnabled`: dedicated with the setting on → one `readCall{ids:[ID1], chat}`; dedicated with it off → none; `self_chat` → none.
- `TestPipelineTypesWhileRunningThenPauses`: a stub that waits 30 ms produces at least two `composing` entries, and the last chat state is `paused`.

### 7b — MessageSource `whatsapp` wherever `telegram` is a chat source

**Files:**
- modify `internal/agent/tool_route.go:55`, `task_router.go:95`, `operational_issues.go:439`, `meshcore_notice.go:15`, `emotion_behavior.go:116,125`;
- modify `internal/prompts/builder.go` (label switch ~1061-1096: add `case "whatsapp": label = "WhatsApp"` after `telegram`);
- modify `internal/agent/agent.go:1179` (comment lists `"whatsapp"`) and `internal/commands/commands.go:1-6,38-43` (comments name "WhatsApp owner chats (dedicated owner DMs and the self chat)" next to Telegram);
- tests: create `internal/agent/chat_source_whatsapp_test.go`; modify `internal/agent/meshcore_test.go` and `internal/prompts/builder_test.go`.

Add `"whatsapp"` directly after `"telegram"` in each case list. Nothing else changes.

**Tests:**
- `TestWhatsAppIsAChatSourceWhereverTelegramIs`: for `telegram` and `whatsapp` alike,
  - `shouldUseSupervisorToolRoute(RunConfig{MessageSource: s})`, `taskRouterEligible(RunConfig{MessageSource: s})` and `shouldConsiderOperationalIssueReminder(RunConfig{MessageSource: s, PlannerDB: &sql.DB{}}, "status?")` are true;
  - `isShortChannel(s)` is true;
  - `channelToneHint("whatsapp") == channelToneHint("telegram")`.
- `TestMeshCoreNoticeOnlyAtDirectContact` (extended): after the existing `ack()`, insert a second quarantined message; `RunConfig{Config: cfg, MessageSource: "whatsapp"}` receives the notice.
- `TestBuildSystemPromptLabelsWhatsAppSource`: the prompt contains `> **Channel:** WhatsApp` and not `> **Channel:** whatsapp` (copy of `builder_test.go:1242-1255`).

Commands:
- `CGO_ENABLED=0 go test ./internal/whatsapp/ -run '(TestPipeline)' -count=1`
- `CGO_ENABLED=0 go test ./internal/agent/ -run '(TestWhatsAppIsAChatSource|TestMeshCoreNoticeOnlyAtDirectContact|TestQuestionAnswerFromAdminSurface)' -count=1`
- `CGO_ENABLED=0 go test ./internal/prompts/ -run '(TestBuildSystemPromptLabels)' -count=1`

**GitNexus impact:** `shouldUseSupervisorToolRoute`, `taskRouterEligible`, `shouldConsiderOperationalIssueReminder`, `appendMeshCoreNotice`, `isShortChannel`, `channelToneHint` and `buildSystemPromptInnerContext`. The last one will be HIGH or CRITICAL as the central prompt builder: warn, and note that the edit adds one label case for a new source and changes no existing output. `QuestionSource` gains a const; no function changes.
**Commits:**
- 7a: `feat(whatsapp): hand owner messages to the agent pipeline`
- 7b: `feat(agent): treat WhatsApp as a chat source like Telegram`

## Task 8 — Runtime and server wiring

**Files:**
- modify `internal/whatsapp/runtime.go` and `runtime_test.go`;
- modify `internal/server/whatsapp_runtime.go` and `whatsapp_runtime_test.go`.

**Runtime changes:**
- `runtimeDeps` gains `agent AgentDeps` and `agentLoop agentLoopFunc`. `defaultRuntimeDeps` sets `agentLoop: agent.ExecuteAgentLoop`, and `start` defaults a nil `agentLoop` to the same, so existing test literals stay valid.
- New signature: `func Start(parent context.Context, cfg *config.Config, logger *slog.Logger, agentDeps AgentDeps) *Bot`. It builds `deps := defaultRuntimeDeps(logger); deps.agent = agentDeps`.
- `Bot` gains `filter *inboundFilter`, `pipeline *pipeline`, `workers sync.WaitGroup` and `slots chan struct{}` (capacity `pipelineWorkers = 5`, Telegram's default `MaxConcurrentWorkers`, `internal/telegram/runtime.go:159-163`). All are built in `start()` over the shared `b.sent`.
- `run()` registers `client.AddEventHandler(func(evt any) { b.handleEvent(client, evt, signals); if m, ok := evt.(*events.Message); ok { b.handleMessage(client, m) } })`. Leave `classifyEvent` unchanged; `Message` is still only counted there.

```go
const offlineCutoff = 10 * time.Minute

// handleMessage runs on whatsmeow's handler queue. The filter is cheap (map lookups, at most
// one LID-store read), so it runs inline under eventMu like handleEvent; a stopped generation
// neither counts nor starts work. The agent turn runs on a worker.
func (b *Bot) handleMessage(client waClient, evt *events.Message) {
	if b.ctx.Err() != nil {
		return
	}
	b.eventMu.Lock()
	defer b.eventMu.Unlock()
	if b.stopped {
		return
	}
	var cutoff time.Time
	if !b.synced {
		// Missed messages arrive after the login and before OfflineSyncCompleted on the
		// same queue; whatsmeow sets LastSuccessfulConnect before any of them. Live
		// messages (after the sync) never meet the cutoff, so local clock skew cannot drop them.
		ref := client.LastSuccessfulConnect()
		if ref.IsZero() {
			ref = b.deps.now()
		}
		cutoff = ref.Add(-offlineCutoff)
	}
	in, reason := b.filter.accept(b.ctx, client, evt, cutoff)
	if reason != "" {
		integrationstatus.CountWhatsAppDrop(reason)
		return
	}
	b.workers.Add(1)
	go func() {
		defer b.workers.Done()
		select {
		case b.slots <- struct{}{}:
		case <-b.ctx.Done():
			return
		}
		defer func() { <-b.slots }()
		b.pipeline.handle(b.ctx, in)
	}()
}
```

**Teardown in `run()`.** Replace the client defer:

```go
defer func() {
	b.eventMu.Lock()
	b.stopped = true // no new workers, no status writes (finish sets it again; idempotent)
	b.eventMu.Unlock()
	b.setClient(nil)
	client.Disconnect()
	b.workers.Wait() // workers end on b.ctx; the store closes only after them
}()
```

**Server** (`whatsapp_runtime.go`):
- Add `func (s *Server) whatsAppAgentDeps() whatsapp.AgentDeps`. It fills the fields from the same `Server` fields `configureTelegramBot` passes (`internal/server/telegram_runtime.go:33`): `BudgetTracker: s.budgetTrackerSnapshot`, `Snapshot: s.chatRuntimeSnapshot`, `RoutingDecisions: taskDecisionService{s}`.
- `configureWhatsAppBotLocked` calls `whatsapp.Start(parent, s.ConfigSnapshot(), s.Logger, s.whatsAppAgentDeps())`, still under `CfgMu.RLock`, as Telegram does.

**Tests:**
- `TestBotRoutesOwnerMessagesToThePipeline` (whatsapp). Start with a fake store, an in-memory memory store in `deps.agent.ShortTermMem` and a recording `agentLoop`. Emit an owner `*events.Message` with `Timestamp = fixedNow()` → exactly one `agentLoop` call with `MessageSource "whatsapp"`. Emit a stranger's message → no call, and `DroppedNonOwner` rises by 1.
- `TestBotOfflineCutoffAppliesOnlyDuringOfflineSync`: `fake.lastConnect = fixedNow()`. Before `OfflineSyncCompleted`, a message stamped `fixedNow()-11m` is counted `stale`. After emitting `&events.OfflineSyncCompleted{}`, a message with the same old stamp and a new ID reaches the pipeline.
- `TestBotStopWaitsForPipelineWorkers`: an `agentLoop` that blocks until its ctx ends. `Stop()` returns only after the loop has seen the cancellation. `TestStoppedBotNeverWritesStatus` stays green.
- `TestWhatsAppAgentDepsCarryServerCollaborators` (server): a `Server` with a non-nil `ShortTermMem` gives `deps.ShortTermMem == s.ShortTermMem` and non-nil `Snapshot`, `BudgetTracker` and `RoutingDecisions`.
- `TestWhatsAppRuntimeGenerationsFollowConfigAndShutdown` stays green; "one Bot per process" still holds.

Commands:
- `CGO_ENABLED=0 go test ./internal/whatsapp/ -count=1`
- `CGO_ENABLED=0 go test ./internal/server/ -run '(WhatsApp)' -count=1`

**GitNexus impact:** `Start` (the only caller is `configureWhatsAppBotLocked`), `start`, `run` in `internal/whatsapp/runtime.go`, `defaultRuntimeDeps`, `runtimeDeps`, `configureWhatsAppBotLocked`.
**Commit:** `feat(whatsapp): route inbound messages through the agent pipeline`

## Task 9 — Status surfaces

**Files:**
- modify `internal/server/dashboard_integrations.go:12` (add `"whatsapp": cfg.WhatsApp.Enabled` after `telegram`; it reports configuration, not health);
- modify `ui/js/dashboard/sections/system.js`: add `whatsapp: dashIcon('chat')` in `icons` (~155) and `whatsapp: t('dashboard.integration_whatsapp')` in `names` (~176);
- modify `ui/lang/dashboard/<16>.json` (key `dashboard.integration_whatsapp`);
- modify `internal/server/self_test_probes.go:19-24` (add `"integration:whatsapp": handleWhatsAppSelfTestStatus()`);
- modify `internal/server/whatsapp_handlers.go` (the probe handler, plus `whatsAppErrorStatus`: `errors.Is(err, whatsapp.ErrRecipientNotOnWhatsApp)` → `409, "recipient_not_on_whatsapp"`);
- modify `ui/cfg/whatsapp.js` `ERRORS` (`recipient_not_on_whatsapp: () => t('config.whatsapp.err_recipient_not_on_whatsapp')`) and `ui/lang/config/whatsapp/<16>.json`;
- tests: create `internal/server/whatsapp_status_surfaces_test.go` and `ui/whatsapp_dashboard_test.go`; modify `internal/server/whatsapp_handlers_test.go` (one map row).

```go
// handleWhatsAppSelfTestStatus is the read-only probe for integration:whatsapp. It reads
// the passive status only; it never connects, links or sends, so a self-test cannot
// message the owner.
func handleWhatsAppSelfTestStatus() http.HandlerFunc
```

- `healthy` → 200 `{"state":"healthy"}`.
- `disabled`, `not_configured` or `not_linked` → 503 `{"state":…,"error":"whatsapp not configured"}`; `selfTestDiagnosis` reads this as `setup`.
- Any other state → 503 `{"state":…,"error":"whatsapp not connected"}`.

**Strings.** docs-agent writes all 16 locales: real translations, German with "Du" and real umlauts.
- `dashboard.integration_whatsapp`: "WhatsApp". Hindi uses Devanagari, as for Telegram ("व्हाट्सऐप").
- `config.whatsapp.err_recipient_not_on_whatsapp`:
  - en: "This owner number is not registered on WhatsApp. Check the number in the settings."
  - de: "Diese Inhabernummer ist nicht bei WhatsApp registriert. Prüfe die Nummer in den Einstellungen."

**Mirror:** the Telegram tile and labels, and `integration:telegram` in the probe map. Unlike Telegram's probe (`handleTelegramConnectionTest`), the WhatsApp probe never touches the network.

**Tests:**
- `TestDashboardIntegrationFlagsIncludeWhatsApp`: present and false by default; true when `WhatsApp.Enabled` (copy of `dashboard_local_wikipedia_test.go:17-27`).
- `TestSelfTestWhatsAppProbeReadsStatusOnly`:
  - `selfTestIntegrations` of an enabled config holds `integration:whatsapp`, and `bindSelfTestProbes` marks it callable;
  - with `SetWhatsAppConfigState(healthy, …)`, `executeSelfTestProbe(ctx, selftest.Case{Tool: "integration:whatsapp"}, nil)` → `PASS`; with `degraded` → `FAIL`;
  - `s.whatsAppCtl` is a fake whose `SendTest` calls `t.Fatal`;
  - restore the status afterwards; no `t.Parallel`.
- `TestWhatsAppHandlersUseNoStoreAndMapErrors`: add the row `whatsapp.ErrRecipientNotOnWhatsApp: {http.StatusConflict, "recipient_not_on_whatsapp"}`.
- `TestDashboardWhatsAppTileIsWired` (ui): the dashboard JS contains both new `system.js` lines; `ui/cfg/whatsapp.js` contains the `ERRORS` entry; all 16 `lang/dashboard` and `lang/config/whatsapp` files hold non-empty values for the two keys (pattern of `ui/local_wikipedia_dashboard_test.go`).

Commands:
- `CGO_ENABLED=0 go test ./internal/server/ -run '(TestDashboardIntegrationFlagsIncludeWhatsApp|TestSelfTestWhatsApp|TestWhatsAppHandlers)' -count=1`
- `go test -count=1 ./ui/... -run '(Precision|Config|I18N|TestDashboardWhatsApp)'`
- `node --check ui/js/dashboard/sections/system.js` and `node --check ui/cfg/whatsapp.js`
- `npm run build:ui -- --check` is not needed: `scripts/build-ui-bundles.js` bundles only desktop parts. No browser smoke: the static test covers the wiring.

**GitNexus impact:** `dashboardIntegrationFlags` (callers: the overview handler and `desktop_sysworld_facts.go`, which gets `integration:whatsapp` for free), `selfTestProbes`, `whatsAppErrorStatus`, `renderIntegrations` (JS).
**Commit:** `feat(dashboard): show WhatsApp in integrations and the self-test`

## Task 10 — Audit and DOX

**Files:**
- modify `internal/audit/audit_manifests.go` (`MessagingIngressManifest`, after `telegram`) and `internal/audit/audit_manifests_test.go` (`required` map, ~1664);
- modify `internal/whatsapp/AGENTS.md`, `internal/server/AGENTS.md` and the root `AGENTS.md:248` (routing row).

```go
{
	Channel: "whatsapp", SourcePath: "internal/whatsapp/pipeline.go",
	Policy: IngressHighThreatBlock, QuarantineDelivery: IngressQuarantineNone, WrapsExternalData: true,
},
```

The test row is `"whatsapp": {IngressHighThreatBlock, IngressQuarantineNone, true, "internal/whatsapp/pipeline.go"}`.

**DOX** (docs-agent).

`internal/whatsapp/AGENTS.md`:
- **Ownership:** add `cache.go`, `inbound.go`, `format.go`, `chunks.go`, `send.go`, `broker.go` and `pipeline.go`.
- **Contracts:**
  - the filter order (steps 2–8, as in Task 3);
  - owner resolution (phone, then a phone `SenderAlt`, then `Store.LIDs`, then a learned LID) and the self-chat rule;
  - the offline cutoff (`LastSuccessfulConnect − 10 min`, only before `OfflineSyncCompleted`, with the Q1 reason);
  - the echo cache shared by filter and sender, and dedupe of accepted IDs only;
  - the single sender (guard, Scrub, 4000-rune chunks, self-chat prefix per chunk, 20/min with burst 5, enforced by `TestOnlyTheSenderCallsSendMessage`);
  - pipeline parity with `processUpdate`, including `AllowOperator: true` and `QuestionSourceWhatsApp`;
  - workers (5 slots, drained before the store closes);
  - the `dropped_filtered` reason codes.
- Update **Stage scope**: text now flows; stage 3 adds media, `send_whatsapp` and notifications. Note for stage 3: `internal/whatsapp` now imports `internal/agent`, `internal/tools`, `internal/commands` and `internal/telegram`, so `internal/tools` must reach WhatsApp through a registered hook, never an import (that would be a cycle).
- **Verification:** list the new test names.

`internal/server/AGENTS.md`, WhatsApp Runtime Contract:
- `whatsAppAgentDeps` and the agent dependencies passed to `whatsapp.Start`;
- the `integration:whatsapp` probe, which reads status only;
- the `recipient_not_on_whatsapp` code in the fixed-code list;
- the dashboard flag.

Root `AGENTS.md:248`: append "text channel (inbound filter, agent pipeline, broker, send queue), dashboard tile".

**DOX pass report**, to include in the commit message body:
- Unchanged, `internal/agent/AGENTS.md`: no contract lists chat sources; `:714` already requires `QuestionSource*` on every `CompleteQuestion`.
- Unchanged, `internal/security/AGENTS.md`: no ingress list.
- Unchanged, `internal/tools/AGENTS.md`: no QuestionSource contract.
- `ui/AGENTS.md:586-591`: unchanged unless it enumerates error codes; if it does, add `recipient_not_on_whatsapp`.
- Unchanged, `internal/qrcode`, `internal/fileutil` and `licences` guides: not touched.

`CGO_ENABLED=0 go test ./internal/audit/ -run '(TestMessagingIngressManifestDeclaresEachIngressPolicy|WhatsApp)' -count=1`

**GitNexus impact:** `MessagingIngressManifest`.
**Commits:**
- `test(audit): declare the WhatsApp messaging ingress`
- `docs(whatsapp): document the stage-2 text channel contracts` (stage the `AGENTS.md` files with `git add -f`)

## Task 11 — Real-number check (user; checklist only)

- [ ] Dedicated mode: send text from the owner phone → AuraGo answers. The reply comes from the bot number, is formatted (bold, italic, strike, code, a list, a table as bullets, a link as "text (url)") and has no link preview.
- [ ] A long answer (over 4000 characters, with a code block) arrives in order as several messages, and every code block renders.
- [ ] Blue ticks appear only with `send_read_receipts: true`, not in `self_chat` mode.
- [ ] "typing…" shows during the run and clears at the end. If it does not show while presence is `unavailable`, record that only: switching presence affects phone notifications and needs a user decision.
- [ ] Self-chat mode on a personal number: a note to yourself gets an answer with the prefix. Messages in other chats, and from contacts, trigger nothing, and the status shows `not_self_chat` drops.
- [ ] `/help` and a pending `question_user` reply work. A forwarded message is treated as third-party content.
- [ ] A message from a stranger gets no reply, and `dropped_non_owner` increases.
- [ ] Several messages sent while AuraGo is stopped for more than 10 minutes: after a restart, those older than 10 minutes before the reconnect are not executed (`dropped_filtered.stale`); newer ones are.
- [ ] The dashboard shows the WhatsApp tile. The self-test `integration:whatsapp` passes while healthy and sends nothing.
- [ ] A test message to an owner number that is not on WhatsApp shows the new "not registered" text in the Config panel.
- [ ] Forward a plain text and a newsletter post from the phone to AuraGo. Both arrive marked as forwarded and run no slash command.
- [ ] Trust boundary: owner resolution is only as strong as the server's `sender_pn`/`sender_lid` attributes, and whatsmeow writes the LID mapping before the filter runs (last writer wins). On the device, check that a stranger's message never resolves as the owner.

---

## Stage verification sweep (test-agent, end of B4)

```bash
CGO_ENABLED=0 go build ./...
CGO_ENABLED=0 go vet ./internal/whatsapp/ ./internal/server/ ./internal/integrationstatus/
CGO_ENABLED=0 go test ./internal/whatsapp/ ./internal/integrationstatus/ -count=1
CGO_ENABLED=0 go test ./internal/server/ -run '(WhatsApp|Dashboard|SelfTest)' -count=1
CGO_ENABLED=0 go test ./internal/agent/ -run '(WhatsApp|MeshCore|QuestionAnswer|TaskRouter)' -count=1
CGO_ENABLED=0 go test ./internal/prompts/ -run '(TestBuildSystemPromptLabels)' -count=1
CGO_ENABLED=0 go test ./internal/audit/ -run '(MessagingIngress|WhatsApp|RouteContract)' -count=1
CGO_ENABLED=0 go test ./internal/telegram/ -run '(Forwarded|Guardian)' -count=1
go test -count=1 ./ui/... -run '(Precision|Config|I18N|Dashboard)'
node --check ui/js/dashboard/sections/system.js && node --check ui/cfg/whatsapp.js
```

The `internal/telegram` line guards the exported `ForwardedContentMarker` that the pipeline now reuses. Report the PASS counts. Then run `node ../../.gitnexus/run.cjs detect-changes --scope compare --base-ref main --repo .` for the regression review; `partial` or `truncated` means run it again.

## Risks

- **Typing while presence is `unavailable`** cannot be decided from the source; Task 11 checks it. The fallback (presence `available` during a run) would cost phone notifications, so it needs a user decision before any change.
- **Clock skew** affects only messages that arrive during an offline sync (Task 8 gate). Live messages are immune.
- **Concurrency:** workers can reorder two quick owner messages before the turn lock, exactly like Telegram's worker pool. The turn lock keeps history consistent.
- **Stage 3 import cycle:** `internal/whatsapp` → `internal/agent` and `internal/tools`. `send_whatsapp` and notifications must reach the Bot through a hook registered by the server (recorded in DOX, Task 10).
