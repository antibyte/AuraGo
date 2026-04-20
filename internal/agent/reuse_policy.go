package agent

import (
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"aurago/internal/memory"
	"aurago/internal/tools"
)

type TaskComplexity string

const (
	TaskComplexityTrivial    TaskComplexity = "trivial"
	TaskComplexityNonTrivial TaskComplexity = "non_trivial"
)

type ReusableArtifactDecision string

const (
	ReusableArtifactNone                          ReusableArtifactDecision = "none"
	ReusableArtifactCreateCheatsheet              ReusableArtifactDecision = "create_cheatsheet"
	ReusableArtifactCreateSkill                   ReusableArtifactDecision = "create_skill"
	ReusableArtifactCreateBoth                    ReusableArtifactDecision = "create_both"
	ReusableArtifactUpdateExistingAgentCheatsheet ReusableArtifactDecision = "update_existing_agent_cheatsheet"
	ReusableArtifactUpdateExistingAgentSkill      ReusableArtifactDecision = "update_existing_agent_skill"
	ReusableArtifactUpdateBoth                    ReusableArtifactDecision = "update_both"
)

type reuseArtifactHit struct {
	Kind       string                    `json:"kind"`
	ID         string                    `json:"id,omitempty"`
	Name       string                    `json:"name"`
	Ownership  string                    `json:"ownership"`
	Score      float64                   `json:"score"`
	Reason     string                    `json:"reason"`
	Excerpt    string                    `json:"excerpt,omitempty"`
	Cheatsheet *tools.CheatSheet         `json:"-"`
	Skill      *tools.SkillRegistryEntry `json:"-"`
}

type ReuseLookupResult struct {
	Query            string             `json:"query"`
	Complexity       TaskComplexity     `json:"complexity"`
	Performed        bool               `json:"performed"`
	CheatsheetHits   []reuseArtifactHit `json:"cheatsheet_hits,omitempty"`
	SkillHits        []reuseArtifactHit `json:"skill_hits,omitempty"`
	JournalHits      []reuseArtifactHit `json:"journal_hits,omitempty"`
	ErrorPatternHits []reuseArtifactHit `json:"error_pattern_hits,omitempty"`
	BestMatch        *reuseArtifactHit  `json:"best_match,omitempty"`
	WhyRelevant      []string           `json:"why_relevant,omitempty"`
	Gap              string             `json:"reuse_gap,omitempty"`
	Prompt           string             `json:"prompt,omitempty"`
}

type ReusabilityEvaluation struct {
	Decision                ReusableArtifactDecision
	Reason                  string
	HighRecurrence          bool
	ExistingAgentCheatsheet *tools.CheatSheet
	ExistingAgentSkill      *tools.SkillRegistryEntry
	TemplateName            string
	SkillName               string
	SkillDescription        string
	SkillCategory           string
	SkillTags               []string
	CheatsheetName          string
	CheatsheetContent       string
}

var (
	reuseSplitter  = regexp.MustCompile(`[^a-z0-9]+`)
	reuseStopWords = map[string]struct{}{
		"a": {}, "an": {}, "and": {}, "are": {}, "bitte": {}, "das": {}, "dem": {}, "den": {}, "der": {}, "die": {},
		"ein": {}, "eine": {}, "einer": {}, "eines": {}, "einem": {}, "es": {}, "for": {}, "from": {}, "have": {},
		"ich": {}, "ist": {}, "mit": {}, "oder": {}, "please": {}, "the": {}, "this": {}, "und": {}, "was": {}, "wie": {},
	}
	reuseNonTrivialCues = []string{
		"analyse", "analyze", "automation", "automate", "backup", "broken", "bug", "build", "cheatsheet",
		"configure", "connect", "crash", "debug", "deploy", "deployment", "error", "failure", "fix", "failing",
		"integration", "issue", "log", "migration", "monitor", "optimize", "ops", "panic", "problem", "reproduce",
		"restore", "runbook", "script", "setup", "skill", "stacktrace", "troubleshoot", "workflow",
		"fehler", "fixen", "integration", "kaputt", "konzept", "problem", "reproduzier", "wiederkehr",
	}
	reuseRecurringCues = []string{
		"api", "backup", "build", "connect", "container", "database", "debug", "deploy", "docker", "error",
		"fix", "integration", "issue", "log", "monitor", "panic", "problem", "remote", "restore", "setup",
		"ssh", "sync", "timeout", "workflow", "fehler", "deploy", "docker", "konfigur", "ssh", "wiederkehr",
	}
	reuseTrivialTaskCues = []string{
		"test", "teste", "testing", "smoke test", "check", "verify", "confirm", "try again", "try",
		"versuch", "pruef", "pruf", "nochmal", "erneut", "quick", "kurz",
	}
	reuseFailureCues = []string{
		"broken", "bug", "crash", "error", "failure", "failing", "issue", "panic", "problem", "regression",
		"timeout", "fehler", "kaputt", "funktioniert nicht", "schlaegt fehl",
	}
	reuseResolutionCues = []string{
		"fixed", "resolved", "solution", "solved", "workaround", "root cause", "mitigated", "recovered",
		"gefixed", "geloest", "loesung", "ursache", "behoben",
	}
	reuseAutomationCues = []string{
		"automate", "automation", "automated", "script", "workflow", "runbook", "recurring", "repeatable",
		"scheduled", "schedule", "pipeline", "routine", "batch", "wiederkehr", "automatisier",
	}
)

