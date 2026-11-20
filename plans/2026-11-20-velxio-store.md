# Plan: Velxio Store app with an AuraGo MCP connection

Worktree: `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\velxio` (branch `feat/velxio-store`, same as main 8f627e5a1). All paths below are relative to it. I wrote no files.

## 1. Summary

Velxio becomes a `dev` Store app:
- Image pinned by digest, web UI on port 80, opens in its own browser tab (`open_external`).
- Five named volumes, `VELXIO_NEWS=off`, no hardening opt-in.
- An entrypoint wrapper (the DeepTutor mechanism) starts Velxio's built-in FastMCP server on container port 8002. It is wrapped in a bearer-token check. The token is a generated Store secret kept only in the Vault.
- The MCP port is published as an extra port that is always pinned to `127.0.0.1`. It is never shown as an openable port and never put on the tailnet.
- AuraGo joins Velxio through the existing Connect-to-agent / managed MCP path as `aurago-store-velxio`:
  - the Read grant gives the 5 design tools;
  - the Execute grant adds `compile_project` and `run_project`;
  - the URL is always derived, and the user enters no URL and no credential.

The live probe results are folded in. The launcher text and the wrapper text are the verified ones.

Shared code changes are small and generic:
1. A catalog `PortSpec.HostIP` is now honoured for app ports (today it only works for companion ports).
2. Port IDs starting with `agent-` are excluded from `IsHTTPPort`, so they cannot be opened, previewed or put on Tailscale.
3. A Velxio-only "settle" retry for the connection check, because the MCP server comes up after the web port.

## 2. Live probe facts (2026-11-20, Docker Desktop 29.9.3, linux/amd64)

**Image**
- Entrypoint `null`, Cmd `["/app/entrypoint.sh"]`, user root, WorkingDir `/app`, about 7.15 GB unpacked.
- Python 3.12.16, mcp 1.30.0, starlette 0.38.6, uvicorn 0.32.0.
- FastMCP has both `streamable_http_app()` and `sse_app()`.
- FastMCP defaults: DNS-rebinding protection on, allowed hosts are localhost only, path `/mcp`.
- Trap: importing `app.mcp.server` runs `arduino-cli config init` and a core install at import time. The launcher therefore waits for `http://127.0.0.1:8001/health` first.

**Wrapper**
- Verified: the `( while … ) & unset …; exec /app/entrypoint.sh "$@"` form.
- PID 1 has no `AURAGO_*` variables; only the restart-loop subshell keeps the token.
- No token or a wrong token gets 401.
- With the right token and `Host: aurago-store-velxio:8002`, `initialize` returns 200.
- `tools/list` returns exactly 7 tools.
- `compile_project` (blink, `arduino:avr:uno`) succeeds, hex is 2556 bytes, about 1 s.
- Root sends `X-Frame-Options: SAMEORIGIN`.

**Timings**

| Case | Web `/health` 200 | MCP listening |
|---|---|---|
| First start, fresh volumes | 75 s | 79 s |
| Restart, existing volumes | 17 s | 24 s |

## 3. Architecture decisions

### D1 Catalog entry
- ID `velxio`, category `dev`, `Icon: "code"`.
  - `code` passes `desktop.NormalizeDesktopIconName`; `bytestash` and `code-server` use it too.
- `LogoSlug: "velxio"`, `LogoURL: ""`.
  - This is the Node64/DeepTutor pattern: no packaged logo; the card shows the theme icon (`storeEntryUsesThemeIcon`).
  - This avoids a remote logo fetch (no external assets) and any question about Velxio's logo trademark.
  - A dedicated icon (Ziro-style, copied into Papirus/WhiteSur) is an optional follow-up.
- `PrimaryPort {ID:"web", Name:"Web UI", ContainerPort:80, tcp}`.
- `ExtraPorts {ID:"agent-mcp", Name:"Agent MCP (loopback only)", ContainerPort:8002, tcp, HostIP:"127.0.0.1"}`.
- Five named volumes: `data` → `/app/data`, `arduino15` → `/root/.arduino15`, `arduino` → `/root/Arduino`, `ccache` → `/var/cache/ccache`, `build` → `/var/lib/velxio-build`.
  - These resolve to `aurago_store_velxio_<suffix>`. `delete_data` removes them; no anonymous volumes are left.
- `Env: ["VELXIO_NEWS=off"]`.
- `GeneratedSecrets: [{Key:"mcp_token", Env:"AURAGO_VELXIO_MCP_TOKEN", Label:"AuraGo MCP token"}]`, not exposed, so there is no credentials button.
- `Metadata {"open_external":"true"}`. No hardening: the image runs nginx, uvicorn, arduino-cli and QEMU as root.

### D2 MCP launcher
- Same mechanism as DeepTutor: `ContainerSpec.Entrypoint = ["/bin/sh","-c", velxioWrapper, "aurago-velxio"]`.
- The verified wrapper is used verbatim.
- At create time, `runtimeContainerSpec` adds:
  - `AURAGO_VELXIO_MCP_PY` = the embedded script (`//go:embed velxio_assets/aurago_velxio_mcp.py`);
  - `AURAGO_VELXIO_MCP_PORT=8002` from a Go constant, so Go is the single source of truth for the port.
- Because the image has no Entrypoint and its Cmd is dropped when an Entrypoint is set, `"$@"` is empty. The explicit `exec /app/entrypoint.sh "$@"` is correct; do not "fix" it.

### D3 Token decision: keep the bearer token
The token goes through the generic generated-secret path; no DeepTutor-style strip code is needed.

The code already does what we need:
- `resolveEnv` (service.go:2240) never writes generated values into `record.Env`.
- `saveInstalled` additionally strips them with `splitGeneratedEnv` (service.go:2747).
- `addGeneratedRuntimeEnv` (generated_env.go:40) injects `AURAGO_VELXIO_MCP_TOKEN` from the Vault at every create (install, update, rollback recreate). It skips only `IsRelayApp` apps.
- If the token is missing but referenced, the create fails closed ("missing Store Vault secret").

