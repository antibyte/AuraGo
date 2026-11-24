# HomeDash M0 Frontend and Visual Foundations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the shell-independent HomeDash engine foundations. That means the pure core (F0); the token, palette, bridge and backdrop stylesheet bundle with its Node guard (V0); static builders for all 20 primitives plus the widget registry and frame host (P0); and the test-only control lab (V1). The lab has a fixture registry, data-defined sheets and a Go browser test. When the plan is done, `npm run test:homedash` and `npm run check:ui` pass, `TestDesktopHomeDashControlLabBrowser` writes screenshot sheets and `probes.json`, and nothing changes for users: no app is registered.

**Architecture:**
- Every engine file is a classic IIFE under `ui/js/homedash/` that extends `window.HomeDashEngine` (called `E` inside the files).
- Load order: `hd-core`, `hd-format`, `hd-colour`, `hd-grid`, `hd-layout`, `hd-doc`, `hd-history`, `hd-icons`, `hd-geometry`, `hd-prim-displays`, `hd-prim-controls`, `hd-widgets`, `widgets/hd-w-misc`.
- The CSS parts are concatenated into `ui/css/homedash.bundle.css`.
- The lab lives under `ui/testdata/`, which `cmd/assetpack` never ships: `assets/web-assets.json:5` lists only its own directories, and `cmd/assetpack/main.go:69` skips all others.
- The Go test serves the lab from `ui.Content`, which is `os.DirFS(ui)` (`ui/embed.go:13-18`).
- Sheet definitions are data in `ui/testdata/homedash-lab-sheets.json`, resolved by Go.

**Tech Stack:** Vanilla JS (ES2020, Chromium 111+), CSS (no `light-dark()`), Node 22 `node:vm` tests, Go 1.27 with go-rod.

**I wrote the code in this plan without running it.** Implementers may fix errors. They must not change anything in "## Contracts" without reporting it to the lead. Milestone M1 (`2026-11-23-homedash-m1-style-lab.md`) builds on these contracts and extends primitives only through the table in its own Task 1.

**Verbatim sections (output-size limit):** The review did not touch Tasks 0–7, 10 and 17. Their text stays exactly as in the base file `docs/superpowers/plans/2026-11-23-homedash-m0-frontend-base.md` (called "the saved file" below), at these lines of the version that still carries the "Lead amendments" block:

| Task | Saved lines |
|---|---|
| Task 0 | 182–192 |
| Task 1 | 193–453 |
| Task 2 | 454–545 |
| Task 3 | 546–681 |
| Task 4 | 682–782 |
| Task 5 | 783–948 |
| Task 6 | 949–1052 |
| Task 7 | 1053–1063 |
| Task 10 | 1320–1426 |
| Task 17 | 2182–2199, with one added bullet below |

These blocks are not copied into this file: implementers read them in the base file at the lines given (the base file is kept unchanged for that purpose). Wherever a task below says "saved" (the saved code, block, test, rules or file), it means the same task in the base file. This file governs wherever the two differ. Everything else in this answer replaces the saved text, and the "Lead amendments" block is removed because its content is folded into the body.

**Global rules for every task:**
- Work in the worktree from Task 0.
- Before editing an EXISTING symbol, run GitNexus `impact`. Before each commit, run `detect_changes`. If the index is unavailable, confirm callers with grep and write "GitNexus unavailable; callers confirmed with grep" in the commit body.
- Commit after every task, with only the task's paths.
- Files under `docs/` and new `AGENTS.md` files are git-ignored and need `git add -f`.
- LF line endings, no BOM.
- No external assets. No `innerHTML`, `outerHTML` or `insertAdjacentHTML`. No `setAttribute('style'`. Styles go through the CSSOM: `style.setProperty`.
- JS modules stay under 1100 lines (aim for under 700). CSS parts stay at or under 900 lines.
- Untrusted text goes through `textContent`.
- Never type `\u` escapes; the Write and Edit tools decode them. In JS use `String.fromCodePoint`.
- Never edit files through Bash heredocs.
- Until `ui/lang/homedash` exists (M1 Task 1 creates it), never call `t('literal.key')` in engine files (`ui/i18n_lint_test.go:450`). Build state words as `t(STATE_PREFIX + word)`.
- The guard runs as `npm run test:homedash` only. Nothing is added to `scripts/test-ui-regressions.mjs`.

## File structure

| Path | New/Edit | Task |
|---|---|---|
| `scripts/test-homedash-ui.mjs`, `scripts/homedash-tests/{harness,dom,css-lint}.mjs`, `scripts/homedash-tests/*.test.mjs` | new | 1–15 |
| `ui/js/homedash/hd-*.js` (12 files), `ui/js/homedash/widgets/hd-w-misc.js` | new | 1–13 |
| `ui/css/homedash-{tokens,bridge,base}.css` with content; the six other parts as comment-only starters; generated `ui/css/homedash.bundle.css` | new | 8 |
| `scripts/build-ui-bundles.js` (cssBundles entry), `package.json` (`test:homedash`) | edit | 8, 1 |
| `ui/desktop_js_line_budget_test.go` | edit | 7 |
| `ui/testdata/homedash-lab.html`, `homedash-lab.css`, `homedash-lab.js`, `homedash-lab-probes.js`, `homedash-lab-sheets.json` | new | 14–16 |
| `ui/desktop_homedash_lab_plan_test.go`, `ui/desktop_homedash_control_lab_browser_test.go` | new | 16 |
| `ui/js/homedash/AGENTS.md` (force-add), `ui/AGENTS.md` index | new/edit | 17 |

## Contracts

### C1 Engine parts
- Parts: `E.core`, `E.el`, `E.svg`, `E.format`, `E.colour`, `E.grid`, `E.layout`, `E.doc`, `E.history`, `E.icons`, `E.geometry`, `E.prim` (20 builders plus `E.prim.util`) and `E.widgets`.
- No file touches the DOM at load.
- Every file except `hd-core` throws if a part it needs is missing.

### C2 Primitive markup (authoritative; M1 only adds what its Task 1 table lists)

**Common to every builder**
- `E.prim.<name>(opts)` returns `{el, set(state), destroy()}`. Some builders return extra fields, listed in their row.
- Common opts: `label` (accessible name, already translated by the caller), `size` (`m` or `l`, written as `data-hd-size`), `variant` (written as `data-hd-variant` where listed) and `lang` (default `en`).
- Common `set()` keys, written by `util.applyState` on `el`:
  - `status` (`ok`, `unavailable`, `unknown` or `error`) → `data-hd-status`
  - `pending`, `stale`, `readonly`, `disabled` → the matching `data-hd-*` attribute as `"true"`, or absent
  - `tone` (`ok`, `warn` or `crit`) → `data-hd-tone`
- On the role element of interactive builders:
  - disabled or unavailable → `aria-disabled="true"`
  - read-only → `aria-readonly="true"`. Plain buttons have no `aria-readonly`; on them read-only becomes `aria-disabled`.
- `destroy()` removes `el` and is idempotent.
- `--hd-value` is a 0–1 fraction with 4 decimals.
- Angles are degrees from 12 o'clock, clockwise, written as `--hd-angle: <n>deg`.

**Dials and gauges**
- They use `viewBox="0 0 200 200"` with the centre at (100,100). Gauges take a `crop` option, which shortens the viewBox height.
- Value arcs are ONE path, `path[pathLength=1000]`. `set()` writes `stroke-dasharray: <lit> 1000` through `style.setProperty`, where lit = frac·1000. With `quantize` N > 0, lit = round(frac·N)/N·1000. Values have 1 decimal.
- The handle, the arc gauge's cap dot and the needle are groups rotated by CSS (`homedash-base.css`): `transform-box: view-box; transform-origin: 100px 100px; transform: rotate(var(--hd-angle))`. Their circles sit at fixed coordinates; JS never moves them.
- The knob alone may have a buffered pair `path.hd-dial-active[data-buf=a|b]` (option `activeSegment`).
- Test hook: the lab alone sets `data-hd-focus="ring"`.

