---
id: execute_sudo
tags: [core]
priority: 100
conditions: ["sudo_enabled"]
---

## Tool: Privileged Shell Execution (`execute_sudo`)

Run a shell command with root privileges via `sudo`. The sudo password is stored in the vault and injected securely at the Go layer — it is **never passed as a tool parameter** and never appears in tool output.

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `command` | string | yes | Shell command to run as root (e.g. `iptables -L -n`, `cat /etc/shadow`) |

### Behavior

- Reads `sudo_password` from the vault automatically — you do **not** need to fetch it yourself.
- Executes the command via `sudo -S` with the password supplied via stdin.
- Returns combined stdout/stderr output.
- Timeout: 30 seconds.

### When to use

Use `execute_sudo` whenever a command requires root privileges:

- Reading privileged system files (`/etc/shadow`, `/proc/...`)
- Managing firewall rules (`iptables`, `ufw`)
- Starting/stopping system services (`systemctl`)
- Changing file ownership or permissions on system paths

### Examples

```json
{"action": "execute_sudo", "command": "iptables -L -n"}
```

```json
{"action": "execute_sudo", "command": "cat /etc/shadow"}
```

```json
{"action": "execute_sudo", "command": "systemctl restart nginx"}
```

### ⚠️ Critical: Do NOT use execute_python or execute_shell for sudo

Never attempt to run sudo via `execute_python` (e.g. `subprocess.run(['sudo', ...], input=password)`) or `execute_shell` (e.g. `echo password | sudo -S ...`). These approaches:

1. **Get blocked by the Security Guardian** — embedding credentials in code is flagged as a security violation
2. **Run inside the Python sandbox container** — which has the `no-new-privileges` kernel flag set, making sudo impossible regardless of the password
3. **Leak the password into tool parameters and logs**

`execute_sudo` is the only correct way to run privileged commands.

### Prerequisites

- `agent.sudo_enabled: true` in config
- `sudo_password` stored in the vault (store via `secrets_vault` → `store` operation)
- The `aurago` user must have sudo rights on the host system

### Unavailability

The tool is not shown in the schema when:
- Running inside a Docker container (`IsDocker = true`)
- The `no-new-privileges` kernel flag is set on the host process (`NoNewPrivileges = true`)
- `agent.sudo_enabled: false`

If sudo fails with "no new privileges", the system administrator needs to remove `no-new-privileges` from the systemd service or container configuration.
