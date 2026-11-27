# HomeDash M4 hand-over (designer)

Branch: `claude/homedash-m4-designer`. Base: `ac7a11c7f` (M0 to M3 on local main). Plan: `docs/superpowers/plans/2026-11-26-homedash-m4-designer.md`. Decisions: `m4-decisions.md` (D1 to D15, kept outside the repository). Engine guide: `ui/js/homedash/AGENTS.md`, "Designer (M4)". Apps guide: `ui/js/desktop/apps/AGENTS.md`, "HomeDash".

## State

Built and gated locally. Not pushed, not deployed. The gate verdicts are the lead's; this file lists what the gate runs recorded.

## What shipped

- **Edit mode** over an open dashboard: a 48 px edit bar (pages, lg/md/sm breakpoint radio, zoom toggle, save chip, Look, More, drawer toggles), the conflict bar, and the host bar hidden in design mode. Enter with Ctrl+E, the edit button, or the empty-page action.
- **Canvas**: move, resize with 8 handles (12 px hit, 44 px coarse), marquee, multi-select, guides, auto-scroll, a FLIP settle after a drop, cancel by Escape, and roving focus. The keyboard map of contract C14 is complete.
- **Palette and inspector**: a widgets tab with live thumbnails and an entities tab (paged, live state for at most 20 visible rows). Drag or tap to add, entity drop to rebind. The inspector covers Look, Data, Actions and Options, and a multi-selection (count, family, align, delete). Below 900 px of root width both become drawers.
- **Look editor**: Surface (Background, Corner rounding) and Grid (Spacing, Size on screen, Row height, Close gaps on this page). Family cards show live previews. The custom palette is saved as the server's `look.palette.custom` with a snapshot in theme mode.
- **Pages**: add, inline rename, icon, duplicate, move, delete (confirm when not empty; the last page is not deletable), drag reorder, Alt+Shift+Left/Right. md and sm overrides materialise on the first edit and reset to automatic.
- **Dashboard management**: new, rename, duplicate, delete, import and export, from the window menus and the context menus. Import is also on the home screen.
- **Save lane**: autosave with an 800 ms debounce and a single PUT in flight. Retry after 2, 4, 8, 16, then 30 s (`Retry-After` clamped to 1 to 30 s). Keepalive for bodies up to 60 KiB. A 409 opens the conflict sheet (Take theirs, Keep mine, Save mine as a copy). A foreign change reloads a clean dashboard and raises the conflict bar on a dirty one. Close and leave sheets guard unsaved work. Another window on the same dashboard sees "Being edited in another window".
- **Zoom** (G2, G14): a design page wider than the stage is scaled to fit (never below half size; then the stage scrolls). "Actual size" is a per-window choice and is not saved. The toggle appears only while the fit is below 100 %.
- **Localized system names** (G20): `E.names`. Search finds a system entity by its localized and by its raw name.
- **Widgets**: the seven variants that were empty placeholders are built: climate stepper and scale, gauge bar and tank, temperature thermometer and comfort, sensor icon. Small-size rules for all families (G13). Climate commands (dial, stepper, scale) and the server's expected states for climate.
- **Security review (review S1, S2, import and clipboard)**: import is refused above 512 KiB before reading, is parsed as untrusted JSON, and is checked against the count limits before anything is placed (over 12 pages, over 120 widgets on a page, over 400 widgets, too many layout rows). The clipboard is decoded as untrusted. Text reaches the DOM only through `textContent`.
- **Gates and probes**: pixel-truth contrast, occluded and offscreen text recorded as skips (never passes), infrastructure failures listed apart, one language end to end (G15), and a repository guard for output directories. Partly covered text is measured outside open overlays, and scroll clipping is told apart from window clipping.

Commits since `147dcade4`, newest first (hash, date, subject; dates as given by the lead):

