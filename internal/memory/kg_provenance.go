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
		INSERT INTO kg_edges (source, target, relation, properties, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(source, target, relation) DO UPDATE SET
			properties = excluded.properties,
			updated_at = CURRENT_TIMESTAMP
	`, source, target, relation, string(propsJSON)); err != nil {
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
