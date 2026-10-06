# Server integrations

## Purpose

Server-owned HTTP and cross-component integration contracts.

## Ownership

`internal/server` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

- MCP connection tests require enabled MCP gates, test only the selected server and bind Vault aliases to its saved launch configuration. The automatic Dograh client grants private access only to the managed service's exact origin. Local/Docker stdio observes shell, sandbox, unsafe-host and Docker mutation grants at launch.
- The incoming MCP endpoint checks Host against configured names/local addresses independently of Origin. Configure server.host or server.https.domain for an external name. Never resolve a request-supplied hostname as authority. MCP sessions are random, signed, expire after 24 hours and bind to the authenticated credential; stateless requests get independent sessions. The MCP allowlist is a hard scope for direct, wrapped and ask_aurago calls; enabling the IDE preset must not widen an explicitly selected list.

- Integration connection tests bind stored credentials to saved targets. A Dograh target override requires an explicit credential. YepAPI tests accept POST only, use the saved base URL and read the free model catalog; they never create a paid search or claim that a public catalog proves key validity.

- Telegram, Discord and Rocket.Chat receive the server-owned budget tracker and the shared isolated context recap formatter. Telegram worker admission, transcription, typing and agent execution inherit the polling owner context; cancellation stops queued work and ongoing model work.

### HTTP trust and shutdown boundaries

- Desktop agent chat, its stream and log APIs require administrative Desktop access. Log tail/search/stream/download scrub registered secrets and credential fields. Passive media proxies reject active HTML/SVG/XML; every inline Knowledge document uses sandbox CSP. Garage HTTP and WebSocket proxies remove local cookies and authorization headers before forwarding.
- Logout is same-origin POST and persistently revokes the presented session until expiry. Nonce-bearing sessions remain independent; revocation persistence failure does not report success. Login combines bounded per-IP state with account verification concurrency/backoff rather than letting one attacker globally lock the owner out.
- Read TokenManager through one synchronized snapshot per operation. Stop HTTP acceptance, cancel active handlers, and close tracked hijacked WebSocket connections before waiting for handlers to drain; request cancellation alone does not unblock WebSocket reads. Close dependencies only after the drain. Re-panic `http.ErrAbortHandler`; never write a replacement JSON response over a partial proxy response. Verify `TestHTTPDrainClosesHijackedWebSocketBeforeWaiting`.
- Cast LAN listeners require a file-specific ticket, reject listing and keep bounded header/idle/stream-write budgets. Store preview calls a configured loopback origin with no redirects; request Host cannot choose it.
- Verify codequality boundary/lifecycle tests, Remote handshake tests and the existing Desktop token/proxy matrix. A local fixture does not establish external delivery or device acceptance.

### Administrator container API

