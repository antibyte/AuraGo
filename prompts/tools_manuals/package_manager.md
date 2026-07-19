---
id: package_manager
tags: [system, packages]
priority: 80
conditions: ["allow_package_manager"]
---

## Tool: Package Manager (`package_manager`)

Manage local system packages with a structured cross-platform wrapper. The tool detects common package managers and uses the existing `sudo_password` vault secret for Linux operations that need root privileges.

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `operation` | string | yes | One of `detect`, `install`, `remove`, `update`, `upgrade`, `search`, `list_installed`, `info` |
| `package` | string | conditional | Required for `install`, `remove`, `search`, and `info`; optional for `upgrade` |
| `manager` | string | no | Optional override: `apt`, `dnf`, `yum`, `pacman`, `zypper`, `apk`, `brew`, `winget`, `choco`, or `scoop` |

### Operations

| Operation | Behavior |
|---|---|
| `detect` | Return the detected package manager without changing the system |
| `search` | Search available packages |
| `list_installed` | List installed packages |
| `info` | Show package metadata/details |
| `install` | Install a package |
| `remove` | Remove or uninstall a package |
| `update` | Refresh package metadata/sources |
| `upgrade` | Upgrade one package, or all packages when `package` is empty |

### Platform Behavior

- Linux: supports `apt`, `dnf`, `yum`, `pacman`, `zypper`, and `apk`. Mutating operations use `execute_sudo` internally with the vault key `sudo_password` and require system-wide writes to be explicitly enabled.
- macOS: supports Homebrew (`brew`) and runs as the current user.
- Windows: supports `winget`, `choco`, and `scoop` in that order.
- Docker: usually unavailable unless explicitly enabled and sudo is usable in the container.

### Examples

```json
{"action":"package_manager","operation":"detect"}
```

```json
{"action":"package_manager","operation":"search","package":"nginx"}
```

```json
{"action":"package_manager","operation":"info","package":"git"}
```

```json
{"action":"package_manager","operation":"install","package":"jq"}
```

### Safety Notes

- Prefer `detect`, `search`, `list_installed`, and `info` before changing packages.
- Confirm with the user before `install` or `remove` unless the user explicitly requested that change.
- `package_manager.readonly: true` blocks `install`, `remove`, `update`, and `upgrade`.
- Linux mutations require `agent.sudo_enabled: true`, `agent.sudo_unrestricted: true`, `sudo_password` stored in the vault, and a service environment without `NoNewPrivileges` or `ProtectSystem=strict`.
- When systemd hardening makes system paths read-only, keep using the read operations and ask the administrator to adjust and restart the service; do not fall back to raw `execute_shell` or `execute_sudo` package commands.
