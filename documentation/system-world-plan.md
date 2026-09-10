# System World — cinematic data metropolis

## Agreed direction

Replace the orbital presentation with a living, cinematic future city:
dark metal, glass, warm interiors, controlled accent lighting and atmospheric
depth. The interface exposes substantially more real system information.

The user selected:
- A persistent **24-hour** history.
- Overview, orbit exploration, map and focused entity views.
- An optional guided tour **and street-level WASD navigation**.
- High-quality original assets created in Blender through MCP, with a compact
  final runtime payload.

Asset authoring is complete in `assets/system-world/`, with 17 designs and
three separately loadable GLBs per design under `ui/3d/system-world/v1/`.
See the asset README for the verified export and integration contracts.
The first integration stage is implemented: isolated renderer, seven fixed
districts, cached/instanced GLBs, PBR lighting, shadows, bloom, orbit/focus,
street/touch controls, explicit tour, accessible map and theme-native inspector.
Existing REST/SSE sources share one browser read model across open windows;
source status and timestamps remain visible. Search, bounded live events,
entity details and selected KG neighbourhoods are available in all 16 locales.

Still planned: the server-side read model and persistent 24-hour history,
timeline/replay, metric charts, deeper relationship-driven traffic, ambient
occlusion and the large-installation stress acceptance. The current event feed
is session-local metadata; it is not a substitute for recorded history.
The sections below retain the complete target and its remaining acceptance.

Current checks: `node scripts/test-system-world.mjs`,
`node scripts/build-system-world.js --check`, focused UI tests and the
`AURAGO_SYSTEM_WORLD_MATRIX=1` real-shell Chrome check (also enable
`AURAGO_RUN_BROWSER_SMOKE=1`). Browser artifacts are written only to ignored
`reports/system-world-integration/`.

## City and presentation

| District | Architectural identity | Actual information |
|---|---|---|
| Agent | Central reactor spire | Working/waiting/tool phase, active provider/model, available usage and cost metrics |
| Infrastructure | Compute foundry, cooling equipment and service racks | CPU, memory, disks, network rates, containers and daemons |
| Integrations | Gateways and category districts | Configured/enabled state, verified connection health, activity and last update |
| Missions | Logistics terminal and transit | Queue, running/completed/failed missions, schedules and co-agents |
| Knowledge | Branching knowledge atrium | Memory categories/counts, graph relationships, selected entity neighbourhood |
| Operations | Signal station | Sanitized operational issues, severity, recurrence and relevant events |

Use stable entity IDs and deterministic city slots. Updates change existing
objects; they do not reshuffle the city or reset the camera. Large installations
aggregate distant objects with visible counts and searchable detail lists.

The compact overlay follows Desktop materials and common icon rendering:
metrics/search/view controls at the top, district navigation on the left, an
inspector on the right and a collapsible timeline below. The city remains dark
in all three Desktop themes; its surrounding chrome follows Standard and Fruity
light/dark. Preserve Fruity global menus and all 16 Desktop translations.

The inspector has Overview, Metrics, Relationships and Events sections, with
small charts using the existing Chart.js. Every value identifies its source,
timestamp and availability. Disabled, disconnected, unknown and stale are
distinct states. Existing integration booleans do not establish online health.

Camera modes:
- Orbit overview and searchable entity focus.
- Accessible 2D map and list.
- Explicit guided tour; any user navigation immediately stops the tour.
- Street mode with WASD, mouse look activated by an explicit pointer-lock action,
  Escape to release, simple building collision and a safe return to overview.
  Touch uses drag look and movement controls; map destinations remain available.
- No automatic camera takeover after inactivity. Reduced motion disables ambient
  camera movement and decorative pulses.

## Renderer and asset integration

Use an isolated ESM bundle of **Three.js 0.185.1** with matching addons.
Do not replace the shared r128 global used by other apps. Keep Vanilla JavaScript
and local, permissively licensed resources; no paid asset-generation dependency.

