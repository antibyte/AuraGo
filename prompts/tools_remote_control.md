---
id: tools_remote_control
tags: [conditional]
priority: 100
conditions: ["remote_control_enabled"]
---

## Remote Control Tool

You can manage and interact with remote devices using the `remote_control` tool. This system allows you to connect to, monitor, and control remote machines running the AuraGo Remote agent.

### Concepts
- **Device** = a remote machine running the AuraGo Remote agent binary
- **Enrollment** = the process of a new device registering and authenticating with the supervisor
- **Shared Key** = HMAC key used for secure supervisor↔device WebSocket communication
- **Read-Only Mode** = when enabled, prevents write operations (file writes, command execution) on the device

### Available Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `list_devices` | List all registered remote devices with status and connectivity | — |
| `device_status` | Get detailed info about a specific device (telemetry, connection state, config) | `device_id` or `device_name` |
| `execute_command` | Run a shell command on a remote device | `device_id` or `device_name`, `command` |
| `read_file` | Read a file from a remote device | `device_id` or `device_name`, `path` |
| `write_file` | Write content to a file on a remote device | `device_id` or `device_name`, `path`, `content` |
| `list_files` | List files in a directory on a remote device | `device_id` or `device_name`, `path`, `recursive` (optional) |
| `sysinfo` | Get system information (OS, CPU, RAM, disk, uptime) from a remote device | `device_id` or `device_name` |
| `revoke_device` | Revoke a device's access and disconnect it | `device_id` or `device_name` |

### Workflow
1. Use `list_devices` to see all registered devices and their connection status
2. Use `device_status` with `device_name` to get detailed info about a specific device
3. Use `sysinfo` to collect system metrics from a connected device
4. Use `execute_command` to run shell commands remotely
5. Use `read_file` / `write_file` / `list_files` for file operations
6. Use `revoke_device` to disconnect and block a device

### Guidelines
- Always use `list_devices` first to confirm device IDs/names before targeted operations
- Check that a device is connected before attempting commands — offline devices cannot execute commands
- Prefer `device_name` over `device_id` for readability when possible
- `execute_command` supports standard shell commands (sh -c on Linux/macOS, cmd /C on Windows)
- File operations respect the device's `allowed_paths` configuration — only paths within allowed directories are accessible
- When a device is in read-only mode, `execute_command`, `write_file`, and `revoke_device` are blocked
- Never expose shared keys or enrollment tokens in responses
- Commands have a 60-second timeout; file operations have a 30-second timeout
