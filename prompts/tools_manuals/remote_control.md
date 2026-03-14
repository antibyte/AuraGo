# Remote Control Tool Manual

## Overview
The Remote Control tool enables management of remote machines running the AuraGo Remote agent. It provides shell execution, file transfer, system information collection, and device lifecycle management over a secure WebSocket connection.

## Architecture
- **Supervisor** (this AuraGo instance) maintains WebSocket connections to all remote agents
- **Remote agents** are lightweight binaries that auto-connect, authenticate via HMAC, and execute commands
- Communication is secured with per-device shared keys and HMAC-SHA256 message signing
- All operations are audited when audit logging is enabled

## Operations

### list_devices
Lists all registered devices with their status and connectivity.
```json
{"operation": "list_devices"}
```

### device_status
Returns detailed device info including telemetry data if connected.
```json
{"operation": "device_status", "device_name": "webserver-01"}
```
Returns: hostname, OS, architecture, IP, status, read-only state, allowed paths, telemetry (CPU, RAM, disk usage), version, tags.

### execute_command
Runs a shell command on the remote device (60s timeout).
```json
{"operation": "execute_command", "device_name": "webserver-01", "command": "df -h"}
```
**Blocked in read-only mode.** Uses `sh -c` on Linux/macOS, `cmd /C` on Windows.

### read_file
Reads a file from the remote device (30s timeout).
```json
{"operation": "read_file", "device_name": "webserver-01", "path": "/etc/hostname"}
```
Only paths within the device's `allowed_paths` configuration are accessible.

### write_file
Writes content to a file on the remote device (30s timeout).
```json
{"operation": "write_file", "device_name": "webserver-01", "path": "/tmp/config.txt", "content": "key=value"}
```
**Blocked in read-only mode.**

### list_files
Lists files in a directory on the remote device.
```json
{"operation": "list_files", "device_name": "webserver-01", "path": "/var/log", "recursive": false}
```

### sysinfo
Collects system metrics from the remote device (15s timeout).
```json
{"operation": "sysinfo", "device_name": "webserver-01"}
```
Returns: hostname, OS, architecture, CPU count, total/free memory, disk usage, uptime.

### revoke_device
Revokes a device's access, disconnects it, and marks it as revoked.
```json
{"operation": "revoke_device", "device_name": "webserver-01"}
```
**Blocked in read-only mode.** The remote agent will clean up and uninstall itself upon receiving the revocation signal.

## Security Notes
- Each device has a unique shared HMAC key stored in the vault
- Messages are timestamped and validated (30s maximum clock drift)
- File operations enforce path allowlists configured per device
- All command executions are logged in the audit trail
- Read-only mode can be toggled per device or globally
