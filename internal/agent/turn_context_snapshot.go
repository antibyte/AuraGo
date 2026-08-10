package agent

import (
	"strings"

	"aurago/internal/memory"
	"aurago/internal/prompts"

	openai "github.com/sashabaranov/go-openai"
)

type turnContextSnapshot struct {
	UserIntent string

	MemoryReady             bool
	MemoryDirty             bool
	RetrievedMemories       string
	PredictedMemories       string
	KnowledgeContext        string
	AvailableMemoryIndex    string
	AvailableKnowledgeIndex string
	RecentActivityOverview  string
	MemoryCandidates        map[string]string
	PendingActions          []memory.EpisodicMemory

	NotesReady        bool
	NotesDirty        bool
	HighPriorityNotes string

	PlanReady         bool
	PlanDirty         bool
	SessionPlanPrompt string
}

func (s *turnContextSnapshot) restoreMemory(flags *prompts.ContextFlags) (map[string]string, []memory.EpisodicMemory, bool) {
	if s == nil || flags == nil || !s.MemoryReady || s.MemoryDirty {
		return nil, nil, false
	}
	flags.RetrievedMemories = s.RetrievedMemories
	flags.PredictedMemories = s.PredictedMemories
	flags.KnowledgeContext = s.KnowledgeContext
	flags.AvailableMemoryContextIndex = s.AvailableMemoryIndex
	flags.AvailableKnowledgeContextIndex = s.AvailableKnowledgeIndex
	flags.RecentActivityOverview = s.RecentActivityOverview
	return cloneStringMap(s.MemoryCandidates), append([]memory.EpisodicMemory(nil), s.PendingActions...), true
}

func (s *turnContextSnapshot) captureMemory(flags *prompts.ContextFlags, candidates map[string]string, pending []memory.EpisodicMemory) {
	if s == nil || flags == nil {
		return
	}
	s.RetrievedMemories = flags.RetrievedMemories
	s.PredictedMemories = flags.PredictedMemories
	s.KnowledgeContext = flags.KnowledgeContext
	s.AvailableMemoryIndex = flags.AvailableMemoryContextIndex
	s.AvailableKnowledgeIndex = flags.AvailableKnowledgeContextIndex
	s.RecentActivityOverview = flags.RecentActivityOverview
	s.MemoryCandidates = cloneStringMap(candidates)
	s.PendingActions = append([]memory.EpisodicMemory(nil), pending...)
	s.MemoryReady = true
	s.MemoryDirty = false
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return make(map[string]string)
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func skipToolsForGuideSnapshot(tools []openai.Tool, adaptiveFiltered []string) []string {
	out := make([]string, 0, len(tools)+len(adaptiveFiltered))
	for _, tool := range tools {
		if tool.Function != nil {
			out = append(out, tool.Function.Name)
		}
	}
	return append(out, adaptiveFiltered...)
}

func mergeTurnGuideSnapshot(priority, cached []string, limit int) []string {
	if limit <= 0 {
		limit = 5
	}
	merged := make([]string, 0, min(limit, len(priority)+len(cached)))
	seen := make(map[string]struct{}, len(priority)+len(cached))
	for _, source := range [][]string{priority, cached} {
		for _, guide := range source {
			guide = strings.TrimSpace(guide)
			if guide == "" {
				continue
			}
			if _, exists := seen[guide]; exists {
				continue
			}
			seen[guide] = struct{}{}
			merged = append(merged, guide)
			if len(merged) >= limit {
				return merged
			}
		}
	}
	return merged
}

func invalidateTurnSnapshotAfterTool(s *agentLoopState, call ToolCall, failed bool) {
	if s == nil || s.turnSnapshot == nil || failed || !toolCallMutatesState(call) {
		if s != nil && s.turnSnapshot != nil && !failed && strings.TrimSpace(string(call.Todo)) != "" {
			s.turnSnapshot.PlanDirty = true
		}
		return
	}
	if strings.TrimSpace(string(call.Todo)) != "" {
		s.turnSnapshot.PlanDirty = true
	}
	action := strings.ToLower(strings.TrimSpace(call.Action))
	switch action {
	case "remember":
		category := strings.ToLower(strings.TrimSpace(call.Category))
		if category == "" && call.Params != nil {
			if raw, ok := call.Params["category"].(string); ok {
				category = strings.ToLower(strings.TrimSpace(raw))
			}
		}
		if category == "task" {
			s.turnSnapshot.NotesDirty = true
		} else if category == "" {
			// Auto-classification can select either a memory store or notes. Mark
			// both candidates dirty without re-evaluating unrelated categories.
			s.turnSnapshot.MemoryDirty = true
			s.turnSnapshot.NotesDirty = true
		} else {
			s.turnSnapshot.MemoryDirty = true
		}
	case "manage_memory", "core_memory", "manage_knowledge", "knowledge_graph", "manage_journal", "journal":
		s.turnSnapshot.MemoryDirty = true
	case "manage_notes", "notes", "todo", "manage_todos":
		s.turnSnapshot.NotesDirty = true
	case "manage_plan":
		s.turnSnapshot.PlanDirty = true
	}
}

func toolCallMutatesState(call ToolCall) bool {
	action := strings.ToLower(strings.TrimSpace(call.Action))
	operation := strings.ToLower(strings.TrimSpace(firstNonEmpty(call.Operation, call.ActionType, call.SubOperation)))
	if operation == "" && call.Params != nil {
		for _, key := range []string{"operation", "action_type", "sub_operation"} {
			if raw, ok := call.Params[key].(string); ok && strings.TrimSpace(raw) != "" {
				operation = strings.ToLower(strings.TrimSpace(raw))
				break
			}
		}
	}
	switch action {
	case "remember":
		return true
	case "manage_memory", "core_memory":
		return !oneOf(operation, "", "read", "query", "search", "list", "get", "status")
	case "manage_knowledge", "knowledge_graph":
		return !oneOf(operation, "", "query", "search", "get", "get_node", "get_neighbors", "subgraph", "list", "status")
	case "manage_journal", "journal":
		return !oneOf(operation, "", "list", "search", "get", "get_summary", "status")
	case "manage_notes", "notes", "todo", "manage_todos":
		return !oneOf(operation, "", "list", "get", "search", "status")
	case "manage_plan":
		return !oneOf(operation, "", "list", "get", "status")
	default:
		return false
	}
}

func oneOf(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}
