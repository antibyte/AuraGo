# Plan: Anthropic API Direct Integration

> **Status**: Draft  
> **Date**: 2026-04-04  
> **Scope**: Native Anthropic Messages API support (beyond OpenAI-compatible proxy)

---

## 1. Current State Analysis

### What Already Exists

| Component | Status | Details |
|-----------|--------|---------|
| Provider type `"anthropic"` | ✅ Recognized | `config_types.go:59`, `config_migrate.go:170` |
| Config resolution | ✅ Works | Provider auto-detection from URL (`config_migrate.go:762`) |
| Pricing table | ✅ Complete | `pricing.go:106-122` — 10 Claude models with hardcoded prices |
| Cloudflare AI Gateway | ✅ Works | `client.go:141` — routes through `anthropic` segment |
| Setup UI | ✅ Present | Provider dropdown includes Anthropic option |
| Web UI provider config | ✅ Present | `ui/cfg/providers.js` handles Anthropic type |
| Env var fallback | ✅ Present | `ANTHROPIC_API_KEY` in `config.go:336` |
| Translations | ✅ Present | All 16 languages have `"setup.step0_provider_anthropic"` |

### What Is Missing (The Core Problem)

**All LLM communication goes through `sashabaranov/go-openai`** which speaks the OpenAI Chat Completions protocol (`/v1/chat/completions`). The Anthropic Messages API (`POST /v1/messages`) has a fundamentally different request/response format:

| Aspect | OpenAI (`/v1/chat/completions`) | Anthropic (`/v1/messages`) |
|--------|----------------------------------|----------------------------|
| Auth header | `Authorization: Bearer sk-...` | `x-api-key: sk-ant-...` |
| Version header | — | `anthropic-version: 2023-06-01` |
| System prompt | `messages[0].role = "system"` | `system` field (top-level, not in messages) |
| Message format | `{ role, content: string }` | `{ role, content: string \| ContentBlock[] }` |
| Tool calling | `tools[].type = "function"` | `tools[].type = "custom"` with `input_schema` |
| Tool result | `role: "tool"` + `tool_call_id` | `role: "user"` with `tool_result` content block |
| Streaming | SSE `data: {...}` | SSE `event: type` + `data: {...}` |
| Response format | `{ choices: [{ message }] }` | `{ content: [...], stop_reason }` |
| Image input | `content: [{ type: "image_url" }]` | `content: [{ type: "image", source: {...} }]` |

**Currently**, selecting `type: anthropic` with `base_url: https://api.anthropic.com` will **fail** because `go-openai` sends an OpenAI-format request to the Anthropic endpoint. Users must route through OpenRouter (which translates formats) or Cloudflare AI Gateway.

---

## 2. Architecture Decision: HTTP Transport Adapter

### Approach: `anthropicTransport` (Recommended)

Add an `http.RoundTripper` that translates OpenAI-format requests to Anthropic-format requests **in-flight**, following the same pattern as the existing `miniMaxTransport` in `client.go:159-197`.

**Why this approach:**
- **Zero changes to the agent loop** — `ChatClient` interface remains identical
- **Zero changes to tool schema building** — OpenAI function-calling format is translated
- **Zero changes to streaming assembly** — Anthropic SSE events are mapped to OpenAI SSE chunks
- **Zero changes to failover/retry** — `FailoverManager` wraps `*openai.Client` transparently
- **Consistent with existing patterns** — `miniMaxTransport` proves the approach works

**Trade-off**: The translation layer adds complexity in one file, but avoids touching 50+ files across the agent, tools, memory, and server packages.

### Alternative Considered: Separate Anthropic Client

A dedicated `anthropic.Client` implementing `ChatClient` would require:
- New `anthropic` package or dependency (`github.com/anthropics/anthropic-sdk-go`)
- Dual client paths in `FailoverManager` (OpenAI ↔ Anthropic failover)
- Separate streaming handler in agent loop
- Separate tool call parsing
- **Estimate: 2000+ lines changed across 20+ files**

Rejected due to blast radius.

---

## 3. Implementation Plan

### Phase 1: Anthropic Transport Layer
**File: `internal/llm/anthropic_transport.go`** (new, ~400 lines)

The transport intercepts HTTP requests destined for `api.anthropic.com` and performs bidirectional translation.

#### 3.1.1 Request Translation (`RoundTrip`)

```
OpenAI Request → anthropicTransport.RoundTrip() → Anthropic Request
```