Reasons to keep the token:
1. FastMCP's Host allowlist has to be switched off so containerized AuraGo can use `aurago-store-velxio:8002`. The token replaces it, so a DNS-rebinding page or a neighbour container on the selected shared Docker network cannot call the agent endpoint.
2. It fits the managed-MCP contract: `storeMCPConfig` always sends `Authorization: Bearer …`, so there is no special no-auth branch.
3. It costs the user nothing: no input in Connect to agent; the server reads the token from the Vault.
4. If a later Velxio release adds stateful MCP tools, they start out gated.

Honest limit, to document: Velxio's own web API on port 80 is unauthenticated and can compile too. Anyone who can reach the UI already has compile capability, and root processes in the container can read the launcher's `/proc/<pid>/environ`. The token is visible in `docker inspect`. So it gates the agent endpoint; it is not a hard security boundary.

### D4 The agent port is loopback-only and never openable
Problem: `allocatePortBindings` (service.go:1922) gives every catalog port the app's bind IP. In LAN mode that would put the MCP port on `0.0.0.0`. The UI (`extraPortButtons`, software-store.js:315) and Tailscale (`desktopStoreTailscaleProxySpecs`, desktop_store_handlers.go:870, through `IsHTTPPort`) would also expose it.

Fix:
- `allocatePortBindings` and `updatePortBindings` use `port.HostIP` when the catalog sets it.
- `NewService` validation (service.go:150-163) accepts a set `HostIP` only if it is a loopback IP.
- `IsHTTPPort` (integration.go:51) returns false for IDs with the prefix `agent-`. This blocks `selectAppPort`, so open-url, preview targets (preview_targets.go:68 goes through `OpenURL`) and Tailscale.
- The UI filters `agent-*` ports out of the extra-port buttons.
- No existing port ID starts with `agent-` (existing IDs: web, main, hub, admin, docker-api, dns-tcp, dns-udp), and no other entry sets `HostIP` on app ports, so existing apps behave the same.

### D5 Agent connection mapping
- Managed name `aurago-store-velxio`, transport `streamable_http`, URL `<base>/mcp`.
- No new `IntegrationSettings` fields:
  - **Read** = enable + design tools: `create_circuit`, `update_circuit`, `import_wokwi_json`, `export_wokwi_json`, `generate_code_files`.
  - **Execute** = compile tools: `compile_project`, `run_project`.
- `AllowDestructive = Execute && !VirtualDesktop.ReadOnly`. This is required: `isMCPToolNameDestructive("run_project")` is true because of the word "run". In read-only Desktop mode the compile tools are withheld, like n8n's `execute_workflow`.
- URL resolution (always derived; any client-sent `url` and `credential` are ignored):
  - native AuraGo: `http://127.0.0.1:<published host port of agent-mcp>`;
  - containerized AuraGo: `http://aurago-store-velxio:8002`, which needs the selected shared Docker network (the existing `docker_network` select plus a Store Update);
  - remote Docker Engine: fails closed with the existing message, because the loopback port on a remote host cannot be reached.
- New helper `desktopstore.IntegrationPortID(appID, adminPort)` returns `"agent-mcp"` for Velxio.
- The capability test is the existing `probeStoreMCP`: initialize + `tools/list` only, and it checks that every allowlisted tool exists.
- Revocation on stop, update, uninstall, configure, config reload and shutdown is automatic once `storeMCPName("velxio")` is non-empty (`revokeStoreIntegration` → `revokeStoreMCP`).
- `delete_data` removes the integration Vault key (`deleteStoreIntegrationData`), the token (`deleteStoreSecrets` over SecretRefs) and the 5 volumes. A normal uninstall keeps all three.

### D6 Readiness and late MCP start
- Store readiness stays on the primary web port only, so a late MCP never fails an install.
- `appReadinessTimeoutFor` gets `velxioReadinessTimeout = 15 * time.Minute`:
  - Containerized AuraGo probes `aurago-store-velxio:80` directly, which needs at least 75 s on a fresh volume (the 60 s default fails).
  - Slow links slow down the core downloads.
  - The image pull plus first start must still fit the Store's 30-minute operation context (`desktopStoreOperationContext(..., 30*time.Minute)`). The docs advise `docker pull` beforehand on very slow links.
- Settle retry (server, Velxio only): after a restart, update, AuraGo startup or a Docker host reboot, the normal post-operation check (`refreshStoreIntegration`) runs before the MCP listens.
  - When that check fails, one background loop per app starts. It pre-probes `tools/list` every 10 s for up to 5 min, without taking the operation slot or `CfgSaveMu`.
  - Only after a successful pre-probe does it run the normal verified test and bind. Then it broadcasts a Store change.
  - It stops on shutdown, when the app is not running, when the settings are disabled, or when the integrations are closed.

### D7 Licence
AuraGo pulls the unmodified AGPL-3.0 image. The embedded launcher imports Velxio's modules, so its file gets `# SPDX-License-Identifier: AGPL-3.0-or-later` (open question Q1). Licence rows go into `licences/UEBERSICHT.md`.

## 4. Files

**Create**
- `internal/desktopstore/velxio.go`: constants, wrapper, embed, `velxioContainerSpec`, `VelxioMCPTokenKey()`.
- `internal/desktopstore/velxio_assets/aurago_velxio_mcp.py`: the verified launcher (Appendix A).
- `internal/desktopstore/velxio_test.go`
- `internal/server/desktop_store_velxio_test.go`
- `documentation/velxio.md`

