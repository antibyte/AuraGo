# Ontofelia Memory Concepts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Übernehme nur die Ontofelia-Konzepte, die AuraGos Memory- und Knowledge-Graph-System messbar erklärbarer, korrigierbarer und vertrauenswürdiger machen.

**Architecture:** AuraGo bleibt eine einzelne Go-Binary mit SQLite, chromem-go und Vanilla-JS UI. Die RDF/OWL/SPARQL-Ideen werden als Konzepte adaptiert, nicht als neuer Runtime-Stack. Der erste Nutzenhebel ist ein explizites Claim/Evidence- und Statusmodell für Wissen, danach Konfliktauflösung, Erklärbarkeit und Regression-Messung.

**Tech Stack:** Go 1.26.1+, SQLite via `modernc.org/sqlite`, chromem-go, existing net/http server, vanilla JavaScript UI, existing translation files.

## Global Constraints

- Keine Rust-, Node-, CGO-, Oxigraph-, SPARQL- oder OWL-Runtime-Abhängigkeit als Default.
- Schemaänderungen müssen über bestehende SQLite-Migrationsmuster laufen und vorhandene Daten erhalten.
- SQLite läuft über `dbutil.Open` mit `PRAGMA foreign_keys=ON`; neue optionale Referenzen dürfen daher nicht mit leeren String-Defaults auf nicht existierende Fremdschlüssel zeigen.
- Neue Subpackages unter `internal/memory/` dürfen keine Import-Zyklen erzeugen. Pure Helper-Packages verwenden eigene DTOs; Adapter in `internal/memory` übersetzen zwischen KG-Modellen und Helper-DTOs.
- `config.yaml` bleibt unverändert, außer ein Feature-Flag ist zwingend und explizit begründet.
- Secrets, Rohtexte und Tool-Ausgaben müssen vor Persistenz und UI-Ausgabe über vorhandene Scrubbing-Mechanismen laufen. Evidence-Excerpts werden vor Persistenz gescrubbt und hart begrenzt.
- Neue UI-Texte erfordern Übersetzungen in allen unterstützten Locale-Dateien unter `ui/lang/`, insbesondere `cs`, `da`, `de`, `el`, `en`, `es`, `fr`, `hi`, `it`, `ja`, `nl`, `no`, `pl`, `pt`, `sv`, `zh`.
- Neue Tool-Operationen müssen Read-only-Konfigurationen respektieren.
- Vor jeder Symboländerung muss GitNexus `impact` auf das Symbol ausgeführt und der Blast Radius berichtet werden.
- Vor Commit muss GitNexus `detect_changes()` laufen.

---

## Review Source

- Bericht: `reports/ontofelia_analysis_2026-07-05.md`
- Aktuelle AuraGo-Bausteine:
  - `internal/memory/graph_schema.go`: SQLite KG Tabellen `kg_nodes`, `kg_edges`, FTS.
  - `internal/memory/graph_edge.go`: `AddEdge`, `UpdateEdge`, `DeleteEdge`, `PruneOutgoingRelationEdges`.
  - `internal/memory/graph_node.go`: `AddNode`, `UpdateNode`, `DeleteNode`, `MergeNodes`.
  - `internal/memory/short_term_init.go`: `memory_meta`, `memory_conflicts`, curation tables.
  - `internal/memory/memory_conflicts.go`: vector-memory conflict registration.
  - `internal/dbutil/open.go`: SQLite connection setup with `PRAGMA foreign_keys=ON`.
  - `internal/agent/remember_tool.go`: single memory write entry point.
  - `internal/agent/tool_args_execution.go`: native tool argument decoding.
  - `internal/agent/native_tools_memory.go`: `remember`, `knowledge_graph`, `memory_reflect` schemas.
  - `prompts/tools_manuals/knowledge_graph.md`: agent-facing KG manual.

## GitNexus Preflight Findings

Index status: 2 commits behind HEAD, still useful for planning. Re-run `npx gitnexus analyze` before implementation if stale.

| Symbol | File | Upstream Risk | Direct Callers | Affected Processes |
|--------|------|---------------|----------------|--------------------|
| `KnowledgeGraph.AddEdge` | `internal/memory/graph_edge.go` | LOW | 0 reported | 0 |
| `KnowledgeGraph.initTables` | `internal/memory/graph_schema.go` | LOW | 1 | `main` |
| `handleRemember` | `internal/agent/remember_tool.go` | LOW | 1 | 0 reported |
| `SQLiteMemory.RegisterMemoryConflict` | `internal/memory/memory_conflicts.go` | LOW | 1 | 0 reported |

Interpretation: The highest-value first phase is feasible because the main touchpoints are not high-risk hubs. Schema migration still affects startup, so it needs tight tests and backward-compatible defaults.

## Re-Review Corrections Applied

