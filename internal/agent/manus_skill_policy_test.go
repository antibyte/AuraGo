package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/manus"
)

func TestDispatchManusListSkillsKeepsProjectAllowlist(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"ok":true,"data":[]}`))
	}))
	defer server.Close()
	client, _ := manus.NewClient("secret", manus.ClientConfig{BaseURL: server.URL})
	cfg := testManusConfig(t)

	out := dispatchManusCallWithClient(context.Background(), manusCallArgs{
		Operation: "list_skills", ProjectID: "project-new",
	}, cfg, client)
	if !strings.Contains(out, "not allowlisted") {
		t.Fatalf("list_skills output = %s", out)
	}
	if calls.Load() != 0 {
		t.Fatalf("agent made %d remote calls for an unallowlisted project", calls.Load())
	}
}
