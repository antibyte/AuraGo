# WhatsApp channel

## Purpose

Owner-only WhatsApp channel over `go.mau.fi/whatsmeow` (WhatsApp Web multi-device). AuraGo is a linked companion device named "AuraGo". Stage 1 covers linking, the device store, the connection generation, status and the owner test message. Stage 2 adds the text channel: the inbound filter, the agent pipeline, the reply broker and the one owner-only send queue. Design: `plans/2026-10-31-whatsapp-integration-design.md`. Plan: `plans/2026-10-31-whatsapp-integration-plan.md`. Stage 2 plan: `plans/2026-10-31-whatsapp-2-text.md`.

## Ownership

- `client.go`: `waClient`, `deviceStore`, `backend`, the whatsmeow adapter (`newWhatsmeowClient` is the only `whatsmeow.NewClient` call), the auto-reconnect cap and the linked-device name and props. Stage 2 adds `MarkRead`, `SendChatPresence`, `OwnLID`, `PNForLID` and `LastSuccessfulConnect`.
- `errors.go`: sentinel errors that the server maps to fixed HTTP codes. `ErrRecipientNotOnWhatsApp` is the owner-facing one (409 in the server); the sender wraps whatsmeow's error under it. `errNotOwnerRecipient` is internal.
- `store.go`: `<data_dir>/whatsapp/device.db`, modernc SQLite with `_pragma=foreign_keys(1)`, `_pragma=busy_timeout(5000)` and `_pragma=secure_delete(1)`, and the `sqlstore` dialect `sqlite`.
- `events.go`: `classifyEvent`, which maps a whatsmeow event to status state, error code, terminal and reconnect policy.
- `runtime.go`: the `Bot` generation (connect loop, backoff, health, owner test message, `Logout`), the inbound routing in `handleMessage` (offline cutoff, worker slots, drain on stop) and `ConfigState`.
- `recipients.go`: `ownerRecipients`, the single owner-recipient resolver, and the test message text. The sender adds the self-chat prefix, not this file.
- `cache.go`: `idCache`, the bounded ID caches: the sent-ID echo cache (5000 IDs, 20 min) and the dedupe cache of accepted IDs (2048 IDs, no TTL).
- `inbound.go`: `inboundFilter.accept` (design steps 2–8), owner resolution, the self-chat rule and the fixed drop codes.
- `pipeline.go`: `AgentDeps`, `pipeline.handle` (the `processUpdate` mirror for text), typing and the answer.
- `broker.go`: `whatsAppBroker`, the `agent.FeedbackBroker` for one owner turn.
- `send.go`: `sender`, the one outbound queue (owner guard, `Scrub`, formatting, chunks, prefix, pacing, echo registration, delivery accounting), and `tokenBucket`.
- `format.go`: `toWhatsApp`, the Markdown-to-WhatsApp conversion. `chunks.go`: `chunkText`, the rune-counting, fence-aware splitter.
- `link.go`, `qr.go`: `LinkSession` (one at a time, 3-minute TTL) and QR PNG data URIs through `internal/qrcode`.
- `log.go`, `mask.go`: whatsmeow-to-slog adapter and digit masking.
- Owned elsewhere:
  - `internal/server/whatsapp_runtime.go`, `whatsapp_handlers.go`: lifecycle, agent dependencies and HTTP; `whatsapp_backup.go`: the backup-walk skipper for the store directory.
  - `internal/integrationstatus/whatsapp.go`: passive status snapshot and drop counters.
  - `internal/config/whatsapp.go`: the `whatsapp:` section, E.164 rules, store and protected paths.
  - `internal/tools/protected_paths.go`: agent file-tool protection.
  - `ui/cfg/whatsapp.js`: Config section and link panel.
  - `internal/audit`: route manifests, network-client inventory and the SQLite backup exclusion.

## Local Contracts

