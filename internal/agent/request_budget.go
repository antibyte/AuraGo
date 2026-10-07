package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/prompts"

	openai "github.com/sashabaranov/go-openai"
)

const (
	requestProtocolSafetyTokens = 256
	minimumSystemPromptTokens   = 500
	minimumRecentMessages       = 4
	historyWorkingSetMinTokens  = 64 * 1024
	historyWorkingSetMaxTokens  = 128 * 1024
	historyWorkingSetPercent    = 70
	historySummaryMaxTokens     = 8 * 1024
)

type historyWorkingSetStats struct {
	LimitTokens      int
	CurrentTokens    int
	KeptTokens       int
	SummaryTokens    int
	DroppedTokens    int
	CurrentUserIndex int
}

// RequestRouteBudget contains the effective model limits for one route.
type RequestRouteBudget struct {
	Limits llm.ModelLimits
}

// RequestBudget reserves output and protocol capacity before any prompt
// content is assembled. Every operation is evaluated against all routes.
type RequestBudget struct {
	Routes            []RequestRouteBudget
	CompletionReserve int
	SafetyMargin      int
	MinimumSystem     int
}

// RequestTokenUsage is content-free structured accounting for one route.
type RequestTokenUsage struct {
	ProviderID       string
	ProviderType     string
	Model            string
	ContextWindow    int
	ContextSource    string
	OutputSource     string
	SystemTokens     int
	SchemaTokens     int
	HistoryTokens    int
	CompletionTokens int
	SafetyTokens     int
	InputTokens      int
	TotalTokens      int
	Fits             bool
	ProbeCacheHit    bool
}

// ContextBudgetExceededError intentionally contains only numeric/model
// metadata. Prompt or user content must never be included in this error.
type ContextBudgetExceededError struct {
	Model             string
	ContextWindow     int
	RequiredInput     int
	CompletionReserve int
	SafetyMargin      int
}

func (e *ContextBudgetExceededError) Error() string {
	if e == nil {
		return "context_budget_exceeded"
	}
	return fmt.Sprintf("context_budget_exceeded: model=%s required=%d reserve=%d safety=%d context=%d",
		e.Model, e.RequiredInput, e.CompletionReserve, e.SafetyMargin, e.ContextWindow)
}

func (e *ContextBudgetExceededError) Code() string { return "context_budget_exceeded" }

func IsContextBudgetExceeded(err error) bool {
	var target *ContextBudgetExceededError
	return errors.As(err, &target)
}

func newRequestBudget(ctx context.Context, cfg *config.Config, client llm.ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger) (*RequestBudget, error) {
	return newRequestBudgetWithLimits(cfg, client, req, func(route llm.ModelRoute, cap int) llm.ModelLimits {
		return llm.ResolveModelLimits(ctx, route, cap, logger)
	})
}

func newRequestBudgetWithLimits(cfg *config.Config, client llm.ChatClient, req openai.ChatCompletionRequest, resolve func(llm.ModelRoute, int) llm.ModelLimits) (*RequestBudget, error) {
	routes := candidateModelRoutes(cfg, client, req)
	if len(routes) == 0 {
		routes = []llm.ModelRoute{{Model: req.Model, Primary: true}}
	}

	globalCap := 0
	if cfg != nil {
		globalCap = cfg.Agent.ContextWindow
	}
	resolved := make([]RequestRouteBudget, 0, len(routes))
	requestedOutput := req.MaxTokens
	knownReasoning := false
	for _, route := range routes {
		limits := resolve(route, globalCap)
		knownReasoning = knownReasoning || limits.Reasoning
		resolved = append(resolved, RequestRouteBudget{Limits: limits})
	}
	if requestedOutput <= 0 {
		requestedOutput = llm.ConservativeOutputTokens
		if knownReasoning {
			requestedOutput = llm.ReasoningOutputTokens
		}
	}
	for _, route := range resolved {
		if route.Limits.MaxOutputTokens > 0 && requestedOutput > route.Limits.MaxOutputTokens {
			requestedOutput = route.Limits.MaxOutputTokens
		}
	}
	if requestedOutput <= 0 {
		requestedOutput = llm.ConservativeOutputTokens
	}

	budget := &RequestBudget{
		Routes:            resolved,
		CompletionReserve: requestedOutput,
		SafetyMargin:      requestProtocolSafetyTokens,
		MinimumSystem:     minimumSystemPromptTokens,
	}
	for _, route := range budget.Routes {
		if route.Limits.ContextWindow-budget.CompletionReserve-budget.SafetyMargin < budget.MinimumSystem {
			return nil, budget.exceededError(route.Limits, budget.MinimumSystem)
		}
	}
	return budget, nil
}

// prepareRequestBudgetAndTools resolves route limits and sheds optional tool
// schemas to a fixed point. Route compatibility can change after schemas are
// removed (for example, a non-tool-capable fallback becomes usable), so each
// changed tool set is resolved again before prompt construction.
func prepareRequestBudgetAndTools(ctx context.Context, cfg *config.Config, client llm.ChatClient, req openai.ChatCompletionRequest, required map[string]bool, cache *tokenCountCache, logger *slog.Logger) (*RequestBudget, []openai.Tool, []string, error) {
	currentTools := append([]openai.Tool(nil), req.Tools...)
	droppedNames := make([]string, 0, len(currentTools))
	seenDropped := make(map[string]bool, len(currentTools))

	for iteration := 0; iteration < len(req.Tools)+2; iteration++ {
		candidateReq := req
		candidateReq.Tools = currentTools
		budget, err := newRequestBudget(ctx, cfg, client, candidateReq, logger)
		if err != nil {
			return nil, nil, nil, err
		}
		kept, dropped, err := budget.shedOptionalTools(candidateReq.Messages, currentTools, required, candidateReq.Model, cache)
		if err != nil {
			return nil, nil, nil, err
		}
		for _, name := range dropped {
			if name = strings.TrimSpace(name); name != "" && !seenDropped[name] {
				seenDropped[name] = true
				droppedNames = append(droppedNames, name)
			}
		}
		if len(dropped) == 0 {
			return budget, kept, droppedNames, nil
		}
		currentTools = kept
	}

	return nil, nil, nil, fmt.Errorf("request budget tool shedding did not converge")
}

