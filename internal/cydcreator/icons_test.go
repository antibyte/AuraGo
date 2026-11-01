package cydcreator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

func testIconLibrary(t *testing.T) (*IconLibrary, fstest.MapFS) {
	t.Helper()
	files := fstest.MapFS{}
	manifest := iconManifest{Version: IconLibraryVersion, Files: map[string]iconFile{}}
	c := Icons()
	for _, size := range c.Sizes {
		for frame := 0; frame < 6; frame++ {
			img := image.NewNRGBA(image.Rect(0, 0, size, size))
			img.SetNRGBA(size/2, size/2, color.NRGBA{R: 255, A: uint8(250 - frame)})
			raw := encodePNG(t, img)
			digest := sha256.Sum256(raw)
			for _, style := range c.Styles {
				for _, tone := range c.Tones {
					for _, motif := range c.Motifs {
						if frame > 0 && motif.Motion != "frames" {
							continue
						}
						key := (IconRequest{Style: style, Tone: tone, Motif: motif.ID, Size: size}).key(frame)
						files["img/cyd-icons/"+key] = &fstest.MapFile{Data: raw}
						manifest.Files[key] = iconFile{SHA256: hex.EncodeToString(digest[:]), Bytes: int64(len(raw))}
					}
				}
			}
		}
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	files["img/cyd-icons/manifest.json"] = &fstest.MapFile{Data: raw}
	library, err := LoadIconLibrary(files)
	if err != nil {
		t.Fatal(err)
	}
	return library, files
}

func TestIconCatalogAndShippedLibrary(t *testing.T) {
	c := Icons()
	if len(c.Styles)*len(c.Tones)*2*len(c.Motifs) != 1760 || len(c.Motifs) != 110 {
		t.Fatal("catalog does not contain 110 motifs and 1760 variants")
	}
	seen, animated := map[string]bool{}, 0
	for i, motif := range c.Motifs {
		if seen[motif.ID] {
			t.Fatalf("duplicate motif %s", motif.ID)
		}
		seen[motif.ID] = true
		if motif.Motion == "frames" {
			animated++
		} else if i >= 60 && motif.Motion != "pulse" {
			t.Fatalf("new single-frame motif %s lost its optional pulse", motif.ID)
		}
	}
	if animated != 32 {
		t.Fatalf("got %d frame animations, want 32", animated)
	}
	library, err := LoadIconLibrary(os.DirFS("../../ui"))
	if err != nil {
		t.Fatal(err)
	}
	if len(library.manifest.Files) != 8640 {
		t.Fatal("shipped library does not contain 8640 PNGs")
	}
	legacy := map[string]bool{}
	for _, motif := range c.Motifs[:60] {
		legacy[motif.ID] = true
	}
	var oldKeys []string
	for key := range library.manifest.Files {
		if legacy[strings.Split(key, "/")[2]] {
			oldKeys = append(oldKeys, key)
		}
	}
	sort.Strings(oldKeys)
	oldHash := sha256.New()
	for _, key := range oldKeys {
		fmt.Fprintf(oldHash, "%s %s\n", key, library.manifest.Files[key].SHA256)
	}
	if len(oldKeys) != 3840 || hex.EncodeToString(oldHash.Sum(nil)) != "d717b8525928bb3ed78f3b1cb5bc6efb2c4019e4bf1e64c0be10c01078e19df9" {
		t.Fatal("the original 3840 PNG paths or hashes changed")
	}
	for _, style := range c.Styles {
		for _, tone := range c.Tones {
			for _, motif := range c.Motifs {
				for _, size := range c.Sizes {
					r := IconRequest{Style: style, Tone: tone, Motif: motif.ID, Size: size}
					count := 1
					if motif.Motion == "frames" {
						count = 6
					}
					for frame := 0; frame < count; frame++ {
						_, asset, err := library.read(r, frame)
						if err != nil {
							t.Fatalf("%s: %v", r.key(frame), err)
						}
						bpp := int64(3)
						if tone == "mono" {
							bpp = 1
						}
						if assetDataSize(asset, true) != int64(size*size)*bpp {
							t.Fatal("wrong icon size")
						}
					}
				}
			}
		}
	}
}

func TestIconExpansionExportParityAndMotion(t *testing.T) {
	var exported struct {
		Motifs   []string `json:"motifs"`
		Variants int      `json:"variants"`
		PNGFiles int      `json:"png_files"`
	}
	raw, err := os.ReadFile("../../assets/cyd-icons/export-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &exported); err != nil {
		t.Fatal(err)
	}
	c := Icons()
	var ids []string
	for _, motif := range c.Motifs {
		ids = append(ids, motif.ID)
	}
	if !reflect.DeepEqual(ids, exported.Motifs) || exported.Variants != 1760 || exported.PNGFiles != 8640 {
		t.Fatal("Go catalog and deterministic exporter disagree")
	}
	library, err := LoadIconLibrary(os.DirFS("../../ui"))
	if err != nil {
		t.Fatal(err)
	}
	for _, style := range c.Styles {
		for _, tone := range c.Tones {
			for _, motif := range c.Motifs[60:] {
				if motif.Motion != "frames" {
					continue
				}
				for _, size := range c.Sizes {
					phases := map[[32]byte]bool{}
					for frame := 0; frame < 6; frame++ {
						r := IconRequest{Style: style, Tone: tone, Motif: motif.ID, Size: size}
						raw, _, err := library.read(r, frame)
						if err != nil {
							t.Fatal(err)
						}
						img, _, err := image.Decode(bytes.NewReader(raw))
						if err != nil {
							t.Fatal(err)
						}
						var pixels []byte
						for y := 0; y < size; y++ {
							for x := 0; x < size; x++ {
								red, green, blue, alpha := img.At(x, y).RGBA()
								if tone != "mono" {
									pixels = append(pixels, byte(red>>8), byte(green>>8), byte(blue>>8))
								}
								pixels = append(pixels, byte(alpha>>8))
							}
						}
						phases[sha256.Sum256(pixels)] = true
					}
					if len(phases) < 2 {
						t.Fatalf("%s/%s/%s/%d has no visible frame motion", style, tone, motif.ID, size)
					}
				}
			}
		}
	}
}

func TestIconImportAtomicDeduplicatedAndPortable(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Icons")
	library, files := testIconLibrary(t)
	r := IconRequest{Style: "modern", Tone: "mono", Motif: "coffee_machine", Size: 32, Animated: true}
	// A damaged last frame must not publish the first five.
	last := "img/cyd-icons/" + r.key(5)
	good := files[last].Data
	files[last].Data = []byte("damaged")
	_, err := env.svc.ImportIcon(ctx, p.ID, 1, r, library)
	wantCode(t, err, CodeBadRequest)
	layout, version, _, err := env.svc.GetLayout(ctx, p.ID)
	if err != nil || version != 1 || len(layout.Assets) != 0 {
		t.Fatalf("partial import: %+v v%d %v", layout.Assets, version, err)
	}
	files[last].Data = good
	res, err := env.svc.ImportIcon(ctx, p.ID, 1, r, library)
	if err != nil {
		t.Fatal(err)
	}
	if res.Version != 2 || res.AddedBytes != 6*1024 || len(res.Frames) != 6 {
		t.Fatalf("import: %+v", res)
	}
	_, err = env.svc.ImportIcon(ctx, p.ID, 1, r, library)
	wantCode(t, err, CodeProjectConflict)
	again, err := env.svc.ImportIcon(ctx, p.ID, 2, r, library)
	if err != nil || again.Version != 2 || again.AddedBytes != 0 {
		t.Fatalf("dedup: %+v %v", again, err)
	}
	layout, _, _, _ = env.svc.GetLayout(ctx, p.ID)
	layout.Screens[0].Widgets = append(layout.Screens[0].Widgets, Widget{ID: "w_icon", Type: "image", W: 32, H: 32, Props: map[string]any{"asset": res.Asset.ID, "icon_animate": true, "icon_motion": "frames", "icon_frames": res.Frames}})
	if _, _, err := env.svc.PutLayout(ctx, p.ID, layout, 2, OriginUser); err != nil {
		t.Fatal(err)
	}
	row, _ := env.svc.loadProjectRow(ctx, p.ID)
	root, err := env.svc.openRoot(row, "")
	if err != nil {
		t.Fatal(err)
	}
	revision, err := env.svc.createRevision(ctx, p.ID, root, "Icon frames", RevisionJob, "", "")
	_ = root.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.DeleteAsset(ctx, p.ID, res.Frames[5]); err != nil {
		t.Fatal(err)
	}
	if _, _, err := env.svc.RestoreRevision(ctx, revision.ID); err != nil {
		t.Fatal(err)
	}
	data, _, err := env.svc.ExportProject(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	copy, err := env.svc.ImportProject(ctx, data, "icons.zip")
	if err != nil {
		t.Fatal(err)
	}
	got, _, _, err := env.svc.GetLayout(ctx, copy.ID)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := MarshalProject(layout)
	b, _ := MarshalProject(got)
	if !bytes.Equal(a, b) {
		t.Fatal("ZIP lost icon metadata or frames")
	}
	for _, asset := range got.Assets {
		file, err := os.ReadFile(filepath.Join(env.cfg.WorkspaceDir, "CYD", copy.Slug, filepath.FromSlash(asset.File)))
		if err != nil || len(file) == 0 {
			t.Fatalf("ZIP lost asset %s: %v", asset.ID, err)
		}
	}
}

func TestIconImportBudgetAndAllowlist(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Budget icons")
	library, _ := testIconLibrary(t)
	_, version, err := env.svc.PutAsset(ctx, p.ID, "background.png", pngBytes(t, 320, 240, true))
	if err != nil {
		t.Fatal(err)
	}
	r := IconRequest{Style: "modern", Tone: "color", Motif: "gopher", Size: 64, Animated: true}
	_, err = env.svc.ImportIcon(ctx, p.ID, version, r, library)
	wantCode(t, err, CodeAssetTooLarge)
	layout, v, _, _ := env.svc.GetLayout(ctx, p.ID)
	if v != version || len(layout.Assets) != 1 {
		t.Fatal("budget rejection published assets")
	}
	r.Tone = "mono"
	res, err := env.svc.ImportIcon(ctx, p.ID, version, r, library)
	if err != nil || res.AddedBytes != 6*4096 {
		t.Fatalf("A8 budget: %+v %v", res, err)
	}
	r.Style = "../../outside"
	_, err = env.svc.ImportIcon(ctx, p.ID, res.Version, r, library)
	wantCode(t, err, CodeBadRequest)
}

func TestIconImportRepeatedFramesAndLock(t *testing.T) {
	env := newTestService(t)
	p := env.createProject(t, "Repeated frames")
	ctx := context.Background()
	library, files := testIconLibrary(t)
	r := IconRequest{Style: "modern", Tone: "mono", Motif: "wifi", Size: 32, Animated: true}
	files["img/cyd-icons/"+r.key(5)] = files["img/cyd-icons/"+r.key(0)]
	library.manifest.Files[r.key(5)] = library.manifest.Files[r.key(0)]
	res, err := env.svc.ImportIcon(ctx, p.ID, 1, r, library)
	if err != nil || res.AddedBytes != 5*1024 || res.Frames[0] != res.Frames[5] {
		t.Fatalf("repeated frame: %+v %v", res, err)
	}
	layout, _, _, _ := env.svc.GetLayout(ctx, p.ID)
	if len(layout.Assets) != 5 {
		t.Fatal("a shared frame is listed twice")
	}
	if _, err := env.svc.db.ExecContext(ctx, "UPDATE cc_projects SET locked_by_job=? WHERE id=?", "ccj_test", p.ID); err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.ImportIcon(ctx, p.ID, res.Version, r, library)
	wantCode(t, err, CodeProjectLocked)
}

func TestIconA8AndGeneratorLifecycle(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{G: 255, A: 128})
	if !reflect.DeepEqual(encodeMask(img), []byte{255, 128, 0}) {
		t.Fatal("A8 must preserve alpha, not luminance")
	}
	asset := Asset{ID: "a_icon", File: "assets/icon.png", W: 3, H: 1, Alpha: true, Encoding: "a8"}
	src := renderAssets([]genImage{{asset: asset, img: img, alpha: true}})
	if !strings.Contains(src, "LV_COLOR_FORMAT_A8, 0, 3, 1, 3, 0") || !strings.Contains(src, "0xFF, 0x80, 0x00") {
		t.Fatal(src)
	}
	p, err := NewProject("Icons", "esp32-2432s028r", "night", "en", "UTC0", &SeqIDs{})
	if err != nil {
		t.Fatal(err)
	}
	p.Assets = []Asset{asset}
	p.Screens[0].Widgets = []Widget{{ID: "w_icon", Type: "image", W: 32, H: 32, Props: map[string]any{"asset": asset.ID, "icon_animate": true, "icon_motion": "pulse"}}}
	th, err := ThemeByID(p.Theme)
	if err != nil {
		t.Fatal(err)
	}
	src = newUIGen(p, th, nil).renderSource()
	for _, marker := range []string{"lv_obj_is_visible", "lv_screen_active", "LV_EVENT_DELETE", "lv_timer_delete", "lv_free", "84, s", "lv_obj_get_style_text_color", "ui__icon_start"} {
		if !strings.Contains(src, marker) {
			t.Errorf("missing lifecycle %s", marker)
		}
	}
	p.Screens[0].Widgets[0].Style.TextColor = "#123456"
	if src = newUIGen(p, th, nil).renderSource(); !strings.Contains(src, "lv_color_hex(0x123456)") {
		t.Fatal("mask lost explicit text style")
	}
}

func TestIconFrameValidation(t *testing.T) {
	p := validFixture(t)
	p.Assets[0].W, p.Assets[0].H, p.Assets[0].Encoding = 32, 32, "a8"
	p.Assets = append(p.Assets, Asset{ID: "a_other", File: "assets/other.png", W: 24, H: 24})
	for _, frames := range [][]string{{"a_sun"}, {"a_sun", "a_sun", "a_sun", "a_sun", "a_sun", "a_missing"}, {"a_sun", "a_sun", "a_sun", "a_sun", "a_sun", "a_other"}} {
		p.Screens[0].Widgets = []Widget{{ID: "w_icon", Type: "image", W: 32, H: 32, Props: map[string]any{"asset": "a_sun", "icon_animate": true, "icon_motion": "frames", "icon_frames": frames}}}
		if issues := Validate(p, ValidateOptions{}); len(issues) == 0 {
			t.Fatalf("accepted broken frames: %v", frames)
		}
	}
}
