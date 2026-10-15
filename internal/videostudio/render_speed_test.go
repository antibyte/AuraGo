package videostudio

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// makeFrameCounterFixture writes a lossless video whose frame N has luminance N mod 256 and a 440 Hz tone, so
// every decoded frame names its own source index.
func makeFrameCounterFixture(t *testing.T, ctx context.Context, ffmpeg, output string, seconds int) {
	t.Helper()
	video := fmt.Sprintf("color=c=black:s=64x32:r=30:d=%d,format=yuv420p,geq=lum='mod(N\\,256)':cb=128:cr=128", seconds)
	tone := fmt.Sprintf("sine=frequency=440:sample_rate=48000:duration=%d", seconds)
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", video, "-f", "lavfi", "-i", tone,
		"-map", "0:v:0", "-map", "1:a:0", "-c:v", "libx264", "-preset", "ultrafast", "-qp", "0", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "96k", "-f", "mp4", output}
	if out, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("frame counter fixture: %v: %s", err, out)
	}
}

// renderTestProject renders project to a new file in root and returns the output path.
func renderTestProject(t *testing.T, ctx context.Context, ffmpeg, root, name string, paths map[string]string, project Project) string {
	t.Helper()
	output := filepath.Join(root, name+".mp4")
	if err := RenderWithOptions(ctx, RenderOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Output: output, Encoder: Encoder{Lossless: true}}); err != nil {
		t.Fatalf("render %s: %v", name, err)
	}
	return output
}

func TestRenderSpeedMatchesFrameMapping(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	root := t.TempDir()
	source := filepath.Join(root, "counter.mp4")
	makeFrameCounterFixture(t, ctx, ffmpeg, source, 4)
	asset, err := Probe(ctx, ffmpeg, source)
	if err != nil {
		t.Fatal(err)
	}
	asset.ID = "counter"
	paths := map[string]string{"counter": source}
	clipProject := func(duration int, speed float64) Project {
		return Project{Version: CurrentVersion, Name: "Speed", Width: 720, Height: 720, FPS: 30, Assets: []Asset{asset},
			Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{{ID: "c", AssetID: "counter", Duration: duration,
				Speed: speed, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}}}}}
	}
	normal := frameHashes(t, ctx, ffmpeg, renderTestProject(t, ctx, ffmpeg, root, "normal", paths, clipProject(60, 0)))
	fast := frameHashes(t, ctx, ffmpeg, renderTestProject(t, ctx, ffmpeg, root, "fast", paths, clipProject(30, 2)))
	if len(normal) < 60 || len(fast) < 30 {
		t.Fatalf("frame counts normal=%d fast=%d", len(normal), len(fast))
	}
	// Output frame n of the 2x clip shows source frame 2n, the frame the 1x clip shows at 2n.
	for n := 0; n < 30; n++ {
		if fast[n] != normal[2*n] {
			t.Fatalf("2x output frame %d differs from source frame %d", n, 2*n)
		}
	}
}

// A clip at 1.5x enters later segments at fractional source positions. Every segmentation must sample the single
// pass's source frames, and output frame n shows source frame floor(offset + 1.5n), the frame the 1x clip shows there.
func TestRenderSpeedSegmentationParity(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	root := t.TempDir()
	source := filepath.Join(root, "counter.mp4")
	makeFrameCounterFixture(t, ctx, ffmpeg, source, 4)
	asset, err := Probe(ctx, ffmpeg, source)
	if err != nil {
		t.Fatal(err)
	}
	asset.ID = "counter"
	paths := map[string]string{"counter": source}
	project := func(offset, duration int, speed float64) Project {
		return Project{Version: CurrentVersion, Name: "Parity", Width: 720, Height: 720, FPS: 30, Assets: []Asset{asset},
			Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{{ID: "c", AssetID: "counter", Offset: offset,
				Duration: duration, Speed: speed, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}}}}}
	}
	render := func(name string, segmentFrames int, project Project) []string {
		output := filepath.Join(root, name+".mp4")
		if err := RenderWithOptions(ctx, RenderOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Output: output,
			SegmentFrames: segmentFrames, Encoder: Encoder{Lossless: true}}); err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		return frameHashes(t, ctx, ffmpeg, output)
	}
	normal := render("normal", 0, project(0, 90, 0))
	single := render("single", 0, project(7, 40, 1.5))
	if len(normal) < 90 || len(single) < 40 {
		t.Fatalf("frame counts normal=%d single=%d", len(normal), len(single))
	}
	for n := 0; n < 40; n++ {
		if want := 7 + 3*n/2; single[n] != normal[want] {
			t.Fatalf("1.5x output frame %d differs from source frame %d", n, want)
		}
	}
	for _, segmentFrames := range []int{7, 13} {
		if got := render(fmt.Sprintf("segments-%d", segmentFrames), segmentFrames, project(7, 40, 1.5)); !reflect.DeepEqual(got, single) {
			t.Fatalf("%d-frame segments differ from the single pass", segmentFrames)
		}
	}
}

