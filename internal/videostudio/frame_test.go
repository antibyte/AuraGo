package videostudio

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRenderFrameGraphEndsInRGB(t *testing.T) {
	clip := func(id string, start, offset, duration int, transition *Transition) Clip {
		return Clip{ID: id, AssetID: "v", Start: start, Offset: offset, Duration: duration, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Transition: transition}
	}
	project := Project{Version: CurrentVersion, Name: "Graph", Width: 1280, Height: 720, FPS: 25,
		Assets: []Asset{{ID: "v", Name: "v.mp4", Kind: AssetVideo, DurationFrames: 900, Width: 64, Height: 36}},
		Tracks: []Track{{ID: "t", Name: "Video", Kind: TrackVideo, Clips: []Clip{
			clip("a", 0, 100, 60, &Transition{Type: TransitionDissolve, Duration: 10}),
			clip("b", 50, 0, 60, nil),
		}}}}
	rate := ProjectRate(project)
	sources := map[string]renderSource{"v": {asset: project.Assets[0], path: "v.mp4", videoEnd: 30}}

	// The encoder wrapper produces exactly the generalized builder's graph, hardware upload included.
	_, wrapped, _, err := buildVideoSegment(project, Segment{55, 80}, sources, rate, 1280, 720, ",format=nv12,hwupload")
	if err != nil {
		t.Fatal(err)
	}
	_, direct, _, err := buildVideoSegmentWith(project, Segment{55, 80}, sources, rate, videoTail{width: 1280, height: 720, upload: ",format=nv12,hwupload"})
	if err != nil {
		t.Fatal(err)
	}
	if wrapped != direct || !strings.HasSuffix(wrapped, ",scale=out_color_matrix=bt709:out_range=tv,format=yuv420p,format=nv12,hwupload[vs_video_out]") {
		t.Fatalf("wrapper graph differs or lost its YUV tail:\n%s", wrapped)
	}
	// One frame inside the dissolve (50–60): the window covers the whole transition, keeps its frame 5, and the
	// tail stays 8-bit RGB without any YUV conversion.
	_, graph, _, err := buildVideoSegmentWith(project, Segment{55, 56}, sources, rate, videoTail{width: 1280, height: 720, rgb: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(graph, "xfade=transition=fade") || !strings.HasSuffix(graph, "trim=start_frame=5:end_frame=6,setpts=PTS-STARTPTS,format=rgb24[vs_video_out]") || strings.Contains(graph, "yuv420p") {
		t.Fatalf("RGB frame graph:\n%s", graph)
	}
	_, graph, _, err = buildVideoSegmentWith(project, Segment{10, 11}, sources, rate, videoTail{width: 640, height: 360, rgb: true})
	if err != nil || !strings.HasSuffix(graph, ",setpts=PTS-STARTPTS,scale=w=640:h=360:flags=bicubic,format=rgb24[vs_video_out]") {
		t.Fatalf("scaled RGB frame graph (%v):\n%s", err, graph)
	}

	// Bad options are refused before FFmpeg runs.
	dir := t.TempDir()
	taken := filepath.Join(dir, "taken.png")
	if err := os.WriteFile(taken, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for i, bad := range []FrameOptions{
		{Project: project, Frame: EnabledDuration(project), Output: filepath.Join(dir, "end.png")},
		{Project: project, Frame: -1, Output: filepath.Join(dir, "negative.png")},
		{Project: project, Width: 641, Height: 360, Output: filepath.Join(dir, "odd.png")},
		{Project: project, Output: taken},
		{Project: project, Output: filepath.Join(dir, "frame.jpg")},
		{Project: Project{Version: CurrentVersion}, Output: filepath.Join(dir, "invalid.png")},
	} {
		if err := RenderFrame(context.Background(), bad); err == nil {
			t.Errorf("bad frame options %d were accepted", i)
		}
	}
}

func TestRenderFrameWritesLosslessRGB(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{"red": filepath.Join(root, "red.png"), "blue": filepath.Join(root, "blue.png")}
	writeFramePNG(t, paths["red"], 640, 360, color.NRGBA{R: 255, A: 255})
	writeFramePNG(t, paths["blue"], 640, 360, color.NRGBA{B: 255, A: 255})
	// A 640x360 still in a 640x360 box at (320, 180): nothing is resampled, so its pixels must arrive exactly.
	box := func(id string, start, duration int, transition *Transition) Clip {
		return Clip{ID: id, AssetID: id, Start: start, Duration: duration, X: 0.25, Y: 0.25, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: FitContain, Transition: transition}
	}
	project := Project{Version: CurrentVersion, Name: "Frame", Width: 1280, Height: 720, FPS: 25, Assets: probeFrameAssets(t, ctx, ffmpeg, paths, "red", "blue"),
		Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{
			box("red", 0, 50, &Transition{Type: TransitionDissolve, Duration: 10}), box("blue", 40, 50, nil)}}}}
	render := func(name string, frame, width, height int) image.Image {
		return renderFrameImage(t, ctx, FrameOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Frame: frame, Width: width, Height: height, Output: filepath.Join(root, name)})
	}

	still := render("still.png", 10, 0, 0)
	if depth, kind := pngHeader(t, filepath.Join(root, "still.png")); depth != 8 || kind != 2 {
		t.Fatalf("PNG bit depth %d, colour type %d; want 8-bit RGB (colour type 2)", depth, kind)
	}
	if size := still.Bounds().Size(); size.X != 1280 || size.Y != 720 {
		t.Fatalf("frame size %v, want 1280x720", size)
	}
	if got := framePixel(still, 640, 360); got != (color.NRGBA{R: 255, A: 255}) {
		t.Fatalf("centre pixel = %v, want exactly (255, 0, 0)", got)
	}
	if got := framePixel(still, 100, 100); got != (color.NRGBA{A: 255}) {
		t.Fatalf("pixel outside the box = %v, want exactly black", got)
	}
	// Frame 45 lies inside the dissolve (40–50): the whole transition is composed and its middle frame kept.
	if mid := framePixel(render("dissolve.png", 45, 0, 0), 640, 360); mid.R < 100 || mid.R > 156 || mid.B < 100 || mid.B > 156 || mid.G > 3 {
		t.Fatalf("dissolve midpoint = %v, want an even red/blue mix", mid)
	}
	small := render("small.png", 10, 640, 360)
	if size := small.Bounds().Size(); size.X != 640 || size.Y != 360 {
		t.Fatalf("scaled frame size %v, want 640x360", size)
	}
	if got := framePixel(small, 320, 180); got.R < 253 || got.G > 2 || got.B > 2 {
		t.Fatalf("scaled centre pixel = %v, want red", got)
	}
}