**Modify**
- `internal/desktopstore/catalog.go`: new entry after `commandcode` (catalog.go:598-622), before `openscad` (:623).
- `internal/desktopstore/integration.go`: `IntegrationApps` (:37), `IsHTTPPort` (:51-53), new `IsAgentOnlyPort` and `IntegrationPortID`.
- `internal/desktopstore/service.go`: `NewService` ExtraPorts validation (:150-163), `allocatePortBindings` (:1922-1944), `updatePortBindings` (:1946-1977).
- `internal/desktopstore/gods_eye.go`: `runtimeContainerSpec` Velxio branch (:247-249).
- `internal/desktopstore/printguard.go`: `appReadinessTimeoutFor` (:319-327).
- `internal/desktopstore/service_test.go`: `TestDefaultCatalogContainsInitialApps` expected map (:30-56).
- `internal/server/desktop_store_mcp.go`: `storeMCPName`, `storeMCPConfig`, `probeStoreMCP`, new `storeMCPApps`, `syncStoreMCPs`, `velxioMCPToken`.
- `internal/server/desktop_store_integration.go`: `storeIntegrationRuntime`, `revokeStoreIntegrations` (:77-88), `refreshStoreIntegration` (:159-180), `storeIntegrationURL` (:192-219), `storeIntegrationStatus` (:232-238), `saveStoreIntegration` (:368, :417-431), `validateStoreIntegrationGrants` (:515), `testStoreIntegration` (:588), `stopStoreIntegrations` (:659-670), new settle functions.
- `internal/server/config_handlers_main.go`:1790-1791 (inside `handleUpdateConfig`) and `internal/server/mcp_handlers.go`:166-167 (inside `persistMCPSectionUpdateLocked`): replace the two hard-coded calls with `s.syncStoreMCPs()`.
- `ui/js/desktop/apps/software-store.js` (:6, :315-321, :576, :584, :587-592) and the rebuilt `ui/js/desktop/bundles/main.bundle.js`.
- `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`: 3 keys each.
- `ui/desktop_store_test.go`
- `documentation/store-agent-connections.md`, `internal/desktopstore/AGENTS.md`, `internal/server/AGENTS.md` (:108), `licences/UEBERSICHT.md` (:101, Store catalog table :179-207)
- Optional: one line in `documentation/manuals/{en,de}/08-integrations.md` (:2517 store list).

**No change needed**
- Root `AGENTS.md`: the routing row (:236) already sends Store work to `internal/desktopstore/AGENTS.md`.
- `ui/AGENTS.md`: no new contract, and a concurrent session has staged edits there; leave it alone.
- README.

## 5. Repo rules for every batch

- **Impact before editing.** Run from the main checkout `C:\Users\Andi\Documents\repo\AuraGo`: `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<symbol>" --direction upstream --repo .` (or MCP `impact` with that repo path; the worktree is not indexed). Report HIGH/CRITICAL in the review and the commit body. Treat UNKNOWN or "not found" as unresolved and confirm callers with Grep. Symbols that exist only on the branch cannot resolve.
- **detect-changes** cannot run in a worktree (indexing a worktree runs out of memory). Put this line in each commit body: "GitNexus detect-changes unavailable in the worktree; judged by symbols and tests." Run `detect-changes --scope compare --base-ref main` from the main checkout after merging, before pushing.
- **Go.** `GOCACHE` stays on D:. Use a backslash `GOTMPDIR`: `D:\tmp\go-build` for desktopstore and ui, a short `C:\tmp\go-build` for `internal/server` (known cross-drive `Rel` failure). If vet is missing, prefix `GOTOOLCHAIN=go1.27.4`. Group `-run` patterns as `(?i)(a|b)` and count the PASS lines.
- **Commit** after each batch, staging only the batch's paths. One combined review per batch (Critical/Important findings only), essential tests only.
- Write tool: Appendix A contains no `\u` sequences; keep it that way (the Write tool decodes them).

## 6. Tasks

### Batch 1: Store core (desktopstore)

**Impact first on:** `DefaultCatalog`, `IsHTTPPort`, `allocatePortBindings`, `updatePortBindings`, `NewService`, `runtimeContainerSpec`, `appReadinessTimeoutFor`, `IntegrationApps` (a var; confirm with Grep). `runtimeContainerSpec`, `IsHTTPPort` and the port functions sit on the install/update/Tailscale/OpenURL paths, so expect HIGH. Warn, keep the changes narrow, and rely on the full package suite.

**Task 1.1: `internal/desktopstore/velxio.go` and the launcher asset (new)**
- Constants:
  - `VelxioAppID = "velxio"`, `VelxioMCPPortID = "agent-mcp"`, `velxioMCPPort = 8002`
  - `velxioTokenKey = "mcp_token"`, `velxioTokenEnv = "AURAGO_VELXIO_MCP_TOKEN"`
  - `velxioPortEnv = "AURAGO_VELXIO_MCP_PORT"`, `velxioScriptEnv = "AURAGO_VELXIO_MCP_PY"`
  - `velxioReadinessTimeout = 15 * time.Minute`
- `func VelxioMCPTokenKey() string { return storeSecretVaultKey(VelxioAppID, velxioTokenKey) }` returns `desktop_store_velxio_mcp_token`.
- `const velxioWrapper`, verified text, one line:
  ```
  ( while :; do python -I -c "$AURAGO_VELXIO_MCP_PY"; sleep 5; done ) & unset AURAGO_VELXIO_MCP_PY AURAGO_VELXIO_MCP_TOKEN; exec /app/entrypoint.sh "$@"
  ```
- `//go:embed velxio_assets/aurago_velxio_mcp.py` → `var velxioMCPLauncher string`.
- `func velxioContainerSpec(spec ContainerSpec) ContainerSpec`:
  - drop any existing `velxioScriptEnv` / `velxioPortEnv` entries (with `hasEnvKey`);
  - append `AURAGO_VELXIO_MCP_PORT=8002` and `AURAGO_VELXIO_MCP_PY=<script>`;
  - set `spec.Entrypoint = []string{"/bin/sh", "-c", velxioWrapper, "aurago-velxio"}`;
  - the token comes from `addGeneratedRuntimeEnv`.
- Doc comment: python `-I` keeps `/app` and `PYTHON*` out of the import path (the script inserts `/app` itself); the script and token are unset before the image entrypoint starts; the restart-loop subshell keeps them.
- Asset `internal/desktopstore/velxio_assets/aurago_velxio_mcp.py`: Appendix A verbatim, plus a 3-line header (SPDX line and a contract pointer).

