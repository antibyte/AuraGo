# Execute Surgery (`execute_surgery`) — Maintenance Only

Spawn a specialized Gemini sub-agent to perform strategic code modifications. This is the **ONLY tool permitted** for code modification during maintenance mode.

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `task_prompt` | string | yes | Detailed description of the code changes or question |

## Example

```json
{"action": "execute_surgery", "task_prompt": "Refactor the logic in internal/server/bridge.go to handle port conflicts more robustly."}
```

## Notes

- **Maintenance mode only**: This tool is only available during maintenance mode for code modifications
- **Detailed prompts**: Provide clear, detailed task prompts for best results
- **Use case**: Strategic code refactoring and targeted modifications
```