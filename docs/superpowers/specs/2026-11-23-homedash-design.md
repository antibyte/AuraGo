# HomeDash — design

Status: design for review, 2026-11-23. Not yet approved, nothing implemented.

HomeDash is a new virtual-desktop app: a dashboard to view and control switches, lights, sensors, climate,
cameras and services from every active source in AuraGo. The user composes dashboards in a drag-and-drop
designer from a library of high-quality controls in four style families. The same dashboards run on a
standalone kiosk page for wall tablets, and the AuraGo agent can read and edit them.

This document holds the decisions and the delivery plan. Three part documents hold the detail:

| Part | File | Content |
| --- | --- | --- |
| Backend | `2026-11-23-homedash/design-backend.md` | Entity hub, source adapters, API, store, kiosk auth, agent tool, config |
| Frontend | `2026-11-23-homedash/design-frontend.md` | Engine modules, grid, designer, widget runtime, hosts, tests |
| Visual system | `2026-11-23-homedash/design-visual-system.md` | Tokens, four families, primitives, widget catalog, quality process |
| Research | `2026-11-23-homedash/research-*.md`, `brief.md` | Verified facts about the repository |

**Where a part document differs from section 4 of this document, this document wins.**

## 1. Scope

Decided by the user on 2026-11-23:

1. Four style families, each complete for every control: Glass, Tactile, Minimal, Instrument.
2. Kiosk mode in version 1.
3. All suitable sources in version 1.
4. Agent access in version 1.

Quality bar, from the request: an extremely polished surface with very high-quality controls in several
variations and styles; no compromises on the visual result.

Out of scope for version 1 (see section 10): image backdrops, nested sections, conditional visibility, a kiosk
PIN, alarm panels, print/VM/storage control, system clipboard, merge on conflict.

## 2. Facts that shape the design

- AuraGo has no unified live device model. Most sources expose nothing to the UI; Home Assistant reaches the UI
  only as switches, polled every 5 s. HomeDash therefore needs its own backend layer.
- The desktop has no grid or drag-and-drop library and no shared switch, slider, gauge or sparkline. HomeDash
  builds its designer and its control library itself.
- A tested pattern for live data exists: Docker Manager's SSE stream with `PUT /stream/subscriptions`.
- Home Assistant's default service policy in AuraGo blocks `cover`, `script`, `lock`, `automation` and
  `alarm_control_panel` until the user allows them in the Home Assistant settings
  (`internal/tools/homeassistant.go:272-283`). HomeDash shows those controls as locked, with the reason.
- Project rules that bind every package: no external assets; 16 locales with real translations; modules under
  1100 lines; untrusted text through `textContent`; modals instead of `alert()`; motion gated by
  `prefers-reduced-motion` and the desktop animation switch; app surfaces painted from `--vd-mat-*`; browser
  tests named into a CI shard range; core shell edits need `npm run build:ui`.

## 3. Architecture

```
 sources (HA, FritzBox, MQTT, go2rtc, system, Docker, Proxmox, printers, Uptime Kuma, AdGuard, TrueNAS, Bluetooth)
        │  one adapter each: catalog, watch (push or poll on demand), command
        ▼
 internal/homedash  ── Entity hub: latest state, coalescing, demand-driven sources, command pipeline, history ring
        │             Dashboard store (homedash.db): documents, revisions, approved revision, kiosk devices
        │             Widget catalog (Go, golden JSON): types, variants, bindings, options, sizes
        ▼
 HTTP: /api/desktop/homedash/*  (desktop, admin scope)      /api/homedash/kiosk/*  (paired display, cookie)
       REST + one SSE stream per page with PUT subscriptions
        ▼
 ui/js/homedash  ── Engine (shell-independent): store + stream client, grid, renderer, widget runtime,
        │           primitives, four families (CSS), designer
        ├── Host A: desktop app window  (ui/js/desktop/apps/homedash.js)
        └── Host B: kiosk page          (/homedash/kiosk/{dashboard_id})
 Agent tool `homedash`: reads entities, edits dashboards through the same validation; sends no device commands.
```

Three properties hold everywhere:

- **One model.** Every source becomes entities with a kind, a state, attributes and capabilities. Each capability
  says whether it is allowed and what blocks it.
- **One document.** A dashboard is versioned JSON, validated by the server on every save, whether it comes from
  the designer or the agent.
