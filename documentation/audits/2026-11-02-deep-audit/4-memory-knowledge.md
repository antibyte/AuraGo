# Audit: Memory, Knowledge & Data Layer

**Area:** 4 of the AuraGo multi-area audit · **Date:** 2026-11-02 · **Mode:** read-only deep code audit
**Repo:** `C:\Users\Andi\Documents\repo\AuraGo` (Go module `aurago`, Go 1.27.3)

---

## Scope & method

### What I covered

| Package | Files | LOC |
| --- | ---: | ---: |
| `internal/memory/**` | 312 | 82 765 |
| `internal/embeddings/**` | 25 | 7 205 |
| `internal/chunking/**` | 3 | 590 |
| `internal/kgextraction/**`, `internal/kgquality/**`, `internal/codegraph/**` | 16 | 2 756 |
| `internal/localwiki/**` | 62 | 14 396 |
| `internal/zim/**` | 63 | 11 221 |
| `internal/office/**` | 21 | 8 222 |
| `internal/decisions/**` | 68 | 11 401 |
| `decisions_runtime/**` (C++) | 1 | 1 238 |
| `internal/bookshelf/**`, `internal/bookshelfconvert/**` | 19 | 5 775 |
| `internal/obsidian/**`, `internal/notebooks/**` | 16 | 1 890 |
| `internal/scraper/**`, `internal/newspaper/**` | 18 | 3 389 |
| `internal/mediabins/**`, `internal/contacts/**`, `internal/inventory/**` | 8 | 1 617 |
| **Total** | **632** | **~152 465** |

### Method

`graft` MCP (`graft_check_freshness`, `graft_file_api`) **timed out at 120 s on every attempt** and was unusable, so I fell back to the `graft` CLI (`graft skeleton`, `graft ask`) plus ripgrep-style content search and targeted `read` slices. ~14 graft CLI calls, ~34 content searches, ~21 targeted file reads, 1 behavioural experiment (Python/SQLite). I did **not** need GitNexus `impact`: every finding below is a *read-the-source* finding, not a call-graph claim, so no "unused symbol" assertion is made anywhere.

I deliberately read code in small slices because large `read` results were externalised out of my context by the harness — every line number quoted below was re-verified against the live file with a verbatim slice.

### What I could NOT cover (honest gaps)

