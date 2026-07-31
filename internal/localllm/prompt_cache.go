package localllm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aurago/internal/config"
)

const (
	maxLocalRequestBytes             = 4 << 20
	maxPromptSeedBytes               = 256 << 10
	promptCacheFileName              = "prompt-cache-decision.json"
	promptCacheDecisionSchemaVersion = 3
	promptCacheQualificationTimeout  = 90 * time.Second
	promptCacheRequestReserve        = 30 * time.Second
	promptCacheQualificationQuiet    = 5 * time.Second
	promptCachePersistInterval       = time.Minute
	promptCacheProbeQuery            = "aurago_prompt_cache_probe"
	promptCacheProbeText             = `Call discover_tools exactly once with {"operation":"search","query":"aurago_prompt_cache_probe"}. Do not answer with normal text.`
	promptCacheRenderBoundary        = "AURAGO_INTERNAL_CACHE_BOUNDARY_4B5DCC569C8E"
)

type promptCacheSeed struct {
	Generation         uint64
	Fingerprint        string
	ToolsetFingerprint string
	ToolCount          int
	SystemPrefix       string
	ApplyTemplateBody  []byte
}

type promptCacheDecisionEntry struct {
	SchemaVersion     int     `json:"schema_version"`
	Fingerprint       string  `json:"fingerprint"`
	Accepted          bool    `json:"accepted"`
	State             string  `json:"state"`
	Reason            string  `json:"reason,omitempty"`
	ReuseThreshold    float64 `json:"reuse_threshold"`
	TTFTGainThreshold float64 `json:"ttft_gain_threshold"`
	WarmupDurationMS  float64 `json:"warmup_duration_ms,omitempty"`
	CachedTokens      uint64  `json:"cached_tokens,omitempty"`
	ProcessedTokens   uint64  `json:"processed_tokens,omitempty"`
	HitRate           float64 `json:"hit_rate,omitempty"`
	ColdTTFTMS        float64 `json:"cold_ttft_ms,omitempty"`
	WarmTTFTMS        float64 `json:"warm_ttft_ms,omitempty"`
	Generation        uint64  `json:"-"`
	Terminal          bool    `json:"-"`
}

type promptCacheObservationPlan struct {
	Generation      uint64
	SeedFingerprint string
	CacheEnabled    bool
}

type promptCacheProbeResult struct {
	ToolCall     string
	TTFT         time.Duration
	CachedTokens uint64
	PromptTokens uint64
	Complete     bool
}

type promptCacheQualificationResult struct {
	SeedTokens      uint64
	CachedTokens    uint64
	ProcessedTokens uint64
	ColdTTFT        time.Duration
	WarmTTFT        time.Duration
}

type promptCacheWarmupResult struct {
	SeedTokens      uint64
	CachedTokens    uint64
	ProcessedTokens uint64
}

func (m *Manager) acquirePromptSlot(ctx context.Context) (func(), error) {
	m.mu.Lock()
	slot := m.promptSlot
	lifecycle := m.lifecycleCtx
	shuttingDown := m.shuttingDown
	m.mu.Unlock()
	if shuttingDown {
		return nil, &UnavailableError{Code: "local_llm_shutting_down"}
	}
	select {
	case slot <- struct{}{}:
		return func() { <-slot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-lifecycle.Done():
		return nil, &UnavailableError{Code: "local_llm_shutting_down", Err: lifecycle.Err()}
	}
}

func preparePromptCacheRequest(req *http.Request) (*http.Request, *promptCacheSeed, bool, string, error) {
	if req == nil || req.Method != http.MethodPost ||
		!strings.HasSuffix(strings.TrimSuffix(req.URL.Path, "/"), "/chat/completions") {
		return req, nil, false, "", nil
	}
	raw, err := io.ReadAll(io.LimitReader(req.Body, maxLocalRequestBytes+1))
	if err != nil {
		return nil, nil, false, "", fmt.Errorf("local_request_read_failed: %w", err)
	}
	if len(raw) > maxLocalRequestBytes {
		return nil, nil, false, "", fmt.Errorf("local_request_too_large")
	}
	_ = req.Body.Close()

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, nil, false, "", fmt.Errorf("local_request_invalid")
	}
	stream := rawJSONBool(object["stream"])
	object["cache_prompt"] = json.RawMessage("false")
	if stream {
		var options map[string]json.RawMessage
		if len(object["stream_options"]) > 0 {
			_ = json.Unmarshal(object["stream_options"], &options)
		}
		if options == nil {
			options = make(map[string]json.RawMessage)
		}
		options["include_usage"] = json.RawMessage("true")
		encoded, _ := json.Marshal(options)
		object["stream_options"] = encoded
	}
	if len(object["tools"]) > 0 {
		var tools []json.RawMessage
		if json.Unmarshal(object["tools"], &tools) == nil {
			sort.SliceStable(tools, func(i, j int) bool {
				return toolNameFromRaw(tools[i]) < toolNameFromRaw(tools[j])
			})
			if encodedTools, encodeErr := json.Marshal(tools); encodeErr == nil {
				object["tools"] = encodedTools
			}
		}
	}
	if err := normalizeLocalSystemMessages(object); err != nil {
		return nil, nil, false, "", err
	}

	seed, err := seedFromChatRequest(object)
	seedError := ""
	if err != nil {
		seed = nil
		seedError = errorCode(err)
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		return nil, nil, false, "", fmt.Errorf("local_request_encode_failed")
	}
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Body = io.NopCloser(bytes.NewReader(encoded))
	clone.ContentLength = int64(len(encoded))
	clone.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(encoded)), nil
	}
	return clone, seed, stream, seedError, nil
}