- **One engine.** The desktop window and the kiosk page run the same engine files and the same stylesheet.

## 4. Lead decisions

These settle the points on which the part documents differ or left a question open.

### 4.1 Transport and API

| Topic | Decision |
| --- | --- |
| Live transport | SSE plus `PUT …/stream/subscriptions`, the Docker Manager pattern. No websocket. One stream per page, shared by all HomeDash windows. |
| Paths | Desktop: `/api/desktop/homedash/*`. Kiosk: `/api/homedash/kiosk/*`. Kiosk page: `/homedash/kiosk/{dashboard_id}`. Backend part section 6 is the contract; the paths assumed in frontend part section 3.1 are replaced by it. `hd-api.js` takes the base path from the host. |
| Entity ids | `<source>:<native id>`, sent in query strings, never in path segments. |
| Approve route | Add `POST /api/desktop/homedash/dashboard/approve?id=&revision=` (admin). It sets `approved_revision`. A save by a user session approves automatically. |
| `dashboard_changed` | Carries id and revisions only; clients refetch. |
| Command timing | The control moves at once. A pending indicator appears after 250 ms. After 8 s without a result the control rolls back and shows the error for 6 s. |

### 4.2 Dashboard document

| Topic | Decision |
| --- | --- |
| Ownership | Dashboards belong to the AuraGo instance, not to a user. |
| Grid columns | Fixed in schema version 1: lg 12, md 8, sm 4. The numbers live in the widget catalog, not in the document. |
| Row unit | `grid.row_unit` is `"auto"` (square cells, clamped to 56–180 px) or 48–200 px. |
| Layout item | `{id, x, y, w, h}` in cells. |
| Responsive layouts | `layouts.lg` is required. `layouts.md` and `layouts.sm` are absent (automatic, derived in the client) or a manual override. The server fills in missing `lg` items only. |
| Compaction | `page.compact` (default `true`): widgets fall upward. With `false` they stay where they are dropped, so deliberate gaps need no spacer widget. Derived md and sm layouts always compact. |
| Density | `compact`, `comfortable`, `spacious`. |
| Radius scale | 0.5–1.5. |
| Palette | `look.palette = {mode: theme \| dark \| light \| custom}`. `custom` stores the inputs `{accent, surface_hue, scheme}`; the colours are derived by the solver in visual part section 1.5. |
| Theme palette on kiosk | When `mode` is `theme`, the designer stores `look.palette.snapshot`: the resolved colour tokens of the desktop theme at save time (hex values for a fixed list of token names; the server rejects anything else). The kiosk page uses the snapshot, so a wall tablet shows the colours the user designed with. Without a snapshot it uses the neutral palette. |
| Backdrop | `theme`, `solid` or `gradient`, coloured from the palette. No user colours, no images. |
| Actions | `actions.tap` and `actions.hold` are objects `{do}` with `do` = `default`, `toggle`, `more_info`, `navigate` (plus `page`) or `none`. Free command actions are deferred. |
| Limits | 12 pages, 120 widgets per page, 400 per dashboard, 256 KiB per document, 50 dashboards. |

Final document shape:

```json
{"schema_version":1,"id":"d_k3x9q2m7a1bc","slug":"living-room","name":"Living room","icon":"sofa",
 "revision":7,"approved_revision":7,"updated_at":"2026-11-23T18:04:11Z","saved_by":"user",
 "look":{"family":"glass","palette":{"mode":"theme","snapshot":{"surface-solid":"#1a212b","ink":"#f1f4f9"}},
         "backdrop":{"kind":"theme"},"density":"comfortable","radius_scale":1.0},
 "grid":{"row_unit":"auto","gap":null,"fit":"scroll"},
 "pages":[{"id":"main","name":"Main","icon":"home","compact":true,
   "widgets":[{"id":"w1","type":"switch","variant":"rocker","family":"tactile","title":"Lamp",
     "bindings":{"primary":"ha:switch.lamp"},"options":{},
     "actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false}],
   "layouts":{"lg":[{"id":"w1","x":0,"y":0,"w":2,"h":2}]}}],
 "kiosk":{"start_page":"main","idle_dim_seconds":120,"idle_dim_level":0.3,"page_rotation_seconds":0,
          "screen_saver":"clock","show_clock":true}}
```