- `kg_claims.evidence_id` is nullable because AuraGo enables SQLite foreign-key checks. Claims without evidence must insert cleanly.
- The restricted reasoner uses package-local DTOs plus an adapter, avoiding an `internal/memory` import cycle.
- Tool-schema changes explicitly include the argument decoder in `internal/agent/tool_args_execution.go`.
- Server/API work now names concrete routes and dashboard payload fields instead of relying on vague "edge detail" wording.
- Delete/retract semantics are clarified: retract preserves an inactive edge and audit history; delete removes the edge but does not purge claim/evidence history unless a future explicit purge operation is added.
- Conflict detection is scoped to exclusive/single-valued predicates so multi-valued relations do not create noisy false positives.

## Concept Decision Matrix

| Ontofelia concept | AuraGo decision | Benefit | Priority |
|-------------------|-----------------|---------|----------|
| Claim/Evidence provenance | Adopt as SQLite tables for KG first, then memory_meta/LTM | Answers "where did this come from?", enables deletion and correction audit | P0 |
| Truth maintenance statuses | Adopt `accepted`, `superseded`, `retracted`, `rejected` | Makes "I was wrong" and "forget that" deterministic | P0 |
| Explicit belief revision | Adopt conflict records with winner/loser resolution | Turns contradictions into resolvable workflow instead of passive flags | P0 |
| Memory quality evals H3/H6 | Adopt deterministic Go test harness | Prevents regressions in contradiction and forget behavior | P1 |
| Provenance UX / explainability | Adopt as tool/API/UI surfaces | High user trust and easier debugging | P1 |
| Structured tool policy audit metadata | Adapt inside existing `audit_events` | Good operational value, lower than memory provenance | P2 |
| Restricted Go reasoner | Adapt later with simple rules only | Useful after provenance is stable | P2 |
| RDF/JSON-LD export | Adapt as optional export only | Interop/debug value without dependency risk | P3 |
| Multi-agent shared-world consolidation | Defer until Invasion Control knowledge sync needs it | Strategic but not immediate | P3 |
| RDF/OWL/SPARQL default backend | Reject for default AuraGo | Breaks single-binary/no-dependency value proposition | Never |
| Oxigraph/Reasonable integration | Reject as built-in | Rust/Node/RDF stack is too heavy for core | Never |

## Target File Structure

| File | Responsibility |
|------|----------------|
| `internal/memory/kg_provenance.go` | Claim, evidence, conflict structs plus helper methods |
| `internal/memory/kg_provenance_test.go` | Unit tests for claim/evidence persistence and lifecycle |
| `internal/memory/kg_reasoner_adapter.go` | Adapter from KG edges to pure reasoner DTOs |
| `internal/memory/graph_schema.go` | Schema migrations for provenance, status, and indexes |
| `internal/memory/graph_edge.go` | Provenance-aware edge writes and status transitions |
| `internal/memory/graph_query.go` | Query filters that prefer accepted knowledge by default |
| `internal/memory/graph_explore.go` | Explain and subgraph output with provenance summaries |
| `internal/memory/memory_conflicts.go` | Existing LTM conflict flow remains; add shared resolution concepts only where needed |
| `internal/agent/remember_tool.go` | Attach session/channel/source provenance when writing KG relationships |
| `internal/agent/native_tools_memory.go` | Add KG operations for explain, retract, supersede, resolve conflict |
| `internal/agent/tool_args_execution.go` | Decode new KG operation arguments |
| `internal/agent/tool_args_execution_test.go` | Verify new KG fields parse correctly |
| `internal/agent/agent_dispatch_exec.go` | Dispatch new KG operations while respecting read-only mode |
| `prompts/tools_manuals/knowledge_graph.md` | Document provenance/status/conflict operations |
| `internal/server/knowledge_graph_handlers.go` | Expose provenance and conflict data for UI |
| `internal/server/server_routes_config.go` | Register additive KG provenance/conflict routes |
| `internal/server/knowledge_graph_handlers_test.go` | API contract tests for provenance/conflict responses |
| `internal/server/dashboard_handlers_memory.go` | Add KG lifecycle/conflict counts to dashboard memory payload |
| `ui/js/dashboard/dashboard-widgets.js` | Surface open KG conflicts and provenance summary where useful |
| `ui/lang/dashboard/*.json` and any other touched locale files | Translations for every new user-visible label |

---

## Phase 1 - Foundation: Provenance And Truth Maintenance

### Task 1: Add KG Claim/Evidence Schema

**Files:**
- Modify: `internal/memory/graph_schema.go`
- Create: `internal/memory/kg_provenance.go`
- Create: `internal/memory/kg_provenance_test.go`

**Interfaces:**
- Produces:
  - `type KGClaimStatus string`
  - `const KGClaimAccepted`, `KGClaimSuperseded`, `KGClaimRetracted`, `KGClaimRejected`
  - `type KGClaim struct`
  - `type KGEvidence struct`
  - `type KGConflict struct`
  - `func (kg *KnowledgeGraph) GetClaimsForEdge(source, target, relation string, includeInactive bool, limit int) ([]KGClaim, error)`

- [ ] **Step 1: Run pre-edit impact**

Run: `mcp__gitnexus.impact({repo:"AuraGo", target:"initTables", file_path:"internal/memory/graph_schema.go", direction:"upstream", summaryOnly:true})`

