package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// RequestUsage contains provider measurements only. Nil means not reported.
// InputTokens includes cache reads and writes; neither is free context capacity.
type RequestUsage struct {
	Provider         string   `json:"provider"`
	Model            string   `json:"model"`
	ServiceTier      string   `json:"service_tier,omitempty"`
	InputTokens      *int     `json:"input_tokens"`
	OutputTokens     *int     `json:"output_tokens"`
	CacheReadTokens  *int     `json:"cache_read_tokens"`
	CacheWriteTokens *int     `json:"cache_write_tokens"`
	DurationMS       int64    `json:"duration_ms"`
	Status           int      `json:"http_status,omitempty"`
	TransportError   bool     `json:"transport_error,omitempty"`
	EstimatedCostUSD *float64 `json:"estimated_cost_usd"`
}

type usageCaptureKey struct{}

// UsageCapture is request-local, including separate transport retry/failover attempts.
type UsageCapture struct {
	mu    sync.Mutex
	items []RequestUsage
}

func CaptureUsage(ctx context.Context) (context.Context, *UsageCapture) {
	c := &UsageCapture{}
	return context.WithValue(ctx, usageCaptureKey{}, c), c
}

func (c *UsageCapture) Snapshot() []RequestUsage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]RequestUsage(nil), c.items...)
}

// This wrapper sees OpenAI-shaped responses after provider translation. It is
// transparent when observation and StepFun compatibility are both unnecessary.
type usageObservationTransport struct {
	base          http.RoundTripper
	provider      string
	directStepFun bool
	pricedRoute   bool
}

func (t *usageObservationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	capture, _ := req.Context().Value(usageCaptureKey{}).(*UsageCapture)
	if req.Method != http.MethodPost || !strings.HasSuffix(req.URL.Path, "/chat/completions") || req.Body == nil || (capture == nil && !t.directStepFun && t.provider != "openrouter") {
		return t.base.RoundTrip(req)
	}
	var model string
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err == nil {
			var request map[string]json.RawMessage
			decodeErr := json.NewDecoder(body).Decode(&request)
			_ = body.Close()
			_ = json.Unmarshal(request["model"], &model)
			// OpenAI streams report usage only when explicitly requested. Keep
			// this opt-in limited to observed calls on the documented API route.
			if decodeErr == nil && capture != nil && t.provider == "openai" && t.pricedRoute && string(request["stream"]) == "true" {
				var options map[string]json.RawMessage
				_ = json.Unmarshal(request["stream_options"], &options)
				if options == nil {
					options = map[string]json.RawMessage{}
				}
				options["include_usage"] = json.RawMessage("true")
				request["stream_options"], _ = json.Marshal(options)
				encoded, marshalErr := json.Marshal(request)
				if marshalErr == nil {
					_ = req.Body.Close()
					req = req.Clone(req.Context())
					req.Body = io.NopCloser(bytes.NewReader(encoded))
					req.ContentLength = int64(len(encoded))
					req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(encoded)), nil }
				}
			}
		}
	}
	stepFun := t.directStepFun || t.provider == "openrouter" && (strings.HasPrefix(model, "stepfun/") || strings.HasPrefix(model, "stepfun-ai/"))
	if capture == nil && !stepFun {
		return t.base.RoundTrip(req)
	}
	started := time.Now()
	usage := RequestUsage{Provider: t.provider, Model: model}
	var usageMu sync.Mutex
	var once sync.Once
	finish := func() {
		once.Do(func() {
			usageMu.Lock()
			defer usageMu.Unlock()
			if capture == nil {
				return
			}
			usage.DurationMS = time.Since(started).Milliseconds()
			if t.pricedRoute {
				usage.EstimatedCostUSD = estimateMeasuredCost(usage)
			}
			capture.mu.Lock()
			capture.items = append(capture.items, usage)
			capture.mu.Unlock()
		})
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		usage.TransportError = true
		finish()
		return resp, err
	}
	usage.Status = resp.StatusCode
	if resp.Body == nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		finish()
		return resp, nil
	}
	consume := func(data []byte) []byte {
		usageMu.Lock()
		defer usageMu.Unlock()
		return normalizeObservedUsage(data, stepFun, &usage)
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		resp.Body = &usageStreamBody{source: resp.Body, reader: bufio.NewReader(resp.Body), consume: consume, finish: finish}
		resp.ContentLength = -1
		resp.Header.Del("Content-Length")
		return resp, nil
	}
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024+1))
	_ = resp.Body.Close()
	if readErr == nil && len(data) > 32*1024*1024 {
		readErr = fmt.Errorf("LLM response exceeds 32 MiB")
	}
	if readErr != nil {
		usage.TransportError = true
		finish()
		return nil, readErr
	}
	data = consume(data)
	finish()
	resp.Body = io.NopCloser(bytes.NewReader(data))
	resp.ContentLength = int64(len(data))
	resp.Header.Del("Content-Length")
	return resp, nil
}

