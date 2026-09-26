package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	openai "github.com/sashabaranov/go-openai"
)

type TaskRoutingDecision struct {
	TurnID         string `json:"turn_id"`
	Area           string `json:"area"`
	Source         string `json:"source"`
	Reason         string `json:"reason"`
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	ActualProvider string `json:"actual_provider"`
	ActualModel    string `json:"actual_model"`
	ElapsedMS      int64  `json:"elapsed_ms"`
	HelperUseful   bool   `json:"helper_useful"`
	SessionID      string `json:"session_id,omitempty"`
}

var taskRoutingSequence atomic.Uint64

type activeTaskRouteProvider interface {
	ActiveRoute() llm.ModelRoute
}

type taskRoutingCacheEntry struct {
	classification llm.TaskClassification
	expires        time.Time
}

type TaskRouterStats struct {
	Since              time.Time      `json:"since"`
	Decisions          map[string]int `json:"decisions"`
	Sources            map[string]int `json:"sources"`
	HelperAttempts     int            `json:"helper_attempts"`
	HelperFailures     int            `json:"helper_failures"`
	HelperInputTokens  int            `json:"helper_input_tokens"`
	HelperOutputTokens int            `json:"helper_output_tokens"`
	HelperUsageReports int            `json:"helper_usage_reports"`
	HelperUsageMissing int            `json:"helper_usage_missing"`
	Fallbacks          int            `json:"fallbacks"`
}

var taskRouterState = struct {
	sync.Mutex
	cache    map[[32]byte]taskRoutingCacheEntry
	previous map[[32]byte]taskRoutingCacheEntry
	cooldown map[string]time.Time
	inflight map[[32]byte]bool
	tokens   float64
	refilled time.Time
	stats    TaskRouterStats
}{cache: map[[32]byte]taskRoutingCacheEntry{}, previous: map[[32]byte]taskRoutingCacheEntry{}, cooldown: map[string]time.Time{}, inflight: map[[32]byte]bool{}, tokens: 2,
	stats: TaskRouterStats{Since: time.Now(), Decisions: map[string]int{}, Sources: map[string]int{}}}

func SnapshotTaskRouterStats() TaskRouterStats {
	taskRouterState.Lock()
	defer taskRouterState.Unlock()
	s := taskRouterState.stats
	s.Decisions = make(map[string]int)
	for k, v := range taskRouterState.stats.Decisions {
		s.Decisions[k] = v
	}
	s.Sources = make(map[string]int)
	for k, v := range taskRouterState.stats.Sources {
		s.Sources[k] = v
	}
	return s
}

func taskRouterEligible(run RunConfig) bool {
	if run.TaskRoutingMode == "off" || run.TaskRoutingMode == "pinned" || run.IsMission || run.IsMaintenance || run.IsCoAgent || run.PreparedPrompt != nil || run.ExecutionHooks != nil || run.Checkpoint != nil {
		return false
	}
	if run.TaskRoutingMode == "auto" {
		return true
	}
	switch run.MessageSource {
	case "web_chat", "telegram", "discord", "rocketchat", "sms", "desktop", "desktop_chat", "virtual_desktop_chat", "agodesk", "agodesk_chat", "meshcore":
		return true
	}
	return false
}

func taskRoutingIntent(req openai.ChatCompletionRequest, run RunConfig) string {
	if run.UserIntent != "" {
		return run.UserIntent
	}
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == openai.ChatMessageRoleUser && !isTextModeToolResult(req.Messages[i]) {
			return messageText(req.Messages[i])
		}
	}
	return ""
}

