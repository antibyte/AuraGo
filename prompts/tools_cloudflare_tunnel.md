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
- `quick_tunnel` — Publish an immutable static snapshot of an active registered Homepage project; optional `project_dir` selects the project
- `logs` — Retrieve recent native logs; Docker mode points to the Docker log tool
- `list_routes` — List currently configured ingress rules
- `install` — Install the pinned, checksum-verified Linux amd64/arm64 native binary (100 MiB limit); other platforms require manual installation

**Parameters:** `operation` (required), `project_dir` (optional quick project selector). Explicit ports are rejected.

**Auth Methods:**
- **token** — Dashboard-managed connector token (stored in vault as `cloudflared_token`); starts `cloudflared tunnel run` without a local `--url`
- **named** — Named tunnel with credentials.json (stored in vault as `cloudflared_credentials`)
- **quick** — TryCloudflare quick tunnel, no Cloudflare account needed (temporary, random URL)

**Modes:** `auto` (Docker preferred, native fallback), `docker`, `native`

Quick publication requires both integrations enabled and an active registered Homepage project with built static output. AuraGo snapshots up to 64 MiB and assigns a private origin port; Web UI defaults and explicit ports are rejected. Named tunnels require explicit hostname/path HTTP(S) ingress routes plus one automatic final 404. Token routes remain dashboard-managed.

Running verified token/named Docker containers are adopted after AuraGo restarts. Status reports unknown state on engine failures; connector startup does not prove public reachability. Restart requires confirmed termination. TLS configuration failures add warnings; use loopback HTTP or manual origin settings. Docker administrators can read runtime container secrets.
