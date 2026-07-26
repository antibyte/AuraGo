package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/manus"
)

func TestHandleManusStatusNeverReturnsAPIKey(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Manus: config.ManusConfig{Enabled: true, ReadOnly: true, APIKey: "never-return-me", AllowedProjectIDs: []string{"project-1"}}}
	recorder := httptest.NewRecorder()
	handleManusStatus(&Server{Cfg: cfg})(recorder, httptest.NewRequest(http.MethodGet, "/api/manus/status", nil))
	if recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), "never-return-me") {
		t.Fatalf("status response = %d %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["configured"] != true || body["read_only"] != true || body["allowed_project_count"] != float64(1) {
		t.Fatalf("status body = %#v", body)
	}
}

func TestHandleManusTestReturnsAuthoritativeCredits(t *testing.T) {
	t.Parallel()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"total_credits":77,"refresh_interval":"weekly"}}`))
	}))
	defer api.Close()
	client, _ := manus.NewClient("secret", manus.ClientConfig{BaseURL: api.URL})
	recorder := httptest.NewRecorder()
	handleManusTestWithClient(client)(recorder, httptest.NewRequest(http.MethodPost, "/api/manus/test", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"total_credits":77`) {
		t.Fatalf("test response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleManusProjectsAnnotatesAllowlist(t *testing.T) {
	t.Parallel()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"id":"project-1","name":"Allowed"},{"id":"project-2","name":"Blocked"}]}`))
	}))
	defer api.Close()
	client, _ := manus.NewClient("secret", manus.ClientConfig{BaseURL: api.URL})
	recorder := httptest.NewRecorder()
	handleManusProjectsWithClient(client, []string{"project-1"})(recorder, httptest.NewRequest(http.MethodGet, "/api/manus/projects", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"allowed":true`) || !strings.Contains(recorder.Body.String(), `"allowed":false`) {
		t.Fatalf("projects response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleManusSkillsAcceptsQuotedUpstreamTimestamps(t *testing.T) {
	t.Parallel()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"id":"skill-1","name":"Research","owner_type":"official","created_at":"2026-07-26T02:03:57Z","updated_at":"1721959437"}]}`))
	}))
	defer api.Close()
	client, _ := manus.NewClient("secret", manus.ClientConfig{BaseURL: api.URL})
	recorder := httptest.NewRecorder()
	handleManusSkillsWithClient(client, []string{"skill-1"})(recorder, httptest.NewRequest(http.MethodGet, "/api/manus/skills", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"id":"skill-1"`) {
		t.Fatalf("skills response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestWriteManusUpstreamErrorPreservesDiagnostics(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	writeManusUpstreamError(recorder, &manus.APIError{
		StatusCode: http.StatusTooManyRequests,
		Code:       "rate_limited",
		Message:    "Too many requests",
		RequestID:  "req-limited",
	})
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", recorder.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != "rate_limited" || body["request_id"] != "req-limited" || body["upstream_status"] != float64(http.StatusTooManyRequests) {
		t.Fatalf("diagnostic body = %#v", body)
	}
}

func TestManusConfigSnapshotCopiesAllowlists(t *testing.T) {
	t.Parallel()

	server := &Server{Cfg: &config.Config{Manus: config.ManusConfig{
		AllowedProjectIDs:   []string{"project-1"},
		AllowedConnectorIDs: []string{"connector-1"},
		AllowedSkillIDs:     []string{"skill-1"},
	}}}
	snapshot := manusConfigSnapshot(server)
	snapshot.AllowedProjectIDs[0] = "changed-project"
	snapshot.AllowedConnectorIDs[0] = "changed-connector"
	snapshot.AllowedSkillIDs[0] = "changed-skill"
	if server.Cfg.Manus.AllowedProjectIDs[0] != "project-1" ||
		server.Cfg.Manus.AllowedConnectorIDs[0] != "connector-1" ||
		server.Cfg.Manus.AllowedSkillIDs[0] != "skill-1" {
		t.Fatalf("snapshot aliases live config: %#v", server.Cfg.Manus)
	}
}
