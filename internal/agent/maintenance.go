package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/planner"
	"aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

// StartMaintenanceLoop spawns a background goroutine that runs daily at the configured time.
func StartMaintenanceLoop(ctx context.Context, cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, historyMgr *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, contactsDB *sql.DB, plannerDB *sql.DB, missionManagerV2 *tools.MissionManagerV2) {
	if !cfg.Maintenance.Enabled {
		logger.Info("Daily maintenance is disabled in config")
		return
	}

	hour, minute, err := parseTime(cfg.Maintenance.Time)
	if err != nil {
		logger.Error("Failed to parse maintenance time, defaulting to 04:00", "error", err, "input", cfg.Maintenance.Time)
		hour, minute = 4, 0
	}

	go func() {
		logger.Info("Started System-Level Maintenance Loop", "time", fmt.Sprintf("%02d:%02d", hour, minute))
		for {
			now := time.Now()
			nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
			if now.After(nextRun) || now.Equal(nextRun) {
				nextRun = nextRun.Add(24 * time.Hour)
			}

			sleepDuration := nextRun.Sub(now)
			logger.Debug("Maintenance loop sleeping", "next_run", nextRun, "duration_hours", sleepDuration.Hours())

			select {
			case <-time.After(sleepDuration):
				runMaintenanceTask(ctx, cfg, logger, llmClient, vault, registry, manifest, cronManager, longTermMem, shortTermMem, historyMgr, kg, inventoryDB, contactsDB, plannerDB, missionManagerV2)
			case <-ctx.Done():
				logger.Info("Maintenance loop shutting down")
				return
			}
		}
	}()
}

func parseTime(t string) (int, int, error) {
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return hour, minute, nil
}

func runMaintenanceTask(ctx context.Context, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, historyMgr *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, contactsDB *sql.DB, plannerDB *sql.DB, missionManagerV2 *tools.MissionManagerV2) {
	logger.Info("[Maintenance] Waking up to perform daily tasks")

	// Phase A5: Clean up old interaction patterns (>90 days)
	if shortTermMem != nil {
		deleted, err := shortTermMem.CleanOldPatterns(90)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean old patterns", "error", err)
		} else if deleted > 0 {
			logger.Info("[Maintenance] Cleaned old interaction patterns", "deleted", deleted)
		}

		deletedEvents, err := shortTermMem.CleanOldArchiveEvents(90)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean old archive events", "error", err)
		} else if deletedEvents > 0 {
			logger.Info("[Maintenance] Cleaned old archive events", "deleted", deletedEvents)
		}

		deletedMoodLog, err := shortTermMem.CleanOldMoodLog(30)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean old mood log entries", "error", err)
		} else if deletedMoodLog > 0 {
			logger.Info("[Maintenance] Cleaned old mood log entries", "deleted", deletedMoodLog)
		}

		cleanDays := cfg.Agent.AdaptiveTools.CleanTransitionsAfterDays
		if cleanDays <= 0 {
			cleanDays = 90
		}
		deletedTrans, err := shortTermMem.CleanOldTransitions(cleanDays)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean old tool transitions", "error", err)
		} else if deletedTrans > 0 {
			logger.Info("[Maintenance] Cleaned stale tool transitions", "deleted", deletedTrans)
		}
	}

	// Phase D8: Personality Engine maintenance — trait decay + journal
	if cfg.Personality.Engine && shortTermMem != nil {
		personalityMaintenance(cfg, shortTermMem, logger)
	}

	// User Profile cleanup: remove stale low-confidence entries
	if cfg.Personality.UserProfiling && shortTermMem != nil {
		removed, err := shortTermMem.CleanupStaleProfileEntries(30)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean stale profile entries", "error", err)
		} else if removed > 0 {
			logger.Info("[Maintenance] Cleaned stale user profile entries", "removed", removed)
		}
	}

	maintenanceBatchDone := false
	if shortTermMem != nil && kg != nil && cfg.Tools.Journal.Enabled && cfg.Journal.DailySummary && cfg.Tools.KnowledgeGraph.Enabled && cfg.Tools.KnowledgeGraph.AutoExtraction {
		maintenanceBatchDone = runBatchedMaintenanceSummaryAndKG(cfg, logger, shortTermMem, kg)
	}

	// Journal: generate daily summary from today's journal entries
	if !maintenanceBatchDone && cfg.Tools.Journal.Enabled && cfg.Journal.DailySummary && shortTermMem != nil {
		generateDailySummary(cfg, logger, client, shortTermMem)
	}

	if shortTermMem != nil {
		if rollup, err := shortTermMem.GenerateDailyActivityRollup(time.Now().Format("2006-01-02")); err != nil {
			logger.Error("[Activity] Failed to generate daily activity rollup", "error", err)
		} else if rollup.Date != "" {
			logger.Info("[Activity] Daily activity rollup stored", "date", rollup.Date)
		}
	}

	// Notes: clean up old completed notes (done for >7 days)
	if cfg.Tools.Notes.Enabled && shortTermMem != nil {
		deleted, err := shortTermMem.DeleteOldDoneNotes(7)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean old done notes", "error", err)
		} else if deleted > 0 {
			logger.Info("[Maintenance] Cleaned old done notes", "deleted", deleted)
		}
	}

	// Knowledge Graph: Garbage collection
	if kg != nil {
		if _, _, err := kg.CleanupStaleGraph(30); err != nil {
			logger.Error("[Maintenance] Failed to clean up stale KG elements", "error", err)
		}
	}

	// Sync contacts and core memory
	if kg != nil {
		SyncContactsToKnowledgeGraph(ctx, contactsDB, kg, logger)
	}
	if kg != nil {
		SyncPlannerToKnowledgeGraph(ctx, plannerDB, kg, logger)
	}
	if kg != nil && shortTermMem != nil {
		SyncCoreMemoryToKnowledgeGraph(ctx, shortTermMem, kg, logger)
	}

	// Knowledge Graph: nightly batch entity extraction from recent conversations
	if !maintenanceBatchDone && cfg.Tools.KnowledgeGraph.Enabled && cfg.Tools.KnowledgeGraph.AutoExtraction && kg != nil && shortTermMem != nil {
		extractKGEntities(cfg, logger, client, shortTermMem, kg)
	}

	// STM→LTM Consolidation: extract knowledge from archived messages into VectorDB
	if cfg.Consolidation.Enabled && shortTermMem != nil && longTermMem != nil && !longTermMem.IsDisabled() {
		consolidateSTMtoLTM(cfg, logger, client, shortTermMem, longTermMem, kg)
		consolidateEpisodicHierarchy(logger, shortTermMem, longTermMem, kg)
		promoteStableLongTermMemoriesToCore(logger, shortTermMem, longTermMem)
		detectMemoryConflictsAcrossLTM(logger, shortTermMem, longTermMem)
	}

	// 1. Load Maintenance Prompt
	promptPath := filepath.Join(cfg.Directories.PromptsDir, "maintenance.md")
	maintenancePrompt, err := os.ReadFile(promptPath)
	if err != nil {
		logger.Error("[Maintenance] Failed to read maintenance prompt", "error", err)
		return
	}

	// 2. Prepare the request
	req := openai.ChatCompletionRequest{
		Model: cfg.LLM.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: string(maintenancePrompt)},
		},
	}

	sessionID := "maintenance"

	// 3. Execute reasoning loop
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.CircuitBreaker.MaintenanceTimeoutMinutes)*time.Minute)
	defer cancel()

	// Use NoopBroker for silent background reasoning
	broker := &NoopBroker{}

	runCfg := RunConfig{
		Config:           cfg,
		Logger:           logger,
		LLMClient:        client,
		ShortTermMem:     shortTermMem,
		HistoryManager:   historyMgr,
		LongTermMem:      longTermMem,
		KG:               kg,
		InventoryDB:      inventoryDB,
		Vault:            vault,
		Registry:         registry,
		Manifest:         manifest,
		CronManager:      cronManager,
		MissionManagerV2: missionManagerV2,
		CoAgentRegistry:  nil,
		BudgetTracker:    nil,
		SessionID:        sessionID,
		IsMaintenance:    false,
		SurgeryPlan:      "",
	}

	resp, err := ExecuteAgentLoop(ctx, req, runCfg, false, broker)
	if err != nil {
		logger.Error("[Maintenance] Agent loop failed", "error", err)
		return
	}

	if len(resp.Choices) > 0 {
		logger.Info("[Maintenance] Task completed successfully", "response_len", len(resp.Choices[0].Message.Content))
	} else {
		logger.Warn("[Maintenance] Agent returned no choices")
	}
}

