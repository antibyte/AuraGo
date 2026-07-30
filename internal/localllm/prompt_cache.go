package localllm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	maxLocalRequestBytes = 4 << 20
	maxPromptSeedBytes   = 256 << 10
	promptCacheFileName  = "prompt-cache-decision.json"
	promptCacheProbeText = "__AURAGO_PREFIX_WARMUP__"
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
	Fingerprint      string  `json:"fingerprint"`
	Accepted         bool    `json:"accepted"`
	State            string  `json:"state"`
	WarmupDurationMS float64 `json:"warmup_duration_ms,omitempty"`
	CachedTokens     uint64  `json:"cached_tokens,omitempty"`
	ProcessedTokens  uint64  `json:"processed_tokens,omitempty"`
	HitRate          float64 `json:"hit_rate,omitempty"`
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

func preparePromptCacheRequest(req *http.Request) (*http.Request, *promptCacheSeed, bool, error) {
	if req == nil || req.Method != http.MethodPost ||
		!strings.HasSuffix(strings.TrimSuffix(req.URL.Path, "/"), "/chat/completions") {
		return req, nil, false, nil
	}
	raw, err := io.ReadAll(io.LimitReader(req.Body, maxLocalRequestBytes+1))
	if err != nil {
		return nil, nil, false, fmt.Errorf("local_request_read_failed: %w", err)
	}
	if len(raw) > maxLocalRequestBytes {
		return nil, nil, false, fmt.Errorf("local_request_too_large")
	}
	_ = req.Body.Close()

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, nil, false, fmt.Errorf("local_request_invalid")
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

	seed, err := seedFromChatRequest(object)
	if err != nil {
		seed = nil
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		return nil, nil, false, fmt.Errorf("local_request_encode_failed")
	}
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Body = io.NopCloser(bytes.NewReader(encoded))
	clone.ContentLength = int64(len(encoded))
	clone.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(encoded)), nil
	}
	return clone, seed, stream, nil
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
	perf := performanceProfileFor(m.profile)
	m.status.PerformanceProfile = perf.Name
	m.status.PromptCache = PromptCacheStatus{
		State: "cold", SeedFingerprint: seed.Fingerprint,
		ToolsetFingerprint: seed.ToolsetFingerprint, StableToolCount: seed.ToolCount,
		CacheRAMMiB: perf.CacheRAMMiB, CheckpointProfile: "32x2048",
	}
	if decision, ok := m.loadPromptCacheDecisionLocked(); ok && decision.State == "rejected" {
		m.status.PromptCache.State = "rejected"
		m.status.PromptCache.ErrorCode = "prompt_cache_profile_rejected"
		m.status.PromptCache.WarmupDurationMS = decision.WarmupDurationMS
		m.status.PromptCache.CachedTokens = decision.CachedTokens
		m.status.PromptCache.ProcessedTokens = decision.ProcessedTokens
		m.status.PromptCache.HitRate = decision.HitRate
	}
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
		m.status.PromptCache.State == "warm"
}

func (m *Manager) ensurePromptCacheWarm(ctx context.Context, plan runtimePlan, key string) {
	m.mu.Lock()
	if plan.Generation != m.generation || m.promptSeed == nil ||
		m.promptSeed.Generation != plan.Generation {
		m.mu.Unlock()
		return
	}
	if m.status.PromptCache.State == "rejected" {
		m.mu.Unlock()
		return
	}
	seed := *m.promptSeed
	seed.ApplyTemplateBody = append([]byte(nil), m.promptSeed.ApplyTemplateBody...)
	if m.status.PromptCache.State == "warm" &&
		m.status.PromptCache.SeedFingerprint == seed.Fingerprint {
		m.mu.Unlock()
		return
	}
	m.status.PromptCache.State = "warming"
	m.status.PromptCache.ErrorCode = ""
	m.mu.Unlock()

	started := time.Now()
	warmCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	rendered, err := m.applyPromptTemplate(warmCtx, plan.Config, key, seed.ApplyTemplateBody)
	if err == nil {
		rendered, err = truncateRenderedPromptAtStaticPrefix(rendered, seed.SystemPrefix)
	}
	if err == nil {
		var response json.RawMessage
		err = m.apiJSONFor(warmCtx, plan.Config, http.MethodPost, "/completion", key, map[string]any{
			"prompt": rendered, "n_predict": 0, "cache_prompt": true, "id_slot": 0,
		}, &response)
		if err == nil {
			var timing struct {
				TokensCached    uint64 `json:"tokens_cached"`
				TokensEvaluated uint64 `json:"tokens_evaluated"`
				Timings         struct {
					CacheN  uint64 `json:"cache_n"`
					PromptN uint64 `json:"prompt_n"`
				} `json:"timings"`
			}
			if json.Unmarshal(response, &timing) == nil {
				seedTokens := timing.Timings.CacheN + timing.Timings.PromptN
				if timing.TokensCached > seedTokens {
					seedTokens = timing.TokensCached
				}
				if timing.TokensEvaluated > seedTokens {
					seedTokens = timing.TokensEvaluated
				}
				m.mu.Lock()
				if plan.Generation == m.generation && m.promptSeed != nil &&
					m.promptSeed.Fingerprint == seed.Fingerprint {
					m.status.PromptCache.SeedTokens = seedTokens
				}
				m.mu.Unlock()
			}
		}
	}
	duration := float64(time.Since(started).Microseconds()) / 1000
	m.mu.Lock()
	defer m.mu.Unlock()
	if plan.Generation != m.generation || m.promptSeed == nil ||
		m.promptSeed.Fingerprint != seed.Fingerprint {
		return
	}
	if err != nil {
		m.status.PromptCache.State = "degraded"
		m.status.PromptCache.ErrorCode = "prompt_cache_warmup_failed"
		m.savePromptCacheDecisionLocked(false, duration)
		return
	}
	m.status.PromptCache.State = "warm"
	m.status.PromptCache.WarmupDurationMS = duration
	m.status.PromptCache.ErrorCode = ""
	m.savePromptCacheDecisionLocked(true, duration)
}