func candidateModelRoutes(cfg *config.Config, client llm.ChatClient, req openai.ChatCompletionRequest) []llm.ModelRoute {
	if provider, ok := client.(llm.RequestRouteProvider); ok {
		if routes := provider.CandidateRoutes(req); len(routes) > 0 {
			return routes
		}
	}
	if cfg == nil {
		return nil
	}
	route := llm.ModelRoute{
		ProviderID:   cfg.LLM.Provider,
		ProviderType: cfg.LLM.ProviderType,
		BaseURL:      cfg.LLM.BaseURL,
		APIKey:       cfg.LLM.APIKey,
		Model:        firstNonEmpty(strings.TrimSpace(req.Model), cfg.LLM.Model),
		Primary:      true,
	}
	// Direct clients do not expose a route snapshot. Resolve limits from the
	// concrete provider/model identity without copying overrides from a provider
	// whose default model or endpoint differs from this request.
	if provider := cfg.FindProvider(cfg.LLM.Provider); provider != nil &&
		strings.EqualFold(strings.TrimSpace(provider.Type), strings.TrimSpace(route.ProviderType)) &&
		strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/") == strings.TrimRight(strings.TrimSpace(route.BaseURL), "/") &&
		strings.TrimSpace(provider.Model) == strings.TrimSpace(route.Model) {
		route.ContextWindowOverride = provider.ContextWindow
		route.MaxOutputTokensOverride = provider.MaxOutputTokens
	}
	return []llm.ModelRoute{route}
}

func (b *RequestBudget) exceededError(limits llm.ModelLimits, requiredInput int) error {
	return &ContextBudgetExceededError{
		Model:             limits.Route.Model,
		ContextWindow:     limits.ContextWindow,
		RequiredInput:     requiredInput,
		CompletionReserve: b.CompletionReserve,
		SafetyMargin:      b.SafetyMargin,
	}
}

func (b *RequestBudget) inputLimit(limits llm.ModelLimits) int {
	return limits.ContextWindow - b.CompletionReserve - b.SafetyMargin
}

func (b *RequestBudget) tokenUsage(messages []openai.ChatCompletionMessage, tools []openai.Tool, cache *tokenCountCache) []RequestTokenUsage {
	if b == nil {
		return nil
	}
	toolJSON, _ := json.Marshal(tools)
	usage := make([]RequestTokenUsage, 0, len(b.Routes))
	for _, routeBudget := range b.Routes {
		limits := routeBudget.Limits
		model := limits.Route.Model
		systemTokens, historyTokens := 0, 0
		for _, message := range messages {
			tokens := routeMessageTokens(message, limits.Route, cache)
			if message.Role == openai.ChatMessageRoleSystem {
				systemTokens += tokens
			} else {
				historyTokens += tokens
			}
		}
		schemaTokens := 0
		if len(toolJSON) > 0 && len(tools) > 0 {
			schemaTokens = cache.Count(string(toolJSON), model)
		}
		input := systemTokens + historyTokens + schemaTokens
		total := input + b.CompletionReserve + b.SafetyMargin
		usage = append(usage, RequestTokenUsage{
			ProviderID:       limits.Route.ProviderID,
			ProviderType:     limits.Route.ProviderType,
			Model:            model,
			ContextWindow:    limits.ContextWindow,
			ContextSource:    limits.ContextSource,
			OutputSource:     limits.OutputSource,
			SystemTokens:     systemTokens,
			SchemaTokens:     schemaTokens,
			HistoryTokens:    historyTokens,
			CompletionTokens: b.CompletionReserve,
			SafetyTokens:     b.SafetyMargin,
			InputTokens:      input,
			TotalTokens:      total,
			Fits:             total <= limits.ContextWindow,
			ProbeCacheHit:    limits.ProbeCacheHit,
		})
	}
	return usage
}

func (b *RequestBudget) validate(messages []openai.ChatCompletionMessage, tools []openai.Tool, cache *tokenCountCache) ([]RequestTokenUsage, error) {
	usage := b.tokenUsage(messages, tools, cache)
	for i, item := range usage {
		if item.Fits {
			continue
		}
		return usage, b.exceededError(b.Routes[i].Limits, item.InputTokens)
	}
	return usage, nil
}

// minimumRequestMessages retains trusted system addenda plus the latest full
// user/tool group. It is used both for preflight and impossible-request checks.
func minimumRequestMessages(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return nil
	}
	groups := buildConversationGroups(messages)
	keep := make([]bool, len(messages))
	for i, message := range messages {
		if message.Role == openai.ChatMessageRoleSystem && !isSheddableHistorySystem(message) {
			keep[i] = true
		}
	}
	if len(groups) > 0 {
		latest := groups[len(groups)-1]
		for i := latest.start; i < latest.end; i++ {
			keep[i] = true
		}
	}
	out := make([]openai.ChatCompletionMessage, 0, len(messages))
	for i, message := range messages {
		if keep[i] {
			out = append(out, message)
		}
	}
	return out
}

// validateMinimum verifies the non-sheddable request before an HTTP stream is
// opened. minimumSystemTokens reserves the prompt builder's mandatory core.
func (b *RequestBudget) validateMinimum(messages []openai.ChatCompletionMessage, tools []openai.Tool, cache *tokenCountCache) error {
	minimal := minimumRequestMessages(messages)
	for _, route := range b.Routes {
		model := route.Limits.Route.Model
		input := minimumSystemPromptTokens
		for _, message := range minimal {
			input += routeMessageTokens(message, route.Limits.Route, cache)
		}
		if len(tools) > 0 {
			encoded, _ := json.Marshal(tools)
			input += cache.Count(string(encoded), model)
		}
		if input+b.CompletionReserve+b.SafetyMargin > route.Limits.ContextWindow {
			return b.exceededError(route.Limits, input)
		}
	}
	return nil
}

