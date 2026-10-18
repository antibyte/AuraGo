package videostudio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// TestSegmentedRenderAnim renders one 40-frame project with every 2e animation form at once (index §4.24, segment
// exactness). 7- and 13-frame segments must equal the single pass at 25 and 30000/1001 fps, on local FFmpeg 8.1.1 and in
// the 5.1 container. Every animation crosses segment boundaries, and the 13-frame plan ends in a one-frame segment.
//   - Video track: a fade-in clip with a circleopen transition into a clip whose slide_up in-animation plays inside the
//     transition, then fades out.
//   - Masked track: pop in on an ellipse-masked clip (the per-frame mask on the pad canvas), wipe out.
//   - Blur track: blur_in on video (the opaque cascade), slide_left out.
//   - Ramp track: a ramped clip with a pop in and a fade out.
//   - Still track: blur_in on a still with alpha (the premultiplied cascade), wipe out.
//
// Overlay tracks take images only, so the four video layers lie on video tracks.
func TestSegmentedRenderAnim(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{
		"motion": filepath.Join(root, "motion.mp4"),
		"bars":   filepath.Join(root, "bars.mp4"),
		"logo":   filepath.Join(root, "logo.png"),
	}
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["motion"], "testsrc2=s=320x180:r=25:d=4")
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["bars"], "smptebars=s=320x180:r=25:d=4")
	writePicturePNG(t, paths["logo"], compositingStillPicture(160, 120))

	anim := func(preset string, frames int) *ClipAnim { return &ClipAnim{Preset: preset, Duration: frames} }
	visual := func(id, asset string, start, offset, duration int, x, y, w, h float64) Clip {
		return Clip{ID: id, AssetID: asset, Start: start, Offset: offset, Duration: duration, X: x, Y: y, Width: w, Height: h, Opacity: 1, Volume: 1, Fit: FitContain}
	}
	first := visual("first", "motion", 0, 0, 22, 0, 0, 1, 1)
	first.AnimIn = anim("fade", 6)
	first.Transition = &Transition{Type: TransitionType("circleopen"), Duration: 6}
	second := visual("second", "bars", 16, 0, 24, 0, 0, 1, 1)
	second.AnimIn, second.AnimOut = anim("slide_up", 8), anim("fade", 7)
	masked := visual("masked", "motion", 3, 10, 30, 0.05, 0.1, 0.45, 0.45)
	masked.Mask = &Mask{Shape: MaskEllipse, X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Feather: 0.05}
	masked.AnimIn, masked.AnimOut = anim("pop", 10), anim("wipe", 9)
	blurred := visual("blurred", "motion", 5, 20, 28, 0.5, 0.05, 0.45, 0.45)
	blurred.AnimIn, blurred.AnimOut = anim("blur_in", 12), anim("slide_left", 8)
	ramped := visual("ramped", "motion", 8, 0, 26, 0.55, 0.5, 0.4, 0.4)
	ramped.Ramp = []RampPoint{{F: 0, Speed: 1, E: EaseInOut}, {F: 20, Speed: 2, E: EaseLinear}}
	ramped.AnimIn, ramped.AnimOut = anim("pop", 9), anim("fade", 6)
	still := visual("logo", "logo", 10, 0, 28, 0.05, 0.55, 0.35, 0.4)
	still.AnimIn, still.AnimOut = anim("blur_in", 9), anim("wipe", 10)

	for _, ntsc := range []bool{false, true} {
		project := Project{Version: CurrentVersion, Name: "Animation segments", Width: 1280, Height: 720, FPS: 25}
		if ntsc {
			project.FPS, project.NTSC = 30, true
		}
		rate := ProjectRate(project)
		for _, id := range []string{"motion", "bars", "logo"} {
			asset, err := Probe(ctx, ffmpeg, paths[id])
			if err != nil {
				t.Fatalf("Probe(%s): %v", id, err)
			}
			asset.ID = id
			project.Assets = append(project.Assets, AssetAtRate(asset, rate))
		}
		project.Tracks = []Track{
			{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{first, second}},
			{ID: "masked", Name: "Masked", Kind: TrackVideo, Clips: []Clip{masked}},
			{ID: "blur", Name: "Blur", Kind: TrackVideo, Clips: []Clip{blurred}},
			{ID: "ramp", Name: "Ramp", Kind: TrackVideo, Clips: []Clip{ramped}},
			{ID: "still", Name: "Still", Kind: TrackOverlay, Clips: []Clip{still}},
		}
		if err := Validate(project); err != nil {
			t.Fatalf("NTSC %t: animation project invalid: %v", ntsc, err)
		}
		// The transition [16, 22) moves the 7-frame cut at 21 to 22; the 13-frame plan ends in the one-frame segment [39, 40).
		for size, want := range map[int][]Segment{
			7:  {{0, 7}, {7, 14}, {14, 22}, {22, 29}, {29, 36}, {36, 40}},
			13: {{0, 13}, {13, 26}, {26, 39}, {39, 40}},
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

// Run with AURAGO_VIDEO_STUDIO_BENCHMARK=1 go test ./internal/videostudio -run '^TestAnimCostRecordOptIn$' -count=1 -v -timeout 45m.
//
// TestAnimCostRecordOptIn records what 2e animation and transitions add per 1080p frame (plan 2e Task 9, index §4.24),
// and checks the transitions and blur_in against the spike's absolute figures (2e review I-1, recordS4Basis).
// Everything except that last check is a record; the over-baseline figures below are not comparable with the spike.
//   - Animations: one 50-frame 1920x1080 clip on a second video track over a full-frame video.
//   - "inside" puts the preset's 25-frame in span on the measured frames 0–24.
//   - "outside" puts its 25-frame out span on frames 25–49, so the measured frames are at rest but the chain carries
//     the stage.
//   - Both are timed over the same 25 frames of the clip without the animation.
//   - blur_in also runs on a 1920x1080 still with alpha on an overlay track, over the same still at rest.
//   - Transitions: two 50-frame clips with a 25-frame transition on frames 25–49, over the same clips cut at frame 25.
//     There are 50 measured frames, and the difference is spread over the 25 transition frames.
//   - Each run is the fastest of three, at FFmpeg's default filter threads and at the export's threadLimit (4, as the
//     spike measured).
//
// The verdict line compares blur_in inside its span at threadLimit, over the no-animation render, with three times the
// spike's cascade: 28.1 ms per frame on FFmpeg 6 and later (measured on 8.1.1), or 37.8 ms on 5.1.9. It is a log line.
// The spike's figures are absolute, so the bound that counts is recordS4Basis (below), which fails the test.
func TestAnimCostRecordOptIn(t *testing.T) {
	if os.Getenv("AURAGO_VIDEO_STUDIO_BENCHMARK") != "1" {
		t.Skip("set AURAGO_VIDEO_STUDIO_BENCHMARK=1 to record the 2e animation and transition costs")
	}
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{"back": filepath.Join(root, "back.mp4"), "front": filepath.Join(root, "front.mp4"), "logo": filepath.Join(root, "logo.png")}
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["back"], "testsrc2=s=1920x1080:r=25:d=4")
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["front"], "color=c=0x3060a0:s=1920x1080:r=25:d=4[bg];testsrc2=s=960x540:r=25:d=4[fg];[bg][fg]overlay=x='480+100*t':y=270[out0]")
	writePicturePNG(t, paths["logo"], compositingStillPicture(1920, 1080))
	project := Project{Version: CurrentVersion, Name: "Animation cost", Width: 1920, Height: 1080, FPS: 25}
	rate := ProjectRate(project)
	for _, id := range []string{"back", "front", "logo"} {
		asset, err := Probe(ctx, ffmpeg, paths[id])
		if err != nil {
			t.Fatalf("Probe(%s): %v", id, err)
		}
		asset.ID = id
		project.Assets = append(project.Assets, AssetAtRate(asset, rate))
	}
	const clipFrames, spanFrames = 50, 25
	full := func(id, asset string, start, offset, duration int) Clip {
		return Clip{ID: id, AssetID: asset, Start: start, Offset: offset, Duration: duration, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}
	}
	variant := func(name string, front []Clip, still bool) Project {
		p := project
		p.Name = name
		kind, id := TrackVideo, "front"
		if still {
			kind, id = TrackOverlay, "still"
		}
		p.Tracks = []Track{
			{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{full("back", "back", 0, 0, clipFrames)}},
			{ID: id, Name: "Front", Kind: kind, Clips: front},
		}
		if err := Validate(p); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return p
	}
	animated := func(asset, in, out string) []Clip {
		c := full("front", asset, 0, 0, clipFrames)
		if in != "" {
			c.AnimIn = &ClipAnim{Preset: in, Duration: spanFrames}
		}
		if out != "" {
			c.AnimOut = &ClipAnim{Preset: out, Duration: spanFrames}
		}
		return []Clip{c}
	}
	type costCase struct {
		name, reference string // reference "" is a baseline
		project         Project
		frames, over    int // measured frames; frames the difference is spread over
	}
	cases := []costCase{
		{name: "clip at rest", project: variant("clip at rest", animated("front", "", ""), false), frames: spanFrames, over: spanFrames},
		{name: "still at rest", project: variant("still at rest", animated("logo", "", ""), true), frames: spanFrames, over: spanFrames},
	}
	for _, preset := range []string{"fade", "slide_up", "slide_down", "slide_left", "slide_right", "pop", "wipe", "blur_in"} {
		cases = append(cases,
			costCase{preset + " inside the span", "clip at rest", variant(preset+" in", animated("front", preset, ""), false), spanFrames, spanFrames},
			costCase{preset + " outside the span", "clip at rest", variant(preset+" out", animated("front", "", preset), false), spanFrames, spanFrames})
	}
	cases = append(cases,
		costCase{"blur_in on a still, inside the span", "still at rest", variant("still blur in", animated("logo", "blur_in", ""), true), spanFrames, spanFrames},
		costCase{"blur_in on a still, outside the span", "still at rest", variant("still blur out", animated("logo", "", "blur_in"), true), spanFrames, spanFrames},
		costCase{name: "two clips cut at frame 25", project: variant("cut", []Clip{full("a", "front", 0, 0, spanFrames), full("b", "front", spanFrames, 0, clipFrames)}, false), frames: clipFrames, over: spanFrames})
	for _, kind := range []TransitionType{TransitionDissolve, "circleopen", "radial", "pixelize", "slideleft", "zoomin"} {
		outgoing := full("a", "front", 0, 0, clipFrames)
		outgoing.Transition = &Transition{Type: kind, Duration: spanFrames}
		cases = append(cases, costCase{"transition " + string(kind), "two clips cut at frame 25",
			variant(string(kind), []Clip{outgoing, full("b", "front", spanFrames, 0, clipFrames)}, false), clipFrames, spanFrames})
	}
	major := ffmpegMajor(t, ffmpeg)
	spike := 28.1 // ms per 1080p frame for the blur_in cascade, FFmpeg 8.1.1 at 4 threads (spike S5)
	if major > 0 && major < 6 {
		spike = 37.8 // FFmpeg 5.1.9
	}
	blurInside := 0.0
	for _, threads := range []int{0, threadLimit} {
		label := "default threads"
		if threads > 0 {
			label = fmt.Sprintf("%d threads (export)", threads)
		}
		took := map[string]time.Duration{}
		for _, c := range cases {
			took[c.name] = compositingCost(t, ctx, ffmpeg, c.project, paths, c.frames, threads)
			if c.reference == "" {
				t.Logf("FFmpeg %d, %s: %s %d ms for %d frames", major, label, c.name, took[c.name].Milliseconds(), c.frames)
				continue
			}
			perFrame := float64(took[c.name]-took[c.reference]) / float64(time.Millisecond) / float64(c.over)
			t.Logf("FFmpeg %d, %s: %s %+.1f ms per frame over %s (%d ms for %d frames)", major, label, c.name, perFrame, c.reference, took[c.name].Milliseconds(), c.frames)
			if threads == threadLimit && c.name == "blur_in inside the span" {
				blurInside = perFrame
			}
		}
	}
	if bound := 3 * spike; blurInside > bound {
		t.Logf("ABOVE THE BOUND: blur_in inside its span costs %.1f ms per frame at %d threads, more than 3 x %.1f ms = %.1f ms; stop and report", blurInside, threadLimit, spike, bound)
	} else {
		t.Logf("within the bound: blur_in inside its span costs %.1f ms per frame at %d threads (bound 3 x %.1f ms = %.1f ms)", blurInside, threadLimit, spike, bound)
	}
	recordS4Basis(t, ctx, ffmpeg, root, major)
}

