package llm

import (
	"encoding/json"
	"fmt"
	"io"
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
// MiniMax requests are delegated to anthropicTransport for full bidirectional
// translation (tools, vision, streaming, response mapping).
type opencodeGoTransport struct {
	base http.RoundTripper
}

const opencodeGoBaseURL = "https://opencode.ai/zen/go"

func (t *opencodeGoTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	isChatCompletion := req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/chat/completions")
	if !isChatCompletion || req.Body == nil {
		return t.baseTransport().RoundTrip(req)
	}

	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("opencode-go transport: read body: %w", err)
	}

	model := extractRequestModel(body)
	if !strings.HasPrefix(strings.ToLower(model), "minimax-") {
		clone := req.Clone(req.Context())
		clone.Header = req.Header.Clone()
		clone.Body = io.NopCloser(strings.NewReader(string(body)))
		clone.ContentLength = int64(len(body))
		return t.baseTransport().RoundTrip(clone)
	}

	apiKey := ""
	if auth := req.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		apiKey = auth[len("Bearer "):]
	}

	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Header.Set("x-api-key", apiKey)
	clone.Header.Set("anthropic-version", anthropicAPIVersion)
	clone.Header.Set("Content-Type", "application/json")
	clone.Header.Del("Authorization")
	clone.Body = io.NopCloser(strings.NewReader(string(body)))
	clone.ContentLength = int64(len(body))

	at := &anthropicTransport{base: t.base}
	return at.RoundTrip(clone)
}

func extractRequestModel(body []byte) string {
	var payload struct {
		Model string `json:"model"`
	}
	if len(body) > 0 {
		_ = json.Unmarshal(body, &payload)
	}
	return payload.Model
}

func (t *opencodeGoTransport) baseTransport() http.RoundTripper {
	if t.base != nil {
		return t.base
	}
	return http.DefaultTransport
}