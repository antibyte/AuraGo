package cydcreator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Wetterstation":        "wetterstation",
		"Küchen-Uhr 2.0":       "kuechen-uhr-2-0",
		"  Café  Crème  ":      "cafe-creme",
		"Straße/../etc":        "strasse-etc",
		"日本語":                  "project",
		"---":                  "project",
		"Ølstue Æblegrød Åben": "olstue-aeblegrod-aben",
		"a very long project name that goes on and on": "a-very-long-project-name-that-goes-on-an",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCreateProjectWritesTheTreeAndRow(t *testing.T) {
	env := newTestService(t)
	p := env.createProject(t, "Wetterstation")
	if p.ID != "ccp_0001" || p.Slug != "wetterstation" || p.Name != "Wetterstation" || p.Board != "esp32-2432s028r" ||
		p.Theme != "night" || p.Version != 1 || p.LockedByJob != "" || p.HasDraft || p.LastBuild != nil || p.CreatedAt == "" {
		t.Fatalf("summary = %+v", p)
	}
	dir := filepath.Join(env.cfg.WorkspaceDir, "CYD", "wetterstation")
	data, err := os.ReadFile(filepath.Join(dir, ProjectFile))
	if err != nil {
		t.Fatal(err)
	}
	layout, err := ParseProject(data)
	if err != nil || layout.Name != "Wetterstation" || layout.Language != "de" || layout.Board != "esp32-2432s028r" {
		t.Fatalf("layout = %+v, %v", layout, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "app", "app_main.cpp")); err != nil {
		t.Fatalf("scaffolded app_main.cpp missing: %v", err)
	}
	got, err := env.svc.GetProject(context.Background(), p.ID)
	if err != nil || got.Slug != p.Slug {
		t.Fatalf("GetProject = %+v, %v", got, err)
	}
}

func TestCreateProjectAllocatesUniqueSlugs(t *testing.T) {
	env := newTestService(t)
	if err := os.MkdirAll(filepath.Join(env.cfg.WorkspaceDir, "CYD", "uhr-2"), 0o750); err != nil {
		t.Fatal(err)
	}
	first := env.createProject(t, "Uhr")
	second := env.createProject(t, "Uhr")
	if first.Slug != "uhr" || second.Slug != "uhr-3" {
		t.Fatalf("slugs = %q, %q; want uhr and uhr-3 (uhr-2 exists on disk)", first.Slug, second.Slug)
	}
}

func TestCreateProjectValidatesInput(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	for name, in := range map[string]CreateProjectInput{
		"empty name":       {Name: " ", Board: "esp32-2432s028r"},
		"unknown board":    {Name: "X", Board: "nope"},
		"unknown theme":    {Name: "X", Board: "esp32-2432s028r", Theme: "pink"},
		"unknown language": {Name: "X", Board: "esp32-2432s028r", Language: "tlh"},
	} {
		if _, err := env.svc.CreateProject(ctx, in); err == nil {
			t.Errorf("%s: CreateProject succeeded", name)
		} else {
			wantCode(t, err, CodeBadRequest)
		}
	}
	p, err := env.svc.CreateProject(ctx, CreateProjectInput{Name: "Defaults", Board: "esp32-2432s028r"})
	if err != nil || p.Theme != "clear" {
		t.Fatalf("default theme = %+v, %v; want clear", p, err)
	}
	entries, _ := os.ReadDir(filepath.Join(env.cfg.WorkspaceDir, "CYD"))
	if len(entries) != 1 {
		t.Fatalf("failed creates left directories: %v", entries)
	}
}

func TestListAndRenameProjects(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	a := env.createProject(t, "Alpha")
	b := env.createProject(t, "Beta")
	events, cancel := env.svc.Subscribe(a.ID)
	defer cancel()
	renamed, err := env.svc.RenameProject(ctx, a.ID, "Alpha Zwei")
	if err != nil || renamed.Name != "Alpha Zwei" || renamed.Version != 2 || renamed.Slug != "alpha" {
		t.Fatalf("RenameProject = %+v, %v", renamed, err)
	}
	layout, version, _, err := env.svc.GetLayout(ctx, a.ID)
	if err != nil || layout.Name != "Alpha Zwei" || version != 2 {
		t.Fatalf("layout after rename = %q v%d, %v", layout.Name, version, err)
	}
	ev := collectEvents(t, events, 1, 2*time.Second, EventProjectChanged)[0]
	if string(ev.Data) != `{"origin":"user","version":2,"what":"meta"}` {
		t.Fatalf("project_changed data = %s", ev.Data)
	}
	list, err := env.svc.ListProjects(ctx)
	if err != nil || len(list) != 2 || list[0].ID != a.ID || list[1].ID != b.ID {
		t.Fatalf("ListProjects = %+v, %v; want the renamed project first", list, err)
	}
	_, err = env.svc.RenameProject(ctx, a.ID, "")
	wantCode(t, err, CodeBadRequest)
	_, err = env.svc.GetProject(ctx, "ccp_missing")
	wantCode(t, err, CodeNotFound)
}

func TestDeleteProjectRemovesEverything(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Doomed")
	keep := env.createProject(t, "Kept")
	env.svc.Emit(p.ID, EventProjectChanged, "", "", nil)
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	if err := env.svc.DeleteProject(ctx, p.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := os.Stat(filepath.Join(env.cfg.WorkspaceDir, "CYD", "doomed")); !os.IsNotExist(err) {
		t.Fatalf("project directory still exists: %v", err)
	}
	_, err := env.svc.GetProject(ctx, p.ID)
	wantCode(t, err, CodeNotFound)
	var n int
	_ = env.svc.db.QueryRow(`SELECT COUNT(*) FROM cc_events WHERE project_id=?`, p.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("%d events of the deleted project remain", n)
	}
	if _, ok := <-events; ok {
		t.Fatal("subscription of the deleted project was not closed")
	}
	if _, err := env.svc.GetProject(ctx, keep.ID); err != nil {
		t.Fatalf("other project lost: %v", err)
	}
}

func (e *testEnv) createProject(t *testing.T, name string) ProjectSummary {
	t.Helper()
	p, err := e.svc.CreateProject(context.Background(), CreateProjectInput{Name: name, Board: "esp32-2432s028r", Theme: "night", Language: "de"})
	if err != nil {
		t.Fatalf("CreateProject(%q): %v", name, err)
	}
	return p
}

// lockForJob simulates plan 3 taking the project lock.
func (e *testEnv) lockForJob(t *testing.T, projectID, jobID string) {
	t.Helper()
	if _, err := e.svc.db.Exec(`UPDATE cc_projects SET locked_by_job=? WHERE id=?`, jobID, projectID); err != nil {
		t.Fatal(err)
	}
}
