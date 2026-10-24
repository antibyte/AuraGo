# Infra Redesign Plan A (Backend) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the additive Infra backend for the canvas-first redesign: schema v3, persistent source ignore, remove-and-ignore, Docker engine host mappings with a runtime-only guest host projection, layout parent and place size, stricter edge creation, and batch import. Everything stays backward compatible.

**Architecture:** `internal/infra` gets an explicit v2→v3 migration (`infra_sources.ignored_at`, `infra_source_hosts`). It also gets store APIs for ignore, delete-and-ignore, source hosts and link-with-layout, plus an extended atomic layout batch. Every write keeps its transaction and version CAS. `internal/server` adds three handlers (`PUT /sources/ignore`, `PUT /source-hosts`, `POST /import/batch`) and extends `DELETE /nodes/{id}`, `GET /snapshot` and `GET /candidates` with query flags. It also projects runtime-only `host_id` / `host_inferred` from three inputs, in this order: Proxmox `runs_on` provider edges, explicit Docker engine mappings, and an address inference that never uses DNS. New JSON fields are `omitempty` or new keys, so existing clients, payloads and tests keep working.

**Tech Stack:** Go 1.27, `database/sql` with `modernc.org/sqlite` v1.60.1 (`foreign_keys=ON` through `internal/dbutil`), `net/http` and `httptest`, `net/netip`.

---

## Execution notes (read once, apply in every task)

- Work only in `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\infra-redesign` (branch `feature/infra-redesign`) and run commands in Git Bash from that directory.
- Line numbers refer to the baseline commit `cacc7a58b`. Earlier tasks move lines, so find each edit by the quoted code.
- Run `gofmt -w` on every Go file you touched before the "verify it passes" step.
- If `go` fails with disk-space errors, trim the D: Go cache (GOCACHE/GOTMPDIR entries older than 3 days, then older than 1 day) and rerun.
- Impact procedure for every task that edits an existing function. Run the GitNexus and graft commands shown in the task. The GitNexus index is stale and usually answers `risk: UNKNOWN`, which is not an all-clear. Confirm callers with the given `git grep` and record them in your task report. If GitNexus reports HIGH or CRITICAL, write that warning into the task report before editing, and do not waive it. Each task lists the expected callers.
- Stage only the paths listed in the task. Every commit message ends with the line `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## File Structure

| File | Status | Responsibility |
|---|---|---|
| `internal/infra/store.go` | modify | schema version 3, migration chain v1→v2→v3, shared `infra_source_hosts` DDL, prune keeps ignored rows |
| `internal/infra/types.go` | modify | `Placement.W/H`, `LayoutItem.ParentID/W/H`, `Source.Ignored`, `Candidate.Ignored`, `Node.HostID/HostInferred`, `SourceHost`, `Snapshot.SourceHosts`, `ViewPlacement`, `ImportLayout` |
| `internal/infra/validate.go` | modify | place-size validation (`validatePlacement`), `placementsForKind`, runtime reset of host fields |
| `internal/infra/nodes.go` | modify | `UpdateNode` drops sizes on kind change, `DeleteNodeAndIgnoreSources`, import link clears ignore, source column list |
| `internal/infra/sources.go` | modify | `sourceColumns`, ignore flag scan, `ListSourcesIncludingIgnored`, `LinkSourceWithLayout`, link clears ignore |
| `internal/infra/layout.go` | modify | layout items change parent and place size |
| `internal/infra/edges.go` | modify | self-loop and duplicate refusal |
| `internal/infra/errors.go` | modify | `ErrEdgeExists` |
| `internal/infra/snapshot.go` | modify | `SnapshotOptions`, `SnapshotWith`, ignored flag, `SourceHosts` |
| `internal/infra/ignore.go` | create | `SetSourcesIgnored` |
| `internal/infra/source_hosts.go` | create | `SetSourceHost` (CAS), stored and placeholder host entries |
| `internal/infra/store_test.go` | modify | store tests |
| `internal/infra/AGENTS.md` | modify | contract bullets |
| `internal/server/desktop_infra.go` | modify | route switch, snapshot/candidates flags, DELETE `ignore_sources`, `desktopInfraStoreErrorStatus`, host projection on write responses |
| `internal/server/desktop_infra_sources.go` | modify | Proxmox reconcile lists ignored sources too (behaviour unchanged) |
| `internal/server/desktop_infra_projection.go` | modify | snapshot decoration runs the host projection |
| `internal/server/desktop_infra_candidates.go` | modify | an explicit engine mapping keeps the host visible |
| `internal/server/desktop_infra_ignore.go` | create | `PUT /sources/ignore` handler |
| `internal/server/desktop_infra_hosts.go` | create | `PUT /source-hosts` handler, guest host projection, local interface addresses |
| `internal/server/desktop_infra_batch.go` | create | `POST /import/batch` handler |
| `internal/server/desktop_infra_ignore_test.go` | create | ignore, candidates/snapshot exclusion, remove-and-ignore route tests |
| `internal/server/desktop_infra_hosts_test.go` | create | source-host route and host projection tests |
| `internal/server/desktop_infra_batch_test.go` | create | batch import tests |
| `internal/server/desktop_infra_routes_test.go` | modify | `edge_exists` mapping case, route registration with read-only refusal |
| `documentation/infra.md` | modify | API overview section |

### Design decisions locked in by this plan

- **Ignored sources are never linked.** Ignoring a linked source returns `ErrSourceLinked`, which the ignore route maps to 409 `source_linked`; the global mapping stays `source_unavailable`, so existing import tests still pass. Importing or linking a source clears its ignore flag. Provider refreshes never touch `ignored_at`. Retention pruning skips ignored rows, so the decision survives long outages.
- **What sees ignored sources by default:** `ListSources` and `Snapshot` exclude them. `ListSourcesIncludingIgnored` and `SnapshotWith(..., IncludeIgnored)` include them. The Proxmox reconcile uses the including variant, so its behaviour is unchanged.
- **Layout `parent_id`:** absent keeps the parent, `""` clears it. **Layout `w`/`h`:** absent keeps the stored size. Size is allowed only for places, 120–20000, and either both values are 0 or both are within bounds. `UpdateNode` drops the size when a place becomes another kind, so a room→device kind change is not refused.
- **Edges:** a self-loop is `ErrValidation` (400), and an existing `(from,to,kind)`, including a provider edge, is `ErrEdgeExists` (409 `edge_exists`). Both checks apply to `CreateEdge` only. Updates stay as they are so legacy duplicates remain editable.
- **Host projection order:** Proxmox `runs_on` provider edge (guest → host), then the Docker mapping in `infra_source_hosts`, then an inferred host for the configured Docker engine only.
  - A local socket or pipe, an empty host, `localhost` or a loopback IP means the server's own non-loopback, non-link-local interface addresses.
  - A `tcp/http/https` IP literal means that address. Host names (for example `tcp://docker-proxy:2375`) are never resolved, so they infer nothing.
  - Inference needs exactly one matching visible device; an ambiguous match infers nothing.
  - The host fields are projected on the snapshot and on node-returning writes (node PUT, import, batch import, layout). The `PUT /source-hosts` response does not carry `inferred_node_id`; the snapshot does.

---

### Task 1: Schema v3 migration

**Files:**
- Modify: `internal/infra/store.go:14` (`schemaVersion`), `:44-147` (`migrate`), add `infraSourceHostsSchema` and `migrateSourceIgnoreV3`
- Modify: `internal/infra/store_test.go:51` and `:637` (backup globs `infra-v2` → `infra-v3`), append one test

- [ ] **Step 1: Impact analysis**

```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "migrate" --direction upstream --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
graft callers migrate
git grep -n "migrate(" -- internal/infra
```

Expected: GitNexus `UNKNOWN`, or matches for unrelated `migrate` symbols in other packages (keep only `internal/infra/store.go`). The text search shows a single caller, `New` (`internal/infra/store.go:31`). Risk LOW. If HIGH or CRITICAL, record the warning.

- [ ] **Step 2: Write the failing test and update the two backup globs**

In `internal/infra/store_test.go`, in `TestMigrationBacksUpInventoryBeforeAddingInfraSchema`, replace:

```go
	backups, err := filepath.Glob(path + ".infra-v2-*.bak")
	if err != nil || len(backups) != 1 {
```

with:

```go
	backups, err := filepath.Glob(path + ".infra-v3-*.bak")
	if err != nil || len(backups) != 1 {
```

In `TestActionLedgerV1UpgradePreservesResultsAndMakesPendingUnknown`, replace:

```go
	backups, err := filepath.Glob(path + ".infra-v2-*.bak")
	if err != nil || len(backups) != 2 {
```

with:

```go
	backups, err := filepath.Glob(path + ".infra-v3-*.bak")
	if err != nil || len(backups) != 2 {
```

Append to `internal/infra/store_test.go`:

