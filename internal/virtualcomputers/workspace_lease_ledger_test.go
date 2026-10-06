package virtualcomputers

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestLeaseScanPagesAndAtomicClaim(t *testing.T) {
	ctx := context.Background()
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "workspaces.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	now := time.Now().UTC()
	for i := 0; i < 501; i++ {
		id := fmt.Sprintf("workspace-%03d", i)
		if err := ledger.UpsertWorkspace(ctx, Workspace{ID: id, OwnerSessionID: "owner", MachineID: id,
			State: WorkspaceStateReady, LeaseExpiresAt: now.Add(time.Minute), MaxExpiresAt: now.Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := ledger.ListLeaseScanPage(ctx, "")
	if err != nil || len(first) != 500 {
		t.Fatalf("first page = %d, %v", len(first), err)
	}
	second, err := ledger.ListLeaseScanPage(ctx, first[len(first)-1].ID)
	if err != nil || len(second) != 1 || second[0].ID != "workspace-500" {
		t.Fatalf("second page = %+v, %v", second, err)
	}
	id := second[0].ID
	if claimed, err := ledger.ClaimExpiredWorkspace(ctx, id, now); err != nil || claimed {
		t.Fatalf("fresh lease claimed = %v, %v", claimed, err)
	}
	activity := now.Add(2 * time.Minute)
	if err := ledger.UpdateWorkspaceActivity(ctx, id, activity, activity.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if claimed, err := ledger.ClaimExpiredWorkspace(ctx, id, activity); err != nil || claimed {
		t.Fatalf("extended lease claimed = %v, %v", claimed, err)
	}
	if claimed, err := ledger.ClaimExpiredWorkspace(ctx, id, activity.Add(2*time.Minute)); err != nil || !claimed {
		t.Fatalf("expired lease claim = %v, %v", claimed, err)
	}
	if err := ledger.UpdateWorkspaceActivity(ctx, id, activity, activity.Add(time.Hour)); err == nil {
		t.Fatal("activity update revived closing workspace")
	}
}

func TestActivityUpdatePreservesConcurrentControlChange(t *testing.T) {
	ctx := context.Background()
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "workspaces.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	now := time.Now().UTC()
	workspace := Workspace{ID: "w", OwnerSessionID: "owner", MachineID: "m", State: WorkspaceStateReady,
		LeaseExpiresAt: now.Add(time.Minute), MaxExpiresAt: now.Add(time.Hour)}
	if err := ledger.UpsertWorkspace(ctx, workspace); err != nil {
		t.Fatal(err)
	}
	workspace.ControlOwner = ControlOwnerHuman
	if err := ledger.UpsertWorkspace(ctx, workspace); err != nil {
		t.Fatal(err)
	}
	if err := ledger.UpdateWorkspaceActivity(ctx, workspace.ID, now, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	stored, ok, err := ledger.GetWorkspace(ctx, workspace.ID)
	if err != nil || !ok || stored.ControlOwner != ControlOwnerHuman {
		t.Fatalf("control change overwritten: %+v, %v", stored, err)
	}
}

// workspaceRow returns every stored column of one workspace row as text so a
// test can prove which columns a scoped update touched.
func workspaceRow(t *testing.T, ledger *Ledger, id string) map[string]string {
	t.Helper()
	rows, err := ledger.db.Query(`SELECT * FROM workspaces WHERE id = ?`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatalf("workspace %q is missing", id)
	}
	values := make([]sql.NullString, len(columns))
	dest := make([]interface{}, len(columns))
	for i := range values {
		dest[i] = &values[i]
	}
	if err := rows.Scan(dest...); err != nil {
		t.Fatal(err)
	}
	row := make(map[string]string, len(columns))
	for i, column := range columns {
		row[column] = values[i].String
		if !values[i].Valid {
			row[column] = "<NULL>"
		}
	}
	return row
}

func seedControlLeaseWorkspace(t *testing.T, ledger *Ledger, id, owner string, controlLease *time.Time, updated time.Time) {
	t.Helper()
	workspace := Workspace{ID: id, OwnerSessionID: "owner-session", MissionID: "mission", Actor: "main-agent",
		MachineID: "machine-" + id, State: WorkspaceStateReady, Template: "desktop", NetworkProfile: "internet_lan",
		VolumeID: "volume-" + id, Capabilities: []string{"shell", "browser"}, InstanceNonce: "nonce-" + id,
		ControlOwner: owner, ControlLeaseExpiresAt: controlLease, LastError: "previous error",
		CreatedAt: updated.Add(-time.Hour), UpdatedAt: updated, LastActivityAt: updated.Add(-time.Minute),
		LeaseExpiresAt: updated.Add(time.Hour), MaxExpiresAt: updated.Add(2 * time.Hour)}
	if err := ledger.UpsertWorkspace(context.Background(), workspace); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseExpiredControlLeaseOnlyTouchesControlColumns(t *testing.T) {
	ctx := context.Background()
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "workspaces.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Minute)
	seedControlLeaseWorkspace(t, ledger, "expired", ControlOwnerHuman, &expired, now.Add(-10*time.Minute))
	before := workspaceRow(t, ledger, "expired")

	released, err := ledger.ReleaseExpiredControlLease(ctx, "expired", now)
	if err != nil || !released {
		t.Fatalf("expired lease release = %v, %v", released, err)
	}
	after := workspaceRow(t, ledger, "expired")
	want := make(map[string]string, len(before))
	for column, value := range before {
		want[column] = value
	}
	want["control_owner"] = ControlOwnerAgent
	want["control_lease_expires_at"] = ""
	want["updated_at"] = timeText(now)
	if !reflect.DeepEqual(after, want) {
		t.Fatalf("release changed more than the control columns:\nbefore=%v\nafter =%v\nwant  =%v", before, after, want)
	}
	stored, ok, err := ledger.GetWorkspace(ctx, "expired")
	if err != nil || !ok || stored.ControlOwner != ControlOwnerAgent || stored.ControlLeaseExpiresAt != nil || !stored.UpdatedAt.Equal(now) {
		t.Fatalf("released workspace = %+v ok=%v err=%v", stored, ok, err)
	}

	released, err = ledger.ReleaseExpiredControlLease(ctx, "expired", now.Add(time.Second))
	if err != nil || released {
		t.Fatalf("second release = %v, %v", released, err)
	}
	if again := workspaceRow(t, ledger, "expired"); !reflect.DeepEqual(again, after) {
		t.Fatalf("second release changed the row:\nbefore=%v\nafter =%v", after, again)
	}
}

func TestReleaseExpiredControlLeaseLeavesOtherRowsUntouched(t *testing.T) {
	ctx := context.Background()
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "workspaces.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Minute)
	past := now.Add(-time.Minute)
	seedControlLeaseWorkspace(t, ledger, "active-lease", ControlOwnerHuman, &future, now.Add(-time.Hour))
	seedControlLeaseWorkspace(t, ledger, "agent-owned", ControlOwnerAgent, &past, now.Add(-time.Hour))
	seedControlLeaseWorkspace(t, ledger, "no-lease", ControlOwnerHuman, nil, now.Add(-time.Hour))
	for _, id := range []string{"active-lease", "agent-owned", "no-lease"} {
		before := workspaceRow(t, ledger, id)
		released, err := ledger.ReleaseExpiredControlLease(ctx, id, now)
		if err != nil || released {
			t.Fatalf("%s: release = %v, %v", id, released, err)
		}
		if after := workspaceRow(t, ledger, id); !reflect.DeepEqual(after, before) {
			t.Fatalf("%s: row changed:\nbefore=%v\nafter =%v", id, before, after)
		}
	}
	if released, err := ledger.ReleaseExpiredControlLease(ctx, "missing", now); err != nil || released {
		t.Fatalf("missing workspace release = %v, %v", released, err)
	}
}

// RFC3339Nano text drops trailing zero fractions, so "…:00Z" sorts after
// "…:00.5Z" as text although it is earlier. The release must compare times.
func TestReleaseExpiredControlLeaseComparesTimesNotText(t *testing.T) {
	ctx := context.Background()
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "workspaces.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	whole := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	half := whole.Add(500 * time.Millisecond)
	seedControlLeaseWorkspace(t, ledger, "whole-second-lease", ControlOwnerHuman, &whole, whole.Add(-time.Hour))
	seedControlLeaseWorkspace(t, ledger, "fractional-lease", ControlOwnerHuman, &half, whole.Add(-time.Hour))
	seedControlLeaseWorkspace(t, ledger, "boundary-lease", ControlOwnerHuman, &whole, whole.Add(-time.Hour))

	if released, err := ledger.ReleaseExpiredControlLease(ctx, "whole-second-lease", half); err != nil || !released {
		t.Fatalf("lease that expired 500ms ago was kept: %v, %v", released, err)
	}
	if released, err := ledger.ReleaseExpiredControlLease(ctx, "fractional-lease", whole); err != nil || released {
		t.Fatalf("lease with 500ms left was released: %v, %v", released, err)
	}
	if released, err := ledger.ReleaseExpiredControlLease(ctx, "boundary-lease", whole); err != nil || !released {
		t.Fatalf("lease expiring exactly now was kept: %v, %v", released, err)
	}
}

func TestReconcileLeasesReleasesExpiredControlWithoutOverwritingConcurrentChanges(t *testing.T) {
	ctx := context.Background()
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "workspaces.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Minute)
	renewed := now.Add(time.Hour)
	seedControlLeaseWorkspace(t, ledger, "expired", ControlOwnerHuman, &expired, now.Add(-10*time.Minute))
	seedControlLeaseWorkspace(t, ledger, "raced", ControlOwnerHuman, &expired, now.Add(-10*time.Minute))
	for _, id := range []string{"expired", "raced"} {
		if err := ledger.UpsertBrowserSession(ctx, BrowserSession{ID: "browser-" + id, WorkspaceID: id, State: BrowserStateOpen,
			ControlOwner: ControlOwnerHuman, ControlLeaseExpiresAt: &expired}); err != nil {
			t.Fatal(err)
		}
	}
	// An expired shell grant that cannot be cleaned up without a transport makes
	// the reconciler report an issue for "raced" after it has read the row; the
	// reporter uses that moment to renew the human lease concurrently. This
	// deliberately relies on reconcileLeases running the credential-expiry
	// cleanup (and its reportIssue) before the control-release block for the
	// same workspace; if that order changes, inject the race elsewhere.
	if err := ledger.UpsertCredentialGrant(ctx, CredentialGrant{ID: "grant-raced", WorkspaceID: "raced", CredentialID: "cred",
		UsageType: GrantUsageShell, JobID: "job-raced", Status: GrantActive, ExpiresAt: now.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	renewedConcurrently := false
	manager, err := NewWorkspaceManager(ledger, slog.New(slog.NewTextHandler(io.Discard, nil)), WorkspaceManagerOptions{
		ClientFactory: func(ToolConfig) (*Client, error) { return nil, fmt.Errorf("control plane offline") },
		IssueReporter: func(issue WorkspaceOperationalIssue) {
			if issue.WorkspaceID != "raced" || renewedConcurrently {
				return
			}
			renewedConcurrently = true
			if _, err := ledger.db.Exec(`UPDATE workspaces SET control_lease_expires_at = ?, last_error = ? WHERE id = ?`,
				timeText(renewed), "concurrent change", "raced"); err != nil {
				t.Errorf("concurrent renew: %v", err)
			}
		},
		Now: func() time.Time { return now }, ReconcileInterval: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.rememberConfig(ToolConfig{AgentControl: AgentControlConfig{Enabled: true}})

	manager.reconcileLeases(ctx)

	if !renewedConcurrently {
		t.Fatal("test did not inject the concurrent renew")
	}
	released, ok, err := ledger.GetWorkspace(ctx, "expired")
	if err != nil || !ok || released.ControlOwner != ControlOwnerAgent || released.ControlLeaseExpiresAt != nil || !released.UpdatedAt.Equal(now) {
		t.Fatalf("expired control lease not released: %+v ok=%v err=%v", released, ok, err)
	}
	raced, ok, err := ledger.GetWorkspace(ctx, "raced")
	if err != nil || !ok || raced.ControlOwner != ControlOwnerHuman || raced.ControlLeaseExpiresAt == nil ||
		!raced.ControlLeaseExpiresAt.Equal(renewed) || raced.LastError != "concurrent change" {
		t.Fatalf("concurrent control change was overwritten: %+v ok=%v err=%v", raced, ok, err)
	}
	for id, wantOwner := range map[string]string{"expired": ControlOwnerAgent, "raced": ControlOwnerHuman} {
		sessions, err := ledger.ListBrowserSessions(ctx, id)
		if err != nil || len(sessions) != 1 || sessions[0].ControlOwner != wantOwner {
			t.Fatalf("%s browser control = %+v, %v; want owner %s", id, sessions, err, wantOwner)
		}
		if wantOwner == ControlOwnerAgent && sessions[0].ControlLeaseExpiresAt != nil {
			t.Fatalf("%s browser lease kept after release: %+v", id, sessions[0])
		}
	}
}
