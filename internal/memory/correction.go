package memory

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MemoryReference identifies a fact, never a fuzzy search or an entire graph.
type MemoryReference struct {
	Store    string `json:"store"`
	ID       string `json:"id,omitempty"`
	Field    string `json:"field,omitempty"`
	Category string `json:"category,omitempty"`
	Key      string `json:"key,omitempty"`
	Target   string `json:"target,omitempty"`
	Relation string `json:"relation,omitempty"`
}

type MemoryCorrectionChange struct {
	Reference MemoryReference `json:"reference"`
	Operation string          `json:"operation"`
	Value     string          `json:"value,omitempty"`
}

// CorrectionEvidence is constructed from runtime-owned evidence, not tool arguments.
type CorrectionEvidence struct {
	Reference   string `json:"reference"`
	Kind        string `json:"kind"`
	Quote       string `json:"quote"`
	ContentHash string `json:"content_hash"`
}

type MemoryCorrectionItem struct {
	MemoryCorrectionChange
	Before        string          `json:"before"`
	Snapshot      json.RawMessage `json:"snapshot"`
	After         json.RawMessage `json:"after,omitempty"`
	ReplacementID string          `json:"replacement_id,omitempty"`
	State         string          `json:"state"`
	Error         string          `json:"error,omitempty"`
}

type MemoryCorrection struct {
	ID            string                 `json:"id"`
	SessionID     string                 `json:"session_id"`
	RunID         string                 `json:"run_id"`
	Reason        string                 `json:"reason"`
	Evidence      []CorrectionEvidence   `json:"evidence"`
	Items         []MemoryCorrectionItem `json:"items"`
	Review        []MemoryReference      `json:"review_required,omitempty"`
	ReviewLimited bool                   `json:"review_limited,omitempty"`
	Status        string                 `json:"status"`
	CreatedAt     time.Time              `json:"created_at"`
}

type CorrectionAccess struct {
	MemoryWrite    bool
	GraphWrite     bool
	ProfileEnabled bool
}

type MemoryCorrectionService struct {
	Memory *SQLiteMemory
	Vector VectorDB
	Graph  *KnowledgeGraph
	Access CorrectionAccess
}

const maxCorrectionItems = 32

func correctionReferenceKey(ref MemoryReference) string {
	b, _ := json.Marshal(ref)
	return string(b)
}

func correctionIdentity(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

// CorrectionFactText removes only the storage envelope, not arbitrary brackets.
func CorrectionFactText(text string) string {
	if concept, _, ok := AnalysisDocumentParts(text); ok {
		if _, fact, found := strings.Cut(concept, "]"); found {
			return strings.TrimSpace(fact)
		}
		return strings.TrimSpace(concept)
	}
	if _, body, ok := strings.Cut(text, "\n\n"); ok {
		return strings.TrimSpace(body)
	}
	return strings.TrimSpace(text)
}

func correctionStoredFact(text string, meta MemoryMeta) string {
	// Correction vectors contain the replacement itself, without a concept envelope.
	if meta.SourceType == "memory_correction" || strings.HasPrefix(meta.SourceType, "memory_correction:") {
		return text
	}
	return CorrectionFactText(text)
}

func correctionTechnicalSource(source string) bool {
	switch strings.TrimPrefix(source, "memory_correction:") {
	case "system_metrics", "remote_control":
		return true
	default:
		return false
	}
}

func correctionReplacementSource(meta *MemoryMeta) string {
	if meta != nil && correctionTechnicalSource(meta.SourceType) {
		return "memory_correction:" + strings.TrimPrefix(meta.SourceType, "memory_correction:")
	}
	return "memory_correction"
}

func (s *SQLiteMemory) initMemoryCorrections() error {
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='memory_corrections'`).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		var populated int
		if err := s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM core_memory)+(SELECT COUNT(*) FROM memory_meta)+(SELECT COUNT(*) FROM user_profile)`).Scan(&populated); err != nil {
			return err
		}
		if populated > 0 {
			if _, err := backupMainDatabase(s.db, "memory-corrections-v1"); err != nil {
				return err
			}
		}
	}
	_, err := s.db.Exec(`
	CREATE TABLE IF NOT EXISTS memory_corrections (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, payload TEXT NOT NULL);
	CREATE TABLE IF NOT EXISTS memory_correction_guards (identity TEXT PRIMARY KEY, correction_id TEXT NOT NULL);
	CREATE TABLE IF NOT EXISTS memory_profile_corrections (category TEXT NOT NULL, key TEXT NOT NULL, correction_id TEXT NOT NULL, PRIMARY KEY(category,key));
	CREATE TABLE IF NOT EXISTS memory_correction_tombstones (reference TEXT PRIMARY KEY, correction_id TEXT NOT NULL, previous_value TEXT NOT NULL);
	CREATE TRIGGER IF NOT EXISTS core_correction_insert BEFORE INSERT ON core_memory
	WHEN EXISTS(SELECT 1 FROM memory_correction_guards WHERE identity=NEW.normalized_fact)
	BEGIN SELECT RAISE(ABORT,'fact was corrected; use a new evidenced correction'); END;
	CREATE TRIGGER IF NOT EXISTS core_correction_update BEFORE UPDATE OF fact ON core_memory
	WHEN EXISTS(SELECT 1 FROM memory_correction_guards WHERE identity=NEW.normalized_fact)
	BEGIN SELECT RAISE(ABORT,'fact was corrected; use a new evidenced correction'); END;`)
	return err
}

