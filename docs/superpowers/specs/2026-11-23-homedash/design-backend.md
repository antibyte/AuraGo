# HomeDash backend design

> Part of the HomeDash design. Where this part differs from section 4 of `../2026-11-23-homedash-design.md`, that document wins. This part was harmonised with it on 2026-11-23.

Status: design input for the lead architect, 2026-11-23. This is a read-only design; no files were changed. Every finding below cites file:line from this session. GitNexus MCP/CLI was not available to this agent, so there is no blast-radius data. Implementers must run `impact` on every existing symbol named in section 13 before they edit it.

---

## 1. Findings (verified)

**Persistence**
- HA Switchboard keeps its board as **one string row in the Virtual Desktop SQLite key/value table**: key `ha_switchboard.board` (`internal/desktop/ha_switchboard.go:14`), parsed by `ParseHASwitchboard` (16 KiB cap, version 1, at most 60 entries, `DisallowUnknownFields`; `:35-57`), read from `listSettings` (`:60-69`). Table `desktop_settings(key, value, updated_at)` is in `internal/desktop/service.go:448-452`. It has no revision column and no optimistic concurrency.
- The desktop DB is shared by many features: registry v5 migration with backup (`service.go:393-413`), plus `desktop_audit` (`:463-473`).
- Recent features use their **own SQLite file**:
  - Chibi Sensei: `chibisensei.OpenStore` with a single connection (`internal/chibisensei/store.go:23-42`).
  - Docker Manager: `docker_manager.db`.
  - CYD Creator: its own DB.
  - Infra is the exception: it migrates into the shared inventory DB (`internal/infra/store.go:41-57`, opened at `internal/server/desktop_infra.go:286`).
- Every SQLite file must be listed in `config.SQLiteArchivePaths` (`internal/config/sqlite_paths.go:46-111`, e.g. `chibi_sensei` at `:98`). That list drives backup/restore and the agent filesystem protection (`SQLiteProtectedPaths`, `:154-165`).

**Live transport**
- `go.mod:39` has `github.com/gorilla/websocket` as a direct dependency. `coder/websocket` is indirect only (`:120`).
- Desktop hub: 8 clients by default, 64-message channel per client, and broadcast drops when a channel is full (`internal/desktop/hub.go:18-38, 65-78`). `handleDesktopWS` re-checks authorization every second and on every event (`internal/server/desktop_handlers.go:246-275, 289-304`).
- Global SSE drops a slow client by unsubscribing it (`internal/server/sse.go:77-97`). Outgoing JSON goes through `scrubSSEJSON` (`sse.go:74, 170`).
- **An existing per-client subscription stream:** Docker Manager `GET /stream` (SSE) plus `PUT /stream/subscriptions {stream_id,...}` (`internal/server/docker_manager_stream.go:40-165`):
  - Frames: `hello{stream_id, heartbeat_ms, limits}`, a heartbeat comment every 15 s, a 30 s write deadline per write (`docker_manager_operations.go:20-29, 99-142`).
  - Hub: demand-driven with idle stop, at most 32 clients, `Publish` never blocks (`internal/dockerwatch/hub.go:54-241`; contract in `internal/dockerwatch/AGENTS.md:9-11`).
  - Root AGENTS.md:199 says SSE write budgets are renewed through `httpstream`.

**Auth and scopes**
- `authMiddleware` order (`internal/server/auth.go:936-1059`):
  1. lockdown;
  2. host check when auth is off;
  3. origin check when auth is off;
  4. **bypass list** (`:988`);
  5. desktop embed tickets (`:994`);
  6. bearer (`validRouteBearer`, `:1018`);
  7. session with CSRF origin check on writes (`:1029-1036`).
- Bypass subtrees whose handlers authenticate themselves: `/api/game-maker/preview/` and `/api/cyd-creator/device/` (`auth.go:798-813`). The test requires subtrees to end in `/` (`auth_bypass_test.go:34-43`).
- `TokenManager` stores SHA-256 hashes, exact-match scopes, enabled flag and expiry in encrypted `tokens.json` (`internal/security/tokens.go:22-34, 301-340, 413-450`). Creating a token over HTTP accepts only allowlisted scopes (`internal/server/token_scopes.go:15-38`; `internal/server/AGENTS.md:435`).
- **Feature-owned device credential precedent:** CYD Creator device tokens are `ccdt_` + 64 hex, kept as SHA-256 in `cc_devices.token_hash` and never in `tokens.json` (`internal/cydcreator/devices.go:18-21, 121-123`; lookup by hash at `ota.go:333`; unique index at `storage.go:169`).
- "Desktop tickets" / "media tickets" are HMAC tickets bound to one path: `parseDesktopEmbedToken` / `validDesktopEmbedResourceToken`, which only covers `/api/3d-printers/*/camera/stream` (`auth.go:240-341`). `security.ValidCastMediaTicket` handles Cast media (`auth.go:988`).
- go2rtc: `requireGo2RTCView` accepts a bearer with `go2rtc.view` or `admin`, or a session (`go2rtc_handlers.go:21, 53-84`). The proxy allows `api/stream.mjpeg` and `api/frame.jpeg` (`:535`) and checks `sameOriginOrNoOrigin` (`:351`). The viewer page sets its own nonce CSP (`:308`).
- Desktop scopes are `desktop:read|write|admin` (`desktop_auth.go:8-12`). HA routes require `desktop:admin` even for reads (`desktop_homeassistant_handlers.go:58`).
- `virtual_desktop.readonly` blocks every desktop write through `checkDesktopOperation` → `beginDesktopRun` (`desktop_operation.go:75-141`). The special cases for read-classified POSTs are in `desktopRequestOperation` (`:39-73`).

**Pages and CSP**
- UI pages are templates from `uiFiles`, registered in `registerUIRoutes` (`server_routes_ui.go:688-790`). The global CSP and `X-Frame-Options: DENY` are set by the security middleware (`server.go:2325-2369`). Handlers override the CSP per response (go2rtc viewer, `go2rtc_handlers.go:308`).

**Home Assistant**
- `haHTTPClient`: same-origin redirects only, deliberately not SSRF-filtered because it targets LAN HA (`internal/tools/homeassistant.go:15-19`). `haRequestContext` does `ValidateHTTPBaseURL`, `RegisterSensitive` and caps response size (`:43-75`).
- `HAGetStatesContext` drops all attributes except name, device_class and unit (`:112-138`).
- `HACallServiceContext` enforces readonly, the service policy and the target refusal (`:184-246`).
- **The default policy (empty `allowed_services`) blocks** `script`, `lock`, `alarm_control_panel`, `cover`, `automation`, `shell_command` and more (`:272-283`).
- There is no websocket client, no history call and no camera_proxy call anywhere in the repo.
- The poller is REST, every 15 s, only for mission-referenced entities (`homeassistant_poller.go:11-79`). It is rebuilt on config change and in egg mode (`homeassistant_runtime.go:17-55`; called from `config_handlers_main.go:1769` and `server.go:710`).

**Config publication**
- `replaceConfigSnapshot` (`server.go:585`) runs while `CfgMu` is held. It calls non-blocking hooks such as `CYDCreator.UpdateLimits` (`:695`) and `haPoller.cancelIfChanged` (`:710`).

**Sources (key facts)**
- FritzBox:
  - `GetSmartHomeDevices` returns switch, temperature, HKR (radiator) and lamp level/colour temperature (`internal/fritzbox/service_smarthome.go:94-162`). **`BlindPosition` is never filled and there is no blind setter.**
  - Setters `SetSwitch`, `SetHeatingTarget` and `SetLampBrightness` check only `SmartHome.ReadOnly` (`:166-229`), not the sub-feature switches (`config_types.go:1737-1748`).
  - The pooled client and lifetime rules are in `internal/fritzbox/AGENTS.md:25-26`.
- MQTT:
  - The publish gate (`mqtt.enabled`, `!mqtt.readonly`) is enforced in **agent dispatch** (`internal/agent/dispatch_network.go:179-199`), not in `tools.MQTTPublish`.
  - Buffer reads match topic filters (`internal/mqtt/buffer.go:73-92`). The broker only delivers subscribed filters (`controller.go:837-853`). Configured filters are `mqtt.topics` (`config_types.go:2327`).
- Docker: `dockerhost.ListContainers` / `ContainerAction` (`internal/dockerhost/engine.go:143, 302`), and the registry `Hosts`/`Client` (`registry.go:155, 314`). `dockerwatch.Hub.WatchHost` delivers Engine events (`hub.go:120-142`).
- Proxmox: Infra calls `tools.ProxmoxInfraResources(ctx, cfg)` directly (`desktop_infra_sources.go:161`).
- System metrics: a server loop every 10 s stores `storeSystemMetricsSnapshot` (`server_routes.go:125-150`).
- 3D printers: `tools.ExecuteThreeDPrinter(... "status")` (`three_d_printer.go:159-197`). The Elegoo camera needs an explicit activation write (`internal/server/AGENTS.md:745`).
- Uptime Kuma: `tools.FetchUptimeKumaSnapshot` (`uptime_kuma_runtime.go:71-74`) returns `{scraped_at, summary, monitors}` (`internal/tools/uptime_kuma.go:68-72`).
- AdGuard: `AdGuardStats`, `AdGuardStatus` and `AdGuardFilteringToggle(cfg, enabled)` (`internal/tools/adguard.go:151-198`). These do not check readonly themselves.
- TrueNAS: `TrueNASConfig.Enabled/ReadOnly` (`config_types.go:2713-2716`), `Client.Health` / `ListAlerts` (`internal/truenas/client.go:231, 278`).
- Bluetooth: `Manager.Subscribe()` gives debounced changes and `Snapshot(ctx)` gives devices (`internal/bluetooth/manager.go:211-234`).
- Desktop app capability gating: `desktopCapabilities.HasCapability` (`internal/server/bluetooth_live.go:14-42`) answers manifest `Requires`, e.g. `"docker"` (`internal/desktop/types.go:871`).

**Agent tools**
- Pattern for a native tool:
  - schema: `rtlSDRSchema` (`native_tools_rtl_sdr.go:5-15`);
  - appended under a feature flag (`native_tools_integrations.go:1119, 1130`);
  - flag from config (`tooling_policy.go:377-381`) plus a runtime availability check (`:533`);
  - feature key (`native_tools_registry.go:142-146`);
  - dispatch (`dispatch_platform.go:313`);
  - category text (`tool_categories.go:231`);
  - self-test op (`self_test_runtime.go:65`).
- Facade pattern: `storymaker.SetAgentFacade` / `CurrentAgentFacade` (`storymaker_tool.go:61-64, 95`). Titles are wrapped with `security.IsolateExternalData` (`:217`).
- The permission matrix for tools is `audit.ToolPermissionMatrix` (`internal/audit/audit_manifests.go:22-70`; storymaker row `:56`). Route contracts are `RouteContractManifest` (`:123-219`; `/api/desktop/` lists GET/POST/PUT only, `:171`). New network clients must be listed in `NetworkClientInventory` (`:225-251`; rule in `internal/server/AGENTS.md:880`).

**Config pattern**
- Chibi Sensei is the model:
  - type plus `NormalizeChibiSenseiConfig` (`internal/config/chibi_sensei.go:11-68`);
  - field at `config_types.go:2213`;
  - defaults at `config.go:602`;
  - Normalize called at `config.go:1136`, `config_handlers_main.go:710` and `config_patch.go:118`;
  - UI `ui/cfg/chibi_sensei.js`;
  - translations `ui/lang/config/chibi_sensei/<lang>.json` plus `ui/lang/meta.json`.

