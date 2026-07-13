# Managed Boring Computers Web UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Project policy forbids subagents unless the user explicitly requests them.

**Goal:** Install and privately serve the upstream Boring Computers management application automatically, then expose it from the enabled Virtual Computers integration through AuraGo's right-hand chat drawer.

**Architecture:** Extend the existing Virtual Computers setup manager with a pinned, loopback-only `apps/web` service on port `18081`. AuraGo proxies `/boring-computers/` through its authenticated server, creates a second managed SSH tunnel in remote mode, and advertises the route through the existing integration-webhosts API only while Virtual Computers is enabled.

**Tech Stack:** Go 1.26.1+, `net/http`, `net/http/httputil`, `golang.org/x/crypto/ssh`, systemd, Node.js/npm, SvelteKit/Vite, Go table-driven tests.

## Global Constraints

- `boringd` and the management service remain loopback-only; no direct network exposure.
- `BORING_TOKEN` stays in the Vault or root-readable service environment and never reaches browser assets, logs, Python tools, or API configuration responses.
- The managed control-plane default remains `http://127.0.0.1:18080`; the management service uses `http://127.0.0.1:18081` and public base `/boring-computers`.
- Both `local_host` and `ssh_host` modes must work without user-managed services, ports, containers, or tunnels.
- Existing `/api/virtual-computers/*`, `/virtual-computers`, Chat, and AgoDesk payloads remain backward compatible.
- All implementation changes require pre-edit GitNexus impact analysis; HIGH or CRITICAL findings must be reported before editing.
- Use TDD: each production behavior is preceded by a failing test and an observed expected failure.
- Before each commit run GitNexus `detect-changes`; final completion requires fresh targeted tests, `go test ./...`, and a build.

---

## File Structure

- `internal/virtualcomputers/management.go`: reviewed upstream revision, management endpoint/base-path constants, component status types, and health helpers.
- `internal/virtualcomputers/setup_web.go`: deterministic shell fragment that installs, overlays, builds, stages, and starts the upstream web application.
- `internal/virtualcomputers/setup.go`: composes the existing boringd setup with the new web setup and verifies both components.
- `internal/virtualcomputers/types.go`: additive setup component status fields.
- `internal/server/virtual_computers_management.go`: local/SSH management reachability, tunnel lifecycle, and authenticated HTTP/WebSocket reverse proxy.
- `internal/server/virtual_computers_handlers.go`: route registration and additive management status payload.
- `internal/server/space_agent_handlers.go`: conditional Boring Computers drawer entry.
- Existing adjacent `*_test.go` files: behavior and regression coverage.
- `prompts/tools_manuals/virtual_computers.md` and `documentation/virtual_computers.md` when present: managed web UI behavior and troubleshooting.

### Task 1: Chat Drawer Contract

**Files:**
- Modify: `internal/server/space_agent_handlers_test.go`
- Modify: `internal/server/space_agent_handlers.go:403-539`
- Test: `internal/server/space_agent_handlers_test.go`

**Interfaces:**
- Consumes: `config.Config.VirtualComputers.Enabled`, existing `webhostIntegration`.
- Produces: webhost ID `boring_computers` with URL `/boring-computers/`.

- [ ] **Step 1: Add failing enabled/disabled drawer tests**

```go
func TestHandleIntegrationWebhostsIncludesEnabledBoringComputers(t *testing.T) {
	cfg := &config.Config{}
	cfg.VirtualComputers.Enabled = true
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodGet, "/api/integrations/webhosts", nil)
	rec := httptest.NewRecorder()
	clearWebhostsCache()
	handleIntegrationWebhosts(s).ServeHTTP(rec, req)
	var payload struct { Webhosts []webhostIntegration `json:"webhosts"` }
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil { t.Fatal(err) }
	if len(payload.Webhosts) != 1 || payload.Webhosts[0].ID != "boring_computers" || payload.Webhosts[0].URL != "/boring-computers/" {
		t.Fatalf("webhosts = %#v", payload.Webhosts)
	}
}

func TestHandleIntegrationWebhostsOmitsDisabledBoringComputers(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	clearWebhostsCache()
	got := integrationWebhostsForRequest(s, httptest.NewRequest(http.MethodGet, "/api/integrations/webhosts", nil))
	if len(got) != 0 { t.Fatalf("webhosts = %#v", got) }
}
```

- [ ] **Step 2: Run the focused tests and observe the enabled test fail**

