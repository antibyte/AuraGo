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

	PlannerReady      bool
	PlannerDirty      bool
	PlannerContext    string
	DailyTodoReminder string
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

func (s *turnContextSnapshot) restorePlanner() (string, string, bool) {
	if s == nil || !s.PlannerReady || s.PlannerDirty {
		return "", "", false
	}
	return s.PlannerContext, s.DailyTodoReminder, true
}

func (s *turnContextSnapshot) capturePlanner(contextText, reminder string) {
	if s == nil {
		return
	}
	s.PlannerContext = contextText
	s.DailyTodoReminder = reminder
	s.PlannerReady = true
	s.PlannerDirty = false
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
	if s == nil || s.turnSnapshot == nil || failed {
		return
	}
	if strings.TrimSpace(string(call.Todo)) != "" {
		s.turnSnapshot.PlanDirty = true
	}
	categories := turnSnapshotMutationCategories(call)
	if categories&snapshotMemory != 0 {
		s.turnSnapshot.MemoryDirty = true
	}
	if categories&snapshotNotes != 0 {
		s.turnSnapshot.NotesDirty = true
	}
	if categories&snapshotSessionPlan != 0 {
		s.turnSnapshot.PlanDirty = true
	}
	if categories&snapshotPlanner != 0 {
		s.turnSnapshot.PlannerDirty = true
	}
}

func toolCallMutatesState(call ToolCall) bool {
	return turnSnapshotMutationCategories(call) != 0
}

type turnSnapshotCategory uint8

const (
	snapshotMemory turnSnapshotCategory = 1 << iota
	snapshotNotes
	snapshotSessionPlan
	snapshotPlanner
)

func turnSnapshotMutationCategories(call ToolCall) turnSnapshotCategory {
	action := strings.ToLower(strings.TrimSpace(call.Action))
	operation := toolCallSnapshotOperation(call)
	switch action {
	case "remember":
		category := strings.ToLower(strings.TrimSpace(call.Category))
		if category == "" && call.Params != nil {
			if raw, ok := call.Params["category"].(string); ok {
				category = strings.ToLower(strings.TrimSpace(raw))
			}
		}
		switch category {
		case "task":
			return snapshotNotes
		case "":
			return snapshotMemory | snapshotNotes
		default:
			return snapshotMemory
		}
	case "manage_memory", "core_memory":
		if oneOf(operation, "", "read", "query", "search", "list", "get", "status", "view_profile") {
			return 0
		}
		return snapshotMemory
	case "manage_knowledge", "knowledge_graph":
		if oneOf(operation, "", "query", "search", "get", "get_node", "get_neighbors", "subgraph", "list", "status",
			"graph_health", "explore", "suggest_relations", "suggest_inferred_relations", "export_jsonld", "explain_edge", "list_conflicts") {
			return 0
		}
		return snapshotMemory
	case "manage_journal", "journal":
		if oneOf(operation, "", "list", "search", "get", "get_summary", "status") {
			return 0
		}
		return snapshotMemory
	case "archive_memory", "optimize_memory":
		return snapshotMemory
	case "memory_reflect":
		return snapshotMemory | snapshotPlanner
	case "manage_notes", "notes", "todo":
		if oneOf(operation, "", "list", "get", "search", "status") {
			return 0
		}
		return snapshotNotes
	case "manage_todos", "manage_appointments":
		if oneOf(operation, "", "list", "get", "search", "status") {
			return 0
		}
		// Planner writes synchronize their corresponding KG nodes.
		return snapshotPlanner | snapshotMemory
	case "manage_plan":
		if oneOf(operation, "", "list", "get", "status") {
			return 0
		}
		return snapshotSessionPlan
	default:
		return 0
	}
}

func toolCallSnapshotOperation(call ToolCall) string {
	operation := strings.ToLower(strings.TrimSpace(firstNonEmpty(call.Operation, call.ActionType, call.SubOperation)))
	if operation == "" && call.Params != nil {
		for _, key := range []string{"operation", "action_type", "sub_operation"} {
			if raw, ok := call.Params[key].(string); ok && strings.TrimSpace(raw) != "" {
				operation = strings.ToLower(strings.TrimSpace(raw))
				break
			}
		}
	}
	return operation
}

type turnGuidePreparationState uint8

const (
	turnGuidesNotEligible turnGuidePreparationState = iota
	turnGuidesResolvedWithoutSearch
	turnGuidesSearchEligible
)

func classifyTurnGuidePreparation(suppress bool, tier string, explicitTools []string) turnGuidePreparationState {
	if suppress {
		return turnGuidesResolvedWithoutSearch
	}
	if strings.EqualFold(strings.TrimSpace(tier), "full") || len(explicitTools) > 0 {
		return turnGuidesSearchEligible
	}
	return turnGuidesNotEligible
}

func oneOf(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}
