# Server integrations

## Purpose

Server-owned HTTP and cross-component integration contracts.

## Ownership

- `/api/qr/` shares the native `internal/qrcode` core and expensive-work admission with the agent tool. Use existing Desktop auth/CSRF/scopes: render/batch/import-image require write, presets/scan/config require read; human access is independent of agent QR enablement. Bound strict JSON/raw images, return no-store/nosniff/sandbox responses and never persist previews. Public image import additionally requires network permission and the shared SSRF-safe transport. Preserve cancellation and row errors; invalid batches never return ZIP bytes. Verify `TestQRCode*`; `internal/qrcode/AGENTS.md` and `documentation/qr-code.md` own format/limits.

- Decisions is constructed passively from the saved snapshot and trusted install
  directory. Start it on the server lifecycle, inject `flowDecisions` into the
  flow services before Mission Control starts, and drain it after flows and on
  early server exit. Configuration replacement publishes CPU revocation and
  disablement immediately; shutdown errors never restore permission. Call
  `PrepareReconfigure` during publication, but run returned cleanup outside
  `CfgMu` and complete it before restarting the runtime.
- `/api/decisions/` is administrator-only, including status and evaluation.
  Session mutations require an exact same-origin Origin; validated admin bearer
  tokens use the existing scope boundary. Bound request bodies, preserve context
  cancellation, and expose stable error codes rather than process output or
  paths. Install/start may outlive the request, never the server. One HTTP
  install or start is admitted at a time; another returns `lifecycle_busy`
  without starting a second operation.
- Provider-backed Decisions uses the same rooted input path and budget checks.
  The task router receives a passive generation-bound scoring capability, not
  credentials or lifecycle methods. It must be wired to every eligible channel.
  Its calls book reported provider cost once under `routing`; other Decisions
  calls retain `decisions`. Revocation invalidates cache identity and late results.
  Provider credentials stay in the Vault; configuration/provider changes cancel
  active calls and discard late results. Passive status must not perform remote
  inference. Local lifecycle actions are unavailable in provider mode. Catalog
  lookups use saved connections only. Unknown paid outcomes remain non-retryable
  through the Flow adapter, even when a node deadline also expired.
- `decisions_media.go` resolves registered media or existing authorized outgoing
  attachment paths to rooted regular-file handles. Never accept caller URLs as
  download authority. Enforce eight images, 12 MiB each, 24 MiB total, and 32
  million pixels per image; enforce separate source and normalized byte totals.
  Normalize GIF and WebP to one bounded PNG frame using the existing Go
  decoder. Flow core receives no filesystem capability. Verify `TestDecisions*`;
  manager and native contracts live in `../decisions/AGENTS.md` and
  `../../decisions_runtime/AGENTS.md`.

- Desktop cloud storage routes under `/api/desktop/storage/` require admin
  access and apply Desktop, integration and mount rights independently.
  Visibility changes revoke active storage contexts. Transfer preflight enforces
  file, entry, depth and aggregate-byte budgets; verified copies precede
  conditional source cleanup. Uncertain results retain sources and report
  partial completion. Revalidate source size and metadata before staging is
  published; incomplete download bodies must abort the HTTP response. Revocation
  must also interrupt a paused upload's blocking request-body read.
  Provider authentication/permission failures and unavailability are distinct
  from readonly; preserve the provider HTTP status without forwarding its body.
  See `documentation/desktop-cloud-storage.md` and
  `TestDesktopStorage*`.

`internal/server` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Managed RustFS S3

- `/api/rustfs` routes are admin-only; browser mutations require same-origin CSRF validation, an empty JSON object, strict methods and at most 1 KiB. Lifecycle POSTs return accepted operation receipts; status and preflight never start a service. Codes are sanitized and credentials are never returned.
- Construct one `rustfs.Manager` on the server lifetime. Config snapshots retain private function callbacks, never a reflected manager pointer. `PrepareReconfigure` runs before persistence under `CfgSaveMu`, outside `CfgMu`; `Configure` performs only in-memory revocation before publication. Drain on shutdown without stopping established storage.
- Complete each prepared save: confirmed abort restores the published configuration and unaffected external leases; a pending managed IAM transition stays blocked until verified recovery. A durable write installs the validated authority even if reload fails. Unknown setup rollback leaves access blocked. Never revive previously issued leases or clear an uncertain managed mutation.
- The saved S3 probe resolves the selected provider. Managed probes use child credentials and scoped reads; missing read authority becomes `read_verification_unavailable`, including BLOCKED_ACCESS in self-tests. Dashboard configuration flags remain separate from observed runtime status.
- Runtime/storage/permission contracts: `../rustfs/AGENTS.md`; operator guide: `documentation/rustfs.md`. Verify `TestRustFS*`, `TestManagedS3*`, existing integration probes and config-save tests.

- `/api/self-tests` and its catalog, run, cancel and download subroutes are admin-only and `no-store`; browser mutations require a matching Origin. Runs bind a saved configuration snapshot, stop new cases after configuration replacement and drain before integrations close. Reuse only reviewed read-only saved-config probes. Never expose raw bindings, credentials, model prose or tool payloads through reports. See `../selftest/AGENTS.md` and `documentation/agent-self-tests.md`; verify `TestSelfTest*`.

- `/api/tts/phonon/voices` is an admin-only passive catalog/resource read.
  Voice downloads are admin/CSRF-protected POSTs accepting pinned catalog IDs only;
  they never change the selected voice or provider. Chat/AgoDesk/SIP readiness
  recognizes `phonon` and preserves explicit Speech Lab routing. See
  `../phonon/AGENTS.md` and `documentation/phonon.md` for shared resource rules.
- `GET /api/tts/routes` is an admin-only no-store summary of the four effective
  speech paths. It names the provider, readiness and a stable code, and it
  never returns a URL, Vault key or voice ID. Readiness means the route has a
  configured backend. Live Speech is ready when its provider needs no key, or
  the key is present.

- Action policy catalog/preview and pending/decision APIs are administrator-only, as are generic config updates and Vault secret mutation. Preview never dispatches or creates approvals. Decisions accept only the server-owned action ID and approve/reject, preserve its originating session and revalidate current gates. Tool Bridge uses expiring originating-run capabilities, current and inherited tool/SQL scopes, loopback transport and bounded calls; a process-wide internal token cannot authenticate bridge execution. Unknown or active workspace `/files/` content is a sandboxed attachment; preserve known audio/image/video playback.
- `POST /api/media-conversion/test` is administrator-only (`isAdminProtectedPath` plus `requireAdmin`) and executes only an approved converter: the configured value is resolved through `internal/mediabins` exactly like the tool resolves it, so a bare name on PATH is accepted, and the base name of the resolved file must identify ffmpeg, ffprobe or magick (a versioned spelling such as `ffmpeg-7.0` is accepted; the legacy `convert` only outside Windows). A missing or foreign binary is refused before a process starts. `tools.media_conversion` is validated on every configuration save path (`/api/config` and the setup path) against the values the patch itself changes, so a pre-existing out-of-range value in `config.yaml` never blocks an unrelated save; `config.Load` clamps it at startup. Verify `TestValidateMediaConversionBinaryPath*`, `TestHandleMediaConversionTest*`, `TestValidateMediaConversionPatch*` and `TestHandleUpdateConfigValidatesMediaConversionPatch`.

### Quick Connect serial access

- `/api/desktop/serial/ports` and `/connect` use Desktop admin authorization,
  including the existing auth-disabled behavior, and the default-off
  `virtual_desktop.serial_host_enabled` grant. Connecting also requires Desktop
  execution permission; readonly permits listing but blocks opening a port.
- `desktop_serial.go` projects host/browser grants into bootstrap and live
  Desktop policy events without recreating the Desktop service. Revalidate
  authorization and grants on every serial operation and cancel idle sessions
  on revocation, cookie expiry or server shutdown. RX never renews a session.
- Deny Web Serial through Permissions-Policy outside the trusted Desktop page
  and the exact `/config` document (CYD web flasher, which stays unframable and
  MIDI-denied); embedded apps deny it through their frame policy too. Never log
  payloads or expose them to an LLM. Port/protocol contracts live in
  `internal/desktop/AGENTS.md`.
- Verify `TestDesktopSerial*`, `TestCYDFlasherConfigPageAllowsWebSerial`, the
  Desktop proxy tests and the simulated browser checks. See
  `documentation/quick-connect-serial.md`; USB hardware acceptance and
  deployment remain separate from local test/build evidence.

- MCP connection tests require enabled MCP gates, test only the selected server and bind Vault aliases to its saved launch configuration. The automatic Dograh client grants private access only to the managed service's exact origin. Local/Docker stdio observes shell, sandbox, unsafe-host and Docker mutation grants at launch.
- `desktop_store_printguard.go` owns admin config/test routes for Store ID
  `printguard`, mandatory isolated HTTPS launch, exact origins and local Engine /
  own-network validation. Support the bundled Compose `docker-proxy` only for
  containerized AuraGo; exclude internal/control networks from app choices.
  Setup probes the configured preview host without
  credentials. Token removal/access-off must still work when setup is broken.
  GET never returns secrets. The managed `printguard-store` MCP bridge requires
  both global gates, fresh anonymous-401/authenticated-read/tool-discovery proof
  and an execution-time grant check. Desktop enabled/read-only changes also
  revoke and rebuild MCP through the config hot-reload path. Revoke before lifecycle changes; reconnect
  only after confirmed start, preserving custom MCP connections. Verify
  `TestPrintGuard*`, `TestManagedMCP*` and `TestPreviewGateway*`; operator contract:
  `documentation/printguard.md`.
- The incoming MCP endpoint checks Host against configured names/local addresses independently of Origin. Configure server.host or server.https.domain for an external name. Never resolve a request-supplied hostname as authority. MCP sessions are random, signed, expire after 24 hours and bind to the authenticated credential; stateless requests get independent sessions. The MCP allowlist is a hard scope for direct, wrapped and ask_aurago calls; enabling the IDE preset must not widen an explicitly selected list.

- Integration connection tests bind stored credentials to saved targets. A Dograh target override requires an explicit credential. YepAPI tests accept POST only, use the saved base URL and read the free model catalog; they never create a paid search or claim that a public catalog proves key validity.
- `RegisterLLMSecrets` (`llm_secrets.go`) runs in `cmd/aurago` right after `ApplyOAuthTokens` (before the LLM client exists), at the top of `Start` and in every `replaceConfigSnapshot`; the OAuth authorize path (`applyOAuthTokenToRuntime`) and the refresher publish through `replaceConfigSnapshot`, so new access tokens register too. Static LLM/provider keys and OAuth client secrets go to the permanent `security.RegisterSensitive` registry; OAuth access tokens from every configured provider (read through the Vault, including SIP-only and inactive providers) and resolved LLM slots use scoped registrations released on rotation (bounded registry). Pass the Vault reader at startup and publication; tokens must never enter the permanent registry. Values under 8 bytes are never registered, and placeholder keys are skipped: a key needs a `[A-Za-z0-9]` run of at least 8 characters mixing letters and digits, or 16 letters mixing case, so `lm-studio` or `ollama-does-not-need-a-key` cannot redact words. Verify `TestRegisterLLMSecrets*`, `TestLLMSecret*` and `TestApplyOAuthTokenToRuntime*`.

- Chat ingresses share `SQLiteMemory.BeginSessionTurn` with bots and loopback.
  `lockChatSessionTurn` retains server activity counters only; do not also acquire
  the legacy server-local mutex. Register cancellation before waiting, including
  AgoDesk active-run cancellation, and hold the gate through response delivery.
  The agent executor never relocks this gate. Verify
  `TestServerIngressSharesMemoryTurnGateAndCancelsQueue` and channel ingress tests.
- Telegram, Discord and Rocket.Chat receive the server-owned budget tracker and the shared isolated context recap formatter. Telegram worker admission, transcription, typing and agent execution inherit the polling owner context; cancellation stops queued work and ongoing model work.
- Long-lived chat bots capture `chatRuntimeSnapshot` once per new message so
  configuration and client come from the same publication; accepted turns stay
  frozen. Telegram and Discord startup require both this callback and the
  Decisions capability, including Discord's config-reload restart path.
- Desktop turn preparation accepts an explicit original `TaskRoutingText`.
  AgoDesk pins it before attachment enrichment or default text, preserving the
  original handoff message; MeshCore supplies only `msg.Text`. Empty originals
  remain empty. Routing never receives attachments, handoff transcripts or radio
  metadata, while the ordinary chat request retains its authorized context.

### Retro-Net Terminal

- `GET /api/desktop/retronet/directory`, `POST /api/desktop/retronet/status`
  and `GET /api/desktop/retronet/connect` (`desktop_retronet*.go`) check, in
  order and before any WebSocket upgrade: Desktop write scope plus the
  operation class (connect is Desktop execution), the live `retronet_enabled`
  grant (`virtual_desktop.retronet_enabled`; false when the Desktop is
  disabled, readonly or without write scope; 403 `retronet_disabled`), the
  method, then for connect a known entry ID (404), a same-host `Origin` (403,
  empty refused) and a WebSocket upgrade (400).
- The setting defaults on (template and Go default in `config.Load`, so also
  when the key is missing): an owner decision and a deliberate exception to the
  root default-off rule for harmful capabilities. An explicit `false` stays off;
  no migration flips it. `GET /api/config` shows the loaded value where the
  YAML has no boolean (`injectRetroNetDefault`), so a Config save never writes
  a false the user did not choose (`TestVirtualDesktopRetroNetToggleDefaultsOnAndPersists`,
  `TestDesktopRetroNetPolicyFollowsLoadedConfigDefault`,
  `TestConfigRetroNetDefaultSurvivesGetAndPut`).
- The browser sends only an entry ID plus `cols`/`rows`; the server looks the
  entry up in the catalog or `retronet.entries` and runs it through the one
  lazily created `retronet.Manager` (limits shared by all users) and its
  guarded `Dialer` (public addresses only). Revalidate policy and
  authorization every second; revocation ends the session with
  `NO CARRIER`/`disabled`, integration shutdown or HTTP drain with
  `server_shutdown`. When the adapter's reader ends (`browserGone`) the
  session context is cancelled with `retronet.ErrClientGone`
  (`remote_closed`) at once, also while it still dials or runs the SSH
  handshake. Keepalive: a ping every 30 s under the write lock, a 75 s read
  deadline extended by every frame and pong (`retroNetPingInterval`,
  `retroNetReadTimeout`), so a half-open browser ends the session too.
