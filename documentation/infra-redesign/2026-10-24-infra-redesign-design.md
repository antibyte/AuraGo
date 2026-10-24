# Infra redesign: canvas-first homelab atlas

Status: approved by the user on 2026-10-24 (design conversation, German). Branch `feature/infra-redesign`,
worktree `.worktrees/infra-redesign`.

## 1. Problem

The desktop Infra app (`ui/js/desktop/apps/infra*.js`, `ui/css/desktop-app-infra.css`, `internal/infra`,
`internal/server/desktop_infra*.go`) works but does not feel designed:

- The map is whatever is left after five fixed bands (topbar, filter bar, map head, legend footer, unplaced
  tray). In an ~800 px tall window it is ~260 px tall. An empty 340 px inspector column is always present.
- Placement works through buttons ("Place at center", "Place upper left", keyboard move). Nothing can be
  dragged from a list onto the map, nothing can be dropped into a room or rack, relations are created through a
  modal with two unsearchable selects.
- The inspector "Application links" section renders one checkbox per target. Every received MeshCore
  conversation (radio stations such as "Iptingen Room Server") becomes a checkbox in every asset's inspector.
- On the real install (aurago-test, 2026-10-24) 76 of 108 nodes are Docker containers and 5 are Proxmox VMs, so
  "Unplaced" says 99. Registry rows from mDNS/UPnP scans produce duplicates (same IP twice: TheBrain and TheBrain
  Mediaserver, four Chromecasts twice) and opaque names. There is no persisted way to ignore a source.
- Audit bugs: Inventory "Unplaced" shows everything, tray click can freeze repaints, errors vanish on the next
  poll or render behind the modal, layout 409 retries with stale versions, Arrange cannot move pinned nodes and
  nothing unpins, single-level undo wiped by unrelated saves, hard-coded colours that ignore 12 of 14 desktop
  themes, polling announces "Loading…/ready" to screen readers every 30 s.

## 2. Goals and non-goals

Goals:

1. The map owns the window. Chrome is one 44 px command bar plus optional side panels.
2. Direct manipulation everywhere: drag from the library onto the map, into rooms and racks, out again; draw
   relations by dragging; multi-select; multi-level undo/redo; keyboard equivalents for all of it.
3. Only homelab-relevant things ask for attention: containers and VMs live inside their host, duplicates and
   irrelevant discoveries are detected, ignoring is persistent, app links are a searchable picker.
4. Looks native in all 14 desktop themes (light and dark), accessible, localized in all 16 desktop locales.
5. Keep every safety property of the current app: version CAS, drafts that survive polls and close, uncertain
   outcome handling for actions and source edits, read-only and policy gating, abort on hide.

Non-goals: new source adapters (Home Assistant, TrueNAS, Tailscale), rack U-slot modelling, server-side undo
history, map virtualization (the browser suite requires 1000 rendered nodes), changing Registry semantics.

## 3. Information architecture

| Concept | User-facing name (en / de) | Notes |
|---|---|---|
| node | asset / Asset | one word everywhere in the UI; "node" stays an API term |
| site, room, rack, group | place / Ort | containers of assets; drawn as regions on the map |
| edge | connection / Verbindung | "relation" and "Connect" are retired as UI words |
| candidate | discovery / Fund | something a source sees that is not in Infra yet |
| source | source / Quelle | registry, docker, proxmox, fritzbox |
| vm, container | guest / Gast | always shown inside its host |

Views: **Map** (`atlas`), **Topology** (`topology`), **List** (`table`). The internal view ids and
`data-view` values stay `atlas`, `topology`, `table`.

## 4. Layout

```
┌ command bar 44px ───────────────────────────────────────────────────────────────────────┐
│ [logo] Infra  [Map|Topology|List]  [🔍 Search or jump…  Ctrl K] [chips…]  ●Live  ↶ ↷  [+ Add ▾] ⋯ │
├ library ──────┬ canvas (everything else) ─────────────────────────────┬ inspector ──────┤
│ To place   4  │                                                        │ (only when a    │
│ On the map 9  │   regions, asset cards, host stacks, connections       │  selection      │
│  ▾ Aurago 23  │                                                        │  exists)        │
│ Cleanup    6  │ [tools ▸ select hand connect | − 90% + fit]   [minimap]│                 │
└───────────────┴────────────────────────────────────────────────────────┴─────────────────┘
```