| Builder | opts (beyond the common ones) | Markup | set() keys | JS writes |
|---|---|---|---|---|
| toggle | – | `button.hd-toggle[type=button][role=switch][aria-label]` > `span.hd-toggle-track[aria-hidden=true]` > `span.hd-toggle-glyph[data-glyph=on]`, `span.hd-toggle-glyph[data-glyph=off]`, `span.hd-toggle-thumb` | on | `aria-checked`, `data-hd-on` |
| rocker | – | `button.hd-rocker[type=button][role=switch][aria-label]` > `span.hd-rocker-bezel[aria-hidden=true]` > `span.hd-rocker-paddle` > (`span.hd-rocker-half[data-half=on]` > `span.hd-rocker-legend` "I"; `span.hd-rocker-half[data-half=off]` > `span.hd-rocker-legend` "O"), then `span.hd-rocker-led` | on | `aria-checked`, `data-hd-on` |
| pushButton | icon, dir (−1\|1), variant chip\|tile | `button.hd-push[type=button][data-hd-variant][data-dir?]` > `svg.hd-ico`? + `span.hd-push-label` | on, text | `data-hd-on`, label text |
| slider | variant h\|v, min, max, step, unit, decimals | `div.hd-slider[role=slider][tabindex=0][data-hd-variant][aria-orientation][aria-label]` > `span.hd-slider-track[aria-hidden=true]` > `span.hd-slider-fill`, `span.hd-slider-thumb` | value, min, max, valueText | `--hd-value`, `aria-valuemin/max/now/text`, `data-hd-empty` |
| knob | variant ring\|knob; start (−135); sweep (270); min; max; step (stored for P1); unit; decimals; quantize (0); ticks (10); legends (false); readout numeral\|seg7\|none; activeSegment (false) | `div.hd-dial[role=slider][tabindex=0][data-hd-variant][data-hd-quantize?][aria-label]` > `svg.hd-dial-svg[viewBox="0 0 200 200"][aria-hidden=true]` > `path.hd-dial-ticks` (`ticks(100,100,80,88,start,sweep,ticks)`), `path.hd-dial-track[pathLength=1000]`, [activeSegment: `path.hd-dial-active[data-buf=a]`, `path.hd-dial-active[data-buf=b]`], `path.hd-dial-arc[pathLength=1000]` (track and arc = `arc(100,100,88,start,sweep)`), `g.hd-dial-handle` > `circle.hd-dial-knob[cx=100][cy=12][r=11]`, [activeSegment: `g.hd-dial-current` > `path.hd-dial-marker[d="M100 0L106 9H94Z"]`]; then `span.hd-dial-cap[aria-hidden=true]` > `span.hd-dial-pointer`; `span.hd-dial-readout` > numeral or seg7; [legends: `span.hd-dial-legend[data-end=min]`, `span.hd-dial-legend[data-end=max]`] | value, min, max, current, valueText | `--hd-value`, `--hd-angle` = start+frac·sweep, `--hd-angle-cur`, arc dasharray, active `d` = `arc(100,100,88,a(lo),a(hi)-a(lo))` written into the hidden buffer, which then becomes root `data-buf`; `aria-value*`; readout |
| stepper | min, max, step, unit, decimals, decLabel, incLabel | `div.hd-stepper[role=group][aria-label]` > `button.hd-step[type=button][data-dir="-1"][aria-label]` (icon minus), `span.hd-step-value[role=spinbutton][tabindex=0][aria-label]` > numeral, `button.hd-step[type=button][data-dir="1"][aria-label]` (icon plus) | value | spinbutton `aria-value*`; buttons get `aria-disabled` at their limit |
| segmented | options `[{id,label,icon?}]` | `div.hd-seg[role=radiogroup][aria-label]` > `span.hd-seg-indicator[aria-hidden=true]` + `button.hd-seg-opt[type=button][role=radio][data-hd-option][aria-checked][tabindex]` > `svg`? + `span.hd-seg-label` | value | `--hd-index` (−1 when none), `--hd-count` (written at build), roving `tabindex` |
| colourWheel | diameter (200) | `div.hd-wheel[role=img][aria-label]` > `canvas.hd-wheel-canvas[width][height][aria-hidden=true]`, `span.hd-wheel-thumb[aria-hidden=true]` | hue, sat, valueText | `--hd-wheel-x` = .5+.5·sat·sin h, `--hd-wheel-y` = .5−.5·sat·cos h, `--hd-wheel-rgb` "r g b" |
| ctStrip | min (2200), max (6500) | `div.hd-ct[role=slider][tabindex=0][aria-label]` > `span.hd-ct-track[aria-hidden=true]`, `span.hd-ct-thumb[aria-hidden=true]` | value | `--hd-value`, `aria-value*` ("4350 K") |
| numeral | unit, decimals, chars (4), degree (false) | `span.hd-numeral` > `span.hd-num-int`, `span.hd-num-frac`, `span.hd-num-deg`, `span.hd-num-unit`, `span.hd-num-trend[aria-hidden=true]` | value, unit, decimals, text, trend | texts from `E.format.split`; frac holds the locale separator plus digits; deg = U+00B0 when `degree`, else empty; `--hd-num-chars`; `data-hd-empty`; `data-trend` with glyph U+2191, U+2193 or U+2192 |
| segReadout | digits (4), decimals, unit | `span.hd-seg7[role=img][aria-label]` > `svg.hd-seg7-svg[viewBox="0 0 16·digits 20"][aria-hidden=true]` > `g.hd-seg7-digit[data-index][transform]` > 7 × `path.hd-seg7-seg[data-seg]` + `path.hd-seg7-dp`; `span.hd-seg7-unit` | value, text | `data-lit` (right-aligned), `aria-label`, `data-hd-empty` |
| arcGauge | min, max, unit, decimals, zones `[{from,to,tone}]`, quantize (0), crop (200) | `div.hd-gauge[data-hd-gauge=arc][role=meter][aria-label]` > `svg.hd-gauge-svg[viewBox="0 0 200 crop"][aria-hidden=true]` > `path.hd-gauge-track[pathLength=1000]`, `path.hd-gauge-zone[data-tone]`* (`arc(100,100,76,…)`), `path.hd-gauge-arc[pathLength=1000]` (`arc(100,100,88,-135,270)`), `g.hd-gauge-cap` > `circle.hd-gauge-capdot[cx=100][cy=12][r=7]`; `span.hd-gauge-readout` > numeral; `span.hd-gauge-min`, `span.hd-gauge-max` | value, tone | `--hd-value`, `--hd-angle` (−135…135), arc dasharray, `data-hd-tone` (state.tone, else the zone that contains the value), `aria-value*` |
| needleGauge | min, max, unit, decimals, zones, majors (10), minors (50), numerals (6, max 6), crop (200) | `div.hd-gauge[data-hd-gauge=needle][role=meter][aria-label]` > svg (same viewBox rule) > `path.hd-gauge-ticks[data-major=true]` (`ticks(100,100,72,88,-120,240,majors)`), `path.hd-gauge-ticks[data-minor=true]` (`ticks(100,100,80,88,-120,240,minors)`), `path.hd-gauge-zone[data-tone]`* (r 84), `text.hd-gauge-numeral[x][y]`* (r 54), `g.hd-gauge-needle` > `path.hd-gauge-needle-shape` (`needle(100,100,88,24,6.4,1.6)`) + `circle.hd-gauge-hub[cx=100][cy=100][r=9]`; readout; min and max spans | value | `--hd-value`, `--hd-angle` (−120…120; kept when `needleChanged` is false), `data-hd-tone`, `aria-value*` |
| bar | variant h\|v\|tank, min, max, unit, decimals, zones, readout | `div.hd-bar[role=meter][data-hd-variant][aria-label]` > `span.hd-bar-track[aria-hidden=true]` > `span.hd-bar-zone[data-tone]`* (`--hd-from`/`--hd-to`), `span.hd-bar-fill`; `span.hd-bar-readout` > numeral | value | `--hd-value`, `data-hd-tone`, `aria-value*` |
| lamp | – | `span.hd-lamp[role=img][aria-label]` > `span.hd-lamp-dot[aria-hidden=true]`, `span.hd-lamp-halo[aria-hidden=true]` | on, text | `data-hd-on`, `aria-label` |
| badge | icon | `span.hd-badge[data-tone]` > `svg`? + `span.hd-badge-text` | text, tone (ok\|warn\|crit\|info\|neutral), icon | text, `data-tone` |
| iconBadge | icon | `span.hd-iconbadge[aria-hidden=true]` > `svg.hd-ico` | icon | svg swap |
| sparkline | w (120), h (40), area (true) | `svg.hd-spark[role=img][aria-label][viewBox="0 0 w h"][width][height]` > `path.hd-spark-area` (omitted when area is false), `path.hd-spark-line`, `circle.hd-spark-last[r=3]`; no `preserveAspectRatio` | points (numbers or null; null leaves a gap), w, h, summary | `d`, size attributes when they change, end dot `cx`/`cy`/`hidden`, `aria-label`, `data-hd-empty` |
| frame | title, sub, icon, more (label or null), foot | `div.hd-frame` > `div.hd-head` > (`span.hd-icon`, `span.hd-title[id]`, `span.hd-sub`); `div.hd-body`; `div.hd-foot[hidden]`; `div.hd-state` > `span.hd-state-text`; `button.hd-more[type=button][aria-label]` (only with more) | title, sub, stateText, icon | texts; also returns `body`, `head`, `foot`, `titleId` |
| sheet | title, kind | `div.hd-sheet-layer[hidden]` > `div.hd-sheet[role=dialog][aria-modal=true][data-kind][aria-labelledby]` > `div.hd-sheet-head` > `h2.hd-sheet-title[id]`; `div.hd-sheet-body`; `div.hd-sheet-foot` | open, title | layer `hidden`; also returns `body`, `foot` |

### C3 Widget host (`E.widgets`)
- `register(type, {bindings, variants, create(api)})`, `get(type)`, `types()`.
- `host({widget, mode='view', size:{w,h}, t, lang='en', api: factory, moreLabel, parent})` returns `{el, body, family, update(), resize(size), setMode(m), destroy()}`.
- **Family resolution:** `widget.family`, else `parent.closest('[data-hd-family]')`'s value, else `glass`. It is always written as `data-hd-family` on `.hd-widget`. When `parent` is given, the host appends `el` to it before `create`.
- **Factory:** `factory({widget, mode, signal})` returns `{entity, can, command, moreInfo, history, announce}`. `history(binding, range)` resolves to `{points:[[t_ms, value|null], …], source}`.
- **The frozen `api`:** `body, widget, variant, family, mode` (getter), `t, fmt, prim, icons, signal, lang`, plus the six factory functions, plus:
  - `setTone(tone|null)`: a widget tone overrides the view tone; `null` clears the override.
  - `setStatus(text|null)`: the state-line text; `null` restores the default word.
- **State line precedence:** the words for unavailable, unknown, error and pending, then the widget status, then `E.format.stateText`.
- `update(view)` receives `view = {mode, entities:{<binding>: viewState|null}}`. The view state may carry `tone`.
- Wrapper: `div.hd-widget[data-hd-type][data-hd-variant][data-hd-family][data-hd-size][data-hd-status][data-hd-on?][data-hd-pending?][data-hd-stale?][data-hd-readonly?][data-hd-tone?][role=group][aria-labelledby]` > frame.

### C4 Lab URL (`/testdata/homedash-lab.html?…`) and `window.hdLab`

| Param | Values (default) |
|---|---|
| family | glass\|tactile\|minimal\|instrument\|placeholder (placeholder) |
| palette | theme\|dark\|light\|custom (dark) |
| scheme | dark\|light; forced for the dark and light palettes; selects the theme variant together with `theme` (dark) |
| theme | desktop theme id, with palette=theme (standard) |
| accent | 6 hex digits, palette=custom (8E5BD9) |
| hue | 0–359, palette=custom (285) |
| backdrop | theme\|solid\|gradient (theme), written as `data-hd-backdrop` |
| ease | native\|fallback (native); fallback writes `data-hd-ease="fallback"` |
| more | 0\|1 (1): the frame's more button |
| types, variants, states, sizes | comma lists. Primitive names and widget types are distinct; sizes are `m`/`l` or `WxH`. |
| dpr | number (1); the gallery reports an error if it differs from `devicePixelRatio` |
| motion | full\|reduced\|none (full) |
| lang | (en) |
| columns | cells per row (0 means wrap) |
| graphics | JSON array of extra selectors for the graphics contrast probe |
| scripts | comma list of `/testdata/…​.js` fixture scripts, loaded in order after the engine and before rendering |

States: `off,on,pending,disabled,unavailable,stale,readonly,error,alert,min,max,focus`.