- `directory` returns catalog then own entries, cached status, `stale` and
  `can_edit` (administrator and not readonly); `status` asks for a probe and
  waits at most 10 s. Both pass the full directory to the prober.
- A first-contact SSH key is persisted only when the session's caller is an
  administrator (`withRetroNetHostKeyPersistence`, fail closed), through
  `SetRetroNetHostKey` with the dialed entry from the session context
  (`withRetroNetDialedEntry`; missing means no store), then announced as a `desktop_changed`/`set_settings`
  event that `filterDesktopEvent` keeps from non-admin clients.
- Audit `desktop_retronet_connect` (attempt, plus `blocked`/`upgrade_failed`
  when the upgrade fails) and `desktop_retronet_session` (code, reason,
  target, byte counts, duration). Never log or audit payloads.
- Verify `go test ./internal/server -run RetroNet -count=1`. Engine contracts
  live in `internal/retronet/AGENTS.md`; operator docs in
  `documentation/retro-net.md`.

### Infra atlas (`desktop_infra*.go`)

- `/api/desktop/infra` (and the trailing-slash subtree) is registered through
  `registerDesktopInfraRoutes`; `handleDesktopInfra` owns the route table:
  `GET snapshot`, `GET candidates`, `GET targets`, `POST nodes`,
  `PUT|DELETE nodes/{id}`, `GET|PUT nodes/{id}/source-edit`,
  `POST nodes/{id}/actions`, `POST import`, `POST edges`,
  `PUT|DELETE edges/{id}`, `PUT layout`. Every route requires the Desktop
  administrator scope; non-GET requests additionally pass
  `checkDesktopOperation(desktopWrite)` and the Desktop/Infra read-only gates
  (`infra_readonly`, `infra_policy_changed`).
- `Server.startDesktopInfraStore` initializes the store during startup and
  `desktopInfraRuntime` caches a failed initialization, so the schema migration
  and its database backup never run inside a request. `drainDesktopInfraRefresh`
  runs after the HTTP drain and before the shared inventory database closes.
- Provider refresh is scheduled at most once per provider instance and shared
  through `runtime.cache` (30 s success TTL, 5 s error TTL, 45 s for FRITZ!Box
  Hosts). Read paths that refresh (`?refresh=1`, candidates, import) register
  through `desktopInfraRefreshContext`, so config revocation cancels them like a
  write. The observation write phase uses `context.WithoutCancel` with a 5 s
  budget: a client disconnect must not mark a healthy source stale.
- `Snapshot` and `Candidates` never expose provider payloads. `source_errors`
  maps `kind|instance` to one bounded code (`provider_unreachable`,
  `auth_failed`, `observation_limit`); a failing provider is logged once per
  failure streak and its recovery once. Never return provider error text.
- Actions reserve in `infra_action_requests` before dispatch, revalidate the
  current configuration and source link, and finish with `completed`, `failed`,
  `unknown`, or `accepted`. The Docker protection list is
  `containerProtectedOwners` for both the observation (`Protected`) and the
  dispatch (`managed_container` reason). Proxmox and FRITZ!Box writes run under
  `desktopInfraContext` and report `unknown` when that budget expired.
- `PUT /layout` answers `{"nodes": [<changed nodes>]}`; it is never a snapshot.
  Clients merge the patch and only an explicit `snapshot` field replaces the
  atlas. `desktopInfraNodeRoute` maps `ErrHasChildren` to 409 `has_children`;
  `desktopInfraStoreError`/`desktopInfraProviderError` own the error mapping.
- Verify `go test ./internal/server -run 'Infra' -count=1`, the Infra package
  tests, `node scripts/test-infra-ui.mjs` and the opt-in
  `TestDesktopInfraBrowser*`. Store contracts live in `internal/infra/AGENTS.md`,
  operator docs in `documentation/infra.md`.

### HTTP trust and shutdown boundaries

- Desktop agent chat, its stream and log APIs require administrative Desktop access. Log tail/search/stream/download scrub registered secrets and credential fields. Passive media proxies reject active HTML/SVG/XML; every inline Knowledge document uses sandbox CSP. Garage HTTP and WebSocket proxies remove local cookies and authorization headers before forwarding.
- Logout is same-origin POST and persistently revokes the presented session until expiry. Nonce-bearing sessions remain independent; revocation persistence failure does not report success. Login combines bounded per-IP state with account verification concurrency/backoff rather than letting one attacker globally lock the owner out.
- `POST /api/tokens` accepts only scopes that some `TokenManager.Validate` call checks (`knownTokenScopes` in `token_scopes.go`, plus the desktop remote device/tag prefixes with a non-empty suffix that has no surrounding whitespace), refuses `cyd` combined with anything and refuses the CYD Creator device scope `cyd_creator_device` (minted per flashed board by CYD Creator, never stored in `tokens.json`; "CYD Creator" below). Add a scope there when a handler starts checking it; the allowlist is creation-time only, so stored tokens keep validating. Verify `TestCreateTokenValidatesScopes`.
- Read TokenManager through one synchronized snapshot per operation. Stop HTTP acceptance, cancel active handlers, and close tracked hijacked WebSocket connections before waiting for handlers to drain; request cancellation alone does not unblock WebSocket reads. Close dependencies only after the drain. Re-panic `http.ErrAbortHandler`; never write a replacement JSON response over a partial proxy response. Verify `TestHTTPDrainClosesHijackedWebSocketBeforeWaiting`.
- Cast LAN listeners require a file-specific ticket, reject listing and keep bounded header/idle/stream-write budgets. When login is disabled, the main `/tts/` and `/cast-media/` listeners require a ticket too. Web chat voice output includes that ticket on the audio path when login is disabled. `GET /api/chromecast/discover` and `POST /api/chromecast/test` are administrator routes. The test route speaks a fixed sentence on one saved device and is refused while Chromecast is disabled or read-only. Store preview calls a configured loopback origin with no redirects; request Host cannot choose it.
- Every static mount under `/files/` and the `/files/desktop/` index, widget and inlined sibling-script reads go through `neuteredFileSystem{rootBoundFileSystem(dir)}` (`fileserver.go`): regular files only (on Windows also non-link reparse files Go reports as irregular), a fresh `os.Root` per request, links followed only while they stay inside the directory (absolute in-root links and Windows junctions through the resolve fallback, whose final open is still root-bound and keeps the requested name), resolution failures and every other non-permission failure 404. Never use `http.Dir` in production code under `internal/server`; `internal/audit` `TestServerFileMountsNeverUseHTTPDir` enforces it. Verify `TestRootBoundFileSystem*` and `TestUIRoutesFilesMountsAreRootBound`. `castMediaAssetHandler` refuses every link by design: its single-component names are checked with `Lstat` and must be regular files.
- Signed AgoDesk media (`/api/agodesk/media/<bucket>/...`) and TTS (`/api/agodesk/tts/<file>`) assets open through `openRegularFileInRoot` (`fileserver.go`) and `http.ServeContent`, never `http.ServeFile`: per-component `Lstat` under an `os.Root`, no link followed even inside the bucket, intermediate components must be directories, the last must pass `isServableMode` before it is opened (a FIFO would block the open; the open itself is also non-blocking on Unix), every failure 404, no directory listing. `os.Root` refuses absolute links by itself but follows relative in-root ones, so the `Lstat` loop is what refuses those. Media without an extension type gets a sniffed type only when it is image/audio/video/PDF/`application/ogg`/plain text (`agodeskMediaAssetSniffedTypes`), else `application/octet-stream`; `Content-Disposition` is built with `mime.FormatMediaType`.
- Contrast: `/files/` follows in-root links because users create them through the desktop `CreateSymlink`; the AgoDesk media buckets and TTS cache are written by AuraGo itself, so no link is followed there. Verify `TestAgodeskMediaAssetRequiresSignedURLForAllowedFiles`, `TestAgodeskTTSAssetBypassesSessionAuthAndServesCachedAudio` and `TestOpenRegularFileInRootRefusesFIFOWithoutOpeningIt` (Unix).
- Verify codequality boundary/lifecycle tests, Remote handshake tests and the existing Desktop token/proxy matrix. A local fixture does not establish external delivery or device acceptance.

### Administrator container API

- Ollama Recreate preflights saved config and writable Docker, returns 202 with an operation receipt, and shares the cancellation-bound serialized runtime used by startup/config-save. Status is passive and distinguishes API readiness, model pulls and terminal errors from Docker running state. The explicit Ollama connection test uses the saved target even when a URL override is supplied; arbitrary private overrides do not gain trust. Generic container protection recognizes the Ollama owner and legacy reserved names. Stop completion must be confirmed before Docker writes are revoked. Verify `TestOllamaReadOnlyRefusesConfigAndRecreate` and `TestOllamaConnectionTestUsesSavedEndpoint`.

- `/api/containers` and `/api/containers/` stay in `validRouteBearer`'s admin catch-all: browser sessions or Bearer tokens with the `admin` scope; every other scope gets 403 `invalid_bearer_scope`. Do not add the paths to desktop, go2rtc, bypass or lockdown lists, and do not wrap them in `requireAdmin` (it would refuse non-admin tokens on auth-disabled installs, which reach the handler today). System World calls `handleContainerAction` directly for start/stop/restart under its own `desktop:admin` gate. Verify `TestContainerRoutesKeepAdminScopeThroughAuthMiddleware`, `TestContainerRoutesStayInTheAdminBearerCatchAll`, `TestContainerRoutesStayOpenWhenAuthIsDisabled` and `TestContainerRoutesAuthDisabledIgnoreBearerScope`.
- Terminal, update and remove classify the target first (`containers_protection.go`): the agent docker tool's owner list (also for a 404 on a reserved name), the container AuraGo runs in and the container serving `docker.host` (one of its network addresses equals an address the endpoint host resolves to). Self needs the Docker runtime plus either a default hostname that prefixes the container ID or AuraGo's own ID from the `/etc/hostname`/`/etc/hosts`/`/etc/resolv.conf` bind mounts in `/proc/self/mountinfo` (mount root `…/<id>/<file>`, so a dedicated `containers` mount matches too), falling back to cgroup v1 `/proc/self/cgroup`; both signals are read once per request. Docker gives every container that joins another's network namespace (`network_mode: container:`/`service:`, e.g. a Tailscale or Gluetun sidecar) the provider's hostname and `/etc` files, so the signals cannot tell AuraGo from its provider: when any listed container's `HostConfig.NetworkMode` references the named container (full ID, an ID prefix of at least 12 hex characters, or name; `TestContainerNetworkModeJoinsMatchesFullIDLongPrefixOrName`), neither it nor the joining containers count as self; they are `shared-network` (confirmation). A third self signal survives network sharing: the overlay `upperdir` of `/` in mountinfo equals inspect `GraphDriver.Data.UpperDir` (overlay2 graph driver only; the containerd image store reports none, verified on aurago-test 2026-10-06), and then the target is `self`; the list inspects only the shared-network group for it and needs exactly one match (`containers_self_proof.go`, `TestOwnOverlayUpperDir*`, `TestContainerUpperDir*`). Where no upper directory is reported (the containerd image store), a marker file decides the group: at startup (`server.Start` calls `initContainerSelfMarker`, Docker runtime only) AuraGo writes `.aurago-self-<32 hex>` into the first of `$TMPDIR`, `/tmp`, `/var/tmp`, `$HOME` whose covering mount is `/` (its private writable layer; never a volume, bind or tmpfs, which could be invisible to the archive API or shared), and each group member is asked `HEAD /containers/{id}/archive?path=<marker>`. Every member must answer 200 or 404 and exactly one 200 proves self; a refusing proxy, an engine error or two matches (a committed copy) keep the confirmation. Known answers are cached per engine and container, unknown ones are not (`TestContainerSelfMarker*`, `TestWriteContainerSelfMarker*`, `TestInitContainerSelfMarker*`; live: `TestContainerSelfMarkerLive` with `AURAGO_LIVE_SELF_MARKER=1` inside a scratch container that joins a scratch provider). The action path makes that one list request only when the signals name the target or the target joins another namespace; a failed list or a failed inspect make the target `unverified` (confirmation), never silently allowed. A failed endpoint lookup falls back to the remote IP of AuraGo's own Docker connection (`httptrace` `GotConn` on the inspect; the list pings `/_ping` for it), but only a listed container with that address proves the endpoint; no observed TCP connection, a loopback or host address, a failed list, a target in the connected container's network namespace (`network_mode: container:`/`service:`; it may serve the port), a target with the connected container's compose service name in any project, or a target whose name, compose service, network alias or DNS name equals the `docker.host` host name (or its first label) keep the target `unverified` as before (`TestContainerEndpointLookupFailureUsesTheConnectionAddress`, `TestContainerEndpointFallbackNeverLoosensAnUnprovenTarget`, `TestDockerEndpointByConnectionKeepsATargetNamedLikeTheHostUnverified`, `TestContainerAnswersDockerHostName`, `TestContainerEndpointLookupFailureNeedsConfirmation`). The list marks no endpoint container when the lookup fails but keeps every other flag. The list reuses one lookup per host for 30 s (failures too) and warns once per failure streak, then at most every 10 minutes; actions always resolve again (`TestAdminContainerListCachesTheEndpointLookup`, `TestContainerActionsResolveTheEndpointEveryTime`). Podman is not covered (no `/.dockerenv`, files under `overlay-containers/<id>/userdata/`): self is never proven there, and the app container falls back to the name/label confirmation. Protected or unverifiable targets need `confirm=protected` (409 `container_protected_confirmation_required`, one message per label); update on proven self or the Docker endpoint always answers 409 `container_self_update_unsupported`. The terminal checks the WebSocket origin and then the WebSocket upgrade before classifying, so a cross-origin handshake or a plain GET causes no Docker request and no DNS lookup. Start, stop, restart, pause, unpause, logs, inspect and stats never consult protection (`TestContainerPauseAndUnpauseRoutesReachDocker`). `GET /api/containers/{id}/protection` reports the same classification read-only (`TestContainerProtectionReportMatchesTheTerminalRules`, `TestContainerProtectionReportStaysAdminOnly`); the page uses it when a handshake closes before `onopen` and never sends `confirm=protected` without the operator's confirmation. `GET /api/containers` adds `protected_owner`, `self`, `docker_endpoint` and `shared_network`; SSE `container_update` stays unannotated and the page merges the last list's flags. Verify `TestContainerProtected*`, `TestContainerUpdateRefuses*`, `TestContainerConfirmationMessagesNameTheReason`, `TestContainerTerminalChecksOriginBeforeProtection`, `TestContainerLifecycleActionsNeverConsultProtection`, `TestClassifyContainerForAction*`, `TestContainerEndpointLookupFailureNeedsConfirmation`, `TestContainerSelfSignalsIgnoreASharedNetworkNamespace`, `TestContainerSelfNeedsTheListToRuleOutASharedNamespace`, `TestContainerIsSelfProvenByMountinfoWithCustomHostname`, `TestOwnContainerID*`, `TestAdminContainerList*` and `TestContainersScriptConfirmsProtectedContainersBeforeSendingTheFlag`.
- `containerProtectedOwners` (`containers_protection.go`) and the agent docker tool's `tools.DockerContainerOwnership` list (`internal/agent/agent_dispatch_services.go`) name the same owners: `aurago-app`, `acestep`, `homepage`, `go2rtc`, `local-llm`, `boring-garage`, `cyd-creator` (the CYD Creator builder `aurago-cyd-builder`), `decisions` (the managed Decisions runtime and discovery containers) and `security-proxy`. A new managed owner joins both lists and the `protected_owner` list in `documentation/manuals/*/21-api-reference.md`. Verify `TestContainerProtectionCovers*`.
- Container and Store terminals reject a request that is not a WebSocket upgrade (400) before any Docker work and before any exec exists; the container terminal does so before the protection lookup too (`TestContainerTerminalRejectsPlainGETBeforeProtectionLookup`). Closing a terminal does not signal the exec: no EOF and no Ctrl-C, so a tmux or screen session, a running job and the shell survive the modal closing; never send EOF or type `exit` into the TTY (it would end a multiplexer at an empty prompt). Only the explicit `{"type":"end"}` text message ends a shell (typed and pasted input arrives as binary frames): `CreateSession` tags each exec with `AURAGO_TERMINAL_SESSION`, and `dockerContainerTerminalSession.End` runs a detached helper exec that sends SIGHUP to the PPid-0 process carrying that tag. Store terminal execs carry the tag too but have no End button. Verify `TestContainerTerminalRejectsPlainGET*`, `TestDesktopStoreTerminalRejectsPlainGET*`, `TestContainerTerminalClosesWithoutEOF`, `TestDesktopStoreTerminalClosesWithoutEOF`, `TestContainerTerminalEndMessageEndsTheShell`, `TestContainerTerminalCloseAndOtherMessagesNeverEndTheShell`, `TestContainerTerminalBinaryEndTextNeverEndsTheShell`, `TestDockerTerminalSessionEndRunsATaggedHangupExec`, plus `TestDockerTerminalEndLive` (`AURAGO_LIVE_DOCKER_TERMINAL_END=1`) on a Linux Docker host. Each browser input write re-arms a `containerTerminalWriteTimeout` (5 min) deadline, so a stalled dockerd stdin ends the session instead of blocking the handler (`TestContainerTerminalBoundsAStalledWrite`), and a session idle for longer than the timeout still accepts input (`TestContainerTerminalRearmsTheWriteDeadlinePerWrite`). Keep it long: a write also waits while a busy program does not read a large paste, and that must never end the session (`TestContainerTerminalWriteTimeoutStaysFiveMinutes`).
- Tool failures answer 502 with the unchanged `{"status":"error",…}` body; 503 stays "Docker disabled", 403 "read-only", 409 the protection codes. Never 401: the shared fetch wrapper redirects to login. Verify `TestContainerToolErrorsUseBadGatewayAndKeepTheBody` and `TestContainersListFailureShowsDockerMessageNotDisabledState`.
- `Start` calls `bindDockerSelfIdentity` (`docker_self_identity.go`): it binds `tools.DockerSelfIdentityFor` to the same self signals (`readContainerSelfSignals`, unchanged) plus one inspect of that container and one container list for `containerSelfInList`, cached for the process (a failed inspect or list is retried after 30 s), which yields the `com.docker.compose.project` label and the volumes (with their host `Source`) and binds mounted at or below the data directory (`Proven`). When another container joins the named container's network namespace (Gluetun, Tailscale), the signals name the provider: the identity is not proven, carries no mounts, and keeps the project only when the provider and every joiner carry the same project label (`TestDockerSelfIdentityDoesNotTakeASharedNetworkProviderForAuraGo`). Native runtimes get the zero identity and make no Docker request. The agent Compose policy uses it to refuse commands on AuraGo's own project. Verify `TestDockerSelfIdentity*`.

