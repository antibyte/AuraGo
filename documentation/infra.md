# Infra: the homelab map

Infra is the Desktop's shared inventory map for hardware, virtual machines,
containers, applications and network segments. The Map, Topology and List views
show the same assets. Positions belong to the AuraGo instance; view, selection
and navigation belong to the current window.

## Configuration

Open Configuration → Virtual Desktop → Infra. Infra is enabled by default when
the Desktop is enabled; an explicitly saved disabled setting remains respected.
Local editing, source editing and device control are separate capabilities:

```yaml
virtual_desktop:
    infra:
        enabled: true
        read_only: false
        allow_source_edit: false
        allow_control: false
```

All Infra API access requires a Desktop administrator. Desktop read-only mode
and Infra read-only mode block edits and actions. Source edits and control also
require their own Infra grant and the source's existing permission. Enabling
Infra does not enable Docker, Proxmox, FRITZ!Box or any other integration.

## Build your map

Infra never adds an asset by itself. **Import discoveries…** [`desktop.infra_import_discoveries`] is in the menu
next to **New asset** [`desktop.infra_new_asset`] and on the empty map. It opens a sheet beside the map that groups
what the configured sources see into **Recommended** [`desktop.infra_import_recommended`] (pre-selected),
**Possible duplicates** [`desktop.infra_import_duplicates`] and **Probably not homelab** [`desktop.infra_import_unlikely`],
each row with its reason. Import a row, link it to an existing asset, ignore it, or drag it onto the map to import and
place it in one step. Linking is always explicit; matching names and addresses are suggestions, never identity. Manual
device assets create a registry device with protocol `none`; assign SSH/VNC and credentials deliberately through the
existing registry and credential UI. VMs, containers, services, places and network segments do not create registry
devices. A VM or container can be explicitly linked to an existing registry device to select its Quick Connect target.

Discovered Registry and FRITZ!Box devices with blank/default names or only a hexadecimal identifier, UUID or MAC
address are not offered as discoveries. Give them a recognizable name in their source to make them available. This
filter also hides untouched, unplaced imports with these names. Registry data and saved assets remain stored;
edited, placed, grouped and connected assets remain visible.

