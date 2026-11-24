# HomeDash Engine - Child DOX Contract

## Purpose

HomeDash is the engine for user-designed smart-home dashboards. This subtree holds the shell-independent browser engine:
a pure core (document, grid, layout, history, colour and format helpers), 20 static primitive builders, a widget registry
with a frame host, the stylesheet parts that style them, and the test-only control lab that renders every primitive and
widget in every state. No desktop app is registered yet, so nothing in the desktop shell loads the engine or the bundle.

## Ownership

- Engine: `ui/js/homedash/hd-*.js` and `ui/js/homedash/widgets/hd-w-misc.js`.
- Stylesheets: `ui/css/homedash-{tokens,bridge,base,primitives,family-glass,family-tactile,family-minimal,family-instrument,widgets}.css`
  and the generated, committed `ui/css/homedash.bundle.css`.
- Guard: `scripts/test-homedash-ui.mjs`, `scripts/homedash-tests/` (including the strict fake DOM `dom.mjs`, the harness
  `harness.mjs`, `css-lint.mjs` and `guard.test.mjs`).
- Test-only control lab (never shipped): `ui/testdata/homedash-lab.html`, `homedash-lab.css`, `homedash-lab.js`,
  `homedash-lab-probes.js`, `homedash-lab-sheets.json`, and the Go tests `ui/desktop_homedash_lab_plan_test.go` and
  `ui/desktop_homedash_control_lab_browser_test.go`.
- Shared files this guide does not own but whose HomeDash entries it relies on: `scripts/build-ui-bundles.js` (`cssBundles`),
  `package.json` (`test:homedash`), `ui/desktop_js_line_budget_test.go` (walks `ui/js/homedash`).
- Not owned here: the desktop app shell and editor, locale files (`ui/lang/homedash` does not exist yet), and all backend code.

## Local Contracts

### Engine parts and load order

- `window.HomeDashEngine` is the namespace, written `E` below. Files are classic IIFE scripts that extend it.
- Load order (`ENGINE_ORDER` in `scripts/homedash-tests/harness.mjs`): `hd-core.js`, `hd-format.js`, `hd-colour.js`, `hd-grid.js`,
  `hd-layout.js`, `hd-doc.js`, `hd-history.js`, `hd-icons.js`, `hd-geometry.js`, `hd-prim-displays.js`, `hd-prim-controls.js`,
  `hd-widgets.js`, `widgets/hd-w-misc.js`.
- Parts: `E.core`, `E.el`, `E.svg`, `E.format`, `E.colour`, `E.grid`, `E.layout`, `E.doc`, `E.history`, `E.icons`, `E.geometry`,
  `E.prim` (20 builders plus `E.prim.util`) and `E.widgets`.
- No file touches the DOM at load. Every file except `hd-core.js` throws when a part it needs is missing.

### Primitives (`E.prim.<name>(opts)`)

- Each builder returns `{el, set(state), destroy()}`; some add extra fields. `destroy()` is idempotent.
- Common opts: `label` (already translated by the caller), `size` (`m` or `l`, written as `data-hd-size`), `variant`, `lang` (default `en`).
- Builders write only attributes, `textContent` and custom properties. Markup (element, class and attribute names, child order)
  is a contract the family CSS depends on; changing it needs the plan owner's decision.
- Common state written by `set()`: `data-hd-status` (`ok`, `unavailable`, `unknown`, `error`), `data-hd-pending`, `data-hd-stale`,
  `data-hd-readonly`, `data-hd-disabled` (as `"true"` or absent), `data-hd-tone` (`ok`, `warn`, `crit`), `data-hd-on`.
- Primitives write `data-hd-on="true"` or omit the attribute.
- Interactive builders: disabled or unavailable sets `aria-disabled="true"`; read-only sets `aria-readonly="true"`
  (plain buttons turn read-only into `aria-disabled`).
- `--hd-value` is a 0-1 fraction with 4 decimals. Angles are degrees from 12 o'clock, clockwise, written as `--hd-angle`.
- Dials and gauges use `viewBox="0 0 200 200"` (a gauge `crop` option shortens the height). The value arc is one path
  with `pathLength=1000`; `set()` writes `stroke-dasharray` through the CSSOM. Handle, cap and needle groups are rotated by CSS
  from `--hd-angle` and `--hd-angle-cur`; JS never moves their circles.

### Widget host (`E.widgets`)

- `E.widgets.register(type, {bindings, variants, create(api)})`, `E.widgets.get(type)`, `E.widgets.types()`.
- `E.widgets.host({widget, mode, size, t, lang, api, moreLabel, parent})` returns `{el, body, family, update(), resize(size), setMode(m), destroy()}`.
  `api` is a factory `({widget, mode, signal})` returning `{entity, can, command, moreInfo, history, announce}`.