func (b *RequestBudget) systemPromptBudget(messages []openai.ChatCompletionMessage, tools []openai.Tool, promptModel string, cache *tokenCountCache) (int, error) {
	minimal := minimumRequestMessages(messages)
	best := int(^uint(0) >> 1)
	encoded, _ := json.Marshal(tools)
	for _, route := range b.Routes {
		model := route.Limits.Route.Model
		// The generated system message itself carries the same fixed protocol
		// overhead as every other message in tokenUsage.
		available := b.inputLimit(route.Limits) - 4
		for _, message := range minimal {
			available -= routeMessageTokens(message, route.Limits.Route, cache)
		}
		if len(tools) > 0 {
			available -= cache.Count(string(encoded), model)
		}
		if available < b.MinimumSystem {
			return 0, b.exceededError(route.Limits, b.MinimumSystem+(b.inputLimit(route.Limits)-available))
		}
		buildModelBudget := prompts.TranslateTokenBudget(available, model, promptModel)
		if buildModelBudget < b.MinimumSystem {
			return 0, b.exceededError(route.Limits, b.MinimumSystem+(b.inputLimit(route.Limits)-available))
		}
		if buildModelBudget < best {
			best = buildModelBudget
		}
	}
	return best, nil
}

func (b *RequestBudget) historyBudget(systemPrompt string, tools []openai.Tool, cache *tokenCountCache) int {
	best := int(^uint(0) >> 1)
	encoded, _ := json.Marshal(tools)
	for _, route := range b.Routes {
		model := route.Limits.Route.Model
		available := b.inputLimit(route.Limits)
		if systemPrompt != "" {
			available -= cache.Count(systemPrompt, model) + 4
		}
		if len(tools) > 0 {
			available -= cache.Count(string(encoded), model)
		}
		if available < best {
			best = available
		}
	}
	if best < 0 {
		return 0
	}
	return best
}

func (b *RequestBudget) historyWorkingSetLimit(systemPrompt string, tools []openai.Tool, cache *tokenCountCache) int {
	available := b.historyBudget(systemPrompt, tools, cache)
	limit := available
	for _, route := range b.Routes {
		routeLimit := route.Limits.ContextWindow * historyWorkingSetPercent / 100
		if routeLimit < historyWorkingSetMinTokens {
			routeLimit = historyWorkingSetMinTokens
		}
		if routeLimit > historyWorkingSetMaxTokens {
			routeLimit = historyWorkingSetMaxTokens
		}
		if routeLimit < limit {
			limit = routeLimit
		}
	}
	if limit < 0 {
		return 0
	}
	return limit
}

func (b *RequestBudget) historyWorkingSetLimitForMessages(messages []openai.ChatCompletionMessage, currentUser int, systemPrompt string, tools []openai.Tool, cache *tokenCountCache) int {
	best := int(^uint(0) >> 1)
	encoded, _ := json.Marshal(tools)
	for _, route := range b.Routes {
		model := route.Limits.Route.Model
		available := b.inputLimit(route.Limits)
		if systemPrompt != "" {
			available -= cache.Count(systemPrompt, model) + 4
		}
		if len(tools) > 0 {
			available -= cache.Count(string(encoded), model)
		}
		if currentUser >= 0 && currentUser < len(messages) {
			available -= routeMessageTokens(messages[currentUser], route.Limits.Route, cache)
		}
		available -= fixedSystemAddendaTokens(messages, model, cache, route.Limits.Route)
		limit := route.Limits.ContextWindow * historyWorkingSetPercent / 100
		if limit < historyWorkingSetMinTokens {
			limit = historyWorkingSetMinTokens
		}
		if limit > historyWorkingSetMaxTokens {
			limit = historyWorkingSetMaxTokens
		}
		if available < limit {
			limit = available
		}
		if limit < best {
			best = limit
		}
	}
	if best < 0 {
		return 0
	}
	return best
}

// trimHistoryWorkingSet bounds carried conversation state independently from
// the current human request. Older complete turns and compact summaries are
// removable; the current request and its two newest native tool rounds remain
// intact even when they alone make the request impossible.
func (b *RequestBudget) trimHistoryWorkingSet(messages []openai.ChatCompletionMessage, currentUserText, systemPrompt string, tools []openai.Tool, cache *tokenCountCache) ([]openai.ChatCompletionMessage, []openai.ChatCompletionMessage, historyWorkingSetStats) {
	return b.trimHistoryWorkingSetWithTaskAnchor(messages, currentUserText, protectedTaskMessageIndex(messages, currentUserText), systemPrompt, tools, cache)
}