`window.hdLab` exposes:
- `ready`, `error`, `errors`
- `fixtures.define(type, variant, entry)` and `fixtures.get(type, variant)`. Variant `*` matches any variant. The entry is `{widget?:{title, icon, options, family}, states?:{<state>: view | (name)=>view}, scenarios?:{<name>:{from: state|view, to: state|view}}, history?:{default: series, <state>: series}}`.
- `report(key, value)`
- `layout()`, `probes()` (Promise)
- `applyState(nameOrScenario)` (Promise). For a scenario it applies `from` with motion forced to none, waits two frames, restores motion and applies `to`.
- `seek(ms)`

### C5 Sheets (`ui/testdata/homedash-lab-sheets.json`)
- Top-level keys: `defaultSheets`, `lists` (`families`, `states`, `themes` (named lists of `{theme, scheme}`), `custom` (named lists of `{name, accent, hue, scheme}`), `heroes` (`H1…` mapped to `{type, variant}`)) and `sheets`.
- A sheet id is a pattern with `{family}`, `{hero}` and `{type}` placeholders and an optional `~variant` suffix.
- Sheet fields: `kind` (prim\|type\|themes\|risk\|hero\|motion\|side-by-side), `families`, `types`, `variants`, `palettes`, `themes`, `custom`, `states` (a named set, sets joined with `+`, or an array), `sizes`, `dpr` (number or array), `motion` (string or array), `backdrop`, `ease`, `more`, `columns`, `frames`, `scenarios`, `graphics`, `scripts`.
- Resolution order: literal ids first, then patterns in file order. A placeholder value must belong to the allowed list. An unknown id fails with all known patterns listed.
- Each panel is one cross-product combination: family × palette × look (theme entry or custom entry) × dpr × motion × scenario.
- Output goes to the repository-root `reports/homedash-lab/` (`.gitignore:127`):
  - images are at most 2400 device px per side and numbered `<id>-NN.png` when split
  - `probes.json` = `{generated, report_only, sheets:[{id, images, panels:[{label, url, probes, extra, errors, oversized}]}], failures}`
- `HOMEDASH_LAB_SHEETS` is a comma list; when empty, `defaultSheets` applies.

### C6 Probes (each failure is a readable string)
The probes and token pairs are unchanged from the saved plan, with these additions:
- **graphics:** the default selector list now reads `.hd-dial-arc, .hd-gauge-arc, .hd-gauge-needle-shape, .hd-spark-line, .hd-slider-fill, .hd-bar-fill, .hd-seg7-seg[data-lit="true"], .hd-toggle-thumb, .hd-lamp-dot`, plus the panel's `graphics` selectors.
- **extra:** `hdLab.report` values come back as `extra`. A value with `ok === false` is a failure that shows `message`.

## Task 0: Workspace
Copy verbatim from the saved file, lines 182–192.

## Tasks 1–7
Copy verbatim from the saved file, lines 193–1063.

## Task 8: Token, bridge, base and backdrop CSS, starters, bundle entry

**Files:** the 9 parts, `scripts/build-ui-bundles.js`, `ui/css/homedash.bundle.css`, `scripts/homedash-tests/bundle.test.mjs`. **Read first:** visual part 1.1–1.4, 5, 9.1–9.3; `scripts/build-ui-bundles.js:263-401`; `ui/desktop_app_materials_test.go:645-708`.

- [ ] Failing test: `bundle.test.mjs` as in the saved file (saved Task 8 block), plus this test:
```js
['base CSS carries the backdrops, rotation groups and the easing fallback', () => { const base = read('ui/css/homedash-base.css'), tok = read('ui/css/homedash-tokens.css');
  for (const s of ['[data-hd-backdrop="theme"]', '[data-hd-backdrop="solid"]', '[data-hd-backdrop="gradient"]', '.hd-dial-handle', '.hd-gauge-needle', '.hd-gauge-cap', '.hd-dial-current', 'transform-origin: 100px 100px']) assert.ok(base.includes(s), s);
  assert.ok(tok.includes('.hd-root[data-hd-ease="fallback"]'), 'ease fallback'); }]
```
- [ ] Run it. Expected: FAIL.
- [ ] `homedash-tokens.css`: the saved block, plus this line at the end:
```css
.hd-root[data-hd-ease="fallback"] { --hd-ease-needle: cubic-bezier(.3, 1.3, .5, 1); }
```
- [ ] `homedash-bridge.css`: the saved block, unchanged.
- [ ] `homedash-base.css`: the saved block, plus these lines at the end:
```css
/* Backdrops (visual part 5): the root itself or a .hd-backdrop layer below it. */
:is(.hd-root.hd-backdrop, .hd-root .hd-backdrop) { background: var(--hd-canvas); }
:is(.hd-root[data-hd-backdrop="theme"].hd-backdrop, .hd-root[data-hd-backdrop="theme"] .hd-backdrop) { background: var(--hd-canvas-fill); }
:is(.hd-root[data-hd-backdrop="solid"].hd-backdrop, .hd-root[data-hd-backdrop="solid"] .hd-backdrop) { background: var(--hd-canvas); }
:is(.hd-root[data-hd-backdrop="gradient"].hd-backdrop, .hd-root[data-hd-backdrop="gradient"] .hd-backdrop) { background: radial-gradient(60% 50% at 15% 0%, color-mix(in srgb, var(--hd-accent) 9%, transparent), transparent), linear-gradient(160deg, var(--hd-canvas), var(--hd-canvas-2)); }
/* Rotated dial and gauge groups (viewBox 0 0 200 200, centre 100,100); CSS transitions sweep them along the arc. */
:is(.hd-dial-handle, .hd-gauge-needle, .hd-gauge-cap) { transform-box: view-box; transform-origin: 100px 100px; transform: rotate(var(--hd-angle, 0deg)); }
.hd-dial-current { transform-box: view-box; transform-origin: 100px 100px; transform: rotate(var(--hd-angle-cur, 0deg)); }
.hd-dial-active[data-buf] { opacity: 0; }
.hd-dial[data-buf="a"] .hd-dial-active[data-buf="a"], .hd-dial[data-buf="b"] .hd-dial-active[data-buf="b"] { opacity: 1; }
```
- [ ] Add the six starters, the bundle entry, `npm run build:ui` and the check steps, exactly as in the saved Task 8.
- [ ] Run `npm run test:homedash` and `npm run check:ui`. Expected: PASS.
- [ ] Commit `feat(homedash): tokens, palettes, theme bridge, base, backdrops and bundle`.

## Task 9: Node guard (CSS and JS rules)

**Files:** `scripts/homedash-tests/css-lint.mjs`, `scripts/homedash-tests/guard.test.mjs`. **Read first:** visual part 9.4; `scripts/test-docker-manager-ui.mjs:2062-2115`.

- [ ] Failing test `guard.test.mjs`: the saved test file with these changes:
  - In `BAD`, replace `.hd-d { backdrop-filter: blur(30px); }` with `.hd-glass-pane { backdrop-filter: blur(30px); }`, and add `@media (prefers-reduced-motion: no-preference) { .hd-root:not([data-hd-motion="reduced"]):not([data-hd-motion="none"]) .hd-x { transition: width 1s; } }`.
  - In `GOOD`, change `.hd-sheet { backdrop-filter: blur(18px); }` to `.hd-frame { backdrop-filter: blur(18px); } .hd-sheet { backdrop-filter: blur(24px); }`, and add `@media (prefers-reduced-motion: no-preference) { .hd-root:not([data-hd-motion="reduced"]):not([data-hd-motion="none"]) .hd-dial-arc { transition: stroke-dasharray var(--hd-dur-value) var(--hd-ease-out), opacity var(--hd-dur-quick) linear; } }`.
  - In the `want` list, replace `'backdrop-filter outside'` with `'backdrop-filter outside .hd-frame and .hd-sheet'`, and add `'transition of width'`.
- [ ] Run it. Expected: FAIL.
- [ ] `css-lint.mjs`: the saved code, with two replacements.
  1. Replace the backdrop-filter branch with:
```js
if (/^(-webkit-)?backdrop-filter$/.test(name) && value !== 'none') {
  if (!parts.every(p => /\.hd-(frame|sheet)(?=[.:[]|$)[^\s>+~]*$/.test(p))) v.push(`${file}: backdrop-filter outside .hd-frame and .hd-sheet (${sel})`);
  for (const m of value.matchAll(/blur\(\s*([\d.]+)px/g)) if (Number(m[1]) > 24) v.push(`${file}: blur ${m[1]}px over 24px`);
}
```
  2. Directly after the existing motion-gate check, add the transition property rule:
```js
if (name === 'transition' || name === 'transition-property') {
  const props = name === 'transition' ? splitSelectors(value).map(s => s.trim().split(/\s+/)[0]) : value.split(',').map(s => s.trim());
  for (const p of props) if (p !== 'none' && !['transform', 'opacity', 'stroke-dasharray'].includes(p)) v.push(`${file}: transition of ${p}; only transform, opacity and stroke-dasharray`);
}
```
  (`splitSelectors` splits on top-level commas, so commas inside `cubic-bezier(…)` stay in place.) Keyframes stay limited to transform and opacity.
- [ ] Run `npm run test:homedash`. Expected: PASS.
- [ ] Commit `test(homedash): stylesheet and source guard`.

## Task 10: hd-geometry.js
Copy verbatim from the saved file, lines 1320–1426. The builders take the centre as a parameter, so the 200-unit geometry needs no change.

## Task 11: hd-icons.js and hd-prim-displays.js

**Files:** `ui/js/homedash/hd-icons.js` (the saved code, unchanged), `ui/js/homedash/hd-prim-displays.js`, `scripts/homedash-tests/displays.test.mjs`. **Read first:** C2.

- [ ] Failing test: the saved `displays.test.mjs`, with these changes.
  - `OPTS.needleGauge = { label: 'P', min: 0, max: 100, unit: '%', majors: 10, minors: 50, numerals: 6, zones: [{ from: 80, to: 100, tone: 'crit' }] }`
  - `OPTS.arcGauge` gets `quantize: 0`.
  - `OPTS.numeral = { chars: 6, unit: 'W' }` is unchanged.
  - Replace the `gauges` test with:
