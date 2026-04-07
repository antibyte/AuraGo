# Remote Control (`remote_control`)

Manage remote machines running the AuraGo Remote agent. Provides shell execution, file transfer, system information collection, and device lifecycle management over a secure WebSocket connection.

## Operations

| Operation | Description |
|-----------|-------------|
| `list_devices` | List all registered devices with status |
| `device_status` | Get detailed device info including telemetry |
| `execute_command` | Run a shell command on the remote device |
| `read_file` | Read a file from the remote device |
| `write_file` | Write content to a file on the remote device |
| `list_files` | List files in a directory on the remote device |
| `sysinfo` | Collect system metrics from the remote device |
| `revoke_device` | Revoke a device's access |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of the operations above |
| `device_name` | string | for most operations | Name of the remote device |
| `device_id` | string | for most operations | Alternative to device_name |
| `command` | string | for execute_command | Shell command to execute |
| `path` | string | for read_file, write_file, list_files | File/directory path |
| `content` | string | for write_file | File content to write |
| `recursive` | boolean | for list_files | List recursively (default: false) |

## Examples

**List devices:**
```json
{"action": "remote_control", "operation": "list_devices"}
```

**Get device status:**
```json
{"action": "remote_control", "operation": "device_status", "device_name": "webserver-01"}
```

**Execute a command:**
```json
{"action": "remote_control", "operation": "execute_command", "device_name": "webserver-01", "command": "df -h"}
```

**Read a file:**
```json
{"action": "remote_control", "operation": "read_file", "device_name": "webserver-01", "path": "/etc/hostname"}
```

**Write a file:**
```json
{"action": "remote_control", "operation": "write_file", "device_name": "webserver-01", "path": "/tmp/config.txt", "content": "key=value"}
```

## Architecture

- **Supervisor**: This AuraGo instance maintains WebSocket connections to all remote agents
- **Remote agents**: Lightweight binaries that auto-connect, authenticate via HMAC, and execute commands
- **Security**: Communication is secured with per-device shared keys and HMAC-SHA256 message signing
- **Audit**: All operations are audited when audit logging is enabled

## Notes

- **Timeouts**: Command execution has 60s timeout, file operations have 30s timeout, sysinfo has 15s timeout
- **Read-only mode**: execute_command, write_file, and revoke_device are blocked when read-only mode is enabled
- **Path restrictions**: File operations only access paths within the device's configured `allowed_paths`
- **Platform support**: Uses `sh -c` on Linux/macOS, `cmd /C` on Windows
