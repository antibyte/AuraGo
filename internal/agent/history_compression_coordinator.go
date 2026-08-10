package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/prompts"

	openai "github.com/sashabaranov/go-openai"
)

const (
	proactiveHistoryCompressionCooldown         = time.Minute
	persistentCompressionMaxMessages            = 256
	persistentCompressionMaxTranscriptChars     = 24 * 1024
	persistentCompressionMaxMessageSummaryChars = 2000
)

type sessionCompressionState struct {
	gate          chan struct{}
	lastCompleted time.Time
	lastUsed      time.Time
}

type sessionCompressionCoordinator struct {
	mu       sync.Mutex
	sessions map[string]*sessionCompressionState
}

type sessionCompressionLease struct {
	coordinator *sessionCompressionCoordinator
	key         string
	state       *sessionCompressionState
	once        sync.Once
}

var historyCompressionCoordinator = &sessionCompressionCoordinator{sessions: make(map[string]*sessionCompressionState)}

func (c *sessionCompressionCoordinator) acquire(ctx context.Context, sessionID string, wait, honorCooldown bool) (*sessionCompressionLease, bool) {
	if c == nil {
		return nil, false
	}
	key := strings.TrimSpace(sessionID)
	if key == "" {
		key = "default"
	}
	c.mu.Lock()
	state := c.sessions[key]
	if state == nil {
		state = &sessionCompressionState{gate: make(chan struct{}, 1)}
		state.gate <- struct{}{}
		c.sessions[key] = state
	}
	state.lastUsed = time.Now()
	c.mu.Unlock()

	if wait {
		select {
		case <-ctx.Done():
			return nil, false
		case <-state.gate:
		}
	} else {
		select {
		case <-state.gate:
		default:
			return nil, false
		}
	}

	c.mu.Lock()
	onCooldown := honorCooldown && !state.lastCompleted.IsZero() && time.Since(state.lastCompleted) < proactiveHistoryCompressionCooldown
	c.mu.Unlock()
	if onCooldown {
		state.gate <- struct{}{}
		return nil, false
	}
	return &sessionCompressionLease{coordinator: c, key: key, state: state}, true
}

func (l *sessionCompressionLease) release(markCompleted bool) {
	if l == nil || l.state == nil {
		return
	}
	l.once.Do(func() {
		if l.coordinator != nil {
			l.coordinator.mu.Lock()
			l.state.lastUsed = time.Now()
			if markCompleted {
				l.state.lastCompleted = time.Now()
			}
			l.coordinator.mu.Unlock()
		}
		l.state.gate <- struct{}{}
	})
}

type persistentCompressionResult struct {
	Compressed bool
	Summary    string
	Dropped    []memory.HistoryMessage
}

