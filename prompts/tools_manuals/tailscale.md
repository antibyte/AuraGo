# tailscale — Tailscale VPN Management

Inspect and manage your Tailscale network via the Tailscale API v2.

> **Requires** `tailscale.enabled: true` and a Tailscale API key (`tskey-api-…`) in `config.yaml` under `tailscale.api_key` or `TAILSCALE_API_KEY` env var.
> The `tailscale.tailnet` field is optional — use `"-"` or leave blank for the default/personal tailnet.

---

## Operations

### devices
List all devices in the tailnet with their Tailscale IPs and online status.

```json
{"action": "tailscale", "operation": "devices"}
{"action": "tailscale", "operation": "list"}
```

Returns: `{count, devices: [{id, hostname, name, addresses, os, last_seen, online}]}`

---

### device
Get full details for a specific device by hostname, MagicDNS name, Tailscale IP, or node ID.

```json
{"action": "tailscale", "operation": "device", "query": "my-server"}
{"action": "tailscale", "operation": "device", "query": "100.64.0.1"}
{"action": "tailscale", "operation": "device", "query": "my-server.tailXXXXX.ts.net"}
```

---

### routes
List the advertised and currently enabled subnet routes for a device.

```json
{"action": "tailscale", "operation": "routes", "query": "my-router"}
```

Returns: `{device_id, routes: {advertisedRoutes: [...], enabledRoutes: [...]}}`

---

### enable_routes
Approve and enable specific subnet routes for a device.
`value` is a comma-separated list of CIDR prefixes.

```json
{"action": "tailscale", "operation": "enable_routes", "query": "my-router", "value": "192.168.1.0/24"}
{"action": "tailscale", "operation": "enable_routes", "query": "my-router", "value": "10.0.0.0/8,172.16.0.0/12"}
```

---

### disable_routes
Remove specific subnet routes from the enabled set for a device.

```json
{"action": "tailscale", "operation": "disable_routes", "query": "my-router", "value": "192.168.1.0/24"}
```

---

### dns
Get the DNS nameserver configuration for the tailnet (MagicDNS / custom resolvers).

```json
{"action": "tailscale", "operation": "dns"}
```

---

### acl
Retrieve the full ACL policy document for the tailnet (JSON format).

```json
{"action": "tailscale", "operation": "acl"}
```

---

### local_status
Query the Tailscale daemon running **on the same host as AuraGo** for real-time connection status and peer list. Returns an error if Tailscale is not installed locally.

```json
{"action": "tailscale", "operation": "local_status"}
{"action": "tailscale", "operation": "status"}
```

Returns: `{backend_state, self: {hostname, dns_name, ips, online}, peers: [{hostname, ips, online, active}], peer_count}`

---

## Parameter Reference

| Parameter | Used By | Description |
|---|---|---|
| `operation` | All | Operation to perform (required) |
| `query` | device, routes, enable_routes, disable_routes | Hostname, MagicDNS name, Tailscale IP (e.g. `100.x.x.x`), or node ID |
| `value` | enable_routes, disable_routes | Comma-separated CIDR prefixes (e.g. `10.0.0.0/8,192.168.1.0/24`) |

---

## Configuration

```yaml
tailscale:
  enabled: true
  api_key: "tskey-api-xxxxxxxxxxxxxxxx"  # Tailscale API key
  tailnet: ""                             # Leave blank for default tailnet, or set e.g. "yourcompany.com"
```

The API key can also be set via the `TAILSCALE_API_KEY` environment variable.

> **Note:** The `local_status` operation does not require an API key — it queries the local Tailscale daemon directly. All other operations use the Tailscale cloud API.