Sites, rooms, racks and groups are places. On the map a placed place is a region: dropping an asset into it makes
the place its parent, dragging the asset out clears the parent, and the region grows to contain its assets. VMs and
containers are shown inside their host: a Proxmox guest through its `runs_on` connection, a Docker container
through the device assigned to its engine. **Cleanup** [`desktop.infra_cleanup`] asks for that device when Infra
cannot infer it. The Topology view draws four lanes, labelled Internet, Network, Servers and storage and Clients and IoT in the
English UI (see [Topology lanes](#topology-lanes)). Assets without a topology position are laid out automatically,
and dragging a card into another lane keeps it there. In Topology, **Arrange automatically**
[`desktop.infra_arrange_auto`] hands the selection, or every card, back to the automatic layout; on the map,
**Arrange** [`desktop.infra_arrange`] re-runs the grid. The List view is a sortable tree of the same assets.

Draw a connection by dragging a card's handle onto another card, or with the Connect tool (`C`). Ethernet, Fiber,
Wi-Fi, Member of, Runs on, Depends on and Powered by stay distinct; a shared subnet does not imply a cable. Layout
changes are saved per view and can be undone and redone. Refreshing source data preserves positions. Cleanup also
suggests duplicate assets and unreadable names; **Remove and ignore** [`desktop.infra_remove_and_ignore`] deletes an
asset and ignores its sources, so they are not discovered again until they are restored under **Ignored**
[`desktop.infra_ignored`].

The details panel separates **Save in Infra** [`desktop.infra_save_in_infra`] (Details tab, sticky save bar,
Ctrl+S) from the source edit on the Sources tab. Local profiles hold hardware details, network interfaces,
CIDR/VLAN, location, management links, notes and custom fields. Blank information remains unknown. Saving local
details never changes an existing registry device or its Quick Connect destination; use the source editor for those
changes. Failed saves and version conflicts keep the draft rather than silently overwriting another editor's
changes. Closing a window with unsaved local, source or layout changes asks whether to discard them or keep editing.

App links are chosen in a searchable picker: Quick Connect selects a registry device without connecting or
replacing the current session, Network Cameras a configured camera, Software Store an installed app and MeshCore a
confirmed conversation. MeshCore conversations are offered only for radio or mesh assets, or once two characters
are typed. Opening these targets does not dispatch power actions.

## Source edits and actions

| Source | Editable source metadata | Control |
| --- | --- | --- |
| Registry | Existing device and connection fields, credential reference | Open Quick Connect |
| Docker | Container name | Start, stop, restart |
| Proxmox | QEMU name / LXC hostname, description, tags | Start, graceful shutdown, reboot |
| FRITZ!Box | Supported host / friendly name | Wake-on-LAN |

Two gates apply on top of the table. Proxmox graceful shutdown and reboot also
require `proxmox.allow_destructive`; without it they are reported as blocked by
source policy. Docker stop/restart and Proxmox shutdown/reboot require the
explicit confirmation of the action dialog, and the API refuses them without
`confirmed: true` (400 `confirmation_required`). A managed container
(go2rtc, RuView, Security Proxy, Decisions, local LLM, ACE-Step, Garage,
Homepage, CYD Creator and AuraGo itself) is shown as protected and answers an
action with `status: "failed"` and `reason: "managed_container"`.

Source forms expose only supported fields. Docker labels are not changed by
recreating a container. Protected and AuraGo-managed containers retain their
existing restrictions, including the rename prohibition. LXC hostname changes
have an explicit warning. Proxmox config digests protect against concurrent
source edits. Pending Proxmox metadata is explicitly marked as pending;
Infra retains the draft until the source confirms the effective change.

Docker, Proxmox guests and FRITZ!Box hosts show a separate power reading:
running, stopped, paused or unknown. It comes from the latest current
observation. A stale source does not keep a power claim. The reading is not
stored in the local role field, and an older imported role is left as written.
FRITZ!Box power means the host was active on the LAN, not that Wake-on-LAN
finished. A Proxmox task acknowledgement and a sent Wake-on-LAN packet mean
accepted, not confirmed device startup. An uncertain operation is never automatically retried. Request
IDs and outcomes survive server restarts, so replay does not repeat an uncertain
mutation. Inspect the source before issuing a new operation after uncertainty.
Revoking Infra or integration permissions cancels affected requests. Operations
that have not been sent are blocked; cancellation after dispatch does not prove
that the device action was stopped and can leave its outcome unknown.

## Persistence and privacy

The `infra_*` tables share the existing Inventory SQLite database. Infra backs
up that database before its additive migration, keeps its own schema version,
and does not change the registry's SQLite version. Deleting an Infra entry does
not delete the source device, container or guest. Linked last-known source data
survives outages and removal; stale and removed are visible states. A failed or
partial enumeration never proves an object was deleted.

Identities use registry IDs, full Docker IDs, Proxmox instance plus guest type
and VMID, or a server-owned opaque FRITZ!Box reference. Proxmox node migration
changes routing, not identity. Names and addresses do not merge identities.

Only bounded, typed projections reach the graph. Credentials, raw provider
responses and Docker environment variables are excluded. Management links must
be HTTP(S) addresses without embedded credentials. Opening Infra immediately
loads the saved map while one bounded background refresh updates configured
sources. Visible windows pick up these observations about every 30 seconds;
**Refresh** waits for fresh source results. Hidden or closed windows stop
requesting refreshes. Infra does not run network scans or continuous monitoring.

The store is initialized when the server starts, so its additive migration and
the one-time database backup never run inside a request. A provider that cannot
be read is reported as a sanitized code (`provider_unreachable`,
`auth_failed`, `observation_limit`) in the snapshot's `source_errors` map and
the affected sources stay stale; provider error text is never forwarded.

## API overview

All routes require a Desktop administrator session or an admin bearer token and
answer `no-store` JSON. Writes additionally require the Desktop write
permission and a configuration that is enabled and not read-only. Request
bodies are limited to 256 KiB and unknown JSON fields are rejected.

| Route | Purpose |
| --- | --- |
| `GET /api/desktop/infra/snapshot` | Stored graph plus `capabilities`, `source_hosts` and optional `source_errors`; `?refresh=1` waits for providers, `?include_ignored=1` adds ignored sources (`ignored: true`) |
| `GET /api/desktop/infra/candidates` | Import preview for configured sources; ignored sources only with `?include_ignored=1`, flagged `ignored: true` |
| `GET /api/desktop/infra/targets` | Selectable app targets (camera, Store app, MeshCore conversation) |
| `POST /api/desktop/infra/nodes` | Create a node (manual devices also create a `protocol: none` registry row) |
| `PUT`/`DELETE /api/desktop/infra/nodes/{id}` | Update (version CAS) or delete a node; `?version=` required for delete; `?ignore_sources=1` also ignores every linked source in the same transaction and answers `ignored` |
| `GET`/`PUT /api/desktop/infra/nodes/{id}/source-edit` | Read or write the source's own metadata |
| `POST /api/desktop/infra/nodes/{id}/actions` | Reserved power action with `request_id` and optional `confirmed` |
| `POST /api/desktop/infra/import` | Import a candidate or link it to `node_id` |
| `POST /api/desktop/infra/import/batch` | Import or link 1–500 `items` (`candidate_id`, optional `node_id`, `parent_id`, `placement {view, x, y, w, h, pinned}`); one forced refresh per provider, one transaction per item; answers `nodes` and per-item `errors` (`candidate_id`, `error`, `message`) |
| `PUT /api/desktop/infra/sources/ignore` | Ignore or restore 1–1000 sources: `{sources: [{kind, instance, key}], ignored}`; a linked source answers 409 `source_linked` |
| `PUT /api/desktop/infra/source-hosts` | Assign (`node_id`) or clear (`""`) the device that runs a Docker engine: `{kind: "docker", instance, node_id, version}`, version 0 creates; answers `source_host` |
| `POST /api/desktop/infra/edges`, `PUT`/`DELETE /edges/{id}` | Manual connections (provider edges are read-only); a self-loop is invalid (create and update) and a second `(from, to, kind)` answers 409 `edge_exists` (create, or an update that changes from, to or kind; an update that keeps them stays allowed beside a legacy duplicate) |
| `PUT /api/desktop/infra/layout` | Atomic layout batch; items may set `parent_id` (absent keeps it, `""` clears it) and `w`/`h` for places (120–20000), or `unplace: true` (with `id`, `version` and optionally `parent_id`, never a position or size) to drop the node's placement in the request's `view`; answers the changed nodes only |

An `unplace` item removes just that view's placement and advances the node's
version. Its optional `parent_id` behaves as on any layout item (absent keeps
the parent, `""` clears it, otherwise it must be a container without a cycle),
so undoing a drop into a room removes the placement and restores the previous
parent in one atomic call. A node without a placement in that view and without
`parent_id` is left untouched (its version is still checked and does not
advance). Like every layout item it is version-checked, and the whole batch
stays atomic.

VMs and containers carry the runtime-only `host_id`, with `host_inferred: true`
when the host was guessed. A Proxmox guest uses its provider `runs_on` edge, a
Docker container uses its engine's `source_hosts` mapping, and otherwise the
only device whose address matches the configured engine: the IP of a TCP
endpoint, or one of this server's own addresses for a local socket, pipe or
loopback endpoint. Host names are never resolved. Each `source_hosts` entry
(`kind`, `instance`, `node_id`, `version`, `inferred_node_id`) covers a stored
mapping or an observed Docker engine. Only devices can host an engine, so changing a
device into another kind deletes its mappings in the same save and its guests
fall back to the inferred host. Ignored sources stay stored, survive
refreshes and the removal retention, and never appear as candidates by default;
importing or linking one clears the flag.

Stable error codes include `infra_disabled`, `infra_readonly`,
`infra_policy_changed`, `invalid_request`, `request_too_large` (413),
`not_found`, `conflict`, `has_children`, `edge_exists`, `source_unavailable`,
`source_linked`, `source_stale`, `source_readonly`, `source_edit_disabled`,
`confirmation_required`, `request_id_conflict`, `action_disabled`,
`invalid_candidate_id`, `invalid_node_id` and `provider_unavailable`.

Infra is an operator surface: the agent has no tool that reads or writes the
map. The registry it links to stays reachable through the existing inventory
tools.

## Desktop app architecture

The Infra window is plain JavaScript in `ui/js/desktop/apps/`, loaded in this order by
`ui/js/desktop/core/module-loader.js`. Every file is an IIFE that sets one global, stays below 1,100 lines and
touches no DOM while loading, so `scripts/test-infra-ui.mjs` can run all of them in a Node `vm`.

| File | Global | Responsibility |
| --- | --- | --- |
| `infra-model.js` | `InfraAppModel` | Pure helpers: viewport math, grid snapping, free slots, regions and drop rules, guests, the filter predicate, the scene builder, edge state, layout intents and history, discovery relevance, cleanup, List sorting |
| `infra-lanes.js` | `InfraAppLanes` | Topology helpers: the four lanes, the lane of an asset, the automatic layout, the lane of a pinned card, lane stripes and the connection paths |
| `infra-store.js` | `InfraAppStore` | State, API client, polling and visibility, CAS writes, the layout queue with one conflict rebase, undo and redo, ignore, batch import, engine hosts |
| `infra-picker.js` | `InfraAppPicker` | Searchable combobox for app links, places and assets |
| `infra-canvas.js` | `InfraAppCanvas` | Keyed rendering of cards, host stacks, regions, connections, topology lanes and the minimap |
| `infra-gestures.js` | `InfraAppGestures` | Pointer and keyboard input: pan, zoom, drag and drop, marquee, connect, move mode, Arrange, shortcuts |
| `infra-library.js` | `InfraAppLibrary` | Left panel sections and drag sources |
| `infra-cleanup.js` | `InfraAppCleanup` | Cleanup and Ignored sections |
| `infra-inspector-forms.js` | `InfraAppInspectorForms` | Details and Sources forms, drafts, save bar, app link picker |
| `infra-inspector.js` | `InfraAppInspector` | Details panel header, tabs, connection and multi-selection views |
| `infra-table.js` | `InfraAppTable` | List view: virtualized, sortable tree with bulk actions |
| `infra-dialogs.js` | `InfraAppDialogs` | Dialogs, confirmations, the connection popover |
| `infra-import.js` | `InfraAppImport` | Import sheet with relevance groups |
| `infra-menus.js` | `InfraAppMenus` | Context menus, shortcut sheet, legend |
| `infra.js` | `InfraApp` | Composition root: `render(host, windowId, ctx)`, `dispose(windowId)`, command bar, event routing |

The styles are `ui/css/desktop-app-infra.css` (shell, panels, dialogs) and `ui/css/desktop-app-infra-canvas.css`
(map). They use desktop theme tokens only (`--vd-theme-*`, `--vd-*`, `--ds-*`). The guard in
`scripts/test-infra-ui.mjs` rejects:
- literal colours outside `color-mix()`, and named colours;
- theme-id selectors;
- `--vd-font`;
- uppercase and monospace labels;
- motion outside `@media (prefers-reduced-motion: no-preference)` combined with the desktop *Window animations*
  gate.

### Coordinates

- World coordinates are CSS pixels at zoom 1. `[data-scene]` carries the only transform, `translate(x, y) scale(z)`, with z from 0.25 to 2.
- An asset placement `(x, y)` is the centre of its 216 × 64 px card.
- A place placement (site, room, rack, group) `(x, y)` is the top-left corner of its region, and `w`/`h` is its size.
  - Unset size: 360 × 240.
  - The app resizes to at least 200 × 140; the server accepts 120–20,000.
  - The drawn region is that rectangle grown to contain its children plus 48 px.
- Drops snap to a 24 px grid. A drop onto another card moves to the nearest free grid slot; a stacked card lands 72 px away.
- The Map (`atlas`) and Topology (`topology`) views have separate placements. Places are drawn only on the map.
- Semantic zoom: below zoom 0.65 a card turns compact (icon and name, without the meta line). The compact form
  changes no geometry. Names, region headers and lane labels are counter-scaled so they stay readable on screen.

### Topology lanes

| Lane | Index | World y | Assets |
| --- | --- | --- | --- |
| Internet | 0 | 0–383 | icons `internet`, `modem-ont`; role words modem, ONT, ISP, WAN, DSL |
| Network | 1 | 384–767 | kind `network`; icons `router`, `firewall`, `vpn`, `managed-switch`, `switch`, `patch-panel`, `access-point`, `mesh-node`, `lan`, `vlan`, `wlan`; role words router, firewall, VPN, switch, gateway, mesh, AP, access point |
| Servers and storage | 2 | 768–1151 | icons `rack-server`, `tower-server`, `mini-pc`, `sbc`, `nas`, `hypervisor`, `kubernetes`, `disk-shelf`, `storage-share`, `backup`, `database`, `nvr`, `ups`, `pdu`; role words server, NAS, disk shelf, storage, database, NVR, recorder, rack, UPS, PDU, hypervisor, Proxmox, ESXi, Kubernetes, k8s; then any asset that runs guests |
| Clients and IoT | 3 | 1152–1535 | everything else, for example cameras, printers, TVs, phones, tablets, laptops and sensors |

- `topologyLane(node)` is the one mapping. It checks the `network` kind, then the icon, then words in the role (camel case is split, so `InternetGatewayDevice` reads as "internet gateway device"), then whether the asset runs guests. Anything else is a client.
- The lanes are full-width stripes around x = 0 (`LANE_EXTENT`). They are decoration: they never take part in scene bounds, fit, the minimap or hit tests. Their labels are sticky (`--infra-lane-x`): they stay 12 px inside the left edge of the view while you pan. They paint above the connection layer and below the cards.
- A card saved `pinned: true` is in the lane its y falls in (`floor(y / 384)`, clamped to 0–3). Dragging a card into another lane therefore overrides the mapping.
- Assets without a topology placement are drawn at computed slots, and nothing is saved until a card is dragged, moved with the keyboard or arranged. Opening Topology never writes.
  - The slots depend only on the assets and their connections, so every window shows the same layout and moving one card never shifts another.
  - A slot under a placed card moves to the nearest free grid point.
  - Places, guests, and VMs or containers without a host get no slot.
- A saved `pinned: false` placement, left over from the retired layered Arrange, is ignored in Topology; its asset is drawn at its auto slot.
- The auto layout walks Ethernet, Fiber and Wi-Fi connections breadth-first, numbering the assets so that parents come before children. Each lane is then ordered by the barycentre of its assets' connected neighbours in the other lanes (one sweep down, one sweep up), which reduces crossings. Unconnected assets go last. The order is deterministic.
- Each lane holds two auto rows: the first row's centres sit 96 px below the lane top, the second 240 px below it. Cards in a row are 240 px apart and centred on x = 0. A lane with more than 16 assets widens its rows (columns = max(8, ceil(n / 2))) instead of growing taller.
- A dragged or keyboard-moved card is saved `pinned: true`, and its lane follows where it was dropped. The lane is highlighted while you drag. The first move of an auto-laid card can be undone back to the automatic layout.
- Arrange in Topology (`desktop.infra_arrange_auto`) hands the selection, or every card with a saved Topology position, back to the automatic layout. It is one undoable `PUT /layout` of `unplace` items, so Undo restores the previous placements. It is disabled when the server cannot unplace (`store.features.unplace` is false).

### Topology connections

- `topologyPaths` produces every Topology path, so the visible line, its hit area and the selection highlight always agree.
- Between lanes: a cubic curve from the bottom edge of the upper card to the top edge of the lower card, with vertical tangents.
- Inside one lane: the straight segment between facing sides when no other card is in the way. Otherwise it is an arc above the row, from the top edge of one card to the top edge of the other, so it never runs behind a card in between.
- Connections that share a card edge spread over the middle 60 % of that edge.
- Ethernet, Fiber, Wi-Fi, Member of and Runs on are always drawn in Topology. Powered by and Depends on have no hit area and stay hidden until one of their ends is selected, hovered or focused, or the connection itself is selected. The Map draws every kind. The inspector's Connections tab lists all of them.
- A selection makes the connections that touch it accent-coloured and draws them above the others; the other visible connections fade to 35 %. A connection with a dimmed (filtered-out) end is dimmed like the cards.

### Layout saves and undo

- **Commands:** every position, parent or size change is a command dispatched to the store.
- **One save per view:** the store keeps one `PUT /layout` in flight per view and merges moves made meanwhile into the next request.
- **Conflicts:**
  - A 409 reloads the snapshot and resends the same intents onto the fresh versions once.
  - A second conflict leaves the cards marked unsaved behind **Positions not saved · Retry**.
  - A conflict clears that view's undo history.
- **Undo depth:** undo and redo hold 50 steps per view. Placing from the library and Arrange are undone with `unplace` layout items, which restore the previous parent (when the server supports `unplace`).
- **Unknown fields:** asset drafts never gain fields the snapshot did not send. Layout items carry `parent_id`, `w`, `h` and `unplace` only when the snapshot has `source_hosts`.

### Browser storage

Each window keeps view preferences, never inventory data, in `localStorage`:
- `infra.layout.v1`: library and details panel width, collapsed library, minimap;
- `infra.viewport.v1.atlas` and `infra.viewport.v1.topology`: centre and zoom;
- `infra.inspector.v1`: selected tab per kind;
- `infra.table.v1`: List sort;
- `infra.dismissed.v1`: dismissed Cleanup suggestions.

### Accessibility

- The map is one tab stop (`role="application"`); arrow keys move between cards. The library tree and the List view are the accessible lists of the same assets. The Topology lanes are visual only.
- The canvas tools are the one toolbar (`role="toolbar"`), with a roving tab stop and arrow, Home and End keys. The details panel tabs follow the tab pattern, and pickers are ARIA 1.2 comboboxes.
- A polite live region (`[data-announce]`) announces state changes only: selection, placement, save outcome, and sync lost or back. Banners and inline errors use `role="status"` and `role="alert"`. A poll never announces, and a poll that keeps failing reports its error once; an explicit Refresh always reports.
- Closing a panel, sheet, picker or dialog gives focus back to the control that opened it. A menu item hands focus to the trigger of its menu.
- Motion follows `prefers-reduced-motion` and the desktop *Window animations* setting.

## Local assets and development

The 64 selectable Imagegen miniatures, Infra app icon and empty-atlas artwork
are local WebP assets under `ui/img/infra/`. Their manifest and prompt provenance
are versioned with the UI. Using Infra never requires an image generator.
The map uses focusable DOM nodes and SVG connections, without a 3D runtime.

The package contract is [internal/infra/AGENTS.md](../internal/infra/AGENTS.md).
Before changing this flow, run `node scripts/test-infra-ui.mjs` (pure helpers, store, CSS guard, translation
coverage and terminology), `node scripts/test-infra-assets.mjs`, the focused Infra package and server tests
(`go test ./internal/infra/... -count=1`, `go test ./internal/server/ -run Infra -count=1`), the UI guards and
`AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopInfra -count=1 -timeout 40m`. The browser suite covers
the map, gestures, panels, topology, accessibility and contrast in the review themes. Local fixtures do not establish live-provider or hardware compatibility;
that acceptance requires an explicitly authorized deployment and device test.
