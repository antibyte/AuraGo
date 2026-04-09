package agent

import (
	"fmt"
	"log/slog"
	"strings"
	"unicode"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/tools"
)

// handleRemember routes a "remember" tool call to the appropriate memory subsystem
// based on an optional type hint or automatic classification.
// This reduces cognitive load on the LLM by providing a single entry point for all memory writes.
func handleRemember(tc ToolCall, cfg *config.Config, logger *slog.Logger,
	shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph,
	sessionID string) string {

	content := tc.Content
	if content == "" {
		content = tc.Fact
	}
	if content == "" {
		content = tc.Value
	}
	if content == "" {
		return `Tool Output: {"status":"error","message":"'content' is required — provide the information to remember"}`
	}

	// Determine target from explicit type hint or auto-classify
	target := normalizeRememberCategory(tc.Category)
	if target == "" {
		target = classifyMemoryTarget(tc, content)
	}

	logger.Info("remember tool routing", "target", target, "content_len", len(content))

	switch target {
	case "fact", "preference", "core":
		return rememberAsFact(content, shortTermMem, cfg, logger)
	case "event", "milestone", "journal":
		return rememberAsJournal(content, tc, shortTermMem, sessionID, logger)
	case "task", "todo", "note":
		return rememberAsNoteWithTitle(content, tc.Title, shortTermMem, logger)
	case "relationship", "entity", "graph":
		return rememberAsGraphEdge(content, tc, kg, logger)
	default:
		// Default: store as core memory fact
		return rememberAsFact(content, shortTermMem, cfg, logger)
	}
}

func normalizeRememberCategory(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "fact", "preference", "core", "memory", "core_memory":
		return "fact"
	case "event", "milestone", "journal", "journal_entry", "timeline":
		return "event"
	case "task", "todo", "note", "notes", "reminder":
		return "task"
	case "relationship", "entity", "graph", "knowledge_graph", "edge":
		return "relationship"
	default:
		return ""
	}
}

// classifyMemoryTarget uses structured hints first and conservative heuristics second.
func classifyMemoryTarget(tc ToolCall, content string) string {
	if tc.Source != "" && tc.Target != "" && tc.Relation != "" {
		return "relationship"
	}
	if tc.EntryType != "" || tc.Tags != "" || tc.Importance > 0 {
		return "event"
	}

	normalized := normalizeHeuristicText(content)

	preferencePatterns := []string{
		"prefer ", "preference", "preferred", "usually uses", "normally uses",
		"spricht", "antwortet auf", "likes ", "dislikes ", "language", "timezone",
		"setup preference", "installation preference", "debugging tip", "tip ",
	}
	for _, p := range preferencePatterns {
		if strings.Contains(normalized, p) {
			return "fact"
		}
	}

	// Task/todo patterns
	todoPatterns := []string{"todo", "task:", "aufgabe:", "erledigen", "reminder:", "erinnerung:",
		"check ", "prufe ", "don't forget", "remember to", "nicht vergessen", "muss noch", "need to", "should ",
		"follow up", "investigate", "review ", "fix ", "schedule ", "send ", "call ", "email "}
	for _, p := range todoPatterns {
		if strings.Contains(normalized, p) {
			return "task"
		}
	}

	// Event/milestone patterns
	eventPatterns := []string{"happened", "completed", "finished", "passiert", "erledigt",
		"abgeschlossen", "milestone", "meilenstein", "today ", "gestern ", "yesterday ", "last night",
		"successfully", "erfolgreich", "set up", "eingerichtet", "configured", "konfiguriert",
		"migrated", "migriert", "was installed", "installed on", "resolved", "deployed"}
	for _, p := range eventPatterns {
		if strings.Contains(normalized, p) {
			return "event"
		}
	}

	// Relationship/entity patterns
	relPatterns := []string{" owns ", " uses ", " manages ", " runs on ", " connected to ",
		" gehort ", " nutzt ", " verwaltet ", " lauft auf ", " verbunden mit ",
		" is part of ", " depends on ", " ist teil von "}
	for _, p := range relPatterns {
		if strings.Contains(normalized, p) {
			return "relationship"
		}
	}

	// Default to core memory fact (preferences, identity, environment)
	return "fact"
}

func normalizeHeuristicText(value string) string {
	value = strings.ToLower(value)
	value = strings.Map(func(r rune) rune {
		switch {
		case r > unicode.MaxASCII:
			return foldHeuristicRune(r)
		case unicode.IsLetter(r), unicode.IsDigit(r), unicode.IsSpace(r):
			return r
		default:
			return ' '
		}
	}, value)
	return strings.Join(strings.Fields(value), " ") + " "
}

