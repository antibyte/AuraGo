# Tool handlers

## Purpose

Agent filesystem, external service and Docker tool safety boundaries.

## Ownership

`internal/tools` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### GitHub repository trust
- Workspace project metadata, including `AgentCreated`, is inventory only. Runtime trust comes from explicit administrator allowlists or the protected, API-bound `github_trust.json` ledger. Legacy inventory requires visible administrator approval. Preserve a private backup and atomically replace ledger updates; malformed ledgers fail closed without being overwritten.
- Repository deletion requires `github.allow_delete` (default false), repository access and writable mode. A create response followed by a ledger failure reports the created repository plus a migration warning; never suggest retrying the creation.

### treg catalog gateway
- `treg_catalog`, `treg_call` and `treg_status` use the fixed public-only treg transport. Never accept model-supplied origins, authentication, cost headers or polling URLs. Already-sent calls are not replayed after ambiguous failures.
- Grants bind endpoint ID, method, path and an explicit read/create/update/delete class. Check the current catalog and run/live policy before requests; use the smaller run/live cost cap, including zero. Keep `treg_token` Vault-only and excluded from Python exports.
- Continuations bind token, session and original grant; status is pending until the provider confirms completion. Reserved, charged and unknown amounts stay distinct. Media downloads carry no treg credentials; uploads obey workspace and protected-path checks.
- The bounded process-local continuation cache is not a billing store. Preserve call IDs in tool history for accounting after restart. See `documentation/treg.md`; verify `TestTreg*` across config, tools, agent, server and UI.

### Bundled Newspaper editorial skill
- Register `aurago-newspaper` through the Agent Skill Manager under the `newspaper` owner. Verify its bundled hash before use; do not let a disk edit silently replace the trusted guide. The guide can plan bounded searches and edit server-supplied evidence but cannot execute arbitrary tools, send or change the publication profile.

### Brave research adapter
- `SearchBrave` is the typed, single-request Web/News adapter with freshness, pagination and sanitized rate-limit metadata. Its caller owns pacing/retries and budgets. Keep the native `ExecuteBraveSearch` Web behavior, isolated result text and native tool schema compatible; server-only News options do not widen the agent tool schema. Verify `TestBraveNewsPaginationAndNativeCompatibility`, `TestBraveRateLimitErrorsAndCancellation` and existing Brave/DDG callers.
- Verify with `go test ./internal/tools -run TestNewspaperBundled`.

