# AdGuard Home (`adguard`)

Manage AdGuard Home DNS server: filtering, rewrites, clients, DHCP, and statistics.

## Operations

| Operation | Description |
|-----------|-------------|
| `status` | Server status |
| `stats` | Aggregated statistics |
| `stats_top` | Top clients/domains |
| `query_log` | DNS query log |
| `filtering_status` | Filter list status |
| `filtering_toggle` | Enable/disable filtering |
| `filtering_add_url` | Add filter list URL |
| `filtering_remove_url` | Remove filter list |
| `filtering_set_rules` | Set custom filtering rules |
| `rewrite_list` | List DNS rewrites |
| `rewrite_add` | Add DNS rewrite |
| `rewrite_delete` | Remove DNS rewrite |
| `blocked_services_list` | List blocked services |
| `blocked_services_set` | Set blocked services |
| `clients` | List all clients |
| `client_add` | Add client |
| `dns_info` | DNS server settings |
| `dhcp_status` | DHCP server status |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of the operations above |
| `query` | string | for query_log | Search query for DNS log |
| `limit` | integer | for query_log | Max results (default: 10) |
| `offset` | integer | for query_log | Pagination offset |
| `enabled` | boolean | for filtering_toggle | Enable/disable filtering |
| `name` | string | for filtering_add_url | Filter list name |
| `url` | string | for filtering_add_url, filtering_remove_url | Filter list URL |
| `rules` | string | for filtering_set_rules | Newline-separated custom rules |
| `domain` | string | for rewrite_add, rewrite_delete | Domain name |
| `answer` | string | for rewrite_add, rewrite_delete | DNS answer (IP address) |
| `services` | array | for blocked_services_set | Array of service names |
| `content` | string | for client_add | JSON client configuration |

## Examples

**Check server status:**
```json
{"action": "adguard", "operation": "status"}
```

**Query DNS log:**
```json
{"action": "adguard", "operation": "query_log", "query": "youtube.com", "limit": 10}
```

**Add DNS rewrite:**
```json
{"action": "adguard", "operation": "rewrite_add", "domain": "myapp.local", "answer": "192.168.1.100"}
```

**Block services:**
```json
{"action": "adguard", "operation": "blocked_services_set", "services": ["tiktok", "facebook"]}
```

**Set custom filtering rules:**
```json
{"action": "adguard", "operation": "filtering_set_rules", "rules": "||ads.example.com^\n||tracking.example.com^"}
```

## Configuration

```yaml
adguard:
  enabled: true
  url: "http://adguard.home:3000"  # AdGuard Home URL
  username: "admin"                  # Username (stored in vault)
  password: ""                       # Password (stored in vault)
```

## Notes

- **DNS rewrites**: Useful for local DNS resolution (e.g., `myapp.local` → `192.168.1.100`)
- **Filter rules**: Follow AdGuard filter rule syntax (e.g., `||domain.com^` blocks domain)
- **Blocked services**: Pre-defined services like "tiktok", "facebook" that can be blocked at DNS level
- **Query log**: Shows DNS queries processed by AdGuard — useful for troubleshooting