| Translation Step | Details |
|------------------|---------|
| **Auth** | Replace `Authorization: Bearer X` → `x-api-key: X` + `anthropic-version: 2023-06-01` |
| **URL** | Rewrite `/v1/chat/completions` → `/v1/messages` |
| **System messages** | Extract all `role: "system"` messages → top-level `system` field |
| **User/Assistant messages** | Map `role: "user"/"assistant"` → Anthropic message format |
| **Tool definitions** | Map `tools[].type: "function"` → `tools[].type: "custom"` with `input_schema` |
| **Tool results** | Map `role: "tool"` messages → `role: "user"` with `tool_result` content blocks |
| **Images** | Map `image_url` content blocks → Anthropic `image` source format |
| **Model** | Pass through (Claude model names are valid) |
| **Max tokens** | Map `max_tokens` field |
| **Temperature** | Pass through |
| **Stream** | Map `stream: true` → `stream: true` |
| **Tool choice** | Map `"auto"` → `{"type": "auto"}`, `"none"` → `{"type": "none"}` |

#### 3.1.2 Non-Streaming Response Translation

```
Anthropic Response → translateResponse() → OpenAI Response
```

| Translation Step | Details |
|------------------|---------|
| **Structure** | Map `content[0].text` → `choices[0].message.content` |
| **Tool calls** | Map `content[].type: "tool_use"` → `choices[0].message.tool_calls[]` |
| **Stop reason** | Map `stop_reason: "end_turn"/"tool_use"` → `finish_reason: "stop"/"tool_calls"` |
| **Usage** | Map `usage.input_tokens/output_tokens` → `usage.prompt_tokens/completion_tokens` |
| **ID** | Generate `chatcmpl-` prefixed ID from Anthropic `id` |
| **Model** | Pass through |

#### 3.1.3 Streaming Response Translation

```
Anthropic SSE → anthropicStreamDecoder → OpenAI SSE chunks
```

Anthropic uses typed SSE events: `event: message_start`, `event: content_block_start`, `event: content_block_delta`, `event: message_stop`, etc.

| Anthropic Event | OpenAI SSE Equivalent |
|----------------|----------------------|
| `message_start` | Initial chunk with `role: "assistant"` |
| `content_block_start` (text) | Delta chunk with `content: ""` |
| `content_block_delta` (text_delta) | Delta chunk with `content: "..."` |
| `content_block_start` (tool_use) | Delta chunk with `tool_calls[0]` (ID + function name) |
| `content_block_delta` (input_json_delta) | Delta chunk with `tool_calls[0].function.arguments` partial |
| `content_block_stop` | No-op |
| `message_delta` (stop_reason) | Final chunk with `finish_reason` |
| `message_stop` | `[DONE]` |

The decoder must buffer and re-emit SSE lines as `data: {"id":"...","choices":[...]}\n\n` and terminate with `data: [DONE]\n\n`.

#### 3.1.4 Edge Cases

| Edge Case | Handling |
|-----------|----------|
| **Consecutive user messages** | Anthropic forbids consecutive `user` messages. Merge them with `\n\n` separator, or inject an empty `assistant` message between them. |
| **System messages mid-conversation** | Anthropic only supports a single top-level `system` field. Concatenate all system messages (including tool-result-as-system from legacy path) into the `system` field. |
| **Empty content** | Anthropic requires non-empty content. Map `content: ""` → `content: " "` (space). |
| **Image base64** | Map OpenAI `image_url.url` (`data:image/png;base64,...`) → Anthropic `{ type: "base64", media_type: "image/png", data: "..." }`. |
| **Image URL** | Anthropic does not support image URLs. Download the image and convert to base64. |
| **Parallel tool calls** | Anthropic supports parallel tool use. Map multiple `tool_use` blocks to OpenAI `tool_calls[]` array. |
| **Streaming error** | Map `event: error` → OpenAI error format. |
| **422 errors** | Some models reject certain message patterns. The existing 422 recovery in `agent_loop.go` already handles this. |

---

### Phase 2: Client Factory Integration
**File: `internal/llm/client.go`** (~20 lines changed)

#### 3.2.1 Changes to `NewClient()`

```go
// After building clientConfig, before creating the client:
if providerType == "anthropic" {
    // Set default Anthropic base URL if not overridden by AI Gateway
    if !cfg.AIGateway.Enabled {
        if clientConfig.BaseURL == "" {
            clientConfig.BaseURL = "https://api.anthropic.com/v1"
        }
    }
    // Wrap HTTP transport with Anthropic adapter
    baseTransport := clientConfig.HTTPClient
    if baseTransport == nil {
        baseTransport = http.DefaultTransport
    }
    clientConfig.HTTPClient = &http.Client{
        Transport: &anthropicTransport{base: baseTransport},
    }
}
```

#### 3.2.2 Changes to `NewClientFromProvider()`

