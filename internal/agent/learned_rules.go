package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/memory"
)

// GenerateLearnedRule creates a learned rule from a recurring error/recovery
// pattern and persists it to SQLite. It runs with a timeout and never blocks
// the caller.
func GenerateLearnedRule(
	ctx context.Context,
	stm *memory.SQLiteMemory,
	toolName, errorPattern, resolution string,
	logger *slog.Logger,
) {
	if stm == nil || toolName == "" || errorPattern == "" {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Fast path: infer a concise rule from the resolution without an LLM call.
	rule := inferRuleFromResolution(toolName, errorPattern, resolution)
	if rule != "" {
		lr := &memory.LearnedRule{
			ToolName:   toolName,
			Pattern:    errorPattern,
			Rule:       rule,
			Confidence: 0.5,
			Hits:       1,
			Active:     true,
		}
		if err := stm.UpsertLearnedRule(lr); err != nil {
			if logger != nil {
				logger.Warn("[AutoLearning] failed to upsert inferred rule", "tool", toolName, "error", err)
			}
		} else if logger != nil {
			logger.Info("[AutoLearning] inferred rule persisted",
				"tool", toolName, "rule", rule, "confidence", lr.Confidence)
		}
		return
	}

	// Fallback: generate a rule via a mini LLM call would go here.
	// For now we skip expensive LLM-based rule generation and rely on the
	// heuristic fast path. This keeps the feature lightweight and safe.
	if logger != nil {
		logger.Debug("[AutoLearning] no trivial rule inferred, skipping LLM generation",
			"tool", toolName, "pattern", errorPattern)
	}
}

// inferRuleFromResolution attempts to derive a concise rule from a known
// resolution string. Returns "" when no confident inference can be made.
func inferRuleFromResolution(toolName, errorPattern, resolution string) string {
	lowerErr := strings.ToLower(errorPattern)
	lowerRes := strings.ToLower(resolution)

	// Docker patterns
	if strings.Contains(toolName, "docker") {
		if strings.Contains(lowerErr, "port") && strings.Contains(lowerErr, "already in use") {
			return "docker port conflict: run 'docker ps' first to find the occupying container"
		}
		if strings.Contains(lowerErr, "container") && strings.Contains(lowerErr, "not found") {
			return "docker container missing: verify name with 'docker ps -a' before referencing"
		}
		if strings.Contains(lowerErr, "image") && strings.Contains(lowerErr, "not found") {
			return "docker image missing: pull explicitly or check registry/tag spelling"
		}
		if strings.Contains(lowerErr, "permission") {
			return "docker permission denied: ensure user is in 'docker' group or use sudo"
		}
	}

	// Shell / SSH patterns
	if toolName == "execute_shell" || toolName == "ssh_exec" || toolName == "execute_sudo" {
		if strings.Contains(lowerErr, "permission denied") || strings.Contains(lowerErr, "access denied") {
			return "permission denied: check file ownership (ls -l) or use sudo if appropriate"
		}
		if strings.Contains(lowerErr, "command not found") || strings.Contains(lowerErr, "not found") {
			return "command not found: verify the binary is installed and in $PATH"
		}
		if strings.Contains(lowerErr, "no such file") || strings.Contains(lowerErr, "does not exist") {
			return "file missing: verify path with 'ls' before operating on files"
		}
		if strings.Contains(lowerErr, "port") && strings.Contains(lowerErr, "refused") {
			return "connection refused: verify the service is running and listening on the expected port"
		}
		if strings.Contains(lowerErr, "timeout") || strings.Contains(lowerErr, "timed out") {
			return "network timeout: check connectivity (ping) and firewall rules before retrying"
		}
	}

	// Git patterns
	if strings.Contains(toolName, "git") {
		if strings.Contains(lowerErr, "merge conflict") || strings.Contains(lowerErr, "conflict") {
			return "git conflict: resolve manually or abort with 'git merge --abort' and re-plan"
		}
		if strings.Contains(lowerErr, "authentication") || strings.Contains(lowerErr, "credentials") {
			return "git auth failed: verify SSH key or personal access token is configured"
		}
	}

	// Generic resolution-based inference
	if strings.Contains(lowerRes, "adjusted parameters") || strings.Contains(lowerRes, "succeeded") {
		// Extract a hint from the error itself when the resolution is generic.
		if strings.Contains(lowerErr, "invalid") {
			return fmt.Sprintf("%s invalid input: double-check required parameters against the schema", toolName)
		}
		if strings.Contains(lowerErr, "timeout") {
			return fmt.Sprintf("%s timeout: increase timeout or verify target availability first", toolName)
		}
	}

	return ""
}

// RunSessionRetro performs a lightweight retrospective on session end (/reset).
// It scans recent unresolved errors and generates learned rules for patterns
// that have recurred. Runs asynchronously with a timeout.
func RunSessionRetro(
	ctx context.Context,
	stm *memory.SQLiteMemory,
	sessionID string,
	logger *slog.Logger,
) {
	if stm == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Fetch recent errors that have a resolution (successful recovery).
	patterns, err := stm.GetRecentErrors(20)
	if err != nil {
		if logger != nil {
			logger.Warn("[AutoLearning] retro failed to fetch recent errors", "error", err)
		}
		return
	}

	generated := 0
	for _, p := range patterns {
		if p.Resolution == "" || p.OccurrenceCount < 2 {
			continue
		}
		// Skip if a rule already exists for this pattern.
		rules, err := stm.GetLearnedRulesForTools([]string{p.ToolName}, 1)
		if err == nil && len(rules) > 0 {
			continue
		}
		rule := inferRuleFromResolution(p.ToolName, p.ErrorMessage, p.Resolution)
		if rule == "" {
			continue
		}
		lr := &memory.LearnedRule{
			ToolName:   p.ToolName,
			Pattern:    p.ErrorMessage,
			Rule:       rule,
			Confidence: 0.5,
			Hits:       1,
			Active:     true,
		}
		if err := stm.UpsertLearnedRule(lr); err != nil {
			if logger != nil {
				logger.Warn("[AutoLearning] retro failed to upsert rule", "tool", p.ToolName, "error", err)
			}
			continue
		}
		generated++
	}
	if logger != nil {
		logger.Info("[AutoLearning] session retro completed", "session", sessionID, "rules_generated", generated)
	}
}

// recordLearnedRuleOutcome updates hit/miss counters for injected learned rules
// that match the executed tool. It runs best-effort and never blocks the caller.
func recordLearnedRuleOutcome(
	stm *memory.SQLiteMemory,
	injected []memory.LearnedRule,
	toolName string,
	failed bool,
	logger *slog.Logger,
) {
	if stm == nil || len(injected) == 0 || toolName == "" {
		return
	}
	for _, r := range injected {
		if r.ToolName != toolName {
			continue
		}
		if failed {
			if err := stm.RecordLearnedRuleMiss(r.ID); err != nil && logger != nil {
				logger.Debug("[AutoLearning] failed to record rule miss", "rule_id", r.ID, "error", err)
			}
		} else {
			if err := stm.RecordLearnedRuleHit(r.ID); err != nil && logger != nil {
				logger.Debug("[AutoLearning] failed to record rule hit", "rule_id", r.ID, "error", err)
			}
		}
	}
}

// buildLearnedRulesContext formats the top learned rules as a concise string
// suitable for injection into the system prompt. Returns "" when no rules
// are available or the feature is disabled.
func buildLearnedRulesContext(rules []memory.LearnedRule, maxTokens int) string {
	if len(rules) == 0 {
		return ""
	}

	var b strings.Builder
	for _, r := range rules {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("- [%s] %s (confidence: %.0f%%)", r.ToolName, r.Rule, r.Confidence*100))
	}

	s := b.String()
	// Rough token guard: ~4 chars per token. If over budget, truncate.
	if maxTokens > 0 && len(s)/4 > maxTokens {
		lines := strings.Split(s, "\n")
		b.Reset()
		for i, line := range lines {
			if i > 0 {
				b.WriteString("\n")
			}
			if b.Len()/4+len(line)/4 > maxTokens {
				break
			}
			b.WriteString(line)
		}
		s = b.String()
	}
	return s
}
