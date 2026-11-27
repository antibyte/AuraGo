package chatrt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"aurago/internal/security"
)

const (
	chatModelAlias = "chibi" // the --alias of every instance
	maxErrorBody   = 16 << 10
)

// newLoopbackClient builds the client for one loopback child. Redirects are never followed, so the bearer key
// cannot leave the loopback origin, and no proxy is used. Keep-alive is off: llama-server closes idle
// connections on its own schedule, and a POST on a connection it just closed fails with EOF (seen in the b9994
// discovery run); a fresh loopback connection per request costs nothing.
func newLoopbackClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			ResponseHeaderTimeout: 5 * time.Minute,
			DisableKeepAlives:     true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// chatBody is the JSON body of one streamed chat completion.
type chatBody struct {
	Model              string          `json:"model"`
	Messages           []Message       `json:"messages"`
	Stream             bool            `json:"stream"`
	StreamOptions      map[string]bool `json:"stream_options"`
	Temperature        float64         `json:"temperature"`
	TopP               float64         `json:"top_p"`
	TopK               int             `json:"top_k"`
	RepeatPenalty      float64         `json:"repeat_penalty"`
	PresencePenalty    float64         `json:"presence_penalty"`
	MaxTokens          int             `json:"max_tokens"`
	Seed               *int64          `json:"seed,omitempty"`
	Stop               []string        `json:"stop,omitempty"`
	ChatTemplateKwargs map[string]any  `json:"chat_template_kwargs"`
	CachePrompt        bool            `json:"cache_prompt"`
}

// buildChatBody maps a request to the wire body. Thinking is always off (decision D4): the trainer used the
// non-thinking prompt, so the runtime must too.
func buildChatBody(req ChatRequest) chatBody {
	body := chatBody{
		Model:              chatModelAlias,
		Messages:           req.Messages,
		Stream:             true,
		StreamOptions:      map[string]bool{"include_usage": true},
		Temperature:        req.Sampling.Temperature,
		TopP:               req.Sampling.TopP,
		TopK:               req.Sampling.TopK,
		RepeatPenalty:      req.Sampling.RepeatPenalty,
		PresencePenalty:    req.Sampling.PresencePenalty,
		MaxTokens:          req.Sampling.MaxTokens,
		Stop:               req.Stop,
		ChatTemplateKwargs: map[string]any{"enable_thinking": false},
		CachePrompt:        true,
	}
	if req.Sampling.Seed > 0 {
		seed := req.Sampling.Seed
		body.Seed = &seed
	}
	return body
}

// streamChat POSTs the request to a loopback llama-server and forwards the answer text. A non-200 answer is
// decoded and classified; cancelling ctx closes the connection, which ends the generation upstream.
func streamChat(ctx context.Context, c *http.Client, baseURL, key string, req ChatRequest, onDelta func(string)) (ChatStats, error) {
	raw, err := json.Marshal(buildChatBody(req))
	if err != nil {
		return ChatStats{}, fmt.Errorf("encode chat request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return ChatStats{}, fmt.Errorf("build chat request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+key)
	started := time.Now()
	resp, err := c.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return ChatStats{}, ctx.Err()
		}
		return ChatStats{}, fmt.Errorf("chat request to llama-server: %s", security.Scrub(err.Error()))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ChatStats{}, decodeErrorAnswer(resp)
	}
	var firstDelta time.Time
	res, err := parseStream(resp.Body, func(text string) {
		if firstDelta.IsZero() {
			firstDelta = time.Now()
		}
		if onDelta != nil {
			onDelta(text)
		}
	})
	if err != nil {
		if ctx.Err() != nil {
			return ChatStats{}, ctx.Err()
		}
		return ChatStats{}, err
	}
	return finishStats(res, started, firstDelta), nil
}

// decodeErrorAnswer reads at most maxErrorBody bytes of a non-200 answer and classifies it.
func decodeErrorAnswer(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	var wrapper struct {
		Error upstreamError `json:"error"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil || (wrapper.Error.Message == "" && wrapper.Error.Type == "") {
		return fmt.Errorf("%w: status %d", errUpstream, resp.StatusCode)
	}
	return classifyUpstream(resp.StatusCode, wrapper.Error)
}

func finishStats(res streamResult, started, firstDelta time.Time) ChatStats {
	now := time.Now()
	stats := ChatStats{
		FinishReason:     res.FinishReason,
		PromptTokens:     res.PromptTokens,
		CompletionTokens: res.CompletionTokens,
		MS:               now.Sub(started).Milliseconds(),
		TokensPerSecond:  res.PerSecond,
	}
	if stats.FinishReason == "" {
		stats.FinishReason = "stop"
	}
	if stats.CompletionTokens == 0 {
		stats.CompletionTokens = res.Deltas
	}
	if stats.TokensPerSecond == 0 && stats.CompletionTokens > 0 && !firstDelta.IsZero() {
		if seconds := now.Sub(firstDelta).Seconds(); seconds > 0 {
			stats.TokensPerSecond = float64(stats.CompletionTokens) / seconds
		}
	}
	return stats
}