- `/api/containers` and `/api/containers/` stay in `validRouteBearer`'s admin catch-all: browser sessions or Bearer tokens with the `admin` scope; every other scope gets 403 `invalid_bearer_scope`. Do not add the paths to desktop, go2rtc, bypass or lockdown lists, and do not wrap them in `requireAdmin` (it would refuse non-admin tokens on auth-disabled installs, which reach the handler today). System World calls `handleContainerAction` directly for start/stop/restart under its own `desktop:admin` gate. Verify `TestContainerRoutesKeepAdminScopeThroughAuthMiddleware`, `TestContainerRoutesStayInTheAdminBearerCatchAll`, `TestContainerRoutesStayOpenWhenAuthIsDisabled` and `TestContainerRoutesAuthDisabledIgnoreBearerScope`.
- Terminal, update and remove classify the target first (`containers_protection.go`): the agent docker tool's owner list (also for a 404 on a reserved name), the container AuraGo runs in and the container serving `docker.host` (one of its network addresses equals an address the endpoint host resolves to). Self needs the Docker runtime plus either a default hostname that prefixes the container ID or AuraGo's own ID from the `/etc/hostname`/`/etc/hosts`/`/etc/resolv.conf` bind mounts in `/proc/self/mountinfo` (mount root `…/<id>/<file>`, so a dedicated `containers` mount matches too), falling back to cgroup v1 `/proc/self/cgroup`; both signals are read once per request. Docker gives every container that joins another's network namespace (`network_mode: container:`/`service:`, e.g. a Tailscale or Gluetun sidecar) the provider's hostname and `/etc` files, so the signals cannot tell AuraGo from its provider: when any listed container's `HostConfig.NetworkMode` references the named container (full ID, an ID prefix of at least 12 hex characters, or name; `TestContainerNetworkModeJoinsMatchesFullIDLongPrefixOrName`), neither it nor the joining containers count as self; they are `shared-network` (confirmation). The action path makes that one list request only when the signals name the target or the target joins another namespace; a failed list, a failed endpoint lookup or a failed inspect make the target `unverified` (confirmation), never silently allowed. The list marks no endpoint container when the lookup fails but keeps every other flag. Podman is not covered (no `/.dockerenv`, files under `overlay-containers/<id>/userdata/`): self is never proven there, and the app container falls back to the name/label confirmation. Protected or unverifiable targets need `confirm=protected` (409 `container_protected_confirmation_required`, one message per label); update on proven self or the Docker endpoint always answers 409 `container_self_update_unsupported`. The terminal checks the WebSocket origin and then the WebSocket upgrade before classifying, so a cross-origin handshake or a plain GET causes no Docker request and no DNS lookup. Start, stop, restart, logs, inspect and stats never consult protection. `GET /api/containers` adds `protected_owner`, `self`, `docker_endpoint` and `shared_network`; SSE `container_update` stays unannotated and the page merges the last list's flags. Verify `TestContainerProtected*`, `TestContainerUpdateRefuses*`, `TestContainerConfirmationMessagesNameTheReason`, `TestContainerTerminalChecksOriginBeforeProtection`, `TestContainerLifecycleActionsNeverConsultProtection`, `TestClassifyContainerForAction*`, `TestContainerEndpointLookupFailureNeedsConfirmation`, `TestContainerSelfSignalsIgnoreASharedNetworkNamespace`, `TestContainerSelfNeedsTheListToRuleOutASharedNamespace`, `TestContainerIsSelfProvenByMountinfoWithCustomHostname`, `TestOwnContainerID*`, `TestAdminContainerList*` and `TestContainersScriptConfirmsProtectedContainersBeforeSendingTheFlag`.
- Container and Store terminals reject a request that is not a WebSocket upgrade (400) before any Docker work and before any exec exists; the container terminal does so before the protection lookup too (`TestContainerTerminalRejectsPlainGETBeforeProtectionLookup`). Closing a terminal does not signal the exec: no EOF and no Ctrl-C, so a tmux or screen session, a running job and (as before) an orphaned shell all survive the modal closing. Docker cannot end an exec, and TTY execs outlive their attach stream, but an EOF would end a multiplexer session at an empty prompt, so do not add one. Verify `TestContainerTerminalRejectsPlainGET*`, `TestDesktopStoreTerminalRejectsPlainGET*`, `TestContainerTerminalClosesWithoutEOF` and `TestDesktopStoreTerminalClosesWithoutEOF`.
- Tool failures answer 502 with the unchanged `{"status":"error",…}` body; 503 stays "Docker disabled", 403 "read-only", 409 the protection codes. Never 401: the shared fetch wrapper redirects to login. Verify `TestContainerToolErrorsUseBadGatewayAndKeepTheBody` and `TestContainersListFailureShowsDockerMessageNotDisabledState`.
- `Start` calls `bindDockerSelfIdentity` (`docker_self_identity.go`): it binds `tools.DockerSelfIdentityFor` to the same self signals (`readContainerSelfSignals`, unchanged) plus one inspect of that container and one container list for `containerSelfInList`, cached for the process (a failed inspect or list is retried after 30 s), which yields the `com.docker.compose.project` label and the volumes and binds mounted at or below the data directory (`Proven`). When another container joins the named container's network namespace (Gluetun, Tailscale), the signals name the provider: the identity is not proven, carries no mounts, and keeps the project only when the provider and every joiner carry the same project label (`TestDockerSelfIdentityDoesNotTakeASharedNetworkProviderForAuraGo`). Native runtimes get the zero identity and make no Docker request. The agent Compose policy uses it to refuse commands on AuraGo's own project. Verify `TestDockerSelfIdentity*`.

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
- External chat ingress, SMS and SIP block raw input at local `ThreatHigh`; authenticated webchat and browser voice log findings without blocking. Voice always uses canonical escaped external-data isolation, even when supplied text already contains boundary tags. Keep current authorization snapshots and tool scopes.
- Quarantine replaces an otherwise agent-directed delivery with a fixed safe notice through the existing route. Apply the original eligibility and forwarding rules, including active matching missions; UI/log-only events with no agent destination stay local. Do not execute original payload callbacks or copy subjects, bodies or scanner prose. A failed notice is not acknowledged; accepted notices cannot replay because a label update failed.
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
- Elegoo SDCP 386 is a write: only enable_camera/disable_camera may send it, behind read-only gates. Camera URL/snapshot/stream reads use a bounded in-memory URL cache populated by successful explicit activation, bound to printer ID, URL and board. Restart requires a new explicit activation.
- `/api/3d-printers/test` is admin-only, accepts only `test_connection`, and executes with read-only runtime rights. Ad-hoc setup probes do not enable the saved integration. Printer snapshot/stream redirects remain at their initial HTTP origin.
- The opt-in `builtin-printer` Desktop widget uses authenticated GET `/api/3d-printers/status`: without `printer_id` it lists only configured IDs/names and the default, with an explicit ID it executes only `status`. Disabled integration blocks reads. Camera expansion reuses the existing same-origin camera stream; widget polling never stores camera snapshots or invokes an LLM.
- Elegoo SDCP status/attributes reads wait for a nonempty matching `Status`/`Attributes` snapshot (top-level or under `Data`), not a command ACK or unrelated push. Negative ACKs fail; the whole command shares one deadline and honors cancellation. Verify with `go test ./internal/tools -run 'Elegoo|ThreeDPrinter'`.
- Klipper/Moonraker API keys are vault-only. Store them under per-printer keys derived from the printer ID (`three_d_printer_klipper_<sanitized-id>_api_key`); never serialize them into `config.yaml`, API config responses, or tool output.
- Normal 3D-printer operations require an explicit `printer_id` unless `three_d_printers.default_printer` is configured. `list_printers` and ad-hoc `/api/3d-printers/test` are the setup exceptions.
- Camera snapshot and stream APIs must enforce `three_d_printers.enabled`. Klipper snapshots prefer Moonraker `snapshot_url`; live streams require a valid HTTP(S) `stream_url` on the configured printer host.

