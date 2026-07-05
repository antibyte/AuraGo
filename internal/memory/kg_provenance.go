package memory

import (
	"aurago/internal/security"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const kgEvidenceRawTextLimit = 2000

type KGClaimStatus string

const (
	KGClaimAccepted   KGClaimStatus = "accepted"
	KGClaimSuperseded KGClaimStatus = "superseded"
	KGClaimRetracted  KGClaimStatus = "retracted"
	KGClaimRejected   KGClaimStatus = "rejected"
)

type KGClaim struct {
	ID              string        `json:"id"`
	SubjectID       string        `json:"subject_id"`
	Predicate       string        `json:"predicate"`
	ObjectID        string        `json:"object_id,omitempty"`
	ObjectLiteral   string        `json:"object_literal,omitempty"`
	AssertedInGraph string        `json:"asserted_in_graph"`
	LearnedAt       string        `json:"learned_at"`
	AcceptedAt      string        `json:"accepted_at,omitempty"`
	Confidence      float64       `json:"confidence"`
	ConfidenceLabel string        `json:"confidence_label,omitempty"`
	SourceKind      string        `json:"source_kind"`
	IngestionRunID  string        `json:"ingestion_run_id,omitempty"`
	Status          KGClaimStatus `json:"status"`
	SupersededBy    string        `json:"superseded_by,omitempty"`
	SourceMessageID string        `json:"source_message_id,omitempty"`
	SessionID       string        `json:"session_id,omitempty"`
	PrivacyClass    string        `json:"privacy_class"`
	RetentionPolicy string        `json:"retention_policy"`
	EvidenceID      string        `json:"evidence_id,omitempty"`
	Evidence        *KGEvidence   `json:"evidence,omitempty"`
}

type KGEvidence struct {
	ID              string `json:"id"`
	EvidenceType    string `json:"evidence_type,omitempty"`
	SourceMessageID string `json:"source_message_id,omitempty"`
	SessionID       string `json:"session_id,omitempty"`
	Channel         string `json:"channel,omitempty"`
	Actor           string `json:"actor,omitempty"`
	RawText         string `json:"raw_text,omitempty"`
	SourceURI       string `json:"source_uri,omitempty"`
	ContentHash     string `json:"content_hash,omitempty"`
	CapturedAt      string `json:"captured_at"`
}

type KGConflict struct {
	ID                int64  `json:"id"`
	SubjectID         string `json:"subject_id"`
	Predicate         string `json:"predicate"`
	LeftClaimID       string `json:"left_claim_id"`
	RightClaimID      string `json:"right_claim_id"`
	WinningClaimID    string `json:"winning_claim_id,omitempty"`
	SupersededClaimID string `json:"superseded_claim_id,omitempty"`
	Reason            string `json:"reason,omitempty"`
	Status            string `json:"status"`
	DetectedAt        string `json:"detected_at"`
	ResolvedAt        string `json:"resolved_at,omitempty"`
}

type KGProvenanceInput struct {
	EvidenceType    string
	SourceMessageID string
	SessionID       string
	Channel         string
	Actor           string
	RawText         string
	SourceURI       string
	AssertedInGraph string
	Confidence      float64
	ConfidenceLabel string
	SourceKind      string
	IngestionRunID  string
	PrivacyClass    string
	RetentionPolicy string
}

func (kg *KnowledgeGraph) AddEdgeWithProvenance(source, target, relation string, properties map[string]string, provenance KGProvenanceInput) (*KGClaim, error) {
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	relation = strings.TrimSpace(relation)
	if source == "" || target == "" || relation == "" {
		return nil, fmt.Errorf("source, target, and relation are required")
	}
	properties = normalizeKnowledgeGraphProperties(properties)
	now := time.Now()

	tx, err := kg.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin add edge with provenance: %w", err)
	}
	defer tx.Rollback()

	for _, id := range []string{source, target} {
		if err := ensureKnowledgeGraphPlaceholderNodeTx(tx, id); err != nil {
			kg.logger.Warn("AddEdgeWithProvenance: failed to ensure node exists", "id", id, "error", err)
		}
	}

	existingProps, found, err := loadKnowledgeGraphEdge(tx, source, target, relation)
	if err != nil {
		return nil, fmt.Errorf("load existing edge for add: %w", err)
	}

	defaultSource := strings.TrimSpace(provenance.SourceKind)
	if defaultSource == "" {
		defaultSource = "system"
	}
	var finalProps map[string]string
	if found {
		finalProps = mergeKnowledgeGraphPropertiesOverwrite(existingProps, properties)
		finalProps = ensureKnowledgeGraphEdgeQualityProperties(finalProps, defaultSource, now)
	} else {
		finalProps = ensureKnowledgeGraphEdgeQualityProperties(properties, defaultSource, now)
	}
	propsJSON, _ := json.Marshal(finalProps)
	if _, err = tx.Exec(`
		INSERT INTO kg_edges (
			source, target, relation, properties, updated_at,
			status, status_reason, superseded_by_claim_id, retracted_at
		)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, ?, '', '', NULL)
		ON CONFLICT(source, target, relation) DO UPDATE SET
			properties = excluded.properties,
			updated_at = CURRENT_TIMESTAMP,
			status = excluded.status,
			status_reason = '',
			superseded_by_claim_id = '',
			retracted_at = NULL
	`, source, target, relation, string(propsJSON), string(KGClaimAccepted)); err != nil {
		return nil, fmt.Errorf("add edge: %w", err)
	}

	evidenceID, err := kg.insertKGEvidenceTx(tx, provenance, now)
	if err != nil {
		return nil, err
	}

	claimID := newKGClaimID(now)
	if _, err := tx.Exec(`
		INSERT INTO kg_claims (
			id, subject_id, predicate, object_id, asserted_in_graph, accepted_at,
			confidence, confidence_label, source_kind, ingestion_run_id, status,
			source_message_id, session_id, privacy_class, retention_policy, evidence_id
		)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, claimID, source, relation, target, defaultString(provenance.AssertedInGraph, "local:worldview"),
		normalizeKGConfidence(provenance.Confidence), strings.TrimSpace(provenance.ConfidenceLabel),
		defaultSource, strings.TrimSpace(provenance.IngestionRunID), string(KGClaimAccepted),
		strings.TrimSpace(provenance.SourceMessageID), strings.TrimSpace(provenance.SessionID),
		defaultString(provenance.PrivacyClass, "normal"), defaultString(provenance.RetentionPolicy, "default"),
		nullableString(evidenceID)); err != nil {
		return nil, fmt.Errorf("insert kg claim: %w", err)
	}
	if err := kg.detectKGConflictsTx(tx, claimID, source, target, relation, finalProps); err != nil {
		return nil, err
	}

	claim, err := getKGClaimByIDTx(tx, claimID)
	if err != nil {
		return nil, fmt.Errorf("load inserted kg claim: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if sourceNode, err := kg.GetNode(source); err == nil && sourceNode != nil {
		kg.indexSemanticNodeAfterWrite(*sourceNode)
	} else if err != nil && kg.logger != nil {
		kg.logger.Warn("AddEdgeWithProvenance: failed to reload source node for semantic index", "id", source, "error", err)
	}
	if targetNode, err := kg.GetNode(target); err == nil && targetNode != nil {
		kg.indexSemanticNodeAfterWrite(*targetNode)
	} else if err != nil && kg.logger != nil {
		kg.logger.Warn("AddEdgeWithProvenance: failed to reload target node for semantic index", "id", target, "error", err)
	}
	kg.indexSemanticEdgeAfterWrite(Edge{Source: source, Target: target, Relation: relation, Properties: finalProps})
	return claim, nil
}

func (kg *KnowledgeGraph) SupersedeEdge(source, target, relation, supersededByClaimID, reason string) error {
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	relation = strings.TrimSpace(relation)
	supersededByClaimID = strings.TrimSpace(supersededByClaimID)
	reason = strings.TrimSpace(reason)
	if source == "" || target == "" || relation == "" {
		return fmt.Errorf("source, target, and relation are required")
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin supersede edge: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		UPDATE kg_edges
		SET status = ?, superseded_by_claim_id = ?, status_reason = ?, updated_at = CURRENT_TIMESTAMP
		WHERE source = ? AND target = ? AND relation = ?
	`, string(KGClaimSuperseded), supersededByClaimID, reason, source, target, relation); err != nil {
		return fmt.Errorf("supersede edge: %w", err)
	}
	if _, err := tx.Exec(`
		UPDATE kg_claims
		SET status = ?, superseded_by = ?
		WHERE subject_id = ? AND object_id = ? AND predicate = ? AND status = ?
	`, string(KGClaimSuperseded), supersededByClaimID, source, target, relation, string(KGClaimAccepted)); err != nil {
		return fmt.Errorf("supersede edge claims: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := kg.removeSemanticEdgeIndex(source, target, relation); err != nil && kg.logger != nil {
		kg.logger.Warn("SupersedeEdge: failed to remove semantic edge index", "source", source, "target", target, "relation", relation, "error", err)
	}
	return nil
}

func (kg *KnowledgeGraph) RetractEdge(source, target, relation, reason string) error {
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	relation = strings.TrimSpace(relation)
	reason = strings.TrimSpace(reason)
	if source == "" || target == "" || relation == "" {
		return fmt.Errorf("source, target, and relation are required")
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin retract edge: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		UPDATE kg_edges
		SET status = ?, status_reason = ?, retracted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE source = ? AND target = ? AND relation = ?
	`, string(KGClaimRetracted), reason, source, target, relation); err != nil {
		return fmt.Errorf("retract edge: %w", err)
	}
	if _, err := tx.Exec(`
		UPDATE kg_claims
		SET status = ?
		WHERE subject_id = ? AND object_id = ? AND predicate = ? AND status = ?
	`, string(KGClaimRetracted), source, target, relation, string(KGClaimAccepted)); err != nil {
		return fmt.Errorf("retract edge claims: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := kg.removeSemanticEdgeIndex(source, target, relation); err != nil && kg.logger != nil {
		kg.logger.Warn("RetractEdge: failed to remove semantic edge index", "source", source, "target", target, "relation", relation, "error", err)
	}
	return nil
}

func (kg *KnowledgeGraph) RegisterKGConflict(subjectID, predicate, leftClaimID, rightClaimID, reason string) error {
	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin kg conflict registration: %w", err)
	}
	defer tx.Rollback()
	if err := registerKGConflictTx(tx, subjectID, predicate, leftClaimID, rightClaimID, reason); err != nil {
		return err
	}
	return tx.Commit()
}

func (kg *KnowledgeGraph) GetOpenKGConflicts(limit int) ([]KGConflict, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := kg.db.Query(`
		SELECT id, subject_id, predicate, left_claim_id, right_claim_id,
		       COALESCE(winning_claim_id, ''), COALESCE(superseded_claim_id, ''),
		       reason, status, COALESCE(detected_at, ''), COALESCE(resolved_at, '')
		FROM kg_conflicts
		WHERE status = 'open'
		ORDER BY detected_at DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query open kg conflicts: %w", err)
	}
	defer rows.Close()
	return scanKGConflictRows(rows)
}

func (kg *KnowledgeGraph) ResolveKGConflict(id int64, winningClaimID, reason string) error {
	winningClaimID = strings.TrimSpace(winningClaimID)
	reason = strings.TrimSpace(reason)
	if id <= 0 || winningClaimID == "" {
		return fmt.Errorf("conflict id and winning claim id are required")
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin kg conflict resolution: %w", err)
	}
	defer tx.Rollback()

	conflict, err := getKGConflictByIDTx(tx, id)
	if err != nil {
		return fmt.Errorf("load kg conflict: %w", err)
	}
	if conflict.Status != "open" {
		return fmt.Errorf("kg conflict %d is not open", id)
	}
	var losingClaimID string
	switch winningClaimID {
	case conflict.LeftClaimID:
		losingClaimID = conflict.RightClaimID
	case conflict.RightClaimID:
		losingClaimID = conflict.LeftClaimID
	default:
		return fmt.Errorf("winning claim %s does not belong to conflict %d", winningClaimID, id)
	}

	winning, err := getKGClaimCoreTx(tx, winningClaimID)
	if err != nil {
		return fmt.Errorf("load winning kg claim: %w", err)
	}
	losing, err := getKGClaimCoreTx(tx, losingClaimID)
	if err != nil {
		return fmt.Errorf("load losing kg claim: %w", err)
	}
	if winning.SubjectID != conflict.SubjectID || winning.Predicate != conflict.Predicate {
		return fmt.Errorf("winning claim %s does not match conflict fact", winningClaimID)
	}

	if _, err := tx.Exec(`
		UPDATE kg_claims
		SET status = ?, superseded_by = ?
		WHERE id = ?
	`, string(KGClaimSuperseded), winningClaimID, losingClaimID); err != nil {
		return fmt.Errorf("supersede losing kg claim: %w", err)
	}
	if _, err := tx.Exec(`
		UPDATE kg_edges
		SET status = ?, superseded_by_claim_id = ?, status_reason = ?, updated_at = CURRENT_TIMESTAMP
		WHERE source = ? AND target = ? AND relation = ?
	`, string(KGClaimSuperseded), winningClaimID, reason, losing.SubjectID, losing.ObjectID, losing.Predicate); err != nil {
		return fmt.Errorf("supersede losing kg edge: %w", err)
	}
	if _, err := tx.Exec(`
		UPDATE kg_conflicts
		SET status = 'resolved', winning_claim_id = ?, superseded_claim_id = ?, reason = ?, resolved_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, winningClaimID, losingClaimID, reason, id); err != nil {
		return fmt.Errorf("resolve kg conflict: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	if err := kg.removeSemanticEdgeIndex(losing.SubjectID, losing.ObjectID, losing.Predicate); err != nil && kg.logger != nil {
		kg.logger.Warn("ResolveKGConflict: failed to remove losing semantic edge index", "source", losing.SubjectID, "target", losing.ObjectID, "relation", losing.Predicate, "error", err)
	}
	return nil
}

func (kg *KnowledgeGraph) GetClaimsForEdge(source, target, relation string, includeInactive bool, limit int) ([]KGClaim, error) {
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	relation = strings.TrimSpace(relation)
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	where := `WHERE c.subject_id = ? AND c.object_id = ? AND c.predicate = ?`
	args := []interface{}{source, target, relation}
	if !includeInactive {
		where += ` AND c.status = ?`
		args = append(args, string(KGClaimAccepted))
	}
	args = append(args, limit)

	rows, err := kg.db.Query(kgClaimsWithEvidenceSQL(where+` ORDER BY c.learned_at DESC, c.id DESC LIMIT ?`), args...)
	if err != nil {
		return nil, fmt.Errorf("query kg claims for edge: %w", err)
	}
	defer rows.Close()
	return scanKGClaimRows(rows)
}

func activeKGEdgePredicate(alias string) string {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return "COALESCE(status, 'accepted') = 'accepted'"
	}
	return fmt.Sprintf("COALESCE(%s.status, 'accepted') = 'accepted'", alias)
}

func (kg *KnowledgeGraph) detectKGConflictsTx(tx *sql.Tx, claimID, subjectID, objectID, predicate string, properties map[string]string) error {
	if !isExclusiveKGPredicate(predicate, properties) {
		return nil
	}
	rows, err := tx.Query(`
		SELECT id
		FROM kg_claims
		WHERE subject_id = ? AND predicate = ? AND status = ? AND id != ?
		  AND COALESCE(object_id, '') != ?
		LIMIT 25
	`, subjectID, predicate, string(KGClaimAccepted), claimID, objectID)
	if err != nil {
		return fmt.Errorf("query exclusive kg claim conflicts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var otherClaimID string
		if err := rows.Scan(&otherClaimID); err != nil {
			return fmt.Errorf("scan exclusive kg claim conflict: %w", err)
		}
		if err := registerKGConflictTx(tx, subjectID, predicate, otherClaimID, claimID, "exclusive predicate has multiple accepted objects"); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate exclusive kg claim conflicts: %w", err)
	}
	return nil
}

func registerKGConflictTx(tx *sql.Tx, subjectID, predicate, leftClaimID, rightClaimID, reason string) error {
	subjectID = strings.TrimSpace(subjectID)
	predicate = strings.TrimSpace(predicate)
	leftClaimID = strings.TrimSpace(leftClaimID)
	rightClaimID = strings.TrimSpace(rightClaimID)
	reason = strings.TrimSpace(reason)
	if subjectID == "" || predicate == "" || leftClaimID == "" || rightClaimID == "" {
		return fmt.Errorf("subject, predicate, and claim ids are required")
	}
	if rightClaimID < leftClaimID {
		leftClaimID, rightClaimID = rightClaimID, leftClaimID
	}
	if _, err := tx.Exec(`
		INSERT OR IGNORE INTO kg_conflicts (subject_id, predicate, left_claim_id, right_claim_id, reason, status)
		VALUES (?, ?, ?, ?, ?, 'open')
	`, subjectID, predicate, leftClaimID, rightClaimID, reason); err != nil {
		return fmt.Errorf("insert kg conflict: %w", err)
	}
	return nil
}

func isExclusiveKGPredicate(predicate string, properties map[string]string) bool {
	predicate = strings.ToLower(strings.TrimSpace(predicate))
	if predicate == "" {
		return false
	}
	props := normalizeKnowledgeGraphProperties(properties)
	switch strings.ToLower(strings.TrimSpace(props["cardinality"])) {
	case "single", "one", "1":
		return true
	}
	switch strings.ToLower(strings.TrimSpace(props["exclusive"])) {
	case "true", "yes", "1":
		return true
	}
	switch predicate {
	case "primary_language", "default_language", "current_ip", "current_hostname", "primary_owner":
		return true
	default:
		return false
	}
}

func scanKGConflictRows(rows *sql.Rows) ([]KGConflict, error) {
	var conflicts []KGConflict
	for rows.Next() {
		var conflict KGConflict
		if err := rows.Scan(
			&conflict.ID, &conflict.SubjectID, &conflict.Predicate,
			&conflict.LeftClaimID, &conflict.RightClaimID,
			&conflict.WinningClaimID, &conflict.SupersededClaimID,
			&conflict.Reason, &conflict.Status, &conflict.DetectedAt, &conflict.ResolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scan kg conflict: %w", err)
		}
		conflicts = append(conflicts, conflict)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate kg conflicts: %w", err)
	}
	return conflicts, nil
}

func getKGConflictByIDTx(tx *sql.Tx, id int64) (*KGConflict, error) {
	rows, err := tx.Query(`
		SELECT id, subject_id, predicate, left_claim_id, right_claim_id,
		       COALESCE(winning_claim_id, ''), COALESCE(superseded_claim_id, ''),
		       reason, status, COALESCE(detected_at, ''), COALESCE(resolved_at, '')
		FROM kg_conflicts
		WHERE id = ?
		LIMIT 1
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	conflicts, err := scanKGConflictRows(rows)
	if err != nil {
		return nil, err
	}
	if len(conflicts) == 0 {
		return nil, sql.ErrNoRows
	}
	return &conflicts[0], nil
}

func getKGClaimCoreTx(tx *sql.Tx, claimID string) (*KGClaim, error) {
	var claim KGClaim
	var status string
	err := tx.QueryRow(`
		SELECT id, subject_id, predicate, object_id, status
		FROM kg_claims
		WHERE id = ?
	`, claimID).Scan(&claim.ID, &claim.SubjectID, &claim.Predicate, &claim.ObjectID, &status)
	if err != nil {
		return nil, err
	}
	claim.Status = KGClaimStatus(status)
	return &claim, nil
}

func (kg *KnowledgeGraph) insertKGEvidenceTx(tx *sql.Tx, provenance KGProvenanceInput, now time.Time) (string, error) {
	rawText := scrubKGEvidenceText(provenance.RawText)
	evidenceType := strings.TrimSpace(provenance.EvidenceType)
	sourceMessageID := strings.TrimSpace(provenance.SourceMessageID)
	sessionID := strings.TrimSpace(provenance.SessionID)
	channel := strings.TrimSpace(provenance.Channel)
	actor := strings.TrimSpace(provenance.Actor)
	sourceURI := strings.TrimSpace(provenance.SourceURI)
	if rawText == "" && evidenceType == "" && sourceMessageID == "" && sessionID == "" && channel == "" && actor == "" && sourceURI == "" {
		return "", nil
	}
	if evidenceType == "" {
		evidenceType = "assertion"
	}
	contentHash := ""
	if rawText != "" {
		sum := sha256.Sum256([]byte(rawText))
		contentHash = hex.EncodeToString(sum[:])
	}
	evidenceID := newKGEvidenceID(now)
	if _, err := tx.Exec(`
		INSERT INTO kg_evidence (
			id, evidence_type, source_message_id, session_id, channel, actor,
			raw_text, source_uri, content_hash
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, evidenceID, evidenceType, sourceMessageID, sessionID, channel, actor, rawText, sourceURI, contentHash); err != nil {
		return "", fmt.Errorf("insert kg evidence: %w", err)
	}
	return evidenceID, nil
}

func kgClaimsWithEvidenceSQL(suffix string) string {
	return `
		SELECT
			c.id, c.subject_id, c.predicate, c.object_id, c.object_literal,
			c.asserted_in_graph, COALESCE(c.learned_at, ''), COALESCE(c.accepted_at, ''),
			c.confidence, c.confidence_label, c.source_kind, c.ingestion_run_id,
			c.status, c.superseded_by, c.source_message_id, c.session_id,
			c.privacy_class, c.retention_policy, c.evidence_id,
			e.id, e.evidence_type, e.source_message_id, e.session_id, e.channel,
			e.actor, e.raw_text, e.source_uri, e.content_hash, COALESCE(e.captured_at, '')
		FROM kg_claims c
		LEFT JOIN kg_evidence e ON e.id = c.evidence_id
	` + suffix
}

func getKGClaimByIDTx(tx *sql.Tx, claimID string) (*KGClaim, error) {
	rows, err := tx.Query(kgClaimsWithEvidenceSQL(`WHERE c.id = ? LIMIT 1`), claimID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	claims, err := scanKGClaimRows(rows)
	if err != nil {
		return nil, err
	}
	if len(claims) == 0 {
		return nil, sql.ErrNoRows
	}
	return &claims[0], nil
}

func scanKGClaimRows(rows *sql.Rows) ([]KGClaim, error) {
	var claims []KGClaim
	for rows.Next() {
		var claim KGClaim
		var status string
		var evidenceID sql.NullString
		var evidence KGEvidence
		var evID, evType, evSourceMessageID, evSessionID, evChannel, evActor, evRawText, evSourceURI, evContentHash, evCapturedAt sql.NullString
		if err := rows.Scan(
			&claim.ID, &claim.SubjectID, &claim.Predicate, &claim.ObjectID, &claim.ObjectLiteral,
			&claim.AssertedInGraph, &claim.LearnedAt, &claim.AcceptedAt,
			&claim.Confidence, &claim.ConfidenceLabel, &claim.SourceKind, &claim.IngestionRunID,
			&status, &claim.SupersededBy, &claim.SourceMessageID, &claim.SessionID,
			&claim.PrivacyClass, &claim.RetentionPolicy, &evidenceID,
			&evID, &evType, &evSourceMessageID, &evSessionID, &evChannel,
			&evActor, &evRawText, &evSourceURI, &evContentHash, &evCapturedAt,
		); err != nil {
			return nil, fmt.Errorf("scan kg claim: %w", err)
		}
		claim.Status = KGClaimStatus(status)
		if evidenceID.Valid {
			claim.EvidenceID = evidenceID.String
		}
		if evID.Valid {
			evidence.ID = evID.String
			evidence.EvidenceType = evType.String
			evidence.SourceMessageID = evSourceMessageID.String
			evidence.SessionID = evSessionID.String
			evidence.Channel = evChannel.String
			evidence.Actor = evActor.String
			evidence.RawText = evRawText.String
			evidence.SourceURI = evSourceURI.String
			evidence.ContentHash = evContentHash.String
			evidence.CapturedAt = evCapturedAt.String
			claim.Evidence = &evidence
		}
		claims = append(claims, claim)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate kg claims: %w", err)
	}
	return claims, nil
}

func newKGClaimID(now time.Time) string {
	return newKGProvenanceID("kg_claim", now)
}

func newKGEvidenceID(now time.Time) string {
	return newKGProvenanceID("kg_evidence", now)
}

func newKGProvenanceID(prefix string, now time.Time) string {
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err == nil {
		return fmt.Sprintf("%s_%d_%s", prefix, now.UTC().UnixNano(), hex.EncodeToString(suffix[:]))
	}
	return fmt.Sprintf("%s_%d", prefix, now.UTC().UnixNano())
}

func scrubKGEvidenceText(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	text = security.RedactSensitiveInfo(security.Scrub(text))
	runes := []rune(text)
	if len(runes) > kgEvidenceRawTextLimit {
		return string(runes[:kgEvidenceRawTextLimit-3]) + "..."
	}
	return text
}

func normalizeKGConfidence(confidence float64) float64 {
	if confidence <= 0 {
		return 0.75
	}
	if confidence > 1 {
		return 1
	}
	return confidence
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func nullableString(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
