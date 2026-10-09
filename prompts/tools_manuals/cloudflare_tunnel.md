# Cloudflare Tunnel Tool (`cloudflare_tunnel`)

Manage administrative named/token tunnels and temporary publication of registered Homepage projects.

## Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `start` | Start the tunnel | — |
| `stop` | Stop the tunnel | — |
| `restart` | Restart the tunnel | — |
| `status` | Get tunnel status | — |
| `quick_tunnel` | Publish a registered Homepage static snapshot | `project_dir` |
| `logs` | View tunnel logs | — |
| `list_routes` | List configured routes | — |
| `install` | Install cloudflared binary | — |

## Examples

```json
{"action": "cloudflare_tunnel", "operation": "status"}
```

```json
{"action": "cloudflare_tunnel", "operation": "quick_tunnel", "project_dir": "my-site"}
```

```json
{"action": "cloudflare_tunnel", "operation": "start"}
```

```json
{"action": "cloudflare_tunnel", "operation": "logs"}
```

## Notes
- Supports Docker and native binary modes
- Lifecycle actions use saved settings, serialize, and preserve failures. Restart will not start a replacement until stopping is confirmed.
- Running verified token/named Docker connectors are adopted after AuraGo restarts even without auto-start. Engine failures report unknown state; a running connector does not prove public reachability. Stop uses the original engine and container ID.
- Named tunnels require explicit `custom_ingress` routes with a hostname or path and an HTTP(S) origin; AuraGo appends one final 404 rule. Web UI/Homepage flags do not create named routes.
- Optional ports are 0 or integers 1–65535; low ports may need OS privileges. Metrics port 0 allocates a loopback port and does not disable metrics.
- TLS auto-configuration failures add sanitized warning codes; use loopback HTTP or configure the Cloudflare origin manually. TLS exceptions remain limited to the local AuraGo HTTPS origin.
- Managed Docker and automatic Linux amd64/arm64 installs pin cloudflared 2026.10.0. Automatic downloads are hash-checked and capped at 100 MiB; Windows/macOS require manual installation.
- Docker administrators can inspect token environment variables and mounted named credentials; restrict engine access.
- Quick Docker publication requires a local engine socket or a verified parent AuraGo container. Use native mode with remote engines or custom container hostnames.
- Token, named tunnel, and quick tunnel authentication
- Token tunnels use the Cloudflare dashboard-managed connector token from the Vault key `cloudflared_token`; AuraGo starts `cloudflared tunnel run` and does not add a local `--url` for token mode
- Quick tunnels are temporary and get a random subdomain
- Quick tunnels require an active registered Homepage project. Build it first; AuraGo snapshots its static output (up to 64 MiB), allocates the origin port, and publishes only those files. Explicit ports and Web UI fallbacks are rejected.
- `project_dir` also selects the project for quick `start` and `restart`; when omitted, the administrator-selected `cloudflare_tunnel.quick_project_dir` is used. All quick entry points use the same boundary.
- Both integrations must be enabled and Cloudflare must be writable. Archiving a project revokes serving immediately; stopping the tunnel or shutting down cleans up the snapshot. Administrative named/token ingress remains separate.