func taskRoutingTarget(cfg *config.Config, area string) (*config.ProviderEntry, string) {
	t := cfg.LLMRouter.Areas[area]
	if strings.TrimSpace(t.Provider) == "" {
		return nil, "unassigned"
	}
	provider := cfg.FindProvider(t.Provider)
	if provider == nil {
		return nil, "provider_missing"
	}
	p := *provider
	if ok, _ := config.SpeechLabChatProviderEligibility(&p); !ok {
		return nil, "provider_ineligible"
	}
	if t.Model != "" && t.Model != p.Model {
		p.Model = t.Model
		p.ContextWindow, p.MaxOutputTokens = 0, 0
		p.Capabilities = config.ProviderCapabilities{}
	}
	if p.Model == "" {
		return nil, "model_missing"
	}
	if p.APIKey == "" && p.AuthType != "oauth2" {
		switch p.Type {
		case "ollama", "llamacpp", "lmstudio", "custom", "copilot", "manifest", "omniroute":
		default:
			return nil, "credentials_missing"
		}
	}
	if p.ID == config.LocalLLMProviderID && (!cfg.LocalLLM.Enabled || (cfg.LocalLLM.ModelFamily != "qwen" && cfg.LocalLLM.ModelFamily != "")) {
		return nil, "local_model_unqualified"
	}
	if p.Model == cfg.LLM.Model && (p.ID == cfg.LLM.Provider ||
		(strings.EqualFold(p.Type, cfg.LLM.ProviderType) && strings.TrimRight(p.BaseURL, "/") == strings.TrimRight(cfg.LLM.BaseURL, "/") && p.APIKey == cfg.LLM.APIKey && p.AccountID == cfg.LLM.AccountID)) {
		return nil, "same_as_default"
	}
	return &p, "selected"
}

func taskRoutingHelperUseful(cfg *config.Config) bool {
	for _, area := range config.LLMRouterAreas {
		if p, _ := taskRoutingTarget(cfg, area); p != nil {
			return true
		}
	}
	return false
}

func taskRoutingCacheKeys(cfg *config.Config, req openai.ChatCompletionRequest, run RunConfig, intent string) ([32]byte, [32]byte) {
	// Full effective config identity prevents stale credential, limit and model reuse.
	schemas, _ := json.Marshal(run.NativeToolSchemas)
	identity := fmt.Sprintf("task-rules-v1|%q|%q|%q|%x|%x", run.SessionID, run.MessageSource, run.TaskRoutingMode, llm.TaskConfigFingerprint(cfg), sha256.Sum256(schemas))
	for _, message := range req.Messages {
		for _, part := range message.MultiContent {
			identity += "|" + string(part.Type)
		}
	}
	if run.TaskRoutingImageInput {
		identity += "|image"
	}
	return sha256.Sum256([]byte(identity + "|" + strings.TrimSpace(intent))), sha256.Sum256([]byte(identity))
}

func taskRoutingCached(key, sessionKey [32]byte, continuation bool) (llm.TaskClassification, bool) {
	taskRouterState.Lock()
	defer taskRouterState.Unlock()
	now := time.Now()
	for _, cache := range []map[[32]byte]taskRoutingCacheEntry{taskRouterState.cache, taskRouterState.previous} {
		for k, v := range cache {
			if now.After(v.expires) {
				delete(cache, k)
			}
		}
	}
	entry, ok := taskRouterState.cache[key]
	if continuation {
		entry, ok = taskRouterState.previous[sessionKey]
	}
	return entry.classification, ok
}

func taskRoutingRemember(key, sessionKey [32]byte, c llm.TaskClassification) {
	taskRouterState.Lock()
	defer taskRouterState.Unlock()
	entry := taskRoutingCacheEntry{c, time.Now().Add(10 * time.Minute)}
	for _, pair := range []struct {
		cache map[[32]byte]taskRoutingCacheEntry
		key   [32]byte
	}{{taskRouterState.cache, key}, {taskRouterState.previous, sessionKey}} {
		if len(pair.cache) >= 256 {
			var oldest [32]byte
			var expiry time.Time
			for k, v := range pair.cache {
				if expiry.IsZero() || v.expires.Before(expiry) {
					oldest, expiry = k, v.expires
				}
			}
			delete(pair.cache, oldest)
		}
		pair.cache[pair.key] = entry
	}
}

