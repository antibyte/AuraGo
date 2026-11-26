# HomeDash Engine - Child DOX Contract

## Purpose

HomeDash is the engine for user-designed smart-home dashboards. This subtree holds the shell-independent browser engine:
a pure core (document, grid, layout, history, colour and format helpers), the static primitive builders (M0's 20 plus `picker`
from M3), a widget registry with a frame host, the hero widget modules (switch, light, climate, gauge, temperature, sensor), the
W1 control modules (action, number, select, and the switch and light variants), the stylesheet parts that style them (structure
frozen since Gate S, four style families that paint only), the M2 data and view layer, the M3 command and input layer, and the
test-only control lab that renders every primitive, widget and hero in every state.

Since M2 the desktop app loads the engine (`ui/js/desktop/apps/homedash.js` and `homedash-home.js` are its host). Since M3 the
engine sends commands for the W1 controls (see "Command and input layer" below). The designer comes in M4 and the kiosk page in M6. Visual rules, the frozen token table and the lab
review process are in `documentation/homedash-visual-system.md`. Desktop chrome and window policy belong to
`ui/js/desktop/apps/AGENTS.md`.

## Ownership

- Engine core and widgets: `ui/js/homedash/hd-*.js` and `ui/js/homedash/widgets/hd-w-*.js`.
- M3 control and input layer (all in `ui/js/homedash/`): `hd-input.js` (`E.input`), `hd-control.js` (`E.control`), `hd-popover.js`
  (`E.popover`), `hd-moreinfo.js` (`E.moreinfo`), `hd-command.js` (`E.command`, the one command client per JS realm) and the widget
  modules `widgets/hd-w-action.js`, `widgets/hd-w-number.js` and `widgets/hd-w-select.js`. `hd-prim-controls.js` holds the control
  primitives with their `input` contract. `widgets/hd-w-switch.js` and `widgets/hd-w-light.js` gained their W1 variants.
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
- M4 designer (all in `ui/js/homedash/`): the new files `hd-clipboard.js`, `hd-commands.js`, `hd-save.js`, `hd-gestures.js`,
  `hd-editbar.js`, `hd-designer.js` and the part-2 files `hd-form.js`, `hd-samples.js`, `hd-palette.js`, `hd-entity-picker.js`,
  `hd-inspector.js`, `hd-look.js`, `hd-designer-dialogs.js`; plus the M4 edits to `hd-app.js` (edit surface, realm lanes),
  `hd-render.js` (design mode), `hd-store.js` (docs cache and lock), `hd-api.js` (save, validate, duplicate, entities) and
  `hd-catalog.js` (`pickWidgetFor`, palette groups). The contract and the test commands are in "Designer (M4)" below.
- Not owned here: the desktop app shell and editor, and all backend code (`internal/homedash`, `internal/server`).

## Local Contracts

### Load order and parts

- `window.HomeDashEngine` is the namespace, written `E` below. Files are classic IIFE scripts that extend it.
- `ENGINE_ORDER` in `scripts/homedash-tests/harness.mjs`: `hd-core.js`, `hd-format.js`, `hd-colour.js`, `hd-grid.js`,
  `hd-layout.js`, `hd-doc.js`, `hd-history.js`, `hd-icons.js`, `hd-geometry.js`, `hd-input.js`, `hd-prim-displays.js`,
  `hd-prim-controls.js`, `hd-widgets.js`, `hd-control.js`, `hd-popover.js`, `widgets/hd-w-misc.js`, `widgets/hd-w-climate.js`,
  `widgets/hd-w-light.js`, `widgets/hd-w-sensor.js`, `widgets/hd-w-switch.js`, `widgets/hd-w-action.js`,
  `widgets/hd-w-select.js`, `widgets/hd-w-number.js`, `hd-moreinfo.js`. The hero modules use `E.widgets.heroStatus`, so they
  load after `hd-widgets.js`. `hd-control.js` and `hd-popover.js` load before the widgets that use them; `hd-moreinfo.js` is last
  in the engine because it needs `E.popover` and `E.control`. The primitives look up `E.input` when a builder binds an `input`.
- `APP_ORDER` (harness) is `ENGINE_ORDER` followed by `hd-tone.js`, `hd-api.js`, `hd-stream.js`, `hd-store.js`, `hd-command.js`, `hd-catalog.js`,
  `hd-render.js`, `hd-sheet.js`, `hd-view.js`, `hd-app.js`. `hd-command.js` comes after the store because it reads the view and
  the stream; `hd-render.js` comes after it because the widget api routes `command` through it. The desktop module-loader entry `homedash` loads the same files in the
  same order, then the host scripts `homedash-home.js` and `homedash.js`. `scripts/homedash-tests/wiring.test.mjs` and
  `TestHomeDashWiring` (`ui/desktop_homedash_wiring_test.go`) pin both lists.
- Parts: `E.core`, `E.el`, `E.svg`, `E.format`, `E.colour`, `E.grid`, `E.layout`, `E.doc`, `E.history`, `E.icons`, `E.geometry`,
  `E.prim` (the builders plus `E.prim.util`; `picker` is new in M3), `E.widgets`, `E.input` (M3), `E.control` (M3), `E.popover` (M3),
  `E.tone`, `E.api`, `E.stream`, `E.store`, `E.command` (M3), `E.catalog`, `E.render`, `E.sheet`, `E.moreinfo` (M3), `E.view` and
  `E.mount`.
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
- Interactive builders: disabled or unavailable sets `aria-disabled="true"`; read-only sets `aria-readonly="true"`.
  M3 amendment A1 (lead, 2026-11-25, after the Batch C review): read-only never sets `aria-disabled`, on buttons too. Plain buttons
  keep their enabled state and announce the long read-only reason when activated (rule 9). `aria-disabled` is for an unavailable
  entity, a missing capability, or a stepper button at its limit.
