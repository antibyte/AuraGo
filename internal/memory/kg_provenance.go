package memory

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
