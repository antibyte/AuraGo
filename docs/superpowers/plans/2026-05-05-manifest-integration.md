# Manifest Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integrate Manifest.build into AuraGo as an optional managed LLM gateway that can run as AuraGo-managed Docker sidecars, be configured from the Web UI, store credentials in the vault, expose its dashboard safely, and be selectable as an OpenAI-compatible provider.

**Architecture:** Add a first-party `manifest` integration that follows the existing Browser Automation and Space Agent sidecar patterns: configuration defaults in `internal/config`, managed container lifecycle in `internal/tools`, status/control endpoints in `internal/server`, optional tsnet reverse proxy in `internal/tsnetnode`, and a vanilla JavaScript config module with all 15 language files. Manifest itself remains an external Docker image; AuraGo owns orchestration, secrets, provider wiring, and UX.

**Tech Stack:** Go 1.26, Docker Engine HTTP API, vanilla JavaScript config SPA, AuraGo vault, existing provider resolver, existing tsnet reverse proxy manager, `manifestdotbuild/manifest:5`, `postgres:15-alpine`.

---

## Report Evaluation

The report in `reports/manifest_integration_bewertung.md` is directionally correct and implementable. The proposed integration model fits AuraGo: Manifest should be a managed sidecar, not a bundled library, and the LLM path should use AuraGo's existing OpenAI-compatible provider system.

Required corrections before implementation:

- Use `manifestdotbuild/manifest:5` as the default image, not `latest`. Official Manifest docs recommend major-version pinning for production control.
- Do not publish PostgreSQL to the host by default. Manifest and Postgres should communicate over an internal Docker network; only the Manifest dashboard/API port should be published or proxied.
- Do not generate `BETTER_AUTH_SECRET` or the Postgres password ephemerally during container creation. They must be persisted in AuraGo's vault before managed auto-start is allowed.
- Treat first-admin creation as a guided manual setup step. AuraGo cannot fully automate the first Manifest account without depending on undocumented internals.
- Prefer `type: manifest` as a thin AuraGo provider alias that resolves to an OpenAI-compatible `custom` client path. This keeps provider UX clean without forking the LLM client.
- Keep Tailscale exposure optional and implemented through the existing tsnet node, matching the report's recommended reverse-proxy approach.

External facts checked against official sources:

- Manifest self-hosting uses Docker, `manifestdotbuild/manifest`, port `2099`, PostgreSQL, `DATABASE_URL`, `BETTER_AUTH_SECRET`, `BETTER_AUTH_URL`, and `MANIFEST_TELEMETRY_DISABLED`.
- The official self-hosting docs bind the dashboard to `127.0.0.1` by default and document `postgres:15-alpine`.
- Docker Hub documentation recommends pinning a major image tag such as `manifestdotbuild/manifest:5`.

Sources:

- `https://manifest.build/docs/docker`
- `https://manifest.build/docs/self-hosted`
- `https://hub.docker.com/r/manifestdotbuild/manifest`

## File Structure

