---
id: execute_shell
tags: [core]
priority: 100
conditions: ["allow_shell"]
---

## Tool: Shell Execution (`execute_shell`)

Execute arbitrary shell commands on the host system. Uses PowerShell (`powershell.exe`) on Windows and `/bin/sh` on Unix.

### Schema

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `command` | string | yes | — | Shell command to execute |
| `background` | boolean | no | `false` | Run in background; returns PID immediately |
| `notify_on_completion` | boolean | no | `false` | Receive notification when background process finishes |

### Behavior

- **Foreground timeout:** 30 seconds. Use `background: true` for longer tasks.
- **Shell flags:** PowerShell runs with `-NoProfile -NonInteractive` for speed.
- **Working directory:** `agent_workspace`
- **Piping and redirection:** Standard shell operators (`|`, `>`, `>>`, `&&`, `||`) are supported.

### Examples

```json
{"action": "execute_shell", "command": "Get-ChildItem -Path ."}
```

```json
{"action": "execute_shell", "command": "uptime"}
```

```json
{"action": "execute_shell", "command": "make build", "background": true, "notify_on_completion": true}
```

> **Security:** This tool provides full shell access. Avoid destructive commands unless absolutely necessary.