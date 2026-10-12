package cydcreator

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// zipEntry is one file of a hand-made archive; mode 0 means a regular file.
type zipEntry struct {
	name string
	data []byte
	mode fs.FileMode
}

func zipOf(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		fh := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			fh.SetMode(e.mode)
		}
		w, err := zw.CreateHeader(fh)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// lyingZip has one entry whose header claims 10 bytes while its deflate
// stream unpacks to 1 MiB (a zip bomb).
func lyingZip(t *testing.T, layout []byte) []byte {
	t.Helper()
	var payload bytes.Buffer
	fw, _ := flate.NewWriter(&payload, flate.BestCompression)
	big := bytes.Repeat([]byte("A"), 1<<20)
	_, _ = fw.Write(big)
	_ = fw.Close()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "cyd.project.json", Method: zip.Deflate})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write(layout)
	raw, err := zw.CreateRaw(&zip.FileHeader{Name: "src/app/bomb.cpp", Method: zip.Deflate, CRC32: crc32.ChecksumIEEE(big[:10]),
		CompressedSize64: uint64(payload.Len()), UncompressedSize64: 10})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = raw.Write(payload.Bytes())
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// importLayout returns the canonical layout of a fresh project.
func importLayout(t *testing.T, name string, mutate func(*Project)) []byte {
	t.Helper()
	p, err := NewProject(name, "esp32-2432s028r", "night", "de", "CET-1CEST,M3.5.0,M10.5.0/3", &SeqIDs{})
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(&p)
	}
	data, err := MarshalProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func projectDirs(t *testing.T, env *testEnv) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(env.cfg.WorkspaceDir, "CYD"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func TestImportProjectRoundTripsAnExport(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	src, asset, _ := exportFixture(t, env, "Wetter Station")
	if _, err := env.svc.WriteFile(ctx, src.ID, "src/app/net/client.cpp", []byte("// Client\n"), "*", OriginUser); err != nil {
		t.Fatal(err)
	}
	data, _, err := env.svc.ExportProject(ctx, src.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := env.svc.ImportProject(ctx, data, `C:\Downloads\wetter-station.zip`)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == src.ID || got.Slug != "wetter-station-2" || got.Name != "Wetter Station" || got.Version != 1 || got.Board != src.Board {
		t.Fatalf("imported = %+v", got)
	}
	srcLayout, _, _, _ := env.svc.GetLayout(ctx, src.ID)
	gotLayout, _, _, err := env.svc.GetLayout(ctx, got.ID)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := MarshalProject(srcLayout)
	b, _ := MarshalProject(gotLayout)
	if !bytes.Equal(a, b) {
		t.Fatalf("layout changed:\n%s\n%s", a, b)
	}
	for _, rel := range []string{"src/app/app_main.cpp", "src/app/sensor.cpp"} {
		want, _, _ := env.svc.ReadFile(ctx, src.ID, rel)
		have, _, err := env.svc.ReadFile(ctx, got.ID, rel)
		if err != nil || !bytes.Equal(want, have) {
			t.Fatalf("%s = %q (%v), want %q", rel, have, err, want)
		}
	}
	f, _, _, err := env.svc.OpenAsset(ctx, got.ID, asset.ID)
	if err != nil {
		t.Fatalf("asset: %v", err)
	}
	_ = f.Close()
	dir := filepath.Join(env.cfg.WorkspaceDir, "CYD", got.Slug)
	for _, generated := range []string{"platformio.ini", "include", "src/generated", "lib", "README.md"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(generated))); err == nil {
			t.Errorf("%s must not be imported", generated)
		}
	}
	var reason, label string
	if err := env.svc.db.QueryRow(`SELECT reason, label FROM cc_revisions WHERE project_id=?`, got.ID).Scan(&reason, &label); err != nil {
		t.Fatalf("import revision: %v", err)
	}
	if reason != "import" || label != "Import wetter-station.zip" {
		t.Fatalf("revision = %q %q", reason, label)
	}
	// Every app file (also below a folder) and every image original come back
	// byte for byte.
	assertSameProjectContent(t, env, src.ID, got.ID)
}