func compressPersistentHistory(
	ctx context.Context,
	runCfg RunConfig,
	maxHistoryTokens int,
	targetChars int,
	force bool,
	model string,
	client llm.ChatClient,
	logger *slog.Logger,
) persistentCompressionResult {
	result := persistentCompressionResult{}
	if runCfg.HistoryManager == nil || runCfg.ShortTermMem == nil || client == nil || strings.TrimSpace(model) == "" {
		return result
	}
	history := runCfg.HistoryManager.GetAll()
	if len(history) < 2+compressionKeepTail {
		return result
	}

	plain := make([]openai.ChatCompletionMessage, len(history))
	totalTokens := 0
	for i, message := range history {
		plain[i] = message.ChatCompletionMessage
		totalTokens += prompts.CountTokensForModel(messageTextWithReasoningForAccounting(message.ChatCompletionMessage), model) + 4
	}
	if !force {
		threshold := int(float64(maxHistoryTokens) * compressionThresholdPct)
		if maxHistoryTokens <= 0 || totalTokens <= threshold {
			return result
		}
	}

	groups := buildConversationGroups(plain)
	if len(groups) < 2 {
		return result
	}
	protectedStart := groups[len(groups)-1].start
	protectedCount := len(history) - protectedStart
	for i := len(groups) - 2; i >= 0 && protectedCount < compressionKeepTail; i-- {
		protectedStart = groups[i].start
		protectedCount = len(history) - protectedStart
	}

	selected := make([]memory.HistoryMessage, 0, protectedStart)
	selectedChars := 0
	for _, group := range groups {
		if group.end > protectedStart {
			break
		}
		eligible := true
		for i := group.start; i < group.end; i++ {
			if history[i].Pinned || history[i].ID <= 0 {
				eligible = false
				break
			}
		}
		if !eligible {
			continue
		}
		groupMessages := group.end - group.start
		groupChars := 0
		for i := group.start; i < group.end; i++ {
			groupChars += len(persistentSummaryMessageText(history[i].ChatCompletionMessage)) + len(history[i].Role) + len("[]: \n\n")
		}
		if len(selected)+groupMessages > persistentCompressionMaxMessages ||
			selectedChars+groupChars > persistentCompressionMaxTranscriptChars {
			// Never split or delete half of a conversation/tool group merely to
			// fit the helper request or SQLite's parameter envelope.
			break
		}
		for i := group.start; i < group.end; i++ {
			selected = append(selected, history[i])
		}
		selectedChars += groupChars
		if force && targetChars > 0 && selectedChars >= targetChars {
			break
		}
	}
	if len(selected) == 0 {
		return result
	}

	prompt := buildPersistentHistorySummaryPrompt(runCfg.HistoryManager.GetSummary(), selected)
	summaryReq := openai.ChatCompletionRequest{
		Model:       model,
		Messages:    []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: prompt}},
		MaxTokens:   summaryMaxTokensForCount(len(selected)),
		Temperature: 0.2,
	}
	summaryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	response, err := client.CreateChatCompletion(summaryCtx, summaryReq)
	if err != nil {
		if logger != nil {
			logger.Warn("[Compression] Persistent summary failed", "error", err)
		}
		return result
	}
	if len(response.Choices) == 0 {
		return result
	}
	summary := strings.TrimSpace(response.Choices[0].Message.Content)
	if summary == "" {
		return result
	}
	requestedIDs := make([]int64, 0, len(selected))
	for _, message := range selected {
		requestedIDs = append(requestedIDs, message.ID)
	}
	// SQLite is the fallible persistent operation. Complete it before mutating
	// the in-memory/file-backed history so a database failure cannot publish a
	// summary while leaving all source messages active in SQLite.
	if err := runCfg.ShortTermMem.DeleteMessagesByID(runCfg.SessionID, requestedIDs); err != nil {
		if logger != nil {
			logger.Warn("[Compression] Failed to remove compressed SQLite messages", "error", err, "count", len(requestedIDs))
		}
		return result
	}
	droppedIDs, err := runCfg.HistoryManager.ApplyCompression(summary, requestedIDs)
	if err != nil || len(droppedIDs) == 0 {
		if err != nil && logger != nil {
			logger.Warn("[Compression] Failed to update persistent history", "error", err)
		}
		return result
	}
	droppedSet := make(map[int64]struct{}, len(droppedIDs))
	for _, id := range droppedIDs {
		droppedSet[id] = struct{}{}
	}
	for _, message := range selected {
		if _, ok := droppedSet[message.ID]; ok {
			result.Dropped = append(result.Dropped, message)
		}
	}
	result.Compressed = true
	result.Summary = summary
	if logger != nil {
		logger.Info("[Compression] Persistent history compressed", "dropped_messages", len(result.Dropped), "summary_tokens", prompts.CountTokensForModel(summary, model))
	}
	return result
}

func buildPersistentHistorySummaryPrompt(existingSummary string, messages []memory.HistoryMessage) string {
	var transcript strings.Builder
	if strings.TrimSpace(existingSummary) != "" {
		transcript.WriteString("[Persistent Summary]:\n")
		transcript.WriteString(existingSummary)
		transcript.WriteString("\n\n")
	}
	transcript.WriteString("[Recent Messages]:\n")
	for _, message := range messages {
		content := persistentSummaryMessageText(message.ChatCompletionMessage)
		_, _ = fmt.Fprintf(&transcript, "[%s]: %s\n\n", message.Role, content)
	}
	return buildSafeConversationSummaryPrompt(
		"Update the persistent summary with the recent messages. Preserve chronological facts, technical decisions, user preferences, tool outcomes, and pending actions. Output only a concise briefing.",
		transcript.String(),
	)
}