// normalizeLocalSystemMessages preserves the meaning of runtime recovery and
// compaction notices while satisfying Qwen's embedded template contract:
// exactly one system message, at the beginning of the conversation. Dynamic
// notices are appended after the original message, so seedFromChatRequest still
// stops at # TURN CONTEXT and never includes them in the reusable prefix.
func normalizeLocalSystemMessages(object map[string]json.RawMessage) error {
	var messages []map[string]json.RawMessage
	if len(object["messages"]) == 0 || json.Unmarshal(object["messages"], &messages) != nil {
		return fmt.Errorf("local_request_invalid")
	}
	var primaryMessage map[string]json.RawMessage
	systemContents := make([]string, 0, 1)
	nonSystem := make([]map[string]json.RawMessage, 0, len(messages))
	for _, message := range messages {
		var role string
		if json.Unmarshal(message["role"], &role) != nil {
			return fmt.Errorf("local_request_invalid")
		}
		if role != "system" {
			nonSystem = append(nonSystem, message)
			continue
		}
		if primaryMessage == nil {
			primaryMessage = message
		}
		content, err := localMessageText(message["content"])
		if err != nil {
			return err
		}
		systemContents = append(systemContents, content)
	}
	if primaryMessage == nil {
		return nil
	}
	primary := systemContents[0]
	if len(systemContents) > 1 {
		primary += "\n\n# RUNTIME SYSTEM CONTEXT\n\n" +
			strings.Join(systemContents[1:], "\n\n")
	}
	encodedContent, err := json.Marshal(primary)
	if err != nil {
		return fmt.Errorf("local_request_encode_failed")
	}
	encodedRole, _ := json.Marshal("system")
	primaryMessage["role"] = encodedRole
	primaryMessage["content"] = encodedContent
	normalized := make([]map[string]json.RawMessage, 0, len(nonSystem)+1)
	normalized = append(normalized, primaryMessage)
	normalized = append(normalized, nonSystem...)
	encodedMessages, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("local_request_encode_failed")
	}
	object["messages"] = encodedMessages
	return nil
}

func localMessageText(raw json.RawMessage) (string, error) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return "", fmt.Errorf("local_system_content_unsupported")
	}
	var builder strings.Builder
	for _, part := range parts {
		if part.Type != "text" {
			return "", fmt.Errorf("local_system_content_unsupported")
		}
		builder.WriteString(part.Text)
	}
	return builder.String(), nil
}

func enablePromptCacheRequest(req *http.Request) (*http.Request, error) {
	reader := req.Body
	if req.GetBody != nil {
		var err error
		reader, err = req.GetBody()
		if err != nil {
			return nil, fmt.Errorf("local_request_read_failed")
		}
		defer reader.Close()
	}
	raw, err := io.ReadAll(io.LimitReader(reader, maxLocalRequestBytes+1))
	if err != nil || len(raw) > maxLocalRequestBytes {
		return nil, fmt.Errorf("local_request_read_failed")
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil {
		return nil, fmt.Errorf("local_request_invalid")
	}
	object["cache_prompt"] = json.RawMessage("true")
	encoded, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Body = io.NopCloser(bytes.NewReader(encoded))
	clone.ContentLength = int64(len(encoded))
	clone.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(encoded)), nil }
	return clone, nil
}

func seedFromChatRequest(object map[string]json.RawMessage) (*promptCacheSeed, error) {
	var messages []map[string]json.RawMessage
	if json.Unmarshal(object["messages"], &messages) != nil {
		return nil, fmt.Errorf("prompt_cache_messages_invalid")
	}
	var prefix string
	for _, message := range messages {
		var role string
		_ = json.Unmarshal(message["role"], &role)
		if role != "system" {
			continue
		}
		var content string
		if json.Unmarshal(message["content"], &content) != nil {
			return nil, fmt.Errorf("prompt_cache_system_content_unsupported")
		}
		marker := strings.Index(content, "# TURN CONTEXT")
		if marker < 0 {
			return nil, fmt.Errorf("prompt_cache_turn_marker_missing")
		}
		prefix = content[:marker]
		break
	}
	if strings.TrimSpace(prefix) == "" {
		return nil, fmt.Errorf("prompt_cache_system_prefix_missing")
	}

	var tools []json.RawMessage
	if len(object["tools"]) > 0 && json.Unmarshal(object["tools"], &tools) != nil {
		return nil, fmt.Errorf("prompt_cache_tools_invalid")
	}
	sort.SliceStable(tools, func(i, j int) bool { return toolNameFromRaw(tools[i]) < toolNameFromRaw(tools[j]) })
	toolBytes, _ := json.Marshal(tools)
	toolHash := sha256.Sum256(toolBytes)

	seedRequest := map[string]any{
		"messages": []map[string]string{
			{"role": "system", "content": prefix},
			// Qwen's embedded template rejects conversations without a user
			// turn. This fixed probe satisfies the template without copying any
			// live user data into the seed. The reusable prefix still ends at
			// the static system text, before this synthetic turn.
			{"role": "user", "content": promptCacheProbeText},
		},
		"tools": tools,
	}
	if kwargs := object["chat_template_kwargs"]; len(kwargs) > 0 {
		seedRequest["chat_template_kwargs"] = kwargs
	}
	if model := object["model"]; len(model) > 0 {
		seedRequest["model"] = model
	}
	if toolChoice := object["tool_choice"]; len(toolChoice) > 0 {
		seedRequest["tool_choice"] = toolChoice
	}
	if parallel := object["parallel_tool_calls"]; len(parallel) > 0 {
		seedRequest["parallel_tool_calls"] = parallel
	}
	body, err := json.Marshal(seedRequest)
	if err != nil || len(body) > maxPromptSeedBytes {
		return nil, fmt.Errorf("prompt_cache_seed_too_large")
	}
	sum := sha256.Sum256(body)
	return &promptCacheSeed{
		Fingerprint: hex.EncodeToString(sum[:]), ToolsetFingerprint: hex.EncodeToString(toolHash[:]),
		ToolCount: len(tools), SystemPrefix: prefix, ApplyTemplateBody: body,
	}, nil
}