- Family resolution: `widget.family`, else the value of the nearest `[data-hd-family]` ancestor of `parent`, else `glass`.
  It is always written to `data-hd-family` on `.hd-widget`. When `parent` is given, `el` is appended to it before `create` runs.
- The frozen `api` given to `create` contains `body`, `widget`, `variant`, `family`, `mode` (getter), `t`, `fmt`, `prim`, `icons`,
  `signal`, `lang`, the six factory functions, `setTone(tone|null)` (overrides the view tone) and `setStatus(text|null)`
  (state-line text; `null` restores the default).
- `history(binding, range)` resolves to `{points: [[t_ms, value|null], ...], source}`.
- State line precedence: the words for unavailable, unknown, error and pending; then the widget status (`setStatus`);
  then `E.format.stateText`. With no primary entity the line shows the `setStatus` text or stays empty, and `data-hd-status` is `ok`.
- The wrapper `div.hd-widget` carries `data-hd-type`, `data-hd-variant`, `data-hd-family`, `data-hd-size`, `data-hd-status`
  and `data-hd-on` as `"true"`, `"false"` or absent (absent for a non-binary entity), plus the optional pending, stale,
  readonly and tone attributes. It has `role="group"` and `aria-labelledby`.
- Modes: `design` makes the frame body `inert` and the wrapper focusable (`tabindex=0`); `thumb` makes the whole frame `inert`
  and the wrapper `aria-hidden`; `view` removes both. After `destroy()` the host ignores `update`, `resize` and `setMode`.
  If the api factory or `create` throws, the host removes the wrapper, destroys the frame and aborts the signal before rethrowing.

### Styling rules (enforced by `scripts/homedash-tests/css-lint.mjs` and `guard.test.mjs`)

- Styles go through the CSSOM (`style.setProperty`). No `innerHTML`, `outerHTML`, `insertAdjacentHTML` or `setAttribute('style'`.
- No `light-dark()` (select scheme values with `[data-hd-scheme]`); no `[data-theme]` selectors (read the bridge tokens instead);
  `!important` only in `[hidden]` rules; no `cc-` class prefix; parts declare only `--hd-*` custom properties (reading `--vd-mat-*`
  and `--ds-*` through `var()` belongs in `homedash-bridge.css`); `url()` only for `/fonts/...` or small inline SVG data URLs;
  no `text-transform: uppercase`.
- Literal colours appear only in token blocks.
- Motion appears only inside `@media (prefers-reduced-motion: no-preference)` on selectors that contain
  `.hd-root:not([data-hd-motion="reduced"]):not([data-hd-motion="none"])`.
- Transitions name only `transform`, `opacity` or `stroke-dasharray`, with the property first in the shorthand
  (`transition: opacity 200ms`). Keyframes animate only `transform` and `opacity`.
- `backdrop-filter` appears only on `.hd-frame` and `.hd-sheet`, with blur at most 24px.
- Derived tokens (`--hd-focus`, `--hd-accent-soft`, `--hd-surface`, `--hd-track`, `--hd-*-ink`, ...) resolve on `.hd-root`
  in `homedash-bridge.css`. A family or widget block that overrides a base token (`--hd-accent`, `--hd-on`, `--hd-ink`,
  `--hd-surface-solid`, ...) re-declares the derived tokens it relies on.
- Classes use the `hd-` prefix. `@font-face` families start with `HomeDash `.

### Stylesheet bundle

- Nine parts, in this order: `tokens`, `bridge`, `base`, `primitives`, the four family files (`glass`, `tactile`, `minimal`,
  `instrument`), `widgets`. The `cssBundles` entry in `scripts/build-ui-bundles.js` defines the order and the output.
- Each part is at most 900 lines (`css-lint.mjs`).
- After editing any part, run `npm run build:ui` and commit the regenerated `ui/css/homedash.bundle.css` with it.

### Translations

- Until `ui/lang/homedash` exists, do not call the translator with a string literal (`t('...')`) in engine files. The static key
  test `TestFrontend_StaticI18nKeysExistInEnglishBundle` in `ui/i18n_lint_test.go` checks literal keys against the English bundle.
- Build state words as `t(STATE_PREFIX + word)`.

### Control lab (test-only, never shipped)

- `assets/web-assets.json` lists no `testdata` directory; the lab is served only by the test harness.
- With `palette=theme` the lab loads `/css/desktop-shell.bundle.css` and makes `#hd-lab-scope` a stand-in for the desktop body:
  class `desktop-body` plus `data-theme`, `data-scheme`, `data-fruity-mode`, `data-accent` and `data-animations`. Keep this set
  equal to what the desktop shell writes on its body; a missing attribute lets a theme fall back to another scheme's colours.