- **Input contract (M3, SCR-M3-4).** The control builders take an optional `input` object: `toggle`, `rocker`, `pushButton`
  (and `pushButton({toggle})`, which gives `role=switch`), `slider`, `knob`, `stepper`, `segmented`, `ctStrip`, `colourWheel`, and
  the new `picker` (a listbox trigger). Without `input` the markup is the M0 markup. With `input` the builder binds `E.input` for
  pointer and keyboard, and `destroy()` unbinds it. The callbacks by control:
  - `toggle`, `rocker`, `pushButton`, `picker`: `{enabled, onBlocked, onActivate, onHold}`.
  - `slider`, `ctStrip`: `{range, onInput, onCommit, onKey, onToggle, onHold, enabled, onBlocked}`. `knob` adds `onToggle` (centre
    tap, Enter, Space).
  - `stepper`: `{range, onStep(v)}`. `segmented`: `{onPick(id, viaKeyboard)}`. `colourWheel`: `{onInput, onCommit, onKey}` with
    `{h 0-360, s 0-100}` values.
  - A `set()` during a drag keeps the dragged value (input lock). A key press paints its value until the next `set()`.
    The binders take injectable `win`, `setTimeout`, `clearTimeout` and `now` for tests.
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
  or `{document}`) and `dashboard` DELETE (`revision`). M3 adds `commands` (POST, one request per action with its own `request_id`) and the `home_assistant`
  template on `dashboards` POST. The client still does not call `entities`, `entity`, `snapshot`, `dashboards/validate`, PUT
  `dashboard`, `duplicate`, `revisions`, `restore` or `approve`.
- Every failure is an Error with `hdApiError`, `status`, `code`, `message`, `data` and `retryAfter`. The code is the body's `code`,
  else its `error`, else a transport word (`aborted`, `not_modified`, `network`, `http_<status>`).
- Codes acted on: `homedash_disabled`, `storage_unavailable`, `desktop_readonly`, `dashboard_not_found`, `limit_exceeded`,
  `revision_conflict`, `stream_not_found`, `unauthorized`. Command codes (M3, `hd-command`): `confirmation_required` arms the
  control, `read_only` and `desktop_readonly` lock the entity (the second also raises the desktop read-only event), and `429`
  drops an intermediate continuous send or retries the final one once.
- **Widget api (M3).** The `api` factory given to widgets has `can(binding, cap)`, `command(binding, cap, args, {phase})` and
  `moreInfo(binding)`. `history` honours the widget's `hours`.
  - `can` is true only in view mode, for an entity that is available, not unknown, not read-only, and has the capability with
    `allowed` not false.
  - `command` never rejects. It resolves `{status}`: `accepted` (202 with `expected`; confirmation comes from the stream), `confirmed`
    (202 without `expected`, confirmed at once), `superseded`, `armed` (a `409` or the first activation of a confirm-required control), `local`
    (a continuous or key phase still in progress; nothing sent yet), `readonly` (no POST), `refused` (no capability, not allowed, or
    the entity is unavailable or read-only at the widget level; widgets check read-only first and announce the reason), `stale`,
    `disposed`, or `failed` with `code`.
  - `moreInfo(binding)` returns `true` when the light sheet opens. It returns `false` for every entity that is not a light, so a
    hold on other entities does nothing (rule 11).
  - Without the `commands` option (the M2 page), `command` rejects with `view_only` and `can` is false for every capability.
