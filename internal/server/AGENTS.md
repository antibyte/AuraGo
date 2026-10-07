# Server integrations

## Purpose

Server-owned HTTP and cross-component integration contracts.

## Ownership

`internal/server` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### HTTP trust and shutdown boundaries

- Desktop agent chat, its stream and log APIs require administrative Desktop access. Log tail/search/stream/download scrub registered secrets and credential fields. Passive media proxies reject active HTML/SVG/XML; every inline Knowledge document uses sandbox CSP. Garage HTTP and WebSocket proxies remove local cookies and authorization headers before forwarding.
- Logout is same-origin POST and persistently revokes the presented session until expiry. Nonce-bearing sessions remain independent; revocation persistence failure does not report success. Login combines bounded per-IP state with account verification concurrency/backoff rather than letting one attacker globally lock the owner out.
- Read TokenManager through one synchronized snapshot per operation. Stop HTTP acceptance, cancel and drain active handlers/WebSockets, then close their dependencies. Re-panic `http.ErrAbortHandler`; never write a replacement JSON response over a partial proxy response.
- Cast LAN listeners require a file-specific ticket, reject listing and keep bounded header/idle/stream-write budgets. Store preview calls a configured loopback origin with no redirects; request Host cannot choose it.
- Verify codequality boundary/lifecycle tests, Remote handshake tests and the existing Desktop token/proxy matrix. A local fixture does not establish external delivery or device acceptance.

### Code Studio

- Save and upload share `code_studio_files.go`: bounded file bytes travel as a
  single generated regular staging file through the Docker archive API. Never
  embed contents in exec arguments. The normal container user verifies the
  staged digest and installs through no-follow directory descriptors.
- File PUT accepts additive `create_only`; New File must use it. Exclusive
  creation returns HTTP 409 on collision; overwrite publishes a complete sibling
  file atomically and preserves executable mode. Failed transfers leave the
  target intact. Keep Desktop/Docker write gates, workspace confinement and
  cleanup on failure. Verify `TestCodeStudioWrite*`, `TestCodeStudioUpload*`,
  `TestCodeStudioFailedTransfer*` and the Linux install-script tests.
- `code_studio_terminal.go` drains bounded WebSocket input in order, retains
  incomplete UTF-8/lines and collapses CRLF across frames. Simple literal `cd`
  validates an accessible physical directory inside `/workspace` before changing
  session cwd; compound shell commands do not persist cwd. Stop on cancellation
  or socket failure. Verify `TestCodeStudioTerminal*` including the WebSocket test.
- Git status uses porcelain v1 with NUL-delimited records, including the second
  rename/copy path. Preserve filename bytes through diff requests. Search uses
  an explicit grep pattern operand (`-e`) and option terminator (`--`). Verify
  `TestCodeStudioGitPorcelain*` and `TestCodeStudioSearchLeadingDash*`.

### Game Maker Voxel persistence

- `game_maker_play_state.go` owns authenticated project `/play-state` GET/PUT/DELETE
  and the trusted `/play` host. Preserve desktop read/write scopes, feature/edit/
  delete gates, no-store responses, the 4 MiB ceiling and HTTP 409 on stale CAS or
  publication bindings. The parent-only play grant is distinct from asset tokens.
- Serve the host from verified UI resources. Its iframe retains opaque-origin
  sandboxing and cannot acquire credentials. Draft/test frames receive temporary
  state only. The owning runtime/storage contracts are in
  `internal/gamemaker/AGENTS.md`; verify `TestGameMakerVoxel*`.

- `jsonError` serializes the API `error` field. Handlers pass a plain message,
  never a pre-encoded JSON object, and use existing `backend.*` translations
  for user-visible errors. Verify `TestLocalizedErrorResponsesContainMessageNotEncodedJSON`.

- `/api/llm-router/status` and `/api/llm-router/preview` require admin access.
  Preview is bounded, same-origin, read-only with respect to task/history state,
  and local by default; only an explicit `helper: true` can consume the saved
  helper quota. Never return prompts, credentials or endpoint URLs. Chat/desktop
  preparation shares the agent routing decision before image processing and
  persistence. Provider deletion guards include every saved router assignment.
  Verify `TestLLMRouter*`; runtime ownership is `internal/agent/AGENTS.md`.

