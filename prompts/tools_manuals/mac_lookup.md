# mac_lookup — MAC Address Lookup via ARP Table

## Purpose
Look up the hardware MAC address of a device on the local network using the OS ARP cache. No root privileges or extra capabilities are required. Works on Linux, Windows, macOS and in Docker without `NET_RAW`.

## Gate
Requires `tools.network_scan.enabled: true` in config (same as `mdns_scan`).

## Parameters
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `ip` | string | ✅ | IPv4 address of the target device (e.g. `192.168.1.42`) |

## Response
```json
{
  "status": "success",
  "ip_address": "192.168.1.42",
  "mac_address": "AA:BB:CC:DD:EE:FF",
  "source": "proc_net_arp"
}
```
or
```json
{
  "status": "not_found",
  "ip_address": "192.168.1.42",
  "message": "IP 192.168.1.42 not found in ARP cache..."
}
```

## Platform implementation
| OS | Method | Notes |
|----|--------|-------|
| Linux | Reads `/proc/net/arp` directly | No subprocess, instant |
| Windows | Executes `arp -a <ip>`, parses output | Works without admin |
| macOS | Executes `arp <ip>`, parses output | Works without root |

## Limitations & best practices
- **ARP cache only**: The device must have communicated recently over the LAN. If the entry has expired, the result will be `not_found`.
- **Local subnet only**: ARP does not cross router boundaries. The device must be on the same Layer 2 network as the agent host.
- **Ping first**: To populate the ARP cache before calling `mac_lookup`, send a `network_ping` to the target IP first.

## Typical workflow
```
1. mdns_scan (auto_register=true)       → discovers devices, adds to registry (MAC auto-populated if cached)
2. mac_lookup ip="192.168.1.42"         → get MAC for a specific IP
3. manage_inventory operation=update    → store the MAC in the device record
```

## Integration with mdnsAutoRegister
When `mdns_scan` is called with `auto_register=true`, MAC addresses are automatically populated for discovered devices that already have a cached ARP entry. No separate `mac_lookup` call is needed in that flow.

## Example tool calls
```json
{"action": "mac_lookup", "ip": "192.168.1.1"}
```
```json
{"action": "mac_lookup", "ip": "10.0.0.50"}
```
