package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"aurago/internal/memory"
)

type memorySourceResult struct {
	Source string      `json:"source"`
	Count  int         `json:"count"`
	Data   interface{} `json:"data"`
}

type contextMemoryResult struct {
	Rank      int     `json:"rank"`
	Source    string  `json:"source"`
	Type      string  `json:"type"`
	Relevance float64 `json:"relevance"`
	Content   string  `json:"content"`
	Reasoning string  `json:"reasoning,omitempty"`
	Date      string  `json:"date,omitempty"`
	DocID     string  `json:"doc_id,omitempty"`
}

func isActivityFocusedQuery(query string) bool {
	q := strings.ToLower(query)
	cues := []string{
		"gestern", "letzte", "last week", "yesterday", "what happened", "woran", "worked on",
		"was wollte", "wanted", "did we do", "haben wir gemacht", "overview", "überblick",
	}
	for _, cue := range cues {
		if strings.Contains(q, cue) {
			return true
		}
	}
	return false
}

func resolveContextTimeRange(value string) (string, string) {
	today := time.Now()
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "all":
		return "", ""
	case "today":
		d := today.Format("2006-01-02")
		return d, d
	case "last_week", "week", "recent":
		return today.AddDate(0, 0, -6).Format("2006-01-02"), today.Format("2006-01-02")
	case "last_month", "month":
		return today.AddDate(0, 0, -29).Format("2006-01-02"), today.Format("2006-01-02")
	default:
		return "", ""
	}
}

func appendActivityResults(stm *memory.SQLiteMemory, query, fromDate, toDate string, limit int, combined *[]memorySourceResult, errors *[]string) {
	if stm == nil {
		return
	}
	entries, err := stm.SearchActivityTurnsInRange(query, fromDate, toDate, limit)
	if err != nil {
		*errors = append(*errors, fmt.Sprintf("activity: %v", err))
		return
	}
	if len(entries) > 0 {
		*combined = append(*combined, memorySourceResult{Source: "activity", Count: len(entries), Data: entries})
	}
}

func appendActivityRollupResults(stm *memory.SQLiteMemory, days int, combined *[]memorySourceResult, errors *[]string) {
	if stm == nil {
		return
	}
	rollups, err := stm.GetRecentDailyActivityRollups(days)
	if err != nil {
		*errors = append(*errors, fmt.Sprintf("daily_activity_rollups: %v", err))
		return
	}
	if len(rollups) > 0 {
		*combined = append(*combined, memorySourceResult{Source: "daily_activity_rollups", Count: len(rollups), Data: rollups})
	}
}

