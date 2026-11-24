# HomeDash M0 Frontend and Visual Foundations Implementation Plan — BASE TEXT (the revision `2026-11-23-homedash-m0-frontend.md` governs; do not change the line count of this file)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the shell-independent HomeDash engine foundations: the pure core (F0), the token, palette and bridge stylesheet bundle with its Node guard (V0), static builders for all 20 primitives plus the widget registry and frame host (P0), and the test-only control lab with its Go browser test (V1). When this plan is done, `npm run test:homedash` and `npm run check:ui` pass, `TestDesktopHomeDashControlLabBrowser` writes screenshot sheets and `probes.json`, and nothing changes for users: no app is registered.

**Architecture:** Each file is a classic IIFE that extends `window.HomeDashEngine` (called `E` inside the files) under `ui/js/homedash/`. Load order: `hd-core`, `hd-format`, `hd-colour`, `hd-grid`, `hd-layout`, `hd-doc`, `hd-history`, `hd-icons`, `hd-geometry`, `hd-prim-displays`, `hd-prim-controls`, `hd-widgets`, `widgets/hd-w-misc`. The CSS parts are concatenated into `ui/css/homedash.bundle.css`. The lab is a fixture under `ui/testdata/`, which `cmd/assetpack` never ships (`assets/web-assets.json:5` lists only 3d, cfg, css, dashboard, partials, js, lang, fonts and img; `cmd/assetpack/main.go:69` skips every other directory). The Go test serves the fixture from `ui.Content`, which is `os.DirFS(ui)` (`ui/embed.go:13-18`).

**Tech Stack:** Vanilla JS (ES2020, Chromium 111+), CSS (no `light-dark()`), Node 22 `node:vm` tests, Go 1.27 with go-rod.

**I wrote the code in this plan without running it.** Implementers may fix errors. They must not change anything in "## Contracts" without reporting it to the lead.

**Global rules for every task:**
- Work in the worktree from Task 0.
- Before editing an EXISTING symbol, run GitNexus `impact`. Before each commit, run `detect_changes`. If the index is unavailable (likely in worktrees; on 2026-11-23 the CLI reported "index version 49, build 47"), confirm callers with grep and write "GitNexus unavailable; callers confirmed with grep" in the commit body.
- Commit after every task, using only the task's paths.
- Files under `docs/` and new `AGENTS.md` files are git-ignored (`.gitignore:134-144`) and need `git add -f`.
- LF line endings, no BOM.
- No external assets. No `innerHTML`, `outerHTML` or `insertAdjacentHTML`. No `setAttribute('style'`.
- JS modules stay under 1100 lines (aim for under 700). CSS parts stay at or under 900 lines.
- Untrusted text goes through `textContent` only.
- The Write and Edit tools decode `\uXXXX` escapes, so never type `\u` sequences. Build non-ASCII characters with `String.fromCodePoint(...)` in JS.
- Never edit files through Bash heredocs.
- Never call `t('literal.key')` in engine files. `TestFrontend_StaticI18nKeysExistInEnglishBundle` scans `ui/js/**` with `\bt\(\s*'…'` (`ui/i18n_lint_test.go:450`). M0 ships no locale files; state words are built dynamically as `t(STATE_PREFIX + word)`.

## Lead amendments (2026-11-23)

These override the body of this plan wherever it says otherwise. Every dispatch includes them.

**A1. No hook in the shared regression script.** Task 9 does not add HomeDash to
`scripts/test-ui-regressions.mjs` (a file other sessions edit, with failures that predate this work). The guard
runs as `npm run test:homedash`, as Docker Manager's guard does; every task and every review runs it.

**A2. Value arcs can sweep.** The dial and arc-gauge primitives expose their value arc as one path with
`pathLength="1000"`, and `set()` writes the lit length through the CSSOM
(`arc.style.strokeDasharray = '<lit> 1000'`). The Node guard allows `transition` of `stroke-dasharray` inside
the motion gate; keyframes stay limited to `transform` and `opacity`. Milestone M1 relies on this.

**A3. Backdrop parameter.** The lab URL also accepts `backdrop` (`theme`, `solid`, `gradient`; default `theme`)
and writes it as `data-hd-backdrop` on `.hd-root`. `homedash-base.css` gets the three backdrop rules of visual
part section 5 (theme, solid, gradient) from the palette tokens.

## File structure

| Path | New/Edit | Task |
|---|---|---|
| `scripts/test-homedash-ui.mjs` (runner), `scripts/homedash-tests/{harness,dom,css-lint}.mjs` | new | 1, 9 |
| `scripts/homedash-tests/*.test.mjs` (core, format, grid, layout, doc, colour, bundle, guard, geometry, displays, controls, widgets, contract, lab) | new | 1–15 |
| `ui/js/homedash/hd-{core,format,colour,grid,layout,doc,history,icons,geometry,prim-displays,prim-controls,widgets}.js`, `ui/js/homedash/widgets/hd-w-misc.js` | new | 1–13 |
| `ui/css/homedash-{tokens,bridge,base}.css` (content); `homedash-{primitives,family-glass,family-tactile,family-minimal,family-instrument,widgets}.css` (comment-only starters); `ui/css/homedash.bundle.css` (generated, committed, `eol=lf` per `.gitattributes:25`) | new | 8 |
| `scripts/build-ui-bundles.js` (cssBundles entry, `:263-335`), `package.json` (`test:homedash`) | edit | 8, 1 |
| `ui/desktop_js_line_budget_test.go` | edit | 7 |
| `scripts/test-ui-regressions.mjs` (one CI hook; Node scripts run in CI only through it, see `security-gates.yml:56-57` and `test-ui-regressions.mjs:5880-5893`) | edit | 9 |
| `ui/testdata/homedash-lab.html`, `homedash-lab.css`, `homedash-lab.js`, `homedash-lab-probes.js` | new | 14, 15 |
| `ui/desktop_homedash_lab_plan_test.go`, `ui/desktop_homedash_control_lab_browser_test.go` | new | 16 |
| `ui/js/homedash/AGENTS.md` (force-add), `ui/AGENTS.md` (Child DOX Index, `:1584`) | new/edit | 17 |

## Contracts

### C1 Engine parts
`E.core`, `E.el`, `E.svg`, `E.format`, `E.colour`, `E.grid`, `E.layout`, `E.doc`, `E.history`, `E.icons`, `E.geometry`, `E.prim` (20 builders plus `E.prim.util`), `E.widgets`. No file touches the DOM at load. Every file except `hd-core` throws if the part it depends on is missing.

### C2 Primitive markup

**Common to every builder:**
- `E.prim.<name>(opts)` returns `{ el, set(state), destroy() }`.
- Common opts: `label` (accessible name, already translated by the caller), `size` (`m` or `l`, written as `data-hd-size`), `variant` (written as `data-hd-variant` where the builder has variants), `lang` (default `en`).
- `set(state)` always runs `util.applyState(el, state)`. It writes on `el`:
  - `data-hd-status` = `ok`, `unavailable`, `unknown` or `error`
  - `data-hd-pending`, `data-hd-stale`, `data-hd-readonly`, `data-hd-disabled` = `"true"` or absent
  - `data-hd-tone` = `ok`, `warn` or `crit`, or absent
- On the role element of interactive builders it also writes:
  - `aria-disabled="true"` when the state is disabled or unavailable
  - `aria-readonly="true"` when read-only. Plain buttons have no `aria-readonly`; on them read-only becomes `aria-disabled`.
- `destroy()` removes `el` and is idempotent.
- JS writes only attributes, `textContent` and custom properties through `style.setProperty`.
- Angles are in degrees from 12 o'clock, clockwise. `--hd-value` is a 0–1 fraction with 4 decimals.
- The lab alone sets the test hook `data-hd-focus="ring"`. Focus-ring CSS matches `:is(:focus-visible,[data-hd-focus="ring"])`.

| Builder | Extra opts | Markup | State read | JS writes |
|---|---|---|---|---|
| toggle | – | `button.hd-toggle[type=button][role=switch][aria-label]` > `span.hd-toggle-track[aria-hidden=true]` > `span.hd-toggle-glyph[data-glyph=on]`, `span.hd-toggle-glyph[data-glyph=off]`, `span.hd-toggle-thumb` | on | `aria-checked`, `data-hd-on` |
| rocker | – | `button.hd-rocker[type=button][role=switch][aria-label]` > `span.hd-rocker-bezel[aria-hidden=true]` > `span.hd-rocker-paddle` > (`span.hd-rocker-half[data-half=on]` > `span.hd-rocker-legend` "I"; `span.hd-rocker-half[data-half=off]` > `span.hd-rocker-legend` "O"), then `span.hd-rocker-led` | on | `aria-checked`, `data-hd-on` |
| pushButton | icon; variant chip\|tile | `button.hd-push[type=button][data-hd-variant]` > `svg.hd-ico`? + `span.hd-push-label` | on, text | `data-hd-on`, label text |
| slider | variant h\|v; min, max, step, unit, decimals | `div.hd-slider[role=slider][tabindex=0][data-hd-variant][aria-orientation][aria-label]` > `span.hd-slider-track[aria-hidden=true]` > `span.hd-slider-fill`, `span.hd-slider-thumb` | value, min, max, valueText | `--hd-value`, `aria-valuemin/max/now/text`, `data-hd-empty` |
| knob | variant ring\|knob; min, max, unit, decimals | `div.hd-dial[role=slider][tabindex=0][data-hd-variant][aria-label]` > `svg.hd-dial-svg[viewBox="0 0 100 100"][aria-hidden=true]` > `path.hd-dial-ticks` (`ticks(50,50,45,49,-135,270,10)`), `path.hd-dial-track[pathLength=100]`, `path.hd-dial-arc[pathLength=100]` (both `arc(50,50,42,-135,270)`), `circle.hd-dial-knob[r=5.5]`; then `span.hd-dial-cap[aria-hidden=true]` > `span.hd-dial-pointer`; then `span.hd-dial-readout` > numeral | value | `--hd-value`, `--hd-angle` (−135…135deg), knob `cx`/`cy` = `point(50,50,42,angle)`, `aria-value*`, readout |
| stepper | min, max, step, unit, decimals, decLabel, incLabel | `div.hd-stepper[role=group][aria-label]` > `button.hd-step[type=button][data-dir="-1"][aria-label]` (icon minus), `span.hd-step-value[role=spinbutton][tabindex=0][aria-label]` > numeral, `button.hd-step[type=button][data-dir="1"][aria-label]` (icon plus) | value | spinbutton `aria-valuemin/max/now/text`; step buttons get `aria-disabled` at the limit or when disabled/unavailable |
| segmented | options `[{id,label,icon?}]` | `div.hd-seg[role=radiogroup][aria-label]` > `span.hd-seg-indicator[aria-hidden=true]`, then `button.hd-seg-opt[type=button][role=radio][data-hd-option][aria-checked][tabindex]` > `svg`? + `span.hd-seg-label` | value (option id) | `--hd-index` (−1 when none), `--hd-count` (written at build), `aria-checked`, roving `tabindex` (checked option, or the first when none, is 0) |
| colourWheel | diameter (200) | `div.hd-wheel[role=img][aria-label]` > `canvas.hd-wheel-canvas[width][height][aria-hidden=true]`, `span.hd-wheel-thumb[aria-hidden=true]` | hue, sat, valueText | `--hd-wheel-x` = .5+.5·sat·sin(h), `--hd-wheel-y` = .5−.5·sat·cos(h) (4 dp), `--hd-wheel-rgb` "r g b" (`hsvToRgb(h,sat,1)`, rounded); canvas painted once at build when `getContext` exists |
| ctStrip | min 2200, max 6500 | `div.hd-ct[role=slider][tabindex=0][aria-label]` > `span.hd-ct-track[aria-hidden=true]`, `span.hd-ct-thumb[aria-hidden=true]` | value (K) | `--hd-value`, `aria-value*` (valuetext `valueText(v,{decimals:0,grouping:false,unit:'K'})`) |
| numeral | unit, decimals, chars (4) | `span.hd-numeral` > `span.hd-num-int`, `span.hd-num-frac`, `span.hd-num-unit`, `span.hd-num-trend[aria-hidden=true]` | value, unit, decimals, text, trend | part texts from `E.format.split`, `--hd-num-chars` (build), `data-hd-empty`, `data-trend` (up\|down\|flat) with glyph U+2191, U+2193 or U+2192 |
| segReadout | digits (4), decimals, unit | `span.hd-seg7[role=img][aria-label]` > `svg.hd-seg7-svg[viewBox="0 0 {16·digits} 20"][aria-hidden=true]` > `g.hd-seg7-digit[data-index][transform="translate(16i 0)"]` > 7 × `path.hd-seg7-seg[data-seg=a..g]` + `path.hd-seg7-dp` (`segmentPaths(12,20,2.4,0.6)`); then `span.hd-seg7-unit` | value, text | `data-lit="true"` on lit paths (right-aligned; extra cells dropped from the left), `aria-label` = valueText, `data-hd-empty` |
| arcGauge | min, max, unit, decimals, zones `[{from,to,tone}]` | `div.hd-gauge[data-hd-gauge=arc][role=meter][aria-label]` > `svg.hd-gauge-svg[viewBox="0 0 100 100"][aria-hidden=true]` > `path.hd-gauge-track[pathLength=100]`, `path.hd-gauge-zone[data-tone]`* (`arc(50,50,36,a(from),a(to)-a(from))`), `path.hd-gauge-arc[pathLength=100]` (both `arc(50,50,42,-135,270)`), `circle.hd-gauge-cap[r=3.5]`; then `span.hd-gauge-readout` > numeral, `span.hd-gauge-min`, `span.hd-gauge-max` | value, tone | `--hd-value`, `--hd-angle` (−135…135), cap `cx`/`cy`, `data-hd-tone` (`state.tone`, else the zone containing the value), `aria-value*` |
| needleGauge | same | `div.hd-gauge[data-hd-gauge=needle][role=meter][aria-label]` > svg > `path.hd-gauge-ticks` (`ticks(50,50,38,46,-120,240,10)`), `path.hd-gauge-ticks[data-minor=true]` (`ticks(50,50,42,46,-120,240,50)`), zones, `g.hd-gauge-needle` > `path.hd-gauge-needle-shape` (`needle(50,50,44,12,3.2,0.8)`) + `circle.hd-gauge-hub[cx=50][cy=50][r=4.5]`; then readout, min and max spans | value | `--hd-value`, `--hd-angle` (−120…120; kept when `needleChanged` is false), `data-hd-tone`, `aria-value*` |
| bar | variant h\|v\|tank; zones, readout | `div.hd-bar[role=meter][data-hd-variant][aria-label]` > `span.hd-bar-track[aria-hidden=true]` > `span.hd-bar-zone[data-tone]`* (`--hd-from`/`--hd-to`), `span.hd-bar-fill`; then `span.hd-bar-readout` > numeral (when readout) | value | `--hd-value`, `data-hd-tone`, `aria-value*` |
| lamp | – | `span.hd-lamp[role=img][aria-label]` > `span.hd-lamp-dot[aria-hidden=true]`, `span.hd-lamp-halo[aria-hidden=true]` | on, text | `data-hd-on`, `aria-label` = text, else label |
| badge | icon | `span.hd-badge[data-tone]` > `svg`? + `span.hd-badge-text` | text, tone (ok\|warn\|crit\|info\|neutral), icon | text, `data-tone` (frontend 9.2 name), icon swap |
| iconBadge | icon | `span.hd-iconbadge[aria-hidden=true]` > `svg.hd-ico` | icon | svg replaced when the icon changes |
| sparkline | w 120, h 40 | `svg.hd-spark[role=img][aria-label][viewBox="0 0 w h"][width][height]` > `path.hd-spark-area`, `path.hd-spark-line`, `circle.hd-spark-last[r=3]`; never `preserveAspectRatio` | points, w, h, summary | `d`, viewBox/width/height on size change, circle `cx`/`cy`/`hidden`, `aria-label` = summary, else label; `data-hd-empty` |
| frame | title, sub, icon, more (label or null), foot | `div.hd-frame` > `div.hd-head` > (`span.hd-icon` > svg?, `span.hd-title[id]`, `span.hd-sub`); `div.hd-body`; `div.hd-foot[hidden]` unless foot; `div.hd-state` > `span.hd-state-text`; `button.hd-more[type=button][aria-label]` (only when more is set) > icon more | title, sub, stateText, icon | texts; also returns `body`, `head`, `foot`, `titleId` |
| sheet | title, kind (moreinfo\|confirm\|form) | `div.hd-sheet-layer[hidden]` > `div.hd-sheet[role=dialog][aria-modal=true][data-kind][aria-labelledby]` > `div.hd-sheet-head` > `h2.hd-sheet-title[id]`; `div.hd-sheet-body`; `div.hd-sheet-foot` | open, title | `hidden` on the layer, title text; also returns `body`, `foot` |

### C3 Widget host (`E.widgets`)
- `register(type, {bindings: string[], variants: string[], create(api)})`.
- `get(type)`, `types()`.
- `host({widget, mode='view', size:{w,h}, t, lang='en', api: factory, moreLabel})` returns `{el, body, update(), resize(size), setMode(mode), destroy()}`.
- The factory is called as `factory({widget, mode, signal})`. It must return `{entity, can, command, moreInfo, history, announce}`.
- The `api` passed to `create` is frozen. It contains `body, widget, variant, family, mode` (getter), `t, fmt (E.format), prim (E.prim), icons (E.icons), signal, lang`, plus the six factory functions.
- `entity()` returns the frontend part 2.2 view state, extended with optional `tone`.
- `update(view)` receives `view = {mode, entities:{<binding>: viewState|null}}`.
- Wrapper element: `div.hd-widget[data-hd-type][data-hd-variant][data-hd-family?][data-hd-size="WxH"][data-hd-status][data-hd-on?][data-hd-pending?][data-hd-stale?][data-hd-readonly?][data-hd-tone?][role=group][aria-labelledby=<frame titleId>]` > frame.
- In M0 only `entity`/`tile` is registered, in `widgets/hd-w-misc.js`.

### C4 Lab URL (`/testdata/homedash-lab.html?…`)

| Param | Values (default) |
|---|---|
| family | glass\|tactile\|minimal\|instrument\|placeholder (placeholder). `placeholder` is painted only by `ui/testdata/homedash-lab.css`. |
| palette | theme\|dark\|light\|custom (dark). `custom` uses solver input `{accent:'#8E5BD9', surface_hue:285, scheme}`. |
| scheme | dark\|light. Forced to dark/light for those palettes; otherwise the param, default dark. |
| theme | desktop theme id, only with palette=theme (standard). The lab puts it on `#hd-lab-scope.desktop-body[data-theme][data-scheme][data-accent=teal][data-animations]` and loads `/css/desktop-shell.bundle.css`. |
| types | comma list of primitive names or widget types; the two name sets do not overlap (all 20 primitives) |
| variants | comma list, a filter (per-type defaults) |
| states | off,on,pending,disabled,unavailable,stale,readonly,error,alert,min,max,focus (all) |
| sizes | `m`/`l` for primitives, `WxH` for widgets (m; widget defaults) |
| dpr | number; the gallery reports an error if it differs from `devicePixelRatio` (1) |
| motion | full\|reduced\|none (full), written as `data-hd-motion`. Go emulates `prefers-reduced-motion: reduce` unless full. |
| lang | (en) |

`window.hdLab` exposes:
- `ready`, `error`, `errors`
- `layout()` returns `{width, height, rows:[{top,bottom}]}` in document px. The first row's top is the panel top; the last row's bottom is the panel bottom.
- `probes()` returns a Promise of the probe report.
- `applyState(name)`
- `seek(ms)` (async)

### C5 Sheets (`HOMEDASH_LAB_SHEETS`; default `prim-placeholder,type-entity-placeholder,themes-placeholder`)
The dark-palette panel is called "dark" below. Every panel uses lang=en.

| Id | Panels |
|---|---|
| `prim-<f>` | all primitives, all states, size m. Four panels: dark; light; theme standard; dark with motion=reduced. |
| `hero-<f>` | types toggle, rocker, knob, needleGauge, arcGauge, numeral, segReadout, sparkline; states off, on, pending, unavailable, alert, focus; sizes m,l. Palettes dark, light, theme standard/fruity(light)/organic/crazy/amiga, each at DPR 1 and 2: 14 panels. |
| `type-<type>-<f>` | `types=<type>`, all states; palettes dark, light, theme standard. |
| `themes-<f>` | all 16 theme variants (14 registry themes; fruity and minimal in light and dark); types toggle, slider, arcGauge, numeral; states off, on, alert. |
| `risk-<f>` | themes fruity(light), lollipop, amiga, crazy, cyberpunk, green_slime; hero types and states. |
| `motion-<hero>-<f>` | one panel, state off, motion full; for each frame at 0, 50, 100, 200, 400 and 800 ms: reload, `applyState('on')`, `seek(ms)`, capture. |
| `side-by-side-<hero>` | four panels (glass, tactile, minimal, instrument), palette dark, hero states. |

**Output:**
- `reports/homedash-lab/<id>.png` when a sheet fits one image; otherwise `<id>-01.png` and onwards.
- Each image is at most 2400 device px per side.
- The test reads from `ui/` and writes to `filepath.Join("..","reports","homedash-lab")`, which is the repository-root `reports/`, ignored by `.gitignore:127`.
- `probes.json` = `{generated, report_only, sheets:[{id, images, panels:[{label, url, probes, errors, oversized}]}], failures}`.

### C6 Probes (each failure is a readable string)

| Probe | Rule |
|---|---|
| contrast/tokens | Pairs measured at the panel root over `--hd-canvas` composited on white. A token that does not resolve is a resolution failure. Graphic pairs under palette=theme are warnings only. |
| contrast/text | Every visible element that owns a text node. Background = composite of ancestor background colours up to the first opaque one; a background image makes the element "unmeasured". Element opacity is multiplied in. Minimum 4.5, or 3 inside `[aria-hidden=true]`. Disabled and unavailable elements are skipped. |
| contrast/graphics | `.hd-dial-arc, .hd-gauge-arc, .hd-gauge-needle-shape, .hd-spark-line, .hd-slider-fill, .hd-bar-fill, .hd-seg7-seg[data-lit="true"], .hd-toggle-thumb, .hd-lamp-dot` against the host element's background, at least 3. |
| hit | `button, [role=slider], [role=switch], [role=radio], [role=spinbutton], [tabindex="0"]` inside `.hd-lab-stage`, at least 44×44 (0.5 px tolerance). |
| shift | Cells with the same `data-hd-lab-control` have the same stage-child size. Live check: `set()` through every state on the first instance must not change its size. |
| animations | With motion other than full: after 2 rAF and 300 ms, no running or pending `document.getAnimations()`. |
| backdrop | Count of elements with a `backdrop-filter` ≤ `.hd-frame` count + 1. |
| resolution | Every palette token resolves. Palette=theme: `--hd-surface-solid` equals `--vd-mat-card-color`. Both HomeDash font faces load. |

Token pairs:
- Text (4.5):
  - ink on surface-solid and on canvas
  - ink-muted on surface-solid and on canvas
  - on-ink on on
  - ink-on-accent on accent
  - selected-ink on selected
  - btn-ink on btn
  - ok/warn/alert/info-ink on surface-solid
- Graphics (3): accent, focus, ok, warn and alert on surface-solid.

`HOMEDASH_LAB_REPORT_ONLY=1` turns failures into report entries.

## Task 0: Workspace

- [ ] From the main checkout:
  - `git worktree add .worktrees/homedash-m0-frontend -b claude/homedash-m0-frontend main`
  - Then, in PowerShell: `robocopy <main>\node_modules <wt>\node_modules /E /MT:16 /NFL /NDL /NJH /NP`. Use a real copy; a junction breaks the Storymaker vendor check.
- [ ] Baseline runs in the worktree:
  - `npm run check:ui` passes.
  - `go test ./ui -run '^TestVirtualDesktop(FirstPartyJSFilesStayBelowLineBudget|DesktopAppMaterialsAdoptedByApps)$' -count=1` passes. Use a backslash `GOTMPDIR` such as `D:\tmp\go-build`, and prefix `GOTOOLCHAIN=go1.27.4` if vet is missing.
  - Write down the run times.
- [ ] Before any browser run, check for leftover rod Chromes (`Temp\rod\user-data`). Ask before stopping any.

## Task 1: Test runner, harness, fake DOM, hd-core

**Files:** `scripts/test-homedash-ui.mjs`, `scripts/homedash-tests/harness.mjs`, `scripts/homedash-tests/dom.mjs`, `scripts/homedash-tests/core.test.mjs`, `ui/js/homedash/hd-core.js`, `package.json`.
**Read first:** frontend part 1.1; `scripts/test-docker-manager-ui.mjs:1-25` (vm pattern).

