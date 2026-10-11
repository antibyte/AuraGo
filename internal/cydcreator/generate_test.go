package cydcreator

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// genOutputFiles are the files the generator writes itself (everything else
// in the tree is the template copy or app code).
var genOutputFiles = []string{
	"include/board_config.h",
	"partitions.csv",
	"platformio.ini",
	"src/generated/fonts.json",
	"src/generated/ui.cpp",
	"src/generated/ui.h",
	"src/generated/ui_assets.cpp",
	"src/generated/ui_hooks_weak.cpp",
	"src/generated/ui_theme.cpp",
}

// testProjectID is the project id the reference and golden trees carry.
const testProjectID = "ccp_000000000001"

// goldenPhotoJPEG is a 16x12 opaque JPEG (quality 80). It is stored as bytes
// because JPEG encoders differ between Go versions; decoding is stable.
const goldenPhotoJPEG = "/9j/2wCEAAYEBQYFBAYGBQYHBwYIChAKCgkJChQODwwQFxQYGBcUFhYaHSUfGhsjHBYWICwgIyYnKSopGR8tMC0oMCUoKSgBBwcH" +
	"CggKEwoKEygaFhooKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKP/AABEIAAwAEAMBIgAC" +
	"EQEDEQH/xAGiAAABBQEBAQEBAQAAAAAAAAAAAQIDBAUGBwgJCgsQAAIBAwMCBAMFBQQEAAABfQECAwAEEQUSITFBBhNRYQcicRQy" +
	"gZGhCCNCscEVUtHwJDNicoIJChYXGBkaJSYnKCkqNDU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6g4SFhoeI" +
	"iYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2drh4uPk5ebn6Onq8fLz9PX29/j5+gEAAwEBAQEB" +
	"AQEBAQAAAAAAAAECAwQFBgcICQoLEQACAQIEBAMEBwUEBAABAncAAQIDEQQFITEGEkFRB2FxEyIygQgUQpGhscEJIzNS8BVictEK" +
	"FiQ04SXxFxgZGiYnKCkqNTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqCg4SFhoeIiYqSk5SVlpeYmZqio6Sl" +
	"pqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2dri4+Tl5ufo6ery8/T19vf4+fr/2gAMAwEAAhEDEQA/AOH03wl0/d/pXU6b" +
	"4S6fu/0r0bTdNtuPkrqdN0224+SvouI8+qanm8JcS1fdP//Z"

// goldenLogoPNG is a 24x16 PNG with an alpha ramp (fully transparent to
// opaque); PNG is lossless, so the decoded pixels are exact.
func goldenLogoPNG(t testing.TB) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 24, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 24; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 10), G: uint8(y * 15), B: 0x80, A: uint8(x * 255 / 23)})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// goldenAssets returns the image data of the golden projects' assets.
func goldenAssets(t testing.TB) map[string][]byte {
	t.Helper()
	photo, err := base64.StdEncoding.DecodeString(goldenPhotoJPEG)
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{"a_logo": goldenLogoPNG(t), "a_photo": photo}
}

// TestGenerateReproducesReference is the plan 1b golden check: for every
// board, generating the reference project gives src/generated/* identical to
// testdata/reference/basic/src/generated/* and include/board_config.h,
// platformio.ini, partitions.csv identical to profiles/<board>/.
func TestGenerateReproducesReference(t *testing.T) {
	boards, err := Boards()
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range boards {
		t.Run(b.ID, func(t *testing.T) {
			p := loadReferenceProject(t)
			p.Board = b.ID
			out, err := Generate(GenerateInput{Project: p, ProjectID: testProjectID})
			if err != nil {
				t.Fatalf("Generate: %v\n%s", err, fmtIssues(out.Issues))
			}
			for _, rel := range genOutputFiles {
				want := filepath.Join(referenceDir, filepath.FromSlash(rel))
				if !strings.HasPrefix(rel, "src/generated/") {
					want = filepath.Join(referenceDir, "profiles", b.ID, filepath.FromSlash(rel))
				}
				data, err := os.ReadFile(want)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(out.Files[rel], data) {
					t.Errorf("%s differs from %s:\n%s", rel, want, firstDiff(string(data), string(out.Files[rel])))
				}
			}
		})
	}
}