**Rate limiting and audit**
- The best limiter pattern is `flowRateLimiter`: sliding window per key, zero value ready (`internal/server/flows_handlers.go:72-107`, unexported). Others: `sipRequestLimiter` (`sip_handlers.go:59`), `webhooks.RateLimiter` (`internal/webhooks/handler.go:730`).
- **Research correction:** `docker_manager_api.go:91` is the HTTP method guard `allow`, not a rate limiter.
- Audit: `desktop.Service.AuditWithRequest` writes `desktop_audit` with IP, session hash and user agent (`internal/desktop/service_audit.go:24-51`), with `desktopAuditRequestInfo` (`desktop_remote_security.go:195`). Source constants `agent|user|system` are in `internal/desktop/types.go:13-15`.

---

## 2. Spine corrections

1. **Live transport: SSE plus REST subscriptions, not a websocket.** The repo already has a tested per-client subscription stream (Docker Manager), and it fits better:
   - SSE goes through `httpstream` write budgets.
   - Every client→server message (subscription, command) is plain REST, so it gets the existing auth middleware, CSRF origin check, body limits and envelope for free. A websocket would need its own per-message authorization and origin layer.
   - SSE passes reverse proxies without upgrade configuration.
   - For kiosk, both transports can only use cookies from a browser, so neither has an edge there.
   - Constraint for the frontend: **one stream per page**. Several HomeDash windows on one desktop share it, and the subscription is the union of their entities. This stays within the HTTP/1.1 limit of 6 connections per origin.
2. **Proxmox comes "via Infra snapshot sources" only in the sense of the same tools function.** Infra refresh is server-owned and persists into the infra store. HomeDash calls `tools.ProxmoxInfraResources` itself, the call Infra makes at `desktop_infra_sources.go:161`, and does not depend on the Infra store.
3. **`readonly` per entity is not enough.** One HA source mixes allowed `switch.turn_on` with blocked `lock.unlock`. Each capability carries `allowed` and `blocked_by`. `Entity.readonly` means "no allowed command capability".
4. **The HA Switchboard endpoints are not reused.** `research-app-anatomy.md` §10 suggested reusing them. They are switch-only, fetch all of `/api/states` on every request, and their reads require admin. HomeDash reuses the **tools layer** instead: the HTTP client policy, `HAServiceAllowed` and `HACallServiceContext`. That keeps a single HA policy without a second HTTP path.
5. **Entity ids can contain `/`** (Docker `docker:local/web`). Ids therefore travel in **query strings** (`?id=`), never in path segments, because ServeMux matches the decoded path.
6. **Grid columns are fixed constants** in schema version 1 (lg 12, md 8, sm 4), published in the widget catalog. The document carries no column counts.
7. **Kiosk authority follows the "approved" revision.** A kiosk device may only read and command the entities of the dashboard revision a **user** last saved. Agent saves do not change what a wall tablet can control until a user saves or approves.
8. **The agent gets no command path through HomeDash.** It reads entities and edits dashboards. Device control stays in the existing tools (`home_assistant`, `mqtt_publish`, ...) with their own gates. This matches user decision 4.
9. **History ring scope.** The in-memory ring covers only entities that are or were recently subscribed, downsampled to one point per `sample_seconds`, capped in count. It is not a ring for every entity.

---

## 3. Package layout

### `internal/homedash/` (core)
Imports only the standard library, `internal/security`, `internal/dbutil` and `modernc.org/sqlite`. Never imports `server`, `agent`, `tools` or integration packages.

| File | Purpose |
|---|---|
| `AGENTS.md` | Child guide: contracts, limits, verification (root index row added) |
| `ids.go` | `EntityID`/`SourceID` grammar, `ParseEntityID`, dashboard/page/widget id rules |
| `entity.go` | `Entity`, `State`, `CapabilitySpec`, `MediaInfo`, `Kind` constants |
| `capability.go` | Capability registry (arg schema, sensitive flag, kiosk-denied flag), `ValidateArgs` |
| `sanitize.go` | Bounded text (runes, control chars), attribute value bounding |
| `source.go` | `Source`, `Sink`, optional `HistorySource`/`SnapshotSource`, `SourceStatus`, `Factory` |
| `runtime_config.go` | `RuntimeConfig` (server-built immutable snapshot), per-source fingerprints |
| `hub.go` | `Hub`, `Options`, `NewHub`, `Close`, read APIs |
| `hub_sources.go` | Source runtimes: start on interest, idle stop, backoff, recover, catalog cache |
| `hub_reconfigure.go` | Diff `RuntimeConfig`, rebuild changed sources only, push meta changes |
| `subscriber.go` | Stream client: `Set`, coalescing pending map, `Ready`/`Drain`, close reasons |
| `principal.go` | `Principal` (session/kiosk/agent), `AllowedSet` (entity → commands) |
| `command.go` | Command pipeline: validate, gates, dedupe, latest-wins in-flight slot, audit callback |
| `ratelimit.go` | Sliding-window limiter (copy of `flowRateLimiter` semantics) |
| `history.go` | Numeric rings, minute downsampling, LRU cap |
| `dashboard.go` | `Dashboard` document types |
| `dashboard_validate.go` | `Validate(doc, catalog, known) Report` with JSON-pointer issues |
| `dashboard_layout.go` | Layout checks + deterministic `Autopack` |
| `dashboard_migrate.go` | `schema_version` chain |
| `dashboard_patch.go` | Patch operations (agent tool) |
| `catalog.go` | `WidgetCatalog` types, `Catalog()`, `Hash()` |
| `catalog_widgets.go` | Widget type definitions (data only) |
| `store.go` | Open `homedash.db`, schema, migrations, `Close` |
| `store_dashboards.go` | CRUD, revision history (20), approved revision |
| `store_kiosk.go` | Pairing codes, kiosk devices, token hashing |
| `store_mqtt.go` | User-defined MQTT entity definitions |
| `facade.go` | `AgentFacade`, `SetAgentFacade`, `CurrentAgentFacade` |
| `testdata/widget_catalog.json` | Golden catalog (also read by the frontend contract test) |
| `sourcetest/fake.go` | `FakeSource`, recording sink, fake clock |

### `internal/homedash/sources/<name>/` (one adapter per package)
Each package imports its own integration.
- `hass/`: `ws_client.go`, `ws_messages.go`, `adapter.go`, `catalog.go`, `domains.go`, `attributes.go`, `commands.go`, `history.go`, `camera.go`, `rest_fallback.go`
- `go2rtcsrc/adapter.go`
- `fritz/adapter.go`, `fritz/mapping.go`
- `mqttsrc/adapter.go`, `mqttsrc/jsonpath.go`, `mqttsrc/topics.go`
- one `adapter.go` each in `system/`, `docker/`, `proxmox/`, `printers/`, `kuma/`, `adguard/`, `truenas/`, `bt/`

### Edits outside the new packages
- `internal/tools/homeassistant_raw.go` (new): `HARequestJSON(ctx, cfg, method, endpoint string, body, out any) error` and `HAGetBytes(ctx, cfg, endpoint string, max int64) ([]byte, string, error)`. Both are thin exported wrappers over `haRequestContext`, so the HA client policy stays single.
- `internal/server/`:
  - `homedash_service.go`: init/close, `RuntimeConfig` builder, Reconfigure hook, facade publication
  - `homedash_sources.go`: factories with server dependencies (Go2RTC manager, registry, metrics snapshot, Bluetooth, TrueNAS factory)
  - `homedash_routes.go`: registration and path dispatch
  - `homedash_api_entities.go`, `homedash_api_dashboards.go`, `homedash_api_commands.go`, `homedash_api_mqtt.go`
  - `homedash_stream.go`: SSE writer and subscriptions, shared by desktop and kiosk
  - `homedash_kiosk_auth.go`: pairing, cookie, principal resolution
  - `homedash_kiosk_api.go`, `homedash_kiosk_page.go`
  - `homedash_camera.go`: snapshot and MJPEG relay
- `internal/config/homedash.go` (new), plus field, defaults, Normalize call sites and template.
- `internal/agent/native_tools_homedash.go`, `homedash_tool.go`, plus registration edits.
- `internal/audit/audit_manifests.go` rows.
- `prompts/tools_manuals/homedash.md`.

---

## 4. Core types and interfaces

```go
package homedash

type SourceID string   // ^[a-z][a-z0-9_]{1,15}$ e.g. "ha", "fritzbox", "docker"
type EntityID string   // "<source>:<native>", native: 1..200 bytes, printable, no whitespace/control
type Kind string       // see KindSwitch ... below
type Capability string // see CapToggle ... below

const (
	KindSwitch Kind = "switch"; KindLight = "light"; KindSensor = "sensor"; KindBinarySensor = "binary_sensor"
	KindClimate = "climate"; KindCover = "cover"; KindFan = "fan"; KindLock = "lock"; KindMediaPlayer = "media_player"
	KindCamera = "camera"; KindScene = "scene"; KindScript = "script"; KindButton = "button"; KindNumber = "number"
	KindSelect = "select"; KindHumidifier = "humidifier"; KindVacuum = "vacuum"; KindPresence = "presence"
	KindWeather = "weather"; KindSun = "sun"; KindUpdate = "update"; KindServiceStatus = "service_status"
	KindPrinter = "printer"; KindGeneric = "generic"
)

// Command capabilities carry args; read capabilities (stream, snapshot, history, numeric) carry none.
const (
	CapToggle Capability = "toggle"; CapTurnOn = "turn_on"; CapTurnOff = "turn_off"
	CapBrightness = "brightness"; CapColorTemp = "color_temp"; CapColor = "color"; CapEffect = "effect"
	CapSetTemperature = "set_temperature"; CapSetTemperatureRange = "set_temperature_range"
	CapHVACMode = "hvac_mode"; CapPresetMode = "preset_mode"; CapFanMode = "fan_mode"
	CapOpen = "open"; CapClose = "close"; CapStop = "stop"; CapSetPosition = "set_position"; CapSetTilt = "set_tilt"
	CapLock = "lock"; CapUnlock = "unlock"; CapSetPercentage = "set_percentage"; CapOscillate = "oscillate"
	CapPlayPause = "media_play_pause"; CapNext = "media_next"; CapPrevious = "media_previous"
	CapVolumeSet = "volume_set"; CapVolumeMute = "volume_mute"; CapSourceSelect = "source_select"
	CapPress = "press"; CapActivate = "activate"; CapSetValue = "set_value"; CapSelectOption = "select_option"
	CapSetHumidity = "set_humidity"; CapSetMode = "set_mode"
	CapVacuumStart = "vacuum_start"; CapVacuumPause = "vacuum_pause"; CapVacuumReturn = "vacuum_return"
	CapContainerStart = "container_start"; CapContainerStop = "container_stop"; CapContainerRestart = "container_restart"
	CapStream = "stream"; CapSnapshot = "snapshot"; CapHistory = "history"; CapNumeric = "numeric"
)

type CapabilitySpec struct {
	Name      Capability `json:"name"`
	Allowed   bool       `json:"allowed"`              // false for read caps is meaningless; always true there
	BlockedBy string     `json:"blocked_by,omitempty"` // config key or "kiosk.view_only", "kiosk.denied", "virtual_desktop.readonly"
	Sensitive bool       `json:"sensitive,omitempty"`  // UI must confirm; server requires "confirmed": true
	Min       *float64   `json:"min,omitempty"`
	Max       *float64   `json:"max,omitempty"`
	Step      *float64   `json:"step,omitempty"`
	Options   []string   `json:"options,omitempty"` // ≤ 64 entries, each ≤ 64 runes
}

type MediaInfo struct {
	Snapshot   bool   `json:"snapshot"`
	Live       string `json:"live,omitempty"`        // "go2rtc_viewer" | "mjpeg" | ""
	ViewerPath string `json:"viewer_path,omitempty"` // desktop only, e.g. /api/desktop/integrations/go2rtc/viewer/<id>
}

type State struct {
	Value      any            `json:"value"`                // string | float64 | bool | nil
	Text       string         `json:"text,omitempty"`       // raw state text, ≤ 255 runes
	Attributes map[string]any `json:"attributes,omitempty"` // allowlisted, ≤ 8 KiB JSON
	Available  bool           `json:"available"`
	Stale      bool           `json:"stale,omitempty"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type Entity struct {
	ID             EntityID         `json:"id"`
	Source         SourceID         `json:"source"`
	Kind           Kind             `json:"kind"`
	Name           string           `json:"name"` // external text, ≤ 120 runes
	Area           string           `json:"area,omitempty"`
	AreaID         string           `json:"area_id,omitempty"`
	DeviceClass    string           `json:"device_class,omitempty"`
	Unit           string           `json:"unit,omitempty"`
	IconHint       string           `json:"icon_hint,omitempty"` // catalog icon id, never a URL
	Capabilities   []CapabilitySpec `json:"capabilities"`
	ReadOnly       bool             `json:"readonly"`
	ReadOnlyReason string           `json:"readonly_reason,omitempty"`
	Media          *MediaInfo       `json:"media,omitempty"`
	State          State            `json:"state"`
}