Run: `go test ./internal/server -run 'TestHandleIntegrationWebhosts(IncludesEnabled|OmitsDisabled)BoringComputers' -count=1`

Expected: FAIL because `boring_computers` is absent.

- [ ] **Step 3: Add the minimal conditional entry**

```go
if cfg.VirtualComputers.Enabled {
	webhosts = append(webhosts, webhostIntegration{
		ID: "boring_computers", Name: "Boring Computers",
		Description: "Managed virtual computer control center",
		Status: "starting", URL: "/boring-computers/", Icon: "terminal",
	})
}
```

- [ ] **Step 4: Run focused and existing webhost tests**

Run: `go test ./internal/server -run 'TestHandleIntegrationWebhosts' -count=1`

Expected: PASS.

- [ ] **Step 5: Run `detect-changes`, stage only Task 1, and commit**

```bash
node .gitnexus/run.cjs detect-changes --repo C:/Users/Andi/Documents/repo/AuraGo --scope all
git add internal/server/space_agent_handlers.go internal/server/space_agent_handlers_test.go
git commit -m "feat: add Boring Computers to chat integrations"
```

### Task 2: Management Endpoint and Component Status Model

**Files:**
- Create: `internal/virtualcomputers/management.go`
- Modify: `internal/virtualcomputers/types.go:114-139`
- Create: `internal/virtualcomputers/management_test.go`
- Modify: `internal/virtualcomputers/setup_test.go`

**Interfaces:**
- Produces: `ManagementBasePath`, `ManagementListenAddr`, `ManagementURL`, `PinnedUpstreamRevision`, `ComponentStatus`, and `ManagementHealthURL(string) string`.
- Produces additive `SetupStatus.ControlPlane` and `SetupStatus.Management` JSON fields.

- [ ] **Step 1: Add failing constant, URL, and JSON compatibility tests**

```go
func TestManagementContract(t *testing.T) {
	if ManagementBasePath != "/boring-computers" { t.Fatalf("base = %q", ManagementBasePath) }
	if ManagementURL != "http://127.0.0.1:18081" { t.Fatalf("url = %q", ManagementURL) }
	if got := ManagementHealthURL(ManagementURL); got != "http://127.0.0.1:18081/boring-computers/" { t.Fatalf("health = %q", got) }
}
```

- [ ] **Step 2: Run and observe undefined-symbol failure**

Run: `go test ./internal/virtualcomputers -run 'TestManagementContract|TestSetupStatus' -count=1`

Expected: build FAIL for missing management contract symbols.

- [ ] **Step 3: Implement the focused model**

```go
const (
	ManagementBasePath = "/boring-computers"
	ManagementListenAddr = "127.0.0.1:18081"
	ManagementURL = "http://" + ManagementListenAddr
	PinnedUpstreamRevision = "9752ac7e4d902e425ab0f4047a975ea5bfba7579"
)

type ComponentStatus struct {
	Configured bool `json:"configured"`
	Healthy bool `json:"healthy"`
	Message string `json:"message,omitempty"`
}
```

Extend `SetupStatus` with `ControlPlane ComponentStatus` and `Management ComponentStatus` using `omitempty`-compatible additive JSON fields.

- [ ] **Step 4: Run package tests**

Run: `go test ./internal/virtualcomputers -count=1`

Expected: PASS.

- [ ] **Step 5: Detect, stage, and commit**

```bash
node .gitnexus/run.cjs detect-changes --repo C:/Users/Andi/Documents/repo/AuraGo --scope all
git add internal/virtualcomputers/management.go internal/virtualcomputers/management_test.go internal/virtualcomputers/types.go internal/virtualcomputers/setup_test.go
git commit -m "feat: define managed Boring Computers endpoint"
```

### Task 3: Idempotent Upstream Web Installer

**Files:**
- Create: `internal/virtualcomputers/setup_web.go`
- Create: `internal/virtualcomputers/setup_web_test.go`
- Modify: `internal/virtualcomputers/setup.go:96-298`
- Modify: `internal/virtualcomputers/setup_test.go`

**Interfaces:**
- Consumes: `SetupInstallOptions`, `PinnedUpstreamRevision`, management constants.
- Produces: `managementInstallScript(opts SetupInstallOptions) string` and dual-component `SetupStatus`.

- [ ] **Step 1: Add failing script-contract tests**

Assert the generated script contains the pinned commit, Node/npm installation, `npm ci`, production build, compatibility-overlay validation, `/etc/boring/boring-web.env` with mode `0600`, `boring-web.service`, loopback `18081`, `systemctl enable`, and separate management health check. Assert it never writes `PUBLIC_BORING_URL` with the private URL.

