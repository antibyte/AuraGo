# HomeDash Engine - Child DOX Contract

## Purpose

HomeDash is the engine for user-designed smart-home dashboards. This subtree holds the shell-independent browser engine:
a pure core (document, grid, layout, history, colour and format helpers), 20 static primitive builders, a widget registry
with a frame host, the hero widget modules (switch, light, climate, gauge, temperature, sensor), the stylesheet parts that
style them (structure frozen since Gate S, four style families that paint only), the M2 data and view layer, and the test-only
control lab that renders every primitive, widget and hero in every state.

Since M2 the desktop app loads the engine (`ui/js/desktop/apps/homedash.js` and `homedash-home.js` are its host). The engine
runs view-only: it reads dashboards and live entity values from the server (SSE plus PUT subscriptions) and sends no
commands. Commands arrive in M3, the designer in M4, the kiosk page in M6. Visual rules, the frozen token table and the lab
review process are in `documentation/homedash-visual-system.md`. Desktop chrome and window policy belong to
`ui/js/desktop/apps/AGENTS.md`.

## Ownership

- Engine core and widgets: `ui/js/homedash/hd-*.js` and `ui/js/homedash/widgets/hd-w-*.js`.
- M2 data and view layer (all in `ui/js/homedash/`): `hd-tone.js` (`E.tone`), `hd-api.js` (`E.api`), `hd-stream.js` (`E.stream`),
  `hd-store.js` (`E.store`), `hd-catalog.js` (`E.catalog`), `hd-render.js` (`E.render`), `hd-sheet.js` (`E.sheet`),
  `hd-view.js` (`E.view`) and `hd-app.js` (`E.mount`). `hd-format.js` (`E.format.duration`) and `hd-widgets.js` belong to the core.
- Host files are not engine files: `ui/js/desktop/apps/homedash.js`, `homedash-home.js` and `ui/css/desktop-app-homedash.css`
  are owned by `ui/js/desktop/apps/AGENTS.md`. The engine must not reference them.
- Stylesheets: `ui/css/homedash-{tokens,bridge,base,primitives,family-glass,family-tactile,family-minimal,family-instrument,widgets,page}.css`
  and the generated, committed `ui/css/homedash.bundle.css`.
- Locale files: `ui/lang/homedash/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` (16 files, `homedash.` keys only).
  M2 added the `homedash.system.*` names and the M2 keys to all 16.
- Guard and tests: `scripts/test-homedash-ui.mjs`, `scripts/homedash-tests/` (the strict fake DOM `dom.mjs`, the harness
  `harness.mjs`, `live-fakes.mjs`, `css-lint.mjs`, `guard.test.mjs`, `m1.test.mjs` and the M2 tests listed under Verification),
  and `scripts/homedash-freeze-tokens.mjs` (writes the token freeze).
- Test-only control lab (never shipped): `ui/testdata/homedash-lab.html`, `homedash-lab.css`, `homedash-lab.js`,
  `homedash-lab-probes.js`, `homedash-lab-sheets.json`, `homedash-lab/hero-fixtures.js`, `homedash-lab/token-freeze.json`, and the Go tests
  `ui/desktop_homedash_lab_plan_test.go`, `ui/desktop_homedash_control_lab_browser_test.go` and `ui/desktop_homedash_lab_cvd_test.go`.
- M2 shared test data (read by Node tests and the Go fixture, never shipped): `ui/testdata/homedash-m2/starter-dashboard.json`
  and `system-entities.json`.
- Shared files this guide does not own but whose HomeDash entries it relies on: `scripts/build-ui-bundles.js` (`cssBundles`),
  `package.json` (`test:homedash`), `ui/desktop_js_line_budget_test.go` (walks `ui/js/homedash`), and the `lang/homedash` case of
  `TestTranslations_SectionFilesOnlyOwnPrefixes` in `ui/i18n_lint_test.go`.
- Not owned here: the desktop app shell and editor, and all backend code (`internal/homedash`, `internal/server`).

## Local Contracts

### Load order and parts

