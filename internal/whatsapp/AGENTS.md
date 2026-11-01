# WhatsApp channel

## Purpose

Owner-only WhatsApp channel over `go.mau.fi/whatsmeow` (WhatsApp Web multi-device). AuraGo is a linked companion device named "AuraGo". Stage 1 covers linking, the device store, the connection generation, status and the owner test message. Design: `plans/2026-10-31-whatsapp-integration-design.md`. Plan: `plans/2026-10-31-whatsapp-integration-plan.md`.

## Ownership

- `client.go`: `waClient`, `deviceStore`, `backend`, the whatsmeow adapter (`newWhatsmeowClient` is the only `whatsmeow.NewClient` call), the auto-reconnect cap and the linked-device name and props.
- `errors.go`: sentinel errors that the server maps to fixed HTTP codes.
- `store.go`: `<data_dir>/whatsapp/device.db`, modernc SQLite with `_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)` and the `sqlstore` dialect `sqlite`.
- `events.go`: `classifyEvent`, which maps a whatsmeow event to status state, error code, terminal and reconnect policy.
- `runtime.go`: the `Bot` generation (connect loop, backoff, health, owner test message, `Logout`) and `ConfigState`.
- `recipients.go`: `ownerRecipients`, the single owner-recipient resolver, and the test message text.
- `link.go`, `qr.go`: `LinkSession` (one at a time, 3-minute TTL) and QR PNG data URIs through `internal/qrcode`.
- `log.go`, `mask.go`: whatsmeow-to-slog adapter and digit masking.
- Owned elsewhere:
  - `internal/server/whatsapp_runtime.go`, `whatsapp_handlers.go`: lifecycle and HTTP.
  - `internal/integrationstatus/whatsapp.go`: passive status snapshot.
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
  - The store connection runs with `secure_delete`, and local removal (`removeLocalDevice`) deletes the device row, closes the store and then removes `device.db` and its `-wal`, `-shm` and `-journal` sidecars. Verify `local_removal_test.go`.
- **Owner-only sends.** Every send resolves recipients through `ownerRecipients`: dedicated mode sends to `owner_numbers`, self-chat mode to the linked own JID. No API accepts arbitrary numbers. The test message in self-chat mode starts with `self_chat_prefix`.
- **Reconnects have one owner per failure kind.**
  - whatsmeow reconnects `Disconnected` and `KeepAliveTimeout` itself. `AutoReconnectHook` caps its delay at 300 s. `StreamError` only marks the status `degraded`.
  - The Bot owns the first connect, store-open retries, connect errors and `ConnectFailure`, which whatsmeow does not retry. These retry with 5 s backoff doubling to 5 min, without limit. The Bot calls `Disconnect` before each reconnect.
  - `logged_out`, `conflict` (StreamReplaced) and `outdated` (ClientOutdated) wait for the user. `temp_banned` retries after WhatsApp's expiry; without an expiry it waits for the user.
- **Health.** `healthy` needs `OfflineSyncCompleted` on the current connection; a `Connected` that arrives after the sync also counts. A keepalive timeout forgets the sync until `KeepAliveRestored`. A healthy connection sends presence `unavailable` so the phone keeps notifications.
- **Stopped generations stay silent.** Event handlers check the stop flag under `eventMu`, and the connect loop checks its context before each write. A stopped generation never writes status; the callers that stop it (`Logout`, a relink) write the next state themselves.
- **Link sessions.**
  - `LinkRequest.Validate` runs before the store is opened.
  - States: `waiting`, `qr`, `code`, `success`, `timeout`, `error`. A cancellation is `error` with code `cancelled`.
  - Phone linking asks WhatsApp for a pairing code; QR linking renders the payload as a PNG data URI.
- **Privacy.**
  - Debug output is dropped. Other whatsmeow lines are scrubbed and digit-masked, and protocol nodes are replaced by their tag.
  - Phone numbers and JIDs keep only their last four digits (`***1234`). Status holds masked values only.
  - QR payloads, pairing codes and message content are never logged or stored. The browser keeps the QR and the code only in the Config panel's DOM.
- **Stage scope.** Stage 1 counts inbound `events.Message` events (`messages_received`, `last_inbound_at`) and nothing else. Filtering, the agent pipeline and the send queue arrive in stage 2 (Stage 2 in `plans/2026-10-31-whatsapp-integration-plan.md`).
- **Release pin and licence.** whatsmeow is pinned by pseudo-version in `go.mod`; a "client outdated" outage needs a release with a newer pin. The module links GPL-3.0 `go.mau.fi/libsignal`, so distributed binaries are GPL-3.0 combined works (`licences/AGENTS.md`).

## Work Guidance

- Add a whatsmeow call to `waClient` and `fakeClient` (`fake_test.go`) first. Only this package imports whatsmeow.
- Map a new event in `classifyEvent` with a row in `TestClassifyEventMapsStatesAndPolicies` before using it.
- A new SQLite file needs an entry in `config.SQLiteDatabasePaths` or in `internal/audit` `sqliteBackupExclusions`.
- A new route needs an admin entry in the `internal/audit` route manifest; a new outbound host needs a network-client inventory entry.
- Log errors only as `maskDigits(security.Scrub(err.Error()))`.

## Verification

- `CGO_ENABLED=0 go test ./internal/whatsapp/ -count=1`: fake client; store tests use real temp files.
- With `CGO_ENABLED=0 go test`:
  - `./internal/server/ -run 'WhatsApp'`
  - `./internal/config/ -run '(WhatsApp|E164)'`
  - `./internal/tools/ -run 'WhatsApp'`
  - `./internal/audit/ -run '(WhatsApp|RouteContract|SQLiteOpens|NetworkClient)'`
- Browser: `AURAGO_RUN_BROWSER_SMOKE=1 go test -count=1 ./ui -run 'TestWhatsAppConfig'` (the three link-panel tests).
- Real-device linking is manual (stage 1 Task 15 of the foundation plan). Never claim it without a phone.

## Child DOX Index

None.