```go
func TestSchemaV3MigrationKeepsV2RowsAndAddsIgnoreAndSourceHosts(t *testing.T) {
	store, db, path := openInfraTestStore(t)
	ctx := context.Background()
	node, err := store.CreateNode(ctx, testNode(NodeDevice, "Docker host", "server"))
	if err != nil {
		t.Fatal(err)
	}
	identity := SourceIdentity{Kind: "docker", Instance: "docker_engine", Key: "container-1"}
	if err := store.ObserveSources(ctx, identity.Kind, identity.Instance, []SourceObservation{{
		Identity: identity, Name: "web", Observed: SourceObserved{Kind: NodeContainer, Name: "web", Icon: "container"},
	}}, true); err != nil {
		t.Fatal(err)
	}
	// Recreate the exact v2 shape with rows: no ignore column, no source host table.
	if _, err := db.Exec(`
		DROP TABLE infra_source_hosts;
		ALTER TABLE infra_sources DROP COLUMN ignored_at;
		UPDATE infra_meta SET version=2 WHERE id=1;
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := New(db); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := db.QueryRow(`SELECT version FROM infra_meta WHERE id=1`).Scan(&version); err != nil || version != 3 {
		t.Fatalf("schema version = %d, %v; want 3", version, err)
	}
	var ignoredAt string
	if err := db.QueryRow(`SELECT ignored_at FROM infra_sources WHERE kind=? AND instance=? AND source_key=?`,
		identity.Kind, identity.Instance, identity.Key).Scan(&ignoredAt); err != nil || ignoredAt != "" {
		t.Fatalf("migrated source ignored_at = %q, %v; want empty", ignoredAt, err)
	}
	if _, err := db.Exec(`INSERT INTO infra_source_hosts(kind, instance, node_id, version, updated_at) VALUES('docker','docker_engine',?,1,'now')`, node.ID); err != nil {
		t.Fatalf("source host table after migration: %v", err)
	}
	if loaded, err := store.GetNode(ctx, node.ID); err != nil || loaded.Name != "Docker host" {
		t.Fatalf("migrated node = %#v, %v", loaded, err)
	}
	backups, err := filepath.Glob(path + ".infra-v3-*.bak")
	if err != nil || len(backups) != 2 {
		t.Fatalf("backups = %v, %v; want the fresh-init and the v2 upgrade copies", backups, err)
	}
	if _, err := New(db); err != nil {
		t.Fatalf("reopening a v3 schema: %v", err)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/infra/ -run '^(TestSchemaV3MigrationKeepsV2RowsAndAddsIgnoreAndSourceHosts|TestMigrationBacksUpInventoryBeforeAddingInfraSchema|TestActionLedgerV1UpgradePreservesResultsAndMakesPendingUnknown)$' -count=1 -v`
Expected: FAIL. The new test fails with `no such table: infra_source_hosts`, and both glob tests fail with `migration backups = [] ...` / `backups after fresh init ... = []`.

- [ ] **Step 4: Implement the migration**

In `internal/infra/store.go`, replace `const schemaVersion = 2` with:

```go
const schemaVersion = 3

// infraSourceHostsSchema is shared by the full schema and the v2 -> v3 step.
const infraSourceHostsSchema = `
	CREATE TABLE IF NOT EXISTS infra_source_hosts (
		kind TEXT NOT NULL,
		instance TEXT NOT NULL,
		node_id TEXT REFERENCES infra_nodes(id) ON DELETE CASCADE,
		version INTEGER NOT NULL CHECK (version > 0),
		updated_at TEXT NOT NULL,
		PRIMARY KEY(kind, instance)
	);
	CREATE INDEX IF NOT EXISTS idx_infra_source_hosts_node ON infra_source_hosts(node_id);
`
```

Replace the whole `func migrate(db *sql.DB) error { ... }` with:

```go
func migrate(db *sql.DB) error {
	var hasMeta bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='infra_meta')`).Scan(&hasMeta); err != nil {
		return fmt.Errorf("inspect infra schema version: %w", err)
	}
	version := 0
	if hasMeta {
		if err := db.QueryRow(`SELECT version FROM infra_meta WHERE id=1`).Scan(&version); err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("read infra schema version: %w", err)
		}
	}
	if version > schemaVersion {
		return fmt.Errorf("infra schema %d is newer than supported version %d", version, schemaVersion)
	}
	if version == schemaVersion {
		return nil
	}
	if err := backupExistingDatabase(db, schemaVersion); err != nil {
		return err
	}
	switch version {
	case 1:
		if err := migrateActionLedgerV2(db); err != nil {
			return err
		}
		return migrateSourceIgnoreV3(db)
	case 2:
		return migrateSourceIgnoreV3(db)
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin infra migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS infra_meta (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			version INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS infra_nodes (
			id TEXT PRIMARY KEY,
			version INTEGER NOT NULL CHECK (version > 0),
			kind TEXT NOT NULL,
			name TEXT NOT NULL,
			icon TEXT NOT NULL,
			registry_id TEXT NOT NULL DEFAULT '',
			parent_id TEXT,
			profile_json TEXT NOT NULL,
			placements_json TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(parent_id) REFERENCES infra_nodes(id) ON DELETE RESTRICT
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_infra_registry_id
			ON infra_nodes(registry_id) WHERE registry_id <> '';
		CREATE INDEX IF NOT EXISTS idx_infra_nodes_parent ON infra_nodes(parent_id);
		CREATE TABLE IF NOT EXISTS infra_sources (
			kind TEXT NOT NULL,
			instance TEXT NOT NULL,
			source_key TEXT NOT NULL,
			node_id TEXT,
			name TEXT NOT NULL DEFAULT '',
			state TEXT NOT NULL,
			fetched_at TEXT NOT NULL DEFAULT '',
			removed_at TEXT NOT NULL DEFAULT '',
			observed_json TEXT NOT NULL,
			ignored_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY(kind, instance, source_key),
			FOREIGN KEY(node_id) REFERENCES infra_nodes(id) ON DELETE SET NULL
		);
		CREATE INDEX IF NOT EXISTS idx_infra_sources_node ON infra_sources(node_id);
		CREATE INDEX IF NOT EXISTS idx_infra_sources_freshness ON infra_sources(kind, instance, state, fetched_at);
		CREATE TABLE IF NOT EXISTS infra_edges (
			id TEXT PRIMARY KEY,
			version INTEGER NOT NULL CHECK (version > 0),
			from_id TEXT NOT NULL,
			to_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			label TEXT NOT NULL DEFAULT '',
			from_port TEXT NOT NULL DEFAULT '',
			to_port TEXT NOT NULL DEFAULT '',
			speed TEXT NOT NULL DEFAULT '',
			source_kind TEXT NOT NULL DEFAULT '',
			source_instance TEXT NOT NULL DEFAULT '',
			source_key TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL,
			FOREIGN KEY(from_id) REFERENCES infra_nodes(id) ON DELETE CASCADE,
			FOREIGN KEY(to_id) REFERENCES infra_nodes(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_infra_edges_from ON infra_edges(from_id);
		CREATE INDEX IF NOT EXISTS idx_infra_edges_to ON infra_edges(to_id);
		CREATE INDEX IF NOT EXISTS idx_infra_edges_source ON infra_edges(source_kind, source_instance, source_key);
		CREATE TABLE IF NOT EXISTS infra_action_requests (
			request_id TEXT PRIMARY KEY,
			fingerprint TEXT NOT NULL,
			action TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status IN ('running','accepted','completed','failed','unknown')),
			upid TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_infra_actions_updated ON infra_action_requests(updated_at);
		INSERT INTO infra_meta(id, version) VALUES(1, 3)
			ON CONFLICT(id) DO UPDATE SET version=excluded.version;
	`); err != nil {
		return fmt.Errorf("create infra schema: %w", err)
	}
	if _, err := tx.Exec(infraSourceHostsSchema); err != nil {
		return fmt.Errorf("create infra source host schema: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit infra migration: %w", err)
	}
	return nil
}
```

Directly after `migrateActionLedgerV2` (after its closing brace), insert:

```go
// migrateSourceIgnoreV3 adds the persisted ignore decision and the engine host
// mapping. Both parts are idempotent: a database whose action ledger was rolled
// back to v1 may already carry the v3 column and table.
func migrateSourceIgnoreV3(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin infra source ignore migration: %w", err)
	}
	defer tx.Rollback()
	var hasIgnored bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM pragma_table_info('infra_sources') WHERE name='ignored_at')`).Scan(&hasIgnored); err != nil {
		return fmt.Errorf("inspect infra source columns: %w", err)
	}
	if !hasIgnored {
		if _, err := tx.Exec(`ALTER TABLE infra_sources ADD COLUMN ignored_at TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add infra source ignore column: %w", err)
		}
	}
	if _, err := tx.Exec(infraSourceHostsSchema); err != nil {
		return fmt.Errorf("create infra source host schema: %w", err)
	}
	if _, err := tx.Exec(`UPDATE infra_meta SET version=3 WHERE id=1`); err != nil {
		return fmt.Errorf("record infra schema version 3: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit infra source ignore migration: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `gofmt -w internal/infra/store.go internal/infra/store_test.go`
Run: `go test ./internal/infra/ -run '^(TestSchemaV3MigrationKeepsV2RowsAndAddsIgnoreAndSourceHosts|TestMigrationBacksUpInventoryBeforeAddingInfraSchema|TestActionLedgerV1UpgradePreservesResultsAndMakesPendingUnknown)$' -count=1 -v`
Expected: PASS (3 tests).
Run: `go test ./internal/infra/ -count=1`
Expected: `ok  	aurago/internal/infra`

- [ ] **Step 6: Commit**

```bash
git add internal/infra/store.go internal/infra/store_test.go
git commit -m "feat(infra): migrate store schema to v3 with ignore column and source hosts" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Placement size validation (place kinds only, 120–20000)

**Files:**
- Modify: `internal/infra/types.go:91-95` (`Placement`)
- Modify: `internal/infra/validate.go:42` (call in `validateNode`), `:187-200` (`validatePlacements`) plus new helpers
- Modify: `internal/infra/nodes.go:162-207` (`UpdateNode`)
- Test: `internal/infra/store_test.go` (append)

- [ ] **Step 1: Impact analysis**

```bash
for symbol in validatePlacements validateNode UpdateNode Placement; do
  node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$symbol" --direction upstream --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
  graft callers "$symbol"
done
git grep -nE "(validatePlacements|validateNode|UpdateNode)\(" -- internal/infra 'internal/server/desktop_infra*'
```

Expected text-search callers:
- `validatePlacements` ← `validateNode` (`validate.go:42`).
- `validateNode` ← `CreateNode`, `CreateManualNode`, `CreateImportedNode` and `UpdateNode` (`nodes.go:33,59,102,169`).
- `UpdateNode` ← `desktopInfraNodeRoute` PUT (`internal/server/desktop_infra.go:495`) and store tests.
- `Placement` is used in `layout.go:43` and in tests (keyed literals only).

Risk MEDIUM: shared validation path. Record any HIGH or CRITICAL warning.

- [ ] **Step 2: Write the failing test**

Append to `internal/infra/store_test.go`:

```go
func TestPlacementSizeIsBoundedAndOnlyForPlaces(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	room := testNode(NodeRoom, "Living room", "room")
	room.Placements = Placements{ViewAtlas: {X: 10, Y: 20, W: 360, H: 240}}
	created, err := store.CreateNode(ctx, room)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.GetNode(ctx, created.ID)
	if err != nil || loaded.Placements[ViewAtlas].W != 360 || loaded.Placements[ViewAtlas].H != 240 {
		t.Fatalf("stored place size = %#v, %v", loaded.Placements, err)
	}
	for _, tc := range []struct {
		name      string
		kind      NodeKind
		placement Placement
	}{
		{"device with size", NodeDevice, Placement{W: 360, H: 240}},
		{"too small", NodeRoom, Placement{W: 119, H: 240}},
		{"too large", NodeRack, Placement{W: 360, H: 20001}},
		{"width only", NodeGroup, Placement{W: 360}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			node := testNode(tc.kind, "Sized", "group")
			node.Placements = Placements{ViewAtlas: tc.placement}
			if _, err := store.CreateNode(ctx, node); !errors.Is(err, ErrValidation) {
				t.Fatalf("CreateNode error = %v; want validation error", err)
			}
		})
	}
	// A sized place that becomes a device keeps its position and loses its size.
	loaded.Kind = NodeDevice
	loaded.Icon = "generic-device"
	changed, err := store.UpdateNode(ctx, loaded)
	if err != nil {
		t.Fatalf("kind change of a sized place: %v", err)
	}
	if got := changed.Placements[ViewAtlas]; got.W != 0 || got.H != 0 || got.X != 10 || got.Y != 20 {
		t.Fatalf("placement after kind change = %#v; want position kept and size dropped", got)
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/infra/ -run '^TestPlacementSizeIsBoundedAndOnlyForPlaces$' -count=1 -v`
Expected: FAIL (build error: `unknown field W in struct literal of type Placement`).

- [ ] **Step 4: Implement**

In `internal/infra/types.go`, replace:

```go
type Placement struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Pinned bool    `json:"pinned"`
}
```

with:

```go
// Placement is one view position. W and H size a place region (sites, rooms,
// racks and groups only); zero means the client default size.
type Placement struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	W      float64 `json:"w,omitempty"`
	H      float64 `json:"h,omitempty"`
	Pinned bool    `json:"pinned"`
}
```

In `internal/infra/validate.go`, in `validateNode`, replace:

```go
	if err := validatePlacements(node.Placements); err != nil {
```

with:

```go
	if err := validatePlacements(node.Kind, node.Placements); err != nil {
```

Replace the whole `func validatePlacements(placements Placements) error { ... }` with:

```go
const (
	minPlaceSize = 120
	maxPlaceSize = 20000
)

func validatePlacements(kind NodeKind, placements Placements) error {
	if placements == nil {
		return fmt.Errorf("%w: placements must be an object", ErrValidation)
	}
	for view, placement := range placements {
		if view != ViewAtlas && view != ViewTopology {
			return fmt.Errorf("%w: unsupported placement view", ErrValidation)
		}
		if err := validatePlacement(kind, placement); err != nil {
			return err
		}
	}
	return nil
}

// validatePlacement checks one view position. A size belongs only to places and
// is either absent (both zero) or fully bounded.
func validatePlacement(kind NodeKind, placement Placement) error {
	if !validCoordinate(placement.X) || !validCoordinate(placement.Y) {
		return fmt.Errorf("%w: placement coordinates must be finite and within bounds", ErrValidation)
	}
	if placement.W == 0 && placement.H == 0 {
		return nil
	}
	if !containerKind(kind) {
		return fmt.Errorf("%w: only places can have a placement size", ErrValidation)
	}
	if !validPlaceSize(placement.W) || !validPlaceSize(placement.H) {
		return fmt.Errorf("%w: place size must be between %d and %d", ErrValidation, minPlaceSize, maxPlaceSize)
	}
	return nil
}

func validPlaceSize(value float64) bool {
	return value >= minPlaceSize && value <= maxPlaceSize
}

// placementsForKind drops place sizes from kinds that cannot carry them.
func placementsForKind(kind NodeKind, placements Placements) Placements {
	if containerKind(kind) || placements == nil {
		return placements
	}
	result := make(Placements, len(placements))
	for view, placement := range placements {
		placement.W, placement.H = 0, 0
		result[view] = placement
	}
	return result
}
```

In `internal/infra/nodes.go`, in `UpdateNode`, replace:

```go
	if node.Version <= 0 {
		return Node{}, fmt.Errorf("%w: node id and current version are required", ErrValidation)
	}
	if err := validateNode(&node); err != nil {
		return Node{}, err
	}
```

with:

```go
	if node.Version <= 0 {
		return Node{}, fmt.Errorf("%w: node id and current version are required", ErrValidation)
	}
	// Stored placements replace the caller's below. Dropping a size the new kind
	// cannot carry keeps a place-to-device kind change valid.
	node.Placements = placementsForKind(node.Kind, node.Placements)
	if err := validateNode(&node); err != nil {
		return Node{}, err
	}
```

and replace:

```go
	node.Placements = previous.Placements
	if err := updateNodeTx(ctx, tx, node, previous.Version); err != nil {
```

with:

```go
	node.Placements = placementsForKind(node.Kind, previous.Placements)
	if err := updateNodeTx(ctx, tx, node, previous.Version); err != nil {
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `gofmt -w internal/infra/types.go internal/infra/validate.go internal/infra/nodes.go internal/infra/store_test.go`
Run: `go test ./internal/infra/ -run '^TestPlacementSizeIsBoundedAndOnlyForPlaces$' -count=1 -v`
Expected: PASS.
Run: `go test ./internal/infra/ -count=1`
Expected: `ok  	aurago/internal/infra`

- [ ] **Step 6: Commit**

```bash
git add internal/infra/types.go internal/infra/validate.go internal/infra/nodes.go internal/infra/store_test.go
git commit -m "feat(infra): bound place region sizes in placements" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Layout batch changes `parent_id` and place size

**Files:**
- Modify: `internal/infra/types.go:188-194` (`LayoutItem`)
- Modify: `internal/infra/layout.go` (whole file)
- Test: `internal/infra/store_test.go` (append)

- [ ] **Step 1: Impact analysis**

```bash
for symbol in SaveLayout LayoutItem; do
  node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$symbol" --direction upstream --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
  graft callers "$symbol"
done
git grep -nE "(SaveLayout\(|LayoutItem\{)" -- internal/infra 'internal/server/desktop_infra*'
```

Expected: `SaveLayout` ← `desktopInfraSaveLayout` (`internal/server/desktop_infra.go:643`) and `TestLayoutCASIsAtomicAcrossNodes` (`store_test.go:310,322`). `LayoutItem{` only in `store_test.go:306`. Risk LOW to MEDIUM.

- [ ] **Step 2: Write the failing test**

Append to `internal/infra/store_test.go`:

```go
func TestLayoutBatchChangesParentAndPlaceSize(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	room, err := store.CreateNode(ctx, testNode(NodeRoom, "Office", "room"))
	if err != nil {
		t.Fatal(err)
	}
	rackNode := testNode(NodeRack, "Rack", "rack")
	rackNode.ParentID = room.ID
	rack, err := store.CreateNode(ctx, rackNode)
	if err != nil {
		t.Fatal(err)
	}
	device, err := store.CreateNode(ctx, testNode(NodeDevice, "Switch", "switch"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateNode(ctx, testNode(NodeDevice, "NAS", "nas"))
	if err != nil {
		t.Fatal(err)
	}
	text := func(value string) *string { return &value }
	size := func(value float64) *float64 { return &value }

	updated, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: room.ID, Version: room.Version, X: 0, Y: 0, W: size(600), H: size(400)},
		{ID: device.ID, Version: device.Version, X: 48, Y: 72, ParentID: text(room.ID)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if updated[0].Placements[ViewAtlas].W != 600 || updated[0].Placements[ViewAtlas].H != 400 || updated[1].ParentID != room.ID {
		t.Fatalf("layout result = %#v", updated)
	}
	// Absent w/h keep the stored size; absent parent_id keeps the parent.
	moved, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: room.ID, Version: updated[0].Version, X: 24, Y: 24},
		{ID: device.ID, Version: updated[1].Version, X: 96, Y: 96},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if moved[0].Placements[ViewAtlas].H != 400 || moved[1].ParentID != room.ID {
		t.Fatalf("absent layout fields changed state: %#v", moved)
	}
	for _, tc := range []struct {
		name string
		item LayoutItem
	}{
		{"cycle", LayoutItem{ID: room.ID, Version: moved[0].Version, ParentID: text(rack.ID)}},
		{"non-container parent", LayoutItem{ID: other.ID, Version: other.Version, ParentID: text(device.ID)}},
		{"missing parent", LayoutItem{ID: other.ID, Version: other.Version, ParentID: text("missing")}},
		{"size on a device", LayoutItem{ID: other.ID, Version: other.Version, W: size(300), H: size(200)}},
		{"place too small", LayoutItem{ID: rack.ID, Version: rack.Version, W: size(100), H: size(200)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The valid first item proves the batch stays atomic.
			_, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
				{ID: device.ID, Version: moved[1].Version, X: 500, Y: 500}, tc.item,
			}})
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("SaveLayout error = %v; want validation error", err)
			}
			unchanged, err := store.GetNode(ctx, device.ID)
			if err != nil || unchanged.Version != moved[1].Version {
				t.Fatalf("rejected batch committed its valid item: %#v, %v", unchanged, err)
			}
		})
	}
	cleared, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: device.ID, Version: moved[1].Version, X: 900, Y: 900, ParentID: text("")},
	}})
	if err != nil || cleared[0].ParentID != "" {
		t.Fatalf("clearing the parent = %#v, %v", cleared, err)
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/infra/ -run '^TestLayoutBatchChangesParentAndPlaceSize$' -count=1 -v`
Expected: FAIL (build error: `unknown field W in struct literal of type LayoutItem`).

- [ ] **Step 4: Implement**

In `internal/infra/types.go`, replace:

```go
type LayoutItem struct {
	ID      string  `json:"id"`
	Version int64   `json:"version"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Pinned  bool    `json:"pinned"`
}
```

with:

```go
// LayoutItem moves one node. ParentID nil keeps the parent and "" clears it;
// W/H nil keep the stored place size.
type LayoutItem struct {
	ID       string   `json:"id"`
	Version  int64    `json:"version"`
	X        float64  `json:"x"`
	Y        float64  `json:"y"`
	Pinned   bool     `json:"pinned"`
	ParentID *string  `json:"parent_id,omitempty"`
	W        *float64 `json:"w,omitempty"`
	H        *float64 `json:"h,omitempty"`
}
```

Replace the whole content of `internal/infra/layout.go` with:

```go
package infra

import (
	"context"
	"fmt"
	"strings"
)

// SaveLayout atomically stores one view's positions. Items may also move a node
// into or out of a place (ParentID: nil keeps the parent, "" clears it) and size
// a place (W, H: nil keeps the stored size).
func (s *Store) SaveLayout(ctx context.Context, request LayoutRequest) ([]Node, error) {
	if request.View != ViewAtlas && request.View != ViewTopology {
		return nil, fmt.Errorf("%w: unsupported layout view", ErrValidation)
	}
	if len(request.Nodes) > 1000 {
		return nil, fmt.Errorf("%w: layout update exceeds 1000 nodes", ErrValidation)
	}
	seen := make(map[string]struct{}, len(request.Nodes))
	for _, item := range request.Nodes {
		if err := validateText("layout node id", item.ID, 128, true); err != nil {
			return nil, err
		}
		if item.Version <= 0 || !validCoordinate(item.X) || !validCoordinate(item.Y) {
			return nil, fmt.Errorf("%w: layout item needs a version and bounded coordinates", ErrValidation)
		}
		if item.ParentID != nil {
			if err := validateText("layout parent id", *item.ParentID, 128, false); err != nil {
				return nil, err
			}
		}
		if _, exists := seen[item.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate node in layout update", ErrValidation)
		}
		seen[item.ID] = struct{}{}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin infra layout save: %w", err)
	}
	defer tx.Rollback()
	for _, item := range request.Nodes {
		node, err := loadNode(ctx, tx, item.ID)
		if err != nil {
			return nil, err
		}
		if node.Version != item.Version {
			return nil, ErrConflict
		}
		placement := Placement{X: item.X, Y: item.Y, Pinned: item.Pinned}
		if current, ok := node.Placements[request.View]; ok {
			placement.W, placement.H = current.W, current.H
		}
		if item.W != nil {
			placement.W = *item.W
		}
		if item.H != nil {
			placement.H = *item.H
		}
		if err := validatePlacement(node.Kind, placement); err != nil {
			return nil, err
		}
		if item.ParentID != nil {
			parentID := strings.TrimSpace(*item.ParentID)
			// Earlier items are already written in this transaction, so a cycle
			// built across several items of one batch is detected as well.
			if err := validateParent(ctx, tx, node.ID, parentID); err != nil {
				return nil, err
			}
			node.ParentID = parentID
		}
		previous := node.Version
		node.Version++
		node.Placements[request.View] = placement
		if err := updateNodeTx(ctx, tx, node, previous); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit infra layout save: %w", err)
	}
	updated := make([]Node, 0, len(request.Nodes))
	for _, item := range request.Nodes {
		node, err := s.GetNode(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		updated = append(updated, node)
	}
	return updated, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `gofmt -w internal/infra/types.go internal/infra/layout.go internal/infra/store_test.go`
Run: `go test ./internal/infra/ -run '^(TestLayoutBatchChangesParentAndPlaceSize|TestLayoutCASIsAtomicAcrossNodes)$' -count=1 -v`
Expected: PASS (2 tests).
Run: `go test ./internal/server/ -run '^TestDesktopInfraLayoutRouteReturnsOnlyChangedNodes$' -count=1`
Expected: `ok  	aurago/internal/server`

- [ ] **Step 6: Commit**

```bash
git add internal/infra/types.go internal/infra/layout.go internal/infra/store_test.go
git commit -m "feat(infra): let layout batches change parents and place sizes" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Refuse self-loop and duplicate edges

**Files:**
- Modify: `internal/infra/errors.go` (whole file)
- Modify: `internal/infra/edges.go:16-39` (`CreateEdge`)
- Modify: `internal/server/desktop_infra.go:701-718` (`desktopInfraStoreError`)
- Modify: `internal/server/desktop_infra_routes_test.go:221` (add mapping case)
- Test: `internal/infra/store_test.go` (append)

- [ ] **Step 1: Impact analysis**

```bash
for symbol in CreateEdge desktopInfraStoreError; do
  node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$symbol" --direction upstream --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
  graft callers "$symbol"
done
git grep -nE "(CreateEdge|desktopInfraStoreError)\(" -- internal/infra 'internal/server/desktop_infra*'
```

Expected:
- `CreateEdge` ← `desktopInfraCreateEdge` (`desktop_infra.go:594`) and `store_test.go:155,159`. The existing network-cycle test creates A→B and B→A, which are distinct triples and must keep passing.
- `desktopInfraStoreError` has 19 call sites (16 in `desktop_infra.go`, 2 in `desktop_infra_mutations.go`, 1 test). GitNexus may report HIGH because of the fan-in. Record the warning; the refactor keeps every existing mapping, and `TestDesktopInfraStoreErrorMapsEverySentinel` guards it.

- [ ] **Step 2: Write the failing tests**

Append to `internal/infra/store_test.go`:

```go
func TestCreateEdgeRejectsSelfLoopsAndDuplicates(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	router, err := store.CreateNode(ctx, testNode(NodeDevice, "Router", "router"))
	if err != nil {
		t.Fatal(err)
	}
	access, err := store.CreateNode(ctx, testNode(NodeDevice, "Switch", "switch"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateEdge(ctx, Edge{From: router.ID, To: router.ID, Kind: EdgeEthernet}); !errors.Is(err, ErrValidation) {
		t.Fatalf("self-loop error = %v; want validation error", err)
	}
	if _, err := store.CreateEdge(ctx, Edge{From: router.ID, To: access.ID, Kind: EdgeEthernet}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateEdge(ctx, Edge{From: router.ID, To: access.ID, Kind: EdgeEthernet, Label: "again"}); !errors.Is(err, ErrEdgeExists) {
		t.Fatalf("duplicate edge error = %v; want ErrEdgeExists", err)
	}
	if _, err := store.CreateEdge(ctx, Edge{From: router.ID, To: access.ID, Kind: EdgeWiFi}); err != nil {
		t.Fatalf("same pair with another kind: %v", err)
	}
}
```

In `internal/server/desktop_infra_routes_test.go`, in `TestDesktopInfraStoreErrorMapsEverySentinel`, replace:

```go
		{infra.ErrHasChildren, http.StatusConflict, "has_children"},
```

with:

```go
		{infra.ErrHasChildren, http.StatusConflict, "has_children"},
		{infra.ErrEdgeExists, http.StatusConflict, "edge_exists"},
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/infra/ -run '^TestCreateEdgeRejectsSelfLoopsAndDuplicates$' -count=1 -v`
Expected: FAIL (build error: `undefined: ErrEdgeExists`).
Run: `go test ./internal/server/ -run '^TestDesktopInfraStoreErrorMapsEverySentinel$' -count=1 -v`
Expected: FAIL (build error: `undefined: infra.ErrEdgeExists`).

- [ ] **Step 4: Implement**

Replace the whole content of `internal/infra/errors.go` with:

```go
package infra

import "errors"

var (
	ErrNotFound       = errors.New("infra item not found")
	ErrConflict       = errors.New("infra version conflict")
	ErrValidation     = errors.New("invalid infra data")
	ErrReadOnly       = errors.New("provider edge is read-only")
	ErrSourceNotFound = errors.New("infra source not found")
	ErrSourceLinked   = errors.New("infra source is already linked")
	ErrSourceRemoved  = errors.New("infra source was removed")
	ErrHasChildren    = errors.New("infra node has children")
	ErrEdgeExists     = errors.New("infra edge already exists")
)
```

In `internal/infra/edges.go`, replace the whole `func (s *Store) CreateEdge(...) { ... }` with:

```go
// CreateEdge adds a manual edge. A node cannot connect to itself, and one
// (from, to, kind) triple exists at most once (including provider edges).
func (s *Store) CreateEdge(ctx context.Context, edge Edge) (Edge, error) {
	if edge.ID != "" || edge.Version != 0 || edge.Source != nil {
		return Edge{}, fmt.Errorf("%w: new manual edges cannot choose identity, version, or source", ErrValidation)
	}
	if err := validateEdge(&edge); err != nil {
		return Edge{}, err
	}
	if edge.From == edge.To {
		return Edge{}, fmt.Errorf("%w: a connection needs two different nodes", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Edge{}, fmt.Errorf("begin infra edge creation: %w", err)
	}
	defer tx.Rollback()
	if err := validateEdgeEndpoints(ctx, tx, edge); err != nil {
		return Edge{}, err
	}
	var duplicate bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM infra_edges WHERE from_id=? AND to_id=? AND kind=?)`,
		edge.From, edge.To, edge.Kind).Scan(&duplicate); err != nil {
		return Edge{}, fmt.Errorf("check duplicate infra edge: %w", err)
	}
	if duplicate {
		return Edge{}, ErrEdgeExists
	}
	edge.ID, edge.Version = uid.New(), 1
	if err := insertEdge(ctx, tx, edge); err != nil {
		return Edge{}, err
	}
	if err := tx.Commit(); err != nil {
		return Edge{}, fmt.Errorf("commit infra edge creation: %w", err)
	}
	return edge, nil
}
```

In `internal/server/desktop_infra.go`, replace the whole `func desktopInfraStoreError(w http.ResponseWriter, err error) { ... }` with:

```go
func desktopInfraStoreError(w http.ResponseWriter, err error) {
	status, code, message := desktopInfraStoreErrorStatus(err)
	desktopInfraError(w, status, code, message)
}

// desktopInfraStoreErrorStatus maps a store error to a status, a stable code and
// a sanitized message. Batch imports report the same codes per item.
func desktopInfraStoreErrorStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, infra.ErrConflict):
		return http.StatusConflict, "conflict", "The item changed; refresh and try again"
	case errors.Is(err, infra.ErrNotFound), errors.Is(err, infra.ErrSourceNotFound):
		return http.StatusNotFound, "not_found", "The Infra item was not found"
	case errors.Is(err, infra.ErrSourceRemoved), errors.Is(err, infra.ErrSourceLinked):
		return http.StatusConflict, "source_unavailable", "The source is no longer available"
	case errors.Is(err, infra.ErrHasChildren):
		return http.StatusConflict, "has_children", "Move or remove child nodes first"
	case errors.Is(err, infra.ErrEdgeExists):
		return http.StatusConflict, "edge_exists", "This connection already exists"
	case errors.Is(err, infra.ErrReadOnly):
		return http.StatusForbidden, "source_readonly", "This source is read-only"
	case errors.Is(err, infra.ErrValidation):
		return http.StatusBadRequest, "invalid_request", "Infra data is invalid"
	default:
		return http.StatusInternalServerError, "infra_error", "Infra could not complete the request"
	}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `gofmt -w internal/infra/errors.go internal/infra/edges.go internal/infra/store_test.go internal/server/desktop_infra.go internal/server/desktop_infra_routes_test.go`
