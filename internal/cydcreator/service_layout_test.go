package cydcreator

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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

// seedLayoutAsset adds an asset entry (and its file) to the stored layout the
// way the asset API does.
func seedLayoutAsset(t *testing.T, env *testEnv, projectID string, a Asset) {
	t.Helper()
	ctx := context.Background()
	unlock := env.svc.lockProject(projectID)
	defer unlock()
	row, err := env.svc.loadProjectRow(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	root, err := env.svc.openRoot(row, "")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	p, err := readLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := rootWriteFile(ctx, root, a.File, pngBytes(t, a.W, a.H, a.Alpha)); err != nil {
		t.Fatal(err)
	}
	p.Assets = append(p.Assets, a)
	if _, err := env.svc.saveLayoutLocked(ctx, root, row, p); err != nil {
		t.Fatal(err)
	}
}

func TestPutLayoutKeepsTheAssetListServerOwned(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Bilder")
	sun := Asset{ID: "a_sun", File: "assets/sun.png", W: 16, H: 16}
	moon := Asset{ID: "a_moon", File: "assets/moon.png", W: 8, H: 8, Alpha: true}
	seedLayoutAsset(t, env, p.ID, sun)
	seedLayoutAsset(t, env, p.ID, moon)
	layout, version, _, err := env.svc.GetLayout(ctx, p.ID)
	if err != nil || len(layout.Assets) != 2 {
		t.Fatalf("seeded layout = %+v, %v", layout.Assets, err)
	}
	with := func(assets ...Asset) Project {
		next := layout
		next.Assets = assets
		return next
	}
	for name, assets := range map[string][]Asset{
		"layout file":       {sun, {ID: "a_moon", File: ProjectFile, W: 8, H: 8, Alpha: true}},
		"app source":        {sun, {ID: "a_moon", File: "src/app/app_main.cpp", W: 8, H: 8, Alpha: true}},
		"traversal":         {sun, {ID: "a_moon", File: "assets/../cyd.project.json", W: 8, H: 8, Alpha: true}},
		"other asset file":  {sun, {ID: "a_moon", File: "assets/sun.png", W: 8, H: 8, Alpha: true}},
		"renamed file":      {sun, {ID: "a_moon", File: "assets/luna.png", W: 8, H: 8, Alpha: true}},
		"changed size":      {sun, {ID: "a_moon", File: "assets/moon.png", W: 9, H: 8, Alpha: true}},
		"new asset":         {sun, moon, {ID: "a_star", File: "assets/star.png", W: 8, H: 8}},
		"listed twice":      {sun, moon, sun},
		"new id, same file": {sun, {ID: "a_moon2", File: "assets/moon.png", W: 8, H: 8, Alpha: true}},
	} {
		_, _, err := env.svc.PutLayout(ctx, p.ID, with(assets...), version, OriginUser)
		typed := wantCode(t, err, CodeInvalidLayout)
		if typed.Details["asset"] == nil || typed.Details["reason"] == nil {
			t.Errorf("%s: details = %v, want asset and reason", name, typed.Details)
		}
	}
	stored, storedVersion, _, _ := env.svc.GetLayout(ctx, p.ID)
	if storedVersion != version || len(stored.Assets) != 2 || stored.Assets[1] != moon {
		t.Fatalf("refused saves changed the layout: v%d %+v", storedVersion, stored.Assets)
	}
	// Keeping the entries unchanged, or dropping one, is a normal save.
	next, _, err := env.svc.PutLayout(ctx, p.ID, with(sun, moon), version, OriginUser)
	if err != nil {
		t.Fatalf("PutLayout with unchanged assets: %v", err)
	}
	if _, _, err := env.svc.PutLayout(ctx, p.ID, with(moon), next, OriginUser); err != nil {
		t.Fatalf("PutLayout dropping an asset entry: %v", err)
	}
}

func TestDamagedLayoutFilesAreInvalidLayouts(t *testing.T) {
	ctx := context.Background()
	cases := map[string]struct {
		damage func(t *testing.T, dir string)
		reason string
	}{
		"missing file": {func(t *testing.T, dir string) { mustRemove(t, filepath.Join(dir, ProjectFile)) }, "missing"},
		"missing dir":  {func(t *testing.T, dir string) { mustRemove(t, dir) }, "missing"},
		"directory": {func(t *testing.T, dir string) {
			mustRemove(t, filepath.Join(dir, ProjectFile))
			if err := os.Mkdir(filepath.Join(dir, ProjectFile), 0o750); err != nil {
				t.Fatal(err)
			}
		}, "not_regular"},
		"too large": {func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, ProjectFile), bytes.Repeat([]byte(" "), MaxLayoutBytes+2), 0o640); err != nil {
				t.Fatal(err)
			}
		}, "too_large"},
		"garbage": {func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, ProjectFile), []byte("{nope"), 0o640); err != nil {
				t.Fatal(err)
			}
		}, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			env := newTestService(t)
			p := env.createProject(t, "Kaputt")
			layout, version, _, _ := env.svc.GetLayout(ctx, p.ID)
			tc.damage(t, filepath.Join(env.cfg.WorkspaceDir, "CYD", p.Slug))
			_, _, _, err := env.svc.GetLayout(ctx, p.ID)
			typed := wantCode(t, err, CodeInvalidLayout)
			if tc.reason != "" && typed.Details["reason"] != tc.reason {
				t.Fatalf("details = %v, want reason %s", typed.Details, tc.reason)
			}
			_, err = env.svc.RenameProject(ctx, p.ID, "Anders")
			wantCode(t, err, CodeInvalidLayout)
			// A save without assets replaces a damaged file.
			_, _, err = env.svc.PutLayout(ctx, p.ID, layout, version, OriginUser)
			switch name {
			case "missing dir":
				wantCode(t, err, CodeInvalidLayout) // nothing to write into
			case "directory":
				wantCode(t, err, CodeInternal) // a rename cannot replace a directory
			default:
				if err != nil {
					t.Fatalf("PutLayout over a damaged file: %v", err)
				}
				if _, _, _, err := env.svc.GetLayout(ctx, p.ID); err != nil {
					t.Fatalf("GetLayout after the repairing save: %v", err)
				}
			}
		})
	}
}

