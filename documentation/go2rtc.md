# go2rtc Integration

AuraGo can manage [go2rtc](https://github.com/AlexxIT/go2rtc) as an optional Docker sidecar for secure camera viewing and snapshots. The integration is disabled by default and manages only its own container.

## Security model

- The reviewed default image is `alexxit/go2rtc:1.9.14@sha256:675c318b23c06fd862a61d262240c9a63436b4050d177ffc68a32710d9e05bae`.
- The go2rtc API and Web UI are never bound directly to the LAN. Native AuraGo binds API port 1984 only to `127.0.0.1`; Docker deployments use an internal network alias.
- Camera source URLs and the internal go2rtc API password are stored only in AuraGo's encrypted Vault. Generated `data/go2rtc/go2rtc.yaml` contains no camera sources or plaintext password.
- Sources are injected through go2rtc's runtime stream API after startup. Only `rtsp`, `rtsps`, `rtspx`, `http`, `https`, and `onvif` network URLs are accepted.
- The container runs as a non-root user with a read-only root filesystem, no capabilities, `no-new-privileges`, a restricted `/tmp`, and CPU, memory, and PID limits.
- AuraGo's proxy removes caller cookies and authorization headers, applies its own internal Basic authentication, and restricts media access to configured, enabled stream IDs.
- Credential-bearing ONVIF SOAP requests use a direct private-network transport and never honor `HTTP_PROXY` or `HTTPS_PROXY` from the environment.
- Upstream go2rtc logging is disabled because producer warnings can contain runtime source URLs. AuraGo exposes sanitized lifecycle and stream status instead.
- Snapshot memory is bounded to 16 entries and 64 MiB. Stored snapshots retain at most 1,000 files or 2 GiB and remove the oldest files first.

## Proxy and WebRTC

The stable viewer contract is `/api/go2rtc/viewer/{stream_id}`. MSE, HLS, MP4, MJPEG, WebSocket, and optional WHEP traffic pass through `/api/go2rtc/proxy/`.

WebRTC media normally flows directly between browser and go2rtc and cannot be carried completely through an HTTP reverse proxy. Direct LAN WebRTC is therefore disabled by default. Enabling it requires a concrete private LAN bind/candidate IP and publishes port 8555 over both TCP and UDP. RTSP port 8554 is never published on the host.

The built-in Virtual Desktop app **Network Cameras** uses this same-origin viewer and proxy boundary without exposing camera URLs or go2rtc credentials to the browser. Open it from the launcher or directly with `/desktop?app=network-cameras`.

## Virtual Desktop app and camera setup

The **Network Cameras** app combines a five-second snapshot grid with one selected live view. Its optional live-grid mode opens at most four viewers at once; hidden, minimized, and closed windows stop polling and tear down their media frames.

Administrators can enable the managed sidecar, discover ONVIF cameras, add a known local ONVIF address, or enter a supported stream URL. Other users with `go2rtc.view` see enabled cameras only. Camera management remains administrator-only.

Automatic ONVIF discovery is capability gated. AuraGo sends one bounded WS-Discovery probe per suitable private IPv4 interface only when `Runtime.BroadcastOK` is available. A normal Docker bridge does not provide reliable LAN multicast discovery, so the app offers manual IP and stream-URL setup without granting host networking or extra privileges.

After AuraGo has selected and validated a concrete private camera IP, its bounded setup client queries ONVIF device information, the media service, media profiles, and snapshot capability directly. The returned profile data can include codec, resolution, frame rate, manufacturer, and model when the camera supplies them. Direct SOAP traffic never uses environment proxies, remains restricted to the selected private host, and does not expose credentials or source URLs to the browser. go2rtc receives only the selected Vault-backed source during normal runtime reconciliation.

Discovery candidates and credential-bearing setup tokens are random, memory-only, limited, single-use, and expire after five minutes. A setup token is reserved during publication, released after any pre-publication failure, and consumed only after the desired configuration has been published.

The app consumes these AuraGo-owned endpoints:

- `GET /api/go2rtc/app/state` for sanitized capability and stream telemetry
- `GET /api/go2rtc/thumbnail/{stream_id}.jpg` for non-persistent JPEG tiles with private caching and ETags
- `POST /api/go2rtc/setup/enable`, `/api/go2rtc/discovery`, and `/api/go2rtc/discovery/profiles` for administrator setup
- `POST /api/go2rtc/streams` plus `PATCH` or `DELETE /api/go2rtc/streams/{stream_id}` for administrator-managed streams

The profile response reports `probe_backend: "direct"`. It includes `snapshot_available` only when the camera explicitly returns a valid ONVIF snapshot-capability flag.

The complete profile operation is limited to 20 seconds. Snapshot-capability detection is optional and receives at most five seconds within that budget. Every SOAP response is limited to 1 MiB.

The raw upstream `api/onvif` endpoint remains blocked for browsers, agents, and the optional administrator Web UI, and AuraGo does not call it during setup. The pinned go2rtc 1.9.14 ONVIF client does not provide the request cancellation and per-camera response limits required at this trust boundary. Camera credentials and returned source URLs are never exposed through AuraGo's API.

### Mutation and recovery contract

Camera create, update, delete, and integration-enable requests publish a fully loaded and validated candidate configuration before replacing the active YAML. Vault changes are rolled back if validation or publication fails. Once published, the YAML/Vault state remains AuraGo's desired state even if Docker or go2rtc is temporarily unavailable; the background manager retries reconciliation.

- Complete create returns HTTP `201`; complete update, delete, and enable return HTTP `200`.
- A published change whose runtime reconciliation failed returns HTTP `202` with `status: "degraded"`, `saved: true`, and `runtime_reconciled: false`. The response includes only the safe saved stream state or remaining stream count.
- Invalid input returns `400`, a missing stream `404`, a duplicate ID `409`, and an unmet integration prerequisite `412`. Config or Vault publication failures return `500` and `saved: false`.

Before enabling the integration, AuraGo checks the live Docker contract with bounded requests. Container and image listing must be readable, network listing must also be readable when AuraGo itself runs in Docker, and a POST to start a cryptographically random nonexistent sentinel container must return `404`. The probe never creates a container. Missing capabilities are reported with `docker_unreachable`, `docker_containers_denied`, `docker_images_denied`, `docker_networks_denied`, or `docker_post_denied`.

## Agent operations

When the integration, agent access, and authenticated API are all available, AuraGo advertises a `go2rtc` tool with:

- `status`
- `list_streams`
- `stream_status`
- `snapshot`
- `analyze_snapshot`
- `show_live_stream`
- `detection_status`, `set_arm_mode`, `list_detection_events`, `analyze_detection_event`, `show_detection_event` (person detection; only while `go2rtc.detection.enabled` is on)

The agent accepts stable stream IDs and detection event IDs only and cannot start or stop the service or modify stream or detection configuration; `set_arm_mode` (`off`, `on`, `schedule`) is its only state change and is recorded in the arm log with the actor `agent`. Autonomous runs (camera wake-ups, cron, heartbeat, relay messages) may only arm (`on`); `off` and `schedule` answer `not_allowed`. Mission runs are not covered by this gate and can still switch the mode to `off` or `schedule`. Snapshot and detection-event analysis use AuraGo's existing Vision provider and budget accounting.

Snapshots stored under `data/go2rtc/snapshots` are prepared as bounded private Vision inputs; only the private copies are removed after analysis. General `analyze_image` calls can use the exact local or `/files/...` path of an existing image in the Media Registry. Neither path grants general agent access to the data directory; unregistered paths and root-escaping symlinks remain blocked.

Stream `reachable` telemetry is based on current codec metadata, not an active camera probe. Idle streams may report `false` with no codecs or consumers and still produce a snapshot on demand. The agent receives this distinction with stream status results; request a snapshot to verify image availability.

## Person detection

The **Network Cameras** app can detect people in the camera images on the AuraGo server itself. A D-FINE model runs in a separate worker process (`aurago --person-detect-worker`) on the GPU or iGPU when possible (ONNX Runtime WebGPU) and on the CPU otherwise; images never leave the server for detection.

### How it works

- For every enabled camera with person detection on, AuraGo takes a frame through go2rtc every `interval_seconds` (1–30 s, default 2, 1280 px wide). One shared worker analyzes the newest frame of each camera in turn, so slow hardware lowers the effective rate instead of building a queue. A camera whose frames fail is shown as unreachable and retried with growing pauses of up to 60 s; it raises no alerts.
- A **hit** is a person box with at least `min_score` (30–95 %, default 50 %) whose **foot point** (bottom centre of the box) lies inside one of the camera's zones. Without zones the whole image counts. Stored frames keep only the boxes that count.
- **Limits of the foot point:** it assumes the box ends where the person stands. Ceiling or top-down cameras see no feet, and a partly hidden person (behind a car or a hedge) gets a box that ends at the hip, so the foot point lands at hip level and may miss a ground zone or hit one behind it. *Test* in the camera settings shows the effect on the current image; draw zones with that in mind.
- **Zones** are polygons with 3–32 points (coordinates 0–1); a camera has up to 8, with unique names of 1–48 characters. A zone covers everything its outline encloses (nonzero winding rule), also where the outline crosses itself. Points near a frame edge snap onto it — in the editor within 1.5 % of the frame, on the server within 1 % — so a person cut off by the frame still counts. The editor warns about a zone that covers less than 0.01 % of the image or whose points lie on one line ("too small or flat"), and trims a zone name only when you edit it.
- An **event** starts with two hits within the last three analyzed frames or one hit of at least 80 %. It ends when no hit came for `max(30 s, 2 × interval + 5 s)` (30 s for intervals up to 12.5 s), or at once when the server clock steps back. AuraGo keeps the frames that started it and then at most one frame every 5 seconds (20 per event); the highest-scoring frame is the cover.
- **Arming:** `Off`, `On` or `Schedule`. Detection and storage always run for enabled cameras; the message, agent and desktop alerts fire only while the system is armed. The alert pause `cooldown_seconds` (0–86,400, default 300) counts per camera **and** per action from that action's last *sent* alert; a failed or skipped alert does not start it, and a restart resets the pauses. The arm mode is runtime state in `detections.db`, not configuration: it survives a restart, and a new database starts with `Off`. Every change is logged with who made it (user, agent, schedule) and shown in the app's timeline.
- **Schedule:** a list of weekday windows in the **server's time zone**; a window that ends before it starts runs past midnight, and one whose start equals its end lasts 24 hours. Switching is instant-based: a repeated autumn hour switches only once, and a spring time that does not exist switches at the jump. Docker images run in UTC unless `TZ` is set — uncomment `- TZ=Europe/Berlin` (or your zone) in the `environment` block of `docker-compose.yml`; the image ships the time zone database. The app shows the server zone next to the schedule (a UTC server reads "UTC") and in the arm status when it differs from the browser's.
- **Alerts** per camera: a desktop notice; a message with photo through the chosen notification channel (Telegram and Discord receive the cover frame with the boxes drawn in); waking the agent with an optional instruction of at most 500 characters; and the mission/flow trigger `person_detected`. The trigger fires at every event start, armed or not, with `armed` in its data and without the camera's alert pause: missions react to armed events only unless `person_include_disarmed` is set, and their own `min_interval_seconds` applies. Each event records the result per action (`sent`, `failed: …`, `skipped: …`).
- **Agent and arming:** an agent run woken by a detection, like other autonomous runs, may arm (`set_arm_mode` `on`) but not disarm: `off` and `schedule` answer `not_allowed`. Mission runs are not covered by this gate.

### Models, runtime and GPU

| Model | Use | Download |
|---|---|---|
| Standard — D-FINE-N | default; finds most people | about 15 MB |
| Accurate — D-FINE-S | also small and distant people; about 2.5 times the computing time | about 41 MB |

Plus the ONNX Runtime: about 9 MB for the CPU (shared with local embeddings) or 25 MB for the GPU (WebGPU; Linux x86-64, Windows x64, macOS arm64). Downloads are pinned by size and SHA-256 and stored under `<data_dir>/embeddings/runtimes` and `<data_dir>/persondetect/models`; nothing half-downloaded is used.

Backend `auto` (default) tries the GPU first — on Linux only when a `/dev/dri/renderD*` node is accessible — and keeps it only if a self-test on a reference image gives the same detections as the CPU and uses at most half of its CPU time (this rejects software renderers). Otherwise AuraGo runs on the CPU and the app shows the reason; with backend `gpu` the status is "degraded". On Linux, give the service user access to the GPU and restart AuraGo:

```bash
sudo usermod -aG render aurago   # the user AuraGo runs as
sudo systemctl restart aurago
```

### go2rtc container

go2rtc decodes the frames, so while at least one camera detects people its container gets 0.5 + 0.25 CPUs per detecting camera (at most 2, and never more than the host has) and 512 MB of memory. Switching detection on or off for a camera changes these limits, and a **running** go2rtc container is recreated with them: live views pause for a few seconds. A **stopped** go2rtc stays stopped — detection does not start it; its detection cameras report frame errors ("unreachable") until go2rtc is started again.

### Storage

Events are stored in `<data_dir>/go2rtc/detections.db` and as JPEG files with 320 px thumbnails under `<data_dir>/go2rtc/detections/YYYY/MM/DD/<camera>/`. Retention runs at start and hourly: events that ended more than `retention_days` (default 14) ago are deleted, and while the total exceeds `max_storage_mb` (default 2048) the oldest events go until usage is below 90 %. A full disk triggers an immediate run and a warning in the app. Detection data is not part of the media registry or of `.ago` backups. Deleting a camera keeps its events until retention removes them. While detection is switched off, stored events stay visible and deletable, and retention keeps running.

A corrupt `detections.db` is moved aside as `detections.db.corrupt-<timestamp>` and replaced by a fresh database (arm mode `Off`). The images of the old database then move to `detections.orphaned-<timestamp>` next to the `detections` folder (into `detections/orphaned-<timestamp>` when that folder is a symlink or a mount point). AuraGo deletes neither; remove them once you no longer need them.

### Using the app

- **Detection mode** (Snapshots | Live grid | Detection): a status strip with the arm control (administrators) and its state, for example "Armed (schedule) until 06:00"; the detector (model, GPU or CPU, runtime, time per image, download progress, fallback reason, errors with *Try again*); one chip per camera with its analyses per minute. Below it the timeline of events and arm changes, filtered by camera and day and loaded page by page, and the event detail with the frames and their boxes (← and → switch images), the alert results and, for administrators, *Delete event*. A click on a person detection notice (the toast or its entry in the notification center) opens the app in this mode on that event, filtered to its camera, also when the app is already open. An event that was deleted meanwhile (by retention, the storage limit or another window) disappears from the timeline when it is opened, with a short notice.
- **Camera settings → Person detection** (administrators): enable, interval, minimum score, alert pause, the alert actions, and the zone editor on the current camera image — click to set points; double-click or Enter closes a zone, Esc discards it, Backspace removes the last point; the zone list renames, selects and deletes zones; drag the points of the selected zone. Saving closes a zone that is still being drawn (one with fewer than 3 points blocks the save with a hint). *Test* analyzes the current frame with the unsaved settings and shows every box: green ones count, dashed ones are ignored. While go2rtc is running, switching detection on or off for a camera recreates its container, so live views pause for a few seconds (the tab says so next to the switch).
- **Settings** (administrators): master switch, model, backend, retention, maximum storage and the arming schedule (with the server's time zone).

### Configuration of person detection

Global settings live in `go2rtc.detection`, camera settings in `go2rtc.streams[].detection`; both are normally edited in the app. The config page keeps no detection fields and points to the app.

```yaml
go2rtc:
  detection:
    enabled: false
    model: standard          # standard | accurate
    backend: auto            # auto | gpu | cpu
    retention_days: 14       # 1–365
    max_storage_mb: 2048     # 100–1,000,000
    schedule:                # used while arming is "schedule"; server time zone
      - days: [mon, tue, wed, thu, fri]
        from: "22:00"
        to: "06:00"
  streams:
    - id: front-door
      name: Front door
      enabled: true
      detection:
        enabled: true
        interval_seconds: 2
        min_score: 0.5
        cooldown_seconds: 300
        zones:
          - name: Driveway
            points: [[0.1, 0.55], [0.9, 0.55], [0.9, 1.0], [0.1, 1.0]]
        actions:
          message: true
          channel: telegram  # ntfy, pushover, telegram, discord, push, telnyx, cyd or all
          agent: false
          agent_instruction: ""
          desktop: true
```

### Person detection API

Base `/api/go2rtc/detection/`, also reachable as `/api/desktop/integrations/go2rtc/detection/`. Reads need `go2rtc.view`; writes need an administrator and a same-origin request (otherwise 403 `forbidden`). Errors are `{"error", "code"}`. Lists are always JSON arrays, never `null`; times are UTC RFC 3339 with millisecond precision; event, frame and arm-log ids are large int64 numbers (around 1.8 × 10¹², still safe JavaScript integers — never parse them through a float path that rounds).

| Method | Path | Answer |
|---|---|---|
| GET | `state?since=<cursor>` | status (detector, arming, cameras, storage) plus events and arm-log rows; without `since` only the status |
| GET | `events?stream_id&from&to&before&limit` | `{events, next_before}`, at most 100 per page |
| GET, DELETE | `events/{id}` | the event with frames and boxes; DELETE removes it with its files (`event_not_found` 404) |
| GET | `images/{frame_id}[?thumb=1]` | the stored JPEG or its thumbnail (`frame_not_found` 404) |
| POST | `arm` `{mode}` | arm state; `off`, `on`, `schedule` (`detection_disabled` 409 while detection is off) |
| PUT | `settings` | replaces `go2rtc.detection`; 400 names the invalid `field` |
| POST | `test` `{stream_id, detection?}` | analyzes one current frame and returns it with all boxes, each marked whether it counts (`detection_disabled` 409, `detector_not_ready` 409, `frame_unavailable` 502) |
| POST | `retry` | leaves the error state and sets the detector up again |

`state?since=0` is a snapshot: the newest events and arm-log rows (at most 200 each) and the cursor of the status. A higher cursor returns the rows changed after it, in pages of about 200. The feed also answers while detection is off, as long as the database is open; without a database it returns the status with empty lists and cursor 0. It reports no deletions: a client that gets 404 `event_not_found` for a listed event drops it. 503 `detection_unavailable` means detection is on but its database is not open (yet). Deleting events works while detection is off.

Camera settings are saved with `PATCH /api/go2rtc/streams/{id}` and a `detection` object, which is **merged**: absent keys keep their stored values, `zones` replaces the whole list, and `null` or unknown keys are rejected (400 `invalid_request` with `field`). `PUT settings` instead **replaces** the global block, so send all fields (body at most 64 KiB). `GET /api/go2rtc/app/state` returns the camera settings per stream and the stored global settings as `detection_settings` (with defaults filled in and `schedule` always a list), both to administrators only.

### Agent operations for person detection

With agent access, the `go2rtc` tool also offers `detection_status`, `set_arm_mode` (`mode`), `list_detection_events` (`stream_id`, `since`, `until`, `limit` ≤ 50), `analyze_detection_event` (`event_id`, `prompt`) and `show_detection_event` (`event_id`); see [Agent operations](#agent-operations) for the arming rules. Arm changes by the agent appear in the timeline with the actor "agent".

## Configuration

Configure the integration under **Config → Network & Remote → go2rtc Cameras**, or use the administrator onboarding in **Network Cameras**. Connection tests, container controls, snapshots, and viewers intentionally use only the last saved configuration.

The optional administrator Web UI is a read-only subset of the original go2rtc interface. Configuration, logs, publishing, process control, and stream mutation routes remain blocked.

## License

go2rtc is Copyright AlexxIT and contributors and is distributed under the MIT License. AuraGo does not modify or rebuild the upstream image; it pulls the pinned published image at runtime. Review the upstream [license](https://github.com/AlexxIT/go2rtc/blob/v1.9.14/LICENSE) and [release notes](https://github.com/AlexxIT/go2rtc/releases/tag/v1.9.14) before overriding the pinned image.