type StateUpdate struct { Native string; State State }

type SourceStatus struct {
	Source SourceID  `json:"source"`
	State  string    `json:"state"`            // disabled|idle|starting|live|polling|degraded|unavailable
	Reason string    `json:"reason,omitempty"` // fixed code: auth_failed, unreachable, timeout, not_configured, ...
	At     time.Time `json:"at"`
}

type Sink interface {
	Catalog(entities []Entity)    // full replacement of this source's entities
	Update(updates ...StateUpdate) // never blocks; hub coalesces
	Status(st SourceStatus)
}

type Source interface {
	ID() SourceID
	Run(ctx context.Context, sink Sink) error       // runs while the hub has interest; returns on ctx.Done
	SetInterest(natives []string)                    // non-blocking hint; poll adapters may ignore
	Catalog(ctx context.Context) ([]Entity, error)   // works whether running or not; ≤ 2000 entities
	Command(ctx context.Context, cmd Command) (CommandResult, error) // must re-check its own gates
}
type HistorySource interface {
	History(ctx context.Context, native string, from, to time.Time, points int) ([]Sample, error)
}
type SnapshotSource interface {
	Snapshot(ctx context.Context, native string, maxW, maxH int) (img []byte, contentType string, err error)
}
type Sample struct { T time.Time `json:"t"`; V float64 `json:"v"` }

type Factory func(cfg RuntimeConfig) (src Source, enabled bool)

type PrincipalKind string // "session" | "kiosk" | "agent"
type Principal struct {
	Kind        PrincipalKind
	Key         string // rate-limit key: session hash, kiosk device id
	DeviceID    string // kiosk only
	DashboardID string // kiosk only
	ViewOnly    bool
	Allowed     *AllowedSet // kiosk only; nil = unrestricted
}
type AllowedSet struct { Commands map[EntityID]map[Capability]bool } // entity present with empty map = read only

type Command struct {
	Entity      EntityID
	Native      string
	Name        Capability
	Args        map[string]any
	Principal   Principal
	RequestID   string // client UUID; deduped 30 s per principal
	Confirmed   bool
	DashboardID string
	WidgetID    string
}
type CommandResult struct { Expected *State `json:"expected,omitempty"` }

type Options struct {
	Now              func() time.Time
	Logger           *slog.Logger
	IdleStop         time.Duration // 60 s
	CatalogTTL       time.Duration // 30 s
	CoalesceInterval time.Duration // 100 ms
	CommandTimeout   time.Duration // 15 s
	Audit            func(ctx context.Context, cmd Command, outcome string) // server binds desktop audit
}

func NewHub(parent context.Context, opts Options) *Hub
func (h *Hub) Register(id SourceID, f Factory)  // before the first Reconfigure
func (h *Hub) Reconfigure(cfg RuntimeConfig)    // non-blocking; safe under CfgMu
func (h *Hub) Attach(p Principal) (*Subscriber, error) // ErrTooManyStreams, ErrHubClosed
func (h *Hub) Subscriber(id string, p Principal) (*Subscriber, bool) // principal must match the attacher
func (h *Hub) Entities(ctx context.Context, f Filter) (items []Entity, total int, sources []SourceStatus)
func (h *Hub) Entity(ctx context.Context, id EntityID) (Entity, error)
func (h *Hub) History(ctx context.Context, id EntityID, from, to time.Time, points int) (Series, error)
func (h *Hub) Snapshot(ctx context.Context, id EntityID, w, hgt int) ([]byte, string, error)
func (h *Hub) Command(ctx context.Context, cmd Command) (CommandResult, error)
func (h *Hub) Sources() []SourceStatus
func (h *Hub) DashboardChanged(id string, revision, approvedRevision int64)
func (h *Hub) RevokeDevice(deviceID, reason string) // ends that device's streams
func (h *Hub) Close()

