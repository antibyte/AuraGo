package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// agnesThinkingTransport enables Agnes Thinking mode for every OpenAI-compatible
// chat completion request. The shared client factory installs it for both main
// and helper Agnes providers.
type agnesThinkingTransport struct {
	base http.RoundTripper
}

func (t *agnesThinkingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body == nil || req.Method != http.MethodPost ||
		!strings.HasSuffix(req.URL.Path, "/chat/completions") {
		return t.baseTransport().RoundTrip(req)
	}

	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("agnes thinking transport: read request body: %w", err)
	}

	body, err = enableAgnesThinking(body)
	if err != nil {
		return nil, fmt.Errorf("agnes thinking transport: enable thinking: %w", err)
	}

	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Body = io.NopCloser(bytes.NewReader(body))
	clone.ContentLength = int64(len(body))
	clone.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}

	return t.baseTransport().RoundTrip(clone)
}

func (t *agnesThinkingTransport) baseTransport() http.RoundTripper {
	if t.base != nil {
		return t.base
	}
	return http.DefaultTransport
}

func enableAgnesThinking(body []byte) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse chat completion request: %w", err)
	}

	templateOptions := make(map[string]json.RawMessage)
	if rawOptions, exists := payload["chat_template_kwargs"]; exists &&
		len(rawOptions) > 0 && !bytes.Equal(bytes.TrimSpace(rawOptions), []byte("null")) {
		if err := json.Unmarshal(rawOptions, &templateOptions); err != nil {
			return nil, fmt.Errorf("chat_template_kwargs must be an object: %w", err)
		}
	}

	templateOptions["enable_thinking"] = json.RawMessage("true")
	rawOptions, err := json.Marshal(templateOptions)
	if err != nil {
		return nil, fmt.Errorf("marshal chat_template_kwargs: %w", err)
	}
	payload["chat_template_kwargs"] = rawOptions

	result, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal chat completion request: %w", err)
	}
	return result, nil
}
