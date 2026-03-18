---
tool: site_monitor
version: 1
tags: ["conditional"]
conditions: ["web_scraper_enabled"]
---

# Site Monitor Tool

Monitor websites for content changes using content hashing. Track when pages change and view change history.

## Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `add_monitor` | Register a URL for monitoring | `url` |
| `remove_monitor` | Remove a monitor | `monitor_id` |
| `list_monitors` | List all configured monitors | — |
| `check_now` | Check a specific monitor or URL for changes | `monitor_id` or `url` |
| `check_all` | Check all monitors at once | — |
| `get_history` | View change history for a monitor | `monitor_id` |

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | string | **Required.** Operation to perform |
| `url` | string | URL to monitor (http/https) |
| `monitor_id` | string | Monitor ID (returned by add_monitor) |
| `selector` | string | CSS selector to focus on specific page content |
| `interval` | string | Suggested check interval (informational, e.g. "every 6 hours") |
| `limit` | integer | Max history entries to return (default: 20, max: 100) |

## Cron Integration

To schedule automatic checks, combine with the `manage_schedule` tool:

```
1. Add a monitor: site_monitor with operation=add_monitor, url=https://example.com
2. Create a cron job: manage_schedule with operation=add, cron_expr="0 */6 * * *",
   task_prompt="Check all site monitors for changes using site_monitor check_all and report any changes"
```

## Examples

Add a monitor:
```json
{"operation": "add_monitor", "url": "https://example.com/pricing"}
```

Check for changes:
```json
{"operation": "check_now", "monitor_id": "mon_abc12345"}
```

Check all monitors:
```json
{"operation": "check_all"}
```

View change history:
```json
{"operation": "get_history", "monitor_id": "mon_abc12345", "limit": 10}
```

## How It Works

1. When a URL is added, the tool fetches the page content and computes a SHA-256 hash
2. On subsequent checks, it fetches again and compares hashes
3. If the hash differs, a change is recorded with timestamp and content preview
4. All data is stored in a local SQLite database

## Notes

- Content is extracted as plain text (HTML tags stripped)
- Changes are detected by content hash comparison (not visual)
- The `check_all` operation checks every registered monitor sequentially
- Monitor IDs are deterministic based on URL + selector
