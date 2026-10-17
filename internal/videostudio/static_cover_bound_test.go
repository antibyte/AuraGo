package videostudio

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"
)

// The pre-crop of a static cover clip and the cover scale that follows it, as the transform stage writes them.
var (
	staticClampPattern = regexp.MustCompile(`^crop=w='min\(iw,max\(ih\*([0-9.]+),(\d+)\)\)':h='min\(ih,max\(iw/([0-9.]+),(\d+)\)\)',scale=`)
	coverScalePattern  = regexp.MustCompile(`scale=w=(\d+):h=(\d+):force_original_aspect_ratio=increase:force_divisible_by=2,crop=w=\d+:h=\d+`)
)

// staticCoverRun is what the transform of a static cover clip does to a source of a given size: the size after the
// pre-crop (the source itself without one), the cover-scaled intermediate that FFmpeg holds before the box crop, and the
// scale factor of the source pixels.
type staticCoverRun struct {
	box, cropped, scaled image.Point
	factor               float64
	cropText             bool
}

// followStaticCover evaluates the pre-crop expression and the cover scale of transform for a source of sourceWidth x
// sourceHeight pixels (FFmpeg truncates the crop expression to whole pixels; the scale rounds up to even sizes).
func followStaticCover(t *testing.T, transform string, sourceWidth, sourceHeight int) staticCoverRun {
	t.Helper()
	run := staticCoverRun{cropped: image.Pt(sourceWidth, sourceHeight)}
	if m := staticClampPattern.FindStringSubmatch(transform); m != nil {
		number := func(text string) float64 {
			value, err := strconv.ParseFloat(text, 64)
			if err != nil {
				t.Fatalf("number %q in the pre-crop of %s: %v", text, transform, err)
			}
			return value
		}
		high, minWidth, low, minHeight := number(m[1]), number(m[2]), number(m[3]), number(m[4])
		run.cropped = image.Pt(int(math.Min(float64(sourceWidth), math.Max(float64(sourceHeight)*high, minWidth))), int(math.Min(float64(sourceHeight), math.Max(float64(sourceWidth)/low, minHeight))))
		run.cropText = true
	}
	scale := coverScalePattern.FindStringSubmatch(transform)
	if scale == nil {
		t.Fatalf("no cover scale in %s", transform)
	}
	boxWidth, _ := strconv.Atoi(scale[1])
	boxHeight, _ := strconv.Atoi(scale[2])
	run.box = image.Pt(boxWidth, boxHeight)
	run.factor = math.Max(float64(boxWidth)/float64(run.cropped.X), float64(boxHeight)/float64(run.cropped.Y))
	run.scaled = image.Pt(evenCeil(float64(run.cropped.X)*run.factor), evenCeil(float64(run.cropped.Y)*run.factor))
	return run
}

