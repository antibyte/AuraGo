# Fritz!Box Network Tool (`fritzbox_network`)

Manage the Fritz!Box router's network features: WLAN radios, connected hosts, Wake-on-LAN, and port forwarding (NAT).

**Requires**: `fritzbox.network.enabled: true` in config.
Write operations additionally require `fritzbox.network.readonly: false`.

## Key Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `get_wlan` | WLAN radio status and settings | `wlan_index` (1=2.4 GHz, 2=5 GHz, 3=Guest) |
| `set_wlan` | Enable or disable a WLAN radio | `wlan_index`, `enabled` |
| `get_hosts` | List all known network hosts (IP, MAC, name, active) | — |
| `wake_on_lan` | Send Wake-on-LAN magic packet to a MAC address | `mac_address` |
| `get_port_forwards` | List all NAT port-forwarding rules | — |
| `add_port_forward` | Add a new port-forwarding rule | `external_port`, `internal_port`, `internal_client`, `protocol`, `description` |
| `delete_port_forward` | Remove a port-forwarding rule | `external_port`, `protocol` |

## Examples

```json
{"action": "fritzbox_network", "operation": "get_wlan", "wlan_index": 1}
```

```json
{"action": "fritzbox_network", "operation": "set_wlan", "wlan_index": 2, "enabled": false}
```

```json
{"action": "fritzbox_network", "operation": "get_hosts"}
```

```json
{"action": "fritzbox_network", "operation": "wake_on_lan", "mac_address": "AA:BB:CC:DD:EE:FF"}
```

```json
{"action": "fritzbox_network", "operation": "get_port_forwards"}
```

```json
{
  "action": "fritzbox_network",
  "operation": "add_port_forward",
  "external_port": "8080",
  "internal_port": "80",
  "internal_client": "192.168.1.50",
  "protocol": "TCP",
  "description": "Web server"
}
```

```json
{"action": "fritzbox_network", "operation": "delete_port_forward", "external_port": "8080", "protocol": "TCP"}
```

## Notes

- `wlan_index` defaults to 1 (2.4 GHz) if omitted.
- `protocol` for port forwarding must be `TCP` or `UDP` (case-insensitive; normalised to uppercase internally).
- `delete_port_forward` identifies the rule by `external_port` + `protocol`; all other fields are ignored.
- Wake-on-LAN only works if the target device supports WoL and its NIC is configured accordingly.