type Subscriber struct{ /* id (32 hex), principal, set, pending map, ready chan, closed reason */ }
func (s *Subscriber) ID() string
func (s *Subscriber) Set(ids []EntityID) (accepted []EntityID, rejected []Rejection, err error)
func (s *Subscriber) Ready() <-chan struct{}  // closed or signalled
func (s *Subscriber) Drain() Batch            // swaps pending under lock
func (s *Subscriber) Close()
type Rejection struct { ID string `json:"id"`; Reason string `json:"reason"` } // invalid_id|unknown_source|source_disabled|forbidden|limit
type Batch struct { Entities []Entity; States map[EntityID]State; Removed []EntityID; Sources []SourceStatus; Dashboard *DashboardEvent; Closed string }
```

`RuntimeConfig` is a plain struct the server builds from `config.Config`. It holds:
- `Enabled`, `ReadOnly`, `DesktopReadOnly`, `EggMode`, `Limits`, `History`;
- one `HomeDashSourceConfig` and the integration settings each adapter needs (URL, token, gates);
- a `Fingerprint()` per source.

Adapters receive the RuntimeConfig and never read `config.Config` directly.

---

## 5. Hub behaviour

**Lifecycle**
- `initHomeDash` (server start) opens `homedash.db`, builds the hub on `s.integrationCtx`, registers the factories, calls `Reconfigure(buildRuntimeConfig(cfg))` and publishes the agent facade.
- Nothing connects to any device until there is interest.
- Egg mode or `homedash.enabled=false` disables every source and closes every stream with reason `disabled`.

**Interest (demand-driven)**
- A source's interest is the union of:
  - natives subscribed by any stream;
  - a catalog warm-up (an `Entities` call keeps the source warm for `CatalogTTL`);
  - in-flight commands.
- Interest > 0 starts `Run` in its own goroutine, wrapped in `recover`. A crash restarts with backoff 1, 2, 4, 8, 16, 30, 60 s and the status becomes `degraded`.
- Interest 0 arms an idle timer (`IdleStop`, 60 s, generation-guarded as in `dockerwatch.idleStop`). Interest that returns sooner cancels it.
- `SetInterest` is called after every change, outside `hub.mu`.

**Catalog**
- `Entities` queries every enabled source concurrently, 8 s timeout each.
- Results are cached for `CatalogTTL`.
- A failure returns partial data plus that source's status. No source can block another.

**Coalescing and back-pressure**
- A sink `Update` locks `hub.mu`, stores the latest state and writes `pending[id]=state` into every subscriber holding the id, then signals `Ready` without blocking.
- The stream writer waits on `Ready` but flushes at most once per `CoalesceInterval`, then `Drain`s and marshals outside the lock.
- Memory per subscriber is bounded by its subscription size (≤ 300). Nothing is ever dropped; only intermediate values of one entity are skipped ("latest wins").
- A slow client only sees fewer intermediate values. Its write deadline (30 s) ends the stream if it stops reading.

**Staleness**
- Poll sources mark a state `stale` when it is older than 3 × poll interval.
- Push sources mark all their entities stale while `degraded`.

**Config reload**
- `replaceConfigSnapshot` calls `s.HomeDash.Reconfigure(rc)` under `CfgMu`. That call only stores the snapshot and signals the hub goroutine.
- The hub goroutine compares fingerprints. For each changed source it cancels the runtime (waiting for it outside any server lock), drops its catalog cache and rebuilds it through the factory. It restarts the source if there is interest, re-fetches the catalog and pushes `entity` (capabilities and gates may flip) or `removed` frames.
- A change to kiosk enablement or a revoked device ends the affected kiosk streams with `revoked`.

**Commands** (`command.go`), applied in this order:
1. Parse the id. Unknown source → `entity_not_found`.
2. The entity must be in the catalog (fetched if needed) → `entity_not_found`.
3. The capability must be present on the entity → `unsupported_command`. `ValidateArgs` (type, min/max/step, options) → `invalid_args`.
4. Hub gates, re-read from the current RuntimeConfig: `homedash.readonly`, `virtual_desktop.readonly`, `sources.<s>.control` → `read_only` with `blocked_by`.
5. Principal gates for kiosk:
   - entity in `Allowed` with that capability, else `forbidden_entity`;
   - `ViewOnly` → `view_only`;
   - capability flagged kiosk-denied (`unlock`, `container_*`) → `forbidden_entity` with `blocked_by: kiosk.denied`.
6. A sensitive capability without `confirmed:true` → `confirmation_required`.
7. Rate limits:
   - per principal: `commands_per_minute` (default 60) with burst 10 per 10 s;
   - per entity: 5 per 5 s;
   - result `rate_limited` with `retry_after_ms`.
8. Dedupe `RequestID` for 30 s and return the first result.
9. In-flight control: at most 4 concurrent commands per source. For `set_value`-like capabilities (brightness, set_position, set_temperature, volume_set, set_percentage, set_value), a single queued slot per (entity, capability) keeps only the latest pending command.
10. `Source.Command` with `CommandTimeout`. The adapter re-checks its own integration gates.
11. The `Audit` callback (outcome `ok|refused:<code>|failed`).
12. Return `expected` (optimistic state). The real state follows on the stream.

**History**
- `History` uses `HistorySource` (HA) when the source has one, otherwise the ring.
- Ring rules:
  - samples come only from subscribed numeric entities;
  - one point per `sample_seconds` bucket, last value wins;
  - `ring_points` per entity;
  - at most `ring_entities` rings, least recently subscribed evicted first;
  - nothing is persisted.
- The response says `{"origin":"source"|"memory","since":<process start>}`.

**Shutdown**
- `closeHomeDash`: `hub.Close()` cancels sources, ends every stream with reason `shutdown` and waits up to 10 s.
- Then the facade is cleared (`SetAgentFacade(nil)`) and the store is closed.
- It runs before the shared integration contexts and databases close, following the HTTP drain rules in `internal/server/AGENTS.md:436`.

---

## 6. API contract

**Envelope** (Docker Manager style):
- success: `{"status":"ok", ...}`
- failure: `{"status":"error","code":"<code>","message":"<english>", ...extra}`
- `Cache-Control: no-store` on everything.
- JSON bodies use `decodeJSONRequest` with a route cap (64 KiB, dashboards 512 KiB) and return 413 `body_too_large`.

**Auth classes**
- **D-admin**: `/api/desktop/homedash/*`. `requireDesktopPermission(..., desktopScopeAdmin)` means a browser session, or a bearer with `desktop:admin`. Writes also pass the desktop operation gate (`desktop_readonly` 403).
- **D-admin-read**: the same, but classified `desktopRead`, so it still works on a read-only desktop. Add the paths to `desktopRequestOperation`.
- **Kiosk**: `/api/homedash/kiosk/*`. The path is in `authBypassSubtrees`. The handler requires the kiosk cookie. Writes also require `checkCSRFOriginWithPolicy(r, true)` and `Content-Type: application/json`.
- **Public page**: `/homedash/kiosk/{dashboard_id}`. In `authBypassSubtrees`. Static HTML with no data.

**Common errors**

| HTTP | Code |
|---|---|
| 400 | `bad_request`, `invalid_args`, `unsupported_command`, `pairing_invalid` |
| 401 | `kiosk_unpaired`, `kiosk_revoked` |
| 403 | `read_only` (+`blocked_by`), `desktop_readonly`, `forbidden_entity`, `view_only`, `csrf_check_failed` |
| 404 | `dashboard_not_found`, `entity_not_found`, `stream_not_found`, `kiosk_disabled` |
| 405 | `method_not_allowed` |
| 409 | `revision_conflict` (+`current_revision`), `confirmation_required`, `schema_too_new` |
| 413 | `body_too_large` |
| 422 | `invalid_document` (+`issues[]`), `limit_exceeded` |
| 429 | `rate_limited` (+`retry_after_ms`), `too_many_streams` |
| 502 | `command_failed`, `source_error` |
| 503 | `homedash_disabled`, `source_unavailable` (+`source`) |

**Desktop routes**

| Method | Path | Class | Purpose |
|---|---|---|---|
| GET | `/api/desktop/homedash/bootstrap` | D-admin-read | versions, limits, source statuses, catalog hash, flags |
| GET | `/api/desktop/homedash/widget-catalog` | D-admin-read | widget catalog (ETag = hash) |
| GET | `/api/desktop/homedash/entities?source=&kind=&area=&q=&limit=&offset=` | D-admin-read | entity catalog with current state (limit ≤ 500) |
| GET | `/api/desktop/homedash/entity?id=` | D-admin-read | one entity |
| GET | `/api/desktop/homedash/history?id=&from=&to=&points=` | D-admin-read | series; range ≤ 7 d, points ≤ 500 |
| GET | `/api/desktop/homedash/snapshot?id=&w=&h=` | D-admin-read | `image/jpeg`, ≤ 8 MiB, never stored |
| POST | `/api/desktop/homedash/commands` | D-admin | command |
| GET | `/api/desktop/homedash/stream` | D-admin-read | SSE |
| PUT | `/api/desktop/homedash/stream/subscriptions` | D-admin-read | replace subscription |
| GET / POST | `/api/desktop/homedash/dashboards` | read / D-admin | list summaries / create |
| GET / PUT / DELETE | `/api/desktop/homedash/dashboard?id=` | read / D-admin / D-admin | get / replace / delete (`&revision=` on DELETE) |
| POST | `/api/desktop/homedash/dashboard/duplicate?id=` | D-admin | copy (new id/slug) |
| POST | `/api/desktop/homedash/dashboards/validate` | D-admin-read | validation report, nothing saved |
| GET | `/api/desktop/homedash/dashboard/revisions?id=` | D-admin-read | last 20 (`revision, saved_at, saved_by`) |
| POST | `/api/desktop/homedash/dashboard/restore?id=&revision=` | D-admin | restore an old revision as a new one |
| POST | `/api/desktop/homedash/dashboard/approve?id=&revision=` | D-admin | set `approved_revision` (a user save approves automatically) |
| GET / POST / PUT / DELETE | `/api/desktop/homedash/mqtt-entities[?id=]` | read / D-admin | user-defined MQTT entities |
| GET | `/api/desktop/homedash/kiosk/devices?dashboard_id=` | D-admin-read | paired devices |
| POST | `/api/desktop/homedash/kiosk/pairings` | D-admin | create a pairing code |
| DELETE | `/api/desktop/homedash/kiosk/device?id=` | D-admin | revoke a device (allowed on a read-only desktop: `desktopStop`) |

**Examples**

`GET bootstrap`:
```json
{"status":"ok","api_version":1,"stream_protocol":1,"schema_version":1,"catalog_hash":"9f3a…",
 "readonly":false,"desktop_readonly":false,"kiosk_enabled":true,
 "limits":{"dashboards":50,"subscriptions":300,"widgets_per_page":120,"pages":12,"document_bytes":262144},
 "sources":[{"source":"ha","state":"live","at":"2026-11-23T18:00:00Z"},{"source":"fritzbox","state":"idle","at":"…"}]}
```

Entity (inside `entities[]`, snapshot frames and `entity` frames):
```json
{"id":"ha:light.kitchen","source":"ha","kind":"light","name":"Kitchen","area":"Kitchen","area_id":"kitchen",
 "capabilities":[{"name":"toggle","allowed":true},{"name":"brightness","allowed":true,"min":0,"max":100,"step":1},
  {"name":"color_temp","allowed":true,"min":2200,"max":6500,"step":50},{"name":"history"}],
 "readonly":false,
 "state":{"value":true,"text":"on","attributes":{"brightness_pct":62,"color_temp_kelvin":3000,"color_mode":"color_temp"},
  "available":true,"updated_at":"2026-11-23T18:04:11Z"}}
```

`POST commands`:
```json
{"entity":"ha:light.kitchen","command":"brightness","args":{"value":40},"request_id":"5f0c…","dashboard_id":"d_k3x9q2m7a1bc","widget_id":"w3","confirmed":false}
```
returns 202:
```json
{"status":"ok","accepted":true,"expected":{"value":true,"text":"on","attributes":{"brightness_pct":40},"available":true,"updated_at":"…"}}
```

Argument names per capability (ValidateArgs):

| Capability | Args |
|---|---|
| `toggle`, `turn_on`, `turn_off`, `open`, `close`, `stop`, `lock`, `unlock`, `press`, `activate`, `media_*`, `vacuum_*`, `container_*` | `{}` |
| `brightness`, `set_position`, `set_tilt`, `set_percentage`, `set_humidity`, `volume_set`, `set_value` | `{value:number}` |
| `color_temp` | `{kelvin:number}` |
| `color` | `{h:0..360, s:0..100}` |
| `set_temperature` | `{value}` |
| `set_temperature_range` | `{low, high}` |
| `hvac_mode`, `preset_mode`, `fan_mode`, `effect`, `source_select`, `select_option`, `set_mode` | `{option:string}` |
| `oscillate`, `volume_mute` | `{on:bool}` |

`PUT dashboard?id=` body is `{"revision":7,"document":{…}}`:
- success 200: `{"status":"ok","dashboard":{…,"revision":8},"warnings":[…]}`
- stale: 409 `{"status":"error","code":"revision_conflict","current_revision":9}`
- invalid: 422 `issues:[{"path":"/pages/0/widgets/2/bindings/primary","code":"binding_required","message":"…"}]`

`POST kiosk/pairings` body `{"dashboard_id":"d_…","label":"Hallway tablet","view_only":false}` returns:
```json
{"status":"ok","pairing_path":"/homedash/kiosk/d_k3x9q2m7a1bc#pair=Q7ZK…","expires_at":"…"}
```
The code (128-bit, base32) is in the URL **fragment**, so it never reaches server logs or proxies.

**Kiosk routes**

| Method | Path | Purpose |
|---|---|---|
| GET | `/homedash/kiosk/{dashboard_id}` | HTML shell (own CSP, see §9) |
| POST | `/api/homedash/kiosk/pair` | `{dashboard_id, code}`. On success: device row, `Set-Cookie: aurago_homedash_kiosk=<hdk_…>; Path=/api/homedash/kiosk/; HttpOnly; SameSite=Strict; Secure` (on HTTPS); `Max-Age` = device TTL or 400 days. 5 attempts per IP per minute. Unknown, expired or used codes all return `pairing_invalid`. |
| GET | `/api/homedash/kiosk/session` | `{status, dashboard_id, device_id, view_only}` or 401 |
| GET | `/api/homedash/kiosk/dashboard` | the **approved** revision document plus the `entities` meta of its bindings |
| GET | `/api/homedash/kiosk/stream` | SSE limited to the allowed set |
| PUT | `/api/homedash/kiosk/stream/subscriptions` | subset of the allowed set |
| POST | `/api/homedash/kiosk/commands` | same body as desktop; kiosk principal |
| GET | `/api/homedash/kiosk/history?id=` | allowed set only |
| GET | `/api/homedash/kiosk/snapshot?id=` | allowed set only |
| GET | `/api/homedash/kiosk/mjpeg?id=` | live camera relay (go2rtc / printer), allowed set only |
| POST | `/api/homedash/kiosk/logout` | deletes the device row and clears the cookie |

**Stream protocol** (SSE; `event: <name>`, `data: <compact JSON>`):
1. `hello`: `{"stream_id":"<32 hex>","protocol":1,"heartbeat_ms":15000,"limits":{"entities":300},"server_time":"…","principal":"session"|"kiosk"}`
2. The client sends `PUT …/stream/subscriptions` with `{"stream_id","entities":[ids]}` (full replacement, ≤ limit). The answer is `{"status":"ok","accepted":[…],"rejected":[{"id","reason"}]}`. Valid ids of enabled sources that are not in the catalog yet are accepted and reported unavailable.
3. `snapshot`: `{"entities":[Entity…],"sources":[SourceStatus…]}`, sent for ids newly added by each accepted PUT.
4. `state`: `{"changes":{"<id>":State,…}}`, coalesced, at most every 100 ms.
5. `entity`: `{"entities":[Entity…]}` when meta, capabilities or gates change.
6. `removed`: `{"ids":[…]}`.
7. `source`: SourceStatus, for streams holding entities of that source.
8. `dashboard_changed`: `{"id","revision","approved_revision"}`. Desktop streams get it for all dashboards; kiosk streams only for theirs, and only when `approved_revision` changes.
9. `revoked`: `{"reason":"device_revoked"|"kiosk_disabled"|"disabled"|"auth_lost"|"shutdown"}`, after which the stream ends.
10. A heartbeat comment `: hb` every 15 s.

Stream rules:
- The handler re-validates authorization every 5 s: desktop like `desktopWSAuthorizationValid`; kiosk by device lookup plus `kiosk.enabled`.
- The 30 s write deadline is armed before each write.
- Every frame goes through `scrubSSEJSON`.
- Caps: 32 desktop and 16 kiosk streams (`too_many_streams`).
- Reconnect: the client closes the EventSource on `error`, waits with backoff 1 s up to 30 s, reopens and sends a new PUT. There is no `Last-Event-ID`, because the protocol resumes from snapshots.

**Versioning**
- REST paths are unversioned. `api_version` and `stream_protocol` are integers in bootstrap and hello.
- Inside v1 only additive changes are allowed: new optional fields and new frame names, which clients ignore.
- A breaking change gets new paths (`/api/desktop/homedash/v2/…`).

**Audit manifest**
- Add `RouteContract`s:
  - `/api/desktop/homedash/` with GET/POST/PUT/DELETE, `session-or-desktop-admin-token`, `application/json` + `text/event-stream` + `image/jpeg`;
  - `/api/homedash/kiosk/` with `homedash-kiosk-device-cookie`;
  - `/homedash/kiosk/` with `public-static`.

---

## 7. Dashboard document schema and widget catalog

**Document**
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

**Fields and rules**
- **Server-owned:** `id` (`d_` + 12 base32 lower), `revision`, `approved_revision`, `updated_at`, `saved_by` (`user|agent`). On create these are ignored. On update `revision` must match.
- `schema_version`: integer.
  - newer than supported → read-only, `schema_too_new`;
  - older → migrated in memory by `dashboard_migrate.go` and persisted on the next save.
- `slug`: `^[a-z0-9][a-z0-9-]{0,47}$`, unique.
- `name`: 1–80 runes. `icon`: catalog icon id `^[a-z0-9-]{1,48}$`.
- **No URLs anywhere in the document.**
- `look.family` ∈ glass, tactile, minimal, instrument.
- `look.palette.mode` ∈ theme, dark, light, custom. `custom` holds the inputs `{accent: ^#[0-9a-f]{6}$, surface_hue: 0–359, scheme: dark|light}`; colours are derived in the client. `snapshot` is optional: a map of at most 40 known colour token names to `^#[0-9a-f]{6}$`, written by the designer when the mode is `theme` and used by the kiosk page; unknown names or other values are rejected.
- `look.backdrop.kind` ∈ theme, solid, gradient. Colours come from the palette; there are no colour fields. Image backdrops are deferred because they would need asset URLs.
- `look.density` ∈ compact, comfortable, spacious. `look.radius_scale` 0.5–1.5.
- `grid.row_unit` is `"auto"` or 48–200. `gap` is null or 0–32. `fit` ∈ scroll, fit_screen. Column counts are catalog constants (lg 12, md 8, sm 4).
- `pages`: 1–12. Page `id` `^[a-z0-9][a-z0-9_-]{0,31}$`, unique. `name` 1–40 runes. `compact` is a boolean, default true.
- `widgets`: ≤ 120 per page, ≤ 400 in total. Widget ids are unique per document.
- Widget fields:
  - `type` and `variant` must exist in the catalog; `family` is an optional override.
  - `title` ≤ 80 runes.
  - `bindings`: keys must be declared by the type; required ones present; values syntactically valid entity ids; ≤ 8.
  - `options`: validated against the type's option schema; ≤ 32 keys; strings ≤ 200 runes.
  - `actions.tap` and `actions.hold` are objects `{do}` with `do` ∈ default, toggle, more_info, navigate, none. `navigate` needs `page` set to an existing page id. `toggle` needs the type to declare the toggle command for `primary`.
- **Entity existence is not required.** Offline entities stay on the board, as HA Switchboard does today. When the entity is known, a kind/capability mismatch is a **warning**; unknown entities are warnings too.
- `layouts.lg` is required; `md` and `sm` are optional. When absent they are automatic: the client derives them and the server does not fill them. Each layout item is `{id,x,y,w,h}`, integers. `x+w ≤ columns` of that breakpoint; the type's min ≤ w,h ≤ max; `y` ≤ 500. Every widget appears exactly once per given layout, and cells do not overlap. `Autopack` places widgets missing from `lg` deterministically (first fit, top-left scan in widget order), so the agent can omit layouts. The server never compacts; the client does when `compact` is true.
- `kiosk`:
  - `start_page` = an existing page id;
  - `idle_dim_seconds` 0–3600; `idle_dim_level` 0.05–1;
  - `page_rotation_seconds` 0 or 10–3600;
  - `screen_saver` ∈ none, clock, black;
  - `show_clock` bool.
- Text fields contain no control characters and must be valid UTF-8. The whole document is ≤ 256 KiB.
- `confirm` (bool) forces a UI confirmation. Sensitive capabilities are always confirmed regardless.

**Store schema** (`homedash.db`, single connection, WAL)
- `hd_meta(id=1, version)`
- `hd_dashboards(id PK, slug UNIQUE, name, schema_version, revision, approved_revision, document_json, saved_by, created_at, updated_at)`
- `hd_dashboard_revisions(dashboard_id, revision, document_json, saved_by, saved_at, PK(dashboard_id, revision))`, keeping the last 20
- `hd_kiosk_pairings(code_hash PK, dashboard_id, label, view_only, expires_at, used_at)`
- `hd_kiosk_devices(id PK, dashboard_id FK ON DELETE CASCADE, label, token_hash UNIQUE, view_only, created_at, last_seen_at, last_ip, expires_at)`
- `hd_mqtt_entities(id PK, name, kind, state_topic, json_path, unit, device_class, command_topic, payload_on, payload_off, options_json, min, max, step, qos, retain, updated_at)`

Store rules:
- A save by a user session sets `approved_revision = revision`. An agent save leaves it unchanged.
- Deleting a dashboard cascades to its kiosk devices, and their streams are revoked.
- Register it as `HomeDashDBFilename = "homedash.db"` with archive key `homedash` in `sqlite_paths.go`.
- **Why a separate DB rather than `desktop_settings`:** documents need revisions, optimistic concurrency, a credential table and per-row size. The shared desktop registry has one value per key, no revisions, and a global migration and backup step. This also matches the CYD Creator, Chibi Sensei and Docker Manager convention. JSON files in the workspace (as Notes does) would bypass validation and expose kiosk hashes to file tools.

**Widget catalog** (Go is the source of truth; JSON-served; golden file `internal/homedash/testdata/widget_catalog.json`)
```go
type WidgetCatalog struct {
	Version      int            `json:"version"`
	Hash         string         `json:"hash"` // sha256 of the canonical JSON without hash
	Families     []string       `json:"families"`
	Kinds        []Kind         `json:"kinds"`
	Capabilities []CapabilityInfo `json:"capabilities"` // name, args schema, sensitive, kiosk_denied
	Widgets      []WidgetType   `json:"widgets"`
}
type WidgetType struct {
	Type        string        `json:"type"`
	Category    string        `json:"category"` // control|sensor|climate|media|camera|layout|status
	Variants    []Variant     `json:"variants"`
	Bindings    []BindingSpec `json:"bindings"`
	Options     []OptionSpec  `json:"options"`
	MinSize     Size          `json:"min_size"`
	DefaultSize Size          `json:"default_size"`
	MaxSize     Size          `json:"max_size"`
	Commands    map[string][]Capability `json:"commands"` // binding name -> commands this widget may issue
}
type Variant     struct { ID string `json:"id"`; MinSize *Size `json:"min_size,omitempty"` }
type BindingSpec struct { Name string `json:"name"`; Required bool `json:"required"`; Kinds []Kind `json:"kinds"`; AnyCapability []Capability `json:"any_capability,omitempty"` }
type OptionSpec  struct { Key string `json:"key"`; Type string `json:"type"` /* bool|int|number|string|enum|color|page */; Default any `json:"default"`
	Min *float64 `json:"min,omitempty"`; Max *float64 `json:"max,omitempty"`; Step *float64 `json:"step,omitempty"`; Enum []string `json:"enum,omitempty"`; MaxLen int `json:"max_len,omitempty"` }
