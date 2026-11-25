# HomeDash M3 Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Home Assistant a live, controllable HomeDash source (package B9b), so that real devices switch and dim from a dashboard. Deliver everything the M3 frontend needs from the server: the `ha` adapter with catalog, areas, state stream, reconnect, REST fallback, typed service calls with `expected`, history and camera snapshot; the `home_assistant` template; `source_unavailable` with `source`; first values at once; and the `revoked shutdown` frame. The plan also freezes the M3 API delta (D2).

**Architecture:** The adapter lives in `internal/homedash/sources/hass` next to the B9a websocket client. It keeps one in-memory map of Home Assistant states and the last registries. It feeds the hub sink from websocket callbacks, from REST polls and from the changed states in service-call answers. Commands go through a new typed function `tools.HACallServiceStates`. That function enforces the existing policy (read-only mode, allowed and blocked services, target refusal) and returns the changed states plus a classified error that never carries Home Assistant's text. The server registers one more row in `homedash_sources.go` and builds the `home_assistant` template from a new non-gated hub read `Hub.SourceCatalog`. Bootstrap never waits for Home Assistant: it reads the cached catalog and refreshes it in the background. On shutdown the hub closes before the HTTP drain, so open streams get `revoked shutdown`. The metrics collector takes its first sample at start. A reusable fake Home Assistant (`hasstest`) serves the websocket and REST APIs, both for these tests and for the frontend's later live-handler browser test.

**Tech Stack:** Go 1.27 (module `aurago`), standard library, `github.com/gorilla/websocket` (already a dependency), `internal/homedash`, `internal/homedash/sourcetest`, `internal/testutil`. No new module and no new external asset.

The code in this plan was written without compiling. Implementers may fix compile errors (imports, typos, small signature slips). They must not change the API contract below or any exported name without reporting it to the lead.

---

## Lead answers (2026-11-25)

The answers to "Questions for the lead" at the end of this plan. They are binding; where an answer differs from a task body, the answer wins.

- **Q1 accepted.** Add the optional `Widget.Icon` (Task 2). design-frontend section 4 already plans an icon override in the inspector (M4), so the field closes a schema gap. Absent `icon` means the bound entity's `icon_hint`.
- **Q2 accepted.** Bootstrap reads the cached catalog and refreshes in the background; the client re-reads bootstrap before it shows template choices.
- **Q3 decided: tolerances plus late adoption.** The contract's tolerances stay. In addition, when the 8 s window ends, a frame received since sending with `available:true` and a matching `value` ends the request silently with the reported attributes (contract section 4). The backend is not affected.
- **Q4 accepted.** Template qualification follows the Home Assistant service policy only.
- **Q5 accepted.** `omitted` is always present on the `home_assistant` answer; new code `422 template_empty`.
- **Q6 accepted.** No camera scaling in M3.
- **Q7 accepted.** Every command capability of the domain table is mapped, `expected` only for W1 verbs. Task 7 tests every mapping row, and the security review (Task 14) checks the lock and cover mappings and that sensitive verbs keep needing `confirmed:true`.
- **Q8 accepted.** `internal/homedash/sources/hass/hasstest` is the importable fake; it is part of the frozen surface the frontend's live-handler browser test uses (`hasstest` server constructor, `DemoOptions()`, the demo entity ids of contract section 2).
- **Q9 accepted.** Five-minute polling window, then one websocket attempt; a failing REST poll returns to the websocket backoff.

**Cross-check amendment X1 (502 vs 503 on a slow Home Assistant).** The contract stays: a call Home Assistant does not answer within 15 s is `502 command_failed`; no connection or a rejected token is `503 source_unavailable`. The code already does this, so there is no new error kind. The hub runs the source call under its own `CommandTimeout` context (`internal/homedash/command.go:546-558`). `haHTTPClient` has a 30 s timeout (`internal/tools/homeassistant.go:19`), so the hub's deadline always fires first. `commandError` then returns `ctx.Err()`, and the hub maps that to `command_failed`. Two changes:
- Task 3: the `HAServiceUnreachable` comment reads `// no answer: transport error or cancellation (a slow answer ends at the hub's command timeout first: 502)`.
- Task 7: add one test. `Adapter.Command`, under a context that expires after 200 ms, calls a service handler that blocks until its request ends. The returned error must satisfy `errors.Is(err, context.DeadlineExceeded)` and must not be a `*homedash.Error`. A comment in the test notes that the hub turns this into 502 `command_failed`.

**Cross-check amendment X2.** Contract §3 also lists `unavailable` / `invalid_url`, which Task 6 emits when the configured URL is not a usable http(s) URL. The frontend shows it with the generic "unavailable" banner. The one-line edit to `internal/server/server.go` (Task 12) is a backend file, in addition to the D15 list.

---

## Lead decisions applied

| Decision | Applied in |
|---|---|
| D1 B9b scope (`adapter.go`, `catalog.go`, `commands.go`, `history.go`, `camera.go`, `rest_fallback.go`; areas, hidden/disabled filter, `state_changed`, 2 s registry debounce, 1–60 s ±20 % backoff, REST fallback after 3 failed handshakes, 5 min websocket retry; server row + Settings; audit network entry; runs only with `homedash.enabled`, `homedash.sources.home_assistant.enabled` and the HA integration on) | Tasks 4–8 (adapter; the B9a client already provides debounce and backoff), Task 9 (row, Settings, audit). `homedash.enabled` is enforced by `RuntimeConfig.Enabled`. The Settings `Enabled` field combines the source switch, `home_assistant.enabled`, a URL and a token. |
| D2 Contract first | Section "API contract (frozen for M3)" |
| D3 Typed service call, no raw HA text, changed states fed at once, `expected`, verb mapping | Task 3 (`tools.HACallServiceStates`, `HAServiceError`), Task 7 (verbs, `expected`, feed, error mapping), Task 1 (`Error.Source`) |
| D4 `home_assistant` template + bootstrap | Task 2 (`Hub.SourceCatalog`), Task 10 |
| D5 Interaction | Frontend. The backend keeps the limits unchanged; the contract restates them and defines the confirmation match. |
| D6 Light attributes | Contract (the adapter already sends `brightness_pct`, `color_temp_kelvin`, `hs_color`, `color_mode`, `supported_color_modes`, `effect`, `effect_list`; ranges and options sit in the capabilities) |
| D7 Carry-overs | (a) Task 12. (b) Not touched; Task 0 checks the separate branch. (c) Contract section "Partial catalogs". `source` field: Task 1. First values: Task 11. |
| D8 Starter header icons (backend part) | Task 2 (`Widget.Icon`), Task 10 (system template icons). The token change is frontend. |
| D9 Icon list | Task 2 (`IconNames`), Task 5 (adapter mapping), contract section "Icon names" |
| D10 Service policy unchanged | No policy edit. Capabilities report `blocked_by`; Task 7 maps a late policy refusal to `read_only` + `blocked_by`. |
| D11 Frontend gates | Not backend |
| D12 Security review | Task 14 |
| D13 Tests | Every task. Fake HA with REST: Task 4. Server end-to-end: Task 13. Shutdown frame: Task 12. |
| D14 Live HA | Task 8 adds an opt-in live test that reads only environment variables. Automated tests use the fake. |
| D15 Order and ownership | File structure and rule R6. Nothing under `ui/`. |

## File structure

Created:
- `internal/homedash/icons.go` — the fixed icon-name list (`IconNames`, `KnownIcon`).
- `internal/homedash/icons_test.go`
- `internal/homedash/hub_catalog.go` — `Hub.SourceCatalog` (raw catalog read, optional fetch, no start, no hub gates).
- `internal/homedash/hub_catalog_test.go`
- `internal/homedash/command_source_test.go` — `source_unavailable` keeps `source` and audits as `failed`.
- `internal/homedash/dashboard_icon_test.go` — optional, validated widget `icon`.
- `internal/tools/homeassistant_service_states.go` — `HACallServiceStates`, `HAChangedState`, `HAServiceError`.
- `internal/tools/homeassistant_service_states_test.go`
- `internal/homedash/sources/hass/hasstest/server.go` — importable fake Home Assistant (websocket + REST).
- `internal/homedash/sources/hass/hasstest/demo.go` — `DemoOptions()`: a small home covering every W1 domain.
- `internal/homedash/sources/hass/hasstest/server_test.go`
- `internal/homedash/sources/hass/ws_fetch.go` — one-shot `WSClient.Fetch`, `nopHandler`.
- `internal/homedash/sources/hass/catalog.go` — `SourceID`, settings keys, area index, `entityFor`, `buildCatalog`, `mapState`, `iconHint`.
- `internal/homedash/sources/hass/catalog_test.go`
- `internal/homedash/sources/hass/adapter.go` — `Adapter`, `Factory`, `Catalog`, `Run`, websocket bridge, state ingest, stale marking.
- `internal/homedash/sources/hass/rest_fallback.go` — polling phase, REST poll.
- `internal/homedash/sources/hass/adapter_test.go`
- `internal/homedash/sources/hass/rest_fallback_test.go`
- `internal/homedash/sources/hass/commands.go` — verb mapping, `expected`, `Command`, error mapping.
- `internal/homedash/sources/hass/commands_test.go`
- `internal/homedash/sources/hass/history.go` — `History` through `/api/history/period`.
- `internal/homedash/sources/hass/camera.go` — `Snapshot` through `/api/camera_proxy`.
- `internal/homedash/sources/hass/history_camera_test.go`
- `internal/homedash/sources/hass/live_test.go` — opt-in live test (D14).
- `internal/server/homedash_template_ha.go` — `home_assistant` template builder, bootstrap template list.
- `internal/server/homedash_template_ha_test.go`
- `internal/server/homedash_first_values_test.go` — collector first sample, system rates in the first snapshot.
- `internal/server/homedash_shutdown_test.go` — `revoked shutdown` before the drain.
- `internal/server/homedash_ha_e2e_test.go` — server end to end with the fake HA.
- `docs/superpowers/specs/2026-11-23-homedash/m3-handover-backend.md` (force-add)

Modified:
- `internal/homedash/errors.go` — `Error.Source`, `SourceUnavailable(...)`.
- `internal/homedash/command.go` — audit outcome `failed` for `source_unavailable`.
- `internal/homedash/dashboard.go` — `Widget.Icon`.
- `internal/homedash/dashboard_validate.go` — validate widget `icon`.
- `internal/homedash/sources/system/system.go` — `Snapshot.NetRxMbps/NetTxMbps`; `compute` prefers them.
- `internal/homedash/sources/system/system_test.go` — one new test.
- `internal/server/homedash_routes.go` — `fail` adds `source`; code `template_empty` (422); bootstrap `templates` from the service.
- `internal/server/homedash_sources.go` — HA row (`homeDashHASpec`, `homeDashHASettings`); system snapshot carries the server's rates.
- `internal/server/homedash_sources_test.go`
- `internal/server/homedash_template.go` — starter widgets get `icon`.
- `internal/server/homedash_api_dashboards.go` — `unassigned_title`, `home_assistant` branch, `omitted`.
- `internal/server/homedash_api_dashboards_test.go` — starter contract JSON with icons.
- `internal/server/homedash_api_commands_test.go` — `source_unavailable` body test.
- `internal/server/homedash_service.go` — `ctx`, `haProbe`, `streams`; `endHomeDashStreams`.
- `internal/server/homedash_stream.go` — open-stream counter.
- `internal/server/homedash_metrics_ring.go` — `latestRates`.
- `internal/server/server_routes.go` — `sampleSystemMetrics`; first sample at start.
- `internal/server/server.go` — `serveWithShutdown` calls `endHomeDashStreams` before `beginHTTPDrain` (one line).
- `internal/audit/audit_manifests.go` — network client entry `internal/homedash/sources/hass/`.
- `internal/audit/homedash_manifest_test.go` — one new test.
- `internal/homedash/AGENTS.md`, `internal/server/AGENTS.md`, `internal/tools/AGENTS.md` (docs task).

Not touched: everything under `ui/`; the HA Switchboard (`internal/server/desktop_homeassistant_handlers.go` and its test, `internal/desktop/ha_switchboard.go` and its test, `ui/js/desktop/apps/ha-switchboard.js`, `ui/css/ha-switchboard.css`, `ui/desktop_ha_switchboard_browser_test.go`); `internal/tools/homeassistant.go` (`HACallServiceContext` stays as is); `statusRecorder`, the gzip wrapper and `internal/httpstream` (D7 gap b); config keys (they exist); locale files; the widget catalog and its golden file (hash unchanged).

---

## API contract (frozen for M3)

This section is a **delta** to "API contract (frozen for M2)" in `docs/superpowers/plans/2026-11-25-homedash-m2-backend.md`. Everything that is not changed here stays valid: base path, auth, classes, envelope, headers, times (RFC 3339 UTC with `Z`), body limits, routes, the stream protocol, the reconnect rules and the limits. The frontend plan consumes this section verbatim.

### 1. What changes

1. A new source `ha` (Home Assistant). Its entities are `ha:<domain>.<object_id>`.
2. `homedash.Error` gains `source`. Any error body may carry `"source":"<source id>"`. In M3 it comes with `503 source_unavailable`.
3. `POST commands` on `ha` entities answers `202` with `expected` (table in §4), or `502 command_failed`, `503 source_unavailable` (+`source`) and the M2 codes.
4. `POST dashboards` also accepts `{"template":"home_assistant","name"?,"unassigned_title"?}`. The answer adds `omitted`. A new code is `422 template_empty`. `bootstrap.templates` lists `home_assistant` only when it can be built (§5).
5. Dashboard widgets gain an optional `icon` (a name from §6). The `system` starter template sets it.
6. `system` values are present right after server start. Network rates are present in the first snapshot once the server has two samples (§9).
7. On shutdown every open stream now actually receives `revoked {"reason":"shutdown"}` before the connection ends (§10).
8. A fixed list of icon names (§6).

### 2. `ha` entities

**Ids.** `ha:<native>`. The native id matches `^[a-z_][a-z0-9_]*\.[a-z0-9_]+$` (at most 200 bytes). The part before the dot is the Home Assistant domain.

**Catalog membership.** It contains every Home Assistant state except:
- the domain `alarm_control_panel`;
- entities that the entity registry marks hidden (`hidden_by`) or disabled (`disabled_by`);
- invalid ids.

It holds at most 2000 entities, cut in entity-id order.

**Kinds (W1 domains in bold; others for reference):**

| Domain | `kind` | Command capabilities (service) | Read capabilities |
|---|---|---|---|
| **light** | `light` | `toggle`, `turn_on`, `turn_off`; `brightness` (when `supported_color_modes` has any of brightness, color_temp, hs, xy, rgb, rgbw, rgbww, white); `color_temp` (mode color_temp); `color` (hs, xy, rgb, rgbw, rgbww); `effect` (non-empty `effect_list`) | – |
| **switch**, **input_boolean** | `switch` | `toggle`, `turn_on`, `turn_off` | `history` |
| **scene** | `scene` | `activate` | – |
| **script** | `script` | `activate` (always `sensitive`; blocked by the default service policy) | – |
| **button**, **input_button** | `button` | `press` | – |
| **select**, **input_select** | `select` | `select_option` (with `options`) | – |
| **number**, **input_number** | `number` | `set_value` (with `min`, `max`, `step`; defaults 0, 100, 1) | `history` |
| sensor | `sensor` | – | `numeric`, `history` when the state is a finite number |
| binary_sensor | `binary_sensor` | – | `history` |
| climate, cover, fan, lock, media_player, humidifier | as design-backend 8.1 | as design-backend 8.1 (mapped, no `expected`; widgets in M5) | climate: `history` |
| camera | `camera` | – | `snapshot`; entity carries `"media":{"snapshot":true}` |
| everything else | per design-backend 8.1 (`generic` for unknown domains) | – | – |

**Fields.**
- `name`: Home Assistant's `friendly_name`, cleaned and at most 120 runes. It falls back to the native id.
- `area`, `area_id`: the entity registry area, else the area of the entity's device. The display name comes from the area registry. Both fields are absent when there is no area. While no registry is known yet (first seconds of a new connection, or REST polling without an earlier registry read), they are absent too, and a later `entity` frame adds them.
- `device_class`, `unit` (`unit_of_measurement`): as Home Assistant reports them.
- `icon_hint`: see §6. Home Assistant's own `icon` (`mdi:…`) is never forwarded.
- `capabilities`: in the table order above. Command capabilities carry `allowed`. When not allowed, they carry `blocked_by`, which is one of `home_assistant.readonly`, `home_assistant.blocked_services`, `home_assistant.allowed_services`, or a HomeDash gate (`homedash.readonly`, `virtual_desktop.readonly`, `homedash.sources.home_assistant.control`). They also carry `sensitive` and the ranges and options.
- `readonly` / `readonly_reason`: as in M2 (the first `blocked_by` when no command is allowed).

**State.**
- On/off domains (switch, input_boolean, light, fan, humidifier, binary_sensor, script, automation): `value` `true`/`false`. Any other text is passed through as a string.
- Numeric domains (sensor, number, input_number): `value` is a number when the state is a finite number, else the string.
- All other domains (scene, button, select, cover, …): `value` is the state string. Scenes and buttons report the time of their last activation or press as text.
- `"unavailable"`: `available:false`, `value:null`.
- `"unknown"`: `available:true`, `value:null`.
- `text` is always Home Assistant's raw state.
- `updated_at` is Home Assistant's `last_updated` in UTC.
- `stale:true` appears for subscribed entities while the source is `degraded` or `unavailable`.

**Attributes** pass through an allowlist:
```
unit_of_measurement device_class state_class color_mode supported_color_modes color_temp_kelvin
min_color_temp_kelvin max_color_temp_kelvin hs_color effect effect_list current_temperature temperature
target_temp_low target_temp_high target_temp_step min_temp max_temp hvac_modes hvac_action preset_mode
preset_modes fan_mode fan_modes current_position current_tilt_position percentage percentage_step
oscillating direction media_title media_artist source source_list volume_level is_volume_muted options
min max step mode battery_level installed_version latest_version next_rising next_setting elevation
humidity pressure wind_speed wind_bearing
```
- `brightness` (0–255) becomes `brightness_pct` (0–100, rounded). It is the truth for brightness (D6).
- Values may be `null`; Home Assistant reports `null` for most light attributes of a light that is off.
- Attribute JSON is at most 8 KiB, strings at most 256 runes and lists at most 64 items.
- These are never sent: `icon`, `entity_picture`, `access_token`, coordinates, and any key containing `url`, `token`, `password`, `secret` or `gps`.