```js
['gauges sweep one arc and rotate their groups', () => { const { E } = boot(); const dash = el => el.style.getPropertyValue('stroke-dasharray');
  const a = E.prim.arcGauge(OPTS.arcGauge); a.set({ value: 50 });
  assert.equal(a.el.querySelector('svg').getAttribute('viewBox'), '0 0 200 200'); assert.equal(v(a.el, '--hd-angle'), '0deg'); assert.equal(v(a.el, '--hd-value'), '0.5000');
  const arcEl = a.el.querySelector('.hd-gauge-arc'); assert.equal(arcEl.getAttribute('pathLength'), '1000'); assert.equal(dash(arcEl), '500 1000');
  assert.equal(arcEl.getAttribute('d'), E.geometry.arc(100, 100, 88, -135, 270)); assert.equal(a.el.querySelectorAll('.hd-gauge-arc').length, 1);
  const cap = a.el.querySelector('g.hd-gauge-cap .hd-gauge-capdot'); assert.deepEqual([cap.getAttribute('cx'), cap.getAttribute('cy')], ['100', '12']);
  a.set({ value: 96 }); assert.equal(a.el.getAttribute('data-hd-tone'), 'crit'); assert.equal(cap.getAttribute('cx'), '100', 'the cap never moves by cx');
  const q = E.prim.arcGauge(Object.assign({}, OPTS.arcGauge, { quantize: 40 })); q.set({ value: 51 }); assert.equal(dash(q.el.querySelector('.hd-gauge-arc')), '500 1000'); assert.equal(q.el.getAttribute('data-hd-quantize'), '40');
  assert.equal(E.prim.arcGauge(Object.assign({}, OPTS.arcGauge, { crop: 172 })).el.querySelector('svg').getAttribute('viewBox'), '0 0 200 172');
  const nd = E.prim.needleGauge(OPTS.needleGauge); nd.set({ value: 50 }); assert.equal(v(nd.el, '--hd-angle'), '0deg'); nd.set({ value: 50.1 }); assert.equal(v(nd.el, '--hd-angle'), '0deg');
  nd.set({ value: 60 }); assert.equal(v(nd.el, '--hd-angle'), '24deg');
  assert.equal(nd.el.querySelector('.hd-gauge-needle-shape').getAttribute('d'), E.geometry.needle(100, 100, 88, 24, 6.4, 1.6));
  assert.equal(nd.el.querySelector('[data-major="true"]').getAttribute('d'), E.geometry.ticks(100, 100, 72, 88, -120, 240, 10));
  assert.deepEqual(nd.el.querySelectorAll('.hd-gauge-numeral').map(t => t.textContent), ['0', '20', '40', '60', '80', '100']);
  const b = E.prim.bar(OPTS.bar); b.set({ value: 25 }); assert.equal(v(b.el, '--hd-value'), '0.2500'); }]
```
  - In the `numeral` test, add:
```js
const d = E.prim.numeral({ chars: 4, degree: true, unit: 'C' }); d.set({ value: 21.5, decimals: 1 });
assert.deepEqual(['.hd-num-int', '.hd-num-frac', '.hd-num-deg', '.hd-num-unit'].map(s => d.el.querySelector(s).textContent), ['21', '.5', fc(0xb0), 'C']);
assert.equal(n.el.querySelector('.hd-num-deg').textContent, '');
```
  - In the `sparkline` test, add `assert.equal(E.prim.sparkline({ label: 'x', area: false }).el.querySelector('.hd-spark-area'), null);`
- [ ] Run it. Expected: FAIL.
- [ ] Implement, following the saved rules 1–10 with these changes:
  - **R4 helper in `E.prim.util`:**
```js
function dash(path, frac, quantize) { const f = quantize > 0 ? Math.round(frac * quantize) / quantize : frac;
    path.style.setProperty('stroke-dasharray', String(Math.round(f * 10000) / 10) + ' 1000'); }
```
    Export it as `util.dash`. Also export `util.fmtN = v => String(Math.round(v * 1000) / 1000)`.
  - **arcGauge:**
    - Root gets `data-hd-quantize` when `quantize` > 0.
    - The svg viewBox is `0 0 200 ${crop||200}`.
    - Paths follow C2. The arc's `d` is the full sweep; every `set` calls `util.dash(arc, frac, quantize)`.
    - The cap is `g.hd-gauge-cap` around a fixed circle. Write `--hd-angle` only.
  - **needleGauge:**
    - Ticks and zones per C2.
    - Numerals: `n = clamp(numerals, 0, 6)`. For i in 0…n−1: value = min + i·(max−min)/(n−1); angle = −120 + 240·i/(n−1); `text.hd-gauge-numeral` with x/y = `point(100,100,54,angle)` and text `E.format.number(value,{lang})`.
    - Needle and hub per C2.
  - **numeral:** `span.hd-num-deg` always exists, after `.hd-num-frac`. Its text is `String.fromCodePoint(0xb0)` when `opts.degree` (or `state.degree`) is set, else empty. A `text` override clears frac, deg and unit.
  - **sparkline:** `opts.area === false` omits `path.hd-spark-area`.
- [ ] Run it. Expected: PASS. Commit `feat(homedash): icon starter set and static display primitives`.

## Task 12: hd-prim-controls.js

**Files:** `ui/js/homedash/hd-prim-controls.js`, `scripts/homedash-tests/controls.test.mjs`.

- [ ] Failing test: the saved `controls.test.mjs`, with these changes.
  - `OPTS.knob = { label: 'Dim', variant: 'ring', unit: '%' }` stays. Add `OPTS.pushButton.dir = -1`.
  - In the pushButton test, add `assert.equal(p.el.getAttribute('data-dir'), '-1');`
  - Replace the `slider and knob` test's knob half with:
```js
const k = E.prim.knob(OPTS.knob); const arcEl = k.el.querySelector('.hd-dial-arc'), knobDot = k.el.querySelector('g.hd-dial-handle .hd-dial-knob');
assert.equal(k.el.querySelector('svg').getAttribute('viewBox'), '0 0 200 200'); assert.equal(arcEl.getAttribute('pathLength'), '1000');
assert.equal(arcEl.getAttribute('d'), E.geometry.arc(100, 100, 88, -135, 270)); assert.deepEqual([knobDot.getAttribute('cx'), knobDot.getAttribute('cy')], ['100', '12']);
k.set({ value: 0 }); assert.equal(v(k.el, '--hd-angle'), '-135deg'); assert.equal(arcEl.style.getPropertyValue('stroke-dasharray'), '0 1000');
k.set({ value: 100 }); assert.equal(v(k.el, '--hd-angle'), '135deg'); assert.equal(arcEl.style.getPropertyValue('stroke-dasharray'), '1000 1000'); assert.equal(knobDot.getAttribute('cx'), '100');
assert.equal(k.el.querySelector('.hd-dial-readout .hd-num-int').textContent, '100'); assert.equal(k.el.querySelectorAll('.hd-dial-active').length, 0);
const t = E.prim.knob({ label: 'T', min: 5, max: 30, step: 0.5, decimals: 1, activeSegment: true, legends: true, readout: 'seg7' });
t.set({ value: 21.5, current: 19.5 }); assert.equal(t.el.getAttribute('data-buf'), 'a');
const lo = -135 + (19.5 - 5) / 25 * 270, hi = -135 + (21.5 - 5) / 25 * 270;
assert.equal(t.el.querySelector('.hd-dial-active[data-buf="a"]').getAttribute('d'), E.geometry.arc(100, 100, 88, lo, hi - lo));
t.set({ value: 22, current: 19.5 }); assert.equal(t.el.getAttribute('data-buf'), 'b'); t.set({ value: 22, current: 19.5 }); assert.equal(t.el.getAttribute('data-buf'), 'b', 'an unchanged segment keeps its buffer');
assert.ok(t.el.querySelector('g.hd-dial-current .hd-dial-marker')); assert.ok(t.el.querySelector('.hd-dial-readout .hd-seg7'));
assert.deepEqual(t.el.querySelectorAll('.hd-dial-legend').map(x => x.textContent), ['5', '30']);
const s40 = E.prim.knob({ label: 'q', quantize: 40 }); s40.set({ value: 51 }); assert.equal(s40.el.querySelector('.hd-dial-arc').style.getPropertyValue('stroke-dasharray'), '500 1000');
```
- [ ] Run it. Expected: FAIL.
- [ ] Implement, following the saved rules 1–5 and 7–11. Rule 6 (the knob) becomes:
  - Opts per C2: `start = opts.start ?? -135`, `sweep = opts.sweep ?? 270`, min, max, step, quantize, ticks (default 10), legends, `readout` (`numeral` default, `seg7`, `none`), activeSegment.
  - The svg follows C2. The handle `g` holds a fixed circle. The readout is `E.prim.numeral({chars:4, unit, lang})` or `E.prim.segReadout({digits:4, decimals, unit})`. Legends show `E.format.number(min/max,{lang})`.
  - `set`:
    - frac via `util.writeValue`; `--hd-angle` = `util.deg(start + frac·sweep)`; `util.dash(arc, frac, quantize)`; root `data-hd-quantize` when quantize > 0
    - readout `set({value, decimals})`
    - with activeSegment and a numeric `state.current`:
      - cf = fraction(current); `--hd-angle-cur` = `deg(start + cf·sweep)`
      - lo/hi = angles of min(cf, frac) and max(cf, frac); d = `arc(100,100,88,lo,hi-lo)`
      - if d differs from the visible buffer's d: write it into the other buffer and switch root `data-buf` to that one. On the first set, buffer `a` is written and shown.
  - pushButton writes `data-dir` when `opts.dir` is −1 or 1.
- [ ] Run it. Expected: PASS. Commit `feat(homedash): static control primitives`.

## Task 13: hd-widgets.js, entity tile, load contract

**Files:** `ui/js/homedash/hd-widgets.js`, `ui/js/homedash/widgets/hd-w-misc.js`, `scripts/homedash-tests/widgets.test.mjs`, `scripts/homedash-tests/contract.test.mjs` (the saved contract test, unchanged).

- [ ] Failing test: the saved `widgets.test.mjs`, with these changes.
  - The factory's history becomes `history: () => Promise.resolve({ points: [[1, 2], [2, null]], source: 'ring' })`.
  - The lifecycle test asserts `a('data-hd-family') === 'glass'` and `api.family === 'glass'` (no parent, no widget family).
  - Add this test:
