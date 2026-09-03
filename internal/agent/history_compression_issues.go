package agent

import (
	"strings"
	"sync"
	"time"

	"aurago/internal/planner"
)

var historyCompressionFailures = struct {
	sync.Mutex
	counts map[string]int
}{counts: make(map[string]int)}

func recordHistoryCompressionFailure(runCfg RunConfig) {
	if runCfg.PlannerDB == nil {
		return
	}
	key := strings.TrimSpace(runCfg.SessionID)
	historyCompressionFailures.Lock()
	historyCompressionFailures.counts[key]++
	count := historyCompressionFailures.counts[key]
	historyCompressionFailures.Unlock()
	if count < 2 {
		return
	}
	recordOperationalIssue(runCfg, planner.OperationalIssue{
		Source: "llm", Context: key, Title: "Conversation summary repeatedly failed",
		Detail:   "History remained available and AuraGo used bounded deterministic request trimming.",
		Severity: "warning", Kind: planner.OperationalIssueKindRuntimeFailure,
		Reference: key, Fingerprint: "history_compression|" + key, OccurredAt: time.Now(),
	}, runCfg.Logger)
}

func resolveHistoryCompressionFailure(runCfg RunConfig) {
	key := strings.TrimSpace(runCfg.SessionID)
	historyCompressionFailures.Lock()
	delete(historyCompressionFailures.counts, key)
	historyCompressionFailures.Unlock()
	if runCfg.PlannerDB != nil {
		_, _ = planner.ResolveOperationalIssue(runCfg.PlannerDB, "history_compression|"+key, "Conversation summarization completed successfully.", time.Now())
	}
}