func classifyTaskComplexity(userMsg string) TaskComplexity {
	trimmed := strings.TrimSpace(strings.ToLower(userMsg))
	if trimmed == "" {
		return TaskComplexityTrivial
	}

	tokens := reuseKeywords(trimmed)
	if len(tokens) >= 8 {
		return TaskComplexityNonTrivial
	}

	for _, cue := range reuseNonTrivialCues {
		if strings.Contains(trimmed, cue) {
			return TaskComplexityNonTrivial
		}
	}

	if strings.Contains(trimmed, " and ") || strings.Contains(trimmed, " then ") || strings.Contains(trimmed, " danach ") || strings.Contains(trimmed, " sowie ") {
		return TaskComplexityNonTrivial
	}

	if strings.Count(trimmed, ",") >= 2 || strings.Count(trimmed, "\n") >= 2 {
		return TaskComplexityNonTrivial
	}

	if len(tokens) <= 4 {
		return TaskComplexityTrivial
	}

	return TaskComplexityNonTrivial
}

func buildReuseLookup(query string, shortTermMem *memory.SQLiteMemory, cheatsheetDB *sql.DB, logger *slog.Logger) ReuseLookupResult {
	result := ReuseLookupResult{
		Query:      strings.TrimSpace(query),
		Complexity: classifyTaskComplexity(query),
	}
	if result.Complexity != TaskComplexityNonTrivial || result.Query == "" {
		result.Gap = "Task classified as trivial; skip reuse lookup."
		return result
	}

	result.Performed = true
	result.CheatsheetHits = searchReusableCheatsheets(cheatsheetDB, result.Query, 3)
	result.SkillHits = searchReusableSkills(result.Query, 3)
	result.JournalHits = searchReusableJournal(shortTermMem, result.Query, 2)
	result.ErrorPatternHits = searchReusableErrorPatterns(shortTermMem, result.Query, 2)
	result.BestMatch = pickBestReuseMatch(result)
	result.WhyRelevant = collectReuseReasons(result)
	result.Gap = deriveReuseGap(result)
	result.Prompt = renderReusePrompt(result)

	if logger != nil {
		logger.Info("[ReuseFirst] Lookup completed",
			"task_non_trivial", true,
			"lookup_performed", true,
			"matched_cheatsheet", topReuseName(result.CheatsheetHits),
			"matched_skill", topReuseName(result.SkillHits),
			"reuse_gap", result.Gap)
	}

	return result
}

func searchReusableCheatsheets(db *sql.DB, query string, limit int) []reuseArtifactHit {
	if db == nil || strings.TrimSpace(query) == "" {
		return nil
	}
	sheets, err := tools.CheatsheetList(db, true)
	if err != nil {
		return nil
	}
	errorRecoveryQuery := isErrorRecoveryQuery(query)
	hits := make([]reuseArtifactHit, 0, len(sheets))
	for i := range sheets {
		if !errorRecoveryQuery && isErrorRecoveryCheatsheet(sheets[i].Name, sheets[i].Content) {
			continue
		}
		score, matched := scoreReuseCandidate(query, sheets[i].Name, sheets[i].Content)
		if score < 0.26 {
			continue
		}
		hits = append(hits, reuseArtifactHit{
			Kind:       "cheatsheet",
			ID:         sheets[i].ID,
			Name:       sheets[i].Name,
			Ownership:  normalizedArtifactOwnership(sheets[i].CreatedBy),
			Score:      score,
			Reason:     matchedReason(matched),
			Excerpt:    trimForPrompt(reuseFirstNonEmpty(sheets[i].Content), 280),
			Cheatsheet: &sheets[i],
		})
	}
	sortReuseHits(hits)
	if limit > 0 && len(hits) > limit {
		return hits[:limit]
	}
	return hits
}

func isErrorRecoveryQuery(query string) bool {
	text := strings.TrimSpace(strings.ToLower(query))
	if text == "" {
		return false
	}
	if strings.Contains(text, "error: your last response") ||
		strings.Contains(text, "text-only") ||
		strings.Contains(text, "function-calling api") ||
		strings.Contains(text, "tool call") {
		return true
	}
	for _, cue := range reuseFailureCues {
		if strings.Contains(text, cue) {
			return true
		}
	}
	return false
}

func isErrorRecoveryCheatsheet(name, content string) bool {
	nameText := strings.TrimSpace(strings.ToLower(name))
	text := strings.TrimSpace(strings.ToLower(name + "\n" + content))
	return strings.HasPrefix(nameText, "error ") ||
		strings.Contains(text, "error: your last response") ||
		strings.Contains(text, "text-only") ||
		strings.Contains(text, "native function-calling api") ||
		strings.Contains(text, "emit the tool call again") ||
		strings.Contains(text, "completion signal")
}

