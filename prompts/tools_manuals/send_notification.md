## Tool: Notification Center (`send_notification`)

Send push notifications to one or more channels. Ideal for alerts, reminders, and autonomous task completion reports.

### Schema

```json
{
  "action": "send_notification",
  "channel": "ntfy",
  "title": "Backup Complete",
  "message": "Daily backup finished successfully. 3.2 GB archived.",
  "tag": "normal"
}
```

### Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `channel` | yes | Target: `"ntfy"`, `"pushover"`, `"telegram"`, `"discord"`, `"push"`, `"telnyx"`, or `"all"` |
| `title`   | no  | Notification title (default: "AuraGo") |
| `message` | yes | Notification body text |
| `tag`     | no  | Priority: `"low"`, `"normal"` (default), `"high"`, `"critical"` |

### Channels

- **ntfy** — Open-source push service (ntfy.sh or self-hosted). Configured in `config.yaml` under `notifications.ntfy`.
- **pushover** — Commercial push service. Credentials stored in vault.
- **telegram** — Uses existing Telegram bot config (`telegram.bot_token` + `telegram_user_id`).
- **discord** — Uses existing Discord bot config (`discord.default_channel_id`).
- **push** — Web Push / PWA Notification. Native background notification to all browsers that have installed the app.
- **telnyx** — SMS notification via Telnyx. Sends to the first number in `telnyx.allowed_numbers`. Requires Telnyx integration enabled.
- **all** — Sends to all enabled channels simultaneously.

### Examples

**Single channel:**
```json
{"action": "send_notification", "channel": "ntfy", "title": "Server Alert", "message": "CPU usage at 95%", "tag": "critical"}
```

**All channels:**
```json
{"action": "send_notification", "channel": "all", "title": "Task Done", "message": "Scheduled report generated."}
```
