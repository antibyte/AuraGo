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
A high-risk call that copies a 48-character span from an earlier isolated tool
body still waits for a one-shot approval, including when the policy is off.
An allow rule does not skip that wait. A still-valid approval for the exact
arguments does. Short original arguments and reads stay on the existing path.
The comparison uses the call arguments as text (every string value, decoded)
against the isolated tool output as the model saw it. Both sides have HTML
entities resolved and every run of whitespace, line breaks included, collapsed
to one space, so a hard-wrapped or re-flowed copy still matches. Each session
keeps the full isolated output of its recent tool calls, up to 256 KiB of text,
and drops the oldest first; one output beyond that keeps its start. The session
todo and guardian bookkeeping fields do not count. A value that holds
percent-encoding, such as a form-encoded body, is compared in its decoded form
too; other encodings such as Base64 are not covered. When many sessions are
active, the one unused for the longest time loses its record first. An unattended copy stays
blocked, and the refusal does not offer an allow rule as the remedy. An
interactive copy up to 256 KiB can still be approved once. The dialog preview is
shortened to 8 KiB and begins with the size and SHA-256 of the whole call and of
each long text field (such as the copied body), so the approver can see how much
the preview leaves out; approval runs the exact call. A larger copy is refused.
Existing endpoint/method/path/action grants in treg remain authoritative.

Outbound fetches and probes (`api_request`, `web_scraper`, `web_capture`, the
browser, crawler and monitor tools, `dns_lookup`, `network_ping`, `port_scanner`,
`whois_lookup`) are mostly not high-risk, but the destination sees their URL or
host name. In an unattended run the same 48-character test therefore covers that
URL or host. Case and percent-encoding are ignored. A target that appears whole
in an earlier output is a link the model followed and passes. A target that
carries a span of earlier output (a query string built from a mail, a host name
made of a memory entry) stops with `action_rule_required` until an allow rule
names the tool and target. This check is part of the action policy: with
`enabled: false` it does not run. The three host probes `dns_lookup`,
`network_ping` and `port_scanner` go further: a host that carries a copied span is
high-risk in every run type (see "What counts as high-risk"), so an interactive
run asks for approval, and an allow rule does not lift an unattended one.

The approval dialog and the LLM Guardian are told which external tools the
session read lately, for example `untrusted content read earlier in this
session: web_scraper, fetch_email`. The record holds the last eight tools whose
non-quarantined output reached the model within the past hour. It informs the
decision and never blocks anything.

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

## What counts as high-risk

Operation names containing destructive or outbound words (delete, remove, kill,
reboot, restart, send, upload, publish, deploy and similar) are high-risk. So are
these calls, which a keyword cannot see. Interactively each opens the approval
dialog. Unattended runs stop with `action_rule_required` until a rule covers them.

- **Scheduling.** `manage_missions` create, add, update, edit, run, run_now and
  execute, `cron_scheduler` and `manage_schedule` add, enable and update, and every
  `schedule_cron` call. A stored mission or cron prompt runs later with the full
  tool catalog and nobody watching. Rules and previews name these calls with the
  operation `schedule`. These tools have no decoded target, so the preview shows
  `tool:manage_missions` and similar; copy it into the rule. Listing, history and
  disabling stay ordinary. `follow_up` and `wait_for_event` queue a model-written
  prompt for a later background run; they get the same treatment in unattended
  runs only. In an interactive chat they stay ordinary, because the user is there
  and a chain of follow-ups is how the agent works through a long task. In a
  follow-up, mqtt, heartbeat, mission or other unattended run they stop with
  `action_rule_required` unless a rule for the tool, the operation `schedule` and
  the target `tool:follow_up` (or `tool:wait_for_event`) exists. A chain that
  continues itself from a follow-up run therefore needs that rule.
- **Actuators.** `home_assistant` `call_service`/`service` on the domains `lock`,
  `alarm_control_panel`, `cover`, `script`, `automation`, `shell_command`, `siren`
  and `valve`, and on the generic `homeassistant`, `hassio`, `rest_command`,
  `python_script` and `pyscript` domains, which reach any entity or run code.
  Lights, switches, media players, climate and fans keep working without a prompt.
  `docker` `stop` and `pause` (`restart` and `exec` already were).
- **Reaching outside the host.** A per-tool table in `action_policy_risk.go` names
  the operations that publish, share, expose a local service, create a remote object
  that carries text, point a remote service at a new endpoint, hand work to an
  external agent or machine, hand over a credential or start a call. Examples:
  `here_now_site` update, duplicate, update_access and set_password; `onedrive`
  share and write; `github` create_issue and create_or_update_file;
  `netlify`/`vercel` set_env and hooks or domains; `cloudflare_tunnel` start;
  `manage_outgoing_webhooks` create and update; `s3_storage` copy and move (a copy
  across buckets can land in a bucket that is served publicly); `homepage_project`
  and `homepage_deploy` webserver_start and tunnel; `agentmail_inboxes`
  create_inbox; `virtual_workspace` request_credential_grant; `virtual_computers`
  run_shell_task and run_desktop_task. A bare `update` or `create` is not a
  keyword: private and local state stays ordinary. A new tool operation of this
  kind has to be added to the table, and a test fails for any operation of the
  audited tools that is neither a read, nor high-risk, nor on the list of
  operations judged ordinary.
- **Code in a virtual machine.** `virtual_workspace` exec, start_job and job_input
  and `virtual_computers` exec run commands inside the sandboxed machine, which has
  its own network profile (internet and LAN by default). A user who watches the
  chat and the workspace gets no prompt. An unattended run needs a rule, because
  those commands could otherwise reach any host.
- **DNS side channel.** `dns_lookup`, `network_ping` and `port_scanner` are
  high-risk when the host looks like data rather than a name: a label longer than
  24 characters; a label of 16 or more characters made of digits only, of hex, of
  letters and digits that alternate, or of letters that are spread widely and have
  few vowels (base32, base36, random letters); the labels left of the last two read
  as one text when there are several short ones; or a whole name longer than 64
  characters. A hyphen does not hide anything: `mfrg-ggzd-fmzt-wq2l-knnw` is read as
  one label, and whole numeric parts of up to three digits (`ec2-203-0-113-25`) are
  ignored. A host that copies 48 characters of earlier isolated tool output is
  high-risk in every run type: interactively it opens the approval dialog, and an
  unattended run is refused even with an allow rule. A name that appears whole in an
  earlier output is a name the model followed and passes. The resolver forwards each
  label to the zone's authoritative server, so such a name can carry data out. The
  shape test is a heuristic, not a guarantee: a host spelled in short words still
  passes, and a host allowlist is the structural answer. Ordinary hostnames, IP
  literals and reverse-lookup names are not affected.
- **Secrets in arguments.** For calls that send something off the host
  (`api_request`, `send_*`, `call_webhook`, mail sending, `dns_lookup`,
  `network_ping`, `port_scanner`, web fetch and browser tools), a registered secret
  value in any string argument refuses the call with `egress_secret_refused`. No
  rule, approval or disabled policy lifts it; the log keeps the arguments hash and
  never the value. A value that only looks like a credential (`token=...`, a bearer
  token, `user:password@` in a URL) makes the call high-risk instead. Recipient
  fields are not scanned, because an SMTP user name is a registered value.

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
