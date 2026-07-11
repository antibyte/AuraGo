---
id: tools_remote_control
tags: [conditional]
priority: 100
conditions: ["remote_control_enabled"]
---

## Remote Control Tool

You can manage and interact with remote devices using focused remote-control tools. This system allows you to connect to, monitor, and control remote machines running the AuraGo Remote agent.

Use `remote_control_devices` for inventory and status, `remote_control_shell` for remote commands, `remote_control_files` for remote file operations, and `remote_control_desktop` for remote desktop/session actions. The legacy `remote_control` action remains accepted for older clients.

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
| `shell_session_start` | Start an interactive/long-running shell session | `device_id` or `device_name`, `command` |
| `shell_session_read` | Poll shell session output | `device_id` or `device_name`, `session_id` |
| `shell_session_input` | Send input to a shell session | `device_id` or `device_name`, `session_id`, `input` |
| `shell_session_stop` | Stop a shell session | `device_id` or `device_name`, `session_id` |
| `shell_session_list` | List client-owned shell sessions | `device_id` or `device_name` |
| `read_file` | Read a file from a remote device | `device_id` or `device_name`, `path` |
| `write_file` | Write content to a file on a remote device | `device_id` or `device_name`, `path`, `content` |
| `file_patch` | Dry-run or apply exact guarded text patches | `device_id` or `device_name`, `path`, `expected_sha256`, `patches` |
| `list_files` | List files in a directory on a remote device | `device_id` or `device_name`, `path`, `recursive` (optional) |
| `sysinfo` | Get system information (OS, CPU, RAM, disk, uptime) from a remote device | `device_id` or `device_name` |
| `revoke_device` | Revoke a device's access and disconnect it | `device_id` or `device_name` |

### Workflow
1. Use `list_devices` to see all registered devices and their connection status
2. Use `device_status` with `device_name` to get detailed info about a specific device
3. Use `sysinfo` to collect system metrics from a connected device
4. Use `remote_control_shell` `execute_command` for one-shot shell commands; use `shell_session_start/read/input/stop/list` only for interactive or long-running commands
5. Use `remote_control_files` `read_file` / `write_file` / `file_patch` / `list_files` for file operations
6. Use `revoke_device` to disconnect and block a device

### Guidelines
- Always use `list_devices` first to confirm device IDs/names before targeted operations
- Check that a device is connected before attempting commands — offline devices cannot execute commands
- Prefer `device_name` over `device_id` for readability when possible
- `execute_command` supports standard shell commands (sh -c on Linux/macOS, cmd /C on Windows)
- For shell sessions, poll with `shell_session_read` after start. `initial_wait_ms` is only the first read wait, not the session lifetime. AuraGo does not store session processes; after reconnect, use `shell_session_list` and then read/input/stop.
- File operations respect the device's `allowed_paths` configuration — only paths within allowed directories are accessible
- Prefer `file_patch` for precise AgoDesk edits: read first, use the returned/current `expected_sha256`, dry-run first (`dry_run` defaults true), then apply with `dry_run:false` only after the dry run is acceptable
- If `file_patch` returns `FILE_PATCH_MISMATCH` or `FILE_HASH_MISMATCH`, read the file again and create a fresh exact patch instead of fuzzy-writing
- When a device is in read-only mode, `execute_command`, all shell session operations, `write_file`, `file_patch`, and `revoke_device` are blocked
- Never expose shared keys or enrollment tokens in responses
- Commands have a 60-second timeout; file operations have a 30-second timeout