type usageStreamBody struct {
	source  io.ReadCloser
	reader  *bufio.Reader
	pending []byte
	consume func([]byte) []byte
	finish  func()
}

func (b *usageStreamBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(b.pending) == 0 {
		// Bound a single event, never buffer the entire generation or start a goroutine.
		var line []byte
		for {
			part, err := b.reader.ReadSlice('\n')
			line = append(line, part...)
			if len(line) > 4*1024*1024 {
				b.finish()
				return 0, fmt.Errorf("LLM stream event exceeds 4 MiB")
			}
			if err == bufio.ErrBufferFull {
				continue
			}
			if err != nil && len(line) == 0 {
				b.finish()
				return 0, err
			}
			break
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			data := bytes.TrimSpace(line[5:])
			if !bytes.Equal(data, []byte("[DONE]")) {
				line = append(append([]byte("data: "), b.consume(data)...), '\n')
			}
		}
		b.pending = line
	}
	n := copy(p, b.pending)
	b.pending = b.pending[n:]
	return n, nil
}

func (b *usageStreamBody) Close() error { err := b.source.Close(); b.finish(); return err }

func normalizeObservedUsage(data []byte, stepFun bool, result *RequestUsage) []byte {
	var payload map[string]json.RawMessage
	if json.Unmarshal(data, &payload) != nil {
		return data
	}
	var model string
	if json.Unmarshal(payload["model"], &model) == nil && model != "" {
		result.Model = model
	}
	var tier string
	if json.Unmarshal(payload["service_tier"], &tier) == nil && tier != "" {
		result.ServiceTier = tier
	}
	var usage map[string]json.RawMessage
	if json.Unmarshal(payload["usage"], &usage) != nil || usage == nil {
		return data
	}
	readCount := func(raw json.RawMessage) *int {
		var n *int
		if json.Unmarshal(raw, &n) != nil || n == nil || *n < 0 || *n > 100_000_000 {
			return nil
		}
		return n
	}
	merge := func(dst **int, n *int) {
		if n != nil && (*dst == nil || *n > **dst) {
			*dst = n
		}
	}
	merge(&result.InputTokens, readCount(usage["prompt_tokens"]))
	merge(&result.OutputTokens, readCount(usage["completion_tokens"]))
	var details map[string]json.RawMessage
	_ = json.Unmarshal(usage["prompt_tokens_details"], &details)
	read := readCount(details["cached_tokens"])
	if stepFun {
		if n := readCount(usage["cached_tokens"]); n != nil {
			read = n
			if details == nil {
				details = map[string]json.RawMessage{}
			}
			details["cached_tokens"] = usage["cached_tokens"]
			usage["prompt_tokens_details"], _ = json.Marshal(details)
			payload["usage"], _ = json.Marshal(usage)
			data, _ = json.Marshal(payload)
		}
	}
	merge(&result.CacheReadTokens, read)
	merge(&result.CacheWriteTokens, readCount(usage["cache_creation_input_tokens"]))
	return data
}

// Exact registry prices only. Missing cache measurements/prices and subscription
// or custom endpoints do not support a monetary estimate.
func estimateMeasuredCost(u RequestUsage) *float64 {
	if u.ServiceTier != "" && u.ServiceTier != "default" {
		return nil // The registry has no prices for priority/flex or other tiers.
	}
	if u.InputTokens == nil || u.OutputTokens == nil || u.CacheReadTokens == nil {
		return nil
	}
	info, ok := GetModelInfo(u.Provider, u.Model)
	if !ok || !strings.EqualFold(info.ID, u.Model) || !strings.EqualFold(info.Provider, u.Provider) || info.InputPricePer1M <= 0 || info.OutputPricePer1M <= 0 {
		return nil
	}
	writes := 0
	if u.CacheWriteTokens != nil {
		writes = *u.CacheWriteTokens
	} else if u.Provider == "anthropic" {
		return nil
	}
	reads := *u.CacheReadTokens
	if reads+writes > *u.InputTokens || reads > 0 && info.CacheReadPricePer1M <= 0 || writes > 0 && info.CacheWritePricePer1M <= 0 {
		return nil
	}
	cost := (float64(*u.InputTokens-reads-writes)*info.InputPricePer1M + float64(*u.OutputTokens)*info.OutputPricePer1M + float64(reads)*info.CacheReadPricePer1M + float64(writes)*info.CacheWritePricePer1M) / 1e6
	return &cost
}

func knownMeteredUsageRoute(provider, baseURL string) bool {
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme != "https" {
		return false
	}
	switch provider {
	case "openai":
		return u.Host == "api.openai.com" && strings.TrimRight(u.Path, "/") == "/v1"
	case "anthropic":
		return u.Host == "api.anthropic.com" && (u.Path == "" || strings.TrimRight(u.Path, "/") == "/v1")
	case "stepfun":
		return (u.Host == "api.stepfun.ai" || u.Host == "api.stepfun.com") && strings.TrimRight(u.Path, "/") == "/v1"
	}
	return false
}
