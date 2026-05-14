# Klipper 3D Printer Integration Design

## Context

AuraGo already has a `three_d_printers` integration for Elegoo Centauri Carbon printers. The next protocol should add Klipper printers through Moonraker, while keeping the same agent-facing `three_d_printer` tool, the same read-only safety model, and the same authenticated camera proxy and snapshot analysis flow.

Moonraker is the supported Klipper API surface for this integration. v1 intentionally uses only standard Moonraker printer actions. It does not expose arbitrary G-code, macros, shell commands, file deletion, or printer configuration writes.

References:

- Moonraker Remote API: https://moonraker.readthedocs.io/en/stable/web_api/
- Moonraker Printer API: https://moonraker.readthedocs.io/en/latest/external_api/printer/
- Moonraker Webcam API: https://moonraker.readthedocs.io/en/latest/external_api/webcams/

## Goals

- Add Klipper as a second protocol under `three_d_printers`.
- Let the agent inspect Klipper printer state, files, print history, and camera availability.
- Let the agent perform standard print actions when read-only mode is disabled.
- Reuse the existing camera snapshot, analysis, and live-stream renderer behavior.
- Keep the implementation safe for Docker installs by making connectivity testable from inside AuraGo.

## Non-Goals

- No arbitrary macro execution in v1.
- No raw `printer.gcode.script` endpoint in v1.
- No file upload, file delete, config edit, restart, firmware restart, update manager, or host shell access.
- No video transcoding. Browser-compatible streams and snapshots are proxied; unsupported streams return a clear fallback.

## Configuration

Add a Klipper subsection under the existing `three_d_printers` root:

```yaml
three_d_printers:
  enabled: true
  readonly: true
  default_printer: "voron"
  klipper:
    enabled: false
    printers:
      - id: "voron"
        name: "Voron 2.4"
        url: "http://192.168.6.60:7125"
        api_key: ""
        timeout_seconds: 10
        webcam_name: ""
```

`api_key` is optional because many local Moonraker setups use trusted clients. If populated, AuraGo will send it as a Moonraker API key header. The field must be treated as sensitive in UI and server-side handling.

`webcam_name` is optional. When empty, AuraGo selects the first enabled webcam returned by Moonraker.

## Agent Tool Behavior

The existing native tool remains `three_d_printer`. It routes by configured printer ID and protocol.

Supported operations for Klipper v1:

- `list_printers`
- `test_connection`
- `status`
- `attributes`
- `files`
- `history`
- `camera_url`
- `camera_snapshot`
- `analyze_camera`
- `show_live_stream`
- `start_print`
- `pause_print`
- `resume_print`
- `cancel_print`

Read-only mode blocks all mutating operations: `start_print`, `pause_print`, `resume_print`, and `cancel_print`. `start_print` still requires an explicit filename; the agent must never guess which file to print.

## Moonraker API Mapping

- `test_connection`: request server info or printer object list with the configured timeout.
- `status`: query common objects via `POST /printer/objects/query`, including `webhooks`, `print_stats`, `toolhead`, `extruder`, `heater_bed`, `display_status`, and `virtual_sdcard`.
- `attributes`: list available printer objects via `GET /printer/objects/list`.
- `files`: list printable files via `GET /server/files/list?root=gcodes`.
- `history`: use Moonraker history list when available; return a helpful unavailable response if history is disabled.
- `start_print`: `POST /printer/print/start?filename=<filename>`.
- `pause_print`: `POST /printer/print/pause`.
- `resume_print`: `POST /printer/print/resume`.
- `cancel_print`: `POST /printer/print/cancel`.
- `camera_url`: use `GET /server/webcams/list`, selecting by `webcam_name` when configured.

## Camera Flow

Klipper webcam data comes from Moonraker's webcam list. AuraGo should prefer a configured webcam's `snapshot_url` for snapshots and `stream_url` for live view.

URL handling:

- Relative webcam URLs are resolved against the Moonraker base URL.
- Proxying is allowed only for HTTP or HTTPS URLs.
- The stream or snapshot host must match the configured Moonraker host. A different port is allowed because Crowsnest and Mainsail setups often serve video separately.
- If the returned camera URL cannot be proxied, the tool returns a clear fallback instead of silently failing.

Snapshots are stored in the existing 3D printer media folder and remain exposed through the existing authenticated media flow.

## UI

The 3D Printers config page gets a Klipper subsection beside Elegoo Centauri Carbon:

- Enable Klipper toggle.
- Add/remove Klipper printer entries.
- Fields: ID, display name, Moonraker URL, optional API key, timeout, optional webcam name.
- Test connection button for each Klipper printer.
- Existing global read-only toggle remains shared across protocols.
- Add translations for all 15 supported UI languages.

The API key field must be rendered as a secret field and must not leak into agent-visible tool output.

## Error Handling

- Disabled integration: return a configuration error that names `three_d_printers.enabled` or `three_d_printers.klipper.enabled`.
- Missing printer: return the requested ID and mention the configured IDs.
- Timeout: return the configured URL and timeout without exposing secrets.
- Moonraker authorization failure: return a concise authentication error and point to the API key field.
- Webcam unavailable: return a status that makes it clear printer control may still work.
- Read-only block: return a hard error before making any HTTP request.

## Testing

Add tests with a mock Moonraker HTTP server:

- Config load/save for `three_d_printers.klipper.printers`, including numeric-keyed UI patch conversion.
- `test_connection`, `status`, `attributes`, `files`, and `history` request paths and response parsing.
- Standard print actions route to Moonraker only when read-only is disabled.
- Read-only blocks mutating operations without contacting the server.
- `start_print` rejects empty filenames.
- Webcam selection uses `webcam_name` when provided and first enabled webcam otherwise.
- Camera proxy accepts same host with different port and rejects unrelated hosts.
- Snapshot analysis reuses the existing image analysis path with a stored mock snapshot.
- UI regression confirms the Klipper section registers without `alert()` and all translation keys exist.

## Open Decisions

No open product decision remains for v1. The user chose standard Moonraker actions only; macro support can be planned later as an explicit allowlist feature.