- Modify `internal/config/config_types.go`: add `ManifestConfig`; extend `Config` and `TsNetConfig`.
- Modify `internal/config/config.go`: add Manifest defaults, URL normalization, tsnet Manifest defaults.
- Modify `internal/config/config_migrate.go`: add vault key mappings, provider type recognition, Manifest provider base URL resolution.
- Modify `internal/config/config_test.go`: add config default, vault, provider, and Docker/bare-metal URL tests.
- Create `internal/tools/manifest.go`: managed Manifest/Postgres sidecar config, Docker payload builders, lifecycle, and status helpers.
- Create `internal/tools/manifest_test.go`: unit tests for sidecar config, network selection, env generation, URL selection, and safety checks.
- Modify `internal/server/server.go`: start managed Manifest during server startup when enabled and auto-start is true.
- Modify `internal/server/config_handlers_main.go`: route Manifest secrets into the vault and handle reload/restart behavior.
- Create `internal/server/manifest_handlers.go`: status, test, start, stop, and setup-readiness endpoints.
- Create `internal/server/manifest_handlers_test.go`: handler tests using httptest and mocked health/lifecycle functions.
- Modify `internal/llm/client.go`: recognize `manifest` in provider mismatch checks while still using OpenAI-compatible behavior.
- Create or modify `internal/llm/client_test.go`: provider URL mismatch and Manifest provider behavior tests.
- Modify `internal/tsnetnode/tsnetnode.go`: optional Manifest HTTPS reverse proxy on the existing tsnet node.
- Create or modify `internal/tsnetnode/tsnetnode_test.go`: target URL and status tests for Manifest exposure helpers.
- Modify `config_template.yaml`: add `manifest:` section, provider example, and tsnet exposure fields.
- Create `ui/cfg/manifest.js`: Manifest config panel.
- Modify `ui/js/config/main.js`: add section metadata, module mapping, and asset version bump.
- Modify `ui/cfg/tailscale.js`: add Manifest exposure controls and status display.
- Modify `ui/config_help.json`: add Manifest help text.
- Modify all `ui/lang/config/sections/*.json`: add Manifest section/config/help strings for `cs`, `da`, `de`, `el`, `en`, `es`, `fr`, `hi`, `it`, `ja`, `nl`, `no`, `pl`, `pt`, `sv`, `zh`.
- Modify all `ui/lang/config/*.json` if section group labels or navigation keys are needed there.
- Modify `ui/chat_regression_test.go` and/or create `ui/manifest_config_regression_test.go`: static UI/i18n coverage for the new config module.
- Create `documentation/manifest.md`: user-facing setup, admin creation, API key, provider selection, backup/upgrade guidance.
- Create `prompts/tools_manuals/manifest.md`: agent-facing explanation of when to use the Manifest provider and what the integration can/cannot do.

## Phase 0: Verify External Assumptions

### Task 1: Capture The Manifest Runtime Contract

**Files:**

- Create: `reports/manifest_runtime_check_2026-05-05.md`

- [ ] **Step 1: Create a short non-versioned runtime note**

Use `reports/` because runtime validation notes are intentionally not committed.

Record:

- Image default: `manifestdotbuild/manifest:5`
- Internal port: `2099`
- Postgres image: `postgres:15-alpine`
- Required env: `DATABASE_URL`, `BETTER_AUTH_SECRET`, `BETTER_AUTH_URL`, `PORT`, `MANIFEST_TELEMETRY_DISABLED`
- Health endpoint result: `/health`, `/api/health`, or TCP-only fallback
- First-admin/API-key flow observed in the UI

- [ ] **Step 2: If Docker is available, run a disposable smoke check**

```powershell
if (-not (Test-Path disposable)) { New-Item -ItemType Directory disposable | Out-Null }
docker pull manifestdotbuild/manifest:5
docker pull postgres:15-alpine
```

Expected: both pulls complete. If Docker is unavailable, record that the runtime smoke check was skipped and continue with unit-testable integration work.

- [ ] **Step 3: Commit**

No commit for the report note because `reports/` is gitignored. Continue to Phase 1.

## Phase 1: Config, Secrets, And Provider Model

### Task 2: Add Manifest Config Defaults

**Files:**

- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `config_template.yaml`

- [ ] **Step 1: Add `ManifestConfig`**

Add a config struct with these fields:

```go
type ManifestConfig struct {
	Enabled               bool   `yaml:"enabled" json:"enabled"`
	AutoStart             bool   `yaml:"auto_start" json:"auto_start"`
	Mode                  string `yaml:"mode" json:"mode"` // managed or external
	URL                   string `yaml:"url" json:"url"`
	ExternalBaseURL       string `yaml:"external_base_url" json:"external_base_url"`
	ContainerName         string `yaml:"container_name" json:"container_name"`
	Image                 string `yaml:"image" json:"image"`
	Host                  string `yaml:"host" json:"host"`
	Port                  int    `yaml:"port" json:"port"`
	HostPort              int    `yaml:"host_port" json:"host_port"`
	NetworkName           string `yaml:"network_name" json:"network_name"`
	PostgresContainerName string `yaml:"postgres_container_name" json:"postgres_container_name"`
	PostgresImage         string `yaml:"postgres_image" json:"postgres_image"`
	PostgresUser          string `yaml:"postgres_user" json:"postgres_user"`
	PostgresDatabase      string `yaml:"postgres_database" json:"postgres_database"`
	PostgresVolume        string `yaml:"postgres_volume" json:"postgres_volume"`
	PostgresPassword      string `yaml:"-" json:"postgres_password,omitempty"`
	BetterAuthSecret      string `yaml:"-" json:"better_auth_secret,omitempty"`
	APIKey                string `yaml:"-" json:"api_key,omitempty"`
	HealthPath            string `yaml:"health_path" json:"health_path"`
}
```

