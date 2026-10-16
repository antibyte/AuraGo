package videostudio

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRampInverseExprText(t *testing.T) {
	// One constant-speed piece (speed 2 over clip frames 0..10): the tree picks the extension below the piece, the piece,
	// or the extension above S(10) = 20.
	pieces := []rampPiece{{t0: 0, t1: 10, s0: 0, a: 2, b: 0}}
	want := "if(lt(ld(0),0)," +
		"(0+((ld(0)-0)/2))," + // below the first piece: t0 + (u − s0)/a
		"if(lt(ld(0),20)," +
		"(st(1,(ld(0)-0));(0+((2*ld(1))/(2+sqrt((4+(0*ld(1))))))))," + // the piece: t0 + 2d/(a + sqrt(a² + 2b·d))
		"(10+((ld(0)-20)/2))))" // above its end: t1 + (u − S(t1))/v1
	if got := rampInverseExpr(pieces, exprLd(0)).String(); got != want {
		t.Fatalf("inverse:\n got %s\nwant %s", got, want)
	}
	for _, c := range []struct{ u, t float64 }{{-4, -2}, {0, 0}, {7, 3.5}, {20, 10}, {26, 13}} {
		if got := exprOp(";", exprSt(0, exprNum(c.u)), rampInverseExpr(pieces, exprLd(0))).eval(nil); math.Abs(got-c.t) > 1e-12 {
			t.Errorf("Sinv(%v) = %v, want %v", c.u, got, c.t)
		}
	}
}

// TestRampResampleSnapsToFloorS replays rampResample's setpts in Go on exact source timestamps. Source frame m first
// shows at clip frame ceil(Sinv((m·Den/Num − 0.001)·Num/Den − offset)), and clip frame n then shows the largest such m:
// floor(offset + S(n) + 0.001·Num/Den), i.e. floor(offset + S(n)) with videoResample's 1 ms tolerance.
func TestRampResampleSnapsToFloorS(t *testing.T) {
	ramps := map[string][]RampPoint{
		"accelerate": {{F: 0, Speed: 1, E: EaseLinear}, {F: 30, Speed: 3, E: EaseLinear}},
		"decelerate": {{F: 0, Speed: 3, E: EaseLinear}, {F: 30, Speed: 1, E: EaseLinear}},
		"eased":      {{F: 5, Speed: 1, E: EaseInOut}, {F: 35, Speed: 2.5, E: EaseLinear}},
		"multi":      {{F: 0, Speed: 2, E: EaseLinear}, {F: 10, Speed: 0.5, E: EaseHold}, {F: 20, Speed: 1.5, E: EaseIn}, {F: 30, Speed: 4, E: EaseLinear}},
		"outside":    {{F: -10, Speed: 0.5, E: EaseLinear}, {F: 100, Speed: 4, E: EaseOut}},
	}
	const duration = 40
	for name, points := range ramps {
		pieces := rampPieces(points, 0, duration)
		inverse := rampInverseExpr(pieces, exprLd(0))
		for _, rate := range []Rate{{Num: 25, Den: 1}, {Num: 30000, Den: 1001}, {Num: 60, Den: 1}} {
			tolerance := 0.001 * float64(rate.Num) / float64(rate.Den)
			for _, offset := range []int{0, 13} {
				low, high := offset-40, offset+int(math.Ceil(rampSource(pieces, duration)))+40
				shows := make(map[int]float64, high-low+1)
				previous := math.Inf(-1)
				for m := low; m <= high; m++ {
					seconds := float64(m) * float64(rate.Den) / float64(rate.Num)
					u := (seconds-0.001)*float64(rate.Num)/float64(rate.Den) - float64(offset)
					shows[m] = math.Ceil(exprOp(";", exprSt(0, exprNum(u)), inverse).eval(nil))
					if math.IsNaN(shows[m]) || shows[m] < previous {
						t.Fatalf("%s at %s, offset %d: source frame %d first shows at clip frame %v, after %v", name, rate, offset, m, shows[m], previous)
					}
					previous = shows[m]
				}
				for n := 0; n < duration; n++ {
					got := low - 1
					for m := low; m <= high; m++ {
						if shows[m] <= float64(n) {
							got = m
						}
					}
					want := int(math.Floor(float64(offset) + rampSource(pieces, float64(n)) + tolerance))
					if got != want {
						t.Errorf("%s at %s, offset %d: clip frame %d shows source frame %d, want floor(offset + S(n)) = %d", name, rate, offset, n, got, want)
					}
				}
			}
		}
	}
}

