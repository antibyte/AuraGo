# HomeDash frontend design

> Part of the HomeDash design. Where this part differs from section 4 of `../2026-11-23-homedash-design.md`, that document wins. This part was harmonised with it on 2026-11-23.

Status: frontend architecture proposal, 2026-11-23. Scope: the JavaScript engine, the two hosts, wiring, i18n and tests. Read-only design work; no files were changed. Citations use `file:line` relative to `C:\Users\Andi\Documents\repo\AuraGo`. "UNVERIFIED" marks anything I could not confirm in code.

## 0. Findings that shape the design (verified)

| # | Fact | Evidence | Consequence |
|---|------|----------|-------------|
| F1 | Docker Manager already runs **SSE plus `PUT /stream/subscriptions`** with a per-window stream id, a debounce of 250 ms, a resubscribe after every reconnect, backoff of 1/2/4/8/16/30 s, and a watchdog that waits for 3 missed heartbeats. | `ui/js/desktop/apps/docker-manager-live.js:1-283` (backoff :17, watchdog :121-140, hello :184-202) | The HomeDash stream client copies this proven, tested pattern (spine correction S1). |
| F2 | The desktop audit browser shell **stubs `window.WebSocket`**, so the live tests have to take the native one back from an iframe. | `ui/js/desktop/apps/AGENTS.md:648` | Choosing a websocket would make every HomeDash browser test harder. |
| F3 | `AuraSSE` is one shared `/events` EventSource with jittered backoff. It skips login/setup paths and lives in `shared-core.js`, which is the main UI runtime. | `ui/js/shared/shared-core.js:1903-1975` | Not usable in kiosk, and its topic is different. The engine brings its own small client. |
| F4 | The desktop websocket (`/api/desktop/ws`) is owned by the shell. | `ui/js/desktop/core/sdk-events-bootstrap.js:279-318` | Kiosk has no shell, and the brief already rejects this hub. |
| F5 | The i18n "section" is the **first key segment**, not the directory. The server merges English first, then the locale, plus `common`. | `internal/i18n/i18n.go:172-179`, `:256-292` | `homedash.*` keys can live in `ui/lang/homedash/<lang>.json` and load as section `homedash`, both in the desktop and in kiosk. |
| F6 | Standalone pages are Go templates. `{{.Lang}}`, `{{.BuildVersion}}` and `TemplateDataJSON` carry the i18n of the sections listed in `uiI18NSections[page]`. `template-data.js` sets `window.I18N`. | `ui/dashboard.html:3,22-28`, `internal/server/server_routes_ui.go:75-94,139-170`, `ui/js/shared/template-data.js:15-25` | Kiosk becomes a template page with its own `uiI18NSections` entry. A ui test can parse it the way `ui/dashboard_overview_browser_test.go:64-81` parses the dashboard. |
| F7 | Global CSP: `script-src 'self' 'unsafe-inline'`, `frame-src 'self' http: https:`, `frame-ancestors 'none'`. The go2rtc viewer page sends its own nonce CSP with `frame-ancestors 'self'`. | `internal/server/server.go:2355-2369`, `internal/server/go2rtc_handlers.go:308` | Kiosk loads same-origin scripts only. A same-origin camera iframe is allowed. |
| F8 | The CI line budget walks **only `ui/js/desktop`**. | `ui/desktop_js_line_budget_test.go:10-13`, `.github/workflows/security-gates.yml:140-141` | Engine files under `ui/js/homedash/` are not guarded unless we add the root (S2). |
| F9 | Materials adoption guard: every `/css/<flat>.css` an app loads must contain `--vd-mat-` or be listed as exempt with a reason. The pattern matches flat paths only. | `ui/desktop_app_materials_test.go:624-708`, pattern `:645` | Engine and family CSS that read only `--hd-*` need exemption entries. Only the desktop bridge reads `--vd-mat-*` (S3). |
| F10 | The app loader runs scripts **sequentially**, and optional CSS failures only warn. | `ui/js/desktop/core/module-loader.js:804-818` | Load cost grows with each round trip. Keep the desktop list to about 30 files (ECalc already loads about 30). |
| F11 | The Docker Manager visibility check uses `document.hidden`, a MutationObserver on `.vd-window` (hidden, `vd-space-hidden`, display), and an IntersectionObserver on the root. | `ui/js/desktop/apps/docker-manager-store.js:279-316` | The desktop host reuses this exact rule as its visibility signal. |
| F12 | Docker Manager handles a revision conflict with a diff dialog: overwrite with mine, take theirs, keep editing. | `ui/js/desktop/apps/AGENTS.md:569-571` | HomeDash uses the same three-way choice on 409. |
| F13 | Infra saves one PUT in flight, coalesces later moves, rebases once on 409 and clears history on conflict. Its history is a capped two-stack. | `ui/js/desktop/apps/infra-store.js:480-559`, `ui/js/desktop/apps/infra-model.js:529-542`, `ui/js/desktop/apps/AGENTS.md:284-290` | Pattern for autosave and the undo stack. |
| F14 | Infra `capturePointer` ends a gesture on pointerup, pointercancel, lostpointercapture, or a mouse move with no button down. It keeps a mouse gesture alive when capture is lost with the button still down (a re-render). EasyDrag's copy lacks that last exception. | `ui/js/desktop/apps/infra-gestures.js:21-57`, `ui/js/desktop/apps/easydrag-core.js:495-525` (see :507) | Copy the Infra version. Never re-render the capturing element during a gesture (`AGENTS.md:296-298`: "gestures defer snapshot repaints"). |
| F15 | Video Studio applies each gesture to a clone of the state taken at gesture start, and converts the snap threshold from pixels (8 px) to model units. | `ui/js/desktop/apps/video-studio-timeline.js:453,496-523` | The designer computes every preview from `gestureStart` plus the delta. Nothing accumulates. |
| F16 | Camera embedding is an iframe to `/api/desktop/integrations/go2rtc/viewer/<id>?role=…&muted=1`. Setting `src="about:blank"` stops it. Sound is switched by `postMessage({type:'aurago:camera-muted'})`. Each viewer page holds a media lease (`AcquireMedia`). | `ui/js/desktop/apps/network-cameras-media.js:62-67,108-119`, `ui/js/go2rtc-viewer.js:96-101`, `internal/server/go2rtc_handlers.go:270-275` | The camera widget reuses the viewer page. Stopping it is mandatory when the widget is hidden. |
| F17 | Chart.js reads CSS variables once, when the chart is created. Hand-written SVG sparklines already exist. | `ui/js/dashboard/dashboard-charts.js:229-260`, `ui/js/desktop/apps/docker-manager-charts.js:53-78` | HomeDash uses its own SVG renderer (section 3.6). |
| F18 | The ui test `TestFrontend_StaticI18nKeysExistInEnglishBundle` scans **all of `ui/js/**`**. Local `t` helpers are registered per file, and dynamic key families are listed separately. | `ui/i18n_lint_test.go:446-503`, `:1056-1110`, `:1126-1131` | Engine files pass full literal keys. Runtime-built families go into `desktopDynamicI18nPrefixes`. |
| F19 | The browser CI shards select on `^TestDesktop[H-R]` and similar ranges, and every test must match exactly one shard. | `.github/workflows/security-gates.yml:183-194`, `internal/audit/ui_browser_ci_test.go:15-104` | All HomeDash browser tests are named `TestDesktopHomeDash…` and land in "Desktop H-R". |
| F20 | The asset pack includes the `ui/js`, `ui/lang` and `ui/css` directories. | `assets/web-assets.json:5` | `ui/js/homedash/` and `ui/lang/homedash/` ship without manifest edits. Root-level `ui/*.html` packaging is UNVERIFIED (assumed, because `dashboard.html` is served). |
| F21 | `updateWindowContext(windowId, patch)` exists and some apps receive it. RuView's dispatch passes `setWindowBeforeClose`. | `ui/js/desktop/core/window-shell-runtime.js:1484`, `ui/js/desktop/core/menus-and-routing.js:1691-1695` | The desktop host can remember the open dashboard per window and warn about unsaved edits. |

## 1. Module map and load order

### 1.1 Engine `ui/js/homedash/` (shell-independent)

Every file is a classic IIFE that adds one part to `window.HomeDashEngine` (created by `hd-core.js`, extended by the others: `E.grid`, `E.store`, …). No file touches the DOM at load. Every file stays under 700 lines (hard limit 1100). Untrusted text goes through `textContent` only. **No `innerHTML`, `outerHTML` or `insertAdjacentHTML` anywhere in HomeDash.** DOM is built with `E.el()` and `E.svg()`; icons are built with `createElementNS` from a path table.