### go2rtc Integration Contract
- A go2rtc status probe publishes and returns its current failure state; never reuse an earlier successful API status after a failed probe.
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
- Gateway probes and runtime preserve explicit custom endpoints; only canonical provider HTTPS endpoints may be rewritten. Report custom_endpoint locally without making a probe request.
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
- An empty `github.allowed_repos` list permits only repositories recorded in the protected `directories.data_dir/github_trust.json` ledger after successful creation. Grants bind API base and canonical repository identity. Workspace `agent_created` markers never grant access; existing markers expose `trust_migration_required` until an administrator explicitly selects the repository in the allowlist.
- `github.allow_delete` defaults off and is required in addition to repository access and disabled read-only mode. Revoke the protected grant before deletion. Preserve the last valid ledger with a synced atomic replacement and private backup; do not automatically import legacy markers.
- Manual `track_project` entries are local inventory only and must never grant remote repository access.

### Homepage Managed Website Ledger
- Quick tunnels use active registry identities and a private static snapshot through the shared Cloudflare publication boundary. The config UI selects `cloudflare_tunnel.quick_project_dir` from registered projects. Explicit tool/API ports and the former in-container arbitrary-port entry point fail closed. Disable/read-only, workspace/registry changes and project selection changes revoke active quick publication; shutdown cannot be blocked by agent read-only grants. Named/token tunnels remain separate.
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

### Rocket.Chat Runtime Contract
- One server-owned consumer processes history chronologically with bounded pagination, including messages sharing a timestamp. Accept ISO timestamps and the legacy date object.
- Every turn uses a current immutable config/client snapshot. Publishing changed Rocket.Chat credentials, channel, allowlist or enablement cancels the old generation immediately; drain outside config locks before starting its replacement. Egg mode and shutdown forbid restart.
- HTTP requests and agent turns inherit the runtime context. Shutdown waits for message processing before closing databases. Integration credentials never follow a foreign redirect.
- Verify `TestRocketChat` in `internal/rocketchat` and `internal/server`.

### Mission and Home Assistant Lifetimes
- Mission manager StartContext inherits the server lifetime; repeated Start is idempotent. Stop is terminal, refuses further work, and waits for the queue, invocation callbacks, timeout guards and completion work. Invocation callbacks must use the manager Context and must not detach their work.
- Mission HTTP handlers remain independent of a caller disconnect, but inherit server shutdown. Their registry cancels and drains every generation before database closure, including replaced runs. History completion finishes within the tracked invocation.
- Home Assistant polling owns one cancellable generation. Publish changed credentials/URL/enablement before draining outside config locks and starting a replacement. Shutdown drains polling before closing mission databases. Entity IDs use the shared path encoder and credentials cannot follow a foreign-origin redirect.
- Verify mission lifecycle and Home Assistant regression tests in tools and server; run Linux CGO race tests for concurrent cancellation.

