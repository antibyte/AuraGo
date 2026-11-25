# HomeDash M3 backend: hand-over

Branch `claude/homedash-m3-backend-47625a`, base `b8c54c716` (local main then; includes SSE fix `954282cb8`). Local main has since moved (`a095fbe86`).
Status: **not merged, not pushed, not deployed.**

## Commits (oldest first)
| Task | Commit | Content |
|---|---|---|
| 1 | 706e24fce | `source` on `source_unavailable`, audited as failed |
| 2 | bd7699ee6 | raw catalog read, icon names, optional `Widget.Icon` |
| 3 | 00df9a1eb | typed `tools.HACallServiceStates`, classified errors |
| 4 | 6feb0156c | importable fake Home Assistant (`hasstest`) |
| B1 | b9512e01f | scrub whole HA answer before log excerpt cut |
| 5 | 1eb16c1dd | catalog mapping, one-shot websocket fetch |
| 6 | 725f4b6bf | adapter: state stream, reconnect, REST fallback |
| 7 | 7ffcefdf2 | commands: verb mapping, `expected`, changed states |
| 8 | 01a0d1608 | history, camera snapshot, opt-in live test |
| 9 | 787b7ed6e | server source row, settings, network inventory |
| B2 | 8c8ba810d | status precedence, fake failure paths |
| 10 | b3229a2e7 | `home_assistant` template, non-blocking bootstrap |
| 11 | 59e039937 | first metrics sample at start, snapshot rates |
| 12 | 99cf81753 | `revoked shutdown` before HTTP drain |
| 13 | 301784495 | HA end-to-end test with the fake HA |
| B3 | 94f9cbfa7 | stream counter, template page ids |
| 14 | – | security review, no fix commit (see open items) |
| 15 | 555a3b62d | docs: adapter, templates, shutdown, typed call |
| 16 | (this commit) | `docs(homedash): M3 backend hand-over` |

## Verification (2026-11-25, Windows)

- `go build ./...` ok; `go vet` (homedash, server, audit, tools) clean; `go test ./internal/homedash/... ./internal/config/ ./internal/audit/... -count=1` ok.
- `go test ./internal/tools -run '(?i)(TestHA)' -v`: **51 PASS**, 0 FAIL (baseline 46).
- Server subset (HomeDash, Metrics, HTTPDrain, BackgroundTasks, ServeWithShutdown, FailedListenerBind, DesktopReadonly, BluetoothLive): **82 PASS**, 0 FAIL.
- Full `internal/server` (`-timeout 40m -skip Browser`, 559 s): one failure, `TestSIPSpeechSynthesisStreamScrubsSecretsAcrossChunkBoundaries` (SIP TTS, unrelated, load-flaky, also on base); rerun alone PASS.
- Untouched-files diff vs `b8c54c716` empty; `hasstest` absent from `go list -deps ./cmd/...`.
- **Race run: NOT run.** No cgo/gcc locally; reported as not possible per lead instruction; aurago-test not used. Flake repeats instead, all green: hass stream `-count=10`, hub status `-count=20`, shutdown `-count=10`, HA e2e `-count=10`.

## API contract

- No deviation from "API contract (frozen for M3)"; no route, field, code or frame renamed or removed.
- A configured but unusable `home_assistant.url` shows `unavailable`/`invalid_url` from a run; a running source's own `unavailable` reason is never overwritten by a concurrent `catalog_failed`; a catalog read while nothing runs still reports `catalog_failed` (both `unavailable`).
- `update_widget` merges `icon` like `title`; climate advertises `history` but answers `points: []` in M3; `home_assistant` page ids are ≤ 28 chars incl. suffix, `unassigned` reserved for the last page.

## Differences from the design (M3)

1. Service calls use typed `tools.HACallServiceStates`, not `HACallServiceContext` (D3).
2. Catalog reads without a live run use a one-shot websocket session (states + registries), else `GET /api/states` without registries.
3. Camera snapshots not scaled in M3 (widget and scaling in M5).
4. Degraded stale marking applies to subscribed entities only.
5. Widget `icon` is a new optional document field (D8).
6. New HTTP code `template_empty` (422).
7. Bootstrap lists `home_assistant` from the cached catalog, never blocks; client re-reads bootstrap before showing templates.

