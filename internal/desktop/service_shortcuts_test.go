package desktop

import (
	"context"
	"testing"
)

func dashboardRows(t *testing.T, svc *Service) []Shortcut {
	t.Helper()
	p, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var out []Shortcut
	for _, sc := range p.Shortcuts {
		if sc.TargetType == ShortcutTargetDashboard {
			out = append(out, sc)
		}
	}
	return out
}

func TestDashboardShortcutAddRenameRemove(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	const id = "d_abcdefghijkl"
	sc, err := svc.AddDesktopDashboardShortcut(ctx, id, "Wohnzimmer", SourceUser)
	if err != nil {
		t.Fatal(err)
	}
	if sc.ID != DashboardShortcutID(id) || sc.ID != "dashboard-"+id {
		t.Fatalf("shortcut id = %q", sc.ID)
	}
	rows := dashboardRows(t, svc)
	if len(rows) != 1 || rows[0].Icon != "homedash" || rows[0].TargetID != id || rows[0].Name != "Wohnzimmer" {
		t.Fatalf("rows = %+v", rows)
	}
	created := rows[0].CreatedAt
	if _, err := svc.AddDesktopDashboardShortcut(ctx, id, "Wohnzimmer", SourceUser); err != nil {
		t.Fatal(err)
	}
	if rows = dashboardRows(t, svc); len(rows) != 1 || !rows[0].CreatedAt.Equal(created) {
		t.Fatalf("second add: rows = %+v, created %v", rows, created)
	}
	if changed, err := svc.RenameTargetShortcuts(ctx, ShortcutTargetDashboard, id, "Wohnzimmer", SourceSystem); err != nil || changed {
		t.Fatalf("same-name rename = %v, %v", changed, err)
	}
	if changed, err := svc.RenameTargetShortcuts(ctx, ShortcutTargetDashboard, id, "Küche", SourceSystem); err != nil || !changed {
		t.Fatalf("rename = %v, %v", changed, err)
	}
	if rows = dashboardRows(t, svc); len(rows) != 1 || rows[0].Name != "Küche" {
		t.Fatalf("after rename: %+v", rows)
	}
	if changed, err := svc.RenameTargetShortcuts(ctx, ShortcutTargetDashboard, "d_zzzzzzzzzzzz", "X", SourceSystem); err != nil || changed {
		t.Fatalf("rename of an unpinned target = %v, %v", changed, err)
	}
	if removed, err := svc.RemoveTargetShortcuts(ctx, ShortcutTargetDashboard, id, SourceSystem); err != nil || !removed {
		t.Fatalf("remove = %v, %v", removed, err)
	}
	if removed, err := svc.RemoveTargetShortcuts(ctx, ShortcutTargetDashboard, id, SourceSystem); err != nil || removed {
		t.Fatalf("second remove = %v, %v", removed, err)
	}
	if rows = dashboardRows(t, svc); len(rows) != 0 {
		t.Fatalf("rows after remove: %+v", rows)
	}

	ro := testServiceWithConfig(t, Config{
		Enabled: true, ReadOnly: true, WorkspaceDir: t.TempDir() + "/ws", DBPath: t.TempDir() + "/ro.db", MaxFileSizeMB: 1,
	})
	if _, err := ro.AddDesktopDashboardShortcut(ctx, id, "X", SourceUser); err == nil {
		t.Fatal("add on a read-only desktop must fail")
	}
}

func TestUpsertDesktopShortcutRejectsBadDashboardTarget(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	for _, target := range []string{"", "has space", "UP/slash", "..", "a;b", "d_abcdefghijk1", "d_abcdefghijklm", "this-id-is-far-too-long-0123456789-0123456789-0123456789-0123456789-xx"} {
		sc := Shortcut{ID: "dashboard-x", TargetType: ShortcutTargetDashboard, TargetID: target, Name: "X"}
		if err := svc.upsertDesktopShortcut(ctx, sc, SourceUser); err == nil {
			t.Fatalf("target %q was accepted", target)
		}
	}
	if len(dashboardRows(t, svc)) != 0 {
		t.Fatal("a rejected target left a row")
	}
}

func TestDashboardShortcutHooksDoNotReopenClosedService(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	const id = "d_abcdefghijkl"
	if _, err := svc.AddDesktopDashboardShortcut(ctx, id, "A", SourceUser); err != nil {
		t.Fatal(err)
	}
	_ = svc.Close() // the docker backend error is irrelevant here
	if changed, err := svc.RenameTargetShortcuts(ctx, ShortcutTargetDashboard, id, "B", SourceSystem); err != nil || changed {
		t.Fatalf("rename on a closed service = %v, %v", changed, err)
	}
	if removed, err := svc.RemoveTargetShortcuts(ctx, ShortcutTargetDashboard, id, SourceSystem); err != nil || removed {
		t.Fatalf("remove on a closed service = %v, %v", removed, err)
	}
	if svc.isReady() {
		t.Fatal("a hook reopened the closed service")
	}
}

func TestDashboardShortcutHooksSkipUninitialisedService(t *testing.T) {
	svc, err := NewService(Config{Enabled: true, WorkspaceDir: t.TempDir() + "/ws", DBPath: t.TempDir() + "/x.db", MaxFileSizeMB: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if removed, err := svc.RemoveTargetShortcuts(context.Background(), ShortcutTargetDashboard, "d_abcdefghijkl", SourceSystem); err != nil || removed {
		t.Fatalf("remove = %v, %v", removed, err)
	}
	if svc.isReady() {
		t.Fatal("a hook initialised the service")
	}
}