**Examples** (from the fake's demo home, `hasstest.DemoOptions()`, default service policy):

Light, dimmable, colour temperature, colour, effects:
```json
{"id":"ha:light.kitchen","source":"ha","kind":"light","name":"Kitchen","area":"Kitchen","area_id":"kitchen","icon_hint":"bulb",
 "capabilities":[{"name":"toggle","allowed":true},{"name":"turn_on","allowed":true},{"name":"turn_off","allowed":true},
  {"name":"brightness","allowed":true,"min":0,"max":100,"step":1},
  {"name":"color_temp","allowed":true,"min":2202,"max":6535,"step":50},
  {"name":"color","allowed":true},
  {"name":"effect","allowed":true,"options":["none","colorloop"]}],
 "readonly":false,
 "state":{"value":true,"text":"on","attributes":{"brightness_pct":40,"color_mode":"color_temp","color_temp_kelvin":3003,"effect":"none","effect_list":["none","colorloop"],"hs_color":[27.2,56.1],"max_color_temp_kelvin":6535,"min_color_temp_kelvin":2202,"supported_color_modes":["color_temp","hs"]},"available":true,"updated_at":"2026-11-25T09:00:00Z"}}
```

Light, on/off only, off:
```json
{"id":"ha:light.sofa","source":"ha","kind":"light","name":"Sofa lamp","area":"Living room","area_id":"living_room","icon_hint":"bulb",
 "capabilities":[{"name":"toggle","allowed":true},{"name":"turn_on","allowed":true},{"name":"turn_off","allowed":true}],
 "readonly":false,
 "state":{"value":false,"text":"off","attributes":{"color_mode":null,"supported_color_modes":["onoff"]},"available":true,"updated_at":"2026-11-25T09:00:00Z"}}
```

Switch (outlet):
```json
{"id":"ha:switch.coffee","source":"ha","kind":"switch","name":"Coffee machine","area":"Kitchen","area_id":"kitchen","device_class":"outlet","icon_hint":"outlet",
 "capabilities":[{"name":"toggle","allowed":true},{"name":"turn_on","allowed":true},{"name":"turn_off","allowed":true},{"name":"history","allowed":true}],
 "readonly":false,
 "state":{"value":false,"text":"off","attributes":{"device_class":"outlet"},"available":true,"updated_at":"2026-11-25T09:00:00Z"}}
```

input_boolean:
```json
{"id":"ha:input_boolean.vacation_mode","source":"ha","kind":"switch","name":"Vacation mode","icon_hint":"power",
 "capabilities":[{"name":"toggle","allowed":true},{"name":"turn_on","allowed":true},{"name":"turn_off","allowed":true},{"name":"history","allowed":true}],
 "readonly":false,
 "state":{"value":false,"text":"off","available":true,"updated_at":"2026-11-25T09:00:00Z"}}
```

Scene:
```json
{"id":"ha:scene.movie","source":"ha","kind":"scene","name":"Movie night","area":"Living room","area_id":"living_room","icon_hint":"scene",
 "capabilities":[{"name":"activate","allowed":true}],
 "readonly":false,
 "state":{"value":"2026-11-24T20:15:00.000000+00:00","text":"2026-11-24T20:15:00.000000+00:00","available":true,"updated_at":"2026-11-25T09:00:00Z"}}
```

Script (default policy: blocked; with `home_assistant.allowed_services` containing `script.turn_on` it becomes `"allowed":true,"sensitive":true` and `readonly:false`):
```json
{"id":"ha:script.goodnight","source":"ha","kind":"script","name":"Good night","icon_hint":"script",
 "capabilities":[{"name":"activate","allowed":false,"blocked_by":"home_assistant.allowed_services","sensitive":true}],
 "readonly":true,"readonly_reason":"home_assistant.allowed_services",
 "state":{"value":false,"text":"off","available":true,"updated_at":"2026-11-25T09:00:00Z"}}
```

Button (an `input_button` has the same shape, id `ha:input_button.<x>`):
```json
{"id":"ha:button.doorbell_test","source":"ha","kind":"button","name":"Doorbell test","icon_hint":"button",
 "capabilities":[{"name":"press","allowed":true}],
 "readonly":false,
 "state":{"value":null,"text":"unknown","available":true,"updated_at":"2026-11-25T09:00:00Z"}}
```

Select (≤ 4 options):
```json
{"id":"ha:select.heater_mode","source":"ha","kind":"select","name":"Heater mode","area":"Kitchen","area_id":"kitchen","icon_hint":"list",
 "capabilities":[{"name":"select_option","allowed":true,"options":["eco","comfort","boost"]}],
 "readonly":false,
 "state":{"value":"comfort","text":"comfort","attributes":{"options":["eco","comfort","boost"]},"available":true,"updated_at":"2026-11-25T09:00:00Z"}}
```

input_select (> 4 options):
```json
{"id":"ha:input_select.radio","source":"ha","kind":"select","name":"Radio","icon_hint":"list",
 "capabilities":[{"name":"select_option","allowed":true,"options":["Jazz","Rock","News","Classic","Pop","Lounge"]}],
 "readonly":false,
 "state":{"value":"Jazz","text":"Jazz","attributes":{"options":["Jazz","Rock","News","Classic","Pop","Lounge"]},"available":true,"updated_at":"2026-11-25T09:00:00Z"}}
```

Number (mode box):
```json
{"id":"ha:number.boiler_target","source":"ha","kind":"number","name":"Boiler target","unit":"°C","icon_hint":"number",
 "capabilities":[{"name":"set_value","allowed":true,"min":30,"max":70,"step":1},{"name":"history","allowed":true}],
 "readonly":false,
 "state":{"value":55,"text":"55.0","attributes":{"max":70,"min":30,"mode":"box","step":1,"unit_of_measurement":"°C"},"available":true,"updated_at":"2026-11-25T09:00:00Z"}}
```

input_number (mode slider):
```json
{"id":"ha:input_number.volume","source":"ha","kind":"number","name":"Volume","area":"Living room","area_id":"living_room","icon_hint":"number",
 "capabilities":[{"name":"set_value","allowed":true,"min":0,"max":100,"step":5},{"name":"history","allowed":true}],
 "readonly":false,
 "state":{"value":35,"text":"35.0","attributes":{"max":100,"min":0,"mode":"slider","step":5},"available":true,"updated_at":"2026-11-25T09:00:00Z"}}
```

Unavailable entity (any domain): `"state":{"value":null,"text":"unavailable","available":false,"updated_at":"…"}`.

### 3. Source status of `ha`, partial catalogs (D7 c)

`SourceStatus` values for `ha` (in `bootstrap.sources`, `entities.sources`, `snapshot.sources` and `source` frames):

| state | reason | meaning |
|---|---|---|
| `disabled` | – | the source switch, the HA integration, the URL or the token is missing, or HomeDash is off |
| `idle` | – | enabled, nobody interested; no connection |
| `starting` | – | connecting for the first time in this run |
| `live` | – | websocket connected, states streaming |
| `polling` | `websocket_unavailable` | websocket blocked (3 failed handshakes while REST answers); values every `poll_seconds` (default 5) while subscribed |
| `degraded` | `unreachable`, `timeout`, `protocol`, `crashed`, `stopped` | connection lost, reconnecting with backoff 1–60 s; subscribed values are `stale` |
| `unavailable` | `auth_failed` | token rejected; retried every 5 min or on config change |
| `unavailable` | `catalog_failed` | the catalog could not be read (hub) |
| `unavailable` | `invalid_url` | `home_assistant.url` is not a usable http(s) URL; retried on config change (amendment X2) |

**Partial catalogs are intended.** `GET entities` always answers 200 with what the hub has after at most 9 s. A source whose catalog failed or timed out contributes no entities, and its status says so (`unavailable`/`catalog_failed`). `total` counts only listed entities, and there is never a 504 for a slow source. Example:
```json
{"status":"ok","entities":[…six system entities…],"total":6,"limit":100,"offset":0,
 "sources":[{"source":"system","state":"polling","at":"2026-11-25T10:00:01Z"},{"source":"ha","state":"unavailable","reason":"catalog_failed","at":"2026-11-25T10:00:09Z"}]}
```
The client shows a per-source notice for any source in `degraded` or `unavailable`. `sources` is an array (as in M2), not a map. `GET entity?id=ha:…` of a source whose catalog never loaded answers `404 entity_not_found`; the client checks `sources` before calling the entity removed.

### 4. Commands (delta)

**Request** (unchanged shape): `{"entity":"ha:light.kitchen","command":"brightness","args":{"value":40},"request_id":"5f0c…","dashboard_id":"d_m3x7q2k5a4bc","widget_id":"w1","confirmed":false}`.

**W1 arguments** (checked by the server; values off the grid answer `400 invalid_args`):

| command | args | rule |
|---|---|---|
| `toggle`, `turn_on`, `turn_off`, `activate`, `press` | `{}` | – |
| `brightness` | `{"value":n}` | integer 0–100 (`min`+k·`step`, step 1) |
| `color_temp` | `{"kelvin":n}` | `min`…`max` of the capability, and `kelvin = min + k·step` (step 50, anchored at `min`, e.g. 2202, 2252, …; `max` itself may be off the grid) |
| `color` | `{"h":n,"s":n}` | h 0–360, s 0–100, any real |
| `effect` | `{"option":"…"}` | one of the capability `options` |
| `select_option` | `{"option":"…"}` | one of the capability `options` |
| `set_value` | `{"value":n}` | `min`…`max`, `value = min + k·step` (tolerance 1e-6·step) |

**What the server sends to Home Assistant** (`POST /api/services/<domain>/<service>`, always with `entity_id` = the native id):

| command | service | data |
|---|---|---|
| toggle / turn_on / turn_off | `<domain>.toggle` / `.turn_on` / `.turn_off` | – |
| brightness | `light.turn_on` | `brightness_pct` (0 turns the light off) |
| color_temp | `light.turn_on` | `color_temp_kelvin` |
| color | `light.turn_on` | `hs_color: [h, s]` |
| effect | `light.turn_on` | `effect` |
| activate | `scene.turn_on` / `script.turn_on` | – |
| press | `button.press` / `input_button.press` | – |
| select_option | `select.select_option` / `input_select.select_option` | `option` |
| set_value | `number.set_value` / `input_number.set_value` | `value` |

Non-W1 capabilities of the domain table are mapped too (climate `temperature`, `target_temp_low/high`, `hvac_mode`, `preset_mode`, `fan_mode`; cover `position`, `tilt_position`; fan `percentage`, `oscillating`; media `volume_level` = value/100, `is_volume_muted`, `source`; lock and cover verbs). They have no `expected`, and their widgets arrive in M5.

**Answer `202`** is sent after Home Assistant answered the service call. It does not mean the device has changed.
```json
{"status":"ok","accepted":true,"superseded":false,"expected":{"value":true,"text":"on","attributes":{"brightness_pct":40},"available":true,"updated_at":"2026-11-25T10:00:01.2Z"}}
```

`expected` per command (the backend omits `expected` where the table says "–"):

| command | `expected` |
|---|---|
| toggle | inverse of the current on/off state: `{"value":false,"text":"off"}` or `{"value":true,"text":"on"}`; omitted when the current state is neither on nor off |
| turn_on / turn_off | `{"value":true,"text":"on"}` / `{"value":false,"text":"off"}` |
| brightness n > 0 | `{"value":true,"text":"on","attributes":{"brightness_pct":n}}` |
| brightness 0 | `{"value":false,"text":"off"}` |
| color_temp k | `{"value":true,"text":"on","attributes":{"color_temp_kelvin":k}}` |
| color h,s | `{"value":true,"text":"on","attributes":{"hs_color":[h,s]}}` |
| effect o | `{"value":true,"text":"on","attributes":{"effect":"o"}}` |
| select_option o | `{"value":"o","text":"o"}` |
| set_value n | `{"value":n,"text":"<n without trailing zeros>"}` |
| activate, press | – (momentary: the 202 confirms) |

`expected` always has `available:true` and the server time as `updated_at`. Its `attributes` hold **only** the attributes the command sets, so merge them into the current attributes. `superseded:true` answers carry no `expected`: a newer value for the same control replaced this one before it was sent (continuous: brightness, color_temp, color, set_value).

**Changed states.** Home Assistant's answer lists the states the call changed. The server pushes them to the stream at once, usually as a `state` frame within 100 ms of the 202. The websocket event follows with the same values. A client must accept the same state twice.

**Confirmation rule (frontend, D5 "a state frame whose value matches expected").** A `state` frame for the entity confirms when all of these hold:
- `available` is true;
- `value` equals `expected.value` (booleans and strings exactly; numbers within half the capability `step`, or exactly when there is no step);
- every key of `expected.attributes` matches: `brightness_pct` ±1; `color_temp_kelvin` ±2 % of the expected value; `hs_color` hue ±4° (circular) and saturation ±4; strings exactly.

Devices round mireds and limit their gamut, so colour values can come back outside these tolerances. **Late adoption (lead answer Q3):** when the 8 s window ends without a confirming frame, but a `state` frame for the entity has arrived since the command was sent (the 202 and the stream race, so count from sending, not from the 202) with `available:true` and a matching `value`, the client drops its overlay silently and shows the reported attributes. Only when no such frame arrived, or the value differs, does it roll back and show the error cue.

**Error bodies** (status, body). Clients key on status plus `code`, falling back to `error`:

| HTTP | body |
|---|---|
| 400 | `{"status":"error","code":"bad_request","message":"request_id must be 1 to 64 characters"}` (missing fields, bad ids, malformed entity id) |
| 400 | `{"status":"error","code":"invalid_args","message":"kelvin is not a multiple of 50"}` |
| 400 | `{"status":"error","code":"unsupported_command","message":"the entity does not offer this command"}` |
| 401 | `{"error":"unauthorized","redirect":"/auth/login"}` |
| 403 | `{"error":"csrf_check_failed","message":"…"}`, `{"error":"desktop_scope_required","message":"…"}`, `{"error":"invalid_bearer_scope"}` |
| 403 | `{"error":"desktop_readonly","code":"desktop_readonly","message":"…"}` |
| 403 | `{"status":"error","code":"read_only","message":"commands are switched off","blocked_by":"home_assistant.allowed_services"}`. `blocked_by` ∈ `homedash.readonly`, `virtual_desktop.readonly`, `homedash.sources.home_assistant.control`, `home_assistant.readonly`, `home_assistant.blocked_services`, `home_assistant.allowed_services` |
| 404 | `{"status":"error","code":"entity_not_found","message":"unknown entity"}` |
| 409 | `{"status":"error","code":"confirmation_required","message":"this command needs a confirmation"}` (`sensitive` without `confirmed:true`) |
| 413 | `{"status":"error","code":"body_too_large","message":"the request body is too large"}` |
| 429 | `{"status":"error","code":"rate_limited","message":"too many commands","retry_after_ms":3200}` + header `Retry-After: 4` |
| 500 | `{"status":"error","code":"internal_error","message":"HomeDash could not complete the request"}` |
| 502 | `{"status":"error","code":"command_failed","message":"Home Assistant did not accept the command"}` (HA answered 4xx or 500, or did not answer within 15 s) |
| 503 | `{"status":"error","code":"source_unavailable","message":"Home Assistant is not reachable","source":"ha"}` (no connection, token rejected, HA answered 502/503/504) |
| 503 | `{"status":"error","code":"hub_closed","message":"HomeDash is shutting down"}`; `homedash_disabled`; `storage_unavailable` |
| 504 | `{"status":"error","code":"timeout","message":"the request did not finish in time"}` |

No error body ever contains Home Assistant's response text.

**Limits** (unchanged, D5): 60 commands per minute and 10 per 10 s per principal; 5 per 5 s per entity; at most 4 in flight per source; one latest-wins slot per entity and continuous capability; `request_id` dedupe for 30 s.

### 5. Templates and bootstrap (D4)

**Request:** `POST dashboards` with `{"template":"home_assistant","name":"Zuhause","unassigned_title":"Ohne Raum"}`.
- `name` is optional (default `"Home Assistant"`, at most 80 runes).
- `unassigned_title` is optional (default `"Unassigned"`, at most 40 runes). The client sends both localized.
- Neither may contain control characters or URLs; otherwise `422 invalid_document`.
- `unassigned_title` with any other form answers `400 bad_request`.

**Content:**
- One page per HA area that has qualifying entities, with the area name as title, sorted by name (case-insensitive). Then a last page titled `unassigned_title` for qualifying entities without an area.
- An entity qualifies when its domain is a W1 domain and at least one of its command capabilities is allowed by the Home Assistant service policy. `home_assistant.readonly` and the allowed/blocked lists count. HomeDash's own switches (`homedash.readonly`, desktop read-only, `homedash.sources.home_assistant.control`) do not.
- Widgets per domain: dimmable light (an allowed `brightness`) → `light/slider`; other light → `light/tile`; switch and input_boolean → `switch/tile`; scene, script, button and input_button → `action/button`; select and input_select → `select/segmented` with ≤ 4 options, else `select/list`; number and input_number → `number/stepper` when the attribute `mode` is `box`, else `number/slider`.
- Order inside a page follows the domain order light, switch, input_boolean, scene, script, button, input_button, select, input_select, number, input_number, then name, then id.
- Caps: 12 pages, 24 widgets per page, `limits.widgets` (400) in total. `omitted` counts the qualifying entities the caps left out; it is always present on this template's answer (0 when nothing was left out).
- Widget ids are `w1`, `w2`, … across the dashboard, with no title and empty `options`.
- Page ids come from the area id (`[a-z0-9_-]`, at most 28 characters, made unique); the last page is `unassigned`.
- Look: family `glass`, palette `theme`, backdrop `theme`, density `comfortable`, radius 1, row unit `auto`, fit `scroll`.
- Sizes come from the catalog defaults through Autopack. The same save pipeline as M2 applies; the slug is `home-assistant`, then `home-assistant-2`, …

**Answer** `201` (demo home):
```json
{"status":"ok","omitted":0,"warnings":[],
 "dashboard":{"schema_version":1,"id":"d_m3x7q2k5a4bc","slug":"home-assistant","name":"Zuhause","revision":1,"approved_revision":1,
  "updated_at":"2026-11-25T10:00:00.123456789Z","saved_by":"user",
  "look":{"family":"glass","palette":{"mode":"theme"},"backdrop":{"kind":"theme"},"density":"comfortable","radius_scale":1},
  "grid":{"row_unit":"auto","gap":null,"fit":"scroll"},
  "pages":[
   {"id":"kitchen","name":"Kitchen","compact":true,"widgets":[
     {"id":"w1","type":"light","variant":"slider","bindings":{"primary":"ha:light.kitchen"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
     {"id":"w2","type":"switch","variant":"tile","bindings":{"primary":"ha:switch.coffee"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
     {"id":"w3","type":"select","variant":"segmented","bindings":{"primary":"ha:select.heater_mode"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false}],
    "layouts":{"lg":[{"id":"w1","x":0,"y":0,"w":4,"h":1},{"id":"w2","x":4,"y":0,"w":2,"h":2},{"id":"w3","x":6,"y":0,"w":3,"h":1}]}},
   {"id":"living_room","name":"Living room","compact":true,"widgets":[
     {"id":"w4","type":"light","variant":"tile","bindings":{"primary":"ha:light.sofa"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
     {"id":"w5","type":"action","variant":"button","bindings":{"primary":"ha:scene.movie"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
     {"id":"w6","type":"number","variant":"slider","bindings":{"primary":"ha:input_number.volume"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false}],
    "layouts":{"lg":[{"id":"w4","x":0,"y":0,"w":2,"h":2},{"id":"w5","x":2,"y":0,"w":2,"h":1},{"id":"w6","x":4,"y":0,"w":2,"h":1}]}},
   {"id":"unassigned","name":"Ohne Raum","compact":true,"widgets":[
     {"id":"w7","type":"switch","variant":"tile","bindings":{"primary":"ha:input_boolean.vacation_mode"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
     {"id":"w8","type":"action","variant":"button","bindings":{"primary":"ha:button.doorbell_test"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
     {"id":"w9","type":"select","variant":"list","bindings":{"primary":"ha:input_select.radio"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
     {"id":"w10","type":"number","variant":"stepper","bindings":{"primary":"ha:number.boiler_target"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false}],
    "layouts":{"lg":[{"id":"w7","x":0,"y":0,"w":2,"h":2},{"id":"w8","x":2,"y":0,"w":2,"h":1},{"id":"w9","x":4,"y":0,"w":2,"h":1},{"id":"w10","x":6,"y":0,"w":2,"h":1}]}}],
  "kiosk":{"start_page":"kitchen","idle_dim_seconds":120,"idle_dim_level":0.3,"page_rotation_seconds":0,"screen_saver":"clock","show_clock":true}}}
```
Some demo entities are not on the dashboard, and they do not count as `omitted`:
- `script.goodnight`: blocked by the policy;
- `input_boolean.guest_mode`: hidden;
- `sensor.outdoor_temperature` and `camera.door`: not W1 domains;
- `alarm_control_panel.home`: excluded.

**Errors:**
- `503 source_unavailable` with `"source":"ha"` when the HA source is off or its catalog cannot be read.
- `422 template_empty` (`{"status":"error","code":"template_empty","message":"Home Assistant has no entity this template can control"}`) when no entity qualifies.
- On a read-only desktop: `403 desktop_readonly`.
- All M2 save errors (`422 invalid_document`, `422 limit_exceeded`, `409 slug_taken`) as in M2.

**Bootstrap:**
```json
{"status":"ok","api_version":1,"stream_protocol":1,"schema_version":1,"catalog_hash":"ee1d3b09a2c21f7f0096bbac5c51ca48b7952358b2b4264b055d23351f6066d0",
 "readonly":false,"desktop_readonly":false,"kiosk_enabled":false,
 "limits":{"dashboards":50,"subscriptions":300,"pages":12,"widgets_per_page":120,"widgets":400,"document_bytes":262144,"history_points":500,"history_range_seconds":604800},
 "templates":["system","home_assistant"],
 "sources":[{"source":"system","state":"idle","at":"2026-11-25T09:58:00Z"},{"source":"ha","state":"idle","at":"2026-11-25T09:58:00Z"}]}
```
- `home_assistant` is listed only when the HA source is enabled and its **cached** catalog has at least one qualifying entity.
- Bootstrap never waits for Home Assistant. A missing or stale catalog is refreshed in the background (one refresh at a time). So the first bootstrap after a server start may omit `home_assistant` even though it can be built.
- **The client calls `bootstrap` again when it opens the "new dashboard" dialog** and builds its template choice from that answer.

### 6. Icon names (D9) and widget `icon` (D8)

One fixed list of HomeDash glyph names. Sources send them as `icon_hint`; templates set them as widget `icon`. The frontend draws every name. Home Assistant's `mdi:` icons are never forwarded. An unknown or absent hint falls back to the kind icon (M2 rule).

| name | sent by | already in `hd-icons.js` |
|---|---|---|
| `cpu` | system `cpu` | yes |
| `memory` | system `memory` | **new** |
| `disk` | system `disk` | yes |
| `download` | system `net_rx` | **new** |
| `upload` | system `net_tx` | **new** |
| `timer` | system `uptime` | **new** |
| `bulb` | ha light | yes |
| `power` | ha switch (not outlet), input_boolean | yes |
| `outlet` | ha switch with device_class `outlet` | **new** |
| `scene` | ha scene | **new** |
| `script` | ha script | **new** |
| `button` | ha button, input_button | **new** |
| `list` | ha select, input_select | **new** |
| `number` | ha number, input_number | **new** |
| `thermometer` | ha climate; sensor with device_class `temperature` | yes |
| `droplet` | ha sensor with device_class `humidity` or `moisture` | yes |
| `gauge` | ha other numeric sensor | yes |
| `lock` | ha lock | yes |
| `brightness` | never sent in M3; control glyph (light sheet, sliders) | **new** |
| `color-temp` | never sent in M3; control glyph | **new** |
| `color` | never sent in M3; control glyph | **new** |
| `effect` | never sent in M3; control glyph | **new** |

Every other ha domain sends no `icon_hint`. The full set of about 70 names completes in M5 W4; names are only ever added.

**Widget `icon`** (new optional document field, schema version stays 1): `{"id":"cpu","type":"gauge","variant":"arc","icon":"cpu",…}`.
- It matches `^[a-z0-9-]{1,48}$`; any other value is a `422 invalid_document` issue `bad_format` at `/pages/N/widgets/M/icon`.
- Absent means "use the bound entity's `icon_hint`".
- The starter template (`{"template":"system"}`) now sets `cpu`, `memory`, `disk`, `download` (net-rx), `upload` (net-tx), `timer` (uptime). Starter dashboards saved under M2 have no `icon`; the entity hints cover them.

### 7. History and snapshot for `ha`

- `GET history?id=ha:…` → `origin:"source"`, no `since`.
- Points come from Home Assistant's recorder: numeric states as numbers, `on`/`off` as 1/0; other states (`unavailable`, text) are skipped. The hub then averages them down to `points`.
- Domains with history: sensor (numeric), number, input_number, switch, input_boolean, binary_sensor. Every other `ha` entity (lights, scenes, …; climate in M3) answers `points: []`.
- A Home Assistant failure answers `502 source_error`.
- `GET snapshot?id=ha:camera.x` → the camera image from `/api/camera_proxy`: `image/jpeg|png|webp`, at most 8 MiB, never stored. `w`/`h` are accepted and ignored for `ha` in M3 (no scaling). Other `ha` entities answer `400 unsupported_command`; failures answer `502 source_error`.

### 8. Stream (delta)

- No new frame. `ha` entities use `snapshot`, `entity`, `removed`, `state` and `source` exactly as in M2.
- `entity` frames arrive when areas, names or capabilities change: registry edits, new `effect_list` or `options`, the policy, or HomeDash gates.
- `removed` arrives when an entity leaves Home Assistant or becomes hidden or disabled.
- `state` frames may come from a service-call answer before the websocket event. Duplicates are possible.
- While `ha` is `degraded` or `unavailable`, subscribed entities get one `state` with `stale:true` (value kept). The next resync clears it.

### 9. `system` first values (D7)

- The server takes its first metrics sample at start (no 10 s wait).
- `cpu`, `memory`, `disk` and `uptime` carry values from then on, including in the first `snapshot` and in `entities`.
- `net_rx` and `net_tx` are `available:false` until the server has two samples (about 10 s after start). From then on they carry values also in the first `snapshot` of a new subscription. Until then the widgets show title and "—".

### 10. Shutdown `revoked` frame (D7 a)

When the server stops, HomeDash closes before the HTTP drain cancels request contexts. Every open stream receives
```
event: revoked
data: {"reason":"shutdown"}
```
and the response ends. The client reconnects with backoff (M2 rule). A crash or a killed process still ends the stream without the frame; treat both the same.

### 11. Error codes (delta to the M2 table)

| HTTP | code | where |
|---|---|---|
| 422 | `template_empty` | `POST dashboards` with `home_assistant` |
| 503 | `source_unavailable` (+`source`) | `commands` (now sent), `POST dashboards` with `home_assistant` |

All other codes are unchanged; `source_unavailable` is no longer "reserved".

### 12. Do not assume

- That `202` means the device changed, or that a `state` frame arrives only from the websocket or only once.
- That `expected.attributes` is the full attribute set (it holds only what the command sets), or that `expected.updated_at` is comparable with Home Assistant times.
- That colour, colour temperature or brightness come back exactly as sent.
- That attributes are present: they may be absent or `null`; read ranges and options from `capabilities`.
- The type of `state.value` for scene, button, select and generic entities (string or null). Do not parse it as a date for scenes or buttons.
- That `area`/`area_id` exist, or that areas arrive with the first snapshot.
- That capability order or the set of capabilities stays stable (an `entity` frame may change them).
- That `bootstrap.templates` is stable between calls.
- That `omitted` exists on the `system` template answer.
- That `ha` history exists for every entity.
- That snapshots respect `w`/`h`.
- That a `color_temp` slider can reach the exact `max` (grid anchored at `min`).
- That every source in `sources` has entities in `entities`.

### 13. Differences from the design (M3)

1. Service calls use the typed sibling `tools.HACallServiceStates`, not `tools.HACallServiceContext` (D3; design-backend 8.1 named the latter).
2. Catalog reads while no run is live use a one-shot websocket session (states and registries), falling back to `GET /api/states` without registries. Home Assistant has no REST registry API.
3. Camera snapshots are not scaled in M3 (design 8.1 "resized like go2rtc"); the camera widget and scaling come in M5.
4. "Push sources mark all their entities stale while degraded" (design 5) is applied to subscribed entities only.
5. Widget `icon` is a new optional document field (D8; the M2 document had none).
6. New HTTP code `template_empty` (422).
7. Bootstrap lists `home_assistant` from the cached catalog and never blocks; the client re-reads bootstrap before showing templates.

---

## Project rules (every task)

- **R1 Worktree.** Work only in the session worktree the app created; run every command there. The main checkout carries other sessions' files.
- **R2 GitNexus.**
  - The GitNexus CLI fails in this repo (storage-version error); only the MCP tools work.
  - Before dispatching a task that edits an existing symbol, the **track lead** runs MCP `impact({target:"<symbol>", direction:"upstream", repo:"AuraGo"})` for each symbol listed under "Impact". The lead pastes callers and risk into the dispatch and warns on HIGH/CRITICAL.
  - `UNKNOWN` or an empty caller set is unresolved: the maker confirms callers with Grep and reports them.
  - Makers **stage only** (`git add <paths>`) and never commit.
  - The track lead runs MCP `detect_changes({scope:"all", repo:"AuraGo", worktree:"<worktree path>"})` and commits by path. A `partial`/`truncated` result is not clean: re-run it.
  - When MCP cannot answer, grep the callers and add `GitNexus unavailable; callers checked with grep: <symbols>` to the commit body.
- **R3 Commits.**
  - One commit per task with only the task's paths: `git commit -m "<message>" -- <paths>`. Never `git add -A`.
  - New `AGENTS.md` files and anything under `docs/` are git-ignored: `git add -f <path>`, then confirm with `git ls-files <path>`.
  - Never `git restore`, `git reset`, `git stash` or `git checkout -- <file>`.
- **R4 Files.**
  - LF line endings, no BOM. No new go.mod module.
  - In Go source never write a literal backslash-u escape (the Write tool decodes it); use `\x5c` if ever needed.
  - No external assets (fonts, libraries, URLs fetched at runtime).
- **R5 Tests.**
  - Use the exact commands given. Group `-run` alternatives as `'(?i)(A|B)'` and count PASS lines with `-v` when a run is evidence.
  - Keep `GOTMPDIR` in backslash form on a short path (`C:\tmp\go-build`).
  - On `no such tool "vet"`, prefix commands with `GOTOOLCHAIN=go1.27.4`.
  - If D: is full, trim `D:\dev-cache\go-build` entries older than 3 days (1 day if that frees nothing).
  - Repository-wide checks (`go build ./...`, `./internal/audit/...`, the full `./internal/server` package) run only in the worktree.
- **R6 Scope.**
  - Never touch `ui/**`, the HomeDash locale sections, `scripts/homedash-tests/**`, the HA Switchboard files (list in "File structure"), `internal/tools/homeassistant.go`, `statusRecorder`, the gzip wrapper or `internal/httpstream`.
  - Never change config keys or the widget catalog and its golden file.
  - No locale key is added in this plan. If one becomes necessary, it needs real translations in all 16 locales and the lead's OK.

---

## Task 0: Workspace

- [ ] Use the branch the app gave you; do not run `git worktree add`. Record `git rev-parse HEAD` and `git status --short` in the session scratchpad file `m3-backend-baseline.txt`.
- [ ] Confirm the base: `git merge-base --is-ancestor 2676770e5 HEAD && echo base-ok`.
- [ ] D7 gap (b) branch check (append the output):
  ```bash
  if git rev-parse --verify --quiet claude/sweet-napier-230a1e >/dev/null; then
    git merge-base --is-ancestor claude/sweet-napier-230a1e HEAD && echo "gap-b branch is in the base" || echo "gap-b branch exists but is NOT in the base"
  else echo "gap-b branch not found locally"; fi
  ```
  If the branch has landed on `main` since this worktree was created but is not in the base, stop and ask the lead whether to restart from `main` (D7).
- [ ] Baseline (append summaries):
  - `go build ./...`
  - `go test ./internal/homedash/... -count=1`
  - `go test ./internal/tools -count=1 -run '(?i)(TestHA)'`
  - `go test ./internal/audit/... -count=1`
  - `go test ./internal/server -count=1 -run '(?i)(HomeDash|Metrics|HTTPDrain|BackgroundTasks|DesktopReadonly)'`
- [ ] Record any failure as test name plus first error line. Later tasks compare against this list. Do not fix unrelated failures.

**Done when:** the scratchpad file lists HEAD, status, the base and branch checks, and the five baseline results.

---

### Task 1: `source` on errors, `failed` audit outcome, `source` in HTTP error bodies

**Files:** modify `internal/homedash/errors.go`, `internal/homedash/command.go`, `internal/server/homedash_routes.go`, `internal/server/homedash_api_commands_test.go`; create `internal/homedash/command_source_test.go`.

**Read first:**
- `internal/homedash/errors.go` (whole file);
- `internal/homedash/command.go:166-185` (`audit`) and `:546-559` (`execute` passes `*Error` through);
- `internal/server/homedash_routes.go:156-221` (`homeDashStatus`, `fail`);
- `internal/homedash/command_test.go:18-32,198-223` (`cmd`, `gateSource`, `SetAuditForTest`);
- `internal/server/homedash_api_commands_test.go:205-209`.

**Impact (lead, MCP):** `Error` (struct), `Hub.audit`, `hdRequest.fail`.

- [ ] Write `internal/homedash/command_source_test.go`:

```go
package homedash_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"aurago/internal/homedash"
)

func TestCommandSourceUnavailableKeepsSourceAndIsAuditedAsFailed(t *testing.T) {
	fa := gateSource()
	fa.CommandErr = homedash.SourceUnavailable("fa", "the source is not reachable")
	h := newTestHub(t, nil, fa)
	var outcomes []string
	homedash.SetAuditForTest(h, func(_ context.Context, _ homedash.Command, outcome string) { outcomes = append(outcomes, outcome) })
	_, err := h.Command(context.Background(), cmd("fa:light", homedash.CapToggle, nil, session("s")))
	var he *homedash.Error
	if !errors.As(err, &he) || he.Code != homedash.CodeSourceUnavailable || he.Source != "fa" {
		t.Fatalf("got %v", err)
	}
	if !errors.Is(err, &homedash.Error{Code: homedash.CodeSourceUnavailable}) {
		t.Fatal("errors.Is must still match by code")
	}
	if !slices.Equal(outcomes, []string{"failed"}) {
		t.Fatalf("outcomes %v, want [failed]", outcomes)
	}
	raw, _ := json.Marshal(he)
	if !strings.Contains(string(raw), `"source":"fa"`) {
		t.Fatalf("json %s", raw)
	}
	plain, _ := json.Marshal(&homedash.Error{Code: homedash.CodeCommandFailed})
	if strings.Contains(string(plain), "source") {
		t.Fatalf("source must be omitted when empty: %s", plain)
	}
}
```

- [ ] Append to `internal/server/homedash_api_commands_test.go`:

```go
func TestHomeDashCommandsSourceUnavailableCarriesSource(t *testing.T) {
	fake := homeDashCommandFake()
	fake.CommandErr = homedash.SourceUnavailable("fake", "the source is not reachable")
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(fake))
	w := env.do(t, env.admin, http.MethodPost, "commands", map[string]any{"entity": "fake:a", "command": "toggle", "request_id": "su-1"})
	var body struct {
		Status, Code, Source string
	}
	hdDecode(t, w, &body)
	if w.Code != http.StatusServiceUnavailable || body.Status != "error" || body.Code != "source_unavailable" || body.Source != "fake" {
		t.Fatalf("answer = %d %s", w.Code, w.Body.String())
	}
	if !env.audits.has("fake:a|toggle|failed") {
		t.Fatalf("audit = %+v", env.audits.entries)
	}
}
```

- [ ] Run `go test ./internal/homedash -count=1 -run '(?i)(TestCommandSourceUnavailable)'`. Expected: build failure (`homedash.SourceUnavailable` undefined).
- [ ] `internal/homedash/errors.go`: add the field after `BlockedBy` and a constructor:

```go
	Source          string  `json:"source,omitempty"` // source id; set with source_unavailable
```

```go
// SourceUnavailable is the refusal for a source that cannot be reached right now. Message is English
// and must not contain upstream text.
func SourceUnavailable(src SourceID, message string) *Error {
	return &Error{Code: CodeSourceUnavailable, Message: message, Source: string(src)}
}
```

- [ ] `internal/homedash/command.go` `audit`: change the refusal case to
  `case errors.As(err, &he) && he.Code != CodeCommandFailed && he.Code != CodeSourceError && he.Code != CodeSourceUnavailable:`
  and add the comment `// a source that cannot be reached is a failure, not a refusal`.
- [ ] `internal/server/homedash_routes.go` `fail`: after the `blocked_by` block add

```go
	if he.Source != "" {
		extra["source"] = he.Source
	}
```

- [ ] Run `go test ./internal/homedash -count=1` and `go test ./internal/server -count=1 -run '(?i)(TestHomeDashCommands)' -v`. Expected: PASS (3 `TestHomeDashCommands*` PASS lines).
- [ ] Stage the five paths. Lead: detect_changes, then commit with `feat(homedash): source on source_unavailable errors, audited as failed`.

**Done when:** both commands pass and a `source_unavailable` body carries `source`.

---

### Task 2: Core additions — `Hub.SourceCatalog`, icon names, widget `icon`

**Files:** create `internal/homedash/hub_catalog.go`, `internal/homedash/hub_catalog_test.go`, `internal/homedash/icons.go`, `internal/homedash/icons_test.go`, `internal/homedash/dashboard_icon_test.go`; modify `internal/homedash/dashboard.go`, `internal/homedash/dashboard_validate.go`.

**Read first:**
- `internal/homedash/hub.go:298-421` (`catalogStale`, `Entities`, `Entity`, `viewLocked`);
- `internal/homedash/hub_lookup.go` (pattern);
- `internal/homedash/hub_sources.go:58-62,279-366`;
- `internal/homedash/dashboard.go:82-93` (`Widget`);
- `internal/homedash/dashboard_validate.go:207-252` (`validator.widget`);
- `internal/homedash/ids.go:24,33` (`ValidIcon`);
- `ui/js/homedash/hd-icons.js:6-27` (existing glyph names, read only).

**Impact (lead, MCP):** `Widget` (struct; used by store, validate, autopack, server templates), `validator.widget`.

- [ ] Write `internal/homedash/hub_catalog_test.go`:

```go
package homedash_test

import (
	"context"
	"testing"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

func TestSourceCatalogReadsRawEntitiesWithoutGatesOrStart(t *testing.T) {
	fa := sourcetest.New("fa", sourcetest.Entity("fa:light", homedash.KindLight, homedash.CapabilitySpec{Name: homedash.CapToggle, Allowed: true}))
	h := newTestHub(t, func(c *homedash.RuntimeConfig) { c.ReadOnly = true }, fa)
	ctx := context.Background()

	list, _, active := h.SourceCatalog(ctx, "fa", false)
	if !active || len(list) != 0 || fa.CatalogCalls() != 0 {
		t.Fatalf("cold cached read = %v active=%v calls=%d", list, active, fa.CatalogCalls())
	}
	list, st, active := h.SourceCatalog(ctx, "fa", true)
	if !active || len(list) != 1 || fa.CatalogCalls() != 1 || st.Source != "fa" {
		t.Fatalf("fetch = %v %+v active=%v calls=%d", list, st, active, fa.CatalogCalls())
	}
	if !list[0].Capabilities[0].Allowed || list[0].Capabilities[0].BlockedBy != "" {
		t.Fatalf("raw read applied the hub gate: %+v", list[0].Capabilities)
	}
	if e, err := h.Entity(ctx, "fa:light"); err != nil || e.Capabilities[0].Allowed {
		t.Fatalf("the entity view must apply homedash.readonly: %+v %v", e, err)
	}
	list[0].Capabilities[0].Allowed = false
	again, _, _ := h.SourceCatalog(ctx, "fa", true)
	if !again[0].Capabilities[0].Allowed || fa.CatalogCalls() != 1 {
		t.Fatalf("copies and freshness: %+v calls=%d", again, fa.CatalogCalls())
	}
	if fa.Runs() != 0 {
		t.Fatal("SourceCatalog started the source")
	}
	if _, _, active := h.SourceCatalog(ctx, "nope", true); active {
		t.Fatal("unknown source reported active")
	}
}
```

- [ ] Write `internal/homedash/icons_test.go`:

```go
package homedash_test

import (
	"testing"

	"aurago/internal/homedash"
)

func TestIconNamesAreValidUniqueAndComplete(t *testing.T) {
	names := homedash.IconNames()
	seen := map[string]bool{}
	for _, n := range names {
		if !homedash.ValidIcon(n) || seen[n] {
			t.Fatalf("bad or duplicate icon name %q", n)
		}
		seen[n] = true
	}
	for _, n := range []string{"cpu", "memory", "disk", "download", "upload", "timer", "bulb", "power", "outlet", "scene",
		"script", "button", "list", "number", "thermometer", "droplet", "gauge", "lock", "brightness", "color-temp", "color", "effect"} {
		if !homedash.KnownIcon(n) {
			t.Errorf("%q missing from the contract list", n)
		}
	}
	if homedash.KnownIcon("mdi:lightbulb") || homedash.KnownIcon("") {
		t.Fatal("foreign icon accepted")
	}
	names[0] = "changed"
	if homedash.IconNames()[0] == "changed" {
		t.Fatal("IconNames must return a copy")
	}
}
```

- [ ] Write `internal/homedash/dashboard_icon_test.go`:

```go
package homedash_test

import (
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/homedash"
)

func TestWidgetIconIsOptionalAndValidated(t *testing.T) {
	raw := `{"name":"I","slug":"i","pages":[{"id":"main","name":"Main","widgets":[{"id":"w1","type":"clock","variant":"digital","icon":"timer","bindings":{},"options":{}}]}]}`
	doc, err := homedash.DecodeDashboard([]byte(raw))
	if err != nil || doc.Pages[0].Widgets[0].Icon != "timer" {
		t.Fatalf("decode = %+v %v", doc, err)
	}
	cat := homedash.Catalog()
	homedash.Normalize(&doc)
	homedash.Autopack(&doc, cat)
	if rep := homedash.Validate(doc, cat, nil); !rep.OK() {
		t.Fatalf("issues = %+v", rep.Issues)
	}
	doc.Pages[0].Widgets[0].Icon = "Bad Icon"
	rep := homedash.Validate(doc, cat, nil)
	if len(rep.Issues) != 1 || rep.Issues[0].Path != "/pages/0/widgets/0/icon" || rep.Issues[0].Code != homedash.IssueBadFormat {
		t.Fatalf("issues = %+v", rep.Issues)
	}
	out, _ := json.Marshal(homedash.Widget{ID: "w", Type: "clock", Variant: "digital"})
	if strings.Contains(string(out), `"icon"`) {
		t.Fatalf("empty icon must be omitted: %s", out)
	}
}
```

- [ ] Run `go test ./internal/homedash -count=1 -run '(?i)(TestSourceCatalog|TestIconNames|TestWidgetIcon)'`. Expected: build failure.
- [ ] Create `internal/homedash/icons.go`:

```go
package homedash

import "slices"

// iconNames is the fixed list of HomeDash glyph names (M3 contract, decision D9). Sources send them
// as Entity.IconHint, templates set them as Widget.Icon; the frontend draws every name. Names are
// only ever added, together with the frontend glyph set (ui/js/homedash/hd-icons.js) and the contract.
var iconNames = []string{
	// system source
	"cpu", "memory", "disk", "download", "upload", "timer",
	// Home Assistant adapter
	"bulb", "power", "outlet", "scene", "script", "button", "list", "number", "thermometer", "droplet", "gauge", "lock",
	// control glyphs of the W1 widgets and the light sheet (never sent as hints in M3)
	"brightness", "color-temp", "color", "effect",
}

// IconNames returns a copy of the fixed icon-name list.
func IconNames() []string { return slices.Clone(iconNames) }

// KnownIcon reports whether name is on the fixed list.
func KnownIcon(name string) bool { return slices.Contains(iconNames, name) }
```

- [ ] Create `internal/homedash/hub_catalog.go`:

```go
package homedash

import (
	"context"
	"slices"
	"sort"
)

// SourceCatalog returns the cataloged entities of one active source as the adapter reported them,
// with their current state, sorted by id. The hub's switches (homedash.readonly,
// virtual_desktop.readonly, the source's control switch) are NOT applied: capabilities carry the
// adapter's own Allowed and BlockedBy. With fetch, a stale catalog is fetched first (CatalogTimeout,
// outside the lock); without fetch only the cached catalog is read. active is false when the source
// is unknown or not enabled or the hub is closed. It adds no interest and never starts the source.
// Templates use it; entity views for clients still go through Entities/Entity.
func (h *Hub) SourceCatalog(ctx context.Context, src SourceID, fetch bool) (list []Entity, status SourceStatus, active bool) {
	h.mu.Lock()
	rt := h.rts[src]
	if rt != nil {
		status = rt.status
	}
	if h.closed || rt == nil || !rt.active || rt.src == nil {
		h.mu.Unlock()
		return []Entity{}, status, false
	}
	var jobs []catalogJob
	if fetch && h.catalogStale(rt, h.opts.Now()) {
		jobs = append(jobs, h.catalogJobLocked(rt))
	}
	h.mu.Unlock()
	h.fetchCatalogs(ctx, jobs)

	h.mu.Lock()
	defer h.mu.Unlock()
	rt = h.rts[src]
	if h.closed || rt == nil || !rt.active {
		return []Entity{}, status, false
	}
	list = make([]Entity, 0, len(rt.catalog))
	for id, e := range rt.catalog {
		if st, ok := h.states[id]; ok {
			e.State = st
		}
		e.Capabilities = slices.Clone(e.Capabilities)
		list = append(list, e)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list, rt.status, true
}
```

- [ ] `internal/homedash/dashboard.go` `Widget`: add `Icon string \`json:"icon,omitempty"\`` right after `Title`, with the comment `// optional glyph name (IconNames); absent = the bound entity's icon_hint`.
- [ ] `internal/homedash/dashboard_validate.go` `validator.widget`: after `v.text(path+"/title", w.Title, 0, 80)` add

```go
	if w.Icon != "" && !ValidIcon(w.Icon) {
		v.err(path+"/icon", IssueBadFormat, "invalid icon id")
	}
```

- [ ] Run `go test ./internal/homedash/... -count=1`. Expected: PASS. The golden catalog test is unchanged, because the catalog does not describe documents.
- [ ] Stage the seven paths. Lead: detect_changes, then commit with `feat(homedash): raw source catalog read, fixed icon names, optional widget icon`.

**Done when:** the package passes and `TestWidgetCatalogGolden` still passes with the same hash.

---

### Task 3: Typed Home Assistant service call in `internal/tools`

**Files:** create `internal/tools/homeassistant_service_states.go`, `internal/tools/homeassistant_service_states_test.go`.

**Read first:**
- `internal/tools/homeassistant.go:15-82` (`haHTTPClient`, `haServiceEndpoint`, `haDo`; do not edit) and `:190-298` (`HACallServiceContext`, `HAServiceRefusal`);
- `internal/tools/homeassistant_targets.go` (`haConcreteEntity`, `HAServiceTargetRefusal`);
- `internal/tools/homeassistant_raw.go` (`HAHTTPError` style);
- `internal/tools/homeassistant_raw_test.go:16-59` (test server pattern);
- `internal/security/scrubber.go:177-182` (`Scrub`).

**Impact:** none. No existing symbol is edited: `HACallServiceContext`, `haDo` and the HA Switchboard stay untouched. Verify with `git diff --stat -- internal/tools/homeassistant.go` (empty).

- [ ] Write `internal/tools/homeassistant_service_states_test.go`:

```go
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"aurago/internal/testutil"
)

func TestHACallServiceStatesReturnsChangedStates(t *testing.T) {
	var mu sync.Mutex
	var method, path, auth string
	var body map[string]any
	ha := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		method, path, auth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Unlock()
		if r.URL.Path == "/api/services/light/toggle" {
			_, _ = w.Write([]byte(`{"changed_states":[{"entity_id":"light.kitchen","state":"off","attributes":{},"last_changed":"2026-11-25T10:00:00.5+00:00","last_updated":"2026-11-25T10:00:00.5+00:00"}],"service_response":null}`))
			return
		}
		_, _ = w.Write([]byte(`[{"entity_id":"light.kitchen","state":"on","attributes":{"brightness":102},"last_changed":"2026-11-25T10:00:00.5+00:00","last_updated":"2026-11-25T10:00:00.5+00:00"},{"entity_id":"bad id","state":"x"}]`))
	}))
	defer ha.Close()
	cfg := HAConfig{URL: ha.URL, AccessToken: "tok"}
	states, err := HACallServiceStates(context.Background(), cfg, "light", "turn_on", "light.kitchen", map[string]any{"brightness_pct": 40.0})
	if err != nil || len(states) != 1 || states[0].State != "on" || states[0].Attributes["brightness"] != 102.0 || states[0].LastUpdated.IsZero() {
		t.Fatalf("states = %+v %v", states, err)
	}
	mu.Lock()
	if method != http.MethodPost || path != "/api/services/light/turn_on" || auth != "Bearer tok" || body["entity_id"] != "light.kitchen" || body["brightness_pct"] != 40.0 {
		t.Fatalf("request = %s %s %q %v", method, path, auth, body)
	}
	mu.Unlock()
	states, err = HACallServiceStates(context.Background(), cfg, "light", "toggle", "light.kitchen", nil)
	if err != nil || len(states) != 1 || states[0].State != "off" {
		t.Fatalf("changed_states shape = %+v %v", states, err)
	}
}

func TestHACallServiceStatesRefusesBeforeTheNetwork(t *testing.T) {
	var hits atomic.Int32
	ha := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); _, _ = w.Write([]byte(`[]`)) }))
	defer ha.Close()
	base := HAConfig{URL: ha.URL, AccessToken: "tok"}
	ro := base
	ro.ReadOnly = true
	blocked := base
	blocked.BlockedServices = []string{"switch.turn_on"}
	for _, tc := range []struct {
		name                    string
		cfg                     HAConfig
		domain, service, entity string
		data                    map[string]any
		kind                    HAServiceErrorKind
	}{
		{"read-only", ro, "switch", "turn_on", "switch.a", nil, HAServicePolicy},
		{"blocked", blocked, "switch", "turn_on", "switch.a", nil, HAServicePolicy},
		{"cover needs allowlist", base, "cover", "open_cover", "cover.g", nil, HAServicePolicy},
		{"script needs allowlist", base, "script", "turn_on", "script.s", nil, HAServicePolicy},
		{"bad domain", base, "Light", "turn_on", "light.a", nil, HAServiceInvalid},
		{"bad service", base, "light", "turn on", "light.a", nil, HAServiceInvalid},
		{"group target", base, "light", "turn_on", "group.all", nil, HAServiceInvalid},
		{"no entity", base, "light", "turn_on", "", nil, HAServiceInvalid},
		{"area in data", base, "light", "turn_on", "light.a", map[string]any{"area_id": "kitchen"}, HAServiceInvalid},
		{"second entity in data", base, "light", "turn_on", "light.a", map[string]any{"entity_id": "light.b"}, HAServiceInvalid},
		{"NaN data", base, "light", "turn_on", "light.a", map[string]any{"brightness_pct": math.NaN()}, HAServiceInvalid},
	} {
		_, err := HACallServiceStates(context.Background(), tc.cfg, tc.domain, tc.service, tc.entity, tc.data)
		var se *HAServiceError
		if !errors.As(err, &se) || se.Kind != tc.kind {
			t.Errorf("%s: %v, want kind %s", tc.name, err, tc.kind)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("server saw %d requests", hits.Load())
	}
}

func TestHACallServiceStatesClassifiesFailuresWithoutUpstreamText(t *testing.T) {
	const secret = "upstream secret detail"
	var status atomic.Int32
	ha := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := int(status.Load())
		w.WriteHeader(code)
		switch code {
		case http.StatusOK + 1000: // marker: unreachable path is tested by closing the server
		}
		if code >= 300 {
			_, _ = w.Write([]byte(`{"message":"` + secret + ` tok-live-123"}`))
			return
		}
		_, _ = w.Write([]byte(`<html>not json</html>`))
	}))
	cfg := HAConfig{URL: ha.URL, AccessToken: "tok-live-123"}
	for _, tc := range []struct {
		status int
		kind   HAServiceErrorKind
	}{{400, HAServiceRejected}, {404, HAServiceRejected}, {401, HAServiceAuth}, {403, HAServiceAuth}, {500, HAServiceUpstream}, {503, HAServiceUpstream}, {200, HAServiceBadAnswer}} {
		status.Store(int32(tc.status))
		_, err := HACallServiceStates(context.Background(), cfg, "switch", "toggle", "switch.a", nil)
		var se *HAServiceError
		if !errors.As(err, &se) || se.Kind != tc.kind || se.Status != tc.status {
			t.Fatalf("HTTP %d: %v", tc.status, err)
		}
		if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "not json") {
			t.Fatalf("HTTP %d: upstream text in Error(): %v", tc.status, err)
		}
		if tc.status >= 300 && (!strings.Contains(se.LogDetail, secret) || strings.Contains(se.LogDetail, "tok-live-123")) {
			t.Fatalf("HTTP %d: log detail %q must keep the text and scrub the token", tc.status, se.LogDetail)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := HACallServiceStates(ctx, cfg, "switch", "toggle", "switch.a", nil)
	var se *HAServiceError
	if !errors.As(err, &se) || se.Kind != HAServiceUnreachable || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled = %v", err)
	}
	ha.Close()
	if _, err := HACallServiceStates(context.Background(), cfg, "switch", "toggle", "switch.a", nil); !errors.As(err, &se) || se.Kind != HAServiceUnreachable {
		t.Fatalf("closed server = %v", err)
	}
}
```

  The `switch` with the marker case inside the handler is not needed; the maker may delete it.

- [ ] Run `go test ./internal/tools -count=1 -run '(?i)(TestHACallServiceStates)'`. Expected: build failure.
- [ ] Create `internal/tools/homeassistant_service_states.go`:

```go
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"aurago/internal/security"
)

// HAChangedState is one entity state Home Assistant reports as changed by a service call.
type HAChangedState struct {
	EntityID    string         `json:"entity_id"`
	State       string         `json:"state"`
	Attributes  map[string]any `json:"attributes"`
	LastChanged time.Time      `json:"last_changed"`
	LastUpdated time.Time      `json:"last_updated"`
}

// HAServiceErrorKind classifies a failed HACallServiceStates call.
type HAServiceErrorKind string

const (
	HAServiceInvalid     HAServiceErrorKind = "invalid"     // bad domain, service, entity id or data; nothing was sent
	HAServicePolicy      HAServiceErrorKind = "policy"      // read-only mode, service or target policy refused; nothing was sent
	HAServiceUnreachable HAServiceErrorKind = "unreachable" // no answer: transport error or cancellation (a slow answer ends at the hub's command timeout first: 502)
	HAServiceAuth        HAServiceErrorKind = "auth"        // HTTP 401 or 403
	HAServiceRejected    HAServiceErrorKind = "rejected"    // another 4xx: Home Assistant refused the call
	HAServiceUpstream    HAServiceErrorKind = "upstream"    // HTTP 5xx
	HAServiceBadAnswer   HAServiceErrorKind = "bad_answer"  // 2xx whose body is not a state list (the call ran)
)

// HAServiceError reports a failed typed service call. Error() never contains Home Assistant's answer.
// Detail is a fixed English text (for policy refusals the shared refusal, which names only the
// service and config keys). LogDetail is a bounded, scrubbed excerpt of the answer for the server log
// only; never show it to a client. Err is the transport or decode error (log only).
type HAServiceError struct {
	Kind      HAServiceErrorKind
	Status    int
	Detail    string
	LogDetail string
	Err       error
}

func (e *HAServiceError) Error() string {
	msg := "home assistant service call failed (" + string(e.Kind) + ")"
	if e.Status != 0 {
		msg += fmt.Sprintf(" HTTP %d", e.Status)
	}
	if e.Kind == HAServicePolicy || e.Kind == HAServiceInvalid {
		msg += ": " + e.Detail
	}
	return msg
}

func (e *HAServiceError) Unwrap() error { return e.Err }

const (
	haServiceAnswerLimit int64 = 2 << 20
	haServiceMaxStates         = 512
	haLogExcerptBytes          = 256
)

var haServiceNameRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// haTargetKeys select targets or override the call; the typed call targets exactly one entity.
var haTargetKeys = []string{"entity_id", "target", "area_id", "device_id", "label_id", "floor_id", "domain", "service"}

// HACallServiceStates calls one Home Assistant service for one concrete entity under the same gates
// as HACallServiceContext (home_assistant.readonly, allowed/blocked services, target refusal) and
// returns the states Home Assistant reports as changed by the call. It is HomeDash's command path;
// HACallServiceContext (agent tool, HA Switchboard) is unchanged.
func HACallServiceStates(ctx context.Context, cfg HAConfig, domain, service, entityID string, data map[string]any) ([]HAChangedState, error) {
	domain, service = strings.ToLower(strings.TrimSpace(domain)), strings.ToLower(strings.TrimSpace(service))
	if !haServiceNameRe.MatchString(domain) || !haServiceNameRe.MatchString(service) {
		return nil, &HAServiceError{Kind: HAServiceInvalid, Detail: "invalid domain or service"}
	}
	if !haConcreteEntity.MatchString(entityID) || strings.HasPrefix(entityID, "group.") {
		return nil, &HAServiceError{Kind: HAServiceInvalid, Detail: "the call needs one concrete entity id"}
	}
	payload := map[string]any{"entity_id": entityID}
	for k, v := range data {
		if slices.Contains(haTargetKeys, k) {
			return nil, &HAServiceError{Kind: HAServiceInvalid, Detail: "service data must not select targets"}
		}
		payload[k] = v
	}
	if cfg.ReadOnly {
		return nil, &HAServiceError{Kind: HAServicePolicy, Detail: "Home Assistant is in read-only mode (home_assistant.readonly)"}
	}
	if msg := HAServiceTargetRefusal(domain, service, payload, cfg.AllowedServices, cfg.BlockedServices); msg != "" {
		return nil, &HAServiceError{Kind: HAServicePolicy, Detail: msg}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &HAServiceError{Kind: HAServiceInvalid, Detail: "service data is not JSON", Err: err}
	}
	raw, code, _, err := haDo(ctx, cfg, http.MethodPost, haServiceEndpoint(domain, service), string(body), haServiceAnswerLimit)
	excerpt := haLogExcerpt(raw)
	switch {
	case err != nil && code == 0:
		return nil, &HAServiceError{Kind: HAServiceUnreachable, Detail: "Home Assistant did not answer", Err: err}
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return nil, &HAServiceError{Kind: HAServiceAuth, Status: code, Detail: "Home Assistant refused the access token", LogDetail: excerpt}
	case code >= 500:
		return nil, &HAServiceError{Kind: HAServiceUpstream, Status: code, Detail: "Home Assistant failed", LogDetail: excerpt}
	case code < 200 || code > 299:
		return nil, &HAServiceError{Kind: HAServiceRejected, Status: code, Detail: "Home Assistant rejected the call", LogDetail: excerpt}
	case err != nil:
		return nil, &HAServiceError{Kind: HAServiceBadAnswer, Status: code, Detail: "the answer could not be read", LogDetail: excerpt, Err: err}
	}
	states, ok := decodeHAChangedStates(raw)
	if !ok {
		return nil, &HAServiceError{Kind: HAServiceBadAnswer, Status: code, Detail: "the answer is not a state list", LogDetail: excerpt}
	}
	return states, nil
}

// decodeHAChangedStates reads a plain state list or {"changed_states": [...]}; an empty body is an
// empty list. Entries with invalid ids are dropped; at most haServiceMaxStates are kept.
func decodeHAChangedStates(raw []byte) ([]HAChangedState, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return []HAChangedState{}, true
	}
	var list []HAChangedState
	if trimmed[0] == '[' {
		if json.Unmarshal(trimmed, &list) != nil {
			return nil, false
		}
	} else {
		var wrapped struct {
			ChangedStates []HAChangedState `json:"changed_states"`
		}
		if json.Unmarshal(trimmed, &wrapped) != nil || wrapped.ChangedStates == nil {
			return nil, false
		}
		list = wrapped.ChangedStates
	}
	out := make([]HAChangedState, 0, min(len(list), haServiceMaxStates))
	for _, st := range list {
		if len(out) >= haServiceMaxStates {
			break
		}
		if haConcreteEntity.MatchString(st.EntityID) {
			out = append(out, st)
		}
	}
	return out, true
}

// haLogExcerpt bounds and scrubs an answer body for the server log. It is never shown to clients.
func haLogExcerpt(raw []byte) string {
	if len(raw) > haLogExcerptBytes {
		raw = raw[:haLogExcerptBytes]
	}
	s := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, strings.ToValidUTF8(string(raw), ""))
	return security.Scrub(strings.TrimSpace(s))
}
```

- [ ] Run `go test ./internal/tools -count=1 -run '(?i)(TestHA)' -v`. Expected: the 3 new tests and every existing `TestHA*` PASS.
- [ ] Stage the two paths. Lead: detect_changes, then commit with `feat(tools): typed Home Assistant service call with changed states and classified errors`.

**Done when:** all `TestHA*` tests pass and `internal/tools/homeassistant.go` is unchanged.

---

### Task 4: `hasstest` — an importable fake Home Assistant

**Files:** create `internal/homedash/sources/hass/hasstest/server.go`, `internal/homedash/sources/hass/hasstest/demo.go`, `internal/homedash/sources/hass/hasstest/server_test.go`.

**Read first:**
- `internal/homedash/sources/hass/ws_client_test.go:18-136` (the B9a fake; its protocol is the reference);
- `internal/homedash/sources/hass/ws_client.go:300-475` (what the client sends and expects);
- `internal/testutil/httptest.go`.

Why a new package: the B9a fake lives in a `_test.go` file and cannot be imported by `internal/server` or by the frontend's `ui` browser test (D13). `hasstest` is a normal package; it must **not** import package `hass`, because `hass` tests import it. The B9a tests keep their own fake (no churn).

**Impact:** none (new package).

- [ ] Write `internal/homedash/sources/hass/hasstest/server_test.go`:

```go
package hasstest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func get(t *testing.T, s *Server, path, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, s.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func post(t *testing.T, s *Server, path string, body any) (*http.Response, []State) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, s.URL+path, bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+s.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out []State
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func TestFakeServesRESTAndRecordsServiceCalls(t *testing.T) {
	s := New(t, DemoOptions())
	if r := get(t, s, "/api/states", ""); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token = %d", r.StatusCode)
	}
	var states []State
	if err := json.NewDecoder(get(t, s, "/api/states", s.Token).Body).Decode(&states); err != nil || len(states) != 15 || s.RESTStateRequests() != 1 {
		t.Fatalf("states = %d %v", len(states), err)
	}
	r, changed := post(t, s, "/api/services/switch/toggle", map[string]any{"entity_id": "switch.coffee"})
	if r.StatusCode != 200 || len(changed) != 1 || changed[0].State != "on" {
		t.Fatalf("toggle = %d %+v", r.StatusCode, changed)
	}
	if c := s.Calls(); len(c) != 1 || c[0].Domain != "switch" || c[0].Service != "toggle" || c[0].Data["entity_id"] != "switch.coffee" {
		t.Fatalf("calls = %+v", c)
	}
	_, changed = post(t, s, "/api/services/light/turn_on", map[string]any{"entity_id": "light.kitchen", "brightness_pct": 60})
	if len(changed) != 1 || changed[0].Attributes["brightness"] != 153.0 {
		t.Fatalf("brightness = %+v", changed)
	}
	s.FailServices(http.StatusServiceUnavailable, "")
	if r, _ := post(t, s, "/api/services/switch/toggle", map[string]any{"entity_id": "switch.coffee"}); r.StatusCode != 503 {
		t.Fatalf("failure switch = %d", r.StatusCode)
	}
	s.FailServices(0, "")
	h := get(t, s, "/api/history/period/2026-11-25T06:00:00Z?filter_entity_id=switch.coffee&end_time=2026-11-25T10:00:00Z&minimal_response&no_attributes", s.Token)
	var series [][]map[string]any
	if err := json.NewDecoder(h.Body).Decode(&series); err != nil || len(series) != 1 || len(series[0]) != 2 || !strings.Contains(s.LastHistoryQuery(), "minimal_response") {
		t.Fatalf("history = %+v %v", series, err)
	}
	if c := get(t, s, "/api/camera_proxy/camera.door", s.Token); c.StatusCode != 200 || c.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("camera = %d %s", c.StatusCode, c.Header.Get("Content-Type"))
	}
}

func TestFakeWebsocketAuthStatesAndEvents(t *testing.T) {
	s := New(t, DemoOptions())
	url := "ws" + strings.TrimPrefix(s.URL, "http") + "/api/websocket"
	dial := func(token string) (*websocket.Conn, string) {
		c, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = c.ReadJSON(&m)
		_ = c.WriteJSON(map[string]any{"type": "auth", "access_token": token})
		_ = c.ReadJSON(&m)
		return c, m["type"].(string)
	}
	if c, typ := dial("wrong"); typ != "auth_invalid" {
		t.Fatalf("wrong token = %s", typ)
	} else {
		c.Close()
	}
	c, typ := dial(s.Token)
	defer c.Close()
	if typ != "auth_ok" {
		t.Fatalf("auth = %s", typ)
	}
	_ = c.WriteJSON(map[string]any{"id": 1, "type": "get_states"})
	var res struct {
		ID     int64   `json:"id"`
		Result []State `json:"result"`
	}
	if err := c.ReadJSON(&res); err != nil || res.ID != 1 || len(res.Result) != 15 {
		t.Fatalf("get_states = %+v %v", res, err)
	}
	_ = c.WriteJSON(map[string]any{"id": 2, "type": "subscribe_events", "event_type": "state_changed"})
	_ = c.ReadJSON(&map[string]any{})
	s.SetState(State{EntityID: "switch.coffee", State: "on"}, true)
	var ev struct {
		ID    int64 `json:"id"`
		Event struct {
			EventType string `json:"event_type"`
			Data      struct {
				NewState State `json:"new_state"`
			} `json:"data"`
		} `json:"event"`
	}
	if err := c.ReadJSON(&ev); err != nil || ev.ID != 2 || ev.Event.EventType != "state_changed" || ev.Event.Data.NewState.State != "on" {
		t.Fatalf("event = %+v %v", ev, err)
	}
	s.BlockWebsocket(true)
	if _, _, err := websocket.DefaultDialer.Dial(url, nil); err == nil {
		t.Fatal("blocked websocket accepted a handshake")
	}
}
```

- [ ] Run `go test ./internal/homedash/sources/hass/hasstest/ -count=1`. Expected: build failure.
- [ ] Create `internal/homedash/sources/hass/hasstest/server.go`:

```go
// Package hasstest is a scripted Home Assistant for tests. It speaks the websocket API HomeDash uses
// (auth, get_states, subscribe_events, the three registry lists, ping) and the REST endpoints
// (GET /api/states, POST /api/services/<domain>/<service>, GET /api/history/period/<from>,
// GET /api/camera_proxy/<entity_id>). Service calls are recorded and applied to the stored states
// the way Home Assistant would, and pushed as state_changed events.
//
// It is a normal (non-test) package so that the server's end-to-end test and the frontend's
// live-handler browser test (package ui) can import it. It must not import package hass.
package hasstest

import (
	"encoding/json"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"aurago/internal/testutil"
)

// UpstreamSecret is the text error answers of the fake carry by default. Tests assert that it never
// reaches a HomeDash client.
const UpstreamSecret = "hasstest-upstream-detail-7f3c"

// State is one entity state as Home Assistant reports it.
type State struct {
	EntityID    string         `json:"entity_id"`
	State       string         `json:"state"`
	Attributes  map[string]any `json:"attributes"`
	LastChanged time.Time      `json:"last_changed"`
	LastUpdated time.Time      `json:"last_updated"`
}

// Area, Device and EntityEntry are registry rows; empty strings are sent as null.
type Area struct{ AreaID, Name string }
type Device struct{ ID, AreaID string }
type EntityEntry struct{ EntityID, DeviceID, AreaID, HiddenBy, DisabledBy string }

// HistoryPoint is one recorder entry.
type HistoryPoint struct {
	State string
	At    time.Time
}

// Call is one recorded service call.
type Call struct {
	Domain, Service string
	Data            map[string]any
}

// Options script the fake.
type Options struct {
	Token             string // default "hasstest-token"
	States            []State
	Areas             []Area
	Devices           []Device
	Entities          []EntityEntry
	History           map[string][]HistoryPoint
	Cameras           map[string][]byte
	CameraContentType string // default "image/jpeg"
	NoEvents          bool   // service calls push no state_changed event
	NoChangedStates   bool   // service calls answer [] instead of the changed states
}

// Server is a running fake Home Assistant.
type Server struct {
	URL   string
	Token string

	opts Options
	srv  *httptest.Server
	up   websocket.Upgrader

	mu          sync.Mutex
	states      map[string]State
	order       []string
	areas       []Area
	devices     []Device
	entities    []EntityEntry
	conns       map[*wsConn]struct{}
	calls       []Call
	wsBlocked   bool
	failStatus  int
	failBody    string
	restStates  int
	wsConnects  int
	historyReqs int
	lastHistory string
}

type wsConn struct {
	mu   sync.Mutex // serialises writes and guards subs
	c    *websocket.Conn
	subs map[string]int64 // event type -> subscription id
}

func (c *wsConn) send(v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = c.c.WriteJSON(v)
}

func (c *wsConn) sub(event string) (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id, ok := c.subs[event]
	return id, ok
}

type change struct{ old, new State }

// New starts the fake on IPv4 loopback; it stops when the test ends.
func New(t testing.TB, o Options) *Server {
	t.Helper()
	if o.Token == "" {
		o.Token = "hasstest-token"
	}
	if o.CameraContentType == "" {
		o.CameraContentType = "image/jpeg"
	}
	s := &Server{Token: o.Token, opts: o, states: map[string]State{}, conns: map[*wsConn]struct{}{},
		areas: slices.Clone(o.Areas), devices: slices.Clone(o.Devices), entities: slices.Clone(o.Entities)}
	for _, st := range o.States {
		s.putLocked(st)
	}
	s.srv = testutil.NewHTTPServer(t, http.HandlerFunc(s.serve))
	s.URL = s.srv.URL
	t.Cleanup(s.Close)
	return s
}

func (s *Server) putLocked(st State) {
	if st.LastUpdated.IsZero() {
		st.LastUpdated = time.Now().UTC()
	}
	if st.LastChanged.IsZero() {
		st.LastChanged = st.LastUpdated
	}
	if st.Attributes == nil {
		st.Attributes = map[string]any{}
	}
	if _, ok := s.states[st.EntityID]; !ok {
		s.order = append(s.order, st.EntityID)
	}
	s.states[st.EntityID] = st
}

// Close drops every websocket and stops the server (Home Assistant is gone). Idempotent.
func (s *Server) Close() {
	s.DropWebsockets()
	s.srv.Close()
}

// DropWebsockets closes every websocket connection (a network blip).
func (s *Server) DropWebsockets() {
	s.mu.Lock()
	conns := make([]*wsConn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	for _, c := range conns {
		_ = c.c.Close()
	}
}

// BlockWebsocket makes the websocket handshake answer 404 (a proxy that blocks websockets).
func (s *Server) BlockWebsocket(block bool) { s.mu.Lock(); s.wsBlocked = block; s.mu.Unlock() }

// FailServices makes service calls answer status with {"message": body} (0 = normal; an empty body
// is UpstreamSecret).
func (s *Server) FailServices(status int, body string) {
	if body == "" {
		body = UpstreamSecret
	}
	s.mu.Lock()
	s.failStatus, s.failBody = status, body
	s.mu.Unlock()
}

func (s *Server) Calls() []Call           { s.mu.Lock(); defer s.mu.Unlock(); return slices.Clone(s.calls) }
func (s *Server) WebsocketConnects() int  { s.mu.Lock(); defer s.mu.Unlock(); return s.wsConnects }
func (s *Server) OpenWebsockets() int     { s.mu.Lock(); defer s.mu.Unlock(); return len(s.conns) }
func (s *Server) RESTStateRequests() int  { s.mu.Lock(); defer s.mu.Unlock(); return s.restStates }
func (s *Server) HistoryRequests() int    { s.mu.Lock(); defer s.mu.Unlock(); return s.historyReqs }
func (s *Server) LastHistoryQuery() string { s.mu.Lock(); defer s.mu.Unlock(); return s.lastHistory }

// State returns one stored state.
func (s *Server) State(id string) (State, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[id]
	return st, ok
}

// SetState stores a state as an integration update would and, with push, sends state_changed to
// every websocket subscribed to it.
func (s *Server) SetState(st State, push bool) {
	if st.LastUpdated.IsZero() {
		st.LastUpdated = time.Now().UTC()
	}
	s.mu.Lock()
	old, had := s.states[st.EntityID]
	s.putLocked(st)
	st = s.states[st.EntityID]
	s.mu.Unlock()
	if push {
		var prev *State
		if had {
			prev = &old
		}
		s.pushEvent("state_changed", map[string]any{"entity_id": st.EntityID, "old_state": prev, "new_state": &st})
	}
}

// SetEntityArea sets the area of one entity registry entry (adding it when missing) and sends
// entity_registry_updated.
func (s *Server) SetEntityArea(entityID, areaID string) {
	s.mu.Lock()
	i := slices.IndexFunc(s.entities, func(e EntityEntry) bool { return e.EntityID == entityID })
	if i < 0 {
		s.entities = append(s.entities, EntityEntry{EntityID: entityID})
		i = len(s.entities) - 1
	}
	s.entities[i].AreaID = areaID
	s.mu.Unlock()
	s.pushEvent("entity_registry_updated", map[string]any{"action": "update", "entity_id": entityID})
}

func (s *Server) pushEvent(event string, data any) {
	s.mu.Lock()
	conns := make([]*wsConn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	for _, c := range conns {
		if id, ok := c.sub(event); ok {
			c.send(map[string]any{"id": id, "type": "event",
				"event": map[string]any{"event_type": event, "data": data, "origin": "LOCAL", "time_fired": time.Now().UTC()}})
		}
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/websocket" {
		s.serveWebsocket(w, r)
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+s.Token {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"message": "Unauthorized"})
		return
	}
	p := r.URL.Path
	switch {
	case r.Method == http.MethodGet && p == "/api/states":
		s.mu.Lock()
		s.restStates++
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, s.stateList())
	case r.Method == http.MethodPost && strings.HasPrefix(p, "/api/services/"):
		s.serveService(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/api/history/period/"):
		s.serveHistory(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/api/camera_proxy/"):
		img, ok := s.opts.Cameras[strings.TrimPrefix(p, "/api/camera_proxy/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", s.opts.CameraContentType)
		_, _ = w.Write(img)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) stateList() []State {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]State, 0, len(s.order))
	for _, id := range s.order {
		if st, ok := s.states[id]; ok {
			out = append(out, st)
		}
	}
	return out
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func (s *Server) registry(kind string) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []map[string]any{}
	switch kind {
	case "areas":
		for _, a := range s.areas {
			out = append(out, map[string]any{"area_id": a.AreaID, "name": a.Name})
		}
	case "devices":
		for _, d := range s.devices {
			out = append(out, map[string]any{"id": d.ID, "area_id": nullable(d.AreaID)})
		}
	default:
		for _, e := range s.entities {
			out = append(out, map[string]any{"entity_id": e.EntityID, "device_id": nullable(e.DeviceID), "area_id": nullable(e.AreaID),
				"hidden_by": nullable(e.HiddenBy), "disabled_by": nullable(e.DisabledBy)})
		}
	}
	return out
}

func (s *Server) serveService(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/services/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.NotFound(w, r)
		return
	}
	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "Invalid JSON"})
		return
	}
	s.mu.Lock()
	s.calls = append(s.calls, Call{Domain: parts[0], Service: parts[1], Data: data})
	if s.failStatus != 0 {
		code, body := s.failStatus, s.failBody
		s.mu.Unlock()
		writeJSON(w, code, map[string]any{"message": body})
		return
	}
	changes, code := s.applyLocked(parts[0], parts[1], data, time.Now().UTC())
	s.mu.Unlock()
	if code != http.StatusOK {
		writeJSON(w, code, map[string]any{"message": UpstreamSecret})
		return
	}
	out := []State{}
	for _, c := range changes {
		if !s.opts.NoEvents {
			s.pushEvent("state_changed", map[string]any{"entity_id": c.new.EntityID, "old_state": &c.old, "new_state": &c.new})
		}
		if !s.opts.NoChangedStates {
			out = append(out, c.new)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func stringsOf(v any) []string {
	raw, _ := v.([]any)
	out := []string{}
	for _, e := range raw {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

// applyLocked applies a service call like Home Assistant does for the domains HomeDash controls.
func (s *Server) applyLocked(domain, service string, data map[string]any, now time.Time) ([]change, int) {
	id, _ := data["entity_id"].(string)
	old, ok := s.states[id]
	if !ok || !strings.HasPrefix(id, domain+".") {
		return nil, http.StatusBadRequest
	}
	st := old
	st.Attributes = maps.Clone(old.Attributes)
	if st.Attributes == nil {
		st.Attributes = map[string]any{}
	}
	switch service {
	case "turn_on":
		switch domain {
		case "scene":
			st.State = now.Format(time.RFC3339Nano)
		case "script": // runs and ends at once: the state stays "off"
		default:
			st.State = "on"
		}
		if domain == "light" {
			applyLight(st.Attributes, data, &st.State)
		}
	case "turn_off":
		st.State = "off"
		if domain == "light" {
			st.Attributes["brightness"] = nil
		}
	case "toggle":
		if st.State == "on" {
			st.State = "off"
			if domain == "light" {
				st.Attributes["brightness"] = nil
			}
		} else {
			st.State = "on"
		}
	case "select_option":
		opt, _ := data["option"].(string)
		if !slices.Contains(stringsOf(st.Attributes["options"]), opt) {
			return nil, http.StatusBadRequest
		}
		st.State = opt
	case "set_value":
		v, ok := number(data["value"])
		if !ok {
			return nil, http.StatusBadRequest
		}
		st.State = strconv.FormatFloat(v, 'f', 1, 64)
	case "press":
		st.State = now.Format(time.RFC3339Nano)
	default:
		return nil, http.StatusBadRequest
	}
	st.LastUpdated = now
	if st.State != old.State {
		st.LastChanged = now
	}
	s.states[id] = st
	return []change{{old: old, new: st}}, http.StatusOK
}

func applyLight(attrs, data map[string]any, state *string) {
	if v, ok := number(data["brightness_pct"]); ok {
		if v <= 0 {
			*state, attrs["brightness"] = "off", nil
			return
		}
		attrs["brightness"] = math.Round(v * 255 / 100)
	} else if attrs["brightness"] == nil && slices.ContainsFunc(stringsOf(attrs["supported_color_modes"]), func(m string) bool { return m != "onoff" }) {
		attrs["brightness"] = 255.0
	}
	if v, ok := number(data["color_temp_kelvin"]); ok {
		attrs["color_temp_kelvin"], attrs["color_mode"] = v, "color_temp"
	}
	if v, ok := data["hs_color"].([]any); ok && len(v) == 2 {
		attrs["hs_color"], attrs["color_mode"] = v, "hs"
	}
	if v, ok := data["effect"].(string); ok {
		attrs["effect"] = v
	}
}

func (s *Server) serveHistory(w http.ResponseWriter, r *http.Request) {
	from, err := time.Parse(time.RFC3339, strings.TrimPrefix(r.URL.Path, "/api/history/period/"))
	q := r.URL.Query()
	end, err2 := time.Parse(time.RFC3339, q.Get("end_time"))
	if err != nil || err2 != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "Invalid datetime"})
		return
	}
	id := q.Get("filter_entity_id")
	s.mu.Lock()
	s.historyReqs++
	s.lastHistory = r.URL.RawQuery
	pts := slices.Clone(s.opts.History[id])
	s.mu.Unlock()
	var series []map[string]any
	for _, p := range pts {
		if p.At.Before(from) || p.At.After(end) {
			continue
		}
		e := map[string]any{"state": p.State, "last_changed": p.At.UTC().Format(time.RFC3339Nano)}
		if len(series) == 0 {
			e["entity_id"] = id
		}
		series = append(series, e)
	}
	out := []any{}
	if len(series) > 0 {
		out = append(out, series)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) serveWebsocket(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	blocked := s.wsBlocked
	s.mu.Unlock()
	if blocked {
		http.NotFound(w, r)
		return
	}
	c, err := s.up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn := &wsConn{c: c, subs: map[string]int64{}}
	s.mu.Lock()
	s.wsConnects++
	s.conns[conn] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
		_ = c.Close()
	}()
	conn.send(map[string]any{"type": "auth_required", "ha_version": "2026.11.0"})
	var auth struct {
		Type        string `json:"type"`
		AccessToken string `json:"access_token"`
	}
	if c.ReadJSON(&auth) != nil {
		return
	}
	if auth.Type != "auth" || auth.AccessToken != s.Token {
		conn.send(map[string]any{"type": "auth_invalid", "message": "Invalid access token or password"})
		return
	}
	conn.send(map[string]any{"type": "auth_ok", "ha_version": "2026.11.0"})
	for {
		var m struct {
			ID        int64  `json:"id"`
			Type      string `json:"type"`
			EventType string `json:"event_type"`
		}
		if c.ReadJSON(&m) != nil {
			return
		}
		var result any
		switch m.Type {
		case "ping":
			conn.send(map[string]any{"id": m.ID, "type": "pong"})
			continue
		case "subscribe_events":
			conn.mu.Lock()
			conn.subs[m.EventType] = m.ID
			conn.mu.Unlock()
		case "get_states":
			result = s.stateList()
		case "config/area_registry/list":
			result = s.registry("areas")
		case "config/device_registry/list":
			result = s.registry("devices")
		case "config/entity_registry/list":
			result = s.registry("entities")
		default:
			conn.send(map[string]any{"id": m.ID, "type": "result", "success": false, "error": map[string]any{"code": "unknown_command", "message": "Unknown command."}})
			continue
		}
		conn.send(map[string]any{"id": m.ID, "type": "result", "success": true, "result": result})
	}
}
```

- [ ] Create `internal/homedash/sources/hass/hasstest/demo.go`:

```go
package hasstest

import "time"

// DemoTime is the last_updated of every demo state.
var DemoTime = time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)

// demoJPEG is a minimal JPEG start and end marker; enough for content sniffing.
var demoJPEG = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9}

// DemoOptions is a small home that covers every W1 domain, areas from the entity and from the device
// registry, a hidden entity, a policy-blocked script, a non-W1 sensor, an excluded alarm panel, a
// camera and recorder history. The M3 contract examples are taken from it. Each call returns fresh
// maps and slices.
func DemoOptions() Options {
	st := func(id, state string, attrs map[string]any) State {
		return State{EntityID: id, State: state, Attributes: attrs, LastChanged: DemoTime, LastUpdated: DemoTime}
	}
	return Options{
		States: []State{
			st("light.kitchen", "on", map[string]any{"friendly_name": "Kitchen", "brightness": 102.0, "color_mode": "color_temp",
				"color_temp_kelvin": 3003.0, "min_color_temp_kelvin": 2202.0, "max_color_temp_kelvin": 6535.0, "hs_color": []any{27.2, 56.1},
				"supported_color_modes": []any{"color_temp", "hs"}, "effect": "none", "effect_list": []any{"none", "colorloop"},
				"supported_features": 44.0, "icon": "mdi:ceiling-light", "entity_picture": "/api/image/serve/abc?token=demo-secret"}),
			st("switch.coffee", "off", map[string]any{"friendly_name": "Coffee machine", "device_class": "outlet"}),
			st("select.heater_mode", "comfort", map[string]any{"friendly_name": "Heater mode", "options": []any{"eco", "comfort", "boost"}}),
			st("light.sofa", "off", map[string]any{"friendly_name": "Sofa lamp", "supported_color_modes": []any{"onoff"}, "color_mode": nil}),
			st("scene.movie", "2026-11-24T20:15:00.000000+00:00", map[string]any{"friendly_name": "Movie night"}),
			st("input_number.volume", "35.0", map[string]any{"friendly_name": "Volume", "min": 0.0, "max": 100.0, "step": 5.0, "mode": "slider"}),
			st("sensor.outdoor_temperature", "4.5", map[string]any{"friendly_name": "Outdoor", "device_class": "temperature", "unit_of_measurement": "°C", "state_class": "measurement"}),
			st("input_boolean.vacation_mode", "off", map[string]any{"friendly_name": "Vacation mode"}),
			st("input_boolean.guest_mode", "off", map[string]any{"friendly_name": "Guest mode"}),
			st("button.doorbell_test", "unknown", map[string]any{"friendly_name": "Doorbell test"}),
			st("input_select.radio", "Jazz", map[string]any{"friendly_name": "Radio", "options": []any{"Jazz", "Rock", "News", "Classic", "Pop", "Lounge"}}),
			st("number.boiler_target", "55.0", map[string]any{"friendly_name": "Boiler target", "min": 30.0, "max": 70.0, "step": 1.0, "mode": "box", "unit_of_measurement": "°C"}),
			st("script.goodnight", "off", map[string]any{"friendly_name": "Good night"}),
			st("alarm_control_panel.home", "disarmed", map[string]any{"friendly_name": "Alarm"}),
			st("camera.door", "idle", map[string]any{"friendly_name": "Door", "entity_picture": "/api/camera_proxy/camera.door?token=demo-secret", "access_token": "demo-secret"}),
		},
		Areas:   []Area{{AreaID: "kitchen", Name: "Kitchen"}, {AreaID: "living_room", Name: "Living room"}, {AreaID: "garage", Name: "Garage"}},
		Devices: []Device{{ID: "dev_kitchen_light", AreaID: "kitchen"}, {ID: "dev_sofa", AreaID: "living_room"}},
		Entities: []EntityEntry{
			{EntityID: "light.kitchen", DeviceID: "dev_kitchen_light"},                 // area from the device
			{EntityID: "switch.coffee", AreaID: "kitchen"},                             // own area
			{EntityID: "select.heater_mode", DeviceID: "dev_sofa", AreaID: "kitchen"}, // own area wins over the device's
			{EntityID: "light.sofa", DeviceID: "dev_sofa"},
			{EntityID: "scene.movie", AreaID: "living_room"},
			{EntityID: "input_number.volume", AreaID: "living_room"},
			{EntityID: "sensor.outdoor_temperature", AreaID: "living_room"},
			{EntityID: "input_boolean.guest_mode", HiddenBy: "user"},
		},
		History: map[string][]HistoryPoint{
			"sensor.outdoor_temperature": {{"4.0", DemoTime.Add(-2 * time.Hour)}, {"unavailable", DemoTime.Add(-90 * time.Minute)}, {"4.5", DemoTime.Add(-time.Hour)}},
			"switch.coffee":              {{"on", DemoTime.Add(-2 * time.Hour)}, {"off", DemoTime.Add(-time.Hour)}},
		},
		Cameras: map[string][]byte{"camera.door": append([]byte(nil), demoJPEG...)},
	}
}
```

- [ ] Run `go test ./internal/homedash/sources/hass/hasstest/ -count=1 -v`. Expected: 2 PASS.
- [ ] Stage the three paths. Lead: detect_changes, then commit with `test(homedash): importable fake Home Assistant with websocket and REST`.

**Done when:** both fake tests pass and the package does not import `aurago/internal/homedash/sources/hass`.

---

### Task 5: Catalog mapping (areas, hidden filter, state, icons) and the one-shot websocket fetch

**Files:** create `internal/homedash/sources/hass/catalog.go`, `internal/homedash/sources/hass/ws_fetch.go`, `internal/homedash/sources/hass/catalog_test.go`.

**Read first:**
- `domains.go` (whole file);
- `attributes.go`;
- `ws_messages.go`;
- `ws_client.go:139-151,211-324` (`NewWSClient`, `session`, `authenticate`, `write`);
- `internal/homedash/entity.go`;
- `internal/homedash/sanitize.go:13-20`;
- contract §2 and §6.

**Impact:** none. These are new symbols only, and `WSClient` gains a method (`Fetch`) in a new file.

- [ ] Write `internal/homedash/sources/hass/catalog_test.go`:

```go
package hass

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass/hasstest"
	"aurago/internal/homedash/sourcetest"
	"aurago/internal/tools"
)

var fastWS = WSOptions{HandshakeTimeout: time.Second, PingInterval: time.Hour, PongTimeout: time.Minute, AuthRetry: time.Hour,
	RegistryDebounce: 20 * time.Millisecond, Backoff: []time.Duration{20 * time.Millisecond}, Jitter: -1}

var discardLog = slog.New(slog.NewTextHandler(io.Discard, nil))

func demoStates(o hasstest.Options) map[string]HAState {
	out := map[string]HAState{}
	for _, s := range o.States {
		out[s.EntityID] = HAState{EntityID: s.EntityID, State: s.State, Attributes: s.Attributes, LastChanged: s.LastChanged, LastUpdated: s.LastUpdated}
	}
	return out
}

func demoRegistries(o hasstest.Options) Registries {
	var r Registries
	for _, a := range o.Areas {
		r.Areas = append(r.Areas, AreaEntry{AreaID: a.AreaID, Name: a.Name})
	}
	for _, d := range o.Devices {
		r.Devices = append(r.Devices, DeviceEntry{ID: d.ID, AreaID: d.AreaID})
	}
	for _, e := range o.Entities {
		r.Entities = append(r.Entities, EntityEntry{EntityID: e.EntityID, DeviceID: e.DeviceID, AreaID: e.AreaID, HiddenBy: e.HiddenBy, DisabledBy: e.DisabledBy})
	}
	return r
}

func demoCatalog(cfg tools.HAConfig) map[homedash.EntityID]homedash.Entity {
	o := hasstest.DemoOptions()
	out := map[homedash.EntityID]homedash.Entity{}
	for _, e := range buildCatalog(cfg, demoStates(o), indexRegistries(demoRegistries(o))) {
		out[e.ID] = e
	}
	return out
}

func TestHAAreasFromEntityThenDeviceRegistry(t *testing.T) {
	c := demoCatalog(tools.HAConfig{})
	for id, want := range map[homedash.EntityID][2]string{
		"ha:light.kitchen":         {"kitchen", "Kitchen"},         // device area
		"ha:select.heater_mode":    {"kitchen", "Kitchen"},         // entity area wins over its device's living_room
		"ha:light.sofa":            {"living_room", "Living room"}, // device area
		"ha:button.doorbell_test":  {"", ""},                       // no registry entry
		"ha:input_number.volume":   {"living_room", "Living room"},
	} {
		if e := c[id]; e.AreaID != want[0] || e.Area != want[1] {
			t.Errorf("%s area = %q/%q, want %q/%q", id, e.AreaID, e.Area, want[0], want[1])
		}
	}
}

func TestHACatalogExcludesHiddenDisabledAndAlarm(t *testing.T) {
	c := demoCatalog(tools.HAConfig{})
	if len(c) != 13 {
		t.Fatalf("catalog has %d entities, want 13 (15 states - hidden guest_mode - alarm panel)", len(c))
	}
	for _, id := range []homedash.EntityID{"ha:input_boolean.guest_mode", "ha:alarm_control_panel.home"} {
		if _, ok := c[id]; ok {
			t.Errorf("%s must be excluded", id)
		}
	}
	reg := Registries{Entities: []EntityEntry{{EntityID: "switch.a", DisabledBy: "user"}}}
	list := buildCatalog(tools.HAConfig{}, map[string]HAState{"switch.a": {EntityID: "switch.a", State: "on"}, "Bad.id": {EntityID: "Bad.id"}}, indexRegistries(reg))
	if len(list) != 0 {
		t.Fatalf("disabled or invalid entities listed: %+v", list)
	}
}

func TestHAStateMappingAndAttributes(t *testing.T) {
	c := demoCatalog(tools.HAConfig{})
	k := c["ha:light.kitchen"]
	if k.Kind != homedash.KindLight || k.Name != "Kitchen" || k.IconHint != "bulb" || k.State.Value != true || k.State.Text != "on" ||
		k.State.Attributes["brightness_pct"] != 40.0 || !k.State.UpdatedAt.Equal(hasstest.DemoTime) || k.State.UpdatedAt.Location() != time.UTC {
		t.Fatalf("kitchen = %+v", k)
	}
	for _, key := range []string{"icon", "entity_picture", "supported_features", "friendly_name", "brightness"} {
		if _, ok := k.State.Attributes[key]; ok {
			t.Errorf("attribute %s forwarded", key)
		}
	}
	if got := capNames(k.Capabilities); !slices.Equal(got, []string{"toggle", "turn_on", "turn_off", "brightness", "color_temp", "color", "effect"}) {
		t.Fatalf("kitchen caps = %v", got)
	}
	if ct, _ := findCap(k.Capabilities, homedash.CapColorTemp); *ct.Min != 2202 || *ct.Max != 6535 || *ct.Step != 50 {
		t.Fatalf("color_temp = %+v", ct)
	}
	if s := c["ha:light.sofa"]; s.State.Value != false || !slices.Equal(capNames(s.Capabilities), []string{"toggle", "turn_on", "turn_off"}) {
		t.Fatalf("sofa = %+v", s)
	}
	if s := c["ha:switch.coffee"]; s.IconHint != "outlet" || s.DeviceClass != "outlet" || s.State.Value != false {
		t.Fatalf("coffee = %+v", s)
	}
	if b := c["ha:button.doorbell_test"]; b.State.Value != nil || b.State.Text != "unknown" || !b.State.Available || b.IconHint != "button" {
		t.Fatalf("button = %+v", b)
	}
	if n := c["ha:number.boiler_target"]; n.State.Value != 55.0 || n.Unit != "°C" || n.IconHint != "number" {
		t.Fatalf("number = %+v", n)
	}
	if s := c["ha:select.heater_mode"]; s.State.Value != "comfort" || s.IconHint != "list" {
		t.Fatalf("select = %+v", s)
	}
	if s := c["ha:script.goodnight"]; !s.ReadOnly || s.IconHint != "script" {
		t.Fatalf("script = %+v", s)
	}
	if s := c["ha:sensor.outdoor_temperature"]; s.IconHint != "thermometer" || s.State.Value != 4.5 {
		t.Fatalf("sensor = %+v", s)
	}
	cam := c["ha:camera.door"]
	if cam.Media == nil || !cam.Media.Snapshot || cam.State.Attributes["access_token"] != nil || cam.State.Attributes["entity_picture"] != nil {
		t.Fatalf("camera = %+v", cam)
	}
	if st := mapState("switch", HAState{EntityID: "switch.x", State: "unavailable"}); st.Available || st.Value != nil {
		t.Fatalf("unavailable = %+v", st)
	}
}

func TestHAIconHintsAreOnTheFixedList(t *testing.T) {
	domains := []string{"alarm_control_panel", "automation", "timer"}
	for d := range domainTable {
		domains = append(domains, d)
	}
	for _, d := range domains {
		for _, dc := range []string{"", "outlet", "temperature", "humidity", "moisture", "door"} {
			for _, numeric := range []bool{false, true} {
				if h := iconHint(d, dc, numeric); h != "" && !homedash.KnownIcon(h) {
					t.Errorf("iconHint(%s, %s, %v) = %q is not on the contract list", d, dc, numeric, h)
				}
			}
		}
	}
}

func TestHAWebsocketFetchOnce(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	c, err := NewWSClient(ha.URL, ha.Token, nopHandler{}, fastWS)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	states, reg, err := c.Fetch(ctx)
	if err != nil || len(states) != 15 || len(reg.Areas) != 3 || len(reg.Devices) != 2 || len(reg.Entities) != 8 {
		t.Fatalf("fetch = %d states, %+v, %v", len(states), reg, err)
	}
	sourcetest.WaitFor(t, time.Second, func() bool { return ha.OpenWebsockets() == 0 }, "the one-shot session closes")
	bad, _ := NewWSClient(ha.URL, "wrong", nopHandler{}, fastWS)
	if _, _, err := bad.Fetch(ctx); err == nil || !errorsIs(err, errAuthInvalid) {
		t.Fatalf("wrong token = %v", err)
	}
}
```

  Use `errors.Is` directly; `errorsIs` is a placeholder, so add the `errors` import.

- [ ] Run `go test ./internal/homedash/sources/hass/ -count=1 -run '(?i)(TestHAAreas|TestHACatalog|TestHAStateMapping|TestHAIconHints|TestHAWebsocketFetchOnce)'`. Expected: build failure.
- [ ] Create `internal/homedash/sources/hass/catalog.go`:

```go
package hass

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"aurago/internal/homedash"
	"aurago/internal/tools"
)

// SourceID is the HomeDash source id of Home Assistant; entity ids are "ha:<domain>.<object_id>".
const SourceID homedash.SourceID = "ha"

// Keys of the homedash.SourceSettings the server builds (internal/server/homedash_sources.go).
const (
	ParamURL            = "url"
	ParamToken          = "token"
	ParamReadOnly       = "readonly"
	ListAllowedServices = "allowed_services"
	ListBlockedServices = "blocked_services"
)

func domainOf(native string) (string, bool) {
	if !NativeIDValid(native) {
		return "", false
	}
	d, _, _ := strings.Cut(native, ".")
	return d, true
}

// iconHint maps a domain and device class to the fixed HomeDash icon list (decision D9). "" lets the
// frontend use the kind icon. Home Assistant's own mdi: icons are never forwarded.
func iconHint(domain, deviceClass string, numeric bool) string {
	switch domain {
	case "light":
		return "bulb"
	case "switch":
		if deviceClass == "outlet" {
			return "outlet"
		}
		return "power"
	case "input_boolean":
		return "power"
	case "scene":
		return "scene"
	case "script":
		return "script"
	case "button", "input_button":
		return "button"
	case "select", "input_select":
		return "list"
	case "number", "input_number":
		return "number"
	case "climate":
		return "thermometer"
	case "lock":
		return "lock"
	case "sensor":
		switch deviceClass {
		case "temperature":
			return "thermometer"
		case "humidity", "moisture":
			return "droplet"
		}
		if numeric {
			return "gauge"
		}
	}
	return ""
}

// registryIndex resolves areas and the hidden/disabled filter from the three registries.
type registryIndex struct {
	areaName map[string]string      // area id -> name
	device   map[string]string      // device id -> area id
	entity   map[string]EntityEntry // entity id -> registry entry
}

func indexRegistries(reg Registries) registryIndex {
	ix := registryIndex{areaName: map[string]string{}, device: map[string]string{}, entity: map[string]EntityEntry{}}
	for _, a := range reg.Areas {
		ix.areaName[a.AreaID] = a.Name
	}
	for _, d := range reg.Devices {
		ix.device[d.ID] = d.AreaID
	}
	for _, e := range reg.Entities {
		ix.entity[e.EntityID] = e
	}
	return ix
}

func (ix registryIndex) hidden(entityID string) bool {
	e, ok := ix.entity[entityID]
	return ok && (e.HiddenBy != "" || e.DisabledBy != "")
}

// area returns the entity registry area, else the device's area, and the area's display name (the
// id when the area registry has no name for it).
func (ix registryIndex) area(entityID string) (id, name string) {
	e := ix.entity[entityID]
	id = e.AreaID
	if id == "" && e.DeviceID != "" {
		id = ix.device[e.DeviceID]
	}
	if id == "" {
		return "", ""
	}
	if name = ix.areaName[id]; name == "" {
		name = id
	}
	return id, name
}

var (
	onOffDomains   = map[string]bool{"switch": true, "input_boolean": true, "light": true, "fan": true, "humidifier": true, "binary_sensor": true, "script": true, "automation": true}
	numericDomains = map[string]bool{"sensor": true, "number": true, "input_number": true}
)

// mapState turns a Home Assistant state into a HomeDash state (contract section 2): on/off domains
// carry a bool, numeric domains a finite number, others the state text; "unavailable" is
// available:false without a value, "unknown" is available without a value. UpdatedAt is
// last_updated (else last_changed) in UTC.
func mapState(domain string, st HAState) homedash.State {
	_, attrs := FilterAttributes(st.Attributes)
	at := st.LastUpdated
	if at.IsZero() {
		at = st.LastChanged
	}
	out := homedash.State{Text: st.State, Attributes: attrs, Available: true, UpdatedAt: at.UTC()}
	switch st.State {
	case "unavailable":
		out.Available = false
		return out
	case "unknown", "":
		return out
	}
	switch {
	case onOffDomains[domain] && (st.State == "on" || st.State == "off"):
		out.Value = st.State == "on"
	case numericDomains[domain]:
		if f, err := strconv.ParseFloat(st.State, 64); err == nil && finite(f) {
			out.Value = f
		} else {
			out.Value = st.State
		}
	default:
		out.Value = st.State
	}
	return out
}

// entityFor maps one state to a HomeDash entity; ok is false for invalid ids, excluded domains and
// registry-hidden or -disabled entities.
func entityFor(cfg tools.HAConfig, st HAState, ix registryIndex) (homedash.Entity, bool) {
	domain, ok := domainOf(st.EntityID)
	if !ok {
		return homedash.Entity{}, false
	}
	spec, ok := MapDomain(domain)
	if !ok || ix.hidden(st.EntityID) {
		return homedash.Entity{}, false
	}
	name, _ := FilterAttributes(st.Attributes)
	dc, _ := st.Attributes["device_class"].(string)
	unit, _ := st.Attributes["unit_of_measurement"].(string)
	caps := Capabilities(cfg, st.EntityID, st.State, st.Attributes)
	numeric := slices.ContainsFunc(caps, func(c homedash.CapabilitySpec) bool { return c.Name == homedash.CapNumeric })
	areaID, area := ix.area(st.EntityID)
	e := homedash.Entity{ID: homedash.MakeEntityID(SourceID, st.EntityID), Source: SourceID, Kind: spec.Kind, Name: name,
		Area: area, AreaID: areaID, DeviceClass: dc, Unit: unit, IconHint: iconHint(domain, dc, numeric), Capabilities: caps,
		ReadOnly: true, State: mapState(domain, st)}
	for _, c := range caps {
		if info, ok := homedash.LookupCapability(c.Name); ok && info.Command && c.Allowed {
			e.ReadOnly = false
		}
	}
	if spec.Kind == homedash.KindCamera {
		e.Media = &homedash.MediaInfo{Snapshot: true}
	}
	return e, true
}

// buildCatalog maps every known state, sorted by entity id, at most homedash.MaxCatalogEntities.
func buildCatalog(cfg tools.HAConfig, states map[string]HAState, ix registryIndex) []homedash.Entity {
	ids := make([]string, 0, len(states))
	for id := range states {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]homedash.Entity, 0, min(len(ids), homedash.MaxCatalogEntities))
	for _, id := range ids {
		if len(out) >= homedash.MaxCatalogEntities {
			break
		}
		if e, ok := entityFor(cfg, states[id], ix); ok {
			out = append(out, e)
		}
	}
	return out
}
```

- [ ] Create `internal/homedash/sources/hass/ws_fetch.go`:

```go
package hass

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"aurago/internal/security"
)

// nopHandler ignores websocket callbacks (one-shot fetches use none).
type nopHandler struct{}

func (nopHandler) OnStates([]HAState)          {}
func (nopHandler) OnStateChanged(StateChanged) {}
func (nopHandler) OnRegistries(Registries)     {}
func (nopHandler) OnStatus(WSStatus)           {}

// Fetch opens one websocket session, reads get_states and the three registry lists, and closes the
// session. It is the adapter's catalog read while no run is live (Home Assistant has no REST registry
// API). A failed registry list leaves that list empty; a failed get_states, a rejected token
// (errAuthInvalid) or the end of ctx is an error. The token only travels in the auth message.
func (c *WSClient) Fetch(ctx context.Context) ([]HAState, Registries, error) {
	if c.token != "" {
		security.RegisterSensitive(c.token)
	}
	dctx, cancel := context.WithTimeout(ctx, c.o.HandshakeTimeout)
	conn, resp, err := c.dialer.DialContext(dctx, c.url, nil)
	cancel()
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return nil, Registries{}, errors.Join(errDial, err)
	}
	defer conn.Close()
	conn.SetReadLimit(c.o.ReadLimit)
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	s := &wsSession{c: c, conn: conn, pending: map[int64]string{}, pendingBatch: map[int64]int64{}}
	if err := s.authenticate(); err != nil {
		return nil, Registries{}, err
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetReadDeadline(dl)
	}
	want := map[int64]string{1: kindStates, 2: kindAreas, 3: kindDevices, 4: kindEntities}
	for id := int64(1); id <= 4; id++ {
		typ := "get_states"
		if k := want[id]; k != kindStates {
			typ = registryCommand[k]
		}
		if err := s.write(map[string]any{"id": id, "type": typ}); err != nil {
			return nil, Registries{}, err
		}
	}
	var states []HAState
	var reg Registries
	for len(want) > 0 {
		var m wsMsg
		if err := conn.ReadJSON(&m); err != nil {
			if ctx.Err() != nil {
				return nil, Registries{}, ctx.Err()
			}
			return nil, Registries{}, err
		}
		kind, ok := want[m.ID]
		if m.Type != "result" || !ok {
			continue
		}
		delete(want, m.ID)
		success := m.Success != nil && *m.Success
		switch kind {
		case kindStates:
			if !success {
				return nil, Registries{}, errProtocol
			}
			if err := json.Unmarshal(m.Result, &states); err != nil {
				return nil, Registries{}, fmt.Errorf("hass: decode states: %w", err)
			}
		case kindAreas:
			if success {
				_ = json.Unmarshal(m.Result, &reg.Areas)
			}
		case kindDevices:
			if success {
				_ = json.Unmarshal(m.Result, &reg.Devices)
			}
		case kindEntities:
			if success {
				_ = json.Unmarshal(m.Result, &reg.Entities)
			}
		}
	}
	return states, reg, nil
}
```

- [ ] Run `go test ./internal/homedash/sources/hass/... -count=1`. Expected: PASS (new tests plus every B9a test).
- [ ] Stage the three paths. Lead: detect_changes, then commit with `feat(homedash): Home Assistant catalog mapping, areas, hidden filter, icons and one-shot fetch`.

**Done when:** the hass package passes and the catalog matches contract §2 on the demo home.

---

### Task 6: Adapter — Source, catalog read, state stream, reconnect, REST fallback

**Files:** create `internal/homedash/sources/hass/adapter.go`, `internal/homedash/sources/hass/rest_fallback.go`, `internal/homedash/sources/hass/adapter_test.go`, `internal/homedash/sources/hass/rest_fallback_test.go`.

**Read first:**
- `internal/homedash/source.go`;
- `internal/homedash/hub_sources.go:428-493` (`hubSink`: `Update` drops updates older than the stored state and keeps states only for cataloged or subscribed natives; `Catalog` pushes `entity` frames only for meta changes, **not** state frames);
- `internal/homedash/AGENTS.md` locking contract;
- `ws_client.go:22-67,171-209`;
- Task 5 code; contract §2, §3, §8.

**Impact:** none (new symbols).

**Locking rule of the adapter:**
- `a.mu` guards all adapter state. It is held while calling the sink, which is safe because the hub never calls a `Source` method while holding `Hub.mu` and sinks never call back into the source.
- `a.mu` is never held across network I/O.
- Websocket callbacks, `Command`, `Catalog` and `SetInterest` all take `a.mu`.

- [ ] Write `internal/homedash/sources/hass/adapter_test.go`:

```go
package hass

import (
	"context"
	"sync"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass/hasstest"
	"aurago/internal/homedash/sourcetest"
	"aurago/internal/tools"
)

type lockedSink struct {
	mu       sync.Mutex
	catalogs [][]homedash.Entity
	updates  []homedash.StateUpdate
	statuses []homedash.SourceStatus
}

func (s *lockedSink) Catalog(e []homedash.Entity)      { s.mu.Lock(); s.catalogs = append(s.catalogs, e); s.mu.Unlock() }
func (s *lockedSink) Update(u ...homedash.StateUpdate) { s.mu.Lock(); s.updates = append(s.updates, u...); s.mu.Unlock() }
func (s *lockedSink) Status(st homedash.SourceStatus)  { s.mu.Lock(); s.statuses = append(s.statuses, st); s.mu.Unlock() }

func (s *lockedSink) sawStatus(state, reason string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.statuses {
		if st.State == state && (reason == "" || st.Reason == reason) {
			return true
		}
	}
	return false
}

func (s *lockedSink) lastStatus() homedash.SourceStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.statuses) == 0 {
		return homedash.SourceStatus{}
	}
	return s.statuses[len(s.statuses)-1]
}

func (s *lockedSink) last(native string) (homedash.State, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.updates) - 1; i >= 0; i-- {
		if s.updates[i].Native == native {
			return s.updates[i].State, true
		}
	}
	return homedash.State{}, false
}

func (s *lockedSink) sawStale(native string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.updates {
		if u.Native == native && u.State.Stale {
			return true
		}
	}
	return false
}

func (s *lockedSink) entity(id homedash.EntityID) (homedash.Entity, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.catalogs) == 0 {
		return homedash.Entity{}, false
	}
	for _, e := range s.catalogs[len(s.catalogs)-1] {
		if e.ID == id {
			return e, true
		}
	}
	return homedash.Entity{}, false
}

func newTestAdapter(t *testing.T, ha *hasstest.Server, cfg tools.HAConfig) *Adapter {
	t.Helper()
	if cfg.URL == "" {
		cfg.URL = ha.URL
	}
	if cfg.AccessToken == "" {
		cfg.AccessToken = ha.Token
	}
	return New(cfg, 5, FactoryOptions{Logger: discardLog, WS: fastWS, RetryWebsocket: 300 * time.Millisecond,
		PollOverride: 30 * time.Millisecond, FetchTimeout: time.Second})
}

func runAdapter(t *testing.T, a *Adapter, sink homedash.Sink) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = a.Run(ctx, sink); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("Run did not return after cancel")
		}
	})
}

func TestHAFactoryGates(t *testing.T) {
	f := Factory(FactoryOptions{})
	rc := func(mut func(*homedash.SourceSettings)) homedash.RuntimeConfig {
		set := homedash.SourceSettings{Enabled: true, Control: true, PollSeconds: 5, ConfigKey: "home_assistant",
			Params: map[string]string{ParamURL: "http://ha.lan:8123", ParamToken: "tok", ParamReadOnly: "false"},
			Lists:  map[string][]string{ListAllowedServices: {"light.turn_on"}}}
		if mut != nil {
			mut(&set)
		}
		return homedash.RuntimeConfig{Sources: map[homedash.SourceID]homedash.SourceSettings{SourceID: set}}
	}
	src, ok := f(rc(nil))
	if !ok || src.ID() != SourceID || src.(*Adapter).ha.AllowedServices[0] != "light.turn_on" || src.(*Adapter).ha.ReadOnly {
		t.Fatalf("factory = %v %v", src, ok)
	}
	for name, mut := range map[string]func(*homedash.SourceSettings){
		"off":         func(s *homedash.SourceSettings) { s.Enabled = false },
		"no token":    func(s *homedash.SourceSettings) { s.Params[ParamToken] = "" },
		"bad url":     func(s *homedash.SourceSettings) { s.Params[ParamURL] = "ftp://ha.lan" },
		"credentials": func(s *homedash.SourceSettings) { s.Params[ParamURL] = "http://u:p@ha.lan" },
	} {
		if _, ok := f(rc(mut)); ok {
			t.Errorf("%s: enabled", name)
		}
	}
	if _, ok := f(homedash.RuntimeConfig{}); ok {
		t.Error("missing settings entry enabled the source")
	}
	ro, _ := f(rc(func(s *homedash.SourceSettings) { s.Params[ParamReadOnly] = "true"; s.PollSeconds = 1 }))
	if a := ro.(*Adapter); !a.ha.ReadOnly || a.poll != 5*time.Second {
		t.Fatalf("read-only/poll = %+v %v", a.ha, a.poll)
	}
}

func TestHAAdapterCatalogWithoutRun(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	a := newTestAdapter(t, ha, tools.HAConfig{})
	ctx := context.Background()
	list, err := a.Catalog(ctx)
	if err != nil || len(list) != 13 || ha.WebsocketConnects() != 1 {
		t.Fatalf("catalog = %d entities, %v, %d websocket sessions", len(list), err, ha.WebsocketConnects())
	}
	area := func(list []homedash.Entity, id homedash.EntityID) string {
		for _, e := range list {
			if e.ID == id {
				return e.AreaID
			}
		}
		return "missing"
	}
	if area(list, "ha:light.kitchen") != "kitchen" {
		t.Fatalf("kitchen area = %q", area(list, "ha:light.kitchen"))
	}
	ha.BlockWebsocket(true)
	list, err = a.Catalog(ctx)
	if err != nil || ha.RESTStateRequests() != 1 || area(list, "ha:light.kitchen") != "kitchen" {
		t.Fatalf("REST catalog keeps the last registries: %v rest=%d area=%q", err, ha.RESTStateRequests(), area(list, "ha:light.kitchen"))
	}
	b := newTestAdapter(t, ha, tools.HAConfig{})
	if list, err = b.Catalog(ctx); err != nil || area(list, "ha:light.kitchen") != "" || len(list) != 14 {
		t.Fatalf("REST catalog without registries: %v area=%q n=%d", err, area(list, "ha:light.kitchen"), len(list))
	}
}

func TestHAAdapterStreamsStatesAndReconnects(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"switch.coffee"})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool {
		e, ok := sink.entity("ha:light.kitchen")
		return sink.sawStatus(homedash.StatusLive, "") && ok && e.AreaID == "kitchen"
	}, "live catalog with areas")

	ha.SetState(hasstest.State{EntityID: "switch.coffee", State: "on", Attributes: map[string]any{"friendly_name": "Coffee machine", "device_class": "outlet"}}, true)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { st, ok := sink.last("switch.coffee"); return ok && st.Value == true }, "state_changed update")

	ha.DropWebsockets()
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusDegraded, "") && sink.sawStale("switch.coffee") }, "degraded and stale")
	sourcetest.WaitFor(t, 2*time.Second, func() bool {
		st, ok := sink.last("switch.coffee")
		return sink.lastStatus().State == homedash.StatusLive && ok && !st.Stale && st.Value == true
	}, "resync after reconnect")

	ha.SetEntityArea("switch.coffee", "living_room")
	sourcetest.WaitFor(t, 2*time.Second, func() bool { e, ok := sink.entity("ha:switch.coffee"); return ok && e.AreaID == "living_room" }, "catalog after a registry update")
}
```

- [ ] Write `internal/homedash/sources/hass/rest_fallback_test.go`:

```go
package hass

import (
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass/hasstest"
	"aurago/internal/homedash/sourcetest"
	"aurago/internal/tools"
)

func TestHAFallsBackToRESTPolling(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	ha.BlockWebsocket(true)
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"switch.coffee"})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusPolling, ReasonWebsocketUnavailable) }, "polling after three failed handshakes")
	n := ha.RESTStateRequests()
	ha.SetState(hasstest.State{EntityID: "switch.coffee", State: "on", Attributes: map[string]any{"friendly_name": "Coffee machine"}}, false)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { st, ok := sink.last("switch.coffee"); return ok && st.Value == true }, "polled change")
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return ha.RESTStateRequests() > n+1 }, "polling every interval")

	ha.BlockWebsocket(false)
	sourcetest.WaitFor(t, 3*time.Second, func() bool { return sink.lastStatus().State == homedash.StatusLive }, "websocket again after the polling window")
	m := ha.RESTStateRequests()
	time.Sleep(150 * time.Millisecond)
	if ha.RESTStateRequests() != m {
		t.Fatal("polling continued while the websocket is live")
	}
}

func TestHAPollingNeedsInterest(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	ha.BlockWebsocket(true)
	a := newTestAdapter(t, ha, tools.HAConfig{})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusPolling, "") }, "polling")
	n := ha.RESTStateRequests()
	time.Sleep(150 * time.Millisecond)
	if ha.RESTStateRequests() != n {
		t.Fatalf("polled without interest: %d -> %d", n, ha.RESTStateRequests())
	}
}
```

- [ ] Run `go test ./internal/homedash/sources/hass/ -count=1 -run '(?i)(TestHAFactory|TestHAAdapter|TestHAFallsBack|TestHAPolling)'`. Expected: build failure.
- [ ] Create `internal/homedash/sources/hass/adapter.go`:

```go
package hass

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/tools"
)

const (
	// ReasonWebsocketUnavailable is the status reason while the adapter polls REST.
	ReasonWebsocketUnavailable = "websocket_unavailable"
	// ReasonInvalidURL reports a configured URL the websocket cannot use.
	ReasonInvalidURL = "invalid_url"

	wsFailuresBeforePolling = 3
)

// FactoryOptions are the adapter's dependencies; zero values take the defaults.
type FactoryOptions struct {
	Logger         *slog.Logger
	WS             WSOptions        // websocket timing; zero = the documented defaults
	Now            func() time.Time // default time.Now
	RetryWebsocket time.Duration    // REST polling lasts this long before one websocket attempt (5 min)
	PollOverride   time.Duration    // tests only: replaces poll_seconds
	FetchTimeout   time.Duration    // REST probe and poll timeout (10 s)
}

func (o FactoryOptions) withDefaults() FactoryOptions {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.RetryWebsocket <= 0 {
		o.RetryWebsocket = 5 * time.Minute
	}
	if o.FetchTimeout <= 0 {
		o.FetchTimeout = 10 * time.Second
	}
	return o
}

// Factory builds the adapter from the runtime configuration. It is disabled without a settings
// entry, when switched off (the server already combined the source switch, home_assistant.enabled,
// URL and token), without a token, or with a URL the websocket cannot use. It never blocks.
func Factory(o FactoryOptions) homedash.Factory {
	o = o.withDefaults()
	return func(cfg homedash.RuntimeConfig) (homedash.Source, bool) {
		set, ok := cfg.Sources[SourceID]
		if !ok || !set.Enabled {
			return nil, false
		}
		ha := tools.HAConfig{URL: set.Params[ParamURL], AccessToken: set.Params[ParamToken], ReadOnly: set.Params[ParamReadOnly] == "true",
			AllowedServices: slices.Clone(set.Lists[ListAllowedServices]), BlockedServices: slices.Clone(set.Lists[ListBlockedServices])}
		if ha.AccessToken == "" {
			return nil, false
		}
		if _, err := WebsocketURL(ha.URL); err != nil {
			return nil, false
		}
		return New(ha, set.PollSeconds, o), true
	}
}

// Adapter is the Home Assistant source (design-backend 8.1, decision D1).
type Adapter struct {
	ha   tools.HAConfig
	poll time.Duration
	o    FactoryOptions
	log  *slog.Logger

	mu          sync.Mutex // guards everything below; held while calling the sink, never across network I/O
	states      map[string]HAState
	reg         Registries
	ix          registryIndex
	complete    bool          // states hold a full list (get_states, a REST poll or a one-shot fetch)
	runStates   bool          // the current run has its own full list (Catalog serves from it)
	delivered   bool          // the current run delivered states at least once
	sink        homedash.Sink // the current run's sink; nil outside Run
	interest    map[string]bool
	metaFP      map[string]string // entity id -> meta fingerprint of the last pushed catalog
	catalogFP   string
	staleMarked bool
}

var (
	_ homedash.Source         = (*Adapter)(nil)
	_ homedash.HistorySource  = (*Adapter)(nil)
	_ homedash.SnapshotSource = (*Adapter)(nil)
)

// New builds an adapter; pollSeconds outside 2..60 becomes 5.
func New(ha tools.HAConfig, pollSeconds int, o FactoryOptions) *Adapter {
	o = o.withDefaults()
	if pollSeconds < 2 || pollSeconds > 60 {
		pollSeconds = 5
	}
	poll := time.Duration(pollSeconds) * time.Second
	if o.PollOverride > 0 {
		poll = o.PollOverride
	}
	return &Adapter{ha: ha, poll: poll, o: o, log: o.Logger, states: map[string]HAState{}, ix: indexRegistries(Registries{}),
		interest: map[string]bool{}, metaFP: map[string]string{}}
}

func (a *Adapter) ID() homedash.SourceID { return SourceID }

// SetInterest stores the subscribed natives; REST polling runs only while there are some.
func (a *Adapter) SetInterest(natives []string) {
	set := make(map[string]bool, len(natives))
	for _, n := range natives {
		set[n] = true
	}
	a.mu.Lock()
	a.interest = set
	a.mu.Unlock()
}

func (a *Adapter) interested() bool { a.mu.Lock(); defer a.mu.Unlock(); return len(a.interest) > 0 }

func (a *Adapter) attach(sink homedash.Sink) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sink, a.runStates, a.delivered, a.staleMarked, a.catalogFP = sink, false, false, false, ""
}

func (a *Adapter) detach(sink homedash.Sink) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sink == sink {
		a.sink, a.runStates = nil, false
	}
}

func (a *Adapter) statusLocked(state, reason string) {
	if a.sink != nil {
		a.sink.Status(homedash.SourceStatus{Source: SourceID, State: state, Reason: reason, At: a.o.Now().UTC()})
	}
}

func (a *Adapter) status(state, reason string) { a.mu.Lock(); a.statusLocked(state, reason); a.mu.Unlock() }

// Catalog works whether a run is live or not. A live run serves its own states; otherwise one
// websocket session reads states and registries (REST /api/states without registries when the
// websocket cannot be used; the last known registries then apply).
func (a *Adapter) Catalog(ctx context.Context) ([]homedash.Entity, error) {
	a.mu.Lock()
	if a.sink != nil && a.runStates {
		list := buildCatalog(a.ha, a.states, a.ix)
		a.mu.Unlock()
		return list, nil
	}
	a.mu.Unlock()
	states, reg, err := a.fetchOnce(ctx)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !(a.sink != nil && a.runStates) { // a run that became live meanwhile wins
		next := make(map[string]HAState, len(states))
		for _, st := range states {
			if NativeIDValid(st.EntityID) {
				next[st.EntityID] = st
			}
		}
		a.states, a.complete = next, true
		if reg != nil {
			a.reg, a.ix = *reg, indexRegistries(*reg)
		}
	}
	return buildCatalog(a.ha, a.states, a.ix), nil
}

func (a *Adapter) fetchOnce(ctx context.Context) ([]HAState, *Registries, error) {
	c, err := NewWSClient(a.ha.URL, a.ha.AccessToken, nopHandler{}, a.o.WS)
	if err == nil {
		states, reg, ferr := c.Fetch(ctx)
		if ferr == nil {
			return states, &reg, nil
		}
		if errors.Is(ferr, errAuthInvalid) || ctx.Err() != nil {
			return nil, nil, ferr
		}
		err = ferr
	}
	var list []HAState
	if rerr := tools.HARequestJSON(ctx, a.ha, http.MethodGet, "/api/states", nil, &list); rerr != nil {
		return nil, nil, errors.Join(err, rerr)
	}
	return list, nil, nil
}

func metaFingerprint(e homedash.Entity) string {
	e.State = homedash.State{}
	raw, _ := json.Marshal(e)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

// pushCatalogLocked sends the catalog when its meta (not only states) changed since the last push.
func (a *Adapter) pushCatalogLocked() {
	if a.sink == nil || !a.complete {
		return
	}
	list := buildCatalog(a.ha, a.states, a.ix)
	fps := make(map[string]string, len(list))
	h := sha256.New()
	for _, e := range list {
		fp := metaFingerprint(e)
		fps[string(e.ID)] = fp
		h.Write([]byte(string(e.ID) + "=" + fp + ";"))
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if sum == a.catalogFP {
		return
	}
	a.catalogFP, a.metaFP = sum, fps
	a.sink.Catalog(list)
}

func (a *Adapter) metaChangedLocked(st HAState) bool {
	id := string(homedash.MakeEntityID(SourceID, st.EntityID))
	e, ok := entityFor(a.ha, st, a.ix)
	if !ok {
		_, had := a.metaFP[id]
		return had
	}
	return a.metaFP[id] != metaFingerprint(e)
}

func sameHAState(x, y HAState) bool { return x.State == y.State && x.LastUpdated.Equal(y.LastUpdated) }

// replaceStatesLocked installs a full state list (get_states or a REST poll): catalog first when its
// meta changed (the hub keeps states only for cataloged entities), then updates for the states that
// changed or were marked stale (a catalog push alone sends no state frames).
func (a *Adapter) replaceStatesLocked(list []HAState) {
	prev := a.states
	next := make(map[string]HAState, len(list))
	for _, st := range list {
		if NativeIDValid(st.EntityID) {
			next[st.EntityID] = st
		}
	}
	a.states, a.complete = next, true
	if a.sink == nil {
		return
	}
	a.pushCatalogLocked()
	var ups []homedash.StateUpdate
	for id, st := range next {
		if old, ok := prev[id]; ok && !a.staleMarked && sameHAState(old, st) {
			continue
		}
		d, _ := domainOf(id)
		ups = append(ups, homedash.StateUpdate{Native: id, State: mapState(d, st)})
	}
	a.staleMarked = false
	if len(ups) > 0 {
		a.sink.Update(ups...)
	}
}

// applyStateLocked installs one state (state_changed or a service answer). An older state is
// ignored; the same state twice (answer, then event) is sent once.
func (a *Adapter) applyStateLocked(st HAState) {
	if !NativeIDValid(st.EntityID) {
		return
	}
	old, had := a.states[st.EntityID]
	if had && !st.LastUpdated.IsZero() && st.LastUpdated.Before(old.LastUpdated) {
		return
	}
	if had && sameHAState(old, st) && !a.staleMarked {
		return
	}
	a.states[st.EntityID] = st
	if a.sink == nil {
		return
	}
	if !had || a.metaChangedLocked(st) {
		a.pushCatalogLocked()
	}
	d, _ := domainOf(st.EntityID)
	a.sink.Update(homedash.StateUpdate{Native: st.EntityID, State: mapState(d, st)})
}

func (a *Adapter) removeStateLocked(id string) {
	if _, ok := a.states[id]; !ok {
		return
	}
	delete(a.states, id)
	a.pushCatalogLocked()
}

// markStaleLocked sends the subscribed entities once with stale:true (design 5: push sources mark
// their entities stale while degraded). The next full list clears it.
func (a *Adapter) markStaleLocked() {
	if a.sink == nil || a.staleMarked {
		return
	}
	var ups []homedash.StateUpdate
	for native := range a.interest {
		st, ok := a.states[native]
		d, okd := domainOf(native)
		if !ok || !okd {
			continue
		}
		s := mapState(d, st)
		s.Stale = true
		ups = append(ups, homedash.StateUpdate{Native: native, State: s})
	}
	a.staleMarked = true
	if len(ups) > 0 {
		a.sink.Update(ups...)
	}
}

func (a *Adapter) markStale() { a.mu.Lock(); a.markStaleLocked(); a.mu.Unlock() }

// wsStatus maps websocket client statuses to source statuses and asks for a REST probe after limit
// failed handshakes in a row.
func (a *Adapter) wsStatus(st WSStatus, limit int, probe chan<- struct{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch st.State {
	case WSLive:
		a.statusLocked(homedash.StatusLive, "")
	case WSConnecting:
		if !a.delivered {
			a.statusLocked(homedash.StatusStarting, "")
		}
	case WSDegraded:
		a.statusLocked(homedash.StatusDegraded, st.Reason)
		a.markStaleLocked()
		if st.Failures >= limit {
			select {
			case probe <- struct{}{}:
			default:
			}
		}
	case WSAuthFailed:
		a.statusLocked(homedash.StatusUnavailable, ReasonAuthFailed)
		a.markStaleLocked()
	}
}

// wsBridge feeds websocket callbacks into the adapter (all on the client's Run goroutine).
type wsBridge struct {
	a     *Adapter
	limit int
	probe chan<- struct{}
}

func (b *wsBridge) OnStates(list []HAState) {
	a := b.a
	a.mu.Lock()
	defer a.mu.Unlock()
	a.runStates, a.delivered = true, true
	a.replaceStatesLocked(list)
}

func (b *wsBridge) OnStateChanged(sc StateChanged) {
	a := b.a
	a.mu.Lock()
	defer a.mu.Unlock()
	if sc.NewState == nil {
		a.removeStateLocked(sc.EntityID)
		return
	}
	st := *sc.NewState
	if st.EntityID == "" {
		st.EntityID = sc.EntityID
	}
	a.applyStateLocked(st)
}

func (b *wsBridge) OnRegistries(reg Registries) {
	a := b.a
	a.mu.Lock()
	defer a.mu.Unlock()
	a.reg, a.ix = reg, indexRegistries(reg)
	a.pushCatalogLocked()
}

func (b *wsBridge) OnStatus(st WSStatus) { b.a.wsStatus(st, b.limit, b.probe) }

// Run connects while the hub has interest. It uses the websocket first. After
// wsFailuresBeforePolling failed handshakes in a row while GET /api/states still answers (a proxy
// that blocks websockets), it polls REST for RetryWebsocket, then makes one websocket attempt, and
// so on. A failing poll means Home Assistant itself is down: the websocket client's backoff
// (1-60 s, +-20 %) takes over again.
func (a *Adapter) Run(ctx context.Context, sink homedash.Sink) error {
	a.attach(sink)
	defer a.detach(sink)
	limit := wsFailuresBeforePolling
	for ctx.Err() == nil {
		if !a.websocketPhase(ctx, limit) {
			return nil
		}
		if a.pollingPhase(ctx) {
			limit = 1
		} else {
			limit = wsFailuresBeforePolling
		}
	}
	return nil
}

// websocketPhase runs the websocket client until ctx ends (false) or the client failed limit
// handshakes in a row while a REST poll succeeds (true: switch to polling). The client's callbacks
// have returned when it returns.
func (a *Adapter) websocketPhase(ctx context.Context, limit int) bool {
	probe := make(chan struct{}, 1)
	client, err := NewWSClient(a.ha.URL, a.ha.AccessToken, &wsBridge{a: a, limit: limit, probe: probe}, a.o.WS)
	if err != nil {
		a.status(homedash.StatusUnavailable, ReasonInvalidURL)
		<-ctx.Done()
		return false
	}
	wctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = client.Run(wctx) }()
	defer func() { cancel(); <-done }()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-done:
			return false
		case <-probe:
			if a.pollOnce(ctx) == nil {
				return true
			}
		}
	}
}
```

- [ ] Create `internal/homedash/sources/hass/rest_fallback.go`:

```go
package hass

import (
	"context"
	"net/http"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/tools"
)

// pollingPhase reads GET /api/states every poll interval while there is interest, for at most
// RetryWebsocket (design-backend 8.1). It returns true when the window (or ctx) ended and false when
// a poll failed: Home Assistant itself is unreachable, so the websocket backoff takes over again.
func (a *Adapter) pollingPhase(ctx context.Context) bool {
	a.status(homedash.StatusPolling, ReasonWebsocketUnavailable)
	window := time.NewTimer(a.o.RetryWebsocket)
	defer window.Stop()
	tick := time.NewTicker(a.poll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return true
		case <-window.C:
			return true
		case <-tick.C:
		}
		if !a.interested() {
			continue
		}
		if err := a.pollOnce(ctx); err != nil {
			if ctx.Err() != nil {
				return true
			}
			a.status(homedash.StatusDegraded, ReasonUnreachable)
			a.markStale()
			return false
		}
	}
}

// pollOnce reads every state over REST and applies it like a websocket get_states.
func (a *Adapter) pollOnce(ctx context.Context) error {
	pctx, cancel := context.WithTimeout(ctx, a.o.FetchTimeout)
	defer cancel()
	var list []HAState
	if err := tools.HARequestJSON(pctx, a.ha, http.MethodGet, "/api/states", nil, &list); err != nil {
		a.log.Debug("homedash: Home Assistant REST poll failed", "error", err)
		return err
	}
	a.mu.Lock()
	a.runStates, a.delivered = true, true
	a.replaceStatesLocked(list)
	a.mu.Unlock()
	return nil
}
```

  `Command`, `History` and `Snapshot` arrive in Tasks 7 and 8. Until then the interface assertions in `adapter.go` do not compile. In this task, add temporary stubs at the end of `rest_fallback.go`:

```go
// Temporary until Tasks 7-8 (removed there).
func (a *Adapter) Command(context.Context, homedash.Command) (homedash.CommandResult, error) {
	return homedash.CommandResult{}, &homedash.Error{Code: homedash.CodeUnsupportedCommand, Message: "not yet"}
}
func (a *Adapter) History(context.Context, string, time.Time, time.Time, int) ([]homedash.Sample, error) {
	return []homedash.Sample{}, nil
}
func (a *Adapter) Snapshot(context.Context, string, int, int) ([]byte, string, error) {
	return nil, "", &homedash.Error{Code: homedash.CodeUnsupportedCommand, Message: "not yet"}
}
```

- [ ] Run `go test ./internal/homedash/sources/hass/... -count=1 -v`. Expected: every test PASS. Then run the stream tests three times: `go test ./internal/homedash/sources/hass/ -count=3 -run '(?i)(TestHAAdapter|TestHAFallsBack|TestHAPolling)'`.
- [ ] Stage the four paths. Lead: detect_changes, then commit with `feat(homedash): Home Assistant adapter with state stream, reconnect and REST fallback`.

**Done when:** the tests pass three times in a row and the source runs against the fake with status, catalog and updates as in contract §3.

---

### Task 7: Commands — verb mapping, `expected`, changed-state feed, error mapping

**Files:** create `internal/homedash/sources/hass/commands.go`, `internal/homedash/sources/hass/commands_test.go`; modify `internal/homedash/sources/hass/rest_fallback.go` (remove the `Command` stub).

**Read first:**
- `domains.go:46-85,132-142` (`domainTable` services, `blockedBy`);
- `internal/homedash/command.go:486-565` (the hub passes `*homedash.Error` through, maps other errors to `command_failed`, sanitizes `Expected` with `BoundAttributes`: lists must be `[]any`);
- Task 3 (`HAServiceError` kinds);
- contract §4.

**Impact:** none (new symbols; the stub removal is in this track's own file).

- [ ] Write `internal/homedash/sources/hass/commands_test.go`:

```go
package hass

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass/hasstest"
	"aurago/internal/homedash/sourcetest"
	"aurago/internal/tools"
)

func TestHAVerbMapping(t *testing.T) {
	for _, tc := range []struct {
		native          string
		c               homedash.Capability
		args            map[string]any
		domain, service string
		data            map[string]any
	}{
		{"switch.coffee", homedash.CapToggle, nil, "switch", "toggle", map[string]any{}},
		{"input_boolean.vacation_mode", homedash.CapTurnOn, nil, "input_boolean", "turn_on", map[string]any{}},
		{"light.kitchen", homedash.CapTurnOff, nil, "light", "turn_off", map[string]any{}},
		{"light.kitchen", homedash.CapBrightness, map[string]any{"value": 40.0}, "light", "turn_on", map[string]any{"brightness_pct": 40.0}},
		{"light.kitchen", homedash.CapColorTemp, map[string]any{"kelvin": 3502.0}, "light", "turn_on", map[string]any{"color_temp_kelvin": 3502.0}},
		{"light.kitchen", homedash.CapColor, map[string]any{"h": 200.0, "s": 80.0}, "light", "turn_on", map[string]any{"hs_color": []any{200.0, 80.0}}},
		{"light.kitchen", homedash.CapEffect, map[string]any{"option": "colorloop"}, "light", "turn_on", map[string]any{"effect": "colorloop"}},
		{"scene.movie", homedash.CapActivate, nil, "scene", "turn_on", map[string]any{}},
		{"script.goodnight", homedash.CapActivate, nil, "script", "turn_on", map[string]any{}},
		{"button.doorbell_test", homedash.CapPress, nil, "button", "press", map[string]any{}},
		{"input_button.bell", homedash.CapPress, nil, "input_button", "press", map[string]any{}},
		{"select.heater_mode", homedash.CapSelectOption, map[string]any{"option": "boost"}, "select", "select_option", map[string]any{"option": "boost"}},
		{"input_select.radio", homedash.CapSelectOption, map[string]any{"option": "Rock"}, "input_select", "select_option", map[string]any{"option": "Rock"}},
		{"number.boiler_target", homedash.CapSetValue, map[string]any{"value": 60.0}, "number", "set_value", map[string]any{"value": 60.0}},
		{"input_number.volume", homedash.CapSetValue, map[string]any{"value": 40.0}, "input_number", "set_value", map[string]any{"value": 40.0}},
		{"climate.hall", homedash.CapSetTemperature, map[string]any{"value": 21.5}, "climate", "set_temperature", map[string]any{"temperature": 21.5}},
		{"cover.garage", homedash.CapSetPosition, map[string]any{"value": 30.0}, "cover", "set_cover_position", map[string]any{"position": 30.0}},
		{"media_player.tv", homedash.CapVolumeSet, map[string]any{"value": 25.0}, "media_player", "volume_set", map[string]any{"volume_level": 0.25}},
	} {
		call, ok := mapCommand(tc.native, tc.c, tc.args)
		if !ok || call.Domain != tc.domain || call.Service != tc.service || !reflect.DeepEqual(call.Data, tc.data) {
			t.Errorf("%s %s = %+v %v, want %s.%s %v", tc.native, tc.c, call, ok, tc.domain, tc.service, tc.data)
		}
	}
	for _, tc := range []struct {
		native string
		c      homedash.Capability
		args   map[string]any
	}{
		{"light.kitchen", homedash.CapPress, nil},
		{"sensor.outdoor_temperature", homedash.CapToggle, nil},
		{"Light.kitchen", homedash.CapToggle, nil},
		{"alarm_control_panel.home", homedash.CapToggle, nil},
		{"light.kitchen", homedash.CapBrightness, map[string]any{"value": "40"}},
		{"select.heater_mode", homedash.CapSelectOption, map[string]any{}},
	} {
		if call, ok := mapCommand(tc.native, tc.c, tc.args); ok {
			t.Errorf("%s %s mapped to %+v", tc.native, tc.c, call)
		}
	}
}

func TestHAExpectedState(t *testing.T) {
	now := time.Date(2026, 11, 25, 10, 0, 1, 0, time.UTC)
	on, off := HAState{State: "on"}, HAState{State: "off"}
	type want struct {
		value any
		text  string
		attrs map[string]any
	}
	for _, tc := range []struct {
		name   string
		domain string
		c      homedash.Capability
		args   map[string]any
		cur    HAState
		have   bool
		want   *want
	}{
		{"toggle on", "switch", homedash.CapToggle, nil, on, true, &want{false, "off", nil}},
		{"toggle off", "light", homedash.CapToggle, nil, off, true, &want{true, "on", nil}},
		{"toggle unknown", "switch", homedash.CapToggle, nil, HAState{State: "unavailable"}, true, nil},
		{"toggle without state", "switch", homedash.CapToggle, nil, HAState{}, false, nil},
		{"turn_on", "input_boolean", homedash.CapTurnOn, nil, off, true, &want{true, "on", nil}},
		{"turn_off", "light", homedash.CapTurnOff, nil, on, true, &want{false, "off", nil}},
		{"brightness", "light", homedash.CapBrightness, map[string]any{"value": 40.0}, off, true, &want{true, "on", map[string]any{"brightness_pct": 40.0}}},
		{"brightness 0", "light", homedash.CapBrightness, map[string]any{"value": 0.0}, on, true, &want{false, "off", nil}},
		{"color_temp", "light", homedash.CapColorTemp, map[string]any{"kelvin": 3502.0}, on, true, &want{true, "on", map[string]any{"color_temp_kelvin": 3502.0}}},
		{"color", "light", homedash.CapColor, map[string]any{"h": 200.0, "s": 80.0}, on, true, &want{true, "on", map[string]any{"hs_color": []any{200.0, 80.0}}}},
		{"effect", "light", homedash.CapEffect, map[string]any{"option": "colorloop"}, on, true, &want{true, "on", map[string]any{"effect": "colorloop"}}},
		{"select", "select", homedash.CapSelectOption, map[string]any{"option": "boost"}, HAState{State: "eco"}, true, &want{"boost", "boost", nil}},
		{"set_value", "number", homedash.CapSetValue, map[string]any{"value": 60.0}, HAState{State: "55.0"}, true, &want{60.0, "60", nil}},
		{"activate", "scene", homedash.CapActivate, nil, HAState{}, true, nil},
		{"press", "button", homedash.CapPress, nil, HAState{}, true, nil},
		{"non-W1", "climate", homedash.CapSetTemperature, map[string]any{"value": 21.0}, HAState{State: "heat"}, true, nil},
	} {
		got := expectedState(tc.domain, tc.c, tc.args, tc.cur, tc.have, now)
		if tc.want == nil {
			if got != nil {
				t.Errorf("%s: expected %+v, want none", tc.name, got)
			}
			continue
		}
		if got == nil || got.Value != tc.want.value || got.Text != tc.want.text || !reflect.DeepEqual(got.Attributes, tc.want.attrs) ||
			!got.Available || !got.UpdatedAt.Equal(now) {
			t.Errorf("%s: expected %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestHACommandFeedsChangedStates(t *testing.T) {
	o := hasstest.DemoOptions()
	o.NoEvents = true // only the service answer can carry the new state
	ha := hasstest.New(t, o)
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"light.kitchen"})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusLive, "") }, "live")
	res, err := a.Command(context.Background(), homedash.Command{Entity: "ha:light.kitchen", Native: "light.kitchen", Name: homedash.CapBrightness, Args: map[string]any{"value": 60.0}})
	if err != nil || res.Expected == nil || res.Expected.Value != true || res.Expected.Attributes["brightness_pct"] != 60.0 {
		t.Fatalf("command = %+v %v", res, err)
	}
	calls := ha.Calls()
	if len(calls) != 1 || calls[0].Domain != "light" || calls[0].Service != "turn_on" || calls[0].Data["entity_id"] != "light.kitchen" || calls[0].Data["brightness_pct"] != 60.0 {
		t.Fatalf("calls = %+v", calls)
	}
	if st, ok := sink.last("light.kitchen"); !ok || st.Attributes["brightness_pct"] != 60.0 || st.Value != true {
		t.Fatalf("fed state = %+v %v", st, ok)
	}
}

func TestHACommandsHonourReadonlyAndServicePolicy(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	cmd := func(native string, c homedash.Capability) homedash.Command {
		return homedash.Command{Entity: homedash.MakeEntityID(SourceID, native), Native: native, Name: c, Args: map[string]any{}, Confirmed: true}
	}
	for _, tc := range []struct {
		cfg       tools.HAConfig
		native    string
		c         homedash.Capability
		blockedBy string
	}{
		{tools.HAConfig{ReadOnly: true}, "switch.coffee", homedash.CapToggle, "home_assistant.readonly"},
		{tools.HAConfig{BlockedServices: []string{"switch.toggle"}}, "switch.coffee", homedash.CapToggle, "home_assistant.blocked_services"},
		{tools.HAConfig{}, "script.goodnight", homedash.CapActivate, "home_assistant.allowed_services"},
		{tools.HAConfig{AllowedServices: []string{"light.turn_on"}}, "switch.coffee", homedash.CapToggle, "home_assistant.allowed_services"},
	} {
		a := newTestAdapter(t, ha, tc.cfg)
		_, err := a.Command(context.Background(), cmd(tc.native, tc.c))
		var he *homedash.Error
		if !errors.As(err, &he) || he.Code != homedash.CodeReadOnly || he.BlockedBy != tc.blockedBy {
			t.Errorf("%s %s with %+v = %v", tc.native, tc.c, tc.cfg, err)
		}
	}
	if n := len(ha.Calls()); n != 0 {
		t.Fatalf("%d calls reached Home Assistant", n)
	}
	a := newTestAdapter(t, ha, tools.HAConfig{AllowedServices: []string{"script.turn_on"}})
	if _, err := a.Command(context.Background(), cmd("script.goodnight", homedash.CapActivate)); err != nil {
		t.Fatalf("allowlisted script = %v", err)
	}
	if c := ha.Calls(); len(c) != 1 || c[0].Domain != "script" || c[0].Service != "turn_on" {
		t.Fatalf("calls = %+v", c)
	}
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) { l.mu.Lock(); defer l.mu.Unlock(); return l.b.Write(p) }
func (l *lockedBuffer) String() string              { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

func TestHACommandErrorsNeverCarryUpstreamText(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	logs := &lockedBuffer{}
	a := New(tools.HAConfig{URL: ha.URL, AccessToken: ha.Token}, 5, FactoryOptions{Logger: slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})), WS: fastWS})
	cmd := homedash.Command{Entity: "ha:switch.coffee", Native: "switch.coffee", Name: homedash.CapToggle, Args: map[string]any{}}
	for _, tc := range []struct {
		status int
		code   string
	}{{400, homedash.CodeCommandFailed}, {404, homedash.CodeCommandFailed}, {500, homedash.CodeCommandFailed},
		{502, homedash.CodeSourceUnavailable}, {503, homedash.CodeSourceUnavailable}, {504, homedash.CodeSourceUnavailable}} {
		ha.FailServices(tc.status, hasstest.UpstreamSecret+" "+ha.Token)
		_, err := a.Command(context.Background(), cmd)
		var he *homedash.Error
		if !errors.As(err, &he) || he.Code != tc.code || strings.Contains(err.Error(), hasstest.UpstreamSecret) {
			t.Fatalf("HTTP %d = %v", tc.status, err)
		}
		if tc.code == homedash.CodeSourceUnavailable && he.Source != string(SourceID) {
			t.Fatalf("HTTP %d: source = %q", tc.status, he.Source)
		}
	}
	ha.FailServices(0, "")
	if out := logs.String(); !strings.Contains(out, hasstest.UpstreamSecret) || strings.Contains(out, ha.Token) {
		t.Fatalf("log must keep the scrubbed excerpt and never the token:\n%s", out)
	}
	bad := New(tools.HAConfig{URL: ha.URL, AccessToken: "wrong-token"}, 5, FactoryOptions{Logger: discardLog, WS: fastWS})
	var he *homedash.Error
	if _, err := bad.Command(context.Background(), cmd); !errors.As(err, &he) || he.Code != homedash.CodeSourceUnavailable {
		t.Fatalf("wrong token = %v", err)
	}
	ha.Close()
	if _, err := a.Command(context.Background(), cmd); !errors.As(err, &he) || he.Code != homedash.CodeSourceUnavailable || he.Source != "ha" {
		t.Fatalf("Home Assistant gone = %v", err)
	}
	if _, err := a.Command(context.Background(), homedash.Command{Entity: "ha:light.kitchen", Native: "light.kitchen", Name: homedash.CapPress}); !errors.As(err, &he) || he.Code != homedash.CodeUnsupportedCommand {
		t.Fatalf("unmapped = %v", err)
	}
}
```

- [ ] Run `go test ./internal/homedash/sources/hass/ -count=1 -run '(?i)(TestHAVerbMapping|TestHAExpectedState|TestHACommand)'`. Expected: build failure (`mapCommand` undefined).
- [ ] Delete the temporary `Command` stub from `rest_fallback.go` and create `internal/homedash/sources/hass/commands.go`:

```go
package hass

import (
	"context"
	"errors"
	"strconv"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/tools"
)

// serviceCall is the Home Assistant service a HomeDash command maps to.
type serviceCall struct {
	Domain, Service string
	Data            map[string]any // service data without entity_id
}

func argNum(args map[string]any, key string) (float64, bool) {
	switch v := args[key].(type) {
	case float64:
		return v, finite(v)
	case int:
		return float64(v), true
	}
	return 0, false
}

func argStr(args map[string]any, key string) (string, bool) {
	s, ok := args[key].(string)
	return s, ok && s != ""
}

// mapCommand maps a validated command to its service (decision D3, design-backend section 6). The
// service comes from the domain table; ok is false when the domain does not offer the capability or
// an argument has the wrong type.
func mapCommand(native string, c homedash.Capability, args map[string]any) (serviceCall, bool) {
	domain, ok := domainOf(native)
	if !ok {
		return serviceCall{}, false
	}
	spec, ok := MapDomain(domain)
	if !ok {
		return serviceCall{}, false
	}
	service := ""
	for _, cs := range spec.Commands {
		if cs.Cap == c {
			service = cs.Service
			break
		}
	}
	if service == "" {
		return serviceCall{}, false
	}
	call := serviceCall{Domain: domain, Service: service, Data: map[string]any{}}
	num := func(key, as string, scale float64) bool {
		v, ok := argNum(args, key)
		if ok {
			call.Data[as] = v * scale
		}
		return ok
	}
	str := func(key, as string) bool {
		v, ok := argStr(args, key)
		if ok {
			call.Data[as] = v
		}
		return ok
	}
	boolean := func(key, as string) bool {
		v, ok := args[key].(bool)
		if ok {
			call.Data[as] = v
		}
		return ok
	}
	switch c {
	case homedash.CapToggle, homedash.CapTurnOn, homedash.CapTurnOff, homedash.CapActivate, homedash.CapPress,
		homedash.CapOpen, homedash.CapClose, homedash.CapStop, homedash.CapLock, homedash.CapUnlock,
		homedash.CapPlayPause, homedash.CapNext, homedash.CapPrevious:
		ok = true
	case homedash.CapBrightness:
		ok = num("value", "brightness_pct", 1)
	case homedash.CapColorTemp:
		ok = num("kelvin", "color_temp_kelvin", 1)
	case homedash.CapColor:
		h, okh := argNum(args, "h")
		s, oks := argNum(args, "s")
		if ok = okh && oks; ok {
			call.Data["hs_color"] = []any{h, s}
		}
	case homedash.CapEffect:
		ok = str("option", "effect")
	case homedash.CapSelectOption:
		ok = str("option", "option")
	case homedash.CapSetValue:
		ok = num("value", "value", 1)
	case homedash.CapSetTemperature:
		ok = num("value", "temperature", 1)
	case homedash.CapSetTemperatureRange:
		ok = num("low", "target_temp_low", 1) && num("high", "target_temp_high", 1)
	case homedash.CapHVACMode:
		ok = str("option", "hvac_mode")
	case homedash.CapPresetMode:
		ok = str("option", "preset_mode")
	case homedash.CapFanMode:
		ok = str("option", "fan_mode")
	case homedash.CapSetPosition:
		ok = num("value", "position", 1)
	case homedash.CapSetTilt:
		ok = num("value", "tilt_position", 1)
	case homedash.CapSetPercentage:
		ok = num("value", "percentage", 1)
	case homedash.CapOscillate:
		ok = boolean("on", "oscillating")
	case homedash.CapVolumeSet:
		ok = num("value", "volume_level", 0.01)
	case homedash.CapVolumeMute:
		ok = boolean("on", "is_volume_muted")
	case homedash.CapSourceSelect:
		ok = str("option", "source")
	default:
		ok = false
	}
	return call, ok
}

// expectedState predicts the state a W1 command leads to (decision D3). It returns nil for momentary
// commands (activate, press), for other capabilities and for a toggle whose current state is not on
// or off. Attributes carry only what the command sets (lists as []any for BoundAttributes).
func expectedState(domain string, c homedash.Capability, args map[string]any, cur HAState, have bool, now time.Time) *homedash.State {
	on := func(attrs map[string]any) *homedash.State {
		return &homedash.State{Value: true, Text: "on", Attributes: attrs, Available: true, UpdatedAt: now}
	}
	off := func() *homedash.State { return &homedash.State{Value: false, Text: "off", Available: true, UpdatedAt: now} }
	switch c {
	case homedash.CapToggle:
		if !have {
			return nil
		}
		switch cur.State {
		case "on":
			return off()
		case "off":
			return on(nil)
		}
		return nil
	case homedash.CapTurnOn:
		return on(nil)
	case homedash.CapTurnOff:
		return off()
	case homedash.CapBrightness:
		v, _ := argNum(args, "value")
		if v <= 0 {
			return off()
		}
		return on(map[string]any{"brightness_pct": v})
	case homedash.CapColorTemp:
		v, _ := argNum(args, "kelvin")
		return on(map[string]any{"color_temp_kelvin": v})
	case homedash.CapColor:
		h, _ := argNum(args, "h")
		s, _ := argNum(args, "s")
		return on(map[string]any{"hs_color": []any{h, s}})
	case homedash.CapEffect:
		o, _ := argStr(args, "option")
		return on(map[string]any{"effect": o})
	case homedash.CapSelectOption:
		o, _ := argStr(args, "option")
		return &homedash.State{Value: o, Text: o, Available: true, UpdatedAt: now}
	case homedash.CapSetValue:
		v, _ := argNum(args, "value")
		return &homedash.State{Value: v, Text: strconv.FormatFloat(v, 'f', -1, 64), Available: true, UpdatedAt: now}
	}
	return nil
}

func errSourceUnavailable() *homedash.Error {
	return homedash.SourceUnavailable(SourceID, "Home Assistant is not reachable")
}

func errCommandFailed() *homedash.Error {
	return &homedash.Error{Code: homedash.CodeCommandFailed, Message: "Home Assistant did not accept the command"}
}

// Command maps the command to its service, predicts the expected state from the state before the
// call, calls the service through tools.HACallServiceStates (read-only mode, service policy and
// target refusal are re-checked there) and feeds the changed states of the answer into the sink at
// once. Clients get fixed codes only; Home Assistant's text goes to the log, scrubbed.
func (a *Adapter) Command(ctx context.Context, cmd homedash.Command) (homedash.CommandResult, error) {
	native := cmd.Native
	if native == "" {
		if src, n, err := homedash.ParseEntityID(string(cmd.Entity)); err == nil && src == SourceID {
			native = n
		}
	}
	call, ok := mapCommand(native, cmd.Name, cmd.Args)
	if !ok {
		return homedash.CommandResult{}, &homedash.Error{Code: homedash.CodeUnsupportedCommand, Message: "Home Assistant cannot run this command for this entity"}
	}
	a.mu.Lock()
	cur, have := a.states[native]
	a.mu.Unlock()
	expected := expectedState(call.Domain, cmd.Name, cmd.Args, cur, have, a.o.Now().UTC())
	changed, err := tools.HACallServiceStates(ctx, a.ha, call.Domain, call.Service, native, call.Data)
	if err != nil {
		var se *tools.HAServiceError
		if !errors.As(err, &se) || se.Kind != tools.HAServiceBadAnswer {
			return homedash.CommandResult{}, a.commandError(ctx, call, err)
		}
		a.log.Debug("homedash: Home Assistant answered a service call with an unexpected body",
			"service", call.Domain+"."+call.Service, "status", se.Status)
	}
	a.mu.Lock()
	for _, st := range changed {
		a.applyStateLocked(HAState{EntityID: st.EntityID, State: st.State, Attributes: st.Attributes, LastChanged: st.LastChanged, LastUpdated: st.LastUpdated})
	}
	a.mu.Unlock()
	return homedash.CommandResult{Expected: expected}, nil
}

// commandError maps a failed call to a fixed HomeDash error. The caller's own cancellation is
// returned as the context error so the hub reports it as such (504/abandoned dedupe entry).
func (a *Adapter) commandError(ctx context.Context, call serviceCall, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var se *tools.HAServiceError
	if !errors.As(err, &se) {
		a.log.Warn("homedash: Home Assistant service call failed", "service", call.Domain+"."+call.Service, "error", err)
		return errCommandFailed()
	}
	a.log.Warn("homedash: Home Assistant service call failed", "service", call.Domain+"."+call.Service,
		"kind", string(se.Kind), "status", se.Status, "detail", se.LogDetail)
	switch se.Kind {
	case tools.HAServicePolicy:
		return &homedash.Error{Code: homedash.CodeReadOnly, Message: "commands are switched off", BlockedBy: blockedBy(a.ha, call.Domain+"."+call.Service)}
	case tools.HAServiceUnreachable, tools.HAServiceAuth:
		return errSourceUnavailable()
	case tools.HAServiceUpstream:
		if se.Status == 502 || se.Status == 503 || se.Status == 504 {
			return errSourceUnavailable()
		}
	}
	return errCommandFailed()
}
```

- [ ] Run `go test ./internal/homedash/sources/hass/... -count=1 -v`. Expected: PASS.
- [ ] Stage `commands.go`, `commands_test.go` and `rest_fallback.go`. Lead: detect_changes, then commit with `feat(homedash): Home Assistant commands with verb mapping, expected state and changed-state feed`.

**Done when:** the verb and expected tables match contract §4, and no error carries `hasstest.UpstreamSecret` or the token.

---

### Task 8: History, camera snapshot, opt-in live test

**Files:** create `internal/homedash/sources/hass/history.go`, `internal/homedash/sources/hass/camera.go`, `internal/homedash/sources/hass/history_camera_test.go`, `internal/homedash/sources/hass/live_test.go`; modify `internal/homedash/sources/hass/rest_fallback.go` (remove the remaining stubs).

**Read first:**
- `internal/homedash/history.go:130-257` (`downsample`, `MaxHistoryRange`, `MaxSnapshotBytes`; the hub calls the source's `History` for every entity of a `HistorySource` and downsamples; it calls `Snapshot` only for entities with `CapSnapshot`);
- `internal/tools/homeassistant_raw.go`;
- `internal/server/homedash_api_entities.go:98-129` (the snapshot route accepts exactly `image/jpeg|png|webp`);
- contract §7.

**Impact:** none.

- [ ] Write `internal/homedash/sources/hass/history_camera_test.go`:

```go
package hass

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/homedash/sources/hass/hasstest"
	"aurago/internal/tools"
)

func TestHAHistoryBoundsRangeAndPoints(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	a := newTestAdapter(t, ha, tools.HAConfig{})
	ctx, at := context.Background(), hasstest.DemoTime
	pts, err := a.History(ctx, "sensor.outdoor_temperature", at.Add(-3*time.Hour), at, 100)
	if err != nil || len(pts) != 2 || pts[0].V != 4.0 || pts[1].V != 4.5 || pts[0].T.Location() != time.UTC {
		t.Fatalf("sensor history = %+v %v", pts, err)
	}
	q := ha.LastHistoryQuery()
	for _, part := range []string{"filter_entity_id=sensor.outdoor_temperature", "end_time=2026-11-25T09%3A00%3A00Z", "minimal_response", "no_attributes"} {
		if !strings.Contains(q, part) {
			t.Fatalf("query %q lacks %q", q, part)
		}
	}
	if pts, err = a.History(ctx, "switch.coffee", at.Add(-3*time.Hour), at, 100); err != nil || len(pts) != 2 || pts[0].V != 1 || pts[1].V != 0 {
		t.Fatalf("switch history = %+v %v", pts, err)
	}
	before := ha.HistoryRequests()
	if pts, err = a.History(ctx, "light.kitchen", at.Add(-time.Hour), at, 10); err != nil || len(pts) != 0 || ha.HistoryRequests() != before {
		t.Fatalf("light history = %+v %v (no request expected)", pts, err)
	}
	if pts, _ = a.History(ctx, "sensor.outdoor_temperature", at.Add(-8*24*time.Hour), at, 10); len(pts) != 0 || ha.HistoryRequests() != before {
		t.Fatal("a range over 7 days must not reach Home Assistant")
	}
	if _, err = a.History(ctx, "bad id", at.Add(-time.Hour), at, 10); err == nil {
		t.Fatal("invalid id accepted")
	}
}

func TestHACameraSnapshotSizeCap(t *testing.T) {
	o := hasstest.DemoOptions()
	o.Cameras["camera.big"] = append(slices.Clone(o.Cameras["camera.door"]), make([]byte, 8<<20)...)
	ha := hasstest.New(t, o)
	a := newTestAdapter(t, ha, tools.HAConfig{})
	ctx := context.Background()
	img, ct, err := a.Snapshot(ctx, "camera.door", 640, 360)
	if err != nil || ct != "image/jpeg" || len(img) == 0 {
		t.Fatalf("snapshot = %d bytes %q %v", len(img), ct, err)
	}
	if _, _, err := a.Snapshot(ctx, "camera.big", 640, 360); err == nil {
		t.Fatal("over 8 MiB accepted")
	}
	if _, _, err := a.Snapshot(ctx, "light.kitchen", 640, 360); err == nil {
		t.Fatal("non-camera accepted")
	}
	o2 := hasstest.DemoOptions()
	o2.CameraContentType = "text/html"
	ha2 := hasstest.New(t, o2)
	if _, _, err := newTestAdapter(t, ha2, tools.HAConfig{}).Snapshot(ctx, "camera.door", 640, 360); err == nil {
		t.Fatal("non-image content type accepted")
	}
}
```

- [ ] Write `internal/homedash/sources/hass/live_test.go`:

```go
package hass

import (
	"context"
	"os"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/tools"
)

// TestHALiveReadOnly runs against a real Home Assistant only when HOMEDASH_HA_LIVE_URL and
// HOMEDASH_HA_LIVE_TOKEN are set (decision D14). It reads the catalog and never calls a service
// unless HOMEDASH_HA_LIVE_TOGGLE names one switch or light, which it toggles twice (back to its state).
func TestHALiveReadOnly(t *testing.T) {
	url, token := os.Getenv("HOMEDASH_HA_LIVE_URL"), os.Getenv("HOMEDASH_HA_LIVE_TOKEN")
	if url == "" || token == "" {
		t.Skip("set HOMEDASH_HA_LIVE_URL and HOMEDASH_HA_LIVE_TOKEN to run against a real Home Assistant")
	}
	a := New(tools.HAConfig{URL: url, AccessToken: token}, 5, FactoryOptions{})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	list, err := a.Catalog(ctx)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	withArea := 0
	for _, e := range list {
		if e.AreaID != "" {
			withArea++
		}
	}
	t.Logf("%d entities, %d with an area", len(list), withArea)
	if native := os.Getenv("HOMEDASH_HA_LIVE_TOGGLE"); native != "" {
		for i := 0; i < 2; i++ {
			res, err := a.Command(ctx, homedash.Command{Entity: homedash.MakeEntityID(SourceID, native), Native: native, Name: homedash.CapToggle, Args: map[string]any{}})
			if err != nil {
				t.Fatalf("toggle %d: %v", i+1, err)
			}
			t.Logf("toggle %d expected %+v", i+1, res.Expected)
			time.Sleep(2 * time.Second)
		}
	}
}
```

- [ ] Run `go test ./internal/homedash/sources/hass/ -count=1 -run '(?i)(TestHAHistory|TestHACamera)'`. Expected: FAIL (the stubs return nothing).
- [ ] Delete the `History` and `Snapshot` stubs from `rest_fallback.go`. Create `internal/homedash/sources/hass/history.go`:

```go
package hass

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/tools"
)

// historyDomains have a numeric or on/off recorder history.
var historyDomains = map[string]bool{"sensor": true, "number": true, "input_number": true, "switch": true, "input_boolean": true, "binary_sensor": true}

const maxHistorySamples = 20000

func historyValue(s string) (float64, bool) {
	switch s {
	case "on":
		return 1, true
	case "off":
		return 0, true
	}
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil && finite(f)
}

// History reads Home Assistant's recorder for one entity (design-backend 8.1): numeric states as
// numbers, on/off as 1/0, other states skipped. Domains without such a history answer an empty
// series without a request. The hub downsamples to points.
func (a *Adapter) History(ctx context.Context, native string, from, to time.Time, points int) ([]homedash.Sample, error) {
	out := []homedash.Sample{}
	domain, ok := domainOf(native)
	if !ok {
		return nil, errors.New("hass: invalid entity id")
	}
	if !historyDomains[domain] || !to.After(from) || to.Sub(from) > homedash.MaxHistoryRange || points < 1 {
		return out, nil
	}
	endpoint := "/api/history/period/" + from.UTC().Format(time.RFC3339) +
		"?filter_entity_id=" + url.QueryEscape(native) +
		"&end_time=" + url.QueryEscape(to.UTC().Format(time.RFC3339)) +
		"&minimal_response&no_attributes"
	var series [][]struct {
		State       string    `json:"state"`
		LastChanged time.Time `json:"last_changed"`
		LastUpdated time.Time `json:"last_updated"`
	}
	if err := tools.HARequestJSON(ctx, a.ha, http.MethodGet, endpoint, nil, &series); err != nil {
		return nil, err
	}
	for _, list := range series {
		for _, p := range list {
			v, ok := historyValue(p.State)
			at := p.LastChanged
			if at.IsZero() {
				at = p.LastUpdated
			}
			if !ok || at.IsZero() {
				continue
			}
			out = append(out, homedash.Sample{T: at.UTC(), V: v})
			if len(out) >= maxHistorySamples {
				break
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].T.Before(out[j].T) })
	return out, nil
}
```

- [ ] Create `internal/homedash/sources/hass/camera.go`:

```go
package hass

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"

	"aurago/internal/homedash"
	"aurago/internal/tools"
)

// Snapshot reads one camera image through Home Assistant's camera proxy (at most 8 MiB, never
// stored). Only JPEG, PNG and WebP pass, and the bytes must match the declared type. maxW and maxH
// are accepted and ignored in M3 (no scaling; the camera widget is M5).
func (a *Adapter) Snapshot(ctx context.Context, native string, _, _ int) ([]byte, string, error) {
	domain, ok := domainOf(native)
	if !ok || domain != "camera" {
		return nil, "", errors.New("hass: not a camera entity")
	}
	img, ct, err := tools.HAGetBytes(ctx, a.ha, "/api/camera_proxy/"+native, homedash.MaxSnapshotBytes)
	if err != nil {
		return nil, "", err
	}
	mt, _, _ := mime.ParseMediaType(ct)
	switch sniffed := http.DetectContentType(img); {
	case mt == "image/jpeg" && sniffed == "image/jpeg", mt == "image/png" && sniffed == "image/png", mt == "image/webp" && sniffed == "image/webp":
		return img, mt, nil
	}
	return nil, "", fmt.Errorf("hass: camera answered %q", mt)
}
```

- [ ] Run `go test ./internal/homedash/sources/hass/... -count=1 -v`. Expected: PASS; `TestHALiveReadOnly` SKIP.
- [ ] Stage the five paths. Lead: detect_changes, then commit with `feat(homedash): Home Assistant history, camera snapshot and opt-in live test`.

**Done when:** the hass package passes with no stub left (grep `not yet` in `internal/homedash/sources/hass` finds nothing).

---

### Task 9: Server row, Settings and audit network entry

**Files:** modify `internal/server/homedash_sources.go`, `internal/server/homedash_sources_test.go`, `internal/audit/audit_manifests.go`, `internal/audit/homedash_manifest_test.go`.

**Read first:**
- `internal/server/homedash_sources.go`;
- `internal/server/homedash_service.go:72-95` (`homeDashRuntimeConfig` sets `ConfigKey`);
- `internal/config/config_types.go:1675-1682` (`HomeAssistant`; the token is vault-only);
- `internal/config/homedash.go:47-68,100`;
- `internal/audit/audit_manifests.go:227-296`;
- `internal/audit/audit_manifests_test.go:164-197,2032-2039`.

**Impact (lead, MCP):** `Server.homeDashSourceSpecs` (callers: `initHomeDash`, tests), `NetworkClientInventory`, test `TestHomeDashSourceSpecsListSystem` (renamed).

- [ ] Replace `TestHomeDashSourceSpecsListSystem` in `internal/server/homedash_sources_test.go` and add a Settings test:

```go
func TestHomeDashSourceSpecsListSystemAndHomeAssistant(t *testing.T) {
	specs := (&Server{}).homeDashSourceSpecs()
	if len(specs) != 2 || specs[0].ID != system.SourceID || specs[0].ConfigKey != "system" ||
		specs[1].ID != hass.SourceID || specs[1].ConfigKey != "home_assistant" || specs[1].Factory == nil || specs[1].Settings == nil {
		t.Fatalf("specs = %+v", specs)
	}
	cfg := &config.Config{}
	cfg.HomeDash.Sources.System = config.HomeDashSourceConfig{Enabled: true, Control: true, PollSeconds: 7}
	if set := specs[0].Settings(cfg); !set.Enabled || set.Control || set.PollSeconds != 7 {
		t.Fatalf("settings = %+v (control must stay off: the source is read-only)", set)
	}
}

func TestHomeDashHASettings(t *testing.T) {
	base := func() *config.Config {
		c := &config.Config{}
		c.HomeAssistant.Enabled, c.HomeAssistant.URL, c.HomeAssistant.AccessToken = true, " http://ha.lan:8123 ", "tok"
		c.HomeAssistant.ReadOnly = true
		c.HomeAssistant.AllowedServices, c.HomeAssistant.BlockedServices = []string{"light.turn_on"}, []string{"lock.unlock"}
		c.HomeDash.Sources.HomeAssistant = config.HomeDashSourceConfig{Enabled: true, Control: true, PollSeconds: 5}
		return c
	}
	cfg := base()
	set := homeDashHASettings(cfg)
	if !set.Enabled || !set.Control || set.PollSeconds != 5 || set.ConfigKey != "home_assistant" ||
		set.Params[hass.ParamURL] != "http://ha.lan:8123" || set.Params[hass.ParamToken] != "tok" || set.Params[hass.ParamReadOnly] != "true" ||
		set.Lists[hass.ListAllowedServices][0] != "light.turn_on" || set.Lists[hass.ListBlockedServices][0] != "lock.unlock" {
		t.Fatalf("settings = %+v", set)
	}
	cfg.HomeAssistant.AllowedServices[0] = "changed"
	if set.Lists[hass.ListAllowedServices][0] != "light.turn_on" {
		t.Fatal("service lists must be copies")
	}
	for name, mut := range map[string]func(*config.Config){
		"integration off": func(c *config.Config) { c.HomeAssistant.Enabled = false },
		"source off":      func(c *config.Config) { c.HomeDash.Sources.HomeAssistant.Enabled = false },
		"no url":          func(c *config.Config) { c.HomeAssistant.URL = "  " },
		"no token":        func(c *config.Config) { c.HomeAssistant.AccessToken = "" },
	} {
		c := base()
		mut(c)
		if homeDashHASettings(c).Enabled {
			t.Errorf("%s: enabled", name)
		}
	}
	c := base()
	c.HomeDash.Sources.HomeAssistant.Control = false
	if homeDashHASettings(c).Control {
		t.Fatal("control switch ignored")
	}
	src, ok := hass.Factory(hass.FactoryOptions{})(homedash.RuntimeConfig{Sources: map[homedash.SourceID]homedash.SourceSettings{hass.SourceID: set}})
	if !ok || src.ID() != hass.SourceID {
		t.Fatalf("factory with the settings = %v %v", src, ok)
	}
}
```

  Add the imports `aurago/internal/homedash` and `aurago/internal/homedash/sources/hass`.
- [ ] Append to `internal/audit/homedash_manifest_test.go`:

```go
func TestHomeDashHANetworkClientIsClassified(t *testing.T) {
	t.Parallel()
	for _, e := range NetworkClientInventory() {
		if e.Path != "internal/homedash/sources/hass/" {
			continue
		}
		if !e.AllowsLocalNet || !e.Credentialed || e.RequiresSSRF || e.Classification != "configured-home-assistant-websocket-same-origin" {
			t.Fatalf("entry = %+v", e)
		}
		return
	}
	t.Fatal("internal/homedash/sources/hass/ must be in NetworkClientInventory")
}
```

- [ ] Run `go test ./internal/server -count=1 -run '(?i)(TestHomeDashSourceSpecs|TestHomeDashHASettings)'` and `go test ./internal/audit -count=1 -run TestHomeDashHANetworkClient`. Expected: FAIL / build failure.
- [ ] `internal/server/homedash_sources.go`: add the imports `"slices"`, `"strconv"`, `"strings"`, `"aurago/internal/homedash/sources/hass"`; extend the list and add the row helpers:

```go
// homeDashSourceSpecs lists the sources of this build. M3: system and Home Assistant.
func (s *Server) homeDashSourceSpecs() []homeDashSourceSpec {
	return []homeDashSourceSpec{
		{
			ID:        system.SourceID,
			ConfigKey: "system",
			Settings:  homeDashSystemSettings,
			Factory:   system.Factory(s.homeDashSystemSnapshot, s.homeDashSystemHistory),
		},
		homeDashHASpec(hass.FactoryOptions{Logger: s.Logger}),
	}
}

// homeDashHASpec is the Home Assistant row; tests pass shorter websocket timings.
func homeDashHASpec(o hass.FactoryOptions) homeDashSourceSpec {
	return homeDashSourceSpec{ID: hass.SourceID, ConfigKey: "home_assistant", Settings: homeDashHASettings, Factory: hass.Factory(o)}
}

// homeDashHASettings: the source runs only when homedash.sources.home_assistant.enabled, the Home
// Assistant integration (home_assistant.enabled), a URL and a token are all present (decision D1;
// homedash.enabled is the hub's own switch). Control is homedash.sources.home_assistant.control. The
// integration's read-only switch and service lists travel to the adapter, which applies the shared
// service policy on every call. No lock is taken (runs under CfgMu).
func homeDashHASettings(cfg *config.Config) homedash.SourceSettings {
	src, ha := cfg.HomeDash.Sources.HomeAssistant, cfg.HomeAssistant
	url := strings.TrimSpace(ha.URL)
	return homedash.SourceSettings{
		Enabled:     src.Enabled && ha.Enabled && url != "" && ha.AccessToken != "",
		Control:     src.Control,
		PollSeconds: src.PollSeconds,
		ConfigKey:   "home_assistant",
		Params:      map[string]string{hass.ParamURL: url, hass.ParamToken: ha.AccessToken, hass.ParamReadOnly: strconv.FormatBool(ha.ReadOnly)},
		Lists:       map[string][]string{hass.ListAllowedServices: slices.Clone(ha.AllowedServices), hass.ListBlockedServices: slices.Clone(ha.BlockedServices)},
	}
}
```

- [ ] `internal/audit/audit_manifests.go` `NetworkClientInventory`: add after the `internal/httporigin/` row
  `{Path: "internal/homedash/sources/hass/", Classification: "configured-home-assistant-websocket-same-origin", AllowsLocalNet: true, Credentialed: true},`
- [ ] Run `go test ./internal/server -count=1 -run '(?i)(HomeDash)'` and `go test ./internal/audit/... -count=1`. Expected: PASS. The M2 HomeDash tests still pass: their envs register only their own specs.
- [ ] Stage the four paths. Lead: detect_changes, then commit with `feat(server): Home Assistant HomeDash source row, settings and network inventory`.

**Done when:** both commands pass.

---

### Task 10: `home_assistant` template, bootstrap advertisement, starter icons

**Files:** create `internal/server/homedash_template_ha.go`, `internal/server/homedash_template_ha_test.go`; modify `internal/server/homedash_routes.go`, `internal/server/homedash_api_dashboards.go`, `internal/server/homedash_template.go`, `internal/server/homedash_service.go`, `internal/server/homedash_api_dashboards_test.go`.

**Read first:**
- `internal/server/homedash_api_dashboards.go:66-90,168-204`;
- `internal/server/homedash_routes.go:20-42,156-187,308-323`;
- `internal/server/homedash_service.go:13-55`;
- `internal/homedash/dashboard_layout.go:89-129` (Autopack default sizes);
- `internal/homedash/catalog_widgets.go:90-145` (variant default sizes);
- Task 2 (`SourceCatalog`, `Widget.Icon`);
- contract §5, §6.

**Impact (lead, MCP):** `homeDashStarterSystem`, `hdRequest.createDashboard`, `homeDashCreateRequest`, `hdRequest.bootstrap`, `homeDashStatus`, `homeDashService` (struct), `newHomeDashService`.

- [ ] Write `internal/server/homedash_template_ha_test.go`:

```go
package server

import (
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass"
	"aurago/internal/homedash/sourcetest"
)

func haEntity(native, area, areaID string, kind homedash.Kind, caps ...homedash.CapabilitySpec) homedash.Entity {
	e := sourcetest.Entity(homedash.MakeEntityID(hass.SourceID, native), kind, caps...)
	e.Name, e.Area, e.AreaID = native, area, areaID
	return e
}

func allowedCap(c homedash.Capability) homedash.CapabilitySpec { return homedash.CapabilitySpec{Name: c, Allowed: true} }

func pageSummary(d homedash.Dashboard) []string {
	var out []string
	for _, p := range d.Pages {
		for _, w := range p.Widgets {
			out = append(out, fmt.Sprintf("%s|%s|%s/%s=%s", p.ID, p.Name, w.Type, w.Variant, w.Bindings["primary"]))
		}
	}
	return out
}

func TestHomeDashHATemplateBuilder(t *testing.T) {
	sel := allowedCap(homedash.CapSelectOption)
	sel.Options = []string{"a", "b", "c"}
	big := allowedCap(homedash.CapSelectOption)
	big.Options = []string{"1", "2", "3", "4", "5"}
	box := haEntity("number.boiler", "", "", homedash.KindNumber, allowedCap(homedash.CapSetValue))
	box.State.Attributes = map[string]any{"mode": "box"}
	blocked := haEntity("script.night", "", "", homedash.KindScript, homedash.CapabilitySpec{Name: homedash.CapActivate, BlockedBy: "home_assistant.allowed_services"})
	list := []homedash.Entity{
		haEntity("switch.b", "Kitchen", "kitchen", homedash.KindSwitch, allowedCap(homedash.CapToggle)),
		haEntity("light.a", "Kitchen", "kitchen", homedash.KindLight, allowedCap(homedash.CapToggle), allowedCap(homedash.CapBrightness)),
		haEntity("light.c", "Bath", "bath", homedash.KindLight, allowedCap(homedash.CapToggle)),
		haEntity("select.s", "Bath", "bath", homedash.KindSelect, sel),
		haEntity("input_select.big", "", "", homedash.KindSelect, big),
		box, blocked,
		haEntity("sensor.t", "Kitchen", "kitchen", homedash.KindSensor),
		sourcetest.Entity("fake:light.x", homedash.KindLight, allowedCap(homedash.CapToggle)),
	}
	doc, omitted, ok := homeDashTemplateHA(list, "Home", "Other", 12, 400)
	want := []string{
		"bath|Bath|light/tile=ha:light.c", "bath|Bath|select/segmented=ha:select.s",
		"kitchen|Kitchen|light/slider=ha:light.a", "kitchen|Kitchen|switch/tile=ha:switch.b",
		"unassigned|Other|select/list=ha:input_select.big", "unassigned|Other|number/stepper=ha:number.boiler",
	}
	if !ok || omitted != 0 || !slices.Equal(pageSummary(doc), want) {
		t.Fatalf("template = %v omitted=%d ok=%v", pageSummary(doc), omitted, ok)
	}
	cat := homedash.Catalog()
	doc.Slug = "home"
	homedash.Normalize(&doc)
	homedash.Autopack(&doc, cat)
	if rep := homedash.Validate(doc, cat, nil); !rep.OK() {
		t.Fatalf("issues = %+v", rep.Issues)
	}

	var many []homedash.Entity
	for i := 0; i < 30; i++ {
		many = append(many, haEntity(fmt.Sprintf("light.l%02d", i), "Hall", "hall", homedash.KindLight, allowedCap(homedash.CapToggle)))
	}
	if doc, omitted, _ = homeDashTemplateHA(many, "H", "O", 12, 400); len(doc.Pages) != 1 || len(doc.Pages[0].Widgets) != 24 || omitted != 6 {
		t.Fatalf("per-page cap: %d widgets, omitted %d", len(doc.Pages[0].Widgets), omitted)
	}
	var areas []homedash.Entity
	for i := 0; i < 14; i++ {
		areas = append(areas, haEntity(fmt.Sprintf("light.a%02d", i), fmt.Sprintf("Area %02d", i), fmt.Sprintf("a%02d", i), homedash.KindLight, allowedCap(homedash.CapToggle)))
	}
	areas = append(areas, haEntity("light.free", "", "", homedash.KindLight, allowedCap(homedash.CapToggle)))
	if doc, omitted, _ = homeDashTemplateHA(areas, "H", "O", 12, 400); len(doc.Pages) != 12 || omitted != 3 {
		t.Fatalf("page cap: %d pages, omitted %d", len(doc.Pages), omitted)
	}
	if doc, omitted, _ = homeDashTemplateHA(many, "H", "O", 12, 5); len(doc.Pages[0].Widgets) != 5 || omitted != 25 {
		t.Fatalf("total cap: %d widgets, omitted %d", len(doc.Pages[0].Widgets), omitted)
	}
	if _, _, ok := homeDashTemplateHA([]homedash.Entity{blocked}, "H", "O", 12, 400); ok {
		t.Fatal("nothing qualifies, but ok")
	}
}

func haTemplateFake() *sourcetest.FakeSource {
	return sourcetest.New(hass.SourceID,
		haEntity("light.kitchen", "Kitchen", "kitchen", homedash.KindLight, allowedCap(homedash.CapToggle), allowedCap(homedash.CapBrightness)),
		haEntity("switch.coffee", "Kitchen", "kitchen", homedash.KindSwitch, allowedCap(homedash.CapToggle)))
}

type hdHATemplateAnswer struct {
	Dashboard homedash.Dashboard `json:"dashboard"`
	Warnings  []homedash.Issue   `json:"warnings"`
	Omitted   *int               `json:"omitted"`
	Code      string             `json:"code"`
	Source    string             `json:"source"`
}

func TestHomeDashHATemplateRoute(t *testing.T) {
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(haTemplateFake()))
	body := map[string]any{"template": "home_assistant", "name": "Zuhause", "unassigned_title": "Ohne Raum"}
	w := env.do(t, env.admin, http.MethodPost, "dashboards", body)
	var a hdHATemplateAnswer
	hdDecode(t, w, &a)
	if w.Code != http.StatusCreated || a.Omitted == nil || *a.Omitted != 0 || len(a.Warnings) != 0 || a.Dashboard.Slug != "home-assistant" || a.Dashboard.Name != "Zuhause" ||
		!slices.Equal(pageSummary(a.Dashboard), []string{"kitchen|Kitchen|light/slider=ha:light.kitchen", "kitchen|Kitchen|switch/tile=ha:switch.coffee"}) {
		t.Fatalf("template = %d %s", w.Code, w.Body.String())
	}
	hdDecode(t, env.do(t, env.admin, http.MethodPost, "dashboards", body), &a)
	if a.Dashboard.Slug != "home-assistant-2" {
		t.Fatalf("second slug = %q", a.Dashboard.Slug)
	}
	if w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "system", "unassigned_title": "x"}); w.Code != http.StatusBadRequest {
		t.Fatalf("system with unassigned_title = %d", w.Code)
	}
	var sys hdHATemplateAnswer
	hdDecode(t, env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "system"}), &sys)
	if sys.Omitted != nil {
		t.Fatal("omitted belongs to the home_assistant answer only")
	}
}

func TestHomeDashHATemplateErrors(t *testing.T) {
	env := newHomeDashTestEnv(t, nil)
	var a hdHATemplateAnswer
	w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "home_assistant"})
	hdDecode(t, w, &a)
	if w.Code != http.StatusServiceUnavailable || a.Code != "source_unavailable" || a.Source != "ha" {
		t.Fatalf("no HA source = %d %s", w.Code, w.Body.String())
	}
	sensors := sourcetest.New(hass.SourceID, haEntity("sensor.t", "Kitchen", "kitchen", homedash.KindSensor))
	env2 := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(sensors))
	w = env2.do(t, env2.admin, http.MethodPost, "dashboards", map[string]any{"template": "home_assistant"})
	hdDecode(t, w, &a)
	if w.Code != http.StatusUnprocessableEntity || a.Code != "template_empty" {
		t.Fatalf("nothing controllable = %d %s", w.Code, w.Body.String())
	}
}

func bootTemplates(t *testing.T, env *homeDashTestEnv) []string {
	t.Helper()
	var boot struct {
		Templates []string `json:"templates"`
	}
	hdDecode(t, env.do(t, env.admin, http.MethodGet, "bootstrap", nil), &boot)
	return boot.Templates
}

func TestHomeDashBootstrapAdvertisesHATemplate(t *testing.T) {
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(haTemplateFake()))
	sourcetest.WaitFor(t, 3*time.Second, func() bool { return slices.Contains(bootTemplates(t, env), "home_assistant") },
		"home_assistant listed after the background catalog refresh")
	sensors := sourcetest.New(hass.SourceID, haEntity("sensor.t", "Kitchen", "kitchen", homedash.KindSensor))
	env2 := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(sensors))
	env2.do(t, env2.admin, http.MethodGet, "entities?source=ha", nil) // catalog loaded
	if got := bootTemplates(t, env2); !slices.Equal(got, []string{"system"}) {
		t.Fatalf("templates = %v, want only system", got)
	}
}
```

- [ ] In `internal/server/homedash_api_dashboards_test.go` `starterContractJSON`, add `"icon":"cpu"`, `"icon":"memory"`, `"icon":"disk"`, `"icon":"download"`, `"icon":"upload"`, `"icon":"timer"` after the `variant` of the six widgets, in that order.
- [ ] Run `go test ./internal/server -count=1 -run '(?i)(TestHomeDashHATemplate|TestHomeDashBootstrapAdvertises|TestHomeDashStarterTemplate)'`. Expected: build failure, then FAIL for the starter icons.
- [ ] `internal/server/homedash_template.go`: give `gauge` and `spark` an `icon` parameter and set `Icon` on all six widgets (`cpu`, `memory`, `disk`, `download`, `upload`, `timer` for `uptime`). Update the doc comment: `Widget icons name the header glyphs (decision D8).`
- [ ] `internal/server/homedash_service.go`: add the imports `"sync/atomic"`, and to `homeDashService` add

```go
	ctx      context.Context // server lifetime; background catalog refreshes stop with it
	haProbe  atomic.Bool     // one background HA catalog refresh at a time (templates)
	streams  atomic.Int64    // open SSE streams (endHomeDashStreams waits for them)
```

  and set `ctx: parent` in `newHomeDashService`'s struct literal. `streams` is used in Task 12.
- [ ] `internal/server/homedash_routes.go`:
  - add `homeDashCodeTemplateEmpty = "template_empty"` to the HTTP-layer codes;
  - add it to the 422 case of `homeDashStatus`: `case homedash.CodeInvalidDocument, homedash.CodeInvalidPatch, homedash.CodeLimitExceeded, homeDashCodeTemplateEmpty:`;
  - in `bootstrap` replace `"templates": []string{homeDashTemplateSystem},` with `"templates": h.svc.templates(),`.
- [ ] Create `internal/server/homedash_template_ha.go`:

```go
package server

import (
	"context"
	"slices"
	"sort"
	"strconv"
	"strings"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass"
)

const (
	homeDashTemplateHA        = "home_assistant"
	homeDashHATemplateName    = "Home Assistant"
	homeDashHAUnassignedTitle = "Unassigned"
	homeDashHAWidgetsPerPage  = 24
	homeDashHASlugBase        = "home-assistant"
)

// homeDashHADomainOrder is the widget order inside a page (decision D4).
var homeDashHADomainOrder = []string{"light", "switch", "input_boolean", "scene", "script", "button", "input_button",
	"select", "input_select", "number", "input_number"}

func homeDashAllowedCap(e homedash.Entity, c homedash.Capability) (homedash.CapabilitySpec, bool) {
	for _, s := range e.Capabilities {
		if s.Name == c && s.Allowed {
			return s, true
		}
	}
	return homedash.CapabilitySpec{}, false
}

func homeDashAnyCommandAllowed(e homedash.Entity) bool {
	for _, s := range e.Capabilities {
		if info, ok := homedash.LookupCapability(s.Name); ok && info.Command && s.Allowed {
			return true
		}
	}
	return false
}

// homeDashHAWidget picks the widget for one Home Assistant entity (decision D4). ok is false outside
// the W1 domains or without an allowed command capability. "Allowed" is the adapter's service policy
// (home_assistant.readonly, allowed/blocked services); HomeDash's own read-only switches do not count.
func homeDashHAWidget(e homedash.Entity) (typ, variant string, rank int, ok bool) {
	src, native, err := homedash.ParseEntityID(string(e.ID))
	if err != nil || src != hass.SourceID {
		return "", "", 0, false
	}
	domain, _, _ := strings.Cut(native, ".")
	rank = slices.Index(homeDashHADomainOrder, domain)
	if rank < 0 || !homeDashAnyCommandAllowed(e) {
		return "", "", 0, false
	}
	switch domain {
	case "light":
		if _, dim := homeDashAllowedCap(e, homedash.CapBrightness); dim {
			return "light", "slider", rank, true
		}
		return "light", "tile", rank, true
	case "switch", "input_boolean":
		return "switch", "tile", rank, true
	case "scene", "script", "button", "input_button":
		return "action", "button", rank, true
	case "select", "input_select":
		if spec, ok := homeDashAllowedCap(e, homedash.CapSelectOption); ok && len(spec.Options) <= 4 {
			return "select", "segmented", rank, true
		}
		return "select", "list", rank, true
	default: // number, input_number
		if mode, _ := e.State.Attributes["mode"].(string); mode == "box" {
			return "number", "stepper", rank, true
		}
		return "number", "slider", rank, true
	}
}

// homeDashHAPageID derives a page id from an area id: [a-z0-9_-], at most 28 characters, unique.
func homeDashHAPageID(key string, used map[string]bool) string {
	var b strings.Builder
	for _, r := range strings.ToLower(key) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
		if b.Len() >= 28 {
			break
		}
	}
	id := strings.TrimLeft(b.String(), "-_")
	if id == "" {
		id = "area"
	}
	base := id
	for n := 2; used[id]; n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	used[id] = true
	return id
}

func homeDashHAPageTitle(area string, i int) string {
	t := homedash.CleanText(area, 40)
	if t == "" || homedash.ContainsURL(t) {
		t = "Area " + strconv.Itoa(i+1)
	}
	return t
}

// homeDashTemplateHA builds the Home Assistant dashboard (decision D4): one page per area sorted by
// area name, a last page for entities without area, widgets in domain order then name, at most
// maxPages pages, 24 widgets per page and maxWidgets in total. omitted counts qualifying entities the
// caps left out; ok is false when no entity qualifies. Layouts stay empty: the save pipeline's
// Autopack places the widgets with the catalog default sizes.
func homeDashTemplateHA(entities []homedash.Entity, name, unassigned string, maxPages, maxWidgets int) (homedash.Dashboard, int, bool) {
	type item struct {
		e            homedash.Entity
		typ, variant string
		rank         int
	}
	groups, titles := map[string][]item{}, map[string]string{}
	for _, e := range entities {
		typ, variant, rank, ok := homeDashHAWidget(e)
		if !ok {
			continue
		}
		groups[e.AreaID] = append(groups[e.AreaID], item{e, typ, variant, rank})
		if e.AreaID != "" && titles[e.AreaID] == "" {
			titles[e.AreaID] = e.Area
		}
	}
	if len(groups) == 0 {
		return homedash.Dashboard{}, 0, false
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		if k != "" {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := strings.ToLower(titles[keys[i]]), strings.ToLower(titles[keys[j]])
		if a != b {
			return a < b
		}
		return keys[i] < keys[j]
	})
	if _, ok := groups[""]; ok {
		keys = append(keys, "")
	}
	used := map[string]bool{}
	var pages []homedash.Page
	placed, omitted := 0, 0
	for i, key := range keys {
		items := groups[key]
		sort.Slice(items, func(x, y int) bool {
			if items[x].rank != items[y].rank {
				return items[x].rank < items[y].rank
			}
			a, b := strings.ToLower(items[x].e.Name), strings.ToLower(items[y].e.Name)
			if a != b {
				return a < b
			}
			return items[x].e.ID < items[y].e.ID
		})
		if len(pages) >= maxPages {
			omitted += len(items)
			continue
		}
		title, pageKey := unassigned, "unassigned"
		if key != "" {
			title, pageKey = homeDashHAPageTitle(titles[key], i), key
		}
		compact := true
		page := homedash.Page{ID: homeDashHAPageID(pageKey, used), Name: title, Compact: &compact}
		for j, it := range items {
			if j >= homeDashHAWidgetsPerPage || placed >= maxWidgets {
				omitted++
				continue
			}
			placed++
			page.Widgets = append(page.Widgets, homedash.Widget{ID: "w" + strconv.Itoa(placed), Type: it.typ, Variant: it.variant,
				Bindings: map[string]string{"primary": string(it.e.ID)}, Options: map[string]any{}})
		}
		if len(page.Widgets) > 0 {
			pages = append(pages, page)
		}
	}
	return homedash.Dashboard{
		Name: name,
		Look: homedash.Look{Family: "glass", Palette: homedash.Palette{Mode: "theme"}, Backdrop: homedash.Backdrop{Kind: "theme"},
			Density: "comfortable", RadiusScale: 1},
		Grid:  homedash.Grid{RowUnit: homedash.RowUnit{Auto: true}, Fit: "scroll"},
		Pages: pages,
	}, omitted, len(pages) > 0
}

// homeAssistantTemplate builds the home_assistant template from the HA catalog (fetched when stale).
func (svc *homeDashService) homeAssistantTemplate(ctx context.Context, name, unassigned string) (homedash.Dashboard, int, error) {
	if name == "" {
		name = homeDashHATemplateName
	}
	if unassigned == "" {
		unassigned = homeDashHAUnassignedTitle
	}
	cctx, cancel := context.WithTimeout(ctx, homeDashCatalogWait)
	defer cancel()
	list, status, active := svc.hub.SourceCatalog(cctx, hass.SourceID, true)
	if !active || (len(list) == 0 && status.State == homedash.StatusUnavailable) {
		return homedash.Dashboard{}, 0, homedash.SourceUnavailable(hass.SourceID, "Home Assistant is not available")
	}
	doc, omitted, ok := homeDashTemplateHA(list, name, unassigned, svc.cat.Limits.Pages, svc.cat.Limits.Widgets)
	if !ok {
		return homedash.Dashboard{}, 0, &homedash.Error{Code: homeDashCodeTemplateEmpty, Message: "Home Assistant has no entity this template can control"}
	}
	return doc, omitted, nil
}

// templates lists what bootstrap advertises (decision D4): system always, home_assistant when the
// cached HA catalog qualifies. A stale catalog is refreshed in the background, one refresh at a
// time, so bootstrap never waits for Home Assistant.
func (svc *homeDashService) templates() []string {
	out := []string{homeDashTemplateSystem}
	list, _, active := svc.hub.SourceCatalog(context.Background(), hass.SourceID, false)
	if !active {
		return out
	}
	if svc.haProbe.CompareAndSwap(false, true) {
		go func() {
			defer svc.haProbe.Store(false)
			parent := svc.ctx
			if parent == nil {
				parent = context.Background()
			}
			ctx, cancel := context.WithTimeout(parent, homeDashCatalogWait)
			defer cancel()
			svc.hub.SourceCatalog(ctx, hass.SourceID, true)
		}()
	}
	for _, e := range list {
		if _, _, _, ok := homeDashHAWidget(e); ok {
			return append(out, homeDashTemplateHA)
		}
	}
	return out
}
```

  Note the `break` inside `homeDashHAPageID`: it sits after the `switch` block, so it breaks the loop.
- [ ] `internal/server/homedash_api_dashboards.go`:
  - add `UnassignedTitle string \`json:"unassigned_title"\`` to `homeDashCreateRequest`;
  - in `createDashboard` declare `var extra map[string]any` and use this switch:

```go
	switch {
	case len(req.Document) > 0 && req.Template == "" && req.Name == "" && req.UnassignedTitle == "":
		d, err := homedash.DecodeDashboard(req.Document)
		if err != nil {
			h.fail(err)
			return
		}
		doc = d
	case len(req.Document) == 0 && req.Template == homeDashTemplateSystem && req.UnassignedTitle == "":
		doc, slugBase = homeDashStarterSystem(req.Name), homeDashTemplateSystem
	case len(req.Document) == 0 && req.Template == homeDashTemplateHA:
		d, omitted, err := h.svc.homeAssistantTemplate(h.r.Context(), req.Name, req.UnassignedTitle)
		if err != nil {
			h.fail(err)
			return
		}
		doc, slugBase, extra = d, homeDashHASlugBase, map[string]any{"omitted": omitted}
	case len(req.Document) == 0 && req.Template != "" && req.Template != homeDashTemplateSystem:
		h.bad("unknown template")
		return
	default:
		h.bad("send either a document or a template; unassigned_title belongs to the home_assistant template")
		return
	}
```

  and build the answer as `fields := map[string]any{"dashboard": saved, "warnings": homeDashNonNil(rep.Warnings)}; for k, v := range extra { fields[k] = v }; homeDashWriteOK(h.w, http.StatusCreated, fields)`.
- [ ] Run `go test ./internal/server -count=1 -run '(?i)(HomeDash)' -v`. Expected: PASS, including `TestHomeDashStarterTemplate` with icons and the routes test (`templates` is `["system"]` without an HA source).
- [ ] Stage the seven paths. Lead: detect_changes, then commit with `feat(server): Home Assistant dashboard template, non-blocking bootstrap advertisement, starter icons`.

**Done when:** the HomeDash server tests pass and the template answer matches contract §5.

---

### Task 11: First values — collector sample at start, `system` rates in the first snapshot

**Files:** modify `internal/server/server_routes.go`, `internal/server/homedash_sources.go`, `internal/server/homedash_metrics_ring.go`, `internal/homedash/sources/system/system.go`, `internal/homedash/sources/system/system_test.go`; create `internal/server/homedash_first_values_test.go`.

**Read first:**
- `internal/server/server_routes.go:123-162` (collector inside `Server.run`);
- `internal/server/dashboard_metrics_snapshot.go`;
- `internal/server/dashboard_metrics_snapshot_test.go:48-56` and `internal/server/homedash_metrics_test.go:40-51` (source-text tests: the strings `s.storeSystemMetricsSnapshot(metricsResult.Data, time.Now())`, `s.storeHostBootTime(`, `s.recordHomeDashMetrics()` must stay in `server_routes.go`);
- `internal/server/homedash_metrics_ring.go`;
- `internal/server/dashboard_handlers_summary.go:262` (another consumer of the sample);
- `system.go:28-41,183-220`.

**Impact (lead, MCP):** `Server.run` (expect HIGH/CRITICAL: a very large function; the edit is local to the collector goroutine, so warn and proceed only with the lead's OK), `system.Snapshot`, `system.Source.compute`, `Server.homeDashSystemSnapshot`, `homeDashMetricsRing`. Other consumers of the sample: the dashboard summary (it gets values earlier, which helps), the HomeDash ring and the boot time. The SSE broadcast, `flushSystemWorld` and the CYD refresh stay on the 10 s tick; the first sample is stored but not broadcast.

- [ ] Write `internal/server/homedash_first_values_test.go`:

```go
package server

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/tools"
)

func TestMetricsCollectorSamplesAtStart(t *testing.T) {
	src, err := os.ReadFile("server_routes.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`go func\(\) \{\s*//[^\n]*\n\s*s\.sampleSystemMetrics\(\)\s*\n\s*ticker := time\.NewTicker\(10 \* time\.Second\)`)
	if !re.Match(src) {
		t.Fatal("the collector goroutine must take its first sample before the 10 s ticker (decision D7)")
	}
	if strings.Count(string(src), "s.sampleSystemMetrics()") < 2 {
		t.Fatal("every tick must sample through sampleSystemMetrics")
	}
}

func TestSampleSystemMetricsStoresSnapshotRingAndBootTime(t *testing.T) {
	s := &Server{}
	if _, _, ok := s.sampleSystemMetrics(); !ok {
		t.Skip("host metrics are not readable in this environment")
	}
	if _, ok := s.loadSystemMetricsSnapshot(); !ok {
		t.Fatal("no snapshot after the first sample")
	}
	now := time.Now()
	if n := len(s.homeDashSystemHistory(now.Add(-time.Minute), now.Add(time.Minute))); n != 1 {
		t.Fatalf("ring entries = %d, want 1", n)
	}
}

func TestHomeDashSystemSnapshotCarriesServerRates(t *testing.T) {
	s := &Server{}
	t0 := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	store := func(at time.Time, recv, sent uint64) {
		s.storeSystemMetricsSnapshot(tools.SystemMetrics{Available: map[string]bool{"cpu": true, "memory": true, "disk": true, "network": true},
			Network: tools.NetworkMetrics{BytesRecv: recv, BytesSent: sent}}, at)
		s.recordHomeDashMetrics()
	}
	store(t0, 1_000_000, 500_000)
	if snap, _ := s.homeDashSystemSnapshot(); snap.NetRxMbps != nil || snap.NetTxMbps != nil {
		t.Fatalf("rates after one sample = %v %v", snap.NetRxMbps, snap.NetTxMbps)
	}
	store(t0.Add(10*time.Second), 13_500_000, 3_000_000)
	snap, _ := s.homeDashSystemSnapshot()
	if snap.NetRxMbps == nil || *snap.NetRxMbps != 10 || snap.NetTxMbps == nil || *snap.NetTxMbps != 2 {
		t.Fatalf("rates after two samples = %v %v", snap.NetRxMbps, snap.NetTxMbps)
	}
}
```

- [ ] Append to `internal/homedash/sources/system/system_test.go`:

```go
func TestSystemCatalogUsesServerRates(t *testing.T) {
	t0 := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	rx, tx := 12.5, 1.25
	snap := Snapshot{SampledAt: t0, CPUPercent: 10, CPUOK: true, MemoryOK: true, DiskOK: true, NetworkOK: true, NetRxMbps: &rx, NetTxMbps: &tx}
	src := New(func() (Snapshot, bool) { return snap, true }, nil, 5, func() time.Time { return t0.Add(time.Second) })
	states := func() map[homedash.EntityID]homedash.State {
		list, _ := src.Catalog(context.Background())
		out := map[homedash.EntityID]homedash.State{}
		for _, e := range list {
			out[e.ID] = e.State
		}
		return out
	}
	got := states()
	if st := got["system:net_rx"]; !st.Available || st.Value != 12.5 {
		t.Fatalf("net_rx = %+v", st)
	}
	if st := got["system:net_tx"]; !st.Available || st.Value != 1.25 {
		t.Fatalf("net_tx = %+v", st)
	}
	snap.NetRxMbps, snap.NetTxMbps = nil, nil
	if st := states()["system:net_rx"]; st.Available {
		t.Fatalf("net_rx without server rates and before two polls = %+v", st)
	}
}
```

- [ ] Run `go test ./internal/server -count=1 -run '(?i)(TestMetricsCollectorSamplesAtStart|TestSampleSystemMetrics|TestHomeDashSystemSnapshotCarriesServerRates)'` and `go test ./internal/homedash/sources/system/ -count=1`. Expected: build failures.
- [ ] `system.go`: add to `Snapshot`

```go
	// NetRxMbps and NetTxMbps are the server's rates between its last two samples in Mbit/s; nil until
	// it has two. The adapter prefers them, so a catalog read carries the rates without a run.
	NetRxMbps, NetTxMbps *float64
```

  In `compute`, replace the two `s.rx`/`s.tx` blocks with

```go
	rx, tx := s.rx, s.tx
	if snap.NetRxMbps != nil && snap.NetTxMbps != nil {
		rx, tx = snap.NetRxMbps, snap.NetTxMbps
	}
	if rx != nil {
		out[NativeNetRX] = sample(*rx, snap.NetworkOK)
	}
	if tx != nil {
		out[NativeNetTX] = sample(*tx, snap.NetworkOK)
	}
```

- [ ] `homedash_metrics_ring.go`: add

```go
// latestRates returns the rates of the newest entry when it belongs to the sample taken at at.
func (r *homeDashMetricsRing) latestRates(at time.Time) (rx, tx *float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.n == 0 {
		return nil, nil
	}
	e := r.buf[(r.start+r.n-1)%len(r.buf)]
	if !e.hasRate || !e.t.Equal(at) {
		return nil, nil
	}
	a, b := e.rx, e.tx
	return &a, &b
}
```

- [ ] `homedash_sources.go` `homeDashSystemSnapshot`: before `return out, true` add `out.NetRxMbps, out.NetTxMbps = s.homeDashMetrics.latestRates(m.SampledAt)`.
- [ ] `server_routes.go`: add the helper (it keeps the three literal calls the source-text tests look for):

```go
// sampleSystemMetrics takes one collector sample: it stores the snapshot (dashboard summary, HomeDash
// system source), the host boot time and one HomeDash history ring entry. ok is false when the
// metrics could not be parsed; hostUptime is 0 when unknown.
func (s *Server) sampleSystemMetrics() (data tools.SystemMetrics, hostUptime uint64, ok bool) {
	raw := tools.GetSystemMetrics("")
	var metricsResult struct {
		Status string              `json:"status"`
		Data   tools.SystemMetrics `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &metricsResult); err != nil {
		return tools.SystemMetrics{}, 0, false
	}
	s.storeSystemMetricsSnapshot(metricsResult.Data, time.Now())
	if hostInfo := tools.GetHostInfo(); hostInfo.Uptime > 0 {
		hostUptime = hostInfo.Uptime
		s.storeHostBootTime(time.Now().Add(-time.Duration(hostInfo.Uptime) * time.Second))
	}
	s.recordHomeDashMetrics()
	return metricsResult.Data, hostUptime, true
}
```

  Then rewrite the collector goroutine:

```go
	// Push system metrics to all SSE clients every 10 seconds so the dashboard
	// does not need to poll /api/dashboard/system independently.
	go func() {
		// First sample at once (HomeDash first values, M3 decision D7): stored, not broadcast.
		s.sampleSystemMetrics()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				data, hostUptime, ok := s.sampleSystemMetrics()
				if !ok {
					continue
				}
				payload := map[string]interface{}{
					"available":      data.Available,
					"cpu":            data.CPU,
					"memory":         data.Memory,
					"disk":           data.Disk,
					"network":        data.Network,
					"sse_clients":    sse.ClientCount(),
					"uptime_seconds": int(time.Since(s.StartedAt).Seconds()),
				}
				if hostUptime > 0 {
					payload["host_uptime_seconds"] = int(hostUptime)
				}
				sse.BroadcastType(EventSystemMetrics, payload)
				s.flushSystemWorld(serverCtx)
				if s.CydHub != nil && s.CydHub.HasRecentDevice(2*time.Minute) {
					s.refreshCydSnapshot()
				}
			case <-serverCtx.Done():
				return
			}
		}
	}()
```

- [ ] Run `go test ./internal/homedash/sources/system/ -count=1` and `go test ./internal/server -count=1 -run '(?i)(Metrics|HomeDashSystem|DashboardSummary|TestSampleSystemMetrics)' -v`. Expected: PASS, including the M2 source-text tests and `TestHomeDashSystemStarterEndToEnd`.
- [ ] Stage the six paths. Lead: detect_changes (judge `Server.run` by the listed symbols), then commit with `feat(server): first metrics sample at start; system snapshot carries the server's rates`.

**Done when:** all listed tests pass, and the SSE broadcast, System World flush and CYD refresh still run only on the tick.

---

### Task 12: `revoked shutdown` before the HTTP drain

**Files:** modify `internal/server/homedash_service.go`, `internal/server/homedash_stream.go`, `internal/server/server.go`; create `internal/server/homedash_shutdown_test.go`.

**Read first:**
- `internal/server/server.go:1238-1240` (init/close placement) and `:2193-2236` (`serveWithShutdown`: `beginHTTPDrain` cancels every request context **and** sets the write deadline to now);
- `internal/server/http_drain.go`;
- `internal/homedash/hub.go:159-188` (`Close`: subscribers get `shutdown`, idempotent, waits ≤ 10 s for sources);
- `internal/server/homedash_stream.go:126-198`;
- `internal/server/homedash_stream_test.go:17-87`.

**Impact (lead, MCP):** `Server.serveWithShutdown` (expect HIGH: the shutdown path of every listener; the edit is one call before `beginHTTPDrain`), `hdRequest.stream`, `Server.closeHomeDash` (comment only).

- [ ] Write `internal/server/homedash_shutdown_test.go`:

```go
package server

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

// homeDashStreamVia opens GET stream on the given base URL and returns its events after hello.
func homeDashStreamVia(t *testing.T, base, token string) <-chan hdEvent {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, base+homeDashAPIPrefix+"stream", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("stream = %v %v", resp, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	events := make(chan hdEvent, 64)
	go func() {
		defer close(events)
		sc := bufio.NewScanner(resp.Body)
		var ev hdEvent
		for sc.Scan() {
			line := sc.Text()
			switch {
			case line == "":
				if ev.name != "" {
					events <- ev
				}
				ev = hdEvent{}
			case strings.HasPrefix(line, "event: "):
				ev.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				ev.data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	waitHomeDashEvent(t, events, "hello")
	return events
}

func TestHomeDashStreamRevokedOnShutdown(t *testing.T) {
	fake := sourcetest.New("fake", sourcetest.Entity("fake:a", homedash.KindSensor))
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(fake))
	mux := http.NewServeMux()
	mux.HandleFunc(homeDashAPIPrefix, handleHomeDash(env.s))
	srv := httptest.NewServer(env.s.trackHTTP(mux))
	defer srv.Close()
	events := homeDashStreamVia(t, srv.URL, env.admin)
	sourcetest.WaitFor(t, time.Second, func() bool { return env.svc.streams.Load() == 1 }, "stream counted")

	env.s.endHomeDashStreams(2 * time.Second) // what serveWithShutdown does first
	env.s.beginHTTPDrain()                    // then the drain cancels request contexts
	if ev := waitHomeDashEvent(t, events, "revoked"); ev.data != `{"reason":"shutdown"}` {
		t.Fatalf("revoked = %s", ev.data)
	}
	if n := env.svc.streams.Load(); n != 0 {
		t.Fatalf("%d streams still open", n)
	}
}

func TestServeWithShutdownEndsHomeDashStreamsBeforeTheDrain(t *testing.T) {
	src, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	start := strings.Index(body, "func (s *Server) serveWithShutdown(")
	if start < 0 {
		t.Fatal("serveWithShutdown not found")
	}
	fn := body[start:]
	if end := strings.Index(fn[1:], "\nfunc "); end > 0 {
		fn = fn[:end+1]
	}
	i, j := strings.Index(fn, "s.endHomeDashStreams("), strings.Index(fn, "s.beginHTTPDrain()")
	if i < 0 || j < 0 || i > j {
		t.Fatal("serveWithShutdown must call endHomeDashStreams before beginHTTPDrain (decision D7 gap a)")
	}
}
```

- [ ] Run `go test ./internal/server -count=1 -run '(?i)(TestHomeDashStreamRevokedOnShutdown|TestServeWithShutdownEndsHomeDashStreams)'`. Expected: build failure.
- [ ] `homedash_service.go`: add

```go
// homeDashShutdownGrace bounds how long shutdown waits for open streams to write revoked.
const homeDashShutdownGrace = 2 * time.Second

// endHomeDashStreams closes the hub before the HTTP drain (decision D7 gap a): every open stream
// receives revoked {"reason":"shutdown"} while its request is still alive, then it waits up to grace
// for the stream handlers to return. The store stays open; closeHomeDash closes it after the drain
// (Hub.Close is idempotent). Requests in between answer 503 hub_closed.
func (s *Server) endHomeDashStreams(grace time.Duration) {
	if s == nil {
		return
	}
	svc := s.homeDash.Load()
	if svc == nil {
		return
	}
	svc.hub.Close()
	deadline := time.Now().Add(grace)
	for svc.streams.Load() > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
}
```

  Update the `closeHomeDash` comment: `Start defers it right after initHomeDash, so it runs after the HTTP drain; the hub itself was already closed by endHomeDashStreams before the drain.`
- [ ] `homedash_stream.go` `stream`: right after the successful `Attach` (after `defer sub.Close()`), add

```go
	h.svc.streams.Add(1)
	defer h.svc.streams.Add(-1)
```

- [ ] `server.go` `serveWithShutdown`: between `s.ready.Store(false)` and `s.beginHTTPDrain()` insert
  `s.endHomeDashStreams(homeDashShutdownGrace) // revoked "shutdown" reaches open HomeDash streams before the drain cancels them`
- [ ] Run `go test ./internal/server -count=3 -run '(?i)(TestHomeDashStream|TestServeWithShutdown|TestHTTPDrain|TestBackgroundTasks|TestFailedListenerBind)' -v`. Expected: PASS three times.
- [ ] Stage the four paths. Lead: detect_changes, then commit with `fix(server): HomeDash streams receive revoked shutdown before the HTTP drain`.

**Done when:** the revoked test passes three times in a row and the drain tests still pass.

---

### Task 13: Server end to end with the fake Home Assistant

**Files:** create `internal/server/homedash_ha_e2e_test.go`.

**Read first:** Tasks 4, 6, 7, 9, 10; `internal/server/homedash_system_e2e_test.go`; `internal/server/homedash_stream_test.go:17-87`; contract §4, §5.

**Impact:** none (test only).

- [ ] Write `internal/server/homedash_ha_e2e_test.go`:

```go
package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass"
	"aurago/internal/homedash/sources/hass/hasstest"
	"aurago/internal/homedash/sourcetest"
)

func newHomeDashHAEnv(t *testing.T, ha *hasstest.Server) *homeDashTestEnv {
	t.Helper()
	spec := homeDashHASpec(hass.FactoryOptions{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		WS: hass.WSOptions{HandshakeTimeout: time.Second, PingInterval: time.Hour, PongTimeout: time.Minute, AuthRetry: time.Hour,
			RegistryDebounce: 20 * time.Millisecond, Backoff: []time.Duration{20 * time.Millisecond}, Jitter: -1}})
	return newHomeDashTestEnv(t, func(c *config.Config) {
		c.HomeAssistant.Enabled, c.HomeAssistant.URL, c.HomeAssistant.AccessToken = true, ha.URL, ha.Token
		c.HomeDash.Sources.HomeAssistant = config.HomeDashSourceConfig{Enabled: true, Control: true, PollSeconds: 5}
	}, spec)
}

func waitHAState(t *testing.T, events <-chan hdEvent, id string, match func(homedash.State) bool) homedash.State {
	t.Helper()
	for i := 0; i < 50; i++ {
		var st struct {
			Changes map[string]homedash.State `json:"changes"`
		}
		if err := json.Unmarshal([]byte(waitHomeDashEvent(t, events, "state").data), &st); err != nil {
			t.Fatal(err)
		}
		if s, ok := st.Changes[id]; ok && match(s) {
			return s
		}
	}
	t.Fatalf("no matching state frame for %s", id)
	return homedash.State{}
}

// M3's result end to end: template -> stream -> snapshot -> command -> the fake Home Assistant
// receives the service call -> a state frame confirms.
func TestHomeDashHomeAssistantEndToEnd(t *testing.T) {
	shortenHomeDashStreamTimers(t)
	ha := hasstest.New(t, hasstest.DemoOptions())
	env := newHomeDashHAEnv(t, ha)

	sourcetest.WaitFor(t, 5*time.Second, func() bool {
		var boot struct {
			Templates []string `json:"templates"`
		}
		hdDecode(t, env.do(t, env.admin, http.MethodGet, "bootstrap", nil), &boot)
		return slices.Contains(boot.Templates, "home_assistant")
	}, "bootstrap lists home_assistant")

	w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "home_assistant", "name": "Zuhause", "unassigned_title": "Ohne Raum"})
	var created struct {
		Dashboard homedash.Dashboard `json:"dashboard"`
		Warnings  []homedash.Issue   `json:"warnings"`
		Omitted   int                `json:"omitted"`
	}
	hdDecode(t, w, &created)
	var names []string
	for _, p := range created.Dashboard.Pages {
		names = append(names, p.Name)
	}
	if w.Code != http.StatusCreated || created.Omitted != 0 || len(created.Warnings) != 0 || !slices.Equal(names, []string{"Kitchen", "Living room", "Ohne Raum"}) {
		t.Fatalf("template = %d %s", w.Code, w.Body.String())
	}

	id, events, stop := openHomeDashStream(t, env, env.admin)
	defer stop()
	bound := homeDashBoundIDs(created.Dashboard)
	ids := make([]string, len(bound))
	for i, b := range bound {
		ids[i] = string(b)
	}
	if w := env.do(t, env.admin, http.MethodPut, "stream/subscriptions", map[string]any{"stream_id": id, "entities": ids}); w.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", w.Code, w.Body.String())
	}
	seen := map[homedash.EntityID]homedash.Entity{}
	for len(seen) < len(ids) {
		var snap struct {
			Entities []homedash.Entity `json:"entities"`
		}
		if err := json.Unmarshal([]byte(waitHomeDashEvent(t, events, "snapshot").data), &snap); err != nil {
			t.Fatal(err)
		}
		for _, e := range snap.Entities {
			seen[e.ID] = e
		}
	}
	kitchen := seen["ha:light.kitchen"]
	if kitchen.Kind != homedash.KindLight || kitchen.State.Attributes["brightness_pct"] != 40.0 || kitchen.AreaID != "kitchen" {
		t.Fatalf("snapshot light = %+v", kitchen)
	}

	w = env.do(t, env.admin, http.MethodPost, "commands", map[string]any{"entity": "ha:light.kitchen", "command": "brightness", "args": map[string]any{"value": 60}, "request_id": "e2e-1"})
	var acc struct {
		Accepted   bool            `json:"accepted"`
		Superseded bool            `json:"superseded"`
		Expected   *homedash.State `json:"expected"`
	}
	hdDecode(t, w, &acc)
	if w.Code != http.StatusAccepted || !acc.Accepted || acc.Expected == nil || acc.Expected.Attributes["brightness_pct"] != 60.0 {
		t.Fatalf("command = %d %s", w.Code, w.Body.String())
	}
	calls := ha.Calls()
	if last := calls[len(calls)-1]; last.Domain != "light" || last.Service != "turn_on" || last.Data["entity_id"] != "light.kitchen" || last.Data["brightness_pct"] != 60.0 {
		t.Fatalf("service call = %+v", last)
	}
	waitHAState(t, events, "ha:light.kitchen", func(s homedash.State) bool { return s.Attributes["brightness_pct"] == 60.0 })

	w = env.do(t, env.admin, http.MethodPost, "commands", map[string]any{"entity": "ha:switch.coffee", "command": "toggle", "request_id": "e2e-2"})
	hdDecode(t, w, &acc)
	if w.Code != http.StatusAccepted || acc.Expected == nil || acc.Expected.Value != true {
		t.Fatalf("toggle = %d %s", w.Code, w.Body.String())
	}
	waitHAState(t, events, "ha:switch.coffee", func(s homedash.State) bool { return s.Value == true })
	if !env.audits.has("ha:light.kitchen|brightness|ok") || !env.audits.has("ha:switch.coffee|toggle|ok") {
		t.Fatalf("audit = %+v", env.audits.entries)
	}
}

func TestHomeDashHomeAssistantUnreachableCommandCarriesSource(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	env := newHomeDashHAEnv(t, ha)
	if w := env.do(t, env.admin, http.MethodGet, "entity?id=ha:switch.coffee", nil); w.Code != http.StatusOK {
		t.Fatalf("entity = %d %s", w.Code, w.Body.String())
	}
	ha.Close()
	w := env.do(t, env.admin, http.MethodPost, "commands", map[string]any{"entity": "ha:switch.coffee", "command": "toggle", "request_id": "gone-1"})
	var body struct {
		Code, Source string
	}
	hdDecode(t, w, &body)
	if w.Code != http.StatusServiceUnavailable || body.Code != "source_unavailable" || body.Source != "ha" || strings.Contains(w.Body.String(), hasstest.UpstreamSecret) {
		t.Fatalf("answer = %d %s", w.Code, w.Body.String())
	}
	if !env.audits.has("ha:switch.coffee|toggle|failed") {
		t.Fatalf("audit = %+v", env.audits.entries)
	}
}
```

- [ ] Run `go test ./internal/server -count=3 -run '(?i)(TestHomeDashHomeAssistant)' -v`. Expected: 2 PASS three times. This task changes no production code; if a test fails, fix the defect in the task that owns the code and note it in this commit body.
- [ ] Stage the path. Lead: detect_changes, then commit with `test(server): HomeDash Home Assistant end to end with the fake HA`.

**Done when:** both tests pass three times in a row.

---

### Task 14: Security review (security-agent, D12)

**Scope:** the HA command path and the template route.
- `internal/tools/homeassistant_service_states.go`
- `internal/homedash/sources/hass/commands.go`, `adapter.go` (`Command` feed, `fetchOnce`), `ws_fetch.go`, `history.go`, `camera.go`
- `internal/homedash/command.go` (`execute`/`audit` pass-through)
- `internal/homedash/errors.go`
- `internal/server/homedash_routes.go` (`fail`)
- `internal/server/homedash_sources.go` (`homeDashHASettings`: token handling)
- `internal/server/homedash_template_ha.go`, `internal/server/homedash_api_dashboards.go` (template branch)
- `internal/server/homedash_service.go` (`endHomeDashStreams`, background refresh)

**Checks:**
1. No Home Assistant answer text, transport error text or token reaches a client body, a stream frame or `homedash.Error.Message`. `LogDetail` is scrubbed and bounded; the token is registered as sensitive before every websocket and REST use.
2. The service policy is applied on every call: read-only mode, blocked before allowed, the empty-allowlist refusals (script, lock, cover, …) and the target refusal. `entity_id` is validated and the only target (no `area_id`/`device_id`/`target`/`group.*`); service names come from the fixed domain table.
3. Arguments are type-checked again in the adapter (after the hub's `ValidateArgs`), with no NaN or Inf. `volume_level` scaling is correct.
4. Gates: `sensitive` (script activate) still needs `confirmed:true`; HomeDash gates are re-checked before the call (hub); `read_only` + `blocked_by` for late policy refusals; kiosk principals are not reachable on desktop routes.
5. The template route: write class (`desktop_readonly` on a read-only desktop); body limit; area names and ids are sanitized into page ids and titles (no URL, no control characters); caps against DoS (12 pages, 24 widgets, 400 total); no Home Assistant text in errors.
6. Bootstrap background refresh: at most one goroutine; bounded by `homeDashCatalogWait`; ends with the server lifetime.
7. The `hasstest` package is test infrastructure only. Confirm it is not imported by production code (`go list -deps ./cmd/... | grep hasstest` is empty).
8. History and camera: endpoint built only from validated ids and RFC 3339 times (`haValidEndpoint` passes); 8 MiB cap; content type plus sniff.

- [ ] security-agent reports Critical/Important findings only (lean workflow).
- [ ] maker-agent fixes them in one commit: `fix(homedash): security review findings (M3 command path and template)`. The lead runs detect_changes first.

**Done when:** no Critical or Important finding is open.

---

### Task 15: Documentation (docs-agent)

**Files:** modify `internal/homedash/AGENTS.md`, `internal/server/AGENTS.md`, `internal/tools/AGENTS.md`.

- [ ] `internal/homedash/AGENTS.md`:
  - Purpose: `sources/hass` is now the full Home Assistant adapter (catalog with areas, state stream, reconnect, REST fallback, typed commands, history, camera snapshot). `sources/hass/hasstest` is the importable fake Home Assistant for tests (no import of `hass`).
  - Local Contracts:
    - replace "Commands go through `tools.HACallServiceContext`" with "Commands go through `tools.HACallServiceStates` (typed, never returns Home Assistant's text)";
    - add the adapter locking rule (`a.mu` is held while calling the sink and never across network I/O);
    - catalog reads without a run use the one-shot `WSClient.Fetch` (REST without registries as fallback);
    - polling only with interest; `Error.Source` with `source_unavailable` (audited as `failed`);
    - `Hub.SourceCatalog` is a raw, non-gated read for templates and is never used for client views;
    - widget `icon` is optional and validated; the icon names are a fixed list (`IconNames`), only ever extended together with `hd-icons.js` and the contract.
  - Verification: add `go test ./internal/homedash/sources/hass/... -count=1`.
- [ ] `internal/server/AGENTS.md`, section `### HomeDash API`:
  - add the HA row (`homeDashHASpec`, `homeDashHASettings`: enabled only with the source switch, the integration, a URL and a token; the service lists are copied);
  - templates `system` (with widget icons) and `home_assistant` (`homedash_template_ha.go`, `422 template_empty`, `omitted`);
  - bootstrap lists templates from the cached catalog with one background refresh;
  - replace the "Shutdown" bullet: `serveWithShutdown` calls `endHomeDashStreams` before `beginHTTPDrain`, so streams get `revoked shutdown`; `closeHomeDash` closes the store after the drain;
  - the collector's first sample at start (`sampleSystemMetrics`; no broadcast of the first sample); the system snapshot carries the ring's latest rates;
  - error bodies may carry `source`;
  - verification adds `go test ./internal/server -run '(?i)(HomeDashHomeAssistant|ServeWithShutdown)' -count=1`.
- [ ] `internal/tools/AGENTS.md`, lines about Home Assistant: add `HACallServiceStates` (`homeassistant_service_states.go`). It is HomeDash's typed write path under the same policy and target refusal as `HACallServiceContext`; it returns changed states and `*HAServiceError` without the body. `HACallServiceContext` stays for the agent tool and the HA Switchboard.
- [ ] Run `go test ./internal/audit/... -count=1` (doc link tests). Expected: PASS.
- [ ] Stage the three tracked files (`git add` suffices). Lead: detect_changes, then commit with `docs(homedash): Home Assistant adapter, templates, shutdown order and typed service call`.

**Done when:** the three guides describe the shipped behaviour and the audit package passes.

---

### Task 16: Final verification, race run, hand-over

**Files:** create `docs/superpowers/specs/2026-11-23-homedash/m3-handover-backend.md` (docs-agent).

- [ ] test-agent, in the worktree:
  - `go build ./...`
  - `go vet ./internal/homedash/... ./internal/server/ ./internal/audit/ ./internal/tools/`
  - `go test ./internal/homedash/... ./internal/config/ ./internal/audit/... -count=1`
  - `go test ./internal/tools -count=1 -run '(?i)(TestHA)' -v` (count PASS lines)
  - `go test ./internal/server -count=1 -run '(?i)(HomeDash|Metrics|HTTPDrain|BackgroundTasks|ServeWithShutdown|FailedListenerBind|DesktopReadonly|BluetoothLive)' -v` (count PASS lines)
  - `go test ./internal/server -count=1 -timeout 40m -skip 'Browser'` (full package once; compare with the Task 0 baseline; any new failure blocks)
  - Untouched-files check: `git diff --stat main -- internal/tools/homeassistant.go internal/server/desktop_homeassistant_handlers.go internal/desktop/ha_switchboard.go ui/ internal/httpstream/` must be empty.
- [ ] Race run once (no local gcc), on aurago-test. Follow the test-server rules: scratch directory only; never touch containers, apps or data.

```bash
git archive --format=tar HEAD | ssh aurago-test 'rm -rf /tmp/hd-m3-race && mkdir -p /tmp/hd-m3-race && tar -x -C /tmp/hd-m3-race'
ssh aurago-test 'cd /tmp/hd-m3-race && go test -race -count=2 ./internal/homedash/... && go test -race -count=1 -run "(?i)(TestHA)" ./internal/tools/ && go test -race -count=3 -run "(?i)(HomeDash|ServeWithShutdown)" ./internal/server/'
ssh aurago-test 'rm -rf /tmp/hd-m3-race'
```

  If aurago-test has no Go toolchain or cannot fetch modules, use the WSL approach of memory `reference_linux_go_test_container.md`, and record which one ran.
- [ ] Lead: detect_changes over the whole branch (`scope: "compare"`, `base_ref: "main"`). Judge it by the listed symbols; a partial result is re-run.
- [ ] docs-agent writes `m3-handover-backend.md` (≤ 90 lines):
  - commits table;
  - verification results with PASS counts and the race result;
  - the contract's "Differences from the design (M3)" list;
  - the contradictions resolved (from this plan's self-review);
  - the answers to "Questions for the lead" as given;
  - for the frontend's live-handler browser test (D13): `hasstest.New(t, hasstest.DemoOptions())` gives `URL`/`Token`; set `home_assistant.enabled/url/access_token` and `homedash.sources.home_assistant.enabled`; `internal/server` has no exported harness, so the frontend plan chooses between a test inside `internal/server` and a test binary;
  - open items for M5 (camera scaling, climate history, the sensor sheet, the rest of the icon set);
  - that nothing is pushed or deployed.

  `git add -f` it, confirm with `git ls-files`, and commit with `docs(homedash): M3 backend hand-over`.

**Done when:** every command above is green (or matches the baseline), the race run is clean, and the hand-over is committed.

---

## Batches and reviews

| Batch | Tasks | Review |
|---|---|---|
| 1 Core and tools | 1, 2, 3 | reader-agent once (Critical/Important only): `Error` JSON and audit mapping, `SourceCatalog` locking and copies, widget icon validation, typed call never leaks the body or token. |
| 2 Adapter (B9b) | 4, 5, 6, 7, 8 | reader-agent once: adapter lock order (`a.mu` → sink → `Hub.mu`, no I/O under `a.mu`), phase transitions (websocket ↔ polling), fingerprint-based catalog pushes, `expected` table vs contract, fake fidelity vs the B9a protocol. |
| 3 Server | 9, 10, 11, 12, 13 | reader-agent once: Settings and hot reload (fingerprint includes token and lists), bootstrap never blocks, template caps and ordering, collector change, shutdown order. |
| Security | 14 | security-agent once over the command path and the template route (D12). |
| 4 Docs and closure | 15, 16 | none beyond the race run. |

Tasks inside a batch run sequentially (shared packages and test helpers). Fix review findings in one follow-up commit per batch; no re-review rounds for minor items (lean workflow).

The frontend track starts immediately against the frozen contract, using its fixture API. Its live-handler browser test runs after this branch has landed (D15).

---

## Self-review

### Coverage of D1–D15

| Decision | Where |
|---|---|
| D1 adapter files and behaviour | `adapter.go` and `rest_fallback.go` (Task 6), `catalog.go` (Task 5), `commands.go` (Task 7), `history.go` and `camera.go` (Task 8); registry debounce and backoff come from B9a `ws_client.go` (unchanged); hidden/disabled filter and area resolution: Task 5 tests; REST fallback after 3 failed handshakes with a 5 min window: Task 6 tests; row, Settings, audit: Task 9 |
| D2 contract | "API contract (frozen for M3)" with an example per W1 domain, the commands request, the 202, every error body, the template request and answer, bootstrap, icon names, `source_unavailable`, partial catalogs, the shutdown frame |
| D3 | Task 3 (typed sibling, classified error, no body), Task 7 (`expected` for toggle/turn_on/turn_off/brightness/color_temp/color/effect/select_option/set_value; none for activate/press; verb mapping; changed-state feed), Task 1 (`source`) |
| D4 | Tasks 2 and 10 (pages per area, unassigned last, qualification, variants, order, caps, `omitted`, glass/theme, same pipeline, bootstrap advertisement) |
| D5 | Contract restates limits and timings and defines the confirmation match; no backend change |
| D6 | Contract §2 (`brightness_pct` truth; attribute names; ranges and options in capabilities) |
| D7 | (a) Task 12; (b) untouched, Task 0 branch check; (c) contract §3; `source`: Task 1; first values: Task 11 |
| D8 | Task 2 (`Widget.Icon`), Task 10 (starter icons) |
| D9 | Task 2 (`IconNames`), Task 5 (`iconHint` with the test against the list), contract §6 |
| D10 | No policy edit; `blocked_by` from `Capabilities`; late refusals mapped in Task 7 |
| D12 | Task 14 |
| D13 | Fake with REST (Task 4); verb/expected tables (Task 7); registry/area/hidden tests (Task 5); REST fallback (Task 6); server end to end (Task 13); shutdown frame (Task 12); the fake is importable for the frontend's live-handler test |
| D14 | Task 8 `TestHALiveReadOnly` (environment variables only, read-only unless a toggle entity is named) |
| D15 | File structure, R6; only backend-owned paths |

### Contradictions found and resolutions

1. **D8 "the system template sets widget `icon`" vs the document schema.** `homedash.Widget` has no `icon` field (`internal/homedash/dashboard.go:83-93`), and documents decode strictly. Resolution: add an optional `Widget.Icon`, validated like page icons (Task 2). This is additive in schema v1, and the catalog hash is unchanged. Recorded as Q1.
2. **D13 "extend the B9a fake server" vs importability.** The B9a fake is in `ws_client_test.go` and cannot be imported by `internal/server` or `ui`. Resolution: a new package `hass/hasstest` (Task 4); the B9a tests keep theirs. Recorded as Q8.
3. **Design 8.1 "Commands go through `tools.HACallServiceContext`" vs D3.** Resolution: D3 wins. A new typed sibling is added, the old function stays untouched, and AGENTS.md is updated (Task 15).
4. **`Source.Catalog` "works whether the source is running or not" vs Home Assistant's registries being websocket-only.** Resolution: one-shot `WSClient.Fetch` (Task 5), falling back to REST states without registries (the last known registries apply).
5. **`hubSink.Catalog` pushes `entity` frames only for meta changes, never state frames** (`hub_sources.go:399-418`). Resolution: after every full list, the adapter sends `Update`s for changed or stale states. Catalogs are pushed only when the meta fingerprint changes (Task 6).
6. **`hubSink.Update` drops states older than the stored one and keeps states only for cataloged or subscribed natives.** Resolution: the catalog is pushed before updates; `updated_at` is Home Assistant's `last_updated`; `expected` (server time) is never stored in the hub.
7. **`Hub.History` calls a `HistorySource` for every entity of the source without checking the `history` capability** (`history.go:204-216`). Resolution: the adapter answers `[]` without a request for domains without history (Task 8).
8. **`ValidateArgs` anchors the step grid at the capability `min`** (`capability.go:229-246`). The `color_temp` grid is therefore `min_color_temp_kelvin + k·50`. Resolution: the contract states the rule explicitly.
9. **`BoundAttributes` keeps only `[]any`/`[]string` lists** (`sanitize.go:154-174`). Resolution: `hs_color` in `expected` and service data uses `[]any`.
10. **M2 shutdown order (drain before `closeHomeDash`) vs D7 (a), and `beginHTTPDrain` setting the write deadline to now.** Resolution: `endHomeDashStreams` closes the hub and waits up to 2 s for streams before `beginHTTPDrain` (Task 12). The M2 hand-over and the server AGENTS.md bullet are updated.
11. **D7 "first sample at start" vs the source-text tests** that pin collector calls in `server_routes.go`. Resolution: `sampleSystemMetrics` lives in `server_routes.go` with the same literal calls. The first sample is not broadcast, so SSE, System World and CYD behaviour is unchanged.
12. **D7 "system snapshot carries the current values"**: the catalog read uses `advance=false`, and rates existed only inside a run. Resolution: the server passes the ring's latest rates in `Snapshot` and the adapter prefers them (Task 11).
13. **D4 "bootstrap lists the template only when the catalog qualifies" vs D8's 1.5 s first-values target.** A cold catalog read can take seconds. Resolution: a cached read plus a background refresh, and the client re-reads bootstrap before showing templates. Recorded as Q2.
14. **D4 "allowed command capability"**: the adapter policy vs HomeDash's own switches. Resolution: the adapter's service policy decides (`home_assistant.readonly` and the service lists count; `homedash.readonly`, desktop read-only and the control switch do not). Recorded as Q4.
15. **D4 `omitted` "when entities were left out"**: always present on this template's answer (0 included) for a simpler client, and it counts only entities cut by the caps. Recorded as Q5.
16. **D7 "sources map" vs M2's `sources` array.** Resolution: the contract keeps the array.
17. **Audit outcome**: `source_unavailable` was `refused:` under M2 logic. Resolution: it is `failed` (Task 1).
18. **Design 8.1 camera snapshot "resized like go2rtc" vs D1** (snapshot only; widget in M5). Resolution: no scaling in M3; `w`/`h` are ignored for `ha`. Recorded as Q6.
19. **Design 5 "push sources mark all their entities stale while degraded."** Resolution: subscribed entities only (the hub stores states for the rest; frames go only to subscribers).
20. **D3 lists only W1 verbs; the domain table has more command capabilities** that the hub would admit (climate is allowed by the default policy). Resolution: map every domain-table capability, with `expected` only for W1. Recorded as Q7.
21. **`TestHomeDashSourceSpecsListSystem` pinned exactly one row.** Resolution: it is replaced (Task 9).
22. **M2 `starterContractJSON` pinned the starter without icons.** Resolution: updated (Task 10).
23. **Times**: Home Assistant sends `+00:00`. Go marshals zero offsets as `Z`; the adapter calls `.UTC()` anyway (M2 rule "UTC with Z").
24. **M2 contract "`source_unavailable` reserved, never sent".** Resolution: it is now sent (contract §11).

### Depth notes

Everything protocol-, concurrency- or security-relevant is written out:
- the typed service call and its error classification (Task 3);
- the fake Home Assistant (Task 4);
- the one-shot fetch, area resolution, state and icon mapping (Task 5);
- the adapter run loop, phase transitions, ingest, fingerprinting and stale marking (Task 6);
- verb mapping, `expected` and error mapping (Task 7);
- history and camera (Task 8);
- settings (Task 9);
- template builder and bootstrap (Task 10);
- collector refactor (Task 11);
- shutdown ordering (Task 12).

Every test is complete code, except where a step names a small placeholder for the maker to fix (`errorsIs` in Task 5; the unused marker case in Task 3). No output limit forced omissions.

---

## Questions for the lead

Each item names the decision it touches, why it cannot be implemented literally (or which reading it needs), and the plan's default. The default is what makers implement unless the lead answers otherwise.

- **Q1 (D8, widget `icon`).** The document has no widget-level `icon`. Default: add an optional `Widget.Icon` (schema v1, additive, validated; catalog hash unchanged; Task 2). Alternative: no backend change; the frontend draws header icons from the bound entity's `icon_hint` (the system entities already carry `cpu`, `memory`, `disk`, `download`, `upload`, `timer`). Confirm the schema addition.
- **Q2 (D4 with D8, bootstrap).** A literal "list only when the catalog qualifies" needs a catalog read that can take seconds on a cold source, which conflicts with the 1.5 s first-values target. Default: read only the cached catalog, refresh in the background, and have the client re-read bootstrap when it opens the new-dashboard dialog. Consequence: the first bootstrap after a server start may omit `home_assistant`.
- **Q3 (D5, "value matches expected").** Devices quantize colour temperature (mireds) and clamp colour to their gamut, so exact matches fail. Default: the contract's tolerances (`brightness_pct` ±1, `color_temp_kelvin` ±2 %, `hs_color` ±4/±4). A device outside them rolls back after 8 s and the late frame then shows the real value. Alternative: for `color` and `color_temp`, confirm on `value` only. Decide before gate P.
- **Q4 (D4, "allowed command capability").** Default reading: the Home Assistant service policy (incl. `home_assistant.readonly`), not HomeDash's own read-only switches. Consequence: with `home_assistant.readonly` on, the template is never offered; with `homedash.readonly` on, it is offered and its controls show read-only.
- **Q5 (D4, `omitted` and the empty case).** Default: `omitted` is always present on the `home_assistant` answer and counts only entities cut by the caps. A template with no qualifying entity answers the new code `422 template_empty`. Confirm the new code.
- **Q6 (D1 vs design 8.1, camera).** Default: no scaling in M3 (`w`/`h` ignored for `ha`); scaling arrives with the M5 camera widget.
- **Q7 (D3 scope).** Default: every command capability of the domain table is mapped (climate, cover, fan, lock, media), so the entity view and the adapter never disagree; `expected` exists only for the W1 verbs. Alternative: refuse non-W1 verbs with `unsupported_command` until M5, but then the catalog would show `allowed:true` for commands the adapter refuses.
- **Q8 (D13, the fake).** Default: the new importable package `internal/homedash/sources/hass/hasstest`; the B9a test-file fake stays for the B9a tests.
- **Q9 (D1, "retry the websocket every 5 min").** Default reading: the polling window lasts 5 min, then one websocket attempt, and a failed attempt returns to polling at once. A failing REST poll (Home Assistant down, not only its websocket) returns to the websocket client's own 1–60 s backoff, so recovery after a Home Assistant restart is not delayed by 5 min.