// personalityMaintenance performs daily trait decay and appends a character journal entry.
func personalityMaintenance(cfg *config.Config, stm *memory.SQLiteMemory, logger *slog.Logger) {
	// 1. Trait decay: nudge all traits toward 0.5, respecting the personality profile's decay rate
	meta := prompts.GetCorePersonalityMeta(cfg.Directories.PromptsDir, cfg.Personality.CorePersonality)
	decayAmount := 0.002 * meta.TraitDecayRate
	if err := stm.DecayAllTraitsWeighted(decayAmount, meta); err != nil {
		logger.Error("[Personality] Trait decay failed", "error", err)
	} else {
		logger.Info("[Personality] Daily weighted trait decay applied", "amount", decayAmount, "decay_rate", meta.TraitDecayRate)
	}

	// 2. Emotion history cleanup
	if cfg.Personality.EmotionSynthesizer.Enabled {
		deleted, err := stm.CleanupEmotionHistory(30, cfg.Personality.EmotionSynthesizer.MaxHistoryEntries)
		if err != nil {
			logger.Error("[EmotionSynthesizer] Emotion history cleanup failed", "error", err)
		} else if deleted > 0 {
			logger.Info("[EmotionSynthesizer] Emotion history cleaned up", "deleted", deleted)
		}
	}

	// 3. Character journal: append today's snapshot to data/character_journal.md
	traits, err := stm.GetTraits()
	if err != nil {
		logger.Error("[Personality] Cannot read traits for journal", "error", err)
		return
	}
	mood := stm.GetCurrentMood()
	milestones, _ := stm.GetMilestones(3)

	journalPath := filepath.Join(cfg.Directories.DataDir, "character_journal.md")
	f, err := os.OpenFile(journalPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Error("[Personality] Cannot open character journal", "error", err)
		return
	}
	defer f.Close()

	entry := fmt.Sprintf("\n## %s\n**Mood:** %s\n**Traits:** C:%.2f T:%.2f Cr:%.2f E:%.2f Co:%.2f A:%.2f L:%.2f\n",
		time.Now().Format("2006-01-02"),
		mood,
		traits[memory.TraitCuriosity],
		traits[memory.TraitThoroughness],
		traits[memory.TraitCreativity],
		traits[memory.TraitEmpathy],
		traits[memory.TraitConfidence],
		traits[memory.TraitAffinity],
		traits[memory.TraitLoneliness],
	)
	if len(milestones) > 0 {
		entry += "**Recent Milestones:**\n"
		for _, m := range milestones {
			entry += fmt.Sprintf("- %s\n", m)
		}
	}

	if _, err := f.WriteString(entry); err != nil {
		logger.Error("[Personality] Failed to write journal entry", "error", err)
	} else {
		logger.Info("[Personality] Character journal updated")
	}
}

// generateDailySummary creates an LLM-generated summary for today based on journal entries.
func generateDailySummary(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, stm *memory.SQLiteMemory) {
	today := time.Now().Format("2006-01-02")

	// Check if a summary already exists for today
	if existing, _ := stm.GetDailySummary(today); existing != nil {
		logger.Debug("[Journal] Daily summary already exists", "date", today)
		return
	}

	// Collect today's journal entries
	entries, err := stm.GetJournalEntries(today, today, nil, 50)
	if err != nil || len(entries) == 0 {
		logger.Debug("[Journal] No journal entries today, skipping summary", "date", today)
		return
	}

	journalInput := buildDailySummaryJournalInput(entries)

	prompt := fmt.Sprintf(`Summarize the following activity log from today (%s) in 2-3 concise sentences.
Focus on: what was accomplished, key decisions, and notable events.
Output ONLY the summary text, no JSON or formatting.

Activity log:
%s`, today, journalInput)

	summaryClient, summaryModel := resolveHelperBackedLLM(cfg, client, cfg.LLM.Model)
	if summaryClient == nil || summaryModel == "" {
		logger.Warn("[Journal] Daily summary skipped: no helper/main LLM available")
		return
	}

	resp, err := llm.ExecuteWithRetry(
		context.Background(),
		summaryClient,
		openai.ChatCompletionRequest{
			Model: summaryModel,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: "You are a concise activity summarizer. Output ONLY 2-3 sentences."},
				{Role: openai.ChatMessageRoleUser, Content: prompt},
			},
			MaxTokens: 300,
		},
		logger,
		nil,
	)
	if err != nil || len(resp.Choices) == 0 {
		logger.Warn("[Journal] Failed to generate daily summary via LLM", "error", err, "model", summaryModel)
		return
	}

	storeDailySummaryText(stm, logger, today, entries, resp.Choices[0].Message.Content)
	return
}

