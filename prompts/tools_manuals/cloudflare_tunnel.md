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
- Quick Docker publication requires a local engine socket or a verified parent AuraGo container. Use native mode with remote engines or custom container hostnames.
- Token, named tunnel, and quick tunnel authentication
- Token tunnels use the Cloudflare dashboard-managed connector token from the Vault key `cloudflared_token`; AuraGo starts `cloudflared tunnel run` and does not add a local `--url` for token mode
- Quick tunnels are temporary and get a random subdomain
- Quick tunnels require an active registered Homepage project. Build it first; AuraGo snapshots its static output (up to 64 MiB), allocates the origin port, and publishes only those files. Explicit ports and Web UI fallbacks are rejected.
- `project_dir` also selects the project for quick `start` and `restart`; when omitted, the administrator-selected `cloudflare_tunnel.quick_project_dir` is used. All quick entry points use the same boundary.
- Both integrations must be enabled and Cloudflare must be writable. Archiving a project revokes serving immediately; stopping the tunnel or shutting down cleans up the snapshot. Administrative named/token ingress remains separate.
