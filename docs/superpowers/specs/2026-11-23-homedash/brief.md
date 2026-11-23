# HomeDash — design brief (architecture spine)

Status: working brief for the design agents, 2026-11-23. The final design lives in
`docs/superpowers/specs/2026-11-23-homedash-design.md`.

## 1. What the user asked for

A new virtual-desktop app, **HomeDash**: a dashboard to view and control switches, sensors, video streams and
more from **all existing and active sources** in AuraGo (Home Assistant is the main example). The user composes
dashboards in a **drag-and-drop designer**. The UI must be **extremely polished**, with **very high-quality
controls** for switches, sensors and temperature in **several variations and styles**. A top-quality surface is
mandatory, no compromises.

## 2. Decisions the user made (2026-11-23)

1. **Four style families**, each designed completely for every control: Glass (modern), Tactile (hardware),
   Minimal (typographic), Instrument (HUD).
2. **Kiosk mode in version 1**: a dashboard also runs outside the virtual desktop as its own fullscreen page
   (wall tablet), with its own, restricted rights.
3. **All sources in version 1** (not Home Assistant first).
4. **Agent access in version 1**: the AuraGo agent can read, create and change dashboards.

Version 1 is therefore large. The design must split it into work packages that can be built and verified one
after another, each leaving the app in a working state.

## 3. Verified facts (see the research files next to this brief)

- `research-app-anatomy.md` — how a desktop app is registered, built, tested (Docker Manager is the template).
- `research-data-sources.md` — every device/sensor/video source, its endpoints, gates and push channels.
- `research-design-system.md` — themes (14), tokens (`--ds-*`, `--vd-*`, `--vd-mat-*`), reusable pieces, gaps.

Facts that shape the design:

- There is **no unified live device/entity model**. Each source has its own endpoints; many sources have none
  for the UI (agent tools only).
- **Home Assistant** reaches the UI only as the `switch` domain through `/api/desktop/home-assistant/*`, polled
  every 5 s. The backend has REST calls (`HAGetStatesContext`, `HACallServiceContext`, `HAServiceAllowed`,
  `home_assistant.readonly`), no websocket subscription.
- **go2rtc** already offers an embeddable viewer page, snapshots and a `go2rtc.view` scope.
- Push channels: global SSE `/events` (`SSEBroadcaster`, typed events) and the desktop websocket hub
  (`/api/desktop/ws`, cap 8 clients, 64-message buffer, drops on full).
- The desktop has **no grid/drag-and-drop library, no shared switch/slider/gauge/sparkline components, no
  per-app fullscreen or kiosk mode**. Chart.js 4.5.1 and three.js are vendored locally.
- Hard project rules: no external assets; 16 locales, never filled with English; modules under 1100 lines;
  untrusted text through `textContent` only; modals instead of `alert()`; animations gated by
  `prefers-reduced-motion` and the desktop animation switch; apps paint their regions from `--vd-mat-*`
  (adoption guard test); browser tests must fall into a CI shard name range; core shell files are compiled into
  `main.bundle.js` (`npm run build:ui`).
- An older app, **HA Switchboard** (`ha-switchboard`), covers switches only. HomeDash supersedes it in function.
  It stays untouched in version 1.

## 4. Architecture spine (decided by the lead; refine, do not replace)

Three layers:

### 4.1 Entity hub (backend, new package `internal/homedash`)

A unified, live entity model over all sources.

```
Entity {
  id            "ha:light.living_room"   // "<source>:<native id>", stable
  source        "ha" | "fritzbox" | "mqtt" | "go2rtc" | "system" | "docker" | "proxmox" | "printer3d" | ...
  kind          switch | light | sensor | binary_sensor | climate | cover | fan | lock | media_player |
                camera | scene | button | number | select | service_status | ...
  name, area, device_class, unit, icon_hint
  state         { value, attributes{...}, available, updated_at }
  capabilities  [toggle, brightness, color_temp, color, set_temperature, hvac_mode, position, open_close,
                 stop, press, set_value, select_option, stream, snapshot, history, ...]
  readonly      bool   // derived from the source's gates
}
```