func TestGenerateTreeContents(t *testing.T) {
	p := loadReferenceProject(t)
	app := map[string][]byte{"src/app/app_main.cpp": []byte("// app\n"), "src/app/sensors/dht.h": []byte("#pragma once\n")}
	out, err := Generate(GenerateInput{Project: p, ProjectID: testProjectID, AppFiles: app})
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	err = fs.WalkDir(TemplateFS(), "lib/cyd_template", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			want = append(want, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, genOutputFiles...)
	want = append(want, slices.Collect(maps.Keys(app))...)
	got := slices.Collect(maps.Keys(out.Files))
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("tree files\n got: %q\nwant: %q", got, want)
	}
	for rel, data := range app {
		if !bytes.Equal(out.Files[rel], data) {
			t.Errorf("%s was not copied verbatim", rel)
		}
	}
	lib, _ := fs.ReadFile(TemplateFS(), "lib/cyd_template/src/cyd.h")
	if !bytes.Equal(out.Files["lib/cyd_template/src/cyd.h"], lib) {
		t.Error("template file differs from TemplateFS()")
	}
	for _, never := range []string{"include/cyd_build_id.h", "strings.json", "cyd.project.json", "scaffold/src/app/app_main.cpp"} {
		if _, ok := out.Files[never]; ok {
			t.Errorf("generated tree must not contain %s", never)
		}
	}
	// The caller's app data must not alias the output.
	app["src/app/app_main.cpp"][0] = 'X'
	if out.Files["src/app/app_main.cpp"][0] == 'X' {
		t.Error("app files are aliased, not copied")
	}
}

func TestGenerateRefusals(t *testing.T) {
	base := loadReferenceProject(t)
	cases := []struct {
		name   string
		mutate func(in *GenerateInput)
		code   string
	}{
		{"validation error", func(in *GenerateInput) { in.Project.Screens[0].Widgets[1].Name = "Bad-Name" }, CodeInvalidLayout},
		{"template version", func(in *GenerateInput) { in.Project.TemplateVersion = "0.9.0" }, CodeInvalidLayout},
		{"asset data missing", func(in *GenerateInput) {
			in.Project.Assets = []Asset{{ID: "a_logo", File: "assets/logo.png", W: 24, H: 16, Alpha: true}}
		}, CodeBadRequest},
		{"asset size differs", func(in *GenerateInput) {
			in.Project.Assets = []Asset{{ID: "a_logo", File: "assets/logo.png", W: 20, H: 16, Alpha: true}}
			in.Assets = map[string][]byte{"a_logo": goldenLogoPNG(t)}
		}, CodeBadRequest},
		{"asset not an image", func(in *GenerateInput) {
			in.Project.Assets = []Asset{{ID: "a_logo", File: "assets/logo.png", W: 24, H: 16, Alpha: true}}
			in.Assets = map[string][]byte{"a_logo": []byte("not a png")}
		}, CodeBadRequest},
		{"app file outside src/app", func(in *GenerateInput) {
			in.AppFiles = map[string][]byte{"src/generated/ui.cpp": []byte("x")}
		}, CodeBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := MarshalProject(base)
			p, _ := ParseProject(raw)
			in := GenerateInput{Project: p, ProjectID: testProjectID}
			tc.mutate(&in)
			out, err := Generate(in)
			if ErrorCode(err) != tc.code {
				t.Fatalf("error %v (code %q), want %s", err, ErrorCode(err), tc.code)
			}
			if out.Files != nil {
				t.Error("a refused generation must not return files")
			}
		})
	}
	// Validation errors come back as issues and in the error details.
	raw, _ := MarshalProject(base)
	p, _ := ParseProject(raw)
	p.Screens[0].Widgets[1].Name = "Bad-Name"
	out, err := Generate(GenerateInput{Project: p})
	var e *Error
	if !errors.As(err, &e) || !HasErrors(out.Issues) || e.Details["issues"] == nil {
		t.Fatalf("want CYD_INVALID_LAYOUT with issues, got %v / %+v", err, out.Issues)
	}
}

func TestGenerateKeepsWarnings(t *testing.T) {
	p := loadReferenceProject(t)
	p.Screens[0].Widgets[1].X = 300 // partly outside the 320 px screen
	out, err := Generate(GenerateInput{Project: p})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Issues) != 1 || out.Issues[0].Code != IssuePartlyOutOfBounds {
		t.Fatalf("issues %+v, want one PARTLY_OUT_OF_BOUNDS warning", out.Issues)
	}
}

func TestScaffoldAppFiles(t *testing.T) {
	files := ScaffoldAppFiles(loadReferenceProject(t))
	want, err := fs.ReadFile(TemplateFS(), "scaffold/src/app/app_main.cpp")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || !bytes.Equal(files["src/app/app_main.cpp"], want) {
		t.Fatalf("ScaffoldAppFiles = %v", slices.Collect(maps.Keys(files)))
	}
	for rel := range files {
		if !IsEditablePath(rel) {
			t.Errorf("%s is not an editable app path", rel)
		}
	}
}
