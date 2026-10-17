package videostudio

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"
)

// renderRawRGB renders seg of project through the export's video graph with the RGB tail (buildVideoSegmentWith,
// stageVideoGraph and runFFmpegIn, command files included) and returns one rgb24 frame per timeline frame at the
// project size.
func renderRawRGB(t *testing.T, ctx context.Context, ffmpeg string, project Project, paths map[string]string, seg Segment) [][]byte {
	t.Helper()
	sources, err := collectRenderSources(ctx, ffmpeg, project, paths)
	if err != nil {
		t.Fatal(err)
	}
	rate := ProjectRate(project)
	inputs, graph, commands, err := buildVideoSegmentWith(project, seg, sources, rate, videoTail{width: project.Width, height: project.Height, rgb: true})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	workDir, graphPath, err := stageVideoGraph(root, graph, rate, commands)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)
	frames := seg.End - seg.Start
	output := filepath.Join(root, "frames.rgb")
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-copyts", "-start_at_zero"}
	args = append(args, segmentInputArgs(inputs, rate)...)
	args = append(args, "-filter_complex_script", graphPath, "-map", "[vs_video_out]", "-frames:v", strconv.Itoa(frames),
		"-fps_mode", "passthrough", "-f", "rawvideo", "-pix_fmt", "rgb24", output)
	if err := runFFmpegIn(ctx, workDir, ffmpeg, args, rate.Seconds(frames), nil, inputPaths(inputs), output); err != nil {
		t.Fatalf("render frames %d–%d: %v", seg.Start, seg.End, err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	size := project.Width * project.Height * 3
	if len(data) != size*frames {
		t.Fatalf("raw output has %d bytes, want %d frames of %d", len(data), frames, size)
	}
	out := make([][]byte, frames)
	for i := range out {
		out[i] = data[i*size : (i+1)*size]
	}
	return out
}

// redBounds is the bounding box of the pure red pixels of an rgb24 frame.
func redBounds(frame []byte, width, height int) (image.Rectangle, bool) {
	var bounds image.Rectangle
	found := false
	for y := range height {
		for x := range width {
			i := (y*width + x) * 3
			if frame[i] < 128 || frame[i+1] > 63 || frame[i+2] > 63 {
				continue
			}
			pixel := image.Rect(x, y, x+1, y+1)
			if !found {
				bounds, found = pixel, true
			} else {
				bounds = bounds.Union(pixel)
			}
		}
	}
	return bounds, found
}

// keyframedBox is the Go box of a keyframed clip at a timeline frame: clipBox over ValueAt.
func keyframedBox(t *testing.T, clip Clip, frame, canvasWidth, canvasHeight int) image.Rectangle {
	t.Helper()
	value := func(path string) float64 {
		v, ok := ValueAt(clip, path, frame)
		if !ok {
			t.Fatalf("ValueAt(%q) reports an unknown path", path)
		}
		return v
	}
	x, y, w, h := clipBox(Clip{X: value("x"), Y: value("y"), Width: value("width"), Height: value("height")}, canvasWidth, canvasHeight)
	return image.Rect(x, y, x+w, y+h)
}

// A red still on black with keyframed x, y, width and height. On every frame of one FFmpeg process the red area must
// be either:
//   - the Go box (cover, through the crop commands), or
//   - a centred fit that fills one axis of it (contain).
//
// A window that starts inside the clip renders the same frames (absolute frames are baked in). This runs on local
// FFmpeg 8.1.1, and the 5.1 container repeats it in Task 15. If cover fails on either version, stop and report
// (plan 2b open question 12).
func TestRenderKeyframedBoxFollowsValueAt(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{"red": filepath.Join(root, "red.png")}
	writeFramePNG(t, paths["red"], 640, 360, color.NRGBA{R: 255, A: 255})
	assets := probeFrameAssets(t, ctx, ffmpeg, paths, "red")
	for _, fit := range []FitMode{FitContain, FitCover} {
		t.Run(string(fit), func(t *testing.T) {
			clip := Clip{ID: "box", AssetID: "red", Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: fit,
				Keyframes: map[string][]Keyframe{
					"x":      {{F: 0, V: 0.05, E: EaseInOut}, {F: 24, V: 0.45, E: EaseLinear}},
					"y":      {{F: 2, V: 0.1, E: EaseLinear}, {F: 20, V: 0.3, E: EaseLinear}},
					"width":  {{F: 0, V: 0.3, E: EaseOut}, {F: 24, V: 0.5, E: EaseLinear}},
					"height": {{F: 0, V: 0.25, E: EaseLinear}, {F: 18, V: 0.6, E: EaseHold}, {F: 26, V: 0.4, E: EaseLinear}},
				}}
			project := Project{Version: CurrentVersion, Name: "Box", Width: 720, Height: 720, FPS: 25, Assets: assets,
				Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{clip}}}}
			if err := Validate(project); err != nil {
				t.Fatalf("keyframed box project invalid: %v", err)
			}
			frames := renderRawRGB(t, ctx, ffmpeg, project, paths, Segment{Start: 0, End: 30})
			for k, frame := range frames {
				box := keyframedBox(t, clip, k, 720, 720)
				got, ok := redBounds(frame, 720, 720)
				if !ok {
					t.Fatalf("frame %d shows no red", k)
				}
				if fit == FitCover {
					if got != box {
						t.Fatalf("frame %d: red area %v, want the box %v", k, got, box)
					}
					continue
				}
				left, right := got.Min.X-box.Min.X, box.Max.X-got.Max.X
				top, bottom := got.Min.Y-box.Min.Y, box.Max.Y-got.Max.Y
				if !got.In(box) || left != right || top != bottom || (left != 0 && top != 0) {
					t.Fatalf("frame %d: red area %v is not a centred fit filling the box %v", k, got, box)
				}
			}
			late := renderRawRGB(t, ctx, ffmpeg, project, paths, Segment{Start: 11, End: 30})
			for i, frame := range late {
				if !bytes.Equal(frame, frames[11+i]) {
					t.Fatalf("frame %d differs when its window starts at frame 11", 11+i)
				}
			}
		})
	}
}

