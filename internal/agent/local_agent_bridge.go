package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/security"
)

const localAgentActivitySource = "agodesk_local_agent"

// LocalAgentTurnSync contains the bounded, already-sanitized data captured for
// one turn executed by a paired local agent.
type LocalAgentTurnSync struct {
	SessionID        string
	UserMessage      string
	AssistantMessage string
	Status           string
	Provider         string
	ClientTimestamp  time.Time
	ToolNames        []string
	ToolSummaries    []string
}

// QueryMemoryForLocalAgent exposes the same memory search implementation used
// by the native query_memory tool without exposing the agent loop.
func QueryMemoryForLocalAgent(query string, limit int, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, kg *memory.KnowledgeGraph, plannerDB *sql.DB, cheatsheetDB *sql.DB) (map[string]interface{}, error) {
	raw, err := executeQueryMemory(ToolCall{
		Content: strings.TrimSpace(query),
		Limit:   limit,
	}, shortTermMem, longTermMem, kg, plannerDB, cheatsheetDB)
	if err != nil {
		return nil, err
	}
	return decodeLocalAgentToolOutput(raw)
}

// RecallMemoryForLocalAgent exposes the same identifier lookup and secret
// scrubbing used by the native recall_memory tool.
func RecallMemoryForLocalAgent(id string, longTermMem memory.VectorDB) (map[string]interface{}, error) {
	raw, err := executeRecallMemory(ToolCall{ID: strings.TrimSpace(id)}, longTermMem)
	if err != nil {
		return nil, err
	}
	return decodeLocalAgentToolOutput(raw)
}

func decodeLocalAgentToolOutput(raw string) (map[string]interface{}, error) {
	raw = security.Scrub(strings.TrimSpace(strings.TrimPrefix(raw, "Tool Output:")))
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("decode local agent tool output: %w", err)
	}
	if status, _ := result["status"].(string); status == "error" {
		return nil, fmt.Errorf("memory operation failed")
	}
	if _, hasSourceErrors := result["errors"]; hasSourceErrors {
		result["errors"] = []string{"One or more memory sources were unavailable."}
	}
	return result, nil
}

// SyncLocalAgentTurn records activity, journal, and eligible long-term memory
// side effects for a turn that was executed by a paired local agent.
func SyncLocalAgentTurn(ctx context.Context, cfg *config.Config, logger *slog.Logger, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, kg *memory.KnowledgeGraph, turn LocalAgentTurnSync) error {
	if shortTermMem == nil {
		return fmt.Errorf("short-term memory unavailable")
	}
	if logger == nil {
		logger = slog.Default()
	}

	userMessage := security.Scrub(strings.TrimSpace(turn.UserMessage))
	assistantMessage := security.Scrub(strings.TrimSpace(turn.AssistantMessage))
	toolNames := uniqueActivityStrings(turn.ToolNames, 12)
	toolSummaries := uniqueActivityStrings(turn.ToolSummaries, 12)
	digest := buildActivityDigest(userMessage, assistantMessage, toolNames, toolSummaries, shortTermMem)
	status := normalizeLocalAgentTurnStatus(turn.Status)
	if provider := strings.TrimSpace(turn.Provider); provider != "" {
		digest.ImportantPoints = append(digest.ImportantPoints, "Provider: "+truncateActivityDigestInput(provider, 120))
	}
	if status != "completed" {
		digest.Outcomes = []string{"Local agent turn " + status}
	}

	timestamp := turn.ClientTimestamp.UTC()
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	if err := insertLocalAgentActivityTurn(shortTermMem, kg, logger, turn.SessionID, userMessage, status, timestamp, toolNames, digest); err != nil {
		return err
	}

	if status == "completed" {
		JournalAutoTrigger(cfg, shortTermMem, logger, turn.SessionID, toolNames, userMessage)
		runMemoryAnalysis(ctx, cfg, logger, shortTermMem, kg, longTermMem, userMessage, assistantMessage, turn.SessionID)
		return nil
	}
	insertLocalAgentStatusJournal(cfg, shortTermMem, logger, turn.SessionID, status, userMessage, toolNames)
	return nil
}

func normalizeLocalAgentTurnStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed":
		return "failed"
	case "cancelled":
		return "cancelled"
	default:
		return "completed"
	}
}

func insertLocalAgentActivityTurn(shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph, logger *slog.Logger, sessionID, userMessage, status string, timestamp time.Time, toolNames []string, digest memory.ActivityDigest) error {
	turnID, err := shortTermMem.InsertActivityTurn(memory.ActivityTurn{
		Timestamp:       timestamp.Format(time.RFC3339),
		Date:            timestamp.Format("2006-01-02"),
		SessionID:       sessionID,
		Channel:         localAgentActivitySource,
		UserRelevant:    true,
		Status:          status,
		Importance:      digest.Importance,
		Intent:          digest.Intent,
		UserRequest:     userMessage,
		UserGoal:        digest.UserGoal,
		ActionsTaken:    digest.ActionsTaken,
		Outcomes:        digest.Outcomes,
		ImportantPoints: digest.ImportantPoints,
		PendingItems:    digest.PendingItems,
		ToolNames:       toolNames,
		LinkedMemoryIDs: digest.Entities,
		Source:          localAgentActivitySource,
	})
	if err != nil {
		return fmt.Errorf("insert local agent activity: %w", err)
	}
	if status == "completed" {
		syncActivityTurnToKnowledgeGraph(kg, turnID, timestamp.Format("2006-01-02"), sessionID, localAgentActivitySource, digest, localAgentActivitySource, logger)
	}
	return nil
}

func insertLocalAgentStatusJournal(cfg *config.Config, shortTermMem *memory.SQLiteMemory, logger *slog.Logger, sessionID, status, userMessage string, toolNames []string) {
	if cfg == nil || !cfg.Tools.Journal.Enabled || !cfg.Journal.AutoEntries {
		return
	}
	title := "Local agent turn " + status
	content := "User request: " + truncateActivityDigestInput(userMessage, 1200)
	if len(toolNames) > 0 {
		content += "\nTools: " + strings.Join(toolNames, ", ")
	}
	if _, err := shortTermMem.InsertJournalEntry(memory.JournalEntry{
		EntryType:     "activity",
		Title:         title,
		Content:       content,
		Tags:          append([]string{"activity", "local_agent", status}, toolNames...),
		Importance:    1,
		SessionID:     sessionID,
		AutoGenerated: true,
	}); err != nil {
		logger.Warn("[Journal] Failed to record local agent status", "status", status, "error", security.Scrub(err.Error()))
	}
}