Same pattern: detect `providerType == "anthropic"` and inject the transport.

#### 3.2.3 Changes to `buildLLMHTTPClient()`

Add Anthropic transport to the transport stack (after AI Gateway auth, before MiniMax):

```go
if providerType == "anthropic" {
    transport = &anthropicTransport{base: transport}
    hasCustomTransport = true
}
```

---

### Phase 3: Model Capabilities
**File: `internal/agent/tooling_policy.go`** (~10 lines changed)

#### 3.3.1 Anthropic Capability Detection

```go
func resolveModelCapabilities(cfg *config.Config) ModelCapabilities {
    // ... existing code ...
    isAnthropic := lowerProvider == "anthropic"
    isClaude := isAnthropic || strings.Contains(lowerModel, "claude")

    return ModelCapabilities{
        // ... existing fields ...
        IsAnthropic:               isAnthropic,
        AutoEnableNativeFunctions: isDeepSeek || isAnthropic, // Claude excels at native FC
    }
}
```

Add `IsAnthropic bool` to `ModelCapabilities` struct.

**Rationale**: Claude models support native function calling natively and should have it auto-enabled (like DeepSeek). They also support parallel tool calls and structured outputs.

---

### Phase 4: LLM Guardian Compatibility
**File: `internal/security/llm_guardian.go`** (~5 lines changed)

The LLM Guardian builds messages via `buildMessages()` which uses `openai.ChatMessageRoleSystem`. When the Guardian's provider is Anthropic, the system prompt must be merged into the user message (already handled for some providers in `buildMessages()` line 181-193).

Add `"anthropic"` to the existing check for providers that need system-role merging:

```go
func (g *LLMGuardian) buildMessages(systemPrompt, userPrompt string) []openai.ChatCompletionMessage {
    pt := strings.ToLower(g.cfg.LLMGuardian.ProviderType)
    if pt == "ollama" {
        // Ollama handles system role fine
        return ...
    }
    // For cloud providers (including Anthropic): merge system prompt into user message
    return ...
}
```

No change needed — the Anthropic transport will handle system message extraction.

---

### Phase 5: Pricing Updates
**File: `internal/llm/pricing.go`** (~10 lines changed)

Add newer Claude model variants:

```go
func directAnthropicPricing() []ModelPricing {
    return []ModelPricing{
        // Existing entries...
        // Add new models:
        {ModelID: "claude-sonnet-4-20250514", InputPerMillion: 3.00, OutputPerMillion: 15.00},
        {ModelID: "claude-opus-4-20250514", InputPerMillion: 15.00, OutputPerMillion: 75.00},
        {ModelID: "claude-haiku-3-5-20241022", InputPerMillion: 0.80, OutputPerMillion: 4.00},
    }
}
```

---

### Phase 6: Config Defaults & UI
**Files: Config defaults + Web UI** (~30 lines changed)

#### 3.6.1 Default Base URL

In `config_migrate.go`, when provider type is `anthropic` and no base URL is set:

```go
case "anthropic":
    resolved.BaseURL = "https://api.anthropic.com/v1"
```

#### 3.6.2 Web UI Default Model Suggestions

In `ui/cfg/providers.js`, ensure Anthropic provider shows Claude model suggestions in the model dropdown.

#### 3.6.3 Setup Wizard

In `ui/setup.html`, ensure the Anthropic provider option:
- Shows `https://api.anthropic.com/v1` as default base URL
- Pre-fills with common Claude model names
- Test connection works (already sends an OpenAI-format request → transport translates)

---

### Phase 7: Testing

#### 3.7.1 Unit Tests (`internal/llm/anthropic_transport_test.go`)

| Test | Description |
|------|-------------|
| `TestAnthropicRequestConversion` | OpenAI request → Anthropic JSON structure |
| `TestAnthropicSystemMessageExtraction` | System messages moved to top-level `system` field |
| `TestAnthropicConsecutiveUserMessages` | Merged with separator |
| `TestAnthropicToolCallTranslation` | OpenAI function tools → Anthropic custom tools |
| `TestAnthropicToolResultTranslation` | `role: "tool"` → `role: "user"` + `tool_result` |
| `TestAnthropicResponseConversion` | Anthropic response → OpenAI response |
| `TestAnthropicStreamingConversion` | Anthropic SSE events → OpenAI SSE chunks |
| `TestAnthropicImageTranslation` | OpenAI image_url → Anthropic base64 image |
| `TestAnthropicEmptyContent` | Empty content → space |
| `TestAnthropicParallelToolCalls` | Multiple tool_use blocks → tool_calls array |
| `TestAnthropicErrorMapping` | Anthropic error response → OpenAI error format |

