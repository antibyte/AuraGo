package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

// memoryAnalysisResult represents extracted memory-worthy content from a conversation turn.
type memoryAnalysisResult struct {
	Facts          []extractedFact `json:"facts,omitempty"`
	Preferences    []extractedFact `json:"preferences,omitempty"`
	Corrections    []extractedFact `json:"corrections,omitempty"`
	PendingActions []pendingAction `json:"pending_actions,omitempty"`
}

type extractedFact struct {
	Content    string  `json:"content"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
}

type pendingAction struct {
	Title      string  `json:"title"`
	Summary    string  `json:"summary"`
	Trigger    string  `json:"trigger_query"`
	Confidence float64 `json:"confidence"`
}

type memoryAnalysisLLMConfig struct {
	providerType string
	baseURL      string
	apiKey       string
	model        string
}

const memoryAnalysisPrompt = `You are a memory extraction assistant. Analyze the following conversation exchange and extract any information worth remembering.

Extract:
1. **Facts**: Concrete facts about the user, their environment, preferences, or projects (e.g., "User runs Proxmox on a Dell R730", "User's name is Alex")
2. **Preferences**: User preferences, habits, or workflows (e.g., "User prefers Go over Python", "User likes minimal logging")
3. **Corrections**: Corrections to previously known information (e.g., "User moved from Berlin to Munich", "User switched from Docker to Podman")
4. **Pending Actions**: Explicit open follow-ups that should be surfaced proactively later (e.g., "Help user with Nextcloud Docker setup", "Follow up on SSL renewal before Friday")

For each extracted item, provide:
- content: The factual statement to remember
- category: A short category label (e.g., "infrastructure", "personal", "workflow", "preference", "recent_operational_details")
- confidence: How confident you are this is worth storing (0.0 to 1.0)

Rules:
- Only extract genuinely useful, long-term information. Skip transient requests like "show me the logs".
- Do NOT extract information that is just part of the current task context.
- Do NOT extract emotions, moods, or temporary states.
- Use category "recent_operational_details" for details likely needed in the next days (paths, versions, hostnames, ports, identifiers, deadlines).
- NEVER extract any claim about whether a tool, integration, capability, or feature is currently available, enabled, configured, active, or missing. This is transient system state that changes with configuration and must never be stored in memory.
- Only create pending actions when the exchange clearly indicates deferred future work, a follow-up promise, or an unfinished task likely relevant in the next days/weeks.
- For pending actions provide: title, summary, trigger_query, confidence.
- If there is nothing worth remembering, return empty arrays.

Respond ONLY with valid JSON in this exact format:
{"facts":[],"preferences":[],"corrections":[],"pending_actions":[]}

User message:
%s

Assistant response:
%s`

// stripToolCallBlocks removes [TOOL_CALL]...[/TOOL_CALL] blocks and JSON tool call
// objects from a string so small LLMs don't confuse them with actionable instructions.
func stripToolCallBlocks(s string) string {
	lower := strings.ToLower(s)
	for {
		start := strings.Index(lower, "[tool_call]")
		if start == -1 {
			break
		}
		end := strings.Index(lower[start:], "[/tool_call]")
		if end == -1 {
			// no closing tag — strip from start to end of string
			s = strings.TrimSpace(s[:start])
			break
		}
		s = s[:start] + s[start+end+12:]
		lower = strings.ToLower(s)
	}
	return strings.TrimSpace(s)
}

func trimJSONResponse(raw string) string {
	raw = strings.TrimSpace(raw)

	// Strip <think>...</think> blocks emitted by reasoning models (e.g. MiniMax-M2.7, DeepSeek-R1).
	// Iterate in case the model emits multiple thinking blocks.
	for {
		lower := strings.ToLower(raw)
		start := strings.Index(lower, "<think>")
		if start == -1 {
			break
		}
		end := strings.Index(lower[start:], "</think>")
		if end == -1 {
			// Unclosed <think> tag — the response was truncated inside the thinking block.
			// Keep the content inside the partial <think> block: the LLM may have written
			// the JSON there before the stream was cut off. The "advance to first {/["
			// step below will then locate any usable JSON fragment.
			raw = strings.TrimSpace(raw[start+len("<think>"):])
			break
		}
		raw = strings.TrimSpace(raw[:start] + raw[start+end+len("</think>"):])
	}

	// Strip markdown code fences (```json ... ``` or ``` ... ```).
	if strings.HasPrefix(raw, "```") {
		if idx := strings.Index(raw[3:], "\n"); idx >= 0 {
			raw = raw[3+idx+1:]
		}
		if strings.HasSuffix(raw, "```") {
			raw = strings.TrimSuffix(raw, "```")
		}
		raw = strings.TrimSpace(raw)
	}

	// If the response still has leading non-JSON text, advance to the first { or [.
	for i, ch := range raw {
		if ch == '{' || ch == '[' {
			raw = raw[i:]
			break
		}
	}

	// Strip trailing non-JSON characters (e.g. backticks, stray chars after the closing bracket).
	// This handles malformed responses from reasoning models that append stray characters.
	if len(raw) > 0 {
		// Find the last valid JSON terminator: } or ]
		lastValid := len(raw) - 1
		for lastValid >= 0 && raw[lastValid] != '}' && raw[lastValid] != ']' {
			lastValid--
		}
		if lastValid >= 0 {
			raw = raw[:lastValid+1]
		}
	}

	return strings.TrimSpace(raw)
}