Add `Manifest ManifestConfig` to `Config`.

- [ ] **Step 2: Add defaults**

Defaults:

- `enabled: false`
- `auto_start: true`
- `mode: managed`
- `url: defaultSidecarURL(runningInDocker, "manifest", 2099)`
- `external_base_url: https://app.manifest.build/v1`
- `container_name: aurago_manifest`
- `image: manifestdotbuild/manifest:5`
- `host: 127.0.0.1`
- `port: 2099`
- `host_port: 2099`
- `network_name: aurago_manifest`
- `postgres_container_name: aurago_manifest_postgres`
- `postgres_image: postgres:15-alpine`
- `postgres_user: manifest`
- `postgres_database: manifest`
- `postgres_volume: aurago_manifest_pgdata`
- `health_path: ""` so health detection can probe known candidates and fallback to TCP

- [ ] **Step 3: Add tests first**

Add tests that fail before the defaults are implemented:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/config -run "TestDefaultConfigManifest|TestManifestDefaultSidecarURL" -count=1
```

Expected before implementation: FAIL because the fields/defaults do not exist.

- [ ] **Step 4: Implement defaults and template**

Add a commented `manifest:` section to `config_template.yaml`. Keep secrets documented as vault-only; do not put example secrets into YAML.

- [ ] **Step 5: Verify focused tests**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/config -run "TestDefaultConfigManifest|TestManifestDefaultSidecarURL" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add internal/config/config_types.go internal/config/config.go internal/config/config_test.go config_template.yaml
git commit -m "Add Manifest integration configuration defaults"
```

### Task 3: Add Vault Secret Mapping And Provider Resolution

**Files:**

- Modify: `internal/config/config_migrate.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/server/config_handlers_main.go`
- Modify: `internal/server/config_handlers_test.go`

- [ ] **Step 1: Add tests for vault extraction**

Expected mappings:

- `manifest.api_key` -> `manifest_api_key`
- `manifest.postgres_password` -> `manifest_postgres_password`
- `manifest.better_auth_secret` -> `manifest_better_auth_secret`

Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/server -run TestExtractSecretsToVaultManifest -count=1
```

Expected before implementation: FAIL.

- [ ] **Step 2: Add provider type support**

Add `manifest` to `knownProviderTypes`.

In provider resolution, if a provider has `type: manifest`:

- If `base_url` is explicit, normalize it normally.
- If `base_url` is empty and `manifest.mode == managed`, use `manifest.url + "/v1"` with the correct in-Docker host.
- If `base_url` is empty and `manifest.mode == external`, use `manifest.external_base_url`.
- Keep runtime behavior OpenAI-compatible by mapping unsupported provider-specific handling to the existing custom/OpenAI-compatible path.

- [ ] **Step 3: Implement secret extraction**

Add the three vault mappings in `vaultKeyMap`. Ensure masked or empty values are removed from config patches and do not overwrite existing vault values.

- [ ] **Step 4: Add tests**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/config ./internal/server -run "TestManifestProvider|TestExtractSecretsToVaultManifest" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/config/config_migrate.go internal/config/config_test.go internal/server/config_handlers_main.go internal/server/config_handlers_test.go
git commit -m "Wire Manifest secrets and provider resolution"
```

## Phase 2: Managed Sidecar Runtime