type Size struct { W int `json:"w"`; H int `json:"h"` }
```

- Labels are not in the catalog. The frontend i18n keys are `homedash.widget.<type>.name` and `homedash.widget.<type>.variant.<id>`.
- `Commands` builds the kiosk `AllowedSet`: for each widget of the approved revision, for each binding, the entity gets those commands. Read-only widgets give read access only.

The types, variants, sizes and bindings of version 1 are the table in section 4.3 of the main document (24 types, 47 variants). The catalog also publishes the grid constants and marks family aliases of variants.

| Type | Variants | Min / default | Binds |
|---|---|---|---|
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

**Frontend contract test:** a Node test reads the golden JSON and asserts the widget registry implements every type × variant.

---

## 8. Source adapters

Shared rules for every adapter:
- Names and states from devices are external text: bounded and control characters stripped in Go, rendered with `textContent` in the UI.
- Gates are re-read from the current RuntimeConfig on every command.
- Each source's `poll_seconds` is clamped to its bounds.
- A source runs only when both `homedash.sources.<s>.enabled` **and** the integration's own `enabled` are true.

### 8.1 Home Assistant (`ha`), effort L (split WS client M + adapter M)

**Transport**
- Websocket URL: `ws(s)://<home_assistant.url>/api/websocket`, derived from the validated base URL (`security.ValidateHTTPBaseURL`), using a `gorilla/websocket.Dialer` (no new dependency).
- `HandshakeTimeout` 10 s. A redirect fails the handshake, which matches the same-origin policy. Call `RegisterSensitive(token)` before dialing.
- Read limit 16 MiB; `get_states` on large installs is several MB.

**Protocol** (external HA API, not present in the repo)
1. Server sends `auth_required`. Client sends `{type:"auth",access_token}`. Server answers `auth_ok` or `auth_invalid`. `auth_invalid` → status `auth_failed`, retry only every 5 min or on reconfigure.
2. Requests `id` 1–5:
   - `get_states`;
   - `subscribe_events {event_type:"state_changed"}`;
   - `config/area_registry/list`, `config/device_registry/list`, `config/entity_registry/list`.
3. Subscribe to `area_registry_updated`, `device_registry_updated` and `entity_registry_updated`. Re-list the registries debounced 2 s.
4. Area resolution: entity registry `area_id`, else the device's `area_id`. Entities hidden or disabled in the registry are excluded from the catalog.
5. Keepalive: `ping` every 30 s; a missing `pong` within 10 s → reconnect.

**Reconnect and fallback**
- Reconnect with backoff 1, 2, 4, 8, 16, 30, 60 s and ±20 % jitter. Every reconnect re-runs `get_states` and pushes changed states.
- REST fallback: after 3 failed handshakes while REST `/api/` answers (a websocket blocked by a proxy), switch to `polling`: `HARequestJSON GET /api/states` every `poll_seconds` (default 5, minimum 2) while there is interest. Retry the websocket every 5 min.

**History and camera**
- History: `GET /api/history/period/<from RFC3339>?filter_entity_id=<id>&end_time=<to>&minimal_response&no_attributes` through `HARequestJSON`. Range ≤ 7 days, downsampled to ≤ `points`.
- Camera snapshot: `GET /api/camera_proxy/<camera.x>` through `HAGetBytes` (≤ 8 MiB), then resized and bounded like go2rtc snapshots.
- Live HA camera: deferred; use go2rtc.

**Commands** go through `tools.HACallServiceContext`, which enforces `home_assistant.readonly`, `allowed_services`/`blocked_services` and the target refusal. At catalog time `Allowed` is computed per capability with `tools.HAServiceAllowed(cfg, domain, service)`, with `blocked_by:"home_assistant.allowed_services"` or `"home_assistant.readonly"`.

**Attributes**
- Allowlist per domain. Kept: friendly_name→name, unit_of_measurement, device_class, state_class, brightness (as brightness_pct), color_mode, supported_color_modes, color_temp_kelvin, min/max_color_temp_kelvin, hs_color, effect, effect_list, current_temperature, temperature, target_temp_low/high, target_temp_step, min_temp, max_temp, hvac_modes, hvac_action, preset_mode(s), fan_mode(s), current_position, current_tilt_position, percentage, percentage_step, oscillating, direction, media_title, media_artist, source, source_list, volume_level, is_volume_muted, options, min, max, step, mode, battery_level, installed_version, latest_version, next_rising, next_setting, elevation, humidity, pressure, wind_speed, wind_bearing.
- **Always dropped:** `entity_picture` (camera entities carry an access-token URL), `access_token`, `latitude`/`longitude`/`gps_accuracy` (presence privacy), anything containing `url`, `token` or `password`.
- Strings ≤ 256 runes, lists ≤ 64, total ≤ 8 KiB.

**Domain table**

| HA domain | Kind | v1 | Capabilities → service |
|---|---|---|---|
| switch, input_boolean | switch | yes | toggle/turn_on/turn_off → `<d>.turn_on/off/toggle` |
| light | light | yes | toggle, brightness (`brightness_pct`), color_temp (`color_temp_kelvin`), color (`hs_color`, if supported_color_modes ∋ hs/rgb/xy), effect → `light.turn_on/off` |
| sensor | sensor | yes | numeric+history if the state parses as a float; else text |
| binary_sensor | binary_sensor | yes | read-only |
| climate | climate | yes | set_temperature, set_temperature_range, hvac_mode, preset_mode, fan_mode → `climate.set_*` |
| cover | cover | yes | open/close/stop/set_position/set_tilt → `cover.*`. **Blocked by the default policy** (allowlist needed). Sensitive for device_class garage/gate/door. |
| fan | fan | yes | toggle, set_percentage, preset_mode, oscillate → `fan.*` |
| lock | lock | yes | lock; unlock is sensitive and **kiosk-denied**. **Blocked by the default policy.** |
| media_player | media_player | yes | play_pause, next, previous, volume_set, volume_mute, source_select, toggle |
| scene | scene | yes | activate → `scene.turn_on` |
| script | script | yes | activate → `script.turn_on`. **Blocked by the default policy.** |
| button, input_button | button | yes | press → `<d>.press` |
| number, input_number | number | yes | set_value → `<d>.set_value` |
| select, input_select | select | yes | select_option → `<d>.select_option` |
| humidifier | humidifier | yes | toggle, set_humidity, set_mode |
| vacuum | vacuum | yes | vacuum_start/pause/return → `vacuum.start/pause/return_to_base` |
| camera | camera | yes | snapshot (camera_proxy) |
| person, device_tracker | presence | yes | read-only; coordinates dropped |
| sun | sun | yes | read-only |
| weather | weather | yes | read-only current state. Forecast deferred: `weather.get_forecasts` needs `return_response`, which `HACallServiceContext` lacks. |
| update | update | yes | read-only. Install deferred (high impact). |
| automation | generic | read-only | toggle deferred: the policy demands an allowlist and it is low value on a dashboard |
| alarm_control_panel | — | **deferred** | needs a code-entry and PIN design; high risk on a wall tablet |
| everything else (input_text, timer, counter, siren, valve, water_heater, lawn_mower, …) | generic | read-only | state text only, so nothing is invisible |

