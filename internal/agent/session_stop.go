package agent

import (
	"aurago/internal/memory"
	"aurago/internal/tools"
)

// StopSession interrupts dispatch and cancels active and queued ingress runs.
func StopSession(store *memory.SQLiteMemory, sessionID string) {
	InterruptSession(sessionID)
	tools.CancelQuestion(sessionID)
	if store != nil {
		store.CancelSessionRuns(sessionID)
	}
}
