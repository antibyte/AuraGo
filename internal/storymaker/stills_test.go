package storymaker

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
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
