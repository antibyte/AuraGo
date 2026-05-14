# three_d_printer

Use the `three_d_printer` tool to inspect and control configured 3D printers.

Supported protocols:

- Elegoo Centauri Carbon over local SDCP WebSocket.
- Klipper through Moonraker HTTP API standard printer actions.

## Safety

- Respect `three_d_printers.readonly`. When read-only mode is active, do not attempt `start_print`, `pause_print`, `resume_print`, `cancel_print`, or `set_camera_light`.
- Never guess a print filename. For `start_print`, first list files and ask the user to confirm the exact file if it is not explicitly provided.
- Treat camera images as local user data. Only analyze them for the printer-related task the user requested.
- For Klipper, use only standard Moonraker actions. Do not request arbitrary G-code, macros, shell commands, file deletion, restart, update manager, or configuration writes.

## Operations

- `list_printers`: list configured printers and the default printer.
- `test_connection` / `status`: request current printer status.
- `attributes`: fetch printer metadata and capabilities.
- `files`: list G-code files. Elegoo accepts optional `directory`, default `/local`; Klipper lists Moonraker `gcodes`.
- `history`: fetch print history IDs.
- `camera_url`: get the printer camera stream URL. Klipper uses Moonraker webcam entries and may select a configured webcam name.
- `camera_snapshot`: capture, store, and register a snapshot in AuraGo's media registry when available.
- `analyze_camera`: capture, store, and register a snapshot, then analyze it with the configured Vision provider.
- `show_live_stream`: return the proxied MJPEG or browser-compatible image stream. Set `show_in_chat` to true when the stream should also be rendered inline in chat.
- `start_print`: start an explicit `filename`. Elegoo also supports `start_layer`, `calibration`, and `time_lapse`; Klipper sends the filename to Moonraker unchanged.
- `pause_print`, `resume_print`, `cancel_print`: control the active job when write access is allowed.
- `set_camera_light`: set `light_on` true or false for Elegoo when write access is allowed. Klipper v1 does not support light control.

## Examples

```json
{"operation":"status","printer_id":"lab-printer"}
```

```json
{"operation":"analyze_camera","printer_id":"lab-printer","prompt":"Check whether the print is still adhering to the bed."}
```

```json
{"operation":"start_print","printer_id":"lab-printer","filename":"/local/bracket.gcode","calibration":true}
```
