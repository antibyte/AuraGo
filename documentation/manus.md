# Manus v2 integration

AuraGo integrates directly with the asynchronous Manus v2 API through one native agent tool named `manus`. The first version focuses on private agent tasks, bounded polling, follow-up messages, stopping tasks, and controlled file transfers. OAuth, webhooks, public sharing, account-wide task listing, task deletion, and Manus account administration are intentionally excluded.

## Setup

1. Open **Config → External AI → Manus**.
2. Store the Manus API key in the encrypted Vault. AuraGo uses the Vault key `manus_api_key`; the key is never serialized into `config.yaml` or exported to Python tools.
3. Enable Manus while leaving **Read-only mode** on for initial testing.
4. Test the connection and verify the displayed credit balance.
5. Load and select the allowed projects, connectors, and skills. AuraGo also loads project-specific skills for already selected projects and whenever you select another project.
6. Enable only the granular operations the agent actually needs. Read-only mode overrides all of them.

Manus API keys have full account access. Rotate a key immediately if it is exposed outside the Vault.

## Security model

- The API base is fixed to `https://api.manus.ai`, and authentication uses `x-manus-api-key`.
- AuraGo stores only its own Manus task IDs plus non-sensitive task metadata in `data/manus.db`. Messages, file content, and foreign account tasks are not persisted.
- Every new task is private. Connectors and explicit skills are checked against local allowlists.
- Remote text and errors are scrubbed and isolated as untrusted external data before the agent sees them.
- Structured output schemas are validated locally before submission, including local references, enums, closed objects, arrays, and the five-level depth limit. Results are accepted only when Manus marks extraction as successful.
- Reads may retry rate limits. Mutations are never retried automatically because the API has no idempotency key for these operations. Unknown outcomes are marked `retry_safe: false`; a confirmed remote mutation with a failed local ledger update is returned as `partial_success` with its task ID and URL.
- `wait_for_task` polls in five-second intervals by default and is capped at 60 seconds. AuraGo does not run a permanent poller or expose a webhook receiver.

## Files

Uploads must originate from the AuraGo agent workspace. AuraGo keeps the same workspace-rooted file handle from validation through upload and rejects symlinks, path escapes, executable mode or magic bytes, configuration, Vault, database, script, and executable files. Presigned uploads must use HTTPS, remain on public network addresses, obey redirect limits, and stay below the configured size limit.

Downloads are discovered only through attachments of a locally tracked task. The client returns bounded bytes, and the runtime writes them through a workspace-rooted filesystem handle below `agent_workspace/workdir/manus/<task-id>/` with exclusive, sanitized filenames. Downloads are never executed automatically.

## Waiting and approvals

When Manus asks a user question, the native tool returns `needs_user_input`. Other confirmation events return `needs_human_approval` with the private Manus task URL. AuraGo does not expose Manus action-confirmation calls in V1; the user approves those operations directly in Manus. Manus reports both ordinary completion and explicit user stops with the remote status `stopped`; AuraGo normalizes those cases to `completed` and `stopped` by checking non-verbose events for `user_stop`. Terminal messages are sanitized and returned with the normalized state.

The configuration UI combines account skills with cached, ID-deduplicated skills from selected projects. The configuration endpoint may inspect a project before it is allowlisted so you can select its skills; the agent-facing `list_skills` operation and every explicit skill request remain restricted to the configured project and skill allowlists. If no explicit skill list is sent, Manus may load account-default skills according to the API's behavior.

See the [Manus v2 API documentation](https://open.manus.im/docs/v2/introduction) for the upstream service contract.