| File | Part | Purpose | ~Lines | Kiosk |
|---|---|---|---|---|
| `hd-core.js` | `E.core` | `el`, `svg`, emitter, rAF coalescer `frame(fn)` (Infra pattern), AbortController `bag`, `clamp`, `uid`, `clone`, `motionReduced()`, `makeT(dict)` (`{{var}}` placeholders, missing key returns the key and warns once), `isTyping(target)` | 250 | yes |
| `hd-format.js` | `E.format` | Pure. Number and unit formatting (Intl, document lang), tabular split into int/frac/unit, relative time, state words, duration | 250 | yes |
| `hd-colour.js` | `E.colour` | Pure. Custom palette solver and contrast maths (visual part section 1.5) | 250 | yes |
| `hd-grid.js` | `E.grid` | Pure grid engine (section 4) | 450 | yes |
| `hd-layout.js` | `E.layout` | Pure. Breakpoints, md/sm derivation, override maintenance, cell metrics, fit-to-screen | 300 | yes |
| `hd-doc.js` | `E.doc` | Pure. Normalise and repair documents, id-keyed patch ops (apply and invert), command builders, clipboard (de)serialisation | 600 | yes (normalise only) |
| `hd-history.js` | `E.history` | Pure undo/redo with coalescing (limit 100) | 150 | no-op |
| `hd-api.js` | `E.api` | **The only file that knows URLs.** Request wrapper over the injected transport, error normalisation `{status, code, message, data}` | 250 | yes |
| `hd-stream.js` | `E.stream` | SSE client plus subscription PUT. One shared instance per JS realm, ref-counted owners (section 3.2) | 450 | yes |
| `hd-store.js` | `E.store` | Entity states, optimistic overlay, command queue and throttle, staleness, history cache, catalog cache (section 3) | 600 | yes |
| `hd-catalog.js` | `E.catalog` | Widget catalog access, `pickWidgetFor(entity)`, translated names, option-schema helpers, sample states for thumbnails | 300 | yes |
| `hd-icons.js` | `E.icons` | Icon path table and `icon(name)` builder | 400 | yes |
| `hd-geometry.js` | `E.geometry` | Pure path builders: arc, ticks, needle, monotone sparkline, history line, seven-segment map | 400 | yes |
| `hd-prim-controls.js` | `E.prim` | Control primitives: toggle, rocker, push button, slider, knob and dial, stepper, segmented control | 700 | yes |
| `hd-prim-displays.js` | `E.prim` | Display primitives: numeral, seven-segment readout, arc gauge, needle gauge, bar, level and tank, lamp, badge, sparkline | 650 | yes |
| `hd-widgets.js` | `E.widgets` | Registry plus widget host lifecycle and state attributes (section 2) | 450 | yes |
| `widgets/hd-w-switch.js` | (registers) | switch, action, binary | 400 | yes |
| `widgets/hd-w-light.js` | (registers) | light | 400 | yes |
| `widgets/hd-w-sensor.js` | (registers) | sensor, temperature, gauge, battery | 600 | yes |
| `widgets/hd-w-climate.js` | (registers) | climate, fan | 450 | yes |
| `widgets/hd-w-cover.js` | (registers) | cover, lock | 350 | yes |
| `widgets/hd-w-media.js` | (registers) | media, number, select | 450 | yes |
| `widgets/hd-w-chart.js` | (registers) | chart | 300 | yes |
| `widgets/hd-w-camera.js` | (registers) | camera (desktop: live iframe; kiosk: MJPEG or snapshot image) | 300 | yes |
| `widgets/hd-w-info.js` | (registers) | status_list, room, weather | 500 | yes |
| `widgets/hd-w-misc.js` | (registers) | clock, heading, nav, entity | 350 | yes |
| `hd-render.js` | `E.render` | Page DOM, ResizeObserver, breakpoints, fit, IntersectionObserver visibility, one rAF update flush | 500 | yes |
| `hd-sheet.js` | `E.sheet` | In-root modal sheet: focus trap, Escape, restore focus. Confirm and notice dialogs | 300 | yes |
| `hd-moreinfo.js` | `E.moreinfo` | "More info" sheets per entity kind | 650 | yes |
| `hd-view.js` | `E.view` | View controller: tabs, tap/hold actions, keyboard, banners, confirm, read-only | 450 | yes |
| `hd-generate.js` | `E.generate` | Pure. Templates and "generate from rooms" | 200 | no |
| `hd-gestures.js` | `E.gestures` | Pointer gestures for move, resize, marquee, palette drag, auto-scroll, guides | 650 | no |
| `hd-palette.js` | `E.palette` | Widget palette plus entity browser | 500 | no |
| `hd-inspector.js` | `E.inspector` | Inspector, option form, variant gallery, entity picker | 650 | no |
| `hd-designer-dialogs.js` | `E.ddialogs` | Look editor, kiosk link, import/export, conflict, page dialogs, templates | 600 | no |
| `hd-designer.js` | `E.designer` | Edit mode: selection, commands, keyboard map, autosave, conflicts, menus | 650 | no |
| `hd-app.js` | `E.mount` | Composition root for one host instance (section 1.4) | 400 | yes |
| `hd-kiosk.js` | `E.kiosk` | Kiosk boot and chrome (section 7) | 350 | only kiosk |

Total: 37 engine files, about 15k lines.

### 1.2 Desktop host
- `ui/js/desktop/apps/homedash.js` sets `window.HomeDashApp = { render, dispose }` (about 350 lines). It maps `ctx` to engine options. It owns the window menus, the context-menu boundary, the visibility signal (F11), the policy event (`aurago:desktop-policy`, as at `docker-manager-store.js:301-308`), `updateWindowContext` and `beforeClose`.

### 1.3 Kiosk page
- `ui/homedash-kiosk.html`: a Go template (section 7.1), served by the backend route.

### 1.4 Engine mount contract (`hd-app.js`)
```js
const inst = HomeDashEngine.mount(hostEl, {
  host: 'desktop' | 'kiosk',
  transport: { request(url, init) /* -> Promise<json>, throws {status, body} like desktop api() (desktop-foundation.js:1089-1118) */,
               eventSource(url) /* -> EventSource */ },
  t, lang, readonly, canEdit, dashboardId, pageId,
  ui: { setMenus?(menus), contextMenuBoundary?(root, handlers), beforeClose?(fn), updateContext?(patch), toast? },
  visibility: { visible(): bool, subscribe(fn): unsubscribe },
  kiosk?: { rotationSec, idleDimSec, wakeLock: bool },
});
inst.dispose(); inst.setReadonly(bool); inst.open(dashboardId, pageId?); inst.enterEdit(); inst.exitEdit();
inst.run(commandName)   // menu actions: 'undo', 'redo', 'copy', ...
inst.state()            // {dashboardId, pageId, mode, dirty, canUndo, canRedo}
```
`hd-app` detects the designer by feature (`E.designer` present). Kiosk does not load the designer files.

### 1.5 CSS (structure only; the visual agent owns the content and may split files)
The engine ships one stylesheet bundle, `/css/homedash.bundle.css`, built by `scripts/build-ui-bundles.js` from the parts listed in visual part section 9.1. It contains the theme bridge (so it reads `--vd-mat-*` and needs no exemption in the materials guard) and the kiosk chrome. `/css/desktop-app-homedash.css` holds the desktop host and the editor chrome. No other HomeDash stylesheet is loaded.

