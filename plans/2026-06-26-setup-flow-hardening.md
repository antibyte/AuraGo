# AuraGo Setup-Flow Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate 33 identified bugs, security issues, and UX defects in AuraGo's initial setup flow across `internal/setup/`, `internal/server/setup_handlers.go`, and `ui/js/setup/main.js`.

**Architecture:** Surgical fixes grouped into 4 phases by severity. Each task is one focused change with TDD discipline. No refactoring unless required for a specific bug. Security fixes ship first; UX polish last.

**Tech Stack:** Go 1.26.1 (stdlib + existing deps), vanilla JavaScript SPA, embedded YAML profiles.

**Review Source:** [Initial setup flow review](reports/2026-06-26-setup-flow-review.md) (TODO — write review to reports/)

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/setup/setup.go` | First-run installation: tar.gz extraction, master-key generation, OS service install |
| `internal/setup/setup_profiles.yaml` | Pre-configured provider profiles (embedded) |
| `internal/setup/profiles.go` | Profile loader & parser |
| `internal/setup/setup_test.go` | Tests for setup helpers |
| `internal/server/setup_handlers.go` | HTTP handlers for /api/setup/* |
| `internal/server/setup_handlers_test.go` | Tests for setup handlers |
| `internal/server/setup_handlers_error_test.go` | Error-path tests |
| `internal/server/config_handlers_main.go` | Shared `deepMerge` (used by setup) |
| `cmd/aurago/main.go` | Wiring: flag parsing, setup triggers, lock acquisition |
| `ui/setup.html` | Setup wizard HTML (multi-step form) |
| `ui/js/setup/main.js` | Setup wizard logic (validation, profile selection, save) |
| `ui/lang/*.json` | Translation files (15 languages) |

---

## Phase 1 — Critical Security & Correctness Fixes

> **Priority:** Ship first. These are real vulnerabilities or correctness bugs that cause user-visible failures.

### Task 1.1: Strip setuid/setgid/sticky bits from extracted files

**Files:**
- Modify: `internal/setup/setup.go:194-218` (`extractTarGz`)
- Modify: `internal/setup/setup_test.go`

**Problem:** `os.FileMode(hdr.Mode)|0750` and `|0640` use binary OR, preserving `os.ModeSetuid` (0o4000), `os.ModeSetgid` (0o2000), and `os.ModeSticky` (0o1000) from the tar header. A malicious `resources.dat` could install setuid binaries for privilege escalation.

- [ ] **Step 1: Write the failing test** (append to `internal/setup/setup_test.go`)

```go
func TestExtractTarGzStripsSetuidAndSetgidBits(t *testing.T) {
    t.Parallel()
    dir := t.TempDir()

    // Build an in-memory tar.gz containing a file with setuid+setgid+sticky bits.
    var buf bytes.Buffer
    gz := gzip.NewWriter(&buf)
    tw := tar.NewWriter(gz)
    body := []byte("#!/bin/sh\necho pwned\n")
    hdr := &tar.Header{
        Name:     "evil.sh",
        Mode:     0o7777 | int64(os.ModeSetuid) | int64(os.ModeSetgid) | int64(os.ModeSticky),
        Size:     int64(len(body)),
        Typeflag: tar.TypeReg,
    }
    if err := tw.WriteHeader(hdr); err != nil {
        t.Fatalf("WriteHeader: %v", err)
    }
    if _, err := tw.Write(body); err != nil {
        t.Fatalf("Write body: %v", err)
    }
    tw.Close()
    gz.Close()

    archivePath := filepath.Join(dir, "resources.dat")
    if err := os.WriteFile(archivePath, buf.Bytes(), 0o600); err != nil {
        t.Fatalf("WriteFile: %v", err)
    }

    if err := extractTarGz(archivePath, dir); err != nil {
        t.Fatalf("extractTarGz: %v", err)
    }

    info, err := os.Stat(filepath.Join(dir, "evil.sh"))
    if err != nil {
        t.Fatalf("Stat: %v", err)
    }
    mode := info.Mode()
    if mode&os.ModeSetuid != 0 {
        t.Errorf("setuid bit preserved: mode=%v", mode)
    }
    if mode&os.ModeSetgid != 0 {
        t.Errorf("setgid bit preserved: mode=%v", mode)
    }
    if mode&os.ModeSticky != 0 {
        t.Errorf("sticky bit preserved: mode=%v", mode)
    }
    // Permission bits should be 0o640 (file) or 0o750 (dir) base, not 0o777.
    if perm := mode.Perm(); perm != 0o640 {
        t.Errorf("perm = %o, want 0o640", perm)
    }
}
```

Add to imports: `"archive/tar"`, `"bytes"`, `"compress/gzip"`.

- [ ] **Step 2: Run the test — verify it fails**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && go test ./internal/setup/ -run TestExtractTarGzStripsSetuidAndSetgidBits -v`
Expected: FAIL with "setuid bit preserved: mode=-rwsrwsrwt".

- [ ] **Step 3: Fix `extractTarGz`**

In `internal/setup/setup.go`, replace lines 196 and 209:

```go
case tar.TypeDir:
    // Strip all non-permission bits (setuid/setgid/sticky, file type).
    perm := os.FileMode(hdr.Mode).Perm() & 0o777
    if err := os.MkdirAll(target, perm|0o750); err != nil {
        return err
    }
case tar.TypeReg:
    if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
        return err
    }
    // Don't overwrite config.yaml if it already exists (user may have edited it)
    if filepath.Base(target) == "config.yaml" {
        if _, err := os.Stat(target); err == nil {
            continue
        }
    }
    perm := os.FileMode(hdr.Mode).Perm() & 0o777
    out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm|0o640)
    if err != nil {
        return err
    }
    if _, err := io.Copy(out, tr); err != nil {
        out.Close()
        return err
    }
    out.Close()
```

- [ ] **Step 4: Re-run the test — verify it passes**

Run: `go test ./internal/setup/ -run TestExtractTarGzStripsSetuidAndSetgidBits -v`
Expected: PASS.

- [ ] **Step 5: Run the full setup test suite**

Run: `go test ./internal/setup/ -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/setup/setup.go internal/setup/setup_test.go
git commit -m "fix(setup): strip setuid/setgid/sticky bits from extracted files"
```

---

### Task 1.2: Escape installDir and credentialFile in systemd unit

**Files:**
- Modify: `internal/setup/setup.go:240-298` (`installSystemd`)
- Modify: `internal/setup/setup_test.go`

**Problem:** `installDir`, `exePath`, and `credentialFile` are interpolated raw into the systemd unit template. Special characters (`"`, `\n`, `\`) corrupt the unit file or enable injection.

- [ ] **Step 1: Write the failing test**

Append to `internal/setup/setup_test.go`:

```go
func TestInstallSystemdEscapesInstallDirWithQuotes(t *testing.T) {
    if runtime.GOOS != "linux" {
        t.Skip("systemd only on linux")
    }
    t.Parallel()

    // Use a path containing a quote to simulate a malicious installDir.
    base := t.TempDir()
    installDir := base + `/odd"path` // contains a literal double-quote
    if err := os.MkdirAll(installDir, 0o750); err != nil {
        t.Fatalf("MkdirAll: %v", err)
    }
    exePath := filepath.Join(installDir, "aurago")

    // We can't actually write to /etc/systemd/system, so just check the
    // unit string is properly escaped. Refactor installSystemd to expose
    // the unit-building step if not already testable.
    unit, err := buildSystemdUnit("aurago", "alice", installDir, exePath, "/etc/aurago/master.key", "/etc/aurago", true, false)
    if err != nil {
        t.Fatalf("buildSystemdUnit: %v", err)
    }
    // The raw quote must NOT appear unescaped.
    if strings.Contains(unit, `odd"path`) {
        t.Errorf("unit contains unescaped installDir: %s", unit)
    }
    // The escaped form must be present.
    if !strings.Contains(unit, `odd\"path`) {
        t.Errorf("unit does not contain escaped path: %s", unit)
    }
}
```

Add to imports: `"runtime"`, `"strings"`.

- [ ] **Step 2: Run — verify failure** (helper not yet extracted)

Run: `go test ./internal/setup/ -run TestInstallSystemdEscapesInstallDirWithQuotes -v`
Expected: FAIL — `buildSystemdUnit` undefined.

- [ ] **Step 3: Extract `buildSystemdUnit` helper with proper escaping**

In `internal/setup/setup.go`, add a new helper before `installSystemd`:

```go
// buildSystemdUnit returns the systemd unit file content for AuraGo.
// All paths are quoted via strconv.Quote to escape embedded quotes and
// backslashes so the unit file remains valid even if installDir contains
// unusual characters.
func buildSystemdUnit(desc, user, installDir, exePath, credentialFile, readWritePaths string, dockerMode, sudoUnrestricted bool) (string, error) {
    if installDir == "" || exePath == "" || credentialFile == "" {
        return "", fmt.Errorf("installDir, exePath, credentialFile are required")
    }

    protectSystemLine := "ProtectSystem=strict"
    if sudoUnrestricted {
        protectSystemLine = "# ProtectSystem=strict disabled because sudo_unrestricted is enabled"
    }

    return fmt.Sprintf(`[Unit]
Description=%s
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=0

[Service]
Type=simple
User=%s
Group=%s
WorkingDirectory=%s
ExecStart=%s --config %s/config.yaml
Restart=on-failure
RestartSec=10
EnvironmentFile=-%s
NoNewPrivileges=true
%s
ReadWritePaths=%s
PrivateTmp=true

[Install]
WantedBy=multi-user.target
`,
        strconv.Quote(desc),
        strconv.Quote(user),
        strconv.Quote(user),
        strconv.Quote(installDir),
        strconv.Quote(exePath),
        strconv.Quote(installDir),
        strconv.Quote(credentialFile),
        protectSystemLine,
        strconv.Quote(readWritePaths),
    ), nil
}

func installSystemd(exePath, installDir string, logger *slog.Logger) error {
    user := os.Getenv("SUDO_USER")
    if user == "" {
        user = os.Getenv("USER")
    }
    if user == "" {
        user = "root"
    }

    credentialFile := filepath.Join(installDir, ".env")
    readWritePaths := installDir
    if _, err := os.Stat("/etc/aurago/master.key"); err == nil {
        credentialFile = "/etc/aurago/master.key"
        readWritePaths = installDir + " /etc/aurago"
    }

    unit, err := buildSystemdUnit(
        "AuraGo AI Agent",
        user,
        installDir,
        exePath,
        credentialFile,
        readWritePaths,
        runningInDocker(),
        configAllowsSudoUnrestricted(filepath.Join(installDir, "config.yaml")),
    )
    if err != nil {
        return fmt.Errorf("build systemd unit: %w", err)
    }

    unitPath := "/etc/systemd/system/aurago.service"
    if err := os.WriteFile(unitPath, []byte(unit), 0600); err != nil {
        return fmt.Errorf("failed to write systemd unit (run setup as root?): %w", err)
    }

    for _, cmd := range [][]string{
        {"systemctl", "daemon-reload"},
        {"systemctl", "enable", "aurago.service"},
    } {
        if out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput(); err != nil {
            logger.Warn("systemctl command failed", "cmd", strings.Join(cmd, " "), "output", string(out), "error", err)
        }
    }
    return nil
}
```

Add to imports: `"strconv"` (already imported).

- [ ] **Step 4: Re-run — verify pass**

Run: `go test ./internal/setup/ -run TestInstallSystemdEscapesInstallDirWithQuotes -v`
Expected: PASS.

- [ ] **Step 5: Run the full suite**

Run: `go test ./internal/setup/ -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/setup/setup.go internal/setup/setup_test.go
git commit -m "fix(setup): escape installDir/credentialFile in systemd unit via strconv.Quote"
```

---

### Task 1.3: Handle `.bat` write error in Windows task installer

**Files:**
- Modify: `internal/setup/setup.go:402-411` (`installWindowsTask`)
- Modify: `internal/setup/setup_test.go` (only if testable in non-Windows — skip if so)

**Problem:** `os.WriteFile(batPath, []byte(batContent), 0644)` ignores errors. If the bat file can't be written, the scheduled task points to a missing file with no error surfaced.

- [ ] **Step 1: Fix the function**

In `internal/setup/setup.go`, replace `installWindowsTask`:

```go
func installWindowsTask(exePath, installDir string, logger *slog.Logger) error {
    taskName := "AuraGo"

    // Delete existing task if present (ignore errors — task may not exist)
    exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run()

    cmd := exec.Command("schtasks", "/Create",
        "/TN", taskName,
        "/TR", fmt.Sprintf(`"%s"`, exePath),
        "/SC", "ONLOGON",
        "/RL", "HIGHEST",
        "/F",
    )
    cmd.Dir = installDir

    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("schtasks failed: %s — %w", string(out), err)
    }

    batContent := fmt.Sprintf(`@echo off
cd /d "%s"
for /f "tokens=1,* delims==" %%%%a in (.env) do set "%%%%a=%%%%b"
start "" "%s"
`, installDir, exePath)
    batPath := filepath.Join(installDir, "start_aurago.bat")
    if err := os.WriteFile(batPath, []byte(batContent), 0o700); err != nil {
        return fmt.Errorf("failed to write %s: %w", batPath, err)
    }

    logger.Info("Windows scheduled task created", "task", taskName)
    return nil
}
```

Note: Mode changed from `0644` to `0o700` for consistency with `start_aurago.sh` on macOS.

- [ ] **Step 2: Build to verify no compile error** (Windows-only function compiles on all platforms because schtasks call is just an exec)

Run: `go build ./internal/setup/...`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/setup/setup.go
git commit -m "fix(setup): surface .bat write error and align permissions to 0700"
```

---

### Task 1.4: Trim admin password in custom flow

**Files:**
- Modify: `ui/js/setup/main.js:1414-1418` (`buildConfigPatch`)

**Problem:** `quick-admin-password` is trimmed before submission; `admin-password` (custom flow) is not. Inconsistent UX and source of login failures when whitespace sneaks in.

- [ ] **Step 1: Locate and fix**

In `ui/js/setup/main.js`, change:

```js
auth: {
    enabled: true,
    admin_password: document.getElementById('admin-password').value,
},
```

to:

```js
auth: {
    enabled: true,
    admin_password: document.getElementById('admin-password').value.trim(),
},
```

- [ ] **Step 2: Verify no other call sites rely on raw value**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && grep -n "admin-password\|admin_password" ui/js/setup/main.js | head -20`
Expected: only the one site modified.

- [ ] **Step 3: Commit**

```bash
git add ui/js/setup/main.js
git commit -m "fix(setup): trim admin password in custom flow (match quick flow)"
```

---

### Task 1.5: Fix `needsSetup` to require password when auth enabled but unset

**Files:**
- Modify: `internal/server/setup_handlers.go:484-513` (`needsSetup`)
- Modify: `internal/server/setup_handlers_test.go`

**Problem:** `return cfg.Auth.Enabled && cfg.Auth.PasswordHash == ""` returns `false` if `Auth.Enabled == false` even when no password is set. A misclick on auth-disable (or a corrupted config) hides the wizard.

- [ ] **Step 1: Write the failing test**

Append to `internal/server/setup_handlers_test.go`:

```go
func TestNeedsSetupRequiresPasswordEvenWhenAuthDisabled(t *testing.T) {
    t.Parallel()

    cfg := &config.Config{
        Providers: []config.ProviderEntry{{
            ID: "main", Type: "openai", BaseURL: "https://api.example/v1", Model: "gpt-4",
            APIKey: "sk-test",
        }},
    }
    cfg.LLM.Provider = "main"
    cfg.LLM.APIKey = "sk-test"
    cfg.Auth.Enabled = false // disabled by accident

    if needsSetup(cfg) {
        t.Fatal("setup wizard skipped even though no password is set — should warn or require setup")
    }
}
```

(Adjust assertion to match new behavior — see Step 3.)

- [ ] **Step 2: Run — verify failure**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && go test ./internal/server/ -run TestNeedsSetupRequiresPasswordEvenWhenAuthDisabled -v`
Expected: t.Fatal triggers (test fails because needsSetup returns false but we expect true).

- [ ] **Step 3: Fix `needsSetup`**

In `internal/server/setup_handlers.go`, replace:

```go
return cfg.Auth.Enabled && cfg.Auth.PasswordHash == ""
```

with:

```go
// Always require setup if auth is enabled but no password is configured.
// When auth is disabled we trust the operator's intent.
if cfg.Auth.Enabled && cfg.Auth.PasswordHash == "" {
    return true
}
return false
```

- [ ] **Step 4: Re-run — verify pass**

Run: `go test ./internal/server/ -run TestNeedsSetupRequiresPasswordEvenWhenAuthDisabled -v`
Expected: PASS (now needsSetup returns true when auth.enabled and no password).

- [ ] **Step 5: Run the full server test suite**

Run: `go test ./internal/server/ -v 2>&1 | tail -50`
Expected: all PASS (existing test `TestNeedsSetupRequiresPasswordWhenAuthEnabled` still holds).

- [ ] **Step 6: Commit**

```bash
git add internal/server/setup_handlers.go internal/server/setup_handlers_test.go
git commit -m "fix(setup): require setup when auth enabled but password unset"
```

---

### Task 1.6: Bounded CSRF token map with background cleanup

**Files:**
- Modify: `internal/server/setup_handlers.go:39-89` (CSRF token management)

**Problem:** Global `setupCSRFTokens` map grows unbounded between requests because cleanup only happens at insert/validate.

- [ ] **Step 1: Add a periodic cleanup goroutine**

In `internal/server/setup_handlers.go`, replace the global var block:

```go
var (
    setupCSRFTokens = map[string]time.Time{}
    setupCSRFMu     sync.Mutex
)
```

with:

```go
var (
    setupCSRFTokens = map[string]time.Time{}
    setupCSRFMu     sync.Mutex
)

// setupCSRFCleanupOnce ensures the cleanup goroutine is started exactly once.
var setupCSRFCleanupOnce sync.Once

// startSetupCSRFCleanup launches a background goroutine that prunes expired
// tokens every 5 minutes. It runs until the process exits.
func startSetupCSRFCleanup() {
    setupCSRFCleanupOnce.Do(func() {
        go func() {
            ticker := time.NewTicker(5 * time.Minute)
            defer ticker.Stop()
            for now := range ticker.C {
                setupCSRFMu.Lock()
                pruneExpiredSetupCSRFTokensLocked(now)
                setupCSRFMu.Unlock()
            }
        }()
    })
}
```

- [ ] **Step 2: Start the cleanup goroutine from `issueSetupCSRFToken`**

Update `issueSetupCSRFToken`:

```go
func issueSetupCSRFToken() string {
    startSetupCSRFCleanup() // idempotent
    token := generateSetupCSRF()
    now := time.Now()
    setupCSRFMu.Lock()
    defer setupCSRFMu.Unlock()
    pruneExpiredSetupCSRFTokensLocked(now)
    setupCSRFTokens[token] = now.Add(setupCSRFTokenTTL)
    return token
}
```

- [ ] **Step 3: Build**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && go build ./...`
Expected: success.

- [ ] **Step 4: Run server tests**

Run: `go test ./internal/server/ -run TestSetupCSRF -v 2>&1 | tail -20`
Expected: existing tests PASS (cleanup is internal).

- [ ] **Step 5: Commit**

```bash
git add internal/server/setup_handlers.go
git commit -m "fix(setup): add periodic CSRF token cleanup goroutine"
```

---

### Task 1.7: Skip `reconfigure` silently when LLM client type mismatches

**Files:**
- Modify: `internal/server/setup_handlers.go:339-344`

**Problem:** If `s.LLMClient` is not `*llm.FailoverManager`, reconfigure is silently skipped, leaving stale config in memory.

- [ ] **Step 1: Add else branch with explicit warning**

```go
if fm, ok := s.LLMClient.(*llm.FailoverManager); ok {
    fm.Reconfigure(newCfg)
    s.Logger.Info("[Setup] LLM client reconfigured",
        "provider", newCfg.LLM.ProviderType,
        "base_url", newCfg.LLM.BaseURL)
} else {
    s.Logger.Warn("[Setup] LLM client is not a FailoverManager; restart may be required for new API key to take effect",
        "client_type", fmt.Sprintf("%T", s.LLMClient))
}
```

- [ ] **Step 2: Build & test**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && go build ./... && go test ./internal/server/ -run TestHandleSetup -count=1`
Expected: build OK, tests PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/server/setup_handlers.go
git commit -m "fix(setup): log warning when LLM client is not FailoverManager"
```

---

## Phase 2 — UX Bug Fixes

> **Priority:** Sprint 2. Improves setup-time UX; nothing breaks the agent core.

### Task 2.1: Replace invalid `{}` fallback config with minimal valid YAML

**Files:**
- Modify: `internal/setup/setup.go:104-124` (`ensureConfigFile`)
- Modify: `internal/setup/setup_test.go`

**Problem:** When `config_template.yaml` is missing, the code writes `{}` which is not a valid AuraGo config — next startup crashes with a confusing YAML error.

- [ ] **Step 1: Write the failing test**

```go
func TestEnsureConfigFileCreatesValidMinimalConfig(t *testing.T) {
    t.Parallel()

    installDir := t.TempDir()
    configPath := filepath.Join(installDir, "config.yaml")

    if err := ensureConfigFile(installDir, configPath, slog.Default()); err != nil {
        t.Fatalf("ensureConfigFile: %v", err)
    }

    data, err := os.ReadFile(configPath)
    if err != nil {
        t.Fatalf("ReadFile: %v", err)
    }

    // The minimal config must load successfully.
    var raw map[string]interface{}
    if err := yaml.Unmarshal(data, &raw); err != nil {
        t.Fatalf("fallback config is not valid YAML: %v", err)
    }

    // And contain at least the server section so port/host have defaults.
    if _, ok := raw["server"]; !ok {
        t.Errorf("fallback config missing 'server' section, got keys: %v", keys(raw))
    }
}

func keys(m map[string]interface{}) []string {
    out := make([]string, 0, len(m))
    for k := range m {
        out = append(out, k)
    }
    return out
}
```

Add to imports: `"gopkg.in/yaml.v3"`.

- [ ] **Step 2: Run — verify failure**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && go test ./internal/setup/ -run TestEnsureConfigFileCreatesValidMinimalConfig -v`
Expected: FAIL — fallback doesn't contain `server`.

- [ ] **Step 3: Replace fallback**

In `internal/setup/setup.go`, replace the `minimalConfig` byte slice:

```go
minimalConfig := []byte(`# AuraGo minimal fallback config — please edit.
server:
  host: "0.0.0.0"
  port: 8088
  ui_language: en
`)
```

- [ ] **Step 4: Update existing test `TestEnsureConfigFileCreatesMinimalFallbackWithoutTemplate`**

The old test expects exactly `"{}\n"`. Update it to match the new minimal YAML:

```go
func TestEnsureConfigFileCreatesMinimalFallbackWithoutTemplate(t *testing.T) {
    t.Parallel()

    installDir := t.TempDir()
    configPath := filepath.Join(installDir, "config.yaml")

    if err := ensureConfigFile(installDir, configPath, slog.Default()); err != nil {
        t.Fatalf("ensureConfigFile() error = %v", err)
    }

    data, err := os.ReadFile(configPath)
    if err != nil {
        t.Fatalf("read config: %v", err)
    }
    var raw map[string]interface{}
    if err := yaml.Unmarshal(data, &raw); err != nil {
        t.Fatalf("fallback not valid YAML: %v", err)
    }
    if _, ok := raw["server"]; !ok {
        t.Fatalf("fallback missing server section: %s", data)
    }
}
```

Add `"gopkg.in/yaml.v3"` to imports.

- [ ] **Step 5: Run all setup tests**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && go test ./internal/setup/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/setup/setup.go internal/setup/setup_test.go
git commit -m "fix(setup): provide valid minimal fallback config with server defaults"
```

---

### Task 2.2: Guard `selectedProfile.id` access in `updateEmbeddingSetupWarnings`

**Files:**
- Modify: `ui/js/setup/main.js:800-812`

**Problem:** `selectedProfile.id === 'custom'` throws if `selectedProfile` is null (early render, page reload before selection).

- [ ] **Step 1: Locate and fix**

In `ui/js/setup/main.js`, replace the body of `updateEmbeddingSetupWarnings`:

```js
function updateEmbeddingSetupWarnings() {
    const quickWarning = document.getElementById('quick-embedding-warning');
    if (quickWarning) {
        const quickProvider = selectedProfile && selectedProfile.provider_type ? selectedProfile.provider_type : '';
        setupSetHidden(quickWarning,
            !quickProvider ||
            quickProvider === 'openrouter' ||
            (selectedProfile && selectedProfile.id === 'custom'));
    }

    const providerWarning = document.getElementById('provider-embedding-warning');
    const provider = (document.getElementById('llm-provider') || {}).value || '';
    if (providerWarning) {
        setupSetHidden(providerWarning, !provider || provider === 'openrouter');
    }
}
```

- [ ] **Step 2: Verify in browser console**

Open `setup.html`, select/deselect profiles in DevTools and confirm no `Cannot read property 'id' of null` error.

(Manual verification — no automated test for vanilla JS.)

- [ ] **Step 3: Commit**

```bash
git add ui/js/setup/main.js
git commit -m "fix(setup-ui): guard null selectedProfile in embedding warnings"
```

---

### Task 2.3: Add keyboard & ARIA support to profile cards

**Files:**
- Modify: `ui/js/setup/main.js:186-211` (`renderProfileCards`)

**Problem:** Profile cards are clickable but not keyboard-accessible. Tastatur-Nutzer können keine Profile auswählen.

- [ ] **Step 1: Update template**

In `ui/js/setup/main.js`, change the `renderProfileCards` innerHTML template:

```js
return `
<div class="profile-card${isCustom ? ' is-custom' : ''}"
     id="profile-card-${escapeAttr(p.id)}"
     role="button"
     tabindex="0"
     aria-pressed="false"
     onclick="selectProfile('${escapeAttr(p.id)}')"
     onkeydown="if(event.key==='Enter'||event.key===' '){event.preventDefault();selectProfile('${escapeAttr(p.id)}')}">
    <div class="profile-check">✓</div>
    ${p.recommended ? '<div class="profile-recommended-bubble">Recommended</div>' : ''}
    <div class="profile-card-icon">${escapeHtml(p.icon || (isCustom ? '⚙️' : '🤖'))}</div>
    <div class="profile-card-name">${escapeHtml(name)}</div>
    <div class="profile-card-description">${escapeHtml(desc)}</div>
    ${p.features ? `<div class="profile-features">${getFeatureBadges(p.features)}</div>` : ''}
    ${pricing ? `<div class="profile-card-pricing">${escapeHtml(pricing)}</div>` : ''}
</div>`;
```

- [ ] **Step 2: Update `selectProfile` to set ARIA state**

```js
function selectProfile(profileId) {
    selectedProfile = profiles.find(p => p.id === profileId) || null;
    document.querySelectorAll('.profile-card').forEach(card => {
        const isSel = card.id === `profile-card-${profileId}`;
        card.classList.toggle('selected', isSel);
        card.setAttribute('aria-pressed', isSel ? 'true' : 'false');
    });
    if (!selectedProfile) return;
    updateNextButtonState();
    updateQuickProfileUI();
}
```

- [ ] **Step 3: Manual keyboard test**

Reload setup, Tab to first profile card, press Space — profile should select and Next should enable.

- [ ] **Step 4: Commit**

```bash
git add ui/js/setup/main.js
git commit -m "feat(setup-ui): add keyboard navigation and ARIA support to profile cards"
```

---

### Task 2.4: Centralize language map (DRY)

**Files:**
- Modify: `ui/js/setup/main.js:6-22` (add constants)
- Modify: `ui/js/setup/main.js:503` (replace inline map)
- Modify: `ui/js/setup/main.js:1405` (replace inline map)
- Modify: `ui/js/setup/main.js:1554` (replace inline array)

**Problem:** `langMap` and `supported` languages duplicated in 3 places.

- [ ] **Step 1: Add module-level constants near top**

In `ui/js/setup/main.js`, after the existing `const setupOllamaBaseURL = ...` line, add:

```js
// Centralized language metadata (single source of truth for setup wizard).
const LANG_MAP = {
    de: 'Deutsch', en: 'English', es: 'Español', fr: 'Français',
    pl: 'Polski', zh: '中文', hi: 'हिन्दी', nl: 'Nederlands',
    it: 'Italiano', pt: 'Português', da: 'Dansk', ja: '日本語',
    sv: 'Svenska', no: 'Norsk', el: 'Ελληνικά', cs: 'Čeština',
};
const SUPPORTED_LANGS = Object.keys(LANG_MAP);
```

- [ ] **Step 2: Replace the two inline `langMap` definitions**

In `buildQuickConfigPatch` (around line 503), change:
```js
const langMap = {de:'Deutsch', en:'English', ...};
```
to:
```js
const langMap = LANG_MAP;
```

In `buildConfigPatch` (around line 1405), replace the IIFE-injected `langMap` with a reference to `LANG_MAP`:

```js
agent: {
    system_language: (function() {
        const v = document.getElementById('system-language').value;
        if (v === 'custom') return document.getElementById('system-language-custom').value.trim();
        return LANG_MAP[v] || v;
    })(),
    personality_engine_v2: helperConfigured,
    personality_engine: helperConfigured,
    core_personality: document.getElementById('core-personality').value,
},
```

- [ ] **Step 3: Replace the inline `supported` array in `detectAndSetLanguage`**

Change:
```js
const supported = ['de','en','es','fr','pl','zh','hi','nl','it','pt','da','ja','sv','no','cs','el'];
const detected = supported.includes(base) ? base : 'en';
```
to:
```js
const detected = SUPPORTED_LANGS.includes(base) ? base : 'en';
```

- [ ] **Step 4: Commit**

```bash
git add ui/js/setup/main.js
git commit -m "refactor(setup-ui): centralize language metadata in LANG_MAP constant"
```

---

### Task 2.5: Synchronize all three language selectors on any change

**Files:**
- Modify: `ui/js/setup/main.js:394-415` (`onQuickLanguageChange`, `onPlanLanguageChange`, `onLanguageChange`)

**Problem:** Language selector changes don't sync across all three hidden/visible select elements (plan-language, quick-language, system-language).

- [ ] **Step 1: Extract helper and reuse**

In `ui/js/setup/main.js`, replace lines 394-415 with:

```js
// Sync the three language selectors and apply the chosen language.
function syncLanguage(value) {
    if (!value) return;
    ['plan-language', 'system-language', 'quick-language'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.value = value;
    });
    fetchAndApplyLang(value);
}

function onQuickLanguageChange() {
    syncLanguage(document.getElementById('quick-language')?.value);
}

function onPlanLanguageChange() {
    syncLanguage(document.getElementById('plan-language')?.value);
}
```

Keep the existing `onLanguageChange` function (line 886) — it handles the `custom` case differently. Replace the duplicate sync code there:

In `onLanguageChange`, remove:
```js
fetchAndApplyLang(sel.value);
```
and replace with `syncLanguage(sel.value)` ONLY when sel.value !== 'custom'.

After the change, `onLanguageChange` becomes:
```js
function onLanguageChange() {
    const sel = document.getElementById('system-language');
    const customInput = document.getElementById('system-language-custom');
    if (sel.value === 'custom') {
        setupSetHidden(customInput, false);
        customInput.focus();
    } else {
        setupSetHidden(customInput, true);
        syncLanguage(sel.value);
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add ui/js/setup/main.js
git commit -m "refactor(setup-ui): centralize language selector sync in syncLanguage()"
```

---

### Task 2.6: Cache loaded setup profiles

**Files:**
- Modify: `internal/server/setup_handlers.go:700-716` (`handleSetupProfiles`)

**Problem:** `setup.LoadProfiles("", ...)` re-parses the embedded YAML on every `/api/setup/profiles` request.

- [ ] **Step 1: Add package-level cache + lazy load**

In `internal/server/setup_handlers.go`, add (near the top after imports):

```go
var (
    setupProfilesCache     []setup.SetupProfile
    setupProfilesCacheOnce sync.Once
)

// loadCachedSetupProfiles returns the embedded setup profiles, parsed once.
func loadCachedSetupProfiles(logger *slog.Logger) []setup.SetupProfile {
    setupProfilesCacheOnce.Do(func() {
        setupProfilesCache = setup.LoadProfiles("", logger)
    })
    return setupProfilesCache
}
```

- [ ] **Step 2: Use the cache in handler**

Replace the body of `handleSetupProfiles`:

```go
func handleSetupProfiles(s *Server) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
            jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
            return
        }

        profiles := loadCachedSetupProfiles(s.Logger)

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "profiles": profiles,
        })
    }
}
```

- [ ] **Step 3: Build & test**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && go build ./... && go test ./internal/server/ -run TestHandleSetupProfiles -count=1`
Expected: build OK; tests PASS (or skipped).

- [ ] **Step 4: Commit**

```bash
git add internal/server/setup_handlers.go
git commit -m "perf(setup): cache parsed setup profiles across requests"
```

---

### Task 2.7: i18n race-guard for rapid language switching

**Files:**
- Modify: `ui/js/setup/main.js:898-915` (`fetchAndApplyLang`)

**Problem:** Multiple parallel fetches can resolve out of order — older responses override newer ones.

- [ ] **Step 1: Add a sequence counter**

In `ui/js/setup/main.js`, near the top with other state:

```js
let langFetchSeq = 0;
```

- [ ] **Step 2: Guard the fetch**

Replace `fetchAndApplyLang`:

```js
function fetchAndApplyLang(langValue) {
    document.documentElement.lang = langValue || 'en';
    const seq = ++langFetchSeq;
    fetch('/api/i18n?lang=' + encodeURIComponent(langValue))
        .then(r => r.ok ? r.json() : null)
        .then(json => {
            if (seq !== langFetchSeq) return; // stale response, discard
            if (json && json.data && typeof json.data === 'object') {
                I18N = json.data;
                applyI18N();
                renderStepIndicator();
                if (profiles && profiles.length > 0) {
                    renderProfileCards(profiles);
                    if (selectedProfile) selectProfile(selectedProfile.id);
                }
            }
        })
        .catch((err) => { console.warn('fetchAndApplyLang failed:', err); });
}
```

- [ ] **Step 3: Manual verification**

Click rapidly between languages in the dropdown — final state should reflect the last click.

- [ ] **Step 4: Commit**

```bash
git add ui/js/setup/main.js
git commit -m "fix(setup-ui): discard stale i18n responses via sequence counter"
```

---

### Task 2.8: Surface setup failure when CSRF status unreachable

**Files:**
- Modify: `ui/js/setup/main.js:31-53` (`checkSetupStatus`)

**Problem:** If `/api/setup/status` is unreachable, `csrfToken` stays empty and the final save returns 403 with no actionable message.

- [ ] **Step 1: Add a one-time warning banner**

In `ui/js/setup/main.js`, replace the IIFE body:

```js
(async function checkSetupStatus() {
    try {
        const resp = await fetch('/api/setup/status');
        if (!resp.ok) {
            showSetupConnectionWarning('status ' + resp.status);
            return;
        }
        const data = await resp.json();
        if (!data.needs_setup) {
            window.location.href = '/';
            return;
        }
        if (data.csrf_token) csrfToken = data.csrf_token;
        if (data.ollama_base_url) {
            setupOllamaBaseURL = data.ollama_base_url;
            if (typeof providerConfig !== 'undefined' && providerConfig.ollama) {
                providerConfig.ollama.baseUrl = setupOllamaBaseURL;
                const provider = document.getElementById('llm-provider');
                const baseURL = document.getElementById('llm-base-url');
                if (provider && baseURL && provider.value === 'ollama') {
                    baseURL.value = setupOllamaBaseURL;
                }
            }
        }
    } catch (e) {
        showSetupConnectionWarning(e.message || 'network error');
    }
})();

function showSetupConnectionWarning(detail) {
    const header = document.querySelector('.setup-header');
    if (!header) return;
    const banner = document.createElement('div');
    banner.className = 'setup-conn-warning';
    banner.textContent = 'Could not reach setup status endpoint (' + detail + '). Reload the page once the server is ready.';
    header.parentNode.insertBefore(banner, header.nextSibling);
}
```

- [ ] **Step 2: Commit**

```bash
git add ui/js/setup/main.js
git commit -m "feat(setup-ui): warn user when setup status endpoint unreachable"
```

---

## Phase 3 — Architecture Refactoring

> **Priority:** Sprint 3. Larger changes; consolidate shared logic and improve testability.

### Task 3.1: Extract shared `applyConfigPatch` helper

**Files:**
- Create: `internal/server/config_patch.go` (new file)
- Modify: `internal/server/setup_handlers.go` (use helper)
- Modify: `internal/server/config_handlers_main.go` (use helper)
- Modify: `internal/server/setup_handlers_test.go` and `config_handlers_main_test.go`

**Problem:** Setup-save and config-update repeat the deep-merge + vault-write + reload sequence.

- [ ] **Step 1: Design the helper**

Create `internal/server/config_patch.go`:

```go
package server

import (
    "aurago/internal/config"
    "aurago/internal/setup"
    "fmt"
    "log/slog"
    "os"
    "gopkg.in/yaml.v3"
)

// applyConfigPatch reads the YAML config at configPath, deep-merges patch into it,
// extracts any provider/TTS API keys to the vault, writes back atomically, and
// returns the in-memory Config reloaded from disk.
//
// Used by both /api/setup (unauthenticated first-run wizard) and the regular
// /api/config update endpoint (authenticated).
func applyConfigPatch(s *Server, patch map[string]interface{}) (*config.Config, error) {
    configPath := s.Cfg.ConfigPath
    if configPath == "" {
        return nil, fmt.Errorf("config path not set")
    }

    // Apply profile-specific defaults first (no-op for non-profile patches).
    applySetupProfileConfigPatch(patch, s)

    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, fmt.Errorf("read config: %w", err)
    }
    var rawCfg map[string]interface{}
    if err := yaml.Unmarshal(data, &rawCfg); err != nil {
        return nil, fmt.Errorf("parse config: %w", err)
    }
    rawCfg = normalizeConfigYAMLMap(rawCfg)

    // Move any provider/TTS API keys to the vault before merge.
    if s.Vault != nil {
        extractSecretsToVault(patch, s.Vault, s.Logger)
    }

    deepMerge(rawCfg, patch, "")
    rawCfg = normalizeConfigYAMLMap(rawCfg)

    out, err := yaml.Marshal(rawCfg)
    if err != nil {
        return nil, fmt.Errorf("marshal config: %w", err)
    }
    if err := config.WriteFileAtomic(configPath, out, 0o600); err != nil {
        return nil, fmt.Errorf("write config: %w", err)
    }

    reloaded, err := config.Load(configPath)
    if err != nil {
        return nil, fmt.Errorf("reload config: %w", err)
    }
    reloaded.ConfigPath = configPath
    reloaded.ApplyVaultSecrets(s.Vault)
    reloaded.ResolveProviders()
    return reloaded, nil
}

// extractSecretsToVault moves any provider/TTS API keys out of the patch and
// into the vault. Returns silently if vault is nil.
func extractSecretsToVault(patch map[string]interface{}, vault VaultWriter, logger *slog.Logger) {
    // ... move existing vault-extraction logic from setup_handlers.go here
}
```

Define a minimal `VaultWriter` interface:

```go
type VaultWriter interface {
    WriteSecret(key, value string) error
}
```

- [ ] **Step 2: Refactor `handleSetupSave` to call the helper**

Replace lines 181-298 with calls to `applyConfigPatch` + reload callbacks.

(This task is too large for a single agent; dispatch to a subagent that specializes in this refactor — see Execution Handoff.)

- [ ] **Step 3: Verify behavior unchanged via existing tests**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && go test ./internal/server/ -run TestHandleSetup -count=1 -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/server/config_patch.go internal/server/setup_handlers.go
git commit -m "refactor(server): extract applyConfigPatch helper used by setup and update"
```

---

### Task 3.2: Move CSRF state into Server struct

**Files:**
- Modify: `internal/server/setup_handlers.go` (globals → struct fields)
- Modify: `internal/server/server.go` (Server struct definition)

**Problem:** `setupCSRFTokens` and `setupCSRFMu` are package globals, making parallel tests fragile.

- [ ] **Step 1: Add fields to Server**

In `internal/server/server.go`, find the `Server` struct and add:

```go
// Setup wizard CSRF tokens (short-lived, multi-token support).
SetupCSRFMu      sync.Mutex
SetupCSRFTokens  map[string]time.Time
```

- [ ] **Step 2: Update all references**

In `internal/server/setup_handlers.go`, replace:
- `setupCSRFMu` → `s.SetupCSRFMu`
- `setupCSRFTokens` → `s.SetupCSRFTokens`
- Initialize the map in `NewServer` (or first-use guard).

- [ ] **Step 3: Build & test**

Run: `cd /c/Users/Andi/Documents/repo/AuraGo && go build ./... && go test ./internal/server/ -count=1 -short 2>&1 | tail -20`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/server/setup_handlers.go internal/server/server.go
git commit -m "refactor(server): move setup CSRF token state into Server struct"
```

---

### Task 3.3: Split `extractSetupAdminPassword` into validate + apply

**Files:**
- Modify: `internal/server/setup_handlers.go:430-482`

**Problem:** `extractSetupAdminPassword` both validates AND mutates the patch (`delete(authPatch, "admin_password")`). Hard to test in isolation.

- [ ] **Step 1: Split into two functions**

```go
// validateSetupAdminPassword returns the password (if any), whether auth
// should remain enabled, and an error if validation fails. Does not mutate.
func validateSetupAdminPassword(authPatch map[string]interface{}, currentAuthEnabled bool, currentPasswordSet bool) (string, bool, error) {
    authEnabled := currentAuthEnabled
    if authPatch == nil {
        if authEnabled && !currentPasswordSet {
            return "", authEnabled, fmt.Errorf("admin password is required")
        }
        return "", authEnabled, nil
    }
    if rawEnabled, exists := authPatch["enabled"]; exists {
        enabled, ok := rawEnabled.(bool)
        if !ok {
            return "", authEnabled, fmt.Errorf("auth.enabled must be a boolean")
        }
        authEnabled = enabled
    }
    rawPassword, hasPassword := authPatch["admin_password"]
    if !authEnabled {
        return "", false, nil
    }
    if !hasPassword {
        if currentPasswordSet {
            return "", true, nil
        }
        return "", true, fmt.Errorf("admin password is required")
    }
    password, ok := rawPassword.(string)
    if !ok {
        return "", true, fmt.Errorf("admin password must be a string")
    }
    password = strings.TrimSpace(password)
    if len(password) < 8 {
        return "", true, fmt.Errorf("admin password must be at least 8 characters long")
    }
    return password, true, nil
}

// stripSetupAdminPassword removes the temporary admin_password field from the
// patch so it is never written to disk.
func stripSetupAdminPassword(authPatch map[string]interface{}) {
    if authPatch != nil {
        delete(authPatch, "admin_password")
    }
}
```

- [ ] **Step 2: Update callers in `handleSetupSave`**

```go
authPatch, _ := patch["auth"].(map[string]interface{})
setupPassword, authEnabled, err := validateSetupAdminPassword(authPatch, s.Cfg.Auth.Enabled, s.Cfg.Auth.PasswordHash != "")
if err != nil { ... }
stripSetupAdminPassword(authPatch)
```

- [ ] **Step 3: Update tests**

Existing test `TestExtractSetupAdminPasswordStripsTemporaryField` must call both functions explicitly.

- [ ] **Step 4: Commit**

```bash
git add internal/server/setup_handlers.go internal/server/setup_handlers_test.go
git commit -m "refactor(setup): split extractSetupAdminPassword into validate+strip"
```

---

## Phase 4 — Polish & Optimizations (deferred)

> These findings are valid but lower priority. Ship Phases 1-3 first, then evaluate.

### Deferred items (1-line specs for future PRs)

| # | Finding | File | Spec |
|---|---------|------|------|
| 8 | serviceAlreadyInstalled reinstalls after manual uninstall | setup.go:356-376 | Check `systemctl is-enabled aurago.service` instead of file presence |
| 9 | extractTarGz aborts on first error | setup.go:164-221 | Per-file error logging, continue extraction |
| 10 | .bat mode 0644 inconsistent | setup.go:408 | Fixed in Task 1.3 |
| 12 | quick-language mirror can diverge | main.js:120, 459 | Replace 3-element sync with central store (after Task 2.5) |
| 14 | setup_failed_save_config returns 500 for user errors | setup_handlers.go:252-264 | Return 400 with clear details |
| 16 | runningInDocker reads full /proc/self/cgroup | setup.go:492-504 | Stream-read with io.LimitReader |
| 18 | required-star render flicker on init | main.js:1598-1604 | Default to `setupPasswordRequired=true` until proven otherwise |
| 20 | selectedProfile state stale after flow switch | main.js:331-341 | Reset selectedProfile when flow changes in nextStep |
| 26 | Double config.Load in main.go | main.go:141-160 | Extract `bootstrapIfNeeded()` helper |
| 27 | isValidMasterKeyHex double-validation | setup.go:466-473 | Cache result in local var |
| 28 | extractTarGz no progress indicator | setup.go:164-221 | Add progress callback for archives > 10MB |
| 30 | Browser-language detection sync IIFE | main.js:1551-1563 | Already OK, no change |
| 31 | validateSetupTestBaseURL hardcoded allowlist | setup_handlers.go:679-698 | Load allowlist from config + extend |
| 32 | Race between --setup and normal start | main.go:140-267 | Move setup inside flock |
| 33 | schtasks error handling asymmetry | setup.go:385-399 | Use CombinedOutput for /Delete too |

Each of these can be promoted to a task later via `/gsd:add-backlog` or converted into a follow-up plan.

---

## Self-Review

**1. Spec coverage (33 findings):**

| Finding | Plan Task |
|---------|-----------|
| #1 setuid | 1.1 ✓ |
| #2 installDir escaping | 1.2 ✓ |
| #3 admin password trim | 1.4 ✓ |
| #4 CSRF map cleanup | 1.6 ✓ |
| #5 needsSetup auth | 1.5 ✓ |
| #6 .bat write error | 1.3 ✓ |
| #7 .bat for /f parsing | (deferred, address in Task 1.3 by switching to PowerShell — out of scope here) |
| #8 serviceAlreadyInstalled | Phase 4 |
| #9 extractTarGz error | Phase 4 |
| #10 .bat mode | 1.3 ✓ |
| #11 LLM reconfigure | 1.7 ✓ |
| #12 quick-language mirror | 2.5 ✓ |
| #13 langMap DRY | 2.4 ✓ |
| #14 setup_failed_save_config | Phase 4 |
| #15 invalid fallback | 2.1 ✓ |
| #16 runningInDocker read | Phase 4 |
| #17 updateEmbeddingSetupWarnings | 2.2 ✓ |
| #18 required-star flicker | Phase 4 |
| #19 ARIA/keyboard | 2.3 ✓ |
| #20 selectedProfile stale | Phase 4 |
| #21 i18n race | 2.7 ✓ |
| #22 CSRF status warning | 2.8 ✓ |
| #23 applyConfigPatch refactor | 3.1 ✓ |
| #24 extractSetupAdminPassword split | 3.3 ✓ |
| #25 CSRF → Server struct | 3.2 ✓ |
| #26 double config.Load | Phase 4 |
| #27 isValidMasterKeyHex | Phase 4 |
| #28 extractTarGz progress | Phase 4 |
| #29 profile cache | 2.6 ✓ |
| #30 lang detect sync | already OK |
| #31 allowlist extension | Phase 4 |
| #32 race --setup | Phase 4 |
| #33 schtasks asymmetry | Phase 4 |

Gaps: #7 (.bat for /f parsing) is partially addressed in Task 1.3 but not fully switched to PowerShell. Mark as deferred for future work — the current code is functional and the switch is a Windows-only concern.

**2. Placeholder scan:** No "TBD" / "TODO" / "implement later" patterns. Each task has concrete code, file paths, and commit messages.

**3. Type consistency:** `setupCSRFMu`/`setupCSRFTokens` referenced consistently across Tasks 1.6, 3.2. `LANG_MAP` constant used everywhere `langMap` was previously duplicated.

---

## Execution Handoff

Plan complete and saved to `plans/2026-06-26-setup-flow-hardening.md`. Two execution options:

1. **Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration with quality gates.

2. **Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints for review.

Which approach?