func (b *RequestBudget) trimHistoryWorkingSetWithTaskAnchor(messages []openai.ChatCompletionMessage, currentUserText string, anchorIndex int, systemPrompt string, tools []openai.Tool, cache *tokenCountCache) ([]openai.ChatCompletionMessage, []openai.ChatCompletionMessage, historyWorkingSetStats) {
	working := append([]openai.ChatCompletionMessage(nil), messages...)
	currentUser := anchorIndex
	stats := historyWorkingSetStats{LimitTokens: b.historyWorkingSetLimitForMessages(working, currentUser, systemPrompt, tools, cache), CurrentUserIndex: currentUser}
	if len(working) == 0 {
		return working, nil, stats
	}
	if !validTaskAnchor(working, currentUser, currentUserText) {
		return working, nil, stats
	}
	stats.CurrentTokens = b.maxMessageTokensAt(working, currentUser, cache)
	stats.KeptTokens = b.maxCarriedHistoryTokens(working, currentUser, cache)
	if b.historyWorkingSetFits(working, currentUser, systemPrompt, tools, cache) {
		stats.SummaryTokens = b.maxSummaryTokens(working, cache)
		return working, nil, stats
	}

	groups := buildConversationGroups(working)
	protectedRounds := newestNativeToolRoundIndices(working, 2)
	var dropped []openai.ChatCompletionMessage
	for len(groups) > 1 && !b.historyWorkingSetFits(working, currentUser, systemPrompt, tools, cache) {
		candidate := -1
		for i, group := range groups {
			if group.end <= currentUser && !groupContainsAnyIndex(group, protectedRounds) {
				candidate = i
				break
			}
		}
		if candidate < 0 {
			break
		}
		group := groups[candidate]
		dropped = append(dropped, working[group.start:group.end]...)
		if group.start < currentUser {
			currentUser -= group.end - group.start
		}
		protectedRounds = rebaseProtectedIndicesAfterRemoval(protectedRounds, group.start, group.end)
		working = append(append([]openai.ChatCompletionMessage(nil), working[:group.start]...), working[group.end:]...)
		groups = buildConversationGroups(working)
	}

	// Compacted tool summaries and old recaps are system messages after the
	// generated system prompt. They are history, not immutable policy.
	for i := 1; i < len(working) && !b.historyWorkingSetFits(working, currentUser, systemPrompt, tools, cache); {
		if !isSheddableHistorySystem(working[i]) {
			i++
			continue
		}
		dropped = append(dropped, working[i])
		working = append(working[:i], working[i+1:]...)
		if i < currentUser {
			currentUser--
		}
		protectedRounds = rebaseProtectedIndicesAfterRemoval(protectedRounds, i, i+1)
	}

	stats.CurrentUserIndex = currentUser
	stats.DroppedTokens = b.maxMessagesTokens(dropped, cache)
	stats.KeptTokens = b.maxCarriedHistoryTokens(working, currentUser, cache)
	stats.SummaryTokens = b.maxSummaryTokens(working, cache)
	return working, dropped, stats
}

func latestGenuineUserIndex(messages []openai.ChatCompletionMessage) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == openai.ChatMessageRoleUser && !isTextModeToolResult(messages[i]) {
			return i
		}
	}
	return -1
}

func currentUserMessageIndex(messages []openai.ChatCompletionMessage, currentUserText string) int {
	wanted := strings.TrimSpace(currentUserText)
	if wanted != "" {
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == openai.ChatMessageRoleUser && !isTextModeToolResult(messages[i]) && strings.TrimSpace(messageText(messages[i])) == wanted {
				return i
			}
		}
	}
	return latestGenuineUserIndex(messages)
}

func (b *RequestBudget) historyWorkingSetFits(messages []openai.ChatCompletionMessage, currentUser int, systemPrompt string, tools []openai.Tool, cache *tokenCountCache) bool {
	for _, route := range b.Routes {
		model := route.Limits.Route.Model
		available := b.inputLimit(route.Limits)
		if systemPrompt != "" {
			available -= cache.Count(systemPrompt, model) + 4
		}
		if len(tools) > 0 {
			encoded, _ := json.Marshal(tools)
			available -= cache.Count(string(encoded), model)
		}
		if currentUser >= 0 && currentUser < len(messages) {
			available -= routeMessageTokens(messages[currentUser], route.Limits.Route, cache)
		}
		available -= fixedSystemAddendaTokens(messages, model, cache, route.Limits.Route)
		limit := route.Limits.ContextWindow * historyWorkingSetPercent / 100
		if limit < historyWorkingSetMinTokens {
			limit = historyWorkingSetMinTokens
		}
		if limit > historyWorkingSetMaxTokens {
			limit = historyWorkingSetMaxTokens
		}
		if available < limit {
			limit = available
		}
		if carriedHistoryTokensForModel(messages, currentUser, model, cache, route.Limits.Route) > limit {
			return false
		}
	}
	return true
}

func carriedHistoryTokensForModel(messages []openai.ChatCompletionMessage, currentUser int, model string, cache *tokenCountCache, routes ...llm.ModelRoute) int {
	route := llm.ModelRoute{Model: model}
	if len(routes) > 0 {
		route = routes[0]
	}
	total := 0
	for i, message := range messages {
		if i == currentUser || message.Role == openai.ChatMessageRoleSystem && (i == 0 || !isSheddableHistorySystem(message)) {
			continue
		}
		total += routeMessageTokens(message, route, cache)
	}
	return total
}

func (b *RequestBudget) maxCarriedHistoryTokens(messages []openai.ChatCompletionMessage, currentUser int, cache *tokenCountCache) int {
	maxTokens := 0
	for _, route := range b.Routes {
		if tokens := carriedHistoryTokensForModel(messages, currentUser, route.Limits.Route.Model, cache, route.Limits.Route); tokens > maxTokens {
			maxTokens = tokens
		}
	}
	return maxTokens
}

func (b *RequestBudget) maxMessageTokensAt(messages []openai.ChatCompletionMessage, index int, cache *tokenCountCache) int {
	if index < 0 || index >= len(messages) {
		return 0
	}
	return b.maxMessagesTokens(messages[index:index+1], cache)
}

func (b *RequestBudget) maxMessagesTokens(messages []openai.ChatCompletionMessage, cache *tokenCountCache) int {
	maxTokens := 0
	for _, route := range b.Routes {
		total := 0
		for _, message := range messages {
			total += routeMessageTokens(message, route.Limits.Route, cache)
		}
		if total > maxTokens {
			maxTokens = total
		}
	}
	return maxTokens
}

func (b *RequestBudget) maxSummaryTokens(messages []openai.ChatCompletionMessage, cache *tokenCountCache) int {
	summaries := make([]openai.ChatCompletionMessage, 0, 2)
	for _, message := range messages {
		if isSheddableHistorySystem(message) {
			summaries = append(summaries, message)
		}
	}
	return b.maxMessagesTokens(summaries, cache)
}

func isSheddableHistorySystem(message openai.ChatCompletionMessage) bool {
	if message.Role != openai.ChatMessageRoleSystem {
		return false
	}
	content := strings.TrimSpace(message.Content)
	return strings.HasPrefix(content, "[CONTEXT_RECAP]:") ||
		strings.HasPrefix(content, "[TRIMMED_CONTEXT_RECAP]:") ||
		strings.HasPrefix(content, "[TaskStateSummary]") ||
		strings.HasPrefix(content, "[RELEVANT_CONVERSATION_CONTEXT]")
}

