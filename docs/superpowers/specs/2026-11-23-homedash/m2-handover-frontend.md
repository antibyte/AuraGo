# HomeDash M2 frontend track — hand-over (session in worktree suspicious-williamson-66fedc)

Branch: `claude/suspicious-williamson-66fedc` (based on main `40bafdfe8`). Plan: `docs/superpowers/plans/2026-11-25-homedash-m2-frontend.md`.
Server contract and backend plan: `docs/superpowers/plans/2026-11-25-homedash-m2-backend.md`, section "API contract (frozen for M2)".

## State (2026-11-25)

**DONE, not merged, not pushed:** the desktop app loads the engine in view-only mode. Dashboards and live entity values come from the
server through SSE and PUT subscriptions; no commands are sent. Commits, oldest first, on top of main `40bafdfe8`:

- `e2d4eeff7` hd-api
- `a258b27d5` tone and glyphs
- `dd2ff5b1e` stream
- `2cbbf385d` store (L2/L3)
- `004a6d966` catalog
- `c63cdbfa9` i18n, 16 locales, L5 test
- `c8f749800` SCR-M2-1/2
- `274d0be3c` page part
- `0b7bfe37d` store dispose fix (Batch A review)
- `4f9f0a6d2` render and testdata
- `ca5281877` sheet
- `c1404a47f` view (L1 refinement)
- `a39c16fc5` hd-app
- `ae0bd6210` Batch B review fixes
- `f889c4a92` home
- `dd408a195` host
- `e6ea14f1f` chrome CSS
- `5573f40b9` wiring
- `f98769b1d` BuiltinApps row and icon
- `83e9bb19b` resume stopped stream after bootstrap
- `732ccfaca` fixture and browser test (L4)
- `8cd30bf72` live read-only flag on retry (Batch C review)
- `b34368f84` gate round 1: duration, tile, decimals (SCR-M2-3)
- `6281705dd` gate round 1: system names
- `d5d2f66ab` gate round 1: choice cards and single focus ring
- `e9f45f780` browser test hardening (Batch D review, G9/G10)
- `8a75af2d2` gate round 2
- `1e7f31db7` gate round 3

Test results at hand-over:

- `npm run test:homedash`: 151/151 pass.
- `npm run check:ui`: ok.
- Wiring, materials and i18n Go tests: ok.
- `go test ./internal/desktop -count=1`: ok.
- `TestDesktopHomeDashBrowser`: PASS, about 30 to 55 s.
- Lead review gate: PASSED in round 4 on fresh screenshots in all four families. Findings are kept locally in the git-ignored
  `reports/homedash-m2/findings/`, not in git.

## Deviations from the plan (one line each, with reason)

- L1 refined: the literal L1 dropped a custom palette's accent, because `applyLook` only writes contrast corrections; the solved accent and ink-on-accent go through `--hd-n-accent` and `--hd-n-ink-on-accent`.
- Polish uses "panel" for a dashboard; "pulpit" is the desktop.
- SCR-M2-3 (decimals) was added at the gate.
- `E.format.duration` (an M0 core helper) was changed at the gate to the largest-plus-next-unit rule.
- Client-side localization of the system entity names: six new keys in 16 locales, added at the gate.
- The uptime tile layout changed inside SCR-M2-1 at the gate.
- App fix: `realm.stream.resume()` after the bootstrap answers, found by the L4 browser steps.
- The browser test goes beyond the plan: L4 revoke and disabled steps, read-only from bootstrap, start-menu screenshots, measured geometry assertions, `<html lang="de">`.
- Batch review decisions: A #2 to #5, B #3, #4, #7, #8 and C #2, #3 were rejected with reasons (see the commit bodies).
- The host CSS neutralises the page-wide focus halo, so the theme focus outline is the only ring.
- The empty-state block is widened to 760 px.

## Contract readings (plan Self-review items 3 to 12, as decided)

- Item 3: the view's `tone` is the channel. The host sets it on the view; widgets keep calling `setTone`. (The design's "through `api.setTone`" is not used by the host.)
- Item 4: the watchdog follows the contract. 45 s without any frame is dead (a heartbeat is an event), not a PUT probe.
- Item 5: jitter is ±20 % (the contract), not the design's 0 to 750 ms.
- Item 6: the backoff resets after a stream has lived 60 s (the contract), not at `hello` as Docker Manager does.
- Item 7: a snapshot covers the ids it lists; ids missing from it are not made unavailable (store rule 1).
- Item 8: a 429 `too_many_streams` looks like any refusal to EventSource. Any failure before `hello` waits at least 5 s and calls `diagnose()` (bootstrap) to find `homedash_disabled` or 401.
- Item 9: the stale banner appears after a 5 s grace, so a 1 s reconnect does not flash it. The browser test refuses two reconnects so the banner shows.
- Item 10: the view's `readonly` applies only to entities with a command capability (store rule 3). A desktop read-only state still locks controllable entities. System sensors are never drawn locked.
- Item 11: the engine computes banners (`E.view.banners`); the desktop host draws them in its chrome; the kiosk (M6) draws its own.
- Item 12: `.hd-more` and the `.hd-sheet` have no production CSS. M2 has no `moreLabel` (more-info is M3). `hd-sheet` serves only the delete confirm, styled by the host CSS under `.hdx-app`.

## Uncovered contract paths (fixture and browser test)

- 401 `unauthorized`; 403 `invalid_bearer_scope`, `desktop_scope_required`, `csrf_check_failed`.
- `schema_too_new` (409, and GET with `schema_too_new: true`).
- 422 `limit_exceeded`; 409 `slug_taken`.
- PUT `dashboard` and its rejections (`limit`, `source_disabled`, `forbidden`), including `revision_conflict` on PUT.
- POST `commands` (404 in the fixture), `snapshot`, `dashboard/duplicate`, `revisions`, `restore`, `approve`, `dashboards/validate`.
- Entity list filters (`source`, `kind`, `area`, `q`, `limit`, `offset`).
- 413 `body_too_large`; 429 `rate_limited`; 500, 502, 504.
- Revoked with `auth_lost`.
- 503 `storage_unavailable` and `hub_closed`.

## Open items

- Sheet styling moves into the engine in M3; today the host CSS styles it under `.hdx-app`.
- Banners for the kiosk page in M6.
- The Instrument wide-sensor readout (M1 fixed 15 px at aspect 1.6 or more) reads small at 5x2; a token change for M3.
- Starter widgets carry no head icons (the template has no `icon`). Accepted as optional; a backend template change could add them.
- The `system` entity names are English in the server contract and localized only in the client. Other clients (kiosk, agent) see English.
- The lab hero fixtures do not exercise `options.decimals`.
