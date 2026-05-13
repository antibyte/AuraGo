# three_d_printer

Use the `three_d_printer` tool to inspect and control configured 3D printers. The first supported protocol is Elegoo Centauri Carbon over local SDCP WebSocket.

## Safety

- Respect `three_d_printers.readonly`. When read-only mode is active, do not attempt `start_print`, `pause_print`, `resume_print`, `cancel_print`, or `set_camera_light`.
- Never guess a print filename. For `start_print`, first list files and ask the user to confirm the exact file if it is not explicitly provided.
- Treat camera images as local user data. Only analyze them for the printer-related task the user requested.

## Operations

- `list_printers`: list configured printers and the default printer.
- `test_connection` / `status`: request current printer status.
- `attributes`: fetch printer metadata and capabilities.
- `files`: list G-code files. Optional `directory`, default `/local`.
- `history`: fetch print history IDs.
- `camera_url`: get the printer camera stream URL.
- `camera_snapshot`: capture and store a snapshot in AuraGo.
- `analyze_camera`: capture a snapshot and analyze it with the Vision provider.
- `show_live_stream`: show the proxied MJPEG stream in chat when browser-compatible.
- `start_print`: start an explicit `filename`; optionally pass `start_layer`, `calibration`, and `time_lapse`.
- `pause_print`, `resume_print`, `cancel_print`: control the active job when write access is allowed.
- `set_camera_light`: set `light_on` true or false when write access is allowed.

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
