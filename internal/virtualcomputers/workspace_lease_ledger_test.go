package virtualcomputers

import (
	"context"
	"fmt"
	"path/filepath"
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