- **Stream (`hd-stream.js`).** One SSE connection per JS realm. The union of the active owners' ids, cut to `hello.limits.entities`,
  goes out with PUT `stream/subscriptions` 250 ms after the last change and after every `hello`.
  - Silence for max(45 s, 3 × heartbeat) or an EventSource error closes the source; the client reconnects itself (after 1, 2, 4, 8,
    16, 30 s, ±20 % jitter), never through the browser's own retry, because the stream id changes.
  - A connection that fails before `hello` waits at least 5 s (a 429 `too_many_streams` looks like any other refusal) and calls
    `diagnose()` (bootstrap) to find `homedash_disabled` or 401. The backoff resets after a stream lived 60 s.
  - `revoked` with `shutdown` reconnects; `disabled` and `auth_lost` stop until `resume()`. No active owner for 10 s closes the stream.
  - PUT `stream/subscriptions` answers accepted and rejected ids; a 404 `stream_not_found` is handled by `hd-stream` `put`, and rejected ids reach the store's `subscribed` handler.
  - `acked()` lists the ids the server last acknowledged (M3; the store's prune rule uses it). When the server has
    acknowledged nothing yet and no PUT is in flight, the first subscription PUT goes out at once. Later changes keep the 250 ms
    debounce.
- **Store (`hd-store.js`).** One per realm (`shared()`, keyed by the API base path). Rules:
  1. A snapshot is the truth for the ids it lists (the contract's snapshot covers newly added ids only).
  2. The server's per-entity `stale` flag is the truth for value age. A source status of `degraded` does not mark anything stale by itself.
     Connection loss (connecting or down for 5 s, or stopped) marks every entry stale, cleared per id by its next snapshot or state.
     A source status of `unavailable` takes that source's entities offline (`available` false, status `unavailable`); the last value
     is kept and returns when the source leaves `unavailable`. Only available entities are stale.
  3. `readonly` means a control that could act is blocked. It applies only to entities with a command capability (not `stream`,
     `snapshot`, `history` or `numeric`) that the server marks read-only or that sit on a read-only desktop. A pure display
     (all `system` sensors) is never drawn locked. `serverReadonly` keeps the raw server value. `readonlyReason` names why
     (M3): `''` when the entity is not read-only, else `virtual_desktop.readonly` on a read-only desktop, the entity's
     `readonly_reason`, the first command capability's `blocked_by`, or `homedash.readonly`.
  4. `removed` and PUT-rejected ids become status `unknown`.
  5. History `{origin, points:[{t, v}]}` becomes the host series `{points: [[t_ms, value|null]], source: origin}`. Cache key
     `id|span|points`, TTL 60 s under 6 h spans and 5 min otherwise, LRU of 100, at most 4 requests at a time. Live numeric states
     append to every cached series of the id (SCR-M2-2 draws the live tail).
  6. Ids a window stops showing stay subscribed for 30 s (linger).
  7. `dispose()` is idempotent: it unhooks the stream, detaches every watcher's stream handle and drops its linger.
     `shared()` replaces a disposed instance.
  8. Pruning (M3): an id that no window watches keeps its data while the stream still holds it. The store removes it only once
     `stream.acked()` no longer lists the id, so the server has acknowledged a subscription without it. A page shown again inside
     the 30 s linger needs no new PUT and gets no new snapshot, so its entry must still be there. The prune runs on `subscribed`,
     on `hello` and on every release. (The M3 browser test found the bug.)
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

### Command and input layer (M3, P1)

- **Command client (`hd-command.js`, `E.command`).** One client per JS realm (`shared()`, keyed by the API base path), so every
  window and the light sheet share one overlay. `send(spec)` is the only sender and never rejects. `spec` is `{entity, command,
  args, phase, widgetKey, widgetId, dashboardId, confirm}`. `decorate(id, view, widgetKey)` merges the overlay into a view.
  `clearLocks(reason)` drops read-only locks (all of them without a reason). `predict` gives the expected state of a command and
  `matches` compares a view with it (rule 3 tolerances). Exports: `create`, `shared`, `predict`, `matches`, `valueMatches`, `capOf`,
  `TIMING` and `CONTINUOUS`. `TIMING` holds the values of the P1 rules: `PENDING_MS` 250, `CONFIRM_MS` 8000, `CUE_MS` 6000,
  `THROTTLE_MS` 1250, `KEY_MS` 400, `ARM_MS` 4000, `FLASH_MS` 600, `RETRY_MS` 1000.
- **Phases.** `discrete` (the default: a tap, a click or a toggle), `input` (a continuous value while it moves), `commit` (the
  release of a continuous value) and `key` (a keyboard change; one trailing send). A continuous phase on a discrete command is
  treated as `discrete`.
- **Bus.** The client emits `readonly` (`{id, reason, desktop}`) and `armed` (`{id, widgetKey}`). `hd-app` listens to `readonly`:
  when `desktop` is true, the store and the view switch to the read-only desktop. `hd-app` clears the locks on every bootstrap
  refresh and when the stream goes live again, so a `403` lock does not outlive its cause.
- **Entity view fields (M3).** `decorate` adds these to a view: `pending` (true 250 ms after an unconfirmed send, never before),
  `error` (`{code, text}` while the 6 s cue lives; `status` reads `error` meanwhile), `armed` (`{command, seconds}` on the armed
  widget), `confirmed` (true during the 600 ms check), `readonly` and `readonlyReason` (store rule 3).
- **Read-only.** `send` refuses with `{status: 'readonly'}` when the view is read-only or a lock exists: no POST, no overlay. Widgets
  check `e.readonly` first (`E.control.activateOnOff`, `input.enabled`) and announce the long reason.
- **Control helpers (`hd-control.js`, `E.control`).** Capabilities, ranges and options from the entity (`cap`, `range`, `ctRange`,
  `options`, `lightModel`; `lightModel` reads the light's brightness from `brightness_pct`, and `ctRange` anchors the colour
  temperature grid at `min`). Read-only and blocked reasons (`reasonKey`, `reasonText`, `sourceName`), cue texts (`cueText`), the
  on and off command (`onOffCommand`, `activateOnOff`), tap and hold actions (`actions`, `tap`, `hold`), the state line
  (`statusText`) and the armed marker (`armMarker`). `tap` and `hold` take the widget's `actions`; `navigate` is not an M3 action.
- **Input (`hd-input.js`, `E.input`).** Pure maths: `range`, `snap`, `page`, `keyValue`, `fraction`, `valueAt`, `dialFraction`,
  `verticalFraction`, `isHoldKey`. Values snap to a grid anchored at `min` that never passes `max`. Binders on pointer and keyboard
  events, never HTML5 drag and drop:
  - `press`: buttons. Activation runs through the native click, so Enter and Space work. Hold is 500 ms. A move of 4 px or more
    swallows the click (a capture listener and `stopImmediatePropagation`), so host click handlers do not see it.
  - `gesture`: draggable controls. Pointer capture, with window listeners as the fallback. 4 px separates a tap from a drag, and a
    drag cancels the hold. A `pointercancel` after a drag commits.
  - `keys`: Arrow keys step, PageUp and PageDown move 10 % of the range (at least one step), Home and End go to the ends, Enter and
    Space toggle where the control has a toggle, and Shift+Enter and ContextMenu run hold.
  - `repeat`: one step per click; repeats while held, 400 ms then 80 ms.
  - `press` and `keys` both skip a hold key whose event is `defaultPrevented`, so a node bound to both runs `onHold` once.
  - Primary pointer only. A mouse gesture also ends on a move without a pressed button. `touch-action` is set in CSS (rule 10).
- **More-info sheet (`hd-moreinfo.js`, `E.moreinfo`).** `build({family, api, popoverRoot, onClose})` renders the dialog without
  a modal (the lab shows it inline). `create({root})` runs it modal inside `.hd-root` through `E.sheet.trap` and returns
  `{open(req), close(), isOpen(), destroy()}`, where `req` is `{api, family, opener, subscribe(fn)}`. The rows follow the
  capabilities: power (always), brightness, colour temperature, colour wheel and effect. A row without its capability is hidden.
  The effect row is an `E.prim.picker`. The foot shows the source, the last update and the read-only reason. Entity and effect
  names reach the page through `textContent` only.
- **Listbox popover (`hd-popover.js`, `E.popover`).** `listbox(o)` opens one non-modal listbox for a trigger. It is anchored below
  the trigger (above when there is no room) when the root is 600 px wide or more, and is a bottom sheet below that. `o` is
  `{root, anchor, options: [{id, label}], value, family, label, onPick(id), onClose()}`. `isOpen(root)` lets the trigger toggle,
  and `close(root)` closes it. A pointer press outside closes the popover. Escape and Tab close it, and focus returns to the trigger
  (the ARIA table below).

### P1 interaction rules (M3)

The rules below are the normative text, copied from the M3 plan without change. The amendments that follow them come from the lead
decisions and apply on top of them.

1. **Request ids.** Every user action gets a new `request_id` (`E.core.randomId('r', 20)`); a 429 retry gets a new one (the server caches failures for 30 s).
2. **Overlay.** The predicted state (contract `expected` table) applies at once to every widget bound to the entity (and the sheet). `pending` appears 250 ms after a send that is still unconfirmed, never before; continuous input that is not yet sent has no pending.
3. **Confirmation** (contract §4): a `state`, `snapshot` or `entity` frame received after sending confirms when `available` is true, `value` equals `expected.value` (booleans/strings exactly, numbers within half the capability step, exactly without step) and every expected attribute matches (`brightness_pct` ±1, `color_temp_kelvin` ±2 %, `hs_color` hue ±4° circular and saturation ±4, strings exactly). The server state already in the store at the 202 counts. A 202 without `expected` (activate, press, toggle from an unknown state) confirms at once. Confirmation shows the check glyph in the state line for 600 ms.
4. **Errors.** `superseded:true` ends silently. Errors roll back at once with a 6 s cue in the state line (alert ink, error icon) and the same text in the root's `aria-live="polite"` region; `409 confirmation_required` arms instead; `403 read_only` and `403 desktop_readonly` roll back and mark the entity read-only with the localized `blocked_by` reason (no red cue, announced); `desktop_readonly` also switches the app to the read-only desktop.
5. **Timeout.** 8 s without confirmation: if a frame since sending had `available:true` and a matching `value`, the overlay drops silently and the reported attributes show (late adoption); otherwise rollback and the timeout cue. Later frames simply update the store.
6. **Continuous** (`brightness`, `color_temp`, `color`, `set_value`): the overlay follows every input frame; intermediate sends at most every 1250 ms (leading edge included); commit (release) always sends the latest value; a 429 on an intermediate is dropped; a 429 on the final is retried once after `retry_after_ms` (default 1000 ms) if it is still the latest value. Pointercancel after a drag started commits.
7. **Keyboard** changes (any command) move the overlay at once and send one trailing command 400 ms after the last key.
8. **Confirm-required** (capability `sensitive`, or widget `confirm:true` on discrete commands): the first activation arms the widget for 4 s (countdown bar + state line "Tap again to confirm · N s", announced); the second activation within 4 s sends `confirmed:true`; a `409` arms. Continuous controls ignore widget `confirm` (Q6).
9. **Read-only** (L2 stays): controls of read-only entities keep full strength with the lock badge; activation does nothing and announces the long reason; the state line reads `word · short reason`. Unavailable entities: `aria-disabled`, word "Unavailable", no affordance.
10. **Pointer/touch:** pointer events only; primary pointer only; pointer capture (try/catch, window listeners as fallback); 4 px separates tap from drag and cancels hold; hold = 500 ms; `touch-action`: horizontal sliders and the light tile `pan-y`, vertical faders `pan-x`, dials and the colour wheel `none`; no hover-only affordance.
11. **More-info** for light entities only: hold, the visible `.hd-more` button (≥ 2×1), Shift+Enter or the ContextMenu key, or tap when `actions.tap.do = more_info`. Hold on other entities does nothing. Right-click is not an affordance in M3 (Q9).
12. **Switch commands:** one activation sends `turn_on`/`turn_off` from the shown state when allowed, `toggle` only as fallback (Q5).

**Amendments (lead decisions, 2026-11-25).**

- **A1, rule 9 wins over the keyboard table.** Read-only never sets `aria-disabled` on any control, buttons included. The action row
  of the table below is amended to match. `aria-disabled` is only for an unavailable entity, a missing capability, or a stepper
  button at its limit. The light sheet follows the same rule.
- **A2, segmented capacity (W-S round 1).** A `segmented` control holds 3 options below width 3 and 5 options from width 3. A select
  with more options than that renders the list control.
- **A3, icon-only word chips at 1×1 (W-G round 1, G-1).** A switch push and an action button at 1×1 show the icon only, centred. The
  word stays in the accessible name, and the state line carries the state. At 2×1 and larger nothing changes. The pending ring
  sits on the icon.
- **A4, prune rule (store rule 8).** The store keeps the data of an unwatched id until `stream.acked()` no longer lists it.

**Keyboard and ARIA per control** (the action row is amended by A1).

| Control | Role / ARIA | Keys |
| --- | --- | --- |
| pill, rocker, switch push, switch tile, light power chip, on/off light tile | `button[role=switch][aria-checked]`, name "title: Power" where needed | Space/Enter toggle (native click); Shift+Enter/ContextMenu more-info (lights) |
| light slider, fader, number slider, sheet brightness | `role=slider` + `aria-valuemin/max/now`, `aria-valuetext` in the unit ("64 %, On"; "35 %"; "55 °C") | Arrow ±step, PgUp/PgDn ±10 % (≥ 1 step), Home/End, Shift+Enter more-info |
| light dial | `role=slider` | as slider; Enter/Space toggle power |
| dimmable light tile | `role=slider`, `aria-roledescription` "dimmer", `aria-keyshortcuts="Enter Space"` | Arrows/PgUp/PgDn/Home/End dim; Enter/Space toggle |
| CT strip | `role=slider`, valuetext "3000 K" | as slider; End = last grid value ≤ max |
| colour wheel | `role=slider`, valuetext "Hue 27°, saturation 56 %" | Left/Right hue ∓/±5° (wrap), PgUp/PgDn hue ±30°, Up/Down saturation ±5, Home/End saturation 0/100 |
| action button | `button`; read-only stays enabled and activation announces the long reason (A1) | Enter/Space |
| segmented | `radiogroup` > `button[role=radio]`, roving tabindex | Arrows move focus and selection (trailing 400 ms send); Space/Enter select now |
| list | `button.hd-pick[aria-haspopup=listbox][aria-expanded]`; popover `ul[role=listbox][aria-activedescendant]` > `li[role=option][aria-selected]` | Enter/Space/ArrowDown open; Up/Down, Home/End, typeahead; Enter/Space pick; Escape/Tab close; focus returns to the trigger |
| stepper | `group` > two `button`s + `span[role=spinbutton]` | spinbutton: Arrows/PgUp/PgDn/Home/End (trailing send); buttons: click steps, hold repeats 400 ms then 80 ms |
| light sheet | `role=dialog aria-modal=true aria-labelledby`, focus trap, Escape, backdrop click, focus return; siblings inert except the live region | Tab cycles; first focus = power toggle |

Focus ring: the M0 box-shadow ring on every interactive role including `option` and `listbox`; family extras as M1. Reduced motion:
no keyframes or transitions run (probe), countdown and pending become static plus words. Light dial and slider ranges follow the
brightness capability, and the stepper spinbutton is named from `C.label` (the entity name when there is no title).

### First values (M3)

- **Parallel boot.** `E.mount` requests bootstrap and the widget catalog at the same time (`Promise.all` in `hd-app.js`).
- **Immediate first PUT.** The first stream subscription goes out at once (see `acked()` in the stream bullets).
- **Visibility unknown is visible.** A window counts as visible until the host has reported. The host signal starts with no report
  (`null`), which counts as visible, and the store watch starts with `active: vis.visible()`. So a window's values load before the
  first intersection report.
- **Templates re-read.** Bootstrap lists the templates. `hd-app` re-reads them (`refreshTemplates`) when the home view is entered,
  when the window becomes visible again, and when the stream goes live after a drop. A re-read also clears the command locks.
- **Target.** First values within 1.5 s (M3 decision D8).

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
  `homedash-primitives.css` and `homedash-widgets.css`. The M2 changes listed above are the recorded exceptions. The M3 exceptions are the structure change requests SCR-M3-1 to SCR-M3-8
  (listed in `documentation/homedash-visual-system.md`, M3 additions); the W1 structure and the overlay layers came in with them.
- Family files (`homedash-family-*.css`) paint only: colours, shadows, gradients, masks, paint radii and motion, through `var()` and
  `color-mix()`. The Instrument family paints the entity tile's numeral and glyph in the HUD readout colour (`--hd-i-val`).
- `data-hd-family` may sit on `.hd-root` (the view sets it). The bare `[data-hd-family]` token blocks on the root only declare family-scoped tokens and non-colour scale tokens that only widgets read; colour and font tokens are scoped to `.hd-widget[data-hd-family]` (B-1).
- Token freeze: `ui/testdata/homedash-lab/token-freeze.json` records every all-`--hd-*` rule of the six token and family
  parts. Since M3 each rule is keyed by its selector plus its at-rule chain (media, supports, container), so two rules with the
  same selector in different contexts no longer overwrite each other. `scripts/homedash-freeze-tokens.mjs` writes the file. `scripts/homedash-tests/m1.test.mjs` fails on any difference. A token change needs the lead's approval,
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
- M3 lab fixtures: `ui/testdata/homedash-lab/w1-fixtures.js` holds the W1 fixtures and the P1 scenarios. The light sheet cells use
  `E.moreinfo.build`, which works without a modal and without `hd-sheet.js`. The M3 sheet ids are listed in
  `documentation/homedash-visual-system.md`.
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

## Designer (M4)

The designer turns an open dashboard into an edit surface. It is engine code: `E.mount` (`hd-app.js`) owns the mode and the
realm lane, and `E.designer` (`hd-designer.js`) owns the document while a window edits it. The contract (C1 to C20) was frozen in
the M4 plan (`docs/superpowers/plans/2026-11-26-homedash-m4-designer.md`); this section describes the code. Where the plan and the
code differ, the code is the truth and the difference is listed under "Plan differences" at the end of this section.

### Load order and globals (C1)

- `APP_ORDER` (`scripts/homedash-tests/harness.mjs`) is the M2/M3 list, then `hd-clipboard.js`, `hd-commands.js`, `hd-save.js`,
  `hd-gestures.js`, `hd-editbar.js` (the M4 core), then the part-2 slot `hd-form.js`, `hd-samples.js`, `hd-palette.js`,
  `hd-entity-picker.js`, `hd-inspector.js`, `hd-look.js`, `hd-designer-dialogs.js`, then `hd-designer.js` and `hd-app.js`.
  The desktop module-loader entry `homedash` and `TestHomeDashWiring` pin the same order. The host scripts follow it.
- No designer file touches the DOM at load. `hd-designer.js` needs `E.commands`, `E.save`, `E.gestures`, `E.editbar` and
  `E.clipboard` at load; `E.palette`, `E.inspector` and `E.ddialogs` are feature-detected at call time. The kiosk (M6) never loads these files.

| File | Global | Exports (as written in the code) |
| --- | --- | --- |
| `hd-clipboard.js` | `E.clipboard` | `KEY`, `MAX_BYTES`, `MAX_WIDGETS`, `encode`, `decode`, `sanitizeWidget`, `channel` |
| `hd-commands.js` | `E.commands` | `NUDGE_MS`, `FIELD_MS`, `LIMITS`, the builders (`layout` … `setDoc`), `entry`, `keymap`, `spatial`, `readingOrder` |
| `hd-save.js` | `E.save` | `BACKOFF`, `DEBOUNCE_MS`, `KEEPALIVE_MAX`, `DOC_MAX`, `ERROR_CODES`, `lane`, `classify`, `utf8Bytes`, `errorText`, `conflictSheet`, `leaveSheet`, `draftsSheet` |
| `hd-gestures.js` | `E.gestures` | `THRESHOLD_CANVAS`, `THRESHOLD_PALETTE`, `HOLD_TOUCH_MS`, `TOUCH_LIST`, `EDGE_PX`, `SCROLL_MAX`, `EDGES`, `math`, `create` |
| `hd-editbar.js` | `E.editbar` | `create` (48 px bar; phases `saved saving retrying invalid conflict readonly gone`) |
| `hd-designer.js` | `E.designer` | `create`, `ACTIONS`, `DRAWER_BELOW` (900) |
| `hd-form.js` | `E.form` | `roving`, `gridMove`, `runes`, `fold`, `textIssue`, `field`, `text`, `number`, `select`, `toggle`, `segmented`, `colour`, `range`, `tabs`, `button`, `section`, `iconGrid`, `formSheet` |
| `hd-samples.js` | `E.samples` | `CELL`, `GAP`, `entities`, `history`, `thumbSize`, `pxOf`, `scaleFor`, `thumb` |
| `hd-palette.js` | `E.palette` | `create`, `model` (`fold`, `filterGroups`, `entInit`, `entReduce`, `entRequest`, `watchIds`, `listView`, `ENT_LIMIT` 200, `WATCH_MAX` 20, …) |
| `hd-entity-picker.js` | `E.entityPicker` | `create`, `model` (`requests`, `merge`, `sourcesOf`, `LIMIT`) |
| `hd-inspector.js` | `E.inspector` | `create`, `TABS`, `model` (`fieldFor`, `commitValue`, `actionChoices`, `actionValue`, `structureKey`, `familyCount`, `draftOf`) |
| `hd-look.js` | `E.look` | `open`, `model` (`custom`, `paletteFor`, `radius`, `swatches`, `previewWidgets`, `hueOf`, the option lists) |
| `hd-designer-dialogs.js` | `E.ddialogs` | `ACTIONS`, `run`, `pageMenu`, `enhanceTabs`, `exportFile`, `importDocument`, `newChoices`, `nameError`, `targetIndex`, `slugify`, `IMPORT_MAX` |

The part-2 `model` objects are pure helpers for the Node tests. No part-1 code reads them.

### Mount surface (C2, `hd-app.js`)

- Options: `canEdit` (default true), `ui: {menu, focusWindow, sheetHost}` (the host functions; `sheetHost()` returns the element that
  carries the sheet layers while the engine root is hidden on the home screen), `storage`.
- Methods: `enterEdit({widgetId?})`, `exitEdit({force?})`, `run(action, arg)`, `contextMenu(event)`, `beforeClose()`.
  `state().edit` is recomputed on every `set` (fields: `mode`, `canEdit`, `reason`, `editElsewhere`, `dirty`, `saving`, `phase`,
  `nextRetryAt`, `errorCode`, `canUndo`, `canRedo`, `selection`, `bp`, `fit`, `drafts`, `hasClipboard`, `actions`).
- Root `data-hd-mode` = `edit.mode` (`view` or `design`). Read-only (desktop policy or bootstrap) sets `canEdit` false.
  `setReadonly(true)` while editing forces `exitEdit`; `setReadonly(false)` retries every realm lane.
- Entering edit takes the realm lock (`store.docs.lock(id, ownerId)`), attaches the realm lane, then starts the designer. A failed
  start unlocks, detaches and returns `false`.
- `run` action owners (C2): the designer owns `undo redo cut copy paste duplicate delete select_all select_none reset_layout
  retry_save resolve_conflict show_draft add_widget focus_inspector toggle_palette toggle_inspector layout_lg layout_md layout_sm
  page_prev page_next` (`E.designer.ACTIONS`); `E.ddialogs` owns `new rename import export look add_page` (`E.ddialogs.ACTIONS`);
  the app owns `new_starter new_empty new_ha open duplicate_dashboard delete_dashboard edit exit_edit show_editor layout_auto fit present`.
  `rename`, `look` and `add_page` in view mode enter edit first, then run again (part-2 reading 8).
- `delete_dashboard` uses the engine confirm sheet and is for the open dashboard only. Home-list delete stays on the host's own
  confirm (Q6).
- Realm lanes: a window's dirty lane outlives the window. `hd-app.js` listens for `pagehide` and flushes every dirty realm lane with
  `keepalive`, and for `beforeunload` asks the browser to confirm while any lane reports `atRisk()`.
- Reconcile (C2, C7; the rules of the plan, not re-checked line by line in this pass): a `doc` event is ignored unless its revision
  is newer than the one shown. A newer revision in design mode goes to `designer.reconcile(...)`; in view mode it refetches unless
  the cache is at least as new. A dashboard that is gone goes home in view mode, and goes to `designer.reconcile({gone: true})` in design mode.
- `presence(owner, {focus} | null)` registers the window's focus handle; `show_editor` (another window holds the lock) calls
  `store.docs.focusEditor(id)`. `editor(id)` and `unlock(id, owner)` return the owner and release the lock (`lock` event).

### Designer controller (C4, C5, C6, C14, C16)

- `E.designer.create(o)` returns `D` (`state`, `on`, `doc`, `exec`, `undo`, `redo`, `seal`, `replace`, `select`, `focusFrame`,
  `setPage`, `setBreakpoint`, `addWidgets`, `slot`, `openPanel`, `closePanels`, `announce`, `toast`, `sheet`, `run`, `handles`,
  `contextMenu`, `reconcile`, `prepareExit`, `flush`, `destroy`). `D.state()` is frozen and rebuilt on every change.
- Every change goes through `exec(cmd)`: the command's `after` ops are applied, the command is pushed to history, the lane takes the
  new document, the view re-renders, the selection is restored by id and the announcement is spoken. Pointer gestures commit once,
  at pointer-up. Text fields pass `text: true` and call `seal()` on blur.
- Commands are pure builders `(doc, args) → command | refusal | limit | null`. A refusal is `{refused: 'limit' | 'last_page'}`; a
  limit is `{limit: 'min' | 'max' | 'edge' | 'blocked'}`. Limits: 12 pages, 120 widgets per page, 400 widgets (`E.commands.LIMITS`).
  Nudge coalesces for 800 ms (`NUDGE_MS`), field edits for 1000 ms (`FIELD_MS`). `add`, `duplicate` and `paste` compact a compact
  page with `E.grid.compact`, so the local document equals the reloaded one (F14).
- History: `E.history.create({limit: 100})` per designer. History is cleared on dashboard switch, on "take theirs" and on a clean
  reload, never on save, and dropped on exit.
- Selection: click selects; Shift and Ctrl/Cmd toggle; marquee replaces (Shift adds); Ctrl+A selects the page; Escape clears;
  switching page clears; the selection is filtered by id after every render, undo and reload.
- Keys (`E.commands.keymap`, root listener, `preventDefault` on each consumed key, ignored while typing): Ctrl/Cmd+Z undo;
  Ctrl/Cmd+Shift+Z and Ctrl+Y redo; Ctrl/Cmd+Alt+PageUp/PageDown page; Escape runs the escape ladder (gesture, drawer, selection,
  then exit edit). Ctrl/Cmd+E (mount level) toggles edit mode. Ctrl+Alt+Arrow is not used (Spaces shortcuts).
- Focus: the designer never leaves focus on `body`. Every command, undo, redo and run ends in `settleFocus()` (`hd-designer.js`),
  which restores focus inside the root (the focused frame, else the page). Roving tabindex: one frame has `tabindex=0`.
- Announcements go to one polite region (`.hd-live`). Repeated text gets a trailing NBSP so it is spoken again.

### Save lane (C8, `hd-save.js`)

- `E.save.lane({api, id, revision, doc, prepare, timers, debounceMs})`. Constants: `DEBOUNCE_MS` 800, `BACKOFF` 2, 4, 8, 16 and
  then 30 s, `DOC_MAX` 262 144 bytes (a larger document fails locally with `limit_exceeded`), `KEEPALIVE_MAX` 61 440 bytes.
- One PUT is in flight at a time. A PUT whose body is at most `KEEPALIVE_MAX` bytes is sent with `keepalive: true`, so it survives
  a tab unload; a longer body is sent without it. `flush({keepalive})` only picks `settle(0)` against `settle(5000)`.
- `flush()` while a PUT is open marks one follow-up (`flushAsk`). The follow-up goes out right after that PUT succeeds, without the
  debounce. A failed PUT drops it. Residual window: a change made while a PUT is open is lost if the whole tab unloads before the
  open PUT answers.
- Generation counter: `conflict`, `overwrite` and `reset` start a new generation. A late answer of an older PUT changes nothing.
- Error mapping: retry for status 0, 401, 408, 429 and 5xx (`retryAfter` wins); 409 goes to `conflict`; 404 to `gone`; 403 to
  `readonly` until `retryNow()`; other 4xx (400, 413, 422 incl. `bad_json`, `limit_exceeded`) to `invalid` until the next `set`.
- `atRisk()` tells the window whether a close could lose changes (a dirty body over `KEEPALIVE_MAX`, or an open PUT that is not
  keepalive or has a queued follow-up). `hd-app.js` uses it for the `beforeunload` prompt.
- Conflict choices: "Take theirs" (GET, `replace`, clear history, `reset`), "Keep mine" (`overwrite(currentRevision)`), "Save mine
  as a copy" (create with a copy suffix, open the copy in edit mode; the original lane is discarded).
- Sheets (`E.sheet`): `conflictSheet`, `leaveSheet` (mode `exit` uses `keep` / `leave` / `discard`; mode `close` uses
  `close_keep` / `close_leave` / `close_discard`; the primary button depends on the phase), `draftsSheet`.

### Store docs, lock and presence (C7, `hd-store.js`)

- `store.docs`: `get(id)`, `put(id, doc)` (refuses an older revision, accepts an equal one, emits `doc`), `drop(id)`, `lock(id, owner)`,
  `unlock(id, owner)`, `editor(id)`, `presence(owner, handle|null)`, `focusEditor(id)`. The lane saves the body with the new
  revision (`Object.assign({}, ev.body, {revision})`).
- The lock is per dashboard and per realm: one owner at a time. Other windows see `editElsewhere` and can focus the editor.

### API (C9, `hd-api.js`) and catalog (C10, `hd-catalog.js`)

- `saveDashboard(id, revision, document, extra)` (PUT `dashboard?id=`), `validateDocument(document, extra)` (POST
  `dashboards/validate`), `duplicateDashboard(id, name?, extra)` (POST `dashboard/duplicate?id=`), `entities` and `entity` (C9).
  `extra.keepalive` sets `init.keepalive`.
- The server answers `422 invalid_document` with `bad_json` for an unknown field, so no client body may carry one (the save lane
  strips the server-owned fields).
- Catalog: `defaults()`, `pickWidgetFor(entity, registry)` (first catalog default with the entity's kind, device class and capability
  when set; else `entity/tile`), `accepts`, `binding`, `optionSpecs`, `canToggle`, `families()`, `paletteTokens()` (at most 40),
  `paletteGroups(registry)` in the order `control, climate, sensor, media, camera, status, layout`.

### Clipboard (C11, `hd-clipboard.js`)

- `encode(doc, pageId, ids)` writes `{"v":1,"widgets":[…]}` in reading order, without ids. `decode(text, {catalog, pages})` treats the
  text as untrusted: length cap before parse, version 1, at most 120 entries, and each widget is rebuilt by `sanitizeWidget`
  (catalog type and variant, text rules from `internal/homedash/sanitize.go`, entity ids through `ParseEntityID` grammar, typed options).
- `channel({storage, win})` is one per realm (`realm.clipboard`); it notifies same-realm windows directly and other tabs through `storage`.

### Gestures and render (C12, C13, `hd-gestures.js`, `hd-render.js`)

- `E.input.track` drives every drag: primary pointer and main button only, thresholds 4 px (canvas) and 5 px (palette), touch
  long-press 300 ms for an unselected frame. Commit kinds: `move`, `resize`, `marquee`, `add`, `rebind`. Cancel (Escape, pointer
  cancel, blur, page switch, reconcile, exit) restores the start layout.
- Auto-scroll within 48 px of the stage edge. Metrics are frozen at gesture start. The page is laid out at the preview width and
  scaled with CSS zoom to the stage (`page.metrics().zoom`); gestures divide client coordinates by the zoom in `local()`.
- `E.render.page({mode, bp, fit, describe, emptyAction})`: design mode uses `role=listbox` with `aria-multiselectable` on the page and
  `role=option` on frames. Forced widths: lg `max(W, 960)`, md `clamp(W, 600, 768)`, sm `390` (`--hd-e-pw`).

### Keyboard, DOM and CSS contract (C14, C15)

- The DOM contract table is C15 in the plan. The styled selectors are in `ui/css/desktop-app-homedash.css` (editor chrome) and the
  bundle parts (widgets and frames). Editor geometry is written as `--hd-e-*` (`fx`, `fy`, `dx`, `dy`, `pw`, `g`, `mx`, `my`, `mw`,
  `mh`, `ax`, `ay`, `tw`, `th`, `ts`). Those are runtime variables, not design tokens. Host-local chrome variables are
  `--hdx-e-*` (see the visual system page).
- The designer root carries `data-hd-mode`, `data-hd-drawers`, `data-hd-panel`, `data-hd-gesture`, `data-hd-has-palette`,
  `data-hd-has-inspector`. Drawers are used below 900 px root width.

### Part 2 rules (C17 to C20)

- **i18n prefixes.** Part 2 adds `palette.*`, `inspector.*`, `look.*`, `dialog.*`, `option.<key>.label|help`, `category.<cat>`,
  `family.<id>`, `device_class.*` and `icon.<name>`. `homedash.icon.` is in the registered dynamic list (`ui/i18n_lint_test.go`).
  Every key is a literal in the code, and every new key exists in all 16 locales.
- **Forms.** `E.form` controls build the shared chrome (`hd-f-*` classes). A form dialog opens on `E.sheet` with no built-in
  actions and its own `.hd-dlg-btn` buttons, so it can refuse to close while invalid. Text that the server refuses
  (`E.form.textIssue`, the copy of the `ContainsControl` and `ContainsURL` rules) is flagged inline and never sent.
- **Thumbs.** Palette thumbs render at their thumb size, capped at 2×2 (Q11), in 56 px cells with an 8 px gap.
- **Entity browser.** Pages of 200 rows; area options come from the loaded rows (Q13); at most 20 visible rows have live state
  through their own watch owner. The picker sends one request per allowed kind and filters with `catalog.accepts`.
- **Look editor.** An end-docked modal sheet with no scrim (Q3). The canvas stays visible and updates live but is inert while the
  sheet is open. The sheet is built on `ui.sheetHost()` when the engine root is hidden.
- **Icons.** `E.icons.choices()` lists the icon names. Each has a translated label `homedash.icon.<name>`; the search matches the
  label and the id, with accents folded (`E.form.fold`).

### Fixture API (C18, Go, test-only)

- The M4 fixture (`ui`, the Go test fixture) adds these routes: `PUT dashboard?id=` (queued answers, dropped connection, delay, revision
  check with 409 `revision_conflict` and `current_revision`, lint with 422 `invalid_document`); `POST dashboards/validate`;
  `POST dashboard/duplicate?id=` (201, name plus " (copy)"); `GET entities` with the filters `q`, `source`, `kind`, `area`, `limit`, `offset`.
- Controls for the Go tests: `answerPut(status, body, times)`, `dropPut(n)`, `setPutDelay(d)`, `putLog()`, `foreignSave(t, id, savedBy, mutate)`,
  `dashboardDoc(id)`. The lint codes follow the server's `validator.text` (empty or missing name and page name `required`, over the
  limits `too_long`, more than 12 pages `too_many`).
- The Go side is not re-read in this pass; the names are the plan's C18.

### Icons (C19)

- New glyph names: `undo redo copy cut paste edit duplicate panel-left panel-right window grip look layers align-left align-top search
  device-desktop device-tablet device-phone fullscreen sync cloud-off arrow-right`. `E.icons.choices()` lists the names that are not UI glyphs.

### Tests and commands (M4)

- Node (`npm run test:homedash`, all suites): `clipboard.test.mjs`, `commands.test.mjs`, `save.test.mjs`, `designer.test.mjs`,
  `palette.test.mjs`, `inspector.test.mjs`, `dialogs.test.mjs`, plus `wiring.test.mjs` (APP_ORDER) and `i18n.test.mjs` (the
  locale key sets and the allowlist of English-equal values; each allowlist entry needs a reason).
- Go, no browser: `go test ./ui -run '(?i)^(TestHomeDashWiring|TestHomeDashFixture.*|TestTranslations_.*|TestFrontend_StaticI18nKeysExistInEnglishBundle|TestFrontend_JS_NoLiteralTranslationFallbacks|TestDesktopDynamicI18nPrefixKeysExistInAllLanguages|TestDesktopAppMaterialsAdoptedByApps)$' -count=1`
  and `go vet ./ui`. The Go tests that read embedded files need a real build, not `-overlay`.
- Gates (browser, Chrome, shell sandbox off, outside the repository): `HOMEDASH_M4_SHOTS=1` runs gates E-S and E-F; `HOMEDASH_M4_SHOTS=all`
  adds E-T (16 variants at 1x and 2x). `HOMEDASH_M4_DIR` is required and receives the sheets, never the repository.
  `HOMEDASH_M4_LANG` (default `en`; `de` for the German pass) and `HOMEDASH_M4_STATES` (a regexp for development) are optional.
  The test is `TestDesktopHomeDashDesignerGatesBrowser` in `ui/desktop_homedash_designer_gates_test.go`; CI skips it.

  ```powershell
  go test -c -o D:\tmp\go-build\hd-m4-ui.test.exe ./ui
  Set-Location ui
  $env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:HOMEDASH_M4_SHOTS='1'; $env:HOMEDASH_M4_DIR='C:\disposable\homedash-m4\<run>'
  & D:\tmp\go-build\hd-m4-ui.test.exe '-test.run=^TestDesktopHomeDashDesignerGatesBrowser$' '-test.v' '-test.timeout=30m'
  ```

- After adding or renaming a browser test, run `go test ./internal/audit -run '^TestUIBrowserCIShardsCoverEveryTest$' -count=1`.

### Plan differences (M4 code against the plan text)

- The test files `form`, `look`, `samples` and `entity-picker` have no Node suite in `scripts/homedash-tests/` (the palette, inspector,
  dialogs, clipboard, commands, save and designer suites exist). Their behaviour is covered by the designer and the browser gates.
- `hd-app.js` listens for `pagehide` and `beforeunload` on the window; the host `homedash.js` does not. The plan places both in the
  mount, and the code agrees.

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
- M3 Node tests (in the same `npm run test:homedash` run): `scripts/homedash-tests/icons.test.mjs`, `input.test.mjs`,
  `command.test.mjs`, `control.test.mjs`, `overlay.test.mjs` (the sheet trap, the listbox popover and the light sheet) and
  `w1.test.mjs` (the W1 widgets, hosted on views from the demo entities in `ui/testdata/homedash-m3/ha-entities.json`).
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

- M3 controls browser test `TestDesktopHomeDashControlsBrowser` (`ui/desktop_homedash_controls_browser_test.go`): the W1 controls
  and the P1 paths in the desktop app, against the M3 fixture (`ui/desktop_homedash_fixture_m3_test.go`, with the commands route
  and the demo entities of `ui/testdata/homedash-m3/`). It needs Chrome and `AURAGO_RUN_BROWSER_SMOKE=1`. Build the binary from the
  repository root into `D:\tmp\go-build` (outside the repository, so Chrome starts), then run it from `ui` with the shell sandbox
  off and the flags quoted. `$env:HOMEDASH_M3_SHOTS='1'` also writes the review screenshots to the repository-root
  `reports/homedash-m3/` (git-ignored). Commands:

```powershell
go test -c -o D:\tmp\go-build\hd-m3-ui.test.exe ./ui
Set-Location ui
$env:AURAGO_RUN_BROWSER_SMOKE='1'; & D:\tmp\go-build\hd-m3-ui.test.exe '-test.run=^TestDesktopHomeDashControlsBrowser$' '-test.v' '-test.timeout=30m'
```

- Colour-vision and greyscale copies (after a lab run, from the repository root): `$env:HOMEDASH_LAB_CVD='1'; go test ./ui -run '^TestHomeDashLabColourVisionSheets$' -count=1`.
- After adding or renaming a browser test, run `go test ./internal/audit -run '^TestUIBrowserCIShardsCoverEveryTest$' -count=1`.

## Child DOX Index

None.
