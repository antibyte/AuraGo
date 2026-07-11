# Remote Control Legacy Compatibility (`remote_control`)

Manage remote machines running the AuraGo Remote agent or a paired agodesk desktop client. Provides shell execution, file transfer, system information collection, device lifecycle management, and agodesk desktop screenshot, discovery, UI automation, browser CDP, and input operations over secure WebSocket connections.

Prefer `remote_control_devices`, `remote_control_shell`, `remote_control_files`, and `remote_control_desktop` when they are visible. The legacy `remote_control` action remains accepted for dispatch compatibility with older clients and prompts.

## Operations

| Operation | Description |
|-----------|-------------|
| `list_devices` | List all registered devices with status |
| `device_status` | Get detailed device info including telemetry |
| `execute_command` | Run a shell command on the remote device |
| `shell_session_start` | Start a persistent shell session for interactive or long-running work |
| `shell_session_read` | Poll output from a persistent shell session |
| `shell_session_input` | Send input to a persistent shell session |
| `shell_session_stop` | Stop a persistent shell session |
| `shell_session_list` | List client-owned persistent shell sessions |
| `read_file` | Read a file from the remote device |
| `write_file` | Write content to a file on the remote device |
| `file_patch` | Dry-run or apply exact text patches guarded by `expected_sha256` |
| `list_files` | List files in a directory on the remote device |
| `sysinfo` | Collect system metrics from the remote device |
| `revoke_device` | Revoke a device's access |
| `desktop_screenshot` | Capture an agodesk display or window screenshot |
| `desktop_permission_request` | Ask agodesk for desktop input permission/status |
| `desktop_input` | Send mouse/keyboard/text input to agodesk after local approval |
| `desktop_list_displays` | List agodesk displays/monitors |
| `desktop_list_windows` | List visible agodesk windows |
| `desktop_active_window` | Return the currently active agodesk window |
| `desktop_host_info` | Return host/platform metadata from agodesk |
| `desktop_ui_tree` | Read the accessibility tree for the active/root window or a supplied `window_id` |
| `desktop_ui_action` | Perform an approved semantic UI action such as click/focus/set_value |
| `desktop_browser_connect` | Connect agodesk to a local browser CDP endpoint |
| `desktop_browser_snapshot` | Read a browser DOM/text snapshot through CDP |
| `desktop_browser_action` | Perform an approved browser CDP action such as click/fill |
| `desktop_browser_disconnect` | End the agodesk browser CDP session |

AgoDesk desktop commands require the client to advertise matching `session.start.client_capabilities`: `remote.desktop.capture` for screenshots, `remote.desktop.permission_request` for permission checks, `remote.desktop.input` for input, `remote.desktop.discovery` for display/window/host discovery, `remote.desktop.ui_automation` for UI tree/action, and `remote.desktop.browser` for browser CDP. If a desktop command returns `UNSUPPORTED_CAPABILITY`, the WebSocket may still be alive for chat/heartbeat, but that client version or configuration is not remote-control capable.

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of the operations above |
| `device_name` | string | for most operations | Name of the remote device |
| `device_id` | string | for most operations | Alternative to device_name |
| `command` | string | for execute_command, shell_session_start | Shell command to execute |
| `cwd_id` | string | optional for shell_session_start | AgoDesk working-directory root id |
| `initial_wait_ms` | integer | optional for shell_session_start | Initial read wait after start; not the session lifetime |
| `session_id` | string | for shell_session_read/input/stop | Shell session id |
| `offset`, `limit`, `wait_ms` | integer | optional for shell_session_read/list | Output offset, bounded read/list size, and long-poll wait |
| `input` | string | for shell_session_input | Text or control input to send |
| `path` | string | for read_file, write_file, file_patch, list_files, file_search | File/directory path |
| `root_id` | string | optional for AgoDesk file access | Stable AgoDesk file-access root id; when set, `path` is relative to that root |
| `content` | string | for write_file | File content to write |
| `expected_sha256` | string | for file_patch | Current SHA-256 of the target file |
| `dry_run` | boolean | optional for file_patch | Defaults to true; set false only to apply after a successful dry run |
| `patches` | array | for file_patch | Exact `{old_text,new_text,expected_occurrences}` replacements |
| `recursive` | boolean | for list_files | List recursively (default: false) |
| `display_id` | string | optional for desktop_screenshot | Monitor id such as `display-0`; omitted captures the primary display |
| `window_id` | string | optional for desktop_screenshot | Window id to capture a single window |
| `format` | string | optional for desktop_screenshot | `png` or `jpeg` |
| `quality` | integer | optional for desktop_screenshot | Image quality 1-100 for lossy formats |
| `include_data_base64` | boolean | optional for desktop_screenshot | Default false stores image data to a workspace file and returns the path |
| `kind` | string | for desktop_input | `mouse_move`, `mouse_click`, `key_down`, `key_up`, or `text` |
| `x`, `y` | integer | for mouse desktop_input | Mouse coordinates |
| `absolute` | boolean | optional for mouse_move | Set true for absolute mouse coordinates |
| `button` | string | for mouse_click | `left`, `right`, or `middle` |
| `input_action` | string | optional for mouse_click | Preferred click action field, e.g. `click`, `down`, or `up`; forwarded to agodesk as protocol `action` |
| `key`, `code` | string/integer | for key_down/key_up | Keyboard key name or numeric key code |
| `text` | string | for text input | Text to type |
| `element_id` | string | for desktop_ui_action | Element id from a prior `desktop_ui_tree` result |
| `action` / `sub_operation` | string | for file_search, file_read_advanced, desktop_ui_action, and desktop_browser_action | `file_search`: `grep`, `grep_recursive`, or `find`; UI/browser action such as `click`, `focus`, `set_value`, or `fill`. Use `sub_operation` in raw JSON fallback calls where `action` is already the tool name. |
| `endpoint` | string | optional for desktop_browser_connect | Browser CDP endpoint, e.g. `http://127.0.0.1:9222` |
| `selector` | string | optional for browser operations | CSS selector for browser snapshot/action |
| `include_html` | boolean | optional for desktop_browser_snapshot | Include HTML when supported by agodesk |
| `value` | string | optional for UI/browser actions | Value for `set_value`, `fill`, `type`, or `select` |

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