func toolNameFromRaw(raw json.RawMessage) string {
	var tool struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	_ = json.Unmarshal(raw, &tool)
	return tool.Function.Name
}

func rawJSONBool(raw json.RawMessage) bool {
	var value bool
	return json.Unmarshal(raw, &value) == nil && value
}

func (m *Manager) rememberPromptSeed(seed *promptCacheSeed) {
	if seed == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	seed.Generation = m.generation
	if m.promptSeed != nil && m.promptSeed.Generation == seed.Generation &&
		m.promptSeed.Fingerprint == seed.Fingerprint {
		return
	}
	copySeed := *seed
	copySeed.ApplyTemplateBody = append([]byte(nil), seed.ApplyTemplateBody...)
	m.promptSeed = &copySeed
	m.promptCacheQualified = false
	perf := performanceProfileFor(m.profile)
	m.status.PerformanceProfile = perf.Name
	m.status.PromptCache = PromptCacheStatus{
		State: "cold", SeedFingerprint: seed.Fingerprint,
		ToolsetFingerprint: seed.ToolsetFingerprint, StableToolCount: seed.ToolCount,
		CacheRAMMiB: perf.CacheRAMMiB, CheckpointProfile: "32x2048",
	}
	if decision, ok := m.loadPromptCacheDecisionLocked(); ok {
		status := &m.status.PromptCache
		status.WarmupDurationMS = decision.WarmupDurationMS
		status.CachedTokens = decision.CachedTokens
		status.ProcessedTokens = decision.ProcessedTokens
		status.HitRate = decision.HitRate
		status.ColdTTFTMS = decision.ColdTTFTMS
		status.WarmTTFTMS = decision.WarmTTFTMS
		status.DecisionPersisted = true
		if decision.Accepted && decision.State == "warm" {
			m.promptCacheQualified = true
			status.Qualified = true
			status.State = "cold"
		} else if decision.State == "rejected" {
			status.State = "rejected"
			status.ErrorCode = decision.Reason
			if status.ErrorCode == "" {
				status.ErrorCode = "prompt_cache_profile_rejected"
			}
		}
	}
}

func (m *Manager) markPromptCacheDegraded(code string) {
	if code == "" {
		code = "prompt_cache_request_failed"
	}
	m.mu.Lock()
	m.promptSeed = nil
	m.promptCacheQualified = false
	m.status.PromptCache.State = "degraded"
	m.status.PromptCache.Qualified = false
	m.status.PromptCache.DecisionPersisted = false
	m.status.PromptCache.ErrorCode = code
	m.mu.Unlock()
}

func (m *Manager) promptCacheReady(seed *promptCacheSeed) bool {
	if seed == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.promptSeed != nil &&
		m.promptSeed.Generation == m.generation &&
		m.promptSeed.Fingerprint == seed.Fingerprint &&
		m.promptCacheQualified &&
		m.status.PromptCache.Qualified &&
		m.status.PromptCache.State == "warm"
}

func (m *Manager) promptCacheNeedsSynchronousWarm(seed *promptCacheSeed) bool {
	if seed == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.promptSeed != nil &&
		m.promptSeed.Generation == m.generation &&
		m.promptSeed.Fingerprint == seed.Fingerprint &&
		m.promptCacheQualified &&
		m.status.PromptCache.Qualified &&
		m.status.PromptCache.State == "cold"
}

func (m *Manager) cancelPromptCacheWarmForRequest() {
	m.mu.Lock()
	m.promptWarmSequence++
	cancel := m.promptWarmCancel
	m.promptWarmCancel = nil
	if cancel != nil && m.status.PromptCache.State == "warming" &&
		!m.promptCacheQualified {
		m.status.PromptCache.State = "cold"
		m.status.PromptCache.ErrorCode = ""
	}
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// schedulePromptCacheQualification waits for a short quiet period after a
// completed real response. A subsequent model turn cancels it before taking the
// single llama.cpp slot, so qualification can never consume most of the user's
// request deadline again.
func (m *Manager) schedulePromptCacheQualification(plan runtimePlan, seed *promptCacheSeed) {
	m.schedulePromptCacheQualificationAfter(plan, seed, promptCacheQualificationQuiet)
}

func (m *Manager) schedulePromptCacheQualificationAfter(plan runtimePlan, seed *promptCacheSeed, quiet time.Duration) {
	if seed == nil {
		return
	}
	m.mu.Lock()
	if m.shuttingDown || plan.Generation != m.generation ||
		m.promptSeed == nil || m.promptSeed.Fingerprint != seed.Fingerprint ||
		m.promptCacheQualified ||
		m.status.PromptCache.State == "rejected" ||
		m.status.PromptCache.State == "degraded" {
		m.mu.Unlock()
		return
	}
	if m.promptWarmCancel != nil {
		m.promptWarmCancel()
	}
	m.promptWarmSequence++
	sequence := m.promptWarmSequence
	lifecycle := m.lifecycleCtx
	if lifecycle == nil {
		lifecycle = context.Background()
	}
	ctx, cancel := context.WithCancel(lifecycle)
	m.promptWarmCancel = cancel
	m.promptWarmWG.Add(1)
	m.mu.Unlock()

	go func() {
		defer m.promptWarmWG.Done()
		defer func() {
			m.mu.Lock()
			if sequence == m.promptWarmSequence {
				m.promptWarmCancel = nil
				if ctx.Err() != nil && m.status.PromptCache.State == "warming" &&
					!m.promptCacheQualified {
					m.status.PromptCache.State = "cold"
					m.status.PromptCache.ErrorCode = ""
				}
			}
			m.mu.Unlock()
		}()
		timer := time.NewTimer(quiet)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return
		}

		releaseSlot, err := m.acquirePromptSlot(ctx)
		if err != nil {
			return
		}
		defer releaseSlot()
		if err := m.acquireRequest(); err != nil {
			return
		}
		defer m.release()
		key, err := m.runtimeKey()
		if err != nil {
			m.markPromptCacheDegraded("runtime_key_unavailable")
			return
		}
		m.ensurePromptCacheWarm(ctx, plan, key)
	}()
}

