# Agent action rules and approvals

Capability toggles, read-only settings, role restrictions and a run's tool scope
are binding. An action rule or one-shot approval cannot enable a disabled
capability or override those restrictions. Permissions are checked again
immediately before dispatch, including approval and Tool Bridge execution.

`agent.action_policy` adds rules for risky actions such as sending messages,
uploading private files, destructive operations and unsafe host commands. It is
enabled by default. Administrators manage it in **Configuration → Danger Zone**.
The model cannot edit these rules through configuration tools.

## Unattended actions

Use the permission preview to inspect a concrete tool call before scheduling it.
The preview uses the complete enabled catalog, independently of the agent's
adaptive tool list. It returns the resolved tool, operation, target, run type,
scope and arguments hash without executing or creating an approval request.

For example, this explicitly permits removing one container in one mission:

```yaml
agent:
  action_policy:
    enabled: true
    allow:
      - tool: docker
        operation: remove
        # Synthetic complete container ID; copy the real ID from the preview.
        target: "container_id:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        run_type: mission
        scope_id: cleanup-fixture
    deny: []
```

Copy the exact canonical fields from the preview. Target strings can represent
integration IDs, recipients, URLs without credentials/query parameters, or
absolute workspace paths. The wildcard `*` matches every value of an entire
tool, operation or target field; there are no partial path globs. Omitting
`run_type` or `scope_id` allows matching runs of any corresponding type. An
optional lowercase SHA-256 `arguments_hash` restricts a rule to the exact
arguments. Prefer narrow rules; deny rules take precedence over allow rules.
Existing endpoint/method/path/action grants in treg remain authoritative.

Docker `exec` and Ansible ad-hoc/module/command or playbook execution require
explicit action authority; merely enabling a writable integration does not
permit arbitrary commands. Compatibility operations use their canonical names
in rules and previews (`docker rm` → `remove`, Ansible `command`/`run_module` →
`adhoc`, and `run`/`run_playbook` → `playbook`). Ordinary isolated Shell/Python
execution retains its existing sandbox and unsafe-host policy.

Targets come from the arguments the tool actually executes. Unrelated fields
cannot substitute for a container, recipient, connection or file. Docker's
`name` container alias resolves to the same `container_id:` target; remote
inventory names resolve to their stored device ID. File targets use canonical
absolute paths, including existing symlink parents. URL targets exclude user
credentials, query parameters and fragments.

Prefer complete immutable Docker container IDs. Container names and ID prefixes
may alias the same object; without a Docker lookup, a concrete container deny
blocks potentially equivalent selectors conservatively. Only two different
complete IDs establish different containers. A name-only narrow allow also
needs the exact arguments hash when identity cannot be verified. This can block
an otherwise unrelated name or prefix; use complete IDs to disambiguate it.

Calls with several targets have a copyable, sorted target set, for example
`targets:["file:/workspace/from.txt","file:/workspace/to.txt"]`. This covers
both ends of a move, every batch path, all To/Cc/Bcc recipients and attachment
sources. A deny for **any** component blocks the call. Allow rules must cover
**every** component, or match the complete target-set string; `target: "*"`
explicitly covers them all. Every contributing rule must also match the tool,
operation, run/scope and its optional arguments hash.

When the effective identity cannot be determined safely (for example recipients
of a stored remote draft or an unsupported tool-specific target), narrow labels
cannot grant authority. Use a preview's exact `arguments_hash` with an appropriate
target rule, or explicitly use `target: "*"` for the whole action. A matching concrete deny
remains blocking when the actual target cannot be verified, even alongside a
broad allow. This requires a supported decoder or a reviewed exact-arguments
rule to disambiguate; adding an arbitrary ID or session field cannot bypass it.

A mission, flow or background task without a required rule stops with
`action_rule_required`. It releases its execution slot and shows the blocked
state and diagnosis. Review the rule and retry explicitly. The system never
generates repeated unattended approval dialogs or automatically replays an
action with an uncertain external outcome.

## Interactive decisions

An interactive high-risk call without a matching rule opens an administrator
approval dialog using the existing action ID and SSE stream. The server stores
the proposed arguments; the browser submits only that ID and approve/reject.
Each decision is valid once, for ten minutes. Stop, shutdown, expiry and reject
invalidate it. Restart discards pending decisions. Current configuration and
scope restrictions are checked again before the stored call executes.

The administrator APIs are `GET/POST /api/agent/actions`,
`POST /api/agent/action-policy/preview` and
`GET /api/agent/action-policy/catalog`. The actions endpoint uses the originating
`X-Session-ID`; administrators can list all pending sessions with `?all=1`.
Persistent action audits contain hashes and scrubbed summaries, not the full
proposed arguments. Approval results can report uncertainty; verify external
state before retrying after an interrupted mutation.

## Execution and queue defaults

Shell, Python, stored tools and supported skill scripts use the existing
SandboxManager supervisor by default. Missing isolation blocks execution.
Unsafe host execution requires the separate explicit administrator exception
and still passes action policy. See [isolated execution](isolated-executor.md)
for pinned images, resource bounds and the network-free Tool Bridge broker.

New cron IDs are limited to 1,000. Existing jobs and seconds syntax remain
valid, and replacement/removal works at the limit. Background defaults are
256 active tasks, 64 starts per tick, 30 days of completed/cancelled retention,
90 days of failed retention, and 2,000 terminal entries. These bounds are under
`agent.max_cron_jobs` and `agent.background_tasks` in the configuration template.

Fresh installations enable a 5 USD daily budget in `warn` mode. Existing
explicit budget settings are preserved. Background model calls share the
tracker, finite output limits and physical-response accounting with foreground
calls. Cache-token observations and estimated costs remain separate.

## External master-key file

Set `AURAGO_MASTER_KEY_FILE` to a private file containing the existing
64-character hexadecimal master key. An explicit `AURAGO_MASTER_KEY` takes
precedence. Startup initializes the Vault before starting child processes,
retains the key privately for trusted updater use, and removes it from the
process environment. This does not rotate the key or rewrite existing secret
files. Restrict file permissions using the host's normal account controls.