### Newspaper integration
- `/api/desktop/newspaper/` is admin-scoped and same-origin for session writes. The server owns research, delivery and configured destinations; source pages cannot set recipients or invoke send tools.
- Newspaper research and capabilities share one resolver for model/guide readiness, network permission, Brave setup, DuckDuckGo, profile feeds and static page reads. DuckDuckGo has no independent toggle. Snapshot permissions and model routing; recheck restrictions before each execution without widening a run. Capabilities expose setup reasons separately from sanitized last-attempt errors; no credentials or provider bodies.
- Planning, search, extraction and editing are separate bounded stages. Keep two planning calls, 200 queued candidates, `max_searches` 32 (1–64 including retries), four reads with one per publisher domain, and two editors. Alternate topics and publishers, retain relevant exact passages and original publication dates, and allow one 24-hour-to-seven-day follow-up. All requests share existing page/time/spend caps; stop discovery at 80% of the deadline. RSS retrieval pins public IPs at redirects and bounds XML input; all search/feed/overview entries remain leads until guarded original reads. Persist source evidence before editing and counters in optional run JSON. Publish only validated immutable revisions under current write permission.
- Story editing receives the actual selected topic/interest, uses the selected provider's output/context limits and JSON mode only when supported. Keyword matches may prioritize leads but cannot reject translated originals or synonyms; the editorial guide rejects unrelated content. Keep exact source-passage validation; when no story passes, the run reason reports bounded counts by failure class without exposing source text or provider responses.
- Email uses a writable configured SMTP account or AgentMail with POST retries disabled. Telegram uses only the configured user ID. Unknown send outcomes remain uncertain in the ledger and must not be replayed automatically.
- For email confirmation, expose sanitized structured outcomes: known AgentMail rejections include the HTTP status and release only that challenge; a 403 bounce suppression has its own actionable code. Transport/server ambiguity keeps its code and cooldown. Never return provider error bodies, credentials, recipient addresses or confirmation codes.
- Verify with `go test ./internal/server -run TestNewspaper` and the Newspaper browser test. Core state contracts live in `internal/newspaper/AGENTS.md`.

### Ingress Security and Browser Lab
- `PUT /api/ui-language` requires an authenticated admin session, accepts only supported locales and saves through the config serialization lock. Login/setup language selection is a local preview until authentication. Public security status returns only setup/lockdown necessities; repeated lockdown status reads must not touch Vault or log each request.
- Normalize provider, OAuth and runtime config completely before publishing a new auth/setup configuration snapshot. A failed normalization or persistence step must leave the previous live snapshot in place.
- `patchAuthConfig` callers hold `CfgSaveMu` (setup already does; the function never locks it). It stages `config.yaml`, then writes all auth Vault keys in one `Vault.WriteSecrets` batch, loads the candidate snapshot, and on any failure restores the previous YAML bytes and Vault values without publishing. Vault fields without a Vault are an error, never silently dropped. Verify `TestPatchAuthConfig*` and `TestVaultWriteSecrets*`.
- Browser sessions retain the configured initial `auth.session_timeout_hours`.
  Same-origin POST `/api/auth/activity` requires a still-valid signed cookie and
  extends its remaining lifetime to at least ten minutes. Never shorten a longer
  session, accumulate extensions, or revive an expired/invalid cookie. Preserve
  HttpOnly, SameSite=Strict and trusted-proxy Secure handling. GET auth status is
  non-cacheable and reports remaining seconds without renewing; ordinary API
  polling and streams cannot extend sessions. Verify `TestAuthSession*`.