// keyframeSegmentProject has two keyframed clips joined by a dissolve (frames 41–51) and a keyframed overlay:
//   - a: contain, with rotation;
//   - b: cover, with opacity;
//   - the overlay: its own opacity keyframes.
//
// 7- and 13-frame segments therefore cut windows inside clips and next to the transition. One-frame pieces in front of
// a concat input are TestSegmentedRenderOneFramePieces' subject.
func keyframeSegmentProject(t *testing.T, ctx context.Context, ffmpeg, root string, ntsc bool) (Project, map[string]string) {
	t.Helper()
	paths := map[string]string{"motion": filepath.Join(root, "motion.mp4"), "logo": filepath.Join(root, "logo.png")}
	if _, err := os.Stat(paths["motion"]); err != nil {
		makeLavfiVideoFixture(t, ctx, ffmpeg, paths["motion"], "testsrc2=s=320x180:r=25:d=6")
		writeFramePNG(t, paths["logo"], 120, 80, color.NRGBA{R: 40, G: 200, B: 90, A: 200})
	}
	project := Project{Version: CurrentVersion, Name: "Keyframe segments", Width: 720, Height: 720, FPS: 25}
	if ntsc {
		project.FPS, project.NTSC = 30, true
	}
	rate := ProjectRate(project)
	for _, id := range []string{"motion", "logo"} {
		asset, err := Probe(ctx, ffmpeg, paths[id])
		if err != nil {
			t.Fatalf("Probe(%s): %v", id, err)
		}
		asset.ID = id
		project.Assets = append(project.Assets, AssetAtRate(asset, rate))
	}
	a := Clip{ID: "a", AssetID: "motion", Duration: 51, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain,
		Transition: &Transition{Type: TransitionDissolve, Duration: 10},
		Keyframes: map[string][]Keyframe{
			"x":        {{F: 0, V: 0, E: EaseInOut}, {F: 40, V: 0.3, E: EaseLinear}},
			"width":    {{F: 0, V: 0.5, E: EaseOut}, {F: 40, V: 0.7, E: EaseLinear}},
			"height":   {{F: 0, V: 0.5, E: EaseLinear}},
			"rotation": {{F: 0, V: 0, E: EaseIn}, {F: 45, V: 30, E: EaseLinear}},
		}}
	b := Clip{ID: "b", AssetID: "motion", Start: 41, Offset: 30, Duration: 49, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitCover,
		Keyframes: map[string][]Keyframe{
			"y":       {{F: 30, V: 0.1, E: EaseHold}, {F: 55, V: 0.2, E: EaseLinear}},
			"width":   {{F: 30, V: 0.4, E: EaseLinear}, {F: 79, V: 0.9, E: EaseInOut}},
			"height":  {{F: 30, V: 0.5, E: EaseLinear}, {F: 79, V: 0.8, E: EaseLinear}},
			"opacity": {{F: 30, V: 0.4, E: EaseLinear}, {F: 60, V: 1, E: EaseLinear}},
		}}
	logo := Clip{ID: "logo", AssetID: "logo", Start: 10, Duration: 70, X: 0.6, Y: 0.1, Width: 0.25, Height: 0.2, Opacity: 0.9, Volume: 1, Fit: FitContain,
		Keyframes: map[string][]Keyframe{
			"x":       {{F: 0, V: 0.6, E: EaseLinear}, {F: 69, V: 0.1, E: EaseLinear}},
			"opacity": {{F: 0, V: 0, E: EaseOut}, {F: 20, V: 0.9, E: EaseLinear}},
		}}
	project.Tracks = []Track{
		{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{a, b}},
		{ID: "overlay", Name: "Overlay", Kind: TrackOverlay, Clips: []Clip{logo}},
	}
	if err := Validate(project); err != nil {
		t.Fatalf("keyframe segment project invalid: %v", err)
	}
	return project, paths
}

// Keyframed geometry (expressions) and opacity and cover crops (sendcmd) render the same frames in one segment and in
// 7- and 13-frame segments, at 25 and 30000/1001 fps.
func TestSegmentedRenderKeyframes(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	root := t.TempDir()
	for _, ntsc := range []bool{false, true} {
		project, paths := keyframeSegmentProject(t, ctx, ffmpeg, root, ntsc)
		render := func(name string, segmentFrames int) []string {
			output := filepath.Join(root, fmt.Sprintf("%s-%t.mp4", name, ntsc))
			if err := RenderWithOptions(ctx, RenderOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Output: output,
				SegmentFrames: segmentFrames, Encoder: Encoder{Lossless: true}}); err != nil {
				t.Fatalf("render %s (NTSC %t): %v", name, ntsc, err)
			}
			return frameHashes(t, ctx, ffmpeg, output)
		}
		single := render("single", 100000)
		if len(single) != 90 {
			t.Fatalf("NTSC %t: %d frames, want 90", ntsc, len(single))
		}
		for _, size := range []int{7, 13} {
			if got := render(fmt.Sprintf("segments-%d", size), size); !reflect.DeepEqual(got, single) {
				t.Fatalf("NTSC %t: %d-frame segments differ from the single pass", ntsc, size)
			}
		}
	}
}
