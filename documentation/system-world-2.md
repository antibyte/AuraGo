# System World 2

System World is a local Three.js city with seven fixed districts. The second
version adds three accessible annexes, a seven-stop tram, an optional drone tour,
discoveries, articulated residents, time/weather controls and a shared recorded
system view. Exploration and decorative life never call an LLM.

## World and interaction

The agent control room, memory archive and mission depot load within 32 metres.
Sliding doors connect their real ground floors to the street. Each lift connects
to an upper balcony. Collision uses ground solids and door apertures rather than
the enclosing bounds of an entire facade. The quay includes elevated walkways,
stairs and ramps. View selection, district selection, map and HTML information
remain available independently of 3D navigation.

WASD/arrow keys move in focused street view; E or the visible interaction button
opens a door, uses a lift, visits a discovery, boards a tram or starts a drone tour.
The exploration panel also offers explicit district/interior destinations. Tram
exit while moving requests the next stop; mode changes end a ride safely. The
drone tour can be stopped at any time. Seven optional discoveries persist locally.

High quality permits five original white robots, nineteen new robots, six drones
and two trams. Low permits eight robots total, two drones and one tram. Residents
change articulated poses for walking, carrying, greeting, work and rest; authored
turn clips are included in the model catalog. Decorative life is independent of
telemetry. Cargo movements require fresh, observed mission state changes; initial
snapshots, replay and hidden/reduced-motion windows do not synthesize activity.

Lighting follows browser-local time unless day, evening or night is selected.
Clear weather is the default; rain and fog are user choices and never signal a
system fault. Rain adjusts particles, fog, ground roughness and ambience.

Sound is initially off and requires a gesture. Ambience, effects and tower speech
have separate gains. Local Web Audio synthesis supplies spatial machine/water/rain
sources and bounded footsteps, door, lift and tram effects; interiors attenuate and
filter the mix. Focus loss, map view, minimize, Spaces, hiding and disposal suspend
or release audio. The existing tower voice and memory excerpt safeguards remain
authoritative; excerpts are drawn only on canvases, never added to diagnostics or
recorded history.

## Shared system view

The existing server metrics worker records once per ten seconds regardless of
the number of windows. Existing SSE events feed a typed reducer; no second OS
measurement loop is created. Slower inventory/budget/memory/graph facts refresh
once per minute. The browser shares a subscription set across open windows.

Metrics distinguish unavailable from measured zero. Network rates derive from
monotonic counter differences; counter reset, missing sample and long gaps are
unknown. Temperatures appear only when a valid sensor reports one. Configured
integration flags do not imply a connected service. Provider/model information is
the effective configured route; the app does not infer an unreported failover or
cache hit. Missing cache values remain unknown. Panels identify source and age.

Read endpoints under `/api/desktop/system-world/` require Desktop admin permission,
an enabled Virtual Desktop and `Cache-Control: no-store`:

| Endpoint | Result |
| --- | --- |
| `snapshot` | Current typed entities, metrics and allowed actions |
| `snapshot?at=<epoch-ms>` | Recorded state or an explicit missing-sample response |
| `entity?id=<id>` | One entity from the same authorized model |
| `history` | Last 24 hours of minute aggregates, including sample counts |
| `events?since=<epoch-ms>&after=<id>` | At most 200 typed state changes per page |
| `actions` (POST) | Validated target/action request using existing service handlers |

The model retains at most 1,000 entities and 1,000 pending deltas. Container/mission
inventories keep bounded details with complete total counts. The existing search
can inspect larger live inventories; graph neighborhoods remain limited to 300
loaded nodes while showing the real total separately. District positions never
depend on inventory size.

The Desktop SQLite database stores minute metric aggregates, five-minute recovery
checkpoints and at most 50,000 state changes for 24 hours. Missing samples, restart
gaps and dropped event prefixes invalidate dependent replay instead of presenting
an invented state. Pause/replay disables system actions immediately, including
while a historical request is pending. History stores only explicit fields:
public identifiers, scrubbed short labels, states, timestamps and numeric values.
It does not store conversation content, reasoning, tool arguments/results, secrets
or memory excerpts. The current live state is not substituted for missing history.

