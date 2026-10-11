package cydcreator

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func appChanged(t *testing.T, env *testEnv, projectID string) bool {
	t.Helper()
	row, err := env.svc.loadProjectRow(context.Background(), projectID)
	if err != nil {
		t.Fatal(err)
	}
	return row.AppChangedSinceJob
}

func TestWriteAndReadFilesWithSHAPreconditions(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Files")
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()

	sha, err := env.svc.WriteFile(ctx, p.ID, "src/app/sensor.cpp", []byte("int x;\n"), "*", OriginUser)
	if err != nil || sha != sha256Hex([]byte("int x;\n")) {
		t.Fatalf("create = %q, %v", sha, err)
	}
	if !appChanged(t, env, p.ID) {
		t.Fatal("a user write must set app_changed_since_job")
	}
	ev := collectEvents(t, events, 1, 2*time.Second, EventProjectChanged)[0]
	if string(ev.Data) != `{"origin":"user","version":1,"what":"files"}` {
		t.Fatalf("project_changed = %s", ev.Data)
	}
	data, gotSHA, err := env.svc.ReadFile(ctx, p.ID, "src/app/sensor.cpp")
	if err != nil || string(data) != "int x;\n" || gotSHA != sha {
		t.Fatalf("ReadFile = %q %q, %v", data, gotSHA, err)
	}
	_, err = env.svc.WriteFile(ctx, p.ID, "src/app/sensor.cpp", []byte("int y;\n"), "*", OriginUser)
	if typed := wantCode(t, err, CodeFileConflict); typed.Details["sha256"] != sha {
		t.Fatalf("create-only conflict details = %v", typed.Details)
	}
	_, err = env.svc.WriteFile(ctx, p.ID, "src/app/sensor.cpp", []byte("int y;\n"), strings.Repeat("0", 64), OriginUser)
	wantCode(t, err, CodeFileConflict)
	_, err = env.svc.WriteFile(ctx, p.ID, "src/app/missing.cpp", []byte("int y;\n"), sha, OriginUser)
	wantCode(t, err, CodeFileConflict)
	next, err := env.svc.WriteFile(ctx, p.ID, "src/app/sensor.cpp", []byte("int y;\n"), sha, OriginUser)
	if err != nil || next == sha {
		t.Fatalf("update = %q, %v", next, err)
	}

	if _, err := env.svc.db.Exec(`UPDATE cc_projects SET app_changed_since_job=0 WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.WriteFile(ctx, p.ID, "src/app/sensor.cpp", []byte("int z;\n"), "", JobOrigin("ccj_0009")); err != nil {
		t.Fatal(err)
	}
	if appChanged(t, env, p.ID) {
		t.Fatal("an agent write must not set app_changed_since_job")
	}
	_, _, err = env.svc.ReadFile(ctx, p.ID, "src/app/none.cpp")
	wantCode(t, err, CodeNotFound)
	_, _, err = env.svc.ReadFile(ctx, p.ID, "../../etc/passwd")
	wantCode(t, err, CodeNotFound)
}

func TestWriteFileRejectsReadOnlyAndInvalidPaths(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Paths")
	for rel, code := range map[string]string{
		"src/generated/ui.cpp":       CodeFileReadOnly,
		"lib/cyd_template/src/cyd.h": CodeFileReadOnly,
		"include/board_config.h":     CodeFileReadOnly,
		"platformio.ini":             CodeFileReadOnly,
		"partitions.csv":             CodeFileReadOnly,
		ProjectFile:                  CodeFileReadOnly,
		"assets/sun.png":             CodeFileReadOnly,
		"src/app/../../x.cpp":        CodeBadRequest,
		"src/app/readme.md":          CodeBadRequest,
		"src/app/.hidden.cpp":        CodeBadRequest,
		"src/main.cpp":               CodeBadRequest,
	} {
		_, err := env.svc.WriteFile(ctx, p.ID, rel, []byte("x"), "", OriginUser)
		wantCode(t, err, code)
		wantCode(t, env.svc.DeleteFile(ctx, p.ID, rel, "", OriginUser), code)
	}
	_, err := env.svc.WriteFile(ctx, p.ID, "src/app/bin.cpp", []byte{0xff, 0xfe, 0x00}, "*", OriginUser)
	wantCode(t, err, CodeBadRequest)
}

func TestWriteFileEnforcesLimits(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Limits")
	big := bytes.Repeat([]byte("a"), MaxAppFileBytes+1)
	_, err := env.svc.WriteFile(ctx, p.ID, "src/app/big.cpp", big, "*", OriginUser)
	if typed := wantCode(t, err, CodeFileLimit); typed.Details["limit"] != "file_bytes" {
		t.Fatalf("details = %v", typed.Details)
	}
	chunk := bytes.Repeat([]byte("b"), 250<<10)
	for i := 0; i < 8; i++ {
		if _, err := env.svc.WriteFile(ctx, p.ID, fmt.Sprintf("src/app/chunk%d.cpp", i), chunk, "*", OriginUser); err != nil {
			t.Fatalf("chunk %d: %v", i, err)
		}
	}
	_, err = env.svc.WriteFile(ctx, p.ID, "src/app/chunk8.cpp", chunk, "*", OriginUser)
	if typed := wantCode(t, err, CodeFileLimit); typed.Details["limit"] != "total_bytes" {
		t.Fatalf("details = %v", typed.Details)
	}
	for i := 0; i < 8; i++ {
		if err := env.svc.DeleteFile(ctx, p.ID, fmt.Sprintf("src/app/chunk%d.cpp", i), "", OriginUser); err != nil {
			t.Fatal(err)
		}
	}
	existing, _ := env.svc.ListFiles(ctx, p.ID)
	appFiles := 0
	for _, f := range existing {
		if f.Editable {
			appFiles++
		}
	}
	for i := appFiles; i < MaxAppFiles; i++ {
		if _, err := env.svc.WriteFile(ctx, p.ID, fmt.Sprintf("src/app/f%02d.h", i), []byte("//\n"), "*", OriginUser); err != nil {
			t.Fatalf("file %d: %v", i, err)
		}
	}
	_, err = env.svc.WriteFile(ctx, p.ID, "src/app/one_too_many.h", []byte("//\n"), "*", OriginUser)
	if typed := wantCode(t, err, CodeFileLimit); typed.Details["limit"] != "files" {
		t.Fatalf("details = %v", typed.Details)
	}
	if _, err := env.svc.WriteFile(ctx, p.ID, fmt.Sprintf("src/app/f%02d.h", appFiles), []byte("// replaced\n"), "", OriginUser); err != nil {
		t.Fatalf("overwriting an existing file at the file limit: %v", err)
	}
}

func TestWriteFileRejectsCaseCollisions(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Case")
	_, err := env.svc.WriteFile(ctx, p.ID, "src/app/APP_MAIN.cpp", []byte("x"), "*", OriginUser)
	wantCode(t, err, CodeFileConflict)
	// With the real file's sha as If-Match: on case-insensitive file systems the lookup
	// finds app_main.cpp, so the clash must be caught before the existence check.
	original, sha, err := env.svc.ReadFile(ctx, p.ID, "src/app/app_main.cpp")
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.WriteFile(ctx, p.ID, "src/app/APP_MAIN.cpp", []byte("x"), sha, OriginUser)
	if typed := wantCode(t, err, CodeFileConflict); typed.Details["existing"] != "src/app/app_main.cpp" {
		t.Fatalf("details = %v", typed.Details)
	}
	after, _, err := env.svc.ReadFile(ctx, p.ID, "src/app/app_main.cpp")
	if err != nil || !bytes.Equal(after, original) {
		t.Fatalf("app_main.cpp changed: %q, %v", after, err)
	}
}

func TestListFilesIncludesGeneratedPaths(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Listing")
	files, err := env.svc.ListFiles(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]FileInfo{}
	for _, f := range files {
		byPath[f.Path] = f
	}
	app, ok := byPath["src/app/app_main.cpp"]
	if !ok || !app.Editable || app.Generated || app.SHA256 == "" || app.Size == 0 {
		t.Fatalf("app_main entry = %+v (present %v)", app, ok)
	}
	gen, ok := byPath["src/generated/ui.cpp"]
	if !ok || gen.Editable || !gen.Generated || gen.SHA256 != sha256Hex([]byte("// Listing\n")) {
		t.Fatalf("generated entry = %+v (present %v)", gen, ok)
	}
	if _, ok := byPath[ProjectFile]; ok {
		t.Fatal("cyd.project.json must not be listed (layout API)")
	}
	for i := 1; i < len(files); i++ {
		if files[i-1].Path > files[i].Path {
			t.Fatalf("files are not sorted: %s > %s", files[i-1].Path, files[i].Path)
		}
	}
	data, sha, err := env.svc.ReadFile(ctx, p.ID, "src/generated/ui.cpp")
	if err != nil || string(data) != "// Listing\n" || sha != gen.SHA256 {
		t.Fatalf("ReadFile generated = %q %q, %v", data, sha, err)
	}
	_, _, err = env.svc.ReadFile(ctx, p.ID, "src/generated/none.cpp")
	wantCode(t, err, CodeNotFound)
}

func TestListFilesWithoutAWorkingGenerator(t *testing.T) {
	env := newTestService(t, testServiceOptions{generate: func(GenerateInput) (GenerateOutput, error) {
		return GenerateOutput{}, fmt.Errorf("boom")
	}})
	p := env.createProject(t, "NoGen")
	files, err := env.svc.ListFiles(context.Background(), p.ID)
	if err != nil || len(files) != 1 || files[0].Path != "src/app/app_main.cpp" {
		t.Fatalf("ListFiles = %+v, %v; want only the app file", files, err)
	}
	_, _, err = env.svc.ReadFile(context.Background(), p.ID, "src/generated/ui.cpp")
	wantCode(t, err, CodeGeneratorError)
}

func TestDeleteFile(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Delete")
	sha, err := env.svc.WriteFile(ctx, p.ID, "src/app/sub/deep.cpp", []byte("//\n"), "*", OriginUser)
	if err != nil {
		t.Fatal(err)
	}
	wantCode(t, env.svc.DeleteFile(ctx, p.ID, "src/app/sub/deep.cpp", strings.Repeat("1", 64), OriginUser), CodeFileConflict)
	if err := env.svc.DeleteFile(ctx, p.ID, "src/app/sub/deep.cpp", sha, OriginUser); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	dir := filepath.Join(env.cfg.WorkspaceDir, "CYD", "delete", "src", "app")
	if _, err := os.Stat(filepath.Join(dir, "sub")); !os.IsNotExist(err) {
		t.Fatal("the empty sub directory was kept")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal("src/app itself must stay")
	}
	wantCode(t, env.svc.DeleteFile(ctx, p.ID, "src/app/sub/deep.cpp", "", OriginUser), CodeNotFound)
}

// TestAppFilesNeedTheExactLetterCase: reads, deletes and writes never reach a
// file or folder through a differently cased path (on Windows and macOS
// opening src/app/APP_MAIN.cpp finds app_main.cpp).
func TestAppFilesNeedTheExactLetterCase(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Exact")
	appMain := filepath.Join(env.cfg.WorkspaceDir, "CYD", "exact", "src", "app", "app_main.cpp")
	original, err := os.ReadFile(appMain)
	if err != nil {
		t.Fatal(err)
	}
	wantCode(t, env.svc.DeleteFile(ctx, p.ID, "src/app/APP_MAIN.cpp", "", OriginUser), CodeNotFound)
	_, _, err = env.svc.ReadFile(ctx, p.ID, "src/app/APP_MAIN.cpp")
	wantCode(t, err, CodeNotFound)
	if after, err := os.ReadFile(appMain); err != nil || !bytes.Equal(after, original) {
		t.Fatalf("app_main.cpp changed: %v", err)
	}

	if _, err := env.svc.WriteFile(ctx, p.ID, "src/app/sub/deep.cpp", []byte("//\n"), "*", OriginUser); err != nil {
		t.Fatal(err)
	}
	wantCode(t, env.svc.DeleteFile(ctx, p.ID, "src/app/SUB/deep.cpp", "", OriginUser), CodeNotFound)
	_, _, err = env.svc.ReadFile(ctx, p.ID, "src/app/SUB/deep.cpp")
	wantCode(t, err, CodeNotFound)
	_, err = env.svc.WriteFile(ctx, p.ID, "src/app/SUB/new.cpp", []byte("//\n"), "*", OriginUser)
	if typed := wantCode(t, err, CodeFileConflict); typed.Details["existing"] != "src/app/sub" {
		t.Fatalf("folder clash details = %v", typed.Details)
	}
	_, err = env.svc.WriteFile(ctx, p.ID, "src/app/SUB/deep.cpp", []byte("// other\n"), "", OriginUser)
	wantCode(t, err, CodeFileConflict)
	files, err := env.svc.ListFiles(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	var editable []string
	for _, f := range files {
		if f.Editable {
			editable = append(editable, f.Path)
		}
	}
	if strings.Join(editable, ",") != "src/app/app_main.cpp,src/app/sub/deep.cpp" {
		t.Fatalf("app files = %v", editable)
	}
	data, _, err := env.svc.ReadFile(ctx, p.ID, "src/app/sub/deep.cpp")
	if err != nil || string(data) != "//\n" {
		t.Fatalf("deep.cpp = %q, %v", data, err)
	}
}

// TestOversizedAndNonRegularAppPaths: a file above the limits put into
// src/app by hand has no hash, so only an unconditional write or delete
// replaces it; a directory at a file path is refused and never removed.
func TestOversizedAndNonRegularAppPaths(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Hand")
	app := filepath.Join(env.cfg.WorkspaceDir, "CYD", "hand", "src", "app")
	huge := filepath.Join(app, "huge.cpp")
	writeHuge := func() {
		t.Helper()
		if err := os.WriteFile(huge, bytes.Repeat([]byte("h"), MaxAppTotalBytes+1), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	writeHuge()
	_, _, err := env.svc.ReadFile(ctx, p.ID, "src/app/huge.cpp")
	wantCode(t, err, CodeFileLimit)
	err = env.svc.DeleteFile(ctx, p.ID, "src/app/huge.cpp", strings.Repeat("1", 64), OriginUser)
	if typed := wantCode(t, err, CodeFileConflict); typed.Details["sha256"] != "" {
		t.Fatalf("conflict details = %v", typed.Details)
	}
	_, err = env.svc.WriteFile(ctx, p.ID, "src/app/huge.cpp", []byte("int h;\n"), "*", OriginUser)
	wantCode(t, err, CodeFileConflict)
	if _, err := env.svc.WriteFile(ctx, p.ID, "src/app/huge.cpp", []byte("int h;\n"), "", OriginUser); err != nil {
		t.Fatalf("unconditional overwrite: %v", err)
	}
	writeHuge()
	if err := env.svc.DeleteFile(ctx, p.ID, "src/app/huge.cpp", "*", OriginUser); err != nil {
		t.Fatalf("unconditional delete: %v", err)
	}
	if _, err := os.Stat(huge); !os.IsNotExist(err) {
		t.Fatal("huge.cpp was kept")
	}

	inner := filepath.Join(app, "dir.cpp", "inner.h")
	if err := os.MkdirAll(filepath.Dir(inner), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inner, []byte("//\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	notRegular := func(err error) {
		t.Helper()
		if typed := wantCode(t, err, CodeBadRequest); typed.Details["reason"] != "not_regular" {
			t.Fatalf("details = %v", typed.Details)
		}
	}
	notRegular(env.svc.DeleteFile(ctx, p.ID, "src/app/dir.cpp", "", OriginUser))
	_, err = env.svc.WriteFile(ctx, p.ID, "src/app/dir.cpp", []byte("x"), "", OriginUser)
	notRegular(err)
	_, _, err = env.svc.ReadFile(ctx, p.ID, "src/app/dir.cpp")
	notRegular(err)
	if _, err := os.Stat(inner); err != nil {
		t.Fatalf("the directory content was removed: %v", err)
	}
}
