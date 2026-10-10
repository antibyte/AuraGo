# MCP runtime

AuraGo's server exports only explicitly selected tool names. Empty server lists
deny all, including existing IDE bridge configurations. Apply the recommended
tools explicitly in Config and save; there is no automatic migration.
Client allowlists retain their separate semantics.

Configuration and secret changes revoke the old manager before connecting the
replacement outside the configuration lock. Secrets are stored before the single
restart. Older generations cannot publish connections or dispatch new calls.
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
