package videostudio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func requireFFmpeg(t *testing.T) string {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	return ffmpeg
}

func makeLavfiVideoFixture(t *testing.T, ctx context.Context, ffmpeg, output, source string) {
	t.Helper()
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", source,
		"-an", "-c:v", "libx264", "-preset", "ultrafast", "-crf", "10", "-pix_fmt", "yuv420p", "-f", "mp4", output}
	if out, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("create fixture %s: %v: %s", source, err, out)
	}
}

func frameHashes(t *testing.T, ctx context.Context, ffmpeg, input string) []string {
	t.Helper()
	out, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostdin", "-loglevel", "error", "-i", input, "-map", "0:v:0", "-f", "framemd5", "pipe:1").Output()
	if err != nil {
		t.Fatalf("framemd5 %s: %v", input, err)
	}
	var hashes []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ",")
		hashes = append(hashes, strings.TrimSpace(fields[len(fields)-1]))
	}
	return hashes
}

func decodedSamples(t *testing.T, ctx context.Context, ffmpeg, input string) []int16 {
	t.Helper()
	out, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostdin", "-loglevel", "error", "-i", input, "-map", "0:a:0", "-f", "s16le", "-ac", "2", "-ar", "48000", "pipe:1").Output()
	if err != nil {
		t.Fatalf("decode audio %s: %v", input, err)
	}
	samples := make([]int16, len(out)/2)
	for i := range samples {
		samples[i] = int16(binary.LittleEndian.Uint16(out[i*2:]))
	}
	return samples
}

// segmentFixtureProject has a dissolve (60–70), a wipe (108–120), a translucent overlay and music with
// fades (fade-in 5–30, fade-out 110–145), so 13-frame segments hit every boundary rule.
func segmentFixtureProject(t *testing.T, ctx context.Context, ffmpeg, root string) (Project, map[string]string) {
	t.Helper()
	paths := map[string]string{"red": filepath.Join(root, "motion.mp4"), "blue": filepath.Join(root, "blue.mp4"), "tone": filepath.Join(root, "tone.wav"), "cover": filepath.Join(root, "green.png")}
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["red"], "testsrc2=s=64x32:r=30:d=3")
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["blue"], "color=c=blue:s=64x32:r=30:d=3")
	toneArgs := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=6", "-c:a", "pcm_s16le", "-f", "wav", paths["tone"]}
	if out, err := exec.CommandContext(ctx, ffmpeg, toneArgs...).CombinedOutput(); err != nil {
		t.Fatalf("tone fixture: %v: %s", err, out)
	}
	makeImageFixture(t, ctx, ffmpeg, paths["cover"])
	var assets []Asset
	for _, id := range []string{"red", "blue", "tone", "cover"} {
		asset, err := Probe(ctx, ffmpeg, paths[id])
		if err != nil {
			t.Fatalf("Probe(%s): %v", id, err)
		}
		asset.ID = id
		assets = append(assets, asset)
	}
	clip := func(id, asset string, start, offset, duration int) Clip {
		return Clip{ID: id, AssetID: asset, Start: start, Offset: offset, Duration: duration, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}
	}
	a, b, c := clip("a", "red", 0, 0, 70), clip("b", "blue", 60, 15, 60), clip("c", "red", 108, 20, 50)
	a.Transition = &Transition{Type: TransitionDissolve, Duration: 10}
	b.Transition = &Transition{Type: TransitionWipeLeft, Duration: 12}
	project := Project{Version: CurrentVersion, Name: "Segments", Width: 720, Height: 720, FPS: 30, Assets: assets,
		Tracks: []Track{
			{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{a, b, c}},
			{ID: "overlay", Name: "Overlay", Kind: TrackOverlay, Clips: []Clip{{ID: "logo", AssetID: "cover", Start: 30, Duration: 90, X: 0.7, Y: 0.1, Width: 0.2, Height: 0.2, Opacity: 0.8, Volume: 1, Fit: FitContain}}},
			{ID: "music", Name: "Music", Kind: TrackAudio, Clips: []Clip{{ID: "music", AssetID: "tone", Start: 5, Offset: 30, Duration: 140, Volume: 0.8, FadeIn: 25, FadeOut: 35}}},
		}}
	if err := Validate(project); err != nil {
		t.Fatalf("fixture project invalid: %v", err)
	}
	return project, paths
}

