# Remote Control (`remote_control`)

Manage remote machines running the AuraGo Remote agent or a paired agodesk desktop client. Provides shell execution, file transfer, system information collection, device lifecycle management, and agodesk desktop screenshot, discovery, UI automation, browser CDP, and input operations over secure WebSocket connections.

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
| `command` | string | for execute_command | Shell command to execute |
| `path` | string | for read_file, write_file, list_files | File/directory path |
| `content` | string | for write_file | File content to write |
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
| `action` | string | for desktop_ui_action and desktop_browser_action | UI/browser action such as `click`, `focus`, `set_value`, or `fill` |
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

**Read a file:**
```json
{"action": "remote_control", "operation": "read_file", "device_name": "webserver-01", "path": "/etc/hostname"}
```

**Write a file:**
```json
{"action": "remote_control", "operation": "write_file", "device_name": "webserver-01", "path": "/tmp/config.txt", "content": "key=value"}
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

## Architecture

- **Supervisor**: This AuraGo instance maintains WebSocket connections to all remote agents
- **Remote agents**: Lightweight binaries that auto-connect, authenticate via HMAC, and execute commands
- **Security**: Communication is secured with per-device shared keys and HMAC-SHA256 message signing
- **Audit**: All operations are audited when audit logging is enabled

## Notes

- **Timeouts**: Command execution has 60s timeout, file operations have 30s timeout, sysinfo has 15s timeout
- **Read-only mode**: execute_command, write_file, revoke_device, edit operations, desktop_input, desktop_ui_action, and desktop_browser_action are blocked when read-only mode is enabled. Discovery, UI tree reads, browser connect/snapshot/disconnect, screenshots, and permission probes remain allowed.
- **Path restrictions**: File operations only access paths within the device's configured `allowed_paths`
- **Platform support**: Uses `sh -c` on Linux/macOS, `cmd /C` on Windows
- **Connection route**: Personalized `aurago-remote` downloads can embed an automatic, Tailscale, or manual supervisor WebSocket URL via `remote_control.connection_mode`.
- **agodesk desktop safety**: Screenshots, discovery, UI tree reads, and browser snapshots do not require local approval. Desktop input, UI actions, and browser actions require explicit local approval in the agodesk remote-control banner; AuraGo cannot approve or bypass that from the backend.
- **agodesk streaming**: Desktop streaming operations are reserved but not available in this backend version.