#### 3.7.2 Integration Tests

| Test | Description |
|------|-------------|
| `TestAnthropicTransportE2E` | Full request/response cycle through httptest.Server |
| `TestAnthropicStreamE2E` | Streaming through httptest.Server with SSE |
| `TestAnthropicWithAIGateway` | Transport stacks correctly (AI Gateway auth + Anthropic) |

#### 3.7.3 Manual Testing Checklist

- [ ] Direct Anthropic API with Claude 3.5 Sonnet — chat completion
- [ ] Direct Anthropic API — streaming chat completion
- [ ] Direct Anthropic API — native function calling (single tool)
- [ ] Direct Anthropic API — parallel function calling (multiple tools)
- [ ] Direct Anthropic API — tool result round-trip
- [ ] Direct Anthropic API — vision/image input
- [ ] Cloudflare AI Gateway proxy with Anthropic provider
- [ ] Failover: Anthropic primary → OpenRouter fallback
- [ ] Setup wizard: Anthropic provider selection + test connection
- [ ] Config UI: Anthropic provider settings
- [ ] LLM Guardian: Anthropic as Guardian LLM
- [ ] Memory analysis / personality engine with Anthropic helper LLM
- [ ] Budget tracking with Anthropic pricing

---

## 4. File Change Summary

| File | Action | Lines | Phase |
|------|--------|-------|-------|
| `internal/llm/anthropic_transport.go` | **New** | ~400 | 1 |
| `internal/llm/anthropic_transport_test.go` | **New** | ~300 | 7 |
| `internal/llm/client.go` | Modify | ~20 | 2 |
| `internal/agent/tooling_policy.go` | Modify | ~10 | 3 |
| `internal/llm/pricing.go` | Modify | ~10 | 5 |
| `internal/config/config_migrate.go` | Modify | ~5 | 6 |
| `ui/cfg/providers.js` | Modify | ~15 | 6 |
| `go.mod` / `go.sum` | No change | 0 | — |

**Total estimated new/changed code: ~760 lines**  
**Files touched: 7 (2 new, 5 modified)**

---

## 5. Dependency Analysis

### No New Dependencies Required

The `anthropicTransport` is a pure `http.RoundTripper` implementation using only `encoding/json`, `net/http`, `io`, `strings`, and `bufio` — all standard library. No need for `anthropic-sdk-go` or any third-party Anthropic library.

### Existing Dependencies (Unchanged)

- `github.com/sashabaranov/go-openai v1.41.2` — remains the primary LLM client
- All other dependencies unchanged

---

## 6. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Anthropic API changes format | Low | High | Pin `anthropic-version` header; add version-specific handling |
| Streaming edge cases (partial events, reconnect) | Medium | Medium | Thorough SSE parser with error recovery; fall back to non-streaming |
| Tool call argument truncation in streaming | Low | Medium | Buffer partial JSON in stream decoder; validate completeness |
| Consecutive user message merging loses context | Low | Low | Use clear separator (`\n\n---\n\n`); add comment explaining merge |
| Image URL download fails | Medium | Low | Return error message as content; agent can retry |
| Claude model-specific quirks (e.g., Opus vs Haiku) | Low | Low | Per-model capability flags in `ModelCapabilities` |

---

## 7. Out of Scope (Future Work)

These items are intentionally excluded from this plan:

1. **Anthropic prompt caching** — `cache_control` content blocks for reducing costs on repeated system prompts. Can be added later as an optimization.
2. **Anthropic extended thinking** — Claude's extended thinking / chain-of-thought features. Requires UI changes to display thinking blocks.
3. **Anthropic PDF/document input** — Claude's native document understanding. Requires content block format changes.
4. **Token counting** — Anthropic uses a different tokenizer (Tiktoken vs Claude tokenizer). Token counts from responses will be used directly.
5. **Batch API** — Anthropic's Message Batches API for async processing. Not relevant to the agent loop.
6. **Citations** — Claude's citation feature for grounded responses.

---

## 8. Implementation Order

```
Phase 1: anthropic_transport.go (core translation layer)
    ↓
Phase 7.1: Unit tests (validate translation correctness)
    ↓
Phase 2: client.go integration (wire transport into factory)
    ↓
Phase 3: tooling_policy.go (auto-enable native functions)
    ↓
Phase 7.2: Integration tests (E2E through httptest)
    ↓
Phase 5: pricing.go (add new models)
    ↓
Phase 6: Config defaults + UI (user-facing changes)
    ↓
Phase 7.3: Manual testing checklist
```

Phases 1-3 can be implemented in a single PR. Phases 5-6 can follow in a second commit. Phase 7.3 (manual testing) requires an Anthropic API key.
