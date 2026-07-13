package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/manus"
)

func TestHandleManusSkillsAllowsProjectDiscoveryBeforeAllowlisting(t *testing.T) {
	t.Parallel()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/skill.list" || r.URL.Query().Get("project_id") != "project-new" {
			t.Fatalf("request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"id":"skill-project","name":"Project skill","owner_type":"project"}]}`))
	}))
	defer api.Close()
	client, _ := manus.NewClient("secret", manus.ClientConfig{BaseURL: api.URL})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/manus/skills?project_id=project-new", nil)

	handleManusSkillsWithClient(client, []string{})(recorder, request)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"id":"skill-project"`) || !strings.Contains(recorder.Body.String(), `"allowed":false`) {
		t.Fatalf("skills response = %d %s", recorder.Code, recorder.Body.String())
	}
}
