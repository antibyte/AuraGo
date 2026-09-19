package optimizer

import (
	"aurago/internal/llm"
	"aurago/internal/prompts"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/sashabaranov/go-openai"
)

func init() {
	prompts.GetActivePromptOverrides = GetActivePromptOverrides
	prompts.SelectToolGuideVariant = selectToolGuideVariant
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
	if ctx.Err() != nil {
		return
	}
	w.runCreationCycle(ctx)
	if ctx.Err() != nil {
		return
	}
	w.pruneTraces(ctx)

	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runEvaluationCycle(ctx)
			if ctx.Err() != nil {
				return
			}
			w.runCreationCycle(ctx)
			if ctx.Err() != nil {
				return
			}
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
	// Evaluate running shadow tests (v2 prompts).
	// Collect all rows first, then close before issuing any further DB calls.
	// Issuing QueryRow/Exec while a rows cursor is open violates the MaxOpenConns
	// invariant and causes a self-deadlock on the connection pool.
	rows, err := w.db.db.QueryContext(ctx, `
		SELECT id, tool_name, mutated_prompt
		FROM prompt_overrides
		WHERE active = 0 AND shadow = 1
	`)
	if err != nil {
		slog.Error("[Optimizer] Failed to query shadow tests", "error", err)
		return
	}

	type shadowEntry struct {
		id       int
		toolName string
		prompt   string
	}
	var entries []shadowEntry
	for rows.Next() {
		var e shadowEntry
		if err := rows.Scan(&e.id, &e.toolName, &e.prompt); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	rows.Close() // must close before any further DB calls

	for _, e := range entries {
		id, toolName := e.id, e.toolName

		versionTag := fmt.Sprintf("v2-shadow-%d", id)
		baselineTag := "v1"
		var activeID int
		if w.db.db.QueryRowContext(ctx, `SELECT id FROM prompt_overrides WHERE tool_name=? AND active=1 ORDER BY id DESC LIMIT 1`, toolName).Scan(&activeID) == nil {
			baselineTag = fmt.Sprintf("active-%d", activeID)
		}
		newSuccessRate, baselineSuccessRate, comparable := w.comparableRates(ctx, toolName, versionTag, baselineTag)
		if !comparable {
			continue
		}

		if newSuccessRate >= baselineSuccessRate+0.1 {
			// Promote!
			tx, txErr := w.db.db.BeginTx(ctx, nil)
			if txErr != nil {
				slog.Warn("[Optimizer] Failed to begin prompt promotion", "tool", toolName, "error", txErr)
				continue
			}
			if _, txErr = tx.ExecContext(ctx, "UPDATE prompt_overrides SET active = 0 WHERE tool_name = ? AND active = 1", toolName); txErr == nil {
				_, txErr = tx.ExecContext(ctx, `UPDATE prompt_overrides SET active = 1, shadow = 0, promotion_reason='comparable_exposures_gain_at_least_0.10' WHERE id = ?`, id)
			}
			if txErr == nil {
				txErr = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
			if txErr != nil {
				slog.Warn("[Optimizer] Failed to persist prompt promotion", "tool", toolName, "error", txErr)
				continue
			}
			prompts.ClearPromptCache()
			slog.Info("[Optimizer] Promoted shadow prompt to active", "tool", toolName, "gain", newSuccessRate-baselineSuccessRate)
		} else {
			// Rollback! Discard it
			w.db.db.ExecContext(ctx, `UPDATE prompt_overrides SET shadow=0, promotion_reason='insufficient_comparable_gain' WHERE id=?`, id)
			w.db.db.ExecContext(ctx, `UPDATE optimizer_metrics SET value = value + 1 WHERE key = 'rejected_mutations'`)
			slog.Info("[Optimizer] Retired shadow prompt", "tool", toolName, "reason", "no significant improvement")
		}
	}
}

func (w *OptimizerWorker) runCreationCycle(ctx context.Context) {
	// Find tools with high consecutive error counts or low success rates in last 7 days
	// Example: threshold < 0.6 success rate, minimum 10 traces
	rows, err := w.db.db.QueryContext(ctx, `
		SELECT tool_name, CAST(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*) as success_rate, COUNT(*) as trace_count
		FROM tool_traces
		WHERE exposure_id IS NOT NULL AND action_identity<>'' AND prompt_version = 'v1' AND timestamp > datetime('now', '-7 days')
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

	// Fetch recent error traces for context.
	// IMPORTANT: rows must be explicitly closed before the LLM call below.
	// The optimizer DB uses MaxOpenConns(1). If rows is still open when the LLM
	// call blocks (e.g. model timeout), GetActivePromptOverrides in the agent loop
	// will wait forever for a connection → chat hangs indefinitely.
	rows, err := w.db.db.QueryContext(ctx, `SELECT error_message FROM tool_traces WHERE tool_name = ? AND success = 0 ORDER BY timestamp DESC LIMIT 5`, toolName)
	if err != nil {
		return
	}

	var errorsList string
	for rows.Next() {
		var em string
		if err := rows.Scan(&em); err == nil {
			errorsList += "- " + em + "\n"
		}
	}
	rows.Close() // explicit close — must happen before any LLM call

	// Load from the runtime-configured prompt source with embedded fallback.
	currentManual := w.db.loadCanonicalManual(toolName)

	reflectionPrompt := fmt.Sprintf(`Rewrite the usage manual for the tool '%s'.
Current manual:
<current_manual>
%s
</current_manual>

Recent execution errors:
%s
Ensure the instructions prevent these errors. Reply ONLY with the new markdown manual.`, toolName, currentManual, errorsList)

	// Use a bounded context for LLM calls — the optimizer must not block the server
	// indefinitely if the LLM is slow or unresponsive.
	llmCtx, llmCancel := context.WithTimeout(ctx, 60*time.Second)
	defer llmCancel()

	// Fallback logic
	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: reflectionPrompt},
		},
	}
	var newPrompt string
	resp, err := w.helperManager.CreateChatCompletion(llmCtx, req)
	if err == nil && len(resp.Choices) > 0 {
		newPrompt = resp.Choices[0].Message.Content
	}
	if err != nil || newPrompt == "" {
		helperReason := "empty response (no choices)"
		if err != nil {
			helperReason = err.Error()
		}
		slog.Warn("[Optimizer] Helper LLM failed, falling back to Primary LLM", "reason", helperReason)
		resp, err = w.primaryManager.CreateChatCompletion(llmCtx, req)
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

	sanitizedPrompt, ok := prompts.SanitizeToolGuideOverride(newPrompt)
	if !ok {
		slog.Warn("[Optimizer] Rejected mutated prompt with unsafe or unusable content", "tool", toolName)
		return
	}
	newPrompt = sanitizedPrompt

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
