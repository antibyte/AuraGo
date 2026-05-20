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
- In native function-calling sessions, use `discover_tools` to check whether `virustotal_scan` is active, hidden, disabled, or exposed as a skill.
- In text-JSON sessions, `list_tools` only shows custom Python tools. Use `list_skills` when you specifically need to discover registered skills.
- If `discover_tools` reports `virustotal_scan` as a skill, call it with `execute_skill`. If it reports a native tool, use the returned `call_method`.
