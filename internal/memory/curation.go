package memory

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	MemoryVerificationUnverified   = "unverified"
	MemoryVerificationConfirmed    = "confirmed"
	MemoryVerificationContradicted = "contradicted"
	MemoryVerificationArchived     = "archived"

	MemoryCurationActionConfirm     = "confirm"
	MemoryCurationActionArchive     = "archive"
	MemoryCurationActionUnverify    = "unverify"
	MemoryCurationActionProtect     = "protect"
	MemoryCurationActionUnprotect   = "unprotect"
	defaultCurationMaxActions       = 100
	defaultCurationConfirmThreshold = 0.92
)

type MemoryCurationOptions struct {
	ConfirmThreshold float64
	MaxActions       int
	Now              time.Time
}

type MemoryCurationAction struct {
	ExpectedMeta    *MemoryMeta `json:"-"`
	DocID           string      `json:"doc_id"`
	Action          string      `json:"action"`
	CurrentStatus   string      `json:"current_status"`
	TargetStatus    string      `json:"target_status"`
	Reason          string      `json:"reason"`
	Confidence      float64     `json:"confidence"`
	Reliability     float64     `json:"reliability"`
	AccessCount     int         `json:"access_count"`
	UsefulCount     int         `json:"useful_count"`
	UselessCount    int         `json:"useless_count"`
	DaysSinceAccess int         `json:"days_since_access"`
}

type MemoryCurationPlan struct {
	GeneratedAt         string                 `json:"generated_at"`
	AutoConfirm         []MemoryCurationAction `json:"auto_confirm"`
	AutoArchive         []MemoryCurationAction `json:"auto_archive"`
	ReviewRequired      []MemoryCurationAction `json:"review_required"`
	AutoConfirmCount    int                    `json:"auto_confirm_count"`
	AutoArchiveCount    int                    `json:"auto_archive_count"`
	ReviewRequiredCount int                    `json:"review_required_count"`
}

type MemoryCurationEvent struct {
	ID             int64  `json:"id"`
	Timestamp      string `json:"timestamp"`
	DocID          string `json:"doc_id"`
	Action         string `json:"action"`
	Actor          string `json:"actor"`
	PreviousStatus string `json:"previous_status"`
	NewStatus      string `json:"new_status"`
	Reason         string `json:"reason"`
	DryRun         bool   `json:"dry_run"`
}

func BuildMemoryCurationPlan(metas []MemoryMeta, usage MemoryUsageStats, opts MemoryCurationOptions) MemoryCurationPlan {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	threshold := opts.ConfirmThreshold
	if threshold <= 0 {
		threshold = defaultCurationConfirmThreshold
	}
	if threshold > 1 {
		threshold = 1
	}
	maxActions := opts.MaxActions
	if maxActions <= 0 {
		maxActions = defaultCurationMaxActions
	}

	usageByID := make(map[string]MemoryUsageAggregate, len(usage.TopReused))
	for _, item := range usage.TopReused {
		usageByID[item.MemoryID] = item
	}

	plan := MemoryCurationPlan{GeneratedAt: now.Format(time.RFC3339)}
	for _, meta := range metas {
		status := normalizeMemoryVerificationStatus(meta.VerificationStatus)
		if IsMemoryArchived(meta) {
			continue
		}
		actionBase := memoryCurationActionBase(meta, now)
		if status == MemoryVerificationContradicted {
			actionBase.Action = "review"
			actionBase.TargetStatus = MemoryVerificationContradicted
			actionBase.Reason = "contradicted memory requires manual review"
			plan.ReviewRequired = append(plan.ReviewRequired, actionBase)
			continue
		}
		if meta.Protected || meta.KeepForever {
			continue
		}

		usageItem := usageByID[meta.DocID]
		if shouldConfirmMemory(meta, usageItem, status, threshold) {
			actionBase.Action = MemoryCurationActionConfirm
			actionBase.TargetStatus = MemoryVerificationConfirmed
			actionBase.Reason = "high-confidence memory with positive usage"
			plan.AutoConfirm = append(plan.AutoConfirm, actionBase)
			continue
		}
		if shouldArchiveMemory(meta, status, actionBase.DaysSinceAccess) {
			actionBase.Action = MemoryCurationActionArchive
			actionBase.TargetStatus = MemoryVerificationArchived
			actionBase.Reason = archiveReason(meta, actionBase.DaysSinceAccess)
			plan.AutoArchive = append(plan.AutoArchive, actionBase)
		}
	}

	sortCurationActions(plan.AutoConfirm)
	sortCurationActions(plan.AutoArchive)
	sortCurationActions(plan.ReviewRequired)
	if len(plan.AutoConfirm) > maxActions {
		plan.AutoConfirm = plan.AutoConfirm[:maxActions]
	}
	remaining := maxActions - len(plan.AutoConfirm)
	if remaining < 0 {
		remaining = 0
	}
	if len(plan.AutoArchive) > remaining {
		plan.AutoArchive = plan.AutoArchive[:remaining]
	}
	plan.AutoConfirmCount = len(plan.AutoConfirm)
	plan.AutoArchiveCount = len(plan.AutoArchive)
	plan.ReviewRequiredCount = len(plan.ReviewRequired)
	return plan
}