// A static cover clip scales its source until the box is covered, so a still far wider or taller than the box made a
// cover-scaled intermediate of any size: a 16384x16 PNG in a 1920x1080 box scaled to about 1.1 million pixels wide (1.8 GB
// a frame) and FFmpeg ran out of memory. Stills are exempt from the video aspect rule (titles are cropped text blocks),
// so the transform pre-crops the source to within 16x of the box aspect. The intermediate then stays within the multiple
// of the box that coverBound allows, and the box shows the same pixels as before: cover trims what the crop removes.
func TestStaticCoverOfExtremeImageIsBounded(t *testing.T) {
	// The longest side is 16 times a box side, plus the margin (1/8 and 4 px), as in TestCoverBoundStaysNearTheCanvas.
	// A source with fewer pixels than the clamp can express keeps 2 px across, which the other box side scales up to at
	// most twice that side (a 20x20 picture in a box 2 px wide is scaled to 108x1080).
	limit := func(side, other int) int { return max(int(math.Ceil(maxCoverAspect*float64(side)*1.125))+4, 2*other) }
	canvases := []image.Point{{X: 1920, Y: 1080}, {X: 720, Y: 1280}}
	boxes := []struct {
		name          string
		width, height float64
	}{{"full", 1, 1}, {"half", 0.5, 0.5}, {"banner", 1, 0.1}, {"column", 0.1, 1}, {"sliver tall", 0.001, 1}, {"sliver wide", 1, 0.001}}
	sources := []image.Point{{X: 16384, Y: 16}, {X: 16, Y: 16384}, {X: 2048, Y: 8}, {X: 8, Y: 2048}, {X: 4096, Y: 64}, {X: 2368, Y: 64},
		{X: 1920, Y: 1080}, {X: 1080, Y: 1920}, {X: 640, Y: 480}, {X: 100, Y: 100}, {X: 20, Y: 20}, {X: 10, Y: 10}, {X: 16, Y: 16}}
	clamped := 0
	for _, canvas := range canvases {
		for _, box := range boxes {
			for _, source := range sources {
				name := fmt.Sprintf("%dx%d/%s/%dx%d", canvas.X, canvas.Y, box.name, source.X, source.Y)
				clip := Clip{ID: "c", AssetID: "a", Duration: 30, Width: box.width, Height: box.height, Opacity: 1, Volume: 1, Fit: FitCover}
				chain := clipChain{item: layerClip{clip: clip, whole: clip, image: true, sourceAspect: float64(source.X) / float64(source.Y)}, canvasWidth: canvas.X, canvasHeight: canvas.Y, rate: Rate{Num: 30, Den: 1}}
				transform := chain.transform()
				run := followStaticCover(t, transform, source.X, source.Y)
				if run.cropText {
					clamped++
				}
				maxX, maxY := limit(run.box.X, run.box.Y), limit(run.box.Y, run.box.X)
				if run.scaled.X > maxX || run.scaled.Y > maxY {
					t.Errorf("%s: cover-scaled intermediate %v, want at most %dx%d (box %v)\n%s", name, run.scaled, maxX, maxY, run.box, transform)
				}
				if run.scaled.X < run.box.X || run.scaled.Y < run.box.Y {
					t.Errorf("%s: cover-scaled intermediate %v does not cover the box %v", name, run.scaled, run.box)
				}
				if run.cropped.X < min(2, source.X) || run.cropped.Y < min(2, source.Y) {
					t.Errorf("%s: pre-crop leaves %v of the %v source", name, run.cropped, source)
				}
				// The box shows the same picture as without the pre-crop: the scale factor is the one of the whole source.
				whole := math.Max(float64(run.box.X)/float64(source.X), float64(run.box.Y)/float64(source.Y))
				if math.Abs(run.factor/whole-1) > 1e-9 {
					t.Errorf("%s: scale factor %v after the pre-crop, %v without it (the box would show another picture)", name, run.factor, whole)
				}
			}
		}
	}
	if clamped == 0 {
		t.Fatal("no combination was pre-cropped: the test does not exercise the clamp")
	}
}

// The two cases of the report, spelled out in numbers: a 16384x16 picture in a 1920x1080 box keeps about 455x16 pixels and
// is scaled to about 30720x1080 (it was 1105920x1080); a 16x16384 one keeps about 16x144 and is scaled to about 1920x17280.
// "About" is the truncation of the crop expression and the rounding of the scale (a few pixels).
func TestStaticCoverOfExtremeImageNumbers(t *testing.T) {
	clip := Clip{ID: "c", AssetID: "a", Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitCover}
	near := func(got, want, slack int) bool { return got >= want-slack && got <= want+slack }
	for _, c := range []struct {
		source, cropped, scaled image.Point
	}{
		{image.Pt(16384, 16), image.Pt(455, 16), image.Pt(30720, 1080)},
		{image.Pt(16, 16384), image.Pt(16, 144), image.Pt(1920, 17280)},
	} {
		chain := clipChain{item: layerClip{clip: clip, whole: clip, image: true, sourceAspect: float64(c.source.X) / float64(c.source.Y)}, canvasWidth: 1920, canvasHeight: 1080, rate: Rate{Num: 30, Den: 1}}
		run := followStaticCover(t, chain.transform(), c.source.X, c.source.Y)
		if !near(run.cropped.X, c.cropped.X, 1) || !near(run.cropped.Y, c.cropped.Y, 1) || !near(run.scaled.X, c.scaled.X, 200) || !near(run.scaled.Y, c.scaled.Y, 200) {
			t.Errorf("%v: pre-crop %v and intermediate %v, want about %v and %v\n%s", c.source, run.cropped, run.scaled, c.cropped, c.scaled, chain.transform())
		}
	}
}