func uniqueTopics(in []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func buildDailySummaryJournalInput(entries []memory.JournalEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", e.EntryType, e.Title, e.Content))
	}
	return strings.TrimSpace(sb.String())
}

func buildKGActivityInput(turns []memory.ActivityTurn) string {
	var sb strings.Builder
	for _, turn := range turns {
		if strings.TrimSpace(turn.Intent) == "" && strings.TrimSpace(turn.UserRequest) == "" && len(turn.ImportantPoints) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("- intent=%s; request=%s; goal=%s\n", turn.Intent, turn.UserRequest, turn.UserGoal))
		if len(turn.ImportantPoints) > 0 {
			sb.WriteString(fmt.Sprintf("  important: %s\n", strings.Join(turn.ImportantPoints, " | ")))
		}
		if len(turn.Outcomes) > 0 {
			sb.WriteString(fmt.Sprintf("  outcomes: %s\n", strings.Join(turn.Outcomes, " | ")))
		}
		if len(turn.PendingItems) > 0 {
			sb.WriteString(fmt.Sprintf("  pending: %s\n", strings.Join(turn.PendingItems, " | ")))
		}
		if sb.Len() > 3000 {
			break
		}
	}
	return strings.TrimSpace(sb.String())
}

func buildKGConversationExcerpt(messages []openai.ChatCompletionMessage) string {
	var sb strings.Builder
	for _, m := range messages {
		if m.Role == "system" || strings.TrimSpace(m.Content) == "" {
			continue
		}
		content := m.Content
		if len(content) > 500 {
			content = truncateUTF8ToLimit(content, 503, "...")
		}
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, content))
		if sb.Len() > 8000 {
			break
		}
	}
	return strings.TrimSpace(sb.String())
}

func buildKGExtractionInput(messages []openai.ChatCompletionMessage, entries []memory.JournalEntry, turns []memory.ActivityTurn) string {
	sections := make([]string, 0, 3)

	if conversation := buildKGConversationExcerpt(messages); conversation != "" {
		sections = append(sections, "Conversation:\n"+conversation)
	}
	if activity := buildKGActivityInput(turns); activity != "" {
		sections = append(sections, "Activity turns:\n"+activity)
	}
	if journal := buildDailySummaryJournalInput(entries); journal != "" {
		sections = append(sections, "Journal entries:\n"+journal)
	}

	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func storeDailySummaryText(stm *memory.SQLiteMemory, logger *slog.Logger, today string, entries []memory.JournalEntry, summaryText string) {
	summaryText = strings.TrimSpace(summaryText)
	if stm == nil || summaryText == "" {
		return
	}

	summary := memory.DailySummary{
		Date:      today,
		Summary:   summaryText,
		Sentiment: "neutral",
	}

	toolUsage := make(map[string]int)
	var topics []string
	for _, e := range entries {
		for _, tag := range e.Tags {
			toolUsage[tag]++
		}
		if e.EntryType != "" {
			topics = append(topics, e.EntryType)
		}
	}
	summary.ToolUsage = toolUsage
	summary.KeyTopics = uniqueTopics(topics)

	if err := stm.InsertDailySummary(summary); err != nil {
		logger.Error("[Journal] Failed to store daily summary", "error", err)
		return
	}

	logger.Info("[Journal] Daily summary stored", "date", today)
	anchor := summary.Summary
	if idx := strings.Index(anchor, "."); idx > 20 {
		anchor = strings.TrimSpace(anchor[:idx+1])
	}
	if len(anchor) > 220 {
		anchor = strings.TrimSpace(anchor[:220]) + "..."
	}
	if err := stm.UpsertDayAnchor(today, anchor); err != nil {
		logger.Warn("[Journal] Failed to store day anchor", "error", err)
	}
}

func storeKGExtraction(logger *slog.Logger, kg *memory.KnowledgeGraph, nodes []memory.Node, edges []memory.Edge) {
	if kg == nil {
		return
	}
	if len(nodes) == 0 && len(edges) == 0 {
		logger.Debug("[KG] No entities extracted")
		return
	}

	for i := range nodes {
		if nodes[i].Properties == nil {
			nodes[i].Properties = make(map[string]string)
		}
		nodes[i].Properties["source"] = "auto_extraction"
		nodes[i].Properties["extracted_at"] = time.Now().Format("2006-01-02")
	}
	for i := range edges {
		if edges[i].Properties == nil {
			edges[i].Properties = make(map[string]string)
		}
		edges[i].Properties["source"] = "auto_extraction"
		edges[i].Properties["extracted_at"] = time.Now().Format("2006-01-02")
	}

	if err := kg.BulkMergeExtractedEntities(nodes, edges); err != nil {
		logger.Error("[KG] Failed to bulk-add extracted entities", "error", err)
		return
	}

	logger.Info("[KG] Nightly entity extraction complete", "nodes", len(nodes), "edges", len(edges))
}

func runBatchedMaintenanceSummaryAndKG(cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory, kg *memory.KnowledgeGraph) bool {
	helperManager := newHelperLLMManager(cfg, logger)
	if helperManager == nil || stm == nil || kg == nil {
		return false
	}

	today := time.Now().Format("2006-01-02")
	if existing, _ := stm.GetDailySummary(today); existing != nil {
		return false
	}

	entries, err := stm.GetJournalEntries(today, today, nil, 50)
	if err != nil || len(entries) == 0 {
		return false
	}
	messages, err := stm.GetRecentMessagesAcrossSessions(100)
	if err != nil || len(messages) == 0 {
		return false
	}
	turns, _ := stm.GetActivityTurnsForDate(today, 20)

	journalInput := buildDailySummaryJournalInput(entries)
	conversationInput := buildKGExtractionInput(messages, entries, turns)
	if journalInput == "" || len(conversationInput) < 50 {
		return false
	}

	existingNodesString := ""
	if existingNodes, err := kg.GetAllNodes(150); err == nil && len(existingNodes) > 0 {
		var contexts []string
		for _, n := range existingNodes {
			contexts = append(contexts, fmt.Sprintf("- ID: %s, Label: %s", n.ID, n.Label))
		}
		existingNodesString = strings.Join(contexts, "\n")
	}

	batchCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	result, err := helperManager.AnalyzeMaintenanceSummaryAndKG(batchCtx, today, journalInput, conversationInput, existingNodesString)
	if err != nil {
		helperManager.ObserveFallback("maintenance_summary_kg", err.Error())
		logger.Warn("[HelperLLM] Maintenance summary/KG batch failed, falling back", "error", err)
		return false
	}
	if result.DailySummary == "" {
		helperManager.ObserveFallback("maintenance_summary_kg", "empty daily summary")
		logger.Warn("[HelperLLM] Maintenance batch returned empty daily summary, falling back")
		return false
	}

	storeDailySummaryText(stm, logger, today, entries, result.DailySummary)
	storeKGExtraction(logger, kg, result.KGExtraction.Nodes, result.KGExtraction.Edges)
	return true
}

// extractKGFromText performs source-agnostic entity extraction from arbitrary text.
// It builds the LLM prompt, executes the extraction call, parses the response, and returns
// nodes and edges. This function does not interact with the knowledge graph directly.
func extractKGFromText(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, inputText string, existingNodesString string) ([]memory.Node, []memory.Edge, error) {
	if len(inputText) < 50 {
		return nil, nil, fmt.Errorf("input text too short for extraction")
	}

	prompt := fmt.Sprintf(`Extract entities and relationships from this conversation.
Return ONLY valid JSON with this exact structure:
{
  "nodes": [{"id": "lowercase_id", "label": "Display Label", "properties": {"type": "person|device|service|software|location|project|concept|event"}}],
  "edges": [{"source": "node_id", "target": "node_id", "relation": "relationship_type"}]
}

Rules:
- IDs must be lowercase with underscores (e.g. "john_doe", "home_server").
- REUSE existing node IDs if the entity matches an existing one.
- Extract only clear, factual entities.
- Vocabulary for types: person, device, service, software, location, project, concept, event.
- Vocabulary for relationships: runs_on, owns, manages, uses, depends_on, connected_to, related_to, part_of, deployed_on, located_in.
- Limit to highly relevant facts. Maximum 15 nodes and 20 edges.

Example:
Excerpt: "I installed adguard on my truenas server at 192.168.1.5"
JSON:
{
  "nodes": [
    {"id": "adguard", "label": "AdGuard", "properties": {"type": "software"}},
    {"id": "truenas", "label": "TrueNAS Server", "properties": {"type": "device", "ip": "192.168.1.5"}}
  ],
  "edges": [
    {"source": "adguard", "target": "truenas", "relation": "runs_on"}
  ]
}

Inputs:
%s%s`, existingNodesString, inputText)

	kgClient, kgModel := resolveHelperBackedLLM(cfg, client, cfg.LLM.Model)
	if kgClient == nil || kgModel == "" {
		return nil, nil, fmt.Errorf("no helper/main LLM available")
	}

	kgCtx, kgCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer kgCancel()

	resp, err := llm.ExecuteWithRetry(
		kgCtx,
		kgClient,
		openai.ChatCompletionRequest{
			Model: kgModel,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: "You are an entity extraction engine. Output ONLY valid JSON, no markdown fences."},
				{Role: openai.ChatMessageRoleUser, Content: prompt},
			},
			MaxTokens: 1500,
		},
		logger,
		nil,
	)
	if err != nil || len(resp.Choices) == 0 {
		return nil, nil, fmt.Errorf("LLM call failed: %w", err)
	}

	rawJSON := trimJSONResponse(resp.Choices[0].Message.Content)

	var extracted struct {
		Nodes []memory.Node `json:"nodes"`
		Edges []memory.Edge `json:"edges"`
	}
	if err := json.Unmarshal([]byte(rawJSON), &extracted); err != nil {
		return nil, nil, fmt.Errorf("JSON parse failed: %w", err)
	}

	return extracted.Nodes, extracted.Edges, nil
}

