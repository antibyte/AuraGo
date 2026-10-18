package agent

import (
	"aurago/internal/config"
	"aurago/internal/memory"
)

// runtimeMemoryWriteAllowed is shared by model-authored memory writers; administrator APIs stay explicit.
func runtimeMemoryWriteAllowed(cfg *config.Config, target, sessionID string) bool {
	if cfg == nil || isCoAgentSession(sessionID) {
		return false
	}
	effective, authorized := dispatchAuthorization(cfg)
	if !authorized || effective == nil {
		return false
	}
	switch target {
	case "memory", "fact", "preference", "core":
		return effective.Tools.Memory.Enabled && !effective.Tools.Memory.ReadOnly
	case "graph", "relationship", "entity":
		return effective.Tools.KnowledgeGraph.Enabled && !effective.Tools.KnowledgeGraph.ReadOnly
	case "journal", "event", "milestone":
		return effective.Tools.Journal.Enabled && !effective.Tools.Journal.ReadOnly
	case "note", "task", "todo":
		return effective.Tools.Notes.Enabled && !effective.Tools.Notes.ReadOnly
	}
	return false
}

func memorySourceSessionActive(stm *memory.SQLiteMemory, sessionID string) bool {
	if stm == nil {
		return false
	}
	deleted, err := stm.IsSessionDeleted(sessionID)
	return err == nil && !deleted
}
