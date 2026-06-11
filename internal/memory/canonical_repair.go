package memory

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var canonicalAliasPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{pattern: regexp.MustCompile(`\bAuroraGo\b`), replacement: "AuraGo"},
}

type CanonicalRepairOptions struct {
	Limit  int
	DryRun bool
	Actor  string
}

type CanonicalRepairItem struct {
	OldDocID  string   `json:"old_doc_id"`
	NewDocIDs []string `json:"new_doc_ids"`
	Reason    string   `json:"reason"`
	Error     string   `json:"error,omitempty"`
}

type CanonicalRepairReport struct {
	GeneratedAt   string                `json:"generated_at"`
	DryRun        bool                  `json:"dry_run"`
	RepairedCount int                   `json:"repaired_count"`
	SkippedCount  int                   `json:"skipped_count"`
	Items         []CanonicalRepairItem `json:"items"`
}

func NormalizeCanonicalMemoryNames(content string) string {
	normalized := content
	for _, alias := range canonicalAliasPatterns {
		normalized = alias.pattern.ReplaceAllString(normalized, alias.replacement)
	}
	return normalized
}

func (s *SQLiteMemory) RepairCanonicalMemoryNames(ltm VectorDB, opts CanonicalRepairOptions) (CanonicalRepairReport, error) {
	report := CanonicalRepairReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		DryRun:      opts.DryRun,
	}
	if s == nil || ltm == nil || ltm.IsDisabled() || !ltm.IsReady() {
		return report, nil
	}
	limit := opts.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	actor := strings.TrimSpace(opts.Actor)
	if actor == "" {
		actor = "system"
	}
	metas, err := s.GetAllMemoryMeta(limit, 0)
	if err != nil {
		return report, fmt.Errorf("load memory meta for canonical repair: %w", err)
	}
	var joinedErr error
	for _, meta := range metas {
		if IsMemoryArchived(meta) || meta.Protected || meta.KeepForever {
			continue
		}
		content, err := ltm.GetByID(meta.DocID)
		if err != nil || strings.TrimSpace(content) == "" {
			report.SkippedCount++
			continue
		}
		normalized := NormalizeCanonicalMemoryNames(content)
		if normalized == content {
			continue
		}
		item := CanonicalRepairItem{
			OldDocID: meta.DocID,
			Reason:   "canonical name repair",
		}
		if opts.DryRun {
			report.RepairedCount++
			report.Items = append(report.Items, item)
			continue
		}
		newIDs, err := ltm.StoreDocument("canonical-repair:"+meta.DocID, normalized)
		if err != nil {
			item.Error = err.Error()
			report.Items = append(report.Items, item)
			report.SkippedCount++
			continue
		}
		if len(newIDs) == 0 {
			item.Error = "normalized document was not stored"
			report.Items = append(report.Items, item)
			report.SkippedCount++
			continue
		}
		item.NewDocIDs = append([]string(nil), newIDs...)
		upsertedIDs := make([]string, 0, len(newIDs))
		var metaErr error
		for _, newID := range newIDs {
			if err := s.UpsertMemoryMetaWithDetails(newID, MemoryMetaUpdate{
				ExtractionConfidence: meta.ExtractionConfidence,
				VerificationStatus:   meta.VerificationStatus,
				SourceType:           meta.SourceType,
				SourceReliability:    meta.SourceReliability,
			}); err != nil {
				metaErr = err
				break
			}
			upsertedIDs = append(upsertedIDs, newID)
		}
		if metaErr != nil {
			item.Error = metaErr.Error()
			if rollbackErr := rollbackCanonicalRepairArtifacts(s, ltm, newIDs, upsertedIDs, "canonical repair rollback after meta upsert failure", actor); rollbackErr != nil {
				wrapped := fmt.Errorf("rollback canonical repair artifacts: %w", rollbackErr)
				item.Error = errors.Join(metaErr, wrapped).Error()
			}
			report.Items = append(report.Items, item)
			report.SkippedCount++
			continue
		}
		reason := "canonical name repair; replacement: " + strings.Join(newIDs, ",")
		if err := s.ApplyMemoryCurationAction(MemoryCurationAction{
			DocID:  meta.DocID,
			Action: MemoryCurationActionArchive,
			Reason: reason,
		}, actor, false); err != nil {
			wrapped := fmt.Errorf("archive old memory meta %s: %w", meta.DocID, err)
			item.Error = wrapped.Error()
			joinedErr = errors.Join(joinedErr, wrapped)
			if rollbackErr := rollbackCanonicalRepairArtifacts(s, ltm, newIDs, upsertedIDs, "canonical repair rollback after archive failure", actor); rollbackErr != nil {
				rollbackWrapped := fmt.Errorf("rollback canonical repair artifacts: %w", rollbackErr)
				item.Error = errors.Join(wrapped, rollbackWrapped).Error()
				joinedErr = errors.Join(joinedErr, rollbackWrapped)
			}
			report.Items = append(report.Items, item)
			report.SkippedCount++
			continue
		}
		if err := ltm.DeleteDocument(meta.DocID); err != nil {
			wrapped := fmt.Errorf("delete old vector doc %s: %w", meta.DocID, err)
			item.Error = wrapped.Error()
			joinedErr = errors.Join(joinedErr, wrapped)
		} else if err := s.CleanupDeletedVectorDocumentReferences(meta.DocID); err != nil {
			wrapped := fmt.Errorf("cleanup old vector doc references %s: %w", meta.DocID, err)
			item.Error = wrapped.Error()
			joinedErr = errors.Join(joinedErr, wrapped)
			report.Items = append(report.Items, item)
			report.SkippedCount++
			continue
		}
		report.RepairedCount++
		report.Items = append(report.Items, item)
	}
	return report, joinedErr
}

func rollbackCanonicalRepairArtifacts(s *SQLiteMemory, ltm VectorDB, docIDs []string, metaDocIDs []string, reason string, actor string) error {
	var joinedErr error
	for _, docID := range docIDs {
		if err := ltm.DeleteDocument(docID); err != nil {
			joinedErr = errors.Join(joinedErr, fmt.Errorf("delete rollback vector doc %s: %w", docID, err))
			continue
		}
		if err := s.CleanupDeletedVectorDocumentReferences(docID); err != nil {
			joinedErr = errors.Join(joinedErr, fmt.Errorf("cleanup rollback vector doc references %s: %w", docID, err))
		}
	}
	for _, docID := range metaDocIDs {
		if err := s.ApplyMemoryCurationAction(MemoryCurationAction{
			DocID:  docID,
			Action: MemoryCurationActionArchive,
			Reason: reason,
		}, actor, false); err != nil {
			joinedErr = errors.Join(joinedErr, fmt.Errorf("archive rollback meta %s: %w", docID, err))
		}
	}
	return joinedErr
}