- Forwarded host, scheme, and client IP count only when `server.https.behind_proxy` is enabled and the immediate peer matches `server.https.trusted_proxy_cidrs`; other forwarding headers are removed before auth and URL construction.
- An auth-disabled remote listener requires `auth.allow_unauthenticated_remote` before startup, config save or setup save (`applyConfigPatch`). The check uses the effective bind host, including `AURAGO_SERVER_HOST`. This exception never opens `/speech-lab/`.
- Setup writes (`/api/setup`, `/api/setup/test`, `/api/setup/local-llm/probe`) and the first admin password during the lockdown require the one-time bootstrap token in `X-Setup-Token`, checked before the setup CSRF token is consumed. Only loopback peers of a listener without remote ingress are exempt. The token lives in memory, is logged at startup for reachable open setups, is never returned over HTTP and is cleared once an owner exists. Verify `TestSetup*Bootstrap*` and `TestAuthSetPasswordLockdown*`.
- Changing an existing password, confirming (enrolling or replacing) TOTP and disabling TOTP require a valid browser session plus a credential step-up (`verifyAdminCredentials`): `current_password` and, while TOTP is active, `current_totp_code`. Step-up failures share the `/auth/login` IP and account lockout keys (`adminLoginKeys`). The first-password bootstrap path is unchanged. Verify `TestAuthStepUp*`, `TestAuthSetPassword*` and `TestAuthTOTP*`.
- A vault that exists but cannot be decrypted keeps setup and the first-password path closed (503 `setup_vault_locked`), even for loopback peers: the lockdown hides an existing owner, not a fresh install. Verify `TestSetupStaysClosedWhenVaultCannotBeDecrypted`.
- `/webhook/` and the Telnyx path mounted at startup skip session auth because their handlers authenticate every request (webhook-bound token or HMAC, Ed25519). The Telnyx bypass follows the mounted path, never the live config. Neither opens during the password lockdown. Verify `TestAuthMiddleware*Webhook*` and `TestAuthMiddlewareBypassesOnlyTheRegisteredTelnyxWebhookPath`.
- Session-auth exemptions match exact paths or `/`-terminated subtrees only (`authBypassExactPaths`/`authBypassSubtrees`, `noPasswordExactPaths`/`noPasswordSubtrees`); never add a bare prefix. `/api/health` and `/api/ready` are public, `/api/health/discord` requires a session or an admin API token. Verify `TestAuthBypass*` and `TestAuthLockdownAllowsOnlyExactSetupDependencies`.
- The API token store fails closed. An existing `tokens.json` that cannot be read, decrypted or parsed leaves a read-only `TokenManager` (`LoadError`, `ErrTokenStoreUnavailable`) that validates nothing and never rewrites the file; a startup warning is raised. Backup import publishes a new manager only through `replaceTokenManager`, which retires the old one; the token admin routes and the webhook handler resolve the live manager per request through `currentTokenManager`. Verify `TestNewTokenManagerRefuses*`, `TestReplaceTokenManager*` and `TestHandlerResolvesTokenManagerThroughSource`. Token `LastUsedAt` is written to disk at most every 5 minutes per token, and CYD/webhook handlers record use only after their rate limiter accepted the request (`TestTouchLastUsedThrottlesPersistence`, `TestCYDRateLimitedSnapshotDoesNotTouchToken`, `TestHandlerRateLimitedRequestDoesNotTouchToken`).
- Parse `Authorization: Bearer` only through `bearerCredential`/`bearerScheme` (case-insensitive scheme, space or tab separator); never `strings.HasPrefix(…, "Bearer ")`. Daemon APIs accept admin-scope tokens exactly like `requireAdmin`, with no cookie fallback for a presented token. Verify `TestBearerScheme*` and `TestDaemonAuthRequiresAdminScopeForBearer`.
- SSE JSON events are redacted per decoded string/number value (`scrubSSEJSON`) before delivery; never run `security.Scrub` over serialized JSON, because JSON escapes (`\u0026`, `\u003c`) hide registered secrets and text replacement can corrupt the event. Messages without a match keep their exact bytes. Verify `TestSSEScrub*`.
- The access log records every mutating request and every response >= 400, including on `/api/dashboard/*`. Only successful GET/HEAD/OPTIONS dashboard/status polls are logged at Debug, and `/events` is never wrapped or logged. Verify `TestAccessLog*`.
- `/speech-lab/` requires an AuraGo session, same-origin writes and WebSocket Origin, and a configuration-owned private backend. Strip AuraGo credentials before forwarding. No separate Tailscale port 8766 listener.
- Desktop embed credentials travel only in `/desktop-ticket/<ticket>/...` paths, are stripped before access logging, and remain scoped to their exact desktop path or camera resource. Query tokens fail.
- `/api/desktop/tresor` accepts only an enabled, valid browser admin session. Reject any Authorization header and insecure remote HTTP; direct localhost is allowed. Require Origin on mutations, record/header revisions for changes, bounded ciphertext and `Cache-Control: no-store`. The server stores only opaque ciphertext and key envelopes in `data/tresor.db`; browser crypto and threat limits are in `documentation/tresor.md`. Verify `TestTresorBrowserBoundaryAndRevisions` and the backup inclusion test.
- The main UI at `/` carries a report-only CSP without `unsafe-inline` while legacy inline handlers/styles are migrated; desktop app CSPs keep their separate contracts.
- POST `/api/realtime-speech/progress-audio` accepts only an active browser voice
  session, its current action ID and a server-owned acknowledgement/wait kind. Use the active Speech Lab
  TTS and voice snapshot for Speech Lab sessions, or effective chat TTS for other
  profiles. Synthesize in memory with bounded concurrency/time/bytes; reject
  foreign origins and sessions and never accept arbitrary client speech text.

### System World Tower Voice
- POST `/api/desktop/system-world/voice` returns one short transient audio clip.
  Reuse the effective chat TTS configuration and in-memory synthesis, including
  the active Speech Lab backend/voice snapshot. Require the desktop admin scope
  for bearer clients; desktop readers must not gain global chat/memory access.