func (m *Manager) ensurePromptCacheWarm(ctx context.Context, plan runtimePlan, key string) {
	m.mu.Lock()
	if plan.Generation != m.generation || m.promptSeed == nil ||
		m.promptSeed.Generation != plan.Generation {
		m.mu.Unlock()
		return
	}
	if m.status.PromptCache.State == "rejected" || m.status.PromptCache.State == "degraded" {
		m.mu.Unlock()
		return
	}
	seed := *m.promptSeed
	seed.ApplyTemplateBody = append([]byte(nil), m.promptSeed.ApplyTemplateBody...)
	qualified := m.promptCacheQualified
	if qualified && m.status.PromptCache.State == "warm" &&
		m.status.PromptCache.SeedFingerprint == seed.Fingerprint {
		m.mu.Unlock()
		return
	}
	m.status.PromptCache.State = "warming"
	m.status.PromptCache.ErrorCode = ""
	m.mu.Unlock()

	started := time.Now()
	warmCtx, cancel, err := promptCacheQualificationContext(ctx)
	if err != nil {
		m.publishPromptCacheFailure(plan, seed, "prompt_cache_qualification_timeout", false, 0)
		return
	}
	defer cancel()
	var result promptCacheQualificationResult
	if qualified {
		var warmup promptCacheWarmupResult
		warmup, err = m.warmPromptPrefix(warmCtx, plan, key, seed)
		result.SeedTokens = warmup.SeedTokens
		result.CachedTokens = warmup.CachedTokens
		result.ProcessedTokens = warmup.ProcessedTokens
	} else {
		result, err = m.qualifyPromptCache(warmCtx, plan, key, seed)
	}
	duration := float64(time.Since(started).Microseconds()) / 1000
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return
		}
		code := errorCode(err)
		permanent := isPermanentPromptCacheRejection(code)
		m.publishPromptCacheFailure(plan, seed, code, permanent, duration)
		return
	}
	m.mu.Lock()
	if plan.Generation != m.generation || m.promptSeed == nil ||
		m.promptSeed.Fingerprint != seed.Fingerprint {
		m.mu.Unlock()
		return
	}
	m.promptCacheQualified = true
	m.status.PromptCache.State = "warm"
	m.status.PromptCache.Qualified = true
	m.status.PromptCache.DecisionPersisted = qualified && m.status.PromptCache.DecisionPersisted
	m.status.PromptCache.SeedTokens = result.SeedTokens
	m.status.PromptCache.WarmupDurationMS = duration
	if result.ColdTTFT > 0 {
		m.status.PromptCache.ColdTTFTMS = float64(result.ColdTTFT.Microseconds()) / 1000
	}
	if result.WarmTTFT > 0 {
		m.status.PromptCache.WarmTTFTMS = float64(result.WarmTTFT.Microseconds()) / 1000
	}
	if result.CachedTokens > 0 || result.ProcessedTokens > 0 {
		m.status.PromptCache.CachedTokens = result.CachedTokens
		m.status.PromptCache.ProcessedTokens = result.ProcessedTokens
		total := result.CachedTokens + result.ProcessedTokens
		if total > 0 {
			m.status.PromptCache.HitRate = float64(result.CachedTokens) / float64(total)
		}
	}
	m.status.PromptCache.ErrorCode = ""
	entry, ok := m.promptCacheDecisionEntryLocked(true, true)
	m.mu.Unlock()
	if ok {
		m.queuePromptCacheDecision(entry, true)
	}
}

func promptCacheQualificationContext(parent context.Context) (context.Context, context.CancelFunc, error) {
	if parent == nil {
		parent = context.Background()
	}
	now := time.Now()
	deadline := now.Add(promptCacheQualificationTimeout)
	if parentDeadline, ok := parent.Deadline(); ok {
		reserved := parentDeadline.Add(-promptCacheRequestReserve)
		if !reserved.After(now) {
			return nil, func() {}, fmt.Errorf("prompt_cache_qualification_timeout")
		}
		if reserved.Before(deadline) {
			deadline = reserved
		}
	}
	ctx, cancel := context.WithDeadline(parent, deadline)
	return ctx, cancel, nil
}

func (m *Manager) publishPromptCacheFailure(plan runtimePlan, seed promptCacheSeed, code string, permanent bool, duration float64) {
	if code == "" {
		code = "prompt_cache_warmup_failed"
	}
	m.mu.Lock()
	if plan.Generation != m.generation || m.promptSeed == nil ||
		m.promptSeed.Fingerprint != seed.Fingerprint {
		m.mu.Unlock()
		return
	}
	m.promptCacheQualified = false
	m.status.PromptCache.Qualified = false
	m.status.PromptCache.DecisionPersisted = false
	m.status.PromptCache.WarmupDurationMS = duration
	m.status.PromptCache.ErrorCode = code
	if permanent {
		m.status.PromptCache.State = "rejected"
	} else {
		m.status.PromptCache.State = "degraded"
	}
	entry, ok := m.promptCacheDecisionEntryLocked(false, permanent)
	m.mu.Unlock()
	if permanent && ok {
		m.queuePromptCacheDecision(entry, true)
	}
}