func reserveTaskRoutingHelper(key [32]byte, session string, quota int) (func(), bool) {
	taskRouterState.Lock()
	defer taskRouterState.Unlock()
	now := time.Now()
	for k, expiry := range taskRouterState.cooldown {
		if now.After(expiry) {
			delete(taskRouterState.cooldown, k)
		}
	}
	if quota <= 0 || taskRouterState.inflight[key] || now.Before(taskRouterState.cooldown[session]) || len(taskRouterState.cooldown) >= 256 {
		return nil, false
	}
	if taskRouterState.refilled.IsZero() {
		taskRouterState.refilled = now
	}
	taskRouterState.tokens = min(2, taskRouterState.tokens+now.Sub(taskRouterState.refilled).Hours()*float64(quota))
	taskRouterState.refilled = now
	if taskRouterState.tokens < 1 {
		return nil, false
	}
	taskRouterState.tokens--
	taskRouterState.cooldown[session] = now.Add(time.Minute)
	taskRouterState.inflight[key] = true
	return func() { taskRouterState.Lock(); delete(taskRouterState.inflight, key); taskRouterState.Unlock() }, true
}

// PrepareTaskRouting is shared by HTTP preflight and the loop. It never mutates
// a published config or changes an already prepared decision.
func PrepareTaskRouting(ctx context.Context, req *openai.ChatCompletionRequest, run *RunConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if run.TaskRouting != nil || run.Config == nil || !run.Config.LLMRouter.Enabled || !taskRouterEligible(*run) {
		return nil
	}
	start := time.Now()
	cfg := run.Config
	d := &TaskRoutingDecision{TurnID: fmt.Sprintf("%d-%d", start.UnixMilli(), taskRoutingSequence.Add(1)), Source: "default", Reason: "unassigned", Provider: cfg.LLM.Provider, Model: cfg.LLM.Model, SessionID: run.SessionID}
	run.TaskRouting = d
	defer func() {
		d.ElapsedMS = time.Since(start).Milliseconds()
		d.ActualProvider, d.ActualModel = d.Provider, d.Model
		if !run.TaskRoutingPreview {
			taskRouterState.Lock()
			taskRouterState.stats.Decisions[d.Area]++
			taskRouterState.stats.Sources[d.Source]++
			if d.Reason != "selected" && d.Reason != "unassigned" && d.Reason != "same_as_default" {
				taskRouterState.stats.Fallbacks++
			}
			taskRouterState.Unlock()
		}
	}()
	if !cfg.LLMRouter.HasAssignments() {
		return nil
	}
	intent := taskRoutingIntent(*req, *run)
	key, sessionKey := taskRoutingCacheKeys(cfg, *req, *run, intent)
	c, hit := taskRoutingCached(key, sessionKey, llm.IsTaskContinuation(intent))
	if hit {
		d.Source = "cache"
	} else {
		c = llm.ClassifyTask(intent)
		d.Source = "rules"
	}
	if c.Uncertain {
		d.Source, d.Reason = "default", "uncertain"
		d.HelperUseful = taskRoutingHelperUseful(cfg)
		if d.HelperUseful && cfg.LLMRouter.HelperFallback && llm.IsHelperLLMAvailable(cfg) && !run.BudgetTracker.IsBlocked("routing") {
			if release, ok := reserveTaskRoutingHelper(key, run.SessionID, cfg.LLMRouter.HelperMaxCallsPerHour); ok {
				defer release()
				if result, err := classifyTaskWithHelper(ctx, *run, intent); err == nil {
					c = result
					d.Source = "helper"
				} else {
					d.Reason = "helper_unavailable"
				}
			} else {
				d.Reason = "helper_limited"
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	d.Area = c.Area()
	if d.Area == "" {
		if !run.TaskRoutingPreview {
			taskRouterState.Lock()
			delete(taskRouterState.previous, sessionKey)
			taskRouterState.Unlock()
		}
		return nil
	}
	if !run.TaskRoutingPreview {
		taskRoutingRemember(key, sessionKey, c)
	}
	p, reason := taskRoutingTarget(cfg, d.Area)
	d.Reason = reason
	if p == nil {
		return nil
	}
	view := llm.TaskProviderConfig(cfg, *p)
	caps := llm.ResolveProviderCapabilities(*p, llm.CapabilityFallback{})
	if run.TaskRoutingImageInput && (!caps.Known || !caps.Multimodal) {
		d.Reason = "capability_mismatch"
		return nil
	}
	if cfg.LLM.UseNativeFunctions && (!caps.Known || !caps.ToolCalling) {
		d.Reason = "capability_mismatch"
		return nil
	}
	client := llm.NewTaskRouteClient(cfg, run.LLMClient, p)
	probe := *req
	probe.Model = p.Model
	if run.TaskRoutingImageInput {
		// Include pending uploads in both capability filtering and token preflight.
		// Actual bytes are prepared by the owning ingress after route acceptance.
		probe.Messages = append([]openai.ChatCompletionMessage{}, probe.Messages...)
		for i := len(probe.Messages) - 1; i >= 0; i-- {
			if probe.Messages[i].Role != "user" {
				continue
			}
			message := &probe.Messages[i]
			alreadyPrepared := false
			for _, part := range message.MultiContent {
				if part.Type == openai.ChatMessagePartTypeImageURL {
					alreadyPrepared = true
					break
				}
			}
			if alreadyPrepared {
				break
			}
			parts := append([]openai.ChatMessagePart{}, message.MultiContent...)
			if message.Content != "" {
				parts = append(parts, openai.ChatMessagePart{Type: openai.ChatMessagePartTypeText, Text: message.Content})
				message.Content = ""
			}
			message.MultiContent = append(parts, openai.ChatMessagePart{Type: openai.ChatMessagePartTypeImageURL, ImageURL: &openai.ChatMessageImageURL{URL: "data:image/jpeg;base64,"}})
			break
		}
	}
	probe.Tools = minimumBudgetToolSchemas(*run, probe)
	routes := client.CandidateRoutes(probe)
	if len(routes) == 0 || routes[0].ProviderID != p.ID || routes[0].Model != p.Model {
		d.Reason = "capability_mismatch"
		return nil
	}
	prepared := *run
	prepared.Config = view
	prepared.LLMClient = client
	budget, budgetErr := newRequestBudgetWithLimits(prepared.Config, prepared.LLMClient, probe, llm.ResolveModelLimitsCached)
	if budgetErr == nil {
		budgetErr = budget.validateMinimum(probe.Messages, probe.Tools, newTokenCountCache(128))
	}
	if budgetErr != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		d.Reason = "context_limit"
		return nil
	}
	run.Config, run.LLMClient = view, client
	req.Model = p.Model
	d.Provider, d.Model = p.ID, p.Model
	return nil
}

func publishTaskRouting(run RunConfig, broker FeedbackBroker) {
	if run.TaskRouting == nil || broker == nil {
		return
	}
	d := *run.TaskRouting
	if client, ok := run.LLMClient.(activeTaskRouteProvider); ok {
		route := client.ActiveRoute()
		d.ActualProvider, d.ActualModel = route.ProviderID, route.Model
		if d.ActualProvider != d.Provider || d.ActualModel != d.Model {
			d.Reason = "provider_failover"
		}
	}
	if typed, ok := broker.(TypedFeedbackBroker); ok {
		typed.SendTyped("llm_route", d)
		return
	}
	// Ordinary messaging channels must not receive diagnostic prose.
	if run.MessageSource == "web_chat" {
		data, _ := json.Marshal(d)
		broker.Send("llm_route", string(data))
	}
}

func finishTaskRouting(run RunConfig, broker FeedbackBroker) {
	publishTaskRouting(run, broker)
	if run.TaskRouting == nil || run.TaskRoutingPreview {
		return
	}
	if client, ok := run.LLMClient.(activeTaskRouteProvider); ok {
		actual := client.ActiveRoute()
		if actual.ProviderID != run.TaskRouting.Provider || actual.Model != run.TaskRouting.Model {
			taskRouterState.Lock()
			taskRouterState.stats.Fallbacks++
			taskRouterState.Unlock()
		}
	}
}