### 4.3 Widget catalog, version 1

24 types, 47 variants. Every variant is drawn in all four families: 188 renderings, of which 32 are hero
renderings with a full specification (visual part section 4.1). Sizes are grid cells, minimum / default.

| Type | Variants | Min / default | Binds |
| --- | --- | --- | --- |
| `switch` | pill, rocker, push, tile | 1×1 / 2×1 (rocker 1×2 / 2×2) | switch, light, fan, humidifier |
| `light` | slider, fader, dial, tile | 2×1 / 4×1; 1×2 / 1×3; 2×2 / 2×2; 1×1 / 2×2 | light |
| `climate` | dial, stepper, scale | 2×2 / 3×3; 2×1 / 2×1; 1×2 / 1×3 | climate |
| `temperature` | numeral, thermometer, comfort | 1×1 / 2×1; 1×2 / 1×2; 2×2 / 2×2 | sensor (+ humidity) |
| `sensor` | numeral, sparkline, icon | 1×1 / 2×1; 2×1 / 2×2; 1×1 / 1×1 | sensor |
| `gauge` | arc, needle, bar, tank | 2×2 / 2×2; 2×2 / 2×2; 2×1 / 3×1; 1×2 / 1×2 | numeric sensor |
| `binary` | indicator, lamp | 1×1 / 2×1; 1×1 / 1×1 | binary sensor, presence |
| `cover` | shutter, buttons | 1×2 / 2×2; 1×1 / 1×2 | cover |
| `action` | button | 1×1 / 2×1 | scene, script, button |
| `select` | segmented, list | 2×1 / 3×1; 1×1 / 2×1 | select |
| `number` | stepper, slider | 2×1 / 2×1 | number |
| `fan` | speed | 2×1 / 2×2 | fan |
| `lock` | slide | 2×1 / 3×1 | lock |
| `media` | compact, cover | 2×1 / 4×1; 2×2 / 3×2 | media player |
| `chart` | line, timeline | 3×2 / 4×2; 3×1 / 4×1 | up to 4 series |
| `camera` | frame | 2×2 / 4×3 | camera |
| `status_list` | list | 2×2 / 2×3 | 3–8 entities of any kind |
| `battery` | glyph | 1×1 / 1×1 | battery sensor |
| `room` | card | 2×2 / 3×2 | temperature, humidity, 2–4 toggles |
| `clock` | digital, analog | 1×1 / 2×1; 2×2 / 2×2 | none |
| `heading` | title | 1×1 / 4×1 | none |
| `nav` | button | 1×1 / 1×1 | none (opens a page) |
| `weather` | compact, forecast | 2×1 / 2×1; 3×2 / 4×2 | Home Assistant weather entity |
| `entity` | tile | 1×1 / 2×1 | anything; the fallback for unknown kinds |

Rules:

- The catalog lives in Go (`internal/homedash/catalog_widgets.go`) with a golden JSON file. A Node contract
  test checks that the frontend registry implements every type, variant and binding.
- A family may draw a variant with another primitive where the variant has no counterpart: Tactile draws
  `gauge/arc` as a needle instrument, Instrument draws `clock/analog` as the digital clock, Minimal draws
  `temperature/thermometer` as a hairline scale. The catalog marks such aliases so the variant gallery does
  not show two identical tiles.
- `weather/forecast` shows current conditions only until the backend can read forecasts (backend part
  section 8.1).
- The type and variant ids in backend part section 7 and the example names in frontend part section 2.1 are
  replaced by this table. Group frames, spacers, printer cards and presence widgets are not in version 1.

### 4.4 Frontend