// extractKGEntities performs nightly batch entity extraction from the past 24h of messages.
// Uses an LLM call to extract entities and relationships, then bulk-adds to the knowledge graph.
// This is a conversation-specific adapter around extractKGFromText.
func extractKGEntities(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, stm *memory.SQLiteMemory, kg *memory.KnowledgeGraph) {
	today := time.Now().Format("2006-01-02")

	// Collect recent messages across all sessions.
	messages, err := stm.GetRecentMessagesAcrossSessions(100)
	if err != nil || len(messages) == 0 {
		logger.Debug("[KG] No recent messages for entity extraction")
		return
	}
	entries, _ := stm.GetJournalEntries(today, today, nil, 30)
	turns, _ := stm.GetActivityTurnsForDate(today, 20)

	conversationExcerpt := buildKGExtractionInput(messages, entries, turns)
	if len(conversationExcerpt) < 50 {
		logger.Debug("[KG] Not enough conversation content for entity extraction")
		return
	}

	existingNodesString := ""
	if existingNodes, err := kg.GetAllNodes(150); err == nil && len(existingNodes) > 0 {
		var contexts []string
		for _, n := range existingNodes {
			contexts = append(contexts, fmt.Sprintf("- ID: %s, Label: %s", n.ID, n.Label))
		}
		existingNodesString = "Existing Nodes (reuse IDs if possible):\n" + strings.Join(contexts, "\n") + "\n\n"
	}

	nodes, edges, err := extractKGFromText(cfg, logger, client, conversationExcerpt, existingNodesString)
	if err != nil {
		logger.Warn("[KG] Entity extraction failed", "error", err)
		return
	}

	storeKGExtraction(logger, kg, nodes, edges)
}

const helperConsolidationBatchSize = 2

type consolidationWorkItem struct {
	batchID      string
	messages     []memory.ArchivedMessage
	messageIDs   []int64
	conversation string
}

func buildConsolidationWorkItem(index int, batch []memory.ArchivedMessage) consolidationWorkItem {
	var sb strings.Builder
	messageIDs := make([]int64, 0, len(batch))
	for _, msg := range batch {
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", msg.Timestamp, msg.Role, msg.Content))
		messageIDs = append(messageIDs, msg.ID)
	}
	return consolidationWorkItem{
		batchID:      fmt.Sprintf("batch_%d", index+1),
		messages:     batch,
		messageIDs:   messageIDs,
		conversation: strings.TrimSpace(sb.String()),
	}
}

