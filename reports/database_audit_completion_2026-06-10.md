# Database Audit — Completion Report

**Date:** 2026-06-10  
**Repository:** AuraGo (`main`)  
**Scope:** Database/code audit follow-up (P0–P2) plus prior maintenance fixes

---

## Executive Summary

All prioritized database audit items (P0, P1, P2) are implemented, tested, and pushed to `origin/main`. Backup coverage, SQLite open paths, agent filesystem protection, shutdown hygiene, inventory query performance, and dead TrueNAS persistence code are now aligned with the actual runtime architecture (chromem LTM in `directories.vectordb_dir`).

---

## Commits (audit remediation)

| Priority | Commit     | Summary |
|----------|------------|---------|
| P0       | `2ff376370` | Complete SQLite backup list; deprecate `long_term_path`; shared `config.SQLiteDatabasePaths()` |
| P1       | `1cccfd880` | Persistent `system_tasks` connection; `dbutil` integrity fail-closed; unify opens via `dbutil.Open` |
| P2       | `b5e1122ec` | Server shutdown DB close; inventory indexes (schema v4); remove dead TrueNAS registry |
| Follow-up | `ad7476d62` | `system_tasks` shutdown/test cleanup; audit manifest + appendix doc fixes |

---

## P0 — Backup list & `long_term_path` drift

### Problem
- Backup handler used an incomplete hardcoded SQLite list (missing `launchpad`, `virtual_desktop`, `system_tasks`, `galaxa`, `desktop_store`).
- `sqlite.long_term_path` documented/used as LTM, but runtime LTM is chromem at `directories.vectordb_dir`.

### Resolution
- Added `internal/config/sqlite_paths.go`:
  - `SQLiteDatabasePaths()` — single source of truth for backups
  - `SQLiteProtectedPaths()` — WAL/SHM sidecars for agent filesystem protection
- `backup_handlers.go` uses shared path helper.
- `agent_loop_helpers.go` protects all configured SQLite DBs (not only STM/inventory/invasion).
- Legacy `long_term.db` included in backup **only when the file exists**.
- Docs/template updated: `long_term_path` marked deprecated; `launchpad_path` added to `config_template.yaml`.

### Verification
- `internal/config/sqlite_paths_test.go`
- Extended `TestHandleBackupCreateIncludesRuntimeFilesAndConsistentSQLiteSnapshots`

---

## P1 — SQLite hardening

### Problem
- `system_tasks_store` opened/closed SQLite on every `load`/`save`.
- `dbutil.runIntegrityCheck` swallowed query errors (not fail-closed).
- Several production paths bypassed `dbutil.Open` (mission history, galaxa, virtual desktop).

### Resolution
- `system_tasks_store` keeps one persistent `*sql.DB` with mutex; proper `close()`.
- Integrity check errors now fail open (consistent with `HealthCheck`).
- Migrated to `dbutil.Open`: `mission_history.go`, `galaxa_handlers.go`, `desktop/service.go`, `desktop/service_media.go`.

### Verification
- `internal/dbutil/open_test.go`
- `internal/tools/system_tasks_store_test.go`
- Existing cron/background_tasks/mission_history/desktop tests pass.

### Follow-up (`ad7476d62`)
Post-audit review found incomplete shutdown/test hygiene after the persistent-connection change:
- Ref-counted pool for `system_tasks.db` (cron + background share one handle per `dataDir`).
- `CronManager.Close()` / `BackgroundTasks.Close()` on shutdown in server, main, agent, lifeboat.
- `TestMain` pool cleanup in `internal/tools` for Windows file-lock safety.
- Audit manifest: `truenas-registry` replaced with `system-tasks`.
- Appendix EN/DE: LTM documented as `vectordb/`, not `long_term.db`.

---

## P2 — Shutdown, indexes, dead code

### Problem
- Server opened `SkillsDB`, `MissionHistoryDB`, `PreparedMissionsDB`, and galaxa singleton without graceful close.
- Inventory lacked indexes for common `type` and case-insensitive `name` queries.
- `internal/truenas/registry.go` + `InitRegistryDB` were never wired; TrueNAS tools passed unused `db` parameter.

### Resolution
- `server_shutdown.go`: `closeRuntimeResources()` on graceful shutdown closes server-owned SQLite handles, galaxa singleton, SQL connection pool, and stops preparation service.
- Inventory schema v4: `idx_devices_type`, `idx_devices_name_ci`.
- Removed `registry.go`; simplified `DispatchTrueNASTool` / `TrueNASPoolList` signatures.

### Verification
- `internal/server/server_shutdown_test.go`
- `TestInitDBCreatesQueryIndexes` in `inventory_test.go`

---

## Related maintenance work (pre-audit, same milestone)

| Area | Change |
|------|--------|
| Nightly maintenance | Phases 1–4 pipeline, ledger, dashboard, hygiene |
| P0 agent fix | `runMaintenanceTask` no longer calls `tools.SetBusy` |
| P1 retention | `resolveMaintenanceRetention` defaults |
| P2 KG hygiene | `SyncExternalSources` removes stale `dev_*` / `inventory_sync` nodes |

---

## Intentionally not in scope (future backlog)

| Item | Notes |
|------|-------|
| VectorDB in default backup | Use `include_vectordb` flag; documented in configuration |
| TrueNAS local registry | Removed dead code; live API remains canonical |
| Full `sql.Open` audit in tests | Test-only direct opens remain acceptable |
| Unified `dbutil` for backup snapshot reads | `backup_sqlite.go` uses read-only immutable mode by design |

---

## Operator checklist

1. **Backup/restore:** Expect all SQLite files under `data/` listed in `documentation/configuration.md` sqlite section.
2. **LTM migration:** New installs use `data/vectordb/`; legacy `long_term.db` is optional legacy artifact only.
3. **Inventory:** Existing DBs auto-gain indexes on next startup (`InitDB`).
4. **Shutdown:** Server-owned DB handles and task stores (`system_tasks.db`) close cleanly before process exit (WAL checkpoint friendly).

---

## Status

**Audit remediation: COMPLETE (P0–P2 + follow-up)**  
Branch `main` synced with `origin/main` as of 2026-06-11.