- **Command bar** (`.infra-commandbar`, 44 px, never wraps; overflow collapses labels to icons, then moves
  secondary items into the `⋯` menu): brand, view switch (segmented, `role=radiogroup` semantics through
  aria-pressed buttons as today), search field with filter popover and active-filter chips, sync indicator,
  undo, redo, primary `Add` split button (`New asset`, `New place`, `Import discoveries…`), `⋯` overflow
  (Refresh, Arrange, Shortcuts, Legend). Undo/redo are icon buttons with tooltips and remain enabled-looking only
  when usable (disabled with `aria-disabled` and a reason tooltip otherwise).
- **Library** (`.infra-library`, default 264 px, resizable 220–400 px by a `role=separator` splitter with
  ArrowLeft/Right, collapsible to a 40 px rail; width and collapsed state persist in `localStorage` key
  `infra.layout.v1`): sections described in §7.
- **Canvas** (`.infra-canvas`): full bleed, dot grid, floating tool cluster bottom-left, minimap bottom-right
  (toggle, persisted), toast region bottom-centre, conflict/retry chip top-centre.
- **Inspector** (`.infra-inspector`, default 360 px, resizable 320–520 px, persisted): appears on selection,
  closes with `Esc`, the close button, or clicking empty canvas. No empty column ever.
- **Responsive** (container queries on `.infra-app`):
  - ≥ 1100 px: library + canvas + docked inspector.
  - 760–1099 px: library docked, inspector becomes an overlay drawer with scrim and visible close.
  - < 760 px: library and inspector are overlay sheets opened from command bar buttons (`data-action="toggle-
    library"`); nothing is unreachable (fixes the hidden scope/places tree).
  - Below 500 px height the canvas keeps at least 200 px; panels scroll internally.

Space budget at 1500×970 maximized: command bar 44 px, canvas ≈ 900 px tall (was ≈ 450 px). At 800 px window
height the canvas is ≈ 700 px (was ≈ 260 px).

## 5. Canvas

### 5.1 Rendering

- World coordinates in px; one CSS transform `translate(x,y) scale(z)` on `[data-scene]`; zoom 0.25–2.0.
  Initial view: fit all placed assets with 48 px margin, capped at 1.0. Viewport per view persists in
  `localStorage` (`infra.viewport.v1.<view>`).
- Asset card (`[data-node]`, native `<button>`): 184×60, icon 36 px, name 16 px/600 (test floor 16 px), meta
  line 14 px tabular (IP, else role), status ring colour plus a status glyph for non-colour users, power dot.
  Full name in `title` and aria-label. Selected: accent outline 2 px + soft shadow. Hover: raise + show
  connection handles (4 small dots on the card edges).
- Host stack: hosts with guests show a badge `▣ 23 · 18 running`. Clicking the badge toggles an attached guest
  shelf below the card (up to 8 guest chips with status dot, then `+15 more` which focuses the host in the
  library). Guests are never free-floating cards in Map view. `[data-toggle-host-services]` stays the toggle
  hook; `[data-hosted-service]` the chip hook.
- Places are regions: rounded rectangle with a header label (16 px, place icon, child count), drawn from the
  place's own placement rectangle (`x, y, w, h`, see §10.3). A place without `w/h` gets 360×240. The region
  grows to contain its children after a drop (bounds union + 48 px padding, test-pinned frame padding).
  Nested places (rack in room) draw inside. Regions are drag targets and can be moved (moves children) and
  resized (corner and edge handles, min 200×140).
- Connections: SVG paths between card edges (not centres), kind-specific stroke (ethernet solid, fiber double,
  wifi dashed, `runs_on` hidden in Map because guests are nested, `depends_on` dotted, `powered_by` amber).
  Labels (label, else ports and speed) render at the midpoint when zoom ≥ 0.75 and on hover. Connections are
  hoverable and clickable (wide invisible hit path); a selected connection opens in the inspector.
- Rendering is keyed and incremental: nodes, regions and edges are patched by id instead of rebuilding
  `innerHTML`; search highlights dim non-matching cards instead of re-rendering.
- Minimap: scaled overview of regions and cards with the viewport rectangle; drag to pan, click to jump.

