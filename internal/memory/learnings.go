package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"aurago/internal/security"
)

// LearningScope is resolved by the caller from an authorized project, never a tool path.
type LearningScope struct {
	Project  string `json:"project,omitempty"`
	Resource string `json:"resource,omitempty"`
}

func (s LearningScope) Allows(other LearningScope) bool {
	return (other.Project == "" || other.Project == s.Project) && (other.Resource == "" || other.Resource == s.Resource)
}

// LearningCondition is an exact, structured constraint. Prose alone never blocks a tool.
type LearningCondition struct {
	Target string `json:"target"`
	Field  string `json:"field"`
	Value  string `json:"value"`
}

type LearningDraft struct {
	Statement    string             `json:"statement"`
	Rationale    string             `json:"rationale"`
	Alternatives []string           `json:"alternatives,omitempty"`
	Kind         string             `json:"kind"`
	Confidence   float64            `json:"confidence"`
	Condition    *LearningCondition `json:"condition,omitempty"`
}

type LearningSource struct {
	Reference  string  `json:"reference"`
	Session    string  `json:"session"`
	Kind       string  `json:"kind"`
	Quote      string  `json:"quote"`
	Hash       string  `json:"hash"`
	Confidence float64 `json:"confidence"`
}

type Learning struct {
	LearningDraft
	ID           string           `json:"id"`
	Scope        LearningScope    `json:"scope"`
	Sources      []LearningSource `json:"sources"`
	Status       string           `json:"status"`
	Revision     int              `json:"revision"`
	Protected    bool             `json:"protected"`
	Fingerprint  string           `json:"fingerprint,omitempty"`
	SupersededBy string           `json:"superseded_by,omitempty"`
	PreviousID   string           `json:"previous_id,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

type LearningRevision struct {
	Learning Learning `json:"learning"`
	Reason   string   `json:"reason"`
	Actor    string   `json:"actor"`
}

func (s *SQLiteMemory) initLearnings(backup bool) error {
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='memory_learnings'`).Scan(&exists); err != nil {
		return fmt.Errorf("inspect learning schema: %w", err)
	}
	if exists == 0 && backup {
		if _, err := backupMainDatabase(s.db, "learnings-v1"); err != nil {
			return fmt.Errorf("back up learning migration: %w", err)
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS memory_learnings (
		id TEXT PRIMARY KEY, project TEXT NOT NULL, resource TEXT NOT NULL, status TEXT NOT NULL,
		revision INTEGER NOT NULL, payload TEXT NOT NULL);
		CREATE INDEX IF NOT EXISTS idx_learning_scope ON memory_learnings(project,resource,status);
		CREATE TABLE IF NOT EXISTS memory_learning_revisions (
		id TEXT NOT NULL, revision INTEGER NOT NULL, payload TEXT NOT NULL, PRIMARY KEY(id,revision));
		CREATE TABLE IF NOT EXISTS memory_learning_metrics (name TEXT PRIMARY KEY, value INTEGER NOT NULL);
		CREATE TABLE IF NOT EXISTS memory_learning_deviations (run TEXT NOT NULL, action_hash TEXT NOT NULL, reason TEXT NOT NULL, revisions TEXT NOT NULL, created_at TEXT NOT NULL, PRIMARY KEY(run,action_hash));
		INSERT INTO memory_schema_meta(key,value) VALUES('learnings','1') ON CONFLICT(key) DO NOTHING;`)
	if err != nil {
		return fmt.Errorf("create learning schema: %w", err)
	}
	return tx.Commit()
}

func validateLearning(d LearningDraft, scope LearningScope) error {
	if strings.TrimSpace(d.Statement) == "" || len(d.Statement) > 2000 || len(d.Rationale) > 2000 || len(d.Alternatives) > 4 || len(scope.Project) > 200 || len(scope.Resource) > 1000 || (scope.Resource != "" && scope.Project == "") {
		return fmt.Errorf("invalid learning content or scope")
	}
	if d.Kind != "decision" && d.Kind != "preference" && d.Kind != "technical" {
		return fmt.Errorf("invalid learning kind")
	}
	if math.IsNaN(d.Confidence) || math.IsInf(d.Confidence, 0) || d.Confidence < 0 || d.Confidence > 1 {
		return fmt.Errorf("invalid learning confidence")
	}
	for _, a := range d.Alternatives {
		if len(a) > 500 {
			return fmt.Errorf("learning alternative too long")
		}
	}
	if c := d.Condition; c != nil && (c.Target == "" || c.Field == "" || len(c.Target) > 1000 || len(c.Field) > 200 || len(c.Value) > 1000) {
		return fmt.Errorf("invalid learning condition")
	}
	return nil
}

func learningHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

func readLearning(ctx context.Context, q correctionQueryer, id string) (Learning, error) {
	var b string
	var l Learning
	if err := q.QueryRowContext(ctx, `SELECT payload FROM memory_learnings WHERE id=?`, id).Scan(&b); err != nil {
		return l, err
	}
	err := json.Unmarshal([]byte(b), &l)
	return l, err
}

func saveLearning(ctx context.Context, tx *sql.Tx, l Learning, reason, actor string) error {
	values := []string{l.Statement, l.Rationale, l.Scope.Project, l.Scope.Resource, reason, actor}
	values = append(values, l.Alternatives...)
	if l.Condition != nil {
		values = append(values, l.Condition.Target, l.Condition.Field, l.Condition.Value)
	}
	for _, source := range l.Sources {
		values = append(values, source.Reference, source.Session, source.Quote)
	}
	for _, value := range values {
		if security.Scrub(value) != value {
			return fmt.Errorf("learning contains sensitive content")
		}
	}
	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO memory_learnings(id,project,resource,status,revision,payload) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET status=excluded.status,revision=excluded.revision,payload=excluded.payload`, l.ID, l.Scope.Project, l.Scope.Resource, l.Status, l.Revision, string(b))
	if err != nil {
		return err
	}
	b, err = json.Marshal(LearningRevision{Learning: l, Reason: reason, Actor: actor})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO memory_learning_revisions(id,revision,payload) VALUES(?,?,?)`, l.ID, l.Revision, string(b))
	return err
}

// ObserveLearning accepts only evidence already checked against a runtime-owned source.
// Its wire adapters must not decode source provenance, scope or activation from model JSON.
func (s *SQLiteMemory) ObserveLearning(ctx context.Context, d LearningDraft, scope LearningScope, source LearningSource, fingerprint string) (Learning, error) {
	source.Confidence = d.Confidence
	if err := validateLearning(d, scope); err != nil {
		return Learning{}, err
	}
	if source.Reference == "" || source.Session == "" || source.Quote == "" || len(source.Quote) > 4000 || source.Hash != learningHash(source.Quote) || (source.Kind != "user" && source.Kind != "technical") {
		return Learning{}, fmt.Errorf("learning requires verified runtime evidence")
	}
	if d.Kind != "technical" && source.Kind != "user" {
		return Learning{}, fmt.Errorf("preferences and decisions require human evidence")
	}
	// A rejected alternative must be present in the actual evidence, not invented by extraction.
	for _, a := range d.Alternatives {
		if !strings.Contains(source.Quote, a) {
			return Learning{}, fmt.Errorf("unsupported rejected alternative")
		}
	}
	s.correctionMu.Lock()
	defer s.correctionMu.Unlock()
	if err := rejectDeletedChatSessionEvidence(ctx, s.db, source.Session); err != nil {
		return Learning{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Learning{}, err
	}
	defer tx.Rollback()
	if err := rejectDeletedChatSessionEvidence(ctx, tx, source.Session); err != nil {
		return Learning{}, err
	}
	identity, _ := json.Marshal([]any{scope, correctionIdentity(d.Statement), d.Condition})
	id := "learning_" + learningHash(string(identity))[:24]
	l, err := readLearning(ctx, tx, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return l, err
	}
	if err == nil {
		if l.Status == "archived" || l.Status == "superseded" || l.Status == "revalidation" || l.Protected {
			return l, nil
		}
		// Evidence for a different claim must not authorize the stored rationale or kind.
		if l.Statement != d.Statement || l.Rationale != d.Rationale || l.Kind != d.Kind || !slices.Equal(l.Alternatives, d.Alternatives) || l.Fingerprint != fingerprint {
			return l, fmt.Errorf("learning differs from the stored draft; review or correct it explicitly")
		}
		for _, e := range l.Sources {
			if e.Session == source.Session && e.Hash == source.Hash {
				return l, nil
			}
		}
		if len(l.Sources) >= 64 {
			return l, nil
		}
	} else {
		l = Learning{LearningDraft: d, ID: id, Scope: scope, Status: "proposed", CreatedAt: time.Now().UTC(), Fingerprint: fingerprint}
	}
	l.Sources = append(l.Sources, source)
	sessions := map[string]bool{}
	for _, e := range l.Sources {
		if e.Kind == source.Kind && e.Confidence >= .95 {
			sessions[e.Session] = true
		}
	}
	if d.Confidence >= 0.95 && (d.Kind != "preference" || len(sessions) >= 2) {
		conflict, err := learningHasConflict(ctx, tx, l)
		if err != nil {
			return l, err
		}
		if !conflict {
			l.Status = "active"
		}
	}
	l.Revision++
	l.UpdatedAt = time.Now().UTC()
	if err := saveLearning(ctx, tx, l, "runtime evidence", "analysis"); err != nil {
		return l, err
	}
	return l, tx.Commit()
}

func learningHasConflict(ctx context.Context, tx *sql.Tx, candidate Learning) (bool, error) {
	if candidate.Condition == nil {
		return false, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT payload FROM memory_learnings WHERE project=? AND resource=? AND status='active' AND id<>?`, candidate.Scope.Project, candidate.Scope.Resource, candidate.ID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var b string
		var l Learning
		if err := rows.Scan(&b); err != nil {
			return false, err
		}
		if err := json.Unmarshal([]byte(b), &l); err != nil {
			return false, err
		}
		if c := l.Condition; c != nil && c.Target == candidate.Condition.Target && c.Field == candidate.Condition.Field && c.Value != candidate.Condition.Value {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *SQLiteMemory) Learning(ctx context.Context, id string, scope LearningScope) (Learning, error) {
	l, err := readLearning(ctx, s.db, id)
	if err == nil && !scope.Allows(l.Scope) {
		return Learning{}, sql.ErrNoRows
	}
	return l, err
}

// ListLearnings filters authorization in SQL before relevance scoring. Empty scope sees global only.
func (s *SQLiteMemory) ListLearnings(ctx context.Context, scope LearningScope, query, status string, limit int) ([]Learning, error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM memory_learnings WHERE (project='' OR project=?) AND (resource='' OR resource=?) AND (?='' OR status=?) ORDER BY revision DESC,id LIMIT 5000`, scope.Project, scope.Resource, status, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type hit struct {
		l     Learning
		score int
	}
	hits := []hit{}
	terms := learningTerms(query)
	for rows.Next() {
		var b string
		var l Learning
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(b), &l); err != nil {
			return nil, err
		}
		words := learningTerms(l.Statement + " " + l.Rationale)
		score := 0
		for term := range terms {
			if words[term] {
				score++
			}
		}
		if len(terms) > 0 && score == 0 && !(scope.Resource != "" && l.Scope.Resource == scope.Resource) {
			continue
		}
		if l.Scope.Project != "" {
			score += 10
		}
		if l.Scope.Resource != "" {
			score += 100
		}
		hits = append(hits, hit{l, score})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].l.UpdatedAt.After(hits[j].l.UpdatedAt)
	})
	out := make([]Learning, 0, min(limit, len(hits)))
	for _, h := range hits[:min(limit, len(hits))] {
		out = append(out, h.l)
	}
	return out, nil
}

func learningTerms(text string) map[string]bool {
	terms := map[string]bool{}
	for _, term := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len(term) < 2 {
			continue
		}
		switch term {
		case "the", "and", "for", "with", "this", "that", "keep", "use", "please", "file", "behalte", "und", "mit", "für", "die", "der", "das", "auf", "eine", "einer", "einen", "nicht", "bitte", "datei":
			continue
		}
		terms[term] = true
	}
	return terms
}

// ReviewLearning is an explicit authorized human action with optimistic concurrency.
func (s *SQLiteMemory) ReviewLearning(ctx context.Context, id string, revision int, scope LearningScope, action, statement, reason string) (Learning, error) {
	s.correctionMu.Lock()
	defer s.correctionMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Learning{}, err
	}
	defer tx.Rollback()
	l, err := readLearning(ctx, tx, id)
	if err != nil {
		return l, err
	}
	if !scope.Allows(l.Scope) {
		return Learning{}, sql.ErrNoRows
	}
	if l.Revision != revision {
		return l, fmt.Errorf("learning revision changed")
	}
	if strings.TrimSpace(reason) == "" || len(reason) > 2000 {
		return l, fmt.Errorf("review requires a reason")
	}
	if l.Protected && action != "unprotect" {
		return l, fmt.Errorf("unprotect learning before changing it")
	}
	switch action {
	case "confirm":
		if l.Status == "archived" || l.Status == "superseded" {
			return l, fmt.Errorf("archived learning cannot be reactivated")
		}
		conflict, err := learningHasConflict(ctx, tx, l)
		if err != nil {
			return l, err
		}
		if conflict {
			return l, fmt.Errorf("resolve the active conflicting learning first")
		}
		l.Status = "active"
	case "archive":
		l.Status = "archived"
	case "replace":
		if strings.TrimSpace(statement) == "" || statement == l.Statement || len(statement) > 2000 {
			return l, fmt.Errorf("replacement must be different and bounded")
		}
		if l.Protected || l.Status == "archived" || l.Status == "superseded" {
			return l, fmt.Errorf("learning is protected or retired")
		}
		l, err = supersedeLearning(ctx, tx, l, statement, reason, "user")
		if err != nil {
			return l, err
		}
		return l, tx.Commit()
	case "protect":
		l.Protected = true
	case "unprotect":
		l.Protected = false
	default:
		return l, fmt.Errorf("unsupported learning action")
	}
	l.Revision++
	l.UpdatedAt = time.Now().UTC()
	if err := saveLearning(ctx, tx, l, reason, "user"); err != nil {
		return l, err
	}
	return l, tx.Commit()
}

func supersedeLearning(ctx context.Context, tx *sql.Tx, old Learning, statement, reason, actor string) (Learning, error) {
	next := old
	next.ID = "learning_" + learningHash(old.ID + ":" + fmt.Sprint(old.Revision) + ":" + statement)[:24]
	next.PreviousID, next.SupersededBy = old.ID, ""
	next.Statement, next.Rationale, next.Condition = statement, reason, nil
	next.Alternatives = nil
	next.Status, next.Revision, next.Confidence = "active", 1, 1
	next.CreatedAt, next.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	next.Sources = []LearningSource{{Reference: actor, Session: actor, Kind: "user", Quote: statement, Hash: learningHash(statement)}}
	old.Status, old.SupersededBy = "superseded", next.ID
	old.Revision++
	old.UpdatedAt = next.UpdatedAt
	if err := saveLearning(ctx, tx, old, reason, actor); err != nil {
		return Learning{}, err
	}
	if err := saveLearning(ctx, tx, next, reason, actor); err != nil {
		return Learning{}, err
	}
	return next, nil
}

func (s *SQLiteMemory) LearningHistory(ctx context.Context, id string, scope LearningScope) ([]LearningRevision, error) {
	if _, err := s.Learning(ctx, id, scope); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM memory_learning_revisions WHERE id=? ORDER BY revision DESC LIMIT 100`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LearningRevision{}
	for rows.Next() {
		var b string
		var v LearningRevision
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(b), &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *SQLiteMemory) RevalidateLearningResource(ctx context.Context, scope LearningScope, fingerprint string) error {
	if scope.Project == "" || scope.Resource == "" || fingerprint == "" {
		return nil
	}
	s.correctionMu.Lock()
	defer s.correctionMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Page by stable identity, independently of retrieval limits and changing revisions.
	for cursor := ""; ; {
		rows, err := tx.QueryContext(ctx, `SELECT id,payload FROM memory_learnings WHERE project=? AND resource=? AND status IN ('active','revalidation') AND id>? ORDER BY id LIMIT 200`, scope.Project, scope.Resource, cursor)
		if err != nil {
			return fmt.Errorf("read resource learnings: %w", err)
		}
		batch := make([]Learning, 0, 200)
		for rows.Next() {
			var payload string
			var l Learning
			if err = rows.Scan(&cursor, &payload); err == nil {
				err = json.Unmarshal([]byte(payload), &l)
			}
			if err != nil {
				rows.Close()
				return fmt.Errorf("decode resource learning: %w", err)
			}
			batch = append(batch, l)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return fmt.Errorf("read resource learnings: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		for _, l := range batch {
			if l.Fingerprint == "" || l.Fingerprint == fingerprint {
				continue
			}
			l.Status = "revalidation"
			l.Fingerprint = fingerprint
			l.Revision++
			l.UpdatedAt = time.Now().UTC()
			if err := saveLearning(ctx, tx, l, "resource fingerprint changed", "runtime"); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO memory_learning_metrics(name,value) VALUES('revalidations',1) ON CONFLICT(name) DO UPDATE SET value=value+1`); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *SQLiteMemory) RecordLearningMetric(ctx context.Context, name string, n int) error {
	switch name {
	case "retrievals", "discarded", "context_tokens", "revalidations", "conflicts", "overrides":
	default:
		return fmt.Errorf("unknown learning metric")
	}
	if n < 0 {
		return fmt.Errorf("invalid learning metric")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO memory_learning_metrics(name,value) VALUES(?,?) ON CONFLICT(name) DO UPDATE SET value=value+excluded.value`, name, n)
	return err
}

func (s *SQLiteMemory) LearningMetrics(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name,value FROM memory_learning_metrics`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var name string
		var n int
		if err := rows.Scan(&name, &n); err != nil {
			return nil, err
		}
		out[name] = n
	}
	return out, rows.Err()
}
