package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

func TestKGTimestampMigrationNormalizesMixedFormatsAndBacksUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graph.db")
	kg, err := NewKnowledgeGraph(path, "", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("n", "Node", nil); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddEdge("n", "m", "rel", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_nodes SET updated_at=?,semantic_indexed_at=? WHERE id='n'`,
		"2026-10-20T16:30:00+02:00", "2026-10-20 17:30:00+03:00"); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_edges SET updated_at=?,semantic_indexed_at=? WHERE source='n'`,
		"2026-10-20 16:45:00.123456789Z", "2026-10-20T18:45:00+02:00"); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`DELETE FROM kg_meta WHERE key=?`, kgTimestampMigrationKey); err != nil {
		t.Fatal(err)
	}
	if err := kg.Close(); err != nil {
		t.Fatal(err)
	}

	kg, err = NewKnowledgeGraph(path, "", slog.Default())
	if err != nil {
		t.Fatalf("reopen and normalize graph timestamps: %v", err)
	}
	defer kg.Close()
	for _, tc := range []struct {
		query string
		want  string
	}{
		{`SELECT CAST(updated_at AS TEXT) FROM kg_nodes WHERE id='n'`, "2026-10-20 14:30:00"},
		{`SELECT CAST(semantic_indexed_at AS TEXT) FROM kg_nodes WHERE id='n'`, "2026-10-20 14:30:00"},
		{`SELECT CAST(updated_at AS TEXT) FROM kg_edges WHERE source='n'`, "2026-10-20 16:45:00"},
		{`SELECT CAST(semantic_indexed_at AS TEXT) FROM kg_edges WHERE source='n'`, "2026-10-20 16:45:00"},
	} {
		var got string
		if err := kg.db.QueryRow(tc.query).Scan(&got); err != nil || got != tc.want {
			t.Errorf("%s = %q, %v; want %q", tc.query, got, err, tc.want)
		}
	}
	var marker string
	if err := kg.db.QueryRow(`SELECT value FROM kg_meta WHERE key=?`, kgTimestampMigrationKey).Scan(&marker); err != nil || marker != "1" {
		t.Fatalf("timestamp migration marker = %q, %v; want 1", marker, err)
	}
	var userVersion int
	if err := kg.db.QueryRow(`PRAGMA user_version`).Scan(&userVersion); err != nil || userVersion != kgSchemaVersion {
		t.Fatalf("KG user_version = %d, %v; want %d", userVersion, err, kgSchemaVersion)
	}
	backups, err := filepath.Glob(path + ".kg-utc-timestamps-v1-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("timestamp migration backups = %v, %v; want one consistent backup", backups, err)
	}
	backup, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var original string
	if err := backup.QueryRow(`SELECT updated_at FROM kg_nodes WHERE id='n'`).Scan(&original); err != nil || original != "2026-10-20T16:30:00+02:00" {
		t.Fatalf("backup timestamp = %q, %v; want original mixed-format value", original, err)
	}
}

func TestKGNewerUserVersionFailsBeforeSchemaMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE keep_me(value TEXT); PRAGMA user_version=999`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := NewKnowledgeGraph(path, "", slog.Default()); err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("NewKnowledgeGraph error = %v, want newer-version guard", err)
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var graphTables int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name LIKE 'kg_%'`).Scan(&graphTables); err != nil || graphTables != 0 {
		t.Fatalf("KG schema mutation count = %d, %v; want no tables", graphTables, err)
	}
}

