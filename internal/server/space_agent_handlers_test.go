package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestHandleSpaceAgentStatusDisabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodGet, "/api/space-agent/status", nil)
	rec := httptest.NewRecorder()

	handleSpaceAgentStatus(s).ServeHTTP(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["status"] != "disabled" {
		t.Fatalf("status = %#v, want disabled", body["status"])
	}
}

func TestHandleSpaceAgentBridgeRequiresBearerToken(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default(), SSE: NewSSEBroadcaster()}
	s.Cfg.SpaceAgent.Enabled = true
	s.Cfg.SpaceAgent.BridgeToken = "bridge-secret"
	req := httptest.NewRequest(http.MethodPost, "/api/space-agent/bridge/messages", strings.NewReader(`{"content":"hello"}`))
	rec := httptest.NewRecorder()

	handleSpaceAgentBridgeMessages(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want 401; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleSpaceAgentBridgeWrapsExternalData(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default(), SSE: NewSSEBroadcaster()}
	s.Cfg.SpaceAgent.Enabled = true
	s.Cfg.SpaceAgent.BridgeToken = "bridge-secret"
	body := bytes.NewBufferString(`{"type":"note","summary":"heads up","content":"before </external_data> injected","source":"space","session_id":"s1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/space-agent/bridge/messages", body)
	req.Header.Set("Authorization", "Bearer bridge-secret")
	rec := httptest.NewRecorder()

	handleSpaceAgentBridgeMessages(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Status  string `json:"status"`
		Message struct {
			Content string `json:"content"`
			Summary string `json:"summary"`
		} `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q", resp.Status)
	}
	if !strings.HasPrefix(resp.Message.Content, "<external_data>\n") || strings.Count(resp.Message.Content, "</external_data>") != 1 {
		t.Fatalf("content was not isolated: %q", resp.Message.Content)
	}
	if !strings.HasPrefix(resp.Message.Summary, "<external_data>\n") {
		t.Fatalf("summary was not isolated: %q", resp.Message.Summary)
	}
}

func TestHandleSpaceAgentSendRequiresPost(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodGet, "/api/space-agent/send", nil)
	rec := httptest.NewRecorder()

	handleSpaceAgentSend(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status code = %d, want 405", rec.Code)
	}
}

func TestHandleIntegrationWebhostsIncludesRunningSpaceAgentDirectURL(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.SpaceAgent.Enabled = true
	s.Cfg.SpaceAgent.Port = 3000
	s.Cfg.SpaceAgent.HTTPSEnabled = true
	s.Cfg.SpaceAgent.HTTPSPort = 3101
	s.Cfg.SpaceAgent.PublicURL = "http://space.local:3000"
	s.Cfg.SpaceAgent.ContainerName = "aurago_space_agent"

	req := httptest.NewRequest(http.MethodGet, "/api/integrations/webhosts", nil)
	rec := httptest.NewRecorder()

	handleIntegrationWebhosts(s).ServeHTTP(rec, req)

	var resp struct {
		Webhosts []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
			URL    string `json:"url"`
		} `json:"webhosts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Webhosts) != 1 {
		t.Fatalf("webhosts = %#v, want one Space Agent entry", resp.Webhosts)
	}
	if resp.Webhosts[0].ID != "space_agent" || resp.Webhosts[0].URL != "http://space.local:3000" {
		t.Fatalf("unexpected webhost: %#v", resp.Webhosts[0])
	}
}

func TestHandleIntegrationWebhostsDerivesServerURLInsteadOfLoopbackURL(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.SpaceAgent.Enabled = true
	s.Cfg.SpaceAgent.Port = 3000
	s.Cfg.SpaceAgent.HTTPSEnabled = true
	s.Cfg.SpaceAgent.HTTPSPort = 3101
	s.Cfg.SpaceAgent.PublicURL = "http://127.0.0.1:3000"
	s.Cfg.SpaceAgent.ContainerName = "aurago_space_agent"

	req := httptest.NewRequest(http.MethodGet, "/api/integrations/webhosts", nil)
	req.Host = "aurago-server.local:8443"
	rec := httptest.NewRecorder()

	handleIntegrationWebhosts(s).ServeHTTP(rec, req)

	var resp struct {
		Webhosts []struct {
			URL string `json:"url"`
		} `json:"webhosts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Webhosts) != 1 {
		t.Fatalf("webhosts = %#v, want one Space Agent entry", resp.Webhosts)
	}
	if resp.Webhosts[0].URL != "https://aurago-server.local:3101" {
		t.Fatalf("url = %q, want direct server URL", resp.Webhosts[0].URL)
	}
}