Native id regex: `^[a-z_][a-z0-9_]*\.[a-z0-9_]+$`.

### 8.2 go2rtc (`go2rtc`), effort S
- Entities `go2rtc:<stream_id>` for **enabled configured streams** (`Go2RTCManager.Config().Streams`, `EnabledStreamAlias`).
- Kind camera. Capabilities snapshot and stream.
- State `streaming`/`idle`/`unavailable` from `ListStreams` every 30 s while subscribed. The field mapping of `Go2RTCStreamStatus` is UNVERIFIED.
- Desktop: `media.viewer_path = /api/desktop/integrations/go2rtc/viewer/<id>`, the existing route with its own gates.
- Snapshot: `Go2RTCManager.Snapshot(ctx, id, {Width, Height, Store:false})`. The non-persistent path is required by `internal/server/AGENTS.md:770`.
- Kiosk live: `GET /api/homedash/kiosk/mjpeg?id=go2rtc:x`. After kiosk auth and the allowed-set check, the server calls the **inner** `handleGo2RTCProxy` with a cloned request `/api/go2rtc/proxy/api/stream.mjpeg?src=<id>` (allowed at `go2rtc_handlers.go:535`). The media lease and enabled-stream checks run inside it. `requireGo2RTCView` and the global auth are not touched.
- No device control (go2rtc has none). Talk and detection are out of scope.

### 8.3 FritzBox smart home (`fritzbox`), effort M
- Gates: `fritzbox.enabled && smart_home.enabled && sources.fritzbox.enabled`.
- One `getdevicelistinfos` per poll: default 15 s, minimum 10 s.
- The adapter owns a pooled `fritzbox.NewClientContext(ctx, cfg)` that lives with `Run` and is closed when it stops or on reconfigure (`internal/fritzbox/AGENTS.md:25`).
- Id: `fritzbox:<AIN without spaces>`.

| Device data | Entity |
|---|---|
| SwitchState | switch (toggle → `SetSwitch`) |
| HKR ActualTemp/SetTemp | climate (set_temperature 8–28, step 0.5 → `SetHeatingTarget`; preset off/boost → 0/30) |
| Temperature | sensor °C (separate entity `…:temp`) |
| LampBrightness | light (brightness → `SetLampBrightness`; colour temperature read-only attribute) |
| `Present=false` | `available:false` |
| Blinds | **deferred** (`BlindPosition` is never filled and there is no setter, `service_smarthome.go:94-229`) |

- Control requires `!smart_home.readonly` **and** the matching sub-feature (`switches`/`heating`/`lamps`). HomeDash checks the sub-features itself because the setters do not.
- Names are external, as `service_smarthome.go:17` notes.

### 8.4 MQTT (`mqtt`), effort M
- Entities are **user-defined** in `hd_mqtt_entities`, through the desktop API by session only. The agent cannot create them.
- Kinds: sensor, binary_sensor, switch, number, select, button.
- State topic rules:
  - an exact topic: no `+` or `#`, no NUL, ≤ 256 bytes, not starting with `$`;
  - it must be matched by a configured `mqtt.topics` filter, otherwise status `not_subscribed` with a hint. HomeDash adds no broker subscriptions in v1.
- Value: `mqtt.GetMessages(state_topic, 1)` polled every 1 s while subscribed (in-process, cheap). `json_path` is a dotted path (`a.b.0.c`) with no expression language. Binary payloads → unavailable. Parsed per kind (`payload_on/off` for binary/switch).
- Control: `tools.MQTTPublish(command_topic, payload, qos, retain, logger)` only when `mqtt.enabled && !mqtt.readonly && sources.mqtt.control` and the entity has a `command_topic` (exact topic, same validation). This copies the gate of `dispatch_network.go:179-185`.
- Optional later: a non-blocking observer hook in `storeMQTTMessage` to replace polling. It must respect "receive callbacks only validate, store and enqueue" (`internal/mqtt/AGENTS.md:49`).

### 8.5 Host system (`system`), effort S
- The server injects `func() (tools.SystemMetrics, time.Time)` that reads `storeSystemMetricsSnapshot` (`server_routes.go:137`), so there is no second sampler. Poll every 5 s.
- Entities: `system:cpu`, `system:memory`, `system:disk`, `system:net_rx`, `system:net_tx`, `system:uptime`. All numeric, history, read-only. The exact `SystemMetrics` fields are UNVERIFIED.
- Host temperature: deferred (UNVERIFIED that it exists).

### 8.6 Docker (`docker`), effort M
- Hosts from `DockerHosts.Hosts(ctx)`. Entities `docker:<host_id>/<container name>`, kind service_status. Names are stable across recreates; ids are not.
- State: running|exited|paused|restarting|dead|created. Attributes: health, status text, image (≤ 80 runes; labels never copied).
- Push: `DockerWatch.WatchHost(host, fn)`; events trigger a re-list, debounced 1 s. A safety poll runs every 60 s.
- Control: container_start/stop/restart → `dockerhost.ContainerAction` only when `docker.enabled && !docker.readonly && sources.docker.control` (**default false**).
  - Kiosk-denied.
  - stop/restart are sensitive.
  - Protected owners and AuraGo itself are excluded through a server-injected predicate. Lifecycle actions do not consult protection server-side (`internal/server/AGENTS.md:448`), so this filter is HomeDash's own extra safety.

### 8.7 Proxmox (`proxmox`), effort S
- `tools.ProxmoxInfraResources(ctx, cfg)` every 30 s. Entities `proxmox:<node>/<type>/<vmid>` and `proxmox:<node>`, kind service_status. Attributes cpu, mem and uptime (field names UNVERIFIED).
- **Read-only in v1.** VM power on a wall display is destructive. Infra already has a guarded action ledger with outcomes.

### 8.8 3D printers (`printer3d`), effort M
- Configured printers. `ExecuteThreeDPrinter(ctx, cfg, {Operation:"status", PrinterID})` every 10 s while printing, otherwise every 30 s.
- Kind printer. State printing|paused|idle|error|offline. Attributes progress, nozzle/bed temperature, remaining time, filename (external, ≤ 80 runes). The per-protocol field mapping (Klipper vs Elegoo) is UNVERIFIED.
- Camera: `media.live = "mjpeg"` through the existing `/api/3d-printers/{id}/camera/stream` (desktop). Kiosk uses the `/kiosk/mjpeg` relay calling that handler internally.
- **The Elegoo camera needs an explicit activation write.** HomeDash never activates it and shows "camera inactive".
- **Read-only.** Print control is risky and is a write by SDCP rules.

### 8.9 Uptime Kuma (`uptime_kuma`), effort S
- `tools.FetchUptimeKumaSnapshot` every max(60 s, `uptime_kuma.poll_interval_seconds`).
- Entities `uptime_kuma:<monitor id>` (service_status up/down/pending/maintenance) and `uptime_kuma:summary`. The monitor field names are UNVERIFIED.
- Read-only: the integration itself is read-only.

### 8.10 AdGuard Home (`adguard`), effort S
- Every 30 s: `adguard:queries_today`, `adguard:blocked_today`, `adguard:blocked_ratio` (sensors) and `adguard:filtering` (switch).
- Toggle → `AdGuardFilteringToggle` only when `adguard.enabled && !adguard.readonly && sources.adguard.control` (**default false**). Sensitive. Kiosk allowed only if bound.
- Response JSON shapes are UNVERIFIED.

### 8.11 TrueNAS (`truenas`), effort S
- Every 60 s: `truenas:system` (health), `truenas:pool/<name>` (status, used %), `truenas:alerts` (count).
- The server injects a client factory; the constructor signature is UNVERIFIED.
- Read-only: the storage actions carry destructive gates.

### 8.12 Bluetooth (`bluetooth`), effort S
- `Manager.Snapshot(ctx)`, refreshed on `Manager.Subscribe()` changes (push) plus every 60 s.
- Entities `bluetooth:adapter` (powered, read-only) and `bluetooth:<address>` (presence: connected/paired).
- The address is the id but is never shown as text. Battery is UNVERIFIED.
- Read-only. Connect/disconnect deferred.

### 8.13 Not in v1, with reasons
- **Device registry:** static inventory, no live state, and IP/MAC data is PII. HomeDash gains nothing until it has live state. Effort if wanted later: S.
- **RTL-SDR, RuView, Frigate, CYD (a display sink), Chromecast, MeshCentral (power gate), Grafana (dashboards, not states):** no state model that fits widgets, or control that is unsafe on a wall tablet.

---

## 9. Security model

| Threat | Mitigation |
|---|---|
| **Kiosk link leaks** (chat, screenshot, photo of the QR code) | The link carries a single-use pairing code in the URL fragment (128-bit, TTL 15 min, hashed at rest, 5 attempts per IP per minute, one generic error). The long-lived device token (`hdk_` + 64 hex, SHA-256 at rest, the CYD Creator pattern) lives only in an **HttpOnly SameSite=Strict cookie scoped to `/api/homedash/kiosk/`**, so page script cannot read it. Devices are listed with last seen time and IP, and revoke is one click; streams end within 5 s. Optional device TTL. `kiosk.enabled` master switch (default off). |
| **Kiosk over-reach** | Server-side `AllowedSet` comes from the **approved** revision's widget bindings × the catalog's `Commands`. Catalog, history, snapshot and MJPEG are limited to that set. `view_only` devices. A kiosk-denied list (unlock, container actions). No dashboard editing, no entity browsing, no pairing from kiosk routes. |
| **Agent widens kiosk authority** | Agent saves never advance `approved_revision`. A user must save or approve before wall tablets see new entities. |
| **CSRF** | Desktop routes: existing session CSRF origin check (`auth.go:1031`) plus SameSite=Strict. Kiosk writes: handler-level `checkCSRFOriginWithPolicy(r, true)` (Origin required), JSON content type required, SameSite=Strict cookie. |
| **Replay / double tap** | `request_id` dedupe for 30 s. Pairing codes single-use. Sensitive commands require `confirmed:true` (UX guard, not a boundary). |
| **Command flooding** | Per-principal and per-entity sliding windows. ≤ 4 in flight per source. Latest-wins slot for continuous controls. Stream caps 32 + 16. Subscription cap 300. Body caps. |
| **SSRF** | HomeDash accepts **no URLs**: not in documents, not in MQTT definitions, not in options. Connections only use the integrations' configured, validated endpoints (HA same-origin client; go2rtc only through manager leases). MQTT topics are validated as exact topics; command topics come only from admin-defined entities. |
| **Stored XSS through entity names and titles** | Go strips control characters, bounds lengths and validates icons and colours against strict grammars. The frontend renders untrusted text with `textContent` only (project rule). Kiosk page CSP has no inline script. |
| **Secret leakage through attributes** | Attribute allowlist (drops `entity_picture`, which carries an HA access token, plus `*token*`, `*url*` and GPS coordinates). `scrubSSEJSON` on all frames. Tokens registered with `RegisterSensitive`. Errors are reported as fixed codes, never raw upstream text. |
| **Agent prompt injection** | Entity names, areas, states and dashboard text in tool output are wrapped with `security.IsolateExternalData`. The tool has **no device command operation**. The agent cannot define MQTT entities or pairings. A2A access is read-only. |
| **Stream auth loss** | Re-validation every 5 s ends the stream with `revoked`, as the desktop websocket does (`desktop_handlers.go:252, 270`). |
| **Resource exhaustion from upstream** | HA websocket read limit 16 MiB. Catalog ≤ 2000 entities per source. Snapshot ≤ 8 MiB, never stored. Ring caps. Every source call has a timeout and runs outside hub locks. |