```js
['family from the parent, setTone, setStatus and the history shape', async () => { const { E, document } = boot(); let api;
  E.widgets.register('zz_two', { bindings: ['primary'], variants: ['a'], create(x) { api = x; return { update() {} }; } });
  const root = document.createElement('section'); root.setAttribute('data-hd-family', 'tactile'); const stage = document.createElement('div'); root.appendChild(stage);
  const state = { view: view({ value: 'on', tone: 'warn' }) };
  const h = E.widgets.host({ widget: { id: 'w', type: 'zz_two', bindings: { primary: 'x' } }, size: { w: 1, h: 1 }, t: E.core.makeT(WORDS), api: factory(state), parent: stage });
  assert.equal(h.el.parentNode, stage); assert.equal(api.family, 'tactile'); assert.equal(h.el.getAttribute('data-hd-family'), 'tactile'); assert.equal(h.family, 'tactile');
  assert.equal(h.el.getAttribute('data-hd-tone'), 'warn'); api.setTone('crit'); assert.equal(h.el.getAttribute('data-hd-tone'), 'crit'); h.update(); assert.equal(h.el.getAttribute('data-hd-tone'), 'crit');
  api.setTone(null); assert.equal(h.el.getAttribute('data-hd-tone'), 'warn');
  api.setStatus('Heating to 21.5'); assert.equal(h.el.querySelector('.hd-state-text').textContent, 'Heating to 21.5');
  state.view = view({ value: 'on', pending: true }); h.update(); assert.equal(h.el.querySelector('.hd-state-text').textContent, 'Sending');
  state.view = view({ value: 'on' }); api.setStatus(null); assert.equal(h.el.querySelector('.hd-state-text').textContent, 'On');
  assert.deepEqual(JSON.parse(JSON.stringify(await api.history('primary', '24h'))), { points: [[1, 2], [2, null]], source: 'ring' });
  const g = E.widgets.host({ widget: { id: 'w2', type: 'zz_two', family: 'minimal', bindings: {} }, size: { w: 1, h: 1 }, t: k => k, api: factory(state), parent: stage });
  assert.equal(g.el.getAttribute('data-hd-family'), 'minimal'); }]
```
- [ ] Run it. Expected: FAIL.
- [ ] Implement `hd-widgets.js`, following the saved rules 1–11 with these changes:
  - **Rule 3a:** `family = widget.family || (opts.parent && opts.parent.closest('[data-hd-family]') ? opts.parent.closest('[data-hd-family]').getAttribute('data-hd-family') : '') || 'glass'`. It is written as `data-hd-family` and exposed as `api.family` and `host.family`. When `opts.parent` is given, append `el` to it right after building the wrapper, before `create`.
  - **Rule 5a:** the api gets these two functions:
    - `setTone(t)`: `toneOverride = t == null ? undefined : t`, then write `data-hd-tone` = override ?? the view tone.
    - `setStatus(text)`: `statusText = text == null ? null : String(text)`, then re-run `writeState()`.
  - **Rule 7a:** `writeState()` sets `frame.set({stateText})` with:
    - `w = E.format.stateWord(p)`
    - stateText = `['unavailable','unknown','error','pending'].includes(w) ? t(STATE_PREFIX + w) : statusText !== null ? statusText : (p ? E.format.stateText(p, t) : '')`
    - `update()` calls `writeState()`. Tone = `toneOverride ?? (p && p.tone)`.
  - `hd-w-misc.js` keeps the saved rules.
- [ ] Run it. Expected: PASS. Commit `feat(homedash): widget registry, frame host and entity tile`.

## Task 14: Lab page, placeholder CSS, gallery, fixture registry

**Files:** `ui/testdata/homedash-lab.html` (saved, unchanged), `ui/testdata/homedash-lab.css`, `ui/testdata/homedash-lab.js`, `scripts/homedash-tests/lab.test.mjs` (saved, unchanged), and a probes stub (`window.hdLabProbes={}`) until Task 15. **Read first:** C4; the saved Task 14.

- [ ] Run the failing test, the saved `lab.test.mjs`. Expected: FAIL.
- [ ] `homedash-lab.css`: the saved file, with these replacements.
  1. `.hd-lab-panel` loses `background: var(--hd-canvas);`. The gallery adds class `hd-backdrop` to the panel, so `homedash-base.css` paints it.
  2. Add `.hd-lab-cells[data-hd-lab-columns] { display: grid; grid-template-columns: repeat(var(--hd-lab-columns), max-content); }`.
  3. Replace the dial and gauge stroke rules with these (200-unit viewBox):
```css
[data-hd-family="placeholder"] :is(.hd-dial-track, .hd-gauge-track) { fill: none; stroke: var(--hd-sunken); stroke-width: 14; }
[data-hd-family="placeholder"] :is(.hd-dial-arc, .hd-gauge-arc) { fill: none; stroke: var(--hd-ink); stroke-width: 14; }
[data-hd-family="placeholder"] .hd-dial-active { fill: none; stroke: var(--hd-ink-muted); stroke-width: 6; }
[data-hd-family="placeholder"] .hd-gauge-zone { fill: none; stroke: var(--hd-ink-muted); stroke-width: 6; }
[data-hd-family="placeholder"] :is(.hd-dial-ticks, .hd-gauge-ticks) { fill: none; stroke: var(--hd-ink-muted); stroke-width: 2.5; }
[data-hd-family="placeholder"] :is(.hd-dial-knob, .hd-gauge-capdot, .hd-gauge-hub, .hd-gauge-needle-shape, .hd-dial-marker) { fill: var(--hd-ink); }
[data-hd-family="placeholder"] .hd-gauge-numeral { fill: var(--hd-ink-muted); font-size: 18px; text-anchor: middle; dominant-baseline: middle; }
[data-hd-family="placeholder"] .hd-dial-legend { position: absolute; bottom: 0; font-size: 11px; color: var(--hd-ink-muted); }
[data-hd-family="placeholder"] .hd-dial-legend[data-end="min"] { left: 8px; } [data-hd-family="placeholder"] .hd-dial-legend[data-end="max"] { right: 8px; }
[data-hd-family="placeholder"] .hd-num-deg { font-size: 13px; }
```
  4. Delete the saved `.hd-gauge-needle` transform rule; `homedash-base.css` owns it now.
- [ ] `homedash-lab.js`: the saved tables (`DEG_C`, `FAMILIES`, `PALETTES`, `MOTIONS`, `LAB_STATES`, `PRIM_VARIANTS`, `LAB_WIDGET_SIZES`, `LAB_WORDS`, `primOpts`, `labState`, `labView`) and the saved assembly rules 1–15, with these changes.
  - **R1, at the very top** (synchronous, before anything else):
```js
const fixtureStore = new Map();
window.hdLab = { ready: false, error: '', errors: [], instances: [], extra: {},
  fixtures: { define(type, variant, entry) { if (typeof type !== 'string' || !entry || typeof entry !== 'object') throw new Error('hdLab.fixtures.define(type, variant, entry)'); fixtureStore.set(type + '/' + (variant || '*'), entry); },
              get(type, variant) { return fixtureStore.get(type + '/' + variant) || fixtureStore.get(type + '/*') || null; } },
  report(key, value) { hdLab.extra[String(key)] = value; } };
const resolveView = (entry, s) => { if (s && typeof s === 'object') return s; const f = entry && entry.states && entry.states[s]; return typeof f === 'function' ? f(s) : f || labView(s); };
const resolveHistory = (entry, s) => { const h = entry && entry.history; return (h && typeof s === 'string' && h[s]) || (h && h.default) || { points: [], source: 'ring' }; };
```
  - **Rule 2 (params):** also parse the R3 parameters with these checks:
    - `backdrop` ∈ theme\|solid\|gradient
    - `accent` matches `/^[0-9a-fA-F]{6}$/`
    - `hue` is an integer 0–359
    - `ease` ∈ native\|fallback
    - `more` ∈ 0\|1
    - `columns` is an integer ≥ 0
    - `graphics` is `JSON.parse`d into an array of strings
    - every entry of `scripts` matches `/^\/testdata\/[\w.\/-]+\.js$/`
  - **New rule 4b:** load every script of `scripts` in order (create a `script` element, set its `src`, append it to the head, await `load`; `error` sets `hdLab.error`). This happens before rendering, after the shell CSS.
  - **Rule 6 (panel root):** add `data-hd-backdrop` (default theme), `data-hd-ease="fallback"` when requested, and class `hd-backdrop`. Custom palette = `solvePalette({accent:'#'+accent, surface_hue:hue, scheme})`. With columns > 0, every `.hd-lab-cells` gets `data-hd-lab-columns` and `style.setProperty('--hd-lab-columns', columns)`.
  - **Rule 10 (widget cell):**
    - `entry = hdLab.fixtures.get(type, variant)`.
    - The widget doc merges `entry.widget` (title, icon, options, family) over the saved defaults.
    - The fixture api becomes `entity: () => resolveView(entry, cur)` and `history: () => Promise.resolve(resolveHistory(entry, cur))`.
    - `moreLabel` = `more === 0 ? null : 'More'`.
    - Pass `parent: stage`, so the family comes from the panel root.
    - The pushed instance holds `entry` and `apply(s) { cur = s; host.update(); }`.
  - **Rule 14 (applyState)**, replaced by:
```js
hdLab.applyState = async function (name) {
  if (LAB_STATES.includes(name)) { for (const i of hdLab.instances) i.apply(name); return hdLab.instances.length; }
  const targets = hdLab.instances.filter(i => i.entry && i.entry.scenarios && i.entry.scenarios[name]);
  if (!targets.length) throw new Error('unknown state or scenario ' + name);
  const root = document.querySelector('.hd-lab-panel'), motion = root.getAttribute('data-hd-motion');
  root.setAttribute('data-hd-motion', 'none'); for (const i of targets) i.apply(i.entry.scenarios[name].from);
  await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)));
  root.setAttribute('data-hd-motion', motion); for (const i of targets) i.apply(i.entry.scenarios[name].to);
  return targets.length;
};
```
    For primitive instances, `apply(s)` accepts only state names.
  - **Rule 13 (probes):** `hdLab.probes = async () => Object.assign(await hdLabProbes.run({root, palette, motion, entries: hdLab.instances, states, graphics}), { extra: hdLab.extra })`.
- [ ] Run `npm run test:homedash`. Expected: PASS. Commit `test(homedash): control lab page, fixture registry, placeholder look and gallery`.

## Task 15: Lab probes