func parseMemoryAnalysisResult(raw string) (memoryAnalysisResult, error) {
	raw = trimJSONResponse(raw)
	var result memoryAnalysisResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return memoryAnalysisResult{}, fmt.Errorf("parse memory analysis response: %w", err)
	}
	return result, nil
}

// runMemoryAnalysis performs async post-response memory extraction using the configured analysis provider.
func runMemoryAnalysis(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
	stm *memory.SQLiteMemory,
	kg *memory.KnowledgeGraph,
	ltm memory.VectorDB,
	userMsg string,
	assistantResp string,
	sessionID string,
) {
	settings := resolveMemoryAnalysisSettings(cfg, stm)
	if !settings.Enabled || !settings.RealTime {
		return
	}
	if userMsg == "" || len(userMsg) < 10 {
		return // too short to analyze
	}
	if isAmbiguousShortCommand(userMsg) {
		return
	}

	llmCfg := resolveMemoryAnalysisLLMConfig(cfg)
	if llmCfg.model == "" {
		logger.Debug("[Memory Analysis] No resolved LLM config available")
		return
	}

	analysisClient := llm.NewClientFromProvider(
		llmCfg.providerType,
		llmCfg.baseURL,
		llmCfg.apiKey,
	)

	// Truncate for analysis (no need to send huge responses)
	truncUser := userMsg
	if len(truncUser) > 2000 {
		truncUser = truncUser[:2000] + "..."
	}
	// Strip tool call blocks before sending to the memory analysis LLM — they confuse
	// small models into outputting tool calls instead of the expected facts JSON.
	truncResp := stripToolCallBlocks(assistantResp)
	if len(truncResp) > 2000 {
		truncResp = truncResp[:2000] + "..."
	}

	prompt := fmt.Sprintf(memoryAnalysisPrompt, truncUser, truncResp)

	req := openai.ChatCompletionRequest{
		Model: llmCfg.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.1,
		MaxTokens:   800,
	}

	resp, err := analysisClient.CreateChatCompletion(ctx, req)
	if err != nil {
		logger.Warn("[Memory Analysis] LLM call failed", "error", err)
		return
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return
	}

	result, err := parseMemoryAnalysisResult(resp.Choices[0].Message.Content)
	if err != nil {
		raw := trimJSONResponse(resp.Choices[0].Message.Content)
		logger.Warn("[Memory Analysis] Failed to parse response", "error", err, "raw", Truncate(raw, 200))
		return
	}

	applyMemoryAnalysisResult(cfg, logger, stm, ltm, sessionID, result)
}

