# Tool: `port_scanner`

## Purpose
Perform TCP connect scans on a target host to discover open ports, identify running services, and grab banners.

## When to Use
- Discover which services are running on a new device in the home lab.
- Verify that a service is listening on the expected port after deployment.
- Check firewall rules by scanning for open/closed ports.
- Inventory services on a network host.

## Parameters
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `host` | string | ✅ | Hostname or IP address to scan |
| `port_range` | string | ❌ | Ports to scan: single (`80`), list (`80,443,8080`), range (`1-1024`), or `common` (default) |
| `timeout_ms` | integer | ❌ | Per-port connection timeout in milliseconds (100–5000, default: 1000) |

## Output
JSON with: `status`, `host`, `open_ports` (array of `{port, status, service?, banner?}`), `closed_count`, `total_scanned`.

## Limits
- Maximum 1024 ports per scan.
- 50 concurrent connections.
- Banner grab attempts a 500ms read on open ports.
- Requires `network_ping` permission to be enabled.

## Example Calls
```json
{ "host": "192.168.1.1" }
{ "host": "nas.local", "port_range": "80,443,8080,9090" }
{ "host": "10.0.0.5", "port_range": "1-1024", "timeout_ms": 2000 }
{ "host": "proxmox.lan", "port_range": "common" }
```