### 1.6 Load order
- **Desktop**, `DESKTOP_APP_ASSETS['homedash']`:
  - `styles: appStyles('/css/homedash.bundle.css', '/css/desktop-app-homedash.css')`
  - `scripts:` hd-core, hd-format, hd-colour, hd-grid, hd-layout, hd-doc, hd-history, hd-api, hd-stream, hd-store, hd-catalog, hd-icons, hd-geometry, hd-prim-controls, hd-prim-displays, hd-widgets, widgets/* (alphabetical), hd-render, hd-sheet, hd-moreinfo, hd-view, hd-generate, hd-gestures, hd-palette, hd-inspector, hd-designer-dialogs, hd-designer, hd-app, then `/js/desktop/apps/homedash.js`.
  - Add `APP_I18N_SECTIONS['homedash'] = ['homedash']`.
- **Kiosk**: the same order without hd-generate, hd-gestures, hd-palette, hd-inspector, hd-designer-dialogs and hd-designer, plus hd-kiosk last. Use `<script defer>` tags so downloads run in parallel and execution keeps the order.

## 2. Widget runtime contract

### 2.1 Registration
```js
HomeDashEngine.widgets.register('light', {
  bindings: ['primary'],                    // must equal the catalog's binding names (contract test)
  variants: ['slider', 'fader', 'dial', 'tile'],   // must equal the catalog's variant names
  create(api) { /* build DOM into api.body */ return { update(view) {}, resize(size) {}, destroy() {} }; }
});
```
The server catalog alone defines sizes (min, default, max per variant), the allowed entity kinds per binding, the option schema, default actions and `confirm` defaults. The client never copies those numbers; it reads them at runtime from `E.catalog`. That removes drift. The contract test (section 12) checks that the set of type × variant × binding names matches the catalog.

### 2.2 The `api` given to `create`
`body` (the `.hd-body` element), `widget` (the document object, read-only), `variant`, `family`, `mode` (`view` | `design` | `thumb`), `t`, `fmt`, `prim` (primitive factory), `icons`, `signal` (aborts on destroy), and:
- `entity(binding)` returns a **view state** `{id, kind, name, area, unit, device_class, value, attributes, available, status, readonly, capabilities, pending, stale, error}`, or `null` when the binding is empty. `status` is `ok`, `unavailable`, `unknown` (the entity is not in the catalog) or `error` (last command failed). Values include the optimistic overlay.
- `can(binding, capability)`: capability present, not read-only, not desktop read-only, mode is `view`.
- `command(binding, action, params, opts)` returns a Promise. `opts = { continuous?: true, commit?: true, optimistic?: patch }`.
- `moreInfo(binding)`, `history(binding, range)` (returns a Promise of a series), `announce(text)`.

### 2.3 Lifecycle (owned by `hd-widgets.js` host code, not by widgets)
1. **create**: the host builds the frame (section 9 contract), calls `def.create(api)`, then calls `update(view)` once synchronously, so there is no empty flash.
2. **mount**: the frame is attached. The host registers it with the page IntersectionObserver.
3. **update(view)**: called from the render flush (one rAF) only when a bound entity changed, the mode changed, or the overlay changed. Widgets must write only `textContent`, `aria-*`, `data-*` and numeric custom properties. **No layout reads** inside update. The host first sets the frame attributes `data-hd-status`, `data-hd-on`, `data-hd-pending`, `data-hd-stale`, `data-hd-readonly`.
4. **resize(size)**: `{w, h, px: {width, height}}`, called when the grid rect or the cell size changes (batched in the same flush). Most adaptation is CSS container queries on `.hd-widget`. JS resize is for canvas-like content (charts recompute their path).
5. **destroy()**: the host aborts `signal` (listeners, history requests), calls `destroy`, removes the element and releases subscriptions.
6. Offscreen widgets (IntersectionObserver `rootMargin: 200px`) get no `update` calls. The host marks them dirty and flushes the latest view when they become visible. Cameras additionally stop (section 10).

### 2.4 Commands, optimism, throttling
- `command()` resolves the binding to an entity id. Unless `can()` allows it, it rejects locally (control shown disabled). If `widget.confirm` is set, or the catalog marks the action sensitive (for example lock/unlock, cover open on device_class garage), `E.sheet.confirm` asks first.
- Optimistic overlay: `store.overlay[id] = { patch, ref, sentAt }`, merged over the server state. `data-hd-pending="true"` appears only after **250 ms** without confirmation (no flicker). The overlay is cleared when (a) the command response is OK **and** a state event with `updated_at >= sentAt` arrived, or (b) 8 s pass without a result: the control rolls back to the server state and shows the error cue. An error response drops the overlay, sets `status:'error'` for 6 s (`data-hd-status="error"`), and shows a toast with `homedash.error.<code>` (unknown codes fall back to the generic error).
- **Continuous controls** (slider, dial, colour): `opts.continuous` per key `entity:action`.
  - The overlay updates every frame (local feel).
  - Network: leading send, then **at most one request per 250 ms** (4/s). While a request is in flight, a newer value replaces the queued one (latest wins). On `commit` (pointerup, keyup, blur) the final value is **always sent** (immediately if idle, otherwise right after the in-flight request).
  - While the user drags, incoming server states for that attribute are held back (input lock), and for 1 s after commit unless they equal the committed value.
  - On 429 with `retryAfter` (desktop `api()` exposes it, `desktop-foundation.js:1108-1109`), keep only the final value and send it after the delay.

### 2.5 Modes
- `view`: interactive.
- `design`: identical DOM and live values. `.hd-body` gets `inert`, so nothing inside receives focus or events. The frame itself is focusable (`tabindex=0`) and selectable. Animations of state changes still run (live preview).
- `thumb`: palette cards and the variant gallery. `inert`, `aria-hidden="true"`, no store subscription. The state is `catalog.sampleState(kind, device_class)` (client synthesises a sensible value when the server has none). Transitions are off (`data-hd-motion="none"`). Rendered at natural size inside a fixed preview box scaled with `transform` (thumbnail only). Thumbnails are created lazily when visible (IntersectionObserver) and cached per `(type, variant, family, palette)` while the panel is open.

### 2.6 Primitives (shared by all four families)
Each factory returns `{ el, set(state), destroy() }` and takes `{label, …}`. Markup and the custom properties each one writes are in section 9.2. Keyboard and ARIA are built into the primitives (section 9.3), so widgets cannot get them wrong.

## 3. State store and stream client

### 3.1 API consumed (contract: backend part section 6; all URLs live in `hd-api.js`)
- The contract is backend part section 6. `hd-api.js` receives the base path from the host: `/api/desktop/homedash` on the desktop, `/api/homedash/kiosk` on the kiosk page. Entity and dashboard ids travel in query strings.
- `GET {base}/bootstrap`, `GET {base}/widget-catalog` (ETag), `GET {base}/entities` and `GET {base}/entity?id=` (desktop only; the kiosk gets its entities with its dashboard).
- `GET/POST {base}/dashboards`, `GET/PUT/DELETE {base}/dashboard?id=`. PUT sends `{revision, document}`; a stale revision returns 409 `revision_conflict` with `current_revision`; an invalid document returns 422 `invalid_document` with `issues[]`.
- `POST {base}/dashboard/approve?id=&revision=` (desktop) marks a revision as approved for wall displays.
- `GET {base}/stream` (SSE) with events `hello`, `snapshot`, `state`, `entity`, `removed`, `source`, `dashboard_changed`, `revoked`, and `PUT {base}/stream/subscriptions {stream_id, entities[]}` returning `{accepted, rejected[]}`.
- `POST {base}/commands {entity, command, args, request_id, dashboard_id, widget_id, confirmed}` returns 202 with the expected state, or an error code.
- `GET {base}/history?id=&from=&to=&points=` and `GET {base}/snapshot?id=&w=&h=`.
- Kiosk only: `POST /api/homedash/kiosk/pair {dashboard_id, code}` (sets an HttpOnly cookie), `GET …/session`, `GET …/dashboard` (approved revision plus its entities), `GET …/mjpeg?id=`, `POST …/logout`.

### 3.2 Stream client (`hd-stream.js`, the DockerManagerLive pattern, F1)
- **One instance per JS realm.** All HomeDash windows of a desktop share it, which saves HTTP/1.1 connection slots (browsers allow about 6 per origin on HTTP/1.1; whether AuraGo serves HTTP/2 on LAN is UNVERIFIED, and Go's `net/http` negotiates HTTP/2 only over TLS).
- `stream.attach(ownerId, {entities:Set, dashboards:Set, active:bool, onState, onDashboard, onConnection})` returns a handle with `set(...)`, `setActive(bool)` and `detach()`.
- The union of active owners' sets is normalised and sorted into a signature, sent 250 ms after the last change, and sent again after every `hello`. When the union exceeds `limits.entities`, priority is: owners' visible page, then open more-info, then lingering (section 3.3). Rejected ids are marked `status:'unknown'` with the reason.
- Backoff 1, 2, 4, 8, 16, 30 s plus 0–750 ms jitter (as AuraSSE, `shared-core.js:1944-1949`). The client closes a broken EventSource itself (the browser's own retry has no backoff, `docker-manager-live.js:4-5`).
- Watchdog `max(45 s, 3 × heartbeat_ms)`: when it fires, PUT the wish again. Only 404, an error or 10 s without an answer counts as dead.
- `dropped` and `resync`: reconnect, which brings a fresh snapshot. `revoked`: stop permanently and emit `onConnection(false, {revoked:true})`.
- No active owner for 10 s: close the stream. When an owner becomes active again: reconnect.
- Timers, the clock and `eventSource` are injectable, so the Node test can drive everything.

### 3.3 Store (`hd-store.js`)
- `states: Map<id, {value, attributes, available, updated_at}>`, holding subscribed ids only.
- `linger`: after a page switch the old page's ids stay subscribed for **30 s**, so switching back is instant, then they are dropped from both the subscription and the map.
- **Snapshot**: replace states for the listed ids. Ids missing from the snapshot become `unavailable`.
- **Change batching**: changed ids go into a Set. One `requestAnimationFrame` flush notifies each view, which maps ids to widgets via `bindIndex: Map<entityId, Set<widgetHost>>`.
- **Staleness**: when the connection is down for more than 5 s, every entity is `stale` (the root gets `data-hd-conn="offline"`, the banner shows). `source_status` of paused/unreachable marks that source's entities stale. A `snapshot` clears staleness.
- **Visibility**: the host calls `setActive`.
  - Desktop: F11 rule; minimised, hidden, other space or offscreen means inactive.
  - Kiosk: `document.hidden` means inactive after 60 s grace.
- **Catalog**: kept in memory with an index by kind, area and source. Refetched on `catalog_changed`, when the entity browser opens (ETag), and after reconnect when older than 10 min. About 5,000 entities fit in roughly 3 MB (estimate, UNVERIFIED).
- **History**: `history(id, {range:'24h', points:96})`.
  - Promise cache keyed by `id|range|points`; TTL 60 s for ranges under 6 h, 5 min otherwise.
  - LRU of 100 series, at most 4 concurrent requests.
  - Live numeric `state` events append to cached series (window-trimmed), so sparklines move without refetching.
- **Commands**: section 2.4. `client_ref = uid()`. At most 8 commands in flight per realm; further ones queue.

### 3.4 Dashboard documents in the store
`docs: Map<id, {doc, revision, loadedAt}>` is shared by all windows in the realm. It is also the edit lock: `editor: ownerId|null` (section 8.4). `dashboard_changed` triggers a refetch for open windows, with the rules of section 5.10.

### 3.5 Memory bounds summary
- States: subscribed plus lingering ids.
- History: 100 series of at most 500 points.
- Undo: 100 entries of id-keyed patches.
- Clipboard: one JSON blob of at most 256 KB.
- Toasts: at most 3.
- Camera iframes: section 10.

### 3.6 Charts decision: own SVG, not Chart.js
Three reasons:
1. Chart.js resolves colours once at creation (F17). Family and palette switches, and desktop theme changes, would require rebuilding charts, while SVG paths pick up `--hd-*` live through `stroke: var(...)`.
2. Twenty or more sparklines as Chart.js instances cost canvases, resize observers and animation loops. An SVG path rebuilt from 96 points is trivial (the Docker Manager precedent, F17).
3. Kiosk would have to load `chart.min.js` (a few hundred KB; exact size UNVERIFIED).

`hd-geometry.js` provides `sparkline(points, opts)`, `historyLine(series, {w, h, min, max, ticks})` with axis labels for the more-info sheet, `arc(start, sweep, value)` for dials and gauges, and `minMaxLast(series)` for the accessible summary. Deferred: bar charts and multi-series comparison. The DOM wrappers live in `hd-prim-displays.js`.

### 3.7 Camera reuse
- **Live**: an iframe to the viewer page (F16), `?role=preview&muted=1`. Sound only from more-info, by `postMessage` (`network-cameras-media.js:64-67`).
- **Snapshot mode**: `<img>` refreshed every 10 s while visible, with an object URL revoked on replace.
- Kiosk: no iframe (the kiosk page's CSP has `frame-src 'none'`). Live video is an `<img>` on `GET /api/homedash/kiosk/mjpeg?id=`; snapshots use `GET /api/homedash/kiosk/snapshot?id=`. Both are limited to the display's allowed entities.

## 4. Grid engine (`hd-grid.js` + `hd-layout.js`, pure, Node-testable)

### 4.1 Model
- An item is `{id, x, y, w, h}` in cells. A layout is an array of items. Constraints are `id → {minW, minH, maxW, maxH}`, taken from the catalog variant.
- `page.layouts = { lg: Item[], md: Item[] | null, sm: Item[] | null }`. `null` means **automatic** (derived from lg). An array means a manual override.
- **Columns**: lg 12, md 8, sm 4.
  - 12 divides by 2, 3, 4 and 6, matches Home Assistant users' habits, and gives a 1×1 cell of about 85 px in a 1180 px window (`window-shell-runtime.js:1347` preset scale). That is comfortably above the 44 px hit target.
  - 24 columns would make touch editing fiddly and double the snapping events.
  - md 8 suits portrait tablets (about 768 px gives about 85 px cells). sm 4 suits phones (390 px gives about 85 px).
- **Row unit**: `grid.row_unit = 'auto' | <px 48–200>`. `auto` means square cells (row height = column width), which keeps dials and gauges round. The cell is clamped to 56–180 px.
- **Gap**: from `look.density` (compact 8, comfortable 12, spacious 16 px) unless `grid.gap` is set. Page padding equals the gap.
- **Breakpoint** by **container** width, not viewport: `lg` at 960 px and up, `md` at 600 px and up, `sm` below. 16 px hysteresis avoids flapping when a scrollbar appears. The designer preview can force a breakpoint.
- **Compaction follows `page.compact`** (default `true`, gravity upward). With `false`, widgets stay where they are dropped and collisions still push down, so deliberate gaps need no spacer widget. Derived md and sm layouts always compact.

### 4.2 Algorithms (pseudocode; all functions return new arrays and never mutate input)
```
collides(a,b) = a.id!=b.id && a.x<b.x+b.w && b.x<a.x+a.w && a.y<b.y+b.h && b.y<a.y+a.h
sortReading(L) = L sorted by (y, x, id)