func fixedSystemAddendaTokens(messages []openai.ChatCompletionMessage, model string, cache *tokenCountCache, routes ...llm.ModelRoute) int {
	route := llm.ModelRoute{Model: model}
	if len(routes) > 0 {
		route = routes[0]
	}
	total := 0
	for i, message := range messages {
		if i == 0 || message.Role != openai.ChatMessageRoleSystem || isSheddableHistorySystem(message) {
			continue
		}
		total += routeMessageTokens(message, route, cache)
	}
	return total
}

func (b *RequestBudget) shedOptionalTools(messages []openai.ChatCompletionMessage, tools []openai.Tool, required map[string]bool, promptModel string, cache *tokenCountCache) ([]openai.Tool, []string, error) {
	kept := append([]openai.Tool(nil), tools...)
	var dropped []string
	for len(kept) > 0 {
		if _, err := b.systemPromptBudget(messages, kept, promptModel, cache); err == nil {
			return kept, dropped, nil
		}
		removeAt := -1
		for i := len(kept) - 1; i >= 0; i-- {
			name := ""
			if kept[i].Function != nil {
				name = kept[i].Function.Name
			}
			if !required[name] {
				removeAt = i
				break
			}
		}
		if removeAt < 0 {
			break
		}
		if kept[removeAt].Function != nil {
			dropped = append(dropped, kept[removeAt].Function.Name)
		}
		kept = append(kept[:removeAt], kept[removeAt+1:]...)
	}
	if _, err := b.systemPromptBudget(messages, kept, promptModel, cache); err != nil {
		return kept, dropped, err
	}
	return kept, dropped, nil
}

type conversationGroup struct {
	start int
	end   int
}

func buildConversationGroups(messages []openai.ChatCompletionMessage) []conversationGroup {
	var groups []conversationGroup
	start := -1
	for i, message := range messages {
		if message.Role == openai.ChatMessageRoleSystem {
			if start >= 0 {
				groups = append(groups, conversationGroup{start: start, end: i})
				start = -1
			}
			continue
		}
		boundary := message.Role == openai.ChatMessageRoleUser && !isTextModeToolResult(message)
		if start < 0 {
			start = i
			continue
		}
		if boundary {
			groups = append(groups, conversationGroup{start: start, end: i})
			start = i
		}
	}
	if start >= 0 {
		groups = append(groups, conversationGroup{start: start, end: len(messages)})
	}
	return groups
}

func nativeToolRoundGroups(messages []openai.ChatCompletionMessage) [][]int {
	var rounds [][]int
	for i, message := range messages {
		if message.Role != openai.ChatMessageRoleAssistant || len(message.ToolCalls) == 0 {
			continue
		}
		group := []int{i}
		for j := i + 1; j < len(messages); j++ {
			if messages[j].Role != openai.ChatMessageRoleTool && messages[j].ToolCallID == "" {
				break
			}
			group = append(group, j)
		}
		rounds = append(rounds, group)
	}
	return rounds
}

func newestNativeToolRoundIndices(messages []openai.ChatCompletionMessage, count int) []int {
	if count <= 0 {
		return nil
	}
	rounds := nativeToolRoundGroups(messages)
	start := len(rounds) - count
	if start < 0 {
		start = 0
	}
	var protected []int
	for _, round := range rounds[start:] {
		protected = append(protected, round...)
	}
	return protected
}

func groupContainsAnyIndex(group conversationGroup, protected []int) bool {
	for _, index := range protected {
		if index >= group.start && index < group.end {
			return true
		}
	}
	return false
}

func rebaseProtectedIndicesAfterRemoval(indices []int, start, end int) []int {
	removed := end - start
	updated := indices[:0]
	for _, index := range indices {
		if index >= start && index < end {
			continue
		}
		if index >= end {
			index -= removed
		}
		updated = append(updated, index)
	}
	return updated
}

func rebaseProtectedIndicesAfterIndicesRemoval(indices, dropped []int) []int {
	updated := indices[:0]
	for _, index := range indices {
		shift := 0
		for _, removed := range dropped {
			if removed < index {
				shift++
			}
		}
		updated = append(updated, index-shift)
	}
	return updated
}

func isTextModeToolResult(message openai.ChatCompletionMessage) bool {
	if message.Role != openai.ChatMessageRoleUser {
		return false
	}
	content := strings.TrimSpace(messageText(message))
	return strings.HasPrefix(content, "Tool Output:") || strings.HasPrefix(content, "[Tool Output]") || strings.HasPrefix(content, "Tool Result:")
}

func (b *RequestBudget) trimHistory(messages []openai.ChatCompletionMessage, tools []openai.Tool, importance bool, logger *slog.Logger, cache *tokenCountCache) ([]openai.ChatCompletionMessage, []openai.ChatCompletionMessage, error) {
	return b.trimHistoryWithCurrentUser(messages, tools, importance, logger, cache, "")
}

// trimHistoryWithCurrentUser keeps the original human request immutable while
// optional importance and chronological trimming shed prior conversation.
func (b *RequestBudget) trimHistoryWithCurrentUser(messages []openai.ChatCompletionMessage, tools []openai.Tool, importance bool, logger *slog.Logger, cache *tokenCountCache, currentUserText string) ([]openai.ChatCompletionMessage, []openai.ChatCompletionMessage, error) {
	anchorIndex := protectedTaskMessageIndex(messages, currentUserText)
	trimmed, dropped, _, err := b.trimHistoryWithTaskAnchor(messages, tools, importance, logger, cache, currentUserText, anchorIndex)
	return trimmed, dropped, err
}