- `window.HomeDashEngine` is the namespace, written `E` below. Files are classic IIFE scripts that extend it.
- `ENGINE_ORDER` in `scripts/homedash-tests/harness.mjs`: `hd-core.js`, `hd-format.js`, `hd-colour.js`, `hd-grid.js`,
  `hd-layout.js`, `hd-doc.js`, `hd-history.js`, `hd-icons.js`, `hd-geometry.js`, `hd-prim-displays.js`, `hd-prim-controls.js`,
  `hd-widgets.js`, `widgets/hd-w-misc.js`, `widgets/hd-w-climate.js`, `widgets/hd-w-light.js`, `widgets/hd-w-sensor.js`,
  `widgets/hd-w-switch.js`. The hero modules use `E.widgets.heroStatus`, so they load after `hd-widgets.js`.
- `APP_ORDER` (harness) is `ENGINE_ORDER` followed by `hd-tone.js`, `hd-api.js`, `hd-stream.js`, `hd-store.js`, `hd-catalog.js`,
  `hd-render.js`, `hd-sheet.js`, `hd-view.js`, `hd-app.js`. The desktop module-loader entry `homedash` loads the same files in the
  same order, then the host scripts `homedash-home.js` and `homedash.js`. `scripts/homedash-tests/wiring.test.mjs` and
  `TestHomeDashWiring` (`ui/desktop_homedash_wiring_test.go`) pin both lists.
- Parts: `E.core`, `E.el`, `E.svg`, `E.format`, `E.colour`, `E.grid`, `E.layout`, `E.doc`, `E.history`, `E.icons`, `E.geometry`,
  `E.prim` (20 builders plus `E.prim.util`), `E.widgets`, `E.tone`, `E.api`, `E.stream`, `E.store`, `E.catalog`, `E.render`,
  `E.sheet`, `E.view` and `E.mount`.
- No file touches the DOM at load. Every file except `hd-core.js` throws when a part it needs is missing.

### Primitives (`E.prim.<name>(opts)`)

- Each builder returns `{el, set(state), destroy()}`; some add extra fields. `destroy()` is idempotent.
- Common opts: `label` (already translated by the caller), `size` (`m` or `l`, written as `data-hd-size`), `variant`, `lang` (default `en`).
- Builders write only attributes, `textContent` and custom properties. Markup (element, class and attribute names, child order)
  is a contract the family CSS depends on; changing it needs the plan owner's decision. The Task 1 additions to M0 markup
  are listed in `documentation/homedash-visual-system.md`; their defaults leave the M0 output unchanged.
- Common state written by `set()`: `data-hd-status` (`ok`, `unavailable`, `unknown`, `error`), `data-hd-pending`, `data-hd-stale`,
  `data-hd-readonly`, `data-hd-disabled` (as `"true"` or absent), `data-hd-tone` (`ok`, `warn`, `crit`), `data-hd-on`.
- Primitives write `data-hd-on="true"` or omit the attribute.
- Interactive builders: disabled or unavailable sets `aria-disabled="true"`; read-only sets `aria-readonly="true"`
  (plain buttons turn read-only into `aria-disabled`).
- `--hd-value` is a 0-1 fraction with 4 decimals. Angles are degrees from 12 o'clock, clockwise, written as `--hd-angle`.
  The colour token for value arcs is `--hd-value-colour`, not `--hd-value`.
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
- `E.widgets.heroStatus.staleText(api, e)` is shared by the hero modules (see below).

### Tone rule (`E.tone`, decision D5, R7)

`hd-tone.js` is pure and Node-tested (`scripts/homedash-tests/tone.test.mjs`). The rule, verbatim:

> Numeric: `options.warn` and `options.alert` are thresholds. Direction is "falling" only when both are numbers and `warn > alert`; otherwise "rising" (one threshold alone, or `warn <= alert`). Rising: value ≥ alert → `crit`, else value ≥ warn → `warn`. Falling: value ≤ alert → `crit`, else value ≤ warn → `warn`. Boundaries count as crossed; `min`/`max` never change the direction. Binary: a `binary_sensor` with device class `smoke`, `gas`, `moisture`, `problem`, `safety` or `carbon_monoxide` is `crit` while on. Unavailable or unknown entities, non-numeric values and values inside the thresholds have no tone (`null`). The host puts the tone on the entity view (`tone`); widgets paint it (they call `api.setTone(e.tone …)` themselves).