- `a23759599` 2026-11-27, the gate measures visible text it used to skip as occluded (round 8)
- `7c59b4f14` 2026-11-27 04:54, voxel field borders hold 3:1, cards align, import paths on their own line (round 7)
- `fc4ee0494` 2026-11-27 04:13, CI runs the widget survey and the live HA test, select keeps focus, device paging complete (Batch F review)
- `930b71ab8` 2026-11-27 02:28, widgets keep their titles at 96 px cells, comfort zone is an area (round 6)
- `2bcc78664` 2026-11-27 02:19, M4 guides and this hand-over (F4, F5)
- `6e85e0c38` 2026-11-27 01:59, server confirms climate commands, live test (round 6)
- `2ce751121` 2026-11-27 01:49, climate dial commands, comfort zone (round 5)
- `af2e45cbd` 2026-11-27 01:04, seven placeholder widgets built, values never cut, state marks (round 4)
- `afe9d695a` 2026-11-27 00:32, chrome findings of the pixel-truth gate (round 4)
- `590492512` 2026-11-27 00:14, widgets read well at small sizes (G13)
- `9462bd4ee` 2026-11-27 00:14, pixel-truth contrast and robust runner (gate)
- `46bc4d88e` 2026-11-27 00:06, localized device names (E.names), one gaps wording in all locales (G18b/G19/G20)
- `067506327` 2026-11-26 23:54, zoom toggle, Look grouping, one-language harness (G14/G15/G17/G18)
- `fcd6e806c` 2026-11-26 23:03, 60-widget drag budget met (D11 part 2), click-eater, tab hover, home sheets, review E
- `0604bef32` 2026-11-26 21:16, review findings: drawers focus, Escape, import limits, grid.place, Retry-After clamp
- `e637513b5` 2026-11-26 21:00, drag without layout thrash (D11 part 1)
- `b7ce4e217` 2026-11-26 20:15, designer browser test F1, gate runner E2, round 2
- `06dbd02a8` / `733838fc1` 2026-11-26 18:49, docs F4 / gate round 1
- `7d02651fa` 2026-11-26 17:30, F2 live save; `c890a7669` 17:26, B/C/D review fixes
- `e1f16cba0`, `0ce756a63`, `29b73694c`, `671642fd7`, `3b6136a34` 2026-11-26 16:53-17:07, part 2, E1, B5/B6
- `d2d368f80`, `0b886f076`, `815391692`, `cca070808`, `d9fecde43`, `3f376a82b` 2026-11-26 16:14-16:24, B batch and A fixes
- `33ccbb475` to `699747c68` 2026-11-26 16:00-16:11, Batch A

## Contract summary (plan "Engine contract (frozen for M4)", C1 to C20)

Implemented as written unless listed under deviations. The engine guide has the file list, the exports and the DOM contract.