// A ramped clip that enters a window late starts at offset + S(entered) and reads S(entered + frames) − S(entered)
// source frames; the fps frame of its first window frame stays offset + entered.
func TestSegmentLayersSeekRampedClipsByS(t *testing.T) {
	ramp := []RampPoint{{F: 0, Speed: 1, E: EaseLinear}, {F: 30, Speed: 3, E: EaseLinear}}
	project := Project{Version: CurrentVersion, Name: "Seek", Width: 1280, Height: 720, FPS: 25,
		Assets: []Asset{{ID: "v", Name: "v.mkv", Kind: AssetVideo, DurationFrames: 250, Width: 32, Height: 32}},
		Tracks: []Track{{ID: "t", Name: "Video", Kind: TrackVideo, Clips: []Clip{
			{ID: "r", AssetID: "v", Offset: 5, Duration: 40, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Ramp: ramp},
		}}}}
	rate := ProjectRate(project)
	sources := map[string]renderSource{"v": {asset: project.Assets[0], path: "v.mkv", videoEnd: 10}}
	layers, inputs, err := segmentLayers(project, Segment{Start: 20, End: 40}, sources, rate)
	if err != nil || len(layers) != 1 || len(inputs) != 1 {
		t.Fatalf("layers %+v, inputs %+v, %v", layers, inputs, err)
	}
	pieces := rampPieces(ramp, 0, 40)
	item, input := layers[0][0], inputs[0]
	from := 5 + rampSource(pieces, 20)
	seekFrame := int(math.Floor(from)) - 25
	limit := rate.SecondsFloat(from+rampSource(pieces, 40)-rampSource(pieces, 20)-float64(seekFrame)) + 1
	if item.from != from || item.first != 25 || math.Abs(input.seek-rate.Seconds(seekFrame)) > 1e-12 || math.Abs(input.limit-limit) > 1e-9 {
		t.Fatalf("from %v first %d seek %v limit %v; want %v, 25, %v, %v", item.from, item.first, input.seek, input.limit, from, rate.Seconds(seekFrame), limit)
	}
}

// makeIndexFixture writes the spike's index fixture and probes it: a lossless 25 fps RGB video (FFV1 in Matroska, whose
// millisecond timestamps are exact at 25 fps) of 250 frames whose frame N is grey level N, so a decoded pixel names
// its source frame.
func makeIndexFixture(t *testing.T, ctx context.Context, ffmpeg, output string) Asset {
	t.Helper()
	source := "nullsrc=s=32x32:r=25:d=10,format=gbrp,geq=r='min(N,255)':g='min(N,255)':b='min(N,255)'"
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", source, "-c:v", "ffv1", "-pix_fmt", "gbrp", "-f", "matroska", output}
	if out, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("index fixture: %v: %s", err, out)
	}
	asset, err := Probe(ctx, ffmpeg, output)
	if err != nil {
		t.Fatalf("Probe(index fixture): %v", err)
	}
	asset.ID = "index"
	return asset
}

