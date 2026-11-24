# HomeDash M0 backend: final report

Branch `claude/jolly-jackson-899374` in worktree
`C:\Users\Andi\Documents\repo\AuraGo\.claude\worktrees\jolly-jackson-899374`, based on main `db51fa524`.
Main has since moved to `6b0b77c05` (2 unrelated commits). Not merged, not pushed. Ready to land
(rebase on main, then `merge --ff-only`). Diff against the merge base: 56 files, 43 of them new under
`internal/homedash/`; outside it only config (`homedash` block, template, three Normalize call sites, SQLite
inventory), `internal/tools` (HA wrappers, `haDo` refactor) and the three AGENTS.md edits.

## Commits (16 = 9 task commits + 7 review-fix commits)

| Commit | Content |
|---|---|
| 06dac3b3e | Task 1 config block |
| 3cb7dff35 | Task 2 core types |
| 43878114d | fix: entity ranges narrow fixed bounds; sensitive attribute keys dropped |
| 2a978feb6 | Task 3 widget catalog + dashboard schema (golden JSON) |
| db83f5b78 | Task 4 HA REST helpers + mapping |
| 55f26ab0d | fix (security review): HA raw wrappers GET-only, strict endpoints, 8 MiB cap |
| fb3e7a51b | fix: layout items bounded before marking (DoS); finite sensor numbers |
| e047ff18d | Task 5 hub |
| cec7bb0a4 | Task 6 HA websocket client |
| 9fe1055c0 | Task 7 store (`homedash.db`) |
| 7384a2272 | fix: hub run ordering, catalog sequence/timeout, bounded state cache, removals after rebuild |
| fe5955b91 | fix: HA registry batches cannot overlap; backoff resets only after a stable session |
| 871006399 | Task 8 command pipeline, rate limiter, kiosk allowed set, history ring |
| 155bc816b | fix: store refuses newer-schema updates; expired displays free their slot; cap 0 refuses |
| 0e24356d0 | fix: gates re-checked before the source call; kiosk reach tightened; approval change ends kiosk streams |
| bee8b00fb | Task 9 docs (`internal/homedash/AGENTS.md` force-added, root index row, tools guide) |

## Task 10 verification (final tree)

All green, no failure against the Task 0 baseline (baseline was fully green):
`go build ./...`; `go vet` homedash/config/tools; `go test ./internal/homedash/...` 60 + 13 top-level PASS
(22 gate subtests), 15 stability rounds clean; config, audit, config-merger packages; `TestHA*` 17 PASS; server
config tests 110 PASS; UI template-rules test PASS; golden catalog `ee1d3b09a2c21f7f0096bbac5c51ca48b7952358b2b4264b055d23351f6066d0`,
24 types, 47 variants. Race detector on aurago-test (go1.27.4): `internal/homedash` clean (count 1 + 5 on the
final tree), `sources/hass` clean (count 2, before the last hub/pipeline fixes, which did not touch hass).
Nothing user-visible: no route, no wiring, no UI file changed.

## Contract deviations (exported surface)

