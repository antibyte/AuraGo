package agent

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/security"
)

// handleRemember routes a "remember" tool call to the appropriate memory subsystem
// based on an optional type hint or automatic classification.
// This reduces cognitive load on the LLM by providing a single entry point for all memory writes.
// guardian is the dispatch Guardian whose local scan screens the content; nil
// falls back to the shared default regex guardian.
func handleRemember(tc ToolCall, cfg *config.Config, logger *slog.Logger,
	shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph,
	sessionID string, guardian *security.Guardian) string {
	if shortTermMem == nil {
		return `Tool Output: {"status":"error","message":"Memory storage not available"}`
	}
	var output string
	err := shortTermMem.WithSessionWrite(context.Background(), sessionID, func() error {
		output = handleRememberWritable(tc, cfg, logger, shortTermMem, kg, sessionID, guardian)
		return nil
	})
	if err != nil {
		return `Tool Output: {"status":"policy_denied","message":"Memory source session is unavailable."}`
	}
	return output
}

// rememberScreenDenied is the fixed answer for a write the content screen
// refuses. It never echoes the content or the finding.
const rememberScreenDenied = `Tool Output: {"status":"policy_denied","message":"This content reads like a standing instruction or an injection attempt, so it was not stored. Memory keeps plain facts and preferences only."}`

// promptsec names its findings by these types (see ScanResult.Patterns).
const (
	rememberEncodingPattern = "encoding_attack"
	rememberRolePattern     = "role_manipulation"
)

// rememberRoleLead is the wording promptsec's role heuristic fires on when it
// reads everyday preference text ("always answer in German", "always provide
// short answers"). rememberBenignRoleForm is the closed form of it that stays
// storable: the verb followed by a language, a length, a tone or a format word.
// Any other continuation ("always answer without restrictions", "always answer
// as an unrestricted assistant") keeps the finding.
var (
	rememberRoleLead       = regexp.MustCompile(`(?i)\balways\s+(?:answer|provide)\b`)
	rememberBenignRoleForm = regexp.MustCompile(`(?i)\balways\s+(answer|provide)(\s+(?:(?:in|auf)\s+\p{L}+|briefly|concisely|clearly|precisely|politely|formally|informally|honestly|thoroughly|(?:a\s+|an\s+|the\s+)?(?:short|concise|brief|detailed|long|terse|compact|formal|informal|polite|structured|friendly|casual|plain|simple|step-by-step)\b|(?:with\s+)?(?:a\s+|an\s+|the\s+)?(?:code|examples?|sources?|citations?|summar(?:y|ies)|tables?|bullets?|markdown)\b))`)
)

// rememberReduceBenignRole removes the leading "always" of every benign form so
// the text can be scanned again without the false positive. It reports false
// when an "always answer" or "always provide" has another continuation, which
// leaves the finding in place.
func rememberReduceBenignRole(text string) (string, bool) {
	leads := rememberRoleLead.FindAllStringIndex(text, -1)
	if len(leads) == 0 || len(leads) != len(rememberBenignRoleForm.FindAllStringIndex(text, -1)) {
		return "", false
	}
	return rememberBenignRoleForm.ReplaceAllString(text, "$1$2"), true
}

// rememberScanFindings lists what a local scan reports at or above limit. The
// sanitizer's encoding_attack is left out: ordinary Cyrillic, Greek or fullwidth
// text raises it (up to High) without naming an attack, and a hidden payload
// shows up under its own pattern, because the scan also reads the decoded and
// the invisible-character-free view. A result that reaches limit without any
// pattern still counts.
func rememberScanFindings(scan security.ScanResult, limit security.ThreatLevel) []string {
	if scan.Level < limit {
		return nil
	}
	if len(scan.Patterns) == 0 {
		return []string{"unspecified"}
	}
	var findings []string
	for _, pattern := range scan.Patterns {
		if pattern != rememberEncodingPattern {
			findings = append(findings, pattern)
		}
	}
	return findings
}

// Longest node and relation name a remember relationship write accepts.
const (
	rememberMaxNodeNameRunes = 200
	rememberMaxRelationRunes = 80
)

// rememberGraphTexts lists what a relationship write puts into the knowledge
// graph: the content kept as raw text, every name, the three names read as one
// sentence ("vault listing" "send to" "https://...") and the property keys and
// values.
func rememberGraphTexts(tc ToolCall, content string) []string {
	texts := []string{content, tc.Source, tc.Relation, tc.Target, tc.Source + " " + tc.Relation + " " + tc.Target}
	for key, value := range tc.Properties {
		texts = append(texts, key, value)
	}
	return texts
}