- The browser test emulates `prefers-reduced-motion` and `prefers-color-scheme` per panel, so sheets do not depend on the
  host's settings.
- URL: `/testdata/homedash-lab.html?...`. Parameters: `family`, `palette`, `scheme`, `theme`, `accent`, `hue`, `backdrop`, `ease`,
  `more`, `types`, `variants`, `states`, `sizes`, `dpr`, `motion`, `lang`, `columns`, `graphics`, `scripts`. Values and defaults:
  the header of `ui/testdata/homedash-lab.js` and contract C4 of `docs/superpowers/plans/2026-11-23-homedash-m0-frontend.md`.
- `window.hdLab` members: `ready`, `error`, `errors`, `fixtures.define(type, variant, entry)`, `fixtures.get(type, variant)`,
  `report(key, value)`, `layout()`, `probes()` (Promise), `applyState(nameOrScenario)` (Promise), `seek(ms)`.
- Fixture entries are `{widget?, states?, scenarios?, history?}`. A variant of `*` matches any variant. A scenario is `{from, to}`:
  `applyState` applies `from` with motion forced to `none`, waits two frames, restores motion and applies `to`.
- Sheets file `ui/testdata/homedash-lab-sheets.json`:
  - Top-level keys `defaultSheets`, `lists` (`families`, `states`, `themes`, `custom`, `heroes`) and `sheets`.
  - A sheet id is a pattern with `{family}`, `{hero}` or `{type}` placeholders and an optional `~variant` suffix.
  - Literal ids resolve before patterns; patterns resolve in file order. A placeholder value must belong to its list; an unknown id
    fails and lists the known patterns.
  - One panel per combination of family, palette, look (theme or custom entry), dpr, motion and scenario.
- New sheets are data: add them to `homedash-lab-sheets.json`, not to Go code.
- Output goes to the repository-root `reports/homedash-lab/`: `<id>.png`, or `<id>-NN.png` when split (at most 2400 device px per
  side), and `probes.json` with `generated`, `report_only`, `sheets`, `failures`.
- Environment: `HOMEDASH_LAB_SHEETS` (comma list of sheet ids; empty means `defaultSheets`) and `HOMEDASH_LAB_REPORT_ONLY=1`
  (report and log probe failures without failing the test).
- Probes: contrast for tokens, text and graphics (the graphics selector list plus the panel's `graphics`); hit size 44x44;
  layout shift; animations under reduced motion; backdrop count; token and font resolution. `hdLab.report` values appear
  as `extra`; a value with `ok === false` is a failure that shows its `message`.
- Never weaken a probe threshold to make a sheet pass. A real failure is fixed or reported, not hidden by `HOMEDASH_LAB_REPORT_ONLY=1`.

## Work Guidance

- Keep JS modules under 1100 lines (the line budget test fails at 1100) and aim for under 700 (the guard warns above 700).
- The fake DOM in `scripts/homedash-tests/dom.mjs` is strict: it throws on selector syntax it does not implement. Extend it
  properly when a test needs more; do not loosen it.
- Write non-ASCII characters in JS sources with `String.fromCodePoint`.
- Tests compare path strings from `E.geometry`, so keep its number format stable.
- The guard has known limits. It does not see a lone `transition-duration`, a `-webkit-transition`, or a blur given through
  `var()` (it measures only literal px blur). Authors follow the styling rules themselves; the guard is a backstop.

## Verification

- `npm run test:homedash` (Node guard and engine tests).
- `npm run check:ui` (bundle drift check, `scripts/build-ui-bundles.js --check`).
- `go test ./ui -run '^(TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestHomeDashLab.*)$' -count=1`.
- Lab browser smoke: needs Chrome and `AURAGO_RUN_BROWSER_SMOKE=1`; writes the sheet images and `probes.json` to `reports/homedash-lab/`.
  Build the test binary from the repository root, then run it from the `ui` directory; quote the flags as shown (PowerShell
  splits an unquoted `-test.run` at the dot). On this project's Windows machines Chrome only starts when the binary lives
  outside the repository (for example under `D:\tmp\go-build`) and the shell sandbox is off:

```powershell
go test -c -o D:\tmp\go-build\hdlab-ui.test.exe ./ui
$env:AURAGO_RUN_BROWSER_SMOKE='1'; & D:\tmp\go-build\hdlab-ui.test.exe '-test.run=^TestDesktopHomeDashControlLabBrowser$' '-test.v' '-test.timeout=30m'
```

- After adding or renaming a browser test, run `go test ./internal/audit -run '^TestUIBrowserCIShardsCoverEveryTest$' -count=1`.

## Child DOX Index

None.
