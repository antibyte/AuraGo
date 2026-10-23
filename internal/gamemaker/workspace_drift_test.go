package gamemaker

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceDriftComparesRevisionWithoutWritingBlobs(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	ctx := context.Background()

	empty, err := service.WorkspaceDrift(ctx, project)
	if err != nil {
		t.Fatalf("revision 0: %v", err)
	}
	if !empty.Empty() || empty.Revision != 0 {
		t.Fatalf("revision 0 drift = %+v", empty)
	}

	removedProject := seedRevision(t, service, createTestProject(t, service, "2d"), map[string][]byte{
		"src/main.ts": []byte("published"),
	}, false)
	removed, err := service.WorkspaceDrift(ctx, removedProject)
	if err != nil {
		t.Fatalf("missing workspace: %v", err)
	}
	if removed.RemovedCount != 1 || len(removed.Removed) != 1 || removed.Removed[0] != "src/main.ts" || removed.AddedCount != 0 {
		t.Fatalf("missing workspace drift = %+v", removed)
	}

	project = seedRevision(t, service, createTestProject(t, service, "2d"), map[string][]byte{
		"src/main.ts":            []byte("published"),
		"notes/kept.txt":         []byte("same"),
		"gone.txt":               []byte("remove me"),
		".aurago/game-plan.json": []byte(`{"template":"minimal"}`),
	}, true)
	root := filepath.Join(service.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey))
	writeDriftFile(t, root, "src/main.ts", "edited")
	writeDriftFile(t, root, "notes/added.txt", "new")
	if err := os.Remove(filepath.Join(root, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	writeDriftFile(t, root, ".aurago/game-plan.json", `{"template":"changed"}`)
	writeDriftFile(t, root, ".hidden/skip.txt", "secret")

	drift, err := service.WorkspaceDrift(ctx, project)
	if err != nil {
		t.Fatalf("drift: %v", err)
	}
	if drift.ChangedCount != 1 || len(drift.Changed) != 1 || drift.Changed[0] != "src/main.ts" {
		t.Fatalf("changed = %+v", drift.Changed)
	}
	if drift.AddedCount != 1 || len(drift.Added) != 1 || drift.Added[0] != "notes/added.txt" {
		t.Fatalf("added = %+v", drift.Added)
	}
	if drift.RemovedCount != 1 || len(drift.Removed) != 1 || drift.Removed[0] != "gone.txt" {
		t.Fatalf("removed = %+v", drift.Removed)
	}
	if drift.Truncated || drift.Revision != 1 {
		t.Fatalf("meta = %+v", drift)
	}
	encoded, _ := json.Marshal(drift)
	if strings.Contains(string(encoded), service.opts.WorkspacePath) || strings.Contains(string(encoded), "game-plan") || strings.Contains(string(encoded), ".hidden") {
		t.Fatalf("drift leaked a host or dot path: %s", encoded)
	}
	blobs, _ := os.ReadDir(service.blobDir)
	if len(blobs) != 0 {
		t.Fatalf("drift wrote %d blob entries", len(blobs))
	}
}

func TestWorkspaceDriftCapsPathLists(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	files := map[string][]byte{}
	for i := 0; i < driftListCap+1; i++ {
		files[twoDigits(i)+".txt"] = []byte("old")
	}
	project = seedRevision(t, service, project, files, true)
	root := filepath.Join(service.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey))
	for i := 0; i < driftListCap+1; i++ {
		writeDriftFile(t, root, twoDigits(i)+".txt", "new")
	}
	drift, err := service.WorkspaceDrift(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	if drift.ChangedCount != driftListCap+1 || len(drift.Changed) != driftListCap || !drift.Truncated {
		t.Fatalf("capped drift = count %d len %d truncated %v", drift.ChangedCount, len(drift.Changed), drift.Truncated)
	}
	view := drift.PromptView()
	if view["changed_count"] != driftListCap+1 || len(view["changed"].([]string)) != driftListCap {
		t.Fatalf("prompt view = %#v", view)
	}
}

func TestWorkspaceDriftRejectsSymlink(t *testing.T) {
	service := newTestService(t)
	project := seedRevision(t, service, createTestProject(t, service, "2d"), map[string][]byte{"src/main.ts": []byte("ok")}, true)
	root := filepath.Join(service.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey))
	target := filepath.Join(root, "src", "main.ts")
	link := filepath.Join(root, "src", "linked.ts")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink is unavailable: %v", err)
	}
	_, err := service.WorkspaceDrift(context.Background(), project)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink error = %v", err)
	}
	if strings.Contains(err.Error(), service.opts.WorkspacePath) {
		t.Fatalf("symlink error contains a host path: %s", err)
	}
}

