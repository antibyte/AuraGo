# HomeDash M3 frontend hand-over

Branch: `claude/homedash-m3-frontend`, on top of `2676770e5` (M0 to M2). Plan: `docs/superpowers/plans/2026-11-25-homedash-m3-frontend.md`. Backend hand-over: `m3-handover-backend.md` on `claude/homedash-m3-backend-47625a`.

## State (2026-11-26)

DONE, not merged, not pushed, not deployed. The engine (`ui/js/homedash/`) owns the interaction layer, so the kiosk (M6) inherits it.

## What M3 frontend delivers

- P1 interaction layer: one command client per realm, a `request_id` per action, optimistic overlay, pending after 250 ms, confirmation with late adoption, 6 s rollback cue, 1250 ms throttle, 400 ms keyboard send, arming, read-only reasons, ARIA, focus and reduced motion.
- W1 controls in all four families: switch push and tile, light slider, fader and tile, the M1 dial on `brightness_pct`, action button, select (segmented or list), number (stepper or slider), and the light more-info sheet.
- M2 carry-overs (first values within 1.5 s, Instrument numeral token, header icons), the `home_assistant` template flow and per-source notices.
- Browser tests against the M3 fixture (`ui/desktop_homedash_fixture_m3_test.go`).

## Commits (oldest first)

| Step | Commits | Notes |
| --- | --- | --- |
| Batch A | 11e525b17 icons, d0a562c89 api, 09764fa6d store/stream, 9207a10a3 texts, 50fbfc8d3 input, a80e98e8e command | fix e58c2c666 |
| Batch B | fbda29bc2 control kit, fe07c6f16 primitive input, b61115bf0 trap/popover/sheet, 4231bf99b render/view/app | fix 7c01d9801 |
| Batch C | b16da24ec switch, b4547cccd light, 5045c0c61 action, c2c249466 select, c70c6fc40 number | fix 899499f9f |
| Batch E | 706c5d01c host, 749ff3c9f fixture, 513da87f5 controls browser test | c7139bf57 linger/PUT fix |
| Batch D | 1a6284dd8 structure CSS, 5fa629dff pre-gate fixes, 53cecb72c lab, 0f296edfb W-S fixes, 36116f6dd Glass, 5e8313892 Tactile, 72fa1806f Minimal, 408efd6cf hardened probe, 84fb68d66 Instrument and D8, f57b6fe96 D+E review fixes | |
| Docs and test fix | 38e9d82ac docs; drag-throttle bound (commit follows) | test only |

## Test results (2026-11-26)

- `npm run test:homedash`: 273 of 273 pass. `npm run check:ui` and `go vet ./ui`: ok.
- Targeted ui tests: 14 PASS. The line-budget test fails only on the 8 baseline non-HomeDash files. `go test ./internal/desktop` and `./internal/audit` (shard coverage): ok. Fixture unit tests: 6 of 6.
- `TestDesktopHomeDashBrowser`: PASS, 31.5 s. `TestDesktopHomeDashControlLabBrowser` (default sheets): PASS.
- `TestDesktopHomeDashControlsBrowser`: PASS, 79.5 s. One drag-throttle flake, fixed by measuring the drag duration.

## Gate rounds

- W-S (placeholder structure): failed round 1 with 11 findings (WS-1 to WS-11). Passed round 2.
- W-G (Glass): 2 findings (G-1 chips at 1x1, G-2 dark light wash). Passed round 2.
- W-T (Tactile): 3 findings (T-1 dark on/off, T-2 steel bezel, T-3 hero check). Passed round 2.
- W-M (Minimal): 1 finding (M-1 dark wash plate). Passed after 2 rounds.
- W-I (Instrument): passed round 1.
- W-X (cross-family, real app page): passed.
- P (pending, armed, rollback, focus, motion): passed.

## Deviations

- Dark light-tile wash differs per family. Glass uses a stronger warm wash than the 18 to 40 % formula (W-G). Tactile shows a backlit smoked lens with opacity 0.7 + 0.3 x level (W-T). Minimal uses a flat 58 % mix, with no plate behind the numeral (W-M). Instrument uses the HUD tint, not a warm wash (W-I).
- Icon-only chips at 1x1 for the switch push and the action button. The word stays in the accessible name (A3).
- Segmented capacity: 3 options below width 3, 5 from width 3. More options use the list control (A2).
- Read-only never sets `aria-disabled`. Activation announces the reason (A1).
- Lab flag `splitSizes` on `sheet-light-{family}` in `homedash-lab-sheets.json` gives the two light-sheet sizes (7x6, 4x7) their own sheets.
- M1 family geometry overrides are recorded as approved (I3). They are M1 hero recipes, documented in the visual-system doc, with no code change.

