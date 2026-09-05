## Tool: Cheap Yellow Display (`cyd_display`)

Control a paired ESP32 Cheap Yellow Display mini-dashboard.

### Schema

```json
{
  "action": "cyd_display",
  "operation": "notify",
  "title": "Backup failed",
  "message": "disk /data 98%",
  "priority": "critical"
}
```

### Operations

| Operation | Description |
|-----------|-------------|
| `notify` | Full-screen overlay. Requires `message`. |
| `show` | Pin a short status line on page 1. Requires `message`. |
| `clear` | Drop overlay and pinned status. |
| `page` | Switch to `status`, `home`, `load`, `work`, or `host` (requires `cyd.allow_agent_control`). |
| `brightness` | Set backlight 0–255 (requires `cyd.allow_agent_control`). |
| `led` | Set RGB LED: `off`, `green`, `yellow`, `red`, `blue`. |
| `status` | List connected displays and the current snapshot. |

Use `send_notification` with `channel: "cyd"` for the same overlay path as ntfy/Telegram.
