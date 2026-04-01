package memory

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// MemoryUsageAggregate captures grouped usage activity for a memory chunk.
type MemoryUsageAggregate struct {
	MemoryID         string  `json:"memory_id"`
	MemoryType       string  `json:"memory_type"`
	Count            int     `json:"count"`
	AvgRelevance     float64 `json:"avg_relevance"`
	LastUsed         string  `json:"last_used"`
	WasCitedRecently bool    `json:"was_cited_recently"`
}

// MemoryUsageStats summarizes recent usage telemetry for retrieval diagnostics.
type MemoryUsageStats struct {
	WindowDays       int                    `json:"window_days"`
	TotalEvents      int                    `json:"total_events"`
	RetrievedEvents  int                    `json:"retrieved_events"`
	PredictedEvents  int                    `json:"predicted_events"`
	CitedEvents      int                    `json:"cited_events"`
	DistinctMemories int                    `json:"distinct_memories"`
	DistinctSessions int                    `json:"distinct_sessions"`
	TopReused        []MemoryUsageAggregate `json:"top_reused"`
}

// MemoryConfidenceStats summarizes metadata quality signals.
type MemoryConfidenceStats struct {
	Total            int `json:"total"`
	Confirmed        int `json:"confirmed"`
	Unverified       int `json:"unverified"`
	Contradicted     int `json:"contradicted"`
	HighConfidence   int `json:"high_confidence"`
	MediumConfidence int `json:"medium_confidence"`
	LowConfidence    int `json:"low_confidence"`
}

// MemoryEffectivenessStats summarizes whether injected memories tend to help or distract.
type MemoryEffectivenessStats struct {
	Tracked         int `json:"tracked"`
	Untested        int `json:"untested"`
	Helpful         int `json:"helpful"`
	Underperforming int `json:"underperforming"`
	TotalUseful     int `json:"total_useful"`
	TotalUseless    int `json:"total_useless"`
}

// MemoryCuratorDryRun is a non-destructive maintenance proposal.
type MemoryCuratorDryRun struct {
	StaleCandidates     int      `json:"stale_candidates"`
	VerificationBacklog int      `json:"verification_backlog"`
	Contradictions      int      `json:"contradictions"`
	LowConfidence       int      `json:"low_confidence"`
	LowEffectiveness    int      `json:"low_effectiveness"`
	OverusedMemories    int      `json:"overused_memories"`
	TopStale            []string `json:"top_stale"`
	TopOverused         []string `json:"top_overused"`
	TopUnderperforming  []string `json:"top_underperforming"`
	Suggestions         []string `json:"suggestions"`
}

// MemoryHealthReport combines retrieval, confidence, episodic, and curator signals.
type MemoryHealthReport struct {
	Usage         MemoryUsageStats         `json:"usage"`
	Confidence    MemoryConfidenceStats    `json:"confidence"`
	Effectiveness MemoryEffectivenessStats `json:"effectiveness"`
	Curator       MemoryCuratorDryRun      `json:"curator"`
}

