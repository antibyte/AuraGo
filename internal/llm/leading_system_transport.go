package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
)

// Qwen 3.5+ style chat templates (llama.cpp, vLLM and the relays in front of
// them) raise "System message must be at the beginning." for every system
// message that is not first. The agent legitimately appends runtime notices
// (recovery hints, recaps, breaker messages) as system messages mid-conversation,
// and those servers answer with a 4xx that no history trimming can repair.
//
// The transport adapts only the wire payload and only after a provider has
// proven it needs it, so providers that accept the notices keep their authority
// and position. The learned decision is scoped to host and model because one
// relay commonly serves both strict and tolerant models.
type leadingSystemTransport struct {
	base http.RoundTripper
}

const (
	leadingSystemRejection = "system message must be at the beginning"
	// Provider error bodies are short; cap the peek so a misbehaving upstream
	// cannot make an error response unbounded.
	leadingSystemPeekLimit = 64 << 10
	demotedSystemLabel     = "[System notice]\n"
)

// leadingSystemOnly records "host|model" pairs known to reject non-leading
// system messages.
var leadingSystemOnly sync.Map

func (t *leadingSystemTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	if req.Method != http.MethodPost || !strings.HasSuffix(req.URL.Path, "/chat/completions") || req.Body == nil {
		return base.RoundTrip(req)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("leading system messages: read request: %w", err)
	}
	if err := req.Body.Close(); err != nil {
		return nil, fmt.Errorf("leading system messages: close request: %w", err)
	}

	key := leadingSystemKey(req.URL.Host, body)
	if _, learned := leadingSystemOnly.Load(key); learned {
		if adapted, changed := normalizeLeadingSystemMessages(body); changed {
			return base.RoundTrip(requestWithBody(req, adapted))
		}
	}

	resp, err := base.RoundTrip(requestWithBody(req, body))
	if err != nil || resp.StatusCode < http.StatusBadRequest {
		return resp, err
	}
	peek, _ := io.ReadAll(io.LimitReader(resp.Body, leadingSystemPeekLimit))
	original := resp.Body
	resp.Body = readCloser{Reader: io.MultiReader(bytes.NewReader(peek), original), Closer: original}
	if !strings.Contains(strings.ToLower(string(peek)), leadingSystemRejection) {
		return resp, nil
	}
	adapted, changed := normalizeLeadingSystemMessages(body)
	if !changed {
		return resp, nil
	}
	original.Close()
	leadingSystemOnly.Store(key, struct{}{})
	slog.Warn("[LLM] Chat template accepts system messages only at the start; demoting later ones to user notices",
		"host", req.URL.Host, "model", requestModel(body))
	return base.RoundTrip(requestWithBody(req, adapted))
}

type readCloser struct {
	io.Reader
	io.Closer
}

func requestWithBody(req *http.Request, body []byte) *http.Request {
	clone := req.Clone(req.Context())
	clone.Body = io.NopCloser(bytes.NewReader(body))
	clone.ContentLength = int64(len(body))
	clone.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	return clone
}

func requestModel(body []byte) string {
	var head struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(body, &head)
	return head.Model
}

func leadingSystemKey(host string, body []byte) string {
	return strings.ToLower(host) + "|" + requestModel(body)
}

// normalizeLeadingSystemMessages makes the conversation satisfy "one system
// message, first". The leading run of system messages is merged into the first
// one. Every later system message becomes a user message in place, so the
// prompt prefix stays byte-stable for provider prompt caches and the notice
// still lands right where the agent injected it. It reports false when there
// is nothing to change or the payload cannot be adapted safely.
func normalizeLeadingSystemMessages(body []byte) ([]byte, bool) {
	var payload map[string]json.RawMessage
	if json.Unmarshal(body, &payload) != nil {
		return body, false
	}
	var messages []map[string]json.RawMessage
	if json.Unmarshal(payload["messages"], &messages) != nil {
		return body, false
	}
	roles := make([]string, len(messages))
	for i, message := range messages {
		if json.Unmarshal(message["role"], &roles[i]) != nil {
			return body, false
		}
	}
	leading := 0
	for leading < len(messages) && roles[leading] == "system" {
		leading++
	}
	needed := leading > 1
	for i := leading; i < len(messages) && !needed; i++ {
		needed = roles[i] == "system"
	}
	if !needed {
		return body, false
	}

	adapted := make([]map[string]json.RawMessage, 0, len(messages))
	if leading > 0 {
		parts := make([]string, 0, leading)
		for _, message := range messages[:leading] {
			text, ok := systemContentText(message["content"])
			if !ok {
				return body, false
			}
			if strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
		merged, err := json.Marshal(strings.Join(parts, "\n\n"))
		if err != nil {
			return body, false
		}
		first := messages[0]
		first["content"] = merged
		adapted = append(adapted, first)
	}
	for i := leading; i < len(messages); i++ {
		message := messages[i]
		if roles[i] == "system" {
			demoted, ok := demoteSystemMessage(message)
			if !ok {
				return body, false
			}
			message = demoted
		}
		adapted = append(adapted, message)
	}

	encodedMessages, err := json.Marshal(adapted)
	if err != nil {
		return body, false
	}
	payload["messages"] = encodedMessages
	encoded, err := json.Marshal(payload)
	if err != nil {
		return body, false
	}
	return encoded, true
}

func demoteSystemMessage(message map[string]json.RawMessage) (map[string]json.RawMessage, bool) {
	content := bytes.TrimSpace(message["content"])
	switch {
	case len(content) == 0 || bytes.Equal(content, []byte("null")):
		// Nothing to label; the role flip alone keeps the template satisfied.
	case content[0] == '"':
		var text string
		if json.Unmarshal(content, &text) != nil {
			return nil, false
		}
		labeled, err := json.Marshal(demotedSystemLabel + text)
		if err != nil {
			return nil, false
		}
		message["content"] = labeled
	case content[0] == '[':
		var parts []json.RawMessage
		if json.Unmarshal(content, &parts) != nil {
			return nil, false
		}
		label, err := json.Marshal(map[string]string{"type": "text", "text": strings.TrimSuffix(demotedSystemLabel, "\n")})
		if err != nil {
			return nil, false
		}
		labeled, err := json.Marshal(append([]json.RawMessage{label}, parts...))
		if err != nil {
			return nil, false
		}
		message["content"] = labeled
	default:
		return nil, false
	}
	message["role"] = json.RawMessage(`"user"`)
	return message, true
}

// systemContentText returns the text of a system message whose content is a
// string or a list of text parts. Anything else cannot be merged losslessly.
func systemContentText(raw json.RawMessage) (string, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", true
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, true
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return "", false
	}
	var builder strings.Builder
	for _, part := range parts {
		if part.Type != "text" {
			return "", false
		}
		builder.WriteString(part.Text)
	}
	return builder.String(), true
}