// s4Spike are the spike's S4 and S5 figures (scratchpad p2espike sec_S4.sh and sec_S5.sh): wall ms per 1080p frame,
// 100 frames, 4 filter threads, fastest of 3, process start and the lavfi sources included, FFmpeg 8.1.1 | 5.1.9. The
// transitions ran as xfade on gbrap with every frame inside the transition (duration 4 s); blur_in is the cascade of two
// maskedmerge stages and two gblurs (S5, only during the animation). The spike's baseline for the transitions was
// 4.0 | 8.7 ms (a plain blend of the two sources).
var s4Spike = []struct {
	name        string
	spike       [2]float64 // FFmpeg 6 and later (measured on 8.1.1), 5.1.9
	transition  TransitionType
	spikeFilter string // what the spike timed
}{
	{"dissolve", [2]float64{4.1, 10.4}, TransitionDissolve, "xfade fade"},
	{"circleopen", [2]float64{13.1, 15.7}, "circleopen", "xfade circleopen"},
	{"radial", [2]float64{14.8, 23.6}, "radial", "xfade radial"},
	{"pixelize", [2]float64{10.0, 17.4}, "pixelize", "xfade pixelize"},
	{"slideleft", [2]float64{6.9, 15.7}, "slideleft", "xfade slideleft"},
	{"zoomin", [2]float64{10.1, 18.8}, "zoomin", "xfade zoomin"},
	{"blur_in", [2]float64{28.1, 37.8}, "", "blur_in cascade"},
}