// GetMemoryUsageStats summarizes recent retrieval/prefetch usage from memory_usage_log.
func (s *SQLiteMemory) GetMemoryUsageStats(windowDays int, topLimit int) (MemoryUsageStats, error) {
	if windowDays <= 0 {
		windowDays = 14
	}
	if topLimit <= 0 {
		topLimit = 5
	}

	stats := MemoryUsageStats{WindowDays: windowDays}

	err := s.db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN memory_type = 'ltm_retrieved' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN memory_type = 'ltm_predicted' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN was_cited = 1 THEN 1 ELSE 0 END), 0),
			COUNT(DISTINCT memory_id),
			COUNT(DISTINCT session_id)
		FROM memory_usage_log
		WHERE used_at >= datetime('now', printf('-%d days', ?))`, windowDays).Scan(
		&stats.TotalEvents,
		&stats.RetrievedEvents,
		&stats.PredictedEvents,
		&stats.CitedEvents,
		&stats.DistinctMemories,
		&stats.DistinctSessions,
	)
	if err != nil {
		return stats, fmt.Errorf("memory usage summary: %w", err)
	}

	rows, err := s.db.Query(`
		SELECT memory_id, memory_type, COUNT(*) AS cnt, 
		       COALESCE(AVG(context_relevance), 0), 
		       MAX(used_at), 
		       MAX(CASE WHEN was_cited = 1 THEN 1 ELSE 0 END)
		FROM memory_usage_log
		WHERE used_at >= datetime('now', printf('-%d days', ?))
		GROUP BY memory_id, memory_type
		ORDER BY cnt DESC, MAX(used_at) DESC
		LIMIT ?`, windowDays, topLimit)
	if err != nil {
		return stats, fmt.Errorf("memory usage top reused: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item MemoryUsageAggregate
		var citedFlag int
		if err := rows.Scan(&item.MemoryID, &item.MemoryType, &item.Count, &item.AvgRelevance, &item.LastUsed, &citedFlag); err != nil {
			return stats, fmt.Errorf("scan memory usage aggregate: %w", err)
		}
		item.WasCitedRecently = citedFlag > 0
		stats.TopReused = append(stats.TopReused, item)
	}
	return stats, rows.Err()
}

// BuildMemoryHealthReport composes a conservative health view for dashboards and dry-run curation.
func BuildMemoryHealthReport(metas []MemoryMeta, usage MemoryUsageStats) MemoryHealthReport {
	report := MemoryHealthReport{
		Usage:         usage,
		Confidence:    buildMemoryConfidenceStats(metas),
		Effectiveness: buildMemoryEffectivenessStats(metas),
		Curator:       buildMemoryCuratorDryRun(metas, usage),
	}
	return report
}

func buildMemoryConfidenceStats(metas []MemoryMeta) MemoryConfidenceStats {
	stats := MemoryConfidenceStats{Total: len(metas)}
	for _, meta := range metas {
		switch strings.ToLower(strings.TrimSpace(meta.VerificationStatus)) {
		case "confirmed":
			stats.Confirmed++
		case "contradicted":
			stats.Contradicted++
		default:
			stats.Unverified++
		}

		confidence := meta.ExtractionConfidence
		if confidence <= 0 {
			confidence = 0.75
		}

		switch {
		case confidence >= 0.8:
			stats.HighConfidence++
		case confidence >= 0.55:
			stats.MediumConfidence++
		default:
			stats.LowConfidence++
		}
	}
	return stats
}

func buildMemoryEffectivenessStats(metas []MemoryMeta) MemoryEffectivenessStats {
	stats := MemoryEffectivenessStats{}
	for _, meta := range metas {
		stats.TotalUseful += meta.UsefulCount
		stats.TotalUseless += meta.UselessCount

		total := meta.UsefulCount + meta.UselessCount
		if total == 0 {
			stats.Untested++
			continue
		}
		stats.Tracked++
		if total >= 2 && meta.UselessCount > meta.UsefulCount {
			stats.Underperforming++
		} else if meta.UsefulCount > meta.UselessCount {
			stats.Helpful++
		}
	}
	return stats
}

type staleEntry struct {
	docID string
	score int
}

func buildMemoryCuratorDryRun(metas []MemoryMeta, usage MemoryUsageStats) MemoryCuratorDryRun {
	report := MemoryCuratorDryRun{}
	stale := make([]staleEntry, 0)
	underperforming := make([]staleEntry, 0)

	for _, meta := range metas {
		status := strings.ToLower(strings.TrimSpace(meta.VerificationStatus))
		confidence := meta.ExtractionConfidence
		if confidence <= 0 {
			confidence = 0.75
		}
		reliability := meta.SourceReliability
		if reliability <= 0 {
			reliability = 0.70
		}

		// Count verification backlog
		if status == "unverified" || status == "" {
			report.VerificationBacklog++
		}

		// Count contradictions
		if status == "contradicted" {
			report.Contradictions++
		}

		// Count low confidence memories
		if confidence < 0.55 {
			report.LowConfidence++
		}

		// Count repeatedly underperforming memories so forgetting can prioritize them.
		totalEffectiveness := meta.UsefulCount + meta.UselessCount
		if totalEffectiveness >= 2 && meta.UselessCount > meta.UsefulCount {
			report.LowEffectiveness++
			underperforming = append(underperforming, staleEntry{
				docID: meta.DocID,
				score: meta.UselessCount - meta.UsefulCount,
			})
		}

		// Find stale candidates (not accessed recently, low access count, not confirmed)
		lastTouched := parseMemoryMetaTime(meta.LastAccessed)
		if !lastTouched.IsZero() {
			daysStale := int(time.Since(lastTouched).Hours() / 24)
			if daysStale >= 21 && meta.AccessCount <= 1 && status != "confirmed" {
				report.StaleCandidates++
				stale = append(stale, staleEntry{
					docID: meta.DocID,
					score: daysStale - meta.AccessCount,
				})
			}
		}
	}

	// Find overused memories from usage stats
	for _, item := range usage.TopReused {
		if item.Count >= 3 {
			report.OverusedMemories++
			report.TopOverused = append(report.TopOverused, item.MemoryID)
		}
	}

	// Sort stale entries by score (highest first)
	sort.Slice(stale, func(i, j int) bool {
		if stale[i].score == stale[j].score {
			return stale[i].docID < stale[j].docID
		}
		return stale[i].score > stale[j].score
	})

	// Take top 5 stale entries
	for i, item := range stale {
		if i >= 5 {
			break
		}
		report.TopStale = append(report.TopStale, item.docID)
	}

	sort.Slice(underperforming, func(i, j int) bool {
		if underperforming[i].score == underperforming[j].score {
			return underperforming[i].docID < underperforming[j].docID
		}
		return underperforming[i].score > underperforming[j].score
	})
	for i, item := range underperforming {
		if i >= 5 {
			break
		}
		report.TopUnderperforming = append(report.TopUnderperforming, item.docID)
	}

	// Generate suggestions
	if report.StaleCandidates > 0 {
		report.Suggestions = append(report.Suggestions,
			fmt.Sprintf("Review %d stale low-touch memories before they become prompt noise.", report.StaleCandidates))
	}
	if report.VerificationBacklog > 0 {
		report.Suggestions = append(report.Suggestions,
			fmt.Sprintf("Verify %d unverified memories to improve retrieval trust.", report.VerificationBacklog))
	}
	if report.Contradictions > 0 {
		report.Suggestions = append(report.Suggestions,
			fmt.Sprintf("Resolve %d contradicted memories or mark them archived.", report.Contradictions))
	}
	if report.OverusedMemories > 0 {
		report.Suggestions = append(report.Suggestions,
			fmt.Sprintf("Inspect %d repeatedly reused memories for consolidation or splitting.", report.OverusedMemories))
	}
	if report.LowEffectiveness > 0 {
		report.Suggestions = append(report.Suggestions,
			fmt.Sprintf("Review %d low-effectiveness memories first during auto-optimize or manual cleanup.", report.LowEffectiveness))
	}
	if len(report.Suggestions) == 0 {
		report.Suggestions = append(report.Suggestions,
			"Memory health looks balanced; only routine verification is recommended.")
	}

	return report
}

func parseMemoryMetaTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