### Task 4: Add Manifest Sidecar Config Helpers

**Files:**

- Create: `internal/tools/manifest.go`
- Create: `internal/tools/manifest_test.go`

- [ ] **Step 1: Write helper tests first**

Cover:

- Managed URL host is `manifest` inside Docker and `127.0.0.1` on bare metal.
- Postgres URL uses the Docker alias, not a host-published port.
- Missing `postgres_password` or `better_auth_secret` blocks managed startup.
- `BETTER_AUTH_URL` uses the browser-facing local URL, not the internal network alias.
- The Docker-in-Docker path attaches to AuraGo's current Docker network.
- The bare-metal path uses a private `aurago_manifest` network.

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/tools -run TestManifest -count=1
```

Expected before implementation: FAIL.

- [ ] **Step 2: Implement pure helpers**

Implement:

- `ManifestSidecarConfig`
- `ResolveManifestSidecarConfig(cfg *config.Config) (ManifestSidecarConfig, error)`
- `ManifestManagedURLHost(runningInDocker bool) string`
- `ManifestProviderBaseURL(cfg *config.Config) string`
- `manifestDatabaseURL(sidecar ManifestSidecarConfig) string`
- Docker container create payload builders for Manifest and Postgres

Keep helper behavior testable without a Docker daemon.

- [ ] **Step 3: Verify helper tests**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/tools -run TestManifest -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add internal/tools/manifest.go internal/tools/manifest_test.go
git commit -m "Add Manifest sidecar configuration helpers"
```

### Task 5: Implement Docker Lifecycle For Manifest And Postgres

**Files:**

- Modify: `internal/tools/manifest.go`
- Modify: `internal/tools/manifest_test.go`

- [ ] **Step 1: Add tests around Docker payloads**

Test payloads rather than requiring a live Docker daemon:

- Postgres has no `PortBindings`.
- Postgres has a named volume mounted at `/var/lib/postgresql/data`.
- Manifest exposes only configured `host:host_port -> port`.
- Manifest env contains `MANIFEST_TELEMETRY_DISABLED=1`.
- Manifest depends on a Postgres alias in its `DATABASE_URL`.

- [ ] **Step 2: Implement lifecycle functions**

Implement:

- `EnsureManifestSidecarsRunning(ctx, dockerHost string, cfg *config.Config, logger *slog.Logger) error`
- `StopManifestSidecars(ctx, dockerHost string, cfg *config.Config, logger *slog.Logger) error`
- `ManifestSidecarStatus(ctx, cfg *config.Config) (ManifestStatus, error)`

Lifecycle behavior:

- If `manifest.enabled` is false, do nothing.
- If `manifest.mode != managed`, do not manage containers.
- If required secrets are missing, return a clear setup error.
- Ensure/create the Docker network before creating containers.
- Start Postgres first, then Manifest.
- Pull missing images only when Docker returns image-not-found.
- Never delete the Postgres volume in normal stop/recreate paths.
- Reuse existing Docker helpers from `internal/tools` where possible.

- [ ] **Step 3: Verify unit tests**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/tools -run TestManifest -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add internal/tools/manifest.go internal/tools/manifest_test.go
git commit -m "Manage Manifest Docker sidecars"
```

### Task 6: Add Manifest Health And Server Control Endpoints

**Files:**

- Create: `internal/server/manifest_handlers.go`
- Create: `internal/server/manifest_handlers_test.go`
- Modify: `internal/server/server.go`
- Modify: route registration file used by nearby sidecar handlers

- [ ] **Step 1: Write handler tests first**

Endpoints:

- `GET /api/manifest/status`
- `POST /api/manifest/test`
- `POST /api/manifest/start`
- `POST /api/manifest/stop`

Expected payload shape:

```json
{
  "enabled": true,
  "mode": "managed",
  "status": "running",
  "url": "http://127.0.0.1:2099",
  "provider_base_url": "http://manifest:2099/v1",
  "admin_setup_required": true,
  "message": "..."
}
```

Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/server -run TestManifestHandlers -count=1
```