func TestSaveLayoutMirrorsOnlyValidMetadata(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Spiegel")
	layout, version, _, _ := env.svc.GetLayout(ctx, p.ID)
	layout.Name = "   "
	layout.Board = "no-such-board"
	layout.Theme = "pink"
	next, issues, err := env.svc.PutLayout(ctx, p.ID, layout, version, OriginUser)
	if err != nil || next != version+1 {
		t.Fatalf("PutLayout with invalid metadata = v%d, %v", next, err)
	}
	codes := map[string]bool{}
	for _, is := range issues {
		codes[is.Code] = true
	}
	if !codes[IssueBoardUnknown] || !codes[IssueThemeUnknown] || !codes[IssuePropInvalid] {
		t.Fatalf("issues = %+v, want board, theme and name issues", issues)
	}
	got, _ := env.svc.GetProject(ctx, p.ID)
	if got.Name != "Spiegel" || got.Board != "esp32-2432s028r" || got.Theme != "night" || got.Version != next {
		t.Fatalf("row after invalid metadata = %+v", got)
	}
}

func TestSaveLayoutRefusesAStaleVersion(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Alt")
	row, err := env.svc.loadProjectRow(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	root, err := env.svc.openRoot(row, "")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	layout, _ := readLayout(root)
	if _, err := env.svc.db.Exec(`UPDATE cc_projects SET version=7 WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.saveLayoutLocked(ctx, root, row, layout); err == nil {
		t.Fatal("saveLayoutLocked accepted a stale row version")
	}
	got, _ := env.svc.GetProject(ctx, p.ID)
	if got.Version != 7 {
		t.Fatalf("version = %d, want the newer 7 untouched", got.Version)
	}
}

func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
}