Expected: LOW risk or a clearly reported higher risk before editing.

- [ ] **Step 2: Write failing schema test**

Create `internal/memory/kg_provenance_test.go` with a test that opens `NewKnowledgeGraph(":memory:", "", logger)` and verifies these tables exist:

```go
func TestKnowledgeGraphInitializesProvenanceTables(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	kg, err := NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })

	for _, table := range []string{"kg_claims", "kg_evidence", "kg_conflicts"} {
		var name string
		err := kg.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
}
```

Add a second test that inserts a `kg_claims` row with `evidence_id` omitted and verifies it succeeds. This guards the `foreign_keys=ON` behavior from `internal/dbutil/open.go`.

- [ ] **Step 3: Run failing test**

Run: `go test ./internal/memory -run TestKnowledgeGraphInitializesProvenanceTables -v`

Expected: FAIL with a missing table assertion.

- [ ] **Step 4: Implement schema**

Add tables in `KnowledgeGraph.initTables`:

```sql
CREATE TABLE IF NOT EXISTS kg_evidence (
    id TEXT PRIMARY KEY,
    evidence_type TEXT NOT NULL DEFAULT '',
    source_message_id TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL DEFAULT '',
    channel TEXT NOT NULL DEFAULT '',
    actor TEXT NOT NULL DEFAULT '',
    raw_text TEXT NOT NULL DEFAULT '',
    source_uri TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    captured_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS kg_claims (
    id TEXT PRIMARY KEY,
    subject_id TEXT NOT NULL,
    predicate TEXT NOT NULL,
    object_id TEXT NOT NULL DEFAULT '',
    object_literal TEXT NOT NULL DEFAULT '',
    asserted_in_graph TEXT NOT NULL DEFAULT 'local:worldview',
    learned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    accepted_at DATETIME,
    confidence REAL NOT NULL DEFAULT 0.75,
    confidence_label TEXT NOT NULL DEFAULT '',
    source_kind TEXT NOT NULL DEFAULT 'system',
    ingestion_run_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'accepted',
    superseded_by TEXT NOT NULL DEFAULT '',
    source_message_id TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL DEFAULT '',
    privacy_class TEXT NOT NULL DEFAULT 'normal',
    retention_policy TEXT NOT NULL DEFAULT 'default',
    evidence_id TEXT DEFAULT NULL,
    FOREIGN KEY(evidence_id) REFERENCES kg_evidence(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS kg_conflicts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    subject_id TEXT NOT NULL,
    predicate TEXT NOT NULL,
    left_claim_id TEXT NOT NULL,
    right_claim_id TEXT NOT NULL,
    winning_claim_id TEXT DEFAULT NULL,
    superseded_claim_id TEXT DEFAULT NULL,
    reason TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open',
    detected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    resolved_at DATETIME
);
```

Add indexes:

```sql
CREATE INDEX IF NOT EXISTS idx_kg_claims_fact ON kg_claims(subject_id, predicate, object_id, object_literal);
CREATE INDEX IF NOT EXISTS idx_kg_claims_status ON kg_claims(status, learned_at DESC);
CREATE INDEX IF NOT EXISTS idx_kg_claims_evidence ON kg_claims(evidence_id);
CREATE INDEX IF NOT EXISTS idx_kg_conflicts_status ON kg_conflicts(status, detected_at DESC);
CREATE INDEX IF NOT EXISTS idx_kg_conflicts_fact ON kg_conflicts(subject_id, predicate);
CREATE UNIQUE INDEX IF NOT EXISTS idx_kg_conflicts_pair_open
    ON kg_conflicts(subject_id, predicate, left_claim_id, right_claim_id)
    WHERE status='open';
```

- [ ] **Step 5: Add Go structs**

In `kg_provenance.go`, define typed structs with JSON tags matching the SQL columns. Keep status constants as strings for stable API output. Use `sql.NullString` or equivalent scan helpers internally for nullable `evidence_id`, `winning_claim_id`, `superseded_claim_id`, and `resolved_at`, while exposing clean JSON fields with `omitempty`.

- [ ] **Step 6: Verify**

Run: `go test ./internal/memory -run TestKnowledgeGraphInitializesProvenanceTables -v`