func extractConsolidationFactsWithLLM(logger *slog.Logger, client llm.ChatClient, model, conversation string) ([]helperConsolidationFact, error) {
	prompt := fmt.Sprintf(`Analyze the following conversation excerpt and extract the most important knowledge.
Return ONLY valid JSON with this exact structure:
{
  "facts": [
    {"concept": "Short topic title", "content": "Detailed factual information extracted"}
  ]
}

Rules:
- Extract concrete facts, decisions, user preferences, technical details, and actionable knowledge
- Each fact should be self-contained and understandable without the original conversation
- Concept should be a brief 2-5 word topic label
- Content should preserve specific details: names, versions, paths, commands, configurations
- Skip generic pleasantries, acknowledgments, and obvious context
- Maximum 10 facts per batch
- If no meaningful facts exist, return {"facts": []}

Conversation:
%s`, conversation)

	resp, err := llm.ExecuteWithRetry(
		context.Background(),
		client,
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: "You are a knowledge extraction engine. Extract factual knowledge from conversations. Output ONLY valid JSON, no markdown fences."},
				{Role: openai.ChatMessageRoleUser, Content: prompt},
			},
			MaxTokens: 1000,
		},
		logger,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("llm extraction failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("llm extraction returned no choices")
	}

	var extracted struct {
		Facts []helperConsolidationFact `json:"facts"`
	}
	if err := json.Unmarshal([]byte(trimJSONResponse(resp.Choices[0].Message.Content)), &extracted); err != nil {
		return nil, fmt.Errorf("json parse failed: %w", err)
	}
	return extracted.Facts, nil
}

func storeConsolidationFacts(logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, facts []helperConsolidationFact) int {
	stored := 0
	for _, fact := range facts {
		if fact.Concept == "" || fact.Content == "" {
			continue
		}
		ids, err := ltm.StoreDocument(fact.Concept, fact.Content)
		if err != nil {
			logger.Warn("[Consolidation] Failed to store fact in LTM", "concept", fact.Concept, "error", err)
			continue
		}
		for _, id := range ids {
			_ = stm.UpsertMemoryMetaWithDetails(id, memory.MemoryMetaUpdate{
				ExtractionConfidence: 0.82,
				VerificationStatus:   "unverified",
				SourceType:           "consolidation",
				SourceReliability:    0.82,
			})
		}
		detectMemoryConflictsForDocIDs(logger, stm, ltm, ids, fact.Content)
		stored++
	}
	return stored
}

func recordConsolidationBatchEpisode(stm *memory.SQLiteMemory, batch []memory.ArchivedMessage, factsCount, batchIndex, batchTotal int) {
	if stm == nil || len(batch) == 0 {
		return
	}
	eventDate := time.Now().Format("2006-01-02")
	if len(batch[0].Timestamp) >= 10 {
		eventDate = batch[0].Timestamp[:10]
	}
	episodeTitle := "Consolidated conversation batch"
	episodeSummary := fmt.Sprintf("%d messages, %d facts extracted", len(batch), factsCount)
	episodeDetails := map[string]string{
		"session_id": batch[0].SessionID,
		"batch":      fmt.Sprintf("%d/%d", batchIndex, batchTotal),
	}
	_ = stm.InsertEpisodicMemoryWithDetails(eventDate, episodeTitle, episodeSummary, episodeDetails, 2, "consolidation", memory.EpisodicMemoryDetails{
		SessionID:        batch[0].SessionID,
		HierarchyLevel:   1,
		Participants:     []string{"user", "agent"},
		EmotionalValence: 0,
	})
}

// consolidateSTMtoLTM extracts knowledge from archived STM messages and stores it in the VectorDB.
// This bridges the gap between the sliding-window short-term memory and the persistent long-term memory.
func consolidateSTMtoLTM(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, stm *memory.SQLiteMemory, ltm memory.VectorDB, kg *memory.KnowledgeGraph) {
	archived, err := stm.GetConsolidationCandidates(cfg.Consolidation.MaxBatchMessages, 3)
	if err != nil {
		logger.Error("[Consolidation] Failed to fetch unconsolidated messages", "error", err)
		return
	}
	if len(archived) == 0 {
		logger.Debug("[Consolidation] No unconsolidated archived messages")
		return
	}

	logger.Info("[Consolidation] Starting STM→LTM consolidation", "messages", len(archived))

	// Group messages into batches of ~4000 characters for LLM processing
	const maxBatchChars = 4000
	var batches [][]memory.ArchivedMessage
	var currentBatch []memory.ArchivedMessage
	currentLen := 0

	for _, msg := range archived {
		msgLen := len(msg.Content)
		if currentLen+msgLen > maxBatchChars && len(currentBatch) > 0 {
			batches = append(batches, currentBatch)
			currentBatch = nil
			currentLen = 0
		}
		currentBatch = append(currentBatch, msg)
		currentLen += msgLen
	}
	if len(currentBatch) > 0 {
		batches = append(batches, currentBatch)
	}

	totalStored := 0
	var allConsolidatedIDs []int64
	consolidationClient, consolidationModel := resolveHelperBackedLLM(cfg, client, resolveConsolidationModel(cfg))
	if consolidationClient == nil || consolidationModel == "" {
		logger.Warn("[Consolidation] STM->LTM consolidation skipped: no helper/main LLM available")
		return
	}
	helperManager := newHelperLLMManager(cfg, logger)
	workItems := make([]consolidationWorkItem, 0, len(batches))
	for i, batch := range batches {
		workItems = append(workItems, buildConsolidationWorkItem(i, batch))
	}

	processWorkItem := func(item consolidationWorkItem, batchIndex int) {
		facts, err := extractConsolidationFactsWithLLM(logger, consolidationClient, consolidationModel, item.conversation)
		if err != nil {
			logger.Warn("[Consolidation] LLM extraction failed for batch", "batch", batchIndex, "error", err)
			_ = stm.MarkConsolidationFailure(item.messageIDs, err.Error())
			return
		}
		totalStored += storeConsolidationFacts(logger, stm, ltm, facts)
		recordConsolidationBatchEpisode(stm, item.messages, len(facts), batchIndex, len(workItems))
		allConsolidatedIDs = append(allConsolidatedIDs, item.messageIDs...)
	}

	for i := 0; i < len(workItems); {
		if helperManager == nil {
			processWorkItem(workItems[i], i+1)
			i++
			continue
		}

		end := i + helperConsolidationBatchSize
		if end > len(workItems) {
			end = len(workItems)
		}

		inputs := make([]helperConsolidationBatchInput, 0, end-i)
		group := workItems[i:end]
		for _, item := range group {
			inputs = append(inputs, helperConsolidationBatchInput{
				BatchID:      item.batchID,
				Conversation: item.conversation,
			})
		}

		consolidationCtx, consolidationCancel := context.WithTimeout(context.Background(), 60*time.Second)
		result, err := helperManager.AnalyzeConsolidationBatches(consolidationCtx, inputs)
		consolidationCancel()
		if err != nil {
			helperManager.ObserveFallback("consolidation_batches", err.Error())
			logger.Warn("[HelperLLM] Consolidation batch failed, falling back", "start_batch", i+1, "error", err)
			for offset, item := range group {
				processWorkItem(item, i+offset+1)
			}
			i = end
			continue
		}

		byID := make(map[string][]helperConsolidationFact, len(result.Batches))
		for _, batchResult := range result.Batches {
			byID[batchResult.BatchID] = batchResult.Facts
		}
		for offset, item := range group {
			facts := byID[item.batchID]
			totalStored += storeConsolidationFacts(logger, stm, ltm, facts)
			recordConsolidationBatchEpisode(stm, item.messages, len(facts), i+offset+1, len(workItems))
			allConsolidatedIDs = append(allConsolidatedIDs, item.messageIDs...)
		}
		i = end
	}

	// Mark all processed messages as consolidated
	if len(allConsolidatedIDs) > 0 {
		if err := stm.MarkConsolidationSuccess(allConsolidatedIDs); err != nil {
			logger.Error("[Consolidation] Failed to mark messages as consolidated", "error", err)
		}
	}

	// Clean up old archived messages
	if cfg.Consolidation.ArchiveRetainDays > 0 {
		cleaned, err := stm.CleanOldArchivedMessages(cfg.Consolidation.ArchiveRetainDays)
		if err != nil {
			logger.Error("[Consolidation] Failed to clean old archived messages", "error", err)
		} else if cleaned > 0 {
			logger.Info("[Consolidation] Cleaned old archived messages", "deleted", cleaned)
		}
	}

	// Auto-optimize: run priority-based forgetting on VectorDB + KG
	if cfg.Consolidation.AutoOptimize && totalStored > 0 {
		autoOptimizeMemory(cfg, logger, client, ltm, stm, kg)
	}

	// Create journal entry for the consolidation run
	if cfg.Tools.Journal.Enabled && totalStored > 0 {
		_, _ = stm.InsertJournalEntry(memory.JournalEntry{
			EntryType: "system",
			Title:     "Nightly STM→LTM Consolidation",
			Content:   fmt.Sprintf("Consolidated %d archived messages into %d LTM facts.", len(allConsolidatedIDs), totalStored),
			Tags:      []string{"consolidation", "maintenance", "memory"},
		})
	}

	logger.Info("[Consolidation] STM→LTM consolidation complete",
		"messages_processed", len(allConsolidatedIDs),
		"facts_stored", totalStored,
		"batches", len(batches))
}