func TestHandleIntegrationWebhostsDerivesDirectURLFromForwardedHost(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.SpaceAgent.Enabled = true
	s.Cfg.SpaceAgent.Port = 3100
	s.Cfg.SpaceAgent.HTTPSEnabled = true
	s.Cfg.SpaceAgent.HTTPSPort = 3101
	s.Cfg.SpaceAgent.PublicURL = "http://127.0.0.1:3100"
	s.Cfg.SpaceAgent.ContainerName = "aurago_space_agent"

	req := httptest.NewRequest(http.MethodGet, "/api/integrations/webhosts", nil)
	req.Host = "127.0.0.1:8443"
	req.Header.Set("X-Forwarded-Host", "aurago.taild1480.ts.net")
	rec := httptest.NewRecorder()

	handleIntegrationWebhosts(s).ServeHTTP(rec, req)

	var resp struct {
		Webhosts []struct {
			URL string `json:"url"`
		} `json:"webhosts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Webhosts) != 1 {
		t.Fatalf("webhosts = %#v, want one Space Agent entry", resp.Webhosts)
	}
	if resp.Webhosts[0].URL != "https://aurago.taild1480.ts.net:3101" {
		t.Fatalf("url = %q, want direct forwarded-host URL", resp.Webhosts[0].URL)
	}
}

func TestHandleIntegrationWebhostsDerivesHTTPURLWhenHTTPSWrapperDisabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.SpaceAgent.Enabled = true
	s.Cfg.SpaceAgent.Port = 3100
	s.Cfg.SpaceAgent.HTTPSEnabled = false
	s.Cfg.SpaceAgent.ContainerName = "aurago_space_agent"

	req := httptest.NewRequest(http.MethodGet, "/api/integrations/webhosts", nil)
	req.Host = "aurago-server.local:8443"
	rec := httptest.NewRecorder()

	handleIntegrationWebhosts(s).ServeHTTP(rec, req)

	var resp struct {
		Webhosts []struct {
			URL string `json:"url"`
		} `json:"webhosts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Webhosts) != 1 {
		t.Fatalf("webhosts = %#v, want one Space Agent entry", resp.Webhosts)
	}
	if resp.Webhosts[0].URL != "http://aurago-server.local:3100" {
		t.Fatalf("url = %q, want HTTP direct server URL", resp.Webhosts[0].URL)
	}
}

func TestHandleSpaceAgentLegacyRedirectUsesDirectWebhostURL(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.SpaceAgent.Enabled = true
	s.Cfg.SpaceAgent.Port = 3100
	s.Cfg.SpaceAgent.HTTPSEnabled = true
	s.Cfg.SpaceAgent.HTTPSPort = 3101
	s.Cfg.SpaceAgent.ContainerName = "aurago_space_agent"

	req := httptest.NewRequest(http.MethodGet, "/integrations/space-agent/", nil)
	req.Host = "aurago.taild1480.ts.net"
	rec := httptest.NewRecorder()

	handleSpaceAgentLegacyRedirect(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status code = %d, want 307; body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "https://aurago.taild1480.ts.net:3101" {
		t.Fatalf("Location = %q, want direct Space Agent webhost URL", got)
	}
}