- `E.render` adds the host tone to each entity through `E.tone.of`; widgets never compute thresholds themselves.

### Hero modules (`widgets/hd-w-*.js`)

- `hd-w-switch.js`: `switch` (pill = H1, rocker = H2; push and tile have empty bodies).
- `hd-w-light.js`: `light` (dial = H3; slider, fader and tile empty).
- `hd-w-climate.js`: `climate` (dial = H4; stepper and scale empty).
- `hd-w-sensor.js`: `gauge` (needle = H5, arc = H6; bar and tank empty), `temperature` (numeral = H7; thermometer and comfort empty)
  and `sensor` (sparkline = H8; numeral without degree; icon empty).
- `hd-w-misc.js`: `entity` (the M2 entity tile, see below).
- Each module is an IIFE that registers its types with `E.widgets.register`. Its `create(api)` builds the DOM only with `api.prim.*`,
  `api.icons` and `E.el`/`E.svg`. It never reads layout and never uses `innerHTML`. Family-dependent options come from `api.family`.
- A non-hero variant returns an empty body and calls `api.setStatus(null)`.
- `update(view)` reads `view.entities.primary`. It writes only primitive `set()` keys, `api.setTone(...)`, `api.setStatus(...)`,
  `data-*` attributes and `textContent` on its own elements. Tone is `e.tone`, else the zone tone of a gauge.
- Read-only is not disabled. A read-only entity keeps full strength with the lock badge; `disabled` is set only when the entity
  is not read-only and the capability is missing.
- Unavailable, unknown and pending entities leave the state line to the host (`setStatus(null)`). Stale ages come from
  `heroStatus.staleText` and are whole minutes.
- `resize(size)` in the sensor module needs `size.px` (`{width, height}`); a resize without `px` is ignored.
- Options read by the modules: `options.decimals` (SCR-M2-3, integer 0-3) sets the readout precision of gauges and sensors;
  any other value keeps 0. `options.hours` sets the sensor history range (the module asks for `24h` without it; the starter sets `hours: 1`).

### Data layer (M2): API, stream, store, catalog

- **API (`hd-api.js`).** The only file that knows the HomeDash routes. Routes used: `bootstrap`, `widget-catalog` (with `ETag`,
  304), `history`, `stream`, `stream/subscriptions` (PUT), `dashboards`, `dashboard` (GET), `dashboards` POST (`{template:"system", name}`
  or `{document}`) and `dashboard` DELETE (`revision`). The M2 client does not call `entities`, `entity`, `snapshot`, `commands`,
  `dashboards/validate`, PUT `dashboard`, `duplicate`, `revisions`, `restore` or `approve`.
- Every failure is an Error with `hdApiError`, `status`, `code`, `message`, `data` and `retryAfter`. The code is the body's `code`,
  else its `error`, else a transport word (`aborted`, `not_modified`, `network`, `http_<status>`).
- Codes acted on: `homedash_disabled`, `storage_unavailable`, `desktop_readonly`, `dashboard_not_found`, `limit_exceeded`,
  `revision_conflict`, `stream_not_found`, `unauthorized`.
- **View-only api.** The `api` factory given to widgets returns `can` false for every capability, `command` rejects with
  `view_only`, and `history` honours the widget's `hours`.
- **Stream (`hd-stream.js`).** One SSE connection per JS realm. The union of the active owners' ids, cut to `hello.limits.entities`,
  goes out with PUT `stream/subscriptions` 250 ms after the last change and after every `hello`.
  - Silence for max(45 s, 3 × heartbeat) or an EventSource error closes the source; the client reconnects itself (after 1, 2, 4, 8,
    16, 30 s, ±20 % jitter), never through the browser's own retry, because the stream id changes.
  - A connection that fails before `hello` waits at least 5 s (a 429 `too_many_streams` looks like any other refusal) and calls
    `diagnose()` (bootstrap) to find `homedash_disabled` or 401. The backoff resets after a stream lived 60 s.
  - `revoked` with `shutdown` reconnects; `disabled` and `auth_lost` stop until `resume()`. No active owner for 10 s closes the stream.
  - PUT `stream/subscriptions` answers accepted and rejected ids; a 404 `stream_not_found` is handled by `hd-stream` `put`, and rejected ids reach the store's `subscribed` handler.
