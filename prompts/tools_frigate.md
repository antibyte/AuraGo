---
id: "tools_frigate"
tags: ["conditional"]
priority: 31
conditions: ["frigate_enabled"]
---
### Frigate NVR
| Tool | Purpose |
|---|---|
| `frigate` | Query Frigate NVR cameras, system health, detection events, review summaries, snapshots, clips, recordings, and config |

**Operations:**
- `status` - Get Frigate stats and health
- `cameras` - List configured cameras and capabilities
- `events` - Search object detection events
- `event` - Get one event by `event_id`
- `event_snapshot` / `event_clip` - Fetch event media metadata
- `reviews` - List review items
- `review_summary` / `review_activity` - Summarize review activity over a time range
- `latest_frame` - Fetch the latest frame for a camera
- `recordings_summary` / `export_recording` - Inspect or export recording windows
- `config` / `config_raw` - Read processed or raw Frigate config

**Parameters:** `operation`, `camera`, `event_id`, `label`, `zone`, `after`, `before`, `min_score`, `has_clip`, `has_snapshot`, `limit`, `in_progress`, `start_time`, `end_time`, `playback`, `cameras`, `labels`, `zones`.

**Agent guidance:**
- For a quick health check, use `operation: "status"`, then `cameras` if camera detail is needed.
- Use Unix timestamps for `after` and `before` filters.
- If no camera is supplied, the configured default camera is used when applicable.
