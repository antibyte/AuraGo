# Tool handlers

## Purpose

Agent filesystem and Docker tool safety boundaries.

## Ownership

`internal/tools` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Security Egress and Host Execution
- Windows shell and every host Python execution path, including Agent Skill scripts and background Python jobs, require `agent.allow_unsafe_host_execution` in addition to their existing tool gate; each allowed run emits an audit warning. Linux shell keeps its sandbox policy.
- Network MCP servers reject private addresses unless that server grants `allow_private_network`; pin DNS for each connection and reject cross-origin redirects and SSE message endpoints.
- Browser automation requires a nonempty sidecar token and an attested `egress-v1` sidecar. A managed sidecar starts only on a verified Docker-internal network with an explicit filtering proxy; browser navigation, resources, and WebSockets share the egress policy.

### Agent Filesystem Jail Contract
- Agent filesystem, file_editor, and other `secureResolve` paths jail to `agent_workspace`, not the AuraGo install root. From `workdir`, `../skills` and `../tools` stay reachable; `../../config.yaml` and `data/` must fail resolution.
- `isProtectedSystemPath` is defense-in-depth: case-insensitive, symlink-resolved, and blocks `directories.data_dir`, configured config/vault/sqlite paths, `.env` files, and `aurago_master.key`.
- Media registry and video-download bounds still use the install root via `detectAuraGoInstallRoot`. Guardian must not label `../../` as a safe in-project path.

### Agent Docker Inspect Contract
- Agent `docker inspect` environment redacts `AURAGO_*` and keys ending in `_PASSWORD`, `_SECRET`, `_TOKEN`, `_API_KEY`, `_ACCESS_KEY`, `_PRIVATE_KEY`, or `_MASTER_KEY`. Administrator container APIs may still inspect the AuraGo app container.
- The agent docker tool must hide and block inspect, lifecycle, log, exec, and copy access to the compose app container `aurago` (including compose-project prefixed replicas). Sidecars such as `aurago-local-llm`, `aurago_gotenberg`, and `aurago-homepage` keep their existing owner gates.

### Homepage Dev Container
- Create `aurago-homepage` with Docker `HostConfig.Init=true` so orphaned Chromium processes are reaped.
- An existing legacy container without init stays in place and reports `rebuild_required`; replace it only through an explicit Homepage rebuild because replacement interrupts dev servers and changes active Cloudflare quick-tunnel URLs. The workspace bind mount survives that rebuild.

### Managed Space Agent Contract
- Keep the default upstream Git ref pinned to a reviewed commit across config loading, the managed sidecar fallback, the Config UI, and the reference config. Explicitly configured refs remain user-controlled.
- Build the AuraGo-injected Space Agent image from the configured upstream ref before replacing an existing sidecar. Fetch and checkout failures must stop the update and leave the existing container intact.
- Run Space Agent's supervisor with automatic upstream releases disabled; otherwise a downloaded release can omit AuraGo's injected `/api/message_async` endpoint.
- Keep Space Agent auth keys under the persistent `space_agent.data_path` mount through `SPACE_AUTH_DATA_DIR`. Admin password and bridge token remain Vault-only; Space Agent provider credentials remain separate from AuraGo.

## Verification

- Run `go test ./internal/tools` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