- **One device owner per process.** At most one `Bot` or one `LinkSession` uses the device store. A link stops the Bot first. A session closes its client and store before `onFinish` runs, so the next Bot is the only user.
- **The device store is a credential.**
  - `<data_dir>/whatsapp/` is 0700 and `device.db` is 0600, both set by `fileutil.RestrictToOwner` (protected owner-only DACL on Windows).
  - Never in `config.SQLiteDatabasePaths` or backups. `device.db` is a documented entry in `internal/audit` `sqliteBackupExclusions`.
  - `config.WhatsAppProtectedPaths` lists `device.db` with `-wal`, `-shm` and `-journal`. `protectedSystemFilesFromConfig` returns them for agent file tools, outgoing attachments and Compose protection.
  - Shell and Python tools get no WhatsApp-specific deny: they keep exactly the vault's existing boundary (sandbox scope and the Danger Zone `allow_unsafe_host_execution` risk, which applies to `vault.bin` equally). The design's "file tools and the Python sandbox" wording is met through that equal treatment, recorded as a stage-1 deviation.
  - The store connection runs with `secure_delete`, and local removal (`removeLocalDevice`) deletes the device row, closes the store and then removes `device.db` and its `-wal`, `-shm` and `-journal` sidecars. Verify `local_removal_test.go`.
- **Owner-only sends.** Every text goes through `sender.deliverText` (or `deliverToOwners`), which resolves recipients through `ownerRecipients` and refuses any recipient that is not one of its entries. Dedicated mode sends to `owner_numbers`, self-chat mode to the linked own JID. No API accepts arbitrary numbers. In `self_chat` mode every outgoing text starts with `self_chat_prefix`, once per chunk; the test message gets it from the sender too.
- **Inbound filter.** `inboundFilter.accept` runs design steps 2–8 in this order. Step 1, the type switch on `*events.Message` in `run()`, ignores `HistorySync` and every other event. A dropped message counts one fixed code and nothing else happens.
  - Step 2, the chat class: `status@broadcast` → `status`; other broadcast lists → `broadcast`; a newsletter chat or sender → `newsletter`; a group (`IsGroup` or `g.us`) → `group`; a bot sender → `unsupported_sender`.
  - Step 3, the kind. whatsmeow has already unwrapped the message, so the filter unwraps only a hand-built event. No content → `empty`; an edit, pin or revoke (`IsEdit` or `Info.Edit`) → `edit`; a protocol message → `protocol`; a reaction or encrypted reaction → `reaction`; any poll creation, update or result → `poll`. Only the kind is read, never the text.
  - An empty message ID → `empty_id`, because neither echo nor duplicate can be judged for it.
  - Step 4, echo: an ID in the sent cache → `echo`. Step 5, dedupe: an ID already accepted → `duplicate`.
  - Step 6, offline cutoff → `stale` (see below).
  - Step 7 (dedicated) or step 8 (self chat) decides the owner.
