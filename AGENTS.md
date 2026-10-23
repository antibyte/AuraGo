# AuraGo — Agent Reference

English code/comments/docs; default application language: German (`Deutsch`). AuraGo is a portable, CGO-free Go home-lab agent (Linux/macOS/Windows, amd64/arm64), with embedded SQLite, shell/Python/filesystem/HTTP/cron tools and Docker, Proxmox, Home Assistant and SSH integrations. Optional runtimes/integrations have separate dependencies. The single backend binary embeds recovery/login only; the full UI is a verified, version-bound external resource set.

## DOX: read, edit, close out

- Each `AGENTS.md` binds its subtree. Before editing, identify target paths and read the root plus every parent/child guide along each route, including indexed children. Re-read this chain in the current session; memory is insufficient. Nearest guidance controls local details; children cannot weaken DOX. Work products, sources, instructions, records, assets and durable docs must remain understandable from that chain.
- Root owns global rules/preferences/workflows and areas without local contracts; children own domain details and Child DOX Indexes. Parents explain child scope and retained ownership; deeper guidance must be more specific/practical.
- Every meaningful change needs a DOX pass: update the nearest owner for changed purpose/scope/ownership, structure/contracts/workflows/rules, inputs/outputs/permissions/constraints/side effects/artifacts, user preferences, or guide creation/deletion/move/rename/index. Update affected parents/children/indexes; remove stale/contradictory text. Behavior-neutral edits may leave guides unchanged after review; report why.
- Create child guides at durable boundaries with distinct purpose/rules/responsibilities/workflow/materials/quality standards. Section order: Purpose → Ownership → Local Contracts → Work Guidance → Verification → Child DOX Index. Work Guidance reflects actual project/user standards; Verification reflects existing checks. Leave either empty until these exist.
- Keep docs concise, current, operational: stable contracts, direct bullets/explicit names; broad rules in parents, concrete details in children. Avoid duplication unless locally necessary; delete diary/history, obvious statements, misplaced detail and obsolete warnings. Record durable user behavior preferences here or in the relevant child.
- Closeout: re-check changed paths against the chain; update owning/affected guides and indexes; run relevant existing verification; report intentionally unchanged guides and reasons.

## Repository map

