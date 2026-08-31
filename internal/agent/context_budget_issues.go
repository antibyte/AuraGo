package agent

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"aurago/internal/planner"
)

var contextBudgetFailures = struct {
	sync.Mutex
	counts map[string]int
}{counts: make(map[string]int)}

func contextBudgetRouteKey(runCfg RunConfig, model string) string {
	providerID, providerType := "", ""
	if runCfg.Config != nil {
		providerID = runCfg.Config.LLM.Provider
		providerType = runCfg.Config.LLM.ProviderType
		if strings.TrimSpace(model) == "" {
			model = runCfg.Config.LLM.Model
		}
	}
	return strings.ToLower(strings.TrimSpace(providerID)) + "|" +
		strings.ToLower(strings.TrimSpace(providerType)) + "|" +
		strings.ToLower(strings.TrimSpace(model))
}

func contextBudgetFingerprint(key string) string {
	return "context_budget|" + key
}

func recordContextBudgetFailure(runCfg RunConfig, model string, err error) {
	if runCfg.PlannerDB == nil || !IsContextBudgetExceeded(err) {
		return
	}
	key := contextBudgetRouteKey(runCfg, model)
	contextBudgetFailures.Lock()
	contextBudgetFailures.counts[key]++
	count := contextBudgetFailures.counts[key]
	contextBudgetFailures.Unlock()
	if count < 2 {
		return
	}
	var exceeded *ContextBudgetExceededError
	_ = errors.As(err, &exceeded)
	detail := "The same provider/model route repeatedly exceeded its effective request budget."
	if exceeded != nil {
		detail = fmt.Sprintf("required=%d reserve=%d safety=%d context=%d",
			exceeded.RequiredInput, exceeded.CompletionReserve, exceeded.SafetyMargin, exceeded.ContextWindow)
	}
	_, recordErr := planner.RecordOperationalIssue(runCfg.PlannerDB, planner.OperationalIssue{
		Source: "llm", Context: key, Title: "LLM context budget repeatedly exceeded",
		Detail: detail, Severity: "warning", Kind: planner.OperationalIssueKindRuntimeFailure,
		Reference: strings.TrimSpace(model), Fingerprint: contextBudgetFingerprint(key), OccurredAt: time.Now(),
	})
	if recordErr != nil && runCfg.Logger != nil {
		runCfg.Logger.Warn("Failed to record context budget issue", "error_code", "operational_issue_persist")
	}
}

func resolveContextBudgetFailure(runCfg RunConfig, model string) {
	if runCfg.PlannerDB == nil {
		return
	}
	key := contextBudgetRouteKey(runCfg, model)
	contextBudgetFailures.Lock()
	delete(contextBudgetFailures.counts, key)
	contextBudgetFailures.Unlock()
	_, _ = planner.ResolveOperationalIssue(runCfg.PlannerDB, contextBudgetFingerprint(key), "The same provider/model route completed an LLM request successfully.", time.Now())
}