**Confirmation required (server-enforced `confirmed:true`):**
- `unlock`;
- cover `open`/`close`/`set_position` for device_class garage, gate or door;
- `container_stop`, `container_restart`;
- AdGuard filtering off;
- `script.activate`.

**Kiosk page headers** (handler-set, overriding the global ones):
- `Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'self'; frame-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'; object-src 'none'`
- `Referrer-Policy: no-referrer`
- `Cache-Control: no-store`
- `Permissions-Policy: camera=(), microphone=(), geolocation=(), serial=()`

`style-src 'self'` means the engine must style through CSSOM and custom properties, not through inline `style` attributes in markup. This is an open question for the frontend agents.

---

## 10. Agent tool `homedash`

**Gate**

`HomeDashEnabled = cfg.HomeDash.Enabled && cfg.HomeDash.Agent.Access != "off" && cfg.VirtualDesktop.Enabled && homedashToolAvailable()`

`homedashToolAvailable()` is true when the facade is published and available. Wiring follows Storymaker exactly:
- `tooling_policy.go` flag plus runtime check;
- `native_tools_integrations.go` append;
- `native_tools_registry.go` feature key;
- `dispatch_platform.go` case;
- `tool_categories.go` entry;
- `self_test_runtime.go` `{"homedash": {"list_dashboards"}}`;
- `audit.ToolPermissionMatrix` row: `{Name:"homedash", ConfigGate:"homedash.enabled + homedash.agent.access + virtual_desktop.enabled (open: virtual_desktop.allow_agent_control + tools.virtual_desktop.enabled; A2A: read operations only)", ReadOnlyGate:"homedash.agent.access=read, virtual_desktop.readonly", Capabilities: read, write, change}`.

Write operations additionally need `access=write` and `!virtual_desktop.readonly`.

**Schema** (`native_tools_homedash.go`, openai tool):
```
operation: enum [list_dashboards, get_dashboard, list_entities, describe_widgets, validate, create, update, patch, open]
dashboard_id: string            revision: integer (update/patch)
document: object                (create/update/validate; full document, server-owned fields ignored)
ops: array of {op: enum[add_widget, update_widget, remove_widget, move_widget, add_page, rename_page, remove_page, set_name, set_look],
               page_id, widget_id, widget: object, x, y, w, h, name, look: object}
filter: {source, kind, area, query, limit (≤100), offset}
widget_type: string             (describe_widgets: one type in detail; omitted: compact list)
include_layouts: boolean        (get_dashboard; default false keeps output small)
```

**Behaviour**
- `list_entities`: rows `{id, kind, name, area, state_text, unit, controllable}`. `name`, `area` and `state_text` are isolated external data. ≤ 100 rows, state text ≤ 64 runes.
- `describe_widgets`: types, variants, bindings (kinds), option keys with ranges, sizes. No labels.
- `validate`, `create`, `update` and `patch` run the same `Validate` and `Autopack` as HTTP. Issues come back as `[{path, code, message}]` so the model can repair them.
- Saves are recorded with `saved_by="agent"`, keep the revision history (the UI offers "undo agent change" through restore), and never move `approved_revision`.
- No delete. No kiosk pairing or devices. No MQTT definitions. **No commands.**
- `open`: the desktop `open_app` event `{app_id:"homedash", dashboard_id}`. Requires `allow_agent_control && tools.virtual_desktop.enabled`, as `storymakerToolOpen` does (`storymaker_tool.go:225-253`).
- Output is encoded without HTML escaping (as `storymakerToolAnswer`) so the isolation tags stay readable.

**Facade** (published by `internal/server/homedash_agent_facade.go`):
```go
type AgentFacade interface {
	Available() bool
	ListDashboards(ctx context.Context) ([]DashboardSummary, error)
	GetDashboard(ctx context.Context, id string) (Dashboard, error)
	Entities(ctx context.Context, f Filter) ([]Entity, int, error)
	Catalog() WidgetCatalog
	Validate(ctx context.Context, doc Dashboard) Report
	Create(ctx context.Context, doc Dashboard) (Dashboard, Report, error)
	Update(ctx context.Context, doc Dashboard, revision int64) (Dashboard, Report, error)
	Patch(ctx context.Context, id string, revision int64, ops []PatchOp) (Dashboard, Report, error)
}
```

**Also needed**
- `prompts/tools_manuals/homedash.md`: workflow is list_entities → describe_widgets → create/patch → validate. Never invent entity ids.
- Read and follow `internal/agent/AGENTS.md` (Prompt and Runtime Drift Contract).
- The external `antibyte/agotrain` `tool_tiers.json` and `operation_contracts.json` must be updated with paired German/English coverage (root AGENTS.md:146-147). That work happens outside this repo.

---

## 11. Config block

```yaml
homedash:
  enabled: true            # App, hub and API. Nothing connects until a dashboard is open. Default true (like storymaker): read-mostly, commands still pass every integration gate.
  readonly: false          # Blocks every HomeDash command (dashboards stay editable).
  agent:
    access: write          # off | read | write. The agent edits dashboards only; it never sends device commands through HomeDash.
  kiosk:
    enabled: false         # Standalone wall-display page and pairing. Off by default: opens a public route.
    max_devices: 10        # 1..100 paired devices in total
    pairing_ttl_minutes: 15   # 5..60
    device_ttl_days: 0     # 0 = until revoked, else 1..3650
    allow_control: true    # false forces every kiosk device to view-only
  limits:
    max_dashboards: 50        # 1..500
    max_streams: 32           # 4..128 desktop streams
    max_kiosk_streams: 16     # 1..100
    max_subscriptions: 300    # 50..1000 entities per stream
    commands_per_minute: 60   # 10..600 per principal
  history:
    ring_points: 360          # 60..2880 points per entity
    ring_entities: 500        # 50..5000
    sample_seconds: 60        # 10..600
  sources:                    # each also requires the integration's own enabled flag
    home_assistant: { enabled: true, control: true,  poll_seconds: 5 }   # poll only in REST fallback, 2..60
    go2rtc:         { enabled: true,                  poll_seconds: 30 }  # 10..300
    fritzbox:       { enabled: true, control: true,  poll_seconds: 15 }  # 10..300
    mqtt:           { enabled: true, control: true }
    system:         { enabled: true,                  poll_seconds: 5 }   # 2..60
    docker:         { enabled: true, control: false, poll_seconds: 60 }  # 15..600
    proxmox:        { enabled: true,                  poll_seconds: 30 }  # 10..600
    three_d_printers: { enabled: true,                poll_seconds: 30 }  # 10..600 (10 s while printing)
    uptime_kuma:    { enabled: true,                  poll_seconds: 60 }  # 30..600
    adguard:        { enabled: true, control: false, poll_seconds: 30 }  # 15..600
    truenas:        { enabled: true,                  poll_seconds: 60 }  # 30..600
    bluetooth:      { enabled: true }
```

**Go type:** `type HomeDashConfig struct { Enabled, ReadOnly bool; Agent struct{Access string}; Kiosk HomeDashKioskConfig; Limits …; History …; Sources HomeDashSources }`, with `type HomeDashSourceConfig struct { Enabled bool; Control bool; PollSeconds int }`. Sources without control in v1 ignore `Control`.

**Placement**
- `internal/config/homedash.go`: types plus `NormalizeHomeDashConfig`. It applies defaults for zero values, rejects out-of-range values and rejects an unknown `agent.access`. Error format: `homedash: <key> must be lo–hi`, as Chibi Sensei does.
- Field `HomeDash HomeDashConfig \`yaml:"homedash" json:"homedash"\`` in `config_types.go` next to `ChibiSensei` (`:2213`).
- Boolean defaults in `config.go` next to `:602`: enabled, all `sources.*.enabled`, and `control` for ha/fritzbox/mqtt.
- Normalize called at the three Chibi Sensei call sites: `config.go:1136`, `config_handlers_main.go:710`, `config_patch.go:118`.
- `config_template.yaml` block.
- `ui/cfg/homedash.js` plus `ui/lang/config/homedash/{16 locales}.json` plus `ui/lang/meta.json`. Translations are real in every locale; German uses Du.
- Hot reload goes through `replaceConfigSnapshot` → `Hub.Reconfigure`. No restart reason is needed.
- `desktopCapabilities.HasCapability("homedash") = cfg.HomeDash.Enabled && cfg.VirtualDesktop.Enabled`, used by manifest `Requires: ["homedash"]`.

---

## 12. Test plan (essential tests)

**`internal/config`**
- `TestNormalizeHomeDashConfigDefaultsAndRanges`
- `TestHomeDashTemplateNormalizes` (template block passes Normalize, as `chibi_sensei_test.go:115`)
- `TestSQLiteArchivePathsIncludeHomeDash`

**`internal/homedash`** (fakes: `sourcetest.FakeSource` with scripted catalog/updates/command results/panics, recording sink, injected `Now`)
- `TestParseEntityID`
- `TestValidateArgsPerCapability`
- `TestSanitizeBoundsTextAndAttributes`
- `TestWidgetCatalogGolden` (`-update` flag rewrites `testdata/widget_catalog.json`)
- `TestValidateDashboard` (table: limits, unknown type/variant, missing required binding, bad id grammar, overlap, out of bounds, control characters, colour format, navigate to a missing page, unknown entity = warning)
- `TestAutopackFillsMissingLayoutsDeterministically`
- `TestMigrateRejectsNewerSchema`
- `TestApplyPatchOps`
- `TestStoreRevisionConflictAndHistoryCap20`
- `TestStoreApprovedRevisionAdvancesOnlyForUserSaves`
- `TestStoreKioskPairingSingleUseAndExpiry`
- `TestStoreKioskTokensStoredOnlyAsHash`
- `TestStoreDeleteDashboardCascadesDevices`
- `TestHubStartsSourceOnInterestAndStopsAfterIdle`
- `TestHubCoalescesLatestStatePerEntity`
- `TestHubSlowSubscriberDoesNotBlockOthers`
- `TestHubReconfigureRebuildsOnlyChangedSource`
- `TestHubSourcePanicIsIsolatedAndRestarted`
- `TestHubCloseEndsSubscribersWithReason`
- `TestCommandPipelineGates` (table: unknown entity, missing capability, bad args, homedash/desktop readonly, source control off, kiosk outside set, view-only, kiosk-denied unlock, sensitive without confirm, rate limited, success calls audit)
- `TestCommandRequestIDDedupe`
- `TestCommandLatestWinsSlot`
- `TestHistoryRingDownsamplesAndEvictsLRU`

**`internal/homedash/sources/hass`** (fake HA websocket server: `httptest` + gorilla `Upgrader`, scripted `auth_required` / `auth_ok|auth_invalid`, `result` frames, pushes `state_changed`, can drop the connection; fake REST mux)
- `TestHAWebsocketAuthGetStatesAndStateChanged`
- `TestHAWebsocketReconnectBackoffAndResync`
- `TestHAAuthInvalidStopsTightRetry`
- `TestHAFallsBackToRESTPolling`
- `TestHADomainMappingTable`
- `TestHAAttributeAllowlistDropsEntityPictureAndCoordinates`
- `TestHAAreasFromEntityThenDeviceRegistry`
- `TestHACommandsHonourReadonlyAndServicePolicy` (lock/cover blocked without an allowlist)
- `TestHAHistoryBoundsRangeAndPoints`
- `TestHACameraSnapshotSizeCap`

**`internal/tools`**
- `TestHARequestJSONKeepsSameOriginRedirectPolicy`