- Randomly sample existing core/long-term memories, active notes and visible
  user/assistant chat. Never expose tool/internal turns, file-index collections,
  archived notes/memories, thinking blocks or registered secrets. Sampling must
  not change source content or access metadata. No LLM, SSE publication, media
  cache or text logging; one synthesis at a time with bounded request cadence.
- GET `/api/desktop/system-world/memory-artifacts` feeds the memory-archive
  hologram with at most eight excerpts of at most 96 runes from the same
  sampler and scrubbing. Same admin desktop scope, Virtual Desktop must be
  enabled, one request at a time with a 4 s cooldown (429 otherwise), `no-store`,
  no LLM, SSE, caching or text logging. The client renders excerpts as canvas
  text only and never exposes them in diagnostics.
- Audio mixing, spatial attenuation, hologram, atmosphere and lifecycle
  contracts live in `ui/js/desktop/apps/AGENTS.md`. Verify with
  `TestSystemWorldVoice*` and `TestSystemWorldMemoryArtifacts*`.

### 3D Printer Integration Contract
- The opt-in `builtin-printer` Desktop widget uses authenticated GET `/api/3d-printers/status`: without `printer_id` it lists only configured IDs/names and the default, with an explicit ID it executes only `status`. Disabled integration blocks reads. Camera expansion reuses the existing same-origin camera stream; widget polling never stores camera snapshots or invokes an LLM.
- Elegoo SDCP status/attributes reads wait for a nonempty matching `Status`/`Attributes` snapshot (top-level or under `Data`), not a command ACK or unrelated push. Negative ACKs fail; the whole command shares one deadline and honors cancellation. Verify with `go test ./internal/tools -run 'Elegoo|ThreeDPrinter'`.
- Klipper/Moonraker API keys are vault-only. Store them under per-printer keys derived from the printer ID (`three_d_printer_klipper_<sanitized-id>_api_key`); never serialize them into `config.yaml`, API config responses, or tool output.
- Normal 3D-printer operations require an explicit `printer_id` unless `three_d_printers.default_printer` is configured. `list_printers` and ad-hoc `/api/3d-printers/test` are the setup exceptions.
- Camera snapshot and stream APIs must enforce `three_d_printers.enabled`. Klipper snapshots prefer Moonraker `snapshot_url`; live streams require a valid HTTP(S) `stream_url` on the configured printer host.

### go2rtc Integration Contract
- AuraGo manages only its own pinned go2rtc Docker sidecar. API/UI port 1984 is loopback-only for native AuraGo and Docker-internal for containerized AuraGo; RTSP port 8554 is never host-published.
- Stream source URLs and the internal API password are Vault-only. Generated go2rtc configuration must contain neither sources nor plaintext credentials; inject enabled sources through the authenticated runtime stream API after startup.
- Keep upstream go2rtc logging disabled because producer warnings can contain runtime source URLs. Bound snapshot memory and stored-media retention so viewer access cannot exhaust AuraGo memory or disk.
- The managed container receives the internal password only for go2rtc startup. Docker inspect output must redact it; direct agent lifecycle, log, exec, copy, and process-list access to the go2rtc container is blocked; and `go2rtc_` Vault keys are forbidden for Python/skill export.
- Accept one network source per stable stream ID and only `rtsp`, `rtsps`, `rtspx`, `http`, `https`, or `onvif`. Reject files, devices, exec, custom ffmpeg, arbitrary URLs, and agent-side stream mutation.
- Keep viewer/proxy access scoped to configured enabled stream IDs. Strip caller authentication and cookies, block raw config/log/process/publishing/mutation routes, and use AuraGo's internal Basic authentication upstream.
- Direct LAN WebRTC is opt-in and requires a concrete private bind/candidate IP before publishing 8555/TCP and 8555/UDP. The `network-cameras` desktop app must use `/api/go2rtc/viewer/{stream_id}`, `/api/go2rtc/thumbnail/{stream_id}.jpg`, and sanitized AuraGo APIs only.
- ONVIF discovery is admin-only and available only with `Runtime.BroadcastOK`. Keep candidates and credentials memory-only, private-network scoped, bounded, short-lived, and single-use; never proxy go2rtc's raw `api/onvif` route.
- Camera tiles must use the non-persistent snapshot-bytes path so `store_media` never registers periodic thumbnails. Viewer access remains `go2rtc.view`; setup and stream mutations remain administrator-only and same-origin.
- Managed camera mutations publish a fully loaded and validated YAML/Vault desired state before runtime reconciliation. Pre-publication failures roll back Vault changes; post-publication reconciliation failures retain the desired state and return HTTP 202 for background retry. Reserve ONVIF setup tokens until publication and keep private ONVIF SOAP traffic proxy-free.
- Enabling go2rtc must actively verify readable Docker container/image endpoints, the network endpoint when AuraGo runs in Docker, and mutation permission through a random nonexistent-container start probe that cannot create a resource.