Expected: PASS.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal/memory/graph_schema.go internal/memory/kg_provenance.go internal/memory/kg_provenance_test.go
git commit -m "feat(memory): add kg provenance schema"
```

### Task 2: Record Claims For Manual KG Edge Writes

**Files:**
- Modify: `internal/memory/kg_provenance.go`
- Modify: `internal/memory/graph_edge.go`
- Modify: `internal/memory/kg_provenance_test.go`

**Interfaces:**
- Produces:
  - `type KGProvenanceInput struct`
  - `func (kg *KnowledgeGraph) AddEdgeWithProvenance(source, target, relation string, properties map[string]string, provenance KGProvenanceInput) (*KGClaim, error)`
  - `func (kg *KnowledgeGraph) GetClaimsForEdge(source, target, relation string, includeInactive bool, limit int) ([]KGClaim, error)`
- Preserves:
  - `func (kg *KnowledgeGraph) AddEdge(source, target, relation string, properties map[string]string) error`

- [ ] **Step 1: Run pre-edit impact**

Run: `mcp__gitnexus.impact({repo:"AuraGo", target:"AddEdge", file_path:"internal/memory/graph_edge.go", direction:"upstream", summaryOnly:true})`

Expected: LOW risk or a user-visible warning if risk increases.

- [ ] **Step 2: Write failing provenance test**

Add a test that calls `AddEdgeWithProvenance("andi", "german", "prefers_language", nil, KGProvenanceInput{SourceKind:"user", SessionID:"s1", Channel:"web", RawText:"Andi prefers German", Confidence:0.95})`, then asserts one accepted claim and one evidence row exist.

Add a companion test that calls `AddEdgeWithProvenance` with only `SourceKind:"manual"` and `Confidence:1.0`; it must create an accepted claim with `evidence_id` unset and no foreign-key failure.

- [ ] **Step 3: Run failing test**

Run: `go test ./internal/memory -run TestAddEdgeWithProvenanceRecordsClaimAndEvidence -v`

Expected: FAIL because `AddEdgeWithProvenance` does not exist.

- [ ] **Step 4: Implement minimal method**

Implement `AddEdgeWithProvenance` as a transaction:

1. Ensure source and target placeholder nodes.
2. Upsert the edge using existing edge merge rules.
3. Normalize status/source defaults and generate claim/evidence IDs with a helper such as `newKGClaimID(now time.Time)` and `newKGEvidenceID(now time.Time)`. Use `crypto/rand` for a short suffix and fall back to timestamp-only IDs if entropy fails.
4. Scrub and truncate `KGProvenanceInput.RawText` before storing it in `kg_evidence.raw_text`; use existing `internal/security` scrubbing helpers and keep the persisted excerpt to a fixed maximum such as 2,000 characters.
5. Insert `kg_evidence` when evidence input has scrubbed raw text, URI, message ID, session, channel, or actor.
6. Insert `kg_claims` with `status='accepted'`; leave `evidence_id` NULL when no evidence row was created.
7. Commit.
8. Reindex nodes and edge using existing semantic-index calls.

Keep `AddEdge` as a wrapper:

```go
func (kg *KnowledgeGraph) AddEdge(source, target, relation string, properties map[string]string) error {
	_, err := kg.AddEdgeWithProvenance(source, target, relation, properties, KGProvenanceInput{
		SourceKind: "manual",
		Confidence: 1.0,
	})
	return err
}
```

- [ ] **Step 5: Verify compatibility**

Run: `go test ./internal/memory -run 'TestAddEdge|TestAddEdgeWithProvenance|TestKnowledgeGraph' -v`

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/memory/graph_edge.go internal/memory/kg_provenance.go internal/memory/kg_provenance_test.go
git commit -m "feat(memory): record provenance for kg edges"
```

### Task 3: Add Accepted/Superseded/Retracted Edge Lifecycle

**Files:**
- Modify: `internal/memory/graph_schema.go`
- Modify: `internal/memory/graph_edge.go`
- Modify: `internal/memory/graph_query.go`
- Modify: `internal/memory/graph_explore.go`
- Modify: `internal/memory/kg_provenance.go`
- Modify: `internal/memory/kg_provenance_test.go`

**Interfaces:**
- Produces:
  - `func (kg *KnowledgeGraph) SupersedeEdge(source, target, relation, supersededByClaimID, reason string) error`
  - `func (kg *KnowledgeGraph) RetractEdge(source, target, relation, reason string) error`
  - Active KG reads exclude `status IN ('superseded','retracted','rejected')` unless an audit operation explicitly asks for inactive knowledge.

- [ ] **Step 1: Run pre-edit impact**

Run impact for `UpdateEdge`, `DeleteEdge`, and the read method being changed, using `summaryOnly:true`.

- [ ] **Step 2: Write failing lifecycle tests**

Add tests for:

1. Superseded edge remains in DB but is hidden from `GetImportantEdges`.
2. Retracted edge remains in DB but is hidden from default search/read paths.
3. `GetClaimsForEdge(..., includeInactive=true, ...)` still returns historical claims.

- [ ] **Step 3: Run failing tests**

Run: `go test ./internal/memory -run 'TestSupersededEdgesAreHiddenFromDefaultReads|TestRetractedEdgesKeepClaimHistory' -v`

Expected: FAIL because lifecycle methods and filters do not exist.

- [ ] **Step 4: Implement edge status columns**

Add nullable/defaulted columns through migration:

```sql
ALTER TABLE kg_edges ADD COLUMN status TEXT NOT NULL DEFAULT 'accepted';
ALTER TABLE kg_edges ADD COLUMN status_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE kg_edges ADD COLUMN superseded_by_claim_id TEXT NOT NULL DEFAULT '';
ALTER TABLE kg_edges ADD COLUMN retracted_at DATETIME;
```

Also add:

```sql
CREATE INDEX IF NOT EXISTS idx_kg_edges_status ON kg_edges(status);
```

