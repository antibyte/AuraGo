package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDockerManagerStaysOutOfTheDesktopIntegrationRoots pins that the Docker
// Manager is not reachable through /api/desktop/integrations/: its routes (the
// terminal and the live stream included) keep the admin gate of /api/ and never
// become part of the Desktop policy.
func TestDockerManagerStaysOutOfTheDesktopIntegrationRoots(t *testing.T) {
	for _, root := range []string{"docker-manager", "docker_manager", "dockermanager"} {
		if desktopIntegrationRoots[root] {
			t.Fatalf("desktopIntegrationRoots lists %q: the Docker Manager must not be a Desktop integration", root)
		}
	}
	s, _, _ := testDesktopPermissionServer(t)
	adminToken, _, err := s.TokenManager.Create("fixture", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := desktopIntegrationHandler(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("%s reached the shared routes through the Desktop integrations", r.URL.Path)
	}))
	for _, path := range []string{"stream", "hosts/local/containers/web1/terminal", "hosts/local/df"} {
		req := httptest.NewRequest(http.MethodGet, "/api/desktop/integrations/docker-manager/"+path, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		h(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET /api/desktop/integrations/docker-manager/%s = %d, want 404", path, rec.Code)
		}
	}
}