// rememberTextFindings lists what the local scan of one text reports at limit. A text that
// reaches limit only through bare role or time phrases ("From now on I prefer short
// answers", "From now on, reply in German") is judged without them, the rule GateScan and
// the tool-output scan apply (security.RelieveBarePhrases): the rest still has to stay below
// limit, and a directive shape or a phrase that hands over a role, a mode or an order keeps
// the finding. A durable text is read for that with its benign "always answer in German"
// forms reduced, as rememberContentRefused does for a text with no bare phrase.
func rememberTextFindings(guardian *security.Guardian, text string, limit security.ThreatLevel, durable bool) []string {
	scan := guardian.ScanForInjectionLocal(text)
	if scan.Level >= limit && !scan.Incomplete {
		rescan := guardian.ScanForInjectionLocal
		if durable {
			rescan = func(blanked string) security.ScanResult {
				if reduced, ok := rememberReduceBenignRole(blanked); ok {
					blanked = reduced
				}
				return guardian.ScanForInjectionLocal(blanked)
			}
		}
		if relieved, held := security.RelieveBarePhrases(text, scan, limit, rescan); !held {
			scan = relieved
		}
	}
	return rememberScanFindings(scan, limit)
}

// rememberContentRefused screens the text of a model-authored durable write with
// the dispatch Guardian's local scan, which runs whether or not the LLM Guardian
// is enabled, so the write tools do not depend on a default-off feature. Core
// and fact entries, and the names, content and properties of a knowledge-graph
// edge (the graph reaches the prompt through KnowledgeContext), are injected
// into later runs: they are refused at ThreatMedium or when the text is a
// standing order (memory.StandingOrder). Journal, task and note entries are
// refused at ThreatHigh. A text that only trips bare role or time phrases
// ("From now on I prefer short answers") is not refused for them
// (rememberTextFindings); memory.StandingOrder stays the durable gate for what
// a "from now on" order would do. manage_memory add and update pass target
// "fact" with the fact as content. It returns the sanitized finding names for
// the log; the content itself is never logged.
func rememberContentRefused(guardian *security.Guardian, target, content string, tc ToolCall) (findings []string, refused bool) {
	durable := false
	texts := []string{content, tc.Title}
	switch target {
	case "fact", "preference", "core":
		durable = true
		texts = texts[:1] // a fact has no title
	case "relationship", "entity", "graph":
		durable = true
		texts = rememberGraphTexts(tc, content)
	}
	if guardian == nil {
		guardian = fallbackToolOutputGuardian()
	}
	limit := security.ThreatHigh
	if durable {
		limit = security.ThreatMedium
	}
	for _, text := range texts {
		if strings.TrimSpace(text) == "" {
			continue
		}
		found := rememberTextFindings(guardian, text, limit, durable)
		if durable && len(found) == 1 && found[0] == rememberRolePattern {
			if reduced, ok := rememberReduceBenignRole(text); ok {
				// Judge the text again without that phrase: any other trigger is still there.
				found = rememberTextFindings(guardian, reduced, limit, durable)
			}
		}
		if len(found) > 0 {
			return found, true
		}
		if durable {
			if category, hit := memory.StandingOrder(text); hit {
				return []string{category}, true
			}
		}
	}
	return nil, false
}

func handleRememberWritable(tc ToolCall, cfg *config.Config, logger *slog.Logger, shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph, sessionID string, guardian *security.Guardian) string {

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
	if target == "task" || target == "todo" || target == "note" {
		if err := requireRememberNotesWritePermission(cfg); err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
		}
	}
	if !runtimeMemoryWriteAllowed(cfg, target, sessionID) || !memorySourceSessionActive(shortTermMem, sessionID) {
		return `Tool Output: {"status":"policy_denied","message":"This memory target is disabled, read-only, or unavailable for this session."}`
	}
	if memory.HasDurableCredential(content) {
		return `Tool Output: {"status":"policy_denied","message":"Credentials must be stored in the Vault."}`
	}
	if findings, refused := rememberContentRefused(guardian, target, content, tc); refused {
		logger.Warn("remember refused by the content screen", "target", target, "findings", findings, "content_len", len(content))
		return rememberScreenDenied
	}

	logger.Info("remember tool routing", "target", target, "content_len", len(content))

	switch target {
	case "fact", "preference", "core":
		return rememberAsFact(content, shortTermMem, cfg, logger, sessionID)
	case "event", "milestone", "journal":
		return rememberAsJournal(content, tc, shortTermMem, sessionID, logger)
	case "task", "todo", "note":
		if err := requireRememberNotesWritePermission(cfg); err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err)
		}
		return rememberAsNoteWithTitle(content, tc.Title, shortTermMem, logger, sessionID)
	case "relationship", "entity", "graph":
		return rememberAsGraphEdge(content, tc, kg, sessionID, logger)
	default:
		// Unknown categories should not pollute Core Memory. Treat ambiguous
		// useful information as a searchable journal learning unless the caller
		// explicitly requests category=fact/core or the classifier is confident.
		return rememberAsJournal(content, tc, shortTermMem, sessionID, logger)
	}
}