- **Owner resolution (dedicated mode).** An own message (`IsFromMe`) → `from_me`. A `s.whatsapp.net` sender is the phone itself. A LID sender resolves, in order, through `SenderAlt` (used only when it is a phone JID, because whatsmeow also fills it with a LID), through `Store.LIDs` (`PNForLID`; an error or an empty result is a miss), and through the owner LIDs learned earlier. Nothing resolves → `unresolved_lid` with one masked WARN. A phone that is not in `owner_numbers` → `non_owner` with one masked INFO line, so each stranger message that reaches this step costs one masked log line. Each accepted owner message teaches its LID (`rememberOwnerLID`), so only LIDs of accepted owners are kept, and a stranger cannot teach one.
- **Self-chat rule (`self_chat` mode).** A message is accepted only when it is `IsFromMe`, the account is known and the chat is the own phone user or the own LID user; the owner is the linked account. An own-device copy (`DeviceSentMeta`) also needs its `DestinationJID` to be the own account, because whatsmeow falls back to the own account as `Chat` when a stanza has no recipient. An empty destination leaves the decision to the chat check. An unparseable or foreign destination is `not_self_chat`, even when `Chat` is the own chat. Messages from contacts never pass.
- **Offline cutoff.** While the connection's offline sync is unfinished (`!synced && !resumeSynced`), a message older than `LastSuccessfulConnect()` minus 10 minutes is `stale`. The reference is whatsmeow's login time, because the Bot's own `Connected` observation is not ordered against message dispatch (`plans/2026-10-31-whatsapp-2-text.md`, Q1). Message timestamps are server time, but `LastSuccessfulConnect` is the local clock: a local clock more than 10 minutes fast drops every queued message, and a clock that runs behind lets old ones through. NTP is assumed. Live messages after `OfflineSyncCompleted` are never compared with the local clock. A keepalive timeout clears `synced` but keeps the finished sync as `resumeSynced` until `KeepAliveRestored`. The flags belong to one login: `observeLogin` clears them when a message or an offline-sync event sees a `LastSuccessfulConnect` it has not seen before (`loginSeen`). That covers whatsmeow's forced reconnect after a failed keepalive, which emits no `Disconnected` and skips `resetConnection`; without it a laptop waking from a long sleep would run the commands queued during the outage.
- **Echo cache and dedupe.** One sent-ID cache (5000 IDs, 20 min TTL) is shared by the filter's echo check and the sender. The sender adds an ID only after `SendMessage` returns, because whatsmeow generates the ID inside the call. whatsmeow's fan-out skips the sending device, so no echo is expected; the cache guards copies that arrive anyway. The dedupe cache holds accepted IDs only (2048, no TTL). A rejected message never enters it, so strangers' traffic in `self_chat` mode cannot evict owner IDs.
- **Single sender.** `sender.deliverText` is the only path to `SendMessage`; `TestOnlyTheSenderCallsSendMessage` allows just `client.go` and `send.go`. In order:
  1. A cancelled context returns its error and counts nothing.
  2. No logged-in client → `ErrNotConnected`.
  3. Owner guard: the recipient must equal one `ownerRecipients` entry. Otherwise a masked WARN, `errNotOwnerRecipient`, nothing sent and nothing counted.
  4. `security.Scrub` runs before `toWhatsApp`. The order matters: a registered secret that holds `|` or `**` would otherwise be broken by the formatter before the scrub could match it. Then `chunkText` splits at 4000 runes minus the self-chat prefix.
  5. The queue is one slot: a channel semaphore held for every chunk of a message. A send that waits for the slot gives up with its context.
  6. Pacing: a token bucket with burst 5 and one token per 3 s (20 per minute).
  7. Per chunk: `SendMessage` with plain `Conversation` text, which carries no link preview. On success the ID enters the echo cache and `MarkWhatsAppOutbound` runs. On failure, unless the context ended, it counts `MarkWhatsAppDeliveryFailure`, logs the masked error and stops the message. `ErrRecipientNotOnWhatsApp` is wrapped so the server can map it to 409.

  `deliverToOwners` sends to every owner, continues after a failure and returns the first error.
- **Pipeline parity.** `pipeline.handle` mirrors `telegram.processUpdate` for text. In order:
  1. Config: the generation's snapshot. A nil config, egg mode or a changed `whatsapp` section drops the message (`no_config`, `egg_mode`, `config_changed`).
  2. **Owner re-check:** `in.Owner` must equal an `ownerRecipients` entry before anything else. The filter is not the only gate; a message that fails this check is dropped with a WARN.
  3. Read receipt for each message the pipeline handles, media included, only in dedicated mode with `send_read_receipts`.
  4. Budget tracking and the session. A session other than `default` gets no history manager.
  5. Text: top-level `Conversation`, else `ExtendedTextMessage.Text`. A message without text logs its kind (`image`, `audio`, `video`, `document`, `sticker`, `location`, `contact` or `other`) at INFO and starts no turn.
  6. Forwarded (`IsForwarded`, `ForwardingScore > 0` or `ForwardedNewsletterMessageInfo` set): the text gets `telegram.ForwardedContentMarker`, no HumanInput, no slash command and no pending-question answer.
  7. A pending `question_user` takes the reply first, with `Source = tools.QuestionSourceWhatsApp`. Otherwise, or when the message is forwarded, the owner gets "Please reply with one of the listed numbers."
  8. The session turn lock, then slash commands with `AllowOperator: true`, then the Guardian (ThreatHigh blocks), then `security.IsolateExternalData`.
  9. History and the agent loop with `MessageSource: "whatsapp"` and `agent.WithVoiceOutputSuppressed` (see stage scope).
  10. The answer: `StripThinkingTags`, then each Markdown image becomes an `[Image: caption]` line, then the sender.

  Logs carry the owner's masked JID, message kinds and drop reasons, never message text.