- **Store (`hd-store.js`).** One per realm (`shared()`, keyed by the API base path). Rules:
  1. A snapshot is the truth for the ids it lists (the contract's snapshot covers newly added ids only).
  2. The server's per-entity `stale` flag is the truth for value age. A source status of `degraded` does not mark anything stale by itself.
     Connection loss (connecting or down for 5 s, or stopped) marks every entry stale, cleared per id by its next snapshot or state.
     A source status of `unavailable` takes that source's entities offline (`available` false, status `unavailable`); the last value
     is kept and returns when the source leaves `unavailable`. Only available entities are stale.
  3. `readonly` means a control that could act is blocked. It applies only to entities with a command capability (not `stream`,
     `snapshot`, `history` or `numeric`) that the server marks read-only or that sit on a read-only desktop. A pure display
     (all `system` sensors) is never drawn locked. `serverReadonly` keeps the raw server value.
  4. `removed` and PUT-rejected ids become status `unknown`.
  5. History `{origin, points:[{t, v}]}` becomes the host series `{points: [[t_ms, value|null]], source: origin}`. Cache key
     `id|span|points`, TTL 60 s under 6 h spans and 5 min otherwise, LRU of 100, at most 4 requests at a time. Live numeric states
     append to every cached series of the id (SCR-M2-2 draws the live tail).
  6. Ids a window stops showing stay subscribed for 30 s (linger).
  7. `dispose()` is idempotent: it unhooks the stream, detaches every watcher's stream handle and drops its linger.
     `shared()` replaces a disposed instance.
  - Change notices are batched into one animation frame. `dashboard_changed` reaches the host as a `'dashboards'` event.
- **Catalog (`hd-catalog.js`).** Wraps the server catalog: default size and constraints per type and variant, option defaults
  merged under a widget's own options. `registryMismatches(E.widgets)` requires every registered type to exist, its variants to
  equal the catalog's and every required binding to be implemented. The client never copies catalog numbers.

### Realm sharing and mount (`hd-app.js`, `E.mount`)

- Windows of one JS realm share one API client, one stream and one store per base path. The host passes `visibility`
  (`visible()`, `subscribe(fn)`); a window owns its entity ids only while visible (owners with visibility).
- `E.mount(host, options)` loads bootstrap and the widget catalog, lists the dashboards, creates the starter (`template: "system"`)
  or an empty dashboard (`document`), deletes one, opens one dashboard in an `hd-view` and keeps its entities subscribed while
  visible. It refetches the list 250 ms after `dashboard_changed` (once per realm) and reconciles the open dashboard by revision.
- The host draws its chrome from the frozen `state` object and the `E.view.banners` list. `homedash_disabled` closes the view and
  sets mode `disabled`.

### View (`hd-view.js`, `hd-render.js`)

- `E.view.create(opts)` owns the content of one `.hd-root`: root attributes from the document look and host (`data-hd-family`,
  `data-hd-palette`, `data-hd-scheme`, `data-hd-density`, `data-hd-backdrop`, `data-hd-motion`), `data-hd-conn`, `data-hd-readonly`,
  `data-hd-bp` and `data-hd-fit`, page tabs, the scroll stage with one `hd-render` page, and the live region.
- `E.widgets.applyLook(root)` runs after every render and look change. It owns the inline values of `--hd-accent`,
  `--hd-light-on`, `--hd-focus`, `--hd-ink-on-accent` and `--hd-hud`, and the `data-hd-corners` attribute. The view never writes or clears
  those five names. A custom palette's solved accent and its ink-on-accent go through `--hd-n-accent` and `--hd-n-ink-on-accent`
  (the bridge maps them; L1 refinement); `--hd-focus` follows `--hd-accent`.
- `E.view.banners(state)` is a pure list of at most two banners by priority. The host draws them in its own chrome. Kinds:
  `auth_lost`, `offline`, `source_unavailable`, `source_degraded`, `schema_too_new`, `readonly`.
- `view.readonly` follows rule 3 of the store (controllable entities only). `hd-render` renders pages with `can()` false and
  `command()` rejecting. Unregistered types show a labelled `.hd-unsupported` cell.
- Layout runs at creation and then once per animation frame per stage resize. Store notices update only bound widgets;
  off-screen widgets update when shown.
- **Entity tile (`hd-w-misc.js`, type `entity`).** Duration sensors (`device_class` `duration`) read through `E.format.duration`
  instead of raw numbers. A known `icon_hint` names the badge glyph; otherwise the widget's icon or `entity`.
- **Duration format (`E.format.duration(seconds, lang)`).** The largest non-zero unit plus only the next smaller unit, the smaller
  one omitted when zero: 7215 s reads "2h", never "2h 15s". Negative or non-finite input gives the en dash.
- **System entity names.** The client localizes the six built-in `system` entities through `homedash.system.<native>` when a widget
  has no title. The server contract keeps English names, so other clients see English.

### Stylesheet bundle and structure

- Ten parts, in this order: `tokens`, `bridge`, `base`, `primitives`, the four family files (`glass`, `tactile`, `minimal`,
  `instrument`), `widgets`, and `page` last (`homedash-page.css`, part 10: the scroll stage, the page placed from
  `--hd-x`/`--hd-y`/`--hd-w`/`--hd-h`, the metrics `--hd-cols`/`--hd-rows`/`--hd-colw`/`--hd-rowh`/`--hd-gap`/`--hd-offx`, the page tabs,
  the empty-page note and the unsupported cell). The `cssBundles` entry in `scripts/build-ui-bundles.js` defines the order and output.
- Each part is at most 900 lines (`css-lint.mjs`). After editing any part, run `npm run build:ui` and commit the regenerated
  `ui/css/homedash.bundle.css` with it.
- The desktop app loads the bundle and `desktop-app-homedash.css` (host chrome) through its app style entry.
- Structure is frozen since Gate S for the M1 parts. M2 changed structure only through recorded requests: SCR-M2-1 (entity tile
  geometry and duration text, including the numeral top-aligned with the sensor numerals, the glyph bottom-left at the card inset,
  stale ink and the Instrument HUD paint), SCR-M2-2 (the sparkline live tail after its history arrives) and SCR-M2-3 (gauge and
  sensor readouts honour `options.decimals`). The rulings are in `documentation/homedash-visual-system.md`. The token freeze is unchanged in M2.

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
- Classes use the `hd-` prefix. `@font-face` families start with `HomeDash `. HomeDash declares only Barlow Condensed and Share Tech Mono;
  the other families come from `/fonts/fonts.css`.

### Style families and the frozen structure

- Structure is frozen since Gate S: `hd-prim-*.js`, `hd-widgets.js`, `widgets/hd-w-*.js`, `homedash-base.css`,
  `homedash-primitives.css` and `homedash-widgets.css`. The M2 changes listed above are the recorded exceptions.
- Family files (`homedash-family-*.css`) paint only: colours, shadows, gradients, masks, paint radii and motion, through `var()` and
  `color-mix()`. The Instrument family paints the entity tile's numeral and glyph in the HUD readout colour (`--hd-i-val`).
- `data-hd-family` may sit on `.hd-root` (the view sets it). The bare `[data-hd-family]` token blocks on the root only declare family-scoped tokens and non-colour scale tokens that only widgets read; colour and font tokens are scoped to `.hd-widget[data-hd-family]` (B-1).
- Token freeze: `ui/testdata/homedash-lab/token-freeze.json` records every all-`--hd-*` rule of the six token and family
  parts (45 selectors). `scripts/homedash-tests/m1.test.mjs` fails on any difference. A token change needs the lead's approval,
  then `node scripts/homedash-freeze-tokens.mjs`, then `npm run test:homedash`. Do not edit the JSON by hand.
- The Glass zero-value rule `@container style(--hd-value: 0.0000)` (`homedash-family-glass.css`) is verified: every element is a style container for style queries and `--hd-value` inherits to the arc paths. Task 20 checked it with a temporary red probe.
- `E.colour.hudFor`, `E.colour.ensureContrast` and `E.widgets.applyLook` are the look helpers (see the view section for the owned names).

### Translations

- Engine files call `t('homedash.…')` with full literal keys (for example `t('homedash.state.no_data')`). The keys live in
  `ui/lang/homedash/<locale>.json` for all 16 locales, with real translations; German uses "Du" and real umlauts.
- Two call sites build a key from a fixed set: `F.STATE_PREFIX + word` in `hd-widgets.js` (`homedash.state.` plus one of
  `unavailable`, `unknown`, `error`, `pending`), and the climate action key in `widgets/hd-w-climate.js` (one of `heating_to`,
  `cooling_to`, `idle`). Those names must exist in `en.json` and in all other locales.
- `scripts/homedash-tests/i18n.test.mjs` (L5): every locale has exactly the English key set; no non-English value equals its English
  value unless the key is on the explicit allowlist (brand names, units and symbols such as `HomeDash`, `CPU`, `Mbit/s`); every
  allowlist entry names an existing key and really equals the English value.
- Checks: `TestFrontend_StaticI18nKeysExistInEnglishBundle` (literal keys against the English bundle) and the `lang/homedash`
  case of `TestTranslations_SectionFilesOnlyOwnPrefixes` (prefix `homedash.`), both in `ui/i18n_lint_test.go`.

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
  The M1 hero fixtures are in `ui/testdata/homedash-lab/hero-fixtures.js`; they also run the `m1-tabular` and `m1-svgcount` extra probes.
  The hero fixtures do not exercise `options.decimals`.
- Sheets file `ui/testdata/homedash-lab-sheets.json`:
  - Top-level keys `defaultSheets`, `lists` (`families`, `states`, `themes`, `custom`, `heroes`) and `sheets`.
  - A sheet id is a pattern with `{family}`, `{hero}` or `{type}` placeholders and an optional `~variant` suffix.
  - Literal ids resolve before patterns; patterns resolve in file order. A placeholder value must belong to its list; an unknown id
    fails and lists the known patterns.
  - One panel per combination of family, palette, look (theme or custom entry), dpr, motion and scenario.
  - The sheet ids in use are listed in `documentation/homedash-visual-system.md`.
- New sheets are data: add them to `homedash-lab-sheets.json`, not to Go code.
- Output goes to the repository-root `reports/homedash-lab/`: `<id>.png`, or `<id>-NN.png` when split (at most 2400 device px per
  side), and `probes.json` with `generated`, `report_only`, `sheets`, `failures`. The folder is git-ignored.
- Environment: `HOMEDASH_LAB_SHEETS` (comma list of sheet ids; empty means `defaultSheets`), `HOMEDASH_LAB_REPORT_ONLY=1`
  (report and log probe failures without failing the test; never used to pass a gate) and `HOMEDASH_LAB_CVD=1` (colour-vision and greyscale
  copies, `TestHomeDashLabColourVisionSheets`).
- Probes: contrast for tokens, text and graphics; hit size 44x44; layout shift; animations under reduced motion; backdrop count;
  token and font resolution; hero extras `m1-tabular` and `m1-svgcount`. `hdLab.report` values appear as `extra`; a value with
  `ok === false` is a failure that shows its `message`. Glass gradients are measured stop by stop, `url()` wallpapers are measured
  against the resolved `--hd-canvas`, SVG graphics start at their own svg, and visually hidden text is skipped. Exact rules and
  what the probes cannot see: `documentation/homedash-visual-system.md`.
- Never weaken a probe threshold to make a sheet pass. A real failure is fixed or reported, not hidden by `HOMEDASH_LAB_REPORT_ONLY=1`.

## Work Guidance

- Keep JS modules under 1100 lines (the line budget test fails at 1100) and aim for under 700 (the guard warns above 700).
- The fake DOM in `scripts/homedash-tests/dom.mjs` is strict: it throws on selector syntax it does not implement. Extend it
  properly when a test needs more; do not loosen it.
- Write non-ASCII characters in JS sources with `String.fromCodePoint`.
- Tests compare path strings from `E.geometry`, so keep its number format stable.
- The guard has known limits. It does not see a lone `transition-duration`, a `-webkit-transition`, or a blur given through
  `var()` (it measures only literal px blur). Authors follow the styling rules themselves; the guard is a backstop.
- Store and view changes: keep the store rules in the header of `hd-store.js` current with this guide; the header is the truth.
- Review, gate and sheet protocol for visual work: `documentation/homedash-visual-system.md`. Judge only fresh sheets.

## Verification

- `npm run test:homedash`: the Node guard, the engine tests, the M1 token freeze in `m1.test.mjs`, and the M2 tests
  `api`, `tone`, `stream`, `store`, `catalog`, `i18n` (L5), `scr`, `render`, `sheet`, `view`, `app`, `home`, `host` and `wiring`,
  plus the existing `format` and `bundle` tests (`scripts/homedash-tests/*.test.mjs`; helpers `harness.mjs`, `live-fakes.mjs`, `dom.mjs`).
- `npm run check:ui` (bundle drift check, `scripts/build-ui-bundles.js --check`).
- `go test ./ui -run '(?i)^TestHomeDashWiring$' -count=1` (`ui/desktop_homedash_wiring_test.go`: load order and registration).
- `go test ./ui -run '^(TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestHomeDashLab.*)$' -count=1`.
- `go test ./ui -run '(?i)^Test(Translations_|Frontend_StaticI18nKeysExistInEnglishBundle|HomeDashLab)' -count=1` (translation keys and lab plan tests).
- `go test ./internal/desktop -count=1` (`BuiltinApps` row, `internal/desktop/homedash_app_test.go`).
- M2 browser test `TestDesktopHomeDashBrowser` (`ui/desktop_homedash_browser_test.go`): starter, live values, sparkline history, empty dashboard,
  delete confirm, read-only desktop, reconnect after `shutdown` and disabled state after `disabled`. It needs Chrome and
  `AURAGO_RUN_BROWSER_SMOKE=1`. Build the test binary from the repository root, then run it from the `ui` directory with the shell
  sandbox off; quote the flags:

```powershell
go test -c -o D:\tmp\go-build\hd-m2-ui.test.exe ./ui
Set-Location ui
$env:AURAGO_RUN_BROWSER_SMOKE='1'; & D:\tmp\go-build\hd-m2-ui.test.exe '-test.run=^TestDesktopHomeDashBrowser$' '-test.v' '-test.timeout=30m'
```

  `$env:HOMEDASH_M2_SHOTS='1'` also writes the review screenshots to the repository-root `reports/homedash-m2/` (git-ignored).
  The test takes about 30 to 55 seconds.
- Lab browser: needs Chrome and `AURAGO_RUN_BROWSER_SMOKE=1`; writes the sheet images and `probes.json` to `reports/homedash-lab/`.
  Build the test binary from the repository root, then run it from the `ui` directory; quote the flags as shown (PowerShell
  splits an unquoted `-test.run` at the dot). On this project's Windows machines Chrome only starts when the binary lives
  outside the repository (for example under `D:\tmp\go-build`) and the shell sandbox is off:

```powershell
go test -c -o D:\tmp\go-build\hdlab-ui.test.exe ./ui
$env:AURAGO_RUN_BROWSER_SMOKE='1'; & D:\tmp\go-build\hdlab-ui.test.exe '-test.run=^TestDesktopHomeDashControlLabBrowser$' '-test.v' '-test.timeout=30m'
```

- Colour-vision and greyscale copies (after a lab run, from the repository root): `$env:HOMEDASH_LAB_CVD='1'; go test ./ui -run '^TestHomeDashLabColourVisionSheets$' -count=1`.
- After adding or renaming a browser test, run `go test ./internal/audit -run '^TestUIBrowserCIShardsCoverEveryTest$' -count=1`.

## Child DOX Index

None.