### AI Gateway Contract
- Cloudflare AI Gateway routing must use provider-native segments where supported. In `auto` mode, unsupported providers must skip gateway routing and report a warning instead of silently falling back to `/openai`.
- Workers AI uses the Cloudflare REST base (`https://api.cloudflare.com/client/v4/accounts/{account}/ai/v1`) with `cf-aig-gateway-id`; provider-native routes use `cf-aig-authorization` for the optional authenticated-gateway token.
- AI Gateway status checks must stay local and non-token-consuming; live Workers AI connection tests validate the Cloudflare API token/account via `/ai/models/search` and include `cf-aig-gateway-id`.
- The privacy-safe default is `log_mode: metadata_only`; metadata headers must never contain secrets.

### here.now Integration Contract
- here.now uses only the fixed `https://here.now` API origin, Vault key `here_now_api_key`, authenticated permanent Sites, and explicit personal or workspace account resolution. Never fall back to anonymous publishing or claim flows.
- Homepage publishing snapshots a relative workspace directory into a private temporary tree before the first provider request. Reject traversal, symlinks/reparse points, special files, credential material, and more than 1,000 publishable files; upload only the snapshot and always remove it afterward.
- Retry reads, presigned upload PUTs, and idempotent finalize calls only. Never automatically replay create, update, duplicate, restore, metadata, access, refresh, or deletion mutations; ambiguous outcomes must be structured as `here_now_outcome_unknown` with `retry_safe`.
- API, upload, and Site verification traffic uses strict public DNS-pinned transports that ignore the loopback SSRF escape hatch. Upload redirects are blocked; Site verification validates and repins every bounded redirect hop before accepting a final `2xx`, `401`, or `403`.
- Access mutations require a complete current provider policy and preserve omitted allowlist fields. Site-password Vault keys bind canonical account ID plus slug, never use slug-only fallback, and are deleted only after verified password removal or successful Site deletion. The generic Homepage ledger records here.now only after finalize and URL verification.

### GitHub Integration Contract
- `github.allowed_repos` is a strict allowlist; prefer `owner/repo` entries. Legacy bare repo names only match the configured `github.owner`.
- An empty `github.allowed_repos` list permits only repositories AuraGo created through the GitHub tool and tracks with `agent_created=true`.
- Manual `track_project` entries are local inventory only and must never grant remote repository access.

### Homepage Managed Website Ledger
- Managed homepage/web projects use `data/homepage_registry.db` as the system of record for project identity, local file state, structured events, revision links, deployment targets, deployment history, remote observations, and drift status.
- Homepage project identity is the `project_dir` relative to `homepage.workspace_path`; avoid storing absolute workspace paths as the canonical project key.
- Mutating homepage operations must keep the ledger current by recording structured events and, when files change, revisions plus file-state snapshots. Remote deploys must be linked to provider IDs/URLs and build artifact hashes when available.
- Server APIs under `/api/homepage/sites` expose the managed-site read model, including detail `deploy_targets` and `remote_observations`, plus the reconciliation path. Keep these APIs additive and compatible with existing `/api/homepage/history`.

### Configuration UI Integration Test Contract
- `/api/treg/` is admin-only. Connection tests read the saved Vault-backed organization and balance; they never execute a catalog endpoint or accept a token in the request body. Catalog, balance and local status routes share the client policy boundary.
- Schema-rendered Telegram, Discord, Rocket.Chat, Home Assistant, Proxmox, S3, Frigate, and Ansible sections expose read-only connection tests through the shared registry in `ui/js/config/`.
- Test actions are enabled only for saved configuration and available Vault-backed credentials. Their backend routes are POST-only and admin-protected; probes must not send messages, execute playbooks, mutate storage, or change remote state.
- The probes use the integrations' documented read-only authentication/status requests. Any new production HTTP client must be classified in `internal/audit.NetworkClientInventory`, and action text must be present in every `ui/lang/config/common/` locale.
- `/api/models/catalog` uses the bundled provider/model catalog for its list. For exact provider/model matches, its structured-output flag follows the `models.dev` registry instead of the catalog's API-family inference; unmatched models keep their catalog flag. Verify with `TestHandleModelCatalogStructuredOutputMatchesModelsDev`.