func searchReusableSkills(query string, limit int) []reuseArtifactHit {
	manager := tools.DefaultSkillManager()
	if manager == nil || strings.TrimSpace(query) == "" {
		return nil
	}
	enabled := true
	skills, err := manager.ListSkillsFiltered("", "", "", &enabled)
	if err != nil {
		return nil
	}
	hits := make([]reuseArtifactHit, 0, len(skills))
	for i := range skills {
		score, matched := scoreReuseCandidate(query, skills[i].Name, skills[i].Description, skills[i].Category, strings.Join(skills[i].Tags, " "))
		if score < 0.24 {
			continue
		}
		excerpt := skills[i].Description
		if skills[i].Category != "" {
			excerpt = fmt.Sprintf("%s (category: %s)", excerpt, skills[i].Category)
		}
		hits = append(hits, reuseArtifactHit{
			Kind:      "skill",
			ID:        skills[i].ID,
			Name:      skills[i].Name,
			Ownership: normalizedArtifactOwnership(skills[i].CreatedBy),
			Score:     score,
			Reason:    matchedReason(matched),
			Excerpt:   trimForPrompt(excerpt, 220),
			Skill:     &skills[i],
		})
	}
	sortReuseHits(hits)
	if limit > 0 && len(hits) > limit {
		return hits[:limit]
	}
	return hits
}

func searchReusableJournal(shortTermMem *memory.SQLiteMemory, query string, limit int) []reuseArtifactHit {
	if shortTermMem == nil || strings.TrimSpace(query) == "" {
		return nil
	}
	entries, err := shortTermMem.SearchJournalEntriesInRange(query, "", "", limit)
	if err != nil {
		return nil
	}
	hits := make([]reuseArtifactHit, 0, len(entries))
	for _, entry := range entries {
		score, matched := scoreReuseCandidate(query, entry.Title, entry.Content, strings.Join(entry.Tags, " "))
		if score < 0.22 {
			continue
		}
		hits = append(hits, reuseArtifactHit{
			Kind:      "journal",
			ID:        fmt.Sprintf("%d", entry.ID),
			Name:      reuseFirstNonEmpty(entry.Title, entry.EntryType, "Journal entry"),
			Ownership: "system",
			Score:     score,
			Reason:    matchedReason(matched),
			Excerpt:   trimForPrompt(entry.Content, 220),
		})
	}
	sortReuseHits(hits)
	return hits
}

func searchReusableErrorPatterns(shortTermMem *memory.SQLiteMemory, query string, limit int) []reuseArtifactHit {
	if shortTermMem == nil || strings.TrimSpace(query) == "" {
		return nil
	}
	patterns, err := shortTermMem.GetFrequentErrors("", reuseMax(limit*2, 6))
	if err != nil {
		return nil
	}
	hits := make([]reuseArtifactHit, 0, len(patterns))
	for _, pattern := range patterns {
		score, matched := scoreReuseCandidate(query, pattern.ToolName, pattern.ErrorMessage, pattern.Resolution)
		if score < 0.20 {
			continue
		}
		hits = append(hits, reuseArtifactHit{
			Kind:      "error_pattern",
			Name:      reuseFirstNonEmpty(pattern.ToolName, "Error pattern"),
			Ownership: "system",
			Score:     score,
			Reason:    matchedReason(matched),
			Excerpt:   trimForPrompt(reuseFirstNonEmpty(pattern.Resolution, pattern.ErrorMessage), 220),
		})
	}
	sortReuseHits(hits)
	if limit > 0 && len(hits) > limit {
		return hits[:limit]
	}
	return hits
}

func pickBestReuseMatch(result ReuseLookupResult) *reuseArtifactHit {
	candidates := make([]reuseArtifactHit, 0, len(result.CheatsheetHits)+len(result.SkillHits)+len(result.JournalHits)+len(result.ErrorPatternHits))
	candidates = append(candidates, result.CheatsheetHits...)
	candidates = append(candidates, result.SkillHits...)
	candidates = append(candidates, result.JournalHits...)
	candidates = append(candidates, result.ErrorPatternHits...)
	if len(candidates) == 0 {
		return nil
	}
	sortReuseHits(candidates)
	best := candidates[0]
	return &best
}

func collectReuseReasons(result ReuseLookupResult) []string {
	reasons := make([]string, 0, 4)
	if len(result.CheatsheetHits) > 0 {
		reasons = append(reasons, fmt.Sprintf("Cheatsheet match: %s", result.CheatsheetHits[0].Name))
	}
	if len(result.SkillHits) > 0 {
		reasons = append(reasons, fmt.Sprintf("Skill match: %s", result.SkillHits[0].Name))
	}
	if len(result.JournalHits) > 0 {
		reasons = append(reasons, fmt.Sprintf("Related journal memory: %s", result.JournalHits[0].Name))
	}
	if len(result.ErrorPatternHits) > 0 {
		reasons = append(reasons, fmt.Sprintf("Known error pattern: %s", result.ErrorPatternHits[0].Name))
	}
	return reasons
}

func deriveReuseGap(result ReuseLookupResult) string {
	switch {
	case len(result.CheatsheetHits) == 0 && len(result.SkillHits) == 0:
		return "No direct cheatsheet or skill match found."
	case len(result.CheatsheetHits) == 0:
		return "No cheatsheet match found."
	case len(result.SkillHits) == 0:
		return "No skill match found."
	default:
		return "Existing reusable artifacts found."
	}
}

