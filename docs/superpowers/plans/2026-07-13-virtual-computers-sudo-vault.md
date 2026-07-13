# Virtual Computers Sudo Vault Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reuse AuraGo's persistent `sudo_password` Vault secret for local Boring Computers preflight and installation, with a Virtual Computers config field that recognizes an already stored password.

**Architecture:** Extend `LocalCommandExecutor` with an optional stdin-only sudo credential while preserving root and passwordless-sudo paths. The server injects the existing Vault secret only for `local_host`, exposes only a boolean stored state, and the UI uses the authenticated Vault API to save or clear the shared key.

**Tech Stack:** Go 1.26.1+, `os/exec`, AuraGo Vault, vanilla JavaScript, JSON i18n, Go tests.

## Global Constraints

- Reuse exactly `sudo_password`; never create a per-integration sudo secret.
- Never serialize the password into YAML, API responses, command arguments, environment variables, generated scripts, audit payloads, or logs.
- Pass password-based sudo credentials only through stdin with `sudo -S -p ""`.
- Keep `agent.sudo_enabled` independent and keep `ssh_host` behavior unchanged.
- Translate cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, and zh.
- Run GitNexus impact before production symbol edits and `detect_changes` before every commit.

---

### Task 1: Password-Capable Local Executor

**Files:**
- Modify: `internal/virtualcomputers/local_executor.go`
- Modify: `internal/virtualcomputers/setup.go`
- Test: `internal/virtualcomputers/setup_test.go`

**Interfaces:**
- Consumes: an optional password already read from the Vault.
- Produces: `LocalCommandExecutor.SudoPassword string`, `LocalCommandExecutor.InputCommandRunner`, and `SetupManager.SudoPassword string`.

- [ ] **Step 1: Analyze impact**

Run GitNexus impact for `LocalCommandExecutor`, `hasSudoOrRoot`, `RunScript`, and `SetupManager.RedactInstallLog`. Warn before editing on HIGH/CRITICAL risk.

- [ ] **Step 2: Write failing tests**

Add executor tests with this wished-for boundary:

```go
executor := LocalCommandExecutor{
	RuntimeGOOS: "linux",
	EffectiveUID: func() int { return 1000 },
	SudoPassword: "vault-sudo-secret",
	CommandRunner: func(context.Context, string, ...string) (string, error) {
		return "", errors.New("passwordless sudo denied")
	},
	InputCommandRunner: func(_ context.Context, name, input string, args ...string) (string, error) {
		if name != "sudo" || input != "vault-sudo-secret\n" || !reflect.DeepEqual(args, []string{"-S", "-p", "", "true"}) {
			t.Fatalf("name=%q input=%q args=%v", name, input, args)
		}
		return "", nil
	},
}
```

Assert preflight succeeds, script execution uses `sudo -S -p "" bash <temp>`, no argument contains the password, the temp file is removed, and `SetupManager.RedactInstallLog` removes the password.

- [ ] **Step 3: Verify RED**

Run `go test ./internal/virtualcomputers -run 'TestLocalCommandExecutor.*VaultSudo|TestSetupManagerRedactsSudoPassword' -count=1`.

Expected: compilation fails because the new fields do not exist.

- [ ] **Step 4: Implement minimal stdin support**

Add:

```go
type InputCommandRunner func(ctx context.Context, name, input string, args ...string) (string, error)
```

The default implementation creates `exec.CommandContext`, assigns `strings.NewReader(input)` to `cmd.Stdin`, and captures combined output. Non-root elevation order is `sudo -n`, then `sudo -S -p ""` only when a trimmed password exists. Add the password to `SetupManager` solely so `RedactInstallLog` can scrub it.

- [ ] **Step 5: Verify GREEN**

Run `gofmt` on the three files and `go test ./internal/virtualcomputers -count=1`.

- [ ] **Step 6: Detect and commit**

Run GitNexus `detect-changes`, then commit as `feat: support Vault sudo password for local setup`.

---

### Task 2: Server Vault Injection and Stored-State API

**Files:**
- Modify: `internal/server/virtual_computers_handlers.go`
- Test: `internal/server/virtual_computers_handlers_test.go`
- Test: `internal/server/virtual_computers_auto_setup_test.go`

**Interfaces:**
- Consumes: Task 1 executor and redaction fields.
- Produces: `sudo_password_stored bool` and Vault-populated local setup managers.

- [ ] **Step 1: Analyze impact**

Run GitNexus impact for `virtualComputersSetupExecutor`, `virtualComputersSetupManager`, and `handleVirtualComputersSetupStatus`.

- [ ] **Step 2: Write failing tests**