The visual pipeline combines PBR materials, environment reflections, soft
shadows, ambient occlusion, selective bloom and restrained atmospheric fog.
Keep work overlays sharp. Depth of field may be used during the optional tour,
but is disabled during reading and navigation.

Load the supplied LODs independently, instance repeated geometry, share materials
and cap visible effects. Dynamic lights and high-resolution shadows receive
explicit per-tier budgets. Provide Low/Medium/High/Ultra and an automatic
frame-time-based choice with hysteresis. No per-frame geometry/material allocation.

Preserve one render loop per visible window, pooled effects, context-loss
handling and complete disposal. Hidden/minimized windows stop rendering.
Without WebGL2, keep the map, metrics, history and inspector usable.

## Read model and 24-hour history

Reuse the existing server-wide ten-second system-metrics collector, SSE and
the existing mission, activity, memory, budget and operational-issue stores.
Avoid a separate OS polling loop for each window.

Add authenticated read-only endpoints beneath
`/api/desktop/system-world/`:
- `snapshot`: stable entity IDs, relationships, metrics, source state and revision.
- `history`: bounded metric series and recorded city-state history for up to 24 hours.
- `events`: paginated, filtered, sanitized event metadata.
- `entity`: bounded detail for the selected permitted entity and timestamp.

Reuse existing read permissions. SSE announces revisions; authorized snapshot
requests fetch sensitive detail rather than broadcasting it to all listeners.
Missing or failed sources retain labelled stale values, never invented zeroes.

Persist minute-level metric aggregates (minimum/maximum/average/sample count),
bounded typed state changes and periodic checkpoints in the existing Desktop
SQLite database. Keep 24 hours, collect while the Desktop feature is enabled
even when no System World window is open, and stop cleanly with the server.
Existing histories remain authoritative where available; do not duplicate raw
prompts, tool payloads, mission output, credentials or graph contents.

The timeline offers Live, pause, scrubbing and replay. Record timestamps in UTC,
display local time and show gaps after downtime or initial installation.
Replay reproduces recorded states and events, not undocumented historical
knowledge-graph contents. Network rates use timestamped counter differences and
handle restarts/reset counters without spikes.

Traffic follows recorded entity relationships. Unknown tool destinations go to
a neutral activity hub; remove the current random-integration fallback.
Do not present simultaneous events as proof of causation. Use deep links to
existing management apps for operations; System World remains an observing view.

## Delivery and acceptance

1. Integrate the asset kit and isolated renderer; establish a real rendered
   overview, street view and inspector reference before expanding effects.
2. Introduce the shared read model and bounded history with source-state tests.
3. Connect all districts, stable focus, search, timeline and navigation.
4. Verify installed/build-bound resources, local licenses and lifecycle cleanup.

Acceptance includes:
- Correct configured/connected/unknown/stale states, authentic traffic mapping,
  pagination/aggregation and preserved focus during live changes.
- History retention, partial-day startup, restart, source failure, event storms,
  timezone changes, counter resets and multiple open windows.
- Chrome at 1920×1080, 1366×768 and narrow touch width; all three themes,
  both densities and DPR 1/2. Inspect overview, street mode, selection, timeline,
  busy/idle/error states, focus/resize/Spaces and context-loss fallback.
- Asset size/hash/LOD checks and rendering of the actual exported files.
- A large fixture with at least 1,000 entities and 10,000 graph nodes using
  aggregation. Measure frame times and resource use on the actual GPU; target
  responsive 60 FPS on the desktop test machine with automatic quality fallback.
- No active rendering in hidden windows and no leaked resources after repeated
  open/close cycles.
- Focused System World, Desktop, localization and UI regression checks; update
  obsolete orbital static tests to the new behaviour.
- Rebuild UI resources and a matching binary, then validate with
  `--check-assets`. Preserve unrelated worktree changes.

Code checks alone do not constitute visual acceptance of the new app.
