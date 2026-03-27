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
	"aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

// StartMaintenanceLoop spawns a background goroutine that runs daily at the configured time.
func StartMaintenanceLoop(ctx context.Context, cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, historyMgr *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManagerV2 *tools.MissionManagerV2) {
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
				runMaintenanceTask(cfg, logger, llmClient, vault, registry, manifest, cronManager, longTermMem, shortTermMem, historyMgr, kg, inventoryDB, missionManagerV2)
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

func runMaintenanceTask(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, historyMgr *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManagerV2 *tools.MissionManagerV2) {
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

	// Journal: generate daily summary from today's journal entries
	if cfg.Tools.Journal.Enabled && cfg.Journal.DailySummary && shortTermMem != nil {
		generateDailySummary(cfg, logger, client, shortTermMem)
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

	// Knowledge Graph: nightly batch entity extraction from recent conversations
	if cfg.Tools.KnowledgeGraph.Enabled && cfg.Tools.KnowledgeGraph.AutoExtraction && kg != nil && shortTermMem != nil {
		extractKGEntities(cfg, logger, client, shortTermMem, kg)
	}

	// STM→LTM Consolidation: extract knowledge from archived messages into VectorDB
	if cfg.Consolidation.Enabled && shortTermMem != nil && longTermMem != nil && !longTermMem.IsDisabled() {
		consolidateSTMtoLTM(cfg, logger, client, shortTermMem, longTermMem, kg)
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

	// Build context for LLM
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", e.EntryType, e.Title, e.Content))
	}

	prompt := fmt.Sprintf(`Summarize the following activity log from today (%s) in 2-3 concise sentences.
Focus on: what was accomplished, key decisions, and notable events.
Output ONLY the summary text, no JSON or formatting.

Activity log:
%s`, today, sb.String())

	resp, err := llm.ExecuteWithRetry(
		context.Background(),
		client,
		openai.ChatCompletionRequest{
			Model: cfg.LLM.Model,
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
		logger.Warn("[Journal] Failed to generate daily summary via LLM", "error", err)
		return
	}

	summary := memory.DailySummary{
		Date:      today,
		Summary:   resp.Choices[0].Message.Content,
		Sentiment: "neutral",
	}

	// Gather tool usage stats from journal tags
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
	} else {
		logger.Info("[Journal] Daily summary stored", "date", today)
		anchor := summary.Summary
		if idx := strings.Index(anchor, "."); idx > 20 {
			anchor = strings.TrimSpace(anchor[:idx+1])
		}
		if len(anchor) > 220 {
			anchor = strings.TrimSpace(anchor[:220]) + "…"
		}
		if err := stm.UpsertDayAnchor(today, anchor); err != nil {
			logger.Warn("[Journal] Failed to store day anchor", "error", err)
		}
	}
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

// extractKGEntities performs nightly batch entity extraction from the past 24h of messages.
// Uses an LLM call to extract entities and relationships, then bulk-adds to the knowledge graph.
func extractKGEntities(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, stm *memory.SQLiteMemory, kg *memory.KnowledgeGraph) {
	// Collect recent messages (past 24h)
	messages, err := stm.GetRecentMessages("default", 100)
	if err != nil || len(messages) == 0 {
		logger.Debug("[KG] No recent messages for entity extraction")
		return
	}

	// Build conversation excerpt for the LLM
	var sb strings.Builder
	for _, m := range messages {
		if m.Role == "system" || m.Content == "" {
			continue
		}
		// Cap individual messages
		content := m.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, content))
		if sb.Len() > 8000 {
			break
		}
	}

	if sb.Len() < 50 {
		logger.Debug("[KG] Not enough conversation content for entity extraction")
		return
	}

	prompt := fmt.Sprintf(`Extract entities and relationships from this conversation.
Return ONLY valid JSON with this exact structure:
{
  "nodes": [{"id": "lowercase_id", "label": "Display Label", "properties": {"type": "person|place|tool|project|concept|device|service"}}],
  "edges": [{"source": "node_id", "target": "node_id", "relation": "relationship_type"}]
}

Rules:
- IDs must be lowercase with underscores (e.g. "john_doe", "home_server")
- Extract people, places, devices, services, projects, concepts mentioned
- Extract relationships like "owns", "uses", "manages", "works_on", "located_at", "connected_to"
- Only extract clear, factual entities — not vague references
- Maximum 15 nodes and 20 edges

Conversation:
%s`, sb.String())

	resp, err := llm.ExecuteWithRetry(
		context.Background(),
		client,
		openai.ChatCompletionRequest{
			Model: cfg.LLM.Model,
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
		logger.Warn("[KG] Entity extraction LLM call failed", "error", err)
		return
	}

	// Parse the LLM response
	rawJSON := resp.Choices[0].Message.Content
	// Strip markdown fences if present
	rawJSON = strings.TrimSpace(rawJSON)
	if strings.HasPrefix(rawJSON, "```") {
		lines := strings.Split(rawJSON, "\n")
		if len(lines) > 2 {
			rawJSON = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var extracted struct {
		Nodes []memory.Node `json:"nodes"`
		Edges []memory.Edge `json:"edges"`
	}
	if err := json.Unmarshal([]byte(rawJSON), &extracted); err != nil {
		logger.Warn("[KG] Failed to parse entity extraction JSON", "error", err, "raw_len", len(rawJSON))
		return
	}

	if len(extracted.Nodes) == 0 && len(extracted.Edges) == 0 {
		logger.Debug("[KG] No entities extracted")
		return
	}

	// Mark extracted nodes with source metadata
	for i := range extracted.Nodes {
		if extracted.Nodes[i].Properties == nil {
			extracted.Nodes[i].Properties = make(map[string]string)
		}
		extracted.Nodes[i].Properties["source"] = "auto_extraction"
		extracted.Nodes[i].Properties["extracted_at"] = time.Now().Format("2006-01-02")
	}

	if err := kg.BulkAddEntities(extracted.Nodes, extracted.Edges); err != nil {
		logger.Error("[KG] Failed to bulk-add extracted entities", "error", err)
		return
	}

	logger.Info("[KG] Nightly entity extraction complete", "nodes", len(extracted.Nodes), "edges", len(extracted.Edges))
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

	for i, batch := range batches {
		// Build conversation text from batch
		var sb strings.Builder
		var batchIDs []int64
		for _, msg := range batch {
			sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", msg.Timestamp, msg.Role, msg.Content))
			batchIDs = append(batchIDs, msg.ID)
		}

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
%s`, sb.String())

		resp, err := llm.ExecuteWithRetry(
			context.Background(),
			client,
			openai.ChatCompletionRequest{
				Model: cfg.LLM.Model,
				Messages: []openai.ChatCompletionMessage{
					{Role: openai.ChatMessageRoleSystem, Content: "You are a knowledge extraction engine. Extract factual knowledge from conversations. Output ONLY valid JSON, no markdown fences."},
					{Role: openai.ChatMessageRoleUser, Content: prompt},
				},
				MaxTokens: 1000,
			},
			logger,
			nil,
		)
		if err != nil || len(resp.Choices) == 0 {
			logger.Warn("[Consolidation] LLM extraction failed for batch", "batch", i+1, "error", err)
			_ = stm.MarkConsolidationFailure(batchIDs, fmt.Sprintf("llm extraction failed: %v", err))
			continue
		}

		// Parse LLM response
		rawJSON := strings.TrimSpace(resp.Choices[0].Message.Content)
		if strings.HasPrefix(rawJSON, "```") {
			lines := strings.Split(rawJSON, "\n")
			if len(lines) > 2 {
				rawJSON = strings.Join(lines[1:len(lines)-1], "\n")
			}
		}

		var extracted struct {
			Facts []struct {
				Concept string `json:"concept"`
				Content string `json:"content"`
			} `json:"facts"`
		}
		if err := json.Unmarshal([]byte(rawJSON), &extracted); err != nil {
			logger.Warn("[Consolidation] Failed to parse extraction JSON", "batch", i+1, "error", err)
			_ = stm.MarkConsolidationFailure(batchIDs, fmt.Sprintf("json parse failed: %v", err))
			continue
		}

		// Store extracted facts in VectorDB
		for _, fact := range extracted.Facts {
			if fact.Concept == "" || fact.Content == "" {
				continue
			}
			ids, err := ltm.StoreDocument(fact.Concept, fact.Content)
			if err != nil {
				logger.Warn("[Consolidation] Failed to store fact in LTM", "concept", fact.Concept, "error", err)
				continue
			}
			// Track metadata for priority-based forgetting
			for _, id := range ids {
				_ = stm.UpsertMemoryMeta(id)
			}
			totalStored++
		}

		eventDate := time.Now().Format("2006-01-02")
		if len(batch) > 0 && len(batch[0].Timestamp) >= 10 {
			eventDate = batch[0].Timestamp[:10]
		}
		episodeTitle := "Consolidated conversation batch"
		episodeSummary := fmt.Sprintf("%d messages, %d facts extracted", len(batch), len(extracted.Facts))
		episodeDetails := map[string]string{
			"session_id": batch[0].SessionID,
			"batch":      fmt.Sprintf("%d/%d", i+1, len(batches)),
		}
		_ = stm.InsertEpisodicMemoryWithDetails(eventDate, episodeTitle, episodeSummary, episodeDetails, 2, "consolidation", memory.EpisodicMemoryDetails{
			SessionID:        batch[0].SessionID,
			Participants:     []string{"user", "agent"},
			EmotionalValence: 0,
		})

		allConsolidatedIDs = append(allConsolidatedIDs, batchIDs...)
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

// autoOptimizeMemory runs priority-based forgetting on VectorDB and Knowledge Graph.
func autoOptimizeMemory(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, ltm memory.VectorDB, stm *memory.SQLiteMemory, kg *memory.KnowledgeGraph) {
	threshold := cfg.Consolidation.OptimizeThreshold

	metas, err := stm.GetAllMemoryMeta()
	if err != nil {
		logger.Error("[AutoOptimize] Failed to fetch memory metadata", "error", err)
		return
	}

	var lowDocs, mediumDocs []string
	for _, meta := range metas {
		if meta.Protected || meta.KeepForever {
			continue
		}
		lastA, err := time.Parse(time.RFC3339, strings.Replace(meta.LastAccessed, " ", "T", 1)+"Z")
		daysSince := 0
		if err == nil {
			daysSince = int(time.Since(lastA).Hours() / 24)
		}
		priority := meta.AccessCount - daysSince
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
	for _, docID := range mediumDocs {
		content, err := ltm.GetByID(docID)
		if err != nil || len(content) < 300 {
			continue
		}
		resp, err := llm.ExecuteWithRetry(
			context.Background(),
			client,
			openai.ChatCompletionRequest{
				Model: cfg.LLM.Model,
				Messages: []openai.ChatCompletionMessage{
					{Role: openai.ChatMessageRoleSystem, Content: "Compress this memory into a dense bullet-point list of core facts. Output ONLY the compressed text."},
					{Role: openai.ChatMessageRoleUser, Content: content},
				},
				MaxTokens: 500,
			},
			logger,
			nil,
		)
		if err == nil && len(resp.Choices) > 0 {
			compressed := resp.Choices[0].Message.Content
			parts := strings.SplitN(content, "\n\n", 2)
			concept := "Compressed Memory"
			if len(parts) == 2 {
				concept = parts[0]
			}
			newIDs, err2 := ltm.StoreDocument(concept, compressed)
			if err2 == nil {
				_ = ltm.DeleteDocument(docID)
				_ = stm.DeleteMemoryMeta(docID)
				for _, newID := range newIDs {
					_ = stm.UpsertMemoryMeta(newID)
				}
			}
		}
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