func truncateRenderedPromptAtStaticPrefix(rendered, prefix string) (string, error) {
	if rendered == "" || prefix == "" {
		return "", fmt.Errorf("prompt_cache_prefix_unavailable")
	}
	index := strings.LastIndex(rendered, prefix)
	if index < 0 {
		return "", fmt.Errorf("prompt_cache_prefix_unavailable")
	}
	return rendered[:index+len(prefix)], nil
}

func (m *Manager) applyPromptTemplate(ctx context.Context, cfg interface{ Endpoint(bool) string }, key string, body []byte) (string, error) {
	base := strings.TrimSuffix(cfg.Endpoint(m.runningInDocker), "/v1")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/apply-template", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
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

func (m *Manager) promptCacheFingerprintLocked() string {
	if m.promptSeed == nil {
		return ""
	}
	payload := m.desiredFingerprint + "\x00" + m.promptSeed.Fingerprint + "\x00" +
		LlamaCPPCommit + "\x00" + m.status.PerformanceProfile
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) loadPromptCacheDecisionLocked() (promptCacheDecisionEntry, bool) {
	payload, err := os.ReadFile(filepath.Join(m.modelDir, promptCacheFileName))
	if err != nil {
		return promptCacheDecisionEntry{}, false
	}
	var entry promptCacheDecisionEntry
	if json.Unmarshal(payload, &entry) != nil || entry.Fingerprint != m.promptCacheFingerprintLocked() {
		return promptCacheDecisionEntry{}, false
	}
	return entry, true
}

func (m *Manager) savePromptCacheDecisionLocked(accepted bool, duration float64) {
	fingerprint := m.promptCacheFingerprintLocked()
	if fingerprint == "" || os.MkdirAll(m.modelDir, 0o700) != nil {
		return
	}
	entry := promptCacheDecisionEntry{
		Fingerprint: fingerprint, Accepted: accepted, State: m.status.PromptCache.State,
		WarmupDurationMS: duration,
		CachedTokens:     m.status.PromptCache.CachedTokens,
		ProcessedTokens:  m.status.PromptCache.ProcessedTokens,
		HitRate:          m.status.PromptCache.HitRate,
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_ = config.WriteFileAtomic(
		filepath.Join(m.modelDir, promptCacheFileName),
		payload,
		0o600,
	)
}

func (m *Manager) observePromptCacheResponse(payload []byte, stream bool, firstByte time.Duration) {
	var cached, prompt uint64
	observe := func(raw []byte) {
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
			return
		}
		if response.Usage.PromptTokens > prompt {
			prompt = response.Usage.PromptTokens
		}
		if response.Usage.PromptTokensDetails.CachedTokens > cached {
			cached = response.Usage.PromptTokensDetails.CachedTokens
		}
		if response.Timings.CacheN > cached {
			cached = response.Timings.CacheN
		}
		if response.Timings.PromptN+response.Timings.CacheN > prompt {
			prompt = response.Timings.PromptN + response.Timings.CacheN
		}
		if response.TokensCached > cached {
			cached = response.TokensCached
		}
		if response.TokensCached+response.TokensEvaluated > prompt {
			prompt = response.TokensCached + response.TokensEvaluated
		}
	}
	if stream {
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
	} else {
		observe(payload)
	}
	if prompt == 0 {
		return
	}
	processed := prompt
	if cached <= prompt {
		processed = prompt - cached
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	status := &m.status.PromptCache
	status.Requests++
	status.LastHit = cached > 0
	if cached > 0 {
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
	if status.State == "warm" && status.SeedTokens > 0 &&
		float64(cached) < float64(status.SeedTokens)*0.80 {
		status.State = "rejected"
		status.ErrorCode = "prompt_cache_reuse_below_80_percent"
	}
	m.savePromptCacheDecisionLocked(status.State == "warm", status.WarmupDurationMS)
}