**Files:** `ui/testdata/homedash-lab-probes.js`, `scripts/homedash-tests/lab.test.mjs` (the saved added test).

- [ ] Follow the saved Task 15 (failing test, code, run, commit), with two replacements in the code.
```js
function graphics(root, r, extraSelectors) { const sel = [GRAPHICS].concat(extraSelectors || []).join(', ');
    for (const el of root.querySelectorAll(sel)) { /* body as in the saved graphics() loop */ } }
async function run(o) {
    const r = { contrast: { checked: 0, failures: [], warnings: [], unmeasured: 0 }, hit: { checked: 0, failures: [] }, shift: { checked: 0, failures: [] }, animations: { checked: false, running: [], failures: [] }, backdrop: { count: 0, limit: 0, failures: [] }, resolution: { checked: 0, failures: [] } };
    tokens(o.root, o.palette, r); text(o.root, r); graphics(o.root, r, o.graphics); hit(o.root, r); shift(o.root, (o.entries || []).filter(e => !e.entry), o.states || [], r); backdrop(o.root, r); await fonts(r); await animations(o.motion, r);
    return r;
}
```
  The live shift check skips instances that have a fixture entry, because their states change the content on purpose.
- [ ] Commit `test(homedash): control lab probes`.

## Task 16: Sheet definitions, Go resolver and control-lab test

**Files:** `ui/testdata/homedash-lab-sheets.json`, `ui/desktop_homedash_lab_plan_test.go`, `ui/desktop_homedash_control_lab_browser_test.go`. **Read first:** C5; `ui/desktop_theme_sweep_browser_test.go:224-319, 367-370`; `ui/desktop_theme_registry_test.go:32-61, 202`; `internal/audit/ui_browser_ci_test.go:15-104` (`TestHomeDashLab…` falls into "General A-M", `TestDesktopHomeDash…` into "Desktop H-R").

- [ ] Write `ui/testdata/homedash-lab-sheets.json` (complete initial file):
```json
{
  "defaultSheets": ["prim-placeholder", "prim-placeholder~reduced", "type-entity-placeholder", "themes-placeholder"],
  "lists": {
    "families": ["glass", "tactile", "minimal", "instrument", "placeholder"],
    "states": {
      "all": ["off", "on", "pending", "disabled", "unavailable", "stale", "readonly", "error", "alert", "min", "max", "focus"],
      "core": ["off", "on", "pending", "unavailable", "alert", "focus"],
      "extra": ["disabled", "stale", "readonly", "error", "min", "max"],
      "trio": ["off", "on", "alert"]
    },
    "themes": {
      "standard": [{"theme": "standard", "scheme": "dark"}],
      "themes16": [{"theme": "standard", "scheme": "dark"}, {"theme": "fruity", "scheme": "light"}, {"theme": "fruity", "scheme": "dark"}, {"theme": "amiga", "scheme": "light"},
        {"theme": "cyberpunk", "scheme": "dark"}, {"theme": "black_gloss", "scheme": "dark"}, {"theme": "crazy", "scheme": "dark"}, {"theme": "lollipop", "scheme": "light"},
        {"theme": "organic", "scheme": "dark"}, {"theme": "electronic_dreams", "scheme": "dark"}, {"theme": "windows_xp", "scheme": "light"}, {"theme": "green_slime", "scheme": "dark"},
        {"theme": "voxel_world", "scheme": "light"}, {"theme": "orbit", "scheme": "dark"}, {"theme": "minimal", "scheme": "light"}, {"theme": "minimal", "scheme": "dark"}],
      "risk6": [{"theme": "fruity", "scheme": "light"}, {"theme": "lollipop", "scheme": "light"}, {"theme": "amiga", "scheme": "light"}, {"theme": "crazy", "scheme": "dark"}, {"theme": "cyberpunk", "scheme": "dark"}, {"theme": "green_slime", "scheme": "dark"}],
      "risk8": [{"theme": "fruity", "scheme": "light"}, {"theme": "lollipop", "scheme": "light"}, {"theme": "amiga", "scheme": "light"}, {"theme": "crazy", "scheme": "dark"}, {"theme": "cyberpunk", "scheme": "dark"}, {"theme": "green_slime", "scheme": "dark"}, {"theme": "standard", "scheme": "dark"}, {"theme": "organic", "scheme": "dark"}]
    },
    "custom": {
      "custom3": [{"name": "warm-light", "accent": "C2410C", "hue": 40, "scheme": "light"}, {"name": "purple", "accent": "8B5CF6", "hue": 270, "scheme": "dark"}, {"name": "near-black", "accent": "22D3EE", "hue": 220, "scheme": "dark"}]
    },
    "heroes": {
      "H1": {"type": "switch", "variant": "pill"}, "H2": {"type": "switch", "variant": "rocker"}, "H3": {"type": "light", "variant": "dial"}, "H4": {"type": "climate", "variant": "dial"},
      "H5": {"type": "gauge", "variant": "needle"}, "H6": {"type": "gauge", "variant": "arc"}, "H7": {"type": "temperature", "variant": "numeral"}, "H8": {"type": "sensor", "variant": "sparkline"}
    }
  },
  "sheets": [
    {"id": "prim-{family}", "kind": "prim", "palettes": ["dark", "light", "theme"], "themes": "standard", "states": "all", "sizes": ["m"], "dpr": 1, "motion": "full"},
    {"id": "prim-{family}~reduced", "kind": "prim", "palettes": ["dark"], "states": "all", "sizes": ["m"], "dpr": 1, "motion": "reduced"},
    {"id": "prim-{family}~gradient", "kind": "prim", "palettes": ["dark", "light"], "states": "core", "sizes": ["m"], "backdrop": "gradient"},
    {"id": "prim-{family}~custom", "kind": "prim", "palettes": ["custom"], "custom": "custom3", "states": "trio", "sizes": ["m"]},
    {"id": "type-{type}-{family}", "kind": "type", "palettes": ["dark", "light", "theme"], "themes": "standard", "states": "all"},
    {"id": "themes-{family}", "kind": "themes", "palettes": ["theme"], "themes": "themes16", "types": ["toggle", "slider", "arcGauge", "numeral"], "states": "trio", "sizes": ["m"]},
    {"id": "motion-type-{type}-{family}", "kind": "motion", "palettes": ["dark"], "states": ["off"], "motion": ["full"], "frames": [0, 50, 100, 200, 400, 800], "scenarios": ["on"]}
  ]
}
```
  M1 Task 0 adds its `hero`, `risk`, `motion-{hero}` and `side-by-side` entries and, if it needs them, scripts. It edits entries in place; a duplicate id is a load error.
- [ ] Failing Go unit test, in `desktop_homedash_lab_plan_test.go` below the code:
```go
func TestHomeDashLabSheetResolve(t *testing.T) {
	file, err := homedashLabLoadFile(filepath.Join("testdata", "homedash-lab-sheets.json"))
	if err != nil { t.Fatal(err) }
	known := map[string]bool{}
	for _, th := range parseDesktopThemeRegistry(t) { known[th.ID] = true }
	for name, list := range file.Lists.Themes { for _, ref := range list { if !known[ref.Theme] { t.Errorf("list %s: unknown theme %q", name, ref.Theme) } } }
	if got := strings.Join(homedashLabSheetIDs("", file), ","); got != strings.Join(file.DefaultSheets, ",") { t.Fatalf("defaults %q", got) }
	for id, want := range map[string]int{"prim-placeholder": 3, "prim-placeholder~reduced": 1, "prim-glass~custom": 3, "type-entity-placeholder": 3, "themes-minimal": 16, "motion-type-toggle-glass": 1} {
		s, err := homedashLabResolve(file, id)
		if err != nil || len(s.Panels) != want { t.Errorf("%s: %d panels, %v; want %d", id, len(s.Panels), err, want) }
	}
	th, _ := homedashLabResolve(file, "themes-minimal")
	seen := map[string]bool{}
	for _, p := range th.Panels { seen[p.Query.Get("theme")+"/"+p.Query.Get("scheme")] = true }
	if !seen["fruity/dark"] || !seen["minimal/dark"] || !seen["amiga/light"] { t.Errorf("theme variants %v", seen) }
	c, _ := homedashLabResolve(file, "prim-glass~custom")
	if q := c.Panels[0].Query; q.Get("accent") != "C2410C" || q.Get("hue") != "40" || q.Get("scheme") != "light" { t.Errorf("custom query %v", q) }
	m, _ := homedashLabResolve(file, "motion-type-toggle-glass")
	if len(m.Panels[0].Frames) != 6 || m.Panels[0].Scenario != "on" { t.Errorf("motion panel %+v", m.Panels[0]) }
	if _, err := homedashLabResolve(file, "hello"); err == nil || !strings.Contains(err.Error(), "prim-{family}") { t.Errorf("unknown id error %v", err) }
	if _, err := homedashLabResolve(file, "prim-neon"); err == nil { t.Error("prim-neon must fail") }
	var synth homedashLabFile
	if err := json.Unmarshal([]byte(`{"lists":{"families":["glass","tactile"],"states":{"all":["off","on","alert"],"core":["off","on"],"extra":["alert"]},"heroes":{"H1":{"type":"switch","variant":"pill"}}},
	 "sheets":[{"id":"hero-glass~x","kind":"hero","families":["glass"],"states":["on"]},{"id":"hero-{family}~x","kind":"hero","states":"core+extra","dpr":[1,2],"motion":["full","reduced"]},
	 {"id":"side-{hero}","kind":"side-by-side","families":["glass","tactile"],"palettes":["dark","light"]}]}`), &synth); err != nil { t.Fatal(err) }
	if s, err := homedashLabResolve(synth, "hero-glass~x"); err != nil || len(s.Panels) != 1 || s.Panels[0].Query.Get("states") != "on" { t.Errorf("literal first: %+v %v", s, err) }
	if s, err := homedashLabResolve(synth, "hero-tactile~x"); err != nil || len(s.Panels) != 4 || s.Panels[0].Query.Get("states") != "off,on,alert" || s.Panels[0].Query.Get("types") != "switch" { t.Errorf("pattern: %+v %v", s, err) }
	if s, err := homedashLabResolve(synth, "side-H1"); err != nil || len(s.Panels) != 4 || s.Panels[0].Query.Get("variants") != "pill" { t.Errorf("hero: %+v %v", s, err) }
	if _, err := homedashLabResolve(synth, "side-H9"); err == nil { t.Error("side-H9 must fail") }
	synth.Sheets = append(synth.Sheets, synth.Sheets[0])
	if err := homedashLabValidate(synth); err == nil { t.Error("duplicate id must fail") }
}
```
  The saved `TestHomeDashLabPack` stays. Delete the saved `TestHomeDashLabSheetPlan` and `homedashLabPlan`.