compact(L):
  placed=[]
  for it in sortReading(L):
    while it.y>0 && !placed.some(p=>collides(p,{...it,y:it.y-1})): it.y--
    while placed.some(p=>collides(p,it)): it.y++          // repairs overlapping input
    placed.push(it)
  return placed

pushDown(L, movers):                       // movers already at their new rects
  queue=[...movers]
  while queue not empty:
    m=queue.shift()
    for o in sortReading(L) where o not in movers and collides(m,o):
      o.y = m.y + m.h; queue.push(o)        // y only grows, so this terminates
  return L

moveItems(L, ids, target{x,y}, cols, origin):          // target = new top-left of the selection bbox
  L=clone(L); S=L.filter(i=>ids.has(i.id)); B=bbox(S)
  dx = clamp(target.x,0,cols-B.w)-B.x ; dy = max(0,target.y)-B.y
  S.forEach(s=>{s.x+=dx; s.y+=dy})
  if S.length==1:                              // swap with an equal-sized tile under the pointer centre
    t = L.find(o=>!ids.has(o.id) && o.w==S[0].w && o.h==S[0].h && covers(o, centre(S[0])))
    if t && moved>=1 cell: t.x=origin.x; t.y=origin.y; return compact(L)
  return compact(pushDown(L,S))

resizeItem(L, id, edge, d{dx,dy}, cols, c):
  r = start rect; apply edge: e→w+=dx; w→x+=dx,w-=dx; s→h+=dy; n→y+=dy,h-=dy (corners combine)
  w=clamp(w,c.minW,min(c.maxW,cols)); h=clamp(h,c.minH,c.maxH); keep opposite edge fixed when clamping
  x=clamp(x,0,cols-w); y=max(0,y)
  return compact(pushDown(L,[r]))

place(L, w, h, cols, startY=0):                  // first fit, row-major
  for y from startY: for x in 0..cols-w: if no collision {x,y,w,h}: return {x,y}

repair(L, cols, C):                              // documents from the agent or an import
  clamp every item's w,h to C and cols, x to 0..cols-w, drop unknown ids, then compact
```

When `page.compact` is `false`, `moveItems` and `resizeItem` return the `pushDown` result without the final `compact` step.

### 4.3 Responsive derivation (`hd-layout.js`)
```
derive(lg, toCols, C):
  out=[]; lastY=0
  for it in sortReading(lg):
    w = round(it.w*toCols/12); if toCols==4 && it.w>=6: w=4     // wide items go full width on phones
    w = clamp(w, C.minW, min(C.maxW,toCols)); h = clamp(it.h, C.minH, C.maxH)
    p = place(out, w, h, toCols, lastY); out.push({id:it.id, ...p, w, h}); lastY=p.y
  return compact(out)
```
- Effective layout for breakpoint bp: `layouts[bp] ?? derive(layouts.lg, cols[bp])`, memoised on the identity of the lg array.
- **Override maintenance** (`E.doc` commands): adding a widget places it in lg, and in every non-null override at `place(override, derivedSize)`. Deleting removes it from all layouts. lg moves and resizes never touch existing overrides. The first edit made in an md or sm preview copies the derived layout into the override and announces that the tablet or phone layout is now custom. A "Reset to automatic" action sets the override back to `null` (undoable).

### 4.4 Metrics and fit-to-screen
```
metrics(W,H, cols, rows, gap, rowUnit, fit):
  colW=(W-2*gap-(cols-1)*gap)/cols
  if fit=='fit_screen':
    cell=min(colW, (H-2*gap-(rows-1)*gap)/rows)
    if cell<48: return {mode:'overflow', colW, rowH: rowUnit=='auto'? clamp(colW,56,180) : rowUnit}
    return {mode:'fit', colW:cell, rowH:cell, offsetX:(W-(cols*cell+(cols-1)*gap))/2}
  return {mode:'scroll', colW, rowH: rowUnit=='auto'? clamp(colW,56,180) : rowUnit}