Use the existing column-migration style in `graph_schema.go` rather than rebuilding tables. Do not rely on new CHECK constraints for altered columns; validate lifecycle status values in Go before writes.

- [ ] **Step 5: Implement status transitions**

`SupersedeEdge` sets:

- `kg_edges.status='superseded'`
- `kg_edges.superseded_by_claim_id=<new claim id>`
- `kg_edges.status_reason=<reason>`
- matching accepted `kg_claims.status='superseded'`

`RetractEdge` sets:

- `kg_edges.status='retracted'`
- `kg_edges.retracted_at=CURRENT_TIMESTAMP`
- matching accepted `kg_claims.status='retracted'`

- [ ] **Step 6: Update default read filters**

Add a small helper such as:

```go
func activeKGEdgePredicate(alias string) string {
	if alias == "" {
		return "COALESCE(status, 'accepted') = 'accepted'"
	}
	return fmt.Sprintf("COALESCE(%s.status, 'accepted') = 'accepted'", alias)
}
```

Apply it to every user-facing read over `kg_edges`, including:

- `GetAllEdges`
- `GetImportantEdges`
- `Search` edge FTS branch
- `getNeighborsWithQueryer`
- `ExploreSubgraph`
- edge quality and cleanup reads that feed dashboard/agent-facing summaries

Audit/explain operations may include inactive edges only when `includeInactive=true`.

- [ ] **Step 7: Verify**

Run:

```bash
go test ./internal/memory -run 'KnowledgeGraph|Edge|Provenance|Retract|Supersede' -v
```

Expected: PASS.

- [ ] **Step 8: Commit**

Run:

```bash
git add internal/memory/graph_schema.go internal/memory/graph_edge.go internal/memory/graph_query.go internal/memory/graph_explore.go internal/memory/kg_provenance.go internal/memory/kg_provenance_test.go
git commit -m "feat(memory): add kg truth maintenance lifecycle"
```

### Task 4: Detect And Resolve KG Claim Conflicts

**Files:**
- Modify: `internal/memory/kg_provenance.go`
- Modify: `internal/memory/kg_provenance_test.go`
- Modify: `internal/memory/graph_edge.go`

**Interfaces:**
- Produces:
  - `func (kg *KnowledgeGraph) RegisterKGConflict(subjectID, predicate, leftClaimID, rightClaimID, reason string) error`
  - `func (kg *KnowledgeGraph) GetOpenKGConflicts(limit int) ([]KGConflict, error)`
  - `func (kg *KnowledgeGraph) ResolveKGConflict(id int64, winningClaimID, reason string) error`
  - `func isExclusiveKGPredicate(predicate string, properties map[string]string) bool`

- [ ] **Step 1: Run pre-edit impact**

Run impact on `AddEdgeWithProvenance` and `RegisterMemoryConflict` once the new method exists.

- [ ] **Step 2: Write failing conflict tests**

Create a test that records:

- `user -[primary_language]-> german`
- `user -[primary_language]-> english`

Expected:

- one open conflict for subject `user` and predicate `primary_language`
- resolving with the second claim supersedes the first claim and edge
- accepted read paths return only `english`

Add a negative test for a multi-valued predicate such as `uses_tool`; two different targets must not create an open conflict.

- [ ] **Step 3: Implement conflict detection**

During `AddEdgeWithProvenance`, after inserting the new claim:

1. Check whether the predicate is exclusive/single-valued using a helper such as `isExclusiveKGPredicate(predicate string, properties map[string]string) bool`.
2. Treat a predicate as exclusive only when properties mark it with `cardinality=single` or it is in a small curated set such as `primary_language`, `default_language`, `current_ip`, `current_hostname`, `primary_owner`.
3. For non-exclusive predicates, do not register conflicts automatically.
4. For exclusive predicates, find accepted claims with same `subject_id` and `predicate`.
5. Ignore exact same object.
6. Register `kg_conflicts` with `left_claim_id`, `right_claim_id`, and `status='open'`. Use the partial unique index to avoid duplicate open conflicts for the same pair.
7. Do not auto-supersede unless the caller explicitly uses a correction operation.

- [ ] **Step 4: Implement resolution**

`ResolveKGConflict` must:

1. Verify conflict exists and is open.
2. Verify winning claim belongs to the conflict subject/predicate.
3. Determine the losing claim from `left_claim_id`/`right_claim_id`.
4. Mark losing accepted claims as `superseded` and set `superseded_by` to the winning claim.
5. Mark losing matching edges as `superseded`.
6. Set `kg_conflicts.status='resolved'`, `winning_claim_id`, `superseded_claim_id`, `reason`, and `resolved_at`.

- [ ] **Step 5: Verify**

Run:

```bash
go test ./internal/memory -run 'KGConflict|Provenance|Supersede' -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/memory/kg_provenance.go internal/memory/kg_provenance_test.go internal/memory/graph_edge.go
git commit -m "feat(memory): resolve kg claim conflicts"
```

---

## Phase 2 - Agent And User Surfaces

### Task 5: Add Tool Operations For Explain, Retract, Supersede, Resolve

