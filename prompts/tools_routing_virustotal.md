---
id: tools_routing_virustotal
tags: [conditional]
priority: 31
conditions: ["virustotal_enabled"]
---
### VirusTotal Scans
| Tool | Purpose |
|---|---|
| `virustotal_scan` | Scan URLs, domains, IPs, or file hashes using VirusTotal |

Notes:
- Do NOT use `list_tools` to check whether VirusTotal exists. `list_tools` only shows custom Python tools.
- If `virustotal_scan` is not exposed as a direct built-in action in the current tool list, use `list_skills` and then `execute_skill` for the `virustotal_scan` skill.
