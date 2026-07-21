# go2rtc

Use the `go2rtc` tool to observe configured camera streams through AuraGo's managed go2rtc sidecar.

The tool is read-only and accepts only stable `stream_id` values returned by `list_streams`. Never invent or request camera source URLs, credentials, go2rtc API credentials, or arbitrary `src` values.

Operations:

- `status`: Check the managed container and authenticated API state.
- `list_streams`: List enabled configured streams with sanitized reachability, codec, producer, and consumer data.
- `stream_status`: Inspect one configured stream.
- `snapshot`: Create a verified JPEG snapshot and show it in chat; persistent registration follows `store_media`. The result also includes machine-readable artifact provenance and recommends `go2rtc/analyze_snapshot` for a later camera-analysis question.
- `analyze_snapshot`: Store a verified snapshot and analyze it through AuraGo's configured vision provider and budget path.
- `show_live_stream`: Open AuraGo's same-origin viewer. Prefer this path over embedding upstream go2rtc URLs.

Snapshots accept optional `width`, `height`, `rotate`, and `cache_seconds`. Valid rotations are 0, 90, 180, and 270. The tool cannot start or stop the service and cannot add, edit, disable, or remove streams.

For follow-up questions about a camera snapshot, keep using `go2rtc` with `analyze_snapshot` and the trusted `stream_id`. Use `analyze_image` only for an existing general-media artifact. Never try to resolve internal `/files/...` URLs through filesystem, shell, Python, or credential lookup.
