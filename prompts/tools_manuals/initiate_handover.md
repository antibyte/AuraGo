## Tool: Initiate Handover (`initiate_handover`) — Supervisor Only

Trigger a transition to Maintenance (Lifeboat) mode. Optionally pass a summary of your plan to the sidecar.

> If you are already in the Lifeboat, use `exit_lifeboat` to return instead.

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `task_prompt` | string | no | Summary of the planned maintenance work |

### Examples

```json
{"action": "initiate_handover"}
```

```json
{"action": "initiate_handover", "task_prompt": "Summary of plan..."}
```