### Code Studio

- Managed terminal, Run, and Git status, diff and log retain the container
  image's environment with `sh -c`; login profiles must not hide installed
  toolchains. Explicit exec argument arrays stay unchanged. Verify
  `TestNormalizeCodeStudioExecCommandPreservesEnvironment` and the terminal tests.
- Save and upload share `code_studio_files.go`: bounded file bytes travel as a
  single generated regular staging file through the Docker archive API. Never
  embed contents in exec arguments. The normal container user verifies the
  staged digest and installs through no-follow directory descriptors.
- File GET and download resolve the physical path with `pwd -P` before
  reading bytes. An intermediate or final symlink whose target leaves
  `/workspace`, a non-regular file, or a probe path that fails
  `sanitizeCodeStudioPath` is HTTP 400 and is not base64-read. The response
  path is the physical path. Verify `TestCodeStudioReadRejectsSymlinkEscape`,
  `TestCodeStudioReadFollowsSymlinkInsideWorkspace` and `TestCodeStudioDownload*`.
- The Code Studio terminal rejects an empty or foreign `Origin` with HTTP 403
  before the container starts and before the WebSocket upgrade.
  `CheckOrigin` is `desktop.SameHostWebSocketOrigin`. Verify
  `TestCodeStudioTerminalRejectsEmptyOrForeignOrigin`.
- The Code Studio terminal attaches a TTY exec as user `developer` in
  `/workspace`. The command is `["/bin/bash"]` with no `-l`, or `["/bin/sh"]`
  when that create or start fails. `TERM=xterm-256color` and
  `AURAGO_TERMINAL_SESSION` match the admin terminal. Binary frames are
  keystrokes and output. Text frames are only resize and `{"type":"end"}`.
  Closing the socket does not send EOF or Ctrl-C. `{"type":"end"}` runs the
  shared hangup helper. `containerTerminalExecPayload` stays free of `User`
  and `WorkingDir`. If the TTY cannot be created, the handler logs
  `code studio terminal fell back to line mode` once, writes `{"type":"line"}`,
  and uses the line terminal. Verify `TestCodeStudioTerminalAttachesATTY`,
  `TestCodeStudioTerminalUsesSHWhenBashCannotBeCreated`,
  `TestCodeStudioTerminalCloseDoesNotEndTheShell`,
  `TestCodeStudioTerminalFallsBackToLineMode` and `TestCodeStudioTerminal*`.
- File PUT accepts additive `create_only`; New File must use it. Exclusive
  creation returns HTTP 409 on collision; overwrite publishes a complete sibling
  file atomically and preserves executable mode. Failed transfers leave the
  target intact. Keep Desktop/Docker write gates, workspace confinement and
  cleanup on failure. Verify `TestCodeStudioWrite*`, `TestCodeStudioUpload*`,
  `TestCodeStudioFailedTransfer*` and the Linux install-script tests.
- The line terminal remains the TTY fallback. It drains bounded WebSocket
  input in order, retains incomplete UTF-8/lines and collapses CRLF across
  frames. It ignores resize. Simple literal `cd` validates an accessible
  physical directory inside `/workspace` before changing session cwd; compound
  shell commands do not persist cwd. Stop on cancellation or socket failure.
  Verify `TestCodeStudioTerminal*` including the WebSocket test.
- Git status uses porcelain v1 with NUL-delimited records, including the second
  rename/copy path. Each change reports `staged` from the index column.
  Preserve filename bytes through diff requests. Search uses
  an explicit grep pattern operand (`-e`) and option terminator (`--`).
  It keeps the first 200 matches and sets `truncated` when more lines remain.
  An invalid line before that cap still fails the search. Verify
  `TestCodeStudioGitPorcelain*`, `TestCodeStudioSearchLeadingDash*` and
  `TestCodeStudioSearchCapsResults`.
- GET `/api/code-studio/status` omits `workspace_host_path`. `network` follows
  the configured mode until the container is running, then the inspected Docker
  mode. `read_only` mirrors the desktop grant. A failed inspect keeps the
  configured network and still returns status. Verify
  `TestCodeStudioStatusOmitsHostPathAndReportsInspectedNetwork`.
- Git branch changes are POST. GET lists branches and returns 405 when
  `action` is `create` or `switch`. POST runs `git checkout` or
  `git checkout -b` with the sanitized name as its own argument. Commit stages
  only the given paths with `git add --`, unless `add_all` is true. A commit
  with neither stages only the index. The message is a `git commit -m`
  argument. Missing name or email returns 409 `git_identity_required`.
  Identity writes only `git config --local user.name` and `user.email`.
  Verify `TestCodeStudioGitBranchGetDoesNotMutate`,
  `TestCodeStudioGitCommitStagesOnlyGivenPaths` and
  `TestCodeStudioGitIdentityIsLocal`.

### Game Maker Voxel persistence

- `game_maker_play_state.go` owns authenticated project `/play-state` GET/PUT/DELETE
  and the trusted `/play` host. Preserve desktop read/write scopes, feature/edit/
  delete gates, no-store responses, the 4 MiB ceiling and HTTP 409 on stale CAS or
  publication bindings. The parent-only play grant is distinct from asset tokens.
- Serve the host from verified UI resources. Its iframe retains opaque-origin
  sandboxing and cannot acquire credentials. Draft/test frames receive temporary
  state only. The owning runtime/storage contracts are in
  `internal/gamemaker/AGENTS.md`; verify `TestGameMakerVoxel*`.
- `GET /api/game-maker/projects/{id}/events` replays persisted events after
  `Last-Event-ID` or `?after=`, then writes `event: stream_ready` with no id
  before live updates. The marker is not stored. Verify
  `TestGameMakerSSEReplaysMoreThanOnePageWithoutGaps`.

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
- Planning, search, extraction and editing are separate bounded stages. Resolve budgets through `newspaper.ResolveBudget`: fixed retains legacy limits; explicit auto scales by requested topics to 400 originals, 128 searches, 64 overviews, 800 candidates, 31 stories and 60 minutes. Keep two planning calls, four reads with one per publisher domain, and two editors. Alternate topics and publishers, retain relevant exact passages and original publication dates, and allow one 24-hour-to-seven-day follow-up. Fixed mode charges overviews against shared pages; auto separates them. Reserve 40% of pages/searches and cross to follow-up by 50% of time without waiting for editors; stop discovery at 80%. Preserve monetary caps, exact evidence, bounded repairs and captured older originals. RSS retrieval pins public IPs at redirects and bounds XML input; all search/feed/overview entries remain leads until guarded original reads. Persist source evidence before editing and counters in optional run JSON. Publish only validated immutable revisions under current write permission.
- Story editing receives the actual selected topic/interest, uses the selected provider's output/context limits and JSON mode only when supported. Keyword matches may prioritize leads but cannot reject translated originals or synonyms; the editorial guide rejects unrelated content. Keep exact source-passage validation; when no story passes, the run reason reports bounded counts by failure class without exposing source text or provider responses.
- Email uses a writable configured SMTP account or AgentMail with POST retries disabled. Telegram uses only the configured user ID. Unknown send outcomes remain uncertain in the ledger and must not be replayed automatically.
- For email confirmation, expose sanitized structured outcomes: known AgentMail rejections include the HTTP status and release only that challenge; a 403 bounce suppression has its own actionable code. Transport/server ambiguity keeps its code and cooldown. Never return provider error bodies, credentials, recipient addresses or confirmation codes.
- Verify with `go test ./internal/server -run TestNewspaper` and the Newspaper browser test. Core state contracts live in `internal/newspaper/AGENTS.md`.

### HTTPS installation and transitions
- HTTPS defaults belong only to fresh configuration creation. Never enable it
  globally in `config.Load`; config-merger preserves the legacy effective
  enabled/mode/ports when fields are absent. Native initialization chooses a
  bindable 443 or 8443 and persists it. Explicit runtime ports never fall back.
- TLS listeners and probes honor `server.host`, including IPv6. Redirects use
  the actual HTTPS port and preserve escaped paths and queries. `http_port: 0`
  creates no HTTP listener, including ACME mode (TLS-ALPN-01 needs public 443).
  Keep the separate internal loopback HTTP listener and stream write budgets.
- Setup status exposes only HTTPS enabled/mode/port. The wizard never changes
  HTTPS. Config opt-out confirms feature loss, saves through the usual draft
  path, and returns a future URL only for direct access. Preserve public proxy
  origins and unknown port mappings. Never trust arbitrary forwarded headers.
- A successful HTTP opt-out clears AuraGo's HSTS rule on the current TLS save
  response and all remaining built-in TLS responses until restart. Failed saves
  keep HTTPS/HSTS intact. External, parent-domain and preload rules cannot be
  removed by AuraGo. Preserve the other security headers.
- Verify `TestHTTPS*`, `TestSetupHTTPS*`, `TestConfigRestartURL*`, security-header,
  internal HTTP and CLI healthcheck tests, plus `ui/TestHTTPSConfigOptOutBrowser`.

### Ingress Security and Browser Lab
- External chat ingress, SMS and SIP block raw input at local `ThreatHigh`; authenticated webchat and browser voice log findings without blocking. Voice always uses canonical escaped external-data isolation, even when supplied text already contains boundary tags. Keep current authorization snapshots and tool scopes.
- Quarantine replaces an otherwise agent-directed delivery with a fixed safe notice through the existing route. Apply the original eligibility and forwarding rules, including active matching missions; UI/log-only events with no agent destination stay local. Do not execute original payload callbacks or copy subjects, bodies or scanner prose. A failed notice is not acknowledged; accepted notices cannot replay because a label update failed.
- `PUT /api/ui-language` requires an authenticated admin session, accepts only supported locales and saves through the config serialization lock. Login/setup language selection is a local preview until authentication. Public security status returns only setup/lockdown necessities; repeated lockdown status reads must not touch Vault or log each request.
- Normalize provider, OAuth and runtime config completely before publishing a new auth/setup configuration snapshot. A failed normalization or persistence step must leave the previous live snapshot in place.
- `patchAuthConfig` callers hold `CfgSaveMu` (setup already does; the function never locks it). It stages `config.yaml`, then writes all auth Vault keys in one `Vault.WriteSecrets` batch, loads the candidate snapshot, and on any failure restores the previous YAML bytes and Vault values without publishing. Vault fields without a Vault are an error, never silently dropped. Verify `TestPatchAuthConfig*` and `TestVaultWriteSecrets*`.
- Browser sessions retain the configured initial `auth.session_timeout_hours`.
  Same-origin POST `/api/auth/activity` requires a still-valid signed cookie and
  extends its remaining lifetime to at least ten minutes. Never shorten a longer
  session, accumulate extensions, or revive an expired/invalid/revoked cookie.
  Renewal preserves the signed session nonce; a new login creates a new one.
  Logout revokes all versions of that session through their maximum renewal
  lifetime, atomically with renewal issuance. Honor legacy token-hash revocations
  after reload and upgrade legacy nonce-free cookies without changing identity.
  Preserve HttpOnly, SameSite=Strict and trusted-proxy Secure handling. GET auth status is
  non-cacheable and reports remaining seconds without renewing; ordinary API
  polling and streams cannot extend sessions. Verify `TestAuthSession*`.