func executeContextMemoryQuery(tc ToolCall, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, kg *memory.KnowledgeGraph) (string, error) {
	if shortTermMem == nil {
		return `Tool Output: {"status":"error","message":"short-term memory unavailable"}`, nil
	}

	query := strings.TrimSpace(tc.Query)
	if query == "" {
		query = strings.TrimSpace(tc.Content)
	}
	if query == "" {
		return `Tool Output: {"status":"error","message":"query is required"}`, nil
	}

	contextDepth := strings.ToLower(strings.TrimSpace(tc.ContextDepth))
	if contextDepth == "" {
		contextDepth = "normal"
	}
	perSourceLimit := 3
	switch contextDepth {
	case "shallow":
		perSourceLimit = 2
	case "deep":
		perSourceLimit = 6
	}

	fromDate, toDate := resolveContextTimeRange(tc.TimeRange)
	if temporalRange, cleanedQuery, ok := memory.ParseTemporalQuery(query); ok {
		fromDate, toDate = temporalRange.FromDate, temporalRange.ToDate
		if cleanedQuery != "" {
			query = cleanedQuery
		}
	}

	sourceMap := map[string]bool{
		"activity": true, "journal": true, "notes": true, "core": true, "kg": true, "ltm": true,
	}
	if len(tc.Sources) > 0 {
		sourceMap = map[string]bool{}
		for _, source := range tc.Sources {
			sourceMap[strings.ToLower(source)] = true
		}
	}

	results := make([]contextMemoryResult, 0, 20)
	addResult := func(source, typ, content, date, reasoning, docID string, relevance float64) {
		if strings.TrimSpace(content) == "" {
			return
		}
		results = append(results, contextMemoryResult{
			Source:    source,
			Type:      typ,
			Content:   content,
			Date:      date,
			Reasoning: reasoning,
			DocID:     docID,
			Relevance: relevance,
		})
	}

	if sourceMap["activity"] {
		if overview, err := shortTermMem.BuildRecentActivityOverview(7, true); err == nil && overview != nil {
			addResult("activity", "overview", overview.OverviewSummary, "", "Recent multi-day activity summary", "", 0.99)
			for _, entry := range overview.Entries {
				if query != "" {
					contentBlob := strings.ToLower(entry.Intent + " " + entry.UserRequest + " " + strings.Join(entry.Outcomes, " ") + " " + strings.Join(entry.ImportantPoints, " "))
					if !strings.Contains(contentBlob, strings.ToLower(query)) && tc.TimeRange == "" && !isActivityFocusedQuery(tc.Query) {
						continue
					}
				}
				addResult("activity", "turn", entry.Intent+" | "+strings.Join(entry.Outcomes, " | "), entry.Date, "Captured turn summary", "", 0.95)
			}
		}
	}
	if sourceMap["journal"] {
		entries, _ := shortTermMem.SearchJournalEntriesInRange(query, fromDate, toDate, perSourceLimit)
		for _, entry := range entries {
			addResult("journal", entry.EntryType, entry.Title+" | "+entry.Content, entry.Date, "Journal event matched query/time window", "", 0.9)
		}
	}
	if sourceMap["notes"] {
		notes, _ := shortTermMem.SearchNotes(query, perSourceLimit)
		for _, note := range notes {
			addResult("notes", "note", note.Title+" | "+note.Content, note.DueDate, "Open or stored note matched the query", "", 0.88)
		}
		if len(query) == 0 {
			if notes, err := shortTermMem.GetHighPriorityOpenNotes(perSourceLimit); err == nil {
				for _, note := range notes {
					addResult("notes", "note", note.Title+" | "+note.Content, note.DueDate, "High-priority open note", "", 0.86)
				}
			}
		}
	}
	if sourceMap["core"] {
		facts, _ := shortTermMem.GetCoreMemoryFacts()
		lowerQ := strings.ToLower(query)
		for _, fact := range facts {
			if lowerQ != "" && !strings.Contains(strings.ToLower(fact.Fact), lowerQ) {
				continue
			}
			addResult("core", "fact", fact.Fact, "", "Persistent core memory fact", fmt.Sprintf("%d", fact.ID), 0.85)
			if len(results) >= 20 {
				break
			}
		}
	}
	if sourceMap["kg"] && kg != nil {
		kgResult := kg.SearchForContext(query, perSourceLimit, 1600)
		if kgResult != "" && kgResult != "No matching entities found." {
			addResult("kg", "entity_network", kgResult, "", "Structured relationships from the knowledge graph", "", 0.92)
		}
		if tc.IncludeRelated && contextDepth == "deep" {
			kgRelated := kg.SearchForContext(query, perSourceLimit+2, 2400)
			if kgRelated != "" && kgRelated != kgResult && kgRelated != "No matching entities found." {
				addResult("kg", "related_network", kgRelated, "", "Expanded related entities", "", 0.84)
			}
		}
	}
	if sourceMap["ltm"] && longTermMem != nil && query != "" {
		memories, docIDs, err := longTermMem.SearchMemoriesOnly(query, perSourceLimit)
		if err == nil {
			for i, memText := range memories {
				docID := ""
				if i < len(docIDs) {
					docID = docIDs[i]
				}
				addResult("ltm", "document", memText, "", "Semantic long-term memory hit", docID, 0.89-float64(i)*0.05)
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Relevance == results[j].Relevance {
			return results[i].Rank < results[j].Rank
		}
		return results[i].Relevance > results[j].Relevance
	})
	for i := range results {
		results[i].Rank = i + 1
	}
	if len(results) > 18 {
		results = results[:18]
	}

	payload := map[string]interface{}{
		"status":           "success",
		"query":            query,
		"context_depth":    contextDepth,
		"time_range":       tc.TimeRange,
		"combined_results": results,
	}
	if fromDate != "" || toDate != "" {
		payload["resolved_range"] = map[string]string{"from_date": fromDate, "to_date": toDate}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal context_memory response: %w", err)
	}
	return "Tool Output: " + string(raw), nil
}
