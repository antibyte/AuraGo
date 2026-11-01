package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const maxMemoryEligibilityIDs = 300

// MemoryEligibility is the latest SQLite eligibility snapshot for one memory
// ID. Missing legacy metadata remains eligible after its vector lookup succeeds.
type MemoryEligibility struct {
	DocID              string
	MetadataExists     bool
	Metadata           MemoryMeta
	Archived           bool
	Expired            bool
	Protected          bool
	KeepForever        bool
	PublicationPending bool
	identityExists     bool
	provenanceExists   bool
	token              *memoryEligibilityToken
}

// MemoryEligibilityCandidate adds the immutable corpus token used to avoid
// fencing a different document that later reuses a raw ID.
type MemoryEligibilityCandidate struct {
	DocID         string
	PublicationID string
	ContentHash   string
}

type memoryEligibilityToken struct {
	store         *SQLiteMemory
	docID         string
	publicationID string
	contentHash   string
}

// Visible reports whether a successfully looked-up vector may be returned to
// a normal memory reader.
func (e MemoryEligibility) Visible() bool {
	return !e.Archived && !e.Expired && !e.PublicationPending
}

// CheckMemoryEligibility reads current archive, expiry, metadata, and
// publication state in bounded batches. Database errors are returned so callers
// fail closed. Raw IDs are preserved exactly; empty IDs are invalid.
func (s *SQLiteMemory) CheckMemoryEligibility(ctx context.Context, docIDs []string) (map[string]MemoryEligibility, error) {
	candidates := make([]MemoryEligibilityCandidate, 0, len(docIDs))
	for _, id := range docIDs {
		candidates = append(candidates, MemoryEligibilityCandidate{DocID: id})
	}
	return s.CheckMemoryDocumentEligibility(ctx, candidates)
}

// CheckMemoryDocumentEligibility includes the content hash and corpus
// publication token when comparing IDs with pending publication journals.
func (s *SQLiteMemory) CheckMemoryDocumentEligibility(ctx context.Context, candidates []MemoryEligibilityCandidate) (map[string]MemoryEligibility, error) {
	return s.checkMemoryDocumentEligibilityWithStats(ctx, candidates, nil)
}

type memoryEligibilityQueryStats struct {
	metadataQueries int
	fenceQueries    int
	corpusQueries   int
}

type memoryPublicationCorpusState struct {
	publicationID string
	pending       bool
	blockedReason string
}

type memoryPublicationCorpusEligibility interface {
	ReadAutomaticMemoryPublicationCorpusStateContext(context.Context, []string) (map[string]memoryPublicationCorpusState, error)
}

func (s *SQLiteMemory) checkMemoryDocumentEligibilityWithStats(ctx context.Context, candidates []MemoryEligibilityCandidate, stats *memoryEligibilityQueryStats) (map[string]MemoryEligibility, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("memory eligibility store is unavailable")
	}
	unique := make([]string, 0, len(candidates))
	expectations := make(map[string]MemoryEligibilityCandidate, len(candidates))
	states := make(map[string]MemoryEligibility, len(candidates))
	for _, candidate := range candidates {
		if candidate.DocID == "" {
			return nil, fmt.Errorf("memory result has an empty document ID")
		}
		if previous, ok := expectations[candidate.DocID]; ok {
			if previous != candidate {
				expectations[candidate.DocID] = MemoryEligibilityCandidate{DocID: candidate.DocID}
			}
			continue
		}
		expectations[candidate.DocID] = candidate
		unique = append(unique, candidate.DocID)
		states[candidate.DocID] = MemoryEligibility{DocID: candidate.DocID}
	}
	for start := 0; start < len(unique); start += maxMemoryEligibilityIDs {
		end := min(start+maxMemoryEligibilityIDs, len(unique))
		chunk := unique[start:end]
		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return nil, fmt.Errorf("begin memory eligibility read: %w", err)
		}
		if stats != nil {
			stats.metadataQueries++
		}
		if err := readMemoryEligibilityChunk(ctx, tx, chunk, states); err != nil {
			tx.Rollback()
			return nil, err
		}
		if stats != nil {
			stats.fenceQueries++
		}
		if err := markMemoryPublicationFences(ctx, tx, states, expectations); err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("finish memory eligibility read: %w", err)
		}
		boundVector := s.boundArchiveVector()
		hasCorpus := true
		if availability, ok := boundVector.(canonicalMemoryCorpusAvailability); ok {
			hasCorpus = availability.hasCanonicalMemoryCorpus()
		}
		if corpus, ok := boundVector.(memoryPublicationCorpusEligibility); ok && hasCorpus {
			if stats != nil {
				stats.corpusQueries++
			}
			corpusStates, err := corpus.ReadAutomaticMemoryPublicationCorpusStateContext(ctx, chunk)
			if err != nil {
				return nil, fmt.Errorf("read pending memory publication corpus rows: %w", err)
			}
			for id, corpusState := range corpusStates {
				state := states[id]
				expected := expectations[id]
				if (corpusState.pending && (corpusState.blockedReason == automaticMemoryPublicationPending || corpusState.publicationID != "")) ||
					(corpusState.publicationID != "" && (!state.MetadataExists || !state.identityExists || !state.provenanceExists)) ||
					(expected.PublicationID != "" && corpusState.publicationID != "" && expected.PublicationID != corpusState.publicationID) {
					state.PublicationPending = true
					state.token = nil
				}
				states[id] = state
			}
		}
	}
	for id, state := range states {
		if state.Visible() {
			expected := expectations[id]
			state.token = &memoryEligibilityToken{store: s, docID: id, publicationID: expected.PublicationID, contentHash: expected.ContentHash}
		}
		states[id] = state
	}
	return states, nil
}