- Forwarded host, scheme, and client IP count only when `server.https.behind_proxy` is enabled and the immediate peer matches `server.https.trusted_proxy_cidrs`; other forwarding headers are removed before auth and URL construction.
- An auth-disabled remote listener requires `auth.allow_unauthenticated_remote` before startup, config save or setup save (`applyConfigPatch`). The check uses the effective bind host, including `AURAGO_SERVER_HOST`. This exception never opens `/speech-lab/`.
- Disabling login retains same-origin checks on writes carrying browser Origin
  or Referer headers, including opaque Game Maker previews. Headerless native
  clients and safe asset reads retain their existing access. Verify
  `TestAuthDisabledBrowserWriteOrigins` and `TestGameMakerPreviewCannotWriteProjectAPI`.
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
  An optional `message` must exactly match a translated server-owned phrase for
  its kind and current UI language; preserve the agent's chosen waiting variant.

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
- `PUT /api/config` replaces `go2rtc.streams` wholesale. Before any Vault write it rejects, with 400, a value that is not a complete list: `null`, a non-list, a gapped numeric map, or an entry that is empty or lacks a valid, unique id. An explicit `[]` removes every stream. Verify `TestHandleUpdateConfigRejectsIncompleteGo2RTCStreamLists` and `TestHandleUpdateConfigSavesCompleteGo2RTCStreamList`.
- The same save rejects, with 400 before any Vault write, an incomplete `a2a.server.skills`, `a2a.client.remote_agents` or `three_d_printers.*.printers` list (`completeConfigListPaths`): `null`, a non-list, a gapped numeric map, or an entry that is empty or has no string `id`. Remote-agent `api_key`/`bearer_token` never reach config.yaml: masked placeholders are dropped, real values go to `a2a_remote_<id>_<field>`. A Klipper key is carried to a new id only for a real rename (the new id is unsaved and the old id left the list), never when a removal shifts printers up. Verify `TestHandleUpdateConfigRejectsIncompleteConfigLists`, `TestHandleUpdateConfigSavesCompleteA2ALists`, `TestHandleUpdateConfigKeepsKlipperKeysWhenPrinterRemoved` and `TestHandleUpdateConfigCarriesKlipperKeyToRenamedPrinter`.
- Enabling go2rtc must actively verify readable Docker container/image endpoints, the network endpoint when AuraGo runs in Docker, and mutation permission through a random nonexistent-container start probe that cannot create a resource.
- A go2rtc config save never starts a stopped sidecar on its own. The soft path (`Go2RTCManager.ApplyConfigured`) starts only on an explicit start (`forceStart`, the setup enable) or with `auto_start` while the administrator has not stopped it; a recreate (`ReconfigureContainer`, taken when the CPU/memory limits of person detection change, a stream is disabled or the container identity changes) also restarts a sidecar that was running. A sidecar that is verifiably not running (no container, or ours and stopped: `go2RTCInspectRunning` known and not running) is left alone: config is published, no reconcile error is reported (the save path and the `auto_start: false` tick share `clearErrorIfSidecarStopped`) and the administrator's stop stays set; a container under the sidecar's name that carries other labels is an unknown state, never "stopped", so the collision stays visible. When Docker cannot say, an administrator's stop still wins over the start-after-recreate. The background retry (`pendingRecreate`/`pendingStart`) is queued only for a sidecar that is meant to run (`*Go2RTCReconfigureError.StartIntended`), and every tick start or recreate (`backgroundStart`, also the auto-start path) re-reads the stop under the lifecycle lock, because an administrator stop holds that lock through its Docker call and sets the flag only at the end; a stop that wins drops the pending flags. The raised detection CPU limit is capped at the Docker host's `NCPU` at create time (`/info`, skipped up to one CPU or when the daemon does not answer); the fingerprint stays config-based. Verify `TestGo2RTCReconfigure*`, `TestGo2RTCApplyConfigured*`, `TestGo2RTCTickRetry*`, `TestGo2RTCTickDoesNotUndoAnInFlightAdministratorStop`, `TestGo2RTCTickDoesNotReportAVerifiablyStoppedSidecar`, `TestGo2RTCStartIntentWhenDockerCannotSay`, `TestGo2RTCStartContainerClamps*` and `TestGo2RTCConfigSaveKeepsTheAdministratorStop`.

### Provider management contract
- HTTP provider saves, AgoDesk upsert/delete, price application and protected Setup share provider validation, allowlisted YAML and reversible Vault writes. Capture current config and secrets inside `CfgSaveMu`; publish only after durable success. Report rollback failures. Price-only saves preserve active failover state.
- Setup stages every extracted credential, including gateway and integration secrets, through the shared config/Vault transaction. Reuse the normal field extractor; preparation never writes to the Vault. Known OAuth access tokens use exact scoped registration independent of the static placeholder heuristic, with release on rotation. Built-in embedding modes (`internal`, `external`, `disabled`, local Granite/Ollama) remain exempt from provider references and preserve their existing resolution.
- Provider reads remain arrays with opaque `ETag`. HTTP saves/pricing require `If-Match` (428 missing, 412 stale); AgoDesk uses the same revision. Every config publication and provider credential change invalidates it. Reject any `PUT /api/config` providers field with 400 `provider_use_api_providers` before secret extraction. The guarded initial Setup creation is the only revision-free bootstrap.
- Use `Config.ProviderReferences` for display, missing-reference checks and deletion protection, including disabled features. A used provider returns 409 `provider_in_use` with all references; AgoDesk force cannot bypass it. New dangling selections conflict; unchanged old invalid references remain repairable.
- Credentials use keep/set/clear semantics. A mask can only keep an existing valid value; explicit mask sets are invalid. Resolve copies under the save lock. Explicit different provider IDs never inherit parent credentials, even at identical origins. Empty selections inherit fully, including OAuth. Keep OAuth runtime access tokens separate from stored static keys.
- `GET /api/providers/status` is passive and authenticated: no network probes, secrets or inferred provider identity. `POST /api/providers/test` accepts only a saved provider ID and metadata/generation kind, freezes config/revision, limits to 20 seconds and one concurrent test per provider. Metadata never escalates to generation; generation is explicit, single-attempt and booked once through shared budget logic. Stale results never attest newer config. Protected Setup testing is metadata-only.
- Verify `TestProvider*`, `TestHandle*Providers*`, `TestAgodeskProvider*`, OAuth/secret suites and the UI provider browser tests. Operator and API guidance lives in `documentation/provider-management.md`.

### AI Gateway Contract
- Cloudflare AI Gateway routing must use provider-native segments where supported. In `auto` mode, unsupported providers must skip gateway routing and report a warning instead of silently falling back to `/openai`.
- Gateway probes and runtime preserve explicit custom endpoints; only canonical provider HTTPS endpoints may be rewritten. Report custom_endpoint locally without making a probe request.
- Workers AI uses the Cloudflare REST base (`https://api.cloudflare.com/client/v4/accounts/{account}/ai/v1`) with `cf-aig-gateway-id`; provider-native routes use `cf-aig-authorization` for the optional authenticated-gateway token.
- AI Gateway status checks must stay local and non-token-consuming; live Workers AI connection tests validate the Cloudflare API token/account via `/ai/models/search` and include `cf-aig-gateway-id`.
- The privacy-safe default is `log_mode: metadata_only`; metadata headers must never contain secrets.
- `metadata_only` leaves the gateway logging switch alone and sends `cf-aig-collect-log-payload: false`; `off` disables logs and `full` explicitly enables payloads. AuraGo owns retries and always sends `cf-aig-max-attempts: 1`.
- `cloudflare-rest` is an explicit provider with its own Vault API token, account and gateway IDs, unchanged canonical model ID and OpenAI chat wire format. It is independent of the native rewrite toggle. Do not infer its billing rates from another provider.
- Hugging Face chat uses an operator-created custom provider with upstream `https://router.huggingface.co` and path `/custom-{slug}/v1/chat/completions`; never substitute the legacy inference API. Preserve custom/local endpoints.
- Status accepts `provider_id` and remains passive. Metadata tests share runtime routing/auth/privacy; unsupported metadata tests are skipped. Only explicit `probe_kind: generation` may generate, once, through budget accounting with 32 output tokens and a twenty-second deadline. Never return provider error bodies. Cancel on request abort and server shutdown.
- Validate gateway/provider settings and complete metadata before any Vault write. Preserve all gateway IDs through YAML, provider API, AgoDesk, helper, fallback and task clients. See `documentation/ai-gateway.md`.
- Gateway hot reload includes the complete gateway configuration and updates cached main/helper/Guardian clients. Game Maker provider switches use the complete provider projection before job-model overrides. Verify `TestAIGateway*Reload`, `TestGameMakerCloudflareRESTPreservesSelectedGateway` and the co-agent/summary identity regressions.

### here.now Integration Contract
- here.now uses only the fixed `https://here.now` API origin, Vault key `here_now_api_key`, authenticated permanent Sites, and explicit personal or workspace account resolution. Never fall back to anonymous publishing or claim flows.
- Homepage publishing snapshots a relative workspace directory into a private temporary tree before the first provider request. Reject traversal, symlinks/reparse points, special files, credential material, and more than 1,000 publishable files; upload only the snapshot and always remove it afterward.
- Retry reads, presigned upload PUTs, and idempotent finalize calls only. Never automatically replay create, update, duplicate, restore, metadata, access, refresh, or deletion mutations; ambiguous outcomes must be structured as `here_now_outcome_unknown` with `retry_safe`.
- API, upload, and Site verification traffic uses strict public DNS-pinned transports that ignore the loopback SSRF escape hatch. Upload redirects are blocked; Site verification validates and repins every bounded redirect hop before accepting a final `2xx`, `401`, or `403`.
- Access mutations require a complete current provider policy and preserve omitted allowlist fields. Site-password Vault keys bind canonical account ID plus slug, never use slug-only fallback, and are deleted only after verified password removal or successful Site deletion. The generic Homepage ledger records here.now only after finalize and URL verification.

### Netlify Integration Contract
- `/api/netlify/status` and `/api/netlify/test-connection` are administrator-only through the shared `/api/` bearer catch-all. Both read the Vault token, pass it through `netlifyServerConfig` (which registers it with `security.RegisterSensitive`) and use saved settings only.
- `status` answers the account plus `site_count` and `site_count_truncated`; a failed site listing never replaces the account answer.
- Provider bodies never reach the browser verbatim: `tools.NetlifyTestConnection` and `tools.NetlifyGetAccount` already answer bounded JSON, and the handlers re-encode the parsed object.
- Verify `go test ./internal/server -run Netlify -count=1` (`TestNetlifyStatusReportsDisabledIntegration`, `TestNetlifyHandlersRejectWrongMethods`) plus the Config UI rows for `cfg/netlify.js`.

### Vercel Integration Contract
- `/api/vercel/status` and `/api/vercel/test-connection` are administrator-only through the shared `/api/` bearer catch-all. Both read the Vault token and pass it through `vercelServerConfig` (which registers it with `security.RegisterSensitive`) and use saved settings only. The Vercel token follows the Netlify contract: a provider token read from the Vault is registered for scrubbing before the first network call.
- Provider bodies never reach the browser verbatim: the handlers re-encode the parsed object and never echo the token.
- Verify `go test ./internal/server -run Vercel -count=1` plus the Config UI rows for `cfg/vercel.js`.

### GitHub Integration Contract
- `github.allowed_repos` is a strict allowlist; prefer `owner/repo` entries. Legacy bare repo names only match the configured `github.owner`.
- An empty `github.allowed_repos` list permits only repositories recorded in the protected `directories.data_dir/github_trust.json` ledger after successful creation. Grants bind API base and canonical repository identity. Workspace `agent_created` markers never grant access; existing markers expose `trust_migration_required` until an administrator explicitly selects the repository in the allowlist.
- `github.allow_delete` defaults off and is required in addition to repository access and disabled read-only mode. Revoke the protected grant before deletion. Preserve the last valid ledger with a synced atomic replacement and private backup; do not automatically import legacy markers.
- Manual `track_project` entries are local inventory only and must never grant remote repository access.
- Config allowlist edits merge the visible page into entries that were not listed. Matching is case-insensitive. A bare name selects only the configured owner's repository.
- Dashboard and `/api/github/repos` list through the paged `GitHubListRepos` helper with no slice limit.

### Homepage Managed Website Ledger
- Quick tunnels use active registry identities and a private static snapshot through the shared Cloudflare publication boundary. The config UI selects `cloudflare_tunnel.quick_project_dir` from registered projects. Explicit tool/API ports and the former in-container arbitrary-port entry point fail closed. Disable/read-only, workspace/registry changes and project selection changes revoke active quick publication; shutdown cannot be blocked by agent read-only grants. Named/token tunnels remain separate.
- Cloudflare start/stop/restart routes require admin/CSRF and use saved settings plus read-only gates. Both config-save paths validate ports and stop revoked connectors before publishing the new Docker permissions; refuse saves on uncertain termination. Startup reconciles verified Docker containers even without auto-start. Dashboard and both status endpoints use the shared observed state, including unknown engine state and TLS warnings. Verify Cloudflare config/API tests and the enabled Cloudflare browser tests.
- Managed homepage/web projects use `data/homepage_registry.db` as the system of record for project identity, local file state, structured events, revision links, deployment targets, deployment history, remote observations, and drift status.
- Homepage project identity is the `project_dir` relative to `homepage.workspace_path`; avoid storing absolute workspace paths as the canonical project key.
- Mutating homepage operations must keep the ledger current by recording structured events and, when files change, revisions plus file-state snapshots. Remote deploys must be linked to provider IDs/URLs and build artifact hashes when available.
- Server APIs under `/api/homepage/sites` expose the managed-site read model, including detail `deploy_targets` and `remote_observations`, plus the reconciliation path. Keep these APIs additive and compatible with existing `/api/homepage/history`.

