---
conditions: ["meshcentral_enabled"]
---
# MeshCentral

The `meshcentral` tool allows you to interact with devices managed by a MeshCentral server. You can list device groups, list individual devices, send Wake-on-LAN packets, execute power actions (sleep, hibernate, power off, reset), and run arbitrary shell commands on remote devices using the MeshAgent.

**Warning:** Running commands and executing power actions are highly privileged operations. Always verify the target `node_id` or `mesh_id` before executing commands that modify state.

## Parameters

- **`operation`**: (Required) The action to perform. Must be one of:
  - `"list_groups"`: Lists all device groups (meshes) accessible by the user.
  - `"list_devices"`: Lists nodes within a specific group or all devices.
  - `"wake"`: Sends a Wake-on-LAN magic packet to a sleeping device.
  - `"power_action"`: Executes a power operation on a device.
  - `"run_command"`: Dispatches a command string to be executed by the remote MeshAgent (fire-and-forget, no output returned).
  - `"shell"`: Executes a command via interactive WebSocket shell and returns the output.
- **`mesh_id`**: (Optional for `list_devices`) The identifier for a specific device group. If omitted, **all devices across all groups** are returned. Required when you already know the group and want to narrow the results.
- **`node_id`**: (Required for `wake`, `power_action`, `run_command`, `shell`) The unique identifier for a specific device endpoint.
- **`power_action`**: (Required for `power_action` operation) Integer representing the action to perform: `1` (Sleep), `2` (Hibernate), `3` (PowerOff), `4` (Reset/Reboot).
- **`command`**: (Required for `run_command` and `shell`) The command string to execute on the remote device's shell.

## Operation Details

### `run_command` vs `shell`

Both operations execute commands on remote devices, but differ in how they handle output:

- **`run_command`**: Fire-and-forget. Sends the command to the agent but does not wait for or return output. Use for long-running commands or when output is not needed.
- **`shell`**: Interactive WebSocket shell. Sends the command and waits for output (up to 10 seconds). Use when you need to see the command result.

## Examples

**List all device groups:**
```json
{
  "action": "meshcentral",
  "operation": "list_groups"
}
```

**List all devices (across all groups):**
```json
{
  "action": "meshcentral",
  "operation": "list_devices"
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

**Reboot a specific device:**
```json
{
  "action": "meshcentral",
  "operation": "power_action",
  "node_id": "node//...",
  "power_action": 4
}
```

**Execute a command on a remote device (fire-and-forget):**
```json
{
  "action": "meshcentral",
  "operation": "run_command",
  "node_id": "node//...",
  "command": "systemctl restart nginx"
}
```

**Execute a shell command and get the output:**
```json
{
  "action": "meshcentral",
  "operation": "shell",
  "node_id": "node//...",
  "command": "ls -la /etc"
}
```
