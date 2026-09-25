# Server integrations

## Purpose

Server-owned HTTP and cross-component integration contracts.

## Ownership

`internal/server` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Newspaper integration
- `/api/desktop/newspaper/` is admin-scoped and same-origin for session writes. The server owns research, delivery and configured destinations; source pages cannot set recipients or invoke send tools.
- Research requires the enabled model, web scraper and network permission, plus Brave Search or profile-selected RSS feeds. RSS retrieval pins a strictly public IP at each redirect and bounds XML input; feed entries remain leads until the guarded scraper reads the original page. Consider candidates across selected sections before filling the issue. Persist fetched source evidence before model editing, and publish only validated immutable revisions. Expose bounded run progress and sanitized capabilities.
- Email uses a writable configured SMTP account or AgentMail with POST retries disabled. Telegram uses only the configured user ID. Unknown send outcomes remain uncertain in the ledger and must not be replayed automatically.
- Verify with `go test ./internal/server -run TestNewspaper` and the Newspaper browser test. Core state contracts live in `internal/newspaper/AGENTS.md`.

### Ingress Security and Browser Lab
- Forwarded host, scheme, and client IP count only when `server.https.behind_proxy` is enabled and the immediate peer matches `server.https.trusted_proxy_cidrs`; other forwarding headers are removed before auth and URL construction.
- An auth-disabled remote listener requires `auth.allow_unauthenticated_remote` before startup or config save. This exception never opens `/speech-lab/`.
- `/speech-lab/` requires an AuraGo session, same-origin writes and WebSocket Origin, and a configuration-owned private backend. Strip AuraGo credentials before forwarding. No separate Tailscale port 8766 listener.
- Desktop embed credentials travel only in `/desktop-ticket/<ticket>/...` paths, are stripped before access logging, and remain scoped to their exact desktop path or camera resource. Query tokens fail.
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
- Schema-rendered Telegram, Discord, Rocket.Chat, Home Assistant, Proxmox, S3, Frigate, and Ansible sections expose read-only connection tests through the shared registry in `ui/js/config/`.
- Test actions are enabled only for saved configuration and available Vault-backed credentials. Their backend routes are POST-only and admin-protected; probes must not send messages, execute playbooks, mutate storage, or change remote state.
- The probes use the integrations' documented read-only authentication/status requests. Any new production HTTP client must be classified in `internal/audit.NetworkClientInventory`, and action text must be present in every `ui/lang/config/common/` locale.
- `/api/models/catalog` uses the bundled provider/model catalog for its list. For exact provider/model matches, its structured-output flag follows the `models.dev` registry instead of the catalog's API-family inference; unmatched models keep their catalog flag. Verify with `TestHandleModelCatalogStructuredOutputMatchesModelsDev`.

## Verification

- Run `go test ./internal/server` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