func (s *SQLiteMemory) MemoryCorrection(id, sessionID string) (*MemoryCorrection, error) {
	var payload string
	if err := s.db.QueryRow(`SELECT payload FROM memory_corrections WHERE id=? AND session_id=?`, id, sessionID).Scan(&payload); err != nil {
		return nil, fmt.Errorf("read memory correction: %w", err)
	}
	var correction MemoryCorrection
	if err := json.Unmarshal([]byte(payload), &correction); err != nil {
		return nil, fmt.Errorf("decode memory correction: %w", err)
	}
	return &correction, nil
}

type correctionExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func saveMemoryCorrection(ctx context.Context, db correctionExecer, correction *MemoryCorrection) error {
	b, err := json.Marshal(correction)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO memory_corrections(id,session_id,payload) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload`, correction.ID, correction.SessionID, string(b))
	return err
}

func (s *SQLiteMemory) IsCorrectedMemoryFact(text string) (bool, error) {
	var exists int
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM memory_correction_guards WHERE identity=?)`, correctionIdentity(text)).Scan(&exists)
	return exists != 0, err
}

func (c *MemoryCorrectionService) checkAccess(ref MemoryReference) error {
	if c.Memory == nil || !c.Access.MemoryWrite {
		return fmt.Errorf("memory corrections require enabled writable memory")
	}
	switch ref.Store {
	case "core", "ltm":
		if ref != (MemoryReference{Store: ref.Store, ID: ref.ID}) || ref.ID == "" {
			return fmt.Errorf("invalid memory reference fields")
		}
	case "profile":
		if ref != (MemoryReference{Store: "profile", Category: ref.Category, Key: ref.Key}) || !isValidProfileCategory(ref.Category) || !isValidProfileKey(ref.Key) {
			return fmt.Errorf("invalid profile reference")
		}
		if !c.Access.ProfileEnabled {
			return fmt.Errorf("user profiling is disabled")
		}
	case "kg_node", "kg_edge":
		if ref.Store == "kg_node" && ref != (MemoryReference{Store: ref.Store, ID: ref.ID, Field: ref.Field}) {
			return fmt.Errorf("invalid graph property reference")
		}
		if ref.Store == "kg_edge" && ref != (MemoryReference{Store: ref.Store, ID: ref.ID, Target: ref.Target, Relation: ref.Relation}) {
			return fmt.Errorf("invalid graph edge reference")
		}
		if c.Graph == nil || !c.Access.GraphWrite {
			return fmt.Errorf("knowledge graph corrections require an enabled writable graph")
		}
	default:
		return fmt.Errorf("unsupported correction store")
	}
	return nil
}

func newCorrection(sessionID, runID, reason string, evidence []CorrectionEvidence) (*MemoryCorrection, error) {
	if sessionID == "" || runID == "" || strings.TrimSpace(reason) == "" || len(reason) > 1000 || len(evidence) == 0 {
		return nil, fmt.Errorf("correction requires a session, run, reason and runtime evidence")
	}
	for _, e := range evidence {
		if e.Reference == "" || e.Quote == "" || e.ContentHash == "" || (e.Kind != "user" && e.Kind != "tool") {
			return nil, fmt.Errorf("invalid runtime correction evidence")
		}
	}
	return &MemoryCorrection{ID: "mc_" + rand.Text(), SessionID: sessionID, RunID: runID, Reason: reason, Evidence: evidence, Status: "preview", CreatedAt: time.Now().UTC()}, nil
}