| Topic | Decision |
| --- | --- |
| Stylesheets | The engine ships as one bundle, `ui/css/homedash.bundle.css`, built by `scripts/build-ui-bundles.js` from the parts in visual part section 9.1. The bundle contains the theme bridge, so it reads `--vd-mat-*` and needs no exemption in the materials guard. `ui/css/desktop-app-homedash.css` holds the desktop host and the editor chrome. The kiosk chrome is a part of the bundle. This replaces the file list and the exemption entries of frontend part sections 1.5 and 14 (S3). |
| Scripts | Loaded unbundled, as other apps do; the kiosk page uses `<script defer>`. The line-budget test is extended to `ui/js/homedash`. |
| Primitive modules | `hd-geometry.js` (pure: arc, tick, needle and sparkline paths, seven-segment map), `hd-prim-controls.js` (toggle, rocker, push button, slider, knob and dial, stepper, segmented control), `hd-prim-displays.js` (numeral, seven-segment readout, arc gauge, needle gauge, bar, level and tank, lamp, badge, sparkline). These replace `hd-charts.js` and `hd-primitives.js` of the frontend part and work package V2 of the visual part. The primitive list of visual part section 3 is the complete set. |
| Colour solver | `hd-colour.js` (pure): the custom palette solver and contrast maths. The designer's widget panel keeps the name `hd-palette.js`. |
| Motion gate | Every transition and animation sits inside `@media (prefers-reduced-motion: no-preference)` on selectors gated by `.hd-root:not([data-hd-motion="reduced"]):not([data-hd-motion="none"])`. The desktop host mirrors the desktop animation switch into `data-hd-motion`. |
| Sparkline | Redrawn on resize; never stretched with `preserveAspectRatio="none"`. |
| Gauge role | `role="meter"` with value attributes. |
| Browser baseline | Chromium/WebView 111, Safari/iPadOS 16.4, Firefox 113. The engine does not use `light-dark()`; scheme-dependent values are selected with `[data-hd-scheme]`. `linear()` easing gets a `cubic-bezier` fallback through `@supports`. |
| Inline styles | The kiosk page runs with `style-src 'self'`. The engine sets geometry and colours through the CSSOM (`style.setProperty`), never through `style` attributes in markup or `setAttribute('style', …)`. A Node guard enforces it. |
| Camera | Desktop: the existing go2rtc viewer page in an iframe, at most 4 live per page. Kiosk: the MJPEG relay or snapshots in an `<img>` (the kiosk CSP allows no frames). |
| Start-menu category | `comms`, next to HA Switchboard and Network Cameras. |

### 4.5 Kiosk and security

| Topic | Decision |
| --- | --- |
| Pairing | A display is paired with a single-use code in the URL fragment (128 bit, valid 15 minutes, shown as link and QR code). Pairing stores a device token in an HttpOnly cookie. The designer's dialog lists paired displays with last-seen time and a revoke button. This replaces the long-lived link of frontend part section 5.13. |
| Reach of a display | Only the entities bound on the **approved** revision of its dashboard, and only the commands its widgets can issue. Checked on the server. |
| Agent edits and kiosk | An agent save never changes what a display shows or controls. The desktop app shows a banner on such a dashboard: review, then approve or undo. |
| Never from kiosk | `unlock` and container actions. |
| Confirmation | Server-enforced for unlock, garage/gate/door covers, container stop/restart, AdGuard filtering off and scripts. |
| Defaults | `homedash.enabled: true`. `homedash.kiosk.enabled: false`, because it opens a public page; the user switches it on. `homedash.agent.access: write`. Device lifetime: until revoked. Control for Docker and AdGuard: off. `virtual_desktop.readonly` also blocks kiosk commands. |
| Agent tool | Reads entities and the widget catalog; creates, updates and patches dashboards; opens the app. No device commands, no delete, no pairing, no MQTT definitions. Text from devices is wrapped as external data. |
| HA Switchboard | Stays untouched. HomeDash does not reuse its endpoints; it reuses the Home Assistant client and service policy in `internal/tools`. |

## 5. Sources in version 1

| Source | Entities | Control | Live |
| --- | --- | --- | --- |
| Home Assistant | all common domains (table in backend part 8.1), rooms from the area registry | yes, through the existing service policy | websocket, REST fallback |
| go2rtc | cameras | no | snapshot, live view |
| FritzBox smart home | switches, radiator thermostats, temperature, lamps | yes (blinds deferred: no data in the client) | poll 15 s |
| MQTT | user-defined entities (topic, JSON path, unit) | yes, publish to a defined command topic | in-process buffer, 1 s |
| Host system | CPU, memory, disk, network, uptime | no | 5 s |
| Docker | containers | start/stop/restart, off by default | Engine events |
| Proxmox | nodes, guests | no | 30 s |
| 3D printers | status, progress, temperatures, camera | no | 10–30 s |
| Uptime Kuma | monitors, summary | no | 60 s |
| AdGuard Home | statistics, filtering switch | toggle, off by default | 30 s |
| TrueNAS | health, pools, alerts | no | 60 s |
| Bluetooth | adapter, device presence | no | push |

