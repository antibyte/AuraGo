package memory

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/security"
)

func TestKGOptimizeGraphConcurrentSafe(t *testing.T) {
	kg := newTestKG(t)

	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("temp_%02d", i)
		if err := kg.AddNode(id, id, map[string]string{"type": "concept"}); err != nil {
			t.Fatalf("AddNode %s: %v", id, err)
		}
	}
	if err := kg.AddNode("keeper", "Keeper", map[string]string{"protected": "true"}); err != nil {
		t.Fatalf("AddNode keeper: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := kg.OptimizeGraph(3); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent OptimizeGraph: %v", err)
	}

	keeper, err := kg.GetNode("keeper")
	if err != nil || keeper == nil {
		t.Fatal("expected protected keeper node to survive concurrent optimize")
	}

	var orphanEdges int
	if err := kg.db.QueryRow(`
		SELECT COUNT(*) FROM kg_edges e
		WHERE NOT EXISTS (SELECT 1 FROM kg_nodes n WHERE n.id = e.source)
		   OR NOT EXISTS (SELECT 1 FROM kg_nodes n WHERE n.id = e.target)
	`).Scan(&orphanEdges); err != nil {
		t.Fatalf("count orphan edges: %v", err)
	}
	if orphanEdges != 0 {
		t.Fatalf("expected no orphan edges after concurrent optimize, got %d", orphanEdges)
	}
}

func TestKGCleanupStaleGraphFlushesAccessBeforeRemoval(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("recently_used", "Recently Used", map[string]string{"type": "device", "source": "manual"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if _, err := kg.db.Exec(`
		UPDATE kg_nodes
		SET updated_at = datetime('now', '-60 days'), access_count = 0
		WHERE id = 'recently_used'
	`); err != nil {
		t.Fatalf("age node: %v", err)
	}
	kg.enqueueAccessHit(knowledgeGraphAccessHit{nodeID: "recently_used"})

	_, nodesRemoved, err := kg.CleanupStaleGraph(30)
	if err != nil {
		t.Fatalf("CleanupStaleGraph: %v", err)
	}
	if nodesRemoved > 0 {
		t.Fatalf("expected flushed access hit to protect node, removed %d nodes", nodesRemoved)
	}

	node, err := kg.GetNode("recently_used")
	if err != nil || node == nil {
		t.Fatal("expected node to survive cleanup after access flush")
	}
	var accessCount int
	if err := kg.db.QueryRow("SELECT access_count FROM kg_nodes WHERE id = 'recently_used'").Scan(&accessCount); err != nil {
		t.Fatalf("query access_count: %v", err)
	}
	if accessCount < 1 {
		t.Fatalf("access_count = %d, want >= 1 after cleanup flush", accessCount)
	}
}

func TestKGSuggestRelationsRespectsBranchLimit(t *testing.T) {
	kg := newTestKG(t)

	for i := 0; i < 80; i++ {
		id := fmt.Sprintf("device_%02d", i)
		if err := kg.AddNode(id, "Device "+id, map[string]string{
			"type":   "device",
			"source": "inventory",
		}); err != nil {
			t.Fatalf("AddNode %s: %v", id, err)
		}
		if _, err := kg.db.Exec("UPDATE kg_nodes SET access_count = 1 WHERE id = ?", id); err != nil {
			t.Fatalf("bump access_count: %v", err)
		}
	}

	start := time.Now()
	result := kg.SuggestRelations(5)
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("SuggestRelations took too long (%s) with large graph", elapsed)
	}
	if result == "" || result == "[]" {
		t.Fatal("expected bounded suggestions for qualified device nodes")
	}

	var suggestions []map[string]string
	if err := json.Unmarshal([]byte(result), &suggestions); err != nil {
		t.Fatalf("unmarshal suggestions: %v", err)
	}
	if len(suggestions) > 5 {
		t.Fatalf("got %d suggestions, want <= 5", len(suggestions))
	}
}

func TestKGGetSubgraphToleratesConcurrentWrites(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("center", "Center", map[string]string{"type": "service"}); err != nil {
		t.Fatalf("AddNode center: %v", err)
	}
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("leaf_%d", i)
		if err := kg.AddNode(id, id, nil); err != nil {
			t.Fatalf("AddNode %s: %v", id, err)
		}
		if err := kg.AddEdge("center", id, "connects_to", nil); err != nil {
			t.Fatalf("AddEdge: %v", err)
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 40; i++ {
			id := fmt.Sprintf("dynamic_%02d", i)
			_ = kg.AddNode(id, id, nil)
			_ = kg.AddEdge("center", id, "connects_to", nil)
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nodes, edges := kg.GetSubgraph("center", 2)
			if len(nodes) == 0 {
				t.Error("expected subgraph nodes during concurrent writes")
			}
			_ = edges
		}()
	}
	wg.Wait()
	<-done
}

func TestKGSearchForContextMasksSensitiveProperties(t *testing.T) {
	kg := newTestKG(t)
	secret := "vault-super-secret-42"
	security.RegisterSensitive(secret)

	if err := kg.AddNode("db_primary", "Primary Database", map[string]string{
		"type":     "service",
		"password": secret,
		"host":     "db.local",
	}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	ctx := kg.SearchForContext("Primary Database", 5, 800)
	if ctx == "" {
		t.Fatal("expected SearchForContext output")
	}
	if strings.Contains(ctx, secret) {
		t.Fatalf("expected sensitive property value to be masked, got %q", ctx)
	}
	if strings.Contains(ctx, "password:") || strings.Contains(ctx, "| password:") {
		t.Fatalf("expected password property to be omitted, got %q", ctx)
	}
	if !strings.Contains(ctx, "db.local") {
		t.Fatalf("expected non-sensitive host property to remain, got %q", ctx)
	}
}

func TestMergeKnowledgeGraphLabelsUnifiedStrategies(t *testing.T) {
	if got := mergeKnowledgeGraphLabel("NAS", "Network Storage"); got != "NAS" {
		t.Fatalf("curated merge should keep existing label, got %q", got)
	}
	if got := choosePreferredAutoExtractedLabel("Pi", "Raspberry Pi 4"); got != "Raspberry Pi 4" {
		t.Fatalf("auto-extracted merge should prefer longer label, got %q", got)
	}
	if got := mergeKnowledgeGraphLabels("Unknown", "NAS", false); got != "NAS" {
		t.Fatalf("unknown replacement = %q, want NAS", got)
	}
}

func TestIsSensitiveKnowledgeGraphPropertyKey(t *testing.T) {
	for _, key := range []string{"password", "api_key", "db_password", "oauth_token"} {
		if !isSensitiveKnowledgeGraphPropertyKey(key) {
			t.Fatalf("expected %q to be sensitive", key)
		}
	}
	if isSensitiveKnowledgeGraphPropertyKey("hostname") {
		t.Fatal("hostname should not be treated as sensitive")
	}
}