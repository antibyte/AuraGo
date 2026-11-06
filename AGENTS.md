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

Go 1.27.3+; standard `net/http`; pure-Go `modernc.org/sqlite`; embedded `chromem-go`; vanilla JavaScript SPA; Python 3.10+ in a sandbox venv; Docker/Compose. Dependencies: `github.com/sashabaranov/go-openai` (compatible LLM client), `github.com/philippgille/chromem-go`, `github.com/go-telegram-bot-api/telegram-bot-api/v5`, `github.com/bwmarrin/discordgo`, `go.mau.fi/whatsmeow` (links GPL-3.0 `go.mau.fi/libsignal`, so distributed binaries are GPL-3.0 combined works), `github.com/robfig/cron/v3`, `golang.org/x/crypto` (SSH/bcrypt/ACME/Let's Encrypt), `github.com/gofrs/flock`, `tailscale.com`, `github.com/aws/aws-sdk-go-v2` (S3).

- `cmd/`: entry points; `cmd/aurago`: startup (`main.go`, `platform_unix.go`, `platform_windows.go`); `cmd/remote`, `cmd/config-merger`, `cmd/assetpack`, `cmd/memory-repair`: remote execution, config merging, UI packaging, reviewed offline analysis duplicate merging.
- `internal/`: private Go production logic. Core: `agent` (loop/dispatch/co-agents), `budget` (token cost), `commands` (slash commands), `config` (YAML/defaults), `llm` (client/failover/retry/pricing), `logger` (structured logs), `media` (files), `memory` (STM/LTM/KG/personality), `prompts` (dynamic prompts), `security` (Vault/tokens/Guardian), `server` (HTTP/API), `services` (indexing/ingestion), `setup` (first run), `tools` (implementations).
- Integrations in `internal/`: `contacts` (address book), `discord`/`rocketchat` (bots), `fritzbox` (TR-064), `invasion` (egg/nest), `inventory` (SQLite SSH devices), `meshcentral` (control WebSocket/devices/wake/power/MeshAgent commands), `mqtt`, `remote` (SSH/protocol), `scraper`, `sqlconnections` (external SQL), `telegram` (text/voice/vision), `telnyx` (SMS/voice), `tsnetnode` (Tailscale), `webhooks` (incoming/outgoing), `whatsapp` (owner-only WhatsApp linked device); `sandbox` owns Linux Landlock.
- `agent_workspace/`: runtime workspace; `agent_workspace/skills`: bundled Python skills; `agent_workspace/tools`: generated tools/manifest; `agent_workspace/workdir`: sandbox/venv. `prompts/`: `prompts/identity.md`, `rules.md`, `personalities/`, `templates/`, `tools_manuals/` own identity/rules/profiles/templates/RAG-indexed manuals.
- `ui/`: external HTML/CSS/JS, 16 locales, UI tests and `embed.go` fixtures. `assets/`: bundled/static/sample assets. `knowledge/`: application knowledge. `mcps/`: connector/tool definitions. `licences/`: third-party records/texts.
- `.github/`: Actions/agents/prompts/automation metadata. `scripts/`, `tools/`: developer/runtime helpers. `ansible_api/`: sidecar API. `browser_automation_sidecar/`: browser sidecar sources/support. `plans/`, `openspec/`: planning/specifications/change management. `docs/`, `documentation/`: user/operator guides.
- `deploy/`: tracked deployment/release inputs and ignored generated artifacts; `deploy/docker/` owns sidecar Dockerfiles and the main image's entrypoint. `config.yaml`: local config; `config_template.yaml`: runtime template and full reference. `Dockerfile`, `docker-compose.yml`: conventional container entrypoints. `install.sh`, `install.ps1`, `update.sh`, `repair-update.sh`: public bootstrap/update paths. Start/stop, service installation and release scripts live in `scripts/`.
- Keep the repository root compact: retain automatically discovered project/tool manifests, README/license/agent guidance, the runtime config template and public installer/updater URLs. Put developer scripts in `scripts/`, auxiliary container inputs in `deploy/docker/`, and third-party inventories/notices in `licences/`. Moving source inputs must preserve installed/archive basenames and existing public bootstrap URLs.
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

Plain `go build` without resource flags produces recovery only. Packaging assets cannot repair an unpinned binary: rebuild with the flags, verify `--check-assets`, then restart. Local startup: `./scripts/start.sh`; Windows: `scripts/start.bat` or PowerShell in `documentation/web-assets.md`.

Production releases must use `./scripts/make_deploy.sh` (Linux/macOS) or `scripts/make_release.bat` / `scripts/make_release.ps1` (Windows), not ad-hoc builds; these scripts do not upload to a test server. Side effects are listed below.

### Verification and editing

- `.gitnexusignore` owns code-index scope: exclude runtime/scratch data, generated resource copies, binary assets and vendored payloads; retain maintained source/tests, including both versioned training utilities under `disposable/`. Validate exclusions and retained paths with the installed GitNexus ignore filter and path walker before rebuilding; avoid broad negations that re-admit dependencies or binaries.
- With ignored scratch Go files use `./scripts/go-test-with-cache.ps1` (also accepts `./...`): production roots and both versioned training utilities, excluding scratch directories. Literal `go test ./...` is for clean checkouts. Temporary Go files belong in `disposable/_<task>/` (Go skips underscore directories); never add a module boundary above tracked `disposable/export_tools` or `disposable/import_training_traces`.
- Focused tests: `go test ./internal/config/...`, `go test ./internal/agent/...`, `go test ./internal/memory/...`, `go test -v ./internal/tools/...`. Coverage: `go test -cover ./internal/...`; race: `go test -race ./...`; benchmarks: `go test -bench=. ./internal/...`.
- Put `*_test.go` beside source; name tests `TestFunctionName`; prefer table-driven cases (e.g. `internal/config/config_test.go`, `internal/tools/shell_test.go`, `internal/memory/history_test.go`, `internal/agent/agent_test.go`). Cover critical/complex paths and new features with focused unit and tool/workflow integration tests.
- Wrap errors (`fmt.Errorf("context: %w", err)`); use structured `slog` (`log.Info("message", "key", value)`). Packages: short/lowercase/no underscores. Files: `snake_case.go`; exported types/constants/functions: `PascalCase`; unexported names/variables: `camelCase`. One responsibility per file; split oversized files.
- Windows/PowerShell: never undo a broken edit with `git checkout <file>` (discards others' work); inspect the diff and revert only the failed edit. `[System.Text.Encoding]::UTF8` adds a BOM; use `[System.Text.UTF8Encoding]::new($false)` or another BOM-free writer; inspect the first three bytes when relevant (BOM breaks Go `encoding/json`). Normalize mixed CRLF/LF to LF per `.gitattributes`.
- Before tests check encoding and `git diff --check`; JS: `node --check <file.js>`; JSON: `python -m json.tool <file.json>` or another parser. Windows isolated build/release checks use `scripts/invoke-clean-worktree.ps1` (`finally` removes worktree/prunes metadata), not ad-hoc `%TEMP%` worktrees.
- GitHub Pages renders `docs/` with Jekyll 3.10. Wrap literal Liquid-like code in `{% raw %}` / `{% endraw %}`; Security Gates build the same documentation without deployment.
- Linux Security Gates Go tests serialize packages (`-p 1`) to keep compilers, tests and race instrumentation within runner resources. The race run covers `llm`, `server`, `memory`, `tools`, `telegram`, `whatsapp`, and `agent` with a 30-minute per-package timeout.
- Security Gates installs and verifies FFmpeg/FFprobe before Go tests. Its Linux UI matrix requires Chrome and `AURAGO_RUN_BROWSER_SMOKE=1`; missing browsers or launch failures fail the job. Run the populated-config matrix, config-layout matrix and remaining UI tests in independent 30-minute test shards with fail-fast disabled; each job gets 60 minutes to include checkout, setup and compilation. `TestUIBrowserCIShardsCoverEveryTest` verifies complete, non-overlapping coverage.
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

The six existing API-key environment overrides apply only after a valid static provider binding: main LLM (`LLM_API_KEY`, then `OPENAI_API_KEY`, then `ANTHROPIC_API_KEY`), co-agent LLM, embeddings, vision, Whisper and fallback LLM. Empty variables preserve the provider key; overrides affect only that resolved slot, never provider entries or persisted config. OAuth, managed, masked and invalid bindings retain their own rules on startup and reload.

### Install and development reset

Docker is recommended: `docker-compose up -d`; `docker-compose -f docker-compose.yml up -d` selects the default file explicitly. Linux:

- Verified install: obtain `install.sh` over HTTPS from `https://github.com/antibyte/AuraGo/releases/latest/download/`, then `bash install.sh`; signature and payload verification are automatic, without user key/tool setup.
- Main-branch quick install: `curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash`.
- Manual: `wget https://github.com/antibyte/AuraGo/releases/latest/download/aurago_linux` (arm64: `aurago_linux_arm64`), `chmod +x aurago_linux`, `./aurago_linux`. Systemd: `sudo ./scripts/install_service_linux.sh`.

Development reset deletes databases/vectors/venv and removes locks after stopping:

```bash
./scripts/kill_all.sh
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
- Keep runtime state, build scratch and browser-test output under `data/`, `tmp/` and `ui/reports/` ignored. Machine-local assistant settings and uncurated scratch plans may use exact `.git/info/exclude` entries; keep the versioned `plans/` area available for reviewed project plans.
- Vault replacement preserves the original until a synced temporary file is renamed through `internal/fileutil`; verify `TestVaultAtomicReplaceHandlesWindowsReaders`.
- Call `security.RegisterSensitive(value)` before any network call, including SDK constructors; shared logging scrubs messages, bound attributes and nested errors to prevent log/LLM disclosure. Add new tool/integration Vault secrets to the Python-tool export denylist; keep them out of the agent environment.
- Config gates: `sudo_enabled` (sudo), `sudo_unrestricted` (writes beyond install dir; requires removing systemd `ProtectSystem=strict`), `allow_shell`, `allow_python`, `allow_filesystem_write`, `allow_network_requests` (HTTP), `allow_remote_shell` (SSH), `allow_self_update`, `allow_mcp`, `allow_web_scraper`. Nonessential tools/integrations need activation toggles; harmful capabilities default off and stay UI-disableable. Assess exposure/security; mutating/critical tools need read-only toggles and, where needed, separate `read`, `write`/create, `change`/update, `delete`/remove grants.
- Operator commands (`/sudopwd`, `/addssh`, `/restart`, `/personality`, `/debug`, `/voice`) require `commands.Context.AllowOperator`: Telegram private chats, WhatsApp owner chats, Discord DMs, `rocketchat.allow_operator_commands`, web console, desktop chat, mission runs; never model-written follow-up loopback turns. Classify each new command in `operatorCommands` or its test.
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
- Music Player (`audio-player`): native window app that opens when an audio file is opened from the file manager or the desktop; the fullscreen gallery lightbox stays for images and videos. A Web Audio analyser feeds the canvas spectrum (`bars`/`radial`/`wave`), ID3 tags and cover art are read from the file itself, and `AuraDesktopMediaSession` carries the OS media keys. Owns `ui/js/desktop/apps/audio-player*.js` and `ui/css/desktop-app-audio-player.css`; the WebAmp/Winamp launcher stays the separate `music-player` app.
- Goldfish Adventure (`goldfish-adventure`, **Bitte nicht das Universum füttern**) is a local built-in Desktop adventure. `plans/2026-11-06-goldfish-adventure.md` owns story and production scope; `ui/js/desktop/apps/goldfish-adventure/AGENTS.md` owns runtime, transactional browser saves and localization; `assets/goldfish-adventure/AGENTS.md` owns ImageGen masters, staging and audio. No backend, cloud save or provider key is needed. For this project use Sol 6.1 at Max for implementation and Terra 6.1 for reviews and supporting tasks, as specifically requested.

### Dependencies, catalogs and delivery

- Update Go/npm/vendor assets/Python locks/Rust TUI together: stable releases, documented security/upstream bounds. Exceptions/rebuild checks: `documentation/dependencies.md`; version `tools/aurago-tui/Cargo.lock`, test `--locked`.
- Registry: `go run scripts/generate_model_registry.go --write` from `https://models.dev/api.json` to `internal/llm/model_registry_data.go`, then `--check`. Catalog: `go run scripts/sync_ohmypi_catalog.go --write`, then `--check`; take `src/models.json` and compiled `src/compat/rules.json` from the same `@oh-my-pi/pi-catalog` npm tarball. LLMs only; preserve explicit `supportsTools`; mark model-only providers catalog-only; test catalog/registry/server.
- Treat `config.yaml` carefully; align install/update scripts and Dockerfiles with system/dependency changes. AuraGo manages its Docker containers; do not assume manual user management.
- Compose socket proxy defaults to `BUILD=0`. Code Studio/most sidecars pull with `IMAGES=1`, `POST=1`; Engine builds (Homepage first-use dev image, CommandCode fallback, local Code Studio runtimes, rate-limit Caddy image) need `BUILD=1` or preexisting host images. `aurago --print-homepage-dockerfile` supports host builds; see `documentation/docker_installation.md` section 4. Changing default to `BUILD=1` requires an explicit decision.
- `docker-publish.yml` publishes GHCR linux/amd64 + linux/arm64 on `v*` tags/manual dispatch (`image=all` / `image=gods-eye-view` / `image=kaggle-notebook`), never `main` pushes alone. Kaggle notebook publication requires amd64/arm64 bridge smokes and signs the fixed protocol image tag; keep `internal/notebooks` and its Dockerfile aligned. Pin workflow `uses:` to full SHA plus ` # vX.Y.Z`; default `permissions: contents: read`, job-specific writes only (`internal/audit` rejects floating refs). `.github/dependabot.yml` refreshes monthly except manual `dtolnay/rust-toolchain`; `govulncheck@v1.8.0` in `security-gates.yml` is manual too. Training Python/uv/GPU research stays in `antibyte/agotrain`; AuraGo owns Go exporter/importer/pinned checks.
- `./scripts/make_deploy.sh` cross-builds and may commit/push `main` by default (`--no-publish` suppresses); no tag/GitHub Release. Windows `scripts/make_release.bat`/`scripts/make_release.ps1` default to `scripts/start-release-ci.ps1`: push committed `main`, dispatch `release.yml` with its expected SHA and wait for success. Local drafts stay local. The workflow requires the exact commit's complete Security Gates run, builds all native assets in a clean checkout and uses GitHub OIDC signing. `scripts/make_release.ps1 -Local` or explicit signing-key configuration retains local builds. Builders pin `GOTOOLCHAIN=go1.27.3`; verify compiler before publishing even if system Go is newer.
- `release.yml` uploads all assets to a draft before publishing, keeps existing releases/tags (`AURAGO_KEEP_RELEASES=1`) and explicitly dispatches `docker-publish.yml` for the completed tag because `GITHUB_TOKEN` tag creation does not trigger push workflows. Local keyed Windows publication retains the latest three stable AuraGo releases and keeps the tags of deleted ones (their GPL-3.0 corresponding source) unless retention is disabled; exclude drafts and integration prereleases before counting.
- Windows release builders resolve Python 3.10+ through `scripts/find-release-python.ps1` (`py -3`, then `python`/`python3`) before cleanup. Python prepares Phonon resources and checks reproducible release-verifier artifacts; end users need neither Python nor Cosign for signature verification.
- Official native install/update paths automatically provision a size/SHA-256-pinned verifier and trust its embedded public key. Missing/invalid signatures fail before shutdown or payload execution. `scripts/build-release-verifier.py` owns generated bootstrap blocks in `install.sh`, `update.sh`, `repair-update.sh`; retain its immutable bootstrap prereleases. See `internal/releaseverify/AGENTS.md` and `documentation/release-signatures.md`.
- Default Windows release dispatch needs Git, GitHub CLI authentication and PowerShell, not a local signing profile. Optional local maintainer signing uses explicit key configuration and `scripts/setup-release-signing.ps1`; keys stay in the user-only LocalAppData profile with a DPAPI-protected password. Actual builders preflight signing before cleanup/build. See `documentation/release-signatures.md` and `internal/upkeep/AGENTS.md`.
- Set `AURAGO_KEEP_RELEASES=1` when publishing without authorization to delete older releases; both Windows builders then skip retention entirely.
- Release cleanup preserves `deploy/docker` sources and verified `deploy/phonon` inputs. A clean worktree needs the ignored Phonon inputs copied from the prepared checkout before packaging; never bypass the dirty-worktree guard to include unrelated changes.

## Shared runtime contracts

- Shutdown signals and the egg master's stop command use one idempotent request. Notify server cancellation before blocking cleanup; drain the server and agent side effects before killing remaining processes or closing shared LLM/site-monitor resources. Signal handlers never close cron/background stores.
- `internal/agent/agent_loop.go`: receive → LLM native function calling → tool dispatch/execution → SSE response → recovery/retry.
- One HTTP server, SSE, REST `/api/`; verified external full UI, embedded recovery/login. Fresh installs use local self-signed HTTPS: native 443 or 8443, Docker 8443; upgrades preserve HTTP/TLS. See `internal/server/AGENTS.md`, `documentation/https.md`.
- AgoDesk extracts prose `/files/...` before signing; Markdown/JSON-escape/query/fragment delimiters never become filename bytes. Preserve escaped filenames/structured payload queries when rewriting; never repair contaminated signed URLs in the asset handler. Tests: `TestAgodeskMediaReferencesServeSignedAudio`, `TestAgodeskChatBrokerDeduplicatesDelimitedMediaPaths`.
- `internal/httpstream.WithWriteTimeout` wraps outer handlers for local/HTTPS/Tailscale listeners/proxies. Successful SSE/MJPEG renew existing finite write budgets per write/flush; ordinary/error responses keep absolute timeout. Write/flush failure cancels requests, never revives streams. Preserve WebSocket hijacking/response-controller access. Verify `go test ./internal/httpstream`, server `TestAgentHTTPServerKeepsSSEAliveThroughMiddleware`, Tailscale proxy tests.

## Child DOX Index / cross-component routing

**Read every matching owner below in addition to the normal DOX chain before changing a listed feature, including server/UI/config/assets/tests/workflows outside its subtree.** Feature contracts remain repository-wide; this combined ownership/routing index does not narrow scope. Child guides own deeper indexes.

| Feature / area | Required AGENTS.md | Scope |
| --- | --- | --- |
| Repository scripts | `scripts/AGENTS.md` | Developer entrypoints, project-root resolution and release helper paths; runtime/update policy remains in root and upkeep guides. |
| Go test utilities | `internal/testutil/AGENTS.md` | Local protocol/TLS fixtures, test-only root trust, synthetic storage and asset helpers. |
| Auxiliary Docker inputs | `deploy/docker/AGENTS.md` | Sidecar Dockerfiles, main entrypoint and repository-root build contexts; feature-specific children retain their contracts. |
| Low-poly Game Maker pack | `assets/game-maker-low-poly/AGENTS.md` | Original 220-model Blender pack, animations, compact exports, playable acceptance scenes. |
| Game Maker presentation | `assets/game-maker-presentation/AGENTS.md` | Effects/audio sources, licensing, builds, runtime limits. |
| Game Maker worlds | `assets/game-maker-worlds/AGENTS.md` | Maritime/isometric Blender sources, catalog counts, exports, runtime limits. |
| AuraGolf | `assets/minigolf/kit/AGENTS.md` | Procedural Blender tracks/courses/props, exports, collisions. |
| Aura Drift assets | `assets/aura-drift/AGENTS.md` | Deterministic Blender model kit (20 GLBs in `ui/3d/aura-drift/v1`), own-synthesis audio generator and CC0 override hook. |
| Goldfish Adventure | `assets/goldfish-adventure/AGENTS.md`, `ui/js/desktop/apps/goldfish-adventure/AGENTS.md` | ImageGen masters, animation staging, local audio, authored story, browser saves, sixteen locales and Desktop integration. |
| Tiefsee screensaver | `assets/screensaver-abyss/AGENTS.md` | Creature generator, deterministic exports, asset checks. |
| System World assets | `assets/system-world/AGENTS.md` | Blender city authoring/original sources/reproducible compact exports. |
| Memory repair CLI | `cmd/memory-repair/AGENTS.md` | Offline analysis duplicate merging, previews/evidence/backups, resumable reference/vector cleanup. |
| ACE-Step | `internal/acestep/AGENTS.md` | Private local music lifecycle, pinned runtime/models, hardware qualification. |
| Agent/tool system | `internal/agent/AGENTS.md` | Runtime prompts/discovery/dispatch/context; Prompt and Runtime Drift Contract. |
| Email integrations and Mailer | `internal/agentmail/AGENTS.md`, `internal/mailer/AGENTS.md`, `internal/server/AGENTS.md`, `internal/agent/AGENTS.md`, `internal/tools/AGENTS.md` | AgentMail HTTPS/redirects/provider errors/delete grant; classic mail target policy, legacy migration, shared probes and Desktop source permissions. |
| A2A protocol | `internal/a2a/AGENTS.md`, `internal/agent/AGENTS.md` | Peer cards/HTTP authentication, clients, session-scoped worker admission and shutdown. |
| Agent self-tests | `internal/selftest/AGENTS.md`, `prompts/self_tests/AGENTS.md` | Prompt catalog, bounded runs, trusted evidence, private reports and admin Dashboard controls. |
| Bluetooth | `internal/bluetooth/AGENTS.md` | Native discovery/permissions/playback. |
| Bookshelf | `internal/bookshelf/AGENTS.md` | Library/sources, reader isolation, conversion. |
| CYD notification devices | `internal/cyd/AGENTS.md`, `internal/server/AGENTS.md` | Glass notifications, MeshCore announcements and shared TTS/PCM playback. |
| CYD Creator | `internal/cydcreator/AGENTS.md`, `internal/server/AGENTS.md`, `internal/agent/AGENTS.md` | Board/device profiles/themes/widgets/layout ops/validation/partitions/config blob; `cyd_template` runtime, builder image/deterministic generator, SQLite/storage, diagnostics/events/service, queue/jobs/revisions/tools/HTTP API. |
| CYD Creator icon artwork | `assets/cyd-icons/AGENTS.md`, `internal/cydcreator/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md` | Shared motif IDs, ImageGen masters/prompts, deterministic sprite exports, translated picker and firmware frame acceptance. |
| Decisions service | `internal/decisions/AGENTS.md` | Optional shared service, verified artifacts/device placement/CPU permission/bounded scoring. |
| Decisions runtime | `decisions_runtime/AGENTS.md` | Pinned llama.cpp/mtmd scorer, private protocol/offload evidence/native-Docker packaging, HTTP `runtime_busy`. |
| Desktop files/archives | `internal/desktop/AGENTS.md` | Rooted operations; owns pets-assets child index. |
| God's Eye View Store | `internal/desktopstore/AGENTS.md` | Store app config/runtime/assets/publication. |
| Detective | `internal/detective/AGENTS.md` | Isolated Desktop research cases/evidence/budgets/revisions/exports. |
| Docker Engine | `internal/dockerutil/AGENTS.md` | Transport/API negotiation, mutation retry, streaming cancellation. |
| Docker Manager hosts | `internal/dockerhost/AGENTS.md` | Host registry, vault secrets, TLS/SSH transport shared with Invasion, Engine reads and container actions behind `/api/docker-manager`. |
| Docker Manager stacks | `internal/dockerstacks/AGENTS.md` | Stack folders per host, compose runner/lister/IO containers, file revisions, operations journal, reattach after restart, lint and `docker run` converter behind `/api/docker-manager`. |
| Docker Manager live data | `internal/dockerwatch/AGENTS.md` | Engine events watcher per host, container stats rings and host gauges, disk-usage cache, stream budget and the stream-client hub behind `/api/docker-manager/stream`. |
| ECalc | `internal/ecalc/AGENTS.md`, `internal/server/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md` | Electronics formula catalog, expression language, units, E-series and solver with shared Go/JS parity vectors; `GET /api/ecalc/catalog`; Desktop app and manual. |
| Storymaker | `internal/storymaker/AGENTS.md`, `internal/storymaker/render/AGENTS.md`, `ui/js/storymaker/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `internal/server/AGENTS.md`, `internal/desktop/AGENTS.md` | Explainer-video Desktop app: story document, validation and catalog index, project store, code-scene bundler, JS player (host, workers, compile, rig renderer, speech, captions), generated starter catalog with glyph data and sound effects (`assets/storymaker`, `ui/img/storymaker`), narration and music jobs, the Go mixer, render job and listener, export options, stems, subtitles, library copy and Video Studio handoff, app modules, config section and `documentation/storymaker.md`. |
| Managed RustFS S3 | `internal/rustfs/AGENTS.md` | Optional Docker storage lifecycle, private IAM credentials, scoped S3 leases and retained-volume recovery; server/UI/config/tool integration. |
| Kaggle integration | `internal/kaggle/AGENTS.md`, `internal/notebooks/AGENTS.md` | Fixed native Kaggle contracts, account grants, durable jobs, project transfers, notebook preservation and managed local Jupyter; agent/server/UI/release integration. |
| File utilities | `internal/fileutil/AGENTS.md` | Shared file replacement/free-space probe. |
| EasyDrag | `internal/flows/AGENTS.md`, `internal/server/AGENTS.md`, `ui/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md` | Flow authoring, execution, approval/session authority, Desktop UI and Decisions media authorization. |
| Fritz!Box | `internal/fritzbox/AGENTS.md` | TR-064 integration/Desktop widget. |
| Game Maker | `internal/gamemaker/AGENTS.md` | Studio/planning/runtime feedback/progression/lifecycle/tool validation/sprite library/exports; owns asset-pack child index. |
| Aura Drift | `ui/js/desktop/apps/AGENTS.md`, `ui/AGENTS.md`, `internal/desktop/AGENTS.md`, `assets/aura-drift/AGENTS.md`, `documentation/aura-drift.md`, `plans/2026-11-06-aura-drift-3d-overhaul.md` | Offline Virtual Desktop racer: three.js 3D view (default, WebGL2 required, Canvas 2D fallback) with Blender models, deterministic 120 Hz bicycle physics, eight spline tracks, seven modes, eight cars, 18 weapons, 16 tricks, own-synthesis Ogg sound pack with live fallbacks, drift-chain style scoring, seeded weather, sixteen locales; verify `TestAuraDrift*`, `TestDesktopAuraDrift*Browser` and the `scripts/test-aura-drift-*.mjs` checks. |
| HTTP streams | `internal/httpstream/AGENTS.md` | Finite write budgets/HTTP writer compatibility. |
| Infra | `internal/infra/AGENTS.md` | Homelab atlas/source identity/layout/durable action outcomes. |
| FindIt | `internal/findit/AGENTS.md`, `assets/findit/AGENTS.md` | Physical inventory, typed templates, stock/project reservations, CSV, photos, QR labels, read-only agent access and Desktop integration. |
| Invasion | `internal/invasion/AGENTS.md` | Egg/nest protocol/auth/connection generations/key rotation/private deployment. |
| LLM transport | `internal/llm/AGENTS.md` | Trust/URL logging/failover state. |
| Launchpad links/icons | `internal/launchpad/AGENTS.md` | Link/icon reference validation, rooted local artwork ownership, download limits and cleanup. |
| Managed local models | `internal/localllm/AGENTS.md` | Lifecycle/routing/attestation/qualification. |
| Local Wikipedia Contract | `internal/localwiki/AGENTS.md` | Offline editions/download/verify/swap/search/tools/content; readers `internal/zim`, `internal/zim/xapian`. |
| Memory | `internal/memory/AGENTS.md` | Retrieval/hygiene/indexing/maintenance. |
| MeshCentral | `internal/meshcentral/AGENTS.md` | Control WebSocket/power gate/command limits/TLS CA/admin connection test. |
| MeshCore | `internal/meshcore/AGENTS.md` | USB/BLE radio/trust/messaging/agent replies. |
| MQTT | `internal/mqtt/AGENTS.md` | Broker config/subscriptions/relays/mission dispatch. |
| Network shares | `internal/networkshares/AGENTS.md` | SMB/NFS capability/ownership/mutation policy. |
| Newspaper | `internal/newspaper/AGENTS.md` | Daily publication/source validation/scheduler/delivery receipts. |
| Desktop Office | `internal/office/AGENTS.md` | Workbook/document preservation/editing/assist. |
| Shared QR codes | `internal/qrcode/AGENTS.md`, `internal/office/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md` | Shared renderer/templates/scanner/config/batch, native tool and Writer/Tabellen/Pixel insertion; font child contract `ui/fonts/qr/AGENTS.md`. |
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
| Local prompt-injection model | `internal/injectionguard/AGENTS.md`, `internal/security/AGENTS.md`, `internal/onnxrt/AGENTS.md` | Optional Horizon CPU model, verified downloads, isolated worker, web/mail gates and administrative Config controls. |
| Serial ports | `internal/serialutil/AGENTS.md` | Shared enumeration/canonical reservations/cancellation-safe leases. |
| HTTP/integrations | `internal/server/AGENTS.md` | HTTP/cross-component contracts: System World Tower Voice, 3D Printer, go2rtc, AI Gateway, here.now, GitHub, Homepage Managed Website Ledger, Configuration UI Integration Test Contract. |
| Bounded PDF extraction | `internal/pdftext/AGENTS.md` | Shared indexer/tool parser, resource limits, cancellation and pinned source/license. |
| Incoming webhooks | `internal/webhooks/AGENTS.md`, `internal/server/AGENTS.md` | Authenticated ingress, optional persistent replay protection, admin/Vault publication and editor. |
| Workspace search | `internal/services/AGENTS.md` | Background services/search. |
| Native SIP | `internal/sipphone/AGENTS.md` | Telephone registration/calls/media/agent policy. |
| Speech Lab | `internal/speechlab/AGENTS.md` | Active ASR/TTS snapshots/speech-driven chat routing. |
| External SQL | `internal/sqlconnections/AGENTS.md` | Permissions/managed SQLite imports. |
| Telnyx | `internal/telnyx/AGENTS.md` | Call/SMS transport/bounded provider-confirmed reconciliation. |
| Desktop Tresor | `internal/tresor/AGENTS.md` | Browser-encrypted storage/revisions/backups/agent file protection. |
| Agent tools | `internal/tools/AGENTS.md` | Filesystem Jail/Docker Inspect/Managed Space Agent/treg catalog gateway safety. |
| Rust TUI | `tools/aurago-tui/AGENTS.md` | Terminal client HTTP/SSE timeouts, private session files and locked Rust verification. |
| Updates | `internal/upkeep/AGENTS.md` | Transactions/artifact retention/maintenance CLI/cleanup safeguards. |
| Release verification | `internal/releaseverify/AGENTS.md`, `internal/upkeep/AGENTS.md` | Offline signature checks, pinned automatic bootstrap and old-updater repair. |
| Virtual Computers | `internal/virtualcomputers/AGENTS.md` | Workspace leases/managed Garage storage lifecycle. |
| Video Studio | `internal/videostudio/AGENTS.md` | Desktop project/timeline model/local FFmpeg/render limits. |
| WhatsApp channel | `internal/whatsapp/AGENTS.md`, `internal/server/AGENTS.md`, `internal/planner/AGENTS.md`, `ui/AGENTS.md`, `licences/AGENTS.md` | Linked-device store/runtime/link sessions, admin endpoints, Config link panel, GPL-3.0 binary licensing; text channel (inbound filter, agent pipeline, broker, send queue), dashboard tile, media in both directions, `send_whatsapp`, notification channel `whatsapp`, camera alert photos, EasyDrag `notify.whatsapp` and the failure channel, newspaper delivery; operator guide (`documentation/whatsapp_setup.md`), security hints, passive briefing check, release licence gate (`internal/audit` `TestLinkedGoModulesHaveGPL3CompatibleLicenses`, `TestReleasePackagingCarriesTheGPLNotices`). |
| External Web UI assets | `internal/webassets/AGENTS.md` | Integrity/install/resolution/verification/external browser resources. |
| ZIM reader | `internal/zim/AGENTS.md` | Read-only archives/cluster cache/decompression bounds/fixtures; owns xapian full-text/title-index child. |
| Licenses | `licences/AGENTS.md` | External-project inventory/license conditions/redistributed texts; linked-module licence gate (`licences/linked-go-modules.tsv`, `internal/audit/linked_module_licenses_test.go`) and release notice packaging (`internal/audit/whatsapp_release_packaging_test.go`). |
| Desktop icon artwork | `assets/desktop-icon-themes/AGENTS.md` | Pinned imports, supplements, source licenses and generated theme manifests. |
| Web UI | `ui/AGENTS.md` | External UI/Precision Workspace opt-in/protected Chat-Desktop surfaces/translations/verification; owns deeper contracts. |
| Quick Connect/shared MeshCore ports | `internal/desktop/AGENTS.md`, `internal/meshcore/AGENTS.md`, `internal/serialutil/AGENTS.md` | Shared serial ownership across features. |
| WebAuthn/passkey owner login | `internal/server/AGENTS.md`, `internal/security/AGENTS.md`, `ui/AGENTS.md`, `internal/webassets/AGENTS.md` | Protected auth policy/recovery/credential registry. |
| Chess 3D | `ui/js/desktop/apps/AGENTS.md`, `ui/3d/chess/AGENTS.md`, `assets/chess-3d/AGENTS.md` | Board/Blender piece kit. |
| Shared ONNX/person detection/Pixel AI | `internal/onnxrt/AGENTS.md`, `internal/persondetect/AGENTS.md`, `internal/pixelmatte/AGENTS.md` | All three own shared binding/cache/detection/background-matting changes. |

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **AuraGo** (228760 symbols, 1440505 relationships, 1834 execution flows).

> Index stale? Run `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs analyze --index-only` from the project root — it auto-selects an available runner. No `../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs` yet? Bootstrap with `npx`, `bunx`, or `pnpm dlx` — e.g. `bunx gitnexus@latest analyze` (npm 11 npx crash; #1939).

## Always Do

- **MUST run impact before editing.** Use `impact({target: "symbolName", direction: "upstream"})` or `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "symbolName" --direction upstream --repo .`; report callers, processes, and risk. Never substitute grep for graph analysis.
- **MUST analyze graph changes before committing.** Use `detect_changes({scope: "all"})` (MCP) or `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo .` (CLI fallback). `partial: true` or `truncated: true` is not a clean check — a zero means unseen, not unaffected; re-run it. For regression review: `detect_changes({scope: "compare", base_ref: "main"})` or `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope compare --base-ref "main" --repo .`.
- MUST warn on HIGH/CRITICAL `risk` pre-edit; never use `riskSharedAxes` to waive a HIGH/CRITICAL `risk` warning. Compare File/symbol: MCP File omits axes; Graph-RAG expands File.
- **MUST treat `risk: UNKNOWN` as unresolved, not as low.** An empty caller set is not evidence the symbol is unused — it can also mean the callers are not resolvable by the index (plain-object property access, dynamic dispatch, cross-language calls). `impact` pairs `UNKNOWN` with a `riskNote` saying so. Confirm with a text search before treating the symbol as safe to change or delete; do not proceed on the strength of a zero.
- **MUST use `query({search_query: "concept"})` for concepts/flows, `context({name: "symbolName"})` for a named symbol, or `impact` for blast radius, on read-only callers, dependencies, imports, or execution flow.** Graph first; text search only for empty/`UNKNOWN`/literals.
- For security review, `explain({target: "fileOrSymbol"})` lists taint findings (source→sink flows; needs `analyze --pdg`).

## Never Do

- NEVER edit a function, class, or method before MCP/CLI impact analysis.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis, and never read `UNKNOWN` as an all-clear — it means the walk could not answer, which is the one verdict that requires confirming by other means.
- NEVER rename symbols with find-and-replace — use `rename` which understands the call graph.
- NEVER commit before MCP/CLI graph change analysis.

## Resources

| Resource | Use for |
| --- | --- |
| `gitnexus://repo/AuraGo/context` | Codebase overview, check index freshness |
| `gitnexus://repo/AuraGo/clusters` | All functional areas |
| `gitnexus://repo/AuraGo/processes` | All execution flows |
| `gitnexus://repo/AuraGo/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
| --- | --- |
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->

<!-- graft:start -->
## Graft — repo context graph

This repo is indexed in `graft/`: small linked markdown nodes that explain each
system and carry exact file:line spans, kept in sync with the code through git.

For ANY task here — understanding how something works, finding where code lives,
or scoping a change — get context from the graph before grepping or opening
source files. Re-ask freely (it's cheap) and reuse literal identifiers you
already have (symbol, error string, file name) as the query. New to this repo?
Run `graft map` first — a token-budgeted orientation (dir clusters, hubs,
hotspots), no LLM, no key.

- Run `graft ask "<your question>" --source` → ranked nodes with the relevant
  code spans inlined (each hit's ≤8-line crux by default; `--full` for whole
  definitions when the crux isn't enough). Match the tool to the task shape:
  for understanding or editing, the top node IS the answer — cite its
  `covers:` file:line spans and edit straight from `--source`. For
  exhaustive tasks ("every occurrence / every caller of this pattern"), ranked
  results are top-N, not complete — run `graft grep "<literal>"` instead
  (exhaustive over indexed files, grouped by enclosing symbol), falling back
  to raw `grep -rn` only for unindexed files.
- `graft skeleton <file>` → every definition's signature + span, ~10× cheaper
  than reading the file; use it to skim an API surface.
- `graft callers <symbol>` gives precomputed, exact edges — who calls this.
  Add `--direction out` for what it calls, or `--depth N` to walk
  transitively for the full blast radius. For structural questions, skip
  ranking and use this directly.
- Or browse: `graft/INDEX.md` lists every node; follow the links.
- Monorepos and folders of multiple repos rank fairly across sub-projects —
  hits carry `[scope/]` labels naming which one they're from. Narrow with
  `graft ask "<task>" --in <scope>/` once you know where you're working.

If a returned span is truncated ("+N more lines"), open the file at that exact
range before finalizing. Only open source files when a node genuinely lacks a
needed detail, and then at the exact file:line the node points to — never
re-read whole files.

After big code changes, refresh the graph with `graft build` (deterministic,
no API key, $0).
<!-- graft:end -->