func applyMemoryAnalysisResult(cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, sessionID string, result memoryAnalysisResult) int {
	threshold := 0.0
	if cfg != nil {
		threshold = cfg.MemoryAnalysis.AutoConfirm
	}
	stored := 0
	// Process facts
	for _, f := range result.Facts {
		minThreshold := thresholdForMemoryCategory(threshold, f.Category)
		if f.Confidence >= minThreshold && f.Content != "" && shouldStoreExtractedMemory(f.Content, f.Category) {
			if ltm != nil {
				concept := fmt.Sprintf("[%s] %s", f.Category, f.Content)
				if ids, err := ltm.StoreDocument(concept, "source:memory_analysis session:"+sessionID); err != nil {
					logger.Warn("[Memory Analysis] Failed to store fact in LTM", "error", err)
				} else {
					if stm != nil {
						for _, id := range ids {
							_ = stm.UpsertMemoryMetaWithDetails(id, memory.MemoryMetaUpdate{
								ExtractionConfidence: f.Confidence,
								VerificationStatus:   "unverified",
								SourceType:           "memory_analysis",
								SourceReliability:    0.85,
							})
						}
					}
					detectMemoryConflictsForDocIDs(logger, stm, ltm, ids, concept)
					stored++
					if stm != nil && strings.EqualFold(f.Category, "recent_operational_details") {
						_ = stm.InsertEpisodicMemoryWithDetails(
							time.Now().Format("2006-01-02"),
							"Operational detail",
							Truncate(f.Content, 220),
							map[string]string{"category": f.Category, "session_id": sessionID},
							3,
							"memory_analysis",
							memory.EpisodicMemoryDetails{
								SessionID:        sessionID,
								Participants:     []string{"user", "agent"},
								RelatedDocIDs:    ids,
								EmotionalValence: 0.10,
							},
						)
					}
				}
			}
		}
	}

	// Process preferences
	for _, p := range result.Preferences {
		minThreshold := thresholdForMemoryCategory(threshold, p.Category)
		if p.Confidence >= minThreshold && p.Content != "" && shouldStoreExtractedMemory(p.Content, p.Category) {
			if ltm != nil {
				concept := fmt.Sprintf("[preference:%s] %s", p.Category, p.Content)
				if ids, err := ltm.StoreDocument(concept, "source:memory_analysis session:"+sessionID); err != nil {
					logger.Warn("[Memory Analysis] Failed to store preference in LTM", "error", err)
				} else {
					if stm != nil {
						for _, id := range ids {
							_ = stm.UpsertMemoryMetaWithDetails(id, memory.MemoryMetaUpdate{
								ExtractionConfidence: p.Confidence,
								VerificationStatus:   "unverified",
								SourceType:           "memory_analysis",
								SourceReliability:    0.85,
							})
						}
					}
					detectMemoryConflictsForDocIDs(logger, stm, ltm, ids, concept)
					stored++
				}
			}
		}
	}

	// Process corrections — these update core memory
	for _, c := range result.Corrections {
		minThreshold := thresholdForMemoryCategory(threshold, c.Category)
		if c.Confidence >= minThreshold && c.Content != "" && shouldStoreExtractedMemory(c.Content, c.Category) {
			if ltm != nil {
				concept := fmt.Sprintf("[correction:%s] %s", c.Category, c.Content)
				if ids, err := ltm.StoreDocument(concept, "source:memory_analysis session:"+sessionID); err != nil {
					logger.Warn("[Memory Analysis] Failed to store correction in LTM", "error", err)
				} else {
					if stm != nil {
						for _, id := range ids {
							_ = stm.UpsertMemoryMetaWithDetails(id, memory.MemoryMetaUpdate{
								ExtractionConfidence: c.Confidence,
								VerificationStatus:   "unverified",
								SourceType:           "memory_analysis",
								SourceReliability:    0.90,
							})
							detectMemoryConflictsForDocIDs(logger, stm, ltm, ids, concept)
						}
					}
					stored++
				}
			}
		}
	}

	for _, action := range result.PendingActions {
		if stm == nil || action.Confidence < 0.65 {
			continue
		}
		title := strings.TrimSpace(action.Title)
		summary := strings.TrimSpace(action.Summary)
		if title == "" || summary == "" {
			continue
		}
		trigger := strings.TrimSpace(action.Trigger)
		if trigger == "" {
			trigger = title
		}
		if err := stm.UpsertPendingEpisodicAction(time.Now().Format("2006-01-02"), title, summary, trigger, sessionID, 3, nil); err != nil && logger != nil {
			logger.Warn("[Memory Analysis] Failed to store pending action", "title", title, "error", err)
		}
	}

	if stored > 0 {
		if logger != nil {
			logger.Info("[Memory Analysis] Stored extracted memories",
				"facts", len(result.Facts),
				"preferences", len(result.Preferences),
				"corrections", len(result.Corrections),
				"stored", stored,
				"session", sessionID,
			)
		}
	}
	return stored
}

func thresholdForMemoryCategory(defaultThreshold float64, category string) float64 {
	if strings.EqualFold(strings.TrimSpace(category), "recent_operational_details") {
		if defaultThreshold > 0.82 {
			return 0.82
		}
	}
	return defaultThreshold
}

func shouldUseRAGForMessage(msg string) bool {
	trimmed := strings.TrimSpace(msg)
	if len(trimmed) < 20 {
		return false
	}
	if isAmbiguousShortCommand(trimmed) {
		return false
	}
	// Capability and availability queries must be answered from the live tool schema
	// in the current context, never from potentially stale memory entries.
	if isCapabilityQuery(trimmed) {
		return false
	}
	return true
}

