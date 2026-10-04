package agent

import (
	"aurago/internal/config"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestComposioMissingMetadataCannotUseAgentToolkit(t *testing.T) {
	var executions atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			executions.Add(1)
			w.Write([]byte(`{}`))
			return
		}
		http.Error(w, "metadata unavailable", 503)
	}))
	defer upstream.Close()
	cfg := &config.Config{}
	cfg.Composio.Enabled = true
	cfg.Composio.APIKey = "fixture-only"
	cfg.Composio.BaseURL = upstream.URL
	cfg.Composio.Toolkits = []config.ComposioToolkitConfig{{Slug: "gmail", Enabled: true}}
	dispatchComposioCall(context.Background(), composioCallArgs{Operation: "execute_tool", ToolSlug: "GMAIL_GET_EMAIL", ToolkitSlug: "gmail", ConnectedAccountID: "fixture-account"}, cfg)
	if executions.Load() != 0 {
		t.Fatal("executed a tool without trusted metadata")
	}
}