**Task 1.2: catalog entry** (`catalog.go`, after `commandcode`)
- Use the D1 values. The comment block records: upstream v3.0.5, AGPL-3.0, only tag `master`, the amd64+arm64 index digest, the probe date and host, root and no hardening, `SAMEORIGIN` → open_external, the MCP launcher pointer.
- `Image: "ghcr.io/davidmonterocrespo24/velxio@sha256:6aa5200e4d5f51235ce6ce024a17bba93d72ca1162653315e0b44f6197000429"`.
- Description (English, as for Node64/DeepTutor; the board wording is confirm item C3):
  > Arduino, ESP32 and Raspberry Pi Pico simulator in the browser: write sketches, wire virtual circuits and run them without hardware. This self-hosted edition covers Arduino AVR boards, ATtiny, the Raspberry Pi Pico and the ESP32 family (emulated with QEMU); STM32 and Raspberry Pi Linux boards exist only on velxio.dev. The image needs about 7 GB of disk space, and the first start downloads the Arduino cores (internet required), which can take several minutes. It opens in its own browser tab. Velxio has no login of its own, so keep access on this machine unless the network is trusted. Connect to agent lets AuraGo design circuits and compile sketches through Velxio's MCP tools; the in-editor AI assistant of velxio.dev is not part of this edition.

**Task 1.3: integration and port plumbing**
- `integration.go:37`: append `"velxio"` to `IntegrationApps`.
- `integration.go`: add
  - `func IsAgentOnlyPort(id string) bool` → `strings.HasPrefix(strings.ToLower(strings.TrimSpace(id)), "agent-")`;
  - `IsHTTPPort` (:51) gets `&& !IsAgentOnlyPort(port.ID)`;
  - `func IntegrationPortID(appID, adminPort string) string` → `VelxioMCPPortID` for Velxio, otherwise `adminPort`.
- `service.go:150-163` (`NewService`): if `strings.TrimSpace(ExtraPorts[i].HostIP) != ""` and `!net.ParseIP(...).IsLoopback()`, return `catalog app %s extra port %s must be pinned to a loopback address`. Apply the same check to `PrimaryPort.HostIP`.
- `service.go:1925-1941` (`allocatePortBindings`): `bindIP := hostIP; if pinned := strings.TrimSpace(port.HostIP); pinned != "" { bindIP = pinned }`, and use `bindIP` in the `PortBinding`.
- `service.go:1967-1973` (`updatePortBindings`): after the existing HostIP fallback, `if pinned := strings.TrimSpace(port.HostIP); pinned != "" { binding.HostIP = pinned }`.

**Task 1.4: runtime spec and readiness**
- `gods_eye.go`, inside `runtimeContainerSpec` after the DeepTutor branch: `if app.AppID == VelxioAppID { return velxioContainerSpec(spec), nil }`. This runs after `addGeneratedRuntimeEnv`, which already injected the token.
- `printguard.go:319` `appReadinessTimeoutFor`: `case VelxioAppID: return velxioReadinessTimeout`.

**Task 1.5: tests (lean)**
- `service_test.go:30-56`: add `"velxio": {image: <digest ref>, port: 80, icon: "code", runtime: RuntimeContainerWebApp}`.
- New `velxio_test.go`:
  1. `TestVelxioCatalogEntry`
     - digest regex `^ghcr\.io/davidmonterocrespo24/velxio@sha256:[0-9a-f]{64}$`;
     - `dev`/`code`/`velxio`/`""` presentation and `open_external`;
     - the exact PrimaryPort and the single ExtraPort (ID `agent-mcp`, ContainerPort `velxioMCPPort`, HostIP `127.0.0.1`);
     - the 5 exact volumes; Env `== ["VELXIO_NEWS=off"]`;
     - exactly one unexposed generated secret `{mcp_token, AURAGO_VELXIO_MCP_TOKEN, "AuraGo MCP token"}`;
     - `Hardening == nil`, no host binds, workspace binds or companions;
     - description phrases "no login", "own browser tab", "7 GB", "first start";
     - `HasIntegration("velxio")`;
     - `appReadinessTimeoutFor(...) >= 10*time.Minute`;
     - `VelxioMCPTokenKey() == "desktop_store_velxio_mcp_token"`.
  2. `TestInstallVelxioWiresMCPLauncherAndKeepsTokenOutOfTheRecord`
     - Install with `BindModeLAN` using `newTestServiceWithSecrets`, `fakeSecretStore` and `fixedPorts(18080, 18002)`.
     - The Entrypoint equals `{"/bin/sh","-c",velxioWrapper,"aurago-velxio"}`.
     - Env: the token equals the Vault value (at least 32 hex), `AURAGO_VELXIO_MCP_PY == velxioMCPLauncher`, `AURAGO_VELXIO_MCP_PORT=8002`, `VELXIO_NEWS=off`.
     - Port bindings: web `0.0.0.0:18080→80`, agent-mcp `127.0.0.1:18002→8002`.
     - `assertVolumeBinding` for the 5 `aurago_store_velxio_*` volumes.
     - `env_json`, `secret_refs_json` and the public `json.Marshal(app)` contain neither the token nor `AURAGO_VELXIO_MCP_PY`; one SecretRef, unexposed.
     - `svc.OpenURL(ctx, "velxio", "", false, "", "agent-mcp")` returns an error.
     - Update: the second create keeps the same token, the MCP port stays `127.0.0.1:18002`, and the Entrypoint is present.
  3. `TestVelxioLauncherScriptAndWrapper`
     - The wrapper contains `python -I -c "$AURAGO_VELXIO_MCP_PY"`, `unset AURAGO_VELXIO_MCP_PY AURAGO_VELXIO_MCP_TOKEN` and `exec /app/entrypoint.sh "$@"`.
     - The script contains `hmac.compare_digest`, `enable_dns_rebinding_protection=False`, `streamable_http_app()`, `stateless_http = True`, `http://127.0.0.1:8001/health`, `AURAGO_VELXIO_MCP_TOKEN`, `AURAGO_VELXIO_MCP_PORT`.
     - If `python3` or `python` is on PATH, `exec.Command(py, "-I", "-c", "import ast,sys; ast.parse(sys.stdin.read())")` with the script on stdin succeeds; otherwise skip.
  4. `TestCatalogPinnedPortHostMustBeLoopback`: `NewService` with a one-entry catalog whose ExtraPort has `HostIP "0.0.0.0"` returns an error; `"127.0.0.1"` is accepted.
- These existing tests must stay green without edits: `TestStoreCatalogEntriesCarryStartMenuCategories`, `TestStoreCatalogIconsAreAcceptedByDesktop`, `TestCatalogHardening*`, `TestCatalogJSONDoesNotExposeHardening`, `TestDockerCreatePayloadEntrypoint`, `TestDeepTutor*`, `TestNode64*`, install-journal and rollback suites.

