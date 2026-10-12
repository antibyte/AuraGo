package cydcreator

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// unzipForTest returns every file of a ZIP by name.
func unzipForTest(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name] = content
	}
	return out
}

func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// exportFixture creates a project with an extra source file, an image and a
// minted device, and returns it with the device token.
func exportFixture(t *testing.T, env *testEnv, name string) (ProjectSummary, Asset, string) {
	t.Helper()
	ctx := context.Background()
	p := env.createProject(t, name)
	if _, err := env.svc.WriteFile(ctx, p.ID, "src/app/sensor.cpp", []byte("// Sensor\nint sensor_value() { return 21; }\n"), "*", OriginUser); err != nil {
		t.Fatal(err)
	}
	asset, _, err := env.svc.PutAsset(ctx, p.ID, "Sonne.png", pngBytes(t, 24, 24, true))
	if err != nil {
		t.Fatal(err)
	}
	build := succeededBuild(t, env, p.ID)
	_, token := mintTestDevice(t, env, p.ID, build.ID)
	return p, asset, token
}

func TestExportProjectContainsTheGeneratedTreeSourcesAndAssets(t *testing.T) {
	env, _ := newDeviceEnv(t)
	p, asset, token := exportFixture(t, env, "Wetter Station")
	data, name, err := env.svc.ExportProject(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if name != "wetter-station.zip" {
		t.Fatalf("file name = %q", name)
	}
	files := unzipForTest(t, data)
	prefix := "wetter-station/"
	want := []string{
		prefix + "README.md", prefix + asset.File, prefix + "cyd.project.json", prefix + "include/board_config.h",
		prefix + "include/cyd_build_id.h",
		prefix + "lib/cyd_template/src/cyd.h", prefix + "partitions.csv", prefix + "platformio.ini",
		prefix + "src/app/app_main.cpp", prefix + "src/app/sensor.cpp", prefix + "src/generated/ui.cpp",
		prefix + "src/generated/ui_assets.cpp",
	}
	sort.Strings(want)
	if got := sortedKeys(files); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("entries:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	layout, err := ParseProject(files[prefix+"cyd.project.json"])
	if err != nil || layout.Name != "Wetter Station" || len(layout.Assets) != 1 {
		t.Fatalf("exported layout = %+v, %v", layout, err)
	}
	if !bytes.Equal(files[prefix+asset.File], pngBytes(t, 24, 24, true)) {
		t.Fatal("the image is not the uploaded original")
	}
	if string(files[prefix+"platformio.ini"]) != "[env:esp32-2432s028r]\n" {
		t.Fatalf("platformio.ini = %q (the export copies the generated tree unchanged)", files[prefix+"platformio.ini"])
	}
	if string(files[prefix+"include/cyd_build_id.h"]) != exportBuildIDContent ||
		!strings.Contains(exportBuildIDContent, `#define CYD_BUILD_ID "export"`) {
		t.Fatalf("cyd_build_id.h = %q", files[prefix+"include/cyd_build_id.h"])
	}
	readme := string(files[prefix+"README.md"])
	for _, s := range []string{"# Wetter Station", "pio run -e esp32-2432s028r", "Wi-Fi credentials and AuraGo device tokens are never exported"} {
		if !strings.Contains(readme, s) {
			t.Errorf("README misses %q", s)
		}
	}
	for name, content := range files {
		if bytes.Contains(content, []byte(token)) || bytes.Contains(content, []byte(deviceTokenPrefix)) {
			t.Fatalf("%s contains a device token", name)
		}
	}
	if _, _, err := env.svc.ExportProject(context.Background(), "ccp_missing"); ErrorCode(err) != CodeNotFound {
		t.Fatalf("unknown project: %v", err)
	}
}

// withFontsJSON wraps fakeGenerate and adds a src/generated/fonts.json.
func withFontsJSON(content string) func(GenerateInput) (GenerateOutput, error) {
	return func(in GenerateInput) (GenerateOutput, error) {
		out, err := fakeGenerate(in)
		if err == nil {
			out.Files["src/generated/fonts.json"] = []byte(content)
		}
		return out, err
	}
}

func TestExportProjectAddsTheFontsOfTheNewestMatchingBuild(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	// Set before any project or build exists; the build worker never calls the generator.
	env.svc.generate = withFontsJSON(`{"fonts":["a"]}`)
	env.builder.build = func(_ context.Context, req BuildRequest, _ func(string)) (BuildOutput, error) {
		out := successOutput(req)
		out.Result.Fonts = map[string]BuildResultArtifact{}
		for name, data := range map[string][]byte{
			"fonts.json":              req.Tree["src/generated/fonts.json"],
			"ui_font_montserrat_14.c": []byte("// font of " + req.BuildID + "\n"),
		} {
			out.Files[ArtifactFontsDir+"/"+name] = data
			out.Result.Fonts[name] = BuildResultArtifact{Size: int64(len(data)), SHA256: sha256Hex(data)}
		}
		return out, nil
	}
	p := env.createProject(t, "Fonts")
	const fontFile = "fonts/src/generated/fonts/ui_font_montserrat_14.c"
	export := func() (map[string][]byte, string) {
		t.Helper()
		data, _, err := env.svc.ExportProject(ctx, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		files := unzipForTest(t, data)
		return files, string(files["fonts/README.md"])
	}
	files, readme := export()
	if _, ok := files[fontFile]; ok || !strings.Contains(readme, "Build the project once in CYD Creator") {
		t.Fatalf("export without a build: font present %v, README:\n%s", ok, readme)
	}
	first := succeededBuild(t, env, p.ID)
	files, readme = export()
	if string(files[fontFile]) != "// font of "+first.ID+"\n" || !strings.Contains(readme, "holds the LVGL fonts of build "+first.ID) {
		t.Fatalf("current fonts: %q\n%s", files[fontFile], readme)
	}
	env.svc.generate = withFontsJSON(`{"fonts":["a","b"]}`)
	files, readme = export()
	if string(files[fontFile]) != "// font of "+first.ID+"\n" || !strings.Contains(readme, "fonts changed after the newest build") {
		t.Fatalf("stale fonts: %q\n%s", files[fontFile], readme)
	}
	second := succeededBuild(t, env, p.ID)
	files, readme = export()
	if string(files[fontFile]) != "// font of "+second.ID+"\n" || strings.Contains(readme, "Warning") {
		t.Fatalf("newest matching build: %q\n%s", files[fontFile], readme)
	}
}

func TestExportProjectWithLayoutErrorsKeepsTheSources(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Kaputt")
	layout, version, _, err := env.svc.GetLayout(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	layout.StartScreen = "s_missing"
	if _, _, err := env.svc.PutLayout(ctx, p.ID, layout, version, OriginUser); err != nil {
		t.Fatal(err)
	}
	data, _, err := env.svc.ExportProject(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	files := unzipForTest(t, data)
	if _, ok := files["kaputt/platformio.ini"]; ok {
		t.Fatal("an invalid layout must not produce generated files")
	}
	if _, ok := files["kaputt/src/app/app_main.cpp"]; !ok {
		t.Fatalf("sources missing: %v", sortedKeys(files))
	}
	if !strings.Contains(string(files["kaputt/README.md"]), "the generated files") {
		t.Fatal("the README must explain the missing generated files")
	}
}

// An unknown board is a layout error too: the README names the project's last
// valid board instead of failing the backup.
func TestExportProjectWithAnUnknownBoardKeepsTheSources(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Fremd")
	layout, version, _, err := env.svc.GetLayout(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	layout.Board = "no-such-board"
	if _, _, err := env.svc.PutLayout(ctx, p.ID, layout, version, OriginUser); err != nil {
		t.Fatal(err)
	}
	data, _, err := env.svc.ExportProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	files := unzipForTest(t, data)
	if _, ok := files["fremd/platformio.ini"]; ok {
		t.Fatal("an unknown board must not produce generated files")
	}
	exported, err := ParseProject(files["fremd/cyd.project.json"])
	if err != nil || exported.Board != "no-such-board" {
		t.Fatalf("exported layout board = %q, %v", exported.Board, err)
	}
	if readme := string(files["fremd/README.md"]); !strings.Contains(readme, "pio run -e esp32-2432s028r") {
		t.Fatalf("README:\n%s", readme)
	}
}

// The real generator refuses image data it cannot use (a damaged or missing
// file) with CYD_BAD_REQUEST before it validates the layout. The export then
// keeps the sources like for an invalid layout, so it still works as a backup:
// a damaged image is exported as stored, a missing one is left out.
func TestExportProjectWithAnUnusableImageKeepsTheSources(t *testing.T) {
	garbage := []byte("not an image")
	for _, tc := range []struct {
		name   string
		damage func(t *testing.T, file string)
		want   []byte // the exported image; nil = no entry
	}{
		{"damaged", func(t *testing.T, file string) {
			if err := os.WriteFile(file, garbage, 0o640); err != nil {
				t.Fatal(err)
			}
		}, garbage},
		{"missing", func(t *testing.T, file string) {
			if err := os.Remove(file); err != nil {
				t.Fatal(err)
			}
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, _ := newDeviceEnv(t)
			env.svc.generate = Generate
			ctx := context.Background()
			p := env.createProject(t, "Bilder")
			asset, _, err := env.svc.PutAsset(ctx, p.ID, "Sonne.png", pngBytes(t, 24, 24, true))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := env.svc.Generate(ctx, p.ID, ""); err != nil {
				t.Fatalf("the intact project must generate: %v", err)
			}
			tc.damage(t, filepath.Join(env.cfg.WorkspaceDir, "CYD", p.Slug, filepath.FromSlash(asset.File)))
			if _, err := env.svc.Generate(ctx, p.ID, ""); ErrorCode(err) != CodeBadRequest {
				t.Fatalf("Generate after the damage = %v, want %s", err, CodeBadRequest)
			}
			data, _, err := env.svc.ExportProject(ctx, p.ID)
			if err != nil {
				t.Fatalf("export: %v", err)
			}
			files := unzipForTest(t, data)
			if _, ok := files["bilder/platformio.ini"]; ok {
				t.Fatal("an unusable image must not produce generated files")
			}
			for _, rel := range []string{"cyd.project.json", "src/app/app_main.cpp"} {
				if _, ok := files["bilder/"+rel]; !ok {
					t.Fatalf("%s missing: %v", rel, sortedKeys(files))
				}
			}
			got, ok := files["bilder/"+asset.File]
			if ok != (tc.want != nil) || !bytes.Equal(got, tc.want) {
				t.Fatalf("image entry = %q (present %v), want %q", got, ok, tc.want)
			}
			if !strings.Contains(string(files["bilder/README.md"]), "the generated files") {
				t.Fatal("the README must explain the missing generated files")
			}
		})
	}
}

func TestExportProjectRefusesFencedProjectsAndOversizedArchives(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	p := env.createProject(t, "Grenzen")

	env.svc.setPublicationFence(p.ID, "publish_ccj_0001")
	_, _, err := env.svc.ExportProject(ctx, p.ID)
	if e := wantCode(t, err, CodeBusy); e.Details["reason"] != "publication_review" {
		t.Fatalf("fenced export details = %v", e.Details)
	}
	env.svc.setPublicationFence(p.ID, "")

	if _, _, err := env.svc.ExportProject(ctx, p.ID); err != nil {
		t.Fatalf("export below the limit: %v", err)
	}
	old := maxExportBytes
	maxExportBytes = 64
	t.Cleanup(func() { maxExportBytes = old })
	_, _, err = env.svc.ExportProject(ctx, p.ID)
	if e := wantCode(t, err, CodeFileLimit); e.Details["limit"] != "export_bytes" {
		t.Fatalf("oversized export details = %v", e.Details)
	}
}

func TestSafeArchivePath(t *testing.T) {
	for _, ok := range []string{"cyd.project.json", "src/app/a.cpp", "assets/sun_2.png", "lib/cyd_template/src/cyd.h"} {
		if !safeArchivePath(ok) {
			t.Errorf("%q should be safe", ok)
		}
	}
	for _, bad := range []string{"", "/etc/passwd", "../x", "a/../b", "a//b", ".pio/x", "a\\b", "c:x", "a/\x01", "./a"} {
		if safeArchivePath(bad) {
			t.Errorf("%q should be unsafe", bad)
		}
	}
}