func resolveConsolidationModel(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if helperCfg := llm.ResolveHelperLLM(cfg); helperCfg.Enabled && helperCfg.Model != "" {
		return helperCfg.Model
	}
	if model := strings.TrimSpace(cfg.Consolidation.Model); model != "" {
		return model
	}
	return strings.TrimSpace(cfg.LLM.Model)
}

func resolveHelperBackedLLM(cfg *config.Config, fallbackClient llm.ChatClient, fallbackModel string) (llm.ChatClient, string) {
	if helperCfg := llm.ResolveHelperLLM(cfg); helperCfg.Enabled && helperCfg.Model != "" {
		manager := getOrCreateHelperLLMManager(cfg, nil)
		if manager != nil && manager.client != nil {
			return manager.client, helperCfg.Model
		}
		helperClient := llm.NewClientFromProvider(helperCfg.ProviderType, helperCfg.BaseURL, helperCfg.APIKey)
		if helperClient != nil {
			return helperClient, helperCfg.Model
		}
	}
	return fallbackClient, strings.TrimSpace(fallbackModel)
}

func consolidateEpisodicHierarchy(logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, kg *memory.KnowledgeGraph) {
	if stm == nil || ltm == nil || ltm.IsDisabled() {
		return
	}
	episodes, err := stm.GetEpisodicMemoriesByHierarchyLevel(1, 40)
	if err != nil || len(episodes) < 2 {
		return
	}
	groups := make(map[string][]memory.EpisodicMemory)
	for _, episode := range episodes {
		if episode.ActionStatus == "pending" {
			continue
		}
		groupKey := episode.SessionID
		if groupKey == "" {
			groupKey = "global"
		}
		if len(episode.EventDate) >= 7 {
			groupKey += "|" + episode.EventDate[:7]
		}
		groups[groupKey] = append(groups[groupKey], episode)
	}
	for groupKey, group := range groups {
		if len(group) < 2 {
			continue
		}
		summary := buildHierarchicalEpisodeSummary(group)
		if strings.TrimSpace(summary) == "" {
			continue
		}
		concept := "Hierarchical memory synthesis " + groupKey
		ids, err := ltm.StoreDocument(concept, summary)
		if err != nil {
			logger.Warn("[Hierarchy] Failed to store episodic synthesis", "group", groupKey, "error", err)
			continue
		}
		for _, id := range ids {
			_ = stm.UpsertMemoryMetaWithDetails(id, memory.MemoryMetaUpdate{
				ExtractionConfidence: 0.88,
				VerificationStatus:   "unverified",
				SourceType:           "hierarchical_consolidation",
				SourceReliability:    0.9,
			})
		}
		if kg != nil {
			uniqueParticipants := uniqueHierarchyStrings(nil)
			for _, episode := range group {
				uniqueParticipants = uniqueHierarchyStrings(append(uniqueParticipants, episode.Participants...))
			}
			for _, participant := range uniqueParticipants {
				if participant == "" {
					continue
				}
				_ = kg.AddEdge(participant, concept, "appears_in_memory_synthesis", map[string]string{"group": groupKey})
			}
		}
		related := make([]string, 0, len(ids))
		related = append(related, ids...)
		episodeIDs := make([]int64, 0, len(group))
		for _, episode := range group {
			episodeIDs = append(episodeIDs, episode.ID)
			related = append(related, episode.RelatedDocIDs...)
		}
		_ = stm.InsertEpisodicMemoryWithDetails(group[0].EventDate, "Hierarchical memory synthesis", truncateHierarchySummary(summary, 240), map[string]string{"group": groupKey}, 3, "hierarchical_consolidation", memory.EpisodicMemoryDetails{
			SessionID:      group[0].SessionID,
			HierarchyLevel: 2,
			Participants:   uniqueHierarchyParticipants(group),
			RelatedDocIDs:  uniqueHierarchyStrings(related),
		})
		_ = stm.MarkEpisodicMemoriesHierarchy(episodeIDs, 2)
	}
}