func isPermanentPromptCacheRejection(code string) bool {
	switch code {
	case "prompt_cache_probe_tool_unavailable",
		"prompt_cache_semantic_mismatch",
		"prompt_cache_reuse_below_80_percent",
		"prompt_cache_ttft_gain_below_70_percent":
		return true
	default:
		return false
	}
}

func promptCacheTemplateBody(seed promptCacheSeed) ([]byte, error) {
	if seed.SystemPrefix == "" || strings.Contains(seed.SystemPrefix, promptCacheRenderBoundary) {
		return nil, fmt.Errorf("prompt_cache_seed_invalid")
	}
	var request map[string]json.RawMessage
	if json.Unmarshal(seed.ApplyTemplateBody, &request) != nil {
		return nil, fmt.Errorf("prompt_cache_seed_invalid")
	}
	var messages []map[string]json.RawMessage
	if json.Unmarshal(request["messages"], &messages) != nil {
		return nil, fmt.Errorf("prompt_cache_seed_invalid")
	}
	found := false
	for index := range messages {
		var role string
		var content string
		if json.Unmarshal(messages[index]["role"], &role) != nil || role != "system" {
			continue
		}
		if json.Unmarshal(messages[index]["content"], &content) != nil || content != seed.SystemPrefix {
			return nil, fmt.Errorf("prompt_cache_seed_invalid")
		}
		encoded, err := json.Marshal(content + promptCacheRenderBoundary)
		if err != nil {
			return nil, fmt.Errorf("prompt_cache_seed_invalid")
		}
		messages[index]["content"] = encoded
		found = true
		break
	}
	if !found {
		return nil, fmt.Errorf("prompt_cache_seed_invalid")
	}
	encodedMessages, err := json.Marshal(messages)
	if err != nil {
		return nil, fmt.Errorf("prompt_cache_seed_invalid")
	}
	request["messages"] = encodedMessages
	body, err := json.Marshal(request)
	if err != nil || len(body) > maxPromptSeedBytes {
		return nil, fmt.Errorf("prompt_cache_seed_too_large")
	}
	return body, nil
}

func truncateRenderedPromptAtBoundary(rendered string) (string, error) {
	if rendered == "" {
		return "", fmt.Errorf("prompt_cache_prefix_unavailable")
	}
	index := strings.LastIndex(rendered, promptCacheRenderBoundary)
	if index < 0 {
		return "", fmt.Errorf("prompt_cache_prefix_unavailable")
	}
	return rendered[:index], nil
}

func (m *Manager) applyPromptTemplate(ctx context.Context, cfg interface{ Endpoint(bool) string }, key string, body []byte) (string, error) {
	base := strings.TrimSuffix(cfg.Endpoint(m.runningInDocker), "/v1")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/apply-template", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.promptCacheHTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("apply_template_http_%d", resp.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxPromptSeedBytes+1))
	if err != nil || len(payload) > maxPromptSeedBytes {
		return "", fmt.Errorf("apply_template_response_invalid")
	}
	var object struct {
		Prompt string `json:"prompt"`
	}
	if json.Unmarshal(payload, &object) == nil && object.Prompt != "" {
		return object.Prompt, nil
	}
	var prompt string
	if json.Unmarshal(payload, &prompt) == nil && prompt != "" {
		return prompt, nil
	}
	return "", fmt.Errorf("apply_template_response_invalid")
}

func (m *Manager) promptCacheHTTPClient() *http.Client {
	return http.DefaultClient
}

func (m *Manager) warmPromptPrefix(ctx context.Context, plan runtimePlan, key string, seed promptCacheSeed) (promptCacheWarmupResult, error) {
	templateBody, err := promptCacheTemplateBody(seed)
	if err != nil {
		return promptCacheWarmupResult{}, err
	}
	rendered, err := m.applyPromptTemplate(ctx, plan.Config, key, templateBody)
	if err != nil {
		if ctx.Err() != nil {
			return promptCacheWarmupResult{}, fmt.Errorf("prompt_cache_qualification_timeout: %w", ctx.Err())
		}
		return promptCacheWarmupResult{}, fmt.Errorf("prompt_cache_template_failed: %w", err)
	}
	rendered, err = truncateRenderedPromptAtBoundary(rendered)
	if err != nil {
		return promptCacheWarmupResult{}, err
	}
	payload, err := json.Marshal(map[string]any{
		"prompt":       rendered,
		"n_predict":    0,
		"cache_prompt": true,
		"id_slot":      0,
	})
	if err != nil {
		return promptCacheWarmupResult{}, fmt.Errorf("prompt_cache_seed_invalid")
	}
	base := strings.TrimSuffix(plan.Config.Endpoint(m.runningInDocker), "/v1")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/completion", bytes.NewReader(payload))
	if err != nil {
		return promptCacheWarmupResult{}, fmt.Errorf("prompt_cache_warmup_failed")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.promptCacheHTTPClient().Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return promptCacheWarmupResult{}, fmt.Errorf("prompt_cache_qualification_timeout: %w", ctx.Err())
		}
		return promptCacheWarmupResult{}, fmt.Errorf("prompt_cache_transport_failed: %w", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxPromptSeedBytes+1))
	if readErr != nil || len(body) > maxPromptSeedBytes {
		return promptCacheWarmupResult{}, fmt.Errorf("prompt_cache_warmup_failed")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return promptCacheWarmupResult{}, promptCacheHTTPError(body)
	}
	cached, prompt := promptCacheUsage(body)
	if prompt == 0 {
		prompt = cached
	}
	if prompt == 0 {
		return promptCacheWarmupResult{}, fmt.Errorf("prompt_cache_seed_tokens_unavailable")
	}
	processed := prompt
	if cached <= prompt {
		processed -= cached
	}
	return promptCacheWarmupResult{
		SeedTokens: prompt, CachedTokens: cached, ProcessedTokens: processed,
	}, nil
}