- C1 load order: `APP_ORDER` in `scripts/homedash-tests/harness.mjs`, the desktop loader entry `homedash`, and `TestHomeDashWiring` pin the same order.
- C2 to C3 mount surface and host context: `canEdit`, `ui.menu`, `ui.focusWindow`, `ui.sheetHost` (the host's `.hdx-app`), `enterEdit`, `exitEdit`, `run`, `contextMenu`, `beforeClose`, `state().edit`.
- C4 to C6 designer, commands and history: `E.designer.create`, the pure builders of `E.commands` (limits 12 pages, 120 widgets per page, 400 in total), coalescing (nudge 800 ms, field edits 1000 ms), history limit 100.
- C7 store docs and lock, C8 save lane, C9 API, C10 catalog (`defaults`, `pickWidgetFor`), C11 clipboard (`localStorage` key `homedash.clipboard.v1`), C12 gestures (thresholds 4 px canvas, 5 px palette, 300 ms touch hold), C13 render and view, C14 keymap, C15 DOM and CSS contract, C16 announcements, C17 i18n prefixes, C18 fixture routes, C19 icon names (31 translated), C20 part-2 interfaces.

### Deviations and readings

Part 1 readings (plan, "Recorded readings and deviations"):
1. Drafts: a widget with an empty required binding stays local and is stripped from every save, export and copy.
2. Catalog `defaults` gains an optional `capability`; `select` maps to list and `number` to slider by default.
3. `state.mode` stays; the edit state is `state.edit`.
4. Design mode uses scroll metrics even for `fit_screen` documents.
5. The frame's More button is hidden in design and thumb modes.
6. The host bar is hidden in design mode; the engine edit bar replaces it.
7. Lanes are realm-scoped. `keepalive` passes through `api()` (verified in `desktop-foundation.js`).
8. Keys: the designer's root listener runs first and calls `preventDefault` on every key it consumes. Ctrl+Alt+Arrow is not used (Spaces).
9. `POST dashboards/validate` returns 422 `invalid_document` with `bad_json` for an unknown field, so no body may carry one.
10. The custom palette is read in the server shape (`look.palette.custom`).
11. The kiosk block is filled by `normalise`.
12. Runtime variables written by JS are `--hd-e-*`; they are not tokens and do not change the freeze.

Part 2 readings (the second half of the plan):
1. Files in the C1 slot: `hd-form.js`, `hd-samples.js`, `hd-palette.js`, `hd-entity-picker.js`, `hd-inspector.js`, `hd-look.js`, `hd-designer-dialogs.js`.
2. Pure helpers exposed on their own namespaces for the Node tests (`model`, `exportFile`, `importDocument` and others).
3. Canvas overlays use `--hd-focus`.
4. One accent: only Done and the empty page's primary action fill with the accent.
5. Muted text on the bar and side panels is `--vd-theme-muted`.
6. The look editor is an end-docked modal sheet with no scrim.
7. Forms open on `E.sheet` with their own buttons, so they can refuse to close while invalid.
8. In view mode, rename, look and add page enter edit first.
9. `new` outside a dashboard opens the home screen; import works with or without a dashboard.
10. Palette thumbs: the code uses 80 px cells and a 12 px gap (`E.samples.CELL`, `GAP`), not the 56 px and 8 px of the reading as first written. The thumb is scaled into its card box.
11. The entity browser pages 200 rows; at most 20 visible rows have live state.
12. E-S uses Fruity light and dark; E-T uses the 16 variants.
13. Icon names are translated (`homedash.icon.<name>`, 31 keys).
14. Server text rules are checked inline (`E.form.textIssue`).
15. Field, select, segmented, device-box and swatch borders are 60 % ink over their surface, and the gate probes them at 3:1 in every variant.
16. Search folding (`E.form.fold`) is shared by the palette, the icon search and the slug.

Lead answers in the plan ("Decisions of the lead"): Q1 Fruity light and dark for E-S; Q2 canvas overlays use `--hd-focus`; Q3 look editor is a modal sheet with no scrim; Q4 view mode enters edit first; Q5 `new` without a dashboard opens the home screen; Q6 home-list delete keeps the host's confirm; Q7 German uses "Du" throughout; Q8 `[data-hd-drawer]` only in drawer mode; Q9 monochrome states, one accent; Q10 icon names translated; Q11 thumbs capped at 2x2; Q12 import from the home screen; Q13 area options from loaded rows; Q14 unknown entity is status `unknown` or a 404 `entity_not_found`; Q15 E-T only with `HOMEDASH_M4_SHOTS=all`; Q16 "Show details" only for lights; Q17 borders probed at 3:1.

Lead decisions made during execution:
- G2 zoom to fit, and G14 actual size (per window, not saved; the View menu pair and the edit bar toggle).
- The save lane keeps retrying after 401 (the session may come back).
- D11 performance is measured and met: a 60-widget drag keeps 90 or more frames in 2 s with no long task over 50 ms (browser test step `20-drag-budget`).
- The seven variants that were placeholders are built in M4, not in the M5 widget waves.
- Climate commands: `set_temperature` (continuous) and `hvac_mode` (the capability name), with the server's expected states.
- The occluded-probe rule: a text that another element covers is a skip, never a pass.
- G15: the one-language harness loads the desktop shell words of the pass language too.
- G17 and G18: Look grouped as Surface and Grid; one page-compaction wording, "Close gaps on this page".

## Tests and commands

Node (`npm run test:homedash`, all suites). The M4 suites are `clipboard`, `commands`, `save`, `designer`, `palette`, `inspector`, `dialogs`, `zoom`, `names`, `climate` and `g13b`, plus `wiring` (APP_ORDER) and `i18n` (the locale key sets and the list of English-equal values, each with a reason). `npm run check:ui` runs the bundle drift check.

Go, no browser:

```
go test ./ui -run '(?i)^(TestHomeDashWiring|TestHomeDashFixture.*|TestTranslations_.*|TestFrontend_StaticI18nKeysExistInEnglishBundle|TestFrontend_JS_NoLiteralTranslationFallbacks|TestDesktopDynamicI18nPrefixKeysExistInAllLanguages|TestDesktopAppMaterialsAdoptedByApps)$' -count=1
go vet ./ui
go test ./internal/homedash -count=1        (includes TestWidgetCatalogDefaults, D2)
go test ./internal/desktop -count=1
go test ./internal/audit -run '^TestUIBrowserCIShardsCoverEveryTest$' -count=1
```

Baseline (plan Task 0): the line-budget test (`TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget`) fails on 8 files outside HomeDash. Those are not ours.

Browser tests (Chrome; `AURAGO_RUN_BROWSER_SMOKE=1`; build the test binary from the repository root outside the repository, then run it from `ui` with the shell sandbox off and the flags quoted):

- `TestDesktopHomeDashDesignerBrowser` (`ui/desktop_homedash_designer_browser_test.go`): the designer flow; its named steps include `20-drag-budget` (the 60-widget drag budget) and `21-zoom-actual` (Actual size).
- `TestDesktopHomeDashDesignerGatesBrowser` (`ui/desktop_homedash_designer_gates_test.go`): the gate sheets. `HOMEDASH_M4_SHOTS=1` (E-S, E-F) or `all` (adds E-T); `HOMEDASH_M4_DIR` is required and must be outside the repository. CI skips it.
- `TestDesktopHomeDashWidgetFitBrowser` (`ui/desktop_homedash_widget_fit_test.go`): the widget fit survey (HOMEDASH_WF_* knobs in the engine guide).
- `TestDesktopHomeDashBrowser` (M2), `TestDesktopHomeDashControlsBrowser` (M3), `TestDesktopHomeDashControlLabBrowser` (lab sheets).
- `TestDesktopHomeDashLiveHABrowser` (`internal/server/desktop_homedash_live_browser_test.go`): the edit-and-save step against the real handlers; the server accepts what the designer saves, and the climate command is confirmed from the state frame.

Environment knobs of the gate and the designer test (full list in the engine guide): `HOMEDASH_M4_SHOTS`, `HOMEDASH_M4_DIR`, `HOMEDASH_M4_LANG` (`en` or `de`), `HOMEDASH_M4_STATES`, `HOMEDASH_M4_VARIANTS`, `HOMEDASH_M4_DPRS`, `HOMEDASH_M4_DEADLINE`, `HOMEDASH_M4_F1_STEPS`, `HOMEDASH_M4_PERF=log`. Output directories are checked by `hdSafeOutDir`, which refuses a path at or under the directory that holds `go.mod`.

### CI

- CI runs `TestDesktopHomeDashWidgetFitBrowser` without `HOMEDASH_WF_DIR`: probes only, in a temp dir, no sheets.
- The ui-browser job runs `TestDesktopHomeDashLiveHABrowser` once, in the Desktop H-R shard.
- The ui-browser job sets `HOMEDASH_M4_PERF=log`: the D11 budget is enforced on developer machines and logged in CI.
- `internal/audit/ui_browser_ci_test.go` checks all three.

## Gate runs

Directories under `C:\disposable\homedash-m4\`. Dates for the E-S, E-T, E-F, de, final and r7 rows are the lead's times (local time). The `g13`, `g13b` and `r5\lab` rows use the `generated` stamps of their `probes.json` (UTC). Verdicts are the lead's, from the gate notes (`gate-r1.md`, `gate-r3.md`, `review-e.md`).

| Directory | What it holds | Date |
| --- | --- | --- |
| `es-r1` | E-S round 1 after G1 to G12 (gate-r1 findings); `probes.json`, `index.html`, sheets | 2026-11-26 ~18:40 |
| `es-r2` | E-S round 2; `probes.json`, `index.html` | 2026-11-26 ~20:10 |
| `et-r3` | E-T round 3, 16 variants at 1x only; sheets, no `probes.json` (the 2x sweep panicked twice in `hardReset`) | 2026-11-26 22:12-23:40 |
| `de-r3` | German pass; `probes.json`, `index.html`; the test agent reported 0 probe failures | 2026-11-26 22:12-23:40 |
| `g16\run1` to `g16\run5` | probe rounds of the G16 maker (pixel-truth contrast, occluded skip, infrastructure reporting); `probes.json`, `index.html` each | 2026-11-26/27 ~23:30-00:10 |
| `r4\gate` | round 4 gate; `probes.json`, `index.html` | 2026-11-27 ~00:30 |
| `g13\before`, `g13\after`, `g13\lab-before`, `g13\lab-after` | G13 widget fit before and after, and the lab sheets (UTC) | 2026-11-26 22:56 to 23:11 UTC |
| `g13b\before`, `g13b\after`, `g13b\after-light`, `g13b\t1` to `g13b\t10` | G13 second round (after and light), and the step runs `t1` to `t10` (UTC) | 2026-11-26 23:21 to 2026-11-27 00:01 UTC |
| `r5\lab\dark`, `r5\lab\light` | lab sheets in both schemes (UTC) | 2026-11-27 ~01:45 local (00:38 UTC) |
| `final-et` | E-T, 16 variants at 1x and 2x, `HOMEDASH_M4_SHOTS=all`; `probes.json`, `index.html`, `ET-<variant>-1x/2x.png` sheets, `ES-*`, `EF.png` | 2026-11-27 ~02:30-03:48 |
| `de-preview` | German pass (`HOMEDASH_M4_LANG=de`) on fruity light and dark, 68 states | 2026-11-27 ~03:54 |
| `final-wf` | widget fit survey, dark and light (`probes-dark.json`, `probes-light.json`, 40 sheets) | 2026-11-27 ~03:56 |
| `r7\et`, `r7\et3`, `r7\et4` | round 7 re-runs: voxel_world, green_slime and electronic_dreams at 1x and 2x (`r7\et`); fruity-light, fruity-dark and standard at 1x (`r7\et3`); fruity-light German states 12 and 31 (`r7\et4`) | 2026-11-27 ~04:45 |

Final gate run and round 7:

- `final-et` covered 1156 states with 0 infra. 52 states failed with 53 lines, in three variants:
  - voxel_world: field borders at 2.24:1 against the field surface. This is a real finding. The material's inset bevel darkens the surface. Fixed in round 7 with the material token `--vd-mat-field-edge-mix` (60 % by default, 80 % for voxel_world).
  - green_slime 24-page-tabs-menu: the page tab context menu half covered the "Second" label. A probe artifact.
  - electronic_dreams 21 and 27: palette scroll clipping at the window edge was read as outside the window. A probe artifact.
- In round 7 the probe changed in two ways. It measures text outside open overlays (context menu, window menu popover, sheet, entity picker popover, toast); theme decorations do not count. Outside-window uses the text as clipped by its scrolling ancestors, with a new check that fails when a scroller itself ends below the window.
- Re-runs after round 7 had 0 failures and 0 infra: the three variants at 1x and 2x (204 rows), fruity-light, fruity-dark and standard at 1x (170 rows), and the German states 12 and 31. The occluded skip counts did not change (52 and 109).
- `de-preview`: 68 states, 0 failures.
- `final-wf`: 8532 tiles per palette, 0 failures on every page (lg, md, sm, c96, thumb).
- Round 7 also fixed two layout points from the German review. Palette, inspector variant and look family cards start their content at the top, so a wrapped label no longer shifts the neighbour's title. Import issue paths sit on their own line under the message.

Final verification at `7c59b4f14` (logs in `final-verify`):

- `npm run test:homedash` 528/528, `npm run check:ui` PASS, `go vet` of ui, internal/server, internal/audit and internal/homedash clean.
- `go test ./internal/homedash/...` 181 passed; `go test ./internal/server -run '(?i)homedash'` 46 passed.
- `go test ./internal/audit` and `go test ./ui` (no browser): only base failures. Besides the ones under Open items, `TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget` fails on files this branch does not touch (audio player, code studio, software store, synth studio, ecalc, notes; byte-identical to main).
- Browser: `TestDesktopHomeDashDesignerBrowser` (D11 budget enforced, first run), `...WidgetFitBrowser` (CI mode), `...Browser`, `...ControlsBrowser`, `...ControlLabBrowser` and `TestDesktopHomeDashLiveHABrowser` PASS. The gates test skips without `HOMEDASH_M4_SHOTS`, as designed.
- Full sweep, 16 variants at 1x: 612 states, 0 infra, 0 failures.

Round 8 (`a23759599`, sweep in `r8\et`): that sweep still skipped 236 visible text rows as occluded: the selection chip, "Delete widget", the drag avatar, an entity state and four labels. The chip and the avatar are `pointer-events: none`, and transparent layers (the dock scroller's hover headroom, sheet and inspector feet) lay over the rest. The probe now tests visual occlusion: it makes the element hit-testable for the check and looks through elements that paint nothing at the point. Sweep after: 612 states, 0 infra, 0 failures, lowest text contrast 4.53. Skips left: the chip under the drawer scrim in state 28 (one per variant, intended) and one green_slime help line outside the inspector's scroll clip (offscreen).

## Open items

- M3 D14: live Home Assistant acceptance by the user, or a local HA demo container with the user's OK (`C:\disposable\homedash-plan\m3-decisions.md`). The live-handler step is covered by `TestDesktopHomeDashLiveHABrowser`; the live acceptance itself is still open.
- M3 D10: the HA service policy allowlist question. The user must answer it before the covers and locks widgets (M5 W3). Not M4 D10 (import/export).
- Base failures that are not ours: `TestFrontend_JS_NoLiteralTranslationFallbacks` (code-studio bundle) and `TestDesktopStoreStartI18n`; and `TestLinkedGoModulesHaveGPL3CompatibleLicenses` in `internal/audit` (stale row `github.com/gobwas/pool` in `licences/linked-go-modules.tsv`; go.mod and licences are unchanged on this branch).
- GitNexus index was unusable during execution, so impact analysis could not be run through it for the edits in this milestone.
- Light slider at 2x1 on lg has a 40 px track; it is narrower than the other sliders.
- Desktop shell, not HomeDash: in the dock shell, `.vd-dock-scroll` keeps 64 px of transparent hover headroom above the dock (`desktop-base.css`, margin-top -64px / padding-top 64px; removed only for coarse pointers). It lies over the bottom edge of windows above the dock and may swallow clicks there (e.g. the inspector's "Delete widget"). Not verified by a click test.
- Polish follow-up: the climate dial at 3x3 shows the mode twice, in the mode bar and in the state line under it.
- M5 widget types need `homedash.widget.<type>.name` and `homedash.widget.<type>.desc` in all 16 locales, and every icon they use needs a `homedash.icon.<name>` key (31 exist today).

## Status

Nothing pushed or deployed.