- **`internal/memory` is 312 files / 83 k LOC.** I read the schema/migration core, the SQLite open layer, the snapshot coordinator, the session runtime, the KG edge/semantic read paths, the corpus migration, the personality-evidence migration, and the legacy initialiser in depth. I did **not** read the embedding/indexing hot paths (`long_term.go` ~1 900 LOC, `index_batches.go`, `corpus_status.go`), the emotion/affect runtime, the privacy-deletion job engine, or the ~20 `Init*Table()` helpers individually.
- **No dynamic testing.** I ran no builds, no test suites, no fuzzers, and no benchmarks (a repo-mutating constraint). Every "confirmed" claim below is a static read of the cited lines.
- **`internal/decisions` / `decisions_runtime`**: only the download/verification path was inspected (SHA-256 pinning, immutable image digests). The 68-file Go package's rule engine and the 1 238-line C++ runtime protocol were not line-by-line audited.
- **`internal/embeddings`**: the ONNX/llama/docker provider managers were not audited beyond their file list.
- **Out-of-scope dependencies read for context only:** `internal/dbutil/**` (the SQLite open layer — it *is* the memory store's storage engine), `internal/security` (SSRF helper), `internal/infra/store.go` (backup-retention helper), `internal/agent` (retrieval fusion budget).

### Scope spillover

- `internal/server/**` and `internal/agent/**` are assigned to other areas; I cite them only as *callers* (e.g. `internal/server/detective_service.go:221`).
- `documentation/audits/2026-10-30-backup-restore-audit.md` is a prior team audit of the backup/snapshot area. It is **not** in my scope, but I read it to avoid re-reporting already-fixed items and to cross-check F-5/F-13 (it lists them as open items M4a / M4e).
- `internal/virtualcomputers/**`, `internal/whatsapp/**`, `internal/telegram/**`, `internal/discord/**` contain their own SQLite stores with `query +=` string building; **not audited** (out of scope).

---

## Architecture map

```
internal/memory/schema_migrations.go:26    NewSQLiteMemory — single entry point for the short-term store
internal/memory/schema_migrations.go:35      -> dbutil.Open(dbPath, WithCorruptionRecovery, WithImmediateTransactions)
internal/dbutil/open.go:27                    Open — corrupt-DB quarantine + retry loop
internal/dbutil/open.go:101                   buildDSN — foreign_keys(1), synchronous(NORMAL), busy_timeout(5000), _txlock=immediate
internal/dbutil/open.go:161                   applyPragmas — PRAGMA journal_mode=WAL (persistent)
internal/dbutil/open.go:170                   runIntegrityCheck — PRAGMA integrity_check(1) on EVERY open
internal/dbutil/open.go:188                   recoverCorruptDB — quarantine db/-wal/-shm, create empty replacement
internal/memory/schema_migrations.go:45       read PRAGMA user_version  (once, outside any transaction)
internal/memory/schema_migrations.go:55       backupMainDatabase -> sqlite_backup.go:15  (VACUUM INTO <label>.bak)
internal/memory/schema_migrations.go:63       legacy branch -> short_term_init.go:55 initializeLegacySQLiteMemory (NOT transactional)
internal/memory/schema_migrations.go:98       memorySchemaSteps — 13..17
internal/memory/schema_migrations.go:115      applyMemorySchemaStep — BeginTx + PRAGMA user_version + Commit
internal/memory/privacy_scrub_schema.go:33    v13 migratePrivacyScrub        (idempotent: IF NOT EXISTS)
internal/memory/maintenance_progress_schema.go:12  v14 migrateMaintenanceProgress (idempotent)
internal/memory/archive_schema.go:9           v15 migrateMemoryArchives      (idempotent)
internal/memory/schema_migrations.go:103      v16 guarded ALTER via pragma_table_info
internal/memory/memory_expiry.go:16           v17 migrateMemoryExpiry        (idempotent)
internal/memory/graph_sqlite.go:164           KG store — own user_version chain, graph_timestamp.go:105 re-reads marker in-tx
internal/memory/document_corpus.go:109        corpus store (documents.sqlite) — synchronous=FULL, no corruption recovery
internal/memory/document_corpus_migrate.go:66 legacy corpus migration (gob/gzip, TOCTOU-hashed)
internal/memory/personality_evidence.go:83    unversioned destructive personality migration + probe rehearsal
internal/memory/snapshot_coordinator.go:63    per-DB coordinator (key = pragma_database_list main path)
internal/memory/snapshot_coordinator.go:182   FreezeMemorySnapshots (3x BEGIN IMMEDIATE reservation)
internal/memory/snapshot_coordinator.go:311   CaptureMemorySnapshots (3 pinned WAL read txns, online backup API)
internal/memory/session_runtime.go:32         per-session turn gate, package-global map
internal/memory/short_term.go:14              SQLiteMemory.Close -> db.Close()  (and nothing else)
internal/memory/conversation.go:173           read path: session-scoped message read
internal/memory/graph_edge.go:298             read path: KG edge expansion, chunked IN-clause
internal/memory/graph_semantic.go:1378        health report (swallowed COUNT(*) errors)
internal/memory/long_term.go:183              ChromemVectorDB.Close — WaitGroups + cancel, unlike SQLiteMemory.Close
```

**Storage engines.** Four SQLite databases (modernc.org/sqlite v1.60.1, `SQLITE_OPEN_URI`): `short_term.sqlite` (messages, core memory, personality, plans, activity, audit), `knowledge_graph.sqlite` (kg_nodes/kg_edges/kg_claims + FTS), `documents.sqlite` (vector corpus, `synchronous=FULL`), plus the chromem vector files. All four are opened through the single `dbutil.Open` funnel, so WAL / `busy_timeout` / `foreign_keys` / `_txlock=immediate` are uniform.

**Migration chains.** Two independent chains: the STM chain (`PRAGMA user_version` 1->17, steps 13-17 in `memorySchemaSteps`) and the KG chain (`kgSchemaVersion`, `graph_timestamp.go`). A third, *unversioned* path (`initPersonalityEvidence`) is gated only by table existence. A fourth, *legacy* path (`initializeLegacySQLiteMemory`) handles pre-v12 stores outside the version machinery.

**Read/write paths.** Writes go through `dbutil.Open` pools (4 conns; `_txlock=immediate` => every `db.Begin()` is `BEGIN IMMEDIATE`, so SQLite never deadlocks on a deferred-tx lock upgrade). Cross-DB consistency is coordinated by `MemorySnapshotCoordinator` lease + freeze, not by SQLite.

**Retrieval flow.** `internal/agent` -> retrieval fusion (out of scope) -> memory/KG search functions -> chromem vector collection or SQLite `LIMIT`-bounded queries -> budgeted context.

---

## Findings

### F-1 [HIGH] SQLite corruption silently replaces the whole memory store with an empty database

**Location:** `internal/dbutil/open.go:27-55` + `:188-240`; `internal/memory/schema_migrations.go:35,55-59`

**Evidence** — `internal/dbutil/open.go:34-52`:

```go
	db, err := openAndConfigure(dbPath, cfg)
	if err != nil {
		if !cfg.corruptionRecovery || !isCorruption(err) {
			return nil, err
		}
		// PRAGMA or integrity check failure — attempt recovery.
		if cfg.recoveryLogger != nil {
			cfg.recoveryLogger.Error("SQLite open failed, attempting corruption recovery",
				"path", dbPath, "error", err)
		}
		if recErr := recoverCorruptDB(dbPath, cfg.recoveryLogger); recErr != nil {
			return nil, fmt.Errorf("%w (recovery also failed: %v)", err, recErr)
		}
		// Retry after recovery.
		db, err = openAndConfigure(dbPath, cfg)
```

`internal/dbutil/open.go:228-234` (inside `recoverCorruptDB`):

```go
	file, err := os.OpenFile(dbPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create replacement database: %w", err)
	}
```

`internal/memory/schema_migrations.go:35` opts into this (`WithCorruptionRecovery(logger)`), and lines 55-59 then take the "pre-migration backup":

```go
	if existing && version < shortTermSchemaVersion {
		if _, err := backupMainDatabase(db, fmt.Sprintf("schema-v%d", shortTermSchemaVersion)); err != nil {
			return nil, fmt.Errorf("back up memory database before migration: %w", err)
		}
	}
```

**Why it matters.** On a corrupt `short_term.sqlite`, `recoverCorruptDB` moves the DB and its `-wal`/`-shm` into a quarantine directory and creates a **0-byte replacement at the original path**. `NewSQLiteMemory` then proceeds as if the store were new: `version` reads 0, the legacy initialiser builds a fresh schema, `EnsureDefaultSession` runs — and AuraGo starts with **zero memories, zero core facts, zero character notes, zero personality evidence**. The user sees an agent that has forgotten everything, with only one `Error` log line to explain it.

Two aggravating details:

1. `existing` was computed at `schema_migrations.go:34` from the *original* file (true), but `backupMainDatabase` at line 56 now copies the **replacement empty database**. The project's own pre-migration safety net captures nothing useful.
2. The quarantined original is never surfaced to the user beyond a log line; recovery requires the operator to go hunting for `short_term.sqlite.corrupt-*` directories.

The sibling stores behave differently and better: `document_corpus.go:109` and `graph_sqlite.go:164` open **without** `WithCorruptionRecovery`, so a corrupt corpus/KG fails startup loudly instead of silently emptying.

**Fix.** In `internal/memory/schema_migrations.go:35`, drop `dbutil.WithCorruptionRecovery(logger)` for the short-term store and let `dbutil.Open` return the integrity error (matching the corpus and KG behaviour); or, if auto-recovery must stay, gate it on a user-visible confirmation and take the `schema-v%d` backup *before* the quarantine happens (i.e. move the `backupMainDatabase` call above the open, or have `recoverCorruptDB` copy rather than move).

**Confidence:** Confirmed — read from source at the cited lines.

---

### F-2 [MEDIUM] The memory migration chain reads `user_version` outside the write transaction, unlike the KG chain in the same package

**Location:** `internal/memory/schema_migrations.go:45-83` and `:115-133`; contrast `internal/memory/graph_timestamp.go:105-127`

**Evidence** — `internal/memory/schema_migrations.go:45-48` and `:72-83`:

```go
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return nil, fmt.Errorf("read memory schema version: %w", err)
	}
	...
	for _, step := range memorySchemaSteps() {
		if step.version <= version {
			continue
		}
		if step.version != version+1 {
			return nil, fmt.Errorf("missing memory schema step after version %d", version)
		}
		if err := applyMemorySchemaStep(ctx, db, step); err != nil {
			return nil, err
		}
		version = step.version
	}
```

`internal/memory/schema_migrations.go:115-133` (`applyMemorySchemaStep`) opens a transaction but **never re-reads `user_version` inside it** — it goes straight to `step.apply` and then `PRAGMA user_version=%d`.

The knowledge-graph store in the same package gets this right — `internal/memory/graph_timestamp.go:105-127`:

```go
	tx, err := kg.db.Begin()
	...
	// Re-read the marker under the write transaction so concurrent startup cannot
	// normalize with a stale version decision.
	raw = sql.NullString{}
	err = tx.QueryRow(`SELECT value FROM kg_meta WHERE key=?`, kgTimestampMigrationKey).Scan(&raw)
```

**Why it matters.** `version` is a stale snapshot for the whole loop. Two AuraGo processes starting against the same `short_term.sqlite` (or one process opening it twice) both read version 12 and both execute steps 13->17. Today this is **benign**: I verified all five steps are idempotent — `CREATE TABLE IF NOT EXISTS` / `CREATE TRIGGER IF NOT EXISTS` (`privacy_scrub_schema.go:33`, `maintenance_progress_schema.go:12`, `archive_schema.go:9`), the `pragma_table_info`-guarded `ALTER TABLE` (`schema_migrations.go:103-110`), and `migrateMemoryExpiry` (`memory_expiry.go:16`).

That safety is **incidental, not enforced**. The moment someone adds a step that is not naturally idempotent (a `CREATE INDEX` without `IF NOT EXISTS`, a data backfill without a marker, an `ALTER TABLE ADD COLUMN` without a `pragma_table_info` guard — exactly the pattern the v16 step had to work around), the second starter corrupts or fails. The `step.version != version+1` guard cannot catch this because it also reads the stale local variable.

**Fix.** In `applyMemorySchemaStep` (`internal/memory/schema_migrations.go:115`), re-read `PRAGMA user_version` after `db.BeginTx` and no-op if `current >= step.version`; and in the loop at `:72`, re-read the version after each committed step instead of trusting the local `version`. Mirror the wording of the KG comment at `graph_timestamp.go:111-112`.

**Confidence:** Confirmed for the code and for the idempotency of steps 13-17 (all five `apply` functions were read). The *harm* is currently latent — the concurrent double-migration is real, the damage is not.

---

### F-3 [MEDIUM] The pre-v12 legacy migration is ~320 lines of non-transactional DDL/DML; a mid-way failure leaves a half-migrated store that re-runs on every start

**Location:** `internal/memory/short_term_init.go:55-374`; invoked from `internal/memory/schema_migrations.go:63-71`

**Evidence** — `internal/memory/short_term_init.go:281-322` (excerpt of the statement sequence; all of it runs in autocommit, one statement group at a time):

```go
	tables, indexes := splitLegacyMemorySchema(schema)
	if _, err := db.Exec(tables); err != nil {
		return fmt.Errorf("failed to create sqlite schema: %w", err)
	}
	...
	for _, migration := range fileIndexMigrations {
		if err := migrateAddColumn(db, logger, "file_indices", migration.column, migration.def); err != nil {
			return fmt.Errorf("file_indices migration %s: %w", migration.column, err)
		}
	}

	deletedCoreDuplicates, err := migrateCoreMemoryUniqueFacts(db, logger)
	if err != nil {
		return fmt.Errorf("core_memory duplicate cleanup: %w", err)
	}
	...
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_core_memory_fact_unique ON core_memory(fact)`); err != nil {
		return fmt.Errorf("core_memory unique index: %w", err)
	}
	...
	if err := applySQLiteMemoryMigrations(db, logger); err != nil {
		return fmt.Errorf("sqlite memory migrations: %w", err)
	}

	if _, err := db.Exec(indexes); err != nil {
		return fmt.Errorf("create legacy memory indexes: %w", err)
	}