### Configuration UI Integration Test Contract

- Page translation payloads are selected by key prefix in `uiI18NSections`, not
  by the locale directory. Config needs `personality_dynamics` and
  `personality_legacy`, Dashboard needs `personality_dynamics`, and Chat needs
  `personality_legacy`. Verify `TestPersonalityTemplateDataIncludesTranslations`
  across all sixteen locales when changing these shared components.
- RuView `GET /api/ruview/board-setup` and its Desktop integration alias are
  administrator-only, `no-store` reads of saved receiver/UDP defaults and LAN
  suggestions. They never resolve DNS, probe, start a runtime or publish UDP.
  Container interfaces cannot stand in for a Docker host; remote/external
  placement cannot enumerate AuraGo's local interfaces as receiver candidates.
  Verify `TestRuViewBoardSetupIsAdminOnlyAndPassive` and `internal/ruview/TestBoardSetup*`.
- RuView firmware/capability catalogs and their Desktop aliases must serialize every
  flash offset, including bootloader address `0`. Browser profile validation requires
  explicit addresses. Verify `TestRuViewFirmwareResponsesPreserveFlashOffsets` and
  `ui/TestRuViewSerializedFirmwareCatalogBrowser` against the Go-encoded catalog.
- RuView `nodes` and its Desktop alias remain admin-only. Reads combine live nodes
  with receiver-scoped saved IDs, including offline boards. Writes accept only a
  node ID after browser readback and retain CSRF/enabled/read-only gates. Never
  store WLAN credentials or turn retained IDs into fresh measurements. Managed
  background health checks collect IDs without opening the app. Keep proxy and
  agent reads live-only. Verify `TestKnownNodes*`, `TestRuViewKnownNodes*`, the
  background lifecycle tests and both RuView browser suites.
  Passive status, UDP scope and this-process recording acknowledgement live in
  `internal/ruview/AGENTS.md`. Verify `TestStatusUDPFieldsStayPassive` and
  `TestRecordingAcknowledgementFollowsForwardedResults`.
- Startup binds RuView edge hints to planner fingerprints `ruview:sensing_stale`, `ruview:storage_limit`, and `ruview:stop_unconfirmed` with kind `runtime_failure`. RuView does not record or notify. Verify `TestRuViewOperationalHintRecordsAndResolvesOnFreshProbe`.
- RuView UI responses, on both `/api/ruview/ui/` and the Desktop alias, replace the outer page policy with `connect-src 'none'`. The outer `securityHeadersMiddleware` policy stays unchanged. GET and HEAD of those pinned files skip the session and the desktop admin gate: the opaque frame cannot send the cookie, and the files contain no session, measurement, or credential. Their `Access-Control-Allow-Origin` is `*` with no credentials. Every other RuView route, including proxy, status, and capabilities, stays admin-only. Public `/fonts/` responses also allow `*` so the frame can load its fonts. Verify the HTML branch of `TestRuViewRoutesRequireAdminAndWriteOrigin`, `TestRuViewUIAssetsSkipTheSession`, and `TestExternalAssetsVersionAndRange`.
- `/api/treg/` is admin-only. Connection tests read the saved Vault-backed organization and balance; they never execute a catalog endpoint or accept a token in the request body. Catalog, balance and local status routes share the client policy boundary.
- Schema-rendered Telegram, Discord, Rocket.Chat, Home Assistant, Proxmox, S3, Frigate, and Ansible sections expose read-only connection tests through the shared registry in `ui/js/config/`.
- Test actions are enabled only for saved configuration and available Vault-backed credentials. Their backend routes are POST-only and admin-protected; probes must not send messages, execute playbooks, mutate storage, or change remote state.
- The probes use the integrations' documented read-only authentication/status requests. Any new production HTTP client must be classified in `internal/audit.NetworkClientInventory`, and action text must be present in every `ui/lang/config/common/` locale.
- `POST /api/meshcentral/test` uses `requireAdmin`. It refuses a login token whose secret is empty before dialing. Power, TLS and vault rules live in `internal/meshcentral/AGENTS.md`.
- `/api/models/catalog` uses the bundled provider/model catalog for its list. For exact provider/model matches, its structured-output flag follows the `models.dev` registry instead of the catalog's API-family inference; unmatched models keep their catalog flag. Verify with `TestHandleModelCatalogStructuredOutputMatchesModelsDev`.

### EasyDrag flows
- Wiring (`flows_service.go`): `initFlows` recovers a panic (logged at Error with the stack; the store is closed again and `s.Flows` stays nil, so flows are off and the server runs on, `TestFF1InitFlowsSurvivesAPanic`). `initFlows` runs before `MissionManagerV2.StartContext` (it installs the `FlowHooks`, so flow triggers and the startup trigger find them) and `startFlows` after it (old runs marked interrupted, Date/Time timers armed, then one `Service.ReconcileMissions` on a goroutine). A `Service.Start` that fails is retried on a goroutine of its own (`retryFlowsStart`: 5 s doubling to 5 min, every failure logged at Warn with the attempt and the next delay, the recovery at Info, then the same reconciliation; it ends on success, on a cancelled context and on `ErrRunnerClosed`, and `shutdownFlows` ends and joins it first, `TestAudit23FlowsStartIsRetriedInTheBackground`), so a locked flows.db at boot no longer leaves the Date/Time timers, the retention and the interrupted-run marking off until the next restart. `shutdownFlows` runs once the HTTP API is drained and before MQTT, mail, MCP, the sandbox, Looper and the databases stop: `Service.Shutdown` within what is left of the shutdown context (5–15 s), then the store. The desktop capability `flows` and the API gate are `flowsAvailable`: the service exists and `flows.enabled` and `tools.missions.enabled` are on.
- Configuration: `initFlows` reads `flows.enabled` and the four limits at boot only, so a config save that changes them reports the restart reason "EasyDrag flows" (`flowsRuntimeConfigChanged`, effective values); `ai_provider` and `agent.*` are read live, and `flows.ai_provider` is a provider reference (`provider_references.go`). `flows.db` lives next to the Game Maker database (`config.FlowsDBPath`) and is listed in `config.SQLiteDatabasePaths` (backups, the agent's protected files).
- Bridge (`flows_bridge*.go`, `flowMissionBridge`): it never calls a Service method that takes a flow lock; `broadcastMissionState` reaches the Service only through the lock-free `NextTimer`. Only a started run (`RunFinishedInfo.Started`) reaches `MissionManagerV2.FlowRunFinishedAtDepth`, with the chain depth read from the run record (`tools.CompletionChainDepth`); a run that never started only broadcasts `flows_changed`, because that path runs inside `DeleteFlow`, `CancelMissionRuns` and `Shutdown` and must stay free of database, file and network work. Outputs and trigger data are scrubbed value by value and bounded before Mission Control sees them (the output text 16 KiB, the dependents' `outputs` 64 KiB, history trigger data 16 KiB).
- `broadcastMissionState` is debounced (`missionUpdateDebouncer`, `missionUpdateDebounce` 250 ms, leading and trailing edge), so a burst of flow run events sends at most two `mission_update` frames per window.
- Failures (`flows_bridge_notify.go`): every finished run that was not cancelled records or resolves the planner issue `flow|<flow id>`, and every recorded failure fires `planner_operational_issue` triggers, as agent missions do. A failure notifies per the flow's `notify_on_error` (`desktop`, the default, `push`, `telegram` or `off`) under the flood rule of `flowFailureNotifier`: once when the flow starts failing, then at most once an hour (`flowFailureNotifyInterval`) while it keeps failing; a success resets it, a cancelled run neither notifies nor resets, and the state is in memory only. Push and Telegram sends run on goroutines with at most 4 in flight per channel (`flowNotifyMaxInFlight`); beyond that a notification is dropped with a Warn and leaves the failing state unchanged. The message carries the flow name (80 runes) and the error (300 runes).
- Hooks (`flowMissionHooks`): `StartFlowRun` first asks `automaticFlowRunsAllowed` (`flowsAvailable`, else `tools.ErrFlowsUnavailable`; then `tools.missions.readonly`, else `tools.ErrFlowRunsPaused`; both from the current config snapshot, so switching flows off or missions read-only stops cron, event and dependent-mission triggers at once, not at the next restart; Mission Control treats both sentinels as a deliberate refusal: a note on the flow mission, one Warn per change then Debug, and a planner admission is `canceled`, not `unknown`), then calls the lock-free `Service.TriggerFromMission`. `flowMissionBridge` implements the optional `flows.RunGate` with the same check, which makes a due Date/Time timer start no run (the occurrence is consumed, Info in the log). A cancel is not refused (`TestAudit23AutomaticRunsFollowReadOnlyAndTheFlowsSwitch`). `FlowMissionDeleted` and `FlowEnabledChanged` take the flow's lock through `DeleteFlowForMission`/`MissionEnabledChanged`, bounded by `flowHookTimeout` (2 min), and the manager calls them only through its tracked `runAsync` work (Stop waits for them; after Stop they are not called); a gone flow is logged at Debug, anything else at Warn. Both read the flow id first with the lock-free `Store.GetFlowByMission` (`flowIDOf`; a delete removes it) and broadcast `flows_changed` with it and the reason `deleted`/`enabled`, so an open editor reacts (`TestFF1MissionHooksBroadcastTheFlowID`). After `shutdownFlows` the hooks only log (`TestC16HooksAfterShutdownOnlyLog`). `flowMissionHooks` also implements `tools.FlowRunReporter` (`StartFlowRunReported`): Mission Control's Run and the trigger API answer a start the flow's skip policy dropped as `tools.MissionTriggerSkippedError{Reason: "flow_busy"}` (`tools.FlowStartSkippedReason`); run id and status are logged at Debug. `flowStartError` wraps `flows.ErrQueueFull` (a full flow queue or `flows.ErrRunnerQueueFull`) with `tools.ErrFlowQueueFull`, a noted refusal like `ErrFlowsUnavailable` and `ErrFlowRunsPaused`.
- The dashboard's cron handlers (`dashboard_handlers_cronjobs.go`, `dashboard_handlers_overview.go`) list a flow's schedule jobs with `managed_by: easydrag` and answer edits, toggles and deletes of them, and adds under an id a flow mission claims (`tools.FlowOwnsCronJob`), with 409 "This schedule is managed by EasyDrag; edit the flow instead."
- `/api/desktop/flows/…` (`flows_handlers*.go`; this section describes the API): bearer tokens need `desktop:read` for GET and HEAD, `desktop:write` for `POST validate` and `desktop:admin` for every other request (`flowsRequiredScope`, applied by `validRouteBearer` and `handleFlows`), because test and live runs execute host tools and publish, enable and secrets write missions, cron jobs, webhooks and the vault (`TestFF1FlowsWritesNeedTheAdminScope`); session users are not affected. Same-origin session writes, `FLOWS_DISABLED` (503) and `FLOW_PERMISSION_DENIED` (403) gates. `FLOWS_DISABLED` means only that flows are switched off or not available at all (`flowsAvailable`: `flows.enabled`, missions off, no flow service because the store did not open; and Mission Control's flow cancel without a flow service); EasyDrag shows its "switched off" card with "Open settings" for this code alone. Every other reason why the API cannot serve a request is a 503 with its own code, which EasyDrag shows as an error with "Try again" (audit 2026-10-08, finding 1.2): `FLOW_MISSION_CONTROL_UNAVAILABLE`, `FLOW_RUNNER_STOPPED`, `FLOW_VAULT_UNAVAILABLE` and `FLOW_REQUEST_CANCELLED` (below). `handleFlows` splits the Desktop admission around these gates: authentication first, then the flows gates (a read-only desktop or missions answer `FLOW_PERMISSION_DENIED`; `POST runs/{run}/cancel` passes a read-only desktop, as Mission Control's cancel does, and passes the `flowsAvailable` gate while the flow service exists (so a run can be stopped after `flows.enabled` or the missions tool was switched off at runtime; it stays `FLOWS_DISABLED` when the service was never created), but not read-only missions, mirroring `CancelCheck`), then a cancelled write gets the 503 below, then `checkDesktopOperation` with `flowsDesktopOperation` (GET, HEAD and POST validate are reads, the run cancel is a stop), so every other write carries a Desktop grant that a readonly switch revokes (`TestFU*`). Errors are `{"error", "code"}` with stable codes and go through `s.flowsErrorFrom(w, r, err)`, which matches with `errors.Is`/`errors.As`, never by text: `FLOW_INVALID` (422, with `issues`), `FLOW_NOT_FOUND` and `FLOW_RUN_NOT_FOUND` (404), `FLOW_REVISION_CONFLICT`, `FLOW_NOT_PUBLISHED`, `FLOW_NO_TRIGGER`, `FLOW_EXISTS` and `FLOW_MISSION_AMBIGUOUS` (409), `FLOW_DISABLED` (409, `flows.ErrFlowDisabled`: `POST {id}/run` of a published flow that is switched off, as Mission Control refuses a disabled mission; test runs stay allowed; the same refusal is `FLOW_MISSION_MISSING` for `flows.ErrFlowMissionMissing` when the mission is gone and 503 `FLOW_MISSION_CONTROL_UNAVAILABLE` for `flows.ErrMissionControlUnavailable`), `FLOW_LOCKED` (409) for `tools.ErrMissionLocked`, `FLOW_MISSION_MISSING` (409) for `tools.ErrFlowMissionNotFound`, `FLOW_RUN_LIMIT` (429) for a full queue, `FLOW_RUNNER_STOPPED` (503) for a closed runner (`flows.ErrRunnerClosed`: the service is shutting down), `FLOW_TOO_LARGE` (413: a body over its limit, `flows.ErrDocumentTooLarge`, `flows.ErrTestDataTooLarge`), `FLOW_BAD_REQUEST` (400) for an unsupported schema or an unknown template, and a `NodeError`'s own code (400, `FLOW_NODE_FAILED` when empty). `FLOW_INTERNAL` (500) carries only a generic message; the cause (paths, SQL, vault text) is logged at Warn, scrubbed and cut to 300 runes, with the route and flow id. A request whose context was cancelled (the client went away, the server drain cancelled it, or revoking the Desktop grants did) gets 503 `FLOW_REQUEST_CANCELLED` "the request was cancelled" (Debug log), never an implicit 200: a client that still waits (drain, revocation) reads it, nothing was done, and 503 lets the editor's saver retry (net/http has no status for a request the server abandoned; a 4xx would make the saver give up). Typed-nil errors land in `FLOW_INTERNAL` without a panic.
- Partial publish: when `Service.Publish` returns a record and an error, the revision is live but Mission Control or the timers were not updated. The answer is 200 `{"flow", "issues", "partial": true, "code": "FLOW_PUBLISH_INCOMPLETE", "error": "Published, but not every part could be updated (Mission Control or timers). Publish again to finish."}`; the flow is broadcast as `published` and audited with status warning. `HasUnpublishedChanges` is false then, and publishing the same draft revision again finishes the update, so the editor keeps its Publish button on `partial`. Exception: when the flow's mission is gone from Mission Control (`tools.ErrFlowMissionNotFound`; a publish never recreates it) the code is `FLOW_MISSION_MISSING` with "Published, but the flow's Mission Control entry is missing. Export the flow, delete it and import it again."; other routes answer that sentinel with 409 `FLOW_MISSION_MISSING`.
- `flowsJSON` encodes before it writes the status (an encode failure is 500 `FLOW_INTERNAL`); `flowsCollectionRoutes` lists only the collection routes `handleFlows` dispatches and must follow it.
- Flow secrets are vault entries `easydrag_<name>` (name `[a-z0-9_]{1,40}`, value at most 4 KiB, else `FLOW_TOO_LARGE`; writes and deletes limited to 30 per minute per client IP, else 429 `FLOW_RATE_LIMITED`; without a vault every secrets route answers 503 `FLOW_VAULT_UNAVAILABLE`). The API lists names and never returns values; a write goes through `WriteUserSecretContext` (not agent-readable, bounded by the request). The value is not registered with the global output scrubber on write: `flowSecrets.ReadSecret` registers it, raw and trimmed, when a run reads it, before any node can output it, so writes do not grow the process-wide scrubber. `tools.IsPythonAccessibleSecret` blocks the `easydrag_` prefix, so the agent's vault tool, Python and skills can neither list, read, create nor delete them (`TestC17FlowSecretsStayOutOfTheAgent`). A delete answers `used_by` with the published flows whose live revision names the secret in a `secret_ref` parameter of an enabled node (literal names only); EasyDrag's secret field deletes through this route and shows `used_by` in a warning. Audit entries carry the name, never the value.
- Run data answers go through `flowsJSONScrubbed` and SSE events through `writeFlowSSE`; both scrub the decoded values (`flowScrubbedJSON`, `scrubFlowValue`), not the JSON text. Flow ids outside `[A-Za-z0-9_-]{1,64}` and unknown sub-paths are `FLOW_NOT_FOUND`; a new `{id}/…` route adds its segment count to `flowRouteSegments`. Run event JSON is scrubbed once per (run, seq) (`flowSSECache`, 1024 entries / 16 MiB, events over 1 MiB per stream) and shared by the streams. The flow store scrubs at write time too (`flowStoreScrubber`, installed by `Store.SetScrubber`), so stored run data is already scrubbed.
- Runs (`flows_handlers_runs.go`): `GET runs/{run}/events` sends `event: snapshot` (the stored `RunDetail`, read after subscribing), then each bus event after the start point as `id: <seq>` + `event: event`, then `event: end`; `: heartbeat` comments every 15 s. The start point is the larger of `?after=` and `Last-Event-ID` (EventSource reconnects with its original URL); values outside 0..2^20 are ignored. `end` means the run is over (run_finished delivered, a final snapshot, a run the bus does not know, or a channel closed before any event); `event: resync` `{"after": <seq>}` means the bus dropped a subscriber that fell 256 events behind while the run goes on, so the client reconnects with `?after=`. At most 16 streams per run, 8 per client address (`ClientIP`) and 64 in total (429 `FLOW_RUN_LIMIT`). Streams end on the shutdown drain (`trackHTTP` cancels the request) and keep `httpstream`'s renewed write deadline (never cleared). `POST runs/{run}/cancel` answers 202 (also again while the run winds down), 404 `FLOW_RUN_NOT_FOUND` for unknown runs and 409 `FLOW_RUN_FINISHED` for ended ones; only the first cancel of a live run is audited. Run ids outside `[A-Za-z0-9_-]{1,64}` are 404. EasyDrag's test dialog reads `GET {id}/publish-preview` before every test for the effects of the real params (`CollectEffects`), so that route stays free of side effects. Test data exists only for enabled triggers of the draft (400 for an id that is no node id, 409 `FLOW_NO_TRIGGER` otherwise). Verify with `go test ./internal/server -run 'TestFlowsAPI|TestC18'`. `POST {id}/test` takes an optional `base_revision` (409 `FLOW_REVISION_CONFLICT` when the draft moved on, `flows.ErrRevisionConflict`). `GET {id}/export` redacts credential headers (`flows.RedactCredentials`).
- Catalog (`flows_handlers_catalog.go`): `GET node-types` is cached per registry generation, configuration snapshot and language (`flowNodeTypesCache`, at most one answer per language) and revalidated against the Decisions availability stored in each answer, then answered with a weak `ETag` and `Cache-Control: private, no-cache`; a matching `If-None-Match` gets 304, and `HEAD` gets the headers of `GET` without the body. The gzip middleware compresses it on the main HTTP/HTTPS listeners (the loopback and tsnet listeners have no gzip). An in-place write to the live configuration (no snapshot swap) does not refresh it, as for `flowCatalogEnv`. The palette's `effects` and `risky` describe a node with default parameters (a generic node: its worst case). They are hints, not a safety guarantee: the publish preview (`CollectEffects` over the real parameters) is authoritative, and at run time every tool call passes the tool's own gates.
- Options (`node-types/{type}/options/{param}`, `flowOptionList`): every label and hint is cut to 120 runes, and each source offers only values its node can use. Webhooks: all of them, like Mission Control's picker, with a disabled one marked in its hint (`easydrag.option.webhook_disabled`). Notification channels: only those `tools.SendNotification` sends to (Discord only when enabled, not read-only and with a default channel). AI models: only providers that can answer an `ai.step` (`flowChatProvider`, the static checks of `flowProviderEntry`); the default names the `flows.ai_provider` model, else the main model. Missions: the hint is the execution type, `agent` for older agent missions without one. Home Assistant entities are sorted by id, cut at 2000 with `"truncated": true`, and reused per configuration snapshot (`flowHACache`: a list for 30 s, an error for 5 s); concurrent misses share one Home Assistant request (singleflight, detached from the first caller, 10 s timeout). `FLOW_OPTIONS_UNAVAILABLE` (502) carries the cause scrubbed and cut to 200 runes. `POST validate` changes nothing, so it also works while missions or the desktop are read-only (`flowsReadOnlySafe`; desktop permission and origin checks still apply); `mode` is `draft` (also when empty) or `publish`, anything else is 400. `validate` with `publish` cannot check rules that need the flow id (the self-trigger of a `trigger.mission_completed` on the flow's own mission); publish and publish-preview do, so "valid" is not "publishable". `templates/…` and `validate/…` are 404 `FLOW_NOT_FOUND`. Verify with `go test ./internal/server -run 'TestFlowsAPICatalog|TestFlowOptions|TestC19'`.
- Mission Control's run and trigger on a flow mission: the desktop app posts `/api/desktop/integrations/missions/v2/{id}/run` (and `/trigger`), which runs under a Desktop grant; `dispatchDesktopOwnedMission` routes a flow mission to `RunNow`/`TriggerMission` (a flow run) before the owner path, because `QueueOwnedMission` is for agent missions and refuses flows (`TestMergeIntegrations*`).
- Mission Control cancel on a flow mission (`handleMissionCancelV2`) cancels the flow's live runs through `Service.CancelMissionRuns`, called after `MissionManagerV2.Get` (a copy; the manager's lock is not held, because unstarted runs are reported to `FlowRunFinished` on this goroutine) with a detached 10 s context: 202 when a run was cancelled (queued and waiting live runs go with it). Its flow answers carry a code through `flowsError` (picked in the handler with `errors.Is`; not `flowsErrorFrom`): `FLOW_NO_ACTIVE_RUN` (409: the mission is not running, `tools.ErrMissionNotRunning` from `CancelCheck`, or no live run was left to cancel; Mission Control shows its EasyDrag hint for this code only), `FLOW_MISSION_AMBIGUOUS` (409), `FLOW_NOT_FOUND` (404), `FLOW_INTERNAL` (500) and `FLOWS_DISABLED` (503, no flow service). Agent missions keep the code-less `jsonError` answers. `CancelCheck` still requires a running mission, so live runs that are all still queued can only be cancelled through `POST /api/desktop/flows/runs/{run}/cancel`; EasyDrag's runs drawer and run view offer Stop on every run that has not ended (queued, waiting, running) and post that route.
- Audit: user actions go to the audit timeline (source `mission`) as `flow_create`, `flow_import`, `flow_publish`, `flow_enable`, `flow_disable`, `flow_delete`, `flow_secret_set`, `flow_secret_delete` and `flow_run_cancel`. A new type needs an option in the dashboard's audit type filter (`ui/dashboard.html`) and a `dashboard.audit_type_<type>` label in all 16 `ui/lang/dashboard` files (`TestC17AuditTypesAreRegisteredInTheDashboard`).
- Verify with `go test ./internal/server -run 'TestFlowsAPI|TestC17'`.