// trimHistoryWithTaskAnchor threads an immutable message identity through every
// trimming pass and returns its adjusted index in the resulting slice.
func (b *RequestBudget) trimHistoryWithTaskAnchor(messages []openai.ChatCompletionMessage, tools []openai.Tool, importance bool, logger *slog.Logger, cache *tokenCountCache, currentUserText string, anchorIndex int) ([]openai.ChatCompletionMessage, []openai.ChatCompletionMessage, int, error) {
	working := append([]openai.ChatCompletionMessage(nil), messages...)
	var dropped []openai.ChatCompletionMessage
	if !validTaskAnchor(working, anchorIndex, currentUserText) {
		return working, nil, anchorIndex, fmt.Errorf("current human request is missing from history; refusing to trim it")
	}
	if _, err := b.validate(working, tools, cache); err == nil {
		return working, nil, anchorIndex, nil
	}

	// Importance scoring gets the first opportunity, but its budget includes
	// schemas and fixed reserves. Group-aware chronological trimming remains the
	// fail-safe and establishes the hard postcondition.
	protectedRounds := newestNativeToolRoundIndices(working, 2)
	if importance && len(b.Routes) > 0 {
		target := b.historyAndSystemLimit(tools, cache)
		original := append([]openai.ChatCompletionMessage(nil), working...)
		originalAnchorIndex := anchorIndex
		protected := append([]int{anchorIndex}, protectedRounds...)
		trimmed, indices, _ := trimByImportanceProtecting(working, target, b.Routes[0].Limits.Route.Model, logger, protected...)
		for _, index := range indices {
			if index >= 0 && index < len(original) {
				dropped = append(dropped, original[index])
				if index < originalAnchorIndex {
					anchorIndex--
				}
			}
		}
		protectedRounds = rebaseProtectedIndicesAfterIndicesRemoval(protectedRounds, indices)
		working = trimmed
	}

	if _, err := b.validate(working, tools, cache); err == nil {
		return working, dropped, anchorIndex, nil
	}
	var chronologicalDropped []openai.ChatCompletionMessage
	working, chronologicalDropped, anchorIndex = b.trimOldestConversationGroupsPreserving(working, tools, cache, anchorIndex, protectedRounds)
	dropped = append(dropped, chronologicalDropped...)
	if _, err := b.validate(working, tools, cache); err != nil {
		return working, dropped, anchorIndex, err
	}
	return working, dropped, anchorIndex, nil
}

func (b *RequestBudget) historyAndSystemLimit(tools []openai.Tool, cache *tokenCountCache) int {
	limit := int(^uint(0) >> 1)
	encoded, _ := json.Marshal(tools)
	for _, route := range b.Routes {
		available := b.inputLimit(route.Limits)
		if len(tools) > 0 {
			available -= cache.Count(string(encoded), route.Limits.Route.Model)
		}
		if available < limit {
			limit = available
		}
	}
	return limit
}

func (b *RequestBudget) trimOldestConversationGroups(messages []openai.ChatCompletionMessage, tools []openai.Tool, cache *tokenCountCache) ([]openai.ChatCompletionMessage, []openai.ChatCompletionMessage) {
	return b.trimOldestConversationGroupsWithCurrentUser(messages, tools, cache, "")
}

func (b *RequestBudget) trimOldestConversationGroupsWithCurrentUser(messages []openai.ChatCompletionMessage, tools []openai.Tool, cache *tokenCountCache, currentUserText string) ([]openai.ChatCompletionMessage, []openai.ChatCompletionMessage) {
	trimmed, dropped, _ := b.trimOldestConversationGroupsWithTaskAnchor(messages, tools, cache, protectedTaskMessageIndex(messages, currentUserText))
	return trimmed, dropped
}

func (b *RequestBudget) trimOldestConversationGroupsWithTaskAnchor(messages []openai.ChatCompletionMessage, tools []openai.Tool, cache *tokenCountCache, anchorIndex int) ([]openai.ChatCompletionMessage, []openai.ChatCompletionMessage, int) {
	return b.trimOldestConversationGroupsPreserving(messages, tools, cache, anchorIndex, newestNativeToolRoundIndices(messages, 2))
}

func (b *RequestBudget) trimOldestConversationGroupsPreserving(messages []openai.ChatCompletionMessage, tools []openai.Tool, cache *tokenCountCache, anchorIndex int, protectedRounds []int) ([]openai.ChatCompletionMessage, []openai.ChatCompletionMessage, int) {
	working := append([]openai.ChatCompletionMessage(nil), messages...)
	protectedRounds = append([]int(nil), protectedRounds...)
	var dropped []openai.ChatCompletionMessage
	for pass := 0; pass < 2; pass++ {
		for {
			if _, err := b.validate(working, tools, cache); err == nil {
				return working, dropped, anchorIndex
			}
			groups := buildConversationGroups(working)
			if len(groups) == 0 {
				return working, dropped, anchorIndex
			}
			protectedGroup := -1
			for i, group := range groups {
				if anchorIndex >= group.start && anchorIndex < group.end {
					protectedGroup = i
					break
				}
			}
			candidateIndex := -1
			for groupIndex := range groups {
				if groupIndex == protectedGroup {
					continue
				}
				if groupContainsAnyIndex(groups[groupIndex], protectedRounds) {
					continue
				}
				if candidateIndex < 0 {
					candidateIndex = groupIndex
				}
			}
			if candidateIndex < 0 {
				return working, dropped, anchorIndex
			}
			candidate := groups[candidateIndex]
			remainingNonSystem := 0
			for groupIndex, group := range groups {
				if groupIndex != candidateIndex {
					remainingNonSystem += group.end - group.start
				}
			}
			if pass == 0 && remainingNonSystem < minimumRecentMessages {
				break
			}
			dropped = append(dropped, working[candidate.start:candidate.end]...)
			if candidate.start < anchorIndex {
				anchorIndex -= candidate.end - candidate.start
			}
			protectedRounds = rebaseProtectedIndicesAfterRemoval(protectedRounds, candidate.start, candidate.end)
			next := make([]openai.ChatCompletionMessage, 0, len(working)-(candidate.end-candidate.start))
			next = append(next, working[:candidate.start]...)
			next = append(next, working[candidate.end:]...)
			working = next
		}
	}
	return working, dropped, anchorIndex
}