Run: `go test ./internal/infra/ -run '^(TestCreateEdgeRejectsSelfLoopsAndDuplicates|TestNodeValidationHierarchyAndNetworkCycles)$' -count=1 -v`
Expected: PASS (2 tests).
Run: `go test ./internal/server/ -run '^TestDesktopInfraStoreErrorMapsEverySentinel$' -count=1 -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/infra/errors.go internal/infra/edges.go internal/infra/store_test.go internal/server/desktop_infra.go internal/server/desktop_infra_routes_test.go
git commit -m "feat(infra): refuse self-loop and duplicate connections" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Persisted ignore in the store

**Files:**
- Modify: `internal/infra/types.go:119-126` (`Source`), `:155-161` (`Candidate`)
- Modify: `internal/infra/sources.go`: `GetCandidate:62`, `ListSources:90-120`, `LinkSource:310`, `loadSource:330-338`, `scanSource:348-373`
- Modify: `internal/infra/nodes.go:144-145` (`CreateImportedNode` link update), `:293-294` (`loadNodeSources` query)
- Modify: `internal/infra/store.go:249-260` (`PruneSources`)
- Create: `internal/infra/ignore.go`
- Test: `internal/infra/store_test.go` (append)

- [ ] **Step 1: Impact analysis**

```bash
for symbol in scanSource loadSource loadNodeSources ListSources GetCandidate PruneSources CreateImportedNode LinkSource Source Candidate; do
  node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$symbol" --direction upstream --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
  graft callers "$symbol"