// isCapabilityQuery returns true when the message is primarily asking about what
// the agent can do, or whether a specific tool/integration is available or enabled.
// For these queries, RAG retrieval is skipped entirely: the source of truth is the
// live tool schema injected into the context, not historical memory.
func isCapabilityQuery(msg string) bool {
	lower := strings.ToLower(strings.TrimSpace(msg))

	// Explicit capability-question patterns across all 15 supported languages.
	capPatterns := []string{
		// German
		"hast du", "kannst du", "habe ich zugang", "gibt es ein tool", "gibt es eine",
		"steht dir", "steht zur verfügung", "ist verfügbar", "ist aktiviert",
		"ist eingeschaltet", "welche tools", "welche werkzeuge", "welche fähigkeiten",
		"was kannst du", "was für tools", "kannst du mit",
		// English
		"do you have", "can you", "are you able", "is it available", "is the tool",
		"which tools", "what tools", "what can you", "what capabilities", "do you support",
		"tool available", "tool enabled", "integration available", "integration enabled",
		"have you got", "do you have access",
		// Spanish
		"tienes", "puedes", "está disponible", "qué herramientas", "cuáles herramientas",
		"tienes acceso",
		// French
		"avez-vous", "pouvez-vous", "est disponible", "quels outils", "outil disponible",
		// Italian
		"hai", "puoi", "è disponibile", "quali strumenti",
		// Portuguese
		"você tem", "pode", "está disponível", "quais ferramentas",
		// Dutch
		"heb je", "kun je", "is beschikbaar", "welke tools",
		// Swedish/Norwegian/Danish
		"har du", "kan du", "är tillgänglig", "er tilgjengelig",
		// Polish
		"czy masz", "czy możesz",
		// Czech
		"máš", "můžeš",
	}
	for _, p := range capPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	// Combo check: message contains BOTH a capability subject (tool name or generic term)
	// AND an availability keyword. This catches "ist chromecast konfiguriert?" or
	// "chromecast disponibile?" even without an explicit pattern above.
	availabilityWords := []string{
		"available", "enabled", "configured", "supported", "active", "installed",
		"verfügbar", "aktiviert", "konfiguriert", "unterstützt", "vorhanden",
		"disponible", "activé", "configuré",
		"disponível", "ativado", "configurado",
		"disponibile", "attivo", "configurato",
		"beschikbaar", "actief",
		"tillgänglig", "tilgjengelig",
	}
	capabilitySubjects := []string{
		"tool", "tools", "integration", "capability", "capabilities", "function", "feature",
		"werkzeug", "werkzeuge", "fähigkeit", "funktion",
		"chromecast", "docker", "proxmox", "telegram", "discord", "mqtt",
		"home assistant", "homeassistant", "tailscale", "truenas", "frigate",
		"google cast", "cast", "tts", "stt", "speech",
	}
	hasSubject := false
	for _, s := range capabilitySubjects {
		if strings.Contains(lower, s) {
			hasSubject = true
			break
		}
	}
	if !hasSubject {
		return false
	}
	for _, a := range availabilityWords {
		if strings.Contains(lower, a) {
			return true
		}
	}
	return false
}

// transientMemoryPhrases lists phrases that indicate transient tool/integration
// availability state. These must never be stored in or served from long-term
// memory because they reflect a point-in-time configuration state that may
// change (e.g. a tool was disabled, then re-enabled).
var transientMemoryPhrases = []string{
	"integration is not available",
	"integration is unavailable",
	"integration is not configured",
	"tool is not available",
	"tool is unavailable",
	"tool does not appear in the tool list",
	"does not appear in the tool list",
	"not in the tool list",
	"is not enabled",
	"is disabled",
	"api key is missing",
	"steht dir nicht zur verfügung",
	"steht nicht zur verfügung",
	"ist nicht verfügbar",
	"ist nicht konfiguriert",
	"nicht in der werkzeugliste auftaucht",
	"taucht nicht in der werkzeugliste auf",
	"taucht nicht in der toolliste auf",
	"api-schlüssel fehlt",
}

