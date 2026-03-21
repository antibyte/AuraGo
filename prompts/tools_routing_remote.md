---
id: tools_routing_remote
tags: [conditional]
priority: 31
conditions: ["allow_remote_shell"]
---
### Remote Servers & SSH
⚠️ NEVER use `execute_shell` or `execute_python` for SSH, remote commands, or key generation.
| Tool | Purpose |
|---|---|
| `execute_remote_shell` | Run a command on a remote server via SSH (auto-auth via vault) |
| `transfer_remote_file` | Upload/download files via SFTP |
| `wake_on_lan` | Send a Wake-on-LAN magic packet to wake up a device (requires MAC address to be registered) |
