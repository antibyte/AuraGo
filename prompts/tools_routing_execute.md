---
id: tools_routing_execute
tags: [conditional]
priority: 31
conditions: ["allow_shell"]
---
### Local Shell Execution (Danger Zone)
⚠️ NEVER use `execute_shell` for SSH or remote connections. For remote servers use `execute_remote_shell`.
| Tool | Purpose |
|---|---|
| `execute_shell` | Run a LOCAL shell command (PowerShell/sh). NOT for remote servers |
| `execute_sudo` | Run a LOCAL command with root privileges via sudo. NEVER use `sudo` inside `execute_shell` |
