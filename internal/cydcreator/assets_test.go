package cydcreator

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSanitizeAssetBase(t *testing.T) {
	cases := map[string]string{
		"Sun.PNG":                        "sun",
		"../../etc/passwd.png":           "passwd",
		`C:\Users\me\Wolke grau.jpeg`:    "wolke_grau",
		"__init__.png":                   "init",
		"Überschrift (2).jpg":            "ueberschrift_2",
		".png":                           "image",
		"":                               "image",
		strings.Repeat("x", 80) + ".png": strings.Repeat("x", 58),
	}
	for in, want := range cases {
		got := sanitizeAssetBase(in)
		if got != want {
			t.Errorf("sanitizeAssetBase(%q) = %q, want %q", in, got, want)
		}
		if !assetFileRegex.MatchString("assets/" + got + "_999.png") {
			t.Errorf("%q does not satisfy the asset file rule", got)
		}
	}
}

func TestPutAssetStoresTheImageAndUpdatesTheLayout(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Assets")
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	data := pngBytes(t, 32, 16, false)
	asset, version, err := env.svc.PutAsset(ctx, p.ID, "Sun.PNG", data)
	if err != nil {
		t.Fatalf("PutAsset: %v", err)
	}
	if asset.File != "assets/sun.png" || asset.W != 32 || asset.H != 16 || asset.Alpha || asset.ID == "" || version != 2 {
		t.Fatalf("asset = %+v v%d", asset, version)
	}
	onDisk, err := os.ReadFile(filepath.Join(env.cfg.WorkspaceDir, "CYD", "assets", "assets", "sun.png"))
	if err != nil || !bytes.Equal(onDisk, data) {
		t.Fatalf("stored file differs: %v", err)
	}
	layout, v, _, _ := env.svc.GetLayout(ctx, p.ID)
	if v != 2 || len(layout.Assets) != 1 || layout.Assets[0] != asset {
		t.Fatalf("layout assets = %+v v%d", layout.Assets, v)
	}
	ev := collectEvents(t, events, 1, 2*time.Second, EventProjectChanged)[0]
	if string(ev.Data) != `{"origin":"user","version":2,"what":"assets"}` {
		t.Fatalf("project_changed = %s", ev.Data)
	}
	second, _, err := env.svc.PutAsset(ctx, p.ID, "sun.png", pngBytes(t, 8, 8, true))
	if err != nil || second.File != "assets/sun_2.png" || !second.Alpha || second.ID == asset.ID {
		t.Fatalf("second asset = %+v, %v", second, err)
	}
	r, got, contentType, err := env.svc.OpenAsset(ctx, p.ID, asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	served, _ := io.ReadAll(r)
	_ = r.Close()
	if got != asset || contentType != "image/png" || !bytes.Equal(served, data) {
		t.Fatalf("OpenAsset = %+v %q (%d bytes)", got, contentType, len(served))
	}
}

func TestPutAssetEnforcesLimits(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Budget")
	big := append(pngBytes(t, 4, 4, false), bytes.Repeat([]byte{0}, MaxAssetUploadBytes)...)
	_, _, err := env.svc.PutAsset(ctx, p.ID, "big.png", big)
	if typed := wantCode(t, err, CodeAssetTooLarge); typed.Details["reason"] != "file_size" {
		t.Fatalf("details = %v", typed.Details)
	}
	_, _, err = env.svc.PutAsset(ctx, p.ID, "wide.png", pngBytes(t, 321, 10, false))
	if typed := wantCode(t, err, CodeAssetTooLarge); typed.Details["reason"] != "dimensions" {
		t.Fatalf("details = %v", typed.Details)
	}
	if _, _, err := env.svc.PutAsset(ctx, p.ID, "portrait.png", pngBytes(t, 240, 320, false)); err != nil {
		t.Fatalf("a portrait image within the display must fit: %v", err)
	}
	_, _, err = env.svc.PutAsset(ctx, p.ID, "notes.txt", []byte("hello"))
	wantCode(t, err, CodeBadRequest)
	// portrait.png already uses 153600 of the 262144-byte budget (AssetBudget4MB, 4 MB board).
	// Each small image decodes to 20000 bytes: five fit (253600), the sixth does not (273600).
	small := pngBytes(t, 100, 100, false)
	for i := 0; i < 5; i++ {
		if _, _, err := env.svc.PutAsset(ctx, p.ID, "small.png", small); err != nil {
			t.Fatalf("image %d within the 256 KiB budget: %v", i, err)
		}
	}
	_, _, err = env.svc.PutAsset(ctx, p.ID, "small.png", small)
	if typed := wantCode(t, err, CodeAssetTooLarge); typed.Details["reason"] != "budget" {
		t.Fatalf("details = %v", typed.Details)
	}
}

func TestDeleteAsset(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Remove")
	asset, version, err := env.svc.PutAsset(ctx, p.ID, "logo.png", pngBytes(t, 10, 10, false))
	if err != nil {
		t.Fatal(err)
	}
	next, err := env.svc.DeleteAsset(ctx, p.ID, asset.ID)
	if err != nil || next != version+1 {
		t.Fatalf("DeleteAsset = v%d, %v", next, err)
	}
	if _, err := os.Stat(filepath.Join(env.cfg.WorkspaceDir, "CYD", "remove", "assets", "logo.png")); !os.IsNotExist(err) {
		t.Fatal("asset file was kept")
	}
	_, err = env.svc.DeleteAsset(ctx, p.ID, asset.ID)
	wantCode(t, err, CodeNotFound)
	_, _, _, err = env.svc.OpenAsset(ctx, p.ID, asset.ID)
	wantCode(t, err, CodeNotFound)
}

func TestGenerateReadsAssetsAndStagingTrees(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Gen")
	asset, _, err := env.svc.PutAsset(ctx, p.ID, "icon.png", pngBytes(t, 4, 4, false))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := env.svc.Generate(ctx, p.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tree["src/generated/ui_assets.cpp"]), asset.ID+":") {
		t.Fatalf("asset was not passed to the generator: %s", tree["src/generated/ui_assets.cpp"])
	}
	staging := filepath.Join(t.TempDir(), "staging")
	if err := os.CopyFS(staging, os.DirFS(filepath.Join(env.cfg.WorkspaceDir, "CYD", "gen"))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "src", "app", "app_main.cpp"), []byte("// staged\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	staged, err := env.svc.Generate(ctx, p.ID, staging)
	if err != nil || string(staged["src/app/app_main.cpp"]) != "// staged\n" {
		t.Fatalf("staging generate = %q, %v", staged["src/app/app_main.cpp"], err)
	}
}

// TestPutAssetChecksTheHeaderBeforeDecoding: an image larger than every
// supported display is refused from its header alone. The crafted PNG has no
// pixel data, so a full decode would have reported it as damaged.
func TestPutAssetChecksTheHeaderBeforeDecoding(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Header")
	// 4000x3000 is a phone photo, above assetHeader's 2048-pixel limit.
	for _, size := range [][2]int{{2000, 1000}, {4000, 3000}} {
		_, _, err := env.svc.PutAsset(ctx, p.ID, "huge.png", headerOnlyPNG(size[0], size[1]))
		typed := wantCode(t, err, CodeAssetTooLarge)
		if typed.Details["reason"] != "dimensions" || strings.Contains(typed.Message, "damaged") {
			t.Fatalf("%v: error = %v (details %v), want the header refusal", size, typed, typed.Details)
		}
	}
	layout, version, _, err := env.svc.GetLayout(ctx, p.ID)
	if err != nil || version != 1 || len(layout.Assets) != 0 {
		t.Fatalf("layout after the refusal = %+v v%d, %v", layout.Assets, version, err)
	}
}

// rawAssetEntry lists an asset entry in the stored layout without writing a
// file and without PutLayout's checks, the way a hand-edited or imported
// layout can carry one.
func rawAssetEntry(t *testing.T, env *testEnv, projectID string, a Asset) {
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
	p.Assets = append(p.Assets, a)
	if _, err := env.svc.saveLayoutLocked(ctx, root, row, p); err != nil {
		t.Fatal(err)
	}
}

// TestAssetEntriesOutsideTheAssetRule: an entry whose file breaks the asset
// file rule is never opened, read or removed (its entry can still be deleted),
// and a file that another entry still lists is kept.
func TestAssetEntriesOutsideTheAssetRule(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Hostile")
	dir := filepath.Join(env.cfg.WorkspaceDir, "CYD", "hostile")
	appMain := filepath.Join(dir, "src", "app", "app_main.cpp")
	original, err := os.ReadFile(appMain)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o750); err != nil {
		t.Fatal(err)
	}
	rawAssetEntry(t, env, p.ID, Asset{ID: "a_src", File: "src/app/app_main.cpp", W: 4, H: 4})
	rawAssetEntry(t, env, p.ID, Asset{ID: "a_layout", File: "assets/../" + ProjectFile, W: 4, H: 4})

	for _, id := range []string{"a_src", "a_layout"} {
		_, _, _, err := env.svc.OpenAsset(ctx, p.ID, id)
		if typed := wantCode(t, err, CodeInvalidLayout); typed.Details["asset"] != id {
			t.Fatalf("OpenAsset(%s) details = %v", id, typed.Details)
		}
	}
	_, err = env.svc.Generate(ctx, p.ID, "")
	if typed := wantCode(t, err, CodeInvalidLayout); typed.Details["asset"] != "a_src" {
		t.Fatalf("Generate details = %v", typed.Details)
	}
	for _, id := range []string{"a_src", "a_layout"} {
		if _, err := env.svc.DeleteAsset(ctx, p.ID, id); err != nil {
			t.Fatalf("DeleteAsset(%s): %v", id, err)
		}
	}
	if after, err := os.ReadFile(appMain); err != nil || !bytes.Equal(after, original) {
		t.Fatalf("app_main.cpp was touched: %v", err)
	}
	layout, _, _, err := env.svc.GetLayout(ctx, p.ID)
	if err != nil || len(layout.Assets) != 0 {
		t.Fatalf("layout after deleting the entries = %+v, %v", layout.Assets, err)
	}

	if err := os.WriteFile(filepath.Join(dir, "assets", "shared.png"), pngBytes(t, 4, 4, false), 0o640); err != nil {
		t.Fatal(err)
	}
	rawAssetEntry(t, env, p.ID, Asset{ID: "a_one", File: "assets/shared.png", W: 4, H: 4})
	rawAssetEntry(t, env, p.ID, Asset{ID: "a_two", File: "assets/shared.png", W: 4, H: 4})
	if _, err := env.svc.DeleteAsset(ctx, p.ID, "a_one"); err != nil {
		t.Fatalf("DeleteAsset(a_one): %v", err)
	}
	r, _, _, err := env.svc.OpenAsset(ctx, p.ID, "a_two")
	if err != nil {
		t.Fatalf("the file the remaining entry lists was removed: %v", err)
	}
	_ = r.Close()
}