Not sources in version 1: device registry (no live state), RTL-SDR, RuView, Frigate, CYD, Chromecast,
MeshCentral, Grafana. Sources connect only while a dashboard that uses them is open.

History for charts and sparklines: Home Assistant's own history where available; for other sources an
in-memory ring since the server started. The UI says which one it shows.

## 6. Visual system

The detail is in the visual part. The essentials:

- **Tokens.** Widgets read only `--hd-*`. A family sets geometry, material, light and motion; a palette sets
  colours; a widget may override its accent.
- **Glass.** Layered translucent glass lit from above that takes the theme's colours.
- **Tactile.** Believable, quiet hardware: rocker switches, knurled knobs, needle instruments, satin steel,
  bakelite, engraved legends, LEDs. CSS and SVG only, no raster textures.
- **Minimal.** Type and hairlines; no shadows, no gradients.
- **Instrument.** A calm dark HUD: segmented arcs, seven-segment numerals, one glow colour.
- **Shared rules.** A state is never shown by colour alone (position, glyph and word). Text contrast 4.5:1,
  graphics 3:1. Hit targets at least 44 px. Only `transform` and `opacity` animate. One line-icon set of about
  70 glyphs, drawn for HomeDash. Editor chrome follows the desktop theme and never competes with the canvas.
- **Theme behaviour.** Glass and Minimal take surfaces, ink and radii from the desktop theme. Tactile and
  Instrument keep their own materials and take accent and status colours.

## 7. Quality process

1. **Style lab before mass production.** Eight hero controls (toggle, rocker, dimmer dial, thermostat dial,
   needle gauge, arc gauge, temperature readout, sensor tile with sparkline) are built in all four families
   first: 32 reference renderings, each in six states, two sizes, several palettes and themes, at 1× and 2×.
2. **Lead review gates.** The lead session judges the screenshot sheets after each family and after the
   cross-family pass. Nothing proceeds on a failed sheet. Review happens on sheets, not on descriptions.
3. **User sign-off.** The user sees the style-lab sheets before the remaining 156 renderings are built.
4. **Control lab.** A test-only gallery renders every type × variant × family × state × palette, with probes
   for contrast, hit size, layout shift and running animations. It is the regression net for all later work.
5. **Per-control checklist** (visual part section 10.3) and the list of ten ways the library could end up
   looking cheap, each with its rule (section 10.4).
6. **Lean verification elsewhere.** Each work package runs its essential tests; one combined review per batch;
   security-agent reviews the kiosk and command paths.

## 8. Delivery plan

About 50 work packages in nine milestones. Package ids refer to the part documents (B, K, A: backend
section 13; F: frontend section 13; V: visual section 11). Merged packages are named here.

Roster per package: maker-agent implements, test-agent runs the named tests, reader-agent reviews the batch,
security-agent reviews where marked, docs-agent writes documentation, debug-agent investigates failures.

| Milestone | Packages | Result |
| --- | --- | --- |
| **M0 Foundations** | Backend: B0, B1, B8, then B2 (catalog and schema freeze), B4a, B9a, then B3, B4b. Frontend: F0. Visual: V0, V1, **P0** (new: `hd-geometry.js` and the static DOM builders of all primitives; replaces V2 and the first half of F3a) | Contracts frozen; pure logic tested; control lab renders placeholders |
| **M1 Style lab** (gate) | V3 Glass, V4 Tactile, V5 Minimal, V6 Instrument, V7 consistency and token freeze | 32 hero renderings approved by the lead and the user |
| **M2 Live skeleton** | B5, B6, then F1, F2 | The app opens, lists dashboards and shows live generic tiles from the host system source |
| **M3 First controls end to end** | B9b (Home Assistant), **P1** (interaction layer: drag, keyboard, ARIA, pending/confirm/rollback; merges V8 and the second half of F3a), **W1** (switch, light, action, select, number, more-info for lights; merges V9 and F3b) | Real Home Assistant devices switch and dim from a hand-written dashboard |
| **M4 Designer** | F4a, F4b, F5, V14 | Dashboards are built by drag and drop, with undo, autosave, pages, breakpoints, import and export |
| **M5 Breadth** | **W2** (climate, temperature, sensor, gauge, battery; merges V10 and part of F6a), **W3** (binary, cover, fan, lock, status list, room; V11 and the rest of F6a), **W4** (media, chart, camera, clock, heading, nav, weather, icon set; V12 and F6b), V13 (backdrops, density, mixed families); in parallel B10–B15 | All 24 widget types in four families; all twelve sources |
| **M6 Kiosk** | K1 (security review), K2, K3 (security review), F7 | Paired wall displays with restricted rights |
| **M7 Agent** | A1, approve route and banner, tool manual | The agent builds and edits dashboards |
| **M8 Release** | Config page section, V15 (accessibility and performance), F8, V16, D1, full security review | Version 1 ready for acceptance on the test server |