// protectedTaskMessageIndex requires an exact identity when the caller supplies
// the immutable task text. Legacy callers without an anchor use the newest packet.
func protectedTaskMessageIndex(messages []openai.ChatCompletionMessage, currentUserText string) int {
	wanted := strings.TrimSpace(currentUserText)
	if wanted != "" {
		return exactTaskMessageIndex(messages, wanted)
	}
	return latestGenuineUserIndex(messages)
}

func exactTaskMessageIndex(messages []openai.ChatCompletionMessage, currentUserText string) int {
	wanted := strings.TrimSpace(currentUserText)
	if wanted == "" {
		return -1
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == openai.ChatMessageRoleUser && !isTextModeToolResult(messages[i]) && strings.TrimSpace(messageText(messages[i])) == wanted {
			return i
		}
	}
	return -1
}

func genuineUserOccurrence(messages []openai.ChatCompletionMessage, anchorIndex int) int {
	if anchorIndex < 0 || anchorIndex >= len(messages) || messages[anchorIndex].Role != openai.ChatMessageRoleUser || isTextModeToolResult(messages[anchorIndex]) {
		return -1
	}
	occurrence := 0
	for i := 0; i < anchorIndex; i++ {
		if messages[i].Role == openai.ChatMessageRoleUser && !isTextModeToolResult(messages[i]) {
			occurrence++
		}
	}
	return occurrence
}

func genuineUserIndexByOccurrence(messages []openai.ChatCompletionMessage, occurrence int) int {
	if occurrence < 0 {
		return -1
	}
	for i, message := range messages {
		if message.Role != openai.ChatMessageRoleUser || isTextModeToolResult(message) {
			continue
		}
		if occurrence == 0 {
			return i
		}
		occurrence--
	}
	return -1
}

func validTaskAnchor(messages []openai.ChatCompletionMessage, anchorIndex int, currentUserText string) bool {
	if anchorIndex < 0 {
		return strings.TrimSpace(currentUserText) == ""
	}
	if anchorIndex >= len(messages) || messages[anchorIndex].Role != openai.ChatMessageRoleUser || isTextModeToolResult(messages[anchorIndex]) {
		return false
	}
	return strings.TrimSpace(currentUserText) == "" || strings.TrimSpace(messageText(messages[anchorIndex])) == strings.TrimSpace(currentUserText)
}

// rebaseTaskAnchorForGeneratedSystemPrompt mirrors the prefix filtering in
// ensureGeneratedSystemPromptMessage while preserving the selected user-message
// identity across prompt replacement and blank-system cleanup.
func rebaseTaskAnchorForGeneratedSystemPrompt(messages []openai.ChatCompletionMessage, anchorIndex int, previousGenerated string) int {
	if anchorIndex < 0 || anchorIndex >= len(messages) || messages[anchorIndex].Role != openai.ChatMessageRoleUser || isTextModeToolResult(messages[anchorIndex]) {
		return -1
	}
	start := 0
	if len(messages) > 0 && messages[0].Role == openai.ChatMessageRoleSystem {
		firstContent := strings.TrimSpace(messages[0].Content)
		if firstContent == "" || (previousGenerated != "" && messages[0].Content == previousGenerated) {
			start = 1
		}
	}
	newIndex := 1 // The generated system prompt is always inserted at index zero.
	for i := start; i < len(messages); i++ {
		message := messages[i]
		if message.Role == openai.ChatMessageRoleSystem && strings.TrimSpace(message.Content) == "" {
			continue
		}
		if i == anchorIndex {
			return newIndex
		}
		newIndex++
	}
	return -1
}

func appendRecapWithinBudget(budget *RequestBudget, messages []openai.ChatCompletionMessage, tools []openai.Tool, dropped []openai.ChatCompletionMessage, cache *tokenCountCache) []openai.ChatCompletionMessage {
	trimmed, _ := appendRecapWithinBudgetWithTaskAnchor(budget, messages, tools, dropped, cache, -1)
	return trimmed
}

// appendRecapWithinBudgetWithTaskAnchor keeps a stable task-message index when
// inserting the recap system message before the conversation history.
func appendRecapWithinBudgetWithTaskAnchor(budget *RequestBudget, messages []openai.ChatCompletionMessage, tools []openai.Tool, dropped []openai.ChatCompletionMessage, cache *tokenCountCache, anchorIndex int) ([]openai.ChatCompletionMessage, int) {
	if len(dropped) == 0 || len(messages) == 0 {
		return messages, anchorIndex
	}
	if anchorIndex >= 0 && !validTaskAnchor(messages, anchorIndex, "") {
		return messages, anchorIndex
	}
	remaining := int(^uint(0) >> 1)
	for _, usage := range budget.tokenUsage(messages, tools, cache) {
		available := usage.ContextWindow - usage.TotalTokens
		if available < remaining {
			remaining = available
		}
	}
	if remaining <= 4 {
		return messages, anchorIndex
	}
	recap := buildTrimmedContextRecap(dropped, remaining-4)
	if recap == "" {
		return messages, anchorIndex
	}
	insertAt := 0
	for insertAt < len(messages) && messages[insertAt].Role == openai.ChatMessageRoleSystem {
		insertAt++
	}
	candidate := make([]openai.ChatCompletionMessage, 0, len(messages)+1)
	candidate = append(candidate, messages[:insertAt]...)
	candidate = append(candidate, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: recap})
	candidate = append(candidate, messages[insertAt:]...)
	if _, err := budget.validate(candidate, tools, cache); err != nil {
		return messages, anchorIndex
	}
	if anchorIndex >= insertAt {
		anchorIndex++
	}
	return candidate, anchorIndex
}