func IsMemoryArchived(meta MemoryMeta) bool {
	return normalizeMemoryVerificationStatus(meta.VerificationStatus) == MemoryVerificationArchived || strings.TrimSpace(meta.ArchivedAt) != ""
}

func normalizeMemoryVerificationStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case MemoryVerificationConfirmed:
		return MemoryVerificationConfirmed
	case MemoryVerificationContradicted:
		return MemoryVerificationContradicted
	case MemoryVerificationArchived:
		return MemoryVerificationArchived
	default:
		return MemoryVerificationUnverified
	}
}

func shouldConfirmMemory(meta MemoryMeta, usage MemoryUsageAggregate, status string, threshold float64) bool {
	if status != MemoryVerificationUnverified {
		return false
	}
	confidence := normalizedConfidence(meta.ExtractionConfidence)
	reliability := normalizedReliability(meta.SourceReliability)
	if confidence < threshold || reliability < 0.80 {
		return false
	}
	if meta.UselessCount > meta.UsefulCount {
		return false
	}
	return meta.AccessCount >= 2 || meta.UsefulCount > 0 || usage.WasCitedRecently || usage.Count >= 2
}

func shouldArchiveMemory(meta MemoryMeta, status string, daysSinceAccess int) bool {
	if meta.Protected || meta.KeepForever || IsMemoryArchived(meta) || status == MemoryVerificationConfirmed || status == MemoryVerificationContradicted || daysSinceAccess < 14 {
		return false
	}
	confidence := normalizedConfidence(meta.ExtractionConfidence)
	totalEffectiveness := meta.UsefulCount + meta.UselessCount
	if totalEffectiveness >= 3 && meta.UselessCount >= meta.UsefulCount+2 {
		return true
	}
	if confidence < 0.55 && daysSinceAccess >= 14 && meta.AccessCount == 0 {
		return true
	}
	return daysSinceAccess >= 45 && meta.AccessCount <= 1 && confidence < 0.80
}

func archiveReason(meta MemoryMeta, daysSinceAccess int) string {
	totalEffectiveness := meta.UsefulCount + meta.UselessCount
	if totalEffectiveness >= 3 && meta.UselessCount >= meta.UsefulCount+2 {
		return "low effectiveness"
	}
	if normalizedConfidence(meta.ExtractionConfidence) < 0.55 {
		return "low confidence and stale"
	}
	return fmt.Sprintf("stale low-touch memory (%d days)", daysSinceAccess)
}

func memoryCurationActionBase(meta MemoryMeta, now time.Time) MemoryCurationAction {
	lastAccessed := parseMemoryMetaTime(meta.LastAccessed)
	days := 0
	if !lastAccessed.IsZero() {
		days = int(now.Sub(lastAccessed).Hours() / 24)
		if days < 0 {
			days = 0
		}
	}
	return MemoryCurationAction{
		ExpectedMeta:    &meta,
		DocID:           meta.DocID,
		CurrentStatus:   normalizeMemoryVerificationStatus(meta.VerificationStatus),
		Confidence:      normalizedConfidence(meta.ExtractionConfidence),
		Reliability:     normalizedReliability(meta.SourceReliability),
		AccessCount:     meta.AccessCount,
		UsefulCount:     meta.UsefulCount,
		UselessCount:    meta.UselessCount,
		DaysSinceAccess: days,
	}
}