Expected before implementation: FAIL.

- [ ] **Step 2: Implement health probing**

Health probe order:

1. `manifest.health_path` if configured.
2. `GET /health`
3. `GET /api/health`
4. TCP connection to host/port as a degraded but useful fallback.

Do not fail the whole config page if HTTP health is unknown but TCP works; return a status message that says HTTP health endpoint was not confirmed.

- [ ] **Step 3: Wire startup**

In server startup, after config and Docker host are available, call `EnsureManifestSidecarsRunning` when:

- `manifest.enabled == true`
- `manifest.auto_start == true`
- `manifest.mode == managed`

Startup errors should be logged and exposed in status, but should not prevent AuraGo from booting.

- [ ] **Step 4: Verify focused server tests**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/server -run TestManifestHandlers -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/server/manifest_handlers.go internal/server/manifest_handlers_test.go internal/server/server.go
git commit -m "Add Manifest server control endpoints"
```

## Phase 3: LLM Provider And Tailscale Exposure

### Task 7: Finish Manifest Provider Integration

**Files:**

- Modify: `internal/llm/client.go`
- Create or modify: `internal/llm/client_test.go`
- Modify: `config_template.yaml`

- [ ] **Step 1: Add tests**

Cover:

- `type: manifest` with managed mode resolves to the managed `/v1` URL.
- `type: manifest` with external mode resolves to `external_base_url`.
- `detectProviderURLMismatch` does not warn for a Manifest provider using a local URL.
- The provider can still use OpenAI-compatible request formatting.

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/config ./internal/llm -run TestManifestProvider -count=1
```

Expected before implementation: FAIL.

- [ ] **Step 2: Implement minimal provider aliasing**

Keep `manifest` as an AuraGo type for UI and config clarity, but delegate runtime calls through the same path as `custom` OpenAI-compatible providers.

Add a provider example to `config_template.yaml`:

```yaml
# - id: manifest
#   type: manifest
#   name: "Manifest Gateway"
#   api_key: "" # store mnfst_... in the vault via UI
#   model: "manifest/auto"
```

- [ ] **Step 3: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/config ./internal/llm -run TestManifestProvider -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add internal/llm/client.go internal/llm/client_test.go internal/config/config_migrate.go internal/config/config_test.go config_template.yaml
git commit -m "Add Manifest as an OpenAI-compatible provider"
```

### Task 8: Add Optional Tsnet Manifest Exposure

**Files:**

- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config.go`
- Modify: `internal/tsnetnode/tsnetnode.go`
- Create or modify: `internal/tsnetnode/tsnetnode_test.go`
- Modify: `ui/cfg/tailscale.js`
- Modify: `config_template.yaml`

- [ ] **Step 1: Add tests for target calculation**

Add helpers if needed so this is testable without starting tsnet:

- Manifest target is `http://127.0.0.1:2099` for bare metal.
- Manifest target is `http://manifest:2099` when AuraGo is running in Docker.
- Status includes `manifest_serving`, `manifest_url`, and `expose_manifest`.

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/tsnetnode -run TestManifestExposure -count=1
```

Expected before implementation: FAIL.

- [ ] **Step 2: Add config fields**

Extend `TsNetConfig`:

```go
ExposeManifest   bool   `yaml:"expose_manifest" json:"expose_manifest"`
ManifestHostname string `yaml:"manifest_hostname" json:"manifest_hostname"`
ManifestPort     int    `yaml:"manifest_port" json:"manifest_port"`
```

Defaults:

- `expose_manifest: false`
- `manifest_hostname: aurago-manifest`
- `manifest_port: 8444`

- [ ] **Step 3: Implement reverse proxy**

Mirror the existing Homepage/Space Agent pattern:

- Add listener/server fields to `Manager`.
- Add `startManifestListener` and `stopManifestListener`.
- Use `httputil.NewSingleHostReverseProxy`.
- Set `X-Forwarded-Host`, `X-Forwarded-Proto`, and `X-Forwarded-For`.
- Only expose when tsnet is enabled, Manifest integration is enabled, and `expose_manifest` is true.

- [ ] **Step 4: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/tsnetnode -run TestManifestExposure -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/config/config_types.go internal/config/config.go internal/tsnetnode/tsnetnode.go internal/tsnetnode/tsnetnode_test.go ui/cfg/tailscale.js config_template.yaml
git commit -m "Expose Manifest through the tsnet node"
```