### CYD Creator

- Icon library routes use the normal CYD gates: `GET icons` needs read access,
  `POST projects/{id}/icons` is a write route requiring `If-Match` and a strict
  catalog selection. Load its manifest/files through `uiFiles` (verified
  external web resources), never from unverified asset roots or request URLs.
  `TestCYDCreatorIconAPI` covers lookup, import, version and selection refusal.

Domain, storage, events and build rules live in `internal/cydcreator/AGENTS.md`; this section owns wiring,
configuration, the HTTP API and the Docker adapter. The desktop client (`ui/js/desktop/apps/cyd-creator-*.js`)
depends on the API, SSE and error contracts below; its side is in `ui/js/desktop/apps/AGENTS.md` (CYD Creator).

- Wiring (`cyd_creator_service.go`): `initCYDCreator` runs after `initGameMaker` only when `cyd_creator.enabled` is
  on; projects live in the desktop workspace (`desktop.ConfigFromAuraConfig`), artefacts in
  `<directories.data_dir>/cyd_creator/`, the database at `sqlite.cyd_creator_path`. `cydCreatorStorageProblem` keeps the feature off (the server runs on)
  when the builder's pull record `cyd_builder_image.json` next to the database (`cydBuilderTrustFile`, passed as
  `TrustFile`) would lie inside the virtual desktop workspace or the agent's file root (the nearest
  `agent_workspace` ancestor of `directories.workspace_dir`) outside the protected data directory: an agent-written
  record would let a locally built image pass as AuraGo's pull. Paths are compared after `EvalSymlinks`, which does
  not resolve Windows junctions or mount points (administrator-made layouts only; `tools.IsProtectedSystemPath` has
  the same blind spot). A refusal or a failed `NewService` leaves a generic reason without paths in
  `Server.cydCreatorProblem`. `shutdownCYDCreator` runs in `Start`'s deferred cleanup after the HTTP drain:
  `Service.Close` (job runs, build worker, database), withdrawing the default service, then `cydDockerBuilder.Close`.
- Configuration (`internal/config/cyd_creator.go`): `config.Load` normalizes the section
  (`NormalizeCYDCreatorConfig`, a warning per change): zero means the default (except `max_repair_attempts`, where 0
  is valid: no repair), numbers are clamped (build 60–1800 s,
  cold 60–3600 s and at least build, memory 512–16384 MB, CPUs 0.5–16 with NaN/±Inf the default 2, idle stop
  1–1440 min, repairs 0–10, job 5–120 min, kept builds 1–50, heartbeat 30–600 s), an invalid `builder_image`
  (`[host[:port]/]path[:tag][@sha256:<hex>]`, lowercase path components, at most 255 bytes) becomes the default and
  `ota.public_url` loses trailing `/` and is blanked (warning without the value) when it breaks the device URL rule;
  a save refuses such a URL (`cydPublicURLProblem` mirrors `cydcreator.ValidateDeviceURL`, host charset
  `[A-Za-z0-9.-]` included, because `internal/config` cannot import `cydcreator`;
  `TestCYDCreatorPublicURLMatchesDeviceRule`). Config saves
  (`handleUpdateConfig`, `applyConfigPatch`) run `validateCYDCreatorSave`: `config.ValidateCYDCreatorConfig` on the
  values the save changes compared with `config.yaml` before it, so a hand-edited value that `Load` clamps never
  blocks saving other sections; cold ≥ warm is checked when either timeout changes. The config page sees loaded
  defaults through `injectCYDCreatorDefaults`; `cyd_creator.model` is a provider reference
  (`provider_references.go`).
- Restart and live changes (`cydCreatorRestartReason`): with a running service a change of builder image, memory,
  CPUs, idle stop or `docker.host` is "CYD Creator builder"; without one (switched on since startup, or a failed or
  refused start), every save that leaves `cyd_creator.enabled` on adds "CYD Creator service", unless the new
  configuration would be refused again (`cydCreatorStorageProblem`). Timeouts, retention,
  model and OTA apply live: `replaceConfigSnapshot` calls `Service.UpdateLimits(cydCreatorLimits(…))`, which
  normalizes a copy first.
- Capability `cyd_creator` (`cydCreatorCapability`): the API is available, `docker.enabled` and not
  `docker.read_only` (the builder creates, starts and execs). API gate (`cydCreatorAvailable`): service present and
  `cyd_creator.enabled`, else 503 `CYD_DISABLED`; switched on without a service it adds `details.problem` (storage
  refusal or failed start) and/or `details.restart_required`.