func readMemoryEligibilityChunk(ctx context.Context, tx *sql.Tx, ids []string, states map[string]MemoryEligibility) error {
	values := make([]string, len(ids))
	for i := range ids {
		values[i] = "(?)"
	}
	query := `WITH candidates(doc_id) AS (VALUES ` + strings.Join(values, ",") + `)
		SELECT c.doc_id,m.doc_id IS NOT NULL,
		COALESCE(m.doc_id,''),COALESCE(m.access_count,0),COALESCE(m.last_accessed,''),COALESCE(m.last_event_at,''),
		COALESCE(m.extraction_confidence,0),COALESCE(m.verification_status,''),COALESCE(m.source_type,''),COALESCE(m.source_reliability,0),
		COALESCE(m.useful_count,0),COALESCE(m.useless_count,0),COALESCE(m.last_effectiveness_at,''),COALESCE(m.protected,0),COALESCE(m.keep_forever,0),
		COALESCE(m.archived_at,''),COALESCE(m.archived_reason,''),COALESCE(m.last_reviewed_at,''),COALESCE(m.review_note,''),
		COALESCE((SELECT MAX(id) FROM memory_curation_events WHERE memory_curation_events.doc_id=m.doc_id AND dry_run=0),0),
		CASE WHEN e.expires_at<=CURRENT_TIMESTAMP AND COALESCE(m.protected,0)=0 AND COALESCE(m.keep_forever,0)=0 THEN 1 ELSE 0 END,
		EXISTS(SELECT 1 FROM memory_document_identities i WHERE i.doc_id=c.doc_id AND i.identity<>''),
		EXISTS(SELECT 1 FROM memory_extraction_sources s WHERE s.doc_id=c.doc_id)
		FROM candidates c LEFT JOIN memory_meta m ON m.doc_id=c.doc_id LEFT JOIN memory_expiry e ON e.doc_id=c.doc_id`
	rows, err := tx.QueryContext(ctx, query, stringsToAny(ids)...)
	if err != nil {
		return fmt.Errorf("read memory eligibility metadata: %w", err)
	}
	for rows.Next() {
		var id string
		var state MemoryEligibility
		var archivedAt string
		if err := rows.Scan(&id, &state.MetadataExists,
			&state.Metadata.DocID, &state.Metadata.AccessCount, &state.Metadata.LastAccessed, &state.Metadata.LastEventAt,
			&state.Metadata.ExtractionConfidence, &state.Metadata.VerificationStatus, &state.Metadata.SourceType, &state.Metadata.SourceReliability,
			&state.Metadata.UsefulCount, &state.Metadata.UselessCount, &state.Metadata.LastEffectivenessAt, &state.Metadata.Protected, &state.Metadata.KeepForever,
			&state.Metadata.ArchivedAt, &state.Metadata.ArchivedReason, &state.Metadata.LastReviewedAt, &state.Metadata.ReviewNote,
			&state.Metadata.CurationRevision, &state.Expired, &state.identityExists, &state.provenanceExists); err != nil {
			rows.Close()
			return fmt.Errorf("scan memory eligibility metadata: %w", err)
		}
		state.DocID = id
		state.Protected = state.Metadata.Protected
		state.KeepForever = state.Metadata.KeepForever
		state.Archived = IsMemoryArchived(state.Metadata)
		states[id] = state
		_ = archivedAt
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return fmt.Errorf("finish memory eligibility metadata: %w", err)
	}
	return nil
}

type memoryEligibilityQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func markMemoryPublicationFences(ctx context.Context, db memoryEligibilityQuerier, states map[string]MemoryEligibility, expectations map[string]MemoryEligibilityCandidate) error {
	rows, err := db.QueryContext(ctx, `SELECT key,value FROM memory_maintenance_meta WHERE key LIKE ?`, automaticMemoryPublicationPrefix+"%")
	if err != nil {
		return fmt.Errorf("read memory publication fences: %w", err)
	}
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			rows.Close()
			return fmt.Errorf("scan memory publication fence: %w", err)
		}
		var journal automaticMemoryPublicationJournal
		if err := json.Unmarshal([]byte(raw), &journal); err != nil || journal.Version != 1 || (journal.Status != automaticMemoryPublicationStaged && journal.Status != automaticMemoryPublicationReady) {
			rows.Close()
			if err == nil {
				err = fmt.Errorf("unsupported journal version or state")
			}
			return fmt.Errorf("decode memory publication fence: %w", err)
		}
		for _, document := range journal.Documents {
			state, ok := states[document.ID]
			if !ok {
				continue
			}
			expected := expectations[document.ID]
			if expected.PublicationID != "" && (expected.PublicationID != key || expected.ContentHash != document.Hash) {
				continue
			}
			state.PublicationPending = true
			state.token = nil
			states[document.ID] = state
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return fmt.Errorf("finish memory publication fence lookup: %w", err)
	}
	return nil
}

// ReadEligibleMemoriesByID rechecks a group of tokens before and after raw
// lookups. Database work stays batched; an individual missing vector is omitted.
func (s *SQLiteMemory) ReadEligibleMemoriesByID(ctx context.Context, vector VectorDB, eligibilities []MemoryEligibility) (map[string]string, error) {
	if s == nil || vector == nil {
		return nil, fmt.Errorf("memory reader is unavailable")
	}
	if len(eligibilities) == 0 {
		return map[string]string{}, nil
	}
	tokens := make([]*memoryEligibilityToken, 0, len(eligibilities))
	seen := make(map[string]*memoryEligibilityToken, len(eligibilities))
	for _, eligibility := range eligibilities {
		token := eligibility.token
		if token == nil || token.store != s || token.docID == "" || token.docID != eligibility.DocID {
			return nil, fmt.Errorf("memory read eligibility token is invalid")
		}
		if previous, ok := seen[token.docID]; ok {
			if previous.publicationID != token.publicationID || previous.contentHash != token.contentHash {
				return nil, fmt.Errorf("memory read eligibility tokens disagree for %s", token.docID)
			}
			continue
		}
		seen[token.docID] = token
		tokens = append(tokens, token)
	}
	if len(tokens) == 0 {
		return map[string]string{}, nil
	}
	current, err := s.checkMemoryEligibilityTokens(ctx, tokens)
	if err != nil {
		return nil, err
	}
	contentByID := make(map[string]string, len(tokens))
	for _, token := range tokens {
		if !current[token.docID].Visible() {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		content, err := vector.GetByID(token.docID)
		if err != nil {
			continue
		}
		if token.contentHash != "" && contentSHA256(content) != token.contentHash {
			continue
		}
		contentByID[token.docID] = content
	}
	verified := make([]*memoryEligibilityToken, 0, len(contentByID))
	for _, token := range tokens {
		if _, ok := contentByID[token.docID]; ok {
			verified = append(verified, token)
		}
	}
	if len(verified) == 0 {
		return contentByID, nil
	}
	current, err = s.checkMemoryEligibilityTokens(ctx, verified)
	if err != nil {
		return nil, err
	}
	for _, token := range verified {
		if !current[token.docID].Visible() {
			delete(contentByID, token.docID)
		}
	}
	return contentByID, nil
}

func (s *SQLiteMemory) checkMemoryEligibilityTokens(ctx context.Context, tokens []*memoryEligibilityToken) (map[string]MemoryEligibility, error) {
	candidates := make([]MemoryEligibilityCandidate, 0, len(tokens))
	for _, token := range tokens {
		candidates = append(candidates, MemoryEligibilityCandidate{DocID: token.docID, PublicationID: token.publicationID, ContentHash: token.contentHash})
	}
	return s.CheckMemoryDocumentEligibility(ctx, candidates)
}

// ReadEligibleMemoryByID is the single-item compatibility helper. GetByID
// remains available for explicit maintenance paths.
func (s *SQLiteMemory) ReadEligibleMemoryByID(ctx context.Context, vector VectorDB, eligibility MemoryEligibility) (string, error) {
	content, err := s.ReadEligibleMemoriesByID(ctx, vector, []MemoryEligibility{eligibility})
	if err != nil {
		return "", err
	}
	body, ok := content[eligibility.DocID]
	if !ok {
		return "", fmt.Errorf("memory %s is unavailable or no longer visible", eligibility.DocID)
	}
	return body, nil
}