### EasyDrag flows
- Wiring (`flows_service.go`): `initFlows` runs before `MissionManagerV2.Start` (it installs the `FlowHooks`, so flow triggers and the startup trigger find them) and `startFlows` after it (old runs marked interrupted, Date/Time timers armed, then one `Service.ReconcileMissions` on a goroutine). `shutdownFlows` runs once the HTTP API is drained and before MQTT, mail, MCP, the sandbox, Looper and the databases stop: `Service.Shutdown` within what is left of the shutdown context (5–15 s), then the store. The desktop capability `flows` and the API gate are `flowsAvailable`: the service exists and `flows.enabled` and `tools.missions.enabled` are on.
- Configuration: `initFlows` reads `flows.enabled` and the four limits at boot only, so a config save that changes them reports the restart reason "EasyDrag flows" (`flowsRuntimeConfigChanged`, effective values); `ai_provider` and `agent.*` are read live, and `flows.ai_provider` is a provider reference (`provider_references.go`). `flows.db` lives next to the Game Maker database (`config.FlowsDBPath`) and is listed in `config.SQLiteDatabasePaths` (backups, the agent's protected files).
- Bridge (`flows_bridge*.go`, `flowMissionBridge`): it never calls a Service method that takes a flow lock; `broadcastMissionState` reaches the Service only through the lock-free `NextTimer`. Only a started run (`RunFinishedInfo.Started`) reaches `MissionManagerV2.FlowRunFinishedAtDepth`, with the chain depth read from the run record (`tools.CompletionChainDepth`); a run that never started only broadcasts `flows_changed`, because that path runs inside `DeleteFlow`, `CancelMissionRuns` and `Shutdown` and must stay free of database, file and network work. Outputs and trigger data are scrubbed value by value and bounded before Mission Control sees them (the output text 16 KiB, the dependents' `outputs` 64 KiB, history trigger data 16 KiB).
- Failures (`flows_bridge_notify.go`): every finished run that was not cancelled records or resolves the planner issue `flow|<flow id>`, and every recorded failure fires `planner_operational_issue` triggers, as agent missions do. A failure notifies per the flow's `notify_on_error` (`desktop`, the default, `push`, `telegram` or `off`) under the flood rule of `flowFailureNotifier`: once when the flow starts failing, then at most once an hour (`flowFailureNotifyInterval`) while it keeps failing; a success resets it, a cancelled run neither notifies nor resets, and the state is in memory only. Push and Telegram sends run on goroutines with at most 4 in flight per channel (`flowNotifyMaxInFlight`); beyond that a notification is dropped with a Warn and leaves the failing state unchanged. The message carries the flow name (80 runes) and the error (300 runes).
- Hooks (`flowMissionHooks`): `StartFlowRun` calls the lock-free `Service.TriggerFromMission`. `FlowMissionDeleted` and `FlowEnabledChanged` take the flow's lock through `DeleteFlowForMission`/`MissionEnabledChanged`, bounded by `flowHookTimeout` (2 min), and the manager calls them only on goroutines of their own; a gone flow is logged at Debug, anything else at Warn. After `shutdownFlows` the hooks only log (`TestC16HooksAfterShutdownOnlyLog`).
- The dashboard's cron handlers (`dashboard_handlers_cronjobs.go`, `dashboard_handlers_overview.go`) list a flow's schedule jobs with `managed_by: easydrag` and answer edits, toggles and deletes of them, and adds under an id a flow mission claims (`tools.FlowOwnsCronJob`), with 409 "This schedule is managed by EasyDrag; edit the flow instead."
- `/api/desktop/flows/…` (`flows_handlers*.go`; this section describes the API): bearer tokens need `desktop:read` for GET and HEAD, `desktop:write` for `POST validate` and `desktop:admin` for every other request (`flowsRequiredScope`, applied by `validRouteBearer` and `handleFlows`), because test and live runs execute host tools and publish, enable and secrets write missions, cron jobs, webhooks and the vault (`TestFF1FlowsWritesNeedTheAdminScope`); session users are not affected. Same-origin session writes, `FLOWS_DISABLED` (503) and `FLOW_PERMISSION_DENIED` (403) gates. Errors are `{"error", "code"}` with stable codes and go through `s.flowsErrorFrom(w, r, err)`, which matches with `errors.Is`/`errors.As`, never by text: `FLOW_INVALID` (422, with `issues`), `FLOW_NOT_FOUND` and `FLOW_RUN_NOT_FOUND` (404), `FLOW_REVISION_CONFLICT`, `FLOW_NOT_PUBLISHED`, `FLOW_NO_TRIGGER`, `FLOW_EXISTS` and `FLOW_MISSION_AMBIGUOUS` (409), `FLOW_LOCKED` (409) for `tools.ErrMissionLocked`, `FLOW_MISSION_MISSING` (409) for `tools.ErrFlowMissionNotFound`, `FLOW_RUN_LIMIT` (429) for a full queue, `FLOWS_DISABLED` (503) for a closed runner, `FLOW_TOO_LARGE` (413: a body over its limit, `flows.ErrDocumentTooLarge`, `flows.ErrTestDataTooLarge`), `FLOW_BAD_REQUEST` (400) for an unsupported schema or an unknown template, and a `NodeError`'s own code (400, `FLOW_NODE_FAILED` when empty). `FLOW_INTERNAL` (500) carries only a generic message; the cause (paths, SQL, vault text) is logged at Warn, scrubbed and cut to 300 runes, with the route and flow id. A request whose context was cancelled (the client went away, or the server drain cancelled it) gets 503 `FLOWS_DISABLED` "the request was cancelled" (Debug log), never an implicit 200. Typed-nil errors land in `FLOW_INTERNAL` without a panic.
- Partial publish: when `Service.Publish` returns a record and an error, the revision is live but Mission Control or the timers were not updated. The answer is 200 `{"flow", "issues", "partial": true, "code": "FLOW_PUBLISH_INCOMPLETE", "error": "Published, but not every part could be updated (Mission Control or timers). Publish again to finish."}`; the flow is broadcast as `published` and audited with status warning. `HasUnpublishedChanges` is false then, and publishing the same draft revision again finishes the update, so the editor keeps its Publish button on `partial`. Exception: when the flow's mission is gone from Mission Control (`tools.ErrFlowMissionNotFound`; a publish never recreates it) the code is `FLOW_MISSION_MISSING` with "Published, but the flow's Mission Control entry is missing. Export the flow, delete it and import it again."; other routes answer that sentinel with 409 `FLOW_MISSION_MISSING`.
- `flowsJSON` encodes before it writes the status (an encode failure is 500 `FLOW_INTERNAL`); `flowsCollectionRoutes` lists only the collection routes `handleFlows` dispatches and must follow it.
- Flow secrets are vault entries `easydrag_<name>` (name `[a-z0-9_]{1,40}`, value at most 4 KiB, else `FLOW_TOO_LARGE`; writes and deletes limited to 30 per minute per client IP, else 429 `FLOW_RATE_LIMITED`). The API lists names and never returns values; a write goes through `WriteUserSecretContext` (not agent-readable, bounded by the request). The value is not registered with the global output scrubber on write: `flowSecrets.ReadSecret` registers it, raw and trimmed, when a run reads it, before any node can output it, so writes do not grow the process-wide scrubber. `tools.IsPythonAccessibleSecret` blocks the `easydrag_` prefix, so the agent's vault tool, Python and skills can neither list, read, create nor delete them (`TestC17FlowSecretsStayOutOfTheAgent`). A delete answers `used_by` with the published flows whose live revision names the secret in a `secret_ref` parameter of an enabled node (literal names only); EasyDrag's secret field deletes through this route and shows `used_by` in a warning. Audit entries carry the name, never the value.
- Run data answers go through `flowsJSONScrubbed` and SSE events through `writeFlowSSE`; both scrub the decoded values (`flowScrubbedJSON`, `scrubFlowValue`), not the JSON text. Flow ids outside `[A-Za-z0-9_-]{1,64}` and unknown sub-paths are `FLOW_NOT_FOUND`; a new `{id}/…` route adds its segment count to `flowRouteSegments`.
- Runs (`flows_handlers_runs.go`): `GET runs/{run}/events` sends `event: snapshot` (the stored `RunDetail`, read after subscribing), then each bus event after the start point as `id: <seq>` + `event: event`, then `event: end`; `: heartbeat` comments every 15 s. The start point is the larger of `?after=` and `Last-Event-ID` (EventSource reconnects with its original URL); values outside 0..2^20 are ignored. `end` means the run is over (run_finished delivered, a final snapshot, a run the bus does not know, or a channel closed before any event); `event: resync` `{"after": <seq>}` means the bus dropped a subscriber that fell 256 events behind while the run goes on, so the client reconnects with `?after=`. At most 16 streams per run and 64 in total (429 `FLOW_RUN_LIMIT`). Streams end on the shutdown drain (`trackHTTP` cancels the request) and keep `httpstream`'s renewed write deadline (never cleared). `POST runs/{run}/cancel` answers 202 (also again while the run winds down), 404 `FLOW_RUN_NOT_FOUND` for unknown runs and 409 `FLOW_RUN_FINISHED` for ended ones; only the first cancel of a live run is audited. Run ids outside `[A-Za-z0-9_-]{1,64}` are 404. EasyDrag's test dialog reads `GET {id}/publish-preview` before every test for the effects of the real params (`CollectEffects`), so that route stays free of side effects. Test data exists only for enabled triggers of the draft (400 for an id that is no node id, 409 `FLOW_NO_TRIGGER` otherwise). Verify with `go test ./internal/server -run 'TestFlowsAPI|TestC18'`.
- Catalog (`flows_handlers_catalog.go`): `GET node-types` is cached per registry generation, configuration snapshot and language (`flowNodeTypesCache`, at most one answer per language) and answered with a weak `ETag` and `Cache-Control: private, no-cache`; a matching `If-None-Match` gets 304, and `HEAD` gets the headers of `GET` without the body. The gzip middleware compresses it on the main HTTP/HTTPS listeners (the loopback and tsnet listeners have no gzip). An in-place write to the live configuration (no snapshot swap) does not refresh it, as for `flowCatalogEnv`. The palette's `effects` and `risky` describe a node with default parameters (a generic node: its worst case). They are hints, not a safety guarantee: the publish preview (`CollectEffects` over the real parameters) is authoritative, and at run time every tool call passes the tool's own gates.
- Options (`node-types/{type}/options/{param}`, `flowOptionList`): every label and hint is cut to 120 runes, and each source offers only values its node can use. Webhooks: all of them, like Mission Control's picker, with a disabled one marked in its hint (`easydrag.option.webhook_disabled`). Notification channels: only those `tools.SendNotification` sends to (Discord only when enabled, not read-only and with a default channel). AI models: only providers that can answer an `ai.step` (`flowChatProvider`, the static checks of `flowProviderEntry`); the default names the `flows.ai_provider` model, else the main model. Missions: the hint is the execution type, `agent` for older agent missions without one. Home Assistant entities are sorted by id, cut at 2000 with `"truncated": true`, and reused per configuration snapshot (`flowHACache`: a list for 30 s, an error for 5 s); concurrent misses share one Home Assistant request (singleflight, detached from the first caller, 10 s timeout). `FLOW_OPTIONS_UNAVAILABLE` (502) carries the cause scrubbed and cut to 200 runes. `POST validate` changes nothing, so it also works while missions or the desktop are read-only (`flowsReadOnlySafe`; desktop permission and origin checks still apply); `mode` is `draft` (also when empty) or `publish`, anything else is 400. `validate` with `publish` cannot check rules that need the flow id (the self-trigger of a `trigger.mission_completed` on the flow's own mission); publish and publish-preview do, so "valid" is not "publishable". `templates/…` and `validate/…` are 404 `FLOW_NOT_FOUND`. Verify with `go test ./internal/server -run 'TestFlowsAPICatalog|TestFlowOptions|TestC19'`.
- Mission Control cancel on a flow mission (`handleMissionCancelV2`) cancels the flow's live runs through `Service.CancelMissionRuns`, called after `MissionManagerV2.Get` (a copy; the manager's lock is not held, because unstarted runs are reported to `FlowRunFinished` on this goroutine) with a detached 10 s context: 202 when a run was cancelled (queued and waiting live runs go with it). Its flow answers carry a code through `flowsError` (picked in the handler with `errors.Is`; not `flowsErrorFrom`): `FLOW_NO_ACTIVE_RUN` (409: the mission is not running, `tools.ErrMissionNotRunning` from `CancelCheck`, or no live run was left to cancel; Mission Control shows its EasyDrag hint for this code only), `FLOW_MISSION_AMBIGUOUS` (409), `FLOW_NOT_FOUND` (404), `FLOW_INTERNAL` (500) and `FLOWS_DISABLED` (503, no flow service). Agent missions keep the code-less `jsonError` answers. `CancelCheck` still requires a running mission, so live runs that are all still queued can only be cancelled through `POST /api/desktop/flows/runs/{run}/cancel`; EasyDrag's runs drawer and run view offer Stop on every run that has not ended (queued, waiting, running) and post that route.
- Audit: user actions go to the audit timeline (source `mission`) as `flow_create`, `flow_import`, `flow_publish`, `flow_enable`, `flow_disable`, `flow_delete`, `flow_secret_set`, `flow_secret_delete` and `flow_run_cancel`. A new type needs an option in the dashboard's audit type filter (`ui/dashboard.html`) and a `dashboard.audit_type_<type>` label in all 16 `ui/lang/dashboard` files (`TestC17AuditTypesAreRegisteredInTheDashboard`).
- Verify with `go test ./internal/server -run 'TestFlowsAPI|TestC17'`.

## Verification

- Run `go test ./internal/server` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