func TestKGResetSemanticIndexRollsBackAtomically(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddEdge("a", "b", "rel", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_nodes SET semantic_indexed_at='2026-10-20 10:00:00'; UPDATE kg_edges SET semantic_indexed_at='2026-10-20 10:00:00';
		CREATE TRIGGER fail_edge_reset BEFORE UPDATE OF semantic_indexed_at ON kg_edges BEGIN SELECT RAISE(ABORT,'edge reset fault'); END`); err != nil {
		t.Fatal(err)
	}
	if err := kg.ResetSemanticIndex(); err == nil {
		t.Fatal("ResetSemanticIndex succeeded despite injected edge update failure")
	}
	var nodesPending, edgesPending int
	if err := kg.db.QueryRow(`SELECT count(*) FROM kg_nodes WHERE semantic_indexed_at IS NULL`).Scan(&nodesPending); err != nil {
		t.Fatal(err)
	}
	if err := kg.db.QueryRow(`SELECT count(*) FROM kg_edges WHERE semantic_indexed_at IS NULL`).Scan(&edgesPending); err != nil {
		t.Fatal(err)
	}
	if nodesPending != 0 || edgesPending != 0 {
		t.Fatalf("partial reset: pending nodes=%d edges=%d", nodesPending, edgesPending)
	}
}

func TestKGReindexKeepsSameSecondEdgeMutationDirty(t *testing.T) {
	kg := newTestKG(t)
	var mutate atomic.Bool
	embed := func(_ context.Context, _ string) ([]float32, error) {
		if mutate.CompareAndSwap(true, false) {
			if _, err := kg.db.Exec(`UPDATE kg_edges SET properties=? WHERE source=? AND target=? AND relation=?`, `{"notes":"changed during embedding"}`, "a", "b", "rel"); err != nil {
				return nil, err
			}
		}
		return []float32{1, 0}, nil
	}
	if err := kg.enableSemanticSearchWithCollection(chromem.NewDB(), embed, nil); err != nil {
		t.Fatal(err)
	}
	waitForSemanticReindexIdle(t, kg)
	if err := kg.AddEdge("a", "b", "rel", map[string]string{"notes": "old"}); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_edges SET semantic_indexed_at=NULL,updated_at='2026-10-20 14:30:00' WHERE source='a' AND target='b' AND relation='rel'`); err != nil {
		t.Fatal(err)
	}
	mutate.Store(true)
	if err := kg.RunSemanticReindex(); err != nil {
		t.Fatal(err)
	}
	var pending bool
	if err := kg.db.QueryRow(`SELECT semantic_indexed_at IS NULL FROM kg_edges WHERE source='a' AND target='b' AND relation='rel'`).Scan(&pending); err != nil || !pending {
		t.Fatalf("same-second edge mutation pending=%v err=%v; want dirty marker retained", pending, err)
	}
}