Order and parallelism:

- M0's three tracks run in parallel. M1 needs V0, V1 and P0 only, so it runs while the backend track and M2
  proceed.
- M3 needs M1 (approved families) and M2 (live data). M4 needs M2 and the primitives. M5's widget batches need
  M3; its source adapters need only B6 and can start right after M2.
- M6 needs B5 and B6 for the backend and F2 and W1 for the page. M7 needs B5.
- Two writers never touch the same files. Adapter packages each add one line to `homedash_sources.go`.

Rules for every package:

- Run GitNexus impact analysis before editing an existing symbol and change detection before each commit.
- New keys go into all 16 locales in the same package, with real translations.
- Work happens in a worktree: the main checkout carries other sessions' uncommitted changes.

After this design is approved, each milestone gets its own implementation plan, starting with M0 and M1.

## 9. Risks

| Risk | Handling |
| --- | --- |
| The control library looks average rather than excellent | Style lab and sign-off before mass production; review on screenshots; hero bar is not lowered. If a family needs more than one revision round, its hero work continues before any batch starts. |
| 188 renderings are a large surface | Primitives carry the family look, so most variants are compositions. The cut list in visual part section 4 stays cut. |
| Tactile detail aliases at 1× (knurling, brushed grain) | Softened gradient stops, minimum pattern period of 2 device px, mandatory 1× sheets in V4. |
| Old wall tablets | Baseline in section 4.4; no `light-dark()`; fallbacks for `linear()`. Tablets below the baseline are not supported. |
| Kiosk opens a public page | Off by default; single-use pairing; per-device revoke; reach limited to the approved revision; security-agent review of K1 and K3. |
| Many unverified upstream field shapes (go2rtc status, system metrics, Uptime Kuma, AdGuard, Proxmox, printers, TrueNAS, Bluetooth) | Each adapter package starts by reading the client it wraps; lists are in backend part section 14. |
| Home Assistant websocket messages are taken from HA's public API, not from this repository | B9a is built against a fake server and then checked once against the live instance on the test server. |
| Screen wake lock needs HTTPS | On plain HTTP the kiosk menu tells the user to keep the screen on in the tablet's settings. |
| Scope: about 50 packages | Milestones each leave a working app; M2 already shows live data, M4 is usable for Home Assistant without kiosk or agent. |

## 10. Deferred

Image backdrops and per-widget backdrops; nested sections and group frames; conditional visibility; free
command actions on tap and hold; kiosk PIN; alarm panels; Home Assistant live camera streams (go2rtc covers
live video); weather forecasts; FritzBox blinds; vacuum commands and humidifier modes (those entities show
their state, humidifiers also toggle); control of printers, Proxmox guests and TrueNAS; Bluetooth connect;
group resize; merge on revision conflict; system clipboard; user templates; bar and multi-series
comparison charts; a persistent time-series store; removing HA Switchboard.

## 11. To verify during implementation

The part documents list what was not confirmed in code. The items that can change a contract:

1. Whether the audit route-contract tests accept a dedicated `/api/desktop/homedash/` contract with DELETE.
2. Whether the inner go2rtc proxy handler keeps every lease and origin check when called from the kiosk MJPEG
   relay (test in K3).
3. Whether a `ui` test may import `internal/homedash`; otherwise both sides read the golden catalog JSON.
4. Whether reverse proxies in front of AuraGo buffer SSE.
5. Tabular figures in the bundled Inter, Geist, Oxanium and Barlow Condensed files (checked in the style lab).
6. How the desktop consumes `Requires` capabilities for app visibility.
7. Packaging of a new root-level kiosk template in the asset pack.