- [ ] Runner `scripts/test-homedash-ui.mjs`:
```js
// HomeDash engine Node tests and guards. Run: npm run test:homedash. Every scripts/homedash-tests/*.test.mjs exports
// `tests`, a list of [name, fn] pairs (fn may be async).
import fs from 'node:fs'; import path from 'node:path'; import { pathToFileURL } from 'node:url';
const dir = path.join(import.meta.dirname, 'homedash-tests');
let failures = 0, count = 0;
for (const file of fs.readdirSync(dir).filter(f => f.endsWith('.test.mjs')).sort()) {
  const { tests } = await import(pathToFileURL(path.join(dir, file)).href);
  for (const [name, fn] of tests) {
    count++;
    try { await fn(); console.log(`PASS ${file}: ${name}`); } catch (e) { failures++; console.error(`FAIL ${file}: ${name}: ${e && e.stack || e}`); }
  }
}
console.log(`homedash: ${count - failures}/${count} passed`);
if (failures || !count) process.exitCode = 1;
```
- [ ] `package.json` scripts: add `"test:homedash": "node scripts/test-homedash-ui.mjs"`.
- [ ] `scripts/homedash-tests/dom.mjs`, a minimal DOM double (full code):
```js
// Minimal DOM double for HomeDash Node tests: createElement(NS), attributes, dataset, style.setProperty, textContent,
// append/remove, simple selectors (tag, .class, [attr], [attr="v"], descendant), listeners with signal.
const kebab = s => s.replace(/[A-Z]/g, c => '-' + c.toLowerCase());
class Text { constructor(doc, t) { this.nodeType = 3; this.ownerDocument = doc; this._t = String(t); this.parentNode = null; }
  get textContent() { return this._t; } set textContent(v) { this._t = String(v); } remove() { if (this.parentNode) this.parentNode._drop(this); } }
class Style { constructor() { this._p = new Map(); } setProperty(n, v) { this._p.set(n, String(v)); }
  getPropertyValue(n) { return this._p.has(n) ? this._p.get(n) : ''; } removeProperty(n) { const v = this.getPropertyValue(n); this._p.delete(n); return v; } }
function matchCompound(el, c) {
  const tag = /^[a-zA-Z][\w-]*/.exec(c); if (tag && el.localName.toLowerCase() !== tag[0].toLowerCase()) return false;
  for (const m of c.matchAll(/\.([\w-]+)/g)) if (!el.classList.contains(m[1])) return false;
  for (const m of c.matchAll(/\[([\w-]+)(?:="([^"]*)")?\]/g)) { if (!el.hasAttribute(m[1])) return false; if (m[2] !== undefined && el.getAttribute(m[1]) !== m[2]) return false; }
  return true;
}
function matchComplex(el, sel) {
  const parts = sel.trim().split(/\s+/); if (!matchCompound(el, parts.pop())) return false;
  let node = el.parentNode;
  for (let i = parts.length - 1; i >= 0; i--) { while (node && node.nodeType === 1 && !matchCompound(node, parts[i])) node = node.parentNode; if (!node || node.nodeType !== 1) return false; node = node.parentNode; }
  return true;
}
export class FakeElement {
  constructor(doc, tag, ns) {
    this.nodeType = 1; this.ownerDocument = doc; this.namespaceURI = ns || 'http://www.w3.org/1999/xhtml';
    this.localName = tag; this.tagName = ns && ns.endsWith('/svg') ? tag : tag.toUpperCase();
    this._a = new Map(); this.childNodes = []; this.parentNode = null; this._l = new Map(); this.style = new Style();
    const self = this;
    this.classList = { contains: c => self._cls().includes(c), add: (...c) => self.setAttribute('class', [...new Set([...self._cls(), ...c])].join(' ')),
      remove: (...c) => self.setAttribute('class', self._cls().filter(x => !c.includes(x)).join(' ')),
      toggle: (c, f) => { const on = f === undefined ? !self.classList.contains(c) : !!f; on ? self.classList.add(c) : self.classList.remove(c); return on; } };
    this.dataset = new Proxy({}, { get: (_, k) => self.getAttribute('data-' + kebab(String(k))) ?? undefined,
      set: (_, k, v) => { self.setAttribute('data-' + kebab(String(k)), v); return true; }, deleteProperty: (_, k) => { self.removeAttribute('data-' + kebab(String(k))); return true; } });
  }
  _cls() { return (this.getAttribute('class') || '').split(/\s+/).filter(Boolean); }
  get children() { return this.childNodes.filter(n => n.nodeType === 1); }
  get firstElementChild() { return this.children[0] || null; }
  get parentElement() { return this.parentNode && this.parentNode.nodeType === 1 ? this.parentNode : null; }
  get id() { return this.getAttribute('id') || ''; } set id(v) { this.setAttribute('id', v); }
  setAttribute(n, v) { this._a.set(String(n), String(v)); } getAttribute(n) { return this._a.has(n) ? this._a.get(n) : null; }
  hasAttribute(n) { return this._a.has(n); } removeAttribute(n) { this._a.delete(n); } getAttributeNames() { return [...this._a.keys()]; }
  toggleAttribute(n, f) { const on = f === undefined ? !this.hasAttribute(n) : !!f; on ? this.setAttribute(n, '') : this.removeAttribute(n); return on; }
  get textContent() { return this.childNodes.map(n => n.textContent).join(''); }
  set textContent(v) { this.replaceChildren(); const s = String(v ?? ''); if (s) this.appendChild(new Text(this.ownerDocument, s)); }
  appendChild(n) { if (n.parentNode) n.parentNode._drop(n); n.parentNode = this; this.childNodes.push(n); return n; }
  append(...ns) { for (const n of ns) this.appendChild(typeof n === 'string' ? new Text(this.ownerDocument, n) : n); }
  insertBefore(n, ref) { if (!ref) return this.appendChild(n); if (n.parentNode) n.parentNode._drop(n); n.parentNode = this; this.childNodes.splice(this.childNodes.indexOf(ref), 0, n); return n; }
  replaceChildren(...ns) { for (const c of this.childNodes) c.parentNode = null; this.childNodes = []; this.append(...ns); }
  remove() { if (this.parentNode) this.parentNode._drop(this); }
  _drop(n) { const i = this.childNodes.indexOf(n); if (i >= 0) this.childNodes.splice(i, 1); n.parentNode = null; }
  matches(sel) { return sel.split(',').some(s => matchComplex(this, s)); }
  closest(sel) { for (let n = this; n && n.nodeType === 1; n = n.parentNode) if (n.matches(sel)) return n; return null; }
  querySelectorAll(sel) { const out = []; const walk = n => { for (const c of n.children) { if (c.matches(sel)) out.push(c); walk(c); } }; walk(this); return out; }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
  addEventListener(t, fn, o) { if (o && o.signal) { if (o.signal.aborted) return; o.signal.addEventListener('abort', () => this.removeEventListener(t, fn)); }
    if (!this._l.has(t)) this._l.set(t, new Set()); this._l.get(t).add(fn); }
  removeEventListener(t, fn) { const s = this._l.get(t); if (s) s.delete(fn); }
  dispatchEvent(ev) { for (const fn of [...(this._l.get(ev.type) || [])]) fn.call(this, ev); return true; }
  getBoundingClientRect() { return { x: 0, y: 0, width: 0, height: 0, top: 0, left: 0, right: 0, bottom: 0 }; }
  focus() { this.ownerDocument.activeElement = this; }
}
export function createDocument() {
  const doc = { activeElement: null };
  doc.createElement = tag => new FakeElement(doc, tag); doc.createElementNS = (ns, tag) => new FakeElement(doc, tag, ns);
  doc.createTextNode = t => new Text(doc, t); doc.body = doc.createElement('body'); doc.documentElement = doc.createElement('html');
  return doc;
}
export function allElements(el) { return [el, ...el.querySelectorAll('*')]; }
```
`querySelectorAll('*')`: `matchCompound` with `*` would fail the tag check. In `matchCompound`, treat `c === '*'` as true by adding `if (c === '*') return true;` as its first line.
- [ ] `scripts/homedash-tests/harness.mjs`:
```js
import fs from 'node:fs'; import path from 'node:path'; import vm from 'node:vm'; import assert from 'node:assert/strict';
import { createDocument, allElements } from './dom.mjs';
export { allElements };
export const root = path.resolve(import.meta.dirname, '..', '..');
export const ENGINE_DIR = 'ui/js/homedash';
export const ENGINE_ORDER = ['hd-core.js', 'hd-format.js', 'hd-colour.js', 'hd-grid.js', 'hd-layout.js', 'hd-doc.js', 'hd-history.js', 'hd-icons.js', 'hd-geometry.js', 'hd-prim-displays.js', 'hd-prim-controls.js', 'hd-widgets.js', 'widgets/hd-w-misc.js'];
export const read = rel => fs.readFileSync(path.join(root, rel), 'utf8');
export const plain = v => (v === undefined ? undefined : JSON.parse(JSON.stringify(v)));
export const eq = (a, e, m) => assert.deepStrictEqual(plain(a), plain(e), m);
export const fc = (...cps) => String.fromCodePoint(...cps);
export function loadEngine(files, { dom = false } = {}) {
  const window = {};
  const context = { window, console, setTimeout, clearTimeout, AbortController, Intl, Map, Set, JSON, Math, Date, URL };
  if (dom) { context.document = createDocument(); window.document = context.document; }
  vm.createContext(context);
  for (const f of files) vm.runInContext(read(`${ENGINE_DIR}/${f}`), context, { filename: f });
  return { E: window.HomeDashEngine, window, document: context.document };
}
export function noStyleAttr(el) { for (const n of allElements(el)) assert.equal(n.getAttribute('style'), null, `style attribute on ${n.localName}`); }
```
- [ ] Failing test `scripts/homedash-tests/core.test.mjs`:
```js
import assert from 'node:assert/strict'; import { loadEngine } from './harness.mjs';
const boot = () => loadEngine(['hd-core.js'], { dom: true });
export const tests = [
  ['loads without a DOM and publishes E.core', () => { const { E } = loadEngine(['hd-core.js']);
    for (const n of ['el','svg','setVars','setAttr','emitter','frame','bag','clamp','uid','randomId','clone','motionReduced','makeT','isTyping']) assert.equal(typeof E.core[n], 'function', n);
    assert.equal(E.el, E.core.el); }],
  ['el/svg build nodes; text stays text; style attributes refused', () => { const { E } = boot();
    const b = E.el('button', { class: 'hd-x', type: 'button', 'aria-checked': true, hidden: false, text: '<b>hi</b>' });
    assert.equal(b.tagName, 'BUTTON'); assert.equal(b.getAttribute('aria-checked'), 'true'); assert.equal(b.hasAttribute('hidden'), false);
    assert.equal(b.textContent, '<b>hi</b>'); assert.equal(b.children.length, 0);
    const s = E.svg('svg', { viewBox: '0 0 24 24' }, [E.svg('path', { d: 'M0 0' }), null, 'x']);
    assert.equal(s.namespaceURI, 'http://www.w3.org/2000/svg'); assert.equal(s.children.length, 1); assert.equal(s.textContent, 'x');
    assert.throws(() => E.el('div', { style: 'color: red' }), /style/);
    const v = E.el('div', { vars: { '--hd-value': 0.5 } }); assert.equal(v.style.getPropertyValue('--hd-value'), '0.5'); assert.equal(v.getAttribute('style'), null);
    assert.throws(() => E.core.setVars(v, { width: '3px' }), /custom properties/);
    E.core.setVars(v, { '--hd-value': null }); assert.equal(v.style.getPropertyValue('--hd-value'), '');
    E.core.setAttr(v, 'data-x', 1); E.core.setAttr(v, 'data-x', false); assert.equal(v.hasAttribute('data-x'), false); }],
  ['makeT fills placeholders; a missing key returns the key and warns once', () => { const { E } = boot(); const w = [];
    const t = E.core.makeT({ 'a.b': 'Hi {{n}} {{n}}' }, { warn: m => w.push(m) });
    assert.equal(t('a.b', { n: 'Ana' }), 'Hi Ana Ana'); assert.equal(t('a.c'), 'a.c'); t('a.c'); assert.equal(w.length, 1); }],
  ['clamp, uid, randomId, clone', () => { const c = boot().E.core;
    assert.equal(c.clamp(5, 0, 3), 3); assert.notEqual(c.uid('x'), c.uid('x')); assert.match(c.randomId('d'), /^d_[a-z0-9]{12}$/);
    const src = { a: [1, { b: 2 }] }, cp = c.clone(src); cp.a[1].b = 3; assert.equal(src.a[1].b, 2); assert.equal(c.clone(undefined), undefined); }],
  ['emitter, frame coalescing, bag', () => { const { E, document } = boot(); const got = [];
    const em = E.core.emitter(); const off = em.on('x', v => got.push(v)); em.emit('x', 1); off(); em.emit('x', 2); assert.deepEqual(got, [1]);
    const q = []; let calls = 0; const f = E.core.frame(() => calls++, { raf: fn => { q.push(fn); return q.length; }, caf: () => {} });
    f(); f(); f(); assert.equal(q.length, 1); q.shift()(); assert.equal(calls, 1); f(); assert.equal(f.pending(), true); f.cancel(); assert.equal(f.pending(), false);
    const b = E.core.bag(), el = document.createElement('div'); let hits = 0, d = 0; b.on(el, 'ping', () => hits++); b.add(() => d++);
    el.dispatchEvent({ type: 'ping' }); b.dispose(); b.dispose(); el.dispatchEvent({ type: 'ping' }); assert.equal(hits, 1); assert.equal(d, 1); assert.equal(b.signal.aborted, true); }],
  ['isTyping and motionReduced', () => { const { E, document } = boot(); const i = document.createElement('input');
    assert.equal(E.core.isTyping(i), true); i.setAttribute('type', 'checkbox'); assert.equal(E.core.isTyping(i), false);
    assert.equal(E.core.isTyping(document.createElement('textarea')), true); assert.equal(E.core.isTyping(null), false);
    const r = document.createElement('div'); r.setAttribute('data-hd-motion', 'none'); assert.equal(E.core.motionReduced(r), true);
    r.setAttribute('data-hd-motion', 'full'); assert.equal(E.core.motionReduced(r), false); }]
];
```
- [ ] Run `npm run test:homedash`. Expected: FAIL with ENOENT for `hd-core.js`.
- [ ] Implement `ui/js/homedash/hd-core.js` (full code):
```js
// HomeDash engine core: DOM builders (no innerHTML, no style attributes), emitter, rAF coalescer, AbortController bag,
// ids, clone, motion and typing checks, translator. Creates window.HomeDashEngine; the other engine files extend it.
(function () {
    'use strict';
    const E = window.HomeDashEngine = window.HomeDashEngine || {};
    const SVG_NS = 'http://www.w3.org/2000/svg';
    let counter = 0;
    function setVars(node, vars) {
        for (const [name, value] of Object.entries(vars || {})) {
            if (!name.startsWith('--')) throw new Error('hd-core: setVars writes custom properties only, got ' + name);
            if (value === null || value === undefined) node.style.removeProperty(name); else node.style.setProperty(name, String(value));
        }
        return node;
    }
    function setAttr(node, name, value) {
        if (name === 'style') throw new Error('hd-core: no style attributes; use setVars');
        if (value === null || value === undefined || value === false) node.removeAttribute(name);
        else node.setAttribute(name, value === true ? 'true' : String(value));
        return node;
    }
    function applyAttrs(node, attrs) {
        for (const [key, value] of Object.entries(attrs || {})) {
            if (key === 'text') node.textContent = value == null ? '' : String(value);
            else if (key === 'vars') setVars(node, value); else setAttr(node, key, value);
        }
        return node;
    }
    function append(node, children) {
        for (const child of [].concat(children == null ? [] : children)) {
            if (child === null || child === undefined || child === false) continue;
            node.appendChild(typeof child === 'string' ? document.createTextNode(child) : child);
        }
        return node;
    }
    const el = (tag, attrs, children) => append(applyAttrs(document.createElement(tag), attrs), children);
    const svg = (tag, attrs, children) => append(applyAttrs(document.createElementNS(SVG_NS, tag), attrs), children);
    function emitter() {
        const map = new Map();
        const api = {
            on(type, fn) { if (!map.has(type)) map.set(type, new Set()); map.get(type).add(fn); return () => api.off(type, fn); },
            off(type, fn) { const s = map.get(type); if (s) s.delete(fn); },
            emit(type, payload) { for (const fn of [...(map.get(type) || [])]) fn(payload); }
        };
        return api;
    }
    function frame(fn, opts) {
        const raf = (opts && opts.raf) || (typeof requestAnimationFrame === 'function' ? requestAnimationFrame : cb => setTimeout(cb, 16));
        const caf = (opts && opts.caf) || (typeof cancelAnimationFrame === 'function' ? cancelAnimationFrame : clearTimeout);
        let handle = null;
        const schedule = () => { if (handle === null) handle = raf(() => { handle = null; fn(); }); };
        schedule.cancel = () => { if (handle !== null) { caf(handle); handle = null; } };
        schedule.pending = () => handle !== null;
        return schedule;
    }
    function bag() {
        const controller = new AbortController(), disposers = [];
        return {
            signal: controller.signal,
            on(target, type, fn, options) { target.addEventListener(type, fn, Object.assign({}, options, { signal: controller.signal })); },
            add(fn) { disposers.push(fn); return fn; },
            dispose() { if (controller.signal.aborted) return; controller.abort(); while (disposers.length) { try { disposers.pop()(); } catch (err) { console.error(err); } } }
        };
    }
    const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));
    const uid = prefix => (prefix || 'hd') + '-' + (++counter).toString(36);
    function randomId(prefix, length) {
        const n = length || 12, abc = 'abcdefghijklmnopqrstuvwxyz0123456789', bytes = new Uint8Array(n);
        if (typeof crypto !== 'undefined' && crypto.getRandomValues) crypto.getRandomValues(bytes); else for (let i = 0; i < n; i++) bytes[i] = Math.floor(Math.random() * 256);
        let out = ''; for (const b of bytes) out += abc[b % abc.length];
        return (prefix ? prefix + '_' : '') + out;
    }
    const clone = v => (v === undefined ? undefined : JSON.parse(JSON.stringify(v)));
    function motionReduced(root) {
        const mode = root && root.getAttribute ? root.getAttribute('data-hd-motion') : null;
        if (mode === 'reduced' || mode === 'none') return true;
        if (mode === 'full') return false;
        return typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
    }
    function makeT(dict, options) {
        const warn = (options && options.warn) || (m => console.warn(m)), warned = new Set();
        return function translate(key, vars) {
            const has = dict && Object.prototype.hasOwnProperty.call(dict, key) && dict[key] != null;
            if (!has) { if (!warned.has(key)) { warned.add(key); warn('HomeDash: missing translation ' + key); } return key; }
            let text = String(dict[key]);
            for (const [name, value] of Object.entries(vars || {})) text = text.split('{{' + name + '}}').join(String(value));
            return text;
        };
    }
    function isTyping(target) {
        if (!target || target.nodeType !== 1) return false;
        if (target.isContentEditable) return true;
        const tag = String(target.tagName || '').toLowerCase();
        if (tag === 'textarea' || tag === 'select') return true;
        if (tag !== 'input') return false;
        return !['checkbox', 'radio', 'button', 'submit', 'reset', 'range', 'color', 'file', 'image'].includes(String(target.getAttribute('type') || 'text').toLowerCase());
    }
    E.core = Object.freeze({ SVG_NS, el, svg, append, applyAttrs, setVars, setAttr, emitter, frame, bag, clamp, uid, randomId, clone, motionReduced, makeT, isTyping });
    E.el = el; E.svg = svg;
})();
```
- [ ] Add `if (c === '*') return true;` to `matchCompound` in dom.mjs.
- [ ] Run `npm run test:homedash`. Expected: `homedash: 6/6 passed`.
- [ ] `git add scripts/test-homedash-ui.mjs scripts/homedash-tests package.json ui/js/homedash/hd-core.js` then `git commit -m "feat(homedash): engine core and Node test harness"`.

**Done when:** core tests pass and no DOM is touched at load.

## Task 2: hd-format.js

**Files:** `ui/js/homedash/hd-format.js`, `scripts/homedash-tests/format.test.mjs`.

- [ ] Failing test:
```js
import assert from 'node:assert/strict'; import { loadEngine, fc } from './harness.mjs';
const F = () => loadEngine(['hd-core.js', 'hd-format.js']).E.format;
const DASH = fc(0x2013), NBSP = fc(0xa0), DEG = fc(0xb0);
export const tests = [
  ['number and split are locale-correct and tabular-ready', () => { const f = F();
    assert.equal(f.number(1234.5, { decimals: 1, lang: 'en' }), '1,234.5'); assert.equal(f.number(21.53), '21.5'); assert.equal(f.number(150.7), '151'); assert.equal(f.number(5), '5'); assert.equal(f.number(null), '');
    assert.deepEqual({ ...f.split(1234.5, { decimals: 1, lang: 'en', unit: 'W' }) }, { int: '1,234', frac: '.5', unit: 'W', empty: false });
    assert.deepEqual({ ...f.split(1234.5, { decimals: 1, lang: 'de' }) }, { int: '1.234', frac: ',5', unit: '', empty: false });
    assert.equal(f.split(-3.2, { decimals: 1 }).int, '-3'); assert.deepEqual({ ...f.split(null, { unit: '%' }) }, { int: DASH, frac: '', unit: '%', empty: true }); }],
  ['valueText, percent, relTime, duration', () => { const f = F();
    assert.equal(f.valueText(21.5, { unit: DEG + 'C' }), '21.5' + NBSP + DEG + 'C'); assert.equal(f.valueText(null), DASH);
    assert.equal(f.valueText(4000, { decimals: 0, grouping: false, unit: 'K' }), '4000' + NBSP + 'K'); assert.equal(f.percent(0.5, { lang: 'en' }), '50%');
    const now = 1_800_000_000_000; assert.equal(f.relTime(now - 120000, now, 'en'), '2 minutes ago'); assert.equal(f.relTime(now, now, 'en'), 'now');
    const u = (unit, n) => new Intl.NumberFormat('en', { style: 'unit', unit, unitDisplay: 'narrow' }).format(n);
    assert.equal(f.duration(3725, 'en'), u('hour', 1) + ' ' + u('minute', 2)); assert.equal(f.duration(0, 'en'), u('second', 0)); }],
  ['isOn, stateWord, stateText', () => { const f = F();
    assert.equal(f.isOn({ value: 'on' }), true); assert.equal(f.isOn({ value: 'closed' }), false); assert.equal(f.isOn({ value: 21 }), null);
    const w = v => f.stateWord(v);
    assert.equal(w(null), 'unavailable'); assert.equal(w({ available: false }), 'unavailable'); assert.equal(w({ status: 'unknown' }), 'unknown');
    assert.equal(w({ status: 'error', value: 'on' }), 'error'); assert.equal(w({ pending: true, value: 'on' }), 'pending'); assert.equal(w({ value: 'off' }), 'off'); assert.equal(w({ value: 3 }), 'value');
    assert.equal(f.stateText({ value: 'on' }, k => '[' + k + ']'), '[homedash.state.on]'); assert.equal(f.stateText({ value: 3 }, k => k), ''); }]
];
```
- [ ] Run it. Expected: FAIL (file missing).
- [ ] Implement:
```js
// HomeDash formatting (pure): numbers and units via Intl in the document language, tabular split into int/frac/unit,
// relative time, durations and state words.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-format.js: load hd-core.js first');
    const DASH = String.fromCodePoint(0x2013), NBSP = String.fromCodePoint(0xa0), STATE_PREFIX = 'homedash.state.';
    const ON = new Set(['on', 'open', 'opening', 'unlocked', 'playing', 'home', 'true']);
    const OFF = new Set(['off', 'closed', 'closing', 'locked', 'idle', 'paused', 'standby', 'not_home', 'false']);
    const cache = new Map();
    const isNum = v => typeof v === 'number' && Number.isFinite(v);
    function nf(lang, d, grouping) {
        const key = (lang || 'en') + '|' + d + '|' + grouping;
        if (!cache.has(key)) cache.set(key, new Intl.NumberFormat(lang || 'en', { minimumFractionDigits: d, maximumFractionDigits: d, useGrouping: grouping }));
        return cache.get(key);
    }
    const decimalsOf = (v, o) => (o.decimals === undefined || o.decimals === null ? (Number.isInteger(v) ? 0 : Math.abs(v) < 100 ? 1 : 0) : o.decimals);
    function number(value, opts) { const o = opts || {}; return isNum(value) ? nf(o.lang, decimalsOf(value, o), o.grouping !== false).format(value) : ''; }
    function split(value, opts) {
        const o = opts || {}, unit = o.unit ? String(o.unit) : '';
        if (!isNum(value)) return { int: DASH, frac: '', unit, empty: true };
        let int = '', frac = '';
        for (const p of nf(o.lang, decimalsOf(value, o), o.grouping !== false).formatToParts(value)) { if (p.type === 'decimal' || p.type === 'fraction') frac += p.value; else int += p.value; }
        return { int, frac, unit, empty: false };
    }
    function valueText(value, opts) { const t = number(value, opts); if (!t) return DASH; const u = opts && opts.unit; return u ? t + NBSP + u : t; }
    const percent = (fr, opts) => (isNum(fr) ? new Intl.NumberFormat((opts && opts.lang) || 'en', { style: 'percent', maximumFractionDigits: 0 }).format(fr) : DASH);
    function relTime(then, now, lang) {
        if (!isNum(then) || !isNum(now)) return '';
        const s = Math.round((then - now) / 1000), a = Math.abs(s), r = new Intl.RelativeTimeFormat(lang || 'en', { numeric: 'auto' });
        if (a < 60) return r.format(s, 'second'); if (a < 3600) return r.format(Math.round(s / 60), 'minute');
        if (a < 86400) return r.format(Math.round(s / 3600), 'hour'); return r.format(Math.round(s / 86400), 'day');
    }
    function duration(seconds, lang) {
        if (!isNum(seconds) || seconds < 0) return DASH;
        let rest = Math.floor(seconds); const parts = [];
        for (const [unit, size] of [['day', 86400], ['hour', 3600], ['minute', 60], ['second', 1]]) {
            const n = Math.floor(rest / size); rest -= n * size;
            if (n > 0 || (unit === 'second' && !parts.length)) parts.push(new Intl.NumberFormat(lang || 'en', { style: 'unit', unit, unitDisplay: 'narrow' }).format(n));
            if (parts.length === 2) break;
        }
        return parts.join(' ');
    }
    function isOn(view) {
        if (!view) return null; const v = view.value;
        if (v === true || v === false) return v;
        const s = typeof v === 'string' ? v.toLowerCase() : null;
        return s && ON.has(s) ? true : s && OFF.has(s) ? false : null;
    }
    function stateWord(view) {
        if (!view || view.available === false || view.status === 'unavailable') return 'unavailable';
        if (view.status === 'unknown') return 'unknown'; if (view.status === 'error') return 'error'; if (view.pending) return 'pending';
        const on = isOn(view); return on === true ? 'on' : on === false ? 'off' : 'value';
    }
    const stateText = (view, t) => { const w = stateWord(view); return w === 'value' ? '' : t(STATE_PREFIX + w); };
    E.format = Object.freeze({ DASH, NBSP, STATE_PREFIX, number, split, valueText, percent, relTime, duration, isOn, stateWord, stateText });
})();
```
- [ ] Run it. Expected: PASS. Commit `feat(homedash): number, unit and state formatting`.

## Task 3: hd-grid.js

**Files:** `ui/js/homedash/hd-grid.js`, `scripts/homedash-tests/grid.test.mjs`.
**Read first:** frontend part 4.1–4.2.

- [ ] Failing test:
```js
import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs';
const G = () => loadEngine(['hd-core.js', 'hd-grid.js']).E.grid;
const it = (id, x, y, w, h) => ({ id, x, y, w, h });
const map = L => Object.fromEntries(L.map(i => [i.id, [i.x, i.y, i.w, i.h]]));
export const tests = [
  ['collides', () => { const g = G(); assert.equal(g.collides(it('a',0,0,2,2), it('b',2,0,1,1)), false); assert.equal(g.collides(it('a',0,0,2,2), it('b',1,1,1,1)), true); assert.equal(g.collides(it('a',0,0,2,2), it('a',0,0,2,2)), false); }],
  ['compact lifts, keeps gaps without gravity, repairs overlaps', () => { const g = G(); const L = [it('a',0,0,2,2), it('b',2,0,2,1), it('c',0,5,1,1), it('d',3,4,1,1)];
    eq(map(g.compact(L)), { a: [0,0,2,2], b: [2,0,2,1], c: [0,2,1,1], d: [3,1,1,1] }); eq(map(g.compact(L, false)), map(L));
    eq(map(g.compact([it('a',0,0,2,2), it('b',1,1,2,2)])), { a: [0,0,2,2], b: [1,2,2,2] }); assert.equal(L[2].y, 5, 'input untouched'); }],
  ['pushDown cascades and terminates', () => { const g = G();
    eq(map(g.pushDown([it('a',0,1,2,1), it('b',0,1,2,1), it('c',0,2,2,1)], ['a'])), { a: [0,1,2,1], b: [0,2,2,1], c: [0,3,2,1] });
    const big = [it('m',0,0,1,1)]; for (let i = 0; i < 60; i++) big.push(it('i' + i, 0, i, 1, 1)); assert.equal(map(g.pushDown(big, ['m'])).i59[1], 60); }],
  ['moveItems: swap, edge clamp, group bbox, push without compaction', () => { const g = G(); const L = [it('a',0,0,2,2), it('b',2,0,2,2), it('c',4,0,2,2)];
    eq(map(g.moveItems(L, ['a'], { x: 2, y: 0 }, 12)), { a: [2,0,2,2], b: [0,0,2,2], c: [4,0,2,2] });
    eq(map(g.moveItems(L, ['c'], { x: 20, y: 0 }, 12)).c, [10,0,2,2]);
    eq(map(g.moveItems(L, ['a','b'], { x: 6, y: 0 }, 12)), { a: [6,0,2,2], b: [8,0,2,2], c: [4,0,2,2] });
    const P = [it('a',0,0,2,1), it('b',0,1,4,1)];
    eq(map(g.moveItems(P, ['a'], { x: 0, y: 1 }, 12, { compact: false })), { a: [0,1,2,1], b: [0,2,4,1] }); eq(map(g.moveItems(P, ['a'], { x: 0, y: 1 }, 12)), { a: [0,0,2,1], b: [0,1,4,1] }); }],
  ['resizeItem: push, min/max, opposite edge fixed, all eight edges', () => { const g = G();
    eq(map(g.resizeItem([it('a',0,0,2,2), it('b',2,0,2,2)], 'a', 'e', { dx: 1, dy: 0 }, 12)), { a: [0,0,3,2], b: [2,2,2,2] });
    eq(map(g.resizeItem([it('a',0,0,2,2)], 'a', 'w', { dx: 5, dy: 0 }, 12, { minW: 1 }, { compact: false })).a, [1,0,1,2]);
    eq(map(g.resizeItem([it('a',0,0,2,2)], 'a', 'n', { dx: 0, dy: -3 }, 12, { maxH: 3 }, { compact: false })).a, [0,0,2,2]);
    eq(map(g.resizeItem([it('a',0,0,2,2)], 'a', 'se', { dx: 1, dy: 1 }, 12, { maxW: 2 })).a, [0,0,2,3]);
    eq(map(g.resizeItem([it('a',10,0,2,1)], 'a', 'e', { dx: 3, dy: 0 }, 12)).a, [10,0,2,1]);
    const want = { n: [4,5,2,1], ne: [4,5,3,1], e: [4,4,3,2], se: [4,4,3,3], s: [4,4,2,3], sw: [5,4,1,3], w: [5,4,1,2], nw: [5,5,1,1] };
    for (const [edge, r] of Object.entries(want)) eq(map(g.resizeItem([it('z',4,4,2,2)], 'z', edge, { dx: 1, dy: 1 }, 12, {}, { compact: false })).z, r, edge); }],
  ['place: first fit, row-major', () => { const g = G();
    eq(g.place([it('a',0,0,12,1)], 2, 1, 12), { x: 0, y: 1 }); eq(g.place([it('a',0,0,4,1), it('b',6,0,6,1)], 2, 1, 12), { x: 4, y: 0 });
    eq(g.place([it('a',0,0,4,1), it('b',6,0,6,1)], 3, 1, 12), { x: 0, y: 1 }); eq(g.place([], 2, 1, 12, 3), { x: 0, y: 3 }); eq(g.place([], 20, 1, 12), { x: 0, y: 0 }); }],
  ['repair clamps, drops duplicates and unknown ids', () => { const g = G();
    eq(map(g.repair([{ id: 'a', x: 11, y: -2, w: 2, h: 1 }, it('a',0,0,1,1), it('zz',0,0,1,1), { id: 'b', x: '3', y: null, w: 20, h: 0 }], 12, { b: { minW: 2, maxW: 4, minH: 1, maxH: 2 } }, { ids: new Set(['a','b']) })), { a: [10,0,2,1], b: [3,0,4,1] }); }]
];
```
- [ ] Run it. Expected: FAIL.
- [ ] Implement:
```js
// HomeDash grid engine (pure; frontend part 4.2). Items {id,x,y,w,h} in cells; functions return new arrays.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-grid.js: load hd-core.js first');
    const clamp = E.core.clamp, PROBE = {};
    const item = i => ({ id: i.id, x: i.x, y: i.y, w: i.w, h: i.h });
    const collides = (a, b) => a.id !== b.id && a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h;
    const sortReading = L => L.slice().sort((a, b) => a.y - b.y || a.x - b.x || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
    function compact(L, gravity) {
        const placed = [];
        for (const src of sortReading(L)) {
            const it = item(src);
            if (gravity !== false) while (it.y > 0 && !placed.some(p => collides(p, Object.assign({}, it, { y: it.y - 1 })))) it.y--;
            while (placed.some(p => collides(p, it))) it.y++;
            placed.push(it);
        }
        return sortReading(placed);
    }
    function pushDown(L, moverIds) {
        const out = L.map(item), fixedIds = new Set(moverIds), fixed = out.filter(i => fixedIds.has(i.id)), queue = fixed.slice();
        let guard = 0;
        while (queue.length) {
            if (++guard > 100000) throw new Error('hd-grid: pushDown did not settle');
            const m = queue.shift();
            for (const o of sortReading(out)) {
                if (fixedIds.has(o.id) || !collides(m, o)) continue;
                o.y = m.y + m.h;
                for (let moved = true; moved;) { moved = false; for (const f of fixed) if (collides(f, o)) { o.y = f.y + f.h; moved = true; } }
                queue.push(o);
            }
        }
        return out;
    }
    function bbox(S) {
        const x = Math.min(...S.map(s => s.x)), y = Math.min(...S.map(s => s.y));
        return { x, y, w: Math.max(...S.map(s => s.x + s.w)) - x, h: Math.max(...S.map(s => s.y + s.h)) - y };
    }
    const settle = (L, o) => (o && o.compact === false ? L : compact(L));
    function moveItems(L, ids, target, cols, opts) {
        const idSet = new Set(ids), out = L.map(item), S = out.filter(i => idSet.has(i.id));
        if (!S.length) return settle(out, opts);
        const B = bbox(S), dx = clamp(target.x, 0, Math.max(0, cols - B.w)) - B.x, dy = Math.max(0, target.y) - B.y;
        if (!dx && !dy) return settle(out, opts);
        for (const s of S) { s.x += dx; s.y += dy; }
        if (S.length === 1) {
            const s = S[0], cx = s.x + s.w / 2, cy = s.y + s.h / 2, origin = L.find(i => i.id === s.id);
            const t = out.find(o => !idSet.has(o.id) && o.w === s.w && o.h === s.h && cx >= o.x && cx < o.x + o.w && cy >= o.y && cy < o.y + o.h);
            if (t) { t.x = origin.x; t.y = origin.y; return settle(out, opts); }
        }
        return settle(pushDown(out, S.map(s => s.id)), opts);
    }
    function resizeItem(L, id, edge, d, cols, c, opts) {
        const out = L.map(item), r = out.find(i => i.id === id);
        if (!r) return settle(out, opts);
        const k = c || {}, minW = Math.max(1, k.minW || 1), minH = Math.max(1, k.minH || 1);
        const maxW = Math.max(minW, Math.min(k.maxW || cols, cols)), maxH = Math.max(minH, k.maxH || Infinity);
        const right = r.x + r.w, bottom = r.y + r.h, dx = (d && d.dx) || 0, dy = (d && d.dy) || 0;
        let { x, y, w, h } = r;
        if (edge.includes('e')) w += dx;
        if (edge.includes('w')) { x += dx; w -= dx; }
        if (edge.includes('s')) h += dy;
        if (edge.includes('n')) { y += dy; h -= dy; }
        w = clamp(w, minW, maxW); h = clamp(h, minH, maxH);
        if (edge.includes('e') && !edge.includes('w')) w = clamp(Math.min(w, cols - x), minW, maxW);
        if (edge.includes('w')) { x = right - w; if (x < 0) { x = 0; w = clamp(right, minW, maxW); } }
        if (edge.includes('n')) { y = bottom - h; if (y < 0) { y = 0; h = clamp(bottom, minH, maxH); } }
        Object.assign(r, { x: clamp(x, 0, Math.max(0, cols - w)), y: Math.max(0, y), w, h });
        return settle(pushDown(out, [id]), opts);
    }
    function place(L, w, h, cols, startY) {
        const ww = Math.min(Math.max(1, w), cols), hh = Math.max(1, h), from = Math.max(0, startY || 0);
        const bottom = Math.max(from, L.reduce((m, i) => Math.max(m, i.y + i.h), 0));
        for (let y = from; y <= bottom; y++) for (let x = 0; x <= cols - ww; x++) { const p = { id: PROBE, x, y, w: ww, h: hh }; if (!L.some(i => collides(i, p))) return { x, y }; }
        return { x: 0, y: bottom };
    }
    function repair(L, cols, C, opts) {
        const known = opts && opts.ids, seen = new Set(), out = [];
        const int = (v, d) => (Number.isFinite(Number(v)) ? Math.round(Number(v)) : d);
        for (const raw of Array.isArray(L) ? L : []) {
            if (!raw || typeof raw.id !== 'string' || seen.has(raw.id) || (known && !known.has(raw.id))) continue;
            seen.add(raw.id);
            const c = (typeof C === 'function' ? C(raw.id) : C && C[raw.id]) || {};
            const w = clamp(int(raw.w, 1), Math.max(1, c.minW || 1), Math.min(c.maxW || cols, cols)), h = clamp(int(raw.h, 1), Math.max(1, c.minH || 1), c.maxH || Infinity);
            out.push({ id: raw.id, x: clamp(int(raw.x, 0), 0, cols - w), y: Math.max(0, int(raw.y, 0)), w, h });
        }
        return compact(out, !(opts && opts.compact === false));
    }
    E.grid = Object.freeze({ collides, sortReading, compact, pushDown, bbox, moveItems, resizeItem, place, repair });
})();
```
- [ ] Run it. Expected: PASS. Commit `feat(homedash): pure grid engine`.