func (m *Manager) qualifyPromptCache(ctx context.Context, plan runtimePlan, key string, seed promptCacheSeed) (promptCacheQualificationResult, error) {
	if !promptCacheSeedHasTool(seed, "discover_tools") {
		return promptCacheQualificationResult{}, fmt.Errorf("prompt_cache_probe_tool_unavailable")
	}
	cold, err := m.runPromptCacheProbe(ctx, plan, key, seed, false)
	if err != nil {
		return promptCacheQualificationResult{}, err
	}
	warmup, err := m.warmPromptPrefix(ctx, plan, key, seed)
	if err != nil {
		return promptCacheQualificationResult{}, err
	}
	warm, err := m.runPromptCacheProbe(ctx, plan, key, seed, true)
	if err != nil {
		return promptCacheQualificationResult{}, err
	}
	if warmup.CachedTokens > warm.CachedTokens {
		warm.CachedTokens = warmup.CachedTokens
	}
	if warmup.SeedTokens > warm.PromptTokens {
		warm.PromptTokens = warmup.SeedTokens
	}
	return validatePromptCacheQualification(cold, warm, warmup.SeedTokens)
}

func validatePromptCacheQualification(cold, warm promptCacheProbeResult, seedTokens uint64) (promptCacheQualificationResult, error) {
	if !cold.Complete || !warm.Complete ||
		!validPromptCacheProbeCall(cold.ToolCall) ||
		normalizeToolCall(cold.ToolCall) != normalizeToolCall(warm.ToolCall) {
		return promptCacheQualificationResult{}, fmt.Errorf("prompt_cache_semantic_mismatch")
	}
	if seedTokens == 0 || float64(warm.CachedTokens) < float64(seedTokens)*0.80 {
		return promptCacheQualificationResult{}, fmt.Errorf("prompt_cache_reuse_below_80_percent")
	}
	if cold.TTFT <= 0 || warm.TTFT <= 0 ||
		float64(warm.TTFT) > float64(cold.TTFT)*0.30 {
		return promptCacheQualificationResult{}, fmt.Errorf("prompt_cache_ttft_gain_below_70_percent")
	}
	processed := warm.PromptTokens
	if warm.CachedTokens <= processed {
		processed -= warm.CachedTokens
	}
	return promptCacheQualificationResult{
		SeedTokens: seedTokens, CachedTokens: warm.CachedTokens,
		ProcessedTokens: processed, ColdTTFT: cold.TTFT, WarmTTFT: warm.TTFT,
	}, nil
}

func promptCacheSeedHasTool(seed promptCacheSeed, name string) bool {
	var request struct {
		Tools []json.RawMessage `json:"tools"`
	}
	if json.Unmarshal(seed.ApplyTemplateBody, &request) != nil {
		return false
	}
	for _, tool := range request.Tools {
		if toolNameFromRaw(tool) == name {
			return true
		}
	}
	return false
}

func (m *Manager) runPromptCacheProbe(ctx context.Context, plan runtimePlan, key string, seed promptCacheSeed, cache bool) (promptCacheProbeResult, error) {
	var request map[string]json.RawMessage
	if json.Unmarshal(seed.ApplyTemplateBody, &request) != nil {
		return promptCacheProbeResult{}, fmt.Errorf("prompt_cache_seed_invalid")
	}
	setJSON := func(name string, value any) error {
		encoded, err := json.Marshal(value)
		if err == nil {
			request[name] = encoded
		}
		return err
	}
	if setJSON("stream", true) != nil ||
		setJSON("stream_options", map[string]bool{"include_usage": true}) != nil ||
		setJSON("cache_prompt", cache) != nil ||
		setJSON("max_tokens", 128) != nil ||
		setJSON("temperature", 0) != nil ||
		setJSON("tool_choice", "required") != nil ||
		setJSON("parallel_tool_calls", false) != nil {
		return promptCacheProbeResult{}, fmt.Errorf("prompt_cache_seed_invalid")
	}
	payload, err := json.Marshal(request)
	if err != nil || len(payload) > maxPromptSeedBytes {
		return promptCacheProbeResult{}, fmt.Errorf("prompt_cache_seed_too_large")
	}
	endpoint := strings.TrimSuffix(plan.Config.Endpoint(m.runningInDocker), "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return promptCacheProbeResult{}, fmt.Errorf("prompt_cache_transport_failed")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	started := time.Now()
	resp, err := m.promptCacheHTTPClient().Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return promptCacheProbeResult{}, fmt.Errorf("prompt_cache_qualification_timeout: %w", ctx.Err())
		}
		return promptCacheProbeResult{}, fmt.Errorf("prompt_cache_transport_failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return promptCacheProbeResult{}, promptCacheHTTPError(body)
	}
	result, err := readPromptCacheProbe(resp.Body, started)
	if err != nil {
		return promptCacheProbeResult{}, err
	}
	return result, nil
}