- [ ] **Step 2: Run and observe missing installer failure**

Run: `go test ./internal/virtualcomputers -run 'TestManagementInstallScript|TestSetupInstall' -count=1`

Expected: build FAIL or assertion FAIL because the web installer is absent.

- [ ] **Step 3: Implement `managementInstallScript`**

The script must stage in `${INSTALL_DIR}/releases/${PinnedUpstreamRevision}`, verify the upstream file hashes expected by the overlay, create an AuraGo Vite/Svelte compatibility config for base `/boring-computers` and preview proxy `/boring-computers/boring`, patch `src/lib/boring.ts` to use `$app/paths.base`, run locked dependency install and build, then atomically update `${INSTALL_DIR}/current` only after success.

The systemd unit must use:

```ini
[Service]
EnvironmentFile=/etc/boring/boring-web.env
WorkingDirectory=/opt/boring/current/apps/web
ExecStart=/usr/bin/npm exec vite -- preview --host 127.0.0.1 --port 18081 --strictPort
Restart=on-failure
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
```

- [ ] **Step 4: Compose installation and return dual health state**

Append the web fragment after boringd is configured, restart both services, and probe `boringdHealthURL` plus `ManagementHealthURL`. Populate both `ComponentStatus` values; top-level `Healthy` is their conjunction.

- [ ] **Step 5: Run package tests and secret scans**

Run: `go test ./internal/virtualcomputers -count=1`

Run: `rg -n 'BORING_TOKEN=.*[^<]redacted|PUBLIC_BORING_URL=http' internal/virtualcomputers`

Expected: tests PASS; scan finds no browser-visible token/private URL assignment.

- [ ] **Step 6: Detect, stage, and commit**

```bash
node .gitnexus/run.cjs detect-changes --repo C:/Users/Andi/Documents/repo/AuraGo --scope all
git add internal/virtualcomputers/setup.go internal/virtualcomputers/setup_test.go internal/virtualcomputers/setup_web.go internal/virtualcomputers/setup_web_test.go
git commit -m "feat: install Boring Computers management service"
```

### Task 4: Local and SSH Management Access

**Files:**
- Create: `internal/server/virtual_computers_management.go`
- Create: `internal/server/virtual_computers_management_test.go`
- Modify: `internal/server/virtual_computers_handlers.go:670-840`

**Interfaces:**
- Produces: `virtualComputersEnsureManagementAccess(*Server, virtualcomputers.ToolConfig) error`, `virtualComputersManagementHealthy(*Server, virtualcomputers.ToolConfig) bool`, and cleanup-aware tunnel state.
- Consumes existing `virtualComputersSSHSetupExecutor`, `startVirtualComputersSSHTunnel`, and control-plane mode helpers.

- [ ] **Step 1: Add failing local, reuse, replacement, and cleanup tests**

Use loopback `httptest.Server` for local health and a fake SSH/tunnel factory injected through package variables for deterministic remote lifecycle assertions. Verify a partial tunnel failure closes any newly created listener.

- [ ] **Step 2: Run and observe missing access helper failure**

Run: `go test ./internal/server -run 'TestVirtualComputersManagement' -count=1`

Expected: build FAIL for missing helpers.

- [ ] **Step 3: Implement access and health helpers**

Local mode probes the management URL directly. SSH mode establishes a separately keyed loopback tunnel for `127.0.0.1:18081`, guarded by a mutex, and reuses it only after a successful health probe. Errors returned to HTTP callers are safe summaries; detailed errors go only to structured server logs.

- [ ] **Step 4: Run focused tests with race detection**

Run: `go test -race ./internal/server -run 'TestVirtualComputersManagement' -count=1`

Expected: PASS with no race reports.

- [ ] **Step 5: Detect, stage, and commit**

```bash
node .gitnexus/run.cjs detect-changes --repo C:/Users/Andi/Documents/repo/AuraGo --scope all
git add internal/server/virtual_computers_management.go internal/server/virtual_computers_management_test.go internal/server/virtual_computers_handlers.go
git commit -m "feat: manage Boring Computers web access"
```

### Task 5: Authenticated Management Proxy and Status

**Files:**
- Modify: `internal/server/virtual_computers_management.go`
- Modify: `internal/server/virtual_computers_management_test.go`
- Modify: `internal/server/virtual_computers_handlers.go:58-225`
- Modify: `internal/server/virtual_computers_handlers_test.go`
- Modify: `internal/server/space_agent_handlers.go`

