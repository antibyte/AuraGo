---
id: execute_shell
tags: [core]
priority: 100
conditions: ["allow_shell"]
---

# Shell Execution (`execute_shell`)

Execute arbitrary shell commands on the host system. Uses PowerShell (`powershell.exe`) on Windows and `/bin/sh` on Unix.

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `command` | string | yes | — | Shell command to execute |
| `background` | boolean | no | `false` | Run in background; returns PID immediately |
| `notify_on_completion` | boolean | no | `false` | Receive notification when background process finishes |

## Examples

```json
{"action": "execute_shell", "command": "Get-ChildItem -Path ."}
```

```json
{"action": "execute_shell", "command": "uptime"}
```

```json
{"action": "execute_shell", "command": "make build", "background": true, "notify_on_completion": true}
```

## Notes

- **Foreground timeout:** 30 seconds. Use `background: true` for longer tasks.
- **Background completion:** Call `wait_for_event` with `event_type: "process_exited"`, then inspect its exit code and log summary before reporting success. Do not poll with `sleep`.
- **Preserve exit codes:** Do not append `| tail` to background commands. AuraGo already bounds captured output with its ring buffer, while a pipeline can hide the actual command's failure.
- **Exact process/container names:** Use exact names or anchored patterns, for example `grep -x 'llama-cpp-vulkan'` or `^llama-cpp-vulkan:`. A name such as `ollama` must not count as a `llama` match.
- **Shell flags:** PowerShell runs with `-NoProfile -NonInteractive` for speed.
- **Working directory:** `agent_workspace`
- **Piping and redirection:** Standard shell operators (`|`, `>`, `>>`, `&&`, `||`) are supported.
- **Environment:** Child processes receive a filtered environment; AuraGo master keys, API keys, tokens, passwords, and similar host secrets are not inherited.
- **Security:** This tool provides host shell access. Landlock isolation is used only when `shell_sandbox.enabled` is explicitly enabled and available on Linux; otherwise commands run with the AuraGo process user. Avoid destructive commands unless absolutely necessary.
