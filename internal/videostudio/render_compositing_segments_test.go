package videostudio

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"
)

// compositingStillPicture is a green-screen still. Around an orange subject lies the key colour #00b140. A greenish
// fringe has alpha 200, and the top-left corner is transparent, so the still's own alpha meets the key and the mask.
func compositingStillPicture(width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			c := color.NRGBA{R: 0, G: 177, B: 64, A: 255}
			switch {
			case x < width/5 && y < height/5:
				c.A = 0
			case x >= width/4 && x < width*3/4 && y >= height/4 && y < height*3/4:
				c = color.NRGBA{R: uint8(180 + x%60), G: uint8(90 + y%40), B: 40, A: 255}
			case x >= width/5 && x < width*4/5 && y >= height/5 && y < height*4/5:
				c = color.NRGBA{R: 60, G: 170, B: 80, A: 200}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// TestSegmentedRenderCompositing renders one 40-frame project with every 2d compositing form at once (index §4.22,
// risk 18). 7- and 13-frame segments must equal the single pass at 25 and 30000/1001 fps, on local FFmpeg 8.1.1 and in
// the 5.1 container.
//   - Blend track: multiply → gap → multiply ⇒ multiply (shared dissolve) ⇒ screen (mixed dissolve, normal in the
//     overlap). The multiply run crosses the 7-frame boundaries 7, 14, 21 and 28 and the 13-frame boundary 13. Each
//     transition lies inside one segment.
//   - Mask track: a keyed ellipse mask with four keys on every mask path and mixed easings, on a keyed, rotated box.
//     This is the per-frame geq in T on the pad canvas.
//   - Key track: a chroma key with keyed similarity and smoothness on a green-screen video. Spill is keyed too when
//     Task 4's command gate made it keyframable.
//   - Still track: a still with a static key, a static rotated rectangle mask (geq once, then loop) and softlight. Its
//     layer has a leading and a trailing gap and is padded in the segments that end after it.
//
// Overlay tracks take images only, so the three tracks that carry video are further video tracks.
func TestSegmentedRenderCompositing(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{
		"motion": filepath.Join(root, "motion.mp4"),
		"bars":   filepath.Join(root, "bars.mp4"),
		"green":  filepath.Join(root, "green.mp4"),
		"logo":   filepath.Join(root, "logo.png"),
	}
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["motion"], "testsrc2=s=320x180:r=25:d=3")
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["bars"], "smptebars=s=320x180:r=25:d=3")
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["green"], "color=c=0x00b140:s=320x180:r=25:d=3[bg];testsrc2=s=120x90:r=25:d=3[fg];[bg][fg]overlay=x='40+30*t':y=45[out0]")
	writePicturePNG(t, paths["logo"], compositingStillPicture(160, 120))

	key := func(f int, v float64, e Easing) Keyframe { return Keyframe{F: f, V: v, E: e} }
	dissolve := func(frames int) *Transition { return &Transition{Type: TransitionDissolve, Duration: frames} }
	visual := func(id, asset string, start, offset, duration int, x, y, w, h float64) Clip {
		return Clip{ID: id, AssetID: asset, Start: start, Offset: offset, Duration: duration, X: x, Y: y, Width: w, Height: h, Opacity: 1, Volume: 1, Fit: FitContain}
	}
	base := visual("base", "motion", 0, 0, 40, 0, 0, 1, 1)
	b1 := visual("b1", "bars", 0, 0, 12, 0.1, 0.1, 0.8, 0.8)
	b1.Blend = "multiply"
	b2 := visual("b2", "motion", 15, 5, 12, 0.1, 0.1, 0.8, 0.8)
	b2.Blend, b2.Transition = "multiply", dissolve(4)
	b3 := visual("b3", "bars", 23, 20, 10, 0.1, 0.1, 0.8, 0.8)
	b3.Blend, b3.Transition = "multiply", dissolve(3)
	b4 := visual("b4", "motion", 30, 30, 10, 0.1, 0.1, 0.8, 0.8)
	b4.Blend = "screen"

	masked := visual("masked", "motion", 0, 0, 40, 0.05, 0.1, 0.6, 0.55)
	masked.Rotation = 8
	masked.Mask = &Mask{Shape: MaskEllipse, X: 0.1, Y: 0.15, W: 0.5, H: 0.6, Rotation: -10, Feather: 0.01}
	masked.Keyframes = map[string][]Keyframe{
		"x":             {key(0, 0.05, EaseInOut), key(20, 0.25, EaseInOut), key(39, 0.10, EaseLinear)},
		"y":             {key(0, 0.10, EaseOut), key(20, 0.05, EaseIn), key(39, 0.20, EaseLinear)},
		"width":         {key(0, 0.60, EaseInOut), key(20, 0.75, EaseInOut), key(39, 0.55, EaseLinear)},
		"height":        {key(0, 0.55, EaseIn), key(20, 0.70, EaseOut), key(39, 0.60, EaseLinear)},
		"mask.x":        {key(0, 0.10, EaseInOut), key(13, 0.30, EaseOut), key(26, 0.20, EaseIn), key(39, 0.40, EaseLinear)},
		"mask.y":        {key(0, 0.15, EaseIn), key(12, 0.25, EaseInOut), key(27, 0.35, EaseLinear), key(39, 0.20, EaseOut)},
		"mask.w":        {key(0, 0.50, EaseLinear), key(10, 0.70, EaseInOut), key(25, 0.45, EaseOut), key(39, 0.60, EaseIn)},
		"mask.h":        {key(0, 0.60, EaseOut), key(14, 0.40, EaseLinear), key(28, 0.65, EaseInOut), key(39, 0.50, EaseLinear)},
		"mask.rotation": {key(0, -10, EaseInOut), key(13, 25, EaseLinear), key(26, 5, EaseInOut), key(39, 40, EaseLinear)},
		"mask.feather":  {key(0, 0.01, EaseLinear), key(12, 0.08, EaseInOut), key(27, 0.03, EaseOut), key(39, 0.12, EaseLinear)},
	}

	keyed := visual("keyed", "green", 0, 0, 40, 0.45, 0.45, 0.5, 0.5)
	chroma := EffectInstance{ID: "chroma_key", On: true, Params: map[string]any{"color": "#00b140", "similarity": 0.2, "smoothness": 0.08, "spill": 0.5},
		Keyframes: map[string][]Keyframe{
			"similarity": {key(0, 0.15, EaseInOut), key(13, 0.30, EaseLinear), key(26, 0.20, EaseOut), key(39, 0.35, EaseLinear)},
			"smoothness": {key(0, 0.02, EaseLinear), key(20, 0.15, EaseInOut), key(39, 0.05, EaseLinear)},
		}}
	spill, ok := effectParamSpec("chroma_key", "spill")
	spillKeyed := ok && spill.Keyframable
	if spillKeyed {
		chroma.Keyframes["spill"] = []Keyframe{key(0, 0.2, EaseLinear), key(20, 0.9, EaseInOut), key(39, 0.5, EaseLinear)}
	}
	t.Logf("spill keyframable (Task 4 despill gate): %t", spillKeyed)
	keyed.Effects = []EffectInstance{chroma}

	still := visual("logo", "logo", 5, 0, 30, 0.6, 0.05, 0.35, 0.3)
	still.Blend = "softlight"
	still.Mask = &Mask{Shape: MaskRect, X: 0.1, Y: 0.1, W: 0.8, H: 0.7, Rotation: 12, Feather: 0.05}
	still.Effects = []EffectInstance{{ID: "chroma_key", On: true, Params: map[string]any{"color": "#00b140", "similarity": 0.25, "smoothness": 0.1, "spill": 0.4}}}

	for _, ntsc := range []bool{false, true} {
		project := Project{Version: CurrentVersion, Name: "Compositing segments", Width: 720, Height: 720, FPS: 25}
		if ntsc {
			project.FPS, project.NTSC = 30, true
		}
		rate := ProjectRate(project)
		for _, id := range []string{"motion", "bars", "green", "logo"} {
			asset, err := Probe(ctx, ffmpeg, paths[id])
			if err != nil {
				t.Fatalf("Probe(%s): %v", id, err)
			}
			asset.ID = id
			project.Assets = append(project.Assets, AssetAtRate(asset, rate))
		}
		project.Tracks = []Track{
			{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{base}},
			{ID: "blend", Name: "Blend", Kind: TrackVideo, Clips: []Clip{b1, b2, b3, b4}},
			{ID: "mask", Name: "Mask", Kind: TrackVideo, Clips: []Clip{masked}},
			{ID: "key", Name: "Key", Kind: TrackVideo, Clips: []Clip{keyed}},
			{ID: "still", Name: "Still", Kind: TrackOverlay, Clips: []Clip{still}},
		}
		if err := Validate(project); err != nil {
			t.Fatalf("NTSC %t: compositing project invalid: %v", ntsc, err)
		}
		// The segmentations cut the multiply run and keep each transition inside one segment.
		for size, want := range map[int][]Segment{
			7:  {{0, 7}, {7, 14}, {14, 21}, {21, 28}, {28, 35}, {35, 40}},
			13: {{0, 13}, {13, 27}, {27, 40}},
		} {
			if got, err := PlanVideoSegments(project, 0, 40, size); err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("NTSC %t: %d-frame plan = %v, %v; want %v", ntsc, size, got, err, want)
			}
		}
		render := func(name string, segmentFrames int) []string {
			output := filepath.Join(root, fmt.Sprintf("%s-%t.mp4", name, ntsc))
			if err := RenderWithOptions(ctx, RenderOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Output: output,
				SegmentFrames: segmentFrames, Encoder: Encoder{Lossless: true}}); err != nil {
				t.Fatalf("render %s (NTSC %t): %v", name, ntsc, err)
			}
			return frameHashes(t, ctx, ffmpeg, output)
		}
		single := render("single", 100000)
		if len(single) != 40 {
			t.Fatalf("NTSC %t: %d frames, want 40", ntsc, len(single))
		}
		for _, size := range []int{7, 13} {
			if got := render(fmt.Sprintf("segments-%d", size), size); !reflect.DeepEqual(got, single) {
				for k := range min(len(got), len(single)) {
					if got[k] != single[k] {
						t.Fatalf("NTSC %t: %d-frame segments differ from the single pass, first at frame %d", ntsc, size, k)
					}
				}
				t.Fatalf("NTSC %t: %d-frame segments give %d frames, the single pass %d", ntsc, size, len(got), len(single))
			}
		}
	}
}