- **Workers.** `handleMessage` runs the filter inline on whatsmeow's handler queue, under `eventMu`; a stopped generation counts nothing and starts nothing. Each accepted message gets one goroutine that takes one of five slots (`pipelineWorkers`, Telegram's default) or gives up when the generation ends. `run()` cancels the generation and waits for the workers before it closes the store. Two quick messages can therefore finish out of order; the session turn lock keeps the history consistent.
- **Broker and typing.** `whatsAppBroker` sends `question_user`, `budget_warning` and `budget_blocked` at once. `tool_start`, `error_recovery`, `api_retry` and `progress` are status updates, at most one per 10 seconds; the others are dropped. `audio`, `youtube_video` and every other event are ignored until stage 3. The broker logs nothing; the sender logs a failed delivery with its label and masked error only. Typing is `composing` at the start, refreshed every 8 seconds, and one `paused` when the run ends (not after a cancel). Whether the phone shows typing while presence is `unavailable` stays open until the Task 11 check.
- **Drop counters.** `dropped_non_owner` and `dropped_unresolved_lid` keep their own fields. Every other filter code is counted in `dropped_filtered`, keyed by the code: `status`, `broadcast`, `newsletter`, `group`, `unsupported_sender`, `empty`, `empty_id`, `edit`, `protocol`, `reaction`, `poll`, `echo`, `duplicate`, `stale`, `from_me` and `not_self_chat`. `messages_received` counts every `events.Message` before the filter, so dropped messages count there too. Counters survive generations. Pipeline-level drops are logged, not counted.
- **No valid owner.** In dedicated mode `ConfigState` reports `not_configured` unless at least one `owner_numbers` entry survives `config.NormalizeE164`; a Bot without one would drop every message as a stranger's. Self-chat mode needs no owner number.
- **Stopped generations stay silent.** Event handlers check the stop flag under `eventMu`, and the connect loop checks its context before each write. A stopped generation never writes status; the callers that stop it (`Logout`, a relink) write the next state themselves.
- **Link sessions.**
  - `LinkRequest.Validate` runs before the store is opened.
  - States: `waiting`, `qr`, `code`, `success`, `timeout`, `error`. A cancellation is `error` with code `cancelled`.
  - Phone linking asks WhatsApp for a pairing code; QR linking renders the payload as a PNG data URI.
- **Health.** `healthy` needs `OfflineSyncCompleted` on the current connection; a `Connected` that arrives after the sync also counts. A keepalive timeout forgets the sync until `KeepAliveRestored`. A healthy connection sends presence `unavailable` so the phone keeps notifications.
- **Reconnects have one owner per failure kind.**
  - whatsmeow reconnects `Disconnected` and `KeepAliveTimeout` itself. `AutoReconnectHook` caps its delay at 300 s. `StreamError` only marks the status `degraded`.
  - The Bot owns the first connect, store-open retries, connect errors and `ConnectFailure`, which whatsmeow does not retry. These retry with 5 s backoff doubling to 5 min, without limit. The Bot calls `Disconnect` before each reconnect.
  - `logged_out`, `conflict` (StreamReplaced) and `outdated` (ClientOutdated) wait for the user. `temp_banned` retries after WhatsApp's expiry; without an expiry it waits for the user.