// A source within 16x of the box aspect (in both orientations, because FFmpeg's autorotation may swap it) gets the chain
// it always had: no pre-crop. So does a source of unknown size.
func TestStaticCoverWithinTheFactorIsUnchanged(t *testing.T) {
	clip := Clip{ID: "c", AssetID: "a", Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitCover}
	transform := func(clip Clip, sourceAspect float64) string {
		return clipChain{item: layerClip{clip: clip, whole: clip, sourceAspect: sourceAspect}, canvasWidth: 1280, canvasHeight: 720, rate: Rate{Num: 30, Den: 1}}.transform()
	}
	plain := transform(clip, 0)
	if want := "scale=w=1280:h=720:force_original_aspect_ratio=increase:force_divisible_by=2,crop=w=1280:h=720,format=rgba"; plain != want {
		t.Fatalf("static cover without a source aspect = %s, want %s", plain, want)
	}
	// Box 16:9: a source and its turned shape both stay within 16x of the box for the aspects 1/9 to 9.
	for _, aspect := range []float64{16.0 / 9, 1, 9.0 / 16, 4.0 / 3, 2.39, 8, 1.0 / 8, 0} {
		if got := transform(clip, aspect); got != plain {
			t.Errorf("source aspect %v: %s\nwant the unchanged %s", aspect, got, plain)
		}
	}
	// A banner box (1280x72, 17.8:1) with a 16:9 video: the video is within 16x of the box, its turned shape (9:16) is not.
	// The pre-crop of that orientation is a no-op for the upright video, so the box shows the same picture either way.
	banner := clip
	banner.Height = 0.1
	if got := transform(banner, 16.0/9); got == transform(banner, 0) {
		t.Errorf("banner box, 16:9 source: no pre-crop, but its turned shape is more than 16x off the box")
	}
	// Outside the factor in the upright orientation, the pre-crop is there.
	for _, aspect := range []float64{29, 1024, 0.1, 1.0 / 1024} {
		if got := transform(clip, aspect); !staticClampPattern.MatchString(got) {
			t.Errorf("source aspect %v: no pre-crop in %s", aspect, got)
		}
	}
	// The limit itself (exactly 16x, in both orientations of a 16:1 source in a square box) is within the factor.
	square := clip
	square.Width, square.Height = 0.5625, 1 // 720x720 on the 1280x720 canvas
	if got := transform(square, 16); got != transform(square, 0) {
		t.Errorf("square box, 16:1 source sits exactly on the limit, but got %s", got)
	}
}

// The pre-crop reaches FFmpeg: a still far wider or taller than the box renders, fills the box and shows the picture's
// centre, as a cover clip does. The box is 320x180 at the top left of the smallest canvas (1280x720); the 1920x1080
// case of the report is covered by the numbers above, because rendering it without the pre-crop needs gigabytes.
func TestRenderStaticCoverOfExtremeImage(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := t.TempDir()
	// Blue everywhere except a red bar around the centre that is wider than the box shows.
	writeBarPNG := func(path string, width, height int, bar image.Rectangle) {
		img := image.NewNRGBA(image.Rect(0, 0, width, height))
		for y := range height {
			for x := range width {
				pixel := color.NRGBA{B: 255, A: 255}
				if image.Pt(x, y).In(bar) {
					pixel = color.NRGBA{R: 255, A: 255}
				}
				img.SetNRGBA(x, y, pixel)
			}
		}
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, img); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	paths := map[string]string{"wide": filepath.Join(root, "wide.png"), "tall": filepath.Join(root, "tall.png"), "huge": filepath.Join(root, "huge.png")}
	writeBarPNG(paths["wide"], 2048, 8, image.Rect(960, 0, 1088, 8))
	writeBarPNG(paths["tall"], 8, 2048, image.Rect(0, 960, 8, 1088))
	writeBarPNG(paths["huge"], 16384, 16, image.Rect(8128, 0, 8256, 16))
	for _, id := range []string{"wide", "tall", "huge"} {
		t.Run(id, func(t *testing.T) {
			assets := probeFrameAssets(t, ctx, ffmpeg, paths, id)
			project := Project{Version: CurrentVersion, Name: "Extreme still", Width: 1280, Height: 720, FPS: 25, Assets: assets,
				Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{
					{ID: "cover", AssetID: id, Duration: 3, Width: 0.25, Height: 0.25, Opacity: 1, Volume: 1, Fit: FitCover}}}}}
			if err := Validate(project); err != nil {
				t.Fatalf("project invalid: %v", err)
			}
			box := image.Rect(0, 0, 320, 180)
			frames := renderRawRGB(t, ctx, ffmpeg, project, paths, Segment{Start: 0, End: 3})
			for k, frame := range frames {
				bounds, found := redBounds(frame, 1280, 720)
				if !found || bounds != box {
					t.Fatalf("frame %d: red area %v (found %v), want the box %v", k, bounds, found, box)
				}
				red := 0
				for i := 0; i < len(frame); i += 3 {
					if frame[i] >= 128 && frame[i+1] <= 63 && frame[i+2] <= 63 {
						red++
					}
				}
				if red != box.Dx()*box.Dy() {
					t.Fatalf("frame %d: %d of %d box pixels are red, want all (the box shows the centre of the picture)", k, red, box.Dx()*box.Dy())
				}
			}
		})
	}
}
