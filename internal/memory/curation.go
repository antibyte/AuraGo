package memory

import (
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
	DocID           string  `json:"doc_id"`
	Action          string  `json:"action"`
	CurrentStatus   string  `json:"current_status"`
	TargetStatus    string  `json:"target_status"`
	Reason          string  `json:"reason"`
	Confidence      float64 `json:"confidence"`
	Reliability     float64 `json:"reliability"`
	AccessCount     int     `json:"access_count"`
	UsefulCount     int     `json:"useful_count"`
	UselessCount    int     `json:"useless_count"`
	DaysSinceAccess int     `json:"days_since_access"`
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
		if status == MemoryVerificationArchived {
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
	return normalizeMemoryVerificationStatus(meta.VerificationStatus) == MemoryVerificationArchived
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
	if status == MemoryVerificationConfirmed || status == MemoryVerificationContradicted {
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
	if s == nil || strings.TrimSpace(action.DocID) == "" {
		return nil
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "system"
	}
	normalizedAction := strings.ToLower(strings.TrimSpace(action.Action))
	reason := strings.TrimSpace(action.Reason)
	if len(reason) > 500 {
		reason = reason[:500]
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin memory curation action: %w", err)
	}
	defer tx.Rollback()

	var previousStatus string
	err = tx.QueryRow(`SELECT COALESCE(verification_status, 'unverified') FROM memory_meta WHERE doc_id = ?`, action.DocID).Scan(&previousStatus)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read memory curation status: %w", err)
	}
	previousStatus = normalizeMemoryVerificationStatus(previousStatus)
	newStatus := previousStatus

	if !dryRun {
		switch normalizedAction {
		case MemoryCurationActionConfirm:
			newStatus = MemoryVerificationConfirmed
			_, err = tx.Exec(`
				UPDATE memory_meta
				SET verification_status = ?, archived_at = NULL, archived_reason = '', last_reviewed_at = CURRENT_TIMESTAMP, review_note = ?, last_event_at = CURRENT_TIMESTAMP
				WHERE doc_id = ?`, newStatus, reason, action.DocID)
		case MemoryCurationActionArchive:
			newStatus = MemoryVerificationArchived
			_, err = tx.Exec(`
				UPDATE memory_meta
				SET verification_status = ?, archived_at = CURRENT_TIMESTAMP, archived_reason = ?, last_reviewed_at = CURRENT_TIMESTAMP, review_note = ?, last_event_at = CURRENT_TIMESTAMP
				WHERE doc_id = ?`, newStatus, reason, reason, action.DocID)
		case MemoryCurationActionUnverify:
			newStatus = MemoryVerificationUnverified
			_, err = tx.Exec(`
				UPDATE memory_meta
				SET verification_status = ?, archived_at = NULL, archived_reason = '', last_reviewed_at = CURRENT_TIMESTAMP, review_note = ?, last_event_at = CURRENT_TIMESTAMP
				WHERE doc_id = ?`, newStatus, reason, action.DocID)
		case MemoryCurationActionProtect:
			_, err = tx.Exec(`UPDATE memory_meta SET protected = 1, last_reviewed_at = CURRENT_TIMESTAMP, review_note = ?, last_event_at = CURRENT_TIMESTAMP WHERE doc_id = ?`, reason, action.DocID)
		case MemoryCurationActionUnprotect:
			_, err = tx.Exec(`UPDATE memory_meta SET protected = 0, keep_forever = 0, last_reviewed_at = CURRENT_TIMESTAMP, review_note = ?, last_event_at = CURRENT_TIMESTAMP WHERE doc_id = ?`, reason, action.DocID)
		default:
			return fmt.Errorf("unsupported memory curation action %q", action.Action)
		}
		if err != nil {
			return fmt.Errorf("apply memory curation action: %w", err)
		}
	} else if normalizedAction == "" {
		normalizedAction = "preview"
	}

	if normalizedAction == MemoryCurationActionProtect || normalizedAction == MemoryCurationActionUnprotect {
		newStatus = previousStatus
	}
	if _, err := tx.Exec(`
		INSERT INTO memory_curation_events (doc_id, action, actor, previous_status, new_status, reason, dry_run)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		action.DocID, normalizedAction, actor, previousStatus, newStatus, reason, dryRun,
	); err != nil {
		return fmt.Errorf("record memory curation event: %w", err)
	}

	return tx.Commit()
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