// TestDamagedAssetFiles: a directory at an asset path is never served or
// passed to the generator, and an image file above the upload limit is a
// typed refusal.
func TestDamagedAssetFiles(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Damaged")
	dir := filepath.Join(env.cfg.WorkspaceDir, "CYD", "damaged", "assets")
	if err := os.MkdirAll(filepath.Join(dir, "folder.png"), 0o750); err != nil {
		t.Fatal(err)
	}
	rawAssetEntry(t, env, p.ID, Asset{ID: "a_dir", File: "assets/folder.png", W: 4, H: 4})
	_, _, _, err := env.svc.OpenAsset(ctx, p.ID, "a_dir")
	wantCode(t, err, CodeNotFound)
	tree, err := env.svc.Generate(ctx, p.ID, "")
	if err != nil || strings.Contains(string(tree["src/generated/ui_assets.cpp"]), "a_dir") {
		t.Fatalf("Generate with a directory as image = %s, %v", tree["src/generated/ui_assets.cpp"], err)
	}

	big := append(pngBytes(t, 4, 4, false), bytes.Repeat([]byte{0}, MaxAssetUploadBytes)...)
	if err := os.WriteFile(filepath.Join(dir, "big.png"), big, 0o640); err != nil {
		t.Fatal(err)
	}
	rawAssetEntry(t, env, p.ID, Asset{ID: "a_big", File: "assets/big.png", W: 4, H: 4})
	_, err = env.svc.Generate(ctx, p.ID, "")
	if typed := wantCode(t, err, CodeAssetTooLarge); typed.Details["asset"] != "a_big" || typed.Details["reason"] != "file_size" {
		t.Fatalf("details = %v", typed.Details)
	}
}

// TestPutAssetKeepsTheImageOfAPublishedLayout: when the version bump fails
// after the layout was written, the stored layout lists the new image, so its
// file stays.
func TestPutAssetKeepsTheImageOfAPublishedLayout(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Bump")
	if _, err := env.svc.db.Exec(`CREATE TRIGGER cc_test_no_bump BEFORE UPDATE OF version ON cc_projects
		BEGIN SELECT RAISE(ABORT, 'no version bump'); END`); err != nil {
		t.Fatal(err)
	}
	_, _, err := env.svc.PutAsset(ctx, p.ID, "kept.png", pngBytes(t, 4, 4, false))
	wantCode(t, err, CodeInternal)
	if _, err := os.Stat(filepath.Join(env.cfg.WorkspaceDir, "CYD", "bump", "assets", "kept.png")); err != nil {
		t.Fatalf("the image the stored layout lists was removed: %v", err)
	}
}