// rampIndexProject places the index ramps of TestRampResampleSnapsToFloorS on one 720x720 video track, 40 frames each:
//   - accelerating, decelerating, eased (subdivided), and multi-piece with hold and ease_in;
//   - with offsets 0, 10, 20, 0.
//
// parity overlaps the first two clips with a dissolve (frames 30–40) and adds a fifth clip, ramped and keyframed, with
// cover.
func rampIndexProject(t *testing.T, asset Asset, ntsc, parity bool) Project {
	t.Helper()
	project := Project{Version: CurrentVersion, Name: "Ramps", Width: 720, Height: 720, FPS: 25}
	if ntsc {
		project.FPS, project.NTSC = 30, true
	}
	project.Assets = []Asset{AssetAtRate(asset, ProjectRate(project))}
	clip := func(id string, start, offset int, ramp ...RampPoint) Clip {
		return Clip{ID: id, AssetID: "index", Start: start, Offset: offset, Duration: 40, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Ramp: ramp}
	}
	starts := []int{0, 40, 80, 120}
	if parity {
		starts = []int{0, 30, 70, 110}
	}
	clips := []Clip{
		clip("accelerate", starts[0], 0, RampPoint{F: 0, Speed: 1, E: EaseLinear}, RampPoint{F: 30, Speed: 3, E: EaseLinear}),
		clip("decelerate", starts[1], 10, RampPoint{F: 0, Speed: 3, E: EaseLinear}, RampPoint{F: 30, Speed: 1, E: EaseLinear}),
		clip("eased", starts[2], 20, RampPoint{F: 5, Speed: 1, E: EaseInOut}, RampPoint{F: 35, Speed: 2.5, E: EaseLinear}),
		clip("multi", starts[3], 0, RampPoint{F: 0, Speed: 2, E: EaseLinear}, RampPoint{F: 10, Speed: 0.5, E: EaseHold}, RampPoint{F: 20, Speed: 1.5, E: EaseIn}, RampPoint{F: 30, Speed: 4, E: EaseLinear}),
	}
	if parity {
		clips[0].Transition = &Transition{Type: TransitionDissolve, Duration: 10}
		keyed := clip("keyed", 150, 5, RampPoint{F: 0, Speed: 0.5, E: EaseOut}, RampPoint{F: 30, Speed: 2, E: EaseLinear})
		keyed.Fit = FitCover
		keyed.Keyframes = map[string][]Keyframe{"x": {{F: 5, V: 0, E: EaseInOut}, {F: 40, V: 0.4, E: EaseLinear}}, "width": {{F: 5, V: 0.6, E: EaseLinear}}}
		clips = append(clips, keyed)
	}
	project.Tracks = []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: clips}}
	if err := Validate(project); err != nil {
		t.Fatalf("ramp project invalid: %v", err)
	}
	return project
}

// Every clip frame n of every index ramp shows source frame floor(offset + S(n)) (with the 1 ms tolerance). A window
// that enters clips late (frames 57–100) renders the same frames.
func TestRenderRampMatchesFloorS(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{"index": filepath.Join(root, "index.mkv")}
	project := rampIndexProject(t, makeIndexFixture(t, ctx, ffmpeg, paths["index"]), false, false)
	rate := ProjectRate(project)
	frames := renderRawRGB(t, ctx, ffmpeg, project, paths, Segment{Start: 0, End: 160})
	tolerance := 0.001 * float64(rate.Num) / float64(rate.Den)
	centre := (360*720 + 360) * 3
	for _, clip := range project.Tracks[0].Clips {
		pieces := rampPieces(clip.Ramp, 0, float64(clip.Duration))
		for n := 0; n < clip.Duration; n++ {
			want := int(math.Floor(float64(clip.Offset) + rampSource(pieces, float64(n)) + tolerance))
			if got := int(frames[clip.Start+n][centre]); got != want {
				t.Errorf("%s: clip frame %d shows source frame %d, want floor(offset + S(n)) = %d", clip.ID, n, got, want)
			}
		}
	}
	part := renderRawRGB(t, ctx, ffmpeg, project, paths, Segment{Start: 57, End: 100})
	for i, frame := range part {
		if !bytes.Equal(frame, frames[57+i]) {
			t.Fatalf("frame %d differs when its window starts at frame 57", 57+i)
		}
	}
}