## Terminal actions

Only existing mission start/cancel, container start/stop/restart and daemon
start/stop operations are accepted. Each request contains `entity`, `action`, a
unique `request_id`, and `confirmed`. Stop, cancel and restart require confirmation
of the concrete target. Existing Docker, mission, daemon, authentication and
read-only gates remain authoritative; the endpoint does not add execution rights.

The client locks before opening confirmation. The server serializes execution and
retains at most 1,024 request receipts for five minutes; same-ID retries return the
same receipt and mismatched reuse is rejected. Docker acknowledgement is completed;
asynchronous operations are only accepted until a fresh matching state arrives.
After 60 seconds without confirmation, the UI asks the user to check the existing
management app rather than claiming success. Rejected actions remain failures.

## Assets and lifecycle

`assets/system-world/build_expansion.py` and its saved Blender scene produce 26
designs and 78 GLBs under `ui/3d/system-world/v2/`. All LODs preserve stable pivot
names and real articulated clips. Manifests include navigation, sizes and hashes.
Original new content is MIT; the white robot retains its existing provenance.

All System World runtime files together must remain below 48 MiB; first display
must remain below 12 MiB. Interiors and alternate LODs load on demand. Static
repetition is instanced, model clones share geometry/materials, mixers are owned
per actor, and all animation uses the existing render loop. Disposal cancels loads
and releases mixers, textures, GPU resources, listeners, timers and audio. A lost
WebGL context falls back to the HTML map. Reduced motion freezes decorative travel
and disables camera tours while retaining the information and terminal interfaces.

## Verification

Run from the repository root with the pinned Go toolchain and existing Node
dependencies. Browser fixtures use real Chrome rendering/input and deterministic
service responses; the API tests separately exercise the actual service handlers.

```powershell
python assets/system-world/check_assets.py
node scripts/test-system-world.mjs
node scripts/test-system-world-expansion.mjs
node scripts/test-system-world-voice.mjs
node scripts/build-system-world.js --check
node scripts/build-ui-bundles.js --check
go test ./internal/systemworld ./internal/server ./internal/tools -run 'Test(History|SystemWorld|DashboardOverview|GetSystemMetrics|DaemonSupervisor|HandleMissionRun|HandleMissionCancel)' -count=1
go test ./ui -run 'TestDesktopSys[Ww]orld|TestDesktop.*Lifecycle|TestDesktopFruityTheme' -count=1
```

For browser acceptance set `AURAGO_RUN_BROWSER_SMOKE=1` and
`AURAGO_SYSTEM_WORLD_MATRIX=1`, then run:

```powershell
go test ./ui -run '^TestDesktopAuroraBrowser$' -count=1 -timeout=8m
```

Run separately with exactly one additional flag for each extended scenario:

| Flag | Coverage |
| --- | --- |
| none | 1080p, 1366x768, touch, Standard/Fruity themes, audio, reduced motion, Spaces, context loss |
| `AURAGO_SYSTEM_WORLD_EXPANSION=1` | Actual walking through three interiors, doors/lift, all seven tram stops, drone and weather |
| `AURAGO_SYSTEM_WORLD_STRESS=1` | 1,000 containers, 10,000-node source with bounded loading, confirmed/deduplicated action, replay, repeated lifecycle, frame timings |
| `AURAGO_SYSTEM_WORLD_MODELS=1` | Every exported LOD and clip rendered from the GLBs in Chrome |

Build the review fixture with `node scripts/build-system-world-review.mjs` before
the model check. Screenshots, hardware/frame reports and the visual acceptance
record belong in ignored `reports/`, not in shipped resources. The 60 FPS high /
30 FPS low targets are measured goals, not guarantees for all integrated GPUs.
Do not infer production integration success from fixture data alone.

Finish by packaging resources and building with the exact generated flags as in
[web-assets.md](web-assets.md), then run the matching binary with `--check-assets`.
No deployment or push is implied by a local build or commit.