func TestSegmentedRenderMatchesSinglePass(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := t.TempDir()
	project, paths := segmentFixtureProject(t, ctx, ffmpeg, root)
	render := func(name string, opts RenderOptions) string {
		output := filepath.Join(root, name)
		opts.FFmpegPath, opts.Project, opts.AssetPaths, opts.Output, opts.Encoder = ffmpeg, project, paths, output, Encoder{Lossless: true}
		if err := RenderWithOptions(ctx, opts); err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		return output
	}
	single := render("single.mp4", RenderOptions{SegmentFrames: 100000})
	many := render("many.mp4", RenderOptions{SegmentFrames: 13})
	singleFrames, manyFrames := frameHashes(t, ctx, ffmpeg, single), frameHashes(t, ctx, ffmpeg, many)
	if len(singleFrames) != 158 || len(manyFrames) != 158 {
		t.Fatalf("frame counts %d and %d, want 158", len(singleFrames), len(manyFrames))
	}
	for i := range singleFrames {
		if singleFrames[i] != manyFrames[i] {
			t.Fatalf("frame %d differs between one segment and 13-frame segments", i)
		}
	}
	a, b := decodedSamples(t, ctx, ffmpeg, single), decodedSamples(t, ctx, ffmpeg, many)
	if len(a) != len(b) {
		t.Fatalf("audio lengths %d and %d differ", len(a), len(b))
	}
	var signal, diff float64
	for i := range a {
		signal += float64(a[i]) * float64(a[i])
		d := float64(a[i]) - float64(b[i])
		diff += d * d
	}
	if signal == 0 || math.Sqrt(diff/signal) > 0.01 {
		t.Fatalf("segmented audio differs from the single pass: relative RMS %.4f", math.Sqrt(diff/signal))
	}
	// A range starting inside the dissolve renders the same frames as the full timeline.
	ranged := render("range.mp4", RenderOptions{Start: 65, End: 100, SegmentFrames: 13})
	rangeFrames := frameHashes(t, ctx, ffmpeg, ranged)
	if len(rangeFrames) != 35 {
		t.Fatalf("range has %d frames, want 35", len(rangeFrames))
	}
	for i, hash := range rangeFrames {
		if hash != singleFrames[65+i] {
			t.Fatalf("range frame %d differs from timeline frame %d", i, 65+i)
		}
	}
}

func TestSegmentedRenderSeeksLateOffsetsExactly(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := t.TempDir()
	counter := filepath.Join(root, "counter.mp4")
	// Every source frame N has its own gray level, so a decoded pixel names the frame.
	makeLavfiVideoFixture(t, ctx, ffmpeg, counter, "nullsrc=s=64x32:r=30:d=10,format=yuv420p,geq=lum='mod(N*13,220)+16':cb=128:cr=128")
	asset, err := Probe(ctx, ffmpeg, counter)
	if err != nil {
		t.Fatal(err)
	}
	asset.ID = "counter"
	project := Project{Version: CurrentVersion, Name: "Seek", Width: 720, Height: 720, FPS: 30, Assets: []Asset{asset},
		Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{{ID: "late", AssetID: "counter", Start: 0, Offset: 200, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}}}}}
	output := filepath.Join(root, "seek.mp4")
	if err := RenderWithOptions(ctx, RenderOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: map[string]string{"counter": counter}, Output: output, SegmentFrames: 10}); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		at    string
		frame int
	}{{"0.000000", 200}, {"0.500000", 215}, {"0.933333", 228}} {
		pixel := decodedPixel(t, ctx, ffmpeg, output, check.at, 360, 360)
		want := float64((check.frame*13)%220) * 255 / 219
		if math.Abs(float64(pixel[0])-want) > 6 {
			t.Fatalf("output at %ss shows %v, want source frame %d (gray %.0f)", check.at, pixel, check.frame, want)
		}
	}
}