**Verify**
```
cd /c/Users/Andi/Documents/repo/AuraGo/.worktrees/velxio
GOTMPDIR='D:\tmp\go-build' go test ./internal/desktopstore -count=1
GOTMPDIR='D:\tmp\go-build' go test ./internal/desktopstore -count=1 -v -run '(?i)(velxio|DefaultCatalogContainsInitialApps|StartMenuCategories|CatalogIconsAreAccepted|CatalogHardening|DockerCreatePayloadEntrypoint|PinnedPortHost)' | grep -c -- '--- PASS'
go vet ./internal/desktopstore
```

**Commit** `feat(store): add Velxio with a loopback-only MCP launcher`. Stage: `internal/desktopstore/{catalog.go,integration.go,service.go,gods_eye.go,printguard.go,velxio.go,velxio_test.go,service_test.go}` and `internal/desktopstore/velxio_assets/aurago_velxio_mcp.py`.

### Batch 2: agent connection (server)

**Impact first on:** `storeMCPName`, `storeMCPConfig`, `probeStoreMCP`, `syncStoreMCP`, `revokeStoreIntegrations`, `stopStoreIntegrations`, `refreshStoreIntegration`, `storeIntegrationURL`, `storeIntegrationStatus`, `saveStoreIntegration`, `validateStoreIntegrationGrants`, `testStoreIntegration`, `handleUpdateConfig` (a hub handler; expect HIGH/CRITICAL, the edit is a 2-line call swap), `persistMCPSectionUpdateLocked`.

**Task 2.1: `desktop_store_mcp.go`**
- `var storeMCPApps = []string{"n8n", "bytestash", desktopstore.VelxioAppID}`. `storeMCPName` (:17) uses `slices.Contains(storeMCPApps, id)`.
- `func (s *Server) syncStoreMCPs()` loops `storeMCPApps` calling `syncStoreMCP`.
- `func (s *Server) velxioMCPToken() (string, error)`: nil-Vault check, `s.Vault.ReadSecret(desktopstore.VelxioMCPTokenKey())`, trimmed non-empty, `security.RegisterSensitive`. On error: "Velxio's agent token is missing in the AuraGo vault; run a Store Update or reinstall Velxio".
- `storeMCPConfig` (:24-49):
  - compute `credential := settings.Credential`; for Velxio, `credential, err = s.velxioMCPToken()`;
  - the empty-credential check now looks at `credential`;
  - Velxio branch: `path = "/mcp"`, `allowed = velxioDesignTools` (the 5 names); if `settings.Execute && !cfg.VirtualDesktop.ReadOnly`, append `compile_project` and `run_project`;
  - Authorization header `"Bearer " + credential`; the `AllowDestructive` expression is unchanged (it already covers Execute).
- `probeStoreMCP` (:56-59): for Velxio, the failure text is "Velxio's MCP server is not reachable yet. After a start it can take a few minutes (the first start installs Arduino cores); AuraGo keeps retrying."

**Task 2.2: `desktop_store_integration.go`**
- `revokeStoreIntegrations` (:79-80) and `stopStoreIntegrations` (:661-662): loop `storeMCPApps`.
- `storeIntegrationURL` (:192):
  - for Velxio, ignore `settings.URL` (always derive, so a reinstall with kept settings and new host ports still works);
  - at :218 use `desktopstore.IntegrationURL(app, desktopstore.IntegrationPortID(app.AppID, settings.AdminPort), cfg.Runtime.IsDocker)`.
- `storeIntegrationStatus` (:232-238): for Velxio, `credentialStored` is true when `velxioMCPToken()` succeeds. Never return the token.
- `saveStoreIntegration`:
  - after building `next` (:368), for Velxio set `next.URL, next.Credential = "", ""` (client values are ignored);
  - :419 uses `IntegrationPortID(app.AppID, next.AdminPort)`.
- `validateStoreIntegrationGrants` (:515): add `"velxio"` to the Execute allowlist.
- `testStoreIntegration` (:588):
  - `case "n8n", "bytestash", desktopstore.VelxioAppID:`;
  - before probing, for Velxio, `settings.URL, err = s.storeIntegrationURL(ctx, app, settings)`, so the binding holds the current URL;
  - the `acquireStoreAccess` base comparison stays consistent because `syncStoreMCP` uses `binding.settings.URL`.
- Settle loop:
  - add `settling map[string]bool` to `storeIntegrationRuntime`;
  - package vars `storeIntegrationSettleBudget = map[string]time.Duration{desktopstore.VelxioAppID: 5 * time.Minute}` and `storeIntegrationSettleInterval = 10 * time.Second`;
  - split the body of `refreshStoreIntegration` into `refreshStoreIntegrationOnce(store, id) (tested bool, err error)`;
  - `refreshStoreIntegration` calls it and, when `tested && err != nil && storeIntegrationSettleBudget[id] > 0`, calls `s.settleStoreIntegration(store, id)`;
  - `settleStoreIntegration` starts one goroutine per app (guarded by `settling` under the mutex). Each iteration:
    1. wait for the interval, or `<-s.ShutdownCh` (a nil channel is fine; `time.After` still fires);
    2. stop if `s.storeIntegrations.closed`, `ReadIntegration` is not Read or has no Revision, or `store.GetInstalled` shows the app not running;
    3. pre-probe: `cfg := s.storeMCPConfig(id, settings with URL := s.storeIntegrationURL(...))`, then `storeMCPProbeTools` with a 5 s timeout (no operation slot, no `CfgSaveMu`);
    4. on pre-probe success, `refreshStoreIntegrationOnce`; on bind, `s.broadcastDesktopStoreChanged("")` and return.
- Leave the `runDesktopStoreOperation` defer (desktop_store_handlers.go:668) unchanged; it already calls `refreshStoreIntegration`.

**Task 2.3: MCP reload call sites**
`config_handlers_main.go:1790-1791` and `mcp_handlers.go:166-167` become `s.syncStoreMCPs()`.

