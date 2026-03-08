## Tool: System Metrics (`get_system_metrics`)

Retrieve platform-independent system metrics for monitoring health, diagnosing bottlenecks, or checking resource availability before intensive tasks.

### Schema

```json
{"action": "get_system_metrics"}
```

### Output

Returns a JSON object with:

| Section | Fields |
|---|---|
| `cpu` | Usage percentage, core count, model name |
| `memory` | Total, available, used (bytes), used percentage |
| `disk` | Total, free, used (bytes), used percentage (root partition) |
| `network` | Total bytes sent and received |