func renderReusePrompt(result ReuseLookupResult) string {
	if !result.Performed {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("This request is classified as non-trivial. Reuse or adapt existing cheatsheets/skills before inventing a fresh path.\n")
	if result.BestMatch != nil {
		sb.WriteString(fmt.Sprintf("Best current match: %s `%s` (%s, score %.2f). %s\n",
			result.BestMatch.Kind, result.BestMatch.Name, result.BestMatch.Ownership, result.BestMatch.Score, result.BestMatch.Reason))
	}
	if len(result.CheatsheetHits) > 0 {
		sb.WriteString("Relevant cheatsheets:\n")
		for _, hit := range result.CheatsheetHits[:reuseMin(len(result.CheatsheetHits), 2)] {
			sb.WriteString(fmt.Sprintf("- `%s` (%s): %s\n", hit.Name, hit.Ownership, trimForPrompt(hit.Excerpt, 180)))
		}
	}
	if len(result.SkillHits) > 0 {
		sb.WriteString("Relevant skills:\n")
		for _, hit := range result.SkillHits[:reuseMin(len(result.SkillHits), 2)] {
			sb.WriteString(fmt.Sprintf("- `%s` (%s): %s\n", hit.Name, hit.Ownership, trimForPrompt(hit.Excerpt, 160)))
		}
	}
	if len(result.JournalHits) > 0 || len(result.ErrorPatternHits) > 0 {
		sb.WriteString("Supporting historical signals:\n")
		if len(result.JournalHits) > 0 {
			hit := result.JournalHits[0]
			sb.WriteString(fmt.Sprintf("- journal: `%s` (%s)\n", hit.Name, hit.Reason))
		}
		if len(result.ErrorPatternHits) > 0 {
			hit := result.ErrorPatternHits[0]
			sb.WriteString(fmt.Sprintf("- error pattern: `%s` (%s)\n", hit.Name, hit.Reason))
		}
	}
	if len(result.CheatsheetHits) == 0 && len(result.SkillHits) == 0 {
		sb.WriteString("No direct reusable artifact match was found. If you solve it successfully and it is likely to recur, create or refine an agent-owned cheatsheet or skill afterwards.\n")
	} else {
		sb.WriteString("Prefer the strongest match first. Only create a new artifact if the existing one is clearly insufficient or missing.\n")
	}
	return strings.TrimSpace(sb.String())
}

func evaluateReusability(query, finalAnswer string, toolNames, toolSummaries []string, lookup ReuseLookupResult) ReusabilityEvaluation {
	eval := ReusabilityEvaluation{
		Decision: ReusableArtifactNone,
		Reason:   "not_evaluated",
	}
	if lookup.Complexity != TaskComplexityNonTrivial {
		eval.Reason = "task_trivial"
		return eval
	}
	if isTrivialReuseTask(query, finalAnswer, toolNames, toolSummaries) {
		eval.Reason = "task_not_substantial_enough"
		return eval
	}
	if !isLikelyRecurringResolution(query, finalAnswer, toolNames, toolSummaries) {
		eval.Reason = "not_likely_recurring"
		return eval
	}

	eval.HighRecurrence = true
	needsCheatsheet := shouldMaterializeCheatsheet(query, finalAnswer, toolNames, toolSummaries)
	templateName, category, tags := deriveReusableSkillTemplate(query, finalAnswer, toolNames)
	needsSkill := shouldMaterializeSkill(query, finalAnswer, toolNames, toolSummaries) && templateName != ""

	existingAgentCS := topAgentCheatsheet(lookup.CheatsheetHits)
	existingAgentSkill := topAgentSkill(lookup.SkillHits)

	eval.ExistingAgentCheatsheet = existingAgentCS
	eval.ExistingAgentSkill = existingAgentSkill

	if !needsCheatsheet && !needsSkill {
		eval.Reason = "no_reusable_artifact_type"
		return eval
	}

	if needsCheatsheet {
		eval.CheatsheetName = deriveReusableCheatsheetName(query, toolNames)
		eval.CheatsheetContent = composeReusableCheatsheetContent(query, finalAnswer, toolNames, toolSummaries)
	}
	if needsSkill {
		eval.TemplateName = templateName
		eval.SkillCategory = category
		eval.SkillTags = tags
		eval.SkillName = deriveReusableSkillName(query, templateName)
		eval.SkillDescription = deriveReusableSkillDescription(query, templateName)
	}

	switch {
	case needsCheatsheet && needsSkill:
		if existingAgentCS != nil || existingAgentSkill != nil {
			eval.Decision = ReusableArtifactUpdateBoth
			eval.Reason = "mixed_reusable_update"
		} else {
			eval.Decision = ReusableArtifactCreateBoth
			eval.Reason = "create_cheatsheet_and_skill"
		}
	case needsCheatsheet:
		if existingAgentCS != nil {
			eval.Decision = ReusableArtifactUpdateExistingAgentCheatsheet
			eval.Reason = "refine_existing_agent_cheatsheet"
		} else {
			eval.Decision = ReusableArtifactCreateCheatsheet
			eval.Reason = "create_agent_cheatsheet"
		}
	case needsSkill:
		if existingAgentSkill != nil {
			eval.Decision = ReusableArtifactUpdateExistingAgentSkill
			eval.Reason = "refine_existing_agent_skill"
		} else {
			eval.Decision = ReusableArtifactCreateSkill
			eval.Reason = "create_agent_skill"
		}
	}

	return eval
}

func applyReusabilityDecision(runCfg RunConfig, logger *slog.Logger, evaluation ReusabilityEvaluation) error {
	if evaluation.Decision == ReusableArtifactNone {
		return nil
	}
	if logger != nil {
		logger.Info("[ReuseFirst] Reusability evaluated",
			"reuse_decision", evaluation.Decision,
			"reason", evaluation.Reason,
			"cheatsheet_name", evaluation.CheatsheetName,
			"skill_name", evaluation.SkillName,
			"template_name", evaluation.TemplateName)
	}

	if needsCheatsheetMutation(evaluation.Decision) {
		if err := applyReusableCheatsheet(runCfg, logger, evaluation); err != nil {
			return err
		}
	}
	if needsSkillMutation(evaluation.Decision) {
		if err := applyReusableSkill(runCfg, logger, evaluation); err != nil {
			return err
		}
	}
	return nil
}

func applyReusableCheatsheet(runCfg RunConfig, logger *slog.Logger, evaluation ReusabilityEvaluation) error {
	if runCfg.CheatsheetDB == nil || strings.TrimSpace(evaluation.CheatsheetContent) == "" || strings.TrimSpace(evaluation.CheatsheetName) == "" {
		return nil
	}

	switch evaluation.Decision {
	case ReusableArtifactCreateCheatsheet, ReusableArtifactCreateBoth:
		createName := evaluation.CheatsheetName
		if existing, err := tools.CheatsheetGetByName(runCfg.CheatsheetDB, createName); err == nil {
			if existing.CreatedBy == "agent" {
				merged := mergeReusableCheatsheetContent(existing.Content, evaluation.CheatsheetContent)
				if strings.TrimSpace(merged) == strings.TrimSpace(existing.Content) {
					return nil
				}
				content := merged
				updated, updateErr := tools.CheatsheetUpdate(runCfg.CheatsheetDB, existing.ID, nil, &content, nil)
				if updateErr != nil {
					return fmt.Errorf("update reusable cheatsheet after name collision: %w", updateErr)
				}
				if storeErr := tools.ReindexCheatsheetInVectorDB(runCfg.CheatsheetDB, runCfg.LongTermMem, updated.ID); storeErr != nil && logger != nil {
					logger.Warn("Failed to reindex reusable cheatsheet", "cs_id", updated.ID, "error", storeErr)
				}
				if runCfg.PreparationService != nil {
					runCfg.PreparationService.InvalidateByCheatsheet(updated.ID)
				}
				return nil
			}
			createName = createName + " (Agent)"
		}
		sheet, err := tools.CheatsheetCreate(runCfg.CheatsheetDB, createName, evaluation.CheatsheetContent, "agent")
		if err != nil {
			return fmt.Errorf("create reusable cheatsheet: %w", err)
		}
		if storeErr := tools.ReindexCheatsheetInVectorDB(runCfg.CheatsheetDB, runCfg.LongTermMem, sheet.ID); storeErr != nil && logger != nil {
			logger.Warn("Failed to index reusable cheatsheet", "cs_id", sheet.ID, "error", storeErr)
		}
		if runCfg.PreparationService != nil {
			runCfg.PreparationService.InvalidateByCheatsheet(sheet.ID)
		}
	case ReusableArtifactUpdateExistingAgentCheatsheet, ReusableArtifactUpdateBoth:
		if evaluation.ExistingAgentCheatsheet == nil || evaluation.ExistingAgentCheatsheet.CreatedBy != "agent" {
			return nil
		}
		merged := mergeReusableCheatsheetContent(evaluation.ExistingAgentCheatsheet.Content, evaluation.CheatsheetContent)
		if strings.TrimSpace(merged) == strings.TrimSpace(evaluation.ExistingAgentCheatsheet.Content) {
			return nil
		}
		content := merged
		sheet, err := tools.CheatsheetUpdate(runCfg.CheatsheetDB, evaluation.ExistingAgentCheatsheet.ID, nil, &content, nil)
		if err != nil {
			return fmt.Errorf("update reusable cheatsheet: %w", err)
		}
		if storeErr := tools.ReindexCheatsheetInVectorDB(runCfg.CheatsheetDB, runCfg.LongTermMem, sheet.ID); storeErr != nil && logger != nil {
			logger.Warn("Failed to reindex reusable cheatsheet", "cs_id", sheet.ID, "error", storeErr)
		}
		if runCfg.PreparationService != nil {
			runCfg.PreparationService.InvalidateByCheatsheet(sheet.ID)
		}
	}
	return nil
}

func applyReusableSkill(runCfg RunConfig, logger *slog.Logger, evaluation ReusabilityEvaluation) error {
	manager := tools.DefaultSkillManager()
	if manager == nil {
		return nil
	}

	switch evaluation.Decision {
	case ReusableArtifactCreateSkill, ReusableArtifactCreateBoth:
		if evaluation.TemplateName == "" || evaluation.SkillName == "" {
			return nil
		}
		createName := evaluation.SkillName
		if existing := findSkillByName(manager, createName); existing != nil {
			if existing.CreatedBy == "agent" {
				return manager.UpdateSkillMetadata(existing.ID, reuseFirstNonEmpty(evaluation.SkillDescription, existing.Description), reuseFirstNonEmpty(evaluation.SkillCategory, existing.Category), mergeSkillTags(existing.Tags, evaluation.SkillTags), "agent")
			}
			createName = createName + "_agent"
		}
		if _, err := tools.CreateSkillFromTemplate(
			runCfg.Config.Directories.SkillsDir,
			evaluation.TemplateName,
			createName,
			evaluation.SkillDescription,
			"",
			nil,
			nil,
		); err != nil {
			return fmt.Errorf("create reusable skill: %w", err)
		}
		tools.ProvisionSkillDependencies(runCfg.Config.Directories.SkillsDir, runCfg.Config.Directories.WorkspaceDir, logger)
		if err := manager.SyncFromDisk(); err != nil {
			return fmt.Errorf("sync reusable skill registry: %w", err)
		}
		if skill := findSkillByName(manager, createName); skill != nil {
			_ = manager.EnsureInitialVersion(skill.ID, "agent", "reuse-first template creation")
			_ = manager.UpdateSkillMetadata(skill.ID, reuseFirstNonEmpty(evaluation.SkillDescription, skill.Description), evaluation.SkillCategory, mergeSkillTags(skill.Tags, evaluation.SkillTags), "agent")
		}
	case ReusableArtifactUpdateExistingAgentSkill, ReusableArtifactUpdateBoth:
		if evaluation.ExistingAgentSkill == nil || evaluation.ExistingAgentSkill.CreatedBy != "agent" {
			return nil
		}
		description := reuseFirstNonEmpty(evaluation.SkillDescription, evaluation.ExistingAgentSkill.Description)
		category := reuseFirstNonEmpty(evaluation.SkillCategory, evaluation.ExistingAgentSkill.Category)
		tags := mergeSkillTags(evaluation.ExistingAgentSkill.Tags, evaluation.SkillTags)
		if description == evaluation.ExistingAgentSkill.Description && category == evaluation.ExistingAgentSkill.Category && strings.Join(tags, ",") == strings.Join(evaluation.ExistingAgentSkill.Tags, ",") {
			return nil
		}
		if err := manager.UpdateSkillMetadata(evaluation.ExistingAgentSkill.ID, description, category, tags, "agent"); err != nil {
			return fmt.Errorf("update reusable skill metadata: %w", err)
		}
	}
	return nil
}

func shouldMaterializeCheatsheet(query, finalAnswer string, toolNames, toolSummaries []string) bool {
	if len(toolNames) == 0 {
		return false
	}
	if isTrivialReuseTask(query, finalAnswer, toolNames, toolSummaries) {
		return false
	}
	combined := normalizeReuseCombined(query, finalAnswer, toolNames, toolSummaries)
	stepCount := reusableExecutionDepth(toolNames, toolSummaries)
	if stepCount < 3 && !(stepCount >= 2 && containsReuseCue(combined, reuseFailureCues) && containsReuseCue(combined, reuseResolutionCues)) {
		return false
	}
	return containsReuseCue(combined, reuseFailureCues) && containsReuseCue(combined, reuseResolutionCues)
}

func shouldMaterializeSkill(query, finalAnswer string, toolNames, toolSummaries []string) bool {
	if len(toolNames) == 0 {
		return false
	}
	if isTrivialReuseTask(query, finalAnswer, toolNames, toolSummaries) {
		return false
	}
	combined := normalizeReuseCombined(query, finalAnswer, toolNames, toolSummaries)
	if reusableExecutionDepth(toolNames, toolSummaries) < 3 {
		return false
	}
	return containsReuseCue(combined, reuseAutomationCues)
}

func isLikelyRecurringResolution(query, finalAnswer string, toolNames, toolSummaries []string) bool {
	if len(toolNames) == 0 {
		return false
	}
	if isTrivialReuseTask(query, finalAnswer, toolNames, toolSummaries) {
		return false
	}
	combined := normalizeReuseCombined(query, finalAnswer, toolNames, toolSummaries)
	stepCount := reusableExecutionDepth(toolNames, toolSummaries)
	hasFailureResolution := containsReuseCue(combined, reuseFailureCues) && containsReuseCue(combined, reuseResolutionCues)
	hasAutomationIntent := containsReuseCue(combined, reuseAutomationCues)

	if stepCount >= 3 && (hasFailureResolution || hasAutomationIntent) {
		return true
	}
	if stepCount >= 2 && hasFailureResolution && len(reuseKeywords(combined)) >= 6 {
		return true
	}
	return false
}

func isTrivialReuseTask(query, finalAnswer string, toolNames, toolSummaries []string) bool {
	trimmedQuery := strings.ToLower(strings.TrimSpace(query))
	combined := normalizeReuseCombined(query, finalAnswer, toolNames, toolSummaries)
	stepCount := reusableExecutionDepth(toolNames, toolSummaries)
	if stepCount >= 3 {
		return false
	}
	if hasPrefixReuseCue(trimmedQuery, reuseTrivialTaskCues) {
		return true
	}
	if containsReuseCue(combined, reuseTrivialTaskCues) && !containsReuseCue(combined, reuseFailureCues) && !containsReuseCue(combined, reuseAutomationCues) {
		return true
	}
	return len(reuseKeywords(trimmedQuery)) <= 6 && stepCount <= 2
}

func normalizeReuseCombined(query, finalAnswer string, toolNames, toolSummaries []string) string {
	parts := make([]string, 0, 2+len(toolNames)+len(toolSummaries))
	parts = append(parts, query, finalAnswer)
	parts = append(parts, toolNames...)
	parts = append(parts, toolSummaries...)
	return strings.ToLower(strings.TrimSpace(strings.Join(parts, " ")))
}

func reusableExecutionDepth(toolNames, toolSummaries []string) int {
	nonEmptySummaries := 0
	for _, summary := range toolSummaries {
		if strings.TrimSpace(summary) != "" {
			nonEmptySummaries++
		}
	}
	uniqueToolCount := len(uniqueStrings(toolNames))
	if nonEmptySummaries > uniqueToolCount {
		return nonEmptySummaries
	}
	return uniqueToolCount
}

func containsReuseCue(combined string, cues []string) bool {
	for _, cue := range cues {
		if strings.Contains(combined, cue) {
			return true
		}
	}
	return false
}

func hasPrefixReuseCue(query string, cues []string) bool {
	for _, cue := range cues {
		if strings.HasPrefix(query, cue+" ") || query == cue {
			return true
		}
	}
	return false
}

func deriveReusableSkillTemplate(query, finalAnswer string, toolNames []string) (string, string, []string) {
	combined := strings.ToLower(strings.Join(append([]string{query, finalAnswer}, toolNames...), " "))
	switch {
	case strings.Contains(combined, "docker") || strings.Contains(combined, "container"):
		return "docker_manager", "ops", []string{"docker", "containers", "reuse-first"}
	case strings.Contains(combined, "log") || strings.Contains(combined, "stacktrace") || strings.Contains(combined, "traceback"):
		return "log_analyzer", "ops", []string{"logs", "debugging", "reuse-first"}
	case strings.Contains(combined, "backup") || strings.Contains(combined, "archive") || strings.Contains(combined, "restore"):
		return "backup_runner", "ops", []string{"backup", "restore", "reuse-first"}
	case strings.Contains(combined, "json") || strings.Contains(combined, "yaml") || strings.Contains(combined, "csv") || strings.Contains(combined, "xml"):
		return "data_transformer", "data", []string{"data", "transform", "reuse-first"}
	case strings.Contains(combined, "database") || strings.Contains(combined, "sql") || strings.Contains(combined, "query"):
		return "database_query", "data", []string{"database", "sql", "reuse-first"}
	case strings.Contains(combined, "ssh") || strings.Contains(combined, "remote"):
		return "ssh_executor", "ops", []string{"ssh", "remote", "reuse-first"}
	case strings.Contains(combined, "monitor") || strings.Contains(combined, "health") || strings.Contains(combined, "endpoint"):
		return "monitor_check", "ops", []string{"monitoring", "healthcheck", "reuse-first"}
	default:
		return "", "", nil
	}
}

func deriveReusableCheatsheetName(query string, toolNames []string) string {
	keywords := reuseKeywords(query)
	parts := make([]string, 0, 3)
	for _, token := range keywords {
		if len(parts) >= 3 {
			break
		}
		parts = append(parts, strings.Title(token))
	}
	if len(parts) == 0 && len(toolNames) > 0 {
		parts = append(parts, strings.Title(strings.ReplaceAll(toolNames[0], "_", " ")))
	}
	if len(parts) == 0 {
		parts = append(parts, "Recurring Workflow")
	}
	name := strings.Join(parts, " ")
	if !strings.Contains(strings.ToLower(name), "workflow") && !strings.Contains(strings.ToLower(name), "fix") {
		name += " Workflow"
	}
	return trimForPrompt(name, 90)
}

func composeReusableCheatsheetContent(query, finalAnswer string, toolNames, toolSummaries []string) string {
	var sb strings.Builder
	sb.WriteString("# Trigger\n")
	sb.WriteString("- ")
	sb.WriteString(trimForPrompt(strings.TrimSpace(query), 220))
	sb.WriteString("\n\n")
	sb.WriteString("# Resolution Pattern\n")
	if len(toolSummaries) == 0 {
		sb.WriteString("- Reproduce the current state with fresh tool output before changing anything.\n")
		sb.WriteString("- Reuse the strongest existing cheatsheet or skill match instead of improvising.\n")
	} else {
		for _, summary := range toolSummaries[:reuseMin(len(toolSummaries), 5)] {
			sb.WriteString("- ")
			sb.WriteString(trimForPrompt(summary, 220))
			sb.WriteString("\n")
		}
	}
	if len(toolNames) > 0 {
		sb.WriteString("\n# Tools Involved\n")
		for _, name := range uniqueStrings(toolNames) {
			sb.WriteString("- `")
			sb.WriteString(name)
			sb.WriteString("`\n")
		}
	}
	sb.WriteString("\n# Validation\n")
	if finalAnswer != "" {
		for _, line := range reusableValidationLines(finalAnswer) {
			sb.WriteString("- ")
			sb.WriteString(trimForPrompt(line, 220))
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("- Confirm the original symptom is gone with fresh tool output.\n")
	}
	sb.WriteString("\n# Boundaries\n")
	sb.WriteString("- Re-verify files, services, and live state before trusting old memory.\n")
	sb.WriteString("- Adapt the steps to the current environment instead of following them blindly.\n")
	return strings.TrimSpace(sb.String())
}

func mergeReusableCheatsheetContent(existing, addition string) string {
	existingTrimmed := strings.TrimSpace(existing)
	additionTrimmed := strings.TrimSpace(addition)
	if additionTrimmed == "" || strings.Contains(existingTrimmed, additionTrimmed) {
		return existingTrimmed
	}
	refinement := fmt.Sprintf("## Refined Notes (%s)\n%s", time.Now().Format("2006-01-02"), additionTrimmed)
	if existingTrimmed == "" {
		return refinement
	}
	return existingTrimmed + "\n\n" + refinement
}

func deriveReusableSkillName(query, templateName string) string {
	tokens := reuseKeywords(query)
	if len(tokens) == 0 {
		return "reuse_" + templateName
	}
	parts := make([]string, 0, 3)
	for _, token := range tokens {
		if len(parts) >= 3 {
			break
		}
		parts = append(parts, token)
	}
	name := strings.Join(parts, "_")
	if name == "" {
		name = "reuse"
	}
	if !strings.HasPrefix(name, "_") && !unicode.IsLetter(rune(name[0])) && name[0] != '_' {
		name = "reuse_" + name
	}
	name = name + "_" + templateName
	if len(name) > 60 {
		name = strings.TrimRight(name[:60], "_-")
	}
	return name
}

func deriveReusableSkillDescription(query, templateName string) string {
	return trimForPrompt(fmt.Sprintf("Agent-maintained reusable helper for recurring %s tasks related to: %s", strings.ReplaceAll(templateName, "_", " "), query), 240)
}

func topAgentCheatsheet(hits []reuseArtifactHit) *tools.CheatSheet {
	for _, hit := range hits {
		if hit.Ownership == "agent" && hit.Cheatsheet != nil {
			return hit.Cheatsheet
		}
	}
	return nil
}

func topAgentSkill(hits []reuseArtifactHit) *tools.SkillRegistryEntry {
	for _, hit := range hits {
		if hit.Ownership == "agent" && hit.Skill != nil {
			return hit.Skill
		}
	}
	return nil
}

func findSkillByName(manager *tools.SkillManager, name string) *tools.SkillRegistryEntry {
	if manager == nil || name == "" {
		return nil
	}
	skills, err := manager.ListSkillsFiltered("", "", "", nil)
	if err != nil {
		return nil
	}
	for i := range skills {
		if skills[i].Name == name {
			return &skills[i]
		}
	}
	return nil
}

func mergeSkillTags(existing, additional []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(additional))
	merged := make([]string, 0, len(existing)+len(additional))
	for _, group := range [][]string{existing, additional} {
		for _, tag := range group {
			tag = strings.ToLower(strings.TrimSpace(tag))
			if tag == "" {
				continue
			}
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			merged = append(merged, tag)
		}
	}
	sort.Strings(merged)
	return merged
}

func reusableValidationLines(finalAnswer string) []string {
	lines := strings.FieldsFunc(strings.TrimSpace(finalAnswer), func(r rune) bool {
		return r == '\n' || r == '.' || r == '!' || r == '?'
	})
	out := make([]string, 0, 3)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
		if len(out) >= 3 {
			break
		}
	}
	if len(out) == 0 {
		return []string{"Confirm the expected outcome directly with fresh tool output."}
	}
	return out
}