- **Source adapters** implement one Go interface: status, entity catalog, watch (push or poll loop), command.
- The hub keeps the latest state, fans it out to subscribers with **coalescing** (latest wins per entity), and
  polls **on demand** (only sources that have subscribed entities), reacting to config reloads.
- **Commands** go through one endpoint, are validated against capability, the source's read-only gate and
  allow/deny lists, rate limited and audited.
- **History** for sparklines/charts: Home Assistant history passthrough where available; otherwise a bounded
  in-memory ring of numeric samples since process start. No new time-series database.
- **Live transport** to the browser: a dedicated HomeDash stream with per-client subscriptions (only the
  entities on the visible dashboard), snapshot on connect, heartbeat, reconnect. Not the desktop hub (too small,
  drops). The lead leans towards a websocket for subscriptions plus REST for commands; the backend agent
  decides after checking existing patterns.

### 4.2 Dashboard documents (persisted, versioned JSON)

```
Dashboard {
  id, slug, name, icon, schema_version, revision (optimistic concurrency), updated_at
  look    { family: glass|tactile|minimal|instrument, palette: theme|dark|light|custom{...},
            backdrop{...}, density, radius_scale }
  grid    { columns, row_unit, gap, fit: scroll|fit_screen }
  pages   [ { id, name, icon, widgets: [ Widget ] , layouts: { lg, md, sm } } ]
  kiosk   { enabled, idle_dim, page_rotation, ... }
}
Widget {
  id, type, variant, family? (override of look.family), title?,
  bindings { primary: entity id, ... }, options { ... }, actions { tap, hold }, confirm?: bool
}
```

- The **widget catalog** (types, variants, allowed entity kinds per binding, option schema, min/default size)
  has one source of truth in Go, so the server and the agent tool can validate documents; the frontend widget
  registry must implement every catalog type (contract test).
- The server validates every document on save. The agent tool writes the same documents.

### 4.3 Frontend engine (shell-independent) with two hosts

- **Engine** `ui/js/homedash/*`: state store + stream client, grid engine (pure logic), renderer, widget
  runtime, control primitives, the four style families, the designer. It must not depend on the desktop shell.
- **Host A — desktop app** `ui/js/desktop/apps/homedash*.js`: window chrome, menus, wiring, materials.
- **Host B — kiosk page**: a standalone fullscreen page that loads the same engine in view mode.
- **Style tokens `--hd-*`**: widgets consume only `--hd-*`. A style family defines geometry, materials, light
  model and motion; a palette defines colours. Inside the desktop the palette `theme` is a bridge from
  `--vd-mat-*` / `--ds-*` to `--hd-*`; in kiosk `theme` resolves to a neutral dark/light palette. Each widget
  type offers several **variants** (structure); each variant is drawn by the active **family** (material).
  Controls are composed from shared **primitives** (track, thumb, knob, dial arc, needle, numeral display,
  label, icon badge, card frame) so four families stay consistent and affordable.

### 4.4 Quality mechanism (non-negotiable)

- **Style lab first**: reference controls for all four families are designed and judged before the widget
  library is mass-produced. The lead session makes the visual judgement itself.
- **Control lab harness**: a gallery that renders every widget type x variant x family x state (off, on,
  pending, disabled, unavailable, error, min, max) in several palettes and desktop themes, for screenshot
  sheets and browser tests.
- Explicit acceptance criteria per control: hit target >= 44 px, focus-visible ring, WCAG 4.5:1 for text,
  keyboard operation with correct ARIA role, reduced-motion alternative, crisp at 1x and 2x, optimistic
  feedback with rollback on failure, no layout shift on state change.

## 5. Roster and workflow constraints

Implementation later uses the user's agent roster: plan-agent (architecture), reader-agent (reading),
maker-agent (all code), test-agent (running tests), docs-agent (documentation), security-agent (security
review), debug-agent (failures). Work packages must therefore be sized for a maker-agent: a few files, a clear
contract, named tests. Keep verification lean: the essential tests per package, one combined review per batch.

Project rules for editing later (not for this design phase): GitNexus impact analysis before editing existing
symbols, detect-changes before commits, commit after every change.
