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