func scoreReuseCandidate(query string, parts ...string) (float64, []string) {
	keywords := reuseKeywords(query)
	if len(keywords) == 0 {
		return 0, nil
	}
	matched := make(map[string]struct{}, len(keywords))
	score := 0.0
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	for _, part := range parts {
		lower := strings.ToLower(part)
		if lower == "" {
			continue
		}
		if len(lowerQuery) >= 10 && strings.Contains(lower, lowerQuery) {
			score += 0.35
		}
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				if _, ok := matched[keyword]; !ok {
					matched[keyword] = struct{}{}
					score += 0.18
				}
			}
		}
	}
	score = math.Min(score, 1.0)
	matches := make([]string, 0, len(matched))
	for keyword := range matched {
		matches = append(matches, keyword)
	}
	sort.Strings(matches)
	return score, matches
}

func reuseKeywords(text string) []string {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return nil
	}
	raw := reuseSplitter.Split(normalized, -1)
	keywords := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, token := range raw {
		token = strings.TrimSpace(token)
		if len(token) < 3 {
			continue
		}
		if _, stop := reuseStopWords[token]; stop {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		keywords = append(keywords, token)
	}
	return keywords
}

func matchedReason(matched []string) string {
	if len(matched) == 0 {
		return "semantic overlap"
	}
	if len(matched) == 1 {
		return "matched keyword: " + matched[0]
	}
	return "matched keywords: " + strings.Join(matched[:reuseMin(len(matched), 4)], ", ")
}

