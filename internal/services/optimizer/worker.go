package optimizer

import (
	"aurago/internal/llm"
	"aurago/internal/prompts"
	promptsembed "aurago/prompts"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/sashabaranov/go-openai"
)

func init() {
	prompts.GetActivePromptOverrides = GetActivePromptOverrides
}

type OptimizerWorker struct {
	db              *OptimizerDB
	helperManager   *llm.FailoverManager // Helper LLM
	primaryManager  *llm.FailoverManager // Primary LLM
	checkInterval   time.Duration
	evaluationLimit int
}

func NewOptimizerWorker(db *OptimizerDB, helperManager, primaryManager *llm.FailoverManager, interval time.Duration) *OptimizerWorker {
	if interval == 0 {
		interval = 6 * time.Hour
	}
	return &OptimizerWorker{
		db:              db,
		helperManager:   helperManager,
		primaryManager:  primaryManager,
		checkInterval:   interval,
		evaluationLimit: 5, // evaluate after 5 trace calls
	}
}

func (w *OptimizerWorker) Start(ctx context.Context) {
	slog.Info("[Optimizer] Starting optimization background worker", "interval", w.checkInterval)

	w.runEvaluationCycle(ctx)
	w.runCreationCycle(ctx)
	w.pruneTraces(ctx)

	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runEvaluationCycle(ctx)
			w.runCreationCycle(ctx)
			w.pruneTraces(ctx)
		}
	}
}

func (w *OptimizerWorker) pruneTraces(ctx context.Context) {
	_, err := w.db.db.ExecContext(ctx, `DELETE FROM tool_traces WHERE timestamp < datetime('now', '-90 days')`)
	if err != nil {
		slog.Error("[Optimizer] Failed to prune traces", "error", err)
	}
}

func (w *OptimizerWorker) runEvaluationCycle(ctx context.Context) {
	// Evaluate running shadow tests (v2 prompts)
	rows, err := w.db.db.QueryContext(ctx, `
		SELECT id, tool_name, mutated_prompt
		FROM prompt_overrides
		WHERE active = 0 AND shadow = 1
	`)
	if err != nil {
		slog.Error("[Optimizer] Failed to query shadow tests", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var toolName, prompt string
		if err := rows.Scan(&id, &toolName, &prompt); err != nil {
			continue
		}

		// Check count of traces for this new prompt version
		var count int
		versionTag := fmt.Sprintf("v2-shadow-%d", id)
		err = w.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tool_traces WHERE tool_name = ? AND prompt_version = ?`, toolName, versionTag).Scan(&count)
		if err != nil || count < w.evaluationLimit {
			continue // Need more traces
		}

		// Got enough traces. Let's compare performance
		var newSuccessRate float64
		err = w.db.db.QueryRowContext(ctx, `
			SELECT CAST(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*) 
			FROM tool_traces 
			WHERE tool_name = ? AND prompt_version = ?`, toolName, versionTag).Scan(&newSuccessRate)
		if err != nil {
			continue
		}

		var baselineSuccessRate float64
		err = w.db.db.QueryRowContext(ctx, `
			SELECT CAST(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*) 
                        FROM (
                                SELECT success
                                FROM tool_traces
                                WHERE tool_name = ? AND prompt_version = 'v1'
                                ORDER BY timestamp DESC LIMIT 50
                        )`, toolName).Scan(&baselineSuccessRate)
		if err != nil {
			continue
		}

		if newSuccessRate >= baselineSuccessRate+0.1 {
			// Promote!
			w.db.db.ExecContext(ctx, "UPDATE prompt_overrides SET active = 0 WHERE tool_name = ? AND active = 1", toolName)
			w.db.db.ExecContext(ctx, `UPDATE prompt_overrides SET active = 1, shadow = 0 WHERE id = ?`, id)
			prompts.ClearPromptCache()
			slog.Info("[Optimizer] Promoted shadow prompt to active", "tool", toolName, "gain", newSuccessRate-baselineSuccessRate)
		} else {
			// Rollback! Discard it
			w.db.db.ExecContext(ctx, `DELETE FROM prompt_overrides WHERE id = ?`, id)
			w.db.db.ExecContext(ctx, `UPDATE optimizer_metrics SET value = value + 1 WHERE key = 'rejected_mutations'`)
			slog.Info("[Optimizer] Rolled back and deleted shadow prompt", "tool", toolName, "reason", "no significant improvement")
		}
	}
}

func (w *OptimizerWorker) runCreationCycle(ctx context.Context) {
	// Find tools with high consecutive error counts or low success rates in last 7 days
	// Example: threshold < 0.6 success rate, minimum 10 traces
	rows, err := w.db.db.QueryContext(ctx, `
		SELECT tool_name, CAST(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*) as success_rate, COUNT(*) as trace_count
		FROM tool_traces
		WHERE prompt_version = 'v1' AND timestamp > datetime('now', '-7 days')
		GROUP BY tool_name
		HAVING success_rate < 0.8 AND trace_count >= 5
		ORDER BY success_rate ASC LIMIT 3
	`)
	if err != nil {
		slog.Error("[Optimizer] Failed to find poorly performing tools", "error", err)
		return
	}
	defer rows.Close()

	var toolsToOptimize []string
	for rows.Next() {
		var toolName string
		var succRate float64
		var traceCount int
		if err := rows.Scan(&toolName, &succRate, &traceCount); err != nil {
			continue
		}
		toolsToOptimize = append(toolsToOptimize, toolName)
	}
	rows.Close()

	for _, toolName := range toolsToOptimize {
		// Do not optimize if there's already a shadow prompt running for this tool
		var existing int
		w.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM prompt_overrides WHERE tool_name = ? AND shadow = 1`, toolName).Scan(&existing)
		if existing > 0 {
			continue
		}

		w.mutateToolPrompt(ctx, toolName)
	}
}