// writeFramePNG writes a solid straight-alpha PNG fixture.
func writeFramePNG(t *testing.T, path string, width, height int, fill color.NRGBA) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = fill.R, fill.G, fill.B, fill.A
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

// probeFrameAssets probes staged fixtures and names each asset after its key in paths.
func probeFrameAssets(t *testing.T, ctx context.Context, ffmpeg string, paths map[string]string, ids ...string) []Asset {
	t.Helper()
	assets := make([]Asset, 0, len(ids))
	for _, id := range ids {
		asset, err := Probe(ctx, ffmpeg, paths[id])
		if err != nil {
			t.Fatalf("Probe(%s): %v", id, err)
		}
		asset.ID = id
		assets = append(assets, asset)
	}
	return assets
}

// renderFrameImage renders one frame with RenderFrame and decodes the PNG.
func renderFrameImage(t *testing.T, ctx context.Context, opts FrameOptions) image.Image {
	t.Helper()
	if err := RenderFrame(ctx, opts); err != nil {
		t.Fatalf("RenderFrame(frame %d): %v", opts.Frame, err)
	}
	return decodeFramePNG(t, opts.Output)
}

func decodeFramePNG(t *testing.T, path string) image.Image {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return img
}

// pngHeader returns the IHDR bit depth and colour type (2 = 8-bit truecolour RGB without alpha).
func pngHeader(t *testing.T, path string) (depth, colourType byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 26 || string(data[1:4]) != "PNG" || string(data[12:16]) != "IHDR" {
		t.Fatalf("%s is not a PNG", path)
	}
	return data[24], data[25]
}

func framePixel(img image.Image, x, y int) color.NRGBA {
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
}
