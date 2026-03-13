---
id: "tools_adguard"
tags: ["conditional"]
priority: 31
conditions: ["adguard_enabled"]
---
### AdGuard Home
| Tool | Purpose |
|---|---|
| `adguard` (operation=`status`) | Get AdGuard Home server status (version, running state, DNS addresses) |
| `adguard` (operation=`stats`) | Get DNS statistics (total queries, blocked, avg processing time) |
| `adguard` (operation=`stats_top`) | Get top clients, top queried domains, top blocked domains |
| `adguard` (operation=`query_log`) | Search DNS query log (params: `search`, `limit`, `offset`) |
| `adguard` (operation=`query_log_clear`) | Clear the entire DNS query log |
| `adguard` (operation=`filtering_status`) | Get filtering config and enabled filter lists |
| `adguard` (operation=`filtering_toggle`) | Enable/disable DNS filtering (param: `enabled`) |
| `adguard` (operation=`filtering_add_url`) | Add a filter list by URL (params: `name`, `url`) |
| `adguard` (operation=`filtering_remove_url`) | Remove a filter list by URL (param: `url`) |
| `adguard` (operation=`filtering_refresh`) | Force-refresh all filter lists |
| `adguard` (operation=`filtering_set_rules`) | Set custom filtering rules (param: `rules`, newline-separated) |
| `adguard` (operation=`rewrite_list`) | List all DNS rewrite rules |
| `adguard` (operation=`rewrite_add`) | Add a DNS rewrite (params: `domain`, `answer`) |
| `adguard` (operation=`rewrite_delete`) | Delete a DNS rewrite (params: `domain`, `answer`) |
| `adguard` (operation=`blocked_services_list`) | List available/blocked services |
| `adguard` (operation=`blocked_services_set`) | Set which services to block (param: `services` array) |
| `adguard` (operation=`safebrowsing_status`) | Check safe browsing status |
| `adguard` (operation=`safebrowsing_toggle`) | Enable/disable safe browsing (param: `enabled`) |
| `adguard` (operation=`parental_status`) | Check parental control status |
| `adguard` (operation=`parental_toggle`) | Enable/disable parental control (param: `enabled`) |
| `adguard` (operation=`dhcp_status`) | Get DHCP server config and active leases |
| `adguard` (operation=`dhcp_set_config`) | Update DHCP config (param: `config` JSON) |
| `adguard` (operation=`dhcp_add_lease`) | Add a static DHCP lease (params: `mac`, `ip`, `hostname`) |
| `adguard` (operation=`dhcp_remove_lease`) | Remove a static DHCP lease (params: `mac`, `ip`, `hostname`) |
| `adguard` (operation=`clients`) | List known clients and settings |
| `adguard` (operation=`client_add`) | Add a known client (param: `config` JSON) |
| `adguard` (operation=`client_update`) | Update a known client (param: `config` JSON) |
| `adguard` (operation=`client_delete`) | Delete a known client (param: `name`) |
| `adguard` (operation=`dns_info`) | Get current DNS configuration |
| `adguard` (operation=`dns_config`) | Update DNS server settings (param: `config` JSON) |
| `adguard` (operation=`test_upstream`) | Test upstream DNS servers for reachability (param: `services` array of DNS servers) |

**Notes:**
- All operations use a single `adguard` tool with an `operation` parameter
- Read-only operations: status, stats, stats_top, query_log, filtering_status, rewrite_list, blocked_services_list, safebrowsing_status, parental_status, dhcp_status, clients, dns_info, test_upstream
- Write operations require `adguard.readonly: false` in config
- The `search` parameter for query_log filters by domain name
- DNS rewrites map a domain to an IP address (or another domain via CNAME)
- The `services` parameter for blocked_services_set is an array of service IDs (e.g. `["tiktok", "facebook"]`)
- DHCP and client config use raw JSON matching the AdGuard Home API schema