func TestRenderDisabledAndDetachedClips(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	root := t.TempDir()
	source := filepath.Join(root, "counter.mp4")
	makeFrameCounterFixture(t, ctx, ffmpeg, source, 2)
	asset, err := Probe(ctx, ffmpeg, source)
	if err != nil {
		t.Fatal(err)
	}
	asset.ID = "counter"
	paths := map[string]string{"counter": source}
	clip := func(id string, start int, mutate func(*Clip)) Clip {
		c := Clip{ID: id, AssetID: "counter", Start: start, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}
		if mutate != nil {
			mutate(&c)
		}
		return c
	}
	project := func(clips ...Clip) Project {
		return Project{Version: CurrentVersion, Name: "Flags", Width: 720, Height: 720, FPS: 30, Assets: []Asset{asset},
			Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: clips}}}
	}
	// A disabled clip contributes nothing: the output matches a timeline without it.
	withDisabled := frameHashes(t, ctx, ffmpeg, renderTestProject(t, ctx, ffmpeg, root, "disabled", paths,
		project(clip("off", 0, func(c *Clip) { c.Disabled = true }), clip("on", 30, nil))))
	without := frameHashes(t, ctx, ffmpeg, renderTestProject(t, ctx, ffmpeg, root, "without", paths, project(clip("on", 30, nil))))
	if !reflect.DeepEqual(withDisabled, without) {
		t.Fatalf("a disabled clip changed the output: %d and %d frames", len(withDisabled), len(without))
	}
	// Detached audio is not mixed from the video clip; without the flag the video clip carries its sound.
	detached, err := Probe(ctx, ffmpeg, renderTestProject(t, ctx, ffmpeg, root, "detached", paths,
		project(clip("c", 0, func(c *Clip) { c.AudioDetached = true }))))
	if err != nil {
		t.Fatal(err)
	}
	if detached.HasAudio {
		t.Fatal("detached audio was still mixed from the video clip")
	}
	control, err := Probe(ctx, ffmpeg, renderTestProject(t, ctx, ffmpeg, root, "attached", paths, project(clip("c", 0, nil))))
	if err != nil {
		t.Fatal(err)
	}
	if !control.HasAudio {
		t.Fatal("the video clip without detaching lost its sound")
	}
}

func TestRenderImageOnVideoTrackWithTransition(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := t.TempDir()
	source := filepath.Join(root, "counter.mp4")
	picture := filepath.Join(root, "picture.png")
	makeFrameCounterFixture(t, ctx, ffmpeg, source, 2)
	makeImageFixture(t, ctx, ffmpeg, picture)
	video, err := Probe(ctx, ffmpeg, source)
	if err != nil {
		t.Fatal(err)
	}
	still, err := Probe(ctx, ffmpeg, picture)
	if err != nil {
		t.Fatal(err)
	}
	video.ID, still.ID = "counter", "picture"
	project := Project{Version: CurrentVersion, Name: "Image", Width: 720, Height: 720, FPS: 30, Assets: []Asset{video, still},
		Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{
			{ID: "picture", AssetID: "picture", Start: 0, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain,
				Transition: &Transition{Type: TransitionDissolve, Duration: 10}},
			{ID: "motion", AssetID: "counter", Start: 20, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain},
		}}}}
	if err := Validate(project); err != nil {
		t.Fatalf("image on a video track is invalid: %v", err)
	}
	output := renderTestProject(t, ctx, ffmpeg, root, "image", map[string]string{"counter": source, "picture": picture}, project)
	got, err := Probe(ctx, ffmpeg, output)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != AssetVideo || got.DurationFrames < 49 || got.DurationFrames > 51 {
		t.Fatalf("output = %s with %d frames, want about 50 frames of video", got.Kind, got.DurationFrames)
	}
}

func TestTempoChainCoversSpeedRange(t *testing.T) {
	for _, speed := range []float64{0.25, 0.5, 0.75, 1, 1.5, 2, 3, 4, 8, 16} {
		factor := 1.0
		for _, part := range splitTempo(tempoChain(speed)) {
			var value float64
			if _, err := fmt.Sscanf(part, "atempo=%f", &value); err != nil || value < 0.5 || value > 2 {
				t.Fatalf("speed %v: bad atempo part %q", speed, part)
			}
			factor *= value
		}
		if d := factor - speed; d > 1e-5 || d < -1e-5 {
			t.Fatalf("speed %v: atempo chain multiplies to %v", speed, factor)
		}
	}
	if tempoChain(1) != "" {
		t.Fatal("speed 1 must add no audio filter")
	}
}

func splitTempo(chain string) []string {
	if chain == "" {
		return nil
	}
	return strings.Split(chain, ",")
}