func promoteStableLongTermMemoriesToCore(logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB) {
	if stm == nil || ltm == nil || ltm.IsDisabled() {
		return
	}
	metas, err := stm.GetAllMemoryMeta(500, 0)
	if err != nil {
		return
	}
	for _, meta := range metas {
		if meta.KeepForever || meta.Protected || meta.VerificationStatus == "contradicted" {
			continue
		}
		if meta.AccessCount < 2 || meta.ExtractionConfidence < 0.85 || meta.SourceReliability < 0.8 {
			continue
		}
		if meta.UsefulCount > 0 && meta.UsefulCount < meta.UselessCount {
			continue
		}
		content, err := ltm.GetByID(meta.DocID)
		if err != nil || strings.TrimSpace(content) == "" {
			continue
		}
		fact := truncateHierarchySummary(strings.Join(strings.Fields(content), " "), 260)
		if fact == "" || stm.CoreMemoryFactExists(fact) {
			continue
		}
		if _, err := stm.AddCoreMemoryFact(fact); err != nil {
			logger.Warn("[Hierarchy] Failed to promote memory to core", "doc_id", meta.DocID, "error", err)
			continue
		}
		_ = stm.SetMemoryMetaProtection(meta.DocID, true, true)
	}
}

func detectMemoryConflictsAcrossLTM(logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB) {
	if stm == nil || ltm == nil || ltm.IsDisabled() {
		return
	}
	metas, err := stm.GetAllMemoryMeta(250, 0)
	if err != nil {
		return
	}
	for _, meta := range metas {
		detectMemoryConflictsForDocIDs(logger, stm, ltm, []string{meta.DocID}, "")
	}
}

func buildHierarchicalEpisodeSummary(group []memory.EpisodicMemory) string {
	if len(group) == 0 {
		return ""
	}
	parts := make([]string, 0, len(group)+1)
	parts = append(parts, fmt.Sprintf("Memory synthesis for %d related episodes:", len(group)))
	for _, episode := range group {
		parts = append(parts, fmt.Sprintf("- %s: %s", episode.Title, episode.Summary))
	}
	return strings.Join(parts, "\n")
}

func uniqueHierarchyParticipants(group []memory.EpisodicMemory) []string {
	values := make([]string, 0, len(group)*2)
	for _, episode := range group {
		values = append(values, episode.Participants...)
	}
	return uniqueHierarchyStrings(values)
}

func uniqueHierarchyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func truncateHierarchySummary(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	if maxLen <= 3 {
		return value[:maxLen]
	}
	return value[:maxLen-3] + "..."
}

// autoOptimizeMemory runs priority-based forgetting on VectorDB and Knowledge Graph.
func autoOptimizeMemory(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, ltm memory.VectorDB, stm *memory.SQLiteMemory, kg *memory.KnowledgeGraph) {
	threshold := cfg.Consolidation.OptimizeThreshold

	metas, err := stm.GetAllMemoryMeta(50000, 0)
	if err != nil {
		logger.Error("[AutoOptimize] Failed to fetch memory metadata", "error", err)
		return
	}

	var lowDocs, mediumDocs []string
	for _, meta := range metas {
		if meta.Protected || meta.KeepForever {
			continue
		}
		priority := adjustedMemoryPriority(meta, time.Now())
		if priority < threshold {
			lowDocs = append(lowDocs, meta.DocID)
		} else if priority < threshold+2 {
			mediumDocs = append(mediumDocs, meta.DocID)
		}
	}

	// Remove low-priority documents
	for _, docID := range lowDocs {
		_ = ltm.DeleteDocument(docID)
		_ = stm.DeleteMemoryMeta(docID)
	}

	// Compress medium-priority documents
	optimizeClient, optimizeModel := resolveHelperBackedLLM(cfg, client, cfg.LLM.Model)
	if optimizeClient == nil || optimizeModel == "" {
		logger.Warn("[AutoOptimize] Compression skipped: no helper/main LLM available")
		return
	}
	helperManager := newHelperLLMManager(cfg, logger)
	type compressionWorkItem struct {
		docID    string
		content  string
		concept  string
		memoryID string
	}
	workItems := make([]compressionWorkItem, 0, len(mediumDocs))
	for _, docID := range mediumDocs {
		content, err := ltm.GetByID(docID)
		if err != nil || len(content) < 300 {
			continue
		}
		concept := "Compressed Memory"
		parts := strings.SplitN(content, "\n\n", 2)
		if len(parts) == 2 {
			concept = parts[0]
		}
		workItems = append(workItems, compressionWorkItem{
			docID:    docID,
			content:  content,
			concept:  concept,
			memoryID: fmt.Sprintf("mem_%d", len(workItems)+1),
		})
	}

	compressOne := func(item compressionWorkItem) {
		compressCtx, compressCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer compressCancel()

		resp, err := llm.ExecuteWithRetry(
			compressCtx,
			optimizeClient,
			openai.ChatCompletionRequest{
				Model: optimizeModel,
				Messages: []openai.ChatCompletionMessage{
					{Role: openai.ChatMessageRoleSystem, Content: "Compress this memory into a dense bullet-point list of core facts. Output ONLY the compressed text."},
					{Role: openai.ChatMessageRoleUser, Content: item.content},
				},
				MaxTokens: 500,
			},
			logger,
			nil,
		)
		if err != nil || len(resp.Choices) == 0 {
			return
		}
		compressed := strings.TrimSpace(resp.Choices[0].Message.Content)
		if compressed == "" {
			return
		}
		newIDs, err2 := ltm.StoreDocument(item.concept, compressed)
		if err2 == nil {
			_ = ltm.DeleteDocument(item.docID)
			_ = stm.DeleteMemoryMeta(item.docID)
			for _, newID := range newIDs {
				_ = stm.UpsertMemoryMeta(newID)
			}
		}
	}

	const helperCompressionBatchSize = 3
	for i := 0; i < len(workItems); {
		if helperManager == nil {
			compressOne(workItems[i])
			i++
			continue
		}

		end := i + helperCompressionBatchSize
		if end > len(workItems) {
			end = len(workItems)
		}
		group := workItems[i:end]
		inputs := make([]helperCompressionBatchInput, 0, len(group))
		for _, item := range group {
			inputs = append(inputs, helperCompressionBatchInput{
				MemoryID: item.memoryID,
				Content:  item.content,
			})
		}

		compressionCtx, compressionCancel := context.WithTimeout(context.Background(), 60*time.Second)
		result, err := helperManager.CompressMemoryBatches(compressionCtx, inputs)
		compressionCancel()
		if err != nil {
			helperManager.ObserveFallback("compress_memories", err.Error())
			logger.Warn("[HelperLLM] Memory compression batch failed, falling back", "start_memory", i+1, "error", err)
			for _, item := range group {
				compressOne(item)
			}
			i = end
			continue
		}

		byID := make(map[string]string, len(result.Memories))
		for _, item := range result.Memories {
			byID[item.MemoryID] = item.Compressed
		}
		for _, item := range group {
			compressed := strings.TrimSpace(byID[item.memoryID])
			if compressed == "" {
				compressOne(item)
				continue
			}
			newIDs, err := ltm.StoreDocument(item.concept, compressed)
			if err != nil {
				logger.Warn("[AutoOptimize] Failed to store compressed memory", "doc_id", item.docID, "error", err)
				continue
			}
			_ = ltm.DeleteDocument(item.docID)
			_ = stm.DeleteMemoryMeta(item.docID)
			for _, newID := range newIDs {
				_ = stm.UpsertMemoryMeta(newID)
			}
		}
		i = end
	}

	// Optimize Knowledge Graph
	graphRemoved := 0
	if kg != nil {
		graphRemoved, _ = kg.OptimizeGraph(threshold)
	}

	if len(lowDocs) > 0 || len(mediumDocs) > 0 || graphRemoved > 0 {
		logger.Info("[AutoOptimize] Memory optimization complete",
			"low_removed", len(lowDocs),
			"medium_compressed", len(mediumDocs),
			"graph_nodes_removed", graphRemoved)
	}
}

