package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

// StartDailyReflectionLoop spawns a background goroutine that runs every 24 hours
// at 03:00 AM local time to reflect on recent knowledge updates and produce a morning briefing.
func StartDailyReflectionLoop(ctx context.Context, cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, historyMgr *memory.HistoryManager, shortTermMem *memory.SQLiteMemory) {
	go func() {
		logger.Info("Started System-Level Daily Reflection Loop (wakes up daily at 03:00 AM)")
		for {
			now := time.Now()
			// Calculate next 03:00 AM
			nextRun := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
			if now.After(nextRun) || now.Equal(nextRun) {
				nextRun = nextRun.Add(24 * time.Hour)
			}

			sleepDuration := nextRun.Sub(now)
			logger.Debug("Daily reflection loop sleeping", "next_run", nextRun, "duration_hours", sleepDuration.Hours())

			select {
			case <-time.After(sleepDuration):
				runDailyReflection(cfg, logger, llmClient, historyMgr, shortTermMem)
			case <-ctx.Done():
				logger.Info("Daily reflection loop shutting down")
				return
			}
		}
	}()
}

func runDailyReflection(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, historyMgr *memory.HistoryManager, shortTermMem *memory.SQLiteMemory) {
	logger.Info("[DailyReflection] Waking up to process daily summary")

	// 1. Gather Context
	rollingSummary := historyMgr.GetSummary()
	recentArchives, err := shortTermMem.GetRecentArchiveEvents(24)
	if err != nil {
		logger.Error("[DailyReflection] Failed to fetch recent archives", "error", err)
		return
	}

	if len(recentArchives) == 0 && rollingSummary == "" {
		logger.Info("[DailyReflection] Nothing to reflect on today")
		// No activity, skip reflection to save tokens
		return
	}

	archivesText := "None"
	if len(recentArchives) > 0 {
		archivesText = ""
		for _, a := range recentArchives {
			archivesText += "- " + a + "\n"
		}
	}

	summaryText := "None"
	if rollingSummary != "" {
		summaryText = rollingSummary
	}

	// 2. Build Prompt
	prompt := `You are an autonomous Supervisor Agent performing your daily 03:00 AM reflection.
Reflect on today's progress. Update the Rolling Summary with new permanent facts and identify contradictions or missing information.
Output the updated Summary and a short 'Morning Briefing' for the user.
The output MUST be a strict JSON object with two fields: "summary" and "briefing". Do not include markdown formatting or extra text.

### Persistent Summary (Current)
` + summaryText + `

### New Knowledge Archived in the last 24h
` + archivesText

	req := openai.ChatCompletionRequest{
		Model: cfg.LLM.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: prompt},
		},
		MaxTokens:   1500,
		Temperature: 0.3,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.CircuitBreaker.LLMTimeoutSeconds)*time.Second)
	defer cancel()

	// Parse intervals from config
	intervals := make([]time.Duration, len(cfg.CircuitBreaker.RetryIntervals))
	for i, s := range cfg.CircuitBreaker.RetryIntervals {
		d, err := time.ParseDuration(s)
		if err != nil {
			logger.Warn("[DailyReflection] Failed to parse retry interval, fallback to 10s", "input", s)
			d = 10 * time.Second
		}
		intervals[i] = d
	}

	resp, err := llm.ExecuteWithCustomRetry(ctx, client, req, logger, nil, intervals, 10*time.Minute)
	if err != nil {
		logger.Error("[DailyReflection] LLM API call failed", "error", err)
		return
	}

	if len(resp.Choices) == 0 {
		logger.Warn("[DailyReflection] LLM returned empty choices")
		return
	}

	content := resp.Choices[0].Message.Content

	// 3. Parse JSON
	type ReflectionOutput struct {
		Summary  string `json:"summary"`
		Briefing string `json:"briefing"`
	}

	var output ReflectionOutput
	if err := json.Unmarshal([]byte(content), &output); err != nil {
		logger.Error("[DailyReflection] Failed to parse JSON output", "error", err, "content", content)
		return
	}

	// 4. Update the actual databases
	if output.Summary != "" {
		historyMgr.SetSummary(output.Summary)
	}
	if output.Briefing != "" {
		shortTermMem.AddNotification(output.Briefing)
	}

	logger.Info("[DailyReflection] Successfully completed daily reflection", "briefing_length", len(output.Briefing))
}