func (w *OptimizerWorker) mutateToolPrompt(ctx context.Context, toolName string) {
	slog.Info("[Optimizer] Initiating self-reflection for tool", "tool", toolName)

	// Fetch recent error traces for context
	rows, err := w.db.db.QueryContext(ctx, `SELECT error_message FROM tool_traces WHERE tool_name = ? AND success = 0 ORDER BY timestamp DESC LIMIT 5`, toolName)
	if err != nil {
		return
	}
	defer rows.Close()

	var errorsList string
	for rows.Next() {
		var em string
		if err := rows.Scan(&em); err == nil {
			errorsList += "- " + em + "\n"
		}
	}

	// Load the current prompt content from embedded/disk
	var currentManual string
	safeToolName := filepath.Base(toolName)
	data, err := os.ReadFile("prompts/tools_manuals/" + safeToolName + ".md")
	if err != nil {
		// fallback to embed
		data, err = promptsembed.FS.ReadFile("tools_manuals/" + safeToolName + ".md")
	}
	if err == nil {
		currentManual = string(data)
	} else {
		currentManual = "(No existing manual found)"
	}

	reflectionPrompt := fmt.Sprintf(`Rewrite the usage manual for the tool '%s'.
Current manual:
<current_manual>
%s
</current_manual>

Recent execution errors:
%s
Ensure the instructions prevent these errors. Reply ONLY with the new markdown manual.`, toolName, currentManual, errorsList)

	// Fallback logic
	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: reflectionPrompt},
		},
	}
	var newPrompt string
	resp, err := w.helperManager.CreateChatCompletion(ctx, req)
	if err == nil && len(resp.Choices) > 0 {
		newPrompt = resp.Choices[0].Message.Content
	}
	if err != nil || newPrompt == "" {
		helperReason := "empty response (no choices)"
		if err != nil {
			helperReason = err.Error()
		}
		slog.Warn("[Optimizer] Helper LLM failed, falling back to Primary LLM", "reason", helperReason)
		resp, err = w.primaryManager.CreateChatCompletion(ctx, req)
		if err == nil && len(resp.Choices) > 0 {
			newPrompt = resp.Choices[0].Message.Content
		}
	}

	if err != nil || newPrompt == "" {
		finalReason := "empty response (no choices)"
		if err != nil {
			finalReason = err.Error()
		}
		slog.Error("[Optimizer] Failed to generate mutated prompt via all LLMs", "reason", finalReason)
		return
	}

	hash := sha256.Sum256([]byte(currentManual))
	hashStr := hex.EncodeToString(hash[:])

	w.db.db.ExecContext(ctx, `DELETE FROM prompt_overrides WHERE tool_name = ? AND shadow = 1`, toolName)

	_, err = w.db.db.ExecContext(ctx, `INSERT INTO prompt_overrides (tool_name, mutated_prompt, original_hash, active, shadow) VALUES (?, ?, ?, 0, 1)`, toolName, newPrompt, hashStr)
	if err != nil {
		slog.Error("[Optimizer] Failed to store mutated shadow prompt", "error", err)
	} else {
		slog.Info("[Optimizer] Successfully created shadow prompt test", "tool", toolName)
	}
}