- [ ] Run `go test ./ui -run '^TestHomeDashLab(SheetResolve|Pack)$' -count=1`. Expected: build failure. This test needs no browser.
- [ ] Implement the plan file. Keep the saved `homedashLabPack`, `homedashLabFileNames`, `homedashLabGroups`, `homedashLabRow` and the constants `homedashLabMaxSide = 2400`, `homedashLabWidth = 1180`. Replace everything else with the following (imports: `encoding/json fmt image image/draw net/url os regexp sort strconv strings testing path/filepath`):
```go
type homedashLabThemeRef struct { Theme string `json:"theme"`; Scheme string `json:"scheme"` }
type homedashLabCustom struct { Name string `json:"name"`; Accent string `json:"accent"`; Hue int `json:"hue"`; Scheme string `json:"scheme"` }
type homedashLabHero struct { Type string `json:"type"`; Variant string `json:"variant"` }
type homedashLabSpec struct {
	ID string `json:"id"`; Kind string `json:"kind"`; Families []string `json:"families"`; Types []string `json:"types"`; Variants []string `json:"variants"`
	Palettes []string `json:"palettes"`; Themes string `json:"themes"`; Custom string `json:"custom"`; States json.RawMessage `json:"states"`; Sizes []string `json:"sizes"`
	DPR json.RawMessage `json:"dpr"`; Motion json.RawMessage `json:"motion"`; Backdrop string `json:"backdrop"`; Ease string `json:"ease"`; More *bool `json:"more"`
	Columns int `json:"columns"`; Frames []int `json:"frames"`; Scenarios []string `json:"scenarios"`; Graphics []string `json:"graphics"`; Scripts []string `json:"scripts"`
}
type homedashLabFile struct {
	DefaultSheets []string `json:"defaultSheets"`
	Lists struct {
		Families []string `json:"families"`; States map[string][]string `json:"states"`
		Themes map[string][]homedashLabThemeRef `json:"themes"`; Custom map[string][]homedashLabCustom `json:"custom"`; Heroes map[string]homedashLabHero `json:"heroes"`
	} `json:"lists"`
	Sheets []homedashLabSpec `json:"sheets"`
}
type homedashLabPanel struct { Label string; Query url.Values; DPR float64; Motion string; Frames []int; Scenario string }
type homedashLabSheet struct { ID string; Panels []homedashLabPanel }

var homedashLabKinds = map[string]bool{"prim": true, "type": true, "themes": true, "risk": true, "hero": true, "motion": true, "side-by-side": true}
var homedashLabPlaceholder = regexp.MustCompile(`\{(family|hero|type)\}`)

func homedashLabLoadFile(path string) (homedashLabFile, error) {
	var f homedashLabFile
	data, err := os.ReadFile(path)
	if err != nil { return f, err }
	if err := json.Unmarshal(data, &f); err != nil { return f, fmt.Errorf("%s: %w", path, err) }
	return f, homedashLabValidate(f)
}
func homedashLabValidate(f homedashLabFile) error {
	seen := map[string]bool{}
	for _, s := range f.Sheets {
		if seen[s.ID] { return fmt.Errorf("duplicate sheet id %q", s.ID) }
		seen[s.ID] = true
		if !homedashLabKinds[s.Kind] { return fmt.Errorf("sheet %q: unknown kind %q", s.ID, s.Kind) }
	}
	return nil
}
func homedashLabSheetIDs(env string, f homedashLabFile) []string {
	ids := f.DefaultSheets
	if strings.TrimSpace(env) != "" { ids = strings.Split(env, ",") }
	var out []string; seen := map[string]bool{}
	for _, id := range ids { id = strings.TrimSpace(id); if id != "" && !seen[id] { seen[id] = true; out = append(out, id) } }
	return out
}
func homedashLabPattern(id string) *regexp.Regexp {
	parts, names := homedashLabPlaceholder.Split(id, -1), homedashLabPlaceholder.FindAllStringSubmatch(id, -1)
	var b strings.Builder
	b.WriteString("^")
	for i, p := range parts {
		b.WriteString(regexp.QuoteMeta(p))
		if i < len(names) {
			switch names[i][1] {
			case "family": b.WriteString(`(?P<family>[a-z]+)`)
			case "hero": b.WriteString(`(?P<hero>[A-Za-z0-9]+)`)
			default: b.WriteString(`(?P<type>[a-z][a-zA-Z0-9_]*)`)
			}
		}
	}
	return regexp.MustCompile(b.String() + "$")
}
func homedashLabContains(list []string, v string) bool { for _, x := range list { if x == v { return true } }; return false }
func homedashLabResolve(f homedashLabFile, id string) (homedashLabSheet, error) {
	var order []homedashLabSpec
	for _, s := range f.Sheets { if !homedashLabPlaceholder.MatchString(s.ID) { order = append(order, s) } }
	for _, s := range f.Sheets { if homedashLabPlaceholder.MatchString(s.ID) { order = append(order, s) } }
	var reasons []string
	for _, s := range order {
		re := homedashLabPattern(s.ID)
		m := re.FindStringSubmatch(id)
		if m == nil { continue }
		vars := map[string]string{}
		for i, n := range re.SubexpNames() { if n != "" { vars[n] = m[i] } }
		allowed := s.Families
		if len(allowed) == 0 { allowed = f.Lists.Families }
		if fam, ok := vars["family"]; ok && !homedashLabContains(allowed, fam) { reasons = append(reasons, fmt.Sprintf("%s: family %q not in %v", s.ID, fam, allowed)); continue }
		if h, ok := vars["hero"]; ok { if _, ok := f.Lists.Heroes[h]; !ok { reasons = append(reasons, fmt.Sprintf("%s: unknown hero %q", s.ID, h)); continue } }
		panels, err := homedashLabExpand(f, s, vars)
		if err != nil { return homedashLabSheet{}, fmt.Errorf("sheet %s (%s): %w", id, s.ID, err) }
		return homedashLabSheet{ID: id, Panels: panels}, nil
	}
	var known []string
	for _, s := range f.Sheets { known = append(known, s.ID) }
	reason := ""
	if len(reasons) > 0 { reason = " (" + strings.Join(reasons, "; ") + ")" }
	return homedashLabSheet{}, fmt.Errorf("unknown HomeDash lab sheet id %q%s; known patterns: %s", id, reason, strings.Join(known, ", "))
}
func homedashLabStrings(raw json.RawMessage, def []string) ([]string, error) {
	if len(raw) == 0 { return def, nil }
	var one string
	if json.Unmarshal(raw, &one) == nil { return []string{one}, nil }
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil { return nil, fmt.Errorf("want a string or a list: %s", raw) }
	return many, nil
}
func homedashLabFloats(raw json.RawMessage) ([]float64, error) {
	if len(raw) == 0 { return []float64{1}, nil }
	var one float64
	if json.Unmarshal(raw, &one) == nil { return []float64{one}, nil }
	var many []float64
	if err := json.Unmarshal(raw, &many); err != nil { return nil, fmt.Errorf("dpr: want a number or a list: %s", raw) }
	return many, nil
}
func homedashLabStates(f homedashLabFile, raw json.RawMessage) ([]string, error) {
	all := f.Lists.States["all"]
	if len(raw) == 0 { return all, nil }
	var name string
	if json.Unmarshal(raw, &name) == nil {
		var out []string
		for _, part := range strings.Split(name, "+") {
			set, ok := f.Lists.States[part]
			if !ok { return nil, fmt.Errorf("unknown state set %q", part) }
			out = append(out, set...)
		}
		return out, nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil { return nil, fmt.Errorf("states: %s", raw) }
	for _, s := range list { if len(all) > 0 && !homedashLabContains(all, s) { return nil, fmt.Errorf("unknown state %q", s) } }
	return list, nil
}
type homedashLabLook struct { Theme, Scheme, Accent, Hue string }
func homedashLabLooks(f homedashLabFile, s homedashLabSpec, palette string) ([]homedashLabLook, error) {
	switch palette {
	case "theme":
		name := s.Themes
		if name == "" { name = "standard" }
		list, ok := f.Lists.Themes[name]
		if !ok && name == "standard" { list, ok = []homedashLabThemeRef{{Theme: "standard", Scheme: "dark"}}, true }
		if !ok { return nil, fmt.Errorf("unknown theme list %q", name) }
		var out []homedashLabLook
		for _, r := range list { out = append(out, homedashLabLook{Theme: r.Theme, Scheme: r.Scheme}) }
		return out, nil
	case "custom":
		if s.Custom == "" { return []homedashLabLook{{Scheme: "dark", Accent: "8E5BD9", Hue: "285"}}, nil }
		list, ok := f.Lists.Custom[s.Custom]
		if !ok { return nil, fmt.Errorf("unknown custom list %q", s.Custom) }
		var out []homedashLabLook
		for _, c := range list { out = append(out, homedashLabLook{Scheme: c.Scheme, Accent: strings.TrimPrefix(c.Accent, "#"), Hue: strconv.Itoa(c.Hue)}) }
		return out, nil
	case "light":
		return []homedashLabLook{{Scheme: "light"}}, nil
	case "dark":
		return []homedashLabLook{{Scheme: "dark"}}, nil
	}
	return nil, fmt.Errorf("unknown palette %q", palette)
}
func homedashLabExpand(f homedashLabFile, s homedashLabSpec, vars map[string]string) ([]homedashLabPanel, error) {
	families := s.Families
	if fam := vars["family"]; fam != "" { families = []string{fam} }
	if len(families) == 0 { return nil, fmt.Errorf("no family") }
	types, variants := s.Types, s.Variants
	if t := vars["type"]; t != "" { types = []string{t} }
	if h := vars["hero"]; h != "" { hero := f.Lists.Heroes[h]; types, variants = []string{hero.Type}, []string{hero.Variant} }
	if s.Kind == "hero" && len(types) == 0 {
		keys := make([]string, 0, len(f.Lists.Heroes))
		for k := range f.Lists.Heroes { keys = append(keys, k) }
		sort.Strings(keys)
		for _, k := range keys {
			h := f.Lists.Heroes[k]
			if !homedashLabContains(types, h.Type) { types = append(types, h.Type) }
			if !homedashLabContains(variants, h.Variant) { variants = append(variants, h.Variant) }
		}
	}
	states, err := homedashLabStates(f, s.States)
	if err != nil { return nil, err }
	dprs, err := homedashLabFloats(s.DPR)
	if err != nil { return nil, err }
	motions, err := homedashLabStrings(s.Motion, []string{"full"})
	if err != nil { return nil, err }
	palettes := s.Palettes
	if len(palettes) == 0 { palettes = []string{"dark"} }
	scenarios := []string{""}
	if len(s.Frames) > 0 { scenarios = s.Scenarios; if len(scenarios) == 0 { scenarios = []string{"on"} } }
	var panels []homedashLabPanel
	for _, fam := range families {
		for _, pal := range palettes {
			looks, err := homedashLabLooks(f, s, pal)
			if err != nil { return nil, err }
			for _, look := range looks { for _, dpr := range dprs { for _, mo := range motions { for _, sc := range scenarios {
				v := url.Values{}
				set := func(k, val string) { if val != "" { v.Set(k, val) } }
				set("family", fam); set("palette", pal); set("scheme", look.Scheme); set("theme", look.Theme); set("accent", look.Accent); set("hue", look.Hue)
				set("types", strings.Join(types, ",")); set("variants", strings.Join(variants, ",")); set("states", strings.Join(states, ",")); set("sizes", strings.Join(s.Sizes, ","))
				set("backdrop", s.Backdrop); set("ease", s.Ease); set("scripts", strings.Join(s.Scripts, ","))
				if s.More != nil && !*s.More { v.Set("more", "0") }
				if s.Columns > 0 { v.Set("columns", strconv.Itoa(s.Columns)) }
				if len(s.Graphics) > 0 { g, _ := json.Marshal(s.Graphics); v.Set("graphics", string(g)) }
				v.Set("motion", mo); v.Set("dpr", strconv.FormatFloat(dpr, 'f', -1, 64)); v.Set("lang", "en")
				label := fmt.Sprintf("family=%s palette=%s", fam, pal)
				if look.Theme != "" { label += " theme=" + look.Theme }
				if look.Accent != "" { label += " accent=" + look.Accent }
				label += fmt.Sprintf(" scheme=%s dpr=%g motion=%s", look.Scheme, dpr, mo)
				if sc != "" { label += " scenario=" + sc }
				panels = append(panels, homedashLabPanel{Label: label, Query: v, DPR: dpr, Motion: mo, Frames: s.Frames, Scenario: sc})
			} } } }
		}
	}
	return panels, nil
}
```
- [ ] Run the unit tests. Expected: PASS. `go vet ./ui`.
- [ ] Implement the browser test file following the saved Task 16 contract 1–7, with these changes.
  - **Contract 1:** `homedashLabPanelReport` gets `Extra map[string]json.RawMessage \`json:"extra,omitempty"\``. `homedashLabProbes` gets `Extra map[string]json.RawMessage \`json:"extra"\``; the runner moves it into the panel report. `failures()` adds, for each extra entry: if it is an object whose `ok` is false, append `"extra <key>: " + message` (or the raw JSON when there is no message).
  - **Contract 5 (motion panels):** for each frame, call `open()`, then eval `async(sc,ms)=>{await hdLab.applyState(sc);await hdLab.seek(ms);}` with `panel.Scenario` and `ms`, then `capture()`. Probes are only read for non-motion panels.
  - **Contract 7:** load the file with `homedashLabLoadFile(filepath.Join("testdata","homedash-lab-sheets.json"))` (`t.Fatal` on error). Ids = `homedashLabSheetIDs(os.Getenv("HOMEDASH_LAB_SHEETS"), file)`. Each id goes through `homedashLabResolve` (`t.Fatal` on error). Output and report-only behaviour as saved.
  - Delete the saved `homedashLabPlan`-based plan code.
