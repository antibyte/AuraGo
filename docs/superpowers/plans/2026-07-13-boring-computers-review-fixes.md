# Boring Computers Review Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Project policy forbids subagents unless the user explicitly requests them. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove every security, availability, lifecycle, rollback, host-isolation, and token-compatibility defect found in the managed Boring Computers review.

**Architecture:** Keep the authenticated AuraGo reverse proxy, but enforce method scopes and read-only policy before forwarding. Separate passive health observation from tunnel creation, make auto-setup generation-aware and reconcile queued configuration changes, and install the web runtime into immutable unique releases with a private Node.js runtime and durable revision marker.

**Tech Stack:** Go 1.26.1+, `net/http`, `httputil.ReverseProxy`, `context`, SSH, systemd, Bash, Node.js, Go tests.

## Global Constraints

- `BORING_TOKEN` remains Vault-only and server-side.
- Ports `18080` and `18081` remain loopback-only.
- Disabled and read-only configuration must be enforced at AuraGo's proxy boundary.
- Drawer and status endpoints must never initiate an unbounded SSH dial.
- Auto-setup must converge to the newest config/revision without overlapping installers.
- Releases must be immutable and rollback-capable when reinstalling the same upstream revision.
- Managed Node.js must not replace host-global `node`, `npm`, or `npx` links.
- Every production change requires a failing test first, GitNexus impact analysis, and a focused commit.

---

### Task 1: Proxy Authorization and Passive Health

**Files:**
- Modify: `internal/server/virtual_computers_management.go`
- Modify: `internal/server/virtual_computers_management_test.go`
- Modify: `internal/server/virtual_computers_handlers.go`
- Test: `internal/server/virtual_computers_management_test.go`

**Interfaces:**
- Produces method-aware authorization through `desktopMethodScope` and passive `virtualComputersManagementHealthy`.
- Keeps tunnel creation exclusively in `virtualComputersEnsureManagementAccess` for actual proxy access/background setup.

- [ ] Add tests proving a read-scoped token cannot POST, read-only blocks mutations, GET remains available, and passive health never calls the SSH executor/tunnel starter.
- [ ] Run the focused tests and observe permission/tunnel assertions fail.
- [ ] Enforce `desktopMethodScope(r.Method)`, reject mutations when `cfg.ReadOnly`, and change component health to bounded HTTP probes only.
- [ ] Change setup status control-plane health to `virtualComputersHealthOK` so status never creates an SSH tunnel.
- [ ] Run focused and full server tests, detect changes, and commit `fix: enforce Boring Computers proxy boundaries`.

### Task 2: Convergent Auto-Setup Lifecycle

**Files:**
- Modify: `internal/server/virtual_computers_auto_setup.go`
- Modify: `internal/server/virtual_computers_auto_setup_test.go`
- Modify: `internal/server/config_handlers_main.go`
- Modify: `internal/server/space_agent_handlers.go`
- Modify: `internal/virtualcomputers/management.go`
- Modify: `internal/virtualcomputers/management_test.go`

**Interfaces:**
- Produces a generation-aware reconciler that queues the newest config while a run is active.
- Produces `ManagementRevisionURL` and a passive installed-revision probe.
- Disabling closes the management tunnel, invalidates drawer cache, and prevents a stale queued reinstall.

- [ ] Add tests for config change during a running setup, disable cleanup, cache invalidation, and healthy-but-outdated revision requiring setup.
- [ ] Run focused tests and observe missing convergence/revision behavior.
- [ ] Replace the drop-on-running state with desired-generation/config state and a reconciliation loop.
- [ ] On disable, clear queued work, close the management tunnel, and clear the drawer cache.
- [ ] Compare the installed revision endpoint/marker with `PinnedUpstreamRevision` before skipping setup.
- [ ] Run focused server/virtualcomputer tests, detect changes, and commit `fix: converge Boring Computers auto provisioning`.

### Task 3: Immutable Installer and Private Runtime

**Files:**
- Modify: `internal/virtualcomputers/setup.go`
- Modify: `internal/virtualcomputers/setup_test.go`
- Modify: `internal/virtualcomputers/setup_web.go`
- Modify: `internal/virtualcomputers/setup_web_test.go`

**Interfaces:**
- Consumes the resolved `SetupManager` token.
- Produces unique immutable release paths, atomic `current` replacement, functional rollback, a revision marker/endpoint, and `${INSTALL_DIR}/runtime/node` without global symlink replacement.

- [ ] Add tests proving token fallback reaches both services, release paths are unique, the active release is never removed, rollback targets a distinct path, and no `/usr/local/bin/node|npm|npx` link is written.
- [ ] Run focused tests and observe all new contract assertions fail.
- [ ] Pass the resolved token into `managementInstallScript`.
- [ ] Stage to a unique release directory, atomically replace `current` with `ln -sfnT`, retain the previous release for rollback, and prune only inactive old releases after success.
- [ ] Install Node.js below `${INSTALL_DIR}/runtime/node` and use its absolute binaries for build/service execution.
- [ ] Write the pinned revision into the release and expose it through the management service for auto-setup comparison.
- [ ] Run package tests and secret scans, detect changes, and commit `fix: harden Boring Computers managed releases`.

### Task 4: Regression Verification and Documentation

**Files:**
- Modify: `documentation/virtual_computers.md`
- Modify: `prompts/tools_manuals/virtual_computers.md`

**Interfaces:**
- Documents authorization, passive status, revision reconciliation, disable behavior, private runtime, and rollback semantics.

- [ ] Update operator and agent documentation.
- [ ] Run `gofmt` on changed Go files.
- [ ] Run `go test ./internal/virtualcomputers ./internal/server ./ui -count=1`.
- [ ] Run `go vet ./internal/virtualcomputers ./internal/server`.
- [ ] Run `go test ./... -count=1` and build `./cmd/aurago` into `disposable/`.
- [ ] Run secret scans, `git diff --check`, and GitNexus compare against `main`.
- [ ] Commit `docs: document hardened Boring Computers lifecycle` and confirm a clean worktree.
