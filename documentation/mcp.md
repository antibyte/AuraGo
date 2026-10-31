# MCP runtime

AuraGo's server exports only explicitly selected tool names. Empty server lists
deny all, including existing IDE bridge configurations. Apply the recommended
tools explicitly in Config and save; there is no automatic migration.
Client allowlists retain their separate semantics.
Overlapping catalog loads cannot replace a newer explicit preset selection.

Configuration and secret changes revoke the old manager before connecting the
replacement outside the configuration lock. Secrets are stored before the single
restart. Older generations cannot publish connections or dispatch new calls.
MCP saves share the configuration writer lock. Vault updates are batched; a failed
configuration save restores prior secrets, and a failed reload also restores the
previous configuration file. Rollback errors are reported rather than hidden.
Discovery transport failures retire only the affected connection; the next call
may reconnect. Tool calls already sent are never automatically replayed: a timeout
or disconnect can leave the remote action's outcome unknown.

Docker stdio accepts `docker_command` without a local `command`. The usual shell,
sandbox, Docker mutation and private-network permissions still apply. Automatically
configured Dograh connections participate in the same reload lifecycle.

Inbound RPC messages are limited to 32 MiB, discovery pages to 16 MiB and
complete discovery catalogs to 64 MiB (also at most 128 pages and 10,000 tools).
Limits include decoded HTTP bodies and complete SSE events. Oversized replies
close the connection; incomplete catalogs are never published. Process stderr
retains only its last 64 KiB.

Cancellation and deadlines cover lock waits and writes as well as response
reads. Calls without a deadline receive 60 seconds; an explicit caller deadline,
including longer llm-sandbox execution budgets, is preserved.
Cancellation while waiting for a discovery or write lock sends nothing and leaves
other requests on the shared connection running.

The server catalog uses the same feature prerequisites as the agent, including
the current configuration snapshot and available service handles. Enabled
integrations such as S3 still require an explicit export grant. Co-agents are
excluded; `ask_aurago` is available only in IDE mode and retains the export scope.

Incoming calls use the same argument decoder as native agent calls. Free-form
object fields advertised as JSON strings are decoded before normal tool dispatch.
`invoke_tool.arguments` accepts either an object or a string containing one complete
JSON object. Malformed, missing, null, array and scalar values are rejected instead
of silently invoking the target with empty parameters. The export allowlist,
capability gates, Guardian and action policy still apply to the resolved target.

Preferred vision tools receive MIME-checked images of at most 50 MiB. An explicit
base64 input takes precedence; URL inputs receive a data URL only when their schema
accepts it. Remote tools that accept only local paths are rejected without
publishing images. Local stdio receives a private staged file; Docker receives
its container path inside the existing workspace mount. Staged files are removed
after success, failure or cancellation. Discovery and execution share the caller's
context and the discovered manager/connection; a replacement requires rediscovery.
Schema validation resolves local references in their original document, including
`$defs` and `definitions`, and does not fetch external references.