## Contradictions resolved (plan self-review, 24 items)

- Schema: optional `Widget.Icon` (1); `sources` stays an array (16); `source_unavailable` audited as `failed` (17) and now sent (24); no camera scaling (18).
- Adapter: `hasstest` beside the untouched B9a fake (2); typed sibling for D3 (3); registries via one-shot fetch (4); catalog before updates, state frames after full lists, catalogs only on meta-fingerprint change (5, 6); `updated_at` = HA `last_updated`, `expected` never stored in hub (6); times `.UTC()` (23); no history request without history capability (7).
- Policy: `color_temp` grid anchored at `min` (8); `hs_color` as `[]any` (9); HA service policy decides, not HomeDash switches (14); all domain capabilities mapped, `expected` only for W1 (20); stale marking subscribed-only (19); source-spec test and starter icons updated (21, 22).
- Bootstrap and answers: cached catalog plus background refresh (13); `omitted` always present, counts only cap cuts (15).
- Shutdown and metrics: `endHomeDashStreams` closes the hub, waits up to 2 s before `beginHTTPDrain` (10); first sample not broadcast (11); snapshot carries ring rates, `advance=false` (12).

## Answers to "Questions for the lead" (as given)

- Q1 and Q2 accepted: optional `Widget.Icon` (absent means the entity's `icon_hint`); cached bootstrap with background refresh, client re-reads before template choices.
- Q3 decided: contract tolerances (brightness ±1, colour temp ±2 %, hs_color ±4/±4) plus late adoption after the 8 s window.
- Q4 and Q6 accepted: template qualification follows the HA service policy only; no camera scaling in M3.
- Q5 accepted: `omitted` always present; `422 template_empty`.
- Q7 accepted: every domain command capability mapped; `expected` only for W1; sensitive verbs keep `confirmed:true`.
- Q8 accepted: `hasstest` is frozen surface (`DemoOptions()`, contract §2 ids).
- Q9 accepted: 5 min polling window, then one websocket attempt; failing REST poll returns to websocket backoff.
- X1 accepted: slow HA answer gives `502 command_failed`, no connection or rejected token gives `503 source_unavailable` (test added). X2 accepted: unusable URL gives `unavailable`/`invalid_url`; one-line `server.go` edit.

## Lead decisions during execution

- Factory: empty URL/token gives disabled; non-empty unusable URL gives an adapter that reports `invalid_url` without network; settings need only a non-empty URL.
- `update_widget` merges `Widget.Icon` (empty never clears); `HACallServiceStates` does not lowercase domain/service; `entityFor` sets `readonly_reason` to the first `blocked_by`; adapter run generation counter; `Command` answers `entity_not_found` outside known full lists; template name/title validated before catalog read; review fixes B1-B3 (commits table).

## Frontend live-handler test (D13)

- Test lives in an `internal/server` test file or a test binary (no exported harness; frontend plan chooses).
- `ha := hasstest.New(t, hasstest.DemoOptions())`; `env := newHomeDashHAEnv(t, ha)` (`internal/server/homedash_ha_e2e_test.go:20`) sets `home_assistant.enabled/url/access_token` from `ha.URL`/`ha.Token` and `homedash.sources.home_assistant` {enabled, control, poll 5}; cleanup checks no websocket is left.
- Requests go through `handleHomeDash(env.s)` (`internal/server/homedash_routes.go:84`), demo ids per contract §2. Failure switches: `BlockWebsocket`, `DropWebsockets`, `FailServices(status, body)`, `NoEvents`, `NoChangedStates`, `FailStates`, `FailRegistries`, `Close()`.

## Open items

- M5: camera scaling, climate history, sensor more-info sheet, rest of icon set (~70 names).
- Before M6: kiosk decision; kiosk gate denies only `unlock` and container actions, per-entity covers/scripts stay allowed with `confirmed:true`.
- D10: non-empty `home_assistant.allowed_services` is a global allowlist; ask the user before M5 W3 (covers, locks).
- Shutdown: `Hub.Close` may take up to 10 s, plus 2 s stream grace. Race run still to do on a cgo-capable Linux host.
- Nothing is pushed or deployed.
