package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
)

// copilotTransport is an http.RoundTripper that adds the mandatory Copilot
// headers and transparently exchanges the stored GitHub token for a short-lived
// Copilot token on every request.
//
// It also handles two Copilot-specific quirks:
//  1. Codex model variants are routed to /responses instead of /chat/completions
//  2. The "copilot/" model prefix (added during discovery) is stripped before
//     forwarding to the API.
type copilotTransport struct {
	base http.RoundTripper
	auth *CopilotAuth
}

var (
	// CopilotResponsesOnlyRe matches OpenAI responses-only models served by Copilot (Codex variants).
	CopilotResponsesOnlyRe = regexp.MustCompile(`^(?:gpt-5(?:\.\d+)?-codex|codex-)`)
)

const (
	CopilotEditorVersion      = "vscode/1.100.0"
	CopilotPluginVersion      = "copilot/1.300.0"
	CopilotIntegrationID      = "vscode-chat"
	CopilotBaseURL            = "https://api.githubcopilot.com"
)

func (t *copilotTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.auth == nil {
		return nil, fmt.Errorf("copilot transport: no auth manager configured")
	}

	// 1. Obtain a valid Copilot token (refreshing if necessary)
	token, err := t.auth.GetToken()
	if err != nil {
		return nil, fmt.Errorf("copilot transport: failed to get token: %w", err)
	}

	// 2. Clone the request so we can mutate it safely
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()

	// 3. Set mandatory Copilot headers
	clone.Header.Set("Authorization", "Bearer "+token)
	clone.Header.Set("Content-Type", "application/json")
	clone.Header.Set("Editor-Version", CopilotEditorVersion)
	clone.Header.Set("Editor-Plugin-Version", CopilotPluginVersion)
	clone.Header.Set("Copilot-Integration-Id", CopilotIntegrationID)
	clone.Header.Set("Accept", "application/json")

	// 4. Rewrite URL if necessary (Codex → /responses, strip copilot/ prefix)
	routeInfo, err := t.rewriteRequest(clone)
	if err != nil {
		return nil, err
	}

	resp, err := t.baseTransport().RoundTrip(clone)
	if err != nil {
		return nil, err
	}
	if routeInfo.responsesRoute {
		return translateCopilotResponsesResponse(resp, routeInfo.model, routeInfo.stream)
	}
	return resp, nil
}

type copilotRouteInfo struct {
	responsesRoute bool
	model          string
	stream         bool
}

// rewriteRequest handles Copilot-specific request rewriting:
//   - Routes Codex models to /responses
//   - Strips the "copilot/" prefix from model IDs
//   - Forces streaming for /responses endpoint
func (t *copilotTransport) rewriteRequest(req *http.Request) (copilotRouteInfo, error) {
	info := copilotRouteInfo{}
	isChatCompletion := req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/chat/completions")
	if !isChatCompletion || req.Body == nil {
		return info, nil
	}

	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return info, fmt.Errorf("copilot transport: read body: %w", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return info, nil
	}

	modelRaw, ok := payload["model"]
	if !ok {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return info, nil
	}
	model, _ := modelRaw.(string)
	if model == "" {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return info, nil
	}

	if strings.HasPrefix(model, "copilot/") {
		model = model[len("copilot/"):]
		payload["model"] = model
		slog.Debug("[Copilot] Stripped model prefix", "model", model)
	}
	info.model = model

	if CopilotResponsesOnlyRe.MatchString(model) {
		req.URL.Path = strings.TrimSuffix(req.URL.Path, "/chat/completions") + "/responses"
		payload["stream"] = true
		info.responsesRoute = true
		info.stream = true
		req.Header.Set("Accept", "text/event-stream")
		slog.Debug("[Copilot] Routed Codex model to /responses", "model", model)
	}

	newBody, err := json.Marshal(payload)
	if err != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return info, nil
	}
	if info.responsesRoute {
		newBody = chatCompletionToResponsesBody(newBody)
	}
	req.Body = io.NopCloser(bytes.NewReader(newBody))
	req.ContentLength = int64(len(newBody))
	return info, nil
}

func (t *copilotTransport) baseTransport() http.RoundTripper {
	if t.base != nil {
		return t.base
	}
	return http.DefaultTransport
}