```
- Fit mode uses real layout, not `transform: scale`. Text stays crisp, hit sizes are honest, and the page is centred horizontally. When cells would drop below 48 px it falls back to scrolling and sets `data-hd-fit="overflow"`.
- The page element carries `--hd-cols`, `--hd-rows`, `--hd-colw`, `--hd-rowh`, `--hd-gap` and `--hd-offx`. Items carry only `--hd-x`, `--hd-y`, `--hd-w` and `--hd-h`; CSS computes `transform` and size with `calc()`. A window resize therefore writes **one** style per page, not per item.

## 5. Designer (version 1 unless marked "deferred")

### 5.1 Layout of the editor
`.hd-editor` grid with the edit bar on top. Under 900 px of window width, the palette and inspector become overlay drawers toggled from the edit bar, and the canvas keeps the full width.

| Region | Content |
|---|---|
| **Edit bar** | Done (exit), Undo, Redo, page tabs (editable), breakpoint segmented control (lg, md, sm), Look…, Kiosk…, More menu (import, export, duplicate dashboard, delete), save state text ("Saved", "Saving…", "Not saved – retrying", "Conflict"). |
| **Palette** (left, 280 px) | Two tabs. Widgets: catalog types grouped (Controls, Climate, Sensors, Media, Cameras, Layout), search, live thumbnails. Entities: the entity browser with search and filters. |
| **Canvas** (centre) | The live page in design mode. In md and sm preview it is constrained to 768 or 390 px and centred. |
| **Inspector** (right, 320 px) | Tabs Data, Look, Options, Actions for the selection. With nothing selected it shows page settings (name, icon). With several selected it shows a summary plus align and delete. |

### 5.2 Entering and leaving edit mode
- Enter through the View menu, the bar's "Edit" button, `Ctrl/Cmd+E`, or the widget context menu "Edit widget" (enters with that widget selected). Hidden when read-only or `canEdit` is false.
- When the dashboard is open in edit mode in another window of the same realm, the button shows "Being edited in another window" with "Show that window" (section 8.4).
- Leaving flushes autosave and waits up to 5 s. If the save is still pending, a sheet offers "Keep editing" or "Leave and keep retrying in the background". The view rebuilds in view mode and keeps the scroll position.

### 5.3 Adding widgets
- **From the palette (pointer)**: pointerdown on a card, drag after 5 px (Infra library threshold, `infra-gestures.js:11`).
  - A fixed-position avatar (`.hd-drop-avatar`) follows the pointer.
  - Over the canvas the cell under the avatar's top-left (anchored at the grab offset) gives a target rect of the catalog default size, shown as `.hd-ghost`. Other widgets reflow live through `moveItems` on a temporary layout.
  - Dropping commits `AddWidget`. Leaving the canvas hides the ghost. Escape cancels.
- **Tap or Enter** on a palette card adds at `place(lg, defaultSize)` below the current selection (or at the end), selects the widget, scrolls it into view and announces it.
- **Touch**: the palette list has `touch-action: pan-y`, so a vertical swipe scrolls. A drag starts on horizontal movement over 8 px, or after a 300 ms long-press.
- **From the entity browser**: dragging an entity row does the same. The widget type and variant come from `catalog.pickWidgetFor(entity)`: the catalog's `defaults` keyed by `kind` plus `device_class`, falling back to `kind`, then to the generic `entity` type. The binding `primary` is set to the entity and the title stays empty (it shows the entity name). Dropping several selected entities places them in reading order. Bindings that do not fit the target widget are refused with a ghost tone of `data-hd-drop="invalid"`.
- **Entity onto an existing widget**: hovering over a widget whose binding accepts the kind highlights it (`data-hd-drop="rebind"`). Dropping rebinds `primary` (undoable).

### 5.4 Move, resize, snapping, guides, auto-scroll
- **Move**: pointerdown on a selected widget frame (mouse, pen) or on any frame. Start after 4 px (`infra-gestures.js:11`), capture with the Infra `capturePointer` (F14). Each pointermove updates only `gesture.pointer`. One rAF computes the target cell from `(pointer − grabOffset + scrollDelta) / (colW+gap, rowH+gap)`, rounded (snap = always the cell grid). Only when the target cell changes does it recompute `moveItems(gestureStart.layout, …)` (F15) and write the changed items' custom properties. The dragged frame follows the pointer freely (`data-hd-dragging`, translated); the ghost shows the snapped cell.
- **Resize**: the primary selection shows 8 handles `.hd-handle[data-edge]`. The visual hit area is 12 px for mouse and 44 px with `(pointer: coarse)` through CSS. With a coarse pointer only SE, E and S handles are shown, to avoid crowding. `resizeItem` previews against the gesture-start layout. Min and max come from the catalog. At the limit, the handle gets `data-hd-limit`.
- **Guides**: during move and resize, compute in grid units the edges of the ghost equal to edges of non-moving widgets (left, right, top, bottom). Render at most 4 `.hd-guide[data-axis]` lines. Cheap, because they are integer comparisons.
- **Auto-scroll**: pointer within 48 px of the top or bottom of `.hd-stage` scrolls by `ceil(16 × proximity)` px per frame in the same rAF loop. The target cell is recomputed with the new `scrollTop`.
- **End**: pointerup commits one `MoveWidgets` or `ResizeWidget` command when the result differs from the start. A cancelled end (pointercancel, Escape, lost capture with no button) restores `gestureStart`. Snapshots and repaints from outside are deferred until the gesture ends (`AGENTS.md:296-298` rule).

### 5.5 Selection, multi-select, clipboard, delete
- Click selects. Shift- or Ctrl-click toggles. A marquee on empty canvas (after 4 px) selects intersecting widgets; Shift adds. `Ctrl/Cmd+A` selects all on the page. Escape clears the selection.
- **Group move**: dragging any selected widget moves the selection's bounding box (`moveItems` with ids). Group resize is deferred.
- **Duplicate** (`Ctrl/Cmd+D`): copies are placed with `place()` after the selection, with new ids.
- **Copy, cut, paste** (`Ctrl/Cmd+C/X/V`): the clipboard is `localStorage['homedash.clipboard.v1']` = `{v:1, widgets:[{…widget without id, lgSize}]}`. Other windows see it through the `storage` event, so paste works across pages, dashboards and windows. Paste creates new ids, keeps bindings (an unknown entity shows `status:'unknown'`), and places at `place()` from the selection's y or the end. System clipboard interop is deferred.
- **Delete** (Delete or Backspace, or the menu): no confirmation (undo exists). Announces "3 widgets deleted. Undo with Ctrl+Z."

### 5.6 Keyboard (all designer actions)
| Key | Action |
|---|---|
| Tab / Shift+Tab | Next or previous widget in reading order (frames `tabindex=0`), then the inspector |
| Arrow keys | Move the selection by 1 cell (`moveItems`). Blocked at column edges |
| Shift+Arrow | Resize the primary selection by 1 cell (right and down grow, left and up shrink) |
| Alt+Arrow | Move focus to the nearest widget in that direction (spatial) |
| Space / Ctrl+Space | Select the focused widget / toggle it in the selection |
| Enter or F2 | Focus the inspector's first field. Escape in the inspector returns to the widget |
| Escape | Cancel the running drag, else clear the selection, else (second press) leave edit mode |
| Ctrl/Cmd+Z, Ctrl/Cmd+Shift+Z, Ctrl+Y | Undo, redo |
| Ctrl/Cmd+C/X/V/D/A, Delete | Clipboard, duplicate, select all, delete |
| Ctrl/Cmd+E | Toggle edit mode |
| Ctrl/Cmd+Alt+PageUp/PageDown | Previous or next page |

Keys are ignored while `isTyping(event.target)` (`infra-gestures.js:58`). Keyboard nudges coalesce in history (section 5.7). The canvas is `role="listbox" aria-multiselectable="true"`. Frames are `role="option" aria-selected`, with `aria-label` like "Light, Living room ceiling, column 3, row 2, 2 by 2". A polite live region announces moves ("Moved to column 4, row 2"), resizes, undo/redo labels, the save state and conflicts (Docker Manager `announce` pattern, `docker-manager.js:96-100`).

### 5.7 Undo and redo (`hd-history.js` + `hd-doc.js`)
- A command is `{label, ops:[{path, before, after}], meta:{kind, ids, at}}`. Paths are **id-keyed** (`['pages', pageId, 'widgets', widgetId, 'options', 'unit']`, `['pages', pageId, 'layouts', 'lg']`), so a reorder never breaks undo. `apply(doc, ops, 'after' | 'before')` is pure and tested.
- **Coalescing**:
  - A pointer drag produces one command at pointerup.
  - Keyboard nudges with the same kind and the same selection within 800 ms merge (keep the first `before`, the last `after`).
  - Inspector text fields merge per field until blur or 1 s of idle.
  - Slider-type option inputs commit on change.
- Limit 100 (Infra uses 50, `infra-model.js:529`; documents are small). Redo clears on any new command.
- History is cleared on dashboard switch and on a conflict reload. It is **not** cleared by saves.

### 5.8 Autosave and revision conflicts
- **Dirty and debounce**: every command marks the document dirty. 800 ms after the last command one `PUT` with the full document and the base revision goes out. One request is in flight at a time. Commands made meanwhile set `dirty` again and go out in the next PUT (Infra lane idea, F13). On success the revision is updated. A pointer gesture in progress postpones the save.
- **Network or 5xx failure**: keep dirty, retry after 2, 4, 8, 16, 30 s. Show "Not saved – retrying" with a "Retry now" button. `beforeClose` warns about unsaved changes.
- **400 validation** (the server validates documents): show the server's field code translated (`homedash.error.<code>`), keep the local change, and offer "Undo last change".
- **409 `revision_conflict`**: stop autosave and open the conflict sheet (Docker Manager pattern, F12). The text names who changed it: "The agent changed this dashboard at 14:02 while you were editing."
  1. **Take theirs**: reload the server document, drop local changes, clear history.
  2. **Keep mine**: resend with the server's new revision (overwrite).
  3. **Save mine as a copy**: POST as a new dashboard with the name suffix "(copy)", then open the copy.

  Merging is deferred.
- **`dashboard_changed`** while editing: with no local unsaved changes, reload silently, keep the selection by id, clear history, and toast "Updated by the agent". With unsaved changes, show the conflict banner at once (do not wait for the 409).

### 5.9 Pages
- Tabs in the edit bar. "+" adds a page ("Page N"). Double-click or Enter on a tab renames inline. The tab context menu offers Rename, Icon…, Duplicate, Move left, Move right, Delete (confirm when it has widgets).
- Dragging tabs to reorder uses pointer gestures with a 5 px threshold. Alt+Shift+Left/Right moves the focused tab.
- The last page cannot be deleted.

### 5.10 Breakpoint preview
The edit bar's segmented control (lg, md, sm) forces the breakpoint and constrains the canvas width. Section 4.3 covers editing behaviour in md and sm. A chip "Automatic" or "Custom" with "Reset to automatic" sits next to the control.

### 5.11 Inspector
- **Data**: one picker per catalog binding.
  - Combobox (`role="combobox"`, listbox popup) with search over name, id and area.
  - Filter chips for source, kind and area. Only kinds allowed by the binding are listed.
  - Each row shows name (textContent), area, source badge and live state (subscribed while visible, about 20 rows).
  - "Clear binding". An unknown entity shows its raw id with a warning.
- **Look**:
  - Variant gallery: live thumbnails of every catalog variant with the bound entity's real state when available, otherwise the sample state. `role="radiogroup"`.
  - Family override: "Dashboard default" plus the four families, as a select.
  - Title (text, placeholder = entity name) and icon override (searchable icon list).
- **Options**: form generated from the catalog option schema.

  | Schema type | Control |
  |---|---|
  | boolean | switch |
  | enum | `<select>` (root AGENTS rule: dropdowns for defined choices) |
  | number with min/max/step | number input plus unit label |
  | string | text |
  | color | native colour input |
  | entity | the entity picker |
  | duration | number plus a unit select |

  Labels and help come from `homedash.option.<key>.label` and `.help`. Validation follows the schema with inline messages.
- **Actions**: tap and hold, each a select of `default` (catalog), `toggle`, `more_info`, `navigate` (with a page select) and `none`. A "Ask before running" switch maps to `confirm`. Free command actions are deferred.
- **Multi-selection**: count, "Apply family to all", delete.

### 5.12 Look editor (dashboard-level sheet)
Family (4 cards with live mini previews), palette (theme, dark, light, custom: accent colour, surface hue and dark/light, solved by `hd-colour.js`), backdrop (theme, solid, gradient), density (compact, comfortable, spacious), radius scale (slider 0.5–1.5), fit (scroll, fit to screen), row unit (auto or px), compaction per page. Changes apply live and are one undoable command per field. When the palette is `theme`, every save also writes `look.palette.snapshot`: the resolved colour tokens read with `getComputedStyle` from `.hd-root`, so the kiosk page can show the same colours. Image backdrops are deferred.

### 5.13 Wall displays dialog
The dialog "Wall displays" lists the dashboard's paired displays (label, created, last seen, view-only) with Revoke (confirm). "Pair a display" asks for a label and whether the display is view-only. The server returns a pairing path that is valid once and for 15 minutes. The sheet shows the URL with a copy button, a QR code (`ui/js/vendor/qrcode.min.js`) and the remaining time, with the note: "Open this link on the display within 15 minutes. It works once." The same dialog edits the document's `kiosk` block (start page, page rotation, idle dimming, screen saver, clock). When the agent has saved a newer revision than the approved one, the app shows a banner on that dashboard: "Changed by the agent. Wall displays still show the approved version." with Review, Approve and Undo (approve route, or restore of the approved revision).

### 5.14 Import, export, templates, first run
- **Export**: a download of `homedash-<slug>.json`, `{schema_version, dashboard}` without id, revision or kiosk credentials.
- **Import**: a file picker (`.json`, ≤ 1 MB) posted to the server for validation. The new dashboard opens. Unknown entities show as `unknown`.
- **No dashboards (first run)**: an empty state (`.vd-empty`-like) with "Create dashboard" and three cards: Empty; "Generate from rooms" (`E.generate.fromAreas(catalog)`: one page per area, at most 8 pages, up to 24 widgets per page via `pickWidgetFor`, ordered by kind priority light, switch, climate, cover, sensor, camera; unassigned entities go to "Other"); "Ask the agent" (opens agent chat with a prefilled prompt, desktop only, if the shell exposes it; UNVERIFIED hook).
- An empty page in edit mode shows "Drag widgets or entities here". In view mode it shows "This page is empty" plus Edit.

### 5.15 Deferred (not version 1)
Group resize, merge on conflict, system clipboard, image backdrops, per-widget custom CSS, sections and containers, conditional visibility, free command actions on tap and hold, kiosk PIN for sensitive actions, user-defined templates, bar and multi-series charts.

## 6. View mode
- **Pages**: tabs (`role="tablist"`). Arrow keys move between tabs and activate. Swipe left or right on the stage (touch only, more than 60 px horizontal, under 300 ms, not starting on a control) switches pages. The URL hash `#page=<id>` is used in kiosk; the desktop uses `updateWindowContext`.
- **Tap and hold**:
  - pointerdown starts a 500 ms hold timer; moving more than 8 px cancels it (the page may scroll, so widgets keep `touch-action: pan-y`).
  - Hold, right-click or the `.hd-more` button opens more-info. Enter and Space run tap; Shift+Enter or the ContextMenu key runs hold.
  - Defaults per kind come from the catalog (toggle for switch, light and fan; more-info for sensor, climate, camera and media).
  - `-webkit-touch-callout: none` and `user-select: none` on frames.
