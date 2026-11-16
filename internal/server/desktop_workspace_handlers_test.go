package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/desktop"
)

func desktopWorkspaceHandlerFixture(t *testing.T) (*Server, string, string, string) {
	t.Helper()
	s := newDesktopFilesystemTestServer(t)
	auth, readToken, writeToken := testDesktopPermissionServer(t)
	s.TokenManager, s.Vault = auth.TokenManager, auth.Vault
	s.Cfg.Auth = auth.Cfg.Auth
	adminToken, _, err := s.TokenManager.Create("workspace admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatalf("create admin token: %v", err)
	}
	return s, readToken, writeToken, adminToken
}

func desktopWorkspaceRequest(s *Server, token string, method string, body interface{}) *httptest.ResponseRecorder {
	encoded, _ := json.Marshal(body)
	req := httptest.NewRequest(method, "/api/desktop/workspaces", bytes.NewReader(encoded))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handleDesktopWorkspaces(s).ServeHTTP(response, req)
	return response
}

func TestDesktopWorkspaceHandlerScopesAndRevisionResponses(t *testing.T) {
	s, readToken, writeToken, adminToken := desktopWorkspaceHandlerFixture(t)
	get := desktopWorkspaceRequest(s, readToken, http.MethodGet, nil)
	if get.Code != http.StatusOK {
		t.Fatalf("reader GET status=%d body=%s", get.Code, get.Body.String())
	}
	var initial struct {
		Status   string                  `json:"status"`
		Layout   desktop.WorkspaceLayout `json:"layout"`
		Settings map[string]string       `json:"settings"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &initial); err != nil {
		t.Fatal(err)
	}
	if initial.Status != "ok" || initial.Layout.Revision != 1 || len(initial.Layout.Workspaces) != 3 {
		t.Fatalf("unexpected GET state: %+v", initial)
	}

	command := desktop.WorkspaceMutation{Operation: "add", Revision: initial.Layout.Revision, Name: "Research"}
	for _, token := range []string{readToken, writeToken} {
		response := desktopWorkspaceRequest(s, token, http.MethodPost, command)
		if response.Code != http.StatusForbidden {
			t.Fatalf("non-admin POST token was allowed: status=%d body=%s", response.Code, response.Body.String())
		}
	}

	missingRevision := desktopWorkspaceRequest(s, adminToken, http.MethodPost, desktop.WorkspaceMutation{Operation: "add", Name: "Research"})
	assertWorkspaceConflictBody(t, missingRevision, "workspace_revision_required", 1)

	created := desktopWorkspaceRequest(s, adminToken, http.MethodPost, command)
	if created.Code != http.StatusOK {
		t.Fatalf("admin POST status=%d body=%s", created.Code, created.Body.String())
	}
	var after struct {
		Layout desktop.WorkspaceLayout `json:"layout"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after.Layout.Revision != 2 || after.Layout.Workspaces[3] != (desktop.Workspace{ID: "4", Name: "Research"}) {
		t.Fatalf("unexpected created workspace: %+v", after.Layout)
	}

	stale := desktopWorkspaceRequest(s, adminToken, http.MethodPost, desktop.WorkspaceMutation{Operation: "rename", Revision: 1, ID: "1", Name: "Stale"})
	assertWorkspaceConflictBody(t, stale, "workspace_revision_conflict", 2)
	latest, err := s.DesktopService.GetWorkspaceState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if latest.Layout.Workspaces[0].Name != "" || latest.Layout.Revision != 2 {
		t.Fatalf("stale request mutated current state: %+v", latest.Layout)
	}

	s.Cfg.VirtualDesktop.ReadOnly = true
	readonly := desktopWorkspaceRequest(s, adminToken, http.MethodPost, desktop.WorkspaceMutation{Operation: "rename", Revision: 2, ID: "1", Name: "Blocked"})
	if readonly.Code != http.StatusLocked {
		t.Fatalf("readonly POST status=%d body=%s", readonly.Code, readonly.Body.String())
	}
}

func assertWorkspaceConflictBody(t *testing.T, response *httptest.ResponseRecorder, code string, revision int64) {
	t.Helper()
	if response.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Code  string `json:"code"`
		State struct {
			Layout desktop.WorkspaceLayout `json:"layout"`
		} `json:"state"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != code || body.State.Layout.Revision != revision {
		t.Fatalf("conflict response missing current state: %+v", body)
	}
}

func TestWriteDesktopWorkspaceErrorMapsWallpaperCodes(t *testing.T) {
	for _, test := range []struct {
		code string
		want int
	}{
		{"wallpaper_invalid_path", http.StatusBadRequest},
		{"wallpaper_not_found", http.StatusNotFound},
		{"wallpaper_too_large", http.StatusRequestEntityTooLarge},
		{"wallpaper_unsupported_format", http.StatusUnsupportedMediaType},
		{"wallpaper_changed", http.StatusConflict},
		{"wallpaper_read_failed", http.StatusServiceUnavailable},
	} {
		t.Run(test.code, func(t *testing.T) {
			response := httptest.NewRecorder()
			writeDesktopWorkspaceError(response, &desktop.WallpaperError{Code: test.code}, desktop.WorkspaceState{})
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
		})
	}
}

func TestDesktopSettingsHandlerKeepsValidationAndReadonlyStatuses(t *testing.T) {
	s, _, _, adminToken := desktopWorkspaceHandlerFixture(t)
	for _, test := range []struct {
		name string
		body map[string]interface{}
	}{
		{"invalid volume", map[string]interface{}{"key": "sound.volume", "value": "2"}},
		{"unknown key", map[string]interface{}{"key": "desktop.unsupported", "value": "true"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, _ := json.Marshal(test.body)
			req := httptest.NewRequest(http.MethodPut, "/api/desktop/settings", bytes.NewReader(encoded))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handleDesktopSettings(s).ServeHTTP(response, req)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid setting status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	encoded, _ := json.Marshal(map[string]interface{}{"key": "appearance.theme", "value": "fruity"})
	req := httptest.NewRequest(http.MethodPut, "/api/desktop/settings", bytes.NewReader(encoded))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handleDesktopSettings(s).ServeHTTP(response, req)
	if response.Code != http.StatusForbidden || !bytes.Contains(response.Body.Bytes(), []byte(`"desktop_readonly"`)) {
		t.Fatalf("readonly setting status=%d body=%s", response.Code, response.Body.String())
	}
}
