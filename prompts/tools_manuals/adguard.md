# AdGuard Home Tool (`adguard`)

Manage AdGuard Home DNS server: filtering, rewrites, clients, DHCP, and statistics.

## Key Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `status` | Server status | — |
| `stats` | Aggregated statistics | — |
| `stats_top` | Top clients/domains | — |
| `query_log` | DNS query log | `query`, `limit`, `offset` |
| `filtering_status` | Filter list status | — |
| `filtering_toggle` | Enable/disable filtering | `enabled` |
| `filtering_add_url` | Add filter list URL | `name`, `url` |
| `filtering_remove_url` | Remove filter list | `url` |
| `filtering_set_rules` | Set custom rules | `rules` (newline-separated) |
| `rewrite_list` | List DNS rewrites | — |
| `rewrite_add` | Add DNS rewrite | `domain`, `answer` |
| `rewrite_delete` | Remove DNS rewrite | `domain`, `answer` |
| `blocked_services_list` | List blocked services | — |
| `blocked_services_set` | Set blocked services | `services` (array) |
| `clients` | List all clients | — |
| `client_add` | Add client | `content` (JSON config) |
| `dns_info` | DNS server settings | — |
| `dhcp_status` | DHCP server status | — |

## Examples

```json
{"action": "adguard", "operation": "status"}
```

```json
{"action": "adguard", "operation": "query_log", "query": "youtube.com", "limit": 10}
```

```json
{"action": "adguard", "operation": "rewrite_add", "domain": "myapp.local", "answer": "192.168.1.100"}
```

```json
{"action": "adguard", "operation": "blocked_services_set", "services": ["tiktok", "facebook"]}
```

```json
{"action": "adguard", "operation": "filtering_set_rules", "rules": "||ads.example.com^\n||tracking.example.com^"}
```
