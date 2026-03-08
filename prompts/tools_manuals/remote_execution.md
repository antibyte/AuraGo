---
id: remote_execution
tags: [core]
priority: 100
conditions: ["allow_remote_shell", "wol_enabled"]
---

# REMOTE EXECUTION TOOLS

> ⚠️ **CRITICAL:** NEVER use `execute_python` or `execute_shell` to manage SSH connections, generate SSH keys, or run remote commands. This is wasteful and will fail. Always use the dedicated tools documented below: `query_inventory`, `execute_remote_shell`, `transfer_remote_file`, `register_device`.

## Tool: Query Inventory (`query_inventory`)

Search the internal SQLite registry for saved network devices (like servers, Chromecasts, printers, etc.) by tag, device type, or name.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `tag` | string | no | Filter by tag (e.g., `"prod"`, `"web"`, `"smart-home"`) |
| `device_type` | string | no | Filter by device type (e.g., `"server"`, `"chromecast"`) |
| `hostname` | string | no | Partial device name match |

Returns a list of matching devices (ID, Name, Type, IP Address, Tags, Description).

```json
{"action": "query_inventory", "tag": "prod", "hostname": "myserver"}
```

---

## Tool: Remote Shell (`execute_remote_shell`)

Run a command on a remote server via SSH. Authentication is handled automatically via the vault.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `server_id` | string | yes | UUID of the server from inventory |
| `command` | string | yes | Shell command to execute |

```json
{"action": "execute_remote_shell", "server_id": "uuid-1234", "command": "uptime"}
```

---

## Tool: Remote File Transfer (`transfer_remote_file`)

Upload or download files via SFTP.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `server_id` | string | yes | UUID of the server |
| `local_path` | string | yes | Absolute path on supervisor. **Must** be within `agent_workspace/workdir/` |
| `remote_path` | string | yes | Absolute path on the remote server |
| `direction` | string | yes | `upload` (local -> remote) or `download` (remote -> local) |

```json
{"action": "transfer_remote_file", "server_id": "uuid-1234", "local_path": "...", "remote_path": "...", "direction": "upload"}
```

---

## Tool: Register Device (`register_device`)

Enroll a new network device or server into the inventory and secure vault. You should proactively save important network devices here to remember them later.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `hostname` | string | yes | Descriptive device name (e.g., "Main Webserver" or "Living Room Chromecast") |
| `device_type` | string | no | Type of device (e.g., "server", "chromecast", "printer"). Default is "server" |
| `description` | string | no | What this device is used for |
| `ip_address` | string | no | Device IP address |
| `port` | integer | no | Port (default: 22 for servers) |
| `username` | string | conditional | SSH username (required for servers) |
| `password` | string | conditional | SSH password (required for servers if no `private_key_path`) |
| `private_key_path` | string | conditional | Path to private key on supervisor (required for servers if no `password`) |
| `mac_address` | string | no | MAC address (e.g. `"AA:BB:CC:DD:EE:FF"`) — optional, only needed for Wake-on-LAN |
| `tags` | string | no | Comma-separated tags (e.g. "prod,web" or "smart-home") |

```json
{"action": "register_device", "hostname": "Wohnzimmer Chromecast", "device_type": "chromecast", "ip_address": "192.168.1.50", "description": "Speaker used for TTS", "tags": "smart-home,speaker"}
```

---

## Tool: Wake-on-LAN (`wake_on_lan`)

Send a Wake-on-LAN magic packet (UDP broadcast) to wake up a sleeping or powered-off device. The device must support WOL and be on the local network.

> ℹ️ You can call `query_inventory` first to find devices that have a MAC address registered (`mac_address` field non-empty).

| Parameter | Type | Required | Description |
|---|---|---|---|
| `server_id` | string | conditional | Device UUID from inventory — the registered MAC address is looked up automatically |
| `mac_address` | string | conditional | MAC address to wake up (e.g. `"AA:BB:CC:DD:EE:FF"`). Required if `server_id` has no MAC registered. |
| `ip_address` | string | no | Optional subnet broadcast address (e.g. `"192.168.1.255"`). Defaults to `255.255.255.255`. |

```json
{"action": "wake_on_lan", "server_id": "uuid-1234"}
{"action": "wake_on_lan", "mac_address": "AA:BB:CC:DD:EE:FF"}
```