- [ ] Smoke run as in the saved Task 16, with the prebuilt binary. Expected: PASS. The default sheets PNGs and `probes.json` appear under `reports\homedash-lab\`. Check one PNG with the Read tool. Report theme text failures to the lead instead of weakening the probe.
- [ ] Run `go test ./internal/audit -run '^TestUIBrowserCIShardsCoverEveryTest$' -count=1`. Expected: PASS.
- [ ] Commit `ui/testdata/homedash-lab-sheets.json` and both Go files: `test(homedash): data-defined lab sheets, resolver and control lab browser test`.

## Task 17: DOX
Copy verbatim from the saved file, lines 2182–2199, and add one bullet to the AGENTS.md content list: "the lab fixture registry (`hdLab.fixtures.define`, `report`, scenarios), the sheets file grammar (C5) and the rule that new sheets are data, not Go code".

## Batches and reviews

| Batch | Tasks | Parallel with | Review focus (reader-agent, once per batch; Critical and Important only) |
|---|---|---|---|
| A | 1–7 | B after Task 1 | grid and doc algorithms; solver; no DOM at load |
| B | 8–9 | A | tokens, backdrops, rotation groups; guard: transition properties and backdrop-filter selectors |
| C | 10–13 | – | C2 completeness (dash, 200-unit geometry, buffers only on the knob); host family, tone and status |
| D | 14–16 | – | fixture registry and scenarios; sheets JSON and resolver; lab not shipped |
| E | 17 | – | DOX accuracy |

At the end, test-agent runs:
- `npm run test:homedash`
- `npm run check:ui`
- `go test ./ui -run '^(TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestHomeDashLab.*|TestDesktopAppMaterialsAdoptedByApps)$' -count=1`
- one lab smoke

The lead lands the branch.

## Self-review

**Scope coverage:**
- F0: Tasks 1–6
- V0: Tasks 7–9 (backdrops in 8)
- P0: Tasks 10–13
- V1: Tasks 14–16
- Nothing user-visible changes.

**M1 hero options (R10).** Included in C2 as geometry or structure:
- knob: `start`, `sweep`, `min`/`max` (B maps its `range:[a,b]` to `min`/`max`), `step` (stored for P1), `quantize`, `ticks`, `legends`, `readout` (numeral or seg7), `activeSegment` with `current`, `--hd-angle-cur` and the marker group. B's `kind:'dial'` maps to `variant:'ring'`, and `climate:true` maps to `activeSegment:true`.
- arcGauge: `quantize`, `zones`, `crop`.
- needleGauge: `majors`, `minors`, `numerals` (at most 6, SVG text at r 54), `zones`, `crop`.
- pushButton: `dir` → `data-dir`.
- numeral: `degree` → `span.hd-num-deg`; the decimal split stays in `.hd-num-frac`.
- sparkline: gaps through null, the end dot, `area:false`.
- rocker: legends I/O and `span.hd-rocker-led`.
- toggle: `data-glyph` on/off.

Left for M1 Task 1 (painting decor):
- dial glow, lead, dots and dots-lit paths, `div.hd-dial-hw` (knurl, steel cap), `span.hd-dial-word`
- the needle's glow and shadow duplicate paths, the gauge bezel and face elements
- the arc gauge glow and lead paths and the buffered zone overlay
- `span.hd-seg-rotary`
- the sparkline split fill, grid and halo
- the lamp's `::after` glow

**Risks:**
- Chrome launch from agent binaries: run from `D:\tmp\go-build` with the sandbox off.
- Leftover rod Chromes.
- `./ui` compile time; use the prebuilt binary.
- Theme contrast findings (Amiga accent on card ≈1.9:1) are reported as warnings.
- A transition property given through `var()` is rejected by design.
- Scenario timing depends on two frames.
- `stroke-dasharray` and canvas colour parsing need Chromium 111+.

**Contradictions and decisions:**
1. Load order: `hd-prim-displays` loads before `hd-prim-controls` (numeral and util reuse), unlike frontend 1.6.
2. The needle is a rotated `g` (visual §3) rather than frontend 9.2's `line`.
3. Theme-palette graphic token pairs are warnings, with the Amiga evidence (`desktop-theme-amiga.css:23`, `desktop-materials-amiga.css:26`).
4. The placeholder look is lab-only.
5. The visual 1.5 "∓" is read as darker in both schemes.
6. Badge keeps `data-tone`.
7. The guard cannot enforce "no theme id", because `minimal` is both a theme and a family.
8. M0 no longer defines primitive-based `hero`, `risk`, `motion-{hero}` or `side-by-side` sheets. Those ids belong to M1's widget heroes. M0 keeps `prim-`, `type-`, `themes-` and `motion-type-` and the generic kinds in Go.
9. `stroke-dasharray` is written with `style.setProperty`, the CSSOM equivalent of the decision's `style.strokeDasharray`. It keeps the fake DOM and the guard simple.
10. Host family resolution needs `parent` at host time, because `api.family` must be known in `create`. The lab passes the stage.

## Change log
- **R1** Fixture registry, scenarios, `report`/`extra`, per-sheet scripts. Changed: C4, C6, Tasks 14, 15, 16.
- **R2** Sheets as data (`homedash-lab-sheets.json`), Go resolver and expansion, resolver unit test. Changed: C5, Task 16.
- **R3** URL parameters `backdrop`, `accent`, `hue`, `ease`, `more`, `columns`, `graphics`, `scripts`; theme plus scheme. Changed: C4, Tasks 8 (easing fallback), 14, 16.
- **R4** Single swept value arc (`pathLength=1000`, dasharray through the CSSOM), buffers only for the knob's `activeSegment`, guard permits `stroke-dasharray` transitions and tests it. Changed: C2, Tasks 9, 11, 12.
- **R5** 200-unit viewBox; handle, cap and needle as CSS-rotated groups. Changed: C2, Tasks 8, 11, 12, 14 (lab CSS).
- **R6** Host family resolution, `setTone`, `setStatus`, history series shape. Changed: C3, Task 13.
- **R7** Backdrop rules in `homedash-base.css`. Changed: Task 8, C4, Task 14.
- **R8** backdrop-filter allowed only on `.hd-frame` and `.hd-sheet`. Changed: Task 9.
- **R9** No hook in `scripts/test-ui-regressions.mjs`. Changed: global rules, file structure, Task 9.
- **R10** C2 is the complete authoritative table; hero options included as listed above. Changed: C2, Tasks 11, 12.
- The "Lead amendments" A1–A3 are folded into R9, R4 and R7.