- **Privacy.**
  - Debug output is dropped. Other whatsmeow lines are scrubbed and digit-masked, and protocol nodes are replaced by their tag.
  - Phone numbers and JIDs keep only their last four digits (`***1234`). Status holds masked values only.
  - QR payloads and pairing codes are never logged or stored. The browser keeps the QR and the code only in the Config panel's DOM.
  - Message text is never logged or put in status. The pipeline stores it in the session history through `InsertMessage`, as Telegram does; the `HumanInput` event ID becomes an opaque key (`personalityObservationKey`), so the chat number is not stored in clear as a turn ID.
- **Release pin and licence.** whatsmeow is pinned by pseudo-version in `go.mod`; a "client outdated" outage needs a release with a newer pin. The module links GPL-3.0 `go.mau.fi/libsignal`, so distributed binaries are GPL-3.0 combined works (`licences/AGENTS.md`).
- **Stage scope.** Stage 2 (text) is in: the inbound filter, the agent pipeline, the broker and the send queue. Inbound text comes only from `Conversation` and `ExtendedTextMessage.Text`; a media-only message passes the filter, logs its kind and starts no turn. Quoted replies get no `[Reply to: …]` prefix yet. Stage 1's inbound counting stays in `handleEvent`, before the filter. Stage 3 adds media in both directions, `send_whatsapp`, voice output (it removes `agent.WithVoiceOutputSuppressed` and `VoiceOutputActive: false` together), notifications and the broker's audio and YouTube capture. The quoted-reply prefix is stage 3 Task 2 of the master plan.
  - Stage 3 import cycle: `internal/whatsapp` already imports `internal/agent`, `internal/tools`, `internal/commands`, `internal/media` and `internal/telegram`. It imports `internal/telegram` only for `telegram.ForwardedContentMarker` (`pipeline.go`), as `internal/discord` already imports it for `telegram.OriginLabel`. Therefore `internal/tools` must never import `internal/whatsapp`. Stage 3 reaches the Bot from `send_whatsapp` and notifications through a hook the server registers, never through an import, or the build cycles.

## Work Guidance

- Add a whatsmeow call to `waClient` and `fakeClient` (`fake_test.go`) first. Only this package imports whatsmeow.
- Map a new event in `classifyEvent` with a row in `TestClassifyEventMapsStatesAndPolicies` before using it.
- A new SQLite file needs an entry in `config.SQLiteDatabasePaths` or in `internal/audit` `sqliteBackupExclusions`.
- A new route needs an admin entry in the `internal/audit` route manifest; a new outbound host needs a network-client inventory entry.
- Log errors only as `maskDigits(security.Scrub(err.Error()))`.
- Send text only through `sender.deliverText`. A new drop reason is a constant in `inbound.go` with a test row, and it is counted only through `integrationstatus.CountWhatsAppDrop`.
- Keep the owner re-check first in `pipeline.handle`; `TestPipelineIgnoresInboundThatIsNotFromAnOwnerRecipient` must stay green.
- A new inbound kind is handled in `kindDropReason` (dropped) or `messageText` (text). Never log its text.

## Verification

