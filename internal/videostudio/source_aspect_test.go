package videostudio

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The aspect rule: a video's long side is at most MaxVideoAspect times its short side. Cover-fit scales a source until it
// covers the clip box, so a 16384x16 video in a 1920x1080 box would be scaled to about 1.1 million pixels wide.
func TestValidVideoAspect(t *testing.T) {
	cases := []struct {
		width, height int
		want          bool
	}{
		{1920, 1080, true}, {1080, 1920, true}, {64, 64, true},
		{16384, 1024, true}, {1024, 16384, true}, {160, 10, true},
		{16384, 1023, false}, {1023, 16384, false}, {16384, 16, false}, {16, 16384, false}, {161, 10, false},
		{0, 100, false}, {100, 0, false}, {-100, 10, false},
	}
	for _, c := range cases {
		if got := validVideoAspect(c.width, c.height); got != c.want {
			t.Errorf("validVideoAspect(%d, %d) = %v, want %v", c.width, c.height, got, c.want)
		}
	}
}

// makeShapedFixture writes a short video (libx264) or a still (PNG) of the given size.
func makeShapedFixture(t *testing.T, ctx context.Context, ffmpeg, output string, width, height int, still bool) {
	t.Helper()
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", fmt.Sprintf("color=c=red:s=%dx%d:r=30:d=0.2", width, height)}
	if still {
		args = append(args, "-frames:v", "1", "-f", "image2", output)
	} else {
		args = append(args, "-an", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-f", "mp4", output)
	}
	if out, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("create %dx%d fixture: %v: %s", width, height, err, out)
	}
}

// Import (Probe) refuses a video beyond the rule with ErrSourceAspect, accepts one exactly at it, and leaves stills
// alone: titles are cropped text blocks that can be far wider than tall.
func TestProbeRejectsExtremeAspectVideo(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := t.TempDir()
	wide, edge, still := filepath.Join(root, "wide.mp4"), filepath.Join(root, "edge.mp4"), filepath.Join(root, "banner.png")
	makeShapedFixture(t, ctx, ffmpeg, wide, 16384, 16, false)
	makeShapedFixture(t, ctx, ffmpeg, edge, 1024, 64, false)
	makeShapedFixture(t, ctx, ffmpeg, still, 4096, 64, true)
	if _, err := Probe(ctx, ffmpeg, wide); !errors.Is(err, ErrSourceAspect) {
		t.Fatalf("Probe(16384x16 video) error = %v, want ErrSourceAspect", err)
	}
	if asset, err := Probe(ctx, ffmpeg, edge); err != nil || asset.Kind != AssetVideo || asset.Width != 1024 || asset.Height != 64 {
		t.Fatalf("Probe(1024x64 video, exactly 16:1) = %+v, %v; want it accepted", asset, err)
	}
	if asset, err := Probe(ctx, ffmpeg, still); err != nil || asset.Kind != AssetImage || asset.Width != 4096 {
		t.Fatalf("Probe(4096x64 still, 64:1) = %+v, %v; want it accepted", asset, err)
	}
}

// extremeAspectProject holds a normal clip (frames 0-30) and a cover clip of a 16384x16 video (frames 60-90) that was
// imported before the rule existed. Validate keeps accepting it: only rendering refuses.
func extremeAspectProject() Project {
	project := validProject()
	project.Width, project.Height = 1920, 1080
	project.Assets = append(project.Assets, Asset{ID: "wide", Name: "wide.mp4", Kind: AssetVideo, DurationFrames: 300, Width: 16384, Height: 16})
	project.Tracks[0].Clips = append(project.Tracks[0].Clips, Clip{ID: "w", AssetID: "wide", Start: 60, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitCover})
	return project
}