## Task 4: hd-layout.js

**Files:** `ui/js/homedash/hd-layout.js`, `scripts/homedash-tests/layout.test.mjs`. **Read first:** frontend part 4.3–4.4.

- [ ] Failing test:
```js
import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs';
const Lx = () => loadEngine(['hd-core.js', 'hd-grid.js', 'hd-layout.js']).E.layout;
const it = (id, x, y, w, h) => ({ id, x, y, w, h }); const map = L => Object.fromEntries(L.map(i => [i.id, [i.x, i.y, i.w, i.h]]));
const LG = [it('a',0,0,6,2), it('b',6,0,6,2), it('c',0,2,3,1)];
export const tests = [
  ['derive md/sm in reading order with the phone full-width rule and constraints', () => { const l = Lx();
    eq(map(l.derive(LG, 8)), { a: [0,0,4,2], b: [4,0,4,2], c: [0,2,2,1] }); eq(map(l.derive(LG, 4)), { a: [0,0,4,2], b: [0,2,4,2], c: [0,4,1,1] });
    eq(map(l.derive(LG, 4, { c: { minW: 2 } })).c, [0,4,2,1]); }],
  ['effective uses overrides and memoises on the lg identity', () => { const l = Lx(); const lay = { lg: LG, md: null, sm: [it('a',0,0,1,1)] };
    assert.equal(l.effective(lay, 'md'), l.effective(lay, 'md')); assert.equal(l.effective(lay, 'sm'), lay.sm); assert.equal(l.effective(lay, 'lg'), LG); }],
  ['breakpoint with 16 px hysteresis', () => { const b = Lx().breakpoint;
    assert.equal(b(950, 'lg'), 'lg'); assert.equal(b(943, 'lg'), 'md'); assert.equal(b(959, 'md'), 'md'); assert.equal(b(960, 'md'), 'lg');
    assert.equal(b(590, 'md'), 'md'); assert.equal(b(583, 'md'), 'sm'); assert.equal(b(500, 'lg'), 'sm'); assert.equal(b(1200), 'lg'); }],
  ['metrics, fit fallback, css vars, gaps, rows', () => { const l = Lx(); const r = v => Math.round(v * 1000) / 1000;
    const s = l.metrics(1180, 780, 12, 6, 12, 'auto', 'scroll'); eq([s.mode, r(s.colW), r(s.rowH), s.offX], ['scroll', 85.333, 85.333, 0]);
    assert.equal(l.metrics(1180, 780, 12, 6, 12, 100, 'scroll').rowH, 100);
    const f = l.metrics(1180, 780, 12, 6, 12, 'auto', 'fit_screen'); eq([f.mode, r(f.colW), r(f.offX)], ['fit', 85.333, 12]);
    assert.equal(l.metrics(1180, 300, 12, 10, 12, 'auto', 'fit_screen').mode, 'overflow'); assert.equal(l.metrics(400, 780, 12, 6, 12, 'auto', 'scroll').rowH, 56);
    eq(l.cssVars(s, 12, 6, 12), { '--hd-cols': 12, '--hd-rows': 6, '--hd-colw': '85.33px', '--hd-rowh': '85.33px', '--hd-gap': '12px', '--hd-offx': '0px' });
    assert.equal(l.gapFor('compact'), 8); assert.equal(l.gapFor('spacious', null), 16); assert.equal(l.gapFor('comfortable', 20), 20); assert.equal(l.rows([it('a',0,0,1,3), it('b',0,5,1,1)]), 6); }],
  ['override maintenance on add, delete, materialise, reset', () => { const l = Lx();
    const lay = { lg: [it('a',0,0,12,1)], md: null, sm: [it('a',0,0,4,1)] }; const added = l.addToLayouts(lay, { id: 'n', w: 4, h: 2 });
    eq(map(added.lg).n, [0,1,4,2]); assert.equal(added.md, null); eq(map(added.sm).n, [0,1,1,2]);
    const removed = l.removeFromLayouts(added, ['n']); eq(map(removed.lg), { a: [0,0,12,1] }); eq(map(removed.sm), { a: [0,0,4,1] });
    const mat = l.materialiseOverride({ lg: LG, md: null, sm: null }, 'md'); eq(map(mat.md), map(l.derive(LG, 8))); assert.equal(l.resetOverride(mat, 'md').md, null); }]
];
```
- [ ] Run it. Expected: FAIL. Implement:
```js
// HomeDash layout (pure): breakpoints, md/sm derivation, overrides, cell metrics and fit-to-screen (frontend 4.3-4.4).
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.grid) throw new Error('hd-layout.js: load hd-grid.js first');
    const G = E.grid, clamp = E.core.clamp;
    const COLS = Object.freeze({ lg: 12, md: 8, sm: 4 }), BP_MIN = Object.freeze({ md: 600, lg: 960 }), HYSTERESIS = 16;
    const GAPS = Object.freeze({ compact: 8, comfortable: 12, spacious: 16 });
    let memo = new WeakMap();
    const constraintOf = (C, id) => (typeof C === 'function' ? C(id) : C && C[id]) || {};
    function derivedSize(item, toCols, c, fromCols) {
        const from = fromCols || COLS.lg, k = c || {};
        let w = Math.max(1, Math.round(item.w * toCols / from));
        if (toCols === COLS.sm && item.w >= from / 2) w = toCols;
        return { w: clamp(w, k.minW || 1, Math.min(k.maxW || toCols, toCols)), h: clamp(item.h, k.minH || 1, k.maxH || Infinity) };
    }
    function derive(lg, toCols, C) {
        const out = []; let lastY = 0;
        for (const it of G.sortReading(lg)) { const s = derivedSize(it, toCols, constraintOf(C, it.id)); const p = G.place(out, s.w, s.h, toCols, lastY); out.push({ id: it.id, x: p.x, y: p.y, w: s.w, h: s.h }); lastY = p.y; }
        return G.compact(out);
    }
    function effective(layouts, bp, C) {
        if (bp === 'lg') return layouts.lg;
        if (Array.isArray(layouts[bp])) return layouts[bp];
        let entry = memo.get(layouts.lg); if (!entry) { entry = new Map(); memo.set(layouts.lg, entry); }
        if (!entry.has(bp)) entry.set(bp, derive(layouts.lg, COLS[bp], C));
        return entry.get(bp);
    }
    function breakpoint(width, prev) {
        const raw = width >= BP_MIN.lg ? 'lg' : width >= BP_MIN.md ? 'md' : 'sm';
        if (prev === 'lg' && raw === 'md' && width >= BP_MIN.lg - HYSTERESIS) return 'lg';
        if (prev === 'md' && raw === 'sm' && width >= BP_MIN.md - HYSTERESIS) return 'md';
        return raw;
    }
    const gapFor = (density, gridGap) => (typeof gridGap === 'number' && gridGap >= 0 ? gridGap : GAPS[density] || GAPS.comfortable);
    function metrics(W, H, cols, rows, gap, rowUnit, fit) {
        const colW = (W - 2 * gap - (cols - 1) * gap) / cols, scrollRow = rowUnit === 'auto' || rowUnit == null ? clamp(colW, 56, 180) : rowUnit;
        if (fit === 'fit_screen') {
            const cell = Math.min(colW, (H - 2 * gap - (rows - 1) * gap) / rows);
            if (cell < 48) return { mode: 'overflow', colW, rowH: scrollRow, offX: 0 };
            return { mode: 'fit', colW: cell, rowH: cell, offX: (W - (cols * cell + (cols - 1) * gap)) / 2 };
        }
        return { mode: 'scroll', colW, rowH: scrollRow, offX: 0 };
    }
    function cssVars(m, cols, rows, gap) { const r = n => Math.round(n * 100) / 100;
        return { '--hd-cols': cols, '--hd-rows': rows, '--hd-colw': r(m.colW) + 'px', '--hd-rowh': r(m.rowH) + 'px', '--hd-gap': gap + 'px', '--hd-offx': r(m.offX) + 'px' }; }
    const rows = L => L.reduce((m, i) => Math.max(m, i.y + i.h), 0);
    function addToLayouts(layouts, item, C) {
        const p = G.place(layouts.lg, item.w, item.h, COLS.lg);
        const out = { lg: layouts.lg.concat([{ id: item.id, x: p.x, y: p.y, w: item.w, h: item.h }]), md: layouts.md == null ? null : layouts.md, sm: layouts.sm == null ? null : layouts.sm };
        for (const bp of ['md', 'sm']) {
            if (!Array.isArray(layouts[bp])) continue;
            const s = derivedSize(item, COLS[bp], constraintOf(C, item.id)), q = G.place(layouts[bp], s.w, s.h, COLS[bp]);
            out[bp] = layouts[bp].concat([{ id: item.id, x: q.x, y: q.y, w: s.w, h: s.h }]);
        }
        return out;
    }
    function removeFromLayouts(layouts, ids) { const drop = new Set([].concat(ids)), keep = L => L.filter(i => !drop.has(i.id));
        return { lg: keep(layouts.lg), md: Array.isArray(layouts.md) ? keep(layouts.md) : null, sm: Array.isArray(layouts.sm) ? keep(layouts.sm) : null }; }
    function materialiseOverride(layouts, bp, C) { if (Array.isArray(layouts[bp])) return layouts; const out = Object.assign({}, layouts); out[bp] = derive(layouts.lg, COLS[bp], C).map(i => Object.assign({}, i)); return out; }
    function resetOverride(layouts, bp) { const out = Object.assign({}, layouts); out[bp] = null; return out; }
    E.layout = Object.freeze({ COLS, BP_MIN, HYSTERESIS, GAPS, derivedSize, derive, effective, breakpoint, gapFor, metrics, cssVars, rows, addToLayouts, removeFromLayouts, materialiseOverride, resetOverride, clearMemo() { memo = new WeakMap(); } });
})();
```
- [ ] Run it. Expected: PASS. Commit `feat(homedash): responsive layout derivation and metrics`.

## Task 5: hd-doc.js and hd-history.js

**Files:** `ui/js/homedash/hd-doc.js`, `ui/js/homedash/hd-history.js`, `scripts/homedash-tests/doc.test.mjs`. **Read first:** main document 4.2 (document shape); frontend part 5.7.

- [ ] Failing test:
```js
import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs';
const boot = () => loadEngine(['hd-core.js', 'hd-grid.js', 'hd-layout.js', 'hd-doc.js', 'hd-history.js']).E;
const freeze = o => { if (o && typeof o === 'object') { Object.values(o).forEach(freeze); Object.freeze(o); } return o; };
const base = E => E.doc.normalise({ pages: [{ id: 'main', widgets: [{ id: 'w1', type: 'sensor', options: { unit: 'C' } }, { id: 'w2', type: 'switch' }], layouts: { lg: [{ id: 'w1', x: 0, y: 0, w: 2, h: 1 }, { id: 'w2', x: 2, y: 0, w: 2, h: 1 }] } }] });
const P = ['pages', 'main', 'widgets', 'w1', 'options', 'unit'];
export const tests = [
  ['normalise fills defaults and repairs layouts without mutating input', () => { const E = boot(); const d = E.doc.normalise({});
    eq([d.schema_version, d.look.family, d.look.palette.mode, d.look.density, d.look.radius_scale], [1, 'glass', 'theme', 'comfortable', 1]);
    eq(d.grid, { row_unit: 'auto', gap: null, fit: 'scroll' }); eq([d.pages[0].id, d.pages[0].compact, d.pages[0].widgets, d.pages[0].layouts], ['main', true, [], { lg: [], md: null, sm: null }]);
    const raw = freeze({ look: { family: 'neon', radius_scale: 9 }, pages: [{ id: 'main', widgets: [{ id: 'w1', type: 'a' }, { id: 'w2', type: 'b' }], layouts: { lg: [{ id: 'w1', x: 0, y: 0, w: 2, h: 2 }, { id: 'ghost', x: 0, y: 0, w: 1, h: 1 }] } }, { id: 'main' }] });
    const n = E.doc.normalise(raw); eq([n.look.family, n.look.radius_scale], ['glass', 1.5]);
    eq(n.pages[0].layouts.lg, [{ id: 'w1', x: 0, y: 0, w: 2, h: 2 }, { id: 'w2', x: 2, y: 0, w: 2, h: 1 }]); eq(n.pages.map(p => p.id), ['main', 'main_']); }],
  ['id-keyed set: apply, before, invert, survives reorder, missing path throws', () => { const E = boot(), D = base(E); const c = E.doc.setField(D, P, 'F');
    const A = E.doc.apply(D, c.ops); assert.equal(E.doc.getAt(A, P), 'F'); assert.equal(E.doc.getAt(D, P), 'C');
    assert.equal(E.doc.getAt(E.doc.apply(A, c.ops, 'before'), P), 'C'); assert.equal(E.doc.getAt(E.doc.apply(A, E.doc.invert(c.ops)), P), 'C');
    const R = JSON.parse(JSON.stringify(A)); R.pages[0].widgets.reverse(); assert.equal(E.doc.getAt(E.doc.apply(R, c.ops, 'before'), P), 'C');
    assert.throws(() => E.doc.apply(D, E.doc.setField(D, ['pages', 'nope', 'name'], 'x').ops), /path not found/); }],
  ['add and remove widgets keep layouts and order through undo', () => { const E = boot(), D = base(E);
    const add = E.doc.addWidget(D, 'main', { id: 'w3', type: 'light' }, { w: 4, h: 2 }); const A = E.doc.apply(D, add.ops);
    eq(A.pages[0].layouts.lg.find(i => i.id === 'w3'), { id: 'w3', x: 4, y: 0, w: 4, h: 2 }); eq(E.doc.apply(A, add.ops, 'before').pages[0].widgets.map(w => w.id), ['w1', 'w2']);
    const rem = E.doc.removeWidgets(D, 'main', ['w1', 'w2']); const X = E.doc.apply(D, rem.ops); eq([X.pages[0].widgets, X.pages[0].layouts.lg], [[], []]);
    const U = E.doc.apply(X, rem.ops, 'before'); eq(U.pages[0].widgets.map(w => w.id), ['w1', 'w2']); eq(U.pages[0].layouts.lg, D.pages[0].layouts.lg);
    const md = [{ id: 'w1', x: 0, y: 0, w: 4, h: 1 }]; eq(E.doc.apply(D, E.doc.setLayout(D, 'main', 'md', md).ops).pages[0].layouts.md, md); }],
  ['clipboard round trip with new ids', () => { const E = boot(), D = base(E); const items = E.doc.fromClipboard(E.doc.toClipboard(D, 'main', ['w1']));
    assert.equal(items.length, 1); assert.equal(items[0].widget.type, 'sensor'); assert.match(items[0].widget.id, /^w_/); eq([items[0].w, items[0].h], [2, 1]);
    eq(E.doc.fromClipboard('nope'), []); eq(E.doc.fromClipboard('{"v":2}'), []); }],
  ['history: undo, redo, redo cleared, limit, coalescing, seal', () => { const E = boot(), D = base(E), N = ['pages', 'main', 'name']; const h = E.history.create();
    const c1 = E.doc.setField(D, N, 'one', { coalesce: 'k', window: 800, at: 1000 }); h.push(c1); const D1 = E.doc.apply(D, c1.ops);
    const c2 = E.doc.setField(D1, N, 'two', { coalesce: 'k', window: 800, at: 1500 }); h.push(c2); const D2 = E.doc.apply(D1, c2.ops);
    assert.equal(h.size(), 1); eq([h.peek().ops[0].before, h.peek().ops[0].after], ['', 'two']);
    h.push(E.doc.setField(D2, N, 'three', { coalesce: 'k', window: 800, at: 2400 })); assert.equal(h.size(), 2);
    h.seal(); h.push(E.doc.setField(D2, N, 'four', { coalesce: 'k', window: 800, at: 2500 })); assert.equal(h.size(), 3);
    const u = h.undo(D2); assert.ok(u && h.canRedo()); h.push(E.doc.setField(D2, N, 'x', { at: 9000 })); assert.equal(h.canRedo(), false);
    const g = E.history.create(); for (let i = 0; i < 105; i++) g.push(E.doc.setField(D, N, 'n' + i, { at: i })); assert.equal(g.size(), 100);
    const k = E.history.create(); k.push(c1); assert.equal(k.undo(D1).doc.pages[0].name, ''); assert.equal(k.redo(D).doc.pages[0].name, 'one'); }]
];
```
- [ ] Run it. Expected: FAIL. Implement `hd-doc.js`:
```js
// HomeDash documents (pure): normalise/repair, id-keyed patch ops (apply/invert), command builders, clipboard.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.layout) throw new Error('hd-doc.js: load hd-layout.js first');
    const { clone, randomId, clamp } = E.core, COLS = E.layout.COLS;
    const isObj = v => v !== null && typeof v === 'object' && !Array.isArray(v), pick = (v, l, d) => (l.includes(v) ? v : d);
    const WIDGET_DEFAULTS = { variant: '', title: '', bindings: {}, options: {}, actions: {}, confirm: false };
    function normalisePage(raw, index, sizeOf) {
        const p = isObj(raw) ? raw : {}, seen = new Set();
        const widgets = (Array.isArray(p.widgets) ? p.widgets : []).filter(w => isObj(w) && typeof w.id === 'string' && typeof w.type === 'string' && !seen.has(w.id) && seen.add(w.id)).map(w => Object.assign({}, WIDGET_DEFAULTS, clone(w)));
        const ids = new Set(widgets.map(w => w.id)), L = isObj(p.layouts) ? p.layouts : {};
        let layouts = { lg: E.grid.repair(L.lg, COLS.lg, null, { ids, compact: p.compact !== false }), md: null, sm: null };
        for (const bp of ['md', 'sm']) if (Array.isArray(L[bp])) layouts[bp] = E.grid.repair(L[bp], COLS[bp], null, { ids, compact: true });
        for (const w of widgets) if (!layouts.lg.some(i => i.id === w.id)) layouts = E.layout.addToLayouts(layouts, Object.assign({ id: w.id }, sizeOf(w)), null);
        for (const bp of ['md', 'sm']) if (Array.isArray(layouts[bp])) for (const w of widgets) if (!layouts[bp].some(i => i.id === w.id)) {
            const s = E.layout.derivedSize(layouts.lg.find(i => i.id === w.id), COLS[bp], {}), q = E.grid.place(layouts[bp], s.w, s.h, COLS[bp]);
            layouts[bp] = layouts[bp].concat([{ id: w.id, x: q.x, y: q.y, w: s.w, h: s.h }]);
        }
        return { id: typeof p.id === 'string' && p.id ? p.id : index === 0 ? 'main' : 'p' + (index + 1), name: typeof p.name === 'string' ? p.name : '', icon: typeof p.icon === 'string' ? p.icon : 'home', compact: p.compact !== false, widgets, layouts };
    }
    function normalise(raw, opts) {
        const d = isObj(raw) ? raw : {}, look = isObj(d.look) ? d.look : {}, grid = isObj(d.grid) ? d.grid : {};
        const sizeOf = (opts && opts.sizeOf) || (() => ({ w: 2, h: 1 }));
        const palette = isObj(look.palette) ? clone(look.palette) : {}; palette.mode = pick(palette.mode, ['theme', 'dark', 'light', 'custom'], 'theme');
        const pages = (Array.isArray(d.pages) && d.pages.length ? d.pages : [{}]).map((p, i) => normalisePage(p, i, sizeOf)), used = new Set();
        for (const p of pages) { while (used.has(p.id)) p.id += '_'; used.add(p.id); }
        return Object.assign({}, clone(d), {
            schema_version: 1,
            look: Object.assign({}, clone(look), { family: pick(look.family, ['glass', 'tactile', 'minimal', 'instrument'], 'glass'), palette, backdrop: isObj(look.backdrop) ? clone(look.backdrop) : { kind: 'theme' }, density: pick(look.density, ['compact', 'comfortable', 'spacious'], 'comfortable'), radius_scale: clamp(Number(look.radius_scale) || 1, 0.5, 1.5) }),
            grid: { row_unit: grid.row_unit === undefined || grid.row_unit === 'auto' ? 'auto' : clamp(Number(grid.row_unit) || 96, 48, 200), gap: typeof grid.gap === 'number' ? grid.gap : null, fit: grid.fit === 'fit_screen' ? 'fit_screen' : 'scroll' },
            pages
        });
    }
    const keyOf = path => JSON.stringify(path);
    const locate = (arr, seg) => (typeof seg === 'number' ? seg : arr.findIndex(x => isObj(x) && x.id === seg));
    function getAt(doc, path) { let n = doc; for (const seg of path) { if (n == null) return undefined; if (Array.isArray(n)) { const i = locate(n, seg); n = i >= 0 ? n[i] : undefined; } else n = n[seg]; } return n; }
    function setAt(node, path, value, index) {
        const [seg, ...rest] = path;
        if (Array.isArray(node)) {
            const copy = node.slice(), i = locate(copy, seg);
            if (!rest.length) {
                if (value === undefined) { if (i >= 0) copy.splice(i, 1); }
                else if (i >= 0) copy[i] = clone(value);
                else copy.splice(index === undefined ? copy.length : clamp(index, 0, copy.length), 0, clone(value));
                return copy;
            }
            if (i < 0) throw new Error('hd-doc: path not found at ' + seg);
            copy[i] = setAt(copy[i], rest, value, index); return copy;
        }
        if (!isObj(node)) throw new Error('hd-doc: path not found at ' + seg);
        const copy = Object.assign({}, node);
        if (!rest.length) { if (value === undefined) delete copy[seg]; else copy[seg] = clone(value); return copy; }
        if (!(seg in copy)) throw new Error('hd-doc: path not found at ' + seg);
        copy[seg] = setAt(copy[seg], rest, value, index); return copy;
    }
    function apply(doc, ops, dir) { const side = dir === 'before' ? 'before' : 'after'; let out = doc;
        for (const op of side === 'before' ? ops.slice().reverse() : ops) out = setAt(out, op.path, op[side], op.index); return out; }
    const invert = ops => ops.slice().reverse().map(op => Object.assign({}, op, { before: op.after, after: op.before }));
    const command = (label, ops, meta) => ({ label, ops, meta: Object.assign({ at: Date.now() }, meta) });
    const op = (doc, path, after, index) => ({ path, before: clone(getAt(doc, path)), after: clone(after), index });
    function pageOf(doc, id) { const p = getAt(doc, ['pages', id]); if (!p) throw new Error('hd-doc: unknown page ' + id); return p; }
    const setField = (doc, path, value, meta) => command('set', [op(doc, path, value)], Object.assign({ kind: 'set' }, meta));
    function layoutOps(doc, pageId, next) { const page = pageOf(doc, pageId), ops = [];
        for (const bp of ['lg', 'md', 'sm']) if (keyOf(page.layouts[bp]) !== keyOf(next[bp])) ops.push(op(doc, ['pages', pageId, 'layouts', bp], next[bp])); return ops; }
    function addWidget(doc, pageId, widget, size, C) {
        const page = pageOf(doc, pageId), w = Object.assign({}, WIDGET_DEFAULTS, clone(widget)); if (!w.id) w.id = randomId('w', 8);
        return command('add', [op(doc, ['pages', pageId, 'widgets', w.id], w), ...layoutOps(doc, pageId, E.layout.addToLayouts(page.layouts, { id: w.id, w: size.w, h: size.h }, C))], { kind: 'add', ids: [w.id] });
    }
    function removeWidgets(doc, pageId, ids) {
        const page = pageOf(doc, pageId), list = [].concat(ids);
        const order = list.map(id => [id, page.widgets.findIndex(w => w.id === id)]).filter(e => e[1] >= 0).sort((a, b) => a[1] - b[1]);
        const ops = order.map(([id, i], k) => ({ path: ['pages', pageId, 'widgets', id], before: clone(page.widgets[i]), after: undefined, index: i - k }));
        return command('remove', ops.concat(layoutOps(doc, pageId, E.layout.removeFromLayouts(page.layouts, list))), { kind: 'remove', ids: list });
    }
    function setLayout(doc, pageId, bp, layout, meta) { const page = pageOf(doc, pageId), next = Object.assign({}, page.layouts); next[bp] = layout;
        return command('layout', layoutOps(doc, pageId, next), Object.assign({ kind: 'layout' }, meta)); }
    function toClipboard(doc, pageId, ids) { const page = pageOf(doc, pageId), set = new Set(ids);
        return JSON.stringify({ v: 1, widgets: page.widgets.filter(w => set.has(w.id)).map(w => { const c = clone(w), l = page.layouts.lg.find(i => i.id === w.id) || { w: 2, h: 1 }; delete c.id; return { widget: c, w: l.w, h: l.h }; }) }); }
    function fromClipboard(text) {
        let data; try { data = JSON.parse(text); } catch (err) { return []; }
        if (!isObj(data) || data.v !== 1 || !Array.isArray(data.widgets)) return [];
        return data.widgets.filter(e => isObj(e) && isObj(e.widget) && typeof e.widget.type === 'string').map(e => ({ widget: Object.assign(clone(e.widget), { id: randomId('w', 8) }), w: clamp(Math.round(Number(e.w) || 2), 1, 12), h: clamp(Math.round(Number(e.h) || 1), 1, 24) }));
    }
    E.doc = Object.freeze({ normalise, keyOf, getAt, setAt, apply, invert, command, op, setField, addWidget, removeWidgets, setLayout, toClipboard, fromClipboard });
})();
```
- [ ] `hd-history.js`:
```js
// HomeDash undo/redo (pure): command stack with coalescing (same meta.coalesce within meta.window ms), limit 100.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.doc) throw new Error('hd-history.js: load hd-doc.js first');
    function create(options) {
        const limit = (options && options.limit) || 100;
        let undoStack = [], redoStack = [], sealed = false;
        function merge(top, cmd) {
            const byPath = new Map(top.ops.map(o => [E.doc.keyOf(o.path), o]));
            for (const o of cmd.ops) { const prev = byPath.get(E.doc.keyOf(o.path)); if (prev) prev.after = o.after; else { const c = Object.assign({}, o); top.ops.push(c); byPath.set(E.doc.keyOf(o.path), c); } }
            top.meta = Object.assign({}, top.meta, { at: cmd.meta.at });
        }
        return {
            push(cmd) {
                const top = undoStack[undoStack.length - 1], m = cmd.meta || {};
                redoStack = [];
                if (!sealed && top && m.coalesce && top.meta.coalesce === m.coalesce && m.at - top.meta.at <= (m.window || 0)) { merge(top, cmd); return top; }
                const entry = { label: cmd.label, ops: cmd.ops.map(o => Object.assign({}, o)), meta: Object.assign({}, m) };
                undoStack.push(entry); sealed = false; if (undoStack.length > limit) undoStack.shift(); return entry;
            },
            undo(doc) { const c = undoStack.pop(); if (!c) return null; redoStack.push(c); sealed = true; return { doc: E.doc.apply(doc, c.ops, 'before'), command: c }; },
            redo(doc) { const c = redoStack.pop(); if (!c) return null; undoStack.push(c); sealed = true; return { doc: E.doc.apply(doc, c.ops, 'after'), command: c }; },
            seal() { sealed = true; }, clear() { undoStack = []; redoStack = []; sealed = false; },
            canUndo: () => undoStack.length > 0, canRedo: () => redoStack.length > 0, size: () => undoStack.length, peek: () => undoStack[undoStack.length - 1] || null
        };
    }
    E.history = Object.freeze({ create, LIMIT: 100 });
})();
```
- [ ] Run it. Expected: PASS. Commit `feat(homedash): document ops and undo history`.

## Task 6: hd-colour.js

**Files:** `ui/js/homedash/hd-colour.js`, `scripts/homedash-tests/colour.test.mjs`. **Read first:** visual part 1.4–1.5.

