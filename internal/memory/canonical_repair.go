package memory

import (
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
		for _, newID := range newIDs {
			if err := s.UpsertMemoryMetaWithDetails(newID, MemoryMetaUpdate{
				ExtractionConfidence: meta.ExtractionConfidence,
				VerificationStatus:   meta.VerificationStatus,
				SourceType:           meta.SourceType,
				SourceReliability:    meta.SourceReliability,
			}); err != nil {
				item.Error = err.Error()
				continue
			}
		}
		reason := "canonical name repair; replacement: " + strings.Join(newIDs, ",")
		if err := s.ApplyMemoryCurationAction(MemoryCurationAction{
			DocID:  meta.DocID,
			Action: MemoryCurationActionArchive,
			Reason: reason,
		}, actor, false); err != nil {
			item.Error = err.Error()
			report.Items = append(report.Items, item)
			report.SkippedCount++
			continue
		}
		if err := ltm.DeleteDocument(meta.DocID); err != nil {
			item.Error = "delete old vector doc: " + err.Error()
		}
		report.RepairedCount++
		report.Items = append(report.Items, item)
	}
	return report, nil
}