- **More-info sheet** (`hd-sheet` inside `.hd-root`, `role="dialog" aria-modal="true"`). It is kept inside the root so a desktop window does not cover the whole desktop. Focus trap, Escape closes, focus returns to the opener.

  | Kind | Contents |
  |---|---|
  | light | brightness slider, colour temperature slider, colour presets plus colour input |
  | climate | dial plus stepper, mode segmented control, fan and preset selects |
  | cover | open, stop, close plus position slider |
  | media | transport, volume, source select |
  | sensor and binary_sensor | 24 h history (range selector 1 h / 24 h / 7 d) plus attribute list (textContent) |
  | camera | large live view, sound toggle |
  | all kinds | footer with source, last update, read-only note |

  The sheet subscribes its entity while open.
- **Confirm**: `E.sheet.confirm({title, body, confirmLabel, tone:'danger'})`, used in both hosts (never `alert()`).
- **Banners** (`.hd-banners`, at most 2, stacked):
  - `offline`: "Reconnecting…" after 5 s down, with the attempt countdown.
  - `source`: "Home Assistant is not reachable; values may be old".
  - `readonly`: "Read-only: controls are disabled".
  - `revoked` (kiosk).
- **Read-only**: `entity.readonly`, a desktop read-only state (ctx plus policy event) or a kiosk scope without control rights sets `data-hd-readonly`. Controls get `aria-disabled="true"` and `title` "Read-only source". State stays visible. Commands are refused before sending.

## 7. Kiosk host

### 7.1 Page (`ui/homedash-kiosk.html`, Go template)
- `<html lang="{{.Lang}}" data-hd-host="kiosk">`, viewport `width=device-width, initial-scale=1, viewport-fit=cover`.
- `<script type="application/json" id="aurago-template-data">{{.TemplateDataJSON}}</script>` and `/js/shared/template-data.js` (F6). The server's `uiI18NSections["homedash_kiosk"] = {"homedash"}` (plus common is automatic, `i18n.go:291`). Extra payload: `{kiosk:{dashboard:"<slug>", rotation, idle_dim, wake_lock}}` (backend).
- `/fonts/fonts.css`, the HomeDash CSS (no desktop CSS), engine scripts with `defer`, then `hd-kiosk.js`.
- It does **not** load `shared-core.js` (main UI runtime, `AuraSSE` and more). Whether it would auto-connect `/events` is UNVERIFIED, so it is avoided.
- Body: `<main class="hd-kiosk" id="hd-kiosk"></main>` and a `<noscript>` text.
- The page is public static HTML: it carries translations but no dashboard data. Kiosk settings arrive with `GET /api/homedash/kiosk/dashboard` after pairing. Its CSP has `script-src 'self'`, `style-src 'self'` and `frame-src 'none'`, so the engine applies geometry and colours through the CSSOM (`style.setProperty`) only.

### 7.2 Boot sequence (`hd-kiosk.js`, runs on DOMContentLoaded)
1. Read template data, build `t` from `window.I18N`, set `data-hd-motion` from `prefers-reduced-motion`.
2. If `location.hash` holds `pair=<code>`: `POST /api/homedash/kiosk/pair {dashboard_id, code}` (the server sets an HttpOnly cookie). Then `history.replaceState` removes the hash. The code is never written to storage. Without a hash, `GET /api/homedash/kiosk/session` tells whether this display is already paired.
3. Transport: `fetch` with `credentials:'same-origin'`, `cache:'no-store'`, errors shaped like desktop `api()`; `eventSource = url => new EventSource(url, {withCredentials:true})`. A cookie session is required because EventSource cannot send headers (if the backend prefers bearer tokens, the stream would need a query token: an open question for the backend).
4. `HomeDashEngine.mount(main, {host:'kiosk', canEdit:false, …})`, which loads the dashboard, the catalog subset and the widget catalog, renders, and starts the stream.
5. Wake lock: `navigator.wakeLock.request('screen')` when enabled and `isSecureContext`. Re-acquire on `visibilitychange`. On plain HTTP it is unavailable: show a one-time notice in the kiosk menu ("Keep the screen on in the tablet's settings or kiosk app"). Most LAN installs run HTTP; UNVERIFIED how many use HTTPS.

### 7.3 Behaviour
- **Kiosk menu**: a 2 s long-press in the top-right 64×64 px corner, or a three-finger tap, opens a small sheet: pages, Fullscreen (`requestFullscreen`, needs this gesture), Reload, Brightness dim now, About (version, connection). There is no designer, no dashboard switching, no settings.
- **Idle dimming**: no input for `idle_dim` seconds sets `data-hd-idle="true"` (CSS dim, animations paused). The first touch only wakes and is swallowed (`capture` listener with `preventDefault`).
- **Page rotation**: while idle, advance pages every `rotation` seconds. Any interaction pauses rotation for 60 s.
- **Reconnect**: stream backoff as in section 3.2. After 2 min offline, try `location.reload()` once per 15 min (it recovers from server upgrades whose BuildVersion changed). Values stay visible as stale.
- **Revoked or invalid credential** (401/403 on boot or diagnose, or the `revoked` event): a full-screen state "This kiosk link is no longer valid. Create a new link in HomeDash." The stream stops. Recheck every 5 min (a re-enabled credential recovers).
- **Dashboard deleted** (404): "This dashboard no longer exists." Recheck every 5 min.
- **Hidden** (screen off): stream inactive after 60 s; on wake, reconnect plus snapshot.

## 8. Desktop host

### 8.1 Registration (mirrors Docker Manager)
- `module-loader.js`: the `DESKTOP_APP_ASSETS['homedash']` entry (section 1.6) and `APP_I18N_SECTIONS['homedash']`.
- `desktop-foundation.js` `appGlobalName`: `'homedash': 'HomeDashApp'` (Docker Manager at :635).
- `menus-and-routing.js` dispatch next to :1697:
  ```js
  if (appId === 'homedash' && window.HomeDashApp) {
      return window.HomeDashApp.render(contentEl(id), id, Object.assign({}, context || {}, {
          esc, t, api, setWindowMenus, clearWindowMenus, showContextMenu, wireContextMenuBoundary, updateWindowContext,
          readonly: desktopReadonly(), setWindowBeforeClose: (winId, handler) => { const win = state.windows.get(winId); if (win) win.beforeClose = handler; }
      }));
  }
  ```
- `window-shell-runtime.js`: preset `'homedash': { width: 1180, height: 780 }`, min `{ width: 420, height: 360 }`.
- `matchesExistingAppWindow` (:1491-1499): `if (appId === 'homedash') return !context?.dashboard_id || win.context?.dashboard_id === context.dashboard_id;`. Opening without a dashboard focuses any existing window; opening a specific dashboard reuses its window or creates a new one.
- Run `npm run build:ui` afterwards (the wiring test checks `main.bundle.js`, as `desktop_docker_manager_wiring_test.go:47`).
- Go side (backend agent): the `BuiltinApps` row, the icon allowlist entry, and capability gating.

### 8.2 `render(host, id, ctx)`
Follows `docker-manager.js:30-47`: `dispose(id)` first, `lifetime = new AbortController()`, and `t = key => ctx.t(key)` with full keys. It builds the visibility signal (F11 with `host.closest('.vd-window')`, as `docker-manager.js:1056`) and the policy listener, mounts the engine, and calls `updateWindowContext(id, {dashboard_id, page_id})` on change so session restore reopens the same dashboard and page. It sets `beforeClose` when dirty.

### 8.3 Menus and context menus
- Window menus (`ctx.setWindowMenus`, `docker-manager.js:466`):
  - **Dashboard**: New…, Open… (list sheet), Rename, Duplicate, Delete…, Import…, Export, Kiosk links…
  - **Edit**: Undo, Redo, Cut, Copy, Paste, Duplicate, Delete, Select all (enabled only in edit mode)
  - **View**: Edit mode, Layout lg/md/sm, Fit to screen, Present (fullscreen of `.hd-root` via `requestFullscreen`)
- Context menus through `wireContextMenuBoundary` (`docker-manager.js:1020`):
  - Widget in view mode: More info, Edit widget.
  - Widget in design mode: Edit, Duplicate, Copy, Cut, Delete, Change variant ▸.
  - Empty canvas: Paste, Add widget…
  - A button that opens a menu must stop its click (`AGENTS.md:636-637`).
- Clear menus on dispose (`docker-manager.js:1081`).

### 8.4 Read-only and multiple windows
- **Desktop read-only**: designer hidden, all commands refused, read-only banner. The policy event can switch it live.
- **Multiple windows**:
  - All windows share the realm's stream, store and `docs` cache (section 3).
  - Two windows on the same dashboard render independently.
  - Only one may be in edit mode (`docs.editor = ownerId`); the other shows "Being edited in another window" with "Show that window" (focus by window id through the host).
  - When the editor saves, the other windows re-render from `docs` with no refetch.

### 8.5 Dispose
Abort `lifetime`, detach the stream owner (last one closes the stream after 10 s), destroy widget hosts (which stops cameras: iframes set to `about:blank` and removed), disconnect observers, cancel rAF handles and timers, release the edit lock, clear menus, `root.replaceChildren()`. A pending dirty document gets one `keepalive` PUT attempt (UNVERIFIED that `api()` supports keepalive; use raw `fetch` with `keepalive: true` there).

## 9. DOM and CSS contract (for the visual system)

