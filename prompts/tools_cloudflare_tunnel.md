---
id: "tools_cloudflare_tunnel"
tags: ["conditional"]
priority: 32
conditions: ["cloudflare_tunnel_enabled"]
---
### Cloudflare Tunnel Management
| Tool | Purpose |
|---|---|
| `cloudflare_tunnel` | Manage a Cloudflare Tunnel (cloudflared) to expose local services to the internet securely |

**Operations:**
- `start` — Start the tunnel using the configured auth method (token, named, or quick)
- `stop` — Stop the running tunnel (removes Docker container or kills native process)
- `restart` — Stop and re-start the tunnel
- `status` — Check current tunnel status, uptime, mode, and public URL (if quick tunnel)
- `quick_tunnel` — Start a temporary TryCloudflare tunnel (no account needed); optional `port` (defaults to web UI port)
- `logs` — Retrieve recent tunnel process logs (native mode only; Docker mode uses docker tool)
- `list_routes` — List currently configured ingress rules
- `install` — Download the cloudflared binary for the current platform (only needed for native mode)

**Parameters:** `operation` (required), `port` (integer, for quick_tunnel only)

**Auth Methods:**
- **token** — Connector token from Cloudflare dashboard (stored in vault as `cloudflared_token`)
- **named** — Named tunnel with credentials.json (stored in vault as `cloudflared_credentials`)
- **quick** — TryCloudflare quick tunnel, no Cloudflare account needed (temporary, random URL)

**Modes:** `auto` (Docker preferred, native fallback), `docker`, `native`
