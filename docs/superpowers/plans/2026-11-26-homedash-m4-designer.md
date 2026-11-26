# HomeDash M4 — Designer implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (or superpowers:executing-plans). Steps use `- [ ]`. This is PART 1 (header, file map, frozen engine contract, Batch A, Batch B). PART 2 (Batches C–F) is written against the "Engine contract (frozen for M4)" below and must not change it without the lead.
>
> **Provenance (2026-11-26).** Every block preceded by `<!-- embed: dryrun-m4/… -->` is byte-identical to a file of the plan dry run in `C:\disposable\homedash-plan\` (a partial copy of main `99bf6a699` with all of this code applied), where it ran: `npm run test:homedash` 363/364 (the one failure, `bundle.test.mjs`, is an artefact of the partial copy), `go vet -overlay` clean for `./ui` and `./internal/server`, `TestHomeDashFixture*` 7/7. The plan was reviewed by two reader cross-checks and two dry runs; every finding is applied (see "Revision 2/3" in part 1 and "Revision 2" in part 2).
> **Not proven by the dry run (prove during execution):** browser tests; the Go tests that read embedded files (`TestDesktopAppMaterialsAdoptedByApps`, `TestHomeDashWiring`: `-overlay` does not reach `//go:embed`); part 1 B1, B2, B4–B7 DOM wiring (palette and inspector were tested against designer doubles); the E2 gate runner stages states 01–03 only (04–33 are specified and staged at execution); `(pointer: coarse)` through touch emulation; the F1 close-button selector; the 60 % field-border mix (gate E-T decides).

**Goal.** M4 "Designer" (decisions D1–D15, `C:\disposable\homedash-plan\m4-decisions.md`): build dashboards in the desktop app by drag and drop and keyboard, with selection, move/resize with snapping, guides and auto-scroll, clipboard across pages/dashboards/windows, undo/redo, autosave with retry, a conflict sheet on 409, a warning on close, pages and md/sm layouts, look editing, import/export, window and context menus, screen-reader announcements, first-class editor chrome in all 16 theme variants. No kiosk, no agent banner, no M5 widget types. Nothing pushed or deployed.

**Architecture.** The designer is engine code mounted by `E.mount` (`ui/js/homedash/`). Part 1 adds six engine files: `hd-clipboard.js` (E.clipboard: untrusted clipboard codec + storage channel), `hd-commands.js` (E.commands: the whole pure command catalogue, key map, spatial focus), `hd-save.js` (E.save: autosave lane + save/conflict/leave sheets), `hd-gestures.js` (E.gestures: move, resize, marquee, palette/entity drop on top of `E.input.track`), `hd-editbar.js` (E.editbar), `hd-designer.js` (E.designer: controller). Existing parts gain a design mode (`hd-render`, `hd-view`, `hd-widgets`), the edit surface (`hd-app`), the docs cache and edit lock (`hd-store`), save/validate/duplicate/entities routes (`hd-api`), `pickWidgetFor` and palette groups (`hd-catalog`), the M0 carry-overs (`hd-layout` memo, `hd-doc.normalise` constraints, solver fix in `hd-colour`). The Go catalog gains `defaults` (the only backend change, D2). The desktop host gets Dashboard/Edit/View menus, context menus, `beforeClose`, the Edit button and the "being edited in another window" chip. Autosave lanes are realm-scoped: they survive a closed window while dirty and flush with `keepalive` on `pagehide`.

**Tech.** Vanilla ES2020 IIFEs, CSSOM only (`style.setProperty` / `E.core.setVars`), Node 22 tests with the strict fake DOM, Go 1.27, rod browser tests.

The code embedded in full below (`hd-clipboard.js`, `hd-commands.js`, `hd-save.js`, their tests, the test diffs of A2–A9, the A10 fixture, the B5 helper block and `toDesktopItems`) was applied to a scratch copy of main and run (dry run 2026-11-26, revision 2); makers copy it as it is. The other code snippets (hd-catalog, hd-layout, hd-doc, hd-colour, hd-api, hd-store, hd-input, hd-icons, hd-widgets, the Go catalog) were applied and run in the same copy as written. The B1, B2, B4, B5 (apart from the helper block), B6 and B7 descriptions were not run: makers may fix typos and obvious slips there but must not change a name, shape or rule of the contract without reporting to the lead.

## Recorded readings and deviations (lead questions Q1–Q11 at the end hold the defaults)

1. **Drafts.** A widget whose required binding (or required option) is empty is a *draft*: it stays in the local document and history but `E.doc.forSave` strips it (and its layout items) from every PUT, export and copy, because the server refuses the whole document with `binding_required`. The edit bar shows "Saved · 1 widget has no device · Show"; leaving or closing warns.
2. **Catalog `defaults` gains an optional `capability`** (`{kind, device_class?, capability?, type, variant}`) — needed for "dimmable light → light/slider" (D2), which is a capability, not a device class. Ordered first-match list; client skips unregistered types and falls back to `entity/tile`. `select → list` and `number → slider` (the HA template's option-count and box-mode refinements cannot be expressed in the catalog).
3. **Mount state:** the existing `state.mode` (loading|home|dashboard|disabled|error) stays; D3's edit state lives in `state.edit` (`edit.mode: 'view'|'design'`).
4. **Design mode renders with scroll metrics** even when `grid.fit = fit_screen` (stable cells while editing); the look editor's fit setting applies to view mode.
5. **More button.** In design and thumb mode the frame's `.hd-more` is `hidden` (the M0 rule "design: body inert, frame not inert" stays; `widgets.test.mjs:78`).
6. **Host bar hidden in design mode**; the engine edit bar replaces it; host banners stay.
7. **Lanes are realm-scoped** (D4 keepalive reading): verified `desktop-foundation.js:1092-1119` — `api()` spreads `options` into `fetch`, adds no CSRF header (CSRF is origin-based, `internal/server/auth.go:985`), so `keepalive` passes through. Every PUT whose body is ≤ 61 440 bytes carries `keepalive` (F2); a longer body goes without it (closing a window does not unload the page, only closing the tab does).
8. **Keys vs desktop shell** (verified `sdk-events-bootstrap.js:729, 836-855`): `handleDesktopKeydown` is a bubble-phase document listener that returns on `defaultPrevented`; its file-history Ctrl+Z/Y runs only when `desktopIconKeysAllowed` (target in `#vd-icons` or body). The designer's root listener runs first and calls `preventDefault` for every key it consumes; focus is always restored inside the root (never body). `handleWindowMenuShortcut` (`menus-and-routing.js:1535`) matches only `shortcut`, so HomeDash menu items use `shortcutHint` (display only). Ctrl+Alt+Arrow are Spaces shortcuts (`spaces-runtime.js:221`) and are not used.
9. **Validate route:** `POST dashboards/validate` answers 200 `{valid, issues, warnings, document}` for a decodable document and **422 `invalid_document` with issue `bad_json`** when the strict decoder (`DisallowUnknownFields`, `dashboard.go:198-212`) refuses it (also on PUT/POST). Unknown fields therefore must never reach a body.
10. **hd-view custom palette bug:** the server shape is `look.palette.custom = {accent, surface_hue, scheme}`; `hd-view.applyRoot` read them flat. Fixed in A3 (flat still accepted for lab fixtures).
11. **Kiosk block** is required by the validator; `normalise` adds the server default when absent and repairs `start_page`; `deletePage` repairs `start_page`, navigate actions and `nav` page options.
12. **Editor runtime variables** written by JS are named `--hd-e-*` (like `--hd-x`, not design tokens; no freeze change).

## Revision 2 (2026-11-26)

Revision 2 applies the lead's decisions F1–F16 (`m4-part1-fixes.md`) to the cross-check findings (`m4-xcheck-part1.md`: C1, I1–I4) and the dry-run findings (`m4-dryrun-part1.md`: defects 1–9, contract problems 1–5). The embedded code ran in the dry-run scratch copy: `npm run test:homedash` 326/327 (the one failure is `bundle.test.mjs`, a partial-copy artefact: `assets/storymaker/runtime-entry.mjs` is not in the copy), `go vet -overlay … ./ui` clean, `TestHomeDashFixture*` 7/7 PASS. One line per item:

- **F1** B5: the designer helper block (`ANNOUNCE` literal map, `emit`, `draftBadge`, `limitText`, `announceCmd`, `describe`) is embedded in full and was run against the real `E.core.el` / `E.core.setAttr` / `E.icons.icon`; `store.view(id).name` read from `hd-store.js`; `limitText` uses one literal `t()` per key (a ternary of keys inside `t()` is a lint trap); `gestures` is constructed before the first `render()` (setup order spelled out); the stray `o.gestures_ && 0;` is removed.
- **F2** C8 rules, B3 `hd-save.js`, B6 dispose: every PUT with a body ≤ 61 440 bytes carries `keepalive`; `flush()` while a PUT is open queues one follow-up PUT (`flushAsk`) that goes out right after that PUT succeeds; B6 `dispose` calls `lane.flush({keepalive: true})` when the lane is dirty; C8 documents the residual window. The `keepalive` argument of `flush()` now only chooses `settle(0)` against `settle(5000)`; the PUT's keepalive follows the body size.
- **F3** B7: `toDesktopItems` embedded (`{separator: true}`; `checked` shown through the `check` icon), run against the shape cases in `check_to_desktop_items.mjs`.
- **F4** B6 `actions()` reports `available: !!E.ddialogs` for `rename`, `import`, `export`, `look`, `add_page`; B7 leaves out items whose action is unavailable; `new` keeps the New ▸ fallback.
- **F5** A11: `homedash.family.glass|tactile|minimal|instrument` in all 16 locales (en Glass/Tactile/Minimal/Instrument, de Glas/Taktil/Minimal/Instrument) with the `SAME_WORD_KEYS` rule. C17 is unchanged: part 2 only adds family keys beyond these four, if any.
- **F6** A2: the tests use the harness `eq` for vm-created arrays (embedded as a diff).
- **F7** C11 names `ParseEntityID` (`internal/homedash/ids.go`) and the text rules of `sanitize.go` (control, bidi, URL); A7 embeds `hd-clipboard.js` with the tested `entityId` and `text`; string options use `text`; `clipboard.test.mjs` carries the hostile cases (`bt:aa:bb:cc`, 1- and 17-character sources, a control character in an id, a title with U+0085 and U+202E, `http://x`, `javascript:`, `//evil`, a string-option test).
- **F8** A10: `slugTakenLocked` is defined in the embedded fixture file.
- **F9** B3 embeds the complete `hd-save.js` with the three sheets (every key literal) and the export; close mode maps retrying/pending/saving → `[close_keep, close_leave (primary)]` and invalid/conflict/gone → `[close_keep (primary), close_discard (danger)]`, exit mode unchanged (C8 states it; the test covers every phase in both modes).
- **F10** B3 lane: generation counter; `conflict`, `overwrite` and `reset` bump it and a late answer of an older PUT is ignored (tests for `reset` and `conflict`); C8 states it; B5 `reconcile` defers while the lane is saving, `lane.conflict` included.
- **F11** A10 fixture lint codes: below the minimum → `required` (name, page name), above → `too_long`; no pages → `required`; C18 states it; assertions added to `TestHomeDashFixtureSaveRoutes`.
- **F12** C5 `add` states the equal-size swap of `moveItems`; `commands.test.mjs` covers the equal-size and the different-size case (both reload-stable).
- **F13** C7: `put(id, doc)` keeps reading `doc.revision`; the caller passes the document carrying the new revision (B6 already builds `Object.assign({}, ev.body, {revision: ev.revision})`); a mount ignores `doc` events whose revision is not newer than the one it shows.
- **F14** `add` (so `paste` and `duplicate`) compacts the three resulting layouts on a compact page with `E.grid.compact`, the step `E.doc.normalise` takes (`E.grid.repair`); the local document equals the reloaded one. Tests: compact-page add, duplicate and paste equal `normalise` of themselves; a free page keeps `startY`. Consequence: on a compact page `startY` is only a hint, a duplicate may settle beside its source instead of below it.
- **F15** Embedded tested code: A7 `hd-clipboard.js` + `clipboard.test.mjs`; A8 `hd-commands.js` + `commands.test.mjs`; B3 `hd-save.js` + `save.test.mjs`; A2–A6 and A9 test diffs (catalog, layout, doc, view, colour, api, store, input, icons, widgets, control); A10 fixture (diff of the M2 file and the full M4 file). Each block is preceded by an `<!-- embed: … -->` comment naming its scratch file; `embed-check.mjs` compares block and file byte for byte.
- **F16** R4: the Node harness `vm` context has no `TextEncoder`; engine files must not use it.

Not a lead item, same revision: reading 7 and the line before "Recorded readings" were aligned with F2 and F15; the B5 designer test also covers the helper block (render on an empty page, draft badge).

### Revision 3 (2026-11-26)

Revision 3 applies the lead answers to part 2 (`m4-plan-part2-answers.md`) and the part-2 cross-check (`m4-xcheck-part2.md`) to part 1. Nothing else changes; the embedded code of revision 2 is untouched. One line per item:

- **Amendment 1 (accepted, with C5)** B5: the designer calls `E.ddialogs?.enhanceTabs?.(D, bar.slot('pages'))` once, after the edit bar is built and the designer object `D` exists (the enhancer reads `D.doc()`, `D.state()`, `D.on`, `D.exec`, so it takes the designer, not the mount api and not the raw `api` of the host). There is no inspector fallback. The tab context menu in `contextMenu(ev)` calls `pageMenu?.(D, id)` with the same `D`.
- **Q6** B7: `askDelete(summary)` for a home-list card keeps the host's own `sheet.confirm` (the host sheet sits on `.hdx-app`, which is visible in home mode) and then `inst.remove(summary)`. `run('delete_dashboard')` (engine confirm sheet) is used only for the open dashboard (window menu, designer more menu), because the engine root is hidden in home mode.
- **Q7** A11: German `homedash.leave.drafts_many` = "{{count}} Widgets haben noch kein Gerät und sind nicht gespeichert. Diese fallen weg, wenn Du beendest." (revision 2 still addressed the reader formally in that string, which `i18n.test.mjs` refuses). Sweep of every German string of parts 1 and 2 (`Sie`, `Ihnen`, `Ihr`, `Ihre`, `Ihren`, `Ihrem`, `Ihrer`, `Ihres` as whole words): this was the only hit.
- **Q10** C17/A11: the dynamic prefix `homedash.icon.` joins the registered list (the 31 key names `homedash.icon.<name>` and their 16 translations are part 2's C0; a registered prefix with no keys yet is harmless to `TestDesktopDynamicI18nPrefixKeysExistInAllLanguages`). The repo list has no HomeDash family at all today, so A11 appends all 14 prefixes.
- **Q12** C2/C3/B6/B7: `import` is enabled when `!readonly` with or without an open dashboard (C2 table); B6 `actions().import = {available: !!E.ddialogs, enabled: !readonly}`; B7 shows Import… in the Dashboard menu in home mode; the optional mount option `ui.sheetHost() → Element` (the desktop host returns `.hdx-app`) is passed by B6 to the `E.ddialogs` ctx, which builds its sheet on it while the engine root is hidden (part 2 D2 `sheetOf`).
- **I6 (lead decision)** C4/B5: `D.addWidgets(entries, {source})` without `at` starts the first fit at `startY` = the bottom row of the lg bounding box of the current selection, or 0 with nothing selected (on a compact page `startY` is only a hint, F14). The palette's Enter therefore adds below the selection. One designer-level assertion was added to `designer.test.mjs` (not run: the designer files are not in the dry-run copy).

## Process rules (every task)

- **R1 Worktree.** One session worktree from `main` (ac7a11c7f); real `node_modules` copy (`robocopy … /E /MT:16`).
- **R2 GitNexus.** Before a task edits an existing symbol, the **lead** runs MCP `impact({target, direction:"upstream", repo:"AuraGo"})` for every symbol under "Existing symbols changed", pastes callers/risk into the dispatch, warns on HIGH/CRITICAL; `UNKNOWN`/empty → the maker greps callers and reports them. Before each commit the lead runs `detect_changes({scope:"all", repo:"AuraGo", worktree})` (re-run on `partial`/`truncated`). Without MCP: commit body line `GitNexus unavailable; callers checked with grep: <symbols>`.
- **R3 Makers stage only.** The lead commits by path: `git commit -m "<msg>" -- <paths>`. Never `git add -A`, `git restore`, `git reset`, `git stash`, `git checkout -- <file>`. New docs/AGENTS.md: `git add -f`.
- **R4 Files.** LF, no BOM. New JS ≤ 650 lines (hard 1100; guard warns > 700). No `innerHTML`/`outerHTML`/`insertAdjacentHTML`/`setAttribute('style'`/`.style =`/`cssText`/bare `confirm(`. Untrusted text only via `textContent`. Literal keys `t('homedash.…')`; dynamic keys only under registered prefixes (C17); no `|| 'literal'` fallbacks. Non-ASCII in JS via `String.fromCodePoint` (regex ranges as `\x00`, never backslash-u: the Write tool decodes it). Go: never a literal backslash-u escape. No external assets. Scripts via the Write tool. The Node harness `vm` context has no `TextEncoder` (nor other browser-only globals): engine files must not use it (count UTF-8 bytes with a loop as `hd-clipboard.js` and `hd-save.js` do; test files run outside the vm and may use it).
- **R5 Wiring step W** (every new engine file, same commit): `ui/js/desktop/core/module-loader.js:35` homedash `scripts` list, `scripts/homedash-tests/harness.mjs` `APP_ORDER`, `ui/desktop_homedash_wiring_test.go` engine list (`wiring.test.mjs` reads APP_ORDER). Order frozen in C1.
- **R6 Translations.** Every new key in all 16 `ui/lang/homedash/*.json` with real translations (German "Du", real umlauts). A value equal to English needs a `SAME_WORD_KEYS` entry with a reason in `i18n.test.mjs`, else rephrase.
- **R7 Go env.** `$env:GOTMPDIR='D:\tmp\go-build'` (backslash form); `no such tool "vet"` → prefix `GOTOOLCHAIN=go1.27.4`; D: full → trim `D:\dev-cache\go-build` entries older than 3 days.
- **R8 Browser tests.** `go test -c -o D:\tmp\go-build\hd-m4-ui.test.exe ./ui`, then `cd ui; $env:AURAGO_RUN_BROWSER_SMOKE='1'; & D:\tmp\go-build\hd-m4-ui.test.exe '-test.run=^TestX$' '-test.v' '-test.timeout=30m'`. Never stop Chrome by image name.
- **R9 Scope (D15).** `ui/js/homedash/**`, `ui/css/homedash-*.css`, `ui/css/desktop-app-homedash.css`, `ui/js/desktop/apps/homedash*.js`, the homedash dispatch block in `menus-and-routing.js` + `main.bundle.js`, `ui` HomeDash tests/fixtures/testdata, `scripts/homedash-tests/**`, HomeDash locale files, `ui/i18n_lint_test.go` (prefix list only), `internal/homedash/catalog*.go` + golden. Any other backend change: stop and ask.
- **R10 Builds.** After `menus-and-routing.js` edits: `npm run build:ui` and commit `main.bundle.js`; `npm run check:ui` must pass.

**How to run tests.** Node: `npm run test:homedash` (all PASS, no FAIL; count = baseline + new). UI: `npm run check:ui`, `npm run build:ui`. Go: `go test ./internal/homedash -run '(?i)^TestWidgetCatalog' -count=1`; `go test ./ui -run '(?i)^(TestHomeDashWiring|TestHomeDashFixture.*|TestTranslations_.*|TestFrontend_StaticI18nKeysExistInEnglishBundle|TestFrontend_JS_NoLiteralTranslationFallbacks|TestDesktopDynamicI18nPrefixKeysExistInAllLanguages|TestDesktopAppMaterialsAdoptedByApps)$' -count=1`; `go vet ./ui`; browser per R8.

---

## File map (whole milestone)

| File | Responsibility | Batch |
|---|---|---|
| `internal/homedash/catalog.go`, `catalog_widgets.go`, `catalog_test.go`, `testdata/widget_catalog.json` | `WidgetDefault`, `defaults`, test, golden | A1 |
| `ui/js/homedash/hd-catalog.js` | defaults, `pickWidgetFor`, `accepts`, `binding`, `optionSpecs`, `canToggle`, `families`, `paletteTokens`, `paletteGroups`, `CATEGORY_ORDER` | A2 |
| `ui/js/homedash/hd-layout.js` | `effective` memo keyed on lg + constraints | A3 |
| `ui/js/homedash/hd-doc.js` | `normalise(raw,{sizeOf,constraints})`, kiosk default, `drafts`, `forSave`; `toClipboard`/`fromClipboard` removed | A3 |
| `ui/js/homedash/hd-view.js` | `palette.custom` read (A3); design mode surface (B2) | A3, B2 |
| `ui/js/homedash/hd-colour.js` | solver step 3 (`on` ≥ 3:1 on `surface-solid`) | A4 |
| `ui/js/homedash/hd-api.js` | `saveDashboard`, `validateDocument`, `duplicateDashboard`, `entities`, `entity`, `extra.keepalive` | A5 |
| `ui/js/homedash/hd-store.js` | `docs` cache, edit lock, presence; events `doc`, `lock` | A6 |
| `ui/js/homedash/hd-clipboard.js` (new) | E.clipboard | A7 |
| `ui/js/homedash/hd-commands.js` (new) | E.commands | A8 |
| `ui/js/homedash/hd-input.js` | `track()` | A9 |
| `ui/js/homedash/hd-icons.js` | 24 editor glyphs, `choices()` | A9 |
| `ui/js/homedash/hd-widgets.js` | `paletteSnapshot`, `.hd-more` hidden outside view | A9 |
| `ui/desktop_homedash_fixture_test.go` (edit), `ui/desktop_homedash_fixture_m4_test.go` (new) | PUT/409, validate, duplicate, entity filters, lint, foreign save, scripted failures; `TestHomeDashFixtureSaveRoutes` | A10 |
| `ui/lang/homedash/*.json` (16), `ui/i18n_lint_test.go`, `scripts/homedash-tests/i18n.test.mjs` | part-1 keys, dynamic prefixes | A11 |
| `ui/js/homedash/hd-render.js` | design mode, diff `update`, `preview` (FLIP), forced bp, metrics | B1 |
| `ui/js/homedash/hd-save.js` (new) | E.save | B3 |
| `ui/js/homedash/hd-gestures.js` (new) | E.gestures | B4 |
| `ui/js/homedash/hd-editbar.js` (new) | E.editbar | B5 |
| `ui/js/homedash/hd-designer.js` (new) | E.designer | B5 |
| `ui/js/homedash/hd-app.js` | edit surface, lanes, reconcile, run, contextMenu, beforeClose | B6 |
| `ui/js/desktop/core/menus-and-routing.js`, `ui/js/desktop/bundles/main.bundle.js` | dispatch passes `showContextMenu`, `wireContextMenuBoundary`, `setWindowBeforeClose`, `focusWindow` | B7 |
| `ui/js/desktop/apps/homedash.js` | menus, context menus, beforeClose, Edit/elsewhere/save chips | B7 |
| `ui/desktop_homedash_wiring_test.go` | engine list (W), dispatch markers | W, B7 |
| `scripts/homedash-tests/*.test.mjs` | new: `clipboard`, `commands`, `save`, `gestures`, `designer`; extended: `catalog`, `layout`, `doc`, `colour`, `api`, `store`, `input`, `icons`, `widgets`, `control`, `render`, `view`, `app`, `host`, `harness.mjs` | A, B |
| `ui/js/homedash/hd-palette.js` (+ `hd-samples.js` if split) | palette, live thumbs, entity browser | C (part 2) |
| `ui/js/homedash/hd-inspector.js` (+ `hd-entity-picker.js` if split) | inspector tabs, entity combobox, multi | C (part 2) |
| `ui/js/homedash/hd-designer-dialogs.js` (+ `hd-look.js` if split) | look editor, page rename/icon/tab strip upgrades, import/export, new/rename/delete/open sheets, first run | D (part 2) |
| `ui/js/desktop/apps/homedash-home.js` | first-run cards per D1 | D (part 2) |
| `ui/css/desktop-app-homedash.css` | all editor chrome and overlay CSS (≤ 900 lines, host.test rules) | E (part 2) |
| `ui/testdata/homedash-lab*` | editor lab scenes if used for gates | E (part 2) |
| `ui/desktop_homedash_designer_browser_test.go` | `TestDesktopHomeDashDesignerBrowser`, `HOMEDASH_M4_SHOTS` | F (part 2) |
| `internal/server/desktop_homedash_live_browser_test.go` | edit-and-save step | F (part 2) |
| `ui/js/homedash/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `documentation/homedash-visual-system.md`, `docs/superpowers/specs/2026-11-23-homedash/m4-handover.md` | docs, hand-over | F (part 2) |

---

## Engine contract (frozen for M4)

### C1 Load order
`APP_ORDER` = `ENGINE_ORDER` (unchanged) + `hd-tone, hd-api, hd-stream, hd-store, hd-command, hd-catalog, hd-render, hd-sheet, hd-view, hd-clipboard, hd-commands, hd-save, hd-gestures, hd-editbar, ⟨part 2: hd-palette, hd-inspector, hd-designer-dialogs and their split files, in this slot⟩, hd-designer, hd-app` (`.js`), then `homedash-home.js`, `homedash.js`. No designer file touches the DOM at load. `hd-designer.js` throws at load without E.commands/E.save/E.gestures/E.editbar/E.clipboard; it feature-detects E.palette, E.inspector, E.ddialogs at call time. The kiosk (M6) never loads the designer files.

### C2 Mount surface (`hd-app.js`)
New options: `canEdit` (default true), `ui: {menu(x, y, items), focusWindow(), sheetHost() → Element}` (host functions; optional; `sheetHost` returns the element that carries E.sheet layers while the engine root is hidden in home mode, the desktop host returns `.hdx-app`), `storage` (Storage-like, default `localStorage` when reachable; first mount of a realm wins). New methods:
- `enterEdit({widgetId?}) → Promise<boolean>`; refuses when `edit.canEdit` false, another owner holds the lock, or already editing.
- `exitEdit({force?}) → Promise<'left'|'kept'>`: flushes and waits ≤ 5 s; then (if needed) the drafts sheet and the leave sheet (C8); `force` skips sheets (read-only switch, delete).
- `run(action, arg) → Promise<boolean>` (table below). `contextMenu(event) → {x, y, items} | null`. `beforeClose() → Promise<boolean>`.
- `state().edit` (frozen, recomputed in every `set`):
```
{ mode:'view'|'design', canEdit, reason:''|'no_designer'|'disabled'|'not_dashboard'|'readonly'|'homedash_readonly'|'schema_too_new',
  editElsewhere, dirty, saving, phase:'saved'|'pending'|'saving'|'retrying'|'invalid'|'conflict'|'readonly'|'gone', nextRetryAt, errorCode,
  canUndo, canRedo, undoLabel, redoLabel (translated), selection:[ids], bp:'lg'|'md'|'sm'|null, fit:'scroll'|'fit_screen'|null, drafts, hasClipboard,
  actions: { [name]: {available, enabled, checked} } }
```
Root `data-hd-mode` = `edit.mode`. Read-only (desktop policy or bootstrap `readonly`) → `canEdit` false; `setReadonly(true)` while editing → `exitEdit({force:true})`; `setReadonly(false)` → `retryNow()` on every realm lane.

**Actions** (`run` names; `available` hides, `enabled` greys):

| action | owner | part | enabled when |
|---|---|---|---|
| `new` | E.ddialogs | 2 | !readonly; absent → host shows New ▸ submenu |
| `new_starter`, `new_empty`, `new_ha` | app (existing create) | 1 | !readonly, !busy; `new_ha` if template listed |
| `open` | app → `home()` | 1 | always |
| `rename`, `look`, `add_page` | E.ddialogs | 2 | dashboard open, !readonly |
| `export` | E.ddialogs | 2 | dashboard open |
| `import` | E.ddialogs | 2 | `!readonly`, with or without an open dashboard (Q12: the home screen offers it; its sheets use `ui.sheetHost()` while the engine root is hidden) |
| `duplicate_dashboard` | app (flush, `api.duplicateDashboard`, open copy, toast) | 1 | dashboard open, !readonly |
| `delete_dashboard` | app (engine confirm sheet, `remove`); only for the open dashboard (menus, designer): home cards keep the host's own confirm (Q6) | 1 | dashboard open, !readonly |
| `undo` `redo` `cut` `copy` `paste` `duplicate` `delete` `select_all` `select_none` `reset_layout` `retry_save` `resolve_conflict` `show_draft` `add_widget` `focus_inspector` `toggle_palette` `toggle_inspector` | designer | 1 | design mode + state |
| `edit` (toggle, checked in design), `exit_edit` | app | 1 | `canEdit` and !editElsewhere |
| `show_editor` | app (`store.docs.focusEditor`) | 1 | editElsewhere |
| `layout_auto` | app (view only) | 1 | view mode |
| `layout_lg/md/sm` | view: forced preview; design: designer bp | 1 | dashboard open |
| `fit` | app, view override (checked = effective fit_screen) | 1 | view mode |
| `present` | app (`root.requestFullscreen`) | 1 | dashboard open |
| `page_prev`, `page_next` | view or designer | 1 | > 1 page |

### C3 Host ctx contract
Dispatch (`menus-and-routing.js` homedash block) passes, in addition to today: `showContextMenu, wireContextMenuBoundary, setWindowBeforeClose: (winId, h) => { const win = state.windows.get(winId); if (win) win.beforeClose = h; }, focusWindow: winId => { if (state.windows.has(winId)) focusWindow(winId); }`. `homedash.js`:
- mounts with `canEdit: true`, `ui: { menu: (x, y, items) => ctx.showContextMenu?.(x, y, toDesktopItems(items)), focusWindow: () => ctx.focusWindow?.(id), sheetHost: () => root }` (`root` = the `.hdx-app` element, Q12);
- `ctx.wireContextMenuBoundary?.(root, { onContextMenu: ev => { const m = inst.contextMenu(ev); if (!m) return false; ctx.showContextMenu(m.x, m.y, toDesktopItems(m.items)); return true; } })`;
- `ctx.setWindowBeforeClose?.(id, () => inst.beforeClose())`, cleared on dispose;
- window menus: in home mode the single dashboard menu offers what needs no open dashboard (New ▸/New…, Open…, ─, Import…, ─, Reload); with a dashboard open: `homedash-dashboard` (New ▸/New…, Open…, ─, Rename…, Duplicate, Delete…, ─, Import…, Export, ─, Reload), `homedash-edit` (Undo, Redo, ─, Cut, Copy, Paste, Duplicate, Delete, ─, Select all), `homedash-view` (Edit mode, ─, Layout ▸ {Automatic, Desktop, Tablet, Phone}, Fit to screen, Present); every item has an explicit `id`, `hidden/disabled/checked` from `edit.actions`, display shortcuts as `shortcutHint` only;
- host bar: `button.hdx-btn.hdx-edit[data-action=edit][aria-keyshortcuts="Control+E"]` (dashboard, view, canEdit, !editElsewhere); `div.hdx-elsewhere` (text + `button.hdx-btn[data-action=show-editor]`) when editElsewhere; `div.hdx-save[data-phase]` (text + `button[data-action=retry-save]` when retrying) in view mode when a lane for the open dashboard is not saved; `.hdx-app[data-hdx-edit=design|view]` hides `.hdx-bar` in design.
- Engine menu item (context menus, More menu): `{id, label, icon, shortcut?, disabled?, danger?, checked?, items?, run?, separator?}`; host maps `icon` ids (`more_info edit duplicate copy cut paste delete variant add_widget check rename icon left right`) to desktop icon names with a fallback glyph.

### C4 Designer (`E.designer.create(o)` → D)
`o = {root, view, store, api, catalog, lane, clipboard (realm channel), t, lang, ownerId, doc (normalised), widgetId?, ui, timers, raf, caf, summary() (list entry of the dashboard), request(action, arg) (→ hd-app run), onState(), openCopy(newId)}`.
State `D.state()` (frozen): `{doc, pageId, page, bp, override:{md:bool, sm:bool}, selection:[ids] (primary = last), primary, focusId, canUndo, canRedo, undoLabel, redoLabel, save (lane state), saveReason, drafts, conflict:null|{revision, savedBy, updatedAt, gone}, drawers, panel:''|'palette'|'inspector', clipboard:bool, gesture:null|'move'|'resize'|'marquee'|'drop', items (effective layout of the page at bp)}`.
API: `state, on('state'|'page', fn)→off, doc(), exec(cmd)→bool, undo(), redo(), seal(), replace(doc, {revision, clearHistory, toastKey}), select(ids, {primary, focus, announce, live}), focusFrame(id), setPage(id, {focus}), setBreakpoint(bp), addWidgets(entries, {at?, source}) → ids (without `at` the first fit starts at `startY` = the bottom row of the lg bounding box of the current selection, or 0 with nothing selected; on a compact page `startY` is only a hint, F14: the palette's Enter adds below the selection), constraints(id), widget(id), slot('palette'|'inspector'|'pages'|'bp') → element, openPanel(name), closePanels(), announce(text), toast(text, tone), sheet() (designer E.sheet instance), gestures, t, lang, catalog, store, api, run(action, arg), handles(action), contextMenu(ev), reconcile(info), prepareExit() → 'leave'|'discard'|'keep', flush(ms), destroy()`. `E.designer.ACTIONS` = the designer-owned action names of C2. `E.designer.DRAWER_BELOW = 900` (root width; separate from `BP_MIN`).
Selection: click selects; Shift/Ctrl/Cmd toggles; marquee replaces (Shift adds); Ctrl+A all on page (reading order, focused frame last); Escape clears; switching page clears; filtered by id after every re-render, undo, reload.

### C5 Command catalogue (`E.commands`, pure)
Every builder: `(doc, args) → command | refusal | limit | null`. Command = `{label (key), vars, ops:[{path, before, after, index?}], meta:{kind, ids, at, coalesce?, window?}, select?, page?, materialised?:'md'|'sm', announce?:{key,…}}`; refusal `{refused:'limit'|'last_page'}`; limit `{limit:'min'|'max'|'edge'|'blocked'}`. `args.now` (ms) sets `meta.at` (default `Date.now()`); `args.C(id)` gives constraints; `args.limitsOf(widget)` constraints of new widgets. Paths are id-keyed.

| builder | args | ops (paths) | label key | coalesce / window |
|---|---|---|---|---|
| `layout` | pageId, bp, next, C, ids, kind, label | `pages/<p>/layouts/<bp>` (from null when md/sm automatic → `materialised`) | given | given |
| `nudge` | pageId, bp, ids, dx, dy, C | layout; vertical retries k=1..64 until the bbox moves (jumps over compacted neighbours) | `cmd.move` | `nudge|<bp>|<sorted ids>` / 800 |
| `resizeStep` | pageId, bp, id, dw, dh, C | layout (edge `e` or `s`) | `cmd.resize` | `resize|<bp>|<id>` / 800 |
| `add` | pageId, bp, entries[{widget,size}], at?{x,y}, startY?, C, limitsOf | `pages/<p>/widgets/<new>` × n, then changed `layouts/lg|md|sm` (lg first fit from startY, or moved to `at` when bp lg: `E.grid.moveItems` pushes a differently sized occupant down, but an occupant of the same size as the new item swaps to the new item's first-fit slot; md/sm drop materialises and moves there; other non-null overrides derived first fit. On a compact page the three resulting layouts are then compacted with `E.grid.compact`, the step `E.doc.normalise` takes, so the local document equals the reloaded one and `startY` is only a hint; `duplicate` and `paste` go through `add` and follow both rules) | `cmd.add` | — |
| `remove` | pageId, ids | `E.doc.removeWidgets` ops | `cmd.delete` (cut: `cmd.cut`) | — |
| `duplicate` | pageId, ids, C, limitsOf | `add` below the selection bbox (lg) | `cmd.duplicate` | — |
| `paste` | pageId, bp, entries, startY, C, limitsOf | `add` | `cmd.paste` | — |
| `setWidget` | pageId, id, path[], value, label?, text? | `pages/<p>/widgets/<id>/<path>` (value undefined deletes) | given (default `cmd.option`) | text: `field|<id>|<path>` / 1000 |
| `bind` | pageId, id, binding, entity | `…/bindings/<binding>` | `cmd.bind` / `cmd.unbind` | — |
| `variant` | pageId, id, variant, limits | `…/variant` + every layout where the item breaks the new limits | `cmd.variant` | — |
| `applyFamily` | pageId, ids, family | `…/<id>/family` each | `cmd.family` | — |
| `align` | pageId, bp, ids, edge 'left'\|'top', primaryId, C | layout | `cmd.align` | — |
| `resetLayout` | pageId, bp | `layouts/<bp>` → null | `cmd.reset_layout` | — |
| `addPage` | name, icon | `pages/<new>` append | `cmd.page_add` | — |
| `renamePage` | pageId, name (1–40) | `pages/<p>/name` | `cmd.page_rename` | `page|<p>|name` / 1000 |
| `setPageIcon` | pageId, icon | `pages/<p>/icon` | `cmd.page_icon` | — |
| `duplicatePage` | pageId, name | `pages/<new>` at i+1 (new widget ids, remapped layouts) | `cmd.page_duplicate` | — |
| `movePage` | pageId, to | remove at i + insert at j on `pages/<p>` | `cmd.page_move` | — |
| `deletePage` | pageId | remove page; `kiosk/start_page`; navigate actions → `{do:'default'}`; `options/page` → first page | `cmd.page_delete` | — |
| `setPageCompact` | pageId, compact | `pages/<p>/compact` + compacted layouts when turning on | `cmd.page_compact` | — |
| `setDoc` | path, value, label, text? | `<path>` | given (`cmd.look`, `cmd.grid`, `cmd.rename`) | text: `doc|<path>` / 1000 |

Also `entry(catalog, type, variant, entityId?) → {widget, size}`, `keymap(ev, {canvas, typing}) → {action, …}|null` (C14), `spatial(items, fromId, dir) → id|null`, `readingOrder(items) → ids`, `LIMITS {pages:12, pageWidgets:120, widgets:400}`, `NUDGE_MS 800`, `FIELD_MS 1000`.

### C6 History wiring
`E.history.create({limit:100})` per designer. `exec` = apply 'after' → push → `lane.set(doc)` → render → select → announce. Pointer gestures commit one command at pointerup (no coalesce). Text fields pass `text:true` and call `D.seal()` on blur. Undo/redo announce `announce.undone/redone` with the translated label; apply failure (path gone) clears history. History clears on dashboard switch, Take theirs and clean reload; never on save; dropped on exit.

### C7 Store additions (`store.docs`, realm-shared)
`get(id)→{doc, revision, loadedAt}|null`, `put(id, doc)` (reads `doc.revision`; refuses older revisions, accepts an equal one; emits `doc {id, revision}`), `drop(id)`, `lock(id, owner)→bool`, `unlock(id, owner)→bool`, `editor(id)→owner|null` (emits `lock {id, owner}`), `presence(owner, {focus}|null)` (null releases that owner's locks), `focusEditor(id)→bool`. Lane saves `put` the saved body with the new revision: the caller passes the document carrying the new revision (`Object.assign({}, ev.body, {revision: ev.revision})` from the lane's `saved` event), not the body with its old one. Other windows of the realm re-render from it without a refetch; a mount ignores a `doc` event whose revision is not newer than the one it shows (its own save emits an equal revision).
**Reconcile** (hd-app, list after `dashboard_changed`): dashboard gone → view: home(`dashboard_deleted`); design: `designer.reconcile({gone:true})`. Newer revision: design → `designer.reconcile({revision, savedBy, updatedAt})`; view → cached doc if `cache.revision ≥ revision` else refetch. Designer: lane saving → defer until settled (the dirty branch too: `lane.conflict` is not called while a PUT is open, and the lane ignores the late answer of an older PUT after `conflict`, `overwrite` or `reset`, C8); `revision ≤ lane.revision()` → ignore (own save); clean → GET, replace (selection by id kept), clear history, `lane.reset`, toast `designer.reload_agent` or `reload_other`; dirty → `lane.conflict(revision)` + conflict bar at once.

### C8 Save lane (`E.save`)
`lane({api, id, revision, doc, prepare, timers, debounceMs=800})` → `{id, on, off, state(), doc(), saved(), revision(), disposed, owner, set(doc), hold(on), flush({keepalive})→Promise<phase>, settle(ms)→Promise<phase>, retryNow(), conflict(rev), overwrite(rev), reset(doc, rev), setPrepare(fn), attach(owner), detach(owner), dispose()}`. State `{phase, dirty, saving, revision, attempt, nextRetryAt, currentRevision, error:{code,status,issues}, warnings}`. Event `saved {revision, doc, body, warnings}`.
Rules: 800 ms debounce from the last change; one PUT `{revision, document: prepare(doc)}` in flight; new changes re-arm after success; `hold(true)` during gestures. Retry (status 0, 401, 408, 429, ≥ 500): 2, 4, 8, 16, 30, 30… s (`retryAfter` wins). 409 `revision_conflict` → `conflict` (`current_revision`). 404 → `gone`. 403 → `readonly` (until `retryNow`). Other 4xx (400, 413, 422 incl. `bad_json`, `limit_exceeded`, `invalid_document` with issues) → `invalid` until the next `set`. Body > 256 KiB → local `invalid` `limit_exceeded`. Every PUT whose body is ≤ 61 440 bytes is sent with `keepalive` (an open PUT survives a tab unload); a longer body goes without. `flush()` while a PUT is open queues exactly one follow-up PUT that goes out right after that PUT succeeds, without the debounce; a failed PUT drops the request. `conflict`, `overwrite` and `reset` start a new generation: the late answer of an older PUT is ignored (no state change, no `saved` event). Residual window: changes made while a PUT is open, when the whole desktop tab unloads, may be lost (the follow-up cannot start before the open PUT answers). Success keeps the local document, takes `dashboard.revision`.
Conflict choices: **Take theirs** → GET, `replace`, clear history, `lane.reset(doc, rev)`. **Keep mine** → `lane.overwrite(currentRevision)`. **Save mine as a copy** → `api.createDocument(forSave(doc) with id '', slug '', name = trim(name, 80 − suffix) + t('homedash.conflict.copy_suffix'))` → open the copy in edit mode; original lane discarded. `gone` offers copy / later only.
Sheets (`E.sheet` open, kinds `conflict` and `leave`): `conflictSheet({sheet, t, lang, info})→'theirs'|'mine'|'copy'|'later'|null`; `leaveSheet({sheet, t, phase, reason, mode:'exit'|'close'})→'keep'|'leave'|'discard'|null` (retrying/pending/saving → `[keep, leave (primary)]`; invalid/conflict/gone → `[keep (primary), discard (danger)]`; mode `close` labels them `close_keep`, `close_leave`, `close_discard`, mode `exit` `keep`, `leave`, `discard`); `draftsSheet({sheet, t, count})→'choose'|'leave'|null`; `errorText(t, error)`; `ERROR_CODES`. Realm: `realm.lanes: Map<id, lane>`; a saved, detached lane is disposed; `pagehide` → `flush({keepalive:true})` for dirty lanes; a mount's `dispose` does the same for its dirty lane before it detaches (the realm keeps the lane).

### C9 API additions (`hd-api.js`)
`saveDashboard(id, revision, document, extra)` PUT `dashboard?id=` body `{revision, document}` → `{dashboard, warnings}`. `validateDocument(document, extra)` POST `dashboards/validate` `{document}` → `{valid, issues, warnings, document}` (422 `bad_json` throws). `duplicateDashboard(id, name?, extra)` POST `dashboard/duplicate?id=` `{name?}` → 201 `{dashboard, warnings}`. `entities({q ≤100 runes, source, kind (one), area ≤80, limit 1–500 default 100, offset ≥0}, extra)` → `{entities, total, limit, offset, sources}`. `entity(id, extra)` → `{entity}`. `extra.keepalive:true` sets `init.keepalive`. Existing `createDocument` is the copy/import create.

### C10 Catalog
Go `WidgetDefault {kind, device_class?, capability?, type, variant}`, `WidgetCatalog.Defaults` (`defaults`, last field). Client: `catalog.defaults()`, `pickWidgetFor(entity, registry) → {type, variant}|null` (first entry with equal kind, device_class when set, capability present when set, type+variant in catalog and registry; else `entity/tile`), `accepts(type, binding, entity)` (kinds empty or containing the kind; `any_capability` satisfied), `binding(type, name)`, `optionSpecs(type)` (type + common), `canToggle(type)`, `families()`, `paletteTokens()` (≤ 40), `paletteGroups(registry) → [{category, types:[{type, variants}]}]` = catalog ∩ registry in `E.catalog.CATEGORY_ORDER = ['control','climate','sensor','media','camera','status','layout']`, empty groups omitted.

### C11 Clipboard (`E.clipboard`)
`KEY 'homedash.clipboard.v1'`, `MAX_BYTES 262144`, `MAX_WIDGETS 120`. `encode(doc, pageId, ids)` → `{"v":1,"widgets":[{"widget":{…no id},"lgSize":{w,h}}]}` in lg reading order. `decode(text, {catalog, pages:Set})` untrusted: length cap before parse, `v === 1`, ≤ 120 entries, `sanitizeWidget` keeps only `type` (catalog), `variant` (else first), `family` (catalog families), `title` (≤ 80 runes; the server's text rules: control characters (C0, DEL, C1) and bidi controls are dropped, a text that still holds `://`, `javascript:`, `data:`, `vbscript:` or starts with `//` becomes empty; reference `ContainsControl`/`ContainsURL` in `internal/homedash/sanitize.go`), `icon` (`^[a-z0-9-]{1,48}$`), `bindings` (catalog binding names; entity-id grammar of `ParseEntityID`, `internal/homedash/ids.go`: source `^[a-z][a-z0-9_]{1,15}$`, `:`, a native part of 1–200 bytes that is printable, without whitespace or bidi control and may contain `:`; an unknown source is still a valid id), `options` (catalog specs, typed, clamped, string options with the same text rule as `title`, colour lower-case, `page` must exist), `actions` (enum; navigate only to existing pages; toggle only when `canToggle`), `confirm` (bool); size clamped to the variant's limits. `channel({storage, win})` → `{read(), has(), write(text)→bool, onChange(fn)→off, dispose()}`; one channel per realm (`realm.clipboard`), notifies same-realm windows directly and other tabs through `storage`.

### C12 Gestures
`E.input.track(ev, {threshold=4, axis?:'x', holdMs?, capture, win, setTimeout, clearTimeout, now, onStart(start, p), onMove(p, start), onEnd(p, start, how), onTap(start, p)}) → {cancel(), dragging(), done()}|null` — primary pointer and main button only; `how` = `up|cancel|lost`; axis `x` abandons a mostly vertical first move (`scroll`); `holdMs` starts after a still press; a mouse keeps its session on `lostpointercapture` with the button down (F14); a click within 400 ms after a drag is swallowed.
`E.gestures`: `THRESHOLD_CANVAS 4`, `THRESHOLD_PALETTE 5`, `TOUCH_LIST {threshold:8, axis:'x', holdMs:300}`, `HOLD_TOUCH_MS 300`, `EDGE_PX 48`, `SCROLL_MAX 16`, `EDGES ['n','ne','e','se','s','sw','w','nw']`, `math {cellOf, cellUnder, pxOf, cellDelta, itemAt, guides, scrollStep, rectOf, hits}`, `create({root, stage, ctx(), select(ids, o), commit(kind, payload), hold(on), announce(text), describeLimit(kind), win, timers, raf, caf}) → {dragFrom(ev, payload), cancel(), active(), showSelection(id, chip), reset(), destroy()}`.
- `ctx()` → `{page (render handle), doc, pageId, bp, selection, items, cols, compact, C, catalog, readonly, widget(id)}`.
- Canvas pointerdown (delegated on stage): `.hd-handle` → resize; frame → move (press selects immediately unless modifier; touch on an unselected frame needs a 300 ms long-press); empty → marquee (touch: long-press). One `E.core.frame` rAF loop; pointermove only stores the pointer; recompute `moveItems`/`resizeItem` from the gesture-start layout only on cell change; `page.preview(next, {skip, animate:true, rows})` writes only changed items; dragged frames follow the pointer via `--hd-e-dx/dy`; ≤ 4 guides; auto-scroll within 48 px of the stage edges by `ceil(16 × proximity)` px per frame (both axes); metrics frozen at gesture start.
- Commit kinds: `move {ids, next, bp}`, `resize {id, next, bp, edge}`, `marquee {ids, additive}`, `add {entries, at, bp}`, `rebind {id, binding:'primary', entity}`. Cancel (Escape, pointercancel, lost without button, blur, page switch, reconcile, exit) restores the start layout. Drop settles with a FLIP from the pointer position (not under reduced motion).
- `dragFrom(ev, payload)` (part 2 palette/entity browser): `payload = {kind:'widget'|'entity', entries() → [{widget, size}], entity?, avatar() → Element, label, onTap?()}`; mouse/pen threshold 5, touch `TOUCH_LIST` with a non-passive `touchmove` guard once armed. Over a widget, an entity shows `rebind` when `catalog.accepts(type,'primary',entity)` else `invalid`; over empty cells a ghost at the default size (pointer-centred) with live reflow; page full → ghost `invalid`.

### C13 Render and view additions
`E.render.page(opts)`: `mode` ('view'|'design'), `bp` (forced or null), `fit` override, `describe(widget, item, entity)`, `emptyAction {cmd, label}|null`; widget apis read the mode live (no command, no more-info outside view). Methods: `update(doc, page)` (diff by widget JSON + effective family; unchanged widgets keep their nodes), `setMode`, `setBreakpoint`, `setFit`, `setEmptyAction`, `preview(items|null, {skip, animate, rows}) → count`, `metrics() → {bp, cols, rows, gap, colW, rowH, offX, mode, width}`, `items()`, `frame(id)`, `order()`. Forced width: lg `max(W, 960)`, md `clamp(W, 600, 768)`, sm `390`; written as `--hd-e-pw` + `data-hd-preview`. Design: page `role=listbox aria-multiselectable=true aria-label`, frames `role=option aria-selected=false` (aria-labelledby removed; restored in view), `aria-label` from `describe`; fit forced to scroll.
`E.view`: `setMode(m)` (root `hd-backdrop` class moves to the stage in design; view tabs hidden), `update(doc, {normalised})`, `select(pageId, {focus})`, `setBreakpoint`, `setFit`, `setDescribe`, `setEmptyAction`, `page()`, `stage`, `doc()`, `announce(text)`, `toast(text, {tone, ms=4000})` (`.hd-toasts`, ≤ 3), `on('page', fn)`; option `onEntities(ids)` after show/update/select.

### C14 Keyboard map (`E.commands.keymap`; root listener, `preventDefault` on every consumed key; ignored while `isTyping`)
Anywhere in the root: Ctrl/Cmd+Z undo; Ctrl/Cmd+Shift+Z, Ctrl+Y redo; Ctrl/Cmd+Alt+PageUp/PageDown page prev/next; Escape → escape ladder. Canvas (focus on a frame, the page or the stage): Ctrl/Cmd+C/X/V/D/A copy/cut/paste/duplicate/select all; Ctrl+Space toggle focused; Arrow nudge (selects the focused frame first); Shift+Arrow resize (single selection only, else `announce.resize_single`); Alt+Arrow spatial focus; Delete/Backspace delete; Space select focused; Enter/F2 focus inspector; Tab/Shift+Tab next/previous frame in reading order (at the ends the browser default leaves the canvas). Ctrl/Cmd+E (mount-level listener) toggles edit mode. Escape ladder: gesture → cancel; open drawer → close (focus to its toggle); selection → clear; else exit edit. Roving tabindex: one frame `tabindex=0` (focused, else primary, else first in reading order), others `-1`; empty page `tabindex=0`.

### C15 DOM/CSS contract (all selectors styled by part 2 in `ui/css/desktop-app-homedash.css`; motion only in `@media (prefers-reduced-motion: no-preference)` with selectors starting `body:not([data-animations="false"])`)
| Element / attribute | Written by | Meaning / geometry |
|---|---|---|
| `.hd-root[data-hd-mode=design]` + `[data-hd-drawers=true|false]` `[data-hd-panel=palette|inspector|'']` `[data-hd-gesture=move|resize|marquee|drop]` `[data-hd-has-palette]` `[data-hd-has-inspector]` | designer | grid: rows `editbar 48px / conflict-bar auto / 1fr`, cols `palette 280px / canvas 1fr / inspector 320px`; drawers below 900 px root width (panels absolute, ≤ 85 % width, scrim) |
| `.hd-editbar[role=group][aria-label]` > `.hd-eb-group[data-hd-group=start|pages|bp|end]` | editbar | 48 px, 8 px grid, 13 px text, 16 px icons; only `.hd-eb-done` uses accent fill |
| `.hd-eb-btn` (icon-only buttons have `aria-label` + `title`; disabled = `aria-disabled=true`), `.hd-eb-done`, `.hd-eb-label`, `.hd-eb-sep`, `.hd-eb-more[aria-haspopup=menu]`, `.hd-eb-drawer[aria-controls][aria-expanded]` (drawer mode only) | editbar | 32 px visual, 44 px with `(pointer: coarse)` |
| `[data-hd-slot=pages]` > `.hd-ptabs[role=tablist]` > `button.hd-ptab[role=tab][aria-selected][tabindex][data-page][aria-controls]` > `.hd-ptab-label` (part 2 adds `.hd-ptab-add`, `.hd-ptab-input`, `.hd-ptab-icon`, `[data-hd-dragging]`) | editbar / part 2 | roving tabs |
| `[data-hd-slot=bp]` > `.hd-bp[role=radiogroup]` > `button.hd-bp-opt[role=radio][aria-checked][data-bp][aria-label][title]`, `.hd-bp-chip[data-state=auto|custom]` (hidden for lg), `button.hd-bp-reset[data-hd-cmd=reset_layout]` | editbar | icons device-desktop/tablet/phone |
| `.hd-save[data-phase=saved|saving|retrying|invalid|conflict|readonly|gone]` > `.hd-save-icon` + `.hd-save-text` + `button.hd-save-action[data-hd-cmd]` | editbar | text box reserves ≥ 22ch (no shift); not a live region |
| `.hd-conflict-bar[role=alert][data-tone=warn]` > icon + `.hd-conflict-text` + `.hd-eb-btn[data-hd-cmd=resolve_conflict]` | designer | row 2 |
| `.hd-palette[role=region][id][aria-label][data-hd-drawer=open|closed]`, `.hd-inspector[…]` (shells; part 2 fills), `.hd-drawer-scrim[aria-hidden]` | designer | `--vd-mat-side` |
| `.hd-stage.hd-backdrop` (design) | view | canvas well (`--vd-mat-well`, `-well-shadow`) |
| `.hd-page[data-hd-preview=lg|md|sm]` `--hd-e-pw` | render | `inline-size: var(--hd-e-pw); margin-inline: auto` |
| `.hd-widget[role=option][aria-selected][tabindex][aria-label]` `[data-hd-draft]` `[data-hd-dragging]` `[data-hd-drop=rebind|invalid]` `[data-hd-flip=from]`; vars `--hd-e-fx/fy` (FLIP px), `--hd-e-dx/dy` (drag px) | render/gestures | design: `transform: translate(calc(var(--hd-e-fx,0px) + var(--hd-e-dx,0px)), calc(var(--hd-e-fy,0px) + var(--hd-e-dy,0px)))`; transition transform 140 ms except `[data-hd-flip=from]` and `[data-hd-dragging]`; selected: 1.5 px dashed `--vd-accent` outline offset 3 px; focus ring distinct (box-shadow); `touch-action: pan-y`, selected `none`; dragging z-index above |
| `.hd-draft-badge` (icon `plug` + text) inside draft frames | designer | centred plate |
| `.hd-edit-layer[aria-hidden=true]` (last child of `.hd-page`, `inset:0`, `pointer-events:none`, above widgets) | gestures | — |
| `.hd-ghost[data-hd-drop?]` (`--hd-x/y/w/h`, same calc as widgets) | gestures | 12 % accent wash + 1 px accent |
| `.hd-guide[data-axis=x|y][data-side=start|end]` `--hd-e-g` (edge index) | gestures | 1 px accent; x/start at `offx+gap+g*(colw+gap)`, x/end at `offx+g*(colw+gap)`; y likewise with rowh, offx 0 |
| `.hd-marquee` `--hd-e-mx/my/mw/mh` (px, page-local) | gestures | 1 px accent, 8 % wash |
| `.hd-sel[data-hd-wid]` (`--hd-x/y/w/h`) > `.hd-sel-chip` + 8 × `.hd-handle[data-edge][data-hd-limit]` | gestures | handles 8×8 visual in `--vd-mat-btn`, hit 12 px; `(pointer: coarse)` 44 px and only `se e s`; hidden while `data-hd-gesture=move`; chip `--vd-mat-card`, 12 px |
| `.hd-drop-avatar[data-hd-drop=add|rebind|invalid|out]` `--hd-e-ax/ay` (px, root-local) | gestures | absolute in root, `translate3d`, `pointer-events:none`, top z |
| `.hd-page-empty[data-hd-mode]` > … + `button.hd-page-empty-action[data-hd-cmd=edit|add_widget]` | render | — |
| `.hd-toasts[aria-hidden=true]` > `.hd-toast[data-tone]` | view | bottom centre |
| `.hd-sheet-layer` > `.hd-sheet[data-kind=confirm|conflict|leave|form]` | E.sheet | conflict/leave reuse the confirm paint; layer must cover the root |
| host `.hdx-edit`, `.hdx-elsewhere`, `.hdx-save[data-phase]`, `.hdx-app[data-hdx-edit]` | host | C3 |

### C16 Announcements and focus
One polite region (`.hd-live`, view); repeated text gets a trailing NBSP toggle. Announced: moves, resizes, limits/edges, adds (draft: `added_draft`), deletes ("… Undo with Ctrl+Z."), copy/cut, paste, marquee/select-all counts, selection cleared, undo/redo with label, materialised md/sm and reset, rebind/invalid drop, save transitions (to retrying, invalid, conflict, back to saved), reloads, enter/exit edit. Focus: before each render remember whether focus was in the stage and its widget id; after render refocus that frame (else `focusId`, else the page) with `preventScroll`; added/pasted widgets are selected, focused and scrolled into view (`block:'nearest'`); sheets restore focus to the opener.

### C17 i18n
Namespaces (part 1 fills; part 2 adds keys in its own sub-groups): `homedash.designer.*` (incl. `designer.announce.*`), `bp.*`, `save.*`, `conflict.*`, `leave.*`, `ctx.*`, `menu.*`, `cmd.*`, `error.*`, `widget.<type>.name` (part 2 adds `.desc`), `variant.<type>.<variant>`. Part 2 uses `palette.*`, `inspector.*`, `look.*`, `dialog.*`, `option.<key>.label|help`, `category.<cat>`, `family.<id>`, `device_class.*`. Dynamic prefixes registered in A11 (`ui/i18n_lint_test.go` `desktopDynamicI18nPrefixes`): `homedash.error.`, `homedash.widget.`, `homedash.variant.`, `homedash.cmd.`, `homedash.bp.`, `homedash.state.`, `homedash.kind.`, `homedash.source.`, `homedash.category.`, `homedash.option.`, `homedash.family.`, `homedash.device_class.`, `homedash.action.`, `homedash.icon.` (Q10: translated icon names, part 2).

### C18 Fixture API (`ui`, Go, test-only)
Routes added: `PUT dashboard?id=` (512 KiB → 413 `body_too_large`; queued answers; dropped connection; delay; revision check → 409 `{code:'revision_conflict', current_revision}`; lint → 422 `invalid_document` with `bad_json` (unknown field) or document issues whose codes follow `validator.text` of the server (a missing or empty name, an empty page name and a document without pages are `required`; a name over 80 or a page name over 40 characters `too_long`; more than 12 pages `too_many`); success 200 `{dashboard, warnings:[]}` + `dashboard_changed`), `POST dashboards/validate` (422 `bad_json`, else 200 `{valid, issues, warnings (entity_unknown), document}`), `POST dashboard/duplicate?id=` (201, name + " (copy)", slug `-copy`), `GET entities` filters `q, source, kind, area, limit 1–500, offset`; `POST dashboards {document}` now keeps the posted document (lint, kiosk default). Controls: `answerPut(status, body, times)`, `dropPut(n)`, `setPutDelay(d)`, `putLog() []homeDashPut`, `foreignSave(t, id, savedBy, mutate) int`, `dashboardDoc(id) map[string]any`.

### C19 Icons (new glyph names, HomeDash line style)
`undo redo copy cut paste edit duplicate panel-left panel-right window grip look layers align-left align-top search device-desktop device-tablet device-phone fullscreen sync cloud-off arrow-right`. `E.icons.choices()` = names minus UI glyphs (icon picker list).

### C20 Part-2 module interfaces
- `E.palette.create({host: D.slot('palette'), designer: D, t, lang, catalog, store, api}) → {el, focusSearch(), setTab('widgets'|'entities'), destroy()}`; palette types = `catalog.paletteGroups(E.widgets)`; drag via `D.gestures.dragFrom`; tap/Enter → `D.addWidgets([E.commands.entry(...)], {source:'palette'})`.
- `E.inspector.create({host: D.slot('inspector'), designer: D, t, lang, catalog, store, api}) → {el, focusFirst(), setTab(tab), destroy()}`; every change = one `D.exec(E.commands.…)`, text fields `text:true` + `D.seal()` on blur.
- `E.ddialogs = {ACTIONS:[…], run(ctx, action, arg) → Promise<boolean>, pageMenu?(D, pageId) → items, enhanceTabs?(D, slotEl)}`; `ctx = {root, t, lang, api, store, catalog, designer|null, mount (public surface), state(), open(id, pageId), home(), sheet, toast, refresh()}`.

---

## Task 0: Workspace
- [ ] `git branch --show-current`; `git merge-base --is-ancestor ac7a11c7f HEAD; echo $LASTEXITCODE` → 0. Copy `node_modules`.
- [ ] Baseline into the scratchpad: `npm run test:homedash` (count), `npm run check:ui`, the Go `./ui` filter above, `go test ./ui -run '^TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget$' -count=1` (8 known non-HomeDash failures), `go test ./internal/homedash -count=1`, `go test ./internal/audit -run '^TestUIBrowserCIShardsCoverEveryTest$' -count=1`, `TestDesktopHomeDashBrowser` + `TestDesktopHomeDashControlsBrowser` (R8).
- [ ] One MCP `impact({target:"realmFor"})` to confirm MCP answers.

---

## Batch A — foundations

### Task A1: Catalog `defaults` (Go, D2)
**Files:** `internal/homedash/catalog.go`, `catalog_widgets.go`, `catalog_test.go`, `testdata/widget_catalog.json`. **Existing symbols changed:** `WidgetCatalog` (struct field), `Catalog` (sets `Defaults`). Hash changes (lead-approved, D2).
- [ ] `catalog.go`: add after `Widgets`:
```go
	Defaults      []WidgetDefault  `json:"defaults"`
```
and the type:
```go
// WidgetDefault is one entry of the ordered widget defaults that pickWidgetFor (hd-catalog.js) reads: the first entry whose kind equals the
// entity's, whose device_class (when set) equals the entity's and whose capability (when set) the entity has wins. The client skips entries whose
// type it does not implement and falls back to entity/tile. Several catch-all entries of one kind form a fallback chain (fan: speed, then switch).
type WidgetDefault struct {
	Kind        Kind       `json:"kind"`
	DeviceClass string     `json:"device_class,omitempty"`
	Capability  Capability `json:"capability,omitempty"`
	Type        string     `json:"type"`
	Variant     string     `json:"variant"`
}
```
In `Catalog()` add `Defaults: widgetDefaults(),` after `Widgets`.
- [ ] `catalog_widgets.go`:
```go
// widgetDefaults agrees with the Home Assistant template (homedash_template_ha.go): dimmable lights get the slider, other lights the tile,
// switches the tile, scenes/scripts/buttons the action button. Select and number use list and slider (the template's option-count and box-mode
// refinements are not catalog data). Every kind has a catch-all entry; qualified entries come before it.
func widgetDefaults() []WidgetDefault {
	d := func(k Kind, dc string, c Capability, typ, v string) WidgetDefault {
		return WidgetDefault{Kind: k, DeviceClass: dc, Capability: c, Type: typ, Variant: v}
	}
	return []WidgetDefault{
		d(KindSwitch, "", "", "switch", "tile"),
		d(KindLight, "", CapBrightness, "light", "slider"), d(KindLight, "", "", "light", "tile"),
		d(KindSensor, "temperature", CapNumeric, "temperature", "numeral"), d(KindSensor, "humidity", CapNumeric, "gauge", "arc"),
		d(KindSensor, "battery", CapNumeric, "battery", "glyph"), d(KindSensor, "", "", "sensor", "numeral"),
		d(KindBinarySensor, "", "", "binary", "indicator"), d(KindClimate, "", "", "climate", "dial"), d(KindCover, "", "", "cover", "shutter"),
		d(KindFan, "", "", "fan", "speed"), d(KindFan, "", "", "switch", "tile"), d(KindLock, "", "", "lock", "slide"),
		d(KindMediaPlayer, "", "", "media", "compact"), d(KindCamera, "", "", "camera", "frame"),
		d(KindScene, "", "", "action", "button"), d(KindScript, "", "", "action", "button"), d(KindButton, "", "", "action", "button"),
		d(KindNumber, "", "", "number", "slider"), d(KindSelect, "", "", "select", "list"), d(KindHumidifier, "", "", "switch", "tile"),
		d(KindVacuum, "", "", "entity", "tile"), d(KindPresence, "", "", "binary", "indicator"), d(KindWeather, "", "", "weather", "compact"),
		d(KindSun, "", "", "entity", "tile"), d(KindUpdate, "", "", "entity", "tile"), d(KindServiceStatus, "", "", "entity", "tile"),
		d(KindPrinter, "", "", "entity", "tile"), d(KindGeneric, "", "", "entity", "tile"),
	}
}
```
- [ ] `catalog_test.go`:
```go
func TestWidgetCatalogDefaults(t *testing.T) {
	c := Catalog()
	catchAll := map[Kind]int{}
	for i, d := range c.Defaults {
		wt, ok := c.Type(d.Type)
		if !ok {
			t.Errorf("default %d: unknown type %s", i, d.Type)
			continue
		}
		if _, ok := wt.Variant(d.Variant); !ok {
			t.Errorf("default %d: %s has no variant %s", i, d.Type, d.Variant)
		}
		if !slices.Contains(c.Kinds, d.Kind) {
			t.Errorf("default %d: unknown kind %s", i, d.Kind)
		}
		b, ok := wt.Binding("primary")
		if !ok {
			t.Errorf("default %d: %s has no primary binding", i, d.Type)
			continue
		}
		if len(b.Kinds) > 0 && !slices.Contains(b.Kinds, d.Kind) {
			t.Errorf("default %d: %s.primary does not accept %s", i, d.Type, d.Kind)
		}
		if d.Capability != "" {
			if _, ok := LookupCapability(d.Capability); !ok {
				t.Errorf("default %d: unknown capability %s", i, d.Capability)
			}
		}
		if len(b.AnyCapability) > 0 && !slices.Contains(b.AnyCapability, d.Capability) {
			t.Errorf("default %d: %s.primary needs one of %v; set capability", i, d.Type, b.AnyCapability)
		}
		qualified := d.DeviceClass != "" || d.Capability != ""
		if j, seen := catchAll[d.Kind]; qualified && seen {
			t.Errorf("default %d (%s/%s) follows the catch-all %d of %s and is never used", i, d.Type, d.Variant, j, d.Kind)
		}
		if _, seen := catchAll[d.Kind]; !qualified && !seen {
			catchAll[d.Kind] = i
		}
	}
	for _, k := range c.Kinds {
		if _, ok := catchAll[k]; !ok {
			t.Errorf("kind %s has no catch-all default", k)
		}
	}
}
```
- [ ] `go test ./internal/homedash -run '^TestWidgetCatalogDefaults$' -count=1` → PASS; `go test ./internal/homedash -run '^TestWidgetCatalogGolden$' -update -count=1`; `go test ./internal/homedash ./internal/server -count=1` → PASS (no test pins the hash; `homedash_routes_test.go` reads it).
- [ ] Commit `catalog.go catalog_widgets.go catalog_test.go testdata/widget_catalog.json` — `feat(homedash): widget catalog defaults for pickWidgetFor (M4 D2)`.

### Task A2: hd-catalog — defaults, pickWidgetFor, palette groups
**Files:** `hd-catalog.js`, `scripts/homedash-tests/catalog.test.mjs`. **Existing symbols changed:** `create` (hd-catalog; additive methods), `E.catalog` export.
- [ ] Inside `create(raw)` before `return`:
```js
        const defaults = Object.freeze((Array.isArray(cat.defaults) ? cat.defaults : []).filter(d => d && typeof d.kind === 'string' && typeof d.type === 'string' && typeof d.variant === 'string'));
        const exact = (type, id) => { const w = byType.get(type); return w && Array.isArray(w.variants) ? w.variants.find(v => v && v.id === id) || null : null; };
        const capNames = e => new Set((e && Array.isArray(e.capabilities) ? e.capabilities : []).map(c => (c && typeof c === 'object' ? c.name : c)).filter(n => typeof n === 'string'));
        const usable = (registry, type, id) => !!exact(type, id) && (!registry || !!(registry.get(type) && registry.get(type).variants.includes(id)));
        // pickWidgetFor (frontend 5.3, M4 C10): the first catalog default with the entity's kind, its device_class when set and a capability the entity has
        // when set, whose type and variant the registry implements; else entity/tile.
        function pickWidgetFor(entity, registry) {
            const e = entity || {}, kind = typeof e.kind === 'string' && e.kind ? e.kind : 'generic', dc = typeof e.device_class === 'string' ? e.device_class : '', caps = capNames(e);
            for (const d of defaults) { if (d.kind !== kind || (d.device_class && d.device_class !== dc) || (d.capability && !caps.has(d.capability))) continue; if (usable(registry, d.type, d.variant)) return { type: d.type, variant: d.variant }; }
            return usable(registry, 'entity', 'tile') ? { type: 'entity', variant: 'tile' } : null;
        }
        const binding = (type, name) => { const w = byType.get(type); return w && Array.isArray(w.bindings) ? w.bindings.find(b => b && b.name === name) || null : null; };
        function accepts(type, name, entity) {
            const b = binding(type, name), e = entity || {}; if (!b) return false;
            const kinds = Array.isArray(b.kinds) ? b.kinds : []; if (kinds.length && !kinds.includes(e.kind)) return false;
            const any = Array.isArray(b.any_capability) ? b.any_capability : []; if (!any.length) return true; const caps = capNames(e); return any.some(c => caps.has(c));
        }
        const optionSpecs = type => { const w = byType.get(type); return (w && Array.isArray(w.options) ? w.options : []).concat(Array.isArray(cat.common_options) ? cat.common_options : []).filter(s => s && typeof s.key === 'string'); };
        const canToggle = type => { const w = byType.get(type), list = w && w.commands && Array.isArray(w.commands.primary) ? w.commands.primary : []; return list.includes('toggle'); };
        function paletteGroups(registry) {
            const groups = new Map();
            for (const w of byType.values()) {
                const def = registry && registry.get(w.type); if (!def) continue;
                const variants = (w.variants || []).map(v => v && v.id).filter(v => def.variants.includes(v)); if (!variants.length) continue;
                const c = typeof w.category === 'string' && w.category ? w.category : 'status'; if (!groups.has(c)) groups.set(c, []);
                groups.get(c).push(Object.freeze({ type: w.type, variants: Object.freeze(variants) }));
            }
            const order = CATEGORY_ORDER.concat([...groups.keys()].filter(k => !CATEGORY_ORDER.includes(k)).sort());
            return Object.freeze(order.filter(k => groups.has(k)).map(k => Object.freeze({ category: k, types: Object.freeze(groups.get(k)) })));
        }
```
Return object adds `defaults: () => defaults, pickWidgetFor, accepts, binding, optionSpecs, canToggle, families: () => (Array.isArray(cat.families) ? cat.families.filter(f => typeof f === 'string') : []), paletteTokens: () => (Array.isArray(cat.palette_tokens) ? cat.palette_tokens.filter(n => typeof n === 'string').slice(0, 40) : []), paletteGroups`. Module top: `const CATEGORY_ORDER = Object.freeze(['control', 'climate', 'sensor', 'media', 'camera', 'status', 'layout']);`; export `E.catalog = Object.freeze({ create, CATEGORY_ORDER })`. Header comment gains one line naming C10.
- [ ] Tests. The first draft compared vm-created arrays with `assert.deepEqual` (reference inequality across the `vm` realm, dry-run defect 1); the tests use the harness `eq`. Apply this diff to `scripts/homedash-tests/catalog.test.mjs` (two tests added after the existing one):
<!-- embed: dryrun-m4/overlay/diffs/catalog.test.mjs.diff -->
```diff
--- a/scripts/homedash-tests/catalog.test.mjs
+++ b/scripts/homedash-tests/catalog.test.mjs
@@ -6,2 +6,20 @@ export const tests = [['catalog: sizes, constraints and option defaults come fro
   eq(c.options({ type: 'gauge', options: { warn: 80 } }), { min: 0, max: 100, decimals: 0, warn: 80 }); eq(c.options({ type: 'entity' }), {});
-  eq(c.registryMismatches(E.widgets), []); }]];
+  eq(c.registryMismatches(E.widgets), []); }],
+  ['catalog: pickWidgetFor follows the ordered defaults, skips unregistered types and falls back to entity', () => {
+    const { E } = loadEngine(ENGINE_ORDER.concat(['hd-catalog.js'])), c = E.catalog.create(JSON.parse(read('internal/homedash/testdata/widget_catalog.json'))), R = E.widgets;
+    const ent = (kind, dc, caps) => ({ kind, device_class: dc || '', capabilities: (caps || []).map(name => ({ name, allowed: true })) });
+    assert.deepEqual({ ...c.pickWidgetFor(ent('light', '', ['brightness']), R) }, { type: 'light', variant: 'slider' });
+    assert.deepEqual({ ...c.pickWidgetFor(ent('light'), R) }, { type: 'light', variant: 'tile' });
+    assert.deepEqual({ ...c.pickWidgetFor(ent('sensor', 'temperature', ['numeric']), R) }, { type: 'temperature', variant: 'numeral' });
+    assert.deepEqual({ ...c.pickWidgetFor(ent('sensor', 'battery', ['numeric']), R) }, { type: 'sensor', variant: 'numeral' }); // battery not registered: next match
+    assert.deepEqual({ ...c.pickWidgetFor(ent('fan'), R) }, { type: 'switch', variant: 'tile' });                                 // fan chain
+    assert.deepEqual({ ...c.pickWidgetFor(ent('cover'), R) }, { type: 'entity', variant: 'tile' });
+    assert.deepEqual({ ...c.pickWidgetFor({}, R) }, { type: 'entity', variant: 'tile' });
+    assert.equal(c.accepts('light', 'primary', ent('switch')), false); assert.equal(c.accepts('switch', 'primary', ent('light')), true);
+    assert.equal(c.accepts('gauge', 'primary', ent('sensor')), false); assert.equal(c.accepts('gauge', 'primary', ent('sensor', '', ['numeric'])), true); }],
+  ['catalog: palette = catalog ∩ registry, in category order, no hard-coded list', () => {
+    const { E } = loadEngine(ENGINE_ORDER.concat(['hd-catalog.js'])), raw = JSON.parse(read('internal/homedash/testdata/widget_catalog.json')), c = E.catalog.create(raw);
+    const groups = c.paletteGroups(E.widgets), types = groups.flatMap(g => g.types.map(x => x.type)).sort();
+    eq(types, raw.widgets.map(w => w.type).filter(t => E.widgets.get(t)).sort());
+    const cats = groups.map(g => g.category); eq(cats, E.catalog.CATEGORY_ORDER.filter(k => cats.includes(k)));
+    assert.ok(groups.every(g => g.types.length > 0)); assert.ok(c.paletteTokens().length <= 40 && c.paletteTokens().includes('surface-solid')); }]];
```
- [ ] `npm run test:homedash` → PASS. Commit — `feat(homedash): pickWidgetFor, binding checks and palette groups from the catalog`.

### Task A3: Layout memo, normalise constraints, kiosk default, drafts, forSave, view palette.custom
**Files:** `hd-layout.js`, `hd-doc.js`, `hd-view.js`, `layout.test.mjs`, `doc.test.mjs`, `view.test.mjs`. **Existing symbols changed:** `effective` (hd-layout; callers render, view, doc), `normalise`, `normalisePage` (hd-doc; callers view, tests), removal of `toClipboard`/`fromClipboard` (callers: `doc.test.mjs` only — confirm by impact), `applyRoot` (hd-view).
- [ ] `hd-layout.js` replace `effective`:
```js
    const signature = (lg, C) => { let s = ''; for (const it of lg) { const c = constraintOf(C, it.id); s += it.id + ':' + (c.minW || 1) + ',' + (c.minH || 1) + ',' + (c.maxW || 0) + ',' + (c.maxH || 0) + ';'; } return s; };
    // Memo keyed on the lg array (identity) and the constraints of its items: a variant change refreshes md and sm. An override array is returned as it is.
    function effective(layouts, bp, C) {
        if (bp === 'lg') return layouts.lg;
        if (Array.isArray(layouts[bp])) return layouts[bp];
        let entry = memo.get(layouts.lg); if (!entry) { entry = new Map(); memo.set(layouts.lg, entry); }
        const key = bp + '|' + signature(layouts.lg, C); let out = entry.get(key);
        if (!out) { out = derive(layouts.lg, COLS[bp], C); entry.set(key, out); if (entry.size > 6) entry.delete(entry.keys().next().value); }
        return out;
    }
```
- [ ] `hd-doc.js`: `normalisePage(raw, index, sizeOf, limitOf)` builds `const byId = new Map(widgets.map(w => [w.id, w])), C = id => { const w = byId.get(id); return w && limitOf ? limitOf(w) || {} : {}; };` and passes `C` to both `repair` calls, to `addToLayouts(…, C)` and `derivedSize(…, C(w.id))`. `normalise(raw, opts)`: `const limitOf = opts && opts.constraints;` → `normalisePage(p, i, sizeOf, limitOf)`; after page-id dedupe:
```js
        const k = isObj(d.kiosk) ? clone(d.kiosk) : { start_page: pages[0].id, idle_dim_seconds: 120, idle_dim_level: 0.3, page_rotation_seconds: 0, screen_saver: 'clock', show_clock: true };
        if (!pages.some(p => p.id === k.start_page)) k.start_page = pages[0].id;
```
and return adds `kiosk: k`. Remove `toClipboard` and `fromClipboard`. Add:
```js
    // Drafts (M4 reading 1): widgets whose required binding or required option is still empty. The server refuses a document with them.
    function drafts(doc, catalog) {
        const out = new Set(); if (!doc || !catalog || !Array.isArray(doc.pages)) return out;
        for (const p of doc.pages) for (const w of p.widgets || []) {
            const spec = catalog.widget(w.type); if (!spec) continue;
            const empty = v => v === undefined || v === null || v === '';
            if ((spec.bindings || []).some(b => b && b.required && empty(w.bindings && w.bindings[b.name])) || (spec.options || []).some(s => s && s.required && empty(w.options && w.options[s.key]))) out.add(w.id);
        }
        return out;
    }
    // The body of a save, export or copy: drafts left out with their layout items; look.palette.snapshot written for mode theme (kept when none is
    // given) and dropped otherwise; palette.custom dropped unless the mode is custom.
    function forSave(doc, opts) {
        const o = opts || {}, skip = o.catalog ? drafts(doc, o.catalog) : new Set(); let out = doc;
        if (skip.size) out = Object.assign({}, out, { pages: out.pages.map(p => (p.widgets.some(w => skip.has(w.id)) ? Object.assign({}, p, { widgets: p.widgets.filter(w => !skip.has(w.id)), layouts: E.layout.removeFromLayouts(p.layouts, [...skip]) }) : p)) });
        const pal = out.look && out.look.palette;
        if (pal) { const next = Object.assign({}, pal);
            if (next.mode === 'theme' && o.snapshot && Object.keys(o.snapshot).length) next.snapshot = Object.assign({}, o.snapshot); else if (next.mode !== 'theme') delete next.snapshot;
            if (next.mode !== 'custom') delete next.custom;
            out = Object.assign({}, out, { look: Object.assign({}, out.look, { palette: next }) }); }
        return out;
    }
```
Export: `E.doc = Object.freeze({ normalise, keyOf, getAt, setAt, apply, invert, command, op, setField, addWidget, removeWidgets, setLayout, drafts, forSave })`. Header: "clipboard moved to hd-clipboard.js".
- [ ] `hd-view.js` `applyRoot`: `const custom = mode === 'custom' ? (look.palette.custom && typeof look.palette.custom === 'object' ? look.palette.custom : look.palette) : null;` `sch` uses `custom.scheme`; `solved = custom ? E.colour.solvePalette(custom).tokens : null`.
- [ ] Tests, as diffs against the existing test files (every other test of the file stays): `layout.test.mjs` (memo keyed on the constraints), `doc.test.mjs` (the clipboard test is removed; normalise with constraints, kiosk default and repair, idempotence; drafts and forSave), `view.test.mjs` (server-shaped `palette.custom`). No existing normalise expectation needed a `kiosk` adjustment in the dry run.
<!-- embed: dryrun-m4/overlay/diffs/layout.test.mjs.diff -->
```diff
--- a/scripts/homedash-tests/layout.test.mjs
+++ b/scripts/homedash-tests/layout.test.mjs
@@ -10,2 +10,6 @@ export const tests = [
     assert.equal(l.effective(lay, 'md'), l.effective(lay, 'md')); assert.equal(l.effective(lay, 'sm'), lay.sm); assert.equal(l.effective(lay, 'lg'), LG); }],
+  ['effective memo is keyed on the constraints too (M4)', () => { const l = Lx(); const lay = { lg: LG, md: null, sm: null }, C1 = () => ({}), C2 = id => (id === 'c' ? { minW: 3 } : {});
+    assert.equal(l.effective(lay, 'md', C1), l.effective(lay, 'md', () => ({}))); assert.notEqual(l.effective(lay, 'md', C1), l.effective(lay, 'md', C2));
+    assert.equal(map(l.effective(lay, 'md', C1)).c[2], 2); assert.equal(map(l.effective(lay, 'md', C2)).c[2], 3); assert.equal(l.effective(lay, 'md', C2), l.effective(lay, 'md', C2));
+    for (let k = 0; k < 12; k++) l.effective(lay, 'sm', id => ({ minW: 1 + (k % 4), maxW: 4 })); eq(map(l.effective(lay, 'md', C2)).c[2], 3); }],
   ['breakpoint with 16 px hysteresis', () => { const b = Lx().breakpoint;
```

<!-- embed: dryrun-m4/overlay/diffs/doc.test.mjs.diff -->
```diff
--- a/scripts/homedash-tests/doc.test.mjs
+++ b/scripts/homedash-tests/doc.test.mjs
@@ -23,5 +23,22 @@ export const tests = [
     const md = [{ id: 'w1', x: 0, y: 0, w: 4, h: 1 }]; eq(E.doc.apply(D, E.doc.setLayout(D, 'main', 'md', md).ops).pages[0].layouts.md, md); }],
-  ['clipboard round trip with new ids', () => { const E = boot(), D = base(E); const items = E.doc.fromClipboard(E.doc.toClipboard(D, 'main', ['w1']));
-    assert.equal(items.length, 1); assert.equal(items[0].widget.type, 'sensor'); assert.match(items[0].widget.id, /^w_/); eq([items[0].w, items[0].h], [2, 1]);
-    eq(E.doc.fromClipboard('nope'), []); eq(E.doc.fromClipboard('{"v":2}'), []); }],
+  ['normalise (M4): constraints clamp lg, kiosk default and repair, idempotent', () => { const E = boot();
+    const raw = { pages: [{ id: 'a', widgets: [{ id: 'w1', type: 'sensor' }], layouts: { lg: [{ id: 'w1', x: 0, y: 0, w: 6, h: 1 }] } }, { id: 'b' }] };
+    const n = E.doc.normalise(raw, { constraints: () => ({ minW: 1, maxW: 4, minH: 1, maxH: 24 }) }); assert.equal(n.pages[0].layouts.lg[0].w, 4);
+    eq(n.kiosk, { start_page: 'a', idle_dim_seconds: 120, idle_dim_level: 0.3, page_rotation_seconds: 0, screen_saver: 'clock', show_clock: true });
+    assert.equal(E.doc.normalise(Object.assign({}, raw, { kiosk: { start_page: 'gone', idle_dim_seconds: 5 } })).kiosk.start_page, 'a');
+    assert.equal(E.doc.normalise(Object.assign({}, raw, { kiosk: { start_page: 'b' } })).kiosk.start_page, 'b');
+    const again = E.doc.normalise(n, { constraints: () => ({ minW: 1, maxW: 4, minH: 1, maxH: 24 }) }); eq(again, n);
+    const wide = E.doc.normalise({ pages: [{ id: 'a', widgets: [{ id: 'w1', type: 't' }], layouts: { lg: [{ id: 'w1', x: 0, y: 0, w: 4, h: 1 }], md: null, sm: [] } }] }, { constraints: () => ({ minW: 3 }) });
+    assert.equal(wide.pages[0].layouts.sm[0].w, 3); }],
+  ['drafts and forSave (M4): drafts leave the body with their layout items, palette fields follow the mode', () => { const E = boot();
+    const cat = { widget: type => (type === 'switch' ? { bindings: [{ name: 'primary', required: true }], options: [] } : type === 'gauge' ? { bindings: [], options: [{ key: 'unit', required: true }] } : { bindings: [], options: [] }) };
+    const D = E.doc.normalise({ look: { palette: { mode: 'dark', custom: { accent: '#fff' }, snapshot: { ink: '#111111' } } }, pages: [{ id: 'main', widgets: [{ id: 'w1', type: 'sensor' }, { id: 'w2', type: 'switch' }, { id: 'w3', type: 'gauge', options: { unit: '' } }, { id: 'w4', type: 'switch', bindings: { primary: 'ha:light.x' } }],
+      layouts: { lg: [], md: [{ id: 'w2', x: 0, y: 0, w: 2, h: 1 }], sm: null } }] });
+    eq([...E.doc.drafts(D, cat)].sort(), ['w2', 'w3']);
+    const S = E.doc.forSave(D, { catalog: cat }); eq(S.pages[0].widgets.map(w => w.id), ['w1', 'w4']); eq(S.pages[0].layouts.lg.map(i => i.id).sort(), ['w1', 'w4']); eq(S.pages[0].layouts.md.map(i => i.id).sort(), ['w1', 'w4']);
+    assert.equal(S.pages[0].layouts.sm, null); assert.equal(S.look.palette.custom, undefined); assert.equal(S.look.palette.snapshot, undefined); assert.ok(D.pages[0].widgets.length === 4, 'input untouched');
+    const T = E.doc.normalise({ look: { palette: { mode: 'theme', snapshot: { ink: '#111111' }, custom: { accent: '#fff' } } } });
+    eq(E.doc.forSave(T, { snapshot: { ink: '#222222' } }).look.palette, { mode: 'theme', snapshot: { ink: '#222222' } }); eq(E.doc.forSave(T).look.palette, { mode: 'theme', snapshot: { ink: '#111111' } });
+    const C = E.doc.normalise({ look: { palette: { mode: 'custom', custom: { accent: '#fff' }, snapshot: { ink: '#111111' } } } }); eq(E.doc.forSave(C, { snapshot: { a: '#000000' } }).look.palette, { mode: 'custom', custom: { accent: '#fff' } });
+    assert.equal(E.doc.forSave(E.doc.normalise({})).pages.length, 1); }],
   ['history: undo, redo, redo cleared, limit, coalescing, seal', () => { const E = boot(), D = base(E), N = ['pages', 'main', 'name']; const h = E.history.create();
```

<!-- embed: dryrun-m4/overlay/diffs/view.test.mjs.diff -->
```diff
--- a/scripts/homedash-tests/view.test.mjs
+++ b/scripts/homedash-tests/view.test.mjs
@@ -29,2 +29,10 @@ export const tests = [
     view.destroy(); assert.equal(root.children.length, 0); }],
+  ['view (M4): the server shape palette.custom {accent, surface_hue, scheme} drives the scheme and the solved tokens', () => {
+    const { E, document } = loadEngine(FILES, { dom: true }), root = E.el('div', { class: 'hd-root' }); document.body.appendChild(root);
+    const bus = E.core.emitter(), store = { view: () => null, on: bus.on, off: bus.off, history: () => Promise.resolve({ points: [], source: 'none' }) };
+    const view = E.view.create({ root, store, catalog: E.catalog.create(CATALOG), t: k => k, lang: 'en', host: 'desktop', scheme: () => 'dark', motion: () => 'full', raf: fn => 1, caf: () => {}, measure: () => ({ width: 1180, height: 700 }) });
+    const custom = { accent: '#f5c542', surface_hue: 250, scheme: 'light' }, solved = E.colour.solvePalette(custom).tokens;
+    view.show(Object.assign({}, TWO, { look: { family: 'minimal', palette: { mode: 'custom', custom } } }));
+    assert.equal(root.getAttribute('data-hd-scheme'), 'light'); assert.equal(root.style.getPropertyValue('--hd-ink'), solved.ink); assert.equal(root.style.getPropertyValue('--hd-n-accent'), solved.accent);
+    view.destroy(); }],
   ['view: a custom palette leaves the five applyLook-owned tokens to applyLook and applies the rest', () => {
```
- [ ] `npm run test:homedash` → PASS. Commit — `fix(homedash): layout memo by constraints, normalise with constraints and kiosk, drafts and save body, custom palette shape`.

### Task A4: Solver fix (design 12.4, D7)
**Files:** `hd-colour.js`, `colour.test.mjs`. **Existing symbols changed:** `solvePalette` (callers view, lab).
- [ ] Replace step 3 in `solvePalette`:
```js
        // 3: on = the accent moved least in lightness so that on-ink (black or white, the better one for the accent first) reaches 4.5:1 on it and on
        // reaches 3:1 on surface-solid (design 12.4: the ON fill carries state). Both inks are tried; on a dark surface black ink and on a light one
        // white ink always fit, so the search ends inside the loop.
        const accentLch = rgbToOklch(accentRgb), a0 = oklchToHex(accentLch), inks = contrast(a0, '#FFFFFF') >= contrast(a0, '#000000') ? ['#FFFFFF', '#000000'] : ['#000000', '#FFFFFF'];
        let onHex = null, onInk = inks[0];
        for (const ink of inks) { for (let k = 0; k <= 100 && !onHex; k++) for (const dir of k ? [1, -1] : [1]) {
            const L = Math.round((accentLch[0] + dir * k * 0.01) * 1e6) / 1e6; if (L < 0 || L > 1) continue;
            const hex = oklchToHex([L, accentLch[1], accentLch[2]]); if (contrast(hex, ink) >= 4.5 && contrast(hex, t['surface-solid']) >= 3) { onHex = hex; onInk = ink; break; } }
            if (onHex) break; }
        t['on-ink'] = onInk; t.on = onHex || (onInk === '#FFFFFF' ? '#000000' : '#FFFFFF');
```
(step 4 keeps `accentLch`.)
- [ ] `colour.test.mjs` (the new `on`/`surface-solid` pair joins the 500-input list; a navy and a pale accent case):
<!-- embed: dryrun-m4/overlay/diffs/colour.test.mjs.diff -->
```diff
--- a/scripts/homedash-tests/colour.test.mjs
+++ b/scripts/homedash-tests/colour.test.mjs
@@ -15,3 +15,3 @@ export const tests = [
       for (const name of C.PALETTE_TOKENS) assert.match(t[name], /^#[0-9A-F]{6}$/, name + ' ' + tag);
-      for (const [a, b, min] of [['ink','surface-solid',4.5],['ink','canvas',4.5],['ink-muted','surface-solid',5],['ink-muted','canvas',4.5],['on-ink','on',4.5],['ink-on-accent','accent',4.5],['accent','surface-solid',3],['selected-ink','selected',4.5],['btn-ink','btn',4.5],['ok','surface-solid',4.5],['warn','surface-solid',4.5],['alert','surface-solid',4.5],['info','surface-solid',4.5]])
+      for (const [a, b, min] of [['ink','surface-solid',4.5],['ink','canvas',4.5],['ink-muted','surface-solid',5],['ink-muted','canvas',4.5],['on-ink','on',4.5],['on','surface-solid',3],['ink-on-accent','accent',4.5],['accent','surface-solid',3],['selected-ink','selected',4.5],['btn-ink','btn',4.5],['ok','surface-solid',4.5],['warn','surface-solid',4.5],['alert','surface-solid',4.5],['info','surface-solid',4.5]])
         assert.ok(at(a, b) >= min, `${a} on ${b} ${at(a, b).toFixed(2)} ${tag}`);
@@ -30,2 +30,5 @@ export const tests = [
     assert.equal(C.ensureContrast('#7AA7FF', '#808080', 99), C.ensureContrast('#7AA7FF', '#808080', 99), 'unreachable minimum returns the best extreme deterministically'); }],
+  ['solver (M4): on reaches 4.5 with its ink and 3:1 on the surface for a dark navy accent', () => { const C = boot().E.colour;
+    for (const input of [{ accent: '#1a237e', surface_hue: 220, scheme: 'dark' }, { accent: '#fff59d', surface_hue: 60, scheme: 'light' }]) { const t = C.solvePalette(input).tokens;
+      assert.ok(C.contrast(t.on, t['on-ink']) >= 4.5, JSON.stringify(input)); assert.ok(C.contrast(t.on, t['surface-solid']) >= 3, JSON.stringify(input)); } }],
   ['applyPalette writes --hd- custom properties', () => { const { E, document } = boot(true); const el = document.createElement('div');
```
- [ ] `npm run test:homedash` → PASS (500 inputs). Commit — `fix(homedash): custom palette ON fill reaches 3:1 on the surface (design 12.4)`.

### Task A5: hd-api additions
**Files:** `hd-api.js`, `api.test.mjs`. **Existing symbols changed:** `create`, `call` (hd-api; every API caller — additive `keepalive`).
- [ ] In `call`: `if (x.keepalive) init.keepalive = true;`. Add:
```js
    function entityQuery(p) { const x = p || {}, int = (v, d, lo, hi) => (Number.isFinite(Number(v)) ? Math.min(hi, Math.max(lo, Math.round(Number(v)))) : d);
        const cut = (v, n) => (typeof v === 'string' ? Array.from(v.trim()).slice(0, n).join('') : undefined);
        return { q: cut(x.q, 100), source: typeof x.source === 'string' ? x.source : undefined, kind: typeof x.kind === 'string' ? x.kind : undefined, area: cut(x.area, 80), limit: int(x.limit, 100, 1, 500), offset: int(x.offset, 0, 0, 1 << 30) }; }
```
and in the returned object:
```js
            saveDashboard: async (id, revision, document, extra) => call('PUT', 'dashboard', { id: dashboardId(id) }, { revision, document }, extra),
            validateDocument: (document, extra) => call('POST', 'dashboards/validate', null, { document }, extra),
            duplicateDashboard: async (id, name, extra) => call('POST', 'dashboard/duplicate', { id: dashboardId(id) }, typeof name === 'string' && name ? { name } : {}, extra),
            entities: (params, extra) => call('GET', 'entities', entityQuery(params), undefined, extra),
            entity: (id, extra) => call('GET', 'entity', { id: String(id) }, undefined, extra),
```
Header line: "Routes include dashboard PUT, validate, duplicate and entities (M4 C9)".
- [ ] Test (diff against `api.test.mjs`; the transport script has six answers):
<!-- embed: dryrun-m4/overlay/diffs/api.test.mjs.diff -->
```diff
--- a/scripts/homedash-tests/api.test.mjs
+++ b/scripts/homedash-tests/api.test.mjs
@@ -55,3 +55,13 @@ export const tests = [
     assert.equal(tr.calls[2].init.method + ' ' + tr.calls[2].url, 'POST /api/desktop/homedash/commands');
-    eq(JSON.parse(tr.calls[2].init.body).args, { value: 40 }); }]
+    eq(JSON.parse(tr.calls[2].init.body).args, { value: 40 }); }],
+  ['hd-api saves, validates, duplicates and lists entities (M4 C9)', async () => {
+    const E = load(), tr = fakeTransport([{ status: 'ok', dashboard: { revision: 4 } }, { status: 'ok', valid: true }, { status: 'ok' }, { status: 'ok', entities: [] }, { status: 'ok' }, { status: 'ok' }]), api = E.api.create({ transport: tr });
+    await api.saveDashboard('d_abcdefghijkl', 3, { name: 'X' }, { keepalive: true }); await api.validateDocument({ name: 'Y' }); await api.duplicateDashboard('d_abcdefghijkl');
+    await api.entities({ q: '  lamp ', kind: 'light', limit: 900, offset: -2 });
+    assert.equal(tr.calls[0].init.method + ' ' + tr.calls[0].url, 'PUT /api/desktop/homedash/dashboard?id=d_abcdefghijkl'); eq(JSON.parse(tr.calls[0].init.body), { revision: 3, document: { name: 'X' } }); assert.equal(tr.calls[0].init.keepalive, true);
+    assert.equal(tr.calls[1].url, '/api/desktop/homedash/dashboards/validate'); assert.equal(tr.calls[2].url, '/api/desktop/homedash/dashboard/duplicate?id=d_abcdefghijkl'); eq(JSON.parse(tr.calls[2].init.body), {});
+    assert.equal(tr.calls[3].url, '/api/desktop/homedash/entities?q=lamp&kind=light&limit=500&offset=0');
+    await api.duplicateDashboard('d_abcdefghijkl', 'Copy'); eq(JSON.parse(tr.calls[4].init.body), { name: 'Copy' }); await api.entity('ha:light.x'); assert.equal(tr.calls[5].url, '/api/desktop/homedash/entity?id=ha%3Alight.x');
+    assert.equal(tr.calls[1].init.keepalive, undefined);
+    await assert.rejects(api.saveDashboard('nope', 1, {}), e => e.code === 'bad_request'); }]
 ];
```
- [ ] PASS. Commit — `feat(homedash): api routes for save, validate, duplicate and entities`.

### Task A6: Store docs cache, edit lock, presence
**Files:** `hd-store.js`, `store.test.mjs`. **Existing symbols changed:** `create` (hd-store; return object, `dispose`).
- [ ] Header rule `9 Dashboard documents …` (C7 text). In `create`:
```js
        const docCache = new Map(), locks = new Map(), presence = new Map();
        const docs = Object.freeze({
            get: id => docCache.get(id) || null,
            put(id, d) { if (typeof id !== 'string' || !isObj(d)) return false; const rev = Number(d.revision) || 0, have = docCache.get(id);
                if (have && have.revision > rev) return false; docCache.set(id, Object.freeze({ doc: d, revision: rev, loadedAt: now() })); bus.emit('doc', { id, revision: rev }); return true; },
            drop(id) { if (docCache.delete(id)) bus.emit('doc', { id, revision: 0 }); },
            lock(id, owner) { const cur = locks.get(id); if (cur && cur !== owner) return false; if (!cur) { locks.set(id, owner); bus.emit('lock', { id, owner }); } return true; },
            unlock(id, owner) { if (locks.get(id) !== owner) return false; locks.delete(id); bus.emit('lock', { id, owner: null }); return true; },
            editor: id => locks.get(id) || null,
            presence(owner, handle) { if (handle) { presence.set(owner, handle); return; } presence.delete(owner);
                for (const [id, o] of [...locks]) if (o === owner) { locks.delete(id); bus.emit('lock', { id, owner: null }); } },
            focusEditor(id) { const o = locks.get(id), h = o && presence.get(o); if (!h || typeof h.focus !== 'function') return false; h.focus(); return true; }
        });
```
Return adds `docs`; `dispose` adds `docCache.clear(); locks.clear(); presence.clear();`.
- [ ] Test (diff against `store.test.mjs`; it uses the file's existing `setup()`):
<!-- embed: dryrun-m4/overlay/diffs/store.test.mjs.diff -->
```diff
--- a/scripts/homedash-tests/store.test.mjs
+++ b/scripts/homedash-tests/store.test.mjs
@@ -115,3 +115,13 @@ export const tests = [
     assert.equal((await store.widgetCatalog('h1')).hash, 'h1'); assert.equal((await store.widgetCatalog('h1')).hash, 'h1'); eq(asked, ['']);
-    answer = { status: 'ok', not_modified: true }; assert.equal((await store.widgetCatalog('h2')).hash, 'h1'); eq(asked, ['', 'h1']); }]
+    answer = { status: 'ok', not_modified: true }; assert.equal((await store.widgetCatalog('h2')).hash, 'h1'); eq(asked, ['', 'h1']); }],
+  ['store (M4 C7): docs cache keeps the newest revision, one editor lock, presence release, focusEditor, dispose clears', () => {
+    const { store, clock } = setup(), docs = store.docs, seen = [], locks = [];
+    store.on('doc', d => seen.push(d.id + '@' + d.revision)); store.on('lock', d => locks.push(d.id + ':' + d.owner));
+    assert.equal(docs.get('a'), null); assert.equal(docs.put('a', { revision: 3, name: 'x' }), true); assert.equal(docs.put('a', { revision: 2 }), false); assert.equal(docs.get('a').revision, 3);
+    assert.equal(docs.put('a', { revision: 3, name: 'same' }), true); assert.equal(docs.get('a').doc.name, 'same'); assert.equal(docs.put('a', null), false); assert.equal(docs.put(7, {}), false);
+    eq(seen, ['a@3', 'a@3']); docs.drop('a'); assert.equal(docs.get('a'), null); eq(seen, ['a@3', 'a@3', 'a@0']);
+    assert.equal(docs.lock('d1', 'A'), true); assert.equal(docs.lock('d1', 'B'), false); assert.equal(docs.lock('d1', 'A'), true); assert.equal(docs.editor('d1'), 'A'); eq(locks, ['d1:A']);
+    assert.equal(docs.unlock('d1', 'B'), false); let focused = 0; docs.presence('A', { focus: () => { focused++; } }); assert.equal(docs.focusEditor('d1'), true); assert.equal(focused, 1); assert.equal(docs.focusEditor('nope'), false);
+    docs.lock('d2', 'A'); docs.presence('A', null); assert.equal(docs.editor('d1'), null); assert.equal(docs.editor('d2'), null); assert.ok(locks.includes('d1:null') && locks.includes('d2:null')); assert.equal(docs.focusEditor('d1'), false);
+    docs.lock('d3', 'A'); docs.put('z', { revision: 1 }); store.dispose(); assert.equal(docs.editor('d3'), null); assert.equal(docs.get('z'), null); void clock; }]
 ];
```
- [ ] PASS. Commit — `feat(homedash): realm docs cache and one-editor lock`.

### Task A7: hd-clipboard.js (new)
**Files:** create `hd-clipboard.js`, `clipboard.test.mjs`; wiring step W (after `hd-view.js`). **Existing symbols changed:** none (lists only).
- [ ] Full file. The first draft had two defects the dry run fixed here: the entity-id grammar is the server's `ParseEntityID` (the old regex rejected `bt:aa:bb:cc` and accepted control characters, one-character sources and sources over 16), and `text()` follows the server's text rules (control, bidi, URL) instead of stripping only `\x00-\x1f\x7f`. The vm context has no `TextEncoder`, hence the byte loop.
<!-- embed: dryrun-m4/ui/js/homedash/hd-clipboard.js -->
```js
// HomeDash clipboard (frontend 5.5, M4 contract C11). encode(doc, pageId, ids) is the text kept at localStorage['homedash.clipboard.v1']:
// {v:1, widgets:[{widget (no id), lgSize:{w,h}}]} in lg reading order. decode(text, {catalog, pages}) treats the text as untrusted (another window,
// tab or a hand edit): at most 256 KiB, v 1, at most 120 widgets, each rebuilt from known fields only (sanitizeWidget) and sized within its variant.
// channel({storage, win}) wraps the storage: write() tells this realm's listeners at once, the 'storage' event tells other tabs.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.grid) throw new Error('hd-clipboard.js: load hd-core.js and hd-grid.js first');
    const { clone, clamp } = E.core, KEY = 'homedash.clipboard.v1', MAX_BYTES = 262144, MAX_WIDGETS = 120;
    const SOURCE = /^[a-z][a-z0-9_]{1,15}$/, ICON = /^[a-z0-9-]{1,48}$/, HEX = /^#[0-9a-f]{6}$/, DO = ['default', 'toggle', 'more_info', 'navigate', 'none'];
    // Entity id grammar of internal/homedash/ids.go ParseEntityID: source (2-16) ':' native (1-200 bytes, printable, no space, no bidi control); the native part may hold ':'.
    function entityId(s) { if (typeof s !== 'string') return false; const i = s.indexOf(':'); if (i < 0 || !SOURCE.test(s.slice(0, i))) return false; const n = s.slice(i + 1); if (!n) return false; let bytes = 0;
        for (const ch of n) { const c = ch.codePointAt(0); bytes += c < 0x80 ? 1 : c < 0x800 ? 2 : c < 0x10000 ? 3 : 4; if (bytes > 200) return false; if (c <= 0x20 || (c >= 0x7f && c <= 0xa0) || c === 0xad || c === 0x2028 || c === 0x2029 || (c >= 0x200b && c <= 0x200f) || (c >= 0x202a && c <= 0x202e) || (c >= 0x2066 && c <= 0x2069) || c === 0xfeff || /\s/.test(ch)) return false; } return true; }
    const isObj = v => v !== null && typeof v === 'object' && !Array.isArray(v);
    // Mirrors the server's text rules (sanitize.go ContainsControl: unicode.IsControl = C0, DEL, C1, plus bidi controls; ContainsURL): control and bidi
    // characters are dropped, a text that still carries a URL or script scheme becomes '' (the server answers url_not_allowed for it).
    const URLISH = ['://', 'javascript:', 'data:', 'vbscript:'];
    function text(v, max) { if (typeof v !== 'string') return ''; let out = '';
        for (const ch of v) { const c = ch.codePointAt(0); if (c < 0x20 || (c >= 0x7f && c <= 0x9f) || c === 0x61c || c === 0x200e || c === 0x200f || (c >= 0x202a && c <= 0x202e) || (c >= 0x2066 && c <= 0x2069)) continue; out += ch; }
        out = Array.from(out).slice(0, max).join(''); const low = out.toLowerCase();
        return URLISH.some(m => low.includes(m)) || low.trim().startsWith('//') ? '' : out; }
    function encode(doc, pageId, ids) {
        const page = doc && Array.isArray(doc.pages) ? doc.pages.find(p => p.id === pageId) : null; if (!page) return '';
        const want = new Set(ids || []), order = E.grid.sortReading(page.layouts.lg.filter(i => want.has(i.id)));
        const widgets = order.map(i => { const w = page.widgets.find(x => x.id === i.id); if (!w) return null; const c = clone(w); delete c.id; return { widget: c, lgSize: { w: i.w, h: i.h } }; }).filter(Boolean).slice(0, MAX_WIDGETS);
        return widgets.length ? JSON.stringify({ v: 1, widgets }) : '';
    }
    function option(s, val, pages) {
        switch (s.type) {
            case 'bool': return typeof val === 'boolean' ? val : undefined;
            case 'int': case 'number': {
                if (val === null && s.default === null) return null; if (typeof val !== 'number' || !Number.isFinite(val)) return undefined;
                const n = s.type === 'int' ? Math.round(val) : val; return clamp(n, Number.isFinite(s.min) ? s.min : -1e12, Number.isFinite(s.max) ? s.max : 1e12); }
            case 'string': return typeof val === 'string' ? text(val, Math.min(s.max_len || 200, 200)) : undefined;
            case 'enum': return typeof val === 'string' && Array.isArray(s.enum) && s.enum.includes(val) ? val : undefined;
            case 'color': return val === '' ? '' : typeof val === 'string' && HEX.test(val.toLowerCase()) ? val.toLowerCase() : undefined;
            case 'page': return typeof val === 'string' && pages && pages.has(val) ? val : undefined;
            default: return undefined;
        }
    }
    function sanitizeWidget(raw, catalog, o) {
        const opts = o || {}; if (!isObj(raw) || typeof raw.type !== 'string' || !catalog || !catalog.has(raw.type)) return null;
        const type = raw.type, spec = catalog.widget(type), v = catalog.variant(type, typeof raw.variant === 'string' ? raw.variant : ''); if (!v) return null;
        const out = { type, variant: v.id, title: text(raw.title, 80), bindings: {}, options: {}, actions: {}, confirm: raw.confirm === true };
        if (typeof raw.family === 'string' && catalog.families().includes(raw.family)) out.family = raw.family;
        if (typeof raw.icon === 'string' && ICON.test(raw.icon)) out.icon = raw.icon;
        if (isObj(raw.bindings)) for (const b of spec.bindings || []) { const id = b && raw.bindings[b.name]; if (entityId(id)) out.bindings[b.name] = id; }
        if (isObj(raw.options)) for (const s of catalog.optionSpecs(type)) { if (!Object.prototype.hasOwnProperty.call(raw.options, s.key)) continue; const val = option(s, raw.options[s.key], opts.pages); if (val !== undefined) out.options[s.key] = val; }
        if (isObj(raw.actions)) for (const k of ['tap', 'hold']) {
            const a = raw.actions[k]; if (!isObj(a) || !DO.includes(a.do)) continue;
            if (a.do === 'navigate') { if (typeof a.page === 'string' && opts.pages && opts.pages.has(a.page)) out.actions[k] = { do: 'navigate', page: a.page }; continue; }
            if (a.do === 'toggle' && !catalog.canToggle(type)) continue; out.actions[k] = { do: a.do };
        }
        return out;
    }
    function decode(input, o) {
        const opts = o || {}, catalog = opts.catalog; if (typeof input !== 'string' || !input || input.length > MAX_BYTES || !catalog) return [];
        let data; try { data = JSON.parse(input); } catch (err) { return []; }
        if (!isObj(data) || data.v !== 1 || !Array.isArray(data.widgets)) return [];
        const out = [], int = (v, d) => (Number.isFinite(Number(v)) ? Math.round(Number(v)) : d);
        for (const e of data.widgets.slice(0, MAX_WIDGETS)) {
            if (!isObj(e)) continue; const widget = sanitizeWidget(e.widget, catalog, opts); if (!widget) continue;
            const c = catalog.constraints(widget), d = catalog.defaultSize(widget), s = isObj(e.lgSize) ? e.lgSize : {};
            out.push({ widget, size: { w: clamp(int(s.w, d.w), c.minW, Math.min(c.maxW, 12)), h: clamp(int(s.h, d.h), c.minH, c.maxH) } });
        }
        return out;
    }
    function channel(o) {
        const opts = o || {}, store = opts.storage || null, win = opts.win || null, listeners = new Set();
        const read = () => { try { return store ? store.getItem(KEY) : null; } catch (err) { return null; } };
        const tell = v => { for (const fn of [...listeners]) { try { fn(v); } catch (err) { console.error(err); } } };
        const onStorage = ev => { if (ev && (ev.key === KEY || ev.key === null)) tell(read()); };
        if (win && typeof win.addEventListener === 'function') win.addEventListener('storage', onStorage);
        return Object.freeze({
            read, has: () => !!read(),
            write(value) { if (!store || typeof value !== 'string' || !value || value.length > MAX_BYTES) return false; try { store.setItem(KEY, value); } catch (err) { return false; } tell(value); return true; },
            onChange(fn) { listeners.add(fn); return () => listeners.delete(fn); },
            dispose() { listeners.clear(); if (win && typeof win.removeEventListener === 'function') win.removeEventListener('storage', onStorage); }
        });
    }
    E.clipboard = Object.freeze({ KEY, MAX_BYTES, MAX_WIDGETS, encode, decode, sanitizeWidget, channel });
})();
```
- [ ] `scripts/homedash-tests/clipboard.test.mjs` (full; 8 tests: round trip, hostile text, sanitise, entity grammar, options and actions, string options, size clamp, channel):
<!-- embed: dryrun-m4/scripts/homedash-tests/clipboard.test.mjs -->
```js
import assert from 'node:assert/strict'; import { loadEngine, APP_ORDER, read, eq, fc } from './harness.mjs';
const FILES = APP_ORDER.slice(0, APP_ORDER.indexOf('hd-clipboard.js') + 1), CATALOG = JSON.parse(read('internal/homedash/testdata/widget_catalog.json'));
const boot = () => { const { E, window } = loadEngine(FILES), cat = E.catalog.create(CATALOG); return { E, window, cat }; };
const docOf = (E, cat) => E.doc.normalise({ name: 'T', pages: [{ id: 'main', name: 'Main', widgets: [
  { id: 'a', type: 'sensor', variant: 'numeral', title: 'Temp', bindings: { primary: 'ha:sensor.temp' }, options: { decimals: 2 }, actions: { tap: { do: 'more_info' } } },
  { id: 'b', type: 'switch', variant: 'tile', bindings: { primary: 'ha:switch.x' }, actions: { tap: { do: 'toggle' } }, family: 'glass' }],
  layouts: { lg: [{ id: 'a', x: 4, y: 0, w: 3, h: 2 }, { id: 'b', x: 0, y: 0, w: 2, h: 1 }] } }] }, { sizeOf: cat.defaultSize, constraints: cat.constraints });
const wrap = (widget, lgSize) => JSON.stringify({ v: 1, widgets: [{ widget, lgSize: lgSize || { w: 2, h: 1 } }] });
const one = (E, cat, widget, pages, size) => E.clipboard.decode(wrap(widget, size), { catalog: cat, pages: pages || new Set(['main']) })[0];
export const tests = [
  ['clipboard: encode keeps lg reading order, strips ids and carries the lg size; decode round-trips', () => { const { E, cat } = boot(), D = docOf(E, cat);
    const text = E.clipboard.encode(D, 'main', ['a', 'b']), data = JSON.parse(text);
    eq(data.widgets.map(w => w.widget.type), ['switch', 'sensor']); assert.ok(data.widgets.every(w => !('id' in w.widget))); eq(data.widgets.map(w => w.lgSize), [{ w: 2, h: 1 }, { w: 3, h: 2 }]); assert.equal(data.v, 1);
    const back = E.clipboard.decode(text, { catalog: cat, pages: new Set(['main']) }); eq(back.map(e => e.widget.type), ['switch', 'sensor']); eq(back[1].size, { w: 3, h: 2 }); eq(back[1].widget.options, { decimals: 2 });
    assert.equal(back[0].widget.family, 'glass'); eq(back[0].widget.actions, { tap: { do: 'toggle' } }); assert.equal(E.clipboard.encode(D, 'nope', ['a']), ''); assert.equal(E.clipboard.encode(D, 'main', ['zz']), ''); }],
  ['clipboard: hostile text (size, version, shape, unknown type) yields nothing', () => { const { E, cat } = boot(), d = t => E.clipboard.decode(t, { catalog: cat, pages: new Set(['main']) });
    eq(d('x'.repeat(300 * 1024)), []); eq(d(JSON.stringify({ v: 1, widgets: [{ widget: { type: 'sensor', title: 'x'.repeat(262144) } }] })), []); eq(d('{"v":2,"widgets":[]}'), []); eq(d('[]'), []); eq(d('nope'), []); eq(d(''), []); eq(d(null), []);
    eq(d('{"v":1,"widgets":"x"}'), []); eq(d(wrap({ type: 'nope', variant: 'x' })), []); eq(d(wrap({ type: '__proto__' })), []); eq(d(wrap('str')), []); eq(d(JSON.stringify({ v: 1, widgets: [null, 5, 'x', { widget: null }] })), []);
    assert.equal(E.clipboard.decode(JSON.stringify({ v: 1, widgets: Array.from({ length: 300 }, () => ({ widget: { type: 'sensor' } })) }), { catalog: cat }).length, 120); eq(E.clipboard.decode(wrap({ type: 'sensor' })), []); }],
  ['clipboard: sanitize keeps known fields only, strips control characters, caps title at 80 runes', () => { const { E, cat } = boot();
    const long = 'a' + String.fromCharCode(7) + 'b'.repeat(200), w = one(E, cat, { type: 'sensor', variant: 'nope', title: long, evil: { x: 1 }, id: 'zz', icon: 'Bad Icon', bindings: { primary: 'ha:sensor.x', other: 'ha:sensor.y' }, family: 'neon', confirm: 'yes' }).widget;
    assert.equal(w.title.length, 80); assert.ok(!w.title.includes(String.fromCharCode(7))); assert.equal(w.variant, cat.variant('sensor', '').id); assert.ok(!('evil' in w) && !('id' in w) && !('icon' in w) && !('family' in w)); eq(w.bindings, { primary: 'ha:sensor.x' }); assert.equal(w.confirm, false);
    const emoji = fc(0x1f600).repeat(100), t2 = one(E, cat, { type: 'sensor', title: emoji }).widget.title; assert.equal(Array.from(t2).length, 80);
    assert.equal(one(E, cat, { type: 'sensor', title: 'a' + fc(0x85) + 'b' + fc(0x7f) + 'c' + fc(0x202e) + 'd' }).widget.title, 'abcd', 'C0, DEL, C1 and bidi controls are dropped like the server does');
    for (const url of ['see http://x.y', 'javascript:alert(1)', '//evil', 'DATA:text/html,x']) assert.equal(one(E, cat, { type: 'sensor', title: url }).widget.title, '', 'url_not_allowed: ' + url);
    assert.equal(one(E, cat, { type: 'sensor', icon: 'bulb-2' }).widget.icon, 'bulb-2'); assert.equal(one(E, cat, { type: 'sensor', confirm: true }).widget.confirm, true); }],
  ['clipboard: entity ids follow the server grammar (source 2-16 chars, native printable without space)', () => { const { E, cat } = boot(), b = id => one(E, cat, { type: 'sensor', bindings: { primary: id } }).widget.bindings.primary;
    assert.equal(b('ha:sensor.x'), 'ha:sensor.x'); assert.equal(b('system:cpu'), 'system:cpu');
    for (const bad of ['light.x', 'a b:c', 'HA:x', 'ha:', 'ha: x', ':x', 'ha:' + 'x'.repeat(201), 5, null, 'ha:' + fc(7)]) assert.equal(b(bad), undefined, String(bad));
    assert.equal(b('x:abc'), undefined, 'a one-character source is refused by the server (sourceIDRe needs 2-16)'); assert.equal(b('abcdefghijklmnopq:x'), undefined, 'a 17-character source is refused by the server');
    assert.equal(b('bt:aa:bb:cc'), 'bt:aa:bb:cc', 'the native part may hold colons (strings.Cut on the first one)'); assert.equal(b('javascript:x'), 'javascript:x', 'syntactically a valid id; an unknown source is only entity_unknown'); }],
  ['clipboard: options are typed, clamped and limited to the catalog; actions need real pages and toggle support', () => { const { E, cat } = boot(), P = new Set(['main', 'other']);
    const s = one(E, cat, { type: 'sensor', options: { decimals: 9, hours: '5', bogus: 1, accent: '#ABCDEF' } }, P).widget; eq(s.options, { decimals: 3, accent: '#abcdef' });
    assert.equal(one(E, cat, { type: 'sensor', options: { decimals: 1.6 } }, P).widget.options.decimals, 2); eq(one(E, cat, { type: 'sensor', options: { accent: 'red' } }, P).widget.options, {});
    eq(one(E, cat, { type: 'gauge', options: { warn: null, min: 5, max: Infinity } }, P).widget.options, { warn: null, min: 5 });
    const a = one(E, cat, { type: 'switch', actions: { tap: { do: 'navigate', page: 'other' }, hold: { do: 'toggle', extra: 1 } } }, P).widget.actions; eq(a, { tap: { do: 'navigate', page: 'other' }, hold: { do: 'toggle' } });
    eq(one(E, cat, { type: 'switch', actions: { tap: { do: 'navigate', page: 'gone' }, hold: { do: 'launch' } } }, P).widget.actions, {}); eq(one(E, cat, { type: 'sensor', actions: { tap: { do: 'toggle' }, hold: { do: 'none' } } }, P).widget.actions, { hold: { do: 'none' } });
    eq(E.clipboard.decode(wrap({ type: 'switch', actions: { tap: { do: 'navigate', page: 'other' } } }), { catalog: cat })[0].widget.actions, {}, 'without a pages set no navigate survives'); }],
  ['clipboard: string options use the same text rule as titles (control, bidi, URL) and their own length cap', () => { const { E, cat } = boot();
    const fake = Object.assign({}, cat, { optionSpecs: () => [{ key: 'label', type: 'string', max_len: 12 }] }), s = v => E.clipboard.decode(wrap({ type: 'sensor', options: { label: v } }), { catalog: fake, pages: new Set() })[0].widget.options.label;
    assert.equal(s('ok' + fc(0x85) + fc(0x202e) + 'go'), 'okgo'); assert.equal(s('a'.repeat(30)), 'a'.repeat(12)); for (const bad of ['http://x', 'javascript:1', '//evil']) assert.equal(s(bad), '', bad); assert.equal(E.clipboard.decode(wrap({ type: 'sensor', options: { label: 5 } }), { catalog: fake })[0].widget.options.label, undefined); }],
  ['clipboard: lgSize is clamped to the variant limits and missing size falls back to the default', () => { const { E, cat } = boot(), c = cat.constraints({ type: 'sensor', variant: 'sparkline' });
    const e = one(E, cat, { type: 'sensor', variant: 'sparkline' }, null, { w: 99, h: -3 }); eq(e.size, { w: c.maxW, h: c.minH }); assert.equal(e.widget.variant, 'sparkline');
    eq(one(E, cat, { type: 'sensor', variant: 'sparkline' }, null, { w: 'x' }).size, { w: cat.defaultSize({ type: 'sensor', variant: 'sparkline' }).w, h: cat.defaultSize({ type: 'sensor', variant: 'sparkline' }).h });
    eq(E.clipboard.decode(JSON.stringify({ v: 1, widgets: [{ widget: { type: 'sensor', variant: 'sparkline' } }] }), { catalog: cat })[0].size, cat.defaultSize({ type: 'sensor', variant: 'sparkline' })); }],
  ['clipboard: the channel tells its own realm at once and other tabs through storage events; a throwing store fails soft', () => { const { E, window } = boot(), mem = new Map(), seen = [], listeners = {};
    const store = { getItem: k => (mem.has(k) ? mem.get(k) : null), setItem: (k, v) => { mem.set(k, v); } }, win = { addEventListener: (t, fn) => { listeners[t] = fn; }, removeEventListener: t => { delete listeners[t]; } };
    const ch = E.clipboard.channel({ storage: store, win }), off = ch.onChange(v => seen.push(v)); assert.equal(ch.has(), false); assert.equal(ch.write('{"v":1}'), true); eq(seen, ['{"v":1}']); assert.equal(ch.has(), true); assert.equal(ch.read(), '{"v":1}');
    mem.set(E.clipboard.KEY, 'other'); listeners.storage({ key: E.clipboard.KEY }); listeners.storage({ key: 'else' }); eq(seen, ['{"v":1}', 'other']); listeners.storage({ key: null }); assert.equal(seen.length, 3);
    assert.equal(ch.write(''), false); assert.equal(ch.write('x'.repeat(E.clipboard.MAX_BYTES + 1)), false); assert.equal(ch.write(5), false); off(); ch.write('{"v":1}'); assert.equal(seen.length, 3);
    const bad = E.clipboard.channel({ storage: { getItem() { throw new Error('denied'); }, setItem() { throw new Error('quota'); } } }); assert.equal(bad.write('x'), false); assert.equal(bad.read(), null); assert.equal(E.clipboard.channel({}).write('x'), false);
    ch.onChange(() => { throw new Error('listener'); }); ch.dispose(); assert.equal(listeners.storage, undefined); void window; }]
];
```
- [ ] Wiring step W. PASS. Commit — `feat(homedash): untrusted clipboard codec and storage channel`.

### Task A8: hd-commands.js (new)
**Files:** create `hd-commands.js`, `commands.test.mjs`; wiring step W (after `hd-clipboard.js`). **Existing symbols changed:** none.
- [ ] Full file (`add` ends with the compaction step of F14):
<!-- embed: dryrun-m4/ui/js/homedash/hd-commands.js -->
```js
// HomeDash designer commands (pure; frontend 4.3, 5.5-5.10; M4 contract C5). Builders take the working document (E.doc.normalise output) and an
// argument object and return a command {label, vars, ops, meta, select?, page?, materialised?, announce?}, a refusal {refused}, a size limit {limit}
// or null when nothing changes. Paths are id-keyed; meta = {kind, ids, at, coalesce?, window?}. An md/sm change while that layout is automatic
// materialises it (one op null -> array). Adding places into lg and every non-null override; deleting removes from all; lg changes never touch
// overrides. keymap(ev, ctx) is the key map (frontend 5.6, C14); spatial(items, id, dir) the Alt+Arrow target; readingOrder(items) the Tab order.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.doc || !E.grid || !E.layout) throw new Error('hd-commands.js: load hd-doc.js first');
    const D = E.doc, G = E.grid, L = E.layout, { clone, randomId, clamp } = E.core, COLS = L.COLS;
    const NUDGE_MS = 800, FIELD_MS = 1000, LIMITS = Object.freeze({ pages: 12, pageWidgets: 120, widgets: 400 });
    const DEFAULTS = Object.freeze({ variant: '', title: '', bindings: {}, options: {}, actions: {}, confirm: false });
    const sortedKey = ids => [...new Set(ids)].sort().join(',');
    const pageOf = (doc, id) => (doc && Array.isArray(doc.pages) ? doc.pages.find(p => p.id === id) : null) || null;
    const indexOf = (doc, id) => doc.pages.findIndex(p => p.id === id);
    const total = doc => doc.pages.reduce((n, p) => n + p.widgets.length, 0);
    const compactOf = page => page.compact !== false;
    const constraint = (C, id) => (typeof C === 'function' ? C(id) : C && C[id]) || {};
    function make(label, ops, meta, extra) {
        const list = ops.filter(Boolean); if (!list.length) return null;
        const m = Object.assign({}, meta); if (!Number.isFinite(m.at)) m.at = Date.now();
        return Object.assign({ label, vars: null, ops: list, meta: m }, extra || {});
    }
    function usedIds(doc) { const s = new Set(); for (const p of doc.pages) { s.add(p.id); for (const w of p.widgets) s.add(w.id); } return s; }
    function fresh(used, prefix, n) { for (;;) { const id = randomId(prefix, n); if (!used.has(id)) { used.add(id); return id; } } }
    function layoutOp(doc, page, bp, next, C) { return D.keyOf(L.effective(page.layouts, bp, C)) === D.keyOf(next) ? null : D.op(doc, ['pages', page.id, 'layouts', bp], next); }
    function layout(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page) return null;
        const bp = a.bp || 'lg', op = layoutOp(doc, page, bp, a.next, a.C);
        return make(a.label || 'homedash.cmd.move', [op], { kind: a.kind || 'layout', ids: (a.ids || []).slice(), at: a.now, coalesce: a.coalesce, window: a.window },
            { materialised: op && bp !== 'lg' && !Array.isArray(page.layouts[bp]) ? bp : null, announce: a.announce || null });
    }
    function nudge(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page || (!a.dx && !a.dy)) return null;
        const bp = a.bp || 'lg', cols = COLS[bp], cur = L.effective(page.layouts, bp, a.C), ids = (a.ids || []).filter(id => cur.some(i => i.id === id)); if (!ids.length) return null;
        const B = G.bbox(cur.filter(i => ids.includes(i.id))), steps = a.dy ? 64 : 1;
        for (let k = 1; k <= steps; k++) {
            const to = { x: B.x + a.dx * k, y: B.y + a.dy * k }; if (to.x < 0 || to.x + B.w > cols || to.y < 0) return { limit: 'edge' };
            const next = G.moveItems(cur, ids, to, cols, { compact: compactOf(page) }), nb = G.bbox(next.filter(i => ids.includes(i.id)));
            if ((a.dx && nb.x !== B.x) || (a.dy > 0 && nb.y > B.y) || (a.dy < 0 && nb.y < B.y))
                return layout(doc, { pageId: page.id, bp, next, C: a.C, ids, now: a.now, kind: 'nudge', label: 'homedash.cmd.move', coalesce: 'nudge|' + bp + '|' + sortedKey(ids), window: NUDGE_MS,
                    announce: ids.length === 1 ? { key: 'moved', col: nb.x + 1, row: nb.y + 1 } : { key: 'moved_many', count: ids.length } });
        }
        return { limit: 'blocked' };
    }
    function resizeStep(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page || (!a.dw && !a.dh)) return null;
        const bp = a.bp || 'lg', cols = COLS[bp], cur = L.effective(page.layouts, bp, a.C), r = cur.find(i => i.id === a.id); if (!r) return null;
        const next = G.resizeItem(cur, a.id, a.dw ? 'e' : 's', { dx: a.dw || 0, dy: a.dh || 0 }, cols, constraint(a.C, a.id), { compact: compactOf(page) }), n = next.find(i => i.id === a.id);
        if (n.w === r.w && n.h === r.h) return { limit: (a.dw || a.dh) < 0 ? 'min' : a.dw > 0 && r.x + r.w >= cols ? 'edge' : 'max' };
        return layout(doc, { pageId: page.id, bp, next, C: a.C, ids: [a.id], now: a.now, kind: 'resize', label: 'homedash.cmd.resize', coalesce: 'resize|' + bp + '|' + a.id, window: NUDGE_MS, announce: { key: 'resized', w: n.w, h: n.h } });
    }
    // Inserts item {id, w, h} (lg size): lg at first fit from startY, or moved to `at` for an lg drop; an md/sm drop materialises that override and moves
    // the item to `at` there; other non-null overrides get the derived size at first fit (frontend 4.3).
    function inserted(layouts, item, C, o) {
        const compact = o.compact !== false, out = { lg: layouts.lg, md: layouts.md, sm: layouts.sm }, lgAt = o.bp === 'lg' && o.at;
        const p = G.place(out.lg, item.w, item.h, COLS.lg, lgAt ? 0 : o.startY || 0);
        out.lg = out.lg.concat([{ id: item.id, x: p.x, y: p.y, w: item.w, h: item.h }]);
        if (lgAt) out.lg = G.moveItems(out.lg, [item.id], o.at, COLS.lg, { compact });
        for (const bp of ['md', 'sm']) {
            const dropHere = o.bp === bp && o.at; if (!dropHere && !Array.isArray(out[bp])) continue;
            const base = Array.isArray(out[bp]) ? out[bp] : L.derive(layouts.lg, COLS[bp], C), s = L.derivedSize(item, COLS[bp], constraint(C, item.id)), q = G.place(base, s.w, s.h, COLS[bp]);
            const added = base.concat([{ id: item.id, x: q.x, y: q.y, w: s.w, h: s.h }]);
            out[bp] = dropHere ? G.moveItems(added, [item.id], o.at, COLS[bp], { compact }) : added;
        }
        return out;
    }
    function add(doc, a) {
        const page = pageOf(doc, a.pageId), list = (a.entries || []).filter(e => e && e.widget && typeof e.widget.type === 'string');
        if (!page || !list.length) return null;
        if (page.widgets.length + list.length > LIMITS.pageWidgets || total(doc) + list.length > LIMITS.widgets) return { refused: 'limit' };
        const used = usedIds(doc), extra = new Map(), Cx = id => extra.get(id) || constraint(a.C, id), ops = [], ids = [], bp = a.bp || 'lg';
        let work = doc, layouts = page.layouts, startY = a.startY || 0;
        list.forEach((e, i) => {
            const w = Object.assign(clone(DEFAULTS), clone(e.widget)); w.id = fresh(used, 'w', 8); ids.push(w.id);
            const c = typeof a.limitsOf === 'function' ? a.limitsOf(w) || {} : {}; extra.set(w.id, c);
            const size = { w: clamp(Math.round(e.size && e.size.w) || 1, c.minW || 1, Math.min(c.maxW || 12, 12)), h: clamp(Math.round(e.size && e.size.h) || 1, c.minH || 1, c.maxH || 24) };
            const op = D.op(work, ['pages', page.id, 'widgets', w.id], w); ops.push(op); work = D.apply(work, [op], 'after');
            layouts = inserted(layouts, { id: w.id, w: size.w, h: size.h }, Cx, { bp, at: i === 0 ? a.at : null, startY, compact: compactOf(page) });
            if (i === 0) startY = layouts.lg.find(x => x.id === w.id).y;
        });
        // A compact page ends compacted, as E.doc.normalise (E.grid.repair) leaves it on the next load: the local document equals the reloaded one.
        if (compactOf(page)) layouts = { lg: G.compact(layouts.lg), md: Array.isArray(layouts.md) ? G.compact(layouts.md) : layouts.md, sm: Array.isArray(layouts.sm) ? G.compact(layouts.sm) : layouts.sm };
        for (const b of ['lg', 'md', 'sm']) if (D.keyOf(page.layouts[b]) !== D.keyOf(layouts[b])) ops.push({ path: ['pages', page.id, 'layouts', b], before: clone(page.layouts[b]), after: clone(layouts[b]) });
        const first = L.effective(layouts, bp, Cx).find(x => x.id === ids[0]) || { x: 0, y: 0 };
        return make(a.label || 'homedash.cmd.add', ops, { kind: a.kind || 'add', ids, at: a.now }, { select: ids, materialised: bp !== 'lg' && a.at && !Array.isArray(page.layouts[bp]) ? bp : null,
            announce: { key: ids.length === 1 ? 'added' : 'added_many', count: ids.length, id: ids[0], col: first.x + 1, row: first.y + 1 } });
    }
    function remove(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page) return null; const ids = (a.ids || []).filter(id => page.widgets.some(w => w.id === id)); if (!ids.length) return null;
        return make(a.label || 'homedash.cmd.delete', D.removeWidgets(doc, page.id, ids).ops, { kind: a.kind || 'delete', ids, at: a.now }, { select: [], announce: { key: ids.length === 1 ? 'deleted' : 'deleted_many', count: ids.length, id: ids[0] } });
    }
    function duplicate(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page) return null; const order = G.sortReading(page.layouts.lg.filter(i => (a.ids || []).includes(i.id))); if (!order.length) return null;
        const entries = order.map(i => { const w = clone(page.widgets.find(x => x.id === i.id)); delete w.id; return { widget: w, size: { w: i.w, h: i.h } }; });
        return add(doc, { pageId: page.id, bp: 'lg', entries, startY: Math.max(...order.map(i => i.y + i.h)), C: a.C, limitsOf: a.limitsOf, now: a.now, label: 'homedash.cmd.duplicate', kind: 'duplicate' });
    }
    const paste = (doc, a) => add(doc, Object.assign({}, a, { at: null, label: 'homedash.cmd.paste', kind: 'paste' }));
    function setWidget(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page || !page.widgets.some(w => w.id === a.id)) return null;
        const path = ['pages', page.id, 'widgets', a.id].concat(a.path); if (D.keyOf(D.getAt(doc, path)) === D.keyOf(a.value)) return null;
        return make(a.label || 'homedash.cmd.option', [D.op(doc, path, a.value)], { kind: a.kind || 'field', ids: [a.id], at: a.now, coalesce: a.text ? 'field|' + a.id + '|' + a.path.join('.') : undefined, window: a.text ? FIELD_MS : undefined }, { vars: a.vars || null });
    }
    const bind = (doc, a) => setWidget(doc, { pageId: a.pageId, id: a.id, path: ['bindings', a.binding || 'primary'], value: a.entity || undefined, label: a.entity ? 'homedash.cmd.bind' : 'homedash.cmd.unbind', kind: 'bind', now: a.now });
    function fitItem(arr, id, c, cols, compact) {
        const r = arr.find(i => i.id === id); if (!r) return arr;
        const w = clamp(r.w, c.minW || 1, Math.min(c.maxW || cols, cols)), h = clamp(r.h, c.minH || 1, c.maxH || Infinity); if (w === r.w && h === r.h) return arr;
        return G.resizeItem(arr, id, 'se', { dx: w - r.w, dy: h - r.h }, cols, c, { compact });
    }
    function variant(doc, a) {
        const page = pageOf(doc, a.pageId), w = page && page.widgets.find(x => x.id === a.id); if (!w || w.variant === a.variant) return null;
        const ops = [D.op(doc, ['pages', page.id, 'widgets', a.id, 'variant'], a.variant)];
        for (const bp of ['lg', 'md', 'sm']) { const arr = page.layouts[bp]; if (!Array.isArray(arr)) continue; const next = fitItem(arr, a.id, a.limits || {}, COLS[bp], compactOf(page)); if (next !== arr) ops.push(D.op(doc, ['pages', page.id, 'layouts', bp], next)); }
        return make('homedash.cmd.variant', ops, { kind: 'variant', ids: [a.id], at: a.now }, { select: [a.id] });
    }
    function applyFamily(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page) return null;
        const ops = page.widgets.filter(w => (a.ids || []).includes(w.id) && (w.family || '') !== (a.family || '')).map(w => D.op(doc, ['pages', page.id, 'widgets', w.id, 'family'], a.family || undefined));
        return make('homedash.cmd.family', ops, { kind: 'family', ids: (a.ids || []).slice(), at: a.now });
    }
    function align(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page) return null; const bp = a.bp || 'lg', cols = COLS[bp], cur = L.effective(page.layouts, bp, a.C), p = cur.find(i => i.id === a.primaryId); if (!p) return null;
        const moved = (a.ids || []).filter(id => id !== p.id && cur.some(i => i.id === id));
        let next = cur.map(i => (moved.includes(i.id) ? Object.assign({}, i, a.edge === 'top' ? { y: p.y } : { x: clamp(p.x, 0, cols - i.w) }) : i));
        next = G.pushDown(next, moved); if (compactOf(page)) next = G.compact(next);
        return layout(doc, { pageId: page.id, bp, next, C: a.C, ids: moved, now: a.now, kind: 'align', label: 'homedash.cmd.align' });
    }
    function resetLayout(doc, a) { const page = pageOf(doc, a.pageId); if (!page || a.bp === 'lg' || !Array.isArray(page.layouts[a.bp])) return null;
        return make('homedash.cmd.reset_layout', [D.op(doc, ['pages', page.id, 'layouts', a.bp], null)], { kind: 'reset', ids: [], at: a.now }, { announce: { key: 'now_auto_' + a.bp } }); }
    function addPage(doc, a) {
        if (doc.pages.length >= LIMITS.pages) return { refused: 'limit' };
        const id = fresh(usedIds(doc), 'p', 6), page = { id, name: Array.from(String(a.name || '')).slice(0, 40).join('') || id, icon: a.icon || 'dashboard', compact: true, widgets: [], layouts: { lg: [], md: null, sm: null } };
        return make('homedash.cmd.page_add', [D.op(doc, ['pages', id], page)], { kind: 'page', ids: [], at: a.now }, { page: id });
    }
    function renamePage(doc, a) { const page = pageOf(doc, a.pageId), name = Array.from(String(a.name || '').trim()).slice(0, 40).join(''); if (!page || !name || name === page.name) return null;
        return make('homedash.cmd.page_rename', [D.op(doc, ['pages', page.id, 'name'], name)], { kind: 'page', ids: [], at: a.now, coalesce: 'page|' + page.id + '|name', window: FIELD_MS }); }
    function setPageIcon(doc, a) { const page = pageOf(doc, a.pageId); if (!page || page.icon === a.icon) return null; return make('homedash.cmd.page_icon', [D.op(doc, ['pages', page.id, 'icon'], a.icon || undefined)], { kind: 'page', ids: [], at: a.now }); }
    function duplicatePage(doc, a) {
        const src = pageOf(doc, a.pageId); if (!src) return null;
        if (doc.pages.length >= LIMITS.pages || total(doc) + src.widgets.length > LIMITS.widgets) return { refused: 'limit' };
        const used = usedIds(doc), id = fresh(used, 'p', 6), map = new Map(src.widgets.map(w => [w.id, fresh(used, 'w', 8)]));
        const remap = arr => (Array.isArray(arr) ? arr.map(i => Object.assign({}, i, { id: map.get(i.id) })) : null);
        const page = Object.assign(clone(src), { id, name: Array.from(String(a.name || src.name)).slice(0, 40).join(''), widgets: src.widgets.map(w => Object.assign(clone(w), { id: map.get(w.id) })), layouts: { lg: remap(src.layouts.lg), md: remap(src.layouts.md), sm: remap(src.layouts.sm) } });
        return make('homedash.cmd.page_duplicate', [{ path: ['pages', id], before: undefined, after: page, index: indexOf(doc, src.id) + 1 }], { kind: 'page', ids: [], at: a.now }, { page: id });
    }
    function movePage(doc, a) {
        const i = indexOf(doc, a.pageId); if (i < 0) return null; const j = clamp(Math.round(a.to), 0, doc.pages.length - 1); if (i === j) return null; const page = clone(doc.pages[i]);
        return make('homedash.cmd.page_move', [{ path: ['pages', page.id], before: page, after: undefined, index: i }, { path: ['pages', page.id], before: undefined, after: page, index: j }], { kind: 'page', ids: [], at: a.now }, { page: page.id });
    }
    function deletePage(doc, a) {
        const i = indexOf(doc, a.pageId); if (i < 0) return null; if (doc.pages.length <= 1) return { refused: 'last_page' };
        const page = doc.pages[i], rest = doc.pages.filter(p => p.id !== page.id), first = rest[0].id, ops = [{ path: ['pages', page.id], before: clone(page), after: undefined, index: i }];
        if (doc.kiosk && doc.kiosk.start_page === page.id) ops.push(D.op(doc, ['kiosk', 'start_page'], first));
        for (const p of rest) for (const w of p.widgets) {
            for (const k of ['tap', 'hold']) { const act = w.actions && w.actions[k]; if (act && act.do === 'navigate' && act.page === page.id) ops.push(D.op(doc, ['pages', p.id, 'widgets', w.id, 'actions', k], { do: 'default' })); }
            if (w.options && w.options.page === page.id) ops.push(D.op(doc, ['pages', p.id, 'widgets', w.id, 'options', 'page'], first));
        }
        return make('homedash.cmd.page_delete', ops, { kind: 'page', ids: page.widgets.map(w => w.id), at: a.now }, { page: rest[Math.max(0, i - 1)].id });
    }
    function setPageCompact(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page || compactOf(page) === !!a.compact) return null; const ops = [D.op(doc, ['pages', page.id, 'compact'], !!a.compact)];
        if (a.compact) for (const bp of ['lg', 'md', 'sm']) { const arr = page.layouts[bp]; if (!Array.isArray(arr)) continue; const next = G.compact(arr); if (D.keyOf(next) !== D.keyOf(arr)) ops.push(D.op(doc, ['pages', page.id, 'layouts', bp], next)); }
        return make('homedash.cmd.page_compact', ops, { kind: 'page', ids: [], at: a.now });
    }
    function setDoc(doc, a) { if (D.keyOf(D.getAt(doc, a.path)) === D.keyOf(a.value)) return null;
        return make(a.label || 'homedash.cmd.look', [D.op(doc, a.path, a.value)], { kind: 'doc', ids: [], at: a.now, coalesce: a.text ? 'doc|' + a.path.join('.') : undefined, window: a.text ? FIELD_MS : undefined }); }
    function entry(catalog, type, id, entityId) {
        const v = catalog && catalog.has(type) ? catalog.variant(type, id) : null; if (!v) return null;
        const widget = { type, variant: v.id, title: '', bindings: entityId ? { primary: entityId } : {}, options: {}, actions: {}, confirm: false };
        return { widget, size: catalog.defaultSize(widget) };
    }
    const ARROWS = Object.freeze({ ArrowLeft: [-1, 0, 'left'], ArrowRight: [1, 0, 'right'], ArrowUp: [0, -1, 'up'], ArrowDown: [0, 1, 'down'] });
    function keymap(ev, c) {
        const x = c || {}; if (!ev || x.typing || ev.isComposing) return null;
        const key = String(ev.key || ''), low = key.toLowerCase(), mod = !!(ev.ctrlKey || ev.metaKey), alt = !!ev.altKey, shift = !!ev.shiftKey, arrow = ARROWS[key];
        if (mod && alt && (key === 'PageUp' || key === 'PageDown')) return { action: key === 'PageUp' ? 'page_prev' : 'page_next' };
        if (mod && !alt) {
            if (low === 'z') return { action: shift ? 'redo' : 'undo' }; if (low === 'y' && !shift) return { action: 'redo' };
            if (!x.canvas || shift) return null;
            return { c: { action: 'copy' }, x: { action: 'cut' }, v: { action: 'paste' }, d: { action: 'duplicate' }, a: { action: 'select_all' }, ' ': { action: 'toggle_focused' } }[low] || null;
        }
        if (key === 'Escape' && !alt && !shift) return { action: 'escape' };
        if (!x.canvas) return null;
        if (arrow && alt && !shift) return { action: 'focus_dir', dir: arrow[2] };
        if (arrow && !alt) return shift ? { action: 'resize_step', dw: arrow[0], dh: arrow[1] } : { action: 'nudge', dx: arrow[0], dy: arrow[1] };
        if (alt) return null;
        if ((key === 'Delete' || key === 'Backspace') && !shift) return { action: 'delete' };
        if (key === ' ' && !shift) return { action: 'select_focused' };
        if ((key === 'Enter' && !shift) || key === 'F2') return { action: 'focus_inspector' };
        if (key === 'Tab') return { action: shift ? 'focus_prev' : 'focus_next' };
        return null;
    }
    function spatial(items, fromId, dir) {
        const a = items.find(i => i.id === fromId); if (!a) return null; const cx = a.x + a.w / 2, cy = a.y + a.h / 2; let best = null, score = Infinity;
        for (const b of items) { if (b.id === fromId) continue; const dx = b.x + b.w / 2 - cx, dy = b.y + b.h / 2 - cy;
            const along = dir === 'right' ? dx : dir === 'left' ? -dx : dir === 'down' ? dy : -dy, across = dir === 'left' || dir === 'right' ? Math.abs(dy) : Math.abs(dx);
            if (along <= 0) continue; const s = along + 2 * across; if (s < score || (s === score && b.id < best)) { score = s; best = b.id; } }
        return best;
    }
    const readingOrder = items => G.sortReading(items).map(i => i.id);
    E.commands = Object.freeze({ NUDGE_MS, FIELD_MS, LIMITS, layout, nudge, resizeStep, add, remove, duplicate, paste, setWidget, bind, variant, applyFamily, align, resetLayout,
        addPage, renamePage, setPageIcon, duplicatePage, movePage, deletePage, setPageCompact, setDoc, entry, keymap, spatial, readingOrder });
})();
```
- [ ] `scripts/homedash-tests/commands.test.mjs` (full; 15 tests: every C5 builder, history coalesce and undo, the equal-size and different-size `at` case, compact-page add/duplicate/paste equal `normalise`, keymap, spatial):
<!-- embed: dryrun-m4/scripts/homedash-tests/commands.test.mjs -->
```js
import assert from 'node:assert/strict'; import { loadEngine, APP_ORDER, read, eq } from './harness.mjs';
const FILES = APP_ORDER.slice(0, APP_ORDER.indexOf('hd-commands.js') + 1), CATALOG = JSON.parse(read('internal/homedash/testdata/widget_catalog.json'));
const boot = () => { const { E } = loadEngine(FILES), cat = E.catalog.create(CATALOG), X = E.commands, D = E.doc; return { E, cat, X, D }; };
const W = (id, type, variant, extra) => Object.assign({ id, type, variant, bindings: { primary: type === 'sensor' ? 'ha:sensor.x' : 'ha:switch.x' } }, extra);
const it = (id, x, y, w, h) => ({ id, x, y, w, h });
const mk = (E, cat, widgets, lg, extra, doc) => E.doc.normalise(Object.assign({ name: 'T', pages: [Object.assign({ id: 'main', name: 'Main', widgets, layouts: { lg } }, extra)] }, doc), { sizeOf: cat.defaultSize, constraints: cat.constraints });
const Cof = (cat, doc) => id => { for (const p of doc.pages) { const w = p.widgets.find(x => x.id === id); if (w) return cat.constraints(w); } return {}; };
const page = (doc, id) => doc.pages.find(p => p.id === (id || 'main')), lgOf = (doc, id) => Object.fromEntries(page(doc).layouts.lg.map(i => [i.id, [i.x, i.y, i.w, i.h]]));
const run = (E, doc, cmd) => E.doc.apply(doc, cmd.ops, 'after'), back = (E, doc, cmd) => E.doc.apply(doc, cmd.ops, 'before');
const key = (k, o) => Object.assign({ key: k, ctrlKey: false, metaKey: false, altKey: false, shiftKey: false }, o);
export const tests = [
  ['commands: nudge coalesces inside 800 ms, splits after it, and undo restores the start document', () => { const { E, cat, X } = boot(), H = E.history.create();
    const d0 = mk(E, cat, [W('a', 'sensor', 'numeral')], [it('a', 0, 0, 2, 1)]); let d = d0;
    for (const now of [0, 500, 1400]) { const c = X.nudge(d, { pageId: 'main', bp: 'lg', ids: ['a'], dx: 1, dy: 0, C: Cof(cat, d), now }); assert.equal(c.label, 'homedash.cmd.move'); assert.equal(c.meta.coalesce, 'nudge|lg|a'); assert.equal(c.meta.window, 800); H.push(c); d = run(E, d, c); }
    assert.equal(H.size(), 2); eq(lgOf(d).a, [3, 0, 2, 1]); const u1 = H.undo(d); eq(lgOf(u1.doc).a, [2, 0, 2, 1]); const u2 = H.undo(u1.doc); eq(u2.doc, d0);
    eq(X.nudge(d, { pageId: 'main', ids: ['a'], dx: 1, dy: 0, C: Cof(cat, d), now: 5 }).announce, { key: 'moved', col: 5, row: 1 }); }],
  ['commands: nudge limits (edge, blocked, no ids), multi-selection key and announce', () => { const { E, cat, X } = boot();
    const d = mk(E, cat, [W('a', 'sensor', 'numeral'), W('b', 'sensor', 'numeral')], [it('a', 0, 0, 2, 1), it('b', 10, 0, 2, 1)]), C = Cof(cat, d);
    eq(X.nudge(d, { pageId: 'main', ids: ['a'], dx: -1, dy: 0, C }), { limit: 'edge' }); eq(X.nudge(d, { pageId: 'main', ids: ['b'], dx: 1, dy: 0, C }), { limit: 'edge' }); eq(X.nudge(d, { pageId: 'main', ids: ['a'], dx: 0, dy: -1, C }), { limit: 'edge' });
    eq(X.nudge(d, { pageId: 'main', ids: ['a'], dx: 0, dy: 1, C }), { limit: 'blocked' }, 'gravity pulls a lone widget back up'); assert.equal(X.nudge(d, { pageId: 'main', ids: ['zz'], dx: 1, dy: 0, C }), null); assert.equal(X.nudge(d, { pageId: 'main', ids: ['a'], dx: 0, dy: 0, C }), null);
    const free = mk(E, cat, [W('a', 'sensor', 'numeral'), W('b', 'sensor', 'numeral')], [it('a', 0, 0, 2, 1), it('b', 4, 0, 2, 1)], { compact: false });
    const c = X.nudge(free, { pageId: 'main', ids: ['b', 'a'], dx: 1, dy: 0, C: Cof(cat, free), now: 1 }); assert.equal(c.meta.coalesce, 'nudge|lg|a,b'); eq(c.announce, { key: 'moved_many', count: 2 }); eq(lgOf(run(E, free, c)), { a: [1, 0, 2, 1], b: [5, 0, 2, 1] });
    const y = X.nudge(free, { pageId: 'main', ids: ['a'], dx: 0, dy: 1, C: Cof(cat, free), now: 1 }); eq(lgOf(run(E, free, y)).a, [0, 1, 2, 1], 'compact:false keeps a one-row step'); }],
  ['commands: nudge jumps over compacted neighbours (A above B goes below B)', () => { const { E, cat, X } = boot();
    const d = mk(E, cat, [W('a', 'sensor', 'numeral'), W('b', 'sensor', 'sparkline')], [it('a', 0, 0, 2, 1), it('b', 0, 1, 2, 2)]); const c = X.nudge(d, { pageId: 'main', ids: ['a'], dx: 0, dy: 1, C: Cof(cat, d), now: 1 });
    const after = lgOf(run(E, d, c)); assert.equal(after.a[1], 2); assert.ok(after.a[1] >= after.b[1] + after.b[3]); eq(after.b, [0, 0, 2, 2]); eq(c.announce, { key: 'moved', col: 1, row: 3 });
    const up = X.nudge(run(E, d, c), { pageId: 'main', ids: ['a'], dx: 0, dy: -1, C: Cof(cat, d), now: 2 }); eq(lgOf(run(E, run(E, d, c), up)).a, [0, 0, 2, 1]); }],
  ['commands: md nudge on an automatic layout materialises it in one op and undo makes it automatic again', () => { const { E, cat, X } = boot();
    const d = mk(E, cat, [W('a', 'sensor', 'numeral'), W('b', 'sensor', 'numeral')], [it('a', 0, 0, 6, 1), it('b', 6, 0, 6, 1)]); assert.equal(page(d).layouts.md, null);
    const c = X.nudge(d, { pageId: 'main', bp: 'md', ids: ['a'], dx: 1, dy: 0, C: Cof(cat, d), now: 1 }); assert.equal(c.materialised, 'md'); assert.equal(c.ops.length, 1); assert.equal(c.ops[0].before, null); assert.equal(c.meta.coalesce, 'nudge|md|a');
    const after = run(E, d, c); assert.ok(Array.isArray(page(after).layouts.md)); assert.equal(page(after).layouts.lg, page(d).layouts.lg); assert.equal(back(E, after, c).pages[0].layouts.md, null);
    const second = X.nudge(after, { pageId: 'main', bp: 'md', ids: ['a'], dx: -1, dy: 0, C: Cof(cat, after), now: 2 }); assert.equal(second.materialised, null); }],
  ['commands: resizeStep grows with a coalesce key and reports min, max and edge', () => { const { E, cat, X } = boot();
    const d = mk(E, cat, [W('a', 'light', 'slider'), W('b', 'light', 'slider')], [it('a', 0, 0, 2, 1), it('b', 0, 1, 6, 1)]), C = Cof(cat, d);
    eq(X.resizeStep(d, { pageId: 'main', id: 'a', dw: -1, dh: 0, C }), { limit: 'min' }); eq(X.resizeStep(d, { pageId: 'main', id: 'a', dw: 0, dh: -1, C }), { limit: 'min' }); eq(X.resizeStep(d, { pageId: 'main', id: 'b', dw: 1, dh: 0, C }), { limit: 'max' });
    const c = X.resizeStep(d, { pageId: 'main', id: 'a', dw: 1, dh: 0, C, now: 1 }); assert.equal(c.meta.coalesce, 'resize|lg|a'); assert.equal(c.meta.window, 800); eq(c.announce, { key: 'resized', w: 3, h: 1 }); eq(lgOf(run(E, d, c)).a, [0, 0, 3, 1]);
    const edge = mk(E, cat, [W('a', 'light', 'slider')], [it('a', 10, 0, 2, 1)]); eq(X.resizeStep(edge, { pageId: 'main', id: 'a', dw: 1, dh: 0, C: Cof(cat, edge) }), { limit: 'edge' }); assert.equal(X.resizeStep(d, { pageId: 'main', id: 'zz', dw: 1, dh: 0, C }), null); }],
  ['commands: add places in lg and every non-null override, selects the new ids and honours the limits', () => { const { E, cat, X } = boot();
    const base = mk(E, cat, [W('a', 'sensor', 'numeral')], [it('a', 0, 0, 2, 1)]), d = E.doc.apply(base, [E.doc.op(base, ['pages', 'main', 'layouts', 'md'], [it('a', 0, 0, 2, 1)])], 'after');
    const e1 = X.entry(cat, 'switch', 'tile', 'ha:switch.k'), e2 = X.entry(cat, 'sensor', 'numeral'); eq(e1.size, cat.defaultSize(e1.widget)); eq(e1.widget.bindings, { primary: 'ha:switch.k' });
    const c = X.add(d, { pageId: 'main', entries: [e1, e2], startY: 3, C: Cof(cat, d), limitsOf: cat.constraints, now: 1 }); assert.equal(c.select.length, 2); assert.equal(c.label, 'homedash.cmd.add'); eq(c.meta.ids, c.select);
    const after = run(E, d, c), p = page(after); assert.equal(p.widgets.length, 3); const first = p.layouts.lg.find(i => i.id === c.select[0]); assert.ok(first.y >= 0); assert.ok(c.select.every(id => p.layouts.md.some(i => i.id === id)), 'md override gains both'); assert.equal(p.layouts.sm, null);
    assert.equal(p.widgets.find(w => w.id === c.select[0]).variant, 'tile'); assert.equal(c.announce.key, 'added_many'); assert.equal(back(E, after, c).pages[0].widgets.length, 1); eq(back(E, after, c), d);
    const full = mk(E, cat, Array.from({ length: 120 }, (_, i) => W('w' + i, 'sensor', 'icon')), Array.from({ length: 120 }, (_, i) => it('w' + i, i % 12, Math.floor(i / 12), 1, 1))); eq(X.add(full, { pageId: 'main', entries: [e2], C: Cof(cat, full), limitsOf: cat.constraints }), { refused: 'limit' });
    assert.equal(X.add(d, { pageId: 'main', entries: [] }), null); assert.equal(X.add(d, { pageId: 'nope', entries: [e2] }), null);
    const norm = E.doc.normalise(after, { sizeOf: cat.defaultSize, constraints: cat.constraints }); eq(norm.pages[0].layouts, page(after).layouts, 'startY 3 on a compact page: the local layouts equal the normalised ones (no gap, no jump on reload)'); assert.ok(first.y < 3, 'the gap is closed');
    const free = mk(E, cat, [W('a', 'sensor', 'numeral')], [it('a', 0, 0, 2, 1)], { compact: false }), cf = X.add(free, { pageId: 'main', entries: [e2], startY: 3, C: Cof(cat, free), limitsOf: cat.constraints, now: 1 }); assert.equal(lgOf(run(E, free, cf))[cf.select[0]][1], 3, 'a free page keeps startY');
    const dupC = X.duplicate(d, { pageId: 'main', ids: ['a'], C: Cof(cat, d), limitsOf: cat.constraints, now: 1 }), pasC = X.paste(d, { pageId: 'main', entries: [e1], startY: 9, C: Cof(cat, d), limitsOf: cat.constraints, now: 1 });
    for (const cmd of [dupC, pasC]) { const x = run(E, d, cmd); eq(E.doc.normalise(x, { sizeOf: cat.defaultSize, constraints: cat.constraints }).pages[0].layouts, page(x).layouts, cmd.label); }
    const c0 = X.add(d, { pageId: 'main', entries: [e1, e2], startY: 0, C: Cof(cat, d), limitsOf: cat.constraints, now: 1 }), a0 = run(E, d, c0); eq(E.doc.normalise(a0, { sizeOf: cat.defaultSize, constraints: cat.constraints }).pages[0].layouts.lg, page(a0).layouts.lg, 'startY 0 survives a reload unchanged'); }],
  ['commands: add with md/sm drop materialises that override and moves the item there', () => { const { E, cat, X } = boot();
    const d = mk(E, cat, [W('a', 'sensor', 'numeral'), W('b', 'sensor', 'numeral')], [it('a', 0, 0, 2, 1), it('b', 2, 0, 2, 1)]); assert.equal(page(d).layouts.sm, null);
    const c = X.add(d, { pageId: 'main', bp: 'sm', at: { x: 0, y: 0 }, entries: [X.entry(cat, 'sensor', 'numeral')], C: Cof(cat, d), limitsOf: cat.constraints, now: 1 }); assert.equal(c.materialised, 'sm');
    const after = run(E, d, c), n = c.select[0], sm = page(after).layouts.sm; assert.ok(Array.isArray(sm) && sm.length === 3); const mine = sm.find(i => i.id === n); eq([mine.x, mine.y], [0, 0]); assert.equal(page(after).layouts.md, null); assert.ok(page(after).layouts.lg.some(i => i.id === n));
    const lgDrop = X.add(d, { pageId: 'main', bp: 'lg', at: { x: 2, y: 0 }, entries: [X.entry(cat, 'sensor', 'sparkline')], C: Cof(cat, d), limitsOf: cat.constraints, now: 1 }), l2 = lgOf(run(E, d, lgDrop)); eq(l2[lgDrop.select[0]].slice(0, 2), [2, 0]); assert.ok(l2.b[1] >= 1, 'the occupant is pushed down'); }],
  ['commands: add at an occupied cell: an equally sized occupant swaps to the first free slot, a differently sized one is pushed down (contract C5)', () => { const { E, cat, X } = boot();
    const d = mk(E, cat, [W('a', 'sensor', 'numeral'), W('b', 'sensor', 'numeral')], [it('a', 0, 0, 2, 1), it('b', 4, 0, 2, 1)]), at = { x: 4, y: 0 };
    const same = X.add(d, { pageId: 'main', bp: 'lg', at, entries: [X.entry(cat, 'sensor', 'numeral')], C: Cof(cat, d), limitsOf: cat.constraints, now: 1 }), ls = lgOf(run(E, d, same));
    eq(ls[same.select[0]], [4, 0, 2, 1], 'equal size: the new widget sits where it was dropped'); eq(ls.b, [2, 0, 2, 1], 'equal size: b swaps to the first-fit slot of the new item, it is not pushed down'); eq(ls.a, [0, 0, 2, 1]);
    const other = X.add(d, { pageId: 'main', bp: 'lg', at, entries: [X.entry(cat, 'sensor', 'sparkline')], C: Cof(cat, d), limitsOf: cat.constraints, now: 1 }), lo = lgOf(run(E, d, other));
    eq(lo[other.select[0]], [4, 0, 2, 2], 'other size: the new widget sits where it was dropped'); eq(lo.b, [4, 2, 2, 1], 'other size: b is pushed below it'); eq(lo.a, [0, 0, 2, 1]);
    for (const [c, x] of [[same, run(E, d, same)], [other, run(E, d, other)]]) eq(E.doc.normalise(x, { sizeOf: cat.defaultSize, constraints: cat.constraints }).pages[0].layouts, page(x).layouts, 'reload-stable ' + c.select[0]); }],
  ['commands: remove, duplicate and paste round trips', () => { const { E, cat, X } = boot();
    const d = mk(E, cat, [W('a', 'sensor', 'numeral', { title: 'A' }), W('b', 'switch', 'tile')], [it('a', 0, 0, 2, 1), it('b', 2, 0, 2, 2)]); const r = X.remove(d, { pageId: 'main', ids: ['a', 'zz'], now: 1 });
    assert.equal(r.label, 'homedash.cmd.delete'); eq(r.select, []); eq(r.announce, { key: 'deleted', count: 1, id: 'a' }); const gone = run(E, d, r); eq(page(gone).widgets.map(w => w.id), ['b']); eq(back(E, gone, r), d); assert.equal(X.remove(d, { pageId: 'main', ids: ['zz'] }), null);
    const m = X.remove(d, { pageId: 'main', ids: ['a', 'b'], label: 'homedash.cmd.cut', kind: 'cut', now: 1 }); assert.equal(m.label, 'homedash.cmd.cut'); assert.equal(m.announce.key, 'deleted_many'); eq(back(E, run(E, d, m), m), d);
    const dup = X.duplicate(d, { pageId: 'main', ids: ['a', 'b'], C: Cof(cat, d), limitsOf: cat.constraints, now: 1 }); assert.equal(dup.label, 'homedash.cmd.duplicate'); const dd = run(E, d, dup), pl = page(dd).layouts.lg; assert.equal(dup.select.length, 2);
    eq(E.doc.normalise(dd, { sizeOf: cat.defaultSize, constraints: cat.constraints }).pages[0].layouts, page(dd).layouts, 'a compact page: the duplicates settle as a reload would leave them'); for (const id of dup.select) assert.ok(pl.some(i => i.id === id));
    const fd = mk(E, cat, [W('a', 'sensor', 'numeral', { title: 'A' }), W('b', 'switch', 'tile')], [it('a', 0, 0, 2, 1), it('b', 2, 0, 2, 2)], { compact: false }), fdup = X.duplicate(fd, { pageId: 'main', ids: ['a', 'b'], C: Cof(cat, fd), limitsOf: cat.constraints, now: 1 });
    for (const id of fdup.select) assert.ok(lgOf(run(E, fd, fdup))[id][1] >= 2, 'a free page: below the selection bbox'); assert.equal(page(dd).widgets.find(w => w.id === dup.select[0]).title, 'A'); assert.ok(!dup.select.includes('a'));
    const pst = X.paste(d, { pageId: 'main', entries: [X.entry(cat, 'sensor', 'numeral')], at: { x: 5, y: 5 }, startY: 0, C: Cof(cat, d), limitsOf: cat.constraints, now: 1 }); assert.equal(pst.label, 'homedash.cmd.paste'); assert.equal(pst.meta.kind, 'paste'); assert.ok(lgOf(run(E, d, pst))[pst.select[0]][1] < 5, 'paste ignores at'); }],
  ['commands: setWidget, bind, variant, family, align and resetLayout', () => { const { E, cat, X } = boot();
    const d = mk(E, cat, [W('a', 'light', 'slider'), W('b', 'sensor', 'numeral'), W('c', 'sensor', 'numeral')], [it('a', 0, 0, 2, 1), it('b', 3, 1, 2, 1), it('c', 6, 2, 2, 1)], null);
    const s1 = X.setWidget(d, { pageId: 'main', id: 'b', path: ['title'], value: 'Hi', text: true, now: 1 }); assert.equal(s1.meta.coalesce, 'field|b|title'); assert.equal(s1.meta.window, 1000); assert.equal(s1.label, 'homedash.cmd.option'); assert.equal(X.setWidget(d, { pageId: 'main', id: 'b', path: ['title'], value: '' }), null);
    const s2 = X.setWidget(d, { pageId: 'main', id: 'b', path: ['options', 'decimals'], value: 2, now: 1 }); assert.equal(s2.meta.coalesce, undefined); assert.equal(page(run(E, d, s2)).widgets[1].options.decimals, 2);
    const b1 = X.bind(d, { pageId: 'main', id: 'b', entity: 'ha:sensor.y', now: 1 }); assert.equal(b1.label, 'homedash.cmd.bind'); assert.equal(page(run(E, d, b1)).widgets[1].bindings.primary, 'ha:sensor.y');
    const b0 = X.bind(d, { pageId: 'main', id: 'b', entity: '', now: 1 }); assert.equal(b0.label, 'homedash.cmd.unbind'); assert.equal('primary' in page(run(E, d, b0)).widgets[1].bindings, false); eq(back(E, run(E, d, b0), b0), d);
    const dm = E.doc.apply(d, [E.doc.op(d, ['pages', 'main', 'layouts', 'md'], [it('a', 0, 0, 2, 1), it('b', 2, 0, 2, 1), it('c', 4, 0, 2, 1)])], 'after');
    const v = X.variant(dm, { pageId: 'main', id: 'a', variant: 'dial', limits: cat.constraints({ type: 'light', variant: 'dial' }), now: 1 }), vd = page(run(E, dm, v)); assert.equal(vd.widgets[0].variant, 'dial');
    eq(vd.layouts.lg.find(i => i.id === 'a').h, 2); eq(vd.layouts.md.find(i => i.id === 'a').h, 2); assert.equal(X.variant(dm, { pageId: 'main', id: 'a', variant: 'slider' }), null); eq(v.select, ['a']);
    const f = X.applyFamily(d, { pageId: 'main', ids: ['a', 'b'], family: 'minimal', now: 1 }); assert.equal(f.ops.length, 2); assert.equal(X.applyFamily(run(E, d, f), { pageId: 'main', ids: ['a', 'b'], family: 'minimal' }), null); assert.equal('family' in page(run(E, run(E, d, f), X.applyFamily(run(E, d, f), { pageId: 'main', ids: ['a'], family: '' }))).widgets[0], false);
    const al = X.align(d, { pageId: 'main', ids: ['a', 'b', 'c'], edge: 'left', primaryId: 'b', C: Cof(cat, d), now: 1 }), la = lgOf(run(E, d, al)); assert.equal(la.a[0], 3); assert.equal(la.c[0], 3); assert.equal(la.b[0], 3); assert.equal(al.meta.kind, 'align');
    const at = X.align(d, { pageId: 'main', ids: ['a', 'b'], edge: 'top', primaryId: 'b', C: Cof(cat, d), now: 1 }); assert.ok(at === null || lgOf(run(E, d, at)).a[1] >= 0);
    assert.equal(X.align(d, { pageId: 'main', ids: ['a'], edge: 'left', primaryId: 'zz', C: Cof(cat, d) }), null);
    assert.equal(X.resetLayout(d, { pageId: 'main', bp: 'md' }), null); assert.equal(X.resetLayout(d, { pageId: 'main', bp: 'lg' }), null); const rl = X.resetLayout(dm, { pageId: 'main', bp: 'md', now: 1 }); eq(rl.announce, { key: 'now_auto_md' }); assert.equal(page(run(E, dm, rl)).layouts.md, null); eq(back(E, run(E, dm, rl), rl), dm); }],
  ['commands: page builders (add, rename, icon, duplicate, move, delete with repairs, compact) and their limits', () => { const { E, cat, X } = boot();
    const widgets = [W('a', 'sensor', 'numeral', { actions: { tap: { do: 'navigate', page: 'p2' } }, options: { page: 'p2' } })];
    const raw = { name: 'T', kiosk: { start_page: 'p2', idle_dim_seconds: 5, idle_dim_level: 0.3, page_rotation_seconds: 0, screen_saver: 'clock', show_clock: true }, pages: [{ id: 'main', name: 'Main', widgets, layouts: { lg: [it('a', 0, 0, 2, 1)] } }, { id: 'p2', name: 'Two', widgets: [] }, { id: 'p3', name: 'Three', widgets: [] }] };
    const d = E.doc.normalise(raw, { sizeOf: cat.defaultSize, constraints: cat.constraints }), ids = doc => doc.pages.map(p => p.id);
    const dp = X.deletePage(d, { pageId: 'p2', now: 1 }), after = run(E, d, dp); eq(ids(after), ['main', 'p3']); assert.equal(after.kiosk.start_page, 'main'); eq(page(after).widgets[0].actions.tap, { do: 'default' }); assert.equal(page(after).widgets[0].options.page, 'main'); assert.equal(dp.page, 'main'); eq(back(E, after, dp), d);
    eq(X.deletePage(E.doc.normalise({}), { pageId: 'main' }), { refused: 'last_page' }); assert.equal(X.deletePage(d, { pageId: 'nope' }), null); assert.equal(X.deletePage(d, { pageId: 'p3' }).page, 'p2');
    const mp = X.movePage(d, { pageId: 'main', to: 2, now: 1 }); eq(ids(run(E, d, mp)), ['p2', 'p3', 'main']); eq(back(E, run(E, d, mp), mp), d); assert.equal(X.movePage(d, { pageId: 'main', to: 0 }), null); eq(ids(run(E, d, X.movePage(d, { pageId: 'p3', to: -5, now: 1 }))), ['p3', 'main', 'p2']);
    const ap = X.addPage(d, { name: 'x'.repeat(60), icon: 'star', now: 1 }), ad = run(E, d, ap); assert.equal(ad.pages.length, 4); assert.equal(ad.pages[3].name.length, 40); assert.equal(ap.page, ad.pages[3].id); assert.equal(ad.pages[3].icon, 'star'); eq(ad.pages[3].layouts, { lg: [], md: null, sm: null });
    const twelve = E.doc.normalise({ pages: Array.from({ length: 12 }, (_, i) => ({ id: 'p' + i, name: 'n' })) }); eq(X.addPage(twelve, { name: 'z' }), { refused: 'limit' }); eq(X.duplicatePage(twelve, { pageId: 'p0' }), { refused: 'limit' });
    const rn = X.renamePage(d, { pageId: 'main', name: '  New  ', now: 1 }); assert.equal(rn.meta.coalesce, 'page|main|name'); assert.equal(page(run(E, d, rn)).name, 'New'); assert.equal(X.renamePage(d, { pageId: 'main', name: 'Main' }), null); assert.equal(X.renamePage(d, { pageId: 'main', name: '   ' }), null);
    const ic = X.setPageIcon(d, { pageId: 'main', icon: 'star', now: 1 }); assert.equal(page(run(E, d, ic)).icon, 'star'); assert.equal(X.setPageIcon(run(E, d, ic), { pageId: 'main', icon: 'star' }), null);
    const dup = X.duplicatePage(d, { pageId: 'main', name: 'Copy', now: 1 }), dd = run(E, d, dup); eq(ids(dd).length, 4); assert.equal(dd.pages[1].id, dup.page); assert.notEqual(dd.pages[1].widgets[0].id, 'a'); eq(dd.pages[1].layouts.lg.map(i => i.id), [dd.pages[1].widgets[0].id]); assert.equal(dd.pages[1].name, 'Copy'); eq(back(E, dd, dup), d);
    const free = mk(E, cat, [W('a', 'sensor', 'numeral'), W('b', 'sensor', 'numeral')], [it('a', 0, 0, 2, 1), it('b', 0, 3, 2, 1)], { compact: false }), pc = X.setPageCompact(free, { pageId: 'main', compact: true, now: 1 }); eq(lgOf(run(E, free, pc)), { a: [0, 0, 2, 1], b: [0, 1, 2, 1] }); assert.equal(page(run(E, free, pc)).compact, true); assert.equal(X.setPageCompact(free, { pageId: 'main', compact: false }), null); eq(back(E, run(E, free, pc), pc), free);
    const sd = X.setDoc(d, { path: ['name'], value: 'Renamed', label: 'homedash.cmd.rename', text: true, now: 1 }); assert.equal(sd.meta.coalesce, 'doc|name'); assert.equal(run(E, d, sd).name, 'Renamed'); assert.equal(X.setDoc(d, { path: ['name'], value: d.name }), null);
    const sl = X.setDoc(d, { path: ['look', 'family'], value: 'minimal', now: 1 }); assert.equal(sl.label, 'homedash.cmd.look'); assert.equal(run(E, d, sl).look.family, 'minimal'); }],
  ['commands: undo of deletePage/duplicatePage/movePage through E.history keeps the document intact', () => { const { E, cat, X } = boot(), H = E.history.create();
    let d = E.doc.normalise({ pages: [{ id: 'main', name: 'A', widgets: [W('a', 'sensor', 'numeral')], layouts: { lg: [it('a', 0, 0, 2, 1)] } }, { id: 'p2', name: 'B' }, { id: 'p3', name: 'C' }] }, { sizeOf: cat.defaultSize, constraints: cat.constraints });
    const d0 = d; for (const c of [X.movePage(d, { pageId: 'main', to: 2, now: 1 })]) { H.push(c); d = run(E, d, c); } for (const c of [X.duplicatePage(d, { pageId: 'p2', name: 'B2', now: 2 })]) { H.push(c); d = run(E, d, c); } const c3 = X.deletePage(d, { pageId: 'p3', now: 3 }); H.push(c3); d = run(E, d, c3);
    for (let i = 0; i < 3; i++) d = H.undo(d).doc; eq(d, d0); for (let i = 0; i < 3; i++) d = H.redo(d).doc; eq(d.pages.map(p => p.id).length, 3); }],
  ['commands: keymap covers the C14 table and stays silent while typing or composing', () => { const { X } = boot(), k = (ev, c) => X.keymap(ev, c || { canvas: true });
    eq(k(key('z', { ctrlKey: true }), { canvas: false }), { action: 'undo' }); eq(k(key('Z', { metaKey: true, shiftKey: true }), { canvas: false }), { action: 'redo' }); eq(k(key('y', { ctrlKey: true }), { canvas: false }), { action: 'redo' }); assert.equal(k(key('y', { ctrlKey: true, shiftKey: true })), null);
    eq(k(key('c', { ctrlKey: true })), { action: 'copy' }); assert.equal(k(key('c', { ctrlKey: true }), { canvas: false }), null); eq(k(key('x', { metaKey: true })), { action: 'cut' }); eq(k(key('v', { ctrlKey: true })), { action: 'paste' }); eq(k(key('d', { ctrlKey: true })), { action: 'duplicate' }); eq(k(key('a', { ctrlKey: true })), { action: 'select_all' }); eq(k(key(' ', { ctrlKey: true })), { action: 'toggle_focused' });
    eq(k(key('ArrowRight')), { action: 'nudge', dx: 1, dy: 0 }); eq(k(key('ArrowUp')), { action: 'nudge', dx: 0, dy: -1 }); eq(k(key('ArrowRight', { shiftKey: true })), { action: 'resize_step', dw: 1, dh: 0 }); eq(k(key('ArrowDown', { shiftKey: true })), { action: 'resize_step', dw: 0, dh: 1 }); eq(k(key('ArrowUp', { altKey: true })), { action: 'focus_dir', dir: 'up' });
    eq(k(key('Delete')), { action: 'delete' }); eq(k(key('Backspace')), { action: 'delete' }); eq(k(key(' ')), { action: 'select_focused' }); eq(k(key('Enter')), { action: 'focus_inspector' }); eq(k(key('F2')), { action: 'focus_inspector' }); eq(k(key('Tab')), { action: 'focus_next' }); eq(k(key('Tab', { shiftKey: true })), { action: 'focus_prev' });
    eq(k(key('PageDown', { ctrlKey: true, altKey: true }), { canvas: false }), { action: 'page_next' }); eq(k(key('PageUp', { metaKey: true, altKey: true })), { action: 'page_prev' }); eq(k(key('Escape'), { canvas: false }), { action: 'escape' });
    assert.equal(k(key('ArrowRight'), { canvas: false }), null); assert.equal(k(key('z', { ctrlKey: true }), { typing: true }), null); assert.equal(k(key('Escape'), { typing: true }), null); assert.equal(k(key('z', { ctrlKey: true, isComposing: true })), null); assert.equal(k(null), null); assert.equal(k(key('q')), null); assert.equal(k(key('ArrowLeft', { altKey: true, shiftKey: true })), null); }],
  ['commands: spatial focus picks the nearest neighbour in the direction and ignores the ones behind; reading order', () => { const { X } = boot();
    const items = [it('a', 0, 0, 2, 1), it('b', 2, 0, 2, 1), it('c', 8, 0, 2, 1), it('d', 0, 2, 2, 1), it('e', 4, 4, 2, 1)];
    assert.equal(X.spatial(items, 'a', 'right'), 'b'); assert.equal(X.spatial(items, 'b', 'right'), 'c'); assert.equal(X.spatial(items, 'a', 'left'), null); assert.equal(X.spatial(items, 'a', 'down'), 'd'); assert.equal(X.spatial(items, 'd', 'up'), 'a'); assert.equal(X.spatial(items, 'zz', 'up'), null);
    eq(X.readingOrder([it('z', 4, 0, 1, 1), it('y', 0, 2, 1, 1), it('x', 0, 0, 1, 1)]), ['x', 'z', 'y']); assert.equal(X.spatial([it('l', 0, 0, 2, 1), it('m', 3, 0, 2, 1), it('n', 3, 0, 2, 1)], 'l', 'right'), 'm', 'ties break by id'); }],
  ['commands: entry, layout and constants', () => { const { E, cat, X } = boot();
    assert.equal(X.entry(cat, 'nope', 'x'), null); assert.equal(X.entry(null, 'sensor', 'numeral'), null); eq(X.entry(cat, 'sensor', 'sparkline').size, { w: 2, h: 2 }); assert.equal(X.entry(cat, 'sensor', 'bogus').widget.variant, 'numeral', 'unknown variant falls back to the first');
    eq([X.NUDGE_MS, X.FIELD_MS, X.LIMITS.pages, X.LIMITS.pageWidgets, X.LIMITS.widgets], [800, 1000, 12, 120, 400]);
    const d = mk(E, cat, [W('a', 'sensor', 'numeral')], [it('a', 0, 0, 2, 1)]), l = X.layout(d, { pageId: 'main', bp: 'lg', next: [it('a', 3, 0, 2, 1)], ids: ['a'], now: 1 }); assert.equal(l.label, 'homedash.cmd.move'); assert.equal(X.layout(d, { pageId: 'main', next: page(d).layouts.lg }), null); }]
];
```
- [ ] Wiring step W. PASS. Commit — `feat(homedash): designer command catalogue, key map and spatial focus`.

### Task A9: input.track, icons, widgets snapshot and design-mode more button
**Files:** `hd-input.js`, `hd-icons.js`, `hd-widgets.js`, `input.test.mjs`, `icons.test.mjs`, `widgets.test.mjs`, `control.test.mjs`. **Existing symbols changed:** `E.input` export (additive), `PATHS`/`icon` (hd-icons, additive), `host` (`update` hides `.hd-more` outside view — HIGH reach, every widget; behaviour additive), `E.widgets` export.
- [ ] `hd-input.js` — add before the export (uses existing `primary`, `finite`, `THRESHOLD_PX`):
```js
    // track(ev, o): one pointer session from a pointerdown the caller received (delegated listeners: canvas, palette). Primary pointer and main button
    // only (null otherwise). Below the threshold nothing happens; o.axis 'x' abandons a mostly vertical first move (a touch list scrolls); o.holdMs starts
    // the drag after a still press. onEnd(point, start, how): 'up', 'cancel' (pointercancel, a lost capture of touch or pen, cancel()), 'lost' (mouse move
    // without a button). A mouse keeps its session when the capture is lost with the button down (re-render; Infra F14). A tap calls onTap. A click
    // within 400 ms after a drag is swallowed.
    function track(ev, opts) {
        const o = opts || {}; if (!ev || !primary(ev)) return null;
        const cap = o.capture || null, doc = cap && cap.ownerDocument, win = o.win || (doc && doc.defaultView) || null;
        const setT = o.setTimeout || ((fn, ms) => setTimeout(fn, ms)), clearT = o.clearTimeout || (id => clearTimeout(id)), now = o.now || (() => Date.now());
        const life = E.core.bag(), threshold = finite(o.threshold) ? o.threshold : THRESHOLD_PX, start = { x: ev.clientX, y: ev.clientY, type: ev.pointerType, target: ev.target, shift: !!ev.shiftKey, ctrl: !!(ev.ctrlKey || ev.metaKey) };
        let drag = false, done = false, timer = null, last = start, captured = false;
        const mine = e => e.pointerId === ev.pointerId, point = e => ({ x: e.clientX, y: e.clientY, type: start.type, target: e.target, shift: !!e.shiftKey, ctrl: !!(e.ctrlKey || e.metaKey) });
        function begin(p) { drag = true; if (timer !== null) { clearT(timer); timer = null; } if (o.onStart) o.onStart(start, p); }
        function swallow() { const until = now() + 400, eat = e => { win.removeEventListener('click', eat, true); if (now() <= until) { e.preventDefault(); e.stopPropagation(); } };
            win.addEventListener('click', eat, true); setT(() => win.removeEventListener('click', eat, true), 450); }
        function end(how, p) { if (done) return; done = true; if (timer !== null) clearT(timer); life.dispose();
            if (captured && cap && typeof cap.releasePointerCapture === 'function') { try { cap.releasePointerCapture(ev.pointerId); } catch (err) { /* released with the up */ } }
            if (drag) { if (how === 'up' && win) swallow(); if (o.onEnd) o.onEnd(how === 'up' ? p : last, start, how); } else if (how === 'up' && o.onTap) o.onTap(start, p); }
        if (cap && typeof cap.setPointerCapture === 'function') { try { cap.setPointerCapture(ev.pointerId); captured = true; } catch (err) { /* synthetic pointer: window listeners carry it */ } }
        if (finite(o.holdMs) && o.holdMs > 0) timer = setT(() => { timer = null; if (!done && !drag) begin(start); }, o.holdMs);
        const c = { capture: true };
        if (win) {
            life.on(win, 'pointermove', e => { if (!mine(e) || done) return; if (start.type === 'mouse' && e.buttons === 0) { end('lost', last); return; }
                const p = point(e); if (!drag) { const dx = p.x - start.x, dy = p.y - start.y; if (Math.hypot(dx, dy) < threshold) return;
                    if (o.axis === 'x' && Math.abs(dy) > Math.abs(dx)) { end('scroll', p); return; } begin(p); }
                last = p; if (o.onMove) o.onMove(p, start); }, c);
            life.on(win, 'pointerup', e => { if (mine(e)) end('up', point(e)); }, c);
            life.on(win, 'pointercancel', e => { if (mine(e)) end('cancel', last); }, c);
        }
        if (cap) life.on(cap, 'lostpointercapture', e => { if (!mine(e) || done) return; captured = false; if (start.type !== 'mouse') end('cancel', last); });
        return Object.freeze({ cancel() { end('cancel', last); }, dragging: () => drag, done: () => done });
    }
```
Export adds `track`.
- [ ] `hd-icons.js` — add to `PATHS`:
```js
        undo: ['M9 14L4 9l5-5', 'M4 9h10.5a5.5 5.5 0 0 1 0 11H11'], redo: ['M15 14l5-5-5-5', 'M20 9H9.5a5.5 5.5 0 0 0 0 11H13'],
        copy: ['M9 9h11v11H9z', 'M5 15H4V4h11v1'], paste: ['M9 4h6v3H9z', 'M15 5.5h3V21H6V5.5h3', 'M9 12h6', 'M9 16h4'],
        cut: ['M6 3.5a2.5 2.5 0 1 0 0 5a2.5 2.5 0 1 0 0-5z', 'M6 15.5a2.5 2.5 0 1 0 0 5a2.5 2.5 0 1 0 0-5z', 'M8.1 7.6L20 18', 'M8.1 16.4L20 6'],
        edit: ['M4 20h4L19 9l-4-4L4 16z', 'M13.5 6.5l4 4'], duplicate: ['M8 8h12v12H8z', 'M4 16V4h12', 'M14 11v6', 'M11 14h6'],
        'panel-left': ['M4 5h16v14H4z', 'M9 5v14'], 'panel-right': ['M4 5h16v14H4z', 'M15 5v14'], window: ['M4 5h16v14H4z', 'M4 9h16', 'M7 7h.01'],
        grip: ['M9 6h.01', 'M15 6h.01', 'M9 12h.01', 'M15 12h.01', 'M9 18h.01', 'M15 18h.01'],
        look: ['M12 3a9 9 0 1 0 0 18a9 9 0 1 0 0-18z', 'M12 3v18', 'M12 7.5h4.5', 'M12 12h6', 'M12 16.5h4.5'],
        layers: ['M12 4l8.5 4.5L12 13 3.5 8.5z', 'M3.5 12.5L12 17l8.5-4.5', 'M3.5 16.5L12 21l8.5-4.5'],
        'align-left': ['M4 3v18', 'M8 7h11v4H8z', 'M8 14h7v4H8z'], 'align-top': ['M3 4h18', 'M7 8h4v11H7z', 'M14 8h4v7h-4z'],
        search: ['M10.5 4a6.5 6.5 0 1 0 0 13a6.5 6.5 0 1 0 0-13z', 'M15.5 15.5L20 20'],
        'device-desktop': ['M3.5 5h17v11h-17z', 'M9 20h6', 'M12 16v4'], 'device-tablet': ['M6 3.5h12v17H6z', 'M11 17.5h2'], 'device-phone': ['M8 3h8v18H8z', 'M11 18h2'],
        fullscreen: ['M4 9V4h5', 'M15 4h5v5', 'M20 15v5h-5', 'M9 20H4v-5'], sync: ['M20 11a8 8 0 0 0-14.3-4.9', 'M4 4v4h4', 'M4 13a8 8 0 0 0 14.3 4.9', 'M20 20v-4h-4'],
        'cloud-off': ['M3 3l18 18', 'M8 7.5A5 5 0 0 1 16.6 10H17a4 4 0 0 1 2.4 7.2', 'M16 19H7a4 4 0 0 1-1.3-7.8'], 'arrow-right': ['M5 12h14', 'M13 6l6 6-6 6']
```
plus `const UI_GLYPHS = new Set(['more', 'check', 'close', 'chevron-down', 'arrow-left', 'arrow-right', 'minus', 'plus', 'trash', 'refresh', 'signal-off', 'alert', 'info', 'mode-auto', 'trend-up', 'trend-down', 'trend-flat', 'undo', 'redo', 'copy', 'cut', 'paste', 'edit', 'duplicate', 'panel-left', 'panel-right', 'window', 'grip', 'look', 'layers', 'align-left', 'align-top', 'search', 'device-desktop', 'device-tablet', 'device-phone', 'fullscreen', 'sync', 'cloud-off']);` and export `choices: () => Object.keys(PATHS).filter(n => !UI_GLYPHS.has(n))`. Header: "M2–M4 set".
- [ ] `hd-widgets.js` — in `update()` replace the moreBtn line with `if (moreBtn) setAttr(moreBtn, 'hidden', mode !== 'view' || (typeof o.moreWhen === 'function' && !o.moreWhen(p)));`. Add (uses `resolveColour`):
```js
    // The palette snapshot (frontend 5.12, D7): the catalog's palette tokens as resolved on `root`, lower-case #rrggbb, only names the root defines,
    // at most 40. Saved with mode theme so the kiosk shows the desktop theme's colours.
    function paletteSnapshot(root, names) {
        const out = {}; if (!root || root.nodeType !== 1) return out;
        const doc = root.ownerDocument, view = doc && doc.defaultView; if (!view || typeof view.getComputedStyle !== 'function') return out;
        const cs = view.getComputedStyle(root), probe = el('span', { 'aria-hidden': 'true' }); probe.style.setProperty('display', 'none'); root.appendChild(probe);
        try { for (const n of (names || []).slice(0, 40)) { if (!/^[a-z0-9-]{1,32}$/.test(n) || !String(cs.getPropertyValue('--hd-' + n) || '').trim()) continue;
            probe.style.setProperty('color', 'var(--hd-' + n + ')'); const rgb = resolveColour(doc, view.getComputedStyle(probe).color); if (rgb) out[n] = E.colour.rgbToHex(rgb).toLowerCase(); } }
        finally { probe.remove(); }
        return out;
    }
```
export adds `paletteSnapshot`.
- [ ] Tests (diffs against the existing test files): `input.test.mjs` (three `track` tests), `icons.test.mjs`, `widgets.test.mjs` (`paletteSnapshot`; the unchanged expectations still pass), `control.test.mjs` (`.hd-more` hidden in design and thumb, shown again in view).
<!-- embed: dryrun-m4/overlay/diffs/input.test.mjs.diff -->
```diff
--- a/scripts/homedash-tests/input.test.mjs
+++ b/scripts/homedash-tests/input.test.mjs
@@ -48,3 +48,28 @@ export const tests = [
     E.input.repeat(b, { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, onStep: () => n++ });
-    b.dispatchEvent({ type: 'click' }); assert.equal(n, 1); b.dispatchEvent(pe('pointerdown', 0, 0)); await clock.advance(400 + 80 * 3); b.dispatchEvent(pe('pointerup', 0, 0)); b.dispatchEvent({ type: 'click', preventDefault() {} }); assert.equal(n, 5); }]
+    b.dispatchEvent({ type: 'click' }); assert.equal(n, 1); b.dispatchEvent(pe('pointerdown', 0, 0)); await clock.advance(400 + 80 * 3); b.dispatchEvent(pe('pointerup', 0, 0)); b.dispatchEvent({ type: 'click', preventDefault() {} }); assert.equal(n, 5); }],
+  ['input (M4): track starts at the threshold, ends with up, taps without a move, ignores non-primary pointers', () => { const { E, document } = boot(), clock = fakeClock(), log = [], T = { win: document, setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now };
+    const mouse = (type, x, y, extra) => pe(type, x, y, Object.assign({ pointerType: 'mouse', buttons: 1 }, extra));
+    const t = E.input.track(mouse('pointerdown', 10, 10), Object.assign({ onStart: (s, p) => log.push('start:' + p.x), onMove: p => log.push('move:' + p.x), onEnd: (p, s, how) => log.push('end:' + how + ':' + p.x), onTap: () => log.push('tap') }, T));
+    document.dispatchEvent(mouse('pointermove', 13, 10)); assert.deepEqual(log, []); assert.equal(t.dragging(), false); document.dispatchEvent(mouse('pointermove', 14, 10)); document.dispatchEvent(mouse('pointermove', 30, 10)); document.dispatchEvent(mouse('pointerup', 31, 10));
+    assert.deepEqual(log, ['start:14', 'move:14', 'move:30', 'end:up:31']); assert.equal(t.done(), true); document.dispatchEvent(mouse('pointermove', 50, 10)); assert.equal(log.length, 4);
+    log.length = 0; E.input.track(mouse('pointerdown', 10, 10), Object.assign({ onTap: () => log.push('tap'), onEnd: () => log.push('end') }, T)); document.dispatchEvent(mouse('pointerup', 10, 10)); assert.deepEqual(log, ['tap']);
+    assert.equal(E.input.track(pe('pointerdown', 1, 1, { isPrimary: false }), T), null); assert.equal(E.input.track(mouse('pointerdown', 1, 1, { button: 2 }), T), null); assert.equal(E.input.track(null, T), null);
+    log.length = 0; E.input.track(mouse('pointerdown', 10, 10), Object.assign({ threshold: 8, onStart: () => log.push('start') }, T)); document.dispatchEvent(mouse('pointermove', 16, 10)); assert.deepEqual(log, []); document.dispatchEvent(mouse('pointermove', 18, 10)); assert.deepEqual(log, ['start']); document.dispatchEvent(mouse('pointerup', 18, 10)); }],
+  ['input (M4): track axis x abandons a vertical first move, holdMs starts after a still press, a mouse move without a button is lost', async () => { const { E, document } = boot(), clock = fakeClock(), log = [], T = { win: document, setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now };
+    const handlers = tag => ({ onStart: () => log.push(tag + ':start'), onEnd: (p, s, how) => log.push(tag + ':end:' + how), onTap: () => log.push(tag + ':tap') });
+    E.input.track(pe('pointerdown', 10, 10), Object.assign({ axis: 'x', threshold: 8 }, handlers('x'), T)); document.dispatchEvent(pe('pointermove', 11, 30)); document.dispatchEvent(pe('pointermove', 60, 30)); document.dispatchEvent(pe('pointerup', 60, 30)); assert.deepEqual(log, []);
+    E.input.track(pe('pointerdown', 10, 10), Object.assign({ axis: 'x', threshold: 8 }, handlers('h'), T)); document.dispatchEvent(pe('pointermove', 30, 12)); document.dispatchEvent(pe('pointerup', 30, 12)); assert.deepEqual(log, ['h:start', 'h:end:up']);
+    log.length = 0; E.input.track(pe('pointerdown', 10, 10), Object.assign({ holdMs: 300 }, handlers('hold'), T)); await clock.advance(299); assert.deepEqual(log, []); await clock.advance(2); assert.deepEqual(log, ['hold:start']); document.dispatchEvent(pe('pointerup', 10, 10)); assert.deepEqual(log, ['hold:start', 'hold:end:up']);
+    log.length = 0; E.input.track(pe('pointerdown', 10, 10), Object.assign({ holdMs: 300 }, handlers('moved'), T)); document.dispatchEvent(pe('pointermove', 40, 10)); await clock.advance(400); document.dispatchEvent(pe('pointerup', 40, 10)); assert.deepEqual(log, ['moved:start', 'moved:end:up'], 'the hold timer is cleared once the drag has begun');
+    log.length = 0; E.input.track(pe('pointerdown', 10, 10, { pointerType: 'mouse', buttons: 1 }), Object.assign({}, handlers('m'), T)); document.dispatchEvent(pe('pointermove', 30, 10, { pointerType: 'mouse', buttons: 1 })); document.dispatchEvent(pe('pointermove', 40, 10, { pointerType: 'mouse', buttons: 0 })); assert.deepEqual(log, ['m:start', 'm:end:lost']);
+    log.length = 0; const s = E.input.track(pe('pointerdown', 10, 10), Object.assign({}, handlers('c'), T)); document.dispatchEvent(pe('pointermove', 30, 10)); s.cancel(); document.dispatchEvent(pe('pointerup', 30, 10)); assert.deepEqual(log, ['c:start', 'c:end:cancel']); s.cancel(); assert.equal(log.length, 2);
+    log.length = 0; E.input.track(pe('pointerdown', 10, 10), Object.assign({}, handlers('pc'), T)); document.dispatchEvent(pe('pointermove', 30, 10)); document.dispatchEvent(pe('pointercancel', 30, 10)); assert.deepEqual(log, ['pc:start', 'pc:end:cancel']); }],
+  ['input (M4): track swallows the click after a drag for 400 ms only, and a mouse survives a lost capture but touch does not', async () => { const { E, document } = boot(), clock = fakeClock(), log = [], T = { win: document, setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now };
+    const click = () => { const ev = { type: 'click', preventDefault() { log.push('prevented'); }, stopPropagation() { log.push('stopped'); } }; document.dispatchEvent(ev); };
+    E.input.track(pe('pointerdown', 10, 10), T); document.dispatchEvent(pe('pointermove', 30, 10)); document.dispatchEvent(pe('pointerup', 30, 10)); click(); assert.deepEqual(log, ['prevented', 'stopped']); log.length = 0; click(); assert.deepEqual(log, [], 'one click only');
+    E.input.track(pe('pointerdown', 10, 10), T); document.dispatchEvent(pe('pointermove', 30, 10)); document.dispatchEvent(pe('pointerup', 30, 10)); await clock.advance(500); click(); assert.deepEqual(log, [], 'a late click is not swallowed');
+    E.input.track(pe('pointerdown', 10, 10), T); document.dispatchEvent(pe('pointerup', 10, 10)); click(); assert.deepEqual(log, [], 'a tap does not swallow');
+    const cap = document.createElement('div'), calls = []; cap.setPointerCapture = id => calls.push('set:' + id); cap.releasePointerCapture = id => calls.push('release:' + id);
+    const ends = []; E.input.track(pe('pointerdown', 10, 10, { pointerType: 'mouse', buttons: 1 }), Object.assign({ capture: cap, onEnd: (p, s, how) => ends.push(how) }, T)); document.dispatchEvent(pe('pointermove', 30, 10, { pointerType: 'mouse', buttons: 1 })); cap.dispatchEvent({ type: 'lostpointercapture', pointerId: 1 }); assert.deepEqual(ends, []); document.dispatchEvent(pe('pointerup', 30, 10, { pointerType: 'mouse', buttons: 0 })); assert.deepEqual(ends, ['up']); assert.equal(calls[0], 'set:1');
+    const te = []; E.input.track(pe('pointerdown', 10, 10), Object.assign({ capture: cap, onEnd: (p, s, how) => te.push(how) }, T)); document.dispatchEvent(pe('pointermove', 30, 10)); cap.dispatchEvent({ type: 'lostpointercapture', pointerId: 1 }); assert.deepEqual(te, ['cancel']); }]
 ];
```

<!-- embed: dryrun-m4/overlay/diffs/icons.test.mjs.diff -->
```diff
--- a/scripts/homedash-tests/icons.test.mjs
+++ b/scripts/homedash-tests/icons.test.mjs
@@ -5,2 +5,8 @@ export const tests = [['icons: every contract name and the M3 UI glyphs are draw
   const { E } = loadEngine(['hd-core.js', 'hd-icons.js'], { dom: true });
-  for (const n of CONTRACT.concat(NEW)) { assert.ok(E.icons.has(n), n); const svg = E.icons.icon(n); assert.equal(svg.getAttribute('data-hd-icon'), n); assert.ok(svg.querySelectorAll('path').length > 0, n); } }]];
+  for (const n of CONTRACT.concat(NEW)) { assert.ok(E.icons.has(n), n); const svg = E.icons.icon(n); assert.equal(svg.getAttribute('data-hd-icon'), n); assert.ok(svg.querySelectorAll('path').length > 0, n); } }],
+  ['icons (M4 C19): every editor glyph is drawn and choices() leaves the UI glyphs out of the picker', () => {
+  const { E } = loadEngine(['hd-core.js', 'hd-icons.js'], { dom: true }), C19 = 'undo redo copy cut paste edit duplicate panel-left panel-right window grip look layers align-left align-top search device-desktop device-tablet device-phone fullscreen sync cloud-off arrow-right'.split(' ');
+  for (const n of C19) { assert.ok(E.icons.has(n), n); const svg = E.icons.icon(n); assert.equal(svg.getAttribute('data-hd-icon'), n); assert.ok(svg.querySelectorAll('path').length > 0, n); }
+  const choices = E.icons.choices(); assert.ok(choices.includes('bulb') && choices.includes('home') && choices.includes('cpu')); for (const n of C19.concat(['more', 'check', 'close', 'plus', 'trash', 'arrow-left'])) assert.ok(!choices.includes(n), n);
+  assert.equal(E.icons.icon('nope').getAttribute('data-hd-icon'), 'entity'); assert.equal(new Set(E.icons.names()).size, E.icons.names().length); }]
+];
```

<!-- embed: dryrun-m4/overlay/diffs/widgets.test.mjs.diff -->
```diff
--- a/scripts/homedash-tests/widgets.test.mjs
+++ b/scripts/homedash-tests/widgets.test.mjs
@@ -77,3 +77,11 @@ export const tests = [
     h.setMode('view'); assert.equal(frame.hasAttribute('inert'), false); assert.equal(h.el.hasAttribute('aria-hidden'), false); assert.equal(h.body.hasAttribute('inert'), false);
-    h.setMode('design'); assert.equal(frame.hasAttribute('inert'), false); assert.equal(h.body.hasAttribute('inert'), true); }]
+    h.setMode('design'); assert.equal(frame.hasAttribute('inert'), false); assert.equal(h.body.hasAttribute('inert'), true); }],
+  ['widgets (M4): paletteSnapshot resolves the defined tokens to lower-case hex, skips undefined and malformed names, caps at 40 and leaves no probe behind', () => {
+    const { E, document } = loadEngine(ENGINE_ORDER, { dom: true }), root = document.createElement('div'); document.body.appendChild(root);
+    const theme = { '--hd-ink': 'rgb(240, 240, 240)', '--hd-canvas': 'rgb(16, 20, 30)', '--hd-accent': 'rgb(122, 167, 255)' };
+    document.defaultView = { getComputedStyle: node => { if (node === root) return { getPropertyValue: n => (theme[n] ? 'x' : '') }; const m = /var\((.*)\)/.exec(node.style.getPropertyValue('color')); return { color: m && theme[m[1]] ? theme[m[1]] : 'rgba(0, 0, 0, 0)' }; } };
+    const snap = E.widgets.paletteSnapshot(root, ['ink', 'canvas', 'accent', 'missing', 'Bad Name', '', 'x'.repeat(40)]); assert.deepEqual({ ...snap }, { ink: '#f0f0f0', canvas: '#10141e', accent: '#7aa7ff' }); assert.equal(root.children.length, 0, 'the probe is removed');
+    assert.deepEqual({ ...E.widgets.paletteSnapshot(root, []) }, {}); assert.deepEqual({ ...E.widgets.paletteSnapshot(null, ['ink']) }, {}); assert.deepEqual({ ...E.widgets.paletteSnapshot({ nodeType: 3 }, ['ink']) }, {});
+    const many = Array.from({ length: 60 }, (_, i) => 't' + i); for (const n of many) theme['--hd-' + n] = 'rgb(1, 2, 3)'; assert.equal(Object.keys(E.widgets.paletteSnapshot(root, many)).length, 40); assert.equal(root.children.length, 0);
+    document.defaultView = undefined; assert.deepEqual({ ...E.widgets.paletteSnapshot(root, ['ink']) }, {}); }]
 ];
```

<!-- embed: dryrun-m4/overlay/diffs/control.test.mjs.diff -->
```diff
--- a/scripts/homedash-tests/control.test.mjs
+++ b/scripts/homedash-tests/control.test.mjs
@@ -40,3 +40,4 @@ export const tests = [
     const more = q('.hd-more'); assert.ok(more.querySelector('svg[data-hd-icon="more"]')); more.dispatchEvent({ type: 'click', preventDefault() {} }); assert.deepEqual(calls, [['primary']]);
-    h.setMode('design'); more.dispatchEvent({ type: 'click', preventDefault() {} }); assert.equal(calls.length, 1); h.destroy(); }]
+    h.setMode('design'); more.dispatchEvent({ type: 'click', preventDefault() {} }); assert.equal(calls.length, 1); assert.equal(more.hasAttribute('hidden'), true);
+    h.setMode('thumb'); assert.equal(more.hasAttribute('hidden'), true); h.setMode('view'); assert.equal(more.hasAttribute('hidden'), false); h.destroy(); }]
 ];
```
- [ ] PASS. Commit — `feat(homedash): pointer track sessions, editor glyphs, palette snapshot`.

### Task A10: Fixture extension (C18)
**Files:** `ui/desktop_homedash_fixture_test.go`, create `ui/desktop_homedash_fixture_m4_test.go`. **Existing symbols changed:** `homeDashFixture` (field `m4 *homeDashM4`), `ServeHTTP` (three cases; entity filters), `create` (documents kept and linted) — test-only, LOW.
- [ ] M2 file edits (`ui/desktop_homedash_fixture_test.go`), as a diff: the struct field, the three new routes, entity paging through `homeDashEntityPage`, and `create` keeping, linting and slug-checking the posted document (`slugTakenLocked` is defined in the M4 file):
<!-- embed: dryrun-m4/overlay/diffs/desktop_homedash_fixture_test.go.diff -->
```diff
--- a/ui/desktop_homedash_fixture_test.go
+++ b/ui/desktop_homedash_fixture_test.go
@@ -53,2 +53,3 @@ type homeDashFixture struct {
 	m3         *homeDashM3 // nil until enableM3 (desktop_homedash_fixture_m3_test.go)
+	m4         *homeDashM4 // nil until the first M4 save route or control (desktop_homedash_fixture_m4_test.go)
 }
@@ -350,3 +351,8 @@ func (f *homeDashFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
 		f.mu.Unlock()
-		homeDashJSON(w, 200, map[string]any{"status": "ok", "entities": list, "total": len(list), "limit": 100, "offset": 0, "sources": sources})
+		page, total, limit, offset, bad := homeDashEntityPage(list, r.URL.Query())
+		if bad != "" {
+			homeDashError(w, 400, "bad_request", "bad "+bad)
+			return
+		}
+		homeDashJSON(w, 200, map[string]any{"status": "ok", "entities": page, "total": total, "limit": limit, "offset": offset, "sources": sources})
 	case "GET entity":
@@ -385,2 +391,8 @@ func (f *homeDashFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
 		f.command(w, raw)
+	case "PUT dashboard":
+		f.update(w, r, raw)
+	case "POST dashboards/validate":
+		f.validate(w, raw)
+	case "POST dashboard/duplicate":
+		f.duplicate(w, r, raw)
 	}
@@ -610,14 +622,16 @@ func (f *homeDashFixture) create(w http.ResponseWriter, raw []byte) {
 	} else {
-		name, _ := body.Document["name"].(string)
-		pages, _ := body.Document["pages"].([]any)
-		if name == "" || len(pages) == 0 {
-			homeDashJSON(w, 422, map[string]any{"status": "error", "code": "invalid_document", "message": "invalid document", "issues": []any{map[string]any{"path": "/pages", "code": "required", "message": "at least one page"}}})
+		if code, issues := f.lintLocked(body.Document); code != "" || len(issues) > 0 {
+			homeDashJSON(w, 422, map[string]any{"status": "error", "code": "invalid_document", "message": "invalid document", "issues": issues})
 			return
 		}
-		page, _ := pages[0].(map[string]any)
-		doc = map[string]any{"schema_version": 1, "name": name, "slug": f.uniqueSlugLocked(homeDashSlug(name)),
-			"look":  map[string]any{"family": "glass", "palette": map[string]any{"mode": "theme"}, "backdrop": map[string]any{"kind": "theme"}, "density": "comfortable", "radius_scale": 1},
-			"grid":  map[string]any{"row_unit": "auto", "gap": nil, "fit": "scroll"},
-			"pages": []any{map[string]any{"id": page["id"], "name": page["name"], "compact": true, "widgets": []any{}, "layouts": map[string]any{"lg": []any{}}}},
-			"kiosk": map[string]any{"start_page": page["id"], "idle_dim_seconds": 120, "idle_dim_level": 0.3, "page_rotation_seconds": 0, "screen_saver": "clock", "show_clock": true}}
+		doc = homeDashClone(body.Document)
+		name, _ := doc["name"].(string)
+		if slug, _ := doc["slug"].(string); slug == "" || f.slugTakenLocked(slug) {
+			doc["slug"] = f.uniqueSlugLocked(homeDashSlug(name))
+		}
+		if doc["kiosk"] == nil {
+			pages, _ := doc["pages"].([]any)
+			first, _ := pages[0].(map[string]any)
+			doc["kiosk"] = map[string]any{"start_page": first["id"], "idle_dim_seconds": 120, "idle_dim_level": 0.3, "page_rotation_seconds": 0, "screen_saver": "clock", "show_clock": true}
+		}
 	}
```
- [ ] New file `ui/desktop_homedash_fixture_m4_test.go`, complete. The dry run compiled it (`go vet` clean) and ran it with `go test -overlay … ./ui -run '(?i)^TestHomeDashFixture'`: 7/7 PASS. The lint codes follow `validator.text` of the server; `slugTakenLocked` is the helper the first draft was missing; the unit test `TestHomeDashFixtureSaveRoutes` closes the file and also pins the lint codes:
<!-- embed: dryrun-m4/overlay/ui/desktop_homedash_fixture_m4_test.go -->
```go
package ui

// M4 additions to the HomeDash fixture (plan 2026-11-26-homedash-m4-designer.md, contract C18): PUT dashboard with the revision check (409
// revision_conflict + current_revision), POST dashboards/validate, POST dashboard/duplicate, entity filters and paging, a lint that mirrors the
// server's strict decoder (unknown field -> 422 invalid_document/bad_json) and the checks a designer can get wrong (ids, names, required bindings,
// lg layout), scripted PUT failures (answers, dropped connections, delay) and a foreign save that sends dashboard_changed.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func init() {
	homeDashRoutes["dashboard"][http.MethodPut] = true
	homeDashRoutes["dashboards/validate"] = map[string]bool{http.MethodPost: false}
	homeDashRoutes["dashboard/duplicate"] = map[string]bool{http.MethodPost: true}
}

type homeDashPut struct {
	ID       string
	Revision int
	Document map[string]any
	At       time.Time
}
type homeDashSaveAnswer struct {
	status int
	body   map[string]any
	times  int
}
type homeDashWidgetSpec struct {
	Type     string `json:"type"`
	Variants []struct {
		ID string `json:"id"`
	} `json:"variants"`
	Bindings []struct {
		Name     string `json:"name"`
		Required bool   `json:"required"`
	} `json:"bindings"`
}
type homeDashM4 struct {
	answers []*homeDashSaveAnswer
	drops   int
	delay   time.Duration
	puts    []homeDashPut
	types   map[string]homeDashWidgetSpec
}

var homeDashDocKeys = map[string][]string{
	"dashboard": {"schema_version", "id", "slug", "name", "icon", "revision", "approved_revision", "updated_at", "saved_by", "look", "grid", "pages", "kiosk"},
	"look": {"family", "palette", "backdrop", "density", "radius_scale"}, "palette": {"mode", "custom", "snapshot"}, "custom": {"accent", "surface_hue", "scheme"},
	"backdrop": {"kind"}, "grid": {"row_unit", "gap", "fit"}, "page": {"id", "name", "icon", "compact", "widgets", "layouts"},
	"widget": {"id", "type", "variant", "family", "title", "icon", "bindings", "options", "actions", "confirm"}, "actions": {"tap", "hold"}, "action": {"do", "page"},
	"layouts": {"lg", "md", "sm"}, "item": {"id", "x", "y", "w", "h"},
	"kiosk": {"start_page", "idle_dim_seconds", "idle_dim_level", "page_rotation_seconds", "screen_saver", "show_clock"},
}
var homeDashItemID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

func homeDashIssue(path, code, msg string) map[string]any {
	return map[string]any{"path": path, "code": code, "message": msg}
}

func (f *homeDashFixture) m4Locked() *homeDashM4 {
	if f.m4 == nil {
		var cat struct {
			Widgets []homeDashWidgetSpec `json:"widgets"`
		}
		_ = json.Unmarshal(f.catalog, &cat)
		f.m4 = &homeDashM4{types: map[string]homeDashWidgetSpec{}}
		for _, w := range cat.Widgets {
			f.m4.types[w.Type] = w
		}
	}
	return f.m4
}

// lintLocked returns ("bad_json", issues) for a field the strict decoder refuses, ("", issues) for document issues and ("", nil) when clean.
func (f *homeDashFixture) lintLocked(doc map[string]any) (string, []map[string]any) {
	var bad []map[string]any
	unknown := func(path, kind string, v any) {
		m, _ := v.(map[string]any)
		for k := range m {
			if !slices.Contains(homeDashDocKeys[kind], k) {
				bad = append(bad, homeDashIssue(path+"/"+k, "bad_json", `json: unknown field "`+k+`"`))
			}
		}
	}
	unknown("", "dashboard", doc)
	if look, _ := doc["look"].(map[string]any); look != nil {
		unknown("/look", "look", look)
		pal, _ := look["palette"].(map[string]any)
		unknown("/look/palette", "palette", pal)
		if pal != nil {
			unknown("/look/palette/custom", "custom", pal["custom"])
		}
		unknown("/look/backdrop", "backdrop", look["backdrop"])
	}
	unknown("/grid", "grid", doc["grid"])
	unknown("/kiosk", "kiosk", doc["kiosk"])
	pages, _ := doc["pages"].([]any)
	for i, rp := range pages {
		p, _ := rp.(map[string]any)
		pp := fmt.Sprintf("/pages/%d", i)
		unknown(pp, "page", p)
		lay, _ := p["layouts"].(map[string]any)
		unknown(pp+"/layouts", "layouts", lay)
		for _, bp := range []string{"lg", "md", "sm"} {
			items, _ := lay[bp].([]any)
			for j, it := range items {
				unknown(fmt.Sprintf("%s/layouts/%s/%d", pp, bp, j), "item", it)
			}
		}
		ws, _ := p["widgets"].([]any)
		for j, rw := range ws {
			wp := fmt.Sprintf("%s/widgets/%d", pp, j)
			unknown(wp, "widget", rw)
			w, _ := rw.(map[string]any)
			acts, _ := w["actions"].(map[string]any)
			unknown(wp+"/actions", "actions", acts)
			for _, k := range []string{"tap", "hold"} {
				if acts != nil {
					unknown(wp+"/actions/"+k, "action", acts[k])
				}
			}
		}
	}
	if len(bad) > 0 {
		return "bad_json", bad
	}
	var issues []map[string]any
	// Codes follow validator.text of the server: fewer characters than the minimum is "required", more than the maximum "too_long".
	textIssue := func(path string, v any, min, max int) {
		n := 0
		if s, ok := v.(string); ok {
			n = utf8.RuneCountInString(s)
		}
		switch {
		case n < min:
			issues = append(issues, homeDashIssue(path, "required", "required"))
		case n > max:
			issues = append(issues, homeDashIssue(path, "too_long", fmt.Sprintf("at most %d characters", max)))
		}
	}
	textIssue("/name", doc["name"], 1, 80)
	switch {
	case len(pages) == 0:
		issues = append(issues, homeDashIssue("/pages", "required", "at least one page"))
	case len(pages) > 12:
		issues = append(issues, homeDashIssue("/pages", "too_many", "at most 12 pages"))
	}
	seen := map[string]bool{}
	for i, rp := range pages {
		p, _ := rp.(map[string]any)
		pp := fmt.Sprintf("/pages/%d", i)
		if id, _ := p["id"].(string); !homeDashItemID.MatchString(id) {
			issues = append(issues, homeDashIssue(pp+"/id", "bad_format", "invalid page id"))
		}
		textIssue(pp+"/name", p["name"], 1, 40)
		ws, _ := p["widgets"].([]any)
		ids := map[string]bool{}
		for j, rw := range ws {
			w, _ := rw.(map[string]any)
			wp := fmt.Sprintf("%s/widgets/%d", pp, j)
			id, _ := w["id"].(string)
			switch {
			case !homeDashItemID.MatchString(id):
				issues = append(issues, homeDashIssue(wp+"/id", "bad_format", "invalid widget id"))
			case seen[id]:
				issues = append(issues, homeDashIssue(wp+"/id", "duplicate_id", "widget id used twice"))
			}
			seen[id], ids[id] = true, true
			spec, ok := f.m4Locked().types[fmt.Sprint(w["type"])]
			if !ok {
				issues = append(issues, homeDashIssue(wp+"/type", "unknown_type", "unknown widget type"))
				continue
			}
			if !slices.ContainsFunc(spec.Variants, func(v struct {
				ID string `json:"id"`
			}) bool {
				return v.ID == w["variant"]
			}) {
				issues = append(issues, homeDashIssue(wp+"/variant", "unknown_variant", "unknown variant"))
			}
			bind, _ := w["bindings"].(map[string]any)
			for _, b := range spec.Bindings {
				if v, _ := bind[b.Name].(string); b.Required && v == "" {
					issues = append(issues, homeDashIssue(wp+"/bindings/"+b.Name, "binding_required", "this binding is required"))
				}
			}
		}
		lay, _ := p["layouts"].(map[string]any)
		lg, _ := lay["lg"].([]any)
		placed, cells := map[string]bool{}, map[[2]int]bool{}
		for j, ri := range lg {
			it, _ := ri.(map[string]any)
			id := fmt.Sprint(it["id"])
			x, y, wd, ht := homeDashInt(it["x"]), homeDashInt(it["y"]), homeDashInt(it["w"]), homeDashInt(it["h"])
			lp := fmt.Sprintf("%s/layouts/lg/%d", pp, j)
			switch {
			case !ids[id]:
				issues = append(issues, homeDashIssue(lp, "layout_unknown_widget", "no such widget"))
			case placed[id]:
				issues = append(issues, homeDashIssue(lp, "layout_duplicate", "placed twice"))
			case x < 0 || wd < 1 || ht < 1 || x+wd > 12 || y < 0:
				issues = append(issues, homeDashIssue(lp, "layout_out_of_bounds", "outside the grid"))
			}
			placed[id] = true
			for cx := x; cx < x+wd && cx < 12; cx++ {
				for cy := y; cy < y+ht && cy < 500; cy++ {
					if cells[[2]int{cx, cy}] {
						issues = append(issues, homeDashIssue(lp, "layout_overlap", "overlaps another widget"))
						cx, cy = 12, 500
					}
					cells[[2]int{cx, cy}] = true
				}
			}
		}
		for id := range ids {
			if !placed[id] {
				issues = append(issues, homeDashIssue(pp+"/layouts/lg", "layout_missing", "widget "+id+" has no place"))
			}
		}
	}
	return "", issues
}

// slugTakenLocked reports whether a stored dashboard already uses the slug (used by create in desktop_homedash_fixture_test.go).
func (f *homeDashFixture) slugTakenLocked(slug string) bool {
	for _, d := range f.dashboards {
		if s, _ := d["slug"].(string); s == slug {
			return true
		}
	}
	return false
}

func (f *homeDashFixture) update(w http.ResponseWriter, r *http.Request, raw []byte) {
	id := r.URL.Query().Get("id")
	if !homeDashIDPattern.MatchString(id) {
		homeDashError(w, 400, "bad_request", "bad id")
		return
	}
	if len(raw) > 512<<10 {
		homeDashError(w, 413, "body_too_large", "request body too large")
		return
	}
	f.mu.Lock()
	m := f.m4Locked()
	delay, drop := m.delay, m.drops > 0
	if drop {
		m.drops--
	}
	var answer *homeDashSaveAnswer
	if len(m.answers) > 0 && !drop {
		answer = m.answers[0]
		if answer.times--; answer.times <= 0 {
			m.answers = m.answers[1:]
		}
	}
	f.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-f.quit:
		}
	}
	if drop {
		if hj, ok := w.(http.Hijacker); ok {
			if conn, _, err := hj.Hijack(); err == nil {
				_ = conn.Close()
				return
			}
		}
		homeDashError(w, 502, "bad_gateway", "connection dropped")
		return
	}
	if answer != nil {
		homeDashJSON(w, answer.status, answer.body)
		return
	}
	var body struct {
		Revision int            `json:"revision"`
		Document map[string]any `json:"document"`
	}
	if json.Unmarshal(raw, &body) != nil || body.Revision < 1 || body.Document == nil {
		homeDashError(w, 400, "bad_request", "send {revision, document}")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cur := f.dashboards[id]
	if cur == nil {
		homeDashError(w, 404, "dashboard_not_found", "no such dashboard")
		return
	}
	have := homeDashInt(cur["revision"])
	if body.Revision != have {
		homeDashJSON(w, 409, map[string]any{"status": "error", "code": "revision_conflict", "message": "stale revision", "current_revision": have})
		return
	}
	if code, issues := f.lintLocked(body.Document); code != "" || len(issues) > 0 {
		homeDashJSON(w, 422, map[string]any{"status": "error", "code": "invalid_document", "message": "invalid document", "issues": issues})
		return
	}
	next := homeDashClone(body.Document)
	if s, _ := next["slug"].(string); s == "" {
		next["slug"] = cur["slug"]
	}
	rev := have + 1
	next["id"], next["revision"], next["approved_revision"], next["updated_at"], next["saved_by"] = id, rev, rev, homeDashNow(), "user"
	f.dashboards[id] = next
	f.m4Locked().puts = append(f.m4Locked().puts, homeDashPut{ID: id, Revision: body.Revision, Document: homeDashClone(body.Document), At: time.Now()})
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": id, "revision": rev, "approved_revision": rev}))
	homeDashJSON(w, 200, map[string]any{"status": "ok", "dashboard": homeDashClone(next), "warnings": []any{}})
}

func (f *homeDashFixture) validate(w http.ResponseWriter, raw []byte) {
	var body struct {
		Document map[string]any `json:"document"`
	}
	if json.Unmarshal(raw, &body) != nil || body.Document == nil {
		homeDashError(w, 400, "bad_request", "send {document}")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	code, issues := f.lintLocked(body.Document)
	if code == "bad_json" {
		homeDashJSON(w, 422, map[string]any{"status": "error", "code": "invalid_document", "message": "invalid document", "issues": issues})
		return
	}
	if issues == nil {
		issues = []map[string]any{}
	}
	warnings := []map[string]any{}
	pages, _ := body.Document["pages"].([]any)
	for i, rp := range pages {
		p, _ := rp.(map[string]any)
		ws, _ := p["widgets"].([]any)
		for j, rw := range ws {
			wd, _ := rw.(map[string]any)
			bind, _ := wd["bindings"].(map[string]any)
			names := make([]string, 0, len(bind))
			for n := range bind {
				names = append(names, n)
			}
			slices.Sort(names)
			for _, n := range names {
				eid, _ := bind[n].(string)
				_, known := f.values[eid]
				if f.m3 != nil && f.m3.ha[eid] != nil {
					known = true
				}
				if eid != "" && !known {
					warnings = append(warnings, homeDashIssue(fmt.Sprintf("/pages/%d/widgets/%d/bindings/%s", i, j, n), "entity_unknown", "no such entity"))
				}
			}
		}
	}
	doc := homeDashClone(body.Document)
	if s, _ := doc["slug"].(string); s == "" {
		doc["slug"] = f.uniqueSlugLocked(homeDashSlug(fmt.Sprint(doc["name"])))
	}
	homeDashJSON(w, 200, map[string]any{"status": "ok", "valid": len(issues) == 0, "issues": issues, "warnings": warnings, "document": doc})
}

func (f *homeDashFixture) duplicate(w http.ResponseWriter, r *http.Request, raw []byte) {
	id := r.URL.Query().Get("id")
	if !homeDashIDPattern.MatchString(id) {
		homeDashError(w, 400, "bad_request", "bad id")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if len(raw) > 0 && json.Unmarshal(raw, &body) != nil {
		homeDashError(w, 400, "bad_request", "bad body")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	src := f.dashboards[id]
	if src == nil {
		homeDashError(w, 404, "dashboard_not_found", "no such dashboard")
		return
	}
	doc := homeDashClone(src)
	name := body.Name
	if name == "" {
		name = string([]rune(fmt.Sprint(src["name"]))[:min(73, utf8.RuneCountInString(fmt.Sprint(src["name"])))]) + " (copy)"
	}
	doc["name"] = name
	doc["slug"] = f.uniqueSlugLocked(fmt.Sprint(src["slug"]) + "-copy")
	f.seq++
	nid := homeDashID(f.seq)
	doc["id"], doc["revision"], doc["approved_revision"], doc["updated_at"], doc["saved_by"] = nid, 1, 1, homeDashNow(), "user"
	f.dashboards[nid] = doc
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": nid, "revision": 1, "approved_revision": 1}))
	homeDashJSON(w, 201, map[string]any{"status": "ok", "dashboard": homeDashClone(doc), "warnings": []any{}})
}

// homeDashEntityPage filters (source = id prefix, kind, area exact, q case-insensitive in name, id and area) and pages the entity list.
func homeDashEntityPage(list []any, q url.Values) (page []any, total, limit, offset int, bad string) {
	limit = 100
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 500 {
			return nil, 0, 0, 0, "limit"
		}
		limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return nil, 0, 0, 0, "offset"
		}
		offset = n
	}
	needle := strings.ToLower(strings.TrimSpace(q.Get("q")))
	var kept []any
	for _, raw := range list {
		e, _ := raw.(map[string]any)
		id, _ := e["id"].(string)
		if s := q.Get("source"); s != "" && !strings.HasPrefix(id, s+":") {
			continue
		}
		if k := q.Get("kind"); k != "" && e["kind"] != k {
			continue
		}
		if a := q.Get("area"); a != "" && e["area"] != a {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(fmt.Sprint(e["name"])+" "+id+" "+fmt.Sprint(e["area"])), needle) {
			continue
		}
		kept = append(kept, raw)
	}
	total = len(kept)
	if offset > total {
		offset = total
	}
	end := min(offset+limit, total)
	page = append([]any{}, kept[offset:end]...)
	return page, total, limit, offset, ""
}

// Controls.
func (f *homeDashFixture) answerPut(status int, body map[string]any, times int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := f.m4Locked()
	m.answers = append(m.answers, &homeDashSaveAnswer{status: status, body: body, times: max(times, 1)})
}
func (f *homeDashFixture) dropPut(n int) { f.mu.Lock(); f.m4Locked().drops = n; f.mu.Unlock() }
func (f *homeDashFixture) setPutDelay(d time.Duration) {
	f.mu.Lock()
	f.m4Locked().delay = d
	f.mu.Unlock()
}
func (f *homeDashFixture) putLog() []homeDashPut {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.m4Locked().puts)
}
func (f *homeDashFixture) dashboardDoc(id string) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	if d := f.dashboards[id]; d != nil {
		return homeDashClone(d)
	}
	return nil
}

// foreignSave edits a stored dashboard like another window or the agent would: revision and updated_at move, saved_by is set (an agent save leaves
// approved_revision alone) and every stream gets dashboard_changed. It returns the new revision.
func (f *homeDashFixture) foreignSave(t *testing.T, id, savedBy string, mutate func(doc map[string]any)) int {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	doc := f.dashboards[id]
	if doc == nil {
		t.Fatalf("no dashboard %s", id)
	}
	if mutate != nil {
		mutate(doc)
	}
	rev := homeDashInt(doc["revision"]) + 1
	doc["revision"], doc["updated_at"], doc["saved_by"] = rev, homeDashNow(), savedBy
	if savedBy != "agent" {
		doc["approved_revision"] = rev
	}
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": id, "revision": rev, "approved_revision": homeDashInt(doc["approved_revision"])}))
	return rev
}

func TestHomeDashFixtureSaveRoutes(t *testing.T) {
	f := newHomeDashFixture(t)
	call := func(method, target, body string) (int, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(method, "/api/desktop/homedash/"+target, strings.NewReader(body))
		rec := httptest.NewRecorder()
		f.ServeHTTP(rec, req)
		out := map[string]any{}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	status, made := call("POST", "dashboards", `{"document":{"name":"Test","pages":[{"id":"main","name":"Main"}]}}`)
	if status != 201 {
		t.Fatalf("create: %d %v", status, made)
	}
	doc := made["dashboard"].(map[string]any)
	id := doc["id"].(string)
	if _, ok := doc["kiosk"].(map[string]any); !ok {
		t.Fatalf("create keeps the posted document and adds the kiosk default: %v", doc)
	}
	put := func(rev int, mutate func(d map[string]any)) (int, map[string]any) {
		t.Helper()
		d := f.dashboardDoc(id)
		if mutate != nil {
			mutate(d)
		}
		raw, _ := json.Marshal(map[string]any{"revision": rev, "document": d})
		return call("PUT", "dashboard?id="+id, string(raw))
	}
	if status, out := put(1, nil); status != 200 || homeDashInt(out["dashboard"].(map[string]any)["revision"]) != 2 {
		t.Fatalf("first PUT: %d %v", status, out)
	}
	if status, out := put(1, nil); status != 409 || out["code"] != "revision_conflict" || homeDashInt(out["current_revision"]) != 2 {
		t.Fatalf("stale PUT: %d %v", status, out)
	}
	bad := func(d map[string]any) {
		p := d["pages"].([]any)[0].(map[string]any)
		p["widgets"] = []any{map[string]any{"id": "w1", "type": "switch", "variant": "tile", "lgSize": map[string]any{"w": 2}, "bindings": map[string]any{"primary": "system:cpu"}, "options": map[string]any{}, "actions": map[string]any{}}}
	}
	if status, out := put(2, bad); status != 422 || out["code"] != "invalid_document" || out["issues"].([]any)[0].(map[string]any)["code"] != "bad_json" {
		t.Fatalf("unknown field: %d %v", status, out)
	}
	draft := func(d map[string]any) {
		p := d["pages"].([]any)[0].(map[string]any)
		p["widgets"] = []any{map[string]any{"id": "w1", "type": "switch", "variant": "tile", "bindings": map[string]any{}, "options": map[string]any{}, "actions": map[string]any{}, "confirm": false}}
		p["layouts"] = map[string]any{"lg": []any{map[string]any{"id": "w1", "x": 0, "y": 0, "w": 2, "h": 1}}}
	}
	if status, out := put(2, draft); status != 422 || out["issues"].([]any)[0].(map[string]any)["code"] != "binding_required" {
		t.Fatalf("draft widget: %d %v", status, out)
	}
	d := f.dashboardDoc(id)
	draft(d)
	raw, _ := json.Marshal(map[string]any{"document": d})
	if status, out := call("POST", "dashboards/validate", string(raw)); status != 200 || out["valid"] != false || out["issues"].([]any)[0].(map[string]any)["code"] != "binding_required" {
		t.Fatalf("validate: %d %v", status, out)
	}
	if status, out := call("POST", "dashboards/validate", `{"document":{"name":"x","pages":[{"id":"a","name":"A","lgSize":1}]}}`); status != 422 {
		t.Fatalf("validate bad_json: %d %v", status, out)
	}
	// Lint codes follow validator.text and the pages check of the server: empty or missing is "required", over the limit "too_long" / "too_many".
	codeAt := func(body, path string) string {
		_, out := call("POST", "dashboards/validate", body)
		list, _ := out["issues"].([]any)
		for _, it := range list {
			if m, _ := it.(map[string]any); m["path"] == path {
				code, _ := m["code"].(string)
				return code
			}
		}
		return ""
	}
	tooLong := strings.Repeat("x", 81)
	for _, c := range []struct{ body, path, want string }{
		{`{"document":{"pages":[{"id":"a","name":"A"}]}}`, "/name", "required"},
		{`{"document":{"name":"","pages":[{"id":"a","name":"A"}]}}`, "/name", "required"},
		{`{"document":{"name":"` + tooLong + `","pages":[{"id":"a","name":"A"}]}}`, "/name", "too_long"},
		{`{"document":{"name":"x","pages":[]}}`, "/pages", "required"},
		{`{"document":{"name":"x","pages":[{"id":"a"}]}}`, "/pages/0/name", "required"},
		{`{"document":{"name":"x","pages":[{"id":"a","name":"` + tooLong[:41] + `"}]}}`, "/pages/0/name", "too_long"},
	} {
		if got := codeAt(c.body, c.path); got != c.want {
			t.Fatalf("lint %s: got %q, want %q", c.path, got, c.want)
		}
	}
	if status, out := call("POST", "dashboard/duplicate?id="+id, ``); status != 201 || out["dashboard"].(map[string]any)["name"] != "Test (copy)" || out["dashboard"].(map[string]any)["slug"] != "test-copy" {
		t.Fatalf("duplicate: %d %v", status, out)
	}
	f.answerPut(503, map[string]any{"status": "error", "code": "storage_unavailable", "message": "later"}, 1)
	if status, _ := put(2, nil); status != 503 {
		t.Fatalf("scripted answer: %d", status)
	}
	if status, _ := put(2, nil); status != 200 {
		t.Fatalf("after the scripted answer: %d", status)
	}
	if got := len(f.putLog()); got != 2 {
		t.Fatalf("put log holds the accepted saves: %d", got)
	}
	if status, out := call("GET", "entities?kind=sensor&limit=1", ""); status != 200 || len(out["entities"].([]any)) != 1 || homeDashInt(out["total"]) < 2 || homeDashInt(out["limit"]) != 1 {
		t.Fatalf("entity filter: %d %v", status, out)
	}
	if status, _ := call("GET", "entities?limit=900", ""); status != 400 {
		t.Fatalf("limit out of range: %d", status)
	}
	if rev := f.foreignSave(t, id, "agent", func(d map[string]any) { d["name"] = "Agent was here" }); rev != 4 || f.dashboardDoc(id)["name"] != "Agent was here" || homeDashInt(f.dashboardDoc(id)["approved_revision"]) != 3 {
		t.Fatalf("foreign save: %d %v", rev, f.dashboardDoc(id))
	}
}
```
- [ ] `go vet ./ui`; `go test ./ui -run '^TestHomeDashFixture' -count=1` → PASS; `go test ./ui -run '^TestDesktopHomeDash' -count=1` compiles (skips). Commit — `test(homedash): fixture save, validate, duplicate, entity paging and foreign saves (M4)`.

### Task A11: Part-1 translations and dynamic prefixes
**Files:** `ui/lang/homedash/*.json` (16), `ui/i18n_lint_test.go` (`desktopDynamicI18nPrefixes` only), `scripts/homedash-tests/i18n.test.mjs` (allowlist as needed: `SAME_WORD_KEYS` entries with reasons, F5). **Existing symbols changed:** `desktopDynamicI18nPrefixes` (var). Changed value: `homedash.view.empty_text`.
- [ ] Register the C17 prefixes (append to the list; Q10: `homedash.icon.` is one of them, its keys come with part 2).
- [ ] Keys (en / de; makers translate the other 14 for real). `{{x}}` placeholders as given.
  - **designer.** done Done/Fertig · editbar Edit tools/Bearbeitungswerkzeuge · undo Undo/Rückgängig · redo Redo/Wiederholen · undo_label Undo: {{label}}/Rückgängig: {{label}} · redo_label Redo: {{label}}/Wiederholen: {{label}} · pages Pages of this dashboard/Seiten dieses Dashboards · canvas Page {{name}}, widgets/Seite {{name}}, Widgets · palette Widgets and devices/Widgets und Geräte · inspector Properties/Eigenschaften · toggle_palette Show widgets and devices/Widgets und Geräte zeigen · toggle_inspector Show properties/Eigenschaften zeigen · close_drawer Close panel/Bereich schließen · more More actions/Weitere Aktionen · look Look…/Aussehen … · add_widget Add a widget/Widget hinzufügen · page_n Page {{n}}/Seite {{n}} · empty_title Drag widgets or devices here/Zieh Widgets oder Geräte hierher · empty_text Or choose a widget and press Enter. Paste with Ctrl+V./Oder wähl ein Widget und drück die Eingabetaste. Einfügen mit Strg+V. · frame_label {{type}}, {{name}}, column {{col}}, row {{row}}, {{w}} by {{h}}/{{type}}, {{name}}, Spalte {{col}}, Zeile {{row}}, {{w}} × {{h}} · frame_unbound no device/kein Gerät · draft_state Choose a device/Gerät auswählen · chip {{type}} · {{variant}}/{{type}} · {{variant}} (allowlist: no letters outside placeholders except separators — confirm with the test) · chip_family {{type}} · {{variant}} · {{family}}/same · edit Edit/Bearbeiten · edit_hint Edit this dashboard (Ctrl+E)/Dieses Dashboard bearbeiten (Strg+E) · elsewhere Being edited in another window/Wird in einem anderen Fenster bearbeitet · show_window Show that window/Dieses Fenster zeigen · enter_announce Edit mode. Arrow keys move the selected widget, Shift with arrows resizes it, Escape leaves./Bearbeitungsmodus. Pfeiltasten verschieben das gewählte Widget, Umschalt mit Pfeiltasten ändert die Größe, Escape beendet. · exit_announce Edit mode closed./Bearbeitungsmodus beendet. · reload_agent Updated by the agent/Vom Agenten aktualisiert · reload_other Updated in another window or browser/In einem anderen Fenster oder Browser aktualisiert · duplicated “{{name}}” created/„{{name}}“ erstellt · present_failed Full screen is not available here/Vollbild ist hier nicht verfügbar
  - **designer.announce.** moved Moved to column {{col}}, row {{row}}/Verschoben nach Spalte {{col}}, Zeile {{row}} · moved_many {{count}} widgets moved/{{count}} Widgets verschoben · resized Size {{w}} by {{h}}/Größe {{w}} × {{h}} · limit_min Smallest size for this widget/Kleinste Größe für dieses Widget · limit_max Largest size for this widget/Größte Größe für dieses Widget · edge At the edge of the grid/Am Rand des Rasters · blocked Cannot move further/Weiter geht es nicht · added {{name}} added at column {{col}}, row {{row}}/{{name}} hinzugefügt in Spalte {{col}}, Zeile {{row}} · added_many {{count}} widgets added/{{count}} Widgets hinzugefügt · added_draft {{name}} added. Choose a device in Properties./{{name}} hinzugefügt. Wähl in den Eigenschaften ein Gerät. · deleted {{name}} deleted. Undo with Ctrl+Z./{{name}} gelöscht. Rückgängig mit Strg+Z. · deleted_many {{count}} widgets deleted. Undo with Ctrl+Z./{{count}} Widgets gelöscht. Rückgängig mit Strg+Z. · copied_one Widget copied/Widget kopiert · copied_many {{count}} widgets copied/{{count}} Widgets kopiert · cut_one Widget cut/Widget ausgeschnitten · cut_many {{count}} widgets cut/{{count}} Widgets ausgeschnitten · selected {{count}} widgets selected/{{count}} Widgets ausgewählt · selection_cleared Selection cleared/Auswahl aufgehoben · undone Undone: {{label}}/Rückgängig gemacht: {{label}} · redone Redone: {{label}}/Wiederholt: {{label}} · nothing_undo Nothing to undo/Nichts rückgängig zu machen · nothing_redo Nothing to redo/Nichts zu wiederholen · resize_single Select one widget to resize it/Wähl ein einzelnes Widget, um seine Größe zu ändern · clipboard_empty The clipboard is empty/Die Zwischenablage ist leer · clipboard_bad The clipboard holds nothing HomeDash can paste/Die Zwischenablage enthält nichts, was HomeDash einfügen kann · page Page {{name}}/Seite {{name}} · rebound {{name}} now shows {{entity}}/{{name}} zeigt jetzt {{entity}} · drop_invalid {{entity}} does not fit this widget/{{entity}} passt nicht zu diesem Widget · cancelled Cancelled/Abgebrochen · limit_widgets This page already has {{max}} widgets/Diese Seite hat schon {{max}} Widgets · limit_pages A dashboard has at most {{max}} pages/Ein Dashboard hat höchstens {{max}} Seiten · last_page The last page cannot be deleted/Die letzte Seite lässt sich nicht löschen
  - **bp.** label Layout/Layout-Ansicht · lg Desktop · 12 columns/Desktop · 12 Spalten · md Tablet · 8 columns/Tablet · 8 Spalten · sm Phone · 4 columns/Smartphone · 4 Spalten · auto Automatic/Automatisch · custom Custom/Angepasst · reset Reset to automatic/Auf automatisch zurücksetzen · now_custom_md The tablet layout of this page is now custom./Das Tablet-Layout dieser Seite ist jetzt angepasst. · now_custom_sm The phone layout of this page is now custom./Das Smartphone-Layout dieser Seite ist jetzt angepasst. · now_auto_md The tablet layout follows the desktop layout again./Das Tablet-Layout folgt wieder dem Desktop-Layout. · now_auto_sm The phone layout follows the desktop layout again./Das Smartphone-Layout folgt wieder dem Desktop-Layout.
  - **save.** label Save state/Speicherstatus · saved Saved/Gespeichert · saving Saving…/Wird gespeichert … · retrying Not saved – retrying in {{seconds}} s/Nicht gespeichert – neuer Versuch in {{seconds}} s · retry_now Retry now/Jetzt erneut versuchen · invalid Not saved: {{reason}}/Nicht gespeichert: {{reason}} · undo_last Undo last change/Letzte Änderung rückgängig · conflict Changed elsewhere/Anderswo geändert · readonly Not saved: read-only/Nicht gespeichert: schreibgeschützt · gone Deleted elsewhere/Anderswo gelöscht · copy Save as a copy/Als Kopie speichern · drafts_one Saved · 1 widget has no device/Gespeichert · 1 Widget ohne Gerät · drafts_many Saved · {{count}} widgets have no device/Gespeichert · {{count}} Widgets ohne Gerät · show_draft Show/Zeigen · announce_failed Changes could not be saved. HomeDash keeps trying./Änderungen ließen sich nicht speichern. HomeDash versucht es weiter. · announce_saved_again All changes saved/Alle Änderungen gespeichert
  - **conflict.** banner This dashboard was changed elsewhere while you were editing./Dieses Dashboard wurde anderswo geändert, während Du es bearbeitet hast. · banner_agent The agent changed this dashboard while you were editing./Der Agent hat dieses Dashboard geändert, während Du es bearbeitet hast. · resolve Resolve…/Klären … · title Dashboard changed elsewhere/Dashboard anderswo geändert · body_agent The agent changed this dashboard at {{time}} while you were editing. Your changes are not saved yet./Der Agent hat dieses Dashboard um {{time}} geändert, während Du es bearbeitet hast. Deine Änderungen sind noch nicht gespeichert. · body_other This dashboard was saved somewhere else at {{time}} while you were editing. Your changes are not saved yet./Dieses Dashboard wurde um {{time}} anderswo gespeichert, während Du es bearbeitet hast. Deine Änderungen sind noch nicht gespeichert. · body_plain This dashboard was saved somewhere else while you were editing. Your changes are not saved yet./Dieses Dashboard wurde anderswo gespeichert, während Du es bearbeitet hast. Deine Änderungen sind noch nicht gespeichert. · theirs Take theirs/Deren Version übernehmen · mine Keep mine/Meine behalten · copy Save mine as a copy/Meine als Kopie speichern · later Decide later/Später entscheiden · copy_suffix  (copy)/ (Kopie) · copy_done Your version was saved as “{{name}}”./Deine Version wurde als „{{name}}“ gespeichert. · theirs_done The newer version is shown. Your changes were dropped./Die neuere Version wird gezeigt. Deine Änderungen wurden verworfen. · mine_done Your version replaced the newer one./Deine Version hat die neuere ersetzt. · gone_title Dashboard deleted elsewhere/Dashboard anderswo gelöscht · gone_body This dashboard was deleted while you were editing. Save your version as a copy to keep it./Dieses Dashboard wurde gelöscht, während Du es bearbeitet hast. Speichere Deine Version als Kopie, um sie zu behalten.
  - **leave.** title Changes are not saved yet/Änderungen sind noch nicht gespeichert · body_retrying HomeDash cannot reach the server. It keeps trying in the background while the desktop stays open./HomeDash erreicht den Server nicht. Es versucht es im Hintergrund weiter, solange der Desktop offen bleibt. · body_invalid The server did not accept the last change: {{reason}}/Der Server hat die letzte Änderung nicht angenommen: {{reason}} · body_conflict This dashboard was changed elsewhere. Decide which version to keep first./Dieses Dashboard wurde anderswo geändert. Entscheide zuerst, welche Version bleibt. · body_gone This dashboard was deleted elsewhere./Dieses Dashboard wurde anderswo gelöscht. · keep Keep editing/Weiter bearbeiten · leave Leave and keep trying/Beenden und weiter versuchen · discard Leave and discard changes/Beenden und Änderungen verwerfen · close_keep Keep window open/Fenster offen lassen · close_leave Close and keep trying/Schließen und weiter versuchen · close_discard Close and discard changes/Schließen und Änderungen verwerfen · drafts_title Widgets without a device/Widgets ohne Gerät · drafts_one 1 widget has no device yet and is not saved. It is dropped when you leave./1 Widget hat noch kein Gerät und ist nicht gespeichert. Es fällt weg, wenn Du beendest. · drafts_many {{count}} widgets have no device yet and are not saved. They are dropped when you leave./{{count}} Widgets haben noch kein Gerät und sind nicht gespeichert. Diese fallen weg, wenn Du beendest. · drafts_choose Choose devices/Geräte auswählen · drafts_leave Leave without them/Ohne sie beenden
  - **ctx.** more_info More info/Weitere Informationen · edit_widget Edit widget/Widget bearbeiten · edit Edit properties/Eigenschaften bearbeiten · duplicate Duplicate/Duplizieren · copy Copy/Kopieren · cut Cut/Ausschneiden · paste Paste/Einfügen · delete Delete/Löschen · variant Change style/Stil ändern · add_widget Add widget…/Widget hinzufügen …
  - **menu.** dashboard Dashboard/Dashboard-Menü · edit Edit/Bearbeiten · view View/Ansicht · new New…/Neu … · open Open…/Öffnen … · rename Rename…/Umbenennen … · duplicate_dashboard Duplicate dashboard/Dashboard duplizieren · delete Delete dashboard…/Dashboard löschen … · import Import…/Importieren … · export Export/Exportieren · undo Undo/Rückgängig · redo Redo/Wiederholen · cut Cut/Ausschneiden · copy Copy/Kopieren · paste Paste/Einfügen · duplicate Duplicate/Duplizieren · delete_widgets Delete/Löschen · select_all Select all/Alles auswählen · edit_mode Edit mode/Bearbeitungsmodus · layout Layout/Layout-Ansicht · fit Fit to screen/An Bildschirm anpassen · present Present/Präsentieren
  - **cmd.** move Move/Verschieben · resize Resize/Größe ändern · add Add widget/Widget hinzufügen · paste Paste/Einfügen · duplicate Duplicate/Duplizieren · delete Delete/Löschen · cut Cut/Ausschneiden · variant Change style/Stil ändern · bind Change device/Gerät ändern · unbind Clear device/Gerät entfernen · family Change family/Familie ändern · align Align/Ausrichten · title Change title/Titel ändern · icon Change icon/Symbol ändern · option Change setting/Einstellung ändern · action Change action/Aktion ändern · reset_layout Reset layout/Layout zurücksetzen · page_add Add page/Seite hinzufügen · page_rename Rename page/Seite umbenennen · page_icon Change page icon/Seitensymbol ändern · page_duplicate Duplicate page/Seite duplizieren · page_move Move page/Seite verschieben · page_delete Delete page/Seite löschen · page_compact Change compaction/Verdichtung ändern · look Change look/Aussehen ändern · grid Change grid/Raster ändern · rename Rename dashboard/Dashboard umbenennen
  - **error.** (new; title/storage/generic exist) bad_request The server did not accept the request./Der Server hat die Anfrage nicht angenommen. · invalid_id The dashboard id is not valid./Die Dashboard-ID ist ungültig. · invalid_document The dashboard has an error./Das Dashboard enthält einen Fehler. · limit_exceeded The dashboard is too large or there are too many dashboards./Das Dashboard ist zu groß oder es gibt zu viele Dashboards. · body_too_large The dashboard is too large to send./Das Dashboard ist zu groß zum Senden. · schema_too_new This dashboard comes from a newer HomeDash version./Dieses Dashboard stammt aus einer neueren HomeDash-Version. · slug_taken Another dashboard already uses this address./Ein anderes Dashboard nutzt diese Adresse schon. · dashboard_not_found The dashboard no longer exists./Das Dashboard gibt es nicht mehr. · read_only HomeDash is read-only./HomeDash ist schreibgeschützt. · desktop_readonly The desktop is read-only./Der Desktop ist schreibgeschützt. · homedash_disabled HomeDash is switched off./HomeDash ist ausgeschaltet. · storage_unavailable The dashboard storage is not available./Der Dashboard-Speicher ist nicht erreichbar. · network No connection to the server./Keine Verbindung zum Server. · csrf_check_failed The request came from an unexpected page./Die Anfrage kam von einer unerwarteten Seite. · rate_limited Too many requests. HomeDash tries again shortly./Zu viele Anfragen. HomeDash versucht es gleich noch einmal. · method_not_allowed The server does not support this action./Diese Aktion unterstützt der Server nicht. · bad_json The file is not a HomeDash dashboard./Die Datei ist kein HomeDash-Dashboard. · required A required field is empty./Ein Pflichtfeld ist leer. · too_long A text is too long./Ein Text ist zu lang. · bad_format A value has the wrong format./Ein Wert hat das falsche Format. · out_of_range A value is out of range./Ein Wert liegt außerhalb des erlaubten Bereichs. · not_allowed A value is not allowed here./Ein Wert ist hier nicht erlaubt. · control_chars A text contains control characters./Ein Text enthält Steuerzeichen. · url_not_allowed Texts may not contain web addresses./Texte dürfen keine Webadressen enthalten. · duplicate_id Two items use the same id./Zwei Elemente nutzen dieselbe ID. · unknown_type A widget type is unknown./Ein Widget-Typ ist unbekannt. · unknown_variant A widget style is unknown./Ein Widget-Stil ist unbekannt. · unknown_binding A widget has a device slot it does not support./Ein Widget hat einen Geräteplatz, den es nicht unterstützt. · binding_required A widget needs a device./Ein Widget braucht ein Gerät. · bad_entity_id A device id is not valid./Eine Geräte-ID ist ungültig. · too_many There are too many items./Es gibt zu viele Elemente. · unknown_option A widget has an unknown setting./Ein Widget hat eine unbekannte Einstellung. · bad_option A widget setting has a wrong value./Eine Widget-Einstellung hat einen falschen Wert. · page_not_found A link points to a page that does not exist./Ein Verweis zeigt auf eine Seite, die es nicht gibt. · toggle_unsupported This widget cannot switch on and off./Dieses Widget kann nicht ein- und ausschalten. · layout_missing A widget has no place in the layout./Ein Widget hat keinen Platz im Layout. · layout_duplicate A widget appears twice in the layout./Ein Widget steht doppelt im Layout. · layout_unknown_widget The layout names a widget that does not exist./Das Layout nennt ein Widget, das es nicht gibt. · layout_overlap Two widgets overlap./Zwei Widgets überlappen sich. · layout_out_of_bounds A widget lies outside the grid./Ein Widget liegt außerhalb des Rasters. · size_out_of_range A widget is too small or too large for its style./Ein Widget ist für seinen Stil zu klein oder zu groß. · document_too_large The dashboard is larger than 256 KiB./Das Dashboard ist größer als 256 KiB. · entity_unknown A device is not known (yet)./Ein Gerät ist (noch) nicht bekannt. · kind_mismatch A device does not fit its widget./Ein Gerät passt nicht zu seinem Widget. · capability_missing A device lacks a function its widget needs./Einem Gerät fehlt eine Funktion, die sein Widget braucht.
  - **widget.<type>.name** switch Switch/Schalter · light Light/Licht · climate Climate/Klima · temperature Temperature/Temperatur · sensor Sensor value/Messwert · gauge Gauge/Messanzeige · binary Contact or presence/Kontakt oder Anwesenheit · cover Cover/Beschattung · action Action/Aktion · select Choice/Auswahl · number Number/Zahlenwert · fan Fan/Ventilator · lock Lock/Schloss · media Media player/Mediaplayer · chart Chart/Diagramm · camera Camera/Kamera · status_list Status list/Statusliste · battery Battery/Batterie · room Room/Raum · clock Clock/Uhr · heading Heading/Überschrift · nav Page link/Seitenlink · weather Weather/Wetter · entity Device tile/Gerätekachel
  - **variant.<type>.<variant>** switch: pill Toggle/Umschalter, rocker Rocker/Wippe, push Push button/Taster, tile Tile/Kachel · light: slider Slider/Schieberegler, fader Vertical fader/Senkrechter Regler, dial Dial/Drehregler, tile Tile/Kachel · climate: dial Dial/Drehregler, stepper Steps/Stufen, scale Scale/Skala · temperature: numeral Number/Zahl, thermometer Thermometer scale/Thermometerskala, comfort Comfort/Komfort · sensor: numeral Number/Zahl, sparkline Sparkline/Verlaufslinie, icon Icon/Symbol · gauge: arc Arc/Bogen, needle Needle/Zeiger, bar Bar/Balken, tank Level/Füllstand · binary: indicator Indicator/Anzeige, lamp Lamp/Leuchte · cover: shutter Shutter/Rollladen, buttons Buttons/Tasten · action: button Button/Schaltfläche · select: segmented Segments/Segmente, list List/Liste · number: stepper Steps/Schritte, slider Slider/Schieberegler · fan: speed Speed/Stufe · lock: slide Slide to unlock/Zum Entsperren schieben · media: compact Compact/Kompakt, cover Cover art/Albumcover · chart: line Line/Linie, timeline Timeline/Zeitleiste · camera: frame Frame/Bild · status_list: list List/Liste · battery: glyph Battery/Batterie · room: card Card/Karte · clock: digital Digital clock/Digitaluhr, analog Analog clock/Analoguhr · heading: title Title/Titel · nav: button Button/Schaltfläche · weather: compact Compact/Kompakt, forecast Forecast/Vorhersage · entity: tile Tile/Kachel
  - **family.<id>** (F5: the part-1 selection chip shows the catalog family) glass Glass/Glas · tactile Tactile/Taktil · minimal Minimal/Minimal · instrument Instrument/Instrument. German `homedash.family.minimal` and `homedash.family.instrument` equal the English value: add both to `SAME_WORD_KEYS.de` with the reason "the German UI uses the same word"; any other locale whose real translation equals English gets an entry with its reason (R6).
  - Changed: `homedash.view.empty_text` = "Switch to edit mode to add widgets." / "Wechsle in den Bearbeitungsmodus, um Widgets hinzuzufügen."
- [ ] `npm run test:homedash` (i18n L5 PASS), `go test ./ui -run '(?i)^(TestTranslations_|TestDesktopDynamicI18nPrefixKeysExistInAllLanguages)' -count=1` → PASS. Commit — `feat(homedash): M4 designer texts in 16 locales and dynamic key families`.

**Batch A review (reader-agent, Critical/Important only):** catalog defaults vs HA template and binding kinds; pickWidgetFor fallback chain; normalise idempotency and kiosk repair; forSave never leaks client fields; solver loop termination; clipboard sanitiser (every field, caps, no unknown keys, navigate/toggle rules); command ops invertible (`apply` after/before round trip for every builder), id-keyed paths, coalesce keys; track edge cases (synthetic pointers, lost capture); fixture lint mirrors the strict decoder. security-agent (D14): clipboard decode and the fixture's strictness assumptions.

---

## Batch B — designer core

### Task B1: hd-render design mode
**Files:** `hd-render.js`, `render.test.mjs`. **Existing symbols changed:** `page` (hd-render; caller hd-view; MEDIUM), internal `apiFor`, `canFor`, `mount`, `layout`, `destroy`.
- [ ] Changes (keep the file < 650 lines):
  - `doc`, `pg`, `family` become `let`; `let mode = o.mode || 'view', forcedBp = o.bp || null, fitOverride = o.fit || null, last = null, emptyAction = o.emptyAction || null;` `const PREVIEW = Object.freeze({ lg: w => Math.max(w, E.layout.BP_MIN.lg), md: w => Math.min(768, Math.max(w, E.layout.BP_MIN.md)), sm: () => 390 });`
  - `apiFor(rec, key, modeOf, onlyId)` with `const now = typeof modeOf === 'function' ? modeOf : () => modeOf;` → `can` uses `now()`; `command` refuses unless `canFor(id, command, now())`; `moreInfo` returns false unless `now() === 'view'`. Widget apis pass `() => rec.mode`; the sheet keeps `'view'`.
  - `mount(widget)`: `rec.mode = mode`, `rec.key = E.doc.keyOf(widget) + '|' + (widget.family || family || '')`, `rec.labelledBy = rec.el.getAttribute('aria-labelledby')`; host created with `mode`; then `applyRole(rec)`. `unmount(rec)`: `io.unobserve`, destroy/remove, drop from `recs`, `byEl`, `byEntity`, `limits`.
  - `applyRole(rec)`: design → `role=option`, `aria-selected=false` (unless set), remove `aria-labelledby`, `tabindex=-1`; view → `role=group`, `aria-labelledby=rec.labelledBy`, remove `aria-selected`, `aria-label`, `tabindex` (unsupported cells same).
  - `layout()`: `bp = forcedBp || E.layout.breakpoint(box.width, bp)`; `width = forcedBp ? PREVIEW[forcedBp](box.width) : box.width`; fit `mode === 'design' ? 'scroll' : fitOverride || doc.grid.fit`; vars add `'--hd-e-pw': forcedBp ? Math.round(width) + 'px' : null`; `setAttr(pageEl, 'data-hd-preview', forcedBp)`; `last = Object.freeze({bp, cols, rows, gap, colW: m.colW, rowH: m.rowH, offX: m.offX, mode: m.mode, width})`; design: write `aria-label` from `o.describe(rec.widget, it, rec.ids[0] ? decorated(rec, rec.ids[0], null) : null)` when changed (`rec.label`); `onChange` refreshes labels of the touched recs in design.
  - Page attributes: design → `role=listbox`, `aria-multiselectable=true`, `aria-label` = `o.label || doc.name`, no `aria-labelledby`; view → today's.
  - Empty note: `renderEmpty()` builds `.hd-page-empty[data-hd-mode]` with mode-specific title/text (`designer.empty_title/_text` in design, `view.empty_*` in view) and `button.hd-page-empty-action[data-hd-cmd]` from `emptyAction`; replaced on mode/emptyAction change; removed when widgets exist.
  - `update(nd, np)`: diff by `rec.key` (unmount changed/missing, mount new), rebuild `byEntity`, `renderEmpty()`, synchronous `layout()`.
  - `preview(items, po)`:
```js
        function preview(items, po) {
            if (destroyed || !last) return 0;
            const p = po || {}, list = items || E.layout.effective(pg.layouts, last.bp, id => limits.get(id)), skip = p.skip || null;
            const rootEl = o.stage.closest ? o.stage.closest('.hd-root') : null, flip = !!p.animate && !E.core.motionReduced(rootEl); let n = 0;
            for (const it of list) {
                const rec = recs.get(it.id); if (!rec || (skip && skip.has(it.id))) continue;
                const cell = [it.x, it.y, it.w, it.h].join(','); if (cell === rec.cell) continue;
                const prev = rec.cell ? rec.cell.split(',').map(Number) : null; rec.cell = cell; n += 1;
                setVars(rec.el, { '--hd-x': it.x, '--hd-y': it.y, '--hd-w': it.w, '--hd-h': it.h });
                if (flip && prev) { setVars(rec.el, { '--hd-e-fx': Math.round((prev[0] - it.x) * (last.colW + last.gap)) + 'px', '--hd-e-fy': Math.round((prev[1] - it.y) * (last.rowH + last.gap)) + 'px' }); setAttr(rec.el, 'data-hd-flip', 'from'); flips.add(rec); flipFrame(); }
                const px = { width: Math.round(it.w * last.colW + (it.w - 1) * last.gap), height: Math.round(it.h * last.rowH + (it.h - 1) * last.gap) }, key = cell + ':' + px.width + 'x' + px.height;
                if (rec.inst && key !== rec.px) { rec.px = key; rec.inst.resize({ w: it.w, h: it.h, px }); }
            }
            setVars(pageEl, { '--hd-rows': Math.max(1, E.layout.rows(list), p.rows || 0) });
            return n;
        }
        const flips = new Set(), flipFrame = E.core.frame(() => { for (const r of flips) { r.el.removeAttribute('data-hd-flip'); setVars(r.el, { '--hd-e-fx': null, '--hd-e-fy': null }); } flips.clear(); }, { raf: o.raf, caf: o.caf });
```
  - Return adds `update, setMode(m) { mode = m; for (const r of recs.values()) { r.mode = m; if (r.inst) r.inst.setMode(m); applyRole(r); } page attributes; renderEmpty(); layout(); }, setBreakpoint(b) { forcedBp = b || null; layout(); }, setFit(f) { fitOverride = f || null; layout(); }, setEmptyAction(a) { emptyAction = a || null; renderEmpty(); }, preview, metrics: () => last, items: () => (last ? E.layout.effective(pg.layouts, last.bp, id => limits.get(id)) : []), frame: id => (recs.get(id) || {}).el || null, order: () => E.commands ? E.commands.readingOrder(items) : []` (`order` uses `E.grid.sortReading`, no dependency on later files). `destroy` cancels `flipFrame`.
- [ ] `render.test.mjs` additions (starter doc, catalog golden, `measure` 1180×700): design mode sets `role=listbox`/`option`, no frame `aria-labelledby`, `.hd-more` hidden, `can()` false and `command()` resolves `{status:'refused'}`; `update` with one changed widget keeps the other nodes (`===`) and replaces the changed one; `preview` writes only changed items and returns the count; `setBreakpoint('sm')` sets `--hd-e-pw` 390px, `data-hd-preview="sm"`, 4 columns; switching back to view restores `role=group` and `aria-labelledby`; `noStyleAttr(page.el)`.
- [ ] PASS; `TestDesktopHomeDashBrowser` (R8) still PASS. Commit — `feat(homedash): design mode, diff update and gesture preview in the page renderer`.

### Task B2: hd-view design surface
**Files:** `hd-view.js`, `view.test.mjs`. **Existing symbols changed:** `create` (hd-view; caller hd-app), `renderPage`, `select`, `renderTabs`.
- [ ] Add state `mode, forcedBp, fitOverride, describe, emptyAction`, an emitter, `toasts = el('div', {class:'hd-toasts', 'aria-hidden':'true'})` appended to the root after `live`. `renderPage` passes `mode, bp: forcedBp, fit: fitOverride, describe, emptyAction`, emits `page` with the handle, calls `o.onEntities(page.entityIds())`. `renderTabs` hides the tablist in design. Returned additions:
```js
            setMode(m) { mode = m === 'design' ? 'design' : 'view'; root.classList.toggle('hd-backdrop', mode === 'view'); stage.classList.toggle('hd-backdrop', mode === 'design'); renderTabs(); if (page) page.setMode(mode); },
            update(raw, uo) { const prev = doc; doc = uo && uo.normalised ? raw : E.doc.normalise(raw, { sizeOf: o.catalog.defaultSize, constraints: o.catalog.constraints });
                if (!doc.pages.some(p => p.id === pageId)) { pageId = doc.pages[0].id; applyRoot(); renderTabs(); renderPage(); E.widgets.applyLook(root); return; }
                const lookChanged = !prev || E.doc.keyOf(prev.look) !== E.doc.keyOf(doc.look); if (lookChanged) applyRoot(); renderTabs();
                page.update(doc, doc.pages.find(p => p.id === pageId)); if (lookChanged) E.widgets.applyLook(root); if (o.onEntities) o.onEntities(page.entityIds()); },
            select(id, so) { select(id, !!(so && so.focus)); },
            setBreakpoint(b) { forcedBp = b || null; if (page) page.setBreakpoint(forcedBp); }, setFit(f) { fitOverride = f || null; if (page) page.setFit(fitOverride); },
            setDescribe(fn) { describe = typeof fn === 'function' ? fn : null; }, setEmptyAction(a) { emptyAction = a || null; if (page) page.setEmptyAction(emptyAction); },
            page: () => page, get stage() { return stage; }, doc: () => doc, announce(text) { live.textContent = String(text); },
            toast(text, to) { const n = el('div', { class: 'hd-toast', 'data-tone': (to && to.tone) || 'info', text: String(text) }); toasts.appendChild(n); while (toasts.children.length > 3) toasts.firstElementChild.remove();
                const ms = (to && to.ms) || 4000; const h = setTimeout(() => n.remove(), ms); life.add(() => clearTimeout(h)); live.textContent = String(text); },
            on: bus.on, off: bus.off,
```
`show()` normalises with `constraints: o.catalog.constraints`. Tab clicks/keys stay view-mode only (`if (mode !== 'view') return;`).
- [ ] `view.test.mjs`: `setMode('design')` moves `hd-backdrop` from root to stage and hides `.hd-tabs`; `update` keeps the page node when the page id stays and re-renders when it disappears (`page` event fires); `toast` keeps at most three; `onEntities` called after update.
- [ ] PASS. Commit — `feat(homedash): view design mode, live update and toasts`.

### Task B3: hd-save.js (new)
**Files:** create `hd-save.js`, `save.test.mjs`; wiring step W (after `hd-commands.js`). **Existing symbols changed:** none.
- [ ] Full file (lane, the three sheets with literal keys only, the export). The first draft ended after `errorText` and described the sheets in prose (dry-run defect 5); the generation counter `gen` and the follow-up `flushAsk` are F10 and F2. Sheet keys are literal `t('homedash…')` calls: a ternary of keys inside `t()` would escape the key lint.
<!-- embed: dryrun-m4/ui/js/homedash/hd-save.js -->
```js
// HomeDash autosave lane and save sheets (frontend 5.8; M4 contract C8). lane(o) keeps one dashboard's local document in step with the server:
// set(doc) marks it dirty and arms an 800 ms debounce; one PUT {revision, document: prepare(doc)} is in flight; changes made meanwhile go out in the
// next PUT; hold(true) (a running gesture) postpones the send. Network, 401, 408, 429 and 5xx retry after 2, 4, 8, 16, 30 s, then every 30 s
// (Retry-After wins); 409 revision_conflict stops in 'conflict' with currentRevision; 404 stops in 'gone'; 403 in 'readonly' until retryNow(); other
// 4xx stop in 'invalid' until the next set(). A document over 256 KiB fails locally. every PUT with a body up to 61 440 bytes is sent with keepalive, so an open PUT survives a tab unload; flush() while a PUT is open queues one follow-up PUT right after it succeeds. conflict, overwrite and reset start a new generation: a late answer of an older PUT is ignored. The realm keeps a
// dirty lane after its window closed (hd-app) and flushes it with keepalive on pagehide.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.api) throw new Error('hd-save.js: load hd-api.js first');
    const BACKOFF = Object.freeze([2000, 4000, 8000, 16000, 30000]), DEBOUNCE_MS = 800, KEEPALIVE_MAX = 61440, DOC_MAX = 262144, STOPS = new Set(['invalid', 'conflict', 'readonly', 'gone']);
    const ERROR_CODES = Object.freeze(['bad_request', 'invalid_id', 'invalid_document', 'limit_exceeded', 'body_too_large', 'schema_too_new', 'slug_taken', 'dashboard_not_found', 'read_only', 'desktop_readonly', 'homedash_disabled', 'storage_unavailable', 'network', 'csrf_check_failed', 'rate_limited', 'method_not_allowed', 'bad_json',
        'required', 'too_long', 'bad_format', 'out_of_range', 'not_allowed', 'control_chars', 'url_not_allowed', 'duplicate_id', 'unknown_type', 'unknown_variant', 'unknown_binding', 'binding_required', 'bad_entity_id', 'too_many', 'unknown_option', 'bad_option', 'page_not_found', 'toggle_unsupported',
        'layout_missing', 'layout_duplicate', 'layout_unknown_widget', 'layout_overlap', 'layout_out_of_bounds', 'size_out_of_range', 'document_too_large', 'entity_unknown', 'kind_mismatch', 'capability_missing']);
    function utf8Bytes(s) { let n = 0; for (let i = 0; i < s.length; i++) { const c = s.charCodeAt(i); if (c < 0x80) n += 1; else if (c < 0x800) n += 2; else if (c >= 0xd800 && c <= 0xdbff && i + 1 < s.length) { n += 4; i += 1; } else n += 3; } return n; }
    function classify(err) { const s = err.status || 0, c = err.code || '';
        if (c === 'revision_conflict') return 'conflict'; if (s === 404 || c === 'dashboard_not_found') return 'gone'; if (s === 403) return 'readonly';
        if (s === 0 || s === 401 || s === 408 || s === 429 || s >= 500) return 'retry'; return 'invalid'; }
    function lane(o) {
        const api = o.api, id = o.id, tm = o.timers || {}, setT = tm.setTimeout || ((fn, ms) => setTimeout(fn, ms)), clearT = tm.clearTimeout || (h => clearTimeout(h)), now = tm.now || (() => Date.now());
        const bus = E.core.emitter(), debounce = Number.isFinite(o.debounceMs) ? o.debounceMs : DEBOUNCE_MS;
        let doc = o.doc, saved = o.doc, revision = Number(o.revision) || 0, sent = null, dirty = false, holding = false, phase = 'saved', attempt = 0, nextAt = 0, current = null, error = null, warnings = [];
        let gen = 0, flushAsk = false, timer = 0, retryTimer = 0, changedAt = 0, owner = null, disposed = false, prepare = typeof o.prepare === 'function' ? o.prepare : (d => d);
        const snapshot = () => Object.freeze({ phase, dirty, saving: !!sent, revision, attempt, nextRetryAt: nextAt, currentRevision: current, error, warnings });
        const emit = () => { if (!disposed) bus.emit('state', snapshot()); };
        function stopTimers() { clearT(timer); timer = 0; clearT(retryTimer); retryTimer = 0; nextAt = 0; }
        function schedule() { clearT(timer); timer = 0; if (disposed || !dirty || holding || sent || retryTimer || STOPS.has(phase)) return;
            phase = 'pending'; timer = setT(() => { timer = 0; send(); }, Math.max(0, debounce - (now() - changedAt))); }
        async function send() {
            if (disposed || sent || !dirty) return; clearT(timer); timer = 0;
            const body = prepare(doc), bytes = utf8Bytes(JSON.stringify(body));
            if (bytes > DOC_MAX) { phase = 'invalid'; error = Object.freeze({ code: 'limit_exceeded', status: 0, issues: [] }); emit(); return; }
            const mine = doc, g = gen; sent = mine; phase = 'saving'; emit();
            try {
                const answer = await api.saveDashboard(id, revision, body, bytes <= KEEPALIVE_MAX ? { keepalive: true } : undefined); if (disposed || g !== gen) return;
                const d = answer && answer.dashboard; revision = Number(d && d.revision) || revision + 1; saved = mine; sent = null; attempt = 0; error = null; current = null;
                warnings = Array.isArray(answer && answer.warnings) ? answer.warnings.slice(0, 50) : []; if (doc === mine) dirty = false; phase = dirty ? 'pending' : 'saved';
                bus.emit('saved', Object.freeze({ revision, doc: mine, body, warnings }));
                if (dirty && flushAsk) { flushAsk = false; send(); } else { flushAsk = false; schedule(); } emit();
            } catch (raw) {
                if (disposed || g !== gen) return; const err = E.api.normalizeError(raw), kind = classify(err); sent = null; flushAsk = false;
                error = Object.freeze({ code: err.code, status: err.status, issues: Array.isArray(err.data && err.data.issues) ? err.data.issues.slice(0, 50) : [] });
                if (kind === 'retry') { const ms = Number.isFinite(err.retryAfter) && err.retryAfter > 0 ? Math.round(err.retryAfter * 1000) : BACKOFF[Math.min(attempt, BACKOFF.length - 1)];
                    attempt += 1; phase = 'retrying'; nextAt = now() + ms; retryTimer = setT(() => { retryTimer = 0; nextAt = 0; if (holding) { phase = 'pending'; emit(); return; } send(); }, ms); }
                else if (kind === 'conflict') { phase = 'conflict'; current = Number(err.data && err.data.current_revision) || null; }
                else phase = kind;
                emit();
            }
        }
        function settle(ms) { return new Promise(resolve => { let done = false, t0 = 0, off = null;
            const check = st => { if (done || st.saving || st.phase === 'pending') return; done = true; if (off) off(); if (t0) clearT(t0); resolve(st.phase); };
            off = bus.on('state', check); if (ms > 0) t0 = setT(() => { if (!done) { done = true; off(); resolve(phase); } }, ms); check(snapshot()); }); }
        return Object.freeze({
            id, on: bus.on, off: bus.off, state: snapshot, doc: () => doc, saved: () => saved, revision: () => revision, settle,
            get disposed() { return disposed; }, get owner() { return owner; },
            set(next) { if (disposed || !next || next === doc) return; doc = next; dirty = doc !== saved; changedAt = now(); if (phase === 'invalid') phase = 'pending'; if (!dirty && !sent && !retryTimer && !STOPS.has(phase)) phase = 'saved'; schedule(); emit(); },
            hold(on) { holding = !!on; if (holding) { clearT(timer); timer = 0; } else schedule(); },
            flush(fo) { if (disposed) return Promise.resolve(phase); const ka = !!(fo && fo.keepalive); if (retryTimer) { clearT(retryTimer); retryTimer = 0; nextAt = 0; }
                if (sent) { if (dirty) flushAsk = true; } else if (dirty && !STOPS.has(phase)) send(); return settle(ka ? 0 : 5000); },
            retryNow() { if (disposed || !dirty || !['retrying', 'readonly', 'pending'].includes(phase)) return; stopTimers(); phase = 'pending'; send(); },
            conflict(rev) { if (disposed) return; gen++; sent = null; flushAsk = false; stopTimers(); phase = 'conflict'; current = Number(rev) || current; emit(); },
            overwrite(rev) { if (disposed) return; gen++; sent = null; flushAsk = false; stopTimers(); revision = Number(rev) || revision; current = null; error = null; phase = 'pending'; dirty = true; send(); },
            reset(next, rev) { if (disposed) return; gen++; sent = null; flushAsk = false; stopTimers(); doc = saved = next; revision = Number(rev) || revision; dirty = false; current = null; error = null; attempt = 0; phase = 'saved'; emit(); },
            setPrepare(fn) { prepare = typeof fn === 'function' ? fn : (d => d); }, attach(who) { owner = who; emit(); }, detach(who) { if (owner === who) owner = null; emit(); },
            dispose() { if (disposed) return; flushAsk = false; stopTimers(); bus.emit('state', Object.freeze(Object.assign({}, snapshot(), { phase: 'stopped' }))); disposed = true; }
        });
    }
    function errorText(t, error) { const e = error || {}, issue = Array.isArray(e.issues) ? e.issues.find(i => i && ERROR_CODES.includes(i.code)) : null, code = issue ? issue.code : ERROR_CODES.includes(e.code) ? e.code : '';
        return code ? t('homedash.error.' + code) : t('homedash.error.generic', { code: e.code || '?' }); }
    // Sheets: every key is a literal t('homedash...') call (a ternary between two keys would escape the key lint).
    function when(info, lang) { const v = info && info.updatedAt, d = typeof v === 'number' ? new Date(v) : typeof v === 'string' ? new Date(v) : null; if (!d || Number.isNaN(d.getTime())) return '';
        try { return new Intl.DateTimeFormat(lang, { hour: '2-digit', minute: '2-digit' }).format(d); } catch (err) { return ''; } }
    function conflictSheet(o) {
        const t = o.t, info = o.info || {}, time = when(info, o.lang), gone = !!info.gone, vars = { time, name: info.savedBy || '' };
        const body = gone ? t('homedash.conflict.gone_body', vars) : info.savedBy === 'agent' ? t('homedash.conflict.body_agent', vars) : info.savedBy ? t('homedash.conflict.body_other', vars) : t('homedash.conflict.body_plain', vars);
        const later = { id: 'later', label: t('homedash.conflict.later') }, copy = { id: 'copy', label: t('homedash.conflict.copy'), tone: 'primary', autofocus: true };
        const actions = gone ? [later, copy] : [later, { id: 'theirs', label: t('homedash.conflict.theirs') }, { id: 'mine', label: t('homedash.conflict.mine') }, copy];
        return o.sheet.open({ title: gone ? t('homedash.conflict.gone_title') : t('homedash.conflict.title'), text: [body], kind: 'conflict', actions });
    }
    function leaveSheet(o) {
        const t = o.t, close = o.mode === 'close', trying = ['retrying', 'pending', 'saving'].includes(o.phase);
        const body = trying ? t('homedash.leave.body_retrying') : o.phase === 'conflict' ? t('homedash.leave.body_conflict') : o.phase === 'gone' ? t('homedash.leave.body_gone') : t('homedash.leave.body_invalid', { reason: o.reason || '' });
        const actions = trying ? [{ id: 'keep', label: close ? t('homedash.leave.close_keep') : t('homedash.leave.keep') }, { id: 'leave', label: close ? t('homedash.leave.close_leave') : t('homedash.leave.leave'), tone: 'primary', autofocus: true }]
            : [{ id: 'keep', label: close ? t('homedash.leave.close_keep') : t('homedash.leave.keep'), tone: 'primary', autofocus: true }, { id: 'discard', label: close ? t('homedash.leave.close_discard') : t('homedash.leave.discard'), tone: 'danger' }];
        return o.sheet.open({ title: t('homedash.leave.title'), text: [body], kind: 'leave', actions });
    }
    function draftsSheet(o) {
        const t = o.t;
        return o.sheet.open({ title: t('homedash.leave.drafts_title'), text: [o.count === 1 ? t('homedash.leave.drafts_one') : t('homedash.leave.drafts_many', { count: o.count })], kind: 'leave',
            actions: [{ id: 'leave', label: t('homedash.leave.drafts_leave') }, { id: 'choose', label: t('homedash.leave.drafts_choose'), tone: 'primary', autofocus: true }] });
    }
    E.save = Object.freeze({ BACKOFF, DEBOUNCE_MS, KEEPALIVE_MAX, DOC_MAX, ERROR_CODES, lane, classify, utf8Bytes, errorText, conflictSheet, leaveSheet, draftsSheet });
})();
```
- [ ] `scripts/homedash-tests/save.test.mjs` (full; 18 tests with the fake clock: debounce, single flight, follow-up after `flush` during an open PUT with keepalive, keepalive size rule, backoff 2/4/8/16/30/30, `retryAfter`, 409, 404, 403, 422, local 256 KiB limit, late answers after `reset` and `conflict`, settle, dispose, and the sheet actions of both modes for every phase):
<!-- embed: dryrun-m4/scripts/homedash-tests/save.test.mjs -->
```js
import assert from 'node:assert/strict'; import { loadEngine, APP_ORDER, eq } from './harness.mjs'; import { fakeClock } from './live-fakes.mjs';
const FILES = APP_ORDER.slice(0, APP_ORDER.indexOf('hd-save.js') + 1);
const err = (status, body, extra) => Object.assign(new Error('HTTP ' + status), { status, body: body || {} }, extra);
// A fake api that plays a script: each entry is {ok, revision}, {fail: Error}, or {hold} (a deferred the test settles by hand).
function setup(script, opts) {
  const { E } = loadEngine(FILES), clock = fakeClock(), calls = [], holds = [], plan = script.slice();
  const api = { saveDashboard(id, rev, doc, extra) { calls.push({ id, rev, doc, extra, at: clock.now() }); const s = plan.shift() || { ok: true };
    if (s.hold) return new Promise((resolve, reject) => { holds.push({ resolve: v => resolve(v || { dashboard: { revision: rev + 1 }, warnings: [] }), reject: e => reject(E.api.normalizeError(e)) }); });
    if (s.fail) return Promise.reject(E.api.normalizeError(s.fail)); return Promise.resolve({ dashboard: { revision: s.revision || rev + 1 }, warnings: s.warnings || [] }); } };
  const lane = E.save.lane(Object.assign({ api, id: 'd_aaaaaaaaaaaa', revision: 1, doc: { n: 0 }, timers: { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now } }, opts));
  const phases = []; lane.on('state', s => phases.push(s.phase));
  return { E, lane, clock, calls, holds, phases, api };
}
const big = n => ({ blob: 'x'.repeat(n) });
export const tests = [
  ['save: three sets inside 800 ms become one PUT 800 ms after the last, with prepare(doc) as the body', async () => { const { lane, clock, calls } = setup([], { prepare: d => ({ wrapped: d.n }) });
    lane.set({ n: 1 }); await clock.advance(200); lane.set({ n: 2 }); await clock.advance(300); lane.set({ n: 3 }); eq(clock.delays(), [800]); assert.equal(lane.state().phase, 'pending'); assert.equal(lane.state().dirty, true); await clock.advance(799); assert.equal(calls.length, 0);
    await clock.advance(1); assert.equal(calls.length, 1); eq(calls[0].doc, { wrapped: 3 }); assert.equal(calls[0].rev, 1); await clock.flush(); assert.equal(lane.state().phase, 'saved'); assert.equal(lane.revision(), 2); assert.equal(lane.state().dirty, false); eq(lane.saved(), { n: 3 }); }],
  ['save: a set during a PUT goes out right after it with the new revision (re-arm after success); a saved event carries the body', async () => { const { lane, clock, calls, holds } = setup([{ hold: true }]), saved = []; lane.on('saved', s => saved.push([s.revision, s.body.n]));
    lane.set({ n: 1 }); await clock.advance(800); assert.equal(calls.length, 1); assert.equal(lane.state().saving, true); assert.equal(lane.state().phase, 'saving'); lane.set({ n: 2 }); await clock.advance(2000); assert.equal(calls.length, 1, 'single flight');
    holds[0].resolve(); await clock.flush(); assert.equal(lane.state().phase, 'pending', 'the newer change is still to be sent'); assert.equal(lane.state().dirty, true); await clock.advance(800); assert.equal(calls.length, 2); assert.equal(calls[1].rev, 2); eq(calls[1].doc, { n: 2 }); await clock.flush(); assert.equal(lane.state().phase, 'saved'); eq(saved, [[2, 1], [3, 2]]); }],
  ['save: hold(true) postpones the send, hold(false) arms it again', async () => { const { lane, clock, calls } = setup([]); lane.set({ n: 1 }); lane.hold(true); await clock.advance(5000); assert.equal(calls.length, 0); lane.hold(false); await clock.advance(800); assert.equal(calls.length, 1);
    await clock.flush(); lane.hold(true); lane.set({ n: 2 }); await clock.advance(2000); assert.equal(calls.length, 1); lane.hold(false); await clock.advance(800); assert.equal(calls.length, 2); }],
  ['save: network errors retry after 2, 4, 8, 16, 30, 30 s and recover; Retry-After wins', async () => { const down = { fail: err(0, {}) }, { lane, clock, calls, phases } = setup([down, down, down, down, down, down, { ok: true }]);
    lane.set({ n: 1 }); await clock.advance(800); await clock.flush(); const seen = [];
    for (let i = 0; i < 6; i++) { assert.equal(lane.state().phase, 'retrying'); seen.push(clock.delays()[0]); assert.equal(lane.state().nextRetryAt - clock.now(), seen[i]); await clock.advance(seen[i]); await clock.flush(); }
    eq(seen, [2000, 4000, 8000, 16000, 30000, 30000]); assert.equal(calls.length, 7); assert.equal(lane.state().phase, 'saved'); assert.equal(lane.state().attempt, 0); assert.ok(phases.includes('retrying'));
    const r = setup([{ fail: err(429, { retry_after_ms: 3000 }) }, { ok: true }]); r.lane.set({ n: 1 }); await r.clock.advance(800); await r.clock.flush(); eq(r.clock.delays(), [3000]); assert.equal(r.lane.state().error.status, 429);
    const t = setup([{ fail: err(503, {}, { retryAfter: 7 }) }]); t.lane.set({ n: 1 }); await t.clock.advance(800); await t.clock.flush(); eq(t.clock.delays(), [7000]);
    for (const status of [401, 408, 500, 502]) { const x = setup([{ fail: err(status, {}) }]); x.lane.set({ n: 1 }); await x.clock.advance(800); await x.clock.flush(); assert.equal(x.lane.state().phase, 'retrying', String(status)); } }],
  ['save: a retry sends the newest document, retryNow skips the wait and a hold during a retry waits', async () => { const { lane, clock, calls } = setup([{ fail: err(0, {}) }, { ok: true }]);
    lane.set({ n: 1 }); await clock.advance(800); await clock.flush(); lane.set({ n: 2 }); assert.equal(lane.state().phase, 'retrying'); lane.retryNow(); await clock.flush(); assert.equal(calls.length, 2); eq(calls[1].doc, { n: 2 }); assert.equal(lane.state().phase, 'saved');
    const h = setup([{ fail: err(0, {}) }, { ok: true }]); h.lane.set({ n: 1 }); await h.clock.advance(800); await h.clock.flush(); h.lane.hold(true); await h.clock.advance(2000); await h.clock.flush(); assert.equal(h.calls.length, 1); assert.equal(h.lane.state().phase, 'pending'); h.lane.hold(false); await h.clock.advance(800); assert.equal(h.calls.length, 2); }],
  ['save: 409 revision_conflict stops in conflict with current_revision; overwrite(rev) sends that revision; no PUT in between', async () => { const { lane, clock, calls } = setup([{ fail: err(409, { code: 'revision_conflict', current_revision: 9 }) }, { ok: true }]);
    lane.set({ n: 1 }); await clock.advance(800); await clock.flush(); assert.equal(lane.state().phase, 'conflict'); assert.equal(lane.state().currentRevision, 9); lane.set({ n: 2 }); await clock.advance(60000); assert.equal(calls.length, 1); assert.equal(lane.state().phase, 'conflict');
    lane.overwrite(9); await clock.flush(); assert.equal(calls.length, 2); assert.equal(calls[1].rev, 9); eq(calls[1].doc, { n: 2 }); assert.equal(lane.state().phase, 'saved'); assert.equal(lane.revision(), 10);
    const c = setup([]); c.lane.set({ n: 1 }); c.lane.conflict(7); assert.equal(c.lane.state().phase, 'conflict'); assert.equal(c.lane.state().currentRevision, 7); await c.clock.advance(5000); assert.equal(c.calls.length, 0); }],
  ['save: 404 is gone, 403 is readonly until retryNow, other 4xx are invalid until the next set', async () => { const g = setup([{ fail: err(404, { code: 'dashboard_not_found' }) }]); g.lane.set({ n: 1 }); await g.clock.advance(800); await g.clock.flush(); assert.equal(g.lane.state().phase, 'gone'); await g.clock.advance(60000); assert.equal(g.calls.length, 1);
    const { lane, clock, calls } = setup([{ fail: err(403, { code: 'desktop_readonly' }) }, { ok: true }]); lane.set({ n: 1 }); await clock.advance(800); await clock.flush(); assert.equal(lane.state().phase, 'readonly'); await clock.advance(60000); assert.equal(calls.length, 1); lane.retryNow(); await clock.flush(); assert.equal(calls.length, 2); assert.equal(lane.state().phase, 'saved');
    const issues = [{ path: '/pages/0/widgets/0/bindings/primary', code: 'binding_required', message: 'x' }], i = setup([{ fail: err(422, { code: 'invalid_document', issues }) }, { ok: true }]); i.lane.set({ n: 1 }); await i.clock.advance(800); await i.clock.flush();
    assert.equal(i.lane.state().phase, 'invalid'); eq(i.lane.state().error.issues, issues); assert.equal(i.E.save.errorText(k => k, i.lane.state().error), 'homedash.error.binding_required'); await i.clock.advance(60000); assert.equal(i.calls.length, 1);
    i.lane.set({ n: 2 }); assert.equal(i.lane.state().phase, 'pending'); await i.clock.advance(800); await i.clock.flush(); assert.equal(i.calls.length, 2); assert.equal(i.lane.state().phase, 'saved');
    for (const [status, code] of [[400, 'bad_request'], [413, 'body_too_large'], [422, 'bad_json'], [422, 'limit_exceeded']]) { const x = setup([{ fail: err(status, { code }) }]); x.lane.set({ n: 1 }); await x.clock.advance(800); await x.clock.flush(); assert.equal(x.lane.state().phase, 'invalid', code); assert.equal(x.lane.state().error.code, code); } }],
  ['save: errorText maps known codes and falls back to generic', () => { const { E } = setup([]), t = (k, v) => k + (v ? JSON.stringify(v) : '');
    assert.equal(E.save.errorText(t, { code: 'limit_exceeded' }), 'homedash.error.limit_exceeded'); assert.equal(E.save.errorText(t, { code: 'weird', issues: [{ code: 'layout_overlap' }] }), 'homedash.error.layout_overlap'); assert.equal(E.save.errorText(t, { code: 'weird', issues: [{ code: 'nope' }] }), 'homedash.error.generic{"code":"weird"}'); assert.equal(E.save.errorText(t, null), 'homedash.error.generic{"code":"?"}');
    assert.ok(E.save.ERROR_CODES.includes('bad_json') && E.save.ERROR_CODES.includes('network')); }],
  ['save: flush({keepalive}) uses keepalive only up to 61 440 bytes; a body over 256 KiB fails locally without a request', async () => { const a = setup([]); a.lane.set(big(1000)); a.lane.flush({ keepalive: true }); await a.clock.flush(); assert.equal(a.calls[0].extra && a.calls[0].extra.keepalive, true);
    const b = setup([]); b.lane.set(big(70 * 1024)); b.lane.flush({ keepalive: true }); await b.clock.flush(); assert.equal(b.calls.length, 1); assert.equal(b.calls[0].extra, undefined);
    const edge = setup([]); edge.lane.set(big(61440 - '{"blob":""}'.length)); assert.equal(E_len(edge.lane.doc()), 61440); edge.lane.flush({ keepalive: true }); await edge.clock.flush(); assert.equal(edge.calls[0].extra.keepalive, true);
    const over = setup([]); over.lane.set(big(61440 - '{"blob":""}'.length + 1)); over.lane.flush({ keepalive: true }); await over.clock.flush(); assert.equal(over.calls[0].extra, undefined);
    const mb = setup([]); mb.lane.set(big(30000)); mb.lane.set({ blob: 'é'.repeat(31000) }); mb.lane.flush({ keepalive: true }); await mb.clock.flush(); assert.equal(mb.calls[0].extra, undefined, 'bytes, not characters: 62 000 bytes of é');
    const c = setup([]); c.lane.set(big(300 * 1024)); await c.clock.advance(800); await c.clock.flush(); assert.equal(c.calls.length, 0); assert.equal(c.lane.state().phase, 'invalid'); assert.equal(c.lane.state().error.code, 'limit_exceeded'); c.lane.set({ n: 1 }); await c.clock.advance(800); await c.clock.flush(); assert.equal(c.calls.length, 1);
    const n = setup([]); n.lane.set(big(262144 - '{"blob":""}'.length)); await n.clock.advance(800); await n.clock.flush(); assert.equal(n.calls.length, 1, 'exactly 256 KiB is allowed'); const m = setup([]); m.lane.set(big(262144 - '{"blob":""}'.length + 1)); await m.clock.advance(800); await m.clock.flush(); assert.equal(m.calls.length, 0); }],
  ['save: settle resolves on the settled phase and times out; flush of a clean lane resolves at once', async () => { const { lane, clock, holds } = setup([{ hold: true }]); assert.equal(await lane.flush(), 'saved'); lane.set({ n: 1 }); let got = null; lane.flush().then(p => { got = p; }); await clock.flush(); assert.equal(got, null);
    holds[0].resolve(); await clock.flush(); assert.equal(got, 'saved'); const w = setup([{ hold: true }]); w.lane.set({ n: 1 }); let to = null; w.lane.settle(1000).then(p => { to = p; }); await w.clock.advance(1000); await w.clock.flush(); assert.equal(to, 'saving', 'the debounce fired and the PUT is still open'); }],
  ['save: reset makes the lane clean, dispose stops timers and ignores an answer that arrives later', async () => { const a = setup([]); a.lane.set({ n: 1 }); a.lane.reset({ n: 9 }, 5); assert.equal(a.lane.state().phase, 'saved'); assert.equal(a.lane.state().dirty, false); assert.equal(a.lane.revision(), 5); await a.clock.advance(5000); assert.equal(a.calls.length, 0);
    const { lane, clock, calls, holds } = setup([{ hold: true }]), seen = []; lane.on('state', s => seen.push(s.phase)); lane.set({ n: 1 }); await clock.advance(800); assert.equal(calls.length, 1); lane.dispose(); assert.equal(lane.disposed, true); assert.equal(seen.at(-1), 'stopped'); holds[0].resolve(); await clock.flush(); assert.equal(lane.revision(), 1, 'no state change after dispose'); assert.equal(seen.at(-1), 'stopped');
    lane.set({ n: 2 }); await clock.advance(5000); assert.equal(calls.length, 1); assert.equal(await lane.flush(), lane.state().phase); lane.dispose();
    const r = setup([{ fail: err(0, {}) }]); r.lane.set({ n: 1 }); await r.clock.advance(800); await r.clock.flush(); assert.equal(r.clock.delays().length, 1); r.lane.dispose(); eq(r.clock.delays(), []); }],
  ['save: reset() while a PUT is open ignores the late answer of the open PUT (generation counter)', async () => { const { lane, clock, holds } = setup([{ hold: true }]); lane.set({ n: 1 }); await clock.advance(800); assert.equal(lane.state().saving, true); lane.reset({ n: 9 }, 7); assert.equal(lane.revision(), 7); assert.equal(lane.state().saving, false, 'reset drops the open PUT');
    holds[0].resolve(); await clock.flush(); assert.equal(lane.revision(), 7, 'the late answer must not move the revision after a reset'); eq(lane.saved(), { n: 9 }); }],
  ['save: every small PUT carries keepalive, a body over 61 440 bytes does not', async () => { const a = setup([]); a.lane.set({ n: 1 }); await a.clock.advance(800); await a.clock.flush(); assert.equal(a.calls[0].extra.keepalive, true, 'an ordinary debounced PUT');
    const b = setup([]); b.lane.set(big(70 * 1024)); await b.clock.advance(800); await b.clock.flush(); assert.equal(b.calls.length, 1); assert.equal(b.calls[0].extra, undefined); }],
  ['save: flush({keepalive}) during an open PUT sends one follow-up PUT right after it succeeds, with keepalive', async () => { const { lane, clock, calls, holds } = setup([{ hold: true }]);
    lane.set({ n: 1 }); await clock.advance(800); assert.equal(calls.length, 1); lane.set({ n: 2 }); lane.flush({ keepalive: true }); lane.flush(); await clock.flush(); assert.equal(calls.length, 1, 'single flight: nothing goes out while the first PUT is open');
    holds[0].resolve(); await clock.flush(); assert.equal(calls.length, 2, 'the follow-up goes out at once, without the debounce'); assert.equal(calls[1].rev, 2); eq(calls[1].doc, { n: 2 }); assert.equal(calls[1].extra.keepalive, true); assert.equal(calls[1].at, calls[0].at, 'no 800 ms wait');
    await clock.flush(); assert.equal(lane.state().phase, 'saved'); assert.equal(lane.revision(), 3); await clock.advance(5000); assert.equal(calls.length, 2, 'exactly one follow-up');
    const q = setup([{ hold: true }]); q.lane.set({ n: 1 }); await q.clock.advance(800); q.lane.flush(); q.holds[0].resolve(); await q.clock.flush(); assert.equal(q.calls.length, 1, 'a flush with nothing newer asks for no follow-up'); assert.equal(q.lane.state().phase, 'saved'); }],
  ['save: a failed PUT drops the flush request, a flush while a PUT is open does not outlive reset', async () => { const f = setup([{ hold: true }, { ok: true }]); f.lane.set({ n: 1 }); await f.clock.advance(800); f.lane.set({ n: 2 }); f.lane.flush({ keepalive: true }); f.holds[0].reject(err(503, {})); await f.clock.flush();
    assert.equal(f.lane.state().phase, 'retrying'); assert.equal(f.calls.length, 1); await f.clock.advance(2000); await f.clock.flush(); assert.equal(f.calls.length, 2);
    const r = setup([{ hold: true }]); r.lane.set({ n: 1 }); await r.clock.advance(800); r.lane.set({ n: 2 }); r.lane.flush(); r.lane.reset({ n: 9 }, 7); r.holds[0].resolve(); await r.clock.flush(); assert.equal(r.calls.length, 1); assert.equal(r.lane.revision(), 7); eq(r.lane.saved(), { n: 9 }); }],
  ['save: conflict() while a PUT is open ignores the late answer', async () => { const { lane, clock, holds } = setup([{ hold: true }]); lane.set({ n: 1 }); await clock.advance(800); lane.conflict(5); assert.equal(lane.state().phase, 'conflict'); holds[0].resolve(); await clock.flush(); assert.equal(lane.state().phase, 'conflict'); assert.equal(lane.revision(), 1); }],
  ['save: the lane keeps working after setPrepare, attach and detach; set of the same or an empty document does nothing', async () => { const { lane, clock, calls } = setup([]); lane.setPrepare(d => ({ p: d.n })); lane.attach('win-1'); assert.equal(lane.owner, 'win-1'); lane.detach('win-2'); assert.equal(lane.owner, 'win-1'); lane.detach('win-1'); assert.equal(lane.owner, null);
    const d = lane.doc(); lane.set(d); lane.set(null); eq(clock.delays(), []); lane.set({ n: 4 }); await clock.advance(800); eq(calls[0].doc, { p: 4 }); lane.setPrepare(null); }],
  ['save: the sheets build the plan\'s actions in order and resolve what the sheet resolves', async () => { const { E } = setup([]), got = [], t = (k, v) => k + (v ? JSON.stringify(v) : ''), sheet = { open: spec => { got.push(spec); return Promise.resolve(spec.actions[spec.actions.length - 1].id); } };
    assert.equal(await E.save.conflictSheet({ sheet, t, lang: 'en', info: { revision: 4, savedBy: 'agent', updatedAt: Date.UTC(2026, 10, 26, 10, 5) } }), 'copy'); eq(got[0].actions.map(a => a.id), ['later', 'theirs', 'mine', 'copy']); assert.equal(got[0].kind, 'conflict'); assert.equal(got[0].actions[3].autofocus, true);
    await E.save.conflictSheet({ sheet, t, lang: 'en', info: { gone: true } }); eq(got[1].actions.map(a => a.id), ['later', 'copy']);
    for (const phase of ['retrying', 'pending', 'saving']) { await E.save.leaveSheet({ sheet, t, phase, reason: '', mode: 'exit' }); eq(got.at(-1).actions.map(a => a.id), ['keep', 'leave'], phase); }
    await E.save.leaveSheet({ sheet, t, phase: 'retrying', mode: 'close' }); eq(got.at(-1).actions.map(a => a.label), ['homedash.leave.close_keep', 'homedash.leave.close_leave']);
    for (const phase of ['invalid', 'conflict', 'gone']) { await E.save.leaveSheet({ sheet, t, phase, reason: 'x', mode: 'exit' }); eq(got.at(-1).actions.map(a => a.id), ['keep', 'discard'], phase); }
    const rows = (mode, phase) => { E.save.leaveSheet({ sheet, t, phase, reason: 'x', mode }); return got.at(-1).actions.map(a => a.id + ':' + a.label + ':' + (a.tone || '')); };
    for (const phase of ['retrying', 'pending', 'saving']) eq(rows('close', phase), ['keep:homedash.leave.close_keep:', 'leave:homedash.leave.close_leave:primary'], 'close ' + phase);
    for (const phase of ['invalid', 'conflict', 'gone']) eq(rows('close', phase), ['keep:homedash.leave.close_keep:primary', 'discard:homedash.leave.close_discard:danger'], 'close ' + phase);
    for (const phase of ['retrying', 'pending', 'saving']) eq(rows('exit', phase), ['keep:homedash.leave.keep:', 'leave:homedash.leave.leave:primary'], 'exit ' + phase);
    for (const phase of ['invalid', 'conflict', 'gone']) eq(rows('exit', phase), ['keep:homedash.leave.keep:primary', 'discard:homedash.leave.discard:danger'], 'exit ' + phase);
    await E.save.draftsSheet({ sheet, t, count: 2 }); eq(got.at(-1).actions.map(a => a.id), ['leave', 'choose']); assert.ok(got.every(s => typeof s.title === 'string' && s.text.every(x => typeof x === 'string'))); }]
];
function E_len(v) { return new TextEncoder().encode(JSON.stringify(v)).length; }
```
- [ ] Wiring step W. PASS. Commit — `feat(homedash): autosave lane with retry, conflicts and save sheets`.

### Task B4: hd-gestures.js (new)
**Files:** create `hd-gestures.js`, `gestures.test.mjs`; wiring step W (after `hd-save.js`). **Existing symbols changed:** none.
- [ ] Maths (full):
```js
    const EDGE_PX = 48, SCROLL_MAX = 16, EDGES = Object.freeze(['n', 'ne', 'e', 'se', 's', 'sw', 'w', 'nw']);
    function guides(items, moving, skip, cols) {
        if (!moving.length) return []; const B = E.grid.bbox(moving), others = items.filter(i => !skip.has(i.id)), out = [];
        for (const [axis, at, side, of] of [['x', B.x, 'start', o => o.x], ['x', B.x + B.w, 'end', o => o.x + o.w], ['y', B.y, 'start', o => o.y], ['y', B.y + B.h, 'end', o => o.y + o.h]]) {
            if (at === 0 || (axis === 'x' && at === cols)) continue; if (others.some(o => of(o) === at)) out.push({ axis, at, side }); }
        return out;
    }
    function scrollStep(p, lo, hi, edge, max) { const e = edge || EDGE_PX, m = max || SCROLL_MAX; if (!(hi - lo > 2 * e)) return 0;
        if (p < lo + e) return -Math.ceil(m * Math.min(1, (lo + e - p) / e)); if (p > hi - e) return Math.ceil(m * Math.min(1, (p - (hi - e)) / e)); return 0; }
    const pxOf = (m, i) => ({ left: m.offX + m.gap + i.x * (m.colW + m.gap), top: m.gap + i.y * (m.rowH + m.gap), width: i.w * m.colW + (i.w - 1) * m.gap, height: i.h * m.rowH + (i.h - 1) * m.gap });
    const rectOf = (a, b) => ({ left: Math.min(a.x, b.x), top: Math.min(a.y, b.y), width: Math.abs(a.x - b.x), height: Math.abs(a.y - b.y) });
    function hits(items, m, r) { return items.filter(i => { const p = pxOf(m, i); return p.left < r.left + r.width && r.left < p.left + p.width && p.top < r.top + r.height && r.top < p.top + p.height; }).map(i => i.id); }
    const math = Object.freeze({
        cellOf: (m, x, y) => ({ x: Math.round((x - m.offX - m.gap) / (m.colW + m.gap)), y: Math.round((y - m.gap) / (m.rowH + m.gap)) }),
        cellUnder: (m, x, y) => ({ x: Math.floor((x - m.offX - m.gap / 2) / (m.colW + m.gap)), y: Math.floor((y - m.gap / 2) / (m.rowH + m.gap)) }),
        pxOf, cellDelta: (m, dx, dy) => ({ dx: Math.round(dx / (m.colW + m.gap)), dy: Math.round(dy / (m.rowH + m.gap)) }),
        itemAt: (items, c) => items.find(i => c.x >= i.x && c.x < i.x + i.w && c.y >= i.y && c.y < i.y + i.h) || null, guides, scrollStep, rectOf, hits
    });
```
- [ ] Controller `create(o)` (essential structure; ≤ 650 lines total):
  - State `g` (active gesture) with `{kind, track, items (start layout), idSet, m (frozen metrics), pageRect, stageRect, scroll0 {x,y}, start {x,y} (client), grab, offset, pointer, lastKey, next, bp, cols, compact}`; `loop = E.core.frame(tick, {raf, caf})`; layer helpers `ensureLayer()` (re-appends `.hd-edit-layer` to the current `page.el`), pooled `ghost(n)`, `guideEls`, `marqueeEl`, `selEl` (8 handles), `avatarEl`.
  - `local(p)` = `{x: p.x - g.pageRect.left + (stage.scrollLeft - g.scroll0.x), y: p.y - g.pageRect.top + (stage.scrollTop - g.scroll0.y)}` (scroll read at the top of `tick`, before writes).
  - Stage `pointerdown` (delegated, ignored when `g`, `button > 0`, `defaultPrevented` or `ctx().readonly`): `.hd-handle` → `startResize`; `.hd-page > .hd-widget` → `startMove`; inside `.hd-page` but not a control → `startMarquee`. Each uses `E.input.track(ev, {threshold: THRESHOLD_CANVAS, holdMs: touch && !selected ? HOLD_TOUCH_MS : 0, capture: stage, win, …timers})`; armed touch long-press adds a non-passive `touchmove` preventer on the target until the end.
  - `startMove`: press selects (`o.select([id], {focus:true})` unless modifier; modifier adds when not selected, toggles on tap when selected); on start: `o.hold(true)`, `root[data-hd-gesture=move]`, frames `data-hd-dragging`, ghosts; `tick`: target = `cellOf(m, local.x − grab.x, local.y − grab.y) − offset`; drag vars `--hd-e-dx/dy` = client delta + scroll delta on every frame; on cell change `next = E.grid.moveItems(items, ids, target, cols, {compact})`, `page.preview(next, {skip: idSet, animate:true, rows: rows(next) + 1})`, ghosts at moved items, `math.guides(next, moved, idSet, cols)`; auto-scroll `scrollStep` on both axes, writes `stage.scrollTop/Left` after the cell work and re-schedules the loop while non-zero.
  - `startResize` (single selection): `d = cellDelta(m, local − startLocal)`; `next = E.grid.resizeItem(items, id, edge, d, cols, ctx().C(id), {compact})`; `data-hd-limit` on the handle when the clamp bit (announce once via `o.describeLimit('min'|'max')`); `.hd-sel` follows the previewed rect; guides.
  - `startMarquee`: tap without drag → `o.select([])` (unless Shift); drag → `.hd-marquee` vars from `rectOf(startLocal, local)`; hits → `o.select(ids, {live:true})` (Shift adds to the start selection) on change; commit `marquee`.
  - `finish(how)`: clear ghosts/guides/marquee/limit, `data-hd-dragging`, root gesture attr; `how === 'up'` and `next` differs → `o.commit(kind, payload)` then settle FLIP for dragged frames (from `start px + drag delta` to the final px; skipped under reduced motion); else `page.preview(null)`; clear drag vars; `o.hold(false)`.
  - `dragFrom(ev, payload)`: `track` with `THRESHOLD_PALETTE` (touch: `TOUCH_LIST` + touchmove guard); on start: avatar `.hd-drop-avatar` in root with `payload.avatar()`, root rect read once; `tick`: avatar `--hd-e-ax/ay`; over the stage (inside `stageRect`): entity payload over an item → `catalog.accepts(type, 'primary', entity)` → frame `data-hd-drop=rebind|invalid`, avatar same, `preview(null)`; else add mode with a temp id (`E.core.uid('hd-new')`) at the first entry's size, pointer-centred target, `moveItems` on `items + temp`, preview without temp, ghost at temp (`invalid` when `page.widgets.length + n > 120`); outside → avatar `out`, `preview(null)`. Up → commit `add {entries, at, bp}` or `rebind {id, binding:'primary', entity}` or announce `drop_invalid`; tap → `payload.onTap()`.
  - `showSelection(id, chip)`, `reset()` (re-attach layer after a page re-render), `cancel()` (`g.track.cancel()`), `active()` (`g ? g.kind : null`), `destroy()`.
- [ ] `gestures.test.mjs`: `cellOf`/`pxOf` round trip for metrics `{colW:85, rowH:85, gap:12, offX:0}`; `cellUnder` on a gap point; `guides` emits at most four, skips x 0 and x = cols; `scrollStep(10, 0, 600)` = −13, `(590, 0, 600)` = 13, middle 0, tiny stage 0; `hits` intersection. Controller smoke (fake DOM, win = document, fake page handle with `metrics()`, `items()`, `preview()`, `frame()`, `el`): pointerdown on a frame, moves 4 px then 100 px right → after the rAF `preview` called with the frame moved one cell, pointerup → `commit('move', …)` with `next` x+1; Escape path via `cancel()` → `preview(null)`, no commit.
- [ ] Wiring step W. PASS. Commit — `feat(homedash): designer gestures for move, resize, marquee and drops`.

### Task B5: hd-editbar.js and hd-designer.js (new)
**Files:** create `hd-editbar.js`, `hd-designer.js`, `designer.test.mjs`; wiring step W (`hd-editbar.js` after `hd-gestures.js`, `hd-designer.js` before `hd-app.js`). **Existing symbols changed:** none.
- [ ] `hd-editbar.js` — `create({t, onCommand(name, arg, ev)})` → `{el, slot(name), update(s), focus(name), destroy()}`. Builds the C15 nodes (`.hd-eb-group[data-hd-group=start]`: Done (check icon + label), sep, Undo, Redo; `[data-hd-slot=pages]` with `.hd-ptabs`; `[data-hd-group=end]`: `[data-hd-slot=bp]` (`.hd-bp` with three `.hd-bp-opt` using `t('homedash.bp.' + bp)` labels and device icons, `.hd-bp-chip`, `.hd-bp-reset`), `.hd-save`, Look (hidden unless `s.hasLook`), More (`aria-haspopup=menu`), two drawer toggles (hidden unless `s.drawers`)). `update(s)`: undo/redo `aria-disabled` + labels (`designer.undo_label` when a label exists); tabs rebuilt only when `[id,name]` signature changes (roving tabindex, `aria-controls` = page id); bp `aria-checked`/tabindex; chip `data-state` + text, hidden for lg; reset visible when custom; save phase (`pending` shown as `saving`), icon by phase (`check`, `sync`, `cloud-off`, `alert`), text (C17 save keys; retry seconds = `max(1, ceil((nextRetryAt − now)/1000))`), action (`retry_save` / `undo` / `resolve_conflict` / `show_draft`); toggles `aria-expanded`. Clicks: `[data-hd-cmd]` (ignored when `aria-disabled`), `.hd-ptab` → `onCommand('page', id)`, `.hd-bp-opt` → `onCommand('layout_' + bp)`; More click calls `ev.stopPropagation()` before `onCommand('more', null, ev)`. Keys: tabs and bp use Left/Right/Home/End with focus following.
- [ ] `hd-designer.js` — `create(o)` (C4). Essential code:
```js
        const root = o.root, view = o.view, t = o.t, catalog = o.catalog, lane = o.lane, C = E.commands, life = E.core.bag(), bus = E.core.emitter(), history = E.history.create({ limit: 100 });
        let doc = o.doc, pageId = view.pageId(), bp = 'lg', selection = [], focusId = null, conflict = null, drawers = false, panel = '', lastLive = '', destroyed = false;
        const NBSP = String.fromCodePoint(0xa0), sheet = E.sheet.create(root), bar = E.editbar.create({ t, onCommand: (name, arg, ev) => command(name, arg, ev) });
        const page = () => doc.pages.find(p => p.id === pageId) || doc.pages[0], widget = id => page().widgets.find(w => w.id === id) || null;
        const constraints = id => { const w = widget(id); return w ? catalog.constraints(w) : {}; }, items = () => E.layout.effective(page().layouts, bp, constraints);
        const typeName = w => t('homedash.widget.' + w.type + '.name'), variantName = w => t('homedash.variant.' + w.type + '.' + w.variant);
        function nameOf(w, d) { if (!w) return ''; const id = w.bindings && w.bindings.primary, v = id ? o.store.view(id) : null; return w.title || (v && v.name) || typeName(w); }
        function announce(text) { if (!text) return; const s = text === lastLive ? text + NBSP : text; lastLive = s; view.announce(s); }
        function render() {
            const act = root.ownerDocument.activeElement, inCanvas = !!(act && view.stage.contains(act)), fid = inCanvas && act.closest('[data-hd-wid]') ? act.closest('[data-hd-wid]').getAttribute('data-hd-wid') : null;
            view.update(doc, { normalised: true }); sync();
            if (inCanvas && !act.isConnected) { const pg = view.page(), f = (fid && pg.frame(fid)) || (focusId && pg.frame(focusId)) || pg.el; if (f && f.focus) f.focus({ preventScroll: true }); }
        }
        function sync() {
            const pg = view.page(); if (!pg) return; const order = C.readingOrder(items()), drafts = E.doc.drafts(doc, catalog);
            selection = selection.filter(id => order.includes(id));
            const anchor = (focusId && order.includes(focusId) && focusId) || selection[selection.length - 1] || order[0] || null;
            for (const id of order) { const f = pg.frame(id); if (!f) continue; E.core.setAttr(f, 'aria-selected', selection.includes(id) ? 'true' : 'false'); E.core.setAttr(f, 'tabindex', id === anchor ? '0' : '-1'); draftBadge(f, drafts.has(id)); }
            E.core.setAttr(pg.el, 'tabindex', order.length ? '-1' : '0');
            const one = selection.length === 1 ? widget(selection[0]) : null;
            gestures.showSelection(one ? one.id : null, one ? (one.family ? t('homedash.designer.chip_family', { type: typeName(one), variant: variantName(one), family: t('homedash.family.' + one.family) }) : t('homedash.designer.chip', { type: typeName(one), variant: variantName(one) })) : '');
            emit();
        }
        function exec(cmd) {
            if (!cmd || destroyed) return false;
            if (cmd.refused) { announce(cmd.refused === 'last_page' ? t('homedash.designer.announce.last_page') : t('homedash.designer.announce.limit_widgets', { max: C.LIMITS.pageWidgets })); return false; }
            if (cmd.limit) { announce(limitText(cmd.limit)); return false; }
            const before = doc; let next; try { next = E.doc.apply(doc, cmd.ops, 'after'); } catch (err) { console.warn('HomeDash: command failed', err); return false; }
            doc = next; history.push(cmd); lane.set(doc); if (cmd.page) { pageId = cmd.page; view.select(pageId); }
            render(); if (cmd.select) select(cmd.select, { focus: true }); if (cmd.materialised) announce(t(cmd.materialised === 'md' ? 'homedash.bp.now_custom_md' : 'homedash.bp.now_custom_sm')); else announceCmd(cmd, before);
            return true;
        }
        function undoRedo(dir) {
            const r = dir === 'undo' ? history.undo(doc) : history.redo(doc);
            if (!r) { announce(t(dir === 'undo' ? 'homedash.designer.announce.nothing_undo' : 'homedash.designer.announce.nothing_redo')); return; }
            doc = r.doc; lane.set(doc); if (!doc.pages.some(p => p.id === pageId)) pageId = doc.pages[0].id; render(); select((r.command.meta.ids || []).filter(id => widget(id)), { focus: false });
            announce(t(dir === 'undo' ? 'homedash.designer.announce.undone' : 'homedash.designer.announce.redone', { label: t(r.command.label) }));
        }
```
Add this helper block to the `create` closure before `render()` (F1). The sketch above calls every one of these names; `currentState()` is the C4 state builder, written in the same closure. The block was run against the real `E.core.el`, `E.core.setAttr` and `E.icons.icon` (`overlay/check_designer_helpers.mjs`):
<!-- embed: dryrun-m4/overlay/designer-helpers.js -->
```js
        // Helpers of render()/sync()/exec() (all in the create() closure, before the first render). currentState() is the C4 state builder.
        const ANNOUNCE = Object.freeze({
            moved: 'homedash.designer.announce.moved', moved_many: 'homedash.designer.announce.moved_many',
            resized: 'homedash.designer.announce.resized', added: 'homedash.designer.announce.added',
            added_many: 'homedash.designer.announce.added_many', deleted: 'homedash.designer.announce.deleted',
            deleted_many: 'homedash.designer.announce.deleted_many',
            now_auto_md: 'homedash.bp.now_auto_md', now_auto_sm: 'homedash.bp.now_auto_sm'
        });
        function emit() { if (!destroyed) bus.emit('state', currentState()); }
        function draftBadge(frame, on) {
            const old = frame.querySelector('.hd-draft-badge');
            if (on && !old) {
                const badge = E.core.el('span', { class: 'hd-draft-badge' });
                badge.appendChild(E.icons.icon('plug'));
                badge.appendChild(E.core.el('span', { class: 'hd-draft-text', text: t('homedash.designer.draft_state') }));
                frame.appendChild(badge);
            } else if (!on && old) old.remove();
            E.core.setAttr(frame, 'data-hd-draft', on ? 'true' : null);
        }
        // One literal t() call per key: a ternary of keys inside t() would escape the key lint (R4).
        function limitText(limit) {
            return limit === 'edge' ? t('homedash.designer.announce.edge')
                : limit === 'min' ? t('homedash.designer.announce.limit_min')
                : limit === 'max' ? t('homedash.designer.announce.limit_max')
                : t('homedash.designer.announce.blocked');
        }
        function announceCmd(cmd) {
            const a = cmd && cmd.announce, key = a ? ANNOUNCE[a.key] : null; // literal map: no dynamic i18n key
            if (!key) return;
            const w = widget(a.id);
            announce(t(key, Object.assign({}, a, { name: w ? nameOf(w) : '' })));
        }
        function describe(w, item) {
            const i = item || { x: 0, y: 0, w: 1, h: 1 };
            const base = t('homedash.designer.frame_label', { type: typeName(w), name: nameOf(w), col: i.x + 1, row: i.y + 1, w: i.w, h: i.h });
            return E.doc.drafts(doc, catalog).has(w.id) ? base + ', ' + t('homedash.designer.frame_unbound') : base;
        }
```
Construction order (F1): `const gestures = E.gestures.create({...})` is declared in the setup before the first `render()` (and before `view.on('page', …)`), because `sync()` reads it on every render, even for an empty page; the `commit` and `hold` callbacks only close over function declarations of the closure, which are hoisted.
  - `select(ids, so)`: set `selection` (primary last), `focusId` on `so.focus` + `focusFrame`; `so.announce` → `announce.selected`; `live` (marquee) only updates attributes.
  - `copy()`: `E.clipboard.encode(E.doc.forSave(doc, {catalog}), pageId, sel)` → `o.clipboard.write` → `copied_one/_many`; `cut()`: copy + `exec(C.remove(doc, {pageId, ids, label:'homedash.cmd.cut', kind:'cut'}))` with `cut_*` announce; `paste()`: `E.clipboard.decode(o.clipboard.read(), {catalog, pages: new Set(doc.pages.map(p => p.id))})` → empty → `clipboard_empty`/`clipboard_bad`; else `exec(C.paste(doc, {pageId, bp, entries, startY: bottom of the selection in lg or the page end, C: constraints, limitsOf: catalog.constraints}))`.
  - `addWidgets(entries, ao)` → `exec(C.add(doc, {pageId, bp, entries, at: ao.at, startY, C: constraints, limitsOf: catalog.constraints}))`, scroll the first frame into view; drafts announce `added_draft`.
  - `onKey(ev)` root listener: skip `defaultPrevented`; `canvas = target.closest('.hd-stage') && !target.closest('.hd-sheet-layer')`; `k = C.keymap(ev, {canvas, typing: E.core.isTyping(ev.target)})`; `handled = handle(k, ev)`; `if (handled) ev.preventDefault()`. `handle` implements C14 (Tab ends return false so the browser leaves the canvas; nudge/resize build `C.nudge`/`C.resizeStep` with `now: Date.now()`; Alt+Arrow `C.spatial`; Escape ladder; Enter → inspector `focusFirst` when present).
  - `contextMenu(ev)`: `.hd-ptab` → `E.ddialogs.pageMenu?.(D, id)`; frame → select it, items `edit` (Enter), `duplicate` (Ctrl+D), sep, `copy`, `cut`, sep, `variant` submenu (registered variants, current `checked` + icon `check`, disabled when < 2), sep, `delete` (danger, Del); empty canvas → `paste` (disabled when `!o.clipboard.has()`), `add_widget`; keyboard-triggered (0,0) anchors at the target rect + 8 px; palette/inspector/edit-bar targets → null.
  - `run(action, arg)` / `handles(action)` for `E.designer.ACTIONS`; `command(name, arg, ev)` from the edit bar (`more` → `o.ui.menu(rect.left, rect.bottom, moreItems())` with `rename/duplicate_dashboard/import/export/delete_dashboard` via `o.request`, available ones only; `page` → `setPage`; `layout_*` → `setBreakpoint`).
  - `setBreakpoint(b)`: `bp = b; view.setBreakpoint(b); selection kept; render()`. `setPage(id)`: cancel gesture, clear selection, `view.select(id)`, `gestures.reset()`.
  - Lane hookup: `lane.on('state', st => { transitions → announce (C16); conflict from a 409 opens the conflict sheet once; editbar.update(state()); emit(); })`; retry ticker (1 s timeout chain while `retrying`).
  - `reconcile(info)` per C7 (deferred while `lane.state().saving` via `lane.settle(0)`; the dirty branch too: `lane.conflict(revision)` is only called after the open PUT settled, and the lane ignores a late answer after `conflict`/`overwrite`/`reset`, F10); clean reload uses `o.api.dashboard(id)` → `E.doc.normalise` → `replace(next, {revision, clearHistory:true})` → `lane.reset(next, revision)` → toast. `resolveConflict()`: `E.save.conflictSheet` → `theirs` (GET + replace + reset + `theirs_done`), `mine` (`lane.overwrite(conflict.revision || lane.state().currentRevision)`, `mine_done` on save), `copy` (`api.createDocument(copyDoc)` → `o.openCopy(id)`), `later`.
  - Drawers: `ResizeObserver` on root → `drawers = root.clientWidth < DRAWER_BELOW`; `data-hd-drawers`, `data-hd-panel`; scrim click closes; opening moves focus into the panel, closing returns it to the toggle.
  - Setup in `create`, in this order (F1): `root[data-hd-mode=design]`, insert `bar.el` as the first child, the conflict bar after it, panel shells (`.hd-palette`/`.hd-inspector` only when E.palette/E.inspector exist; then `E.palette.create(...)`, `E.inspector.create(...)`), `view.setMode('design')`, then `const gestures = E.gestures.create(...)` with the `commit` mapping (`move`/`resize` → `exec(C.layout(doc, {pageId, bp, next, C: constraints, ids, kind, label: kind === 'resize' ? 'homedash.cmd.resize' : 'homedash.cmd.move', announce}))`, `marquee` → `select`, `add` → `addWidgets`, `rebind` → `exec(C.bind(...))` + `rebound` announce) and `hold: on => lane.hold(on)`; only now `view.setDescribe(describe)`, `view.setEmptyAction({cmd:'add_widget', label: t('homedash.designer.add_widget')})`, `view.setBreakpoint('lg')`, `view.on('page', () => gestures.reset())`, the first `render()`; initial selection `[o.widgetId]` when given; `announce(enter_announce)`; focus the selected or first frame.
  - Tab upgrade (amendment 1, C5): as the last setup step, once the designer object `D` (the frozen API `create` returns) exists, call `E.ddialogs?.enhanceTabs?.(D, bar.slot('pages'))` once. The enhancer is idempotent per `D`; it reads `D.doc()`, `D.state()`, `D.on` and `D.exec`, which is why it takes `D` and not the mount api. There is no second call site (no inspector fallback).
  - `prepareExit()`: cancel gesture; `history.seal()`; `await lane.flush()` (≤ 5 s); drafts → `draftsSheet` (`choose` → select first draft, open inspector Data, return 'keep'); phase not saved → `leaveSheet({mode:'exit'})` mapping keep/leave/discard.
  - `destroy()`: cancel gesture, destroy palette/inspector/editbar/sheet, remove chrome, `view.setMode('view')`, `view.setDescribe(null)`, `view.setEmptyAction(null)`, `view.setBreakpoint(null)`, root attrs removed, `life.dispose()`, `announce(exit_announce)`.
- [ ] `designer.test.mjs` (mount through `E.mount` with the app test's contract server extended by `PUT dashboard` → revision+1; fake storage; `measure` 1180×700; keys dispatched on the root with `target` = a frame): enter edit → root `data-hd-mode=design`, `.hd-editbar` first child, frames `role=option`, one `tabindex=0`; ArrowRight on the selected frame → frame `--hd-x` +1, live region "…column 2…"; second ArrowRight within 800 ms → one history entry; Ctrl+Z → back, live "Undone: …"; Ctrl+C then Ctrl+V → one more widget, selected, PUT after 800 ms contains it and `revision` = previous; Delete → removed, live contains "Undo with Ctrl+Z"; adding a widget via `D.addWidgets([E.commands.entry(cat,'switch','pill')])` → `data-hd-draft`, PUT body lacks it; with the first widget selected, `D.addWidgets` without `at` puts the new widget's lg `y` at or below that widget's bottom row (I6), with nothing selected at the first free place from row 0; `contextMenu` on a frame lists `variant` with the current variant checked; every consumed key had `defaultPrevented` true; `noStyleAttr(root)`; entering edit on an empty page renders without throwing (`sync()` and the helper block run on every render); a draft frame carries `.hd-draft-badge` and `data-hd-draft`, the badge goes once a device is bound; `announceCmd` of an unknown key says nothing.
- [ ] Wiring step W (both files). PASS. Commit — `feat(homedash): edit bar and designer controller`.

### Task B6: hd-app edit surface
**Files:** `hd-app.js`, `app.test.mjs`. **Existing symbols changed:** `realmFor` (lanes, clipboard, pagehide), `mount` (options, `set`, `show`, `reconcile`, `remove`, `setReadonly`, `dispose`, return surface) — HIGH by reach (every HomeDash window); keep view-mode behaviour identical.
- [ ] `realmFor(base, transport, timers, storage)`: `realm.lanes = new Map()`, `realm.clipboard = E.clipboard.channel({storage: storage || safeLocalStorage(), win: typeof window !== 'undefined' ? window : null})` (`safeLocalStorage` returns null on exception), `pagehide` listener when `window.addEventListener` exists (flush dirty lanes with keepalive).
- [ ] `mount`: `ownerId = E.core.uid('hd-win')` reused for `store.watch` and `store.docs.presence(ownerId, {focus: () => o.ui && o.ui.focusWindow && o.ui.focusWindow()})` (released in `life`); `hook('lock', …)` and `hook('doc', …)` (view mode: newer cached revision → `view.update(cached.doc)`); `show()` → `store.docs.put(d.id, d)`; view created with `onEntities: ids => watch.set(ids)` and `setEmptyAction({cmd:'edit', label: t('homedash.designer.edit')})` when `canEdit`; `reconcile` per C7; `remove` → `store.docs.drop(id)` and lane dispose; `setReadonly` per C2; root listeners: Ctrl/Cmd+E (C14) and `[data-hd-cmd]` clicks outside `.hd-editbar` → `run`.
- [ ] Functions `editReason`, `laneFor(id, revision, doc)` (registers `saved` → `store.docs.put(id, Object.assign({}, ev.body, {revision: ev.revision}))` and refreshes `doc.revision`; disposes a saved detached lane), `enterEdit`, `exitEdit`, `beforeClose` (flush, `settle(1500)`, then `leaveSheet({mode:'close'})`: leave → true, lane kept; discard → lane disposed, true; keep → false), `run` (C2 table; unknown actions forwarded to `designer.run` or `E.ddialogs.run(ctx, …)`), `actions()`, `contextMenu` (design → designer; view → `more_info` for frames whose `.hd-more` is visible, `edit_widget` when editable), `duplicateDashboard` (flush when editing, `api.duplicateDashboard`, exit edit, `realm.refresh`, `open(new)`, `view.toast(duplicated)`), `deleteDashboard` (engine `E.sheet.create(root).confirm` with the existing `homedash.delete.*` keys; exit edit forced; `remove(summary)`), `present`, `openCopy(id)` (refresh, open, enterEdit), `snapshot()` (`E.widgets.paletteSnapshot(root, catalog.paletteTokens())` while connected, else the last value). `set()` adds `next.edit = editState(next)` (C2). Return adds `enterEdit, exitEdit, run, contextMenu, beforeClose`; `dispose` force-exits edit (lane kept when dirty), releases presence/lock.
- [ ] `app.test.mjs`: contract server gains `PUT dashboard` (revision check, 409 `current_revision`), `POST dashboard/duplicate`; tests: (1) enterEdit → `edit.mode` design, lock held; a second mount on the same dashboard has `editElsewhere` true and `enterEdit` false; `run('show_editor')` calls the first mount's `ui.focusWindow`; (2) a command → PUT after 800 ms with `revision` 1 → `edit.phase` saved, revision 2; the second mount (view) re-renders from the docs cache without a GET; (3) foreign save (server revision bump + `dashboard_changed`) while clean → GET, toast, history cleared; while dirty → `edit.phase` conflict, no PUT; (4) PUT answering 409 → conflict; `designer.run('resolve_conflict')` choosing mine → PUT with `current_revision`; (5) `beforeClose` clean → true; with 503 retrying → leave sheet shown, `leave` → true and the lane keeps retrying after `dispose`; (6) `setReadonly(true)` while editing → view mode; (7) view-mode `contextMenu` on a light frame lists `more_info` and `edit_widget`.
- [ ] Revision 2 rules for B6 (F2, F4, F13):
  - `dispose`: when the lane is dirty, call `lane.flush({keepalive: true})` before detaching (nothing is awaited; the realm keeps the lane, it keeps retrying and `pagehide` flushes it again). Lane PUTs ≤ 61 440 bytes already carry keepalive, so the open PUT survives a tab unload, and `flush` queues one follow-up PUT for a change made while a PUT is open (residual window: C8).
  - `laneFor`'s `saved` handler passes `Object.assign({}, ev.body, {revision: ev.revision})` to `store.docs.put` (C7: `put` reads `doc.revision`). The `doc` hook of a mount ignores an event whose revision is not newer than the one the mount shows (a window's own save emits an equal revision).
  - `actions()` reports `available: !!E.ddialogs` for `rename`, `import`, `export`, `look` and `add_page` (their owner is part 2, C2), so the host never offers items that do nothing. `new` stays available (the host falls back to the New ▸ submenu); `run` forwards an unknown action to `E.ddialogs.run` only when it exists and answers `false` otherwise.
  - Tests (added to `app.test.mjs`): `actions().rename.available` is false while `E.ddialogs` is absent and `new_starter` stays available; `dispose` of a dirty, editing mount sends one PUT with `keepalive` (fake api) and the lane stays registered in the realm; a `saved` event stores a document whose `revision` is the new one and a second mount ignores an equal-revision `doc` event.
- [ ] Revision 3 rules for B6 (Q12):
  - `actions().import = {available: !!E.ddialogs, enabled: !readonly}`: import needs no open dashboard. `rename`, `look`, `add_page` stay `enabled: dashboard open && !readonly`, `export` `enabled: dashboard open`.
  - `run` hands `E.ddialogs.run` a ctx that carries `sheetHost: o.ui && o.ui.sheetHost` next to `root, t, lang, api, store, catalog, designer, mount, state, open, home, sheet, toast, refresh`; the ctx `sheet` is the mount's own (engine root) sheet, and `E.ddialogs` builds its sheet on `sheetHost()` while `state().mode !== 'dashboard'`.
  - Test (`app.test.mjs`): `actions().import` is available and enabled with no dashboard open, disabled when read-only; a ctx with `sheetHost` reaches `E.ddialogs.run` (fake `E.ddialogs`).
- [ ] PASS. Commit — `feat(homedash): mount edit surface with realm save lanes, lock and reconcile rules`.

### Task B7: Host wiring
**Files:** `ui/js/desktop/core/menus-and-routing.js`, `ui/js/desktop/bundles/main.bundle.js` (generated), `ui/js/desktop/apps/homedash.js`, `scripts/homedash-tests/host.test.mjs`, `ui/desktop_homedash_wiring_test.go`. **Existing symbols changed:** `renderAppContent` homedash branch (MEDIUM: desktop dispatch), `render`, `menus`, `paint` (homedash.js), `TestHomeDashWiring`.
- [ ] Dispatch block (C3) exactly:
```js
        if (appId === 'homedash' && window.HomeDashApp) {
            return window.HomeDashApp.render(contentEl(id), id, Object.assign({}, context || {}, {
                esc, t, api, setWindowMenus, clearWindowMenus, showContextMenu, wireContextMenuBoundary, readonly: desktopReadonly(),
                updateWindowContext: (windowId, patch) => { updateWindowContext(windowId, patch); scheduleSessionPersist(); },
                setWindowBeforeClose: (winId, handler) => { const win = state.windows.get(winId); if (win) win.beforeClose = handler; },
                focusWindow: winId => { if (state.windows.has(winId)) focusWindow(winId); }
            }));
        }
```
Then `npm run build:ui`.
- [ ] `homedash.js`: mount options (C3); `toDesktopItems(items)` as below (F3: the desktop's `showContextMenu` tests `item.separator`, not `type`, and does not render `checked`); `wireContextMenuBoundary`; `setWindowBeforeClose` (and `null` on dispose); `menus(s)` rebuilt on a signature of `mode, desktopReadonly, busy, templates, edit.mode, edit.bp, edit.fit, undo/redo labels, actions` with the three C3 menus (explicit ids, `shortcutHint`, `hidden`/`disabled`/`checked` from `edit.actions`; an item whose `edit.actions[name].available === false` is left out (F4: `rename`, `import`, `export`, `look` and `add_page` until part 2 ships `E.ddialogs`); `new` item when available, else `New ▸` with starter/empty/HA); `paint(s)`: `data-hdx-edit`, `.hdx-edit`, `.hdx-elsewhere`, `.hdx-save` (save keys) per C3; click actions `edit` → `inst.run('edit')`, `show-editor` → `inst.run('show_editor')`, `retry-save` → `inst.run('retry_save')`; `askDelete(summary)` (home cards) keeps the host's own `sheet.confirm` (`homedash.delete.*`, sheet on `.hdx-app`) and then `inst.remove(summary)` (Q6); `inst.run('delete_dashboard')` is used only for the open dashboard by the Dashboard menu and the designer's more menu. Q12: the home-mode menu lists Import… (`inst.run('import')`, hidden when `edit.actions.import.available` is false, disabled when read-only) and the mount option `ui.sheetHost` returns `root`.
<!-- embed: dryrun-m4/overlay/to-desktop-items.js -->
```js
    // Engine menu items -> items of the desktop's showContextMenu. menus-and-routing.js tests `item.separator` (not `type`) and does not render
    // `checked`: a checked item carries the `check` icon. `icon` is the engine icon id; the maker maps the C3 ids onto desktop icon names that exist
    // (iconMarkup draws `fallback`, the first letter of the label, when a name is unknown).
    function toDesktopItems(items) {
        return (items || []).map(it => (it.separator ? { separator: true } : {
            id: it.id, label: it.label, icon: it.icon, fallback: it.label ? String(it.label).charAt(0) : '', shortcut: it.shortcut || it.shortcutHint || '',
            disabled: !!it.disabled, disabledHint: it.disabledHint, danger: !!it.danger, items: it.items ? toDesktopItems(it.items) : undefined,
            action: () => { if (typeof it.run === 'function') it.run(); }
        }));
    }
```
- [ ] `host.test.mjs`: render the host with a fake ctx (stub engine mount via the real engine files or a minimal `HomeDashEngine.mount` stub returning a scripted state) → `setWindowMenus` receives three menus with ids `homedash-dashboard|edit|view`, Edit items disabled in view mode, `shortcutHint` set and no `shortcut`; `wireContextMenuBoundary` and `setWindowBeforeClose` called; `toDesktopItems` maps separators to `{separator: true}` and keeps submenus (the embedded block was run against exactly these cases, `overlay/check_to_desktop_items.mjs`); with `E.ddialogs` absent the Dashboard menu has no Rename, Import or Export item and Edit has no Look item; a home card delete asks through the host sheet and calls `inst.remove`, never `run('delete_dashboard')`; in home mode the menu has Import…; files still pass `lintJs` and < 1100 lines.
- [ ] `TestHomeDashWiring`: engine list = C1 part-1 order; add a check that the substring between `appId === 'homedash' && window.HomeDashApp` and the next `if (appId ===` contains `setWindowBeforeClose`, `showContextMenu`, `wireContextMenuBoundary` and `focusWindow` in both `menus-and-routing.js` and `main.bundle.js`.
- [ ] `npm run test:homedash`, `npm run check:ui`, `go test ./ui -run '(?i)^TestHomeDashWiring$' -count=1`, `TestDesktopHomeDashBrowser` and `TestDesktopHomeDashControlsBrowser` (R8) PASS. Commit — `feat(homedash): desktop menus, context menus, close guard and edit chips`.

**Batch B review (reader-agent, Critical/Important only):** no command from design mode (api mode getter, more button); focus never lands on body after a render, undo, delete or sheet; every consumed key prevented, Tab ends not prevented; one rAF per frame and preview writes only changed items; gesture cancel restores; lane single flight, retry, 409, keepalive size, realm lifetime and pagehide; reconcile race with an in-flight PUT; lock release on dispose; host menus have ids and hints only; view mode unchanged (both M2/M3 browser tests green). security-agent (D14): keepalive save path and snapshot tokens.

---

## Handed to part 2 (with the contract items each relies on)
- **Batch C — palette, entity browser, inspector:** `hd-palette.js` (+ samples) and `hd-inspector.js` (+ entity picker) in the C1 slot; C20 interfaces; C4 designer API; C5 builders (`entry`, `add`, `bind`, `setWidget`, `variant`, `applyFamily`, `align`, `remove`); C9 `entities`/`entity` (single kind per request, ≤ 500, visible-rows subscription through a separate `store.watch` owner, ≤ ~20); C10 `paletteGroups`, `pickWidgetFor`, `accepts`, `optionSpecs`, `canToggle`; C12 `dragFrom` payload; D5 thresholds; drawers under `DRAWER_BELOW`; thumbs via `E.widgets.host({mode:'thumb'})`; text fields `text:true` + `seal()`; icon picker `E.icons.choices()`; keys `palette.*`, `inspector.*`, `option.*`, `category.*`, `family.*`, `device_class.*`, `widget.<type>.desc`.
- **Batch D — look editor, pages, breakpoints polish, import/export, first run, dashboard sheets:** `E.ddialogs` (C20) owning `new, rename, import, export, look, add_page` and `pageMenu(D, id)`/`enhanceTabs(D, slot)` on the C15 `.hd-ptabs` nodes (called from `hd-designer.js`, amendment 1) (add, inline rename 1–40, icon, duplicate, move, delete with confirm, 5 px drag reorder, Alt+Shift+Left/Right) via C5 page builders; look fields via `setDoc` (palette objects in the server shape with lower-case accent, integer `surface_hue` 0–359, `scheme`; snapshot written by the lane's `prepare`); export = `{schema_version, dashboard}` of `E.doc.forSave` minus id, revision, approved_revision, saved_by, updated_at; import ≤ 512 KiB before reading, envelope or bare, `validateDocument` (200 valid:false or 422 `bad_json` → issues sheet with `E.save.errorText`), then `createDocument` and open, warnings listed; first run per D1 in `homedash-home.js`; `homedash.view.empty_*` already updated.
- **Batch E — V14 chrome CSS, lab sheets, gates E-S/E-T/E-F:** every selector and variable of C15 in `desktop-app-homedash.css` (host.test rules, ≤ 900 lines, `--vd-mat-*` only, no new `--hd-*` tokens), coarse-pointer handles, reduced motion, no layout shift on selection, focus visible everywhere, sheet kinds `conflict`/`leave`/`form`, toasts, drawers, avatar; probes incl. selection/handles on every family canvas.
- **Batch F — tests and docs:** `TestDesktopHomeDashDesignerBrowser` (D13 list; C18 fixture controls; shard check), `HOMEDASH_M4_SHOTS=1` sheets; live-handler edit-and-save step in `internal/server/desktop_homedash_live_browser_test.go` (server accepts the designer's `forSave` body); docs (engine guide C1–C20, apps guide, visual system), `m4-handover.md`, German preview pack.

## Questions for the lead (defaults are implemented unless answered otherwise)
- **Q1 Drafts** (reading 1): unbound required widgets are stripped from saves and warned on leave/close. Default: yes.
- **Q2 `defaults.capability`** (reading 2, D2 shape `+capability?`): needed for dimmable lights. Default: yes; select/number defaults list/slider.
- **Q3 Editor runtime variables `--hd-e-*`** written by JS are not tokens; no freeze regeneration. Default: yes.
- **Q4 Host bar hidden in design mode.** Default: yes.
- **Q5 Design mode uses scroll metrics for `fit_screen` documents.** Default: yes.
- **Q6 Realm-scoped lanes** (retry continues after window close; keepalive on `pagehide`; the close guard offers "Close and keep trying"). Default: yes.
- **Q7 Part 1 ships a working minimal bp control (segmented, chip, reset) and select-only page tabs;** part 2 upgrades the same nodes. Default: yes.
- **Q8 `state.edit`** instead of a new meaning for `state.mode`. Default: yes.
- **Q9 `.hd-more` hidden outside view mode** (frame stays non-inert per the M0 test). Default: yes.
- **Q10 Dashboard delete confirm moves into the engine** (works in design mode, same `homedash.delete.*` keys). Default: yes.
- **Q11 `delete_dashboard`/`duplicate_dashboard` implemented in part 1** (no new sheet), `rename/new/open-sheet/import/export` in part 2. Default: yes.

---

# Part 2 — Batches C–F

> Part 2 of `docs/superpowers/plans/2026-11-26-homedash-m4-designer.md`. **Revision 2 (2026-11-26):** the code of this part was applied to the dry-run copy of main `99bf6a699` (part 1 revision 2 code applied) and run; the embedded files are the tested ones (see "Revision 2" below). It is written against part 1's "Engine contract (frozen for M4)" (C1–C20) as changed by part 1 **revision 3** (`m4-plan-part1-v3.md`): amendment 1 accepted (`enhanceTabs(D, slot)` called from `hd-designer.js`), `ui.sheetHost()`, import enabled without an open dashboard, home-card delete through the host sheet, `homedash.icon.` prefix. Family keys stay in part 1 (A11); host context-menu items use `{separator: true}` and `checked` is drawn as the `check` icon. Nothing here renames or reshapes C1–C20.

## Part-2 readings (decided inside part-2 files; defaults of the questions at the end)

1. **Files in the C1 slot**, in this order between `hd-editbar.js` and `hd-designer.js`: `hd-form.js` (E.form: shared chrome controls), `hd-samples.js` (E.samples: sample states and thumbs), `hd-palette.js` (E.palette), `hd-entity-picker.js` (E.entityPicker), `hd-inspector.js` (E.inspector), `hd-look.js` (E.look), `hd-designer-dialogs.js` (E.ddialogs). The names match the wiring test's `[a-z/-]+` pattern.
2. **Namespaces.** C20 fixes `create(...)` and what it returns. Part 2 also adds pure helpers on its own namespaces (`E.palette.model`, `E.inspector.model`, `E.look.model`, plus `E.ddialogs.exportFile/importDocument/newChoices/nameError/targetIndex`) for Node tests. No part-1 code reads them.
3. **Canvas overlays use `--hd-focus`.** This covers the selection outline, focus ring, handles, ghost, guides and marquee. `--hd-focus` is the dashboard accent that `E.widgets.applyLook` already holds at 3:1 on cards and canvas. With palette `theme` it equals `--vd-accent` unless that accent would fail. Chrome (bar, panels, sheets) uses `--vd-*` and `--vd-mat-*` only.
4. **One accent.** Only `.hd-eb-done` and the empty page's primary action use an accent fill. Switches (on), segmented controls, tabs and the chosen variant/family cards are monochrome (`--vd-text` ink), and selected rows use `--vd-mat-selected`.
5. **Muted text.** On the bar and the side panels, muted text is `--vd-theme-muted`: the materials test checks it on bar and side. `--vd-mat-label-color` is used only on cards and sheets, the only surfaces it is checked on.
6. **Look editor.** It opens as a modal `E.sheet` form docked at the end edge, with no scrim. The canvas stays visible and updates live but is inert while the sheet is open.
7. **Forms on `E.sheet`.** `E.form.formSheet` opens the designer's or app's `E.sheet` with no built-in actions and adds its own `.hd-dlg-btn` buttons, which E.sheet does not auto-close. A form can therefore refuse to close while invalid. The dialog kind is `form`.
8. **View-mode entry.** In view mode, `rename`, `look` and `add_page` first call `ctx.mount.enterEdit()` and then `ctx.mount.run(action)` again. There is no second save path.
9. **`new` and `import` outside a dashboard.** In home mode the engine root sits in the hidden `.hdx-view`, so `run(ctx,'new')` calls `ctx.home()` (the home screen is the chooser). `import` works with or without an open dashboard (Q12): the home screen shows "Import…" and the Dashboard menu lists it in home mode; the sheets for it are built on `ctx.sheetHost()` (the desktop host's `.hdx-app`) while the engine root is hidden (`sheetOf` in `E.ddialogs`).
10. **Palette thumbs.** Each thumb renders at its thumb size: the default size capped to 2×2 and never below the minimum. It uses 56 px cells and an 8 px gap, scaled into a fixed box with `--hd-e-tw/-th/-ts`. These are runtime variables (Q3 of part 1).
11. **Entity browser.** It pages 200 at a time. The area options are the union of areas in the loaded rows. Live state covers at most 20 visible rows through its own `store.watch` owner. The picker in the inspector sends one request per allowed kind (C9: one kind per request), merges the answers and filters them with `catalog.accepts`.
12. **E-S theme.** E-S uses "placeholder theme" = Fruity light and Fruity dark (stock desktop theme with a coloured accent and plain materials). E-T uses the 16 variants from `parseDesktopThemeRegistry`. Gate sheets are written to `$HOMEDASH_M4_DIR` (lead: `C:\disposable\homedash-m4\<run>`), never into the repo.

13. **Icon names are translated (Q10).** Every name of `E.icons.choices()` has `homedash.icon.<name>` (31 keys, registered prefix). The icon grid shows that label as `title` and `aria-label`, and its search matches the translated label and the id (accent-folded); `data-icon` stays the id.
14. **Server text rules inline (addendum).** `E.form.textIssue(s)` is the client copy of `ContainsControl` + `ContainsURL` (`internal/homedash/sanitize.go`). Titles, string options, the page-name field, the page rename and the dashboard name refuse such text inline (`aria-invalid`, `homedash.error.<code>`) and never `D.exec` it, so the save lane never sticks in `invalid` on typed text.
15. **Field borders are probed (Q17).** Field, select, segmented, device-box and swatch borders are drawn from `--vd-text` mixed 60 % into `--vd-mat-field-color` (an opaque colour, so the gradient materials work) and the gate probe measures them at 3:1 against their own surface in every variant. `color-mix` passes the host guard (it refuses only literal colour functions). The 60 % is the starting value; the E-T gate decides.
16. **Search folding** (`E.form.fold`: NFD, combining marks removed, lower case) is shared by the palette search, the icon grid search and `E.ddialogs.slugify`.

## Revision 2 (2026-11-26)

Revision 2 applies the lead's answers (`m4-plan-part2-answers.md`), the addendum (`m4-plan-part2-addendum.md`) and the cross-check (`m4-xcheck-part2.md`), and the dry run in the scratch copy: `npm run test:homedash` 363/364 (baseline 326/327; 37 new tests; the one failure is `bundle.test.mjs`, a partial-copy artefact), guard, i18n and wiring tests PASS, `go vet -overlay … ./ui` and `./internal/server` clean, no browser test run. One line per change:

- **Embedded code.** `hd-form.js`, `hd-samples.js`, `hd-palette.js`, `hd-entity-picker.js`, `hd-inspector.js`, `hd-look.js`, `hd-designer-dialogs.js`, `designer-kit.mjs`, `palette.test.mjs`, `inspector.test.mjs`, `dialogs.test.mjs`, the key files, the three Go files of E2/F1 and the diffs of the edited files are the dry-run files (`<!-- embed: … -->` blocks, compared byte for byte by `embed-check.mjs`). The prose of v1 for the palette, picker, inspector, look and dialogs DOM was not code: the embedded files are its implementation (the structure notes stay as the contract). Makers copy them as they are.
- **Q6, Q7, Q10, Q12, amendment 1** are part-1 revision 3; part 2 implements Q10 (translated icon names), Q12 (import from the home screen, `sheetHost`) and C5 (`enhanceTabs(D, slot)`).
- **Q17** CSS rule + probe row for field borders (reading 15, E1, E2). The "not probed" sentence is gone.
- **C0/D0** the 239 keys are embedded as two JSON files (en, de); the 14 other locales are the makers' real translations. `SAME_WORD_KEYS.de` gains `homedash.palette.tab_widgets`, `homedash.kind.sensor`, `homedash.dialog.name`, `homedash.icon.thermometer`, `homedash.icon.dashboard`, `homedash.icon.download`, `homedash.icon.upload` and `homedash.icon.timer` (the last five were missing from v1). Placeholder-only strings (`palette.card_label`, `look.radius_value`) need no entry: the test removes `{{…}}` first (cross-check I10 resolved).
- **C1** `E.form` exports `textIssue` and `fold`; `iconGrid` is translated; `field`, `text`, `number`, `select`, `toggle`, `segmented`, `colour`, `range`, `tabs`, `button`, `section`, `iconGrid`, `formSheet` implemented (v1 gave them as prose).
- **C2** palette DOM implemented (v1 prose); one add per click, drawer closes first, Escape ladder, live watch of at most 20 rows.
- **C3** picker and inspector DOM implemented (v1 prose). The 404 of `GET entity` is `entity_not_found`; the picker reads `{entity}`; `commitValue` refuses text the server refuses (addendum); title, string options and page name flag the same text inline; the tap/hold labels use literal keys.
- **D1/D2** look editor and dialogs implemented. Import: `run` checks `import` before the dashboard test, the bare document is normalised (kiosk default) before `validate`, unknown devices are listed after a successful import (I7); export reads `ctx.state().dashboard` and `ctx.store.docs.get(id)` (C7); the New sheet has an Import… button; `pageMenu` duplicate label is `homedash.ctx.duplicate`; `enhanceTabs(D, slot)`.
- **D3** `HomeDashHome.choiceState` (pure, includes Import), Import… on the home screen (first run below the choices, and in the list head), `onImport: () => inst.run('import')`, `onEmpty` opens the designer.
- **E1** the CSS is applied and measured: 381 lines (budget 900); the revision-2 states of I8 are added (switch hover/disabled, range hover, input/select disabled, hover of toggle, empty-page action, new cards, variant and family cards); `.hd-eb-more` and `.hd-eb-drawer` join the bar button rules; the flip/dragging transition reset is two full selectors (the one-selector `:is()` form failed `host.test.mjs`).
- **E2** the gates file compiles (`MaxTouchPoints` is `*int`, C6); field borders are probed; the F1 skeleton builds `hdDesignerDoc` from the starter and seeds a second dashboard for window B (I9).
- **Compact pages (addendum).** F1 step 3 asserts `x = 6` and a compacted `y`; step 15 asserts type, binding and new id only.
- **Contract questions.** Q6, Q7, Q10, Q12, Q17 and amendment 1 are decided; the rest of the questions are recorded as decided at the end.

## File map (part 2)

| File | Task |
|---|---|
| `ui/lang/homedash/*.json` (16), `scripts/homedash-tests/i18n.test.mjs` | C0, D0 |
| `ui/js/homedash/hd-form.js`, `hd-samples.js` (new) | C1 |
| `ui/js/homedash/hd-palette.js` (new), `scripts/homedash-tests/designer-kit.mjs`, `palette.test.mjs` (new) | C2 (with C1) |
| `ui/js/homedash/hd-entity-picker.js`, `hd-inspector.js` (new), `scripts/homedash-tests/inspector.test.mjs` (new) | C3 |
| `ui/js/homedash/hd-look.js`, `hd-designer-dialogs.js` (new), `scripts/homedash-tests/dialogs.test.mjs` (new) | D1, D2 |
| `ui/js/desktop/apps/homedash-home.js`, `homedash.js` (small), `scripts/homedash-tests/home.test.mjs` | D3 |
| `ui/css/desktop-app-homedash.css` | E1 |
| `ui/desktop_homedash_designer_helpers_test.go`, `ui/desktop_homedash_designer_gates_test.go` (new) | E2 |
| `ui/desktop_homedash_designer_browser_test.go` (new) | F1 |
| `internal/server/desktop_homedash_live_browser_test.go` | F2 |
| `ui/js/homedash/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `documentation/homedash-visual-system.md`, `docs/superpowers/specs/2026-11-23-homedash/m4-handover.md` | F4, F5 |

Wiring step W applies to every new engine file (R5): add it to `module-loader.js:35`, `harness.mjs` `APP_ORDER` and the `TestHomeDashWiring` engine list, in the slot order of reading 1 (between `hd-editbar.js` and `hd-designer.js`: `hd-form, hd-samples, hd-palette, hd-entity-picker, hd-inspector, hd-look, hd-designer-dialogs`). The dry-run copy has no part-1 B files, so it ran the same seven names before `hd-app.js`; `wiring.test.mjs` and the loader regexp of the Go wiring test (`'/js/([a-z/-]+\.js)'`) accept all seven. Each task adds only the files it creates. The Go wiring test and the materials test read `//go:embed` content, which `go test -overlay` does not replace: run them in the real worktree.

---

## Batch C — palette, entity browser, inspector (F4b; D5, D6)

### Task C0: Batch C texts
**Files:** `ui/lang/homedash/*.json` (16), `scripts/homedash-tests/i18n.test.mjs` (`SAME_WORD_KEYS.de` adds `homedash.palette.tab_widgets` — "Widgets is the German UI term" —, `homedash.kind.sensor` — "Sensor is the German word" —, `homedash.icon.thermometer`, `homedash.icon.dashboard`, `homedash.icon.download`, `homedash.icon.upload` and `homedash.icon.timer` — "the German UI uses the same word"; the `homedash.dialog.name` entry belongs to D0). **Existing symbols changed:** `SAME_WORD_KEYS` (LOW).
- [ ] Add the keys below to all 16 files. English and German are the tested files (merge them into `en.json` and `de.json`); makers translate the other 14 locales for real. German uses informal "Du", real umlauts, no "Sie"/"Ihr". The 239 keys of C0 and D0 are in the two files; groups: `palette.*`, `inspector.*` (incl. `tap`, `hold`, the `do_*` labels), `option.<key>.label|help`, `category.*`, `kind.*` (21 new; `scene`, `script`, `button` exist), `widget.<type>.desc` (the 10 registered types), `look.*`, `dialog.*`, `menu.page_*`, **`icon.<name>`** (Q10: the 31 names of `E.icons.choices()`; a type added later needs its `widget.<type>.desc` and an icon an `icon.<name>` key) and the changed `home.create_empty_hint`.
<!-- embed: dryrun-m4/overlay/keys/part2-keys.en.json -->
```json
{
  "homedash.category.camera": "Cameras",
  "homedash.category.climate": "Climate",
  "homedash.category.control": "Controls",
  "homedash.category.layout": "Layout",
  "homedash.category.media": "Media",
  "homedash.category.sensor": "Sensors",
  "homedash.category.status": "Status",
  "homedash.dialog.cancel": "Cancel",
  "homedash.dialog.close": "Close",
  "homedash.dialog.export_done": "Download of “{{file}}” started.",
  "homedash.dialog.export_drafts": "{{count}} widgets without a device were left out.",
  "homedash.dialog.import_done": "“{{name}}” imported.",
  "homedash.dialog.import_failed": "The dashboard could not be imported: {{reason}}",
  "homedash.dialog.import_issues_text": "The dashboard in the file has {{count}} problems:",
  "homedash.dialog.import_issues_title": "This file cannot be imported",
  "homedash.dialog.import_more": "and {{count}} more",
  "homedash.dialog.import_title": "Import a dashboard",
  "homedash.dialog.import_too_large": "The file is larger than 512 KiB.",
  "homedash.dialog.import_unknown": "{{count}} devices are not known here yet.",
  "homedash.dialog.name": "Name",
  "homedash.dialog.name_empty": "Enter a name.",
  "homedash.dialog.name_long": "At most {{max}} characters.",
  "homedash.dialog.new_import": "Import a file…",
  "homedash.dialog.new_title": "New dashboard",
  "homedash.dialog.page_delete_text": "The {{count}} widgets on it are deleted too. You can undo this.",
  "homedash.dialog.page_delete_title": "Delete page “{{name}}”?",
  "homedash.dialog.rename_title": "Rename dashboard",
  "homedash.dialog.save": "Save",
  "homedash.home.create_empty_hint": "A blank page. The designer opens right away.",
  "homedash.icon.brightness": "Brightness",
  "homedash.icon.bulb": "Bulb",
  "homedash.icon.button": "Button",
  "homedash.icon.chart": "Chart",
  "homedash.icon.clock": "Clock",
  "homedash.icon.color": "Colour",
  "homedash.icon.color-temp": "Colour temperature",
  "homedash.icon.cpu": "Processor",
  "homedash.icon.dashboard": "Dashboard",
  "homedash.icon.disk": "Disk",
  "homedash.icon.download": "Download",
  "homedash.icon.droplet": "Droplet",
  "homedash.icon.effect": "Effect",
  "homedash.icon.entity": "Device",
  "homedash.icon.fan-heater": "Fan heater",
  "homedash.icon.flame": "Flame",
  "homedash.icon.gauge": "Gauge",
  "homedash.icon.home": "Home",
  "homedash.icon.list": "List",
  "homedash.icon.lock": "Lock",
  "homedash.icon.memory": "Memory",
  "homedash.icon.number": "Number",
  "homedash.icon.outlet": "Outlet",
  "homedash.icon.plug": "Plug",
  "homedash.icon.power": "Power",
  "homedash.icon.scene": "Scene",
  "homedash.icon.script": "Script",
  "homedash.icon.snowflake": "Snowflake",
  "homedash.icon.thermometer": "Thermometer",
  "homedash.icon.timer": "Timer",
  "homedash.icon.upload": "Upload",
  "homedash.inspector.align_hint": "Aligns to {{name}}, the widget selected last.",
  "homedash.inspector.align_left": "Align left edges",
  "homedash.inspector.align_top": "Align top edges",
  "homedash.inspector.binding_humidity": "Humidity sensor",
  "homedash.inspector.binding_primary": "Device",
  "homedash.inspector.choose_device": "Choose a device…",
  "homedash.inspector.clear_binding": "Clear device",
  "homedash.inspector.colour_auto": "Dashboard colour",
  "homedash.inspector.confirm": "Ask before running",
  "homedash.inspector.confirm_hint": "The first tap prepares the action, the second one runs it.",
  "homedash.inspector.delete_many": "Delete {{count}} widgets",
  "homedash.inspector.delete_one": "Delete widget",
  "homedash.inspector.do_default": "Default",
  "homedash.inspector.do_more_info": "Show details",
  "homedash.inspector.do_navigate": "Go to a page",
  "homedash.inspector.do_none": "Do nothing",
  "homedash.inspector.do_toggle": "Switch on and off",
  "homedash.inspector.draft_note": "This widget is saved once it has a device.",
  "homedash.inspector.empty_none": "Empty for none",
  "homedash.inspector.enum_center": "Centre",
  "homedash.inspector.enum_start": "Start",
  "homedash.inspector.family": "Family",
  "homedash.inspector.family_default": "Like the dashboard ({{family}})",
  "homedash.inspector.family_mixed": "Mixed",
  "homedash.inspector.family_warn": "This page now uses three families. Two look calmer.",
  "homedash.inspector.hold": "Hold",
  "homedash.inspector.icon": "Icon",
  "homedash.inspector.icon_auto": "Automatic",
  "homedash.inspector.icon_none": "No icon matches",
  "homedash.inspector.icon_search": "Search icons",
  "homedash.inspector.multi_family": "Family for all",
  "homedash.inspector.multi_title": "{{count}} widgets",
  "homedash.inspector.no_match": "No matching device",
  "homedash.inspector.page_choose": "Choose a page",
  "homedash.inspector.page_compact": "Move widgets up to close gaps",
  "homedash.inspector.page_hint": "Select a widget to edit it. Shift-click or drag across the page to select several.",
  "homedash.inspector.page_icon": "Page icon",
  "homedash.inspector.page_name": "Page name",
  "homedash.inspector.page_title": "Page",
  "homedash.inspector.required": "required",
  "homedash.inspector.search_failed": "Devices could not be loaded.",
  "homedash.inspector.searching": "Searching…",
  "homedash.inspector.tab_actions": "Actions",
  "homedash.inspector.tab_data": "Data",
  "homedash.inspector.tab_look": "Look",
  "homedash.inspector.tab_options": "Options",
  "homedash.inspector.tap": "Tap",
  "homedash.inspector.target_page": "Page",
  "homedash.inspector.title": "Title",
  "homedash.inspector.unknown_entity": "Unknown device {{id}}. It may have been removed or renamed.",
  "homedash.inspector.value_colour": "Enter a colour as #rrggbb.",
  "homedash.inspector.value_max": "Enter at most {{max}}.",
  "homedash.inspector.value_min": "Enter at least {{min}}.",
  "homedash.inspector.value_number": "Enter a number.",
  "homedash.inspector.value_range": "Enter a value from {{min}} to {{max}}.",
  "homedash.inspector.value_required": "Enter a value.",
  "homedash.inspector.variant": "Style",
  "homedash.kind.binary_sensor": "Binary sensor",
  "homedash.kind.camera": "Camera",
  "homedash.kind.climate": "Climate",
  "homedash.kind.cover": "Cover",
  "homedash.kind.fan": "Fan",
  "homedash.kind.generic": "Other",
  "homedash.kind.humidifier": "Humidifier",
  "homedash.kind.light": "Light",
  "homedash.kind.lock": "Lock",
  "homedash.kind.media_player": "Media player",
  "homedash.kind.number": "Number",
  "homedash.kind.presence": "Presence",
  "homedash.kind.printer": "Printer",
  "homedash.kind.select": "Choice",
  "homedash.kind.sensor": "Sensor",
  "homedash.kind.service_status": "Service status",
  "homedash.kind.sun": "Sun",
  "homedash.kind.switch": "Switch",
  "homedash.kind.update": "Update",
  "homedash.kind.vacuum": "Vacuum cleaner",
  "homedash.kind.weather": "Weather",
  "homedash.look.accent": "Accent",
  "homedash.look.backdrop": "Background",
  "homedash.look.backdrop_gradient": "Gradient",
  "homedash.look.backdrop_solid": "Solid",
  "homedash.look.colours": "Colours",
  "homedash.look.compaction": "Gaps on this page",
  "homedash.look.custom_hint": "HomeDash adjusts the contrast so text stays readable.",
  "homedash.look.density": "Spacing",
  "homedash.look.density_comfortable": "Balanced",
  "homedash.look.density_compact": "Compact",
  "homedash.look.density_spacious": "Spacious",
  "homedash.look.desc_glass": "Layered glass in the theme’s colours",
  "homedash.look.desc_instrument": "A calm technical display",
  "homedash.look.desc_minimal": "Type and hairlines",
  "homedash.look.desc_tactile": "Quiet, well-made hardware",
  "homedash.look.family": "Family",
  "homedash.look.fit": "Size on screen",
  "homedash.look.fit_note": "In edit mode the page always scrolls, so cells keep their size while you arrange widgets.",
  "homedash.look.fit_scroll": "Scroll",
  "homedash.look.grid": "Grid",
  "homedash.look.mode_custom": "Custom",
  "homedash.look.mode_dark": "Dark",
  "homedash.look.mode_light": "Light",
  "homedash.look.mode_theme": "Desktop theme",
  "homedash.look.radius": "Corner rounding",
  "homedash.look.radius_value": "{{value}}×",
  "homedash.look.row_auto": "Automatic",
  "homedash.look.row_fixed": "Fixed",
  "homedash.look.row_unit": "Row height",
  "homedash.look.scheme": "Brightness",
  "homedash.look.surface": "Surface",
  "homedash.look.surface_hue": "Surface tint",
  "homedash.look.title": "Look",
  "homedash.menu.page_delete": "Delete page…",
  "homedash.menu.page_icon": "Icon…",
  "homedash.menu.page_left": "Move left",
  "homedash.menu.page_rename": "Rename",
  "homedash.menu.page_right": "Move right",
  "homedash.option.accent.help": "Empty uses the dashboard accent.",
  "homedash.option.accent.label": "Accent colour",
  "homedash.option.alert.help": "Values from here on show as an alert. Empty for none.",
  "homedash.option.alert.label": "Alert from",
  "homedash.option.align.label": "Alignment",
  "homedash.option.decimals.label": "Decimal places",
  "homedash.option.hours.help": "How far back the chart reaches, in hours.",
  "homedash.option.hours.label": "History span",
  "homedash.option.live.label": "Live picture",
  "homedash.option.max.label": "Highest value",
  "homedash.option.min.label": "Lowest value",
  "homedash.option.page.label": "Target page",
  "homedash.option.refresh_seconds.label": "Refresh every",
  "homedash.option.show_date.label": "Show date",
  "homedash.option.show_minmax.label": "Show minimum and maximum",
  "homedash.option.show_seconds.label": "Show seconds",
  "homedash.option.warn.help": "Values from here on show as a warning. Empty for none.",
  "homedash.option.warn.label": "Warning from",
  "homedash.palette.all_areas": "All areas",
  "homedash.palette.all_kinds": "All kinds",
  "homedash.palette.all_sources": "All sources",
  "homedash.palette.card_label": "{{type}}, {{variant}}",
  "homedash.palette.clear_search": "Clear search",
  "homedash.palette.count": "{{count}} devices",
  "homedash.palette.count_of": "{{shown}} of {{total}} devices",
  "homedash.palette.empty": "No device matches these filters",
  "homedash.palette.entity_hint": "Enter adds a widget for this device. Drag it onto a widget to show this device there.",
  "homedash.palette.failed": "The devices could not be loaded.",
  "homedash.palette.filter_area": "Area",
  "homedash.palette.filter_kind": "Kind",
  "homedash.palette.filter_source": "Source",
  "homedash.palette.load_more": "Show more",
  "homedash.palette.loading": "Loading devices…",
  "homedash.palette.no_widgets": "No widget matches “{{q}}”",
  "homedash.palette.retry": "Try again",
  "homedash.palette.sample_cpu": "CPU load",
  "homedash.palette.sample_high": "High",
  "homedash.palette.sample_humidity": "Humidity",
  "homedash.palette.sample_lamp": "Desk lamp",
  "homedash.palette.sample_light": "Reading light",
  "homedash.palette.sample_low": "Low",
  "homedash.palette.sample_mid": "Medium",
  "homedash.palette.sample_mode": "Heater mode",
  "homedash.palette.sample_outside": "Outside",
  "homedash.palette.sample_power": "Kitchen power",
  "homedash.palette.sample_room": "Living room",
  "homedash.palette.sample_scene": "Movie night",
  "homedash.palette.sample_volume": "Volume",
  "homedash.palette.search_entities": "Search devices",
  "homedash.palette.search_widgets": "Search widgets",
  "homedash.palette.tab_entities": "Devices",
  "homedash.palette.tab_widgets": "Widgets",
  "homedash.palette.widget_hint": "Enter adds the widget below the selection. Drag it to place it.",
  "homedash.widget.action.desc": "Runs a scene, script or button",
  "homedash.widget.climate.desc": "Sets the room temperature",
  "homedash.widget.entity.desc": "Shows any device as a tile",
  "homedash.widget.gauge.desc": "Shows a value on a scale",
  "homedash.widget.light.desc": "Switches and dims a light",
  "homedash.widget.number.desc": "Sets a number such as a target or volume",
  "homedash.widget.select.desc": "Picks one of a device’s options",
  "homedash.widget.sensor.desc": "Shows a measured value",
  "homedash.widget.switch.desc": "Turns a device on and off",
  "homedash.widget.temperature.desc": "Shows a temperature"
}
```
<!-- embed: dryrun-m4/overlay/keys/part2-keys.de.json -->
```json
{
  "homedash.category.camera": "Kameras",
  "homedash.category.climate": "Klima",
  "homedash.category.control": "Steuerung",
  "homedash.category.layout": "Gestaltung",
  "homedash.category.media": "Medien",
  "homedash.category.sensor": "Sensoren",
  "homedash.category.status": "Statusanzeigen",
  "homedash.dialog.cancel": "Abbrechen",
  "homedash.dialog.close": "Schließen",
  "homedash.dialog.export_done": "Download von „{{file}}“ gestartet.",
  "homedash.dialog.export_drafts": "{{count}} Widgets ohne Gerät wurden weggelassen.",
  "homedash.dialog.import_done": "„{{name}}“ importiert.",
  "homedash.dialog.import_failed": "Das Dashboard ließ sich nicht importieren: {{reason}}",
  "homedash.dialog.import_issues_text": "Das Dashboard in der Datei hat {{count}} Probleme:",
  "homedash.dialog.import_issues_title": "Diese Datei lässt sich nicht importieren",
  "homedash.dialog.import_more": "und {{count}} weitere",
  "homedash.dialog.import_title": "Dashboard importieren",
  "homedash.dialog.import_too_large": "Die Datei ist größer als 512 KiB.",
  "homedash.dialog.import_unknown": "{{count}} Geräte sind hier noch nicht bekannt.",
  "homedash.dialog.name": "Name",
  "homedash.dialog.name_empty": "Gib einen Namen ein.",
  "homedash.dialog.name_long": "Höchstens {{max}} Zeichen.",
  "homedash.dialog.new_import": "Datei importieren …",
  "homedash.dialog.new_title": "Neues Dashboard",
  "homedash.dialog.page_delete_text": "Die {{count}} Widgets darauf werden mit gelöscht. Du kannst das rückgängig machen.",
  "homedash.dialog.page_delete_title": "Seite „{{name}}“ löschen?",
  "homedash.dialog.rename_title": "Dashboard umbenennen",
  "homedash.dialog.save": "Speichern",
  "homedash.home.create_empty_hint": "Eine leere Seite. Der Designer öffnet sich sofort.",
  "homedash.icon.brightness": "Helligkeit",
  "homedash.icon.bulb": "Glühbirne",
  "homedash.icon.button": "Taste",
  "homedash.icon.chart": "Diagramm",
  "homedash.icon.clock": "Uhr",
  "homedash.icon.color": "Farbe",
  "homedash.icon.color-temp": "Farbtemperatur",
  "homedash.icon.cpu": "Prozessor",
  "homedash.icon.dashboard": "Dashboard",
  "homedash.icon.disk": "Festplatte",
  "homedash.icon.download": "Download",
  "homedash.icon.droplet": "Tropfen",
  "homedash.icon.effect": "Effekt",
  "homedash.icon.entity": "Gerät",
  "homedash.icon.fan-heater": "Heizlüfter",
  "homedash.icon.flame": "Flamme",
  "homedash.icon.gauge": "Messanzeige",
  "homedash.icon.home": "Startseite",
  "homedash.icon.list": "Liste",
  "homedash.icon.lock": "Schloss",
  "homedash.icon.memory": "Arbeitsspeicher",
  "homedash.icon.number": "Zahl",
  "homedash.icon.outlet": "Steckdose",
  "homedash.icon.plug": "Stecker",
  "homedash.icon.power": "Ein-Aus-Schalter",
  "homedash.icon.scene": "Szene",
  "homedash.icon.script": "Skript",
  "homedash.icon.snowflake": "Schneeflocke",
  "homedash.icon.thermometer": "Thermometer",
  "homedash.icon.timer": "Timer",
  "homedash.icon.upload": "Upload",
  "homedash.inspector.align_hint": "Richtet an {{name}} aus, dem zuletzt gewählten Widget.",
  "homedash.inspector.align_left": "Linke Kanten ausrichten",
  "homedash.inspector.align_top": "Obere Kanten ausrichten",
  "homedash.inspector.binding_humidity": "Feuchtesensor",
  "homedash.inspector.binding_primary": "Gerät",
  "homedash.inspector.choose_device": "Gerät auswählen …",
  "homedash.inspector.clear_binding": "Gerät entfernen",
  "homedash.inspector.colour_auto": "Dashboard-Farbe",
  "homedash.inspector.confirm": "Vor dem Ausführen nachfragen",
  "homedash.inspector.confirm_hint": "Der erste Tipp bereitet die Aktion vor, der zweite führt sie aus.",
  "homedash.inspector.delete_many": "{{count}} Widgets löschen",
  "homedash.inspector.delete_one": "Widget löschen",
  "homedash.inspector.do_default": "Standard",
  "homedash.inspector.do_more_info": "Details zeigen",
  "homedash.inspector.do_navigate": "Zu einer Seite wechseln",
  "homedash.inspector.do_none": "Nichts tun",
  "homedash.inspector.do_toggle": "Ein- und ausschalten",
  "homedash.inspector.draft_note": "Dieses Widget wird gespeichert, sobald es ein Gerät hat.",
  "homedash.inspector.empty_none": "Leer für keinen",
  "homedash.inspector.enum_center": "Mitte",
  "homedash.inspector.enum_start": "Anfang",
  "homedash.inspector.family": "Familie",
  "homedash.inspector.family_default": "Wie das Dashboard ({{family}})",
  "homedash.inspector.family_mixed": "Gemischt",
  "homedash.inspector.family_warn": "Diese Seite nutzt jetzt drei Familien. Zwei wirken ruhiger.",
  "homedash.inspector.hold": "Halten",
  "homedash.inspector.icon": "Symbol",
  "homedash.inspector.icon_auto": "Automatisch",
  "homedash.inspector.icon_none": "Kein Symbol passt",
  "homedash.inspector.icon_search": "Symbole suchen",
  "homedash.inspector.multi_family": "Familie für alle",
  "homedash.inspector.multi_title": "{{count}} Widgets",
  "homedash.inspector.no_match": "Kein passendes Gerät",
  "homedash.inspector.page_choose": "Seite wählen",
  "homedash.inspector.page_compact": "Widgets nach oben rücken, um Lücken zu schließen",
  "homedash.inspector.page_hint": "Wähl ein Widget, um es zu bearbeiten. Mit Umschalt-Klick oder Ziehen über die Seite wählst Du mehrere.",
  "homedash.inspector.page_icon": "Seitensymbol",
  "homedash.inspector.page_name": "Seitenname",
  "homedash.inspector.page_title": "Seite",
  "homedash.inspector.required": "erforderlich",
  "homedash.inspector.search_failed": "Geräte ließen sich nicht laden.",
  "homedash.inspector.searching": "Suche läuft …",
  "homedash.inspector.tab_actions": "Aktionen",
  "homedash.inspector.tab_data": "Daten",
  "homedash.inspector.tab_look": "Aussehen",
  "homedash.inspector.tab_options": "Optionen",
  "homedash.inspector.tap": "Tippen",
  "homedash.inspector.target_page": "Seite",
  "homedash.inspector.title": "Titel",
  "homedash.inspector.unknown_entity": "Unbekanntes Gerät {{id}}. Es wurde vielleicht entfernt oder umbenannt.",
  "homedash.inspector.value_colour": "Gib eine Farbe als #rrggbb ein.",
  "homedash.inspector.value_max": "Gib höchstens {{max}} ein.",
  "homedash.inspector.value_min": "Gib mindestens {{min}} ein.",
  "homedash.inspector.value_number": "Gib eine Zahl ein.",
  "homedash.inspector.value_range": "Gib einen Wert von {{min}} bis {{max}} ein.",
  "homedash.inspector.value_required": "Gib einen Wert ein.",
  "homedash.inspector.variant": "Stil",
  "homedash.kind.binary_sensor": "Binärsensor",
  "homedash.kind.camera": "Kamera",
  "homedash.kind.climate": "Klima",
  "homedash.kind.cover": "Beschattung",
  "homedash.kind.fan": "Ventilator",
  "homedash.kind.generic": "Sonstiges",
  "homedash.kind.humidifier": "Luftbefeuchter",
  "homedash.kind.light": "Licht",
  "homedash.kind.lock": "Schloss",
  "homedash.kind.media_player": "Mediaplayer",
  "homedash.kind.number": "Zahl",
  "homedash.kind.presence": "Anwesenheit",
  "homedash.kind.printer": "Drucker",
  "homedash.kind.select": "Auswahl",
  "homedash.kind.sensor": "Sensor",
  "homedash.kind.service_status": "Dienststatus",
  "homedash.kind.sun": "Sonne",
  "homedash.kind.switch": "Schalter",
  "homedash.kind.update": "Aktualisierung",
  "homedash.kind.vacuum": "Staubsauger",
  "homedash.kind.weather": "Wetter",
  "homedash.look.accent": "Akzent",
  "homedash.look.backdrop": "Hintergrund",
  "homedash.look.backdrop_gradient": "Verlauf",
  "homedash.look.backdrop_solid": "Einfarbig",
  "homedash.look.colours": "Farben",
  "homedash.look.compaction": "Lücken auf dieser Seite",
  "homedash.look.custom_hint": "HomeDash passt den Kontrast an, damit Text lesbar bleibt.",
  "homedash.look.density": "Abstände",
  "homedash.look.density_comfortable": "Ausgewogen",
  "homedash.look.density_compact": "Kompakt",
  "homedash.look.density_spacious": "Großzügig",
  "homedash.look.desc_glass": "Geschichtetes Glas in den Farben des Themes",
  "homedash.look.desc_instrument": "Eine ruhige technische Anzeige",
  "homedash.look.desc_minimal": "Schrift und Haarlinien",
  "homedash.look.desc_tactile": "Ruhige, gut gemachte Hardware",
  "homedash.look.family": "Familie",
  "homedash.look.fit": "Größe am Bildschirm",
  "homedash.look.fit_note": "Im Bearbeitungsmodus scrollt die Seite immer, damit Zellen beim Anordnen ihre Größe behalten.",
  "homedash.look.fit_scroll": "Scrollen",
  "homedash.look.grid": "Raster",
  "homedash.look.mode_custom": "Eigene",
  "homedash.look.mode_dark": "Dunkel",
  "homedash.look.mode_light": "Hell",
  "homedash.look.mode_theme": "Desktop-Theme",
  "homedash.look.radius": "Eckenrundung",
  "homedash.look.radius_value": "{{value}}×",
  "homedash.look.row_auto": "Automatisch",
  "homedash.look.row_fixed": "Fest",
  "homedash.look.row_unit": "Zeilenhöhe",
  "homedash.look.scheme": "Helligkeit",
  "homedash.look.surface": "Fläche",
  "homedash.look.surface_hue": "Flächenton",
  "homedash.look.title": "Aussehen",
  "homedash.menu.page_delete": "Seite löschen …",
  "homedash.menu.page_icon": "Symbol …",
  "homedash.menu.page_left": "Nach links",
  "homedash.menu.page_rename": "Umbenennen",
  "homedash.menu.page_right": "Nach rechts",
  "homedash.option.accent.help": "Leer nutzt den Akzent des Dashboards.",
  "homedash.option.accent.label": "Akzentfarbe",
  "homedash.option.alert.help": "Werte ab hier erscheinen als Alarm. Leer für keinen.",
  "homedash.option.alert.label": "Alarm ab",
  "homedash.option.align.label": "Ausrichtung",
  "homedash.option.decimals.label": "Nachkommastellen",
  "homedash.option.hours.help": "Wie weit der Verlauf zurückreicht, in Stunden.",
  "homedash.option.hours.label": "Verlaufsdauer",
  "homedash.option.live.label": "Livebild",
  "homedash.option.max.label": "Größter Wert",
  "homedash.option.min.label": "Kleinster Wert",
  "homedash.option.page.label": "Zielseite",
  "homedash.option.refresh_seconds.label": "Aktualisieren alle",
  "homedash.option.show_date.label": "Datum zeigen",
  "homedash.option.show_minmax.label": "Minimum und Maximum zeigen",
  "homedash.option.show_seconds.label": "Sekunden zeigen",
  "homedash.option.warn.help": "Werte ab hier erscheinen als Warnung. Leer für keine.",
  "homedash.option.warn.label": "Warnung ab",
  "homedash.palette.all_areas": "Alle Bereiche",
  "homedash.palette.all_kinds": "Alle Arten",
  "homedash.palette.all_sources": "Alle Quellen",
  "homedash.palette.card_label": "{{type}}, {{variant}}",
  "homedash.palette.clear_search": "Suche leeren",
  "homedash.palette.count": "{{count}} Geräte",
  "homedash.palette.count_of": "{{shown}} von {{total}} Geräten",
  "homedash.palette.empty": "Kein Gerät passt zu diesen Filtern",
  "homedash.palette.entity_hint": "Die Eingabetaste fügt ein Widget für dieses Gerät hinzu. Auf ein Widget ziehen, damit es dieses Gerät zeigt.",
  "homedash.palette.failed": "Die Geräte ließen sich nicht laden.",
  "homedash.palette.filter_area": "Bereich",
  "homedash.palette.filter_kind": "Art",
  "homedash.palette.filter_source": "Quelle",
  "homedash.palette.load_more": "Mehr zeigen",
  "homedash.palette.loading": "Geräte werden geladen …",
  "homedash.palette.no_widgets": "Kein Widget passt zu „{{q}}“",
  "homedash.palette.retry": "Erneut versuchen",
  "homedash.palette.sample_cpu": "CPU-Last",
  "homedash.palette.sample_high": "Hoch",
  "homedash.palette.sample_humidity": "Luftfeuchte",
  "homedash.palette.sample_lamp": "Schreibtischlampe",
  "homedash.palette.sample_light": "Leselampe",
  "homedash.palette.sample_low": "Niedrig",
  "homedash.palette.sample_mid": "Mittel",
  "homedash.palette.sample_mode": "Heizmodus",
  "homedash.palette.sample_outside": "Draußen",
  "homedash.palette.sample_power": "Küchenstrom",
  "homedash.palette.sample_room": "Wohnzimmer",
  "homedash.palette.sample_scene": "Filmabend",
  "homedash.palette.sample_volume": "Lautstärke",
  "homedash.palette.search_entities": "Geräte suchen",
  "homedash.palette.search_widgets": "Widgets suchen",
  "homedash.palette.tab_entities": "Geräte",
  "homedash.palette.tab_widgets": "Widgets",
  "homedash.palette.widget_hint": "Die Eingabetaste fügt das Widget unter der Auswahl ein. Zum Platzieren ziehen.",
  "homedash.widget.action.desc": "Startet eine Szene, ein Skript oder eine Taste",
  "homedash.widget.climate.desc": "Stellt die Raumtemperatur ein",
  "homedash.widget.entity.desc": "Zeigt ein beliebiges Gerät als Kachel",
  "homedash.widget.gauge.desc": "Zeigt einen Wert auf einer Skala",
  "homedash.widget.light.desc": "Schaltet und dimmt ein Licht",
  "homedash.widget.number.desc": "Stellt eine Zahl ein, etwa Sollwert oder Lautstärke",
  "homedash.widget.select.desc": "Wählt eine Option eines Geräts",
  "homedash.widget.sensor.desc": "Zeigt einen Messwert",
  "homedash.widget.switch.desc": "Schaltet ein Gerät ein und aus",
  "homedash.widget.temperature.desc": "Zeigt eine Temperatur"
}
```
- [ ] Run `npm run test:homedash` (i18n PASS) and `go test ./ui -run '(?i)^(TestTranslations_.*|TestDesktopDynamicI18nPrefixKeysExistInAllLanguages|TestFrontend_StaticI18nKeysExistInEnglishBundle)$' -count=1`. Both PASS. Commit — `feat(homedash): palette, inspector, look and dialog texts in 16 locales`.
- The dry run checked every literal `t('homedash.…')` key of the seven files against base `en.json` + part-1 A11 + these keys (250 literal keys, 22 dynamic uses, all under registered prefixes): none missing.

### Task C1: hd-form.js and hd-samples.js
**Files:** create `ui/js/homedash/hd-form.js`, `hd-samples.js` (tests: `palette.test.mjs` and `designer-kit.mjs` of C2; C1 and C2 are one commit). Wiring step W for both. **Existing symbols changed:** none (lists only).
- [ ] `hd-form.js`, in full (pure key maths, the server text rules, every builder):
<!-- embed: dryrun-m4/ui/js/homedash/hd-form.js -->
```js
// HomeDash editor form controls (M4 part 2; design-visual-system 6, contract C15/C20). Builders shared by the palette, inspector, look editor and
// dialogs: native elements painted from --vd-mat-* by desktop-app-homedash.css, labelled by <label for> or aria-label, 32 px high (44 px coarse).
// Each returns {el, set(value), focus()} (+ wrap when the control sits in a decorated box); set() never fires a callback, so the inspector patches
// values in place. segmented()/tabs() are roving groups (arrows move and select); roving()/gridMove() are their pure key maths. iconGrid() is a
// searchable group of aria-pressed icon buttons. formSheet() opens a form in an E.sheet with its own .hd-dlg-btn buttons. textIssue() is the
// client copy of the server's text rules. No DOM at load.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.icons || !E.sheet) throw new Error('hd-form.js: load hd-icons.js and hd-sheet.js first');
    const { el, setAttr, uid } = E.core, STEP = Object.freeze({ ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }), HORIZONTAL = Object.freeze(['ArrowLeft', 'ArrowRight', 'Home', 'End']);
    const runes = (v, max) => Array.from(String(v == null ? '' : v)).slice(0, max).join('');
    const hit = (ev, sel) => (ev && ev.target && typeof ev.target.closest === 'function' ? ev.target.closest(sel) : null);
    const FOLD = new RegExp('[' + String.fromCodePoint(0x300) + '-' + String.fromCodePoint(0x36f) + ']', 'g');
    // Case- and accent-insensitive text for search (an umlaut matches its base letter).
    const fold = s => String(s == null ? '' : s).normalize('NFD').replace(FOLD, '').toLowerCase();
    function roving(key, index, count) { if (!(count > 0)) return null; if (key === 'Home') return 0; if (key === 'End') return count - 1; const d = STEP[key]; return d ? (index + d + count) % count : null; }
    // Grouped grid: `sizes` are item counts of consecutive groups laid out in `cols` columns. Left/Right walk the flat order; Up/Down keep the
    // column and cross into the neighbouring group; Home/End jump to the ends; no wrap.
    function gridMove(key, index, sizes, cols) {
        const c = Math.max(1, cols | 0), list = (sizes || []).map(n => Math.max(0, n | 0)), total = list.reduce((a, b) => a + b, 0); if (!total) return null;
        const i = Math.min(Math.max(index | 0, 0), total - 1);
        if (key === 'Home') return 0; if (key === 'End') return total - 1;
        if (key === 'ArrowLeft') return Math.max(0, i - 1); if (key === 'ArrowRight') return Math.min(total - 1, i + 1);
        if (key !== 'ArrowUp' && key !== 'ArrowDown') return null;
        let g = 0, start = 0; while (i >= start + list[g]) { start += list[g]; g += 1; }
        const col = (i - start) % c, row = Math.floor((i - start) / c), rows = n => Math.ceil(n / c), first = k => list.slice(0, k).reduce((a, b) => a + b, 0);
        const at = (k, r) => first(k) + Math.min(list[k] - 1, r * c + col);
        if (key === 'ArrowDown') { if (row + 1 < rows(list[g])) return at(g, row + 1); for (let k = g + 1; k < list.length; k++) if (list[k]) return at(k, 0); return i; }
        if (row > 0) return at(g, row - 1); for (let k = g - 1; k >= 0; k--) if (list[k]) return at(k, rows(list[k]) - 1); return i;
    }
    // Server text rules (internal/homedash/sanitize.go ContainsControl + ContainsURL). Returns '' when the server accepts `s`, else the issue code
    // ('control_chars' | 'url_not_allowed') that homedash.error.<code> explains. Titles, string options, page and dashboard names use it inline,
    // so the save lane never sticks in 'invalid' on text the user typed.
    const CTRL = /[\x00-\x1f\x7f-\x9f]/;
    const BIDI = new RegExp('[' + [0x061c, 0x200e, 0x200f].map(c => String.fromCodePoint(c)).join('') + String.fromCodePoint(0x202a) + '-' + String.fromCodePoint(0x202e) + String.fromCodePoint(0x2066) + '-' + String.fromCodePoint(0x2069) + ']');
    const URLISH = Object.freeze(['://', 'javascript:', 'data:', 'vbscript:']);
    function loneSurrogate(v) { for (let i = 0; i < v.length; i++) { const c = v.charCodeAt(i);
        if (c >= 0xd800 && c <= 0xdbff) { const d = v.charCodeAt(i + 1); if (d >= 0xdc00 && d <= 0xdfff) { i += 1; continue; } return true; } if (c >= 0xdc00 && c <= 0xdfff) return true; } return false; }
    function textIssue(s) {
        const v = String(s == null ? '' : s); if (CTRL.test(v) || BIDI.test(v) || loneSurrogate(v)) return 'control_chars';
        const low = v.toLowerCase(); return URLISH.some(m => low.includes(m)) || low.trim().startsWith('//') ? 'url_not_allowed' : '';
    }

    // ---- DOM builders (essential structure; classes and events are the contract of desktop-app-homedash.css) ----
    const ico = name => E.icons.icon(name), labelable = n => !!n && /^(input|select|button|textarea)$/i.test(n.localName || '');
    function field(o) {
        const x = o || {}, id = uid('hd-f'), c = x.control || {}, ctl = c.el || null, box = x.wrap || c.wrap || ctl;
        const label = el('label', { class: 'hd-f-label', id: id + '-l' }, [el('span', { text: x.label || '' }), x.required ? el('span', { class: 'hd-f-req', text: x.required === true ? '*' : x.required }) : null]);
        const help = x.help ? el('p', { class: 'hd-f-help', id: id + '-h', text: x.help }) : null, err = el('p', { class: 'hd-f-error', id: id + '-e', hidden: true });
        const root = el('div', { class: 'hd-f', 'data-hd-inline': x.inline ? 'true' : null }, [label, box, help, err]);
        if (ctl) { if (!ctl.id) ctl.id = id + '-c'; if (labelable(ctl)) label.setAttribute('for', ctl.id); else setAttr(ctl, 'aria-labelledby', id + '-l'); }
        const describe = shown => { if (ctl) setAttr(ctl, 'aria-describedby', [help ? id + '-h' : '', shown ? id + '-e' : ''].filter(Boolean).join(' ') || null); };
        describe(false);
        function error(text) { const on = !!text; err.textContent = on ? String(text) : ''; setAttr(err, 'hidden', on ? null : true); if (ctl) setAttr(ctl, 'aria-invalid', on ? true : null); describe(on); }
        return { el: root, control: c, error };
    }
    function text(o) {
        const x = o || {}, max = x.max || 200, input = el('input', { class: 'hd-f-input', type: x.search ? 'search' : 'text', spellcheck: 'false', autocomplete: 'off', 'data-hd-field': x.key, 'aria-label': x.label, placeholder: x.placeholder });
        input.value = x.value == null ? '' : String(x.value);
        const cut = () => { const v = runes(input.value, max); if (v !== input.value) input.value = v; return v; };
        let composing = false;
        input.addEventListener('compositionstart', () => { composing = true; });
        input.addEventListener('compositionend', () => { composing = false; const v = cut(); if (x.onInput) x.onInput(v); });
        input.addEventListener('input', ev => { if (composing || (ev && ev.isComposing)) return; const v = cut(); if (x.onInput) x.onInput(v); });
        input.addEventListener('change', () => { if (x.onCommit) x.onCommit(cut()); });
        input.addEventListener('blur', () => { if (x.onBlur) x.onBlur(input.value); });
        input.addEventListener('keydown', ev => { if (ev.key === 'Enter' && !ev.isComposing) { ev.preventDefault(); if (x.onEnter) x.onEnter(cut()); } });
        return { el: input, set(v) { input.value = v == null ? '' : String(v); }, get: () => input.value, focus() { input.focus(); if (x.select && typeof input.select === 'function') input.select(); } };
    }
    function number(o) {
        const x = o || {}, step = x.step === undefined || x.step === null ? 'any' : x.step;
        const input = el('input', { class: 'hd-f-input', type: 'number', inputmode: step === 1 ? 'numeric' : 'decimal', step: String(step), min: x.min, max: x.max, placeholder: x.placeholder, 'aria-label': x.label, 'data-hd-field': x.key });
        input.value = x.value == null ? '' : String(x.value); let last = input.value;
        const commit = () => { if (input.value === last) return; last = input.value; if (x.onCommit) x.onCommit(input.value); };
        input.addEventListener('change', commit); input.addEventListener('keydown', ev => { if (ev.key === 'Enter') { ev.preventDefault(); commit(); } });
        const wrap = el('div', { class: 'hd-f-num' }, [input, x.unit ? el('span', { class: 'hd-f-unit', 'aria-hidden': 'true', text: x.unit }) : null]);
        return { el: input, wrap, set(v) { input.value = v == null ? '' : String(v); last = input.value; }, focus() { input.focus(); } };
    }
    function select(o) {
        const x = o || {}, sel = el('select', { class: 'hd-f-select', 'aria-label': x.label, 'data-hd-field': x.key });
        const fill = (list, value) => { sel.replaceChildren(); for (const it of list || []) sel.appendChild(el('option', { value: it.id, disabled: it.disabled ? true : null, text: it.label })); sel.value = value == null ? '' : String(value); };
        fill(x.options, x.value); sel.addEventListener('change', () => { if (x.onChange) x.onChange(sel.value); });
        const wrap = el('div', { class: 'hd-f-selectwrap' }, [sel, ico('chevron-down')]);
        return { el: sel, wrap, set(v) { sel.value = v == null ? '' : String(v); }, options: fill, focus() { sel.focus(); } };
    }
    function toggle(o) {
        const x = o || {}, b = el('button', { type: 'button', class: 'hd-f-switch', role: 'switch', 'aria-checked': x.value === true ? 'true' : 'false', 'aria-label': x.label, 'data-hd-field': x.key }, [el('span', { class: 'hd-f-switch-track', 'aria-hidden': 'true' }, el('span', { class: 'hd-f-switch-thumb' }))]);
        b.addEventListener('click', () => { const on = b.getAttribute('aria-checked') !== 'true'; b.setAttribute('aria-checked', on ? 'true' : 'false'); if (x.onChange) x.onChange(on); });
        return { el: b, set(v) { b.setAttribute('aria-checked', v === true ? 'true' : 'false'); }, focus() { b.focus(); } };
    }
    function segmented(o) {
        const x = o || {}, root = el('div', { class: 'hd-f-seg', role: 'radiogroup', 'aria-label': x.label, 'aria-labelledby': x.labelledBy, 'data-hd-field': x.key });
        let cur = x.value == null ? '' : String(x.value);
        const btns = (x.options || []).map(opt => { const iconOnly = !!opt.iconOnly, b = el('button', { type: 'button', class: 'hd-f-seg-opt', role: 'radio', 'data-value': String(opt.id), 'aria-checked': 'false', tabindex: '-1',
            'aria-label': iconOnly ? opt.label : null, title: iconOnly ? opt.label : null }, [opt.icon ? ico(opt.icon) : null, iconOnly ? null : el('span', { text: opt.label })]); root.appendChild(b); return b; });
        const paint = () => { const on = btns.findIndex(b => b.getAttribute('data-value') === cur); btns.forEach((b, i) => { b.setAttribute('aria-checked', i === on ? 'true' : 'false'); b.setAttribute('tabindex', i === (on < 0 ? 0 : on) ? '0' : '-1'); }); };
        const choose = i => { const b = btns[i]; if (!b) return; cur = b.getAttribute('data-value'); paint(); b.focus(); if (x.onChange) x.onChange(cur); };
        root.addEventListener('click', ev => { const b = hit(ev, '.hd-f-seg-opt'); if (b && btns.includes(b)) choose(btns.indexOf(b)); });
        root.addEventListener('keydown', ev => { const at = btns.findIndex(b => b === ev.target || (ev.target && b.contains && b.contains(ev.target))), n = roving(ev.key, Math.max(0, at), btns.length); if (n === null || at < 0) return; ev.preventDefault(); choose(n); });
        paint(); return { el: root, set(v) { cur = v == null ? '' : String(v); paint(); }, focus() { const b = btns.find(y => y.getAttribute('tabindex') === '0') || btns[0]; if (b) b.focus(); } };
    }
    const parseHex = v => { const m = /^#?([0-9a-f]{6})$/i.exec(String(v == null ? '' : v).trim()); return m ? '#' + m[1].toLowerCase() : null; };
    function colour(o) {
        const x = o || {}, sw = el('input', { type: 'color', class: 'hd-f-swatch', 'aria-label': x.label, 'data-hd-field': x.key ? x.key + ':swatch' : null });
        const hex = el('input', { type: 'text', class: 'hd-f-input hd-f-hex', maxlength: '7', spellcheck: 'false', autocomplete: 'off', 'aria-label': x.label, placeholder: x.placeholder, 'data-hd-field': x.key });
        const root = el('div', { class: 'hd-f-colour' }, [sw, hex]);
        const paint = v => { const c = parseHex(v); hex.value = c || ''; sw.value = c || '#000000'; setAttr(root, 'data-hd-empty', c ? null : 'true'); return c; };
        sw.addEventListener('input', () => { const c = paint(sw.value); if (c && x.onInput) x.onInput(c); }); sw.addEventListener('change', () => { const c = paint(sw.value); if (c && x.onCommit) x.onCommit(c); });
        const fromText = () => { const v = hex.value.trim(); if (!v) { if (x.allowEmpty) { paint(''); if (x.onCommit) x.onCommit(''); } else if (x.onInvalid) x.onInvalid(v); return; }
            const c = parseHex(v); if (c) { paint(c); if (x.onCommit) x.onCommit(c); } else if (x.onInvalid) x.onInvalid(v); };
        hex.addEventListener('change', fromText); hex.addEventListener('keydown', ev => { if (ev.key === 'Enter') { ev.preventDefault(); fromText(); } });
        if (x.allowEmpty) root.appendChild(button({ label: x.clearLabel || '', icon: 'close', iconOnly: true, tone: 'quiet', onClick: () => { paint(''); if (x.onCommit) x.onCommit(''); } }).el);
        paint(x.value); return { el: hex, wrap: root, set: paint, focus() { hex.focus(); } };
    }
    function range(o) {
        const x = o || {}, input = el('input', { type: 'range', class: 'hd-f-range', min: x.min, max: x.max, step: x.step, 'aria-label': x.label, 'data-hd-field': x.key }), out = el('output', { class: 'hd-f-out', 'aria-hidden': 'true' });
        const sync = () => { const v = Number(input.value), txt = x.valueText ? x.valueText(v) : String(v); out.textContent = txt; setAttr(input, 'aria-valuetext', txt); };
        input.value = String(x.value == null ? x.min : x.value); sync();
        input.addEventListener('input', () => { sync(); if (x.onInput) x.onInput(Number(input.value)); }); input.addEventListener('change', () => { sync(); if (x.onCommit) x.onCommit(Number(input.value)); });
        return { el: input, wrap: el('div', { class: 'hd-f-rangewrap' }, [x.track || null, input, out]), set(v) { input.value = String(v); sync(); }, focus() { input.focus(); } };
    }
    function tabs(o) {
        const x = o || {}, base = uid('hd-tab'), root = el('div', { class: 'hd-f-tabs', role: 'tablist', 'aria-label': x.label });
        let cur = x.value; const idOf = id => base + '-' + id;
        const btns = (x.tabs || []).map(t => { const b = el('button', { type: 'button', class: 'hd-f-tab', role: 'tab', id: idOf(t.id), 'data-tab': t.id, 'aria-selected': 'false', 'aria-controls': x.controls, tabindex: '-1', text: t.label }); root.appendChild(b); return b; });
        const paint = () => btns.forEach(b => { const on = b.getAttribute('data-tab') === cur; b.setAttribute('aria-selected', on ? 'true' : 'false'); b.setAttribute('tabindex', on ? '0' : '-1'); });
        const choose = (i, focus) => { const b = btns[i]; if (!b) return; cur = b.getAttribute('data-tab'); paint(); if (focus) b.focus(); if (x.onChange) x.onChange(cur); };
        root.addEventListener('click', ev => { const b = hit(ev, '.hd-f-tab'); if (b && btns.includes(b)) choose(btns.indexOf(b), false); });
        root.addEventListener('keydown', ev => { if (!HORIZONTAL.includes(ev.key)) return; const at = btns.findIndex(b => b === ev.target), n = roving(ev.key, Math.max(0, at), btns.length); if (n === null || at < 0) return; ev.preventDefault(); choose(n, true); });
        paint(); return { el: root, set(v) { cur = v; paint(); }, idOf, focus() { const b = btns.find(y => y.getAttribute('tabindex') === '0') || btns[0]; if (b) b.focus(); } };
    }
    function button(o) {
        const x = o || {}, iconOnly = !!x.iconOnly, b = el('button', { type: 'button', class: 'hd-btn', 'data-tone': x.tone || 'default', 'data-hd-icon-only': iconOnly ? 'true' : null, 'aria-label': iconOnly ? x.label : null, title: iconOnly ? (x.title || x.label) : x.title },
            [x.icon ? ico(x.icon) : null, iconOnly ? null : el('span', { text: x.label || '' })]);
        b.addEventListener('click', ev => { if (b.getAttribute('aria-disabled') === 'true') return; if (x.onClick) x.onClick(ev); });
        return { el: b, set(v) { b.setAttribute('aria-disabled', v === false ? 'true' : 'false'); }, focus() { b.focus(); } };
    }
    function section(title, children) {
        const id = uid('hd-sec'), s = el('section', { class: 'hd-f-sec', 'aria-labelledby': id }, [el('h3', { class: 'hd-f-sec-title', id, text: title })]);
        for (const c of [].concat(children || [])) if (c) s.appendChild(c.el || c); return s;
    }
    // Searchable icon list: one aria-pressed button per E.icons.choices() name, labelled with its translated name (homedash.icon.<name>), the search
    // matches that label and the id. Arrow keys move a roving tabindex over the visible buttons (6 columns).
    function iconGrid(o) {
        const x = o || {}, t = x.t, label = n => (n ? t('homedash.icon.' + n) : t('homedash.inspector.icon_auto')), names = (x.allowAuto ? [''] : []).concat(E.icons.choices());
        const root = el('div', { class: 'hd-icongrid' }), list = el('div', { class: 'hd-icongrid-list', role: 'group', 'aria-label': x.label }), none = el('p', { class: 'hd-f-help', hidden: true, text: t('homedash.inspector.icon_none') });
        const search = text({ key: 'icon-search', search: true, max: 40, label: t('homedash.inspector.icon_search'), placeholder: t('homedash.inspector.icon_search'), onInput: v => filter(v) });
        const opts = names.map(n => { const b = el('button', { type: 'button', class: 'hd-icongrid-opt', 'aria-pressed': 'false', title: label(n), 'aria-label': label(n), 'data-icon': n, tabindex: '-1' }, ico(n || 'mode-auto')); list.appendChild(b); return b; });
        const keyOf = b => fold(label(b.getAttribute('data-icon')) + ' ' + b.getAttribute('data-icon')), visible = () => opts.filter(b => !b.hasAttribute('hidden'));
        let cur = x.value || '';
        const paint = () => { opts.forEach(b => b.setAttribute('aria-pressed', b.getAttribute('data-icon') === cur ? 'true' : 'false')); const v = visible(), on = v.find(b => b.getAttribute('aria-pressed') === 'true') || v[0];
            opts.forEach(b => b.setAttribute('tabindex', b === on ? '0' : '-1')); };
        function filter(q) { const k = fold(q).trim().split(/\s+/).filter(Boolean); opts.forEach(b => setAttr(b, 'hidden', k.every(w => keyOf(b).includes(w)) ? null : true)); setAttr(none, 'hidden', visible().length ? true : null); paint(); }
        list.addEventListener('click', ev => { const b = hit(ev, '.hd-icongrid-opt'); if (!b) return; cur = b.getAttribute('data-icon'); paint(); if (x.onPick) x.onPick(cur); });
        list.addEventListener('keydown', ev => { const v = visible(), at = v.findIndex(b => b === ev.target); if (at < 0) return; if (ev.key === 'Enter') { ev.preventDefault(); cur = v[at].getAttribute('data-icon'); paint(); if (x.onPick) x.onPick(cur); return; }
            const n = gridMove(ev.key, at, [v.length], 6); if (n === null) return; ev.preventDefault(); v[n].focus(); opts.forEach(b => b.setAttribute('tabindex', b === v[n] ? '0' : '-1')); if (typeof v[n].scrollIntoView === 'function') v[n].scrollIntoView({ block: 'nearest' }); });
        root.append(search.el, list, none); paint();
        return { el: root, set(v) { cur = v || ''; paint(); }, focus() { search.focus(); }, search };
    }
    // Opens a form in the designer's or app's E.sheet with no built-in actions and adds its own .hd-dlg-btn buttons (E.sheet does not auto-close
    // them), so a form can refuse to close while invalid: onAction(id) === false keeps it open.
    function formSheet(sheet, o) {
        const done = sheet.open({ title: o.title, text: [], actions: [], kind: o.kind || 'form' });
        const body = sheet.el.querySelector('.hd-sheet-body'), foot = sheet.el.querySelector('.hd-sheet-foot');
        body.appendChild(o.content);
        for (const a of o.actions || []) foot.appendChild(el('button', { type: 'button', class: 'hd-dlg-btn', 'data-action': a.id, 'data-tone': a.tone || 'default', text: a.label }));
        const onClick = ev => { const b = hit(ev, '.hd-dlg-btn'); if (!b) return; const id = b.getAttribute('data-action'); if (!o.onAction || o.onAction(id) !== false) sheet.close(id); };
        foot.addEventListener('click', onClick);
        const first = typeof o.first === 'function' ? o.first() : o.first; if (first && typeof first.focus === 'function') first.focus();
        return done.then(v => { foot.removeEventListener('click', onClick); return v; });
    }
    E.form = Object.freeze({ roving, gridMove, runes, fold, textIssue, field, text, number, select, toggle, segmented, colour, range, tabs, button, section, iconGrid, formSheet });
})();
```
  Builder contract (the file is authoritative; names, classes and events are styled by E1):
  - `field({label, help, required, control, wrap, inline})` → `{el, control, error(text)}`. Structure: `div.hd-f[data-hd-inline]` > `label.hd-f-label[for]` (span text + optional `span.hd-f-req`), then `wrap || control`, `p.hd-f-help[id]`, `p.hd-f-error[id][hidden]`. `error()` toggles `hidden` and `aria-invalid`, and keeps `aria-describedby` = help id plus the error id while it is shown.
  - `text({key, value, placeholder, label, max, search, select, onInput, onCommit, onBlur, onEnter})` builds `input.hd-f-input[type=text|search][spellcheck=false][autocomplete=off][data-hd-field]`. `onInput` is skipped while `isComposing` and fires once on `compositionend`. `change` → `onCommit(runes(value, max))`. Enter → `preventDefault` + `onEnter`.
  - `number({key, value, min, max, step, unit, placeholder, label, onCommit(raw)})` builds `div.hd-f-num` > `input.hd-f-input[type=number][inputmode]` + `span.hd-f-unit[aria-hidden]`. It commits on `change` and on Enter, de-duplicated by the last committed value. The unit also goes into the field label (`"History span (h)"`).
  - `select({key, options:[{id,label,disabled}], value, label, onChange})` builds `div.hd-f-selectwrap` > `select.hd-f-select` + chevron `hd-ico`. Extra method `options(list, value)`.
  - `toggle({key, value, label, onChange})` builds `button.hd-f-switch[type=button][role=switch][aria-checked]` > `span.hd-f-switch-track[aria-hidden]` > `span.hd-f-switch-thumb`. A click flips `aria-checked` and calls back.
  - `segmented({key, options:[{id,label,icon,iconOnly}], value, label|labelledBy, onChange})` builds `div.hd-f-seg[role=radiogroup]` > `button.hd-f-seg-opt[role=radio][aria-checked][tabindex][data-value]`. Arrow keys (`roving`) move focus and select; values are compared as strings.
  - `colour({key, value, label, allowEmpty, clearLabel, placeholder, onInput, onCommit, onInvalid})` builds `div.hd-f-colour[data-hd-empty]` > `input[type=color].hd-f-swatch` (input → `onInput(lower hex)`, change → `onCommit`), `input.hd-f-input.hd-f-hex[maxlength=7]` (change/Enter parses `^#?[0-9a-f]{6}$`i to lower-case; empty → `''` when `allowEmpty`, else `onInvalid`) and, with `allowEmpty`, a quiet clear `.hd-btn`.
  - `range({key, min, max, step, value, label, valueText, track, onInput, onCommit})` builds `div.hd-f-rangewrap` > optional `track` element + `input[type=range].hd-f-range[aria-valuetext]` + `output.hd-f-out[aria-hidden]`.
  - `tabs({tabs:[{id,label}], value, label, controls, onChange})` builds `div.hd-f-tabs[role=tablist]` > `button.hd-f-tab[role=tab][id][aria-selected][aria-controls][tabindex][data-tab]`. It uses automatic activation: Left/Right/Home/End move focus and select; Up and Down are ignored. Extra method `idOf(id)`.
  - `button({label, icon, tone:'default'|'primary'|'quiet'|'danger-quiet', iconOnly, title, onClick})` builds `button.hd-btn[type=button][data-tone][data-hd-icon-only]`. Icon-only buttons get `aria-label` and `title`. `onClick` is skipped while `aria-disabled=true`.
  - `section(title, children)` builds `section.hd-f-sec[aria-labelledby]` > `h3.hd-f-sec-title`.
  - `iconGrid({t, value, label, allowAuto, onPick(name|'')})` builds `div.hd-icongrid` with a search `text` (`homedash.inspector.icon_search`), a `div.hd-icongrid-list[role=group][aria-label]` of `button.hd-icongrid-opt[aria-pressed][title][aria-label][data-icon]` over `E.icons.choices()` (first, when `allowAuto`, a `mode-auto` button with `data-icon=""`, labelled `homedash.inspector.icon_auto`) and `p.hd-f-help[hidden]` (`homedash.inspector.icon_none`) when nothing matches. **Q10:** `title` and `aria-label` are `t('homedash.icon.' + name)`, the search matches that label and the id (accent-folded), `data-icon` is the id. Arrows move a roving tabindex over the visible buttons (`gridMove(key, i, [visible.length], 6)`); Enter or a click calls `onPick`.
  - `formSheet(sheet, {title, content, actions:[{id,label,tone}], onAction(id) → false keeps it open, first})` opens the sheet with no built-in actions, appends its own `.hd-dlg-btn` buttons to the foot and closes with the action id.
  - `textIssue(s)` → `''` | `control_chars` | `url_not_allowed` (addendum; reading 14); `fold(s)` (reading 16).
- [ ] `hd-samples.js`, in full:
<!-- embed: dryrun-m4/ui/js/homedash/hd-samples.js -->
```js
// HomeDash sample states (M4 part 2, D5): one sample entity view per registered widget type (the values of ui/testdata/homedash-lab/*-fixtures.js,
// names through literal keys), deterministic history, and thumb(): a widget host in 'thumb' mode at its thumb size on 56 px cells, scaled into a
// fixed box with --hd-e-tw/-th/-ts. A thumb never sends a command (thumb mode, can() false) and never subscribes; entity(binding) may give a real view.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.widgets || !E.catalog) throw new Error('hd-samples.js: load hd-widgets.js and hd-catalog.js first');
    const { el, setVars, clamp, uid } = E.core, DEG_C = String.fromCodePoint(0xb0) + 'C', CELL = 56, GAP = 8;
    const cap = (name, extra) => Object.freeze(Object.assign({ name, allowed: true }, extra || {}));
    function view(id, kind, name, extra) {
        const x = extra || {};
        return Object.freeze(Object.assign({ id, source: 'sample', kind, name, area: '', unit: '', device_class: '', icon_hint: '', value: null, text: '', available: true, status: 'ok', readonly: false,
            readonlyReason: '', serverReadonly: false, capabilities: [], pending: false, stale: false, error: null, tone: null }, x, { attributes: Object.freeze(Object.assign({ updated_at: new Date().toISOString() }, x.attributes)) }));
    }
    const TABLE = Object.freeze({
        switch: t => ({ primary: view('sample:switch', 'switch', t('homedash.palette.sample_lamp'), { value: 'on', icon_hint: 'power', capabilities: [cap('toggle'), cap('turn_on'), cap('turn_off')] }) }),
        light: t => ({ primary: view('sample:light', 'light', t('homedash.palette.sample_light'), { value: 'on', icon_hint: 'bulb', attributes: { brightness_pct: 64, color_mode: 'brightness', supported_color_modes: ['brightness'] },
            capabilities: [cap('toggle'), cap('turn_on'), cap('turn_off'), cap('brightness', { min: 0, max: 100, step: 1 })] }) }),
        action: t => ({ primary: view('sample:scene', 'scene', t('homedash.palette.sample_scene'), { value: '2026-11-24T20:15:00+00:00', icon_hint: 'scene', capabilities: [cap('activate')] }) }),
        select: t => { const options = [t('homedash.palette.sample_low'), t('homedash.palette.sample_mid'), t('homedash.palette.sample_high')];
            return { primary: view('sample:select', 'select', t('homedash.palette.sample_mode'), { value: options[1], icon_hint: 'list', attributes: { options }, capabilities: [cap('select_option', { options })] }) }; },
        number: t => ({ primary: view('sample:number', 'number', t('homedash.palette.sample_volume'), { value: 65, icon_hint: 'number', attributes: { min: 0, max: 100, step: 5, mode: 'slider' }, capabilities: [cap('set_value', { min: 0, max: 100, step: 5 })] }) }),
        climate: t => ({ primary: view('sample:climate', 'climate', t('homedash.palette.sample_room'), { value: 'heat', unit: DEG_C, attributes: { current_temperature: 19.5, temperature: 21.5, min_temp: 5, max_temp: 30, target_temp_step: 0.5,
            hvac_action: 'heating', hvac_modes: ['off', 'heat', 'cool', 'auto'] }, capabilities: [cap('set_temperature', { min: 5, max: 30, step: 0.5 }), cap('hvac_mode', { options: ['off', 'heat', 'cool', 'auto'] })] }) }),
        gauge: t => ({ primary: view('sample:gauge', 'sensor', t('homedash.palette.sample_cpu'), { value: 42, unit: '%', capabilities: [cap('numeric'), cap('history')] }) }),
        temperature: t => ({ primary: view('sample:temperature', 'sensor', t('homedash.palette.sample_room'), { value: 21.4, unit: DEG_C, device_class: 'temperature', capabilities: [cap('numeric'), cap('history')] }),
            humidity: view('sample:humidity', 'sensor', t('homedash.palette.sample_humidity'), { value: 48, unit: '%', device_class: 'humidity', capabilities: [cap('numeric')] }) }),
        sensor: t => ({ primary: view('sample:sensor', 'sensor', t('homedash.palette.sample_power'), { value: 412, unit: 'W', device_class: 'power', icon_hint: 'plug', capabilities: [cap('numeric'), cap('history')] }) }),
        entity: t => ({ primary: view('sample:entity', 'sensor', t('homedash.palette.sample_outside'), { value: 7.5, unit: DEG_C, device_class: 'temperature', capabilities: [cap('numeric')] }) })
    });
    const SERIES = Object.freeze({ sensor: [380, 220, 120], gauge: [42, 15, 6], temperature: [21.2, 1.4, 0.3] });
    const entities = (type, t) => (TABLE[type] ? TABLE[type](t) : { primary: view('sample:' + type, 'generic', t('homedash.widget.' + type + '.name')) });
    function history(type, now) { const p = SERIES[type], end = Number.isFinite(now) ? now : Date.now(); if (!p) return { points: [], source: 'none' }; const points = [];
        for (let i = 0; i < 48; i++) points.push([end - (47 - i) * 1800000, Math.round((p[0] + p[1] * Math.sin(i / 48 * Math.PI * 2 - 1.9) + p[2] * Math.sin(i * 0.9)) * 10) / 10]);
        return { points, source: 'ring' }; }
    // Thumb size: the variant's default size capped to 2x2 for legibility, never below its minimum.
    function thumbSize(catalog, type, variant) { const v = catalog.variant(type, variant) || {}, d = v.default_size || { w: 2, h: 1 }, m = v.min_size || { w: 1, h: 1 };
        return { w: clamp(d.w, m.w, Math.max(m.w, 2)), h: clamp(d.h, m.h, Math.max(m.h, 2)) }; }
    const pxOf = s => ({ width: s.w * CELL + (s.w - 1) * GAP, height: s.h * CELL + (s.h - 1) * GAP });
    const scaleFor = (px, box) => Math.round(Math.min(1, box.width / px.width, box.height / px.height) * 1000) / 1000;
    function thumb(o) {
        const w = o.widget, size = o.size || thumbSize(o.catalog, w.type, w.variant), px = pxOf(size), samples = entities(w.type, o.t);
        const node = el('div', { class: 'hd-thumb', 'aria-hidden': 'true' }); setVars(node, { '--hd-e-tw': px.width + 'px', '--hd-e-th': px.height + 'px', '--hd-e-ts': scaleFor(px, o.box) });
        const entity = b => (typeof o.entity === 'function' && o.entity(b)) || samples[b] || null;
        const api = () => ({ widget: w, t: o.t, lang: o.lang, entity, can: () => false, command: () => Promise.resolve({ status: 'refused' }), moreInfo: () => false, history: () => Promise.resolve(history(w.type)), announce: () => {} });
        let host = null;
        try { host = E.widgets.host({ widget: Object.assign({ id: uid('hd-thumb') }, w), mode: 'thumb', size, t: o.t, lang: o.lang, parent: node, api }); host.resize({ w: size.w, h: size.h, px }); }
        catch (err) { host = null; node.replaceChildren(E.icons.icon('entity')); node.setAttribute('data-hd-thumb', 'none'); }
        if (o.parent) o.parent.appendChild(node);
        return Object.freeze({ el: node, update() { if (host) host.update(); }, destroy() { if (host) host.destroy(); host = null; node.remove(); } });
    }
    E.samples = Object.freeze({ CELL, GAP, entities, history, thumbSize, pxOf, scaleFor, thumb });
})();
```
- [ ] The tests of C1 are the first five of `palette.test.mjs` (embedded in C2): grid moves, thumb size and inertness, segmented/field/text behaviour, the translated icon grid, the form sheet that stays open.
- [ ] Wiring step W for both files (with C2). Commit with C2.

### Task C2: hd-palette.js (widgets and devices)
**Files:** create `hd-palette.js`; the tests are `designer-kit.mjs` and `palette.test.mjs` (one commit with C1). Wiring step W. **Existing symbols changed:** none.
- [ ] `hd-palette.js`, in full (model and DOM):
<!-- embed: dryrun-m4/ui/js/homedash/hd-palette.js -->
```js
// HomeDash palette (M4 part 2, F4b; D5): the Widgets tab (catalog types the registry implements, live sample thumbs, search, grid keys) and the Devices
// tab (GET entities with q/source/kind/area, paging 200, live state of at most 20 visible rows). Pure models on E.palette.model; create() builds the
// DOM into the C15 shell. Drag goes through D.gestures.dragFrom, tap/Enter through D.addWidgets. Untrusted names only via textContent.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.form || !E.samples || !E.commands || !E.catalog) throw new Error('hd-palette.js: load hd-form.js, hd-samples.js and hd-commands.js first');
    const { el, setAttr, uid } = E.core, DOT = ' ' + String.fromCodePoint(0xb7) + ' ', fold = E.form.fold;
    const ENT_LIMIT = 200, WATCH_MAX = 20, SEARCH_MS = 250, COLS = 2, THUMB_BOX = Object.freeze({ width: 116, height: 76 }), FILTERS = Object.freeze(['q', 'source', 'kind', 'area']);
    const TYPE_ICONS = Object.freeze({ switch: 'power', light: 'bulb', action: 'scene', select: 'list', number: 'number', climate: 'thermometer', temperature: 'thermometer', gauge: 'gauge', sensor: 'chart', entity: 'entity' });
    // Widget cards for a search: every whitespace-separated token must appear (case- and accent-insensitively) in the type name, variant name,
    // description, category name, type id or variant id. Catalog order kept; empty groups dropped.
    function filterGroups(groups, query, names) {
        const tokens = fold(query).split(/\s+/).filter(Boolean), out = [];
        for (const g of groups || []) { const items = [];
            for (const x of g.types || []) for (const v of x.variants || []) { const n = names(x.type, v, g.category) || {};
                if (tokens.every(k => fold([n.type, n.variant, n.desc, n.category, x.type, v].join(' ')).includes(k))) items.push(Object.freeze({ type: x.type, variant: v })); }
            if (items.length) out.push(Object.freeze({ category: g.category, items: Object.freeze(items) })); }
        return Object.freeze(out);
    }
    const entInit = () => Object.freeze({ q: '', source: '', kind: '', area: '', items: Object.freeze([]), total: 0, offset: 0, loading: false, error: '', seq: 0, sources: Object.freeze([]), areas: Object.freeze([]) });
    const nx = (s, p) => Object.freeze(Object.assign({}, s, p));
    // Entity list state. filter resets the list (seq+1); more asks the next page; loaded/failed of an older seq are ignored; a page with no new
    // rows ends paging; areas accumulate across filters so the area select keeps its options.
    function entReduce(s, a) {
        switch (a && a.type) {
            case 'filter': { const f = {}; for (const k of FILTERS) f[k] = typeof a[k] === 'string' ? (k === 'q' ? Array.from(a[k].trim()).slice(0, 100).join('') : a[k]) : s[k];
                if (!a.force && s.seq > 0 && FILTERS.every(k => f[k] === s[k])) return s;
                return nx(s, Object.assign(f, { items: Object.freeze([]), total: 0, offset: 0, loading: true, error: '', seq: s.seq + 1 })); }
            case 'more': return s.loading || s.error || s.items.length >= s.total ? s : nx(s, { offset: s.items.length, loading: true, seq: s.seq + 1 });
            case 'retry': return nx(s, { loading: true, error: '', seq: s.seq + 1 });
            case 'loaded': { if (a.seq !== s.seq) return s; const ans = a.answer || {}, seen = new Set(s.offset ? s.items.map(e => e.id) : []);
                const fresh = (Array.isArray(ans.entities) ? ans.entities : []).filter(e => e && typeof e.id === 'string' && !seen.has(e.id) && seen.add(e.id));
                const items = (s.offset ? s.items : []).concat(fresh), total = fresh.length ? Math.max(items.length, Number(ans.total) || 0) : items.length;
                const areas = [...new Set(s.areas.concat(items.map(e => (typeof e.area === 'string' ? e.area : '')).filter(Boolean)))].sort();
                const sources = Array.isArray(ans.sources) ? [...new Set(ans.sources.map(x => (x && typeof x.source === 'string' ? x.source : '')).filter(Boolean))] : s.sources;
                return nx(s, { items: Object.freeze(items), total, loading: false, error: '', areas: Object.freeze(areas), sources: Object.freeze(sources) }); }
            case 'failed': return a.seq !== s.seq ? s : nx(s, { loading: false, error: a.code || 'network' });
            default: return s;
        }
    }
    const entRequest = s => ({ q: s.q || undefined, source: s.source || undefined, kind: s.kind || undefined, area: s.area || undefined, limit: ENT_LIMIT, offset: s.offset });
    const watchIds = (order, visible, max) => order.filter(id => visible.has(id)).slice(0, max || WATCH_MAX);
    const kindsOf = catalog => [...new Set(catalog.defaults().map(d => d.kind))];
    const sourceName = (t, src) => (src === 'system' ? t('homedash.source.system') : src === 'ha' ? t('homedash.source.ha') : String(src || ''));
    function stateText(t, lang, v) { if (!v || ['scene', 'script', 'button'].includes(v.kind)) return ''; const w = E.format.stateWord(v); if (w !== 'value') return t(E.format.STATE_PREFIX + w);
        return typeof v.value === 'number' ? E.format.valueText(v.value, { lang, unit: v.unit }) : v.text || (v.value == null ? E.format.DASH : String(v.value)); }
    function listView(e) { const s = e && e.state && typeof e.state === 'object' ? e.state : {};
        return { kind: e.kind, unit: e.unit || '', icon_hint: e.icon_hint || '', value: s.value === undefined ? null : s.value, text: typeof s.text === 'string' ? s.text : '', available: s.available !== false, status: 'ok', pending: false }; }

    function create(o) {
        const D = o.designer, t = o.t, lang = o.lang || 'en', catalog = o.catalog, store = o.store, api = o.api, life = E.core.bag(), ids = { w: uid('hd-palw'), e: uid('hd-pale'), hw: uid('hd-palhw'), he: uid('hd-palhe') };
        const timers = o.timers || { setTimeout: (f, ms) => setTimeout(f, ms), clearTimeout: x => clearTimeout(x) }, collator = new Intl.Collator(lang), root = el('div', { class: 'hd-pal' }), thumbs = [];
        const typeName = x => t('homedash.widget.' + x + '.name'), variantName = (x, v) => t('homedash.variant.' + x + '.' + v);
        const names = (type, v, cat) => ({ type: typeName(type), variant: variantName(type, v), desc: t('homedash.widget.' + type + '.desc'), category: t('homedash.category.' + cat) });
        let tab = 'widgets', ent = entInit(), entStarted = false, timer = null, queryW = '', queryE = '', destroyed = false;
        // head
        const tabsCtl = E.form.tabs({ tabs: [{ id: 'widgets', label: t('homedash.palette.tab_widgets') }, { id: 'entities', label: t('homedash.palette.tab_entities') }], value: tab, label: t('homedash.designer.palette'), onChange: id => setTab(id) });
        const search = E.form.text({ key: 'palette-search', search: true, max: 100, onInput: v => onSearch(v) });
        const clearBtn = E.form.button({ label: t('homedash.palette.clear_search'), icon: 'close', iconOnly: true, tone: 'quiet', onClick: () => { search.set(''); onSearch(''); search.focus(); } });
        setAttr(clearBtn.el, 'hidden', true);
        root.appendChild(el('div', { class: 'hd-pal-head' }, [tabsCtl.el, el('div', { class: 'hd-pal-search' }, [E.icons.icon('search'), search.el, clearBtn.el])]));
        // widgets panel
        const panelW = el('div', { class: 'hd-pal-panel', role: 'tabpanel', id: ids.w, 'aria-labelledby': tabsCtl.idOf('widgets') }), statusW = el('p', { class: 'hd-pal-status', hidden: true, role: 'status' });
        panelW.appendChild(el('p', { class: 'hd-sr-only', id: ids.hw, text: t('homedash.palette.widget_hint') }));
        const groups = catalog.paletteGroups(E.widgets), cards = new Map(), groupEls = [];
        for (const g of groups) {
            const hid = uid('hd-palg'), grid = el('div', { class: 'hd-pal-grid' }), sec = el('section', { class: 'hd-pal-group', 'aria-labelledby': hid }, [el('h3', { class: 'hd-pal-group-title', id: hid, text: t('homedash.category.' + g.category) }), grid]);
            groupEls.push({ category: g.category, el: sec, grid, keys: [] });
            for (const x of g.types) for (const v of x.variants) {
                const n = names(x.type, v, g.category), box = el('div', { class: 'hd-pal-thumb' }), key = x.type + '/' + v;
                const hitBtn = el('button', { type: 'button', class: 'hd-pal-hit', tabindex: '-1', 'aria-label': t('homedash.palette.card_label', { type: n.type, variant: n.variant }), 'aria-describedby': ids.hw, title: n.desc }, [el('span', { class: 'hd-pal-name', text: n.type }), el('span', { class: 'hd-pal-variant', text: n.variant })]);
                const card = el('div', { class: 'hd-pal-card', 'data-type': x.type, 'data-variant': v }, [box, hitBtn]); grid.appendChild(card); cards.set(key, card); groupEls[groupEls.length - 1].keys.push(key);
                thumbs.push(E.samples.thumb({ widget: { type: x.type, variant: v, title: '', bindings: {}, options: catalog.optionDefaults(x.type), actions: {}, confirm: false }, catalog, t, lang, box: THUMB_BOX, parent: box }));
            }
            panelW.appendChild(sec);
        }
        panelW.appendChild(statusW); root.appendChild(panelW);
        // devices panel
        const panelE = el('div', { class: 'hd-pal-panel', role: 'tabpanel', id: ids.e, 'aria-labelledby': tabsCtl.idOf('entities'), hidden: true });
        const selSource = E.form.select({ key: 'palette-source', label: t('homedash.palette.filter_source'), onChange: v => filter({ source: v }) }), selKind = E.form.select({ key: 'palette-kind', label: t('homedash.palette.filter_kind'), onChange: v => filter({ kind: v }) });
        const selArea = E.form.select({ key: 'palette-area', label: t('homedash.palette.filter_area'), onChange: v => filter({ area: v }) });
        const statusText = el('span', { class: 'hd-pal-status-text' }), retryBtn = E.form.button({ label: t('homedash.palette.retry'), tone: 'quiet', onClick: () => { ent = entReduce(ent, { type: 'retry' }); renderEnt(); load(); } });
        const statusE = el('p', { class: 'hd-pal-status', role: 'status', 'data-state': 'idle' }, [statusText, retryBtn.el]), list = el('ul', { class: 'hd-ent-list', 'aria-label': t('homedash.palette.tab_entities') });
        const moreBtn = E.form.button({ label: t('homedash.palette.load_more'), tone: 'quiet', onClick: () => more() }), moreItem = el('li', { class: 'hd-ent-moreitem', hidden: true }, moreBtn.el);
        panelE.append(el('p', { class: 'hd-sr-only', id: ids.he, text: t('homedash.palette.entity_hint') }), el('div', { class: 'hd-pal-filters' }, [selSource.wrap, selKind.wrap, selArea.wrap]), statusE, list); root.appendChild(panelE);
        const kindList = kindsOf(catalog).map(k => ({ id: k, label: t('homedash.kind.' + k) })).sort((a, b) => collator.compare(a.label, b.label));
        selKind.options([{ id: '', label: t('homedash.palette.all_kinds') }].concat(kindList), ''); selSource.options([{ id: '', label: t('homedash.palette.all_sources') }], ''); selArea.options([{ id: '', label: t('homedash.palette.all_areas') }], '');
        let shownSources = '', shownAreas = '';
        const rows = new Map(), order = [], visible = new Set(), byId = new Map();
        const watch = store.watch(uid('hd-pal'), { ids: [], active: true });
        const syncWatch = E.core.frame(() => { if (!destroyed) watch.set(tab === 'entities' ? watchIds(order, visible) : []); }, { raf: o.raf, caf: o.caf });
        const IO = typeof IntersectionObserver === 'function' ? IntersectionObserver : null;
        const rowObserver = IO ? new IO(list2 => { for (const en of list2) { const id = en.target.getAttribute('data-entity'); if (en.isIntersecting) visible.add(id); else visible.delete(id); } syncWatch(); }, { root: panelE, rootMargin: '64px' }) : null;
        const moreObserver = IO ? new IO(list2 => { if (list2.some(en => en.isIntersecting)) more(); }, { root: panelE, rootMargin: '64px' }) : null;
        if (moreObserver) moreObserver.observe(moreItem);

        function avatar(icon, title, sub) { return el('div', { class: 'hd-pal-avatar' }, [E.icons.icon(icon), el('span', { class: 'hd-pal-avatar-text' }, [el('strong', { text: title }), sub ? el('small', { text: sub }) : null])]); }
        function addEntry(entry, source) { if (!entry) return; if (D.state().drawers) D.closePanels(); D.addWidgets([entry], { source }); }
        function entryFor(e) { const pick = catalog.pickWidgetFor({ kind: e.kind, device_class: e.device_class, capabilities: e.capabilities }, E.widgets); return pick ? E.commands.entry(catalog, pick.type, pick.variant, e.id) : null; }
        // ---- widgets tab ----
        function applyWidgetSearch() {
            const shown = new Set(filterGroups(groups, queryW, names).flatMap(g => g.items.map(i => i.type + '/' + i.variant)));
            for (const [key, card] of cards) setAttr(card, 'hidden', shown.has(key) ? null : true);
            for (const g of groupEls) setAttr(g.el, 'hidden', g.keys.some(k => shown.has(k)) ? null : true);
            statusW.textContent = shown.size || !queryW.trim() ? '' : t('homedash.palette.no_widgets', { q: queryW.trim() }); setAttr(statusW, 'hidden', statusW.textContent ? null : true); roveW();
        }
        const hitsW = () => [...cards.values()].filter(c => !c.hasAttribute('hidden')).map(c => c.querySelector('.hd-pal-hit'));
        function roveW(focus) { const v = hitsW(), cur = v.find(b => b.getAttribute('tabindex') === '0' && v.includes(b)) || v[0]; for (const b of cards.values()) b.querySelector('.hd-pal-hit').setAttribute('tabindex', '-1'); if (cur) { cur.setAttribute('tabindex', '0'); if (focus) cur.focus(); } }
        life.on(panelW, 'click', ev => { const h = ev.target && ev.target.closest ? ev.target.closest('.hd-pal-hit') : null, card = h && h.closest('.hd-pal-card'); if (card) addEntry(E.commands.entry(catalog, card.getAttribute('data-type'), card.getAttribute('data-variant')), 'palette'); });
        life.on(panelW, 'pointerdown', ev => { const card = ev.target && ev.target.closest ? ev.target.closest('.hd-pal-card') : null; if (!card || (ev.button !== undefined && ev.button !== 0)) return;
            const type = card.getAttribute('data-type'), variant = card.getAttribute('data-variant'), entry = E.commands.entry(catalog, type, variant);
            D.gestures.dragFrom(ev, { kind: 'widget', label: typeName(type), entries: () => [entry], avatar: () => avatar(TYPE_ICONS[type] || 'dashboard', typeName(type), variantName(type, variant)) }); });
        life.on(panelW, 'keydown', ev => { const v = hitsW(), at = v.findIndex(b => b === ev.target); if (at < 0) return;
            const sizes = groupEls.map(g => g.keys.filter(k => !cards.get(k).hasAttribute('hidden')).length), n = E.form.gridMove(ev.key, at, sizes, COLS); if (n === null) return;
            ev.preventDefault(); for (const b of v) b.setAttribute('tabindex', b === v[n] ? '0' : '-1'); v[n].focus(); if (typeof v[n].scrollIntoView === 'function') v[n].scrollIntoView({ block: 'nearest' }); });
        // ---- devices tab ----
        function filter(patch) { ent = entReduce(ent, Object.assign({ type: 'filter' }, patch)); renderEnt(); load(); }
        function more() { const next = entReduce(ent, { type: 'more' }); if (next === ent) return; ent = next; renderEnt(); load(); }
        function load() {
            const seq = ent.seq; Promise.resolve().then(() => api.entities(entRequest(ent))).then(answer => { if (!destroyed) { ent = entReduce(ent, { type: 'loaded', seq, answer }); renderEnt(); } },
                err => { if (!destroyed) { ent = entReduce(ent, { type: 'failed', seq, code: err && err.code ? String(err.code) : 'network' }); renderEnt(); } });
        }
        function rowState(id) { const v = store.view(id), e = byId.get(id); return stateText(t, lang, v && v.status !== 'unknown' ? v : e ? listView(e) : null); }
        function buildRow(e) {
            const v = store.view(e.id) || listView(e), icon = E.widgets.iconFor(null, v) || 'entity', source = sourceName(t, e.source || String(e.id).split(':')[0]);
            const btn = el('button', { type: 'button', class: 'hd-ent-row', tabindex: '-1', 'data-entity': e.id, 'aria-describedby': ids.he }, [el('span', { class: 'hd-ent-icon' }, E.icons.icon(icon)),
                el('span', { class: 'hd-ent-text' }, [el('span', { class: 'hd-ent-name', text: String(e.name || e.id) }), el('span', { class: 'hd-ent-meta', text: [e.area, source].filter(Boolean).join(DOT) })]), el('span', { class: 'hd-ent-state', text: stateText(t, lang, v) })]);
            const li = el('li', { class: 'hd-ent-item' }, btn); if (rowObserver) rowObserver.observe(btn); return { li, btn };
        }
        function renderEnt() {
            if (destroyed) return;
            if (ent.items.length < order.length || (order.length && ent.items[0] && ent.items[0].id !== order[0])) { for (const r of rows.values()) { if (rowObserver) rowObserver.unobserve(r.btn); r.li.remove(); } rows.clear(); order.length = 0; byId.clear(); visible.clear(); }
            for (const e of ent.items) { if (rows.has(e.id)) continue; const r = buildRow(e); rows.set(e.id, r); order.push(e.id); byId.set(e.id, e); list.appendChild(r.li); if (!rowObserver && visible.size < WATCH_MAX) visible.add(e.id); }
            list.appendChild(moreItem); setAttr(moreItem, 'hidden', ent.items.length < ent.total && !ent.error ? null : true);
            const failed = !!ent.error, state = ent.loading ? 'loading' : failed ? 'failed' : !ent.items.length ? 'empty' : 'ready';
            statusText.textContent = state === 'loading' && !ent.items.length ? t('homedash.palette.loading') : state === 'failed' ? t('homedash.palette.failed') : state === 'empty' ? t('homedash.palette.empty')
                : ent.items.length < ent.total ? t('homedash.palette.count_of', { shown: ent.items.length, total: ent.total }) : t('homedash.palette.count', { count: ent.items.length });
            setAttr(statusE, 'data-state', state); setAttr(retryBtn.el, 'hidden', failed ? null : true);
            const srcKey = ent.sources.join(','), areaKey = ent.areas.join(',');
            if (srcKey !== shownSources) { shownSources = srcKey; selSource.options([{ id: '', label: t('homedash.palette.all_sources') }].concat(ent.sources.map(s => ({ id: s, label: sourceName(t, s) }))), ent.source); }
            if (areaKey !== shownAreas) { shownAreas = areaKey; selArea.options([{ id: '', label: t('homedash.palette.all_areas') }].concat(ent.areas.map(a => ({ id: a, label: a }))), ent.area); }
            roveE(); syncWatch();
        }
        const btnsE = () => [...rows.values()].map(r => r.btn);
        function roveE(focus) { const v = btnsE(), cur = v.find(b => b.getAttribute('tabindex') === '0') || v[0]; for (const b of v) b.setAttribute('tabindex', b === cur ? '0' : '-1'); if (cur && focus) cur.focus(); }
        life.on(list, 'click', ev => { const b = ev.target && ev.target.closest ? ev.target.closest('.hd-ent-row') : null, e = b && byId.get(b.getAttribute('data-entity')); if (e) addEntry(entryFor(e), 'entity'); });
        life.on(list, 'pointerdown', ev => { const b = ev.target && ev.target.closest ? ev.target.closest('.hd-ent-row') : null, e = b && byId.get(b.getAttribute('data-entity')); if (!e || (ev.button !== undefined && ev.button !== 0)) return;
            const entry = entryFor(e), icon = E.widgets.iconFor(null, listView(e)) || 'entity'; D.gestures.dragFrom(ev, { kind: 'entity', entity: e, label: String(e.name || e.id), entries: () => (entry ? [entry] : []), avatar: () => avatar(icon, String(e.name || e.id), e.area || '') }); });
        life.on(list, 'keydown', ev => { const v = btnsE(), at = v.findIndex(b => b === ev.target); if (at < 0) return; const n = E.form.roving(ev.key, at, v.length); if (n === null || ev.key === 'ArrowLeft' || ev.key === 'ArrowRight') return;
            ev.preventDefault(); for (const b of v) b.setAttribute('tabindex', b === v[n] ? '0' : '-1'); v[n].focus(); if (typeof v[n].scrollIntoView === 'function') v[n].scrollIntoView({ block: 'nearest' }); });
        life.add(store.on('change', changed => { if (tab !== 'entities') return; for (const [id, r] of rows) if (!changed || changed.has(id)) { const s = r.btn.querySelector('.hd-ent-state'); if (s) s.textContent = rowState(id); } }));
        // ---- head wiring ----
        function onSearch(v) {
            setAttr(clearBtn.el, 'hidden', v ? null : true);
            if (tab === 'widgets') { queryW = v; applyWidgetSearch(); return; }
            queryE = v; timers.clearTimeout(timer); timer = timers.setTimeout(() => { timer = null; filter({ q: queryE }); }, SEARCH_MS);
        }
        function setTab(next) {
            if (next !== 'widgets' && next !== 'entities') return; tab = next; tabsCtl.set(next);
            setAttr(panelW, 'hidden', next === 'widgets' ? null : true); setAttr(panelE, 'hidden', next === 'entities' ? null : true);
            const v = next === 'widgets' ? queryW : queryE; search.set(v); setAttr(clearBtn.el, 'hidden', v ? null : true);
            const label = t(next === 'widgets' ? 'homedash.palette.search_widgets' : 'homedash.palette.search_entities'); setAttr(search.el, 'aria-label', label); setAttr(search.el, 'placeholder', label); setAttr(search.el, 'aria-controls', next === 'widgets' ? ids.w : ids.e);
            for (const b of tabsCtl.el.querySelectorAll('.hd-f-tab')) setAttr(b, 'aria-controls', b.getAttribute('data-tab') === 'widgets' ? ids.w : ids.e);
            if (next === 'entities' && !entStarted) { entStarted = true; filter({ force: true, q: queryE }); } else syncWatch();
        }
        life.on(root, 'keydown', ev => {
            if (ev.key === 'ArrowDown' && ev.target === search.el) { const first = tab === 'widgets' ? hitsW()[0] : btnsE()[0]; if (first) { ev.preventDefault(); first.focus(); } return; }
            if (ev.key !== 'Escape') return;
            if (ev.target === search.el && search.get()) { ev.preventDefault(); ev.stopPropagation(); search.set(''); onSearch(''); return; }
            ev.preventDefault(); const s = D.state(); if (s.drawers) { D.closePanels(); return; } const id = s.focusId || s.primary || (s.items && s.items[0] && s.items[0].id); if (id) D.focusFrame(id);
        });
        setTab('widgets'); applyWidgetSearch(); if (o.host) o.host.appendChild(root);
        return Object.freeze({ el: root, focusSearch() { search.focus(); }, setTab, destroy() {
            if (destroyed) return; destroyed = true; timers.clearTimeout(timer); syncWatch.cancel(); if (rowObserver) rowObserver.disconnect(); if (moreObserver) moreObserver.disconnect(); life.dispose(); watch.release(); for (const th of thumbs) th.destroy(); thumbs.length = 0; root.remove(); } });
    }
    E.palette = Object.freeze({ create, model: Object.freeze({ fold, filterGroups, entInit, entReduce, entRequest, watchIds, kindsOf, sourceName, stateText, listView, TYPE_ICONS, ENT_LIMIT, WATCH_MAX }) });
})();
```
  (`E.palette.model` exports `fold, filterGroups, entInit, entReduce, entRequest, watchIds, kindsOf, sourceName, stateText, listView, TYPE_ICONS, ENT_LIMIT, WATCH_MAX`; `fold` is `E.form.fold`.) Behaviour notes of `create(o)` follow; the file is authoritative.
- [ ] `create(o)` structure. Everything is appended to `o.host` (the C15 `.hd-palette` shell) as `div.hd-pal`:
  - `div.hd-pal-head`: `E.form.tabs` (label `homedash.designer.palette`; tabs `widgets` / `entities` using `palette.tab_*`) and `div.hd-pal-search` > search icon + `E.form.text({search:true})` (placeholder, `aria-label` and `aria-controls` follow the tab) + a quiet icon-only clear `.hd-btn` (`palette.clear_search`, hidden while empty).
  - Widgets panel `div.hd-pal-panel[role=tabpanel][aria-labelledby]`:
    - `p.hd-sr-only#hintW` (`palette.widget_hint`).
    - Per group: `section.hd-pal-group[aria-labelledby]` > `h3.hd-pal-group-title` (`homedash.category.<cat>`) and `div.hd-pal-grid`.
    - Per variant: `div.hd-pal-card[data-type][data-variant]` > `div.hd-pal-thumb` (`E.samples.thumb` with `THUMB_BOX`) + `button.hd-pal-hit[type=button][tabindex][aria-label=palette.card_label][aria-describedby=hintW][title=widget.<type>.desc]` > `span.hd-pal-name` + `span.hd-pal-variant`. The thumb stays outside the button, so no `div` sits inside a `button`.
    - `p.hd-pal-status[hidden]` (`palette.no_widgets`).
    - Cards and thumbs are built once. Search toggles `hidden` on cards and groups from `filterGroups(catalog.paletteGroups(E.widgets), q, names)`. `names` returns `widget.<t>.name`, `variant.<t>.<v>`, `widget.<t>.desc`, `category.<c>` (registered dynamic prefixes).
  - Devices panel `div.hd-pal-panel[hidden]`:
    - `p.hd-sr-only#hintE` (`palette.entity_hint`).
    - `div.hd-pal-filters` with three `E.form.select` controls: source (from `ent.sources`, `sourceName`), kind (from `kindsOf`, sorted by `Intl.Collator(lang)` over `homedash.kind.<k>`), area (from `ent.areas`). Each has an "All …" first option.
    - `p.hd-pal-status[role=status][data-state]` (text: `loading` / `failed` + a quiet retry button / `empty` / `count_of` / `count`).
    - `ul.hd-ent-list[aria-label]` of `li.hd-ent-item` > `button.hd-ent-row[tabindex][data-entity][aria-describedby=hintE]` > `span.hd-ent-icon` (`E.widgets.iconFor(null, view)`) + `span.hd-ent-text` (`.hd-ent-name` textContent, `.hd-ent-meta` = area · source) + `span.hd-ent-state`.
    - The last row is `li.hd-ent-moreitem` > quiet `palette.load_more` button. It is also auto-loaded when it scrolls into view.
  - Events (one `E.core.bag()`):
    - Widgets: `click` on `.hd-pal-hit` → `addEntry(E.commands.entry(catalog, type, variant), 'palette')`. `pointerdown` on `.hd-pal-card` (main button) → `D.gestures.dragFrom(ev, {kind:'widget', label: typeName, entries: () => [entry], avatar: () => avatar(TYPE_ICONS[type] || 'dashboard', typeName, variantName)})`. There is no `onTap`: the click adds, and `track` swallows the click that follows a drag.
    - `keydown` on the widgets panel → `E.form.gridMove(key, i, sizesOfVisibleGroups, 2)`, roving `tabindex`, `scrollIntoView({block:'nearest'})`.
    - Rows: `click` → `addEntry(entryFor(e), 'entity')`. `entryFor(e)` = `catalog.pickWidgetFor(e, E.widgets)` → `E.commands.entry(catalog, pick.type, pick.variant, e.id)`. `pointerdown` → `dragFrom(ev, {kind:'entity', entity: e, label: e.name, entries: () => [entryFor(e)], avatar})`. `keydown` Up/Down/Home/End moves the roving tabindex.
    - `addEntry(entry, source)`: `if (D.state().drawers) D.closePanels(); D.addWidgets([entry], {source});`. The drawer is closed first so the designer's focus on the new frame wins.
    - Root `keydown`: ArrowDown in the search box focuses the first visible card or row. Escape in a non-empty search clears it. Any other Escape → `preventDefault()` + `drawers ? D.closePanels() : D.focusFrame(focusId || primary || items[0].id)`, so the designer's Escape ladder does not clear the selection.
    - The search input debounces the devices query by 250 ms → `ent = entReduce(ent, {type:'filter', q})`, then `load()`. `load()` captures `seq`, calls `api.entities(entRequest(ent))` and dispatches `loaded`/`failed`. Opening the Devices tab the first time calls `filter` with `force:true`.
    - Live state: an `IntersectionObserver` (root = devices panel, `rootMargin:'64px'`; skipped when absent, as in Node, and then the first 20 rows count as visible) keeps a `visible` Set. An `E.core.frame` sets `watch.set(watchIds(rowOrder, visible))`, or `[]` while the Widgets tab shows. `store.on('change', ids => …)` rewrites `.hd-ent-state` from `store.view(id)` for rows in `ids` (`null` = all). `watch = store.watch(E.core.uid('hd-pal'), {ids: [], active: true})`.
  - `avatar(icon, title, sub)` = `div.hd-pal-avatar` > icon + `span.hd-pal-avatar-text` > `strong` + `small`.
  - Return `Object.freeze({el, focusSearch, setTab, destroy})`. `destroy` clears the timer, cancels the frame, disconnects the observer, disposes the bag, calls `watch.release()`, destroys the thumbs and removes the element.
  - Export `E.palette = Object.freeze({create, model: Object.freeze({fold, filterGroups, entInit, entReduce, entRequest, watchIds, kindsOf, sourceName, stateText, listView, TYPE_ICONS, ENT_LIMIT, WATCH_MAX})})`.
- [ ] `designer-kit.mjs` (shared by the three DOM test files; boots the engine up to a file with the fake DOM, a fake store/api and a designer double that applies the commands) and `palette.test.mjs` (11 tests: form, samples, search folding, palette = catalog ∩ registry, the reducer, the DOM smoke, drawer mode, the retry state):
<!-- embed: dryrun-m4/scripts/homedash-tests/designer-kit.mjs -->
```js
// Shared test kit for the M4 part-2 DOM modules (palette, inspector, look, dialogs): boots the engine up to a file with the fake DOM, the golden catalog,
// a fake store/api and a fake designer D that applies the executed commands to its document. Not a test file (no `.test.mjs` suffix).
import { loadEngine, APP_ORDER, read, plain } from './harness.mjs';
import { FakeElement } from './dom.mjs';
export const CATALOG = JSON.parse(read('internal/homedash/testdata/widget_catalog.json'));
export const FILES = upto => APP_ORDER.slice(0, APP_ORDER.indexOf(upto) + 1);
export const boot = upto => { const { E, window, document } = loadEngine(FILES(upto), { dom: true }); return { E, window, document, cat: E.catalog.create(CATALOG) }; };
export { FakeElement };
// Dispatches on `node` itself with `target` set: the fake DOM has no bubbling, so delegated listeners are fed directly.
export const fire = (node, type, extra, target) => { const ev = Object.assign({ type, target: target || node, preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; } }, extra); node.dispatchEvent(ev); return ev; };
// fire() returns the event, so a test reads ev.defaultPrevented and ev.stopped.
export const flush = (ms = 30) => new Promise(r => setTimeout(r, ms));
export function fakeStore(views) {
    const set = new Set(), calls = [], v = views || {}, released = [];
    return { views: v, calls, released, listeners: set, view: id => v[id] || null, on(type, fn) { set.add(fn); return () => set.delete(fn); }, emit(ids) { for (const fn of [...set]) fn(ids); },
        watch() { return { set(ids) { calls.push(ids.slice()); }, setActive() {}, release() { released.push(true); } }; }, docs: { get: () => null } };
}
export function fakeApi(entities) {
    const log = [], list = entities || [];
    return { log, entities: async p => { log.push(p); const rows = list.filter(e => (!p.kind || e.kind === p.kind) && (!p.q || (e.name + e.id).toLowerCase().includes(String(p.q).toLowerCase()))); return { entities: rows.slice(p.offset || 0, (p.offset || 0) + p.limit), total: rows.length, sources: [{ source: 'ha' }, { source: 'system' }] }; },
        entity: async id => { log.push({ entity: id }); const e = list.find(x => x.id === id); if (!e) { const err = new Error('nf'); err.status = 404; err.code = 'entity_not_found'; throw err; } return { entity: e }; } };
}
// A designer double: state() is rebuilt from the document after every exec; on('state') fires; commands are applied with E.doc.apply and recorded.
export function fakeD(E, cat, doc0, o) {
    const em = E.core.emitter(), x = o || {}, execs = [], calls = [], t = x.t || (k => k); let doc = doc0, sel = x.selection || [], pageId = x.pageId || doc0.pages[0].id;
    const state = () => { const page = doc.pages.find(p => p.id === pageId) || doc.pages[0]; pageId = page.id; return { doc, pageId, page, bp: 'lg', selection: sel, primary: sel[sel.length - 1] || null, focusId: x.focusId || null, drawers: !!x.drawers, items: page.layouts.lg }; };
    const D = { t, lang: 'en', catalog: cat, execs, calls, state, doc: () => doc, on: (type, fn) => em.on(type, fn), constraints: id => cat.constraints(doc.pages.find(p => p.id === pageId).widgets.find(w => w.id === id)),
        exec(cmd) { execs.push(cmd); if (!cmd || cmd.refused || cmd.limit) return false; doc = E.doc.apply(doc, cmd.ops); em.emit('state', state()); return true; },
        select(ids) { sel = ids; em.emit('state', state()); }, setDoc(d) { doc = d; em.emit('state', state()); },
        seal() { calls.push('seal'); }, announce(text) { calls.push('announce:' + text); }, closePanels() { calls.push('closePanels'); }, focusFrame(id) { calls.push('focusFrame:' + id); }, addWidgets(entries, opts) { calls.push({ addWidgets: plain(entries), opts }); return []; },
        gestures: { dragFrom(ev, payload) { calls.push({ dragFrom: payload.kind, label: payload.label }); D.lastDrag = payload; } }, sheet: () => x.sheet || null };
    return D;
}
export const docOf = (E, cat, widgets, layout, extra) => E.doc.normalise(Object.assign({ name: 'Test', pages: [{ id: 'main', name: 'Main', widgets, layouts: { lg: layout } }, { id: 'p2', name: 'Second', widgets: [], layouts: { lg: [] } }] }, extra), { sizeOf: cat.defaultSize, constraints: cat.constraints });
```
<!-- embed: dryrun-m4/scripts/homedash-tests/palette.test.mjs -->
```js
import assert from 'node:assert/strict'; import { eq, fc, noStyleAttr } from './harness.mjs'; import { boot, fire, flush, fakeD, fakeStore, fakeApi, docOf } from './designer-kit.mjs';
const ENTS = [{ id: 'ha:light.kitchen', source: 'ha', kind: 'light', name: 'Kitchen', area: 'Kitchen', capabilities: [{ name: 'brightness' }], state: { value: 'on' } },
  { id: 'system:cpu', source: 'system', kind: 'sensor', name: 'CPU', unit: '%', capabilities: [{ name: 'numeric' }], state: { value: 12 } }];
function mount(o) { const k = boot('hd-palette.js'), doc = docOf(k.E, k.cat, [], []), D = fakeD(k.E, k.cat, doc, o), store = fakeStore(), api = fakeApi(ENTS), host = k.document.createElement('div'); k.document.body.appendChild(host);
  const pal = k.E.palette.create({ host, designer: D, t: x => x, lang: 'en', catalog: k.cat, store, api }); return Object.assign(k, { D, store, api, host, pal, panels: host.querySelectorAll('.hd-pal-panel') }); }
export const tests = [
  ['form: grouped grid moves keep the column and cross groups', () => { const { E } = boot('hd-form.js'); const g = (k, i) => E.form.gridMove(k, i, [3, 2], 2);
    assert.equal(g('ArrowDown', 1), 2); assert.equal(g('ArrowDown', 2), 3); assert.equal(g('ArrowDown', 3), 3); assert.equal(g('ArrowUp', 4), 2); assert.equal(g('ArrowUp', 0), 0);
    assert.equal(g('ArrowRight', 2), 3); assert.equal(g('End', 0), 4); assert.equal(g('Tab', 0), null); assert.equal(E.form.roving('ArrowLeft', 0, 3), 2); }],
  ['samples: thumb size caps at 2x2, keeps the minimum; a thumb is inert and never commands', () => { const { E, cat, document } = boot('hd-samples.js');
    eq(E.samples.thumbSize(cat, 'light', 'slider'), { w: 2, h: 1 }); eq(E.samples.thumbSize(cat, 'light', 'fader'), { w: 1, h: 2 }); eq(E.samples.thumbSize(cat, 'climate', 'dial'), { w: 2, h: 2 });
    const th = E.samples.thumb({ widget: { type: 'switch', variant: 'pill', title: '', bindings: {}, options: {}, actions: {}, confirm: false }, catalog: cat, t: k => k, lang: 'en', box: { width: 116, height: 76 }, parent: document.body });
    const w = th.el.querySelector('.hd-widget'); assert.equal(w.getAttribute('aria-hidden') || th.el.getAttribute('aria-hidden'), 'true'); assert.equal(th.el.style.getPropertyValue('--hd-e-ts'), '0.967'); noStyleAttr(th.el); th.destroy(); }],
  ['form: segmented roves with the arrows, field errors set aria-invalid, text coalesces composition', () => { const { E, document } = boot('hd-form.js'), F = E.form, seen = [];
    const seg = F.segmented({ label: 'x', options: [{ id: 'a', label: 'A' }, { id: 'b', label: 'B' }, { id: 'c', label: 'C' }], value: 'a', onChange: v => seen.push(v) }), opts = seg.el.querySelectorAll('.hd-f-seg-opt');
    assert.equal(opts[0].getAttribute('aria-checked'), 'true'); fire(seg.el, 'keydown', { key: 'ArrowRight' }, opts[0]); eq(seen, ['b']); assert.equal(opts[1].getAttribute('aria-checked'), 'true'); assert.equal(opts[1].getAttribute('tabindex'), '0');
    fire(seg.el, 'keydown', { key: 'End' }, opts[1]); eq(seen, ['b', 'c']); seg.set('a'); assert.equal(opts[0].getAttribute('aria-checked'), 'true'); eq(seen, ['b', 'c']);
    const typed = [], tx = F.text({ key: 'k', max: 5, onInput: v => typed.push(v) }); tx.el.value = 'abcdefgh'; fire(tx.el, 'input', {}); eq(typed, ['abcde']); assert.equal(tx.el.value, 'abcde');
    fire(tx.el, 'compositionstart', {}); tx.el.value = 'xy'; fire(tx.el, 'input', {}); eq(typed, ['abcde']); fire(tx.el, 'compositionend', {}); eq(typed, ['abcde', 'xy']);
    const f = F.field({ label: 'L', help: 'H', required: '*', control: tx }); document.body.appendChild(f.el); assert.equal(f.el.querySelector('label').getAttribute('for'), tx.el.id); assert.equal(tx.el.getAttribute('aria-invalid'), null);
    f.error('bad'); assert.equal(tx.el.getAttribute('aria-invalid'), 'true'); assert.ok(tx.el.getAttribute('aria-describedby').includes('-e')); f.error(''); assert.equal(tx.el.getAttribute('aria-invalid'), null); noStyleAttr(f.el); }],
  ['form: the icon grid labels and searches by the translated name and keeps the id', () => { const { E, document } = boot('hd-form.js'), picked = [], de = { 'homedash.icon.bulb': 'Gl' + fc(0xfc) + 'hbirne', 'homedash.icon.thermometer': 'Thermometer' };
    const grid = E.form.iconGrid({ t: k => de[k] || k, label: 'Icons', value: 'power', allowAuto: true, onPick: n => picked.push(n) }); document.body.appendChild(grid.el);
    const bulb = grid.el.querySelector('[data-icon="bulb"]'); assert.equal(bulb.getAttribute('aria-label'), 'Gl' + fc(0xfc) + 'hbirne'); assert.equal(bulb.getAttribute('title'), 'Gl' + fc(0xfc) + 'hbirne'); assert.equal(grid.el.querySelector('[data-icon=""]').getAttribute('aria-label'), 'homedash.inspector.icon_auto');
    const visible = () => grid.el.querySelectorAll('.hd-icongrid-opt').filter(b => !b.hasAttribute('hidden')).map(b => b.getAttribute('data-icon'));
    grid.search.el.value = 'gluhb'; fire(grid.search.el, 'input', {}); eq(visible(), ['bulb']); grid.search.el.value = 'THERMO'; fire(grid.search.el, 'input', {}); eq(visible(), ['thermometer']);
    grid.search.el.value = 'bulb'; fire(grid.search.el, 'input', {}); eq(visible(), ['bulb']); grid.search.el.value = 'zzzz'; fire(grid.search.el, 'input', {}); eq(visible(), []); assert.equal(grid.el.querySelector('p.hd-f-help').hasAttribute('hidden'), false);
    grid.search.el.value = ''; fire(grid.search.el, 'input', {}); const list = grid.el.querySelector('.hd-icongrid-list'); fire(list, 'click', {}, bulb); eq(picked, ['bulb']); assert.equal(bulb.getAttribute('aria-pressed'), 'true');
    fire(list, 'keydown', { key: 'ArrowRight' }, bulb); assert.equal(grid.el.querySelectorAll('.hd-icongrid-opt').filter(b => b.getAttribute('tabindex') === '0').length, 1); }],
  ['form: a form sheet stays open while onAction refuses and resolves with the action id', async () => { const { E, document } = boot('hd-form.js'), box = document.createElement('div'); document.body.appendChild(box);
    const sheet = E.sheet.create(box); let ok = false; const done = E.form.formSheet(sheet, { title: 'T', content: document.createElement('div'), actions: [{ id: 'save', label: 'S', tone: 'primary' }], onAction: () => ok });
    const foot = sheet.el.querySelector('.hd-sheet-foot'), btn = foot.querySelector('.hd-dlg-btn'); let result = 'pending'; done.then(v => { result = v; });
    fire(foot, 'click', {}, btn); await flush(5); assert.equal(result, 'pending'); assert.ok(sheet.isOpen()); ok = true; fire(foot, 'click', {}, btn); await flush(5); assert.equal(result, 'save'); assert.ok(!sheet.isOpen()); }],
  ['palette: search folds accents and tokens; groups keep catalog order; empty query returns everything', () => { const { E, cat } = boot('hd-palette.js'), M = E.palette.model, groups = cat.paletteGroups(E.widgets);
    const names = (type, v, c) => ({ type: type === 'climate' ? 'L' + fc(0xfc) + 'ftung' : type, variant: v === 'dial' ? 'Drehregler' : v, desc: '', category: c });
    eq(M.filterGroups(groups, 'luftung', names).flatMap(g => g.items.map(i => i.type)), ['climate', 'climate', 'climate'].slice(0, cat.widget('climate').variants.length));
    eq(M.filterGroups(groups, 'LUFTUNG drehr', names).flatMap(g => g.items), [{ type: 'climate', variant: 'dial' }]); eq(M.filterGroups(groups, 'light dial', names).flatMap(g => g.items), [{ type: 'light', variant: 'dial' }]);
    const all = M.filterGroups(groups, '  ', names); eq(all.map(g => g.category), groups.map(g => g.category)); eq(all.flatMap(g => g.items.map(i => i.type + '/' + i.variant)), groups.flatMap(g => g.types.flatMap(x => x.variants.map(v => x.type + '/' + v))));
    eq(M.filterGroups(groups, 'nothing-matches', names), []); }],
  ['palette: palette = catalog and registry; an unregistered catalog type never appears', () => { const { E, cat } = boot('hd-palette.js'), types = cat.paletteGroups(E.widgets).flatMap(g => g.types.map(x => x.type)).sort();
    eq(types, E.widgets.types().filter(t2 => cat.has(t2)).sort()); assert.ok(!types.includes('weather') && !types.includes('camera')); const reg = { get: t2 => (t2 === 'switch' ? E.widgets.get('switch') : null) }; eq(cat.paletteGroups(reg).flatMap(g => g.types.map(x => x.type)), ['switch']); }],
  ['palette: entity list reducer pages, dedupes, ignores stale answers and keeps areas', () => { const { E } = boot('hd-palette.js'), M = E.palette.model, rows = (a, n) => Array.from({ length: n }, (x, i) => ({ id: 'ha:e' + (a + i), name: 'E' + (a + i), area: i % 2 ? 'Hall' : 'Attic' }));
    let s = M.entReduce(M.entInit(), { type: 'filter' }); assert.equal(s.seq, 1); assert.equal(s.loading, true); assert.equal(M.entReduce(s, { type: 'filter' }), M.entReduce(s, { type: 'filter' }), 'same filter, same state'); assert.equal(M.entReduce(s, { type: 'loaded', seq: 0, answer: { entities: rows(0, 3), total: 3 } }), s);
    s = M.entReduce(s, { type: 'loaded', seq: 1, answer: { entities: rows(0, 200), total: 450 } }); assert.equal(s.items.length, 200); assert.equal(s.total, 450); eq(s.areas, ['Attic', 'Hall']);
    s = M.entReduce(s, { type: 'more' }); assert.equal(s.offset, 200); assert.equal(s.seq, 2); assert.equal(M.entReduce(s, { type: 'more' }), s, 'busy: no second page');
    const old = s; s = M.entReduce(s, { type: 'loaded', seq: 2, answer: { entities: rows(198, 5), total: 450 } }); assert.equal(s.items.length, 203, 'three repeated ids are dropped'); assert.equal(M.entReduce(old, { type: 'loaded', seq: 1, answer: { entities: rows(900, 2), total: 9 } }), old);
    s = M.entReduce(s, { type: 'more' }); s = M.entReduce(s, { type: 'loaded', seq: s.seq, answer: { entities: rows(0, 3), total: 450 } }); assert.equal(s.total, s.items.length, 'no new rows ends paging'); assert.equal(M.entReduce(s, { type: 'more' }), s);
    s = M.entReduce(s, { type: 'filter', q: ' kitchen ', force: true }); assert.equal(s.q, 'kitchen'); assert.deepEqual(Object.keys(M.entRequest(s)).filter(k => M.entRequest(s)[k] !== undefined).sort(), ['limit', 'offset', 'q']); assert.equal(M.entRequest(s).limit, 200);
    const f = M.entReduce(s, { type: 'failed', seq: s.seq, code: 'network' }); assert.equal(f.error, 'network'); assert.equal(M.entReduce(f, { type: 'more' }), f); eq(M.entReduce(f, { type: 'filter', area: 'Hall' }).areas, ['Attic', 'Hall']);
    assert.equal(M.watchIds(Array.from({ length: 50 }, (x, i) => 'a' + i), new Set(Array.from({ length: 50 }, (x, i) => 'a' + i))).length, 20); eq(M.watchIds(['a', 'b', 'c'], new Set(['c', 'a'])), ['a', 'c']); }],
  ['palette: DOM smoke - cards, tap adds once, drag payload, search, keys, devices tab with live watch, destroy releases', async () => { const m = mount(), { E, cat, D, store, api, host, pal } = m, panelW = m.panels[0], panelE = m.panels[1];
    const total = cat.paletteGroups(E.widgets).reduce((n, g) => n + g.types.reduce((k, x) => k + x.variants.length, 0), 0); assert.equal(host.querySelectorAll('.hd-pal-card').length, total); assert.ok(total > 10);
    const pill = host.querySelector('.hd-pal-card[data-type="switch"][data-variant="pill"]'), hit = pill.querySelector('.hd-pal-hit'); assert.equal(pill.querySelector('.hd-pal-thumb').querySelector('.hd-thumb') !== null, true, 'a thumb sits outside the button'); assert.equal(hit.querySelector('.hd-thumb'), null);
    fire(panelW, 'click', {}, hit); assert.equal(D.calls.length, 1); eq(D.calls[0].addWidgets.map(e => [e.widget.type, e.widget.variant]), [['switch', 'pill']]); eq(D.calls[0].opts, { source: 'palette' });
    fire(panelW, 'pointerdown', { button: 0 }, pill); eq(D.calls[1], { dragFrom: 'widget', label: 'homedash.widget.switch.name' }); assert.equal(D.lastDrag.entries()[0].widget.variant, 'pill'); assert.ok(D.lastDrag.avatar().querySelector('strong'));
    const search = host.querySelector('.hd-pal-search input'); search.value = 'pill'; fire(search, 'input', {}); const shown = host.querySelectorAll('.hd-pal-card').filter(c => !c.hasAttribute('hidden')); eq(shown.map(c => c.getAttribute('data-type')), ['switch']);
    search.value = 'zzz'; fire(search, 'input', {}); assert.equal(panelW.querySelector('.hd-pal-status').textContent, 'homedash.palette.no_widgets'); search.value = ''; fire(search, 'input', {}); assert.equal(host.querySelectorAll('.hd-pal-card').filter(c => c.hasAttribute('hidden')).length, 0);
    const first = host.querySelectorAll('.hd-pal-hit')[0]; fire(panelW, 'keydown', { key: 'ArrowRight' }, first); assert.equal(host.querySelectorAll('.hd-pal-hit')[1].getAttribute('tabindex'), '0'); assert.equal(first.getAttribute('tabindex'), '-1');
    pal.setTab('entities'); assert.equal(panelE.hasAttribute('hidden'), false); assert.equal(panelW.hasAttribute('hidden'), true); await flush(40); assert.equal(api.log.length, 1); assert.equal(api.log[0].limit, 200); assert.equal(api.log[0].offset, 0);
    const rows = host.querySelectorAll('.hd-ent-row'); eq(rows.map(r => r.getAttribute('data-entity')), ['ha:light.kitchen', 'system:cpu']); assert.equal(rows[0].querySelector('.hd-ent-name').textContent, 'Kitchen'); assert.ok(store.calls.some(c => c.includes('ha:light.kitchen')), 'visible rows are watched');
    const list = host.querySelector('.hd-ent-list'); fire(list, 'click', {}, rows[0]); const e = D.calls[D.calls.length - 1]; eq([e.addWidgets[0].widget.type, e.addWidgets[0].widget.variant, e.addWidgets[0].widget.bindings], ['light', 'slider', { primary: 'ha:light.kitchen' }]); eq(e.opts, { source: 'entity' });
    fire(list, 'pointerdown', { button: 0 }, rows[1]); assert.equal(D.lastDrag.kind, 'entity'); assert.equal(D.lastDrag.entity.id, 'system:cpu'); store.views['system:cpu'] = { id: 'system:cpu', kind: 'sensor', value: 55, unit: '%', available: true, status: 'ok', pending: false }; store.emit(new Set(['system:cpu']));
    assert.ok(rows[1].querySelector('.hd-ent-state').textContent.includes('55'));
    pal.setTab('widgets'); await flush(30); eq(store.calls[store.calls.length - 1], [], 'the Widgets tab watches nothing');
    fire(host.querySelector('.hd-pal'), 'keydown', { key: 'Escape' }, host.querySelectorAll('.hd-pal-hit')[2]); assert.equal(D.calls.filter(c => typeof c === 'string' && c.startsWith('focusFrame')).length, 0, 'no frame to focus on an empty page');
    noStyleAttr(host); pal.destroy(); assert.equal(store.released.length, 1); assert.equal(host.querySelector('.hd-pal'), null); }],
  ['palette: in drawer mode the drawer closes before the add; Escape in the search clears it first', () => { const m = mount({ drawers: true, focusId: 'w1' }), { D, host } = m, hit = host.querySelector('.hd-pal-hit');
    fire(m.panels[0], 'click', {}, hit); assert.equal(D.calls[0], 'closePanels'); assert.equal(typeof D.calls[1], 'object'); const search = host.querySelector('.hd-pal-search input'); search.value = 'x'; fire(search, 'input', {});
    fire(host.querySelector('.hd-pal'), 'keydown', { key: 'Escape' }, search); assert.equal(search.value, ''); assert.equal(D.calls.filter(c => c === 'closePanels').length, 1);
    fire(host.querySelector('.hd-pal'), 'keydown', { key: 'Escape' }, hit); assert.equal(D.calls.filter(c => c === 'closePanels').length, 2); }],
  ['palette: a failed device query shows the retry state; the retry loads', async () => { const m = mount(); let fail = true; const real = m.api.entities; m.api.entities = async p => { if (fail) { const e = new Error('x'); e.code = 'network'; throw e; } return real(p); };
    m.pal.setTab('entities'); await flush(30); const status = m.host.querySelector('.hd-pal-filters').parentElement.querySelector('p.hd-pal-status'); assert.equal(status.getAttribute('data-state'), 'failed'); fail = false;
    fire(status.querySelector('button'), 'click', {}); await flush(30); assert.equal(m.host.querySelectorAll('.hd-ent-row').length, 2); assert.equal(status.getAttribute('data-state'), 'ready'); }]
];
```
- [ ] Run wiring step W for C1 and C2. PASS (`npm run test:homedash`, `go test ./ui -run '^TestHomeDashWiring$' -count=1` in the worktree). Commit — `feat(homedash): editor form controls, sample thumbs and the palette with the device browser`.

### Task C3: hd-entity-picker.js and hd-inspector.js
**Files:** create `hd-entity-picker.js`, `hd-inspector.js`, `scripts/homedash-tests/inspector.test.mjs`. Wiring step W for both. **Existing symbols changed:** none.
- [ ] `hd-entity-picker.js`, in full (model `E.entityPicker.model` = `requests, merge, sourcesOf, LIMIT`, and the combobox):
<!-- embed: dryrun-m4/ui/js/homedash/hd-entity-picker.js -->
```js
// HomeDash entity picker (M4 part 2, D6): the Data tab's combobox (WAI-ARIA APG, popup inside the box) that binds a widget slot to a device. One
// GET entities per allowed kind (C9), merged, filtered with catalog.accepts and sorted by name; live state of the first 20 options while open; the
// bound device shows name, area and source, or a warning when the store says `unknown` or GET entity answers 404. Names only via textContent.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.form || !E.palette || !E.catalog) throw new Error('hd-entity-picker.js: load hd-form.js and hd-palette.js first');
    const { el, setAttr, uid } = E.core, DOT = ' ' + String.fromCodePoint(0xb7) + ' ', LIMIT = 50, SEARCH_MS = 200, WATCH_MAX = 20, CACHE_MS = 60000, cache = new Map();
    // One GET entities per allowed kind (C9: one kind per request); a chosen kind narrows to it; no kinds = any kind.
    function requests(f, kinds) { const base = { q: f.q || undefined, source: f.source || undefined, area: f.area || undefined, limit: LIMIT, offset: 0 };
        return (f.kind ? [f.kind] : kinds && kinds.length ? kinds : [null]).map(k => Object.assign({}, base, k ? { kind: k } : {})); }
    function merge(answers, accept, compare) { const seen = new Set(), out = [];
        for (const a of answers) for (const e of a && Array.isArray(a.entities) ? a.entities : []) { if (!e || typeof e.id !== 'string' || seen.has(e.id) || !accept(e)) continue; seen.add(e.id); out.push(e); }
        return out.sort((x, y) => compare(String(x.name || x.id), String(y.name || y.id))).slice(0, LIMIT); }
    const sourcesOf = answers => [...new Set(answers.flatMap(a => (a && Array.isArray(a.sources) ? a.sources : []).map(s => s && s.source).filter(s => typeof s === 'string')))];

    function create(o) {
        const D = o.designer, t = o.t, lang = o.lang || 'en', catalog = o.catalog, store = o.store, api = o.api, name = typeof o.binding === 'string' ? o.binding : o.binding.name, spec = catalog.binding(o.type, name) || {};
        const kinds = Array.isArray(spec.kinds) ? spec.kinds : [], timers = o.timers || { setTimeout: (f, ms) => setTimeout(f, ms), clearTimeout: x => clearTimeout(x) }, collator = new Intl.Collator(lang), life = E.core.bag();
        const id = uid('hd-ep'), listId = id + '-list', stateOf = e => E.palette.model.stateText(t, lang, store.view(e.id) || E.palette.model.listView(e));
        let value = o.value || '', open = false, options = [], active = -1, seq = 0, timer = null, destroyed = false, bound = null, boundSeq = 0, areas = [], sources = [], shownSources = '', shownAreas = '';
        const filters = { q: '', source: '', kind: '', area: '' }, watch = store.watch(uid('hd-ep'), { ids: [], active: true });
        const input = el('input', { class: 'hd-f-input hd-ep-input', type: 'text', id, role: 'combobox', 'aria-autocomplete': 'list', 'aria-expanded': 'false', 'aria-controls': listId, 'aria-haspopup': 'listbox', 'aria-activedescendant': null,
            placeholder: t('homedash.inspector.choose_device'), spellcheck: 'false', autocomplete: 'off', 'data-hd-field': 'binding:' + name });
        const clear = E.form.button({ label: t('homedash.inspector.clear_binding'), icon: 'close', iconOnly: true, tone: 'quiet', onClick: () => { pick(undefined); input.focus(); } });
        const toggle = el('button', { type: 'button', class: 'hd-ep-toggle', tabindex: '-1', 'aria-label': o.label || name }, E.icons.icon('chevron-down'));
        const selKind = E.form.select({ key: 'ep-kind', label: t('homedash.palette.filter_kind'), onChange: v => refilter({ kind: v }) }), selSource = E.form.select({ key: 'ep-source', label: t('homedash.palette.filter_source'), onChange: v => refilter({ source: v }) });
        const selArea = E.form.select({ key: 'ep-area', label: t('homedash.palette.filter_area'), onChange: v => refilter({ area: v }) });
        const status = el('p', { class: 'hd-ep-status', role: 'status' }), list = el('ul', { class: 'hd-ep-list', role: 'listbox', id: listId, 'aria-label': o.label || name });
        const filterBar = el('div', { class: 'hd-ep-filters' }, [kinds.length > 1 ? selKind.wrap : null, selSource.wrap, selArea.wrap]), pop = el('div', { class: 'hd-ep-pop', hidden: true }, [filterBar, list, status]);
        const box = el('div', { class: 'hd-ep-box' }, [el('span', { class: 'hd-ep-icon' }, E.icons.icon('search')), input, clear.el, toggle, pop]);
        const label = el('label', { class: 'hd-f-label', for: id }, [el('span', { text: o.label || name }), o.required ? el('span', { class: 'hd-f-req', text: t('homedash.inspector.required') }) : null]);
        const meta = el('p', { class: 'hd-ep-meta' }), warn = el('p', { class: 'hd-ep-warn', hidden: true, role: 'status' });
        const root = el('div', { class: 'hd-ep', 'data-hd-state': 'empty' }, [label, box, meta, warn]);
        selKind.options([{ id: '', label: t('homedash.palette.all_kinds') }].concat(kinds.map(k => ({ id: k, label: t('homedash.kind.' + k) })).sort((a, b) => collator.compare(a.label, b.label))), '');
        selSource.options([{ id: '', label: t('homedash.palette.all_sources') }], ''); selArea.options([{ id: '', label: t('homedash.palette.all_areas') }], '');

        function boundText() { return bound ? String(bound.name || bound.id) : value; }
        function paintBound() {
            const v = value ? store.view(value) : null, st = !value ? 'empty' : v && v.status === 'unknown' ? 'unknown' : bound === false ? 'unknown' : 'bound';
            const b = v && v.status !== 'unknown' ? v : bound && bound !== false ? bound : null;
            setAttr(root, 'data-hd-state', st); setAttr(clear.el, 'hidden', value ? null : true);
            if (!open) input.value = b ? String(b.name || value) : value;
            meta.textContent = b ? [value, b.area, b.source].filter(Boolean).join(DOT) : ''; setAttr(meta, 'hidden', b ? null : true);
            warn.replaceChildren(); setAttr(warn, 'hidden', st === 'unknown' ? null : true);
            if (st === 'unknown') { warn.appendChild(E.icons.icon('alert')); warn.appendChild(el('span', { text: t('homedash.inspector.unknown_entity', { id: value }) })); }
        }
        function resolve() {
            const my = ++boundSeq; bound = null; if (!value) { paintBound(); return; }
            const v = store.view(value); if (v) { paintBound(); return; }
            const hitc = cache.get(value); if (hitc && Date.now() - hitc.at < CACHE_MS) { bound = hitc.entity; paintBound(); return; }
            paintBound();
            Promise.resolve().then(() => api.entity(value)).then(ans => { const e = ans && ans.entity && typeof ans.entity === 'object' ? ans.entity : null; cache.set(value, { at: Date.now(), entity: e || false }); if (my === boundSeq && !destroyed) { bound = e || false; paintBound(); } },
                err => { const gone = !!err && err.code === 'entity_not_found'; if (gone) cache.set(value, { at: Date.now(), entity: false }); if (my === boundSeq && !destroyed) { bound = gone ? false : null; paintBound(); } });
        }
        function setOpen(on) {
            if (open === on) return; open = on; setAttr(pop, 'hidden', on ? null : true); setAttr(input, 'aria-expanded', on ? 'true' : 'false'); if (!on) { setAttr(input, 'aria-activedescendant', null); active = -1; watch.set([]); }
            if (on) search(); else paintBound();
        }
        function search() {
            const my = ++seq; status.textContent = t('homedash.inspector.searching');
            Promise.all(requests(filters, kinds).map(r => Promise.resolve().then(() => api.entities(r)))).then(answers => {
                if (my !== seq || destroyed) return; options = merge(answers, e => catalog.accepts(o.type, name, e), collator.compare); sources = sourcesOf(answers); areas = [...new Set(areas.concat(options.map(e => e.area).filter(Boolean)))].sort(); renderOptions();
            }, () => { if (my !== seq || destroyed) return; options = []; renderOptions(); status.textContent = t('homedash.inspector.search_failed'); });
        }
        function renderOptions() {
            list.replaceChildren(); active = options.length ? Math.max(0, options.findIndex(e => e.id === value)) : -1;
            options.forEach((e, i) => list.appendChild(el('li', { class: 'hd-ep-opt', role: 'option', id: id + '-o' + i, 'aria-selected': e.id === value ? 'true' : 'false', 'data-entity': e.id, 'data-hd-active': i === active ? 'true' : null },
                [el('span', { class: 'hd-ep-opt-name', text: String(e.name || e.id) }), el('span', { class: 'hd-ep-opt-meta', text: [e.area, e.source].filter(Boolean).join(DOT) }), el('span', { class: 'hd-ep-opt-state', text: stateOf(e) })])));
            status.textContent = options.length ? '' : t('homedash.inspector.no_match'); setAttr(input, 'aria-activedescendant', active >= 0 ? id + '-o' + active : null);
            if (shownSources !== sources.join(',')) { shownSources = sources.join(','); selSource.options([{ id: '', label: t('homedash.palette.all_sources') }].concat(sources.map(s => ({ id: s, label: E.palette.model.sourceName(t, s) }))), filters.source); }
            if (shownAreas !== areas.join(',')) { shownAreas = areas.join(','); selArea.options([{ id: '', label: t('homedash.palette.all_areas') }].concat(areas.map(a => ({ id: a, label: a }))), filters.area); }
            watch.set(options.slice(0, WATCH_MAX).map(e => e.id));
        }
        function refilter(patch) { Object.assign(filters, patch); if (open) search(); }
        function move(d) { if (!options.length) return; const prev = list.children[active]; if (prev) prev.removeAttribute('data-hd-active'); active = (active + d + options.length) % options.length;
            const cur = list.children[active]; if (cur) { cur.setAttribute('data-hd-active', 'true'); if (typeof cur.scrollIntoView === 'function') cur.scrollIntoView({ block: 'nearest' }); } setAttr(input, 'aria-activedescendant', id + '-o' + active); }
        function pick(entityId) { setOpen(false); value = entityId || ''; bound = entityId && options.find(e => e.id === entityId) || null; filters.q = ''; paintBound(); if (o.onPick) o.onPick(entityId || undefined); }
        life.on(input, 'input', () => { filters.q = input.value; if (!open) { setOpen(true); return; } timers.clearTimeout(timer); timer = timers.setTimeout(() => { timer = null; search(); }, SEARCH_MS); });
        life.on(input, 'focus', () => { input.select && input.select(); });
        life.on(input, 'keydown', ev => {
            if (ev.isComposing) return;
            if (ev.key === 'ArrowDown' || ev.key === 'ArrowUp') { ev.preventDefault(); if (!open) setOpen(true); else move(ev.key === 'ArrowDown' ? 1 : -1); return; }
            if (ev.key === 'Enter') { if (open && active >= 0 && options[active]) { ev.preventDefault(); pick(options[active].id); } return; }
            if (ev.key === 'Escape' && open) { ev.preventDefault(); ev.stopPropagation(); setOpen(false); filters.q = ''; paintBound(); return; }
            if (ev.key === 'Tab') setOpen(false);
        });
        life.on(toggle, 'click', () => { setOpen(!open); if (open) input.focus(); });
        life.on(list, 'pointerdown', ev => ev.preventDefault());
        life.on(list, 'click', ev => { const li = ev.target && ev.target.closest ? ev.target.closest('.hd-ep-opt') : null; if (li) pick(li.getAttribute('data-entity')); });
        life.on(root, 'focusout', ev => { if (!ev.relatedTarget || !root.contains(ev.relatedTarget)) setOpen(false); });
        life.add(store.on('change', changed => { if (open) options.forEach((e, i) => { if (!changed || changed.has(e.id)) { const li = list.children[i], s = li && li.querySelector('.hd-ep-opt-state'); if (s) s.textContent = stateOf(e); } }); if (!open && value && (!changed || changed.has(value))) paintBound(); }));
        resolve();
        return Object.freeze({ el: root, binding: name, set(v) { const next = v || ''; if (next === value) return; value = next; filters.q = ''; resolve(); }, focus() { input.focus(); }, input, destroy() { if (destroyed) return; destroyed = true; timers.clearTimeout(timer); life.dispose(); watch.release(); root.remove(); } });
    }
    E.entityPicker = Object.freeze({ create, model: Object.freeze({ requests, merge, sourcesOf, LIMIT }) });
})();
```
- [ ] Picker structure and rules (the file is authoritative). `create({designer, t, lang, catalog, store, api, type, binding (name), label, required, value, onPick})` → `{el, binding, set(value), focus(), input, destroy()}`. Structure (APG combobox, popup inside the box):
  - `div.hd-ep[data-hd-state=empty|bound|unknown]`
    - `label.hd-f-label[for]` (binding label; `span.hd-f-req` = `inspector.required` when required)
    - `div.hd-ep-box`
      - `span.hd-ep-icon`
      - `input.hd-f-input.hd-ep-input[role=combobox][aria-autocomplete=list][aria-expanded][aria-controls][aria-haspopup=listbox][aria-activedescendant][placeholder=inspector.choose_device][data-hd-field="binding:<name>"]`
      - quiet icon-only clear `.hd-btn` (`inspector.clear_binding`, hidden while empty)
      - `button.hd-ep-toggle[tabindex=-1]` (chevron)
      - `div.hd-ep-pop[hidden]`
        - `div.hd-ep-filters` (kind select only when the binding allows more than one kind; source; area)
        - `ul.hd-ep-list[role=listbox][id]` > `li.hd-ep-opt[role=option][id][aria-selected][data-entity][data-hd-active]` > `.hd-ep-opt-name`, `.hd-ep-opt-meta`, `.hd-ep-opt-state`
        - `p.hd-ep-status[role=status]` (`searching` / `no_match` / `search_failed`)
    - `p.hd-ep-meta` (id · area · source)
    - `p.hd-ep-warn[hidden]` (alert icon + `inspector.unknown_entity {id}`)
- [ ] Picker rules:
  - Typing opens the popup and re-searches after 200 ms. ArrowDown/ArrowUp open the popup or move `aria-activedescendant`. Enter picks. Escape with the popup open → `preventDefault` + `stopPropagation` + close and restore the text; with it closed, the event bubbles to the inspector. Tab closes the popup.
  - `pointerdown` on the list → `preventDefault` (focus stays in the input). A `focusout` that leaves the picker closes it.
  - Search: `Promise.all(requests(filter, binding.kinds).map(api.entities))` → `merge(…, e => catalog.accepts(type, binding, e), collator.compare)`. Stale answers (`seq`) are dropped.
  - Live state: `watch = store.watch(uid('hd-ep'), {ids:[], active:true})` gets the first 20 options while open and `[]` when closed. `store.on('change')` refreshes the option states.
  - Display: `store.view(value)` with status not `unknown` → name/area/source. Status `unknown` → warning. No view → `api.entity(value)` (answers `{entity}`; module cache, 60 s TTL); a failure with `code === 'entity_not_found'` (the server's 404) → warning.
  - A pick → `onPick(id)`; clear → `onPick(undefined)`.
- [ ] `hd-inspector.js`, in full (model `E.inspector.model` = `fieldFor, commitValue, actionChoices, actionValue, structureKey, familyCount, draftOf`, and the tabs):
<!-- embed: dryrun-m4/ui/js/homedash/hd-inspector.js -->
```js
// HomeDash inspector (M4 part 2, F4b; D6, design 5.11): page properties when nothing is selected, four tabs (Data, Look, Options, Actions) for one
// widget, and family/align/delete for several. Every field change is one D.exec(E.commands.*) (text fields coalesce with text:true and seal on blur;
// text that the server would refuse is flagged inline and never executed). The structure is rebuilt only when structureKey changes, otherwise the
// controls are patched in place (never the focused one). Pure models on E.inspector.model. Untrusted names only via textContent.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.form || !E.entityPicker || !E.samples || !E.commands) throw new Error('hd-inspector.js: load hd-form.js, hd-samples.js, hd-entity-picker.js and hd-commands.js first');
    const { el, setAttr, uid } = E.core, DOT = ' ' + String.fromCodePoint(0xb7) + ' ', C = E.commands, F = E.form, TABS = Object.freeze(['data', 'look', 'options', 'actions']);
    const UNITS = Object.freeze({ hours: 'h', refresh_seconds: 's' }), HELP = Object.freeze(['accent', 'hours', 'warn', 'alert']);
    // Option spec -> control (D6, catalog OptionSpec types bool|int|number|string|enum|color|page).
    function fieldFor(spec, value, ctx) {
        const s = spec || {}, v = value === undefined ? s.default : value, base = { key: s.key, help: HELP.includes(s.key), required: s.required === true };
        switch (s.type) {
            case 'bool': return Object.assign(base, { control: 'switch', value: v === true });
            case 'int': case 'number': return Object.assign(base, { control: 'number', value: typeof v === 'number' && Number.isFinite(v) ? v : null, min: Number.isFinite(s.min) ? s.min : null, max: Number.isFinite(s.max) ? s.max : null,
                step: s.type === 'int' ? 1 : Number.isFinite(s.step) && s.step > 0 ? s.step : 'any', unit: UNITS[s.key] || '', nullable: s.default === null });
            case 'string': return Object.assign(base, { control: 'text', value: typeof v === 'string' ? v : '', maxLen: Math.min(Number(s.max_len) || 200, 200) });
            case 'enum': return Object.assign(base, { control: 'select', value: typeof v === 'string' ? v : '', options: (Array.isArray(s.enum) ? s.enum : []).filter(x => typeof x === 'string') });
            case 'color': return Object.assign(base, { control: 'color', value: typeof v === 'string' && /^#[0-9a-f]{6}$/i.test(v) ? v.toLowerCase() : '', allowEmpty: !s.required });
            case 'page': return Object.assign(base, { control: 'page', value: typeof v === 'string' ? v : '', options: ((ctx && ctx.pages) || []).filter(p => p.id !== (ctx && ctx.currentPageId)).map(p => p.id) });
            default: return null;
        }
    }
    // Control value -> option value: {ok, value} (undefined = back to the catalog default) or {ok:false, error:'required'|'bad_number'|'range'|'bad_colour'|'control_chars'|'url_not_allowed', min, max}.
    function commitValue(spec, raw) {
        const s = spec || {}, done = v => ({ ok: true, value: E.doc.keyOf(v) === E.doc.keyOf(s.default) ? undefined : v });
        switch (s.type) {
            case 'bool': return done(raw === true);
            case 'int': case 'number': { const txt = typeof raw === 'string' ? raw.trim().replace(',', '.') : raw;
                if (txt === '' || txt === null || txt === undefined) return s.required && s.default !== null ? { ok: false, error: 'required' } : { ok: true, value: undefined };
                let n = typeof txt === 'number' ? txt : Number(txt); if (!Number.isFinite(n)) return { ok: false, error: 'bad_number' }; if (s.type === 'int') n = Math.round(n);
                if ((Number.isFinite(s.min) && n < s.min) || (Number.isFinite(s.max) && n > s.max)) return { ok: false, error: 'range', min: s.min, max: s.max }; return done(n); }
            case 'string': { const v = Array.from(String(raw == null ? '' : raw)).slice(0, Math.min(Number(s.max_len) || 200, 200)).join(''), bad = E.form.textIssue(v);
                if (bad) return { ok: false, error: bad }; return v === '' && s.required ? { ok: false, error: 'required' } : done(v); }
            case 'enum': return Array.isArray(s.enum) && s.enum.includes(raw) ? done(raw) : { ok: false, error: 'required' };
            case 'color': { const v = String(raw || '').trim().toLowerCase(); if (!v) return s.required ? { ok: false, error: 'required' } : done(''); const m = /^#?([0-9a-f]{6})$/.exec(v); return m ? done('#' + m[1]) : { ok: false, error: 'bad_colour' }; }
            case 'page': return raw ? done(String(raw)) : s.required ? { ok: false, error: 'required' } : done('');
            default: return { ok: false, error: 'required' };
        }
    }
    function actionChoices(o) { const out = ['default']; if (o.canToggle) out.push('toggle'); if (o.entityKind === 'light' || (o.current && o.current.do === 'more_info')) out.push('more_info');
        if ((o.pages || []).length || (o.current && o.current.do === 'navigate')) out.push('navigate'); out.push('none'); return out; }
    function actionValue(choice, page, pages) { if (choice === 'default') return undefined;
        if (choice === 'navigate') { const p = (pages || []).find(x => x.id === page) || (pages || [])[0]; return p ? { do: 'navigate', page: p.id } : undefined; } return { do: choice }; }
    // Rebuild key: mode, tab, page, selection (ids, types, variants) and what changes the structure of a tab; everything else is patched in place.
    function structureKey(s, tab) {
        const page = s.page || { widgets: [] }, ws = (s.selection || []).map(id => page.widgets.find(w => w.id === id)).filter(Boolean), mode = ws.length === 0 ? 'page' : ws.length === 1 ? 'single' : 'multi', w = ws[0];
        return [mode, mode === 'single' ? tab : '', s.pageId, ws.map(x => x.id + ':' + x.type + ':' + x.variant).join(','), mode === 'single' && (tab === 'options' || tab === 'actions') ? ((s.doc && s.doc.pages) || []).map(p => p.id + ':' + p.name).join(',') : '',
            mode === 'single' && tab === 'actions' ? JSON.stringify(w.actions || {}) + ':' + ((w.bindings || {}).primary || '') : '', mode === 'multi' ? s.primary : ''].join('|');
    }
    const familyCount = s => new Set(((s.page && s.page.widgets) || []).map(w => w.family || (s.doc && s.doc.look.family) || 'glass')).size;
    function draftOf(catalog, w) { const spec = catalog.widget(w.type); if (!spec) return null; const empty = v => v === undefined || v === null || v === '';
        const b = (spec.bindings || []).find(x => x && x.required && empty((w.bindings || {})[x.name])); if (b) return { binding: b.name };
        const o = (spec.options || []).find(x => x && x.required && empty((w.options || {})[x.key])); return o ? { option: o.key } : null; }

    function create(o) {
        const D = o.designer, t = o.t, lang = o.lang || 'en', catalog = o.catalog, store = o.store, api = o.api, life = E.core.bag(), id = uid('hd-insp'), doc = () => (o.host && o.host.ownerDocument) || document;
        const timers = o.timers, root = el('div', { class: 'hd-insp' }), titleEl = el('h2', { class: 'hd-insp-title', id: id + '-t' }), subEl = el('p', { class: 'hd-insp-sub' }), iconBox = el('span', { class: 'hd-insp-icon' });
        const body = el('div', { class: 'hd-insp-body', id: id + '-b' }), foot = el('div', { class: 'hd-insp-foot' });
        const tabsCtl = F.tabs({ tabs: [{ id: 'data', label: t('homedash.inspector.tab_data') }, { id: 'look', label: t('homedash.inspector.tab_look') }, { id: 'options', label: t('homedash.inspector.tab_options') }, { id: 'actions', label: t('homedash.inspector.tab_actions') }],
            value: 'data', label: t('homedash.designer.inspector'), controls: id + '-b', onChange: v => setTab(v) });
        root.append(el('div', { class: 'hd-insp-head' }, [iconBox, el('div', { class: 'hd-insp-heading' }, [titleEl, subEl])]), tabsCtl.el, body, foot);
        let tab = 'data', lastKey = null, fields = new Map(), pickers = [], thumbs = [], destroyed = false;
        const typeName = x => t('homedash.widget.' + x + '.name'), variantName = (x, v) => t('homedash.variant.' + x + '.' + v);
        const cur = () => { const s = D.state(), page = s.page || { widgets: [] }, ws = (s.selection || []).map(i => page.widgets.find(w => w.id === i)).filter(Boolean); return { s, page, ws, w: ws[0] || null }; };
        const exec = cmd => (cmd ? D.exec(cmd) : false), reg = (key, ctl, read) => { fields.set(key, { ctl, read }); return ctl; };
        const view = w => { const b = w && w.bindings && w.bindings.primary; return b ? store.view(b) : null; };
        function errorText(r) { switch (r.error) {
            case 'required': return t('homedash.inspector.value_required'); case 'bad_number': return t('homedash.inspector.value_number'); case 'bad_colour': return t('homedash.inspector.value_colour');
            case 'range': return Number.isFinite(r.min) && Number.isFinite(r.max) ? t('homedash.inspector.value_range', { min: r.min, max: r.max }) : Number.isFinite(r.min) ? t('homedash.inspector.value_min', { min: r.min }) : t('homedash.inspector.value_max', { max: r.max });
            default: return t('homedash.error.' + r.error); } }
        const focused = ctl => { const a = doc().activeElement; return !!a && (ctl.el === a || (ctl.wrap && ctl.wrap.contains(a))); };
        // Text field that flags what the server would refuse and never executes it; blur restores the last valid value and seals the typing window.
        function inlineText(key, label, o2) {
            let field = null, bad = false;
            const ctl = reg(key, F.text({ key, max: o2.max, label, placeholder: o2.placeholder, value: o2.read(),
                onInput: v => { const issue = F.textIssue(v); bad = !!issue; if (issue) { field.error(t('homedash.error.' + issue)); return; } field.error(''); o2.apply(v); },
                onBlur: v => { if (bad || (o2.required && !String(v).trim())) { ctl.set(o2.read()); field.error(''); bad = false; } D.seal(); } }), o2.read);
            field = F.field({ label, control: ctl, help: o2.help }); return field;
        }
        function disclosure(key, label, read, pick, allowAuto) {
            const gridId = uid('hd-ig'), name = n => (n ? t('homedash.icon.' + n) : t('homedash.inspector.icon_auto')), preview = el('span', { class: 'hd-insp-icopv' }), text = el('span', { text: '' });
            const grid = F.iconGrid({ t, label, value: read(), allowAuto, onPick: n => pick(n) }), box = el('div', { class: 'hd-icongrid-box', id: gridId, hidden: true }, grid.el);
            const btn = el('button', { type: 'button', class: 'hd-btn hd-insp-iconbtn', 'data-tone': 'default', 'aria-expanded': 'false', 'aria-controls': gridId, 'data-hd-field': key }, [preview, text]);
            const paint = v => { preview.replaceChildren(E.icons.icon(v || 'mode-auto')); text.textContent = name(v); };
            btn.addEventListener('click', () => { const on = btn.getAttribute('aria-expanded') !== 'true'; btn.setAttribute('aria-expanded', on ? 'true' : 'false'); setAttr(box, 'hidden', on ? null : true); if (on) grid.focus(); });
            const ctl = { el: btn, set(v) { paint(v || ''); grid.set(v || ''); }, focus() { btn.focus(); } }; ctl.set(read()); reg(key, ctl, read);
            return F.field({ label, control: ctl, wrap: el('div', { class: 'hd-insp-disclosure' }, [btn, box]) });
        }
        function familySelect(key, label, ids, mixedLabel) {
            const k = cur(), dash = (k.s.doc && k.s.doc.look && k.s.doc.look.family) || 'glass', fam = () => { const ws = cur().ws, set = new Set(ws.map(w => w.family || '')); return set.size === 1 ? [...set][0] : '__mixed'; };
            const opts = (mixed) => (mixed ? [{ id: '__mixed', label: t('homedash.inspector.family_mixed'), disabled: true }] : []).concat([{ id: '', label: mixedLabel || t('homedash.inspector.family_default', { family: t('homedash.family.' + dash) }) }], catalog.families().map(f => ({ id: f, label: t('homedash.family.' + f) })));
            const ctl = F.select({ key, label, options: opts(fam() === '__mixed'), value: fam(), onChange: v => { if (v !== '__mixed') exec(C.applyFamily(D.doc(), { pageId: cur().s.pageId, ids: cur().ws.map(w => w.id), family: v })); } });
            reg(key, { el: ctl.el, wrap: ctl.wrap, set(v) { ctl.options(opts(v === '__mixed'), v); }, focus: ctl.focus }, fam); return F.field({ label, control: ctl });
        }
        // ---- page ----
        function buildPage() {
            const k = cur(), pageId = k.s.pageId, read = () => (cur().page.name || '');
            body.appendChild(el('p', { class: 'hd-insp-note', text: t('homedash.inspector.page_hint') }));
            body.appendChild(inlineText('page:name', t('homedash.inspector.page_name'), { max: 40, read, required: true, apply: v => { if (v.trim()) exec(C.renamePage(D.doc(), { pageId, name: v })); } }).el);
            body.appendChild(disclosure('page:icon', t('homedash.inspector.page_icon'), () => cur().page.icon || 'dashboard', n => exec(C.setPageIcon(D.doc(), { pageId, icon: n || undefined })), false).el);
            const tg = reg('page:compact', F.toggle({ key: 'page:compact', label: t('homedash.inspector.page_compact'), value: cur().page.compact !== false, onChange: on => exec(C.setPageCompact(D.doc(), { pageId, compact: on })) }), () => cur().page.compact !== false);
            body.appendChild(F.field({ label: t('homedash.inspector.page_compact'), control: tg, inline: true }).el);
        }
        // ---- data ----
        function buildData(w) {
            const spec = catalog.widget(w.type) || { bindings: [] }, k = cur();
            if (draftOf(catalog, w)) body.appendChild(el('p', { class: 'hd-insp-note', 'data-tone': 'warn', text: t('homedash.inspector.draft_note') }));
            for (const b of spec.bindings || []) {
                const label = b.name === 'primary' ? t('homedash.inspector.binding_primary') : b.name === 'humidity' ? t('homedash.inspector.binding_humidity') : b.name;
                const pk = E.entityPicker.create({ designer: D, t, lang, catalog, store, api, timers, type: w.type, binding: b.name, label, required: b.required === true, value: (w.bindings || {})[b.name],
                    onPick: entity => exec(C.bind(D.doc(), { pageId: k.s.pageId, id: w.id, binding: b.name, entity })) });
                pickers.push(pk); fields.set('binding:' + b.name, { ctl: { el: pk.input, wrap: pk.el, set: v => pk.set(v), focus: pk.focus }, read: () => ((cur().w || {}).bindings || {})[b.name] }); body.appendChild(pk.el);
            }
        }
        // ---- look ----
        function buildLook(w) {
            const k = cur(), pageId = k.s.pageId, wid = w.id, spec = catalog.widget(w.type) || { variants: [] }, def = E.widgets.get(w.type), ids = (spec.variants || []).map(v => v.id).filter(v => !def || def.variants.includes(v));
            const gallery = el('div', { class: 'hd-insp-variants', role: 'radiogroup', 'aria-label': t('homedash.inspector.variant') }), hits = [];
            for (const v of ids) {
                const box = el('div', { class: 'hd-insp-thumb' }), hit = el('button', { type: 'button', class: 'hd-insp-vhit', role: 'radio', 'aria-checked': w.variant === v ? 'true' : 'false', tabindex: '-1', 'data-hd-field': 'variant:' + v, text: variantName(w.type, v) });
                gallery.appendChild(el('div', { class: 'hd-insp-variant' }, [box, hit])); hits.push(hit);
                thumbs.push(E.samples.thumb({ widget: Object.assign({}, w, { variant: v, options: catalog.options(w) }), catalog, t, lang, box: { width: 132, height: 80 }, parent: box, entity: b => { const e = (w.bindings || {})[b]; return e ? store.view(e) : null; } }));
            }
            const paintV = () => { const now = (cur().w || {}).variant; hits.forEach((h, i) => { const on = h.getAttribute('data-hd-field') === 'variant:' + now; h.setAttribute('aria-checked', on ? 'true' : 'false'); h.setAttribute('tabindex', on || (!hits.some(x => x.getAttribute('aria-checked') === 'true') && i === 0) ? '0' : '-1'); }); };
            const chooseV = v => exec(C.variant(D.doc(), { pageId, id: wid, variant: v, limits: catalog.constraints({ type: w.type, variant: v }) }));
            gallery.addEventListener('click', ev => { const h = ev.target && ev.target.closest ? ev.target.closest('.hd-insp-vhit') : null; if (h) chooseV(h.getAttribute('data-hd-field').slice(8)); });
            gallery.addEventListener('keydown', ev => { const at = hits.findIndex(h => h === ev.target); if (at < 0) return; const n = F.gridMove(ev.key, at, [hits.length], 2); if (n === null) return; ev.preventDefault(); hits[n].focus(); chooseV(hits[n].getAttribute('data-hd-field').slice(8)); });
            paintV(); fields.set('variants', { ctl: { el: gallery, set: paintV, focus() { (hits.find(h => h.getAttribute('tabindex') === '0') || hits[0]).focus(); } }, read: () => null });
            body.appendChild(F.section(t('homedash.inspector.variant'), gallery));
            const entityName = (view(w) || {}).name, ttl = inlineText('title', t('homedash.inspector.title'), { max: 80, placeholder: entityName || typeName(w.type), read: () => (cur().w || {}).title || '',
                apply: v => exec(C.setWidget(D.doc(), { pageId, id: wid, path: ['title'], value: v || undefined, text: true, label: 'homedash.cmd.title' })) });
            body.appendChild(ttl.el);
            body.appendChild(disclosure('icon', t('homedash.inspector.icon'), () => (cur().w || {}).icon || '', n => exec(C.setWidget(D.doc(), { pageId, id: wid, path: ['icon'], value: n || undefined, label: 'homedash.cmd.icon' })), true).el);
            body.appendChild(familySelect('family', t('homedash.inspector.family'), [wid]).el);
            if (familyCount(k.s) > 2) body.appendChild(el('p', { class: 'hd-insp-note', 'data-tone': 'warn', text: t('homedash.inspector.family_warn') }));
        }
        // ---- options ----
        function enumLabel(v) { return v === 'start' ? t('homedash.inspector.enum_start') : v === 'center' ? t('homedash.inspector.enum_center') : v; }
        function optionField(w, spec) {
            const k = cur(), pageId = k.s.pageId, wid = w.id, f = fieldFor(spec, (w.options || {})[spec.key], { pages: k.s.doc.pages, currentPageId: pageId }); if (!f) return null;
            const key = 'option:' + spec.key, label = t('homedash.option.' + spec.key + '.label') + (f.unit ? ' (' + f.unit + ')' : ''), read = () => { const x = cur().w; return fieldFor(spec, ((x || {}).options || {})[spec.key], { pages: cur().s.doc.pages, currentPageId: pageId }).value; };
            let field = null, bad = false, ctl = null;
            const apply = (raw, live) => { const r = commitValue(spec, raw); bad = !r.ok; if (!r.ok) { field.error(errorText(r)); return false; } field.error('');
                exec(C.setWidget(D.doc(), { pageId, id: wid, path: ['options', spec.key], value: r.value, label: 'homedash.cmd.option', text: !!live })); return true; };
            switch (f.control) {
                case 'switch': ctl = F.toggle({ key, label, value: f.value, onChange: on => apply(on) }); break;
                case 'number': ctl = F.number({ key, label, value: f.value, min: f.min, max: f.max, step: f.step, unit: f.unit, onCommit: raw => apply(raw) }); break;
                case 'text': ctl = F.text({ key, label, value: f.value, max: f.maxLen, onInput: v => apply(v, true), onBlur: () => { if (bad) { ctl.set(read()); field.error(''); bad = false; } D.seal(); } }); break;
                case 'select': ctl = F.select({ key, label, value: f.value, options: f.options.map(v => ({ id: v, label: enumLabel(v) })), onChange: v => apply(v) }); break;
                case 'color': ctl = F.colour({ key, label, value: f.value, allowEmpty: f.allowEmpty, clearLabel: t('homedash.inspector.colour_auto'), placeholder: t('homedash.inspector.colour_auto'), onInput: hex => apply(hex, true),
                    onCommit: hex => { apply(hex); D.seal(); }, onInvalid: () => field.error(t('homedash.inspector.value_colour')) }); break;
                default: ctl = F.select({ key, label, value: f.value, options: [{ id: '', label: t('homedash.inspector.page_choose') }].concat(f.options.map(p => ({ id: p, label: (k.s.doc.pages.find(x => x.id === p) || {}).name || p }))), onChange: v => apply(v) });
            }
            reg(key, ctl, read); field = F.field({ label, help: f.help ? t('homedash.option.' + spec.key + '.help') : '', required: f.required ? t('homedash.inspector.required') : false, control: ctl, inline: f.control === 'switch' }); return field;
        }
        function buildOptions(w) { for (const spec of catalog.optionSpecs(w.type)) { const f = optionField(w, spec); if (f) body.appendChild(f.el); } }
        // ---- actions ----
        function buildActions(w) {
            const k = cur(), pageId = k.s.pageId, wid = w.id, pages = k.s.doc.pages.filter(p => p.id !== pageId), kind = (view(w) || {}).kind;
            const label = c => ({ default: t('homedash.inspector.do_default'), toggle: t('homedash.inspector.do_toggle'), more_info: t('homedash.inspector.do_more_info'), navigate: t('homedash.inspector.do_navigate'), none: t('homedash.inspector.do_none') })[c];
            for (const which of ['tap', 'hold']) {
                const whichLabel = which === 'tap' ? t('homedash.inspector.tap') : t('homedash.inspector.hold'), current = (w.actions || {})[which], choice = current && current.do ? current.do : 'default', choices = actionChoices({ canToggle: catalog.canToggle(w.type), entityKind: kind, pages, current });
                const sel = F.select({ key: 'action:' + which, label: whichLabel, options: choices.map(c => ({ id: c, label: label(c) })), value: choice, onChange: v => exec(C.setWidget(D.doc(), { pageId, id: wid, path: ['actions', which], value: actionValue(v, null, pages), label: 'homedash.cmd.action' })) });
                reg('action:' + which, sel, () => { const a = ((cur().w || {}).actions || {})[which]; return a && a.do ? a.do : 'default'; }); body.appendChild(F.field({ label: whichLabel, control: sel }).el);
                if (choice === 'navigate') { const pg = F.select({ key: 'action:' + which + ':page', label: t('homedash.inspector.target_page'), options: pages.map(p => ({ id: p.id, label: p.name })), value: current.page || '', onChange: v => exec(C.setWidget(D.doc(), { pageId, id: wid, path: ['actions', which], value: actionValue('navigate', v, pages), label: 'homedash.cmd.action' })) });
                    reg('action:' + which + ':page', pg, () => (((cur().w || {}).actions || {})[which] || {}).page || ''); body.appendChild(F.field({ label: t('homedash.inspector.target_page'), control: pg }).el); }
            }
            const spec = catalog.widget(w.type) || {}; if (spec.commands && Array.isArray(spec.commands.primary) && spec.commands.primary.length) {
                const tg = reg('confirm', F.toggle({ key: 'confirm', label: t('homedash.inspector.confirm'), value: w.confirm === true, onChange: on => exec(C.setWidget(D.doc(), { pageId, id: wid, path: ['confirm'], value: on, label: 'homedash.cmd.option' })) }), () => (cur().w || {}).confirm === true);
                body.appendChild(F.field({ label: t('homedash.inspector.confirm'), help: t('homedash.inspector.confirm_hint'), control: tg, inline: true }).el);
            }
        }
        // ---- several ----
        function buildMulti() {
            const k = cur(), pageId = k.s.pageId, ids = k.ws.map(w => w.id), primary = k.page.widgets.find(w => w.id === k.s.primary) || k.ws[k.ws.length - 1];
            body.appendChild(familySelect('family', t('homedash.inspector.multi_family'), ids, t('homedash.inspector.family_default', { family: t('homedash.family.' + ((k.s.doc.look && k.s.doc.look.family) || 'glass')) })).el);
            const edge = e => exec(C.align(D.doc(), { pageId, bp: cur().s.bp || 'lg', ids: cur().ws.map(w => w.id), edge: e, primaryId: cur().s.primary, C: wid => D.constraints(wid) }));
            const name = primary ? primary.title || typeName(primary.type) : '';
            body.appendChild(F.section(t('homedash.cmd.align'), [el('div', { class: 'hd-f-row' }, [F.button({ label: t('homedash.inspector.align_left'), icon: 'align-left', onClick: () => edge('left') }).el, F.button({ label: t('homedash.inspector.align_top'), icon: 'align-top', onClick: () => edge('top') }).el]),
                el('p', { class: 'hd-f-help', text: t('homedash.inspector.align_hint', { name }) })]));
            foot.appendChild(F.button({ label: t('homedash.inspector.delete_many', { count: ids.length }), icon: 'trash', tone: 'danger-quiet', onClick: () => exec(C.remove(D.doc(), { pageId, ids: cur().ws.map(w => w.id) })) }).el);
        }
        // ---- render ----
        function head(k) {
            const n = k.ws.length; let icon = 'dashboard', title = k.page.name || '', sub = t('homedash.inspector.page_title');
            if (n === 1) { const w = k.w, v = view(w); icon = E.widgets.iconFor(w, v) || 'entity'; title = w.title || (v && v.name) || typeName(w.type); sub = typeName(w.type) + DOT + variantName(w.type, w.variant); }
            else if (n > 1) { icon = 'layers'; title = t('homedash.inspector.multi_title', { count: n }); sub = ''; }
            iconBox.replaceChildren(E.icons.icon(icon)); titleEl.textContent = title; subEl.textContent = sub; setAttr(subEl, 'hidden', sub ? null : true);
        }
        function destroyBuilt() { for (const p of pickers) p.destroy(); for (const th of thumbs) th.destroy(); pickers = []; thumbs = []; fields = new Map(); }
        function rebuild(k) {
            const a = doc().activeElement, node = a && body.contains(a) && a.closest ? a.closest('[data-hd-field]') : null, keep = node ? node.getAttribute('data-hd-field') : null;
            destroyBuilt(); body.replaceChildren(); foot.replaceChildren();
            const n = k.ws.length; setAttr(tabsCtl.el, 'hidden', n === 1 ? null : true); setAttr(body, 'role', n === 1 ? 'tabpanel' : 'group'); setAttr(body, 'aria-labelledby', n === 1 ? tabsCtl.idOf(tab) : titleEl.id);
            if (n === 0) buildPage(); else if (n > 1) buildMulti(); else {
                const w = k.w; tabsCtl.set(tab);
                if (tab === 'data') buildData(w); else if (tab === 'look') buildLook(w); else if (tab === 'options') buildOptions(w); else buildActions(w);
                foot.appendChild(F.button({ label: t('homedash.inspector.delete_one'), icon: 'trash', tone: 'danger-quiet', onClick: () => exec(C.remove(D.doc(), { pageId: cur().s.pageId, ids: [w.id] })) }).el);
            }
            if (keep) { const next = body.querySelector('[data-hd-field="' + keep + '"]'); if (next && typeof next.focus === 'function') next.focus(); }
        }
        function render() {
            if (destroyed) return; const k = cur(), key = structureKey(k.s, tab); head(k);
            if (key !== lastKey) { lastKey = key; rebuild(k); return; }
            for (const f of fields.values()) if (!focused(f.ctl)) f.ctl.set(f.read(k.s));
        }
        function setTab(next) { if (!TABS.includes(next)) return; tab = next; render(); }
        life.add(D.on('state', render)); life.add(store.on('change', () => { for (const th of thumbs) th.update(); }));
        life.on(root, 'keydown', ev => { if (ev.key !== 'Escape' || ev.defaultPrevented) return; ev.preventDefault(); const s = D.state(); if (s.drawers) D.closePanels(); const idf = s.primary || (s.selection || [])[0]; if (idf) D.focusFrame(idf); });
        // Choose devices (Q1): one selected draft goes to the field that is missing; otherwise the first control of the body.
        function focusFirst() {
            render(); const k = cur(), draft = k.ws.length === 1 ? draftOf(catalog, k.w) : null;
            if (draft) { setTab(draft.binding ? 'data' : 'options'); const f = body.querySelector('[data-hd-field="' + (draft.binding ? 'binding:' + draft.binding : 'option:' + draft.option) + '"]'); if (f) { f.focus(); return; } }
            const first = body.querySelector('input, select, button, [tabindex="0"]'); if (first) first.focus(); else tabsCtl.focus();
        }
        render(); if (o.host) o.host.appendChild(root);
        return Object.freeze({ el: root, focusFirst, setTab, destroy() { if (destroyed) return; destroyed = true; life.dispose(); destroyBuilt(); root.remove(); } });
    }
    E.inspector = Object.freeze({ create, TABS, model: Object.freeze({ fieldFor, commitValue, actionChoices, actionValue, structureKey, familyCount, draftOf }) });
})();
```
- [ ] Inspector structure and render rules (the file is authoritative). `create({host, designer: D, t, lang, catalog, store, api})` → `{el, focusFirst(), setTab(tab), destroy()}`. Structure appended to the C15 shell:
  - `div.hd-insp`
    - `div.hd-insp-head` > `span.hd-insp-icon` + `div.hd-insp-heading` > `h2.hd-insp-title[id]` + `p.hd-insp-sub`
    - `E.form.tabs` (Data/Look/Options/Actions; hidden unless exactly one widget is selected)
    - `div.hd-insp-body[role=tabpanel|group][aria-labelledby]` (scrolls)
    - `div.hd-insp-foot`
- [ ] Inspector render rules:
  - `D.on('state', render)`. `render` compares `structureKey(state, tab)`. When it differs, rebuild: remember the focused `[data-hd-field]`, destroy pickers and thumbs, build, then refocus that field key. Otherwise patch: each registered field gets `ctl.set(read(state))` unless it contains `document.activeElement`. The head is refreshed and thumbs `update()`. `store.on('change')` → thumbs `update()`.
  - **Page** (nothing selected):
    - `inspector.page_hint` note.
    - Page name: `text`, max 40, `onInput` → `C.renamePage` (coalesced by C5), blur → `D.seal()` and restore the name if empty.
    - Page icon: disclosure `button.hd-btn.hd-insp-iconbtn[aria-expanded][aria-controls]` → inline `E.form.iconGrid` → `C.setPageIcon`.
    - Compaction: toggle → `C.setPageCompact`.
  - **Data**:
    - `inspector.draft_note` (warn) when `draftOf` is set.
    - One picker per catalog binding (label `binding_primary` / `binding_humidity` / raw name). `onPick` → `D.exec(C.bind(D.doc(), {pageId, id, binding, entity}))`.
  - **Look**:
    - Section "Style": `div.hd-insp-variants[role=radiogroup]` of `div.hd-insp-variant` > `div.hd-insp-thumb` (thumb of the variant with box 132×80; `entity` = `store.view` of the real binding, samples otherwise) + `button.hd-insp-vhit[role=radio][aria-checked][tabindex][data-hd-field="variant:<id>"]`. Arrows (`gridMove`, 2 columns) move focus and choose. Choosing → `C.variant(doc, {pageId, id, variant, limits: catalog.constraints({type, variant})})`.
    - Title: `text` max 80, placeholder = entity name or type name. `onInput` → `C.setWidget({path:['title'], text:true, label:'homedash.cmd.title'})`; blur → `D.seal()`.
    - Icon: disclosure + `iconGrid(allowAuto)` → `setWidget(['icon'], name || undefined, 'homedash.cmd.icon')`.
    - Family: select with `family_default {family}` + the 4 `homedash.family.*` → `C.applyFamily(…, {ids:[id], family})`. `inspector.family_warn` shows when `familyCount > 2`.
  - **Options**: one control per `catalog.optionSpecs(type)` via `fieldFor`. The label is `homedash.option.<key>.label` plus ` (unit)`; help is `.help` only for `HELP` keys.
    - Commits go through `commitValue` → `C.setWidget({path:['options', key], value, label:'homedash.cmd.option'})`. An error sets `field.error()` with `value_required` / `value_number` / `value_colour` / `value_range` (both bounds) / `value_min` / `value_max`.
    - Colour `onInput` → `text:true` (live preview), change → commit + `D.seal()`.
    - Enum labels: `start` → `enum_start`, `center` → `enum_center`, anything else raw. Page options show page names.
  - **Actions**:
    - tap and hold selects from `actionChoices`, labelled `do_*` → `setWidget(['actions', k], actionValue(...), 'homedash.cmd.action')`. A `target_page` select appears when the value is navigate.
    - `confirm` toggle + `confirm_hint` when the type has primary commands.
  - **Multi**:
    - Head shows `layers` and `multi_title {count}`.
    - `multi_family` select (a disabled `family_mixed` entry while the families differ) → `applyFamily` for every id.
    - Section `homedash.cmd.align` with `align_left` / `align_top` buttons → `C.align(doc, {pageId, bp, ids: selection, edge, primaryId: primary, C: id => D.constraints(id)})`, and `align_hint {name of primary}`.
    - Foot: `delete_many` → `C.remove`.
  - Single foot: `delete_one` (tone `danger-quiet`).
  - Root `keydown` Escape (not `defaultPrevented`) → `preventDefault`. In drawer mode `D.closePanels()` runs first, then `D.focusFrame(primary)` ("Escape in the inspector returns to the widget").
  - **Choose devices target** (Q1 answer): `focusFirst()` renders. With one widget selected and `draftOf` giving a binding, it switches to Data and focuses that picker's input. Otherwise it focuses the first `input, select, button, [tabindex="0"]` of the body, else the tabs. Part 1's drafts sheet (`setTab('data')` + `focusFirst()`) and `show_draft` land there.
  - Export `E.inspector = Object.freeze({create, TABS, model: Object.freeze({fieldFor, commitValue, actionChoices, actionValue, structureKey, familyCount, draftOf})})`, and `E.entityPicker = Object.freeze({create, model: Object.freeze({requests, merge, sourcesOf, LIMIT})})`.
  - **Text the server refuses (addendum).** The title, string options and the page name check `E.form.textIssue` on every `input`: an issue sets the field error (`homedash.error.<code>`, `aria-invalid`) and no `D.exec` runs; valid text clears the error and executes as before (`text:true` coalescing); blur with invalid text restores the last valid document value, clears the error and seals. `commitValue` of a string spec returns `{ok:false, error:'control_chars'|'url_not_allowed'}`.
- [ ] `inspector.test.mjs` (12 tests: the text rules, the option model, actions/structure/drafts, the picker model, the Options tab, the refused title, the variant gallery and family, page and multi-selection, the Actions tab, the picker with the keys, the unknown device):
<!-- embed: dryrun-m4/scripts/homedash-tests/inspector.test.mjs -->
```js
import assert from 'node:assert/strict'; import { eq, fc, noStyleAttr } from './harness.mjs'; import { boot, fire, flush, fakeD, fakeStore, fakeApi, docOf } from './designer-kit.mjs';
const ENTS = [{ id: 'ha:light.kitchen', source: 'ha', kind: 'light', name: 'Kitchen', area: 'Kitchen', capabilities: [{ name: 'toggle' }], state: { value: 'on' } }, { id: 'system:cpu', source: 'system', kind: 'sensor', name: 'CPU', capabilities: [{ name: 'numeric' }], state: { value: 12 } }];
const WIDGETS = [{ id: 'g1', type: 'gauge', variant: 'arc', bindings: { primary: 'system:cpu' }, options: {} }, { id: 's1', type: 'switch', variant: 'pill', bindings: {}, options: {} }, { id: 's2', type: 'switch', variant: 'tile', bindings: { primary: 'ha:switch.x' }, options: {} }];
const LAYOUT = [{ id: 'g1', x: 0, y: 0, w: 2, h: 2 }, { id: 's1', x: 2, y: 0, w: 2, h: 1 }, { id: 's2', x: 4, y: 0, w: 2, h: 2 }];
function mount(sel, o) { const k = boot('hd-inspector.js'), doc = docOf(k.E, k.cat, WIDGETS, LAYOUT), D = fakeD(k.E, k.cat, doc, Object.assign({ selection: sel }, o)), store = fakeStore({ 'system:cpu': { id: 'system:cpu', kind: 'sensor', name: 'CPU', value: 12, unit: '%', status: 'ok', available: true, icon_hint: '' } });
  const api = fakeApi(ENTS), host = k.document.createElement('div'); k.document.body.appendChild(host);
  const insp = k.E.inspector.create({ host, designer: D, t: x => x, lang: 'en', catalog: k.cat, store, api }); return Object.assign(k, { D, store, api, host, insp }); }
const field = (m, key) => m.host.querySelector('[data-hd-field="' + key + '"]');
export const tests = [
  ['textIssue: the server text rules (control, bidi, lone surrogate, urls)', () => { const { E } = boot('hd-form.js'), ti = E.form.textIssue;
    for (const ok of ['Kitchen', '20 ' + fc(0xb0) + 'C', '', 'Gl' + fc(0xfc) + 'hbirne', fc(0x1f600)]) assert.equal(ti(ok), '', JSON.stringify(ok));
    for (const bad of ['a' + fc(7) + 'b', fc(0x202e) + 'x', fc(0x85), String.fromCharCode(0xd800), fc(0x7f), fc(0x200f), fc(0x2066)]) assert.equal(ti(bad), 'control_chars', JSON.stringify(bad));
    for (const bad of ['see http://x', ' //cdn', 'JavaScript:alert', 'Data: 5', 'a vbscript:x', 'x://y']) assert.equal(ti(bad), 'url_not_allowed', bad); assert.equal(ti('a//b'), ''); assert.equal(ti(null), ''); }],
  ['inspector model: option specs become controls and commit like the server wants', () => { const { E, cat } = boot('hd-inspector.js'), M = E.inspector.model, spec = (t, k) => cat.optionSpecs(t).find(s => s.key === k), dec = spec('sensor', 'decimals'), warn = spec('gauge', 'warn'), acc = spec('sensor', 'accent');
    const f = M.fieldFor(dec, undefined, {}); assert.equal(f.control, 'number'); assert.equal(f.step, 1); assert.equal(f.min, 0); assert.equal(f.max, 3); assert.equal(M.fieldFor(warn, undefined, {}).nullable, true);
    assert.deepEqual({ ...M.commitValue(dec, '5') }, { ok: false, error: 'range', min: 0, max: 3 }); eq(M.commitValue(dec, '2'), { ok: true, value: 2 }); eq(M.commitValue(dec, String(dec.default)), { ok: true, value: undefined }); eq(M.commitValue(dec, 'x').error, 'bad_number'); eq(M.commitValue(dec, '2,6'), { ok: true, value: 3 });
    eq(M.commitValue(warn, ''), { ok: true, value: undefined }); eq(M.commitValue(warn, '80'), { ok: true, value: 80 }); eq(M.commitValue(acc, '#ABCDEF'), { ok: true, value: '#abcdef' }); eq(M.commitValue(acc, 'red').error, 'bad_colour'); eq(M.commitValue(acc, ''), { ok: true, value: undefined });
    const str = { type: 'string', key: 's', default: '', max_len: 5 }; eq(M.commitValue(str, 'a://b'), { ok: false, error: 'url_not_allowed' }); eq(M.commitValue(str, 'a' + fc(7)).error, 'control_chars'); eq(M.commitValue(str, 'abcdefgh'), { ok: true, value: 'abcde' }); eq(M.commitValue({ type: 'string', required: true, default: '' }, '').error, 'required');
    eq(M.commitValue({ type: 'bool', default: false }, true), { ok: true, value: true }); eq(M.commitValue({ type: 'enum', enum: ['a', 'b'], default: 'a' }, 'b'), { ok: true, value: 'b' }); eq(M.commitValue({ type: 'enum', enum: ['a', 'b'], default: 'a' }, 'z').error, 'required');
    const pg = M.fieldFor({ type: 'page', key: 'page', default: '' }, 'p1', { pages: [{ id: 'p1' }, { id: 'p2' }], currentPageId: 'p1' }); eq(pg.options, ['p2']); assert.equal(M.fieldFor({ type: 'nope' }, 1, {}), null); }],
  ['inspector model: action choices, structure key and drafts', () => { const { E, cat } = boot('hd-inspector.js'), M = E.inspector.model;
    eq(M.actionChoices({ canToggle: true, entityKind: 'light', pages: [{ id: 'p2' }], current: null }), ['default', 'toggle', 'more_info', 'navigate', 'none']); eq(M.actionChoices({ canToggle: false, entityKind: 'sensor', pages: [] }), ['default', 'none']);
    eq(M.actionValue('navigate', 'gone', [{ id: 'p2' }]), { do: 'navigate', page: 'p2' }); assert.equal(M.actionValue('default'), undefined); eq(M.actionValue('none'), { do: 'none' }); assert.equal(M.actionValue('navigate', 'x', []), undefined);
    const doc = docOf(E, cat, WIDGETS, LAYOUT), s = (sel, ex) => Object.assign({ doc, pageId: 'main', page: doc.pages[0], selection: sel, primary: sel[sel.length - 1] }, ex), k1 = M.structureKey(s(['g1']), 'look');
    assert.notEqual(k1, M.structureKey(s(['g1']), 'data')); assert.notEqual(k1, M.structureKey(s(['s1']), 'look')); assert.notEqual(k1, M.structureKey(s([]), 'look'));
    const d2 = E.doc.apply(doc, E.commands.setWidget(doc, { pageId: 'main', id: 'g1', path: ['title'], value: 'Hello', now: 1 }).ops); assert.equal(M.structureKey(s(['g1'], { doc: d2, page: d2.pages[0] }), 'look'), k1, 'a title change patches in place');
    const d3 = E.doc.apply(doc, E.commands.variant(doc, { pageId: 'main', id: 'g1', variant: 'needle', limits: cat.constraints({ type: 'gauge', variant: 'needle' }) }).ops); assert.notEqual(M.structureKey(s(['g1'], { doc: d3, page: d3.pages[0] }), 'look'), k1);
    eq(M.draftOf(cat, WIDGETS[1]), { binding: 'primary' }); assert.equal(M.draftOf(cat, WIDGETS[0]), null); assert.equal(M.familyCount({ doc, page: doc.pages[0] }), 1); }],
  ['entity picker model: one request per allowed kind, merge dedupes, filters and sorts', () => { const { E } = boot('hd-entity-picker.js'), M = E.entityPicker.model, r = M.requests({ q: 'k' }, ['switch', 'light']);
    eq(r.map(x => x.kind), ['switch', 'light']); assert.equal(r[0].limit, 50); assert.equal(r[0].q, 'k'); eq(M.requests({ kind: 'light' }, ['switch', 'light']).map(x => x.kind), ['light']); eq(M.requests({}, []).map(x => x.kind), [undefined]); assert.ok(!('kind' in M.requests({}, [])[0]));
    const a = { entities: [{ id: 'b', name: 'Beta', kind: 'light' }, { id: 'a', name: 'alpha', kind: 'switch' }], sources: [{ source: 'ha' }] }, b = { entities: [{ id: 'a', name: 'dup', kind: 'switch' }, { id: 'c', name: 'Gamma', kind: 'sensor' }, null], sources: [{ source: 'system' }, { source: 'ha' }] };
    eq(M.merge([a, b, null], e => e.kind !== 'sensor', new Intl.Collator('en').compare).map(e => e.id), ['a', 'b']); eq(M.sourcesOf([a, b, null]), ['ha', 'system']); }],
  ['inspector DOM: Options tab commits one setWidget per change, shows range errors, keeps the focused field', () => { const m = mount(['g1']); m.insp.setTab('options'); const dec = field(m, 'option:decimals'), n0 = m.D.execs.length;
    dec.value = '2'; fire(dec, 'change', {}); assert.equal(m.D.execs.length, n0 + 1); eq(m.D.execs[n0].ops[0].path, ['pages', 'main', 'widgets', 'g1', 'options', 'decimals']); assert.equal(m.D.execs[n0].ops[0].after, 2); assert.equal(m.D.doc().pages[0].widgets[0].options.decimals, 2);
    assert.equal(field(m, 'option:decimals'), dec, 'patched in place, not rebuilt'); dec.value = '9'; fire(dec, 'change', {}); assert.equal(m.D.execs.length, n0 + 1); assert.equal(dec.getAttribute('aria-invalid'), 'true');
    const acc = field(m, 'option:accent'); acc.value = '#ABCDEF'; fire(acc, 'change', {}); assert.equal(m.D.doc().pages[0].widgets[0].options.accent, '#abcdef'); assert.ok(m.D.calls.includes('seal')); noStyleAttr(m.host); }],
  ['inspector DOM: title text the server refuses is flagged and never executed; blur restores the last valid value', () => { const m = mount(['g1']); m.insp.setTab('look'); const ti = field(m, 'title'), n0 = m.D.execs.length;
    ti.value = 'http://x'; fire(ti, 'input', {}); assert.equal(m.D.execs.length, n0); assert.equal(ti.getAttribute('aria-invalid'), 'true');
    ti.value = 'Kitchen'; fire(ti, 'input', {}); assert.equal(m.D.execs.length, n0 + 1); assert.equal(ti.getAttribute('aria-invalid'), null); assert.ok(m.D.execs[n0].meta.coalesce && m.D.execs[n0].meta.coalesce.startsWith('field|g1|title'), 'text coalesces');
    ti.value = 'x' + fc(0x202e); fire(ti, 'input', {}); assert.equal(m.D.execs.length, n0 + 1); fire(ti, 'blur', {}); assert.equal(ti.value, 'Kitchen'); assert.equal(ti.getAttribute('aria-invalid'), null); assert.ok(m.D.calls.includes('seal')); }],
  ['inspector DOM: the variant gallery, family select and delete are one command each', () => { const m = mount(['g1']); m.insp.setTab('look'); const gal = m.host.querySelector('.hd-insp-variants'), hits = gal.querySelectorAll('.hd-insp-vhit'); assert.ok(hits.length >= 3); assert.equal(m.host.querySelectorAll('.hd-insp-thumb .hd-thumb').length, hits.length);
    const needle = field(m, 'variant:needle'); fire(gal, 'click', {}, needle); assert.equal(m.D.doc().pages[0].widgets[0].variant, 'needle'); assert.equal(m.D.execs[0].label, 'homedash.cmd.variant');
    const fam = field(m, 'family'); fam.value = 'tactile'; fire(fam, 'change', {}); assert.equal(m.D.doc().pages[0].widgets[0].family, 'tactile');
    const del = m.host.querySelector('.hd-insp-foot .hd-btn'); fire(del, 'click', {}); assert.equal(m.D.doc().pages[0].widgets.some(w => w.id === 'g1'), false); }],
  ['inspector DOM: page properties, multi-selection align and delete, Escape returns to the widget', () => { const none = mount([]); assert.equal(none.host.querySelector('.hd-insp-title').textContent, 'Main'); const nm = field(none, 'page:name'), n0 = none.D.execs.length;
    nm.value = 'Renamed'; fire(nm, 'input', {}); assert.equal(none.D.doc().pages[0].name, 'Renamed'); assert.equal(none.D.execs.length, n0 + 1); nm.value = ''; fire(nm, 'input', {}); assert.equal(none.D.execs.length, n0 + 1); fire(nm, 'blur', {}); assert.equal(nm.value, 'Renamed');
    fire(field(none, 'page:compact'), 'click', {}); assert.equal(none.D.doc().pages[0].compact, false);
    const m = mount(['g1', 's2']); assert.equal(m.host.querySelector('.hd-insp-title').textContent, 'homedash.inspector.multi_title'); assert.ok(m.host.querySelector('.hd-f-tabs').hasAttribute('hidden'));
    const [left] = m.host.querySelectorAll('.hd-f-row .hd-btn'); fire(left, 'click', {}); assert.equal(m.D.execs.length, 1); assert.equal(m.D.execs[0].label, 'homedash.cmd.align');
    const fam = field(m, 'family'); assert.ok(fam.querySelectorAll('option').some(o => o.getAttribute('value') === 'tactile')); fam.value = 'minimal'; fire(fam, 'change', {}); assert.ok(m.D.doc().pages[0].widgets.filter(w => ['g1', 's2'].includes(w.id)).every(w => w.family === 'minimal'));
    const del = m.host.querySelector('.hd-insp-foot .hd-btn'); fire(del, 'click', {}); eq(m.D.doc().pages[0].widgets.map(w => w.id), ['s1']);
    const e = mount(['g1'], { drawers: true }); const ev = fire(e.host.querySelector('.hd-insp'), 'keydown', { key: 'Escape' }); assert.equal(ev.defaultPrevented, true); eq(e.D.calls, ['closePanels', 'focusFrame:g1']); }],
  ['inspector DOM: Actions tab offers what the widget can do and writes the action object', () => { const m = mount(['s2']); m.insp.setTab('actions'); const tap = field(m, 'action:tap'); eq(tap.querySelectorAll('option').map(o => o.getAttribute('value')), ['default', 'toggle', 'navigate', 'none']);
    tap.value = 'navigate'; fire(tap, 'change', {}); eq(m.D.doc().pages[0].widgets[2].actions.tap, { do: 'navigate', page: 'p2' }); assert.ok(field(m, 'action:tap:page'), 'the target page select appears');
    const conf = field(m, 'confirm'); fire(conf, 'click', {}); assert.equal(m.D.doc().pages[0].widgets[2].confirm, true); }],
  ['inspector DOM: Choose devices lands on the draft binding; the picker searches by kind, picks with the keys and binds', async () => { const m = mount(['s1']); m.insp.focusFirst(); const input = field(m, 'binding:primary'); assert.equal(m.document.activeElement, input); assert.equal(m.host.querySelector('.hd-ep').getAttribute('data-hd-state'), 'empty');
    assert.ok(m.host.querySelector('.hd-insp-note'), 'the draft note shows'); input.value = 'kit'; fire(input, 'input', {}); await flush(40); assert.equal(input.getAttribute('aria-expanded'), 'true'); assert.deepEqual(m.api.log.map(r => r.kind).sort(), ['fan', 'humidifier', 'light', 'switch']);
    const opts = m.host.querySelectorAll('.hd-ep-opt'); eq(opts.map(o => o.getAttribute('data-entity')), ['ha:light.kitchen']); fire(input, 'keydown', { key: 'ArrowDown' }); fire(input, 'keydown', { key: 'Enter' });
    assert.equal(m.D.doc().pages[0].widgets[1].bindings.primary, 'ha:light.kitchen'); assert.equal(m.D.execs[0].label, 'homedash.cmd.bind'); await flush(5); assert.equal(input.getAttribute('aria-expanded'), 'false');
    fire(input, 'input', {}); const ev = fire(input, 'keydown', { key: 'Escape' }); assert.equal(ev.stopped, true, 'Escape with the popup open stays in the picker'); const ev2 = fire(input, 'keydown', { key: 'Escape' }); assert.equal(ev2.stopped, undefined, 'a closed popup lets Escape bubble to the inspector');
    const clear = m.host.querySelector('.hd-ep-box .hd-btn'); fire(clear, 'click', {}); assert.equal(m.D.doc().pages[0].widgets[1].bindings.primary, undefined); noStyleAttr(m.host); m.insp.destroy(); assert.ok(m.store.released.length >= 1); }],
  ['inspector DOM: an unknown device (404 entity_not_found or store status unknown) shows the warning; a known one shows name and area', async () => { const m = mount(['s2']); await flush(20); const ep = m.host.querySelector('.hd-ep'); assert.equal(ep.getAttribute('data-hd-state'), 'unknown'); assert.equal(m.host.querySelector('.hd-ep-warn').hasAttribute('hidden'), false);
    assert.ok(m.api.log.some(r => r.entity === 'ha:switch.x')); const k = mount(['g1']); await flush(5); assert.equal(k.host.querySelector('.hd-ep').getAttribute('data-hd-state'), 'bound'); assert.ok(k.host.querySelector('.hd-ep-meta').textContent.includes('system:cpu'));
    const u = mount(['s2']); u.store.views['ha:switch.x'] = { id: 'ha:switch.x', kind: 'switch', status: 'unknown', name: 'x' }; u.insp.setTab('look'); u.insp.setTab('data'); assert.equal(u.host.querySelector('.hd-ep').getAttribute('data-hd-state'), 'unknown'); }]
];
```
- [ ] Run wiring step W, `npm run test:homedash`, `npm run check:ui`, `go test ./ui -run '(?i)^(TestHomeDashWiring|TestFrontend_StaticI18nKeysExistInEnglishBundle|TestFrontend_JS_NoLiteralTranslationFallbacks)$' -count=1`. All PASS. Commit — `feat(homedash): inspector tabs, multi-selection and the device picker`.

**Batch C review (reader-agent, Critical/Important only):**
- Palette = catalog ∩ registry with nothing hard-coded.
- One add per click; no double add after a drag; the drawer closes before the focus moves.
- Entity paging stops when no new rows arrive; at most 20 watched ids; the watch is released on destroy and when the picker closes.
- Every field change is one command; text coalesces and seals on blur; no rebuild steals the caret.
- Escape never clears the selection from a panel; `focusFirst` reaches the draft binding.
- Untrusted names only via `textContent`.

security-agent (D14): entity names and areas in rows and options, and the `api.entity` cache.

---

## Batch D — look editor, pages, dialogs, first run (F5; D7, D8, D10)

### Task D0: Batch D texts
**Files:** 16 locale files, `i18n.test.mjs` (`SAME_WORD_KEYS.de` adds `homedash.dialog.name` — "Name is the German word"). The `look.*`, `dialog.*` and `menu.page_*` keys and the changed `home.create_empty_hint` are in the key files of C0 (one merge, one commit with C0 if the maker prefers).
- [ ] PASS as in C0. Commit — `feat(homedash): look editor and dialog texts in 16 locales`.

### Task D1: hd-look.js
**Files:** create `hd-look.js` (its tests are in `dialogs.test.mjs` of D2; one commit with D2). Wiring step W. **Existing symbols changed:** none.
- [ ] `hd-look.js`, in full (model `E.look.model` = `custom, paletteFor, radius, rowUnit, swatches, previewWidgets, hueOf, FAMILIES, MODES, BACKDROPS, DENSITIES`, and `open(ctx)`):
<!-- embed: dryrun-m4/ui/js/homedash/hd-look.js -->
```js
// HomeDash look editor (M4 part 2, D7, design 5.12): family cards with live previews of three widgets of the dashboard, colour mode (theme, dark,
// light, custom with accent, surface hue and scheme), backdrop, density, radius, fit, row unit and page compaction, in one end-docked E.sheet form
// over a live canvas. Every field is one undoable D.exec(E.commands.setDoc ...) (sliders and the native colour input use text:true while they move
// and seal on change). The pure model follows the server rules: lower-case #rrggbb accent, integer surface_hue 0-359, scheme dark|light.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.form || !E.samples || !E.commands || !E.colour) throw new Error('hd-look.js: load hd-form.js, hd-samples.js and hd-commands.js first');
    const { el, svg, setAttr, setVars, uid } = E.core, F = E.form, C = E.commands;
    const FAMILIES = Object.freeze(['glass', 'tactile', 'minimal', 'instrument']), MODES = Object.freeze(['theme', 'dark', 'light', 'custom']), BACKDROPS = Object.freeze(['theme', 'solid', 'gradient']), DENSITIES = Object.freeze(['compact', 'comfortable', 'spacious']);
    const HEX = /^#?([0-9a-f]{6})$/i, SLOT_A = Object.freeze({ width: 64, height: 88 }), SLOT_B = Object.freeze({ width: 64, height: 40 });
    const hueOf = v => { const n = Math.round(Number(v)); return Number.isFinite(n) ? ((n % 360) + 360) % 360 : 220; };
    // Server rules (dashboard_validate.go): lower-case #rrggbb accent, integer surface_hue 0-359, scheme dark|light.
    function custom(c, seed) { const x = c && typeof c === 'object' ? c : {}, f = seed || {}, m = HEX.exec(String(x.accent || '').trim()) || HEX.exec(String(f.accent || '').trim());
        return { accent: m ? '#' + m[1].toLowerCase() : '#5b8def', surface_hue: hueOf(x.surface_hue !== undefined ? x.surface_hue : f.surface_hue), scheme: (x.scheme || f.scheme) === 'light' ? 'light' : 'dark' }; }
    // The palette object a mode change writes (server shape). The custom block is kept so custom comes back as it was; E.doc.forSave drops it unless
    // the mode is custom and writes the theme snapshot through the lane's prepare (part 1).
    function paletteFor(current, mode, seed) { const cur = current && typeof current === 'object' ? current : {}, out = Object.assign({}, cur, { mode: MODES.includes(mode) ? mode : 'theme' });
        if (out.mode === 'custom' || cur.custom) out.custom = custom(cur.custom, seed); return out; }
    const radius = v => Math.round(E.core.clamp(Number(v) || 1, 0.5, 1.5) * 20) / 20;
    const rowUnit = v => (v === 'auto' || v === undefined || v === null || v === '' ? 'auto' : Math.round(E.core.clamp(Number(v) || 96, 48, 200)));
    function swatches(c) { const k = E.colour.solvePalette(custom(c)).tokens; return { canvas: k.canvas, surface: k['surface-solid'], ink: k.ink, accent: k.accent, on: k.on }; }
    const SAMPLES = Object.freeze([['light', 'dial', 'a'], ['switch', 'pill', 'b'], ['temperature', 'numeral', 'c']]);
    // Family card previews (D7): one square widget (thumb 2x2, slot a) and two wide ones (2x1, slots b and c) of the page in reading order; samples fill gaps.
    function previewWidgets(page, catalog, registry) {
        const out = {}, items = page && page.layouts ? E.grid.sortReading(page.layouts.lg) : [];
        for (const it of items) { const w = page.widgets.find(x => x.id === it.id); if (!w || !registry.get(w.type)) continue; const s = E.samples.thumbSize(catalog, w.type, w.variant);
            const slot = s.w === 2 && s.h === 2 ? 'a' : s.h === 1 ? (out.b ? 'c' : 'b') : null; if (slot && !out[slot]) out[slot] = { widget: w, real: true }; }
        for (const [type, variant, slot] of SAMPLES) if (!out[slot] && registry.get(type) && catalog.variant(type, variant))
            out[slot] = { widget: { type, variant, title: '', bindings: {}, options: catalog.optionDefaults(type), actions: {}, confirm: false }, real: false };
        return ['a', 'b', 'c'].filter(k => out[k]).map(k => Object.assign({ slot: k, size: k === 'a' ? { w: 2, h: 2 } : { w: 2, h: 1 } }, out[k]));
    }

    function open(ctx) {
        const D = ctx.designer, t = ctx.t || D.t, lang = ctx.lang || D.lang || 'en', catalog = ctx.catalog || D.catalog, store = ctx.store || D.store, life = E.core.bag(), thumbs = [], controls = [];
        const set = (path, value, label, text) => D.exec(C.setDoc(D.doc(), { path, value, label: label || 'homedash.cmd.look', text: !!text }));
        const doc = () => D.doc(), look = () => doc().look, seed = () => ({ accent: (E.widgets.paletteSnapshot(ctx.root, ['accent']) || {}).accent, scheme: ctx.root && ctx.root.getAttribute && ctx.root.getAttribute('data-hd-scheme') === 'light' ? 'light' : 'dark', surface_hue: 250 });
        const ownerDoc = () => (ctx.root && ctx.root.ownerDocument) || document, focused = ctl => { const a = ownerDoc().activeElement; return !!a && (ctl.el === a || (ctl.wrap && ctl.wrap.contains(a))); };
        const track = (ctl, read) => { controls.push({ ctl, read }); return ctl; };
        const content = el('div', { class: 'hd-look' });
        // Family
        const famEls = FAMILIES.map(f => { const prev = el('div', { class: 'hd-look-preview' }), hit = el('button', { type: 'button', class: 'hd-look-fhit', role: 'radio', 'aria-checked': 'false', tabindex: '-1', 'data-family': f },
            [el('strong', { text: t('homedash.family.' + f) }), el('small', { text: ({ glass: t('homedash.look.desc_glass'), tactile: t('homedash.look.desc_tactile'), minimal: t('homedash.look.desc_minimal'), instrument: t('homedash.look.desc_instrument') })[f] })]);
            for (const p of previewWidgets(doc().pages.find(x => x.id === D.state().pageId) || doc().pages[0], catalog, E.widgets)) { const slot = el('span', { class: 'hd-look-slot', 'data-slot': p.slot }); prev.appendChild(slot);
                thumbs.push(E.samples.thumb({ widget: Object.assign({}, p.widget, { family: f }), catalog, t, lang, size: p.size, box: p.slot === 'a' ? SLOT_A : SLOT_B, parent: slot, entity: b => { const e = p.real && (p.widget.bindings || {})[b]; return e ? store.view(e) : null; } })); }
            return { f, card: el('div', { class: 'hd-look-family' }, [prev, hit]), hit }; });
        const famGroup = el('div', { class: 'hd-look-families', role: 'radiogroup', 'aria-label': t('homedash.look.family') }, famEls.map(x => x.card));
        const paintFam = () => famEls.forEach((x, i) => { const on = x.f === look().family; x.hit.setAttribute('aria-checked', on ? 'true' : 'false'); x.hit.setAttribute('tabindex', on || (!FAMILIES.includes(look().family) && i === 0) ? '0' : '-1'); });
        const chooseFam = f => set(['look', 'family'], f);
        famGroup.addEventListener('click', ev => { const h = ev.target && ev.target.closest ? ev.target.closest('.hd-look-fhit') : null; if (h) chooseFam(h.getAttribute('data-family')); });
        famGroup.addEventListener('keydown', ev => { const at = famEls.findIndex(x => x.hit === ev.target); if (at < 0) return; const n = F.gridMove(ev.key, at, [famEls.length], 2); if (n === null) return; ev.preventDefault(); famEls[n].hit.focus(); chooseFam(famEls[n].f); });
        controls.push({ ctl: { el: famGroup, set: paintFam }, read: () => null }); content.appendChild(F.section(t('homedash.look.family'), famGroup));
        // Colours
        const modeLabels = { theme: t('homedash.look.mode_theme'), dark: t('homedash.look.mode_dark'), light: t('homedash.look.mode_light'), custom: t('homedash.look.mode_custom') };
        const mode = track(F.segmented({ key: 'palette-mode', label: t('homedash.look.colours'), options: MODES.map(m => ({ id: m, label: modeLabels[m] })), value: look().palette.mode, onChange: m => set(['look', 'palette'], paletteFor(look().palette, m, seed())) }), () => look().palette.mode);
        const customBox = el('div', { class: 'hd-look-custom', hidden: true });
        const accent = track(F.colour({ key: 'palette-accent', label: t('homedash.look.accent'), value: '', onInput: hex => { set(['look', 'palette', 'custom', 'accent'], hex, null, true); paintSw(); }, onCommit: hex => { set(['look', 'palette', 'custom', 'accent'], hex); D.seal(); paintSw(); } }), () => custom(look().palette.custom).accent);
        const gid = uid('hd-hue'), hue = svg('svg', { class: 'hd-look-hue', 'aria-hidden': 'true', viewBox: '0 0 100 8', preserveAspectRatio: 'none' }), grad = svg('linearGradient', { id: gid, x1: '0', x2: '1', y1: '0', y2: '0' });
        for (let i = 0; i <= 12; i++) grad.appendChild(svg('stop', { offset: String(Math.round(i / 12 * 100) / 100), 'stop-color': E.colour.oklchToHex([0.72, 0.11, i * 30]).toLowerCase() }));
        hue.append(svg('defs', {}, grad), svg('rect', { x: '0', y: '0', width: '100', height: '8', fill: 'url(#' + gid + ')' }));
        const hueCtl = track(F.range({ key: 'palette-hue', label: t('homedash.look.surface_hue'), min: 0, max: 359, step: 1, value: 220, track: hue, onInput: v => { set(['look', 'palette', 'custom', 'surface_hue'], hueOf(v), null, true); paintSw(); }, onCommit: v => { set(['look', 'palette', 'custom', 'surface_hue'], hueOf(v)); D.seal(); paintSw(); } }), () => custom(look().palette.custom).surface_hue);
        const scheme = track(F.segmented({ key: 'palette-scheme', label: t('homedash.look.scheme'), options: [{ id: 'dark', label: t('homedash.look.mode_dark') }, { id: 'light', label: t('homedash.look.mode_light') }], value: 'dark', onChange: v => { set(['look', 'palette', 'custom', 'scheme'], v); paintSw(); } }), () => custom(look().palette.custom).scheme);
        const swEls = ['canvas', 'surface', 'ink', 'accent', 'on'].map(n => el('span', { class: 'hd-look-swatch', 'data-swatch': n })), swBox = el('div', { class: 'hd-look-swatches', 'aria-hidden': 'true' }, swEls);
        function paintSw() { const sw = swatches(look().palette.custom); swEls.forEach(s => setVars(s, { '--hd-e-sw': sw[s.getAttribute('data-swatch')] })); }
        customBox.append(F.field({ label: t('homedash.look.accent'), control: accent }).el, F.field({ label: t('homedash.look.surface_hue'), control: hueCtl }).el, F.field({ label: t('homedash.look.scheme'), control: scheme }).el, swBox, el('p', { class: 'hd-f-help', text: t('homedash.look.custom_hint') }));
        content.appendChild(F.section(t('homedash.look.colours'), [F.field({ label: t('homedash.look.colours'), control: mode }).el, customBox]));
        // Surface
        const backdrop = track(F.segmented({ key: 'backdrop', label: t('homedash.look.backdrop'), options: [{ id: 'theme', label: t('homedash.look.mode_theme') }, { id: 'solid', label: t('homedash.look.backdrop_solid') }, { id: 'gradient', label: t('homedash.look.backdrop_gradient') }], value: 'theme',
            onChange: v => set(['look', 'backdrop'], { kind: v }) }), () => (look().backdrop && look().backdrop.kind) || 'theme');
        const density = track(F.segmented({ key: 'density', label: t('homedash.look.density'), options: [{ id: 'compact', label: t('homedash.look.density_compact') }, { id: 'comfortable', label: t('homedash.look.density_comfortable') }, { id: 'spacious', label: t('homedash.look.density_spacious') }], value: 'comfortable',
            onChange: v => set(['look', 'density'], v) }), () => look().density);
        const nf = new Intl.NumberFormat(lang, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
        const rad = track(F.range({ key: 'radius', label: t('homedash.look.radius'), min: 0.5, max: 1.5, step: 0.05, value: 1, valueText: v => t('homedash.look.radius_value', { value: nf.format(v) }), onInput: v => set(['look', 'radius_scale'], radius(v), null, true), onCommit: v => { set(['look', 'radius_scale'], radius(v)); D.seal(); } }), () => radius(look().radius_scale));
        content.appendChild(F.section(t('homedash.look.backdrop'), [F.field({ label: t('homedash.look.backdrop'), control: backdrop }).el, F.field({ label: t('homedash.look.density'), control: density }).el, F.field({ label: t('homedash.look.radius'), control: rad }).el]));
        // Grid
        const fit = track(F.segmented({ key: 'fit', label: t('homedash.look.fit'), options: [{ id: 'scroll', label: t('homedash.look.fit_scroll') }, { id: 'fit_screen', label: t('homedash.menu.fit') }], value: 'scroll', onChange: v => set(['grid', 'fit'], v, 'homedash.cmd.grid') }), () => doc().grid.fit);
        const rowKind = track(F.segmented({ key: 'row-kind', label: t('homedash.look.row_unit'), options: [{ id: 'auto', label: t('homedash.look.row_auto') }, { id: 'fixed', label: t('homedash.look.row_fixed') }], value: 'auto',
            onChange: v => { set(['grid', 'row_unit'], v === 'auto' ? 'auto' : rowUnit(rowPx.get() || 96), 'homedash.cmd.grid'); sync(); } }), () => (doc().grid.row_unit === 'auto' ? 'auto' : 'fixed'));
        const rowPx = F.number({ key: 'row-px', label: t('homedash.look.row_unit'), min: 48, max: 200, step: 4, unit: 'px', value: 96, onCommit: raw => set(['grid', 'row_unit'], rowUnit(raw), 'homedash.cmd.grid') });
        rowPx.get = () => Number(rowPx.el.value); track(rowPx, () => (doc().grid.row_unit === 'auto' ? '' : doc().grid.row_unit));
        const compact = track(F.toggle({ key: 'compact', label: t('homedash.look.compaction'), value: true, onChange: on => D.exec(C.setPageCompact(D.doc(), { pageId: D.state().pageId, compact: on })) }), () => { const p = doc().pages.find(x => x.id === D.state().pageId); return !p || p.compact !== false; });
        const rowWrap = rowPx.wrap;
        content.appendChild(F.section(t('homedash.look.grid'), [F.field({ label: t('homedash.look.fit'), help: t('homedash.look.fit_note'), control: fit }).el, F.field({ label: t('homedash.look.row_unit'), control: rowKind }).el, rowWrap, F.field({ label: t('homedash.look.compaction'), control: compact, inline: true }).el]));
        function sync() {
            const isCustom = look().palette.mode === 'custom'; setAttr(customBox, 'hidden', isCustom ? null : true); setAttr(rowWrap, 'hidden', doc().grid.row_unit === 'auto' ? true : null);
            for (const c of controls) { if (c.ctl.el && focused(c.ctl)) continue; const v = c.read(); if (v !== null) c.ctl.set(v); else c.ctl.set(); }
            if (isCustom) paintSw();
        }
        sync();
        const off = D.on('state', sync), offStore = store.on('change', () => { for (const th of thumbs) th.update(); });
        const first = () => famEls.find(x => x.hit.getAttribute('aria-checked') === 'true') ? famEls.find(x => x.hit.getAttribute('aria-checked') === 'true').hit : famEls[0].hit;
        return F.formSheet(D.sheet(), { title: t('homedash.look.title'), content, actions: [{ id: 'done', label: t('homedash.designer.done'), tone: 'primary' }], first }).then(() => {
            off(); offStore(); life.dispose(); for (const th of thumbs) th.destroy(); thumbs.length = 0; D.seal(); return true; });
    }
    E.look = Object.freeze({ open, model: Object.freeze({ custom, paletteFor, radius, rowUnit, swatches, previewWidgets, hueOf, FAMILIES, MODES, BACKDROPS, DENSITIES }) });
})();
```
- [ ] `open(ctx)` notes (ctx.designer present; the file is authoritative). It opens `E.form.formSheet(D.sheet(), {title: look.title, content: div.hd-look, actions:[{id:'done', label: designer.done, tone:'primary'}], first: checked family radio})`. Every field is one undoable `D.exec(E.commands.setDoc(D.doc(), {path, value, label, text}))` (D7). Sliders, the hue and the native colour input use `text:true` while moving; `change` → `D.seal()`. Sections:
  - **Family**: `div.hd-look-families[role=radiogroup]` of `div.hd-look-family` > `div.hd-look-preview` (three `span.hd-look-slot[data-slot]` with thumbs from `previewWidgets`, `widget.family` forced to the card's family, boxes `SLOT_A`/`SLOT_B`, real entity views when `real`) + `button.hd-look-fhit[role=radio][aria-checked][tabindex][data-family]` > `strong` (`homedash.family.<f>`) + `small` (`look.desc_<f>`, literal switch). Arrows (`gridMove`, 2 columns) choose. → `['look','family']`.
  - **Colours**: segmented row (label span + `labelledBy`) over `MODES` (`look.mode_*`) → `['look','palette']`, `paletteFor(cur, m, seed())`, where `seed()` = `E.widgets.paletteSnapshot(ctx.root, ['accent'])` + scheme from `root[data-hd-scheme]` + hue 250. `div.hd-look-custom[hidden unless custom]`:
    - `E.form.colour` accent → `['look','palette','custom','accent']`.
    - `E.form.range` 0..359 with `track` = SVG `svg.hd-look-hue[aria-hidden]` holding a `linearGradient` of 13 stops `E.colour.oklchToHex([0.72, 0.11, h]).toLowerCase()`; `fill="url(#id)"` is an attribute, not CSS. → `custom.surface_hue` (integer).
    - Scheme segmented (`mode_dark`/`mode_light`) → `custom.scheme`.
    - `div.hd-look-swatches` of five `span.hd-look-swatch` with `--hd-e-sw` from `swatches()`.
    - `look.custom_hint`.
    - The A4 solver guarantees the ON fill.
  - **Surface**: backdrop segmented (`mode_theme`, `backdrop_solid`, `backdrop_gradient`) → `['look','backdrop']` `{kind}`. Density segmented → `['look','density']`. Radius range 0.5–1.5 step 0.05 (`radius_value`, `Intl.NumberFormat(lang, 2 decimals)`) → `['look','radius_scale']`.
  - **Grid**:
    - Fit segmented (`fit_scroll`, `homedash.menu.fit`) + help `look.fit_note` (Q5) → `['grid','fit']`, label `homedash.cmd.grid`.
    - Row height segmented auto/fixed + `E.form.number` 48–200 step 4 px (visible when fixed) → `['grid','row_unit']`, `rowUnit()`.
    - Compaction toggle (`look.compaction`) → `C.setPageCompact(doc, {pageId, compact})`.
  - `D.on('state', update)` re-sets each control from `D.doc().look`/`grid`, skipping the focused control. `store.on('change')` updates the thumbs. On close: off, destroy the thumbs, `D.seal()`, resolve true.
  - **Update rule (dry-run fix).** `D.on('state', sync)` re-sets every control except a text-entry control that holds the focus (the control itself or its box). Containers (the family radiogroup, segmented groups) are always repainted: skipping them because the clicked card had focus left the old card checked.
  - Export `E.look = Object.freeze({open, model})`.
- [ ] The look tests are in `dialogs.test.mjs` (D2): the model (server shape, hue wrap, clamps, swatches, previews) and the sheet (one `setDoc` per field, custom block only in custom mode).
- [ ] Wiring step W for `hd-look.js` (with D2). Commit with D2.

### Task D2: hd-designer-dialogs.js (E.ddialogs)
**Files:** create `hd-designer-dialogs.js`; create `scripts/homedash-tests/dialogs.test.mjs` (one commit with D1). Wiring step W. **Existing symbols changed:** none.
- [ ] `hd-designer-dialogs.js`, in full (pure helpers, `run`, flows, `pageMenu`, `enhanceTabs`):
<!-- embed: dryrun-m4/ui/js/homedash/hd-designer-dialogs.js -->
```js
// HomeDash designer dialogs (M4 part 2, D8, D10; contract C20 E.ddialogs): run(ctx, action) for new / rename / import / export / look / add_page, the page tab
// menu and enhanceTabs (add button, icons, inline rename, Alt+Shift+Arrow and drag reorder on part 1's tab strip), export and import of dashboard files.
// Import is untrusted: the 512 KiB cap is checked before the file is read, the text is parsed and sanitised here and judged by POST dashboards/validate
// before anything is created or rendered. Names and issue paths reach the page only through textContent. Forms run on E.sheet (E.form.formSheet).
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.form || !E.look || !E.commands || !E.save || !E.doc) throw new Error('hd-designer-dialogs.js: load hd-form.js, hd-look.js, hd-commands.js and hd-save.js first');
    const { el, setAttr, setVars } = E.core, F = E.form, C = E.commands;
    const ACTIONS = Object.freeze(['new', 'rename', 'import', 'export', 'look', 'add_page']), IMPORT_MAX = 524288, NAME_MAX = 80, PAGE_NAME_MAX = 40, STRIP = Object.freeze(['id', 'revision', 'approved_revision', 'saved_by', 'updated_at']);
    const FOLD = new RegExp('[' + String.fromCodePoint(0x300) + '-' + String.fromCodePoint(0x36f) + ']', 'g'), isObj = v => v !== null && typeof v === 'object' && !Array.isArray(v);
    const slugify = name => String(name || '').normalize('NFD').replace(FOLD, '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 48).replace(/-+$/, '') || 'dashboard';
    // Export (D10): {schema_version, dashboard} of E.doc.forSave (drafts dropped, theme snapshot) without server fields; the kiosk block stays.
    function exportFile(doc, o) { const opts = o || {}, body = E.doc.forSave(doc, { catalog: opts.catalog, snapshot: opts.snapshot }), dashboard = {};
        for (const k of Object.keys(body)) if (!STRIP.includes(k)) dashboard[k] = E.core.clone(body[k]);
        const slug = typeof dashboard.slug === 'string' && /^[a-z0-9][a-z0-9-]{0,63}$/.test(dashboard.slug) ? dashboard.slug : slugify(dashboard.name);
        return Object.freeze({ filename: 'homedash-' + slug + '.json', text: JSON.stringify({ schema_version: Number(dashboard.schema_version) || 1, dashboard }, null, 2) + '\n', drafts: opts.catalog ? E.doc.drafts(doc, opts.catalog).size : 0 }); }
    // Import (D10): untrusted text; size cap again after reading; envelope {schema_version, dashboard} or a bare document with pages; id and slug
    // cleared, server fields removed; everything else is judged by POST dashboards/validate before anything is created or rendered.
    function importDocument(text) {
        if (typeof text !== 'string' || !text) return { ok: false, code: 'bad_json' }; if (E.save.utf8Bytes(text) > IMPORT_MAX) return { ok: false, code: 'too_large' };
        let data; try { data = JSON.parse(text.charCodeAt(0) === 0xfeff ? text.slice(1) : text); } catch (err) { return { ok: false, code: 'bad_json' }; }
        const d = isObj(data) && isObj(data.dashboard) && !Array.isArray(data.pages) ? data.dashboard : isObj(data) && Array.isArray(data.pages) ? data : null;
        if (!d || !Array.isArray(d.pages)) return { ok: false, code: 'bad_json' };
        const doc = E.core.clone(d); for (const k of STRIP) delete doc[k]; doc.id = ''; doc.slug = '';
        if (d !== data && Number.isInteger(data.schema_version) && doc.schema_version === undefined) doc.schema_version = data.schema_version;
        return { ok: true, document: doc };
    }
    const nameError = (raw, max) => { const n = Array.from(String(raw == null ? '' : raw).trim()).length; return n === 0 ? 'empty' : n > max ? 'long' : ''; };
    const newChoices = o => (o && o.readonly ? [] : [{ kind: 'starter', icon: 'gauge', primary: true }, { kind: 'empty', icon: 'dashboard' }].concat(o && Array.isArray(o.templates) && o.templates.includes('home_assistant') ? [{ kind: 'ha', icon: 'home' }] : []));
    // Drop index of a dragged tab among the others (C5 movePage target): the number of other tabs whose midpoint lies left of x.
    const targetIndex = (rects, x, from) => rects.reduce((n, r, i) => (i !== from && x > r.left + r.width / 2 ? n + 1 : n), 0);
    const copyName = (t, name) => { const sfx = t('homedash.conflict.copy_suffix'); return Array.from(String(name || '')).slice(0, Math.max(1, PAGE_NAME_MAX - Array.from(sfx).length)).join('') + sfx; };

    // ---- sheets: the designer's own, else one per visible host (the engine root is hidden in home mode, so the host element carries the sheet) ----
    const hostSheets = new WeakMap();
    function sheetOf(ctx) {
        if (ctx.designer) return ctx.designer.sheet();
        const st = ctx.state(), hostEl = st.mode !== 'dashboard' && typeof ctx.sheetHost === 'function' ? ctx.sheetHost() : null;
        if (hostEl) { if (!hostSheets.has(hostEl)) hostSheets.set(hostEl, E.sheet.create(hostEl)); return hostSheets.get(hostEl); }
        return ctx.sheet;
    }
    const closeAction = ctx => ({ id: 'close', label: ctx.t('homedash.dialog.close'), tone: 'primary' });
    const notice = (ctx, title, text) => sheetOf(ctx).open({ title, text: [text], actions: [Object.assign(closeAction(ctx), { autofocus: true })], kind: 'confirm' });
    function issuesSheet(ctx, issues, imported) {
        const t = ctx.t, list = el('ul', { class: 'hd-dlg-issues' }), shown = issues.slice(0, 8);
        for (const i of shown) list.appendChild(el('li', {}, [el('span', { text: E.save.errorText(t, { code: i && i.code, issues: [i] }) }), el('code', { text: Array.from(String((i && i.path) || '')).slice(0, 120).join('') })]));
        const parts = imported ? [list] : [el('p', { class: 'hd-sheet-text', text: t('homedash.dialog.import_issues_text', { count: issues.length }) }), list];
        if (issues.length > shown.length) parts.push(el('p', { class: 'hd-f-help', text: t('homedash.dialog.import_more', { count: issues.length - shown.length }) }));
        return F.formSheet(sheetOf(ctx), { title: imported ? t('homedash.dialog.import_unknown', { count: issues.length }) : t('homedash.dialog.import_issues_title'), content: el('div', { class: 'hd-dlg' }, parts), actions: [closeAction(ctx)] });
    }
    const tooLarge = ctx => notice(ctx, ctx.t('homedash.dialog.import_title'), ctx.t('homedash.dialog.import_too_large'));

    // ---- run ----
    async function run(ctx, action, arg) {
        if (!ACTIONS.includes(action)) return false; const st = ctx.state(), ro = !!st.desktopReadonly;
        if (action === 'new') return ro ? false : newSheet(ctx);
        if (action === 'import') return ro ? false : importFlow(ctx);          // Q12: with or without an open dashboard
        if (st.mode !== 'dashboard') return false;
        if (action === 'export') return exportFlow(ctx);
        if (ro) return false;
        if (!ctx.designer) { const ok = await ctx.mount.enterEdit(); return ok ? ctx.mount.run(action, arg) : false; }   // reading 8
        if (action === 'rename') return renameSheet(ctx);
        if (action === 'look') return E.look.open(ctx);
        return addPage(ctx.designer);
    }
    async function newSheet(ctx) {
        const t = ctx.t, st = ctx.state(); if (st.mode !== 'dashboard') { ctx.home(); return true; }                 // reading 9: the home screen is the chooser
        const TEXT = { starter: () => [t('homedash.home.create_starter'), t('homedash.home.create_starter_hint')], empty: () => [t('homedash.home.create_empty'), t('homedash.home.create_empty_hint')], ha: () => [t('homedash.home.create_ha'), t('homedash.home.create_ha_hint')] };
        const content = el('div', { class: 'hd-dlg hd-newcards' });
        for (const c of newChoices({ readonly: st.desktopReadonly, templates: st.templates })) { const [title, hint] = TEXT[c.kind]();
            content.appendChild(el('button', { type: 'button', class: 'hd-newcard', 'data-kind': c.kind }, [el('span', { class: 'hd-newcard-icon', 'aria-hidden': 'true' }, E.icons.icon(c.icon)), el('span', {}, [el('strong', { text: title }), el('small', { text: hint })])])); }
        const imp = F.button({ label: t('homedash.dialog.new_import'), icon: 'upload', tone: 'quiet' }); setAttr(imp.el, 'data-kind', 'import'); content.appendChild(imp.el);
        const sheet = sheetOf(ctx), first = content.querySelector('.hd-newcard');
        content.addEventListener('click', ev => { const b = ev.target && ev.target.closest ? ev.target.closest('[data-kind]') : null; if (b) sheet.close(b.getAttribute('data-kind') === 'import' ? 'import' : 'new:' + b.getAttribute('data-kind')); });
        const picked = await F.formSheet(sheet, { title: t('homedash.dialog.new_title'), content, actions: [{ id: 'cancel', label: t('homedash.dialog.cancel') }], first });
        if (picked === 'import') return importFlow(ctx);
        if (typeof picked !== 'string' || !picked.startsWith('new:')) return false;
        const kind = picked.slice(4);
        if (ctx.designer) { const left = await ctx.mount.exitEdit(); if (left !== 'left') return false; }
        const ok = await ctx.mount.run('new_' + kind); if (ok && kind === 'empty') await ctx.mount.enterEdit(); return ok;
    }
    // ---- export ----
    async function exportFlow(ctx) {
        const st = ctx.state(), dash = st.dashboard; if (!dash) return false; const catalog = ctx.catalog, t = ctx.t;
        let raw = ctx.designer ? ctx.designer.doc() : null;
        if (!raw) { const cached = ctx.store.docs.get(dash.id); raw = cached ? cached.doc : null; if (!raw) { try { const res = await ctx.api.dashboard(dash.id); raw = res && res.dashboard; } catch (err) { return false; } } }
        const doc = E.doc.normalise(raw, { sizeOf: w => catalog.defaultSize(w), constraints: w => catalog.constraints(w) });
        const file = exportFile(doc, { catalog, snapshot: E.widgets.paletteSnapshot(ctx.root, catalog.paletteTokens()) });
        const win = ctx.root.ownerDocument.defaultView, blob = new win.Blob([file.text], { type: 'application/json' }), href = win.URL.createObjectURL(blob);
        const a = el('a', { href, download: file.filename, hidden: true }); ctx.root.appendChild(a); a.click(); a.remove(); win.setTimeout(() => win.URL.revokeObjectURL(href), 1000);
        ctx.toast(t('homedash.dialog.export_done', { file: file.filename }) + (file.drafts ? ' ' + t('homedash.dialog.export_drafts', { count: file.drafts }) : '')); return true;
    }
    // ---- import ----
    function pickFile(ctx) {
        const st = ctx.state(), hostEl = st.mode !== 'dashboard' && typeof ctx.sheetHost === 'function' ? ctx.sheetHost() : ctx.root, input = el('input', { type: 'file', accept: '.json,application/json', hidden: true, 'aria-hidden': 'true', tabindex: '-1', 'data-hd-import': 'true' });
        hostEl.appendChild(input);
        return new Promise(resolve => { input.addEventListener('change', () => { const f = input.files && input.files[0] ? input.files[0] : null; input.remove(); resolve(f); }, { once: true }); input.addEventListener('cancel', () => { input.remove(); resolve(null); }, { once: true }); input.click(); });
    }
    async function importFlow(ctx) {
        const t = ctx.t, file = await pickFile(ctx); if (!file) return false;
        if (file.size > IMPORT_MAX) { await tooLarge(ctx); return false; }
        let text; try { text = await file.text(); } catch (err) { await notice(ctx, t('homedash.dialog.import_title'), t('homedash.dialog.import_failed', { reason: E.save.errorText(t, { code: 'bad_json' }) })); return false; }
        const parsed = importDocument(text); if (!parsed.ok) { if (parsed.code === 'too_large') await tooLarge(ctx); else await issuesSheet(ctx, [{ code: parsed.code }]); return false; }
        const failed = err => notice(ctx, t('homedash.dialog.import_title'), t('homedash.dialog.import_failed', { reason: E.save.errorText(t, { code: err && err.code }) }));
        // A bare document may lack the kiosk block the validator requires: normalise (server defaults) before validate and create.
        const document = E.doc.normalise(parsed.document, { sizeOf: w => ctx.catalog.defaultSize(w), constraints: w => ctx.catalog.constraints(w) });
        let res; try { res = await ctx.api.validateDocument(document); }
        catch (err) { const issues = err && err.data && Array.isArray(err.data.issues) ? err.data.issues : null; if (issues && issues.length) await issuesSheet(ctx, issues); else await failed(err); return false; }
        if (!res || res.valid !== true) { await issuesSheet(ctx, res && Array.isArray(res.issues) && res.issues.length ? res.issues : [{ code: 'invalid_document' }]); return false; }
        let made; try { made = await ctx.api.createDocument(document); } catch (err) { await failed(err); return false; }
        const d = made && made.dashboard; if (!d) { await failed(null); return false; }
        const unknownList = (Array.isArray(made.warnings) ? made.warnings : Array.isArray(res.warnings) ? res.warnings : []).filter(w => w && w.code === 'entity_unknown'), unknown = unknownList.length;
        ctx.refresh(); if (ctx.designer) { const left = await ctx.mount.exitEdit(); if (left !== 'left') { ctx.toast(t('homedash.dialog.import_done', { name: d.name || '' })); return true; } }
        await ctx.open(d.id, null); ctx.toast(t('homedash.dialog.import_done', { name: d.name || '' }) + (unknown ? ' ' + t('homedash.dialog.import_unknown', { count: unknown }) : ''));
        if (unknown) await issuesSheet(ctx, unknownList, true);    // D10: the unknown devices are listed after the import (at most 8 rows plus import_more)
        return true;
    }
    // ---- rename ----
    async function renameSheet(ctx) {
        const D = ctx.designer, t = ctx.t; let field = null;
        const ctl = F.text({ key: 'dashboard-name', max: NAME_MAX, label: t('homedash.dialog.name'), value: D.doc().name || '', select: true, onEnter: () => submit() });
        field = F.field({ label: t('homedash.dialog.name'), control: ctl });
        const sheet = D.sheet();
        function check() { const v = ctl.get(), bad = nameError(v, NAME_MAX) || F.textIssue(v);
            if (!bad) { field.error(''); return true; } field.error(bad === 'empty' ? t('homedash.dialog.name_empty') : bad === 'long' ? t('homedash.dialog.name_long', { max: NAME_MAX }) : t('homedash.error.' + bad)); return false; }
        function submit() { if (check()) sheet.close('save'); }
        const picked = await F.formSheet(sheet, { title: t('homedash.dialog.rename_title'), content: el('div', { class: 'hd-dlg' }, field.el), actions: [{ id: 'cancel', label: t('homedash.dialog.cancel') }, { id: 'save', label: t('homedash.dialog.save'), tone: 'primary' }], onAction: id => (id === 'save' ? check() : true), first: ctl });
        if (picked !== 'save') return false; D.exec(C.setDoc(D.doc(), { path: ['name'], value: ctl.get().trim(), label: 'homedash.cmd.rename' })); D.seal(); return true;
    }
    const tabsOf = new WeakMap(), lastOf = n => { const k = n.children; return k.length ? k[k.length - 1] : null; };
    function addPage(D) {
        const doc = D.doc(), cmd = C.addPage(doc, { name: D.t('homedash.designer.page_n', { n: doc.pages.length + 1 }), icon: 'dashboard' });
        const ok = D.exec(cmd), h = tabsOf.get(D); if (ok && h && cmd && cmd.page) h.rename(cmd.page); return !!ok;   // a refusal (12 pages) is announced by exec
    }
    // ---- page tab menu ----
    function iconSheet(D, pageId) {
        const t = D.t, page = D.doc().pages.find(p => p.id === pageId); if (!page) return Promise.resolve(false);
        const grid = F.iconGrid({ t, label: t('homedash.inspector.page_icon'), value: page.icon || 'dashboard', allowAuto: false, onPick: n => D.exec(C.setPageIcon(D.doc(), { pageId, icon: n || undefined })) });
        return F.formSheet(D.sheet(), { title: t('homedash.inspector.page_icon'), content: el('div', { class: 'hd-dlg' }, grid.el), actions: [{ id: 'done', label: t('homedash.designer.done'), tone: 'primary' }], first: grid }).then(() => { D.seal(); return true; });
    }
    async function deletePage(D, pageId) {
        const t = D.t, page = D.doc().pages.find(p => p.id === pageId); if (!page) return false;
        if (page.widgets.length) { const yes = await D.sheet().confirm({ title: t('homedash.dialog.page_delete_title', { name: page.name }), text: [t('homedash.dialog.page_delete_text', { count: page.widgets.length })], confirmLabel: t('homedash.cmd.page_delete'), cancelLabel: t('homedash.dialog.cancel'), tone: 'danger' }); if (!yes) return false; }
        return D.exec(C.deletePage(D.doc(), { pageId }));
    }
    function movePageBy(D, pageId, to) { const ok = D.exec(C.movePage(D.doc(), { pageId, to })); const h = tabsOf.get(D); if (ok && h) h.focusTab(pageId); return ok; }
    function pageMenu(D, pageId) {
        const t = D.t, doc = D.doc(), i = doc.pages.findIndex(p => p.id === pageId), n = doc.pages.length, page = doc.pages[i]; if (!page) return [];
        const h = () => tabsOf.get(D);
        return [{ id: 'page_rename', label: t('homedash.menu.page_rename'), icon: 'rename', shortcut: 'F2', run: () => { if (h()) h().rename(pageId); } },
            { id: 'page_icon', label: t('homedash.menu.page_icon'), icon: 'icon', run: () => iconSheet(D, pageId) },
            { id: 'page_duplicate', label: t('homedash.ctx.duplicate'), icon: 'duplicate', run: () => D.exec(C.duplicatePage(D.doc(), { pageId, name: copyName(t, page.name) })) },
            { separator: true },
            { id: 'page_left', label: t('homedash.menu.page_left'), icon: 'left', shortcut: 'Alt+Shift+Left', disabled: i === 0, run: () => movePageBy(D, pageId, i - 1) },
            { id: 'page_right', label: t('homedash.menu.page_right'), icon: 'right', shortcut: 'Alt+Shift+Right', disabled: i === n - 1, run: () => movePageBy(D, pageId, i + 1) },
            { separator: true },
            { id: 'page_delete', label: t('homedash.menu.page_delete'), icon: 'delete', danger: true, disabled: n <= 1, run: () => deletePage(D, pageId) }];
    }
    // ---- tab strip upgrade (part 1 builds the select-only tabs; this decorates the same nodes) ----
    function enhanceTabs(D, slot) {
        if (tabsOf.has(D)) return tabsOf.get(D);
        const t = D.t, life = E.core.bag(); let add = null, input = null, caret = null, drag = null;
        const pageOf = tab => (tab ? tab.getAttribute('data-page') : null), tabsEl = () => Array.from(slot.querySelectorAll('.hd-ptab')), tabFor = id => tabsEl().find(x => pageOf(x) === id) || null;
        function decorate() {
            const pages = D.doc().pages;
            if (!add) { add = el('button', { type: 'button', class: 'hd-ptab-add', 'aria-label': t('homedash.cmd.page_add'), title: t('homedash.cmd.page_add') }, E.icons.icon('plus')); slot.appendChild(add); }
            else if (lastOf(slot) !== add && !input) slot.appendChild(add);
            setAttr(add, 'aria-disabled', pages.length >= C.LIMITS.pages ? 'true' : null);
            for (const tab of tabsEl()) { const page = pages.find(p => p.id === pageOf(tab)); if (!page) continue; const want = E.icons.has(page.icon) ? page.icon : 'dashboard', have = tab.querySelector('.hd-ptab-icon');
                if (have && have.getAttribute('data-hd-icon') === want) continue; const ico = E.icons.icon(want); setAttr(ico, 'class', 'hd-ico hd-ptab-icon'); if (have) have.remove(); tab.insertBefore(ico, tab.children[0] || null); }
        }
        const schedule = E.core.frame(decorate), off = D.on('state', schedule); decorate();
        const focusTab = id => { const tab = tabFor(id); if (tab) tab.focus(); };
        function rename(pageId, retry) {
            const tab = tabFor(pageId), page = D.doc().pages.find(p => p.id === pageId);
            if (!tab || !page) { if (!retry && page) E.core.frame(() => rename(pageId, true))(); return false; } if (input) cancelRename(false);
            const sr = slot.getBoundingClientRect(), tr = tab.getBoundingClientRect(); let closed = false;
            input = el('input', { class: 'hd-ptab-input', type: 'text', spellcheck: 'false', autocomplete: 'off', 'aria-label': t('homedash.inspector.page_name') }); input.value = page.name;
            setVars(input, { '--hd-e-ix': (tr.left - sr.left) + 'px', '--hd-e-iw': Math.max(96, tr.width) + 'px' }); setAttr(tab, 'data-hd-renaming', 'true'); slot.appendChild(input); input.focus(); if (typeof input.select === 'function') input.select();
            const finish = restore => { if (closed) return; closed = true; const node = input; input = null; node.remove(); const tb = tabFor(pageId); if (tb) tb.removeAttribute('data-hd-renaming'); focusTab(pageId); E.core.frame(() => focusTab(pageId))(); if (add && lastOf(slot) !== add) slot.appendChild(add); return restore; };
            const commit = byEnter => { if (closed) return; const v = F.runes(input.value, PAGE_NAME_MAX), bad = F.textIssue(v);
                if (!v.trim()) { finish(); return; }
                if (bad) { if (byEnter) { setAttr(input, 'aria-invalid', 'true'); D.announce(t('homedash.error.' + bad)); return; } finish(); return; }
                D.exec(C.renamePage(D.doc(), { pageId, name: v })); D.seal(); finish(); };
            input.addEventListener('keydown', ev => { if (ev.isComposing) return; if (ev.key === 'Enter') { ev.preventDefault(); ev.stopPropagation(); commit(true); } else if (ev.key === 'Escape') { ev.preventDefault(); ev.stopPropagation(); finish(); } });
            input.addEventListener('input', () => { setAttr(input, 'aria-invalid', null); }); input.addEventListener('blur', () => commit(false)); return true;
        }
        function cancelRename() { if (!input) return; const node = input; input = null; node.remove(); }
        life.on(slot, 'keydown', ev => {
            if (ev.target === input) return; const tab = ev.target && ev.target.closest ? ev.target.closest('.hd-ptab') : null, pageId = pageOf(tab) || D.state().pageId, pages = D.doc().pages, i = pages.findIndex(p => p.id === pageId);
            if (ev.altKey && ev.shiftKey && (ev.key === 'ArrowLeft' || ev.key === 'ArrowRight') && i >= 0) { ev.preventDefault(); ev.stopPropagation(); const to = i + (ev.key === 'ArrowLeft' ? -1 : 1); if (to >= 0 && to < pages.length) movePageBy(D, pageId, to); return; }
            if (tab && (ev.key === 'F2' || (ev.key === 'Enter' && tab.getAttribute('aria-selected') === 'true'))) { ev.preventDefault(); ev.stopPropagation(); rename(pageId); }
        }, { capture: true });
        life.on(slot, 'click', ev => { const b = ev.target && ev.target.closest ? ev.target.closest('.hd-ptab-add') : null; if (b && b.getAttribute('aria-disabled') !== 'true') addPage(D); });
        life.on(slot, 'dblclick', ev => { const tab = ev.target && ev.target.closest ? ev.target.closest('.hd-ptab') : null; if (tab) rename(pageOf(tab)); });
        life.on(slot, 'pointerdown', ev => {
            const tab = ev.target && ev.target.closest ? ev.target.closest('.hd-ptab') : null; if (!tab || input || (ev.button !== undefined && ev.button !== 0)) return;
            const from = tabsEl().indexOf(tab), pageId = pageOf(tab), rects = () => tabsEl().map(x => x.getBoundingClientRect());
            E.input.track(ev, { threshold: 5, axis: 'x', capture: tab,
                onStart: () => { drag = { rects: rects(), from }; setAttr(tab, 'data-hd-dragging', 'true'); caret = el('span', { class: 'hd-ptab-caret', 'aria-hidden': 'true' }); slot.appendChild(caret); },
                onMove: (p, start) => { if (!drag) return; setVars(tab, { '--hd-e-tx': (p.x - start.x) + 'px' }); const idx = targetIndex(drag.rects, p.x, drag.from), others = drag.rects.filter((r, k) => k !== drag.from), sr = slot.getBoundingClientRect();
                    const at = idx < others.length ? others[idx].left : (others.length ? others[others.length - 1].left + others[others.length - 1].width : 0); setVars(caret, { '--hd-e-cx': (at - sr.left) + 'px' }); },
                onEnd: (p, start, how) => { const d = drag; drag = null; tab.removeAttribute('data-hd-dragging'); setVars(tab, { '--hd-e-tx': null }); if (caret) { caret.remove(); caret = null; }
                    if (d && how === 'up') { const to = targetIndex(d.rects, p.x, d.from); if (to !== d.from) movePageBy(D, pageId, to); } } });
        });
        const api = Object.freeze({ rename, focusTab, decorate, destroy() { off(); schedule.cancel(); life.dispose(); if (add) add.remove(); if (input) input.remove(); tabsOf.delete(D); } });
        tabsOf.set(D, api); return api;
    }
    E.ddialogs = Object.freeze({ ACTIONS, run, pageMenu, enhanceTabs, exportFile, importDocument, newChoices, nameError, targetIndex, slugify, IMPORT_MAX });
})();
```
- [ ] `run(ctx, action, arg)` order (cross-check C2): `new` (not read-only) → `import` (not read-only, with or without a dashboard) → the dashboard test → `export` → read-only guard → view mode enters edit first (`ctx.mount.enterEdit()` then `ctx.mount.run(action)` again) → `rename` / `look` / `add_page`.
- [ ] Flows (the file is authoritative):
  - `sheetOf(ctx)`: the designer's own sheet when designing; else, while `state().mode !== 'dashboard'` and `ctx.sheetHost` exists, one `E.sheet` per host element (WeakMap) on `ctx.sheetHost()`; else `ctx.sheet`.
  - `newSheet`: not on a dashboard → `ctx.home()`; otherwise a form sheet `dialog.new_title` with one `button.hd-newcard[data-kind]` per `newChoices` (starter, empty, HA when listed; literal home keys), a quiet `dialog.new_import` button and Cancel. Then `exitEdit()` when editing (abort unless `'left'`) → `ctx.mount.run('new_starter'|'new_empty'|'new_ha')`; empty then `enterEdit()`.
  - `exportFlow`: the document is `D.doc()` while designing, else `ctx.store.docs.get(ctx.state().dashboard.id).doc` (fallback `api.dashboard(id)`); `E.doc.normalise(raw, {sizeOf: w => catalog.defaultSize(w), constraints: w => catalog.constraints(w)})` → `exportFile` with the theme snapshot → Blob download, toast `export_done` (+ `export_drafts`).
  - `importFlow`: hidden `input[type=file]` (appended to the visible host); `file.size > 524288` → notice **before reading**; `importDocument` → `E.doc.normalise` (kiosk default) → `POST dashboards/validate` (a throw with `data.issues`, a `valid !== true` answer → issues sheet of at most 8 rows + `import_more`; another error → notice `import_failed`) → `POST dashboards` → `ctx.open(id)`, toast `import_done` (+ `import_unknown`), then the unknown devices (`entity_unknown` warnings) in the same sheet (I7). While designing, the editor is left first (a refusal keeps it and only toasts).
  - `renameSheet`: form sheet with `nameError(v, 80) || textIssue(v)`; a text issue shows `homedash.error.<code>` and the sheet stays open; Enter submits; save → `D.exec(C.setDoc(…['name']…, label 'homedash.cmd.rename'))` + `seal`.
  - `addPage(D)`: `C.addPage` + `D.exec`, then the inline rename of the new tab through the enhancer.
- [ ] `pageMenu(D, pageId)` (engine items, C3 shape; `{separator:true}`): Rename (F2), Icon… (form sheet with the translated icon grid), Duplicate (`homedash.ctx.duplicate`), Move left/right (disabled at the ends), Delete page… (danger, disabled with one page, confirm sheet when the page has widgets).
- [ ] `enhanceTabs(D, slot)` (amendment 1, called by `hd-designer.js`). Idempotent per `D` (WeakMap); upgrades part 1's nodes without replacing them:
  - decorates on `D.on('state')` + `E.core.frame`: `button.hd-ptab-add` stays the slot's last element child, a sibling of `.hd-ptabs`; each `.hd-ptab` gets an `hd-ico.hd-ptab-icon` (its page icon or `dashboard`) before its label;
  - inline rename: `input.hd-ptab-input[aria-label=inspector.page_name]` with `--hd-e-ix/-iw`; Enter or blur commits through `C.renamePage` (1–40 runes; empty cancels) + `D.seal()`; Escape cancels (`stopPropagation`); **text the server refuses (addendum):** Enter keeps the input open, sets `aria-invalid="true"` and calls `D.announce(t('homedash.error.' + code))`, blur cancels and the old name stays; the tab is focused again afterwards;
  - keys (capture listener on the slot): Alt+Shift+Left/Right → `C.movePage` ±1; F2, or Enter on the selected tab → rename; `dblclick` → rename;
  - drag reorder: `pointerdown` on a tab → `E.input.track(ev, {threshold:5, axis:'x', capture: tab})`; start sets `data-hd-dragging`, moves write `--hd-e-tx` and place `span.hd-ptab-caret` (`--hd-e-cx`) at `targetIndex`; up → `C.movePage(doc, {pageId, to})`; cancel resets.
- [ ] Export `E.ddialogs = Object.freeze({ACTIONS, run, pageMenu, enhanceTabs, exportFile, importDocument, newChoices, nameError, targetIndex, slugify, IMPORT_MAX})`.
- [ ] `dialogs.test.mjs` (14 tests: the look model and previews, the export and import models, names/text rules/choices/drop index, export from view and design mode, import from the home screen on the host sheet, import refusals (600 KiB never read, broken file, server issues, read-only), the New sheet, rename, the look sheet, the tab upgrade incl. drag, the page menu):
<!-- embed: dryrun-m4/scripts/homedash-tests/dialogs.test.mjs -->
```js
import assert from 'node:assert/strict'; import { eq, fc, noStyleAttr } from './harness.mjs'; import { boot, fire, flush, fakeD, fakeStore, fakeApi, docOf, FakeElement } from './designer-kit.mjs'; import { createDocument } from './dom.mjs';
// `click()` of the fake DOM: a file input hands itself to globalThis.__pick (the "user" chooses a file); every other element counts the clicks.
FakeElement.prototype.click = function () { if (this.getAttribute('type') === 'file' && globalThis.__pick) globalThis.__pick(this); else this._clicks = (this._clicks || 0) + 1; };
const tr = (k, v) => (v ? k + JSON.stringify(v) : k), WIDGETS = [{ id: 'g1', type: 'gauge', variant: 'arc', bindings: { primary: 'system:cpu' }, options: {} }, { id: 's1', type: 'switch', variant: 'pill', bindings: {}, options: {} }];
const LAYOUT = [{ id: 'g1', x: 0, y: 0, w: 2, h: 2 }, { id: 's1', x: 2, y: 0, w: 2, h: 1 }];
const sheetBtn = (sheet, action) => sheet.el.querySelectorAll('.hd-sheet-btn, .hd-dlg-btn').find(b => b.getAttribute('data-action') === action);
const press = (sheet, action) => { const b = sheetBtn(sheet, action); assert.ok(b, 'no button ' + action); fire(b.getAttribute('class').includes('hd-dlg-btn') ? sheet.el.querySelector('.hd-sheet-foot') : sheet.el, 'click', {}, b); };
// A dialogs context over the fake DOM: a root with a real E.sheet, a window stub (Blob/URL), recorded mount calls and a fake API.
function setup(o) {
  const k = boot('hd-designer-dialogs.js'), x = o || {}, doc = docOf(k.E, k.cat, WIDGETS, LAYOUT, x.docExtra), root = k.document.createElement('div'), hostEl = k.document.createElement('div'); k.document.body.append(root, hostEl);
  const blobs = [], win = { Blob: class { constructor(p, op) { this.parts = p; this.type = op.type; } }, URL: { createObjectURL: b => { blobs.push(b); return 'blob:x'; }, revokeObjectURL() {} }, setTimeout: () => 0 }; k.document.defaultView = win;
  const sheet = k.E.sheet.create(root), log = [], api = Object.assign({ validated: [], created: [], async validateDocument(d) { this.validated.push(d); return { valid: true, issues: [], warnings: [{ code: 'entity_unknown', path: '/pages/0/widgets/0' }], document: d }; },
    async createDocument(d) { this.created.push(d); return { dashboard: { id: 'd_aaaaaaaaaaab', name: d.name || 'N' }, warnings: [{ code: 'entity_unknown', path: '/pages/0/widgets/0' }] }; } }, x.api || {});
  const state = x.state || { mode: 'dashboard', dashboard: { id: 'd_aaaaaaaaaaaa', name: 'Test' }, desktopReadonly: false, templates: [] };
  const store = fakeStore(); store.docs.get = id => (id === 'd_aaaaaaaaaaaa' ? { doc, revision: 3 } : null);
  const D = x.designer ? fakeD(k.E, k.cat, doc, { selection: [], sheet, t: tr }) : null;
  const mount = { async enterEdit() { log.push('enterEdit'); return true; }, async exitEdit() { log.push('exitEdit'); return 'left'; }, async run(a, arg) { log.push('run:' + a); return true; } };
  const ctx = { root, t: tr, lang: 'en', api, store, catalog: k.cat, designer: D, mount, state: () => state, async open(id, p) { log.push('open:' + id); }, home() { log.push('home'); }, sheet, sheetHost: x.hostSheet ? () => hostEl : undefined, toast: s => log.push('toast:' + s), refresh: () => log.push('refresh') };
  return Object.assign(k, { ctx, D, doc, log, api, sheet, blobs, hostEl, root, win });
}
const withFile = (file, fn) => { globalThis.__pick = input => { input.files = [file]; fire(input, 'change', {}); }; return fn().finally(() => { delete globalThis.__pick; }); };
const fileOf = (text, size) => { const f = { size: size === undefined ? Buffer.byteLength(text) : size, reads: 0, async text() { this.reads += 1; return text; } }; return f; };
export const tests = [
  ['look model: palette follows the server shape, hue wraps, radius and row unit clamp, swatches are hex', () => { const { E } = boot('hd-look.js'), M = E.look.model;
    eq(M.paletteFor({ mode: 'theme' }, 'custom', { accent: '#ABCDEF', scheme: 'light', surface_hue: 12.6 }), { mode: 'custom', custom: { accent: '#abcdef', surface_hue: 13, scheme: 'light' } });
    const back = M.paletteFor({ mode: 'custom', custom: { accent: '#112233', surface_hue: 5, scheme: 'dark' } }, 'theme'); assert.equal(back.mode, 'theme'); eq(back.custom, { accent: '#112233', surface_hue: 5, scheme: 'dark' }); assert.ok(!('custom' in M.paletteFor({ mode: 'theme' }, 'dark')));
    assert.equal(M.custom({ surface_hue: 360 }).surface_hue, 0); assert.equal(M.custom({ surface_hue: -5 }).surface_hue, 355); assert.equal(M.custom({ accent: 'nope' }, { accent: '#AA00BB' }).accent, '#aa00bb'); assert.equal(M.custom({ accent: 'nope', scheme: 'x' }).scheme, 'dark'); assert.equal(M.custom({}).accent, '#5b8def');
    assert.equal(M.radius(0.47), 0.5); assert.equal(M.radius(1.234), 1.25); assert.equal(M.radius(9), 1.5); assert.equal(M.rowUnit(30), 48); assert.equal(M.rowUnit('auto'), 'auto'); assert.equal(M.rowUnit(250), 200); assert.equal(M.rowUnit(''), 'auto');
    const sw = M.swatches({ accent: '#abcdef', scheme: 'dark', surface_hue: 200 }); eq(Object.keys(sw), ['canvas', 'surface', 'ink', 'accent', 'on']); for (const v of Object.values(sw)) assert.match(v, /^#[0-9a-fA-F]{6}$/); }],
  ['look model: previews take a real light/dial for slot a and fill b and c with samples', () => { const { E, cat } = boot('hd-look.js'), M = E.look.model, doc = docOf(E, cat, [{ id: 'l1', type: 'light', variant: 'dial', bindings: { primary: 'ha:light.x' } }], [{ id: 'l1', x: 0, y: 0, w: 2, h: 2 }]);
    const p = M.previewWidgets(doc.pages[0], cat, E.widgets); eq(p.map(x => [x.slot, x.widget.type, x.real]), [['a', 'light', true], ['b', 'switch', false], ['c', 'temperature', false]]); eq(p[0].size, { w: 2, h: 2 }); eq(p[1].size, { w: 2, h: 1 });
    eq(M.previewWidgets({ widgets: [], layouts: { lg: [] } }, cat, E.widgets).map(x => x.real), [false, false, false]); }],
  ['dialogs model: export drops server fields and drafts, keeps kiosk; the file name is the slug or the folded name', () => { const { E, cat } = boot('hd-designer-dialogs.js'), X = E.ddialogs;
    const doc = docOf(E, cat, WIDGETS, LAYOUT, { name: 'K' + fc(0xfc) + 'che ' + fc(0xdc) + 'bersicht', id: 'd_aaaaaaaaaaaa', revision: 7, approved_revision: 5, saved_by: 'agent', updated_at: 'x', slug: '' }), f = X.exportFile(doc, { catalog: cat, snapshot: { accent: '#112233' } }), data = JSON.parse(f.text);
    for (const k of ['id', 'revision', 'approved_revision', 'saved_by', 'updated_at']) assert.ok(!(k in data.dashboard), k); assert.ok(data.dashboard.kiosk); assert.equal(data.schema_version, 1); assert.equal(f.drafts, 1); eq(data.dashboard.pages[0].widgets.map(w => w.id), ['g1']); assert.ok(f.text.endsWith('\n'));
    assert.equal(f.filename, 'homedash-kuche-ubersicht.json'); assert.equal(X.exportFile(Object.assign({}, doc, { slug: 'my-home' }), { catalog: cat }).filename, 'homedash-my-home.json'); assert.equal(data.dashboard.look.palette.snapshot && data.dashboard.look.palette.snapshot.accent, '#112233'); assert.equal(X.slugify('???'), 'dashboard'); }],
  ['dialogs model: import accepts the envelope and a bare document, clears id and slug, refuses hostile files', () => { const { E, cat } = boot('hd-designer-dialogs.js'), X = E.ddialogs, doc = docOf(E, cat, WIDGETS, LAYOUT, { id: 'd_aaaaaaaaaaaa', slug: 'abc', revision: 3 });
    const exported = X.exportFile(doc, { catalog: cat }).text, back = X.importDocument(exported); assert.ok(back.ok); assert.equal(back.document.id, ''); assert.equal(back.document.slug, ''); eq(back.document.pages, JSON.parse(exported).dashboard.pages); assert.ok(!('revision' in back.document));
    const bare = X.importDocument(JSON.stringify({ pages: [], id: 'x', saved_by: 'u', name: 'n' })); assert.ok(bare.ok); assert.equal(bare.document.id, ''); assert.ok(!('saved_by' in bare.document)); assert.equal(X.importDocument(JSON.stringify({ schema_version: 1, dashboard: { pages: [] } })).document.schema_version, 1);
    for (const bad of ['[]', '{"pages":"x"}', '{"dashboard":[]}', 'nope', '', '{"dashboard":{"name":"x"}}', 'null', '"s"']) assert.equal(X.importDocument(bad).code, 'bad_json', bad);
    assert.equal(X.importDocument('x'.repeat(513 * 1024)).code, 'too_large'); assert.equal(X.importDocument(String.fromCharCode(0xfeff) + '{"pages":[]}').ok, true); assert.equal(X.IMPORT_MAX, 524288);
    const big = '{"pages":[],"n":"' + fc(0x1f600).repeat(140000) + '"}'; assert.ok(big.length < 524288 && Buffer.byteLength(big) > 524288); assert.equal(X.importDocument(big).code, 'too_large', 'the cap counts bytes, not characters'); }],
  ['dialogs model: names, text rules, new choices and tab drop index', () => { const { E } = boot('hd-designer-dialogs.js'), X = E.ddialogs;
    assert.equal(X.nameError('  ', 80), 'empty'); assert.equal(X.nameError('x'.repeat(81), 80), 'long'); assert.equal(X.nameError(fc(0x1f600).repeat(80), 80), ''); assert.equal(X.nameError('ok', 80) || E.form.textIssue('x://y'), 'url_not_allowed'); assert.equal(X.nameError('ok', 80) || E.form.textIssue('Kitchen'), '');
    eq(X.newChoices({ readonly: true, templates: ['home_assistant'] }), []); eq(X.newChoices({ templates: [] }).map(c => c.kind), ['starter', 'empty']); eq(X.newChoices({ templates: ['home_assistant'] }).map(c => c.kind), ['starter', 'empty', 'ha']);
    const r = [{ left: 0, width: 100 }, { left: 100, width: 100 }, { left: 200, width: 100 }]; assert.equal(X.targetIndex(r, 260, 0), 2); assert.equal(X.targetIndex(r, 10, 2), 0); assert.equal(X.targetIndex(r, 120, 0), 0); assert.equal(X.targetIndex(r, 160, 0), 1); eq([...X.ACTIONS], ['new', 'rename', 'import', 'export', 'look', 'add_page']); }],
  ['dialogs run: export works from view mode (cached document) and from design mode (drafts left out, announced)', async () => { const v = setup(); assert.equal(await v.E.ddialogs.run(v.ctx, 'export'), true); assert.equal(v.blobs.length, 1); const data = JSON.parse(v.blobs[0].parts[0]); assert.equal(data.dashboard.name, 'Test'); assert.ok(data.dashboard.kiosk); assert.ok(!('id' in data.dashboard));
    assert.ok(v.log.some(l => l.startsWith('toast:homedash.dialog.export_done')) && v.log.some(l => l.includes('export_drafts')), 'view mode exports the cached document without drafts'); noStyleAttr(v.root);
    const d = setup({ designer: true }); await d.E.ddialogs.run(d.ctx, 'export'); eq(JSON.parse(d.blobs[0].parts[0]).dashboard.pages[0].widgets.map(w => w.id), ['g1']); assert.equal(d.blobs[0].type, 'application/json');
    const home = setup({ state: { mode: 'home', desktopReadonly: false } }); assert.equal(await home.E.ddialogs.run(home.ctx, 'export'), false); }],
  ['dialogs run: import works from the home screen on the host sheet, validates the normalised document, creates, opens and lists unknown devices', async () => {
    const m = setup({ state: { mode: 'home', desktopReadonly: false, templates: [] }, hostSheet: true }), bare = JSON.stringify({ name: 'Bare', pages: [{ id: 'p', name: 'P', widgets: [], layouts: { lg: [] } }] }), file = fileOf(bare);
    const p = withFile(file, () => m.E.ddialogs.run(m.ctx, 'import')); await flush(10); assert.equal(file.reads, 1); assert.equal(m.api.validated.length, 1); assert.ok(m.api.validated[0].kiosk, 'a bare document gets the kiosk default before validate'); assert.equal(m.api.validated[0].id, ''); assert.equal(m.api.validated[0].slug, '');
    eq(m.api.created[0], m.api.validated[0]); assert.ok(m.log.includes('open:d_aaaaaaaaaaab')); assert.ok(m.log.some(l => l.startsWith('toast:homedash.dialog.import_done'))); assert.ok(m.hostEl.querySelector('.hd-sheet-layer'), 'the sheet sits on the visible host, not in the hidden engine root'); assert.equal(m.root.querySelector('.hd-sheet-layer'), m.sheet.el, 'the context sheet in the engine root is untouched');
    const sheet = m.hostEl.querySelector('.hd-sheet-layer'); assert.ok(sheet.querySelector('.hd-dlg-issues'), 'the unknown devices are listed'); assert.equal(sheet.querySelectorAll('.hd-dlg-issues li').length, 1); const foot = sheet.querySelector('.hd-sheet-foot'); fire(foot, 'click', {}, foot.querySelector('.hd-dlg-btn')); assert.equal(await p, true); assert.ok(!m.hostEl.querySelector('input'), 'the file input is removed'); }],
  ['dialogs run: import refuses a big file before reading, a broken file and server issues without creating anything', async () => {
    const m = setup(), big = fileOf('{"pages":[]}', 600 * 1024); let p = withFile(big, () => m.E.ddialogs.run(m.ctx, 'import')); await flush(10); assert.equal(big.reads, 0, 'file.text is never called'); assert.equal(m.sheet.el.querySelector('.hd-sheet-title').textContent, 'homedash.dialog.import_title'); press(m.sheet, 'close'); assert.equal(await p, false);
    const bad = fileOf('{"pages":"x"}'); p = withFile(bad, () => m.E.ddialogs.run(m.ctx, 'import')); await flush(10); assert.equal(m.api.validated.length, 0); assert.ok(m.sheet.el.querySelector('.hd-dlg-issues li span').textContent.includes('homedash.error.bad_json')); const foot = m.sheet.el.querySelector('.hd-sheet-foot'); fire(foot, 'click', {}, foot.querySelector('.hd-dlg-btn')); assert.equal(await p, false);
    const server = setup({ api: { async validateDocument() { const e = new Error('x'); e.code = 'invalid_document'; e.data = { issues: [{ code: 'too_long', path: '/name' + 'y'.repeat(300) }] }; throw e; } } });
    p = withFile(fileOf('{"pages":[]}'), () => server.E.ddialogs.run(server.ctx, 'import')); await flush(10); const li = server.sheet.el.querySelector('.hd-dlg-issues li'); assert.ok(li.querySelector('span').textContent.includes('homedash.error.too_long')); assert.equal(Array.from(li.querySelector('code').textContent).length, 120); assert.equal(server.api.created.length, 0);
    const f2 = server.sheet.el.querySelector('.hd-sheet-foot'); fire(f2, 'click', {}, f2.querySelector('.hd-dlg-btn')); await p; const ro = setup({ state: { mode: 'dashboard', dashboard: { id: 'd_aaaaaaaaaaaa' }, desktopReadonly: true } }); assert.equal(await ro.E.ddialogs.run(ro.ctx, 'import'), false); assert.equal(await ro.E.ddialogs.run(ro.ctx, 'new'), false); }],
  ['dialogs run: New offers the choices, leaves edit mode first and the home screen is the chooser without a dashboard', async () => {
    const home = setup({ state: { mode: 'home', desktopReadonly: false } }); assert.equal(await home.E.ddialogs.run(home.ctx, 'new'), true); eq(home.log, ['home']);
    const d = setup({ designer: true, state: { mode: 'dashboard', dashboard: { id: 'd_aaaaaaaaaaaa' }, desktopReadonly: false, templates: ['home_assistant'] } }); const p = d.E.ddialogs.run(d.ctx, 'new'); await flush(5); const cards = d.sheet.el.querySelectorAll('.hd-newcard'); eq(cards.map(c => c.getAttribute('data-kind')), ['starter', 'empty', 'ha']);
    assert.ok(d.sheet.el.querySelector('[data-kind="import"]')); fire(d.sheet.el.querySelector('.hd-newcards'), 'click', {}, cards[1]); assert.equal(await p, true); eq(d.log, ['exitEdit', 'run:new_empty', 'enterEdit']);
    const s = setup(); const p2 = s.E.ddialogs.run(s.ctx, 'new'); await flush(5); press(s.sheet, 'cancel'); assert.equal(await p2, false); eq(s.log, []); }],
  ['dialogs run: rename refuses text the server refuses and keeps the sheet open; view mode enters edit first', async () => {
    const view = setup(); assert.equal(await view.E.ddialogs.run(view.ctx, 'rename'), true); eq(view.log, ['enterEdit', 'run:rename']); assert.equal(await view.E.ddialogs.run(view.ctx, 'look'), true); assert.equal(await view.E.ddialogs.run(view.ctx, 'nope'), false);
    const m = setup({ designer: true }), p = m.E.ddialogs.run(m.ctx, 'rename'), input = m.sheet.el.querySelector('[data-hd-field="dashboard-name"]'); assert.equal(input.value, 'Test'); assert.equal(m.D.execs.length, 0);
    input.value = 'x://y'; press(m.sheet, 'save'); assert.ok(m.sheet.isOpen()); assert.equal(input.getAttribute('aria-invalid'), 'true'); assert.equal(m.sheet.el.querySelector('.hd-f-error').textContent, 'homedash.error.url_not_allowed');
    input.value = '   '; press(m.sheet, 'save'); assert.ok(m.sheet.isOpen()); assert.equal(m.sheet.el.querySelector('.hd-f-error').textContent, 'homedash.dialog.name_empty'); input.value = 'Living room '; fire(input, 'keydown', { key: 'Enter' }); assert.equal(await p, true);
    assert.equal(m.D.doc().name, 'Living room'); assert.equal(m.D.execs[0].label, 'homedash.cmd.rename'); assert.ok(m.D.calls.includes('seal')); const c = setup({ designer: true }); const p3 = c.E.ddialogs.run(c.ctx, 'rename'); press(c.sheet, 'cancel'); assert.equal(await p3, false); assert.equal(c.D.execs.length, 0); }],
  ['dialogs: the look editor writes one setDoc per field, clamps like the server and shows custom colours only in custom mode', async () => {
    const m = setup({ designer: true }); m.ctx.root.setAttribute('data-hd-scheme', 'light'); const p = m.E.look.open(m.ctx), q = s => m.sheet.el.querySelector(s), custom = q('.hd-look-custom'); assert.equal(m.sheet.el.querySelector('.hd-sheet').getAttribute('data-kind'), 'form');
    assert.equal(m.sheet.el.querySelectorAll('.hd-look-family').length, 4); assert.equal(m.sheet.el.querySelectorAll('.hd-look-slot').length, 12); assert.ok(custom.hasAttribute('hidden')); assert.equal(q('[data-family="glass"]').getAttribute('aria-checked'), 'true');
    fire(q('.hd-look-families'), 'click', {}, q('[data-family="tactile"]')); eq(m.D.execs[0].ops[0].path, ['look', 'family']); assert.equal(m.D.doc().look.family, 'tactile'); assert.equal(q('[data-family="tactile"]').getAttribute('aria-checked'), 'true');
    const seg = (key, v) => fire(q('[data-hd-field="' + key + '"]'), 'click', {}, q('[data-hd-field="' + key + '"] [data-value="' + v + '"]')); seg('palette-mode', 'custom'); eq(m.D.doc().look.palette.custom, { accent: '#5b8def', surface_hue: 250, scheme: 'light' }); assert.equal(m.D.doc().look.palette.mode, 'custom'); assert.equal(custom.hasAttribute('hidden'), false); assert.equal(m.sheet.el.querySelectorAll('.hd-look-swatch').length, 5);
    const acc = q('[data-hd-field="palette-accent"]'); acc.value = '#ABCDEF'; fire(acc, 'change', {}); assert.equal(m.D.doc().look.palette.custom.accent, '#abcdef'); const hue = q('[data-hd-field="palette-hue"]'); hue.value = '400'; fire(hue, 'change', {}); assert.equal(m.D.doc().look.palette.custom.surface_hue, 40);
    hue.value = '123.6'; fire(hue, 'input', {}); assert.equal(m.D.doc().look.palette.custom.surface_hue, 124); assert.ok(m.D.execs[m.D.execs.length - 1].meta.coalesce, 'moving the slider coalesces'); seg('palette-scheme', 'dark'); assert.equal(m.D.doc().look.palette.custom.scheme, 'dark');
    seg('palette-mode', 'theme'); assert.equal(m.D.doc().look.palette.mode, 'theme'); assert.ok(m.D.doc().look.palette.custom, 'custom is kept for the way back (forSave drops it)'); assert.ok(custom.hasAttribute('hidden'));
    seg('backdrop', 'gradient'); eq(m.D.doc().look.backdrop, { kind: 'gradient' }); seg('density', 'spacious'); assert.equal(m.D.doc().look.density, 'spacious'); const rad = q('[data-hd-field="radius"]'); rad.value = '0.47'; fire(rad, 'change', {}); assert.equal(m.D.doc().look.radius_scale, 0.5);
    seg('fit', 'fit_screen'); assert.equal(m.D.doc().grid.fit, 'fit_screen'); assert.equal(m.D.execs[m.D.execs.length - 1].label, 'homedash.cmd.grid'); assert.ok(q('[data-hd-field="row-px"]').parentElement.hasAttribute('hidden')); seg('row-kind', 'fixed'); assert.equal(m.D.doc().grid.row_unit, 96); assert.equal(q('[data-hd-field="row-px"]').parentElement.hasAttribute('hidden'), false);
    const px = q('[data-hd-field="row-px"]'); px.value = '30'; fire(px, 'change', {}); assert.equal(m.D.doc().grid.row_unit, 48); fire(q('[data-hd-field="compact"]'), 'click', {}); assert.equal(m.D.doc().pages[0].compact, false); noStyleAttr(m.sheet.el);
    const foot = m.sheet.el.querySelector('.hd-sheet-foot'); fire(foot, 'click', {}, foot.querySelector('.hd-dlg-btn')); assert.equal(await p, true); assert.ok(m.D.calls.includes('seal')); }],
  ['dialogs: the page tab upgrade adds the + button and icons, renames inline with the server text rules, moves with Alt+Shift+Arrow and by drag', async () => {
    const m = setup({ designer: true }), { E, document, D } = m, slot = document.createElement('div'); slot.setAttribute('data-hd-slot', 'pages'); const tabs = document.createElement('div'); tabs.setAttribute('class', 'hd-ptabs'); slot.appendChild(tabs); document.body.appendChild(slot);
    const mk = (id, name, sel) => { const b = document.createElement('button'); b.setAttribute('class', 'hd-ptab'); b.setAttribute('role', 'tab'); b.setAttribute('data-page', id); b.setAttribute('aria-selected', sel ? 'true' : 'false'); const l = document.createElement('span'); l.setAttribute('class', 'hd-ptab-label'); l.textContent = name; b.appendChild(l); tabs.appendChild(b); return b; };
    const t1 = mk('main', 'Main', true), t2 = mk('p2', 'Second', false); [t1, t2].forEach((t, i) => { t.getBoundingClientRect = () => ({ left: i * 100, width: 100, top: 0, height: 32, right: i * 100 + 100, bottom: 32 }); }); slot.getBoundingClientRect = () => ({ left: 0, top: 0, width: 400, height: 32, right: 400, bottom: 32 });
    const h = E.ddialogs.enhanceTabs(D, slot); assert.equal(E.ddialogs.enhanceTabs(D, slot), h, 'idempotent per designer'); const add = slot.querySelector('.hd-ptab-add'); assert.ok(add); assert.equal(slot.children[slot.children.length - 1], add); assert.equal(tabs.querySelector('.hd-ptab-add'), null, 'no non-tab inside the tablist');
    assert.equal(t1.querySelectorAll('.hd-ptab-icon').length, 1); assert.equal(t1.children[0].getAttribute('data-hd-icon'), D.doc().pages[0].icon); D.exec(E.commands.setPageIcon(D.doc(), { pageId: 'main', icon: 'bulb' })); await flush(40); assert.equal(t1.querySelectorAll('.hd-ptab-icon').length, 1); assert.equal(t1.querySelector('.hd-ptab-icon').getAttribute('data-hd-icon'), 'bulb');
    D.execs.length = 0; const kd = (key, extra, target) => fire(slot, 'keydown', Object.assign({ key }, extra), target); const ev = kd('ArrowRight', { altKey: true, shiftKey: true }, t1); assert.equal(ev.defaultPrevented, true); eq(D.doc().pages.map(p => p.id), ['p2', 'main']); assert.equal(D.execs[0].label, 'homedash.cmd.page_move');
    kd('ArrowLeft', { altKey: true, shiftKey: true }, t1); eq(D.doc().pages.map(p => p.id), ['main', 'p2']); D.execs.length = 0;
    kd('F2', {}, t1); let input = slot.querySelector('.hd-ptab-input'); assert.ok(input); assert.equal(input.value, 'Main'); assert.equal(t1.getAttribute('data-hd-renaming'), 'true'); assert.equal(document.activeElement, input);
    input.value = 'http://x'; const bad = fire(input, 'keydown', { key: 'Enter' }); assert.equal(bad.defaultPrevented, true); assert.ok(slot.querySelector('.hd-ptab-input'), 'an invalid name keeps the input open'); assert.equal(input.getAttribute('aria-invalid'), 'true'); assert.equal(D.execs.length, 0); assert.ok(D.calls.includes('announce:homedash.error.url_not_allowed'));
    input.value = 'Kitchen'; fire(input, 'keydown', { key: 'Enter' }); assert.equal(slot.querySelector('.hd-ptab-input'), null); assert.equal(D.doc().pages[0].name, 'Kitchen'); assert.equal(t1.hasAttribute('data-hd-renaming'), false); assert.equal(slot.children[slot.children.length - 1], add);
    kd('Enter', {}, t2); assert.equal(slot.querySelector('.hd-ptab-input'), null, 'Enter renames only the selected tab'); kd('Enter', {}, t1); input = slot.querySelector('.hd-ptab-input'); assert.ok(input); fire(input, 'keydown', { key: 'Escape' }); assert.equal(slot.querySelector('.hd-ptab-input'), null); assert.equal(D.doc().pages[0].name, 'Kitchen');
    fire(slot, 'dblclick', {}, t2); input = slot.querySelector('.hd-ptab-input'); input.value = 'x' + fc(0x202e); fire(input, 'blur', {}); assert.equal(slot.querySelector('.hd-ptab-input'), null, 'blur with an invalid name cancels'); assert.equal(D.doc().pages[1].name, 'Second');
    fire(slot, 'dblclick', {}, t2); input = slot.querySelector('.hd-ptab-input'); input.value = '   '; fire(input, 'keydown', { key: 'Enter' }); assert.equal(slot.querySelector('.hd-ptab-input'), null, 'empty cancels'); assert.equal(D.doc().pages[1].name, 'Second');
    const before = D.doc().pages.length; fire(slot, 'click', {}, add); assert.equal(D.doc().pages.length, before + 1); await flush(40); assert.ok(slot.querySelector('.hd-ptab-input') === null, 'the new tab exists only after the bar rebuilds; no crash without it');
    const win = createDocument(); document.defaultView = win; D.execs.length = 0; const pd = fire(slot, 'pointerdown', { button: 0, pointerId: 1, isPrimary: true, pointerType: 'mouse', clientX: 50, clientY: 10, buttons: 1 }, t1);
    fire(win, 'pointermove', { pointerId: 1, pointerType: 'mouse', clientX: 53, clientY: 10, buttons: 1 }); assert.equal(t1.hasAttribute('data-hd-dragging'), false, 'below the 5 px threshold'); fire(win, 'pointermove', { pointerId: 1, pointerType: 'mouse', clientX: 140, clientY: 10, buttons: 1 }); assert.equal(t1.getAttribute('data-hd-dragging'), 'true'); assert.equal(t1.style.getPropertyValue('--hd-e-tx'), '90px'); assert.ok(slot.querySelector('.hd-ptab-caret'));
    fire(win, 'pointerup', { pointerId: 1, pointerType: 'mouse', clientX: 160, clientY: 10 }); eq(D.doc().pages.map(p => p.id).slice(0, 2), ['p2', 'main']); assert.equal(t1.hasAttribute('data-hd-dragging'), false); assert.equal(slot.querySelector('.hd-ptab-caret'), null); assert.equal(D.execs.length, 1); assert.ok(pd); noStyleAttr(slot); h.destroy(); }],
  ['dialogs: the page menu follows the position, disables the impossible and asks before deleting a page with widgets', async () => {
    const m = setup({ designer: true }), { E, D } = m, items = E.ddialogs.pageMenu(D, 'main'), by = id => items.find(i => i.id === id); eq(items.map(i => i.id || '-'), ['page_rename', 'page_icon', 'page_duplicate', '-', 'page_left', 'page_right', '-', 'page_delete']); assert.equal(by('page_left').disabled, true); assert.equal(by('page_right').disabled, false); assert.equal(by('page_duplicate').label, 'homedash.ctx.duplicate'); assert.equal(by('page_delete').danger, true);
    assert.equal(E.ddialogs.pageMenu(D, 'p2').find(i => i.id === 'page_right').disabled, true); by('page_right').run(); eq(D.doc().pages.map(p => p.id), ['p2', 'main']); by('page_duplicate').run(); assert.equal(D.doc().pages.length, 3);
    const one = setup({ designer: true, docExtra: {} }); one.D.setDoc(Object.assign({}, one.D.doc(), { pages: [one.D.doc().pages[0]] })); assert.equal(E.ddialogs.pageMenu(one.D, 'main').find(i => i.id === 'page_delete').disabled, true);
    const del = E.ddialogs.pageMenu(D, 'main').find(i => i.id === 'page_delete'), n0 = D.doc().pages.length; const p = del.run(); await flush(5); assert.equal(m.sheet.el.querySelector('.hd-sheet-title').textContent, 'homedash.dialog.page_delete_title{"name":"Main"}'); press(m.sheet, 'cancel'); await p; assert.equal(D.doc().pages.length, n0);
    const p2 = del.run(); await flush(5); press(m.sheet, 'confirm'); await p2; assert.equal(D.doc().pages.length, n0 - 1); const ic = E.ddialogs.pageMenu(D, 'p2').find(i => i.id === 'page_icon'); const p3 = ic.run(); await flush(5); const list = m.sheet.el.querySelector('.hd-icongrid-list'); fire(list, 'click', {}, list.querySelector('[data-icon="bulb"]')); assert.equal(D.doc().pages.find(x => x.id === 'p2').icon, 'bulb');
    const foot = m.sheet.el.querySelector('.hd-sheet-foot'); fire(foot, 'click', {}, foot.querySelector('.hd-dlg-btn')); await p3; }]
];
```
- [ ] Run wiring step W for D1 and D2. PASS. Commit — `feat(homedash): look editor, dashboard dialogs, import/export and page tab upgrades`.

### Task D3: First run and import from the home screen (D1, Q12)
**Files:** `ui/js/desktop/apps/homedash-home.js`, `ui/js/desktop/apps/homedash.js`, `scripts/homedash-tests/home.test.mjs`. **Existing symbols changed:** `render` and the click handler (homedash-home; LOW), `onEmpty` and the `Home.create` call (homedash.js; impact first, LOW). The checkout has CRLF line endings for these files (index LF): edit with a tool that keeps them.
- [ ] `homedash-home.js`: the pure export `HomeDashHome.choiceState(state, pending)` → `{'create-starter', 'create-empty', 'create-ha', import}`, each `{hidden, disabled, busy}` (hidden = read-only, or HA not in `templates`; import is hidden only when read-only; disabled = busy; busy = `pending === action`); `render` applies it to every `[data-action]` of the four. A secondary `button.hdx-btn[data-action=import]` ("Import…", `homedash.menu.import`) sits below the first-run choices (`div.hdx-secondary`) and in the list head; its click calls `o.onImport()` without a pending create. First run keeps D1's three choices (no "generate from rooms", no "ask the agent").
- [ ] `homedash.js`: `onEmpty: () => inst.createEmpty().then(() => { const s = inst.state(); if (s.mode === 'dashboard' && s.edit && s.edit.canEdit && s.edit.mode === 'view') return inst.enterEdit(); })` (an empty dashboard opens straight in the designer) and `onImport: () => inst.run('import')`.
<!-- embed: dryrun-m4/overlay/diffs2/homedash-home.js.diff -->
```diff
--- a/ui/js/desktop/apps/homedash-home.js
+++ b/ui/js/desktop/apps/homedash-home.js
@@ -1,3 +1,3 @@
 /* HomeDash desktop home view (F2): the dashboard list and the first-run empty state, painted from the desktop theme (desktop-app-homedash.css).
-   window.HomeDashHome.create({t, lang, onOpen(id), onStarter(), onEmpty(), onHa(), sourceName(src), onDelete(summary)}) returns {el, render(state), destroy()}.
+   window.HomeDashHome.create({t, lang, onOpen(id), onStarter(), onEmpty(), onHa(), onImport(), sourceName(src), onDelete(summary)}) returns {el, render(state), destroy()}.
    render(state) reads state.dashboards (null while loading), desktopReadonly, busy and notice (Task 12 state). Names and dates come from the
@@ -7,2 +7,8 @@
     const DOT = ' ' + String.fromCodePoint(0xb7) + ' ';
+    // Visibility of the create choices and of Import... as a pure function of the host state (M4 D3, Q12): hidden = read-only, or Home Assistant is not a listed
+    // template; disabled = a create is running; busy = the pressed choice. Import needs no dashboard and no template, only a writable desktop.
+    function choiceState(state, pending) {
+        const st = state || {}, ro = !!st.desktopReadonly, busy = !!st.busy, ha = Array.isArray(st.templates) && st.templates.includes('home_assistant'), one = (action, hidden) => ({ hidden, disabled: busy, busy: busy && pending === action });
+        return { 'create-starter': one('create-starter', ro), 'create-empty': one('create-empty', ro), 'create-ha': one('create-ha', ro || !ha), import: { hidden: ro, disabled: busy, busy: false } };
+    }
     const NOTICE_TONES = { limit: 'warn', create_failed: 'warn', list_failed: 'warn', open_failed: 'warn', delete_failed: 'warn', delete_conflict: 'warn', template_empty: 'warn', source_unavailable: 'warn', readonly: 'info', dashboard_deleted: 'info', dashboard_missing: 'info' };
@@ -42,3 +48,3 @@
             [button('create-starter', 'gauge', t('homedash.home.create_starter'), true), button('create-empty', 'plus', t('homedash.home.create_empty'), false),
-                button('create-ha', 'home', t('homedash.home.create_ha'), false)])]);
+                button('create-ha', 'home', t('homedash.home.create_ha'), false), button('import', 'upload', t('homedash.menu.import'), false)])]);
         const notice = el('p', { class: 'hdx-note hdx-notice', role: 'status', 'data-tone': 'info' }); /* always rendered, empty when there is nothing to say: a live region must exist before its text does */
@@ -51,3 +57,4 @@
                 choice('create-empty', 'dashboard', t('homedash.home.create_empty'), t('homedash.home.create_empty_hint'), false),
-                choice('create-ha', 'home', t('homedash.home.create_ha'), t('homedash.home.create_ha_hint'), false)])]);
+                choice('create-ha', 'home', t('homedash.home.create_ha'), t('homedash.home.create_ha_hint'), false)]),
+            el('div', { class: 'hdx-secondary' }, button('import', 'upload', t('homedash.menu.import'), false))]);
         const root = el('section', { class: 'hdx-home', 'data-hdx-mode': 'loading', 'aria-label': t('homedash.home.title') }, el('div', { class: 'hdx-home-inner' }, [head, notice, roNote, skeleton, list, empty]));
@@ -74,5 +81,5 @@
             setAttr(head, 'hidden', mode !== 'list'); setAttr(list, 'hidden', mode !== 'list'); setAttr(empty, 'hidden', mode !== 'empty'); setAttr(skeleton, 'hidden', mode !== 'loading');
-            const ha = Array.isArray(st.templates) && st.templates.includes('home_assistant');
-            for (const b of root.querySelectorAll('[data-action="create-starter"], [data-action="create-empty"], [data-action="create-ha"]')) {
-                setAttr(b, 'hidden', ro || (b.getAttribute('data-action') === 'create-ha' && !ha)); setAttr(b, 'aria-disabled', busy ? 'true' : null); setAttr(b, 'aria-busy', busy && b.getAttribute('data-action') === pending ? 'true' : null);
+            const states = choiceState(st, pending);
+            for (const b of root.querySelectorAll('[data-action="create-starter"], [data-action="create-empty"], [data-action="create-ha"], [data-action="import"]')) {
+                const c = states[b.getAttribute('data-action')]; setAttr(b, 'hidden', c.hidden); setAttr(b, 'aria-disabled', c.disabled ? 'true' : null); setAttr(b, 'aria-busy', c.busy ? 'true' : null);
             }
@@ -93,2 +100,3 @@
             else if (action === 'create-ha') { pending = action; o.onHa(); }
+            else if (action === 'import') { if (typeof o.onImport === 'function') o.onImport(); }
             else if (action === 'open' && id) o.onOpen(id);
@@ -98,3 +106,3 @@
     }
-    window.HomeDashHome = Object.freeze({ create });
+    window.HomeDashHome = Object.freeze({ create, choiceState });
 })();
```
<!-- embed: dryrun-m4/overlay/diffs2/homedash.js.diff -->
```diff
--- a/ui/js/desktop/apps/homedash.js
+++ b/ui/js/desktop/apps/homedash.js
@@ -37,3 +37,3 @@
         const viewHost = el('div', { class: 'hdx-view', hidden: 'true' }), statePanel = el('div', { class: 'hdx-state', hidden: 'true' });
-        const home = Home.create({ t, lang, onOpen: dashboardId => inst.open(dashboardId), onStarter: () => inst.createStarter(), onEmpty: () => inst.createEmpty(), onHa: () => inst.createHomeAssistant(), sourceName: src => sourceName(src), onDelete: summary => askDelete(summary) });
+        const home = Home.create({ t, lang, onOpen: dashboardId => inst.open(dashboardId), onStarter: () => inst.createStarter(), onEmpty: () => inst.createEmpty().then(() => { const s = inst.state(); if (s.mode === 'dashboard' && s.edit && s.edit.canEdit && s.edit.mode === 'view') return inst.enterEdit(); }), onHa: () => inst.createHomeAssistant(), onImport: () => inst.run('import'), sourceName: src => sourceName(src), onDelete: summary => askDelete(summary) });
         setAttr(home.el, 'hidden', true);
```
- [ ] `home.test.mjs` (two tests added; the existing ones stay green):
<!-- embed: dryrun-m4/overlay/diffs2/home.test.mjs.diff -->
```diff
--- a/scripts/homedash-tests/home.test.mjs
+++ b/scripts/homedash-tests/home.test.mjs
@@ -4,3 +4,3 @@ function setup() {
   const { window, document } = loadFiles(FILES, { dom: true }), calls = [];
-  const home = window.HomeDashHome.create({ t: (k, v) => (v && v.name ? k + ':' + v.name : v && v.source ? k + ':' + v.source : k), lang: 'en', onOpen: id => calls.push('open:' + id), onStarter: () => calls.push('starter'),
+  const home = window.HomeDashHome.create({ onImport: () => calls.push('import'), t: (k, v) => (v && v.name ? k + ':' + v.name : v && v.source ? k + ':' + v.source : k), lang: 'en', onOpen: id => calls.push('open:' + id), onStarter: () => calls.push('starter'),
     onEmpty: () => calls.push('empty'), onHa: () => calls.push('ha'), sourceName: x => 'src:' + x, onDelete: s => calls.push('delete:' + s.id) });
@@ -48,3 +48,13 @@ export const tests = [
     home.render({ dashboards: [], notice: 'source_unavailable', noticeSource: 'ha' }); assert.equal(n.getAttribute('data-tone'), 'warn');
-    assert.equal(n.textContent, 'homedash.home.notice_source_unavailable:src:ha'); }]
+    assert.equal(n.textContent, 'homedash.home.notice_source_unavailable:src:ha'); }],
+  ['home: choiceState is a pure table (read-only hides everything, Home Assistant needs the template, busy marks the pressed choice, import ignores templates)', () => {
+    const { window } = loadFiles(FILES, { dom: true }), cs = window.HomeDashHome.choiceState, plain = v => JSON.parse(JSON.stringify(v));
+    assert.deepEqual(plain(cs({ desktopReadonly: false, templates: [] }, '')), { 'create-starter': { hidden: false, disabled: false, busy: false }, 'create-empty': { hidden: false, disabled: false, busy: false }, 'create-ha': { hidden: true, disabled: false, busy: false }, import: { hidden: false, disabled: false, busy: false } });
+    assert.equal(cs({ templates: ['home_assistant'] }, '')['create-ha'].hidden, false); assert.ok(Object.values(cs({ desktopReadonly: true, templates: ['home_assistant'] }, '')).every(c => c.hidden));
+    const busy = plain(cs({ busy: true, templates: ['home_assistant'] }, 'create-empty')); assert.equal(busy['create-empty'].busy, true); assert.equal(busy['create-starter'].busy, false); assert.ok(Object.values(busy).every(c => c.disabled)); assert.equal(busy.import.busy, false); assert.doesNotThrow(() => cs(null, undefined)); }],
+  ['home: Import is a secondary action below the first-run choices and in the list head, hidden when read-only, and runs onImport without a pending create', () => {
+    const { home, calls, click } = setup(); home.render({ dashboards: [], desktopReadonly: false }); const imp = home.el.querySelector('.hdx-empty [data-action="import"]'); assert.ok(imp); assert.equal(imp.textContent, 'homedash.menu.import');
+    assert.equal(imp.parentElement.getAttribute('class'), 'hdx-secondary'); click(imp); assert.deepEqual(calls, ['import']); home.render({ dashboards: [D], desktopReadonly: false }); assert.ok(home.el.querySelector('.hdx-home-head [data-action="import"]'));
+    click(home.el.querySelector('.hdx-home-head [data-action="import"]')); assert.deepEqual(calls, ['import', 'import']); home.render({ dashboards: [], desktopReadonly: false, busy: true }); click(imp); assert.equal(calls.length, 2, 'the busy guard holds');
+    home.render({ dashboards: [], desktopReadonly: true }); assert.ok(imp.hasAttribute('hidden')); assert.ok(home.el.querySelector('.hdx-home-head [data-action="import"]').hasAttribute('hidden')); }]
 ];
```
- [ ] Run `npm run test:homedash` and `npm run check:ui`. PASS. Commit — `feat(homedash): empty dashboards open in the designer, import from the home screen`.

**Batch D review (reader-agent, Critical/Important only):**
- The import cap is checked before reading; envelope detection is right; nothing renders before validate; issue texts use textContent only.
- Export holds nothing server-owned or secret.
- Every look field is one command; text coalescing is sealed.
- The palette shape follows the server rules; the snapshot comes only through the lane's prepare.
- Tab rename, reorder and menu keep focus; the last page cannot be deleted.
- The New flow exits edit mode first.

security-agent (D14): import and export.

---

## Batch E — V14 chrome CSS, harness, gates (D12)

### Task E1: Editor chrome CSS
**Files:** `ui/css/desktop-app-homedash.css`. **Existing symbols changed:** the sheet rules at lines 100–110 (host confirm sheet; LOW). The file is CRLF in the checkout (index LF).
- [ ] Measure `(Get-Content ui/css/desktop-app-homedash.css).Count` (124 today). It must stay ≤ 900 (host.test). The dry-run result is **381 lines**.
- [ ] Apply the diff below (it carries the header comment, the new sheet rules for the kinds `confirm`, `conflict`, `leave` and `form` in place of lines 100–110, and the editor block before the final `@media (prefers-reduced-motion…)` block). Every rule is written out; the comment lines state the contracts. Revision-2 content: the field-border rule (Q17, after `.hd-f-input` rules), the I8 states, `.hd-eb-more`/`.hd-eb-drawer` in the bar button lists, and the two-selector transition reset.
<!-- embed: dryrun-m4/overlay/diffs2/desktop-app-homedash.css.diff -->
```diff
--- a/ui/css/desktop-app-homedash.css
+++ b/ui/css/desktop-app-homedash.css
@@ -1,2 +1,2 @@
-/* HomeDash desktop host: window bar, banners, home view (list and first run), state panels and the confirm sheet. Painted from the desktop
+/* HomeDash desktop host: window bar, banners, home view (list and first run), state panels the confirm sheet and the designer chrome of .hd-root[data-hd-mode=design] (contract C15). Painted from the desktop
    theme's app materials (--vd-mat-*, --vd-*, --ds-*): no hex or literal colour functions, no theme ids, no uppercase labels; motion only
@@ -99,13 +99,20 @@
 .hdx-spinner { inline-size: 32px; block-size: 32px; border: 3px solid color-mix(in srgb, var(--vd-text) 14%, transparent); border-block-start-color: var(--vd-accent); border-radius: 50%; }
-/* Confirm sheet (hd-sheet inside the app root; the engine ships no layer styles) */
-.hdx-app > .hd-sheet-layer:not([data-kind]) { position: absolute; inset: 0; z-index: 20; display: grid; place-items: center; padding: 24px; background: var(--vd-theme-overlay-bg, color-mix(in srgb, var(--vd-text) 28%, transparent)); }
-.hdx-app .hd-sheet[data-kind="confirm"] { inline-size: min(440px, 100%); padding: 20px 22px 18px; border: 1px solid var(--vd-mat-card-border, var(--hdx-line)); border-radius: var(--hdx-radius); background: var(--vd-mat-card, var(--hdx-card)); box-shadow: var(--vd-mat-card-shadow, 0 18px 48px color-mix(in srgb, var(--vd-text) 22%, transparent)); color: var(--vd-text); }
-.hdx-app .hd-sheet[data-kind="confirm"] .hd-sheet-title { margin: 0 0 8px; color: var(--vd-mat-heading-color, var(--vd-text)); font: var(--vd-mat-heading-weight, 650) 17px/1.3 var(--vd-mat-heading-font, var(--vd-font-ui, system-ui, sans-serif)); }
-.hdx-app .hd-sheet[data-kind="confirm"] .hd-sheet-text { margin: 0 0 6px; color: var(--hdx-muted); }
-.hdx-app .hd-sheet[data-kind="confirm"] .hd-sheet-foot { display: flex; justify-content: flex-end; gap: 8px; margin-block-start: 18px; }
-.hdx-app .hd-sheet[data-kind="confirm"] .hd-sheet-btn { min-block-size: 36px; padding: 0 16px; border: 1px solid var(--vd-mat-btn-border, var(--hdx-line)); border-radius: var(--vd-mat-btn-radius, 8px); background: var(--vd-mat-btn, var(--vd-theme-control-bg)); color: var(--vd-mat-btn-text, var(--vd-text)); font-weight: 600; cursor: pointer; }
-.hdx-app .hd-sheet[data-kind="confirm"] .hd-sheet-btn:hover { background: var(--vd-mat-btn-hover, var(--vd-theme-control-hover)); }
-.hdx-app .hd-sheet[data-kind="confirm"] .hd-sheet-btn[data-tone="primary"] { border-color: transparent; background: var(--vd-mat-primary, var(--vd-accent)); color: var(--vd-mat-primary-text, var(--vd-accent-contrast)); }
-.hdx-app .hd-sheet[data-kind="confirm"] .hd-sheet-btn[data-tone="danger"] { border-color: transparent; background: color-mix(in srgb, var(--ds-color-danger-fg) 84%, var(--vd-text)); color: var(--vd-theme-app-bg); }
-.hdx-app .hd-sheet[data-kind="confirm"] .hd-sheet-btn[data-tone="danger"]:hover { background: color-mix(in srgb, var(--ds-color-danger-fg) 74%, var(--vd-text)); }
+/* Sheets: the host's confirm sheet and the engine's sheets inside .hd-root (confirm, conflict, leave, form); the look editor docks at the end edge */
+.hdx-app > .hd-sheet-layer:not([data-kind]), .hdx-app .hd-root > .hd-sheet-layer:not([data-kind]) { position: absolute; inset: 0; z-index: 40; display: grid; place-items: center; padding: 24px; background: var(--vd-theme-overlay-bg, color-mix(in srgb, var(--vd-text) 28%, transparent)); }
+.hdx-app .hd-sheet:is([data-kind="confirm"], [data-kind="conflict"], [data-kind="leave"], [data-kind="form"]) { box-sizing: border-box; display: flex; flex-direction: column; inline-size: min(460px, 100%); max-block-size: 100%; padding: 20px 22px 18px; overflow: auto; border: 1px solid var(--vd-mat-card-border, var(--hdx-line)); border-radius: var(--hdx-radius); background: var(--vd-mat-card, var(--hdx-card)); box-shadow: var(--vd-mat-card-shadow, 0 18px 48px color-mix(in srgb, var(--vd-text) 22%, transparent)); color: var(--vd-text); }
+.hdx-app .hd-sheet:is([data-kind="confirm"], [data-kind="conflict"], [data-kind="leave"], [data-kind="form"]) .hd-sheet-title { margin: 0 0 8px; color: var(--vd-mat-heading-color, var(--vd-text)); font: var(--vd-mat-heading-weight, 650) 17px/1.3 var(--vd-mat-heading-font, var(--vd-font-ui, system-ui, sans-serif)); }
+.hdx-app .hd-sheet:is([data-kind="confirm"], [data-kind="conflict"], [data-kind="leave"], [data-kind="form"]) .hd-sheet-text { margin: 0 0 6px; color: var(--hdx-muted); }
+.hdx-app .hd-sheet:is([data-kind="confirm"], [data-kind="conflict"], [data-kind="leave"], [data-kind="form"]) .hd-sheet-foot { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; margin-block-start: 18px; }
+.hdx-app .hd-sheet .hd-sheet-foot:empty { display: none; }
+.hdx-app .hd-sheet :is(.hd-sheet-btn, .hd-dlg-btn) { min-block-size: 36px; padding: 0 16px; border: 1px solid var(--vd-mat-btn-border, var(--hdx-line)); border-radius: var(--vd-mat-btn-radius, 8px); background: var(--vd-mat-btn, var(--vd-theme-control-bg)); color: var(--vd-mat-btn-text, var(--vd-text)); font-weight: 600; cursor: pointer; }
+.hdx-app .hd-sheet :is(.hd-sheet-btn, .hd-dlg-btn):hover { background: var(--vd-mat-btn-hover, var(--vd-theme-control-hover)); }
+.hdx-app .hd-sheet :is(.hd-sheet-btn, .hd-dlg-btn)[data-tone="primary"] { border-color: transparent; background: var(--vd-mat-primary, var(--vd-accent)); color: var(--vd-mat-primary-text, var(--vd-accent-contrast)); }
+.hdx-app .hd-sheet :is(.hd-sheet-btn, .hd-dlg-btn)[data-tone="danger"] { border-color: transparent; background: color-mix(in srgb, var(--ds-color-danger-fg) 84%, var(--vd-text)); color: var(--vd-theme-app-bg); }
+.hdx-app .hd-sheet :is(.hd-sheet-btn, .hd-dlg-btn)[data-tone="danger"]:hover { background: color-mix(in srgb, var(--ds-color-danger-fg) 74%, var(--vd-text)); }
+.hdx-app .hd-sheet[data-kind="form"] .hd-sheet-body { display: grid; gap: 16px; }
+.hdx-app .hd-root > .hd-sheet-layer:not([data-kind]):has(.hd-look) { place-items: stretch end; padding: 0; background: transparent; }
+.hdx-app .hd-sheet[data-kind="form"]:has(.hd-look) { inline-size: min(360px, 100%); block-size: 100%; max-block-size: none; padding: 0; overflow: hidden; border-block: 0; border-inline-end: 0; border-radius: 0; }
+.hdx-app .hd-sheet[data-kind="form"]:has(.hd-look) > .hd-sheet-head { padding: 16px 20px 4px; }
+.hdx-app .hd-sheet[data-kind="form"]:has(.hd-look) > .hd-sheet-body { flex: 1 1 auto; min-block-size: 0; overflow: auto; padding: 8px 20px 20px; }
+.hdx-app .hd-sheet[data-kind="form"]:has(.hd-look) > .hd-sheet-foot { margin: 0; padding: 12px 20px; border-block-start: 1px solid var(--hdx-line); }
 @media (pointer: coarse) { .hdx-btn, .hdx-back, .hdx-app .hd-sheet-btn { min-block-size: 44px; } .hdx-card-delete { inline-size: 44px; block-size: 44px; } }
@@ -114,2 +121,253 @@
   .hdx-conn-text { position: absolute; inline-size: 1px; block-size: 1px; margin: -1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; } }
+/* ==== Designer chrome (M4, V14). Painted from --vd-mat-*/--vd-*/--ds-* only: no blur, glass or glow, one accent (Done; canvas selection via
+   --hd-focus, the dashboard accent that E.widgets.applyLook keeps at 3:1 on cards and canvas). Text on bar and side panels: --vd-text and
+   --vd-theme-muted (pairs the materials test measures there); --vd-mat-label-color only on cards and sheets. JS geometry: --hd-e-* (C15).
+   8 px grid, 13 px text, 16 px icons, 32 px controls (44 px coarse). Selection and focus never change layout. */
+.hd-root[data-hd-host] { --hdx-e-line: var(--vd-mat-divider, var(--vd-theme-border)); --hdx-e-muted: var(--vd-theme-muted); --hdx-e-sel: var(--hd-focus, var(--vd-accent)); --hdx-e-ring: var(--ds-color-border-focus, var(--vd-accent)); --hdx-e-r: var(--vd-mat-btn-radius, 6px); --hdx-e-fr: var(--vd-mat-field-radius, 6px); }
+.hdx-app[data-hdx-edit="design"] > .hdx-bar { display: none; }
+.hdx-elsewhere, .hdx-save { display: inline-flex; flex: none; align-items: center; gap: 8px; color: var(--vd-theme-muted); font-size: 12.5px; font-weight: 600; }
+.hdx-elsewhere { min-block-size: 32px; padding: 0 4px 0 12px; border: 1px solid var(--hdx-line); border-radius: 999px; }
+.hdx-app :is(.hdx-elsewhere, .hdx-save) .hdx-btn { min-block-size: 26px; padding: 0 10px; font-size: 12.5px; }
+.hdx-save:is([data-phase="retrying"], [data-phase="invalid"], [data-phase="conflict"], [data-phase="gone"]) { color: var(--vd-text); }
+.hdx-save:is([data-phase="retrying"], [data-phase="invalid"], [data-phase="conflict"], [data-phase="gone"])::before { content: ""; inline-size: 8px; block-size: 8px; border-radius: 50%; background: var(--ds-color-warning-fg); }
+/* Layout: bar / conflict bar / palette 280 · canvas · inspector 320; drawers below 900 px of root width */
+.hd-root[data-hd-host][data-hd-mode="design"] { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; grid-template-rows: 48px auto minmax(0, 1fr); color: var(--vd-text); font: 13px/1.4 var(--vd-font-ui, system-ui, sans-serif); }
+.hd-root[data-hd-mode="design"] > .hd-editbar { grid-area: 1 / 1 / 2 / -1; min-inline-size: 0; }
+.hd-root[data-hd-mode="design"] > .hd-conflict-bar { grid-area: 2 / 1 / 3 / -1; }
+.hd-root[data-hd-mode="design"] > .hd-palette { grid-area: 3 / 1 / 4 / 2; inline-size: 280px; border-inline-end: 1px solid var(--hdx-e-line); }
+.hd-root[data-hd-mode="design"] > .hd-stage { grid-area: 3 / 2 / 4 / 3; min-inline-size: 0; }
+.hd-root[data-hd-mode="design"] > .hd-inspector { grid-area: 3 / 3 / 4 / 4; inline-size: 320px; border-inline-start: 1px solid var(--hdx-e-line); }
+.hd-root[data-hd-mode="design"] > :is(.hd-palette, .hd-inspector) { position: relative; display: flex; flex-direction: column; min-block-size: 0; overflow: hidden; background: var(--vd-mat-side, var(--vd-theme-chrome-bg)); box-shadow: var(--vd-mat-side-edge, none); color: var(--vd-text); }
+.hd-root[data-hd-mode="design"] > :is(.hd-tabs, .hd-drawer-scrim) { display: none; }
+.hd-root[data-hd-mode="design"] > :is(.hd-toasts, .hd-sheet-layer, .hd-drop-avatar) { grid-area: 1 / 1 / -1 / -1; }
+.hd-root[data-hd-mode="design"][data-hd-drawers="true"] { grid-template-columns: minmax(0, 1fr); }
+.hd-root[data-hd-mode="design"][data-hd-drawers="true"] > .hd-stage { grid-area: 3 / 1 / 4 / 2; }
+.hd-root[data-hd-mode="design"][data-hd-drawers="true"] > :is(.hd-palette, .hd-inspector) { position: absolute; grid-area: 3 / 1 / 4 / 2; inset-block: 0; z-index: 14; inline-size: min(320px, 85%); border: 0; box-shadow: var(--vd-mat-card-shadow, none); }
+.hd-root[data-hd-mode="design"][data-hd-drawers="true"] > .hd-palette { inset-inline-start: 0; border-inline-end: 1px solid var(--hdx-e-line); }
+.hd-root[data-hd-mode="design"][data-hd-drawers="true"] > .hd-inspector { inset-inline-end: 0; border-inline-start: 1px solid var(--hdx-e-line); }
+.hd-root[data-hd-mode="design"][data-hd-drawers="true"] > [data-hd-drawer="closed"] { display: none; }
+.hd-root[data-hd-mode="design"][data-hd-drawers="true"]:is([data-hd-panel="palette"], [data-hd-panel="inspector"]) > .hd-drawer-scrim { display: block; position: absolute; grid-area: 3 / 1 / 4 / 2; inset: 0; z-index: 13; background: var(--vd-theme-overlay-bg, color-mix(in srgb, var(--vd-text) 24%, transparent)); }
+.hd-root[data-hd-drawers="true"][data-hd-gesture="drop"] > :is(.hd-palette, .hd-drawer-scrim) { opacity: 0; pointer-events: none; }
+.hd-root :is(.hd-editbar, .hd-palette, .hd-inspector, .hd-conflict-bar, .hd-sheet-layer:not([data-kind]), .hd-drop-avatar) .hd-ico:not(.hd-widget .hd-ico) { flex: none; inline-size: 16px; block-size: 16px; stroke-width: 1.75; }
+.hd-root :is(.hd-editbar, .hd-conflict-bar, .hd-palette, .hd-inspector, .hd-sheet-layer:not([data-kind])) :is(button, input, select, [tabindex="0"]):focus-visible { outline: 2px solid var(--hdx-e-ring); outline-offset: 2px; }
+/* Edit bar (48 px; Done is the only accent fill) */
+.hd-editbar { display: flex; align-items: center; gap: 4px; padding: 0 8px; background: var(--vd-mat-bar, var(--vd-theme-chrome-bg)); box-shadow: var(--vd-mat-bar-edge, none); border-block-end: 1px solid var(--hdx-e-line); color: var(--vd-text); }
+.hd-eb-group { display: flex; flex: none; align-items: center; gap: 4px; min-inline-size: 0; }
+.hd-eb-group[data-hd-group="pages"] { flex: 1 1 auto; margin-inline: 4px; }
+.hd-eb-group[data-hd-group="end"] { gap: 8px; }
+.hd-eb-sep { flex: none; inline-size: 1px; block-size: 20px; margin-inline: 4px; background: var(--hdx-e-line); }
+.hd-root :is(.hd-eb-btn, .hd-eb-more, .hd-eb-drawer, .hd-ptab, .hd-ptab-add, .hd-bp-reset, .hd-save-action, .hd-btn) { display: inline-flex; flex: none; align-items: center; justify-content: center; gap: 6px; box-sizing: border-box; min-block-size: 32px; min-inline-size: 32px; margin: 0; padding: 0 10px; border: 1px solid transparent; border-radius: var(--hdx-e-r); background: transparent; color: var(--vd-text); font: 500 13px/1 var(--vd-font-ui, system-ui, sans-serif); white-space: nowrap; cursor: pointer; }
+.hd-root :is(.hd-eb-btn, .hd-eb-more, .hd-eb-drawer, .hd-ptab-add, .hd-bp-reset):not(:has(.hd-eb-label, .hd-ptab-label)) { inline-size: 32px; padding: 0; }
+.hd-root :is(.hd-eb-btn, .hd-eb-more, .hd-eb-drawer, .hd-ptab, .hd-ptab-add, .hd-bp-reset, .hd-save-action, .hd-btn):hover:not([aria-disabled="true"], :disabled) { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
+.hd-root :is(.hd-eb-btn, .hd-eb-more, .hd-eb-drawer, .hd-btn):active:not([aria-disabled="true"], :disabled) { background: var(--vd-mat-btn-hover, var(--vd-theme-control-hover)); }
+.hd-root :is(.hd-eb-btn, .hd-eb-more, .hd-eb-drawer, .hd-btn)[aria-expanded="true"] { background: var(--vd-mat-selected, var(--vd-theme-accent-soft)); color: var(--vd-mat-selected-text, var(--vd-text)); }
+.hd-root :is(.hd-eb-btn, .hd-eb-more, .hd-eb-drawer, .hd-ptab-add, .hd-bp-reset, .hd-btn, .hd-f-seg-opt)[aria-disabled="true"] { opacity: .45; cursor: default; }
+.hd-root .hd-eb-btn.hd-eb-done { padding: 0 12px 0 10px; border-color: var(--vd-mat-primary-border, transparent); background: var(--vd-mat-primary, var(--vd-accent)); box-shadow: var(--vd-mat-primary-shadow, none); color: var(--vd-mat-primary-text, var(--vd-accent-contrast)); font-weight: 600; }
+.hd-root .hd-eb-btn.hd-eb-done:hover:not([aria-disabled="true"]) { background: var(--vd-mat-primary-hover, var(--vd-mat-primary, var(--vd-accent))); }
+[data-hd-slot="pages"] { position: relative; display: flex; flex: 1 1 auto; align-items: center; gap: 2px; min-inline-size: 0; }
+.hd-ptabs { display: flex; align-items: center; gap: 2px; min-inline-size: 0; overflow-x: auto; scrollbar-width: none; }
+.hd-root .hd-ptab { position: relative; max-inline-size: 200px; padding: 0 12px; color: var(--hdx-e-muted); }
+.hd-root .hd-ptab .hd-ico { inline-size: 14px; block-size: 14px; }
+.hd-ptab-label { overflow: hidden; text-overflow: ellipsis; }
+.hd-root .hd-ptab:hover:not([aria-selected="true"]) { color: var(--vd-text); }
+.hd-root .hd-ptab[aria-selected="true"] { background: var(--vd-mat-selected, var(--vd-theme-accent-soft)); box-shadow: var(--vd-mat-selected-edge, none); color: var(--vd-mat-selected-text, var(--vd-text)); }
+.hd-root .hd-ptab[data-hd-renaming] { visibility: hidden; }
+.hd-root .hd-ptab[data-hd-dragging] { z-index: 2; border-color: var(--hdx-e-line); background: var(--vd-mat-btn, var(--vd-theme-control-bg)); transform: translateX(var(--hd-e-tx, 0px)); cursor: grabbing; }
+.hd-ptab-input { position: absolute; inset-block: 8px; inset-inline-start: var(--hd-e-ix, 0px); inline-size: var(--hd-e-iw, 120px); box-sizing: border-box; padding: 0 8px; border: 1px solid var(--hdx-e-ring); border-radius: var(--hdx-e-r); background: var(--vd-mat-field, var(--vd-theme-control-bg)); color: var(--vd-text); font: 500 13px/1 var(--vd-font-ui, system-ui, sans-serif); outline: none; }
+.hd-ptab-caret { position: absolute; inset-block: 10px; inset-inline-start: var(--hd-e-cx, 0px); inline-size: 2px; border-radius: 1px; background: var(--hdx-e-ring); pointer-events: none; }
+.hd-bp { display: inline-flex; flex: none; gap: 2px; padding: 2px; border: 1px solid var(--vd-mat-field-border, var(--hdx-e-line)); border-radius: var(--hdx-e-r); background: var(--vd-mat-field, var(--vd-theme-control-bg)); }
+.hd-root .hd-bp-opt { display: inline-grid; place-items: center; inline-size: 28px; block-size: 26px; padding: 0; border: 1px solid transparent; border-radius: var(--hdx-e-r); background: transparent; color: var(--vd-text); cursor: pointer; }
+.hd-root .hd-bp-opt:hover:not([aria-checked="true"]) { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
+.hd-root .hd-bp-opt[aria-checked="true"] { border-color: var(--vd-mat-btn-border, var(--hdx-e-line)); background: var(--vd-mat-btn, var(--vd-theme-control-bg)); box-shadow: var(--vd-mat-btn-shadow, none); color: var(--vd-mat-btn-text, var(--vd-text)); }
+.hd-bp-chip { display: inline-flex; flex: none; align-items: center; block-size: 22px; padding: 0 8px; border: 1px solid var(--hdx-e-line); border-radius: 11px; color: var(--hdx-e-muted); font-size: 12px; font-weight: 600; white-space: nowrap; }
+.hd-bp-chip[data-state="custom"] { border-color: color-mix(in srgb, var(--vd-text) 45%, var(--hdx-e-line)); color: var(--vd-text); }
+.hd-save { display: inline-flex; flex: none; align-items: center; gap: 6px; color: var(--hdx-e-muted); font-size: 12px; font-weight: 500; }
+.hd-save-text { min-inline-size: 22ch; max-inline-size: 34ch; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
+.hd-save .hd-ico { inline-size: 14px; block-size: 14px; }
+.hd-save:is([data-phase="retrying"], [data-phase="invalid"], [data-phase="conflict"], [data-phase="readonly"], [data-phase="gone"]) { color: var(--vd-text); }
+.hd-save:is([data-phase="retrying"], [data-phase="invalid"], [data-phase="conflict"], [data-phase="readonly"]) .hd-save-icon { color: color-mix(in srgb, var(--ds-color-warning-fg) 80%, var(--vd-text)); }
+.hd-save[data-phase="gone"] .hd-save-icon { color: color-mix(in srgb, var(--ds-color-danger-fg) 80%, var(--vd-text)); }
+.hd-root .hd-save-action { min-block-size: 24px; padding: 0 8px; border-color: var(--vd-mat-btn-border, var(--hdx-e-line)); background: var(--vd-mat-btn, var(--vd-theme-control-bg)); color: var(--vd-mat-btn-text, var(--vd-text)); font-size: 12px; }
+.hd-conflict-bar { display: flex; align-items: center; gap: 12px; padding: 8px 12px; border-block-end: 1px solid color-mix(in srgb, var(--ds-color-warning-fg) 45%, var(--hdx-e-line)); background: color-mix(in srgb, var(--ds-color-warning-fg) 12%, var(--vd-mat-bar-color, var(--vd-theme-chrome-bg))); color: var(--vd-text); }
+.hd-conflict-bar > .hd-ico { color: color-mix(in srgb, var(--ds-color-warning-fg) 80%, var(--vd-text)); }
+.hd-conflict-text { flex: 1 1 auto; min-inline-size: 0; }
+.hd-root .hd-conflict-bar .hd-eb-btn { inline-size: auto; padding: 0 12px; border-color: var(--vd-mat-btn-border, var(--hdx-e-line)); background: var(--vd-mat-btn, var(--vd-theme-control-bg)); color: var(--vd-mat-btn-text, var(--vd-text)); font-weight: 600; }
+/* Canvas: the stage is the well; the page carries the dashboard backdrop and hairline column guides at 8 % ink */
+.hd-root[data-hd-mode="design"] > .hd-stage.hd-backdrop { background: var(--vd-mat-well, var(--vd-theme-app-bg)); box-shadow: var(--vd-mat-well-shadow, none); }
+.hd-root[data-hd-mode="design"] .hd-page { inline-size: var(--hd-e-pw, auto); margin-inline: auto; background: var(--hd-canvas); }
+.hd-root[data-hd-mode="design"][data-hd-backdrop="theme"] .hd-page { background: var(--hd-canvas-fill); }
+.hd-root[data-hd-mode="design"][data-hd-backdrop="gradient"] .hd-page { background: radial-gradient(60% 50% at 15% 0%, color-mix(in srgb, var(--hd-accent) 9%, transparent), transparent), linear-gradient(160deg, var(--hd-canvas), var(--hd-canvas-2)); }
+.hd-root[data-hd-mode="design"] .hd-page:is([data-hd-preview="md"], [data-hd-preview="sm"]) { margin-block: 16px; min-block-size: calc(100% - 32px); border-radius: 12px; box-shadow: 0 0 0 1px var(--hdx-e-line); }
+.hd-root[data-hd-mode="design"] .hd-page::before { content: ""; position: absolute; inset: 0; pointer-events: none; background: linear-gradient(to right, color-mix(in srgb, var(--hd-ink) 8%, transparent) 0 1px, transparent 1px calc(var(--hd-colw) - 1px), color-mix(in srgb, var(--hd-ink) 8%, transparent) calc(var(--hd-colw) - 1px) var(--hd-colw), transparent var(--hd-colw)) calc(var(--hd-offx, 0px) + var(--hd-gap, 12px)) 0 / calc(var(--hd-colw) + var(--hd-gap, 12px)) 100% repeat-x; }
+.hd-root[data-hd-mode="design"] .hd-page > .hd-widget { transform: translate(calc(var(--hd-e-fx, 0px) + var(--hd-e-dx, 0px)), calc(var(--hd-e-fy, 0px) + var(--hd-e-dy, 0px))); outline-offset: 3px; touch-action: pan-y; cursor: grab; }
+.hd-root[data-hd-mode="design"] .hd-page > .hd-widget[aria-selected="true"] { outline: 1.5px dashed var(--hdx-e-sel); touch-action: none; }
+.hd-root[data-hd-mode="design"] .hd-page > .hd-widget:focus-visible { box-shadow: 0 0 0 2px var(--hdx-e-sel); }
+.hd-root[data-hd-mode="design"] .hd-page > .hd-widget:focus-visible:not([aria-selected="true"]) { outline: none; }
+.hd-root[data-hd-mode="design"] .hd-page > .hd-widget[data-hd-draft]:not([aria-selected="true"]) { outline: 1.5px dashed color-mix(in srgb, var(--hd-ink) 30%, transparent); }
+.hd-root[data-hd-mode="design"] .hd-page > .hd-widget[data-hd-dragging] { z-index: 6; opacity: .94; cursor: grabbing; }
+.hd-root[data-hd-mode="design"] .hd-page > .hd-widget[data-hd-drop="rebind"] { outline: 2px solid var(--hdx-e-sel); box-shadow: 0 0 0 6px color-mix(in srgb, var(--hdx-e-sel) 18%, transparent); }
+.hd-root[data-hd-mode="design"] .hd-page > .hd-widget[data-hd-drop="invalid"] { outline: 2px dashed var(--ds-color-danger-fg); cursor: no-drop; }
+.hd-root[data-hd-gesture="move"], .hd-root[data-hd-gesture="move"] .hd-page > .hd-widget { cursor: grabbing; }
+.hd-draft-badge { position: absolute; inset: 0; z-index: 3; display: inline-flex; align-items: center; gap: 6px; inline-size: fit-content; block-size: fit-content; margin: auto; padding: 4px 10px; border: 1px solid var(--vd-mat-card-border, var(--hdx-e-line)); border-radius: 999px; background: var(--vd-mat-card, var(--vd-theme-panel-bg)); color: var(--vd-text); font: 600 12px/1.2 var(--vd-font-ui, system-ui, sans-serif); pointer-events: none; }
+.hd-draft-badge .hd-ico { inline-size: 14px; block-size: 14px; }
+.hd-root[data-hd-mode="design"] .hd-page-empty { inset: 16px; border: 1.5px dashed color-mix(in srgb, var(--hd-ink) 22%, transparent); border-radius: var(--hd-radius-card, 14px); }
+.hd-page-empty-action { margin-block-start: 12px; min-block-size: 36px; padding: 0 16px; border: 1px solid var(--vd-mat-primary-border, transparent); border-radius: var(--vd-mat-btn-radius, 6px); background: var(--vd-mat-primary, var(--vd-accent)); color: var(--vd-mat-primary-text, var(--vd-accent-contrast)); font: 600 13px/1 var(--vd-font-ui, system-ui, sans-serif); cursor: pointer; }
+.hd-page-empty-action:hover { background: var(--vd-mat-primary-hover, var(--vd-mat-primary, var(--vd-accent))); }
+.hd-page-empty-action:focus-visible { outline: 2px solid var(--ds-color-border-focus, var(--vd-accent)); outline-offset: 2px; }
+/* Edit layer: ghost, guides, marquee, selection with chip and handles (12 px hit, 8 px visual; coarse 44 px, SE/E/S only) */
+.hd-edit-layer { position: absolute; inset: 0; z-index: 5; pointer-events: none; }
+.hd-edit-layer > :is(.hd-ghost, .hd-sel) { position: absolute; box-sizing: border-box; inset-inline-start: calc(var(--hd-offx, 0px) + var(--hd-gap, 12px) + var(--hd-x, 0) * (var(--hd-colw, 96px) + var(--hd-gap, 12px))); inset-block-start: calc(var(--hd-gap, 12px) + var(--hd-y, 0) * (var(--hd-rowh, 96px) + var(--hd-gap, 12px))); inline-size: calc(var(--hd-w, 1) * var(--hd-colw, 96px) + (var(--hd-w, 1) - 1) * var(--hd-gap, 12px)); block-size: calc(var(--hd-h, 1) * var(--hd-rowh, 96px) + (var(--hd-h, 1) - 1) * var(--hd-gap, 12px)); }
+.hd-ghost { border: 1px solid var(--hdx-e-sel); border-radius: var(--hd-radius-card, 14px); background: color-mix(in srgb, var(--hdx-e-sel) 12%, transparent); }
+.hd-ghost[data-hd-drop="invalid"] { border-style: dashed; border-color: var(--ds-color-danger-fg); background: color-mix(in srgb, var(--ds-color-danger-fg) 10%, transparent); }
+.hd-guide { position: absolute; background: var(--hdx-e-sel); }
+.hd-guide[data-axis="x"] { inset-block: 0; inline-size: 1px; }
+.hd-guide[data-axis="x"][data-side="start"] { inset-inline-start: calc(var(--hd-offx, 0px) + var(--hd-gap, 12px) + var(--hd-e-g, 0) * (var(--hd-colw, 96px) + var(--hd-gap, 12px))); }
+.hd-guide[data-axis="x"][data-side="end"] { inset-inline-start: calc(var(--hd-offx, 0px) + var(--hd-e-g, 0) * (var(--hd-colw, 96px) + var(--hd-gap, 12px)) - 1px); }
+.hd-guide[data-axis="y"] { inset-inline: 0; block-size: 1px; }
+.hd-guide[data-axis="y"][data-side="start"] { inset-block-start: calc(var(--hd-gap, 12px) + var(--hd-e-g, 0) * (var(--hd-rowh, 96px) + var(--hd-gap, 12px))); }
+.hd-guide[data-axis="y"][data-side="end"] { inset-block-start: calc(var(--hd-e-g, 0) * (var(--hd-rowh, 96px) + var(--hd-gap, 12px)) - 1px); }
+.hd-marquee { position: absolute; inset-inline-start: var(--hd-e-mx, 0px); inset-block-start: var(--hd-e-my, 0px); inline-size: var(--hd-e-mw, 0px); block-size: var(--hd-e-mh, 0px); border: 1px solid var(--hdx-e-sel); background: color-mix(in srgb, var(--hdx-e-sel) 8%, transparent); }
+.hd-root[data-hd-gesture="move"] .hd-sel { display: none; }
+.hd-sel-chip { position: absolute; inset-block-start: calc(100% + 14px); inset-inline-start: -4px; max-inline-size: 260px; padding: 0 8px; overflow: hidden; border: 1px solid var(--vd-mat-card-border, var(--hdx-e-line)); border-radius: 6px; background: var(--vd-mat-card, var(--vd-theme-panel-bg)); color: var(--vd-text); font: 600 12px/22px var(--vd-font-ui, system-ui, sans-serif); white-space: nowrap; text-overflow: ellipsis; }
+.hd-sel-chip:empty { display: none; }
+.hd-handle { position: absolute; inline-size: 12px; block-size: 12px; margin-block-start: -6px; margin-inline-start: -6px; pointer-events: auto; touch-action: none; }
+.hd-handle::before { content: ""; position: absolute; inset: 2px; box-sizing: border-box; border: 1px solid var(--hdx-e-sel); border-radius: 2px; background: var(--vd-mat-btn, var(--vd-theme-control-bg)); }
+.hd-handle[data-hd-limit]::before { background: var(--hdx-e-sel); }
+.hd-handle[data-edge="nw"] { inset-inline-start: -4px; inset-block-start: -4px; cursor: nwse-resize; } .hd-handle[data-edge="n"] { inset-inline-start: 50%; inset-block-start: -4px; cursor: ns-resize; }
+.hd-handle[data-edge="ne"] { inset-inline-start: calc(100% + 4px); inset-block-start: -4px; cursor: nesw-resize; } .hd-handle[data-edge="e"] { inset-inline-start: calc(100% + 4px); inset-block-start: 50%; cursor: ew-resize; }
+.hd-handle[data-edge="se"] { inset-inline-start: calc(100% + 4px); inset-block-start: calc(100% + 4px); cursor: nwse-resize; } .hd-handle[data-edge="s"] { inset-inline-start: 50%; inset-block-start: calc(100% + 4px); cursor: ns-resize; }
+.hd-handle[data-edge="sw"] { inset-inline-start: -4px; inset-block-start: calc(100% + 4px); cursor: nesw-resize; } .hd-handle[data-edge="w"] { inset-inline-start: -4px; inset-block-start: 50%; cursor: ew-resize; }
+.hd-drop-avatar { position: absolute; inset-block-start: 0; inset-inline-start: 0; z-index: 60; pointer-events: none; transform: translate3d(var(--hd-e-ax, 0px), var(--hd-e-ay, 0px), 0); }
+.hd-pal-avatar { display: inline-flex; align-items: center; gap: 8px; min-block-size: 36px; padding: 6px 12px 6px 10px; border: 1px solid var(--vd-mat-card-border, var(--hdx-e-line)); border-radius: 8px; background: var(--vd-mat-card, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-card-shadow, none); color: var(--vd-text); font: 13px/1.25 var(--vd-font-ui, system-ui, sans-serif); white-space: nowrap; }
+.hd-pal-avatar-text { display: grid; } .hd-pal-avatar small { color: var(--vd-mat-label-color, var(--vd-theme-muted)); font-size: 12px; }
+.hd-drop-avatar[data-hd-drop="rebind"] .hd-pal-avatar { border-color: var(--hdx-e-sel); } .hd-drop-avatar[data-hd-drop="invalid"] .hd-pal-avatar { border-color: var(--ds-color-danger-fg); } .hd-drop-avatar[data-hd-drop="out"] { opacity: .72; }
+.hd-toasts { position: absolute; inset-block-end: 16px; inset-inline: 0; z-index: 50; display: grid; justify-items: center; gap: 8px; pointer-events: none; }
+.hd-toast { max-inline-size: min(480px, calc(100% - 32px)); padding: 8px 14px; border: 1px solid var(--vd-mat-card-border, var(--hdx-e-line)); border-radius: 8px; background: var(--vd-mat-card, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-card-shadow, none); color: var(--vd-text); font: 13px/1.4 var(--vd-font-ui, system-ui, sans-serif); }
+.hd-toast[data-tone="warn"] { border-color: color-mix(in srgb, var(--ds-color-warning-fg) 55%, var(--hdx-e-line)); } .hd-toast[data-tone="danger"] { border-color: color-mix(in srgb, var(--ds-color-danger-fg) 55%, var(--hdx-e-line)); }
+/* Form controls (E.form) */
+.hd-f { display: grid; gap: 6px; min-inline-size: 0; }
+.hd-f[data-hd-inline] { grid-template-columns: minmax(0, 1fr) auto; align-items: center; column-gap: 12px; } .hd-f[data-hd-inline] > :is(.hd-f-help, .hd-f-error) { grid-column: 1 / -1; }
+.hd-f-label { display: flex; align-items: baseline; gap: 6px; color: var(--vd-text); font-size: 12px; font-weight: 600; }
+.hd-f-req, .hd-f-help, .hd-f-sec-title, .hd-f-out { color: var(--hdx-e-muted); font-size: 12px; } .hd-f-req { font-weight: 500; } .hd-f-help { margin: 0; line-height: 1.45; }
+.hd-f-error { margin: 0; color: var(--vd-text); font-size: 12px; } .hd-f-error::before { content: ""; display: inline-block; inline-size: 6px; block-size: 6px; margin-inline-end: 6px; border-radius: 50%; background: var(--ds-color-danger-fg); vertical-align: middle; }
+.hd-root :is(.hd-f-input, .hd-f-select) { box-sizing: border-box; inline-size: 100%; min-block-size: 32px; margin: 0; padding: 0 10px; border: 1px solid var(--vd-mat-field-border, var(--hdx-e-line)); border-radius: var(--hdx-e-fr); background: var(--vd-mat-field, var(--vd-theme-control-bg)); box-shadow: var(--vd-mat-field-shadow, none); color: var(--vd-text); font: 13px/1.2 var(--vd-font-ui, system-ui, sans-serif); }
+.hd-root :is(.hd-f-input, .hd-f-select):focus-visible { outline-offset: 0; border-color: var(--hdx-e-ring); }
+/* Q17: field and segmented borders are drawn from --vd-text mixed into the field surface (60 %), so they hold 3:1 on every material; the gate probe measures them */
+.hd-root :is(.hd-f-input, .hd-f-select, .hd-f-seg, .hd-ep-box, .hd-f-swatch) { border-color: color-mix(in srgb, var(--vd-text) 60%, var(--vd-mat-field-color, var(--vd-theme-control-bg))); }
+.hd-f-input::placeholder { color: var(--hdx-e-muted); opacity: 1; } .hd-f-input[aria-invalid="true"] { border-color: var(--ds-color-danger-fg); } .hd-f-input::-webkit-search-cancel-button { appearance: none; }
+.hd-f-selectwrap, .hd-f-num { position: relative; display: flex; } .hd-root .hd-f-select { appearance: none; padding-inline-end: 30px; cursor: pointer; }
+.hd-f-selectwrap > .hd-ico { position: absolute; inset-inline-end: 9px; inset-block-start: 50%; margin-block-start: -8px; color: var(--hdx-e-muted); pointer-events: none; }
+.hd-f-num > .hd-f-input { padding-inline-end: 34px; } .hd-f-unit { position: absolute; inset-inline-end: 10px; inset-block: 0; display: grid; place-items: center; color: var(--hdx-e-muted); font-size: 12px; pointer-events: none; }
+.hd-f-switch { position: relative; display: inline-grid; place-items: center; inline-size: 40px; block-size: 32px; margin: 0; padding: 0; border: 0; background: transparent; cursor: pointer; }
+.hd-f-switch-track { position: relative; box-sizing: border-box; inline-size: 34px; block-size: 20px; border: 1.5px solid var(--hdx-e-muted); border-radius: 10px; }
+.hd-f-switch-thumb { position: absolute; inset-block-start: 2.5px; inset-inline-start: 2.5px; inline-size: 12px; block-size: 12px; border-radius: 50%; background: var(--hdx-e-muted); }
+.hd-root .hd-f-switch:hover:not(:is(:disabled, [aria-disabled="true"])) .hd-f-switch-track { border-color: var(--vd-text); }
+.hd-root .hd-f-switch:is(:disabled, [aria-disabled="true"]), .hd-root :is(.hd-f-input, .hd-f-select, .hd-f-range):is(:disabled, [aria-disabled="true"]) { opacity: .45; cursor: default; }
+.hd-f-switch[aria-checked="true"] .hd-f-switch-track { border-color: var(--vd-text); background: var(--vd-text); } .hd-f-switch[aria-checked="true"] .hd-f-switch-thumb { background: var(--vd-mat-field-color, var(--vd-theme-control-bg)); transform: translateX(14px); }
+.hd-f-seg { display: flex; gap: 2px; padding: 2px; border: 1px solid var(--vd-mat-field-border, var(--hdx-e-line)); border-radius: var(--hdx-e-fr); background: var(--vd-mat-field, var(--vd-theme-control-bg)); }
+.hd-root .hd-f-seg-opt { display: inline-flex; flex: 1 1 0; align-items: center; justify-content: center; gap: 6px; min-inline-size: 0; min-block-size: 26px; padding: 0 8px; border: 1px solid transparent; border-radius: var(--hdx-e-fr); background: transparent; color: var(--vd-text); font: 500 12.5px/1 var(--vd-font-ui, system-ui, sans-serif); cursor: pointer; }
+.hd-f-seg-opt > span { overflow: hidden; white-space: nowrap; text-overflow: ellipsis; } .hd-root .hd-f-seg-opt:hover:not([aria-checked="true"]) { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
+.hd-root .hd-f-seg-opt[aria-checked="true"] { border-color: var(--vd-mat-btn-border, var(--hdx-e-line)); background: var(--vd-mat-btn, var(--vd-theme-control-bg)); box-shadow: var(--vd-mat-btn-shadow, none); color: var(--vd-mat-btn-text, var(--vd-text)); font-weight: 600; }
+.hd-f-colour { display: grid; grid-template-columns: 32px minmax(0, 1fr) auto; align-items: center; gap: 8px; }
+.hd-f-swatch { box-sizing: border-box; inline-size: 32px; block-size: 32px; margin: 0; padding: 3px; border: 1px solid var(--vd-mat-field-border, var(--hdx-e-line)); border-radius: var(--hdx-e-fr); background: var(--vd-mat-field, var(--vd-theme-control-bg)); cursor: pointer; }
+.hd-f-swatch::-webkit-color-swatch-wrapper { padding: 0; } .hd-f-swatch::-webkit-color-swatch { border: 0; border-radius: 3px; } .hd-f-colour[data-hd-empty] .hd-f-swatch { opacity: .45; }
+.hd-root .hd-f-hex { font-family: var(--vd-mat-mono-font, ui-monospace, monospace); }
+.hd-f-rangewrap { position: relative; display: grid; grid-template-columns: minmax(0, 1fr) 6ch; align-items: center; gap: 10px; } .hd-f-out { text-align: end; font-variant-numeric: tabular-nums; }
+.hd-f-range:hover:not(:is(:disabled, [aria-disabled="true"]))::-webkit-slider-thumb { border-width: 2px; }
+.hd-f-range { appearance: none; inline-size: 100%; block-size: 24px; margin: 0; background: transparent; cursor: pointer; }
+.hd-f-range::-webkit-slider-runnable-track { block-size: 4px; border-radius: 2px; background: color-mix(in srgb, var(--vd-text) 26%, transparent); }
+.hd-f-range::-webkit-slider-thumb { appearance: none; inline-size: 16px; block-size: 16px; margin-block-start: -6px; box-sizing: border-box; border: 1.5px solid var(--vd-text); border-radius: 50%; background: var(--vd-mat-field-color, var(--vd-theme-control-bg)); }
+.hd-f-range::-moz-range-track { block-size: 4px; border-radius: 2px; background: color-mix(in srgb, var(--vd-text) 26%, transparent); }
+.hd-f-range::-moz-range-thumb { inline-size: 14px; block-size: 14px; border: 1.5px solid var(--vd-text); border-radius: 50%; background: var(--vd-mat-field-color, var(--vd-theme-control-bg)); }
+.hd-f-rangewrap:has(.hd-look-hue) .hd-f-range::-webkit-slider-runnable-track { background: transparent; }
+.hd-f-tabs { display: flex; flex: none; gap: 4px; padding: 0 12px; border-block-end: 1px solid var(--hdx-e-line); }
+.hd-root .hd-f-tab { position: relative; min-block-size: 36px; padding: 0 8px; border: 0; background: transparent; color: var(--hdx-e-muted); font: 500 13px/1 var(--vd-font-ui, system-ui, sans-serif); cursor: pointer; }
+.hd-root .hd-f-tab:hover, .hd-root .hd-f-tab[aria-selected="true"] { color: var(--vd-text); } .hd-root .hd-f-tab[aria-selected="true"] { font-weight: 600; }
+.hd-root .hd-f-tab[aria-selected="true"]::after { content: ""; position: absolute; inset-inline: 8px; inset-block-end: -1px; block-size: 2px; border-radius: 1px; background: var(--vd-text); }
+.hd-root .hd-btn { border-color: var(--vd-mat-btn-border, var(--hdx-e-line)); background: var(--vd-mat-btn, var(--vd-theme-control-bg)); box-shadow: var(--vd-mat-btn-shadow, none); color: var(--vd-mat-btn-text, var(--vd-text)); }
+.hd-root .hd-btn:hover:not([aria-disabled="true"], :disabled) { background: var(--vd-mat-btn-hover, var(--vd-theme-control-hover)); }
+.hd-root .hd-btn:is([data-tone="quiet"], [data-tone="danger-quiet"]) { border-color: transparent; background: transparent; box-shadow: none; }
+.hd-root .hd-btn:is([data-tone="quiet"], [data-tone="danger-quiet"]):hover:not([aria-disabled="true"], :disabled) { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
+.hd-root .hd-btn[data-tone="danger-quiet"] { color: color-mix(in srgb, var(--ds-color-danger-fg) 72%, var(--vd-text)); } .hd-root .hd-btn[data-hd-icon-only] { inline-size: 32px; padding: 0; }
+.hd-f-sec { display: grid; gap: 10px; } .hd-f-sec-title { margin: 0; font-weight: 600; } .hd-f-row { display: flex; flex-wrap: wrap; gap: 8px; }
+.hd-icongrid { display: grid; gap: 8px; margin-block-start: 8px; } .hd-icongrid-list { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 4px; max-block-size: 208px; overflow: auto; }
+.hd-root .hd-icongrid-opt { display: grid; place-items: center; aspect-ratio: 1; min-block-size: 36px; padding: 0; border: 1px solid transparent; border-radius: 6px; background: transparent; color: var(--vd-text); cursor: pointer; }
+.hd-root .hd-icongrid-opt:hover { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); } .hd-root .hd-icongrid-opt[aria-pressed="true"] { border-color: var(--vd-text); background: var(--vd-mat-selected, var(--vd-theme-accent-soft)); color: var(--vd-mat-selected-text, var(--vd-text)); }
+.hd-root .hd-icongrid-opt .hd-ico { inline-size: 20px; block-size: 20px; }
+/* Palette */
+.hd-pal, .hd-insp { display: flex; flex: 1 1 auto; flex-direction: column; min-block-size: 0; }
+.hd-pal-head { display: grid; gap: 12px; padding-block-end: 12px; border-block-end: 1px solid var(--hdx-e-line); }
+.hd-pal-search { position: relative; margin-inline: 12px; } .hd-pal-search > .hd-ico { position: absolute; inset-inline-start: 10px; inset-block-start: 50%; margin-block-start: -8px; color: var(--hdx-e-muted); pointer-events: none; }
+.hd-root .hd-pal-search > .hd-f-input { padding-inline: 32px 34px; } .hd-pal-search > .hd-btn { position: absolute; inset-block-start: 0; inset-inline-end: 0; }
+.hd-pal-panel { flex: 1 1 auto; min-block-size: 0; overflow: auto; overscroll-behavior: contain; padding: 4px 12px 16px; scrollbar-gutter: stable; }
+.hd-pal-group { margin-block-start: 8px; }
+.hd-pal-group-title { position: sticky; inset-block-start: -4px; z-index: 1; margin: 0 -12px 8px; padding: 6px 12px; background: var(--vd-mat-side, var(--vd-theme-chrome-bg)); color: var(--hdx-e-muted); font: 600 12px/1.2 var(--vd-font-ui, system-ui, sans-serif); }
+.hd-pal-grid, .hd-insp-variants, .hd-look-families { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
+.hd-pal-card, .hd-insp-variant, .hd-look-family { position: relative; display: grid; gap: 6px; padding: 4px 4px 8px; border: 1px solid var(--vd-mat-card-border, var(--hdx-e-line)); border-radius: 8px; background: var(--vd-mat-card-color, var(--vd-theme-panel-bg)); }
+.hd-pal-card { touch-action: pan-y; cursor: grab; } .hd-pal-card:hover { border-color: color-mix(in srgb, var(--vd-text) 32%, var(--hdx-e-line)); }
+.hd-pal-card:has(.hd-pal-hit:focus-visible), .hd-insp-variant:has(.hd-insp-vhit:focus-visible), .hd-look-family:has(.hd-look-fhit:focus-visible) { outline: 2px solid var(--hdx-e-ring); outline-offset: 2px; }
+.hd-root :is(.hd-pal-card .hd-pal-hit, .hd-insp-variant .hd-insp-vhit, .hd-look-family .hd-look-fhit):focus-visible { outline: none; }
+.hd-pal-thumb, .hd-insp-thumb { position: relative; display: block; block-size: 76px; overflow: hidden; border-radius: 5px; background: var(--hd-canvas); pointer-events: none; } .hd-insp-thumb { block-size: 80px; }
+.hd-thumb { position: relative; inline-size: 100%; block-size: 100%; overflow: hidden; }
+.hd-thumb > .hd-widget { position: absolute; inset-inline-start: 50%; inset-block-start: 50%; box-sizing: border-box; inline-size: var(--hd-e-tw); block-size: var(--hd-e-th); transform: translate(-50%, -50%) scale(var(--hd-e-ts, 1)); pointer-events: none; }
+.hd-pal-hit, .hd-insp-vhit, .hd-look-fhit { display: grid; gap: 2px; padding: 0 4px; border: 0; background: transparent; color: var(--vd-text); text-align: start; cursor: inherit; }
+.hd-pal-hit::before, .hd-insp-vhit::before, .hd-look-fhit::before { content: ""; position: absolute; inset: 0; border-radius: 8px; }
+.hd-pal-name, .hd-pal-variant, .hd-ent-name, .hd-ent-meta, .hd-ep-opt-name, .hd-ep-opt-meta, .hd-ep-meta, .hd-insp-title, .hd-insp-sub { overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
+.hd-pal-name { font-size: 12.5px; font-weight: 600; } .hd-pal-variant, .hd-look-fhit small { color: var(--vd-mat-label-color, var(--vd-theme-muted)); font-size: 12px; }
+.hd-pal-status { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-block-size: 28px; margin: 0; color: var(--hdx-e-muted); font-size: 12px; }
+.hd-pal-filters { display: grid; grid-template-columns: repeat(auto-fit, minmax(110px, 1fr)); gap: 6px; margin-block: 8px 4px; } .hd-root .hd-pal-filters .hd-f-select { min-block-size: 28px; padding-inline: 8px 26px; font-size: 12px; }
+.hd-ent-list, .hd-ep-list { display: grid; gap: 2px; margin: 0; padding: 0; list-style: none; }
+.hd-root .hd-ent-row { display: grid; grid-template-columns: 28px minmax(0, 1fr) auto; align-items: center; gap: 8px; inline-size: 100%; min-block-size: 44px; padding: 4px 8px; border: 1px solid transparent; border-radius: 6px; background: transparent; color: var(--vd-text); text-align: start; touch-action: pan-y; cursor: grab; }
+.hd-root .hd-ent-row:hover { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
+.hd-ent-icon, .hd-insp-icon { display: grid; place-items: center; inline-size: 28px; block-size: 28px; border-radius: 6px; background: color-mix(in srgb, var(--vd-text) 7%, transparent); }
+.hd-ent-text { display: grid; min-inline-size: 0; } .hd-ent-name { font-size: 13px; font-weight: 500; } .hd-ent-meta { color: var(--hdx-e-muted); font-size: 12px; }
+.hd-ent-state { max-inline-size: 9ch; overflow: hidden; color: var(--hdx-e-muted); font-size: 12px; font-variant-numeric: tabular-nums; white-space: nowrap; text-overflow: ellipsis; } .hd-ent-moreitem { display: grid; padding-block: 8px; }
+/* Inspector and device picker */
+.hd-insp-head { display: grid; grid-template-columns: 32px minmax(0, 1fr); align-items: center; gap: 10px; padding: 12px 16px; } .hd-insp-icon { inline-size: 32px; block-size: 32px; border-radius: 8px; }
+.hd-insp-heading { min-inline-size: 0; } .hd-insp-title { margin: 0; color: var(--vd-text); font: 600 14px/1.3 var(--vd-mat-heading-font, var(--vd-font-ui, system-ui, sans-serif)); } .hd-insp-sub { margin: 2px 0 0; color: var(--hdx-e-muted); font-size: 12px; }
+.hd-insp-body { display: grid; flex: 1 1 auto; align-content: start; gap: 16px; min-block-size: 0; overflow: auto; overscroll-behavior: contain; padding: 16px; scrollbar-gutter: stable; }
+.hd-insp-foot { display: flex; padding: 8px 12px; border-block-start: 1px solid var(--hdx-e-line); } .hd-insp-foot:empty { display: none; }
+.hd-insp-note { margin: 0; padding: 8px 10px; border-radius: 6px; background: color-mix(in srgb, var(--vd-text) 6%, transparent); color: var(--vd-text); font-size: 12px; line-height: 1.45; }
+.hd-insp-note[data-tone="warn"] { box-shadow: inset 3px 0 0 var(--ds-color-warning-fg); }
+.hd-root :is(.hd-insp-variant, .hd-look-family, .hd-pal-card):hover { border-color: color-mix(in srgb, var(--vd-text) 32%, var(--hdx-e-line)); }
+.hd-ep-toggle:hover { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
+.hd-insp-variant:has([aria-checked="true"]), .hd-look-family:has([aria-checked="true"]) { border-color: var(--vd-text); box-shadow: 0 0 0 1px var(--vd-text); }
+.hd-insp-vhit { font-size: 12.5px; font-weight: 500; cursor: pointer; } .hd-root .hd-insp-iconbtn { justify-content: flex-start; inline-size: 100%; } .hd-insp-iconbtn > span { flex: 1 1 auto; text-align: start; }
+.hd-ep { display: grid; gap: 6px; }
+.hd-ep-box { position: relative; display: grid; grid-template-columns: 28px minmax(0, 1fr) auto 28px; align-items: center; min-block-size: 32px; border: 1px solid var(--vd-mat-field-border, var(--hdx-e-line)); border-radius: var(--hdx-e-fr); background: var(--vd-mat-field, var(--vd-theme-control-bg)); }
+.hd-ep[data-hd-state="empty"] .hd-ep-box { border-style: dashed; } .hd-ep-box:focus-within { border-color: var(--hdx-e-ring); box-shadow: 0 0 0 1px var(--hdx-e-ring); }
+.hd-root .hd-ep-input { min-block-size: 30px; padding: 0 4px; border: 0; background: transparent; box-shadow: none; } .hd-root .hd-ep-input:focus-visible { outline: none; }
+.hd-ep-icon, .hd-ep-toggle { display: grid; place-items: center; color: var(--hdx-e-muted); } .hd-ep-toggle { inline-size: 28px; block-size: 30px; padding: 0; border: 0; background: transparent; cursor: pointer; } .hd-root .hd-ep-box .hd-btn { inline-size: 28px; min-block-size: 28px; }
+.hd-ep-meta { margin: 0; color: var(--hdx-e-muted); font-size: 12px; } .hd-ep-warn { display: flex; gap: 6px; margin: 0; color: var(--vd-text); font-size: 12px; } .hd-ep-warn > .hd-ico { color: color-mix(in srgb, var(--ds-color-warning-fg) 80%, var(--vd-text)); }
+.hd-ep-pop { position: absolute; inset-inline: -1px; inset-block-start: calc(100% + 4px); z-index: 8; display: grid; gap: 4px; padding: 6px; border: 1px solid var(--vd-mat-card-border, var(--hdx-e-line)); border-radius: 8px; background: var(--vd-mat-card, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-card-shadow, none); }
+.hd-ep-filters { display: grid; grid-template-columns: repeat(auto-fit, minmax(90px, 1fr)); gap: 4px; } .hd-ep-list { max-block-size: 264px; overflow: auto; }
+.hd-ep-opt { display: grid; grid-template-columns: minmax(0, 1fr) auto; column-gap: 8px; align-content: center; min-block-size: 40px; padding: 4px 8px; border-radius: 6px; color: var(--vd-text); cursor: pointer; }
+.hd-ep-opt-meta { grid-row: 2; color: var(--vd-mat-label-color, var(--vd-theme-muted)); font-size: 12px; } .hd-ep-opt-state { grid-row: 1 / 3; grid-column: 2; align-self: center; color: var(--vd-mat-label-color, var(--vd-theme-muted)); font-size: 12px; font-variant-numeric: tabular-nums; }
+.hd-ep-opt:hover, .hd-ep-opt[data-hd-active] { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); } .hd-ep-opt[aria-selected="true"] { background: var(--vd-mat-selected, var(--vd-theme-accent-soft)); color: var(--vd-mat-selected-text, var(--vd-text)); }
+.hd-ep-status { margin: 0; padding: 4px 8px; color: var(--vd-mat-label-color, var(--vd-theme-muted)); font-size: 12px; } .hd-ep-status:empty { display: none; }
+/* Look editor and dialogs */
+.hd-look, .hd-dlg { display: grid; gap: 20px; } .hd-look-custom { display: grid; gap: 12px; } .hd-look-family { padding: 8px; border-radius: 10px; }
+.hd-look-preview { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); grid-template-rows: repeat(2, minmax(0, 1fr)); gap: 8px; block-size: 88px; overflow: hidden; border-radius: 6px; background: var(--hd-canvas); pointer-events: none; }
+.hd-look-slot { position: relative; overflow: hidden; } .hd-look-slot[data-slot="a"] { grid-row: 1 / 3; } .hd-look-fhit { cursor: pointer; } .hd-look-fhit strong { font-size: 13px; }
+.hd-look-hue { position: absolute; inset-inline: 0 calc(6ch + 10px); inset-block-start: 50%; block-size: 8px; margin-block-start: -4px; border-radius: 4px; pointer-events: none; }
+.hd-look-swatches { display: flex; gap: 6px; } .hd-look-swatch { inline-size: 28px; block-size: 20px; border-radius: 4px; background: var(--hd-e-sw); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--vd-text) 18%, transparent); }
+.hd-newcards { display: grid; gap: 8px; }
+.hd-newcard { display: grid; grid-template-columns: 36px minmax(0, 1fr); align-items: center; gap: 12px; min-block-size: 64px; padding: 12px 14px; border: 1px solid var(--vd-mat-card-border, var(--hdx-e-line)); border-radius: 10px; background: transparent; color: var(--vd-text); text-align: start; cursor: pointer; }
+.hd-newcard:hover { border-color: color-mix(in srgb, var(--vd-text) 40%, var(--hdx-e-line)); background: var(--vd-mat-hover, var(--vd-theme-control-hover)); } .hd-newcard strong { display: block; font-size: 14px; font-weight: 650; } .hd-newcard small { display: block; color: var(--vd-mat-label-color, var(--vd-theme-muted)); font-size: 12.5px; }
+.hd-newcard-icon { display: grid; place-items: center; inline-size: 36px; block-size: 36px; border-radius: 9px; background: color-mix(in srgb, var(--vd-text) 8%, transparent); }
+.hd-dlg-issues { display: grid; gap: 6px; margin: 0; padding-inline-start: 18px; } .hd-dlg-issues code { margin-inline-start: 6px; color: var(--vd-mat-label-color, var(--vd-theme-muted)); font: 12px var(--vd-mat-mono-font, ui-monospace, monospace); overflow-wrap: anywhere; }
+@media (pointer: coarse) {
+  .hd-handle { inline-size: 44px; block-size: 44px; margin-block-start: -22px; margin-inline-start: -22px; } .hd-handle::before { inset: 16px; } .hd-handle:not([data-edge="se"], [data-edge="e"], [data-edge="s"]) { display: none; } .hd-sel-chip { inset-block-start: calc(100% + 28px); }
+  .hd-root :is(.hd-eb-btn, .hd-eb-more, .hd-eb-drawer, .hd-ptab, .hd-ptab-add, .hd-bp-reset, .hd-btn, .hd-f-tab, .hd-f-seg-opt, .hd-ep-opt, .hd-icongrid-opt, .hd-f-input, .hd-f-select) { min-block-size: 44px; }
+  .hd-root :is(.hd-eb-btn, .hd-eb-more, .hd-eb-drawer, .hd-ptab-add, .hd-bp-reset):not(:has(.hd-eb-label, .hd-ptab-label)), .hd-root .hd-btn[data-hd-icon-only] { inline-size: 44px; } .hd-root .hd-bp-opt { inline-size: 44px; block-size: 40px; } .hd-f-switch { block-size: 44px; } }
+@media (prefers-reduced-motion: no-preference) {
+  body:not([data-animations="false"]) .hd-root[data-hd-mode="design"]:not([data-hd-motion="reduced"]) .hd-page > .hd-widget { transition: transform 140ms cubic-bezier(.2, 0, 0, 1); }
+  body:not([data-animations="false"]) .hd-root[data-hd-mode="design"] .hd-page > .hd-widget[data-hd-flip="from"], body:not([data-animations="false"]) .hd-root[data-hd-mode="design"] .hd-page > .hd-widget[data-hd-dragging] { transition: none; }
+  body:not([data-animations="false"]) .hd-root:not([data-hd-motion="reduced"]) .hd-f-switch-thumb { transition: transform 120ms ease; }
+  body:not([data-animations="false"]) .hd-root[data-hd-drawers="true"]:not([data-hd-motion="reduced"]) > .hd-palette[data-hd-drawer="open"] { animation: hdx-drawer-start 160ms cubic-bezier(.2, 0, 0, 1); }
+  body:not([data-animations="false"]) .hd-root[data-hd-drawers="true"]:not([data-hd-motion="reduced"]) > .hd-inspector[data-hd-drawer="open"] { animation: hdx-drawer-end 160ms cubic-bezier(.2, 0, 0, 1); }
+  body:not([data-animations="false"]) .hd-root:not([data-hd-motion="reduced"]) > .hd-toasts > .hd-toast { animation: hdx-rise 160ms ease-out; } }
+@keyframes hdx-drawer-start { from { opacity: 0; transform: translateX(-12px); } }
+@keyframes hdx-drawer-end { from { opacity: 0; transform: translateX(12px); } }
+@keyframes hdx-rise { from { opacity: 0; transform: translateY(6px); } }
 @media (prefers-reduced-motion: no-preference) {
```
- [ ] Q3 condition (dry-run result: no `--hd-e-*` is declared anywhere in `ui/css`, `host.test.mjs` PASS, the token-freeze comparison PASS):
  - Run `npm run test:homedash` (guard: lintCss on every bundle part, the m1 token-freeze comparison against `token-freeze.json`, host.test on this file).
  - Run `go test ./ui -run '(?i)^(TestDesktopAppMaterialsAdoptedByApps|TestHomeDashWiring)$' -count=1` in the worktree (these read `//go:embed` content: `-overlay` cannot test them; the Node run proves the same facts: the file reads `--vd-mat-*`, the loader names match the wiring regexp).
  - Run `rg -n -- "--hd-e-[a-z]+\s*:" ui/css`: it must find nothing. `--hd-e-*` is only read with `var()`, never declared.
  - If any guard or freeze test refuses `--hd-e-*` → stop and report. Do not regenerate the freeze.
- [ ] Q17: the field-border rule uses `--vd-text` mixed into `--vd-mat-field-color`; the gate probe (E2) measures `.hd-f-input`, `.hd-f-select`, `.hd-ep-box`, `.hd-f-seg` and `.hd-f-swatch` borders at 3:1 against their own surface in all 16 variants. The browser probe was not run in the dry run: the 60 % mix is computed to give about 3.9:1 (light) and 5:1 (dark) for a 6:1 text/surface pair and is checked by the gate.
- [ ] Then `npm run check:ui`, `TestDesktopHomeDashBrowser` and `TestDesktopHomeDashControlsBrowser` (R8, view mode unchanged). Commit — `feat(homedash): designer chrome styling (V14)`.

### Task E2: Shared browser helpers, probes, gate runner
**Files:** create `ui/desktop_homedash_designer_helpers_test.go`, `ui/desktop_homedash_designer_gates_test.go`. **Existing symbols changed:** none. Dry run: both files (with the F1 skeleton and part 1's fixture overlay) pass `go vet -overlay … ./ui` clean; no browser test was run.
- [ ] Helpers file (in full; uses `golang.org/x/image`, already a direct dependency of `internal/media`):
<!-- embed: dryrun-m4/overlay/ui/desktop_homedash_designer_helpers_test.go -->
```go
package ui

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// hdDesigner drives one HomeDash window in the desktop shell against the M4 fixture (contract C18); shared by the designer and gate tests.
type hdDesigner struct {
	t       *testing.T
	page    *rod.Page
	fixture *homeDashFixture
	words   map[string]string
}

// hdDesignerSetupJS installs the words, an error log, an export capture (blob text of every application/json object URL) and a no-op
// click for download anchors, so exports never touch the disk.
const hdDesignerSetupJS = `(w)=>{Object.assign(window.fixtureWords,JSON.parse(w));window.hdErrs=[];
	addEventListener('error',e=>hdErrs.push(String(e.message)));addEventListener('unhandledrejection',e=>hdErrs.push(String(e.reason&&e.reason.message||e.reason)));
	window.__exports=[];const make=URL.createObjectURL.bind(URL);URL.createObjectURL=b=>{if(b&&b.type==='application/json')b.text().then(x=>__exports.push(x));return make(b);};
	const click=HTMLElement.prototype.click;HTMLAnchorElement.prototype.click=function(){if(this.hasAttribute('download'))return;click.call(this);};}`

func newHDDesigner(t *testing.T, lang string) *hdDesigner {
	t.Helper()
	fixture := newHomeDashFixture(t)
	fixture.enableM3(t)
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/homedash/": fixture}).CancelTimeout().Timeout(40 * time.Minute)
	t.Cleanup(fixture.shutdown)
	raw := readDesktopAssetText(t, "lang/homedash/"+lang+".json")
	words := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &words); err != nil {
		t.Fatal(err)
	}
	page.MustEval(hdDesignerSetupJS, raw)
	page.MustEval(`(l)=>{document.documentElement.lang=l;}`, lang)
	return &hdDesigner{t: t, page: page, fixture: fixture, words: words}
}

func (h *hdDesigner) str(js string, args ...any) string   { return h.page.MustEval(js, args...).Str() }
func (h *hdDesigner) yes(js string, args ...any) bool     { return h.page.MustEval(js, args...).Bool() }
func (h *hdDesigner) num(js string, args ...any) float64  { return h.page.MustEval(js, args...).Num() }
func (h *hdDesigner) word(key string) string               { return h.words[key] }
func (h *hdDesigner) fill(key string, vars map[string]string) string {
	s := h.words[key]
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}
func (h *hdDesigner) diag() string {
	return h.str(`()=>JSON.stringify({errs:window.hdErrs,mode:(document.querySelector('.hd-root')||{dataset:{}}).dataset.hdMode,save:(document.querySelector('.hd-save')||{dataset:{}}).dataset.phase,
		live:(document.querySelector('.hd-root .hd-live')||{}).textContent,sel:[...document.querySelectorAll('.hd-widget[aria-selected="true"]')].map(x=>x.dataset.hdWid),
		sheet:(document.querySelector('.hd-root > .hd-sheet-layer:not([hidden]) .hd-sheet-title')||{}).textContent})`)
}
func (h *hdDesigner) until(within time.Duration, what, js string, args ...any) {
	h.t.Helper()
	deadline := time.Now().Add(within)
	for !h.page.MustEval(js, args...).Bool() {
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out after %v waiting for %s\n%s", within, what, h.diag())
		}
		time.Sleep(20 * time.Millisecond)
	}
}
func hdFrame(id string) string { return `.hd-root[data-hd-mode="design"] .hd-page > [data-hd-wid="` + id + `"]` }
func (h *hdDesigner) center(sel string) (float64, float64) {
	h.t.Helper()
	r := h.page.MustEval(`(s)=>{const e=document.querySelector(s);if(!e)return null;e.scrollIntoView({block:'nearest',inline:'nearest'});const r=e.getBoundingClientRect();return [r.left+r.width/2,r.top+r.height/2];}`, sel)
	if r.Nil() {
		h.t.Fatalf("no element %s\n%s", sel, h.diag())
	}
	return r.Arr()[0].Num(), r.Arr()[1].Num()
}
// drag presses at (x0,y0), moves in `steps` frames to (x1,y1), runs hold (a screenshot or a check) and releases unless hold returns false.
func (h *hdDesigner) drag(x0, y0, x1, y1 float64, steps int, hold func() bool) {
	m := h.page.Mouse
	m.MustMoveTo(x0, y0)
	m.MustDown(proto.InputMouseButtonLeft)
	for i := 1; i <= steps; i++ {
		f := float64(i) / float64(steps)
		m.MustMoveTo(x0+(x1-x0)*f, y0+(y1-y0)*f)
		time.Sleep(16 * time.Millisecond)
	}
	if hold == nil || hold() {
		m.MustUp(proto.InputMouseButtonLeft)
	}
}
func (h *hdDesigner) openDashboard(id string) {
	h.page.MustEval(`(id)=>auditTest.openApp('homedash',{dashboard_id:id})`, id)
	h.until(20*time.Second, "dashboard "+id, `()=>!!document.querySelector('.hdx-view:not([hidden]) .hd-page')`)
}
func (h *hdDesigner) enterEdit() {
	h.page.MustEval(`()=>document.querySelector('.hdx-edit').click()`)
	h.until(10*time.Second, "design mode", `()=>!!document.querySelector('.hd-root[data-hd-mode="design"] > .hd-editbar')&&!!document.querySelector('.hd-pal-card')`)
}
// closeSheet presses the named action button of the open engine sheet (E.sheet confirm buttons or form buttons).
func (h *hdDesigner) press(action string) {
	h.t.Helper()
	h.page.MustEval(`(a)=>{const b=document.querySelector('.hd-root .hd-sheet-layer:not([hidden]) [data-action="'+a+'"]');if(!b)throw new Error("no sheet button "+a);b.click();}`, action)
}
func (h *hdDesigner) waitPuts(after int, what string) homeDashPut {
	h.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if log := h.fixture.putLog(); len(log) > after {
			return log[len(log)-1]
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("no PUT for %s\n%s", what, h.diag())
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// hdThemeVariant is one of the 16 desktop theme variants (14 themes plus the dark schemes of Fruity and Minimal).
type hdThemeVariant struct{ Name, Theme, Setting, Mode, Scheme string }

func hdThemeVariants(t *testing.T) []hdThemeVariant {
	t.Helper()
	var out []hdThemeVariant
	for _, th := range parseDesktopThemeRegistry(t) {
		if th.SchemeSetting != "" {
			for _, m := range []string{"light", "dark"} {
				out = append(out, hdThemeVariant{th.ID + "-" + m, th.ID, th.SchemeSetting, m, m})
			}
			continue
		}
		out = append(out, hdThemeVariant{th.ID, th.ID, "", "", th.Scheme})
	}
	if len(out) != 16 {
		t.Fatalf("%d theme variants, want 16", len(out))
	}
	return out
}
func (h *hdDesigner) theme(v hdThemeVariant) {
	h.page.MustEval(`(theme,setting,mode,scheme)=>{const s=auditTest.state.bootstrap.settings;s['appearance.theme']=theme;if(setting)s[setting]=mode;document.documentElement.dataset.theme=scheme;auditTest.applyDesktopSettings();}`, v.Theme, v.Setting, v.Mode, v.Scheme)
	h.page.MustWait(`(theme,scheme)=>document.body.dataset.theme===theme&&document.body.dataset.scheme===scheme`, v.Theme, v.Scheme)
	h.page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>r(true))))`)
}
func (h *hdDesigner) capture(dir, name string) string {
	h.t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.t.Fatal(err)
	}
	path := filepath.Join(dir, name+".png")
	if err := os.WriteFile(path, h.page.MustElement(`.vd-window[data-app-id="homedash"]`).MustScreenshot(), 0o644); err != nil {
		h.t.Fatal(err)
	}
	return path
}

type hdTile struct{ Label, File string }

// writeHDSheet lays tiles out in `cols` columns, each labelled "NN state", scaled by `scale` (2x shots at 0.5 keep sheets comparable).
func writeHDSheet(t *testing.T, out string, tiles []hdTile, cols int, scale float64) {
	t.Helper()
	if len(tiles) == 0 {
		return
	}
	imgs, tw, th := make([]image.Image, len(tiles)), 0, 0
	for i, tile := range tiles {
		f, err := os.Open(tile.File)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		imgs[i] = img
		b := img.Bounds()
		tw, th = max(tw, int(float64(b.Dx())*scale)), max(th, int(float64(b.Dy())*scale))
	}
	const label, pad = 20, 12
	rows := (len(tiles) + cols - 1) / cols
	sheet := image.NewRGBA(image.Rect(0, 0, pad+cols*(tw+pad), pad+rows*(th+label+pad)))
	xdraw.Draw(sheet, sheet.Bounds(), &image.Uniform{color.RGBA{42, 42, 46, 255}}, image.Point{}, xdraw.Src)
	for i, img := range imgs {
		x, y := pad+(i%cols)*(tw+pad), pad+(i/cols)*(th+label+pad)
		d := font.Drawer{Dst: sheet, Src: image.NewUniform(color.RGBA{238, 238, 238, 255}), Face: basicfont.Face7x13, Dot: fixed.P(x, y+14)}
		d.DrawString(fmt.Sprintf("%02d %s", i+1, tiles[i].Label))
		b := img.Bounds()
		xdraw.CatmullRom.Scale(sheet, image.Rect(x, y+label, x+int(float64(b.Dx())*scale), y+label+int(float64(b.Dy())*scale)), img, b, xdraw.Over, nil)
	}
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, sheet); err != nil {
		t.Fatal(err)
	}
}
```
- [ ] Gates file (in full): the probe script (Q17 adds the `field-border` rows), the probe types and runner, the layout-shift and focus-walk helpers, the coarse pass, gate E-F, and the state table. States 01–03 are staged; the others are listed with their names and held flags and are logged `NOT STAGED` until the E3 maker stages them as described below (this is the only part of part 2 that stays a skeleton).
<!-- embed: dryrun-m4/overlay/ui/desktop_homedash_designer_gates_test.go -->
```go
package ui

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// homeDashGateProbeJS measures the designer chrome: text 4.5:1 against its composited background (gradient materials use their opaque -color
// token), UI parts 3:1 (selection outline, handles, guides, ghost, marquee and focus rings against the page canvas, switch tracks against the
// panel, field and segmented borders against their own surface), hit sizes (24 px fine / 44 px coarse; handles 12 / 44; card hit buttons
// measured on their card) and running animations. It returns {contrast, ui, hits, anim}; every entry has `pass`.
const homeDashGateProbeJS = `()=>{
 const root=document.querySelector('.vd-window[data-app-id="homedash"] .hd-root'),out={contrast:[],ui:[],hits:[],anim:[]};if(!root)return out;
 const cv=document.createElement('canvas');cv.width=cv.height=1;const g=cv.getContext('2d',{willReadFrequently:true});
 const rgba=v=>{g.clearRect(0,0,1,1);g.fillStyle='rgba(0,0,0,0)';g.fillStyle=v;g.fillRect(0,0,1,1);const d=g.getImageData(0,0,1,1).data;return [d[0],d[1],d[2],d[3]/255];};
 const over=(a,b)=>{const al=a[3]+b[3]*(1-a[3]);return al?[0,1,2].map(i=>(a[i]*a[3]+b[i]*b[3]*(1-a[3]))/al).concat([al]):[0,0,0,0];};
 const SURF=[['.hd-f-input,.hd-f-select,.hd-ep-box,.hd-bp,.hd-f-seg','--vd-mat-field-color'],['.hd-btn,.hd-save-action,.hd-bp-opt[aria-checked="true"],.hd-f-seg-opt[aria-checked="true"]','--vd-mat-btn-color'],
  ['.hd-eb-done','--vd-mat-primary-color'],['.hd-ptab[aria-selected="true"],.hd-ep-opt[aria-selected="true"]','--vd-mat-selected-color'],['.hd-sheet,.hd-sel-chip,.hd-toast,.hd-pal-card,.hd-insp-variant,.hd-ep-pop,.hd-pal-avatar,.hd-draft-badge','--vd-mat-card-color'],
  ['.hd-palette,.hd-inspector','--vd-mat-side-color'],['.hd-editbar,.hdx-bar','--vd-mat-bar-color'],['.hd-page','--hd-canvas']];
 const bg=el=>{const L=[];for(let n=el;n&&n.nodeType===1;n=n.parentElement){const cs=getComputedStyle(n);let c=rgba(cs.backgroundColor);
   if(cs.backgroundImage!=='none'){const s=SURF.find(x=>n.matches(x[0]));if(s){const v=cs.getPropertyValue(s[1]).trim();if(v)c=rgba(v);}}
   if(c[3]>0){L.push(c);if(c[3]>=.99)break;}}let r=[255,255,255,1];for(let i=L.length-1;i>=0;i--)r=over(L[i],r);return r;};
 const lum=c=>{const f=v=>{v/=255;return v<=.03928?v/12.92:Math.pow((v+.055)/1.055,2.4);};return .2126*f(c[0])+.7152*f(c[1])+.0722*f(c[2]);};
 const ratio=(a,b)=>{const x=lum(a),y=lum(b);return Math.round((Math.max(x,y)+.05)/(Math.min(x,y)+.05)*100)/100;};
 const shown=el=>{const r=el.getBoundingClientRect();if(!r.width||!r.height)return false;const s=getComputedStyle(el);return s.visibility!=='hidden'&&Number(s.opacity)>.05&&!el.closest('[hidden],[inert],.hd-thumb');};
 const path=el=>el.tagName.toLowerCase()+'.'+[...el.classList].slice(0,2).join('.')+(el.dataset.hdWid?'#'+el.dataset.hdWid:'');
 const CHROME='.hd-editbar,.hd-conflict-bar,.hd-palette,.hd-inspector,.hd-sheet-layer:not([data-kind]) .hd-sheet,.hd-sel-chip,.hd-toast,.hd-drop-avatar,.hd-draft-badge,.hdx-elsewhere,.hdx-save';
 const coarse=matchMedia('(pointer: coarse)').matches;
 for(const host of root.closest('.hdx-app').querySelectorAll(CHROME))for(const el of [host,...host.querySelectorAll('*')]){
  if(!shown(el)||el.closest('[aria-disabled="true"],:disabled'))continue;const txt=[...el.childNodes].some(n=>n.nodeType===3&&n.textContent.trim());if(!txt)continue;
  const b=bg(el),c=over(rgba(getComputedStyle(el).color),b),r=ratio(c,b);out.contrast.push({el:path(el),ratio:r,pass:r>=4.5});}
 const page=root.querySelector('.hd-page'),canvas=page?rgba(getComputedStyle(root).getPropertyValue('--hd-canvas').trim()||getComputedStyle(page).backgroundColor):null;
 const ui=(el,what,col,against)=>{if(!el||!shown(el)||!against)return;const r=ratio(over(rgba(col),against),against);out.ui.push({el:path(el),what,ratio:r,pass:r>=3});};
 for(const w of root.querySelectorAll('.hd-page > .hd-widget[aria-selected="true"]'))ui(w,'selection',getComputedStyle(w).outlineColor,canvas);
 for(const hd of root.querySelectorAll('.hd-handle'))ui(hd,'handle',getComputedStyle(hd,'::before').borderTopColor,canvas);
 for(const x of root.querySelectorAll('.hd-guide'))ui(x,'guide',getComputedStyle(x).backgroundColor,canvas);
 for(const x of root.querySelectorAll('.hd-ghost,.hd-marquee'))ui(x,x.className,getComputedStyle(x).borderTopColor,canvas);
 for(const s of root.querySelectorAll('.hd-f-switch')){const tr=s.querySelector('.hd-f-switch-track'),cs=getComputedStyle(tr),on=s.getAttribute('aria-checked')==='true';ui(s,on?'switch-on':'switch-off',on?cs.backgroundColor:cs.borderTopColor,bg(s.parentElement));}
 for(const f of root.closest('.hdx-app').querySelectorAll('.hd-f-input,.hd-f-select,.hd-ep-box,.hd-f-seg,.hd-f-swatch')){if(!shown(f)||f.matches('.hd-ep-input'))continue;ui(f,'field-border',getComputedStyle(f).borderTopColor,bg(f));}
 const act=document.activeElement;if(act&&root.contains(act)&&act!==document.body){const cs=getComputedStyle(act),col=cs.outlineStyle!=='none'?cs.outlineColor:(cs.boxShadow.match(/rgba?\([^)]*\)/)||[''])[0];
  out.ui.push({el:path(act),what:'focus',ratio:col?ratio(over(rgba(col),bg(act.parentElement)),bg(act.parentElement)):0,pass:!!col&&ratio(over(rgba(col),bg(act.parentElement)),bg(act.parentElement))>=3});}
 for(const el of root.querySelectorAll('button,input:not([type=file]),select,[role=tab],[role=radio],[role=switch],.hd-handle')){if(!shown(el)||el.closest('.hd-page > .hd-widget'))continue;
  const box=(el.matches('.hd-pal-hit,.hd-insp-vhit,.hd-look-fhit')?el.parentElement:el).getBoundingClientRect(),min=el.matches('.hd-handle')?(coarse?44:12):(coarse?44:24);
  out.hits.push({el:path(el),w:Math.round(box.width),h:Math.round(box.height),pass:box.width>=min-.5&&box.height>=min-.5});}
 for(const a of document.getAnimations()){const tg=a.effect&&a.effect.target;if(!tg||!root.contains(tg)||a.playState!=='running')continue;const it=a.effect.getComputedTiming().iterations;out.anim.push({el:path(tg),name:a.animationName||a.transitionProperty||'',infinite:it===Infinity,pass:it!==Infinity});}
 return out;}`

type hdProbeRow struct {
	El    string  `json:"el"`
	What  string  `json:"what,omitempty"`
	Ratio float64 `json:"ratio,omitempty"`
	W     int     `json:"w,omitempty"`
	H     int     `json:"h,omitempty"`
	Name  string  `json:"name,omitempty"`
	Pass  bool    `json:"pass"`
}

type hdProbe struct {
	Contrast []hdProbeRow `json:"contrast"`
	UI       []hdProbeRow `json:"ui"`
	Hits     []hdProbeRow `json:"hits"`
	Anim     []hdProbeRow `json:"anim"`
}

func (p hdProbe) failures() []string {
	var out []string
	for kind, rows := range map[string][]hdProbeRow{"contrast": p.Contrast, "ui": p.UI, "hits": p.Hits, "anim": p.Anim} {
		for _, r := range rows {
			if !r.Pass {
				out = append(out, fmt.Sprintf("%s %s %s ratio=%v size=%dx%d %s", kind, r.El, r.What, r.Ratio, r.W, r.H, r.Name))
			}
		}
	}
	return out
}

func (h *hdDesigner) probe() hdProbe {
	h.t.Helper()
	var p hdProbe
	if err := json.Unmarshal([]byte(h.str("()=>JSON.stringify(("+homeDashGateProbeJS+")())")), &p); err != nil {
		h.t.Fatal(err)
	}
	return p
}

// rects returns the bounding boxes of the widgets, the edit bar and the inspector (layout-shift probe: equal before and after a selection).
func (h *hdDesigner) rects() string {
	return h.str(`()=>JSON.stringify([...document.querySelectorAll('.hd-root .hd-page > .hd-widget,.hd-root .hd-editbar,.hd-root .hd-inspector')].map(e=>{const r=e.getBoundingClientRect();return [Math.round(r.x*2)/2,Math.round(r.y*2)/2,Math.round(r.width*2)/2,Math.round(r.height*2)/2];}))`)
}

// hdGateState is one staged editor state of gate E-S/E-T. enter stages it, leave undoes it; held states keep a mouse button down while shot.
// A state with a nil enter is listed so the sheets keep their order, but the runner logs it as NOT STAGED (the E3 maker stages it as described in the plan).
type hdGateState struct {
	name         string
	held         bool
	enter, leave func(h *hdDesigner)
}

func hdGateStates() []hdGateState {
	return []hdGateState{
		{name: "01-empty-page", enter: func(h *hdDesigner) { h.openDashboard(h.fixture.seedDashboard(hdGateEmptyDoc(h.fixture))); h.enterEdit() }},
		{name: "02-palette-widgets", enter: func(h *hdDesigner) { h.openDashboard(h.fixture.seedDashboard(hdDesignerDoc(h.fixture))); h.enterEdit() }},
		{name: "03-palette-devices", enter: func(h *hdDesigner) {
			h.page.MustEval(`()=>document.querySelector('.hd-palette .hd-f-tab[data-tab="entities"]').click()`)
			h.until(15*time.Second, "device rows", `()=>document.querySelectorAll('.hd-ent-row').length>0`)
		}},
		{name: "04-drop-ghost-reflow", held: true}, {name: "05-move-guides", held: true}, {name: "06-resize-limit", held: true}, {name: "07-marquee", held: true},
		{name: "08-multi-select"}, {name: "09-inspector-data"}, {name: "10-inspector-look"}, {name: "11-inspector-options"}, {name: "12-inspector-actions"}, {name: "13-inspector-multi"},
		{name: "14-look-family"}, {name: "15-look-custom"}, {name: "16-look-surface-grid"}, {name: "17-conflict-sheet"}, {name: "18-save-saving"}, {name: "19-save-retrying"}, {name: "20-save-invalid"},
		{name: "21-conflict-bar"}, {name: "22-preview-md-custom"}, {name: "23-preview-sm-auto"}, {name: "24-page-tabs-menu", held: true}, {name: "25-context-widget", held: true}, {name: "26-context-canvas", held: true},
		{name: "27-drawers-palette"}, {name: "28-drawers-inspector"}, {name: "29-readonly"}, {name: "30-edit-elsewhere"}, {name: "31-import-issues"}, {name: "32-new-sheet"}, {name: "33-rename-sheet"},
	}
}

// hdGateEmptyDoc is the dashboard of state 01: the designer document with one empty page.
func hdGateEmptyDoc(f *homeDashFixture) map[string]any {
	d := hdDesignerDoc(f)
	d["name"], d["pages"] = "Empty page", []any{map[string]any{"id": "main", "name": "Main", "icon": "dashboard", "compact": true, "widgets": []any{}, "layouts": map[string]any{"lg": []any{}, "md": nil, "sm": nil}}}
	return d
}

type hdGateResult struct {
	State, Variant string
	DPR            int
	Failures       []string
}

// TestDesktopHomeDashDesignerGatesBrowser stages every editor state and measures it (gates E-S, E-T, E-F). HOMEDASH_M4_SHOTS=1 runs E-S and E-F,
// =all adds E-T (16 variants at 1x and 2x); HOMEDASH_M4_DIR is required; HOMEDASH_M4_LANG defaults to en. CI skips it.
func TestDesktopHomeDashDesignerGatesBrowser(t *testing.T) {
	mode, dir, lang := os.Getenv("HOMEDASH_M4_SHOTS"), os.Getenv("HOMEDASH_M4_DIR"), os.Getenv("HOMEDASH_M4_LANG")
	if mode != "1" && mode != "all" {
		t.Skip("set HOMEDASH_M4_SHOTS=1 (E-S, E-F) or all (also E-T) to write the gate sheets")
	}
	if dir == "" {
		t.Fatal("HOMEDASH_M4_DIR is required")
	}
	if lang == "" {
		lang = "en"
	}
	h := newHDDesigner(t, lang)
	var results []hdGateResult
	variants := hdThemeVariants(t)
	pick := func(name string) hdThemeVariant {
		for _, v := range variants {
			if v.Name == name {
				return v
			}
		}
		t.Fatalf("no theme variant %s", name)
		return hdThemeVariant{}
	}
	run := func(group, root string, vs []hdThemeVariant, dprs []int, scale map[int]float64) {
		for _, dpr := range dprs {
			h.page.MustSetViewport(1600, 1000, float64(dpr), false)
			for _, v := range vs {
				var tiles []hdTile
				for _, st := range hdGateStates() {
					if st.enter == nil {
						t.Logf("%s %s: NOT STAGED", group, st.name)
						continue
					}
					st.enter(h)
					h.theme(v)
					tiles = append(tiles, hdTile{Label: st.name, File: h.capture(filepath.Join(dir, root, v.Name, fmt.Sprintf("%dx", dpr)), st.name)})
					p := h.probe()
					results = append(results, hdGateResult{State: st.name, Variant: v.Name, DPR: dpr, Failures: p.failures()})
					for _, f := range p.failures() {
						t.Errorf("%s %s %dx %s: %s", group, v.Name, dpr, st.name, f)
					}
					if st.leave != nil {
						st.leave(h)
					}
				}
				writeHDSheet(t, filepath.Join(dir, fmt.Sprintf("%s-%s-%dx.png", group, v.Name, dpr)), tiles, 3, scale[dpr])
			}
		}
	}
	run("ES", "ES", []hdThemeVariant{pick("fruity-light"), pick("fruity-dark")}, []int{1}, map[int]float64{1: 0.5})
	if !h.hdLayoutShiftFree() {
		t.Errorf("selecting and deselecting a widget shifts widgets, edit bar or inspector")
	}
	for _, bad := range h.hdFocusWalk(60) {
		t.Errorf("focus ring: %s", bad)
	}
	h.runFamilies(t, dir, pick)
	if mode == "all" {
		run("ET", "ET", variants, []int{1, 2}, map[int]float64{1: 0.5, 2: 0.25})
	}
	h.coarseProbe(t, dir)
	raw, _ := json.MarshalIndent(results, "", " ")
	_ = os.WriteFile(filepath.Join(dir, "probes.json"), raw, 0o644)
	var rows strings.Builder
	for _, r := range results {
		for _, f := range r.Failures {
			rows.WriteString("<tr><td>" + html.EscapeString(r.State+" "+r.Variant+fmt.Sprintf(" %dx", r.DPR)) + "</td><td>" + html.EscapeString(f) + "</td></tr>\n")
		}
	}
	index := "<!doctype html><meta charset=utf-8><title>HomeDash M4 gates</title><h1>Gate sheets</h1><p>See the ES-*, EF and ET-* PNG files in this folder.</p><h2>Failing probes</h2><table>" + rows.String() + "</table>\n"
	_ = os.WriteFile(filepath.Join(dir, "index.html"), []byte(index), 0o644)
}

// runFamilies is gate E-F: the designer on each of the four family canvases (selection with handles) in fruity-light and fruity-dark; the probes include
// selection and handles against that family's canvas. Shots go to EF/<family>-<variant>.png and the sheet to EF.png (light | dark per row).
func (h *hdDesigner) runFamilies(t *testing.T, dir string, pick func(string) hdThemeVariant) {
	t.Helper()
	var tiles []hdTile
	for _, family := range []string{"glass", "tactile", "minimal", "instrument"} {
		id := h.fixture.seedDashboard(hdDesignerDoc(h.fixture))
		h.fixture.setFamily(t, id, family)
		h.openDashboard(id)
		h.enterEdit()
		h.page.MustEval(`()=>document.querySelector('.hd-root .hd-page > .hd-widget').click()`)
		for _, name := range []string{"fruity-light", "fruity-dark"} {
			h.theme(pick(name))
			tiles = append(tiles, hdTile{Label: family + " " + name, File: h.capture(filepath.Join(dir, "EF"), family+"-"+name)})
			for _, f := range h.probe().failures() {
				t.Errorf("EF %s %s: %s", family, name, f)
			}
		}
	}
	writeHDSheet(t, filepath.Join(dir, "EF.png"), tiles, 2, 0.5)
}

// coarseProbe re-runs the hit-size probe with touch emulation on (UNVERIFIED that emulation flips (pointer: coarse); if it does not, it logs and skips).
func (h *hdDesigner) coarseProbe(t *testing.T, dir string) {
	t.Helper()
	five := 5
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true, MaxTouchPoints: &five}).Call(h.page); err != nil {
		t.Logf("touch emulation unavailable: %v", err)
		return
	}
	defer func() { _ = (proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(h.page) }()
	if !h.yes(`()=>matchMedia('(pointer: coarse)').matches`) {
		t.Log("touch emulation does not flip (pointer: coarse); coarse hit pass skipped")
		return
	}
	for _, name := range []string{"02-palette-widgets", "06-resize-limit", "09-inspector-data"} {
		for _, st := range hdGateStates() {
			if st.name == name && st.enter != nil {
				st.enter(h)
				for _, f := range h.probe().failures() {
					t.Errorf("coarse %s: %s", name, f)
				}
			}
		}
	}
	_ = dir
}

// hdFocusWalk presses Tab n times from the edit bar and returns the focus probe of every stop (each stop needs a 3:1 outline or ring).
func (h *hdDesigner) hdFocusWalk(n int) []string {
	var bad []string
	for i := 0; i < n; i++ {
		h.page.Keyboard.MustType(input.Tab)
		for _, r := range h.probe().UI {
			if r.What == "focus" && !r.Pass {
				bad = append(bad, fmt.Sprintf("stop %d %s ratio=%v", i, r.El, r.Ratio))
			}
		}
	}
	return bad
}

// hdLayoutShiftFree selects and deselects the first widget and reports whether the rects of widgets, bar and inspector stay equal.
func (h *hdDesigner) hdLayoutShiftFree() bool {
	before := h.rects()
	h.page.MustEval(`()=>{const w=document.querySelector('.hd-root .hd-page > .hd-widget');w.click();}`)
	h.page.MustEval(`()=>{const l=document.querySelector('.hd-root .hd-page');l.click();}`)
	return h.rects() == before
}
```
- [ ] Staging of the remaining states (the maker writes `enter`/`leave` for each):
  - Held states (04–07, 24–26) use `h.drag(..., hold: return false)`; the leave step releases with Escape and `Mouse.MustUp`.
  - Save states use `fixture.setPutDelay(3s)`, `answerPut(503, …, 99)` and `answerPut(422, {code:'invalid_document', issues:[{code:'too_long', path:'/name'}]}, 1)`.
  - Conflict states use `fixture.foreignSave` while dirty. Menus use `contextmenu` events dispatched at element centres.
  - Drawers: `.vd-window` resized to 860×720 by test-only style. Read-only: the `aurago:desktop-policy` event. Edit elsewhere: a second `openApp` window on the same dashboard.
  - Import issues: inject `{"pages":"x"}` through `DataTransfer` into the hidden file input (`[data-hd-import]`).
  - E-S: `fruity-light`, `fruity-dark` → `capture(dir/ES/<variant>/1x, <state>)`, sheet `dir/ES-<variant>-1x.png`, 3 columns, scale 0.5. E-T (`all`): DPR 1 and 2, 16 variants, sheets `dir/ET-<variant>-<dpr>x.png`. E-F: `runFamilies` (selection with handles on each family canvas; the held palette drag over empty cells is added by the E3 maker).
  - Strict probes: layout shift (`hdLayoutShiftFree`), focus ring on 60 Tab stops (`hdFocusWalk`), reduced motion (`windows.animations=false`: `anim` empty after settle), coarse hits (`coarseProbe`; UNVERIFIED that touch emulation flips `(pointer: coarse)`; if it does not, it logs and skips). Output: `dir/probes.json` and `dir/index.html` with the failing rows; every failed probe is a `t.Errorf` (field borders are probed, Q17).
- [ ] Run `go vet ./ui`, `go test ./ui -run '^TestDesktopHomeDash' -count=1` (it compiles and skips) and `go test ./internal/audit -run '^TestUIBrowserCIShardsCoverEveryTest$' -count=1` (the new names fall in "Desktop H-R"). PASS. Commit — `test(homedash): designer browser helpers, gate probes and sheet writer`.

### Task E3: Gates E-S, E-T, E-F (lead review)
- [ ] Run (R8):
```
$env:HOMEDASH_M4_SHOTS='1'; $env:HOMEDASH_M4_DIR='C:\disposable\homedash-m4\es-1'
& D:\tmp\go-build\hd-m4-ui.test.exe '-test.run=^TestDesktopHomeDashDesignerGatesBrowser$' '-test.v' '-test.timeout=60m'
```
  For E-T use `HOMEDASH_M4_SHOTS=all` and `…\et-1` (about 40 min).
- [ ] The lead reviews in the order `ES-fruity-light.png`, `ES-fruity-dark.png`, `EF.png`, then the 16 `ET-<variant>-1x.png` and spot checks of the `-2x` full-size PNGs for crispness. Every tile is named `NN state`. `index.html` shows the probe failures.
  - The bar: quiet, precise, nothing competing with the canvas; one accent; no clipping in German.
  - Findings are fixed in E1 (CSS) or the C/D modules and re-run on fresh sheets until the lead passes the gate. No commit without a pass.
  - Commit fixes per round — `fix(homedash): designer chrome gate findings round N`.

**Batch E review (reader-agent, Critical/Important only):**
- Every C15 selector is styled.
- host.test rules hold (motion gated, no literal colours, ≤ 900 lines).
- No `--hd-e-*` is declared.
- Coarse handles show SE/E/S only.
- Drawers never hide a focused element.
- The probe script measures real backgrounds; the sheet paths stay outside the repo.

---

## Batch F — tests, docs, hand-over (D13, D14)

### Task F1: TestDesktopHomeDashDesignerBrowser
**Files:** create `ui/desktop_homedash_designer_browser_test.go`. **Existing symbols changed:** none.
- [ ] Skeleton (in full; the doc is built on the starter so look, grid and kiosk keep the server shape; a second dashboard is seeded for window B, I9):
<!-- embed: dryrun-m4/overlay/ui/desktop_homedash_designer_browser_test.go -->
```go
package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDesktopHomeDashDesignerBrowser drives the designer end to end against the M4 fixture (D13): palette tap and drag add, entity drop and
// rebind, move, resize, keyboard nudge and resize, marquee, multi-select, undo/redo, autosave PUT body and revision, 409 with all three
// choices, foreign change clean and dirty, pages, md override and reset, clipboard across two windows, import/export round trip, look editor
// custom palette, read-only, beforeClose and the 60-widget drag budget. HOMEDASH_M4_SHOTS=1 writes step shots to $HOMEDASH_M4_DIR/flow.
func TestDesktopHomeDashDesignerBrowser(t *testing.T) {
	h := newHDDesigner(t, "en")
	shots, dir := os.Getenv("HOMEDASH_M4_SHOTS") != "", filepath.Join(os.Getenv("HOMEDASH_M4_DIR"), "flow")
	started, current := time.Now(), ""
	step := func(name string) {
		if current != "" {
			t.Logf("step %s done in %v", current, time.Since(started).Round(10*time.Millisecond))
			if shots && os.Getenv("HOMEDASH_M4_DIR") != "" {
				h.capture(dir, current)
			}
		}
		current, started = name, time.Now()
	}
	id := h.fixture.seedDashboard(hdDesignerDoc(h.fixture))
	id2 := h.fixture.seedDashboard(hdDesignerDoc(h.fixture)) // window B of step 15 (clipboard across windows) opens this one
	_ = id2
	h.openDashboard(id)
	// … steps 1-20 below (step 15 opens id2 in window B) …
	step("")
	if errs := h.str(`()=>hdErrs.join('\n')`); errs != "" {
		t.Fatalf("page errors:\n%s", errs)
	}
}

// hdDesignerDoc builds on the starter document (look, grid and kiosk keep the server shape) and replaces the pages. Page main with sw (switch/pill ha:switch.coffee 0,0 2x1), li (light/slider ha:light.kitchen 2,0 4x1), cpu (gauge/arc
// system:cpu 0,1 2x2), mem (sensor/numeral system:memory 2,1 2x1); page p2 "Second" with one entity tile.
func hdDesignerDoc(f *homeDashFixture) map[string]any {
	d := homeDashClone(f.starter)
	d["name"], d["slug"] = "Designer test", ""
	w := func(id, typ, variant, entity string) map[string]any {
		return map[string]any{"id": id, "type": typ, "variant": variant, "title": "", "bindings": map[string]any{"primary": entity}, "options": map[string]any{}, "actions": map[string]any{}, "confirm": false}
	}
	it := func(id string, x, y, wd, h int) map[string]any { return map[string]any{"id": id, "x": x, "y": y, "w": wd, "h": h} }
	d["pages"] = []any{
		map[string]any{"id": "main", "name": "Main", "icon": "dashboard", "compact": true,
			"widgets": []any{w("sw", "switch", "pill", "ha:switch.coffee"), w("li", "light", "slider", "ha:light.kitchen"), w("cpu", "gauge", "arc", "system:cpu"), w("mem", "sensor", "numeral", "system:memory")},
			"layouts": map[string]any{"lg": []any{it("sw", 0, 0, 2, 1), it("li", 2, 0, 4, 1), it("cpu", 0, 1, 2, 2), it("mem", 2, 1, 2, 1)}, "md": nil, "sm": nil}},
		map[string]any{"id": "p2", "name": "Second", "icon": "dashboard", "compact": true,
			"widgets": []any{w("ent", "entity", "tile", "system:cpu")},
			"layouts": map[string]any{"lg": []any{it("ent", 0, 0, 2, 2)}, "md": nil, "sm": nil}},
	}
	if k, ok := d["kiosk"].(map[string]any); ok {
		k["start_page"] = "main"
	}
	return d
}
```
- [ ] Steps. Each step has exact assertions. Ids come from `hdDesignerDoc`; texts come from `h.word`.
  1. **Enter.** Ctrl+E on the focused page → `data-hd-mode=design`, `.hd-editbar` first child, frames `role=option`, exactly one `tabindex=0`.
  2. **Palette tap.** Click `.hd-pal-card[data-type=switch][data-variant=pill] .hd-pal-hit` → a new widget is selected with `[data-hd-draft]`. The next PUT body lacks it (draft). In the inspector Data picker, type "coffee" → ArrowDown, Enter → the draft badge is gone and the next PUT contains the widget bound to `ha:switch.coffee`.
  3. **Palette drag.** Drag the light/dial card to the canvas cell (6,3). Mid-drag `.hd-ghost` is visible and some frame has a changed `--hd-x/--hd-y` (reflow). Drop → `fixture.dashboardDoc(id)`: the item has x = 6 and y = the row that `E.grid.compact` gives it (read it from the document; check that no free row in columns 6–7 sits above it) — part 1 compacts the lg result of `add` on a compact page, so y = 3 is not asserted literally.
  4. **Entity drop and rebind.**
     - Devices tab → drag the `ha:light.sofa` row to an empty cell → a new light/slider bound to it.
     - Drag `ha:input_boolean.vacation_mode` onto `sw` → mid-drag `[data-hd-drop=rebind]`; drop → binding changed; live contains `designer.announce.rebound`.
     - Drag `ha:select.heater_mode` onto `li` → `[data-hd-drop=invalid]`; drop → no change; live `drop_invalid`.
  5. **Move.** Drag `mem` one cell right → mid-drag `.hd-guide` ≥ 1; after the PUT, lg x = 3.
  6. **Resize.** Drag the `se` handle of `cpu` +1 column → w 3. Drag the `w` handle towards the minimum → `.hd-handle[data-hd-limit]` mid-drag.
  7. **Keyboard.** Focus `sw`; ArrowRight ×2 within 800 ms → one history entry (one Ctrl+Z restores both); Shift+ArrowDown → h 2; live text matches `designer.announce.moved`.
  8. **Marquee and multi-select.** Drag on empty canvas across `cpu` and `mem` → 2 selected, inspector title `inspector.multi_title {2}`. Shift+click `sw` → 3. "Align left edges" → equal lg x.
  9. **Undo/redo.** Ctrl+Z → align undone, live `announce.undone` with `cmd.align`. Ctrl+Shift+Z → re-applied.
  10. **Autosave.**
      - PUT revisions chain by +1.
      - No 422 in `putLog`.
      - Bodies have no client-only fields: the fixture lint `bad_json` never fires.
      - `look.palette.snapshot` exists with ≤ 40 entries, each `^#[0-9a-f]{6}$`.
  11. **409 three ways.** `setPutDelay(2s)`, edit, then `foreignSave(t, id, "agent", rename)` before the PUT → 409 → `.hd-conflict-bar`.
      - Resolve → Take theirs: the agent's name is shown and Ctrl+Z says "nothing to undo".
      - Repeat → Keep mine: the next PUT carries `current_revision`.
      - Repeat → Save mine as a copy: a new dashboard named `<name> (copy)` opens in design mode.
  12. **Foreign change, clean.** `foreignSave` while saved → toast `designer.reload_agent`, selection kept by id, no conflict bar.
  13. **Pages.**
      - `.hd-ptab-add` → inline input focused → type "Kitchen", Enter → tab text.
      - Tab context menu → "Move left" → order changes.
      - Drag a tab ≥ 5 px past its neighbour → order changes.
      - Delete a page with widgets → confirm sheet → removed.
      - With one page left, "Delete page…" is disabled.
  14. **md override.** md → chip "Automatic"; ArrowRight on a frame → chip "Custom", PUT `layouts.md` non-null; Reset → PUT `layouts.md` null.
  15. **Clipboard across windows.** In window A, Ctrl+C on `sw`. `openApp` the second dashboard `id2` (window B) → `enterEdit` → Ctrl+V → a new widget with a new id, same type, same binding (assert type, binding and id only, not the position: paste is compacted too).
  16. **Export/import round trip.**
      - Run export → `__exports[0]` is the envelope with `schema_version` and without id, revision, approved_revision, saved_by and updated_at; `kiosk` present.
      - Inject the same text as a file → validate → create → the new dashboard opens; pages, widgets and layouts equal the export; the issues sheet lists the unknown devices of the warnings after the import (I7, at most 8 rows).
      - Inject `{"pages":"x"}` → the issues sheet shows `error.bad_json`.
      - A file of 600 KiB → the too-large notice without reading (`file.text` never called, checked with a spy).
  17. **Look editor.**
      - Open via the Look button. Custom → `root[data-hd-palette=custom]`.
      - Set the accent through `input[type=color]` (value + input + change) and the hue range → the PUT body palette is `{mode:'custom', custom:{accent lower, surface_hue int, scheme}}` without `snapshot`.
      - Family card Tactile → `data-hd-family=tactile`.
      - Theme mode → the PUT has `snapshot` and no `custom`.
      - Each field is one undo step.
  18. **Read-only.** The `aurago:desktop-policy` event with readonly true while editing → view mode, no `.hdx-edit`, banner present. False → `.hdx-edit` back.
  19. **beforeClose.**
      - `answerPut(503, …, 99)`, edit → retrying.
      - Close the window through its title-bar close control (UNVERIFIED selector; the maker checks `window-shell-runtime.js` for the close button and `beforeClose`) → the leave sheet (close kind). "Keep window open" → the window stays.
      - Close again → "Close and keep trying" → the window is gone and `putLog` still grows. `answerPut` cleared → the last PUT is 200.
  20. **60-widget drag budget.**
      - Seed 60 entity tiles (2×1, 6 per row, bound to the six system ids) and enter edit.
      - Install `new PerformanceObserver(l=>…).observe({type:'longtask'})` and a rAF counter.
      - Press on tile 12 and move in 16 ms steps for 2 s across the grid; release.
      - Assert no long task > 50 ms (D11). Log the frame count.
- [ ] Run R8 `-test.run=^TestDesktopHomeDashDesignerBrowser$`, `go test ./internal/audit -run '^TestUIBrowserCIShardsCoverEveryTest$' -count=1`, then `TestDesktopHomeDashBrowser` and `TestDesktopHomeDashControlsBrowser`. All PASS. Commit — `test(homedash): designer browser test (M4 D13)`.

### Task F2: Live handlers accept the designer's save
**Files:** `internal/server/desktop_homedash_live_browser_test.go`. **Existing symbols changed:** `TestDesktopHomeDashLiveHABrowser` (test only, LOW). Dry run: `go vet -overlay … ./internal/server` clean (the browser test was not run).
- [ ] Insert before the final `UpstreamSecret` check (diff against base):
<!-- embed: dryrun-m4/overlay/diffs2/desktop_homedash_live_browser_test.go.diff -->
```diff
--- a/internal/server/desktop_homedash_live_browser_test.go
+++ b/internal/server/desktop_homedash_live_browser_test.go
@@ -164,2 +164,51 @@ func TestDesktopHomeDashLiveHABrowser(t *testing.T) {
 	}
+	// M4 (D13): edit and save against the real handlers. The designer's forSave body (kiosk block, theme snapshot, a widget added from the device
+	// browser through pickWidgetFor) must pass the strict decoder and the validator.
+	type hdDash struct {
+		Dashboard map[string]any `json:"dashboard"`
+	}
+	var list struct {
+		Dashboards []struct {
+			ID       string `json:"id"`
+			Revision int    `json:"revision"`
+		} `json:"dashboards"`
+	}
+	hdDecode(t, env.do(t, env.admin, http.MethodGet, "dashboards", nil), &list)
+	if len(list.Dashboards) != 1 {
+		t.Fatalf("dashboards = %+v", list.Dashboards)
+	}
+	dashID, rev0 := list.Dashboards[0].ID, list.Dashboards[0].Revision
+	server := func() map[string]any { var d hdDash; hdDecode(t, env.do(t, env.admin, http.MethodGet, "dashboard?id="+dashID, nil), &d); return d.Dashboard }
+	waitRev := func(after int) map[string]any {
+		var d map[string]any
+		sourcetest.WaitFor(t, 15*time.Second, func() bool { d = server(); n, _ := d["revision"].(float64); return int(n) > after }, "a saved revision")
+		return d
+	}
+	page.MustEval(`()=>document.querySelector('.hdx-edit').click()`)
+	until(10*time.Second, "design mode", `()=>!!document.querySelector('.hd-root[data-hd-mode="design"] .hd-editbar')`)
+	page.MustEval(`()=>document.querySelector('.hd-root[data-hd-mode="design"] [data-hd-wid="w2"]').focus()`)
+	page.Keyboard.MustType(input.ArrowRight)
+	d1 := waitRev(rev0)
+	page.MustEval(`()=>document.querySelector('.hd-palette .hd-f-tab[data-tab="entities"]').click()`)
+	until(15*time.Second, "the kitchen light row", `()=>!!document.querySelector('.hd-ent-row[data-entity="ha:light.kitchen"]')`)
+	page.MustEval(`()=>document.querySelector('.hd-ent-row[data-entity="ha:light.kitchen"]').click()`)
+	d2 := waitRev(int(d1["revision"].(float64)))
+	found := false
+	for _, p := range d2["pages"].([]any) {
+		for _, w := range p.(map[string]any)["widgets"].([]any) {
+			wm := w.(map[string]any)
+			if b, _ := wm["bindings"].(map[string]any); b != nil && b["primary"] == "ha:light.kitchen" && wm["type"] == "light" {
+				found = true
+			}
+		}
+	}
+	if !found {
+		t.Fatalf("no light widget for ha:light.kitchen in %v", d2["pages"])
+	}
+	pal := d2["look"].(map[string]any)["palette"].(map[string]any)
+	if snap, _ := pal["snapshot"].(map[string]any); pal["mode"] == "theme" && (len(snap) == 0 || len(snap) > 40) {
+		t.Fatalf("theme snapshot = %v", pal["snapshot"])
+	}
+	page.MustEval(`()=>document.querySelector('.hd-eb-done').click()`)
+	until(10*time.Second, "view mode", `()=>document.querySelector('.hdx-view .hd-root').dataset.hdMode==='view'`)
 
```
  (`ha:light.kitchen` follows the `ha:` prefix of the M3 entities; UNVERIFIED in the hasstest env. If the id differs, the maker takes it from `GET entities`.)
- [ ] Run (R8 for `./internal/server`) `TestDesktopHomeDashLiveHABrowser`. PASS. Commit — `test(homedash): live handlers accept the designer's saves`.

### Task F3: Security review (D14)
- [ ] security-agent, scope:
  - **Import:** 512 KiB cap before `file.text()`; parse limits; envelope detection; id/slug/server fields cleared; nothing rendered before validate; issue paths shown via textContent and cut to 120 runes; create errors mapped.
  - **Clipboard:** part 1's decode, plus the paste path through `D` (part 2 adds no clipboard code).
  - **Export:** forSave only; no `saved_by`/revision; kiosk block without secrets; blob URL revoked.
  - **Snapshot tokens:** lower-case hex, catalog names only, ≤ 40.
  - **Keepalive save:** part 1.
  - **Part-2 DOM:** entity names, areas and ids, template and option strings only via textContent/`text:`; the icon list is internal; no `innerHTML`; file input `accept`.
- [ ] Fix Critical and Important findings before F4. Commit — `fix(homedash): security review findings (M4)`, if any.

### Task F4: Docs
**Files:** `ui/js/homedash/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `documentation/homedash-visual-system.md`. Before committing, check `git ls-files -v ui/js/homedash/AGENTS.md` (a lowercase `h` means assume-unchanged; see the theme-system memory) and fix that.
- [ ] Engine guide, section "Designer (M4)":
  - load order (C1 with the part-2 slot);
  - mount edit surface (C2);
  - designer API and state (C4);
  - commands, history and save lanes (C5, C6, C8);
  - store docs and lock (C7);
  - API (C9) and catalog (C10);
  - clipboard (C11); gestures (C12); render/view (C13); keys (C14);
  - the DOM/CSS contract table (C15); announcements (C16); i18n prefixes (C17); fixture controls (C18); icons (C19); part-2 interfaces (C20);
  - the part-2 rules: E.form, thumbs, `--hd-e-*`, forms on E.sheet, one accent, panel text pairs;
  - the test commands.
- [ ] Apps guide (HomeDash section): host menus (shortcutHint only), context-menu mapping, beforeClose, Edit/elsewhere/save chips, the bar hidden in design, Empty → designer, sheets inside `.hd-root`, gate harness env vars.
- [ ] Visual system: section "Editor chrome (V14)" with tokens, sizes, states, coarse rules, motion, the gates, the probe definitions and the sheet locations.
- [ ] Commit — `docs(homedash): designer guides and editor chrome`.

### Task F5: Hand-over and German preview pack
- [ ] `docs/superpowers/specs/2026-11-23-homedash/m4-handover.md` (new; `git add -f`):
  - what shipped;
  - contract summary and deviations: readings 1–12 of both parts, the lead answers;
  - tests and commands;
  - the gate runs with directories and the dates they passed;
  - open items (the open items of the Decisions section, M5 types needing `widget.<t>.desc` and an `icon.<name>` key);
  - nothing pushed or deployed.
- [ ] German preview pack (not blocking): `HOMEDASH_M4_SHOTS=1`, `HOMEDASH_M4_LANG=de`, `HOMEDASH_M4_DIR=C:\disposable\homedash-m4\de-preview`. Point the user to `index.html` there.
- [ ] Commit — `docs(homedash): M4 hand-over`.

**Batch F review (reader-agent, Critical/Important only):**
- The D13 list is fully covered with no duplicate coverage of Node tests.
- The shard check is green; nothing stays a skip in CI except the gates.
- The live step proves server acceptance.
- Docs match the code.

---

## Contract amendments requested

None open. **Amendment 1** (accepted): `hd-designer.js` calls `E.ddialogs?.enhanceTabs?.(D, bar.slot('pages'))` once, after `D` exists (part 1 revision 3, B5). C20 names the hook with `D`; there is no inspector fallback.

## Decisions of the lead (formerly questions; all defaults accepted unless noted)

- **Q1** E-S uses Fruity light and Fruity dark. **Q2** Canvas overlays use `--hd-focus`. **Q3** The look editor is an end-docked modal sheet with no scrim and a live canvas. **Q4** In view mode, rename, look and add page enter edit first. **Q5** `new` without a dashboard opens the home screen. **Q8** `[data-hd-drawer]` only in drawer mode. **Q9** States are monochrome; one accent. **Q11** Thumbs are capped at 2×2. **Q13** Area options come from the loaded rows. **Q14** Unknown entity = store status `unknown` or `GET entity` answering 404 `entity_not_found`. **Q15** E-T runs only with `HOMEDASH_M4_SHOTS=all`; CI skips the gates test. **Q16** "Show details" only for lights.
- **Q6 (decided, part 1 B7)** Home-list delete keeps the host's `sheet.confirm` + `inst.remove(summary)`; `run('delete_dashboard')` only for the open dashboard.
- **Q7 (decided, part 1 A11)** German `leave.drafts_many` = "… Diese fallen weg, wenn Du beendest."; no formal address anywhere in parts 1 and 2 (swept).
- **Q10 (decided)** Icon names are translated: `homedash.icon.<name>` for all 31 names, prefix registered in part 1.
- **Q12 (decided)** Import is available from the home screen (`ui.sheetHost()`).
- **Q17 (decided)** Field and segmented borders are probed at 3:1 in all 16 variants; the editor CSS draws them from `--vd-*`.
- **Open for the lead:** none that blocks the plan. Unverified until the gates run: the 60 % border mix, touch emulation flipping `(pointer: coarse)`, the `ha:light.kitchen` id in the hasstest environment, the close button selector of F1 step 19.
