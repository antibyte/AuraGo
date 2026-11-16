# Node-RED (`node_red`)

Use an explicitly connected Store instance to inspect flow names and allowed
Inject nodes. Never substitute shell, raw HTTP, deployment or flow editing when
the connection or execution grant is unavailable.

| Operation | Arguments | Result |
|---|---|---|
| `list_flows` | optional `limit` (1–50, default 25), `offset` | Flow IDs/names and administratively allowed Inject IDs/names |
| `trigger` | `node_id` | Acceptance only; the flow may still be running or fail later |

```json
{"action":"node_red","operation":"list_flows"}
```

```json
{"action":"node_red","operation":"trigger","node_id":"inject-backup"}
```

Triggers use the Inject node's saved payload. Payload overrides, flow changes,
runtime restarts and arbitrary nodes are unavailable. An uncertain response must
not be retried automatically. Store stop, update, disconnect and grant revocation
cancel the request; cancellation does not undo an already accepted workflow.

Setup: enable Node-RED admin authentication, create an access token, connect the
Store instance, select Inject IDs and grant execution separately. Tokens stay in
the Vault and are never agent-readable.