**Files:**
- Modify: `internal/agent/native_tools_memory.go`
- Modify: `internal/agent/tool_args_execution.go`
- Modify: `internal/agent/tool_args_execution_test.go`
- Modify: `internal/agent/agent_dispatch_exec.go`
- Modify: `internal/agent/agent_dispatch_exec_test.go`
- Modify: `prompts/tools_manuals/knowledge_graph.md`

**Interfaces:**
- Adds `knowledge_graph.operation` enum values:
  - `explain_edge`
  - `list_conflicts`
  - `resolve_conflict`
  - `supersede_edge`
  - `retract_edge`

- [ ] **Step 1: Run pre-edit impact**

Run impact on the dispatch switch function and `appendMemoryToolSchemas`.

- [ ] **Step 2: Write dispatch tests**

Add tests that verify:

- read-only mode permits `explain_edge` and `list_conflicts`
- read-only mode blocks `resolve_conflict`, `supersede_edge`, and `retract_edge`
- unknown conflict IDs return structured error JSON

- [ ] **Step 3: Update tool schema**

Add operation enum values and properties:

- `claim_id`
- `conflict_id`
- `reason`
- `include_inactive`

Update `knowledgeGraphArgs` and `decodeKnowledgeGraphArgs` in `internal/agent/tool_args_execution.go` with matching fields:

- `ClaimID string`
- `ConflictID int64`
- `Reason string`
- `IncludeInactive bool`

Add tests in `internal/agent/tool_args_execution_test.go` that parse all four fields from JSON.

- [ ] **Step 4: Implement dispatch**

Map operations to memory methods:

- `explain_edge` -> `GetClaimsForEdge(source, target, relation, includeInactive, limit)`
- `list_conflicts` -> `GetOpenKGConflicts`
- `resolve_conflict` -> `ResolveKGConflict`
- `supersede_edge` -> `SupersedeEdge`
- `retract_edge` -> `RetractEdge`

- [ ] **Step 5: Update manual**

Document that `delete_edge` removes the active edge row, while `retract_edge` preserves the edge as inactive with audit history. Claims and evidence are retained for both operations unless a later explicit purge operation is introduced. Recommend `retract_edge` for "forget/correction" workflows unless the user asks for deletion.

- [ ] **Step 6: Verify**

Run:

```bash
go test ./internal/agent -run 'KnowledgeGraph|DispatchExec' -v
go test ./internal/memory -run 'KGConflict|Provenance|Retract|Supersede' -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal/agent/native_tools_memory.go internal/agent/tool_args_execution.go internal/agent/tool_args_execution_test.go internal/agent/agent_dispatch_exec.go internal/agent/agent_dispatch_exec_test.go prompts/tools_manuals/knowledge_graph.md
git commit -m "feat(agent): expose kg provenance operations"
```

### Task 6: Wire `remember` To Provenance-Aware KG Writes

**Files:**
- Modify: `internal/agent/remember_tool.go`
- Modify: `internal/agent/remember_tool_test.go` if present, otherwise create it

**Interfaces:**
- Consumes: `KnowledgeGraph.AddEdgeWithProvenance`
- Produces: `remember` relationship writes with `source_kind='user'`, current `session_id`, and raw content evidence.

- [ ] **Step 1: Run pre-edit impact**

Run: `mcp__gitnexus.impact({repo:"AuraGo", target:"handleRemember", file_path:"internal/agent/remember_tool.go", direction:"upstream", summaryOnly:true})`

- [ ] **Step 2: Write failing test**

Test `rememberAsGraphEdge` with explicit `Source`, `Target`, `Relation`, and `Content`, then verify `GetClaimsForEdge` returns one accepted claim with the session ID supplied to `handleRemember`.

- [ ] **Step 3: Implement**

Replace direct `kg.AddEdge` call in `rememberAsGraphEdge` with `kg.AddEdgeWithProvenance`, passing:

- `SourceKind: "user"`
- `SessionID: sessionID`
- `RawText: content`
- `Confidence: 0.90`

Adjust the helper signature explicitly:

```go
func rememberAsGraphEdge(content string, tc ToolCall, kg *memory.KnowledgeGraph, sessionID string, logger *slog.Logger) string
```

Update the direct call inside `handleRemember` to pass its `sessionID`. Do not thread session data through unrelated memory paths.

- [ ] **Step 4: Verify**

Run:

```bash
go test ./internal/agent -run 'Remember|MemoryTarget' -v
go test ./internal/memory -run 'Provenance' -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/agent/remember_tool.go internal/agent/remember_tool_test.go
git commit -m "feat(agent): attach provenance to remembered relationships"
```

### Task 7: Add Explainability API And Dashboard Surfacing

**Files:**
- Modify: `internal/server/knowledge_graph_handlers.go`
- Modify: `internal/server/server_routes_config.go`
- Modify: `internal/server/knowledge_graph_handlers_test.go`
- Modify: `internal/server/dashboard_handlers_memory.go`
- Modify: `ui/js/dashboard/dashboard-widgets.js`
- Modify: `ui/lang/dashboard/*.json` and any other relevant locale files

