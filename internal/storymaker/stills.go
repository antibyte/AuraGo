package storymaker

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/bits"
	"strings"

	xdraw "golang.org/x/image/draw"
)

const (
	// StillsSheetColumns and StillsThumbWidth shape the contact sheet: four columns of 480 pixel wide thumbnails.
	StillsSheetColumns = 4
	StillsThumbWidth   = 480
	stillsGap          = 8

	lookAlikeMinRun = 3
	// lookAlikeMaxDistance is the most bits (of 64) in which the hashes of two neighbouring scenes may differ for the scenes to
	// count as alike. The value is tuned on the real run in task E6.
	lookAlikeMaxDistance = 6
)

// StillInfo is one captured still: the scene, its index in the story and the difference hash of the picture.
type StillInfo struct {
	SceneID string
	Index   int
	Hash    uint64
}

func decodeStill(data []byte) (image.Image, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("storymaker: a still is not a PNG: %w", err)
	}
	return img, nil
}

func luma(c color.RGBA) int { return 299*int(c.R) + 587*int(c.G) + 114*int(c.B) }

// DifferenceHash is the 64 bit difference hash of a PNG: the picture is scaled to 9x8, and each bit says whether a pixel is
// brighter than its right neighbour. Pictures that look alike differ in few bits.
func DifferenceHash(data []byte) (uint64, error) {
	img, err := decodeStill(data)
	if err != nil {
		return 0, err
	}
	small := image.NewRGBA(image.Rect(0, 0, 9, 8))
	xdraw.CatmullRom.Scale(small, small.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	var hash uint64
	bit := uint(0)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if luma(small.RGBAAt(x, y)) > luma(small.RGBAAt(x+1, y)) {
				hash |= 1 << bit
			}
			bit++
		}
	}
	return hash, nil
}

// HashDistance is the number of bits in which two hashes differ.
func HashDistance(a, b uint64) int { return bits.OnesCount64(a ^ b) }

// ContactSheet lays PNG stills out in rows of up to four columns, each scaled to 480 pixels wide, on a dark ground, and
// returns the sheet as a PNG. The order is left to right, top to bottom. The thumbnails keep the aspect of the first still.
func ContactSheet(pngs [][]byte) ([]byte, error) {
	if len(pngs) == 0 {
		return nil, fmt.Errorf("storymaker: a contact sheet needs at least one still")
	}
	images := make([]image.Image, len(pngs))
	for i, data := range pngs {
		img, err := decodeStill(data)
		if err != nil {
			return nil, fmt.Errorf("still %d: %w", i+1, err)
		}
		images[i] = img
	}
	first := images[0].Bounds()
	thumbH := max(1, StillsThumbWidth*first.Dy()/max(first.Dx(), 1))
	cols := min(StillsSheetColumns, len(images))
	rows := (len(images) + cols - 1) / cols
	sheet := image.NewRGBA(image.Rect(0, 0, cols*StillsThumbWidth+(cols+1)*stillsGap, rows*thumbH+(rows+1)*stillsGap))
	draw.Draw(sheet, sheet.Bounds(), &image.Uniform{C: color.RGBA{R: 24, G: 24, B: 28, A: 255}}, image.Point{}, draw.Src)
	for i, img := range images {
		x := stillsGap + (i%cols)*(StillsThumbWidth+stillsGap)
		y := stillsGap + (i/cols)*(thumbH+stillsGap)
		xdraw.CatmullRom.Scale(sheet, image.Rect(x, y, x+StillsThumbWidth, y+thumbH), img, img.Bounds(), xdraw.Over, nil)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, sheet); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// BuildStills hashes every still and lays them out as a contact sheet. The three lists have the same length and the same
// order; the stills are PNG bytes.
func BuildStills(sceneIDs []string, indexes []int, pngs [][]byte) (sheet []byte, stills []StillInfo, err error) {
	if len(sceneIDs) != len(pngs) || len(indexes) != len(pngs) {
		return nil, nil, fmt.Errorf("storymaker: %d scene ids, %d indexes and %d stills do not match", len(sceneIDs), len(indexes), len(pngs))
	}
	for i, data := range pngs {
		hash, err := DifferenceHash(data)
		if err != nil {
			return nil, nil, fmt.Errorf("still of scene %s: %w", sceneIDs[i], err)
		}
		stills = append(stills, StillInfo{SceneID: sceneIDs[i], Index: indexes[i], Hash: hash})
	}
	sheet, err = ContactSheet(pngs)
	if err != nil {
		return nil, nil, err
	}
	return sheet, stills, nil
}

// LookAlikeIssues finds runs of three or more consecutive stills whose neighbours differ in at most lookAlikeMaxDistance bits
// and reports each run once, at its third scene: that is the one to change. The fix asks for another environment, another
// framing and the actor on the other side, at every second scene of the run.
func LookAlikeIssues(stills []StillInfo) []Issue {
	var out []Issue
	start := 0
	flush := func(end int) {
		if end-start < lookAlikeMinRun {
			return
		}
		run := stills[start:end]
		ids := make([]string, len(run))
		for i, s := range run {
			ids[i] = s.SceneID
		}
		var change []string
		for i := 2; i < len(run); i += 2 {
			change = append(change, run[i].SceneID)
		}
		out = append(out, Issue{
			Severity: SeverityWarning, SceneID: run[2].SceneID, Path: jp("scenes", run[2].Index), Code: CodeScenesLookAlike,
			Message: fmt.Sprintf("Scenes %s look alike: the same stage and nearly the same picture.", strings.Join(ids, ", ")),
			Fix: fmt.Sprintf(`Change scene %s so it differs from its neighbours: another environment with "transition":"wipe", a framing such as {"camera":{"framing":"medium"}}, and the actor on the other side ("at":"right" or "left") with the whole actors list.`,
				strings.Join(change, " and ")),
		})
	}
	for i := 1; i <= len(stills); i++ {
		if i == len(stills) || HashDistance(stills[i-1].Hash, stills[i].Hash) > lookAlikeMaxDistance {
			flush(i)
			start = i
		}
	}
	return out
}
