package storymaker

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"strings"
	"testing"
)

// stillPNG draws a 64x36 horizontal gradient, light to dark when rising is false. A patch of 2x2 bright pixels in the corner
// is a small difference that must not change the picture.
func stillPNG(t *testing.T, rising bool, patch bool) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 36))
	for y := 0; y < 36; y++ {
		for x := 0; x < 64; x++ {
			v := uint8(x * 255 / 63)
			if !rising {
				v = 255 - v
			}
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	if patch {
		for y := 0; y < 2; y++ {
			for x := 62; x < 64; x++ {
				img.Set(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDifferenceHashIsStableAndTellsPicturesApart(t *testing.T) {
	a, err := DifferenceHash(stillPNG(t, true, false))
	if err != nil {
		t.Fatal(err)
	}
	same, _ := DifferenceHash(stillPNG(t, true, false))
	patched, _ := DifferenceHash(stillPNG(t, true, true))
	flipped, _ := DifferenceHash(stillPNG(t, false, false))
	if HashDistance(a, same) != 0 {
		t.Errorf("the same picture hashes the same: %d", HashDistance(a, same))
	}
	if d := HashDistance(a, patched); d > lookAlikeMaxDistance {
		t.Errorf("a 2x2 patch moved the hash by %d bits", d)
	}
	if d := HashDistance(a, flipped); d < 48 {
		t.Errorf("a flipped gradient differs by %d bits, want almost all 64", d)
	}
	if _, err := DifferenceHash([]byte("not a png")); err == nil {
		t.Error("a broken still must be an error")
	}
}

func TestContactSheetLaysStillsOutInFourColumns(t *testing.T) {
	var pngs [][]byte
	for i := 0; i < 5; i++ {
		pngs = append(pngs, stillPNG(t, i%2 == 0, false))
	}
	sheet, err := ContactSheet(pngs)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(sheet))
	if err != nil {
		t.Fatal(err)
	}
	// 64x36 stills scaled to 480 wide are 270 high: 4 columns, 2 rows, 8 pixel gaps.
	want := image.Rect(0, 0, 4*StillsThumbWidth+5*stillsGap, 2*270+3*stillsGap)
	if img.Bounds() != want {
		t.Fatalf("sheet is %v, want %v", img.Bounds(), want)
	}
	if _, err := ContactSheet(nil); err == nil {
		t.Error("a sheet without stills is an error")
	}
	short, err := ContactSheet(pngs[:2])
	if err != nil {
		t.Fatal(err)
	}
	if img, _ := png.Decode(bytes.NewReader(short)); img.Bounds().Dx() != 2*StillsThumbWidth+3*stillsGap {
		t.Errorf("two stills use two columns, got width %d", img.Bounds().Dx())
	}
}

func TestBuildStillsKeepsOrderAndRefusesAMismatch(t *testing.T) {
	pngs := [][]byte{stillPNG(t, true, false), stillPNG(t, false, false)}
	sheet, stills, err := BuildStills([]string{"s1", "s3"}, []int{0, 2}, pngs)
	if err != nil || len(sheet) == 0 {
		t.Fatalf("BuildStills = %d bytes, %v", len(sheet), err)
	}
	if len(stills) != 2 || stills[0].SceneID != "s1" || stills[1].SceneID != "s3" || stills[1].Index != 2 || HashDistance(stills[0].Hash, stills[1].Hash) < 48 {
		t.Fatalf("stills = %+v", stills)
	}
	if _, _, err := BuildStills([]string{"s1"}, []int{0, 1}, pngs); err == nil {
		t.Error("lists of different length are refused")
	}
	if _, _, err := BuildStills([]string{"s1", "s2"}, []int{0, 1}, [][]byte{pngs[0], []byte("x")}); err == nil {
		t.Error("a broken still is refused")
	}
}

func TestLookAlikeIssuesFindsRunsOfThreeOrMore(t *testing.T) {
	still := func(id string, index int, hash uint64) StillInfo {
		return StillInfo{SceneID: id, Index: index, Hash: hash}
	}
	far := ^uint64(0)
	half := uint64(0xFFFFFFFF) // 32 bits away from both far and 0
	stills := []StillInfo{
		still("s1", 0, 0), still("s2", 1, 0b1), still("s3", 2, 0b11), // a run of three
		still("s4", 3, far),
		still("s5", 4, half), still("s6", 5, half), // a run of two
		still("s7", 6, 0), still("s8", 7, 0), still("s9", 8, 0), still("s10", 9, 0), still("s11", 10, 0), // a run of five
	}
	got := LookAlikeIssues(stills)
	if len(got) != 2 {
		t.Fatalf("issues = %+v", got)
	}
	if got[0].Code != CodeScenesLookAlike || got[0].SceneID != "s3" || got[0].Path != "/scenes/2" || got[0].Severity != SeverityWarning || got[0].Fix == "" {
		t.Errorf("first run: %+v", got[0])
	}
	if got[1].SceneID != "s9" || !strings.Contains(got[1].Fix, "s9") || !strings.Contains(got[1].Fix, "s11") {
		t.Errorf("second run: %+v", got[1])
	}
	if !IssueLayout(CodeScenesLookAlike) || !IssueModelFixable(got[0]) {
		t.Error("scenes_look_alike belongs to the layout class")
	}
	if LookAlikeIssues(stills[:2]) != nil || LookAlikeIssues(nil) != nil {
		t.Error("fewer than three stills never make a run")
	}
}

// The list of stills has no entry for a scene without a measured layout (a code scene, say): identical pictures on both sides of
// such a gap are not neighbours in the film.
func TestLookAlikeIssuesBreaksTheRunAtAnIndexGap(t *testing.T) {
	same := func(indexes ...int) []StillInfo {
		out := make([]StillInfo, len(indexes))
		for i, index := range indexes {
			out[i] = StillInfo{SceneID: fmt.Sprintf("s%d", index+1), Index: index}
		}
		return out
	}
	for _, tc := range []struct {
		name    string
		indexes []int
		want    []string // the path of each reported run
	}{
		{"a code scene between", []int{0, 2, 3}, nil},
		{"neighbours after the gap", []int{2, 3, 4}, []string{"/scenes/4"}},
		{"a run after a lone scene", []int{0, 2, 3, 4}, []string{"/scenes/4"}},
		{"two gaps", []int{0, 2, 4, 5, 7}, nil},
		{"a run on each side of a gap", []int{0, 1, 2, 4, 5, 6}, []string{"/scenes/2", "/scenes/6"}},
		{"no gap", []int{0, 1, 2}, []string{"/scenes/2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := LookAlikeIssues(same(tc.indexes...))
			var paths []string
			for _, is := range got {
				paths = append(paths, is.Path)
			}
			if !reflect.DeepEqual(paths, tc.want) {
				t.Fatalf("indexes %v: runs reported at %v, want %v (%+v)", tc.indexes, paths, tc.want, got)
			}
			for _, is := range got {
				if want := fmt.Sprintf("s%d", mustIndex(t, is.Path)+1); is.SceneID != want {
					t.Errorf("the issue at %s names scene %s, want %s", is.Path, is.SceneID, want)
				}
			}
		})
	}
}

// mustIndex reads the scene index of a "/scenes/<n>" path.
func mustIndex(t *testing.T, path string) int {
	t.Helper()
	var n int
	if _, err := fmt.Sscanf(path, "/scenes/%d", &n); err != nil {
		t.Fatalf("path %q: %v", path, err)
	}
	return n
}

// grayPNG is a uniform grey PNG of the given size; it compresses to a few bytes, whatever its size.
func grayPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sheetSize(t *testing.T, sheet []byte) image.Point {
	t.Helper()
	cfg, err := png.DecodeConfig(bytes.NewReader(sheet))
	if err != nil {
		t.Fatal(err)
	}
	return image.Pt(cfg.Width, cfg.Height)
}

func repeatBytes(data []byte, n int) [][]byte {
	out := make([][]byte, n)
	for i := range out {
		out[i] = data
	}
	return out
}

// The sheet of a long film shrinks its thumbnails to stay within 16 million pixels, and a short film keeps them at 480.
func TestContactSheetStaysWithinThePixelBudget(t *testing.T) {
	// 200 stills of a 9:16 film (MaxScenes): 480 pixel thumbnails would make 84 million pixels.
	sheet, err := ContactSheet(repeatBytes(grayPNG(t, 36, 64), 200))
	if err != nil {
		t.Fatal(err)
	}
	size := sheetSize(t, sheet)
	thumbW := (size.X - 5*stillsGap) / StillsSheetColumns
	if size.X*size.Y > stillsSheetMaxPixels || thumbW < stillsMinThumbWidth || thumbW >= StillsThumbWidth {
		t.Fatalf("a 200 scene 9:16 sheet is %v (%d pixels, %d pixel thumbnails)", size, size.X*size.Y, thumbW)
	}
	// Seven scenes of 16:9 are far below the budget: the thumbnails stay at 480 pixels, as before.
	sheet, err = ContactSheet(repeatBytes(grayPNG(t, 64, 36), 7))
	if err != nil {
		t.Fatal(err)
	}
	if size := sheetSize(t, sheet); size != image.Pt(4*StillsThumbWidth+5*stillsGap, 2*270+3*stillsGap) {
		t.Fatalf("a 7 scene 16:9 sheet is %v", size)
	}

	// The geometry: the widest thumbnail that fits, and no sheet when the narrowest does not.
	for _, tc := range []struct {
		name    string
		w, h, n int
		thumbW  int
		fits    bool
	}{
		{"seven scenes", 1920, 1080, 7, StillsThumbWidth, true},
		{"200 scenes of 16:9", 1920, 1080, 200, 360, true},
		{"200 scenes of 9:16", 1080, 1920, 200, 180, true},
		{"a hundred thousand scenes", 1920, 1080, 100000, 0, false},
		{"a very tall still", 4, 100000, 1, 0, false},
	} {
		thumbW, thumbH, ok := sheetGeometry(tc.w, tc.h, tc.n)
		if ok != tc.fits || thumbW != tc.thumbW {
			t.Errorf("%s: thumbnail width %d, fits %v; want %d, %v", tc.name, thumbW, ok, tc.thumbW, tc.fits)
			continue
		}
		if ok {
			cols := min(StillsSheetColumns, tc.n)
			rows := (tc.n + cols - 1) / cols
			if pixels := (cols*thumbW + (cols+1)*stillsGap) * (rows*thumbH + (rows+1)*stillsGap); pixels > stillsSheetMaxPixels {
				t.Errorf("%s: %d pixels", tc.name, pixels)
			}
		}
	}

	// A film whose sheet cannot fit still gets its hashes: no sheet, no error.
	tall := grayPNG(t, 4, 100000)
	if _, err := ContactSheet([][]byte{tall}); !errors.Is(err, ErrSheetTooLarge) {
		t.Fatalf("ContactSheet of a very tall still = %v, want ErrSheetTooLarge", err)
	}
	sheet, stills, err := BuildStills([]string{"s1"}, []int{0}, [][]byte{tall})
	if err != nil || sheet != nil || len(stills) != 1 || stills[0].SceneID != "s1" {
		t.Fatalf("BuildStills of a very tall still: sheet %d bytes, stills %+v, error %v", len(sheet), stills, err)
	}
}

// A still over 8 MB, or over 16 million pixels, is refused before it is decoded.
func TestStillsRefuseOversizedPNGs(t *testing.T) {
	tooBig := make([]byte, MaxStillBytes+1)
	copy(tooBig, "\x89PNG\r\n\x1a\n")
	tooWide := grayPNG(t, 4100, 4100) // 16.8 million pixels in a few hundred bytes
	for name, data := range map[string][]byte{"too many bytes": tooBig, "too many pixels": tooWide} {
		if _, err := DifferenceHash(data); err == nil {
			t.Errorf("%s: DifferenceHash accepted the still", name)
		}
		if _, err := ContactSheet([][]byte{stillPNG(t, true, false), data}); err == nil {
			t.Errorf("%s: ContactSheet accepted the still", name)
		}
		if _, err := ContactSheet([][]byte{data}); err == nil {
			t.Errorf("%s: ContactSheet accepted the still as its first", name)
		}
		if _, _, err := BuildStills([]string{"s1"}, []int{0}, [][]byte{data}); err == nil {
			t.Errorf("%s: BuildStills accepted the still", name)
		}
	}
	// The limits are inclusive: a still of exactly 4000x4000 pixels is fine.
	if _, err := DifferenceHash(grayPNG(t, 4000, 4000)); err != nil {
		t.Errorf("16 million pixels exactly: %v", err)
	}
}