func sortCurationActions(actions []MemoryCurationAction) {
	sort.SliceStable(actions, func(i, j int) bool {
		if actions[i].DaysSinceAccess == actions[j].DaysSinceAccess {
			return actions[i].DocID < actions[j].DocID
		}
		return actions[i].DaysSinceAccess > actions[j].DaysSinceAccess
	})
}

func normalizedConfidence(value float64) float64 {
	if value <= 0 {
		return 0.75
	}
	if value > 1 {
		return 1
	}
	return value
}

func normalizedReliability(value float64) float64 {
	if value <= 0 {
		return 0.70
	}
	if value > 1 {
		return 1
	}
	return value
}

func (s *SQLiteMemory) ApplyMemoryCurationAction(action MemoryCurationAction, actor string, dryRun bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.ApplyMemoryCurationActionContext(ctx, action, actor, dryRun)
}

func (s *SQLiteMemory) ApplyMemoryCurationActionContext(ctx context.Context, action MemoryCurationAction, actor string, dryRun bool) error {
	_, err := s.applyMemoryCurationAction(ctx, action, actor, dryRun, false)
	return err
}

// ApplyAutomaticMemoryCurationAction requires an internal snapshot and reports skipped actions.
// Actor labels never grant permission to override a current human decision.
func (s *SQLiteMemory) ApplyAutomaticMemoryCurationAction(action MemoryCurationAction) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.ApplyAutomaticMemoryCurationActionContext(ctx, action)
}

func (s *SQLiteMemory) ApplyAutomaticMemoryCurationActionContext(ctx context.Context, action MemoryCurationAction) (bool, error) {
	if action.ExpectedMeta == nil {
		return false, fmt.Errorf("automatic curation requires a metadata snapshot")
	}
	return s.applyMemoryCurationAction(ctx, action, "system", false, true)
}

