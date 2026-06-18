# AgoDesk Coding Agent: Remote Shell Implementation

Implement AgoDesk-side remote shell support against the AuraGo WebSocket protocol. Remote shell must be a local, explicit AgoDesk setting. If the user has not enabled it in AgoDesk config, the client must not advertise or execute shell commands.

## Contract

Advertise shell support in `session.start` only when local AgoDesk config enables it:

```json
{
  "client_capabilities": ["remote.shell.exec"],
  "shell_access": {
    "enabled": true,
    "requires_approval": true,
    "default_cwd": "~",
    "allowed_cwds": [
      {
        "cwd_id": "workspace",
        "label": "Workspace",
        "path_display": "~/Projects/AuraGo"
      }
    ],
    "shells": ["powershell", "cmd"],
    "max_command_chars": 4000,
    "max_output_bytes": 1048576,
    "default_timeout_ms": 30000,
    "max_timeout_ms": 120000
  }
}
```

AuraGo may then send `desktop.command.operation=shell_exec`:

```json
{
  "command_id": "cmd-shell-1",
  "operation": "shell_exec",
  "params": {
    "command": "git status --short",
    "cwd_id": "workspace",
    "timeout_ms": 30000
  }
}
```

Return `desktop.result` with `ok=true` when the command was launched and completed, even if the command itself exits non-zero:

```json
{
  "command_id": "cmd-shell-1",
  "ok": true,
  "data": {
    "exit_code": 0,
    "stdout": " M src/main.ts\n",
    "stderr": "",
    "duration_ms": 145,
    "timed_out": false,
    "truncated": false,
    "cwd_display": "~/Projects/AuraGo",
    "shell": "powershell"
  }
}
```

Return `ok=false` for policy, approval, validation, spawn, timeout, or protocol failures. Use stable error codes: `SHELL_ACCESS_DISABLED`, `SHELL_ACCESS_DENIED`, `SHELL_APPROVAL_REQUIRED`, `SHELL_APPROVAL_DENIED`, `SHELL_COMMAND_REJECTED`, `SHELL_TIMEOUT`, `SHELL_OUTPUT_TOO_LARGE`, and `SHELL_SPAWN_FAILED`.

Do not implement `shell_exec_stream` in this v1 protocol.

## Required AgoDesk Changes

1. Add a local remote-shell settings model with:
   - `enabled`
   - `requiresApproval`
   - stable `cwdId`
   - display label
   - canonical absolute working directory path
   - `pathDisplay`
   - selected shell
   - max command characters
   - max output bytes
   - default and max timeout milliseconds
2. Include `remote.shell.exec` in `session.start.client_capabilities` only when `enabled=true`.
3. Include `shell_access` in `session.start`, keeping canonical paths local and sending only display paths.
4. Store `session.accepted.advertised_capabilities` and execute `shell_exec` only when it contains `remote.shell.exec`.
5. Handle `desktop.command.operation=shell_exec`.
6. If `requiresApproval=true`, show a local approval prompt with command, cwd label/display, timeout, and origin. Run the command only after approval.
7. Return `desktop.result` with stable shell error codes for disabled config, denied cwd, missing approval, rejected command, timeout, output overflow, or spawn failure.
8. Write audit logs without stdout, stderr, environment variables, secrets, keychain paths, or shell history.

## Command Validation

For every `shell_exec`:

1. Re-read local shell config at command time. If disabled, return `SHELL_ACCESS_DISABLED`.
2. Verify `remote.shell.exec` was negotiated in `session.accepted.advertised_capabilities`.
3. Validate `command` is a non-empty string and not longer than `max_command_chars`.
4. Reject commands matching local deny rules before spawning a shell.
5. Resolve `cwd_id` to a configured shell working directory. If `cwd_id` is absent and `cwd` is present, accept it only when its canonical path is inside one configured shell working directory.
6. Canonicalize the configured root and selected working directory before execution.
7. Reject `..` traversal, drive-prefix tricks, UNC escapes, and symlinks that resolve outside the allowed working directory.
8. Clamp `timeout_ms` to `default_timeout_ms` and `max_timeout_ms`.
9. Capture stdout and stderr separately.
10. Enforce `max_output_bytes` across stdout plus stderr. Prefer safe truncation with `truncated=true`; if safe truncation is not possible, terminate and return `SHELL_OUTPUT_TOO_LARGE`.
11. Kill the process tree on timeout and return `ok=false`, `error:"SHELL_TIMEOUT"`, and `data.timed_out=true` when useful.

## Execution Rules

- Windows default shell should be PowerShell with non-interactive flags. Support `cmd` only when selected in local settings.
- Linux/macOS default shell should be `/bin/sh` or the configured shell from local settings.
- Do not inherit secret-bearing environment variables from AgoDesk. Start from a minimal environment and add only safe platform defaults needed for commands to run.
- Do not support interactive stdin in v1.
- Do not support background processes in v1; commands must end within the effective timeout.
- Do not silently fall back to another working directory or shell when validation fails.
- Do not expose canonical local paths in protocol payloads unless the user explicitly configured that display path.

## Suggested Type Shape

```ts
type ShellAccessCwd = {
  cwdId: string;
  label: string;
  canonicalPath: string;
  pathDisplay: string;
};

type ShellAccessConfig = {
  enabled: boolean;
  requiresApproval: boolean;
  defaultCwd?: string;
  allowedCwds: ShellAccessCwd[];
  shells: Array<"powershell" | "cmd" | "sh" | "bash" | "zsh">;
  selectedShell: "powershell" | "cmd" | "sh" | "bash" | "zsh";
  maxCommandChars: number;
  maxOutputBytes: number;
  defaultTimeoutMs: number;
  maxTimeoutMs: number;
};

type ShellExecParams = {
  command: string;
  cwd_id?: string;
  cwd?: string;
  timeout_ms?: number;
};

type ShellExecResult = {
  exit_code: number;
  stdout: string;
  stderr: string;
  duration_ms: number;
  timed_out: boolean;
  truncated: boolean;
  cwd_display: string;
  shell: string;
};
```

## UI Requirements

- Put remote shell behind an explicit local config toggle. Default must be off.
- Show clear copy that remote shell allows AuraGo to run commands on this computer.
- Require at least one allowed working directory before enabling the feature.
- If `requiresApproval=true`, show a command approval prompt before each run and include Deny, Run, and Stop current session controls.
- Show connection/status UI based on negotiated capability: configured locally, advertised, accepted by AuraGo, or unavailable.
- If AuraGo does not negotiate `remote.shell.exec`, keep the local setting but display that the server policy currently disables remote shell.

## Acceptance Criteria

- AgoDesk does not advertise `remote.shell.exec` when local remote shell config is disabled.
- `session.start.shell_access.enabled` matches the local setting.
- `shell_exec` is rejected when `remote.shell.exec` was not negotiated.
- `shell_exec` works in an allowed cwd and returns stdout, stderr, exit code, duration, timeout flag, truncation flag, cwd display, and shell name.
- Non-zero command exit codes return `ok=true` with the non-zero `exit_code`.
- Commands outside configured working directories are denied.
- Symlinks that escape an allowed working directory are denied.
- Commands longer than `max_command_chars` are rejected.
- Commands exceeding `max_output_bytes` are truncated safely or rejected with `SHELL_OUTPUT_TOO_LARGE`.
- Timeouts kill the process tree and return `SHELL_TIMEOUT`.
- Approval denial returns `SHELL_APPROVAL_DENIED`.
- Logs and audits never contain stdout, stderr, environment variables, secrets, or shell history.
- Older AuraGo servers without `remote.shell.exec` still keep chat, persona assets, desktop control, and file access working.