**Task 2.4: tests** (`internal/server/desktop_store_velxio_test.go`)
1. `TestVelxioStoreMCPUsesGeneratedTokenAndGrantAllowlists`
   - Setup: `storeIntegrationFixture(t, "velxio")` (web 18000, agent-mcp 18080), and `s.Vault.WriteSecret(desktopstore.VelxioMCPTokenKey(), tok)`.
   - Assert:
     - `storeMCPName == "aurago-store-velxio"`;
     - `saveStoreIntegration` with `{URL:"http://evil.example:1", Credential:"user-typed", Read:true}` saves `URL == "http://127.0.0.1:18080"` and `Credential == ""`;
     - `storeMCPConfig` with Read gives URL suffix `/mcp`, header `Bearer tok`, exactly the 5 design tools, `!AllowDestructive`;
     - with Execute it adds `compile_project` and `run_project` and `AllowDestructive`;
     - with the Vault key deleted it returns an error;
     - stubbed `storeMCPProbeTools` returning all 7 names passes; without `compile_project` (Execute) it fails;
     - `storeIntegrationStatus` JSON does not contain `tok` and has `credential_stored: true`;
     - `desktopStoreTailscaleProxySpecs` for the installed record with `TailscaleEnabled` yields no spec whose ID ends in `-agent-mcp`.
2. `TestVelxioConnectionSettlesWhenMCPStartsLate`
   - Set `storeIntegrationSettleInterval = 20*time.Millisecond`; the probe stub fails twice, then succeeds.
   - Save Read; call `s.refreshStoreIntegration(store, "velxio")`.
   - Poll up to 3 s until `s.storeApplicationEnabled("velxio")` (the binding exists) and assert the stub was called at least 3 times.
- These existing tests must stay green: `TestStoreMCPExactCapabilitiesAndMissingTool`, `TestStoreIntegration*`, `TestStoreAppExecutionNeedsVerifiedLiveGrant`, `TestDesktopStore*Tailscale*`.

**Verify**
```
GOTMPDIR='C:\tmp\go-build' go test ./internal/server -count=1 -v -run '(?i)(velxio|storemcp|storeintegration|storeapp|tailscale)' | grep -c -- '--- PASS'
GOTMPDIR='D:\tmp\go-build' go test ./internal/desktopstore ./internal/tools -count=1 -run '(?i)(mcp|integration|velxio)'
go vet ./internal/server
```

**Commit** `feat(store): connect Velxio's MCP server to the agent`. Stage: `internal/server/{desktop_store_mcp.go,desktop_store_integration.go,config_handlers_main.go,mcp_handlers.go,desktop_store_velxio_test.go}`.

### Batch 3: UI, translations, bundle

**Impact first on:** `openIntegrationModal`, `extraPortButtons` (JS; if unresolved, confirm with Grep).

**Task 3.1: `ui/js/desktop/apps/software-store.js`**
- :6: add `'velxio'` to `integrationApps`.
- :317: `entry.extra_ports.filter(port => !String(port.id || '').startsWith('agent-')).map(...)`, and return `''` when the filtered list is empty.
- :576: add `'velxio'` to the execute-grant list.
- :584: add `'velxio'` to the help list (shows `ig_help_velxio`).
- :587-590: hide `field('url')`, `field('credential','password')` and the `clear_credential` choice for `['ziro-designer','velxio']`. Keep `data-ig-stored` and the `docker_network` select (it shows only when containerized).
- :592: grant label becomes `label(appId === 'open-webui' && name === 'read' ? 'relay_enable' : appId === 'velxio' ? 'velxio_' + name : name)`.
- Payload code (:628-630) is unchanged: absent fields send `''`, and the server ignores url and credential for Velxio.

**Task 3.2: translations.** Add 3 keys right after `desktop.store.ig_help_ziro-designer` (en.json:12479) in all 16 Desktop locales. Inserting there keeps the hunks away from the concurrent Synth Studio edits in the main checkout.

| Key | en |
|---|---|
| `desktop.store.ig_help_velxio` | "No account or token is needed: AuraGo connects to Velxio's built-in MCP server with a generated token kept in the vault. Design tools return circuit JSON and code; compiling runs arduino-cli inside the Velxio container. Results go to the agent, not into an open editor; import them in Velxio. Right after a start the connection can take a minute." |
| `desktop.store.ig_velxio_read` | "Design circuits and code (create, update, import, export, generate)" |
| `desktop.store.ig_velxio_execute` | "Compile and run sketches in the Velxio container" |

de:
- help: "Kein Konto und kein Token nötig: AuraGo verbindet sich mit einem erzeugten, im Tresor gespeicherten Token mit dem eingebauten MCP-Server von Velxio. Entwurfswerkzeuge liefern Schaltungs-JSON und Code; kompiliert wird mit arduino-cli im Velxio-Container. Ergebnisse gehen an den Agenten, nicht in einen offenen Editor; importiere sie in Velxio. Direkt nach einem Start kann die Verbindung eine Minute dauern."
- read: "Schaltungen und Code entwerfen (erstellen, ändern, importieren, exportieren, erzeugen)"
- execute: "Sketches im Velxio-Container kompilieren und ausführen"

Translate the other 14 locales in the same plain style; keep "MCP" and "arduino-cli" as they are.

**Task 3.3: bundle rebuild (needed: `main.bundle.js` contains software-store.js)**
- The worktree needs a real `node_modules` copy, not a junction: `robocopy <main>\node_modules <worktree>\node_modules /E /MT:16 /NFL /NDL /NJH /NP`.
- Then `npm run build:ui` and `npm run check:ui`.

**Task 3.4: UI test** in `ui/desktop_store_test.go`: `TestSoftwareStoreVelxioAgentConnection`.
- Source markers: `'velxio'` inside the `integrationApps` set line, `startsWith('agent-')`, `'velxio_' + name`.
- Every `lang/desktop/*.json` has non-empty values for the 3 keys (same loop as `TestSoftwareStoreExpandedCapabilityTranslations`, ui/desktop_store_test.go:366-388).