- `CGO_ENABLED=0 go test ./internal/whatsapp/ -count=1`: fake client; store tests use real temp files. The stage 2 tests:
  - `cache_test.go`: `TestIDCacheExpiresAfterTTL`, `TestIDCacheEvictsOldestBeyondCapacity`.
  - `inbound_test.go`: `TestInboundResolvesOwnerByPhoneAltStoreAndMemory`, `TestInboundDropsUnresolvedAndNonOwnerSendersWithoutContent`, `TestInboundDropsChatsAuraGoNeverServes`, `TestInboundDropsNonConversationKinds`, `TestInboundAcceptsEphemeralAndViewOnceContent`, `TestInboundEchoDedupeAndOfflineCutoff`, `TestInboundSelfChatAcceptsOnlyTheOwnChat`, `TestInboundSelfChatRejectsDeviceSentToContact`.
  - `format_test.go`: `TestToWhatsAppConvertsMarkdown`, `TestToWhatsAppNeverTouchesCode`, `TestToWhatsAppTurnsTablesIntoBullets`.
  - `chunks_test.go`: `TestChunkTextEdgeCases`, `TestChunkTextKeepsBlankLinesInsideFences`, `TestChunkTextPrefersParagraphThenLine`, `TestChunkTextReopensCodeFences`, `TestChunkTextHardSplitsLongLinesOnRunes`.
  - `send_test.go`: `TestSenderRefusesNonOwnerRecipients`, `TestTokenBucketAllowsBurstThenTwentyPerMinute`, `TestSenderRegistersSentIDsAndCountsFailures`, `TestSenderQueueWaitHonoursContext`, `TestSenderScrubsBeforeFormatting`, `TestSenderSurfacesRecipientNotOnWhatsApp`, `TestSenderPrefixesEveryChunkInSelfChat`, `TestOnlyTheSenderCallsSendMessage`.
  - `broker_test.go`: `TestBrokerDeliversQuestionsAndBudgetNoticesImmediately`, `TestBrokerCoalescesStatusToOnePerTenSeconds`.
  - `pipeline_test.go`: `TestPipelineRunsTheAgentInTheDefaultSession`, `TestPipelineAnswersAPendingQuestionFirst`, `TestPipelineRunsSlashCommandsWithOperatorAllowed`, `TestPipelineMarksForwardedMessagesAndSkipsHumanInputCommandsAndAnswers`, `TestPipelineBlocksHighThreatInputThenIsolates`, `TestPipelineReadReceiptsOnlyInDedicatedModeWhenEnabled`, `TestPipelineTypesWhileRunningThenPauses`, `TestPipelineStartsNoTurnWithoutTopLevelText`, `TestPipelineIgnoresInboundThatIsNotFromAnOwnerRecipient`, `TestPipelineLogsAMessageDroppedForAReplacedGeneration`, `TestPipelineEndsTypingAndReleasesTheSessionAfterAnAgentPanic`, `TestAppendImageLinesFoldsCaptionsOntoOneLine`.
  - `runtime_test.go`: `TestBotRoutesOwnerMessagesToThePipeline`, `TestBotOfflineCutoffAppliesOnlyDuringOfflineSync`, `TestBotForcedKeepAliveReconnectKeepsTheOfflineCutoff`, `TestBotStopWaitsForPipelineWorkers`, `TestBotStaleGenerationRoutesNothing`, and the stage 1 `TestBotSendTestMessageReachesOnlyOwners`, which stays green.
- With `CGO_ENABLED=0 go test`:
  - `./internal/server/ -run 'WhatsApp'`
  - `./internal/config/ -run '(WhatsApp|E164)'`
  - `./internal/tools/ -run 'WhatsApp'`
  - `./internal/audit/ -run '(WhatsApp|RouteContract|SQLiteOpens|NetworkClient)'`
  - `./internal/integrationstatus/ -run '(TestWhatsApp)'` (stage 2: `TestWhatsAppDropCountersAreSeparatedAndCopied`)
- `go test -count=1 ./ui/... -run 'TestDashboardWhatsApp'` (stage 2: `TestDashboardWhatsAppTileIsWired`).
- Browser: `AURAGO_RUN_BROWSER_SMOKE=1 go test -count=1 ./ui -run 'TestWhatsAppConfig'` (the three link-panel tests).
- Real-device linking is manual (stage 1 Task 15 of the foundation plan). Never claim it without a phone. The stage 2 real-number checks are in `plans/2026-10-31-whatsapp-2-text.md`, Task 11; nothing in this file claims them.

## Child DOX Index

None.