- Routes (`cyd_creator_routes.go`, handlers in `cyd_creator_handlers.go`, `cyd_creator_build_handlers.go`,
  `cyd_creator_job_handlers.go` and `cyd_creator_devices_handlers.go`): `/api/desktop/cyd-creator/` dispatches
  through `cydCreatorRoutes(s)` (the plan 2 table, plan 3's `cydCreatorJobRoutes` and plan 5's
  `cydCreatorDeviceDesktopRoutes`), `/api/cyd-creator/device/` through `cydCreatorDeviceRoutes(s)`
  (`cydCreatorDeviceEndpointRoutes`). That prefix is a session-auth bypass subtree (`authBypassSubtrees`; the
  password lockdown still blocks it), so `handleCYDCreatorDevice` answers 401 `CYD_UNAUTHORIZED` to every request
  without a device token before the route lookup, the feature gate and any body read: `cydDeviceTokenPresent`
  needs exactly one `Authorization: Bearer` credential matching `cydDeviceTokenPattern` (`ccdt_` + 64 lowercase
  hex); session cookies and API tokens of any scope never reach a device handler. Each handler then authenticates
  the token through the service (hash lookup, device binding, failed-authentication limit;
  `internal/cydcreator/AGENTS.md`, "Devices, OTA and transfer"). Desktop routes check in
  order: authentication with the route's scope, 405 (with `Allow`, HEAD wherever GET) or 404, `CYD_DISABLED`, a
  same-origin `Origin` for every session request other than GET/HEAD (403), then `Write` routes answer 403
  `CYD_READONLY` while the desktop is read-only and pass `checkDesktopOperation(desktopWrite)`, `Stop` routes (build
  and job cancel) pass `checkDesktopOperation(desktopStop)`, which read-only allows. Bearer tokens need
  `desktop:admin` for `Write` and `Stop` routes and `desktop:read` for the rest (including
  `POST builds/{id}/decode-backtrace`); `validRouteBearer` uses `cydCreatorBearerScope`. A new mutating route sets
  `Write` or `Stop` (`TestCYDCreatorEveryMutatingRouteNeedsTheWritePermission`,
  `TestCYDCreatorBearerScopeMatchesTheRouteTable`).
- Requests and answers: errors use the flows-style envelope `{"error","code","details"}` with the status table
  `cydCreatorHTTPStatus`; untyped errors and typed `CYD_INTERNAL` are 500 `CYD_INTERNAL` "internal error" without
  details (cause logged, scrubbed), a request the client or the drain cancelled is 503 `CYD_INTERNAL`. JSON bodies
  hold exactly one value without unknown fields or trailing data (400 with a generic text; the layout PUT 422
  `CYD_INVALID_LAYOUT`) and are capped before reading: 64 KiB (413), job start and confirm 192 KiB
  (`cydJobBodyLimit`: 8000 prompt characters and 16 KiB of attachments even as six-byte escapes), the layout
  `MaxLayoutBytes` + 64 KiB, an app file 256 KiB (413 `CYD_FILE_LIMIT`), the multipart asset upload 2 MiB + 64 KiB
  (413 `CYD_ASSET_TOO_LARGE`). `If-Match` loses quotes and a `W/` prefix and is lower-cased; the layout PUT needs the
  version (≥ 1), file PUT and DELETE the SHA-256 or `*`. Assets and artefacts go through `cydServeReader`
  (`http.ServeContent` with the Content-Type set first, Range/HEAD, the file always closed); every file answer carries
  `X-Content-Type-Options: nosniff`, artefacts are attachments with `X-CYD-SHA256`. `GET capabilities` overlays the
  service answer: `enabled` from the config, `readonly` when the desktop is read-only or a bearer token lacks
  `desktop:admin`.
- SSE `GET projects/{id}/events`: subscribes, then sends the backlog after max(`?after=`, `Last-Event-ID`) in pages
  of 500 and drops live events with a seq it already sent (ephemeral events, seq 0, have no `id:`), then
  `event: stream_ready` without id, live events and `: ping` every 15 s. `stream_ready` carries `{"after":N}`, N the
  seq the live loop continues after (the last backlog event, else the resume point), plus `"reload":true` when
  events after the resume point were pruned or the resume point lies above the project's newest event (forged, or a
  restored or reset database; the stream continues from the newest, N); the client moves its cursor to N. `armCYDSSEWrite` sets a 30 s write
  deadline before each frame; on the API listeners `httpstream.WithWriteTimeout` renews it to its own budget
  (`agentHTTPWriteTimeout`, 40 min, on the main listener) on every write and flush, so a stuck client times out
  there; the deadline is never cleared. A failed write or flush, a closed subscription (subscriber dropped, project
  deleted, service closed) or the client leaving ends the stream, and the client resumes with `Last-Event-ID`. HEAD answers
  headers only.
- Agent jobs (plan 3; job rules in `internal/cydcreator/AGENTS.md`): `initCYDCreator` purges leftover
  `cyd-creator-<job>` short-term-memory sessions (`sweepCYDCreatorAgentSessions` via `memory.TransientSessionIDs`,
  keeping those of queued or running jobs) and installs the runner (`SetRunner`) before it publishes the default
  service. `cydCreatorAgentRunner` (`cyd_creator_runner.go`) runs each phase like Game Maker's runner:
  `agent.ExecuteAgentLoop` under `beginDesktopRun` in fresh session `cyd-creator-<job>:<run>` (permanently purged after each phase) with the
  prepared profile `cyd-creator/v1/{planning,layout,coding}` (repairing uses coding; `cyd_creator_prompt_profile.go`:
  shared ground rules, every workflow reads API reference topic overview first), `AllowedTools` exactly
  `agent.CYDCreatorPhaseToolSchemas(phase)`, tool budget `gameMakerToolCallLimit`, Game Maker's `StreamThroughput`,
  RunComplete hooks `PlanAccepted` (planning) and `JobFinished` (coding, repairing) and the job bound with
  `cydcreator.WithJobContext`. The phase request is the prompt plus a JSON job context inside `<external_data>`
  (plan, user changes, attachments, hooks, repair diagnostics, tool budget, seconds left rounded down to 30).
- `cyd_creator.model` is a provider ID ("" = main provider); an unknown ID or an empty model is `CYD_DISABLED`. The
  private checkpoint (`Service.SaveAgentContext`) keeps user, assistant and tool messages only, scrubbed, continues
  from the bounded request view and is capped at 2 MiB (`cydCreatorTrimConversation` drops the oldest rounds); an
  unreadable one is skipped. The run sets `PreserveReasoning`: reasoning is kept and replayed on the same provider
  and model, and dropped when either changed. A failed loop returns
  `cydCreatorAgentFailure`: its text is a generic category (the normal chat's texts, "could not be reached", the
  quota text), the raw error is logged and stays in the chain; the service scrubs the text again
  (`ServiceConfig.Scrub` = `security.Scrub`). A tool-limit final response outside planning ends the round normally.
  The broker forwards nothing: chat text and progress come from the service (`NoteAgentReply` keeps the final
  answer as the fallback summary).
- Job routes (`cydCreatorJobRoutes`): `POST projects/{id}/jobs` (Write, 202 `{"job"}`), `GET projects/{id}/jobs`,
  `GET jobs/{id}`, `POST jobs/{id}/confirm` (Write, 200 `{"job"}`), `POST jobs/{id}/cancel` (Stop, 202 `{"job"}`,
  409 `CYD_JOB_STATE` once finished), `POST projects/{id}/draft/discard` (Write, 204 also without a draft),
  `GET projects/{id}/messages`, `GET projects/{id}/revisions`, `GET revisions/{id}/diff`,
  `POST revisions/{id}/restore` (Write, 200 `{"version","revision"}`). Every job POST body may be empty; start and
  confirm ≤ 192 KiB, the others ≤ 64 KiB. Verify `TestCYDCreatorJob*`, `TestCYDCreatorRunner*`,
  `TestCYDCreatorPromptProfiles` and `TestCYDCreatorStartupSweepPurgesLeftoverAgentSessions`.
- Builder (`cyd_creator_builder.go`, `cydDockerBuilder`): one container `aurago-cyd-builder`, created from the
  verified image ID (`sleep infinity`, init, user 1000:1000, `HOME=/tmp`, `NetworkMode none`, `CapDrop ALL`,
  `no-new-privileges`, read-only rootfs, tmpfs `/tmp` 1 GiB `rw,exec,nosuid,nodev`, memory = memory-swap and CPUs
  from the config, 512 pids, restart `no`, managed labels with a fingerprint of the settings and the image ID that
  triggers recreation), and the labelled named volumes `aurago-cyd-work` at `/work` (the archive API cannot write
  into tmpfs or a read-only root) and `aurago-cyd-cache` at `/cache`, shared by all projects. Trees go in with
  `PUT /archive` (uid/gid 1000), artefacts come back with `tools.DockerGetArchiveContext` (256 MiB cap),
  `cyd-build`/`cyd-addr2line` run as execs whose multiplexed output is streamed (`tools.DockerStreamContext`). All
  Engine calls pass the runtime Docker gates (`docker.enabled`; mutations need `docker.read_only` off). The image is
  only pulled (two-hour limit, progress from the JSON stream), never built; image trust and the pull record:
  `internal/cydcreator/AGENTS.md` (Builder image).
- Container output is untrusted: arguments are checked again (board env, cache key, build id, 1–64 addresses of at
  most 8 hex digits); `extractArtifacts` accepts only the exact `out/` paths (merged.bin and firmware.bin ≤ 16 MiB,
  firmware.elf ≤ 128 MiB, result.json ≤ 1 MiB, `fonts/ui_font_*.c` and `fonts/fonts.json` at the service's font cap,
  `build.log` read but not kept; at most 1024 entries and 256 MiB in total, sizes checked before reading) and
  refuses links, devices, unsafe names, duplicates and unknown entries instead of skipping them; the exec demuxer
  refuses frames above 1 MiB and stream bytes other than stdout/stderr, raw (TTY) output stops at 8 MiB, log lines
  are cut at 4096 bytes on a rune boundary. Users see generic errors (`cydPublicDockerError`: operation and HTTP
  status or "not reachable"; `cydPullReason` for pulls), because `ToolchainStatus.Error` reaches read-only sessions;
  Engine text, socket paths and addresses go to the server log only.
- Container lifecycle: the first use in a process replaces a running managed container (an orphan `cyd-build` of an
  earlier process could hold its lock); a container with the builder's name but without the CYD Creator labels is
  refused with `CYD_DOCKER_UNAVAILABLE` and never touched. A (re)started container clears `/work/ccb_*` and
  `/work/a2l_*`; each build removes its work directory. Cancel, timeout and a broken or hostile output stream stop
  and remove the container (the Engine cannot kill one exec) and the next build creates a fresh one; `Close` stops
  and removes it too (the volumes stay); the `idle_stop_minutes` timer only stops it. The agent docker tool, agent
  Compose and the image reservation protect the container, its volumes and the image repository
  (`internal/tools/AGENTS.md`); the administrator container API lists the owner `cyd-creator` (above).
- Default socket proxy (`tecnativa/docker-socket-proxy` v0.5.0 with the compose environment; rules read from the
  pinned image's `haproxy.cfg.template` on 2026-10-11; live-checked the same day: `TestCYDBuilderStubMechanicsLive`
  passes with `DOCKER_HOST` pointing at that proxy, only `POST /build` is refused): every
  non-GET request needs `POST=1`; container inspect, create, start, stop and remove, `PUT`/`GET /archive` and exec
  create are `/containers` paths (`CONTAINERS=1`), exec start and inspect need `EXEC=1`, image inspect and pull
  `IMAGES=1`, volume create `VOLUMES=1`, API negotiation `VERSION=1`; `BUILD=0` is fine. HAProxy's
  `timeout server 10m` ends an exec stream that stays silent for ten minutes (the build fails and the container is
  discarded).
- Devices and transfer (plan 5, `cyd_creator_devices_handlers.go`): desktop routes `GET projects/{id}/devices`,
  `POST projects/{id}/devices` (Write; 503 `CYD_OTA_DISABLED` while OTA is off, else 201
  `{device, config_blob, config_offset}` with the blob in base64), `PATCH devices/{id}` (Write,
  `{name?, target_build_id?}` → `{device}`), `DELETE devices/{id}` (Write, 204), `GET projects/{id}/export`
  (read; `application/zip` attachment with `Content-Length`, `no-store`, nosniff) and `POST import` (Write; multipart field `file`, the body capped at
  `MaxImportBytes` + 1 MiB (`cydImportBodySlop`), larger is 422 `CYD_IMPORT_INVALID` `too_large`; 201 `{project}`).
  Read-only viewers list devices and export. Status table: `CYD_FILE_LIMIT` 413, `CYD_IMPORT_INVALID` and
  `CYD_TEMPLATE_TOO_NEW` 422, `CYD_UNAUTHORIZED` 401, `CYD_RATE_LIMITED` 429, `CYD_OTA_DISABLED` 503,
  `CYD_DEVICE_NOT_FOUND` 404.