// SyncContactsToKnowledgeGraph synchronizes contacts to the knowledge graph.
func SyncContactsToKnowledgeGraph(ctx context.Context, contactsDB *sql.DB, kg *memory.KnowledgeGraph, logger *slog.Logger) {
	if contactsDB == nil || kg == nil {
		return
	}

	logger.Info("[Maintenance] Syncing Contacts to Knowledge Graph")

	rows, err := contactsDB.QueryContext(ctx, "SELECT id, name, email, phone, mobile, relationship FROM contacts")
	if err != nil {
		logger.Error("[Maintenance] Failed to query contacts for KG sync", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, name string
		var email, phone, mobile, relationship sql.NullString
		if err := rows.Scan(&id, &name, &email, &phone, &mobile, &relationship); err != nil {
			logger.Error("[Maintenance] Failed to scan contact", "error", err)
			continue
		}

		nodeID := "contact_" + id
		props := map[string]string{
			"type": "person",
		}
		if email.Valid && email.String != "" {
			props["email"] = email.String
		}
		if phone.Valid && phone.String != "" {
			props["phone"] = phone.String
		}
		if mobile.Valid && mobile.String != "" {
			props["mobile"] = mobile.String
		}
		if relationship.Valid && relationship.String != "" {
			props["relationship"] = relationship.String
		}

		err := kg.AddNode(nodeID, name, props)
		if err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			logger.Debug("[Maintenance] AddNode returned error", "nodeID", nodeID, "error", err)
		}

		if relationship.Valid && relationship.String != "" {
			relSlug := strings.ToLower(strings.ReplaceAll(relationship.String, " ", "_"))
			relNodeID := "org_" + relSlug

			_ = kg.AddNode(relNodeID, relationship.String, map[string]string{"type": "organization"})
			_ = kg.AddEdge(nodeID, relNodeID, "belongs_to", nil)
		}
	}
}

// SyncPlannerToKnowledgeGraph synchronizes appointments and todos to the knowledge graph.
func SyncPlannerToKnowledgeGraph(ctx context.Context, plannerDB *sql.DB, kg planner.KnowledgeGraph, logger *slog.Logger) {
	if plannerDB == nil || kg == nil {
		return
	}

	logger.Info("[Maintenance] Syncing Planner to Knowledge Graph")

	appointments, err := planner.ListAppointments(plannerDB, "", "")
	if err != nil {
		logger.Error("[Maintenance] Failed to list appointments for KG sync", "error", err)
	} else {
		for _, a := range appointments {
			if a.Status == "cancelled" {
				continue
			}
			props := map[string]string{
				"type":   "event",
				"source": "planner",
				"date":   a.DateTime,
				"status": a.Status,
			}
			if a.Description != "" {
				props["description"] = a.Description
			}
			if err := kg.AddNode(a.KGNodeID, a.Title, props); err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				logger.Debug("[Maintenance] AddNode returned error", "nodeID", a.KGNodeID, "error", err)
			}
		}
	}

	todos, err := planner.ListTodos(plannerDB, "", "")
	if err != nil {
		logger.Error("[Maintenance] Failed to list todos for KG sync", "error", err)
		return
	}
	for _, t := range todos {
		props := map[string]string{
			"type":     "task",
			"source":   "planner",
			"priority": t.Priority,
			"status":   t.Status,
		}
		if t.DueDate != "" {
			props["due_date"] = t.DueDate
		}
		if t.Description != "" {
			props["description"] = t.Description
		}
		if err := kg.AddNode(t.KGNodeID, t.Title, props); err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			logger.Debug("[Maintenance] AddNode returned error", "nodeID", t.KGNodeID, "error", err)
		}
	}
}

// SyncCoreMemoryToKnowledgeGraph synchronizes core memory facts to the knowledge graph.
func SyncCoreMemoryToKnowledgeGraph(ctx context.Context, stm *memory.SQLiteMemory, kg *memory.KnowledgeGraph, logger *slog.Logger) {
	if stm == nil || kg == nil {
		return
	}

	logger.Info("[Maintenance] Syncing Core Memory to Knowledge Graph")

	facts, err := stm.GetCoreMemoryFacts()
	if err != nil {
		logger.Error("[Maintenance] Failed to get core memory facts for KG sync", "error", err)
		return
	}

	for _, fact := range facts {
		nodeID := fmt.Sprintf("core_fact_%d", fact.ID)
		label := fact.Fact
		if len(label) > 50 {
			label = label[:47] + "..."
		}
		props := map[string]string{
			"type":    "concept",
			"content": fact.Fact,
		}

		err := kg.AddNode(nodeID, label, props)
		if err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			logger.Debug("[Maintenance] AddNode returned error", "nodeID", nodeID, "error", err)
		}
	}
}