## Phase 4: Configuration UI And Documentation

### Task 9: Add Manifest Config Panel

**Files:**

- Create: `ui/cfg/manifest.js`
- Modify: `ui/js/config/main.js`
- Create or modify: `ui/manifest_config_regression_test.go`
- Modify: `ui/config_help.json`

- [ ] **Step 1: Add UI regression tests first**

Test for static markers:

- `manifest` exists in `SECTIONS`.
- `manifest: { m: 'manifest', fn: 'renderManifestSection' }` exists in `SECTION_MODULES`.
- `cfg/manifest.js` includes no `alert(`.
- UI has buttons for test, start, and stop.
- UI uses vault-safe secret inputs for API key, Postgres password, and Better Auth secret.

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestManifestConfig -count=1
```

Expected before implementation: FAIL.

- [ ] **Step 2: Implement `ui/cfg/manifest.js`**

Panel requirements:

- Integration toggle.
- Mode dropdown: `managed` / `external`.
- Managed fields: image, host, host port, container name, Postgres image, Postgres volume.
- External fields: external base URL.
- Vault fields: API key, Postgres password, Better Auth secret.
- Buttons: test connection, start sidecars, stop sidecars.
- Status area: running/starting/stopped/error, dashboard URL, provider base URL, admin setup guidance.
- No `alert()`.
- Use existing `btn-save`, `dc-test-result`, `password-wrap`, and config field patterns.

- [ ] **Step 3: Register the section**

In `ui/js/config/main.js`:

- Add Manifest next to other agent/provider integrations.
- Add module mapping.
- Bump `CONFIG_ASSET_VERSION`.

- [ ] **Step 4: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestManifestConfig -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add ui/cfg/manifest.js ui/js/config/main.js ui/manifest_config_regression_test.go ui/config_help.json
git commit -m "Add Manifest configuration UI"
```

### Task 10: Add All Translations

**Files:**

- Modify: `ui/lang/config/sections/cs.json`
- Modify: `ui/lang/config/sections/da.json`
- Modify: `ui/lang/config/sections/de.json`
- Modify: `ui/lang/config/sections/el.json`
- Modify: `ui/lang/config/sections/en.json`
- Modify: `ui/lang/config/sections/es.json`
- Modify: `ui/lang/config/sections/fr.json`
- Modify: `ui/lang/config/sections/hi.json`
- Modify: `ui/lang/config/sections/it.json`
- Modify: `ui/lang/config/sections/ja.json`
- Modify: `ui/lang/config/sections/nl.json`
- Modify: `ui/lang/config/sections/no.json`
- Modify: `ui/lang/config/sections/pl.json`
- Modify: `ui/lang/config/sections/pt.json`
- Modify: `ui/lang/config/sections/sv.json`
- Modify: `ui/lang/config/sections/zh.json`
- Modify: `ui/i18n_lint_test.go` only if a new allowlist exception is truly needed.

- [ ] **Step 1: Add translation keys in all languages**

Required key families:

- `config.section.manifest.label`
- `config.section.manifest.desc`
- `config.manifest.*`
- `help.manifest.*`
- `config.tailscale.tsnet_expose_manifest_*`
- `config.tailscale.tsnet_manifest_*`

Do not copy English into every file. Provide concise localized strings.

