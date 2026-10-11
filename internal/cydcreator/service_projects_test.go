package cydcreator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

func TestSlugifyAvoidsWindowsDeviceNames(t *testing.T) {
	for in, want := range map[string]string{
		"nul": "nul-project", "NUL": "nul-project", "Con": "con-project", "aux": "aux-project", "PRN": "prn-project",
		"COM1": "com1-project", "Lpt9": "lpt9-project", "con.txt": "con-txt", "console": "console", "com10": "com10",
	} {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
	env := newTestService(t)
	p := env.createProject(t, "NUL")
	if p.Slug != "nul-project" {
		t.Fatalf("slug = %q, want nul-project", p.Slug)
	}
	if _, err := os.Stat(filepath.Join(env.cfg.WorkspaceDir, "CYD", "nul-project", ProjectFile)); err != nil {
		t.Fatalf("project file missing: %v", err)
	}
}

func TestCreateProjectIsSafeConcurrently(t *testing.T) {
	env := newTestService(t)
	const n = 8
	var wg sync.WaitGroup
	results := make([]ProjectSummary, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = env.svc.CreateProject(context.Background(), CreateProjectInput{Name: "Uhr", Board: "esp32-2432s028r"})
		}(i)
	}
	wg.Wait()
	slugs, ids := map[string]bool{}, map[string]bool{}
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("CreateProject %d: %v", i, errs[i])
		}
		slugs[results[i].Slug], ids[results[i].ID] = true, true
	}
	if len(slugs) != n || len(ids) != n || !slugs["uhr"] || !slugs[fmt.Sprintf("uhr-%d", n)] {
		t.Fatalf("slugs = %v, ids = %v; want %d distinct (uhr .. uhr-%d)", slugs, ids, n, n)
	}
	list, err := env.svc.ListProjects(context.Background())
	if err != nil || len(list) != n {
		t.Fatalf("ListProjects = %d, %v", len(list), err)
	}
}

func TestCreateProjectRecreatesTheProjectsFolder(t *testing.T) {
	env := newTestService(t)
	if err := os.RemoveAll(filepath.Join(env.cfg.WorkspaceDir, "CYD")); err != nil {
		t.Fatal(err)
	}
	p := env.createProject(t, "Neu")
	if _, err := os.Stat(filepath.Join(env.cfg.WorkspaceDir, "CYD", p.Slug, ProjectFile)); err != nil {
		t.Fatalf("project file missing: %v", err)
	}
}

func TestProjectWritesHonourTheJobLock(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Gesperrt")
	layout, version, _, _ := env.svc.GetLayout(ctx, p.ID)
	env.lockForJob(t, p.ID, "ccj_0001")

	_, err := env.svc.RenameProject(ctx, p.ID, "Anders")
	wantCode(t, err, CodeProjectLocked)
	_, _, err = env.svc.PutLayout(ctx, p.ID, layout, version, OriginUser)
	wantCode(t, err, CodeProjectLocked)
	_, _, err = env.svc.PutLayout(ctx, p.ID, layout, version, JobOrigin("ccj_0002"))
	wantCode(t, err, CodeProjectLocked)
	wantCode(t, env.svc.DeleteProject(ctx, p.ID), CodeProjectLocked)

	next, _, err := env.svc.PutLayout(ctx, p.ID, layout, version, JobOrigin("ccj_0001"))
	if err != nil || next != version+1 {
		t.Fatalf("PutLayout by the lock holder = v%d, %v", next, err)
	}
	got, err := env.svc.GetProject(ctx, p.ID)
	if err != nil || got.LockedByJob != "ccj_0001" || got.Name != "Gesperrt" {
		t.Fatalf("project after locked writes = %+v, %v", got, err)
	}
}

func TestDeleteProjectRefusesWhileABuildIsActive(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Baut")
	if _, err := env.svc.db.Exec(`INSERT INTO cc_builds(id,project_id,origin,status,board,project_version,created_at)
		VALUES('ccb_0001',?,'user','running','esp32-2432s028r',1,'2026-10-10T08:00:00.000Z')`, p.ID); err != nil {
		t.Fatal(err)
	}
	artifacts := filepath.Join(env.cfg.DataDir, "cyd_creator", "artifacts", "ccb_0001")
	if err := os.MkdirAll(artifacts, 0o750); err != nil {
		t.Fatal(err)
	}
	wantCode(t, env.svc.DeleteProject(ctx, p.ID), CodeBusy)
	if _, err := env.svc.GetProject(ctx, p.ID); err != nil {
		t.Fatalf("refused delete removed the project: %v", err)
	}
	if _, err := env.svc.db.Exec(`UPDATE cc_builds SET status='succeeded' WHERE id='ccb_0001'`); err != nil {
		t.Fatal(err)
	}
	if err := env.svc.DeleteProject(ctx, p.ID); err != nil {
		t.Fatalf("DeleteProject after the build: %v", err)
	}
	if _, err := os.Stat(artifacts); !os.IsNotExist(err) {
		t.Fatalf("artefacts of the deleted project remain: %v", err)
	}
	var n int
	_ = env.svc.db.QueryRow(`SELECT COUNT(*) FROM cc_builds WHERE project_id=?`, p.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("%d builds of the deleted project remain", n)
	}
}

func TestDeleteProjectFailsSafelyWhenBuildsCannotBeChecked(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Sicher")
	if _, err := env.svc.db.Exec(`DROP TABLE cc_builds`); err != nil {
		t.Fatal(err)
	}
	wantCode(t, env.svc.DeleteProject(ctx, p.ID), CodeInternal)
	if _, err := env.svc.GetProject(ctx, p.ID); err != nil {
		t.Fatalf("failed delete removed the row: %v", err)
	}
	if _, err := os.Stat(filepath.Join(env.cfg.WorkspaceDir, "CYD", p.Slug)); err != nil {
		t.Fatalf("failed delete removed the directory: %v", err)
	}
}

func TestDeleteProjectWithoutItsDirectory(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Verschwunden")
	if err := os.RemoveAll(filepath.Join(env.cfg.WorkspaceDir, "CYD", p.Slug)); err != nil {
		t.Fatal(err)
	}
	if err := env.svc.DeleteProject(ctx, p.ID); err != nil {
		t.Fatalf("DeleteProject without directory: %v", err)
	}
	_, err := env.svc.GetProject(ctx, p.ID)
	wantCode(t, err, CodeNotFound)
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