func foldHeuristicRune(r rune) rune {
	switch r {
	case 'ä', 'á', 'à', 'â', 'ã', 'å':
		return 'a'
	case 'ö', 'ó', 'ò', 'ô', 'õ':
		return 'o'
	case 'ü', 'ú', 'ù', 'û':
		return 'u'
	case 'ß':
		return 's'
	case 'é', 'è', 'ê', 'ë':
		return 'e'
	case 'í', 'ì', 'î', 'ï':
		return 'i'
	case 'ñ':
		return 'n'
	case 'ç':
		return 'c'
	default:
		return ' '
	}
}

func rememberAsFact(content string, stm *memory.SQLiteMemory, cfg *config.Config, logger *slog.Logger) string {
	if stm == nil {
		return `Tool Output: {"status":"error","message":"Memory storage not available"}`
	}
	result, err := tools.ManageCoreMemory("add", content, 0, stm, cfg.Agent.CoreMemoryMaxEntries, cfg.Agent.CoreMemoryCapMode, cfg.Server.UILanguage)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
	}
	return fmt.Sprintf(`Tool Output: {"status":"success","stored_as":"core_memory","message":"Fact stored in core memory","details":%s}`, result)
}

func rememberAsJournal(content string, tc ToolCall, stm *memory.SQLiteMemory, sessionID string, logger *slog.Logger) string {
	if stm == nil {
		return `Tool Output: {"status":"error","message":"Journal storage not available"}`
	}
	entryType := tc.EntryType
	if entryType == "" {
		entryType = "learning"
	}
	importance := tc.Importance
	if importance < 1 || importance > 4 {
		importance = 2
	}
	title := tc.Title
	if title == "" {
		// Use first 80 chars of content as title
		title = content
		if len(title) > 80 {
			title = title[:80] + "..."
		}
	}
	var tags []string
	if tc.Tags != "" {
		for _, t := range strings.Split(tc.Tags, ",") {
			if s := strings.TrimSpace(t); s != "" {
				tags = append(tags, s)
			}
		}
	}
	id, err := stm.InsertJournalEntry(memory.JournalEntry{
		EntryType:     entryType,
		Title:         title,
		Content:       content,
		Tags:          tags,
		Importance:    importance,
		SessionID:     sessionID,
		AutoGenerated: false,
	})
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
	}
	return fmt.Sprintf(`Tool Output: {"status":"success","stored_as":"journal","message":"Event stored in journal","id":%d}`, id)
}

func rememberAsNote(content string, stm *memory.SQLiteMemory, logger *slog.Logger) string {
	if stm == nil {
		return `Tool Output: {"status":"error","message":"Notes storage not available"}`
	}
	return rememberAsNoteWithTitle(content, "", stm, logger)
}

func rememberAsNoteWithTitle(content, explicitTitle string, stm *memory.SQLiteMemory, logger *slog.Logger) string {
	if stm == nil {
		return `Tool Output: {"status":"error","message":"Notes storage not available"}`
	}
	// Use content as title for notes
	title := strings.TrimSpace(explicitTitle)
	if title == "" {
		title = content
	}
	if len(title) > 120 {
		title = title[:120] + "..."
	}
	id, err := stm.AddNote("general", title, "", 2, "")
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
	}
	return fmt.Sprintf(`Tool Output: {"status":"success","stored_as":"note","message":"Task stored as note","id":%d}`, id)
}

func rememberAsGraphEdge(content string, tc ToolCall, kg *memory.KnowledgeGraph, logger *slog.Logger) string {
	if kg == nil {
		return `Tool Output: {"status":"error","message":"Knowledge graph not available"}`
	}
	// If source/target/relation are explicitly provided, use them
	if tc.Source != "" && tc.Target != "" && tc.Relation != "" {
		err := kg.AddEdge(tc.Source, tc.Target, tc.Relation, tc.Properties)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status":"success","stored_as":"knowledge_graph","message":"Relationship stored: %s -[%s]-> %s"}`, tc.Source, tc.Relation, tc.Target)
	}
	// Fallback: store as core memory fact if no structured relationship provided
	return `Tool Output: {"status":"error","message":"For knowledge graph storage, provide 'source', 'target', and 'relation' fields. Or omit 'category' to auto-classify."}`
}
