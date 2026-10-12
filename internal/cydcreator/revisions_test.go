package cydcreator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readProjectFile(t *testing.T, env *testEnv, p ProjectSummary, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(env.cfg.WorkspaceDir, "CYD", p.Slug, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
}

func TestRevisionDiffAndRestore(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	scaffold := readProjectFile(t, env, p, "src/app/app_main.cpp")
	row, _ := env.svc.loadProjectRow(ctx, p.ID)
	root, err := env.svc.openRoot(row, "")
	if err != nil {
		t.Fatal(err)
	}
	before, err := env.svc.createRevision(ctx, p.ID, root, "Show   the\ntemperature", RevisionBeforeJob, "ccj_1", "")
	if err != nil || before.Label != "Show the temperature" {
		t.Fatalf("revision = %+v, %v", before, err)
	}
	if _, err := env.svc.WriteFile(ctx, p.ID, "src/app/app_main.cpp", []byte(scaffold+"// agent\n"), "", OriginUser); err != nil {
		t.Fatal(err)
	}
	after, err := env.svc.createRevision(ctx, p.ID, root, "Job done", RevisionJob, "ccj_1", "ccb_1")
	// The swap renames the project directory: no handle may stay open (Windows).
	_ = root.Close()
	if err != nil {
		t.Fatal(err)
	}
	revs, _ := env.svc.ListRevisions(ctx, p.ID)
	if len(revs) != 2 || revs[0].ID != after.ID || revs[0].BuildID != "ccb_1" {
		t.Fatalf("revisions = %+v", revs)
	}
	diff, err := env.svc.RevisionDiff(ctx, after.ID)
	if err != nil {
		t.Fatal(err)
	}
	if diff.LayoutChanged || len(diff.Files) != 1 || diff.Files[0].Path != "src/app/app_main.cpp" || diff.Files[0].Status != "modified" ||
		!strings.Contains(diff.Files[0].Diff, "+// agent") {
		t.Fatalf("job revision diff = %+v", diff)
	}
	first, _ := env.svc.RevisionDiff(ctx, before.ID)
	if !first.LayoutChanged || len(first.Files) != 1 || first.Files[0].Status != "added" || !strings.HasPrefix(first.Files[0].Diff, "--- /dev/null") {
		t.Fatalf("first revision diff = %+v", first)
	}
	if _, err := env.svc.RevisionDiff(ctx, "ccr_missing"); ErrorCode(err) != CodeNotFound {
		t.Fatalf("missing revision: %v", err)
	}

	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	summary, _ := env.svc.GetProject(ctx, p.ID)
	version, rev, err := env.svc.RestoreRevision(ctx, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if version != summary.Version+1 || rev.Reason != RevisionRestore || rev.Label != before.Label {
		t.Fatalf("restore = v%d %+v (was v%d)", version, rev, summary.Version)
	}
	if got := readProjectFile(t, env, p, "src/app/app_main.cpp"); got != scaffold {
		t.Fatalf("restored file = %q", got)
	}
	got := collectEvents(t, events, 2, 2*time.Second, EventProjectChanged)
	var whats []string
	for _, ev := range got {
		var d struct{ Origin, What string }
		_ = json.Unmarshal(ev.Data, &d)
		whats = append(whats, d.Origin+"/"+d.What)
	}
	if strings.Join(whats, ",") != "restore/layout,restore/files" {
		t.Fatalf("events = %v", whats)
	}
	row, _ = env.svc.loadProjectRow(ctx, p.ID)
	if !row.AppChangedSinceJob {
		t.Fatal("restore did not mark the app files as changed by the user")
	}
	revs, _ = env.svc.ListRevisions(ctx, p.ID)
	if len(revs) != 3 || revs[0].ID != rev.ID {
		t.Fatalf("revisions after restore = %+v", revs)
	}
	if entries, _ := os.ReadDir(env.svc.stagingRoot()); len(entries) != 0 {
		t.Fatalf("restore stage left: %v", entries)
	}
	if _, _, err := env.svc.RestoreRevision(ctx, "ccr_missing"); ErrorCode(err) != CodeNotFound {
		t.Fatalf("missing revision: %v", err)
	}
}

func TestRestoreRespectsTheProjectLock(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	row, _ := env.svc.loadProjectRow(ctx, p.ID)
	root, err := env.svc.openRoot(row, "")
	if err != nil {
		t.Fatal(err)
	}
	rev, err := env.svc.createRevision(ctx, p.ID, root, "x", RevisionJob, "", "")
	_ = root.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := env.svc.setProjectLock(ctx, p.ID, "ccj_busy"); err != nil {
		t.Fatal(err)
	}
	_, _, err = env.svc.RestoreRevision(ctx, rev.ID)
	wantCode(t, err, CodeProjectLocked)
}

func countBlobs(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}

func TestRevisionsAreTrimmedAndBlobsDeduplicatedAndPruned(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	row, _ := env.svc.loadProjectRow(ctx, p.ID)
	root, err := env.svc.openRoot(row, "")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for i := 0; i < maxRevisionsPerProject+3; i++ {
		if _, err := env.svc.createRevision(ctx, p.ID, root, fmt.Sprintf("rev %d", i), RevisionJob, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	revs, _ := env.svc.ListRevisions(ctx, p.ID)
	if len(revs) != maxRevisionsPerProject || revs[len(revs)-1].Label != "rev 3" {
		t.Fatalf("%d revisions, oldest %q", len(revs), revs[len(revs)-1].Label)
	}
	var files int
	_ = env.svc.db.QueryRow(`SELECT COUNT(*) FROM cc_revision_files`).Scan(&files)
	if files != maxRevisionsPerProject {
		t.Fatalf("%d revision file rows", files)
	}
	if n := countBlobs(t, env.svc.blobsDir()); n != 2 {
		t.Fatalf("%d blobs, want layout + app_main.cpp", n)
	}
	if err := env.svc.DeleteProject(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := env.svc.pruneOrphanBlobs(ctx); err != nil {
		t.Fatal(err)
	}
	if n := countBlobs(t, env.svc.blobsDir()); n != 0 {
		t.Fatalf("%d blobs left after pruning", n)
	}
}

func TestRecordRevision(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	rev, err := env.svc.RecordRevision(ctx, p.ID, "Import  wetter.zip", RevisionImport)
	if err != nil || rev.Reason != RevisionImport || rev.Label != "Import wetter.zip" || rev.JobID != "" {
		t.Fatalf("RecordRevision = %+v, %v", rev, err)
	}
	revs, err := env.svc.ListRevisions(ctx, p.ID)
	if err != nil || len(revs) != 1 || revs[0].ID != rev.ID {
		t.Fatalf("revisions = %+v, %v", revs, err)
	}
	if _, err := env.svc.RecordRevision(ctx, p.ID, "x", "whatever"); ErrorCode(err) != CodeBadRequest {
		t.Fatalf("unknown reason: %v", err)
	}
	if _, err := env.svc.RecordRevision(ctx, "ccp_missing", "x", RevisionImport); ErrorCode(err) != CodeNotFound {
		t.Fatalf("unknown project: %v", err)
	}
}

// recordRaw stores the current project directory as a revision.
func recordRaw(t *testing.T, env *testEnv, p ProjectSummary) Revision {
	t.Helper()
	rev, err := env.svc.RecordRevision(context.Background(), p.ID, "raw", RevisionImport)
	if err != nil {
		t.Fatal(err)
	}
	return rev
}

func TestRestoreRefusesABrokenAssetEntry(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	layout, _, _, err := env.svc.GetLayout(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	layout.Assets = []Asset{{ID: "a_bad", File: "src/app/app_main.cpp", W: 1, H: 1}}
	data, err := MarshalProject(layout)
	if err != nil {
		t.Fatal(err)
	}
	original := readProjectFile(t, env, p, ProjectFile)
	path := filepath.Join(env.svc.projectDir(p.Slug), ProjectFile)
	writeTestFile(t, path, string(data))
	rev := recordRaw(t, env, p)
	writeTestFile(t, path, original)
	_, _, err = env.svc.RestoreRevision(ctx, rev.ID)
	if e := wantCode(t, err, CodeInvalidLayout); e.Details["asset"] != "a_bad" {
		t.Fatalf("details = %v", e.Details)
	}
	if got := readProjectFile(t, env, p, ProjectFile); got != original {
		t.Fatal("the refused restore changed the project")
	}
	if entries, _ := os.ReadDir(env.svc.stagingRoot()); len(entries) != 0 {
		t.Fatalf("restore stage left: %v", entries)
	}
}

func TestRestoreRefusesCaseVariantPaths(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	rev := recordRaw(t, env, p)
	sha, err := env.svc.writeBlob([]byte("int b;\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.db.Exec(`INSERT INTO cc_revision_files(revision_id,path,sha256,size) VALUES(?,?,?,?)`,
		rev.ID, "src/app/APP_MAIN.cpp", sha, 7); err != nil {
		t.Fatal(err)
	}
	_, _, err = env.svc.RestoreRevision(ctx, rev.ID)
	if e := wantCode(t, err, CodeFileConflict); e.Details["path"] != "src/app/app_main.cpp" || e.Details["existing"] != "src/app/APP_MAIN.cpp" {
		t.Fatalf("details = %v", e.Details)
	}
}

func TestRevisionDiffShowsBinaryAppFilesAsANote(t *testing.T) {
	env, p := newProjectEnv(t)
	recordRaw(t, env, p)
	writeTestFile(t, filepath.Join(env.svc.projectDir(p.Slug), "src", "app", "blob.h"), "a\x00b\n")
	rev := recordRaw(t, env, p)
	diff, err := env.svc.RevisionDiff(context.Background(), rev.ID)
	if err != nil || len(diff.Files) != 1 || diff.Files[0].Diff != "Binary file, 0 → 4 bytes\n" {
		t.Fatalf("diff = %+v, %v", diff, err)
	}
}

func TestReadBlobRejectsCorruption(t *testing.T) {
	env := newTestService(t)
	sha, err := env.svc.writeBlob([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(env.svc.blobPath(sha), []byte("jello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.readBlob(sha); err == nil {
		t.Fatal("corrupt blob accepted")
	}
	if _, err := env.svc.readBlob("../x"); err == nil {
		t.Fatal("invalid blob name accepted")
	}
}

func TestReadBlobIsCapped(t *testing.T) {
	env := newTestService(t)
	name := strings.Repeat("ab", 32)
	path := env.svc.blobPath(name)
	writeTestFile(t, path, "")
	if err := os.Truncate(path, maxBlobBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.readBlob(name); !errors.Is(err, errFileTooLarge) {
		t.Fatalf("oversized blob: %v", err)
	}
}

func TestRestoreRefusesCaseVariantFolders(t *testing.T) {
	env, p := newProjectEnv(t)
	rev := recordRaw(t, env, p)
	sha, err := env.svc.writeBlob([]byte("int b;\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"src/app/net/a.cpp", "src/app/NET/b.cpp"} {
		if _, err := env.svc.db.Exec(`INSERT INTO cc_revision_files(revision_id,path,sha256,size) VALUES(?,?,?,?)`, rev.ID, path, sha, 7); err != nil {
			t.Fatal(err)
		}
	}
	_, _, err = env.svc.RestoreRevision(context.Background(), rev.ID)
	if e := wantCode(t, err, CodeFileConflict); e.Details["path"] != "src/app/net" || e.Details["existing"] != "src/app/NET" {
		t.Fatalf("details = %v", e.Details)
	}
}

func TestMalformedRevisionIDsAreNotFound(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	for _, id := range []string{"../x", "a/b", "", strings.Repeat("r", 65)} {
		if _, err := env.svc.RevisionDiff(ctx, id); ErrorCode(err) != CodeNotFound {
			t.Fatalf("RevisionDiff(%q) = %v", id, err)
		}
		if _, _, err := env.svc.RestoreRevision(ctx, id); ErrorCode(err) != CodeNotFound {
			t.Fatalf("RestoreRevision(%q) = %v", id, err)
		}
	}
}

func TestLayoutRowMeta(t *testing.T) {
	row := projectRow{Name: "Old", Board: "esp32-2432s028r", Theme: "night"}
	name, board, theme := layoutRowMeta(row, Project{Name: "New", Board: "esp32-2432s024r", Theme: "clear"})
	if name != "New" || board != "esp32-2432s024r" || theme != "clear" {
		t.Fatalf("valid values = %q %q %q", name, board, theme)
	}
	name, board, theme = layoutRowMeta(row, Project{Name: "", Board: "no-such-board", Theme: "no-such-theme"})
	if name != "Old" || board != row.Board || theme != row.Theme {
		t.Fatalf("invalid values = %q %q %q", name, board, theme)
	}
}

func TestRestoreSavesUnrecordedChangesFirst(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	rev := recordRaw(t, env, p)
	writeTestFile(t, filepath.Join(env.svc.projectDir(p.Slug), "src", "app", "untracked.h"), "int u;\n")
	if _, _, err := env.svc.RestoreRevision(ctx, rev.ID); err != nil {
		t.Fatal(err)
	}
	revs, _ := env.svc.ListRevisions(ctx, p.ID)
	if len(revs) != 3 || revs[1].Label != "Before restoring: raw" || revs[1].Reason != RevisionRestore {
		t.Fatalf("revisions = %+v", revs)
	}
	saved, err := env.svc.RevisionDiff(ctx, revs[1].ID)
	if err != nil || len(saved.Files) != 1 || saved.Files[0].Path != "src/app/untracked.h" || saved.Files[0].Status != "added" {
		t.Fatalf("saved state diff = %+v, %v", saved, err)
	}
	if _, err := os.Stat(filepath.Join(env.svc.projectDir(p.Slug), "src", "app", "untracked.h")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the restore kept a file the revision does not have")
	}
	// Nothing changed since the newest revision: no extra snapshot.
	if _, _, err := env.svc.RestoreRevision(ctx, revs[1].ID); err != nil {
		t.Fatal(err)
	}
	if again, _ := env.svc.ListRevisions(ctx, p.ID); len(again) != 4 {
		t.Fatalf("%d revisions after a restore without changes, want 4", len(again))
	}
}

func TestRevisionDiffTakesAssetSizesFromTheRows(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	recordRaw(t, env, p)
	writeTestFile(t, filepath.Join(env.svc.projectDir(p.Slug), "assets", "logo.png"), "not really a png")
	rev := recordRaw(t, env, p)
	files, _ := env.svc.revisionFiles(ctx, rev.ID)
	if err := os.Remove(env.svc.blobPath(files["assets/logo.png"])); err != nil {
		t.Fatal(err)
	}
	diff, err := env.svc.RevisionDiff(ctx, rev.ID)
	if err != nil || len(diff.Files) != 1 || diff.Files[0].Diff != "Binary file, 0 → 16 bytes\n" {
		t.Fatalf("diff = %+v, %v", diff, err)
	}
}

func TestRevisionDiffIsCappedInTotal(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	const files, lines = 20, 8000
	write := func(prefix string) {
		for i := 0; i < files; i++ {
			var b strings.Builder
			for n := 0; n < lines; n++ {
				fmt.Fprintf(&b, "%s%02d-%d\n", prefix, i, n)
			}
			writeTestFile(t, filepath.Join(env.svc.projectDir(p.Slug), "src", "app", fmt.Sprintf("f%02d.cpp", i)), b.String())
		}
	}
	write("a")
	recordRaw(t, env, p)
	write("b")
	rev := recordRaw(t, env, p)
	diff, err := env.svc.RevisionDiff(ctx, rev.ID)
	if err != nil || len(diff.Files) != files {
		t.Fatalf("diff = %d files, %v", len(diff.Files), err)
	}
	total, omitted := 0, 0
	for _, f := range diff.Files {
		total += len(f.Diff)
		if strings.HasPrefix(f.Diff, "… diff omitted") {
			omitted++
		}
	}
	if omitted == 0 || total > maxRevisionDiffBytes+maxDiffOutput+files*64 {
		t.Fatalf("total %d bytes, %d files omitted", total, omitted)
	}
}
