# System Metrics (`get_system_metrics`)

Retrieve platform-independent system metrics for monitoring health, diagnosing bottlenecks, or checking resource availability before intensive tasks.

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| — | — | — | No parameters needed |

## Examples

```json
{"action": "get_system_metrics"}
```

## Output

Returns a JSON object with:

| Section | Fields |
|---------|--------|
| `cpu` | Usage percentage, core count, model name |
| `memory` | Total, available, used (bytes), used percentage |
| `disk` | Total, free, used (bytes), used percentage (root partition) |
| `network` | Total bytes sent and received |

## Notes

- **Platform-independent**: Works on Linux, macOS, and Windows without modification.
- **Use cases**: Check resource availability before heavy tasks (builds, ML inference, large file processing).
- **Memory reporting**: `used` includes buffers and cache on Linux; actual application memory may be lower.
- **Disk metrics**: Reports root partition only. For specific volumes, use `execute_shell` with `df` or `Get-PSDrive`.