Go 1.27.2+; standard `net/http`; pure-Go `modernc.org/sqlite`; embedded `chromem-go`; vanilla JavaScript SPA; Python 3.10+ in a sandbox venv; Docker/Compose. Dependencies: `github.com/sashabaranov/go-openai` (compatible LLM client), `github.com/philippgille/chromem-go`, `github.com/go-telegram-bot-api/telegram-bot-api/v5`, `github.com/bwmarrin/discordgo`, `github.com/robfig/cron/v3`, `golang.org/x/crypto` (SSH/bcrypt/ACME/Let's Encrypt), `github.com/gofrs/flock`, `tailscale.com`, `github.com/aws/aws-sdk-go-v2` (S3).

- `cmd/`: entry points; `cmd/aurago`: startup (`main.go`, `platform_unix.go`, `platform_windows.go`); `cmd/remote`, `cmd/config-merger`, `cmd/assetpack`, `cmd/memory-repair`: remote execution, config merging, UI packaging, reviewed offline analysis duplicate merging.
- `internal/`: private Go production logic. Core: `agent` (loop/dispatch/co-agents), `budget` (token cost), `commands` (slash commands), `config` (YAML/defaults), `llm` (client/failover/retry/pricing), `logger` (structured logs), `media` (files), `memory` (STM/LTM/KG/personality), `prompts` (dynamic prompts), `security` (Vault/tokens/Guardian), `server` (HTTP/API), `services` (indexing/ingestion), `setup` (first run), `tools` (implementations).
- Integrations in `internal/`: `contacts` (address book), `discord`/`rocketchat` (bots), `fritzbox` (TR-064), `invasion` (egg/nest), `inventory` (SQLite SSH devices), `meshcentral` (control WebSocket/devices/wake/power/MeshAgent commands), `mqtt`, `remote` (SSH/protocol), `scraper`, `sqlconnections` (external SQL), `telegram` (text/voice/vision), `telnyx` (SMS/voice), `tsnetnode` (Tailscale), `webhooks` (incoming/outgoing); `sandbox` owns Linux Landlock.
- `agent_workspace/`: runtime workspace; `agent_workspace/skills`: bundled Python skills; `agent_workspace/tools`: generated tools/manifest; `agent_workspace/workdir`: sandbox/venv. `prompts/`: `prompts/identity.md`, `rules.md`, `personalities/`, `templates/`, `tools_manuals/` own identity/rules/profiles/templates/RAG-indexed manuals.
- `ui/`: external HTML/CSS/JS, 16 locales, UI tests and `embed.go` fixtures. `assets/`: bundled/static/sample assets. `knowledge/`: application knowledge. `mcps/`: connector/tool definitions. `licences/`: third-party records/texts.
- `.github/`: Actions/agents/prompts/automation metadata. `scripts/`, `tools/`: developer/runtime helpers. `ansible_api/`: sidecar API. `browser_automation_sidecar/`: browser sidecar sources/support. `plans/`, `openspec/`: planning/specifications/change management. `docs/`, `documentation/`: user/operator guides.
- `deploy/`: tracked deployment/release inputs and ignored generated artifacts. `config.yaml`: local config; `config_template.yaml`: full reference. `Dockerfile`, `docker-compose.yml`, `Dockerfile.ansible`: containers. `install.sh`, `update.sh`, `make_deploy.sh`, `make_release.bat`: install/update/release.
- External `../agotrain` (`antibyte/agotrain`): training datasets/curated contracts/Python environments/Needle3; workflow: `documentation/training.md`. Ordinary builds need no training checkout.
- Ignored/runtime `bin/` (binaries), `data/` (state), `reports/` (analysis), `node_modules/`, `.venv/`, `.worktrees/`, `terminals/` are not child DOX owners.

## Build, test and operate

### Builds

```bash
# Development: version-bound UI, then backend
go run ./cmd/assetpack -out deploy -stage assets/web
go build -ldflags="$(cat deploy/web-assets.ldflags)" -o aurago ./cmd/aurago
# All binaries (after asset packaging)
mkdir -p bin
go build -ldflags="$(cat deploy/web-assets.ldflags)" -o bin/aurago ./cmd/aurago
go build -o bin/aurago-remote ./cmd/remote
go build -o bin/config-merger ./cmd/config-merger
# Individual cross-build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w $(cat deploy/web-assets.ldflags)" -o aurago ./cmd/aurago
# Docker
docker build -t aurago:latest .
docker-compose up -d
docker-compose logs -f aurago
```

Plain `go build` without resource flags produces recovery only. Packaging assets cannot repair an unpinned binary: rebuild with the flags, verify `--check-assets`, then restart. Local startup: `./start.sh`; Windows: `start.bat` or PowerShell in `documentation/web-assets.md`.

Production releases must use `./make_deploy.sh` (Linux/macOS) or `make_release.bat` / `make_release.ps1` (Windows), not ad-hoc builds; these scripts do not upload to a test server. Side effects are listed below.

### Verification and editing

- With ignored scratch Go files use `./scripts/go-test-with-cache.ps1` (also accepts `./...`): production roots and both versioned training utilities, excluding scratch directories. Literal `go test ./...` is for clean checkouts. Temporary Go files belong in `disposable/_<task>/` (Go skips underscore directories); never add a module boundary above tracked `disposable/export_tools` or `disposable/import_training_traces`.
- Focused tests: `go test ./internal/config/...`, `go test ./internal/agent/...`, `go test ./internal/memory/...`, `go test -v ./internal/tools/...`. Coverage: `go test -cover ./internal/...`; race: `go test -race ./...`; benchmarks: `go test -bench=. ./internal/...`.
- Put `*_test.go` beside source; name tests `TestFunctionName`; prefer table-driven cases (e.g. `internal/config/config_test.go`, `internal/tools/shell_test.go`, `internal/memory/history_test.go`, `internal/agent/agent_test.go`). Cover critical/complex paths and new features with focused unit and tool/workflow integration tests.
- Wrap errors (`fmt.Errorf("context: %w", err)`); use structured `slog` (`log.Info("message", "key", value)`). Packages: short/lowercase/no underscores. Files: `snake_case.go`; exported types/constants/functions: `PascalCase`; unexported names/variables: `camelCase`. One responsibility per file; split oversized files.
- Windows/PowerShell: never undo a broken edit with `git checkout <file>` (discards others' work); inspect the diff and revert only the failed edit. `[System.Text.Encoding]::UTF8` adds a BOM; use `[System.Text.UTF8Encoding]::new($false)` or another BOM-free writer; inspect the first three bytes when relevant (BOM breaks Go `encoding/json`). Normalize mixed CRLF/LF to LF per `.gitattributes`.
- Before tests check encoding and `git diff --check`; JS: `node --check <file.js>`; JSON: `python -m json.tool <file.json>` or another parser. Windows isolated build/release checks use `scripts/invoke-clean-worktree.ps1` (`finally` removes worktree/prunes metadata), not ad-hoc `%TEMP%` worktrees.
- Commit completed changes locally with a descriptive message, only intended files. Before each commit inspect `git diff --cached`, run `git diff --cached --check` and scan contextually for secrets without exposing values. Temporary scripts/files go in `disposable/`; clean up temporary files/logs and never push them.

### Configuration

```yaml
providers:
  - id: main
    type: openrouter
    name: "Haupt-LLM"
    base_url: https://openrouter.ai/api/v1
    model: "<supported-model-id>"
llm:
  provider: main
```

Set provider keys through Setup Wizard/Config UI into the encrypted Vault (`provider_main_api_key` here), never `config.yaml` or agent-visible data. Environment: `AURAGO_MASTER_KEY` = startup Vault key (64 hex characters/32 bytes); `AURAGO_SERVER_HOST` = bind override (Docker `0.0.0.0`); `LLM_API_KEY` / alternative `OPENAI_API_KEY` = LLM key override; `TAILSCALE_API_KEY` = Tailscale; `ANSIBLE_API_TOKEN` = sidecar auth.

Debug: `./aurago -debug` or `agent.debug_mode: true` in `config.yaml`.

### Install and development reset

Docker is recommended: `docker-compose up -d`; `docker-compose -f docker-compose.yml up -d` selects the default file explicitly. Linux:

- Verified install: download `install.sh` and `SHA256SUMS` from `https://github.com/antibyte/AuraGo/releases/latest/download/`, then `sha256sum -c --ignore-missing SHA256SUMS && bash install.sh`.
- Main-branch quick install: `curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash`.
- Manual: `wget https://github.com/antibyte/AuraGo/releases/latest/download/aurago_linux` (arm64: `aurago_linux_arm64`), `chmod +x aurago_linux`, `./aurago_linux`. Systemd: `sudo ./install_service_linux.sh`.

Development reset deletes databases/vectors/venv and removes locks after stopping:

```bash
./kill_all.sh
rm -rf data/*.db data/vectordb/* agent_workspace/workdir/venv
rm -f data/aurago.lock data/maintenance.lock
```

Master-key generation examples (keep secrets out of Git):

```bash
export AURAGO_MASTER_KEY="$(openssl rand -hex 32)"
echo "AURAGO_MASTER_KEY=$AURAGO_MASTER_KEY" > .env
```

```powershell
$bytes = New-Object byte[] 32
(New-Object System.Security.Cryptography.RNGCryptoServiceProvider).GetBytes($bytes)
$AURAGO_MASTER_KEY = ($bytes | ForEach-Object { $_.ToString("x2") }) -join ""
```

## Security and permissions

- Secrets live in the AES-256-GCM Vault (`data/vault.bin`), never code/config/repository. Tools retrieve them at runtime; the agent normally has no direct access, though users may supply secrets for Vault storage. Never commit `.env`/Vault files, PII, master/API keys, tokens/passwords/credentials, sensitive logs, memory dumps or conversation history. Analysis/sensitive reports/logs belong in ignored `reports/`, never `documentation/` or commits. If a secret was committed, assume compromise, rotate immediately and coordinate history removal (e.g. BFG)/any force push with the team.
- Vault replacement preserves the original until a synced temporary file is renamed through `internal/fileutil`; verify `TestVaultAtomicReplaceHandlesWindowsReaders`.
- Call `security.RegisterSensitive(value)` before any network call, including SDK constructors; shared logging scrubs messages, bound attributes and nested errors to prevent log/LLM disclosure. Add new tool/integration Vault secrets to the Python-tool export denylist; keep them out of the agent environment.
- Config gates: `sudo_enabled` (sudo), `sudo_unrestricted` (writes beyond install dir; requires removing systemd `ProtectSystem=strict`), `allow_shell`, `allow_python`, `allow_filesystem_write`, `allow_network_requests` (HTTP), `allow_remote_shell` (SSH), `allow_self_update`, `allow_mcp`, `allow_web_scraper`. Nonessential tools/integrations need activation toggles; harmful capabilities default off and stay UI-disableable. Assess exposure/security; mutating/critical tools need read-only toggles and, where needed, separate `read`, `write`/create, `change`/update, `delete`/remove grants.
- Operator commands (`/sudopwd`, `/addssh`, `/restart`, `/personality`, `/debug`, `/voice`) require `commands.Context.AllowOperator`: Telegram private chats, Discord DMs, `rocketchat.allow_operator_commands`, web console, desktop chat, mission runs; never model-written follow-up loopback turns. Classify each new command in `operatorCommands` or its test.
- External content is untrusted: wrap in `<external_data>`; never let it steer behavior/tool calls. `IsolateExternalData` entity-escapes the whole body. Only Game Maker source tools and CYD Creator read tools (`cyd_creator_read_project`, `cyd_creator_read_file`, `cyd_creator_build`, `cyd_creator_api_reference`) use `IsolateSourceData` for copyable code; fully escape any closing-tag shape/boundary name in common encodings or Guardian-critical content. `toolResultPayloadForm`/`isolateToolPayload` preserve the form.
- All channels obey shell sandbox policy. Desktop Notes never override disabled isolation/unsafe fallback. Preserve tool gates/native Notes/file mutation protection; unisolated code can bypass the latter (state in shell hint/manual). Active Landlock rejects writable-path overlap with Notes; unavailable required isolation stays blocked. See `documentation/desktop-notes.md`.
- Telegram splits at its UTF-16 limit without splitting code points. AgentMail auto-retries GET/HEAD only; never replay uncertain mutations. Relay seeding finishes before forwarding; accepted notifications deduplicate independently of label updates; drain on shutdown. Web Push retains subscriptions on transient errors, counts 2xx delivery only and removes on 404/410 only.

## Development workflows

### Tools, integrations and training

- New tool: implement/register in `internal/tools/your_tool.go`, add `prompts/tools_manuals/your_tool.md`, update registry as needed and add `internal/tools/your_tool_test.go`. Tool/integration changes also update `prompts/tools_manuals/`, agent prompts, Web UI help and documentation.
- Reconcile `../agotrain/training/tool_tiers.json` and `operation_contracts.json` with effective strict schema snapshots; preserve curated entries. Regenerate in `antibyte/agotrain`; run `.github/workflows/training-dataset.yml` against `.github/agotrain-revision`. Follow `documentation/training.md`; sync validator tool count/schema-token limit with exporter. `--check` is read-only for committed packs. Catalog changes retain paired German/English direct coverage for every operation and selector-free tool.
- Keep curated arguments sparse. Exporter uses the original argument schema to add optional null placeholders on a copy of Strict wire calls, never inventing required values. Catalogs retain both schemas for independent Python validation/semantic evaluation. Required/excluded operation fields apply to semantic arguments, not synthetic placeholders; domain-null remains meaningful. Schema-projection-only changes preserve manifests and explicitly update their schema hash; never bootstrap again.
- New integration: package/client/service in `internal/your_integration/`, types in `internal/config/config_types.go`, defaults in `internal/config/config.go`, optional `internal/server/` handlers, genuine translations in all 16 `ui/lang/` locales, docs in `documentation/`.
- SQLite migrations run at startup; new DBs initialize to current schema. Schema changes require backward compatibility, existing-data handling, backup before migration and a staging migration test.

### Skill creation

- Python skills: deterministic execution, APIs, parsing/data/file transforms, Vault/Tool Bridge access, structured automation. Agent Skills (`SKILL.md`): reusable behavior, review/debug/checklist/domain workflows, references/templates, agentskills.io/Codex/Claude-style packages.
- Create/import Agent Skills only through Agent Skill Manager/API/UI, then verify, approve warnings if needed and enable; never write directly to `agent_workspace/agent_skills`. Helpers obey `tools.skill_manager.allowed_script_languages`: Python needs `agent.allow_python`; Bash/JavaScript need `agent.allow_shell`. `allowed-tools` frontmatter is review metadata, not a native permission boundary.
- Disk reconciliation/optional remote scans run after manager/Game Maker initialization in a shutdown-bound background task capped at five minutes; never block core HTTP readiness/previously verified Game Maker bundles. Exact binary packages verify locally during manager initialization; cancellation before verification cannot grant readiness. Cancelled remote scans must not persist verdicts; package-hash checks still reject changed skills before use.
- Keep `prompts/rules/skill_creation/rule.md`, `prompts/ctx_capability_creation.md`, `prompts/identity.md` consistent when creation behavior changes.

### UI and product

- Fit existing style: polished/native-feeling UX, clear jargon-free instructions, pending/success/failure/disabled feedback. Review changed flows as a user; add relevant Dashboard features/useful connection-test controls. Use dropdowns for defined choices; show defaults or explain empty selects the documented default. Keep menus usable; modals, never `alert()`; selectable LLMs use the provider system.
- Distinct themes: `fruity` = Apple-inspired WhiteSur icons/topbar/floating dock/soft chrome; `standard` = Papirus icons/clear taskbar/structured start menu/restrained dark surfaces.
- Translate changes into all 16 locales (`cs`, `da`, `de`, `el`, `en`, `es`, `fr`, `hi`, `it`, `ja`, `nl`, `no`, `pl`, `pt`, `sv`, `zh`); never fill locales with English. German: `Du`/real umlauts, never `Sie` or `ae`/`ue`/`oe` substitutes.
- `README.md`: English, playful/geeky, user-facing, original AuraGo gopher; compact casual copy about real features/integrations, source-aligned claims, no corporate slogans. `assets/readme/` depicts real features/structure/connections; verify labels/arrows, authentic screenshots, Markdown explanations, light/dark desktop/mobile rendering.
- Task LLM router defaults off: operator guide `documentation/llm-router.md`, runtime `internal/agent/AGENTS.md`. Align Newspaper implementation with product/editorial/delivery/acceptance plan `documentation/newspaper-plan.md`.
- TeeVee retains supplied wood/metal CRT skin across themes; `documentation/teevee-retro-ui-plan.md` owns source/render/validation. CRT filter/glass reflection toggle independently; native playback survives blocked textures/WebGL. One decoder; explicit source-scoped proxy reconnect. Hardware 1080p/60 and external live-stream acceptance stay separate from local fixtures.
- HA Switchboard (`ha-switchboard`): existing HA integration/Desktop shell, walnut cabinet/silver lever per selected `switch.*`; admin-only routes preserve HA service policy and both read-only gates. Validated `ha_switchboard.board` persists order/selection/labels; live reads confirm explicit on/off writes. Follow `documentation/ha-switchboard-plan.md` and owning UI/app contracts.

### Dependencies, catalogs and delivery

- Update Go/npm/vendor assets/Python locks/Rust TUI together: stable releases, documented security/upstream bounds. Exceptions/rebuild checks: `documentation/dependencies.md`; version `tools/aurago-tui/Cargo.lock`, test `--locked`.
- Registry: `go run scripts/generate_model_registry.go --write` from `https://models.dev/api.json` to `internal/llm/model_registry_data.go`, then `--check`. Catalog: `go run scripts/sync_ohmypi_catalog.go --write`, then `--check`; take `src/models.json` and compiled `src/compat/rules.json` from the same `@oh-my-pi/pi-catalog` npm tarball. LLMs only; preserve explicit `supportsTools`; mark model-only providers catalog-only; test catalog/registry/server.
- Treat `config.yaml` carefully; align install/update scripts and Dockerfiles with system/dependency changes. AuraGo manages its Docker containers; do not assume manual user management.
- Compose socket proxy defaults to `BUILD=0`. Code Studio/most sidecars pull with `IMAGES=1`, `POST=1`; Engine builds (Homepage first-use dev image, CommandCode fallback, local Code Studio runtimes, rate-limit Caddy image) need `BUILD=1` or preexisting host images. `aurago --print-homepage-dockerfile` supports host builds; see `documentation/docker_installation.md` section 4. Changing default to `BUILD=1` requires an explicit decision.
- `docker-publish.yml` publishes GHCR linux/amd64 + linux/arm64 on `v*` tags/manual dispatch (`image=all` / `image=gods-eye-view`), never `main` pushes alone. Pin workflow `uses:` to full SHA plus ` # vX.Y.Z`; default `permissions: contents: read`, job-specific writes only (`internal/audit` rejects floating refs). `.github/dependabot.yml` refreshes monthly except manual `dtolnay/rust-toolchain`; `govulncheck@v1.8.0` in `security-gates.yml` is manual too. Training Python/uv/GPU research stays in `antibyte/agotrain`; AuraGo owns Go exporter/importer/pinned checks.
- `./make_deploy.sh` cross-builds and may commit/push `main` by default (`--no-publish` suppresses); no tag/GitHub Release. Windows `make_release.bat`/`make_release.ps1` build, commit/push as needed, publish versioned GitHub Releases with binaries and retain the latest three. Builders pin `GOTOOLCHAIN=go1.27.2`; verify compiler before publishing even if system Go is newer.

## Shared runtime contracts

- `internal/agent/agent_loop.go`: receive → LLM native function calling → tool dispatch/execution → SSE response → recovery/retry.
- One HTTP server, SSE, REST `/api/`; verified external full UI, embedded recovery/login. Fresh installs use local self-signed HTTPS: native 443 or 8443, Docker 8443; upgrades preserve HTTP/TLS. See `internal/server/AGENTS.md`, `documentation/https.md`.
- AgoDesk extracts prose `/files/...` before signing; Markdown/JSON-escape/query/fragment delimiters never become filename bytes. Preserve escaped filenames/structured payload queries when rewriting; never repair contaminated signed URLs in the asset handler. Tests: `TestAgodeskMediaReferencesServeSignedAudio`, `TestAgodeskChatBrokerDeduplicatesDelimitedMediaPaths`.
- `internal/httpstream.WithWriteTimeout` wraps outer handlers for local/HTTPS/Tailscale listeners/proxies. Successful SSE/MJPEG renew existing finite write budgets per write/flush; ordinary/error responses keep absolute timeout. Write/flush failure cancels requests, never revives streams. Preserve WebSocket hijacking/response-controller access. Verify `go test ./internal/httpstream`, server `TestAgentHTTPServerKeepsSSEAliveThroughMiddleware`, Tailscale proxy tests.

## Child DOX Index / cross-component routing

**Read every matching owner below in addition to the normal DOX chain before changing a listed feature, including server/UI/config/assets/tests/workflows outside its subtree.** Feature contracts remain repository-wide; this combined ownership/routing index does not narrow scope. Child guides own deeper indexes.

| Feature / area | Required AGENTS.md | Scope |
| --- | --- | --- |
| Low-poly Game Maker pack | `assets/game-maker-low-poly/AGENTS.md` | Original 220-model Blender pack, animations, compact exports, playable acceptance scenes. |
| Game Maker presentation | `assets/game-maker-presentation/AGENTS.md` | Effects/audio sources, licensing, builds, runtime limits. |
| Game Maker worlds | `assets/game-maker-worlds/AGENTS.md` | Maritime/isometric Blender sources, catalog counts, exports, runtime limits. |
| AuraGolf | `assets/minigolf/kit/AGENTS.md` | Procedural Blender tracks/courses/props, exports, collisions. |
| Tiefsee screensaver | `assets/screensaver-abyss/AGENTS.md` | Creature generator, deterministic exports, asset checks. |
| System World assets | `assets/system-world/AGENTS.md` | Blender city authoring/original sources/reproducible compact exports. |
| Memory repair CLI | `cmd/memory-repair/AGENTS.md` | Offline analysis duplicate merging, previews/evidence/backups, resumable reference/vector cleanup. |
| ACE-Step | `internal/acestep/AGENTS.md` | Private local music lifecycle, pinned runtime/models, hardware qualification. |
| Agent/tool system | `internal/agent/AGENTS.md` | Runtime prompts/discovery/dispatch/context; Prompt and Runtime Drift Contract. |
| Bluetooth | `internal/bluetooth/AGENTS.md` | Native discovery/permissions/playback. |
| Bookshelf | `internal/bookshelf/AGENTS.md` | Library/sources, reader isolation, conversion. |
| CYD Creator | `internal/cydcreator/AGENTS.md`, `internal/server/AGENTS.md`, `internal/agent/AGENTS.md` | Board/device profiles/themes/widgets/layout ops/validation/partitions/config blob; `cyd_template` runtime, builder image/deterministic generator, SQLite/storage, diagnostics/events/service, queue/jobs/revisions/tools/HTTP API. |
| Decisions service | `internal/decisions/AGENTS.md` | Optional shared service, verified artifacts/device placement/CPU permission/bounded scoring. |
| Decisions runtime | `decisions_runtime/AGENTS.md` | Pinned llama.cpp/mtmd scorer, private protocol/offload evidence/native-Docker packaging, HTTP `runtime_busy`. |
| Desktop files/archives | `internal/desktop/AGENTS.md` | Rooted operations; owns pets-assets child index. |
| God's Eye View Store | `internal/desktopstore/AGENTS.md` | Store app config/runtime/assets/publication. |
| Detective | `internal/detective/AGENTS.md` | Isolated Desktop research cases/evidence/budgets/revisions/exports. |
| Docker Engine | `internal/dockerutil/AGENTS.md` | Transport/API negotiation, mutation retry, streaming cancellation. |
| File utilities | `internal/fileutil/AGENTS.md` | Shared file replacement/free-space probe. |
| EasyDrag | `internal/flows/AGENTS.md` | Flows and Decisions media authorization. |
| Fritz!Box | `internal/fritzbox/AGENTS.md` | TR-064 integration/Desktop widget. |
| Game Maker | `internal/gamemaker/AGENTS.md` | Studio/planning/runtime feedback/progression/lifecycle/tool validation/sprite library/exports; owns asset-pack child index. |
| HTTP streams | `internal/httpstream/AGENTS.md` | Finite write budgets/HTTP writer compatibility. |
| Infra | `internal/infra/AGENTS.md` | Homelab atlas/source identity/layout/durable action outcomes. |
| Invasion | `internal/invasion/AGENTS.md` | Egg/nest protocol/auth/connection generations/key rotation/private deployment. |
| LLM transport | `internal/llm/AGENTS.md` | Trust/URL logging/failover state. |
| Managed local models | `internal/localllm/AGENTS.md` | Lifecycle/routing/attestation/qualification. |
| Local Wikipedia Contract | `internal/localwiki/AGENTS.md` | Offline editions/download/verify/swap/search/tools/content; readers `internal/zim`, `internal/zim/xapian`. |
| Memory | `internal/memory/AGENTS.md` | Retrieval/hygiene/indexing/maintenance. |
| MeshCentral | `internal/meshcentral/AGENTS.md` | Control WebSocket/power gate/command limits/TLS CA/admin connection test. |
| MeshCore | `internal/meshcore/AGENTS.md` | USB/BLE radio/trust/messaging/agent replies. |
| MQTT | `internal/mqtt/AGENTS.md` | Broker config/subscriptions/relays/mission dispatch. |
| Network shares | `internal/networkshares/AGENTS.md` | SMB/NFS capability/ownership/mutation policy. |
| Newspaper | `internal/newspaper/AGENTS.md` | Daily publication/source validation/scheduler/delivery receipts. |
| Desktop Office | `internal/office/AGENTS.md` | Workbook/document preservation/editing/assist. |
| ONNX Runtime | `internal/onnxrt/AGENTS.md` | Purego binding/verified pinned asset cache/native workers shared by embeddings/person detection. |
| Personal Radio | `internal/personalradio/AGENTS.md` | Stations/durable audio library/rotation/news/provider quotas/Desktop playback. |
| Person detection | `internal/persondetect/AGENTS.md` | D-FINE pinned models/WebGPU patcher/`--person-detect-worker` protocol/backend selection/supervision. |
| Pixel AI matting | `internal/pixelmatte/AGENTS.md` | Pinned ISNet/BiRefNet/pre-postprocessing/`--pixel-matte-worker`/`/api/pixel/cutout*` manager. |
| Operational issues | `internal/planner/AGENTS.md` | Lifecycle/notifications/background retries. |
| Security proxy | `internal/proxy/AGENTS.md` | Managed Caddy/Vault Basic Auth/rate-limit image/container placement/reload. |
| Remote | `internal/remote/AGENTS.md` | Enrollment/socket ownership/connection/command-result lifecycle. |
| Retro-Net Terminal | `internal/retronet/AGENTS.md` | Catalog/directory, guarded Telnet/SSH dialing/negotiation/charsets/session limits; server/Desktop setting/Terminal app. |
| RTL-SDR | `internal/rtlsdr/AGENTS.md` | Optional receive-only runtime/schedules/leases/recordings/ASR. |
| RuView | `internal/ruview/AGENTS.md` | Pinned sensing runtime/passive status/UDP readiness/this-process recording acknowledgement. |
| Default speech | `internal/phonon/AGENTS.md`, `internal/sanotts/AGENTS.md` | Bundled Phonon CPU runtime/catalog/downloads and the sanoTTS local fallback. |
| Security | `internal/security/AGENTS.md` | Primitives/Vault-token persistence/import grants/public URL trust/Guardian caching/Cast tickets. |
| Serial ports | `internal/serialutil/AGENTS.md` | Shared enumeration/canonical reservations/cancellation-safe leases. |
| HTTP/integrations | `internal/server/AGENTS.md` | HTTP/cross-component contracts: System World Tower Voice, 3D Printer, go2rtc, AI Gateway, here.now, GitHub, Homepage Managed Website Ledger, Configuration UI Integration Test Contract. |
| Workspace search | `internal/services/AGENTS.md` | Background services/search. |
| Native SIP | `internal/sipphone/AGENTS.md` | Telephone registration/calls/media/agent policy. |
| Speech Lab | `internal/speechlab/AGENTS.md` | Active ASR/TTS snapshots/speech-driven chat routing. |
| External SQL | `internal/sqlconnections/AGENTS.md` | Permissions/managed SQLite imports. |
| Telnyx | `internal/telnyx/AGENTS.md` | Call/SMS transport/bounded provider-confirmed reconciliation. |
| Desktop Tresor | `internal/tresor/AGENTS.md` | Browser-encrypted storage/revisions/backups/agent file protection. |
| Agent tools | `internal/tools/AGENTS.md` | Filesystem Jail/Docker Inspect/Managed Space Agent/treg catalog gateway safety. |
| Updates | `internal/upkeep/AGENTS.md` | Transactions/artifact retention/maintenance CLI/cleanup safeguards. |
| Virtual Computers | `internal/virtualcomputers/AGENTS.md` | Workspace leases/managed Garage storage lifecycle. |
| Video Studio | `internal/videostudio/AGENTS.md` | Desktop project/timeline model/local FFmpeg/render limits. |
| External Web UI assets | `internal/webassets/AGENTS.md` | Integrity/install/resolution/verification/external browser resources. |
| ZIM reader | `internal/zim/AGENTS.md` | Read-only archives/cluster cache/decompression bounds/fixtures; owns xapian full-text/title-index child. |
| Licenses | `licences/AGENTS.md` | External-project inventory/license conditions/redistributed texts. |
| Web UI | `ui/AGENTS.md` | External UI/Precision Workspace opt-in/protected Chat-Desktop surfaces/translations/verification; owns deeper contracts. |
| Quick Connect/shared MeshCore ports | `internal/desktop/AGENTS.md`, `internal/meshcore/AGENTS.md`, `internal/serialutil/AGENTS.md` | Shared serial ownership across features. |
| WebAuthn/passkey owner login | `internal/server/AGENTS.md`, `internal/security/AGENTS.md`, `ui/AGENTS.md`, `internal/webassets/AGENTS.md` | Protected auth policy/recovery/credential registry. |
| Chess 3D | `ui/js/desktop/apps/AGENTS.md`, `ui/3d/chess/AGENTS.md`, `assets/chess-3d/AGENTS.md` | Board/Blender piece kit. |
| Shared ONNX/person detection/Pixel AI | `internal/onnxrt/AGENTS.md`, `internal/persondetect/AGENTS.md`, `internal/pixelmatte/AGENTS.md` | All three own shared binding/cache/detection/background-matting changes. |

<!-- gitnexus:start -->
## GitNexus — Code Intelligence

Indexed repository: **AuraGo**. On query/context/impact/cypher object results check `staleness.status`, branch/lastCommit; `current` means clone HEAD, not main. Re-analyze only for behind/diverged: `node .gitnexus/run.cjs analyze --index-only`. Missing runner: bootstrap via `npx`, `bunx` or `pnpm dlx`, e.g. `bunx gitnexus@latest analyze` (#1939).

- **Before edits run impact**, never edit functions/classes/methods without it: `impact({target: "symbolName", direction: "upstream"})` or `node .gitnexus/run.cjs impact "symbolName" --direction upstream --repo .`. Report callers/processes/risk. Warn on HIGH/CRITICAL `risk`; `riskSharedAxes` never waives it. File/symbol risks differ (MCP File omits axes; Graph-RAG expands File).
- `risk: UNKNOWN` is unresolved, never safe/low. Empty callers may reflect plain-object property access/dynamic/cross-language calls; heed `riskNote` and confirm by text search before change/deletion. Never substitute grep for graph impact.
- Read-only graph-first analysis: `query({search_query: "concept"})` for concepts/flows, `context({name: "symbolName"})` for symbols, impact for blast radius; text search only for empty/UNKNOWN/literals. Security: `explain({target: "fileOrSymbol"})` for source→sink taint (needs `analyze --pdg`).
- Rename only with graph `rename`: preview `dry_run: true`, review semantic spans/coverage, apply `dry_run: false`; never find-and-replace.
- **Before commit run change analysis**: `detect_changes({scope: "all"})` or `node .gitnexus/run.cjs detect-changes --scope all --repo .`. `partial: true`/`truncated: true` is incomplete; rerun, never interpret unseen zeros as unaffected. Regression review: fetch remote `main`, compute merge base with reviewed HEAD, pass SHA to `detect_changes({scope: "compare", base_ref: "<merge-base SHA>"})` or `node .gitnexus/run.cjs detect-changes --scope compare --base-ref "<merge-base SHA>" --repo .`.

Resources: `gitnexus://repo/AuraGo/context` (overview/freshness), `gitnexus://repo/AuraGo/clusters` (areas), `gitnexus://repo/AuraGo/processes` (flows), `gitnexus://repo/AuraGo/process/{name}` (trace). Read the matching skill:

| Task | Skill |
| --- | --- |
| Architecture/flows | `.claude/skills/gitnexus-exploring/SKILL.md` |
| Blast radius | `.claude/skills/gitnexus-impact-analysis/SKILL.md` |
| Bugs | `.claude/skills/gitnexus-debugging/SKILL.md` |
| Rename/extract/split/refactor | `.claude/skills/gitnexus-refactoring/SKILL.md` |
| Tools/resources/schema | `.claude/skills/gitnexus-guide/SKILL.md` |
| Index/status/clean/wiki CLI | `.claude/skills/gitnexus-cli/SKILL.md` |
<!-- gitnexus:end -->

<!-- graft:start -->
## Graft — repo context graph

`graft/` contains linked system/context Markdown nodes with exact file:line spans (`graft/INDEX.md` is the entry). Consult the graph before source reads/search for understanding/location/scoping; reuse known symbols/errors/filenames. New repository: `graft map` gives directory clusters/hubs/hotspots (token-budgeted, deterministic, no key/LLM).

- `graft ask "<task>" --source`: ranked nodes/≤8-line excerpts; `--full` for whole definitions. Use top result, cite `covers:` file:line, edit there. Top-N is not exhaustive: `graft grep "<literal>"` finds all occurrences grouped by enclosing symbol; raw `grep -rn` only for unindexed files.
- `graft skeleton <file>`: every signature/span, ~10× cheaper than reading a file. `graft callers <symbol>`: exact callers; `--direction out` for callees, `--depth N` for transitive blast radius. Structural questions use callers directly.
- Monorepo hits carry `[scope/]`; scope with `graft ask "<task>" --in <scope>/`. Open truncated `+N more lines` at the exact range before finalizing; read source only for missing detail at returned file:line, never whole-file re-reads.
- After big code changes run `graft build` (deterministic, no key, $0).
<!-- graft:end -->