### 9.1 Structure
```
.hd-root [data-hd-host=desktop|kiosk] [data-hd-mode=view|design] [data-hd-family] [data-hd-palette=theme|dark|light|custom]
         [data-hd-scheme=dark|light] [data-hd-density] [data-hd-bp=lg|md|sm] [data-hd-fit=scroll|fit|overflow]
         [data-hd-motion=full|reduced|none] [data-hd-readonly] [data-hd-conn=live|connecting|offline] [data-hd-idle]
  .hd-backdrop
  .hd-bar                     (view: title, page tabs, edit button | design: .hd-editbar)
  .hd-tabs[role=tablist] > .hd-tab[role=tab][aria-selected]
  .hd-banners > .hd-banner[data-tone=info|warn|danger][role=status|alert]
  .hd-editor (design only) > .hd-palette | .hd-canvas | .hd-inspector
  .hd-stage (scroll container) > .hd-page[role=tabpanel|listbox]  style: --hd-cols --hd-rows --hd-colw --hd-rowh --hd-gap --hd-offx
     .hd-widget[data-hd-type][data-hd-variant][data-hd-family?][data-hd-size="2x1"][data-hd-status=ok|unavailable|unknown|error]
               [data-hd-on=true|false][data-hd-pending][data-hd-stale][data-hd-readonly][data-hd-tone=ok|warn|crit]
               [aria-selected (design)][data-hd-dragging][data-hd-drop=rebind|invalid]   style: --hd-x --hd-y --hd-w --hd-h
        .hd-frame > .hd-head(.hd-icon .hd-title .hd-sub) .hd-body .hd-foot? .hd-state  + button.hd-more
     .hd-ghost  .hd-guide[data-axis=x|y]  .hd-marquee  .hd-sel > .hd-handle[data-edge=n|ne|e|se|s|sw|w|nw]
  .hd-sheet-layer > .hd-sheet[role=dialog][aria-modal=true][data-kind=moreinfo|confirm|form]
  .hd-toasts > .hd-toast[data-tone]   .hd-live.hd-sr-only[aria-live=polite]
.hd-drop-avatar (fixed, outside the stage)
```
`.hd-widget` is `container-type: size; container-name: hd-widget`. Primitives adapt with `@container hd-widget (…)` queries. Container queries are already used by Chibi Sensei (`AGENTS.md:112`).

### 9.2 Primitive markup and the dynamic properties JS writes
JS writes only numbers and states. Colours, materials and geometry tokens (`--hd-surface`, `--hd-ink`, `--hd-accent`, `--hd-on-color`, `--hd-radius`, `--hd-pad`, `--hd-font-*`, `--hd-track-*`, `--hd-thumb-*`, `--hd-shadow-*`, `--hd-motion-*`) are defined by the families and palettes (visual agent).

| Primitive | Markup | JS writes |
|---|---|---|
| toggle | `button.hd-toggle[role=switch][aria-checked]` > `.hd-toggle-track` > `.hd-toggle-thumb` | `aria-checked`, `data-hd-pending` |
| slider | `div.hd-slider[role=slider][tabindex=0][data-orient=h\|v]` > `.hd-slider-track` > `.hd-slider-fill`, plus `.hd-slider-thumb` | `--hd-value` (0..1), `aria-valuenow`, `aria-valuetext` |
| dial | `div.hd-dial[role=slider]` > `svg.hd-dial-svg` (`path.hd-dial-track`, `path.hd-dial-arc`, `circle.hd-dial-knob`, optional `path.hd-dial-ticks`), plus `.hd-dial-readout` (numeral) | `--hd-value`, `--hd-angle` (deg), arc `d` |
| gauge | `div.hd-gauge[role=meter]` > svg (`path.hd-gauge-track`, `path.hd-gauge-arc`, `line.hd-gauge-needle`), plus numeral | `--hd-value`, `--hd-angle`, `data-hd-tone`, `aria-valuemin`, `aria-valuemax`, `aria-valuenow` |
| numeral | `span.hd-numeral` > `.hd-num-int`, `.hd-num-frac`, `.hd-num-unit` | textContent (tabular figures in CSS, so no layout shift) |
| segmented | `div.hd-seg[role=radiogroup]` > `button.hd-seg-opt[role=radio][aria-checked]`, plus `.hd-seg-indicator` | `--hd-index`, `--hd-count` |
| stepper | `div.hd-stepper` > `button.hd-step[data-dir=-1]`, `span.hd-step-value[role=spinbutton]`, `button.hd-step[data-dir=1]` | `aria-valuenow`, textContent |
| badge | `span.hd-badge[data-tone]` | textContent |
| icon | `svg.hd-ico[data-hd-icon][aria-hidden=true]` | path data |
| sparkline | `svg.hd-spark` > `path.hd-spark-area`, `path.hd-spark-line`, `circle.hd-spark-last` | `d`, `role=img` + `aria-label` summary (redrawn on resize, never stretched) |
| light colour | on `.hd-widget[data-hd-type=light]` | `--hd-light-level` (0..1), `--hd-light-rgb` ("r g b", integers 0–255 validated) |
| camera | `.hd-cam` > `iframe.hd-cam-frame[title]` or `img.hd-cam-still`, plus `.hd-cam-state` | `data-hd-cam=live\|still\|paused\|error` |

The complete primitive set is the list in visual part section 3 (it adds rocker, push button, knob, needle gauge, bar, level and tank, lamp, seven-segment readout, card frame, colour wheel and colour-temperature strip). Their markup is fixed in work package P0.

### 9.3 CSS rules the engine relies on
- Motion only inside `@media (prefers-reduced-motion: no-preference)`, on selectors gated by `.hd-root:not([data-hd-motion="reduced"]):not([data-hd-motion="none"])`. In the desktop, the root mirrors `body[data-animations="false"]`, giving `reduced`, so one rule set works in both hosts.
- `touch-action`:
  - `.hd-stage` and frames in view mode: `pan-y`.
  - Design mode, selected frames and handles: `none`.
  - Sliders and dials: `none`.
- No class prefix contains "ad" (`ui/AGENTS.md:21-26`); `hd-` is safe.

## 10. Accessibility, input, performance
- **ARIA**: as in section 9.2.
  - The widget frame in view mode has no role (a group with `aria-labelledby` the title). Each control carries its own role and an accessible name "<title>: <control>".
  - Read-only state values are plain text, so screen readers read them.
  - Sliders: Arrow keys step, PageUp/PageDown move 10 %, Home/End go to min/max. Each key press commits (continuous throttle with commit on keyup).
  - Segmented controls use a roving tabindex. Tabs follow the APG tabs pattern.
- **Focus**: designer focus is managed by widget id across re-renders (`keepFocus` idea, `docker-manager.js:127-130`; call `focus()` only when the focused element was removed). Sheets trap focus and restore it. A `focus-visible` ring on every interactive element.
- **Hit sizes**: at least 44×44 px for every control. The minimum cell is 56 px (48 in fit mode, so fit falls back to scrolling below that). Resize handles are 44 px with a coarse pointer.
- **Reduced motion**: section 9.3. Primitives drop transitions and spring easing; the numeral changes instantly.
- **Input**: pointer events only (no HTML5 drag-and-drop). Secondary pointers are ignored during a gesture (`isPrimary`). Pen behaves like a mouse. Hold-for-more-info uses 500 ms.
- **Budget**: 60 widgets on one page at 60 fps while 50 entities update per second, on a mid-range tablet (UNVERIFIED hardware; the browser test runs with 4× CPU throttle as a proxy).
  - Render flush at most 4 ms of scripting per frame.
  - Initial render of 60 widgets under 150 ms after data arrives.
  - A drag recomputes the layout only on cell change.
- **Rendering rules**:
  - One rAF flush for state, resize and layout.
  - Widgets never read layout in update.
  - Item geometry via custom properties plus `calc()`.
  - `contain: layout paint style` on `.hd-widget`.
  - Offscreen widgets skip updates (IntersectionObserver).
  - No timers per widget: a single 1 s "clock" tick for relative times, only while something visible needs it.
- **Cameras**: at most **4 live iframes** per page (option 1–6 per dashboard). Live only while the widget is at least 50 % visible and the host is active. After 2 s hidden, set `src='about:blank'` and remove the iframe (this releases the media lease). Beyond the cap, widgets show snapshots. Kiosk idle keeps cameras live only if the dashboard option allows it.
- **History**: at most 4 concurrent requests. Sparklines fetch only when visible.

## 11. i18n plan
- **Files**: `ui/lang/homedash/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`, all keys `homedash.*`, no English filler, German with "Du" and real umlauts. App name `desktop.app_homedash` goes in `ui/lang/desktop/*.json`.
- **Loading**: desktop through `APP_I18N_SECTIONS` (F5, `module-loader.js:887-908`); kiosk through the `uiI18NSections` entry in template data (F6). Both deliver the same dictionary to `window.I18N`.
- **Calls**: engine files call `t('homedash.…')` with **full literal keys** (Chibi practice, `AGENTS.md:116`), so no local-helper registration is needed. Runtime-built families go into `desktopDynamicI18nPrefixes` (`i18n_lint_test.go:1126-1131`): `homedash.widget.`, `homedash.variant.`, `homedash.option.`, `homedash.state.`, `homedash.kind.`, `homedash.device_class.`, `homedash.source.`, `homedash.error.`, `homedash.action.`.
- **Catalog names**: the server catalog carries ids only. The client translates `homedash.widget.<type>.name`, `.desc`, `homedash.variant.<type>.<variant>`, `homedash.option.<key>.label` and `.help`. A missing key shows the key (no literal fallback; `TestFrontend_JS_NoLiteralTranslationFallbacks`). Entity names, areas and attribute values are untrusted server text and never translated.
- **Namespaces and approximate counts**:

  | Namespace | ~Keys |
  |---|---|
  | `app` | 40 |
  | `view` | 35 |
  | `designer` | 120 |
  | `inspector` | 60 |
  | `dialog` | 70 |
  | `kiosk` | 30 |
  | `state` | 70 |
  | `widget` | 50 |
  | `variant` | 90 |
  | `option` | 120 |
  | `action` | 25 |
  | `kind`, `device_class`, `source` | 70 |
  | `error` | 35 |
  | **Total per locale** | **about 815** |

- Add a `TestTranslations_SectionFilesOnlyOwnPrefixes` case for `lang/homedash` with prefix `homedash.` (`i18n_lint_test.go:1276-1285`).

