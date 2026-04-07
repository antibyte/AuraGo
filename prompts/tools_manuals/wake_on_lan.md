# Wake-on-LAN (`wake_on_lan`)

Send a Wake-on-LAN magic packet to wake up a device on the local network. The device must support WOL and be physically connected to the network (or be on the same broadcast domain).

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `server_id` | string | no | Device ID from the inventory (MAC address is auto-resolved) |
| `mac_address` | string | no* | MAC address of the device (e.g. `AA:BB:CC:DD:EE:FF`). Required if server_id not provided. |
| `ip_address` | string | no | Broadcast IP address (e.g. `192.168.1.255`). Defaults to `255.255.255.255`. |

*Either `server_id` or `mac_address` must be provided.

## Examples

**Wake a device by inventory ID:**
```json
{"action": "wake_on_lan", "server_id": "device-123"}
```

**Wake a device by MAC address:**
```json
{"action": "wake_on_lan", "mac_address": "AA:BB:CC:DD:EE:FF"}
```

**Wake with custom broadcast address:**
```json
{"action": "wake_on_lan", "mac_address": "AA:BB:CC:DD:EE:FF", "ip_address": "192.168.1.255"}
```

## How It Works

The Wake-on-LAN packet is a broadcast UDP packet sent to port 9. It contains:
- 6 bytes of `0xFF`
- Followed by 16 repetitions of the 6-byte MAC address

## Notes

- **Network scope**: The magic packet is broadcast by default to `255.255.255.255`. For routed WOL, you may need to configure your router's directed broadcast or use the specific subnet broadcast IP.
- **Device support**: The target device must have WOL enabled in its BIOS/UEFI and network card configuration.
- **Inventory integration**: If you use `server_id`, the device must be registered in AuraGo's inventory with a valid MAC address.
- **Authentication**: No authentication required for WOL packets (this is a limitation of the WOL protocol itself).
