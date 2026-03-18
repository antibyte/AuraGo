# Tool: `network_ping`

## Purpose
Send ICMP echo requests (ping) to a hostname or IP address and return detailed latency statistics: round-trip time (min/avg/max/stddev), packet loss, and per-packet RTT list.

## When to Use
- Check whether a host is reachable before attempting a connection.
- Diagnose network latency to a server, router, or IoT device.
- Verify that a newly deployed service or device responds on the network.
- Monitor network health in a home-lab environment.

## Parameters
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `host` | string | ✅ | Hostname or IP address to ping |
| `count` | integer | ❌ | Number of ICMP packets to send (1–20, default: 4) |
| `timeout` | integer | ❌ | Total timeout in seconds (1–60, default: 10) |

## Output
JSON with these fields:
- `status` — `"success"` or `"error"`
- `host` — resolved target address
- `ip_addr` — resolved IP address
- `packets_sent` — number of packets transmitted
- `packets_recv` — number of packets received
- `packet_loss_percent` — percentage of packets lost (0 = no loss)
- `min_rtt`, `avg_rtt`, `max_rtt`, `stddev_rtt` — latency statistics (formatted duration strings, e.g. `"1.234ms"`)
- `rtts` — array of individual round-trip times
- `message` — error description (only present on error)

## Privilege Notes
ICMP raw sockets require elevated privileges on some platforms:
- **Windows**: Works without elevation in most cases.
- **Linux/macOS**: Requires `root` or the `CAP_NET_RAW` capability. If running in a Docker container, add `--cap-add=NET_RAW`. If the process lacks permission, the tool returns a helpful error explaining the requirement.

## Example Calls
```json
{ "host": "192.168.1.1" }
{ "host": "google.com", "count": 10, "timeout": 15 }
{ "host": "proxmox.lan", "count": 3 }
```