// containsTransientMemoryPhrase returns true if text contains any phrase that
// indicates a transient tool/integration availability state.
func containsTransientMemoryPhrase(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, phrase := range transientMemoryPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// shouldServeRAGMemory returns false for memories that must not be served to the agent
// because they contain stale, transient system-state claims:
//  1. Entries containing transient phrases ("tool is not available", etc.) — language-specific filter
//     for entries written before the category-based write filter existed.
//  2. Entries categorized as [tool_availability] — language-agnostic, covers all future entries
//     regardless of the language the LLM used when creating them.
func shouldServeRAGMemory(text string) bool {
	if containsTransientMemoryPhrase(text) {
		return false
	}
	// Drop entries explicitly categorised as tool_availability. The concept string
	// is stored as the first line of the document content, so checking the prefix
	// covers both single-document and chunked storage.
	lower := strings.ToLower(strings.TrimSpace(text))
	if strings.HasPrefix(lower, "[tool_availability]") {
		return false
	}
	return true
}

func shouldStoreExtractedMemory(content, category string) bool {
	text := strings.ToLower(strings.TrimSpace(content))
	if text == "" {
		return false
	}

	// Block tool/integration availability claims regardless of language —
	// the LLM assigns this category when the memoryAnalysisPrompt rule is followed.
	if strings.EqualFold(strings.TrimSpace(category), "tool_availability") {
		return false
	}

	if containsTransientMemoryPhrase(content) {
		return false
	}

	if isEphemeralExecutionClaim(text) || isWeakOperationalLabel(text) || isTransientMediaAnalysisClaim(text) {
		return false
	}

	if strings.EqualFold(strings.TrimSpace(category), "recent_operational_details") {
		if strings.Contains(text, "bug report") || strings.Contains(text, "fehlerbericht") ||
			strings.Contains(text, "document sent") || strings.Contains(text, "dokument gesendet") {
			return false
		}
		if strings.Contains(text, "agent_workspace/workdir/attachments/") ||
			strings.Contains(text, "vision api is down") ||
			strings.Contains(text, "authentication issues") ||
			(strings.Contains(text, "uploaded") && (strings.Contains(text, "mb") || strings.Contains(text, "gb"))) {
			return false
		}
		if strings.Contains(text, "integration") || strings.Contains(text, "tool") {
			if strings.Contains(text, "available") || strings.Contains(text, "verfügbar") ||
				strings.Contains(text, "configured") || strings.Contains(text, "konfiguriert") ||
				strings.Contains(text, "enabled") || strings.Contains(text, "aktiviert") {
				return false
			}
		}
	}
	if strings.EqualFold(strings.TrimSpace(category), "user_preferences") {
		if strings.Contains(text, "direct document sending") || strings.Contains(text, "dokument direkt senden") ||
			strings.Contains(text, "bug report") || strings.Contains(text, "fehlerbericht") {
			return false
		}
	}

	return true
}

func isWeakOperationalLabel(text string) bool {
	prefixes := []string{
		"song:", "titel:", "title:", "target device:", "ziel:", "device:",
		"tool:", "problem:", "attempts:", "versuche:", "tries:", "chromecast tool:",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func isTransientMediaAnalysisClaim(text string) bool {
	prefixes := []string{
		"image analysis result:",
		"vision analysis result:",
		"image description:",
		"bildanalyse:",
		"bildanalyse ergebnis:",
		"analyseergebnis bild:",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func isEphemeralExecutionClaim(text string) bool {
	playbackContext := []string{
		"song", "audio", "music", "musik", "google home", "google home mini",
		"chromecast", "tts", "speaker", "lautsprecher",
	}
	playbackVerbs := []string{
		"is playing", "playing on", "now playing", "wird abgespielt", "wird jetzt abgespielt",
		"läuft auf", "läuft jetzt", "abgespielt", "gespielt", "playing", "speaking on",
		"hat gesprochen", "spricht gerade",
	}
	hasContext := false
	for _, needle := range playbackContext {
		if strings.Contains(text, needle) {
			hasContext = true
			break
		}
	}
	if !hasContext {
		return false
	}
	for _, needle := range playbackVerbs {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

// isAmbiguousShortCommand returns true if the message is a short, context-dependent
// follow-up command that references previous actions without specifying a topic.
// These messages cause poor RAG results because they match unrelated old memories.
func isAmbiguousShortCommand(msg string) bool {
	if len(msg) > 60 {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(msg))

	// Direct match for very common short follow-up commands
	ambiguousExact := []string{
		"ja", "nein", "ok", "yes", "no", "sure", "klar",
		"weiter", "continue", "go", "go ahead", "mach weiter",
		"danke", "thanks", "thank you", "thx",
	}
	for _, a := range ambiguousExact {
		if lower == a {
			return true
		}
	}

	// Pattern match for retry/repeat commands that lack topic specificity
	ambiguousPatterns := []string{
		"versuche es erneut", "versuch es nochmal", "versuch es noch mal",
		"try again", "retry", "nochmal", "noch mal", "noch einmal",
		"wiederholen", "repeat", "do it again", "mach das nochmal",
		"mach das noch mal", "erneut versuchen", "nochmals",
		"das gleiche nochmal", "the same again",
		"teste erneut", "teste nochmal", "teste noch mal",
		"nochmal testen", "noch mal testen", "test again",
	}
	for _, pattern := range ambiguousPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// expandQueryForRAG uses the MemoryAnalysis LLM to generate optimized search keywords
// from the user's message for better RAG retrieval. Returns the expanded query string
// or the original message on failure/timeout.
func expandQueryForRAG(ctx context.Context, cfg *config.Config, logger *slog.Logger, userMsg string, stm *memory.SQLiteMemory) string {
	settings := resolveMemoryAnalysisSettings(cfg, stm)
	if !settings.Enabled || !settings.QueryExpansion || len(userMsg) <= 20 {
		return userMsg
	}

	llmCfg := resolveMemoryAnalysisLLMConfig(cfg)
	if llmCfg.model == "" {
		return userMsg
	}

	expandCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()

	client := llm.NewClientFromProvider(
		llmCfg.providerType,
		llmCfg.baseURL,
		llmCfg.apiKey,
	)

	model := llmCfg.model

	truncMsg := userMsg
	if len(truncMsg) > 500 {
		truncMsg = truncMsg[:500]
	}

	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Extract 2-3 concise search keywords from this message. Output ONLY the keywords separated by spaces, nothing else.\n\nMessage: " + truncMsg,
			},
		},
		Temperature: 0.0,
		MaxTokens:   50,
	}

	resp, err := client.CreateChatCompletion(expandCtx, req)
	if err != nil {
		logger.Debug("[RAG Query Expansion] LLM call failed, using original query", "error", err)
		return userMsg
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return userMsg
	}

	expanded := strings.TrimSpace(resp.Choices[0].Message.Content)
	if expanded == "" || len(expanded) > 200 {
		return userMsg
	}

	// Combine original query with expanded keywords for better embedding coverage
	combined := userMsg + " " + expanded
	logger.Debug("[RAG Query Expansion] Expanded query", "original_len", len(userMsg), "keywords", expanded)
	return combined
}

// rerankWithLLM uses the MemoryAnalysis LLM to score the relevance of RAG candidates
// against the user query. Returns re-ranked results or falls back to the input order on failure.
// Skips LLM reranking if all candidates already have high vector scores (≥0.9) since
// embedding-based similarity is already reliable in that range.
func rerankWithLLM(ctx context.Context, cfg *config.Config, logger *slog.Logger, candidates []rankedMemory, userQuery string, stm *memory.SQLiteMemory) []rankedMemory {
	settings := resolveMemoryAnalysisSettings(cfg, stm)
	if !settings.Enabled || !settings.LLMReranking || len(candidates) == 0 {
		return candidates
	}

	highConfidenceThreshold := 0.9
	allHighConfidence := true
	for _, c := range candidates {
		if c.score < highConfidenceThreshold {
			allHighConfidence = false
			break
		}
	}
	if allHighConfidence && len(candidates) > 1 {
		logger.Debug("[RAG LLM Rerank] Skipping — all candidates already have high vector similarity (≥0.9)")
		return candidates
	}

	llmCfg := resolveMemoryAnalysisLLMConfig(cfg)
	if llmCfg.model == "" {
		return candidates
	}

	rerankCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	client := llm.NewClientFromProvider(
		llmCfg.providerType,
		llmCfg.baseURL,
		llmCfg.apiKey,
	)

	model := llmCfg.model

	// Build candidate list for the prompt
	var sb strings.Builder
	for i, c := range candidates {
		text := c.text
		if len(text) > 300 {
			text = text[:300]
		}
		sb.WriteString(fmt.Sprintf("[%d] %s\n", i, text))
	}

	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("Rate how relevant each memory is to the query. Output ONLY a JSON array of scores (0-10), one per memory, in order.\n\nQuery: %s\n\nMemories:\n%s",
					userQuery, sb.String()),
			},
		},
		Temperature: 0.0,
		MaxTokens:   100,
	}

	resp, err := client.CreateChatCompletion(rerankCtx, req)
	if err != nil {
		logger.Debug("[RAG LLM Rerank] LLM call failed, keeping original order", "error", err)
		return candidates
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return candidates
	}

	raw := strings.TrimSpace(resp.Choices[0].Message.Content)
	// Strip markdown code fences if present
	if strings.HasPrefix(raw, "```") {
		if idx := strings.Index(raw[3:], "\n"); idx >= 0 {
			raw = raw[3+idx+1:]
		}
		if strings.HasSuffix(raw, "```") {
			raw = strings.TrimSuffix(raw, "```")
		}
		raw = strings.TrimSpace(raw)
	}

	var scores []float64
	if err := json.Unmarshal([]byte(raw), &scores); err != nil {
		logger.Debug("[RAG LLM Rerank] Failed to parse scores", "error", err, "raw", raw)
		return candidates
	}

	if len(scores) != len(candidates) {
		logger.Debug("[RAG LLM Rerank] Score count mismatch", "expected", len(candidates), "got", len(scores))
		return candidates
	}

	// Apply LLM scores: blend with existing similarity score (70% LLM, 30% original)
	for i := range candidates {
		llmScore := scores[i]
		if llmScore < 0 {
			llmScore = 0
		}
		if llmScore > 10 {
			llmScore = 10
		}
		normalizedLLM := llmScore / 10.0
		candidates[i].score = normalizedLLM*0.7 + candidates[i].score*0.3
	}

	// Sort by new blended score descending
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	logger.Debug("[RAG LLM Rerank] Re-ranked candidates", "count", len(candidates), "scores", scores)
	return candidates
}

func applyHelperRAGScores(logger *slog.Logger, candidates []rankedMemory, result helperRAGBatchResult) []rankedMemory {
	if len(candidates) == 0 || len(result.CandidateScores) == 0 {
		return candidates
	}

	scoreByID := make(map[string]float64, len(result.CandidateScores))
	for _, item := range result.CandidateScores {
		if item.MemoryID == "" {
			continue
		}
		scoreByID[item.MemoryID] = item.Score
	}

	applied := 0
	for i := range candidates {
		llmScore, ok := scoreByID[candidates[i].docID]
		if !ok {
			continue
		}
		normalizedLLM := llmScore / 10.0
		candidates[i].score = normalizedLLM*0.7 + candidates[i].score*0.3
		applied++
	}
	if applied == 0 {
		return candidates
	}

	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	if logger != nil {
		logger.Debug("[RAG Helper Batch] Re-ranked candidates", "count", len(candidates), "applied_scores", applied)
	}
	return candidates
}

// weeklyReflectionDue checks if the weekly reflection should run today.
func weeklyReflectionDue(cfg *config.Config, stm *memory.SQLiteMemory) bool {
	settings := resolveMemoryAnalysisSettings(cfg, stm)
	if !settings.Enabled || !settings.WeeklyReflection {
		return false
	}
	today := strings.ToLower(time.Now().Weekday().String())
	return today == strings.ToLower(settings.ReflectionDay)
}

const reflectionPrompt = `You are a memory analyst. Review the following memory data and produce a structured reflection.

Analyze:
1. **Patterns**: Recurring themes, topics, or behaviors across memories
2. **Contradictions**: Facts that conflict with each other (e.g., two different locations stored)
3. **Knowledge Gaps**: Areas where the user has mentioned topics but key details are missing
4. **Suggestions**: Specific recommendations for memory maintenance (what to consolidate, what to verify, what to remove)

Memory data (%s scope):

=== Recent Journal Entries ===
%s

=== Knowledge Graph Sample ===
%s

=== Core Memory Facts ===
%s

Respond in this JSON format:
{"patterns":["pattern1","pattern2"],"contradictions":["contradiction1"],"gaps":["gap1","gap2"],"suggestions":["suggestion1","suggestion2"],"summary":"Brief 2-3 sentence overall assessment"}`

// generateMemoryReflection produces a LLM-driven analysis of memory health and patterns.
func generateMemoryReflection(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
	stm *memory.SQLiteMemory,
	kg *memory.KnowledgeGraph,
	ltm memory.VectorDB,
	mainClient llm.ChatClient,
	scope string,
) (interface{}, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	var journalData, kgData, coreData, curatorData string
	scopeKey := strings.ToLower(strings.TrimSpace(scope))
	switch scopeKey {
	case "", "recent":
		scopeKey = "recent"
	case "week":
		scopeKey = "recent"
	case "month":
		scopeKey = "monthly"
	case "all_time":
		scopeKey = "full"
	}

	// Journal entries
	if stm != nil {
		limit := 10
		if scopeKey == "monthly" {
			limit = 30
		} else if scopeKey == "full" {
			limit = 50
		}
		entries, err := stm.SearchJournalEntries("", limit)
		if err == nil && len(entries) > 0 {
			b, _ := json.Marshal(entries)
			journalData = string(b)
		}
		if journalData == "" {
			journalData = "(no journal entries)"
		}
	} else {
		journalData = "(unavailable)"
	}

	// Knowledge Graph
	if kg != nil {
		kgData = kg.SearchForContext("*", 20, 2000)
		if kgData == "" {
			kgData = "(no knowledge graph data)"
		}
	} else {
		kgData = "(unavailable)"
	}

	// Core Memory
	if stm != nil {
		facts, err := stm.GetCoreMemoryFacts()
		if err == nil && len(facts) > 0 {
			b, _ := json.Marshal(facts)
			coreData = string(b)
		}
		if coreData == "" {
			coreData = "(no core memory facts)"
		}
	} else {
		coreData = "(unavailable)"
	}

	var curatorPayload interface{}
	if stm != nil {
		usageStats, usageErr := stm.GetMemoryUsageStats(14, 5)
		if usageErr == nil {
			metas, metaErr := stm.GetAllMemoryMeta(50000, 0)
			if metaErr == nil {
				report := memory.BuildMemoryHealthReport(metas, usageStats)
				if b, err := json.Marshal(report.Curator); err == nil {
					curatorData = string(b)
					curatorPayload = report.Curator
				}
			}
		}
	}
	if curatorData == "" {
		curatorData = "(unavailable)"
	}
	if curatorPayload == nil {
		curatorPayload = curatorData
	}

	if stm != nil && scopeKey == "recent" {
		if overview, err := stm.BuildRecentActivityOverview(7, true); err == nil && overview != nil {
			if b, err := json.Marshal(overview); err == nil {
				journalData = journalData + "\n\n=== Recent Activity Overview ===\n" + string(b)
			}
		}
	}

	prompt := fmt.Sprintf(reflectionPrompt, scopeKey, journalData, kgData, coreData) + "\n\n=== Curator Dry Run ===\n" + curatorData

	llmCfg := resolveMemoryAnalysisLLMConfig(cfg)
	analysisClient := mainClient
	model := strings.TrimSpace(cfg.MemoryAnalysis.ResolvedModel)
	if llmCfg.model != "" {
		analysisClient = llm.NewClientFromProvider(
			llmCfg.providerType,
			llmCfg.baseURL,
			llmCfg.apiKey,
		)
		model = llmCfg.model
	}

	if model == "" {
		model = cfg.LLM.Model
	}

	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.3,
		MaxTokens:   2000, // reasoning models need budget for thinking + JSON response
	}

	resp, err := analysisClient.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("reflection LLM call: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("empty reflection response")
	}

	raw := resp.Choices[0].Message.Content
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		if idx := strings.Index(raw[3:], "\n"); idx >= 0 {
			raw = raw[3+idx+1:]
		}
		if strings.HasSuffix(raw, "```") {
			raw = strings.TrimSuffix(raw, "```")
		}
		raw = strings.TrimSpace(raw)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		// If JSON parse fails, return as plain text
		return map[string]interface{}{
			"summary":         raw,
			"curator_dry_run": curatorPayload,
		}, nil
	}
	result["curator_dry_run"] = curatorPayload

	// Store reflection as a journal entry for future reference
	// Map internal scope names to human-readable labels
	if stm != nil {
		summary, _ := result["summary"].(string)
		if summary != "" {
			scopeLabel := scopeKey
			switch scopeKey {
			case "recent":
				scopeLabel = "weekly"
			case "monthly":
				scopeLabel = "monthly"
			case "full":
				scopeLabel = "full"
			}
			_, _ = stm.InsertJournalEntry(memory.JournalEntry{
				EntryType:     "reflection",
				Title:         fmt.Sprintf("Memory Reflection (%s)", scopeLabel),
				Content:       summary,
				Importance:    3,
				AutoGenerated: true,
			})
		}
	}

	return result, nil
}

func resolveMemoryAnalysisLLMConfig(cfg *config.Config) memoryAnalysisLLMConfig {
	if cfg == nil {
		return memoryAnalysisLLMConfig{}
	}

	if helperCfg := llm.ResolveHelperLLM(cfg); helperCfg.Enabled && helperCfg.Model != "" {
		return memoryAnalysisLLMConfig{
			providerType: helperCfg.ProviderType,
			baseURL:      helperCfg.BaseURL,
			apiKey:       helperCfg.APIKey,
			model:        helperCfg.Model,
		}
	}

	return memoryAnalysisLLMConfig{
		providerType: strings.TrimSpace(cfg.MemoryAnalysis.ProviderType),
		baseURL:      strings.TrimSpace(cfg.MemoryAnalysis.BaseURL),
		apiKey:       strings.TrimSpace(cfg.MemoryAnalysis.APIKey),
		model:        strings.TrimSpace(cfg.MemoryAnalysis.ResolvedModel),
	}
}
