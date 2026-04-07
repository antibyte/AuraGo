# Initiate Handover (`initiate_handover`) — Supervisor Only

Trigger a transition to Maintenance (Lifeboat) mode. Optionally pass a summary of the planned maintenance work to the sidecar.

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `task_prompt` | string | no | Summary of the planned maintenance work |

## Examples

```json
{"action": "initiate_handover"}
```

```json
{"action": "initiate_handover", "task_prompt": "Summary of plan..."}
```

## Notes

- **Supervisor only**: This tool is only available in the main supervisor context
- **Lifeboat mode**: Transitions AuraGo to maintenance mode for code modifications
- **If already in Lifeboat**: Use `exit_lifeboat` to return instead
```