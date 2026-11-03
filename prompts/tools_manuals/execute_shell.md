---
id: execute_shell
tags: [core]
priority: 100
conditions: ["allow_shell"]
---

# Shell Execution (`execute_shell`)

Execute shell commands through the isolated executor by default. Explicitly authorized host execution uses PowerShell (`powershell.exe`) on Windows and `/bin/sh` on Unix.

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

- **Permissions and lifetime:** Both the current run and live server must authorize startup. Foreground commands stop with the run; accepted background processes retain their managed lifetime after the dispatch ends.
- **Command policy:** Literal executable paths, quoted executable names, Windows .exe names and env/assignment, command and BusyBox prefixes obey the same high-risk checks. `command -v`/`-V` queries and ordinary read-only `find` remain available; `find -exec`, `-execdir`, `-ok` and `-okdir` are refused. The env split-string modes are refused like nested interpreter commands. These checks supplement OS isolation and do not make arbitrary shell code safe.
- **Foreground timeout:** 30 seconds. Use `background: true` for longer tasks.
- **Background completion:** Call `wait_for_event` with `event_type: "process_exited"`, then inspect its exit code and log summary before reporting success. Do not poll with `sleep`.
- **Preserve exit codes:** Do not append `| tail` to background commands. AuraGo already bounds captured output with its ring buffer, while a pipeline can hide the actual command's failure.
- **Exact process/container names:** Use exact names or anchored patterns, for example `grep -x 'llama-cpp-vulkan'` or `^llama-cpp-vulkan:`. A name such as `ollama` must not count as a `llama` match.
- **Shell flags:** PowerShell runs with `-NoProfile -NonInteractive` for speed.
- **Working directory:** `agent_workspace`
- **Piping and redirection:** Standard shell operators (`|`, `>`, `>>`, `&&`, `||`) are supported.
- **Environment:** Host child processes receive a filtered environment, including removal of both AWS access-key IDs and secret/session keys. Isolated workers inherit no host environment.
- **Security:** Missing or null `agent.allow_unsandboxed_shell` never authorizes host execution, including after an upgrade. On Linux/macOS, explicit `agent.allow_unsandboxed_shell: true` selects the host shell path; `agent.allow_unsafe_host_execution: true` can also authorize it and is required for host execution on Windows. Landlock on the host path requires `shell_sandbox.enabled` and a functional Linux backend. If required isolation is unavailable, execution is blocked unless its unsafe fallback was explicitly allowed. Unisolated commands use the AuraGo process user's permissions; the command heuristic does not make that safe. Missing isolated-executor support never automatically selects the host path.
- **Desktop Notes protection:** Native Notes/file tools prevent changes to existing Notes. Permitted local shell, Python and skills still run when isolation is disabled or unsafe fallback was explicitly allowed. Such unisolated code can bypass the native file guards; do not claim Notes are protected against it. With active Landlock, a writable-path overlap with Notes refuses execution. Kernel support alone does not establish active isolation. Use `desktop_notes` for supported note operations.
