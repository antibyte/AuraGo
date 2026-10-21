package agent

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"aurago/internal/planner"
)

type historyCompressionFailureState struct {
	count      int
	retryAfter time.Time
}

var historyCompressionFailures = struct {
	sync.Mutex
	states map[string]historyCompressionFailureState
}{states: make(map[string]historyCompressionFailureState)}

func historyCompressionOnCooldown(sessionID string) bool {
	historyCompressionFailures.Lock()
	defer historyCompressionFailures.Unlock()
	return time.Now().Before(historyCompressionFailures.states[strings.TrimSpace(sessionID)].retryAfter)
}

func recordHistoryCompressionFailure(runCfg RunConfig, code string) {
	key := strings.TrimSpace(runCfg.SessionID)
	historyCompressionFailures.Lock()
	state := historyCompressionFailures.states[key]
	state.count++
	state.retryAfter = time.Now().Add(proactiveHistoryCompressionCooldown)
	historyCompressionFailures.states[key] = state
	historyCompressionFailures.Unlock()
	if state.count < 2 {
		return
	}
	recordOperationalIssue(runCfg, planner.OperationalIssue{
		Source: "llm", Context: key, Title: "Conversation summary repeatedly failed",
		Detail:   fmt.Sprintf("Summary failed (code=%s). History remained available and AuraGo used bounded deterministic request trimming.", code),
		Severity: "warning", Kind: planner.OperationalIssueKindRuntimeFailure,
		Reference: key, Fingerprint: "history_compression|" + key, OccurredAt: time.Now(),
	}, runCfg.Logger)
}

func resolveHistoryCompressionFailure(runCfg RunConfig) {
	key := strings.TrimSpace(runCfg.SessionID)
	historyCompressionFailures.Lock()
	delete(historyCompressionFailures.states, key)
	historyCompressionFailures.Unlock()
	if runCfg.PlannerDB != nil {
		_, _ = planner.ResolveOperationalIssue(runCfg.PlannerDB, "history_compression|"+key, "Conversation summarization completed successfully.", time.Now())
	}
}