**Start and poll a shell session:**
```json
{"action": "remote_control", "operation": "shell_session_start", "device_name": "office-pc", "command": "npm run dev", "cwd_id": "workspace", "initial_wait_ms": 1000}
```

```json
{"action": "remote_control", "operation": "shell_session_read", "device_name": "office-pc", "session_id": "sh-abc", "offset": -2000, "limit": 2000, "wait_ms": 250}
```

**Read a file:**
```json
{"action": "remote_control", "operation": "read_file", "device_name": "webserver-01", "path": "/etc/hostname"}
```

**Write a file:**
```json
{"action": "remote_control", "operation": "write_file", "device_name": "webserver-01", "path": "/tmp/config.txt", "content": "key=value"}
```

**Patch a file through AgoDesk file access:**
```json
{"action": "remote_control", "operation": "file_patch", "device_name": "office-pc", "root_id": "workspace", "path": "src/main.go", "expected_sha256": "<sha256-from-read>", "dry_run": true, "patches": [{"old_text": "socket.connect();", "new_text": "await socket.connect();", "expected_occurrences": 1}]}
```

**Capture an agodesk display:**
```json
{"action": "remote_control", "operation": "desktop_screenshot", "device_name": "office-pc", "display_id": "display-0", "format": "png"}
```

**Inspect active window and UI tree:**
```json
{"action": "remote_control", "operation": "desktop_active_window", "device_name": "office-pc"}
```

```json
{"action": "remote_control", "operation": "desktop_ui_tree", "device_name": "office-pc", "window_id": "win-12345678"}
```

**Perform an approved UI action:**
```json
{"action": "remote_control", "operation": "desktop_ui_action", "device_name": "office-pc", "element_id": "elem-42", "action": "click"}
```

**Use browser CDP through agodesk:**
```json
{"action": "remote_control", "operation": "desktop_browser_connect", "device_name": "office-pc", "endpoint": "http://127.0.0.1:9222"}
```

```json
{"action": "remote_control", "operation": "desktop_browser_snapshot", "device_name": "office-pc", "selector": "main"}
```

**Request local input approval:**
```json
{"action": "remote_control", "operation": "desktop_permission_request", "device_name": "office-pc"}
```

**Send approved desktop input:**
```json
{"action": "remote_control", "operation": "desktop_input", "device_name": "office-pc", "kind": "mouse_click", "x": 100, "y": 200, "button": "left", "input_action": "click"}
```

**Search files through AgoDesk file access:**
```json
{"action": "remote_control", "operation": "file_search", "device_name": "office-pc", "root_id": "workspace", "path": ".", "sub_operation": "grep_recursive", "pattern": "TODO"}
```

## Architecture

- **Supervisor**: This AuraGo instance maintains WebSocket connections to all remote agents
- **Remote agents**: Lightweight binaries that auto-connect, authenticate via HMAC, and execute commands
- **Security**: Communication is secured with per-device shared keys and HMAC-SHA256 message signing
- **Audit**: All operations are audited when audit logging is enabled

## Notes

- **Timeouts**: Command execution has 60s timeout, file operations have 30s timeout, sysinfo has 15s timeout
- **Read-only mode**: execute_command, all shell_session operations, write_file, file_patch, revoke_device, edit operations, desktop_input, desktop_ui_action, and desktop_browser_action are blocked when read-only mode is enabled. Discovery, UI tree reads, browser connect/snapshot/disconnect, screenshots, and permission probes remain allowed.
- **Shell sessions**: Use one-shot `execute_command` unless the command is interactive or long-running. Poll after `shell_session_start`; AuraGo does not persist processes, so reconnect recovery is `shell_session_list` plus read/input/stop against client-owned sessions.
- **File patching**: Dry-run first. If AgoDesk returns `FILE_PATCH_MISMATCH` or `FILE_HASH_MISMATCH`, read the file again and build a fresh exact patch.
- **Path restrictions**: Classic remote-agent file operations only access paths within the device's configured `allowed_paths`. AgoDesk file operations should use the `root_id`s reported in the active AgoDesk chat context; AuraGo forwards only known roots and AgoDesk enforces canonical local path boundaries.
- **Platform support**: Uses `sh -c` on Linux/macOS, `cmd /C` on Windows
- **Connection route**: Personalized `aurago-remote` downloads can embed an automatic, Tailscale, or manual supervisor WebSocket URL via `remote_control.connection_mode`.
- **agodesk desktop safety**: Screenshots, discovery, UI tree reads, and browser snapshots do not require local approval. Desktop input, UI actions, and browser actions require explicit local approval in the agodesk remote-control banner; AuraGo cannot approve or bypass that from the backend.
- **agodesk streaming**: Desktop streaming operations are reserved but not available in this backend version.