**Interfaces:**
- Produces: `handleVirtualComputersManagement(*Server) http.HandlerFunc` registered for `/boring-computers/` and redirect from `/boring-computers`.
- Adds `management` to setup/status JSON without changing existing keys.

- [ ] **Step 1: Add failing disabled, HTTP rewrite, WebSocket, and status tests**

Verify disabled access returns `404`, unavailable enabled access returns safe `503`, enabled HTTP requests preserve `/boring-computers/...`, WebSocket upgrade data is relayed, and setup/status responses include separate `control_plane` and `management` states without secrets.

- [ ] **Step 2: Run and observe expected failures**

Run: `go test ./internal/server -run 'TestVirtualComputersManagementProxy|TestVirtualComputersSetupStatus' -count=1`

Expected: FAIL because routes and additive status are absent.

- [ ] **Step 3: Implement and register the reverse proxy**

```go
mux.HandleFunc(virtualcomputers.ManagementBasePath, redirectToManagementSlash)
mux.HandleFunc(virtualcomputers.ManagementBasePath+"/", handleVirtualComputersManagement(s))
```

The handler checks `cfg.Enabled`, calls `virtualComputersEnsureManagementAccess`, and uses `httputil.ReverseProxy` with the loopback management origin. Preserve the full base-prefixed path and forwarded host/proto. Do not add CORS headers or browser authorization headers.

- [ ] **Step 4: Make drawer status health-aware**

Use `virtualComputersManagementHealthy(s, cfg)` to emit `running`; otherwise keep `starting`. Rely on the existing ten-second webhost cache to bound probe frequency.

- [ ] **Step 5: Run focused server and UI contract tests**

Run: `go test ./internal/server -run 'TestVirtualComputers|TestHandleIntegrationWebhosts' -count=1`

Run: `go test ./ui -run 'TestChatFrontend_IntegrationsDrawer' -count=1`

Expected: PASS.

- [ ] **Step 6: Detect, stage, and commit**

```bash
node .gitnexus/run.cjs detect-changes --repo C:/Users/Andi/Documents/repo/AuraGo --scope all
git add internal/server/virtual_computers_management.go internal/server/virtual_computers_management_test.go internal/server/virtual_computers_handlers.go internal/server/virtual_computers_handlers_test.go internal/server/space_agent_handlers.go
git commit -m "feat: proxy Boring Computers management UI"
```

### Task 6: Documentation and Full Verification

**Files:**
- Modify: `prompts/tools_manuals/virtual_computers.md`
- Create: `documentation/virtual_computers.md`

**Interfaces:**
- Documents `/boring-computers/`, loopback ports, automatic installation, SSH tunneling, repair, and security boundaries.

- [ ] **Step 1: Update operator and agent documentation**

State that Install/Repair manages both services, the drawer link appears only when enabled, ports `18080` and `18081` stay private, and `503` should be diagnosed with setup status/repair rather than exposing either service.

- [ ] **Step 2: Run formatting and targeted verification**

Run: `gofmt -w internal/virtualcomputers/*.go internal/server/virtual_computers*.go internal/server/space_agent_handlers*.go`

Run: `go test ./internal/virtualcomputers ./internal/server ./ui -count=1`

Expected: PASS.

- [ ] **Step 3: Run full race-sensitive and repository verification**

Run: `go test -race ./internal/virtualcomputers ./internal/server -count=1`

Run: `go test ./... -count=1`

Run: `go build ./cmd/aurago`

Expected: all commands exit `0` with no test failures.

- [ ] **Step 4: Review requirements and secrets**

Run: `git diff --check`

Run: `git diff --cached`

Run: `rg -n 'AURAGO_MASTER_KEY|sk-or-|BORING_TOKEN=[^$]|password|secret' internal/virtualcomputers internal/server documentation prompts/tools_manuals`

Expected: no credential values; only intentional field names and documentation references.

- [ ] **Step 5: Run final GitNexus scope review and commit**

```bash
node .gitnexus/run.cjs detect-changes --repo C:/Users/Andi/Documents/repo/AuraGo --scope compare --base-ref main
git add prompts/tools_manuals/virtual_computers.md documentation/virtual_computers.md
git commit -m "docs: document managed Boring Computers UI"
```

- [ ] **Step 6: Confirm clean handoff**

Run: `git status --short`

Expected: no uncommitted implementation files; any pre-existing user changes are listed separately and explicitly preserved.