Create a temporary `security.Vault`, store `sudo_password`, and assert:

```go
manager, err := virtualComputersSetupManager(server, localCfg, "boring-token")
executor := manager.Executor.(virtualcomputers.LocalCommandExecutor)
if executor.SudoPassword != "vault-sudo-secret" || manager.SudoPassword != "vault-sudo-secret" {
	t.Fatal("local setup did not reuse central sudo_password")
}
```

Add a remote-mode test proving the SSH path does not consume it. Add status tests for true/false `sudo_password_stored` and assert the response never includes the secret value.

- [ ] **Step 3: Verify RED**

Run `go test ./internal/server -run 'TestVirtualComputers.*SudoPassword' -count=1`.

- [ ] **Step 4: Implement local-only lookup**

Add an internal helper:

```go
func virtualComputersSudoPassword(s *Server) string {
	if s == nil || s.Vault == nil { return "" }
	value, err := s.Vault.ReadSecret("sudo_password")
	if err != nil { return "" }
	return strings.TrimSpace(value)
}
```

Populate the local executor and manager only in `local_host`. Add only `sudo_password_stored: virtualComputersSudoPassword(s) != ""` to status.

- [ ] **Step 5: Verify GREEN**

Run `gofmt` and `go test ./internal/server -run 'TestVirtualComputers' -count=1`.

- [ ] **Step 6: Detect and commit**

Run GitNexus `detect-changes`, then commit as `feat: inject Vault sudo password into Virtual Computers setup`.

---

### Task 3: Config UI and Translations

**Files:**
- Modify: `ui/cfg/virtual_computers.js`
- Modify: `ui/config_virtual_computers_test.go`
- Modify: `ui/lang/config/virtual_computers/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`
- Modify: `documentation/virtual_computers.md`
- Modify: `prompts/tools_manuals/virtual_computers.md`

**Interfaces:**
- Consumes: Task 2 boolean and existing `POST/DELETE /api/vault/secrets`.
- Produces: a local-host-only Vault field for the central key.

- [ ] **Step 1: Write failing UI tests**

Assert the JS contains `vc-sudo-password-input`, the exact key `sudo_password`, status refresh through `/api/virtual-computers/setup/status`, and DELETE through `/api/vault/secrets?key=`. Assert the field is inside the `isLocalHost` branch and that no `data-path`, `configData`, or `AuraConfigState` write includes `sudo_password`. Check the new translation keys in all 16 files.

- [ ] **Step 2: Verify RED**

Run `go test ./ui -run 'TestVirtualComputers.*Sudo' -count=1`.

- [ ] **Step 3: Implement Save, Clear, and status**

Render an empty password input plus Save/Clear buttons only for local mode. `vcCfgRefreshSudoPasswordStatus()` reads only the boolean. Saving posts `{key:'sudo_password', value}`; clearing first awaits the existing translated `showConfirm(...)` modal and then uses `DELETE ?key=sudo_password`. Make `vcCfgSaveSecret` return its Promise and skip `cfgMarkSecretStored` when no config path is supplied. Never preload or copy the password into config state; do not use native `alert()` or `confirm()`.

- [ ] **Step 4: Add translations and docs**

Add these keys in all languages:

```text
config.virtual_computers.sudo_password_label
config.virtual_computers.sudo_password_stored
config.virtual_computers.sudo_password_missing
config.virtual_computers.sudo_password_saved
config.virtual_computers.sudo_password_cleared
config.virtual_computers.sudo_password_clear
config.virtual_computers.sudo_password_clear_confirm
config.virtual_computers.sudo_password_save_failed
help.virtual_computers.sudo_password
```

German uses personal form and native characters. Document `/sudopwd` reuse, stdin-only delivery, and independence from `agent.sudo_enabled`.

- [ ] **Step 5: Verify GREEN**

Run `go test ./ui ./internal/virtualcomputers ./internal/server -count=1`.

- [ ] **Step 6: Detect and commit**

Run GitNexus `detect-changes`, then commit as `feat: add Virtual Computers sudo Vault field`.

---

### Task 4: Full Verification

**Files:** Verify only.

**Interfaces:** Consumes Tasks 1-3 and produces merge-ready evidence.

- [ ] Run `go vet ./internal/virtualcomputers ./internal/server ./ui`.
- [ ] Run `go test ./... -count=1`.
- [ ] Build `./cmd/aurago` to `disposable/aurago-sudo-vault-check.exe`, then remove it.
- [ ] Scan the branch diff for plaintext credentials and forbidden serialization.
- [ ] Run `git diff --check` and GitNexus compare against `main`.
- [ ] Confirm `git status --short` is clean.
