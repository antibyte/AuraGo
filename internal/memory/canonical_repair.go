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

const canonicalRepairCursorKey = "canonical_repair.v1.cursor"

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
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	actor := strings.TrimSpace(opts.Actor)
	if actor == "" {
		actor = "system"
	}
	cursor := ""
	var err error
	if !opts.DryRun {
		cursor, err = s.GetMemoryMaintenanceState(canonicalRepairCursorKey)
		if err != nil {
			return report, fmt.Errorf("load canonical repair cursor: %w", err)
		}
	}
	metas, err := s.GetMemoryMetaAfter(cursor, limit)
	if err != nil {
		return report, fmt.Errorf("load memory meta for canonical repair: %w", err)
	}
	var joinedErr error
	for _, meta := range metas {
		if IsMemoryArchived(meta) || meta.Protected || meta.KeepForever {
			continue
		}
		content, err := ltm.GetByID(meta.DocID)
		if err != nil {
			report.SkippedCount++
			joinedErr = errors.Join(joinedErr, fmt.Errorf("read canonical repair source %s: %w", meta.DocID, err))
			continue
		}
		if strings.TrimSpace(content) == "" {
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
		reason := "canonical name repair"
		newIDs, err := s.ReplaceMemoryDocument(ltm, meta.DocID, "canonical-repair:"+meta.DocID, content, normalized, meta, reason, actor)
		if err != nil {
			item.Error = err.Error()
			if len(newIDs) == 0 && strings.Contains(item.Error, "delete replacement vector") {
				item.Error = "rollback canonical repair artifacts: " + item.Error
			}
			item.NewDocIDs = append([]string(nil), newIDs...)
			joinedErr = errors.Join(joinedErr, err)
			report.Items = append(report.Items, item)
			report.SkippedCount++
			continue
		}
		item.NewDocIDs = append([]string(nil), newIDs...)
		report.RepairedCount++
		report.Items = append(report.Items, item)
	}
	if !opts.DryRun {
		if len(metas) < limit {
			if err := s.ClearMemoryMaintenanceState(canonicalRepairCursorKey); err != nil {
				joinedErr = errors.Join(joinedErr, fmt.Errorf("clear canonical repair cursor: %w", err))
			}
		} else if err := s.SetMemoryMaintenanceState(canonicalRepairCursorKey, metas[len(metas)-1].DocID); err != nil {
			joinedErr = errors.Join(joinedErr, fmt.Errorf("persist canonical repair cursor: %w", err))
		}
	}
	return report, joinedErr
}