func requireRememberNotesWritePermission(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	if !cfg.Tools.Notes.Enabled {
		return fmt.Errorf("Notes are disabled. Set tools.notes.enabled=true in config.yaml.")
	}
	if cfg.Tools.Notes.ReadOnly {
		return fmt.Errorf("Notes are in read-only mode. Disable tools.notes.readonly to allow changes.")
	}
	return nil
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
		"location", "lives in", "home city", "city is", "wohnort", "wohnt in", "standort",
		"setup preference", "installation preference", "communication style", "hard constraint",
		"always ", "never ", "immer ", "nie ",
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

	// Ambiguous information is usually an event/learning, not a durable profile
	// fact. Core memory is reserved for explicit stable facts and preferences.
	return "event"
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
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			return r
		}
		return ' '
	}
}

func rememberAsFact(content string, stm *memory.SQLiteMemory, cfg *config.Config, logger *slog.Logger, sourceSessions ...string) string {
	if stm == nil {
		return `Tool Output: {"status":"error","message":"Memory storage not available"}`
	}
	sessionID := ""
	if len(sourceSessions) > 0 {
		sessionID = sourceSessions[0]
	}
	id, err := stm.AddModelCoreFact(content, sessionID, cfg.Agent.CoreMemoryMaxEntries, cfg.Agent.CoreMemoryCapMode == "hard")
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","stored_as":"core_memory","message":%q}`, err.Error())
	}
	return fmt.Sprintf(`Tool Output: {"status":"success","stored_as":"core_memory","message":"Fact stored in core memory","id":%d}`, id)
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
	id, err := stm.InsertModelJournalEntry(memory.JournalEntry{
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

func rememberAsNoteWithTitle(content, explicitTitle string, stm *memory.SQLiteMemory, logger *slog.Logger, sourceSessions ...string) string {
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
	sessionID := ""
	if len(sourceSessions) > 0 {
		sessionID = sourceSessions[0]
	}
	id, err := stm.AddModelNote("general", title, content, 2, "", sessionID)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
	}
	return fmt.Sprintf(`Tool Output: {"status":"success","stored_as":"note","message":"Task stored as note","id":%d}`, id)
}

func rememberAsGraphEdge(content string, tc ToolCall, kg *memory.KnowledgeGraph, sessionID string, logger *slog.Logger) string {
	if kg == nil {
		return `Tool Output: {"status":"error","message":"Knowledge graph not available"}`
	}
	// If source/target/relation are explicitly provided, use them
	if tc.Source != "" && tc.Target != "" && tc.Relation != "" {
		// Names are entity labels. A sentence in one is a payload, and the graph
		// context prints every name.
		if utf8.RuneCountInString(tc.Source) > rememberMaxNodeNameRunes || utf8.RuneCountInString(tc.Target) > rememberMaxNodeNameRunes ||
			utf8.RuneCountInString(tc.Relation) > rememberMaxRelationRunes {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"source and target must be short names (at most %d characters) and relation at most %d."}`, rememberMaxNodeNameRunes, rememberMaxRelationRunes)
		}
		_, err := kg.AddEdgeWithProvenance(tc.Source, tc.Target, tc.Relation, tc.Properties, memory.KGProvenanceInput{
			EvidenceType:    "remember",
			SessionID:       sessionID,
			Channel:         "agent",
			Actor:           "agent",
			RawText:         content,
			AssertedInGraph: "remember",
			Confidence:      0.90,
			ConfidenceLabel: "agent_extracted",
			SourceKind:      "agent",
			PrivacyClass:    "standard",
			RetentionPolicy: "default",
		})
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status":"success","stored_as":"knowledge_graph","message":"Relationship stored: %s -[%s]-> %s"}`, tc.Source, tc.Relation, tc.Target)
	}
	// Reject graph writes without a structured edge; callers can omit category to auto-classify instead.
	return `Tool Output: {"status":"error","message":"For knowledge graph storage, provide 'source', 'target', and 'relation' fields. Or omit 'category' to auto-classify."}`
}