- [ ] **Step 2: Run i18n tests**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run "TestI18n|TestManifestConfig" -count=1
```

Expected: PASS.

- [ ] **Step 3: Commit**

```powershell
git add ui/lang/config/sections/*.json ui/i18n_lint_test.go
git commit -m "Localize Manifest configuration"
```

### Task 11: Add User And Agent Documentation

**Files:**

- Create: `documentation/manifest.md`
- Create: `prompts/tools_manuals/manifest.md`
- Modify: `README.md` only if the integrations list needs a short mention.

- [ ] **Step 1: Write docs**

`documentation/manifest.md` must cover:

- Managed vs external mode.
- First admin account creation.
- API key format `mnfst_...`.
- Vault storage.
- Provider setup and model selection.
- Docker network and Postgres persistence.
- Backup command for the Postgres volume.
- Upgrade guidance for `manifestdotbuild/manifest:5`.
- Optional Tailscale exposure.
- Security warning: do not expose the dashboard/API publicly without auth and HTTPS.

`prompts/tools_manuals/manifest.md` must cover:

- What Manifest is used for in AuraGo.
- How the agent should select the Manifest provider.
- What the agent cannot do automatically: first admin creation and API key creation unless credentials/API are already available.
- Troubleshooting signals from `/api/manifest/status`.

- [ ] **Step 2: Verify docs are embedded or indexed by existing conventions**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui ./internal/server -run "Test.*Embed|Test.*Manual|TestManifest" -count=1
```

Expected: PASS or no matching tests where appropriate.

- [ ] **Step 3: Commit**

```powershell
git add documentation/manifest.md prompts/tools_manuals/manifest.md README.md
git commit -m "Document Manifest integration"
```

## Phase 5: Integration Verification

### Task 12: Full Verification And Cleanup

**Files:**

- No planned source edits unless tests reveal a gap.

- [ ] **Step 1: Run focused packages**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/config ./internal/tools ./internal/server ./internal/llm ./internal/tsnetnode ./ui -count=1
```

Expected: PASS.

- [ ] **Step 2: Run broader regression tests if time allows**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./... -count=1
```

Expected: PASS. If unrelated existing failures appear, record them with package, test name, and first failing assertion.

- [ ] **Step 3: Check formatting and accidental secret leakage**

```powershell
gofmt -w internal/config/config_types.go internal/config/config.go internal/config/config_migrate.go internal/tools/manifest.go internal/tools/manifest_test.go internal/server/manifest_handlers.go internal/server/manifest_handlers_test.go internal/llm/client.go internal/tsnetnode/tsnetnode.go internal/tsnetnode/tsnetnode_test.go
git diff --check
rg -n "mnfst_|BETTER_AUTH_SECRET=.*[A-Za-z0-9]{16,}|POSTGRES_PASSWORD=.*[A-Za-z0-9]{8,}|sk-or-" --glob '!reports/**' --glob '!disposable/**'
```

Expected:

- `git diff --check` prints no errors.
- `rg` finds no real secrets. Placeholder strings in documentation are acceptable only if clearly marked as placeholders.

- [ ] **Step 4: Inspect dirty worktree carefully**

```powershell
git status --short
git diff --stat
```

Expected: only intended Manifest files plus any unrelated pre-existing local files. Do not stage unrelated files such as `ui/css/radio.css`, `ui/js/desktop/apps/radio.js`, or `.playwright-mcp/`.

- [ ] **Step 5: Final commit if any verification fixes were needed**

```powershell
git add <only-intended-files>
git commit -m "Harden Manifest integration verification"
```

Expected: commit created only if Phase 5 changed files.

## Acceptance Criteria

- AuraGo can be configured with `manifest.enabled: true` without storing secrets in `config.yaml`.
- Managed mode starts Manifest and Postgres with a persistent Postgres volume and no published Postgres port.
- Status/test endpoints clearly report missing secrets, stopped containers, running containers, dashboard URL, and provider base URL.
- The Manifest provider can be selected from AuraGo's provider system and sends requests through the existing OpenAI-compatible path.
- The config UI exposes all important controls, never uses `alert()`, and has complete translations for all 15 supported languages.
- Optional tsnet exposure uses the existing AuraGo tsnet node and reports its serving status.
- Documentation explains the unavoidable first-admin/API-key manual step.
- Focused tests pass for `internal/config`, `internal/tools`, `internal/server`, `internal/llm`, `internal/tsnetnode`, and `ui`.