// assertSameProjectContent compares the editable files and the asset
// originals of two projects.
func assertSameProjectContent(t *testing.T, env *testEnv, srcID, gotID string) {
	t.Helper()
	ctx := context.Background()
	appFiles := func(id string) map[string]string {
		files, err := env.svc.ListFiles(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, f := range files {
			if f.Editable {
				out[f.Path] = f.SHA256
			}
		}
		return out
	}
	want, have := appFiles(srcID), appFiles(gotID)
	if len(want) == 0 || len(want) != len(have) {
		t.Fatalf("app files = %v, want %v", have, want)
	}
	for rel, sha := range want {
		if have[rel] != sha {
			t.Fatalf("%s: sha %q, want %q", rel, have[rel], sha)
		}
	}
	layout, _, _, err := env.svc.GetLayout(ctx, srcID)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range layout.Assets {
		read := func(id string) []byte {
			f, _, _, err := env.svc.OpenAsset(ctx, id, a.ID)
			if err != nil {
				t.Fatalf("asset %s: %v", a.ID, err)
			}
			defer f.Close()
			data, err := io.ReadAll(f)
			if err != nil {
				t.Fatal(err)
			}
			return data
		}
		if !bytes.Equal(read(srcID), read(gotID)) {
			t.Fatalf("asset %s differs", a.ID)
		}
	}
}

func TestImportProjectAcceptsFlatArchivesAndScaffoldsMissingSources(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	data := zipOf(t,
		zipEntry{name: "cyd.project.json", data: importLayout(t, "Flach", nil)},
		zipEntry{name: "README.md", data: []byte("# old")},
		zipEntry{name: ".vscode/settings.json", data: []byte("{}")},
		zipEntry{name: "__MACOSX/._cyd.project.json", data: []byte("meta")},
		zipEntry{name: "platformio.ini", data: []byte("[env]")},
		zipEntry{name: "src/generated/ui.cpp", data: []byte("// old")},
	)
	got, err := env.svc.ImportProject(ctx, data, "flach.zip")
	if err != nil {
		t.Fatal(err)
	}
	files, err := env.svc.ListFiles(ctx, got.ID)
	if err != nil {
		t.Fatal(err)
	}
	var app []string
	for _, f := range files {
		if f.Editable {
			app = append(app, f.Path)
		}
	}
	if strings.Join(app, ",") != "src/app/app_main.cpp" {
		t.Fatalf("app files = %v (the scaffold replaces missing sources)", app)
	}
}

// TestImportProjectCountsOnlyKeptEntries: generated files, fonts and images
// the layout does not list are dropped without being read, so neither their
// size nor a lying header counts against MaxImportBytes (an export with large
// converted fonts must import again).
func TestImportProjectCountsOnlyKeptEntries(t *testing.T) {
	env, _ := newDeviceEnv(t)
	huge := make([]byte, MaxImportBytes)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range []zipEntry{
		{name: "p/cyd.project.json", data: importLayout(t, "Gross", nil)},
		{name: "p/src/generated/fonts/ui_font_big.c", data: huge},
		{name: "p/assets/unused.png", data: huge},
		{name: "p/README.md", data: huge},
	} {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: e.name, Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(e.data)
	}
	// A dropped entry whose header lies about its size is never opened.
	raw, err := zw.CreateRaw(&zip.FileHeader{Name: "p/lib/cyd_template/src/x.cpp", Method: zip.Store, UncompressedSize64: 1 << 62, CompressedSize64: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = raw.Write([]byte("x"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := env.svc.ImportProject(context.Background(), buf.Bytes(), "gross.zip")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(env.cfg.WorkspaceDir, "CYD", got.Slug)
	for _, dropped := range []string{"src/generated", "assets", "README.md", "lib"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(dropped))); err == nil {
			t.Errorf("%s must not be imported", dropped)
		}
	}
}

func TestImportProjectRejectsUnsafeAndForeignArchives(t *testing.T) {
	env, _ := newDeviceEnv(t)
	ctx := context.Background()
	layout := importLayout(t, "Test", nil)
	withLayout := func(extra ...zipEntry) []byte {
		return zipOf(t, append([]zipEntry{{name: "cyd.project.json", data: layout}}, extra...)...)
	}
	tooMany := []zipEntry{{name: "cyd.project.json", data: layout}}
	for i := 0; i < MaxImportEntries; i++ {
		tooMany = append(tooMany, zipEntry{name: "assets/a" + strings.Repeat("x", i%5) + string(rune('a'+i%26)) + ".png", data: []byte{1}})
	}
	pngWithAsset := importLayout(t, "Bild", func(p *Project) {
		p.Assets = []Asset{{ID: "a_sun", File: "assets/sun.jpg", W: 4, H: 4}}
	})
	// Nine listed 4x4 PNGs padded to 1.9 MiB each: every file is within the
	// upload limit and the declared total within MaxImportBytes, but the
	// originals exceed MaxAssetTotalBytes.
	padded := append(pngBytes(t, 4, 4, false), make([]byte, 1900<<10)...)
	var heavy []Asset
	heavyEntries := []zipEntry{}
	for i := 0; i < 9; i++ {
		file := "assets/p" + string(rune('a'+i)) + ".png"
		heavy = append(heavy, Asset{ID: "a_p" + string(rune('a'+i)), File: file, W: 4, H: 4})
		heavyEntries = append(heavyEntries, zipEntry{name: file, data: padded})
	}
	heavyLayout := importLayout(t, "Schwer", func(p *Project) { p.Assets = heavy })
	cases := []struct {
		name   string
		data   []byte
		code   string
		reason string
	}{
		{"empty", nil, CodeImportInvalid, "too_large"},
		{"not a zip", []byte("PK? no"), CodeImportInvalid, "not_zip"},
		{"zip slip", withLayout(zipEntry{name: "../evil.cpp", data: []byte("x")}), CodeImportInvalid, "unsafe_path"},
		{"nested zip slip", withLayout(zipEntry{name: "src/app/../../../evil.cpp", data: []byte("x")}), CodeImportInvalid, "unsafe_path"},
		{"absolute", withLayout(zipEntry{name: "/etc/cron.d/x", data: []byte("x")}), CodeImportInvalid, "unsafe_path"},
		{"drive letter", withLayout(zipEntry{name: "C:/x.cpp", data: []byte("x")}), CodeImportInvalid, "unsafe_path"},
		{"backslash", withLayout(zipEntry{name: `src\app\x.cpp`, data: []byte("x")}), CodeImportInvalid, "unsafe_path"},
		{"symlink", withLayout(zipEntry{name: "src/app/link.cpp", data: []byte("/etc/passwd"), mode: fs.ModeSymlink | 0o777}), CodeImportInvalid, "unsafe_path"},
		{"named pipe", withLayout(zipEntry{name: "src/app/fifo.cpp", mode: fs.ModeNamedPipe | 0o644}), CodeImportInvalid, "unsafe_path"},
		{"device name", withLayout(zipEntry{name: "src/app/aux.cpp", data: []byte("x")}), CodeImportInvalid, "unsafe_path"},
		{"trailing dot", withLayout(zipEntry{name: "src/app/x.cpp.", data: []byte("x")}), CodeImportInvalid, "unsafe_path"},
		{"trailing space", withLayout(zipEntry{name: "src/app/x.cpp ", data: []byte("x")}), CodeImportInvalid, "unsafe_path"},
		{"duplicate entry", withLayout(zipEntry{name: "src/app/a.cpp", data: []byte("x")}, zipEntry{name: "src/app/a.cpp", data: []byte("y")}), CodeImportInvalid, "unsafe_path"},
		{"duplicate layout", withLayout(zipEntry{name: "cyd.project.json", data: layout}), CodeImportInvalid, "unsafe_path"},
		{"too many entries", zipOf(t, tooMany...), CodeImportInvalid, "too_many_entries"},
		{"declared too large", withLayout(zipEntry{name: "src/app/big.cpp", data: make([]byte, MaxImportBytes)}), CodeImportInvalid, "too_large"},
		{"zip bomb", lyingZip(t, layout), CodeImportInvalid, "app_files"},
		{"no layout", zipOf(t, zipEntry{name: "src/app/app_main.cpp", data: []byte("x")}), CodeImportInvalid, "layout_missing"},
		{"layout too deep", zipOf(t, zipEntry{name: "a/b/cyd.project.json", data: layout}), CodeImportInvalid, "layout_missing"},
		{"foreign file type", withLayout(zipEntry{name: "src/main.cpp", data: []byte("x")}), CodeImportInvalid, "file_type"},
		{"binary in src/app", withLayout(zipEntry{name: "src/app/data.bin", data: []byte("x")}), CodeImportInvalid, "file_type"},
		{"text file", withLayout(zipEntry{name: "notes.txt", data: []byte("x")}), CodeImportInvalid, "file_type"},
		{"outside the folder", zipOf(t, zipEntry{name: "p/cyd.project.json", data: layout}, zipEntry{name: "q/x.cpp", data: []byte("x")}), CodeImportInvalid, "file_type"},
		{"broken layout", zipOf(t, zipEntry{name: "cyd.project.json", data: []byte("{")}), CodeImportInvalid, "layout"},
		{"unknown format", zipOf(t, zipEntry{name: "cyd.project.json", data: bytes.Replace(layout, []byte(`"format": 1`), []byte(`"format": 2`), 1)}), CodeImportInvalid, "layout"},
		{"unknown board", zipOf(t, zipEntry{name: "cyd.project.json", data: importLayout(t, "B", func(p *Project) { p.Board = "esp32-unknown" })}), CodeImportInvalid, "board"},
		{"newer template", zipOf(t, zipEntry{name: "cyd.project.json", data: importLayout(t, "N", func(p *Project) { p.TemplateVersion = "9.0.0" })}), CodeTemplateTooNew, ""},
		{"source not UTF-8", withLayout(zipEntry{name: "src/app/x.cpp", data: []byte{0xff, 0xfe, 0x00}}), CodeImportInvalid, "app_files"},
		{"case collision", withLayout(zipEntry{name: "src/app/a.cpp", data: []byte("x")}, zipEntry{name: "src/app/A.cpp", data: []byte("y")}), CodeImportInvalid, "app_files"},
		{"folder case collision", withLayout(zipEntry{name: "src/app/Lib/a.cpp", data: []byte("x")}, zipEntry{name: "src/app/lib/b.cpp", data: []byte("y")}), CodeImportInvalid, "app_files"},
		{"file and folder", withLayout(zipEntry{name: "src/app/x.h", data: []byte("x")}, zipEntry{name: "src/app/x.h/y.cpp", data: []byte("y")}), CodeImportInvalid, "app_files"},
		{"png as jpg", zipOf(t, zipEntry{name: "cyd.project.json", data: pngWithAsset}, zipEntry{name: "assets/sun.jpg", data: pngBytes(t, 4, 4, false)}), CodeImportInvalid, "asset"},
		{"asset size differs", zipOf(t, zipEntry{name: "cyd.project.json", data: importLayout(t, "S", func(p *Project) {
			p.Assets = []Asset{{ID: "a_sun", File: "assets/sun.png", W: 4, H: 4}}
		})}, zipEntry{name: "assets/sun.png", data: pngBytes(t, 8, 8, false)}), CodeImportInvalid, "asset"},
		{"asset larger than every display", zipOf(t, zipEntry{name: "cyd.project.json", data: importLayout(t, "L", func(p *Project) {
			p.Assets = []Asset{{ID: "a_wide", File: "assets/wide.png", W: 900, H: 10}}
		})}, zipEntry{name: "assets/wide.png", data: pngBytes(t, 900, 10, false)}), CodeImportInvalid, "asset"},
		{"asset entry outside assets", zipOf(t, zipEntry{name: "cyd.project.json", data: importLayout(t, "O", func(p *Project) {
			p.Assets = []Asset{{ID: "a_src", File: "src/app/app_main.cpp", W: 4, H: 4}}
		})}), CodeImportInvalid, "asset"},
		{"device-name asset", zipOf(t, zipEntry{name: "cyd.project.json", data: importLayout(t, "D", func(p *Project) {
			p.Assets = []Asset{{ID: "a_aux", File: "assets/aux.png", W: 4, H: 4}}
		})}), CodeImportInvalid, "asset"},
		{"images above the total cap", zipOf(t, append([]zipEntry{{name: "cyd.project.json", data: heavyLayout}}, heavyEntries...)...), CodeImportInvalid, "asset"},
	}
	for _, tc := range cases {
		_, err := env.svc.ImportProject(ctx, tc.data, "x.zip")
		typed := wantCodeQuiet(err, tc.code)
		if typed == nil {
			t.Errorf("%s: error = %v, want %s", tc.name, err, tc.code)
			continue
		}
		if tc.reason != "" && typed.Details["reason"] != tc.reason {
			t.Errorf("%s: reason = %v, want %s (%s)", tc.name, typed.Details["reason"], tc.reason, typed.Message)
		}
		if tc.name == "images above the total cap" && typed.Details["limit"] != "asset_bytes" {
			t.Errorf("%s: details = %v, want limit asset_bytes", tc.name, typed.Details)
		}
	}
	if dirs := projectDirs(t, env); len(dirs) != 0 {
		t.Fatalf("refused imports left directories: %v", dirs)
	}
	if list, _ := env.svc.ListProjects(ctx); len(list) != 0 {
		t.Fatalf("refused imports created projects: %+v", list)
	}
}

// wantCodeQuiet returns the typed error when it has code, else nil.
func wantCodeQuiet(err error, code string) *Error {
	if typed, ok := err.(*Error); ok && typed.Code == code {
		return typed
	}
	return nil
}

func TestCompareTemplateVersions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{{"1.0.0", "1.0.0", 0}, {"1.0.1", "1.0.0", 1}, {"1.2.0", "1.10.0", -1}, {"2.0.0", "1.9.9", 1}} {
		if got, err := compareTemplateVersions(tc.a, tc.b); err != nil || got != tc.want {
			t.Errorf("%s vs %s = %d, %v", tc.a, tc.b, got, err)
		}
	}
	for _, bad := range []string{"", "1.0", "1.0.0.0", "v1.0.0", "1.-1.0", "01.0.0"} {
		if _, err := compareTemplateVersions(bad, "1.0.0"); err == nil {
			t.Errorf("%q should be invalid", bad)
		}
	}
}
