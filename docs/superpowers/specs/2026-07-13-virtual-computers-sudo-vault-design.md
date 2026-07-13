# Virtual Computers Sudo Vault Design

## Goal

Allow AuraGo to install and repair Boring Computers automatically in `local_host` mode when the service account needs password-based sudo. The Virtual Computers configuration page must reuse an existing central sudo password without requiring re-entry.

## Existing Behavior

AuraGo already stores a shared sudo password under the Vault key `sudo_password`. Users can manage it through `/sudopwd` or the generic Secrets page. Virtual Computers currently ignores that secret: its local preflight only accepts root or passwordless sudo, and its installer invokes `sudo -n`.

## User Experience

When Virtual Computers is enabled and `control_plane.mode` is `local_host`, the control-plane section shows a password field labeled **Sudo password** with Save and Clear actions.

- If `sudo_password` exists, the field remains empty and shows **Stored in Vault**. The user does not need to enter it again.
- Saving a non-empty value replaces the shared `sudo_password` secret.
- Clearing requires the explicit Clear action and removes the shared secret.
- The UI explains that this is the same password used by `/sudopwd` and other privileged AuraGo operations.
- The field is not shown in `ssh_host` mode because SSH credentials remain the source of remote elevation.
- Status, success, empty-input, save failure, and clear confirmation text must be translated in all supported languages: cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, and zh.

## Secret Storage and API Contract

The implementation reuses exactly one Vault key: `sudo_password`. No password field is added to `config.yaml` or to public configuration structures.

The existing authenticated Vault mutation API remains the only browser write path. The Virtual Computers setup-status response adds a boolean `sudo_password_stored`; it never returns the value, length, hash, or any derived password material. This boolean lets the page render the stored state immediately and refresh it after Save or Clear.

The setup-status endpoint remains protected by Desktop read scope. Vault mutations retain their existing authentication and authorization checks.

## Backend Data Flow

For `local_host` setup only:

1. The server reads `sudo_password` directly from the Vault when constructing the local setup executor.
2. The password exists only in process memory for the duration of preflight or installation.
3. Preflight checks sudo using `sudo -S -p "" true` with the password written to stdin. If the process is already root or `sudo -n true` succeeds, no password is required.
4. Installation runs the generated setup script using `sudo -S -p "" bash <temporary-script>` and supplies the password through stdin.
5. Error text and captured output pass through existing secret-redaction paths. The password is never placed in command arguments, environment variables, generated scripts, logs, audit payloads, or API responses.

For `ssh_host`, behavior is unchanged. The remote SSH account must already be root or have passwordless sudo because the selected SSH credential is not repurposed as a sudo password.

`agent.sudo_enabled` is not required. That setting controls whether the AI agent may call `execute_sudo`; `virtual_computers.auto_setup` plus Desktop admin authorization separately controls managed installation.

## Executor Boundary

`virtualcomputers.LocalCommandExecutor` gains an optional sudo password and a stdin-capable command runner boundary. It preserves current behavior in this order:

1. Run directly when effective UID is root.
2. Use passwordless `sudo -n` when it succeeds.
3. Otherwise use `sudo -S -p ""` when a Vault password is available.
4. Report the existing unsupported preflight result when neither elevation path succeeds.

Temporary scripts retain mode `0700` and are removed after execution.

## Error Handling

- Missing Vault: status reports `sudo_password_stored=false`; setup continues to test root/passwordless sudo.
- Missing password with password-required sudo: preflight reports that root, passwordless sudo, or a stored sudo password is required.
- Invalid password: setup returns a sanitized sudo authentication failure and must not echo the password.
- Vault read failure: setup returns a safe Vault availability error without leaking internal secret data.
- Clear action: only removes `sudo_password`; it does not change `agent.sudo_enabled` or Virtual Computers configuration.

## Testing

Backend tests must prove:

- local preflight accepts a correct stdin password and rejects a missing one;
- local script execution uses stdin and never command arguments or environment variables;
- root and passwordless-sudo behavior remains compatible;
- Virtual Computers reads the existing `sudo_password` key for local setup;
- SSH-host setup does not read or use the local sudo password;
- setup status exposes only the boolean stored state;
- setup/install error output redacts the password.

UI tests must prove:

- the field appears only for `local_host`;
- an existing stored state does not require re-entry;
- Save and Clear use the central `sudo_password` Vault key;
- no password is serialized into `configData` or the normal configuration save payload;
- all translation keys exist for all supported languages.

The final verification includes focused backend/UI tests, `go vet`, the full Go test suite, a production binary build in `disposable/`, a secret scan, and GitNexus change detection before commits.

## Out of Scope

- Changing sudoers or granting operating-system privileges automatically.
- Storing sudo passwords per integration.
- Reusing SSH login passwords as sudo passwords.
- Enabling `agent.sudo_enabled` automatically.
- Resolving unrelated port conflicts in existing user configuration.
