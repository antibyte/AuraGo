package agent

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"aurago/internal/memory"
	"aurago/internal/planner"
	"aurago/internal/tools"
)

type memorySourceResult struct {
	Source string      `json:"source"`
	Count  int         `json:"count"`
	Data   interface{} `json:"data"`
}

type memorySearchBundle struct {
	Results          []memorySourceResult
	Errors           []string
	TemporalRange    memory.TemporalQueryRange
	NormalizedQuery  string
	HasTemporalRange bool
	SourceMap        map[string]bool
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

type plannerMemoryPayload struct {
	Summary          string                `json:"summary,omitempty"`
	OpenTodoCount    int                   `json:"open_todo_count,omitempty"`
	OverdueTodoCount int                   `json:"overdue_todo_count,omitempty"`
	Todos            []planner.Todo        `json:"todos,omitempty"`
	Appointments     []planner.Appointment `json:"appointments,omitempty"`
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

func appendPlannerResults(plannerDB *sql.DB, query, timeRange string, limit int, combined *[]memorySourceResult, errors *[]string) {
	if plannerDB == nil {
		return
	}
	result, err := searchPlannerMemory(plannerDB, query, timeRange, time.Now(), limit, limit)
	if err != nil {
		*errors = append(*errors, fmt.Sprintf("planner: %v", err))
		return
	}
	if result.Summary == "" && len(result.Todos) == 0 && len(result.Appointments) == 0 {
		return
	}
	payload := plannerMemoryPayload{
		Summary:          result.Summary,
		OpenTodoCount:    result.OpenTodoCount,
		OverdueTodoCount: result.OverdueTodoCount,
		Todos:            result.Todos,
		Appointments:     result.Appointments,
	}
	count := len(result.Todos) + len(result.Appointments)
	if result.Summary != "" {
		count++
	}
	*combined = append(*combined, memorySourceResult{Source: "planner", Count: count, Data: payload})
}

func normalizeMemorySourceMap(sources []string, defaults map[string]bool) map[string]bool {
	if len(sources) == 0 {
		copied := make(map[string]bool, len(defaults))
		for key, enabled := range defaults {
			copied[key] = enabled
		}
		return copied
	}

	aliases := map[string]string{
		"activity":        "activity",
		"journal":         "journal",
		"episodic":        "episodic",
		"notes":           "notes",
		"core":            "core",
		"core_memory":     "core",
		"kg":              "kg",
		"knowledge_graph": "kg",
		"ltm":             "ltm",
		"vector_db":       "ltm",
		"planner":         "planner",
		"todos":           "planner",
		"appointments":    "planner",
		"cheatsheet":      "cheatsheets",
		"cheatsheets":     "cheatsheets",
		"error_patterns":  "error_patterns",
	}

	normalized := map[string]bool{}
	for _, source := range sources {
		key := strings.ToLower(strings.TrimSpace(source))
		if key == "" {
			continue
		}
		if canonical, ok := aliases[key]; ok {
			normalized[canonical] = true
			continue
		}
		normalized[key] = true
	}
	return normalized
}

func gatherMemorySourceResults(searchContent string, tc ToolCall, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, kg *memory.KnowledgeGraph, plannerDB *sql.DB, cheatsheetDB *sql.DB, perSourceLimit int, defaults map[string]bool, sourceLabels map[string]string, includeActivityRollups bool) memorySearchBundle {
	bundle := memorySearchBundle{
		Results:   make([]memorySourceResult, 0, 8),
		Errors:    make([]string, 0, 4),
		SourceMap: normalizeMemorySourceMap(tc.Sources, defaults),
	}

	if perSourceLimit <= 0 || perSourceLimit > 20 {
		perSourceLimit = 5
	}

	labelFor := func(source string) string {
		if label, ok := sourceLabels[source]; ok && strings.TrimSpace(label) != "" {
			return label
		}
		return source
	}

	bundle.NormalizedQuery = searchContent
	if temporalRange, cleanedQuery, hasTemporalRange := memory.ParseTemporalQuery(searchContent); hasTemporalRange {
		bundle.TemporalRange = temporalRange
		bundle.NormalizedQuery = cleanedQuery
		bundle.HasTemporalRange = true
	}
	searchContent = strings.TrimSpace(bundle.NormalizedQuery)
	hasSemanticQuery := searchContent != ""

	if bundle.SourceMap["activity"] && shortTermMem != nil {
		appendActivityResults(shortTermMem, searchContent, bundle.TemporalRange.FromDate, bundle.TemporalRange.ToDate, perSourceLimit, &bundle.Results, &bundle.Errors)
		if len(bundle.Results) > 0 {
			bundle.Results[len(bundle.Results)-1].Source = labelFor("activity")
		}
		if includeActivityRollups && (isActivityFocusedQuery(tc.Query) || isActivityFocusedQuery(tc.Content) || bundle.HasTemporalRange) {
			appendActivityRollupResults(shortTermMem, 7, &bundle.Results, &bundle.Errors)
		}
	}

	if bundle.SourceMap["ltm"] && longTermMem != nil && hasSemanticQuery {
		results, _, err := longTermMem.SearchMemoriesOnly(searchContent, perSourceLimit)
		if err != nil {
			bundle.Errors = append(bundle.Errors, fmt.Sprintf("%s: %v", labelFor("ltm"), err))
		} else if len(results) > 0 {
			bundle.Results = append(bundle.Results, memorySourceResult{Source: labelFor("ltm"), Count: len(results), Data: results})
		}
	}

	if bundle.SourceMap["kg"] && kg != nil && hasSemanticQuery {
		kgResult := kg.SearchForContext(searchContent, perSourceLimit, 2000)
		if kgResult != "" && kgResult != "No matching entities found." {
			bundle.Results = append(bundle.Results, memorySourceResult{Source: labelFor("kg"), Count: 1, Data: kgResult})
		}
	}

	if bundle.SourceMap["journal"] && shortTermMem != nil {
		entries, err := shortTermMem.SearchJournalEntriesInRange(searchContent, bundle.TemporalRange.FromDate, bundle.TemporalRange.ToDate, perSourceLimit)
		if err != nil {
			bundle.Errors = append(bundle.Errors, fmt.Sprintf("journal: %v", err))
		} else if len(entries) > 0 {
			bundle.Results = append(bundle.Results, memorySourceResult{Source: labelFor("journal"), Count: len(entries), Data: entries})
		}
	}

	if bundle.SourceMap["episodic"] && shortTermMem != nil {
		entries, err := shortTermMem.SearchEpisodicMemoriesInRange(searchContent, bundle.TemporalRange.FromDate, bundle.TemporalRange.ToDate, perSourceLimit)
		if err != nil {
			bundle.Errors = append(bundle.Errors, fmt.Sprintf("episodic: %v", err))
		} else if len(entries) > 0 {
			bundle.Results = append(bundle.Results, memorySourceResult{Source: labelFor("episodic"), Count: len(entries), Data: entries})
		}
	}

	if bundle.SourceMap["notes"] && shortTermMem != nil && hasSemanticQuery {
		notes, err := shortTermMem.SearchNotes(searchContent, perSourceLimit)
		if err != nil {
			bundle.Errors = append(bundle.Errors, fmt.Sprintf("notes: %v", err))
		} else if len(notes) > 0 {
			bundle.Results = append(bundle.Results, memorySourceResult{Source: labelFor("notes"), Count: len(notes), Data: notes})
		}
	}

	if bundle.SourceMap["planner"] && plannerDB != nil {
		appendPlannerResults(plannerDB, searchContent, tc.TimeRange, perSourceLimit, &bundle.Results, &bundle.Errors)
		if len(bundle.Results) > 0 {
			lastIdx := len(bundle.Results) - 1
			if bundle.Results[lastIdx].Source == "planner" {
				bundle.Results[lastIdx].Source = labelFor("planner")
			}
		}
	}

	if bundle.SourceMap["cheatsheets"] && cheatsheetDB != nil && hasSemanticQuery {
		hits := searchReusableCheatsheets(cheatsheetDB, searchContent, perSourceLimit)
		if len(hits) > 0 {
			sheets := make([]tools.CheatSheet, 0, len(hits))
			for _, hit := range hits {
				if hit.Cheatsheet != nil {
					sheets = append(sheets, *hit.Cheatsheet)
				}
			}
			if len(sheets) > 0 {
				bundle.Results = append(bundle.Results, memorySourceResult{Source: labelFor("cheatsheets"), Count: len(sheets), Data: sheets})
			}
		}
	}

	if bundle.SourceMap["core"] && shortTermMem != nil && hasSemanticQuery {
		facts, err := shortTermMem.GetCoreMemoryFacts()
		if err != nil {
			bundle.Errors = append(bundle.Errors, fmt.Sprintf("%s: %v", labelFor("core"), err))
		} else {
			lowerQ := strings.ToLower(searchContent)
			matched := make([]memory.CoreMemoryFact, 0, perSourceLimit)
			for _, fact := range facts {
				if strings.Contains(strings.ToLower(fact.Fact), lowerQ) {
					matched = append(matched, fact)
					if len(matched) >= perSourceLimit {
						break
					}
				}
			}
			if len(matched) > 0 {
				bundle.Results = append(bundle.Results, memorySourceResult{Source: labelFor("core"), Count: len(matched), Data: matched})
			}
		}
	}

	if bundle.SourceMap["error_patterns"] && shortTermMem != nil && hasSemanticQuery {
		errPatterns, err := shortTermMem.GetFrequentErrors("", perSourceLimit)
		if err != nil {
			bundle.Errors = append(bundle.Errors, fmt.Sprintf("error_patterns: %v", err))
		} else {
			lowerQ := strings.ToLower(searchContent)
			matched := make([]memory.ErrorPattern, 0, perSourceLimit)
			for _, ep := range errPatterns {
				if strings.Contains(strings.ToLower(ep.ToolName), lowerQ) || strings.Contains(strings.ToLower(ep.ErrorMessage), lowerQ) || strings.Contains(strings.ToLower(ep.Resolution), lowerQ) {
					matched = append(matched, ep)
					if len(matched) >= perSourceLimit {
						break
					}
				}
			}
			if len(matched) > 0 {
				bundle.Results = append(bundle.Results, memorySourceResult{Source: labelFor("error_patterns"), Count: len(matched), Data: matched})
			}
		}
	}

	return bundle
}

func executeQueryMemory(tc ToolCall, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, kg *memory.KnowledgeGraph, plannerDB *sql.DB, cheatsheetDB *sql.DB) (string, error) {
	searchContent := tc.Content
	if searchContent == "" {
		searchContent = tc.Query
	}
	if searchContent == "" {
		return `Tool Output: {"status": "error", "message": "'content' or 'query' (search query) is required"}`, nil
	}

	bundle := gatherMemorySourceResults(
		searchContent,
		tc,
		shortTermMem,
		longTermMem,
		kg,
		plannerDB,
		cheatsheetDB,
		tc.Limit,
		map[string]bool{"activity": true, "ltm": true, "kg": true, "journal": true, "episodic": true, "notes": true, "planner": true, "core": true, "cheatsheets": true, "error_patterns": true},
		map[string]string{"activity": "activity", "ltm": "vector_db", "kg": "knowledge_graph", "journal": "journal", "episodic": "episodic", "notes": "notes", "planner": "planner", "core": "core_memory", "cheatsheets": "cheatsheets", "error_patterns": "error_patterns"},
		true,
	)

	if len(bundle.Results) == 0 && len(bundle.Errors) == 0 {
		return `Tool Output: {"status": "success", "message": "No matching memories found across any source."}`, nil
	}

	response := map[string]interface{}{
		"status":  "success",
		"results": bundle.Results,
	}
	if bundle.HasTemporalRange {
		response["temporal_range"] = bundle.TemporalRange
		if bundle.NormalizedQuery != "" && bundle.NormalizedQuery != tc.Query && bundle.NormalizedQuery != tc.Content {
			response["normalized_query"] = bundle.NormalizedQuery
		}
	}
	if len(bundle.Errors) > 0 {
		response["errors"] = bundle.Errors
	}
	raw, err := json.Marshal(response)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to serialize results: %v"}`, err), nil
	}
	return "Tool Output: " + string(raw), nil
}

func executeContextMemoryQuery(tc ToolCall, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, kg *memory.KnowledgeGraph, plannerDB *sql.DB, cheatsheetDB *sql.DB) (string, error) {
	if shortTermMem == nil && longTermMem == nil && kg == nil && plannerDB == nil && cheatsheetDB == nil {
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

	sourceMap := normalizeMemorySourceMap(tc.Sources, map[string]bool{
		"activity": true, "journal": true, "notes": true, "planner": true, "core": true, "kg": true, "ltm": true, "cheatsheets": true,
	})

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

	bundle := gatherMemorySourceResults(
		query,
		tc,
		shortTermMem,
		longTermMem,
		kg,
		plannerDB,
		cheatsheetDB,
		perSourceLimit,
		map[string]bool{"activity": true, "journal": true, "notes": true, "planner": true, "core": true, "kg": true, "ltm": true, "cheatsheets": true},
		map[string]string{"activity": "activity", "journal": "journal", "notes": "notes", "planner": "planner", "core": "core", "kg": "kg", "ltm": "ltm", "cheatsheets": "cheatsheets"},
		false,
	)

	for _, result := range bundle.Results {
		switch result.Source {
		case "activity":
			entries, ok := result.Data.([]memory.ActivityTurn)
			if !ok {
				continue
			}
			for _, entry := range entries {
				addResult("activity", "turn", entry.Intent+" | "+strings.Join(entry.Outcomes, " | "), entry.Date, "Captured turn summary", "", 0.95)
			}
		case "journal":
			entries, ok := result.Data.([]memory.JournalEntry)
			if !ok {
				continue
			}
			for _, entry := range entries {
				addResult("journal", entry.EntryType, entry.Title+" | "+entry.Content, entry.Date, "Journal event matched query/time window", "", 0.9)
			}
		case "notes":
			notes, ok := result.Data.([]memory.Note)
			if !ok {
				continue
			}
			for _, note := range notes {
				addResult("notes", "note", note.Title+" | "+note.Content, note.DueDate, "Open or stored note matched the query", "", 0.88)
			}
		case "planner":
			payload, ok := result.Data.(plannerMemoryPayload)
			if !ok {
				continue
			}
			if payload.Summary != "" {
				addResult("planner", "planner_summary", payload.Summary, "", "Structured planner overview for active tasks and appointments", "", 0.93)
			}
			for _, todo := range payload.Todos {
				content := todo.Title
				if strings.TrimSpace(todo.Description) != "" {
					content += " | " + todo.Description
				}
				if todo.DueDate != "" {
					content += " | due: " + todo.DueDate
				}
				addResult("planner", "todo", content, todo.DueDate, "Open planner todo matched the query or active time window", todo.ID, 0.91)
			}
			for _, appointment := range payload.Appointments {
				content := appointment.Title
				if strings.TrimSpace(appointment.Description) != "" {
					content += " | " + appointment.Description
				}
				content += " | at: " + appointment.DateTime
				addResult("planner", "appointment", content, appointment.DateTime, "Relevant planner appointment matched the query or active time window", appointment.ID, 0.9)
			}
		case "core":
			facts, ok := result.Data.([]memory.CoreMemoryFact)
			if !ok {
				continue
			}
			for _, fact := range facts {
				addResult("core", "fact", fact.Fact, "", "Persistent core memory fact", fmt.Sprintf("%d", fact.ID), 0.85)
				if len(results) >= 20 {
					break
				}
			}
		case "kg":
			kgResult, ok := result.Data.(string)
			if !ok {
				continue
			}
			addResult("kg", "entity_network", kgResult, "", "Structured relationships from the knowledge graph", "", 0.92)
		case "ltm":
			memories, ok := result.Data.([]string)
			if !ok {
				continue
			}
			for i, memText := range memories {
				addResult("ltm", "document", memText, "", "Semantic long-term memory hit", "", 0.89-float64(i)*0.05)
			}
		case "cheatsheets":
			sheets, ok := result.Data.([]tools.CheatSheet)
			if !ok {
				continue
			}
			for i, sheet := range sheets {
				addResult("cheatsheets", "cheatsheet", sheet.Name+" | "+trimForPrompt(sheet.Content, 180), sheet.UpdatedAt, "Reusable cheatsheet matched the query", sheet.ID, 0.9-float64(i)*0.04)
			}
		}
	}

	if sourceMap["notes"] && len(query) == 0 {
		if notes, err := shortTermMem.GetHighPriorityOpenNotes(perSourceLimit); err == nil {
			for _, note := range notes {
				addResult("notes", "note", note.Title+" | "+note.Content, note.DueDate, "High-priority open note", "", 0.86)
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
