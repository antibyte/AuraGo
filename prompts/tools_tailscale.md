---
id: "tools_tailscale"
tags: ["conditional"]
priority: 32
conditions: ["tailscale_enabled"]
---
### Tailscale Network Management
| Tool | Purpose |
|---|---|
| `tailscale` | Inspect and manage the Tailscale VPN: list devices, device details, subnet routes, DNS, ACL, local node status |

**Operations:**
- `devices` / `list` — List all tailnet devices with Tailscale IPs and online status
- `device` — Get full details for a specific device (requires `query`: hostname, IP, or node ID)
- `routes` — List advertised and enabled subnet routes for a device (requires `query`)
- `enable_routes` — Approve/enable subnet routes for a device (requires `query` + `value`: comma-separated CIDRs)
- `disable_routes` — Remove enabled subnet routes (requires `query` + `value`: comma-separated CIDRs)
- `dns` — Get tailnet DNS nameserver configuration
- `acl` — Get tailnet ACL policy document
- `local_status` — Query local Tailscale daemon status and peer list (only available if Tailscale runs on the same host)

**Parameters:** `operation`, `query` (hostname / MagicDNS name / Tailscale IP / node ID), `value` (comma-separated CIDR routes)
