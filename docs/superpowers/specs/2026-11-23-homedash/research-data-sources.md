READ-ONLY RESEARCH REPORT: AuraGo device, sensor and video sources for HomeDash. Nothing was modified. All paths are absolute under C:\Users\Andi\Documents\repo\AuraGo\ (abbreviated below as ROOT\ only in the table note; full paths are given in the sections). Items marked UNVERIFIED were not confirmed by reading code.

## Summary table

| Source | Enabled flag | UI endpoints | Control possible | Live push | Video |
|---|---|---|---|---|---|
| Home Assistant | home_assistant.enabled | Yes: /api/desktop/home-assistant/{entities,states,switch} (switch domain only) | Yes: switch on/off only | No (UI polls 5 s; backend REST poll 15 s, missions only) | No |
| go2rtc (RTSP/cameras, Tuya cloud cams) | go2rtc.enabled | Yes: /api/go2rtc/* (status, streams, snapshot, viewer, proxy) | No device control; stream CRUD, talk, detection | No (media is WebSocket/MSE inside viewer) | Yes |
| MQTT | mqtt.enabled | Pull only: /api/mqtt/status, /messages | Agent tool only (no UI publish route found) | No (in-memory buffer, pull) | No |
| Frigate | frigate.enabled | Test only: /api/frigate/test | Agent only | Events go to agent via MQTT relay | UNVERIFIED (no browser route found) |
| FritzBox (TR-064 + AHA smart home) | fritzbox.enabled (+ smart_home.enabled) | Status/test only | Agent tool fritzbox_smarthome | No (telephony poller to agent) | TV stream URLs UNVERIFIED |
| Docker | docker.enabled | /api/containers, /api/docker-manager/ | Yes (readonly flag) | container_update SSE type exists, publisher UNVERIFIED | No |
| Proxmox | proxmox.enabled | Test only; data via Infra snapshot | Agent only (allow_destructive gate) | Infra poll | No |
| Bluetooth | bluetooth.enabled | /api/bluetooth/* (admin) | Yes (power, device actions, audio) | Yes: desktop hub bluetooth_changed | No |
| RTL-SDR | rtl_sdr.enabled | /api/desktop/rtl-sdr/* | Tune/scan/record | None found | No (audio) |
| RuView (WiFi sensing) | ruview.enabled | /api/ruview/* (admin) | Setup/firmware | UNVERIFIED | Opaque UI iframe only |
| 3D printers (Elegoo CC, Klipper) | three_d_printers.enabled | /api/3d-printers/status, /{id}/camera/stream | UNVERIFIED | UNVERIFIED | Yes (camera stream) |
| TrueNAS | truenas (enabled flag UNVERIFIED) | /api/truenas/* (admin) | Pools/datasets/shares CRUD | None | No |
| AdGuard Home | adguard.enabled | /api/adguard/status, /test | Agent (readonly gate) | No | No |
| Uptime Kuma | uptime_kuma.enabled | /api/uptime-kuma/status, /test | Read only | No | No |
| Grafana | grafana.enabled | /api/grafana/status, /test | No | No | No |
| MeshCentral | meshcentral.enabled | Test only | Agent (power gated) | No | No |
| Chromecast | chromecast.enabled | /api/chromecast/discover, /test (admin) | UNVERIFIED | No | No |
| CYD ESP32 displays | cyd.enabled | /api/cyd/* incl. WS | Agent (allow_agent_control) | Device WS | No |
| Host system metrics | none | /api/dashboard/system | n/a | SSE system_metrics every 10 s | No |
| Device registry | none | /api/devices CRUD | Edit only | No | No |

## Sources

**1. Home Assistant**
a) Config: home_assistant.enabled, .readonly, .url, .allowed_services, .blocked_services; access_token is vault-only. Struct at C:\Users\Andi\Documents\repo\AuraGo\internal\config\config_types.go:1675-1682. Gates: readonly, the desktop board ReadOnly flag, and HAServiceAllowed allow/deny lists (C:\Users\Andi\Documents\repo\AuraGo\internal\tools\homeassistant.go:249-258).
b) Package internal\tools (homeassistant.go). REST calls: HAGetStatesContext (l.83, GET /api/states), HAGetState (l.149), HACallServiceContext (l.184, POST /api/services/{domain}/{service}), HAListServices (l.298). The desktop board only uses the switch domain.
c) Endpoints (registered C:\Users\Andi\Documents\repo\AuraGo\internal\server\server_routes.go:582-584, handler desktop_homeassistant_handlers.go:49-156):
- GET /api/desktop/home-assistant/entities: all switch entities (catalog).
- GET /api/desktop/home-assistant/states: only switches selected on the board.
- POST /api/desktop/home-assistant/switch with body {"entity_id","state":"on"|"off"}.
Scope: desktop ADMIN, including reads (desktop_homeassistant_handlers.go:58). Response shape: {ready, readonly, board_readonly, can_on, can_off, board, entities:[{entity_id, friendly_name, state}], checked_at}. Write returns {accepted:true}. Errors: setup_required, invalid_switch (400), switch_not_allowed (403), switch_unavailable (409), ha_unavailable and switch_failed (502). Admin test route: POST /api/home-assistant/test (server_routes_config.go:133).
d) Live updates: none. No websocket and no state_changed subscription. The UI polls every 5 s (ui\js\desktop\apps\ha-switchboard.js:375). The backend poller (internal\tools\homeassistant_poller.go:11-40) is REST, 15 s ticker, only for entities referenced by mission triggers, started from internal\server\homeassistant_runtime.go:30-55.
e) Video: none.

**2. go2rtc (cameras, RTSP, Tuya cloud cameras)**
a) Config: go2rtc.enabled, auto_start, web_ui_enabled, agent_access, store_media, streams[] (id, name, enabled, playback_mode, talk_enabled, detection), tuya_accounts, webrtc.* in C:\Users\Andi\Documents\repo\AuraGo\internal\config\config_types.go:398-440. Scope go2rtc.view; mutations are admin.
b) Package internal\tools: Go2RTCManager (go2rtc.go:114), Status (l.440), ListStreams (l.536), Snapshot (l.594), EnabledStreamAlias (l.952). AcquireMedia (internal\tools\go2rtc_media.go:145), HLSLease (l.534). Sidecar in internal\tools\go2rtc_sidecar. Capabilities: list, snapshot, stream CRUD, two-way talk, person detection. No device control.
c) Endpoints (C:\Users\Andi\Documents\repo\AuraGo\internal\server\go2rtc_handlers.go:27-50):
- GET /api/go2rtc/streams (view): {status, streams:[...]}; POST creates (admin).
- GET /api/go2rtc/status (admin).
- POST /api/go2rtc/snapshot (view, same-origin): body {stream_id,width,height,rotate,cache_seconds,store}.
- GET /api/go2rtc/viewer/{id}?role=&muted= (view): returns an HTML page with a CSP, muted by default (go2rtc_handlers.go:247-322).
- /api/go2rtc/proxy/ (view): go2rtc API passthrough, including api/ws?src= (go2rtc_handlers.go:324).
- /api/go2rtc/thumbnail/ (view); /api/go2rtc/tuya/* (admin, l.40-43); /api/go2rtc/detection/* (go2rtc_detection_handlers.go:22).
- Desktop mirror: /api/desktop/integrations/go2rtc/* (root whitelisted at C:\Users\Andi\Documents\repo\AuraGo\internal\server\desktop_integration_routes.go:16).
d) Live: none at backend level for state. The video itself streams.
e) Video: the desktop cameras app builds iframe src /api/desktop/integrations/go2rtc/viewer/{id} (ui\js\desktop\apps\network-cameras-media.js:62, applied at l.119). The viewer loads ui\js\go2rtc-viewer.js:17 (WebSocket) and l.85 (/api/go2rtc/proxy/api/ws?src=...). Auth is requireGo2RTCView (go2rtc_handlers.go:53-84): bearer go2rtc.view or admin, or session. Tuya cloud import: C:\Users\Andi\Documents\repo\AuraGo\internal\config\go2rtc_tuya.go:12.
UNVERIFIED: whether AcquireMedia rejects unconfigured src values (go2rtc_handlers.go:379 passes raw src).

**3. MQTT**
a) Config: mqtt.enabled, readonly, broker, topics, qos, relay_to_agent, relay_topics, relay_grants, allow_untrusted_transport, allow_unauthenticated_relay, tls, buffer, availability (C:\Users\Andi\Documents\repo\AuraGo\internal\config\config_types.go:2320-2340).
b) Packages internal\mqtt (client.go, controller.go, buffer.go, subscriptions_runtime.go, frigate_relay.go). Publish and subscribe are agent-side.
c) Endpoints (C:\Users\Andi\Documents\repo\AuraGo\internal\server\server_routes_config.go:274-276, handlers mqtt_handlers.go): GET /api/mqtt/status (l.13; state, subscriptions, buffer length, password source), POST /api/mqtt/test (l.84), GET /api/mqtt/messages?topic= (l.139-165; {status, messages, count}, limit 50). Session-gated, not admin-wrapped. No UI publish route found.
d) Live: pull from an in-memory buffer only.
e) Video: none.

**4. Frigate**
a) Config: frigate.enabled, readonly, url, api_token (vault), internal_port, default_camera, event_relay, review_relay, store_media, mqtt_topic_prefix (C:\Users\Andi\Documents\repo\AuraGo\internal\config\config_types.go:2080-2092). Struct: C:\Users\Andi\Documents\repo\AuraGo\internal\tools\frigate.go:27.
b) Agent-side tool; event relay via internal\mqtt\frigate_relay.go. Person detection in internal\server\person_detection.go.
c) Only POST /api/frigate/test (server_routes_config.go:258, admin).
d) Events go to the agent through MQTT, not the browser.
e) UNVERIFIED: no browser stream or snapshot route found.

**5. FritzBox (TR-064, AHA smart home, telephony, storage, TV)**
a) Config: fritzbox.enabled plus feature blocks (system, network, telephony, smart_home, storage, tv), each with enabled and readonly (C:\Users\Andi\Documents\repo\AuraGo\internal\config\config_types.go:1683-1769; smart_home at l.1737-1748: devices, switches, heating, blinds, lamps). Password vault-only.
b) Package internal\fritzbox: client.go, tr064_client.go, aha_client.go (smart home), poller.go, service_*.go. Agent tool "fritzbox_smarthome" (internal\server\python_tool_bridge_handler.go:43).
c) GET /api/fritzbox/status (fritzbox_handlers.go:16; server_routes_config.go:279), POST /api/fritzbox/test (l.88; route l.280). No UI smart-home list endpoint found.
d) Telephony poller feeds the agent, not the browser.
e) TV stream URLs (tv.stream_urls): UNVERIFIED route.

**6. Docker**
a) Config: docker.enabled, readonly, host, allow_host_access (C:\Users\Andi\Documents\repo\AuraGo\internal\config\config_types.go:1803-1808).
b) Handlers: containers_handlers.go:27 (list), :48 (action); docker_manager_api.go:194 (manager). dockerwatch hub: C:\Users\Andi\Documents\repo\AuraGo\internal\dockerwatch\hub.go:19.
c) GET /api/containers and /api/containers/{id}/... (server_routes_tools.go:146-147), /api/docker-manager/ (l.150). The desktop root "containers" is whitelisted (desktop_integration_routes.go:15); docker-manager is excluded on purpose.
d) SSE event constant container_update (sse.go:27). The publisher is UNVERIFIED.
e) None.

**7. Proxmox**
a) Config: proxmox.enabled, readonly, allow_destructive, url, token_id, secret (vault), node, insecure (config_types.go:2070-2079).
b) tools/proxmox.go; Infra refresh at C:\Users\Andi\Documents\repo\AuraGo\internal\server\desktop_infra_sources.go:155-230.
c) POST /api/proxmox/test only (server_routes_config.go:134, admin). Data reaches the UI via the Infra snapshot.
d) Pull, TTL-based refresh.
e) None.

**8. Bluetooth**
a) Config: bluetooth (C:\Users\Andi\Documents\repo\AuraGo\internal\config\config_types.go:1995; struct at l.1116). Dashboard flag requires Enabled && Runtime.Bluetooth.Usable.
b) Package internal\bluetooth: manager.go, operations.go, headset_*.go, bus_linux.go.
c) All admin (server_routes_config.go:162-171): /api/bluetooth/{status, reprobe, discover, devices/action, audio/test, audio/stop, power, discovery, discoverable, interactions/}. Desktop root "bluetooth" whitelisted.
d) Yes: C:\Users\Andi\Documents\repo\AuraGo\internal\server\bluetooth_live.go:70-75 publishes bluetooth_changed and bluetooth_interaction through broadcastDesktopEvent. Change source (BlueZ signal) UNVERIFIED.
e) None.

**9. RTL-SDR**
a) Config: rtl_sdr.enabled, read_only, allow_agent, device, quota_gb (C:\Users\Andi\Documents\repo\AuraGo\internal\config\rtl_sdr.go:12-18).
b) Package internal\rtlsdr; adapters in internal\server\rtl_sdr_adapters.go; agent tool in internal\agent\native_tools_rtl_sdr.go:11.
c) /api/desktop/rtl-sdr/* routed to Server.handleRTLSDR (C:\Users\Andi\Documents\repo\AuraGo\internal\server\rtl_sdr_handlers.go:38-60; config via requireAdmin). Operations: scan, tune, stop, recordings. Not a whitelisted integration root.
d) None found.
e) Audio recordings only. No 433 MHz sensor decoder found (UNVERIFIED that none exists elsewhere).

**10. RuView (WiFi sensing)**
a) Config: ruview.enabled, readonly, agent_access, mode, source, url, http_port (default 3022), udp_* and storage_mb (C:\Users\Andi\Documents\repo\AuraGo\internal\config\ruview.go:13-32).
b) Package internal\ruview (client.go, nodes.go, read.go, proxy.go, runtime.go). Pose streaming UNVERIFIED.
c) /api/ruview/ (requireAdmin, C:\Users\Andi\Documents\repo\AuraGo\internal\server\ruview_handlers.go:99-108) proxies to the sidecar. /api/ruview/ui/ serves an opaque UI (auth-exempt). Desktop root "ruview" whitelisted.
d) UNVERIFIED.
e) Only the opaque UI iframe.

**11. 3D printers (Elegoo Centauri Carbon and Klipper/Moonraker)**
a) Config keys: three_d_printers.enabled, readonly, default_printer, elegoo_centauri_carbon.{enabled,printers}, klipper.{enabled,printers} (keys at C:\Users\Andi\Documents\repo\AuraGo\internal\config\config.go:3235-3241; types at config_types.go:955-1013).
b) Client package UNVERIFIED.
c) GET /api/3d-printers/status (server_routes_config.go:260, not admin-wrapped), POST /api/3d-printers/test (l.259, admin), /api/3d-printers/ (l.261, handler UNVERIFIED), GET /api/3d-printers/{id}/camera/stream (auth bypass with media or desktop ticket, C:\Users\Andi\Documents\repo\AuraGo\internal\server\auth.go:340). Desktop root "3d-printers" whitelisted.
d) UNVERIFIED.
e) Yes: camera stream route; format UNVERIFIED.

**12. TrueNAS**
a) Config: truenas (C:\Users\Andi\Documents\repo\AuraGo\internal\config\config_types.go:2687); enabled flag UNVERIFIED. Package internal\truenas (client.go).
c) C:\Users\Andi\Documents\repo\AuraGo\internal\server\truenas_handlers.go:81-100: /api/truenas/{status, health, pools, pools/{id}, datasets, datasets/{id}, snapshots, snapshots/{id}, shares/smb, shares/nfs}. Test at server_routes_config.go:137 (admin).
d) None found. e) None.

**13. AdGuard Home**: config adguard.{enabled, readonly, url, username, password (vault)} at config_types.go:2311-2318. Routes /api/adguard/status (server_routes_config.go:242; handler internal\server\adguard_handlers.go:11) and /api/adguard/test (l.243). Agent tool internal\tools\adguard.go. No live push.

**14. Uptime Kuma**: config uptime_kuma (config_types.go:2690). Read-only monitor states via Prometheus metrics (agent tool, native_tools_integrations.go:1710). Routes /api/uptime-kuma/status and /test (server_routes_config.go:249-250).

**15. Grafana**: config grafana (config_types.go:2693), internal\tools\grafana.go. Routes /api/grafana/status and /test (server_routes_config.go:251-252). Dashboards, not device state.

**16. MeshCentral**: config meshcentral.* (config_types.go:1791-1802; allow_power_actions gate). Route /api/meshcentral/test only (server_routes_config.go:119, admin).

**17. Chromecast**: config chromecast (config_types.go:1993; tools/chromecast.go:20). Routes GET /api/chromecast/discover and POST /api/chromecast/test (server_routes_config.go:157-158, admin). Control path UNVERIFIED.

**18. CYD (ESP32 displays)**: config cyd.{enabled, poll_seconds, overlay_ttl_seconds, allow_agent_control, mqtt_mirror} (config_types.go:2342-2348). Hub: internal\cyd\hub.go:36. Routes /api/cyd/{snapshot, heartbeat, ack, speak/, persona, ws, status, test, firmware/...} (cyd_handlers.go:775-785). The server refreshes the device snapshot every 10 s (server_routes.go:152-154). This is a display sink, not a source.

**19. Host system metrics (no config flag)**: collected by tools.GetSystemMetrics every 10 s (C:\Users\Andi\Documents\repo\AuraGo\internal\server\server_routes.go:124-150), pushed as SSE system_metrics with payload {available, cpu, memory, disk, network, sse_clients, uptime_seconds, host_uptime_seconds}. Pull endpoint GET /api/dashboard/system (server_routes_config.go:478; handler dashboard_handlers_overview.go:35). Host temperature UNVERIFIED.

**20. Device registry**: /api/devices (GET, POST) and /api/devices/{id} (GET, PUT, DELETE) (server_routes_config.go:334-354; C:\Users\Andi\Documents\repo\AuraGo\internal\server\device_handlers.go). Static inventory with no live state.

**21. Wyoming (voice)**: internal\tools\wyoming.go, a voice protocol client. Not a device-state source. Config key UNVERIFIED.

## Not present (grep-verified)
Shelly, Philips Hue, Zigbee2MQTT, Homematic, KNX, ESPHome, Tasmota, UniFi, Netatmo, OctoPrint, Pi-hole, Netdata, Sonos, Govee, Meross, UPS (NUT/apcupsd), energy/solar inverters (Victron, Modbus, SMA, Enphase), weather (open-meteo appears only in test fixtures). Brand names occur only in test fixtures (internal\infra\atlaspdf\fixture_test.go), ecalc catalog data and bundled apps. Tuya smart-home devices are not present; Tuya exists only as go2rtc cloud-camera import. Prometheus appears only as a datasource type for Grafana and Uptime Kuma.

## f) Aggregators
- Desktop Infra: C:\Users\Andi\Documents\repo\AuraGo\internal\server\desktop_infra.go:22,56-57 (base path /api/desktop/infra, snapshot). Source kinds in C:\Users\Andi\Documents\repo\AuraGo\internal\server\desktop_infra_sources.go: registry (l.47), docker (l.147), proxmox (l.228), fritzbox (l.278). Refresh TTL via desktopInfraRefreshOne (l.286). Store package: internal\infra (not read). Home Assistant and MQTT are NOT Infra sources.
- Dashboard flags map (see g).
- SysWorld facts (internal\server\desktop_sysworld_facts.go:73-79) expose integrations as config-only entities.
- No unified live device/entity model exists. HomeDash would need one.

## g) Enabled flags
GET /api/dashboard/overview (server_routes_config.go:518; handler dashboard_handlers_overview.go:626, integrations map at l.648 and l.871) returns {"integrations": {"home_assistant": true, "go2rtc": true, "mqtt": false, ...}}. Keys come from C:\Users\Andi\Documents\repo\AuraGo\internal\server\dashboard_integrations.go:10-83, which states it reports configuration, not health. Per-source health uses each source's status route. HA desktop payload also returns ready, readonly, can_on, can_off.

## h) Security conventions
- Global auth: authMiddleware (C:\Users\Andi\Documents\repo\AuraGo\internal\server\auth.go:936). Lockdown when auth is enabled without password (l.952). Auth disabled: host check (l.972) and origin check on writes (l.981). Session or bearer otherwise.
- Admin wrapper: requireAdmin (auth.go:453), requireAdminUnlessGET (l.440). Several status routes are session-only, not admin: /api/mqtt/status, /api/fritzbox/status, /api/adguard/status, /api/dashboard/overview, /api/3d-printers/status.
- Desktop scopes and operations: requireDesktopPermission (C:\Users\Andi\Documents\repo\AuraGo\internal\server\desktop_auth.go:23); operation map (desktop_operation.go:25-52; desktop_integration_routes.go:60-86). The HA desktop route requires admin scope even for reads.
- go2rtc: go2rtc.view scope (go2rtc_handlers.go:21, 53-84).
- CSRF/origin: sameOriginOrNoOrigin (C:\Users\Andi\Documents\repo\AuraGo\internal\server\websocket_origin.go:8), used on go2rtc snapshot and proxy (go2rtc_handlers.go:216, 351). Global CSRF token for logged-in mutations: UNVERIFIED.
- Read-only: per-source readonly flags (HA config_types.go:1678; ruview.go:15; rtl_sdr.go:14; docker, etc.), plus desktop board ReadOnly (desktop_homeassistant_handlers.go:78).
- Audit: svc.Audit("ha_switch", ...) (desktop_homeassistant_handlers.go:140); route contract manifest with auth classes at C:\Users\Andi\Documents\repo\AuraGo\internal\audit\audit_manifests.go:169,188.
- Rate limiting: no limiter on the HA switch or go2rtc. Limiters exist for WebAuthn start (internal\server\auth_webauthn.go:188), detective notify (detective_notify.go:20), docker manager (docker_manager_api.go:91), MQTT trigger interval (config_types.go:2336), FritzBox callbacks (config_types.go:1733).
- SSRF: internal\security\ssrf.go exists; its application to HA and go2rtc URLs is UNVERIFIED.
- Scrubbing: security.Scrub on SSE JSON (sse.go:63, 74) and MQTT errors (mqtt_handlers.go:33).
- HA service policy: allowed and blocked service lists (tools\homeassistant.go:249-258).

## i) Event channels
- desktop.Hub (C:\Users\Andi\Documents\repo\AuraGo\internal\desktop\hub.go:10-78): default cap 8 clients (l.18-20), 64-message per-client buffer, non-blocking Broadcast that drops on full. Exposed as WebSocket /api/desktop/ws (server_routes.go:601; handleDesktopWS in C:\Users\Andi\Documents\repo\AuraGo\internal\server\desktop_handlers.go:199-287, scope read). Frames: {id, type, payload, created_at}. Types include welcome, desktop_policy, pong, desktop_changed, bluetooth_changed, plant_changed, pet_changed.
- Global SSE: SSEBroadcaster (C:\Users\Andi\Documents\repo\AuraGo\internal\server\sse.go:48-143), route /events (server_routes.go:501). Envelope {"type","payload"}. Type names are snake_case constants at sse.go:18-44 (system_metrics, container_update, virtual_desktop_event, ...).
- Publisher example: server_routes.go:150 sse.BroadcastType(EventSystemMetrics, payload). Desktop fan-out: broadcastDesktopEvent (desktop_handlers.go:308-318) sends to the hub and to SSE as virtual_desktop_event.
- Subscriber example: ui\js\chat\chat-streaming.js:158-257 uses window.AuraSSE.on('llm_stream_delta', fn). AuraSSE's definition file was not located (UNVERIFIED).
- For HomeDash: add a new SSEEventType constant in sse.go, publish via BroadcastType, or send through broadcastDesktopEvent with a new type string.

## Unverified (not confirmed by reading code)
AuraSSE client file path; the desktop WebSocket consumer path; 3D printer client package and control routes; RuView live push and pose streaming; Bluetooth event source; dockerwatch wiring; Frigate browser stream; SSRF guard inside AcquireMedia and HA calls; FritzBox AHA control routes; TrueNAS enabled flag; Chromecast control path; host temperature; Wyoming config key; 433 MHz decoding; CSRF token for logged-in mutations; MQTT publish path.
