package storymaker

import (
	"bytes"
	"errors"
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
	// StillsSheetColumns and StillsThumbWidth shape the contact sheet: four columns of thumbnails, up to 480 pixels wide.
	StillsSheetColumns = 4
	StillsThumbWidth   = 480
	stillsGap          = 8

	// The pixel budget of the contact sheet. A film has up to MaxScenes (200) scenes and the sheet holds one thumbnail per scene,
	// so a sheet at full size would be far too large for a long film (200 stills of 9:16 are 84 million pixels, over 300 MB as
	// an RGBA buffer). The sheet keeps stillsSheetMaxPixels at most: the thumbnail width goes down from StillsThumbWidth in steps
	// of stillsThumbStep to stillsMinThumbWidth until the sheet fits; a short film (seven scenes of 16:9 are 1.1 million pixels)
	// keeps the full width. If even the narrowest thumbnails do not fit, there is no sheet (ErrSheetTooLarge); the hashes stay.
	stillsSheetMaxPixels = 16_000_000
	stillsMinThumbWidth  = 120
	stillsThumbStep      = 60

	// MaxStillBytes bounds one PNG still (a 1920x1080 screenshot is far below it) and maxStillPixels its width times its height,
	// so a still cannot make the decoder allocate more than about 130 MB (at most eight bytes a pixel).
	MaxStillBytes  = 8 << 20
	maxStillPixels = 16_000_000

	lookAlikeMinRun = 3
	// lookAlikeMaxDistance is the most bits (of 64) in which the hashes of two neighbouring scenes may differ for the scenes to
	// count as alike. Measured on the real firewall run (neighbour distances 0 0 5 10 6 3 without composition v2, 11 4 8 9 7 8
	// with it) and on the varied story (31 33 23 30 37 36): 16 flags both versions of the real run and leaves the varied story.
	lookAlikeMaxDistance = 16
)

// StillInfo is one captured still: the scene, its index in the story and the difference hash of the picture.
type StillInfo struct {
	SceneID string
	Index   int
	Hash    uint64
}

// ErrSheetTooLarge says that a contact sheet would exceed the pixel budget even with the narrowest thumbnails.
var ErrSheetTooLarge = errors.New("storymaker: the contact sheet would be too large")

// stillConfig reads the size of a still from its header, without decoding the pixels, and refuses a still over MaxStillBytes or
// maxStillPixels. Every decode goes through it, so the caps hold before any pixel memory is allocated.
func stillConfig(data []byte) (image.Config, error) {
	if len(data) > MaxStillBytes {
		return image.Config{}, fmt.Errorf("storymaker: a still of %d bytes is over the limit of %d bytes", len(data), MaxStillBytes)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return image.Config{}, fmt.Errorf("storymaker: a still is not a PNG: %w", err)
	}
	if cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > maxStillPixels {
		return image.Config{}, fmt.Errorf("storymaker: a still of %dx%d pixels is over the limit of %d pixels", cfg.Width, cfg.Height, maxStillPixels)
	}
	return cfg, nil
}

func decodeStill(data []byte) (image.Image, error) {
	if _, err := stillConfig(data); err != nil {
		return nil, err
	}
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

// sheetGeometry sizes the contact sheet of n stills of w x h pixels: the widest thumbnail (StillsThumbWidth down to
// stillsMinThumbWidth, in steps of stillsThumbStep) at which the sheet stays within stillsSheetMaxPixels, and its height. ok is
// false when even the narrowest thumbnail does not fit. The thumbnails keep the aspect of the still.
func sheetGeometry(w, h, n int) (thumbW, thumbH int, ok bool) {
	cols := min(StillsSheetColumns, n)
	rows := (n + cols - 1) / cols
	for tw := StillsThumbWidth; tw >= stillsMinThumbWidth; tw -= stillsThumbStep {
		th := max(1, int64(tw)*int64(h)/int64(max(w, 1)))
		sheetW := int64(cols*tw + (cols+1)*stillsGap)
		sheetH := int64(rows)*th + int64(rows+1)*stillsGap
		if sheetW*sheetH <= stillsSheetMaxPixels {
			return tw, int(th), true
		}
	}
	return 0, 0, false
}

// ContactSheet lays PNG stills out in rows of up to four columns, each scaled to at most 480 pixels wide, on a dark ground, and
// returns the sheet as a PNG. The order is left to right, top to bottom. The thumbnails keep the aspect of the first still, and
// all stills of a film have one size. The sheet stays within stillsSheetMaxPixels: a long film gets narrower thumbnails, and one
// that does not fit even at the narrowest gets ErrSheetTooLarge. The stills are decoded one at a time and each is drawn into
// the sheet at once, so only one decoded picture and the sheet are in memory.
func ContactSheet(pngs [][]byte) ([]byte, error) {
	if len(pngs) == 0 {
		return nil, fmt.Errorf("storymaker: a contact sheet needs at least one still")
	}
	first, err := stillConfig(pngs[0])
	if err != nil {
		return nil, fmt.Errorf("still 1: %w", err)
	}
	thumbW, thumbH, ok := sheetGeometry(first.Width, first.Height, len(pngs))
	if !ok {
		return nil, ErrSheetTooLarge
	}
	cols := min(StillsSheetColumns, len(pngs))
	rows := (len(pngs) + cols - 1) / cols
	sheet := image.NewRGBA(image.Rect(0, 0, cols*thumbW+(cols+1)*stillsGap, rows*thumbH+(rows+1)*stillsGap))
	draw.Draw(sheet, sheet.Bounds(), &image.Uniform{C: color.RGBA{R: 24, G: 24, B: 28, A: 255}}, image.Point{}, draw.Src)
	for i, data := range pngs {
		img, err := decodeStill(data)
		if err != nil {
			return nil, fmt.Errorf("still %d: %w", i+1, err)
		}
		x := stillsGap + (i%cols)*(thumbW+stillsGap)
		y := stillsGap + (i/cols)*(thumbH+stillsGap)
		xdraw.CatmullRom.Scale(sheet, image.Rect(x, y, x+thumbW, y+thumbH), img, img.Bounds(), xdraw.Over, nil)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, sheet); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// BuildStills hashes every still and lays them out as a contact sheet. The three lists have the same length and the same
// order; the stills are PNG bytes. A sheet over the pixel budget is no failure: the sheet is nil and the hashes stay, so the
// look-alike finding still works.
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
	if errors.Is(err, ErrSheetTooLarge) {
		return nil, stills, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return sheet, stills, nil
}

// LookAlikeIssues finds runs of three or more stills of neighbouring scenes whose neighbours differ in at most
// lookAlikeMaxDistance bits and reports each run once, at its third scene: that is the one to change. The fix asks for another
// environment, another framing and the actor on the other side, at every second scene of the run. The list of stills leaves out
// the scenes without a measured layout (code scenes, scenes whose hold frame is out of range), so two successive stills are
// neighbours only when their story indexes are consecutive: a gap ends the run, because the scenes across it are not seen next
// to each other in the film.
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
		if i == len(stills) || stills[i].Index != stills[i-1].Index+1 || HashDistance(stills[i-1].Hash, stills[i].Hash) > lookAlikeMaxDistance {
			flush(i)
			start = i
		}
	}
	return out
}