// recordS4Basis measures the six transitions of 2e that the spike timed (dissolve, circleopen, radial, pixelize,
// slideleft, zoomin) and blur_in on the spike's own basis (2e review I-1; index §4.24 "within 3x the spike figures"), and
// fails when an entry costs more than three times its spike figure.
//   - Basis: absolute wall milliseconds per output frame, 1920x1080, 4 filter threads (the export's threadLimit), the
//     fastest of five runs, process start included, over the frames of the transition or animation alone. This is the
//     spike's S4 basis; the 25-frame over-baseline figures above are not.
//   - A transition entry is one video track with two 150-frame clips overlapping by 100 frames (4 s, as the spike's
//     xfade duration), timed over exactly those 100 frames. blur_in is one 150-frame clip with a 75-frame in-animation (the
//     longest, 3 s), timed over those 75 frames.
//   - The export path decodes its H.264 sources and composes through RGBA, which the spike's lavfi graphs do not: the
//     "floor" is the same frames of one clip at rest on that path, logged beside each entry so the share of the pipeline
//     is visible. The bound applies to the entry, not to its excess over the floor.
func recordS4Basis(t *testing.T, ctx context.Context, ffmpeg, root string, major int) {
	t.Helper()
	const clipFrames, transitionFrames, animFrames, runs = 150, 100, 75, 5
	column := 0
	if major > 0 && major < 6 {
		column = 1
	}
	paths := map[string]string{"s4a": filepath.Join(root, "s4a.mp4"), "s4b": filepath.Join(root, "s4b.mp4")}
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["s4a"], "testsrc2=s=1920x1080:r=25:d=6")
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["s4b"], "rgbtestsrc=s=1920x1080:r=25:d=6")
	base := Project{Version: CurrentVersion, Name: "S4 basis", Width: 1920, Height: 1080, FPS: 25}
	rate := ProjectRate(base)
	for _, id := range []string{"s4a", "s4b"} {
		asset, err := Probe(ctx, ffmpeg, paths[id])
		if err != nil {
			t.Fatalf("Probe(%s): %v", id, err)
		}
		asset.ID = id
		base.Assets = append(base.Assets, AssetAtRate(asset, rate))
	}
	clip := func(id, asset string, start int) Clip {
		return Clip{ID: id, AssetID: asset, Start: start, Duration: clipFrames, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}
	}
	build := func(name string, clips ...Clip) Project {
		p := base
		p.Name = name
		p.Tracks = []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: clips}}
		if err := Validate(p); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return p
	}
	cost := func(p Project, seg Segment) float64 {
		return float64(segmentCost(t, ctx, ffmpeg, p, paths, seg, threadLimit, runs)) / float64(time.Millisecond) / float64(seg.End-seg.Start)
	}
	// One clip at rest: the export path's floor, over the same number of frames.
	rest := build("S4 floor", clip("a", "s4a", 0))
	transitionFloor := cost(rest, Segment{Start: clipFrames - transitionFrames, End: clipFrames})
	animFloor := cost(rest, Segment{Start: 0, End: animFrames})
	for _, entry := range s4Spike {
		var value, floor float64
		var frames int
		if entry.transition != "" {
			// a plays [0, 150) and b [50, 200); the transition covers [50, 150).
			a, b := clip("a", "s4a", 0), clip("b", "s4b", clipFrames-transitionFrames)
			a.Transition = &Transition{Type: entry.transition, Duration: transitionFrames}
			frames, floor = transitionFrames, transitionFloor
			value = cost(build("S4 "+entry.name, a, b), Segment{Start: clipFrames - transitionFrames, End: clipFrames})
		} else {
			a := clip("a", "s4a", 0)
			a.AnimIn = &ClipAnim{Preset: entry.name, Duration: animFrames}
			frames, floor = animFrames, animFloor
			value = cost(build("S4 "+entry.name, a), Segment{Start: 0, End: animFrames})
		}
		spike := entry.spike[column]
		ratio := value / spike
		t.Logf("FFmpeg %d, S4 basis (absolute wall ms per output frame, 1920x1080, %d filter threads, fastest of %d, over %d frames): %s %.1f ms; spike %s %.1f ms; ratio %.2f (bound 3); floor, one clip at rest over the same frames, %.1f ms",
			major, threadLimit, runs, frames, entry.name, value, entry.spikeFilter, spike, ratio, floor)
		if ratio > 3 {
			t.Errorf("S4 basis: %s costs %.1f ms per frame, %.2f x the spike's %.1f ms (%s) on FFmpeg %d; the bound is 3 x", entry.name, value, ratio, spike, entry.spikeFilter, major)
		}
	}
}
