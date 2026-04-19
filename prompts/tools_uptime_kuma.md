---
id: "tools_uptime_kuma"
tags: ["conditional"]
priority: 31
conditions: ["uptime_kuma_enabled"]
---
### Uptime Kuma
| Tool | Purpose |
|---|---|
| `uptime_kuma` (operation=`summary`) | Get the total number of monitors that are up, down, or unknown |
| `uptime_kuma` (operation=`list_monitors`) | List all monitors with their target, type, status, and response time |
| `uptime_kuma` (operation=`get_monitor`) | Get the current state of one monitor by its friendly `monitor_name` |

**Notes:**
- This integration is read-only and uses the official `/metrics` endpoint
- Status values are normalized to `up`, `down`, and `unknown`
- `get_monitor` expects the friendly monitor name exactly as it appears in Uptime Kuma
- When a service is reported as `down`, it is usually useful to inspect the target and compare with the latest known response time