## 12. Test plan (essential only)
- **Node** `scripts/test-homedash-ui.mjs` (+ `npm run test:homedash`), using the vm harness of `scripts/test-docker-manager-ui.mjs` (no DOM in the vm, `:15-21`):
  1. Load contract: every engine file under 1100 lines (warn over 700), no DOM touch at load, each publishes its part, load order equals the module-loader entry and the kiosk template's script list.
  2. Source guards: no `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `eval`, `new Function`, `alert(`, `confirm(`, `prompt(` in HomeDash files, and no `setAttribute('style'` (the kiosk CSP forbids inline style attributes).
  3. CSS: motion gated as in section 9.3, no `ad` in class prefixes.
  4. `hd-grid`: collides, compact (including repairing overlapping input), pushDown termination, moveItems (edges, swap rule, group bbox), resizeItem (all 8 edges, min/max), place.
  5. `hd-layout`: derive (reading order, full-width rule on phones), override maintenance on add and delete, metrics and fit fallback, breakpoint hysteresis.
  6. `hd-doc` + `hd-history`: patch apply and invert round trip, id-keyed paths survive reorder, coalescing rules, limit 100.
  7. `hd-stream` with fake EventSource and fake timers: hello then PUT, union across 2 owners, debounce, resubscribe after reconnect, backoff sequence, watchdog probe, dropped and revoked, inactive close after 10 s.
  8. `hd-store`: optimistic overlay lifecycle (confirm, timeout, rollback), continuous throttle (at most 4/s, final value always sent, input lock), linger 30 s, staleness, history cache TTL and LRU.
  9. `hd-format` and `hd-generate`.
  10. Registry contract against a catalog fixture.
- **Go** `ui/desktop_homedash_wiring_test.go` `TestHomeDashWiring`: a copy of `TestDockerManagerWiring` (module-loader entry and order, i18n section, dispatch, `appGlobalName`, size presets, `matchesExistingAppWindow` marker, `main.bundle.js` markers).
- **Go** `ui/homedash_catalog_contract_test.go`: imports the Go catalog (`internal/homedash`) and asserts that every catalog type, variant and binding is registered in `ui/js/homedash/widgets/*.js` (`register('<type>'` plus the variants and bindings arrays), and that every catalog name and option has its `en.json` key. Whether a ui test can import `internal/homedash` without a cycle is UNVERIFIED; fallback: the backend writes a JSON fixture checked by both sides.
- **Line budget**: extend `TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget` to also walk `js/homedash` (S2).
- **Materials**: no exemption entries; the bundle contains the bridge, so `TestDesktopAppMaterialsAdoptedByApps` stays green.
- **Browser** (rod, `AURAGO_RUN_BROWSER_SMOKE=1`, all in shard "Desktop H-R", fixture via `desktopAuditBrowserWithAPI(t, {"/api/homedash/": fixture})` as `desktop_docker_manager_browser_test.go:447`; SSE so no WebSocket stub problem):
  1. `TestDesktopHomeDashBrowser`: open, render page, toggle a switch (optimistic, then confirmed), failed command rolls back with a toast, more-info slider sends throttled plus final value, read-only.
  2. `TestDesktopHomeDashDesignerBrowser`: enter edit, palette drag with pointer events, move, resize, arrow keys and Shift+arrows, undo/redo, autosave PUT body and revision, 409 conflict sheet with "Take theirs".
  3. `TestDesktopHomeDashLiveBrowser`: snapshot, subscription PUT on page switch, stream drop, stale banner, reconnect, camera iframe stops when its page is hidden.
  4. `TestDesktopHomeDashKioskBrowser`: kiosk template parsed in the test as `dashboard_overview_browser_test.go:64-81`, token exchange and hash removal, view-only (no designer files loaded), revoked state.
  5. Owned by the visual agent: `TestDesktopHomeDashControlLabBrowser`.

## 13. Work packages
Legend: **[B]** depends on a backend package, **[V]** depends on the visual system, **∥** can run in parallel with the previous package.

The main document (section 8) merges some of these packages with visual packages: P0 (geometry and static primitive builders) and P1 (interaction layer) replace F3a; W1 replaces F3b; W2 and W3 replace F6a; W4 replaces F6b.

| WP | Goal | Files | Contract / tests | Depends | Effort |
|---|---|---|---|---|---|
| F0 | Pure core | hd-core, hd-format, hd-grid, hd-layout, hd-doc, hd-history | Sections 4, 5.7; Node tests 1, 4–6, 9 | none | M |
| F1 ∥ | Live data layer | hd-api, hd-stream, hd-store, hd-catalog | Sections 3.1–3.5; Node tests 7–8 (against the assumed contract; adjust URLs in hd-api only) | final names [B-API] | M |
| F2 | App shell, view-only | homedash.js, hd-render, hd-widgets (registry with a generic `entity` widget), hd-sheet, hd-view, hd-app, hd-icons; module-loader, foundation, routing, window-shell edits; `ui/lang/homedash` skeleton; base CSS structure | Wiring test, i18n tests, empty state, dashboard list, generic tiles live | F0, F1, [B: dashboards CRUD, catalog, stream] | M |
| F3a ∥ | Primitives plus runtime polish | hd-geometry, hd-prim-controls, hd-prim-displays; control-lab page hooks | Sections 2.4–2.6, 9.2; Node tests for path builders; lab gallery renders every primitive in every state | F0, [V style lab for reference looks] | L |
| F3b | Widget set 1 | widgets/hd-w-switch, -light, -sensor; hd-moreinfo (light, sensor) | Browser test 1 | F2, F3a, [V families] | M |
| F4a | Designer interactions | hd-gestures, hd-designer (selection, move, resize, keyboard, history, autosave, conflict) | Sections 5.2–5.8; browser test 2 | F2 | L |
| F4b | Palette and inspector | hd-palette, hd-inspector | Sections 5.3, 5.11 | F4a, F3a | L |
| F5 | Dashboard management | hd-designer-dialogs, hd-generate; pages, breakpoints, clipboard, import/export, look editor, first run | Sections 5.5, 5.9–5.14 | F4b | M |
| F6a ∥ | Widget set 2 | hd-w-climate, -cover; more-info parts | Lab plus browser assertions | F3b, [V] | M |
| F6b ∥ | Widget set 3 | hd-w-media, -camera, -misc | Camera rules, section 10; browser test 3 | F3b, [B camera paths] | M |
| F7 | Kiosk host | ui/homedash-kiosk.html, hd-kiosk.js, kiosk CSS hooks; kiosk link dialog in F5 file | Section 7; browser test 4 | F2, F3b, [B kiosk auth, page route, `uiI18NSections` entry] | M |
| F8 | Hardening | Perf pass (60-widget fixture, throttled CPU), a11y pass, catalog contract test, docs (`documentation/homedash.md`, apps AGENTS.md section) | All tests green | all | M |

Every package leaves the app working: after F2 it is view-only with generic tiles; after F3b real controls; after F4a editable; after F5 manageable; after F7 kiosk.

## 14. Spine corrections, open questions, UNVERIFIED

### Spine corrections (with evidence)
- **S1 Transport**: use **SSE plus `PUT /stream/subscriptions`**, not a websocket. Evidence: F1 (proven pattern with Node and browser tests), F2 (audit shell stubs WebSocket), cookie auth works with EventSource (kiosk), and commands are REST anyway. Share **one stream per JS realm** across HomeDash windows (connection slots).
- **S2 Line budget**: the CI guard covers `ui/js/desktop` only (F8). Extend the test root to `ui/js/homedash` (one-line Go change), in addition to the Node check.
- **S3 Materials guard**: superseded. The engine CSS ships as one bundle that contains the theme bridge, so no exemption is needed. Keep CSS paths flat.
- **S4 i18n location**: `ui/lang/homedash/` plus `APP_I18N_SECTIONS` (desktop) and a `uiI18NSections` page entry (kiosk). Not the flat desktop JSON, because the kiosk only loads sections (F5, F6).
- **S5 Grid**: `grid.columns` fixed at 12 in schema v1, with md 8 and sm 4 derived. `layouts.md` and `layouts.sm` are `null` (automatic) or an override array. `row_unit` is `'auto'` (square) or px. The server should validate these. Compaction follows `page.compact`.
- **S6 Kiosk page**: a Go template like `dashboard.html`, not a desktop window. Ui browser tests parse it directly (F6).
- **S7 Catalog additions**: `heading` layout widget, per-kind `defaults` for `pickWidgetFor`, `sample` states for thumbnails, a per-action `sensitive` flag for confirmation. No spacer widget: `page.compact = false` allows gaps.
- **S8 Charts**: own SVG renderer; Chart.js is not loaded (F17).

### Open questions

All seven are answered in sections 4.2, 4.4 and 4.5 of the main document.

1. Are dashboards per user or global (multi-user AuraGo)?
2. Kiosk scopes: may a kiosk control entities, or view only by default? Per-page allow-list?
3. Kiosk PIN for sensitive actions in v1 or later?
4. Start-menu category: `system` or another? It must match `DesktopAppCategories`.
5. History ranges and point counts the backend can serve (Home Assistant passthrough versus the in-memory ring)?
6. Should `dashboard_changed` carry the full document (saves a refetch) or just the revision?
7. Does the agent tool hold an edit lock, or only bump revisions?

### UNVERIFIED
- Registration point of `/api/desktop/integrations/go2rtc/viewer/` (used by the client at `network-cameras-media.js:62`; not found by grep in route files).
- Packaging of root-level `ui/*.html` (assumed from `dashboard.html`).
- Whether `shared-core.js` auto-connects `/events` at load.
- HTTP/2 availability.
- The `qrcode` vendor file and API.
- `keepalive` with desktop `api()`.
- `navigator.wakeLock` on LAN HTTP (expected unavailable).
- Target tablet browser support for `inert`, container queries and `:has` (Chrome 105+ / Safari 16+ assumed).
- Whether a ui test may import `internal/homedash`.
- The "Ask the agent" shell hook.
- Performance targets on real hardware.
- Chart.js file size.
- Catalog size estimates.
