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
- **Security:** This tool provides host shell access. Landlock isolation requires `shell_sandbox.enabled` and a functional Linux backend. If enabled but unavailable, execution is blocked unless unsafe fallback is explicitly allowed. With isolation disabled, commands use the AuraGo process user's permissions, subject to separate protection rules. Avoid destructive commands unless absolutely necessary.
- **Desktop Notes protection:** Protected Notes can block local shell, Python and skill execution when Landlock isolation is **not active**. This refusal does not mean Landlock is enabled. Kernel support alone also does not establish active isolation. Use `desktop_notes` for notes or an isolated virtual workspace for code execution; do not claim Landlock blocked the command when the error identifies the separate Notes guard.
