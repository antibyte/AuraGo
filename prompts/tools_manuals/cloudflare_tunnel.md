# Cloudflare Tunnel Tool (`cloudflare_tunnel`)

Manage a Cloudflare Tunnel to expose local services to the internet securely.

## Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `start` | Start the tunnel | — |
| `stop` | Stop the tunnel | — |
| `restart` | Restart the tunnel | — |
| `status` | Get tunnel status | — |
| `quick_tunnel` | Start a quick (temporary) tunnel | `port` |
| `logs` | View tunnel logs | — |
| `list_routes` | List configured routes | — |
| `install` | Install cloudflared binary | — |

## Examples

```json
{"action": "cloudflare_tunnel", "operation": "status"}
```

```json
{"action": "cloudflare_tunnel", "operation": "quick_tunnel", "port": 8080}
```

```json
{"action": "cloudflare_tunnel", "operation": "start"}
```

```json
{"action": "cloudflare_tunnel", "operation": "logs"}
```

## Notes
- Supports Docker and native binary modes
- Token, named tunnel, and quick tunnel authentication
- Quick tunnels are temporary and get a random subdomain