func TestSegmentedRenderAtOtherFrameRates(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := t.TempDir()
	videoPath, tonePath := filepath.Join(root, "red.mp4"), filepath.Join(root, "tone.wav")
	makeVideoFixture(t, ctx, ffmpeg, videoPath, "red")
	makeAudioFixture(t, ctx, ffmpeg, tonePath)
	video, err := Probe(ctx, ffmpeg, videoPath)
	if err != nil {
		t.Fatal(err)
	}
	tone, err := Probe(ctx, ffmpeg, tonePath)
	if err != nil {
		t.Fatal(err)
	}
	video.ID, tone.ID = "video", "tone"
	for _, c := range []struct {
		fps  int
		ntsc bool
		want string
	}{{25, false, "25/1"}, {30, true, "30000/1001"}, {60, false, "60/1"}} {
		project := Project{Version: CurrentVersion, Name: "Rate", Width: 1280, Height: 720, FPS: c.fps, NTSC: c.ntsc}
		rate := ProjectRate(project)
		project.Assets = []Asset{AssetAtRate(video, rate), AssetAtRate(tone, rate)}
		project.Tracks = []Track{
			{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{{ID: "v", AssetID: "video", Start: 0, Offset: 2, Duration: 20, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}}},
			{ID: "music", Name: "Music", Kind: TrackAudio, Clips: []Clip{{ID: "m", AssetID: "tone", Start: 0, Duration: 20, Volume: 1}}},
		}
		output := filepath.Join(root, strings.ReplaceAll(rate.String(), "/", "_")+".mp4")
		if err := RenderWithOptions(ctx, RenderOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: map[string]string{"video": videoPath, "tone": tonePath}, Output: output, SegmentFrames: 7}); err != nil {
			t.Fatalf("%s fps: %v", rate, err)
		}
		got, err := Probe(ctx, ffmpeg, output)
		if err != nil {
			t.Fatal(err)
		}
		if got.SourceFPS != c.want || !got.HasAudio || math.Abs(float64(got.DurationUs)-rate.Seconds(20)*1e6) > 60_000 {
			t.Fatalf("%s fps output = %+v", rate, got)
		}
	}
}

func TestConcatListDurationsDoNotDrift(t *testing.T) {
	rate := Rate{30000, 1001}
	var names []string
	var segments []Segment
	for i := 0; i < 360; i++ {
		names = append(names, "v.ts")
		segments = append(segments, Segment{100 + i*299, 100 + (i+1)*299})
	}
	var total int64
	for _, line := range strings.Split(concatList(names, segments, rate), "\n") {
		if value, ok := strings.CutPrefix(line, "duration "); ok {
			whole, fraction, _ := strings.Cut(value, ".")
			var seconds, micros int64
			if _, err := fmt.Sscan(whole, &seconds); err != nil {
				t.Fatal(err)
			}
			if _, err := fmt.Sscan(fraction, &micros); err != nil || len(fraction) != 6 {
				t.Fatalf("duration %q", value)
			}
			total += seconds*1_000_000 + micros
		}
	}
	if want := int64(math.Round(rate.Seconds(360*299) * 1e6)); total != want {
		t.Fatalf("summed durations %d µs, want %d µs", total, want)
	}
}

func TestRenderSpaceAndSegmentErrors(t *testing.T) {
	if err := checkRenderSpace(t.TempDir(), Rate{60, 1}, MaxDurationSeconds*60, 16384, 16384, true); !errors.Is(err, ErrInsufficientDiskSpace) {
		t.Fatalf("an impossible render passed the space check: %v", err)
	}
	if err := checkRenderSpace(t.TempDir(), Rate{30, 1}, 30, 64, 32, false); err != nil {
		t.Fatalf("a tiny render failed the space check: %v", err)
	}
	segment := &SegmentError{Index: 11, Start: 120, End: 130, Err: errors.New("boom")}
	if segment.Location() != "video segment 12 (2:00–2:10)" || !strings.Contains(segment.Error(), "boom") || !errors.Is(segment, segment.Err) {
		t.Fatalf("SegmentError = %q / %q", segment.Location(), segment.Error())
	}
}
