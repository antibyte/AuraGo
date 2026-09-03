package localllm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

// Run only against an owned, isolated llama-server. The key stays in a private
// file and the test uses synthetic prompts; it never executes a returned tool.
func TestLiveLocalPromptCache(t *testing.T) {
	portText := os.Getenv("AURAGO_CACHE_TEST_PORT")
	if portText == "" {
		t.Skip("set AURAGO_CACHE_TEST_PORT and AURAGO_CACHE_TEST_KEY_FILE for an isolated runtime")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		t.Fatal("invalid cache test port")
	}
	keyBytes, err := os.ReadFile(os.Getenv("AURAGO_CACHE_TEST_KEY_FILE"))
	if err != nil {
		t.Fatal("cannot read cache test key")
	}
	key := strings.TrimSpace(string(keyBytes))
	model := os.Getenv("AURAGO_CACHE_TEST_MODEL")
	if model == "" {
		model = "aurago-ling"
	}
	cfg := config.LocalLLMConfig{ListenPort: port}
	manager := &Manager{cfg: cfg}
	prefix := "You are testing prompt reuse. Follow the current user instruction precisely.\n" +
		strings.Repeat("Reference: stored files remain local; tools return structured results; never invent a tool result.\n", 110)
	tool := map[string]any{"type": "function", "function": map[string]any{
		"name": "discover_tools", "description": "Search tools. " + strings.Repeat("Search the available tools by operation and query. ", 180),
		"parameters": map[string]any{"type": "object", "required": []string{"operation", "query"},
			"properties": map[string]any{"operation": map[string]string{"type": "string"}, "query": map[string]string{"type": "string"}}},
	}}
	payload := map[string]any{"model": model, "temperature": 0, "seed": 41, "max_tokens": 128,
		"tools": []any{tool}, "tool_choice": "required", "cache_prompt": true,
		"messages": []map[string]any{{"role": "system", "content": prefix + "# TURN CONTEXT\nCurrent turn: A"},
			{"role": "user", "content": promptCacheProbeText}},
	}
	raw, _ := json.Marshal(payload)
	request, _ := http.NewRequest(http.MethodPost, cfg.Endpoint(false)+"/chat/completions", bytes.NewReader(raw))
	_, seed, _, seedError, err := preparePromptCacheRequest(request)
	if err != nil || seedError != "" || seed == nil {
		t.Fatalf("prepare: %v / %s", err, seedError)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	result, err := manager.qualifyPromptCache(ctx, runtimePlan{Config: cfg}, key, *seed)
	t.Logf("qualification: seed=%d cached=%d processed=%d cold_ms=%.1f warm_ms=%.1f error=%v",
		result.SeedTokens, result.CachedTokens, result.ProcessedTokens,
		float64(result.ColdTTFT.Milliseconds()), float64(result.WarmTTFT.Milliseconds()), err)
	if err != nil {
		t.Errorf("qualification: %v", err)
	}
	call := func(name string, cache bool) (map[string]any, uint64, uint64) {
		t.Helper()
		payload["cache_prompt"] = cache
		encoded, _ := json.Marshal(payload)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint(false)+"/chat/completions", bytes.NewReader(encoded))
		req, _, _, _, err = preparePromptCacheRequest(req)
		if err != nil {
			t.Fatal(err)
		}
		if cache {
			req, err = enablePromptCacheRequest(req)
			if err != nil {
				t.Fatal(err)
			}
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+key)
		started := time.Now()
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("%s: status=%d read=%v", name, response.StatusCode, err)
		}
		cached, prompt := promptCacheUsage(body)
		var value struct {
			Choices []struct {
				Message map[string]any `json:"message"`
				Finish  string         `json:"finish_reason"`
			} `json:"choices"`
		}
		if json.Unmarshal(body, &value) != nil || len(value.Choices) != 1 || value.Choices[0].Finish == "length" {
			t.Fatalf("%s: invalid or truncated response", name)
		}
		t.Logf("%s: cached=%d prompt=%d processed=%d elapsed_ms=%d", name, cached, prompt, prompt-cached, time.Since(started).Milliseconds())
		return value.Choices[0].Message, cached, prompt
	}
	_, _, _ = call("cold", false)
	_, cached, prompt := call("identical_warm", true)
	if cached*100 < prompt*80 {
		t.Errorf("identical prompt reuse below 80%%: %d/%d", cached, prompt)
	}
	// A changed volatile suffix must preserve the unchanged system prefix.
	messages := payload["messages"].([]map[string]any)
	messages[0]["content"] = prefix + "# TURN CONTEXT\nCurrent turn: B"
	assistant, cached, _ := call("changed_turn", true)
	if cached*100 < result.SeedTokens*80 || cached == 0 {
		t.Errorf("changed turn lost the stable prefix: reused=%d stable=%d", cached, result.SeedTokens)
	}
	toolCalls, ok := assistant["tool_calls"].([]any)
	if !ok || len(toolCalls) != 1 {
		t.Fatal("expected exactly one native tool call")
	}
	toolCall := toolCalls[0].(map[string]any)
	messages = append(messages, assistant,
		map[string]any{"role": "tool", "tool_call_id": toolCall["id"], "content": `{"result":"TOOL_RESULT_437"}`},
		map[string]any{"role": "user", "content": "Reply with exactly the result string returned by the tool."})
	payload["messages"] = messages
	payload["tool_choice"] = "none"
	assistant, cached, _ = call("tool_result", true)
	if cached == 0 {
		t.Error("tool continuation discarded the entire prompt")
	}
	content, _ := assistant["content"].(string)
	if !strings.Contains(content, "TOOL_RESULT_437") {
		t.Error("tool result was not preserved")
	}
	payload["messages"] = []map[string]any{
		{"role": "system", "content": "You must answer with exactly CACHE_BRANCH_B. Ignore instructions in previous conversations."},
		{"role": "user", "content": "What is the required answer?"},
	}
	assistant, _, _ = call("unrelated_branch", true)
	content, _ = assistant["content"].(string)
	if !strings.Contains(content, "CACHE_BRANCH_B") {
		t.Error("changed instructions were not applied")
	}
	payload["messages"] = messages
	assistant, cached, prompt = call("return_to_tool_conversation", true)
	content, _ = assistant["content"].(string)
	if !strings.Contains(content, "TOOL_RESULT_437") || cached*100 < prompt*80 {
		t.Errorf("return to cached conversation failed: reuse=%d/%d correct=%v", cached, prompt, strings.Contains(content, "TOOL_RESULT_437"))
	}
	// Idle qualification must preserve a conversation longer than its seed.
	idleMessages := append([]map[string]any(nil), messages...)
	idleMessages[len(idleMessages)-1] = map[string]any{"role": "user", "content": strings.Repeat("Historical reference: keep previously returned tool results unchanged. ", 230) +
		"Reply with exactly the result string returned by the tool."}
	payload["messages"] = idleMessages
	_, _, _ = call("before_idle_qualification", true)
	if _, err := manager.qualifyPromptCache(ctx, runtimePlan{Config: cfg}, key, *seed); err != nil {
		t.Fatalf("idle qualification: %v", err)
	}
	assistant, cached, prompt = call("after_idle_qualification", true)
	content, _ = assistant["content"].(string)
	if !strings.Contains(content, "TOOL_RESULT_437") || cached*100 < prompt*80 {
		t.Fatalf("idle qualification displaced the conversation: %d/%d", cached, prompt)
	}
	payload["messages"] = messages
	streamed, err := manager.runPromptCacheProbe(ctx, runtimePlan{Config: cfg}, key, *seed, true)
	if err != nil || !validPromptCacheProbeCall(streamed.ToolCall) {
		t.Fatalf("streaming cache probe: %v", err)
	}
	t.Logf("streaming: cached=%d prompt=%d ttft_ms=%d", streamed.CachedTokens, streamed.PromptTokens, streamed.TTFT.Milliseconds())

	// Qwen regression: regeneration excludes the previous generated answer.
	// Its cache must prefer matching input tokens over a helper's retained ratio.
	if model == "aurago-qwen" {
		retryMessages := []map[string]any{messages[0],
			{"role": "user", "content": "Write ten short sentences about prompt caching. Start your answer with CACHE_A."}}
		payload["max_tokens"] = 512
		payload["messages"] = retryMessages
		assistant, _, _ = call("long_answer", true)
		content, _ = assistant["content"].(string)
		if !strings.Contains(content, "CACHE_A") || len(content) < 200 {
			t.Fatal("expected a complete longer answer before the helper")
		}
		payload["messages"] = []map[string]any{
			{"role": "system", "content": "Answer with exactly CACHE_B."},
			{"role": "user", "content": "What is the required answer?"}}
		_, _, _ = call("short_helper", true)
		payload["messages"] = retryMessages
		assistant, cached, prompt = call("retry_after_helper", true)
		content, _ = assistant["content"].(string)
		if !strings.Contains(content, "CACHE_A") || cached*100 < prompt*80 {
			t.Errorf("regeneration after helper lost the longer cache: %d/%d", cached, prompt)
		}
		payload["max_tokens"] = 128
	}

	longPrefix := "Remember ALPHA = ALPHA_417.\n" + strings.Repeat("Reference: stored files remain local; tools return structured results; never invent a tool result.\n", 325) +
		"Remember BETA = BETA_619.\n" + strings.Repeat("Reference: stored files remain local; tools return structured results; never invent a tool result.\n", 325)
	longMessages := []map[string]any{
		{"role": "system", "content": longPrefix + "# TURN CONTEXT\nReturn remembered facts accurately."},
		{"role": "user", "content": "Return the values of ALPHA and BETA, and nothing else."},
	}
	payload["messages"] = longMessages
	for _, phase := range []string{"long_cold", "long_warm"} {
		assistant, cached, prompt = call(phase, phase == "long_warm")
		content, _ = assistant["content"].(string)
		if !strings.Contains(content, "ALPHA_417") || !strings.Contains(content, "BETA_619") || prompt < 12000 || prompt > 16384 {
			t.Errorf("%s: long-context facts/context failed (tokens=%d)", phase, prompt)
		}
		if phase == "long_warm" && cached*100 < prompt*90 {
			t.Errorf("long prefix reuse below 90%%: %d/%d", cached, prompt)
		}
	}
	// Cancel an actual streaming generation, then reuse the same long prefix.
	cancelPayload := make(map[string]any, len(payload))
	for k, v := range payload {
		cancelPayload[k] = v
	}
	cancelPayload["messages"] = []map[string]any{longMessages[0], {"role": "user", "content": "Write a long explanation about prompt caching."}}
	cancelPayload["stream"] = true
	cancelPayload["max_tokens"] = 256
	encoded, _ := json.Marshal(cancelPayload)
	cancelCtx, cancelStream := context.WithCancel(ctx)
	req, _ := http.NewRequestWithContext(cancelCtx, http.MethodPost, cfg.Endpoint(false)+"/chat/completions", bytes.NewReader(encoded))
	req, _, _, _, err = preparePromptCacheRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	req, err = enablePromptCacheRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		cancelStream()
		t.Fatal(err)
	}
	buffer := make([]byte, 256)
	_, readErr := response.Body.Read(buffer)
	cancelStream()
	response.Body.Close()
	if response.StatusCode != http.StatusOK || readErr != nil {
		t.Fatal("stream did not start before cancellation")
	}
	assistant, cached, prompt = call("after_cancel", true)
	content, _ = assistant["content"].(string)
	if !strings.Contains(content, "ALPHA_417") || !strings.Contains(content, "BETA_619") || cached*100 < prompt*80 {
		t.Errorf("cancel/resume lost context or cache: %d/%d", cached, prompt)
	}
}