**Verify**
```
npm run check:ui
GOTMPDIR='D:\tmp\go-build' go test ./ui -count=1 -v -run '^(TestSoftwareStoreVelxioAgentConnection|TestSoftwareStoreSupportsExpandedAppCapabilities|TestSoftwareStoreExpandedCapabilityTranslations|TestSoftwareStoreOpensExternalStoreAppsInBrowserTab|TestSoftwareStoreTranslatesStructuredOperationErrors)$'
```
Recommended, because the dialog changed: `-run '^TestSoftwareStoreDisposesSensitiveDialogsBrowser$' -timeout 30m`. Check for leftover rod Chromes first.

**Commit** `feat(store): Velxio agent connection in the Software Store`. Stage: `ui/js/desktop/apps/software-store.js`, `ui/js/desktop/bundles/main.bundle.js`, the 16 `ui/lang/desktop/*.json`, `ui/desktop_store_test.go`.

### Batch 4: documentation and licence

**Task 4.1: `documentation/velxio.md` (new, Node64 style)**
- What it is (AGPL-3.0, upstream link).
- Install: about 7 GB disk; first start needs internet for the cores; 75 s on a fast link, longer on slow ones; very slow links should `docker pull` first (30-minute Store operation limit).
- Open: own tab, no login, LAN warning.
- Boards (self-hosted vs velxio.dev).
- Privacy: `VELXIO_NEWS=off`, plus confirm item C5.
- Data: the 5 volumes; uninstall vs Delete data.
- Agent connection:
  - grants and tools; generated token in the Vault;
  - loopback-only port, never openable, never on the tailnet;
  - containerized needs a shared network plus an Update; a remote Engine is unsupported;
  - results are stateless and go to the agent, not into an open editor;
  - after a start the connection retries for up to 5 minutes;
  - AuraGo's 60 s MCP call limit (`mcpCallToolTimeout`): a cold ESP32 build may exceed it, and ccache makes the retry fast.
- Security limits from D3.
- Update: new versions come with AuraGo releases that change the digest pin.
- Licence: unmodified image; the launcher in `internal/desktopstore/velxio_assets/`.
- Live-verification record from Batch 5.

**Task 4.2: `documentation/store-agent-connections.md`** — new section "## Velxio (Arduino simulator)" after "Workflows and snippets" (:84-115):
- reserved name `aurago-store-velxio`;
- exact read and execute allowlists;
- no URL or credential, Vault-only token;
- read-only Desktop withholds compile;
- the settle retry;
- the test reads `tools/list` only.
- Extend the reserved-name sentence at :97.

**Task 4.3: `internal/desktopstore/AGENTS.md`** — new "### Velxio" contract after Node64 (:195-220):
- the pin and how to re-probe on a bump;
- ports, including the generic rule: a catalog `HostIP` pin must be loopback, and `agent-*` IDs are never HTTP/openable/Tailscale ports;
- volumes and env;
- token path (generic generated secret, injected by `addGeneratedRuntimeEnv`, never in the record);
- wrapper and launcher contract: why `"$@"` stays, why it waits for `:8001/health`, `-I`, the restart loop, the unset;
- readiness 15 min on the web port only;
- the probe evidence;
- the verify list: `TestVelxio*`, `TestInstallVelxio*`, `TestCatalogPinnedPortHostMustBeLoopback`, `TestDefaultCatalogContainsInitialApps`, the server `TestVelxio*`.

**Task 4.4: `internal/server/AGENTS.md:108`** — "n8n/ByteStash/Velxio managed MCP connections …; Velxio's bearer comes from the Vault key `desktop_store_velxio_mcp_token`, its URL is always derived, and a failed post-operation check retries in the background for up to 5 minutes (pre-probe without the operation slot)."

**Task 4.5: `licences/UEBERSICHT.md`**
- Store catalog table (:179-207), add: `| Velxio | ghcr.io/davidmonterocrespo24/velxio@sha256:6aa5200e4d5f51235ce6ce024a17bba93d72ca1162653315e0b44f6197000429 (v3.0.5, Tag master) | AGPL-3.0 (der Autor bietet zusätzlich eine kommerzielle Lizenz an). Unverändertes Image; der AuraGo-Starter für den eingebauten MCP-Server liegt in internal/desktopstore/velxio_assets/ (AGPL-3.0-or-later). |`
- Line :101: add Velxio to "OliveTin, RomM, MinIO, Garage".

**Task 4.6 (optional):** add "Velxio" to the catalog sentence in `documentation/manuals/{en,de}/08-integrations.md:2517`.

**Verify:** `git diff --check`. **Commit** `docs(store): Velxio app and agent connection` with only these doc paths.

### Batch 5: live verification (outside the repo, opt-in)

**Harness:** `C:\disposable\_velxio-live\zz_velxio_live_test.go`, mapped into `internal/desktopstore` with `go test -overlay overlay.json`, gated by `AURAGO_VELXIO_LIVE=1`.
- Build a permissive `config.Config` (Docker enabled, network requests, MCP) and call `tools.ConfigureRuntimePermissions(tools.RuntimePermissionsFromConfig(cfg))`.
- Real `security.NewVault` (temporary key and file), `NewService` with `Docker: NewToolsDockerAdapter("", tmpWorkspace, logger)`, default port allocator and probe, no Desktop or Launchpad.

**Steps**
1. Install `velxio` with BindModeLAN. Measure the install time (operation start to `RunOperation` return).
2. `docker inspect`:
   - web bound to `0.0.0.0`, 8002 bound to `127.0.0.1` only;
   - five named volumes, no anonymous volumes;
   - Entrypoint matches.
3. Poll `tools.ProbeMCPServerTools` against `http://127.0.0.1:<agent-mcp host port>/mcp` with `Bearer <vault token>`, `AllowPrivateNetwork: true`. Record when the MCP comes up; expect the 7 names. A wrong token must error.
4. `tools.InitMCPManager(nil, logger)`, `tools.SetManagedMCPServer("aurago-store-velxio", cfg{ManagedRevision:"live", AllowedTools: all 7, AllowDestructive: true})`, then `tools.MCPCallTool(ctx, "aurago-store-velxio", "compile_project", <blink args for arduino:avr:uno>)`: expect success with non-empty hex. Take the argument names from the probe's call or the `tools/list` inputSchema (C4). Also call `create_circuit` once.
5. `docker exec aurago-store-velxio sh -c 'tr "\0" "\n" </proc/1/environ | grep AURAGO_'` must print nothing.
6. Restart operation: measure web and MCP times (expect about 17 and 24 s).
7. Uninstall with `delete_data`: the 5 volumes are gone (`docker volume ls`) and the Vault token is deleted.
8. Clean up the container, volumes and image only if you pulled it (ask first).

