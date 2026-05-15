package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// opencodeGoTransport is an http.RoundTripper for OpenCode Go.
//
// OpenCode Go serves models through two different API formats depending on the
// model family:
//   - Most models use OpenAI-compatible /v1/chat/completions (Bearer auth)
//   - MiniMax models use Anthropic-compatible /v1/messages (x-api-key auth)
//
// This transport inspects the request body, determines the model, and routes
// to the correct endpoint + auth scheme. The API key is read from the
// incoming Authorization header (set by go-openai from the client config).
type opencodeGoTransport struct {
	base http.RoundTripper
}

const opencodeGoBaseURL = "https://opencode.ai/zen/go"

func (t *opencodeGoTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()

	// Extract API key from the Authorization header set by go-openai
	apiKey := ""
	if auth := req.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		apiKey = auth[len("Bearer "):]
	}

	// Read body once so we can inspect the model and later rewrite it.
	var body []byte
	if req.Body != nil && req.Method == http.MethodPost {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("opencode-go transport: read body: %w", err)
		}
		req.Body.Close()
	}

	// Determine model from body
	model := ""
	if len(body) > 0 {
		var payload struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &payload)
		model = payload.Model
	}
	isMiniMax := strings.HasPrefix(strings.ToLower(model), "minimax-")

	if isMiniMax {
		// Anthropic format: x-api-key + anthropic-version header
		clone.Header.Set("x-api-key", apiKey)
		clone.Header.Set("anthropic-version", "2023-06-01")
		clone.Header.Set("Content-Type", "application/json")
		// Remove Bearer header (not used by Anthropic endpoint)
		clone.Header.Del("Authorization")

		// Rewrite path to /v1/messages
		if strings.HasSuffix(clone.URL.Path, "/chat/completions") {
			clone.URL.Path = strings.TrimSuffix(clone.URL.Path, "/chat/completions") + "/messages"
			slog.Debug("[OpenCode-Go] Routed MiniMax model to /v1/messages", "model", model)
		}

		// Rewrite body from OpenAI to Anthropic format
		newBody := t.rewriteToAnthropic(body)
		clone.Body = io.NopCloser(bytes.NewReader(newBody))
		clone.ContentLength = int64(len(newBody))
	} else {
		// OpenAI format: standard Bearer auth (already present)
		clone.Header.Set("Content-Type", "application/json")
		if len(body) > 0 {
			clone.Body = io.NopCloser(bytes.NewReader(body))
			clone.ContentLength = int64(len(body))
		}
	}

	return t.baseTransport().RoundTrip(clone)
}

// rewriteToAnthropic converts an OpenAI-format request body to Anthropic
// Messages API format. This is a lightweight conversion; for full feature
// parity (tools, vision, streaming) the anthropicTransport should be reused.
func (t *opencodeGoTransport) rewriteToAnthropic(body []byte) []byte {
	if len(body) == 0 {
		return body
	}

	var openaiReq struct {
		Model     string                   `json:"model"`
		Messages  []map[string]interface{} `json:"messages"`
		Stream    bool                     `json:"stream,omitempty"`
		MaxTokens int                      `json:"max_tokens,omitempty"`
	}
	if err := json.Unmarshal(body, &openaiReq); err != nil {
		// Not parseable — pass through unchanged
		return body
	}

	// Build Anthropic-style request
	anthropicReq := map[string]interface{}{
		"model": openaiReq.Model,
	}
	if openaiReq.MaxTokens > 0 {
		anthropicReq["max_tokens"] = openaiReq.MaxTokens
	} else {
		anthropicReq["max_tokens"] = 4096
	}
	if openaiReq.Stream {
		anthropicReq["stream"] = true
	}

	// Convert messages: system → system string, rest → messages array
	var systemParts []string
	var messages []map[string]interface{}
	for _, m := range openaiReq.Messages {
		role, _ := m["role"].(string)
		switch role {
		case "system":
			if content, ok := m["content"].(string); ok {
				systemParts = append(systemParts, content)
			}
		case "user", "assistant":
			messages = append(messages, m)
		}
	}
	if len(systemParts) > 0 {
		anthropicReq["system"] = strings.Join(systemParts, "\n\n")
	}
	anthropicReq["messages"] = messages

	newBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return body
	}
	return newBody
}

func (t *opencodeGoTransport) baseTransport() http.RoundTripper {
	if t.base != nil {
		return t.base
	}
	return http.DefaultTransport
}