**Interfaces:**
- Produces:
  - `GET /api/knowledge-graph/edge/claims?source=&target=&relation=&include_inactive=`
  - `GET /api/knowledge-graph/conflicts?limit=`
  - `POST /api/knowledge-graph/conflicts/resolve`
  - Dashboard payload fields `open_conflicts`, `accepted_edges`, `superseded_edges`, `retracted_edges`.

- [ ] **Step 1: Run API impact**

Run GitNexus `impact` on the affected handler symbols and the route-registration symbol in `internal/server/server_routes_config.go`. Report any HIGH or CRITICAL route blast radius before editing.

Also run impact on `knowledgeGraphDashboardPayload` before changing dashboard payload shape.

- [ ] **Step 2: Write API contract tests**

Add tests that:

- create a KG edge with provenance
- call `/api/knowledge-graph/edge/claims`
- assert response includes claim ID, status, source kind, confidence, learned timestamp, and sanitized evidence summary
- create an open conflict and assert `/api/knowledge-graph/conflicts` returns it without raw evidence text

- [ ] **Step 3: Implement API output**

Expose concise claim fields only. Do not return raw evidence text by default; return hash, source kind, session/channel, and a scrubbed short excerpt when needed.

Register the new routes in `internal/server/server_routes_config.go`. Keep existing `/api/knowledge-graph/node`, `/api/knowledge-graph/edge`, `/api/knowledge-graph/health`, and dashboard routes backward compatible. The two GET routes are read endpoints; `POST /api/knowledge-graph/conflicts/resolve` must use the same admin protection pattern as other KG mutations.

Add the dashboard counts in `knowledgeGraphDashboardPayload` in `internal/server/dashboard_handlers_memory.go`.

- [ ] **Step 4: Update dashboard**

Add a compact KG health section row for:

- open KG conflicts
- accepted facts count
- retracted facts count
- superseded facts count

Use existing dashboard visual style. Do not add alert dialogs.

- [ ] **Step 5: Update translations**

Add new labels in all supported dashboard locale files: `cs`, `da`, `de`, `el`, `en`, `es`, `fr`, `hi`, `it`, `ja`, `nl`, `no`, `pl`, `pt`, `sv`, `zh`. German must use personal form where phrasing addresses the user.

- [ ] **Step 6: Verify**

Run:

```bash
go test ./internal/server -run 'KnowledgeGraph|Dashboard' -v
go test ./ui -run 'Dashboard|Lang|Translation' -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal/server/knowledge_graph_handlers.go internal/server/server_routes_config.go internal/server/knowledge_graph_handlers_test.go internal/server/dashboard_handlers_memory.go ui/js/dashboard/dashboard-widgets.js ui/lang
git commit -m "feat(ui): show kg provenance and conflicts"
```

---

## Phase 3 - Measurement And Safe Semantics

### Task 8: Add Memory Quality Eval Harness For H3 And H6

**Files:**
- Create: `internal/memory/memory_eval_test.go`
- Modify: `internal/memory/memory_conflicts_test.go`
- Modify: `internal/memory/kg_provenance_test.go`

**Interfaces:**
- Produces deterministic tests for:
  - H3 contradiction flagging.
  - H6 forget/retract on command.

- [ ] **Step 1: Write eval tests**

Add table-driven tests:

| Scenario | Expected |
|----------|----------|
| "User primary language is German" then "User primary language is English" | conflict is open |
| resolve conflict with English | German is superseded, English accepted |
| retract English | accepted read path returns no active language preference |
| delete edge | active edge row is removed, while claim/evidence history remains queryable through provenance APIs |

- [ ] **Step 2: Verify tests fail where functionality is missing**

Run: `go test ./internal/memory -run 'Eval|Conflict|Retract|Supersede' -v`

Expected: PASS after Phase 1 and FAIL before it, which confirms the harness catches behavior.

- [ ] **Step 3: Add benchmark-style docs inside test comments**

Document the H3/H6 names in comments so future maintainers understand why these scenarios matter.

- [ ] **Step 4: Commit**

Run:

```bash
git add internal/memory/memory_eval_test.go internal/memory/memory_conflicts_test.go internal/memory/kg_provenance_test.go
git commit -m "test(memory): cover contradiction and forget semantics"
```

### Task 9: Add Restricted Go Reasoning Layer

**Files:**
- Create: `internal/memory/kgreasoner/reasoner.go`
- Create: `internal/memory/kgreasoner/reasoner_test.go`
- Create: `internal/memory/kg_reasoner_adapter.go`
- Modify: `internal/agent/native_tools_memory.go`
- Modify: `internal/agent/agent_dispatch_exec.go`
- Modify: `internal/agent/agent_dispatch_exec_test.go`
- Modify: `prompts/tools_manuals/knowledge_graph.md`

**Interfaces:**
- Produces:
  - `type RuleSet struct`
  - `type EdgeFact struct`
  - `type InferredFact struct`
  - `func Infer(edges []EdgeFact, rules RuleSet) []InferredFact`

- [ ] **Step 1: Keep scope intentionally narrow**

Support only:

- transitive relations: `part_of`, `located_in`, `depends_on`
- inverse relations: configured pairs such as `hosts` <-> `hosted_by`
- disjoint type conflicts for `person`, `device`, `service`

- [ ] **Step 2: Write pure package tests**

Tests must not require an LLM or external service.

The `kgreasoner` package must not import `aurago/internal/memory`; it uses `EdgeFact` DTOs only. Put AuraGo-specific conversion in `internal/memory/kg_reasoner_adapter.go`.

- [ ] **Step 3: Implement pure Go inference**

Return inferred facts as candidates. Do not auto-write them to the KG until a later explicit operation approves them.

- [ ] **Step 4: Add optional KG operation**

Add read-only `knowledge_graph` operation `suggest_inferred_relations` that returns inferred candidates with reason strings.

- [ ] **Step 5: Verify**

Run:

```bash
go test ./internal/memory/kgreasoner -v
go test ./internal/agent -run 'KnowledgeGraph' -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/memory/kgreasoner internal/memory/kg_reasoner_adapter.go internal/agent/native_tools_memory.go internal/agent/agent_dispatch_exec.go internal/agent/agent_dispatch_exec_test.go prompts/tools_manuals/knowledge_graph.md
git commit -m "feat(memory): add restricted kg inference suggestions"
```

### Task 10: Add Optional RDF/JSON-LD Export

**Files:**
- Create: `internal/memory/kg_export.go`
- Create: `internal/memory/kg_export_test.go`
- Modify: `internal/agent/native_tools_memory.go`
- Modify: `internal/agent/agent_dispatch_exec.go`
- Modify: `prompts/tools_manuals/knowledge_graph.md`

**Interfaces:**
- Produces:
  - `func (kg *KnowledgeGraph) ExportJSONLD(includeInactive bool, limit int) ([]byte, error)`
  - Optional `knowledge_graph` operation `export_jsonld`

- [ ] **Step 1: Write export tests**

Create KG nodes/edges/claims and assert JSON-LD contains:

- `@context`
- node IDs
- relation names
- claim IDs
- status fields

- [ ] **Step 2: Implement JSON-LD only**

Do not add Turtle, SPARQL, RDF store, or sidecar runtime. JSON-LD is enough for interoperability and debugging.

- [ ] **Step 3: Add tool operation**

Return compact JSON-LD with `limit` and `include_inactive` controls. Apply output compression policy through existing tool-output flow.

- [ ] **Step 4: Verify**

Run:

```bash
go test ./internal/memory -run 'ExportJSONLD|KnowledgeGraph' -v
go test ./internal/agent -run 'KnowledgeGraph' -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/memory/kg_export.go internal/memory/kg_export_test.go internal/agent/native_tools_memory.go internal/agent/agent_dispatch_exec.go prompts/tools_manuals/knowledge_graph.md
git commit -m "feat(memory): export kg as jsonld"
```

---

## Lower-Priority Adaptations

### Structured Tool Policy Audit Metadata

Adopt only after Phase 1/2. AuraGo already has `RuntimePermissions`, feature flags, read-only toggles, Guardian-style checks, and `audit_events`. The useful addition is not a new policy engine; it is structured metadata per tool call:

- `policy_decision`: allowed, denied, sanitized, guardian_required
- `permission_gate`: config field or runtime permission that decided it
- `sandbox_backend`: none, landlock, process, sidecar
- `rate_limit_status`: not_applicable, allowed, blocked

This belongs in `audit_events.metadata_json`, not in a parallel `audit.jsonl`.

### Multi-Agent Shared World Consolidation

Defer until Invasion Control has a concrete knowledge-sync use case. The right AuraGo shape is:

- local facts stay in `local:<nest>:worldview`
- promoted facts move to `shared:world`
- promotion requires independent evidence, confidence threshold, and no open conflict

Do not implement this before claim/evidence status is stable.

## Explicit Non-Goals

- No RDF triplestore as default storage.
- No SPARQL query engine.
- No OWL-DL reasoner.
- No Rust/Node sidecar for core memory.
- No automatic deletion of contradicted or retracted knowledge.
- No raw evidence text in prompt injection or dashboard by default.

## Recommended Execution Order

1. Task 1
2. Task 2
3. Task 3
4. Task 4
5. Task 8
6. Task 5
7. Task 6
8. Task 7
9. Task 9
10. Task 10

This order ships the most valuable behavior before UI polish and optional semantic extras. Tasks 9 and 10 can be skipped without weakening the core benefit.

## Definition Of Done

- `go test ./internal/memory/...` passes.
- `go test ./internal/agent/...` passes for affected dispatch/tool tests.
- `go test ./internal/server/...` passes when API/UI surfaces are changed.
- Translation tests pass when UI labels are added.
- `knowledge_graph` read-only mode blocks mutations and allows explain/list operations.
- A user can ask "why do you believe this relationship?" and receive source, confidence, status, and evidence summary.
- A correction creates or resolves a conflict without deleting historical evidence.
- `detect_changes({scope:"all"})` reports only expected Memory, Agent, Server, UI, and prompt-manual scope.