### 5.2 Gestures

| Input | Action |
|---|---|
| drag empty canvas | pan |
| Space + drag, middle button drag | pan (any tool) |
| wheel / two-finger scroll | pan |
| Ctrl/⌘ + wheel, pinch | zoom at pointer |
| Shift + drag empty canvas | marquee select |
| click card / Shift-click | select / toggle in selection |
| drag card (or selection) | move; snaps to 24 px grid; release inside a region sets that place as parent, release outside all regions clears the parent; overlap is resolved by nudging to the nearest free slot |
| drag a card's connection handle onto another card | create connection; an inline popover offers kind (default ethernet for devices, wifi when either side is an access point/mesh node), label, ports |
| drag a library row onto the canvas | place at drop point (with parent if dropped into a region); multiple selected rows are placed as a compact grid |
| drag a discovery row (import sheet) onto the canvas | import and place in one step |
| drag region header | move region with all children |
| double-click card | open inspector on Details tab |
| right-click / long-press | context menu (Open, Move to place…, Connect to…, Open in app, Remove from Infra, Remove and ignore) via the shell `showContextMenu` |

Tools (floating cluster, `role=toolbar` with roving tabindex): Select (`V`), Hand (`H`), Connect (`C`, drag
from any card to another without handles). Zoom out/in, zoom percentage (click resets to 100 %), Fit (`F`).

During a drag: a ghost card follows the pointer at the current zoom, the target region highlights with
"Drop into Living room", invalid targets (read-only, policy) show a not-allowed cursor and a reason tooltip.
`Esc` cancels any drag, marquee or connect gesture. Pointer capture with `lostpointercapture` cancellation, rAF
coalescing, `touch-action: none` on the canvas only.

### 5.3 Keyboard model

- The canvas is one tab stop (`role=application`, `aria-roledescription` "map", labelled "Infrastructure map").
  Focus lands on the selected card or the first card in reading order.
- Arrow keys move focus spatially to the nearest card in that direction. `Enter` opens the inspector.
  `Space` toggles selection. `M` enters move mode for the selection (assets, or one place with everything
  inside it): arrows move 24 px (Shift: 8 px), `R` switches a place between moving and resizing (arrows then resize by 24 px, minimum 200 x 140; the browser reserves Shift+Alt for the input language),
  `Enter` commits one layout save. `Esc`, `Tab`, losing focus and a pointer press revert and announce it; only
  `Enter` keeps a keyboard move. `M` with nothing movable selected, or without write access, announces why.
  `P` on a library row or unplaced asset places it at the viewport centre and enters move mode. `Delete` asks
  to remove the selection from Infra.
- Global: `Ctrl K` search/jump, `Ctrl Z` / `Ctrl Shift Z` undo/redo, `Ctrl A` select all visible, `F` fit,
  `+`/`-`/`0` zoom, `V/H/C` tools, `?` shortcut sheet, `Ctrl S` save inspector draft. Shortcuts are ignored
  while typing in inputs.
- A visually hidden live region announces only state changes (selection, placement result, save outcome, sync
  becoming stale or recovering), never routine polls.

## 6. Topology view

- Same canvas engine, `placements.topology`, regions hidden.
- Four labelled horizontal lanes drawn behind the cards: Internet (internet, modem-ont), Network (router,
  firewall, managed switch, access point, mesh node, `network` kind, patch panel), Compute (servers,
  hypervisors, NAS, mini PC, SBC, rack server, kubernetes, `device` kinds with guests), Clients and IoT
  (everything else). Lane membership comes from kind, icon and role through one pure function
  (`topologyLane(node)`) with a manual override (dragging a card into another lane stores its topology
  placement; the lane is derived from y).
- Auto layout: assets without a topology placement are laid out per lane ordered by connection order (parents
  before children along ethernet/fiber/wifi edges), centred; `Arrange` re-runs it for the selection or, with no
  selection, for everything.
- Guests are nested in host stacks as in Map view.

## 7. Library (left panel)

Sections, each collapsible, counts reflect current search and filters:

1. **To place** — assets that need a position in the current view: kind not vm/container/service with a host,
   no placement in this view. Rows are draggable (`[data-library-item]`, `[data-place]` kept on the row's
   "place" button for keyboard users). Empty state: "Everything is on the map".
