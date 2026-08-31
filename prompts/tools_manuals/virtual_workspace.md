# virtual_workspace

Create and control a stateful agent workspace inside a boring-computers Firecracker VM. Commands run as root inside the guest, never on the AuraGo host. AuraGo binds every workspace to the trusted current chat or mission; owner identifiers are not accepted from tool arguments.

## Lifecycle

- Use `open` with `template=desktop` when a visible browser is needed, or `template=python` for shell-only work.
- Workspaces are ephemeral by default. `checkpoint` creates an explicit `workspace_v2` volume that contains only `/workspace`.
- Use `get` and `list` to inspect state. Always use `close` when the work is complete; closing stops jobs and browser sessions, revokes grants, clears runtime state, and destroys the VM.
- A missing workspace-agent capability returns `workspace_agent_upgrade_required`. Never fall back silently to legacy Boring LLM tasks.

## Shell and files

- `exec` is for bounded synchronous commands. Use `start_job` for long-running work, servers, or PTY sessions.
- Continue a job with `job_status`, cursor-based `job_output`, and `job_input`. Output pages are capped at 65536 bytes. Cancel process groups with `cancel_job`.
- The default working directory is `/workspace`. Structured file operations (`list_files`, `read_file`, `write_file`, `upload`, `download`) are confined there.
- Jobs may live for at most the configured workspace/job limits. Active jobs extend the idle lease but never the absolute workspace lifetime.

## Credentials

- `list_grantable_credentials` returns metadata only, never secret values.
- `request_credential_grant` creates a user-visible approval request. The agent cannot approve it.
- Shell grants bind to one job. Browser grants bind to one workspace and exact HTTP(S) origin.
- Grants expire, are revoked with `revoke_credential_grant`, and never survive job end, workspace close, or checkpoint.

## Safety

The VM may access only the configured public/LAN network profile. Host services, metadata endpoints, other guests, and unapproved private networks remain blocked. Do not place credentials or browser profiles under `/workspace`.
