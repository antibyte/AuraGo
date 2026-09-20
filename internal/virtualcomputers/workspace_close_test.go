package virtualcomputers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWorkspaceLeaseCloseReconcilesMissingMachine(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		wantError bool
	}{
		{"already absent", http.StatusNotFound, `{"error":"not found"}`, false},
		{"deleted", http.StatusNoContent, "", false},
		{"router 404", http.StatusNotFound, "404 page not found", true},
		{"other JSON 404", http.StatusNotFound, `{"error":"route unavailable"}`, true},
		{"malformed JSON 404", http.StatusNotFound, `{"error":"not found"`, true},
		{"server failure", http.StatusInternalServerError, `{"error":"not found"}`, true},
		{"unauthorized", http.StatusUnauthorized, `{"error":"not found"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			now := time.Now().UTC()
			status, body, deletes := tc.status, tc.body, 0
			var serverMu sync.Mutex
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				serverMu.Lock()
				defer serverMu.Unlock()
				if r.Method != http.MethodDelete || r.URL.Path != "/v1/machines/missing-machine" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				deletes++
				w.WriteHeader(status)
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			ledger, err := OpenLedger(filepath.Join(t.TempDir(), "workspaces.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer ledger.Close()
			var issues []WorkspaceOperationalIssue
			manager, err := NewWorkspaceManager(ledger, slog.New(slog.NewTextHandler(io.Discard, nil)), WorkspaceManagerOptions{
				ClientFactory:    func(ToolConfig) (*Client, error) { return NewClient(ClientConfig{BaseURL: server.URL}) },
				TransportFactory: func(*Client) WorkspaceTransport { return &workspaceTestTransport{} },
				IssueReporter:    func(issue WorkspaceOperationalIssue) { issues = append(issues, issue) },
				Now:              func() time.Time { return now }, ReconcileInterval: time.Hour,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer manager.Close()
			manager.rememberConfig(ToolConfig{AgentControl: AgentControlConfig{Enabled: true}})
			workspace := Workspace{ID: "workspace-1", OwnerSessionID: "session-1", MachineID: "missing-machine",
				State: WorkspaceStateLost, LastError: "previous delete failed", MaxExpiresAt: now.Add(-time.Minute), LeaseExpiresAt: now.Add(-time.Minute)}
			if err := ledger.UpsertWorkspace(ctx, workspace); err != nil {
				t.Fatal(err)
			}
			if err := ledger.UpsertWorkspaceJob(ctx, WorkspaceJob{ID: "job-1", WorkspaceID: workspace.ID, State: JobStateRunning}); err != nil {
				t.Fatal(err)
			}
			if err := ledger.UpsertBrowserSession(ctx, BrowserSession{ID: "browser-1", WorkspaceID: workspace.ID, State: BrowserStateOpen}); err != nil {
				t.Fatal(err)
			}
			if err := ledger.UpsertCredentialGrant(ctx, CredentialGrant{ID: "grant-1", WorkspaceID: workspace.ID, CredentialID: "cred-1", Status: GrantActive, ExpiresAt: now.Add(time.Hour)}); err != nil {
				t.Fatal(err)
			}

			manager.reconcileLeases(ctx)
			if len(issues) != 1 || issues[0].Kind != "lease_close_failed" || issues[0].Resolved == tc.wantError {
				t.Fatalf("first reconciliation issues = %+v", issues)
			}
			if tc.wantError {
				stored, _, _ := ledger.GetWorkspace(ctx, workspace.ID)
				if stored.State != WorkspaceStateLost || stored.LastError == "" {
					t.Fatalf("failure was hidden: %+v", stored)
				}
				serverMu.Lock()
				status, body = http.StatusNotFound, `{"error":"not found"}`
				serverMu.Unlock()
				manager.reconcileLeases(ctx)
				if len(issues) != 2 || !issues[1].Resolved {
					t.Fatalf("recovery missing: %+v", issues)
				}
			}
			stored, _, err := ledger.GetWorkspace(ctx, workspace.ID)
			if err != nil || stored.State != WorkspaceStateClosed || stored.LastError != "" {
				t.Fatalf("workspace not closed: %+v, %v", stored, err)
			}
			job, _, err := ledger.GetWorkspaceJob(ctx, "job-1")
			if err != nil || job.State != JobStateInterrupted {
				t.Fatalf("job not interrupted: %+v %v", job, err)
			}
			sessions, err := ledger.ListBrowserSessions(ctx, workspace.ID)
			if err != nil || len(sessions) != 1 || sessions[0].State != BrowserStateClosed {
				t.Fatalf("browser cleanup: %+v %v", sessions, err)
			}
			grants, err := ledger.ListCredentialGrants(ctx, workspace.ID)
			if err != nil || len(grants) != 1 || grants[0].Status != GrantRevoked {
				t.Fatalf("grant cleanup: %+v %v", grants, err)
			}
			serverMu.Lock()
			before := deletes
			serverMu.Unlock()
			manager.reconcileLeases(ctx)
			serverMu.Lock()
			after := deletes
			serverMu.Unlock()
			if after != before {
				t.Fatalf("closed workspace was deleted again: %d -> %d", before, after)
			}
		})
	}
}