**Other adapters**
- `fritz`: `TestFritzMappingSwitchHKRLampTemp`, `TestFritzControlRequiresReadonlyOffAndSubFeature` (fake AHA `httptest`)
- `mqttsrc`: `TestJSONPathExtract`, `TestMQTTStateTopicMustBeCoveredByConfiguredFilter`, `TestMQTTPublishGate`
- `docker`: `TestDockerAdapterMapsListAndGatesActions` (reuse the dockerhost fake engine pattern)
- `system`: `TestSystemAdapterMapsSnapshot`
- `kuma`, `adguard`, `proxmox`, `truenas`, `printers`, `bt`: one mapping test each

**`internal/server`**
- `TestHomeDashRoutesRequireDesktopAdmin`
- `TestHomeDashDashboardCRUDAndRevisionConflict`
- `TestHomeDashSubscriptionsWorkOnReadOnlyDesktop`
- `TestHomeDashCommandsRefusedOnReadOnlyDesktop`
- `TestHomeDashStreamHelloSnapshotStateHeartbeat`
- `TestHomeDashStreamEndsOnAuthLoss`
- `TestHomeDashCommandsAreAudited`
- `TestHomeDashKioskPairSetsHttpOnlyScopedCookie`
- `TestHomeDashKioskPairingErrorsAreUniform`
- `TestHomeDashKioskRejectsEntitiesOutsideApprovedRevision`
- `TestHomeDashKioskViewOnlyRefusesCommands`
- `TestHomeDashKioskRevocationEndsStream`
- `TestHomeDashKioskWritesRequireOrigin`
- `TestHomeDashKioskPageCSP`
- `TestHomeDashKioskDisabledHidesRoutes`
- extend `TestAuthBypassMatchesExactPathsOrSegmentSubtrees` with the two new subtrees and their negatives (`/homedash/kioskx`, `/api/homedash/kiosk`)
- `TestHomeDashKioskMJPEGOnlyForAllowedCamera`

**`internal/agent`**
- `TestHomeDashToolHiddenWithoutFacadeOrAccess`
- `TestHomeDashToolReadAccessRefusesWrites`
- `TestHomeDashToolIsolatesEntityText`
- `TestHomeDashToolRevokedOnConfigChange` (pattern `rtl_sdr_test.go:37-50`)
- `TestHomeDashToolA2AReadOnly`

**`internal/audit`**
- the existing manifest tests cover the new route contracts, the tool permission row and the network client entry (`internal/homedash/sources/hass/`: `configured-home-assistant-websocket-same-origin`, `AllowsLocalNet`, `Credentialed`)

**Frontend contract (owned by the frontend agents)**
- a Node test reading `internal/homedash/testdata/widget_catalog.json`

---

## 13. Work packages

Each package compiles, passes `go test` for its packages and is committed on its own. "∥" means it can run in parallel with the packages named. Run GitNexus `impact` before editing every existing symbol listed under "Edits".

| WP | Goal | Files | Contract / tests | Deps | Effort |
|---|---|---|---|---|---|
| **B0** Config | `homedash` block | `config/homedash.go` (+test); edits `config_types.go`, `config.go`, `config_handlers_main.go:710`, `config_patch.go:118`, `config_template.yaml` | §11; config tests | — ∥ B1, B8 | S |
| **B1** Core types | ids, entity, capabilities, sanitize, source interfaces, RuntimeConfig | `homedash/ids.go`, `entity.go`, `capability.go`, `sanitize.go`, `source.go`, `runtime_config.go`, `AGENTS.md` | §4; id/args/sanitize tests | — ∥ B0, B8 | S |
| **B2** Catalog + schema | widget catalog, document types, validate, autopack, migrate, patch | `catalog*.go`, `dashboard*.go`, `testdata/widget_catalog.json` | §7; catalog golden + validation tests. **Freeze point for frontend.** | B1 | M |
| **B3** Store | `homedash.db` | `store*.go`; edit `config/sqlite_paths.go` | §7 schema; store tests | B2 | M |
| **B4a** Hub core | sources runtime, interest, idle, subscribers, coalescing, reconfigure | `hub.go`, `hub_sources.go`, `hub_reconfigure.go`, `subscriber.go`, `sourcetest/` | §5; hub tests | B1 ∥ B2, B3 | M |
| **B4b** Commands + history | pipeline, limiter, dedupe, ring | `command.go`, `ratelimit.go`, `principal.go`, `history.go` | §5; command/history tests | B4a | M |
| **B5** Server wiring + dashboards API | init/close, Reconfigure hook, bootstrap, catalog, dashboard CRUD, revisions, validate, audit contracts, capability `homedash` | `server/homedash_service.go`, `homedash_routes.go`, `homedash_api_dashboards.go`, `homedash_agent_facade.go` (stub `Available`); edits `server.go` (`replaceConfigSnapshot`), `desktop_operation.go`, `bluetooth_live.go` (`HasCapability`), `audit_manifests.go` | §6 dashboard routes; server tests | B0, B3, B4a | M |
| **B6** Entities, commands, stream | entities/entity/history/snapshot, commands, SSE + subscriptions, `system` source | `homedash_api_entities.go`, `homedash_api_commands.go`, `homedash_stream.go`, `homedash_sources.go`, `sources/system/` | §6 stream protocol; stream tests. **Freeze point for frontend engine.** | B4b, B5 | M |
| **B8** HA REST helpers + mapping | `HARequestJSON`/`HAGetBytes`, domain table, attribute allowlist | `tools/homeassistant_raw.go`, `sources/hass/domains.go`, `attributes.go` | §8.1 table; mapping tests | B1 ∥ B0 | S |
| **B9a** HA websocket client | protocol client with fake server | `sources/hass/ws_client.go`, `ws_messages.go` | §8.1; websocket tests | B1 ∥ B2–B6 | M |
| **B9b** HA adapter | catalog/areas, state stream, REST fallback, commands, history, camera | `sources/hass/adapter.go`, `catalog.go`, `commands.go`, `history.go`, `camera.go`, `rest_fallback.go`; register in `homedash_sources.go`; audit network entry | §8.1; adapter tests | B6, B8, B9a | M |
| **B10** go2rtc | camera entities + snapshot | `sources/go2rtcsrc/` | §8.2 | B6 ∥ B9b, B11–B15 | S |
| **B11** FritzBox | smart-home entities + control | `sources/fritz/` | §8.3 | B6 ∥ | M |
| **B12** MQTT | definitions API + adapter | `homedash/store_mqtt.go` (if not in B3), `server/homedash_api_mqtt.go`, `sources/mqttsrc/` | §8.4 | B6 ∥ | M |
| **B13** Docker | containers + gated actions | `sources/docker/` | §8.6 | B6 ∥ | M |
| **B14** Read-only batch | Proxmox, Uptime Kuma, AdGuard (+toggle), TrueNAS | `sources/proxmox/`, `kuma/`, `adguard/`, `truenas/` | §8.7, 8.9–8.11 | B6 ∥ | M |
| **B15** Printers + Bluetooth | — | `sources/printers/`, `sources/bt/` | §8.8, 8.12 | B6 ∥ | S |
| **K1** Kiosk auth | pairing API, device cookie, principal, bypass subtrees, revocation | `homedash_kiosk_auth.go`, desktop pairing/devices routes; edits `auth.go` (`authBypassSubtrees`), `auth_bypass_test.go` | §6 kiosk, §9; kiosk auth tests. **security-agent review.** | B5 | M |
| **K2** Kiosk API + page | kiosk dashboard (approved revision), stream, commands, history, snapshot, page handler + CSP | `homedash_kiosk_api.go`, `homedash_kiosk_page.go`; edit `server_routes_ui.go` | §6, §9; kiosk API tests | K1, B6 | M |
| **K3** Kiosk live camera | MJPEG relay for go2rtc and printers | `homedash_camera.go` | §8.2/8.8; MJPEG test. **security-agent review.** | K2, B10 (B15 for printers) | M |
| **A1** Agent tool | facade impl, schema, dispatch, registration, manifest, manual, self-test | `agent/native_tools_homedash.go`, `homedash_tool.go` (+test); edits `tooling_policy.go`, `native_tools_integrations.go`, `native_tools_registry.go`, `dispatch_platform.go`, `tool_categories.go`, `self_test_runtime.go`, `audit_manifests.go`; `prompts/tools_manuals/homedash.md`; full facade in server | §10; agent tests | B5 (B6 for entities) ∥ K1–K3, B9–B15 | M |
| **D1** Docs | `documentation/homedash.md`, server/root AGENTS sections + index row | docs-agent | — | after each batch | S |

**Batches:**
1. B0 ∥ B1 ∥ B8
2. B2 ∥ B4a ∥ B9a
3. B3, B4b
4. B5
5. B6, which unblocks the frontend engine with real data
6. B9b ∥ B10–B15 ∥ K1 ∥ A1
7. K2
8. K3

Every adapter WP adds one registration line in `homedash_sources.go`. Expect trivial merge conflicts on that file when adapter WPs run in parallel.

---

## 14. Open questions and UNVERIFIED items

**Open questions for the lead**

Answered in section 4.5 of the main document:

1. Default `homedash.agent.access`: proposed `write`, because dashboards are benign and the user asked for agent editing. `read` is the conservative alternative. Decision: write.
2. Kiosk device TTL default: `0` (until revoked) favours wall tablets. A finite TTL is safer but breaks displays silently. Decision: 0, until revoked.
3. Kiosk page `style-src 'self'` vs `'unsafe-inline'` for styles. This depends on how the frontend engine applies geometry. Decision: `style-src 'self'`; the engine uses the CSSOM only.
4. Should `virtual_desktop.readonly` also block **kiosk** commands? Proposed yes, for consistency with HA Switchboard's board readonly. Decision: yes.
5. Does a device revoked because its dashboard was deleted need a user-visible notice on the tablet? Proposed: kiosk page shows "disconnected" on `revoked`. Decision: yes, the kiosk page shows a disconnected state.
6. `/api/desktop/` route contract lists only GET/POST/PUT (`audit_manifests.go:171`). Is the dedicated `/api/desktop/homedash/` contract with DELETE acceptable, or do the audit tests expect something else? Open: verify in B5.

**UNVERIFIED** (not confirmed in code during this session)
- Field shapes of:
  - `Go2RTCStreamStatus`, `tools.SystemMetrics`, `UptimeKumaMonitorSnapshot`;
  - AdGuard status/stats JSON;
  - `ProxmoxInfraResources` items;
  - 3D-printer status JSON per protocol;
  - the TrueNAS client constructor;
  - Bluetooth `Device` fields (battery).
- Exact HA websocket message names and history/camera_proxy endpoints. These are taken from HA's public API, not from this repo.
- Whether `internal/proxy` (managed Caddy) or other reverse proxies buffer SSE. Docker Manager's `X-Accel-Buffering: no` suggests it is handled.
- How `desktopCapabilities.HasCapability` is consumed for `Requires`. Only the answering side was read.
- The registration mechanism of a config UI section beyond the files named (`ui/cfg/*.js`, `ui/lang/config/<section>/`, `meta.json`).
- Whether `decodeJSONRequest` honours a per-route cap argument as described in `internal/server/AGENTS.md:150` (signature at `json_request.go:32` seen, body not read).
- Whether calling the inner `handleGo2RTCProxy` for MJPEG from the kiosk route keeps every lease and revision check. `sameOriginOrNoOrigin` and `AcquireMedia` are inside it (`go2rtc_handlers.go:351, 379`), but this must be confirmed by a test in K3.
- GitNexus impact for: `authBypassSubtrees`/`isAuthBypassed`, `replaceConfigSnapshot`, `desktopRequestOperation`, `SQLiteArchivePaths`, `desktopCapabilities.HasCapability`, `buildToolFlagsFromConfig`, `dispatchPlatform`, `ToolPermissionMatrix`, `RouteContractManifest`, `registerUIRoutes`. The tool was unavailable here.

**Research corrections recorded**
- `docker_manager_api.go:91` is not a rate limiter.
- FritzBox blinds have neither position data nor a setter.
- `research-app-anatomy.md` recommends reusing the HA Switchboard endpoints; see §2.4 for why they are not reused.
