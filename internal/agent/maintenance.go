package agent

import (
	"context"
	"database/sql"
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
func StartMaintenanceLoop(ctx context.Context, cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, historyMgr *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManager *tools.MissionManager) {
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
				runMaintenanceTask(cfg, logger, llmClient, vault, registry, manifest, cronManager, longTermMem, shortTermMem, historyMgr, kg, inventoryDB, missionManager)
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

func runMaintenanceTask(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, historyMgr *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManager *tools.MissionManager) {
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
	}

	// Phase D8: Personality Engine maintenance — trait decay + journal
	if cfg.Agent.PersonalityEngine && shortTermMem != nil {
		personalityMaintenance(cfg, shortTermMem, logger)
	}

	// User Profile cleanup: remove stale low-confidence entries
	if cfg.Agent.UserProfiling && shortTermMem != nil {
		removed, err := shortTermMem.CleanupStaleProfileEntries(30)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean stale profile entries", "error", err)
		} else if removed > 0 {
			logger.Info("[Maintenance] Cleaned stale user profile entries", "removed", removed)
		}
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
		Config:          cfg,
		Logger:          logger,
		LLMClient:       client,
		ShortTermMem:    shortTermMem,
		HistoryManager:  historyMgr,
		LongTermMem:     longTermMem,
		KG:              kg,
		InventoryDB:     inventoryDB,
		Vault:           vault,
		Registry:        registry,
		Manifest:        manifest,
		CronManager:     cronManager,
		MissionManager:  missionManager,
		CoAgentRegistry: nil,
		BudgetTracker:   nil,
		SessionID:       sessionID,
		IsMaintenance:   false,
		SurgeryPlan:     "",
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
	meta := prompts.GetCorePersonalityMeta(cfg.Directories.PromptsDir, cfg.Agent.CorePersonality)
	decayAmount := 0.002 * meta.TraitDecayRate
	if err := stm.DecayAllTraits(decayAmount); err != nil {
		logger.Error("[Personality] Trait decay failed", "error", err)
	} else {
		logger.Info("[Personality] Daily trait decay applied", "amount", decayAmount, "decay_rate", meta.TraitDecayRate)
	}

	// 2. Character journal: append today's snapshot to data/character_journal.md
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