**Command**
```
AURAGO_VELXIO_LIVE=1 GOTMPDIR='D:\tmp\go-build' go test -overlay C:\disposable\_velxio-live\overlay.json ./internal/desktopstore -run '^TestVelxioLive$' -v -timeout 40m
```

**Not covered:** containerized AuraGo on a shared network, and an agent chat that uses the tools. The user's acceptance on a scratch AuraGo covers those: Connect to agent → Save → Test → ask the agent to compile blink. Do not install on aurago-test without the user's approval (its rule is no creating or updating apps).

Write the measured timings and the date into the catalog comment, the AGENTS.md "Velxio" section and `documentation/velxio.md`. **Commit** `docs(store): record Velxio live verification`.

## 7. Confirm items (resolved by the controller, 2026-11-20)

- **C1** No `HEALTHCHECK` in the image (`.Config.Healthcheck` is `null`); readiness is the Store's own web-port check.
- **C2** Image labels: `org.opencontainers.image.revision` `5ed52d7332b8bd87f4dcd71f1ed49d8ffad964aa`, created `2026-11-18T17:12:16Z`, version `master`, source `https://github.com/davidmonterocrespo24/velxio`. Use that commit as the AGPL source pointer in the docs and the licence row.
- **C3** Board wording: keep the README's self-hosted list (Arduino AVR incl. ATtiny85, Raspberry Pi Pico, ESP32/ESP32-S3/ESP32-C3 via QEMU). The entrypoint installs `arduino:avr`, `rp2040:rp2040`, `ATTinyCore:avr`; ESP32 builds use the bundled ESP-IDF. ESP32 emulation is not live-checked; say so in `documentation/velxio.md`.
- **C4** `compile_project` arguments verified live: `{"files":[{"name":"blink.ino","content":"..."}],"board":"arduino:avr:uno"}`; result keys `success, hex_content, binary_content, stdout, stderr`. `create_circuit` takes `board_fqbn, components[], connections[]`.
- **C5** Not checked; the docs only state that `VELXIO_NEWS=off` disables the news feed and its remote media.
- **C6** Not checked; document the 60 s MCP call limit as a known limit for a cold ESP32 build.

## 8. Decisions (controller defaults; reported to the user)

- **Q1** Launcher header `SPDX-License-Identifier: AGPL-3.0-or-later`.
- **Q2** `Icon: "code"`; a dedicated Velxio theme icon is a possible follow-up.
- **Q3** English-only store description, like Node64/DeepTutor.
- **Q4** Digest bump per AuraGo release with a re-probe, like Node64.

## 9. Risks

- **Shared port plumbing.** HostIP pinning and the `agent-` exclusion touch the install/update/Tailscale/OpenURL paths. They are inert for existing entries (no app port sets HostIP, no `agent-` IDs today). The full desktopstore suite and the Tailscale server tests guard them.
- **Settle loop.** Bounded to 5 min, one loop per app, no operation slot or config lock while pre-probing. A Stop during a verified attempt is refused for a few seconds at most.
- **Native readiness** is a plain TCP connect. On Docker Desktop and the Linux userland proxy, a published port accepts early, so "running" can come before nginx serves. The first Open may need a reload; the docs say so. This is existing Store behaviour.
- **Install size.** The 1.6 GB pull plus first start must fit the 30-minute operation context; covered in the docs.

## Appendix A: `internal/desktopstore/velxio_assets/aurago_velxio_mcp.py` (verified live, copy verbatim; add only the header comment)

Source: `C:\Users\Andi\AppData\Local\Temp\claude\9361e9463ef9762e\4551ab40-d1c5-4042-86b1-2f9d9ec52e6a\scratchpad\velxio-probe\mcp_launcher.py`
```python
import hmac
import os
import sys
import time
import urllib.request

sys.path.insert(0, "/app")
os.chdir("/app")

token = os.environ.get("AURAGO_VELXIO_MCP_TOKEN", "")
port = int(os.environ.get("AURAGO_VELXIO_MCP_PORT", "8002"))
if not token:
    print("aurago-velxio-mcp: no token, MCP server not started", file=sys.stderr)
    sys.exit(0)

# Importing app.mcp.server initialises arduino-cli (config + core install).
# Wait until the image's own entrypoint has finished that and started the
# backend, so the two never install cores at the same time.
deadline = time.monotonic() + 1800
while True:
    try:
        with urllib.request.urlopen("http://127.0.0.1:8001/health", timeout=3) as r:
            if r.status == 200:
                break
    except Exception:
        pass
    if time.monotonic() > deadline:
        print("aurago-velxio-mcp: backend not healthy, giving up", file=sys.stderr)
        sys.exit(1)
    time.sleep(2)

from app.mcp.server import mcp  # noqa: E402
from mcp.server.transport_security import TransportSecuritySettings  # noqa: E402

# The bearer token replaces FastMCP's localhost-only Host allowlist, which
# would reject AuraGo reaching the server by container name.
mcp.settings.transport_security = TransportSecuritySettings(
    enable_dns_rebinding_protection=False
)
mcp.settings.stateless_http = True
mcp.settings.json_response = True
inner = mcp.streamable_http_app()
expected = ("Bearer " + token).encode()


async def guarded(scope, receive, send):
    if scope["type"] == "http":
        auth = b""
        for key, value in scope.get("headers", []):
            if key == b"authorization":
                auth = value
                break
        if not hmac.compare_digest(auth, expected):
            await send(
                {
                    "type": "http.response.start",
                    "status": 401,
                    "headers": [(b"content-type", b"text/plain")],
                }
            )
            await send({"type": "http.response.body", "body": b"unauthorized"})
            return
    await inner(scope, receive, send)


import uvicorn  # noqa: E402

print("aurago-velxio-mcp: listening on port %d" % port, file=sys.stderr)
uvicorn.run(guarded, host="0.0.0.0", port=port, log_level="warning")
```
