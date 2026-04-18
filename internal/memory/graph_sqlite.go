package memory

import (
	"aurago/internal/dbutil"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var validIdentifierRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func validIdentifier(name string) bool {
	return name != "" && validIdentifierRE.MatchString(name)
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

type Node struct {
	ID         string            `json:"id"`
	Label      string            `json:"label"`
	Properties map[string]string `json:"properties"`
	Protected  bool              `json:"protected,omitempty"`
}

type Edge struct {
	Source     string            `json:"source"`
	Target     string            `json:"target"`
	Relation   string            `json:"relation"`
	Properties map[string]string `json:"properties"`
}

type KnowledgeGraphDuplicateCandidate struct {
	Label           string   `json:"label"`
	NormalizedLabel string   `json:"normalized_label"`
	Count           int      `json:"count"`
	IDs             []string `json:"ids"`
}

type KnowledgeGraphQualityReport struct {
	Nodes               int                                `json:"nodes"`
	Edges               int                                `json:"edges"`
	ProtectedNodes      int                                `json:"protected_nodes"`
	IsolatedNodes       int                                `json:"isolated_nodes"`
	UntypedNodes        int                                `json:"untyped_nodes"`
	DuplicateGroups     int                                `json:"duplicate_groups"`
	DuplicateNodes      int                                `json:"duplicate_nodes"`
	IsolatedSample      []Node                             `json:"isolated_sample"`
	UntypedSample       []Node                             `json:"untyped_sample"`
	DuplicateCandidates []KnowledgeGraphDuplicateCandidate `json:"duplicate_candidates"`
}

type KnowledgeGraph struct {
	db          *sql.DB
	logger      *slog.Logger
	accessQueue chan knowledgeGraphAccessHit
	doneChan    chan struct{}
	closeOnce   sync.Once
	wg          sync.WaitGroup
	semantic    *knowledgeGraphSemanticIndex
	droppedHits atomic.Int64 // count of access-queue hits dropped due to full channel
}

const knowledgeGraphWriteTimeout = 5 * time.Second
const knowledgeGraphPropertyValueLimit = 500
const coOccurrenceThreshold = 15

var ErrKnowledgeGraphProtectedNode = errors.New("knowledge graph node is protected")

type knowledgeGraphAccessHit struct {
	nodeID   string
	source   string
	target   string
	relation string
}

type ImportantNode struct {
	Node
	ImportanceScore int `json:"importance_score"`
}

type KnowledgeGraphStats struct {
	TotalNodes      int            `json:"total_nodes"`
	TotalEdges      int            `json:"total_edges"`
	MeaningfulEdges int            `json:"meaningful_edges"`
	CoMentionEdges  int            `json:"co_mention_edges"`
	ByType          map[string]int `json:"by_type"`
	BySource        map[string]int `json:"by_source"`
}

type FileSyncStats struct {
	NodeCount    int            `json:"node_count"`
	EdgeCount    int            `json:"edge_count"`
	ByEntityType map[string]int `json:"by_entity_type"`
	ByCollection map[string]int `json:"by_collection"`
}

type KGCollectionStats struct {
	NodeCount  int        `json:"node_count"`
	EdgeCount  int        `json:"edge_count"`
	FileCount  int        `json:"file_count"`
	LastSyncAt *time.Time `json:"last_sync_at,omitempty"`
}

type kgBFSLevel struct {
	nodeID string
	depth  int
}

func NewKnowledgeGraph(dbPath string, jsonMigratePath string, logger *slog.Logger) (*KnowledgeGraph, error) {
	maxOpenConns := 2
	// SQLite in-memory databases are scoped to a single connection unless callers
	// explicitly opt into a shared-cache DSN. The KG uses an async worker that may
	// read/write on a different pooled connection, so plain ":memory:" must stay on
	// one connection or tests and access counters observe different databases.
	if strings.TrimSpace(dbPath) == ":memory:" {
		maxOpenConns = 1
	}
	db, err := dbutil.Open(dbPath, dbutil.WithMaxOpenConns(maxOpenConns))
	if err != nil {
		return nil, fmt.Errorf("open knowledge graph db: %w", err)
	}

	kg := &KnowledgeGraph{
		db:          db,
		logger:      logger,
		accessQueue: make(chan knowledgeGraphAccessHit, 1000),
		doneChan:    make(chan struct{}),
	}
	kg.wg.Add(1)
	go kg.accessCountWorker()
	if err := kg.initTables(); err != nil {
		close(kg.doneChan)
		kg.drainAccessQueue()
		if closeErr := db.Close(); closeErr != nil {
			logger.Warn("KG db close failed after init error", "error", closeErr)
		}
		return nil, fmt.Errorf("init knowledge graph tables: %w", err)
	}

	if jsonMigratePath != "" {
		kg.migrateFromJSON(jsonMigratePath)
	}

	return kg, nil
}

func (kg *KnowledgeGraph) Close() error {
	var err error
	kg.closeOnce.Do(func() {
		close(kg.doneChan)
		kg.wg.Wait()
		kg.drainAccessQueue()
		if kg.semantic != nil {
			kg.semantic.Close()
		}
		err = kg.db.Close()
	})
	return err
}

func (kg *KnowledgeGraph) drainAccessQueue() {
	for {
		select {
		case hit := <-kg.accessQueue:
			ctx, cancel := context.WithTimeout(context.Background(), knowledgeGraphWriteTimeout)
			var execErr error
			switch {
			case hit.nodeID != "":
				_, execErr = kg.db.ExecContext(ctx, "UPDATE kg_nodes SET access_count = access_count + 1 WHERE id = ?", hit.nodeID)
			case hit.source != "" && hit.target != "" && hit.relation != "":
				_, execErr = kg.db.ExecContext(ctx, "UPDATE kg_edges SET access_count = access_count + 1 WHERE source = ? AND target = ? AND relation = ?", hit.source, hit.target, hit.relation)
			}
			cancel()
			if execErr != nil {
				kg.logger.Warn("KG access count update failed during drain", "hit", hit, "error", execErr)
			}
		default:
			return
		}
	}
}

func (kg *KnowledgeGraph) accessCountWorker() {
	defer kg.wg.Done()

	nodeHits := make(map[string]int)
	type edgeKey struct{ source, target, relation string }
	edgeHits := make(map[edgeKey]int)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	flush := func() {
		if len(nodeHits) == 0 && len(edgeHits) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), knowledgeGraphWriteTimeout)
		defer cancel()

		tx, err := kg.db.BeginTx(ctx, nil)
		if err != nil {
			kg.logger.Warn("KG access count batch begin failed", "error", err)
			nodeHits = make(map[string]int)
			edgeHits = make(map[edgeKey]int)
			return
		}

		if len(nodeHits) > 0 {
			stmt, err := tx.PrepareContext(ctx, "UPDATE kg_nodes SET access_count = access_count + ? WHERE id = ?")
			if err == nil {
				for id, count := range nodeHits {
					if _, execErr := stmt.ExecContext(ctx, count, id); execErr != nil {
						kg.logger.Warn("KG access count update failed for node", "id", id, "count", count, "error", execErr)
					}
				}
				stmt.Close()
			}
		}

		if len(edgeHits) > 0 {
			stmt, err := tx.PrepareContext(ctx, "UPDATE kg_edges SET access_count = access_count + ? WHERE source = ? AND target = ? AND relation = ?")
			if err == nil {
				for e, count := range edgeHits {
					if _, execErr := stmt.ExecContext(ctx, count, e.source, e.target, e.relation); execErr != nil {
						kg.logger.Warn("KG access count update failed for edge", "source", e.source, "target", e.target, "relation", e.relation, "error", execErr)
					}
				}
				stmt.Close()
			}
		}

		if err := tx.Commit(); err != nil {
			kg.logger.Warn("KG access count batch commit failed", "error", err)
		}

		nodeHits = make(map[string]int)
		edgeHits = make(map[edgeKey]int)
	}

	for {
		select {
		case <-kg.doneChan:
			flush()
			return
		case hit, ok := <-kg.accessQueue:
			if !ok {
				flush()
				return
			}
			if hit.nodeID != "" {
				nodeHits[hit.nodeID]++
			} else if hit.source != "" && hit.target != "" && hit.relation != "" {
				edgeHits[edgeKey{hit.source, hit.target, hit.relation}]++
			}

			if len(nodeHits)+len(edgeHits) > 200 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (kg *KnowledgeGraph) Stats() (nodes int, edges int, err error) {
	var nodeErr, edgeErr error
	if nodeErr = kg.db.QueryRow("SELECT COUNT(*) FROM kg_nodes").Scan(&nodes); nodeErr != nil {
		kg.logger.Warn("KG Stats: failed to count nodes", "error", nodeErr)
		nodes = -1
	}
	if edgeErr = kg.db.QueryRow("SELECT COUNT(*) FROM kg_edges").Scan(&edges); edgeErr != nil {
		kg.logger.Warn("KG Stats: failed to count edges", "error", edgeErr)
		edges = -1
	}
	return nodes, edges, errors.Join(nodeErr, edgeErr)
}

func (kg *KnowledgeGraph) ResetSemanticIndex() error {
	if kg == nil || kg.db == nil {
		return fmt.Errorf("knowledge graph not initialized")
	}

	result, err := kg.db.Exec("UPDATE kg_nodes SET semantic_indexed_at = NULL WHERE semantic_indexed_at IS NOT NULL")
	if err != nil {
		return fmt.Errorf("reset kg node semantic_indexed_at: %w", err)
	}
	nodesReset, _ := result.RowsAffected()

	result, err = kg.db.Exec("UPDATE kg_edges SET semantic_indexed_at = NULL WHERE semantic_indexed_at IS NOT NULL")
	if err != nil {
		return fmt.Errorf("reset kg edge semantic_indexed_at: %w", err)
	}
	edgesReset, _ := result.RowsAffected()

	if kg.logger != nil && (nodesReset > 0 || edgesReset > 0) {
		kg.logger.Info("KG semantic index reset — will be rebuilt with new model",
			"nodes_reset", nodesReset, "edges_reset", edgesReset)
	}

	return nil
}

func (kg *KnowledgeGraph) enqueueAccessHit(hit knowledgeGraphAccessHit) {
	select {
	case kg.accessQueue <- hit:
	default:
		kg.droppedHits.Add(1)
		kg.logger.Debug("KG access queue full, dropping update", "hit", hit)
	}
}

// DroppedAccessHits returns the total number of access-counter updates that were
// discarded because the async queue was full. Useful for diagnostics and health checks.
func (kg *KnowledgeGraph) DroppedAccessHits() int64 {
	return kg.droppedHits.Load()
}

func normalizeKnowledgeGraphProperties(properties map[string]string) map[string]string {
	if properties == nil {
		return make(map[string]string)
	}

	safe := make(map[string]string, len(properties))
	for k, v := range properties {
		runes := []rune(v)
		if len(runes) > knowledgeGraphPropertyValueLimit {
			safe[k] = string(runes[:knowledgeGraphPropertyValueLimit-3]) + "..."
			continue
		}
		safe[k] = v
	}
	return safe
}

func sanitizeKnowledgeGraphNodeProperties(properties map[string]string, protected bool) map[string]string {
	safe := normalizeKnowledgeGraphProperties(properties)
	delete(safe, "access_count")
	delete(safe, "protected")
	if protected {
		safe["protected"] = "true"
	}
	return safe
}

func decodeKnowledgeGraphNodeProperties(logger *slog.Logger, scope, nodeID, propsJSON string, protected int) map[string]string {
	properties := make(map[string]string)
	if propsJSON != "" {
		if err := json.Unmarshal([]byte(propsJSON), &properties); err != nil {
			logger.Warn(scope+": corrupt node properties JSON", "id", nodeID, "error", err)
			properties = make(map[string]string)
		}
	}
	return sanitizeKnowledgeGraphNodeProperties(properties, protected != 0)
}

func mergeKnowledgeGraphProperties(existing, incoming map[string]string) map[string]string {
	out := normalizeKnowledgeGraphProperties(existing)
	for key, value := range normalizeKnowledgeGraphProperties(incoming) {
		if strings.TrimSpace(out[key]) == "" && strings.TrimSpace(value) != "" {
			out[key] = value
		}
	}
	return out
}

func mergeKnowledgeGraphPropertiesOverwrite(existing, incoming map[string]string) map[string]string {
	out := normalizeKnowledgeGraphProperties(existing)
	for key, value := range normalizeKnowledgeGraphProperties(incoming) {
		if strings.TrimSpace(value) != "" {
			out[key] = value
		}
	}
	return out
}

func mergeAutoExtractedProperties(existing, incoming map[string]string) map[string]string {
	out := normalizeKnowledgeGraphProperties(existing)
	for key, value := range normalizeKnowledgeGraphProperties(incoming) {
		current := strings.TrimSpace(out[key])
		next := strings.TrimSpace(value)
		switch {
		case current == "":
			out[key] = value
		case next == "":
			continue
		case current == next:
			continue
		case len([]rune(next)) > len([]rune(current)):
			out[key] = value
		}
	}
	return out
}

func mergeKnowledgeGraphLabel(existing, incoming string) string {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	switch {
	case existing == "" || strings.EqualFold(existing, "unknown"):
		if incoming != "" {
			return incoming
		}
	case incoming == "" || strings.EqualFold(incoming, "unknown"):
		return existing
	}
	if existing == "" {
		return incoming
	}
	return existing
}

func knowledgeGraphEdgeKey(source, target, relation string) string {
	return source + "\x00" + target + "\x00" + relation
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