2. **On the map** — tree: places (site › room › rack › group) with their assets; hosts with their guests
   (collapsed by default, count badge). Selecting a row selects and reveals the asset on the map (pan into
   view). Rows of places are drop targets too (drop a row onto a place row to reparent).
3. **Needs attention** — stale, removed or unknown sources (any source, not only the best), failed saves.
4. **Cleanup** — suggestions computed by `infraCleanup(nodes, sources)` (§9.2): possible duplicates, unnamed
   discoveries already imported, guests whose engine host is unknown. Each suggestion has one primary action
   (e.g. "Remove duplicate and ignore", "Assign engine host") and "Dismiss" (stored per suggestion id in
   `localStorage`, `infra.dismissed.v1`).
5. **Ignored** (collapsed) — ignored sources with "Restore".

The left rail's old scope nav (`All / Unplaced / Needs attention`) becomes these sections; `Places` creation
moves to `Add ▸ New place` and the section header `+` on "On the map".

## 8. Inspector

- Sticky header: icon (click opens icon picker), inline-editable name, kind · place breadcrumb
  (`Home › Basement › Rack 1`, each crumb clickable), state pill and power pill, primary quick action (`Open
  in` for the first app link, else `Edit`), `⋯` menu (Connect to…, Move to place…, Remove from Infra, Remove and
  ignore, source actions).
- Tabs (`role=tablist`): **Overview** (read-only facts: address, MAC, vendor/model, place, host and guests,
  connections summary, linked apps with open buttons, last seen), **Details** (edit form), **Sources**
  (observed sources, actions, source edit), **Connections** (list with kind, other end, ports, edit and remove,
  "Add connection" with a searchable asset picker). Selected tab persists per kind.
- Details form groups: Identity (name, type, icon, place), Network (IP, MAC, CIDR and VLAN for networks,
  interfaces), Hardware (vendor, model, serial, OS, firmware, CPU, RAM, storage, rack position), Links and apps,
  Notes, Custom fields. Groups with no values start collapsed with an "Add" affordance; groups with values start
  open. `[data-detail-form]`, `[data-node-field]`, `[data-list-add]`, `[data-list-row]` hooks stay.
- Save model: one sticky save bar at the bottom appears only when the draft is dirty: "Unsaved changes ·
  Discard · Save in Infra" (`[data-detail-form] button[type=submit]`). `Ctrl S` saves. Field-level dirty
  markers. Draft persistence, conflict compare and "Reapply draft" behave exactly as today.
- Source edit lives in its own `<form data-source-form>` on the Sources tab (fixes Enter-submit and validity
  leaks). Hooks `[data-edit-source]`, `[data-source-field]`, `[data-action="save-source"]`,
  `[data-action="review-source"]`, `[data-action="reapply-source-draft"]`, `.infra-source-edit`,
  `.infra-source-row`, `[data-source-action]`, `[data-run-action]`, `[data-action-rearm]` stay.
- **App links picker** replaces the checkbox wall: bound links are chips with "Open" and "Remove"; "Link an
  app…" opens a combobox (`role=combobox` + listbox) grouped by app with search. Suggestions first (name or
  address similarity). MeshCore conversations are offered only when the asset is a radio/mesh device (icon
  `mesh-node` or `radio-gateway`, or role/tags containing `mesh`) or when the user types at least two
  characters. `[data-app-target][data-app=…]` stays as the option hook.
- A selected connection shows a compact connection inspector (from, to, kind, ports, speed, label, remove).
- A multi-selection shows a selection summary (count by kind) with bulk actions: Move to place, Arrange, Remove
  from Infra, Ignore.

## 9. Relevance and cleanup

### 9.1 Guests in their host

- Server projects a runtime-only `host_id` onto vm and container nodes (reset on read/write like `power`):
  1. Proxmox: the target of the guest's provider `runs_on` edge (guest → host).
  2. Docker: the node mapped to the guest's Docker engine instance in `infra_source_hosts`; if no mapping
     exists, an inferred host: the node whose IP equals the engine's TCP host, or for a local socket/pipe the
     node whose IP is one of the AuraGo server's own interface addresses. Inferred hosts set
     `host_inferred: true`.
