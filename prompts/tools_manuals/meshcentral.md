---
conditions: ["meshcentral_enabled"]
---
# MeshCentral

The `meshcentral` tool interacts with devices managed by a MeshCentral server through the MeshCentral control WebSocket. It can inspect server state, list groups and devices, read device details and events, wake devices, run MeshAgent commands, and send supported power actions.

**Warning:** `wake`, `power_action`, and `run_command` are privileged operations. Verify the target `node_id` or `mesh_id` before changing remote device state.

## Parameters

- **`operation`**: Required. One of:
  - `"server_info"`: Returns MeshCentral server metadata.
  - `"list_groups"`: Lists all device groups (meshes) accessible by the configured user.
  - `"list_devices"`: Lists nodes within a specific group or across all groups.
  - `"device_info"`: Reads device detail responses for one node.
  - `"list_events"`: Lists audit events, optionally filtered by `node_id` or `user_id`.
  - `"wake"`: Sends a Wake-on-LAN request through MeshCentral.
  - `"power_action"`: Sends a supported power operation to a node.
  - `"run_command"`: Runs a command through MeshAgent and waits for the MeshCentral response.
- **`mesh_id`**: Optional for `list_devices`.
- **`node_id`**: Required for `device_info`, `wake`, `power_action`, and `run_command`. Optional filter for `list_events`.
- **`user_id`**: Optional user filter for `list_events`.
- **`limit`**: Optional maximum number of events for `list_events`.
- **`power_action`**: Required for `power_action`. Use one of `off`, `reset`, `sleep`, `amt_on`, `amt_off`, or `amt_reset`. Legacy numeric values are accepted only for compatibility: `1` maps to `sleep`, `3` maps to `off`, and `4` maps to `reset`; legacy `2` is rejected because current MeshCentral uses action type `2` for power off, not hibernate.
- **`command`**: Required for `run_command`.

## Notes

- Interactive shell, file transfer, desktop relay, WebRelay, invite links, device sharing, reports, and MeshCentral user/group administration are not exposed by this tool yet. Those features require additional MeshCentral relay or admin command handling.
- `run_command` uses MeshCentral's `runcommands` control action. Returned data depends on the MeshCentral server and agent response.
- The old `shell` operation is intentionally unsupported until a real `meshrelay.ashx` tunnel is implemented.

## Examples

**List all device groups:**
```json
{
  "action": "meshcentral",
  "operation": "list_groups"
}
```

**List devices in a specific group:**
```json
{
  "action": "meshcentral",
  "operation": "list_devices",
  "mesh_id": "mesh//..."
}
```

**Read device details:**
```json
{
  "action": "meshcentral",
  "operation": "device_info",
  "node_id": "node//..."
}
```

**Reboot a device:**
```json
{
  "action": "meshcentral",
  "operation": "power_action",
  "node_id": "node//...",
  "power_action": "reset"
}
```

**Run a command on a remote device:**
```json
{
  "action": "meshcentral",
  "operation": "run_command",
  "node_id": "node//...",
  "command": "systemctl status nginx"
}
```