- [ ] Failing test:
```js
import assert from 'node:assert/strict'; import { loadEngine } from './harness.mjs';
const boot = dom => loadEngine(['hd-core.js', 'hd-colour.js'], { dom });
function rng(seed) { return () => { seed |= 0; seed = seed + 0x6D2B79F5 | 0; let t = Math.imul(seed ^ seed >>> 15, 1 | seed); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; }; }
export const tests = [
  ['conversions and contrast', () => { const C = boot().E.colour;
    assert.equal(C.contrast('#000000', '#FFFFFF').toFixed(2), '21.00'); assert.equal(C.contrast('#777777', '#FFFFFF').toFixed(2), '4.48');
    assert.deepEqual([...C.hexToRgb('#abc')], [170, 187, 204]); assert.equal(C.rgbToHex([170, 187, 204]), '#AABBCC');
    for (const h of ['#1A1F29', '#7AA7FF', '#C4352B', '#FFFFFF', '#000000']) { const a = C.hexToRgb(h), b = C.hexToRgb(C.oklchToHex(C.rgbToOklch(a))); a.forEach((v, i) => assert.ok(Math.abs(v - b[i]) <= 1, h)); }
    const w = C.rgbToOklch([255, 255, 255]); assert.ok(Math.abs(w[0] - 1) < 1e-3 && w[1] < 1e-3);
    assert.deepEqual(C.hsvToRgb(0, 1, 1).map(Math.round), [255, 0, 0]); assert.deepEqual(C.hsvToRgb(120, 1, 1).map(Math.round), [0, 255, 0]); }],
  ['the custom palette solver meets every pair for 500 random inputs', () => { const C = boot().E.colour, r = rng(7);
    for (let i = 0; i < 500; i++) {
      const accent = C.rgbToHex([r() * 255, r() * 255, r() * 255]), input = { accent, surface_hue: r() * 360, scheme: r() < 0.5 ? 'dark' : 'light' };
      const t = C.solvePalette(input).tokens, at = (a, b) => C.contrast(t[a], t[b]), tag = JSON.stringify(input);
      for (const name of C.PALETTE_TOKENS) assert.match(t[name], /^#[0-9A-F]{6}$/, name + ' ' + tag);
      for (const [a, b, min] of [['ink','surface-solid',4.5],['ink','canvas',4.5],['ink-muted','surface-solid',5],['ink-muted','canvas',4.5],['on-ink','on',4.5],['ink-on-accent','accent',4.5],['accent','surface-solid',3],['selected-ink','selected',4.5],['btn-ink','btn',4.5],['ok','surface-solid',4.5],['warn','surface-solid',4.5],['alert','surface-solid',4.5],['info','surface-solid',4.5]])
        assert.ok(at(a, b) >= min, `${a} on ${b} ${at(a, b).toFixed(2)} ${tag}`);
    } }],
  ['applyPalette writes --hd- custom properties', () => { const { E, document } = boot(true); const el = document.createElement('div');
    E.colour.applyPalette(el, E.colour.solvePalette({ accent: '#8E5BD9', surface_hue: 285, scheme: 'dark' }).tokens);
    assert.match(el.style.getPropertyValue('--hd-ink'), /^#/); E.colour.clearPalette(el); assert.equal(el.style.getPropertyValue('--hd-ink'), ''); }]
];
```
- [ ] Run it. Expected: FAIL. Implement:
```js
// HomeDash colour (pure): sRGB/OKLCH conversion, WCAG contrast and the custom palette solver (visual part 1.5).
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-colour.js: load hd-core.js first');
    const clamp = E.core.clamp;
    const PALETTE_TOKENS = Object.freeze(['canvas', 'canvas-2', 'surface-solid', 'surface-raised', 'sunken', 'field', 'ink', 'ink-muted', 'ink-faint', 'ink-on-accent', 'accent', 'on', 'on-ink', 'selected', 'selected-ink', 'btn', 'btn-ink', 'hairline', 'focus', 'ok', 'warn', 'alert', 'info', 'heat', 'cool', 'edge-tint', 'shadow-ink']);
    const NEUTRAL = Object.freeze({
        dark: { ok: '#4AD29C', warn: '#F2B84B', alert: '#FF7A72', info: '#5AA9FF', heat: '#FF9A57', cool: '#5CC8FF', 'edge-tint': '#FFFFFF', 'shadow-ink': '#000000' },
        light: { ok: '#1A7D52', warn: '#9A6200', alert: '#C4352B', info: '#1F63C7', heat: '#D9622B', cool: '#1E8FCB', 'edge-tint': '#FFFFFF', 'shadow-ink': '#1B2333' }
    });
    function hexToRgb(hex) { const m = /^#?([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(String(hex || '').trim()); if (!m) return null;
        const h = m[1].length === 3 ? m[1].replace(/./g, c => c + c) : m[1]; return [0, 2, 4].map(i => parseInt(h.slice(i, i + 2), 16)); }
    const rgbToHex = rgb => '#' + rgb.map(v => clamp(Math.round(v), 0, 255).toString(16).padStart(2, '0')).join('').toUpperCase();
    const toLinear = c => (c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4));
    const toGamma = c => (c <= 0.0031308 ? 12.92 * c : 1.055 * Math.pow(c, 1 / 2.4) - 0.055);
    function rgbToOklch(rgb) {
        const [r, g, b] = rgb.map(v => toLinear(v / 255));
        const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b), m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b), s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
        const L = 0.2104542553 * l + 0.7936177850 * m - 0.0040720468 * s, A = 1.9779984951 * l - 2.4285922050 * m + 0.4505937099 * s, B = 0.0259040371 * l + 0.7827717662 * m - 0.8086757660 * s;
        const C = Math.sqrt(A * A + B * B); let H = Math.atan2(B, A) * 180 / Math.PI; if (H < 0) H += 360;
        return [L, C, C < 1e-6 ? 0 : H];
    }
    function oklchToLinear([L, C, H]) {
        const a = C * Math.cos(H * Math.PI / 180), b = C * Math.sin(H * Math.PI / 180);
        const l = Math.pow(L + 0.3963377774 * a + 0.2158037573 * b, 3), m = Math.pow(L - 0.1055613458 * a - 0.0638541728 * b, 3), s = Math.pow(L - 0.0894841775 * a - 1.2914855480 * b, 3);
        return [4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s, -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s, -0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s];
    }
    const inGamut = lin => lin.every(v => v >= -1e-4 && v <= 1 + 1e-4);
    function oklchToRgb(lch) {
        const L = clamp(lch[0], 0, 1), H = lch[2]; let C = Math.max(0, lch[1]);
        if (!inGamut(oklchToLinear([L, C, H]))) { let lo = 0, hi = C; for (let i = 0; i < 24; i++) { const mid = (lo + hi) / 2; if (inGamut(oklchToLinear([L, mid, H]))) lo = mid; else hi = mid; } C = lo; }
        return oklchToLinear([L, C, H]).map(v => toGamma(clamp(v, 0, 1)) * 255);
    }
    const oklchToHex = lch => rgbToHex(oklchToRgb(lch));
    function luminance(rgb) { const f = v => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); }; return 0.2126 * f(rgb[0]) + 0.7152 * f(rgb[1]) + 0.0722 * f(rgb[2]); }
    function contrast(a, b) { const x = luminance(typeof a === 'string' ? hexToRgb(a) : a), y = luminance(typeof b === 'string' ? hexToRgb(b) : b); return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05); }
    function stepUntil(lch, step, ok) { const cur = lch.slice(); let hex = oklchToHex(cur);
        for (let i = 0; i < 60 && !ok(hex); i++) { cur[0] = clamp(cur[0] + step, 0, 1); hex = oklchToHex(cur); } return hex; }
    function solvePalette(input) {
        const scheme = input && input.scheme === 'light' ? 'light' : 'dark', dark = scheme === 'dark', away = dark ? 1 : -1;
        const hue = (((Number(input && input.surface_hue) || 0) % 360) + 360) % 360, surfL = dark ? 0.22 : 0.985;
        const accentRgb = hexToRgb(input && input.accent) || hexToRgb(dark ? '#7AA7FF' : '#2F6FE0');
        const tone = (L, C) => oklchToHex([clamp(L, 0, 1), C === undefined ? 0.012 : C, hue]), t = {};
        // 1: surfaces; canvas and sunken are darker than the surface in both schemes
        t['surface-solid'] = tone(surfL); t.canvas = tone(surfL - 0.04); t['canvas-2'] = tone(surfL - (dark ? 0.01 : 0.07));
        t.sunken = tone(surfL - 0.07); t.field = t.sunken; t['surface-raised'] = dark ? tone(surfL + 0.03) : t['surface-solid'];
        // 2: ink, then muted walked toward the surface while 5.0:1 (surface) and 4.5:1 (canvas) hold
        const inkL = dark ? 0.96 : 0.18; t.ink = tone(inkL, 0.01);
        let mutedL = inkL, muted = t.ink;
        for (let i = 0; i < 100; i++) { const L = mutedL - away * 0.01, c = tone(L, 0.01); if (contrast(c, t['surface-solid']) < 5 || contrast(c, t.canvas) < 4.5) break; mutedL = L; muted = c; }
        t['ink-muted'] = muted; t['ink-faint'] = tone((mutedL + surfL) / 2, 0.01); t.hairline = tone(surfL + away * 0.12, 0.01);
        // 3: on = accent, on-ink = black or white, on stepped to 4.5:1
        const accentLch = rgbToOklch(accentRgb), onInk = contrast(accentRgb, [255, 255, 255]) >= contrast(accentRgb, [0, 0, 0]) ? '#FFFFFF' : '#000000';
        t['on-ink'] = onInk; t.on = stepUntil(accentLch, onInk === '#FFFFFF' ? -0.02 : 0.02, h => contrast(h, onInk) >= 4.5);
        // 4: accent as a graphic, 3:1 on the surface
        t.accent = stepUntil(accentLch, away * 0.02, h => contrast(h, t['surface-solid']) >= 3);
        t['ink-on-accent'] = contrast(t.accent, '#FFFFFF') >= contrast(t.accent, '#000000') ? '#FFFFFF' : '#000000'; t.focus = t.accent;
        t.selected = oklchToHex([clamp(surfL + away * 0.08, 0, 1), Math.min(accentLch[1], 0.06), accentLch[2]]); t['selected-ink'] = t.ink;
        t.btn = tone(surfL + away * 0.05); t['btn-ink'] = t.ink;
        // 5: status colours from the neutral set, stepped to 4.5:1 on the surface
        for (const n of ['ok', 'warn', 'alert', 'info']) t[n] = stepUntil(rgbToOklch(hexToRgb(NEUTRAL[scheme][n])), away * 0.02, h => contrast(h, t['surface-solid']) >= 4.5);
        for (const n of ['heat', 'cool', 'edge-tint', 'shadow-ink']) t[n] = NEUTRAL[scheme][n];
        return { scheme, tokens: t };
    }
    const applyPalette = (el, tokens) => E.core.setVars(el, Object.fromEntries(PALETTE_TOKENS.map(n => ['--hd-' + n, tokens[n]])));
    const clearPalette = el => E.core.setVars(el, Object.fromEntries(PALETTE_TOKENS.map(n => ['--hd-' + n, null])));
    function hsvToRgb(h, s, v) { const f = n => { const k = (n + h / 60) % 6; return (v - v * s * Math.max(0, Math.min(k, 4 - k, 1))) * 255; }; return [f(5), f(3), f(1)]; }
    E.colour = Object.freeze({ PALETTE_TOKENS, NEUTRAL, hexToRgb, rgbToHex, rgbToOklch, oklchToRgb, oklchToHex, luminance, contrast, solvePalette, applyPalette, clearPalette, hsvToRgb });
})();
```
- [ ] Run it. Expected: PASS. If a random case fails, fix the solver step, not the test. Commit `feat(homedash): OKLCH colour maths and custom palette solver`.

## Task 7: Line budget covers ui/js/homedash

**Files:** `ui/desktop_js_line_budget_test.go`. **Read first:** that file `:10-67`; `.github/workflows/security-gates.yml:141` (CI runs it by name; no workflow edit is needed).

- [ ] Run impact on `TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget`. Expected: no callers; it is a test function.
- [ ] Failing check: create a scratch file `ui/js/homedash/zz-budget-probe.js` of 1100 lines (one `//` per line). Run `go test ./ui -run '^TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget$' -count=1`. Expected: PASS, which demonstrates the gap.
- [ ] Wrap the `WalkDir` in `for _, root := range []string{filepath.Join("js", "desktop"), filepath.Join("js", "homedash")} { … if err != nil { t.Fatal(err) } }`. Keep the body unchanged. A missing root fails the walk on purpose.
- [ ] Run again. Expected: FAIL naming `js/homedash/zz-budget-probe.js has 1100 lines`.
- [ ] Delete the scratch file and run again. Expected: PASS.
- [ ] Commit `test(ui): extend the JS line budget to ui/js/homedash`.

## Task 8: Token, bridge and base CSS, starters, bundle entry

**Files:** `ui/css/homedash-*.css` (9 parts), `scripts/build-ui-bundles.js`, `ui/css/homedash.bundle.css`, `scripts/homedash-tests/bundle.test.mjs`.
**Read first:** visual part 1.1–1.4, 9.1–9.3; main document 4.4; `scripts/build-ui-bundles.js:263-401` (`--check` compares byte-for-byte; bundles are committed); `ui/desktop_app_materials_test.go:645-708`. Only sheets that `module-loader.js` loads are checked, so this bundle is not checked yet. Later the check only needs the bundle to contain `--vd-mat-`, and the bridge gives it that.

- [ ] Failing test `bundle.test.mjs`:
```js
import assert from 'node:assert/strict'; import { spawnSync } from 'node:child_process'; import fs from 'node:fs'; import path from 'node:path'; import { read, root } from './harness.mjs';
export const PARTS = ['tokens', 'bridge', 'base', 'primitives', 'family-glass', 'family-tactile', 'family-minimal', 'family-instrument', 'widgets'].map(p => `ui/css/homedash-${p}.css`);
export const tests = [
  ['the bundle entry lists the nine parts in order and the parts are LF without BOM', () => {
    const m = /out: 'ui\/css\/homedash\.bundle\.css',\s*parts: \[([^\]]*)\]/.exec(read('scripts/build-ui-bundles.js')); assert.ok(m, 'bundle entry missing');
    assert.deepEqual([...m[1].matchAll(/'([^']+)'/g)].map(x => x[1]), PARTS);
    for (const p of PARTS) { const t = read(p); assert.ok(!t.startsWith(String.fromCodePoint(0xfeff)), p + ' BOM'); assert.ok(!t.includes('\r'), p + ' CRLF'); } }],
  ['the generated bundle is current and carries the bridge and the font faces', () => {
    const r = spawnSync(process.execPath, ['scripts/build-ui-bundles.js', '--check'], { cwd: root, encoding: 'utf8' });
    assert.equal(r.status, 0, r.stdout + r.stderr); assert.match(r.stdout, /Checked ui\/css\/homedash\.bundle\.css \(9 parts\)/);
    const b = read('ui/css/homedash.bundle.css'); for (const s of ['--vd-mat-card-color', '"HomeDash Barlow Condensed"', '"HomeDash Share Tech Mono"', '--hd-n-ink']) assert.ok(b.includes(s), s);
    for (const f of ['BarlowCondensed-SemiBold.ttf', 'ShareTechMono-Regular.ttf']) assert.ok(fs.existsSync(path.join(root, 'ui/fonts', f)), f); }]
];
```
- [ ] Run it. Expected: FAIL (entry missing).
- [ ] `homedash-tokens.css` (full):
```css
/* HomeDash tokens: .hd-root defaults, neutral palettes (--hd-n-*), type, motion, radii, strokes, spacing and the HomeDash
   font faces. Literal colours live only in the token blocks of this file (guard: scripts/homedash-tests/css-lint.mjs). */
@font-face { font-family: "HomeDash Barlow Condensed"; src: url('/fonts/BarlowCondensed-SemiBold.ttf') format('truetype'); font-weight: 600; font-style: normal; font-display: swap; }
@font-face { font-family: "HomeDash Share Tech Mono"; src: url('/fonts/ShareTechMono-Regular.ttf') format('truetype'); font-weight: 400; font-style: normal; font-display: swap; }

.hd-root {
    --hd-n-canvas: #0E1116; --hd-n-canvas-2: #171C26; --hd-n-surface: #1A1F29; --hd-n-raised: #222834; --hd-n-sunken: #0A0D12;
    --hd-n-ink: #F1F4F9; --hd-n-ink-muted: #A3ADBD; --hd-n-ink-faint: #5E6878;
    --hd-n-accent: #7AA7FF; --hd-n-on: #8FBFFF; --hd-n-on-ink: #0B1220; --hd-n-ink-on-accent: #0B1220;
    --hd-n-selected: #2A3A57; --hd-n-selected-ink: #F1F4F9; --hd-n-btn: #262D3A; --hd-n-btn-ink: #F1F4F9; --hd-n-accent-2: #F2B84B; --hd-n-hairline: #2E3542;
    --hd-n-ok: #4AD29C; --hd-n-warn: #F2B84B; --hd-n-alert: #FF7A72; --hd-n-info: #5AA9FF; --hd-n-heat: #FF9A57; --hd-n-cool: #5CC8FF;
    --hd-n-ct-2200: #FFA552; --hd-n-ct-4000: #FFE2B8; --hd-n-ct-6500: #E4EEFF; --hd-n-edge-tint: #FFFFFF; --hd-n-shadow-ink: #000000;
    --hd-font-ui: "Inter", system-ui, sans-serif; --hd-font-label: var(--hd-font-ui); --hd-font-num: var(--hd-font-ui); --hd-font-heading: var(--hd-font-ui); --hd-num-features: "tnum" 1;
    --hd-text-xs: 11px; --hd-text-s: 12.5px; --hd-text-m: 14px; --hd-text-l: 17px; --hd-text-xl: 21px; --hd-label-tracking: 0;
    --hd-dur-press: 90ms; --hd-dur-quick: 140ms; --hd-dur-state: 220ms; --hd-dur-value: 420ms; --hd-dur-needle: 1100ms;
    --hd-ease-standard: cubic-bezier(.2, 0, 0, 1); --hd-ease-out: cubic-bezier(.16, 1, .3, 1); --hd-ease-spring: cubic-bezier(.34, 1.45, .64, 1); --hd-ease-snap: cubic-bezier(.6, 0, .25, 1.25);
    --hd-ease-needle: linear(0, .038 4.2%, .135 8.3%, .262 12.5%, .402 16.7%, .664 25%, .862 33.3%, .984 41.7%, 1.042 50%, 1.059 58.3%, 1.052 66.7%, 1.037 75%, 1.021 83.3%, 1);
    --hd-radius-scale: 1; --hd-radius-card: calc(14px * var(--hd-radius-scale)); --hd-radius-control: calc(8px * var(--hd-radius-scale)); --hd-radius-pill: 9999px;
    --hd-stroke-hair: 1px; --hd-stroke-1: 1.5px; --hd-stroke-2: 2px; --hd-stroke-arc: 0.075; --hd-stroke-tick: 1.25px; --hd-icon-stroke: 1.75;
    --hd-unit: 4px; --hd-space-1: var(--hd-unit); --hd-space-2: calc(var(--hd-unit) * 2); --hd-space-3: calc(var(--hd-unit) * 3); --hd-space-4: calc(var(--hd-unit) * 4);
    --hd-space-5: calc(var(--hd-unit) * 5); --hd-space-6: calc(var(--hd-unit) * 6); --hd-space-7: calc(var(--hd-unit) * 8); --hd-space-8: calc(var(--hd-unit) * 12);
    --hd-pad-card: 16px; --hd-glow: none; --hd-scrim-alpha: 0.45;
}
.hd-root[data-hd-scheme="light"] {
    --hd-n-canvas: #EEF0F4; --hd-n-canvas-2: #E2E6ED; --hd-n-surface: #FFFFFF; --hd-n-raised: #FFFFFF; --hd-n-sunken: #E7EAF0;
    --hd-n-ink: #12161D; --hd-n-ink-muted: #4D5666; --hd-n-ink-faint: #9AA2AF;
    --hd-n-accent: #2F6FE0; --hd-n-on: #2F6FE0; --hd-n-on-ink: #FFFFFF; --hd-n-ink-on-accent: #FFFFFF;
    --hd-n-selected: #DCE7FB; --hd-n-selected-ink: #12161D; --hd-n-btn: #E7EAF0; --hd-n-btn-ink: #12161D; --hd-n-accent-2: #9A6200; --hd-n-hairline: #D5DAE2;
    --hd-n-ok: #1A7D52; --hd-n-warn: #9A6200; --hd-n-alert: #C4352B; --hd-n-info: #1F63C7; --hd-n-heat: #D9622B; --hd-n-cool: #1E8FCB; --hd-n-shadow-ink: #1B2333;
}
@supports not (transition-timing-function: linear(0, 1)) { .hd-root { --hd-ease-needle: cubic-bezier(.3, 1.3, .5, 1); } }
.hd-root[data-hd-density="compact"] { --hd-unit: 3.4px; --hd-pad-card: 12px; }
.hd-root[data-hd-density="spacious"] { --hd-unit: 4.6px; --hd-pad-card: 20px; }
```
- [ ] `homedash-bridge.css` (full). Host rule, which the lab follows: palette dark or light means `data-hd-scheme` is set to the same value.
```css
/* HomeDash palettes: neutral mapping (dark, light, custom, kiosk), the desktop theme bridge for palette=theme (reads
   --vd-mat-* and --ds-*, neutral fallback) and scheme-selected status text colours. No light-dark(). */
.hd-root {
    --hd-canvas: var(--hd-n-canvas); --hd-canvas-2: var(--hd-n-canvas-2); --hd-canvas-fill: var(--hd-canvas);
    --hd-surface-solid: var(--hd-n-surface); --hd-surface-raised: var(--hd-n-raised); --hd-sunken: var(--hd-n-sunken); --hd-field: var(--hd-n-sunken);
    --hd-ink: var(--hd-n-ink); --hd-ink-muted: var(--hd-n-ink-muted); --hd-ink-faint: var(--hd-n-ink-faint); --hd-ink-on-accent: var(--hd-n-ink-on-accent);
    --hd-accent: var(--hd-n-accent); --hd-accent-2: var(--hd-n-accent-2); --hd-on: var(--hd-n-on); --hd-on-ink: var(--hd-n-on-ink);
    --hd-selected: var(--hd-n-selected); --hd-selected-ink: var(--hd-n-selected-ink); --hd-btn: var(--hd-n-btn); --hd-btn-ink: var(--hd-n-btn-ink); --hd-hairline: var(--hd-n-hairline);
    --hd-ok: var(--hd-n-ok); --hd-warn: var(--hd-n-warn); --hd-alert: var(--hd-n-alert); --hd-info: var(--hd-n-info); --hd-heat: var(--hd-n-heat); --hd-cool: var(--hd-n-cool);
    --hd-ct-2200: var(--hd-n-ct-2200); --hd-ct-4000: var(--hd-n-ct-4000); --hd-ct-6500: var(--hd-n-ct-6500); --hd-light-on: var(--hd-n-ct-2200);
    --hd-edge-tint: var(--hd-n-edge-tint); --hd-shadow-ink: var(--hd-n-shadow-ink); --hd-scrim: var(--hd-shadow-ink); --hd-focus: var(--hd-accent);
    --hd-theme-radius: 14px; --hd-theme-border: transparent; --hd-theme-edge: none; --hd-theme-glow: none;
    --hd-accent-soft: color-mix(in srgb, var(--hd-accent) 16%, transparent); --hd-pending: var(--hd-warn); --hd-unavailable: var(--hd-ink-faint);
    --hd-focus-gap: var(--hd-surface-solid); --hd-surface: var(--hd-surface-solid);
    --hd-track: color-mix(in srgb, var(--hd-ink) 14%, transparent); --hd-track-fill: var(--hd-on); --hd-thumb: var(--hd-surface-raised); --hd-thumb-edge: var(--hd-hairline);
    --hd-ok-ink: var(--hd-ok); --hd-warn-ink: var(--hd-warn); --hd-alert-ink: var(--hd-alert); --hd-info-ink: var(--hd-info);
}
.hd-root[data-hd-palette="theme"] {
    --hd-canvas: var(--vd-mat-canvas-color, var(--hd-n-canvas)); --hd-canvas-fill: var(--vd-mat-canvas, var(--hd-canvas));
    --hd-surface-solid: var(--vd-mat-card-color, var(--hd-n-surface)); --hd-sunken: var(--vd-mat-well-color, var(--hd-n-sunken)); --hd-field: var(--vd-mat-field-color, var(--hd-n-sunken));
    --hd-ink: var(--vd-text, var(--hd-n-ink)); --hd-ink-muted: var(--vd-theme-muted, var(--hd-n-ink-muted)); --hd-hairline: var(--vd-mat-divider, var(--hd-n-hairline));
    --hd-accent: var(--vd-accent, var(--hd-n-accent)); --hd-ink-on-accent: var(--vd-accent-contrast, var(--hd-n-ink-on-accent));
    --hd-on: var(--vd-mat-primary-color, var(--hd-n-on)); --hd-on-ink: var(--vd-mat-primary-text, var(--hd-n-on-ink));
    --hd-selected: var(--vd-mat-selected-color, var(--hd-n-selected)); --hd-selected-ink: var(--vd-mat-selected-text, var(--hd-n-selected-ink));
    --hd-btn: var(--vd-mat-btn-color, var(--hd-n-btn)); --hd-btn-ink: var(--vd-mat-btn-text, var(--hd-n-btn-ink)); --hd-accent-2: var(--vd-mat-accent-2, var(--hd-n-accent-2));
    --hd-ok: var(--ds-color-success-fg, var(--hd-n-ok)); --hd-warn: var(--ds-color-warning-fg, var(--hd-n-warn));
    --hd-alert: var(--ds-color-danger-fg, var(--hd-n-alert)); --hd-info: var(--ds-color-info-fg, var(--hd-n-info));
    --hd-focus: var(--ds-color-border-focus, var(--hd-accent));
    --hd-theme-radius: var(--vd-mat-card-radius, 14px); --hd-theme-border: var(--vd-mat-card-border, transparent);
    --hd-theme-edge: var(--vd-mat-card-shadow, none); --hd-theme-glow: var(--vd-mat-glow, none); --hd-font-heading: var(--vd-mat-heading-font, var(--hd-font-ui));
}
.hd-root[data-hd-scheme="light"] {
    --hd-ok-ink: color-mix(in srgb, var(--hd-ok) 34%, var(--hd-ink)); --hd-warn-ink: color-mix(in srgb, var(--hd-warn) 34%, var(--hd-ink));
    --hd-alert-ink: color-mix(in srgb, var(--hd-alert) 34%, var(--hd-ink)); --hd-info-ink: color-mix(in srgb, var(--hd-info) 34%, var(--hd-ink));
}
```
- [ ] `homedash-base.css` (full):
```css
/* HomeDash base: root, frame skeleton, reserved status line, icons, focus ring (also the lab hook data-hd-focus="ring"),
   utilities and skin guards. Motion rules added later must sit inside @media (prefers-reduced-motion: no-preference)
   on selectors that contain .hd-root:not([data-hd-motion="reduced"]):not([data-hd-motion="none"]). */
.hd-root { color-scheme: dark; font-family: var(--hd-font-ui); font-size: var(--hd-text-m); line-height: 1.35; color: var(--hd-ink); }
.hd-root[data-hd-scheme="light"] { color-scheme: light; }
.hd-root [hidden] { display: none !important; }
.hd-sr-only { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0; }
.hd-ico { width: 24px; height: 24px; flex: none; fill: none; stroke: currentColor; stroke-width: var(--hd-icon-stroke); stroke-linecap: round; stroke-linejoin: round; }
.hd-widget { box-sizing: border-box; container-type: size; container-name: hd-widget; contain: layout paint style; }
.hd-frame { box-sizing: border-box; display: grid; grid-template-rows: auto minmax(0, 1fr) auto; min-width: 0; min-height: 0; }
.hd-head { display: flex; align-items: center; gap: var(--hd-space-2); min-width: 0; }
.hd-title { flex: 1 1 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--hd-ink); font-size: var(--hd-text-s); font-weight: 500; }
.hd-sub { color: var(--hd-ink-muted); font-size: var(--hd-text-xs); }
.hd-body { position: relative; min-width: 0; min-height: 0; }
.hd-state { min-height: calc(var(--hd-text-s) * 1.35); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--hd-ink-muted); font-size: var(--hd-text-s); }
.hd-numeral { font-family: var(--hd-font-num); font-feature-settings: var(--hd-num-features); font-variant-numeric: tabular-nums; }
.hd-root :is(button, [role="slider"], [role="switch"], [role="radio"], [role="spinbutton"]):focus { outline: none; }
.hd-root :is(button, [role="slider"], [role="switch"], [role="radio"], [role="spinbutton"]):is(:focus-visible, [data-hd-focus="ring"]) { box-shadow: 0 0 0 2px var(--hd-focus-gap), 0 0 0 4px var(--hd-focus); }
.hd-root [aria-disabled="true"] { cursor: default; }
.hd-root.hd-root ::selection { background: var(--hd-selected); color: var(--hd-selected-ink); }
.hd-root.hd-root { scrollbar-color: var(--hd-ink-faint) transparent; }
```
- [ ] Six starters, each a single comment line:
  - `homedash-primitives.css`: `/* HomeDash primitives: family-neutral structure of every primitive (sizes, hit areas, layout). Filled in milestone M1. */`
  - Each family file: `/* HomeDash family <Name>: token block and painting. Filled in milestone M1 (style lab). */`
  - `homedash-widgets.css`: `/* HomeDash widgets: per-widget layout with container queries. Filled from milestone M3. */`
- [ ] Run impact on `cssBundles` (build-ui-bundles.js). In `scripts/build-ui-bundles.js`, after the `chat.bundle.css` entry and before `];` at `:335`, add:
```js
  {
    out: 'ui/css/homedash.bundle.css',
    parts: [
      'ui/css/homedash-tokens.css',
      'ui/css/homedash-bridge.css',
      'ui/css/homedash-base.css',
      'ui/css/homedash-primitives.css',
      'ui/css/homedash-family-glass.css',
      'ui/css/homedash-family-tactile.css',
      'ui/css/homedash-family-minimal.css',
      'ui/css/homedash-family-instrument.css',
      'ui/css/homedash-widgets.css'
    ]
  }
```
(Insert a comma after the previous entry.)
- [ ] Run `npm run build:ui`, then `git status --short`. Only `ui/css/homedash.bundle.css` may be new among the generated files.
- [ ] Run `npm run test:homedash` and `npm run check:ui`. Expected: PASS.
- [ ] Commit the 9 parts, the bundle and `scripts/build-ui-bundles.js`: `feat(homedash): tokens, neutral palettes, theme bridge, base and bundle`.

## Task 9: Node guard (CSS and JS rules) and CI hook

**Files:** `scripts/homedash-tests/css-lint.mjs`, `scripts/homedash-tests/guard.test.mjs`, `scripts/test-ui-regressions.mjs`. **Read first:** visual part 9.4; `scripts/test-docker-manager-ui.mjs:2062-2115`.

