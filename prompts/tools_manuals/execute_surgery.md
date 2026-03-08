## Tool: Execute Surgery (`execute_surgery`) — Maintenance Only

Spawn a specialized Gemini sub-agent to perform strategic code modifications. This is the ONLY tool permitted for code modification during maintenance mode.

Provide a clear, detailed plan in the `task_prompt`. You may also use this tool to ask Gemini questions about the codebase.

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `task_prompt` | string | yes | Detailed description of the code changes or question |

### Example

```json
{"action": "execute_surgery", "task_prompt": "Refactor the logic in internal/server/bridge.go to handle port conflicts more robustly."}
```