func TestPlanSummaryPayloadStaysBounded(t *testing.T) {
	controls := map[string]string{}
	for _, key := range []string{"move", "primary", "restart", "zoom", "map", "pause", "inventory", "drop", "extra"} {
		controls[key] = "bound"
	}
	assets := make([]PlanAsset, 0, 13)
	for i := 0; i < 13; i++ {
		assets = append(assets, PlanAsset{Role: "prop", AssetID: twoDigits(i)})
	}
	assets[12].AssetID = ""
	assets[12].AssemblyID = "asm"
	payload := planSummaryPayload(Job{SmallModel: true, BuildingMode: BuildingModeAdapt}, &GamePlan{
		Template:    "minimal",
		Objective:   strings.Repeat("ä", 241),
		Perspective: "top",
		Controls:    controls,
		Assets:      assets,
	})
	if got := []rune(payload["objective"].(string)); len(got) != 240 {
		t.Fatalf("objective runes = %d", len(got))
	}
	if len(payload["controls"].([]string)) != 8 || len(payload["assets"].([]map[string]string)) != 12 {
		t.Fatalf("payload = %#v", payload)
	}
	if _, ok := payload["scene"]; ok {
		t.Fatal("summary included scene")
	}
	// The plan summary names the small-model decision the job was started with.
	if payload["small_model"] != true || payload["building_mode"] != BuildingModeAdapt {
		t.Fatalf("summary lost the small-model decision: %#v", payload)
	}
	if quiet := planSummaryPayload(Job{BuildingMode: BuildingModeRewrite}, &GamePlan{Template: "minimal"}); quiet["small_model"] != nil {
		t.Fatalf("normal summary claims a small model: %#v", quiet)
	}
}

func TestJobOutcomeColumnsMigrateOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gm.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE gm_jobs (
		id TEXT PRIMARY KEY, project_id TEXT NOT NULL, kind TEXT NOT NULL, prompt TEXT NOT NULL,
		status TEXT NOT NULL, phase TEXT NOT NULL, provider_id TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL DEFAULT '', error TEXT NOT NULL DEFAULT '',
		base_revision INTEGER NOT NULL DEFAULT 0, result_revision INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL, started_at DATETIME, finished_at DATETIME)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO gm_jobs(id,project_id,kind,prompt,status,phase,created_at) VALUES('job','project','edit','change','failed','failed',?)`, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := ensureJobOutcomeColumns(db); err != nil {
		t.Fatal(err)
	}
	if err := ensureJobOutcomeColumns(db); err != nil {
		t.Fatal(err)
	}
	backups, _ := filepath.Glob(path + ".before-job-outcome-*.bak")
	if len(backups) != 1 {
		t.Fatalf("backups = %d", len(backups))
	}
	job, err := scanJob(db.QueryRow(`SELECT ` + jobSelectColumns + ` FROM gm_jobs WHERE id='job'`))
	if err != nil {
		t.Fatal(err)
	}
	if job.ErrorCode != "" || job.ChecksPassed != nil || job.ChecksFailed != nil || job.RepairPasses != nil || job.GameplayStatus != "" {
		t.Fatalf("old job gained outcome data: %+v", job)
	}
	// Additive small-model and building-mode columns keep old rows ordinary.
	if job.SmallModel || job.BuildingMode != "" {
		t.Fatalf("old job gained a small-model decision: %+v", job)
	}
	var smallModel int
	var buildingMode string
	if err := db.QueryRow(`SELECT small_model, building_mode FROM gm_jobs WHERE id='job'`).Scan(&smallModel, &buildingMode); err != nil {
		t.Fatal(err)
	}
	if smallModel != 0 || buildingMode != "" {
		t.Fatalf("migrated defaults = %d/%q", smallModel, buildingMode)
	}

	fresh := newTestService(t)
	freshBackups, _ := filepath.Glob(fresh.opts.DBPath + ".before-job-outcome-*.bak")
	if len(freshBackups) != 0 {
		t.Fatalf("fresh database wrote %d backups", len(freshBackups))
	}
}

func seedRevision(t *testing.T, service *Service, project Project, files map[string][]byte, writeWorkspace bool) Project {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	var total int64
	for _, data := range files {
		total += int64(len(data))
	}
	result, err := service.db.ExecContext(ctx, `INSERT INTO gm_revisions(project_id,number,parent,source,summary,file_count,total_bytes,created_at)
		VALUES(?,1,0,'test','fixture',?,?,?)`, project.ID, len(files), total, now)
	if err != nil {
		t.Fatal(err)
	}
	revisionID, _ := result.LastInsertId()
	root := filepath.Join(service.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey))
	for path, data := range files {
		sum := sha256.Sum256(data)
		if _, err := service.db.ExecContext(ctx, `INSERT INTO gm_revision_files(revision_id,path,content_hash,size) VALUES(?,?,?,?)`,
			revisionID, path, hex.EncodeToString(sum[:]), len(data)); err != nil {
			t.Fatal(err)
		}
		if writeWorkspace {
			writeDriftFile(t, root, path, string(data))
		}
	}
	if _, err := service.db.ExecContext(ctx, `UPDATE gm_projects SET current_revision=1,updated_at=? WHERE id=?`, now, project.ID); err != nil {
		t.Fatal(err)
	}
	reloaded, err := service.GetProject(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	return reloaded
}

func writeDriftFile(t *testing.T, root, rel, content string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
}

func twoDigits(value int) string {
	if value < 10 {
		return "0" + string(rune('0'+value))
	}
	return string(rune('0'+value/10)) + string(rune('0'+value%10))
}