func TestKGReindexKeepsStatusMutationDirty(t *testing.T) {
	kg := newTestKG(t)
	var mutate atomic.Bool
	embed := func(_ context.Context, _ string) ([]float32, error) {
		if mutate.CompareAndSwap(true, false) {
			if _, err := kg.db.Exec(`UPDATE kg_edges SET status='retracted',retracted_at=? WHERE source=? AND target=? AND relation=?`,
				formatKGTimestamp(time.Now()), "a", "b", "rel"); err != nil {
				return nil, err
			}
		}
		return []float32{1, 0}, nil
	}
	if err := kg.enableSemanticSearchWithCollection(chromem.NewDB(), embed, nil); err != nil {
		t.Fatal(err)
	}
	waitForSemanticReindexIdle(t, kg)
	if err := kg.AddEdge("a", "b", "rel", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_edges SET semantic_indexed_at=NULL WHERE source='a' AND target='b' AND relation='rel'`); err != nil {
		t.Fatal(err)
	}
	mutate.Store(true)
	if err := kg.RunSemanticReindex(); err != nil {
		t.Fatal(err)
	}
	var pending bool
	if err := kg.db.QueryRow(`SELECT semantic_indexed_at IS NULL FROM kg_edges WHERE source='a' AND target='b' AND relation='rel'`).Scan(&pending); err != nil || !pending {
		t.Fatalf("status mutation pending=%v err=%v; want semantic index left dirty", pending, err)
	}
}

func TestKGGetNeighborsMatchesAtoiForMalformedCoMentionWeight(t *testing.T) {
	kg := newTestKG(t)
	for _, edge := range []Edge{
		{Source: "hub", Target: "malformed", Relation: "co_mentioned_with", Properties: map[string]string{"source": "transcript", "weight": "2x"}},
		{Source: "hub", Target: "valid", Relation: "co_mentioned_with", Properties: map[string]string{"source": "transcript", "weight": "2"}},
	} {
		if err := kg.AddEdge(edge.Source, edge.Target, edge.Relation, edge.Properties); err != nil {
			t.Fatal(err)
		}
	}
	nodes, _ := kg.GetNeighbors("hub", 10)
	for _, node := range nodes {
		if node.ID == "malformed" {
			t.Fatal("default neighbors exposed a malformed weight that strconv.Atoi treats as zero")
		}
	}
	if len(nodes) != 1 || nodes[0].ID != "valid" {
		t.Fatalf("default co-mention neighbors = %#v; want only the valid weight-2 neighbor", nodes)
	}
	nodes, _ = kg.GetNeighborsWithOptions("hub", 10, KnowledgeGraphQueryOptions{IncludeLowConfidence: true})
	if len(nodes) != 2 {
		t.Fatalf("IncludeLowConfidence neighbors = %#v; want both neighbors", nodes)
	}
}

func TestKGGetNeighborsMatchesGoWhitespaceTrimForCoMentions(t *testing.T) {
	kg := newTestKG(t)
	for _, id := range []string{"hub", "tab_weight", "unicode_manual", "padded_pending", "padded_relation", "other_relation"} {
		if err := kg.AddNode(id, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	edges := []struct {
		source, target, relation string
		properties               map[string]string
	}{
		{source: "hub", target: "tab_weight", relation: "co_mentioned_with", properties: map[string]string{"source": "transcript", "weight": "\t2\n"}},
		{source: "hub", target: "unicode_manual", relation: "co_mentioned_with", properties: map[string]string{"source": "\u00a0manual\t", "weight": "1"}},
		{source: "hub", target: "padded_pending", relation: "co_mentioned_with", properties: map[string]string{"source": " \tpending\u00a0", "weight": "15"}},
		{source: "hub", target: "padded_relation", relation: "\u00a0co_mentioned_with\r", properties: map[string]string{"source": "activity_turn", "weight": "1"}},
		{source: "hub", target: "other_relation", relation: "\trelated_to\n", properties: map[string]string{"source": "activity_turn", "weight": "1"}},
	}
	for _, edge := range edges {
		properties, err := json.Marshal(edge.properties)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := kg.db.Exec(`INSERT INTO kg_edges(source,target,relation,properties) VALUES(?,?,?,?)`, edge.source, edge.target, edge.relation, string(properties)); err != nil {
			t.Fatalf("insert legacy edge %q: %v", edge.target, err)
		}
	}

	nodes, _ := kg.GetNeighbors("hub", 10)
	visible := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		visible[node.ID] = true
	}
	for _, id := range []string{"tab_weight", "unicode_manual", "other_relation"} {
		if !visible[id] {
			t.Errorf("default neighbors hid %q; Go strings.TrimSpace policy keeps it visible", id)
		}
	}
	for _, id := range []string{"padded_pending", "padded_relation"} {
		if visible[id] {
			t.Errorf("default neighbors exposed %q; Go strings.TrimSpace policy hides it", id)
		}
	}
	allNodes, _ := kg.GetNeighborsWithOptions("hub", 10, KnowledgeGraphQueryOptions{IncludeLowConfidence: true})
	if len(allNodes) != len(edges) {
		t.Fatalf("IncludeLowConfidence neighbors=%d, want all %d legacy edges", len(allNodes), len(edges))
	}
}

func TestKGCurrentVersionMissingRequiredTableFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-table.db")
	kg, err := NewKnowledgeGraph(path, "", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if err := kg.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE kg_edges`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := NewKnowledgeGraph(path, "", slog.Default()); err == nil || !strings.Contains(err.Error(), "required table kg_edges is missing") {
		t.Fatalf("NewKnowledgeGraph error = %v; want fail-closed missing-table error", err)
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var exists int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='kg_edges'`).Scan(&exists); err != nil || exists != 0 {
		t.Fatalf("missing kg_edges recreated=%d err=%v; want no schema mutation", exists, err)
	}
}

func TestKGNeighborsBoundsDistinctNodesAndKeepsParallelEdges(t *testing.T) {
	kg := newTestKG(t)
	const candidates = 48
	for i := 0; i < candidates; i++ {
		neighbor := fmt.Sprintf("neighbor-%02d", i)
		if err := kg.AddEdge("hub", neighbor, "rel", nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, relation := range []string{"parallel-a", "parallel-b", "parallel-c"} {
		if err := kg.AddEdge("hub", "neighbor-00", relation, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := kg.db.Exec(`UPDATE kg_edges SET updated_at='2026-10-20 14:30:00' WHERE target='neighbor-00'; UPDATE kg_edges SET updated_at='2025-10-20 14:30:00' WHERE target!='neighbor-00'`); err != nil {
		t.Fatal(err)
	}
	nodes, edges := kg.GetNeighbors("hub", 5)
	if len(nodes) != 5 {
		t.Fatalf("neighbor nodes = %d, want bounded limit 5", len(nodes))
	}
	if nodes[0].ID != "neighbor-00" {
		t.Fatalf("most recent neighbor = %q, want neighbor-00", nodes[0].ID)
	}
	if len(edges) != 8 {
		t.Fatalf("qualifying incident edges = %d, want all 4 edges for neighbor-00 and one for each other selected neighbor", len(edges))
	}
}

func TestKGMergeReindexesEveryDirtyEdgeBeyondOldImportantEdgeLimit(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddNode("target", "Target", nil); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("source", "Source", nil); err != nil {
		t.Fatal(err)
	}
	const edgeCount = 520
	tx, err := kg.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < edgeCount; i++ {
		neighbor := fmt.Sprintf("node-%03d", i)
		if _, err := tx.Exec(`INSERT INTO kg_nodes(id,label,properties) VALUES(?,?, '{}')`, neighbor, neighbor); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`INSERT INTO kg_edges(source,target,relation,properties,updated_at,semantic_indexed_at) VALUES(?,?,?,'{}',?,?)`,
			"source", neighbor, fmt.Sprintf("rel-%03d", i), formatKGTimestamp(time.Now()), formatKGTimestamp(time.Now())); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := kg.enableSemanticSearchWithCollection(chromem.NewDB(), func(context.Context, string) ([]float32, error) { return []float32{1, 0}, nil }, nil); err != nil {
		t.Fatal(err)
	}
	waitForSemanticReindexIdle(t, kg)
	if err := kg.MergeNodes("target", "source"); err != nil {
		t.Fatal(err)
	}
	var pending int
	if err := kg.db.QueryRow(`SELECT count(*) FROM kg_edges WHERE source='target' AND (semantic_indexed_at IS NULL OR semantic_indexed_at<updated_at)`).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if pending != 0 {
		t.Fatalf("pending merged edges = %d; want all %d reindexed in the bounded batch", pending, edgeCount)
	}
	if _, err := kg.semanticIndex().Collection.GetByID(context.Background(), "edge://target\x00node-519\x00rel-519"); err != nil {
		t.Fatalf("last merged edge missing from semantic index: %v", err)
	}
}

func TestKGAddEdgeWithProvenanceDoesNotReindexExistingEndpoints(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddNode("source", "Existing Source", nil); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("target", "Existing Target", nil); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var texts []string
	if err := kg.enableSemanticSearchWithCollection(chromem.NewDB(), func(_ context.Context, text string) ([]float32, error) {
		mu.Lock()
		texts = append(texts, text)
		mu.Unlock()
		return []float32{1, 0}, nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	waitForSemanticReindexIdle(t, kg)
	mu.Lock()
	texts = nil
	mu.Unlock()
	if _, err := kg.AddEdgeWithProvenance("source", "target", "uses", nil, KGProvenanceInput{SourceKind: "user"}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	for _, text := range texts {
		if strings.Contains(text, "Existing Source") || strings.Contains(text, "Existing Target") {
			mu.Unlock()
			t.Fatalf("existing endpoint was re-embedded: %q", text)
		}
	}
	texts = nil
	mu.Unlock()
	if _, err := kg.AddEdgeWithProvenance("source", "new-endpoint", "uses", nil, KGProvenanceInput{SourceKind: "user"}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	indexedNewPlaceholder := false
	for _, text := range texts {
		if strings.Contains(text, "new-endpoint") {
			indexedNewPlaceholder = true
		}
		if strings.Contains(text, "Existing Source") {
			t.Fatalf("existing source endpoint was re-embedded while creating a new endpoint: %q", text)
		}
	}
	if !indexedNewPlaceholder {
		t.Fatalf("new placeholder was not embedded; observed texts=%q", texts)
	}
}

func TestKGLexicalQuotedEmptyTermDoesNotMatchEverything(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddNode("alpha", "Alpha", nil); err != nil {
		t.Fatal(err)
	}
	result, err := kg.SearchResultWithOptions(`""`, KnowledgeGraphQueryOptions{})
	if err != nil {
		t.Fatalf("SearchResultWithOptions quoted-empty: %v", err)
	}
	if len(result.Nodes) != 0 || len(result.Edges) != 0 {
		t.Fatalf("quoted-empty search returned nodes=%v edges=%v", result.Nodes, result.Edges)
	}
	pattern, args := lexicalLikeScore(`""`, "id", "label")
	if pattern != "0" || len(args) != 0 {
		t.Fatalf("quoted-empty LIKE score = %q args=%v; want constant zero without wildcard parameters", pattern, args)
	}
}

func TestKGExploreKeepsLIKEResultsAndReportsFTSFailure(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddNode("alice", "Alice Smith", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`DROP TABLE kg_nodes_fts`); err != nil {
		t.Fatal(err)
	}
	var first struct {
		Nodes  []Node   `json:"nodes"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(kg.Explore("Alice")), &first); err != nil {
		t.Fatal(err)
	}
	if len(first.Nodes) == 0 || first.Nodes[0].ID != "alice" || len(first.Errors) == 0 {
		t.Fatalf("Explore partial result = %+v; want LIKE node plus named FTS error", first)
	}
	var exists int
	if err := kg.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='kg_nodes_fts'`).Scan(&exists); err != nil || exists != 1 {
		t.Fatalf("FTS repair table count=%d err=%v; want repaired index", exists, err)
	}
}

func TestKGFTSIntegrityCheckRepairsStaleExternalContentIndex(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddNode("integrity", "FTS Integrity Sentinel", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`INSERT INTO kg_nodes_fts(kg_nodes_fts,rowid,id,label,properties)
		SELECT 'delete',rowid,id,label,properties FROM kg_nodes WHERE id='integrity'`); err != nil {
		t.Fatalf("remove node from FTS index: %v", err)
	}
	before, err := kg.SearchResultWithOptions("Sentinel", KnowledgeGraphQueryOptions{})
	if err != nil || len(before.Nodes) != 1 {
		t.Fatalf("LIKE fallback before repair nodes=%v err=%v", before.Nodes, err)
	}
	kg.repairKGFTSAfterQueryFailure(fmt.Errorf("test FTS failure signal"))
	after, err := kg.SearchResultWithOptions("Sentinel", KnowledgeGraphQueryOptions{})
	if err != nil || len(after.Nodes) != 1 || after.Nodes[0].ID != "integrity" {
		t.Fatalf("search after integrity repair nodes=%v err=%v", after.Nodes, err)
	}
}

type sqliteCodeTestError int

func (e sqliteCodeTestError) Error() string { return fmt.Sprintf("sqlite test code %d", e) }
func (e sqliteCodeTestError) Code() int     { return int(e) }

func TestKGFTSCorruptionPredicateRequiresSQLiteCorruptCode(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
		want bool
	}{
		{name: "corrupt", code: 11, want: true},
		{name: "corrupt virtual table", code: 11 | (3 << 8), want: true},
		{name: "io error", code: 10},
		{name: "full disk", code: 13},
		{name: "read only", code: 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSQLiteCorruptionError(sqliteCodeTestError(tc.code)); got != tc.want {
				t.Fatalf("isSQLiteCorruptionError(%d) = %v, want %v", tc.code, got, tc.want)
			}
		})
	}
	if isSQLiteCorruptionError(fmt.Errorf("untyped FTS query failure")) {
		t.Fatal("untyped query failure was treated as SQLite corruption")
	}
}

func TestKGFTSRepairRecognizesMissingShadowArtifact(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddNode("missing-shadow", "Missing FTS Shadow", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`DROP TABLE kg_nodes_fts_data`); err != nil {
		t.Fatalf("drop FTS shadow table: %v", err)
	}
	kg.repairKGFTSAfterQueryFailure(fmt.Errorf("FTS query could not read a missing shadow table"))
	var exists bool
	if err := kg.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='kg_nodes_fts_data')`).Scan(&exists); err != nil || !exists {
		t.Fatalf("repaired FTS shadow table exists=%v err=%v", exists, err)
	}
}

