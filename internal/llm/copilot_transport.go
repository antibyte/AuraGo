package llm

import (
	"fmt"
	"net/http"
	"regexp"
)

// Protocol selection belongs to the client adapter, before SDK validation.
// This transport retains Copilot authentication for both Chat and Responses.
type copilotTransport struct {
	base http.RoundTripper
	auth *CopilotAuth
}

var CopilotResponsesOnlyRe = regexp.MustCompile(`^(?:gpt-5(?:\.\d+)?-codex|codex-)`)

const (
	CopilotEditorVersion = "vscode/1.100.0"
	CopilotPluginVersion = "copilot/1.300.0"
	CopilotIntegrationID = "vscode-chat"
	CopilotBaseURL       = "https://api.githubcopilot.com"
)

func (t *copilotTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.auth == nil {
		return nil, completionConfigError("copilot transport: no auth manager configured")
	}
	token, err := t.auth.GetToken()
	if err != nil {
		return nil, fmt.Errorf("copilot transport: failed to get token: %w", err)
	}
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Header.Set("Authorization", "Bearer "+token)
	clone.Header.Set("Content-Type", "application/json")
	clone.Header.Set("Editor-Version", CopilotEditorVersion)
	clone.Header.Set("Editor-Plugin-Version", CopilotPluginVersion)
	clone.Header.Set("Copilot-Integration-Id", CopilotIntegrationID)
	if clone.Header.Get("Accept") == "" {
		clone.Header.Set("Accept", "application/json")
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
}