// compositingCost runs frames [0, frames) of project through the export's video graph into FFmpeg's null muxer three
// times and returns the fastest wall time. The graph is built by buildVideoSegmentWith with the RGB tail, then staged
// with stageVideoGraph and run with runFFmpegIn. threads 0 leaves FFmpeg's default filter threading, as the 2d spike
// measured; threadLimit is the export's setting.
func compositingCost(t *testing.T, ctx context.Context, ffmpeg string, project Project, paths map[string]string, frames, threads int) time.Duration {
	t.Helper()
	sources, err := collectRenderSources(ctx, ffmpeg, project, paths)
	if err != nil {
		t.Fatal(err)
	}
	rate := ProjectRate(project)
	inputs, graph, files, err := buildVideoSegmentWith(project, Segment{Start: 0, End: frames}, sources, rate, videoTail{width: project.Width, height: project.Height, rgb: true})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	workDir, graphPath, err := stageVideoGraph(root, graph, rate, files)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-copyts", "-start_at_zero"}
	args = append(args, segmentInputArgs(inputs, rate)...)
	if threads > 0 {
		args = append(args, "-filter_threads", strconv.Itoa(threads), "-filter_complex_threads", strconv.Itoa(threads))
	}
	args = append(args, "-filter_complex_script", graphPath, "-map", "[vs_video_out]", "-frames:v", strconv.Itoa(frames), "-f", "null", "-")
	best := time.Duration(math.MaxInt64)
	for range 3 {
		started := time.Now()
		if err := runFFmpegIn(ctx, workDir, ffmpeg, args, rate.Seconds(frames), nil, inputPaths(inputs), filepath.Join(root, "null-output")); err != nil {
			t.Fatalf("cost run of %q: %v", project.Name, err)
		}
		best = min(best, time.Since(started))
	}
	return best
}