func TestKGExclusiveSourceDeletionReportsOnlyCommittedMutations(t *testing.T) {
	kg := newTestKG(t)
	for _, id := range []string{"source", "target"} {
		if err := kg.AddNode(id, id, map[string]string{"source": "chat", "session_id": "cleanup-fault"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := kg.db.Exec(`INSERT INTO kg_edges(source,target,relation,properties) VALUES('source','target','rel',?)`, `{"source":"chat","session_id":"cleanup-fault"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`INSERT INTO kg_evidence(id,raw_text) VALUES('orphan-evidence','sensitive fixture')`); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`CREATE TRIGGER fail_orphan_evidence_cleanup BEFORE DELETE ON kg_evidence BEGIN SELECT RAISE(ABORT,'injected orphan cleanup failure'); END`); err != nil {
		t.Fatal(err)
	}

	deleted, _, err := kg.deleteExclusiveGraphSource(context.Background(), "session_id", "cleanup-fault")
	if err == nil || !strings.Contains(err.Error(), "injected orphan cleanup failure") {
		t.Fatalf("deleteExclusiveGraphSource error=%v; want injected cleanup failure", err)
	}
	if deleted != 0 {
		t.Fatalf("reported deleted=%d after rolled-back cleanup; want zero committed deletions", deleted)
	}
	var edgeCount, evidenceCount int
	if err := kg.db.QueryRow(`SELECT count(*) FROM kg_edges WHERE source='source' AND target='target' AND relation='rel'`).Scan(&edgeCount); err != nil {
		t.Fatal(err)
	}
	if err := kg.db.QueryRow(`SELECT count(*) FROM kg_evidence WHERE id='orphan-evidence'`).Scan(&evidenceCount); err != nil {
		t.Fatal(err)
	}
	if edgeCount != 1 || evidenceCount != 1 {
		t.Fatalf("rollback left edge=%d evidence=%d; want both original rows", edgeCount, evidenceCount)
	}
}

func TestKGFTSTriggerIgnoresUnindexedColumnUpdates(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddNode("a", "Alpha", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_nodes SET semantic_indexed_at='2026-10-20 10:00:00' WHERE id='a'`); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`DROP TABLE kg_nodes_fts`); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_nodes SET access_count=access_count+1 WHERE id='a'`); err != nil {
		t.Fatalf("non-indexed node update fired FTS trigger: %v", err)
	}
	var pending bool
	if err := kg.db.QueryRow(`SELECT semantic_indexed_at IS NULL FROM kg_nodes WHERE id='a'`).Scan(&pending); err != nil || pending {
		t.Fatalf("non-indexed node update dirtied semantic index: pending=%v err=%v", pending, err)
	}
}

func TestKGCorrectionReceiptHelpersUseExactIDs(t *testing.T) {
	kg := newTestKG(t)
	if _, err := kg.db.Exec(`INSERT INTO kg_correction_receipts(id,after_json) VALUES ('corr:1','{"ok":true}'),('corr:10','{"ok":false}')`); err != nil {
		t.Fatal(err)
	}
	receipts, err := kg.correctionReceipts(context.Background(), []string{"corr:1", "corr:1", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if string(receipts["corr:1"]) != `{"ok":true}` || len(receipts) != 1 {
		t.Fatalf("exact correction receipts = %s", receipts)
	}
	if err := kg.deleteCorrectionReceipts(context.Background(), []string{"corr:1", "corr:1", "missing"}); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := kg.db.QueryRow(`SELECT count(*) FROM kg_correction_receipts`).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatalf("remaining receipts=%d err=%v; want the non-matching corr:10 row", remaining, err)
	}
}

func TestKnowledgeGraphExportJSONLDBatchesClaimsWithPerEdgeOrderingAndFilters(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddEdge("a", "b", "knows", nil); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddEdge("b", "c", "uses", nil); err != nil {
		t.Fatal(err)
	}
	tx, err := kg.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`DELETE FROM kg_claims`); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 120; i++ {
		status := string(KGClaimAccepted)
		if i == 119 {
			status = string(KGClaimRetracted)
		}
		if _, err := tx.Exec(`INSERT INTO kg_claims(id,subject_id,predicate,object_id,learned_at,status)
			VALUES(?,?,?,?,?,?)`, fmt.Sprintf("claim-%03d", i), "a", "knows", "b", base.Add(time.Duration(i)*time.Second), status); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec(`INSERT INTO kg_claims(id,subject_id,predicate,object_id,learned_at,status) VALUES('other-edge','b','uses','c',?,'accepted')`, base.Add(121*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	active, err := kg.ExportJSONLD(false, 20)
	if err != nil {
		t.Fatal(err)
	}
	activeRels := collectJSONLDRelationships(t, active)
	if len(activeRels) != 2 {
		t.Fatalf("active relationships = %d, want 2", len(activeRels))
	}
	activeClaims := activeRels[0].Claims
	if activeRels[0].Relation != "knows" {
		activeClaims = activeRels[1].Claims
	}
	if len(activeClaims) != 100 {
		t.Fatalf("active claim count = %d, want per-edge cap 100", len(activeClaims))
	}
	if activeClaims[0].ID != "claim-118" || activeClaims[99].ID != "claim-019" {
		t.Fatalf("active claim window = %s..%s, want newest accepted 118..019", activeClaims[0].ID, activeClaims[99].ID)
	}
	for _, claim := range activeClaims {
		if claim.Status != KGClaimAccepted {
			t.Fatalf("inactive claim leaked into active export: %+v", claim)
		}
	}

	inactive, err := kg.ExportJSONLD(true, 20)
	if err != nil {
		t.Fatal(err)
	}
	inactiveRels := collectJSONLDRelationships(t, inactive)
	var inactiveClaims []KGClaim
	for _, relation := range inactiveRels {
		if relation.Relation == "knows" {
			inactiveClaims = relation.Claims
		}
	}
	if len(inactiveClaims) != 100 || inactiveClaims[0].ID != "claim-119" || inactiveClaims[99].ID != "claim-020" {
		t.Fatalf("include-inactive claim window = %d %v..%v, want newest overall 119..020", len(inactiveClaims), claimIDAt(inactiveClaims, 0), claimIDAt(inactiveClaims, len(inactiveClaims)-1))
	}
	if inactiveClaims[0].Status != KGClaimRetracted {
		t.Fatalf("include-inactive export status = %q, want retracted newest claim", inactiveClaims[0].Status)
	}
}

func claimIDAt(claims []KGClaim, index int) string {
	if index < 0 || index >= len(claims) {
		return ""
	}
	return claims[index].ID
}