- New exported constant `CloseApprovalChanged = "approval_changed"` (hub close reason; lead decision after the
  security review: a kiosk stream ends when its dashboard's approved revision moves).
- `HARequestJSON` keeps its signature but accepts only GET without a body (security review: other methods bypassed
  HA read-only mode and the service policy). `HAGetBytes` requires 1 byte to 8 MiB.
- `RedeemPairing`: "kiosk full" and a cap of 0 return the uniform `pairing_invalid` (plan: `limit_exceeded`,
  and 0 meant unlimited).
- `sourcetest.FakeSource` gained test fields/hooks (`PanicOnCommand`, `OnCommand`, `CatalogDelay`, `CatalogScript`,
  `SetCatalog`, run exit delay); `export_test.go` adds test hooks. No other exported name, signature, JSON tag or
  error code differs from the plan.
- Issue code for an explicit out-of-range `radius_scale`/`row_unit` is `out_of_range` inside an `invalid_document`
  error (amendment A3 says "invalid_document issue").

## Behaviour deviations and lead decisions

- A1 extended to the entity view: one helper `controlBlock` serves the hub view and the command gate.
- Gate order: `kiosk_disabled` comes after principal kind/key, request id and hub closed/HomeDash disabled (A2
  requires it before the allowed-set check; satisfied). A kiosk is refused for an entity outside its set BEFORE
  any lookup (same error for unknown and foreign entities). Switches are re-checked right before the source call.
- A3: field types unchanged; absent keys are defaulted while decoding, so `Normalize` no longer repairs zero
  values: Go-built `Dashboard` values must set `Look.RadiusScale` and `Grid.RowUnit`. `RowUnit.MarshalJSON` writes
  `"auto"` only when `Auto` is true.
- Store timestamps use a fixed-width UTC format (RFC3339Nano broke SQL string comparison).
- `BuildAllowedSet` is stricter: unknown type/variant/undeclared binding contributes nothing.
- Per-entity rate window is per principal kind; empty principal key refused; superseded commands audited as
  `superseded`; failed command results stay cached under their request id for 30 s (the design's "return the
  first result"; confirm).
- Several plan tests were corrected (append aliasing in the migrate test, a racy status read in a hub test, the
  dedupe loop count); `TestCommandLatestWinsSlot` uses a deterministic hook instead of a sleep.

## Deferred to later milestones (from the reviews; decide where they go)

1. Kiosk HTTP milestone: the hub read functions (`Entities`, `Entity`, `History`, `Snapshot`) take no principal:
   kiosk routes must filter by `AllowedSet`, each with a test for a non-allowed entity.
2. Kiosk principal fields (`Allowed`, `ViewOnly`) are supplied by the caller; a hub-side resolver was suggested.
3. `BuildAllowedSet` ignores binding kinds: a crafted but approved document could bind a widget to an entity of
   another kind offering the same capability (unlock/container actions stay kiosk-denied; sensitive needs confirm).
4. HTTP layer must add: per-device limiting of refused kiosk commands, pairing limit 5/min/IP, `RemoteAddr` (not
   forwarded headers) for `last_ip`, and pass `kiosk.max_devices` to `RedeemPairing` (`RuntimeConfig.Limits` has no
   field for it).
5. Save pipeline: `ApplyPatch` alone does not validate; the server's save path must run Normalize, Autopack,
   Validate (documented in the guide).
6. Not fixed (judged non-blocking): `Normalize` turns explicit empty enum strings into defaults; `saved_by` not
   validated in `Validate` (the store overwrites it); `ContainsURL` misses `http:/x` and `www.x`; HA attribute
   allowlist is global, not per domain; cover `stop`/`set_tilt` not marked sensitive for garage covers; `blockedBy`
   label may differ from `HAServiceRefusal` normalisation; revision storage up to ~21 x 256 KiB per dashboard.
7. Pre-existing, outside this plan: existing HA tool functions put response bodies into errors
   (`homeassistant.go` errJSON calls); `readHTTPResponseBody` can overflow for a limit near MaxInt64; gofmt flags
   `internal/config/media_conversion_test.go` and `virtual_computers_test.go`.
8. Design text: `design-backend.md:307` still says a nil `Allowed` means unrestricted (code and plan fail closed);
   backend section 6 lists codes not in the core yet (`desktop_readonly`, `csrf_check_failed`,
   `method_not_allowed`, `body_too_large`) and a `source` field for `source_unavailable`: add with the HTTP layer.

## Not verified

- HA websocket message shapes are the plan's assumptions (fake server); check once against aurago-test in B9b.
- Race run of `sources/hass` predates the last two fix commits (they did not change that package).
- GitNexus index is behind main and cannot see the new package: detect-changes judged only the existing symbols
  (`load`, `Config`, `SQLiteArchivePaths`, `handleUpdateConfig`, `applyConfigPatch`, `haRequestContext`).