- [ ] Failing test `guard.test.mjs`:
```js
import assert from 'node:assert/strict'; import fs from 'node:fs'; import path from 'node:path';
import { read, root } from './harness.mjs'; import { lintCss, lintJs } from './css-lint.mjs'; import { PARTS } from './bundle.test.mjs';
const BAD = `.x { color: #fff; }
.hd-a { color: red; background: rgb(0 0 0); text-transform: uppercase; }
.hd-b { transition: opacity 1s; }
@media (prefers-reduced-motion: no-preference) { .hd-c { animation: spin 1s; } }
@keyframes spin { to { width: 10px; } }
.hd-d { backdrop-filter: blur(30px); }
.hd-e { --hd-x: #123456; --vd-y: 1; }
.hd-f { background: url(https://example.com/a.png); color: light-dark(var(--hd-a), var(--hd-b)); }
@font-face { font-family: "Plain"; src: url('/fonts/a.ttf'); }
[data-theme="x"] .hd-g { color: var(--hd-ink) !important; }
.cc-banner { color: var(--hd-ink); }`;
const GOOD = `.hd-root { --hd-n-ink: #F1F4F9; }
.hd-root[data-hd-scheme="light"] { --hd-n-ink: #12161D; }
[data-hd-family="glass"] { --hd-glass: linear-gradient(#FFF, transparent); }
@media (prefers-reduced-motion: no-preference) { .hd-root:not([data-hd-motion="reduced"]):not([data-hd-motion="none"]) .hd-toggle-thumb { transition: transform var(--hd-dur-state) var(--hd-ease-spring); } }
@keyframes hd-pulse { from { opacity: 1; } to { opacity: .5; transform: scale(1.02); } }
.hd-sheet { backdrop-filter: blur(18px); }
.hd-root [hidden] { display: none !important; }
@font-face { font-family: "HomeDash Test"; src: url('/fonts/x.ttf'); }
.hd-icon { background: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg'/%3E"); }`;
const walk = d => fs.readdirSync(d, { withFileTypes: true }).flatMap(e => e.isDirectory() ? walk(path.join(d, e.name)) : e.name.endsWith('.js') ? [path.join(d, e.name)] : []);
export const tests = [
  ['the CSS guard reports every rule', () => { const v = lintCss(BAD, { file: 'bad.css', kind: 'family' }).join('\n');
    for (const want of ['class .x', 'literal colour in .x', 'named colour', 'uppercase', 'ungated motion in .hd-b', 'ungated motion in .hd-c', 'keyframes animate width', 'backdrop-filter outside', 'blur 30px', 'literal colour in --hd-x outside a token block', 'custom property --vd-y', 'url(https://example.com', 'light-dark()', 'must start with "HomeDash "', '[data-theme]', '!important', 'cc- prefix'])
      assert.ok(v.includes(want), 'missing: ' + want + '\n' + v);
    assert.deepEqual(lintCss(GOOD, { file: 'good.css', kind: 'family' }), []); }],
  ['every bundle part passes the guard', () => { for (const p of PARTS) assert.deepEqual(lintCss(read(p), { file: p, kind: /family-/.test(p) ? 'family' : 'part' }), [], p); }],
  ['the JS guard reports forbidden calls', () => { const v = lintJs('a.innerHTML = x; b.setAttribute("style", y); alert(1); new Function("x"); c.style.cssText = ""', 'x.js').join('\n');
    for (const want of ['innerHTML', "setAttribute('style'", 'browser dialogs', 'new Function', 'style.cssText']) assert.ok(v.includes(want), want);
    assert.deepEqual(lintJs('E.sheet.confirm(x); el.style.setProperty("--hd-v", 1);', 'ok.js'), []); }],
  ['engine JS is clean and within the line budget', () => { for (const f of walk(path.join(root, 'ui/js/homedash'))) { const t = fs.readFileSync(f, 'utf8'), n = t.split('\n').length;
    assert.deepEqual(lintJs(t, f), [], f); assert.ok(n < 1100, `${f}: ${n} lines`); if (n > 700) console.warn(`WARN ${f}: ${n} lines (aim under 700)`); } }]
];
```
- [ ] Run it. Expected: FAIL (css-lint.mjs missing).
- [ ] `css-lint.mjs` (full):
```js
// HomeDash stylesheet and source guard (visual part 9.4). lintCss/lintJs return readable violations; [] is clean.
export const TOKEN_BLOCK = /^(?:\.hd-root(?:\[data-hd-(?:palette|scheme)="\w+"\])?|\[data-hd-family="\w+"\])$/;
export const MOTION_GATE = '.hd-root:not([data-hd-motion="reduced"]):not([data-hd-motion="none"])';
const COLOUR = /#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color)\(/i;
const NAMED = /(^|[\s,(])(white|black|red|green|blue|gray|grey|silver|orange|yellow|purple|pink|navy|teal|maroon|olive|lime|aqua|fuchsia|gold|brown|cyan|magenta)(?=$|[\s,)])/i;
const COLOR_PROPS = /^(color|background|background-color|background-image|border|border-(top|right|bottom|left)|border(-(top|right|bottom|left))?-color|outline|outline-color|fill|stroke|box-shadow|text-shadow|caret-color|accent-color|text-decoration-color|column-rule|stop-color|flood-color)$/;
const MOTION = /^(transition|transition-property|transition-duration|animation|animation-name|animation-duration)$/;
export function leafRules(text) { const rules = [], stack = []; let buf = '';
  for (const ch of text) { if (ch === '{') { stack.push(buf.trim()); buf = ''; } else if (ch === '}') { const head = stack.pop() || ''; if (buf.trim()) rules.push({ selector: head, body: buf.trim(), context: [...stack] }); buf = ''; } else buf += ch; }
  return rules; }
export function splitSelectors(sel) { const out = []; let depth = 0, cur = '';
  for (const ch of sel) { if (ch === '(') depth++; if (ch === ')') depth--; if (ch === ',' && !depth) { out.push(cur.trim()); cur = ''; } else cur += ch; }
  if (cur.trim()) out.push(cur.trim()); return out; }
const decls = body => body.split(';').map(r => { const i = r.indexOf(':'); return i < 0 ? null : [r.slice(0, i).trim().toLowerCase(), r.slice(i + 1).trim()]; }).filter(d => d && d[0]);
export function lintCss(text, { file = 'css', kind = 'part' } = {}) {
  const v = [], css = text.replace(/\/\*[\s\S]*?\*\//g, ''), lines = text.split('\n').length;
  if (kind !== 'lab' && lines > 900) v.push(`${file}: ${lines} lines, want at most 900`);
  if (/light-dark\(/i.test(css)) v.push(`${file}: uses light-dark(); select scheme values with [data-hd-scheme]`);
  if (/\[data-theme\b/.test(css)) v.push(`${file}: keys on [data-theme]; read the bridge tokens`);
  if (/(^|[^a-z-])cc-[a-z]/i.test(css)) v.push(`${file}: uses the cc- prefix`);
  for (const m of css.matchAll(/url\(\s*(?:"([^"]*)"|'([^']*)'|([^)\s]*))\s*\)/g)) { const u = m[1] ?? m[2] ?? m[3] ?? '';
    const rest = u.slice(18).replace(/http:\/\/www\.w3\.org\/2000\/svg/g, '');
    if (!(u.startsWith('/fonts/') || (u.startsWith('data:image/svg+xml') && u.length <= 4096 && !/script|foreignObject|https?:|\/\//i.test(rest)))) v.push(`${file}: url(${u.slice(0, 60)}) is not allowed`); }
  for (const rule of leafRules(css)) {
    const sel = rule.selector, parts = splitSelectors(sel), ds = decls(rule.body);
    if (sel.startsWith('@font-face')) { const f = ds.find(d => d[0] === 'font-family'); if (!f || !/^["']HomeDash /.test(f[1])) v.push(`${file}: @font-face family ${f ? f[1] : '(none)'} must start with "HomeDash "`); continue; }
    if (rule.context.some(h => h.startsWith('@keyframes'))) { for (const [n] of ds) if (n !== 'transform' && n !== 'opacity') v.push(`${file}: keyframes animate ${n}; only transform and opacity`); continue; }
    if (kind !== 'lab') for (const p of parts) for (const m of p.replace(/\[[^\]]*\]/g, '').matchAll(/\.(-?[_a-zA-Z][\w-]*)/g)) if (!m[1].startsWith('hd-')) v.push(`${file}: class .${m[1]} in "${p}" does not start with hd-`);
    const tokenBlock = parts.every(p => TOKEN_BLOCK.test(p));
    for (const [name, value] of ds) {
      if (/!important/i.test(value) && !parts.every(p => /\[hidden\]$/.test(p))) v.push(`${file}: !important in ${sel} { ${name} }`);
      if (name.startsWith('--')) {
        if (kind !== 'lab' && !name.startsWith('--hd-')) v.push(`${file}: custom property ${name} outside the --hd- namespace`);
        if ((COLOUR.test(value) || NAMED.test(value)) && !tokenBlock) v.push(`${file}: literal colour in ${name} outside a token block (${sel})`);
        continue;
      }
      if (COLOUR.test(value)) v.push(`${file}: literal colour in ${sel} { ${name}: ${value} }`);
      if (COLOR_PROPS.test(name) && NAMED.test(value)) v.push(`${file}: named colour in ${sel} { ${name}: ${value} }`);
      if (name === 'text-transform' && /uppercase/i.test(value)) v.push(`${file}: uppercase text in ${sel}`);
      if (MOTION.test(name) && !/^(none|0m?s)\b/i.test(value) && !(rule.context.some(h => /prefers-reduced-motion\s*:\s*no-preference/.test(h)) && parts.every(p => p.includes(MOTION_GATE)))) v.push(`${file}: ungated motion in ${sel} { ${name} }`);
      if (/^(-webkit-)?backdrop-filter$/.test(name) && value !== 'none') {
        if (!parts.every(p => /\.hd-(glass-pane|sheet)(?=[.:[]|$)[^\s>+~]*$/.test(p))) v.push(`${file}: backdrop-filter outside .hd-glass-pane and .hd-sheet (${sel})`);
        for (const m of value.matchAll(/blur\(\s*([\d.]+)px/g)) if (Number(m[1]) > 24) v.push(`${file}: blur ${m[1]}px over 24px`);
      }
      if (kind === 'family' && /^(stroke-width|border-radius|box-shadow)$/.test(name) && !/var\(--hd-/.test(value) && !/^(0|none)$/.test(value)) v.push(`${file}: ${name} in ${sel} must use a --hd- token`);
    }
  }
  return v;
}
const JS_RULES = [[/\.innerHTML\b/, 'innerHTML'], [/\.outerHTML\b/, 'outerHTML'], [/insertAdjacentHTML/, 'insertAdjacentHTML'], [/\beval\s*\(/, 'eval'], [/new\s+Function\s*\(/, 'new Function'],
  [/document\.write/, 'document.write'], [/(^|[^.\w])(alert|confirm|prompt)\s*\(/, 'browser dialogs'], [/setAttribute\(\s*['"]style['"]/, "setAttribute('style'"], [/\.style\.cssText/, 'style.cssText'],
  [/\.style\s*=[^=]/, 'style assignment'], [/setAttribute\(\s*['"]on[a-z]+['"]/, 'inline event handler attribute']];
export function lintJs(text, file) { return JS_RULES.filter(([re]) => re.test(text)).map(([, what]) => `${file}: uses ${what}`); }
```
- [ ] Run `npm run test:homedash`. Expected: PASS. If a real part fails, fix the CSS, not the rule.
- [ ] Run impact on `tests` in `scripts/test-ui-regressions.mjs`. Then insert right after the `for (const name of [...]) {...}` block (`:5880-5893`) and before `let failures = 0;`:
```js
tests.push(['HomeDash engine, guard and lab contracts', () => {
  const result = spawnSync(process.execPath, ['scripts/test-homedash-ui.mjs'], { cwd: root, encoding: 'utf8', timeout: 120000 });
  assert.equal(result.status, 0, `${result.error?.message || ''}\n${result.stdout}${result.stderr}`);
}]);
```
- [ ] Run `node scripts/test-homedash-ui.mjs`. Expected: PASS. Optionally run `npm run test:ui-regressions` once (long).
- [ ] Commit `test(homedash): stylesheet and source guard; run it with the UI regressions`.

## Task 10: hd-geometry.js

**Files:** `ui/js/homedash/hd-geometry.js`, `scripts/homedash-tests/geometry.test.mjs`. **Read first:** visual part 3 and 4.1 (H3–H6, H8).

- [ ] Failing test:
```js
import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs';
const Gm = () => loadEngine(['hd-core.js', 'hd-geometry.js']).E.geometry;
const cubic = (p0, p1, p2, p3, t) => (1 - t) ** 3 * p0 + 3 * (1 - t) ** 2 * t * p1 + 3 * (1 - t) * t * t * p2 + t ** 3 * p3;
export const tests = [
  ['arc, ticks, needle', () => { const g = Gm();
    assert.equal(g.arc(50, 50, 40, 0, 90), 'M50 10A40 40 0 0 1 90 50'); assert.equal(g.arc(50, 50, 40, 225, 270), 'M21.716 78.284A40 40 0 1 1 78.284 78.284');
    assert.equal(g.arc(50, 50, 42, -135, 270), g.arc(50, 50, 42, 225, 270)); assert.equal((g.arc(50, 50, 40, 0, 360).match(/A/g) || []).length, 2);
    assert.equal((g.ticks(50, 50, 40, 46, 225, 270, 2).match(/M/g) || []).length, 3);
    assert.equal(g.needle(50, 50, 44, 12, 3.2, 0.8), 'M49.6 6L50.4 6L51.6 50L51.6 62L48.4 62L48.4 50Z'); }],
  ['angles and needle jitter', () => { const g = Gm();
    assert.equal(g.valueToAngle(50, 0, 100, -135, 270), 0); assert.equal(g.valueToAngle(150, 0, 100, -135, 270), 135); assert.equal(g.valueToAngle(NaN, 0, 100, -135, 270), -135);
    assert.equal(g.needleChanged(50, 50.1, 0, 100), false); assert.equal(g.needleChanged(50, 50.3, 0, 100), true); eq(g.minMaxLast([3, null, 1, 5]), { min: 1, max: 5, last: 5 }); assert.equal(g.minMaxLast([null]), null); }],
  ['sparkline: padding, gaps, flat, empty', () => { const g = Gm(); const s = g.sparkline([1, 2, 3], { w: 100, h: 40 });
    assert.ok(s.line.startsWith('M2 ')); assert.equal((s.line.match(/C/g) || []).length, 2); assert.ok(s.area.endsWith('Z')); assert.equal(s.last.x, 98); assert.ok(Math.abs(s.last.y - 5.484) < 0.01);
    assert.equal((g.sparkline([1, null, 3, 4], { w: 100, h: 40 }).line.match(/M/g) || []).length, 2); assert.equal(g.sparkline([null, null]).line, ''); assert.ok(!g.sparkline([5, 5, 5]).line.includes('NaN')); }],
  ['monotone sparkline never overshoots a segment', () => { const g = Gm(); const { line } = g.sparkline([0, 10, 10, 10, 0, 5, 5, 20, 19, 30], { w: 300, h: 100 });
    let x0, y0; for (const tok of line.match(/[MC][^MC]*/g)) { const n = tok.slice(1).trim().split(/[\s,]+/).map(Number);
      if (tok[0] === 'M') { [x0, y0] = n; continue; } const [, c1, , c2, x3, y3] = n;
      for (let t = 0; t <= 1; t += 0.05) { const y = cubic(y0, c1, c2, y3, t); assert.ok(y >= Math.min(y0, y3) - 1e-6 && y <= Math.max(y0, y3) + 1e-6, `overshoot ${y}`); } x0 = x3; y0 = y3; } }],
  ['seven-segment map and paths', () => { const g = Gm();
    assert.deepEqual('0123456789'.split('').map(c => g.SEGMENTS[c].length), [6, 2, 5, 5, 4, 5, 6, 3, 7, 6]); assert.equal(g.SEGMENTS['-'], 'g');
    eq(g.sevenSeg('21.5'), [{ char: '2', segs: ['a', 'b', 'd', 'e', 'g'], dp: false }, { char: '1', segs: ['b', 'c'], dp: true }, { char: '5', segs: ['a', 'c', 'd', 'f', 'g'], dp: false }]);
    eq(g.sevenSeg('.5')[0], { char: ' ', segs: [], dp: true }); const p = g.segmentPaths(12, 20, 2.4, 0.6);
    for (const k of ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'dp']) assert.ok(p[k].startsWith('M') && p[k].endsWith('Z'), k); }]
];
```
- [ ] Run it. Expected: FAIL. Implement:
```js
// HomeDash geometry (pure): arc, tick and needle paths, monotone (Fritsch-Carlson) sparklines, seven-segment digits.
// Angles: degrees from 12 o'clock, clockwise.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-geometry.js: load hd-core.js first');
    const clamp = E.core.clamp, n = v => String(Math.round(v * 1000) / 1000), rad = d => d * Math.PI / 180;
    const point = (cx, cy, r, deg) => [cx + r * Math.sin(rad(deg)), cy - r * Math.cos(rad(deg))];
    function arc(cx, cy, r, start, sweep) {
        const s = clamp(sweep, 0, 360);
        if (s >= 360) return arc(cx, cy, r, start, 180) + arc(cx, cy, r, start + 180, 180).replace(/^M[^A]*/, '');
        const [x0, y0] = point(cx, cy, r, start), [x1, y1] = point(cx, cy, r, start + s);
        return 'M' + n(x0) + ' ' + n(y0) + 'A' + n(r) + ' ' + n(r) + ' 0 ' + (s > 180 ? 1 : 0) + ' 1 ' + n(x1) + ' ' + n(y1);
    }
    function ticks(cx, cy, r1, r2, start, sweep, steps) { let d = '';
        for (let i = 0; i <= steps; i++) { const a = start + sweep * i / steps, [x0, y0] = point(cx, cy, r1, a), [x1, y1] = point(cx, cy, r2, a); d += 'M' + n(x0) + ' ' + n(y0) + 'L' + n(x1) + ' ' + n(y1); }
        return d; }
    function needle(cx, cy, length, tail, base, tip) {
        const pts = [[cx - tip / 2, cy - length], [cx + tip / 2, cy - length], [cx + base / 2, cy], [cx + base / 2, cy + tail], [cx - base / 2, cy + tail], [cx - base / 2, cy]];
        return 'M' + pts.map(p => n(p[0]) + ' ' + n(p[1])).join('L') + 'Z';
    }
    const fraction = (v, min, max) => (typeof v !== 'number' || !Number.isFinite(v) || !(max > min) ? 0 : clamp((v - min) / (max - min), 0, 1));
    const valueToAngle = (v, min, max, start, sweep) => start + fraction(v, min, max) * sweep;
    const needleChanged = (prev, next, min, max) => (typeof prev !== 'number' || typeof next !== 'number' ? prev !== next : Math.abs(next - prev) >= 0.0025 * (max - min));
    function minMaxLast(series) { const vals = (series || []).filter(v => typeof v === 'number' && Number.isFinite(v)); return vals.length ? { min: Math.min(...vals), max: Math.max(...vals), last: vals[vals.length - 1] } : null; }
    function monotone(pts) {
        const k = pts.length, d = [], m = new Array(k);
        for (let i = 0; i < k - 1; i++) d.push((pts[i + 1][1] - pts[i][1]) / (pts[i + 1][0] - pts[i][0]));
        m[0] = d[0]; m[k - 1] = d[k - 2];
        for (let i = 1; i < k - 1; i++) m[i] = d[i - 1] * d[i] <= 0 ? 0 : (d[i - 1] + d[i]) / 2;
        for (let i = 0; i < k - 1; i++) {
            if (d[i] === 0) { m[i] = 0; m[i + 1] = 0; continue; }
            const a = m[i] / d[i], b = m[i + 1] / d[i], s = a * a + b * b;
            if (s > 9) { const t = 3 / Math.sqrt(s); m[i] = t * a * d[i]; m[i + 1] = t * b * d[i]; }
        }
        let out = '';
        for (let i = 0; i < k - 1; i++) { const [x0, y0] = pts[i], [x1, y1] = pts[i + 1], dx = (x1 - x0) / 3;
            out += 'C' + n(x0 + dx) + ' ' + n(y0 + m[i] * dx) + ' ' + n(x1 - dx) + ' ' + n(y1 - m[i + 1] * dx) + ' ' + n(x1) + ' ' + n(y1); }
        return out;
    }
    function sparkline(values, opts) {
        const o = opts || {}, w = o.w || 100, h = o.h || 40, pad = o.pad === undefined ? 2 : o.pad, yPad = o.yPad === undefined ? 0.12 : o.yPad;
        const list = Array.isArray(values) ? values : [], st = minMaxLast(list);
        if (!st) return { line: '', area: '', last: null, min: null, max: null };
        const raw = st.max - st.min, padv = raw > 0 ? raw * yPad : Math.max(Math.abs(st.max) * 0.1, 1), lo = st.min - padv, hi = st.max + padv;
        const stepX = list.length > 1 ? (w - 2 * pad) / (list.length - 1) : 0, X = i => pad + i * stepX, Y = v => h - pad - (v - lo) / (hi - lo) * (h - 2 * pad);
        const segs = []; let cur = [];
        list.forEach((v, i) => { if (typeof v === 'number' && Number.isFinite(v)) cur.push([X(i), Y(v)]); else if (cur.length) { segs.push(cur); cur = []; } });
        if (cur.length) segs.push(cur);
        let line = '', area = '';
        for (const s of segs) { const head = 'M' + n(s[0][0]) + ' ' + n(s[0][1]), body = s.length > 1 ? monotone(s) : 'h0.001';
            line += head + body; area += head + body + 'L' + n(s[s.length - 1][0]) + ' ' + n(h) + 'L' + n(s[0][0]) + ' ' + n(h) + 'Z'; }
        const lp = segs[segs.length - 1][segs[segs.length - 1].length - 1];
        return { line, area, last: { x: Math.round(lp[0] * 1000) / 1000, y: Math.round(lp[1] * 1000) / 1000 }, min: lo, max: hi };
    }
    const SEGMENTS = Object.freeze({ '0': 'abcdef', '1': 'bc', '2': 'abdeg', '3': 'abcdg', '4': 'bcfg', '5': 'acdfg', '6': 'acdefg', '7': 'abc', '8': 'abcdefg', '9': 'abcdfg', '-': 'g', ' ': '', 'E': 'adefg', 'r': 'eg', 'o': 'cdeg', 'n': 'ceg', 'C': 'adef', 'F': 'aefg', 'H': 'bcefg', 'L': 'def', 'P': 'abefg' });
    const has = k => Object.prototype.hasOwnProperty.call(SEGMENTS, k);
    function sevenSeg(text) { const out = [];
        for (const ch of String(text == null ? '' : text)) {
            if (ch === '.' || ch === ',') { if (out.length && !out[out.length - 1].dp) out[out.length - 1].dp = true; else out.push({ char: ' ', segs: [], dp: true }); continue; }
            const key = has(ch) ? ch : has(ch.toUpperCase()) ? ch.toUpperCase() : ' '; out.push({ char: key, segs: SEGMENTS[key].split(''), dp: false });
        }
        return out; }
    const hexH = (x0, x1, y, t) => { const r = t / 2; return 'M' + n(x0) + ' ' + n(y) + 'L' + n(x0 + r) + ' ' + n(y - r) + 'L' + n(x1 - r) + ' ' + n(y - r) + 'L' + n(x1) + ' ' + n(y) + 'L' + n(x1 - r) + ' ' + n(y + r) + 'L' + n(x0 + r) + ' ' + n(y + r) + 'Z'; };
    const hexV = (y0, y1, x, t) => { const r = t / 2; return 'M' + n(x) + ' ' + n(y0) + 'L' + n(x + r) + ' ' + n(y0 + r) + 'L' + n(x + r) + ' ' + n(y1 - r) + 'L' + n(x) + ' ' + n(y1) + 'L' + n(x - r) + ' ' + n(y1 - r) + 'L' + n(x - r) + ' ' + n(y0 + r) + 'Z'; };
    function segmentPaths(w, h, t, gap) { const g = gap === undefined ? t * 0.25 : gap, L = t / 2, R = w - t / 2, T = t / 2, M = h / 2, B = h - t / 2;
        return { a: hexH(L + g, R - g, T, t), g: hexH(L + g, R - g, M, t), d: hexH(L + g, R - g, B, t), f: hexV(T + g, M - g, L, t), b: hexV(T + g, M - g, R, t),
            e: hexV(M + g, B - g, L, t), c: hexV(M + g, B - g, R, t), dp: 'M' + n(w + t * 0.4) + ' ' + n(B - t / 2) + 'h' + n(t) + 'v' + n(t) + 'h' + n(-t) + 'Z' }; }
    E.geometry = Object.freeze({ point, arc, ticks, needle, fraction, valueToAngle, needleChanged, minMaxLast, sparkline, SEGMENTS, sevenSeg, segmentPaths });
})();
```
- [ ] Run it. Expected: PASS. Commit `feat(homedash): geometry path builders and seven-segment map`.

## Task 11: hd-icons.js and hd-prim-displays.js

**Files:** `ui/js/homedash/hd-icons.js`, `ui/js/homedash/hd-prim-displays.js`, `scripts/homedash-tests/displays.test.mjs`. **Read first:** Contracts C2 (display rows); frontend part 9.2; visual part 3.

- [ ] `hd-icons.js` (full):
```js
// HomeDash line icons: 24 px grid, stroked by CSS (.hd-ico). Starter set for the eight hero controls; ~70 glyphs in W4.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-icons.js: load hd-core.js first');
    const PATHS = Object.freeze({
        power: ['M12 3v8', 'M6.3 6.3a8 8 0 1 0 11.4 0'],
        bulb: ['M9 18h6', 'M10 21h4', 'M12 3a6 6 0 0 0-3.5 10.9c.9.7 1.5 1.6 1.5 2.6V17h4v-.5c0-1 .6-1.9 1.5-2.6A6 6 0 0 0 12 3z'],
        thermometer: ['M14 14.8V5a2 2 0 0 0-4 0v9.8a4 4 0 1 0 4 0z', 'M12 9v7'],
        flame: ['M12 3c1 3 5 5 5 10a5 5 0 0 1-10 0c0-2.5 1.5-4 2.5-5 .3 2 1.5 3 2.5 3 0-3-1-5 0-8z'],
        snowflake: ['M12 2v20', 'M3.3 7l17.4 10', 'M3.3 17l17.4-10', 'M9 4l3 2 3-2', 'M9 20l3-2 3 2'],
        gauge: ['M4.9 17a8 8 0 1 1 14.2 0', 'M12 13l4-4', 'M11 13a1 1 0 1 0 2 0a1 1 0 1 0-2 0'],
        droplet: ['M12 3s6 6.5 6 11a6 6 0 0 1-12 0c0-4.5 6-11 6-11z'], chart: ['M4 19h16', 'M5 15l4-4 3 3 6-6'],
        minus: ['M5 12h14'], plus: ['M12 5v14', 'M5 12h14'], alert: ['M12 4l9 16H3z', 'M12 10v4', 'M12 17h.01'],
        clock: ['M12 3a9 9 0 1 0 0 18a9 9 0 1 0 0-18z', 'M12 7v5l3 2'], lock: ['M6 11h12v9H6z', 'M8 11V8a4 4 0 0 1 8 0v3'],
        check: ['M5 12.5l4.5 4.5L19 7'], more: ['M6 12h.01', 'M12 12h.01', 'M18 12h.01'], entity: ['M5 5h14v14H5z', 'M9 12h6']
    });
    const has = name => Object.prototype.hasOwnProperty.call(PATHS, name);
    function icon(name) { const key = has(name) ? name : 'entity';
        const node = E.svg('svg', { class: 'hd-ico', viewBox: '0 0 24 24', 'aria-hidden': 'true', focusable: 'false', 'data-hd-icon': key });
        for (const d of PATHS[key]) node.appendChild(E.svg('path', { d })); return node; }
    E.icons = Object.freeze({ icon, has, names: () => Object.keys(PATHS) });
})();
```
- [ ] Failing test `displays.test.mjs`:
```js
import assert from 'node:assert/strict'; import { loadEngine, noStyleAttr, fc } from './harness.mjs';
export const FILES = ['hd-core.js', 'hd-format.js', 'hd-colour.js', 'hd-icons.js', 'hd-geometry.js', 'hd-prim-displays.js'];
export const STATES = [{}, { on: true, value: 70 }, { pending: true }, { disabled: true }, { status: 'unavailable', value: null }, { stale: true }, { readonly: true }, { status: 'error' }, { tone: 'crit', value: 96 }, { value: 0 }, { value: 100 }, { on: true }];
export const OPTS = { numeral: { chars: 6, unit: 'W' }, segReadout: { label: 'T', digits: 4, decimals: 1, unit: '%' }, arcGauge: { label: 'CPU', min: 0, max: 100, unit: '%', zones: [{ from: 80, to: 100, tone: 'crit' }] },
  needleGauge: { label: 'P', min: 0, max: 100, unit: '%' }, bar: { label: 'B', variant: 'h', readout: true }, lamp: { label: 'Lamp' }, badge: { icon: 'alert' }, iconBadge: { icon: 'droplet' },
  sparkline: { label: 'Temp', w: 120, h: 40 }, frame: { title: 'Lamp', sub: 'Office', icon: 'bulb', more: 'More' }, sheet: { title: 'Details', kind: 'moreinfo' } };
export function sweep(E, document, names) { for (const name of names) { const inst = E.prim[name](OPTS[name] || { label: name });
  assert.equal(typeof inst.set, 'function', name); const host = document.createElement('div'); host.appendChild(inst.el);
  for (const s of STATES) { inst.set(s); assert.match(inst.el.getAttribute('data-hd-status'), /^(ok|unavailable|unknown|error)$/, name); assert.equal(inst.el.getAttribute('data-hd-pending'), s.pending ? 'true' : null, name); }
  noStyleAttr(inst.el); inst.destroy(); inst.destroy(); assert.equal(host.children.length, 0, name); } }
const boot = () => loadEngine(FILES, { dom: true }); const v = (el, k) => el.style.getPropertyValue(k);
export const tests = [
  ['every display builder: common state, no style attribute, idempotent destroy', () => { const { E, document } = boot(); sweep(E, document, Object.keys(OPTS)); assert.equal(typeof E.prim.util.applyState, 'function'); }],
  ['numeral', () => { const { E } = boot(); const n = E.prim.numeral(OPTS.numeral); n.set({ value: 1234.5, decimals: 1 });
    assert.deepEqual(['.hd-num-int', '.hd-num-frac', '.hd-num-unit'].map(s => n.el.querySelector(s).textContent), ['1,234', '.5', 'W']); assert.equal(v(n.el, '--hd-num-chars'), '6');
    n.set({ value: null }); assert.equal(n.el.querySelector('.hd-num-int').textContent, fc(0x2013)); assert.equal(n.el.getAttribute('data-hd-empty'), 'true');
    n.set({ text: 'heat' }); assert.equal(n.el.querySelector('.hd-num-int').textContent, 'heat'); assert.equal(n.el.querySelector('.hd-num-frac').textContent, '');
    n.set({ value: 3, trend: 'up' }); const tr = n.el.querySelector('.hd-num-trend'); assert.equal(tr.getAttribute('data-trend'), 'up'); assert.equal(tr.textContent, fc(0x2191)); }],
  ['segReadout lights right-aligned segments', () => { const { E } = boot(); const s = E.prim.segReadout(OPTS.segReadout); s.set({ value: 21.5 });
    const lit = (i, seg) => s.el.querySelector(`g[data-index="${i}"] [data-seg="${seg}"]`).getAttribute('data-lit') === 'true';
    assert.equal(lit(1, 'a'), true); assert.equal(lit(2, 'b'), true); assert.equal(lit(2, 'a'), false); assert.equal(lit(0, 'g'), false);
    assert.equal(s.el.querySelector('g[data-index="2"] .hd-seg7-dp').getAttribute('data-lit'), 'true'); assert.equal(s.el.getAttribute('aria-label'), '21.5' + fc(0xa0) + '%'); }],
  ['gauges', () => { const { E, document } = boot(); const a = E.prim.arcGauge(OPTS.arcGauge); a.set({ value: 50 });
    assert.equal(a.el.getAttribute('role'), 'meter'); assert.equal(v(a.el, '--hd-value'), '0.5000'); assert.equal(v(a.el, '--hd-angle'), '0deg'); assert.equal(a.el.getAttribute('aria-valuenow'), '50');
    assert.equal(a.el.getAttribute('data-hd-tone'), null); assert.equal(a.el.querySelectorAll('.hd-gauge-zone').length, 1);
    const cap = a.el.querySelector('.hd-gauge-cap'); assert.equal(cap.getAttribute('cx'), '50'); assert.equal(cap.getAttribute('cy'), '8');
    assert.equal(a.el.querySelector('.hd-gauge-track').getAttribute('d'), E.geometry.arc(50, 50, 42, -135, 270)); a.set({ value: 96 }); assert.equal(a.el.getAttribute('data-hd-tone'), 'crit');
    const nd = E.prim.needleGauge(OPTS.needleGauge); nd.set({ value: 50 }); assert.equal(v(nd.el, '--hd-angle'), '0deg'); nd.set({ value: 50.1 }); assert.equal(v(nd.el, '--hd-angle'), '0deg');
    nd.set({ value: 60 }); assert.equal(v(nd.el, '--hd-angle'), '24deg'); assert.ok(nd.el.querySelector('g.hd-gauge-needle .hd-gauge-needle-shape'));
    const b = E.prim.bar(OPTS.bar); b.set({ value: 25 }); assert.equal(v(b.el, '--hd-value'), '0.2500'); assert.equal(b.el.getAttribute('data-hd-variant'), 'h'); }],
  ['lamp, badge, iconBadge', () => { const { E } = boot(); const l = E.prim.lamp(OPTS.lamp); l.set({ on: true, text: 'On' });
    assert.equal(l.el.getAttribute('data-hd-on'), 'true'); assert.equal(l.el.getAttribute('aria-label'), 'On'); l.set({ on: false }); assert.equal(l.el.getAttribute('aria-label'), 'Lamp');
    const b = E.prim.badge(OPTS.badge); b.set({ text: '<i>x</i>', tone: 'crit', icon: 'alert' }); const t = b.el.querySelector('.hd-badge-text');
    assert.equal(t.textContent, '<i>x</i>'); assert.equal(t.children.length, 0); assert.equal(b.el.getAttribute('data-tone'), 'crit'); assert.equal(b.el.querySelector('svg').getAttribute('data-hd-icon'), 'alert');
    const i = E.prim.iconBadge(OPTS.iconBadge); i.set({ icon: 'flame' }); assert.equal(i.el.querySelector('svg').getAttribute('data-hd-icon'), 'flame'); i.set({ icon: 'nope' }); assert.equal(i.el.querySelector('svg').getAttribute('data-hd-icon'), 'entity'); }],
  ['sparkline redraws on resize and never stretches', () => { const { E } = boot(); const s = E.prim.sparkline(OPTS.sparkline); s.set({ points: [1, 2, 3] });
    assert.ok(s.el.querySelector('.hd-spark-line').getAttribute('d').startsWith('M')); assert.equal(s.el.getAttribute('viewBox'), '0 0 120 40'); assert.equal(s.el.getAttribute('preserveAspectRatio'), null);
    assert.equal(s.el.querySelector('.hd-spark-last').hasAttribute('hidden'), false); s.set({ points: [], w: 200, h: 50 });
    assert.equal(s.el.getAttribute('viewBox'), '0 0 200 50'); assert.equal(s.el.getAttribute('data-hd-empty'), 'true'); assert.equal(s.el.querySelector('.hd-spark-last').hasAttribute('hidden'), true); }],
  ['frame and sheet', () => { const { E } = boot(); const f = E.prim.frame(OPTS.frame);
    assert.equal(f.el.querySelector('.hd-title').textContent, 'Lamp'); assert.equal(f.el.querySelector('.hd-more').getAttribute('aria-label'), 'More'); assert.equal(f.el.querySelector('.hd-foot').hasAttribute('hidden'), true);
    assert.equal(f.body, f.el.querySelector('.hd-body')); assert.equal(f.titleId, f.el.querySelector('.hd-title').id); f.set({ title: 'X', stateText: 'On' });
    assert.equal(f.el.querySelector('.hd-title').textContent, 'X'); assert.equal(f.el.querySelector('.hd-state-text').textContent, 'On'); assert.equal(E.prim.frame({ title: 'a' }).el.querySelector('.hd-more'), null);
    const s = E.prim.sheet(OPTS.sheet); const dlg = s.el.querySelector('.hd-sheet'); assert.equal(dlg.getAttribute('aria-labelledby'), s.el.querySelector('.hd-sheet-title').id);
    assert.equal(s.el.hasAttribute('hidden'), true); s.set({ open: true }); assert.equal(s.el.hasAttribute('hidden'), false); s.set({ open: false }); assert.equal(s.el.hasAttribute('hidden'), true); }]
];
```
- [ ] Run it. Expected: FAIL.
- [ ] Implement `hd-prim-displays.js` to the contract. Rules:
  1. File header comment. Throw unless `E.geometry` and `E.icons` exist. Set `E.prim = E.prim || {}`.
  2. Define `E.prim.util` with this exact code:
```js
const { setAttr, setVars, el } = E.core, G = E.geometry, F = E.format;
const deg = a => String(Math.round(a * 1000) / 1000) + 'deg';
function applyState(node, state, opts) {
    const s = state || {}, o = opts || {}, status = ['ok', 'unavailable', 'unknown', 'error'].includes(s.status) ? s.status : 'ok';
    setAttr(node, 'data-hd-status', status); setAttr(node, 'data-hd-pending', !!s.pending); setAttr(node, 'data-hd-stale', !!s.stale);
    setAttr(node, 'data-hd-readonly', !!s.readonly); setAttr(node, 'data-hd-disabled', !!s.disabled); setAttr(node, 'data-hd-tone', ['ok', 'warn', 'crit'].includes(s.tone) ? s.tone : null);
    if (!o.interactive) return;
    const off = !!s.disabled || status === 'unavailable';
    for (const t of o.targets || [node]) { if (o.readonlyAria === false) setAttr(t, 'aria-disabled', off || !!s.readonly); else { setAttr(t, 'aria-disabled', off); setAttr(t, 'aria-readonly', !!s.readonly); } }
}
function writeValue(node, state, opts, aria) {
    const s = state || {}, o = opts || {}, min = typeof s.min === 'number' ? s.min : o.min ?? 0, max = typeof s.max === 'number' ? s.max : o.max ?? 100;
    const v = typeof s.value === 'number' && Number.isFinite(s.value) ? s.value : null, frac = v === null ? 0 : G.fraction(v, min, max);
    setVars(node, { '--hd-value': frac.toFixed(4) }); setAttr(node, 'data-hd-empty', v === null);
    if (aria !== false) { setAttr(node, 'aria-valuemin', String(min)); setAttr(node, 'aria-valuemax', String(max)); setAttr(node, 'aria-valuenow', v === null ? null : String(v));
        setAttr(node, 'aria-valuetext', s.valueText || F.valueText(v, { decimals: o.decimals, lang: o.lang, unit: o.unit, grouping: o.grouping })); }
    return { v, frac, min, max };
}
const root = (tag, cls, opts, attrs) => el(tag, Object.assign({ class: cls, 'data-hd-size': (opts && opts.size) || 'm' }, opts && opts.variant ? { 'data-hd-variant': opts.variant } : {}, attrs));
function finish(node, set, extra) { let done = false; return Object.assign({ el: node, set, destroy() { if (done) return; done = true; node.remove(); } }, extra); }
E.prim.util = Object.freeze({ applyState, writeValue, root, finish, deg });
```
  3. Every builder starts from `util.root` and returns `util.finish(el, set, extra)`. The first line of `set` is `util.applyState(el, state)`. `set` must also tolerate an empty state.
  4. **numeral:** uses `E.format.split(value, {decimals: state.decimals ?? opts.decimals, lang, unit: state.unit ?? opts.unit})`. When `state.text` is a string, write it into `.hd-num-int` and clear frac and unit. Trend glyphs come from `String.fromCodePoint(0x2191, 0x2193, 0x2192)`; any other trend clears the glyph and the attribute.
  5. **segReadout:** text = `state.text ?? E.format.number(value, {decimals, lang, grouping:false})`. Cells = `sevenSeg(text)`, right-aligned into `digits`. When value is null and there is no text, every path is unlit and `data-hd-empty` is true. `aria-label` = valueText, or the en dash.
  6. **arcGauge / needleGauge:**
     - Angle = start + frac × sweep, with (−135, 270) for arc and (−120, 240) for needle. Write it with `util.deg`.
     - Cap = `point(50,50,42,angle)` written as attributes, values formatted like geometry `n()`: `String(Math.round(v*1000)/1000)`.
     - Tone = `state.tone`, else the first zone with `from ≤ v ≤ to`.
     - The needle keeps its previous value and angle when `needleChanged` is false.
     - Min and max labels use `E.format.number(min/max, {lang})`.
     - Zones and ticks are built at build time.
  7. **bar:** zones get `--hd-from`/`--hd-to` (fractions with 4 decimals) at build time.
  8. **lamp, badge, iconBadge, frame, sheet:** follow the C2 rows. Ids come from `E.core.uid('hd-t')`. Icons come from `E.icons.icon(name)` and are replaced only when the name changes.
  9. **sparkline:** use `E.geometry.sparkline(points, {w, h})`. Write `width`, `height` and `viewBox` only when w or h change. Hide the circle and set `data-hd-empty` when there is no last point.
  10. Aim for at most about 600 lines.
- [ ] Run it. Expected: PASS. Commit `feat(homedash): icon starter set and static display primitives`.

## Task 12: hd-prim-controls.js

**Files:** `ui/js/homedash/hd-prim-controls.js`, `scripts/homedash-tests/controls.test.mjs`. **Read first:** Contracts C2 (control rows); visual part 4.1 H1–H3.

- [ ] Failing test:
```js
import assert from 'node:assert/strict'; import { loadEngine, fc } from './harness.mjs'; import { FILES, sweep } from './displays.test.mjs';
const OPTS = { toggle: { label: 'Lamp' }, rocker: { label: 'Fan' }, pushButton: { label: 'Scene', icon: 'bulb' }, slider: { label: 'Dim', variant: 'v', unit: '%' }, knob: { label: 'Dim', variant: 'ring', unit: '%' },
  stepper: { label: 'Target', min: 5, max: 30, step: 0.5, unit: 'C', decimals: 1, decLabel: 'Decrease', incLabel: 'Increase' },
  segmented: { label: 'Mode', options: [{ id: 'heat', label: 'Heat' }, { id: 'cool', label: 'Cool' }, { id: 'off', label: 'Off' }] }, colourWheel: { label: 'Colour', diameter: 120 }, ctStrip: { label: 'White', min: 2200, max: 6500 } };
const boot = () => loadEngine([...FILES, 'hd-prim-controls.js'], { dom: true }); const v = (el, k) => el.style.getPropertyValue(k);
export const tests = [
  ['every control builder passes the common sweep', () => { const { E, document } = boot(); for (const n of Object.keys(OPTS)) { const orig = E.prim[n]; E.prim[n] = o => orig(Object.assign({}, OPTS[n], o)); } sweep(E, document, Object.keys(OPTS)); }],
  ['toggle and rocker are switches', () => { const { E } = boot(); const t = E.prim.toggle(OPTS.toggle);
    assert.equal(t.el.tagName, 'BUTTON'); assert.equal(t.el.getAttribute('role'), 'switch'); assert.equal(t.el.getAttribute('type'), 'button'); assert.equal(t.el.getAttribute('aria-label'), 'Lamp');
    t.set({ on: true }); assert.equal(t.el.getAttribute('aria-checked'), 'true'); assert.equal(t.el.getAttribute('data-hd-on'), 'true');
    t.set({ readonly: true }); assert.equal(t.el.getAttribute('aria-readonly'), 'true'); t.set({ status: 'unavailable' }); assert.equal(t.el.getAttribute('aria-disabled'), 'true');
    const r = E.prim.rocker(OPTS.rocker); assert.deepEqual(r.el.querySelectorAll('.hd-rocker-legend').map(x => x.textContent), ['I', 'O']); r.set({ on: false }); assert.equal(r.el.getAttribute('aria-checked'), 'false'); }],
  ['pushButton maps read-only to aria-disabled', () => { const { E } = boot(); const p = E.prim.pushButton(OPTS.pushButton);
    assert.equal(p.el.querySelector('.hd-push-label').textContent, 'Scene'); assert.ok(p.el.querySelector('svg.hd-ico')); p.set({ readonly: true });
    assert.equal(p.el.getAttribute('aria-disabled'), 'true'); assert.equal(p.el.getAttribute('aria-readonly'), null); }],
  ['slider and knob write value, angle and ARIA', () => { const { E } = boot(); const s = E.prim.slider(OPTS.slider);
    assert.equal(s.el.getAttribute('aria-orientation'), 'vertical'); s.set({ value: 25 }); assert.equal(v(s.el, '--hd-value'), '0.2500'); assert.equal(s.el.getAttribute('aria-valuenow'), '25');
    assert.equal(s.el.getAttribute('aria-valuetext'), '25' + fc(0xa0) + '%'); s.set({ value: null }); assert.equal(s.el.getAttribute('aria-valuenow'), null); assert.equal(s.el.getAttribute('aria-valuetext'), fc(0x2013));
    const k = E.prim.knob(OPTS.knob); k.set({ value: 0 }); assert.equal(v(k.el, '--hd-angle'), '-135deg'); const h = k.el.querySelector('.hd-dial-knob');
    assert.equal(h.getAttribute('cx'), '20.302'); assert.equal(h.getAttribute('cy'), '79.698'); k.set({ value: 100 }); assert.equal(v(k.el, '--hd-angle'), '135deg');
    assert.equal(k.el.querySelector('.hd-dial-readout .hd-num-int').textContent, '100'); assert.equal(k.el.querySelector('.hd-dial-track').getAttribute('d'), E.geometry.arc(50, 50, 42, -135, 270)); }],
  ['stepper disables a button at its limit', () => { const { E } = boot(); const s = E.prim.stepper(OPTS.stepper); const [dec, inc] = s.el.querySelectorAll('button.hd-step');
    s.set({ value: 5 }); assert.equal(dec.getAttribute('aria-disabled'), 'true'); assert.equal(inc.getAttribute('aria-disabled'), null); assert.equal(s.el.querySelector('[role="spinbutton"]').getAttribute('aria-valuenow'), '5');
    s.set({ value: 30 }); assert.equal(inc.getAttribute('aria-disabled'), 'true'); assert.equal(dec.getAttribute('aria-disabled'), null); assert.equal(dec.getAttribute('aria-label'), 'Decrease'); }],
  ['segmented uses a roving tabindex', () => { const { E } = boot(); const g = E.prim.segmented(OPTS.segmented); g.set({ value: 'cool' }); const o = g.el.querySelectorAll('.hd-seg-opt');
    assert.equal(v(g.el, '--hd-index'), '1'); assert.equal(v(g.el, '--hd-count'), '3'); assert.deepEqual(o.map(x => [x.getAttribute('aria-checked'), x.getAttribute('tabindex')]), [['false', '-1'], ['true', '0'], ['false', '-1']]);
    g.set({ value: 'zzz' }); assert.equal(v(g.el, '--hd-index'), '-1'); assert.equal(o[0].getAttribute('tabindex'), '0'); }],
  ['colour wheel and colour temperature strip', () => { const { E } = boot(); const w = E.prim.colourWheel(OPTS.colourWheel); w.set({ hue: 90, sat: 1 });
    assert.equal(v(w.el, '--hd-wheel-x'), '1.0000'); assert.equal(v(w.el, '--hd-wheel-y'), '0.5000'); assert.equal(v(w.el, '--hd-wheel-rgb'), '128 255 0'); assert.ok(w.el.querySelector('canvas.hd-wheel-canvas'));
    const c = E.prim.ctStrip(OPTS.ctStrip); c.set({ value: 4350 }); assert.equal(v(c.el, '--hd-value'), '0.5000'); assert.equal(c.el.getAttribute('aria-valuetext'), '4350' + fc(0xa0) + 'K'); }]
];
```
- [ ] Run it. Expected: FAIL.
- [ ] Implement. Rules:
  1. Throw unless `E.prim && E.prim.util && E.prim.numeral` exist. This file loads after hd-prim-displays.
  2. Builders follow the C2 rows exactly.
  3. Interactive builders call `applyState(el, state, {interactive:true})`. For pushButton add `readonlyAria:false`. The stepper's targets are both buttons plus the spinbutton, and each button then additionally gets `aria-disabled` at its limit.
  4. `toggle`, `rocker` and `pushButton` write `aria-checked` ('true'/'false') where the row says so, plus `data-hd-on`.
  5. `slider`, `knob` and `ctStrip` use `util.writeValue`. ctStrip passes `{decimals:0, grouping:false, unit:'K'}`.
  6. `knob`:
     - angle = −135 + frac × 270
     - handle attributes = `point(50,50,42,angle)` with the 3-decimal format
     - the readout is an `E.prim.numeral({chars:4, unit:opts.unit, lang})` instance; its `set` receives `{value, decimals:opts.decimals}`
  7. `stepper` puts an `E.prim.numeral` inside the spinbutton.
  8. `segmented` writes `--hd-count` at build time and `--hd-index` on every set.
  9. `colourWheel`:
     - At build: if `canvas.getContext` exists, paint the HSV disc at `diameter × (window.devicePixelRatio || 1)` with `ImageData`. For each pixel: hue = atan2(dx, −dy) in degrees (0 at the top, clockwise), sat = r, colour = `E.colour.hsvToRgb(hue, sat, 1)`, alpha 0 outside the disc. Set the canvas `width`/`height` attributes to the pixel size.
     - On set: write the three variables listed in C2.
  10. No pointer or keyboard listeners in M0; P1 adds them.
  11. Aim for at most about 500 lines.
- [ ] Run it. Expected: PASS. Commit `feat(homedash): static control primitives`.

## Task 13: hd-widgets.js, entity tile, load contract

**Files:** `ui/js/homedash/hd-widgets.js`, `ui/js/homedash/widgets/hd-w-misc.js`, `scripts/homedash-tests/widgets.test.mjs`, `scripts/homedash-tests/contract.test.mjs`. **Read first:** frontend part 2.1–2.3, 2.5, 9.1; Contracts C3.

- [ ] Failing tests. `widgets.test.mjs`:
```js
import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, fc } from './harness.mjs';
const boot = () => loadEngine(ENGINE_ORDER, { dom: true });
const WORDS = { 'homedash.state.on': 'On', 'homedash.state.pending': 'Sending', 'homedash.state.unavailable': 'Unavailable' };
const view = p => Object.assign({ id: 'x:1', kind: 'switch', name: 'Desk lamp', value: 'off', attributes: {}, available: true, status: 'ok', readonly: false, capabilities: ['toggle'], pending: false, stale: false, error: null, tone: null }, p);
function factory(state) { return ctx => ({ entity: () => state.view, can: () => true, command: () => Promise.resolve({}), moreInfo() {}, history: () => Promise.resolve([]), announce() {} }); }
export const tests = [
  ['register validates its input', () => { const { E } = boot();
    assert.throws(() => E.widgets.register('Bad-Type', { bindings: [], variants: ['a'], create() {} })); assert.throws(() => E.widgets.register('zz', { bindings: [], variants: ['a'] }));
    assert.throws(() => E.widgets.register('entity', { bindings: [], variants: ['tile'], create() {} }), /registered/); assert.ok(E.widgets.types().includes('entity')); }],
  ['host lifecycle: create, synchronous update, attributes, resize, mode, destroy', () => { const { E, document } = boot(); const calls = []; let api;
    E.widgets.register('zz_probe', { bindings: ['primary'], variants: ['a', 'b'], create(a) { api = a; calls.push('create'); return { update(v) { calls.push(['update', v.mode, v.entities.primary && v.entities.primary.name]); }, resize(s) { calls.push(['resize', s.w, s.h]); }, destroy() { calls.push('destroy'); } }; } });
    const state = { view: view({ value: 'on', pending: true, stale: true, readonly: true, tone: 'crit' }) };
    const h = E.widgets.host({ widget: { id: 'w1', type: 'zz_probe', variant: 'b', title: '', bindings: { primary: 'x:1' } }, size: { w: 2, h: 1 }, t: E.core.makeT(WORDS), api: factory(state), moreLabel: 'More' });
    document.body.appendChild(h.el); assert.deepEqual(calls.slice(0, 2), ['create', ['update', 'view', 'Desk lamp']]);
    for (const k of ['body', 'widget', 'variant', 'family', 'mode', 't', 'fmt', 'prim', 'icons', 'signal', 'lang', 'entity', 'can', 'command', 'moreInfo', 'history', 'announce']) assert.ok(k in api, k);
    assert.equal(api.variant, 'b'); assert.equal(api.body, h.body); assert.throws(() => { api.mode = 'x'; });
    const a = n => h.el.getAttribute(n); assert.deepEqual([a('data-hd-type'), a('data-hd-variant'), a('data-hd-size'), a('data-hd-on'), a('data-hd-pending'), a('data-hd-stale'), a('data-hd-readonly'), a('data-hd-tone'), a('data-hd-status')], ['zz_probe', 'b', '2x1', 'true', 'true', 'true', 'true', 'crit', 'ok']);
    assert.equal(h.el.querySelector('.hd-title').textContent, 'Desk lamp'); assert.equal(h.el.querySelector('.hd-state-text').textContent, 'Sending'); assert.ok(h.el.querySelector('.hd-more'));
    h.resize({ w: 3, h: 2, px: { width: 300, height: 200 } }); assert.equal(a('data-hd-size'), '3x2'); assert.deepEqual(calls[2], ['resize', 3, 2]);
    h.setMode('design'); assert.equal(h.body.hasAttribute('inert'), true); assert.equal(api.mode, 'design'); state.view = view({ available: false, status: 'unavailable' }); h.update(); assert.equal(a('data-hd-status'), 'unavailable');
    const signal = api.signal; h.destroy(); h.destroy(); assert.equal(signal.aborted, true); assert.equal(calls.filter(c => c === 'destroy').length, 1); assert.equal(document.body.children.length, 0); }],
  ['the api factory must supply every function', () => { const { E } = boot();
    assert.throws(() => E.widgets.host({ widget: { id: 'w', type: 'entity', bindings: {} }, size: { w: 1, h: 1 }, t: k => k, api: () => ({ entity() {} }) }), /api\.can/); }],
  ['entity tile shows numbers, binary state and text', () => { const { E } = boot(); const state = { view: view({ kind: 'sensor', value: 21.5, unit: 'C' }) };
    const h = E.widgets.host({ widget: { id: 'w', type: 'entity', variant: 'tile', bindings: { primary: 'x' } }, size: { w: 2, h: 1 }, t: E.core.makeT(WORDS), api: factory(state) });
    const num = h.el.querySelector('.hd-numeral'), lamp = h.el.querySelector('.hd-lamp');
    assert.equal(num.querySelector('.hd-num-int').textContent, '21'); assert.equal(lamp.hasAttribute('hidden'), true);
    state.view = view({ value: 'on' }); h.update(); assert.equal(lamp.hasAttribute('hidden'), false); assert.equal(lamp.getAttribute('data-hd-on'), 'true'); assert.equal(num.hasAttribute('hidden'), true);
    state.view = view({ value: 'heat' }); h.update(); assert.equal(num.querySelector('.hd-num-int').textContent, 'heat'); state.view = null; h.update(); assert.equal(num.querySelector('.hd-num-int').textContent, fc(0x2013)); }]
];
```
`contract.test.mjs`:
```js
import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, read, ENGINE_DIR } from './harness.mjs';
const PARTS = { 'hd-core.js': 'core', 'hd-format.js': 'format', 'hd-colour.js': 'colour', 'hd-grid.js': 'grid', 'hd-layout.js': 'layout', 'hd-doc.js': 'doc', 'hd-history.js': 'history', 'hd-icons.js': 'icons', 'hd-geometry.js': 'geometry', 'hd-prim-displays.js': 'prim', 'hd-prim-controls.js': 'prim', 'hd-widgets.js': 'widgets' };
const PRIMS = ['toggle', 'rocker', 'pushButton', 'slider', 'knob', 'stepper', 'segmented', 'numeral', 'segReadout', 'arcGauge', 'needleGauge', 'bar', 'lamp', 'badge', 'iconBadge', 'sparkline', 'frame', 'sheet', 'colourWheel', 'ctStrip'];
export const tests = [['the M0 engine loads in order without a DOM and publishes every part', () => {
  const { E } = loadEngine(ENGINE_ORDER); for (const [f, part] of Object.entries(PARTS)) assert.ok(E[part], f + ' publishes E.' + part);
  for (const f of ENGINE_ORDER) assert.ok(read(`${ENGINE_DIR}/${f}`).split('\n').length < 1100, f);
  assert.deepEqual(Object.keys(E.prim).filter(k => k !== 'util').sort(), PRIMS.slice().sort()); assert.ok(E.widgets.get('entity')); }]];
```
- [ ] Run it. Expected: FAIL.
- [ ] Implement `hd-widgets.js`. Rules:
  1. Throw unless `E.prim.frame` exists.
  2. `register`:
     - `type` must match `/^[a-z][a-z0-9_]*$/`.
     - `bindings` must be an array of strings; `variants` a non-empty array of strings; `create` a function.
     - A duplicate type throws `widget type <t> is already registered`.
     - Store a frozen copy.
  3. `host(opts)`:
     - Look up the def (throw `unknown widget type`). Variant = `widget.variant` if it is in `def.variants`, else the first variant.
     - Build the wrapper per C3 with `E.el`.
     - Frame = `E.prim.frame({title:'', icon: widget.icon || null, more: opts.moreLabel || null})`. Set `aria-labelledby` = `frame.titleId`.
  4. `bag = E.core.bag()`. `fns = opts.api({widget, mode, signal: bag.signal})`. For each of entity, can, command, moreInfo, history, announce: if it is not a function, throw `hd-widgets: api.<name> missing`.
  5. `api = Object.freeze(Object.defineProperties({...}, {mode: {get: () => mode, enumerable: true}}))`. Fields per C3, with `fmt: E.format`, `prim: E.prim`, `icons: E.icons` and `family: widget.family || null`.
  6. `instance = def.create(api)`. It must have an `update` function; default `resize` and `destroy` to no-ops. Then call `applyMode()` and `update()` synchronously.
  7. `update()`:
     - `entities[b] = fns.entity(b)` for each binding; `p` = first binding's entity or null.
     - Status: `!p` gives 'ok'; `p.available === false` gives 'unavailable'; otherwise `p.status` if valid, else 'ok'.
     - Write `data-hd-on` from `E.format.isOn(p)` (true/false/absent), plus pending, stale, readonly and tone via `E.core.setAttr`.
     - `frame.set({title: widget.title || (p && p.name) || '', stateText: p ? E.format.stateText(p, t) : ''})`.
     - Finally `instance.update({mode, entities})`.
  8. `applyMode()`:
     - design: `frame.body` gets `inert`; wrapper gets `tabindex=0`.
     - thumb: body gets `inert`; wrapper gets `aria-hidden=true` and `data-hd-motion=none`.
     - view: remove all of these.
  9. `setMode(m)`: set the mode, call `applyMode()`, then `update()`.
  10. `resize(size)`: write `data-hd-size`, then call `instance.resize(size)`.
  11. `destroy()`: idempotent. Calls `bag.dispose()`, `instance.destroy()`, `frame.destroy()`, then removes the wrapper.
- [ ] Implement `widgets/hd-w-misc.js`. Rules:
  - Call `E.widgets.register('entity', {bindings:['primary'], variants:['tile'], create(api)})`.
  - `create` builds `div.hd-tile` inside `api.body`, containing `E.prim.iconBadge({icon: api.widget.icon || 'entity', size:'s'}).el`, `E.prim.numeral({chars:5, lang: api.lang}).el` and `E.prim.lamp({label:''}).el`.
  - `update(v)` with `e = v.entities.primary`:
    - no `e`: numeral `{value:null}` visible, lamp hidden
    - number value: numeral `{value, unit: e.unit}`, lamp hidden
    - `isOn(e)` is not null: lamp `{on, text: E.format.stateText(e, api.t), status, pending}` visible, numeral hidden
    - otherwise: numeral `{text: String(e.value)}`, lamp hidden
  - `destroy` destroys the three primitives.
- [ ] Run it. Expected: PASS. Commit `feat(homedash): widget registry, frame host and entity tile`.

## Task 14: Lab page, placeholder CSS, gallery

**Files:** `ui/testdata/homedash-lab.html`, `ui/testdata/homedash-lab.css`, `ui/testdata/homedash-lab.js`, `scripts/homedash-tests/lab.test.mjs`. **Read first:** Contracts C4–C6; `ui/desktop_theme_token_contrast_browser_test.go:37-78`; `ui/css/desktop-base.css:325-341` (`.desktop-body` is fixed and clipped, so the lab overrides that).

- [ ] Failing test `lab.test.mjs`:
```js
import assert from 'node:assert/strict'; import vm from 'node:vm'; import { read, ENGINE_ORDER } from './harness.mjs'; import { lintCss, lintJs } from './css-lint.mjs';
export const tests = [
  ['the lab page loads the engine in ENGINE_ORDER, then the probes and the gallery', () => { const srcs = [...read('ui/testdata/homedash-lab.html').matchAll(/<script src="([^"]+)"/g)].map(m => m[1]);
    assert.deepEqual(srcs, [...ENGINE_ORDER.map(f => '/js/homedash/' + f), '/testdata/homedash-lab-probes.js', '/testdata/homedash-lab.js']); }],
  ['lab CSS and JS obey the guard and parse', () => { assert.deepEqual(lintCss(read('ui/testdata/homedash-lab.css'), { file: 'homedash-lab.css', kind: 'lab' }), []);
    for (const f of ['ui/testdata/homedash-lab.js', 'ui/testdata/homedash-lab-probes.js']) { const t = read(f); assert.deepEqual(lintJs(t, f), []); new vm.Script(t, { filename: f }); } }]
];
```
Task 15 creates the probes file. Until then, create an empty stub, `(function(){'use strict';window.hdLabProbes={};})();`, so this task's test can pass.
- [ ] Run it. Expected: FAIL.
- [ ] `homedash-lab.html`:
```html
<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>HomeDash control lab (test fixture)</title>
<script>window.hdLabEarlyErrors=[];addEventListener('error',e=>hdLabEarlyErrors.push(String(e.message)));</script>
<link rel="stylesheet" href="/fonts/fonts.css"><link rel="stylesheet" href="/css/homedash.bundle.css"><link rel="stylesheet" href="/testdata/homedash-lab.css">
</head><body class="hd-lab-body"><div id="hd-lab-scope"><main id="hd-lab" aria-label="HomeDash control lab"></main></div>
<!-- one <script src="/js/homedash/<file>"> per ENGINE_ORDER entry, in order -->
<script src="/testdata/homedash-lab-probes.js"></script><script src="/testdata/homedash-lab.js"></script>
</body></html>
```
Replace the comment with the 13 script tags. Keep exactly `<script src="` spelling so the test regex matches.
- [ ] `homedash-lab.css` (full). The placeholder look uses only palette tokens, no motion:
```css
/* HomeDash control lab (test fixture, never shipped): page layout and the neutral "placeholder" family. */
.hd-lab-body { margin: 0; }
#hd-lab-scope.desktop-body { position: static; inset: auto; min-height: 0; overflow: visible; }
.hd-lab-panel { box-sizing: border-box; width: 1180px; padding: 16px; background: var(--hd-canvas); color: var(--hd-ink); }
.hd-lab-title { margin: 0 0 12px; font-size: 14px; font-weight: 600; color: var(--hd-ink); }
.hd-lab-row { display: flex; gap: 12px; align-items: flex-start; padding: 8px 0; border-top: 1px solid var(--hd-hairline); }
.hd-lab-rowhead { flex: 0 0 136px; font-size: 12px; color: var(--hd-ink-muted); overflow-wrap: anywhere; }
.hd-lab-cells { display: flex; flex: 1; flex-wrap: wrap; gap: 8px; }
.hd-lab-cell { display: flex; flex-direction: column; gap: 4px; min-width: 64px; padding: 6px; border-radius: 8px; background: var(--hd-surface-solid); }
.hd-lab-statecap { font-size: 11px; color: var(--hd-ink-muted); }
.hd-lab-stage { display: flex; align-items: center; justify-content: center; min-height: 52px; }
.hd-lab-stage > .hd-widget { width: var(--hd-lab-w); height: var(--hd-lab-h); }
.hd-lab-probe { position: absolute; visibility: hidden; pointer-events: none; }
[data-hd-family="placeholder"] :is(.hd-toggle, .hd-rocker, .hd-push, .hd-step, .hd-seg-opt, .hd-more) { box-sizing: border-box; min-width: 44px; min-height: 44px; padding: 0 8px; border: 1px solid var(--hd-ink-muted); border-radius: 6px; background: var(--hd-btn); color: var(--hd-btn-ink); font: inherit; display: inline-flex; align-items: center; justify-content: center; gap: 4px; }
[data-hd-family="placeholder"] :is(.hd-toggle, .hd-rocker, .hd-push, .hd-seg-opt)[data-hd-on="true"], [data-hd-family="placeholder"] .hd-seg-opt[aria-checked="true"] { background: var(--hd-on); color: var(--hd-on-ink); }
[data-hd-family="placeholder"] :is(.hd-toggle-glyph, .hd-seg-indicator, .hd-dial-cap, .hd-lamp-halo, .hd-rocker-led) { display: none; }
[data-hd-family="placeholder"] .hd-toggle-track { position: relative; display: block; width: 36px; height: 20px; border-radius: 10px; background: var(--hd-sunken); }
[data-hd-family="placeholder"] .hd-toggle-thumb { position: absolute; top: 2px; left: 2px; width: 16px; height: 16px; border-radius: 50%; background: var(--hd-ink); }
[data-hd-family="placeholder"] .hd-toggle[aria-checked="true"] .hd-toggle-thumb { transform: translateX(16px); background: var(--hd-on-ink); }
[data-hd-family="placeholder"] .hd-rocker-paddle { display: flex; flex-direction: column; gap: 2px; }
[data-hd-family="placeholder"] .hd-slider, [data-hd-family="placeholder"] .hd-ct { position: relative; display: flex; align-items: center; min-width: 120px; min-height: 44px; }
[data-hd-family="placeholder"] .hd-slider[data-hd-variant="v"] { min-width: 44px; min-height: 120px; justify-content: center; }
[data-hd-family="placeholder"] :is(.hd-slider-track, .hd-ct-track) { position: relative; display: block; flex: 1; height: 6px; border-radius: 3px; background: var(--hd-sunken); }
[data-hd-family="placeholder"] .hd-ct-track { height: 10px; background: linear-gradient(90deg, var(--hd-ct-2200), var(--hd-ct-4000), var(--hd-ct-6500)); }
[data-hd-family="placeholder"] .hd-slider[data-hd-variant="v"] .hd-slider-track { flex: none; width: 6px; height: 112px; }
[data-hd-family="placeholder"] .hd-slider-fill { position: absolute; inset: 0; background: var(--hd-ink); transform-origin: left center; transform: scaleX(var(--hd-value, 0)); }
[data-hd-family="placeholder"] .hd-slider[data-hd-variant="v"] .hd-slider-fill { transform-origin: center bottom; transform: scaleY(var(--hd-value, 0)); }
[data-hd-family="placeholder"] :is(.hd-slider-thumb, .hd-ct-thumb) { position: absolute; top: 50%; left: calc(var(--hd-value, 0) * 100%); width: 16px; height: 16px; margin: -8px 0 0 -8px; border-radius: 50%; background: var(--hd-ink); }
[data-hd-family="placeholder"] .hd-slider[data-hd-variant="v"] .hd-slider-thumb { top: calc((1 - var(--hd-value, 0)) * 100%); left: 50%; }
[data-hd-family="placeholder"] :is(.hd-dial, .hd-gauge) { position: relative; width: 104px; height: 104px; }
[data-hd-family="placeholder"] :is(.hd-dial-svg, .hd-gauge-svg) { display: block; width: 100%; height: 100%; }
[data-hd-family="placeholder"] :is(.hd-dial-track, .hd-gauge-track) { fill: none; stroke: var(--hd-sunken); stroke-width: 7; }
[data-hd-family="placeholder"] :is(.hd-dial-arc, .hd-gauge-arc) { fill: none; stroke: var(--hd-ink); stroke-width: 7; stroke-dasharray: calc(var(--hd-value, 0) * 100) 100; }
[data-hd-family="placeholder"] .hd-gauge-zone { fill: none; stroke: var(--hd-ink-muted); stroke-width: 3; }
[data-hd-family="placeholder"] :is(.hd-dial-ticks, .hd-gauge-ticks) { fill: none; stroke: var(--hd-ink-muted); stroke-width: 1.25; }
[data-hd-family="placeholder"] :is(.hd-dial-knob, .hd-gauge-cap, .hd-gauge-hub, .hd-gauge-needle-shape) { fill: var(--hd-ink); }
[data-hd-family="placeholder"] .hd-gauge-needle { transform-box: view-box; transform-origin: 50px 50px; transform: rotate(var(--hd-angle, 0deg)); }
[data-hd-family="placeholder"] :is(.hd-dial-readout, .hd-gauge-readout) { position: absolute; inset: 0; display: grid; place-items: center; }
[data-hd-family="placeholder"] :is(.hd-gauge-min, .hd-gauge-max) { position: absolute; bottom: 0; font-size: 11px; color: var(--hd-ink-muted); }
[data-hd-family="placeholder"] .hd-gauge-min { left: 8px; } [data-hd-family="placeholder"] .hd-gauge-max { right: 8px; }
[data-hd-family="placeholder"] .hd-stepper, [data-hd-family="placeholder"] .hd-seg { display: flex; align-items: center; gap: 4px; }
[data-hd-family="placeholder"] .hd-step-value { display: grid; place-items: center; min-width: 64px; min-height: 44px; }
[data-hd-family="placeholder"] .hd-numeral { display: inline-flex; align-items: baseline; justify-content: flex-end; min-width: calc(var(--hd-num-chars, 4) * 1ch + 2ch); font-size: 22px; color: var(--hd-ink); }
[data-hd-family="placeholder"] :is(.hd-num-frac, .hd-num-unit, .hd-num-trend) { font-size: 13px; } [data-hd-family="placeholder"] .hd-num-unit { margin-left: 2px; color: var(--hd-ink-muted); }
[data-hd-family="placeholder"] .hd-seg7 { display: inline-flex; align-items: flex-end; gap: 4px; color: var(--hd-ink); }
[data-hd-family="placeholder"] .hd-seg7-svg { height: 32px; width: 102px; } [data-hd-family="placeholder"] :is(.hd-seg7-seg, .hd-seg7-dp) { fill: var(--hd-ink); opacity: .08; }
[data-hd-family="placeholder"] :is(.hd-seg7-seg, .hd-seg7-dp)[data-lit="true"] { opacity: 1; }
[data-hd-family="placeholder"] .hd-bar { display: flex; align-items: center; gap: 8px; min-width: 140px; }
[data-hd-family="placeholder"] .hd-bar-track { position: relative; display: block; flex: 1; height: 10px; overflow: hidden; border-radius: 5px; background: var(--hd-sunken); }
[data-hd-family="placeholder"] .hd-bar:is([data-hd-variant="v"], [data-hd-variant="tank"]) { min-width: 0; } [data-hd-family="placeholder"] .hd-bar:is([data-hd-variant="v"], [data-hd-variant="tank"]) .hd-bar-track { flex: none; width: 12px; height: 88px; }
[data-hd-family="placeholder"] .hd-bar[data-hd-variant="tank"] .hd-bar-track { width: 28px; }
[data-hd-family="placeholder"] .hd-bar-fill { position: absolute; inset: 0; background: var(--hd-ink); transform-origin: left center; transform: scaleX(var(--hd-value, 0)); }
[data-hd-family="placeholder"] .hd-bar:is([data-hd-variant="v"], [data-hd-variant="tank"]) .hd-bar-fill { transform-origin: center bottom; transform: scaleY(var(--hd-value, 0)); }
[data-hd-family="placeholder"] .hd-bar-zone { display: none; }
[data-hd-family="placeholder"] .hd-lamp { display: inline-grid; place-items: center; width: 20px; height: 20px; }
[data-hd-family="placeholder"] .hd-lamp-dot { box-sizing: border-box; width: 14px; height: 14px; border: 2px solid var(--hd-ink-muted); border-radius: 50%; }
[data-hd-family="placeholder"] .hd-lamp[data-hd-on="true"] .hd-lamp-dot { border-color: var(--hd-ink); background: var(--hd-ink); }
[data-hd-family="placeholder"] .hd-badge { display: inline-flex; align-items: center; gap: 4px; padding: 2px 8px; border: 1px solid var(--hd-ink-muted); border-radius: 999px; font-size: 12px; color: var(--hd-ink); }
[data-hd-family="placeholder"] .hd-badge .hd-ico, [data-hd-family="placeholder"] .hd-push .hd-ico, [data-hd-family="placeholder"] .hd-seg-opt .hd-ico { width: 16px; height: 16px; }
[data-hd-family="placeholder"] .hd-iconbadge { display: inline-grid; place-items: center; width: 32px; height: 32px; border-radius: 50%; background: var(--hd-sunken); color: var(--hd-ink); }
[data-hd-family="placeholder"] .hd-iconbadge .hd-ico { width: 20px; height: 20px; }
[data-hd-family="placeholder"] .hd-spark { display: block; } [data-hd-family="placeholder"] .hd-spark-line { fill: none; stroke: var(--hd-ink); stroke-width: 1.75; stroke-linecap: round; stroke-linejoin: round; }
[data-hd-family="placeholder"] .hd-spark-area { fill: var(--hd-ink); opacity: .12; } [data-hd-family="placeholder"] .hd-spark-last { fill: var(--hd-ink); }
[data-hd-family="placeholder"] .hd-frame { width: 100%; height: 100%; min-width: 180px; min-height: 96px; gap: 4px; padding: 8px; border: 1px solid var(--hd-hairline); border-radius: 8px; background: var(--hd-surface-solid); }
[data-hd-family="placeholder"] .hd-widget .hd-frame { min-width: 0; min-height: 0; } [data-hd-family="placeholder"] .hd-head .hd-ico { width: 18px; height: 18px; }
[data-hd-family="placeholder"] .hd-tile { display: flex; align-items: center; gap: 6px; }
[data-hd-family="placeholder"] .hd-sheet-layer { position: static; } [data-hd-family="placeholder"] .hd-sheet { width: 200px; padding: 8px; border: 1px solid var(--hd-hairline); border-radius: 8px; background: var(--hd-surface-raised); }
[data-hd-family="placeholder"] .hd-sheet-title { margin: 0; font-size: 14px; color: var(--hd-ink); }
[data-hd-family="placeholder"] .hd-wheel { position: relative; width: 120px; height: 120px; } [data-hd-family="placeholder"] .hd-wheel-canvas { width: 100%; height: 100%; }
[data-hd-family="placeholder"] .hd-wheel-thumb { position: absolute; left: calc(var(--hd-wheel-x, .5) * 100%); top: calc(var(--hd-wheel-y, .5) * 100%); width: 14px; height: 14px; margin: -7px; border: 2px solid var(--hd-ink); border-radius: 50%; box-sizing: border-box; }
```
- [ ] `homedash-lab.js`. Write these tables verbatim, then follow the assembly rules.
```js
const DEG_C = String.fromCodePoint(0xb0) + 'C';
const FAMILIES = ['glass', 'tactile', 'minimal', 'instrument', 'placeholder'], PALETTES = ['theme', 'dark', 'light', 'custom'], MOTIONS = ['full', 'reduced', 'none'];
const LAB_STATES = ['off', 'on', 'pending', 'disabled', 'unavailable', 'stale', 'readonly', 'error', 'alert', 'min', 'max', 'focus'];
const PRIM_VARIANTS = { toggle: ['default'], rocker: ['default'], pushButton: ['chip', 'tile'], slider: ['h', 'v'], knob: ['ring', 'knob'], stepper: ['default'], segmented: ['default'], numeral: ['default'], segReadout: ['default'], arcGauge: ['default'], needleGauge: ['default'], bar: ['h', 'v', 'tank'], lamp: ['default'], badge: ['default'], iconBadge: ['default'], sparkline: ['default'], frame: ['default'], sheet: ['default'], colourWheel: ['default'], ctStrip: ['default'] };
const LAB_WIDGET_SIZES = { entity: ['1x1', '2x1'] }, CELL = 96, GAP = 12;
const LAB_WORDS = { 'homedash.state.on': 'On', 'homedash.state.off': 'Off', 'homedash.state.pending': 'Sending', 'homedash.state.unavailable': 'Unavailable', 'homedash.state.unknown': 'Unknown', 'homedash.state.error': 'Failed' };
function primOpts(type, variant, size, lang) {
    const o = { label: 'Lab ' + type, size, lang, min: 0, max: 100, unit: '%' };
    if (variant !== 'default') o.variant = variant;
    const extra = { pushButton: { label: 'Scene', icon: 'bulb' }, stepper: { min: 5, max: 30, step: 0.5, unit: DEG_C, decimals: 1, decLabel: 'Decrease', incLabel: 'Increase' },
        segmented: { options: [{ id: 'heat', label: 'Heat', icon: 'flame' }, { id: 'cool', label: 'Cool', icon: 'snowflake' }, { id: 'off', label: 'Off' }] },
        numeral: { chars: 5, decimals: 1 }, segReadout: { digits: 4, decimals: 1 }, bar: { readout: true }, iconBadge: { icon: 'droplet' }, badge: { icon: 'alert' }, sparkline: { w: 120, h: 40 },
        frame: { title: 'Living room', sub: 'Sensor', icon: 'thermometer', more: 'More' }, sheet: { title: 'Details', kind: 'moreinfo' }, colourWheel: { diameter: 120 }, ctStrip: { min: 2200, max: 6500, unit: 'K' } }[type];
    if (['arcGauge', 'needleGauge', 'bar'].includes(type)) o.zones = [{ from: 80, to: 90, tone: 'warn' }, { from: 90, to: 100, tone: 'crit' }];
    return Object.assign(o, extra);
}
function labState(type, name) {
    const s = Object.assign({}, { off: { on: false, value: 30 }, on: { on: true, value: 70 }, pending: { on: true, value: 70, pending: true }, disabled: { on: false, value: 30, disabled: true }, unavailable: { on: false, value: null, status: 'unavailable' }, stale: { on: true, value: 70, stale: true }, readonly: { on: true, value: 70, readonly: true }, error: { on: false, value: 30, status: 'error' }, alert: { on: true, value: 96, tone: 'crit' }, min: { on: false, value: 0 }, max: { on: true, value: 100 }, focus: { on: true, value: 70 } }[name]);
    const v = s.value;
    if (type === 'segmented') s.value = v === null ? null : ['heat', 'cool', 'off'][v < 34 ? 0 : v < 67 ? 1 : 2];
    if (type === 'stepper') s.value = v === null ? null : 5 + v / 100 * 25;
    if (type === 'ctStrip') s.value = v === null ? null : 2200 + v / 100 * 4300;
    if (type === 'colourWheel') Object.assign(s, { hue: v === null ? 0 : v * 3.6, sat: v === null ? 0 : 0.8 });
    if (type === 'sparkline') s.points = v === null ? [] : [18, 19.5, 19, 20.5, 22, 21.5, 23, 22.5].map(p => p + v / 50);
    if (type === 'badge' || type === 'lamp') s.text = LAB_WORDS['homedash.state.' + (name === 'unavailable' ? 'unavailable' : s.on ? 'on' : 'off')];
    if (type === 'frame') s.stateText = LAB_WORDS['homedash.state.' + (s.on ? 'on' : 'off')];
    if (type === 'sheet') s.open = true;
    if (type === 'numeral' || type === 'segReadout') s.decimals = 1;
    return s;
}
function labView(name) {
    return Object.assign({ id: 'lab:' + name, kind: 'switch', name: 'Desk lamp', area: 'Office', unit: '', device_class: '', value: 'off', attributes: {}, available: true, status: 'ok', readonly: false, capabilities: ['toggle'], pending: false, stale: false, error: null, tone: null },
        { off: {}, on: { value: 'on' }, pending: { value: 'on', pending: true }, disabled: { value: 'on', capabilities: [] }, unavailable: { value: null, available: false, status: 'unavailable' }, stale: { value: 'on', stale: true }, readonly: { value: 'on', readonly: true }, error: { status: 'error' }, alert: { kind: 'sensor', name: 'Humidity', value: 96, unit: '%', tone: 'crit' }, min: { kind: 'sensor', name: 'Humidity', value: 0, unit: '%' }, max: { kind: 'sensor', name: 'Humidity', value: 100, unit: '%' }, focus: { value: 'on' } }[name]);
}
```
Assembly rules (IIFE, `'use strict'`):
1. Create `window.hdLab = {ready:false, error:'', errors:[], instances:[]}`. Merge `window.hdLabEarlyErrors` into `errors`, and keep collecting through `error` and `unhandledrejection` listeners.
2. Parse `location.search` with `URLSearchParams` and apply the C4 defaults.
   - Validate family, palette, motion and states against the tables, and dpr as a number.
   - A type must be a key of `PRIM_VARIANTS` or satisfy `E.widgets.get(type)`.
   - On the first problem set `hdLab.error = 'bad param <name>: <value>'` and stop.
   - If `Math.abs(devicePixelRatio - dpr) > 0.01`, set an error.
3. Scheme: palette dark → 'dark'; light → 'light'; otherwise the param, defaulting to 'dark'. Set `document.documentElement.lang`.
4. If palette is theme, configure `#hd-lab-scope`:
   - add class `desktop-body`
   - `data-theme` = theme param (default 'standard'), `data-scheme` = scheme, `data-accent` = 'teal'
   - `data-animations` = 'false' unless motion is full
   - insert `<link rel=stylesheet href="/css/desktop-shell.bundle.css">` before the homedash bundle link, and await its `load` (an `error` event sets `hdLab.error`)
5. `await document.fonts.ready`.
6. Panel root: `E.el('section', {...})` with:
   - class `hd-lab-panel hd-root`
   - `data-hd-host` = 'desktop' for palette theme, else 'kiosk'
   - `data-hd-family`, `data-hd-palette`, `data-hd-scheme`, `data-hd-motion`, `data-hd-mode="view"`, `data-hd-density="comfortable"`
   - For palette custom: `E.colour.applyPalette(root, E.colour.solvePalette({accent:'#8E5BD9', surface_hue:285, scheme}).tokens)`.
   - Append the root to `#hd-lab`.
7. Title: `h1.hd-lab-title` with text `HomeDash control lab: family=… palette=… theme=… scheme=… dpr=… motion=…` (theme only when palette is theme).
8. Rows:
   - For each type, then each variant (`PRIM_VARIANTS[type]`, or the widget def's variants, filtered by the `variants` param), then each size, build `div.hd-lab-row` with `div.hd-lab-rowhead` (text `type / variant / size`) and `div.hd-lab-cells`.
   - Primitive sizes: params matching `/^(m|l)$/`, default `['m']`.
   - Widget sizes: params matching `/^\d+x\d+$/`, default `LAB_WIDGET_SIZES[type] || ['2x1']`.
   - One cell per state: `div.hd-lab-cell[data-hd-lab-control="type/variant/size"][data-hd-lab-state]` > `span.hd-lab-statecap` (state name) + `div.hd-lab-stage`.
9. Primitive cell:
   - `inst = E.prim[type](primOpts(type, variant, size, lang))`, then `inst.set(labState(type, state))`, then append `inst.el` to the stage.
   - For the focus state, set `data-hd-focus="ring"` on the first match of `[role="switch"], [role="slider"], [role="radio"][aria-checked="true"], button, [role="spinbutton"]`, trying `inst.el` itself first.
   - Push `{control, state, el: inst.el, apply: n => inst.set(labState(type, n))}` to `hdLab.instances`.
10. Widget cell:
   - `[w,h] = size.split('x')`.
   - `host = E.widgets.host({widget:{id:'lab-'+(++n), type, variant, title:'', bindings:{primary:'lab:'+state}, options:{}, actions:{}, confirm:false}, size:{w,h}, t: E.core.makeT(LAB_WORDS, {warn(){}}), lang, api: fixtureApi, moreLabel:'More'})`.
   - `fixtureApi` is a factory whose `entity` returns `labView(cur)` (with `let cur = state` captured per cell). `can` returns false for disabled, unavailable and readonly; `command` and `history` resolve; `moreInfo` and `announce` are no-ops.
   - Write the size with CSSOM only: `host.el.style.setProperty('--hd-lab-w', (w*CELL+(w-1)*GAP)+'px')`, and likewise `--hd-lab-h`.
   - Call `host.update()`, then `host.resize({w,h,px:{width,height}})`.
   - Focus state: put `data-hd-focus="ring"` on `.hd-more`.
   - Push `{..., apply: s => { cur = s; host.update(); }}`.
11. After render, wait two rAFs and set `hdLab.ready = true`. Wrap everything in try/catch so an exception sets `hdLab.error`.
12. `hdLab.layout()`: panel rect plus `scrollY`. Rows = the title and each `.hd-lab-row` as `{top, bottom}`. Set rows[0].top to the panel top and the last bottom to the panel bottom.
13. `hdLab.probes()` = `hdLabProbes.run({root: panel, palette, motion, entries: hdLab.instances, states})`.
14. `hdLab.applyState(name)`: call `apply(name)` on each instance; return the count.
15. `hdLab.seek(ms)`: await one rAF, then for each animation in `document.getAnimations()`: `a.pause(); a.currentTime = ms`.
- [ ] Run `npm run test:homedash`. Expected: PASS. Commit `test(homedash): control lab fixture page, placeholder look and gallery`.

## Task 15: Lab probes

**Files:** `ui/testdata/homedash-lab-probes.js` (replaces the stub), plus additions to `scripts/homedash-tests/lab.test.mjs`.

- [ ] Failing test: append to `tests` in lab.test.mjs:
```js
['probe colour maths', () => { const ctx = { window: {}, Math, JSON }; vm.createContext(ctx); vm.runInContext(read('ui/testdata/homedash-lab-probes.js'), ctx);
  const p = ctx.window.hdLabProbes; assert.equal(p.ratio([0, 0, 0, 1], [255, 255, 255, 1]).toFixed(2), '21.00'); assert.deepEqual([...p.over([255, 255, 255, 0.5], [0, 0, 0, 1])], [127.5, 127.5, 127.5, 1]); }]
```
- [ ] Run it. Expected: FAIL.
- [ ] Implement (full code; the canvas is created lazily):
```js
// HomeDash control lab probes (test fixture, never shipped). hdLabProbes.run() measures the rendered panel.
(function () {
    'use strict';
    const TEXT_MIN = 4.5, GRAPHIC_MIN = 3, HIT_MIN = 44, EPS = 0.5, WHITE = [255, 255, 255, 1];
    let ctx = null;
    function rgba(value) {
        if (!ctx) { const c = document.createElement('canvas'); c.width = c.height = 1; ctx = c.getContext('2d', { willReadFrequently: true }); }
        ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = 'rgba(0, 0, 0, 0)'; ctx.fillStyle = value; ctx.fillRect(0, 0, 1, 1);
        const d = ctx.getImageData(0, 0, 1, 1).data; return [d[0], d[1], d[2], d[3] / 255];
    }
    function over(t, b) { const a = t[3] + b[3] * (1 - t[3]); if (!a) return [0, 0, 0, 0]; return [0, 1, 2].map(i => (t[i] * t[3] + b[i] * b[3] * (1 - t[3])) / a).concat([a]); }
    function lum(c) { const f = v => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); }; return 0.2126 * f(c[0]) + 0.7152 * f(c[1]) + 0.0722 * f(c[2]); }
    function ratio(a, b) { const x = lum(a), y = lum(b); return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05); }
    const show = v => (Math.floor(v * 100) / 100).toFixed(2);
    function label(el) { const cell = el.closest('[data-hd-lab-control]'), where = cell ? cell.getAttribute('data-hd-lab-control') + ' ' + cell.getAttribute('data-hd-lab-state') : 'panel';
        return where + ' .' + String(el.getAttribute('class') || el.tagName.toLowerCase()).split(/\s+/)[0]; }
    const exempt = el => !!el.closest('[aria-disabled="true"], [data-hd-status="unavailable"], [data-hd-disabled="true"]');
    function backgroundOf(el) { const layers = [];
        for (let n = el; n && n.nodeType === 1; n = n.parentElement) { const cs = getComputedStyle(n); if (cs.backgroundImage && cs.backgroundImage !== 'none') return null;
            const c = rgba(cs.backgroundColor); if (c[3] > 0) { layers.push(c); if (c[3] >= 1) break; } }
        let bg = WHITE; for (let i = layers.length - 1; i >= 0; i--) bg = over(layers[i], bg); return bg; }
    function opacityOf(el) { let o = 1; for (let n = el; n && n.nodeType === 1; n = n.parentElement) o *= Number(getComputedStyle(n).opacity); return o; }
    const PALETTE = ['--hd-canvas', '--hd-surface-solid', '--hd-sunken', '--hd-ink', '--hd-ink-muted', '--hd-ink-faint', '--hd-accent', '--hd-on', '--hd-on-ink', '--hd-ink-on-accent', '--hd-selected', '--hd-selected-ink', '--hd-btn', '--hd-btn-ink', '--hd-hairline', '--hd-focus', '--hd-ok', '--hd-warn', '--hd-alert', '--hd-info', '--hd-ok-ink', '--hd-warn-ink', '--hd-alert-ink', '--hd-info-ink'];
    const S = '--hd-surface-solid', C = '--hd-canvas';
    const PAIRS = [['--hd-ink', S, 4.5, 't'], ['--hd-ink-muted', S, 4.5, 't'], ['--hd-ink', C, 4.5, 't'], ['--hd-ink-muted', C, 4.5, 't'], ['--hd-on-ink', '--hd-on', 4.5, 't'], ['--hd-ink-on-accent', '--hd-accent', 4.5, 't'],
        ['--hd-selected-ink', '--hd-selected', 4.5, 't'], ['--hd-btn-ink', '--hd-btn', 4.5, 't'], ['--hd-ok-ink', S, 4.5, 't'], ['--hd-warn-ink', S, 4.5, 't'], ['--hd-alert-ink', S, 4.5, 't'], ['--hd-info-ink', S, 4.5, 't'],
        ['--hd-accent', S, 3, 'g'], ['--hd-focus', S, 3, 'g'], ['--hd-ok', S, 3, 'g'], ['--hd-warn', S, 3, 'g'], ['--hd-alert', S, 3, 'g']];
    function tokens(root, palette, r) {
        const probe = document.createElement('span'); probe.className = 'hd-lab-probe'; root.appendChild(probe);
        const read = t => { probe.style.setProperty('color', 'var(' + t + ', rgba(0, 0, 0, 0))'); const c = rgba(getComputedStyle(probe).color); return c[3] > 0 ? c : null; };
        try {
            for (const t of PALETTE) { r.resolution.checked++; if (!read(t)) r.resolution.failures.push('token ' + t + ' does not resolve'); }
            if (palette === 'theme') { r.resolution.checked++; const a = read(S), b = read('--vd-mat-card-color'); if (!a || !b || a.slice(0, 3).some((v, i) => Math.abs(v - b[i]) > 1)) r.resolution.failures.push('bridge: --hd-surface-solid does not follow --vd-mat-card-color'); }
            const canvasBg = over(read(C) || WHITE, WHITE);
            for (const [fg, bg, min, kind] of PAIRS) { const f = read(fg), b = read(bg); if (!f || !b) continue;
                const base = bg === C ? canvasBg : over(b, canvasBg), v = ratio(over(f, base), base); r.contrast.checked++;
                if (v < min) { const msg = 'token ' + fg + ' on ' + bg + ' ' + show(v) + ' < ' + min; if (kind === 'g' && palette === 'theme') r.contrast.warnings.push(msg + ' (theme colours, reported only)'); else r.contrast.failures.push(msg); } }
        } finally { probe.remove(); }
    }
    function text(root, r) { const owners = new Set(), walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
        for (let n = walker.nextNode(); n; n = walker.nextNode()) if (n.textContent.trim() && n.parentElement) owners.add(n.parentElement);
        for (const el of owners) { const cs = getComputedStyle(el), rect = el.getBoundingClientRect();
            if (cs.visibility === 'hidden' || !rect.width || !rect.height || el.closest('[hidden]') || exempt(el)) continue;
            const bg = backgroundOf(el); if (!bg) { r.contrast.unmeasured++; continue; }
            const fg = rgba(cs.color); fg[3] *= opacityOf(el); const min = el.closest('[aria-hidden="true"]') ? GRAPHIC_MIN : TEXT_MIN, v = ratio(over(fg, bg), bg); r.contrast.checked++;
            if (v < min) r.contrast.failures.push('text ' + label(el) + ' ' + JSON.stringify(el.textContent.trim().slice(0, 24)) + ' ' + show(v) + ' < ' + min); } }
    const GRAPHICS = '.hd-dial-arc, .hd-gauge-arc, .hd-gauge-needle-shape, .hd-spark-line, .hd-slider-fill, .hd-bar-fill, .hd-seg7-seg[data-lit="true"], .hd-toggle-thumb, .hd-lamp-dot';
    function graphics(root, r) { for (const el of root.querySelectorAll(GRAPHICS)) { const rect = el.getBoundingClientRect(); if (exempt(el) || !rect.width || !rect.height) continue;
        const cs = getComputedStyle(el), svg = el instanceof SVGElement; let paint = null;
        if (cs.stroke && cs.stroke !== 'none') { const s = rgba(cs.stroke); if (s[3] > 0) paint = s; }
        if (!paint && svg && cs.fill && cs.fill !== 'none') { const f = rgba(cs.fill); if (f[3] > 0) paint = f; }
        if (!paint && !svg) { const b = rgba(cs.backgroundColor); if (b[3] > 0) paint = b; }
        if (!paint) continue; paint[3] *= opacityOf(el);
        const bg = backgroundOf(svg ? el.ownerSVGElement.parentElement : el.parentElement); if (!bg) { r.contrast.unmeasured++; continue; }
        const v = ratio(over(paint, bg), bg); r.contrast.checked++; if (v < GRAPHIC_MIN) r.contrast.failures.push('graphic ' + label(el) + ' ' + show(v) + ' < 3'); } }
    const INTERACTIVE = ['button', '[role="slider"]', '[role="switch"]', '[role="radio"]', '[role="spinbutton"]', '[tabindex="0"]'].map(s => '.hd-lab-stage ' + s).join(', ');
    function hit(root, r) { for (const el of root.querySelectorAll(INTERACTIVE)) { const b = el.getBoundingClientRect(); if (!b.width && !b.height) continue; r.hit.checked++;
        if (b.width < HIT_MIN - EPS || b.height < HIT_MIN - EPS) r.hit.failures.push('hit ' + label(el) + ' ' + show(b.width) + 'x' + show(b.height) + ' < 44x44'); } }
    function shift(root, entries, states, r) { const groups = new Map();
        for (const cell of root.querySelectorAll('[data-hd-lab-control]')) { const t = cell.querySelector('.hd-lab-stage > *'); if (!t) continue; const b = t.getBoundingClientRect(), k = cell.getAttribute('data-hd-lab-control');
            if (!groups.has(k)) groups.set(k, []); groups.get(k).push({ s: cell.getAttribute('data-hd-lab-state'), w: b.width, h: b.height }); }
        for (const [k, list] of groups) { r.shift.checked++; for (const it of list.slice(1)) if (Math.abs(it.w - list[0].w) > EPS || Math.abs(it.h - list[0].h) > EPS) r.shift.failures.push('shift ' + k + ' ' + list[0].s + ' ' + show(list[0].w) + 'x' + show(list[0].h) + ' vs ' + it.s + ' ' + show(it.w) + 'x' + show(it.h)); }
        const seen = new Set(); for (const e of entries) { if (seen.has(e.control)) continue; seen.add(e.control); const b0 = e.el.getBoundingClientRect();
            for (const s of states) { e.apply(s); const b = e.el.getBoundingClientRect(); if (Math.abs(b.width - b0.width) > EPS || Math.abs(b.height - b0.height) > EPS) r.shift.failures.push('live shift ' + e.control + ' at ' + s); }
            e.apply(e.state); r.shift.checked++; } }
    async function animations(motion, r) { if (motion === 'full') return; await new Promise(res => requestAnimationFrame(() => requestAnimationFrame(res))); await new Promise(res => setTimeout(res, 300)); r.animations.checked = true;
        for (const a of document.getAnimations()) if (a.playState === 'running' || a.pending) { const t = a.effect && a.effect.target; r.animations.running.push((t ? label(t) : '?') + ' ' + (a.animationName || a.transitionProperty || 'animation')); }
        for (const x of r.animations.running) r.animations.failures.push('still running with motion=' + motion + ': ' + x); }
    function backdrop(root, r) { let n = 0; for (const el of root.querySelectorAll('*')) { const cs = getComputedStyle(el), v = cs.backdropFilter || cs.webkitBackdropFilter; if (v && v !== 'none') n++; }
        r.backdrop.count = n; r.backdrop.limit = root.querySelectorAll('.hd-frame').length + 1; if (n > r.backdrop.limit) r.backdrop.failures.push('backdrop-filter on ' + n + ' elements, limit ' + r.backdrop.limit); }
    async function fonts(r) { for (const face of ['600 16px "HomeDash Barlow Condensed"', '16px "HomeDash Share Tech Mono"']) { r.resolution.checked++; const list = await document.fonts.load(face); if (!list.length) r.resolution.failures.push('font ' + face + ' did not load'); } }
    async function run(o) {
        const r = { contrast: { checked: 0, failures: [], warnings: [], unmeasured: 0 }, hit: { checked: 0, failures: [] }, shift: { checked: 0, failures: [] }, animations: { checked: false, running: [], failures: [] }, backdrop: { count: 0, limit: 0, failures: [] }, resolution: { checked: 0, failures: [] } };
        tokens(o.root, o.palette, r); text(o.root, r); graphics(o.root, r); hit(o.root, r); shift(o.root, o.entries || [], o.states || [], r); backdrop(o.root, r); await fonts(r); await animations(o.motion, r);
        return r;
    }
    window.hdLabProbes = Object.freeze({ run, ratio, over, rgba });
})();
```
- [ ] Run `npm run test:homedash`. Expected: PASS. Commit `test(homedash): control lab probes`.

## Task 16: Go control-lab test

**Files:** `ui/desktop_homedash_lab_plan_test.go`, `ui/desktop_homedash_control_lab_browser_test.go`. **Read first:**
- `ui/desktop_theme_sweep_browser_test.go:224-319` (`themeSweepConcat`, parts served per request), `:367-370` (reduced-motion emulation)
- `ui/desktop_theme_registry_test.go:32-50, 61, 202` (`parseDesktopThemeRegistry`, `desktopBundleParts`)
- `ui/precision_workspace_browser_test.go:13` (`requirePrecisionBrowserSmoke`)
- `ui/destructive_flow_browser_test.go:57` (`newSmokeBrowserWith`)
- `internal/audit/ui_browser_ci_test.go:15-104`: `TestDesktopHomeDash…` falls only into "Desktop H-R"; `TestHomeDashLab…` falls only into "General A-M".

- [ ] Failing unit test (`ui/desktop_homedash_lab_plan_test.go`, test part):
```go
func TestHomeDashLabSheetPlan(t *testing.T) {
	themes := parseDesktopThemeRegistry(t)
	if got := strings.Join(homedashLabSheetIDs(""), ","); got != homedashLabDefaultSheets { t.Fatalf("default sheets %q", got) }
	if got := strings.Join(homedashLabSheetIDs(" prim-glass, ,prim-glass,hero-glass "), ","); got != "prim-glass,hero-glass" { t.Fatalf("parsed %q", got) }
	for id, want := range map[string]int{"prim-placeholder": 4, "hero-glass": 14, "type-entity-placeholder": 3, "themes-minimal": 16, "risk-tactile": 6, "motion-toggle-glass": 1, "side-by-side-knob": 4} {
		sheet, err := homedashLabPlan(id, themes)
		if err != nil || len(sheet.Panels) != want { t.Errorf("%s: %d panels, err %v; want %d", id, len(sheet.Panels), err, want) }
	}
	m, _ := homedashLabPlan("motion-toggle-glass", themes)
	if len(m.Panels[0].Frames) != 6 || m.Panels[0].Query.Get("states") != "off" { t.Errorf("motion panel %+v", m.Panels[0]) }
	p, _ := homedashLabPlan("prim-placeholder", themes)
	if p.Panels[2].Query.Get("palette") != "theme" || p.Panels[2].Query.Get("theme") != "standard" || p.Panels[3].Motion != "reduced" { t.Errorf("prim panels %+v", p.Panels) }
	th, _ := homedashLabPlan("themes-minimal", themes)
	seen := map[string]bool{}
	for _, panel := range th.Panels { seen[panel.Query.Get("theme")+"/"+panel.Query.Get("scheme")] = true }
	for _, want := range []string{"fruity/light", "fruity/dark", "minimal/light", "minimal/dark", "amiga/light"} { if !seen[want] { t.Errorf("themes sheet misses %s", want) } }
	for _, bad := range []string{"prim-neon", "motion-nope-glass", "side-by-side-", "hello", "type--glass"} { if _, err := homedashLabPlan(bad, themes); err == nil { t.Errorf("%s: want an error", bad) } }
}

func TestHomeDashLabPack(t *testing.T) {
	var images []image.Image
	for i := 0; i < 5; i++ { images = append(images, image.NewRGBA(image.Rect(0, 0, 1180, 1000))) }
	parts := homedashLabPack(images, 2400)
	if len(parts) != 2 || parts[0].Bounds().Dx() != 2360 || parts[0].Bounds().Dy() != 2000 || parts[1].Bounds().Dx() != 1180 || parts[1].Bounds().Dy() != 1000 { t.Fatalf("parts %v", parts) }
	if got := homedashLabFileNames("x", 1); len(got) != 1 || got[0] != "x.png" { t.Fatal(got) }
	if got := homedashLabFileNames("x", 3); strings.Join(got, ",") != "x-01.png,x-02.png,x-03.png" { t.Fatal(got) }
	g := homedashLabGroups([]homedashLabRow{{0, 100}, {100, 1300}, {1300, 2500}}, 2400)
	if len(g) != 2 || g[0] != [2]float64{0, 1300} || g[1] != [2]float64{1300, 2500} { t.Fatal(g) }
}
```
- [ ] Run `go test ./ui -run '^TestHomeDashLab(SheetPlan|Pack)$' -count=1`. Expected: build failure (undefined).
- [ ] Implement the plan file (same file, above the tests). Imports: `fmt image image/draw net/url regexp strconv strings testing`.
```go
// HomeDash control lab: sheet planning and image packing (pure Go). The browser runner is in
// desktop_homedash_control_lab_browser_test.go.
const (
	homedashLabDefaultSheets  = "prim-placeholder,type-entity-placeholder,themes-placeholder"
	homedashLabMaxSide        = 2400
	homedashLabWidth          = 1180
	homedashLabFamilyPattern  = `(glass|tactile|minimal|instrument|placeholder)`
	homedashLabAllStates      = "off,on,pending,disabled,unavailable,stale,readonly,error,alert,min,max,focus"
	homedashLabHeroStates     = "off,on,pending,unavailable,alert,focus"
)
var (
	homedashLabHeroes       = []string{"toggle", "rocker", "knob", "needleGauge", "arcGauge", "numeral", "segReadout", "sparkline"}
	homedashLabRiskThemes   = []string{"fruity", "lollipop", "amiga", "crazy", "cyberpunk", "green_slime"}
	homedashLabMotionFrames = []int{0, 50, 100, 200, 400, 800}
	homedashLabPrimID       = regexp.MustCompile(`^prim-` + homedashLabFamilyPattern + `$`)
	homedashLabHeroID       = regexp.MustCompile(`^hero-` + homedashLabFamilyPattern + `$`)
	homedashLabTypeID       = regexp.MustCompile(`^type-([a-z][a-z0-9_]*)-` + homedashLabFamilyPattern + `$`)
	homedashLabThemesID     = regexp.MustCompile(`^themes-` + homedashLabFamilyPattern + `$`)
	homedashLabRiskID       = regexp.MustCompile(`^risk-` + homedashLabFamilyPattern + `$`)
	homedashLabMotionID     = regexp.MustCompile(`^motion-([A-Za-z]+)-` + homedashLabFamilyPattern + `$`)
	homedashLabSideID       = regexp.MustCompile(`^side-by-side-([A-Za-z]+)$`)
)
type homedashLabPanel struct { Label string; Query url.Values; DPR float64; Motion string; Frames []int }
type homedashLabSheet struct { ID string; Panels []homedashLabPanel }
type homedashLabQuery struct { Family, Palette, Scheme, Theme, Types, States, Sizes, Motion string; DPR float64 }
type homedashLabRow struct { Top float64 `json:"top"`; Bottom float64 `json:"bottom"` }

func homedashLabSheetIDs(env string) []string {
	if strings.TrimSpace(env) == "" { env = homedashLabDefaultSheets }
	var out []string; seen := map[string]bool{}
	for _, id := range strings.Split(env, ",") { id = strings.TrimSpace(id); if id != "" && !seen[id] { seen[id] = true; out = append(out, id) } }
	return out
}
func homedashLabPanelOf(q homedashLabQuery) homedashLabPanel {
	if q.Motion == "" { q.Motion = "full" }
	if q.DPR == 0 { q.DPR = 1 }
	v := url.Values{}
	for k, val := range map[string]string{"family": q.Family, "palette": q.Palette, "scheme": q.Scheme, "theme": q.Theme, "types": q.Types, "states": q.States, "sizes": q.Sizes} { if val != "" { v.Set(k, val) } }
	v.Set("motion", q.Motion); v.Set("dpr", strconv.FormatFloat(q.DPR, 'f', -1, 64)); v.Set("lang", "en")
	label := "family=" + q.Family + " palette=" + q.Palette
	if q.Theme != "" { label += " theme=" + q.Theme }
	return homedashLabPanel{Label: label + fmt.Sprintf(" scheme=%s dpr=%g motion=%s", q.Scheme, q.DPR, q.Motion), Query: v, DPR: q.DPR, Motion: q.Motion}
}
func homedashLabScheme(themes []desktopThemeDescriptor, id string) string {
	for _, th := range themes { if th.ID == id { return th.Scheme } }
	return "dark"
}
func homedashLabIsHero(name string) bool { for _, h := range homedashLabHeroes { if h == name { return true } }; return false }
func homedashLabPlan(id string, themes []desktopThemeDescriptor) (homedashLabSheet, error) {
	sheet := homedashLabSheet{ID: id}
	add := func(q homedashLabQuery) { sheet.Panels = append(sheet.Panels, homedashLabPanelOf(q)) }
	heroTypes := strings.Join(homedashLabHeroes, ",")
	themed := func(q homedashLabQuery, theme string) homedashLabQuery { q.Palette, q.Theme, q.Scheme = "theme", theme, homedashLabScheme(themes, theme); return q }
	switch {
	case homedashLabPrimID.MatchString(id):
		f := homedashLabPrimID.FindStringSubmatch(id)[1]
		add(homedashLabQuery{Family: f, Palette: "dark", Scheme: "dark"}); add(homedashLabQuery{Family: f, Palette: "light", Scheme: "light"})
		add(themed(homedashLabQuery{Family: f}, "standard")); add(homedashLabQuery{Family: f, Palette: "dark", Scheme: "dark", Motion: "reduced"})
	case homedashLabHeroID.MatchString(id):
		f := homedashLabHeroID.FindStringSubmatch(id)[1]
		for _, dpr := range []float64{1, 2} {
			base := homedashLabQuery{Family: f, Types: heroTypes, States: homedashLabHeroStates, Sizes: "m,l", DPR: dpr}
			d, l := base, base; d.Palette, d.Scheme = "dark", "dark"; l.Palette, l.Scheme = "light", "light"; add(d); add(l)
			for _, th := range []string{"standard", "fruity", "organic", "crazy", "amiga"} { add(themed(base, th)) }
		}
	case homedashLabTypeID.MatchString(id):
		m := homedashLabTypeID.FindStringSubmatch(id)
		add(homedashLabQuery{Family: m[2], Palette: "dark", Scheme: "dark", Types: m[1]}); add(homedashLabQuery{Family: m[2], Palette: "light", Scheme: "light", Types: m[1]})
		add(themed(homedashLabQuery{Family: m[2], Types: m[1]}, "standard"))
	case homedashLabThemesID.MatchString(id):
		f := homedashLabThemesID.FindStringSubmatch(id)[1]
		for _, th := range themes {
			schemes := []string{th.Scheme}
			if th.SchemeSetting != "" { schemes = []string{"light", "dark"} }
			for _, s := range schemes { add(homedashLabQuery{Family: f, Palette: "theme", Theme: th.ID, Scheme: s, Types: "toggle,slider,arcGauge,numeral", States: "off,on,alert"}) }
		}
	case homedashLabRiskID.MatchString(id):
		f := homedashLabRiskID.FindStringSubmatch(id)[1]
		for _, th := range homedashLabRiskThemes { add(themed(homedashLabQuery{Family: f, Types: heroTypes, States: homedashLabHeroStates}, th)) }
	case homedashLabMotionID.MatchString(id):
		m := homedashLabMotionID.FindStringSubmatch(id)
		if !homedashLabIsHero(m[1]) { return sheet, fmt.Errorf("motion sheet %q: unknown hero %q (heroes: %s)", id, m[1], heroTypes) }
		p := homedashLabPanelOf(homedashLabQuery{Family: m[2], Palette: "dark", Scheme: "dark", Types: m[1], States: "off"}); p.Frames = homedashLabMotionFrames
		sheet.Panels = append(sheet.Panels, p)
	case homedashLabSideID.MatchString(id):
		hero := homedashLabSideID.FindStringSubmatch(id)[1]
		if !homedashLabIsHero(hero) { return sheet, fmt.Errorf("side-by-side sheet %q: unknown hero %q", id, hero) }
		for _, f := range []string{"glass", "tactile", "minimal", "instrument"} { add(homedashLabQuery{Family: f, Palette: "dark", Scheme: "dark", Types: hero, States: homedashLabHeroStates}) }
	default:
		return sheet, fmt.Errorf("unknown HomeDash lab sheet id %q (want prim-<f>, hero-<f>, type-<type>-<f>, themes-<f>, risk-<f>, motion-<hero>-<f> or side-by-side-<hero>)", id)
	}
	return sheet, nil
}
func homedashLabGroups(rows []homedashLabRow, limit float64) [][2]float64 {
	var groups [][2]float64
	for _, r := range rows { if n := len(groups); n > 0 && r.Bottom-groups[n-1][0] <= limit { groups[n-1][1] = r.Bottom; continue }; groups = append(groups, [2]float64{r.Top, r.Bottom}) }
	return groups
}
func homedashLabPack(images []image.Image, maxSide int) []*image.RGBA {
	type placed struct { img image.Image; x, y int }
	var parts []*image.RGBA; var cur []placed
	x, y, rowH, w, h := 0, 0, 0, 0, 0
	flush := func() {
		if len(cur) == 0 { return }
		out := image.NewRGBA(image.Rect(0, 0, w, h)); draw.Draw(out, out.Bounds(), image.White, image.Point{}, draw.Src)
		for _, p := range cur { b := p.img.Bounds(); draw.Draw(out, image.Rect(p.x, p.y, p.x+b.Dx(), p.y+b.Dy()), p.img, b.Min, draw.Src) }
		parts = append(parts, out); cur, x, y, rowH, w, h = nil, 0, 0, 0, 0, 0
	}
	for _, img := range images {
		iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
		if x > 0 && x+iw > maxSide { y += rowH; x, rowH = 0, 0 }
		if y > 0 && y+ih > maxSide { flush() }
		cur = append(cur, placed{img, x, y}); x += iw
		if ih > rowH { rowH = ih }; if x > w { w = x }; if y+rowH > h { h = y + rowH }
	}
	flush(); return parts
}
func homedashLabFileNames(id string, n int) []string {
	if n == 1 { return []string{id + ".png"} }
	out := make([]string, n); for i := range out { out[i] = fmt.Sprintf("%s-%02d.png", id, i+1) }; return out
}
```
Run gofmt. If `testing` is unused in the plan part, it is still used by the tests.
- [ ] Run `go test ./ui -run '^TestHomeDashLab(SheetPlan|Pack)$' -count=1`. Expected: PASS.
- [ ] Implement `ui/desktop_homedash_control_lab_browser_test.go`. Imports: `bytes encoding/json fmt image image/png math net/http net/http/httptest os path/filepath strings testing time`, go-rod `rod` and `lib/proto`. Contract:
  1. Report types:
     - `homedashLabCount{Checked int; Failures []string}`.
     - `homedashLabProbes` mirrors C6 JSON: `contrast{checked,failures,warnings,unmeasured}`, `hit`, `shift`, `animations{checked bool,running,failures}`, `backdrop{count,limit,failures}`, `resolution`.
     - `homedashLabPanelReport{Label, URL; Probes *homedashLabProbes; Errors, Oversized []string}` with method `failures()`. It prefixes entries with `error:`, `size:`, `contrast:`, `hit:`, `shift:`, `animations:`, `backdrop:` and `resolution:`.
     - `homedashLabSheetReport{ID, Images, Panels}`; `homedashLabReport{Generated, ReportOnly, Sheets, Failures}`. All snake_case JSON tags as in C5.
  2. `newHomeDashLabServer(t) string`:
     - mux `/` → `http.FileServer(http.FS(Content))`
     - `/css/homedash.bundle.css` and `/css/desktop-shell.bundle.css` → `themeSweepConcat(desktopBundleParts(t, "ui/css/<name>"), false)` per request, with `Content-Type: text/css` and `Cache-Control: no-store`
     - start `httptest.NewServer`, register `t.Cleanup(server.Close)`, return the URL
  3. `homedashLabViewport(t, page, height int, dpr)` calls `page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: homedashLabWidth, Height: height, DeviceScaleFactor: dpr})`.
  4. `homedashLabShot(t, page, top, height) image.Image` calls `page.Screenshot(false, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng, Clip: &proto.PageViewport{X: 0, Y: top, Width: homedashLabWidth, Height: height, Scale: 1}, CaptureBeyondViewport: true})`, then `png.Decode(bytes.NewReader(...))`.
  5. `homedashLabRunPanel(t, page, base, panel) ([]image.Image, homedashLabPanelReport)`:
     - Emulate media `prefers-reduced-motion`: `no-preference` when motion is full, else `reduce`, via `proto.EmulationSetEmulatedMedia`.
     - `open()`: set the viewport to height 900; `page.Timeout(60s).Navigate(base+"/testdata/homedash-lab.html?"+query)`; poll `()=>!!window.hdLab&&(hdLab.ready===true||!!hdLab.error)` every 50 ms for up to 30 s. A non-empty `hdLab.error`, or the timeout, goes into `report.Errors` and the panel is skipped.
     - `capture()`:
       - Read `hdLab.layout()` with `.Value.Unmarshal`.
       - If height > 7000, add an Oversized entry and cap at 7000.
       - Set the viewport height to `ceil(height)`. Await fonts and two rAFs.
       - Groups = `homedashLabGroups(rows, floor(2400/dpr))`. Shoot each group; an image taller than 2400 adds an Oversized entry.
     - Normal panel: `open`, `capture`, then probes = `JSON.parse` of `async()=>JSON.stringify(await hdLab.probes())`.
     - Motion panel: for each frame `ms`: `open`, eval `async(ms)=>{hdLab.applyState('on');await hdLab.seek(ms);}` with `ms`, then `capture`.
     - Always append `JSON.stringify(hdLab.errors||[])` entries to `report.Errors`.
     - Use `page.Timeout(60*time.Second).MustEval` for every eval.
  6. `homedashLabWriteSheet(t, dir, id, images) []string`:
     - Remove the old `id.png` and `id-[0-9][0-9].png` files.
     - Pack with `homedashLabPack(images, homedashLabMaxSide)`.
     - Name with `homedashLabFileNames`, write each with `png.Encode`, return the names.
  7. `TestDesktopHomeDashControlLabBrowser`:
     - Call `requirePrecisionBrowserSmoke`. Plan every id from `HOMEDASH_LAB_SHEETS`; `t.Fatal` on a plan error. `reportOnly := os.Getenv("HOMEDASH_LAB_REPORT_ONLY") == "1"`.
     - Start the server; `page := newSmokeBrowserWith(t, nil).MustPage()` with cleanup. `outDir := filepath.Join("..", "reports", "homedash-lab")`, then `MkdirAll`.
     - Loop over sheets and panels. Collect failures as `"<sheet> [<label>]: <failure>"`; write each sheet; write `probes.json` with `json.MarshalIndent`.
     - Log one line per sheet with its images and failure count.
     - If there are failures: `t.Logf` them when report-only, else `t.Fatalf("%d HomeDash lab probe failures (HOMEDASH_LAB_REPORT_ONLY=1 reports without failing):\n%s", ...)`.
     - Doc comment with the run command (below).
- [ ] `go vet ./ui` (compile check).
- [ ] Smoke run. PowerShell, sandbox off, backslash GOTMPDIR. Build once, then run the binary from `<wt>\ui`:
  - `go test -c -o D:\tmp\go-build\hdlab-ui.test.exe ./ui`
  - `$env:AURAGO_RUN_BROWSER_SMOKE='1'; D:\tmp\go-build\hdlab-ui.test.exe -test.run '^TestDesktopHomeDashControlLabBrowser$' -test.v -test.timeout 30m`
  - Expected: PASS. `reports\homedash-lab\` holds the sheet PNGs and `probes.json`.
  - Open one PNG with the Read tool to confirm it is legible.
  - If the themes sheet reports a real theme text failure, do NOT weaken the probe. Report it to the lead with the token values; the lead decides between a bridge fix and REPORT_ONLY.
- [ ] Also run `go test ./internal/audit -run '^TestUIBrowserCIShardsCoverEveryTest$' -count=1`. Expected: PASS.
- [ ] Commit both Go files: `test(homedash): control lab browser test with sheets and probes`.

## Task 17: DOX

**Files:** `ui/js/homedash/AGENTS.md` (new, `git add -f`), `ui/AGENTS.md` (Child DOX Index after `:1609`). docs-agent writes them.

- [ ] `ui/js/homedash/AGENTS.md` sections, in order: Purpose, Ownership, Local Contracts, Work Guidance, Verification, Child DOX Index (empty). Content:
  - engine scope and load order (C1)
  - the injected-api widget host (C3)
  - CSSOM-only styling; no innerHTML
  - motion gate string; no `light-dark()`
  - token-block rule
  - bundle parts and `npm run build:ui`
  - lab location (`ui/testdata/homedash-lab*`, never shipped), URL, sheets, `HOMEDASH_LAB_SHEETS`, `HOMEDASH_LAB_REPORT_ONLY`
  - no `t('literal')` until `ui/lang/homedash` exists
  - Verification: `npm run test:homedash`, `npm run check:ui`, the line budget test, the lab command
- [ ] Add one line to the `ui/AGENTS.md` index: `` - `js/homedash/AGENTS.md` — HomeDash engine (pure core, primitives, widget host), its stylesheet bundle and the test-only control lab. ``
- [ ] Run `go test ./internal/audit/... -count=1`. Expected: PASS. Then `git ls-files ui/js/homedash/AGENTS.md` must list the file.
- [ ] Commit `docs(homedash): engine guide and DOX index`.

## Batches and reviews

| Batch | Tasks | Parallel with | Review focus (reader-agent, once per batch; Critical and Important only) |
|---|---|---|---|
| A | 1–7 | B can start after Task 1 | grid and doc algorithms against frontend part 4 and 5.7; solver steps; no DOM at load |
| B | 8–9 | A | token values against visual part 1.3–1.4; guard regexes; bundle check |
| C | 10–13 | – (needs A) | markup against C2/C3; no `style` attributes; numeral reuse; host lifecycle |
| D | 14–16 | – (needs B and C) | lab is not shipped; probe correctness; CI shard; report paths |
| E | 17 | – | DOX accuracy |

test-agent runs each task's named command, plus once at the end: `npm run test:homedash`, `npm run check:ui`, `go test ./ui -run '^(TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestHomeDashLab.*|TestDesktopAppMaterialsAdoptedByApps)$' -count=1`, and one lab smoke. Landing the branch on main is the lead's step: rebase, `npm run check:ui`, `merge --ff-only`.

## Self-review

**Scope coverage**

| Requirement | Task |
|---|---|
| F0: hd-core, format, colour, grid, layout, doc, history plus Node tests | 1–6 |
| V0: tokens, bridge, base; empty starters; bundle entry; @font-face; Node guard plus npm script; line budget for ui/js/homedash | 8, 9, 1, 7 |
| V1: lab page and script (not shipped), Go test `TestDesktopHomeDashControlLabBrowser`, URL, sheets, probes, outputs | 14–16 |
| P0: hd-geometry with tests, all 20 primitive builders (static), starter hd-icons, widget registry plus host with injected api, one `entity`/tile widget | 10–13 |
| Nothing user-visible: no module-loader, catalog or locale edits | all |

**Risks**
- Chrome cannot start from repo-built binaries on this machine. Run tests from `D:\tmp\go-build` with the sandbox off.
- Leftover rod Chromes can slow runs.
- `./ui` compile time grows when sessions run in parallel. Use a prebuilt test binary; CSS and JS are read from disk per request.
- Theme contrast findings (see C3 below) may block the default smoke set.
- `stroke-dasharray: calc()` in the placeholder CSS needs Chromium 111+.
- Canvas parsing of computed `color(srgb …)`/`oklch` strings needs Chromium 111+.
- Very tall panels are capped at 7000 CSS px.
- GitNexus is likely unavailable in the worktree; use the grep fallback.
- `test-ui-regressions.mjs` is shared with other sessions (one appended block).

**Contradictions and decisions (with evidence)**
1. Frontend part 1.6 loads `hd-prim-controls` before `hd-prim-displays`. This plan loads displays first, because the shared `E.prim.util` and the numeral (used by knob and stepper) live there. The main document 4.4 names the modules but no order.
2. Frontend 9.2 has `line.hd-gauge-needle`. Visual part §3 says "needle = transformed `<g>`". The plan uses `g.hd-gauge-needle` > `path.hd-gauge-needle-shape`, a tapered polygon (H5).
3. Grounded in data: Amiga's accent `#6688bb` (`desktop-theme-amiga.css:23`) on card `#BBBBBB` (`desktop-materials-amiga.css:26`) is about 1.9:1. Theme-palette graphic token pairs are therefore warnings, not failures, and the placeholder look paints graphics in ink. M1/V7 must decide how families use the accent.
4. Lead decision vs. lab needs: the starter `homedash-primitives.css` is empty, yet the lab needs a "neutral placeholder look". That look lives only in `ui/testdata/homedash-lab.css` under `[data-hd-family="placeholder"]`.
5. Visual 1.5 says "canvas ∓0.04 L, sunken ∓0.07". In the light scheme a lighter canvas would exceed L 1, so both schemes go darker, which matches the neutral table in 1.4.
6. Badge keeps `data-tone` (frontend 9.2); every other primitive uses `data-hd-tone`.
7. Frontend 2.2/2.3 left the view-state `tone` and the `update(view)` shape undefined. Both are defined in C3.
8. Visual 9.4 rule 2 ("no theme id") cannot be enforced, because `minimal` is both a theme id and a family name. Only `[data-theme` is checked.
9. The lab URL contract has no `accent` or custom-palette inputs. User-accent themes render with `data-accent=teal`, and custom uses one fixed input. The accent-preset sweep (visual 12.9) needs a new parameter later.
10. CI ran no HomeDash-style Node script directly; `test:docker-manager` is not in CI either. Task 9 adds one hook to `test-ui-regressions.mjs`. The lead may drop it.

**Files the lead will save/hand out:** this plan → `C:\Users\Andi\Documents\repo\AuraGo\docs\superpowers\plans\` (force-add).