done
git grep -nE "(scanSource|loadSource|loadNodeSources|ListSources|GetCandidate|PruneSources|CreateImportedNode|LinkSource)\(" -- internal/infra 'internal/server/desktop_infra*'
```

Expected text-search callers:
- `scanSource` ← `loadNodeSources`, `ListSources`, `loadSource`.
- `loadSource` ← `GetSource`, `GetCandidate`, `CreateImportedNode`, `LinkSource`, `ReplaceProviderEdges`.
- `ListSources` ← `desktopInfraConfiguredSources` (`desktop_infra.go:824`), `desktopInfraReconcileProxmoxEdges` (`desktop_infra_sources.go:409`), `store_test.go:733`.
- `GetCandidate` ← `desktopInfraCandidates`, `desktopInfraImport` and many tests.
- `PruneSources` ← `ObserveSources` and a test.
- `CreateImportedNode` / `LinkSource` ← `desktopInfraImport` and tests.

`Source` and `Candidate` are hub types and may be HIGH or CRITICAL. Record the warning; the change only adds `omitempty` fields. Other packages named `ListSources` (mailer, bookshelf) are unrelated.

- [ ] **Step 2: Write the failing test**

Append to `internal/infra/store_test.go`:

```go
func TestIgnoredSourcesLeaveDiscoveriesAndSurviveRefresh(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	observed := func(key string) SourceObservation {
		return SourceObservation{Identity: SourceIdentity{Kind: "docker", Instance: "engine", Key: key}, Name: key,
			Observed: SourceObserved{Kind: NodeContainer, Name: key, Icon: "container"}}
	}
	tv, web := observed("tv"), observed("web")
	if err := store.ObserveSources(ctx, "docker", "engine", []SourceObservation{tv, web}, true); err != nil {
		t.Fatal(err)
	}
	webCandidate, err := store.GetCandidate(ctx, CandidateID(web.Identity))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateImportedNode(ctx, webCandidate.Node, web.Identity); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSourcesIgnored(ctx, []SourceIdentity{web.Identity}, true); !errors.Is(err, ErrSourceLinked) {
		t.Fatalf("ignoring a linked source = %v; want ErrSourceLinked", err)
	}
	if _, err := store.SetSourcesIgnored(ctx, []SourceIdentity{{Kind: "docker", Instance: "engine", Key: "missing"}}, true); !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("ignoring an unknown source = %v; want ErrSourceNotFound", err)
	}
	if _, err := store.SetSourcesIgnored(ctx, nil, true); !errors.Is(err, ErrValidation) {
		t.Fatalf("empty ignore request = %v; want validation error", err)
	}
	for range 2 {
		ignored, err := store.SetSourcesIgnored(ctx, []SourceIdentity{tv.Identity}, true)
		if err != nil || len(ignored) != 1 || !ignored[0].Ignored {
			t.Fatalf("ignore result = %#v, %v; want one ignored source on every call", ignored, err)
		}
	}
	// A provider refresh keeps the decision.
	if err := store.ObserveSources(ctx, "docker", "engine", []SourceObservation{tv, web}, true); err != nil {
		t.Fatal(err)
	}
	listed, err := store.ListSources(ctx, "docker", "engine")
	if err != nil || len(listed) != 1 || listed[0].Key != "web" {
		t.Fatalf("default source list = %#v, %v; want the ignored source left out", listed, err)
	}
	all, err := store.ListSourcesIncludingIgnored(ctx, "docker", "engine")
	if err != nil || len(all) != 2 {
		t.Fatalf("source list with ignored = %#v, %v; want both sources", all, err)
	}
	candidate, err := store.GetCandidate(ctx, CandidateID(tv.Identity))
	if err != nil || !candidate.Ignored || !candidate.Source.Ignored {
		t.Fatalf("ignored candidate = %#v, %v; want the ignored flag after a refresh", candidate, err)
	}
	// Ignored observations outlive the removed-source retention window.
	if err := store.ObserveSources(ctx, "docker", "engine", []SourceObservation{web}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PruneSources(ctx, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if source, err := store.GetSource(ctx, tv.Identity); err != nil || !source.Ignored || source.State != SourceRemoved {
		t.Fatalf("ignored removed source after retention = %#v, %v; want it kept", source, err)
	}
	restored, err := store.SetSourcesIgnored(ctx, []SourceIdentity{tv.Identity}, false)
	if err != nil || len(restored) != 1 || restored[0].Ignored {
		t.Fatalf("restore = %#v, %v", restored, err)
	}
	// Importing an ignored source clears the flag: linked sources are never ignored.
	if err := store.ObserveSources(ctx, "docker", "engine", []SourceObservation{tv, web}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSourcesIgnored(ctx, []SourceIdentity{tv.Identity}, true); err != nil {
		t.Fatal(err)
	}
	tvCandidate, err := store.GetCandidate(ctx, CandidateID(tv.Identity))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateImportedNode(ctx, tvCandidate.Node, tv.Identity); err != nil {
		t.Fatalf("importing an ignored source: %v", err)
	}
	if source, err := store.GetSource(ctx, tv.Identity); err != nil || source.Ignored {
		t.Fatalf("imported source = %#v, %v; want the ignore flag cleared by the link", source, err)
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/infra/ -run '^TestIgnoredSourcesLeaveDiscoveriesAndSurviveRefresh$' -count=1 -v`
Expected: FAIL (build error: `store.SetSourcesIgnored undefined`).

- [ ] **Step 4: Implement**

In `internal/infra/types.go`, replace the `Source` struct with:

```go
type Source struct {
	SourceIdentity
	Name      string         `json:"name,omitempty"`
	State     SourceState    `json:"state"`
	FetchedAt *time.Time     `json:"fetched_at,omitempty"`
	RemovedAt *time.Time     `json:"removed_at,omitempty"`
	Observed  SourceObserved `json:"observed"`
	// Ignored hides the source from discoveries. An ignored source is never linked.
	Ignored bool `json:"ignored,omitempty"`
}
```

and replace the `Candidate` struct with:

```go
type Candidate struct {
	ID          string   `json:"id"`
	Source      Source   `json:"source"`
	Node        Node     `json:"node"`
	RegistryID  string   `json:"registry_id,omitempty"`
	Suggestions []string `json:"suggestions"`
	Ignored     bool     `json:"ignored,omitempty"`
}
```

In `internal/infra/sources.go`, directly above `func CandidateID(`, insert:

```go
// sourceColumns is the column list scanSource reads, in order.
const sourceColumns = `kind, instance, source_key, name, state, fetched_at, removed_at, observed_json, ignored_at`
```

In `GetCandidate`, replace:

```go
	candidate := Candidate{ID: id, Source: source, Node: node, Suggestions: []string{}}
```

with:

```go
	candidate := Candidate{ID: id, Source: source, Node: node, Suggestions: []string{}, Ignored: source.Ignored}
```

Replace the whole `func (s *Store) ListSources(...) { ... }` with:

```go
// ListSources lists observations, optionally of one kind and instance, without
// ignored sources. ListSourcesIncludingIgnored also returns those.
func (s *Store) ListSources(ctx context.Context, kind, instance string) ([]Source, error) {
	return s.listSources(ctx, kind, instance, false)
}

// ListSourcesIncludingIgnored lists observations including ignored sources.
func (s *Store) ListSourcesIncludingIgnored(ctx context.Context, kind, instance string) ([]Source, error) {
	return s.listSources(ctx, kind, instance, true)
}

func (s *Store) listSources(ctx context.Context, kind, instance string, includeIgnored bool) ([]Source, error) {
	query := `SELECT ` + sourceColumns + ` FROM infra_sources WHERE 1=1`
	args := make([]any, 0, 2)
	if kind != "" {
		query += ` AND kind=?`
		args = append(args, kind)
	}
	if instance != "" {
		query += ` AND instance=?`
		args = append(args, instance)
	}
	if !includeIgnored {
		query += ` AND ignored_at=''`
	}
	query += ` ORDER BY kind, instance, source_key`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list infra sources: %w", err)
	}
	defer rows.Close()
	sources := make([]Source, 0)
	for rows.Next() {
		source, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read infra sources: %w", err)
	}
	return sources, nil
}
```

In `LinkSource`, replace:

```go
	res, err = tx.ExecContext(ctx, `UPDATE infra_sources SET node_id=? WHERE kind=? AND instance=? AND source_key=? AND node_id IS NULL`, nodeID, identity.Kind, identity.Instance, identity.Key)
```

with:

```go
	res, err = tx.ExecContext(ctx, `UPDATE infra_sources SET node_id=?, ignored_at='' WHERE kind=? AND instance=? AND source_key=? AND node_id IS NULL`, nodeID, identity.Kind, identity.Instance, identity.Key)
```

Replace the whole `func loadSource(...) { ... }` with:

```go
func loadSource(ctx context.Context, q queryer, identity SourceIdentity) (Source, error) {
	row := q.QueryRowContext(ctx, `SELECT `+sourceColumns+`
		FROM infra_sources WHERE kind=? AND instance=? AND source_key=?`, identity.Kind, identity.Instance, identity.Key)
	source, err := scanSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Source{}, ErrSourceNotFound
	}
	return source, err
}
```

Replace the whole `func scanSource(row sourceScanner) (Source, error) { ... }` with:

```go
func scanSource(row sourceScanner) (Source, error) {
	var source Source
	var fetchedAt, removedAt, observedJSON, ignoredAt string
	if err := row.Scan(&source.Kind, &source.Instance, &source.Key, &source.Name, &source.State, &fetchedAt, &removedAt, &observedJSON, &ignoredAt); err != nil {
		return Source{}, fmt.Errorf("read infra source: %w", err)
	}
	source.Ignored = ignoredAt != ""
	if fetchedAt != "" {
		t, err := time.Parse(timeLayout, fetchedAt)
		if err != nil {
			return Source{}, fmt.Errorf("parse infra source timestamp: %w", err)
		}
		source.FetchedAt = &t
	}
	if removedAt != "" {
		t, err := time.Parse(timeLayout, removedAt)
		if err != nil {
			return Source{}, fmt.Errorf("parse infra source removal timestamp: %w", err)
		}
		source.RemovedAt = &t
	}
	if err := json.Unmarshal([]byte(observedJSON), &source.Observed); err != nil {
		return Source{}, fmt.Errorf("decode infra source observation: %w", err)
	}
	normalizeObserved(&source.Observed)
	return source, nil
}
```

In `internal/infra/nodes.go`, in `CreateImportedNode`, replace:

```go
	res, err := tx.ExecContext(ctx, `UPDATE infra_sources SET node_id=? WHERE kind=? AND instance=? AND source_key=? AND node_id IS NULL`,
		node.ID, identity.Kind, identity.Instance, identity.Key)
```

with:

```go
	res, err := tx.ExecContext(ctx, `UPDATE infra_sources SET node_id=?, ignored_at='' WHERE kind=? AND instance=? AND source_key=? AND node_id IS NULL`,
		node.ID, identity.Kind, identity.Instance, identity.Key)
```

In `loadNodeSources`, replace:

```go
	rows, err := q.QueryContext(ctx, `SELECT kind, instance, source_key, name, state, fetched_at, removed_at, observed_json
		FROM infra_sources WHERE node_id=? ORDER BY kind, instance, source_key`, node.ID)
```

with:

```go
	rows, err := q.QueryContext(ctx, `SELECT `+sourceColumns+`
		FROM infra_sources WHERE node_id=? ORDER BY kind, instance, source_key`, node.ID)
```

In `internal/infra/store.go`, replace the whole `func (s *Store) PruneSources(...) { ... }` with:

```go
// PruneSources deletes unlinked removed observations older than olderThan.
// Ignored observations are kept: the operator's decision must outlive an outage.
func (s *Store) PruneSources(ctx context.Context, olderThan time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM infra_sources
		WHERE node_id IS NULL AND state='removed' AND removed_at<>'' AND removed_at<? AND ignored_at=''`, formatTime(olderThan))
	if err != nil {
		return 0, fmt.Errorf("prune expired infra source observations: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count expired infra source observations: %w", err)
	}
	return n, nil
}
```

Create `internal/infra/ignore.go`:

```go
package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// maxIgnoreBatch bounds one ignore or restore request.
const maxIgnoreBatch = 1000

// SetSourcesIgnored hides sources from discoveries (ignored) or restores them.
// The batch is atomic and idempotent; repeating an ignore keeps the first
// ignore time. A linked source cannot be ignored (ErrSourceLinked) and an
// unknown identity is ErrSourceNotFound.
func (s *Store) SetSourcesIgnored(ctx context.Context, identities []SourceIdentity, ignored bool) ([]Source, error) {
	if len(identities) == 0 || len(identities) > maxIgnoreBatch {
		return nil, fmt.Errorf("%w: ignore between 1 and %d sources", ErrValidation, maxIgnoreBatch)
	}
	seen := make(map[SourceIdentity]struct{}, len(identities))
	for _, identity := range identities {
		if err := validateIdentity(identity); err != nil {
			return nil, err
		}
		if _, exists := seen[identity]; exists {
			return nil, fmt.Errorf("%w: duplicate source in ignore request", ErrValidation)
		}
		seen[identity] = struct{}{}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin infra source ignore: %w", err)
	}
	defer tx.Rollback()
	now := formatTime(time.Now().UTC())
	for _, identity := range identities {
		if err := setSourceIgnoredTx(ctx, tx, identity, ignored, now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit infra source ignore: %w", err)
	}
	sources := make([]Source, 0, len(identities))
	for _, identity := range identities {
		source, err := loadSource(ctx, s.db, identity)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, nil
}

func setSourceIgnoredTx(ctx context.Context, tx *sql.Tx, identity SourceIdentity, ignored bool, now string) error {
	var linkedID sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT node_id FROM infra_sources WHERE kind=? AND instance=? AND source_key=?`,
		identity.Kind, identity.Instance, identity.Key).Scan(&linkedID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSourceNotFound
	}
	if err != nil {
		return fmt.Errorf("read infra source for ignore: %w", err)
	}
	switch {
	case !ignored:
		_, err = tx.ExecContext(ctx, `UPDATE infra_sources SET ignored_at='' WHERE kind=? AND instance=? AND source_key=?`,
			identity.Kind, identity.Instance, identity.Key)
	case linkedID.Valid:
		return ErrSourceLinked
	default:
		_, err = tx.ExecContext(ctx, `UPDATE infra_sources SET ignored_at=? WHERE kind=? AND instance=? AND source_key=? AND ignored_at=''`,
			now, identity.Kind, identity.Instance, identity.Key)
	}
	if err != nil {
		return fmt.Errorf("record infra source ignore: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `gofmt -w internal/infra/types.go internal/infra/sources.go internal/infra/nodes.go internal/infra/store.go internal/infra/ignore.go internal/infra/store_test.go`
Run: `go test ./internal/infra/ -run '^TestIgnoredSourcesLeaveDiscoveriesAndSurviveRefresh$' -count=1 -v`
Expected: PASS.
Run: `go test ./internal/infra/ -count=1`
Expected: `ok  	aurago/internal/infra`

- [ ] **Step 6: Commit**

```bash
git add internal/infra/types.go internal/infra/sources.go internal/infra/nodes.go internal/infra/store.go internal/infra/ignore.go internal/infra/store_test.go
git commit -m "feat(infra): persist ignored sources and keep them out of discoveries" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Snapshot and candidates hide ignored sources unless `include_ignored=1`

**Files:**
- Modify: `internal/infra/snapshot.go:11-82` (`Snapshot` → `Snapshot` + `SnapshotWith`)
- Modify: `internal/server/desktop_infra.go:309` (`desktopInfraSnapshot`), `:407-433` (`desktopInfraCandidates`), `:809-834` (`desktopInfraConfiguredSources`)
- Modify: `internal/server/desktop_infra_sources.go:409` (`desktopInfraReconcileProxmoxEdges`)
- Test: `internal/infra/store_test.go` (append)
- Create: `internal/server/desktop_infra_ignore_test.go`

- [ ] **Step 1: Impact analysis**

```bash
for symbol in Snapshot desktopInfraSnapshot desktopInfraCandidates desktopInfraConfiguredSources desktopInfraReconcileProxmoxEdges; do
  node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$symbol" --direction upstream --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
  graft callers "$symbol"
done
git grep -nE "(store\.Snapshot|desktopInfraSnapshot|desktopInfraCandidates|desktopInfraConfiguredSources|desktopInfraReconcileProxmoxEdges)\(" -- internal/infra 'internal/server/desktop_infra*'
```

Expected:
- Infra `Snapshot` ← `desktopInfraSnapshot` (`desktop_infra.go:309`), `desktopInfraReconcileProxmoxEdges` (`desktop_infra_sources.go:397`) and tests. Other packages' `Snapshot` symbols are unrelated.
- `desktopInfraSnapshot` / `desktopInfraCandidates` ← `handleDesktopInfra` and tests.
- `desktopInfraConfiguredSources` ← `desktopInfraCandidates` only.
- `desktopInfraReconcileProxmoxEdges` ← `desktopInfraRefreshProxmox`, the DELETE node route, `desktopInfraImport` and a test.

`Snapshot` may be HIGH (generic name); record it.

- [ ] **Step 2: Write the failing tests**

Append to `internal/infra/store_test.go`:

```go
func TestSnapshotListsIgnoredSourcesOnlyOnRequest(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	identity := SourceIdentity{Kind: "docker", Instance: "engine", Key: "chromecast"}
	if err := store.ObserveSources(ctx, "docker", "engine", []SourceObservation{
		{Identity: identity, Name: "chromecast", Observed: SourceObserved{Kind: NodeContainer, Name: "chromecast", Icon: "container"}},
		{Identity: SourceIdentity{Kind: "docker", Instance: "engine", Key: "web"}, Name: "web", Observed: SourceObserved{Kind: NodeContainer, Name: "web", Icon: "container"}},
	}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSourcesIgnored(ctx, []SourceIdentity{identity}, true); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Snapshot(ctx)
	if err != nil || len(snapshot.Sources) != 1 || snapshot.Sources[0].Key != "web" {
		t.Fatalf("default snapshot sources = %#v, %v; want the ignored source left out", snapshot.Sources, err)
	}
	withIgnored, err := store.SnapshotWith(ctx, SnapshotOptions{IncludeIgnored: true})
	if err != nil || len(withIgnored.Sources) != 2 {
		t.Fatalf("snapshot with ignored = %#v, %v", withIgnored.Sources, err)
	}
	for _, source := range withIgnored.Sources {
		if source.Ignored != (source.Key == "chromecast") {
			t.Fatalf("source %s ignored = %v", source.Key, source.Ignored)
		}
	}
}
```

Create `internal/server/desktop_infra_ignore_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/infra"
	"aurago/internal/inventory"
	"aurago/internal/tools"
)

// desktopInfraEmptyDocker answers a Docker container list without containers.
func desktopInfraEmptyDocker(context.Context, tools.DockerConfig, string, string, string) ([]byte, int, error) {
	return []byte("[]"), http.StatusOK, nil
}

func TestDesktopInfraCandidatesAndSnapshotHideIgnoredSourcesByDefault(t *testing.T) {
	s, cfg, store, runtime := newDesktopInfraRefreshFixture(t)
	runtime.dockerRequest = desktopInfraEmptyDocker
	ctx := context.Background()
	ignoredKey := ""
	for _, name := range []string{"Office TV", "Kitchen speaker"} {
		id, err := inventory.CreateDevice(s.InventoryDB, name, "device", inventory.ProtocolNone, "192.0.2.40", 0, "", "", "", "", nil, "")
		if err != nil {
			t.Fatal(err)
		}
		ignoredKey = id
	}
	desktopInfraRefreshRegistry(ctx, s, cfg, store, runtime, true)
	if _, err := store.SetSourcesIgnored(ctx, []infra.SourceIdentity{{Kind: "registry", Instance: "inventory", Key: ignoredKey}}, true); err != nil {
		t.Fatal(err)
	}
	candidates := func(query string) []infra.Candidate {
		t.Helper()
		response := httptest.NewRecorder()
		desktopInfraCandidates(response, httptest.NewRequest(http.MethodGet, desktopInfraBasePath+"/candidates"+query, nil), s, cfg, store, runtime)
		if response.Code != http.StatusOK {
			t.Fatalf("candidates%s = %d %s", query, response.Code, response.Body.String())
		}
		var payload struct {
			Candidates []infra.Candidate `json:"candidates"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		return payload.Candidates
	}
	if got := candidates(""); len(got) != 1 || got[0].Node.Name != "Office TV" {
		t.Fatalf("default candidates = %+v; want the ignored speaker left out", got)
	}
	all := candidates("?include_ignored=1")
	if len(all) != 2 {
		t.Fatalf("candidates with ignored = %+v; want both", all)
	}
	for _, candidate := range all {
		if candidate.Ignored != (candidate.Source.Key == ignoredKey) {
			t.Fatalf("candidate %q ignored = %v", candidate.Node.Name, candidate.Ignored)
		}
	}
	sources := func(query string) int {
		t.Helper()
		response := httptest.NewRecorder()
		desktopInfraSnapshot(response, httptest.NewRequest(http.MethodGet, desktopInfraBasePath+"/snapshot"+query, nil), s, cfg, store, runtime)
		if response.Code != http.StatusOK {
			t.Fatalf("snapshot%s = %d %s", query, response.Code, response.Body.String())
		}
		var payload struct {
			Sources []infra.Source `json:"sources"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		return len(payload.Sources)
	}
	if got := sources(""); got != 1 {
		t.Fatalf("default snapshot sources = %d; want 1", got)
	}
	if got := sources("?include_ignored=1"); got != 2 {
		t.Fatalf("snapshot sources with ignored = %d; want 2", got)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/infra/ -run '^TestSnapshotListsIgnoredSourcesOnlyOnRequest$' -count=1 -v`
Expected: FAIL (build error: `store.SnapshotWith undefined`).
Run: `go test ./internal/server/ -run '^TestDesktopInfraCandidatesAndSnapshotHideIgnoredSourcesByDefault$' -count=1 -v`
Expected: FAIL with `candidates with ignored = [...]; want both`.

- [ ] **Step 4: Implement**

In `internal/infra/snapshot.go`, replace the whole `func (s *Store) Snapshot(ctx context.Context) (Snapshot, error) { ... }` with:

```go
// SnapshotOptions selects optional snapshot content.
type SnapshotOptions struct {
	// IncludeIgnored adds sources the operator ignored; they are never linked.
	IncludeIgnored bool
}

// Snapshot returns the stored graph without ignored sources.
func (s *Store) Snapshot(ctx context.Context) (Snapshot, error) {
	return s.SnapshotWith(ctx, SnapshotOptions{})
}

// SnapshotWith returns the stored graph with optional content.
func (s *Store) SnapshotWith(ctx context.Context, options SnapshotOptions) (Snapshot, error) {
	result := Snapshot{Nodes: []Node{}, Edges: []Edge{}, Sources: []Source{}}
	rows, err := s.db.QueryContext(ctx, `SELECT id, version, kind, name, icon, registry_id, parent_id, profile_json, placements_json
		FROM infra_nodes ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list infra nodes: %w", err)
	}
	byID := make(map[string]int)
	for rows.Next() {
		var node Node
		var profileJSON, placementsJSON string
		var parentID sql.NullString
		if err := rows.Scan(&node.ID, &node.Version, &node.Kind, &node.Name, &node.Icon, &node.RegistryID, &parentID, &profileJSON, &placementsJSON); err != nil {
			_ = rows.Close()
			return Snapshot{}, fmt.Errorf("read infra node: %w", err)
		}
		if parentID.Valid {
			node.ParentID = parentID.String
		}
		if err := decodeNodeJSON(&node, profileJSON, placementsJSON); err != nil {
			_ = rows.Close()
			return Snapshot{}, err
		}
		byID[node.ID] = len(result.Nodes)
		result.Nodes = append(result.Nodes, node)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return Snapshot{}, fmt.Errorf("read infra nodes: %w", err)
	}
	if err := rows.Close(); err != nil {
		return Snapshot{}, fmt.Errorf("close infra node list: %w", err)
	}
	// Unlinked removed observations are retained only for the retention window and
	// carry no graph information; they stay out of the editor snapshot. Ignored
	// sources (never linked) appear only on request.
	sourceQuery := `SELECT node_id, kind, instance, source_key, name, state, fetched_at, removed_at, observed_json, ignored_at
		FROM infra_sources WHERE (node_id IS NOT NULL OR state<>'removed')`
	if !options.IncludeIgnored {
		sourceQuery += ` AND ignored_at=''`
	}
	rows, err = s.db.QueryContext(ctx, sourceQuery+` ORDER BY kind, instance, source_key`)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list infra sources: %w", err)
	}
	for rows.Next() {
		var nodeID sql.NullString
		var source Source
		var fetchedAt, removedAt, observedJSON, ignoredAt string
		if err := rows.Scan(&nodeID, &source.Kind, &source.Instance, &source.Key, &source.Name, &source.State, &fetchedAt, &removedAt, &observedJSON, &ignoredAt); err != nil {
			_ = rows.Close()
			return Snapshot{}, fmt.Errorf("read infra source: %w", err)
		}
		if err := decodeSource(&source, fetchedAt, removedAt, observedJSON); err != nil {
			_ = rows.Close()
			return Snapshot{}, err
		}
		source.Ignored = ignoredAt != ""
		result.Sources = append(result.Sources, source)
		if nodeID.Valid {
			if i, exists := byID[nodeID.String]; exists {
				result.Nodes[i].Sources = append(result.Nodes[i].Sources, source)
			}
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return Snapshot{}, fmt.Errorf("read infra sources: %w", err)
	}
	if err := rows.Close(); err != nil {
		return Snapshot{}, fmt.Errorf("close infra source list: %w", err)
	}
	result.Edges, err = s.listEdges(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	return result, nil
}
```

In `internal/server/desktop_infra.go`, in `desktopInfraSnapshot`, replace:

```go
	snapshot, err := store.Snapshot(r.Context())
```

with:

```go
	snapshot, err := store.SnapshotWith(r.Context(), infra.SnapshotOptions{IncludeIgnored: r.URL.Query().Get("include_ignored") == "1"})
```

In `desktopInfraCandidates`, replace:

```go
	sources, err := desktopInfraConfiguredSources(r.Context(), s, cfg, store)
```

with:

```go
	// Ignored sources stay out of discoveries unless the caller asks for them;
	// they then carry ignored: true.
	includeIgnored := r.URL.Query().Get("include_ignored") == "1"
	sources, err := desktopInfraConfiguredSources(r.Context(), s, cfg, store, includeIgnored)
```

Replace the whole `func desktopInfraConfiguredSources(...) { ... }` with:

```go
func desktopInfraConfiguredSources(ctx context.Context, s *Server, cfg *config.Config, store *infra.Store, includeIgnored bool) ([]infra.Source, error) {
	sources := make([]infra.Source, 0)
	providers := []struct {
		kind, instance string
		enabled        bool
	}{
		{"registry", "inventory", s.InventoryDB != nil},
		{"docker", desktopInfraDockerInstance(cfg), cfg.Docker.Enabled},
		{"proxmox", desktopInfraProxmoxInstance(cfg), cfg.Proxmox.Enabled},
		{"fritzbox", desktopInfraFritzInstance(cfg), desktopInfraFritzAllowed(cfg)},
	}
	list := store.ListSources
	if includeIgnored {
		list = store.ListSourcesIncludingIgnored
	}
	for _, provider := range providers {
		if !provider.enabled {
			continue
		}
		found, err := list(ctx, provider.kind, provider.instance)
		if err != nil {
			return nil, err
		}
		sources = append(sources, found...)
	}
	if sources == nil {
		sources = []infra.Source{}
	}
	return sources, nil
}
```

In `internal/server/desktop_infra_sources.go`, in `desktopInfraReconcileProxmoxEdges`, replace:

```go
	sources, err := store.ListSources(ctx, "proxmox", instance)
```

with:

```go
	// Keep reconciling ignored guests too so provider edges never go stale.
	sources, err := store.ListSourcesIncludingIgnored(ctx, "proxmox", instance)
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `gofmt -w internal/infra/snapshot.go internal/infra/store_test.go internal/server/desktop_infra.go internal/server/desktop_infra_sources.go internal/server/desktop_infra_ignore_test.go`
Run: `go test ./internal/infra/ -run '^TestSnapshotListsIgnoredSourcesOnlyOnRequest$' -count=1 -v`
Expected: PASS.
Run: `go test ./internal/server/ -run 'Infra' -count=1`
Expected: `ok  	aurago/internal/server`

- [ ] **Step 6: Commit**

```bash
git add internal/infra/snapshot.go internal/infra/store_test.go internal/server/desktop_infra.go internal/server/desktop_infra_sources.go internal/server/desktop_infra_ignore_test.go
git commit -m "feat(infra): hide ignored sources from snapshot and candidates by default" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Remove-and-ignore and the ignore route handler

**Files:**
- Modify: `internal/infra/nodes.go:209-255` (`DeleteNode` → `DeleteNode`, `DeleteNodeAndIgnoreSources`, `deleteNode`, `linkedSourceIdentities`)
- Modify: `internal/server/desktop_infra.go:502-525` (DELETE branch of `desktopInfraNodeRoute`), add `desktopInfraFlagQuery` after `desktopInfraVersionQuery` (`:687-695`)
- Create: `internal/server/desktop_infra_ignore.go`
- Test: `internal/infra/store_test.go` (append), `internal/server/desktop_infra_ignore_test.go` (imports + append)

- [ ] **Step 1: Impact analysis**

```bash
for symbol in DeleteNode desktopInfraNodeRoute; do
  node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$symbol" --direction upstream --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
  graft callers "$symbol"
done
git grep -nE "(\.DeleteNode|desktopInfraNodeRoute)\(" -- internal/infra 'internal/server/desktop_infra*'
```

Expected: `DeleteNode` ← DELETE branch (`desktop_infra.go:512`), `store_test.go:115` and `desktop_infra_revocation_test.go:174`. The agent and memory `kg.DeleteNode` callers are a different type. `desktopInfraNodeRoute` ← `handleDesktopInfra` (`desktop_infra.go:81`) and `TestDesktopInfraDeleteRefusesANodeWithChildren`. Risk MEDIUM.

- [ ] **Step 2: Write the failing tests**

Append to `internal/infra/store_test.go`:

```go
func TestDeleteNodeAndIgnoreSourcesIsAtomic(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	identity := SourceIdentity{Kind: "docker", Instance: "engine", Key: "duplicate-tv"}
	if err := store.ObserveSources(ctx, "docker", "engine", []SourceObservation{{
		Identity: identity, Name: "tv", Observed: SourceObserved{Kind: NodeContainer, Name: "tv", Icon: "container"},
	}}, true); err != nil {
		t.Fatal(err)
	}
	candidate, err := store.GetCandidate(ctx, CandidateID(identity))
	if err != nil {
		t.Fatal(err)
	}
	node, err := store.CreateImportedNode(ctx, candidate.Node, identity)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DeleteNodeAndIgnoreSources(ctx, node.ID, node.Version+1); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale delete = %v; want conflict", err)
	}
	if source, err := store.GetSource(ctx, identity); err != nil || source.Ignored {
		t.Fatalf("a refused delete ignored the source: %#v, %v", source, err)
	}
	ignored, err := store.DeleteNodeAndIgnoreSources(ctx, node.ID, node.Version)
	if err != nil || len(ignored) != 1 || ignored[0] != identity {
		t.Fatalf("delete and ignore = %#v, %v; want the linked source", ignored, err)
	}
	if _, err := store.GetNode(ctx, node.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted node lookup = %v; want not found", err)
	}
	if source, err := store.GetSource(ctx, identity); err != nil || !source.Ignored {
		t.Fatalf("source after delete and ignore = %#v, %v; want ignored", source, err)
	}
	if listed, err := store.ListSources(ctx, "docker", "engine"); err != nil || len(listed) != 0 {
		t.Fatalf("discoverable sources = %#v, %v; want none", listed, err)
	}
}
```

In `internal/server/desktop_infra_ignore_test.go`, replace the import block with:

```go
import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/infra"
	"aurago/internal/inventory"
	"aurago/internal/tools"
)
```

and append:

```go
func TestDesktopInfraIgnoreRouteAndRemoveAndIgnore(t *testing.T) {
	s, cfg, store, runtime := newDesktopInfraRefreshFixture(t)
	ctx := context.Background()
	instance := desktopInfraDockerInstance(cfg)
	free := infra.SourceIdentity{Kind: "docker", Instance: instance, Key: strings.Repeat("a1", 32)}
	linked := infra.SourceIdentity{Kind: "docker", Instance: instance, Key: strings.Repeat("b2", 32)}
	if err := store.ObserveSources(ctx, "docker", instance, []infra.SourceObservation{
		{Identity: free, Name: "chromecast", Observed: infra.SourceObserved{Kind: infra.NodeContainer, Name: "chromecast", Icon: "container"}},
		{Identity: linked, Name: "web", Observed: infra.SourceObserved{Kind: infra.NodeContainer, Name: "web", Icon: "container"}},
	}, true); err != nil {
		t.Fatal(err)
	}
	candidate, err := store.GetCandidate(ctx, infra.CandidateID(linked))
	if err != nil {
		t.Fatal(err)
	}
	node, err := store.CreateImportedNode(ctx, candidate.Node, linked)
	if err != nil {
		t.Fatal(err)
	}
	identityJSON := func(identity infra.SourceIdentity) string {
		t.Helper()
		data, err := json.Marshal(identity)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	ignore := func(body string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		desktopInfraIgnoreSources(response, httptest.NewRequest(http.MethodPut, desktopInfraBasePath+"/sources/ignore", strings.NewReader(body)), cfg, store)
		return response
	}
	for _, tc := range []struct {
		name, body, code string
		status           int
	}{
		{"linked source", `{"sources":[` + identityJSON(linked) + `],"ignored":true}`, "source_linked", http.StatusConflict},
		{"unknown source", `{"sources":[{"kind":"docker","instance":"` + instance + `","key":"missing"}],"ignored":true}`, "not_found", http.StatusNotFound},
		{"missing flag", `{"sources":[` + identityJSON(free) + `]}`, "invalid_request", http.StatusBadRequest},
	} {
		if response := ignore(tc.body); response.Code != tc.status || !strings.Contains(response.Body.String(), tc.code) {
			t.Fatalf("%s = %d %s; want %d %s", tc.name, response.Code, response.Body.String(), tc.status, tc.code)
		}
	}
	response := ignore(`{"sources":[` + identityJSON(free) + `],"ignored":true}`)
	var ignored struct {
		Sources []infra.Source `json:"sources"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &ignored); err != nil || response.Code != http.StatusOK || len(ignored.Sources) != 1 || !ignored.Sources[0].Ignored {
		t.Fatalf("ignore = %d %s, %v", response.Code, response.Body.String(), err)
	}

	removePath := desktopInfraBasePath + "/nodes/" + node.ID + "?version=" + strconv.FormatInt(node.Version, 10)
	invalid := httptest.NewRecorder()
	desktopInfraNodeRoute(invalid, httptest.NewRequest(http.MethodDelete, removePath+"&ignore_sources=yes", nil), s, cfg, store, runtime, node.ID)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid ignore flag = %d %s; want 400", invalid.Code, invalid.Body.String())
	}
	removed := httptest.NewRecorder()
	desktopInfraNodeRoute(removed, httptest.NewRequest(http.MethodDelete, removePath+"&ignore_sources=1", nil), s, cfg, store, runtime, node.ID)
	var payload struct {
		Status  string                 `json:"status"`
		Ignored []infra.SourceIdentity `json:"ignored"`
	}
	if err := json.Unmarshal(removed.Body.Bytes(), &payload); err != nil || removed.Code != http.StatusOK {
		t.Fatalf("remove and ignore = %d %s, %v", removed.Code, removed.Body.String(), err)
	}
	if payload.Status != "deleted" || len(payload.Ignored) != 1 || payload.Ignored[0] != linked {
		t.Fatalf("remove and ignore answer = %#v", payload)
	}
	if source, err := store.GetSource(ctx, linked); err != nil || !source.Ignored {
		t.Fatalf("source after remove and ignore = %#v, %v", source, err)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/infra/ -run '^TestDeleteNodeAndIgnoreSourcesIsAtomic$' -count=1 -v`
Expected: FAIL (build error: `store.DeleteNodeAndIgnoreSources undefined`).
Run: `go test ./internal/server/ -run '^TestDesktopInfraIgnoreRouteAndRemoveAndIgnore$' -count=1 -v`
Expected: FAIL (build error: `undefined: desktopInfraIgnoreSources`).

- [ ] **Step 4: Implement**

In `internal/infra/nodes.go`, replace the whole `func (s *Store) DeleteNode(ctx context.Context, id string, version int64) error { ... }` with:

```go
// DeleteNode removes a node and unlinks its sources.
func (s *Store) DeleteNode(ctx context.Context, id string, version int64) error {
	_, err := s.deleteNode(ctx, id, version, false)
	return err
}

// DeleteNodeAndIgnoreSources removes a node and, in the same transaction,
// unlinks and ignores every source it was linked to. It returns those sources.
func (s *Store) DeleteNodeAndIgnoreSources(ctx context.Context, id string, version int64) ([]SourceIdentity, error) {
	return s.deleteNode(ctx, id, version, true)
}

func (s *Store) deleteNode(ctx context.Context, id string, version int64, ignore bool) ([]SourceIdentity, error) {
	if err := validateText("node id", id, 128, true); err != nil {
		return nil, err
	}
	if version <= 0 {
		return nil, fmt.Errorf("%w: node id and version are required", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin infra node removal: %w", err)
	}
	defer tx.Rollback()
	var current int64
	if err := tx.QueryRowContext(ctx, `SELECT version FROM infra_nodes WHERE id=?`, id).Scan(&current); err == sql.ErrNoRows {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("read infra node version: %w", err)
	}
	if current != version {
		return nil, ErrConflict
	}
	var children bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM infra_nodes WHERE parent_id=?)`, id).Scan(&children); err != nil {
		return nil, fmt.Errorf("check infra node children: %w", err)
	}
	if children {
		return nil, ErrHasChildren
	}
	ignored := []SourceIdentity{}
	if ignore {
		ignored, err = linkedSourceIdentities(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE infra_sources SET node_id=NULL, ignored_at=? WHERE node_id=?`, formatTime(time.Now().UTC()), id); err != nil {
			return nil, fmt.Errorf("unlink and ignore infra node sources: %w", err)
		}
	} else if _, err := tx.ExecContext(ctx, `UPDATE infra_sources SET node_id=NULL WHERE node_id=?`, id); err != nil {
		return nil, fmt.Errorf("unlink infra node sources: %w", err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM infra_nodes WHERE id=? AND version=?`, id, version)
	if err != nil {
		return nil, fmt.Errorf("delete infra node: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("count deleted infra nodes: %w", err)
	}
	if n != 1 {
		return nil, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit infra node removal: %w", err)
	}
	return ignored, nil
}

func linkedSourceIdentities(ctx context.Context, tx *sql.Tx, nodeID string) ([]SourceIdentity, error) {
	rows, err := tx.QueryContext(ctx, `SELECT kind, instance, source_key FROM infra_sources WHERE node_id=? ORDER BY kind, instance, source_key`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("list infra node sources: %w", err)
	}
	defer rows.Close()
	identities := []SourceIdentity{}
	for rows.Next() {
		var identity SourceIdentity
		if err := rows.Scan(&identity.Kind, &identity.Instance, &identity.Key); err != nil {
			return nil, fmt.Errorf("read infra node source: %w", err)
		}
		identities = append(identities, identity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read infra node sources: %w", err)
	}
	return identities, nil
}
```

In `internal/server/desktop_infra.go`, in `desktopInfraNodeRoute`, replace the whole `case http.MethodDelete:` branch (from `case http.MethodDelete:` through `desktopInfraJSON(w, http.StatusOK, map[string]any{"status": "deleted", "id": nodeID})`) with:

```go
	case http.MethodDelete:
		version, ok := desktopInfraVersionQuery(w, r)
		if !ok {
			return
		}
		ignoreSources, ok := desktopInfraFlagQuery(w, r, "ignore_sources")
		if !ok {
			return
		}
		node, err := store.GetNode(r.Context(), nodeID)
		if err != nil {
			desktopInfraStoreError(w, err)
			return
		}
		ignored := []infra.SourceIdentity{}
		if ignoreSources {
			ignored, err = store.DeleteNodeAndIgnoreSources(r.Context(), nodeID, version)
		} else {
			err = store.DeleteNode(r.Context(), nodeID, version)
		}
		if err != nil {
			desktopInfraStoreError(w, err)
			return
		}
		instances := make(map[string]struct{})
		for _, source := range node.Sources {
			if source.Kind == "proxmox" {
				instances[source.Instance] = struct{}{}
			}
		}
		for instance := range instances {
			_ = desktopInfraReconcileProxmoxEdges(r.Context(), store, instance)
		}
		response := map[string]any{"status": "deleted", "id": nodeID}
		if ignoreSources {
			response["ignored"] = ignored
		}
		desktopInfraJSON(w, http.StatusOK, response)
```

Directly after `func desktopInfraVersionQuery(...) { ... }`, insert:

```go
// desktopInfraFlagQuery reads an optional boolean query flag ("1"/"true" or
// "0"/"false"; absent means false). Anything else is refused so a typo never
// silently drops the requested behaviour.
func desktopInfraFlagQuery(w http.ResponseWriter, r *http.Request, name string) (bool, bool) {
	switch r.URL.Query().Get(name) {
	case "":
		return false, true
	case "1", "true":
		return true, true
	case "0", "false":
		return false, true
	default:
		desktopInfraError(w, http.StatusBadRequest, "invalid_request", "Query flag "+name+" must be 1 or 0")
		return false, false
	}
}
```

Create `internal/server/desktop_infra_ignore.go`:

```go
package server

import (
	"errors"
	"net/http"

	"aurago/internal/config"
	"aurago/internal/infra"
)

type desktopInfraIgnoreRequest struct {
	Sources []infra.SourceIdentity `json:"sources"`
	Ignored *bool                  `json:"ignored"`
}

// desktopInfraIgnoreSources hides sources from discoveries (ignored: true) or
// restores them (ignored: false). The flag is required so an incomplete body
// never restores by accident.
func desktopInfraIgnoreSources(w http.ResponseWriter, r *http.Request, cfg *config.Config, store *infra.Store) {
	var request desktopInfraIgnoreRequest
	if !desktopInfraDecode(w, r, &request) {
		return
	}
	if request.Ignored == nil || len(request.Sources) == 0 || len(request.Sources) > 1000 {
		desktopInfraError(w, http.StatusBadRequest, "invalid_request", "One to 1000 sources and the ignored flag are required")
		return
	}
	sources, err := store.SetSourcesIgnored(r.Context(), request.Sources, *request.Ignored)
	if err != nil {
		if errors.Is(err, infra.ErrSourceLinked) {
			desktopInfraError(w, http.StatusConflict, "source_linked", "A source linked to an asset cannot be ignored")
			return
		}
		desktopInfraStoreError(w, err)
		return
	}
	desktopInfraJSON(w, http.StatusOK, map[string]any{"sources": desktopInfraProjectSourceStates(cfg, sources)})
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `gofmt -w internal/infra/nodes.go internal/infra/store_test.go internal/server/desktop_infra.go internal/server/desktop_infra_ignore.go internal/server/desktop_infra_ignore_test.go`
Run: `go test ./internal/infra/ -run '^(TestDeleteNodeAndIgnoreSourcesIsAtomic|TestManualNodeCreatesCanonicalNonExecutableRegistryDevice)$' -count=1 -v`
Expected: PASS (2 tests).
Run: `go test ./internal/server/ -run '^(TestDesktopInfraIgnoreRouteAndRemoveAndIgnore|TestDesktopInfraDeleteRefusesANodeWithChildren|TestDesktopInfraNodeUnlinkDuringPreflightBlocksProviderAction)$' -count=1 -v`
Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/infra/nodes.go internal/infra/store_test.go internal/server/desktop_infra.go internal/server/desktop_infra_ignore.go internal/server/desktop_infra_ignore_test.go
git commit -m "feat(infra): add remove-and-ignore and the source ignore handler" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Docker engine host mappings (store, handler, snapshot `source_hosts`)

**Files:**
- Modify: `internal/infra/types.go:207-212` (`Snapshot`), add `SourceHost`
- Create: `internal/infra/source_hosts.go`
- Modify: `internal/infra/snapshot.go` (`SnapshotWith`: result literal and tail)
- Modify: `internal/server/desktop_infra.go:334-343` (`desktopInfraSnapshotResponse`)
- Create: `internal/server/desktop_infra_hosts.go`
- Test: `internal/infra/store_test.go` (append)
- Create: `internal/server/desktop_infra_hosts_test.go`

- [ ] **Step 1: Impact analysis**

```bash
for symbol in SnapshotWith desktopInfraSnapshotResponse Snapshot; do
  node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$symbol" --direction upstream --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
  graft callers "$symbol"
done
git grep -nE "(SnapshotWith|desktopInfraSnapshotResponse)\(|infra\.Snapshot\{" -- internal/infra 'internal/server/desktop_infra*'
```

Expected: `SnapshotWith` ← `Snapshot` and `desktopInfraSnapshot`. `desktopInfraSnapshotResponse` ← `desktopInfraSnapshot`. `infra.Snapshot{` literals appear in `desktop_infra_test.go:456` and `desktop_infra_candidates_test.go:140`; they are keyed and unaffected. Risk MEDIUM (snapshot payload); the change only adds the key `source_hosts`.

- [ ] **Step 2: Write the failing tests**

Append to `internal/infra/store_test.go`:

```go
func TestSourceHostMappingUsesVersionCAS(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	host, err := store.CreateNode(ctx, testNode(NodeDevice, "TheBrain", "tower-server"))
	if err != nil {
		t.Fatal(err)
	}
	room, err := store.CreateNode(ctx, testNode(NodeRoom, "Office", "room"))
	if err != nil {
		t.Fatal(err)
	}
	identity := SourceIdentity{Kind: "docker", Instance: "docker_engine", Key: "web"}
	if err := store.ObserveSources(ctx, identity.Kind, identity.Instance, []SourceObservation{{
		Identity: identity, Name: "web", Observed: SourceObserved{Kind: NodeContainer, Name: "web", Icon: "container"},
	}}, true); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		host SourceHost
		want error
	}{
		{"unobserved engine", SourceHost{Kind: "docker", Instance: "docker_unknown", NodeID: host.ID}, ErrSourceNotFound},
		{"other source kind", SourceHost{Kind: "proxmox", Instance: "docker_engine", NodeID: host.ID}, ErrValidation},
		{"place as host", SourceHost{Kind: "docker", Instance: "docker_engine", NodeID: room.ID}, ErrValidation},
		{"missing node", SourceHost{Kind: "docker", Instance: "docker_engine", NodeID: "missing"}, ErrValidation},
	} {
		if _, err := store.SetSourceHost(ctx, tc.host); !errors.Is(err, tc.want) {
			t.Fatalf("%s error = %v; want %v", tc.name, err, tc.want)
		}
	}
	created, err := store.SetSourceHost(ctx, SourceHost{Kind: "docker", Instance: "docker_engine", NodeID: host.ID, InferredNodeID: "forged"})
	if err != nil || created.Version != 1 || created.NodeID != host.ID || created.InferredNodeID != "" {
		t.Fatalf("created mapping = %#v, %v", created, err)
	}
	if _, err := store.SetSourceHost(ctx, SourceHost{Kind: "docker", Instance: "docker_engine"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("second create error = %v; want conflict", err)
	}
	cleared, err := store.SetSourceHost(ctx, SourceHost{Kind: "docker", Instance: "docker_engine", Version: 1})
	if err != nil || cleared.Version != 2 || cleared.NodeID != "" {
		t.Fatalf("cleared mapping = %#v, %v", cleared, err)
	}
	if _, err := store.SetSourceHost(ctx, SourceHost{Kind: "docker", Instance: "docker_engine", NodeID: host.ID, Version: 2}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Snapshot(ctx)
	if err != nil || len(snapshot.SourceHosts) != 1 || snapshot.SourceHosts[0].NodeID != host.ID || snapshot.SourceHosts[0].Version != 3 {
		t.Fatalf("snapshot source hosts = %#v, %v", snapshot.SourceHosts, err)
	}
	// Deleting the host node cascades the mapping; the observed engine stays listed.
	if err := store.DeleteNode(ctx, host.ID, host.Version); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Snapshot(ctx)
	if err != nil || len(snapshot.SourceHosts) != 1 || snapshot.SourceHosts[0].NodeID != "" || snapshot.SourceHosts[0].Version != 0 {
		t.Fatalf("source hosts after deleting the host = %#v, %v; want an unassigned observed engine", snapshot.SourceHosts, err)
	}
}
```

Create `internal/server/desktop_infra_hosts_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/infra"
)

func TestDesktopInfraSourceHostRouteUsesVersionCAS(t *testing.T) {
	_, cfg, store, runtime := newDesktopInfraRefreshFixture(t)
	ctx := context.Background()
	instance := desktopInfraDockerInstance(cfg)
	if err := store.ObserveSources(ctx, "docker", instance, []infra.SourceObservation{{
		Identity: infra.SourceIdentity{Kind: "docker", Instance: instance, Key: strings.Repeat("c3", 32)}, Name: "web",
		Observed: infra.SourceObserved{Kind: infra.NodeContainer, Name: "web", Icon: "container"},
	}}, true); err != nil {
		t.Fatal(err)
	}
	host, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeDevice, Name: "TheBrain", Icon: "tower-server"})
	if err != nil {
		t.Fatal(err)
	}
	call := func(body string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		desktopInfraSetSourceHost(response, httptest.NewRequest(http.MethodPut, desktopInfraBasePath+"/source-hosts", strings.NewReader(body)), store)
		return response
	}
	response := call(`{"kind":"docker","instance":"` + instance + `","node_id":"` + host.ID + `","version":0}`)
	if response.Code != http.StatusOK {
		t.Fatalf("assign host = %d %s", response.Code, response.Body.String())
	}
	var payload struct {
		SourceHost infra.SourceHost `json:"source_host"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.SourceHost.NodeID != host.ID || payload.SourceHost.Version != 1 {
		t.Fatalf("assigned source host = %#v", payload.SourceHost)
	}
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"stale version", `{"kind":"docker","instance":"` + instance + `","node_id":"","version":0}`, http.StatusConflict},
		{"unknown engine", `{"kind":"docker","instance":"docker_missing","node_id":"","version":0}`, http.StatusNotFound},
		{"runtime field", `{"kind":"docker","instance":"` + instance + `","node_id":"","version":1,"inferred_node_id":"x"}`, http.StatusBadRequest},
	} {
		if got := call(tc.body); got.Code != tc.status {
			t.Fatalf("%s = %d %s; want %d", tc.name, got.Code, got.Body.String(), tc.status)
		}
	}
	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(desktopInfraSnapshotResponse(snapshot, runtime))
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		SourceHosts []infra.SourceHost `json:"source_hosts"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.SourceHosts) != 1 || decoded.SourceHosts[0].NodeID != host.ID || decoded.SourceHosts[0].Version != 1 {
		t.Fatalf("snapshot source_hosts = %#v", decoded.SourceHosts)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/infra/ -run '^TestSourceHostMappingUsesVersionCAS$' -count=1 -v`
Expected: FAIL (build error: `undefined: SourceHost`).
Run: `go test ./internal/server/ -run '^TestDesktopInfraSourceHostRouteUsesVersionCAS$' -count=1 -v`
Expected: FAIL (build error: `undefined: desktopInfraSetSourceHost`).

- [ ] **Step 4: Implement**

In `internal/infra/types.go`, replace:

```go
type Snapshot struct {
	Nodes        []Node       `json:"nodes"`
	Edges        []Edge       `json:"edges"`
	Sources      []Source     `json:"sources"`
	Capabilities Capabilities `json:"capabilities"`
}
```

with:

```go
// SourceHost maps a provider engine instance (Docker only) to the Infra device
// that runs it. InferredNodeID is a runtime projection and is never stored.
type SourceHost struct {
	Kind           string `json:"kind"`
	Instance       string `json:"instance"`
	NodeID         string `json:"node_id"`
	Version        int64  `json:"version"`
	InferredNodeID string `json:"inferred_node_id,omitempty"`
}

type Snapshot struct {
	Nodes        []Node       `json:"nodes"`
	Edges        []Edge       `json:"edges"`
	Sources      []Source     `json:"sources"`
	SourceHosts  []SourceHost `json:"source_hosts"`
	Capabilities Capabilities `json:"capabilities"`
}
```

Create `internal/infra/source_hosts.go`:

```go
package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// SetSourceHost assigns (NodeID) or clears ("") the device that runs one Docker
// engine instance. Version 0 creates the mapping; any other version must match
// the stored one. The instance must have been observed and only devices can
// host an engine. InferredNodeID is a runtime projection and is ignored.
func (s *Store) SetSourceHost(ctx context.Context, host SourceHost) (SourceHost, error) {
	host.NodeID = strings.TrimSpace(host.NodeID)
	host.InferredNodeID = ""
	if host.Kind != "docker" {
		return SourceHost{}, fmt.Errorf("%w: only Docker engines can be assigned a host", ErrValidation)
	}
	if err := validateText("source instance", host.Instance, 256, true); err != nil {
		return SourceHost{}, err
	}
	if err := validateText("host node id", host.NodeID, 128, false); err != nil {
		return SourceHost{}, err
	}
	if host.Version < 0 {
		return SourceHost{}, fmt.Errorf("%w: source host version must not be negative", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SourceHost{}, fmt.Errorf("begin infra source host update: %w", err)
	}
	defer tx.Rollback()
	var observed bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM infra_sources WHERE kind=? AND instance=?)`,
		host.Kind, host.Instance).Scan(&observed); err != nil {
		return SourceHost{}, fmt.Errorf("check infra source host instance: %w", err)
	}
	if !observed {
		return SourceHost{}, ErrSourceNotFound
	}
	if host.NodeID != "" {
		var kind NodeKind
		err := tx.QueryRowContext(ctx, `SELECT kind FROM infra_nodes WHERE id=?`, host.NodeID).Scan(&kind)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return SourceHost{}, fmt.Errorf("%w: host node does not exist", ErrValidation)
		case err != nil:
			return SourceHost{}, fmt.Errorf("read infra source host node: %w", err)
		case kind != NodeDevice:
			return SourceHost{}, fmt.Errorf("%w: only devices can run a Docker engine", ErrValidation)
		}
	}
	now := formatTime(time.Now().UTC())
	var res sql.Result
	if host.Version == 0 {
		res, err = tx.ExecContext(ctx, `INSERT INTO infra_source_hosts(kind, instance, node_id, version, updated_at)
			VALUES(?,?,?,1,?) ON CONFLICT(kind, instance) DO NOTHING`, host.Kind, host.Instance, nullIfEmpty(host.NodeID), now)
	} else {
		res, err = tx.ExecContext(ctx, `UPDATE infra_source_hosts SET node_id=?, version=version+1, updated_at=?
			WHERE kind=? AND instance=? AND version=?`, nullIfEmpty(host.NodeID), now, host.Kind, host.Instance, host.Version)
	}
	if err != nil {
		return SourceHost{}, fmt.Errorf("save infra source host: %w", err)
	}
	changed, err := res.RowsAffected()
	if err != nil {
		return SourceHost{}, fmt.Errorf("count saved infra source hosts: %w", err)
	}
	if changed != 1 {
		return SourceHost{}, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return SourceHost{}, fmt.Errorf("commit infra source host update: %w", err)
	}
	host.Version++
	return host, nil
}

func (s *Store) listSourceHosts(ctx context.Context) ([]SourceHost, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT kind, instance, COALESCE(node_id, ''), version FROM infra_source_hosts ORDER BY kind, instance`)
	if err != nil {
		return nil, fmt.Errorf("list infra source hosts: %w", err)
	}
	defer rows.Close()
	hosts := make([]SourceHost, 0)
	for rows.Next() {
		var host SourceHost
		if err := rows.Scan(&host.Kind, &host.Instance, &host.NodeID, &host.Version); err != nil {
			return nil, fmt.Errorf("read infra source host: %w", err)
		}
		hosts = append(hosts, host)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read infra source hosts: %w", err)
	}
	return hosts, nil
}

// snapshotSourceHosts returns every stored mapping plus an unassigned entry
// (version 0) for each Docker instance seen among the snapshot's sources.
func (s *Store) snapshotSourceHosts(ctx context.Context, sources []Source) ([]SourceHost, error) {
	hosts, err := s.listSourceHosts(ctx)
	if err != nil {
		return nil, err
	}
	known := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		known[host.Kind+"\x00"+host.Instance] = struct{}{}
	}
	for _, source := range sources {
		key := source.Kind + "\x00" + source.Instance
		if _, exists := known[key]; exists || source.Kind != "docker" {
			continue
		}
		known[key] = struct{}{}
		hosts = append(hosts, SourceHost{Kind: source.Kind, Instance: source.Instance})
	}
	sort.Slice(hosts, func(i, j int) bool {
		if hosts[i].Kind != hosts[j].Kind {
			return hosts[i].Kind < hosts[j].Kind
		}
		return hosts[i].Instance < hosts[j].Instance
	})
	return hosts, nil
}
```

In `internal/infra/snapshot.go`, in `SnapshotWith`, replace:

```go
	result := Snapshot{Nodes: []Node{}, Edges: []Edge{}, Sources: []Source{}}
```

with:

```go
	result := Snapshot{Nodes: []Node{}, Edges: []Edge{}, Sources: []Source{}, SourceHosts: []SourceHost{}}
```

and replace the tail:

```go
	result.Edges, err = s.listEdges(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	return result, nil
}
```

with:

```go
	result.Edges, err = s.listEdges(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	result.SourceHosts, err = s.snapshotSourceHosts(ctx, result.Sources)
	if err != nil {
		return Snapshot{}, err
	}
	return result, nil
}
```

In `internal/server/desktop_infra.go`, replace:

```go
func desktopInfraSnapshotResponse(snapshot infra.Snapshot, runtime *desktopInfraRuntime) map[string]any {
	response := map[string]any{
		"nodes": snapshot.Nodes, "edges": snapshot.Edges, "sources": snapshot.Sources,
		"capabilities": snapshot.Capabilities,
	}
```

with:

```go
func desktopInfraSnapshotResponse(snapshot infra.Snapshot, runtime *desktopInfraRuntime) map[string]any {
	sourceHosts := snapshot.SourceHosts
	if sourceHosts == nil {
		sourceHosts = []infra.SourceHost{}
	}
	response := map[string]any{
		"nodes": snapshot.Nodes, "edges": snapshot.Edges, "sources": snapshot.Sources,
		"source_hosts": sourceHosts, "capabilities": snapshot.Capabilities,
	}
```

Create `internal/server/desktop_infra_hosts.go`:

```go
package server

import (
	"net/http"

	"aurago/internal/infra"
)

type desktopInfraSourceHostRequest struct {
	Kind     string `json:"kind"`
	Instance string `json:"instance"`
	NodeID   string `json:"node_id"`
	Version  int64  `json:"version"`
}

// desktopInfraSetSourceHost assigns or clears the device that runs a Docker
// engine. Inferred hosts are runtime projections reported by the snapshot.
func desktopInfraSetSourceHost(w http.ResponseWriter, r *http.Request, store *infra.Store) {
	var request desktopInfraSourceHostRequest
	if !desktopInfraDecode(w, r, &request) {
		return
	}
	if request.NodeID != "" && !desktopInfraValidPathID(request.NodeID) {
		desktopInfraError(w, http.StatusBadRequest, "invalid_node_id", "Node ID is invalid")
		return
	}
	host, err := store.SetSourceHost(r.Context(), infra.SourceHost{
		Kind: request.Kind, Instance: request.Instance, NodeID: request.NodeID, Version: request.Version,
	})
	if err != nil {
		desktopInfraStoreError(w, err)
		return
	}
	desktopInfraJSON(w, http.StatusOK, map[string]any{"source_host": host})
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `gofmt -w internal/infra/types.go internal/infra/source_hosts.go internal/infra/snapshot.go internal/infra/store_test.go internal/server/desktop_infra.go internal/server/desktop_infra_hosts.go internal/server/desktop_infra_hosts_test.go`
Run: `go test ./internal/infra/ -run '^TestSourceHostMappingUsesVersionCAS$' -count=1 -v`
Expected: PASS.
Run: `go test ./internal/server/ -run '^(TestDesktopInfraSourceHostRouteUsesVersionCAS|TestDesktopInfraRouteRequiresAdminAndAppliesInfraReadOnly)$' -count=1 -v`
Expected: PASS (2 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/infra/types.go internal/infra/source_hosts.go internal/infra/snapshot.go internal/infra/store_test.go internal/server/desktop_infra.go internal/server/desktop_infra_hosts.go internal/server/desktop_infra_hosts_test.go
git commit -m "feat(infra): map Docker engines to their host device with version CAS" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Runtime-only guest host projection (`host_id`, `host_inferred`, `inferred_node_id`)

**Files:**
- Modify: `internal/infra/types.go:27-44` (`Node`)
- Modify: `internal/infra/validate.go:51-58` (`normalizeNodeRuntime`)
- Modify: `internal/server/desktop_infra_hosts.go` (whole file: handler plus projection)
- Modify: `internal/server/desktop_infra_projection.go:178-187` (`desktopInfraDecorateSnapshot`)
- Modify: `internal/server/desktop_infra_candidates.go:37-39` (`desktopInfraVisibleNodes`)
- Modify: `internal/server/desktop_infra.go:500` (node PUT), `:585` (import), `:651` (layout)
- Test: `internal/infra/store_test.go` (append), `internal/server/desktop_infra_hosts_test.go` (imports + append)

- [ ] **Step 1: Impact analysis**

```bash
for symbol in normalizeNodeRuntime Node desktopInfraDecorateSnapshot desktopInfraVisibleNodes desktopInfraNodeRoute desktopInfraImport desktopInfraSaveLayout desktopInfraSetSourceHost; do
  node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$symbol" --direction upstream --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
  graft callers "$symbol"
done
git grep -nE "(normalizeNodeRuntime|desktopInfraDecorateSnapshot|desktopInfraVisibleNodes|desktopInfraNodeRoute|desktopInfraImport|desktopInfraSaveLayout|desktopInfraSetSourceHost)\(" -- internal/infra 'internal/server/desktop_infra*'
```

Expected:
- `normalizeNodeRuntime` ← `validateNode`, `loadNode`, `decodeNodeJSON`.
- `desktopInfraDecorateSnapshot` ← `desktopInfraSnapshot` and two tests.
- `desktopInfraVisibleNodes` ← `desktopInfraDecorateSnapshot` and a test.
- The handlers ← `handleDesktopInfra` and tests.
- `desktopInfraSetSourceHost` ← its test only.

`Node` is a hub type and likely HIGH or CRITICAL. Record the warning; the change only adds two `omitempty` runtime fields that are reset on every store read and write.

- [ ] **Step 2: Write the failing tests**

Append to `internal/infra/store_test.go`:

```go
func TestHostProjectionFieldsAreRuntimeOnly(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	node := testNode(NodeContainer, "web", "container")
	node.HostID, node.HostInferred = "forged-host", true
	created, err := store.CreateNode(context.Background(), node)
	if err != nil {
		t.Fatal(err)
	}
	if created.HostID != "" || created.HostInferred {
		t.Fatalf("caller-supplied host projection survived: %#v", created)
	}
}
```

In `internal/server/desktop_infra_hosts_test.go`, replace the import block with:

```go
import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/infra"
)
```

and append:

```go
func TestDesktopInfraDockerEngineAddresses(t *testing.T) {
	local := []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("fe80::1"), netip.MustParseAddr("192.0.2.10")}
	for _, tc := range []struct {
		host string
		want []string
	}{
		{"", []string{"192.0.2.10"}},
		{"unix:///var/run/docker.sock", []string{"192.0.2.10"}},
		{"npipe:////./pipe/docker_engine", []string{"192.0.2.10"}},
		{"tcp://localhost:2375", []string{"192.0.2.10"}},
		{"tcp://127.0.0.1:2375", []string{"192.0.2.10"}},
		{"tcp://192.0.2.50:2375", []string{"192.0.2.50"}},
		{"tcp://docker-proxy:2375", nil},
		{"mock://docker", nil},
	} {
		var got []string
		for _, addr := range desktopInfraDockerEngineAddresses(tc.host, local) {
			got = append(got, addr.String())
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("engine addresses for %q = %v; want %v", tc.host, got, tc.want)
		}
	}
	nodes := []infra.Node{
		{ID: "a", Kind: infra.NodeDevice, Profile: infra.Profile{IPAddress: "192.0.2.50"}},
		{ID: "b", Kind: infra.NodeDevice, Profile: infra.Profile{Interfaces: []infra.NetworkInterface{{Name: "eth0", IPAddress: "192.0.2.50"}}}},
	}
	engine := []netip.Addr{netip.MustParseAddr("192.0.2.50")}
	if got := desktopInfraNodeWithAddress(nodes, engine); got != "" {
		t.Fatalf("ambiguous address inferred %q", got)
	}
	if got := desktopInfraNodeWithAddress(nodes[:1], engine); got != "a" {
		t.Fatalf("unique address inferred %q; want a", got)
	}
}

func TestDesktopInfraHostProjectionUsesProxmoxEdgesDockerMappingAndInference(t *testing.T) {
	s, cfg, store, _ := newDesktopInfraRefreshFixture(t)
	ctx := context.Background()
	original := desktopInfraLocalAddresses
	desktopInfraLocalAddresses = func() []netip.Addr {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("192.0.2.10")}
	}
	t.Cleanup(func() { desktopInfraLocalAddresses = original })
	cfg.Docker.Host = "unix:///var/run/docker.sock"
	dockerInstance := desktopInfraDockerInstance(cfg)

	importSource := func(observation infra.SourceObservation) infra.Node {
		t.Helper()
		if err := store.UpsertObservation(ctx, observation); err != nil {
			t.Fatal(err)
		}
		candidate, err := store.GetCandidate(ctx, infra.CandidateID(observation.Identity))
		if err != nil {
			t.Fatal(err)
		}
		node, err := store.CreateImportedNode(ctx, candidate.Node, observation.Identity)
		if err != nil {
			t.Fatal(err)
		}
		return node
	}
	observed := func(kind infra.NodeKind, name, icon string) infra.SourceObserved {
		return infra.SourceObserved{Kind: kind, Name: name, Icon: icon, Tags: []string{}, Interfaces: []infra.NetworkInterface{}}
	}
	guestObserved := observed(infra.NodeVM, "web-vm", "vm")
	guestObserved.Host = "pve-a"
	guest := importSource(infra.SourceObservation{Identity: infra.SourceIdentity{Kind: "proxmox", Instance: "pve-test", Key: "qemu:101"}, Name: "web-vm", Observed: guestObserved})
	hypervisor := importSource(infra.SourceObservation{Identity: infra.SourceIdentity{Kind: "proxmox", Instance: "pve-test", Key: "node:pve-a"}, Name: "pve-a", Observed: observed(infra.NodeDevice, "pve-a", "hypervisor")})
	if err := desktopInfraReconcileProxmoxEdges(ctx, store, "pve-test"); err != nil {
		t.Fatal(err)
	}
	container := importSource(infra.SourceObservation{Identity: infra.SourceIdentity{Kind: "docker", Instance: dockerInstance, Key: "c0ffee"}, Name: "radarr", Observed: observed(infra.NodeContainer, "radarr", "container")})
	brain, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeDevice, Name: "TheBrain", Icon: "tower-server", Profile: infra.Profile{IPAddress: "192.0.2.10"}})
	if err != nil {
		t.Fatal(err)
	}
	nas, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeDevice, Name: "NAS", Icon: "nas"})
	if err != nil {
		t.Fatal(err)
	}

	decorated := func() infra.Snapshot {
		t.Helper()
		snapshot, err := store.Snapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		desktopInfraDecorateSnapshot(ctx, s, cfg, &snapshot)
		return snapshot
	}
	nodeByID := func(snapshot infra.Snapshot, id string) infra.Node {
		t.Helper()
		for _, node := range snapshot.Nodes {
			if node.ID == id {
				return node
			}
		}
		t.Fatalf("node %s missing from the snapshot", id)
		return infra.Node{}
	}
	snapshot := decorated()
	if got := nodeByID(snapshot, guest.ID); got.HostID != hypervisor.ID || got.HostInferred {
		t.Fatalf("Proxmox guest host = %q inferred=%v; want %q from runs_on", got.HostID, got.HostInferred, hypervisor.ID)
	}
	if got := nodeByID(snapshot, container.ID); got.HostID != brain.ID || !got.HostInferred {
		t.Fatalf("Docker guest host = %q inferred=%v; want inferred %q", got.HostID, got.HostInferred, brain.ID)
	}
	if got := nodeByID(snapshot, brain.ID); got.HostID != "" {
		t.Fatalf("a device received a host: %q", got.HostID)
	}
	if len(snapshot.SourceHosts) != 1 || snapshot.SourceHosts[0].Instance != dockerInstance ||
		snapshot.SourceHosts[0].InferredNodeID != brain.ID || snapshot.SourceHosts[0].Version != 0 {
		t.Fatalf("source hosts = %#v", snapshot.SourceHosts)
	}
	if _, err := store.SetSourceHost(ctx, infra.SourceHost{Kind: "docker", Instance: dockerInstance, NodeID: nas.ID}); err != nil {
		t.Fatal(err)
	}
	if got := nodeByID(decorated(), container.ID); got.HostID != nas.ID || got.HostInferred {
		t.Fatalf("mapped Docker guest host = %q inferred=%v; want %q", got.HostID, got.HostInferred, nas.ID)
	}
	written := desktopInfraApplyHosts(ctx, store, cfg, []infra.Node{container})
	if written[0].HostID != nas.ID || written[0].HostInferred {
		t.Fatalf("write response host = %q inferred=%v; want %q", written[0].HostID, written[0].HostInferred, nas.ID)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/infra/ -run '^TestHostProjectionFieldsAreRuntimeOnly$' -count=1 -v`
Expected: FAIL (build error: `node.HostID undefined`).
Run: `go test ./internal/server/ -run '^(TestDesktopInfraDockerEngineAddresses|TestDesktopInfraHostProjectionUsesProxmoxEdgesDockerMappingAndInference)$' -count=1 -v`
Expected: FAIL (build error: `undefined: desktopInfraDockerEngineAddresses`).

- [ ] **Step 4: Implement**

In `internal/infra/types.go`, replace the `Node` struct with:

```go
type Node struct {
	ID                  string                       `json:"id"`
	Version             int64                        `json:"version"`
	Kind                NodeKind                     `json:"kind"`
	Name                string                       `json:"name"`
	Icon                string                       `json:"icon"`
	RegistryID          string                       `json:"registry_id,omitempty"`
	ParentID            string                       `json:"parent_id,omitempty"`
	Profile             Profile                      `json:"profile"`
	Placements          Placements                   `json:"placements"`
	Sources             []Source                     `json:"sources"`
	AvailableActions    []string                     `json:"available_actions"`
	AppLinks            []AppLink                    `json:"app_links"`
	Power               string                       `json:"power,omitempty"`
	ActionReasons       map[string]string            `json:"action_reasons,omitempty"`
	SourceActions       map[string][]string          `json:"source_actions,omitempty"`
	SourceActionReasons map[string]map[string]string `json:"source_action_reasons,omitempty"`
	// HostID and HostInferred are runtime projections for VMs and containers:
	// the device running the guest, and whether that host was inferred.
	HostID       string `json:"host_id,omitempty"`
	HostInferred bool   `json:"host_inferred,omitempty"`
}
```

In `internal/infra/validate.go`, replace:

```go
func normalizeNodeRuntime(node *Node) {
	node.AvailableActions = []string{}
	node.AppLinks = []AppLink{}
	node.Power = ""
	node.ActionReasons = nil
	node.SourceActions = nil
	node.SourceActionReasons = nil
}
```

with:

```go
func normalizeNodeRuntime(node *Node) {
	node.AvailableActions = []string{}
	node.AppLinks = []AppLink{}
	node.Power = ""
	node.ActionReasons = nil
	node.SourceActions = nil
	node.SourceActionReasons = nil
	node.HostID = ""
	node.HostInferred = false
}
```

Replace the whole content of `internal/server/desktop_infra_hosts.go` with:

```go
package server

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"aurago/internal/config"
	"aurago/internal/infra"
)

type desktopInfraSourceHostRequest struct {
	Kind     string `json:"kind"`
	Instance string `json:"instance"`
	NodeID   string `json:"node_id"`
	Version  int64  `json:"version"`
}

// desktopInfraSetSourceHost assigns or clears the device that runs a Docker
// engine. Inferred hosts are runtime projections reported by the snapshot.
func desktopInfraSetSourceHost(w http.ResponseWriter, r *http.Request, store *infra.Store) {
	var request desktopInfraSourceHostRequest
	if !desktopInfraDecode(w, r, &request) {
		return
	}
	if request.NodeID != "" && !desktopInfraValidPathID(request.NodeID) {
		desktopInfraError(w, http.StatusBadRequest, "invalid_node_id", "Node ID is invalid")
		return
	}
	host, err := store.SetSourceHost(r.Context(), infra.SourceHost{
		Kind: request.Kind, Instance: request.Instance, NodeID: request.NodeID, Version: request.Version,
	})
	if err != nil {
		desktopInfraStoreError(w, err)
		return
	}
	desktopInfraJSON(w, http.StatusOK, map[string]any{"source_host": host})
}

// desktopInfraLocalAddresses lists the AuraGo server's own interface addresses.
// A local Docker socket or pipe runs on the machine with these addresses.
// Tests replace it.
var desktopInfraLocalAddresses = func() []netip.Addr {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	result := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		prefix, err := netip.ParsePrefix(address.String())
		if err != nil {
			continue
		}
		result = append(result, prefix.Addr().Unmap())
	}
	return result
}

// desktopInfraDockerEngineAddresses returns the addresses that identify the
// machine running the configured Docker engine. A local socket or pipe, an
// empty host, localhost or a loopback IP means this server; a TCP endpoint with
// an IP literal names that machine. Host names are never resolved.
func desktopInfraDockerEngineAddresses(host string, local []netip.Addr) []netip.Addr {
	host = strings.TrimSpace(host)
	if host == "" || strings.HasPrefix(host, "unix://") || strings.HasPrefix(host, "npipe://") {
		return desktopInfraUsableAddresses(local)
	}
	parsed, err := url.Parse(host)
	if err != nil || (parsed.Scheme != "tcp" && parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil
	}
	name := parsed.Hostname()
	if strings.EqualFold(name, "localhost") {
		return desktopInfraUsableAddresses(local)
	}
	addr, err := netip.ParseAddr(name)
	if err != nil {
		return nil
	}
	addr = addr.Unmap()
	if addr.IsLoopback() {
		return desktopInfraUsableAddresses(local)
	}
	return []netip.Addr{addr}
}

func desktopInfraUsableAddresses(addresses []netip.Addr) []netip.Addr {
	result := make([]netip.Addr, 0, len(addresses))
	for _, addr := range addresses {
		if !addr.IsValid() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsUnspecified() {
			continue
		}
		result = append(result, addr.Unmap())
	}
	return result
}

// desktopInfraNodeWithAddress returns the only device whose profile or
// interface address is one of addresses. Ambiguity infers nothing.
func desktopInfraNodeWithAddress(nodes []infra.Node, addresses []netip.Addr) string {
	if len(addresses) == 0 {
		return ""
	}
	wanted := make(map[netip.Addr]struct{}, len(addresses))
	for _, addr := range addresses {
		wanted[addr.Unmap()] = struct{}{}
	}
	match := ""
	for _, node := range nodes {
		if node.Kind != infra.NodeDevice || !desktopInfraNodeHasAddress(node, wanted) {
			continue
		}
		if match != "" {
			return ""
		}
		match = node.ID
	}
	return match
}

func desktopInfraNodeHasAddress(node infra.Node, wanted map[netip.Addr]struct{}) bool {
	values := make([]string, 0, len(node.Profile.Interfaces)+1)
	values = append(values, node.Profile.IPAddress)
	for _, iface := range node.Profile.Interfaces {
		values = append(values, iface.IPAddress)
	}
	for _, value := range values {
		addr, err := netip.ParseAddr(strings.TrimSpace(value))
		if err != nil {
			continue
		}
		if _, ok := wanted[addr.Unmap()]; ok {
			return true
		}
	}
	return false
}

// desktopInfraProjectHosts sets the runtime-only host of every guest (VM or
// container) among the snapshot's nodes, and the inferred host of each Docker
// engine entry. Proxmox guests use their provider runs_on edge; Docker guests
// use the operator's engine mapping, else an inferred device for the configured
// engine (host_inferred). Only nodes present in the snapshot can be hosts.
func desktopInfraProjectHosts(cfg *config.Config, snapshot *infra.Snapshot, local []netip.Addr) {
	if snapshot == nil {
		return
	}
	visible := make(map[string]bool, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		visible[node.ID] = true
	}
	engine, inferred := "", ""
	if cfg != nil && cfg.Docker.Enabled {
		engine = desktopInfraDockerInstance(cfg)
		inferred = desktopInfraNodeWithAddress(snapshot.Nodes, desktopInfraDockerEngineAddresses(cfg.Docker.Host, local))
	}
	mapped := make(map[string]string, len(snapshot.SourceHosts))
	for i := range snapshot.SourceHosts {
		host := &snapshot.SourceHosts[i]
		host.InferredNodeID = ""
		if host.Kind != "docker" {
			continue
		}
		if host.Instance == engine {
			host.InferredNodeID = inferred
		}
		if host.NodeID != "" && visible[host.NodeID] {
			mapped[host.Instance] = host.NodeID
		}
	}
	runsOn := make(map[string]string)
	for _, edge := range snapshot.Edges {
		if edge.Kind == infra.EdgeRunsOn && edge.Source != nil && edge.Source.Kind == "proxmox" && visible[edge.To] {
			runsOn[edge.From] = edge.To
		}
	}
	for i := range snapshot.Nodes {
		node := &snapshot.Nodes[i]
		node.HostID, node.HostInferred = "", false
		if node.Kind != infra.NodeVM && node.Kind != infra.NodeContainer {
			continue
		}
		if host, ok := runsOn[node.ID]; ok {
			node.HostID = host
			continue
		}
		for _, source := range node.Sources {
			if source.Kind != "docker" {
				continue
			}
			if host := mapped[source.Instance]; host != "" {
				node.HostID = host
			} else if source.Instance == engine && inferred != "" {
				node.HostID, node.HostInferred = inferred, true
			}
			break
		}
	}
}

// desktopInfraHostedSnapshot reads the stored graph with the snapshot's
// visibility rule and host projection, without app-link or action projection.
func desktopInfraHostedSnapshot(ctx context.Context, store *infra.Store, cfg *config.Config) (infra.Snapshot, error) {
	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		return infra.Snapshot{}, err
	}
	snapshot.Nodes = desktopInfraVisibleNodes(&snapshot)
	desktopInfraProjectHosts(cfg, &snapshot, desktopInfraLocalAddresses())
	return snapshot, nil
}

// desktopInfraApplyHosts projects host_id/host_inferred onto nodes returned by
// a write. It is best effort: without a readable graph the fields stay empty and
// the next snapshot carries them.
func desktopInfraApplyHosts(ctx context.Context, store *infra.Store, cfg *config.Config, nodes []infra.Node) []infra.Node {
	if store == nil || len(nodes) == 0 {
		return nodes
	}
	snapshot, err := desktopInfraHostedSnapshot(ctx, store, cfg)
	if err != nil {
		return nodes
	}
	projected := make(map[string]infra.Node, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		projected[node.ID] = node
	}
	for i := range nodes {
		host := projected[nodes[i].ID]
		nodes[i].HostID, nodes[i].HostInferred = host.HostID, host.HostInferred
	}
	return nodes
}
```

In `internal/server/desktop_infra_projection.go`, replace the whole `func desktopInfraDecorateSnapshot(...) { ... }` with:

```go
func desktopInfraDecorateSnapshot(ctx context.Context, s *Server, cfg *config.Config, snapshot *infra.Snapshot) {
	if snapshot == nil {
		return
	}
	snapshot.Sources = desktopInfraProjectSourceStates(cfg, snapshot.Sources)
	snapshot.Nodes = desktopInfraProjectNodes(ctx, s, cfg, desktopInfraVisibleNodes(snapshot))
	if snapshot.Nodes == nil {
		snapshot.Nodes = []infra.Node{}
	}
	if snapshot.SourceHosts == nil {
		snapshot.SourceHosts = []infra.SourceHost{}
	}
	desktopInfraProjectHosts(cfg, snapshot, desktopInfraLocalAddresses())
}
```

In `internal/server/desktop_infra_candidates.go`, in `desktopInfraVisibleNodes`, replace:

```go
	for _, node := range snapshot.Nodes {
		referenced[node.ParentID] = true
	}
```

with:

```go
	for _, node := range snapshot.Nodes {
		referenced[node.ParentID] = true
	}
	// An explicit Docker engine mapping is a deliberate use, like an edge.
	for _, host := range snapshot.SourceHosts {
		referenced[host.NodeID] = true
	}
```

In `internal/server/desktop_infra.go`, in the PUT branch of `desktopInfraNodeRoute`, replace:

```go
		updated = desktopInfraDecorateNode(r.Context(), s, cfg, updated)
		desktopInfraJSON(w, http.StatusOK, updated)
```

with:

```go
		updated = desktopInfraDecorateNode(r.Context(), s, cfg, updated)
		updated = desktopInfraApplyHosts(r.Context(), store, cfg, []infra.Node{updated})[0]
		desktopInfraJSON(w, http.StatusOK, updated)
```

In `desktopInfraImport`, replace:

```go
	node = desktopInfraDecorateNode(r.Context(), s, cfg, node)
	desktopInfraJSON(w, http.StatusCreated, node)
```

with:

```go
	node = desktopInfraDecorateNode(r.Context(), s, cfg, node)
	node = desktopInfraApplyHosts(r.Context(), store, cfg, []infra.Node{node})[0]
	desktopInfraJSON(w, http.StatusCreated, node)
```

In `desktopInfraSaveLayout`, replace:

```go
	updated = desktopInfraProjectNodes(r.Context(), s, cfg, updated)
	desktopInfraJSON(w, http.StatusOK, map[string]any{"nodes": updated})
```

with:

```go
	updated = desktopInfraProjectNodes(r.Context(), s, cfg, updated)
	updated = desktopInfraApplyHosts(r.Context(), store, cfg, updated)
	desktopInfraJSON(w, http.StatusOK, map[string]any{"nodes": updated})
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `gofmt -w internal/infra/types.go internal/infra/validate.go internal/infra/store_test.go internal/server/desktop_infra_hosts.go internal/server/desktop_infra_projection.go internal/server/desktop_infra_candidates.go internal/server/desktop_infra.go internal/server/desktop_infra_hosts_test.go`
Run: `go test ./internal/infra/ -run '^(TestHostProjectionFieldsAreRuntimeOnly|TestNodeRuntimeProjectionFieldsAreNotTrustedOrPersisted)$' -count=1 -v`
Expected: PASS (2 tests).
Run: `go test ./internal/server/ -run 'Infra' -count=1`
Expected: `ok  	aurago/internal/server`

- [ ] **Step 6: Commit**

```bash
git add internal/infra/types.go internal/infra/validate.go internal/infra/store_test.go internal/server/desktop_infra_hosts.go internal/server/desktop_infra_projection.go internal/server/desktop_infra_candidates.go internal/server/desktop_infra.go internal/server/desktop_infra_hosts_test.go
git commit -m "feat(infra): project runtime-only guest hosts from Proxmox, Docker mapping and inference" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: Batch import with parent and placement

**Files:**
- Modify: `internal/infra/types.go` (append `ViewPlacement`, `ImportLayout`)
- Modify: `internal/infra/sources.go:235-324` (`LinkSource` → `LinkSource` + `LinkSourceWithLayout`)
- Create: `internal/server/desktop_infra_batch.go`
- Create: `internal/server/desktop_infra_batch_test.go`

- [ ] **Step 1: Impact analysis**

```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "LinkSource" --direction upstream --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
graft callers LinkSource
git grep -nE "LinkSource\(" -- internal/infra 'internal/server/desktop_infra*'
```

Expected: `LinkSource` ← `desktopInfraImport` (`desktop_infra.go:568`) and `store_test.go:422,453,478,498,776,791`. Risk MEDIUM: registry link safety. The registry block is copied unchanged, and the existing registry tests guard it.

- [ ] **Step 2: Write the failing test**

Create `internal/server/desktop_infra_batch_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/infra"
	"aurago/internal/tools"
)

func TestDesktopInfraBatchImportRefreshesOnceAndReportsPartialFailures(t *testing.T) {
	s, cfg, store, runtime := newDesktopInfraRefreshFixture(t)
	ctx := context.Background()
	instance := desktopInfraDockerInstance(cfg)
	ids := []string{strings.Repeat("d1", 32), strings.Repeat("d2", 32), strings.Repeat("d3", 32)}
	names := []string{"radarr", "sonarr", "plex"}
	listCalls := 0
	runtime.dockerRequest = func(_ context.Context, _ tools.DockerConfig, method, endpoint, _ string) ([]byte, int, error) {
		if method != http.MethodGet || endpoint != "/containers/json?all=true" {
			t.Errorf("unexpected Docker request %s %s", method, endpoint)
			return nil, http.StatusNotFound, nil
		}
		listCalls++
		rows := make([]map[string]any, 0, len(ids))
		for i, id := range ids {
			rows = append(rows, map[string]any{"Id": id, "Names": []string{"/" + names[i]}, "Image": "lscr.io/" + names[i], "State": "running", "Labels": map[string]string{}})
		}
		body, err := json.Marshal(rows)
		return body, http.StatusOK, err
	}
	desktopInfraRefreshDocker(ctx, cfg, store, runtime, true)
	listCalls = 0
	room, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeRoom, Name: "Living room", Icon: "room"})
	if err != nil {
		t.Fatal(err)
	}
	notAPlace, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeDevice, Name: "Switch", Icon: "switch"})
	if err != nil {
		t.Fatal(err)
	}
	manual, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeContainer, Name: "Plex (manual)", Icon: "container"})
	if err != nil {
		t.Fatal(err)
	}
	candidateID := func(i int) string {
		return infra.CandidateID(infra.SourceIdentity{Kind: "docker", Instance: instance, Key: ids[i]})
	}
	batch := func(body string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		desktopInfraImportBatch(response, httptest.NewRequest(http.MethodPost, desktopInfraBasePath+"/import/batch", strings.NewReader(body)), s, cfg, store, runtime)
		return response
	}
	response := batch(`{"items":[` +
		`{"candidate_id":"` + candidateID(0) + `","parent_id":"` + room.ID + `","placement":{"view":"atlas","x":48,"y":72}},` +
		`{"candidate_id":"` + candidateID(1) + `","parent_id":"` + notAPlace.ID + `"},` +
		`{"candidate_id":"` + candidateID(2) + `","node_id":"` + manual.ID + `","parent_id":"` + room.ID + `","placement":{"view":"topology","x":10,"y":20}},` +
		`{"candidate_id":"infra_bad"}]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("batch import = %d %s", response.Code, response.Body.String())
	}
	var payload struct {
		Nodes  []infra.Node              `json:"nodes"`
		Errors []desktopInfraBatchError `json:"errors"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if listCalls != 1 {
		t.Fatalf("Docker list calls = %d; want one forced refresh for the whole batch", listCalls)
	}
	if len(payload.Nodes) != 2 {
		t.Fatalf("imported nodes = %#v; want two", payload.Nodes)
	}
	created, linked := payload.Nodes[0], payload.Nodes[1]
	if created.Name != "radarr" || created.ParentID != room.ID || created.Placements[infra.ViewAtlas].X != 48 || created.Placements[infra.ViewAtlas].Y != 72 {
		t.Fatalf("created node = %#v", created)
	}
	if linked.ID != manual.ID || linked.ParentID != room.ID || linked.Placements[infra.ViewTopology].Y != 20 || len(linked.Sources) != 1 {
		t.Fatalf("linked node = %#v", linked)
	}
	codes := make(map[string]string, len(payload.Errors))
	for _, failure := range payload.Errors {
		codes[failure.CandidateID] = failure.Error
	}
	if len(codes) != 2 || codes[candidateID(1)] != "invalid_request" || codes["infra_bad"] != "invalid_request" {
		t.Fatalf("batch errors = %#v", payload.Errors)
	}
	if _, err := store.GetCandidate(ctx, candidateID(1)); err != nil {
		t.Fatalf("a failed item consumed its candidate: %v", err)
	}
	for _, body := range []string{
		`{"items":[]}`,
		`{"items":[{"candidate_id":"` + candidateID(1) + `"},{"candidate_id":"` + candidateID(1) + `"}]}`,
	} {
		if rejected := batch(body); rejected.Code != http.StatusBadRequest {
			t.Fatalf("batch %s = %d %s; want 400", body, rejected.Code, rejected.Body.String())
		}
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/server/ -run '^TestDesktopInfraBatchImportRefreshesOnceAndReportsPartialFailures$' -count=1 -v`
Expected: FAIL (build error: `undefined: desktopInfraImportBatch`).

- [ ] **Step 4: Implement**

Append to `internal/infra/types.go`:

```go
// ViewPlacement positions a node in one view (batch import and link).
type ViewPlacement struct {
	View   View    `json:"view"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	W      float64 `json:"w,omitempty"`
	H      float64 `json:"h,omitempty"`
	Pinned bool    `json:"pinned,omitempty"`
}

// AsPlacement returns the stored form of the position.
func (p ViewPlacement) AsPlacement() Placement {
	return Placement{X: p.X, Y: p.Y, W: p.W, H: p.H, Pinned: p.Pinned}
}

// ImportLayout optionally places a node while a source is linked to it.
// ParentID nil keeps the parent and "" clears it.
type ImportLayout struct {
	ParentID  *string
	Placement *ViewPlacement
}
```

In `internal/infra/sources.go`, replace the whole `func (s *Store) LinkSource(ctx context.Context, nodeID string, identity SourceIdentity) (Node, error) { ... }` with:

```go
// LinkSource links an observed source to an existing node.
func (s *Store) LinkSource(ctx context.Context, nodeID string, identity SourceIdentity) (Node, error) {
	return s.LinkSourceWithLayout(ctx, nodeID, identity, ImportLayout{})
}

// LinkSourceWithLayout links a source and, in the same transaction and version
// step, applies an optional parent and view placement to the node.
func (s *Store) LinkSourceWithLayout(ctx context.Context, nodeID string, identity SourceIdentity, layout ImportLayout) (Node, error) {
	if err := validateIdentity(identity); err != nil {
		return Node{}, err
	}
	if err := validateText("node id", nodeID, 128, true); err != nil {
		return Node{}, err
	}
	if layout.Placement != nil && layout.Placement.View != ViewAtlas && layout.Placement.View != ViewTopology {
		return Node{}, fmt.Errorf("%w: unsupported placement view", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin infra source link: %w", err)
	}
	defer tx.Rollback()
	source, err := loadSource(ctx, tx, identity)
	if err != nil {
		return Node{}, err
	}
	if source.State == SourceRemoved {
		return Node{}, ErrSourceRemoved
	}
	node, err := loadNode(ctx, tx, nodeID)
	if err != nil {
		return Node{}, err
	}
	var linkedID sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT node_id FROM infra_sources WHERE kind=? AND instance=? AND source_key=?`, identity.Kind, identity.Instance, identity.Key).Scan(&linkedID); err != nil {
		return Node{}, fmt.Errorf("check infra source link: %w", err)
	}
	if linkedID.Valid {
		if linkedID.String != nodeID {
			return Node{}, ErrSourceLinked
		}
		if err := tx.Commit(); err != nil {
			return Node{}, fmt.Errorf("commit existing infra source link: %w", err)
		}
		return s.GetNode(ctx, nodeID)
	}
	if identity.Kind == "registry" {
		if !registryLinkableKind(node.Kind) {
			return Node{}, fmt.Errorf("%w: registry sources can link only to devices, VMs, or containers", ErrValidation)
		}
		if node.RegistryID != "" && node.RegistryID != identity.Key {
			return Node{}, fmt.Errorf("%w: node already has a different registry device", ErrValidation)
		}
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id=?)`, identity.Key).Scan(&exists); err != nil {
			return Node{}, fmt.Errorf("check registry device: %w", err)
		}
		if !exists {
			return Node{}, ErrSourceNotFound
		}
		// The unique registry index owns the one-node-per-device rule. Report a
		// claim by another node as a link conflict instead of a raw SQL error.
		var claimedBy string
		switch err := tx.QueryRowContext(ctx, `SELECT id FROM infra_nodes WHERE registry_id=?`, identity.Key).Scan(&claimedBy); {
		case errors.Is(err, sql.ErrNoRows):
		case err != nil:
			return Node{}, fmt.Errorf("check registry link ownership: %w", err)
		case claimedBy != nodeID:
			return Node{}, ErrSourceLinked
		}
		node.RegistryID = identity.Key
	}
	if layout.ParentID != nil {
		parentID := strings.TrimSpace(*layout.ParentID)
		if err := validateParent(ctx, tx, node.ID, parentID); err != nil {
			return Node{}, err
		}
		node.ParentID = parentID
	}
	if layout.Placement != nil {
		placement := layout.Placement.AsPlacement()
		if err := validatePlacement(node.Kind, placement); err != nil {
			return Node{}, err
		}
		node.Placements[layout.Placement.View] = placement
	}
	placementsJSON, err := json.Marshal(node.Placements)
	if err != nil {
		return Node{}, fmt.Errorf("encode infra node placements: %w", err)
	}
	previous := node.Version
	node.Version++
	res, err := tx.ExecContext(ctx, `UPDATE infra_nodes SET registry_id=?, parent_id=?, placements_json=?, version=?, updated_at=? WHERE id=? AND version=?`,
		node.RegistryID, nullIfEmpty(node.ParentID), string(placementsJSON), node.Version, formatTime(time.Now().UTC()), node.ID, previous)
	if err != nil {
		return Node{}, fmt.Errorf("advance infra node version for source link: %w", err)
	}
	if changed, err := res.RowsAffected(); err != nil || changed != 1 {
		if err != nil {
			return Node{}, fmt.Errorf("count linked infra source nodes: %w", err)
		}
		return Node{}, ErrConflict
	}
	res, err = tx.ExecContext(ctx, `UPDATE infra_sources SET node_id=?, ignored_at='' WHERE kind=? AND instance=? AND source_key=? AND node_id IS NULL`, nodeID, identity.Kind, identity.Instance, identity.Key)
	if err != nil {
		return Node{}, fmt.Errorf("set infra source link: %w", err)
	}
	if changed, err := res.RowsAffected(); err != nil || changed != 1 {
		if err != nil {
			return Node{}, fmt.Errorf("count linked infra sources: %w", err)
		}
		return Node{}, ErrSourceLinked
	}
	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit infra source link: %w", err)
	}
	return s.GetNode(ctx, nodeID)
}
```

Create `internal/server/desktop_infra_batch.go`:

```go
package server

import (
	"context"
	"net/http"
	"strings"

	"aurago/internal/config"
	"aurago/internal/infra"
)

// desktopInfraBatchLimit bounds one batch import.
const desktopInfraBatchLimit = 500

type desktopInfraBatchItem struct {
	CandidateID string               `json:"candidate_id"`
	NodeID      string               `json:"node_id,omitempty"`
	ParentID    *string              `json:"parent_id,omitempty"`
	Placement   *infra.ViewPlacement `json:"placement,omitempty"`
}

type desktopInfraBatchError struct {
	CandidateID string `json:"candidate_id"`
	Error       string `json:"error"`
	Message     string `json:"message"`
}

// desktopInfraImportBatch imports or links 1 to 500 candidates. Every provider
// behind the batch is refreshed once (forced) before any write; each item then
// runs in its own store transaction, so one failure never undoes another item.
func desktopInfraImportBatch(w http.ResponseWriter, r *http.Request, s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime) {
	var request struct {
		Items []desktopInfraBatchItem `json:"items"`
	}
	if !desktopInfraDecode(w, r, &request) {
		return
	}
	if len(request.Items) == 0 || len(request.Items) > desktopInfraBatchLimit {
		desktopInfraError(w, http.StatusBadRequest, "invalid_request", "A batch needs 1 to 500 items")
		return
	}
	seen := make(map[string]struct{}, len(request.Items))
	for _, item := range request.Items {
		if strings.TrimSpace(item.CandidateID) == "" {
			desktopInfraError(w, http.StatusBadRequest, "invalid_candidate_id", "Every item needs a candidate ID")
			return
		}
		if _, duplicate := seen[item.CandidateID]; duplicate {
			desktopInfraError(w, http.StatusBadRequest, "invalid_request", "A candidate appears more than once")
			return
		}
		seen[item.CandidateID] = struct{}{}
	}
	failures := make([]desktopInfraBatchError, 0)
	pending := make([]desktopInfraBatchItem, 0, len(request.Items))
	providers := make(map[string]infra.SourceIdentity)
	for _, item := range request.Items {
		candidate, err := store.GetCandidate(r.Context(), item.CandidateID)
		if err != nil {
			failures = append(failures, desktopInfraBatchStoreError(item, err))
			continue
		}
		if !desktopInfraSourceConfigured(cfg, candidate.Source) {
			failures = append(failures, desktopInfraBatchError{CandidateID: item.CandidateID, Error: "source_unavailable", Message: "This source is no longer configured"})
			continue
		}
		providers[candidate.Source.Kind+"\x00"+candidate.Source.Instance] = candidate.Source.SourceIdentity
		pending = append(pending, item)
	}
	for _, identity := range providers {
		desktopInfraRefreshSource(r.Context(), s, cfg, store, runtime, identity)
	}
	nodes := make([]infra.Node, 0, len(pending))
	proxmoxInstances := make(map[string]struct{})
	for _, item := range pending {
		node, identity, failure := desktopInfraImportOne(r.Context(), store, item)
		if failure != nil {
			failures = append(failures, *failure)
			continue
		}
		if identity.Kind == "proxmox" {
			proxmoxInstances[identity.Instance] = struct{}{}
		}
		nodes = append(nodes, node)
	}
	for instance := range proxmoxInstances {
		// The graph writes already succeeded; placement edges catch up on the next
		// refresh if this best-effort reconcile fails.
		_ = desktopInfraReconcileProxmoxEdges(r.Context(), store, instance)
	}
	nodes = desktopInfraApplyHosts(r.Context(), store, cfg, desktopInfraProjectNodes(r.Context(), s, cfg, nodes))
	desktopInfraJSON(w, http.StatusOK, map[string]any{"nodes": nodes, "errors": failures})
}

// desktopInfraImportOne re-reads a candidate after the batch refresh and imports
// it (or links it to item.NodeID) with its optional parent and placement.
func desktopInfraImportOne(ctx context.Context, store *infra.Store, item desktopInfraBatchItem) (infra.Node, infra.SourceIdentity, *desktopInfraBatchError) {
	candidate, err := store.GetCandidate(ctx, item.CandidateID)
	if err != nil {
		failure := desktopInfraBatchStoreError(item, err)
		return infra.Node{}, infra.SourceIdentity{}, &failure
	}
	identity := candidate.Source.SourceIdentity
	if candidate.Source.State != infra.SourceCurrent {
		return infra.Node{}, identity, &desktopInfraBatchError{CandidateID: item.CandidateID, Error: "source_stale", Message: "The source could not be freshly verified"}
	}
	var node infra.Node
	if item.NodeID != "" {
		if !desktopInfraValidPathID(item.NodeID) {
			return infra.Node{}, identity, &desktopInfraBatchError{CandidateID: item.CandidateID, Error: "invalid_node_id", Message: "Node ID is invalid"}
		}
		node, err = store.LinkSourceWithLayout(ctx, item.NodeID, identity, infra.ImportLayout{ParentID: item.ParentID, Placement: item.Placement})
	} else {
		node = candidate.Node
		node.ID, node.Version, node.RegistryID, node.Sources = "", 0, "", []infra.Source{}
		if item.ParentID != nil {
			node.ParentID = strings.TrimSpace(*item.ParentID)
		}
		if item.Placement != nil {
			node.Placements = infra.Placements{item.Placement.View: item.Placement.AsPlacement()}
		}
		node, err = store.CreateImportedNode(ctx, node, identity)
	}
	if err != nil {
		failure := desktopInfraBatchStoreError(item, err)
		return infra.Node{}, identity, &failure
	}
	return node, identity, nil
}

func desktopInfraBatchStoreError(item desktopInfraBatchItem, err error) desktopInfraBatchError {
	_, code, message := desktopInfraStoreErrorStatus(err)
	return desktopInfraBatchError{CandidateID: item.CandidateID, Error: code, Message: message}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `gofmt -w internal/infra/types.go internal/infra/sources.go internal/server/desktop_infra_batch.go internal/server/desktop_infra_batch_test.go`
Run: `go test ./internal/server/ -run '^TestDesktopInfraBatchImportRefreshesOnceAndReportsPartialFailures$' -count=1 -v`
Expected: PASS.
Run: `go test ./internal/infra/ -count=1`
Expected: `ok  	aurago/internal/infra` (registry link tests still pass).
Run: `go test ./internal/server/ -run 'Infra' -count=1`
Expected: `ok  	aurago/internal/server`

- [ ] **Step 6: Commit**

```bash
git add internal/infra/types.go internal/infra/sources.go internal/server/desktop_infra_batch.go internal/server/desktop_infra_batch_test.go
git commit -m "feat(infra): add batch import with one refresh and per-item placement" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: Register the new routes; admin and read-only refusal

**Files:**
- Modify: `internal/server/desktop_infra.go:82-85` (`handleDesktopInfra` switch)
- Modify: `internal/server/desktop_infra_routes_test.go:3-17` (imports), append one test

- [ ] **Step 1: Impact analysis**

```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "handleDesktopInfra" --direction upstream --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
graft callers handleDesktopInfra
git grep -nE "(handleDesktopInfra|registerDesktopInfraRoutes)\(" -- internal/server
```

Expected: `handleDesktopInfra` ← `registerDesktopInfraRoutes` (`desktop_infra.go:50`). `registerDesktopInfraRoutes` ← server route setup and tests (`desktop_infra_test.go:39`, `desktop_infra_local_update_test.go:80`). GitNexus may rate the route hub HIGH or CRITICAL; record the warning. The change adds three switch cases and does not touch existing ones.

- [ ] **Step 2: Write the failing test**

In `internal/server/desktop_infra_routes_test.go`, replace the import block with:

```go
import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/infra"
	"aurago/internal/inventory"
	"aurago/internal/tools"
)
```

and append:

```go
func TestDesktopInfraNewWriteRoutesAreRoutedAndHonourReadOnly(t *testing.T) {
	s, _, writeToken := testDesktopPermissionServer(t)
	adminToken, _, err := s.TokenManager.Create("infra admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s.InventoryDB = db
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.VirtualDesktop.Infra.Enabled = true
	mux := http.NewServeMux()
	registerDesktopInfraRoutes(mux, s)
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		return response
	}
	type route struct {
		method, path, body string
		status             int
	}
	routes := []route{
		{http.MethodPut, desktopInfraBasePath + "/sources/ignore", `{}`, http.StatusBadRequest},
		{http.MethodPut, desktopInfraBasePath + "/source-hosts", `{}`, http.StatusBadRequest},
		{http.MethodPost, desktopInfraBasePath + "/import/batch", `{}`, http.StatusBadRequest},
		{http.MethodDelete, desktopInfraBasePath + "/nodes/missing?version=1&ignore_sources=1", "", http.StatusNotFound},
	}
	for _, tc := range routes {
		if response := call(tc.method, tc.path, writeToken, tc.body); response.Code != http.StatusForbidden && response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s without admin scope = %d %s", tc.method, tc.path, response.Code, response.Body.String())
		}
		// An admin reaches the handler's own validation, so the route exists.
		if response := call(tc.method, tc.path, adminToken, tc.body); response.Code != tc.status {
			t.Fatalf("%s %s = %d %s; want %d", tc.method, tc.path, response.Code, response.Body.String(), tc.status)
		}
	}
	s.Cfg.VirtualDesktop.Infra.ReadOnly = true
	for _, tc := range routes {
		response := call(tc.method, tc.path, adminToken, tc.body)
		if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "infra_readonly") {
			t.Fatalf("%s %s on read-only Infra = %d %s; want 403 infra_readonly", tc.method, tc.path, response.Code, response.Body.String())
		}
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/server/ -run '^TestDesktopInfraNewWriteRoutesAreRoutedAndHonourReadOnly$' -count=1 -v`
Expected: FAIL with `PUT /api/desktop/infra/sources/ignore = 405 {"error":"method_not_allowed",...}; want 400`.

- [ ] **Step 4: Implement**

In `internal/server/desktop_infra.go`, in `handleDesktopInfra`, replace:

```go
		case path == "import" && r.Method == http.MethodPost:
			desktopInfraImport(w, r, s, cfg, store, runtime)
```

with:

```go
		case path == "import" && r.Method == http.MethodPost:
			desktopInfraImport(w, r, s, cfg, store, runtime)
		case path == "import/batch" && r.Method == http.MethodPost:
			desktopInfraImportBatch(w, r, s, cfg, store, runtime)
		case path == "sources/ignore" && r.Method == http.MethodPut:
			desktopInfraIgnoreSources(w, r, cfg, store)
		case path == "source-hosts" && r.Method == http.MethodPut:
			desktopInfraSetSourceHost(w, r, store)
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `gofmt -w internal/server/desktop_infra.go internal/server/desktop_infra_routes_test.go`
Run: `go test ./internal/server/ -run '^(TestDesktopInfraNewWriteRoutesAreRoutedAndHonourReadOnly|TestDesktopInfraRouteRequiresAdminAndAppliesInfraReadOnly|TestDesktopInfraLocalRegistryEditDoesNotWriteRegistryWithoutSourceGrant)$' -count=1 -v`
Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/server/desktop_infra.go internal/server/desktop_infra_routes_test.go
git commit -m "feat(infra): route ignore, source-host and batch import endpoints" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: Contract and API documentation

**Files:**
- Modify: `internal/infra/AGENTS.md` (whole file)
- Modify: `documentation/infra.md:141-164` (API overview section up to and including the error code paragraph)

- [ ] **Step 1: Check that the contract file is not assume-unchanged**

Run: `git ls-files -v internal/infra/AGENTS.md documentation/infra.md`
Expected: both lines start with `H `. If one starts with a lowercase `h`, run `git update-index --no-assume-unchanged <path>` before editing.

- [ ] **Step 2: Rewrite `internal/infra/AGENTS.md`**

Replace the whole content of `internal/infra/AGENTS.md` with:

```markdown
# Infra Store Contract

`internal/infra` owns the persistent editable infrastructure graph and its safe, typed observations of configured source adapters. It shares the inventory `*sql.DB`; the store never closes it or changes SQLite `user_version`.

- Keep nodes, explicit source links, edges, source freshness, ignore decisions, engine host mappings, layout revisions, and action request outcomes in this package. Never persist raw provider payloads, credentials, Fritz!Box MAC identities, or arbitrary provider JSON.
- Schema changes are additive and backward compatible. Back up the existing main database before the first schema migration with the SQLite backup pattern, and test migration against existing inventory rows. Do not alter inventory schema or execution credentials here. Keep the newest three migration backups next to the database and prune older copies. The migration chain is v1 → v2 → v3 with one explicit, idempotent step per version (v3 adds `infra_sources.ignored_at` and `infra_source_hosts`); the full-create path writes the current version, and every step is tested from a populated older schema.
- `Store.New` runs during server startup, not inside a request: the migration and its full database copy must not sit on the HTTP path. Callers cache an initialization failure instead of repeating the migration per request.
- Node, source-link, edge, and layout mutations use persisted version checks. Layout batches are atomic; an item may also set `parent_id` (absent keeps the parent, `""` clears it; the container and cycle rules of node updates apply, also across items of one batch) and the size `w`/`h` of a place (sites, rooms, racks and groups only, 120–20000; an absent size keeps the stored one). A place that becomes another kind loses its size. Parent hierarchy cycles are invalid; network-edge cycles are allowed. Manual edges reject self-loops (`ErrValidation`) and a second edge with the same `(from, to, kind)` (`ErrEdgeExists`). Provider-owned edges are read-only. A node that still has children may not become a kind that cannot contain nodes (`ErrHasChildren`).
- Manual device nodes atomically create protocol-`none` inventory rows. Explicit registry links may associate existing inventory devices with Infra devices, VMs, or containers; they never create synthetic inventory rows for VMs or containers. A registry device may be claimed by one node only; a second claim is `ErrSourceLinked`, never a raw unique-index error. Local Infra node updates never mutate existing Registry data; only the explicit, permission-gated source-edit handler writes Registry fields. Editing or deleting an Infra node must preserve operator-configured inventory protocol and credentials, and deleting Infra never deletes inventory devices.
- Source identities are `(kind, instance, key)`. Linking is always explicit. Incomplete observations stale unseen records; only complete enumerations remove them, and an empty complete enumeration is a valid removal signal for a provider without resources. Startup marks current observations stale. Linked last-known source data and nodes persist through outages and source removal. Only unlinked removed candidate observations may expire after the declared retention window; they stay out of `Snapshot`, which keeps linked removed sources visible.
- Ignored sources (`ignored_at`) are never linked: ignoring a linked source is `ErrSourceLinked`, and importing or linking a source clears its ignore flag. Observations never change `ignored_at`, retention never prunes ignored rows, and `ListSources`, `Snapshot` and candidates leave ignored sources out unless asked (`ListSourcesIncludingIgnored`, `SnapshotOptions.IncludeIgnored`). `SetSourcesIgnored` is atomic and idempotent for 1–1000 identities. `DeleteNodeAndIgnoreSources` deletes a node and ignores its sources in one transaction.
- `infra_source_hosts` maps a Docker engine instance to the device that runs it, with version CAS (version 0 creates; only observed instances and device nodes). Deleting the node cascades the mapping. `Snapshot.SourceHosts` lists every stored mapping plus an unassigned entry for each observed Docker instance.
- `ObserveSources` is the enumeration path. `UpsertObservation` is the single-item write (for example after a source edit) and must never stale sibling observations.
- Infra hides blank/default device names and opaque hexadecimal, UUID or MAC-only names from Registry and FRITZ!Box candidates and untouched unplaced imports. Keep observations and nodes stored; edited (version > 1), placed, grouped, edge-linked and engine-host-mapped nodes stay visible. Named devices and non-device provider resources remain available.
- Passive snapshots return stored nodes immediately and schedule at most one bounded, config-revocable refresh owned by the server lifetime. Explicit refreshes and action verification still await providers. Drain background refreshes before closing the shared database.
- Server-populated `Node` action, app-link, `Power`, `HostID` and `HostInferred` fields and `SourceHost.InferredNodeID` are runtime projections only. Reset them on store reads/writes and never persist caller-supplied permissions or hosts. A guest's host is its Proxmox `runs_on` provider edge target, else its Docker engine mapping, else the only device whose address matches the configured engine (`HostInferred`); the projection never resolves host names. `SourceObserved.Power` is the adapter's bounded vocabulary (`running`, `stopped`, `paused`, `unknown`, or empty) and is stored with the observation. Per-source action maps use `CandidateID(SourceIdentity)` as the key so actions remain scoped to their source.
- Reserve each mutating action request before dispatch. A lingering `running` reservation becomes `unknown` after restart; final `accepted`, `completed`, `failed`, and `unknown` results remain replayable. Reusing a request ID with a different action fingerprint is a conflict. A `running` row older than `actionRunningTimeout` is reported and stored as `unknown` on read, so a lost completion cannot hang a replay for the process lifetime.
- Keep source validation bounded and typed. Only absolute HTTP(S) links without userinfo are accepted. Add focused tests for migration, identity/link uniqueness, validation, freshness/removal, ignore, layout CAS and parents, edge duplicates, source host CAS, action replay, and manual registry safety.
```

- [ ] **Step 3: Rewrite the API overview of `documentation/infra.md`**

In `documentation/infra.md`, replace everything from the line `## API overview` up to and including the paragraph that ends with `` `request_id_conflict`, `action_disabled` and `provider_unavailable`. `` with:

```markdown
## API overview

All routes require a Desktop administrator session or an admin bearer token and
answer `no-store` JSON. Writes additionally require the Desktop write
permission and a configuration that is enabled and not read-only. Request
bodies are limited to 256 KiB and unknown JSON fields are rejected.

| Route | Purpose |
| --- | --- |
| `GET /api/desktop/infra/snapshot` | Stored graph plus `capabilities`, `source_hosts` and optional `source_errors`; `?refresh=1` waits for providers, `?include_ignored=1` adds ignored sources (`ignored: true`) |
| `GET /api/desktop/infra/candidates` | Import preview for configured sources; ignored sources only with `?include_ignored=1`, flagged `ignored: true` |
| `GET /api/desktop/infra/targets` | Selectable app targets (camera, Store app, MeshCore conversation) |
| `POST /api/desktop/infra/nodes` | Create a node (manual devices also create a `protocol: none` registry row) |
| `PUT`/`DELETE /api/desktop/infra/nodes/{id}` | Update (version CAS) or delete a node; `?version=` required for delete; `?ignore_sources=1` also ignores every linked source in the same transaction and answers `ignored` |
| `GET`/`PUT /api/desktop/infra/nodes/{id}/source-edit` | Read or write the source's own metadata |
| `POST /api/desktop/infra/nodes/{id}/actions` | Reserved power action with `request_id` and optional `confirmed` |
| `POST /api/desktop/infra/import` | Import a candidate or link it to `node_id` |
| `POST /api/desktop/infra/import/batch` | Import or link 1–500 `items` (`candidate_id`, optional `node_id`, `parent_id`, `placement {view, x, y, w, h, pinned}`); one forced refresh per provider, one transaction per item; answers `nodes` and per-item `errors` (`candidate_id`, `error`, `message`) |
| `PUT /api/desktop/infra/sources/ignore` | Ignore or restore 1–1000 sources: `{sources: [{kind, instance, key}], ignored}`; a linked source answers 409 `source_linked` |
| `PUT /api/desktop/infra/source-hosts` | Assign (`node_id`) or clear (`""`) the device that runs a Docker engine: `{kind: "docker", instance, node_id, version}`, version 0 creates; answers `source_host` |
| `POST /api/desktop/infra/edges`, `PUT`/`DELETE /edges/{id}` | Manual relationships (provider edges are read-only); a self-loop is invalid and a second `(from, to, kind)` answers 409 `edge_exists` |
| `PUT /api/desktop/infra/layout` | Atomic layout batch; items may set `parent_id` (absent keeps it, `""` clears it) and `w`/`h` for places (120–20000); answers the changed nodes only |

VMs and containers carry the runtime-only `host_id`, with `host_inferred: true`
when the host was guessed. A Proxmox guest uses its provider `runs_on` edge, a
Docker container uses its engine's `source_hosts` mapping, and otherwise the
only device whose address matches the configured engine: the IP of a TCP
endpoint, or one of this server's own addresses for a local socket, pipe or
loopback endpoint. Host names are never resolved. Each `source_hosts` entry
(`kind`, `instance`, `node_id`, `version`, `inferred_node_id`) covers a stored
mapping or an observed Docker engine. Ignored sources stay stored, survive
refreshes and the removal retention, and never appear as candidates by default;
importing or linking one clears the flag.

Stable error codes include `infra_disabled`, `infra_readonly`,
`infra_policy_changed`, `invalid_request`, `request_too_large` (413),
`not_found`, `conflict`, `has_children`, `edge_exists`, `source_unavailable`,
`source_linked`, `source_stale`, `source_readonly`, `source_edit_disabled`,
`confirmation_required`, `request_id_conflict`, `action_disabled`,
`invalid_candidate_id`, `invalid_node_id` and `provider_unavailable`.
```

Keep the following paragraph (`Infra is an operator surface: ...`) and the `## Local assets and development` section unchanged.

- [ ] **Step 4: Commit**

```bash
git add internal/infra/AGENTS.md documentation/infra.md
git commit -m "docs(infra): document ignore, source hosts, batch import and layout extensions" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: Final verification

**Files:** none modified (fix only what these checks reveal, in a follow-up commit that stages just the fixed paths)

- [ ] **Step 1: Formatting**

Run: `gofmt -l internal/infra internal/server`
Expected: no output. If a file this plan did not touch is listed, confirm it is pre-existing with `git diff --name-only cacc7a58b -- <file>` (empty output) and leave it alone.

- [ ] **Step 2: Vet**

Run: `go vet ./internal/infra/ ./internal/server/`
Expected: no output, exit code 0.

- [ ] **Step 3: Tests**

Run: `go test ./internal/infra/ ./internal/server/ -run 'Infra' -count=1`
Expected: `ok  	aurago/internal/infra` and `ok  	aurago/internal/server`. Most `internal/infra` test names do not contain "Infra", so also run the whole store package:
Run: `go test ./internal/infra/ -count=1 -v 2>&1 | grep -cE '^--- PASS'`
Expected: a count that includes the 12 new store tests, and no `--- FAIL` lines (`go test ./internal/infra/ -count=1` prints `ok`).

- [ ] **Step 4: Graph change analysis**

Run: `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"`
Expected: a report of the changed symbols and affected flows. Report the result. If the output says `partial: true` or `truncated: true`, the check is not clean: rerun it, and if it stays partial, say so explicitly in the report instead of calling it clean. Also run the regression comparison:
Run: `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope compare --base-ref "main" --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"`
Expected: changes limited to `internal/infra`, `internal/server/desktop_infra*`, `documentation/infra.md` and `documentation/infra-redesign/`.

- [ ] **Step 5: Hand-off notes for the orchestrator**

Report:
- the commit list (12 commits);
- every HIGH or CRITICAL impact warning recorded in Tasks 1–11, with the callers confirmed by text search;
- the detect-changes outcome;
- that a security-agent review of `PUT /sources/ignore`, `PUT /source-hosts`, `POST /import/batch` and `DELETE /nodes/{id}?ignore_sources=1` is due (spec §14).

---

## Spec coverage (self-review)

| Spec item | Task |
|---|---|
| §9.1 runtime `host_id` / `host_inferred`, reset like `power` (Proxmox edge, Docker mapping, inferred via own interface IPs or TCP IP) | 9 |
| §9.1 snapshot `source_hosts [{kind, instance, node_id, version, inferred_node_id}]` | 8 (stored and placeholder), 9 (inferred) |
| §9.3 `ignored_at`, never a candidate, excluded by default, `ignored` flag in the snapshot (client freshness ledger) | 5, 6 |
| §9.3 remove-and-ignore in one transaction | 7 |
| §10.1 schema v3, explicit v2→v3, full create writes 3, backup, migration test with rows | 1 |
| §10.2 `PUT /sources/ignore` (1–1000, idempotent, 409 `source_linked`, 404 unknown) | 5, 7, 11 |
| §10.2 `DELETE /nodes/{id}?version=N&ignore_sources=1` → `{status, id, ignored}`, `has_children` kept | 7, 11 |
| §10.2 `POST /import/batch` (1–500, one forced refresh, per-item tx, container parent, validated placement, per-item errors) | 10, 11 |
| §10.2 `PUT /source-hosts` (CAS, device only, `""` clears) | 8, 11 |
| §10.2 `PUT /layout` with `parent_id` pointer semantics and place `w/h` | 3 |
| §10.2 `POST /edges` self-loop 400, duplicate 409 `edge_exists` | 4 |
| §10.2 `GET /snapshot` / `GET /candidates` `?include_ignored=1` | 6 |
| §10.3 `Placement.w/h`, place kinds only, 120–20000, in `validatePlacements` and the layout batch | 2, 3 |
| §10.4 agent tools unaffected (no package outside `internal/server` imports `internal/infra`) | n/a |
| §13 Go tests incl. read-only and policy (non-admin) refusals | 1–11 |
| `documentation/infra.md` API section, `internal/infra/AGENTS.md` | 12 |