### Uptime Kuma and EvoMap
- Uptime Kuma uses one server-owned cancellable generation, one notification consumer, at most sixteen pending prompts and a five-minute per-turn deadline. Overflow is logged. Monitor fields are bounded external data; use a dedicated session. Config publication cancels stale generations, and shutdown drains them before database closure.
- EvoMap registration is writable-only in the agent and API. The server callback owns network registration and serialized config/Vault publication under CfgSaveMu; never mutate captured config snapshots or save them from the agent. Persist only the node ID into current YAML, keep secrets in Vault, restore prior local state on save failure, and never retry uncertain registration automatically.
- Verify Uptime Kuma lifecycle, EvoMap registration/revocation and configuration preservation regressions.

### Webhook publication
- Mission webhook registrations use stable mission keys, replacing the old callback and removing it on disable, delete, manager replacement or shutdown. A-B-A trigger changes must not accumulate callbacks.
- Incoming webhook handlers obtain current immutable config and Guardian services together under the server config lock. Disabled integrations reject new deliveries; scan policy changes take effect without restart.
- Webhook config load errors fail closed and preserve the source. Validate a detached candidate, persist atomically, then publish; failed create/update/delete and signature migration must preserve the last valid in-memory state. Return detached read values.
- Verify webhook transaction, callback roundtrip and live policy regression tests plus mission suites.

## Desktop invocation ownership

- Desktop-owned shared integration routes use server-selected handlers and keep
  original integration gates. Never trust a header or client origin assertion as
  Desktop ownership. Readonly has stable HTTP 403 code desktop_readonly; it also
  revokes background owner contexts and rejects late local publication.
- Stop/pause/cancel routes require the normal authenticated scope but bypass
  readonly write admission. Body-selected actions (Detective) are decoded and
  classified once; finish/resume/new execution remain writes.
- Game Maker policy publication and agent phases, Detective reports, queued
  local missions, mission preparation and Virtual Computer tasks retain this
  ownership across HTTP completion. Server cancellation drains before stores close.
- Desktop HTTP/SDK file and archive contracts are in documentation/desktop-api.md.

## Verification

- `preview_gateway.go` dispatches configured guest hosts outside the main-site
  auth/CSP router, after trusted-proxy normalization. One-use launch grants and
  host-only partitioned cookies retain resource and parent-session ownership;
  expired/revoked sessions close both sides of upgraded connections. Never send
  the boringd management bearer through its unauthenticated guest web route.
  Preserve guest Cookie, Authorization and CSRF headers while removing AuraGo
  credentials. `server.preview_domain` prepares hosts; `preview_enabled` stays
  off until the acceptance procedure in `documentation/preview-isolation.md`.
  No shared-origin fallback after activation. Verify `TestPreviewGateway*`.
- Desktop HTML, script inlining and fallback file serving all use rooted opens.
  A prior path/integrity check does not authorize a later unrooted read. Preserve
  contained links and verify `TestDesktopHTMLServingContainsSymlinks` under Linux.

- Desktop authority is enforced by `desktop_operation.go` in addition to token
  scopes and integration policy. Readonly blocks writes and execution even for
  administrators; reads and explicit stop actions remain available. Config
  publication revokes admitted request/background runs. Bounded local results
  use `publishDesktopResult` or the inherited file publication gate; never hold
  that gate during provider/network work. Keep the shared Desktop service alive
  when only readonly changes, so readers, stores and encrypted drafts survive.
- Shared integrations used by Desktop enter through the administrator-only
  `/api/desktop/integrations/` allowlist, retaining the original handler's
  permissions. Client Origin/header claims never select Desktop policy.
- HTTP bootstrap and WebSocket welcome/events share the same scope projection.
  Scoped non-admin clients receive no administrative settings, provider choices
  or management events. Keep WebSocket writes serialized and recheck token
  revocation; policy events update open browsers without remounting editors.
- Verify `TestDesktopReadonly*`, `TestDesktopRevocation*`,
  `TestDesktopIntegrationRoutes*` and `TestDesktopHTTPAndWebSocket*`.

- Run `go test ./internal/server` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
