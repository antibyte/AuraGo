# Uptime Kuma (`uptime_kuma`)

Read monitor states from Uptime Kuma using the official Prometheus `/metrics` endpoint.

## Operations

| Operation | Description |
|-----------|-------------|
| `summary` | Return aggregated counts for `up`, `down`, and `unknown` monitors |
| `list_monitors` | Return every exported monitor with current status and response time |
| `get_monitor` | Return a single monitor by its friendly `monitor_name` |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of `summary`, `list_monitors`, `get_monitor` |
| `monitor_name` | string | for get_monitor | Friendly Uptime Kuma monitor name |

## Examples

**Get the overall health summary:**
```json
{"action": "uptime_kuma", "operation": "summary"}
```

**List every monitor:**
```json
{"action": "uptime_kuma", "operation": "list_monitors"}
```

**Inspect one monitor:**
```json
{"action": "uptime_kuma", "operation": "get_monitor", "monitor_name": "Main Website"}
```

## Configuration

```yaml
uptime_kuma:
  enabled: true
  base_url: "https://uptime-kuma.local:3001"
  insecure_ssl: false
  request_timeout: 15
  poll_interval_seconds: 30
  relay_to_agent: false
```

The API key is stored in the encrypted vault under `uptime_kuma_api_key`.

## Notes

- The integration is intentionally read-only for v1
- AuraGo scrapes `/metrics` and parses `monitor_status` plus `monitor_response_time`
- Status changes reported to the agent are limited to `UP -> DOWN` and `DOWN -> UP`
- Unknown states are shown in results but do not trigger agent wake-ups
