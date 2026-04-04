package optimizer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

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

	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runEvaluationCycle(ctx)
			w.runCreationCycle(ctx)
		}
	}
}

func (w *OptimizerWorker) runEvaluationCycle(ctx context.Context) {
	// Evaluate running shadow tests (v2 prompts)
	rows, err := w.db.db.Query(`
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
		err = w.db.db.QueryRow(`SELECT COUNT(*) FROM tool_traces WHERE tool_name = ? AND prompt_version = ?`, toolName, versionTag).Scan(&count)
		if err != nil || count < w.evaluationLimit {
			continue // Need more traces
		}

		// Got enough traces. Let's compare performance
		var newSuccessRate float64
		w.db.db.QueryRow(`
			SELECT CAST(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*) 
			FROM tool_traces 
			WHERE tool_name = ? AND prompt_version = ?`, toolName, versionTag).Scan(&newSuccessRate)

		var baselineSuccessRate float64
		w.db.db.QueryRow(`
			SELECT CAST(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*) 
                        FROM (
                                SELECT success
                                FROM tool_traces
                                WHERE tool_name = ? AND prompt_version = 'v1'
                                ORDER BY timestamp DESC LIMIT 50
                        )`, toolName).Scan(&baselineSuccessRate)

		if newSuccessRate >= baselineSuccessRate+0.1 {
			// Promote!
			w.db.db.Exec(`UPDATE prompt_overrides SET active = 1, shadow = 0 WHERE id = ?`, id)
			slog.Info("[Optimizer] Promoted shadow prompt to active", "tool", toolName, "gain", newSuccessRate-baselineSuccessRate)
		} else {
			// Rollback! Discard it
			w.db.db.Exec(`DELETE FROM prompt_overrides WHERE id = ?`, id)
			w.db.db.Exec(`UPDATE optimizer_metrics SET value = value + 1 WHERE key = 'rejected_mutations'`)
			slog.Info("[Optimizer] Rolled back and deleted shadow prompt", "tool", toolName, "reason", "no significant improvement")
		}
	}
}

func (w *OptimizerWorker) runCreationCycle(ctx context.Context) {
	// Find tools with high consecutive error counts or low success rates in last 7 days
	// Example: threshold < 0.6 success rate, minimum 10 traces
	rows, err := w.db.db.Query(`
		SELECT tool_name, CAST(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*) as success_rate, COUNT(*) as trace_count
		FROM tool_traces
		WHERE prompt_version = 'v1' AND timestamp > datetime('now', '-7 days')
		GROUP BY tool_name
		HAVING success_rate < 0.8 AND trace_count >= 5
		ORDER BY success_rate ASC LIMIT 1
	`)
	if err != nil {
		slog.Error("[Optimizer] Failed to find poorly performing tools", "error", err)
		return
	}
	defer rows.Close()

	if !rows.Next() {
		return // Nothing to optimize
	}

	var toolName string
	var succRate float64
	var traceCount int
	if err := rows.Scan(&toolName, &succRate, &traceCount); err != nil {
		return
	}
	rows.Close() // Finished with rows to free connection

	// Do not optimize if there's already a shadow prompt running for this tool
	var existing int
	w.db.db.QueryRow(`SELECT COUNT(*) FROM prompt_overrides WHERE tool_name = ? AND shadow = 1`, toolName).Scan(&existing)
	if existing > 0 {
		return
	}

	w.mutateToolPrompt(ctx, toolName)
}

func (w *OptimizerWorker) mutateToolPrompt(ctx context.Context, toolName string) {
	slog.Info("[Optimizer] Initiating self-reflection for tool", "tool", toolName)

	// Fetch recent error traces for context
	rows, err := w.db.db.Query(`SELECT error_message FROM tool_traces WHERE tool_name = ? AND success = 0 ORDER BY timestamp DESC LIMIT 5`, toolName)
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

	// You would typically load the current prompt content from embedded/disk here.
	// For demonstration, we just mock the reflection logic
	reflectionPrompt := fmt.Sprintf(`Rewrite the usage manual for the tool '%s'.
Recent execution errors:
%s
Ensure the instructions prevent these errors. Reply ONLY with the new markdown manual.`, toolName, errorsList)

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
		slog.Warn("[Optimizer] Helper LLM failed, falling back to Primary LLM", "error", err)
		resp, err = w.primaryManager.CreateChatCompletion(ctx, req)
		if err == nil && len(resp.Choices) > 0 {
			newPrompt = resp.Choices[0].Message.Content
		}
	}

	if err != nil || newPrompt == "" {
		slog.Error("[Optimizer] Failed to generate mutated prompt via all LLMs", "error", err)
		return
	}

	// Save as shadow prompt
	_, err = w.db.db.Exec(`INSERT INTO prompt_overrides (tool_name, mutated_prompt, active, shadow) VALUES (?, ?, 0, 1)`, toolName, newPrompt)
	if err != nil {
		slog.Error("[Optimizer] Failed to store mutated shadow prompt", "error", err)
	} else {
		slog.Info("[Optimizer] Successfully created shadow prompt test", "tool", toolName)
	}
}