func persistentSummaryMessageText(message openai.ChatCompletionMessage) string {
	parts := make([]string, 0, 5)
	if content := messageText(message); content != "" {
		parts = append(parts, content)
	}
	if message.FunctionCall != nil {
		if encoded, err := json.Marshal(message.FunctionCall); err == nil {
			parts = append(parts, "function_call="+string(encoded))
		}
	}
	if len(message.ToolCalls) > 0 {
		if encoded, err := json.Marshal(message.ToolCalls); err == nil {
			parts = append(parts, "tool_calls="+string(encoded))
		}
	}
	if toolCallID := strings.TrimSpace(message.ToolCallID); toolCallID != "" {
		parts = append(parts, "tool_call_id="+toolCallID)
	}
	if refusal := strings.TrimSpace(message.Refusal); refusal != "" {
		parts = append(parts, "refusal="+refusal)
	}
	content := strings.Join(parts, "\n")
	if len(content) > persistentCompressionMaxMessageSummaryChars {
		content = truncateUTF8ToLimit(content, persistentCompressionMaxMessageSummaryChars+3, "...")
	}
	return content
}

func applyPersistentCompressionToRequest(messages []openai.ChatCompletionMessage, result persistentCompressionResult) []openai.ChatCompletionMessage {
	if !result.Compressed || strings.TrimSpace(result.Summary) == "" {
		return messages
	}
	drop := make([]bool, len(messages))
	cursor := 0
	for _, stored := range result.Dropped {
		for i := cursor; i < len(messages); i++ {
			if matchesStoredHistoryMessage(messages[i], stored) {
				drop[i] = true
				cursor = i + 1
				break
			}
		}
	}
	filtered := make([]openai.ChatCompletionMessage, 0, len(messages)+1)
	for i, message := range messages {
		if drop[i] || (message.Role == openai.ChatMessageRoleSystem && strings.HasPrefix(strings.TrimSpace(message.Content), "[CONTEXT_RECAP]:")) {
			continue
		}
		filtered = append(filtered, message)
	}
	recap := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: formatContextRecapForPrompt(result.Summary)}
	insertAt := 0
	if len(filtered) > 0 && filtered[0].Role == openai.ChatMessageRoleSystem {
		insertAt = 1
	}
	filtered = append(filtered, openai.ChatCompletionMessage{})
	copy(filtered[insertAt+1:], filtered[insertAt:])
	filtered[insertAt] = recap
	return filtered
}

func matchesStoredHistoryMessage(candidate openai.ChatCompletionMessage, stored memory.HistoryMessage) bool {
	if candidate.Role != stored.Role || candidate.ToolCallID != stored.ToolCallID {
		return false
	}
	return strings.TrimSpace(messageText(candidate)) == strings.TrimSpace(messageText(stored.ChatCompletionMessage))
}

// ScheduleProactiveHistoryCompression starts at most one background compressor
// for the default persistent session after a completed turn.
func ScheduleProactiveHistoryCompression(runCfg RunConfig) bool {
	if runCfg.Config == nil || runCfg.HistoryManager == nil || runCfg.ShortTermMem == nil || runCfg.IsCoAgent || runCfg.IsMission || strings.TrimSpace(runCfg.SessionID) != "default" {
		return false
	}
	charLimit := runCfg.Config.Agent.MemoryCompressionCharLimit
	if charLimit <= 0 || runCfg.HistoryManager.TotalChars() < int(float64(charLimit)*compressionThresholdPct) {
		return false
	}
	lease, ok := historyCompressionCoordinator.acquire(context.Background(), runCfg.SessionID, false, true)
	if !ok {
		return false
	}
	go func() {
		defer lease.release(true)
		client, model := resolveHelperBackedLLM(runCfg.Config, runCfg.LLMClient, runCfg.Config.LLM.Model)
		result := compressPersistentHistory(context.Background(), runCfg, 0, charLimit/5, true, model, client, runCfg.Logger)
		if !result.Compressed || runCfg.LongTermMem == nil || runCfg.LongTermMem.IsDisabled() {
			return
		}
		concept := fmt.Sprintf("Conversation summary %s", time.Now().Format("2006-01-02 15:04"))
		if _, err := runCfg.LongTermMem.StoreDocument(concept, result.Summary); err != nil && runCfg.Logger != nil {
			runCfg.Logger.Warn("[Compression] VectorDB archive of summary failed", "error", err)
		}
	}()
	return true
}
