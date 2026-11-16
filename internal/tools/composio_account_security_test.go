package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestComposioExecutionRequiresBoundActiveAccount(t *testing.T) {
	owner := ComposioConnectedAccount{ID: "owner-account", UserID: "owner", ToolkitSlug: "github", Status: "ACTIVE"}
	other := owner
	other.ID = "second-account"
	foreign := owner
	foreign.UserID = "other"
	inactive := owner
	inactive.Status = "EXPIRED"
	unknown := owner
	unknown.Status = ""
	wrongToolkit := owner
	wrongToolkit.ToolkitSlug = "gmail"
	for _, tc := range []struct {
		name                 string
		accounts             []ComposioConnectedAccount
		preferred, requested string
		allowed              bool
	}{
		{"unique", []ComposioConnectedAccount{owner}, "", "", true},
		{"selected", []ComposioConnectedAccount{owner, other}, owner.ID, "", true},
		{"foreign user", []ComposioConnectedAccount{foreign}, "", owner.ID, false},
		{"foreign toolkit", []ComposioConnectedAccount{wrongToolkit}, "", owner.ID, false},
		{"inactive", []ComposioConnectedAccount{inactive}, owner.ID, "", false},
		{"unknown status", []ComposioConnectedAccount{unknown}, "", "", false},
		{"ambiguous", []ComposioConnectedAccount{owner, other}, "", owner.ID, false},
		{"preferred mismatch", []ComposioConnectedAccount{owner, other}, owner.ID, other.ID, false},
		{"missing preferred", []ComposioConnectedAccount{owner}, other.ID, "", false},
		{"requested mismatch", []ComposioConnectedAccount{owner}, "", other.ID, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var executions atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					json.NewEncoder(w).Encode(map[string]interface{}{"items": tc.accounts})
					return
				}
				executions.Add(1)
				w.Write([]byte(`{}`))
			}))
			defer server.Close()
			client := NewComposioClient(ComposioClientConfig{BaseURL: server.URL, APIKey: "fixture-account-key"})
			_, err := client.ExecuteTool(context.Background(), ComposioExecuteRequest{ToolSlug: "GITHUB_GET_REPO", ToolkitSlug: "github", UserID: "owner", ConnectedAccountID: tc.requested, PreferredAccountID: tc.preferred})
			if (err == nil) != tc.allowed || (executions.Load() == 1) != tc.allowed {
				t.Fatalf("error=%v executions=%d", err, executions.Load())
			}
		})
	}
}