- Device endpoints (`cydCreatorDeviceEndpointRoutes`): `POST heartbeat` (`{interval_s, update}`),
  `GET firmware/{id}` and `POST ota-result` (204); JSON bodies are strict and at most 64 KiB. `cydDeviceRequest`
  passes the bearer token and `ClientIP(r, server.https.behind_proxy)` to the service, so boards behind a reverse
  proxy do not share one failed-authentication bucket. Firmware goes through `cydServeReader` with
  `Content-Length` (the reader is cut to the build record's size), `X-CYD-SHA256`, `Cache-Control: no-store,
  no-transform` and nosniff, and `gzipRequestEligible` keeps the whole subtree uncompressed: the firmware compares
  `Content-Length` with the offered size. `update.url` is a path, never absolute (`internal/cydcreator/AGENTS.md`).
- Device tokens (scope `cyd_creator_device`, `cydcreator.TokenScopeDevice`) are stored as SHA-256 in `cc_devices`,
  never in the API token store: `POST /api/tokens` refuses the scope (`validateTokenScopes`), API tokens never pass
  the device gate, and a device token on a desktop route is 403 (`validRouteBearer` rejects it). Verify
  `TestCYDCreatorDeviceTokensStayInTheirLane`, `TestCYDCreatorDeviceEndpointsBypassOnlyTheirSessionAuth` and
  `TestCYDCreatorDeviceRoutesRefuseMissingTokensBeforeTheBody`.
- Device URL and pin (`cyd_creator_devices_endpoint.go`): `cydCreatorDeviceBaseURL` is `cyd_creator.ota.public_url`
  (trailing `/` dropped) or derived: the HTTPS domain or `server.host` with the HTTPS port (443 omitted), a LAN IPv4
  (`firstLANIPv4`) for wildcard and loopback binds, else `http://` with `server.port`. A wildcard bind inside Docker
  derives the container address and an IPv6 address is no device URL: such setups need `public_url`.
  `cydCreatorDeviceEndpoint` (minting only) checks the URL with `cydcreator.ValidateDeviceURL` (`cydCheckDeviceURL`)
  and for https resolves the pin, the SHA-256 of the DER leaf certificate: AuraGo's own certificate file for a
  derived URL (self-signed, custom `cert_file`, or the autocert cache entry named after the domain), a TLS probe
  (`cydTLSProbe`, 5 s, verifies nothing on purpose) when that file cannot be read (Let's Encrypt has not issued
  yet) and always for a configured URL (a reverse proxy may present its own certificate). Failures are
  `CYD_BAD_REQUEST` with `details.reason` `device_url` or `tls_pin` (with `url`) and `details.hint`
  `cyd_creator.ota.public_url`, which `cydError` passes to the flash dialog; never `CYD_INTERNAL`. A renewed leaf
  certificate (Let's Encrypt about every 60 days) breaks the pin, and pinned boards need a USB re-flash (v1
  limitation).
- `GET capabilities` replaces `ota.public_url` with the resolved URL and sets `ota.insecure` for `http://`
  (`cydCreatorOTACapability`) only while OTA is on and the URL passes the device rule. Capabilities never read a
  certificate or dial out; only minting resolves the pin.
- Verify `go test ./internal/server -run '^(TestCYD|TestInitCYDCreator|TestValidateCYDCreatorSave|TestDemuxDocker|TestBuildTar|TestExtractArtifacts|TestCreateTokenValidatesScopes|TestAuthBypass|TestGzip)'`;
  the live adapter checks are opt-in (`internal/cydcreator/AGENTS.md`, Verification).

### Video Studio Contract
- Video Studio storage is rooted at `Documents/Video Studio/<project-id>` in the Virtual Desktop workspace; project JSON uses strong `ETag`/`If-Match` writes, and client asset metadata is always reconstructed from the private import manifest. Media uploads stream through the rooted Desktop service and never accept arbitrary host paths.
- `/api/desktop/video-studio/` routes require Desktop read/write scopes plus both Video Studio and Desktop policy gates. Job IDs remain fetchable without project context. One per-server queue serializes FFmpeg probe, preview and render work; generation is optional, admin-only, budget-checked, and never retried automatically after an uncertain provider outcome.
- Render submission requires the saved project ETag and snapshots that immutable revision before queueing. Final MP4 publication is create-only and guarded by both Desktop and Video Studio revocation gates. Config root changes cancel active jobs; graceful shutdown drains/cancels the worker before closing Desktop storage.
- Per-project operations use bounded lock striping: a project ID always maps to the same exclusion lock, while misses and deleted projects do not retain per-ID locks. Stripe collisions may serialize otherwise independent projects.
- Generation checks project quota before contacting the provider and again before importing its result. Stable job errors distinguish `project_size_limit`, `asset_size_limit`, and local `generation_import_failed` from `generation_failed`; quota errors are shared by preview, render, and generation workers. A render that reaches its output cap (`videostudio.ErrOutputLimit`) reports `project_size_limit` when the remaining project quota was the smaller cap, otherwise `export_size_limit`.
- `external_status_unknown` is persisted when the first provider connection is ready (`httptrace` `GotConn`), before any request byte is written; failures before that (daily limit, local setup, DNS, refused connection) are plain `generation_failed`. The provider's file in `data/generated_videos` is kept on purpose: it is the Desktop "Videos" library entry in the media registry and the only copy of a paid result when the import fails.
- Once a job's publication gate committed its output, `cancelJob`, config changes and shutdown no longer flip it to cancelled; the worker records the outcome (project delete still cancels). Shutdown closes the manager first (`shutdownDesktopStorage`) and `getDesktopService` never reopens Desktop storage afterwards.
- HTTP codes: cancel of an ended or committed job is 409 `job_finished` with the `job`; enqueue/upload/delete refused by a configuration change is 409 `configuration_changed`; uploads answer 413 only for `asset_size_limit`/`project_size_limit`, 400 `media_upload_failed` for a broken client stream, 500 `media_store_failed` otherwise; a project delete runs before cancelling its jobs and fails with 500 `project_delete_failed` (404 only when missing). `GET /projects` reads the 200 most recently changed projects and adds `"truncated": true` beyond that.
- Generation image inputs are private project image assets normalized locally to bounded PNG bytes. Only the configured provider's supported image mode is exposed; external image URLs are never fetched automatically. Render staging skips muted, hidden, and zero-volume audio clips but retains visual inputs regardless of clip volume. Verify `TestVideoStudio*` and the browser export flow.
- Media removal is recorded before any file is deleted and finishes on a retry, at worker start or in the background sweep a media listing queues; that sweep and the poster backfill of older media run in one upkeep goroutine with a bounded queue beside the job queue (the backfill runs FFmpeg only while no job is queued or running) (`video_studio_media_upkeep.go`). The rules live in `internal/videostudio/AGENTS.md` (Media and projects).

### Rocket.Chat Runtime Contract
- One server-owned consumer processes history chronologically with bounded pagination, including messages sharing a timestamp. Accept ISO timestamps and the legacy date object.
- Every turn uses a current immutable config/client snapshot. Publishing changed Rocket.Chat credentials, channel, allowlist or enablement cancels the old generation immediately; drain outside config locks before starting its replacement. Egg mode and shutdown forbid restart.
- HTTP requests and agent turns inherit the runtime context. Shutdown waits for message processing before closing databases. Integration credentials never follow a foreign redirect.
- Verify `TestRocketChat` in `internal/rocketchat` and `internal/server`.

### Mission and Home Assistant Lifetimes
- Mission manager StartContext inherits the server lifetime; repeated Start is idempotent. Stop is terminal, refuses further work, and waits for the queue, invocation callbacks, timeout guards and completion work. Invocation callbacks must use the manager Context and must not detach their work.
- Mission HTTP handlers remain independent of a caller disconnect, but inherit server shutdown. Their registry cancels and drains every generation before database closure, including replaced runs. History completion finishes within the tracked invocation.
- Mission owner cancellation (including shutdown) records an interrupted run without creating, incrementing or resolving its operational issue. Explicit user cancellation retains its separate output; real deadlines and execution failures still record errors. Verify `TestMissionFailureCancellationPreservesOperationalIssues`.
- Mission completion rejects explicit blocked/failed status declarations even after tool activity; a saved failure report is not evidence that the requested work completed. Preserve completed reports and quoted historical failures. Verify mission completion assessment tests.
- Home Assistant polling owns one cancellable generation. Publish changed credentials/URL/enablement before draining outside config locks and starting a replacement. Shutdown drains polling before closing mission databases. Entity IDs use the shared path encoder and credentials cannot follow a foreign-origin redirect.
- Verify mission lifecycle and Home Assistant regression tests in tools and server; run Linux CGO race tests for concurrent cancellation.

### Uptime Kuma and EvoMap
- Uptime Kuma uses one server-owned cancellable generation, one notification consumer, at most sixteen pending prompts and a five-minute per-turn deadline. Overflow is logged. Monitor fields are bounded external data; use a dedicated session. Config publication cancels stale generations, and shutdown drains them before database closure.
- EvoMap registration is writable-only in the agent and API. The server callback owns network registration and serialized config/Vault publication under CfgSaveMu; never mutate captured config snapshots or save them from the agent. Persist only the node ID into current YAML, keep secrets in Vault, restore prior local state on save failure, and never retry uncertain registration automatically.
- Verify Uptime Kuma lifecycle, EvoMap registration/revocation and configuration preservation regressions.

### File indexer replacement
- `restartFileIndexer` replaces the indexer under `CfgSaveMu` while handlers run. Read it only through `s.fileIndexer()` into a local, once per use, and write it only through `setFileIndexer`; never re-read between the nil check and the call. Verify `TestRestartFileIndexerConcurrentWithReads` with `-race` (Linux CGO).

### Webhook publication
- Mission webhook registrations use stable mission keys, replacing the old callback and removing it on disable, delete, manager replacement or shutdown. A-B-A trigger changes must not accumulate callbacks.
- Incoming webhook handlers obtain current immutable config and Guardian services together under the server config lock. Disabled integrations reject new deliveries; scan policy changes take effect without restart.
- Webhook config load errors fail closed and preserve the source. Validate a detached candidate, persist atomically, then publish; failed create/update/delete and signature migration must preserve the last valid in-memory state. Return detached read values.
- Only an HMAC-SHA256 signature authenticates an incoming webhook without its bearer token. A sha1 webhook must also present its token (the signature is still verified); loading one logs a deprecation warning once per webhook ID. Verify `TestHandlerSHA1Signature*` and `TestHandlerSHA256SignedOnly*` in `internal/webhooks`.
- `manage_webhooks` uses the single live `*webhooks.Manager` published on the server. The tool does not open a second manager. A nil manager means the receiver is not running. Tool delete removes vault key `webhook_<id>_signature_secret` before the file and restores that secret when the file delete fails.
- Agent `manage_outgoing_webhooks` updates merge only fields that were sent. An empty method, URL or body, and a missing `headers` value, keep the stored values. An explicit `headers` object replaces headers, and `{}` clears secret headers. `PUT /api/outgoing-webhooks` stays a full replace. Outgoing names are unique case-insensitively; the next save rejects an existing duplicate. `call_webhook` returns `status: error` with `http_status_code` for HTTP status 400 and above, and lists IDs when several hooks share the name.
- Loopback agent delivery retries a transport failure twice, with a short pause, inside the existing goroutine and delivery slot. HTTP status 400 and above is not retried. The incoming webhook still answers before that delivery.
- Verify webhook transaction, callback roundtrip and live policy regression tests plus mission suites.

## Desktop invocation ownership

- Desktop-owned shared integration routes use server-selected handlers and keep
  original integration gates. Never trust a header or client origin assertion as
  Desktop ownership. Readonly has stable HTTP 403 code desktop_readonly; it also
  revokes background owner contexts and rejects late local publication.
- Stop/pause/cancel routes require the normal authenticated scope but bypass
  readonly write admission. Body-selected actions (Detective) are decoded and
  classified once; finish/resume/new execution remain writes. Turning Detective
  from allowed to denied revokes in-flight desktop runs on that transition.
  A later save while research is already denied does not cancel other desktop runs.
- Game Maker policy publication and agent phases, Detective reports, queued
  local missions, mission preparation and Virtual Computer tasks retain this
  ownership across HTTP completion. Server cancellation drains before stores close.
- Desktop HTTP/SDK file and archive contracts are in documentation/desktop-api.md.
- Binary `/api/desktop/download` responses carry a strong ETag over the exact
  delivered bytes, using the same version function as conditional file writes.
  Keep it on range/conditional downloads; Ziro's Shared adapter owns that version
  alongside its snapshot, never a later HEAD observation.
- Server Desktop events go out only through `broadcastDesktopEvent`, which sends
  each one over the Desktop WebSocket hub and SSE with one `id` (assigned when
  empty). The shell handles an id once; never give two events the same id or
  broadcast to only one transport (`TestBroadcastDesktopEvent*`).

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

- Insights and native graph: `documentation/learnings.md` and
  `internal/codegraph/AGENTS.md`. Resolve project identities and roots from live
  registrations. Central knowledge routes require admin; Desktop routes also
  require Desktop scopes/policy. Job indexes are capabilities for the current
  working copy only. Release registrations and advisory write targets on every
  completion/cancellation path and drain the graph service on shutdown.

## WebAuthn owner login

- `auth_webauthn*.go` owns UV-required WebAuthn registration/login, the reserved
  version-1 Vault registry and server-local two-minute ceremonies. Reuse the
  pinned library for verification; never implement signature/COSE/CBOR checks.
- `passkey` completes login with WebAuthn and retains the existing password/TOTP
  fallback. `second_factor` requires password plus WebAuthn, or password plus
  enrolled TOTP; no password-only fallback, including `/auth/login`, TUI and
  resource recovery. Unknown modes fail closed. Feature defaults off.
- Public exemptions are exact: `/auth/login`, `/auth/logout`, the tiny
  `/auth/webauthn/client.js`, and POST `/auth/webauthn/login/begin`/`finish`.
  Never exempt the `/auth/` prefix. The WebAuthn routes stay outside the
  missing-password allowlist. Methods, bounded JSON, same-origin Origin,
  configured RP/origins, shared verification budgets and cookie binding are
  enforced by the handlers. Public status never exposes credential metadata.
- Enrollment/policy/rename/removal require an owner browser session and current
  password plus active TOTP step-up. Bearer-only requests fail. Registration
  binds that owner's signed session nonce across activity renewal; a different
  login, logout or auth-policy change must still invalidate it. Protect the last second-factor method.
  Generic config/setup patches must not change policy or overwrite active
  factors; check under `CfgSaveMu` before generic write publication.
- Lock order is `CfgSaveMu` before ceremony mutex. Bind transactions to browser,
  operation, origin, auth fingerprint, config/Vault identity and registry hash.
  Consume atomically, re-read current registry, reject UV/counter anomalies,
  and persist before setting a session cookie. Registry writes invalidate
  concurrent pending work. Never log challenge/assertion/raw credential IDs.
  Administrative credential URLs use opaque digests.
  Every successful begin refreshes the same ten-minute browser-binding cookie;
  a fresh two-minute transaction cannot outlive it or rotate another transaction's binding.
- Read the registry fresh from Vault; preserve full library credential records,
  stable owner handle and RP across backups. Corrupt/unknown records never
  become empty accounts. Explicit RP reset requires disabled WebAuthn and
  step-up. Removal revokes future key use, not existing sessions.
- Recovery embeds only the shared client and generated 16-locale subset. Keep
  `scripts/build-webauthn-locales.mjs --check`, `scripts/webauthn-client.test.mjs`,
  `TestWebAuthn*`, auth bypass/lockdown/step-up and asset-recovery tests passing.
  `documentation/webauthn.md` owns operator guidance and device acceptance limits.

## Child DOX Index

None.


### Homepage ledger execution and observation

- Server/configuration Caddy lifecycle uses managed wrappers with a system reason. Draft/unverified attempts may have intent/outcome events; successful deployment records remain verification gated.
- Start reconciliation independently of startup enablement; each pass reads current config under CfgMu. Recover unfinished intents once before new work.
- Site detail includes provenance, pinned local locations and events. The sites/{id}/events endpoint provides bounded incremental reads without changing existing envelopes.
- Targets retain account/environment identity and active/inactive/unknown state. Closing temporary publication ends its target; development and Caddy lifecycles remain separate.