## Contradictions resolved (self-review, 28 items)

- Timing and input: the 250 ms throttle became 1250 ms (D5). A 4 px move splits tap from drag and cancels a hold. Arming replaces confirm dialogs. Errors show an inline cue, not a toast.
- Confirmation: frames count from the send and must match `expected`. A 202 without `expected` confirms at once. Switches send `turn_on` or `turn_off`. A 429 retry gets a new `request_id`.
- Scope and layout: more-info for lights only, no presets, no history (D1). `brightness_pct` everywhere (D6). The popover is anchored at 600 px or more, else a bottom sheet. Overlays carry `hd-widget` and the family (SCR-M3-3).
- Data and files: bootstrap is re-read on home entry, on visibility and after the stream goes live. Banners cover only the sources the open dashboard uses. The host, module-loader entry and host CSS are frontend files. The backend helpers are unexported, so the live test needs its file in `internal/server` (Q1).

## Lead answers Q1 to Q10

- Q1: the live-handler test is a test-only file in `internal/server` (Task 28). No production change under `internal/`.
- Q2: the module-loader `homedash` entry, `TestHomeDashWiring`, `homedash-home.js` and `desktop-app-homedash.css` are frontend files.
- Q3: `--hd-mask-check` joins the `.hd-root` token block. The freeze was regenerated for the approved change.
- Q4: SCR-M3-1 to SCR-M3-7 approved. Any hero regression at W-X is a gate failure.
- Q5: switches send `turn_on` or `turn_off` from the shown state. `toggle` is only the fallback.
- Q6: continuous controls ignore widget `confirm`.
- Q7: a 403 rolls back, marks the entity read-only with its reason, and shows no red cue. `desktop_readonly` also switches the app.
- Q8: banners cover only the sources bound by the open dashboard. `catalog_failed` has its own text.
- Q9: right-click is not a more-info action in M3. Desktop menus come in M4.
- Q10: a segmented control with more than five options renders the list control.

## Engine bug found by the browser test

The store could drop an id that the stream still held. A page shown again inside the 30 s linger got no new PUT and no snapshot,
so it came back empty. Fix in c7139bf57: an unwatched id keeps its data until `stream.acked()` no longer lists it (store rule 8).

## Uncovered contract paths

- Commands: 401, 413, 500 and 504. Also `schema_too_new` and `hub_closed`. The M3 fixture and browser test do not exercise them.

## Known gaps

- No lab cell shows a focused climate step (+ or -). The same focus ring is composed there, but no sheet shows it (Gate P).
- New `role=status` banners may not be announced by every screen reader.
- GitNexus compare stayed partial, because the index cannot be rebuilt here.

## Open items

- D14: live Home Assistant acceptance by the user, or a local HA demo container with the user's OK.
- Kiosk inherits P1 in M6. Sensor sheet in M5. Right-click menus in M4. D10 allowlist question before M5.

## Live-handler test (Task 28)

2026-11-26, after both M3 branches landed on main (89ef81820): `internal/server/desktop_homedash_live_browser_test.go`
(`TestDesktopHomeDashLiveHABrowser`) drives the desktop shell against the real `handleHomeDash` handlers with `hasstest` as Home
Assistant. It creates the `home_assistant` dashboard from the start screen, switches `switch.coffee` on and dims `light.kitchen` by two
arrow keys (one trailing `brightness_pct` send of the demo value + 2). Each widget then confirms with no `data-hd-pending` and status ok.
The fake records exactly one `light.turn_on`. No upstream text and no page errors reach the page. PASS three times (4.6-5.1 s) with
`AURAGO_RUN_BROWSER_SMOKE=1`, SKIP without. No product bug found. Trap: in empty mode the header `create-ha` button stays in the DOM
but hidden, so a selector must target `.hdx-choice[data-action="create-ha"]` (or check `[hidden]` ancestors).

Nothing is pushed or deployed. Both M3 branches are merged into local main.
