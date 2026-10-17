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
	"reflect"
	"testing"
	"time"
)

// animatedCropSSIM is goldenframe.ThresholdGeometry. goldenframe imports this package, so these tests cannot import it.
const animatedCropSSIM = 0.98

// animatedCoverSSIM is the SSIM floor of the cover subtests of TestRenderAnimatedCropMatchesStatic. The static
// reference takes the 2b static cover path: scale with force_original_aspect_ratio=increase (rounded to even), then a
// centre crop. The animated path crops an integer box-aspect window (coverWindow) and scales it exactly, so the two
// frames differ by up to about 0.3 output px, which the red marker's edge and the chart lines turn into a lower SSIM.
// Measured minima over the 16 frames of one run:
//   - FFmpeg 8.1.1: 0.9651 (cover_flip), 0.9692 (cover_keyed_box);
//   - FFmpeg 5.1.9: 0.9369 (cover_flip), 0.9459 (cover_keyed_box).
//
// A real geometry error of 1–2 output px scores 0.50–0.59, so 0.90 still catches it. Contain keeps animatedCropSSIM.
const animatedCoverSSIM = 0.90

// writePicturePNG writes img as a PNG fixture. Go stores an opaque picture as 8-bit RGB.
func writePicturePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
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

// cropChart is an opaque test picture:
//   - a colour gradient whose blue stays above 63, so redBounds never counts it;
//   - dark grid lines every 64 px, for structure;
//   - a pure red marker left of the centre, so a horizontal flip moves it.
func cropChart(width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	marker := image.Rect(width*15/32, height*5/12, width*19/32, height*7/12)
	for y := range height {
		for x := range width {
			c := color.NRGBA{R: uint8(40 + x*120/width), G: uint8(60 + y*100/height), B: 170, A: 255}
			if x%64 == 0 || y%64 == 0 {
				c = color.NRGBA{R: 20, G: 20, B: 90, A: 255}
			}
			if image.Pt(x, y).In(marker) {
				c = color.NRGBA{R: 255, A: 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// markerPicture is a black picture with a pure red marker.
func markerPicture(width, height int, marker image.Rectangle) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			c := color.NRGBA{A: 255}
			if image.Pt(x, y).In(marker) {
				c.R = 255
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// cropRGB copies the rectangle r out of an rgb24 frame that is stride pixels wide.
func cropRGB(frame []byte, stride int, r image.Rectangle) []byte {
	out := make([]byte, 0, r.Dx()*r.Dy()*3)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		out = append(out, frame[(y*stride+r.Min.X)*3:(y*stride+r.Max.X)*3]...)
	}
	return out
}

// frameSSIM is goldenframe.SSIM for two rgb24 frames of one size. It returns the smallest of the three per-channel
// mean structural similarities: 11x11 Gaussian window, sigma 1.5, C1 = (0.01·255)², C2 = (0.03·255)², only windows that
// fit completely.
func frameSSIM(a, b []byte, width, height int) float64 {
	const window, sigma = 11, 1.5
	c1, c2 := math.Pow(0.01*255, 2), math.Pow(0.03*255, 2)
	var kernel [window]float64
	sum := 0.0
	for i := range kernel {
		d := float64(i - window/2)
		kernel[i] = math.Exp(-d * d / (2 * sigma * sigma))
		sum += kernel[i]
	}
	for i := range kernel {
		kernel[i] /= sum
	}
	ow, oh := width-window+1, height-window+1
	blur := func(plane []float64) []float64 {
		rows := make([]float64, ow*height)
		for y := range height {
			for x := range ow {
				s := 0.0
				for k, w := range kernel {
					s += w * plane[y*width+x+k]
				}
				rows[y*ow+x] = s
			}
		}
		out := make([]float64, ow*oh)
		for y := range oh {
			for x := range ow {
				s := 0.0
				for k, w := range kernel {
					s += w * rows[(y+k)*ow+x]
				}
				out[y*ow+x] = s
			}
		}
		return out
	}
	n := width * height
	score := 1.0
	for c := range 3 {
		p, q, pp, qq, pq := make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
		for i := range n {
			p[i], q[i] = float64(a[i*3+c]), float64(b[i*3+c])
			pp[i], qq[i], pq[i] = p[i]*p[i], q[i]*q[i], p[i]*q[i]
		}
		mu1, mu2, s11, s22, s12 := blur(p), blur(q), blur(pp), blur(qq), blur(pq)
		total := 0.0
		for i := range mu1 {
			m1, m2 := mu1[i], mu2[i]
			total += (2*m1*m2 + c1) * (2*(s12[i]-m1*m2) + c2) / ((m1*m1 + m2*m2 + c1) * (s11[i] - m1*m1 + s22[i] - m2*m2 + c2))
		}
		score = math.Min(score, total/float64(len(mu1)))
	}
	return score
}

// boundsWithin reports rectangles whose edges differ by at most slack pixels.
func boundsWithin(got, want image.Rectangle, slack int) bool {
	return absInt(got.Min.X-want.Min.X) <= slack && absInt(got.Min.Y-want.Min.Y) <= slack &&
		absInt(got.Max.X-want.Max.X) <= slack && absInt(got.Max.Y-want.Max.Y) <= slack
}

// A static crop with flips (index §4.20). The crop fractions refer to the unflipped source and the flips follow them,
// so a red marker lands where cropRect and the mirror put it. The crop is 400x224 (l 0.25, t 0.25, r 0.125, b 0.125
// of 640x360) and sits in a 400x224 box, so nothing is resampled:
//   - the still's marker must arrive exactly;
//   - the H.264 video may blur the marker's edge by a pixel.
func TestRenderCropAndFlip(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{"still": filepath.Join(root, "marker.png"), "video": filepath.Join(root, "marker.mp4")}
	writePicturePNG(t, paths["still"], markerPicture(640, 360, image.Rect(200, 120, 240, 160)))
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["video"], "color=c=black:s=640x360:r=25:d=2,drawbox=x=200:y=120:w=40:h=40:color=red:t=fill")
	assets := probeFrameAssets(t, ctx, ffmpeg, paths, "still", "video")
	crop := &Crop{L: 0.25, T: 0.25, R: 0.125, B: 0.125}
	if x, y, w, h := cropRect(*crop, 640, 360); x != 160 || y != 90 || w != 400 || h != 224 {
		t.Fatalf("cropRect = (%d, %d, %d, %d), want (160, 90, 400, 224)", x, y, w, h)
	}
	for _, c := range []struct {
		name, asset  string
		flipH, flipV bool
		want         image.Rectangle
		slack        int
	}{
		{"still_no_flip", "still", false, false, image.Rect(40, 30, 80, 70), 0},
		{"still_both_flips", "still", true, true, image.Rect(320, 154, 360, 194), 0},
		{"video_flip_h", "video", true, false, image.Rect(320, 30, 360, 70), 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			clip := Clip{ID: "pic", AssetID: c.asset, Duration: 10, Width: 400.0 / 1280, Height: 224.0 / 720, Opacity: 1, Volume: 1, Fit: FitContain,
				Crop: crop, FlipH: c.flipH, FlipV: c.flipV}
			project := Project{Version: CurrentVersion, Name: "Crop", Width: 1280, Height: 720, FPS: 25, Assets: assets,
				Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{clip}}}}
			if err := Validate(project); err != nil {
				t.Fatalf("crop project invalid: %v", err)
			}
			frame := renderRawRGB(t, ctx, ffmpeg, project, paths, Segment{Start: 3, End: 4})[0]
			got, ok := redBounds(frame, 1280, 720)
			if !ok || !boundsWithin(got, c.want, c.slack) {
				t.Fatalf("marker at %v (found %t), want %v within %d px", got, ok, c.want, c.slack)
			}
		})
	}
}

// The FFmpeg 5.1 gate of plan 2c (decision A3, index risk 14). The clip's crop is keyframed, so its size and position
// change on every frame. Each frame must render like a project whose static crop holds that frame's values:
//   - contain: the same crop rectangle and scale, so the red marker's bounds are exact;
//   - cover: the animated chain cuts the cover window in whole source pixels and scales it exactly, while the static
//     chain scales the crop and then cuts the box, so the bounds may differ by one output pixel (the 1280x720 chart is
//     scaled down into a box of about 360x216);
//   - every frame's box keeps SSIM >= 0.98 (goldenframe.ThresholdGeometry) for contain and >= animatedCoverSSIM for
//     cover.
//
// It runs on local FFmpeg 8.1.1 and in the FFmpeg 5.1 container (plan 2c Task 7 Step 8, Task 18). A 5.1 failure stops
// the slice for a report.
func TestRenderAnimatedCropMatchesStatic(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{"chart": filepath.Join(root, "chart.png")}
	writePicturePNG(t, paths["chart"], cropChart(1280, 720))
	assets := probeFrameAssets(t, ctx, ffmpeg, paths, "chart")
	const frames = 16
	keys := func(from, to float64) []Keyframe {
		return []Keyframe{{F: 0, V: from, E: EaseLinear}, {F: frames - 1, V: to, E: EaseLinear}}
	}
	value := func(clip Clip, path string, frame int) float64 {
		v, ok := ValueAt(clip, path, frame)
		if !ok {
			t.Fatalf("ValueAt(%q) reports an unknown path", path)
		}
		return v
	}
	project := func(clip Clip) Project {
		return Project{Version: CurrentVersion, Name: "Crop", Width: 720, Height: 720, FPS: 25, Assets: assets,
			Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{clip}}}}
	}
	for _, c := range []struct {
		name     string
		fit      FitMode
		flipH    bool
		keyedBox bool
		slack    int
		ssim     float64
	}{
		{"contain", FitContain, false, false, 0, animatedCropSSIM},
		{"cover_flip", FitCover, true, false, 1, animatedCoverSSIM},
		{"cover_keyed_box", FitCover, false, true, 1, animatedCoverSSIM},
	} {
		t.Run(c.name, func(t *testing.T) {
			clip := Clip{ID: "pic", AssetID: "chart", Duration: frames, Width: 0.5, Height: 0.3, Opacity: 1, Volume: 1, Fit: c.fit, FlipH: c.flipH,
				Keyframes: map[string][]Keyframe{"crop.l": keys(0.05, 0.25), "crop.t": keys(0.1, 0.05), "crop.r": keys(0.05, 0.15), "crop.b": keys(0.05, 0.25)}}
			if c.keyedBox {
				clip.Keyframes["width"], clip.Keyframes["height"] = keys(0.4, 0.6), keys(0.35, 0.25)
			}
			if err := Validate(project(clip)); err != nil {
				t.Fatalf("animated crop project invalid: %v", err)
			}
			animated := renderRawRGB(t, ctx, ffmpeg, project(clip), paths, Segment{Start: 0, End: frames})
			var first, last image.Rectangle
			for k, frame := range animated {
				still := clip
				still.Keyframes = nil
				still.Crop = &Crop{L: value(clip, "crop.l", k), T: value(clip, "crop.t", k), R: value(clip, "crop.r", k), B: value(clip, "crop.b", k)}
				still.Width, still.Height = value(clip, "width", k), value(clip, "height", k)
				if err := Validate(project(still)); err != nil {
					t.Fatalf("frame %d: static crop project invalid: %v", k, err)
				}
				want := renderRawRGB(t, ctx, ffmpeg, project(still), paths, Segment{Start: 0, End: 1})[0]
				got, gotOK := redBounds(frame, 720, 720)
				expected, wantOK := redBounds(want, 720, 720)
				if !gotOK || !wantOK || !boundsWithin(got, expected, c.slack) {
					t.Fatalf("frame %d: marker at %v (found %t); the static crop shows it at %v (found %t); slack %d px", k, got, gotOK, expected, wantOK, c.slack)
				}
				box := keyframedBox(t, clip, k, 720, 720)
				if score := frameSSIM(cropRGB(want, 720, box), cropRGB(frame, 720, box), box.Dx(), box.Dy()); score < c.ssim {
					t.Fatalf("frame %d: SSIM %.4f against the static crop, want at least %.2f", k, score, c.ssim)
				}
				if k == 0 {
					first = got
				}
				last = got
			}
			if first == last {
				t.Fatalf("the marker stayed at %v: the crop did not animate", first)
			}
		})
	}
}

