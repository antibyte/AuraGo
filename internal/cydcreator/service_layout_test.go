package cydcreator

import (
	"context"
	"testing"
	"time"
)

func TestPutLayoutBumpsVersionAndDetectsConflicts(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Layout")
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	layout, version, issues, err := env.svc.GetLayout(ctx, p.ID)
	if err != nil || version != 1 || len(issues) != 0 {
		t.Fatalf("GetLayout = v%d %v, %v", version, issues, err)
	}
	layout.Screens[0].Title = "Start"
	next, issues, err := env.svc.PutLayout(ctx, p.ID, layout, 1, OriginUser)
	if err != nil || next != 2 || len(issues) != 0 {
		t.Fatalf("PutLayout = v%d %v, %v", next, issues, err)
	}
	ev := collectEvents(t, events, 1, 2*time.Second, EventProjectChanged)[0]
	if string(ev.Data) != `{"origin":"user","version":2,"what":"layout"}` {
		t.Fatalf("project_changed = %s", ev.Data)
	}
	_, _, err = env.svc.PutLayout(ctx, p.ID, layout, 1, OriginUser)
	typed := wantCode(t, err, CodeProjectConflict)
	if typed.Details["version"] != int64(2) {
		t.Fatalf("conflict details = %v, want version 2", typed.Details)
	}
	stored, version, _, err := env.svc.GetLayout(ctx, p.ID)
	if err != nil || version != 2 || stored.Screens[0].Title != "Start" {
		t.Fatalf("stored layout v%d title %q, %v", version, stored.Screens[0].Title, err)
	}
}

func TestPutLayoutSavesSemanticIssues(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Issues")
	layout, version, _, _ := env.svc.GetLayout(ctx, p.ID)
	layout.Screens[0].Widgets = append(layout.Screens[0].Widgets, Widget{ID: "w_bad", Type: "label", Name: "9bad", X: 0, Y: 0, W: 50, H: 20})
	next, issues, err := env.svc.PutLayout(ctx, p.ID, layout, version, OriginUser)
	if err != nil || next != version+1 {
		t.Fatalf("PutLayout with issues = v%d, %v", next, err)
	}
	found := false
	for _, is := range issues {
		found = found || is.Code == IssueNameInvalid
	}
	if !found {
		t.Fatalf("issues = %+v, want %s", issues, IssueNameInvalid)
	}
	_, _, again, err := env.svc.GetLayout(ctx, p.ID)
	if err != nil || len(again) != len(issues) {
		t.Fatalf("GetLayout issues = %+v, %v", again, err)
	}
}

func TestPutLayoutMirrorsMetadataAndRejectsBadStructure(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Mirror")
	layout, version, _, _ := env.svc.GetLayout(ctx, p.ID)
	layout.Name = "Mirror Two"
	layout.Theme = "terminal"
	if _, _, err := env.svc.PutLayout(ctx, p.ID, layout, version, OriginUser); err != nil {
		t.Fatal(err)
	}
	got, _ := env.svc.GetProject(ctx, p.ID)
	if got.Name != "Mirror Two" || got.Theme != "terminal" || got.Slug != "mirror" {
		t.Fatalf("summary after PutLayout = %+v", got)
	}
	bad := layout
	bad.Screens = append([]Screen(nil), layout.Screens...)
	bad.Screens[0].ID = "bad id!"
	_, _, err := env.svc.PutLayout(ctx, p.ID, bad, version+1, OriginUser)
	wantCode(t, err, CodeInvalidLayout)
	bad.Format = 2
	_, _, err = env.svc.PutLayout(ctx, p.ID, bad, version+1, OriginUser)
	wantCode(t, err, CodeInvalidLayout)
}