func readPromptCacheProbe(reader io.Reader, started time.Time) (promptCacheProbeResult, error) {
	type toolDelta struct {
		Index    int `json:"index"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	var result promptCacheProbeResult
	type callParts struct {
		name      strings.Builder
		arguments strings.Builder
	}
	calls := make(map[int]*callParts)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), maxLocalRequestBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			result.Complete = true
			continue
		}
		if data == "" {
			continue
		}
		if failed := promptCacheHTTPError([]byte(data)); errorCode(failed) != "prompt_cache_transport_failed" {
			return promptCacheProbeResult{}, failed
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string      `json:"content"`
					ToolCalls []toolDelta `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		cached, prompt := promptCacheUsage([]byte(data))
		if cached > result.CachedTokens {
			result.CachedTokens = cached
		}
		if prompt > result.PromptTokens {
			result.PromptTokens = prompt
		}
		for _, choice := range chunk.Choices {
			if result.TTFT == 0 && (choice.Delta.Content != "" || len(choice.Delta.ToolCalls) > 0) {
				result.TTFT = time.Since(started)
			}
			for _, delta := range choice.Delta.ToolCalls {
				parts := calls[delta.Index]
				if parts == nil {
					parts = &callParts{}
					calls[delta.Index] = parts
				}
				parts.name.WriteString(delta.Function.Name)
				parts.arguments.WriteString(delta.Function.Arguments)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return promptCacheProbeResult{}, fmt.Errorf("prompt_cache_transport_failed: %w", err)
	}
	if !result.Complete || len(calls) != 1 {
		return promptCacheProbeResult{}, fmt.Errorf("prompt_cache_semantic_mismatch")
	}
	parts, ok := calls[0]
	if !ok {
		return promptCacheProbeResult{}, fmt.Errorf("prompt_cache_semantic_mismatch")
	}
	encoded, _ := json.Marshal(map[string]any{
		"name":      parts.name.String(),
		"arguments": parts.arguments.String(),
	})
	result.ToolCall = string(encoded)
	return result, nil
}

func validPromptCacheProbeCall(value string) bool {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if json.Unmarshal([]byte(value), &call) != nil || call.Name != "discover_tools" {
		return false
	}
	var argumentText string
	if json.Unmarshal(call.Arguments, &argumentText) != nil {
		return false
	}
	var arguments struct {
		Operation string `json:"operation"`
		Query     string `json:"query"`
	}
	return json.Unmarshal([]byte(argumentText), &arguments) == nil &&
		arguments.Operation == "search" && arguments.Query == promptCacheProbeQuery
}

func promptCacheHTTPError(payload []byte) error {
	text := strings.ToLower(string(payload))
	switch {
	case strings.Contains(text, "out of memory"), strings.Contains(text, "oom"):
		return fmt.Errorf("prompt_cache_oom")
	case strings.Contains(text, "offload"), strings.Contains(text, "device lost"):
		return fmt.Errorf("prompt_cache_offload_failed")
	default:
		return fmt.Errorf("prompt_cache_transport_failed")
	}
}

func promptCacheUsage(raw []byte) (cached, prompt uint64) {
	var response struct {
		TokensCached    uint64 `json:"tokens_cached"`
		TokensEvaluated uint64 `json:"tokens_evaluated"`
		Usage           struct {
			PromptTokens        uint64 `json:"prompt_tokens"`
			PromptTokensDetails struct {
				CachedTokens uint64 `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
		Timings struct {
			CacheN  uint64 `json:"cache_n"`
			PromptN uint64 `json:"prompt_n"`
		} `json:"timings"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return 0, 0
	}
	prompt = response.Usage.PromptTokens
	cached = response.Usage.PromptTokensDetails.CachedTokens
	if response.Timings.CacheN > cached {
		cached = response.Timings.CacheN
	}
	if total := response.Timings.PromptN + response.Timings.CacheN; total > prompt {
		prompt = total
	}
	if response.TokensCached > cached {
		cached = response.TokensCached
	}
	if total := response.TokensCached + response.TokensEvaluated; total > prompt {
		prompt = total
	}
	return cached, prompt
}

func (m *Manager) promptCacheFingerprintLocked() string {
	if m.promptSeed == nil {
		return ""
	}
	payload := m.desiredFingerprint + "\x00" + m.promptSeed.Fingerprint + "\x00" +
		LlamaCPPCommit + "\x00" + m.status.PerformanceProfile + "\x00schema=3"
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) loadPromptCacheDecisionLocked() (promptCacheDecisionEntry, bool) {
	payload, err := os.ReadFile(filepath.Join(m.modelDir, promptCacheFileName))
	if err != nil {
		return promptCacheDecisionEntry{}, false
	}
	var entry promptCacheDecisionEntry
	if json.Unmarshal(payload, &entry) != nil ||
		entry.SchemaVersion != promptCacheDecisionSchemaVersion ||
		entry.Fingerprint != m.promptCacheFingerprintLocked() ||
		(entry.State != "warm" && entry.State != "rejected") {
		return promptCacheDecisionEntry{}, false
	}
	return entry, true
}

func (m *Manager) promptCacheDecisionEntryLocked(accepted, terminal bool) (promptCacheDecisionEntry, bool) {
	fingerprint := m.promptCacheFingerprintLocked()
	if fingerprint == "" ||
		(m.status.PromptCache.State != "warm" && m.status.PromptCache.State != "rejected") {
		return promptCacheDecisionEntry{}, false
	}
	entry := promptCacheDecisionEntry{
		SchemaVersion: promptCacheDecisionSchemaVersion,
		Fingerprint:   fingerprint, Accepted: accepted, State: m.status.PromptCache.State,
		Reason:            m.status.PromptCache.ErrorCode,
		ReuseThreshold:    0.80,
		TTFTGainThreshold: 0.70,
		WarmupDurationMS:  m.status.PromptCache.WarmupDurationMS,
		CachedTokens:      m.status.PromptCache.CachedTokens,
		ProcessedTokens:   m.status.PromptCache.ProcessedTokens,
		HitRate:           m.status.PromptCache.HitRate,
		ColdTTFTMS:        m.status.PromptCache.ColdTTFTMS,
		WarmTTFTMS:        m.status.PromptCache.WarmTTFTMS,
		Generation:        m.generation,
		Terminal:          terminal,
	}
	return entry, true
}

func (m *Manager) queuePromptCacheSnapshot(terminal bool) {
	m.mu.Lock()
	entry, ok := m.promptCacheDecisionEntryLocked(m.promptCacheQualified, terminal)
	m.mu.Unlock()
	if ok {
		m.queuePromptCacheDecision(entry, terminal)
	}
}

func (m *Manager) queuePromptCacheDecision(entry promptCacheDecisionEntry, terminal bool) {
	m.mu.Lock()
	if entry.Generation != m.generation || entry.Fingerprint != m.promptCacheFingerprintLocked() {
		m.mu.Unlock()
		return
	}
	entry.Terminal = terminal
	copyEntry := entry
	if m.promptPersistPending != nil && m.promptPersistPending.Terminal {
		copyEntry.Terminal = true
	}
	m.promptPersistPending = &copyEntry
	if m.promptPersistRunning {
		m.mu.Unlock()
		return
	}
	if !copyEntry.Terminal && !m.promptPersistLast.IsZero() &&
		time.Since(m.promptPersistLast) < promptCachePersistInterval {
		m.mu.Unlock()
		return
	}
	m.promptPersistRunning = true
	m.promptPersistWG.Add(1)
	m.mu.Unlock()
	go m.promptCachePersistLoop()
}

func (m *Manager) promptCachePersistLoop() {
	defer m.promptPersistWG.Done()
	for {
		m.mu.Lock()
		entry := m.promptPersistPending
		m.promptPersistPending = nil
		modelDir := m.modelDir
		writer := m.promptDecisionWrite
		m.mu.Unlock()
		if entry == nil {
			m.mu.Lock()
			m.promptPersistRunning = false
			m.mu.Unlock()
			return
		}
		payload, err := json.Marshal(entry)
		if err == nil {
			err = os.MkdirAll(modelDir, 0o700)
		}
		if err == nil {
			if writer == nil {
				writer = config.WriteFileAtomic
			}
			m.promptPersistCommit.Lock()
			m.mu.Lock()
			current := entry.Generation == m.generation &&
				entry.Fingerprint == m.promptCacheFingerprintLocked()
			m.mu.Unlock()
			if current {
				err = writer(filepath.Join(modelDir, promptCacheFileName), payload, 0o600)
			}
			m.promptPersistCommit.Unlock()
		}
		now := time.Now()
		m.mu.Lock()
		if entry.Generation == m.generation &&
			entry.Fingerprint == m.promptCacheFingerprintLocked() {
			if err != nil {
				m.status.PromptCache.DecisionPersisted = false
				m.status.PromptCache.ErrorCode = "prompt_cache_decision_write_failed"
			} else {
				m.promptPersistLast = now
				m.status.PromptCache.DecisionPersisted = true
				if m.status.PromptCache.ErrorCode == "prompt_cache_decision_write_failed" {
					m.status.PromptCache.ErrorCode = ""
				}
			}
		}
		next := m.promptPersistPending
		if next == nil || (!next.Terminal && time.Since(m.promptPersistLast) < promptCachePersistInterval) {
			m.promptPersistRunning = false
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()
	}
}

func (m *Manager) waitPromptCachePersistence(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		m.promptPersistWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func promptCacheResponseMetrics(payload []byte, stream bool) (cached, prompt uint64) {
	observe := func(raw []byte) {
		currentCached, currentPrompt := promptCacheUsage(raw)
		if currentCached > cached {
			cached = currentCached
		}
		if currentPrompt > prompt {
			prompt = currentPrompt
		}
	}
	if !stream {
		observe(payload)
		return cached, prompt
	}
	for _, line := range bytes.Split(payload, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(data) > 0 && !bytes.Equal(data, []byte("[DONE]")) {
			observe(data)
		}
	}
	return cached, prompt
}

func (m *Manager) observePromptCacheResponse(plan promptCacheObservationPlan, payload []byte, stream bool, firstByte time.Duration, complete bool) {
	if !complete {
		return
	}
	cached, prompt := promptCacheResponseMetrics(payload, stream)
	if prompt == 0 {
		return
	}
	processed := prompt
	if cached <= prompt {
		processed = prompt - cached
	}
	m.mu.Lock()
	if plan.Generation != m.generation || m.promptSeed == nil ||
		plan.SeedFingerprint != m.promptSeed.Fingerprint {
		m.mu.Unlock()
		return
	}
	status := &m.status.PromptCache
	status.Requests++
	status.LastHit = plan.CacheEnabled && cached > 0
	if plan.CacheEnabled && cached > 0 {
		status.Hits++
		status.WarmTTFTMS = float64(firstByte.Microseconds()) / 1000
	} else {
		status.ColdTTFTMS = float64(firstByte.Microseconds()) / 1000
	}
	status.CachedTokens += cached
	status.ProcessedTokens += processed
	total := status.CachedTokens + status.ProcessedTokens
	if total > 0 {
		status.HitRate = float64(status.CachedTokens) / float64(total)
	}
	if plan.CacheEnabled && status.State == "warm" && status.SeedTokens > 0 &&
		float64(cached) < float64(status.SeedTokens)*0.80 {
		status.State = "rejected"
		status.Qualified = false
		m.promptCacheQualified = false
		status.ErrorCode = "prompt_cache_reuse_below_80_percent"
	}
	terminal := status.State == "rejected"
	entry, ok := m.promptCacheDecisionEntryLocked(status.State == "warm", terminal)
	m.mu.Unlock()
	if ok {
		m.queuePromptCacheDecision(entry, terminal)
	}
}