### Security Egress and Host Execution
- Every Cloudflare quick entry point publishes one active registered Homepage project's immutable static snapshot, at a server-allocated port. Reject explicit ports, Web UI defaults, inactive/unregistered projects, symlinks and credential files; bound copying to 64 MiB. Named/token ingress remains administrative. Docker requires a local engine socket or a verified parent container namespace. The origin is host-bound, supports read-only HTTP and rechecks project revocation; uncertain start/termination retains a disabled listener until the original daemon or process confirms termination. Record the request and resulting URL plus snapshot hash in the Homepage ledger. Verify `TestHomepageQuick*`, `TestCloudflareQuick*` and the configuration browser test.
- Cloudflare's automatic certificate exception belongs only to the configured local AuraGo HTTPS origin. Never set global `originRequest.noTLSVerify`; migrate that legacy default to the matching ingress rule and preserve unrelated provider configuration and origin options. Named tunnel YAML follows the same origin match. Verify `TestCloudflareAPIExceptionAppliesOnlyToAuraGoOrigin` and `TestCloudflareNamedExceptionAppliesOnlyToAuraGoOrigin`.
- Local Ansible performs effective context-bound shell/unsafe-host checks at the execution point, including check, status, inventory and facts. Preserve the selected sandbox and filtered environment. Stage bounded, rooted playbook snapshots and static inventories before execution; no absolute-path or executable-inventory bypass. SSH host-key checking defaults on. Verify `TestAnsibleLocalOperationsRequireEffectiveShellGrant` and `TestAnsibleInputsSnapshotAndRejectUncontrolledPaths`.
- Process termination requires the shell grant and a live process handle owned by the background registry; never terminate an arbitrary PID. Listing and statistics retain their read behavior.
- Dependency installation is host code execution. Provisioning and reuse pass the run context and effective Python/shell/unsafe-host gates before venv creation or pip. Accept package requirements only, rejecting URLs, paths, pip flags and requirements files. Prepared shell/native skills follow the selected shell sandbox; service operations use bounded foreground execution and filtered environments.
- Cast staging reads through `OpenToolInputFile`, publishes unique synced snapshots, and issues a ticket for that exact basename. A later upload must not replace bytes reachable through an older ticket.
- Verify `TestPackageRequirementsAndHostGrants`, `TestManageProcessesUsesOnlyRegisteredHandle`, `TestCastPublishedMediaRetainsContentsWhenSourceNameIsReused` and the existing sandbox/foreground-runner tests.
- Venv creation and waiting for the shared initializer honor the provisioning/skill context, with a two-minute ceiling and foreground process-tree cleanup. Verify `TestCancelledVenvWaitDoesNotCreateEnvironment` and concurrent/lazy-first-use tests.
- Windows shell and every host Python execution path, including Agent Skill scripts and background Python jobs, require `agent.allow_unsafe_host_execution` in addition to their existing tool gate; each allowed run emits an audit warning. Linux shell keeps its sandbox policy.
- Daemon skills apply the same Python/Shell and host-execution gates at start and restart. Revoking a gate stops affected running daemons before another restart can be scheduled. Starts and restarts also require a plain skill executable (no traversal, no symlink) and, while the Skill Manager is active, an enabled registry entry whose security status allows execution and whose file hash is unchanged. `tools.skill_manager.require_sandbox` denies daemons because they run only on the host. Each daemon gets its own process group and RLIMIT_AS, but no CPU-time limit and no RLIMIT_NPROC (Linux counts it per user including AuraGo's own threads); stop and max runtime kill the whole tree. Verify `TestDaemon*` and, on Linux, `TestDaemonStopKillsProcessGroupAndAppliesDaemonLimits`.
- Direct tool gates read the server's published snapshot through `SetRuntimePermissionResolver` (bound in `internal/server/runtime_permissions.go`). Only startup and server config publication call `ConfigureRuntimePermissions`, which is the standalone/test fallback. Agent runs narrow a single dispatch with `WithRuntimePermissions` and can never widen the server snapshot. New gate call sites that have a dispatch context use the `...Context(ctx)` gate variant. A new `RuntimePermissions` field must be added to `intersectRuntimePermissions` (`TestIntersectRuntimePermissionsCoversEveryField`). Verify `TestRuntimePermission*` and `internal/audit` `TestRuntimePermissionsAreWrittenOnlyByStartupAndServer`.
- Docker ownership checks resolve all protected owners with one inspect and one list request. For container-targeted agent operations they fail closed: when Docker answers neither, the docker tool returns `docker_ownership_unverified` instead of running the operation. `DockerContainerManagedBy` treats unverifiable ownership as managed. Verify `TestDockerContainerOwnership*` and `TestDispatchDockerBlocksContainerOpsWhenOwnershipUnverified`.
- Tool error envelopes use `ErrorJSON`/`ErrorJSONf`; do not interpolate `%v`/`%s` into a hand-built JSON string (ratchet: `internal/audit` `TestToolErrorJSONIsNotHandFormatted`).
- Network MCP servers reject private addresses unless that server grants `allow_private_network`; pin DNS for each connection and reject cross-origin redirects and SSE message endpoints.
- Browser automation requires a nonempty sidecar token and an attested `egress-v1` sidecar. A managed sidecar starts only on a verified Docker-internal network with an explicit filtering proxy; browser navigation, resources, and WebSockets share the egress policy.
- Writable Landlock sessions require an ABI that handles `TRUNCATE`; older kernels fail closed unless the existing explicit unsafe-host gate authorizes fallback. Docker bind mounts use canonical host paths and reject protected targets.

### Agent Filesystem Jail Contract
- Native editors and MissionV2 stores use `internal/fileutil` for replacement;
  keep their caller-owned path/permission checks, locks, and file modes. Rooted tool writes and filesystem `copy` replace
  their destination through `writeRootFromReaderAtomic` (temporary file, Sync, checked Close, rename inside the `os.Root`).
- Agent filesystem, file_editor, and other `secureResolve` paths jail to `agent_workspace`, not the AuraGo install root. From `workdir`, `../skills` and `../tools` stay reachable; `../../config.yaml` and `data/` must fail resolution.
- Keep absolute and relative path checks equivalent. Perform filesystem reads, writes, walks and replacements through `os.Root` after resolution; do not hand a validated workspace path to an external converter. Media conversion, PDF operations, Docker copy and patching stage inputs in private temporary directories and publish workspace outputs through the rooted writer. Document conversion and MCP vision stage workspace inputs before handing paths to other components. Docker copy accepts regular files only.
- Koofr uploads read only from `agent_workspace`; never fall back to the runtime `data` directory.
- `tools.IsProtectedSystemPath` is defense-in-depth: case-insensitive, symlink-resolved, and blocks `directories.data_dir`, configured config/vault/sqlite paths, `.env` files, and `aurago_master.key`. The agent dispatcher's `isProtectedSystemPath` delegates to it, and `secureResolve` applies it to every resolved path through `RuntimePermissions.ProtectedDataDir`/`ProtectedSystemFiles`; the file-name checks apply even without a runtime snapshot. Recursive walks (archive create, file search) only resolve their root through `secureResolve`.
- `data/tresor.db` and its WAL/SHM sidecars are listed through `config.SQLiteProtectedPaths`; agent file tools must never read or modify them. Host shell execution outside isolation can bypass native guards.
- Media registry and video-download bounds still use the install root via `detectAuraGoInstallRoot`. Guardian must not label `../../` as a safe in-project path.

### Agent Docker Inspect Contract
- Agent `docker inspect` requires the Docker runtime permission. Its environment redacts `AURAGO_*` and keys ending in `_PASSWORD`, `_SECRET`, `_TOKEN`, `_API_KEY`, `_ACCESS_KEY`, `_PRIVATE_KEY`, or `_MASTER_KEY`, and passes other values through `security.RedactSensitiveInfo` (URL credentials). `Cmd` masks the value of credential flags (`--password x`, `--requirepass x`, `--token=x`); `Labels` masks values whose key names a password, secret, token, key, basic-auth or credential and redacts URL credentials in the rest; ownership labels such as `aurago.managed` stay visible. Administrator container APIs share this redacted view and may still inspect the AuraGo app container. Verify `TestDockerInspectRedacts*` and `TestDockerInspectRequiresDockerPermission`.
- The agent docker tool must hide and block inspect, lifecycle, log, exec, and copy access to the compose app container `aurago` (including compose-project prefixed replicas). Sidecars such as `aurago-local-llm`, `aurago_gotenberg`, and `aurago-homepage` keep their existing owner gates.

### Homepage Dev Container
- Create `aurago-homepage` with Docker `HostConfig.Init=true` so orphaned Chromium processes are reaped.
- An existing legacy container without init stays in place and reports `rebuild_required`; replace it only through an explicit Homepage rebuild because replacement interrupts dev servers and changes active Cloudflare quick-tunnel URLs. The workspace bind mount survives that rebuild.

### Managed Space Agent Contract
- Keep the default upstream Git ref pinned to a reviewed commit across config loading, the managed sidecar fallback, the Config UI, and the reference config. Explicitly configured refs remain user-controlled.
- Build the AuraGo-injected Space Agent image from the configured upstream ref before replacing an existing sidecar. Fetch and checkout failures must stop the update and leave the existing container intact.
- Run Space Agent's supervisor with automatic upstream releases disabled; otherwise a downloaded release can omit AuraGo's injected `/api/message_async` endpoint.
- Keep Space Agent auth keys under the persistent `space_agent.data_path` mount through `SPACE_AUTH_DATA_DIR`. Admin password and bridge token remain Vault-only; Space Agent provider credentials remain separate from AuraGo.

### Mission and Home Assistant Lifetimes
- Mission manager StartContext inherits the server lifetime; repeated Start is idempotent. Stop is terminal, refuses further work, and waits for the queue, invocation callbacks, timeout guards and completion work. Invocation callbacks must use the manager Context and must not detach their work.
- Mission HTTP handlers remain independent of a caller disconnect, but inherit server shutdown. Their registry cancels and drains every generation before database closure, including replaced runs. History completion finishes within the tracked invocation.
- Home Assistant polling owns one cancellable generation. Publish changed credentials/URL/enablement before draining outside config locks and starting a replacement. Shutdown drains polling before closing mission databases. Entity IDs use the shared path encoder and credentials cannot follow a foreign-origin redirect.
- Verify mission lifecycle and Home Assistant regression tests in tools and server; run Linux CGO race tests for concurrent cancellation.

### Integration state preservation
- Frigate config/config_raw returns a conservative parsed projection: strings, unknown scalar fields and secret-bearing sections are redacted. Preserve only allowlisted numeric/boolean settings and structure. Unsupported YAML, aliases, duplicate keys or parse failures have no raw fallback; upstream error bodies never become config output. Frigate credentials and printer camera requests cannot follow foreign-origin redirects.
- AdGuard filtering toggles must read and preserve the configured update interval; missing or malformed status forbids the write.
- Mission webhook callbacks must be keyed by mission ID, replaced on trigger changes and removed on disable/delete/shutdown.
- Uptime Kuma pollers inherit an owner context. Stop is terminal, cancels I/O and waits for completion; callbacks must not detach unbounded work.
- Python Vault export requires both agent-created provenance and the system-key blocklist. Reserve actual `sql_`, `cloudflared_`, `cloudflare_` and `three_d_printer_klipper_` integration prefixes alongside legacy names.

## Verification

- Hugging Face repository IDs are validated by the shared canonical validator
  before permission checks and HTTP path construction. Mutations require an
  explicit namespace/repository. Encoded/traversal identities never grant access.
- Composio execution requires successfully fetched matching tool metadata.
  Global or toolkit read-only gates override tool allowlists. Custom API keys
  remain at their configured origin across redirects; validate base URL syntax.

- Run `go test ./internal/tools` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