// Run with AURAGO_VIDEO_STUDIO_BENCHMARK=1 go test ./internal/videostudio -run '^TestCompositingCostRecordOptIn$' -count=1 -v.
//
// TestCompositingCostRecordOptIn records what 2d compositing adds per 1080p frame (plan 2d Task 8, index §4.23). It
// asserts nothing.
//   - Setup: one 1920x1080 clip on a second video track over a full-frame video, 50 frames. Overlay tracks take images
//     only, and the clip is moving video.
//   - Each case is timed against its reference without the feature, fastest of three runs. The realistic keyed mask's
//     reference is the same keyed box without a mask; the mixed blend layer's is the same four clips without blends.
//   - Every case runs twice: with FFmpeg's default filter threads (as the 2d spike measured) and with the export's
//     threadLimit.
//
// The mixed blend layer cuts the 50 frames into four clips of one layer: normal, multiply, screen and overlay. The
// export splits it into one window-long sublayer per mode and composites them one after the other.
//
// The verdict line compares the realistic keyed mask (four keys with mixed easings on every mask path, on a keyed box)
// at default threads with three times the spike's figure: 16 ms per frame (FFmpeg 6 and later, measured on 8.1.1) or
// 52 ms (5.1.9). This is 2d remaining decision 1.
func TestCompositingCostRecordOptIn(t *testing.T) {
	if os.Getenv("AURAGO_VIDEO_STUDIO_BENCHMARK") != "1" {
		t.Skip("set AURAGO_VIDEO_STUDIO_BENCHMARK=1 to record the 2d compositing costs")
	}
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{"back": filepath.Join(root, "back.mp4"), "front": filepath.Join(root, "front.mp4")}
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["back"], "testsrc2=s=1920x1080:r=25:d=3")
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["front"], "color=c=0x00b140:s=1920x1080:r=25:d=3[bg];testsrc2=s=960x540:r=25:d=3[fg];[bg][fg]overlay=x='480+100*t':y=270[out0]")
	project := Project{Version: CurrentVersion, Name: "Compositing cost", Width: 1920, Height: 1080, FPS: 25}
	rate := ProjectRate(project)
	for _, id := range []string{"back", "front"} {
		asset, err := Probe(ctx, ffmpeg, paths[id])
		if err != nil {
			t.Fatalf("Probe(%s): %v", id, err)
		}
		asset.ID = id
		project.Assets = append(project.Assets, AssetAtRate(asset, rate))
	}
	const frames = 50
	key := func(f int, v float64, e Easing) Keyframe { return Keyframe{F: f, V: v, E: e} }
	linear := func(from, to float64) []Keyframe {
		return []Keyframe{key(0, from, EaseLinear), key(frames-1, to, EaseLinear)}
	}
	boxKeys := func() map[string][]Keyframe {
		return map[string][]Keyframe{
			"x":      {key(0, 0, EaseInOut), key(24, 0.075, EaseInOut), key(49, 0.02, EaseLinear)},
			"y":      {key(0, 0.05, EaseOut), key(24, 0, EaseIn), key(49, 0.04, EaseLinear)},
			"width":  {key(0, 1, EaseInOut), key(24, 0.85, EaseInOut), key(49, 0.95, EaseLinear)},
			"height": {key(0, 0.9, EaseIn), key(24, 1, EaseOut), key(49, 0.92, EaseLinear)},
		}
	}
	withMask := func(c *Clip) {
		c.Mask = &Mask{Shape: MaskEllipse, X: 0.2, Y: 0.15, W: 0.6, H: 0.7, Rotation: 10, Feather: 0.04}
	}
	chroma := func(keys map[string][]Keyframe) []EffectInstance {
		return []EffectInstance{{ID: "chroma_key", On: true, Params: map[string]any{"color": "#00b140", "similarity": 0.25, "smoothness": 0.1, "spill": 0.5}, Keyframes: keys}}
	}
	// clip is one clip of the front track: frames [start, start+duration) of the front source, full canvas.
	clip := func(id string, start, duration int, blend string) Clip {
		return Clip{ID: id, AssetID: "front", Start: start, Offset: start, Duration: duration, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Blend: blend}
	}
	type costCase struct {
		name, reference string // reference "" is the baseline: the same project without any 2d feature
		clips           func() []Clip
	}
	one := func(edit func(*Clip)) func() []Clip {
		return func() []Clip {
			front := clip("front", 0, frames, "")
			if edit != nil {
				edit(&front)
			}
			return []Clip{front}
		}
	}
	// four returns one layer of four clips that follow each other; blends are the modes of the last three, the first is
	// normal (a mixed layer has a normal sublayer when blends is set).
	four := func(blends ...string) func() []Clip {
		return func() []Clip {
			mode := func(i int) string {
				if len(blends) == 0 || i == 0 {
					return ""
				}
				return blends[i-1]
			}
			return []Clip{clip("c1", 0, 13, mode(0)), clip("c2", 13, 13, mode(1)), clip("c3", 26, 12, mode(2)), clip("c4", 38, 12, mode(3))}
		}
	}
	cases := []costCase{
		{"baseline", "", nil},
		{"blend multiply", "", one(func(c *Clip) { c.Blend = "multiply" })},
		{"blend overlay", "", one(func(c *Clip) { c.Blend = "overlay" })},
		{"clips four normal", "", four()},
		{"blend mixed layer (normal, multiply, screen, overlay)", "clips four normal", four("multiply", "screen", "overlay")},
		{"mask static (geq once per process)", "", one(withMask)},
		{"mask keyed linear (spike-like)", "", one(func(c *Clip) {
			withMask(c)
			c.Keyframes = map[string][]Keyframe{"mask.x": linear(0.1, 0.3), "mask.y": linear(0.1, 0.2), "mask.w": linear(0.5, 0.7),
				"mask.h": linear(0.5, 0.75), "mask.rotation": linear(0, 40), "mask.feather": linear(0.02, 0.06)}
		})},
		{"box keyed", "", one(func(c *Clip) { c.Keyframes = boxKeys() })},
		{"mask keyed realistic", "box keyed", one(func(c *Clip) {
			withMask(c)
			keys := boxKeys()
			keys["mask.x"] = []Keyframe{key(0, 0.10, EaseInOut), key(16, 0.30, EaseOut), key(33, 0.20, EaseIn), key(49, 0.40, EaseLinear)}
			keys["mask.y"] = []Keyframe{key(0, 0.15, EaseIn), key(16, 0.25, EaseInOut), key(33, 0.35, EaseLinear), key(49, 0.20, EaseOut)}
			keys["mask.w"] = []Keyframe{key(0, 0.50, EaseLinear), key(16, 0.70, EaseInOut), key(33, 0.45, EaseOut), key(49, 0.60, EaseIn)}
			keys["mask.h"] = []Keyframe{key(0, 0.60, EaseOut), key(16, 0.40, EaseLinear), key(33, 0.65, EaseInOut), key(49, 0.50, EaseLinear)}
			keys["mask.rotation"] = []Keyframe{key(0, -10, EaseInOut), key(16, 25, EaseLinear), key(33, 5, EaseInOut), key(49, 40, EaseLinear)}
			keys["mask.feather"] = []Keyframe{key(0, 0.01, EaseLinear), key(16, 0.08, EaseInOut), key(33, 0.03, EaseOut), key(49, 0.12, EaseLinear)}
			c.Keyframes = keys
		})},
		{"key static", "", one(func(c *Clip) { c.Effects = chroma(nil) })},
		{"key keyed", "", one(func(c *Clip) {
			c.Effects = chroma(map[string][]Keyframe{"similarity": linear(0.15, 0.35), "smoothness": linear(0.02, 0.15)})
		})},
	}
	variant := func(name string, clips []Clip) Project {
		p := project
		p.Name = name
		// Overlay tracks take images only, and the front clip is moving video: it lies on a second video track.
		p.Tracks = []Track{
			{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{{ID: "back", AssetID: "back", Duration: frames, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}}},
			{ID: "front", Name: "Front", Kind: TrackVideo, Clips: clips},
		}
		if err := Validate(p); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return p
	}
	major := ffmpegMajor(t, ffmpeg)
	spike := 16.0 // ms per frame for the keyed mask, FFmpeg 8.1.1 (2d spike, default threads)
	if major > 0 && major < 6 {
		spike = 52.0 // FFmpeg 5.1.9
	}
	realistic := 0.0
	for _, threads := range []int{0, threadLimit} {
		label := "default threads"
		if threads > 0 {
			label = fmt.Sprintf("%d threads (export)", threads)
		}
		took := map[string]time.Duration{}
		for _, c := range cases {
			clips := []Clip(nil)
			if c.clips != nil {
				clips = c.clips()
			} else {
				clips = one(nil)()
			}
			took[c.name] = compositingCost(t, ctx, ffmpeg, variant(c.name, clips), paths, frames, threads)
			if c.clips == nil {
				t.Logf("FFmpeg %d, %s: baseline %d ms for %d frames", major, label, took[c.name].Milliseconds(), frames)
				continue
			}
			reference := c.reference
			if reference == "" {
				reference = "baseline"
			}
			perFrame := float64(took[c.name]-took[reference]) / float64(time.Millisecond) / frames
			t.Logf("FFmpeg %d, %s: %s %+.1f ms per frame over %s (%d ms for %d frames)", major, label, c.name, perFrame, reference, took[c.name].Milliseconds(), frames)
			if threads == 0 && c.name == "mask keyed realistic" {
				realistic = perFrame
			}
		}
	}
	if bound := 3 * spike; realistic > bound {
		t.Logf("ABOVE THE BOUND: the realistic keyed mask costs %.1f ms per frame at default threads, more than 3 x %.0f ms = %.0f ms; stop for 2d remaining decision 1", realistic, spike, bound)
	} else {
		t.Logf("within the bound: the realistic keyed mask costs %.1f ms per frame at default threads (bound 3 x %.0f ms = %.0f ms)", realistic, spike, bound)
	}
}
