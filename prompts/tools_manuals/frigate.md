# Frigate NVR Tool (`frigate`)

Query Frigate NVR cameras, object detection events, review summaries, snapshots, clips, recordings, and configuration.

| Operation | Description | Key parameters |
|---|---|---|
| `status` | Cameras, stats, and health | none |
| `cameras` | Configured cameras and capabilities | none |
| `events` | Search detection events | `camera`, `label`, `zone`, `after`, `before`, `min_score`, `has_clip`, `has_snapshot`, `limit` |
| `event` | Get one event | `event_id` |
| `event_snapshot` | Fetch event snapshot metadata | `event_id` |
| `event_clip` | Fetch event clip metadata | `event_id` |
| `reviews` | List review items | `camera`, `after`, `before`, `limit`, `in_progress` |
| `review_summary` | Review summary | `after`, `before`, `cameras`, `labels`, `zones` |
| `review_activity` | Motion/audio activity over time | `after`, `before`, `cameras`, `in_progress` |
| `latest_frame` | Fetch latest camera frame metadata | `camera` |
| `recordings_summary` | Recording availability | `camera`, `start_time`, `end_time` |
| `export_recording` | Export recording segment metadata | `camera`, `start_time`, `end_time`, `playback` |
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
- `frigate.readonly` blocks future mutating operations such as event deletion or config saving.
- `default_camera` is used for camera-specific queries when `camera` is omitted.
- Time filters use Unix seconds. `start_time` and `end_time` may use values accepted by the Frigate API.