func (s *SQLiteMemory) applyMemoryCurationAction(ctx context.Context, action MemoryCurationAction, actor string, dryRun, automatic bool) (bool, error) {
	if s == nil || s.db == nil || strings.TrimSpace(action.DocID) == "" {
		return false, fmt.Errorf("memory curation store or document ID is unavailable")
	}
	if err := lockMemoryMutex(ctx, &s.automaticMemoryMu); err != nil {
		return false, err
	}
	defer s.automaticMemoryMu.Unlock()
	ctx, release, err := s.BeginMemoryPublication(ctx)
	if err != nil {
		return false, err
	}
	defer release()
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "system"
	}
	normalizedAction := strings.ToLower(strings.TrimSpace(action.Action))
	reason := truncateUTF8Bytes(strings.TrimSpace(action.Reason), 500)
	var identity string
	vector := s.boundArchiveVector()
	if !dryRun && (normalizedAction == MemoryCurationActionArchive ||
		normalizedAction == MemoryCurationActionConfirm || normalizedAction == MemoryCurationActionUnverify) {
		identity, _, err = archiveDocumentIdentity(ctx, vector, action.DocID)
		if err != nil {
			return false, err
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin memory curation action: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE memory_meta SET doc_id=doc_id WHERE doc_id=?`, action.DocID); err != nil {
		return false, err
	}
	var current MemoryMeta
	err = scanMemoryMeta(tx.QueryRowContext(ctx, `SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, action.DocID), &current)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read memory curation status: %w", err)
	}
	if automatic {
		if current.Protected || current.KeepForever || IsMemoryArchived(current) || !memoryMetaEqual(current, *action.ExpectedMeta) {
			return false, nil
		}
		if normalizedAction != MemoryCurationActionConfirm && normalizedAction != MemoryCurationActionArchive {
			return false, fmt.Errorf("unsupported automatic curation action %q", action.Action)
		}
		if normalizedAction == MemoryCurationActionArchive && !shouldArchiveMemory(current, normalizeMemoryVerificationStatus(current.VerificationStatus), memoryCurationActionBase(current, time.Now().UTC()).DaysSinceAccess) {
			return false, nil
		}
	}
	previousStatus := normalizeMemoryVerificationStatus(current.VerificationStatus)
	newStatus := previousStatus
	if !dryRun {
		if normalizedAction == MemoryCurationActionArchive {
			if identity != "" {
				if _, err := tx.ExecContext(ctx, `INSERT INTO memory_archive_identities(doc_id,identity) VALUES(?,?)
					ON CONFLICT(doc_id) DO UPDATE SET identity=excluded.identity`, action.DocID, identity); err != nil {
					return false, err
				}
			} else if _, err := tx.ExecContext(ctx, `INSERT INTO memory_archive_identities(doc_id,identity)
				SELECT doc_id,identity FROM memory_document_identities WHERE doc_id=?
				ON CONFLICT(doc_id) DO NOTHING`, action.DocID); err != nil {
				return false, err
			}
		} else if IsMemoryArchived(current) && (normalizedAction == MemoryCurationActionConfirm || normalizedAction == MemoryCurationActionUnverify) {
			var blocked bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_archive_identities WHERE doc_id=?)`, action.DocID).Scan(&blocked); err != nil {
				return false, err
			}
			if blocked && identity == "" {
				return false, fmt.Errorf("archived document content is unavailable; restore its source before reactivating it")
			}
			for _, table := range []string{"memory_archive_cleanup", "memory_archive_identities"} {
				if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE doc_id=?`, action.DocID); err != nil {
					return false, err
				}
			}
		}
	}

	if !dryRun {
		switch normalizedAction {
		case MemoryCurationActionConfirm:
			newStatus = MemoryVerificationConfirmed
			_, err = tx.ExecContext(ctx, `
				UPDATE memory_meta
				SET verification_status = ?, archived_at = NULL, archived_reason = '', last_reviewed_at = CURRENT_TIMESTAMP, review_note = ?, last_event_at = CURRENT_TIMESTAMP
				WHERE doc_id = ?`, newStatus, reason, action.DocID)
		case MemoryCurationActionArchive:
			newStatus = MemoryVerificationArchived
			_, err = tx.ExecContext(ctx, `
				UPDATE memory_meta
				SET verification_status = ?, archived_at = CURRENT_TIMESTAMP, archived_reason = ?, last_reviewed_at = CURRENT_TIMESTAMP, review_note = ?, last_event_at = CURRENT_TIMESTAMP
				WHERE doc_id = ?`, newStatus, reason, reason, action.DocID)
		case MemoryCurationActionUnverify:
			newStatus = MemoryVerificationUnverified
			_, err = tx.ExecContext(ctx, `
				UPDATE memory_meta
				SET verification_status = ?, archived_at = NULL, archived_reason = '', last_reviewed_at = CURRENT_TIMESTAMP, review_note = ?, last_event_at = CURRENT_TIMESTAMP
				WHERE doc_id = ?`, newStatus, reason, action.DocID)
		case MemoryCurationActionProtect:
			_, err = tx.ExecContext(ctx, `UPDATE memory_meta SET protected = 1, last_reviewed_at = CURRENT_TIMESTAMP, review_note = ?, last_event_at = CURRENT_TIMESTAMP WHERE doc_id = ?`, reason, action.DocID)
		case MemoryCurationActionUnprotect:
			_, err = tx.ExecContext(ctx, `UPDATE memory_meta SET protected = 0, keep_forever = 0, last_reviewed_at = CURRENT_TIMESTAMP, review_note = ?, last_event_at = CURRENT_TIMESTAMP WHERE doc_id = ?`, reason, action.DocID)
		default:
			return false, fmt.Errorf("unsupported memory curation action %q", action.Action)
		}
		if err != nil {
			return false, fmt.Errorf("apply memory curation action: %w", err)
		}
	} else if normalizedAction == "" {
		normalizedAction = "preview"
	}

	if normalizedAction == MemoryCurationActionProtect || normalizedAction == MemoryCurationActionUnprotect {
		newStatus = previousStatus
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO memory_curation_events (doc_id, action, actor, previous_status, new_status, reason, dry_run)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		action.DocID, normalizedAction, actor, previousStatus, newStatus, reason, dryRun,
	); err != nil {
		return false, fmt.Errorf("record memory curation event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return !dryRun, nil
}

// ArchiveMemoryMetaBatch archives a batch of memory_meta rows for budget
// enforcement or bulk curation. It updates verification_status and records a
// single curation event per docID in one transaction.
func (s *SQLiteMemory) ArchiveMemoryMetaBatch(docIDs []string, reason, actor string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := s.ArchiveMemoryMetaBatchContext(ctx, docIDs, reason, actor)
	return err
}

// ArchiveMemoryMetaBatchContext returns only the documents actually archived.
func (s *SQLiteMemory) ArchiveMemoryMetaBatchContext(ctx context.Context, docIDs []string, reason, actor string) ([]string, error) {
	if s == nil || len(docIDs) == 0 {
		return nil, nil
	}
	if err := lockMemoryMutex(ctx, &s.automaticMemoryMu); err != nil {
		return nil, err
	}
	defer s.automaticMemoryMu.Unlock()
	ctx, release, err := s.BeginMemoryPublication(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	reason = truncateUTF8Bytes(strings.TrimSpace(reason), 500)
	if reason == "" {
		reason = "bulk archive"
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "system"
	}
	var candidates []memoryArchiveCandidate
	seen := make(map[string]bool)
	for _, id := range docIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		var candidate memoryArchiveCandidate
		err := scanMemoryMeta(s.db.QueryRowContext(ctx, `SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, id), &candidate.meta)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		candidate.identity, _, err = archiveDocumentIdentity(ctx, s.boundArchiveVector(), id)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var applied []string
	for _, candidate := range candidates {
		var current MemoryMeta
		err := scanMemoryMeta(tx.QueryRowContext(ctx, `SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, candidate.meta.DocID), &current)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		if current.Protected || current.KeepForever || IsMemoryArchived(current) || !memoryMetaEqual(current, candidate.meta) {
			continue
		}
		if err := archiveMemoryCandidate(ctx, tx, candidate, reason, actor, false); err != nil {
			return nil, err
		}
		applied = append(applied, current.DocID)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return applied, nil
}

func (s *SQLiteMemory) ListMemoryCurationEvents(limit int) ([]MemoryCurationEvent, error) {
	if s == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT id, timestamp, doc_id, action, actor, previous_status, new_status, reason, dry_run
		FROM memory_curation_events
		ORDER BY id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list memory curation events: %w", err)
	}
	defer rows.Close()

	events := make([]MemoryCurationEvent, 0, limit)
	for rows.Next() {
		var item MemoryCurationEvent
		if err := rows.Scan(&item.ID, &item.Timestamp, &item.DocID, &item.Action, &item.Actor, &item.PreviousStatus, &item.NewStatus, &item.Reason, &item.DryRun); err != nil {
			return nil, fmt.Errorf("scan memory curation event: %w", err)
		}
		events = append(events, item)
	}
	return events, rows.Err()
}

func (s *SQLiteMemory) ListArchivedMemoryMeta(limit int) ([]MemoryMeta, error) {
	if s == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT doc_id, access_count, last_accessed, last_event_at, extraction_confidence,
		       verification_status, source_type, source_reliability, useful_count, useless_count,
		       COALESCE(last_effectiveness_at, ''), protected, keep_forever,
		       COALESCE(archived_at, ''), COALESCE(archived_reason, ''), COALESCE(last_reviewed_at, ''), COALESCE(review_note, '')
		FROM memory_meta
		WHERE verification_status = 'archived'
		ORDER BY archived_at DESC, doc_id ASC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list archived memory meta: %w", err)
	}
	defer rows.Close()

	metas := make([]MemoryMeta, 0, limit)
	for rows.Next() {
		var m MemoryMeta
		if err := rows.Scan(
			&m.DocID,
			&m.AccessCount,
			&m.LastAccessed,
			&m.LastEventAt,
			&m.ExtractionConfidence,
			&m.VerificationStatus,
			&m.SourceType,
			&m.SourceReliability,
			&m.UsefulCount,
			&m.UselessCount,
			&m.LastEffectivenessAt,
			&m.Protected,
			&m.KeepForever,
			&m.ArchivedAt,
			&m.ArchivedReason,
			&m.LastReviewedAt,
			&m.ReviewNote,
		); err != nil {
			return nil, fmt.Errorf("scan archived memory meta: %w", err)
		}
		metas = append(metas, m)
	}
	return metas, rows.Err()
}
