package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// StepFun rejects omitted content on assistant tool calls and nonstandard
// reasoning_content fields. Keep the agent's private history intact and adapt
// only the wire payload. Responses use the SDK's private reasoning field.
type stepFunChatMessageTransport struct {
	base   http.RoundTripper
	direct bool
}

func isStepFunAPIBaseURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "api.stepfun.ai", "api.stepfun.com":
		return true
	default:
		return false
	}
}

func (t *stepFunChatMessageTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	if req.Method != http.MethodPost || !strings.HasSuffix(req.URL.Path, "/chat/completions") || req.Body == nil {
		return base.RoundTrip(req)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("stepfun messages: read request: %w", err)
	}
	if err := req.Body.Close(); err != nil {
		return nil, fmt.Errorf("stepfun messages: close request: %w", err)
	}
	body, err = normalizeStepFunMessages(body, t.direct)
	if err != nil {
		return nil, fmt.Errorf("stepfun messages: normalize request: %w", err)
	}
	req = req.Clone(req.Context())
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	return base.RoundTrip(req)
}

func normalizeStepFunMessages(body []byte, direct bool) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if !direct {
		var model string
		if err := json.Unmarshal(payload["model"], &model); err != nil {
			return nil, err
		}
		model = strings.ToLower(model)
		if !strings.HasPrefix(model, "stepfun/") && !strings.HasPrefix(model, "stepfun-ai/") {
			return body, nil
		}
	}
	var messages []map[string]json.RawMessage
	if err := json.Unmarshal(payload["messages"], &messages); err != nil {
		return nil, err
	}
	clean := make([]map[string]json.RawMessage, 0, len(messages))
	for _, message := range messages {
		var role string
		if err := json.Unmarshal(message["role"], &role); err != nil {
			return nil, err
		}
		delete(message, "reasoning_content")
		switch role {
		case "assistant":
			_, hasContent := message["content"]
			_, hasCalls := message["tool_calls"]
			if !hasContent && !hasCalls {
				continue // A reasoning-only or empty completion has no replayable message.
			}
			if !hasContent {
				message["content"] = json.RawMessage("null")
			}
			if hasCalls {
				var calls []map[string]json.RawMessage
				if err := json.Unmarshal(message["tool_calls"], &calls); err != nil {
					return nil, err
				}
				for _, call := range calls {
					delete(call, "index") // Stream chunk position is not a request field.
				}
				encodedCalls, err := json.Marshal(calls)
				if err != nil {
					return nil, err
				}
				message["tool_calls"] = encodedCalls
			}
		case "tool":
			delete(message, "name")
			if _, hasContent := message["content"]; !hasContent {
				message["content"] = json.RawMessage(`""`)
			}
		}
		clean = append(clean, message)
	}
	encodedMessages, err := json.Marshal(clean)
	if err != nil {
		return nil, err
	}
	payload["messages"] = encodedMessages
	return json.Marshal(payload)
}

// StepFun defaults to "reasoning"; go-openai decodes "reasoning_content".
// Map both ordinary and SSE responses, including chunks without usage. Never
// turn reasoning into visible content or concatenate duplicate aliases.
func normalizeStepFunReasoning(payload map[string]json.RawMessage) bool {
	var choices []map[string]json.RawMessage
	if json.Unmarshal(payload["choices"], &choices) != nil {
		return false
	}
	changed := false
	for _, choice := range choices {
		for _, key := range []string{"message", "delta"} {
			var message map[string]json.RawMessage
			if json.Unmarshal(choice[key], &message) != nil || message == nil {
				continue
			}
			var reasoning, canonical string
			if json.Unmarshal(message["reasoning"], &reasoning) != nil || reasoning == "" {
				continue
			}
			if json.Unmarshal(message["reasoning_content"], &canonical) != nil || canonical == "" {
				message["reasoning_content"] = message["reasoning"]
			}
			delete(message, "reasoning")
			choice[key], _ = json.Marshal(message)
			changed = true
		}
	}
	if changed {
		payload["choices"], _ = json.Marshal(choices)
	}
	return changed
}