```

...followed by ~14 further `stm.Init*Table()` calls at `short_term_init.go:323-366`.

**Why it matters.** The version bump to 12 happens *afterwards*, in a separate transaction (`schema_migrations.go:67`, `applyMemorySchemaStep` with `apply == nil`). So the sequence is:

1. `db.Exec(tables)` — commits.
2. `migrateCoreMemoryUniqueFacts` — **deletes rows** from `core_memory` (its own transaction, `short_term_init.go:449-481`; it does take a `.bak` backup first at `:440`).
3. `CREATE UNIQUE INDEX ... idx_core_memory_fact_unique` — separate statement, separate commit.
4. `db.Exec(indexes)` — separate commit.
5. ...15 more initialisers.
6. *Then* `user_version = 12`.

If step 4 or 5 fails (e.g. an index references a column an existing legacy table lacks — the code even comments on this at `schema_migrations.go:158-169`: *"Existing tables may still lack columns used by indexes"*), the store is left with `user_version` still `< 12`. **Every subsequent start re-enters `initializeLegacySQLiteMemory` and re-runs the whole sequence**, including the row-deleting `migrateCoreMemoryUniqueFacts`. The `.bak` backup made at step 2 is a full DB copy that is itself never pruned (see F-5), so repeated failed starts also grow the disk.

The delete at step 2 and the unique-index creation at step 3 are not in the same transaction: a crash between them leaves duplicates deleted and the constraint unenforced.

**Fix.** Wrap the whole of `initializeLegacySQLiteMemory` (`internal/memory/short_term_init.go:55-374`) in one `db.Begin()` transaction and commit it *before* the `user_version = 12` step, so a failure leaves `user_version` untouched and the schema untouched. Move `applySQLiteMemoryMigrations` and the `db.Exec(indexes)` call inside that transaction too (they are currently separate commits at `:316` and `:320`).

**Confidence:** Confirmed — the statement sequence and the version-bump ordering were read at the cited lines. (The *row deletion* itself is backed up at `short_term_init.go:440`, which is why I rate this MEDIUM rather than HIGH.)

---

### F-4 [MEDIUM] Unbounded gzip decompression in the legacy corpus migration — startup OOM in a function that otherwise defends hard against tampering

**Location:** `internal/memory/document_corpus_migrate.go:20-46`

**Evidence:**

```go
func readLegacyGob(path string, dest any) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("legacy vector must be regular: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	var reader io.Reader = file
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer gz.Close()
		reader = gz
	}
	if err := gob.NewDecoder(reader).Decode(dest); err != nil {
		return fmt.Errorf("decode legacy vector %s: %w", filepath.Base(path), err)
	}
	return nil
}
```

**Why it matters.** `gzip.NewReader` imposes no decompression ceiling; the only bound is the *compressed* file size on disk. The decoded target is `chromem.Document`, which carries a full `Content string` and an `Embedding []float32`, so the entire decompressed payload is materialised in RAM. A 10 MB gzip of zeros expands to roughly 10 GB (gzip's maximum ratio is ~1032:1) — a startup OOM on the machine that runs AuraGo.

The inconsistency is what makes this worth reporting: the *same function* defends obsessively against every other form of tampering. `migrateLegacy` rejects symlinked legacy vectors (`document_corpus_migrate.go:80-82`), hashes every source before decoding (`:109-114`), re-hashes after decoding to catch TOCTOU mutation (`:137-143`), re-hashes again before commit (`:227-235`), and verifies the decoded row contents inside the transaction (`:209-216`). Everything is protected **except the size of the thing being decoded** — and the hash checks all happen *after* the decode has already consumed the memory.

**Fix.** In `internal/memory/document_corpus_migrate.go:33-41`, wrap the gzip stream in an `io.LimitReader` sized to a sane ceiling (the corpus schema's `content`/`vector` columns bound what a legitimate document can be — e.g. 64 MiB), and check the compressed file's `info.Size()` against a cap before opening. The repo already has the pattern to copy: `internal/zim/cluster.go:27` (`maxClusterBytes = 32 << 20`) with `internal/zim/invalid_test.go:153-158` pinning the limit.

**Confidence:** Confirmed for the unbounded read. The *remote-exploitability* is low — the source is the user's own `VectorDBDir`, and the migrator refuses symlinks — so this is a local-tampering / disk-corruption DoS, not an internet-facing bug.

---

### F-5 [MEDIUM] Memory migration backups are never pruned, while `internal/infra` already ships a retention helper for exactly this

**Location:** `internal/memory/sqlite_backup.go:15-31`; existing helper at `internal/infra/store.go:275-300`

**Evidence** — `internal/memory/sqlite_backup.go:15-31`:

```go
func backupMainDatabase(db *sql.DB, label string) (string, error) {
	path, err := mainDatabasePath(db)
	if err != nil {
		return "", err
	}
	...
	// VACUUM INTO includes WAL content
	backupPath := fmt.Sprintf("%s.%s.bak", path, label)
	...
```

There is no prune step in `internal/memory` — the only `.bak` handling in the repo is `internal/infra/store.go:275-300`:

```go
// migrationBackupRetention bounds the full database copies a failed or repeated
// migration can leave next to the inventory database.
const migrationBackupRetention = 3

func pruneMigrationBackups(databasePath string) {
	matches, err := filepath.Glob(databasePath + ".infra-v*.bak")
	if err != nil || len(matches) <= migrationBackupRetention {
```

**Why it matters.** `backupMainDatabase` produces a **full copy** of the database (`VACUUM INTO`), and the memory store calls it repeatedly and from several places:

- `internal/memory/schema_migrations.go:56` — `schema-v17` on every version bump;
- `internal/memory/personality_evidence.go:111` — `personality-evidence-v1`;
- `internal/memory/personality_evidence.go:115` — `personality-evidence-probe` (a second full copy, taken purely to rehearse a migration);
- `internal/memory/short_term_init.go:440` — `core-memory-normalized-v1` before a row deletion.

None of these are ever deleted. `short_term.sqlite` is the largest store in the system (messages + core memory + personality + plans + audit). Every failed or repeated legacy start therefore leaves another full-size `.bak` next to it, permanently. On a multi-GB store this is a disk-exhaustion path, and it silently competes with the user's real data.

This is a **repo-consistency violation**: the retention pattern already exists in the codebase (`internal/infra/store.go`) and the memory store does not use it. It is also a **known-open item** — `documentation/audits/2026-10-30-backup-restore-audit.md` lists it as **M4a** ("Migrations-`.bak` werden nie gelöscht | `sqlite_backup.go:25-31`"). It is still open.

**Fix.** Add a `pruneMemoryBackups(path)` in `internal/memory/sqlite_backup.go` mirroring `internal/infra/store.go:279-300` (keep the newest N per label), and call it after each successful `backupMainDatabase`. Alternatively, generalise `internal/infra/store.go:279` into a shared helper and call it from both.

**Confidence:** Confirmed — both call sites and the absence of any prune in `internal/memory` were checked by content search over the package.

---

### F-6 [MEDIUM] `PRAGMA integrity_check(1)` runs a full-page scan on every database open

**Location:** `internal/dbutil/open.go:83-93` and `:170-177`

**Evidence** — `internal/dbutil/open.go:83-93`:

```go
	// Run integrity check
	corrupt, err := runIntegrityCheck(db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run integrity check: %w", err)
	}

	if corrupt {
		db.Close()
		return nil, errIntegrityCorrupt
	}
```

`internal/dbutil/open.go:170-177`:

```go
// runIntegrityCheck runs PRAGMA integrity_check and returns whether corruption was detected.
func runIntegrityCheck(db *sql.DB) (bool, error) {
	var result string
	if err := db.QueryRow("PRAGMA integrity_check(1)").Scan(&result); err != nil {
		return false, fmt.Errorf("integrity check query failed: %w", err)
	}
	return result != "ok", nil
}
```

**Why it matters.** `integrity_check(1)` limits the number of *reported errors* to 1, not the amount of the database scanned — SQLite still walks every page of the database. This runs on **every** `dbutil.Open`, i.e. at every process start for the short-term store, the KG store, the corpus, and roughly thirty other stores in the repo. Startup latency therefore scales linearly with the size of `short_term.sqlite`, which for a long-running agent is the largest file in the data directory. The marginal value is low: WAL already protects against torn writes, and a *cheap* `PRAGMA quick_check` (used correctly by the snapshot path at `internal/memory/snapshot_coordinator.go:407`) would catch most corruption at a fraction of the cost.

**Fix.** In `internal/dbutil/open.go:83`, run `PRAGMA quick_check` on the startup path and reserve the full `integrity_check` for an explicit maintenance/backup command. Alternatively gate the full check behind an option (`dbutil.WithFullIntegrityCheck()`) and enable it only for the pre-backup path.

**Confidence:** Confirmed for the code path and SQLite's documented semantics of `integrity_check(1)`. The *magnitude* of the latency depends on the user's data volume and was not measured here (no benchmark was run).

---

### F-7 [MEDIUM] `SQLiteMemory.Close()` releases only the SQL handle — session runs, snapshot lease and the global turn-gate map survive

**Location:** `internal/memory/short_term.go:14-16`; `internal/memory/session_runtime.go:32-56`

**Evidence** — `internal/memory/short_term.go:14-16`:

```go
func (s *SQLiteMemory) Close() error {
	return s.db.Close()
}
```

`internal/memory/session_runtime.go` keeps a **package-global** map of per-session turn gates, keyed by the store pointer (`LockSessionTurn` / `BeginSessionTurn`, `session_runtime.go:32-56`). The `SQLiteMemory` struct also holds the snapshot-coordinator reference (`inMemorySnapshot`, `snapshot_coordinator.go:74-77`) and a `sessionRuns` cancel registry.

By contrast, the vector store's `Close` is thorough — `internal/memory/long_term.go:183` acquires `lifecycleMu`, sets `closed`, cancels `validationCancel`, and drains `indexingWg` / `validationWg` / `storeWg` / `batchWg` / `searchWg`.

**Why it matters.**

- `s.db.Close()` returns immediately and does **not** cancel in-flight work. A turn that is mid-flight keeps running and will get `sql: database is closed` from the next query instead of a clean cancellation.
- `sessionTurnGates` is only pruned when a gate's refcount reaches zero (`drop()` deletes the map entry). An aborted turn that never calls its release function leaves the entry behind **forever**, keeping the closed `*SQLiteMemory` reachable and leaking the map entry.
- The key contains the store **pointer**. A later store allocated by the Go allocator at the same address with the same session id would match the stale key. I could not confirm a live deadlock from this (it requires the stale gate's token to still be held), so I am not claiming one — but the aliasing hazard is real and the map has no `Close`-time sweep.

**Fix.** In `internal/memory/short_term.go:14`, make `Close` cancel every registered session run, release any held snapshot lease, and sweep `sessionTurnGates` entries whose `store` field equals `s`. Add a `once` guard so a second `Close` is a no-op.

**Confidence:** Confirmed for the code (the three-line `Close`, the global map, the contrast with `ChromemVectorDB.Close`). The *pointer-aliasing -> deadlock* consequence is **Needs-verification**: to confirm, call `BeginSessionTurn` on a store, drop the release function without calling it, `Close()` the store, allocate a new store, and check whether `BeginSessionTurn` blocks on the same session id.

---

### F-8 [MEDIUM] Swallowed errors on KG health-report paths produce silently wrong numbers

**Location:** `internal/memory/graph_semantic.go:1378-1379` and `:1456-1457`

**Evidence:**

```go
	_ = kg.db.QueryRow("SELECT COUNT(*) FROM kg_nodes").Scan(&report.TotalNodes)
	_ = kg.db.QueryRow("SELECT COUNT(*) FROM kg_edges").Scan(&report.TotalEdges)
```

(identical pair at `graph_semantic.go:1456-1457`)

**Why it matters.** These are diagnostics/health reports — precisely the surface a user or operator consults when the knowledge graph looks wrong. If the query fails (table missing on a half-migrated KG, DB closed, I/O error), `Scan` leaves the destination at its zero value and the error is discarded. The report then states **0 nodes and 0 edges** — indistinguishable from "the graph really is empty". Every other error in the same function is handled properly (`lifecycle, err := kg.GetLifecycleCounts(); if err != nil { return nil, ... }` at `:1380-1383`), so this is an inconsistency inside one function, not a house style.

The same pattern appears at `internal/memory/character_notes.go:147` (`_ = res` — a discarded `sql.Result` whose `LastInsertId`/`RowsAffected` are then not checked), and in ~15 `json.Unmarshal` call sites across `internal/memory/activity.go` / `plans.go` where the error is dropped and a zero value is used.

**Fix.** In `internal/memory/graph_semantic.go:1378-1379` and `:1456-1457`, capture the error and return it (or set an explicit `report.Error` field) rather than discarding it — matching the `GetLifecycleCounts` handling four lines below. In `internal/memory/character_notes.go:147`, check `res.RowsAffected()` or drop the variable.

**Confidence:** Confirmed — both call sites read verbatim.

---

### F-9 [MEDIUM] `initPersonalityEvidence` is an unversioned destructive migration outside the `user_version` chain, and it takes two full DB copies while pending

**Location:** `internal/memory/personality_evidence.go:83-137` and `:139`; `internal/memory/personality_observations.go:77-84`

**Evidence** — `internal/memory/personality_evidence.go:82-120` (excerpt):

```go
// initPersonalityEvidence quarantines unproven text atomically, once per store.
func (s *SQLiteMemory) initPersonalityEvidence() error {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name='personality_profile_evidence')`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		...
	}
	if _, err := backupMainDatabase(s.db, "personality-evidence-v1"); err != nil {
		return fmt.Errorf("back up personality migration: %w", err)
	}
	// Rehearse the exact transactional migration on a protected consistent copy.
	probePath, err := backupMainDatabase(s.db, "personality-evidence-probe")
```

and the destructive part, `internal/memory/personality_evidence.go:139`:

```go
func migratePersonalityEvidence(db *sql.DB) error {
	tx, err := db.Begin()
```

(the migration moves rows from `user_profile`, `character_notes` and `emotion_history` into a `personality_legacy_review` table).

**Why it matters.**

1. **It is outside the version chain.** Every other schema change in this store is gated by `PRAGMA user_version` (`schema_migrations.go:45-83`). This one is gated only by *"does the table `personality_profile_evidence` exist"* (`personality_evidence.go:85`), which is checked in **two** places (`personality_evidence.go:85` and `personality_observations.go:78`). A store that gets the table created but the migration only half-applied can never be re-entered by the version machinery.
2. **Two full DB copies per pending start.** Lines 111 and 115 each call `backupMainDatabase`, which is a `VACUUM INTO` of the entire `short_term.sqlite`. The second copy exists only to rehearse the migration. On a large store that is 2x the database size written to disk *on every start* until the migration completes — and neither copy is ever pruned (F-5).
3. **The rehearsal is not the live transaction.** The probe is opened with a bare DSN (`internal/memory/personality_evidence.go:120`: `sql.Open("sqlite", probePath)`), so it does **not** inherit the live connection's `foreign_keys(1)` / `synchronous` / `_txlock=immediate` settings that `dbutil.buildDSN` applies (`internal/dbutil/open.go:130-142`). The rehearsed transaction therefore has different semantics from the one that will actually run against the live database.

**Fix.** Move `initPersonalityEvidence` into `memorySchemaSteps()` as a proper numbered step so it is gated by `user_version` like everything else, and drop the second `backupMainDatabase` — the probe can be a copy of the first backup rather than a fresh `VACUUM INTO`. If the probe must stay, open it through `dbutil.Open(probePath, dbutil.WithImmediateTransactions())` so the rehearsal matches the live connection.

**Confidence:** Confirmed for the code, the two copies, the unversioned gate, and the bare-DSN probe. (The "half-applied migration leaves the store stuck" consequence is a **hypothesis** — the body of `migratePersonalityEvidence` is transactional, so I could not confirm an unrecoverable state; I am claiming the *structural* divergence, not a proven bricking.)

---

### F-10 [LOW] `NewSQLiteMemory(":memory:")` rebuilds the entire schema on every call, and it is called per detective run

**Location:** `internal/memory/schema_migrations.go:26-113`; caller `internal/server/detective_service.go:221` (out of scope, cited as evidence)

**Evidence** — `internal/server/detective_service.go:221-225`:

```go
	isolatedMemory, err := memory.NewSQLiteMemory(":memory:", s.Logger)
	if err != nil {
		return err
	}
	defer isolatedMemory.Close()
```

Every `NewSQLiteMemory` call runs, for an in-memory store: `dbutil.Open` (including the `integrity_check(1)` of F-6), the full legacy schema DDL (`short_term_init.go:57-280`, approx. 220 lines of `CREATE TABLE`/`CREATE INDEX`), `splitLegacyMemorySchema`, the row-deleting `migrateCoreMemoryUniqueFacts`, `applySQLiteMemoryMigrations`, ~14 `Init*Table()` calls, `EnsureDefaultSession`, then steps 13->17 with `validateMemorySchema` and `recoverStartup`.

**Why it matters.** Each detective run pays for a full schema build (approx. 20 tables + approx. 40 indexes + triggers) before the work starts, and the store is thrown away at the end of the run. For an in-memory store the legacy path is pure overhead — there is no pre-v12 data to migrate.

**Fix.** In `internal/memory/schema_migrations.go:63`, skip the legacy branch when `dbPath == ":memory:"` and jump straight to `user_version = legacyMemorySchemaVersion` after creating the current schema — or add a `NewInMemorySQLiteMemory()` constructor that applies only `memorySchemaSteps()`.

**Confidence:** Confirmed — the call site and the unconditional legacy branch were both read. The latency cost was not benchmarked.

---

### F-11 [LOW] Dead code in the KG edge-expansion query builder and in character notes

**Location:** `internal/memory/graph_edge.go:290-296` (dead `args`) and `:309-317` (`allArgs` actually used); `internal/memory/character_notes.go:147`

**Evidence** — `internal/memory/graph_edge.go:290-296`:

```go
		placeholders := make([]string, len(nodeIDs))
		args := make([]interface{}, 0, len(nodeIDs)+1)
		for i, id := range nodeIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		args = append(args, limit)
```

...while the query is actually executed at `graph_edge.go:309-317` with a different slice:

```go
		allArgs := make([]interface{}, 0, len(nodeIDs)*2+1)
		for _, id := range nodeIDs {
			allArgs = append(allArgs, id)
		}
		for _, id := range nodeIDs {
			allArgs = append(allArgs, id)
		}
		allArgs = append(allArgs, limit)
		rows, err = kg.db.Query(query, allArgs...)
```

The `args` slice built at 291-296 is never read. The earlier sibling at `graph_edge.go:97-108` *does* use `args` correctly, so the dead copy was clearly left behind during the `allArgs` rewrite.

`internal/memory/character_notes.go:144-150`:

```go
	if err != nil {
		return CharacterNote{}, fmt.Errorf("insert character note: %w", err)
	}
	_ = res
	var protected int
```

**Why it matters.** No runtime effect — but `args` allocates a slice proportional to `nodeIDs` on every edge expansion for nothing, and `_ = res` is a discarded `sql.Result` whose `RowsAffected` is never checked, which is the exact spot where an "inserted 0 rows" would go unnoticed.

**Fix.** Delete `internal/memory/graph_edge.go:291-296` (and the now-unused `args` allocation). In `internal/memory/character_notes.go:147`, either drop `res` or check `res.RowsAffected()`.

**Confidence:** Confirmed — both regions read verbatim; `args` at 291-296 has no reader between its construction at 296 and the `Query` at 317.

---

### F-12 [LOW] `migrateCoreMemoryUniqueFacts` loads the whole `core_memory` table into memory

**Location:** `internal/memory/short_term_init.go:401-437`

**Evidence** — `internal/memory/short_term_init.go:401-431`:

```go
	rows, err := db.Query("SELECT id, fact, normalized_fact FROM core_memory ORDER BY id ASC")
	...
	seen := make(map[string]int64)
	updates := make([]coreFactRow, 0)
	duplicates := make([]coreFactDuplicate, 0)
	for rows.Next() {
		var id int64
		var fact, stored string
		if err := rows.Scan(&id, &fact, &stored); err != nil {
			...
		}
		normalized := normalizeCoreMemoryFactForDedupe(fact)
		if keptID, ok := seen[normalized]; ok {
			duplicates = append(duplicates, coreFactDuplicate{id: id, keptID: keptID})
			continue
		}
		seen[normalized] = id
		if stored != normalized {
			updates = append(updates, coreFactRow{id: id, normalized: normalized})
		}
	}
```

**Why it matters.** The whole table is streamed into three in-memory structures: a `map[string]int64` keyed by the normalized fact plus two slices. The memory cost is O(rows x len(fact)). `core_memory` is the small "durable facts" table in normal operation, so today this is bounded — but there is no `LIMIT`, and the query is unbounded by construction. A pathological `core_memory` (e.g. after a botched import) turns a startup migration into an OOM.

**Fix.** Page the scan with `LIMIT ? OFFSET ?` (or a keyset cursor on `id`) and keep only the `seen` map, which is inherently O(distinct facts). If `core_memory` is expected to stay small, add a hard row-count guard that aborts the migration with a clear error above a threshold.

**Confidence:** Confirmed — the read is verbatim; the practical exposure depends on how large `core_memory` can grow, which I did not measure.

---

### F-13 [LOW] Two different coordinator key spaces exist for the same short-term database file

**Location:** `internal/memory/snapshot_coordinator.go:63-80` (store side) vs `:186` (freeze side)

**Evidence** — `internal/memory/snapshot_coordinator.go:63-80`:

```go
func (s *SQLiteMemory) snapshotCoordinator(contexts ...context.Context) (*MemorySnapshotCoordinator, error) {
	...
	var path string
	if err := s.db.QueryRowContext(ctx, `SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&path); err != nil {
		...
	}
	...
	return snapshotCoordinator(snapshotPathKey(path)), nil
}
```

versus `internal/memory/snapshot_coordinator.go:186`:

```go
	c := snapshotCoordinator(snapshotPathKey(cfg.SQLite.ShortTermPath))
```

`internal/memory/snapshot_coordinator.go:42-47` (`snapshotPathKey`) normalises with `filepath.Abs` + `filepath.Clean` — it does **not** resolve symlinks.

**Why it matters.** The store registers itself under the path SQLite reports for `main`; the freeze path registers under the *configured* path. If the two strings differ after normalisation (symlinked data directory, a relative config path resolved against a different working directory, case differences on a case-insensitive volume, or a config path that differs from the path actually opened), the freeze acquires a **different** `MemorySnapshotCoordinator` than the one the writers use. `c.freeze(ctx)` then fences nothing, and `CaptureMemorySnapshots` (`snapshot_coordinator.go:311-414`) can copy a snapshot while a lease-holding writer is mid-publication — the torn snapshot the coordinator exists to prevent.

**Fix.** Derive both keys from the same source: have `FreezeMemorySnapshots` resolve `cfg.SQLite.ShortTermPath` through `pragma_database_list` of an open handle (or normalise both with `filepath.EvalSymlinks`) before calling `snapshotPathKey`.

**Confidence:** **Needs-verification.** The two key derivations are confirmed by source; I did not confirm that they actually differ in any real AuraGo configuration. To verify: start AuraGo with a symlinked data directory and assert that `store.snapshotCoordinator()` returns the same pointer as `snapshotCoordinator(snapshotPathKey(cfg.SQLite.ShortTermPath))`. The team's own audit lists this as **M4e** (`documentation/audits/2026-10-30-backup-restore-audit.md`).

---

## Areas inspected and found in good shape (no finding raised)

Stating these explicitly so the next reviewer does not re-audit them.

| Area | Evidence | Why it is sound |
| --- | --- | --- |
| **ZIM untrusted-archive handling** | `internal/zim/cluster.go:27` (`maxClusterBytes = 32 << 20`), `:34` (`zstdMaxMemory`), `internal/zim/blob.go:9` (`maxMetadataBytes = 1 << 20`), `internal/zim/archive.go:15` (`maxDirentSize = 64 << 10`), `internal/zim/cache.go:13` (`DefaultClusterCacheBytes = 64 << 20`) | Every decompression path is capped (zstd window, XZ dictionary, cluster size, metadata size, dirent size); the limits are **pinned by tests** (`internal/zim/invalid_test.go:153-158`, `internal/zim/xz_test.go:165-171`), redirects are depth-limited, and offsets are range-checked. No path-traversal surface (ZIM has no filesystem paths). |
| **Office DOCX/XLSX parsing** | `internal/office/docx_preservation.go:24-38` | `zip.NewReader` over an in-memory buffer, `len(reader.File) > 4096` part cap, 128 MiB expanded-size budget, and a `path.Clean(file.Name) != file.Name || strings.HasPrefix(file.Name, "/") || strings.HasPrefix(file.Name, "../")` zip-slip guard. XML is parsed with `encoding/xml` (no external entity resolution => no XXE) and HTML with `golang.org/x/net/html`. No "billion laughs" surface (no custom entity expansion). |
| **Bookshelf / archive extraction** | `internal/bookshelf/archive.go`, `internal/bookshelf/metadata.go:577` (`validateArchivePath`) | `filepath.Base(filename) != filename` guards before every `filepath.Join`; archives are read through a bounded reader with a part/size budget. |
| **localwiki untrusted HTML** | `internal/localwiki/render_clean.go`, `internal/localwiki/render.go` | Every text node is HTML-escaped before assembly; `script`, `style`, `link`, `meta`, `iframe`, `object` and event-handler attributes are stripped. The comment at `render.go` ("*tidyMarkdown* must never un-escape brackets") shows the escaping invariant is consciously maintained. |
| **localwiki / scraper network egress** | `internal/localwiki/httpclient.go`, `internal/scraper/scraper.go` | HTTPS-only redirect policy, a dial-time guard, a kiwix host allowlist, and `security.ValidateSSRF` + `security.GetPublic` on the scraper's fetch path. |
| **decisions runtime download** | `internal/decisions/download.go` | SHA-256 pinning of downloaded artifacts; container images are referenced by immutable digest, not tag. |
| **SQL injection** | `internal/memory/conversation.go:173`, `internal/memory/audit.go` (`auditWhereClause`), `internal/memory/graph_edge.go:298-317` | Every query I read is parameterised. The three `fmt.Sprintf` SQL sites in `internal/memory` interpolate only **internal constants** (a table name, a timestamp expression) or a `strings.Join` of `?` placeholders — never user input. `auditWhereClause` builds a fixed predicate list with `?` placeholders. **No SQL injection found.** |
| **Retrieval bounds** | `internal/memory/search_bounds_test.go`, `internal/memory/graph_edge.go:284` (`nodeIDs = nodeIDs[:defaultInClauseChunkSize]`), `internal/memory/graph_semantic.go:1378` | Every search path carries a `LIMIT`; the KG IN-clause is chunked so the SQLite variable limit cannot be exceeded; the agent layer applies a character budget to the fused context. **No unbounded scan found in the paths I read.** |
| **Transaction discipline (versioned path)** | `internal/memory/schema_migrations.go:115-133` | `BeginTx` -> `defer tx.Rollback()` -> `Commit`, with the version bump inside the same transaction. Multi-statement writes in the versioned chain are atomic. **No leaked open transaction found.** (The *legacy* path is the exception — F-3.) |
| **SQLite concurrency primitives** | `internal/dbutil/open.go:130-142`, `internal/dbutil/options.go:30-32` | WAL is enabled and persistent (`applyPragmas`, `open.go:161-168`); `busy_timeout(5000)` is embedded **per connection** in the DSN; `_txlock=immediate` makes every `db.Begin()` a `BEGIN IMMEDIATE`, which eliminates the classic deferred-transaction lock-upgrade deadlock; `foreign_keys(1)` is always on; `synchronous=FULL` for the document corpus. |
| **Vector store lifecycle** | `internal/memory/long_term.go:183` | `ChromemVectorDB.Close` cancels validation, drains five WaitGroups and sets a `closed` flag — the model that `SQLiteMemory.Close` (F-7) should follow. |
| **C++ decisions runtime** | `decisions_runtime/` (1 238 LOC) | No `strcpy` / `strcat` / `sprintf` / `alloca` / unchecked `malloc` — a content search over the whole directory returned no matches. |
| **Data isolation** | `internal/memory` | AuraGo is a single-user, local-first agent: there is **no tenant/workspace concept** (a search for `tenant|workspace_id|user_id` over `internal/memory` returns nothing), so there is no cross-tenant leak to find. The real boundary is `session_id`, and it is enforced on the message read/write paths. Cross-session retrieval (`GetRecentMessagesAcrossSessions`) is a deliberate context-building feature, not a leak. **No isolation finding.** |

---

## Metrics

| Metric | Value |
| --- | --- |
| Files in scope | 632 |
| LOC in scope | ~152 465 (of which `internal/memory` = 82 765) |
| Files read in full or near-full | 21 |
| Files read in verified slices | 26 |
| Packages inspected at directory level | 21 |
| graft CLI calls (`skeleton`, `ask`) | ~14 |
| Content searches (ripgrep over the workspace) | ~34 |
| Verbatim evidence slices re-read for line-number verification | 14 |
| Behavioural experiments | 1 (SQLite URI percent-decoding, Python `sqlite3` 3.50.4) |
| Builds / test runs / fuzzers | 0 (repo-mutating constraint) |
| GitNexus `impact` runs | 0 (not needed — no call-graph claims made) |
| **Findings — CRITICAL** | **0** |
| **Findings — HIGH** | **1** (F-1) |
| **Findings — MEDIUM** | **8** (F-2 ... F-9) |
| **Findings — LOW** | **4** (F-10 ... F-13) |
| Total findings | 13 |
| Confirmed / Probable / Needs-verification | 12 / 0 / 1 (F-13) |
| Areas checked and found sound | 13 (table above) |
| Prior-audit overlap (still open) | F-5 = M4a, F-13 = M4e in `documentation/audits/2026-10-30-backup-restore-audit.md` |

---

## Not covered / follow-ups

### Not covered in this pass

1. **`internal/memory/long_term.go` (~1 900 LOC)** — the chromem indexing, dedup, batch-store and parallel-search paths. `ChromemVectorDB` has five WaitGroups and an `embedSlot` semaphore; I read only its `Close`. The `dedupSem` and `storeEmbedSlot` fairness/timeout behaviour is unreviewed.
2. **`internal/memory` embedding/indexing hot path** — `index_batches.go`, `corpus_status.go`, `candidates.go`. Unbounded queue growth on a stalled embedding provider is the obvious thing to check next.
3. **The privacy-deletion job engine** (`privacy_deletion_jobs.go`, `privacy_graph_apply.go`, `personality_privacy_counts`) — this is the "delete my data" path and deserves the same scrutiny as the backup path, especially whether every table covered by `personality_privacy_counts` is actually purged.
4. **`internal/decisions`** (68 files / 11 401 LOC) — only the download/verification path was read. The rule engine, the C++ runtime protocol (`decisions_runtime/`, 1 238 LOC) and the Go<->C++ boundary were not audited.
5. **`internal/embeddings`** — the ONNX worker, llama docker restart/pull logic and the Vulkan loader were not read.
6. **`internal/newspaper`** — `service.go` has a 123-line `execute` function; feed parsing was not reviewed for XXE/HTML injection.
7. **No dynamic verification.** No benchmark for the `integrity_check` cost (F-6), no repro of the corruption path (F-1), no memory measurement for F-4/F-12.

### Recommended next passes

1. **F-1 first.** It is the only finding with user-visible data loss, and the fix is a one-line option change plus an ordering change in `NewSQLiteMemory`.
2. **Audit the privacy-deletion engine** as a separate area — "the user asked for deletion" is the same class of promise as "the user asked for a backup", and it currently has neither the visibility nor the test coverage that the backup path has.
3. **F-3 + F-2 together.** Bringing the legacy path into a transaction and re-reading `user_version` under the write lock would let the whole migration surface be governed by one rule, and would align the STM chain with the KG chain that already gets it right.
4. **Add the three missing regression tests** that the findings imply: (a) a corruption-recovery test asserting that the pre-migration backup contains the *pre-quarantine* bytes; (b) a concurrent-migration test that starts two stores on the same file and asserts each versioned step runs exactly once; (c) a decompression-cap test on `readLegacyGob` asserting a bomb-sized `.gob.gz` is rejected rather than allocated.
5. **Re-run this pass over `internal/memory` with `graft` MCP available** — the MCP timed out on every call here, which cost the deeper call-graph work (e.g. confirming that `ChromemVectorDB.Close` really is the only owner of the corpus handle, and enumerating every `NewSQLiteMemory` caller).

---

*Report generated read-only. The only file written by this audit is this report.*