// An export refuses a used extreme-aspect video before it probes the sources or starts a segment: the staged paths are
// missing here, so reaching collectRenderSources would fail with another error. The stand-in FFmpeg is a plain file that
// cannot run.
func TestRenderRefusesExtremeAspectAssets(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "ffmpeg-stub")
	if err := os.WriteFile(stub, []byte("not a program"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	project := extremeAspectProject()
	if err := Validate(project); err != nil {
		t.Fatalf("a saved project with such an asset must stay valid: %v", err)
	}
	render := func(project Project, start, end int) error {
		return RenderWithOptions(ctx, RenderOptions{FFmpegPath: stub, Project: project, AssetPaths: map[string]string{}, Output: filepath.Join(dir, "out.mp4"), Start: start, End: end})
	}
	err := render(project, 0, 0)
	if !errors.Is(err, ErrSourceAspect) {
		t.Fatalf("export error = %v, want ErrSourceAspect", err)
	}
	if !strings.Contains(err.Error(), "wide.mp4") || !strings.Contains(err.Error(), "16384x16") {
		t.Fatalf("export error %q does not name the asset and its size", err)
	}
	if err := render(project, 60, 90); !errors.Is(err, ErrSourceAspect) {
		t.Fatalf("range over the clip: error = %v, want ErrSourceAspect", err)
	}
	if err := render(project, 0, 30); errors.Is(err, ErrSourceAspect) {
		t.Fatalf("a range without the clip was refused for its aspect: %v", err)
	}
	disabled := extremeAspectProject()
	disabled.Tracks[0].Clips[1].Disabled = true
	if err := render(disabled, 0, 0); errors.Is(err, ErrSourceAspect) {
		t.Fatalf("a disabled clip was refused for its aspect: %v", err)
	}
	// A frame (golden frames) checks the clips that show it.
	frame := func(n int) error {
		return RenderFrame(ctx, FrameOptions{FFmpegPath: stub, Project: project, AssetPaths: map[string]string{}, Frame: n, Output: filepath.Join(dir, "frame.png")})
	}
	if err := frame(70); !errors.Is(err, ErrSourceAspect) {
		t.Fatalf("RenderFrame over the clip: error = %v, want ErrSourceAspect", err)
	}
	if err := frame(10); errors.Is(err, ErrSourceAspect) {
		t.Fatalf("RenderFrame of the normal clip was refused for its aspect: %v", err)
	}
}

// The cover bound stays within a fixed multiple of the canvas, whatever the clip's boxes and the source look like. Without
// the clamp, a box 2 px tall (aspect 960) next to a 1080 px tall one made a 1.04 million pixel wide bound.
func TestCoverBoundStaysNearTheCanvas(t *testing.T) {
	const canvasWidth, canvasHeight = 1920, 1080
	// The longest picture side is 16 times a box side, plus the margin (1/8 and 4 px).
	limit := func(side int) int { return int(math.Ceil(maxCoverAspect*float64(side)*1.125)) + 4 }
	heights := []int{2, 10, 540, 1080}
	widths := []int{2, 10, 960, 1920}
	for _, source := range []float64{0, 1.0 / 1024, 1.0 / 16, 0.5, 1, 16.0 / 9, 16, 1024} {
		var boxW, boxH []int
		for _, w := range widths {
			for _, h := range heights {
				boxW, boxH = append(boxW, w), append(boxH, h)
			}
		}
		w, h, low, high := coverBound(boxW, boxH, source)
		if w > limit(canvasHeight) || h > limit(canvasWidth) {
			t.Errorf("source aspect %v: bound %dx%d, want at most %dx%d", source, w, h, limit(canvasHeight), limit(canvasWidth))
		}
		if low < 1.0/maxCoverAspect || high > maxCoverAspect || low > high {
			t.Errorf("source aspect %v: clamp range %v..%v leaves 1/%d..%d", source, low, high, maxCoverAspect, maxCoverAspect)
		}
	}
	// The case of the review: boxes 1920x1080 and 1920x2 and a 1024:1 source stay near the canvas, and a clamp range that
	// holds all legitimate boxes (1:1 to 4:1) is unchanged.
	w, h, low, high := coverBound([]int{1920, 1920}, []int{1080, 2}, 1024)
	if area := w * h; area > 16*canvasWidth*canvasHeight {
		t.Errorf("review case: bound %dx%d has %d times the canvas area, want at most 16", w, h, area/(canvasWidth*canvasHeight))
	}
	if high != maxCoverAspect || math.Abs(low-1920.0/1080) > 1e-9 {
		t.Errorf("review case: clamp range %v..%v, want %v..%d", low, high, 1920.0/1080, maxCoverAspect)
	}
}

// A keyed cover clip whose height animates to a sliver, over an extreme source, builds a bounded graph: the crop clamps the
// source to 16:1 and the pad bound stays within the multiple of the canvas.
func TestKeyedCoverOfExtremeSourceIsBounded(t *testing.T) {
	clip := Clip{ID: "c", AssetID: "wide", Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitCover,
		Keyframes: map[string][]Keyframe{"height": {{F: 0, V: 1, E: EaseLinear}, {F: 29, V: 0.0001, E: EaseLinear}}}}
	chain := clipChain{item: layerClip{clip: clip, whole: clip, sourceAspect: 1024}, canvasWidth: 1920, canvasHeight: 1080, rate: Rate{Num: 30, Den: 1}}
	g := chain.geometry()
	if g.boundWidth > 19500 || g.boundHeight > 1300 || g.high > maxCoverAspect || g.low < 1.0/maxCoverAspect {
		t.Fatalf("bound %dx%d, clamp %v..%v; want at most 19500x1300 and 1/16..16", g.boundWidth, g.boundHeight, g.low, g.high)
	}
	filter := chain.filter("out")
	if !strings.Contains(filter, "crop=w='min(iw,ih*16)':h='min(ih,iw/1.7777") {
		t.Fatalf("the cover crop does not clamp the source aspect to 16:\n%s", filter)
	}
	if !strings.Contains(filter, fmt.Sprintf("pad=w=%d:h=%d:", g.boundWidth, g.boundHeight)) {
		t.Fatalf("the bound pad is missing:\n%s", filter)
	}
}