func sortReuseHits(hits []reuseArtifactHit) {
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].Name < hits[j].Name
		}
		return hits[i].Score > hits[j].Score
	})
}

func normalizedArtifactOwnership(createdBy string) string {
	switch strings.ToLower(strings.TrimSpace(createdBy)) {
	case "agent":
		return "agent"
	case "user":
		return "user"
	default:
		return "system"
	}
}

func needsCheatsheetMutation(decision ReusableArtifactDecision) bool {
	switch decision {
	case ReusableArtifactCreateCheatsheet, ReusableArtifactCreateBoth, ReusableArtifactUpdateExistingAgentCheatsheet, ReusableArtifactUpdateBoth:
		return true
	default:
		return false
	}
}

func needsSkillMutation(decision ReusableArtifactDecision) bool {
	switch decision {
	case ReusableArtifactCreateSkill, ReusableArtifactCreateBoth, ReusableArtifactUpdateExistingAgentSkill, ReusableArtifactUpdateBoth:
		return true
	default:
		return false
	}
}

func topReuseName(hits []reuseArtifactHit) string {
	if len(hits) == 0 {
		return ""
	}
	return hits[0].Name
}

func trimForPrompt(value string, maxLen int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	return strings.TrimSpace(value[:maxLen-1]) + "…"
}

func reuseFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func reuseMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func reuseMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