// The test renders three clips in one segment and in 7- and 13-frame segments, and every segmentation must give the
// single pass's frames:
//   - commanded crops of a cover video with flip_h;
//   - a static crop with flip_v;
//   - an overlay whose crop and keyframed x go through the animated contain chain.
func TestSegmentedRenderCrop(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{"motion": filepath.Join(root, "motion.mp4"), "chart": filepath.Join(root, "chart.png")}
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["motion"], "testsrc2=s=320x180:r=25:d=4")
	writePicturePNG(t, paths["chart"], cropChart(640, 360))
	project := Project{Version: CurrentVersion, Name: "Crop segments", Width: 720, Height: 720, FPS: 25}
	rate := ProjectRate(project)
	for _, id := range []string{"motion", "chart"} {
		asset, err := Probe(ctx, ffmpeg, paths[id])
		if err != nil {
			t.Fatalf("Probe(%s): %v", id, err)
		}
		asset.ID = id
		project.Assets = append(project.Assets, AssetAtRate(asset, rate))
	}
	video := Clip{ID: "a", AssetID: "motion", Duration: 60, Width: 1, Height: 0.6, Opacity: 1, Volume: 1, Fit: FitCover, FlipH: true, Crop: &Crop{T: 0.05},
		Keyframes: map[string][]Keyframe{
			"crop.l": {{F: 0, V: 0, E: EaseInOut}, {F: 50, V: 0.3, E: EaseLinear}},
			"crop.b": {{F: 10, V: 0.2, E: EaseOut}, {F: 45, V: 0, E: EaseLinear}},
		}}
	still := Clip{ID: "b", AssetID: "chart", Start: 60, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, FlipV: true,
		Crop: &Crop{L: 0.1, T: 0.2, R: 0.05, B: 0.1}}
	overlay := Clip{ID: "o", AssetID: "chart", Start: 5, Duration: 70, X: 0.5, Y: 0.6, Width: 0.4, Height: 0.3, Opacity: 1, Volume: 1, Fit: FitContain,
		Keyframes: map[string][]Keyframe{
			"crop.r": {{F: 0, V: 0, E: EaseLinear}, {F: 60, V: 0.4, E: EaseLinear}},
			"x":      {{F: 0, V: 0.5, E: EaseLinear}, {F: 69, V: 0.1, E: EaseLinear}},
		}}
	project.Tracks = []Track{
		{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{video, still}},
		{ID: "overlay", Name: "Overlay", Kind: TrackOverlay, Clips: []Clip{overlay}},
	}
	if err := Validate(project); err != nil {
		t.Fatalf("crop segment project invalid: %v", err)
	}
	render := func(name string, segmentFrames int) []string {
		output := filepath.Join(root, name+".mp4")
		if err := RenderWithOptions(ctx, RenderOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Output: output,
			SegmentFrames: segmentFrames, Encoder: Encoder{Lossless: true}}); err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		return frameHashes(t, ctx, ffmpeg, output)
	}
	single := render("single", 100000)
	if len(single) != 90 {
		t.Fatalf("%d frames, want 90", len(single))
	}
	for _, size := range []int{7, 13} {
		if got := render(fmt.Sprintf("segments-%d", size), size); !reflect.DeepEqual(got, single) {
			t.Fatalf("%d-frame segments differ from the single pass", size)
		}
	}
}