func appendRecapWithinWorkingSet(budget *RequestBudget, messages []openai.ChatCompletionMessage, currentUserText, systemPrompt string, tools []openai.Tool, dropped []openai.ChatCompletionMessage, cache *tokenCountCache) []openai.ChatCompletionMessage {
	trimmed, _ := appendRecapWithinWorkingSetWithTaskAnchor(budget, messages, currentUserText, protectedTaskMessageIndex(messages, currentUserText), systemPrompt, tools, dropped, cache)
	return trimmed
}

func appendRecapWithinWorkingSetWithTaskAnchor(budget *RequestBudget, messages []openai.ChatCompletionMessage, currentUserText string, anchorIndex int, systemPrompt string, tools []openai.Tool, dropped []openai.ChatCompletionMessage, cache *tokenCountCache) ([]openai.ChatCompletionMessage, int) {
	if budget == nil || len(dropped) == 0 || len(messages) == 0 {
		return messages, anchorIndex
	}
	if !validTaskAnchor(messages, anchorIndex, currentUserText) {
		return messages, anchorIndex
	}
	currentUser := anchorIndex
	remaining := budget.historyWorkingSetLimitForMessages(messages, currentUser, systemPrompt, tools, cache) - budget.maxCarriedHistoryTokens(messages, currentUser, cache)
	if remaining <= 4 {
		return messages, anchorIndex
	}
	if remaining > historySummaryMaxTokens {
		remaining = historySummaryMaxTokens
	}
	recap := buildTrimmedContextRecap(dropped, remaining-4)
	if recap == "" {
		return messages, anchorIndex
	}
	insertAt := 1
	if insertAt > len(messages) {
		insertAt = len(messages)
	}
	candidate := make([]openai.ChatCompletionMessage, 0, len(messages)+1)
	candidate = append(candidate, messages[:insertAt]...)
	candidate = append(candidate, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: recap})
	candidate = append(candidate, messages[insertAt:]...)
	newAnchorIndex := anchorIndex
	if insertAt <= anchorIndex {
		newAnchorIndex++
	}
	if !budget.historyWorkingSetFits(candidate, newAnchorIndex, systemPrompt, tools, cache) {
		return messages, anchorIndex
	}
	return candidate, newAnchorIndex
}

func requiredToolSchemasForState(s *agentLoopState) map[string]bool {
	required := map[string]bool{
		"discover_tools": true,
		"invoke_tool":    true,
	}
	if s == nil {
		return required
	}
	ff := buildToolFeatureFlags(s.runCfg, s.toolingPolicy)
	for _, name := range channelAdaptiveAlwaysInclude(s.runCfg, adaptiveHardAlwaysInclude(s.runCfg.Config), ff) {
		required[strings.TrimSpace(name)] = true
	}
	for _, name := range s.explicitTools {
		required[strings.TrimSpace(name)] = true
	}
	if s.runCfg.Config != nil {
		for _, name := range s.runCfg.Config.Agent.AdaptiveTools.AlwaysInclude {
			for _, expanded := range expandAdaptiveAlwaysIncludeAlias(name) {
				required[strings.TrimSpace(expanded)] = true
			}
		}
	}
	if s.currentToolRoute.valid() {
		required[strings.TrimSpace(s.currentToolRoute.ToolName)] = true
	}
	for _, call := range s.pendingTCs {
		required[strings.TrimSpace(call.Action)] = true
	}
	if len(s.req.Messages) > 0 {
		for _, call := range s.req.Messages[len(s.req.Messages)-1].ToolCalls {
			required[strings.TrimSpace(call.Function.Name)] = true
		}
	}
	delete(required, "")
	return required
}

func minimumBudgetToolSchemas(runCfg RunConfig, req openai.ChatCompletionRequest) []openai.Tool {
	if runCfg.Config == nil {
		return req.Tools
	}
	intent := strings.TrimSpace(runCfg.UserIntent)
	if intent == "" {
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == openai.ChatMessageRoleUser && !isTextModeToolResult(req.Messages[i]) {
				intent = strings.TrimSpace(messageText(req.Messages[i]))
				if intent != "" {
					break
				}
			}
		}
	}
	policy := buildToolingPolicy(runCfg.Config, intent)
	if !policy.UseNativeFunctions {
		return req.Tools
	}
	ff := buildToolFeatureFlags(runCfg, policy)
	var snapshot *nativeToolSchemaSnapshot
	if runCfg.NativeToolSchemas != nil {
		snapshot = newNativeToolSchemaSnapshot(runCfg.NativeToolSchemas)
	} else {
		snapshot = BuildNativeToolSchemaSnapshot(runCfg.Config.Directories.SkillsDir, runCfg.Manifest, ff, runCfg.Logger)
	}
	schemas := filterSchemasByAllowedTools(snapshot.FullSchemas(), runCfg.AllowedTools)
	if runCfg.VaultSecretPrompter == nil {
		schemas = filterSchemasByName(schemas, "request_vault_secret")
	}
	if shouldSuppressCoAgentTools(runCfg) {
		return nil
	}
	required := requiredToolSchemasForState(&agentLoopState{runCfg: runCfg, toolingPolicy: policy})
	requiredNames := make([]string, 0, len(required))
	for name := range required {
		requiredNames = append(requiredNames, name)
	}
	schemas = filterSchemasByAllowedTools(schemas, requiredNames)
	if policy.StructuredOutputsEnabled {
		schemas = strictSchemasForActiveTools(snapshot, schemas)
	}
	return schemas
}

// ValidateMinimumRequestBudget is a cheap public preflight for HTTP callers.
// It prevents opening an SSE response for a request that cannot possibly fit.
func ValidateMinimumRequestBudget(ctx context.Context, req openai.ChatCompletionRequest, runCfg RunConfig) error {
	req.Tools = minimumBudgetToolSchemas(runCfg, req)
	budget, err := newRequestBudget(ctx, runCfg.Config, runCfg.LLMClient, req, runCfg.Logger)
	if err != nil {
		return err
	}
	return budget.validateMinimum(req.Messages, req.Tools, newTokenCountCache(128))
}
