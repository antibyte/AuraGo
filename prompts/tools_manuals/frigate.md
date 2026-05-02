# Frigate NVR Tool (`frigate`)

Query Frigate NVR cameras, object detection events, review summaries, snapshots, clips, recordings, and configuration.

| Operation | Description | Key parameters |
|---|---|---|
| `status` / `health` | Frigate stats and health check via `/api/stats` | none |
| `cameras` | Configured cameras and capabilities | none |
| `events` | Search detection events | `camera`, `label`, `zone`, `after`, `before`, `min_score`, `has_clip`, `has_snapshot`, `limit`, `offset` |
| `event` | Get one event | `event_id` |
| `event_snapshot` | Fetch and, when `frigate.store_media` is enabled, store an event snapshot | `event_id` |
| `event_clip` | Fetch and, when `frigate.store_media` is enabled, store an event clip | `event_id` |
| `reviews` | List review items | `camera`, `after`, `before`, `limit`, `offset`, `in_progress` |
| `review_summary` | Review summary | `after`, `before`, `cameras`, `labels`, `zones` |
| `review_activity` | Motion/audio activity over time | `after`, `before`, `cameras`, `in_progress` |
| `latest_frame` | Fetch and, when `frigate.store_media` is enabled, store the latest camera frame | `camera` |
| `recordings_summary` | Recording availability | `camera`, `start_time`, `end_time` |
| `export_recording` | Export and, when `frigate.store_media` is enabled, store a recording segment | `camera`, `start_time`, `end_time`, `playback` |
| `config` | Read processed config | none |
| `config_raw` | Read raw config | none |

Examples:

```json
{"action":"frigate","operation":"status"}
```

```json
{"action":"frigate","operation":"events","camera":"doorbell","label":"person","after":1767225600,"limit":20}
```

```json
{"action":"frigate","operation":"review_summary","after":1767225600,"before":1767312000,"cameras":"doorbell,garage","labels":"person,car"}
```

Notes:
- Current Frigate tool operations are read-only against the Frigate API. Media operations may write fetched files into AuraGo's local data directory when `frigate.store_media` is enabled.
- `default_camera` is used for camera-specific queries when `camera` is omitted.
- Time filters use Unix seconds. `start_time` and `end_time` may use values accepted by the Frigate API.
- `offset` paginates `events` and `reviews` together with `limit`.
- Stored media responses include `local_path`, `web_path`, `sha256`, and `media_id` when the media registry is available.
- `event_relay` and `review_relay` subscribe to Frigate MQTT event/review topics when MQTT is enabled.