- Snapshot adds `source_hosts: [{kind, instance, node_id, version, inferred_node_id}]` for Docker instances.
- UI: guests with a host render inside the host stack. Guests without a host appear under Cleanup ("76 guests
  belong to a Docker engine. Which asset runs it?" with an asset picker), never under To place.

### 9.2 Discoveries and cleanup (client, pure functions in `infra-model.js`)

- `discoveryRelevance(candidate, nodes, candidates)` returns `{group: "recommended" | "unlikely" | "duplicate",
  reasons: [...], duplicateOf?: {nodeId|candidateId, match: "ip"|"mac"}}`. Reasons vocabulary:
  `duplicate_ip`, `duplicate_mac`, `generic_discovery` (registry role `mdns-device`, `dial`, `WFADevice`,
  `Basic` with tags only `network-scan`/`full-scan` and no friendly vendor/model), `inactive` (FRITZ!Box host with
  power `stopped`, Docker container that is stopped and has no host mapping), `link_local` (IPv6 link-local
  only address). Within a duplicate group, the candidate with the most specific name (not opaque, longest
  vendor/model information, registry before fritzbox) is recommended; the rest are `duplicate`.
- Import sheet groups discoveries into Recommended (pre-selected), Possible duplicates, Probably not homelab;
  each row shows the reason in plain language and offers Import, Link to existing asset (`node_id`), Ignore.
  Rows are draggable onto the canvas.
- `infraCleanup(nodes, sources, sourceHosts)` returns suggestions for already-imported assets: duplicates by IP
  or MAC among nodes, unnamed/opaque untouched imports, guests without host.

### 9.3 Ignore (server, persisted)

- `infra_sources.ignored_at`. Ignored sources never appear as candidates, are excluded from the freshness
  ledger ("x of y sources current"), and are listed in the library's Ignored section with Restore.
- "Remove and ignore" deletes the node and ignores all its sources in one transaction.

## 10. Backend changes (additive)

All under `/api/desktop/infra`, same access rules as today (admin scope; writes need desktop write permission
and a writable config; source edit and control keep their own flags).

### 10.1 Schema v3

- `ALTER TABLE infra_sources ADD COLUMN ignored_at TEXT NOT NULL DEFAULT ''`.
- `CREATE TABLE infra_source_hosts (kind TEXT NOT NULL, instance TEXT NOT NULL, node_id TEXT REFERENCES
  infra_nodes(id) ON DELETE CASCADE, version INTEGER NOT NULL CHECK (version > 0), updated_at TEXT NOT NULL,
  PRIMARY KEY(kind, instance))`.
- Migration chain becomes v1 → v2 → v3 with an explicit v2 → v3 branch; the full-create path writes version 3.
  Back up before migrating (existing backup pattern, keep newest three). Tests migrate a v2 database with rows.

### 10.2 Endpoints

| Method and path | Request | Response | Rules |
|---|---|---|---|
| `PUT /sources/ignore` | `{sources:[{kind,instance,key}], ignored: bool}` (1–1000) | `{sources:[Source]}` | idempotent; linked source → 409 `source_linked`; unknown → 404 |
| `DELETE /nodes/{id}?version=N&ignore_sources=1` | — | `{status:"deleted", id, ignored:[identity]}` | delete + ignore in one tx; children → 409 `has_children` as today |
| `POST /import/batch` | `{items:[{candidate_id, node_id?, parent_id?, placement?:{view,x,y,w?,h?}}]}` (1–500) | `{nodes:[Node], errors:[{candidate_id, error, message}]}` | one forced refresh per batch; each item in its own tx; parent must be a container; placement validated |
| `PUT /source-hosts` | `{kind:"docker", instance, node_id ("" clears), version (0 for new)}` | `{source_host}` | CAS on version; node must be device kind |
| `PUT /layout` (extended) | items gain optional `parent_id` (absent = unchanged, `""` = clear) and optional `w`, `h` for place kinds | unchanged | still atomic and version-checked; parent rules as node update (container kind, no cycles) |
| `POST /edges` (tightened) | — | — | self-loop → 400 `invalid_request`; same `(from,to,kind)` as an existing edge → 409 `edge_exists` |
| `GET /snapshot` (extended) | — | adds `source_hosts`, `Source.ignored` flag (ignored rows included only when `?include_ignored=1`), `Node.host_id`, `Node.host_inferred` | runtime projections only |
| `GET /candidates` (extended) | `?include_ignored=1` | ignored candidates carry `ignored: true` | default excludes ignored |

### 10.3 Placement size

`Placement` gains optional `w`, `h` (float64, omitempty). Allowed only for place kinds; bounds 120–20000;
validated in `validatePlacements` and the layout batch.

### 10.4 Agent tools

`internal/agent/agent_dispatch_infra.go` keeps working; snapshot readers ignore new fields. No new agent tool
in this project.

## 11. Frontend architecture

New files in `ui/js/desktop/apps/` (each < 1100 lines, registered in `ui/js/desktop/core/module-loader.js:27`
and in `scripts/test-infra-ui.mjs`), loaded in this order:

| File | Global | Responsibility |
|---|---|---|
| `infra-model.js` | `InfraAppModel` | pure helpers (existing exports kept), viewport math, snapping, free slot, region hit-testing, topology lanes, relevance, cleanup, filter predicate (single copy) |
| `infra-store.js` | `InfraAppStore` | state, API client, polling and visibility, CAS writes, layout queue with conflict rebase, undo/redo command stack, notices, event subscription |
| `infra-canvas.js` | `InfraAppCanvas` | keyed rendering of cards, host stacks, regions, edges, lanes, minimap, selection and drop highlights |
| `infra-gestures.js` | `InfraAppGestures` | pointer and keyboard interaction for canvas and library drags, marquee, connect, move mode, shortcuts |
| `infra-library.js` | `InfraAppLibrary` | left panel sections and drag sources |
| `infra-inspector.js` | `InfraAppInspector` | inspector header, tabs, forms, source edit, app links picker, connection and multi-selection inspectors |
| `infra-table.js` | `InfraAppTable` | List view (virtualized, sortable, tree, bulk actions) |
| `infra-dialogs.js` | `InfraAppDialogs` | import sheet, new asset/place, connection popover, confirmations, shortcut sheet |
| `infra.js` | `InfraApp` | composition root: `render(host, windowId, ctx)`, `dispose(windowId)`, command bar, window menus, wiring |

`infra-views.js` is removed. CSS: `desktop-app-infra.css` (tokens, shell, command bar, panels, inspector,
dialogs) and `desktop-app-infra-canvas.css` (canvas, cards, regions, edges, minimap).

Store contract (sketch): `store.state` (read-only snapshot of nodes, edges, sources, sourceHosts, capabilities,
candidates, ui), `store.subscribe(fn)`, `store.dispatch(command)` for every mutation. Commands are objects with
`apply()` / `revert()` used by the undo stack: `MoveNodes`, `Reparent`, `ResizePlace`, `CreateEdge`,
`DeleteEdge`, `PlaceFromLibrary`. Undo depth 50, cleared per view on structural conflicts.

Layout queue: a single in-flight `PUT /layout` per view; moves made meanwhile coalesce. On 409 the store reloads
the snapshot, reapplies the intended coordinates/parents onto fresh versions once, and resends; a second
conflict leaves the nodes marked unsaved with the chip "Positions not saved · Retry" (`[data-action="retry-
layout"]`). Other write errors surface inline where the action happened (sheet, popover, inspector) and as a
toast with Retry when the origin is gone; the 30 s poll never clears an error.

Shell contracts kept: `window.InfraApp.render/dispose`, `ctx.setWindowMenus`, `ctx.setWindowBeforeClose`,
`aurago:desktop-policy`, poll interval 30000 ms, abort on hide, the `ctx.openApp` context keys.

## 12. Visual language

- Tokens only: surfaces `--vd-theme-app-bg`, `--vd-theme-panel-bg`, `--vd-theme-panel-bg-strong`,
  `--vd-theme-control-bg/hover`, `--vd-theme-overlay-bg`; lines `--vd-theme-border(-strong)`; text `--vd-text`,
  `--vd-theme-muted`; accent `--vd-accent`, `--vd-accent-contrast`, `--vd-theme-accent-soft/border`; status
  `--ds-color-success/warning/danger/info-fg`; radii `--vd-theme-radius-panel/modal`, `--ds-radius-*`; shadow
  `--vd-theme-shadow-panel/modal`; font `--vd-font-ui`; spacing `--ds-space-*`; motion `--ds-motion-*`; focus
  `--ds-focus-ring-width/offset` + `--ds-color-border-focus`. No hex colours in the app CSS except inside
  `color-mix()` fallbacks; the copper accent and `--vd-font` go away.
- Type: section labels sentence case 13 px/600 muted (no uppercase monospace micro-labels), body 14 px, card
  name 16 px/600, numbers tabular. Minimums enforced by the browser suite (14 px chrome, 16 px card labels).
- Density: 8 px grid inside panels, 24 px world grid, 36 px controls (44 px on coarse pointers).
- Motion: 120–200 ms ease-out for panel open, selection, drop settle; all behind `prefers-reduced-motion` and
  the desktop `data-animations` gate (JS animations check the same).
- Empty canvas state: illustration (`/img/infra/empty-atlas.webp`), headline "Start your homelab map", three
  steps (connect sources, import discoveries, drag onto the map), buttons Import discoveries / New asset
  (`[data-action="import-empty"]`, `[data-action="add-empty"]`). A search with no match shows "No asset matches"
  with Clear search instead (fixes the misleading empty state).

## 13. Testing and verification

- Go: store migration v2→v3 with rows, ignore idempotency and exclusion, delete+ignore atomicity, batch import
  partial failure, layout parent changes (cycle, non-container, unparent), placement w/h validation, edge
  duplicate/self-loop, host projection (Proxmox edge, Docker mapping, inferred), route tests for every new
  endpoint incl. read-only and policy refusals.
- Node (`scripts/test-infra-ui.mjs`): new pure helpers (viewport, snap, free slot, region hit-test, lanes,
  relevance, cleanup, filter predicate), file list and line budgets, i18n coverage for all new keys in 16
  locales, no English prose copied.
- Browser (`ui/desktop_infra_browser_test.go`, `ui/desktop_infra_regression_browser_test.go`): updated
  consciously for the new layout (library instead of tray, narrow layout sheets instead of hidden sidebar,
  inspector only on selection), plus new scenarios: drag library row into a region (parent + placement saved in
  one layout call), drag card out of region (parent cleared), connect by handle drag, undo/redo of a move,
  layout 409 rebase, import sheet grouping, app link picker hides MeshCore for a TV, canvas height ≥ 75 % of the
  app at 1500×970. Existing safety scenarios (drafts, conflicts, uncertain actions, abort on close) keep
  passing.
- Visual review: rod screenshots at 1500×970, 1000×700, 430×700 in Standard, Fruity light, Minimal light and
  dark, Orbit, Windows XP; checked by a reviewer against this spec.
- Guards: `TestShippedWebAssetsLoadNothingExternal`, `TestUIAvoidsCookieConsentClassPrefix`,
  `TestFrontend_JS_NoLiteralTranslationFallbacks`, `TestDesktopUsedI18nKeysExistInAllLanguages`, JS line budget.

## 14. Delivery

Four plans, executed in order in `.worktrees/infra-redesign`, each ending with tests green and a commit:

- **Plan A — backend**: schema v3, ignore, delete+ignore, batch import, source hosts and host projection,
  layout parent and place size, edge tightening, Go tests, `documentation/infra.md` API section. Security review
  of the new endpoints.
- **Plan B — canvas foundation**: new file structure, store with layout queue and undo, command bar and shell
  layout, canvas rendering (cards, host stacks, regions, edges, minimap), gestures (pan, zoom, drag, drop,
  marquee, connect, keyboard), minimal library and inspector shells so the app stays usable.
- **Plan C — panels and flows**: full library sections, inspector tabs and forms, app links picker, connection
  and multi-selection inspectors, List view, import sheet with relevance groups, cleanup and ignore flows,
  context menus, shortcut sheet.
- **Plan D — topology, polish, docs**: topology lanes and auto layout, theme and responsive QA across themes,
  accessibility pass, 16-locale translations review, browser suite updates and new scenarios, screenshots,
  manuals `documentation/manuals/{de,en}/29-infra.md` and `documentation/infra.md`.

Agent roster (project CLAUDE.md): plan-agent drafts each plan, maker-agent implements, test-agent verifies,
security-agent reviews Plan A endpoints, docs-agent writes documentation, debug-agent investigates failures.
One combined review per plan (Critical/Important only).