// Ramps (with a dissolve and a ramped, keyframed cover clip) render the same frames in one segment and in 7- and
// 13-frame segments, at 25 and 30000/1001 fps.
func TestSegmentedRenderRamp(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{"index": filepath.Join(root, "index.mkv")}
	asset := makeIndexFixture(t, ctx, ffmpeg, paths["index"])
	for _, ntsc := range []bool{false, true} {
		project := rampIndexProject(t, asset, ntsc, true)
		render := func(name string, segmentFrames int) []string {
			output := filepath.Join(root, fmt.Sprintf("%s-%t.mp4", name, ntsc))
			if err := RenderWithOptions(ctx, RenderOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Output: output,
				SegmentFrames: segmentFrames, Encoder: Encoder{Lossless: true}}); err != nil {
				t.Fatalf("render %s (NTSC %t): %v", name, ntsc, err)
			}
			return frameHashes(t, ctx, ffmpeg, output)
		}
		single := render("single", 100000)
		if len(single) != 190 {
			t.Fatalf("NTSC %t: %d frames, want 190", ntsc, len(single))
		}
		for _, size := range []int{7, 13} {
			if got := render(fmt.Sprintf("segments-%d", size), size); !reflect.DeepEqual(got, single) {
				t.Fatalf("NTSC %t: %d-frame segments differ from the single pass", ntsc, size)
			}
		}
	}
}

// The linked audio of a ramped video-track clip leaves the mix; its detached partner on an audio track still sounds.
func TestRampMutesLinkedAudio(t *testing.T) {
	asset := Asset{ID: "v", Name: "v.mp4", Kind: AssetVideo, DurationFrames: 300, DurationUs: 10_000_000, Width: 64, Height: 36, HasAudio: true}
	ramped := Clip{ID: "r", AssetID: "v", Duration: 60, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Link: "l",
		Ramp: []RampPoint{{F: 0, Speed: 1, E: EaseLinear}, {F: 30, Speed: 2, E: EaseLinear}}}
	partner := Clip{ID: "p", AssetID: "v", Duration: 60, Volume: 1, Link: "l"}
	video := Track{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{ramped}}
	audio := Track{ID: "audio", Name: "Audio", Kind: TrackAudio, Clips: []Clip{partner}}
	if audibleClip(video, ramped, asset) {
		t.Fatal("the linked audio of a ramped video clip still sounds")
	}
	plain := ramped
	plain.Ramp = nil
	if !audibleClip(video, plain, asset) || !audibleClip(audio, partner, asset) {
		t.Fatal("unramped and detached audio must still sound")
	}
	project := Project{Version: CurrentVersion, Name: "Ramp audio", Width: 1280, Height: 720, FPS: 30, Assets: []Asset{asset}, Tracks: []Track{video}}
	sources := map[string]renderSource{"v": {asset: asset, path: "v.mp4"}}
	rate := ProjectRate(project)
	if rangeHasAudio(project, sources, 0, 60) {
		t.Fatal("a timeline with only a ramped video clip reports audio")
	}
	if inputs, graph, err := buildAudioSegment(project, Segment{Start: 0, End: 60}, sources, rate); err != nil || len(inputs) != 0 || !strings.Contains(graph, "anullsrc") {
		t.Fatalf("ramped clip audio: %d inputs, %v:\n%s", len(inputs), err, graph)
	}
	project.Tracks = append(project.Tracks, audio)
	if inputs, _, err := buildAudioSegment(project, Segment{Start: 0, End: 60}, sources, rate); err != nil || len(inputs) != 1 || inputs[0].path != "v.mp4" {
		t.Fatalf("the detached partner must stay in the mix: %+v, %v", inputs, err)
	}
}
